package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// dbExecer abstracts *sql.DB and *sql.Tx so functions can run inside
// or outside a transaction.
type dbExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func initDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// For in-memory databases, limit to 1 connection so all operations
	// share the same database instance (each :memory: connection is separate).
	// This also ensures PRAGMA settings apply to all queries.
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		// SQLite only supports one writer at a time. Limiting connections
		// prevents SQLITE_BUSY contention under concurrent requests.
		db.SetMaxOpenConns(1)
	}

	// Enable foreign key enforcement (off by default in SQLite)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		log.Printf("[warn] failed to enable WAL mode: %v", err)
	}
	// NORMAL sync is safe with WAL and much faster than FULL
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		log.Printf("[warn] failed to set synchronous mode: %v", err)
	}
	// Wait up to 5s for locks instead of failing immediately
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		log.Printf("[warn] failed to set busy timeout: %v", err)
	}

	// Create the users table if it doesn't exist.
	// "IF NOT EXISTS" means this is safe to run every time the app starts.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			phone_number TEXT    UNIQUE,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating users table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS list_items (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			name       TEXT    NOT NULL,
			department TEXT    NOT NULL DEFAULT 'Other',
			checked    INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating list_items table: %w", err)
	}

	// Add columns introduced after initial schema (safe to run repeatedly — errors ignored)
	for _, stmt := range []string{
		"ALTER TABLE list_items ADD COLUMN brand TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE list_items ADD COLUMN quantity INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE list_items ADD COLUMN size TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE list_items ADD COLUMN source TEXT NOT NULL DEFAULT 'app'",
		"ALTER TABLE list_items ADD COLUMN urgent INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE list_items ADD COLUMN claimed_by INTEGER DEFAULT NULL",
		"ALTER TABLE list_items ADD COLUMN claimed_at DATETIME DEFAULT NULL",
		"ALTER TABLE list_items ADD COLUMN notes TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE list_items ADD COLUMN swap_savings_cents INTEGER NOT NULL DEFAULT 0",
	} {
		db.Exec(stmt) // "duplicate column name" error is expected and harmless
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_lists (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			name       TEXT    NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating saved_lists table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_list_items (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			saved_list_id INTEGER NOT NULL,
			name          TEXT    NOT NULL,
			brand         TEXT    NOT NULL DEFAULT '',
			department    TEXT    NOT NULL DEFAULT 'Other',
			quantity      INTEGER NOT NULL DEFAULT 1,
			size          TEXT    NOT NULL DEFAULT '',
			FOREIGN KEY (saved_list_id) REFERENCES saved_lists(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating saved_list_items table: %w", err)
	}
	db.Exec("ALTER TABLE saved_list_items ADD COLUMN notes TEXT NOT NULL DEFAULT ''")

	// --- Household tables ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS households (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating households table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS household_members (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			household_id INTEGER NOT NULL,
			user_id      INTEGER NOT NULL UNIQUE,
			role         TEXT NOT NULL DEFAULT 'member',
			invited_by   INTEGER,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating household_members table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS invites (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			household_id INTEGER NOT NULL,
			code         TEXT UNIQUE NOT NULL,
			created_by   INTEGER NOT NULL,
			expires_at   DATETIME NOT NULL,
			used_by      INTEGER,
			used_at      DATETIME,
			FOREIGN KEY (household_id) REFERENCES households(id),
			FOREIGN KEY (created_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating invites table: %w", err)
	}

	// Add columns introduced after initial schema (safe to run repeatedly)
	for _, stmt := range []string{
		"ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN email TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN household_id INTEGER",
		"ALTER TABLE users ADD COLUMN referral_code TEXT",
		"ALTER TABLE users ADD COLUMN referral_points INTEGER DEFAULT 0",
		"ALTER TABLE users ADD COLUMN last_active DATETIME",
		"ALTER TABLE users ADD COLUMN login_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN address TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN onboarding_flags TEXT NOT NULL DEFAULT '{}'",
		"ALTER TABLE users ADD COLUMN referral_source TEXT NOT NULL DEFAULT ''",
		// M2a: dedup flag for join notification SMS
		"ALTER TABLE invites ADD COLUMN notified INTEGER NOT NULL DEFAULT 0",
		// M2c: editable household/list name
		"ALTER TABLE households ADD COLUMN name TEXT NOT NULL DEFAULT ''",
	} {
		db.Exec(stmt)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS item_modifications (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id    INTEGER NOT NULL,
			user_id    INTEGER,
			action     TEXT NOT NULL,
			detail     TEXT,
			source     TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (item_id) REFERENCES list_items(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating item_modifications table: %w", err)
	}

	// Migrate item_modifications to add ON DELETE CASCADE if missing.
	// SQLite doesn't support ALTER TABLE for constraints, so we recreate the table.
	// Check if the existing table has ON DELETE CASCADE by looking at the schema.
	var createSQL string
	db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='item_modifications'").Scan(&createSQL)
	if createSQL != "" && !strings.Contains(strings.ToUpper(createSQL), "ON DELETE CASCADE") {
		tx, err := db.Begin()
		if err != nil {
			return nil, fmt.Errorf("starting item_modifications migration: %w", err)
		}
		migrationErr := func() error {
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS item_modifications_new (
					id         INTEGER PRIMARY KEY AUTOINCREMENT,
					item_id    INTEGER NOT NULL,
					user_id    INTEGER,
					action     TEXT NOT NULL,
					detail     TEXT,
					source     TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (item_id) REFERENCES list_items(id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES users(id)
				)
			`); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT INTO item_modifications_new SELECT * FROM item_modifications"); err != nil {
				return err
			}
			if _, err := tx.Exec("DROP TABLE item_modifications"); err != nil {
				return err
			}
			if _, err := tx.Exec("ALTER TABLE item_modifications_new RENAME TO item_modifications"); err != nil {
				return err
			}
			return nil
		}()
		if migrationErr != nil {
			tx.Rollback()
			return nil, fmt.Errorf("migrating item_modifications: %w", migrationErr)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("committing item_modifications migration: %w", err)
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS referrals (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			referrer_id INTEGER NOT NULL,
			referred_id INTEGER NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (referrer_id) REFERENCES users(id),
			FOREIGN KEY (referred_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating referrals table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS redemptions (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      INTEGER NOT NULL,
			points       INTEGER NOT NULL,
			amount_cents INTEGER NOT NULL,
			status       TEXT DEFAULT 'pending',
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating redemptions table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nps_responses (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			score      INTEGER NOT NULL,
			comment    TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating nps_responses table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token        TEXT PRIMARY KEY,
			user_id      INTEGER NOT NULL,
			phone_number TEXT,
			auth_method  TEXT NOT NULL DEFAULT 'phone',
			created_at   DATETIME NOT NULL,
			expires_at   DATETIME NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating sessions table: %w", err)
	}

	// --- Multi-auth tables ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_methods (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER NOT NULL,
			provider    TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			password_hash TEXT DEFAULT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(provider, provider_id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating auth_methods table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_resets (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL,
			token      TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			used       INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating password_resets table: %w", err)
	}

	// Ensure penny/brand columns exist BEFORE the table recreation migration
	// (the recreation copies these columns, so they must be present first).
	for _, stmt := range []string{
		"ALTER TABLE users ADD COLUMN penny_brand_mode TEXT NOT NULL DEFAULT 'ask'",
		"ALTER TABLE users ADD COLUMN deal_accept_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN deal_shown_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN penny_slider INTEGER NOT NULL DEFAULT 15",
		"ALTER TABLE users ADD COLUMN penny_autotune INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE users ADD COLUMN penny_deals INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN last_cleared_items TEXT NOT NULL DEFAULT '[]'",
	} {
		db.Exec(stmt) // errors ignored — column may already exist
	}

	// Migrate users.phone_number from NOT NULL to nullable.
	// SQLite can't ALTER COLUMN, so we recreate the table if needed.
	var usersSQL string
	db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='users'").Scan(&usersSQL)
	if usersSQL != "" && strings.Contains(usersSQL, "phone_number") && strings.Contains(strings.ToUpper(usersSQL), "NOT NULL") && strings.Contains(usersSQL, "phone_number TEXT") {
		// Check if it's specifically phone_number that has NOT NULL (not another column)
		// The original schema is: phone_number TEXT UNIQUE NOT NULL
		if strings.Contains(usersSQL, "phone_number TEXT") && strings.Contains(usersSQL, "UNIQUE NOT NULL") {
			// Disable FK checks during table recreation (other tables reference users)
			db.Exec("PRAGMA foreign_keys = OFF")
			tx, err := db.Begin()
			if err != nil {
				db.Exec("PRAGMA foreign_keys = ON")
				return nil, fmt.Errorf("starting users migration: %w", err)
			}
			migErr := func() error {
				// Get all column names from the existing table by reading a row
				if _, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS users_new (
						id              INTEGER PRIMARY KEY AUTOINCREMENT,
						phone_number    TEXT    UNIQUE,
						created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
						name            TEXT NOT NULL DEFAULT '',
						email           TEXT DEFAULT '',
						household_id    INTEGER,
						referral_code   TEXT,
						referral_points INTEGER DEFAULT 0,
						last_active     DATETIME,
						login_count     INTEGER NOT NULL DEFAULT 0,
						address         TEXT NOT NULL DEFAULT '',
						penny_brand_mode TEXT NOT NULL DEFAULT 'ask',
						deal_accept_count INTEGER NOT NULL DEFAULT 0,
						deal_shown_count  INTEGER NOT NULL DEFAULT 0,
						penny_slider     INTEGER NOT NULL DEFAULT 15,
						penny_autotune   INTEGER NOT NULL DEFAULT 1,
						penny_deals      INTEGER NOT NULL DEFAULT 0,
						last_cleared_items TEXT NOT NULL DEFAULT '[]',
						swap_credits_cents INTEGER NOT NULL DEFAULT 0
					)
				`); err != nil {
					return err
				}
				if _, err := tx.Exec(`
					INSERT INTO users_new (id, phone_number, created_at, name, email, household_id,
						referral_code, referral_points, last_active, login_count, address,
						penny_brand_mode, deal_accept_count, deal_shown_count, penny_slider,
						penny_autotune, penny_deals, last_cleared_items, swap_credits_cents)
					SELECT id, phone_number, created_at, name, email, household_id,
						referral_code, referral_points, last_active, login_count, address,
						penny_brand_mode, deal_accept_count, deal_shown_count, penny_slider,
						penny_autotune, penny_deals, last_cleared_items, COALESCE(swap_credits_cents, 0)
					FROM users
				`); err != nil {
					return err
				}
				if _, err := tx.Exec("DROP TABLE users"); err != nil {
					return err
				}
				if _, err := tx.Exec("ALTER TABLE users_new RENAME TO users"); err != nil {
					return err
				}
				return nil
			}()
			if migErr != nil {
				tx.Rollback()
				db.Exec("PRAGMA foreign_keys = ON")
				return nil, fmt.Errorf("migrating users table: %w", migErr)
			}
			if err := tx.Commit(); err != nil {
				db.Exec("PRAGMA foreign_keys = ON")
				return nil, fmt.Errorf("committing users migration: %w", err)
			}
			db.Exec("PRAGMA foreign_keys = ON")
			log.Println("[migration] users.phone_number is now nullable")
		}
	}

	// Backfill auth_methods from existing users with phone numbers
	db.Exec(`
		INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id)
		SELECT id, 'phone', phone_number FROM users
		WHERE phone_number IS NOT NULL AND phone_number != ''
	`)

	// Migrate sessions table: add auth_method column, make phone_number nullable
	var sessionsSQL string
	db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&sessionsSQL)
	if sessionsSQL != "" && !strings.Contains(sessionsSQL, "auth_method") {
		tx, err := db.Begin()
		if err != nil {
			return nil, fmt.Errorf("starting sessions migration: %w", err)
		}
		migErr := func() error {
			if _, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS sessions_new (
					token        TEXT PRIMARY KEY,
					user_id      INTEGER NOT NULL,
					phone_number TEXT,
					auth_method  TEXT NOT NULL DEFAULT 'phone',
					created_at   DATETIME NOT NULL,
					expires_at   DATETIME NOT NULL
				)
			`); err != nil {
				return err
			}
			if _, err := tx.Exec(`
				INSERT INTO sessions_new (token, user_id, phone_number, auth_method, created_at, expires_at)
				SELECT token, user_id, phone_number, 'phone', created_at, expires_at FROM sessions
			`); err != nil {
				return err
			}
			if _, err := tx.Exec("DROP TABLE sessions"); err != nil {
				return err
			}
			if _, err := tx.Exec("ALTER TABLE sessions_new RENAME TO sessions"); err != nil {
				return err
			}
			return nil
		}()
		if migErr != nil {
			tx.Rollback()
			return nil, fmt.Errorf("migrating sessions table: %w", migErr)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("committing sessions migration: %w", err)
		}
		log.Println("[migration] sessions table now has auth_method column")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_owners (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_owners table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brands (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT UNIQUE NOT NULL COLLATE NOCASE,
			domain     TEXT NOT NULL,
			owner_id   INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES brand_owners(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brands table: %w", err)
	}

	if err := seedBrands(db); err != nil {
		log.Printf("WARNING: failed to seed brands: %v", err)
	}

	// --- Brand logos (stored as BLOBs to survive Railway's ephemeral filesystem) ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_logos (
			brand_id      INTEGER PRIMARY KEY,
			logo_data     BLOB    NOT NULL,
			content_type  TEXT    NOT NULL DEFAULT 'image/png',
			fetched_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			source        TEXT    NOT NULL DEFAULT 'logodev',
			FOREIGN KEY (brand_id) REFERENCES brands(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_logos table: %w", err)
	}

	// --- Shopping trips ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shopping_trips (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      INTEGER NOT NULL,
			household_id INTEGER NOT NULL,
			trip_date    TEXT    NOT NULL,
			trip_time    TEXT    DEFAULT NULL,
			time_label   TEXT    DEFAULT NULL,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (household_id) REFERENCES households(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating shopping_trips table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_shopping_trips_household_id ON shopping_trips(household_id)")

	// Migrate old department names to new names (safe to run repeatedly — updates 0 rows if already migrated)
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

	// --- Brand preference learning tables ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_choices (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          INTEGER NOT NULL,
			product_category TEXT NOT NULL,
			brand            TEXT NOT NULL,
			source           TEXT NOT NULL DEFAULT 'disambiguation',
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_choices table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_preferences (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          INTEGER NOT NULL,
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
		return nil, fmt.Errorf("creating brand_preferences table: %w", err)
	}

	// Brand preference user columns (safe to run repeatedly — errors ignored)
	for _, stmt := range []string{
		"ALTER TABLE users ADD COLUMN penny_brand_mode TEXT NOT NULL DEFAULT 'ask'",
		"ALTER TABLE users ADD COLUMN deal_accept_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN deal_shown_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN penny_slider INTEGER NOT NULL DEFAULT 15",
		"ALTER TABLE users ADD COLUMN penny_autotune INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE users ADD COLUMN penny_deals INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN last_cleared_items TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE users ADD COLUMN swap_credits_cents INTEGER NOT NULL DEFAULT 0",
	} {
		db.Exec(stmt)
	}

	// =========================================================================
	// PENNY SWAPS TABLES
	// =========================================================================

	// brand_deals: deals seeded by admin, matching competitor products to offers
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_deals (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			competitor_product TEXT    NOT NULL,
			offer_brand        TEXT    NOT NULL,
			offer_product      TEXT    NOT NULL,
			discount_cents     INTEGER NOT NULL DEFAULT 0,
			daily_budget_cents INTEGER NOT NULL DEFAULT 0, -- misnomer: this is cumulative lifetime budget, not daily (B15)
			budget_spent_cents INTEGER NOT NULL DEFAULT 0,
			status             TEXT    NOT NULL DEFAULT 'active',
			starts_at          TEXT    NOT NULL DEFAULT (datetime('now')),
			ends_at            TEXT,
			created_at         TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_deals table: %w", err)
	}

	// swap_events: tracks every swap offer/accept/decline per user
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			deal_id    INTEGER NOT NULL REFERENCES brand_deals(id),
			item_id    INTEGER,
			action     TEXT    NOT NULL,
			created_at TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating swap_events table: %w", err)
	}

	// swap_suppressions: tracks per-user deal dismissals, 3+ = permanent suppress
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_suppressions (
			user_id       INTEGER NOT NULL REFERENCES users(id),
			deal_id       INTEGER NOT NULL REFERENCES brand_deals(id),
			decline_count INTEGER NOT NULL DEFAULT 0,
			UNIQUE(user_id, deal_id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating swap_suppressions table: %w", err)
	}

	// swap_rebates: immutable ledger tracking rebates owed per swap accept
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_rebates (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id        INTEGER NOT NULL REFERENCES users(id),
			swap_event_id  INTEGER NOT NULL REFERENCES swap_events(id),
			deal_id        INTEGER NOT NULL REFERENCES brand_deals(id),
			amount_cents   INTEGER NOT NULL,
			status         TEXT NOT NULL DEFAULT 'pending',
			created_at     TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating swap_rebates table: %w", err)
	}

	// profile_photos: stores user avatar images as BLOBs (one per user)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profile_photos (
			user_id      INTEGER PRIMARY KEY REFERENCES users(id),
			photo_data   BLOB NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'image/jpeg',
			updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating profile_photos table: %w", err)
	}

	// receipts: tracks receipt photo submissions for dedup
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipts (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id       INTEGER NOT NULL REFERENCES users(id),
			image_hash    TEXT    NOT NULL,
			items_found   INTEGER NOT NULL DEFAULT 0,
			items_matched INTEGER NOT NULL DEFAULT 0,
			created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating receipts table: %w", err)
	}

	// shared_links: deep-link share tokens for read-only list sharing
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS shared_links (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			code         TEXT NOT NULL UNIQUE,
			user_id      INTEGER NOT NULL,
			household_id INTEGER,
			invite_code  TEXT,
			expires_at   DATETIME NOT NULL,
			view_count   INTEGER DEFAULT 0,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating shared_links table: %w", err)
	}

	// Indexes for common query patterns (placed after all table creation)
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_household_members_household_id ON household_members(household_id)",
		"CREATE INDEX IF NOT EXISTS idx_household_members_user_id ON household_members(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_list_items_user_id ON list_items(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_lists_user_id ON saved_lists(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_list_items_saved_list_id ON saved_list_items(saved_list_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_referrals_unique ON referrals(referrer_id, referred_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_item_modifications_item_id ON item_modifications(item_id)",
		"CREATE INDEX IF NOT EXISTS idx_item_modifications_created_at ON item_modifications(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_brand_preferences_user_cat ON brand_preferences(user_id, product_category)",
		"CREATE INDEX IF NOT EXISTS idx_brand_choices_user_cat ON brand_choices(user_id, product_category)",
		"CREATE INDEX IF NOT EXISTS idx_auth_methods_user_id ON auth_methods(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_auth_methods_provider ON auth_methods(provider, provider_id)",
		"CREATE INDEX IF NOT EXISTS idx_password_resets_token ON password_resets(token)",
		"CREATE INDEX IF NOT EXISTS idx_list_items_user_checked ON list_items(user_id, checked)",
		"CREATE INDEX IF NOT EXISTS idx_swap_events_user_deal ON swap_events(user_id, deal_id)",
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
	for _, col := range []string{
		"ALTER TABLE brand_deals ADD COLUMN offer_price_cents INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE brand_deals ADD COLUMN offer_size TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN offer_unit_price TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN competitor_price_cents INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE brand_deals ADD COLUMN competitor_size TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN competitor_unit_price TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN unit_label TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN why_bullets TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE brand_deals ADD COLUMN allergen_notes TEXT NOT NULL DEFAULT ''",
	} {
		db.Exec(col)
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
			fetched_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (brand, category, zip)
		)
	`)
	if err != nil {
		log.Printf("[warn] failed to create price_cache table: %v", err)
	}

	// B15: rename daily_budget_cents → total_budget_cents (the column was never daily)
	db.Exec("ALTER TABLE brand_deals ADD COLUMN total_budget_cents INTEGER NOT NULL DEFAULT 0")
	db.Exec("UPDATE brand_deals SET total_budget_cents = daily_budget_cents WHERE total_budget_cents = 0 AND daily_budget_cents != 0")

	// Sprint 3: swap type expansion columns
	for _, col := range []string{
		"ALTER TABLE brand_deals ADD COLUMN deal_type TEXT NOT NULL DEFAULT 'sku_swap'",
		"ALTER TABLE brand_deals ADD COLUMN quantity_offered INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE brand_deals ADD COLUMN trial_product_name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN trial_size TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE brand_deals ADD COLUMN campaign_tag TEXT NOT NULL DEFAULT ''",
	} {
		db.Exec(col)
	}

	if err := seedDeals(db); err != nil {
		log.Printf("WARNING: failed to seed deals: %v", err)
	}

	// A/B testing tables
	if err := initABTables(db); err != nil {
		log.Printf("WARNING: failed to init A/B tables: %v", err)
	}

	// --- Brand Portal tables (M4) ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			company_name  TEXT NOT NULL DEFAULT '',
			verified      INTEGER NOT NULL DEFAULT 0,
			verify_token  TEXT DEFAULT NULL,
			badge         TEXT NOT NULL DEFAULT '',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_users table: %w", err)
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
		return nil, fmt.Errorf("creating brand_sessions table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_sessions_expires ON brand_sessions(expires_at)")

	// Link brand_deals to brand_users (optional — legacy deals have no brand_user_id)
	db.Exec("ALTER TABLE brand_deals ADD COLUMN brand_user_id INTEGER DEFAULT NULL")

	// Brand balance for Stripe pre-funded deposits
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_balance (
			brand_user_id INTEGER PRIMARY KEY,
			balance_cents INTEGER NOT NULL DEFAULT 0,
			last_deposit_at DATETIME DEFAULT NULL,
			FOREIGN KEY (brand_user_id) REFERENCES brand_users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_balance table: %w", err)
	}

	// Brand transactions ledger
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
		return nil, fmt.Errorf("creating brand_transactions table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_brand_transactions_user ON brand_transactions(brand_user_id)")

	// Bug reports table (Help/Bug Report quick win)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bug_reports (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER,
			description TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating bug_reports table: %w", err)
	}

	// User profile photo column
	db.Exec("ALTER TABLE users ADD COLUMN onboarding_flags TEXT NOT NULL DEFAULT '{}'")
	db.Exec("ALTER TABLE users ADD COLUMN profile_photo TEXT NOT NULL DEFAULT ''")

	// --- M7: Receipt Reading Enhancement ---

	// Structured line items extracted from receipts
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS receipt_line_items (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			receipt_id  INTEGER NOT NULL,
			raw_text    TEXT NOT NULL,
			price_cents INTEGER NOT NULL DEFAULT 0,
			quantity    INTEGER NOT NULL DEFAULT 1,
			matched_item_id INTEGER DEFAULT NULL,
			matched_name    TEXT NOT NULL DEFAULT '',
			confidence      TEXT NOT NULL DEFAULT 'low',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (receipt_id) REFERENCES receipts(id) ON DELETE CASCADE,
			FOREIGN KEY (matched_item_id) REFERENCES list_items(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating receipt_line_items table: %w", err)
	}

	// Swap claims: links receipts to accepted swaps for purchase verification
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_claims (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			receipt_id    INTEGER NOT NULL,
			swap_event_id INTEGER NOT NULL,
			deal_id       INTEGER NOT NULL,
			user_id       INTEGER NOT NULL,
			status        TEXT NOT NULL DEFAULT 'pending',
			verified_at   DATETIME DEFAULT NULL,
			payout_held_until DATETIME DEFAULT NULL,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (receipt_id) REFERENCES receipts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating swap_claims table: %w", err)
	}

	// Retailer abbreviation database (flywheel: user confirmations improve matching)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS retailer_abbreviations (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			abbreviation TEXT NOT NULL,
			full_name    TEXT NOT NULL,
			retailer     TEXT NOT NULL DEFAULT '',
			confirmations INTEGER NOT NULL DEFAULT 1,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(abbreviation, retailer)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating retailer_abbreviations table: %w", err)
	}

	// Add structured fields to receipts table
	for _, col := range []string{
		"ALTER TABLE receipts ADD COLUMN store_name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE receipts ADD COLUMN store_address TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE receipts ADD COLUMN transaction_date TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE receipts ADD COLUMN subtotal_cents INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE receipts ADD COLUMN tax_cents INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE receipts ADD COLUMN total_cents INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE receipts ADD COLUMN status TEXT NOT NULL DEFAULT 'processed'",
		"ALTER TABLE receipts ADD COLUMN swaps_verified INTEGER NOT NULL DEFAULT 0",
	} {
		db.Exec(col)
	}

	db.Exec("CREATE INDEX IF NOT EXISTS idx_receipt_line_items_receipt ON receipt_line_items(receipt_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_claims_receipt ON swap_claims(receipt_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_claims_user ON swap_claims(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_retailer_abbreviations_abbr ON retailer_abbreviations(abbreviation)")

	// Earnings: attachment support on swap_rebates
	db.Exec("ALTER TABLE swap_rebates ADD COLUMN attachment_path TEXT NOT NULL DEFAULT ''")

	// savings_milestones: tracks which savings milestones a user has acknowledged
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS savings_milestones (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			cents      INTEGER NOT NULL,
			reached_at TEXT NOT NULL DEFAULT (datetime('now')),
			acked_at   TEXT DEFAULT NULL,
			UNIQUE(user_id, cents)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating savings_milestones table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_savings_milestones_user ON savings_milestones(user_id)")

	// Brand digest preferences
	db.Exec("ALTER TABLE brand_users ADD COLUMN digest_opt_out INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE brand_users ADD COLUMN last_digest_at TEXT DEFAULT NULL")

	// ==========================================================================
	// LEARNING LOOP INFRASTRUCTURE (Sprint 11)
	// ==========================================================================

	// user_signals: captures discrete behavioral events for learning
	// Each row is a signal that teaches Penny about user preferences/behavior
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_signals (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES users(id),
			signal     TEXT NOT NULL,
			category   TEXT NOT NULL DEFAULT '',
			value      TEXT NOT NULL DEFAULT '',
			context    TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating user_signals table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_user ON user_signals(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_signal ON user_signals(signal)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_signals_created ON user_signals(created_at)")

	// penny_learnings: aggregated insights Penny has derived from signals
	// These are the "knowledge" Penny has accumulated per user/household/brand
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_learnings (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			scope      TEXT NOT NULL,
			scope_id   INTEGER NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0.5,
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(scope, scope_id, key)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating penny_learnings table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_penny_learnings_scope ON penny_learnings(scope, scope_id)")

	// store_prices: moved to end of initDB (after stores table creation)

	// --- Dietary profile table ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_dietary_profiles (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER NOT NULL UNIQUE REFERENCES users(id),
			allergens   TEXT NOT NULL DEFAULT '[]',
			diets       TEXT NOT NULL DEFAULT '[]',
			avoid_brands TEXT NOT NULL DEFAULT '[]',
			updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating user_dietary_profiles table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_dietary_user ON user_dietary_profiles(user_id)")

	// penny_context_cache: pre-computed context for Penny prompts
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS penny_context_cache (
			user_id      INTEGER PRIMARY KEY REFERENCES users(id),
			context_json TEXT NOT NULL DEFAULT '{}',
			computed_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating penny_context_cache table: %w", err)
	}

	// swap_events: decline_reason column for tracking why users decline swaps
	db.Exec("ALTER TABLE swap_events ADD COLUMN decline_reason TEXT NOT NULL DEFAULT ''")
	// swap_events: enrichment columns for swap interaction analytics (CEO panel recommendation #1)
	db.Exec("ALTER TABLE swap_events ADD COLUMN time_to_action_ms INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE swap_events ADD COLUMN why_tapped INTEGER NOT NULL DEFAULT 0")

	// ==========================================================================
	// DAILY METRICS AGGREGATION TABLES (deal performance learning)
	// ==========================================================================

	// deal_performance_daily: materialized daily deal metrics, recomputed by batch job
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
		return nil, fmt.Errorf("creating deal_performance_daily table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_deal_perf_daily_deal ON deal_performance_daily(deal_id, date)")

	// category_swap_insights: aggregated swap metrics across deals in a category
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_swap_insights (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			category            TEXT NOT NULL UNIQUE,
			avg_acceptance_rate REAL NOT NULL DEFAULT 0.0,
			avg_discount_pct    REAL NOT NULL DEFAULT 0.0,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			last_computed       TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating category_swap_insights table: %w", err)
	}
	// Add top_decline_reason column for category-level decline reason tracking
	db.Exec("ALTER TABLE category_swap_insights ADD COLUMN top_decline_reason TEXT NOT NULL DEFAULT ''")

	// business_metrics_daily: daily snapshot of key business metrics
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
			computed_at         TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating business_metrics_daily table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_business_metrics_date ON business_metrics_daily(date)")

	// Churn prediction signals
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS churn_signals (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      INTEGER NOT NULL REFERENCES users(id),
			signal_type  TEXT NOT NULL,
			signal_value REAL NOT NULL,
			risk_score   REAL NOT NULL DEFAULT 0.0,
			computed_at  TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, signal_type)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating churn_signals table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_churn_signals_user ON churn_signals(user_id, signal_type)")

	// User cohort retention tracking
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_cohorts (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id             INTEGER NOT NULL UNIQUE REFERENCES users(id),
			cohort_week         TEXT NOT NULL,
			signup_source       TEXT NOT NULL DEFAULT '',
			first_swap_at       TEXT DEFAULT NULL,
			first_receipt_at    TEXT DEFAULT NULL,
			total_swaps         INTEGER NOT NULL DEFAULT 0,
			total_savings_cents INTEGER NOT NULL DEFAULT 0,
			last_active         TEXT DEFAULT NULL,
			is_churned          INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating user_cohorts table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_cohorts_week ON user_cohorts(cohort_week)")

	// Marketplace health index — daily aggregate
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS marketplace_health (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			date                TEXT NOT NULL UNIQUE,
			supply_score        REAL NOT NULL DEFAULT 0.0,
			demand_score        REAL NOT NULL DEFAULT 0.0,
			balance_score       REAL NOT NULL DEFAULT 0.0,
			revenue_concentration REAL NOT NULL DEFAULT 0.0,
			health_index        REAL NOT NULL DEFAULT 0.0,
			computed_at         TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating marketplace_health table: %w", err)
	}

	// --- Category taxonomy ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT NOT NULL UNIQUE,
			parent_category TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating categories table: %w", err)
	}

	if err := seedCategories(db); err != nil {
		log.Printf("WARNING: failed to seed categories: %v", err)
	}

	// Link brand_deals to categories (0 = uncategorized)
	db.Exec("ALTER TABLE brand_deals ADD COLUMN category_id INTEGER NOT NULL DEFAULT 0")

	// v1 Stretch: rich product descriptions for conversational swap explanations
	db.Exec("ALTER TABLE brand_deals ADD COLUMN product_description TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE brand_deals ADD COLUMN key_benefits TEXT NOT NULL DEFAULT '[]'") // JSON array
	db.Exec("ALTER TABLE brand_deals ADD COLUMN brand_story TEXT NOT NULL DEFAULT ''")

	// Swap conversational depth tracking
	db.Exec("ALTER TABLE swap_events ADD COLUMN conversation_depth INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE swap_events ADD COLUMN why_tapped_at TEXT DEFAULT NULL")

	// --- Category pricing tiers ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_pricing_tiers (
			parent_category    TEXT PRIMARY KEY,
			min_price_cents    INTEGER NOT NULL DEFAULT 100,
			suggested_cents    INTEGER NOT NULL DEFAULT 150,
			tier_label         TEXT NOT NULL DEFAULT 'standard'
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating category_pricing_tiers table: %w", err)
	}
	if err := seedPricingTiers(db); err != nil {
		log.Printf("WARNING: failed to seed pricing tiers: %v", err)
	}

	// --- User swap predictions ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_swap_predictions (
			user_id         INTEGER NOT NULL,
			category_id     INTEGER NOT NULL DEFAULT 0,
			brand           TEXT NOT NULL DEFAULT '',
			acceptance_rate REAL NOT NULL DEFAULT 0.0,
			total_offers    INTEGER NOT NULL DEFAULT 0,
			total_accepts   INTEGER NOT NULL DEFAULT 0,
			avg_time_to_action_ms INTEGER NOT NULL DEFAULT 0,
			confidence      REAL NOT NULL DEFAULT 0.0,
			computed_at     TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, category_id, brand)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating user_swap_predictions table: %w", err)
	}

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_user_swap_predictions_user ON user_swap_predictions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_user_swap_predictions_category ON user_swap_predictions(user_id, category_id)",
	} {
		db.Exec(stmt)
	}

	// --- Stores (L4 Location Learning foundation) ---
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
			created_at   TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(name, address)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating stores table: %w", err)
	}

	// Store price observations: track real prices seen at stores
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS store_prices (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			store_id     INTEGER NOT NULL REFERENCES stores(id),
			product_name TEXT NOT NULL,
			brand        TEXT NOT NULL DEFAULT '',
			price_cents  INTEGER NOT NULL,
			category_id  INTEGER NOT NULL DEFAULT 0,
			source       TEXT NOT NULL DEFAULT 'receipt',
			observed_at  TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(store_id, product_name, brand, observed_at)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating store_prices table: %w", err)
	}

	// Link receipts to stores
	db.Exec("ALTER TABLE receipts ADD COLUMN store_id INTEGER DEFAULT NULL")

	// Link shopping trips to stores
	db.Exec("ALTER TABLE shopping_trips ADD COLUMN store_id INTEGER DEFAULT NULL")

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_stores_chain ON stores(chain)",
		"CREATE INDEX IF NOT EXISTS idx_stores_place_id ON stores(place_id)",
		"CREATE INDEX IF NOT EXISTS idx_store_prices_store ON store_prices(store_id)",
		"CREATE INDEX IF NOT EXISTS idx_store_prices_product ON store_prices(product_name, brand)",
	} {
		db.Exec(stmt)
	}

	// --- Swap propagation tracking (organic adoption by household members) ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS swap_propagations (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			deal_id         INTEGER NOT NULL REFERENCES brand_deals(id),
			source_user_id  INTEGER NOT NULL,
			adopter_user_id INTEGER NOT NULL,
			household_id    INTEGER NOT NULL,
			item_name       TEXT NOT NULL,
			brand           TEXT NOT NULL,
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(deal_id, adopter_user_id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating swap_propagations table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_swap_propagations_deal ON swap_propagations(deal_id)")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS brand_subscriptions (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			brand_user_id  INTEGER NOT NULL,
			tier           TEXT NOT NULL DEFAULT 'free',
			started_at     TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at     TEXT,
			UNIQUE(brand_user_id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating brand_subscriptions table: %w", err)
	}

	// --- Category alerts (disruption signals) ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_alerts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type  TEXT NOT NULL,
			brand       TEXT NOT NULL DEFAULT '',
			category_id INTEGER NOT NULL DEFAULT 0,
			severity    TEXT NOT NULL DEFAULT 'low',
			message     TEXT NOT NULL,
			data_json   TEXT NOT NULL DEFAULT '{}',
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating category_alerts table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_category_alerts_type ON category_alerts(alert_type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_category_alerts_category ON category_alerts(category_id)")

	// --- Feedback system ---
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feedback_requests (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          INTEGER NOT NULL REFERENCES users(id),
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
		return nil, fmt.Errorf("creating feedback_requests table: %w", err)
	}
	db.Exec("CREATE INDEX IF NOT EXISTS idx_feedback_requests_status ON feedback_requests(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_feedback_requests_user_id ON feedback_requests(user_id)")

	return db, nil
}

// seedCategories populates the categories table with common grocery
// subcategories organized under parent groups. Idempotent — only inserts
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

	stmt, err := tx.Prepare("INSERT INTO categories (name, parent_category) VALUES (?, ?)")
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
func categorizeProduct(db *sql.DB, productName string) int {
	if productName == "" {
		return 0
	}
	var id int
	// Try exact-ish match first: category name appears in product name
	err := db.QueryRow(
		"SELECT id FROM categories WHERE LOWER(?) LIKE '%' || LOWER(name) || '%' ORDER BY LENGTH(name) DESC LIMIT 1",
		productName,
	).Scan(&id)
	if err == nil {
		return id
	}
	return 0
}

// seedPricingTiers populates default pricing tiers based on category acceptance
// rates. Higher-value categories (Dairy, Beverages) cost more per swap.
// Idempotent — only inserts when table is empty.
func seedPricingTiers(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM category_pricing_tiers").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	type tier struct {
		parent    string
		minCents  int
		sugCents  int
		label     string
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
			"INSERT INTO category_pricing_tiers (parent_category, min_price_cents, suggested_cents, tier_label) VALUES (?, ?, ?, ?)",
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
func getCategoryPricingTier(db *sql.DB, categoryID int) (minCents int, suggestedCents int, tierLabel string) {
	if categoryID == 0 {
		return 100, 150, "economy"
	}
	var parent string
	if err := db.QueryRow("SELECT parent_category FROM categories WHERE id = ?", categoryID).Scan(&parent); err != nil {
		return 100, 150, "economy"
	}
	if err := db.QueryRow(
		"SELECT min_price_cents, suggested_cents, tier_label FROM category_pricing_tiers WHERE parent_category = ?",
		parent,
	).Scan(&minCents, &suggestedCents, &tierLabel); err != nil {
		return 100, 150, "economy"
	}
	return
}

// seedDeals populates the brand_deals table with test swap deals if empty.
// Idempotent — only inserts when no deals exist.
func seedDeals(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM brand_deals").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// $20 total budget ($2000 cents) split across 5 brand-level deals.
	// Prices are realistic Walmart averages; will be overridden by BlueCart API
	// when BLUECART_API_KEY is set and user has a zip code.
	deals := []struct {
		competitor, offerBrand, offerProduct       string
		discountCents, budgetCents                 int
		offerPriceCents, competitorPriceCents       int
		offerSize, competitorSize                   string
		offerUnitPrice, competitorUnitPrice         string
		unitLabel, whyBullets, allergenNotes        string
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
			 VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.competitor, d.offerBrand, d.offerProduct, d.discountCents, d.budgetCents,
			d.offerPriceCents, d.competitorPriceCents, d.offerSize, d.competitorSize,
			d.offerUnitPrice, d.competitorUnitPrice, d.unitLabel, d.whyBullets, d.allergenNotes,
		); err != nil {
			return fmt.Errorf("seeding deal %s→%s: %w", d.competitor, d.offerBrand, err)
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
			time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		},
	}
	for _, d := range newTypeDeals {
		if _, err := db.Exec(
			`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
			 discount_cents, total_budget_cents, status,
			 offer_price_cents, competitor_price_cents, offer_size, competitor_size,
			 offer_unit_price, competitor_unit_price, unit_label, why_bullets, allergen_notes,
			 deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag, ends_at)
			 VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))`,
			d.competitor, d.offerBrand, d.offerProduct, d.discountCents, d.budgetCents,
			d.offerPriceCents, d.competitorPriceCents, d.offerSize, d.competitorSize,
			d.offerUnitPrice, d.competitorUnitPrice, d.unitLabel, d.whyBullets, d.allergenNotes,
			d.dealType, d.quantityOffered, d.trialProductName, d.trialSize, d.campaignTag, d.endsAt,
		); err != nil {
			return fmt.Errorf("seeding %s deal %s→%s: %w", d.dealType, d.competitor, d.offerBrand, err)
		}
	}
	log.Printf("[deals] seeded %d new-type deals (bogo, trial, seasonal)", len(newTypeDeals))

	return nil
}

// seedBrands populates the brands and brand_owners tables if they are empty.
// This is idempotent — it only inserts when the brands table has no rows.
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
		"Procter & Gamble", "Unilever", "Nestlé", "PepsiCo", "Coca-Cola",
		"General Mills", "Kellogg's", "Kraft Heinz", "Colgate-Palmolive",
		"Johnson & Johnson", "Mondelez", "Mars", "Danone", "Conagra",
		"Campbell Soup", "Hormel", "Tyson", "Smucker's", "Church & Dwight",
		"Reckitt", "SC Johnson", "Clorox", "Energizer", "Henkel",
	}
	ownerIDs := make(map[string]int64)
	for _, name := range owners {
		res, err := tx.Exec("INSERT INTO brand_owners (name) VALUES (?)", name)
		if err != nil {
			return fmt.Errorf("inserting owner %q: %w", name, err)
		}
		id, _ := res.LastInsertId()
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
		{"TRESemmé", "tresemme.com", "Unilever"},
		{"Seventh Generation", "seventhgeneration.com", "Unilever"},
		{"Suave", "suave.com", "Unilever"},
		{"Magnum", "magnumicecream.com", "Unilever"},
		// Nestlé
		{"Nescafé", "nescafe.com", "Nestlé"},
		{"Gerber", "gerber.com", "Nestlé"},
		{"Stouffer's", "stouffers.com", "Nestlé"},
		{"Hot Pockets", "hotpockets.com", "Nestlé"},
		{"DiGiorno", "digiorno.com", "Nestlé"},
		{"Coffee Mate", "coffeemate.com", "Nestlé"},
		{"Lean Cuisine", "leancuisine.com", "Nestlé"},
		{"Toll House", "verybestbaking.com", "Nestlé"},
		{"Purina", "purina.com", "Nestlé"},
		{"Fancy Feast", "fancyfeast.com", "Nestlé"},
		{"Häagen-Dazs", "haagendazs.com", "Nestlé"},
		{"San Pellegrino", "sanpellegrino.com", "Nestlé"},
		{"Perrier", "perrier.com", "Nestlé"},
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
		{"Lärabar", "larabar.com", "General Mills"},
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
		{"Café Bustelo", "cafebustelo.com", "Smucker's"},
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
		{"Spindrift", "drinkspindrift.com", ""},
		{"Health-Ade", "health-ade.com", ""},
		{"GT's Kombucha", "gtslivingfoods.com", ""},
		{"Califia Farms", "califiafarms.com", ""},
		{"Nutpods", "nutpods.com", ""},
		{"Ripple", "ripplefoods.com", ""},
		{"StarKist", "starkist.com", ""},
	}

	stmtBrand, err := tx.Prepare("INSERT INTO brands (name, domain, owner_id) VALUES (?, ?, ?)")
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
			// Skip duplicates (e.g. RXBAR appears twice)
			if strings.Contains(err.Error(), "UNIQUE") {
				continue
			}
			return fmt.Errorf("inserting brand %q: %w", b.name, err)
		}
	}

	return tx.Commit()
}

func recordModification(ex dbExecer, itemID, userID int, action, detail, source string) {
	_, err := ex.Exec(
		"INSERT INTO item_modifications (item_id, user_id, action, detail, source) VALUES (?, ?, ?, ?, ?)",
		itemID, userID, action, detail, source,
	)
	if err != nil {
		log.Printf("[mod] error recording modification for item %d: %v", itemID, err)
	}
}

// getGlobalBrandDefault returns the most-chosen brand for a product category
// across all users. Used as a smart default when a new user has no preference.
// Returns ("", false) if no data or no clear winner (top brand needs ≥2 picks
// and ≥40% of total to be a meaningful default).
// If userDiets are provided, tries dietary-matched users first before falling
// back to general popularity.
func getGlobalBrandDefault(db *sql.DB, category string, userDiets ...[]string) (string, bool) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "", false
	}

	// Try dietary-matched defaults first if user has dietary preferences
	if len(userDiets) > 0 && len(userDiets[0]) > 0 {
		if brand, ok := getDietaryBrandDefault(db, category, userDiets[0]); ok {
			return brand, true
		}
		// Fall through to general popularity
	}

	rows, err := db.Query(`
		SELECT brand, COUNT(*) as cnt
		FROM brand_choices
		WHERE product_category = ? AND brand != ''
		GROUP BY brand
		ORDER BY cnt DESC
		LIMIT 2`, category)
	if err != nil {
		return "", false
	}
	defer rows.Close()

	var topBrand string
	var topCount, secondCount int
	i := 0
	for rows.Next() {
		var brand string
		var cnt int
		if err := rows.Scan(&brand, &cnt); err != nil {
			continue
		}
		if i == 0 {
			topBrand = brand
			topCount = cnt
		} else {
			secondCount = cnt
		}
		i++
	}
	if topBrand == "" || topCount < 2 {
		return "", false
	}
	// Require ≥40% share (top must be meaningfully dominant)
	total := topCount + secondCount
	if total > 0 && float64(topCount)/float64(total) < 0.4 {
		return "", false
	}
	return topBrand, true
}

// getUserDiets returns the user's dietary preferences from user_dietary_profiles.
// Returns nil if no profile exists.
func getUserDiets(db *sql.DB, userID int) []string {
	var dietsJSON string
	err := db.QueryRow("SELECT COALESCE(diets, '[]') FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&dietsJSON)
	if err != nil {
		return nil
	}
	var diets []string
	if err := json.Unmarshal([]byte(dietsJSON), &diets); err != nil || len(diets) == 0 {
		return nil
	}
	return diets
}

// getDietaryBrandDefault returns the most popular brand for a category
// among users who share at least one dietary preference with the requesting user.
// Same thresholds as getGlobalBrandDefault (≥2 picks, ≥40% share).
func getDietaryBrandDefault(db *sql.DB, category string, diets []string) (string, bool) {
	if len(diets) == 0 {
		return "", false
	}
	// Build LIKE conditions for each diet in the user's profile
	// This matches users whose diets JSON array contains any of the same values
	args := make([]interface{}, 0, len(diets)+1)
	args = append(args, category)
	likes := make([]string, len(diets))
	for i, diet := range diets {
		likes[i] = "dp.diets LIKE ?"
		args = append(args, "%"+strings.ToLower(diet)+"%")
	}

	query := fmt.Sprintf(`
		SELECT bc.brand, COUNT(*) as cnt
		FROM brand_choices bc
		JOIN user_dietary_profiles dp ON dp.user_id = bc.user_id
		WHERE bc.product_category = ? AND bc.brand != ''
		AND (%s)
		GROUP BY bc.brand
		ORDER BY cnt DESC
		LIMIT 2`, strings.Join(likes, " OR "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return "", false
	}
	defer rows.Close()

	var topBrand string
	var topCount, secondCount int
	i := 0
	for rows.Next() {
		var brand string
		var cnt int
		if err := rows.Scan(&brand, &cnt); err != nil {
			continue
		}
		if i == 0 {
			topBrand = brand
			topCount = cnt
		} else {
			secondCount = cnt
		}
		i++
	}
	if topBrand == "" || topCount < 2 {
		return "", false
	}
	total := topCount + secondCount
	if total > 0 && float64(topCount)/float64(total) < 0.4 {
		return "", false
	}
	return topBrand, true
}

// recordBrandChoice logs a brand choice and updates the derived preference.
func recordBrandChoice(db *sql.DB, userID int, category, brand, source string) {
	category = strings.ToLower(strings.TrimSpace(category))
	brand = strings.TrimSpace(brand)
	if category == "" || brand == "" {
		return
	}

	// Append-only log
	if _, err := db.Exec("INSERT INTO brand_choices (user_id, product_category, brand, source) VALUES (?, ?, ?, ?)",
		userID, category, brand, source); err != nil {
		log.Printf("[warn] brand choice log insert failed: %v", err)
	}

	// Check existing preference
	var existingBrand string
	var streak int
	err := db.QueryRow("SELECT brand, streak FROM brand_preferences WHERE user_id = ? AND product_category = ?",
		userID, category).Scan(&existingBrand, &streak)

	if err != nil {
		// No existing preference — insert
		if _, err := db.Exec(`INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked)
			VALUES (?, ?, ?, 1, 1, CURRENT_TIMESTAMP)`, userID, category, brand); err != nil {
			log.Printf("[warn] brand preference insert failed: %v", err)
		}
		return
	}

	if strings.EqualFold(existingBrand, brand) {
		// Same brand — increment streak
		if _, err := db.Exec(`UPDATE brand_preferences SET streak = streak + 1, pick_count = pick_count + 1,
			last_picked = CURRENT_TIMESTAMP WHERE user_id = ? AND product_category = ?`,
			userID, category); err != nil {
			log.Printf("[warn] brand preference update failed: %v", err)
		}
	} else {
		// Different brand — reset streak
		if _, err := db.Exec(`UPDATE brand_preferences SET brand = ?, streak = 1, pick_count = pick_count + 1,
			last_picked = CURRENT_TIMESTAMP WHERE user_id = ? AND product_category = ?`,
			brand, userID, category); err != nil {
			log.Printf("[warn] brand preference update failed: %v", err)
		}
	}
}

// resolveBrandAlternatives applies brand preferences to parsed items with alternatives.
// Returns disambiguations the user still needs to resolve.
// When useGlobalDefaults is true, new users with no preference get the most popular brand auto-assigned.
func resolveBrandAlternatives(db *sql.DB, userID int, pennyBrandMode string, items []ListItem, parsedItems []ParsedItem, useGlobalDefaults ...bool) []BrandDisambiguation {
	globalDefaults := len(useGlobalDefaults) > 0 && useGlobalDefaults[0]
	var disambiguations []BrandDisambiguation

	// Build a lookup from parsed item name to added item (handles skipped items)
	itemByName := make(map[string]ListItem)
	for _, item := range items {
		itemByName[strings.ToLower(item.Name)] = item
	}

	for _, p := range parsedItems {
		if len(p.BrandAlternatives) == 0 {
			continue
		}

		item, ok := itemByName[strings.ToLower(p.Name)]
		if !ok {
			continue
		}
		// Use product name as category key (more specific than department)
		category := strings.ToLower(strings.TrimSpace(p.Name))
		if category == "" {
			category = strings.ToLower(strings.TrimSpace(p.Department))
		}

		// Query preference
		var prefBrand string
		var streak int
		var lastPicked string
		err := db.QueryRow(
			"SELECT brand, streak, last_picked FROM brand_preferences WHERE user_id = ? AND product_category = ?",
			userID, category,
		).Scan(&prefBrand, &streak, &lastPicked)

		hasPref := err == nil
		isStale := false
		if hasPref {
			if lp, err := time.Parse("2006-01-02 15:04:05", lastPicked); err == nil {
				isStale = time.Since(lp) > 90*24*time.Hour && streak < 5
			}
		}
		if isStale {
			hasPref = false
		}

		// Strong preference (streak >= 3): auto-apply in all modes
		if hasPref && streak >= 3 {
			db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", prefBrand, item.ID)
			continue
		}

		// Preference exists + auto mode: use last pick
		if hasPref && pennyBrandMode == "auto" {
			db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", prefBrand, item.ID)
			continue
		}

		// Check global brand popularity as fallback for new users (A/B gated)
		if !hasPref && globalDefaults {
			userDiets := getUserDiets(db, userID)
			if globalBrand, ok := getGlobalBrandDefault(db, category, userDiets); ok {
				// Auto-assign the popular brand and send as auto_assigned so
				// the frontend can show 1-tap correction instead of full prompt
				db.Exec("UPDATE list_items SET brand = ? WHERE id = ?", globalBrand, item.ID)
				alts := make([]string, len(p.BrandAlternatives))
				copy(alts, p.BrandAlternatives)
				disambiguations = append(disambiguations, BrandDisambiguation{
					ItemID:          item.ID,
					ProductCategory: category,
					CurrentBrand:    globalBrand,
					Alternatives:    alts,
					AutoAssigned:    true,
				})
				continue
			}
		}

		// Build disambiguation — reorder with preferred brand first if exists
		alts := make([]string, len(p.BrandAlternatives))
		copy(alts, p.BrandAlternatives)
		if hasPref {
			// Move preferred brand to front
			for j, a := range alts {
				if strings.EqualFold(a, prefBrand) {
					alts[0], alts[j] = alts[j], alts[0]
					break
				}
			}
		}

		disambiguations = append(disambiguations, BrandDisambiguation{
			ItemID:          item.ID,
			ProductCategory: category,
			CurrentBrand:    p.Brand,
			Alternatives:    alts,
		})
	}

	return disambiguations
}

func upsertUser(db *sql.DB, phone string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT OR IGNORE INTO users (phone_number) VALUES (?)",
		phone,
	)
	if err != nil {
		return User{}, fmt.Errorf("upserting user: %w", err)
	}

	var user User
	var householdID sql.NullInt64
	var name, email sql.NullString
	err = tx.QueryRow(
		"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE phone_number = ?",
		phone,
	).Scan(&user.ID, &user.PhoneNumber, &name, &email, &householdID, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user: %w", err)
	}
	user.Name = name.String
	user.Email = email.String
	if householdID.Valid {
		user.HouseholdID = int(householdID.Int64)
	}

	// Track login activity and increment login count
	if _, err := tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID); err != nil {
		log.Printf("[warn] failed to update login count for user %d: %v", user.ID, err)
	}

	// Auto-create household if user doesn't have one
	if !householdID.Valid {
		hID, err := createSoloHouseholdTx(tx, user.ID)
		if err != nil {
			return User{}, err
		}
		user.HouseholdID = hID
	}

	// Ensure auth_methods row exists for phone login
	tx.Exec("INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)", user.ID, phone)

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing upsert: %w", err)
	}

	return user, nil
}

// createSoloHouseholdTx creates a new household with the given user as owner
// using the provided transaction or db handle.
func createSoloHouseholdTx(ex dbExecer, userID int) (int, error) {
	result, err := ex.Exec("INSERT INTO households DEFAULT VALUES")
	if err != nil {
		return 0, fmt.Errorf("creating household: %w", err)
	}
	hID, _ := result.LastInsertId()

	_, err = ex.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID)
	if err != nil {
		return 0, fmt.Errorf("setting household_id: %w", err)
	}

	_, err = ex.Exec(
		"INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'owner')",
		hID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting household member: %w", err)
	}

	return int(hID), nil
}

// createSoloHousehold wraps createSoloHouseholdTx in a transaction.
func createSoloHousehold(db *sql.DB, userID int) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	hID, err := createSoloHouseholdTx(tx, userID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing household creation: %w", err)
	}
	return hID, nil
}

// resolveOrCreateUser finds or creates a user for the given auth provider.
// 1. Check auth_methods for existing (provider, provider_id) → return that user
// 2. If email provided, check users for matching email → link auth method (auto-merge)
// 3. Otherwise: create new user + auth_methods row + solo household
func resolveOrCreateUser(db *sql.DB, provider, providerID, email, name string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Check if this auth method already exists
	var userID int
	err = tx.QueryRow(
		"SELECT user_id FROM auth_methods WHERE provider = ? AND provider_id = ?",
		provider, providerID,
	).Scan(&userID)
	if err == nil {
		// Auth method exists — fetch and return the user
		var user User
		var householdID sql.NullInt64
		var uName, uEmail, uPhone sql.NullString
		err = tx.QueryRow(
			"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
			userID,
		).Scan(&user.ID, &uPhone, &uName, &uEmail, &householdID, &user.CreatedAt)
		if err != nil {
			return User{}, fmt.Errorf("fetching user: %w", err)
		}
		user.PhoneNumber = uPhone.String
		user.Name = uName.String
		user.Email = uEmail.String
		if householdID.Valid {
			user.HouseholdID = int(householdID.Int64)
		}
		// Track login
		tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID)
		if err := tx.Commit(); err != nil {
			return User{}, fmt.Errorf("committing: %w", err)
		}
		return user, nil
	}

	// 2. If email provided, try to merge with existing user by email
	if email != "" {
		var existingID int
		err = tx.QueryRow(
			"SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND email != ''",
			email,
		).Scan(&existingID)
		if err == nil {
			// Found existing user with same email — link this auth method
			_, err = tx.Exec(
				"INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, ?, ?)",
				existingID, provider, providerID,
			)
			if err != nil {
				return User{}, fmt.Errorf("linking auth method: %w", err)
			}

			// Update name if empty
			if name != "" {
				tx.Exec("UPDATE users SET name = ? WHERE id = ? AND (name IS NULL OR name = '')", name, existingID)
			}

			var user User
			var householdID sql.NullInt64
			var uName, uEmail, uPhone sql.NullString
			err = tx.QueryRow(
				"SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
				existingID,
			).Scan(&user.ID, &uPhone, &uName, &uEmail, &householdID, &user.CreatedAt)
			if err != nil {
				return User{}, fmt.Errorf("fetching merged user: %w", err)
			}
			user.PhoneNumber = uPhone.String
			user.Name = uName.String
			user.Email = uEmail.String
			if householdID.Valid {
				user.HouseholdID = int(householdID.Int64)
			}
			tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID)
			if err := tx.Commit(); err != nil {
				return User{}, fmt.Errorf("committing: %w", err)
			}
			return user, nil
		}
	}

	// 3. Create new user
	result, err := tx.Exec(
		"INSERT INTO users (phone_number, name, email) VALUES (NULL, ?, ?)",
		name, email,
	)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	newID, _ := result.LastInsertId()

	// Create auth method
	_, err = tx.Exec(
		"INSERT INTO auth_methods (user_id, provider, provider_id) VALUES (?, ?, ?)",
		newID, provider, providerID,
	)
	if err != nil {
		return User{}, fmt.Errorf("creating auth method: %w", err)
	}

	// Auto-create household
	hID, err := createSoloHouseholdTx(tx, int(newID))
	if err != nil {
		return User{}, err
	}

	tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = 1 WHERE id = ?", newID)

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing: %w", err)
	}

	return User{
		ID:          int(newID),
		Name:        name,
		Email:       email,
		HouseholdID: hID,
	}, nil
}

func getHouseholdUserIDs(db *sql.DB, userID int) ([]int, error) {
	rows, err := db.Query(
		"SELECT user_id FROM household_members WHERE household_id = (SELECT household_id FROM users WHERE id = ? AND household_id IS NOT NULL)",
		userID,
	)
	if err != nil {
		log.Printf("[warn] household lookup failed for user %d, falling back to single user: %v", userID, err)
		return []int{userID}, nil
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return []int{userID}, nil
	}
	return ids, nil
}

// householdItemsWhereClause returns a SQL fragment "user_id IN (?,?,...)" and
// the corresponding args, for queries that should span the whole household.
// Pass optional pre-computed household IDs to avoid re-querying.
func householdItemsWhereClause(db *sql.DB, userID int, cachedIDs ...[]int) (string, []interface{}) {
	var ids []int
	if len(cachedIDs) > 0 && cachedIDs[0] != nil {
		ids = cachedIDs[0]
	} else {
		ids, _ = getHouseholdUserIDs(db, userID) // error logged inside, falls back to [userID]
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return "user_id IN (" + strings.Join(placeholders, ",") + ")", args
}

// insertSavedListItems inserts items into saved_list_items within a transaction.
func insertSavedListItems(tx *sql.Tx, savedListID int64, items []itemData) error {
	for _, d := range items {
		if _, err := tx.Exec(
			"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size, notes) VALUES (?, ?, ?, ?, ?, ?, ?)",
			savedListID, d.Name, d.Brand, d.Department, d.Quantity, d.Size, d.Notes,
		); err != nil {
			return err
		}
	}
	return nil
}
func scanListItem(scanner interface{ Scan(dest ...interface{}) error }) (ListItem, error) {
	var item ListItem
	var checked, urgent int
	var addedByName, addedByPhone sql.NullString
	var claimedByID sql.NullInt64
	var claimedByName sql.NullString
	err := scanner.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Notes, &item.Source, &checked, &urgent, &item.CreatedAt, &addedByName, &addedByPhone, &claimedByID, &claimedByName, &item.SwapSavingsCents)
	if err != nil {
		return ListItem{}, err
	}
	item.Checked = checked == 1
	item.Urgent = urgent == 1
	item.AddedByName = addedByName.String
	item.AddedByPhone = addedByPhone.String
	if claimedByID.Valid {
		id := int(claimedByID.Int64)
		item.ClaimedByID = &id
		item.ClaimedByName = claimedByName.String
	}
	return item, nil
}

// maskPhone masks a phone number: "3101234567" → "310***567"
func maskPhone(phone string) string {
	if len(phone) == 10 {
		return phone[:3] + "***" + phone[7:]
	}
	// E.164 format like +15551234567
	if len(phone) >= 12 && phone[0] == '+' {
		return phone[:5] + "***" + phone[8:]
	}
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:2] + "***" + phone[len(phone)-2:]
}

// getItemByID fetches a single list item belonging to a household.
func getItemByID(db *sql.DB, id, userID int) (ListItem, error) {
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	args := append([]interface{}{id}, whereArgs...)
	row := db.QueryRow(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li.id = ? AND li."+whereClause,
		args...,
	)
	return scanListItem(row)
}

// upsertListItem adds a new item or merges into an existing unchecked item
// with the same name (case-insensitive) across the household. Returns the
// resulting ListItem and whether it was newly created.
// The SELECT+UPDATE/INSERT is wrapped in a transaction to prevent duplicate
// items from concurrent requests.
func upsertListItem(db *sql.DB, userID int, name, brand, department, size string, quantity int, source, notes string) (ListItem, bool, error) {
	if quantity < 1 {
		quantity = 1
	}
	if department == "" {
		department = "Other"
	}
	if source == "" {
		source = "app"
	}

	// Get household user IDs before the transaction (uses its own query)
	whereClause, whereArgs := householdItemsWhereClause(db, userID)

	tx, err := db.Begin()
	if err != nil {
		return ListItem{}, false, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Look for an existing item with the same name across household (checked or unchecked)
	queryArgs := append(append([]interface{}{}, whereArgs...), name)
	var existingID int
	var existingQty int
	var existingBrand, existingSize, existingNotes string
	var existingChecked int
	err = tx.QueryRow(
		"SELECT id, quantity, brand, size, checked, COALESCE(notes, '') FROM list_items WHERE "+whereClause+" AND LOWER(name) = LOWER(?)",
		queryArgs...,
	).Scan(&existingID, &existingQty, &existingBrand, &existingSize, &existingChecked, &existingNotes)

	var itemID int
	var isNew bool

	if err == nil {
		if existingChecked == 1 {
			// Re-adding a checked item — uncheck it and set new quantity
			newBrand := brand
			if newBrand == "" {
				newBrand = existingBrand
			}
			newSize := size
			if newSize == "" {
				newSize = existingSize
			}
			_, err := tx.Exec(
				"UPDATE list_items SET checked = 0, quantity = ?, brand = ?, size = ?, notes = ? WHERE id = ?",
				quantity, newBrand, newSize, notes, existingID,
			)
			if err != nil {
				return ListItem{}, false, fmt.Errorf("updating existing item: %w", err)
			}
			recordModification(tx, existingID, userID, "re-added", "", source)
			itemID = existingID
			isNew = true
		} else {
			// Existing unchecked item — merge quantity
			newQty := existingQty + quantity
			newBrand := existingBrand
			if newBrand == "" && brand != "" {
				newBrand = brand
			}
			newSize := existingSize
			if newSize == "" && size != "" {
				newSize = size
			}
			mergeNotes := notes
			if mergeNotes == "" {
				mergeNotes = existingNotes
			}
			_, err := tx.Exec(
				"UPDATE list_items SET quantity = ?, brand = ?, size = ?, notes = ? WHERE id = ?",
				newQty, newBrand, newSize, mergeNotes, existingID,
			)
			if err != nil {
				return ListItem{}, false, fmt.Errorf("updating existing item: %w", err)
			}
			detail := fmt.Sprintf("quantity %d → %d", existingQty, newQty)
			recordModification(tx, existingID, userID, "quantity_changed", detail, source)
			itemID = existingID
			// For SMS/email sources, report as "added" (not skipped) since the user
			// can't interactively dedupe via one-way communication.
			isNew = source == "sms" || source == "email"
		}
	} else {
		// No match — insert new (attributed to the requesting user)
		result, err := tx.Exec(
			"INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			userID, name, brand, department, quantity, size, source, notes,
		)
		if err != nil {
			return ListItem{}, false, fmt.Errorf("inserting item: %w", err)
		}
		id, _ := result.LastInsertId()
		recordModification(tx, int(id), userID, "added", "", source)
		itemID = int(id)
		isNew = true
	}

	if err := tx.Commit(); err != nil {
		return ListItem{}, false, fmt.Errorf("committing upsert: %w", err)
	}

	item, err := getItemByID(db, itemID, userID)
	return item, isNew, err
}

// stripBrandFromName removes the brand name from the item name if it appears
// as a prefix (case-insensitive). e.g. "Budweiser Beer" with brand "Budweiser" → "Beer".
func stripBrandFromName(name, brand string) string {
	if brand == "" {
		return name
	}
	lower := strings.ToLower(name)
	brandLower := strings.ToLower(brand)
	if strings.HasPrefix(lower, brandLower+" ") {
		stripped := strings.TrimSpace(name[len(brand)+1:])
		if stripped != "" {
			return stripped
		}
	}
	if strings.HasSuffix(lower, " "+brandLower) {
		stripped := strings.TrimSpace(name[:len(name)-len(brand)-1])
		if stripped != "" {
			return stripped
		}
	}
	return name
}

// addParsedItems loops over parsed items, skips empties, and upserts each one.
// Returns the list of added items and skipped items (already on the list).
func addParsedItems(db *sql.DB, userID int, parsed []ParsedItem, source string) (added []ListItem, skipped []SkippedItem) {
	for _, p := range parsed {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		p.Name = stripBrandFromName(p.Name, p.Brand)
		item, isNew, err := upsertListItem(db, userID, p.Name, p.Brand, p.Department, p.Size, p.Quantity, source, p.Notes)
		if err != nil {
			log.Printf("[%s] error upserting item %q: %v", source, p.Name, err)
			continue
		}
		if isNew {
			added = append(added, item)
		} else {
			skipped = append(skipped, SkippedItem{Parsed: p, ExistingID: item.ID})
		}
	}
	return added, skipped
}
func collectListItems(db *sql.DB, userID int) ([]itemData, error) {
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	rows, err := db.Query(
		"SELECT name, brand, department, quantity, size, notes FROM list_items WHERE "+whereClause,
		whereArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []itemData
	for rows.Next() {
		var d itemData
		if err := rows.Scan(&d.Name, &d.Brand, &d.Department, &d.Quantity, &d.Size, &d.Notes); err != nil {
			continue
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return items, fmt.Errorf("row iteration: %w", err)
	}
	return items, nil
}
