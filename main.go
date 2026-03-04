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
	AnthropicAPIKey     string // starts with "sk-ant-"
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
		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
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

	return db, nil
}

// User represents a row in the users table.
type User struct {
	ID          int
	PhoneNumber string
	CreatedAt   string
}

// upsertUser ensures a user exists for this phone number and returns them.
// If the phone is new, it creates a row. If it already exists, it just
// looks up the existing row. Either way, you get back a User.
func upsertUser(db *sql.DB, phone string) (User, error) {
	// INSERT OR IGNORE: if phone_number already exists (UNIQUE constraint),
	// this silently does nothing instead of returning an error.
	_, err := db.Exec(
		"INSERT OR IGNORE INTO users (phone_number) VALUES (?)",
		phone,
	)
	if err != nil {
		return User{}, fmt.Errorf("upserting user: %w", err)
	}

	// Now SELECT the user — whether they were just created or already existed.
	var user User
	err = db.QueryRow(
		"SELECT id, phone_number, created_at FROM users WHERE phone_number = ?",
		phone,
	).Scan(&user.ID, &user.PhoneNumber, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user: %w", err)
	}

	return user, nil
}

// ListItem represents a row in the list_items table.
type ListItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Brand      string `json:"brand"`
	Department string `json:"department"`
	Quantity   int    `json:"quantity"`
	Size       string `json:"size"`
	Checked    bool   `json:"checked"`
	CreatedAt  string `json:"created_at"`
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
}

type RealTwilioClient struct {
	AccountSID    string
	AuthToken     string
	VerifyService string
}

// SendVerification asks Twilio to text an OTP code to the phone number.
func (t *RealTwilioClient) SendVerification(phone string) error {
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/Verifications",
		t.VerifyService,
	)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("Channel", "sms")

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling Twilio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CheckVerification asks Twilio if the user-entered code is correct.
func (t *RealTwilioClient) CheckVerification(phone, code string) (bool, error) {
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/VerificationCheck",
		t.VerifyService,
	)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("Code", code)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling Twilio: %w", err)
	}
	defer resp.Body.Close()

	// If Twilio returns an error HTTP status (like 404 for expired codes),
	// treat it as "not approved" rather than crashing on the JSON parse.
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Twilio verify returned %d: %s", resp.StatusCode, string(body))
		return false, nil
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("parsing response: %w", err)
	}
	return result.Status == "approved", nil
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
	parseItems func(apiKey, text, imageBase64 string) ([]ParsedItem, error)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
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
		log.Printf("Error sending code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to send verification code",
		})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
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
		log.Printf("Error checking code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to verify code",
		})
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
		log.Printf("Error upserting user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create user",
		})
		return
	}

	token, err := s.sessions.CreateSession(user.ID, digits)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to create session",
		})
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
	})
}

// getItemByID fetches a single list item belonging to a user.
func getItemByID(db *sql.DB, id, userID int) (ListItem, error) {
	var item ListItem
	var checked int
	err := db.QueryRow(
		"SELECT id, name, brand, department, quantity, size, checked, created_at FROM list_items WHERE id = ? AND user_id = ?",
		id, userID,
	).Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &checked, &item.CreatedAt)
	if err != nil {
		return ListItem{}, err
	}
	item.Checked = checked == 1
	return item, nil
}

// upsertListItem adds a new item or merges into an existing unchecked item
// with the same name (case-insensitive). Returns the resulting ListItem and
// whether it was newly created.
func upsertListItem(db *sql.DB, userID int, name, brand, department, size string, quantity int) (ListItem, bool, error) {
	if quantity < 1 {
		quantity = 1
	}
	if department == "" {
		department = "Other"
	}

	// Look for an existing unchecked item with the same name
	var existingID int
	var existingQty int
	var existingBrand, existingSize string
	err := db.QueryRow(
		"SELECT id, quantity, brand, size FROM list_items WHERE user_id = ? AND LOWER(name) = LOWER(?) AND checked = 0",
		userID, name,
	).Scan(&existingID, &existingQty, &existingBrand, &existingSize)

	if err == nil {
		// Existing unchecked item found — merge
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
		item, err := getItemByID(db, existingID, userID)
		return item, false, err
	}

	// No match — insert new
	result, err := db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, name, brand, department, quantity, size,
	)
	if err != nil {
		return ListItem{}, false, fmt.Errorf("inserting item: %w", err)
	}
	id, _ := result.LastInsertId()
	item, err := getItemByID(db, int(id), userID)
	return item, true, err
}

// findExistingItem checks if an unchecked item with the same name (case-insensitive)
// already exists for this user. Returns the existing item ID and true if found.
func findExistingItem(db *sql.DB, userID int, name string) (int, bool) {
	var existingID int
	err := db.QueryRow(
		"SELECT id FROM list_items WHERE user_id = ? AND LOWER(name) = LOWER(?) AND checked = 0",
		userID, name,
	).Scan(&existingID)
	return existingID, err == nil
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
func parseItemsWithClaude(apiKey, userText, imageBase64 string) ([]ParsedItem, error) {
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

If the user provides an image, identify all grocery items visible and parse them.

Return ONLY a JSON array, no markdown, no code fences, no explanation.`

	// Build message content — either plain text or multi-block (image + text)
	var messageContent interface{}
	if imageBase64 != "" {
		// Extract media type and raw base64 from data URI
		mediaType := "image/jpeg"
		b64Data := imageBase64
		if strings.HasPrefix(imageBase64, "data:") {
			if idx := strings.Index(imageBase64, ";base64,"); idx > 0 {
				mediaType = imageBase64[5:idx]
				b64Data = imageBase64[idx+8:]
			}
		}

		contentBlocks := []map[string]interface{}{
			{
				"type": "image",
				"source": map[string]string{
					"type":       "base64",
					"media_type": mediaType,
					"data":       b64Data,
				},
			},
		}
		textContent := userText
		if textContent == "" {
			textContent = "What grocery items do you see in this image?"
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

	if tail == "clear" && r.Method == http.MethodDelete {
		s.handleListClear(w, r)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[parse] body decode error: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
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

	parsed, err := s.parseItems(s.config.AnthropicAPIKey, req.Text, req.Image)
	if err != nil {
		log.Printf("Error parsing with Claude: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse items"})
		return
	}

	resp := ParseResponse{Items: []ListItem{}, Skipped: []SkippedItem{}}
	for _, p := range parsed {
		if strings.TrimSpace(p.Name) == "" {
			continue
		}
		qty := p.Quantity
		if qty < 1 {
			qty = 1
		}

		// Check if item already exists on the list
		if existingID, found := findExistingItem(s.db, session.UserID, p.Name); found {
			resp.Skipped = append(resp.Skipped, SkippedItem{
				Parsed:     p,
				ExistingID: existingID,
			})
			continue
		}

		// New item — insert
		dept := p.Department
		if dept == "" {
			dept = "Other"
		}
		result, err := s.db.Exec(
			"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
			session.UserID, p.Name, p.Brand, dept, qty, p.Size,
		)
		if err != nil {
			log.Printf("Error inserting parsed item %q: %v", p.Name, err)
			continue
		}
		id, _ := result.LastInsertId()
		if item, err := getItemByID(s.db, int(id), session.UserID); err == nil {
			resp.Items = append(resp.Items, item)
			resp.Added++
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}

// handleListGet handles GET /api/list
func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(
		"SELECT id, name, brand, department, quantity, size, checked, created_at FROM list_items WHERE user_id = ? ORDER BY name",
		session.UserID,
	)
	if err != nil {
		log.Printf("Error querying list: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load list"})
		return
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		var item ListItem
		var checked int
		if err := rows.Scan(&item.ID, &item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Size, &checked, &item.CreatedAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		item.Checked = checked == 1
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
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

	item, _, err := upsertListItem(s.db, session.UserID, req.Name, "", req.Department, "", 1)
	if err != nil {
		log.Printf("Error upserting item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to add item"})
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

	_, err := s.db.Exec(
		"UPDATE list_items SET checked = NOT checked WHERE id = ? AND user_id = ?",
		id, session.UserID,
	)
	if err != nil {
		log.Printf("Error toggling item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Item not found"})
		return
	}

	writeJSON(w, http.StatusOK, item)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name cannot be empty"})
			return
		}
		if _, err := s.db.Exec("UPDATE list_items SET name = ? WHERE id = ? AND user_id = ?", name, id, session.UserID); err != nil {
			log.Printf("Error updating item name: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
			return
		}
	}
	if req.Brand != nil {
		if _, err := s.db.Exec("UPDATE list_items SET brand = ? WHERE id = ? AND user_id = ?", *req.Brand, id, session.UserID); err != nil {
			log.Printf("Error updating item brand: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
			return
		}
	}
	if req.Department != nil {
		if _, err := s.db.Exec("UPDATE list_items SET department = ? WHERE id = ? AND user_id = ?", *req.Department, id, session.UserID); err != nil {
			log.Printf("Error updating item department: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
			return
		}
	}
	if req.Quantity != nil {
		qty := *req.Quantity
		if qty < 1 {
			qty = 1
		}
		if _, err := s.db.Exec("UPDATE list_items SET quantity = ? WHERE id = ? AND user_id = ?", qty, id, session.UserID); err != nil {
			log.Printf("Error updating item quantity: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
			return
		}
	}
	if req.Size != nil {
		if _, err := s.db.Exec("UPDATE list_items SET size = ? WHERE id = ? AND user_id = ?", *req.Size, id, session.UserID); err != nil {
			log.Printf("Error updating item size: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
			return
		}
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

	_, err := s.db.Exec(
		"DELETE FROM list_items WHERE id = ? AND user_id = ?",
		id, session.UserID,
	)
	if err != nil {
		log.Printf("Error deleting item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete item"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleListClear handles DELETE /api/list/clear
func (s *Server) handleListClear(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	_, err := s.db.Exec("DELETE FROM list_items WHERE user_id = ?", session.UserID)
	if err != nil {
		log.Printf("Error clearing list: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to clear list"})
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

	rows, err := s.db.Query(`
		SELECT sl.id, sl.name, COUNT(sli.id), sl.created_at
		FROM saved_lists sl
		LEFT JOIN saved_list_items sli ON sli.saved_list_id = sl.id
		WHERE sl.user_id = ?
		GROUP BY sl.id
		ORDER BY sl.created_at DESC
	`, session.UserID)
	if err != nil {
		log.Printf("Error querying saved lists: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load saved lists"})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	// Get all items — close rows before writing to avoid deadlock
	rows, err := s.db.Query(
		"SELECT name, brand, department, quantity, size FROM list_items WHERE user_id = ?",
		session.UserID,
	)
	if err != nil {
		log.Printf("Error querying list items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read list"})
		return
	}

	type itemData struct {
		Name, Brand, Department, Size string
		Quantity                       int
	}
	var itemsToSave []itemData
	for rows.Next() {
		var d itemData
		if err := rows.Scan(&d.Name, &d.Brand, &d.Department, &d.Quantity, &d.Size); err != nil {
			continue
		}
		itemsToSave = append(itemsToSave, d)
	}
	rows.Close()

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
		log.Printf("Error creating saved list: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save list"})
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

	// Verify ownership
	var ownerID int
	err := s.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", id).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}
	if ownerID != session.UserID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}

	// Get all items — close rows before writing to avoid deadlock
	rows, err := s.db.Query(
		"SELECT name, brand, department, quantity, size FROM list_items WHERE user_id = ?",
		session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read list"})
		return
	}

	type itemData struct {
		Name, Brand, Department, Size string
		Quantity                       int
	}
	var itemsToSave []itemData
	for rows.Next() {
		var d itemData
		if err := rows.Scan(&d.Name, &d.Brand, &d.Department, &d.Quantity, &d.Size); err != nil {
			continue
		}
		itemsToSave = append(itemsToSave, d)
	}
	rows.Close()

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

	// Only delete if owned by this user
	result, err := s.db.Exec("DELETE FROM saved_lists WHERE id = ? AND user_id = ?", id, session.UserID)
	if err != nil {
		log.Printf("Error deleting saved list: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete saved list"})
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

	// Verify ownership
	var ownerID int
	err := s.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", id).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}
	if ownerID != session.UserID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}

	// If mode=overwrite, clear active list first
	if r.URL.Query().Get("mode") == "overwrite" {
		if _, err := s.db.Exec("DELETE FROM list_items WHERE user_id = ?", session.UserID); err != nil {
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

	type savedItem struct {
		Name, Brand, Department, Size string
		Quantity                       int
	}
	var savedItems []savedItem
	for rows.Next() {
		var si savedItem
		if err := rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size); err != nil {
			continue
		}
		savedItems = append(savedItems, si)
	}
	rows.Close()

	added := 0
	for _, si := range savedItems {
		_, _, err := upsertListItem(s.db, session.UserID, si.Name, si.Brand, si.Department, si.Size, si.Quantity)
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
// HELPERS
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// =============================================================================
// MAIN
// =============================================================================

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
	}

	http.HandleFunc("/auth/send-code", server.handleSendCode)
	http.HandleFunc("/auth/verify-code", server.handleVerifyCode)
	http.HandleFunc("/auth/logout", server.handleLogout)
	http.HandleFunc("/auth/me", server.handleMe)
	http.HandleFunc("/api/list", server.handleList)
	http.HandleFunc("/api/list/", server.handleList)
	http.HandleFunc("/api/saved-lists", server.handleSavedLists)
	http.HandleFunc("/api/saved-lists/", server.handleSavedLists)

	// Serve the React frontend. The custom handler supports SPA routing:
	// known files are served directly, everything else gets index.html.
	fs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat("static" + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "static/index.html")
	})

	// Wrap with request logging
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[req] %s %s (Content-Length: %d)", r.Method, r.URL.Path, r.ContentLength)
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	log.Printf("thriftly is running on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
