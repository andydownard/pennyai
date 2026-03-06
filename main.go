// =============================================================================
// thriftly — main.go
//
// The backend server. Handles:
//   1. Serving the React frontend (static files from "static/")
//   2. Phone verification via Twilio Verify (OTP codes)
//   3. Session management (login/logout via cookies)
//   4. User persistence in SQLite (upserts on each login)
//
// PHONE NUMBER HANDLING:
//   The frontend sends raw 10-digit US numbers like "5551234567".
//   This server prepends "+1" before sending to Twilio, which requires
//   E.164 format ("+15551234567"). The user never sees or types "+1".
//
// HOW TO RUN:
//   export TWILIO_ACCOUNT_SID=ACxxxxxxxx
//   export TWILIO_AUTH_TOKEN=xxxxxxxx
//   export TWILIO_VERIFY_SERVICE_SID=VAxxxxxxxx
//   go run main.go
//
// SECURITY:
//   - Twilio credentials live in environment variables, never in code
//   - .env is in .gitignore so it never gets committed
//   - The frontend NEVER sees your API keys — all Twilio calls happen
//     server-side, and the Go server never exposes env vars via any endpoint
//   - Session cookies are HttpOnly (JavaScript can't read them)
//   - Session tokens are 256 bits of cryptographic randomness
//
// GO CONCEPTS IN THIS FILE:
//   - Structs: custom data types (like objects in JS)
//   - Methods: functions attached to a struct (like class methods)
//   - Interfaces: contracts — "any type with these methods qualifies"
//   - sync.RWMutex: a lock so concurrent requests don't corrupt shared data
//   - database/sql: Go's standard database interface
//   - Error handling: Go returns errors as values, no try/catch
// =============================================================================

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// The underscore import registers the SQLite driver with database/sql.
	// We never call this package directly — it just runs an init() function
	// that says "hey database/sql, I can handle 'sqlite' databases."
	_ "modernc.org/sqlite"
)

// =============================================================================
// CONFIGURATION
//
// We read secrets from environment variables, never from source code.
// Your .env file holds the real values. It's in .gitignore so it won't
// get pushed to GitHub where anyone could see it.
// =============================================================================

type Config struct {
	TwilioAccountSID    string // starts with "AC"
	TwilioAuthToken     string // your secret — never share this
	TwilioVerifyService string // starts with "VA"
	TwilioPhoneNumber   string // your Twilio phone number in E.164 format ("+1...")
	AnthropicAPIKey     string // starts with "sk-ant-"
	AdminPhone          string // phone number with admin access (10 digits, no +1)
	LogoDevToken        string // Logo.dev API token for brand logos
	NPSIntervalDays     int    // days between NPS re-surveys (default 90)
	Port                string // HTTP port (default "8080")
	DBPath              string // SQLite file path (default "thriftly.db")
	SecureCookies       bool   // set Secure flag on cookies (auto-detected from RAILWAY_ENVIRONMENT_NAME)
}

// loadDotEnv reads a .env file and sets any variables not already in the environment.
// Supports lines like KEY=value and export KEY=value. Skips comments and blanks.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // no .env file is fine
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Only set if not already in environment (env takes precedence)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func loadConfig() Config {
	loadDotEnv(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "thriftly.db"
	}
	npsInterval := 90
	if v := os.Getenv("NPS_INTERVAL_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			npsInterval = parsed
		}
	}

	return Config{
		TwilioAccountSID:    os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:     os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioVerifyService: os.Getenv("TWILIO_VERIFY_SERVICE_SID"),
		TwilioPhoneNumber:   os.Getenv("TWILIO_PHONE_NUMBER"),
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AdminPhone:          os.Getenv("ADMIN_PHONE"),
		LogoDevToken:        os.Getenv("LOGODEV_TOKEN"),
		NPSIntervalDays:     npsInterval,
		Port:                port,
		DBPath:              dbPath,
		SecureCookies:       os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "",
	}
}

// =============================================================================
// DATABASE
//
// SQLite is a database that lives in a single file (thriftly.db). No server
// to install, no configuration — just a file on disk. Perfect for small apps.
//
// We use Go's "database/sql" package, which works the same way regardless of
// which database you use (SQLite, Postgres, MySQL). If we ever outgrow SQLite,
// we just change the driver and connection string.
//
// The users table stores every phone number that has ever logged in.
// "INSERT OR IGNORE" means: create the user if they're new, do nothing if
// they already exist. This is called an "upsert" (update + insert).
// =============================================================================

// initDB opens (or creates) the SQLite database and sets up tables.
// The path parameter is the file location. Use ":memory:" for tests.
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
			phone_number TEXT    UNIQUE NOT NULL,
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

	// Indexes for common query patterns
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_household_members_household_id ON household_members(household_id)",
		"CREATE INDEX IF NOT EXISTS idx_household_members_user_id ON household_members(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_list_items_user_id ON list_items(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_lists_user_id ON saved_lists(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_saved_list_items_saved_list_id ON saved_list_items(saved_list_id)",
	} {
		db.Exec(stmt)
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
		db.Exec(`
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
		`)
		db.Exec("INSERT INTO item_modifications_new SELECT * FROM item_modifications")
		db.Exec("DROP TABLE item_modifications")
		db.Exec("ALTER TABLE item_modifications_new RENAME TO item_modifications")
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
			phone_number TEXT NOT NULL,
			created_at   DATETIME NOT NULL,
			expires_at   DATETIME NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating sessions table: %w", err)
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

// handleLogoProxy serves brand logos with a 3-layer cache:
//   1. In-memory sync.Map (sub-ms)
//   2. SQLite BLOB (survives restarts, persists on Railway)
//   3. Logo.dev API fetch → store in SQLite + memory
//   4. SVG placeholder fallback (deterministic color from brand name)
//
// URL: /api/logo?domain=kraft.com
func (s *Server) handleLogoProxy(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	domain := r.URL.Query().Get("domain")
	brand := r.URL.Query().Get("brand")

	// If brand name provided, resolve to domain via brands table
	if brand != "" && domain == "" {
		var d string
		if err := s.db.QueryRow("SELECT domain FROM brands WHERE LOWER(name) = LOWER(?)", brand).Scan(&d); err == nil {
			domain = d
		} else {
			// Guess domain from brand name
			domain = strings.ToLower(brand)
			domain = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					return r
				}
				return -1
			}, domain) + ".com"
		}
	}

	if domain == "" {
		http.Error(w, "Missing domain or brand parameter", http.StatusBadRequest)
		return
	}
	// Sanitize: only allow alphanumeric, dots, hyphens
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			http.Error(w, "Invalid domain", http.StatusBadRequest)
			return
		}
	}

	cacheKey := strings.ToLower(domain)

	// Layer 1: in-memory cache
	if cached, ok := s.logoCache.Load(cacheKey); ok {
		cl := cached.(*cachedLogo)
		w.Header().Set("Content-Type", cl.contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(cl.data)
		return
	}

	// Layer 2: SQLite BLOB
	var logoData []byte
	var contentType string
	err := s.db.QueryRow(
		"SELECT bl.logo_data, bl.content_type FROM brand_logos bl JOIN brands b ON b.id = bl.brand_id WHERE LOWER(b.domain) = LOWER(?)",
		domain,
	).Scan(&logoData, &contentType)
	if err == nil && len(logoData) > 0 {
		s.logoCache.Store(cacheKey, &cachedLogo{data: logoData, contentType: contentType})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(logoData)
		return
	}

	// Layer 3: fetch from external APIs
	var brandID int
	var brandName string
	s.db.QueryRow("SELECT id, name FROM brands WHERE LOWER(domain) = LOWER(?)", domain).Scan(&brandID, &brandName)

	logoData, contentType = fetchBrandLogo(domain, s.config.LogoDevToken)
	if logoData != nil {
		// Store in SQLite if brand exists (async)
		if brandID > 0 {
			go func(id int, data []byte, ct string) {
				s.db.Exec(
					"INSERT OR REPLACE INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'logodev')",
					id, data, ct,
				)
			}(brandID, logoData, contentType)
		}
		s.logoCache.Store(cacheKey, &cachedLogo{data: logoData, contentType: contentType})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(logoData)
		return
	}

	// Layer 4: SVG placeholder
	name := brandName
	if name == "" {
		name = strings.TrimSuffix(domain, ".com")
	}
	s.servePlaceholderLogo(w, name)
}

// fetchBrandLogo fetches a brand logo from Logo.dev.
func fetchBrandLogo(domain, token string) ([]byte, string) {
	if token == "" {
		return nil, ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://img.logo.dev/" + url.QueryEscape(domain) + "?token=" + url.QueryEscape(token) + "&size=128&format=png")
	if err == nil && resp.StatusCode == 200 {
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(data) > 200 {
			ct := resp.Header.Get("Content-Type")
			if ct == "" {
				ct = "image/png"
			}
			return data, ct
		}
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil, ""
}

// servePlaceholderLogo returns an SVG with the brand's initials in a deterministic color.
func (s *Server) servePlaceholderLogo(w http.ResponseWriter, name string) {
	initials := logoInitials(name)
	color := logoColor(name)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <rect width="128" height="128" rx="16" fill="%s"/>
  <text x="64" y="64" dominant-baseline="central" text-anchor="middle"
        font-family="system-ui,sans-serif" font-weight="600" font-size="48" fill="white">%s</text>
</svg>`, color, initials)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Write([]byte(svg))
}

func logoInitials(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return "?"
	}
	if len(words) == 1 {
		r := []rune(words[0])
		return strings.ToUpper(string(r[0:1]))
	}
	return strings.ToUpper(string([]rune(words[0])[0:1]) + string([]rune(words[1])[0:1]))
}

func logoColor(name string) string {
	colors := []string{"#4F46E5", "#7C3AED", "#DB2777", "#DC2626", "#EA580C", "#CA8A04", "#16A34A", "#0891B2"}
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return colors[h%len(colors)]
}

// warmLogoCache pre-fetches logos for brands missing from the brand_logos table.
// Runs in the background after server starts.
func (s *Server) warmLogoCache() {
	// First, load all existing logos into memory
	rows, err := s.db.Query("SELECT b.domain, bl.logo_data, bl.content_type FROM brand_logos bl JOIN brands b ON b.id = bl.brand_id")
	if err == nil {
		defer rows.Close()
		count := 0
		for rows.Next() {
			var domain, ct string
			var data []byte
			if rows.Scan(&domain, &data, &ct) == nil {
				s.logoCache.Store(strings.ToLower(domain), &cachedLogo{data: data, contentType: ct})
				count++
			}
		}
		if count > 0 {
			log.Printf("[logos] loaded %d logos from database", count)
		}
	}

	// Then fetch missing logos
	rows2, err := s.db.Query(`
		SELECT b.id, b.name, b.domain FROM brands b
		LEFT JOIN brand_logos bl ON bl.brand_id = b.id
		WHERE bl.brand_id IS NULL AND b.domain != ''
	`)
	if err != nil {
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var id int
		var name, domain string
		if rows2.Scan(&id, &name, &domain) != nil {
			continue
		}
		data, ct := fetchBrandLogo(domain, s.config.LogoDevToken)
		if data != nil {
			s.db.Exec(
				"INSERT OR REPLACE INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'logodev')",
				id, data, ct,
			)
			s.logoCache.Store(strings.ToLower(domain), &cachedLogo{data: data, contentType: ct})
			log.Printf("[logos] fetched logo for %s (%s)", name, domain)
		}
		time.Sleep(500 * time.Millisecond) // rate limit
	}
	log.Printf("[logos] warm-up complete")
}

// handleTrips handles GET/POST/DELETE /api/trips for the shopping trip planner.
func (s *Server) handleTrips(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get household_id for the user
	var householdID sql.NullInt64
	s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)
	if !householdID.Valid {
		// User not in a household — use a synthetic household_id of -userID
		householdID.Int64 = int64(-session.UserID)
		householdID.Valid = true
	}

	switch r.Method {
	case http.MethodGet:
		// Clean up old trips (older than 48 hours)
		s.db.Exec("DELETE FROM shopping_trips WHERE trip_date < date('now', '-2 days')")

		// Get all trips for the household
		var rows *sql.Rows
		var err error
		if householdID.Int64 < 0 {
			// Solo user — just their own trips
			rows, err = s.db.Query(
				`SELECT st.id, st.user_id, COALESCE(u.name, ''), st.trip_date, st.trip_time, st.time_label, st.created_at
				 FROM shopping_trips st JOIN users u ON u.id = st.user_id
				 WHERE st.user_id = ? ORDER BY st.trip_date, st.trip_time`,
				session.UserID,
			)
		} else {
			rows, err = s.db.Query(
				`SELECT st.id, st.user_id, COALESCE(u.name, ''), st.trip_date, st.trip_time, st.time_label, st.created_at
				 FROM shopping_trips st JOIN users u ON u.id = st.user_id
				 WHERE st.household_id = ? ORDER BY st.trip_date, st.trip_time`,
				householdID.Int64,
			)
		}
		if err != nil {
			dbError(w, err, "Failed to load trips")
			return
		}
		defer rows.Close()

		type TripResponse struct {
			ID        int    `json:"id"`
			UserID    int    `json:"user_id"`
			UserName  string `json:"user_name"`
			TripDate  string `json:"trip_date"`
			TripTime  string `json:"trip_time"`
			TimeLabel string `json:"time_label"`
			IsMine    bool   `json:"is_mine"`
			CreatedAt string `json:"created_at"`
		}
		trips := []TripResponse{}
		for rows.Next() {
			var t TripResponse
			var tripTime, timeLabel, createdAt sql.NullString
			if err := rows.Scan(&t.ID, &t.UserID, &t.UserName, &t.TripDate, &tripTime, &timeLabel, &createdAt); err != nil {
				log.Printf("[trips] scan error: %v", err)
				continue
			}
			t.TripTime = tripTime.String
			t.TimeLabel = timeLabel.String
			t.CreatedAt = createdAt.String
			t.IsMine = t.UserID == session.UserID
			trips = append(trips, t)
		}
		writeJSON(w, http.StatusOK, trips)

	case http.MethodPost:
		var req struct {
			TripDate  string `json:"trip_date"`
			TripTime  string `json:"trip_time"`
			TimeLabel string `json:"time_label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
			return
		}
		if req.TripDate == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trip_date is required"})
			return
		}

		// Delete any existing future trip for this user (one active trip per user)
		s.db.Exec("DELETE FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day')", session.UserID)

		// Use real household_id or 0 for solo users
		hid := householdID.Int64
		if hid < 0 {
			hid = 0
		}

		var tripTime, timeLabel *string
		if req.TripTime != "" {
			tripTime = &req.TripTime
		}
		if req.TimeLabel != "" {
			timeLabel = &req.TimeLabel
		}

		result, err := s.db.Exec(
			"INSERT INTO shopping_trips (user_id, household_id, trip_date, trip_time, time_label) VALUES (?, ?, ?, ?, ?)",
			session.UserID, hid, req.TripDate, tripTime, timeLabel,
		)
		if err != nil {
			dbError(w, err, "Failed to save trip")
			return
		}
		id, _ := result.LastInsertId()
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "ok": true})

	case http.MethodDelete:
		s.db.Exec("DELETE FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day')", session.UserID)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBrands returns a JSON map of lowercased brand name → domain.
// No auth required — brand data is public and needed for fast page load.
func (s *Server) handleBrands(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	rows, err := s.db.Query("SELECT name, domain FROM brands")
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, domain string
		if err := rows.Scan(&name, &domain); err != nil {
			continue
		}
		result[strings.ToLower(name)] = domain
	}
	if err := rows.Err(); err != nil {
		log.Printf("[brands] row iteration error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(result)
}

// User represents a row in the users table.
type User struct {
	ID          int
	PhoneNumber string
	Name        string
	Email       string
	HouseholdID int
	CreatedAt   string
}

// dbExecer abstracts *sql.DB and *sql.Tx so functions can run inside
// or outside a transaction.
type dbExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// recordModification inserts a row into item_modifications for audit history.
func recordModification(ex dbExecer, itemID, userID int, action, detail, source string) {
	_, err := ex.Exec(
		"INSERT INTO item_modifications (item_id, user_id, action, detail, source) VALUES (?, ?, ?, ?, ?)",
		itemID, userID, action, detail, source,
	)
	if err != nil {
		log.Printf("[mod] error recording modification for item %d: %v", itemID, err)
	}
}

// upsertUser ensures a user exists for this phone number and returns them.
// If the phone is new, it creates a row. If it already exists, it just
// looks up the existing row. Either way, you get back a User.
// Also auto-creates a household if the user doesn't have one.
// Wraps all operations in a transaction to prevent race conditions.
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

// getHouseholdUserIDs returns all user IDs sharing the same household.
// Falls back to [userID] if no household is found.
func getHouseholdUserIDs(db *sql.DB, userID int) ([]int, error) {
	var householdID sql.NullInt64
	err := db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&householdID)
	if err != nil || !householdID.Valid {
		return []int{userID}, nil
	}

	rows, err := db.Query("SELECT user_id FROM household_members WHERE household_id = ?", householdID.Int64)
	if err != nil {
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
	if err := rows.Err(); err != nil {
		log.Printf("[household] row iteration error: %v", err)
	}
	if len(ids) == 0 {
		return []int{userID}, nil
	}
	return ids, nil
}

// householdItemsWhereClause returns a SQL fragment "user_id IN (?,?,...)" and
// the corresponding args, for queries that should span the whole household.
func householdItemsWhereClause(db *sql.DB, userID int) (string, []interface{}) {
	ids, _ := getHouseholdUserIDs(db, userID)
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return "user_id IN (" + strings.Join(placeholders, ",") + ")", args
}

// itemData holds the fields shared between list_items and saved_list_items.
// Used by collectListItems and saved-list handlers.
type itemData struct {
	Name, Brand, Department, Size, Notes string
	Quantity                             int
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

// ListItem represents a row in the list_items table.
type ListItem struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Brand         string `json:"brand"`
	Department    string `json:"department"`
	Quantity      int    `json:"quantity"`
	Size          string `json:"size"`
	Notes         string `json:"notes"`
	Source        string `json:"source"`
	Checked       bool   `json:"checked"`
	Urgent        bool   `json:"urgent"`
	CreatedAt     string `json:"created_at"`
	AddedByName   string `json:"added_by_name"`
	AddedByPhone  string `json:"added_by_phone"`
	ClaimedByID   *int   `json:"claimed_by_id"`
	ClaimedByName string `json:"claimed_by_name"`
}

// =============================================================================
// SESSION STORE
//
// Keeps track of who's logged in. A session maps a random token (in a browser
// cookie) to a user ID and phone number (on the server).
//
// Sessions are stored in SQLite so they survive server restarts and deploys.
// Each session expires after 30 days. Expired sessions are cleaned up hourly.
// =============================================================================

type Session struct {
	UserID      int       // the user's database ID
	PhoneNumber string    // 10-digit format: "5551234567"
	CreatedAt   time.Time // when this session was created
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

// CreateSession generates a random 64-char hex token, stores the session
// in SQLite, and returns the token. Sessions expire after 30 days.
func (s *SessionStore) CreateSession(userID int, phone string) (string, error) {
	b := make([]byte, 32) // 32 bytes = 256 bits of randomness
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	_, err := s.db.Exec(
		"INSERT INTO sessions (token, user_id, phone_number, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		token, userID, phone, time.Now().UTC(), time.Now().UTC().Add(30*24*time.Hour),
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *SessionStore) GetSession(token string) (Session, bool) {
	var session Session
	err := s.db.QueryRow(
		"SELECT user_id, phone_number, created_at FROM sessions WHERE token = ? AND expires_at > ?",
		token, time.Now().UTC(),
	).Scan(&session.UserID, &session.PhoneNumber, &session.CreatedAt)
	if err != nil {
		return Session{}, false
	}
	return session, true
}

func (s *SessionStore) DeleteSession(token string) {
	if _, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token); err != nil {
		log.Printf("[warn] failed to delete session: %v", err)
	}
}

// CleanExpired removes expired sessions. Called periodically.
func (s *SessionStore) CleanExpired() {
	if _, err := s.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now().UTC()); err != nil {
		log.Printf("[warn] failed to clean expired sessions: %v", err)
	}
}

// =============================================================================
// TWILIO VERIFY CLIENT
//
// Interface + real implementation. Tests use a mock (in main_test.go).
// =============================================================================

type TwilioClient interface {
	SendVerification(phone string) error
	CheckVerification(phone, code string) (bool, error)
	SendSMS(from, to, body string) error
}

type RealTwilioClient struct {
	AccountSID    string
	AuthToken     string
	VerifyService string
}

// doRequest performs a Twilio API POST with Basic Auth and checks the status.
func (t *RealTwilioClient) doRequest(apiURL string, data url.Values, expectStatus int) ([]byte, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Twilio: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expectStatus {
		return body, fmt.Errorf("twilio returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// SendVerification asks Twilio to text an OTP code to the phone number.
func (t *RealTwilioClient) SendVerification(phone string) error {
	apiURL := fmt.Sprintf("https://verify.twilio.com/v2/Services/%s/Verifications", t.VerifyService)
	data := url.Values{"To": {phone}, "Channel": {"sms"}}
	_, err := t.doRequest(apiURL, data, http.StatusCreated)
	return err
}

// CheckVerification asks Twilio if the user-entered code is correct.
func (t *RealTwilioClient) CheckVerification(phone, code string) (bool, error) {
	apiURL := fmt.Sprintf("https://verify.twilio.com/v2/Services/%s/VerificationCheck", t.VerifyService)
	data := url.Values{"To": {phone}, "Code": {code}}
	body, err := t.doRequest(apiURL, data, http.StatusOK)
	if err != nil {
		log.Printf("Twilio verify error: %v", err)
		return false, nil
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("parsing response: %w", err)
	}
	return result.Status == "approved", nil
}

// SendSMS sends an SMS message via the Twilio Messages API.
func (t *RealTwilioClient) SendSMS(from, to, body string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.AccountSID)
	data := url.Values{"From": {from}, "To": {to}, "Body": {body}}
	_, err := t.doRequest(apiURL, data, http.StatusCreated)
	return err
}

// =============================================================================
// PHONE NUMBER NORMALIZATION
//
// The frontend sends raw 10-digit numbers. Twilio needs E.164 ("+15551234567").
// We also handle common edge cases: leading country code "1", formatting
// characters like parens/dashes/dots/spaces, and accidental letters.
// =============================================================================

// normalizePhone strips non-digits, drops a leading country code 1,
// and returns E.164 format. Returns ("", false) if the result isn't 10 digits.
func normalizePhone(input string) (string, bool) {
	// Keep only digits — drop parens, dashes, dots, spaces, letters, everything
	var digits strings.Builder
	for _, r := range input {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	// If someone typed "15551234567" or pasted "+1 (555) 123-4567" (which
	// becomes "15551234567" after stripping), drop the leading country code.
	if len(d) == 11 && d[0] == '1' {
		d = d[1:]
	}

	// Must be exactly 10 digits after cleanup
	if len(d) != 10 {
		return "", false
	}

	return "+1" + d, true
}

// =============================================================================
// HTTP SERVER
// =============================================================================

// SkippedItem represents a parsed item that was skipped because it already
// exists on the user's list. The frontend can offer to increase quantity.
type SkippedItem struct {
	Parsed     ParsedItem `json:"parsed"`
	ExistingID int        `json:"existing_id"`
}

// ParseResponse is the response format for the parse-and-add endpoint.
type ParseResponse struct {
	Items   []ListItem    `json:"items"`
	Skipped []SkippedItem `json:"skipped"`
	Added   int           `json:"added"`
}

type cachedLogo struct {
	data        []byte
	contentType string
}

type Server struct {
	config     Config
	db         *sql.DB
	sessions   *SessionStore
	twilio     TwilioClient
	parseItems func(apiKey, text string, images []string) ([]ParsedItem, error)
	smsBuffer  *messageBuffer
	logoCache  sync.Map // domain -> *cachedLogo
}

// handleSendCode handles POST /auth/send-code
// Request:  { "phone_number": "5551234567" }
// Response: { "success": true }
func (s *Server) handleSendCode(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	e164, ok := normalizePhone(req.PhoneNumber)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Please enter a valid 10-digit US phone number",
		})
		return
	}

	log.Printf("Sending verification code to %s", e164)

	if err := s.twilio.SendVerification(e164); err != nil {
		dbError(w, err, "We couldn't send a verification code right now. Please try again in a moment.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleVerifyCode handles POST /auth/verify-code
// Request:  { "phone_number": "5551234567", "code": "123456" }
// Response: { "success": true, "user_id": 1, "phone_number": "5551234567" }
//
// On success: upserts the user in the database, creates a session, sets cookie.
func (s *Server) handleVerifyCode(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
		Code        string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	e164, ok := normalizePhone(req.PhoneNumber)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid phone number",
		})
		return
	}

	log.Printf("Checking verification code for %s", e164)

	approved, err := s.twilio.CheckVerification(e164, req.Code)
	if err != nil {
		dbError(w, err, "We couldn't verify your code. Please try again.")
		return
	}

	if !approved {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid code",
		})
		return
	}

	// Phone verified — upsert the user in the database.
	// If this phone has logged in before, we get the existing user.
	// If it's new, a row is created. Either way, we have a user ID.
	digits := strings.TrimPrefix(e164, "+1")

	user, err := upsertUser(s.db, digits)
	if err != nil {
		dbError(w, err, "We couldn't set up your account. Please try again.")
		return
	}

	token, err := s.sessions.CreateSession(user.ID, digits)
	if err != nil {
		dbError(w, err, "We couldn't log you in. Please try again.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,                 // JS can't read this cookie
		Secure:   s.config.SecureCookies,
		SameSite: http.SameSiteLaxMode, // CSRF protection
		MaxAge:   60 * 60 * 24 * 30,    // 30 days
	})

	log.Printf("User %d (%s) logged in", user.ID, digits)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"user_id":      user.ID,
		"phone_number": digits,
		"is_admin":     s.config.AdminPhone != "" && digits == s.config.AdminPhone,
	})
}

// handleLogout handles POST /auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if cookie, err := r.Cookie("session"); err == nil {
		s.sessions.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.config.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleMe handles GET /auth/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Not logged in",
		})
		return
	}

	session, ok := s.sessions.GetSession(cookie.Value)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Session expired",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logged_in":    true,
		"user_id":      session.UserID,
		"phone_number": session.PhoneNumber,
		"is_admin":     s.config.AdminPhone != "" && session.PhoneNumber == s.config.AdminPhone,
	})
}

// listItemColumns is the shared SELECT column list for list_items queries.
const listItemColumns = "li.id, li.name, li.brand, li.department, li.quantity, li.size, li.notes, li.source, li.checked, li.urgent, li.created_at, COALESCE(u.name,''), u.phone_number, li.claimed_by, COALESCE(cu.name,'')"

// listItemJoins is the shared FROM/JOIN clause for list_items queries.
const listItemJoins = "list_items li JOIN users u ON u.id = li.user_id LEFT JOIN users cu ON cu.id = li.claimed_by"

// scanListItem scans a row into a ListItem. The row must match listItemColumns.
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
		return phone[:3] + "***" + phone[6:]
	}
	return phone
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
	queryArgs := append(whereArgs, name)
	var existingID int
	var existingQty int
	var existingBrand, existingSize string
	var existingChecked int
	err = tx.QueryRow(
		"SELECT id, quantity, brand, size, checked FROM list_items WHERE "+whereClause+" AND LOWER(name) = LOWER(?)",
		queryArgs...,
	).Scan(&existingID, &existingQty, &existingBrand, &existingSize, &existingChecked)

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
			_, err := tx.Exec(
				"UPDATE list_items SET quantity = ?, brand = ?, size = ?, notes = ? WHERE id = ?",
				newQty, newBrand, newSize, notes, existingID,
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

// =============================================================================
// ANTHROPIC CLIENT
//
// Calls Claude to parse free-form grocery text into structured items.
// We make raw HTTP calls (no SDK) to keep dependencies minimal.
// =============================================================================

// ParsedItem is what Claude returns for each grocery item.
type ParsedItem struct {
	Name       string `json:"name"`
	Brand      string `json:"brand"`
	Department string `json:"department"`
	Quantity   int    `json:"quantity"`
	Size       string `json:"size"`
	Notes      string `json:"notes"`
}

// parseItemsWithClaude sends the user's raw text (and optional image) to Claude
// and gets back a list of grocery items with departments assigned.
func parseItemsWithClaude(apiKey, userText string, images []string) ([]ParsedItem, error) {
	systemPrompt := `You are a grocery shopping assistant. Parse the user's input into individual grocery items.

For each item return:
- "name": the specific product name, properly capitalized, WITHOUT the brand. PRESERVE flavor, variety, and variant info in the name. Examples: "Tangerine Sparkling Water" not just "Sparkling Water", "Grape Sparkling Water" not "Grapes".
- "brand": brand name if mentioned, otherwise "" (e.g. "Jif", "LaCroix", "Waterloo", "Organic Valley")
- "department": one of: Produce, Dairy & Eggs, Meat & Seafood, Bakery, Frozen, Pantry, Beverages, Household, Other
- "quantity": number of units to buy, integer, default 1
- "size": package size/weight if specified, otherwise "" (e.g. "16 oz", "1 lb", "Large", "1 gal")
- "notes": any extra detail the user mentioned that doesn't fit above, otherwise "" (e.g. "the one in the blue box", "boneless skinless", "ripe ones")

CRITICAL RULES:
1. NEVER genericize product names. Keep the specific product the user asked for.
2. Flavor/variety is part of the name, NOT the brand. "LaCroix Tangerine" → name: "Tangerine Sparkling Water", brand: "LaCroix"
3. Do NOT rename products. "Waterloo Grape" → name: "Grape Sparkling Water", brand: "Waterloo", NOT "Grapes"
4. When in doubt, keep the user's exact words in the name rather than guessing a generic version.

Separate brand from product name. Examples:
- "jif pb" → {"name":"Peanut Butter","brand":"Jif","department":"Pantry","quantity":1,"size":"","notes":""}
- "2 boxes kraft mac and cheese" → {"name":"Macaroni & Cheese","brand":"Kraft","department":"Pantry","quantity":2,"size":"","notes":""}
- "large organic eggs" → {"name":"Organic Eggs","brand":"","department":"Dairy & Eggs","quantity":1,"size":"Large","notes":""}
- "gallon of whole milk" → {"name":"Whole Milk","brand":"","department":"Dairy & Eggs","quantity":1,"size":"1 gal","notes":""}
- "lacroix tangerine" → {"name":"Tangerine Sparkling Water","brand":"LaCroix","department":"Beverages","quantity":1,"size":"","notes":""}
- "waterloo grape" → {"name":"Grape Sparkling Water","brand":"Waterloo","department":"Beverages","quantity":1,"size":"","notes":""}
- "tillamook sharp cheddar block" → {"name":"Sharp Cheddar","brand":"Tillamook","department":"Dairy & Eggs","quantity":1,"size":"Block","notes":""}
- "diet coke 12 pack" → {"name":"Diet Coke","brand":"Coca-Cola","department":"Beverages","quantity":1,"size":"12 pack","notes":""}

Image handling — adapt to what you see:
- Grocery items or products: extract each item.
- Handwritten or printed shopping list: transcribe each item.
- Recipe (on paper, screen, or in text): extract the ingredients as grocery items.
- Fridge, pantry, or cupboard photo: use the user's text as context (e.g. "what am I missing", "what's running low") to suggest items that appear to be low or absent. If no text is provided, list items that appear to be running low.

IMPORTANT: Do NOT break down foods into their constituent ingredients unless the user explicitly asks (e.g. "what do I need to make lasagna") or a recipe image is provided. Treat compound foods as single items:
- "lasagna" → [{"name":"Lasagna",...}] NOT individual ingredients
- "chicken stir fry" → [{"name":"Chicken Stir Fry",...}]

Return ONLY a JSON array, no markdown, no code fences, no explanation.`

	// Build message content — either plain text or multi-block (images + text)
	var messageContent interface{}
	if len(images) > 0 {
		var contentBlocks []map[string]interface{}
		for _, img := range images {
			mediaType := "image/jpeg"
			b64Data := img
			if strings.HasPrefix(img, "data:") {
				if idx := strings.Index(img, ";base64,"); idx > 0 {
					mediaType = img[5:idx]
					b64Data = img[idx+8:]
				}
			}
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "image",
				"source": map[string]string{
					"type":       "base64",
					"media_type": mediaType,
					"data":       b64Data,
				},
			})
		}
		textContent := userText
		if textContent == "" {
			textContent = "Identify all grocery items in these images. If it's a fridge or pantry, list items that appear to be running low or missing."
		}
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "text",
			"text": textContent,
		})
		messageContent = contentBlocks
	} else {
		messageContent = userText
	}

	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages": []map[string]interface{}{
			{"role": "user", "content": messageContent},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}
	if apiResp.StopReason == "max_tokens" {
		log.Printf("[parse] WARNING: Claude response truncated (max_tokens reached)")
	}

	text := apiResp.Content[0].Text
	// Strip markdown code fences if Claude wrapped the JSON despite instructions
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		// Remove opening fence (```json or ```)
		if i := strings.Index(text, "\n"); i >= 0 {
			text = text[i+1:]
		}
		// Remove closing fence
		if i := strings.LastIndex(text, "```"); i >= 0 {
			text = text[:i]
		}
		text = strings.TrimSpace(text)
	}

	var items []ParsedItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		// If truncated, try to salvage partial JSON by closing the array
		if apiResp.StopReason == "max_tokens" {
			// Find the last complete object by looking for the last "},"
			if idx := strings.LastIndex(text, "},"); idx > 0 {
				salvaged := text[:idx+1] + "]"
				if err2 := json.Unmarshal([]byte(salvaged), &items); err2 == nil {
					log.Printf("[parse] Salvaged %d items from truncated response", len(items))
					return items, nil
				}
			}
		}
		return nil, fmt.Errorf("parsing Claude JSON: %w", err)
	}

	return items, nil
}

// =============================================================================
// LIST HANDLERS
// =============================================================================

// requireSession extracts and validates the session cookie.
// Returns (Session, true) on success, writes a 401 and returns false on failure.
func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		log.Printf("[auth] no session cookie on %s %s: %v", r.Method, r.URL.Path, err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not logged in"})
		return Session{}, false
	}
	session, ok := s.sessions.GetSession(cookie.Value)
	if !ok {
		log.Printf("[auth] invalid/expired session on %s %s (token prefix: %s...)", r.Method, r.URL.Path, cookie.Value[:8])
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Session expired"})
		return Session{}, false
	}
	return session, true
}

// requireAdmin checks for a valid session and that the phone is the admin phone.
// Returns the session and true if authorized, or writes an error and returns false.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return Session{}, false
	}
	if s.config.AdminPhone == "" || session.PhoneNumber != s.config.AdminPhone {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return Session{}, false
	}
	return session, true
}

// handleAdminDashboard handles GET /api/admin/dashboard?days=30
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	modifier := fmt.Sprintf("-%d days", days)

	var activeUsers, totalUsers, newUsers int
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT id) FROM users WHERE last_active >= datetime('now', ?)", modifier).Scan(&activeUsers); err != nil {
		log.Printf("[admin] failed to count active users: %v", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers); err != nil {
		log.Printf("[admin] failed to count total users: %v", err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= datetime('now', ?)", modifier).Scan(&newUsers); err != nil {
		log.Printf("[admin] failed to count new users: %v", err)
	}

	var referralsInPeriod int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM referrals WHERE created_at >= datetime('now', ?)", modifier).Scan(&referralsInPeriod); err != nil {
		log.Printf("[admin] failed to count referrals: %v", err)
	}

	var kFactor float64
	if newUsers > 0 {
		kFactor = float64(referralsInPeriod) / float64(newUsers)
	}

	type leaderEntry struct {
		Phone    string `json:"phone"`
		Name     string `json:"name"`
		Points   int    `json:"points"`
		Referrals int   `json:"referrals"`
	}

	rows, err := s.db.Query(`
		SELECT u.phone_number, COALESCE(u.name,''), COALESCE(u.referral_points,0), COUNT(r.id) as ref_count
		FROM users u
		JOIN referrals r ON r.referrer_id = u.id
		GROUP BY u.id
		ORDER BY COALESCE(u.referral_points,0) DESC
		LIMIT 20
	`)
	var leaderboard []leaderEntry
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e leaderEntry
			if err := rows.Scan(&e.Phone, &e.Name, &e.Points, &e.Referrals); err == nil {
				e.Phone = maskPhone(e.Phone)
				leaderboard = append(leaderboard, e)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[admin] leaderboard row iteration error: %v", err)
		}
	}
	if leaderboard == nil {
		leaderboard = []leaderEntry{}
	}

	// NPS data
	type npsEntry struct {
		Score     int    `json:"score"`
		Comment   string `json:"comment"`
		UserName  string `json:"user_name"`
		UserPhone string `json:"user_phone"`
		CreatedAt string `json:"created_at"`
	}

	var npsAvg float64
	if err := s.db.QueryRow("SELECT COALESCE(AVG(score), 0) FROM nps_responses").Scan(&npsAvg); err != nil {
		log.Printf("[admin] failed to calculate NPS average: %v", err)
	}

	npsRows, err := s.db.Query(`
		SELECT n.score, n.comment, COALESCE(u.name,''), u.phone_number, n.created_at
		FROM nps_responses n
		JOIN users u ON u.id = n.user_id
		ORDER BY n.created_at DESC
		LIMIT 100
	`)
	var npsResponses []npsEntry
	if err == nil {
		defer npsRows.Close()
		for npsRows.Next() {
			var e npsEntry
			if err := npsRows.Scan(&e.Score, &e.Comment, &e.UserName, &e.UserPhone, &e.CreatedAt); err == nil {
				e.UserPhone = maskPhone(e.UserPhone)
				npsResponses = append(npsResponses, e)
			}
		}
		if err := npsRows.Err(); err != nil {
			log.Printf("[admin] NPS row iteration error: %v", err)
		}
	}
	if npsResponses == nil {
		npsResponses = []npsEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_users":  activeUsers,
		"total_users":   totalUsers,
		"new_users":     newUsers,
		"k_factor":      kFactor,
		"leaderboard":   leaderboard,
		"days":          days,
		"nps_responses": npsResponses,
		"nps_average":   npsAvg,
	})
}

// legalPageHTML wraps legal page content in a simple styled HTML shell.
func legalPageHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — Penny by Thriftly</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Avenir','Avenir Next',-apple-system,'Segoe UI',sans-serif;color:#2A2A2A;background:#F7F6F3;line-height:1.6}
.wrap{max-width:640px;margin:0 auto;padding:40px 20px}
h1{font-size:24px;font-weight:700;margin-bottom:8px}
.updated{font-size:13px;color:#6B6B6B;margin-bottom:32px}
h2{font-size:18px;font-weight:600;margin:28px 0 8px}
p,ul{font-size:15px;margin-bottom:12px}
ul{padding-left:20px}
li{margin-bottom:6px}
a{color:#35644F}
.back{display:inline-block;margin-bottom:24px;font-size:14px;color:#35644F;text-decoration:none}
</style></head><body><div class="wrap">
<a class="back" href="/">← Back to Penny</a>
%s
</div></body></html>`, title, body)
}

func handlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, legalPageHTML("Privacy Policy", `
<h1>Privacy Policy</h1>
<p class="updated">Last updated: March 4, 2026</p>

<p>Penny by Thriftly ("Penny," "we," "us," or "our") operates the Penny shopping list application
and related SMS messaging service. This Privacy Policy describes what information we collect,
how we use it, and your choices.</p>

<h2>1. Information We Collect</h2>
<ul>
<li><strong>Phone number</strong> — provided when you sign up, used for account verification and to send/receive SMS messages.</li>
<li><strong>Name and email</strong> — optionally provided in your profile settings.</li>
<li><strong>Shopping list data</strong> — items you add via the app, text message, or email.</li>
<li><strong>Usage data</strong> — basic analytics such as page views and feature usage to improve the service.</li>
</ul>

<h2>2. How We Use Your Information</h2>
<ul>
<li>To verify your identity and manage your account.</li>
<li>To send and receive SMS messages related to your shopping list (e.g., adding items, notifications).</li>
<li>To operate, maintain, and improve the Penny service.</li>
<li>To respond to your support requests.</li>
</ul>

<h2>3. Information Sharing</h2>
<p>We do <strong>not</strong> sell, rent, or share your personal information with third parties for their
marketing purposes. We share data only in these limited circumstances:</p>
<ul>
<li><strong>Service providers</strong> — We use Twilio to deliver SMS messages and Anthropic to process natural-language list items. These providers access only the data necessary to perform their services.</li>
<li><strong>Household members</strong> — If you join a household, other members can see the shared shopping list and your display name.</li>
<li><strong>Anonymized &amp; aggregated data</strong> — We may share anonymized, aggregated shopping data with brands, retailers, and advertising partners to enable product rebates and improve the shopping experience. This data does not identify you personally — it contains no names, phone numbers, or other personal identifiers.</li>
<li><strong>Legal requirements</strong> — We may disclose information if required by law or to protect our rights.</li>
</ul>

<h2>4. Data Retention</h2>
<p>We retain your account data for as long as your account is active. You can delete your list data
at any time through the app. To delete your account entirely, contact us at the address below.</p>

<h2>5. Security</h2>
<p>We use industry-standard measures to protect your data, including encrypted connections (HTTPS),
HttpOnly session cookies, and secure credential storage.</p>

<h2>6. Children's Privacy</h2>
<p>Penny is not directed at children under 13 and we do not knowingly collect information from children under 13.</p>

<h2>7. Changes</h2>
<p>We may update this policy from time to time. We will notify you of material changes by posting the updated policy on this page.</p>

<h2>8. Contact Us</h2>
<p>If you have questions about this Privacy Policy, contact us at
<a href="mailto:privacy@thrift.ly">privacy@thrift.ly</a>.</p>
`))
}

func handleTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, legalPageHTML("Terms & Conditions", `
<h1>Terms &amp; Conditions</h1>
<p class="updated">Last updated: March 4, 2026</p>

<h2>1. Program Name</h2>
<p><strong>Penny by Thriftly</strong> — an AI-powered shared shopping list service.</p>

<h2>2. Service Description</h2>
<p>Penny lets you create and manage a shared grocery shopping list. You can add items by typing in the app,
sending a text message (SMS), or forwarding an email. Penny uses AI to parse your input into organized list items.</p>

<h2>3. Messaging Terms</h2>
<ul>
<li><strong>Message frequency:</strong> Message frequency varies based on your usage. You will receive SMS messages
when household members add items, and in response to messages you send to Penny.</li>
<li><strong>Message and data rates may apply.</strong> Your carrier's standard messaging and data rates apply to all
SMS messages sent to and received from Penny.</li>
</ul>

<h2>4. Opt-Out</h2>
<p>You can opt out of SMS messages at any time by texting <strong>STOP</strong> to the Penny phone number.
After opting out, you will receive one final confirmation message and no further SMS messages.</p>

<h2>5. Help</h2>
<p>For support, text <strong>HELP</strong> to the Penny phone number, or email us at
<a href="mailto:support@thrift.ly">support@thrift.ly</a>.</p>

<h2>6. Eligibility</h2>
<p>You must be at least 13 years old and have a valid US mobile phone number to use Penny.</p>

<h2>7. User Conduct</h2>
<p>You agree to use Penny only for lawful purposes related to managing shopping lists. You are
responsible for all activity under your account.</p>

<h2>8. Intellectual Property</h2>
<p>All content, branding, and technology associated with Penny are owned by Thriftly. You retain
ownership of your shopping list data.</p>

<h2>8a. Anonymized Data &amp; Brand Partnerships</h2>
<p>By using Penny, you agree that we may anonymize and aggregate your shopping list data and share
it with brand and retail partners. This data is stripped of all personally identifying information
(such as your name, phone number, and household details) and is used solely to enable product
rebates, improve product recommendations, and support market research. You will never be personally
identified to any partner.</p>

<h2>9. Disclaimer of Warranties</h2>
<p>Penny is provided "as is" without warranties of any kind. We do not guarantee uninterrupted
or error-free service.</p>

<h2>10. Limitation of Liability</h2>
<p>To the fullest extent permitted by law, Thriftly shall not be liable for any indirect, incidental,
or consequential damages arising from your use of Penny.</p>

<h2>11. Changes</h2>
<p>We may update these terms at any time. Continued use of Penny after changes constitutes acceptance
of the updated terms.</p>

<h2>12. Contact</h2>
<p>Questions? Email us at <a href="mailto:support@thrift.ly">support@thrift.ly</a>.</p>
`))
}

// handleList routes requests under /api/list:
//
//	GET  /api/list              → list all items for the user
//	POST /api/list              → add a single item
//	POST /api/list/parse        → parse free-form text with Claude and bulk-add items
//	PATCH /api/list/{id}/toggle → toggle checked
//	DELETE /api/list/{id}       → remove an item
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/list")
	tail = strings.Trim(tail, "/")

	if tail == "parse" && r.Method == http.MethodPost {
		s.handleParseAndAdd(w, r)
		return
	}

	if tail == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListGet(w, r)
		case http.MethodPost:
			s.handleListAdd(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	parts := strings.Split(tail, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "toggle" && r.Method == http.MethodPatch {
		s.handleListToggle(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "urgent" && r.Method == http.MethodPatch {
		s.handleListUrgent(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "claim" && r.Method == http.MethodPatch {
		s.handleListClaim(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
		s.handleItemHistory(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.handleListEdit(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleListDelete(w, r, id)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleParseAndAdd handles POST /api/list/parse
// Request:  { "text": "milk eggs sourdough chicken", "image": "data:image/jpeg;base64,..." }
// Response: ParseResponse with items, added count, and updated count
func (s *Server) handleParseAndAdd(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Text   string   `json:"text"`
		Image  string   `json:"image"`
		Images []string `json:"images"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	req.Image = strings.TrimSpace(req.Image)
	// Merge: support both single image and images array
	images := req.Images
	if len(images) == 0 && req.Image != "" {
		images = []string{req.Image}
	}
	if req.Text == "" && len(images) == 0 {
		log.Printf("[parse] empty text and images")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Text or image is required"})
		return
	}
	if s.config.AnthropicAPIKey == "" {
		log.Printf("[parse] no API key configured")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
		return
	}

	log.Printf("Parsing grocery text for user %d: len=%d (images: %d)", session.UserID, len(req.Text), len(images))
	parsed, err := s.parseItems(s.config.AnthropicAPIKey, req.Text, images)
	if err != nil {
		log.Printf("[parse] error for user %d: %v", session.UserID, err)
		dbError(w, err, "We couldn't understand that right now. Please try again.")
		return
	}

	added, skippedItems := addParsedItems(s.db, session.UserID, parsed, "app")
	resp := ParseResponse{
		Items:   added,
		Skipped: skippedItems,
		Added:   len(added),
	}
	if resp.Items == nil {
		resp.Items = []ListItem{}
	}
	if resp.Skipped == nil {
		resp.Skipped = []SkippedItem{}
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleListGet handles GET /api/list
func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your list. Please try again.")
		return
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		item, err := scanListItem(rows)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[list] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, items)
}

// handleListAdd handles POST /api/list
// Request: { "name": "Milk", "department": "Dairy & Eggs" }
func (s *Server) handleListAdd(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name       string `json:"name"`
		Department string `json:"department"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}
	if req.Department == "" {
		req.Department = "Other"
	}

	item, _, err := upsertListItem(s.db, session.UserID, req.Name, "", req.Department, "", 1, "app", "")
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

// handleListToggle handles PATCH /api/list/{id}/toggle
func (s *Server) handleListToggle(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	toggleArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET checked = NOT checked WHERE id = ? AND "+whereClause,
		toggleArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	action := "checked"
	if !item.Checked {
		action = "unchecked"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	writeJSON(w, http.StatusOK, item)
}

// handleListUrgent handles PATCH /api/list/{id}/urgent
func (s *Server) handleListUrgent(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	urgentArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET urgent = NOT urgent WHERE id = ? AND "+whereClause,
		urgentArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	action := "marked urgent"
	if !item.Urgent {
		action = "unmarked urgent"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	writeJSON(w, http.StatusOK, item)
}

// handleListClaim handles PATCH /api/list/{id}/claim
func (s *Server) handleListClaim(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Check item exists in household
	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	if item.ClaimedByID != nil && *item.ClaimedByID == session.UserID {
		// Unclaim
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = NULL, claimed_at = NULL WHERE id = ?", id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "unclaimed", "", "app")
	} else {
		// Claim (or override)
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = ?, claimed_at = CURRENT_TIMESTAMP WHERE id = ?", session.UserID, id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "claimed", "", "app")
	}

	updated, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleItemHistory handles GET /api/list/{id}/history
func (s *Server) handleItemHistory(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Verify item belongs to household
	_, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	rows, err := s.db.Query(
		"SELECT m.action, m.detail, m.source, m.created_at, COALESCE(u.name, u.phone_number) FROM item_modifications m LEFT JOIN users u ON u.id = m.user_id WHERE m.item_id = ? ORDER BY m.created_at DESC",
		id,
	)
	if err != nil {
		dbError(w, err, "We couldn't load item history. Please try again.")
		return
	}
	defer rows.Close()

	type modification struct {
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		Source    string `json:"source"`
		CreatedAt string `json:"created_at"`
		UserName  string `json:"user_name"`
	}
	var mods []modification
	for rows.Next() {
		var m modification
		var detail, source, userName sql.NullString
		if err := rows.Scan(&m.Action, &detail, &source, &m.CreatedAt, &userName); err != nil {
			continue
		}
		m.Detail = detail.String
		m.Source = source.String
		m.UserName = userName.String
		mods = append(mods, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[history] row iteration error: %v", err)
	}
	if mods == nil {
		mods = []modification{}
	}

	writeJSON(w, http.StatusOK, mods)
}

// handleListEdit handles PATCH /api/list/{id}
// Request: { "name": "...", "brand": "...", "department": "...", "quantity": 1, "size": "..." } (all optional)
func (s *Server) handleListEdit(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name       *string `json:"name"`
		Brand      *string `json:"brand"`
		Department *string `json:"department"`
		Quantity   *int    `json:"quantity"`
		Size       *string `json:"size"`
		Notes      *string `json:"notes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)

	// Build a single UPDATE with only the fields that were provided
	var setClauses []string
	var setArgs []interface{}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name cannot be empty"})
			return
		}
		setClauses = append(setClauses, "name = ?")
		setArgs = append(setArgs, name)
	}
	if req.Brand != nil {
		setClauses = append(setClauses, "brand = ?")
		setArgs = append(setArgs, *req.Brand)
	}
	if req.Department != nil {
		setClauses = append(setClauses, "department = ?")
		setArgs = append(setArgs, *req.Department)
	}
	if req.Quantity != nil {
		qty := *req.Quantity
		if qty < 1 {
			qty = 1
		}
		setClauses = append(setClauses, "quantity = ?")
		setArgs = append(setArgs, qty)
	}
	if req.Size != nil {
		setClauses = append(setClauses, "size = ?")
		setArgs = append(setArgs, *req.Size)
	}
	if req.Notes != nil {
		setClauses = append(setClauses, "notes = ?")
		setArgs = append(setArgs, *req.Notes)
	}

	if len(setClauses) > 0 {
		query := "UPDATE list_items SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND " + whereClause
		args := append(setArgs, id)
		args = append(args, whereArgs...)
		if _, err := s.db.Exec(query, args...); err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "updated", strings.Join(setClauses, ", "), "app")
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleListDelete handles DELETE /api/list/{id}
func (s *Server) handleListDelete(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	// Delete modifications first to avoid FK constraint violation on older DBs without CASCADE
	s.db.Exec("DELETE FROM item_modifications WHERE item_id = ?", id)
	args := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"DELETE FROM list_items WHERE id = ? AND "+whereClause,
		args...,
	)
	if err != nil {
		dbError(w, err, "We couldn't remove that item. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleListClear handles POST /api/list/clear
func (s *Server) handleListClear(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	_, err := s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...)
	if err != nil {
		dbError(w, err, "We couldn't clear your list. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleListRestore handles POST /api/list/restore — bulk re-insert items (used by undo)
func (s *Server) handleListRestore(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var items []struct {
		Name       string `json:"name"`
		Brand      string `json:"brand"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
		Size       string `json:"size"`
		Checked    bool   `json:"checked"`
		Urgent     bool   `json:"urgent"`
	}
	if !decodeJSON(w, r, &items) {
		return
	}

	for _, item := range items {
		qty := item.Quantity
		if qty < 1 {
			qty = 1
		}
		dept := item.Department
		if dept == "" {
			dept = "Other"
		}
		checked := 0
		if item.Checked {
			checked = 1
		}
		urgent := 0
		if item.Urgent {
			urgent = 1
		}
		_, err := s.db.Exec(
			`INSERT INTO list_items (user_id, name, brand, department, quantity, size, checked, urgent, source)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'app')`,
			session.UserID, strings.TrimSpace(item.Name), strings.TrimSpace(item.Brand),
			dept, qty, strings.TrimSpace(item.Size), checked, urgent,
		)
		if err != nil {
			dbError(w, err, "We couldn't restore your list. Please try again.")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// SAVED LISTS HANDLERS
// =============================================================================

// handleSavedLists routes requests under /api/saved-lists:
//
//	GET    /api/saved-lists          → list all saved lists
//	POST   /api/saved-lists          → save current unchecked items as a new saved list
//	PUT    /api/saved-lists/{id}     → update saved list items from current unchecked items
//	DELETE /api/saved-lists/{id}     → delete a saved list
//	POST   /api/saved-lists/{id}/load → load saved list items into active list
func (s *Server) handleSavedLists(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/saved-lists")
	tail = strings.Trim(tail, "/")

	if tail == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleSavedListsGet(w, r)
		case http.MethodPost:
			s.handleSavedListsCreate(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	parts := strings.Split(tail, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "load" && r.Method == http.MethodPost {
		s.handleSavedListLoad(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPut {
		s.handleSavedListUpdate(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleSavedListDelete(w, r, id)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleSavedListsGet handles GET /api/saved-lists
func (s *Server) handleSavedListsGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	rows, err := s.db.Query(
		"SELECT sl.id, sl.name, COUNT(sli.id), sl.created_at FROM saved_lists sl LEFT JOIN saved_list_items sli ON sli.saved_list_id = sl.id WHERE sl."+whereClause+" GROUP BY sl.id ORDER BY sl.created_at DESC",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your saved lists. Please try again.")
		return
	}
	defer rows.Close()

	type SavedListSummary struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		ItemCount int    `json:"item_count"`
		CreatedAt string `json:"created_at"`
	}

	lists := []SavedListSummary{}
	for rows.Next() {
		var sl SavedListSummary
		if err := rows.Scan(&sl.ID, &sl.Name, &sl.ItemCount, &sl.CreatedAt); err != nil {
			log.Printf("Error scanning saved list row: %v", err)
			continue
		}
		lists = append(lists, sl)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[saved-lists] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, lists)
}

// handleSavedListsCreate handles POST /api/saved-lists
// Saves the current unchecked list items as a new saved list.
func (s *Server) handleSavedListsCreate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	itemsToSave, err := collectListItems(s.db, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to read list")
		return
	}

	if len(itemsToSave) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No items to save"})
		return
	}

	// Create saved list in a transaction so items are all-or-nothing
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "Failed to save list")
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)",
		session.UserID, req.Name,
	)
	if err != nil {
		dbError(w, err, "Failed to save list")
		return
	}
	savedListID, _ := result.LastInsertId()

	if err := insertSavedListItems(tx, savedListID, itemsToSave); err != nil {
		dbError(w, err, "Failed to save list items")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "Failed to save list")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         savedListID,
		"name":       req.Name,
		"item_count": len(itemsToSave),
	})
}

// handleSavedListUpdate handles PUT /api/saved-lists/{id}
// Replaces saved list items with current unchecked list items.
func (s *Server) handleSavedListUpdate(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	if !s.requireSavedListAccess(w, session.UserID, id) {
		return
	}

	itemsToSave, err := collectListItems(s.db, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to read list")
		return
	}

	if len(itemsToSave) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No items to save"})
		return
	}

	// Delete old items and insert new ones in a transaction
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "Failed to update saved list")
		return
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM saved_list_items WHERE saved_list_id = ?", id)
	if err := insertSavedListItems(tx, int64(id), itemsToSave); err != nil {
		dbError(w, err, "Failed to update saved list items")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "Failed to update saved list")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"item_count": len(itemsToSave),
	})
}

// handleSavedListDelete handles DELETE /api/saved-lists/{id}
func (s *Server) handleSavedListDelete(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Only delete if owned by someone in the household
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	deleteArgs := append([]interface{}{id}, whereArgs...)
	result, err := s.db.Exec("DELETE FROM saved_lists WHERE id = ? AND "+whereClause, deleteArgs...)
	if err != nil {
		dbError(w, err, "We couldn't delete that list. Please try again.")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleSavedListLoad handles POST /api/saved-lists/{id}/load
// Upserts saved list items into the active list.
func (s *Server) handleSavedListLoad(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	if !s.requireSavedListAccess(w, session.UserID, id) {
		return
	}

	// If mode=overwrite, clear household's active list first
	if r.URL.Query().Get("mode") == "overwrite" {
		whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
		if _, err := s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...); err != nil {
			log.Printf("Error clearing list for overwrite: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to clear list"})
			return
		}
	}

	// Get saved list items — collect all before upserting to avoid holding
	// the rows cursor open while doing writes (deadlocks with single-conn pools).
	rows, err := s.db.Query(
		"SELECT name, brand, department, quantity, size, notes FROM saved_list_items WHERE saved_list_id = ?",
		id,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load saved list"})
		return
	}

	var savedItems []itemData
	for rows.Next() {
		var si itemData
		if err := rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes); err != nil {
			continue
		}
		savedItems = append(savedItems, si)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[saved-lists] load row iteration error: %v", err)
	}
	rows.Close()

	added := 0
	for _, si := range savedItems {
		_, _, err := upsertListItem(s.db, session.UserID, si.Name, si.Brand, si.Department, si.Size, si.Quantity, "app", "")
		if err != nil {
			log.Printf("Error upserting saved list item %q: %v", si.Name, err)
			continue
		}
		added++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"added":   added,
	})
}

// =============================================================================
// SETTINGS HANDLERS
// =============================================================================

// handleSettings multiplexes GET/PATCH for /api/settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSettingsGet(w, r)
	case http.MethodPatch:
		s.handleSettingsUpdate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSettingsGet handles GET /api/settings
func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var name, email, phone sql.NullString
	err := s.db.QueryRow(
		"SELECT COALESCE(name,''), COALESCE(email,''), phone_number FROM users WHERE id = ?",
		session.UserID,
	).Scan(&name, &email, &phone)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load settings"})
		return
	}

	// Get household members in a single query (avoids N+1)
	type memberInfo struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	var members []memberInfo
	memberRows, err := s.db.Query(
		`SELECT COALESCE(u.name,''), u.phone_number FROM users u
		 JOIN household_members hm ON hm.user_id = u.id
		 WHERE hm.household_id = (SELECT household_id FROM users WHERE id = ?)
		 AND u.id != ?`,
		session.UserID, session.UserID,
	)
	if err == nil {
		defer memberRows.Close()
		for memberRows.Next() {
			var mName, mPhone sql.NullString
			if memberRows.Scan(&mName, &mPhone) == nil {
				members = append(members, memberInfo{Name: mName.String, Phone: mPhone.String})
			}
		}
	}
	if members == nil {
		members = []memberInfo{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":              name.String,
		"email":             email.String,
		"phone":             phone.String,
		"penny_phone":       s.config.TwilioPhoneNumber,
		"household_members": members,
	})
}

// handleSettingsUpdate handles PATCH /api/settings
func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Name != nil {
		if _, err := s.db.Exec("UPDATE users SET name = ? WHERE id = ?", strings.TrimSpace(*req.Name), session.UserID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update name"})
			return
		}
	}

	if req.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*req.Email))
		if email != "" {
			if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email format"})
				return
			}
			// Check uniqueness
			var existingID int
			err := s.db.QueryRow("SELECT id FROM users WHERE email = ? AND id != ?", email, session.UserID).Scan(&existingID)
			if err == nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "Email already in use"})
				return
			}
		}
		if _, err := s.db.Exec("UPDATE users SET email = ? WHERE id = ?", email, session.UserID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update email"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// VCARD ENDPOINT
// =============================================================================

// handleVCard handles GET /api/penny.vcf
func (s *Server) handleVCard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	phoneNumber := s.config.TwilioPhoneNumber
	if phoneNumber == "" {
		phoneNumber = "+10000000000"
	}

	vcard := fmt.Sprintf(`BEGIN:VCARD
VERSION:3.0
N:AI;Penny;;;
FN:Penny AI
ORG:Thriftly
TITLE:Shopping Assistant
TEL;TYPE=CELL:%s
EMAIL:penny@list.thrift.ly
END:VCARD`, phoneNumber)

	w.Header().Set("Content-Type", "text/vcard")
	w.Header().Set("Content-Disposition", "attachment; filename=\"penny.vcf\"")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, vcard)
}

// =============================================================================
// FAMILY INVITE FLOW
// =============================================================================

// handleInvite handles POST /api/settings/invite
func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Phone string `json:"phone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Phone is optional — if provided, validate it
	var e164 string
	hasPhone := strings.TrimSpace(req.Phone) != ""
	if hasPhone {
		var phoneOK bool
		e164, phoneOK = normalizePhone(req.Phone)
		if !phoneOK {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid phone number"})
			return
		}
	}

	// Get user's household
	var householdID sql.NullInt64
	err := s.db.QueryRow("SELECT household_id FROM users WHERE id = ?", session.UserID).Scan(&householdID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No household found"})
		return
	}

	// Auto-create household if user doesn't have one (legacy users before ALTER TABLE)
	if !householdID.Valid {
		hID, err := createSoloHousehold(s.db, session.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create household"})
			return
		}
		householdID = sql.NullInt64{Int64: int64(hID), Valid: true}
	}

	// Generate random invite code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate code"})
		return
	}
	code := hex.EncodeToString(codeBytes)

	// Store invite (7-day expiry)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec(
		"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)",
		householdID.Int64, code, session.UserID, expiresAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create invite"})
		return
	}

	// Send SMS invite only if phone was provided
	if hasPhone {
		smsBody := fmt.Sprintf("Join my Thriftly list! https://app.thrift.ly/join/%s", code)
		if err := s.twilio.SendSMS(s.config.TwilioPhoneNumber, e164, smsBody); err != nil {
			log.Printf("[invite] error sending SMS to %s: %v", e164, err)
			// Still return success — invite is created, SMS delivery is best-effort
		}
	}

	link := fmt.Sprintf("https://app.thrift.ly/join/%s", code)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"code":    code,
		"link":    link,
	})
}

// handleLeaveHousehold handles POST /api/settings/leave-household
func (s *Server) handleLeaveHousehold(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Remove from current household and create a new solo one
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
		return
	}
	if _, err := tx.Exec("UPDATE users SET household_id = NULL WHERE id = ?", session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave household"})
		return
	}

	// Create new solo household (uses its own queries)
	if _, err := createSoloHousehold(s.db, session.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create household"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// handleJoin handles POST /api/join
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Code is required"})
		return
	}

	// Look up invite
	var inviteID int
	var householdID int
	var createdBy int
	var expiresAt time.Time
	var usedBy sql.NullInt64
	err := s.db.QueryRow(
		"SELECT id, household_id, created_by, expires_at, used_by FROM invites WHERE code = ?",
		req.Code,
	).Scan(&inviteID, &householdID, &createdBy, &expiresAt, &usedBy)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "That invite code doesn't look right. Please check and try again."})
		return
	}

	if usedBy.Valid {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has already been used."})
		return
	}

	if time.Now().After(expiresAt) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This invite has expired. Ask for a new one."})
		return
	}

	// Move user to the inviter's household — all in one transaction
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE users SET household_id = ? WHERE id = ?", householdID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}

	if _, err := tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID); err != nil {
		log.Printf("[join] failed to delete old household member: %v", err)
	}
	_, err = tx.Exec(
		"INSERT INTO household_members (household_id, user_id, role, invited_by) VALUES (?, ?, 'member', ?)",
		householdID, session.UserID, createdBy,
	)
	if err != nil {
		log.Printf("[join] error inserting household member: %v", err)
	}

	if _, err := tx.Exec("UPDATE invites SET used_by = ?, used_at = CURRENT_TIMESTAMP WHERE id = ?", session.UserID, inviteID); err != nil {
		log.Printf("[join] failed to mark invite as used: %v", err)
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
	}

	// Award points to inviter for household invite
	if createdBy > 0 {
		s.db.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + 10 WHERE id = ?", createdBy)
		log.Printf("[invite] awarded 10 points to user %d for household invite", createdBy)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// EMAIL INGESTION (SendGrid Inbound Parse)
// =============================================================================

// extractEmail parses an email address from formats like "Name <email>" or bare "email".
func extractEmail(from string) string {
	from = strings.TrimSpace(from)
	if idx := strings.Index(from, "<"); idx >= 0 {
		end := strings.Index(from, ">")
		if end > idx {
			return strings.TrimSpace(from[idx+1 : end])
		}
	}
	return strings.ToLower(from)
}

// handleEmailWebhook handles POST /api/email — SendGrid Inbound Parse webhook.
func (s *Server) handleEmailWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Always return 200 so SendGrid doesn't retry
	defer func() {
		if w.Header().Get("Content-Type") == "" {
			w.WriteHeader(http.StatusOK)
		}
	}()

	// Parse multipart form (SendGrid sends multipart)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// Fall back to regular form parse
		if err := r.ParseForm(); err != nil {
			log.Printf("[email] error parsing form: %v", err)
			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
	}

	from := extractEmail(r.FormValue("from"))
	text := strings.TrimSpace(r.FormValue("text"))

	log.Printf("[email] incoming from %s: %q", from, text)

	if from == "" {
		log.Printf("[email] no from address")
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Look up user by email
	var userID int
	var userPhone string
	err := s.db.QueryRow(
		"SELECT id, phone_number FROM users WHERE LOWER(email) = LOWER(?)",
		from,
	).Scan(&userID, &userPhone)
	if err != nil {
		log.Printf("[email] unknown sender %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Collect all image attachments (max 5 MB each)
	const maxImageSize = 5 * 1024 * 1024
	var images []string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, headers := range r.MultipartForm.File {
			for _, h := range headers {
				if h.Size > maxImageSize {
					log.Printf("[email] skipping oversized image %s (%d bytes)", h.Filename, h.Size)
					continue
				}
				f, err := h.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil || len(data) == 0 {
					continue
				}
				ct := h.Header.Get("Content-Type")
				if strings.HasPrefix(ct, "image/") {
					images = append(images, fmt.Sprintf("data:%s;base64,%s", ct, base64.StdEncoding.EncodeToString(data)))
				}
			}
		}
	}

	if text == "" && len(images) == 0 {
		log.Printf("[email] empty email from %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Parse items with Claude — all images in one shot
	parsed, err := s.parseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[email] error parsing items for %s: %v", from, err)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Add items to user's list
	added, _ := addParsedItems(s.db, userID, parsed, "email")

	log.Printf("[email] added %d items for user %d from %s", len(added), userID, from)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// SMS WEBHOOK
//
// Twilio sends an HTTP POST to /api/sms when someone texts the Twilio number.
// Because Twilio sometimes splits image+text across two rapid webhook calls,
// we buffer messages per phone number for a configurable duration before
// processing them together.
// =============================================================================

// pendingMessage holds accumulated text and image data while we wait for
// additional webhook calls from the same phone number.
type pendingMessage struct {
	text   string
	images []string
	timer  *time.Timer
}

// messageBuffer is a mutex-protected map of phone → pendingMessage.
type messageBuffer struct {
	mu             sync.Mutex
	pending        map[string]*pendingMessage
	bufferDuration time.Duration
}

func newMessageBuffer(d time.Duration) *messageBuffer {
	return &messageBuffer{
		pending:        make(map[string]*pendingMessage),
		bufferDuration: d,
	}
}

// downloadTwilioMedia fetches an image from a Twilio MediaUrl (which requires
// Basic Auth) and returns a data URI string like "data:image/jpeg;base64,...".
func downloadTwilioMedia(mediaURL, contentType, accountSID, authToken string) string {
	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		log.Printf("[sms] error creating media request: %v", err)
		return ""
	}
	req.SetBasicAuth(accountSID, authToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[sms] error downloading media: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// Limit download to 5MB to prevent memory exhaustion
	const maxMediaSize = 5 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+1))
	if err != nil {
		log.Printf("[sms] error reading media body: %v", err)
		return ""
	}
	if len(data) > maxMediaSize {
		log.Printf("[sms] media too large (%d bytes), skipping", len(data))
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

// handleSMSWebhook handles POST /api/sms — the Twilio inbound SMS webhook.
// It buffers incoming messages per phone number and responds with empty TwiML.
func (s *Server) handleSMSWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("[sms] error parsing form: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	from := r.FormValue("From")
	body := strings.TrimSpace(r.FormValue("Body"))
	numMedia, _ := strconv.Atoi(r.FormValue("NumMedia"))

	log.Printf("[sms] incoming from %s: body=%q numMedia=%d", from, body, numMedia)

	// Download all image attachments
	var newImages []string
	for i := 0; i < numMedia; i++ {
		mediaURL := r.FormValue(fmt.Sprintf("MediaUrl%d", i))
		contentType := r.FormValue(fmt.Sprintf("MediaContentType%d", i))
		if mediaURL != "" {
			img := downloadTwilioMedia(
				mediaURL, contentType,
				s.config.TwilioAccountSID, s.config.TwilioAuthToken,
			)
			if img != "" {
				newImages = append(newImages, img)
			}
		}
	}

	// Buffer the message — group rapid-fire webhooks from the same phone
	s.smsBuffer.mu.Lock()
	pm, exists := s.smsBuffer.pending[from]
	if !exists {
		pm = &pendingMessage{}
		s.smsBuffer.pending[from] = pm
	}

	// Append text (with newline separator if both old and new have content)
	if body != "" {
		if pm.text != "" {
			pm.text += "\n" + body
		} else {
			pm.text = body
		}
	}
	// Collect all images
	pm.images = append(pm.images, newImages...)

	// Reset/start the debounce timer
	if pm.timer != nil {
		pm.timer.Stop()
	}
	pm.timer = time.AfterFunc(s.smsBuffer.bufferDuration, func() {
		s.processSMSMessage(from)
	})
	s.smsBuffer.mu.Unlock()

	// Respond with empty TwiML so Twilio doesn't retry
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "<Response></Response>")
}

// processSMSMessage is called by the debounce timer after messages have been
// buffered. It parses items with Claude, adds them to the user's list, and
// sends an SMS reply.
func (s *Server) processSMSMessage(phone string) {
	// Extract and remove the pending message
	s.smsBuffer.mu.Lock()
	pm, exists := s.smsBuffer.pending[phone]
	if !exists {
		s.smsBuffer.mu.Unlock()
		return
	}
	text := pm.text
	images := pm.images
	delete(s.smsBuffer.pending, phone)
	s.smsBuffer.mu.Unlock()

	if text == "" && len(images) == 0 {
		log.Printf("[sms] empty message from %s, skipping", phone)
		return
	}

	// Normalize the phone and look up (or create) the user.
	// Twilio sends E.164 format like "+15551234567" — strip to 10 digits.
	digits := strings.TrimPrefix(phone, "+1")
	user, err := upsertUser(s.db, digits)
	if err != nil {
		log.Printf("[sms] error upserting user for %s: %v", phone, err)
		return
	}

	// Parse items using the same Claude function the web app uses
	parsed, err := s.parseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[sms] error parsing items for %s: %v", phone, err)
		s.sendSMSReply(phone, "Sorry, I had trouble understanding that. Please try again!")
		return
	}

	if len(parsed) == 0 {
		log.Printf("[sms] no items parsed for %s", phone)
		s.sendSMSReply(phone, "I couldn't find any grocery items in your message. Try something like \"milk, eggs, bread\".")
		return
	}

	// Add each parsed item to the user's list
	added, skippedItems := addParsedItems(s.db, user.ID, parsed, "sms")
	var addedNames []string
	for _, item := range added {
		addedNames = append(addedNames, item.Name)
	}
	var updatedNames []string
	for _, si := range skippedItems {
		updatedNames = append(updatedNames, si.Parsed.Name)
	}

	// Build reply message
	var lines []string
	if len(addedNames) > 0 {
		lines = append(lines, "Added to your list:")
		for _, n := range addedNames {
			lines = append(lines, "- "+n)
		}
	}
	if len(updatedNames) > 0 {
		lines = append(lines, "Updated quantity:")
		for _, n := range updatedNames {
			lines = append(lines, "- "+n)
		}
	}

	if len(lines) > 0 {
		s.sendSMSReply(phone, strings.Join(lines, "\n"))
	}
}

// sendSMSReply sends an SMS from the configured Twilio phone number.
func (s *Server) sendSMSReply(to, body string) {
	if s.config.TwilioPhoneNumber == "" {
		log.Printf("[sms] cannot send reply — TWILIO_PHONE_NUMBER not configured")
		return
	}
	if err := s.twilio.SendSMS(s.config.TwilioPhoneNumber, to, body); err != nil {
		log.Printf("[sms] error sending reply to %s: %v", to, err)
	}
}

// =============================================================================
// HELPERS
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[warn] failed to encode JSON response: %v", err)
	}
}

// requireMethod checks that the request uses the expected HTTP method.
// Returns true if the method matches; otherwise writes 405 and returns false.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// dbError logs an error and writes a JSON 500 response. Replaces 15+ identical blocks.
func dbError(w http.ResponseWriter, err error, msg string) {
	log.Printf("%s: %v", msg, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}

// decodeJSON reads a JSON request body into v. Returns false (and writes 400) on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10 MB limit
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		msg := "Invalid request body"
		if err.Error() == "http: request body too large" {
			msg = "Request too large"
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return false
	}
	return true
}

// requireSavedListAccess checks that listID exists and belongs to a member of
// the caller's household. Returns false (and writes an error response) on failure.
func (s *Server) requireSavedListAccess(w http.ResponseWriter, userID, listID int) bool {
	var ownerID int
	err := s.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", listID).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return false
	}
	householdIDs, _ := getHouseholdUserIDs(s.db, userID)
	for _, uid := range householdIDs {
		if uid == ownerID {
			return true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
	return false
}

// collectListItems queries all list_items for the household and returns them.
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

// =============================================================================
// REFERRAL SYSTEM
// =============================================================================

// handleReferral handles GET /api/referral — returns referral code, points, and friends list.
func (s *Server) handleReferral(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get or generate referral code
	var referralCode sql.NullString
	var points int
	err := s.db.QueryRow("SELECT referral_code, COALESCE(referral_points, 0) FROM users WHERE id = ?", session.UserID).Scan(&referralCode, &points)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't load your referral info. Please try again."})
		return
	}

	code := referralCode.String
	if !referralCode.Valid || code == "" {
		// Generate a new referral code
		codeBytes := make([]byte, 16)
		if _, err := rand.Read(codeBytes); err != nil {
			log.Printf("[referral] failed to generate code: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate referral code"})
			return
		}
		code = hex.EncodeToString(codeBytes)
		if _, err := s.db.Exec("UPDATE users SET referral_code = ? WHERE id = ?", code, session.UserID); err != nil {
			log.Printf("[referral] failed to save referral code: %v", err)
		}
	}

	// Get referred friends
	type referredFriend struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	var friends []referredFriend
	rows, err := s.db.Query(
		"SELECT COALESCE(u.name, ''), r.created_at FROM referrals r JOIN users u ON u.id = r.referred_id WHERE r.referrer_id = ? ORDER BY r.created_at DESC",
		session.UserID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f referredFriend
			if rows.Scan(&f.Name, &f.CreatedAt) == nil {
				friends = append(friends, f)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[referral] friends row iteration error: %v", err)
		}
	}
	if friends == nil {
		friends = []referredFriend{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":           code,
		"points":         points,
		"referral_count": len(friends),
		"friends":        friends,
	})
}

// handleReferralRedeem handles POST /api/referral/redeem — redeem 100 points for $10.
func (s *Server) handleReferralRedeem(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var points int
	err := s.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", session.UserID).Scan(&points)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}

	if points < 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You need at least 100 points to redeem."})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE users SET referral_points = referral_points - 100 WHERE id = ?", session.UserID); err != nil {
		log.Printf("[referral] failed to deduct points: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO redemptions (user_id, points, amount_cents) VALUES (?, 100, 1000)", session.UserID); err != nil {
		log.Printf("[referral] failed to insert redemption: %v", err)
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "$10 will be Venmo'd to your number!"})
}

// handleReferralClaim handles POST /api/referral/claim — called on signup with a referral code.
func (s *Server) handleReferralClaim(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Referral code is required."})
		return
	}

	// Look up referrer by code
	var referrerID int
	err := s.db.QueryRow("SELECT id FROM users WHERE referral_code = ?", req.Code).Scan(&referrerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Invalid referral code."})
		return
	}

	// Don't let users refer themselves
	if referrerID == session.UserID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "You can't use your own referral code."})
		return
	}

	// Idempotent — check if already claimed
	var existingID int
	err = s.db.QueryRow("SELECT id FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, session.UserID).Scan(&existingID)
	if err == nil {
		// Already claimed — return success without double-crediting
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, session.UserID); err != nil {
		log.Printf("[referral] failed to insert referral: %v", err)
	}
	if _, err := tx.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + 10 WHERE id = ?", referrerID); err != nil {
		log.Printf("[referral] failed to update points: %v", err)
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleNPSCheck handles GET /api/nps/check — returns whether the NPS survey should show.
func (s *Server) handleNPSCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var loginCount int
	if err := s.db.QueryRow("SELECT COALESCE(login_count, 0) FROM users WHERE id = ?", session.UserID).Scan(&loginCount); err != nil {
		log.Printf("[nps] error checking login count for user %d: %v", session.UserID, err)
	}
	if loginCount < 3 {
		writeJSON(w, http.StatusOK, map[string]bool{"show_nps": false})
		return
	}

	// Check if user has a recent NPS response
	intervalDays := fmt.Sprintf("-%d days", s.config.NPSIntervalDays)
	var recentCount int
	if err := s.db.QueryRow(
		"SELECT COUNT(*) FROM nps_responses WHERE user_id = ? AND created_at >= datetime('now', ?)",
		session.UserID, intervalDays,
	).Scan(&recentCount); err != nil {
		log.Printf("[nps] error checking recent responses for user %d: %v", session.UserID, err)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"show_nps": recentCount == 0})
}

// handleNPS handles POST /api/nps — submits an NPS response.
func (s *Server) handleNPS(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Score   int    `json:"score"`
		Comment string `json:"comment"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Score < 0 || req.Score > 10 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Score must be 0-10"})
		return
	}
	if len(req.Comment) > 500 {
		req.Comment = req.Comment[:500]
	}
	req.Comment = strings.TrimSpace(req.Comment)

	_, err := s.db.Exec(
		"INSERT INTO nps_responses (user_id, score, comment) VALUES (?, ?, ?)",
		session.UserID, req.Score, req.Comment,
	)
	if err != nil {
		log.Printf("[nps] error inserting response: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save response"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// MAIN
// =============================================================================

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func main() {
	cfg := loadConfig()

	if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioVerifyService == "" {
		log.Fatal(`
Missing Twilio credentials. Set these environment variables:

  export TWILIO_ACCOUNT_SID=ACxxxxxxxx
  export TWILIO_AUTH_TOKEN=xxxxxxxx
  export TWILIO_VERIFY_SERVICE_SID=VAxxxxxxxx

Get these from https://console.twilio.com (see README for full setup).
`)
	}

	// Open the database (creates the file if it doesn't exist)
	db, err := initDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	server := &Server{
		config:   cfg,
		db:       db,
		sessions: NewSessionStore(db),
		twilio: &RealTwilioClient{
			AccountSID:    cfg.TwilioAccountSID,
			AuthToken:     cfg.TwilioAuthToken,
			VerifyService: cfg.TwilioVerifyService,
		},
		parseItems: parseItemsWithClaude,
		smsBuffer:  newMessageBuffer(10 * time.Second),
	}

	// Clean expired sessions every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			server.sessions.CleanExpired()
		}
	}()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	http.HandleFunc("/auth/send-code", server.handleSendCode)
	http.HandleFunc("/auth/verify-code", server.handleVerifyCode)
	http.HandleFunc("/auth/logout", server.handleLogout)
	http.HandleFunc("/auth/me", server.handleMe)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
	http.HandleFunc("/api/list/clear", server.handleListClear)
	http.HandleFunc("/api/list/restore", server.handleListRestore)
	http.HandleFunc("/api/saved-lists", server.handleSavedLists)
	http.HandleFunc("/api/saved-lists/", server.handleSavedLists)
	http.HandleFunc("/api/settings", server.handleSettings)
	http.HandleFunc("/api/settings/invite", server.handleInvite)
	http.HandleFunc("/api/settings/leave-household", server.handleLeaveHousehold)
	http.HandleFunc("/api/penny.vcf", server.handleVCard)
	http.HandleFunc("/api/join", server.handleJoin)
	http.HandleFunc("/api/email", server.handleEmailWebhook)
	http.HandleFunc("/api/sms", server.handleSMSWebhook)
	http.HandleFunc("/api/referral", server.handleReferral)
	http.HandleFunc("/api/referral/redeem", server.handleReferralRedeem)
	http.HandleFunc("/api/referral/claim", server.handleReferralClaim)
	http.HandleFunc("/api/nps/check", server.handleNPSCheck)
	http.HandleFunc("/api/nps", server.handleNPS)
	http.HandleFunc("/api/admin/dashboard", server.handleAdminDashboard)
	http.HandleFunc("/api/brands", server.handleBrands)
	http.HandleFunc("/api/logo", server.handleLogoProxy)
	http.HandleFunc("/api/trips", server.handleTrips)
	http.HandleFunc("/privacy", handlePrivacyPolicy)
	http.HandleFunc("/terms", handleTerms)

	// Serve the React frontend. The custom handler supports SPA routing:
	// known files are served directly, everything else gets index.html.
	// /r/{code} referral links also serve the SPA (frontend reads the code).
	fs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat("static" + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "static/index.html")
	})

	// Wrap with panic recovery and request/response logging
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("[panic] %s %s: %v", r.Method, r.URL.Path, rv)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		log.Printf("[req] %s %s (Content-Length: %d)", r.Method, r.URL.Path, r.ContentLength)
		rec := &statusRecorder{ResponseWriter: w, statusCode: 200}
		http.DefaultServeMux.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("[res] %s %s → %d", r.Method, r.URL.Path, rec.statusCode)
		}
	})

	go server.warmLogoCache()

	log.Printf("thriftly is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
