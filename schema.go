package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// addColumnIfNotExists safely adds a column to a table, ignoring the error
// if the column already exists (Postgres raises an error unlike SQLite which
// silently ignores duplicate ALTER TABLE ADD COLUMN).
func addColumnIfNotExists(db dbExecer, table, column, definition string) {
	_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		log.Printf("[schema] warning adding %s.%s: %v", table, column, err)
	}
}

func initDB(db *sql.DB) error {
	// =========================================================================
	// USERS TABLE — owned by pennyai
	// =========================================================================
	// Pennyai owns its own users table. Identity federation with Rally/timpager
	// happens via a server-to-server API call at verify time (PR 3), not via a
	// shared database.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id           TEXT PRIMARY KEY,
			phone        TEXT UNIQUE,
			email        TEXT NOT NULL DEFAULT '',
			display_name TEXT NOT NULL DEFAULT '',
			name         TEXT NOT NULL DEFAULT '',
			created_at   TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating users table: %w", err)
	}

	// Penny-specific columns added via ALTER so existing databases (prod users
	// table predates this migration) pick them up without recreation.
	for _, c := range []struct{ col, def string }{
		{"household_id", "INTEGER"},
		{"referral_code", "TEXT"},
		{"referral_points", "INTEGER DEFAULT 0"},
		{"last_active", "TIMESTAMPTZ"},
		{"login_count", "INTEGER NOT NULL DEFAULT 0"},
		{"address", "TEXT NOT NULL DEFAULT ''"},
		{"penny_brand_mode", "TEXT NOT NULL DEFAULT 'ask'"},
		{"deal_accept_count", "INTEGER NOT NULL DEFAULT 0"},
		{"deal_shown_count", "INTEGER NOT NULL DEFAULT 0"},
		{"penny_slider", "INTEGER NOT NULL DEFAULT 15"},
		{"penny_autotune", "INTEGER NOT NULL DEFAULT 1"},
		{"penny_deals", "INTEGER NOT NULL DEFAULT 0"},
		{"last_cleared_items", "JSONB NOT NULL DEFAULT '[]'::jsonb"},
		{"swap_credits_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"sms_consent_at", "TIMESTAMPTZ"},
		{"sms_consent_ip", "TEXT NOT NULL DEFAULT ''"},
		{"sms_consent_user_agent", "TEXT NOT NULL DEFAULT ''"},
		{"onboarding_flags", "JSONB NOT NULL DEFAULT '{}'::jsonb"},
		{"profile_photo", "TEXT NOT NULL DEFAULT ''"},
		{"name_source", "TEXT NOT NULL DEFAULT 'auto'"},
		{"referral_source", "TEXT NOT NULL DEFAULT ''"},
		{"nps_dismissed_at", "TEXT NOT NULL DEFAULT ''"},
		{"store_order_enabled", "INTEGER NOT NULL DEFAULT 1"},
		{"daily_digest_enabled", "INTEGER NOT NULL DEFAULT 0"},
		{"daily_digest_opted_in_at", "TIMESTAMPTZ"},
		{"daily_digest_last_sent", "TIMESTAMPTZ"},
		// Rally sign-in (Clerk satellite): the `sub` claim of a verified Clerk
		// session JWT. Nullable with no default — every legacy row keeps a NULL
		// and nothing about the existing four auth methods changes.
		{"clerk_user_id", "TEXT"},
	} {
		addColumnIfNotExists(db, "users", c.col, c.def)
	}

	// One Clerk identity maps to at most one local user. A PARTIAL unique index
	// (not a column UNIQUE constraint) so unlimited NULLs coexist — a column
	// constraint would be a table-shape change and could fail on a database
	// that already has rows. Works on Postgres and the SQLite test tier alike.
	// This index, not application code, is the real backstop against two rows
	// claiming one Clerk identity under concurrent first logins.
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_clerk_user_id ON users(clerk_user_id) WHERE clerk_user_id IS NOT NULL")

	// =========================================================================
	// SESSIONS TABLE — owned by pennyai
	// =========================================================================
	// Session cookies live on .groflo.io only. No cross-domain handoff.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			phone      TEXT NOT NULL DEFAULT '',
			user_id    TEXT,
			role       TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used  TIMESTAMPTZ,
			expires_at TIMESTAMPTZ NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating sessions table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)")

	// =========================================================================
	// LIST ITEMS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS list_items (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			name       TEXT NOT NULL,
			department TEXT NOT NULL DEFAULT 'Other',
			checked    INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating list_items table: %w", err)
	}

	for _, c := range []struct{ col, def string }{
		{"brand", "TEXT NOT NULL DEFAULT ''"},
		{"quantity", "INTEGER NOT NULL DEFAULT 1"},
		{"size", "TEXT NOT NULL DEFAULT ''"},
		{"source", "TEXT NOT NULL DEFAULT 'app'"},
		{"urgent", "INTEGER NOT NULL DEFAULT 0"},
		{"claimed_by", "TEXT DEFAULT NULL"},
		{"claimed_at", "TIMESTAMPTZ DEFAULT NULL"},
		{"notes", "TEXT NOT NULL DEFAULT ''"},
		{"swap_savings_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"emoji", "TEXT NOT NULL DEFAULT ''"},
		{"needs_enrichment", "INTEGER NOT NULL DEFAULT 0"},
	} {
		addColumnIfNotExists(db, "list_items", c.col, c.def)
	}

	// =========================================================================
	// SAVED LISTS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_lists (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			name       TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating saved_lists table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_list_items (
			id            SERIAL PRIMARY KEY,
			saved_list_id INTEGER NOT NULL,
			name          TEXT NOT NULL,
			brand         TEXT NOT NULL DEFAULT '',
			department    TEXT NOT NULL DEFAULT 'Other',
			quantity      INTEGER NOT NULL DEFAULT 1,
			size          TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (saved_list_id) REFERENCES saved_lists(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating saved_list_items table: %w", err)
	}
	addColumnIfNotExists(db, "saved_list_items", "notes", "TEXT NOT NULL DEFAULT ''")
	addColumnIfNotExists(db, "saved_lists", "store_name", "TEXT NOT NULL DEFAULT ''")
	addColumnIfNotExists(db, "saved_lists", "is_master", "INTEGER NOT NULL DEFAULT 0")
	addColumnIfNotExists(db, "saved_lists", "updated_at", "TIMESTAMPTZ DEFAULT NOW()")

	// =========================================================================
	// HOUSEHOLDS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS households (
			id         SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating households table: %w", err)
	}
	addColumnIfNotExists(db, "households", "name", "TEXT NOT NULL DEFAULT ''")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS household_members (
			id           SERIAL PRIMARY KEY,
			household_id INTEGER NOT NULL,
			user_id      TEXT NOT NULL UNIQUE,
			role         TEXT NOT NULL DEFAULT 'member',
			invited_by   TEXT,
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating household_members table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS invites (
			id           SERIAL PRIMARY KEY,
			household_id INTEGER NOT NULL,
			code         TEXT UNIQUE NOT NULL,
			created_by   TEXT NOT NULL,
			expires_at   TIMESTAMPTZ NOT NULL,
			used_by      TEXT,
			used_at      TIMESTAMPTZ,
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (created_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating invites table: %w", err)
	}
	addColumnIfNotExists(db, "invites", "notified", "INTEGER NOT NULL DEFAULT 0")

	// =========================================================================
	// ITEM MODIFICATIONS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS item_modifications (
			id         SERIAL PRIMARY KEY,
			item_id    INTEGER NOT NULL,
			user_id    TEXT,
			action     TEXT NOT NULL,
			detail     TEXT,
			source     TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (item_id) REFERENCES list_items(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating item_modifications table: %w", err)
	}

	// =========================================================================
	// REFERRALS & REDEMPTIONS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS referrals (
			id          SERIAL PRIMARY KEY,
			referrer_id TEXT NOT NULL,
			referred_id TEXT NOT NULL,
			created_at  TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (referrer_id) REFERENCES users(id),
			FOREIGN KEY (referred_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating referrals table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS redemptions (
			id           SERIAL PRIMARY KEY,
			user_id      TEXT NOT NULL,
			points       INTEGER NOT NULL,
			amount_cents INTEGER NOT NULL,
			status       TEXT DEFAULT 'pending',
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating redemptions table: %w", err)
	}

	// =========================================================================
	// NPS RESPONSES
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nps_responses (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			score      INTEGER NOT NULL,
			comment    TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating nps_responses table: %w", err)
	}

	// =========================================================================
	// MULTI-AUTH TABLES
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_methods (
			id            SERIAL PRIMARY KEY,
			user_id       TEXT NOT NULL,
			provider      TEXT NOT NULL,
			provider_id   TEXT NOT NULL,
			password_hash TEXT DEFAULT NULL,
			created_at    TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(provider, provider_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating auth_methods table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_resets (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			token      TEXT UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used       INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating password_resets table: %w", err)
	}

	// Backfill auth_methods from users with phone numbers
	db.Exec(`
		INSERT INTO auth_methods (user_id, provider, provider_id)
		SELECT id, 'phone', phone FROM users
		WHERE phone IS NOT NULL AND phone != ''
		ON CONFLICT DO NOTHING
	`)

	// =========================================================================
	// BRANDS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_owners (
			id         SERIAL PRIMARY KEY,
			name       TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_owners table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brands (
			id         SERIAL PRIMARY KEY,
			name       TEXT UNIQUE NOT NULL,
			domain     TEXT NOT NULL,
			owner_id   INTEGER,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (owner_id) REFERENCES brand_owners(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brands table: %w", err)
	}

	if err := seedBrands(db); err != nil {
		log.Printf("WARNING: failed to seed brands: %v", err)
	}

	// Brand logos (stored as BYTEa to survive ephemeral filesystems)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_logos (
			brand_id      INTEGER PRIMARY KEY,
			logo_data     BYTEA   NOT NULL,
			content_type  TEXT    NOT NULL DEFAULT 'image/png',
			fetched_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			source        TEXT    NOT NULL DEFAULT 'logodev',
			FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_logos table: %w", err)
	}

	// =========================================================================
	// SHOPPING TRIPS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shopping_trips (
			id           SERIAL PRIMARY KEY,
			user_id      TEXT NOT NULL,
			household_id INTEGER NOT NULL,
			trip_date    TEXT    NOT NULL,
			trip_time    TEXT    DEFAULT NULL,
			time_label   TEXT    DEFAULT NULL,
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (household_id) REFERENCES households(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating shopping_trips table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_shopping_trips_household_id ON shopping_trips(household_id)")

	// Migrate old department names (safe to run repeatedly)
	for _, stmt := range []string{
		"UPDATE list_items SET department = 'Bakery & Deli' WHERE department = 'Bakery'",
		"UPDATE list_items SET department = 'Pantry & Dry Goods' WHERE department = 'Pantry'",
		"UPDATE list_items SET department = 'Household & Cleaning' WHERE department = 'Household'",
		"UPDATE saved_list_items SET department = 'Bakery & Deli' WHERE department = 'Bakery'",
		"UPDATE saved_list_items SET department = 'Pantry & Dry Goods' WHERE department = 'Pantry'",
		"UPDATE saved_list_items SET department = 'Household & Cleaning' WHERE department = 'Household'",
	} {
		db.Exec(stmt)
	}

	// =========================================================================
	// BRAND PREFERENCE LEARNING
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_choices (
			id               SERIAL PRIMARY KEY,
			user_id          TEXT NOT NULL,
			product_category TEXT NOT NULL,
			brand            TEXT NOT NULL,
			source           TEXT NOT NULL DEFAULT 'disambiguation',
			created_at       TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_choices table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_preferences (
			id               SERIAL PRIMARY KEY,
			user_id          TEXT NOT NULL,
			product_category TEXT NOT NULL,
			brand            TEXT NOT NULL,
			streak           INTEGER NOT NULL DEFAULT 1,
			pick_count       INTEGER NOT NULL DEFAULT 1,
			last_picked      TIMESTAMPTZ DEFAULT NOW(),
			created_at       TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(user_id, product_category)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_preferences table: %w", err)
	}

	// =========================================================================
	// PENNY SWAPS TABLES
	// =========================================================================

	// brand_deals: deals seeded by admin, matching competitor products to offers
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_deals (
			id                 SERIAL PRIMARY KEY,
			competitor_product TEXT    NOT NULL,
			offer_brand        TEXT    NOT NULL,
			offer_product      TEXT    NOT NULL,
			discount_cents     INTEGER NOT NULL DEFAULT 0,
			daily_budget_cents INTEGER NOT NULL DEFAULT 0,
			budget_spent_cents INTEGER NOT NULL DEFAULT 0,
			status             TEXT    NOT NULL DEFAULT 'active',
			starts_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ends_at            TIMESTAMPTZ,
			created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_deals table: %w", err)
	}

	// swap_events: tracks every swap offer/accept/decline per user
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_events (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			deal_id    INTEGER NOT NULL REFERENCES brand_deals(id),
			item_id    INTEGER,
			action     TEXT    NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_events table: %w", err)
	}

	// swap_suppressions: tracks per-user deal dismissals, 3+ = permanent suppress
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

	// swap_rebates: immutable ledger tracking rebates owed per swap accept
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_rebates (
			id             SERIAL PRIMARY KEY,
			user_id        TEXT NOT NULL REFERENCES users(id),
			swap_event_id  INTEGER NOT NULL REFERENCES swap_events(id),
			deal_id        INTEGER NOT NULL REFERENCES brand_deals(id),
			amount_cents   INTEGER NOT NULL,
			status         TEXT NOT NULL DEFAULT 'pending',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_rebates table: %w", err)
	}

	// profile_photos: stores user avatar images as BYTEa (one per user)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profile_photos (
			user_id      TEXT PRIMARY KEY REFERENCES users(id),
			photo_data   BYTEA NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'image/jpeg',
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating profile_photos table: %w", err)
	}

	// receipts: tracks receipt photo submissions for dedup
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipts (
			id            SERIAL PRIMARY KEY,
			user_id       TEXT NOT NULL REFERENCES users(id),
			image_hash    TEXT    NOT NULL,
			items_found   INTEGER NOT NULL DEFAULT 0,
			items_matched INTEGER NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating receipts table: %w", err)
	}

	// shared_links: deep-link share tokens for read-only list sharing
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shared_links (
			id           SERIAL PRIMARY KEY,
			code         TEXT NOT NULL UNIQUE,
			user_id      TEXT NOT NULL,
			household_id INTEGER,
			invite_code  TEXT,
			expires_at   TIMESTAMPTZ NOT NULL,
			view_count   INTEGER DEFAULT 0,
			created_at   TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating shared_links table: %w", err)
	}

	// =========================================================================
	// INDEXES (placed after all table creation)
	// =========================================================================
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_household_members_household_id ON household_members(household_id)",
		"CREATE INDEX IF NOT EXISTS idx_household_members_user_id ON household_members(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_list_items_user_id ON list_items(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_lists_user_id ON saved_lists(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_list_items_saved_list_id ON saved_list_items(saved_list_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_referrals_unique ON referrals(referrer_id, referred_id)",
		"CREATE INDEX IF NOT EXISTS idx_item_modifications_item_id ON item_modifications(item_id)",
		"CREATE INDEX IF NOT EXISTS idx_item_modifications_user_id ON item_modifications(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_item_modifications_created_at ON item_modifications(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_brand_preferences_user_cat ON brand_preferences(user_id, product_category)",
		"CREATE INDEX IF NOT EXISTS idx_brand_choices_user_cat ON brand_choices(user_id, product_category)",
		"CREATE INDEX IF NOT EXISTS idx_auth_methods_user_id ON auth_methods(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_auth_methods_provider ON auth_methods(provider, provider_id)",
		"CREATE INDEX IF NOT EXISTS idx_password_resets_token ON password_resets(token)",
		"CREATE INDEX IF NOT EXISTS idx_list_items_user_checked ON list_items(user_id, checked)",
		"CREATE INDEX IF NOT EXISTS idx_swap_events_user_deal ON swap_events(user_id, deal_id)",
		"CREATE INDEX IF NOT EXISTS idx_swap_events_deal_id ON swap_events(deal_id)",
		"CREATE INDEX IF NOT EXISTS idx_swap_events_created_at ON swap_events(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_brand_deals_status ON brand_deals(status)",
		"CREATE INDEX IF NOT EXISTS idx_receipts_user_hash ON receipts(user_id, image_hash)",
		"CREATE INDEX IF NOT EXISTS idx_swap_rebates_user ON swap_rebates(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_swap_rebates_status ON swap_rebates(status)",
		"CREATE INDEX IF NOT EXISTS idx_shared_links_code ON shared_links(code)",
		"CREATE INDEX IF NOT EXISTS idx_shared_links_expires_at ON shared_links(expires_at)",
	} {
		db.Exec(stmt)
	}

	// brand_deals price comparison & why columns (swap card redesign M1)
	for _, c := range []struct{ col, def string }{
		{"offer_price_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"offer_size", "TEXT NOT NULL DEFAULT ''"},
		{"offer_unit_price", "TEXT NOT NULL DEFAULT ''"},
		{"competitor_price_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"competitor_size", "TEXT NOT NULL DEFAULT ''"},
		{"competitor_unit_price", "TEXT NOT NULL DEFAULT ''"},
		{"unit_label", "TEXT NOT NULL DEFAULT ''"},
		{"why_bullets", "JSONB NOT NULL DEFAULT '[]'::jsonb"},
		{"allergen_notes", "TEXT NOT NULL DEFAULT ''"},
	} {
		addColumnIfNotExists(db, "brand_deals", c.col, c.def)
	}

	// BlueCart price cache table
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
			fetched_at   TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (brand, category, zip)
		)
	`)
	if err != nil {
		log.Printf("[warn] failed to create price_cache table: %v", err)
	}

	// B15: rename daily_budget_cents -> total_budget_cents
	addColumnIfNotExists(db, "brand_deals", "total_budget_cents", "INTEGER NOT NULL DEFAULT 0")
	db.Exec("UPDATE brand_deals SET total_budget_cents = daily_budget_cents WHERE total_budget_cents = 0 AND daily_budget_cents != 0")

	// Sprint 3: swap type expansion columns
	for _, c := range []struct{ col, def string }{
		{"deal_type", "TEXT NOT NULL DEFAULT 'sku_swap'"},
		{"quantity_offered", "INTEGER NOT NULL DEFAULT 1"},
		{"trial_product_name", "TEXT NOT NULL DEFAULT ''"},
		{"trial_size", "TEXT NOT NULL DEFAULT ''"},
		{"campaign_tag", "TEXT NOT NULL DEFAULT ''"},
	} {
		addColumnIfNotExists(db, "brand_deals", c.col, c.def)
	}

	if err := seedDeals(db); err != nil {
		log.Printf("WARNING: failed to seed deals: %v", err)
	}

	// A/B testing tables
	if err := initABTables(db); err != nil {
		log.Printf("WARNING: failed to init A/B tables: %v", err)
	}

	// =========================================================================
	// BRAND PORTAL TABLES (M4)
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_users (
			id            SERIAL PRIMARY KEY,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			company_name  TEXT NOT NULL DEFAULT '',
			verified      INTEGER NOT NULL DEFAULT 0,
			verify_token  TEXT DEFAULT NULL,
			badge         TEXT NOT NULL DEFAULT '',
			created_at    TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_users table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_sessions (
			token      TEXT PRIMARY KEY,
			brand_user_id INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_sessions table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_sessions_expires ON brand_sessions(expires_at)")

	// Link brand_deals to brand_users
	addColumnIfNotExists(db, "brand_deals", "brand_user_id", "INTEGER DEFAULT NULL")

	// Brand logo
	addColumnIfNotExists(db, "brand_users", "logo_data", "TEXT DEFAULT NULL")
	addColumnIfNotExists(db, "brand_users", "logo_mime", "TEXT DEFAULT NULL")

	// Brand balance for Stripe pre-funded deposits
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_balance (
			brand_user_id   INTEGER PRIMARY KEY,
			balance_cents   INTEGER NOT NULL DEFAULT 0,
			last_deposit_at TIMESTAMPTZ DEFAULT NULL,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_balance table: %w", err)
	}

	// Brand transactions ledger
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_transactions (
			id            SERIAL PRIMARY KEY,
			brand_user_id INTEGER NOT NULL,
			type          TEXT NOT NULL,
			amount_cents  INTEGER NOT NULL,
			description   TEXT NOT NULL DEFAULT '',
			stripe_id     TEXT DEFAULT NULL,
			created_at    TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_transactions table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_transactions_user ON brand_transactions(brand_user_id)")

	// Bug reports table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bug_reports (
			id          SERIAL PRIMARY KEY,
			user_id     TEXT,
			description TEXT NOT NULL,
			created_at  TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating bug_reports table: %w", err)
	}

	// =========================================================================
	// M7: RECEIPT READING ENHANCEMENT
	// =========================================================================

	// Structured line items extracted from receipts
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipt_line_items (
			id              SERIAL PRIMARY KEY,
			receipt_id      INTEGER NOT NULL,
			raw_text        TEXT NOT NULL,
			price_cents     INTEGER NOT NULL DEFAULT 0,
			quantity        INTEGER NOT NULL DEFAULT 1,
			matched_item_id INTEGER DEFAULT NULL,
			matched_name    TEXT NOT NULL DEFAULT '',
			confidence      TEXT NOT NULL DEFAULT 'low',
			created_at      TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (receipt_id) REFERENCES receipts(id) ON DELETE CASCADE,
			FOREIGN KEY (matched_item_id) REFERENCES list_items(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating receipt_line_items table: %w", err)
	}

	// Swap claims: links receipts to accepted swaps for purchase verification
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_claims (
			id                SERIAL PRIMARY KEY,
			receipt_id        INTEGER NOT NULL,
			swap_event_id     INTEGER NOT NULL,
			deal_id           INTEGER NOT NULL,
			user_id           TEXT NOT NULL,
			status            TEXT NOT NULL DEFAULT 'pending',
			verified_at       TIMESTAMPTZ DEFAULT NULL,
			payout_held_until TIMESTAMPTZ DEFAULT NULL,
			created_at        TIMESTAMPTZ DEFAULT NOW(),
			FOREIGN KEY (receipt_id) REFERENCES receipts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_claims table: %w", err)
	}

	// Retailer abbreviation database
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS retailer_abbreviations (
			id            SERIAL PRIMARY KEY,
			abbreviation  TEXT NOT NULL,
			full_name     TEXT NOT NULL,
			retailer      TEXT NOT NULL DEFAULT '',
			confirmations INTEGER NOT NULL DEFAULT 1,
			created_at    TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(abbreviation, retailer)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating retailer_abbreviations table: %w", err)
	}

	// Add structured fields to receipts table
	for _, c := range []struct{ col, def string }{
		{"store_name", "TEXT NOT NULL DEFAULT ''"},
		{"store_address", "TEXT NOT NULL DEFAULT ''"},
		{"transaction_date", "TEXT NOT NULL DEFAULT ''"},
		{"subtotal_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"tax_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"total_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"status", "TEXT NOT NULL DEFAULT 'processed'"},
		{"swaps_verified", "INTEGER NOT NULL DEFAULT 0"},
	} {
		addColumnIfNotExists(db, "receipts", c.col, c.def)
	}

	db.Exec("CREATE INDEX IF NOT EXISTS idx_receipt_line_items_receipt ON receipt_line_items(receipt_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_claims_receipt ON swap_claims(receipt_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_claims_user ON swap_claims(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_retailer_abbreviations_abbr ON retailer_abbreviations(abbreviation)")

	// Earnings: attachment support on swap_rebates
	addColumnIfNotExists(db, "swap_rebates", "attachment_path", "TEXT NOT NULL DEFAULT ''")

	// savings_milestones: tracks which savings milestones a user has acknowledged
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS savings_milestones (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			cents      INTEGER NOT NULL,
			reached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			acked_at   TIMESTAMPTZ DEFAULT NULL,
			UNIQUE(user_id, cents)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating savings_milestones table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_savings_milestones_user ON savings_milestones(user_id)")

	// Brand digest preferences
	addColumnIfNotExists(db, "brand_users", "digest_opt_out", "INTEGER NOT NULL DEFAULT 0")
	addColumnIfNotExists(db, "brand_users", "last_digest_at", "TIMESTAMPTZ DEFAULT NULL")

	// =========================================================================
	// LEARNING LOOP INFRASTRUCTURE (Sprint 11)
	// =========================================================================

	// user_signals: captures discrete behavioral events for learning
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_signals (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id),
			signal     TEXT NOT NULL,
			category   TEXT NOT NULL DEFAULT '',
			value      TEXT NOT NULL DEFAULT '',
			context    JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_signals table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_user ON user_signals(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_signal ON user_signals(signal)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_created ON user_signals(created_at)")

	// penny_learnings: aggregated insights Penny has derived from signals
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_learnings (
			id         SERIAL PRIMARY KEY,
			scope      TEXT NOT NULL,
			scope_id   TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL DEFAULT '',
			confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(scope, scope_id, key)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating penny_learnings table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_penny_learnings_scope ON penny_learnings(scope, scope_id)")

	// --- Dietary profile table ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_dietary_profiles (
			id          SERIAL PRIMARY KEY,
			user_id     TEXT NOT NULL UNIQUE REFERENCES users(id),
			allergens   JSONB NOT NULL DEFAULT '[]'::jsonb,
			diets       JSONB NOT NULL DEFAULT '[]'::jsonb,
			avoid_brands JSONB NOT NULL DEFAULT '[]'::jsonb,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_dietary_profiles table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_dietary_user ON user_dietary_profiles(user_id)")

	// penny_context_cache: pre-computed context for Penny prompts
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_context_cache (
			user_id      TEXT PRIMARY KEY REFERENCES users(id),
			context_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			computed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating penny_context_cache table: %w", err)
	}

	// swap_events: additional columns
	addColumnIfNotExists(db, "swap_events", "decline_reason", "TEXT NOT NULL DEFAULT ''")
	addColumnIfNotExists(db, "swap_events", "time_to_action_ms", "INTEGER NOT NULL DEFAULT 0")
	addColumnIfNotExists(db, "swap_events", "why_tapped", "INTEGER NOT NULL DEFAULT 0")

	// =========================================================================
	// DAILY METRICS AGGREGATION TABLES
	// =========================================================================

	// deal_performance_daily: materialized daily deal metrics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS deal_performance_daily (
			id              SERIAL PRIMARY KEY,
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
	db.Exec("CREATE INDEX IF NOT EXISTS idx_deal_perf_daily_deal ON deal_performance_daily(deal_id, date)")

	// category_swap_insights: aggregated swap metrics across deals in a category
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_swap_insights (
			id                  SERIAL PRIMARY KEY,
			category            TEXT NOT NULL UNIQUE,
			avg_acceptance_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			avg_discount_pct    DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			last_computed       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating category_swap_insights table: %w", err)
	}
	addColumnIfNotExists(db, "category_swap_insights", "top_decline_reason", "TEXT NOT NULL DEFAULT ''")

	// business_metrics_daily: daily snapshot of key business metrics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS business_metrics_daily (
			id                  SERIAL PRIMARY KEY,
			date                TEXT NOT NULL UNIQUE,
			dau                 INTEGER NOT NULL DEFAULT 0,
			wau                 INTEGER NOT NULL DEFAULT 0,
			mau                 INTEGER NOT NULL DEFAULT 0,
			new_users           INTEGER NOT NULL DEFAULT 0,
			swaps_offered       INTEGER NOT NULL DEFAULT 0,
			swaps_accepted      INTEGER NOT NULL DEFAULT 0,
			swaps_declined      INTEGER NOT NULL DEFAULT 0,
			acceptance_rate     DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			gross_revenue_cents INTEGER NOT NULL DEFAULT 0,
			active_deals        INTEGER NOT NULL DEFAULT 0,
			active_brands       INTEGER NOT NULL DEFAULT 0,
			computed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating business_metrics_daily table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_business_metrics_date ON business_metrics_daily(date)")

	// Churn prediction signals
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS churn_signals (
			id           SERIAL PRIMARY KEY,
			user_id      TEXT NOT NULL REFERENCES users(id),
			signal_type  TEXT NOT NULL,
			signal_value DOUBLE PRECISION NOT NULL,
			risk_score   DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			computed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, signal_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating churn_signals table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_churn_signals_user ON churn_signals(user_id, signal_type)")

	// User cohort retention tracking
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_cohorts (
			id                  SERIAL PRIMARY KEY,
			user_id             TEXT NOT NULL UNIQUE REFERENCES users(id),
			cohort_week         TEXT NOT NULL,
			signup_source       TEXT NOT NULL DEFAULT '',
			first_swap_at       TIMESTAMPTZ DEFAULT NULL,
			first_receipt_at    TIMESTAMPTZ DEFAULT NULL,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			total_savings_cents INTEGER NOT NULL DEFAULT 0,
			last_active         TIMESTAMPTZ DEFAULT NULL,
			is_churned          INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_cohorts table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_cohorts_week ON user_cohorts(cohort_week)")

	// Marketplace health index
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS marketplace_health (
			id                    SERIAL PRIMARY KEY,
			date                  TEXT NOT NULL UNIQUE,
			supply_score          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			demand_score          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			balance_score         DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			revenue_concentration DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			health_index          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			computed_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating marketplace_health table: %w", err)
	}

	// =========================================================================
	// CATEGORY TAXONOMY
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id              SERIAL PRIMARY KEY,
			name            TEXT NOT NULL UNIQUE,
			parent_category TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("creating categories table: %w", err)
	}

	if err := seedCategories(db); err != nil {
		log.Printf("WARNING: failed to seed categories: %v", err)
	}

	// Link brand_deals to categories
	addColumnIfNotExists(db, "brand_deals", "category_id", "INTEGER NOT NULL DEFAULT 0")

	// v1 Stretch: rich product descriptions
	addColumnIfNotExists(db, "brand_deals", "product_description", "TEXT NOT NULL DEFAULT ''")
	addColumnIfNotExists(db, "brand_deals", "key_benefits", "JSONB NOT NULL DEFAULT '[]'::jsonb")
	addColumnIfNotExists(db, "brand_deals", "brand_story", "TEXT NOT NULL DEFAULT ''")

	// Swap conversational depth tracking
	addColumnIfNotExists(db, "swap_events", "conversation_depth", "INTEGER NOT NULL DEFAULT 0")
	addColumnIfNotExists(db, "swap_events", "why_tapped_at", "TIMESTAMPTZ DEFAULT NULL")

	// =========================================================================
	// CATEGORY PRICING TIERS
	// =========================================================================
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
	if err := seedPricingTiers(db); err != nil {
		log.Printf("WARNING: failed to seed pricing tiers: %v", err)
	}

	// =========================================================================
	// USER SWAP PREDICTIONS
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_swap_predictions (
			user_id             TEXT NOT NULL,
			category_id         INTEGER NOT NULL DEFAULT 0,
			brand               TEXT NOT NULL DEFAULT '',
			acceptance_rate     DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			total_offers        INTEGER NOT NULL DEFAULT 0,
			total_accepts       INTEGER NOT NULL DEFAULT 0,
			avg_time_to_action_ms INTEGER NOT NULL DEFAULT 0,
			confidence          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			computed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, category_id, brand)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating user_swap_predictions table: %w", err)
	}

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_user_swap_predictions_user ON user_swap_predictions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_user_swap_predictions_category ON user_swap_predictions(user_id, category_id)",
	} {
		db.Exec(stmt)
	}

	// =========================================================================
	// STORES (L4 Location Learning)
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stores (
			id           SERIAL PRIMARY KEY,
			name         TEXT NOT NULL,
			chain        TEXT NOT NULL DEFAULT '',
			address      TEXT NOT NULL DEFAULT '',
			city         TEXT NOT NULL DEFAULT '',
			state        TEXT NOT NULL DEFAULT '',
			zip          TEXT NOT NULL DEFAULT '',
			latitude     DOUBLE PRECISION,
			longitude    DOUBLE PRECISION,
			place_id     TEXT NOT NULL DEFAULT '',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(name, address)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating stores table: %w", err)
	}

	// Store price observations
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS store_prices (
			id           SERIAL PRIMARY KEY,
			store_id     INTEGER NOT NULL REFERENCES stores(id),
			product_name TEXT NOT NULL,
			brand        TEXT NOT NULL DEFAULT '',
			price_cents  INTEGER NOT NULL,
			category_id  INTEGER NOT NULL DEFAULT 0,
			source       TEXT NOT NULL DEFAULT 'receipt',
			observed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(store_id, product_name, brand, observed_at)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating store_prices table: %w", err)
	}

	// Link receipts/trips to stores
	addColumnIfNotExists(db, "receipts", "store_id", "INTEGER DEFAULT NULL")
	addColumnIfNotExists(db, "shopping_trips", "store_id", "INTEGER DEFAULT NULL")

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_stores_chain ON stores(chain)",
		"CREATE INDEX IF NOT EXISTS idx_stores_place_id ON stores(place_id)",
		"CREATE INDEX IF NOT EXISTS idx_store_prices_store ON store_prices(store_id)",
		"CREATE INDEX IF NOT EXISTS idx_store_prices_product ON store_prices(product_name, brand)",
	} {
		db.Exec(stmt)
	}

	// =========================================================================
	// SWAP PROPAGATION TRACKING
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_propagations (
			id              SERIAL PRIMARY KEY,
			deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
			source_user_id  TEXT NOT NULL,
			adopter_user_id TEXT NOT NULL,
			household_id    INTEGER NOT NULL,
			item_name       TEXT NOT NULL,
			brand           TEXT NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(deal_id, adopter_user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating swap_propagations table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_propagations_deal ON swap_propagations(deal_id)")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_subscriptions (
			id             SERIAL PRIMARY KEY,
			brand_user_id  INTEGER NOT NULL,
			tier           TEXT NOT NULL DEFAULT 'free',
			started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at     TIMESTAMPTZ,
			UNIQUE(brand_user_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_subscriptions table: %w", err)
	}

	// =========================================================================
	// CATEGORY ALERTS (disruption signals)
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_alerts (
			id          SERIAL PRIMARY KEY,
			alert_type  TEXT NOT NULL,
			brand       TEXT NOT NULL DEFAULT '',
			category_id INTEGER NOT NULL DEFAULT 0,
			severity    TEXT NOT NULL DEFAULT 'low',
			message     TEXT NOT NULL,
			data_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating category_alerts table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_category_alerts_type ON category_alerts(alert_type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_category_alerts_category ON category_alerts(category_id)")

	// =========================================================================
	// FEEDBACK SYSTEM
	// =========================================================================
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feedback_requests (
			id               SERIAL PRIMARY KEY,
			user_id          TEXT NOT NULL REFERENCES users(id),
			body             TEXT NOT NULL,
			type             TEXT NOT NULL DEFAULT 'feature',
			urgency          TEXT NOT NULL DEFAULT 'normal',
			status           TEXT NOT NULL DEFAULT 'new',
			admin_notes      TEXT NOT NULL DEFAULT '',
			github_issue_url TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMPTZ DEFAULT NOW(),
			updated_at       TIMESTAMPTZ
		)
	`)
	if err != nil {
		return fmt.Errorf("creating feedback_requests table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_feedback_requests_status ON feedback_requests(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_feedback_requests_user_id ON feedback_requests(user_id)")

	// Account merge audit trail
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS account_merges (
			id              SERIAL PRIMARY KEY,
			surviving_user  TEXT NOT NULL REFERENCES users(id),
			merged_user     TEXT,
			provider_linked TEXT NOT NULL,
			provider_id     TEXT NOT NULL,
			data_actions    JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at      TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating account_merges table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_account_merges_surviving ON account_merges(surviving_user)")

	// Brand portal feedback
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_feedback_requests (
			id               SERIAL PRIMARY KEY,
			brand_user_id    INTEGER NOT NULL REFERENCES brand_users(id),
			body             TEXT NOT NULL,
			type             TEXT NOT NULL DEFAULT 'feature',
			urgency          TEXT NOT NULL DEFAULT 'normal',
			status           TEXT NOT NULL DEFAULT 'new',
			admin_notes      TEXT NOT NULL DEFAULT '',
			github_issue_url TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMPTZ DEFAULT NOW(),
			updated_at       TIMESTAMPTZ
		)
	`)
	if err != nil {
		return fmt.Errorf("creating brand_feedback_requests table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_feedback_status ON brand_feedback_requests(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_feedback_user ON brand_feedback_requests(brand_user_id)")

	// F3: Auto-Learn Store Department Order
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS department_order_events (
			id         SERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			store_name TEXT NOT NULL DEFAULT '',
			department TEXT NOT NULL,
			checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			trip_date  TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating department_order_events table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_dept_order_user_store ON department_order_events(user_id, store_name)")

	// --- Campaign Review (compliance) ---
	for _, c := range []struct{ col, def string }{
		{"review_status", "TEXT NOT NULL DEFAULT ''"},
		{"review_note", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_by", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_at", "TIMESTAMPTZ"},
		{"submitted_at", "TIMESTAMPTZ"},
	} {
		addColumnIfNotExists(db, "brand_deals", c.col, c.def)
	}

	// --- Fully Free Trial Campaigns ---
	for _, c := range []struct{ col, def string }{
		{"max_rebate_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"platform_fee_cents", "INTEGER NOT NULL DEFAULT 100"},
		{"require_verified_identity", "INTEGER NOT NULL DEFAULT 1"},
		{"receipt_grace_days", "INTEGER NOT NULL DEFAULT 7"},
		{"upc", "TEXT NOT NULL DEFAULT ''"},
	} {
		addColumnIfNotExists(db, "brand_deals", c.col, c.def)
	}
	// swap_rebates: receipt verification fields for fully_free
	for _, c := range []struct{ col, def string }{
		{"verified_amount_cents", "INTEGER NOT NULL DEFAULT 0"},
		{"verified_at", "TIMESTAMPTZ"},
		{"receipt_image_url", "TEXT NOT NULL DEFAULT ''"},
	} {
		addColumnIfNotExists(db, "swap_rebates", c.col, c.def)
	}

	// Saved list sharing: nullable saved_list_id on shared_links
	addColumnIfNotExists(db, "shared_links", "saved_list_id", "INTEGER DEFAULT NULL")

	// Backfill emojis for existing items
	backfillItemEmojis(db)

	return nil
}

// seedCategories populates the categories table with common grocery
// subcategories organized under parent groups. Idempotent -- only inserts
// when the table is empty.
func seedCategories(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	type cat struct {
		name, parent string
	}
	categories := []cat{
		// Dairy
		{"Milk", "Dairy"},
		{"Cheese", "Dairy"},
		{"Yogurt", "Dairy"},
		{"Butter", "Dairy"},
		{"Cream", "Dairy"},
		{"Eggs", "Dairy"},
		{"Sour Cream", "Dairy"},
		{"Cottage Cheese", "Dairy"},
		// Produce
		{"Fresh Fruit", "Produce"},
		{"Fresh Vegetables", "Produce"},
		{"Salad Greens", "Produce"},
		{"Herbs", "Produce"},
		{"Organic Produce", "Produce"},
		// Pantry
		{"Pasta", "Pantry"},
		{"Rice", "Pantry"},
		{"Canned Vegetables", "Pantry"},
		{"Canned Soup", "Pantry"},
		{"Pasta Sauce", "Pantry"},
		{"Cooking Oil", "Pantry"},
		{"Flour", "Pantry"},
		{"Sugar", "Pantry"},
		{"Cereal", "Pantry"},
		{"Oatmeal", "Pantry"},
		{"Beans", "Pantry"},
		// Beverages
		{"Water", "Beverages"},
		{"Soda", "Beverages"},
		{"Juice", "Beverages"},
		{"Coffee", "Beverages"},
		{"Tea", "Beverages"},
		{"Sports Drinks", "Beverages"},
		{"Energy Drinks", "Beverages"},
		{"Sparkling Water", "Beverages"},
		{"Kombucha", "Beverages"},
		// Frozen
		{"Frozen Meals", "Frozen"},
		{"Frozen Vegetables", "Frozen"},
		{"Frozen Fruit", "Frozen"},
		{"Ice Cream", "Frozen"},
		{"Frozen Pizza", "Frozen"},
		{"Frozen Breakfast", "Frozen"},
		// Meat
		{"Chicken", "Meat"},
		{"Beef", "Meat"},
		{"Pork", "Meat"},
		{"Deli Meat", "Meat"},
		{"Sausage", "Meat"},
		{"Seafood", "Meat"},
		{"Plant-Based Meat", "Meat"},
		// Snacks
		{"Chips", "Snacks"},
		{"Crackers", "Snacks"},
		{"Cookies", "Snacks"},
		{"Nuts", "Snacks"},
		{"Granola Bars", "Snacks"},
		{"Popcorn", "Snacks"},
		{"Dried Fruit", "Snacks"},
		{"Candy", "Snacks"},
		// Condiments
		{"Ketchup", "Condiments"},
		{"Mustard", "Condiments"},
		{"Mayonnaise", "Condiments"},
		{"Hot Sauce", "Condiments"},
		{"Salad Dressing", "Condiments"},
		{"Soy Sauce", "Condiments"},
		{"Salsa", "Condiments"},
		// Baking
		{"Baking Mix", "Baking"},
		{"Chocolate Chips", "Baking"},
		{"Vanilla Extract", "Baking"},
		{"Baking Powder", "Baking"},
		{"Frosting", "Baking"},
		// Cleaning
		{"Laundry Detergent", "Cleaning"},
		{"Dish Soap", "Cleaning"},
		{"All-Purpose Cleaner", "Cleaning"},
		{"Disinfectant", "Cleaning"},
		{"Trash Bags", "Cleaning"},
		{"Paper Towels", "Cleaning"},
		// Personal Care
		{"Shampoo", "Personal Care"},
		{"Body Wash", "Personal Care"},
		{"Toothpaste", "Personal Care"},
		{"Deodorant", "Personal Care"},
		{"Lotion", "Personal Care"},
		{"Soap", "Personal Care"},
		// Baby
		{"Diapers", "Baby"},
		{"Baby Food", "Baby"},
		{"Baby Formula", "Baby"},
		{"Baby Wipes", "Baby"},
		// Pet
		{"Dog Food", "Pet"},
		{"Cat Food", "Pet"},
		{"Pet Treats", "Pet"},
		{"Cat Litter", "Pet"},
		// Paper Goods
		{"Toilet Paper", "Paper Goods"},
		{"Napkins", "Paper Goods"},
		{"Tissues", "Paper Goods"},
		{"Aluminum Foil", "Paper Goods"},
		{"Plastic Wrap", "Paper Goods"},
		{"Storage Bags", "Paper Goods"},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO categories (name, parent_category) VALUES ($1, $2)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range categories {
		if _, err := stmt.Exec(c.name, c.parent); err != nil {
			return fmt.Errorf("seeding category %q: %w", c.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("[categories] seeded %d categories", len(categories))
	return nil
}

// categorizeProduct attempts to match a product name to a category using
// case-insensitive substring matching against category names. Returns the
// category ID, or 0 if no match is found.
func categorizeProduct(db dbExecer, productName string) int {
	if productName == "" {
		return 0
	}
	var id int
	err := db.QueryRow(
		"SELECT id FROM categories WHERE LOWER($1) LIKE '%' || LOWER(name) || '%' ORDER BY LENGTH(name) DESC LIMIT 1",
		productName,
	).Scan(&id)
	if err == nil {
		return id
	}
	return 0
}

// seedPricingTiers populates default pricing tiers based on category acceptance
// rates. Higher-value categories (Dairy, Beverages) cost more per swap.
// Idempotent -- only inserts when table is empty.
func seedPricingTiers(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM category_pricing_tiers").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	type tier struct {
		parent   string
		minCents int
		sugCents int
		label    string
	}
	tiers := []tier{
		// Premium: high-frequency, high-acceptance categories
		{"Dairy", 250, 300, "premium"},
		{"Beverages", 250, 300, "premium"},
		{"Snacks", 200, 250, "premium"},
		// Standard: moderate acceptance
		{"Produce", 150, 200, "standard"},
		{"Pantry", 150, 200, "standard"},
		{"Meat", 150, 200, "standard"},
		{"Frozen", 150, 200, "standard"},
		{"Cereal", 150, 200, "standard"},
		// Economy: lower acceptance, price-sensitive categories
		{"Condiments", 100, 150, "economy"},
		{"Baking", 100, 150, "economy"},
		{"Cleaning", 100, 150, "economy"},
		{"Personal Care", 100, 150, "economy"},
		{"Baby", 100, 150, "economy"},
		{"Pet", 100, 150, "economy"},
		{"Paper Goods", 100, 150, "economy"},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range tiers {
		if _, err := tx.Exec(
			"INSERT INTO category_pricing_tiers (parent_category, min_price_cents, suggested_cents, tier_label) VALUES ($1, $2, $3, $4)",
			t.parent, t.minCents, t.sugCents, t.label,
		); err != nil {
			return fmt.Errorf("seeding tier %q: %w", t.parent, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("[pricing] seeded %d category pricing tiers", len(tiers))
	return nil
}

// getCategoryPricingTier returns the min price and tier label for a given category.
// Falls back to economy tier ($1.00) if no specific tier is found.
func getCategoryPricingTier(db dbExecer, categoryID int) (minCents int, suggestedCents int, tierLabel string) {
	if categoryID == 0 {
		return 100, 150, "economy"
	}
	var parent string
	if err := db.QueryRow("SELECT parent_category FROM categories WHERE id = $1", categoryID).Scan(&parent); err != nil {
		return 100, 150, "economy"
	}
	if err := db.QueryRow(
		"SELECT min_price_cents, suggested_cents, tier_label FROM category_pricing_tiers WHERE parent_category = $1",
		parent,
	).Scan(&minCents, &suggestedCents, &tierLabel); err != nil {
		return 100, 150, "economy"
	}
	return
}

// seedDeals populates the brand_deals table with test swap deals if empty.
// Idempotent -- only inserts when no deals exist.
func seedDeals(db dbExecer) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM brand_deals").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// $20 total budget ($2000 cents) split across 5 brand-level deals.
	deals := []struct {
		competitor, offerBrand, offerProduct        string
		discountCents, budgetCents                  int
		offerPriceCents, competitorPriceCents        int
		offerSize, competitorSize                    string
		offerUnitPrice, competitorUnitPrice          string
		unitLabel, whyBullets, allergenNotes         string
	}{
		{
			"skippy", "Jif", "Creamy Peanut Butter", 125, 400,
			348, 473, "16 oz", "16.3 oz", "$0.22/oz", "$0.29/oz", "oz",
			`["No palm oil","4.6★ avg rating","24% less per ounce"]`, "",
		},
		{
			"lacroix", "Waterloo", "Sparkling Water Variety Pack", 50, 400,
			498, 548, "12-pack", "12-pack", "$0.42/can", "$0.46/can", "can",
			`["Bolder natural flavors","4.5★ avg rating","No artificial sweeteners"]`, "",
		},
		{
			"tide", "All Free Clear", "Liquid Laundry Detergent", 100, 400,
			897, 1197, "88 oz", "92 oz", "$0.10/oz", "$0.13/oz", "oz",
			`["Hypoallergenic","Dermatologist recommended","23% less per load"]`,
			"Free of dyes and perfumes.",
		},
		{
			"ragu", "Rao's", "Marinara Sauce", 50, 400,
			498, 548, "24 oz", "24 oz", "$0.21/oz", "$0.23/oz", "oz",
			`["All-natural ingredients","No added sugar","Slow-simmered recipe"]`, "",
		},
		{
			"archer", "Chomps", "Beef Sticks", 75, 400,
			199, 274, "1.15 oz", "1 oz", "$1.73/oz", "$2.74/oz", "oz",
			`["100% grass-fed beef","Whole30 approved","37% less per ounce"]`,
			"Contains: beef. Free of top 8 allergens.",
		},
	}

	for _, d := range deals {
		if _, err := db.Exec(
			`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
			 discount_cents, total_budget_cents, status,
			 offer_price_cents, competitor_price_cents, offer_size, competitor_size,
			 offer_unit_price, competitor_unit_price, unit_label, why_bullets, allergen_notes)
			 VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			d.competitor, d.offerBrand, d.offerProduct, d.discountCents, d.budgetCents,
			d.offerPriceCents, d.competitorPriceCents, d.offerSize, d.competitorSize,
			d.offerUnitPrice, d.competitorUnitPrice, d.unitLabel, d.whyBullets, d.allergenNotes,
		); err != nil {
			return fmt.Errorf("seeding deal %s->%s: %w", d.competitor, d.offerBrand, err)
		}
	}
	log.Printf("[deals] seeded %d sku_swap deals ($20 total budget)", len(deals))

	// Seed one deal of each new type for testing
	newTypeDeals := []struct {
		competitor, offerBrand, offerProduct string
		discountCents, budgetCents           int
		offerPriceCents, competitorPriceCents int
		offerSize, competitorSize             string
		offerUnitPrice, competitorUnitPrice   string
		unitLabel, whyBullets, allergenNotes  string
		dealType                              string
		quantityOffered                       int
		trialProductName, trialSize           string
		campaignTag, endsAt                   string
	}{
		{
			"cheerios", "Magic Spoon", "Cereal", 150, 400,
			898, 598, "7 oz", "18 oz", "$1.28/oz", "$0.33/oz", "oz",
			`["High protein (13g/serving)","Zero sugar","Keto-friendly"]`, "",
			"bogo", 2, "", "", "", "",
		},
		{
			"folgers", "Counter Culture", "Coffee Sample", 100, 400,
			0, 899, "2 oz sample", "12 oz", "", "$0.75/oz", "oz",
			`["Single-origin Ethiopian","Direct trade certified","Try before you buy"]`, "",
			"trial", 1, "Counter Culture Coffee Sample", "2 oz", "", "",
		},
		{
			"oreo", "Partake", "Holiday Cookie Box", 200, 400,
			599, 499, "7 oz", "15.25 oz", "$0.86/oz", "$0.33/oz", "oz",
			`["Allergy-friendly (top 9 free)","Limited holiday edition","Vegan & non-GMO"]`,
			"Free of top 9 allergens.",
			"seasonal", 1, "", "", "holiday_2026",
			time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05Z"),
		},
	}
	for _, d := range newTypeDeals {
		if _, err := db.Exec(
			`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
			 discount_cents, total_budget_cents, status,
			 offer_price_cents, competitor_price_cents, offer_size, competitor_size,
			 offer_unit_price, competitor_unit_price, unit_label, why_bullets, allergen_notes,
			 deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag, ends_at)
			 VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, NULLIF($20, '')::timestamptz)`,
			d.competitor, d.offerBrand, d.offerProduct, d.discountCents, d.budgetCents,
			d.offerPriceCents, d.competitorPriceCents, d.offerSize, d.competitorSize,
			d.offerUnitPrice, d.competitorUnitPrice, d.unitLabel, d.whyBullets, d.allergenNotes,
			d.dealType, d.quantityOffered, d.trialProductName, d.trialSize, d.campaignTag, d.endsAt,
		); err != nil {
			return fmt.Errorf("seeding %s deal %s->%s: %w", d.dealType, d.competitor, d.offerBrand, err)
		}
	}
	log.Printf("[deals] seeded %d new-type deals (bogo, trial, seasonal)", len(newTypeDeals))

	return nil
}

// seedBrands populates the brands and brand_owners tables if they are empty.
// This is idempotent -- it only inserts when the brands table has no rows.
func seedBrands(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM brands").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert brand owners
	owners := []string{
		"Procter & Gamble", "Unilever", "Nestle", "PepsiCo", "Coca-Cola",
		"General Mills", "Kellogg's", "Kraft Heinz", "Colgate-Palmolive",
		"Johnson & Johnson", "Mondelez", "Mars", "Danone", "Conagra",
		"Campbell Soup", "Hormel", "Tyson", "Smucker's", "Church & Dwight",
		"Reckitt", "SC Johnson", "Clorox", "Energizer", "Henkel",
	}
	ownerIDs := make(map[string]int64)
	for _, name := range owners {
		var id int64
		err := tx.QueryRow("INSERT INTO brand_owners (name) VALUES ($1) RETURNING id", name).Scan(&id)
		if err != nil {
			return fmt.Errorf("inserting owner %q: %w", name, err)
		}
		ownerIDs[name] = id
	}

	// brand entries: name, domain, owner key
	type brand struct {
		name, domain, owner string
	}
	brands := []brand{
		// Procter & Gamble
		{"Tide", "tide.com", "Procter & Gamble"},
		{"Bounty", "bountytowels.com", "Procter & Gamble"},
		{"Charmin", "charmin.com", "Procter & Gamble"},
		{"Pampers", "pampers.com", "Procter & Gamble"},
		{"Gillette", "gillette.com", "Procter & Gamble"},
		{"Downy", "downy.com", "Procter & Gamble"},
		{"Crest", "crest.com", "Procter & Gamble"},
		{"Oral-B", "oralb.com", "Procter & Gamble"},
		{"Dawn", "dawn-dish.com", "Procter & Gamble"},
		{"Febreze", "febreze.com", "Procter & Gamble"},
		{"Swiffer", "swiffer.com", "Procter & Gamble"},
		{"Head & Shoulders", "headandshoulders.com", "Procter & Gamble"},
		{"Old Spice", "oldspice.com", "Procter & Gamble"},
		{"Gain", "gainlaundry.com", "Procter & Gamble"},
		{"Mr. Clean", "mrclean.com", "Procter & Gamble"},
		{"Cascade", "cascadeclean.com", "Procter & Gamble"},
		// Unilever
		{"Dove", "dove.com", "Unilever"},
		{"Axe", "axe.com", "Unilever"},
		{"Hellmann's", "hellmanns.com", "Unilever"},
		{"Knorr", "knorr.com", "Unilever"},
		{"Ben & Jerry's", "benjerry.com", "Unilever"},
		{"Degree", "degreedeodorant.com", "Unilever"},
		{"Vaseline", "vaseline.com", "Unilever"},
		{"TRESemme", "tresemme.com", "Unilever"},
		{"Seventh Generation", "seventhgeneration.com", "Unilever"},
		{"Suave", "suave.com", "Unilever"},
		{"Magnum", "magnumicecream.com", "Unilever"},
		// Nestle
		{"Nescafe", "nescafe.com", "Nestle"},
		{"Gerber", "gerber.com", "Nestle"},
		{"Stouffer's", "stouffers.com", "Nestle"},
		{"Hot Pockets", "hotpockets.com", "Nestle"},
		{"DiGiorno", "digiorno.com", "Nestle"},
		{"Coffee Mate", "coffeemate.com", "Nestle"},
		{"Lean Cuisine", "leancuisine.com", "Nestle"},
		{"Toll House", "verybestbaking.com", "Nestle"},
		{"Purina", "purina.com", "Nestle"},
		{"Fancy Feast", "fancyfeast.com", "Nestle"},
		{"Haagen-Dazs", "haagendazs.com", "Nestle"},
		{"San Pellegrino", "sanpellegrino.com", "Nestle"},
		{"Perrier", "perrier.com", "Nestle"},
		// PepsiCo
		{"Pepsi", "pepsi.com", "PepsiCo"},
		{"Lay's", "lays.com", "PepsiCo"},
		{"Doritos", "doritos.com", "PepsiCo"},
		{"Gatorade", "gatorade.com", "PepsiCo"},
		{"Quaker", "quakeroats.com", "PepsiCo"},
		{"Cheetos", "cheetos.com", "PepsiCo"},
		{"Tropicana", "tropicana.com", "PepsiCo"},
		{"Mountain Dew", "mountaindew.com", "PepsiCo"},
		{"Tostitos", "tostitos.com", "PepsiCo"},
		{"Ruffles", "ruffles.com", "PepsiCo"},
		{"Fritos", "fritos.com", "PepsiCo"},
		{"Aquafina", "aquafina.com", "PepsiCo"},
		{"Bubly", "bubly.com", "PepsiCo"},
		{"SunChips", "sunchips.com", "PepsiCo"},
		// Coca-Cola
		{"Coca-Cola", "coca-cola.com", "Coca-Cola"},
		{"Sprite", "sprite.com", "Coca-Cola"},
		{"Fanta", "fanta.com", "Coca-Cola"},
		{"Dasani", "dasani.com", "Coca-Cola"},
		{"Minute Maid", "minutemaid.com", "Coca-Cola"},
		{"Smartwater", "drinksmartwater.com", "Coca-Cola"},
		{"Powerade", "powerade.com", "Coca-Cola"},
		{"Honest Tea", "honesttea.com", "Coca-Cola"},
		{"Topo Chico", "topochicousa.com", "Coca-Cola"},
		{"Simply", "simplyorangejuice.com", "Coca-Cola"},
		{"Gold Peak", "goldpeaktea.com", "Coca-Cola"},
		{"Fairlife", "fairlife.com", "Coca-Cola"},
		// General Mills
		{"Cheerios", "cheerios.com", "General Mills"},
		{"Nature Valley", "naturevalley.com", "General Mills"},
		{"Betty Crocker", "bettycrocker.com", "General Mills"},
		{"Pillsbury", "pillsbury.com", "General Mills"},
		{"Yoplait", "yoplait.com", "General Mills"},
		{"Annie's", "annies.com", "General Mills"},
		{"Progresso", "progresso.com", "General Mills"},
		{"Old El Paso", "oldelpaso.com", "General Mills"},
		{"Totino's", "totinos.com", "General Mills"},
		{"Larabar", "larabar.com", "General Mills"},
		{"Cascadian Farm", "cascadianfarm.com", "General Mills"},
		{"Blue Buffalo", "bluebuffalo.com", "General Mills"},
		{"Fiber One", "fiberone.com", "General Mills"},
		// Kellogg's
		{"Kellogg's", "kelloggs.com", "Kellogg's"},
		{"Pringles", "pringles.com", "Kellogg's"},
		{"Cheez-It", "cheezit.com", "Kellogg's"},
		{"Pop-Tarts", "poptarts.com", "Kellogg's"},
		{"Eggo", "eggowaffles.com", "Kellogg's"},
		{"Rice Krispies", "ricekrispies.com", "Kellogg's"},
		{"Frosted Flakes", "frostedflakes.com", "Kellogg's"},
		{"Froot Loops", "frootloops.com", "Kellogg's"},
		{"Club Crackers", "clubcrackers.com", "Kellogg's"},
		{"RXBAR", "rxbar.com", "Kellogg's"},
		{"MorningStar Farms", "morningstarfarms.com", "Kellogg's"},
		// Kraft Heinz
		{"Kraft", "kraft.com", "Kraft Heinz"},
		{"Heinz", "heinz.com", "Kraft Heinz"},
		{"Oscar Mayer", "oscarmayer.com", "Kraft Heinz"},
		{"Philadelphia", "philadelphia.com", "Kraft Heinz"},
		{"Velveeta", "velveeta.com", "Kraft Heinz"},
		{"Jell-O", "jello.com", "Kraft Heinz"},
		{"Kool-Aid", "koolaid.com", "Kraft Heinz"},
		{"Capri Sun", "caprisun.com", "Kraft Heinz"},
		{"Ore-Ida", "oreida.com", "Kraft Heinz"},
		{"Lunchables", "lunchables.com", "Kraft Heinz"},
		{"Grey Poupon", "greypoupon.com", "Kraft Heinz"},
		// Colgate-Palmolive
		{"Colgate", "colgate.com", "Colgate-Palmolive"},
		{"Palmolive", "palmolive.com", "Colgate-Palmolive"},
		{"Ajax", "ajaxcleaner.com", "Colgate-Palmolive"},
		{"Irish Spring", "irishspring.com", "Colgate-Palmolive"},
		{"Softsoap", "softsoap.com", "Colgate-Palmolive"},
		{"Speed Stick", "speedstick.com", "Colgate-Palmolive"},
		{"Tom's of Maine", "tomsofmaine.com", "Colgate-Palmolive"},
		{"Hill's", "hillspet.com", "Colgate-Palmolive"},
		// Johnson & Johnson
		{"Band-Aid", "band-aid.com", "Johnson & Johnson"},
		{"Tylenol", "tylenol.com", "Johnson & Johnson"},
		{"Neutrogena", "neutrogena.com", "Johnson & Johnson"},
		{"Listerine", "listerine.com", "Johnson & Johnson"},
		{"Aveeno", "aveeno.com", "Johnson & Johnson"},
		{"Johnson's Baby", "johnsonsbaby.com", "Johnson & Johnson"},
		{"Zyrtec", "zyrtec.com", "Johnson & Johnson"},
		{"Benadryl", "benadryl.com", "Johnson & Johnson"},
		// Mondelez
		{"Oreo", "oreo.com", "Mondelez"},
		{"Ritz", "ritzcrackers.com", "Mondelez"},
		{"Triscuit", "triscuit.com", "Mondelez"},
		{"Wheat Thins", "wheatthins.com", "Mondelez"},
		{"Chips Ahoy!", "chipsahoy.com", "Mondelez"},
		{"Cadbury", "cadbury.com", "Mondelez"},
		{"Sour Patch Kids", "sourpatchkids.com", "Mondelez"},
		{"Swedish Fish", "swedishfish.com", "Mondelez"},
		{"Toblerone", "toblerone.com", "Mondelez"},
		{"belVita", "belvitabreakfast.com", "Mondelez"},
		{"Clif Bar", "clifbar.com", "Mondelez"},
		// Mars
		{"M&M's", "mms.com", "Mars"},
		{"Snickers", "snickers.com", "Mars"},
		{"Twix", "twix.com", "Mars"},
		{"Skittles", "skittles.com", "Mars"},
		{"Starburst", "starburst.com", "Mars"},
		{"Pedigree", "pedigree.com", "Mars"},
		{"Uncle Ben's", "unclebens.com", "Mars"},
		{"Dove Chocolate", "dovechocolate.com", "Mars"},
		{"Kind", "kindsnacks.com", "Mars"},
		// Danone
		{"Dannon", "dannon.com", "Danone"},
		{"Evian", "evian.com", "Danone"},
		{"Oikos", "oikosyogurt.com", "Danone"},
		{"Silk", "silk.com", "Danone"},
		{"So Delicious", "sodeliciousdairyfree.com", "Danone"},
		{"International Delight", "internationaldelight.com", "Danone"},
		{"Horizon Organic", "horizon.com", "Danone"},
		{"Activia", "activia.com", "Danone"},
		// Conagra
		{"Hunt's", "hunts.com", "Conagra"},
		{"Healthy Choice", "healthychoice.com", "Conagra"},
		{"Marie Callender's", "mariecallenders.com", "Conagra"},
		{"Slim Jim", "slimjim.com", "Conagra"},
		{"Reddi-Wip", "reddiwip.com", "Conagra"},
		{"PAM", "pamcookingspray.com", "Conagra"},
		{"Duncan Hines", "duncanhines.com", "Conagra"},
		{"Birds Eye", "birdseye.com", "Conagra"},
		{"Vlasic", "vlasic.com", "Conagra"},
		{"Banquet", "banquet.com", "Conagra"},
		// Campbell Soup
		{"Campbell's", "campbells.com", "Campbell Soup"},
		{"V8", "v8juice.com", "Campbell Soup"},
		{"Goldfish", "pepperidgefarm.com", "Campbell Soup"},
		{"Pepperidge Farm", "pepperidgefarm.com", "Campbell Soup"},
		{"Prego", "prego.com", "Campbell Soup"},
		{"Pace", "pacefoods.com", "Campbell Soup"},
		{"Swanson", "swanson.com", "Campbell Soup"},
		{"SpaghettiOs", "spaghettios.com", "Campbell Soup"},
		{"Lance", "lance.com", "Campbell Soup"},
		// Hormel
		{"Hormel", "hormel.com", "Hormel"},
		{"SPAM", "spam.com", "Hormel"},
		{"Skippy", "skippy.com", "Hormel"},
		{"Applegate", "applegate.com", "Hormel"},
		{"Wholly Guacamole", "eatwholly.com", "Hormel"},
		{"Planters", "planters.com", "Hormel"},
		// Tyson
		{"Tyson", "tyson.com", "Tyson"},
		{"Jimmy Dean", "jimmydean.com", "Tyson"},
		{"Hillshire Farm", "hillshirefarm.com", "Tyson"},
		{"Ball Park", "ballparkbrand.com", "Tyson"},
		{"Sara Lee", "saraleedeli.com", "Tyson"},
		// Smucker's
		{"Smucker's", "smuckers.com", "Smucker's"},
		{"Jif", "jif.com", "Smucker's"},
		{"Folgers", "folgers.com", "Smucker's"},
		{"Cafe Bustelo", "cafebustelo.com", "Smucker's"},
		{"Uncrustables", "uncrustables.com", "Smucker's"},
		{"Meow Mix", "meowmix.com", "Smucker's"},
		{"Milk-Bone", "milkbone.com", "Smucker's"},
		// Church & Dwight
		{"Arm & Hammer", "armandhammer.com", "Church & Dwight"},
		{"OxiClean", "oxiclean.com", "Church & Dwight"},
		{"Trojan", "trojanbrands.com", "Church & Dwight"},
		{"Batiste", "batistehair.com", "Church & Dwight"},
		{"Vitafusion", "vitafusion.com", "Church & Dwight"},
		{"Orajel", "orajel.com", "Church & Dwight"},
		// Reckitt
		{"Lysol", "lysol.com", "Reckitt"},
		{"Finish", "finishdishwashing.com", "Reckitt"},
		{"Air Wick", "airwick.com", "Reckitt"},
		{"Woolite", "woolite.com", "Reckitt"},
		{"Mucinex", "mucinex.com", "Reckitt"},
		{"Enfamil", "enfamil.com", "Reckitt"},
		// SC Johnson
		{"Windex", "windex.com", "SC Johnson"},
		{"Ziploc", "ziploc.com", "SC Johnson"},
		{"Glade", "glade.com", "SC Johnson"},
		{"Scrubbing Bubbles", "scrubbingbubbles.com", "SC Johnson"},
		{"Raid", "raid.com", "SC Johnson"},
		{"OFF!", "off.com", "SC Johnson"},
		{"Pledge", "pledge.com", "SC Johnson"},
		// Clorox
		{"Clorox", "clorox.com", "Clorox"},
		{"Glad", "glad.com", "Clorox"},
		{"Pine-Sol", "pinesol.com", "Clorox"},
		{"Burt's Bees", "burtsbees.com", "Clorox"},
		{"Kingsford", "kingsford.com", "Clorox"},
		{"Hidden Valley", "hiddenvalley.com", "Clorox"},
		{"Fresh Step", "freshstep.com", "Clorox"},
		// Energizer
		{"Energizer", "energizer.com", "Energizer"},
		{"Duracell", "duracell.com", "Energizer"},
		{"Schick", "schick.com", "Energizer"},
		// Henkel
		{"Persil", "persil.com", "Henkel"},
		{"Dial", "dialsoap.com", "Henkel"},
		{"Purex", "purex.com", "Henkel"},
		{"all", "all-laundry.com", "Henkel"},
		{"Snuggle", "snuggle.com", "Henkel"},
		// Store brands (no owner)
		{"Kirkland Signature", "costco.com", ""},
		{"Great Value", "walmart.com", ""},
		{"Trader Joe's", "traderjoes.com", ""},
		{"Good & Gather", "target.com", ""},
		{"Market Pantry", "target.com", ""},
		{"365 by Whole Foods", "wholefoodsmarket.com", ""},
		{"Member's Mark", "samsclub.com", ""},
		{"Simple Truth", "kroger.com", ""},
		{"O Organics", "safeway.com", ""},
		{"Open Nature", "safeway.com", ""},
		{"Bowl & Basket", "shoprite.com", ""},
		{"Publix", "publix.com", ""},
		{"H-E-B", "heb.com", ""},
		{"Aldi", "aldi.us", ""},
		// DTC / trending brands (no owner)
		{"Oatly", "oatly.com", ""},
		{"Beyond Meat", "beyondmeat.com", ""},
		{"Impossible Foods", "impossiblefoods.com", ""},
		{"Siete", "sietefoods.com", ""},
		{"Liquid Death", "liquiddeath.com", ""},
		{"Athletic Brewing", "athleticbrewing.com", ""},
		{"Olipop", "drinkolipop.com", ""},
		{"Poppi", "drinkpoppi.com", ""},
		{"Banza", "eatbanza.com", ""},
		{"Chomps", "chomps.com", ""},
		{"RXBAR", "rxbar.com", ""},
		{"Hu Kitchen", "hukitchen.com", ""},
		{"Partake Foods", "partakefoods.com", ""},
		{"Fly by Jing", "flybyjing.com", ""},
		{"Graza", "grfraza.co", ""},
		{"Ithaca Hummus", "ithacahummus.com", ""},
		{"Sir Kensington's", "sirkensingtons.com", ""},
		{"Primal Kitchen", "primalkitchen.com", ""},
		{"Chobani", "chobani.com", ""},
		{"Stonyfield", "stonyfield.com", ""},
		{"Siggi's", "siggis.com", ""},
		{"Justin's", "justins.com", ""},
		{"Late July", "latejuly.com", ""},
		{"Kettle Brand", "kettlebrand.com", ""},
		{"Bob's Red Mill", "bobsredmill.com", ""},
		{"King Arthur", "kingarthurbaking.com", ""},
		{"Dr. Bronner's", "drbronner.com", ""},
		{"Mrs. Meyer's", "mrsmeyers.com", ""},
		{"Method", "methodhome.com", ""},
		{"Grove Co.", "grove.co", ""},
		{"Blueland", "blueland.com", ""},
		{"Stasher", "stasherbag.com", ""},
		{"Yeti", "yeti.com", ""},
		{"Hydro Flask", "hydroflask.com", ""},
		{"S'well", "swell.com", ""},
		{"White Claw", "whiteclaw.com", ""},
		{"Celsius", "celsius.com", ""},
		{"Hint Water", "drinkhint.com", ""},
		{"LaCroix", "lacroixwater.com", ""},
		{"Waterloo", "waterloodrinks.com", ""},
		{"Spindrift", "drinkspindrift.com", ""},
		{"Health-Ade", "health-ade.com", ""},
		{"GT's Kombucha", "gtslivingfoods.com", ""},
		{"Califia Farms", "califiafarms.com", ""},
		{"Nutpods", "nutpods.com", ""},
		{"Ripple", "ripplefoods.com", ""},
		{"StarKist", "starkist.com", ""},
	}

	stmtBrand, err := tx.Prepare("INSERT INTO brands (name, domain, owner_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING")
	if err != nil {
		return err
	}
	defer stmtBrand.Close()

	for _, b := range brands {
		var ownerID interface{}
		if b.owner != "" {
			ownerID = ownerIDs[b.owner]
		}
		if _, err := stmtBrand.Exec(b.name, b.domain, ownerID); err != nil {
			return fmt.Errorf("inserting brand %q: %w", b.name, err)
		}
	}

	return tx.Commit()
}

// backfillItemEmojis re-computes emoji for all list_items using the latest keyword list.
// Runs once at startup -- idempotent (only updates items whose computed emoji differs).
func backfillItemEmojis(db dbExecer) {
	rows, err := db.Query("SELECT id, name, department, emoji FROM list_items")
	if err != nil {
		log.Printf("[emoji-backfill] query error: %v", err)
		return
	}
	defer rows.Close()

	type item struct {
		id         int
		name       string
		department string
		emoji      string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.name, &it.department, &it.emoji); err != nil {
			continue
		}
		items = append(items, it)
	}

	if len(items) == 0 {
		return
	}

	updated := 0
	for _, it := range items {
		computed := inferItemEmoji(it.name, it.department)
		if computed != "" && computed != it.emoji {
			_, err := db.Exec("UPDATE list_items SET emoji = $1 WHERE id = $2", computed, it.id)
			if err == nil {
				updated++
			}
		}
	}
	if updated > 0 {
		log.Printf("[emoji-backfill] refreshed %d items with updated emojis", updated)
	}
}
