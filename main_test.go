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
		config:   Config{Port: "8080"},
		db:       db,
		sessions: NewSessionStore(),
		twilio:   mock,
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
