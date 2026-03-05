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
	Port                string // HTTP port (default "8080")
	DBPath              string // SQLite file path (default "thriftly.db")
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
	return Config{
		TwilioAccountSID:    os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:     os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioVerifyService: os.Getenv("TWILIO_VERIFY_SERVICE_SID"),
		TwilioPhoneNumber:   os.Getenv("TWILIO_PHONE_NUMBER"),
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AdminPhone:          os.Getenv("ADMIN_PHONE"),
		Port:                port,
		DBPath:              dbPath,
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
			FOREIGN KEY (item_id) REFERENCES list_items(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("creating item_modifications table: %w", err)
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

	return db, nil
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

	// Track login activity
	_, _ = tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE id = ?", user.ID)

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

// itemData holds the 5 fields shared between list_items and saved_list_items.
// Used by collectListItems and saved-list handlers.
type itemData struct {
	Name, Brand, Department, Size string
	Quantity                      int
}

// ListItem represents a row in the list_items table.
type ListItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Brand        string `json:"brand"`
	Department   string `json:"department"`
	Quantity     int    `json:"quantity"`
	Size         string `json:"size"`
	Source       string `json:"source"`
	Checked      bool   `json:"checked"`
	CreatedAt    string `json:"created_at"`
	AddedByName  string `json:"added_by_name"`
	AddedByPhone string `json:"added_by_phone"`
}

// =============================================================================
// SESSION STORE
//
// Keeps track of who's logged in. A session maps a random token (in a browser
// cookie) to a user ID and phone number (on the server).
//
// Currently in-memory — server restart = everyone logs out.
// Good enough for local development. We'll move to SQLite later.
//
// sync.RWMutex prevents data corruption when multiple requests arrive
// at the same time:
//   - RLock/RUnlock: many readers can read simultaneously
//   - Lock/Unlock: one writer at a time, blocks all others
// =============================================================================

type Session struct {
	UserID      int       // the user's database ID
	PhoneNumber string    // 10-digit format: "5551234567"
	CreatedAt   time.Time // when this session was created
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session // token → session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]Session),
	}
}

// CreateSession generates a random 64-char hex token, stores the session,
// and returns the token.
func (s *SessionStore) CreateSession(userID int, phone string) (string, error) {
	b := make([]byte, 32) // 32 bytes = 256 bits of randomness
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	s.sessions[token] = Session{
		UserID:      userID,
		PhoneNumber: phone,
		CreatedAt:   time.Now(),
	}
	s.mu.Unlock()

	return token, nil
}

func (s *SessionStore) GetSession(token string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[token]
	return session, ok
}

func (s *SessionStore) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
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

type Server struct {
	config     Config
	db         *sql.DB
	sessions   *SessionStore
	twilio     TwilioClient
	parseItems func(apiKey, text string, images []string) ([]ParsedItem, error)
	smsBuffer  *messageBuffer
}

// handleSendCode handles POST /auth/send-code
// Request:  { "phone_number": "5551234567" }
// Response: { "success": true }
func (s *Server) handleSendCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		SameSite: http.SameSiteLaxMode, // CSRF protection
		MaxAge:   60 * 60 * 24 * 30,    // 30 days
	})

	log.Printf("User %d (%s) logged in", user.ID, digits)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"user_id":      user.ID,
		"phone_number": digits,
	})
}

// handleLogout handles POST /auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if cookie, err := r.Cookie("session"); err == nil {
		s.sessions.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleMe handles GET /auth/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

// getItemByID fetches a single list item belonging to a household.
func getItemByID(db *sql.DB, id, userID int) (ListItem, error) {
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	args := append([]interface{}{id}, whereArgs...)
	var item ListItem
	var checked int
	var addedByName, addedByPhone sql.NullString
	err := db.QueryRow(
		"SELECT li.id, li.name, li.brand, li.department, li.quantity, li.size, li.source, li.checked, li.created_at, COALESCE(u.name,''), u.phone_number FROM list_items li JOIN users u ON u.id = li.user_id WHERE li.id = ? AND li."+whereClause,
		args...,
	).Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Source, &checked, &item.CreatedAt, &addedByName, &addedByPhone)
	if err != nil {
		return ListItem{}, err
	}
	item.Checked = checked == 1
	item.AddedByName = addedByName.String
	item.AddedByPhone = addedByPhone.String
	return item, nil
}

// upsertListItem adds a new item or merges into an existing unchecked item
// with the same name (case-insensitive) across the household. Returns the
// resulting ListItem and whether it was newly created.
func upsertListItem(db *sql.DB, userID int, name, brand, department, size string, quantity int, source string) (ListItem, bool, error) {
	if quantity < 1 {
		quantity = 1
	}
	if department == "" {
		department = "Other"
	}

	// Look for an existing unchecked item with the same name across household
	whereClause, whereArgs := householdItemsWhereClause(db, userID)
	queryArgs := append(whereArgs, name)
	var existingID int
	var existingQty int
	var existingBrand, existingSize string
	err := db.QueryRow(
		"SELECT id, quantity, brand, size FROM list_items WHERE "+whereClause+" AND LOWER(name) = LOWER(?) AND checked = 0",
		queryArgs...,
	).Scan(&existingID, &existingQty, &existingBrand, &existingSize)

	if err == nil {
		// Existing unchecked item found — merge quantity
		newQty := existingQty + quantity
		newBrand := existingBrand
		if newBrand == "" && brand != "" {
			newBrand = brand
		}
		newSize := existingSize
		if newSize == "" && size != "" {
			newSize = size
		}
		_, err := db.Exec(
			"UPDATE list_items SET quantity = ?, brand = ?, size = ? WHERE id = ?",
			newQty, newBrand, newSize, existingID,
		)
		if err != nil {
			return ListItem{}, false, fmt.Errorf("updating existing item: %w", err)
		}
		detail := fmt.Sprintf("quantity %d → %d", existingQty, newQty)
		recordModification(db, existingID, userID, "quantity_changed", detail, source)
		item, err := getItemByID(db, existingID, userID)
		// For SMS/email sources, report as "added" (not skipped) since the user
		// can't interactively dedupe via one-way communication.
		if source == "sms" || source == "email" {
			return item, true, err
		}
		return item, false, err
	}

	// No match — insert new (attributed to the requesting user)
	if source == "" {
		source = "app"
	}
	result, err := db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, ?, ?, ?, ?, ?, ?)",
		userID, name, brand, department, quantity, size, source,
	)
	if err != nil {
		return ListItem{}, false, fmt.Errorf("inserting item: %w", err)
	}
	id, _ := result.LastInsertId()
	recordModification(db, int(id), userID, "added", "", source)
	item, err := getItemByID(db, int(id), userID)
	return item, true, err
}

// addParsedItems loops over parsed items, skips empties, and upserts each one.
// Returns the list of added items and skipped items (already on the list).
func addParsedItems(db *sql.DB, userID int, parsed []ParsedItem, source string) (added []ListItem, skipped []SkippedItem) {
	for _, p := range parsed {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		item, isNew, err := upsertListItem(db, userID, p.Name, p.Brand, p.Department, p.Size, p.Quantity, source)
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
}

// parseItemsWithClaude sends the user's raw text (and optional image) to Claude
// and gets back a list of grocery items with departments assigned.
func parseItemsWithClaude(apiKey, userText string, images []string) ([]ParsedItem, error) {
	systemPrompt := `You are a grocery shopping assistant. Parse the user's input into individual grocery items.

For each item return:
- "name": the generic product name, properly capitalized, WITHOUT the brand (e.g. "Peanut Butter", "Whole Milk", "Chicken Breast")
- "brand": brand name if mentioned, otherwise "" (e.g. "Jif", "Heinz", "Organic Valley")
- "department": one of: Produce, Dairy & Eggs, Meat & Seafood, Bakery, Frozen, Pantry, Beverages, Household, Other
- "quantity": number of units to buy, integer, default 1
- "size": package size/weight if specified, otherwise "" (e.g. "16 oz", "1 lb", "Large", "1 gal")

Separate brand from product name. Examples:
- "jif pb" → {"name":"Peanut Butter","brand":"Jif","department":"Pantry","quantity":1,"size":""}
- "2 boxes kraft mac and cheese" → {"name":"Macaroni & Cheese","brand":"Kraft","department":"Pantry","quantity":2,"size":""}
- "large organic eggs" → {"name":"Eggs","brand":"","department":"Dairy & Eggs","quantity":1,"size":"Large"}
- "gallon of whole milk" → {"name":"Whole Milk","brand":"","department":"Dairy & Eggs","quantity":1,"size":"1 gal"}

Image handling — adapt to what you see:
- Grocery items or products: extract each item.
- Handwritten or printed shopping list: transcribe each item.
- Recipe (on paper, screen, or in text): extract the ingredients as grocery items.
- Fridge, pantry, or cupboard photo: use the user's text as context (e.g. "what am I missing", "what's running low") to suggest items that appear to be low or absent. If no text is provided, list items that appear to be running low.

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
			textContent = "Identify all grocery items in these images. If it's a recipe, extract the ingredients. If it's a fridge or pantry, list items that appear to be running low or missing."
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
		"max_tokens": 1024,
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
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
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
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	s.db.QueryRow("SELECT COUNT(DISTINCT id) FROM users WHERE last_active >= datetime('now', ?)", modifier).Scan(&activeUsers)
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= datetime('now', ?)", modifier).Scan(&newUsers)

	var referralsInPeriod int
	s.db.QueryRow("SELECT COUNT(*) FROM referrals WHERE created_at >= datetime('now', ?)", modifier).Scan(&referralsInPeriod)

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
		SELECT u.phone_number, COALESCE(u.name,''), COALESCE(u.referral_points,0),
		       (SELECT COUNT(*) FROM referrals WHERE referrer_id = u.id) as ref_count
		FROM users u
		WHERE (SELECT COUNT(*) FROM referrals WHERE referrer_id = u.id) > 0
		ORDER BY COALESCE(u.referral_points,0) DESC
		LIMIT 20
	`)
	var leaderboard []leaderEntry
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e leaderEntry
			if err := rows.Scan(&e.Phone, &e.Name, &e.Points, &e.Referrals); err == nil {
				// Mask phone: 310***1833
				if len(e.Phone) == 10 {
					e.Phone = e.Phone[:3] + "***" + e.Phone[6:]
				}
				leaderboard = append(leaderboard, e)
			}
		}
	}
	if leaderboard == nil {
		leaderboard = []leaderEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_users": activeUsers,
		"total_users":  totalUsers,
		"new_users":    newUsers,
		"k_factor":     kFactor,
		"leaderboard":  leaderboard,
		"days":         days,
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
		Text  string `json:"text"`
		Image string `json:"image"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	req.Image = strings.TrimSpace(req.Image)
	if req.Text == "" && req.Image == "" {
		log.Printf("[parse] empty text and image")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Text or image is required"})
		return
	}
	if s.config.AnthropicAPIKey == "" {
		log.Printf("[parse] no API key configured")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
		return
	}

	log.Printf("Parsing grocery text for user %d: %q (image: %v, image_len: %d)", session.UserID, req.Text, req.Image != "", len(req.Image))

	var images []string
	if req.Image != "" {
		images = []string{req.Image}
	}
	parsed, err := s.parseItems(s.config.AnthropicAPIKey, req.Text, images)
	if err != nil {
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
		"SELECT li.id, li.name, li.brand, li.department, li.quantity, li.size, li.source, li.checked, li.created_at, COALESCE(u.name,''), u.phone_number FROM list_items li JOIN users u ON u.id = li.user_id WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your list. Please try again.")
		return
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		var item ListItem
		var checked int
		var addedByName, addedByPhone sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &item.Source, &checked, &item.CreatedAt, &addedByName, &addedByPhone); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		item.Checked = checked == 1
		item.AddedByName = addedByName.String
		item.AddedByPhone = addedByPhone.String
		items = append(items, item)
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

	item, _, err := upsertListItem(s.db, session.UserID, req.Name, "", req.Department, "", 1, "app")
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
	// Delete modifications first to avoid FK constraint violation
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	// Delete modifications first to avoid FK constraint violation
	s.db.Exec("DELETE FROM item_modifications WHERE item_id IN (SELECT id FROM list_items WHERE "+whereClause+")", whereArgs...)
	_, err := s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...)
	if err != nil {
		dbError(w, err, "We couldn't clear your list. Please try again.")
		return
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

	// Create saved list
	result, err := s.db.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)",
		session.UserID, req.Name,
	)
	if err != nil {
		dbError(w, err, "Failed to save list")
		return
	}
	savedListID, _ := result.LastInsertId()

	// Insert items
	for _, d := range itemsToSave {
		_, err := s.db.Exec(
			"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
			savedListID, d.Name, d.Brand, d.Department, d.Quantity, d.Size,
		)
		if err != nil {
			log.Printf("Error inserting saved list item: %v", err)
		}
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

	// Delete old items and insert new ones
	s.db.Exec("DELETE FROM saved_list_items WHERE saved_list_id = ?", id)
	for _, d := range itemsToSave {
		s.db.Exec(
			"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
			id, d.Name, d.Brand, d.Department, d.Quantity, d.Size,
		)
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
		// Delete modifications first to avoid FK constraint violation
		s.db.Exec("DELETE FROM item_modifications WHERE item_id IN (SELECT id FROM list_items WHERE "+whereClause+")", whereArgs...)
		if _, err := s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...); err != nil {
			log.Printf("Error clearing list for overwrite: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to clear list"})
			return
		}
	}

	// Get saved list items — collect all before upserting to avoid holding
	// the rows cursor open while doing writes (deadlocks with single-conn pools).
	rows, err := s.db.Query(
		"SELECT name, brand, department, quantity, size FROM saved_list_items WHERE saved_list_id = ?",
		id,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load saved list"})
		return
	}

	var savedItems []itemData
	for rows.Next() {
		var si itemData
		if err := rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size); err != nil {
			continue
		}
		savedItems = append(savedItems, si)
	}
	rows.Close()

	added := 0
	for _, si := range savedItems {
		_, _, err := upsertListItem(s.db, session.UserID, si.Name, si.Brand, si.Department, si.Size, si.Quantity, "app")
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

	// Get household members
	householdIDs, _ := getHouseholdUserIDs(s.db, session.UserID)
	type memberInfo struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	var members []memberInfo
	for _, uid := range householdIDs {
		if uid == session.UserID {
			continue
		}
		var mName, mPhone sql.NullString
		err := s.db.QueryRow("SELECT COALESCE(name,''), phone_number FROM users WHERE id = ?", uid).Scan(&mName, &mPhone)
		if err == nil {
			members = append(members, memberInfo{Name: mName.String, Phone: mPhone.String})
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
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID)
	tx.Exec("UPDATE users SET household_id = NULL WHERE id = ?", session.UserID)

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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	tx.Exec("DELETE FROM household_members WHERE user_id = ?", session.UserID)
	_, err = tx.Exec(
		"INSERT INTO household_members (household_id, user_id, role, invited_by) VALUES (?, ?, 'member', ?)",
		householdID, session.UserID, createdBy,
	)
	if err != nil {
		log.Printf("[join] error inserting household member: %v", err)
	}

	tx.Exec("UPDATE invites SET used_by = ?, used_at = CURRENT_TIMESTAMP WHERE id = ?", session.UserID, inviteID)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't join you to this household. Please try again."})
		return
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	// Collect all image attachments
	var images []string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, headers := range r.MultipartForm.File {
			for _, h := range headers {
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[sms] error reading media body: %v", err)
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

// handleSMSWebhook handles POST /api/sms — the Twilio inbound SMS webhook.
// It buffers incoming messages per phone number and responds with empty TwiML.
func (s *Server) handleSMSWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	json.NewEncoder(w).Encode(data)
}

// dbError logs an error and writes a JSON 500 response. Replaces 15+ identical blocks.
func dbError(w http.ResponseWriter, err error, msg string) {
	log.Printf("%s: %v", msg, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}

// decodeJSON reads a JSON request body into v. Returns false (and writes 400) on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
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
		"SELECT name, brand, department, quantity, size FROM list_items WHERE "+whereClause,
		whereArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []itemData
	for rows.Next() {
		var d itemData
		if err := rows.Scan(&d.Name, &d.Brand, &d.Department, &d.Quantity, &d.Size); err != nil {
			continue
		}
		items = append(items, d)
	}
	return items, nil
}

// =============================================================================
// REFERRAL SYSTEM
// =============================================================================

// handleReferral handles GET /api/referral — returns referral code, points, and friends list.
func (s *Server) handleReferral(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		rand.Read(codeBytes)
		code = hex.EncodeToString(codeBytes)
		s.db.Exec("UPDATE users SET referral_code = ? WHERE id = ?", code, session.UserID)
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	tx.Exec("UPDATE users SET referral_points = referral_points - 100 WHERE id = ?", session.UserID)
	tx.Exec("INSERT INTO redemptions (user_id, points, amount_cents) VALUES (?, 100, 1000)", session.UserID)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "$10 will be Venmo'd to your number!"})
}

// handleReferralClaim handles POST /api/referral/claim — called on signup with a referral code.
func (s *Server) handleReferralClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	tx.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, session.UserID)
	tx.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + 10 WHERE id = ?", referrerID)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
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
		sessions: NewSessionStore(),
		twilio: &RealTwilioClient{
			AccountSID:    cfg.TwilioAccountSID,
			AuthToken:     cfg.TwilioAuthToken,
			VerifyService: cfg.TwilioVerifyService,
		},
		parseItems: parseItemsWithClaude,
		smsBuffer:  newMessageBuffer(10 * time.Second),
	}

	http.HandleFunc("/auth/send-code", server.handleSendCode)
	http.HandleFunc("/auth/verify-code", server.handleVerifyCode)
	http.HandleFunc("/auth/logout", server.handleLogout)
	http.HandleFunc("/auth/me", server.handleMe)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
	http.HandleFunc("/api/list/clear", server.handleListClear)
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
	http.HandleFunc("/api/admin/dashboard", server.handleAdminDashboard)
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

	// Wrap with request/response logging
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[req] %s %s (Content-Length: %d)", r.Method, r.URL.Path, r.ContentLength)
		rec := &statusRecorder{ResponseWriter: w, statusCode: 200}
		http.DefaultServeMux.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("[res] %s %s → %d", r.Method, r.URL.Path, rec.statusCode)
		}
	})

	log.Printf("thriftly is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
