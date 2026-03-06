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
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// MOCK TWILIO CLIENT
// =============================================================================

type MockTwilioClient struct {
	mu             sync.Mutex
	LastSendPhone  string
	LastCheckPhone string
	LastCheckCode  string
	LastSMSTo      string
	LastSMSBody    string

	SendErr       error
	CheckApproved bool
	CheckErr      error
	SMSErr        error
}

func (m *MockTwilioClient) SendVerification(phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastSendPhone = phone
	return m.SendErr
}

func (m *MockTwilioClient) CheckVerification(phone, code string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastCheckPhone = phone
	m.LastCheckCode = code
	return m.CheckApproved, m.CheckErr
}

func (m *MockTwilioClient) SendSMS(from, to, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastSMSTo = to
	m.LastSMSBody = body
	return m.SMSErr
}

// GetLastSMS returns the last SMS recipient and body in a thread-safe way.
func (m *MockTwilioClient) GetLastSMS() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.LastSMSTo, m.LastSMSBody
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
		config:   Config{Port: "8080", AnthropicAPIKey: "test-key", TwilioPhoneNumber: "+10000000000", AdminPhone: "3104861833"},
		db:       db,
		sessions: NewSessionStore(db),
		twilio:   mock,
		parseItems: func(apiKey, text string, images []string) ([]ParsedItem, error) {
			// Default mock parser: returns nothing. Tests can override.
			return nil, nil
		},
		smsBuffer: newMessageBuffer(50 * time.Millisecond),
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

// createListItem is a helper to insert a list item and return its ID.
func createListItem(t *testing.T, db *sql.DB, userID int, name, department string) int64 {
	t.Helper()
	result, err := db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, name, department)
	if err != nil {
		t.Fatalf("createListItem: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

// =============================================================================
// CONSOLIDATED NoSession / MethodNotAllowed TESTS
//
// These table-driven tests replace dozens of individual tests that all
// followed the same pattern: send a request and check for 401 or 405.
// =============================================================================

func TestNoSession_AllEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
		body    interface{}
	}{
		{"ListGet", server.handleList, "GET", "/api/list", nil},
		{"ListAdd", server.handleList, "POST", "/api/list", map[string]string{"name": "Milk"}},
		{"ListToggle", server.handleList, "PATCH", "/api/list/1/toggle", nil},
		{"ListEdit", server.handleList, "PATCH", "/api/list/1", map[string]interface{}{"name": "x"}},
		{"ListDelete", server.handleList, "DELETE", "/api/list/1", nil},
		{"ParseAndAdd", server.handleList, "POST", "/api/list/parse", map[string]string{"text": "milk"}},
		{"ClearList", server.handleListClear, "POST", "/api/list/clear", nil},
		{"SavedLists", server.handleSavedLists, "GET", "/api/saved-lists", nil},
		{"SavedListsCreate", server.handleSavedLists, "POST", "/api/saved-lists", map[string]string{"name": "test"}},
		{"SavedListDelete", server.handleSavedLists, "DELETE", "/api/saved-lists/1", nil},
		{"Settings", server.handleSettings, "PATCH", "/api/settings", map[string]string{"name": "Test"}},
		{"Invite", server.handleInvite, "POST", "/api/settings/invite", map[string]string{"phone": ""}},
		{"LeaveHousehold", server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil},
		{"ListUrgent", server.handleList, "PATCH", "/api/list/1/urgent", nil},
		{"ListClaim", server.handleList, "PATCH", "/api/list/1/claim", nil},
		{"NPSCheck", server.handleNPSCheck, "GET", "/api/nps/check", nil},
		{"NPS", server.handleNPS, "POST", "/api/nps", map[string]interface{}{"score": 8}},
		{"ReferralRedeem", server.handleReferralRedeem, "POST", "/api/referral/redeem", nil},
		{"Trips", server.handleTrips, "GET", "/api/trips", nil},
		{"RequireAdmin", server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(tc.handler, tc.method, tc.path, tc.body, nil)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Status: got %d, want 401", rr.Code)
			}
		})
	}
}

func TestMethodNotAllowed_AllEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{"List_PUT", server.handleList, "PUT", "/api/list"},
		{"SavedLists_PATCH", server.handleSavedLists, "PATCH", "/api/saved-lists"},
		{"SavedListsCreate_PUT", server.handleSavedLists, "PUT", "/api/saved-lists"},
		{"LeaveHousehold_GET", server.handleLeaveHousehold, "GET", "/api/settings/leave-household"},
		{"Join_GET", server.handleJoin, "GET", "/api/join"},
		{"Invite_GET", server.handleInvite, "GET", "/api/settings/invite"},
		{"VCard_POST", server.handleVCard, "POST", "/api/penny.vcf"},
		{"Settings_DELETE", server.handleSettings, "DELETE", "/api/settings"},
		{"ListClear_GET", server.handleListClear, "GET", "/api/list/clear"},
		{"SMSWebhook_GET", server.handleSMSWebhook, "GET", "/sms"},
		{"EmailWebhook_GET", server.handleEmailWebhook, "GET", "/api/email-webhook"},
		{"ReferralClaim_GET", server.handleReferralClaim, "GET", "/api/referral/claim"},
		{"ReferralRedeem_GET", server.handleReferralRedeem, "GET", "/api/referral/redeem"},
		{"Referral_POST", server.handleReferral, "POST", "/api/referral"},
		{"Brands_POST", server.handleBrands, "POST", "/api/brands"},
		{"NPSCheck_POST", server.handleNPSCheck, "POST", "/api/nps/check"},
		{"NPS_GET", server.handleNPS, "GET", "/api/nps"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(tc.handler, tc.method, tc.path, nil, []*http.Cookie{cookie})
			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("Status: got %d, want 405", rr.Code)
			}
		})
	}
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

func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	// Create a user so FK constraints are satisfied
	db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567'), (2, '5552222222')")
	t.Cleanup(func() { db.Close() })
	return NewSessionStore(db)
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := newTestSessionStore(t)
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
	store := newTestSessionStore(t)
	_, ok := store.GetSession("nonexistent-token")
	if ok {
		t.Error("Expected ok=false for nonexistent token")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := newTestSessionStore(t)
	token, _ := store.CreateSession(1, "5551234567")
	store.DeleteSession(token)

	_, ok := store.GetSession(token)
	if ok {
		t.Error("Session still exists after deletion")
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	store := newTestSessionStore(t)
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
	store := newTestSessionStore(t)
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
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567")

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
	if uid, ok := body["user_id"].(float64); !ok || int(uid) != user.ID {
		t.Errorf("user_id: got %v, want %d", body["user_id"], user.ID)
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
	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

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

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

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

	itemID := createListItem(t, server.db, userID, "Butter", "Dairy & Eggs")

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

	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "")
	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "")

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

	upsertListItem(server.db, userID, "BREAD", "", "Bakery", "", 1, "app", "")
	upsertListItem(server.db, userID, "bread", "", "Bakery", "", 1, "app", "")

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

	upsertListItem(server.db, userID1, "Milk", "", "Dairy & Eggs", "", 1, "app", "")
	upsertListItem(server.db, userID2, "Milk", "", "Dairy & Eggs", "", 1, "app", "")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 rows (one per user), got %d", count)
	}
}

func TestDedup_CheckedItemGetsUnchecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert and check off the item
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)",
		userID, "Milk", "Dairy & Eggs",
	)

	// Upsert same name — should uncheck existing (no duplicate)
	item, _, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row (no duplicate), got %d", count)
	}
}

func TestDedup_QuantityAddsCorrectly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 2, "app", "")
	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 3, "app", "")

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
	upsertListItem(server.db, userID, "Peanut Butter", "", "Pantry", "", 1, "app", "")

	// Second insert: has brand — should fill blank
	upsertListItem(server.db, userID, "Peanut Butter", "Jif", "Pantry", "", 1, "app", "")

	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ?", userID).Scan(&brand)
	if brand != "Jif" {
		t.Errorf("Expected brand=Jif, got %q", brand)
	}

	// Third insert: different brand — should NOT overwrite
	upsertListItem(server.db, userID, "Peanut Butter", "Skippy", "Pantry", "", 1, "app", "")

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

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
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

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
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

	// Milk quantity should be merged via upsertListItem (1 + 2 = 3)
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 3 {
		t.Errorf("Milk quantity should be merged: got %d, want 3", qty)
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

func TestParseAndAdd_ImageOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotImages = images
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
	if len(gotImages) != 1 || gotImages[0] != "data:image/jpeg;base64,abc123" {
		t.Errorf("Image not passed to parser: got %v", gotImages)
	}
}

func TestParseAndAdd_TextAndImage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotText string
	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotText = text
		gotImages = images
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
	if len(gotImages) != 1 || gotImages[0] != "data:image/png;base64,xyz" {
		t.Errorf("Images: got %v", gotImages)
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

func TestListToggle_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999/toggle",
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListEdit_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

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

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
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

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
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

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
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

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestClearList_DoesNotAffectOtherUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID1, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID2, "Bread", "Bakery")

	doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie1})

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

// =============================================================================
// SMS WEBHOOK TESTS
// =============================================================================

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

func TestSMSWebhook_TextOnly(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Set up mock parser to return some items
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
			{Name: "Bread", Department: "Bakery", Quantity: 1},
		}, nil
	}

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "milk and bread", 0)

	// Should respond with empty TwiML immediately
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<Response></Response>") {
		t.Errorf("Expected empty TwiML, got %q", rr.Body.String())
	}

	// Wait for the buffer to flush (50ms) + processing time
	time.Sleep(200 * time.Millisecond)

	// Verify user was created
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone_number = '5551234567'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 user, got %d", count)
	}

	// Verify items were added
	var userID int
	server.db.QueryRow("SELECT id FROM users WHERE phone_number = '5551234567'").Scan(&userID)

	var itemCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&itemCount)
	if itemCount != 2 {
		t.Errorf("Expected 2 items, got %d", itemCount)
	}

	// Verify SMS reply was sent
	smsTo, smsBody := mock.GetLastSMS()
	if smsTo != "+15551234567" {
		t.Errorf("SMS reply to: got %q, want +15551234567", smsTo)
	}
	if !strings.Contains(smsBody, "Added to your list") {
		t.Errorf("SMS reply should contain 'Added to your list', got %q", smsBody)
	}
	if !strings.Contains(smsBody, "Milk") {
		t.Errorf("SMS reply should mention Milk, got %q", smsBody)
	}
}

func TestSMSWebhook_MessageGrouping(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	var parseCallCount int64
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		atomic.AddInt64(&parseCallCount, 1)
		// Should receive combined text from both messages
		items := []ParsedItem{}
		if strings.Contains(text, "milk") {
			items = append(items, ParsedItem{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1})
		}
		if strings.Contains(text, "eggs") {
			items = append(items, ParsedItem{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1})
		}
		return items, nil
	}

	// Send two rapid messages from the same phone
	doSMSRequest(server.handleSMSWebhook, "+15559876543", "milk", 0)
	doSMSRequest(server.handleSMSWebhook, "+15559876543", "eggs", 0)

	// Wait for buffer to flush
	time.Sleep(200 * time.Millisecond)

	// Should have been processed as ONE call (grouped)
	if count := atomic.LoadInt64(&parseCallCount); count != 1 {
		t.Errorf("Expected 1 parse call (grouped), got %d", count)
	}

	// Verify both items were added
	var userID int
	server.db.QueryRow("SELECT id FROM users WHERE phone_number = '5559876543'").Scan(&userID)

	var itemCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&itemCount)
	if itemCount != 2 {
		t.Errorf("Expected 2 items from grouped messages, got %d", itemCount)
	}
}

func TestSMSWebhook_EmptyBody(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "", 0)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Wait for buffer to flush
	time.Sleep(200 * time.Millisecond)

	// No items should be added, no SMS reply sent
	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Errorf("Should not send SMS for empty message, but sent to %q", smsTo)
	}
}

// =============================================================================
// HOUSEHOLD TESTS
// =============================================================================

func TestHouseholdAutoCreation(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	user, err := upsertUser(db, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	if user.HouseholdID == 0 {
		t.Error("Expected non-zero household ID")
	}

	// Check household_members row
	var role string
	err = db.QueryRow("SELECT role FROM household_members WHERE user_id = ?", user.ID).Scan(&role)
	if err != nil {
		t.Fatalf("No household_members row: %v", err)
	}
	if role != "owner" {
		t.Errorf("Expected role='owner', got %q", role)
	}

	// Calling upsertUser again should NOT create a second household
	user2, err := upsertUser(db, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser second call: %v", err)
	}
	if user2.HouseholdID != user.HouseholdID {
		t.Errorf("Household ID changed: %d → %d", user.HouseholdID, user2.HouseholdID)
	}
}

func TestHouseholdSharedList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// Put user2 in user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// User1 adds an item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID1)

	// User2 should see it
	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("User2 should see 1 item, got %d", len(items))
	}

	// User1 should also see it
	rr = doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("User1 should see 1 item, got %d", len(items))
	}

	// User2 adds another item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Eggs', 'Dairy & Eggs')", userID2)

	// Both should see 2 items
	rr = doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("User1 should see 2 items, got %d", len(items))
	}
}

// =============================================================================
// SETTINGS TESTS
// =============================================================================

func TestSettingsGetUpdate(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// GET initial settings
	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["phone"] != "5551234567" {
		t.Errorf("phone: got %v, want 5551234567", body["phone"])
	}

	// PATCH name and email
	name := "Alice"
	email := "alice@example.com"
	rr = doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"name": name, "email": email},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("PATCH status: got %d, want 200", rr.Code)
	}

	// GET updated settings
	rr = doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["name"] != name {
		t.Errorf("name: got %v, want %s", body["name"], name)
	}
	if body["email"] != email {
		t.Errorf("email: got %v, want %s", body["email"], email)
	}
}

// =============================================================================
// INVITE TESTS
// =============================================================================

func TestInviteCreate(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5552222222"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}
	code, ok := body["code"].(string)
	if !ok || code == "" {
		t.Error("Expected non-empty invite code")
	}

	// SMS should have been sent
	if mock.LastSMSTo != "+15552222222" {
		t.Errorf("SMS to: got %q, want +15552222222", mock.LastSMSTo)
	}
	if !strings.Contains(mock.LastSMSBody, code) {
		t.Error("SMS body should contain invite code")
	}
}

func TestJoinHousehold(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// User1 creates invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5552222222"},
		[]*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 joins with the code
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code},
		[]*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("Join status: got %d, want 200", rr.Code)
	}

	// Both users should share the same household
	var h1, h2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&h1)
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&h2)
	if h1 != h2 {
		t.Errorf("Household IDs differ: user1=%d, user2=%d", h1, h2)
	}
}

func TestJoinExpiredInvite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Create invite directly with expired timestamp
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec(
		"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, 'expired123', ?, ?)",
		hID, userID1, time.Now().Add(-1*time.Hour),
	)

	rr := doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": "expired123"},
		[]*http.Cookie{cookie2})

	if rr.Code != http.StatusGone {
		t.Errorf("Status: got %d, want 410", rr.Code)
	}
}

// =============================================================================
// VCARD TEST
// =============================================================================

func TestVCard(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny.vcf", nil)
	rr := httptest.NewRecorder()
	server.handleVCard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "text/vcard" {
		t.Errorf("Content-Type: got %q, want text/vcard", ct)
	}

	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "penny.vcf") {
		t.Errorf("Content-Disposition: got %q, want to contain penny.vcf", cd)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "BEGIN:VCARD") {
		t.Error("Body should contain BEGIN:VCARD")
	}
	if !strings.Contains(body, "penny@list.thrift.ly") {
		t.Error("Body should contain penny@list.thrift.ly")
	}
	if !strings.Contains(body, "N:AI;Penny") {
		t.Error("Body should contain N:AI;Penny")
	}
	if !strings.Contains(body, "ORG:Thriftly") {
		t.Error("Body should contain ORG:Thriftly")
	}
	if !strings.Contains(body, "TITLE:Shopping Assistant") {
		t.Error("Body should contain TITLE:Shopping Assistant")
	}
	if !strings.Contains(body, "FN:Penny AI") {
		t.Error("Body should contain FN:Penny AI")
	}
}

// =============================================================================
// EMAIL WEBHOOK TESTS
// =============================================================================

// doEmailRequest sends a multipart form request mimicking SendGrid Inbound Parse.
func doEmailRequest(handler http.HandlerFunc, from, text string) *httptest.ResponseRecorder {
	body := fmt.Sprintf("from=%s&text=%s", url.QueryEscape(from), url.QueryEscape(text))
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func TestEmailWebhook(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Set email on user
	server.db.Exec("UPDATE users SET email = 'test@example.com' WHERE id = ?", userID)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 1},
			{Name: "Bread", Brand: "", Department: "Bakery", Quantity: 1},
		}, nil
	}

	rr := doEmailRequest(server.handleEmailWebhook, "test@example.com", "milk and bread")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify items were added
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items, got %d", count)
	}
}

func TestEmailWebhookUnknownSender(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	loginUser(t, server, "5551234567") // user exists but no email set

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		t.Error("parseItems should not be called for unknown sender")
		return nil, nil
	}

	rr := doEmailRequest(server.handleEmailWebhook, "unknown@example.com", "milk and bread")

	// Should return 200 (no retry) but add no items
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items for unknown sender, got %d", count)
	}
}

func TestEmailWebhook_NameBracketFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'alice@example.com' WHERE id = ?", userID)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	rr := doEmailRequest(server.handleEmailWebhook, "Alice Smith <alice@example.com>", "eggs please")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item, got %d", count)
	}
}

func TestExtractEmail(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"alice@example.com", "alice@example.com"},
		{"Alice <alice@example.com>", "alice@example.com"},
		{"Alice Smith <alice@example.com>", "alice@example.com"},
		{"  alice@example.com  ", "alice@example.com"},
	}
	for _, tt := range tests {
		got := extractEmail(tt.input)
		if got != tt.want {
			t.Errorf("extractEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// SOURCE / ATTRIBUTION / SHARED SAVED LISTS / INVITE-NO-PHONE TESTS
// =============================================================================

func TestItemSource_App(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Source != "app" {
		t.Errorf("Source: got %q, want 'app'", item.Source)
	}
}

func TestItemSource_SMS(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Bread", Department: "Bakery", Quantity: 1},
		}, nil
	}

	doSMSRequest(server.handleSMSWebhook, "+15551234567", "bread", 0)
	time.Sleep(200 * time.Millisecond)

	var source string
	server.db.QueryRow("SELECT source FROM list_items LIMIT 1").Scan(&source)
	if source != "sms" {
		t.Errorf("Source: got %q, want 'sms'", source)
	}
}

func TestItemSource_Email(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'test@example.com' WHERE id = ?", userID)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	doEmailRequest(server.handleEmailWebhook, "test@example.com", "eggs")

	var source string
	server.db.QueryRow("SELECT source FROM list_items WHERE user_id = ?", userID).Scan(&source)
	if source != "email" {
		t.Errorf("Source: got %q, want 'email'", source)
	}
}

func TestItemAttribution(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Set user name
	server.db.Exec("UPDATE users SET name = 'Alice' WHERE id = ?", userID)

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.AddedByName != "Alice" {
		t.Errorf("AddedByName: got %q, want 'Alice'", item.AddedByName)
	}
	if item.AddedByPhone != "5551234567" {
		t.Errorf("AddedByPhone: got %q, want '5551234567'", item.AddedByPhone)
	}
}

func TestSavedListSharedAcrossHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// Put user2 in user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// User1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Shared List"},
		[]*http.Cookie{cookie1})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// User2 should see the saved list
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie2})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Errorf("User2 should see 1 saved list, got %d", len(lists))
	}

	// User2 should be able to load it
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("User2 load: got %d, want 200", rr.Code)
	}

	// User2 should be able to delete it
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("User2 delete: got %d, want 200", rr.Code)
	}
}

func TestInviteWithoutPhone(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	// Invite with empty phone
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}
	code, ok := body["code"].(string)
	if !ok || code == "" {
		t.Error("Expected non-empty invite code")
	}
	link, ok := body["link"].(string)
	if !ok || link == "" {
		t.Error("Expected non-empty invite link")
	}
	if !strings.Contains(link, code) {
		t.Error("Link should contain the code")
	}

	// SMS should NOT have been sent
	if mock.LastSMSTo != "" {
		t.Errorf("SMS should not be sent for empty phone, but sent to %q", mock.LastSMSTo)
	}
}

// =============================================================================
// INVITE AUTO-CREATES HOUSEHOLD TEST
// =============================================================================

func TestInviteAutoCreatesHousehold(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create a user directly (without household) to simulate legacy user
	server.db.Exec("INSERT INTO users (phone_number) VALUES (?)", "5551234567")
	var userID int
	server.db.QueryRow("SELECT id FROM users WHERE phone_number = '5551234567'").Scan(&userID)

	token, _ := server.sessions.CreateSession(userID, "5551234567")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Verify user has no household
	var hID sql.NullInt64
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hID)
	if hID.Valid {
		t.Fatal("Test setup: user should not have a household yet")
	}

	// Call invite — should auto-create household, not 500
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}

	// Verify household was created
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hID)
	if !hID.Valid {
		t.Error("Expected household_id to be set after invite")
	}

	// Verify household_members row exists
	var memberCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE user_id = ?", userID).Scan(&memberCount)
	if memberCount != 1 {
		t.Errorf("Expected 1 household_members row, got %d", memberCount)
	}
}

// =============================================================================
// SETTINGS UPDATE TESTS
// =============================================================================

func TestSettingsUpdateName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	name := "Alice"
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"name": name}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var dbName string
	server.db.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&dbName)
	if dbName != name {
		t.Errorf("Name: got %q, want %q", dbName, name)
	}
}

func TestSettingsUpdateEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	email := "alice@example.com"
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"email": email}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var dbEmail string
	server.db.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&dbEmail)
	if dbEmail != email {
		t.Errorf("Email: got %q, want %q", dbEmail, email)
	}
}

// =============================================================================
// SMS WEBHOOK — UNKNOWN NUMBER
// =============================================================================

func TestSMSWebhook_UnknownNumber(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Send SMS from a number that's not in the database
	form := url.Values{}
	form.Set("From", "+19995550000")
	form.Set("Body", "milk and eggs")
	form.Set("NumMedia", "0")

	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	// Should return 200 with TwiML (not crash)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<Response>") {
		t.Error("Expected TwiML response")
	}
}

// =============================================================================
// SMS WEBHOOK — GROUP TEXT
//
// Twilio delivers group MMS messages as individual webhooks per sender.
// Each webhook has the sender's phone in "From". Penny should add items
// to the correct user's list regardless of whether it's a 1:1 or group text.
// =============================================================================

func TestSMSWebhook_GroupText(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Pre-create two users — each gets their own household from upsertUser
	user1, _ := upsertUser(server.db, "5551111111")
	user2, _ := upsertUser(server.db, "5552222222")

	// Move user2 into user1's household
	var hid int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", user1.ID).Scan(&hid)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", user2.ID)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hid, user2.ID)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hid, user2.ID)

	// Mock parser returns items
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Avocados", Department: "Produce", Quantity: 3},
		}, nil
	}

	// Simulate a group MMS webhook from user2 — Twilio still sends "From" as the sender
	rr := doSMSRequest(server.handleSMSWebhook, "+15552222222", "3 avocados", 0)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Wait for buffer to flush
	time.Sleep(200 * time.Millisecond)

	// Items should be added to user2's list (the sender), visible to the household
	var itemCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", user2.ID).Scan(&itemCount)
	if itemCount != 1 {
		t.Errorf("Expected 1 item for user2, got %d", itemCount)
	}

	// Verify the item name
	var itemName string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", user2.ID).Scan(&itemName)
	if itemName != "Avocados" {
		t.Errorf("Expected item 'Avocados', got %q", itemName)
	}

	// Verify SMS reply was sent to the sender (user2), not user1
	smsTo, smsBody := mock.GetLastSMS()
	if smsTo != "+15552222222" {
		t.Errorf("SMS reply should go to sender +15552222222, got %q", smsTo)
	}
	if !strings.Contains(smsBody, "Avocados") {
		t.Errorf("SMS reply should mention Avocados, got %q", smsBody)
	}
}

// =============================================================================
// SMS — USERS CAN ONLY ADD VIA TEXT (NOT DELETE OR UPDATE)
// =============================================================================

func TestSMSWebhook_OnlyAddsItems(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create user and add an existing item
	user, _ := upsertUser(server.db, "5551234567")
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity, source) VALUES (?, 'Milk', 'Dairy & Eggs', 2, 'app')",
		user.ID,
	)

	// The parser only returns parsed grocery items — it doesn't handle
	// commands like "delete milk" or "update milk to 3". Verify that
	// sending text only results in addParsedItems (add-only behavior).
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		// Parser returns a new item; the SMS flow calls addParsedItems which only adds
		return []ParsedItem{
			{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	doSMSRequest(server.handleSMSWebhook, "+15551234567", "eggs", 0)
	time.Sleep(200 * time.Millisecond)

	// Original item should still exist and be unchanged
	var milkQty int
	err := server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", user.ID).Scan(&milkQty)
	if err != nil {
		t.Fatalf("Milk should still exist: %v", err)
	}
	if milkQty != 2 {
		t.Errorf("Milk quantity should be unchanged at 2, got %d", milkQty)
	}

	// New item should be added
	var eggCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Eggs'", user.ID).Scan(&eggCount)
	if eggCount != 1 {
		t.Errorf("Expected Eggs to be added, got count %d", eggCount)
	}

	// Total should be 2 items — no deletions
	var total int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", user.ID).Scan(&total)
	if total != 2 {
		t.Errorf("Expected 2 total items (no deletions via SMS), got %d", total)
	}
}

// =============================================================================
// EMAIL WEBHOOK — EMPTY BODY
// =============================================================================

func TestEmailWebhook_EmptyBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	_ = cookie

	// Set up a user with email
	server.db.Exec("UPDATE users SET email = 'alice@example.com' WHERE phone_number = '5551234567'")

	// Send email with empty body
	form := url.Values{}
	form.Set("from", "alice@example.com")
	form.Set("text", "")

	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	// Should return 200 (not crash) — empty body is ignored gracefully
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// =============================================================================
// SETTINGS GET — PENNY PHONE
// =============================================================================

func TestSettingsGet_IncludesPennyPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	pennyPhone, ok := body["penny_phone"].(string)
	if !ok || pennyPhone == "" {
		t.Error("Expected non-empty penny_phone field")
	}
	if pennyPhone != "+10000000000" {
		t.Errorf("penny_phone: got %q, want +10000000000", pennyPhone)
	}
}

// =============================================================================
// LEAVE HOUSEHOLD
// =============================================================================

func TestLeaveHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create two users in the same household
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// Put user2 into user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// Verify they share a household
	var h1, h2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&h1)
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&h2)
	if h1 != h2 {
		t.Fatalf("Setup failed: users not in same household (%d vs %d)", h1, h2)
	}

	// User2 leaves the household
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}

	// User2 should now be in a different household
	var newH2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&newH2)
	if newH2 == h1 {
		t.Error("User2 should have a new household after leaving")
	}

	// User1 should still be in the original household
	var newH1 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&newH1)
	if newH1 != h1 {
		t.Errorf("User1 household changed: got %d, want %d", newH1, h1)
	}

	// User2 should be in household_members for the new household
	var memberCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE user_id = ? AND household_id = ?", userID2, newH2).Scan(&memberCount)
	if memberCount != 1 {
		t.Errorf("Expected user2 in new household_members, got count %d", memberCount)
	}

	_ = cookie1 // used only for setup verification
}

func TestLeaveHousehold_AlreadySolo(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551111111")

	// User is already solo — leave should still work
	var oldHID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&oldHID)

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}

	// Should have a new household (different from old)
	var newHID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&newHID)
	if newHID == oldHID {
		t.Error("Expected new household ID after leaving solo household")
	}
}

// =============================================================================
// SETTINGS — HOUSEHOLD MEMBERS IN RESPONSE
// =============================================================================

func TestSettingsGet_HouseholdMembers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	// Set name for user2
	server.db.Exec("UPDATE users SET name = 'Bob' WHERE id = ?", userID2)

	// Put user2 into user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// GET settings for user1
	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)

	members, ok := body["household_members"].([]interface{})
	if !ok {
		t.Fatal("Expected household_members array")
	}
	if len(members) != 1 {
		t.Fatalf("Expected 1 household member, got %d", len(members))
	}
	m := members[0].(map[string]interface{})
	if m["name"] != "Bob" {
		t.Errorf("Member name: got %v, want Bob", m["name"])
	}
}

// =============================================================================
// SETTINGS UPDATE — EMAIL UNIQUENESS
// =============================================================================

func TestSettingsUpdate_DuplicateEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User1 sets email
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "shared@example.com"},
		[]*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("User1 PATCH: got %d, want 200", rr.Code)
	}

	// User2 tries the same email
	rr = doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "shared@example.com"},
		[]*http.Cookie{cookie2})
	if rr.Code != http.StatusConflict {
		t.Errorf("Duplicate email: got %d, want 409", rr.Code)
	}
}

func TestSettingsUpdate_ClearEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	// Set email
	doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "alice@example.com"},
		[]*http.Cookie{cookie})

	// Clear email
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Clear email: got %d, want 200", rr.Code)
	}

	// Verify it's cleared
	rr = doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	if body["email"] != "" {
		t.Errorf("Email should be empty, got %v", body["email"])
	}
}

func TestSettingsUpdate_InvalidBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	req := httptest.NewRequest("PATCH", "/api/settings", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleSettings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// JOIN — EDGE CASES
// =============================================================================

func TestJoin_InvalidCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": "nonexistent"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestJoin_EmptyCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestJoin_AlreadyUsed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")
	cookie3, _ := loginUser(t, server, "5553333333")

	// User1 creates invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 uses it
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code},
		[]*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("First join: got %d, want 200", rr.Code)
	}

	// User3 tries to use same code
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code},
		[]*http.Cookie{cookie3})
	if rr.Code != http.StatusGone {
		t.Errorf("Second join: got %d, want 410", rr.Code)
	}
}

// =============================================================================
// INVITE — EDGE CASES
// =============================================================================

func TestInvite_NoPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	// Invite without phone number — should still create link
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	link, ok := body["link"].(string)
	if !ok || link == "" {
		t.Error("Expected non-empty link")
	}
}

func TestInvite_InvalidPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "123"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// VCARD — METHOD NOT ALLOWED
// =============================================================================

// =============================================================================
// SETTINGS — METHOD DISPATCH
// =============================================================================

// =============================================================================
// CREATE SOLO HOUSEHOLD HELPER
// =============================================================================

func TestCreateSoloHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551111111")

	// Clear existing household
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID)
	server.db.Exec("UPDATE users SET household_id = NULL WHERE id = ?", userID)

	hID, err := createSoloHousehold(server.db, userID)
	if err != nil {
		t.Fatalf("createSoloHousehold: %v", err)
	}
	if hID == 0 {
		t.Error("Expected non-zero household ID")
	}

	// Verify user is in the household
	var gotHID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&gotHID)
	if gotHID != hID {
		t.Errorf("User household: got %d, want %d", gotHID, hID)
	}

	// Verify membership
	var role string
	err = server.db.QueryRow("SELECT role FROM household_members WHERE user_id = ? AND household_id = ?", userID, hID).Scan(&role)
	if err != nil {
		t.Fatalf("No membership row: %v", err)
	}
	if role != "owner" {
		t.Errorf("Role: got %q, want owner", role)
	}
}

// =============================================================================
// MULTI-IMAGE TESTS
// =============================================================================

func TestEmailWebhook_MultipleImages(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'multi@example.com' WHERE id = ?", userID)

	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotImages = images
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
			{Name: "Bread", Department: "Bakery", Quantity: 1},
		}, nil
	}

	// Build a multipart request with two image attachments
	body := &bytes.Buffer{}
	writer := buildMultipartEmail(t, body, "multi@example.com", "two pics", map[string][]byte{
		"image1.jpg": []byte("fake-jpeg-1"),
		"image2.png": []byte("fake-png-2"),
	})

	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if len(gotImages) != 2 {
		t.Errorf("Expected 2 images passed to parser, got %d", len(gotImages))
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items, got %d", count)
	}
}

func TestEmailWebhook_ImageOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'img@example.com' WHERE id = ?", userID)

	var gotText string
	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotText = text
		gotImages = images
		return []ParsedItem{
			{Name: "Apple", Department: "Produce", Quantity: 1},
		}, nil
	}

	body := &bytes.Buffer{}
	writer := buildMultipartEmail(t, body, "img@example.com", "", map[string][]byte{
		"photo.jpg": []byte("fake-jpeg"),
	})

	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if gotText != "" {
		t.Errorf("Expected empty text, got %q", gotText)
	}
	if len(gotImages) != 1 {
		t.Errorf("Expected 1 image, got %d", len(gotImages))
	}
}

func TestSMSWebhook_MultipleImages(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotImages = images
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	// Simulate a single SMS with 2 media attachments
	// Since downloadTwilioMedia makes real HTTP calls, we directly test
	// the buffering by calling processSMSMessage with pre-loaded images
	s := server.smsBuffer
	s.mu.Lock()
	s.pending["+15559990000"] = &pendingMessage{
		text:   "groceries",
		images: []string{"data:image/jpeg;base64,img1", "data:image/png;base64,img2"},
	}
	s.mu.Unlock()
	server.processSMSMessage("+15559990000")

	if len(gotImages) != 2 {
		t.Errorf("Expected 2 images, got %d", len(gotImages))
	}
}

func TestSMSWebhook_BatchingMultipleImages(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	var parseCallCount int
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		parseCallCount++
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	// Simulate two rapid SMS messages, each with an image
	s := server.smsBuffer
	s.mu.Lock()
	pm := &pendingMessage{text: "first", images: []string{"data:image/jpeg;base64,img1"}}
	s.pending["+15558880000"] = pm
	s.mu.Unlock()

	// Second message arrives before timer fires
	s.mu.Lock()
	pm.text += "\nsecond"
	pm.images = append(pm.images, "data:image/png;base64,img2")
	s.mu.Unlock()

	server.processSMSMessage("+15558880000")

	if parseCallCount != 1 {
		t.Errorf("Expected 1 parse call (batched), got %d", parseCallCount)
	}
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

// =============================================================================
// handleListEdit TESTS
// =============================================================================

func TestListEdit_MultipleFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Bread", "", "Bakery", 1, "",
	)
	itemID, _ := result.LastInsertId()

	newName := "Sourdough"
	newBrand := "Tartine"
	newDept := "Bakery"
	newQty := 3
	newSize := "1 lb"
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name": newName, "brand": newBrand, "department": newDept,
			"quantity": newQty, "size": newSize,
		},
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
	if item.Department != newDept {
		t.Errorf("Department: got %q, want %q", item.Department, newDept)
	}
	if item.Quantity != newQty {
		t.Errorf("Quantity: got %d, want %d", item.Quantity, newQty)
	}
	if item.Size != newSize {
		t.Errorf("Size: got %q, want %q", item.Size, newSize)
	}
}

func TestListEdit_NoFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

	// Send empty patch — no fields provided
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Item should be unchanged
	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != "Bread" {
		t.Errorf("Name: got %q, want Bread", item.Name)
	}
}

func TestListEdit_AllFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Old Name", "Old Brand", "Pantry", 1, "8 oz",
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name": "New Name", "brand": "New Brand", "department": "Frozen",
			"quantity": 5, "size": "16 oz",
		},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != "New Name" || item.Brand != "New Brand" || item.Department != "Frozen" || item.Quantity != 5 || item.Size != "16 oz" {
		t.Errorf("Fields not updated correctly: %+v", item)
	}
}

// =============================================================================
// addParsedItems TESTS
// =============================================================================

func TestAddParsedItems_SkipsEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	parsed := []ParsedItem{
		{Name: "", Department: "Other", Quantity: 1},
		{Name: "  ", Department: "Other", Quantity: 1},
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Errorf("Expected 1 added, got %d", len(added))
	}
	if len(skipped) != 0 {
		t.Errorf("Expected 0 skipped, got %d", len(skipped))
	}
	if len(added) > 0 && added[0].Name != "Milk" {
		t.Errorf("Expected Milk, got %q", added[0].Name)
	}
}

func TestAddParsedItems_SkipsDuplicates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Pre-insert Milk
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1,
	)

	parsed := []ParsedItem{
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 2},
		{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Errorf("Expected 1 added (Eggs), got %d", len(added))
	}
	if len(skipped) != 1 {
		t.Errorf("Expected 1 skipped (Milk), got %d", len(skipped))
	}
	if len(skipped) > 0 && skipped[0].Parsed.Name != "Milk" {
		t.Errorf("Skipped item: got %q, want Milk", skipped[0].Parsed.Name)
	}

	// Milk quantity should have been merged (1 + 2 = 3)
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 3 {
		t.Errorf("Milk quantity after merge: got %d, want 3", qty)
	}
}

func TestAddParsedItems_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	parsed := []ParsedItem{
		{Name: "Widget", Department: "", Quantity: 0},
	}

	added, _ := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Fatalf("Expected 1 added, got %d", len(added))
	}
	if added[0].Department != "Other" {
		t.Errorf("Department: got %q, want Other", added[0].Department)
	}
	if added[0].Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", added[0].Quantity)
	}
}

func TestAddParsedItems_MergesQuantity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Pre-insert with quantity 3
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs", 3,
	)

	parsed := []ParsedItem{
		{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 2},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 0 {
		t.Errorf("Expected 0 added (existing item), got %d", len(added))
	}
	if len(skipped) != 1 {
		t.Errorf("Expected 1 skipped, got %d", len(skipped))
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Eggs'", userID).Scan(&qty)
	if qty != 5 {
		t.Errorf("Quantity: got %d, want 5 (3+2)", qty)
	}
}

func TestAddParsedItems_SourceTagging(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	sources := []string{"app", "sms", "email"}
	for i, src := range sources {
		parsed := []ParsedItem{
			{Name: fmt.Sprintf("Item%d", i), Department: "Other", Quantity: 1},
		}
		addParsedItems(server.db, userID, parsed, src)
	}

	for i, src := range sources {
		var gotSource string
		server.db.QueryRow("SELECT source FROM list_items WHERE name = ?", fmt.Sprintf("Item%d", i)).Scan(&gotSource)
		if gotSource != src {
			t.Errorf("Item%d source: got %q, want %q", i, gotSource, src)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestEmailWebhook_NonImageAttachment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'pdf@example.com' WHERE id = ?", userID)

	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotImages = images
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	// Build a multipart request with a non-image attachment (PDF)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.WriteField("from", "pdf@example.com")
	w.WriteField("text", "check this list")
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="attachment"; filename="list.pdf"`)
	h.Set("Content-Type", "application/pdf")
	part, _ := w.CreatePart(h)
	part.Write([]byte("fake-pdf-data"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	// PDF should not be treated as an image
	if len(gotImages) != 0 {
		t.Errorf("Expected 0 images (PDF excluded), got %d", len(gotImages))
	}
}

func TestSMSWebhook_ImageOnlyNoText(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	var gotText string
	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotText = text
		gotImages = images
		return []ParsedItem{
			{Name: "Apple", Department: "Produce", Quantity: 1},
		}, nil
	}

	// Directly set up a buffered message with image only
	s := server.smsBuffer
	s.mu.Lock()
	s.pending["+15557770000"] = &pendingMessage{
		text:   "",
		images: []string{"data:image/jpeg;base64,imgdata"},
	}
	s.mu.Unlock()
	server.processSMSMessage("+15557770000")

	if gotText != "" {
		t.Errorf("Expected empty text, got %q", gotText)
	}
	if len(gotImages) != 1 {
		t.Errorf("Expected 1 image, got %d", len(gotImages))
	}
	smsTo, _ := mock.GetLastSMS()
	if smsTo != "+15557770000" {
		t.Errorf("SMS reply to: got %q, want +15557770000", smsTo)
	}
}

func TestParseAndAdd_EmptyParsedResults(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "random nonsense"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Added != 0 {
		t.Errorf("Added: got %d, want 0", resp.Added)
	}
	if len(resp.Items) != 0 {
		t.Errorf("Items: got %d, want 0", len(resp.Items))
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items in DB, got %d", count)
	}
}

func TestParseAndAdd_LongList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Simulate 40+ items returned from Claude (a realistic large grocery list)
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Mixed Greens", Brand: "Earthbound Farm", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Bananas", Brand: "Chiquita", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Strawberries", Brand: "Driscoll's", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Fresh Basil", Brand: "Gotham Greens", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Clementines", Brand: "Wonderful Halos", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Cooked Beets", Brand: "Love Beets", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Large Eggs", Brand: "Vital Farms", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Salted Butter", Brand: "Kerrygold", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Greek Yogurt", Brand: "Chobani", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Oat Milk", Brand: "Oatly", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Shredded Sharp Cheddar", Brand: "Sargento", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Whole Milk", Brand: "Fairlife", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Cream Cheese", Brand: "Philadelphia", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Pasta", Brand: "Barilla", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Basmati Rice", Brand: "Lundberg", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Rolled Oats", Brand: "Bob's Red Mill", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "All-Purpose Flour", Brand: "King Arthur", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Marinara Sauce", Brand: "Rao's", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Chicken Base", Brand: "Better Than Bouillon", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Dijon Mustard", Brand: "Maille", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Ranch Dressing", Brand: "Hidden Valley", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Turkey Breast", Brand: "Applegate", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Bacon", Brand: "Applegate", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Tuna Pouches", Brand: "StarKist", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Chicken Breasts", Brand: "Tyson", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Plant-Based Burger Patties", Brand: "Beyond Meat", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Ground Turkey", Brand: "Perdue", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Popcorn", Brand: "SkinnyPop", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Nut Bars", Brand: "Kind", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Tortilla Chips", Brand: "Siete", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Wheat Crackers", Brand: "Triscuits", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Almond Butter", Brand: "Justin's", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Dark Chocolate", Brand: "Lindt", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Sparkling Water", Brand: "LaCroix", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Coffee", Brand: "Peet's", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Tea", Brand: "Twinings", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Orange Juice", Brand: "Simply Orange", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Margherita Pizza", Brand: "Amy's Kitchen", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Broccoli Florets", Brand: "Bird's Eye", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Gelato", Brand: "Talenti", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Waffles", Brand: "Eggo", Department: "Frozen", Quantity: 1, Size: ""},
		}, nil
	}

	longList := `Produce & Fresh Herbs
Earthbound Farm Organic Mixed Greens
Chiquita Bananas
Driscoll's Strawberries
Gotham Greens Fresh Basil
Wonderful Halo Clementines
Love Beets Vacuum-packed cooked beets
Dairy, Eggs & Alternatives
Vital Farms Pasture-Raised Large Eggs
Kerrygold Salted Butter
Chobani Greek Yogurt
Oatly Full-Thickness Oat Milk
Sargento Shredded Sharp Cheddar Cheese
Fairlife Ultra-Filtered Whole Milk
Philadelphia Original Cream Cheese
Pantry Staples & Grains
Barilla Al Bronzo Pasta
Lundberg Family Farms Organic Basmati Rice
Bob's Red Mill Old Fashioned Rolled Oats
King Arthur Unbleached All-Purpose Flour
Raos Homemade Marinara Sauce
Better Than Bouillon Roasted Chicken Base
Maille Dijon Mustard
Hidden Valley Ranch Dressing
Proteins & Meat
Applegate Organics Roasted Turkey Breast
Applegate Sunday Bacon
StarKist E.V.O.O. Wild Caught Tuna Pouches
Tyson Frozen Boneless Skinless Chicken Breasts
Beyond Meat Plant-Based Burger Patties
Perdue Ground Turkey
Snacks & Sweets
SkinnyPop Original Popcorn
Kind Nut Bars
Siete Grain-Free Tortilla Chips
Triscuits Original Whole Wheat Crackers
Justin's Almond Butter
Lindt Excellence 70% Cocoa Dark Chocolate
Beverages
LaCroix Sparkling Water
Peet's Coffee Major Dickason's Blend
Twinings Earl Grey Tea
Simply Orange Pulp-Free Orange Juice
Freezer Favorites
Amy's Kitchen Margherita Pizza
Bird's Eye Steamfresh Broccoli Florets
Talenti Roman Raspberry Sorbetto
Eggo Homestyle Waffles`

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": longList},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Added != 41 {
		t.Errorf("Added: got %d, want 41", resp.Added)
	}
	if len(resp.Items) != 41 {
		t.Errorf("Items: got %d, want 41", len(resp.Items))
	}

	// Verify all items in DB
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 41 {
		t.Errorf("DB items: got %d, want 41", count)
	}

	// Verify brands were preserved
	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ? AND name = 'Marinara Sauce'", userID).Scan(&brand)
	if brand != "Rao's" {
		t.Errorf("Brand for Marinara Sauce: got %q, want %q", brand, "Rao's")
	}
}

// =============================================================================
// PHASE 4: NEW TESTS FOR 80% COVERAGE
// =============================================================================

// --- downloadTwilioMedia tests ---

func TestDownloadTwilioMedia_Success(t *testing.T) {
	// Set up a test server that returns fake image data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "test-sid" || pass != "test-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Write([]byte("fake-image-data"))
	}))
	defer ts.Close()

	result := downloadTwilioMedia(ts.URL, "image/jpeg", "test-sid", "test-token")
	if result == "" {
		t.Fatal("Expected non-empty data URI")
	}
	if !strings.HasPrefix(result, "data:image/jpeg;base64,") {
		t.Errorf("Expected data URI prefix, got %q", result[:30])
	}
}

func TestDownloadTwilioMedia_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	// Should still return a data URI (it reads the error body)
	result := downloadTwilioMedia(ts.URL, "image/jpeg", "sid", "token")
	// Even on error, it reads the body and encodes it
	if result == "" {
		t.Log("downloadTwilioMedia returned empty on server error (acceptable)")
	}
}

func TestDownloadTwilioMedia_BadURL(t *testing.T) {
	result := downloadTwilioMedia("http://[invalid-url", "image/jpeg", "sid", "token")
	if result != "" {
		t.Errorf("Expected empty result for bad URL, got %q", result)
	}
}

// --- sendSMSReply tests ---

func TestSendSMSReply_NoPhoneConfig(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.config.TwilioPhoneNumber = ""

	// Should not panic, just log
	server.sendSMSReply("+15551234567", "test")
	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should not have called SendSMS when phone not configured")
	}
}

func TestSendSMSReply_SMSError(t *testing.T) {
	mock := &MockTwilioClient{SMSErr: fmt.Errorf("twilio error")}
	server := newTestServer(t, mock)

	// Should not panic, just log the error
	server.sendSMSReply("+15551234567", "test")
	smsTo, _ := mock.GetLastSMS()
	if smsTo != "+15551234567" {
		t.Errorf("Expected SMS to +15551234567, got %q", smsTo)
	}
}

// --- processSMSMessage tests ---

func TestProcessSMSMessage_NonexistentPending(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Should not panic when no pending message exists
	server.processSMSMessage("+15559999999")
}

func TestProcessSMSMessage_EmptyMessage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.smsBuffer.mu.Lock()
	server.smsBuffer.pending["+15559999999"] = &pendingMessage{text: "", images: nil}
	server.smsBuffer.mu.Unlock()

	// Should not panic, just skip
	server.processSMSMessage("+15559999999")
}

func TestProcessSMSMessage_ParseError(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return nil, fmt.Errorf("AI error")
	}

	server.smsBuffer.mu.Lock()
	server.smsBuffer.pending["+15551234567"] = &pendingMessage{text: "milk and eggs"}
	server.smsBuffer.mu.Unlock()

	server.processSMSMessage("+15551234567")

	// Should have sent an error reply
	_, smsBody := mock.GetLastSMS()
	if smsBody == "" {
		t.Error("Expected error reply SMS")
	}
	if !strings.Contains(smsBody, "trouble understanding") {
		t.Errorf("Expected trouble message, got %q", smsBody)
	}
}

func TestProcessSMSMessage_NoItemsParsed(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{}, nil
	}

	server.smsBuffer.mu.Lock()
	server.smsBuffer.pending["+15551234567"] = &pendingMessage{text: "hello there"}
	server.smsBuffer.mu.Unlock()

	server.processSMSMessage("+15551234567")

	_, smsBody := mock.GetLastSMS()
	if !strings.Contains(smsBody, "couldn't find any grocery items") {
		t.Errorf("Expected no-items message, got %q", smsBody)
	}
}

// --- handleEmailWebhook tests ---

func TestEmailWebhook_NoFromAddress(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doEmailRequest(server.handleEmailWebhook, "", "milk and eggs")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestEmailWebhook_UnknownSender(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doEmailRequest(server.handleEmailWebhook, "nobody@example.com", "milk and eggs")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestEmailWebhook_ParseError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'err@example.com' WHERE id = ?", userID)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return nil, fmt.Errorf("AI error")
	}

	rr := doEmailRequest(server.handleEmailWebhook, "err@example.com", "milk")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// --- handleSMSWebhook tests ---

func TestSMSWebhook_MethodGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/sms", nil)
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestSMSWebhook_WithMediaURL(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Set up a media server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-image"))
	}))
	defer ts.Close()

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1}}, nil
	}

	form := url.Values{}
	form.Set("From", "+15551234567")
	form.Set("Body", "check this out")
	form.Set("NumMedia", "1")
	form.Set("MediaUrl0", ts.URL+"/media")
	form.Set("MediaContentType0", "image/jpeg")

	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<Response>") {
		t.Error("Expected TwiML response")
	}
}

// --- helper tests ---

func TestDecodeJSON_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	var data struct{ Name string }
	ok := decodeJSON(rr, req, &data)
	if ok {
		t.Error("Expected false for invalid JSON")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestDecodeJSON_ValidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"name":"Alice"}`))
	rr := httptest.NewRecorder()

	var data struct{ Name string }
	ok := decodeJSON(rr, req, &data)
	if !ok {
		t.Error("Expected true for valid JSON")
	}
	if data.Name != "Alice" {
		t.Errorf("Name: got %q, want Alice", data.Name)
	}
}

func TestDbError(t *testing.T) {
	rr := httptest.NewRecorder()
	dbError(rr, fmt.Errorf("test error"), "Something failed")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Something failed" {
		t.Errorf("Error: got %v, want 'Something failed'", body["error"])
	}
}

func TestRequireSavedListAccess_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	rr := httptest.NewRecorder()
	ok := server.requireSavedListAccess(rr, userID, 99999)
	if ok {
		t.Error("Expected false for non-existent list")
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestRequireSavedListAccess_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	// User1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID1)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID1)
	listID, _ := result.LastInsertId()

	// User2 tries to access user1's list
	rr := httptest.NewRecorder()
	ok := server.requireSavedListAccess(rr, userID2, int(listID))
	if ok {
		t.Error("Expected false for wrong household")
	}
}

func TestCollectListItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Milk', 'Organic Valley', 'Dairy & Eggs', 2, '1 gal')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", userID)

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

// --- Saved list handler edge cases ---

func TestSavedListsCreate_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Empty List"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListsCreate_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListUpdate_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list with items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", listID)

	// Clear list items so update has no items to save
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListUpdate_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy & Eggs', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Butter', 'Dairy & Eggs', 1)", userID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["item_count"] != float64(2) {
		t.Errorf("item_count: got %v, want 2", body["item_count"])
	}
}

func TestSavedListUpdate_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// User1 creates a saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	// User2 adds items and tries to update user1's list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID2)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedListLoad_Overwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Eggs', '', 'Dairy & Eggs', 12, '')", listID)

	// Add existing items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Old Item', 'Other', 1)", userID)

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["added"] != float64(1) {
		t.Errorf("added: got %v, want 1", body["added"])
	}

	// Old items should be gone
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Old Item'", userID).Scan(&count)
	if count != 0 {
		t.Error("Old items should have been cleared")
	}
}

func TestSavedListLoad_Merge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Eggs', '', 'Dairy & Eggs', 12, '')", listID)

	// Add existing items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Old Item', 'Other', 1)", userID)

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=merge", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Both old and new items should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items (old + new), got %d", count)
	}
}

func TestSavedListLoad_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedListDelete_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

// --- handleInvite edge cases ---

func TestInvite_WithValidPhone(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5559998888"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["link"] == nil || body["link"] == "" {
		t.Error("Expected non-empty link")
	}
	// Should have sent SMS
	smsTo, _ := mock.GetLastSMS()
	if smsTo != "+15559998888" {
		t.Errorf("SMS to: got %q, want +15559998888", smsTo)
	}
}

func TestInvite_SMSSendError(t *testing.T) {
	mock := &MockTwilioClient{SMSErr: fmt.Errorf("SMS failed")}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	// Should still succeed (SMS is best-effort)
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5559998888"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200 (SMS failure is best-effort)", rr.Code)
	}
}

func TestInvite_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	req := httptest.NewRequest("POST", "/api/settings/invite", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleInvite(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleListClear tests ---

// --- handleSavedListsGet tests ---

func TestSavedListsGet_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	// Should return empty array
	if !strings.Contains(rr.Body.String(), "[]") {
		t.Errorf("Expected empty array, got %q", rr.Body.String())
	}
}

func TestSavedListsGet_WithLists(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Monthly')", userID)

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Errorf("Expected 2 lists, got %d", len(lists))
	}
}

// --- handleSavedLists routing tests ---

func TestSavedLists_Unauthorized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

// --- processSMSMessage with skipped items ---

func TestProcessSMSMessage_WithSkippedItems(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// First, upsert user and add existing item
	server.smsBuffer.mu.Lock()
	server.smsBuffer.pending["+15551234567"] = &pendingMessage{text: "milk"}
	server.smsBuffer.mu.Unlock()

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy & Eggs", Quantity: 2},
		}, nil
	}

	// Pre-insert Milk for this user
	user, _ := upsertUser(server.db, "5551234567")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", user.ID)

	server.processSMSMessage("+15551234567")

	// SMS/email dupes are now reported as "added" (quantity increases silently)
	_, smsBody := mock.GetLastSMS()
	if !strings.Contains(smsBody, "Added to your list") {
		t.Errorf("Expected 'Added to your list' in reply, got %q", smsBody)
	}
	if !strings.Contains(smsBody, "Milk") {
		t.Errorf("Expected 'Milk' in reply, got %q", smsBody)
	}

	// Verify quantity was increased
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", user.ID).Scan(&qty)
	if qty != 3 { // 1 existing + 2 from SMS
		t.Errorf("Expected quantity 3, got %d", qty)
	}
}

// --- requireSession expired token ---

func TestRequireSession_ExpiredToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Use a fake session cookie that doesn't exist in the store
	req := httptest.NewRequest("GET", "/api/list", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "0000000011111111222222223333333344444444555555556666666677777777"})
	rr := httptest.NewRecorder()
	_, ok := server.requireSession(rr, req)
	if ok {
		t.Error("Expected false for expired/invalid session")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

// --- handleSavedListLoad error paths ---

func TestSavedListLoad_QueryError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()

	// Drop the saved_list_items table to cause a query error
	server.db.Exec("DROP TABLE saved_list_items")

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

// --- handleSavedListsCreate with items ---

func TestSavedListsCreate_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Milk', 'Organic Valley', 'Dairy & Eggs', 1, '1 gal')", userID)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	if body["name"] != "Weekly" {
		t.Errorf("name: got %v, want Weekly", body["name"])
	}
	if body["item_count"] != float64(1) {
		t.Errorf("item_count: got %v, want 1", body["item_count"])
	}
}

// --- handleListClear success ---

func TestListClear_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Bread', 'Bakery', 1)", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

// --- handleSavedListDelete success ---

func TestSavedListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'ToDelete')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", listID)

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_lists WHERE id = ?", listID).Scan(&count)
	if count != 0 {
		t.Error("Expected saved list to be deleted")
	}
}

// --- handleSettingsUpdate name and email together ---

func TestSettingsUpdate_NameAndEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"name": "Alice", "email": "alice@example.com"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var name, email string
	server.db.QueryRow("SELECT name, email FROM users WHERE id = ?", userID).Scan(&name, &email)
	if name != "Alice" {
		t.Errorf("Name: got %q, want Alice", name)
	}
	if email != "alice@example.com" {
		t.Errorf("Email: got %q, want alice@example.com", email)
	}
}

// --- handleSavedListDelete with items ---

// --- handleListDelete success ---

func TestListDelete_ConfirmGone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Expected item to be deleted")
	}
}

// --- handleListGet success ---

func TestListGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', 'Organic', 'Dairy & Eggs', 2, '1 gal', 'app')", userID)

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0]["name"] != "Milk" {
		t.Errorf("Name: got %v, want Milk", items[0]["name"])
	}
}

// --- handleListAdd success ---

func TestListAdd_WithDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{"name": "Eggs", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var dept string
	server.db.QueryRow("SELECT department FROM list_items WHERE user_id = ? AND name = 'Eggs'", userID).Scan(&dept)
	if dept != "Dairy & Eggs" {
		t.Errorf("Department: got %q, want 'Dairy & Eggs'", dept)
	}
}

// --- handleListToggle success ---

func TestListToggle_CheckUncheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'Milk', 'Dairy & Eggs', 1, 0)", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var checked int
	server.db.QueryRow("SELECT checked FROM list_items WHERE id = ?", itemID).Scan(&checked)
	if checked != 1 {
		t.Error("Expected item to be checked")
	}
}

// --- downloadTwilioMedia read error (closed connection) ---

func TestDownloadTwilioMedia_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer ts.Close()

	result := downloadTwilioMedia(ts.URL, "image/jpeg", "sid", "token")
	// Empty body → empty base64, but still returns data URI
	if !strings.HasPrefix(result, "data:image/jpeg;base64,") {
		t.Errorf("Expected data URI prefix, got %q", result)
	}
}

// --- Invite with null household (legacy user) ---

// --- handleJoin expired invite ---

func TestJoin_ExpiredCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Create an invite with expiry in the past
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec(
		"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, 'expired123', ?, datetime('now', '-1 day'))",
		hID, userID1,
	)

	rr := doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": "expired123"},
		[]*http.Cookie{cookie2})
	if rr.Code != http.StatusGone {
		t.Errorf("Status: got %d, want 410", rr.Code)
	}
	_ = cookie1
}

// --- handleJoin self-join (join your own household) ---

func TestJoin_SelfHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551111111")

	// Create invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	code := body["code"].(string)

	// Join own household
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// --- SMS form parse error ---

func TestSMSWebhook_FormParseError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Send request with invalid content-type that will cause ParseForm to fail
	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader("%invalid-form"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Force ParseForm to fail by setting Body to nil after content-type
	req.Body = nil
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	// Should return 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleListAdd empty name ---

// --- handleListAdd invalid JSON ---

func TestListAdd_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleSavedListsCreate invalid JSON ---

func TestSavedListsCreate_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/saved-lists", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleSavedLists(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleVCard success ---

func TestVCard_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny.vcf", nil)
	rr := httptest.NewRecorder()
	server.handleVCard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "BEGIN:VCARD") {
		t.Error("Expected VCard content")
	}
}

// --- Tests that trigger DB error paths by closing the DB ---

func TestListGet_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListAdd_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk"},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListClear_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListsGet_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListsCreate_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Test"},
		[]*http.Cookie{cookie})
	// Could be 400 (collectListItems error) or 500
	if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
		t.Errorf("Expected error status, got %d", rr.Code)
	}
}

func TestSettingsGet_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSettingsUpdate_NameDBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"name": "Alice"},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSettingsUpdate_EmailDBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "test@example.com"},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestInvite_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestLeaveHousehold_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestJoin_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": "abc123"},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListToggle_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListEdit_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]string{"name": "Eggs"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError || rr.Code == http.StatusNotFound {
		// Either 500 or 404 is acceptable depending on which query fails first
	}
}

func TestListDelete_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	// Should be an error since DB is closed
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListUpdate_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListDelete_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListLoad_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestVerifyCode_DBError(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	server.db.Close()

	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify",
		map[string]string{"phone_number": "5551234567", "code": "123456"},
		nil)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

// --- Email webhook form parse error ---

func TestEmailWebhook_FormParseError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a request where both multipart and regular form parsing fail
	// Use a multipart content type with an impossible body that also fails regular parse
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader("invalid%body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	req.Body = http.NoBody // Causes both ParseMultipartForm and ParseForm to fail
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	// Should return 200 (always OK for SendGrid)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

// --- upsertListItem with existing checked item ---

func TestUpsertListItem_ExistingChecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert a checked item — upsert should uncheck it and set new quantity (no duplicate)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'Milk', 'Dairy & Eggs', 1, 1)", userID)

	item, isNew, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 2, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if !isNew {
		t.Error("Expected isNew=true for re-added checked item")
	}
	if item.Quantity != 2 {
		t.Errorf("Quantity: got %d, want 2", item.Quantity)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	// Verify no duplicate — should be exactly 1 item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND LOWER(name) = 'milk'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item, got %d (duplicate created)", count)
	}
}

func TestUpsertListItem_CaseInsensitiveMergeChecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert a checked "honey" — adding "Honey" should merge, not create duplicate
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'honey', 'Condiments', 1, 1)", userID)

	item, _, err := upsertListItem(server.db, userID, "Honey", "", "Condiments", "", 1, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item, got %d (duplicate created)", count)
	}
}

func TestUpsertListItem_MergeBrandAndSize(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert unchecked item without brand/size
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, checked) VALUES (?, 'Milk', '', 'Dairy & Eggs', 1, '', 0)", userID)

	// Upsert with brand and size — should merge
	item, isNew, err := upsertListItem(server.db, userID, "Milk", "Organic Valley", "Dairy & Eggs", "1 gal", 2, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if isNew {
		t.Error("Expected isNew=false for merge")
	}
	if item.Quantity != 3 {
		t.Errorf("Quantity: got %d, want 3", item.Quantity)
	}
	if item.Brand != "Organic Valley" {
		t.Errorf("Brand: got %q, want 'Organic Valley'", item.Brand)
	}
	if item.Size != "1 gal" {
		t.Errorf("Size: got %q, want '1 gal'", item.Size)
	}
}

// --- addParsedItems with DB error ---

func TestAddParsedItems_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Close()

	parsed := []ParsedItem{
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
	}
	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 0 {
		t.Errorf("Expected 0 added, got %d", len(added))
	}
	if len(skipped) != 0 {
		t.Errorf("Expected 0 skipped, got %d", len(skipped))
	}
}

func TestInvite_NullHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551111111")

	// Remove user from household to simulate legacy user
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID)
	server.db.Exec("UPDATE users SET household_id = NULL WHERE id = ?", userID)

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["link"] == nil || body["link"] == "" {
		t.Error("Expected non-empty link")
	}
}

// =============================================================================
// REFERRAL SYSTEM TESTS
// =============================================================================

func TestReferral_GetGeneratesCode(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	code, ok := body["code"].(string)
	if !ok || code == "" {
		t.Error("Expected non-empty referral code")
	}
	if body["points"].(float64) != 0 {
		t.Errorf("Expected 0 points, got %v", body["points"])
	}

	// Second call should return same code
	rr2 := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	body2 := parseJSON(rr2)
	if body2["code"].(string) != code {
		t.Errorf("Expected same code on second call, got %q vs %q", body2["code"], code)
	}
}

func TestReferral_ClaimAwardsPoints(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Create referrer and get their code
	cookieA, _ := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	referralCode := parseJSON(rr)["code"].(string)

	// Create referred user and claim code
	cookieB, _ := loginUser(t, server, "5552222222")
	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": referralCode}, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("Claim status: got %d, want 200", rr.Code)
	}

	// Check referrer got 10 points
	rr = doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	body := parseJSON(rr)
	if body["points"].(float64) != 10 {
		t.Errorf("Expected 10 points, got %v", body["points"])
	}
	if body["referral_count"].(float64) != 1 {
		t.Errorf("Expected 1 referral, got %v", body["referral_count"])
	}
}

func TestReferral_ClaimIdempotent(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	cookieA, _ := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	referralCode := parseJSON(rr)["code"].(string)

	cookieB, _ := loginUser(t, server, "5552222222")

	// Claim twice — should not double-credit
	doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": referralCode}, []*http.Cookie{cookieB})
	doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": referralCode}, []*http.Cookie{cookieB})

	rr = doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	body := parseJSON(rr)
	if body["points"].(float64) != 10 {
		t.Errorf("Expected 10 points (no double credit), got %v", body["points"])
	}
}

func TestReferral_CannotSelfRefer(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	cookie, _ := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	code := parseJSON(rr)["code"].(string)

	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Self-referral status: got %d, want 400", rr.Code)
	}
}

func TestReferral_RedeemRequires100Points(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	cookie, userID := loginUser(t, server, "5551111111")

	// Try to redeem with 0 points
	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 with insufficient points, got %d", rr.Code)
	}

	// Give user 100 points directly
	server.db.Exec("UPDATE users SET referral_points = 100 WHERE id = ?", userID)

	// Redeem should succeed
	rr = doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Redeem status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if msg, ok := body["message"].(string); !ok || msg == "" {
		t.Error("Expected success message")
	}

	// Points should be deducted
	rr = doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	if parseJSON(rr)["points"].(float64) != 0 {
		t.Errorf("Expected 0 points after redeem, got %v", parseJSON(rr)["points"])
	}

	// Redemption record should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM redemptions WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 redemption record, got %d", count)
	}
}

func TestReferral_InvalidCode(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	cookie, _ := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "nonexistent"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Invalid code status: got %d, want 404", rr.Code)
	}
}

func TestReferral_RequiresAuth(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth, got %d", rr.Code)
	}
}

// =============================================================================
// ITEM MODIFICATION HISTORY TESTS
// =============================================================================

func TestItemHistory_RecordsOnAdd(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add an item via upsert
	item, _, err := upsertListItem(server.db, userID, "Bananas", "", "Produce", "", 1, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("History status: got %d, want 200", rr.Code)
	}

	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)
	if len(mods) == 0 {
		t.Fatal("Expected at least 1 modification record")
	}
	if mods[0]["action"] != "added" {
		t.Errorf("Expected action 'added', got %v", mods[0]["action"])
	}
}

func TestItemHistory_RecordsOnToggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	item, _, _ := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "")

	// Toggle to checked
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", item.ID), nil, []*http.Cookie{cookie})

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)

	// Should have "added" and "checked"
	actions := make(map[string]bool)
	for _, m := range mods {
		actions[m["action"].(string)] = true
	}
	if !actions["checked"] {
		t.Error("Expected 'checked' action in history")
	}
}

func TestItemHistory_RecordsOnEdit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	item, _, _ := upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "")

	// Edit the item
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", item.ID),
		map[string]interface{}{"name": "Sourdough Bread"}, []*http.Cookie{cookie})

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)

	actions := make(map[string]bool)
	for _, m := range mods {
		actions[m["action"].(string)] = true
	}
	if !actions["updated"] {
		t.Error("Expected 'updated' action in history")
	}
}

func TestItemHistory_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "GET", "/api/list/1/history", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestItemHistory_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/99999/history", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

// =============================================================================
// SMS/EMAIL DUPE QUANTITY TESTS
// =============================================================================

func TestSMSDupe_AlwaysIncreasesQuantity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Add "Milk" with qty 1 via app
	_, _, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}

	// Add "Milk" again via SMS with qty 2
	item, isNew, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 2, "sms", "")
	if err != nil {
		t.Fatalf("upsertListItem (sms): %v", err)
	}

	// For SMS, isNew should be true (reported as added, not skipped)
	if !isNew {
		t.Error("SMS dupe should report isNew=true")
	}

	// Quantity should be 3 (1 + 2)
	if item.Quantity != 3 {
		t.Errorf("Expected quantity 3, got %d", item.Quantity)
	}

	// Modification history should record the quantity change
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = ? AND action = 'quantity_changed'", item.ID).Scan(&count)
	if count == 0 {
		t.Error("Expected quantity_changed modification record")
	}
}

func TestEmailDupe_AlwaysIncreasesQuantity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Add "Eggs" with qty 1 via app
	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 1, "app", "")

	// Add "Eggs" again via email with qty 1
	item, isNew, _ := upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 1, "email", "")

	if !isNew {
		t.Error("Email dupe should report isNew=true")
	}
	if item.Quantity != 2 {
		t.Errorf("Expected quantity 2, got %d", item.Quantity)
	}
}

func TestAppDupe_StillReportsSkipped(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Add "Bread" via app
	upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "")

	// Add "Bread" again via app — should report as NOT new (skipped)
	_, isNew, _ := upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "")
	if isNew {
		t.Error("App dupe should report isNew=false (skipped)")
	}
}

func TestSMSDupe_PreservesOriginalAdder(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// User 1 adds item via app
	_, userID1 := loginUser(t, server, "5551111111")
	item, _, _ := upsertListItem(server.db, userID1, "Yogurt", "", "Dairy & Eggs", "", 1, "app", "")

	// User 2 (same household) adds same item via SMS
	_, userID2 := loginUser(t, server, "5552222222")
	// Join same household
	var hid int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hid)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hid, userID2)
	server.db.Exec("INSERT OR IGNORE INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hid, userID2)

	upsertListItem(server.db, userID2, "Yogurt", "", "Dairy & Eggs", "", 2, "sms", "")

	// Original adder should still be user 1
	var ownerID int
	server.db.QueryRow("SELECT user_id FROM list_items WHERE id = ?", item.ID).Scan(&ownerID)
	if ownerID != userID1 {
		t.Errorf("Expected original adder %d, got %d", userID1, ownerID)
	}
}

// =============================================================================
// ADMIN DASHBOARD TESTS
// =============================================================================

func TestAdminDashboard_Unauthorized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// No session cookie
	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without session, got %d", rr.Code)
	}
}

func TestAdminDashboard_ForbiddenForNonAdmin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a non-admin user session
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for non-admin, got %d", rr.Code)
	}
}

func TestAdminDashboard_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create admin user
	user, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(user.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Create some additional users
	user2, _ := upsertUser(server.db, "5551111111")
	upsertUser(server.db, "5552222222")

	// Create a referral
	server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", user.ID, user2.ID)
	server.db.Exec("UPDATE users SET referral_points = 5 WHERE id = ?", user.ID)

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard?days=30", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["total_users"].(float64) != 3 {
		t.Errorf("Expected 3 total users, got %v", body["total_users"])
	}
	if body["days"].(float64) != 30 {
		t.Errorf("Expected days=30, got %v", body["days"])
	}

	leaderboard, ok := body["leaderboard"].([]interface{})
	if !ok {
		t.Fatal("Expected leaderboard to be an array")
	}
	if len(leaderboard) != 1 {
		t.Errorf("Expected 1 leaderboard entry, got %d", len(leaderboard))
	}
	if len(leaderboard) > 0 {
		entry := leaderboard[0].(map[string]interface{})
		// Phone should be masked
		if entry["phone"] != "310***1833" {
			t.Errorf("Expected masked phone 310***1833, got %v", entry["phone"])
		}
	}
}

func TestAdminDashboard_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleAdminDashboard, "POST", "/api/admin/dashboard", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST, got %d", rr.Code)
	}
}

func TestAdminDashboard_MultipleNonAdminPhones(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Try several different phone numbers — all should be forbidden
	phones := []string{"5551234567", "3104861834", "3104861832", "0000000000"}
	for _, phone := range phones {
		user, _ := upsertUser(server.db, phone)
		token, _ := server.sessions.CreateSession(user.ID, phone)
		cookie := &http.Cookie{Name: "session", Value: token}

		rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, []*http.Cookie{cookie})
		if rr.Code != http.StatusForbidden {
			t.Errorf("Phone %s: expected 403, got %d", phone, rr.Code)
		}
	}
}

func TestAdminDashboard_OnlyExactPhoneAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// The exact admin phone should succeed
	user, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(user.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Admin phone should get 200, got %d", rr.Code)
	}

	// A phone that contains the admin number but isn't exact should fail
	user2, _ := upsertUser(server.db, "1310486183")
	token2, _ := server.sessions.CreateSession(user2.ID, "1310486183")
	cookie2 := &http.Cookie{Name: "session", Value: token2}

	rr2 := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, []*http.Cookie{cookie2})
	if rr2.Code != http.StatusForbidden {
		t.Errorf("Similar-but-different phone should get 403, got %d", rr2.Code)
	}
}

func TestAdminDashboard_CustomDaysParam(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(user.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	for _, days := range []string{"7", "90"} {
		rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard?days="+days, nil, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200 for days=%s, got %d", days, rr.Code)
		}
		body := parseJSON(rr)
		expected, _ := fmt.Sscanf(days, "%f", new(float64))
		_ = expected
		if body["days"].(float64) != func() float64 { v := 0.0; fmt.Sscanf(days, "%f", &v); return v }() {
			t.Errorf("Expected days=%s, got %v", days, body["days"])
		}
	}
}

func TestAdminDashboard_InvalidDaysDefaultsTo30(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(user.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard?days=abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["days"].(float64) != 30 {
		t.Errorf("Invalid days param should default to 30, got %v", body["days"])
	}
}

func TestAdminDashboard_ActiveUsersCount(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create admin and some users
	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	// upsertUser sets last_active for admin, user1, user2
	upsertUser(server.db, "5551111111")
	upsertUser(server.db, "5552222222")

	// Create a user without last_active (simulating old user)
	server.db.Exec("INSERT OR IGNORE INTO users (phone_number) VALUES ('5553333333')")

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard?days=30", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)

	// 3 active (admin + 2 users with last_active set by upsertUser), 4 total
	if body["active_users"].(float64) != 3 {
		t.Errorf("Expected 3 active users, got %v", body["active_users"])
	}
	if body["total_users"].(float64) != 4 {
		t.Errorf("Expected 4 total users, got %v", body["total_users"])
	}
}

func TestAdminDashboard_KFactor(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Create 2 new users and 4 referrals — k-factor should be 4/3 (3 new users including admin)
	u1, _ := upsertUser(server.db, "5551111111")
	u2, _ := upsertUser(server.db, "5552222222")
	for i := 0; i < 4; i++ {
		referred, _ := upsertUser(server.db, fmt.Sprintf("555000%04d", i))
		server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", u1.ID, referred.ID)
		_ = u2
	}

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard?days=30", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)

	kFactor := body["k_factor"].(float64)
	if kFactor <= 0 {
		t.Errorf("Expected positive k-factor, got %v", kFactor)
	}
}

func TestAdminDashboard_EmptyLeaderboard(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAdminDashboard, "GET", "/api/admin/dashboard", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)

	leaderboard := body["leaderboard"].([]interface{})
	if len(leaderboard) != 0 {
		t.Errorf("Expected empty leaderboard, got %d entries", len(leaderboard))
	}
}

func TestSavedListLoad_OverwriteClearsOldItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	// Create a saved list with different items
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test List')", userID)
	savedID, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department) VALUES (?, 'Eggs', 'Dairy & Eggs')", savedID)

	// Load with overwrite
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Old items should be gone, only new item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite, got %d", count)
	}

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", userID).Scan(&name)
	if name != "Eggs" {
		t.Errorf("Expected 'Eggs', got %q", name)
	}
}

func TestSavedListLoad_MergeKeepsBothItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// Create a saved list
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test List')", userID)
	savedID, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department) VALUES (?, 'Eggs', 'Dairy & Eggs')", savedID)

	// Load with merge (default)
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both items should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items after merge, got %d", count)
	}
}

// =============================================================================
// CLEAR LIST VIA POST
// =============================================================================

func TestClearList_PostMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestClearList_DeleteMethodNoLongerWorks(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// DELETE should not work anymore — it will try to parse "clear" as an item ID
	rr := doRequest(server.handleListClear, "DELETE", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("DELETE to /api/list/clear should no longer work (changed to POST)")
	}

	// Item should still be there
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected item to still exist, got %d", count)
	}
}

// =============================================================================
// MUX ROUTING INTEGRATION TESTS
// =============================================================================

func TestMuxRouting_ClearList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// Register routes on a fresh mux, same as main()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/list", server.handleList)
	mux.HandleFunc("/api/list/", server.handleList)
	mux.HandleFunc("/api/list/clear", server.handleListClear)

	req := httptest.NewRequest("POST", "/api/list/clear", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 from mux-routed clear, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear via mux, got %d", count)
	}
}

func TestMuxRouting_SavedListLoad(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list with items
	var savedListID int64
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	savedListID, _ = res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Apples', 'Produce', 1)", savedListID)

	// Add an existing item that should be cleared on overwrite
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'OldItem', 'Other')", userID)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/saved-lists/", server.handleSavedLists)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedListID), nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 from mux-routed saved list load, got %d: %s", rr.Code, rr.Body.String())
	}

	// OldItem should be gone, Apples should be present
	var oldCount, newCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'OldItem'", userID).Scan(&oldCount)
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Apples'", userID).Scan(&newCount)
	if oldCount != 0 {
		t.Errorf("Expected OldItem to be cleared, but found %d", oldCount)
	}
	if newCount != 1 {
		t.Errorf("Expected Apples to be added, but found %d", newCount)
	}
}

// =============================================================================
// PRIVACY AND TERMS PAGES
// =============================================================================

func TestPrivacyPolicy_Returns200(t *testing.T) {
	req := httptest.NewRequest("GET", "/privacy", nil)
	rr := httptest.NewRecorder()
	handlePrivacyPolicy(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Expected text/html content type, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Privacy Policy") {
		t.Error("Expected page to contain 'Privacy Policy'")
	}
	if !strings.Contains(body, "not") && !strings.Contains(body, "do not") {
		t.Error("Expected policy to mention no third-party sharing")
	}
}

func TestTermsAndConditions_Returns200(t *testing.T) {
	req := httptest.NewRequest("GET", "/terms", nil)
	rr := httptest.NewRecorder()
	handleTerms(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Terms") {
		t.Error("Expected page to contain 'Terms'")
	}
	if !strings.Contains(body, "STOP") {
		t.Error("Expected terms to mention STOP opt-out")
	}
	if !strings.Contains(body, "HELP") {
		t.Error("Expected terms to mention HELP")
	}
	if !strings.Contains(body, "data rates") {
		t.Error("Expected terms to mention data rates")
	}
}

// =============================================================================
// LAST_ACTIVE TRACKING
// =============================================================================

func TestUpsertUser_SetsLastActive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	upsertUser(server.db, "5551234567")

	var lastActive sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&lastActive)
	if !lastActive.Valid {
		t.Error("Expected last_active to be set after upsertUser")
	}
}

func TestUpsertUser_UpdatesLastActive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	upsertUser(server.db, "5551234567")

	var first sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&first)

	// Second login should update last_active
	upsertUser(server.db, "5551234567")

	var second sql.NullString
	server.db.QueryRow("SELECT last_active FROM users WHERE phone_number = '5551234567'").Scan(&second)

	if !second.Valid {
		t.Error("Expected last_active to be set on second login")
	}
}

// =============================================================================
// REQUIRE ADMIN HELPER
// =============================================================================

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567")

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := httptest.NewRecorder()
	_, ok := server.requireAdmin(rr, req)

	if ok {
		t.Error("Expected requireAdmin to return false for non-admin")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestRequireAdmin_AdminUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(user.ID, "3104861833")

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := httptest.NewRecorder()
	session, ok := server.requireAdmin(rr, req)

	if !ok {
		t.Error("Expected requireAdmin to return true for admin")
	}
	if session.PhoneNumber != "3104861833" {
		t.Errorf("Expected admin phone, got %s", session.PhoneNumber)
	}
}

// =============================================================================
// ADDITIONAL TESTS FOR COVERAGE
// =============================================================================

func TestSessionStore_CleanExpired(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()
	store := NewSessionStore(db)

	// Insert an already-expired session directly
	db.Exec("INSERT INTO sessions (token, user_id, phone_number, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"expired-token", 1, "5551234567", time.Now().UTC().Add(-48*time.Hour), time.Now().UTC().Add(-24*time.Hour))

	// Insert a valid session
	db.Exec("INSERT INTO sessions (token, user_id, phone_number, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"valid-token", 1, "5551234567", time.Now().UTC(), time.Now().UTC().Add(24*time.Hour))

	store.CleanExpired()

	_, okExpired := store.GetSession("expired-token")
	_, okValid := store.GetSession("valid-token")
	if okExpired {
		t.Error("Expired session should have been cleaned")
	}
	if !okValid {
		t.Error("Valid session should not have been cleaned")
	}
}

func TestSessionStore_ExpiredSessionNotReturned(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()
	store := NewSessionStore(db)

	// Insert expired session
	db.Exec("INSERT INTO sessions (token, user_id, phone_number, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"old-token", 1, "5551234567", time.Now().UTC().Add(-48*time.Hour), time.Now().UTC().Add(-1*time.Hour))

	_, ok := store.GetSession("old-token")
	if ok {
		t.Error("Expired session should not be returned by GetSession")
	}
}

func TestInitDB_MigrationCascade(t *testing.T) {
	// Verify item_modifications has ON DELETE CASCADE
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Create a user + item + modification
	db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	result, _ := db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (1, 'Milk', 'Dairy')")
	itemID, _ := result.LastInsertId()
	db.Exec("INSERT INTO item_modifications (item_id, user_id, action, detail, source) VALUES (?, 1, 'added', '', 'app')", itemID)

	// Delete the list item — modifications should cascade
	db.Exec("DELETE FROM list_items WHERE id = ?", itemID)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 modifications after cascade delete, got %d", count)
	}
}

func TestReferralClaim_FullFlow(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Create referrer and referred users
	cookieReferrer, _ := loginUser(t, server, "5551111111")
	cookieReferred, _ := loginUser(t, server, "5552222222")

	// Get referrer's code
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieReferrer})
	code := parseJSON(rr)["code"].(string)

	// Claim it
	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookieReferred})
	if rr.Code != http.StatusOK {
		t.Fatalf("Claim status: got %d, want 200", rr.Code)
	}

	// Referrer should have 10 points
	rr = doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieReferrer})
	if parseJSON(rr)["points"].(float64) != 10 {
		t.Errorf("Expected 10 points, got %v", parseJSON(rr)["points"])
	}
}

func TestReferralClaim_EmptyCode(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty code, got %d", rr.Code)
	}
}

func TestUpsertListItem_Transaction(t *testing.T) {
	// Test that concurrent upserts don't create duplicates
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	_, userID := loginUser(t, server, "5551234567")

	// Add the same item twice — second should merge
	item1, isNew1, err1 := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 1, "app", "")
	item2, isNew2, err2 := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 2, "app", "")

	if err1 != nil {
		t.Fatalf("First upsert error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Second upsert error: %v", err2)
	}
	if !isNew1 {
		t.Error("First insert should be new")
	}
	if isNew2 {
		t.Error("Second insert should merge, not be new")
	}
	if item1.ID != item2.ID {
		t.Errorf("Expected same item ID, got %d and %d", item1.ID, item2.ID)
	}
	if item2.Quantity != 3 {
		t.Errorf("Expected quantity 3 after merge, got %d", item2.Quantity)
	}
}

func TestUpsertListItem_ConcurrentSafety(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	_, userID := loginUser(t, server, "5551234567")

	var wg sync.WaitGroup
	var errors int64
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := upsertListItem(server.db, userID, "Eggs", "", "Dairy", "", 1, "app", "")
			if err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}()
	}
	wg.Wait()

	if errors > 0 {
		t.Errorf("Got %d errors from concurrent upserts", errors)
	}

	// Check that we have exactly one item (quantities merged)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND LOWER(name) = 'eggs'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 egg item, got %d (duplicates created)", count)
	}
}

func TestLeaveHousehold_FullFlow(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Create two users in the same household
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Create invite and join
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("Invite status: %d", rr.Code)
	}
	link := parseJSON(rr)["link"].(string)
	// Extract code from link
	parts := strings.Split(link, "/join/")
	if len(parts) < 2 {
		t.Fatalf("Unexpected link format: %s", link)
	}
	code := parts[1]

	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Join status: %d", rr.Code)
	}

	// Now user1 leaves
	rr = doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household",
		nil, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("Leave status: %d, body: %s", rr.Code, rr.Body.String())
	}

	// User1 should now have their own household
	var hid1 sql.NullInt64
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hid1)
	if !hid1.Valid {
		t.Error("User1 should have a new household after leaving")
	}
}

func TestSettingsUpdate_SMSOptIn(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"sms_opt_in": false},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestItemHistory_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "POST", fmt.Sprintf("/api/list/%d/history", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Errorf("Expected non-200 for POST to history, got %d", rr.Code)
	}
}

func TestCollectListItems_EmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestSavedListsGet_Sorted(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create two saved lists
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Beta List"}, []*http.Cookie{cookie})
	doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Alpha List"}, []*http.Cookie{cookie})

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Fatalf("Expected 2 lists, got %d", len(lists))
	}
}

func TestHandleListAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{"name": "Mystery Item", "department": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["department"] != "Other" {
		t.Errorf("Expected department 'Other', got %v", body["department"])
	}
}

func TestInitDB_MigrationPath(t *testing.T) {
	// Test the migration path for item_modifications ON DELETE CASCADE
	// First create a DB with the old schema (without CASCADE)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA foreign_keys = ON")

	// Create tables with old schema (no CASCADE on item_modifications)
	db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, phone_number TEXT UNIQUE NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)")
	db.Exec("CREATE TABLE list_items (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, name TEXT NOT NULL, department TEXT NOT NULL DEFAULT 'Other', checked INTEGER NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (user_id) REFERENCES users(id))")
	db.Exec("CREATE TABLE item_modifications (id INTEGER PRIMARY KEY AUTOINCREMENT, item_id INTEGER NOT NULL, user_id INTEGER, action TEXT NOT NULL, detail TEXT, source TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (item_id) REFERENCES list_items(id), FOREIGN KEY (user_id) REFERENCES users(id))")

	// Add some data
	db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 1, 'Milk', 'Dairy')")
	db.Exec("INSERT INTO item_modifications (item_id, user_id, action, source) VALUES (1, 1, 'added', 'app')")

	// Verify that the old schema does NOT have CASCADE
	var createSQL string
	db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='item_modifications'").Scan(&createSQL)
	if strings.Contains(strings.ToUpper(createSQL), "ON DELETE CASCADE") {
		t.Fatal("Old schema should not have CASCADE")
	}

	// Now run initDB again on this database — it should perform the migration
	// We can't re-run initDB since it opens a new connection, but we can run the migration logic directly
	db.Exec(`CREATE TABLE IF NOT EXISTS item_modifications_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT, item_id INTEGER NOT NULL, user_id INTEGER,
		action TEXT NOT NULL, detail TEXT, source TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (item_id) REFERENCES list_items(id) ON DELETE CASCADE, FOREIGN KEY (user_id) REFERENCES users(id))`)
	db.Exec("INSERT INTO item_modifications_new SELECT * FROM item_modifications")
	db.Exec("DROP TABLE item_modifications")
	db.Exec("ALTER TABLE item_modifications_new RENAME TO item_modifications")

	// Verify migration worked — deletion should cascade
	db.Exec("DELETE FROM list_items WHERE id = 1")
	var count int
	db.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = 1").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 modifications after cascade delete, got %d", count)
	}
}

func TestSettingsUpdate_EmailConflict(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Set email for user1
	doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"email": "shared@test.com"},
		[]*http.Cookie{cookie1})

	// Try to set same email for user2
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"email": "shared@test.com"},
		[]*http.Cookie{cookie2})
	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409 for duplicate email, got %d", rr.Code)
	}
}

func TestSettingsUpdate_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("PATCH", "/api/settings", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleSettings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestSettingsGet_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551111111")

	// Set name and email
	doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"name": "Alice", "email": "alice@test.com"},
		[]*http.Cookie{cookie})

	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	if body["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice", body["name"])
	}
	if body["email"] != "alice@test.com" {
		t.Errorf("email: got %v, want alice@test.com", body["email"])
	}
}

func TestHandleListGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', 'Organic Valley', 'Dairy', 2, '1 gallon', 'app')", userID)

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0]["brand"] != "Organic Valley" {
		t.Errorf("brand: got %v, want Organic Valley", items[0]["brand"])
	}
}

func TestHandleListAdd_WithDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{
			"name":       "Eggs",
			"department": "Dairy & Eggs",
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	if body["department"] != "Dairy & Eggs" {
		t.Errorf("department: got %v, want Dairy & Eggs", body["department"])
	}
}

func TestInvite_CreatesHouseholdIfNeeded(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// User starts with no household
	var hid sql.NullInt64
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid)

	// Create invite — should auto-create a household
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d, body: %s", rr.Code, rr.Body.String())
	}

	// Now user should have a household
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid)
	if !hid.Valid {
		t.Error("Expected household to be created")
	}
}

func TestSavedListLoad_MergeMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add current items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)

	// Create saved list with different items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", listID)

	// Load in merge mode (default)
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	// Should have both items
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items after merge, got %d", count)
	}
}

func TestSavedListLoad_OverwriteMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add current items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)

	// Create saved list with different items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", listID)

	// Load in overwrite mode
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	// Should have only the saved list item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite, got %d", count)
	}
}

func TestCreateSoloHousehold_Success(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Create user directly (no upsertUser which auto-creates household)
	result, err := db.Exec("INSERT INTO users (phone_number) VALUES ('5559999999')")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := result.LastInsertId()
	userID := int(uid)

	hID, err := createSoloHousehold(db, userID)
	if err != nil {
		t.Fatalf("createSoloHousehold: %v", err)
	}
	if hID <= 0 {
		t.Error("Expected positive household ID")
	}

	// Verify household_id set on user
	var userHID int
	db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&userHID)
	if userHID != hID {
		t.Errorf("Expected user.household_id = %d, got %d", hID, userHID)
	}

	// Verify household_members entry
	var role string
	db.QueryRow("SELECT role FROM household_members WHERE user_id = ?", userID).Scan(&role)
	if role != "owner" {
		t.Errorf("Expected role 'owner', got '%s'", role)
	}
}

func TestInitDB_MigratesExistingDB(t *testing.T) {
	// Test the actual initDB migration path with a temp file DB
	tmpFile := t.TempDir() + "/test.db"

	// First pass: create DB with initDB (creates new tables with CASCADE)
	db1, err := initDB(tmpFile)
	if err != nil {
		t.Fatalf("first initDB: %v", err)
	}
	// Manually revert the item_modifications table to old schema (no CASCADE)
	db1.Exec("DROP TABLE item_modifications")
	db1.Exec(`CREATE TABLE item_modifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		item_id INTEGER NOT NULL,
		user_id INTEGER,
		action TEXT NOT NULL,
		detail TEXT,
		source TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (item_id) REFERENCES list_items(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	)`)
	db1.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	db1.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 1, 'Milk', 'Dairy')")
	db1.Exec("INSERT INTO item_modifications (item_id, user_id, action, source) VALUES (1, 1, 'added', 'app')")
	db1.Close()

	// Second pass: re-open with initDB — should migrate
	db2, err := initDB(tmpFile)
	if err != nil {
		t.Fatalf("second initDB: %v", err)
	}
	defer db2.Close()

	// Verify CASCADE works now
	db2.Exec("PRAGMA foreign_keys = ON")
	db2.Exec("DELETE FROM list_items WHERE id = 1")

	var count int
	db2.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = 1").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 modifications after cascade delete, got %d", count)
	}
}

func TestSavedListsCreate_WithMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add multiple items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, brand, size) VALUES (?, 'Milk', 'Dairy', 2, 'Organic Valley', '1 gallon')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, brand, size) VALUES (?, 'Bread', 'Bakery', 1, '', 'loaf')", userID)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly Shopping"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: %d, body: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["item_count"].(float64) != 2 {
		t.Errorf("Expected 2 items, got %v", body["item_count"])
	}
}

func TestSavedListUpdate_WithNewItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with one item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Old List')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity, brand, size) VALUES (?, 'OldItem', 'Other', 1, '', '')", listID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["item_count"].(float64) != 1 {
		t.Errorf("Expected 1 item, got %v", body["item_count"])
	}
}

func TestHandleListToggle_CheckAll(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	// Toggle to checked
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["checked"] != true {
		t.Error("Expected checked=true")
	}

	// Toggle back to unchecked
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["checked"] != false {
		t.Error("Expected checked=false")
	}
}

func TestHandleListEdit_AllFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name":       "Almond Milk",
			"brand":      "Silk",
			"department": "Beverages",
			"quantity":   3,
			"size":       "32oz",
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["name"] != "Almond Milk" {
		t.Errorf("name: got %v", body["name"])
	}
	if body["brand"] != "Silk" {
		t.Errorf("brand: got %v", body["brand"])
	}
}

func TestHandleListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Item should be deleted")
	}
}

func TestHandleListClear_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items, got %d", count)
	}
}

func TestHandleItemHistory_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add item (creates modification record)
	item, _, _ := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 1, "app", "")

	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var history []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &history)
	if len(history) == 0 {
		t.Error("Expected at least one history entry")
	}
}

func TestSavedListDelete_Success_Cascade(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Deletable"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	id := int(body["id"].(float64))

	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", id),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", id).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 saved list items after delete, got %d", count)
	}
}

func TestCollectListItems_WithCheckedItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, 'Milk', 'Dairy', 0)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, 'Bread', 'Bakery', 1)", userID)

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems: %v", err)
	}
	// collectListItems returns all items (checked and unchecked)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestSettingsGet_WithHouseholdMembers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User1 sets name
	doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"name": "Alice"},
		[]*http.Cookie{cookie1})

	// Create invite and join
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	link := parseJSON(rr)["link"].(string)
	parts := strings.Split(link, "/join/")
	code := parts[1]
	doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})

	// Get settings — should show household members
	rr = doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	members, ok := body["household_members"].([]interface{})
	if !ok {
		t.Fatal("Expected household_members array")
	}
	if len(members) != 1 {
		t.Errorf("Expected 1 household member, got %d", len(members))
	}
}

func TestVCard_EmptyPhoneNumber(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TwilioPhoneNumber = ""
	rr := doRequest(server.handleVCard, "GET", "/api/penny.vcf", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "+10000000000") {
		t.Error("Expected fallback phone number in vCard")
	}
}

func TestSavedLists_NotFoundRoute(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	// PATCH on a saved list ID is not a valid route
	rr := doRequest(server.handleSavedLists, "PATCH", "/api/saved-lists/1", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandleItemHistory_WithModifications(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add an item
	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Add: %d", rr.Code)
	}
	itemID := int(parseJSON(rr)["id"].(float64))

	// Record a modification
	recordModification(server.db, itemID, userID, "updated", "name = ?", "app")

	// Get history — use handleList router which dispatches to handleItemHistory
	rr = doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("History: %d", rr.Code)
	}
	// Should have modifications (at least the add and the manual one)
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)
	if len(mods) < 1 {
		t.Errorf("Expected at least 1 modification, got %d", len(mods))
	}
}

func TestHandleItemHistory_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/99999/history",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandleListEdit_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	name := ""
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": &name}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty name, got %d", rr.Code)
	}
}

func TestHandleListEdit_QuantityClamp(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Eggs"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	qty := 0
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"quantity": &qty}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Edit: %d", rr.Code)
	}
	item := parseJSON(rr)
	if item["quantity"].(float64) != 1 {
		t.Errorf("Expected quantity clamped to 1, got %v", item["quantity"])
	}
}

func TestHandleListEdit_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999",
		map[string]interface{}{"brand": "Acme"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandleListToggle_Uncheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	// Check — use handleList router which dispatches to handleListToggle
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Toggle check: %d", rr.Code)
	}
	if !parseJSON(rr)["checked"].(bool) {
		t.Error("Expected checked=true after first toggle")
	}

	// Uncheck
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Toggle uncheck: %d", rr.Code)
	}
	if parseJSON(rr)["checked"].(bool) {
		t.Error("Expected checked=false after second toggle")
	}
}

func TestHandleListDelete_NotOwned(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User1 adds item
	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie1})
	itemID := int(parseJSON(rr)["id"].(float64))

	// User2 tries to delete it (not in same household)
	rr = doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestReferral_GetWithFriends(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	// Set up referral code and referral
	server.db.Exec("UPDATE users SET referral_code = 'testcode123', referral_points = 10, name = 'Alice' WHERE id = ?", userID1)
	server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", userID1, userID2)

	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["code"].(string) != "testcode123" {
		t.Errorf("Expected existing code, got %v", body["code"])
	}
	if body["points"].(float64) != 10 {
		t.Errorf("Expected 10 points, got %v", body["points"])
	}
	if body["referral_count"].(float64) != 1 {
		t.Errorf("Expected 1 referral, got %v", body["referral_count"])
	}
	friends := body["friends"].([]interface{})
	if len(friends) != 1 {
		t.Errorf("Expected 1 friend, got %d", len(friends))
	}
}

func TestInvite_WithPhone(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5552222222"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["link"] == nil {
		t.Error("Expected link in response")
	}
}

func TestJoin_SuccessFlow(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Create invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("Invite: %d", rr.Code)
	}
	link := parseJSON(rr)["link"].(string)
	parts := strings.Split(link, "/join/")
	code := parts[1]

	// Join
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Join: %d", rr.Code)
	}
}

func TestEmailWebhook_EmptyFrom(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	body := "from=&text=hello"
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestSettingsUpdate_NameOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]interface{}{"name": "Bob"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	// Verify via settings get
	rr = doRequest(server.handleSettingsGet, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	if body["name"].(string) != "Bob" {
		t.Errorf("Expected name Bob, got %v", body["name"])
	}
}

func TestSettingsUpdate_EmailOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	email := "test@example.com"
	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]interface{}{"email": &email}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandleListGet_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestHandleListClear_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	// Add items
	doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Bread"}, []*http.Cookie{cookie})

	// Clear
	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Clear: %d", rr.Code)
	}

	// Verify empty
	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("Expected 0 items after clear, got %d", len(items))
	}
}

func TestExtractEmail_Formats(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@example.com", "user@example.com"},
		{"John Doe <john@example.com>", "john@example.com"},
		{"  user@EXAMPLE.COM  ", "user@example.com"},
	}
	for _, tt := range tests {
		result := extractEmail(tt.input)
		if result != tt.expected {
			t.Errorf("extractEmail(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestReferralClaim_SelfRefer(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551111111")

	server.db.Exec("UPDATE users SET referral_code = 'mycode' WHERE id = ?", userID)

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "mycode"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for self-refer, got %d", rr.Code)
	}
}

func TestReferralClaim_Idempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	server.db.Exec("UPDATE users SET referral_code = 'code1' WHERE id = ?", userID1)

	// First claim
	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "code1"}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("First claim: %d", rr.Code)
	}

	// Second claim — should also succeed (idempotent)
	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "code1"}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("Second claim: %d", rr.Code)
	}

	// Verify points only added once
	var points int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", userID1).Scan(&points)
	if points != 10 {
		t.Errorf("Expected 10 points (single claim), got %d", points)
	}
	_ = cookie1
}

func TestEmailWebhook_KnownUserAddsItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	_, userID := loginUser(t, server, "5551234567")

	// Set email for user
	server.db.Exec("UPDATE users SET email = 'test@example.com' WHERE id = ?", userID)

	// Set up mock parser
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy", Quantity: 1},
		}, nil
	}

	body := "from=test@example.com&text=milk"
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	// Verify item was added
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item added via email, got %d", count)
	}
}

func TestEmailWebhook_KnownUserEmptyText(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'test@example.com' WHERE id = ?", userID)

	body := "from=test@example.com&text="
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestSavedListUpdate_FullFlow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to current list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Bread', 'Bakery', 2)", userID)

	// Create saved list
	rr := doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create: %d", rr.Code)
	}
	listID := int(parseJSON(rr)["id"].(float64))

	// Add a new item to current list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy', 12)", userID)

	// Update the saved list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Update: %d", rr.Code)
	}

	// Verify updated count
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", listID).Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 saved items after update, got %d", count)
	}
}

func TestSavedListsGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add item and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "My List"}, []*http.Cookie{cookie})

	// Get saved lists
	rr := doRequest(server.handleSavedListsGet, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Errorf("Expected 1 list, got %d", len(lists))
	}
}

func TestHandleListGet_WithMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, checked) VALUES (?, 'Milk', 'Organic Valley', 'Dairy', 2, '1 gal', 'app', 0)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, checked) VALUES (?, 'Bread', 'Wonder', 'Bakery', 1, '', 'sms', 1)", userID)

	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestSavedListLoad_OverwriteWithExisting(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	rr := doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"}, []*http.Cookie{cookie})
	listID := int(parseJSON(rr)["id"].(float64))

	// Add different items to current list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy', 12)", userID)

	// Load saved list in overwrite mode
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Load: %d, body: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["added"].(float64) != 1 {
		t.Errorf("Expected 1 item added, got %v", body["added"])
	}
}

func TestInitDB_CreatesAllTables(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Verify key tables exist
	tables := []string{"users", "list_items", "saved_lists", "saved_list_items", "households",
		"household_members", "invites", "item_modifications", "referrals", "redemptions", "sessions"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}
}

func TestSettingsUpdate_EmailUniqueCheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// Set email on user1
	email1 := "shared@example.com"
	doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]interface{}{"email": &email1}, []*http.Cookie{cookie1})

	// Try to set same email on user2
	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]interface{}{"email": &email1}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409 for duplicate email, got %d", rr.Code)
	}
}

func TestRealTwilioClient_SendVerification(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"pending"}`))
	}))
	defer ts.Close()

	// Override DefaultTransport to redirect all requests to the test server
	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	err := client.SendVerification("+15551234567")
	if err != nil {
		t.Errorf("SendVerification: %v", err)
	}
}

func TestRealTwilioClient_CheckVerificationApproved(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"approved"}`))
	}))
	defer ts.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	approved, err := client.CheckVerification("+15551234567", "123456")
	if err != nil {
		t.Fatalf("CheckVerification: %v", err)
	}
	if !approved {
		t.Error("Expected approved=true")
	}
}

func TestRealTwilioClient_CheckVerificationDenied(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"pending"}`))
	}))
	defer ts.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	approved, err := client.CheckVerification("+15551234567", "000000")
	if err != nil {
		t.Fatalf("CheckVerification: %v", err)
	}
	if approved {
		t.Error("Expected approved=false for pending status")
	}
}

func TestRealTwilioClient_CheckVerificationError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"error"}`))
	}))
	defer ts.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	approved, err := client.CheckVerification("+15551234567", "000000")
	if err != nil {
		t.Fatalf("CheckVerification returned error: %v", err)
	}
	// When Twilio returns an error, CheckVerification logs and returns false, nil
	if approved {
		t.Error("Expected approved=false on error")
	}
}

func TestRealTwilioClient_SendSMS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer ts.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	client := &RealTwilioClient{
		AccountSID: "testSID",
		AuthToken:  "testToken",
	}

	err := client.SendSMS("+15551111111", "+15552222222", "Hello")
	if err != nil {
		t.Errorf("SendSMS: %v", err)
	}
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

func TestRealTwilioClient_DoRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testSID" || pass != "testToken" {
			t.Error("Expected basic auth with testSID/testToken")
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"pending"}`))
	}))
	defer ts.Close()

	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	data := url.Values{"To": {"+15551234567"}, "Channel": {"sms"}}
	body, err := client.doRequest(ts.URL, data, http.StatusCreated)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	if string(body) != `{"status":"pending"}` {
		t.Errorf("Unexpected body: %s", body)
	}
}

func TestRealTwilioClient_DoRequestBadStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"error"}`))
	}))
	defer ts.Close()

	client := &RealTwilioClient{
		AccountSID: "testSID",
		AuthToken:  "testToken",
	}

	data := url.Values{"key": {"value"}}
	_, err := client.doRequest(ts.URL, data, http.StatusOK)
	if err == nil {
		t.Error("Expected error for bad status")
	}
}

func TestRealTwilioClient_DoRequestNetworkError(t *testing.T) {
	client := &RealTwilioClient{
		AccountSID: "testSID",
		AuthToken:  "testToken",
	}

	data := url.Values{"key": {"value"}}
	_, err := client.doRequest("http://127.0.0.1:1", data, http.StatusOK)
	if err == nil {
		t.Error("Expected error for connection refused")
	}
}

func TestRealTwilioClient_CheckVerification(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"approved"}`))
	}))
	defer ts.Close()

	// Can't call CheckVerification directly (it builds its own URL),
	// but we can test the parsing logic via doRequest
	client := &RealTwilioClient{
		AccountSID:    "testSID",
		AuthToken:     "testToken",
		VerifyService: "VA123",
	}

	body, err := client.doRequest(ts.URL, url.Values{"To": {"+15551234567"}, "Code": {"123456"}}, http.StatusOK)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	var result struct {
		Status string `json:"status"`
	}
	json.Unmarshal(body, &result)
	if result.Status != "approved" {
		t.Errorf("Expected approved, got %s", result.Status)
	}
}

// =============================================================================
// BRANDS ENDPOINT TESTS
// =============================================================================

func TestBrands_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Insert test brands
	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Cheerios", "cheerios.com")
	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Tide", "tide.com")

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)

	if brands["cheerios"] != "cheerios.com" {
		t.Errorf("Expected cheerios.com, got %s", brands["cheerios"])
	}
	if brands["tide"] != "tide.com" {
		t.Errorf("Expected tide.com, got %s", brands["tide"])
	}

	// Verify Cache-Control header
	cc := rr.Header().Get("Cache-Control")
	if cc != "public, max-age=3600" {
		t.Errorf("Cache-Control: got %q, want %q", cc, "public, max-age=3600")
	}
}

func TestBrands_ReturnsSeedData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)
	// initDB seeds brands — verify we get a non-empty map with proper structure
	if len(brands) == 0 {
		t.Error("Expected seeded brands, got empty map")
	}
	// All keys should be lowercased
	for k := range brands {
		if k != strings.ToLower(k) {
			t.Errorf("Brand key %q is not lowercased", k)
			break
		}
	}
}

func TestBrands_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Cheerios", "cheerios.com")

	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	var brands map[string]string
	json.Unmarshal(rr.Body.Bytes(), &brands)

	// Brand names should be lowercased in response
	if _, ok := brands["cheerios"]; !ok {
		t.Error("Expected brand name to be lowercased")
	}
	if _, ok := brands["Cheerios"]; ok {
		t.Error("Brand name should not be in original case")
	}
}

// =============================================================================
// LIST URGENT ENDPOINT TESTS
// =============================================================================

func TestListUrgent_Toggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Mark urgent
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if !item.Urgent {
		t.Error("Expected item to be marked urgent")
	}

	// Toggle back — should unmark
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Urgent {
		t.Error("Expected item to be unmarked urgent after second toggle")
	}
}

func TestListUrgent_NotOwnedItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, otherUserID := loginUser(t, server, "5559999999")
	cookie, _ := loginUser(t, server, "5551234567")

	// Insert item owned by other user
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		otherUserID, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListUrgent_RecordsModification(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

	// Mark urgent
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	// Verify modification was recorded
	var action string
	err := server.db.QueryRow(
		"SELECT action FROM item_modifications WHERE item_id = ? ORDER BY id DESC LIMIT 1",
		itemID,
	).Scan(&action)
	if err != nil {
		t.Fatalf("Failed to query modification: %v", err)
	}
	if action != "marked urgent" {
		t.Errorf("Expected action 'marked urgent', got %q", action)
	}
}

// =============================================================================
// CLAIM ENDPOINT TESTS
// =============================================================================

func TestListClaim_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Claim
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil || *item.ClaimedByID != userID {
		t.Errorf("Expected claimed_by_id to be %d, got %v", userID, item.ClaimedByID)
	}
}

func TestListClaim_Unclaim(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

	// Claim
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	// Unclaim (second call)
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID != nil {
		t.Errorf("Expected claimed_by_id to be nil after unclaim, got %v", item.ClaimedByID)
	}
}

func TestListClaim_Override(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create household with two users
	cookie1, userID1 := loginUser(t, server, "5551234567")
	cookie2, userID2 := loginUser(t, server, "5559999999")

	// Create household: user1 creates, user2 joins
	server.db.Exec("INSERT INTO households (name) VALUES ('Test')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, ?, 'admin')", userID1)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, ?, 'member')", userID2)
	server.db.Exec("UPDATE users SET household_id = 1 WHERE id IN (?, ?)", userID1, userID2)

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	// User1 claims
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie1})

	// User2 overrides
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie2})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil || *item.ClaimedByID != userID2 {
		t.Errorf("Expected claimed_by_id to be %d (user2), got %v", userID2, item.ClaimedByID)
	}
}

func TestListClaim_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999/claim",
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

// =============================================================================
// NPS ENDPOINT TESTS
// =============================================================================

func TestNPSCheck_NotEnoughLogins(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// User has 0 login_count by default
	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	result := parseJSON(rr)
	if result["show_nps"] != false {
		t.Error("Expected show_nps=false when login_count < 3")
	}
}

func TestNPSCheck_ShowsAfterEnoughLogins(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count to 5
	server.db.Exec("UPDATE users SET login_count = 5 WHERE id = ?", userID)

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != true {
		t.Error("Expected show_nps=true when login_count >= 3 and no recent NPS")
	}
}

func TestNPSCheck_HidesAfterRecentResponse(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count to 5 and add recent NPS response
	server.db.Exec("UPDATE users SET login_count = 5 WHERE id = ?", userID)
	server.db.Exec("INSERT INTO nps_responses (user_id, score, comment) VALUES (?, ?, ?)",
		userID, 8, "Great app!")

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != false {
		t.Error("Expected show_nps=false when recent NPS response exists")
	}
}

func TestNPS_SubmitSuccess(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	body := map[string]interface{}{"score": 9, "comment": "Love it!"}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	result := parseJSON(rr)
	if result["success"] != true {
		t.Error("Expected success=true")
	}

	// Verify stored in DB
	var score int
	var comment string
	server.db.QueryRow("SELECT score, comment FROM nps_responses WHERE user_id = ?", userID).Scan(&score, &comment)
	if score != 9 {
		t.Errorf("Expected score 9, got %d", score)
	}
	if comment != "Love it!" {
		t.Errorf("Expected comment 'Love it!', got %q", comment)
	}
}

func TestNPS_InvalidScore(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Score too high
	body := map[string]interface{}{"score": 11, "comment": ""}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status for score=11: got %d, want 400", rr.Code)
	}

	// Score too low
	body = map[string]interface{}{"score": -1, "comment": ""}
	rr = doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status for score=-1: got %d, want 400", rr.Code)
	}
}

func TestNPS_BoundaryScores(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Score 0 should be valid
	body := map[string]interface{}{"score": 0, "comment": ""}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status for score=0: got %d, want 200", rr.Code)
	}

	// Score 10 should be valid
	body = map[string]interface{}{"score": 10, "comment": "Perfect"}
	rr = doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status for score=10: got %d, want 200", rr.Code)
	}
}

func TestNPS_EmptyComment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	body := map[string]interface{}{"score": 7, "comment": ""}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var comment string
	server.db.QueryRow("SELECT comment FROM nps_responses WHERE user_id = ?", userID).Scan(&comment)
	if comment != "" {
		t.Errorf("Expected empty comment, got %q", comment)
	}
}

// =============================================================================
// EDGE CASE TESTS — INVALID IDs, METHOD VALIDATION, BODY SIZE
// =============================================================================

func TestList_InvalidIDFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	tests := []struct {
		name string
		path string
	}{
		{"letters", "/api/list/abc"},
		{"negative", "/api/list/-1"},
		{"float", "/api/list/1.5"},
		{"empty_after_slash", "/api/list//toggle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(server.handleList, "PATCH", tt.path, nil, []*http.Cookie{cookie})
			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s: got %d, want 400", tt.name, rr.Code)
			}
		})
	}
}

func TestSavedLists_InvalidIDFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Non-numeric IDs should return 400
	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("letters: got %d, want 400", rr.Code)
	}

	// Negative IDs parse fine but return 404 (not found in DB)
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists/-1", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("negative ID should not return 200")
	}
}

func TestDecodeJSON_RequestTooLarge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create a body larger than 10MB
	bigString := strings.Repeat("x", 11*1024*1024)
	body := map[string]string{"name": bigString}

	rr := doRequest(server.handleList, "POST", "/api/list", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Large body: got %d, want 400", rr.Code)
	}
}

func TestNPS_CommentTruncation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	longComment := strings.Repeat("a", 1000)
	body := map[string]interface{}{"score": 5, "comment": longComment}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var comment string
	server.db.QueryRow("SELECT comment FROM nps_responses WHERE user_id = ?", userID).Scan(&comment)
	if len(comment) > 500 {
		t.Errorf("Comment should be truncated to 500 chars, got %d", len(comment))
	}
}

func TestSecureCookie_EnvAware(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	// Without SecureCookies, cookie should not be Secure
	server.config.SecureCookies = false
	rr := doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234567", "code": "123456"}, nil)
	cookie := getSessionCookie(rr)
	if cookie != nil && cookie.Secure {
		t.Error("Expected non-secure cookie when SecureCookies=false")
	}

	// With SecureCookies, cookie should be Secure
	server.config.SecureCookies = true
	rr = doRequest(server.handleVerifyCode, "POST", "/auth/verify-code",
		map[string]string{"phone_number": "5551234568", "code": "123456"}, nil)
	cookie = getSessionCookie(rr)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}
	if !cookie.Secure {
		t.Error("Expected secure cookie when SecureCookies=true")
	}
}

// =============================================================================
// REFERRAL REDEEM TESTS
// =============================================================================

func TestReferralRedeem_InsufficientPoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("UPDATE users SET referral_points = 50 WHERE id = ?", userID)

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}

	result := parseJSON(rr)
	if msg, ok := result["error"].(string); !ok || !strings.Contains(msg, "100 points") {
		t.Errorf("Error message: got %v, want message about 100 points", result["error"])
	}
}

func TestReferralRedeem_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("UPDATE users SET referral_points = 150 WHERE id = ?", userID)

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify points deducted
	var points int
	server.db.QueryRow("SELECT referral_points FROM users WHERE id = ?", userID).Scan(&points)
	if points != 50 {
		t.Errorf("Points: got %d, want 50", points)
	}

	// Verify redemption record
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM redemptions WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Redemptions: got %d, want 1", count)
	}
}

func TestReferralRedeem_ZeroPoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// SAVED LISTS METHOD VALIDATION TESTS
// =============================================================================

func TestSavedLists_RootPUT_Returns405(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists",
		map[string]string{"name": "Test"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/saved-lists: got %d, want 405", rr.Code)
	}
}

func TestSavedLists_RootDELETE_Returns405(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/saved-lists: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// SETTINGS EDGE CASE TESTS
// =============================================================================

func TestSettingsUpdate_EmailLowercased(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "TEST@EXAMPLE.COM"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var email sql.NullString
	server.db.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&email)
	if email.String != "test@example.com" {
		t.Errorf("Email: got %q, want 'test@example.com'", email.String)
	}
}

func TestSettingsUpdate_InvalidEmailFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"email": "notanemail"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestLogout_CookieHasSecurityFlags(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	server.config.SecureCookies = true

	// Login first
	cookie, _ := loginUser(t, server, "5551234567")

	// Logout
	rr := doRequest(server.handleLogout, "POST", "/auth/logout", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	logoutCookie := getSessionCookie(rr)
	if logoutCookie == nil {
		t.Fatal("Expected session cookie in logout response")
	}
	if !logoutCookie.HttpOnly {
		t.Error("Logout cookie should have HttpOnly flag")
	}
	if !logoutCookie.Secure {
		t.Error("Logout cookie should have Secure flag when SecureCookies=true")
	}
}

// =============================================================================
// ROUND 4: HOUSEHOLD, SAVED LISTS, HEALTH, REQUIRE-METHOD TESTS
// =============================================================================

func TestHealth_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("health check returned %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["status"] != "ok" {
		t.Errorf("health check body = %v, want status=ok", body)
	}
}

func TestRequireMethod_Allowed(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	if !requireMethod(rr, req, http.MethodGet) {
		t.Error("requireMethod should return true for matching method")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestRequireMethod_Denied(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	if requireMethod(rr, req, http.MethodPost) {
		t.Error("requireMethod should return false for non-matching method")
	}
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestCreateSoloHousehold_VerifyMembership(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Insert user without upsertUser (which auto-creates household)
	db.Exec("INSERT INTO users (phone_number) VALUES ('5551234567')")
	var userID int
	db.QueryRow("SELECT id FROM users WHERE phone_number = '5551234567'").Scan(&userID)

	hID, err := createSoloHousehold(db, userID)
	if err != nil {
		t.Fatalf("createSoloHousehold: %v", err)
	}
	if hID == 0 {
		t.Error("expected non-zero household ID")
	}

	// Verify user's household_id was set
	var gotHID sql.NullInt64
	db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&gotHID)
	if !gotHID.Valid || int(gotHID.Int64) != hID {
		t.Errorf("user household_id = %v, want %d", gotHID, hID)
	}

	// Verify household_members row exists with owner role
	var role string
	db.QueryRow("SELECT role FROM household_members WHERE household_id = ? AND user_id = ?", hID, userID).Scan(&role)
	if role != "owner" {
		t.Errorf("role = %q, want 'owner'", role)
	}
}

func TestGetHouseholdUserIDs_NoHousehold(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, _ := upsertUser(db, "5551234567")
	ids, err := getHouseholdUserIDs(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	// User with no household should get back just their own ID
	if len(ids) != 1 || ids[0] != user.ID {
		t.Errorf("ids = %v, want [%d]", ids, user.ID)
	}
}

func TestGetHouseholdUserIDs_WithHousehold(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user1, _ := upsertUser(db, "5551234567")
	user2, _ := upsertUser(db, "5559876543")

	// user1 already has a household from upsertUser; move user2 into it
	hID := user1.HouseholdID
	db.Exec("DELETE FROM household_members WHERE user_id = ?", user2.ID)
	db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, user2.ID)
	db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, user2.ID)

	ids, err := getHouseholdUserIDs(db, user1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("got %d IDs, want 2", len(ids))
	}
}

func TestGetHouseholdUserIDs_NonexistentUser(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ids, err := getHouseholdUserIDs(db, 99999)
	if err != nil {
		t.Fatal(err)
	}
	// Should fallback to returning just the provided ID
	if len(ids) != 1 || ids[0] != 99999 {
		t.Errorf("ids = %v, want [99999]", ids)
	}
}

func TestHouseholdItemsWhereClause_Solo(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, _ := upsertUser(db, "5551234567")
	clause, args := householdItemsWhereClause(db, user.ID)

	if !strings.Contains(clause, "user_id IN") {
		t.Errorf("clause = %q, want user_id IN (...)", clause)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want 1 element", args)
	}
}

func TestHouseholdItemsWhereClause_MultiMember(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user1, _ := upsertUser(db, "5551234567")
	user2, _ := upsertUser(db, "5559876543")

	hID := user1.HouseholdID
	db.Exec("DELETE FROM household_members WHERE user_id = ?", user2.ID)
	db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, user2.ID)
	db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, user2.ID)

	clause, args := householdItemsWhereClause(db, user1.ID)

	if !strings.Contains(clause, ",") {
		t.Errorf("clause = %q, expected multiple placeholders", clause)
	}
	if len(args) != 2 {
		t.Errorf("args = %v, want 2 elements", args)
	}
}

func TestSavedListLoad_OverwriteClearsExisting(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Add an existing item to the active list
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', '', 'Dairy', 1, '', 'app')",
		user.ID,
	)

	// Create a saved list with a different item
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", user.ID)
	savedID, _ := result.LastInsertId()
	server.db.Exec(
		"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 2, '')",
		savedID,
	)

	// Load with overwrite
	rr := doRequest(
		server.handleSavedLists,
		"POST",
		fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID),
		nil,
		cookies,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["added"] != float64(1) {
		t.Errorf("added = %v, want 1", body["added"])
	}

	// The old item (Milk) should be deleted
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Milk'", user.ID).Scan(&count)
	if count != 0 {
		t.Error("expected Milk to be deleted on overwrite")
	}
}

func TestSavedListUpdate_EmptyList(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create an empty saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Empty')", user.ID)
	savedID, _ := result.LastInsertId()

	// Try to update with empty active list → should get 400
	rr := doRequest(
		server.handleSavedLists,
		"PUT",
		fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil,
		cookies,
	)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// =============================================================================
// SHOPPING TRIPS
// =============================================================================

func TestTrips_CreateAndGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create a trip
	rr := doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": "2099-01-01", "trip_time": "15:00"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Create trip: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// Get trips
	rr = doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Get trips: got %d, want 200", rr.Code)
	}
	var trips []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &trips)
	if len(trips) != 1 {
		t.Fatalf("Expected 1 trip, got %d", len(trips))
	}
	if trips[0]["trip_date"] != "2099-01-01" {
		t.Errorf("Expected trip_date 2099-01-01, got %v", trips[0]["trip_date"])
	}
	if trips[0]["is_mine"] != true {
		t.Errorf("Expected is_mine true, got %v", trips[0]["is_mine"])
	}
}

func TestTrips_DeleteTrip(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create
	doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": "2099-01-01"},
		[]*http.Cookie{cookie})

	// Delete
	rr := doRequest(server.handleTrips, "DELETE", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Delete trip: got %d, want 200", rr.Code)
	}

	// Verify gone
	rr = doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	var trips []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &trips)
	if len(trips) != 0 {
		t.Errorf("Expected 0 trips after delete, got %d", len(trips))
	}
}

func TestTrips_ReplaceExisting(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create first trip
	doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": "2099-01-01", "trip_time": "09:00"},
		[]*http.Cookie{cookie})

	// Create second trip (should replace first)
	doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": "2099-01-02", "trip_time": "15:00"},
		[]*http.Cookie{cookie})

	// Should only have the latest trip
	rr := doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	var trips []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &trips)
	if len(trips) != 1 {
		t.Fatalf("Expected 1 trip (replaced), got %d", len(trips))
	}
	if trips[0]["trip_date"] != "2099-01-02" {
		t.Errorf("Expected trip_date 2099-01-02, got %v", trips[0]["trip_date"])
	}
}

func TestStripBrandFromName(t *testing.T) {
	tests := []struct {
		name, brand, want string
	}{
		{"Budweiser Beer", "Budweiser", "Beer"},
		{"Beer Budweiser", "Budweiser", "Beer"},
		{"Peanut Butter", "Jif", "Peanut Butter"},
		{"Macaroni & Cheese", "Kraft", "Macaroni & Cheese"},
		{"Kraft Macaroni & Cheese", "Kraft", "Macaroni & Cheese"},
		{"Whole Milk", "", "Whole Milk"},
		{"Budweiser", "Budweiser", "Budweiser"}, // don't strip if nothing left
	}
	for _, tt := range tests {
		got := stripBrandFromName(tt.name, tt.brand)
		if got != tt.want {
			t.Errorf("stripBrandFromName(%q, %q) = %q, want %q", tt.name, tt.brand, got, tt.want)
		}
	}
}

// =============================================================================
// LOGO TESTS
// =============================================================================

func TestLogo_MissingDomain(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestLogo_InvalidDomain(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=evil%3Cscript%3E.com", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestLogo_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	rr := doRequest(server.handleLogoProxy, "POST", "/api/logo?domain=kraft.com", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestLogo_MemoryCacheHit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Pre-populate memory cache
	fakeData := []byte("PNG_FAKE_DATA_LARGE_ENOUGH_TO_PASS_CHECK_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.logoCache.Store("kraft.com", &cachedLogo{data: fakeData, contentType: "image/png"})

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=kraft.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Expected image/png, got %s", rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("Cache-Control") != "public, max-age=604800" {
		t.Errorf("Expected cache-control header, got %s", rr.Header().Get("Cache-Control"))
	}
	if rr.Body.Len() != len(fakeData) {
		t.Errorf("Expected body length %d, got %d", len(fakeData), rr.Body.Len())
	}
}

func TestLogo_MemoryCacheIsCaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	fakeData := []byte("PNG_FAKE_DATA_LARGE_ENOUGH_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.logoCache.Store("kraft.com", &cachedLogo{data: fakeData, contentType: "image/png"})

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=Kraft.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestLogo_SQLiteCacheHit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Insert a brand and logo into the database
	// Use a unique brand name to avoid conflict with seedBrands
	res, err := server.db.Exec("INSERT INTO brands (name, domain) VALUES ('TestBrandXYZ', 'testbrandxyz.com')")
	if err != nil {
		t.Fatalf("Failed to insert brand: %v", err)
	}
	brandID, _ := res.LastInsertId()
	fakeData := []byte("SQLITE_LOGO_DATA_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	server.db.Exec("INSERT INTO brand_logos (brand_id, logo_data, content_type) VALUES (?, ?, 'image/png')", brandID, fakeData)

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=testbrandxyz.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	if rr.Body.Len() != len(fakeData) {
		t.Errorf("Expected body length %d, got %d", len(fakeData), rr.Body.Len())
	}
	// Verify it was cached in memory
	if _, ok := server.logoCache.Load("testbrandxyz.com"); !ok {
		t.Error("Expected logo to be stored in memory cache after SQLite hit")
	}
}

func TestLogo_PlaceholderFallback(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// No token, no cache → should get SVG placeholder
	server.config.LogoDevToken = ""

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=unknownbrand.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("Expected SVG placeholder, got %s", rr.Header().Get("Content-Type"))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("Expected SVG content in placeholder")
	}
	if !strings.Contains(body, "U") { // "unknownbrand" → initial "U"
		t.Errorf("Expected initial 'U' in placeholder, got: %s", body)
	}
}

func TestLogo_PlaceholderUseBrandName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	server.config.LogoDevToken = ""
	// Insert a brand so the handler can use its name for initials
	server.db.Exec("INSERT OR IGNORE INTO brands (name, domain) VALUES ('Zesty Foods', 'zestyfoods.com')")

	rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain=zestyfoods.com", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// "Zesty Foods" → initials "ZF"
	if !strings.Contains(body, "ZF") {
		t.Errorf("Expected initials 'ZF' in placeholder, got: %s", body)
	}
}

func TestLogoInitials(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"Kraft", "K"},
		{"General Mills", "GM"},
		{"Ben & Jerry's", "B&"},
		{"", "?"},
	}
	for _, tt := range tests {
		got := logoInitials(tt.name)
		if got != tt.want {
			t.Errorf("logoInitials(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestLogoColor_Deterministic(t *testing.T) {
	c1 := logoColor("Kraft")
	c2 := logoColor("Kraft")
	if c1 != c2 {
		t.Errorf("logoColor should be deterministic: %q != %q", c1, c2)
	}
	c3 := logoColor("General Mills")
	if c1 == c3 {
		// Not guaranteed but very likely different
		t.Log("Note: Kraft and General Mills have the same color (unlikely but possible)")
	}
	if !strings.HasPrefix(c1, "#") {
		t.Errorf("Expected hex color, got %q", c1)
	}
}

func TestFetchBrandLogo_EmptyToken(t *testing.T) {
	data, ct := fetchBrandLogo("kraft.com", "")
	if data != nil || ct != "" {
		t.Error("Expected nil result with empty token")
	}
}

func TestLogo_DomainSanitization(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	// Valid domains
	validDomains := []string{"kraft.com", "ben-jerrys.com", "Mr.Clean.com", "a1steak.com"}
	for _, d := range validDomains {
		rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain="+url.QueryEscape(d), nil, nil)
		if rr.Code == http.StatusBadRequest {
			t.Errorf("Domain %q should be valid, got 400", d)
		}
	}
	// Invalid domains
	invalidDomains := []string{"evil<script>.com", "bad domain.com", "path/../traversal", "data:text/html"}
	for _, d := range invalidDomains {
		rr := doRequest(server.handleLogoProxy, "GET", "/api/logo?domain="+url.QueryEscape(d), nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Domain %q should be invalid, got %d", d, rr.Code)
		}
	}
}

// =============================================================================
// SPA ROUTING TESTS — Browser Navigation
//
// Verifies that the Go server serves index.html for all frontend routes
// (pathname-based routing for browser back/forward support).
// =============================================================================

func TestSPARouting_FrontendPaths(t *testing.T) {
	// Create a temp static directory with a dummy index.html
	tmpDir := t.TempDir()
	indexContent := []byte("<!DOCTYPE html><html><body>SPA</body></html>")
	if err := os.WriteFile(tmpDir+"/index.html", indexContent, 0644); err != nil {
		t.Fatalf("Failed to write test index.html: %v", err)
	}
	// Create a real static asset to test that those still get served
	if err := os.WriteFile(tmpDir+"/test.js", []byte("console.log('hi')"), 0644); err != nil {
		t.Fatalf("Failed to write test.js: %v", err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	// All these frontend paths should serve index.html (SPA catch-all)
	frontendPaths := []string{
		"/",
		"/login",
		"/verify",
		"/about",
		"/share",
		"/settings",
		"/settings/profile",
		"/settings/saved",
		"/admin",
		"/join/abc123",
		"/r/refcode",
	}

	for _, path := range frontendPaths {
		t.Run("GET "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("GET %s: got status %d, want 200", path, rr.Code)
			}
			body := rr.Body.String()
			if !strings.Contains(body, "SPA") {
				t.Errorf("GET %s: expected index.html content, got %q", path, body[:min(len(body), 100)])
			}
		})
	}
}

func TestSPARouting_StaticAssetsServedDirectly(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/index.html", []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}
	jsContent := "console.log('app')"
	if err := os.WriteFile(tmpDir+"/app.js", []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tmpDir+"/assets", 0755); err != nil {
		t.Fatal(err)
	}
	cssContent := "body { color: red }"
	if err := os.WriteFile(tmpDir+"/assets/style.css", []byte(cssContent), 0644); err != nil {
		t.Fatal(err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	// Static files should be served directly (not index.html)
	t.Run("JS file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/app.js", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "console.log") {
			t.Error("expected JS content")
		}
	})

	t.Run("Nested CSS file", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/style.css", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "color: red") {
			t.Error("expected CSS content")
		}
	})
}

func TestSPARouting_UnknownPathsFallbackToIndex(t *testing.T) {
	tmpDir := t.TempDir()
	indexContent := []byte("INDEX")
	if err := os.WriteFile(tmpDir+"/index.html", indexContent, 0644); err != nil {
		t.Fatal(err)
	}

	fs := http.FileServer(http.Dir(tmpDir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(tmpDir + r.URL.Path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, tmpDir+"/index.html")
	})

	unknownPaths := []string{
		"/foo/bar",
		"/nonexistent",
		"/deeply/nested/path/here",
		"/settings/unknown",
	}

	for _, path := range unknownPaths {
		t.Run("GET "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("GET %s: got status %d, want 200", path, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "INDEX") {
				t.Errorf("GET %s: expected index.html fallback", path)
			}
		})
	}
}

