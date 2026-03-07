package main

import (
	"database/sql"
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
						penny_slider     INTEGER NOT NULL DEFAULT 35,
						penny_autotune   INTEGER NOT NULL DEFAULT 1,
						penny_deals      INTEGER NOT NULL DEFAULT 0,
						last_cleared_items TEXT NOT NULL DEFAULT '[]'
					)
				`); err != nil {
					return err
				}
				if _, err := tx.Exec(`
					INSERT INTO users_new (id, phone_number, created_at, name, email, household_id,
						referral_code, referral_points, last_active, login_count, address,
						penny_brand_mode, deal_accept_count, deal_shown_count, penny_slider,
						penny_autotune, penny_deals, last_cleared_items)
					SELECT id, phone_number, created_at, name, email, household_id,
						referral_code, referral_points, last_active, login_count, address,
						penny_brand_mode, deal_accept_count, deal_shown_count, penny_slider,
						penny_autotune, penny_deals, last_cleared_items
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
		"ALTER TABLE users ADD COLUMN penny_slider INTEGER NOT NULL DEFAULT 35",
		"ALTER TABLE users ADD COLUMN penny_autotune INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE users ADD COLUMN penny_deals INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE users ADD COLUMN last_cleared_items TEXT NOT NULL DEFAULT '[]'",
	} {
		db.Exec(stmt)
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
	} {
		db.Exec(stmt)
	}

	return db, nil
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
func resolveBrandAlternatives(db *sql.DB, userID int, pennyBrandMode string, items []ListItem, parsedItems []ParsedItem) []BrandDisambiguation {
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
	err := scanner.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Notes, &item.Source, &checked, &urgent, &item.CreatedAt, &addedByName, &addedByPhone, &claimedByID, &claimedByName)
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

// maskPhone masks a 10-digit phone number: "3101234567" → "310***4567"
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
