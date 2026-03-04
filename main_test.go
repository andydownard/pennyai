// =============================================================================
// thriftly — main_test.go
//
// Complete test suite. Uses:
//   - MockTwilioClient: fake Twilio (no network, no credentials)
//   - In-memory SQLite: real database logic, but no file on disk
//   - httptest: Go's built-in HTTP testing tools
//
// HOW TO RUN:
//   go test -v              ← verbose output
//   go test -v -run TestNormalizePhone   ← run one group
//   go test -cover          ← show coverage percentage
//   go test -race           ← detect concurrency bugs
// =============================================================================

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// MOCK TWILIO CLIENT
// =============================================================================

type MockTwilioClient struct {
	LastSendPhone  string
	LastCheckPhone string
	LastCheckCode  string

	SendErr       error
	CheckApproved bool
	CheckErr      error
}

func (m *MockTwilioClient) SendVerification(phone string) error {
	m.LastSendPhone = phone
	return m.SendErr
}

func (m *MockTwilioClient) CheckVerification(phone, code string) (bool, error) {
	m.LastCheckPhone = phone
	m.LastCheckCode = code
	return m.CheckApproved, m.CheckErr
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// newTestServer creates a Server with a mock Twilio client and an in-memory
// SQLite database. Each test gets a fresh database — no leaking state between
// tests.
func newTestServer(t *testing.T, mock *MockTwilioClient) *Server {
	t.Helper() // marks this as a helper so errors show the caller's line number

	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	// t.Cleanup runs when the test finishes — like "defer" but for tests.
	// This ensures the database is closed even if the test fails early.
	t.Cleanup(func() { db.Close() })

	return &Server{
		config:   Config{Port: "8080", AnthropicAPIKey: "test-key"},
		db:       db,
		sessions: NewSessionStore(),
		twilio:   mock,
		parseItems: func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
			// Default mock parser: returns nothing. Tests can override.
			return nil, nil
		},
	}
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

// =============================================================================
// PHONE NUMBER NORMALIZATION TESTS
// =============================================================================

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantE164 string
		wantOK   bool
	}{
		// --- Valid: raw digits ---
		{"10 raw digits", "5551234567", "+15551234567", true},

		// --- Valid: with formatting ---
		{"parens and dash", "(555) 123-4567", "+15551234567", true},
		{"dashes only", "555-123-4567", "+15551234567", true},
		{"dots", "555.123.4567", "+15551234567", true},
		{"spaces", "555 123 4567", "+15551234567", true},

		// --- Valid: leading country code stripped ---
		{"11 digits with leading 1", "15551234567", "+15551234567", true},
		{"formatted with leading 1", "1 (555) 123-4567", "+15551234567", true},
		{"+1 prefix stripped", "+15551234567", "+15551234567", true},
		{"1-555-123-4567", "1-555-123-4567", "+15551234567", true},

		// --- Invalid ---
		{"too short (9 digits)", "555123456", "", false},
		{"too long (12 digits)", "125551234567", "", false},
		{"empty string", "", "", false},
		{"all letters", "abcdefghij", "", false},
		{"5 digits", "55512", "", false},
		{"leading 2 with 11 digits", "25551234567", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizePhone(tt.input)
			if ok != tt.wantOK {
				t.Errorf("normalizePhone(%q): ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if ok && got != tt.wantE164 {
				t.Errorf("normalizePhone(%q) = %q, want %q", tt.input, got, tt.wantE164)
			}
		})
	}
}

// =============================================================================
// DATABASE TESTS
// =============================================================================

func TestUpsertUser_CreatesNewUser(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	user, err := upsertUser(db, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected non-zero user ID")
	}
	if user.PhoneNumber != "5551234567" {
		t.Errorf("Phone: got %s, want 5551234567", user.PhoneNumber)
	}
	if user.CreatedAt == "" {
		t.Error("Expected non-empty CreatedAt")
	}
}

func TestUpsertUser_SamePhoneTwice(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	first, _ := upsertUser(db, "5551234567")
	second, _ := upsertUser(db, "5551234567")

	// Same phone should return the same user — not create a duplicate
	if first.ID != second.ID {
		t.Errorf("Same phone created two users: ID %d and %d", first.ID, second.ID)
	}
}

func TestUpsertUser_DifferentPhones(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	u1, _ := upsertUser(db, "5551111111")
	u2, _ := upsertUser(db, "5552222222")

	if u1.ID == u2.ID {
		t.Error("Different phones got the same user ID")
	}
}

func TestInitDB_CreatesTable(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Verify the users table exists by querying it
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Users table doesn't exist: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 users in fresh DB, got %d", count)
	}
}

// =============================================================================
// SESSION STORE TESTS
// =============================================================================

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore()
	token, err := store.CreateSession(1, "5551234567")

	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if token == "" {
		t.Fatal("Empty token")
	}
	if len(token) != 64 {
		t.Errorf("Token length: got %d, want 64", len(token))
	}

	session, ok := store.GetSession(token)
	if !ok {
		t.Fatal("GetSession returned false for valid token")
	}
	if session.UserID != 1 {
		t.Errorf("UserID: got %d, want 1", session.UserID)
	}
	if session.PhoneNumber != "5551234567" {
		t.Errorf("Phone: got %s, want 5551234567", session.PhoneNumber)
	}
}

func TestSessionStore_GetNonexistent(t *testing.T) {
	store := NewSessionStore()
	_, ok := store.GetSession("nonexistent-token")
	if ok {
		t.Error("Expected ok=false for nonexistent token")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	token, _ := store.CreateSession(1, "5551234567")
	store.DeleteSession(token)

	_, ok := store.GetSession(token)
	if ok {
		t.Error("Session still exists after deletion")
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	store := NewSessionStore()
	token1, _ := store.CreateSession(1, "5551111111")
	token2, _ := store.CreateSession(2, "5552222222")

	if token1 == token2 {
		t.Error("Two sessions got the same token")
	}

	s1, _ := store.GetSession(token1)
	s2, _ := store.GetSession(token2)

	if s1.PhoneNumber != "5551111111" || s2.PhoneNumber != "5552222222" {
		t.Error("Sessions mapped to wrong data")
	}
}

func TestSessionStore_DeleteDoesNotAffectOthers(t *testing.T) {
	store := NewSessionStore()
	token1, _ := store.CreateSession(1, "5551111111")
	token2, _ := store.CreateSession(2, "5552222222")

	store.DeleteSession(token1)

	_, ok1 := store.GetSession(token1)
	_, ok2 := store.GetSession(token2)
	if ok1 {
		t.Error("Deleted session still exists")
	}
	if !ok2 {
		t.Error("Unrelated session was deleted")
	}
}

// =============================================================================
// SEND CODE HANDLER TESTS
// =============================================================================

func TestSendCode_Success(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]string{"phone_number": "5551234567"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if mock.LastSendPhone != "+15551234567" {
		t.Errorf("Twilio got %s, want +15551234567", mock.LastSendPhone)
	}
}

func TestSendCode_FormattedInput(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]string{"phone_number": "(555) 123-4567"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if mock.LastSendPhone != "+15551234567" {
		t.Errorf("Twilio got %s, want +15551234567", mock.LastSendPhone)
	}
}

func TestSendCode_WithLeadingOne(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]string{"phone_number": "15551234567"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if mock.LastSendPhone != "+15551234567" {
		t.Errorf("Twilio got %s, want +15551234567", mock.LastSendPhone)
	}
}

func TestSendCode_InvalidPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	tests := []struct {
		name  string
		phone string
	}{
		{"too short", "55512345"},
		{"too long", "125551234567"},
		{"empty", ""},
		{"letters", "abcdefghij"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
				map[string]string{"phone_number": tt.phone}, nil)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("Phone %q: got %d, want 400", tt.phone, rr.Code)
			}
		})
	}
}

func TestSendCode_TwilioFails(t *testing.T) {
	mock := &MockTwilioClient{SendErr: fmt.Errorf("twilio is down")}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]string{"phone_number": "5551234567"}, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

func TestSendCode_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleSendCode, "GET", "/auth/send-code", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestSendCode_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/send-code", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleSendCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// VERIFY CODE HANDLER TESTS
// =============================================================================

func TestVerifyCode_Success(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "123456"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}
	if body["phone_number"] != "5551234567" {
		t.Errorf("phone_number: got %v, want 5551234567", body["phone_number"])
	}
	// user_id should be a number > 0
	if uid, ok := body["user_id"].(float64); !ok || uid < 1 {
		t.Errorf("user_id: got %v, want positive integer", body["user_id"])
	}

	// Session cookie checks
	cookie := getSessionCookie(rr)
	if cookie == nil {
		t.Fatal("No session cookie set")
	}
	if cookie.Value == "" {
		t.Error("Session cookie is empty")
	}
	if !cookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}

	// Verify Twilio received the right data
	if mock.LastCheckPhone != "+15551234567" {
		t.Errorf("Twilio phone: got %s, want +15551234567", mock.LastCheckPhone)
	}
	if mock.LastCheckCode != "123456" {
		t.Errorf("Twilio code: got %s, want 123456", mock.LastCheckCode)
	}
}

func TestVerifyCode_CreatesUser(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "123456"}, nil)

	// Verify a user row exists in the database
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone_number = '5551234567'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 user row, got %d", count)
	}
}

func TestVerifyCode_SamePhoneTwiceDoesNotDuplicate(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Verify the same phone number twice
	doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "111111"}, nil)
	doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "222222"}, nil)

	// Should still be exactly one user row
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone_number = '5551234567'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 user row after two logins, got %d", count)
	}
}

func TestVerifyCode_WrongCode(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: false}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "000000"}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}

	cookie := getSessionCookie(rr)
	if cookie != nil && cookie.Value != "" {
		t.Error("Should not set session cookie for wrong code")
	}

	// Should NOT create a user for a failed verification
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 users after wrong code, got %d", count)
	}
}

func TestVerifyCode_TwilioFails(t *testing.T) {
	mock := &MockTwilioClient{CheckErr: fmt.Errorf("timeout")}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "123456"}, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

func TestVerifyCode_InvalidPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "123", "code": "123456"}, nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestVerifyCode_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleVerifyCode, "GET", "/auth/verify-code", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestVerifyCode_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/verify-code", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleVerifyCode(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// ME HANDLER TESTS
// =============================================================================

func TestMe_LoggedIn(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	token, _ := server.sessions.CreateSession(42, "5551234567")

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{{Name: "session", Value: token}})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["logged_in"] != true {
		t.Error("Expected logged_in=true")
	}
	if body["phone_number"] != "5551234567" {
		t.Errorf("phone: got %v, want 5551234567", body["phone_number"])
	}
	if uid, ok := body["user_id"].(float64); !ok || uid != 42 {
		t.Errorf("user_id: got %v, want 42", body["user_id"])
	}
}

func TestMe_NotLoggedIn(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMe_ExpiredSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{{Name: "session", Value: "fake-expired-token"}})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMe_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleMe, "POST", "/auth/me", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// LOGOUT HANDLER TESTS
// =============================================================================

func TestLogout_ClearsSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	token, _ := server.sessions.CreateSession(1, "5551234567")

	rr := doRequest(server.handleLogout, "POST", "/auth/logout", nil,
		[]*http.Cookie{{Name: "session", Value: token}})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	_, ok := server.sessions.GetSession(token)
	if ok {
		t.Error("Session still exists after logout")
	}

	cookie := getSessionCookie(rr)
	if cookie != nil && cookie.MaxAge >= 0 {
		t.Error("Cookie should have negative MaxAge")
	}
}

func TestLogout_NoCookie(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleLogout, "POST", "/auth/logout", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestLogout_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleLogout, "GET", "/auth/logout", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// WRITE JSON HELPER TEST
// =============================================================================

func TestWriteJSON_SetsContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"hello": "world"})

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", ct)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// =============================================================================
// FULL FLOW INTEGRATION TEST
//
// Simulates: send → verify → me (logged in) → logout → me (denied)
// Also verifies the user was created in the database.
// =============================================================================

func TestFullLoginFlow(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Step 1: Send code
	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]string{"phone_number": "5559876543"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Step 1 (send): got %d, want 200", rr.Code)
	}

	// Step 2: Verify → get cookie
	rr = doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5559876543", "code": "123456"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Step 2 (verify): got %d, want 200", rr.Code)
	}
	cookie := getSessionCookie(rr)
	if cookie == nil {
		t.Fatal("Step 2: no session cookie")
	}

	// Step 3: Check /auth/me → logged in
	rr = doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Step 3 (me): got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["phone_number"] != "5559876543" {
		t.Errorf("Step 3: phone = %v, want 5559876543", body["phone_number"])
	}

	// Step 4: Verify user exists in database
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone_number = '5559876543'").Scan(&count)
	if count != 1 {
		t.Errorf("Step 4: expected 1 user row, got %d", count)
	}

	// Step 5: Logout
	rr = doRequest(server.handleLogout, "POST", "/auth/logout", nil,
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Step 5 (logout): got %d, want 200", rr.Code)
	}

	// Step 6: /auth/me → denied
	rr = doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Step 6 (me after logout): got %d, want 401", rr.Code)
	}
}

// =============================================================================
// LIST HANDLER TESTS
// =============================================================================

// loginUser creates a user in the DB and returns a session cookie for that user.
func loginUser(t *testing.T, server *Server, phone string) (*http.Cookie, int) {
	t.Helper()
	user, err := upsertUser(server.db, phone)
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}
	token, err := server.sessions.CreateSession(user.ID, phone)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token}, user.ID
}

func TestInitDB_CreatesListItemsTable(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count); err != nil {
		t.Fatalf("list_items table doesn't exist: %v", err)
	}
}

func TestListGet_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var items []ListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestListGet_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "GET", "/api/list", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestListAdd_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &item); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if item.Name != "Milk" {
		t.Errorf("Name: got %q, want Milk", item.Name)
	}
	if item.Department != "Dairy & Eggs" {
		t.Errorf("Department: got %q, want Dairy & Eggs", item.Department)
	}
	if item.ID == 0 {
		t.Error("Expected non-zero ID")
	}
	if item.Checked {
		t.Error("New item should not be checked")
	}
	if item.Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1", item.Quantity)
	}
}

func TestListAdd_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "  ", "department": "Produce"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListToggle_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert an item directly
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs",
	)
	itemID, _ := result.LastInsertId()

	// Toggle it
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if !item.Checked {
		t.Error("Expected item to be checked after toggle")
	}

	// Toggle again — should uncheck
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Checked {
		t.Error("Expected item to be unchecked after second toggle")
	}
}

func TestListEdit_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	newName := "Sourdough"
	newBrand := "Tartine"
	newSize := "1 lb"
	newQty := 2
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": newName, "brand": newBrand, "size": newSize, "quantity": newQty},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != newName {
		t.Errorf("Name: got %q, want %q", item.Name, newName)
	}
	if item.Brand != newBrand {
		t.Errorf("Brand: got %q, want %q", item.Brand, newBrand)
	}
	if item.Size != newSize {
		t.Errorf("Size: got %q, want %q", item.Size, newSize)
	}
	if item.Quantity != newQty {
		t.Errorf("Quantity: got %d, want %d", item.Quantity, newQty)
	}
}

func TestListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Butter", "Dairy & Eggs",
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify gone from DB
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Item still in DB after delete")
	}
}

func TestListCannotAccessOtherUsersItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User 1 adds an item
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Secret Item", "Pantry",
	)
	itemID, _ := result.LastInsertId()

	// User 2 tries to delete user 1's item
	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie2})

	if rr.Code != http.StatusOK {
		t.Errorf("Delete returned %d", rr.Code)
	}

	// Item should still exist (user 2's delete was a no-op)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 1 {
		t.Error("User 2 was able to delete user 1's item")
	}

	// User 1 can see their own item
	rr = doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("User 1 should see 1 item, got %d", len(items))
	}
}

// =============================================================================
// DEDUP TESTS
// =============================================================================

func TestDedup_SameItemTwice(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1)
	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1)

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity=2, got %d", qty)
	}
}

func TestDedup_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "BREAD", "", "Bakery", "", 1)
	upsertListItem(server.db, userID, "bread", "", "Bakery", "", 1)

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row (case-insensitive match), got %d", count)
	}
}

func TestDedup_DifferentUsersDontCrossDedup(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	upsertListItem(server.db, userID1, "Milk", "", "Dairy & Eggs", "", 1)
	upsertListItem(server.db, userID2, "Milk", "", "Dairy & Eggs", "", 1)

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 rows (one per user), got %d", count)
	}
}

func TestDedup_CheckedItemGetsNewEntry(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert and check off the item
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)",
		userID, "Milk", "Dairy & Eggs",
	)

	// Upsert same name — should create new because existing is checked
	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1)

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 rows (checked + new unchecked), got %d", count)
	}
}

func TestDedup_QuantityAddsCorrectly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 2)
	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 3)

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 5 {
		t.Errorf("Expected quantity=5, got %d", qty)
	}
}

func TestDedup_BrandFillsBlankButDoesNotOverwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// First insert: no brand
	upsertListItem(server.db, userID, "Peanut Butter", "", "Pantry", "", 1)

	// Second insert: has brand — should fill blank
	upsertListItem(server.db, userID, "Peanut Butter", "Jif", "Pantry", "", 1)

	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ?", userID).Scan(&brand)
	if brand != "Jif" {
		t.Errorf("Expected brand=Jif, got %q", brand)
	}

	// Third insert: different brand — should NOT overwrite
	upsertListItem(server.db, userID, "Peanut Butter", "Skippy", "Pantry", "", 1)

	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ?", userID).Scan(&brand)
	if brand != "Jif" {
		t.Errorf("Expected brand to stay Jif, got %q", brand)
	}
}

// =============================================================================
// PARSE ENDPOINT TESTS
// =============================================================================

func TestParseAndAdd_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: "1 gal"},
			{Name: "Bread", Brand: "", Department: "Bakery", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk and bread"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Added != 2 {
		t.Errorf("Added: got %d, want 2", resp.Added)
	}
	if len(resp.Items) != 2 {
		t.Errorf("Items: got %d, want 2", len(resp.Items))
	}
	if len(resp.Skipped) != 0 {
		t.Errorf("Skipped: got %d, want 0", len(resp.Skipped))
	}
}

func TestParseAndAdd_SkipsDuplicates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Pre-insert Milk
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1,
	)
	milkID, _ := result.LastInsertId()

	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 2, Size: ""},
			{Name: "Eggs", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "2 milk eggs"},
		[]*http.Cookie{cookie})

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Added != 1 {
		t.Errorf("Added: got %d, want 1", resp.Added)
	}
	if len(resp.Skipped) != 1 {
		t.Errorf("Skipped: got %d, want 1", len(resp.Skipped))
	}
	if len(resp.Skipped) > 0 && resp.Skipped[0].ExistingID != int(milkID) {
		t.Errorf("Skipped existing_id: got %d, want %d", resp.Skipped[0].ExistingID, milkID)
	}
	if len(resp.Skipped) > 0 && resp.Skipped[0].Parsed.Name != "Milk" {
		t.Errorf("Skipped parsed name: got %q, want Milk", resp.Skipped[0].Parsed.Name)
	}

	// Milk quantity should be unchanged (still 1, not merged)
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 1 {
		t.Errorf("Milk quantity should be unchanged: got %d, want 1", qty)
	}
}

func TestParseAndAdd_EmptyTextNoImage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": ""},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestParseAndAdd_NoAPIKey(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.AnthropicAPIKey = ""
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Status: got %d, want 503", rr.Code)
	}
}

func TestParseAndAdd_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestParseAndAdd_ImageOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotImage string
	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		gotImage = imageBase64
		return []ParsedItem{
			{Name: "Apple", Brand: "", Department: "Produce", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "", "image": "data:image/jpeg;base64,abc123"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	if gotImage != "data:image/jpeg;base64,abc123" {
		t.Errorf("Image not passed to parser: got %q", gotImage)
	}
}

func TestParseAndAdd_TextAndImage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotText, gotImage string
	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		gotText = text
		gotImage = imageBase64
		return []ParsedItem{
			{Name: "Banana", Brand: "", Department: "Produce", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "also bananas", "image": "data:image/png;base64,xyz"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	if gotText != "also bananas" {
		t.Errorf("Text: got %q, want 'also bananas'", gotText)
	}
	if gotImage != "data:image/png;base64,xyz" {
		t.Errorf("Image: got %q", gotImage)
	}
}

// =============================================================================
// DIRECT ADD DEDUP TEST
// =============================================================================

func TestListAdd_DedupSameName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})
	doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after adding same name twice, got %d", count)
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity=2, got %d", qty)
	}
}

// =============================================================================
// ADDITIONAL HANDLER TESTS — edge cases for coverage
// =============================================================================

func TestListAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Mystery Item"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Department != "Other" {
		t.Errorf("Department: got %q, want Other", item.Department)
	}
}

func TestListAdd_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListAdd_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestListToggle_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999/toggle",
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListToggle_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "PATCH", "/api/list/1/toggle", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestListEdit_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	emptyName := "  "
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": emptyName},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListEdit_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999",
		map[string]interface{}{"name": "Whatever"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListEdit_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "PATCH", "/api/list/1",
		map[string]interface{}{"name": "x"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestListEdit_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery",
	)

	req := httptest.NewRequest("PATCH", "/api/list/1", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListEdit_QuantityClamp(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Bread", "Bakery", 1,
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"quantity": 0},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", item.Quantity)
	}
}

func TestListDelete_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "DELETE", "/api/list/1", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestListDelete_NonexistentItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "DELETE", "/api/list/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestList_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/abc/toggle", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestList_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PUT", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestList_UnknownSubpath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/1/unknown", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestParseAndAdd_ClaudeError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		return nil, fmt.Errorf("Claude API error")
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

func TestParseAndAdd_EmptyItemName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "", Brand: "", Department: "Other", Quantity: 1, Size: ""},
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Added != 1 {
		t.Errorf("Added: got %d, want 1 (empty name should be skipped)", resp.Added)
	}
}

func TestParseAndAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text, imageBase64 string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Widget", Brand: "", Department: "", Quantity: 0, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "widget"},
		[]*http.Cookie{cookie})

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Department != "Other" {
		t.Errorf("Department: got %q, want Other", resp.Items[0].Department)
	}
	if resp.Items[0].Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", resp.Items[0].Quantity)
	}
}

func TestParseAndAdd_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list/parse", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListGet_ReturnsMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Bread", "Bakery")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})

	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestListGet_OnlyReturnsOwnItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID1, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID2, "Bread", "Bakery")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})

	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("Expected 1 item for user 1, got %d", len(items))
	}
	if len(items) > 0 && items[0].Name != "Milk" {
		t.Errorf("Expected Milk, got %q", items[0].Name)
	}
}

// =============================================================================
// CLEAR LIST TESTS
// =============================================================================

func TestClearList_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Bread", "Bakery")

	rr := doRequest(server.handleList, "DELETE", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestClearList_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "DELETE", "/api/list/clear", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestClearList_DoesNotAffectOtherUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID1, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID2, "Bread", "Bakery")

	doRequest(server.handleList, "DELETE", "/api/list/clear", nil, []*http.Cookie{cookie1})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID2).Scan(&count)
	if count != 1 {
		t.Errorf("User 2 items affected by user 1 clear: got %d, want 1", count)
	}
}

// =============================================================================
// SAVED LISTS TESTS
// =============================================================================

func TestSavedLists_GetEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("Expected 0 saved lists, got %d", len(lists))
	}
}

func TestSavedLists_CreateAndGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add some items to the list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Milk", "Organic Valley", "Dairy & Eggs", 2, "1 gal")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")

	// Save the list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly Groceries"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Create status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["name"] != "Weekly Groceries" {
		t.Errorf("Name: got %v, want Weekly Groceries", body["name"])
	}
	if body["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v, want 2", body["item_count"])
	}

	// Get saved lists
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Fatalf("Expected 1 saved list, got %d", len(lists))
	}
	if lists[0]["name"] != "Weekly Groceries" {
		t.Errorf("Name: got %v", lists[0]["name"])
	}
	if lists[0]["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v", lists[0]["item_count"])
	}
}

func TestSavedLists_CreateEmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "  "},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_CreateNoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// No items at all on the list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Empty List"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_CreateSavesAllItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 0)",
		userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)",
		userID, "Bread", "Bakery")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Full List"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v, want 2 (all items)", body["item_count"])
	}
}

func TestSavedLists_Delete(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	// Create
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "To Delete"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Delete
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Delete status: got %d, want 200", rr.Code)
	}

	// Verify items are also gone (cascade)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 saved list items after cascade delete, got %d", count)
	}

	// Verify list is gone
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("Expected 0 saved lists after delete, got %d", len(lists))
	}
}

func TestSavedLists_DeleteNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists/99999",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_Update(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Change the active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs")

	// Update saved list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Update status: got %d, want 200", rr.Code)
	}

	updateBody := parseJSON(rr)
	if updateBody["item_count"].(float64) != 2 {
		t.Errorf("Item count after update: got %v, want 2", updateBody["item_count"])
	}

	// Verify old items replaced
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 saved list items after update, got %d", count)
	}
}

func TestSavedLists_UpdateNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/99999",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_Load(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Milk", "Organic Valley", "Dairy & Eggs", 1, "1 gal")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Clear active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)

	// Load saved list
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	loadBody := parseJSON(rr)
	if loadBody["added"].(float64) != 2 {
		t.Errorf("Added: got %v, want 2", loadBody["added"])
	}

	// Verify items are in active list
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 active list items after load, got %d", count)
	}
}

func TestSavedLists_LoadNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/99999/load",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_CrossUserIsolation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User 1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "User1 List"},
		[]*http.Cookie{cookie1})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// User 2 tries to load user 1's list
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Load other user's list: got %d, want 404", rr.Code)
	}

	// User 2 tries to delete user 1's list
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Delete other user's list: got %d, want 404", rr.Code)
	}

	// User 2 tries to update user 1's list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Update other user's list: got %d, want 404", rr.Code)
	}

	// User 2 sees no saved lists
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie2})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("User 2 should see 0 saved lists, got %d", len(lists))
	}
}

func TestSavedLists_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET status: got %d, want 401", rr.Code)
	}

	rr = doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "test"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("POST status: got %d, want 401", rr.Code)
	}
}

func TestSavedLists_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "PATCH", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestSavedLists_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_LoadOverwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Add a different item to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs")

	// Load with overwrite — should clear Eggs and Milk, then add back Milk from saved
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	// Should have only the saved list item (Milk), not Eggs
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite load, got %d", count)
	}

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", userID).Scan(&name)
	if name != "Milk" {
		t.Errorf("Expected Milk, got %q", name)
	}
}

func TestSavedLists_LoadMergesWithExisting(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Load into list that already has Milk — should merge (upsert)
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	// Should still be 1 row but with quantity 2
	var count, qty int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 list item (merged), got %d", count)
	}
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity 2 after merge, got %d", qty)
	}
}
