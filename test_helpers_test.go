package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// =============================================================================
// MOCK TWILIO CLIENT
// =============================================================================

type MockTwilioClient struct {
	mu          sync.Mutex
	LastSMSTo   string
	LastSMSBody string
	SMSCount    int

	SMSErr error
}

func (m *MockTwilioClient) SendSMS(from, to, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastSMSTo = to
	m.LastSMSBody = body
	m.SMSCount++
	return m.SMSErr
}

// GetSMSCount returns the total number of SMS messages sent.
func (m *MockTwilioClient) GetSMSCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SMSCount
}

// GetLastSMS returns the last SMS recipient and body in a thread-safe way.
func (m *MockTwilioClient) GetLastSMS() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.LastSMSTo, m.LastSMSBody
}

// =============================================================================
// TEST DATABASE SETUP
//
// Since initDB uses PostgreSQL-specific DDL (SERIAL, TIMESTAMPTZ, JSONB),
// we maintain a SQLite-compatible schema for tests. This creates all the
// tables the tests need using SQLite syntax.
// =============================================================================

// initTestDB creates all tables using SQLite-compatible DDL. This mirrors
// schema.go's initDB but uses SQLite types instead of PostgreSQL types.
func initTestDB(db *sql.DB) error {
	if postgresTestURL() != "" {
		// Postgres tier: the database is built from the production schema, so a
		// re-init means running the production migration again.
		return initDB(db)
	}
	db.Exec("PRAGMA foreign_keys = ON")

	// Users table (pennyai-owned — see schema.go initDB).
	// SQLite DDL here mirrors the PostgreSQL CREATE in schema.go so tests catch
	// drift between the two. When schema.go adds a column, add it here too.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id          TEXT PRIMARY KEY,
			phone       TEXT UNIQUE,
			email       TEXT DEFAULT '',
			display_name TEXT DEFAULT '',
			name        TEXT DEFAULT '',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			household_id INTEGER,
			referral_code TEXT,
			referral_points INTEGER DEFAULT 0,
			last_active DATETIME,
			login_count INTEGER NOT NULL DEFAULT 0,
			address TEXT NOT NULL DEFAULT '',
			penny_brand_mode TEXT NOT NULL DEFAULT 'ask',
			deal_accept_count INTEGER NOT NULL DEFAULT 0,
			deal_shown_count INTEGER NOT NULL DEFAULT 0,
			penny_slider INTEGER NOT NULL DEFAULT 15,
			penny_autotune INTEGER NOT NULL DEFAULT 1,
			penny_deals INTEGER NOT NULL DEFAULT 0,
			last_cleared_items TEXT NOT NULL DEFAULT '[]',
			swap_credits_cents INTEGER NOT NULL DEFAULT 0,
			sms_consent_at DATETIME,
			sms_consent_ip TEXT NOT NULL DEFAULT '',
			sms_consent_user_agent TEXT NOT NULL DEFAULT '',
			onboarding_flags TEXT NOT NULL DEFAULT '{}',
			profile_photo TEXT NOT NULL DEFAULT '',
			name_source TEXT NOT NULL DEFAULT 'auto',
			referral_source TEXT NOT NULL DEFAULT '',
			nps_dismissed_at TEXT NOT NULL DEFAULT '',
			store_order_enabled INTEGER NOT NULL DEFAULT 1,
			daily_digest_enabled INTEGER NOT NULL DEFAULT 0,
			daily_digest_opted_in_at DATETIME,
			daily_digest_last_sent DATETIME,
			merged INTEGER NOT NULL DEFAULT 0,
			clerk_user_id TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("creating users table: %w", err)
	}

	// Mirrors the partial unique index schema.go creates on Postgres. Without
	// it the Clerk tests would exercise a schema production does not have.
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_clerk_user_id ON users(clerk_user_id) WHERE clerk_user_id IS NOT NULL"); err != nil {
		return fmt.Errorf("creating idx_users_clerk_user_id: %w", err)
	}

	// Sessions table (pennyai-owned — see schema.go initDB).
	// Session cookies live on .groflo.io only; no cross-domain handoff.
	// `role` is read by penny as auth_method for reporting.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			phone      TEXT DEFAULT '',
			user_id    TEXT,
			role       TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL,
			last_used  DATETIME,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating sessions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS list_items (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			name       TEXT NOT NULL,
			department TEXT NOT NULL DEFAULT 'Other',
			checked    INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			brand      TEXT NOT NULL DEFAULT '',
			quantity   INTEGER NOT NULL DEFAULT 1,
			size       TEXT NOT NULL DEFAULT '',
			source     TEXT NOT NULL DEFAULT 'app',
			urgent     INTEGER NOT NULL DEFAULT 0,
			claimed_by TEXT DEFAULT NULL,
			claimed_at DATETIME DEFAULT NULL,
			notes      TEXT NOT NULL DEFAULT '',
			swap_savings_cents INTEGER NOT NULL DEFAULT 0,
			emoji      TEXT NOT NULL DEFAULT '',
			needs_enrichment INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating list_items table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_lists (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			name       TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			store_name TEXT NOT NULL DEFAULT '',
			is_master  INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating saved_lists table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_list_items (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			saved_list_id INTEGER NOT NULL,
			name          TEXT NOT NULL,
			brand         TEXT NOT NULL DEFAULT '',
			department    TEXT NOT NULL DEFAULT 'Other',
			quantity      INTEGER NOT NULL DEFAULT 1,
			size          TEXT NOT NULL DEFAULT '',
			notes         TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (saved_list_id) REFERENCES saved_lists(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating saved_list_items table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS households (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			name       TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating households table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS household_members (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			household_id INTEGER NOT NULL,
			user_id      TEXT NOT NULL UNIQUE,
			role         TEXT NOT NULL DEFAULT 'member',
			invited_by   TEXT,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating household_members table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS invites (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			household_id INTEGER NOT NULL,
			code         TEXT UNIQUE NOT NULL,
			created_by   TEXT NOT NULL,
			expires_at   DATETIME NOT NULL,
			used_by      TEXT,
			used_at      DATETIME,
			notified     INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (created_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating invites table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS item_modifications (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id    INTEGER NOT NULL,
			user_id    TEXT,
			action     TEXT NOT NULL,
			detail     TEXT,
			source     TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (item_id) REFERENCES list_items(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating item_modifications table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS referrals (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			referrer_id TEXT NOT NULL,
			referred_id TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (referrer_id) REFERENCES users(id),
			FOREIGN KEY (referred_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating referrals table: %w", err)
	}
	// Mirrors schema.go idx_referrals_unique. Without it the referral
	// idempotency tests passed against code that double-credits.
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_referrals_unique ON referrals(referrer_id, referred_id)")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS redemptions (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      TEXT NOT NULL,
			points       INTEGER NOT NULL,
			amount_cents INTEGER NOT NULL,
			status       TEXT DEFAULT 'pending',
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating redemptions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nps_responses (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			score      INTEGER NOT NULL,
			comment    TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating nps_responses table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_methods (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id       TEXT NOT NULL,
			provider      TEXT NOT NULL,
			provider_id   TEXT NOT NULL,
			password_hash TEXT DEFAULT NULL,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(provider, provider_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating auth_methods table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_resets (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			token      TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			used       INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating password_resets table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_owners (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_owners table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brands (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT UNIQUE NOT NULL,
			domain     TEXT NOT NULL,
			owner_id   INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES brand_owners(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brands table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_logos (
			brand_id      INTEGER PRIMARY KEY,
			logo_data     BLOB NOT NULL,
			content_type  TEXT NOT NULL DEFAULT 'image/png',
			fetched_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			source        TEXT NOT NULL DEFAULT 'logodev',
			FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_logos table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shopping_trips (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      TEXT NOT NULL,
			household_id INTEGER NOT NULL,
			trip_date    TEXT NOT NULL,
			trip_time    TEXT DEFAULT NULL,
			time_label   TEXT DEFAULT NULL,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			store_id     INTEGER DEFAULT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (household_id) REFERENCES households(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating shopping_trips table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_choices (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          TEXT NOT NULL,
			product_category TEXT NOT NULL,
			brand            TEXT NOT NULL,
			source           TEXT NOT NULL DEFAULT 'disambiguation',
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_choices table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_preferences (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          TEXT NOT NULL,
			product_category TEXT NOT NULL,
			brand            TEXT NOT NULL,
			streak           INTEGER NOT NULL DEFAULT 1,
			pick_count       INTEGER NOT NULL DEFAULT 1,
			last_picked      DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(user_id, product_category)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_preferences table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_deals (
			id                 INTEGER PRIMARY KEY AUTOINCREMENT,
			competitor_product TEXT NOT NULL,
			offer_brand        TEXT NOT NULL,
			offer_product      TEXT NOT NULL,
			discount_cents     INTEGER NOT NULL DEFAULT 0,
			daily_budget_cents INTEGER NOT NULL DEFAULT 0,
			total_budget_cents INTEGER NOT NULL DEFAULT 0,
			budget_spent_cents INTEGER NOT NULL DEFAULT 0,
			status             TEXT NOT NULL DEFAULT 'active',
			starts_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ends_at            DATETIME,
			created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			offer_price_cents      INTEGER NOT NULL DEFAULT 0,
			offer_size             TEXT NOT NULL DEFAULT '',
			offer_unit_price       TEXT NOT NULL DEFAULT '',
			competitor_price_cents INTEGER NOT NULL DEFAULT 0,
			competitor_size        TEXT NOT NULL DEFAULT '',
			competitor_unit_price  TEXT NOT NULL DEFAULT '',
			unit_label             TEXT NOT NULL DEFAULT '',
			why_bullets            TEXT NOT NULL DEFAULT '[]',
			allergen_notes         TEXT NOT NULL DEFAULT '',
			deal_type              TEXT NOT NULL DEFAULT 'sku_swap',
			quantity_offered       INTEGER NOT NULL DEFAULT 1,
			trial_product_name     TEXT NOT NULL DEFAULT '',
			trial_size             TEXT NOT NULL DEFAULT '',
			campaign_tag           TEXT NOT NULL DEFAULT '',
			brand_user_id          INTEGER DEFAULT NULL,
			category_id            INTEGER NOT NULL DEFAULT 0,
			product_description    TEXT NOT NULL DEFAULT '',
			key_benefits           TEXT NOT NULL DEFAULT '[]',
			brand_story            TEXT NOT NULL DEFAULT '',
			review_status          TEXT NOT NULL DEFAULT '',
			review_note            TEXT NOT NULL DEFAULT '',
			reviewed_by            TEXT NOT NULL DEFAULT '',
			reviewed_at            DATETIME,
			submitted_at           DATETIME,
			max_rebate_cents       INTEGER NOT NULL DEFAULT 0,
			platform_fee_cents     INTEGER NOT NULL DEFAULT 100,
			require_verified_identity INTEGER NOT NULL DEFAULT 1,
			receipt_grace_days     INTEGER NOT NULL DEFAULT 7,
			upc                    TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_deals table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL REFERENCES users(id),
			deal_id    INTEGER NOT NULL REFERENCES brand_deals(id),
			item_id    INTEGER,
			action     TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			decline_reason TEXT NOT NULL DEFAULT '',
			time_to_action_ms INTEGER NOT NULL DEFAULT 0,
			why_tapped INTEGER NOT NULL DEFAULT 0,
			conversation_depth INTEGER NOT NULL DEFAULT 0,
			why_tapped_at DATETIME DEFAULT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_events table: %w", err)
	}
	// Mirrors schema.go idx_swap_events_one_accept.
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_swap_events_one_accept
		ON swap_events(user_id, deal_id) WHERE action = 'accepted'`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_suppressions (
			user_id       TEXT NOT NULL REFERENCES users(id),
			deal_id       INTEGER NOT NULL REFERENCES brand_deals(id),
			decline_count INTEGER NOT NULL DEFAULT 0,
			UNIQUE(user_id, deal_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_suppressions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_rebates (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id        TEXT NOT NULL REFERENCES users(id),
			swap_event_id  INTEGER NOT NULL REFERENCES swap_events(id),
			deal_id        INTEGER NOT NULL REFERENCES brand_deals(id),
			amount_cents   INTEGER NOT NULL,
			status         TEXT NOT NULL DEFAULT 'pending',
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			attachment_path TEXT NOT NULL DEFAULT '',
			verified_amount_cents INTEGER NOT NULL DEFAULT 0,
			verified_at    DATETIME,
			receipt_image_url TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_rebates table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profile_photos (
			user_id      TEXT PRIMARY KEY REFERENCES users(id),
			photo_data   BLOB NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'image/jpeg',
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating profile_photos table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipts (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id       TEXT NOT NULL REFERENCES users(id),
			image_hash    TEXT NOT NULL,
			items_found   INTEGER NOT NULL DEFAULT 0,
			items_matched INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			store_name    TEXT NOT NULL DEFAULT '',
			store_address TEXT NOT NULL DEFAULT '',
			transaction_date TEXT NOT NULL DEFAULT '',
			subtotal_cents INTEGER NOT NULL DEFAULT 0,
			tax_cents      INTEGER NOT NULL DEFAULT 0,
			total_cents    INTEGER NOT NULL DEFAULT 0,
			status         TEXT NOT NULL DEFAULT 'processed',
			swaps_verified INTEGER NOT NULL DEFAULT 0,
			store_id       INTEGER DEFAULT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating receipts table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shared_links (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			code         TEXT NOT NULL UNIQUE,
			user_id      TEXT NOT NULL,
			household_id INTEGER,
			invite_code  TEXT,
			expires_at   DATETIME NOT NULL,
			view_count   INTEGER DEFAULT 0,
			saved_list_id INTEGER DEFAULT NULL,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating shared_links table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS price_cache (
			brand        TEXT NOT NULL,
			category     TEXT NOT NULL,
			zip          TEXT NOT NULL,
			price_cents  INTEGER NOT NULL,
			unit_price   TEXT NOT NULL DEFAULT '',
			unit_label   TEXT NOT NULL DEFAULT '',
			size         TEXT NOT NULL DEFAULT '',
			product_name TEXT NOT NULL DEFAULT '',
			store_name   TEXT NOT NULL DEFAULT 'Walmart',
			fetched_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (brand, category, zip)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating price_cache table: %w", err)
	}

	// Brand portal tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			company_name  TEXT NOT NULL DEFAULT '',
			verified      INTEGER NOT NULL DEFAULT 0,
			verify_token  TEXT DEFAULT NULL,
			badge         TEXT NOT NULL DEFAULT '',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			logo_data     TEXT DEFAULT NULL,
			logo_mime     TEXT DEFAULT NULL,
			digest_opt_out INTEGER NOT NULL DEFAULT 0,
			last_digest_at DATETIME DEFAULT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_users table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_sessions (
			token      TEXT PRIMARY KEY,
			brand_user_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_sessions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_balance (
			brand_user_id   INTEGER PRIMARY KEY,
			balance_cents   INTEGER NOT NULL DEFAULT 0,
			last_deposit_at DATETIME DEFAULT NULL,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_balance table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_transactions (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			brand_user_id INTEGER NOT NULL,
			type          TEXT NOT NULL,
			amount_cents  INTEGER NOT NULL,
			description   TEXT NOT NULL DEFAULT '',
			stripe_id     TEXT DEFAULT NULL,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_transactions table: %w", err)
	}
	// Mirrors schema.go idx_brand_transactions_stripe_id. Partial, because the
	// dev/test deposit fallback writes stripe_id = '' for every row.
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_brand_transactions_stripe_id
		ON brand_transactions(stripe_id) WHERE stripe_id IS NOT NULL AND stripe_id <> ''`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bug_reports (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     TEXT,
			description TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating bug_reports table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipt_line_items (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			receipt_id      INTEGER NOT NULL,
			raw_text        TEXT NOT NULL,
			price_cents     INTEGER NOT NULL DEFAULT 0,
			quantity        INTEGER NOT NULL DEFAULT 1,
			matched_item_id INTEGER DEFAULT NULL,
			matched_name    TEXT NOT NULL DEFAULT '',
			confidence      TEXT NOT NULL DEFAULT 'low',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (receipt_id) REFERENCES receipts(id) ON DELETE CASCADE,
			FOREIGN KEY (matched_item_id) REFERENCES list_items(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating receipt_line_items table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_claims (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			receipt_id        INTEGER NOT NULL,
			swap_event_id     INTEGER NOT NULL,
			deal_id           INTEGER NOT NULL,
			user_id           TEXT NOT NULL,
			status            TEXT NOT NULL DEFAULT 'pending',
			verified_at       DATETIME DEFAULT NULL,
			payout_held_until DATETIME DEFAULT NULL,
			created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (receipt_id) REFERENCES receipts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_claims table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS retailer_abbreviations (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			abbreviation  TEXT NOT NULL,
			full_name     TEXT NOT NULL,
			retailer      TEXT NOT NULL DEFAULT '',
			confirmations INTEGER NOT NULL DEFAULT 1,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(abbreviation, retailer)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating retailer_abbreviations table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS savings_milestones (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL REFERENCES users(id),
			cents      INTEGER NOT NULL,
			reached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			acked_at   DATETIME DEFAULT NULL,
			UNIQUE(user_id, cents)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating savings_milestones table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_signals (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL REFERENCES users(id),
			signal     TEXT NOT NULL,
			category   TEXT NOT NULL DEFAULT '',
			value      TEXT NOT NULL DEFAULT '',
			context    TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_signals table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_learnings (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			scope      TEXT NOT NULL,
			scope_id   TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0.5,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(scope, scope_id, key)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating penny_learnings table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_dietary_profiles (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     TEXT NOT NULL UNIQUE REFERENCES users(id),
			allergens   TEXT NOT NULL DEFAULT '[]',
			diets       TEXT NOT NULL DEFAULT '[]',
			avoid_brands TEXT NOT NULL DEFAULT '[]',
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_dietary_profiles table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_context_cache (
			user_id      TEXT PRIMARY KEY REFERENCES users(id),
			context_json TEXT NOT NULL DEFAULT '{}',
			computed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating penny_context_cache table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS deal_performance_daily (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			deal_id         INTEGER NOT NULL,
			date            TEXT NOT NULL,
			impressions     INTEGER NOT NULL DEFAULT 0,
			accepts         INTEGER NOT NULL DEFAULT 0,
			declines        INTEGER NOT NULL DEFAULT 0,
			revenue_cents   INTEGER NOT NULL DEFAULT 0,
			verified_count  INTEGER NOT NULL DEFAULT 0,
			UNIQUE(deal_id, date)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating deal_performance_daily table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_swap_insights (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			category            TEXT NOT NULL UNIQUE,
			avg_acceptance_rate REAL NOT NULL DEFAULT 0.0,
			avg_discount_pct    REAL NOT NULL DEFAULT 0.0,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			last_computed       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			top_decline_reason  TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating category_swap_insights table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS business_metrics_daily (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			date                TEXT NOT NULL UNIQUE,
			dau                 INTEGER NOT NULL DEFAULT 0,
			wau                 INTEGER NOT NULL DEFAULT 0,
			mau                 INTEGER NOT NULL DEFAULT 0,
			new_users           INTEGER NOT NULL DEFAULT 0,
			swaps_offered       INTEGER NOT NULL DEFAULT 0,
			swaps_accepted      INTEGER NOT NULL DEFAULT 0,
			swaps_declined      INTEGER NOT NULL DEFAULT 0,
			acceptance_rate     REAL NOT NULL DEFAULT 0.0,
			gross_revenue_cents INTEGER NOT NULL DEFAULT 0,
			active_deals        INTEGER NOT NULL DEFAULT 0,
			active_brands       INTEGER NOT NULL DEFAULT 0,
			computed_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating business_metrics_daily table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS churn_signals (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      TEXT NOT NULL REFERENCES users(id),
			signal_type  TEXT NOT NULL,
			signal_value REAL NOT NULL,
			risk_score   REAL NOT NULL DEFAULT 0.0,
			computed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, signal_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating churn_signals table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_cohorts (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id             TEXT NOT NULL UNIQUE REFERENCES users(id),
			cohort_week         TEXT NOT NULL,
			signup_source       TEXT NOT NULL DEFAULT '',
			first_swap_at       DATETIME DEFAULT NULL,
			first_receipt_at    DATETIME DEFAULT NULL,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			total_savings_cents INTEGER NOT NULL DEFAULT 0,
			last_active         DATETIME DEFAULT NULL,
			is_churned          INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_cohorts table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS marketplace_health (
			id                    INTEGER PRIMARY KEY AUTOINCREMENT,
			date                  TEXT NOT NULL UNIQUE,
			supply_score          REAL NOT NULL DEFAULT 0.0,
			demand_score          REAL NOT NULL DEFAULT 0.0,
			balance_score         REAL NOT NULL DEFAULT 0.0,
			revenue_concentration REAL NOT NULL DEFAULT 0.0,
			health_index          REAL NOT NULL DEFAULT 0.0,
			computed_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating marketplace_health table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT NOT NULL UNIQUE,
			parent_category TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating categories table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_pricing_tiers (
			parent_category    TEXT PRIMARY KEY,
			min_price_cents    INTEGER NOT NULL DEFAULT 100,
			suggested_cents    INTEGER NOT NULL DEFAULT 150,
			tier_label         TEXT NOT NULL DEFAULT 'standard'
		)
	`)
	if err != nil {
		return fmt.Errorf("creating category_pricing_tiers table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_swap_predictions (
			user_id             TEXT NOT NULL,
			category_id         INTEGER NOT NULL DEFAULT 0,
			brand               TEXT NOT NULL DEFAULT '',
			acceptance_rate     REAL NOT NULL DEFAULT 0.0,
			total_offers        INTEGER NOT NULL DEFAULT 0,
			total_accepts       INTEGER NOT NULL DEFAULT 0,
			avg_time_to_action_ms INTEGER NOT NULL DEFAULT 0,
			confidence          REAL NOT NULL DEFAULT 0.0,
			computed_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, category_id, brand)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_swap_predictions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stores (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			chain        TEXT NOT NULL DEFAULT '',
			address      TEXT NOT NULL DEFAULT '',
			city         TEXT NOT NULL DEFAULT '',
			state        TEXT NOT NULL DEFAULT '',
			zip          TEXT NOT NULL DEFAULT '',
			latitude     REAL,
			longitude    REAL,
			place_id     TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name, address)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating stores table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS store_prices (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			store_id     INTEGER NOT NULL REFERENCES stores(id),
			product_name TEXT NOT NULL,
			brand        TEXT NOT NULL DEFAULT '',
			price_cents  INTEGER NOT NULL,
			category_id  INTEGER NOT NULL DEFAULT 0,
			source       TEXT NOT NULL DEFAULT 'receipt',
			observed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(store_id, product_name, brand, observed_at)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating store_prices table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_propagations (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
			source_user_id  TEXT NOT NULL,
			adopter_user_id TEXT NOT NULL,
			household_id    INTEGER NOT NULL,
			item_name       TEXT NOT NULL,
			brand           TEXT NOT NULL,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(deal_id, adopter_user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_propagations table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_subscriptions (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			brand_user_id  INTEGER NOT NULL,
			tier           TEXT NOT NULL DEFAULT 'free',
			started_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at     DATETIME,
			UNIQUE(brand_user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_subscriptions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_alerts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type  TEXT NOT NULL,
			brand       TEXT NOT NULL DEFAULT '',
			category_id INTEGER NOT NULL DEFAULT 0,
			severity    TEXT NOT NULL DEFAULT 'low',
			message     TEXT NOT NULL,
			data_json   TEXT NOT NULL DEFAULT '{}',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating category_alerts table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feedback_requests (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          TEXT NOT NULL REFERENCES users(id),
			body             TEXT NOT NULL,
			type             TEXT NOT NULL DEFAULT 'feature',
			urgency          TEXT NOT NULL DEFAULT 'normal',
			status           TEXT NOT NULL DEFAULT 'new',
			admin_notes      TEXT NOT NULL DEFAULT '',
			github_issue_url TEXT NOT NULL DEFAULT '',
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME
		)
	`)
	if err != nil {
		return fmt.Errorf("creating feedback_requests table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS account_merges (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			surviving_user  TEXT NOT NULL REFERENCES users(id),
			merged_user     TEXT,
			provider_linked TEXT NOT NULL,
			provider_id     TEXT NOT NULL,
			data_actions    TEXT NOT NULL DEFAULT '{}',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating account_merges table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_feedback_requests (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			brand_user_id    INTEGER NOT NULL REFERENCES brand_users(id),
			body             TEXT NOT NULL,
			type             TEXT NOT NULL DEFAULT 'feature',
			urgency          TEXT NOT NULL DEFAULT 'normal',
			status           TEXT NOT NULL DEFAULT 'new',
			admin_notes      TEXT NOT NULL DEFAULT '',
			github_issue_url TEXT NOT NULL DEFAULT '',
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_feedback_requests table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS department_order_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    TEXT NOT NULL,
			store_name TEXT NOT NULL DEFAULT '',
			department TEXT NOT NULL,
			checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			trip_date  TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating department_order_events table: %w", err)
	}

	// A/B testing tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ab_tests (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			variants    TEXT NOT NULL DEFAULT '["control","variant"]',
			key_metric  TEXT NOT NULL DEFAULT '',
			min_sample_per_variant INTEGER NOT NULL DEFAULT 100,
			measurement_days INTEGER NOT NULL DEFAULT 7,
			traffic_pct INTEGER NOT NULL DEFAULT 50,
			status      TEXT NOT NULL DEFAULT 'active',
			winner      TEXT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		)
	`)
	if err != nil {
		return fmt.Errorf("creating ab_tests table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ab_assignments (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			test_id INTEGER NOT NULL,
			user_id TEXT NOT NULL,
			variant TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(test_id, user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating ab_assignments table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ab_events (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			test_name TEXT NOT NULL,
			user_id TEXT NOT NULL,
			variant TEXT NOT NULL,
			event   TEXT NOT NULL,
			value   REAL NOT NULL DEFAULT 1.0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating ab_events table: %w", err)
	}

	// Seed brands
	seedTestBrands(db)

	// Seed deals
	seedTestDeals(db)

	// Seed categories
	seedTestCategories(db)

	// Seed pricing tiers
	seedTestPricingTiers(db)

	return nil
}

// seedTestBrands inserts a minimal set of brands for tests.
func seedTestBrands(db *sql.DB) {
	brands := []struct{ name, domain string }{
		{"Skippy", "skippy.com"}, {"Jif", "jif.com"},
		{"LaCroix", "lacroixwater.com"}, {"Waterloo", "drinkwaterloo.com"},
		{"Tide", "tide.com"}, {"All", "all-laundry.com"},
		{"Ragu", "ragu.com"}, {"Rao's", "raos.com"},
		{"Cheerios", "cheerios.com"}, {"Folgers", "folgers.com"},
		{"Oreo", "oreo.com"},
	}
	for _, b := range brands {
		db.Exec("INSERT OR IGNORE INTO brands (name, domain) VALUES (?, ?)", b.name, b.domain)
	}
}

// seedTestDeals inserts seed deals matching production seedDeals.
// Mirrors the rich price metadata in schema.go so swap tests have expected values.
func seedTestDeals(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM brand_deals").Scan(&count)
	if count > 0 {
		return
	}
	deals := []struct {
		competitor, offerBrand, offerProduct string
		discountCents, budgetCents           int
		offerPriceCents, competitorPriceCents int
		offerSize, competitorSize             string
		offerUnitPrice, competitorUnitPrice   string
		whyBullets, allergenNotes             string
	}{
		{
			"skippy", "Jif", "Creamy Peanut Butter", 125, 400,
			348, 473, "16 oz", "16.3 oz", "$0.22/oz", "$0.29/oz",
			`["No palm oil","4.6★ avg rating","24% less per ounce"]`, "",
		},
		{
			"lacroix", "Waterloo", "Sparkling Water Variety Pack", 50, 400,
			498, 548, "12-pack", "12-pack", "$0.42/can", "$0.46/can",
			`["Bolder natural flavors","4.5★ avg rating","No artificial sweeteners"]`, "",
		},
		{
			"tide", "All Free Clear", "Liquid Laundry Detergent", 100, 400,
			897, 1197, "88 oz", "92 oz", "$0.10/oz", "$0.13/oz",
			`["Hypoallergenic","Dermatologist recommended","23% less per load"]`,
			"Free of dyes and perfumes.",
		},
		{
			"ragu", "Rao's", "Marinara Sauce", 50, 400,
			498, 548, "24 oz", "24 oz", "$0.21/oz", "$0.23/oz",
			`["All-natural ingredients","No added sugar","Slow-simmered recipe"]`, "",
		},
		{
			"archer", "Chomps", "Beef Sticks", 75, 400,
			199, 274, "1.15 oz", "1 oz", "$1.73/oz", "$2.74/oz",
			`["100% grass-fed beef","Whole30 approved","37% less per ounce"]`,
			"Contains: beef. Free of top 8 allergens.",
		},
	}
	for _, d := range deals {
		db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
			discount_cents, total_budget_cents, status,
			offer_price_cents, competitor_price_cents, offer_size, competitor_size,
			offer_unit_price, competitor_unit_price, why_bullets, allergen_notes)
			VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.competitor, d.offerBrand, d.offerProduct, d.discountCents, d.budgetCents,
			d.offerPriceCents, d.competitorPriceCents, d.offerSize, d.competitorSize,
			d.offerUnitPrice, d.competitorUnitPrice, d.whyBullets, d.allergenNotes)
	}
	// New type deals
	db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, deal_type, quantity_offered)
		VALUES ('cheerios', 'Magic Spoon', 'Cereal', 150, 400, 'active', 'bogo', 2)`)
	db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, deal_type, trial_product_name, trial_size)
		VALUES ('folgers', 'Counter Culture', 'Coffee Sample', 100, 400, 'active', 'trial', 'Counter Culture Coffee Sample', '2 oz')`)
	db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, deal_type, campaign_tag, ends_at)
		VALUES ('oreo', 'Partake', 'Holiday Cookie Box', 200, 400, 'active', 'seasonal', 'holiday_2026', datetime('now', '+30 days'))`)
}

// seedTestCategories inserts a minimal set of categories for tests.
func seedTestCategories(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count)
	if count > 0 {
		return
	}
	cats := []struct{ name, parent string }{
		{"Milk", "Dairy"}, {"Cheese", "Dairy"}, {"Yogurt", "Dairy"}, {"Butter", "Dairy"},
		{"Cream", "Dairy"}, {"Eggs", "Dairy"}, {"Sour Cream", "Dairy"}, {"Cottage Cheese", "Dairy"},
		{"Fresh Fruit", "Produce"}, {"Fresh Vegetables", "Produce"}, {"Salad Greens", "Produce"},
		{"Herbs", "Produce"}, {"Organic Produce", "Produce"},
		{"Pasta", "Pantry"}, {"Rice", "Pantry"}, {"Canned Vegetables", "Pantry"},
		{"Canned Soup", "Pantry"}, {"Pasta Sauce", "Pantry"}, {"Cooking Oil", "Pantry"},
		{"Flour", "Pantry"}, {"Sugar", "Pantry"}, {"Cereal", "Pantry"}, {"Oatmeal", "Pantry"},
		{"Beans", "Pantry"},
		{"Water", "Beverages"}, {"Soda", "Beverages"}, {"Juice", "Beverages"},
		{"Coffee", "Beverages"}, {"Tea", "Beverages"}, {"Sports Drinks", "Beverages"},
		{"Energy Drinks", "Beverages"}, {"Sparkling Water", "Beverages"}, {"Kombucha", "Beverages"},
		{"Frozen Meals", "Frozen"}, {"Frozen Vegetables", "Frozen"}, {"Frozen Fruit", "Frozen"},
		{"Ice Cream", "Frozen"}, {"Frozen Pizza", "Frozen"}, {"Frozen Breakfast", "Frozen"},
		{"Chicken", "Meat"}, {"Beef", "Meat"}, {"Pork", "Meat"}, {"Deli Meat", "Meat"},
		{"Sausage", "Meat"}, {"Seafood", "Meat"}, {"Plant-Based Meat", "Meat"},
		{"Chips", "Snacks"}, {"Crackers", "Snacks"}, {"Cookies", "Snacks"},
		{"Nuts", "Snacks"}, {"Granola Bars", "Snacks"}, {"Popcorn", "Snacks"},
		{"Dried Fruit", "Snacks"}, {"Candy", "Snacks"},
		{"Ketchup", "Condiments"}, {"Mustard", "Condiments"}, {"Mayonnaise", "Condiments"},
		{"Hot Sauce", "Condiments"}, {"Salad Dressing", "Condiments"}, {"Soy Sauce", "Condiments"},
		{"Salsa", "Condiments"},
		{"Baking Mix", "Baking"}, {"Chocolate Chips", "Baking"}, {"Vanilla Extract", "Baking"},
		{"Baking Powder", "Baking"}, {"Frosting", "Baking"},
		{"Laundry Detergent", "Cleaning"}, {"Dish Soap", "Cleaning"},
		{"All-Purpose Cleaner", "Cleaning"}, {"Disinfectant", "Cleaning"},
		{"Trash Bags", "Cleaning"}, {"Paper Towels", "Cleaning"},
		{"Shampoo", "Personal Care"}, {"Body Wash", "Personal Care"},
		{"Toothpaste", "Personal Care"}, {"Deodorant", "Personal Care"},
		{"Lotion", "Personal Care"}, {"Soap", "Personal Care"},
		{"Diapers", "Baby"}, {"Baby Food", "Baby"}, {"Baby Formula", "Baby"}, {"Baby Wipes", "Baby"},
		{"Dog Food", "Pet"}, {"Cat Food", "Pet"}, {"Pet Treats", "Pet"}, {"Cat Litter", "Pet"},
		{"Toilet Paper", "Paper Goods"}, {"Napkins", "Paper Goods"}, {"Tissues", "Paper Goods"},
		{"Aluminum Foil", "Paper Goods"}, {"Plastic Wrap", "Paper Goods"}, {"Storage Bags", "Paper Goods"},
	}
	for _, c := range cats {
		db.Exec("INSERT INTO categories (name, parent_category) VALUES (?, ?)", c.name, c.parent)
	}
}

// seedTestPricingTiers inserts pricing tiers for tests.
func seedTestPricingTiers(db *sql.DB) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM category_pricing_tiers").Scan(&count)
	if count > 0 {
		return
	}
	tiers := []struct {
		parent   string
		minCents int
		sugCents int
		label    string
	}{
		{"Dairy", 250, 300, "premium"}, {"Beverages", 250, 300, "premium"}, {"Snacks", 200, 250, "premium"},
		{"Produce", 150, 200, "standard"}, {"Pantry", 150, 200, "standard"}, {"Meat", 150, 200, "standard"},
		{"Frozen", 150, 200, "standard"}, {"Cereal", 150, 200, "standard"},
		{"Condiments", 100, 150, "economy"}, {"Baking", 100, 150, "economy"}, {"Cleaning", 100, 150, "economy"},
		{"Personal Care", 100, 150, "economy"}, {"Baby", 100, 150, "economy"},
		{"Pet", 100, 150, "economy"}, {"Paper Goods", 100, 150, "economy"},
	}
	for _, t := range tiers {
		db.Exec("INSERT INTO category_pricing_tiers (parent_category, min_price_cents, suggested_cents, tier_label) VALUES (?, ?, ?, ?)",
			t.parent, t.minCents, t.sugCents, t.label)
	}
}

// openTestDB opens an in-memory SQLite database with full test schema.
// Returns the raw *sql.DB and a *PGDB wrapper.
func openTestDB(t *testing.T) (*sql.DB, *PGDB) {
	t.Helper()
	if adminURL := postgresTestURL(); adminURL != "" {
		return openPostgresTestDB(t, adminURL)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	db.SetMaxOpenConns(1) // SQLite needs single connection for :memory:
	if err := initTestDB(db); err != nil {
		db.Close()
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	pgdb := WrapPG(db)
	pgdb.skipConversion = true
	return db, pgdb
}

// =============================================================================
// CONCURRENCY TEST HELPER
// =============================================================================

// hammer runs fn concurrently n times and waits for all of them to finish,
// releasing them as close to simultaneously as a start barrier allows. Use it
// to exercise TOCTOU windows in money-handling paths.
//
// IMPORTANT: the test database is SQLite :memory: with SetMaxOpenConns(1), so
// individual statements serialise. A check-then-act interleaving is still
// possible (the check releases the connection before the transaction begins),
// but whether it happens depends on goroutine scheduling. Every hammer test
// MUST therefore be paired with a deterministic negative test that pre-seeds
// the conflicting row and asserts the guard fires, so correctness never
// depends on scheduler luck.
func hammer(t *testing.T, n int, fn func(i int)) {
	t.Helper()
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			fn(i)
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// newTestServer creates a Server with a mock Twilio client and an in-memory
// SQLite database. Each test gets a fresh database -- no leaking state between
// tests.
func newTestServer(t *testing.T, mock *MockTwilioClient) *Server {
	t.Helper() // marks this as a helper so errors show the caller's line number

	_, pgdb := openTestDB(t)

	server := &Server{
		config:   Config{Port: "8080", AnthropicAPIKey: "test-key", TwilioPhoneNumber: "+10000000000", AdminPhone: "3104861833", FeedbackPhones: []string{"5551234567"}, BcryptCost: 4},
		db:       pgdb,
		sessions: NewSessionStore(pgdb),
		twilio:   mock,
		parseItems: func(apiKey, text string, images []string) ([]ParsedItem, error) {
			// Default mock parser: returns nothing. Tests can override.
			return nil, nil
		},
		smsBuffer: newMessageBuffer(50 * time.Millisecond),
	}
	// Default parseCommand wraps parseItems so existing tests work
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		parsed, err := server.parseItems(apiKey, text, nil)
		if err != nil {
			return nil, err
		}
		return &ParsedCommand{Action: "add", Items: parsed}, nil
	}
	return server
}

// doRequest sends a fake HTTP request to a handler and returns the response.
func doRequest(
	handler http.HandlerFunc,
	method, path string,
	body interface{},
	cookies []*http.Cookie,
) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// parseJSON reads the response body into a map for easy field checking.
func parseJSON(rr *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	return result
}

// getSessionCookie extracts the session cookie from a response.
// Returns nil if no session cookie was set.
func getSessionCookie(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

// createListItem is a helper to insert a list item and return its ID.
func createListItem(t *testing.T, db *PGDB, userID string, name, department string) int64 {
	t.Helper()
	result, err := db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, name, department)
	if err != nil {
		t.Fatalf("createListItem: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

// newTestSessionStore creates a SessionStore backed by an in-memory SQLite DB.
func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	rawDB, pgdb := openTestDB(t)
	// Create users so FK constraints are satisfied
	rawDB.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567'), ('user-2', '5552222222')")
	return NewSessionStore(pgdb)
}

// loginUser creates a user in the DB and returns a session cookie for that user.
func loginUser(t *testing.T, server *Server, phone string) (*http.Cookie, string) {
	t.Helper()
	user, err := upsertUser(server.db, phone)
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}
	token, err := server.sessions.CreateSession(user.ID, phone, "test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token}, user.ID
}

// doSMSRequest sends a Twilio-style form POST to the SMS webhook handler.
func doSMSRequest(handler http.HandlerFunc, from, body string, numMedia int) *httptest.ResponseRecorder {
	form := url.Values{}
	form.Set("From", from)
	form.Set("Body", body)
	form.Set("NumMedia", fmt.Sprintf("%d", numMedia))

	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// doEmailRequest sends a multipart form request mimicking SendGrid Inbound Parse.
func doEmailRequest(handler http.HandlerFunc, from, text string) *httptest.ResponseRecorder {
	body := fmt.Sprintf("from=%s&text=%s", url.QueryEscape(from), url.QueryEscape(text))
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// buildMultipartEmail creates a multipart form body mimicking SendGrid's format.
func buildMultipartEmail(t *testing.T, buf *bytes.Buffer, from, text string, images map[string][]byte) *multipart.Writer {
	t.Helper()
	w := multipart.NewWriter(buf)
	w.WriteField("from", from)
	w.WriteField("text", text)
	for name, data := range images {
		ct := "image/jpeg"
		if strings.HasSuffix(name, ".png") {
			ct = "image/png"
		}
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="attachment"; filename="%s"`, name))
		h.Set("Content-Type", ct)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		part.Write(data)
	}
	w.Close()
	return w
}

// testTransport redirects all HTTP requests to a test server URL
type testTransport struct {
	targetURL string
	real      http.RoundTripper
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	u, _ := url.Parse(t.targetURL)
	newReq.URL.Scheme = u.Scheme
	newReq.URL.Host = u.Host
	return t.real.RoundTrip(newReq)
}

// makeJPEG returns minimal valid JPEG bytes (FF D8 header + padding).
func makeJPEG(size int) []byte {
	data := make([]byte, size)
	data[0] = 0xFF
	data[1] = 0xD8
	return data
}

// makePNG returns minimal valid PNG bytes (89 50 4E 47 header + padding).
func makePNG(size int) []byte {
	data := make([]byte, size)
	data[0] = 0x89
	data[1] = 0x50
	data[2] = 0x4E
	data[3] = 0x47
	return data
}

// brandRegister is a helper that registers a brand user and returns the session cookie and brand_user_id.
func brandRegister(t *testing.T, server *Server, email, password, companyName string) (*http.Cookie, int) {
	t.Helper()
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        email,
		"password":     password,
		"company_name": companyName,
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("brandRegister: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "brand_session" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("brandRegister: no brand_session cookie returned")
	}
	body := parseJSON(rr)
	var brandUserID int
	switch v := body["brand_user_id"].(type) {
	case float64:
		brandUserID = int(v)
	case string:
		fmt.Sscanf(v, "%d", &brandUserID)
	}
	return cookie, brandUserID
}

// createTestDeal is a helper that creates a brand deal and returns the deal ID.
// Deals are created as pending_review and auto-approved so existing tests work.
func createTestDeal(t *testing.T, server *Server, cookie *http.Cookie) int {
	t.Helper()
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product":     "Competitor Cola 12pk",
		"offer_brand":            "TestBrand",
		"offer_product":          "TestBrand Cola 12pk",
		"discount_cents":         150,
		"daily_budget_cents":     10000,
		"offer_price_cents":      499,
		"competitor_price_cents": 599,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("createTestDeal: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	var dealID int
	switch v := body["id"].(type) {
	case float64:
		dealID = int(v)
	case string:
		fmt.Sscanf(v, "%d", &dealID)
	}
	// Auto-approve the deal so tests that expect active deals work
	server.db.Exec("UPDATE brand_deals SET status = 'active', review_status = 'approved' WHERE id = ?", dealID)
	return dealID
}

// loginAdmin logs in the admin user (phone matches config.AdminPhone = "3104861833").
func loginAdmin(t *testing.T, server *Server) *http.Cookie {
	t.Helper()
	cookie, _ := loginUser(t, server, server.config.AdminPhone)
	return cookie
}

// doRequestMux sends a request through a custom http.ServeMux (for sub-route testing).
func doRequestMux(mux *http.ServeMux, method, path string, body interface{}, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// setupEarningsTestData creates a user with brand_deals, swap_events, and swap_rebates
// for earnings tests. Returns (cookie, userID, dealID, rebateID).
func setupEarningsTestData(t *testing.T, server *Server) (*http.Cookie, string, int, int) {
	t.Helper()
	cookie, userID := loginUser(t, server, "5559991234")

	// Create a brand deal
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES ('Heinz Ketchup', 'Hunts', 'Tomato Ketchup', 150, 5000, 'active')`)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'Heinz Ketchup'").Scan(&dealID)

	// Create a list item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'ketchup', 'Condiments')", userID)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ?", userID).Scan(&itemID)

	// Create a swap event
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'accept')", userID, dealID, itemID)
	var swapEventID int
	server.db.QueryRow("SELECT id FROM swap_events WHERE user_id = ?", userID).Scan(&swapEventID)

	// Create a swap rebate
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, 150, 'pending')", userID, swapEventID, dealID)
	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE user_id = ?", userID).Scan(&rebateID)

	return cookie, userID, dealID, rebateID
}

// captureLog redirects the standard logger to a buffer for the duration of fn
// and returns everything written.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	fn()
	return buf.String()
}
