package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)


func TestSessionStore_CreateAndGet(t *testing.T) {
	store := newTestSessionStore(t)
	token, err := store.CreateSession(1, "5551234567", "phone")

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
	token, _ := store.CreateSession(1, "5551234567", "phone")
	store.DeleteSession(token)

	_, ok := store.GetSession(token)
	if ok {
		t.Error("Session still exists after deletion")
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	store := newTestSessionStore(t)
	token1, _ := store.CreateSession(1, "5551111111", "phone")
	token2, _ := store.CreateSession(2, "5552222222", "phone")

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
	token1, _ := store.CreateSession(1, "5551111111", "phone")
	token2, _ := store.CreateSession(2, "5552222222", "phone")

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
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")

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
	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")

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

// =============================================================================
// REQUIRE ADMIN HELPER
// =============================================================================

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")

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
	token, _ := server.sessions.CreateSession(user.ID, "3104861833", "phone")

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
// RESOLVE OR CREATE USER TESTS
// =============================================================================

func TestResolveOrCreateUser_NewProvider(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	user, err := resolveOrCreateUser(db, "google", "google-sub-123", "alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	if user.ID == 0 {
		t.Error("Expected non-zero user ID")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "alice@example.com")
	}
}

func TestResolveOrCreateUser_SameProviderReturnsSameUser(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	u1, err := resolveOrCreateUser(db, "google", "google-sub-456", "bob@example.com", "Bob")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	u2, err := resolveOrCreateUser(db, "google", "google-sub-456", "bob@example.com", "Bob")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("Same provider+sub created two users: %d vs %d", u1.ID, u2.ID)
	}
}

func TestResolveOrCreateUser_EmailMergeAcrossProviders(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Register via email first
	u1, err := resolveOrCreateUser(db, "email", "carol@example.com", "carol@example.com", "Carol")
	if err != nil {
		t.Fatalf("email user: %v", err)
	}
	// Set the user's email directly so the merge lookup can find it
	db.Exec("UPDATE users SET email = 'carol@example.com' WHERE id = ?", u1.ID)

	// Now sign in via Google with same email — should merge to same user
	u2, err := resolveOrCreateUser(db, "google", "google-carol-sub", "carol@example.com", "Carol")
	if err != nil {
		t.Fatalf("google user: %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("Same email across providers created two users: %d vs %d", u1.ID, u2.ID)
	}
}

func TestResolveOrCreateUser_LoginCountIncremented(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	user, _ := resolveOrCreateUser(db, "email", "dave@example.com", "dave@example.com", "Dave")

	// Call again to trigger the login_count increment path
	resolveOrCreateUser(db, "email", "dave@example.com", "dave@example.com", "Dave")

	var loginCount int
	db.QueryRow("SELECT login_count FROM users WHERE id = ?", user.ID).Scan(&loginCount)
	if loginCount < 1 {
		t.Errorf("login_count: got %d, want >= 1", loginCount)
	}
}

func TestResolveOrCreateUser_MergedFlag(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	// Create user via email
	u1, err := resolveOrCreateUser(db, "email", "merge@example.com", "merge@example.com", "MergeTest")
	if err != nil {
		t.Fatalf("email user: %v", err)
	}
	if u1.Merged {
		t.Error("New user should not be marked as merged")
	}
	db.Exec("UPDATE users SET email = 'merge@example.com' WHERE id = ?", u1.ID)

	// Sign in via Google with same email — should merge and set Merged flag
	u2, err := resolveOrCreateUser(db, "google", "google-merge-sub", "merge@example.com", "MergeTest")
	if err != nil {
		t.Fatalf("google merge: %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("Expected same user, got %d vs %d", u1.ID, u2.ID)
	}
	if !u2.Merged {
		t.Error("Expected Merged=true on first merge login")
	}

	// Login again with Google — should NOT be marked as merged (auth method already exists)
	u3, err := resolveOrCreateUser(db, "google", "google-merge-sub", "merge@example.com", "MergeTest")
	if err != nil {
		t.Fatalf("second google login: %v", err)
	}
	if u3.Merged {
		t.Error("Expected Merged=false on subsequent logins (auth method already exists)")
	}
}

// =============================================================================
// EMAIL REGISTER HANDLER TESTS
// =============================================================================

func TestEmailRegister_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "eve@example.com", "password": "securepassword1", "name": "Eve"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}
	if uid, ok := body["user_id"].(float64); !ok || uid < 1 {
		t.Errorf("user_id: got %v, want positive integer", body["user_id"])
	}
	// Session cookie must be set
	if getSessionCookie(rr) == nil {
		t.Error("No session cookie set after register")
	}
}

func TestEmailRegister_InvalidEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "not-an-email", "password": "securepassword1"}, nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestEmailRegister_PasswordTooShort(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "frank@example.com", "password": "short"}, nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestEmailRegister_DuplicateEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register once
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "grace@example.com", "password": "securepassword1"}, nil)

	// Try to register again with the same email
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "grace@example.com", "password": "differentpass99"}, nil)

	if rr.Code != http.StatusConflict {
		t.Errorf("Status: got %d, want 409 Conflict", rr.Code)
	}
}

func TestEmailRegister_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailRegister, "GET", "/auth/email/register", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// EMAIL LOGIN HANDLER TESTS
// =============================================================================

func TestEmailLogin_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register first
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "henry@example.com", "password": "correcthorsebattery"}, nil)

	// Then log in
	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "henry@example.com", "password": "correcthorsebattery"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}
	if getSessionCookie(rr) == nil {
		t.Error("No session cookie set after login")
	}
}

func TestEmailLogin_WrongPassword(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "iris@example.com", "password": "correctpassword1"}, nil)

	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "iris@example.com", "password": "wrongpassword99"}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestEmailLogin_UnknownEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "nobody@example.com", "password": "somepassword1"}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestEmailLogin_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailLogin, "GET", "/auth/email/login", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestEmailLogin_EmailCaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register with lowercase email
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "julia@example.com", "password": "testpassword1"}, nil)

	// Log in with uppercase email — should still work
	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "JULIA@EXAMPLE.COM", "password": "testpassword1"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200 (email login should be case-insensitive)", rr.Code)
	}
}

// =============================================================================
// EMAIL FORGOT / RESET HANDLER TESTS
// =============================================================================

func TestEmailForgot_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailForgot, "GET", "/auth/email/forgot", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestEmailForgot_UnknownEmail_ReturnsOK(t *testing.T) {
	// For security, the forgot-password endpoint should always return 200
	// regardless of whether the email exists (prevents user enumeration).
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "nosuchuser@example.com"}, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200 (should not reveal whether email exists)", rr.Code)
	}
}

func TestEmailForgot_RateLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// 5 requests should succeed
	for i := 0; i < 5; i++ {
		rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
			map[string]string{"email": "test@example.com"}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("Request %d: got %d, want 200", i+1, rr.Code)
		}
	}

	// 6th request should be rate limited
	rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "test@example.com"}, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: got %d, want 429", rr.Code)
	}
}

func TestEmailForgot_EmptyEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": ""}, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for empty email, got %d", rr.Code)
	}
}

func TestEmailForgot_ValidUser_CreatesToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register a user
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "reset@example.com", "password": "password123", "name": "ResetUser"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %d", rr.Code)
	}

	// Request forgot password
	rr = doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "reset@example.com"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify a reset token was created in the database
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM password_resets WHERE used = 0").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 password_reset token, got %d", count)
	}
}

func TestEmailForgot_FullResetFlow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "flow@example.com", "password": "oldpassword1", "name": "Flow"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register: %d", rr.Code)
	}

	// Request reset
	rr = doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "flow@example.com"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("forgot: %d", rr.Code)
	}

	// Get the token from DB
	var token string
	server.db.QueryRow("SELECT token FROM password_resets WHERE used = 0 ORDER BY id DESC LIMIT 1").Scan(&token)
	if token == "" {
		t.Fatal("no reset token found in DB")
	}

	// Reset with the token
	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": token, "password": "newpassword2"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("reset: %d", rr.Code)
	}

	// Login with new password
	rr = doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "flow@example.com", "password": "newpassword2"}, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("expected login with new password to succeed, got %d", rr.Code)
	}
}

func TestMagicLink_RateLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// 5 requests should succeed
	for i := 0; i < 5; i++ {
		rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
			map[string]string{"email": "test@example.com"}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("Request %d: got %d, want 200", i+1, rr.Code)
		}
	}

	// 6th request should be rate limited
	rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": "test@example.com"}, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: got %d, want 429", rr.Code)
	}
}

func TestEmailReset_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailReset, "GET", "/auth/email/reset", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestEmailReset_InvalidToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "bogus-token", "password": "newpassword1"}, nil)

	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 400 or 401 for invalid reset token", rr.Code)
	}
}

func TestEmailReset_MissingToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "", "password": "newpassword1"}, nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty token, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if !strings.Contains(fmt.Sprint(body["error"]), "Missing reset token") {
		t.Errorf("expected 'Missing reset token' error, got %v", body["error"])
	}
}

func TestEmailReset_ShortPassword(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "some-token", "password": "short"}, nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if !strings.Contains(fmt.Sprint(body["error"]), "8-72 characters") {
		t.Errorf("expected password length error, got %v", body["error"])
	}
}

func TestEmailReset_ExpiredToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a user with email auth
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "test@example.com", "password": "password123", "name": "Test"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %d", rr.Code)
	}

	// Insert an expired reset token
	server.db.Exec(`INSERT INTO password_resets (user_id, token, expires_at, used)
		SELECT id, 'expired-token-123', datetime('now', '-1 hour'), 0 FROM users WHERE email = 'test@example.com'`)

	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "expired-token-123", "password": "newpassword1"}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for expired token, got %d", rr.Code)
	}
}

func TestEmailReset_AlreadyUsedToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "test@example.com", "password": "password123", "name": "Test"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %d", rr.Code)
	}

	// Insert a used reset token
	server.db.Exec(`INSERT INTO password_resets (user_id, token, expires_at, used)
		SELECT id, 'used-token-123', datetime('now', '+1 hour'), 1 FROM users WHERE email = 'test@example.com'`)

	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "used-token-123", "password": "newpassword1"}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for used token, got %d", rr.Code)
	}
}

func TestEmailReset_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register user
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "test@example.com", "password": "password123", "name": "Test"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %d", rr.Code)
	}

	// Insert a valid reset token
	server.db.Exec(`INSERT INTO password_resets (user_id, token, expires_at, used)
		SELECT id, 'valid-token-123', datetime('now', '+1 hour'), 0 FROM users WHERE email = 'test@example.com'`)

	// Reset password
	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "valid-token-123", "password": "newpassword456"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid reset, got %d", rr.Code)
	}

	// Verify can login with new password
	rr = doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "test@example.com", "password": "newpassword456"}, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("expected login with new password to succeed, got %d", rr.Code)
	}

	// Verify old password no longer works
	rr = doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "test@example.com", "password": "password123"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected old password to fail, got %d", rr.Code)
	}

	// Verify token is marked as used
	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "valid-token-123", "password": "anotherpassword"}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for reused token, got %d", rr.Code)
	}
}

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================
// VALIDATE TWILIO SIGNATURE TESTS
// =============================================================================

func TestValidateTwilioSignature_MissingHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader("Body=hello"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No X-Twilio-Signature header
	if validateTwilioSignature(req, "authtoken", "https://example.com") {
		t.Error("Expected false when X-Twilio-Signature is missing")
	}
}

func TestValidateTwilioSignature_WrongSignature(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/sms", strings.NewReader("Body=hello"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", "not-the-right-signature")
	if validateTwilioSignature(req, "authtoken", "https://example.com") {
		t.Error("Expected false for wrong signature")
	}
}

// =============================================================================
// REQUIRE SAVED LIST ACCESS TESTS
// =============================================================================
// ONBOARDING TESTS
// =============================================================================

func TestOnboarding_MeReturnsLoginCount(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// User created via upsertUser gets login_count=1 (first login)
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	lc, ok := body["login_count"].(float64)
	if !ok {
		t.Fatal("login_count not in response")
	}
	if int(lc) != 1 {
		t.Errorf("expected login_count=1 for new user, got %d", int(lc))
	}

	// Name should not be present when empty
	if _, hasName := body["name"]; hasName {
		t.Error("expected no 'name' key for user without name")
	}
}

func TestOnboarding_MeReturnsLoginCountAfterLogins(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Simulate a phone login which increments login_count
	server.db.Exec("INSERT INTO users (id, phone_number, login_count) VALUES (1, '5551234567', 0)")
	server.db.Exec("INSERT INTO auth_methods (user_id, provider, provider_id) VALUES (1, 'phone', '5551234567')")

	// Manually increment login_count as would happen during auth
	server.db.Exec("UPDATE users SET login_count = 3 WHERE id = 1")

	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	body := parseJSON(rr)
	lc := body["login_count"].(float64)
	if int(lc) != 3 {
		t.Errorf("expected login_count=3, got %d", int(lc))
	}
}

func TestOnboarding_MeReturnsName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO users (id, phone_number, name) VALUES (1, '5551234567', 'Andy')")
	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	body := parseJSON(rr)

	name, ok := body["name"].(string)
	if !ok || name != "Andy" {
		t.Errorf("expected name='Andy', got %v", body["name"])
	}
}

func TestOnboarding_OnboardingFlagsColumnExists(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Verify onboarding_flags column exists with default value
	var flags string
	err := server.db.QueryRow("SELECT onboarding_flags FROM users WHERE id = 1").Scan(&flags)
	if err == nil && flags != "{}" {
		t.Errorf("expected default onboarding_flags='{}', got '%s'", flags)
	}

	// Insert a user and verify default
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES (99, '5559999999')")
	err = server.db.QueryRow("SELECT onboarding_flags FROM users WHERE id = 99").Scan(&flags)
	if err != nil {
		t.Fatalf("failed to read onboarding_flags: %v", err)
	}
	if flags != "{}" {
		t.Errorf("expected default '{}', got '%s'", flags)
	}
}

func TestOnboarding_NameSavedViaSettings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Save name via PATCH /api/settings
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"name": "Penny Tester"}, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH settings: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify name persisted
	var name string
	server.db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
	if name != "Penny Tester" {
		t.Errorf("expected name='Penny Tester', got '%s'", name)
	}

	// Verify /auth/me returns the name
	rr = doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	body := parseJSON(rr)
	if body["name"] != "Penny Tester" {
		t.Errorf("expected name='Penny Tester' in /auth/me, got %v", body["name"])
	}
}

// =============================================================================
// BRAND PORTAL TESTS
// =============================================================================

// brandRegister is a helper that registers a brand user and returns the session cookie and brand_user_id.


func TestGoogleAuthAutoPopulatesPhoto(t *testing.T) {
	// Create a mock image server (HTTPS via TLS)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9}
	imageServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegData)
	}))
	defer imageServer.Close()

	// Create mock Google tokeninfo server with picture URL
	testClientID := "test-google-client-id"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub":            "google-photo-sub-1",
		"email":          "photouser@example.com",
		"email_verified": "true",
		"name":           "Photo User",
		"aud":            testClientID,
		"picture":        imageServer.URL + "/photo.jpg",
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	// Override http.DefaultTransport to trust the TLS test server's certificate
	originalTransport := http.DefaultTransport
	http.DefaultTransport = imageServer.Client().Transport
	defer func() { http.DefaultTransport = originalTransport }()

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	userID := int(body["user_id"].(float64))

	// Verify profile photo was stored
	var photoData []byte
	var contentType string
	err := server.db.QueryRow("SELECT photo_data, content_type FROM profile_photos WHERE user_id = ?", userID).Scan(&photoData, &contentType)
	if err != nil {
		t.Fatalf("expected profile photo to be stored, got error: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("content_type: got %s, want image/jpeg", contentType)
	}
	if len(photoData) == 0 {
		t.Error("expected non-empty photo data")
	}
	// Verify magic bytes
	if photoData[0] != 0xFF || photoData[1] != 0xD8 {
		t.Error("photo data does not start with JPEG magic bytes")
	}
}

func TestGoogleAuthDoesNotOverwriteExistingPhoto(t *testing.T) {
	// Create a mock image server
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9}
	imageServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegData)
	}))
	defer imageServer.Close()

	testClientID := "test-google-client-id"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub":            "google-photo-sub-2",
		"email":          "existingphoto@example.com",
		"email_verified": "true",
		"name":           "Existing Photo User",
		"aud":            testClientID,
		"picture":        imageServer.URL + "/photo.jpg",
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	originalTransport := http.DefaultTransport
	http.DefaultTransport = imageServer.Client().Transport
	defer func() { http.DefaultTransport = originalTransport }()

	// First login to create the user
	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("first login: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	userID := int(body["user_id"].(float64))

	// Manually set a custom photo (PNG magic bytes)
	customPhoto := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	server.db.Exec(
		`UPDATE profile_photos SET photo_data = ?, content_type = 'image/png', updated_at = datetime('now') WHERE user_id = ?`,
		customPhoto, userID,
	)

	// Second login — should NOT overwrite the custom photo
	rr = doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("second login: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify custom photo is still intact
	var photoData []byte
	var contentType string
	err := server.db.QueryRow("SELECT photo_data, content_type FROM profile_photos WHERE user_id = ?", userID).Scan(&photoData, &contentType)
	if err != nil {
		t.Fatalf("expected profile photo to exist: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("expected custom photo to remain (image/png), got %s", contentType)
	}
	if photoData[0] != 0x89 || photoData[1] != 0x50 {
		t.Error("expected custom PNG photo data to remain unchanged")
	}
}

func TestGoogleAuthHandlesMissingPicture(t *testing.T) {
	testClientID := "test-google-client-id"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub":            "google-nopic-sub",
		"email":          "nopic@example.com",
		"email_verified": "true",
		"name":           "No Pic User",
		"aud":            testClientID,
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	userID := int(body["user_id"].(float64))

	// Verify no profile photo was stored
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM profile_photos WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 profile photos, got %d", count)
	}
}

func TestGoogleAuthHandlesInvalidPictureURL(t *testing.T) {
	testClientID := "test-google-client-id"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub":            "google-badurl-sub",
		"email":          "badurl@example.com",
		"email_verified": "true",
		"name":           "Bad URL User",
		"aud":            testClientID,
		"picture":        "http://not-https.example.com/photo.jpg",
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)

	// Auth should still succeed despite invalid picture URL
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	userID := int(body["user_id"].(float64))

	// Verify no profile photo was stored (HTTP URL was rejected)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM profile_photos WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 profile photos (HTTP URL should be rejected), got %d", count)
	}
}

// =============================================================================
// AUTH LINK TESTS (Sprint 8 WS1: Multi-Method Linking)
// =============================================================================

func TestAuthLink_EmailHappyPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Link an email to the phone-authed user
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "test@example.com"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Error("Expected success=true")
	}

	// Verify auth_methods table has the email entry
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE user_id = ? AND provider = 'email' AND provider_id = 'test@example.com'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("auth_methods email row count: got %d, want 1", count)
	}

	// Verify users.email was updated
	var email string
	server.db.QueryRow("SELECT COALESCE(email,'') FROM users WHERE id = ?", userID).Scan(&email)
	if email != "test@example.com" {
		t.Errorf("users.email: got %q, want 'test@example.com'", email)
	}
}

func TestAuthLink_PhoneHappyPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Create user via email auth
	emailUser, err := resolveOrCreateUser(server.db, "email", "user@example.com", "user@example.com", "Test User")
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	token, err := server.sessions.CreateSession(emailUser.ID, "", "email")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	cookie := &http.Cookie{Name: "session", Value: token}

	// Link a phone number (test mode auto-approves code "000000")
	server.config.TestMode = true
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "+15559876543", "code": "000000"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify auth_methods table has the phone entry
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE user_id = ? AND provider = 'phone' AND provider_id = '5559876543'", emailUser.ID).Scan(&count)
	if count != 1 {
		t.Errorf("auth_methods phone row count: got %d, want 1", count)
	}

	// Verify users.phone_number was updated
	var phone sql.NullString
	server.db.QueryRow("SELECT phone_number FROM users WHERE id = ?", emailUser.ID).Scan(&phone)
	if !phone.Valid || phone.String != "5559876543" {
		t.Errorf("users.phone_number: got %q, want '5559876543'", phone.String)
	}
}

func TestAuthLink_Idempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	body := map[string]string{"provider": "email", "email": "test@example.com"}

	// First link
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("First link status: got %d, want 200", rr.Code)
	}

	// Second link (idempotent)
	rr = doRequest(server.handleAuthLink, "POST", "/api/auth/link", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Second link status: got %d, want 200", rr.Code)
	}

	// Verify no duplicate auth_methods row
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE user_id = ? AND provider = 'email' AND provider_id = 'test@example.com'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("auth_methods duplicate check: got %d rows, want 1", count)
	}
}

func TestAuthLink_InvalidProvider(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "foo"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestAuthLink_Unauthorized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "test@example.com"},
		nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestAuthLink_EmailAlreadyTaken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create another user who already has "taken@example.com"
	otherUser, err := resolveOrCreateUser(server.db, "email", "taken@example.com", "taken@example.com", "Other User")
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "taken@example.com", otherUser.ID)

	// Log in as a different user
	cookie, _ := loginUser(t, server, "5551234567")

	// Try to link the already-taken email
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "taken@example.com"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusConflict {
		t.Errorf("Status: got %d, want 409", rr.Code)
	}
}

func TestAuthLink_PhoneAlreadyTaken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	server.config.TestMode = true

	// Create another user who already has phone 5559999999
	_, err := upsertUser(server.db, "5559999999")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	// Create an email-auth user
	emailUser, err := resolveOrCreateUser(server.db, "email", "user2@example.com", "user2@example.com", "User Two")
	if err != nil {
		t.Fatalf("resolveOrCreateUser: %v", err)
	}
	token, err := server.sessions.CreateSession(emailUser.ID, "", "email")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	cookie := &http.Cookie{Name: "session", Value: token}

	// Try to link the already-taken phone
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "+15559999999", "code": "000000"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusConflict {
		t.Errorf("Status: got %d, want 409", rr.Code)
	}
}

func TestAuthLink_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "GET", "/api/auth/link", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestAuthMe_ReturnsAuthMethods(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create user with phone auth
	user, err := upsertUser(server.db, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	// Also add an email auth method
	server.db.Exec("INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'email', 'user@example.com')", user.ID)

	token, err := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{{Name: "session", Value: token}})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)

	// Check that auth_methods is present and contains both providers
	methods, ok := body["auth_methods"].([]interface{})
	if !ok {
		t.Fatalf("auth_methods not an array: %v", body["auth_methods"])
	}

	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[m.(string)] = true
	}

	if !methodSet["phone"] {
		t.Error("auth_methods missing 'phone'")
	}
	if !methodSet["email"] {
		t.Error("auth_methods missing 'email'")
	}
}

func TestAuthMe_ReturnsAuthMethods_SingleMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create user with only phone auth
	user, err := upsertUser(server.db, "5551234567")
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}

	token, err := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{{Name: "session", Value: token}})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	methods, ok := body["auth_methods"].([]interface{})
	if !ok {
		t.Fatalf("auth_methods not an array: %v", body["auth_methods"])
	}

	if len(methods) != 1 {
		t.Errorf("auth_methods length: got %d, want 1", len(methods))
	}
	if methods[0] != "phone" {
		t.Errorf("auth_methods[0]: got %v, want 'phone'", methods[0])
	}
}

// =============================================================================
// WRAPPED ENDPOINT TESTS
// =============================================================================

// --- Account Merge: Audit Trail ---

func TestAccountMerge_AuditTrailLogged(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create user via email first
	user1, _ := resolveOrCreateUser(server.db, "email", "email1", "merge@test.com", "Alice")
	// Merge via Google with same email
	user2, _ := resolveOrCreateUser(server.db, "google", "goog123", "merge@test.com", "Alice G")
	if user2.ID != user1.ID {
		t.Fatalf("expected merge to same user, got %d vs %d", user2.ID, user1.ID)
	}
	if !user2.Merged {
		t.Fatal("expected Merged flag to be true")
	}

	// Check audit trail
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM account_merges WHERE surviving_user = ?", user1.ID).Scan(&count)
	if count == 0 {
		t.Error("expected account_merges row to be created")
	}
	var providerLinked, dataActions string
	server.db.QueryRow("SELECT provider_linked, data_actions FROM account_merges WHERE surviving_user = ? ORDER BY id DESC LIMIT 1", user1.ID).Scan(&providerLinked, &dataActions)
	if providerLinked != "google" {
		t.Errorf("expected provider_linked=google, got %s", providerLinked)
	}
	if dataActions == "" || dataActions == "{}" {
		t.Error("expected data_actions to contain merge details")
	}
}

// --- Account Merge: Household Dedup ---

func TestAccountMerge_HouseholdDedup_SameHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Create user 1 with email
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "hh@test.com", "User1")
	// Create user 2 separately (different email)
	user2, _ := resolveOrCreateUser(server.db, "email", "e2", "hh2@test.com", "User2")

	// Put user2 in user1's household
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (?, ?)", user1.HouseholdID, user2.ID)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", user1.HouseholdID, user2.ID)

	// Now update user2's email to match user1 for a merge scenario
	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "hh@test.com", user2.ID)

	// Trigger merge via a new auth method for the same email
	merged, _ := resolveOrCreateUser(server.db, "google", "g_hh", "hh@test.com", "")

	if merged.ID != user1.ID {
		t.Fatalf("expected merge to user1, got %d", merged.ID)
	}

	// User2 should no longer be in the household
	var memberCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE user_id = ?", user2.ID).Scan(&memberCount)
	if memberCount != 0 {
		t.Errorf("expected orphaned user removed from household, but still has %d memberships", memberCount)
	}
}

func TestAccountMerge_HouseholdDedup_DifferentHouseholds(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "diff@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	// Both should be in different households
	if user1.HouseholdID == user2.HouseholdID {
		t.Fatal("expected different households")
	}

	oldHH := user2.HouseholdID
	// Set user2's email to match user1 for merge
	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "diff@test.com", user2.ID)

	// Trigger merge
	merged, _ := resolveOrCreateUser(server.db, "google", "g_diff", "diff@test.com", "")
	if merged.ID != user1.ID {
		t.Fatalf("expected merge to user1, got %d", merged.ID)
	}

	// User2 should be removed from old household
	var memberCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE user_id = ? AND household_id = ?", user2.ID, oldHH).Scan(&memberCount)
	if memberCount != 0 {
		t.Error("expected orphaned user removed from their old household")
	}
}

// --- Account Merge: Conflict Resolution ---

func TestAccountMerge_ListItems_Union(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "list@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	// User1 has Milk (qty 1) and Bread (qty 2)
	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Milk', 1)", user1.ID)
	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Bread', 2)", user1.ID)
	// User2 has Milk (qty 2) and Eggs (qty 1)
	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Milk', 2)", user2.ID)
	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Eggs', 1)", user2.ID)

	// Set user2's email to match user1
	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "list@test.com", user2.ID)

	// Trigger merge
	resolveOrCreateUser(server.db, "google", "g_list", "list@test.com", "")

	// User1 should now have: Milk (qty 3), Bread (qty 2), Eggs (qty 1)
	var milkQty, breadQty, eggsQty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", user1.ID).Scan(&milkQty)
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Bread'", user1.ID).Scan(&breadQty)
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Eggs'", user1.ID).Scan(&eggsQty)

	if milkQty != 3 {
		t.Errorf("expected Milk qty=3, got %d", milkQty)
	}
	if breadQty != 2 {
		t.Errorf("expected Bread qty=2, got %d", breadQty)
	}
	if eggsQty != 1 {
		t.Errorf("expected Eggs qty=1, got %d", eggsQty)
	}

	// User2 should have no list items
	var u2Count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", user2.ID).Scan(&u2Count)
	if u2Count != 0 {
		t.Errorf("expected orphaned user to have 0 list items, got %d", u2Count)
	}
}

func TestAccountMerge_ProfilePhoto_KeepsNewest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "photo@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	// User1 has older photo, user2 has newer photo
	server.db.Exec("INSERT INTO profile_photos (user_id, photo_data, content_type, updated_at) VALUES (?, X'01', 'image/png', '2026-01-01')", user1.ID)
	server.db.Exec("INSERT INTO profile_photos (user_id, photo_data, content_type, updated_at) VALUES (?, X'02', 'image/png', '2026-03-01')", user2.ID)

	// Set user2's email to match user1
	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "photo@test.com", user2.ID)

	// Trigger merge
	resolveOrCreateUser(server.db, "google", "g_photo", "photo@test.com", "")

	// User1 should have user2's newer photo (0x02)
	var photoData []byte
	server.db.QueryRow("SELECT photo_data FROM profile_photos WHERE user_id = ?", user1.ID).Scan(&photoData)
	if len(photoData) != 1 || photoData[0] != 0x02 {
		t.Errorf("expected user1 to have newer photo (0x02), got %v", photoData)
	}

	// User2 should have no photo
	var u2Count int
	server.db.QueryRow("SELECT COUNT(*) FROM profile_photos WHERE user_id = ?", user2.ID).Scan(&u2Count)
	if u2Count != 0 {
		t.Errorf("expected orphaned user to have 0 photos, got %d", u2Count)
	}
}

func TestAccountMerge_ProfilePhoto_TransfersIfOnlySurvivingHasNone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "nophoto@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	// Only user2 has a photo
	server.db.Exec("INSERT INTO profile_photos (user_id, photo_data, content_type, updated_at) VALUES (?, X'AA', 'image/jpeg', '2026-02-01')", user2.ID)

	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "nophoto@test.com", user2.ID)
	resolveOrCreateUser(server.db, "google", "g_nophoto", "nophoto@test.com", "")

	var photoData []byte
	server.db.QueryRow("SELECT photo_data FROM profile_photos WHERE user_id = ?", user1.ID).Scan(&photoData)
	if len(photoData) != 1 || photoData[0] != 0xAA {
		t.Errorf("expected user1 to get user2's photo (0xAA), got %v", photoData)
	}
}

func TestAccountMerge_DisplayName_PrefersManual(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "name@test.com", "AutoName")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "ManualName")

	// User1 has auto name, user2 has manually set name
	server.db.Exec("UPDATE users SET name_source = 'auto' WHERE id = ?", user1.ID)
	server.db.Exec("UPDATE users SET name_source = 'manual' WHERE id = ?", user2.ID)

	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "name@test.com", user2.ID)
	merged, _ := resolveOrCreateUser(server.db, "google", "g_name", "name@test.com", "")

	if merged.Name != "ManualName" {
		t.Errorf("expected merged user to have manual name 'ManualName', got '%s'", merged.Name)
	}

	// Verify name_source is now manual on surviving user
	var src string
	server.db.QueryRow("SELECT name_source FROM users WHERE id = ?", user1.ID).Scan(&src)
	if src != "manual" {
		t.Errorf("expected name_source=manual, got %s", src)
	}
}

func TestAccountMerge_DisplayName_KeepsSurvivingManual(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "keep@test.com", "MyName")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "OtherManual")

	// Both have manual names — surviving keeps theirs
	server.db.Exec("UPDATE users SET name_source = 'manual' WHERE id = ?", user1.ID)
	server.db.Exec("UPDATE users SET name_source = 'manual' WHERE id = ?", user2.ID)

	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "keep@test.com", user2.ID)
	merged, _ := resolveOrCreateUser(server.db, "google", "g_keep", "keep@test.com", "")

	if merged.Name != "MyName" {
		t.Errorf("expected surviving user's name 'MyName', got '%s'", merged.Name)
	}
}

func TestAccountMerge_OrphanedUserSoftDeleted(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "soft@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "soft@test.com", user2.ID)
	resolveOrCreateUser(server.db, "google", "g_soft", "soft@test.com", "")

	var orphanName, orphanEmail string
	server.db.QueryRow("SELECT COALESCE(name,''), COALESCE(email,'') FROM users WHERE id = ?", user2.ID).Scan(&orphanName, &orphanEmail)

	expectedName := fmt.Sprintf("[merged into user %d]", user1.ID)
	if orphanName != expectedName {
		t.Errorf("expected orphan name '%s', got '%s'", expectedName, orphanName)
	}
	if orphanEmail != "" {
		t.Errorf("expected orphan email to be empty, got '%s'", orphanEmail)
	}
}

func TestAccountMerge_SavedListsTransferred(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "saved@test.com", "User1")
	user2, _ := resolveOrCreateUser(server.db, "phone", "p2", "", "User2")

	// User2 has a saved list
	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly Groceries')", user2.ID)

	server.db.Exec("UPDATE users SET email = ? WHERE id = ?", "saved@test.com", user2.ID)
	resolveOrCreateUser(server.db, "google", "g_saved", "saved@test.com", "")

	// Saved list should now belong to user1
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_lists WHERE user_id = ?", user1.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 saved list transferred to user1, got %d", count)
	}

	var u2Count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_lists WHERE user_id = ?", user2.ID).Scan(&u2Count)
	if u2Count != 0 {
		t.Errorf("expected 0 saved lists remaining on user2, got %d", u2Count)
	}
}

func TestAccountMerge_NoOrphan_AuthMethodOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Normal case: no orphaned user, just auth method linking
	user1, _ := resolveOrCreateUser(server.db, "email", "e1", "authonly@test.com", "Alice")
	user2, _ := resolveOrCreateUser(server.db, "google", "g_auth", "authonly@test.com", "Alice G")

	if user2.ID != user1.ID {
		t.Fatalf("expected same user, got %d vs %d", user2.ID, user1.ID)
	}
	if !user2.Merged {
		t.Fatal("expected Merged flag to be true")
	}

	// Audit trail should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM account_merges WHERE surviving_user = ?", user1.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 audit trail entry, got %d", count)
	}

	// merged_user should be NULL (no orphaned user)
	var mergedUser sql.NullInt64
	server.db.QueryRow("SELECT merged_user FROM account_merges WHERE surviving_user = ?", user1.ID).Scan(&mergedUser)
	if mergedUser.Valid {
		t.Errorf("expected merged_user=NULL for auth-only merge, got %d", mergedUser.Int64)
	}
}

// =============================================================================
// Magic Link Send Tests
// =============================================================================

func TestMagicLink_EmptyEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": ""}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty email, got %d", rr.Code)
	}
}

func TestMagicLink_UnknownEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": "nobody@example.com"}, nil)
	// Should return success to prevent email enumeration
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for unknown email, got %d", rr.Code)
	}
}

func TestMagicLink_ValidUser_CreatesToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register a user first
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "magic@example.com", "password": "password123", "name": "Magic"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %d", rr.Code)
	}

	// Request magic link
	rr = doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": "magic@example.com"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify token was created in database
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM password_resets WHERE token LIKE 'magic:%' AND used = 0").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 magic link token, got %d", count)
	}
}

func TestMagicLink_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleMagicLink, "GET", "/auth/email/magic-link", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- Email Webhook Tests ---

// =============================================================================
// Magic Link Verify
// =============================================================================

func TestMagicLinkVerify_ValidToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a user
	user, _ := upsertUser(server.db, "5559990005")
	server.db.Exec("UPDATE users SET email = 'magic@test.com' WHERE id = ?", user.ID)

	// Insert a magic link token
	magicToken := "test-magic-token-123"
	server.db.Exec(`INSERT INTO password_resets (user_id, token, used, expires_at)
		VALUES (?, ?, 0, datetime('now', '+1 hour'))`, user.ID, "magic:"+magicToken)

	rr := doRequest(server.handleMagicLinkVerify, "GET",
		"/auth/email/magic-verify?token="+magicToken, nil, nil)
	// Should redirect to home
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/" {
		t.Errorf("redirect location: got %s, want /", loc)
	}

	// Should have a session cookie
	var hasCookie bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session" && c.Value != "" {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Error("no session cookie set after magic link verify")
	}

	// Token should be marked as used
	var used int
	server.db.QueryRow("SELECT used FROM password_resets WHERE token = ?", "magic:"+magicToken).Scan(&used)
	if used != 1 {
		t.Errorf("token used: got %d, want 1", used)
	}
}

func TestMagicLinkVerify_ExpiredToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "5559990006")
	server.db.Exec(`INSERT INTO password_resets (user_id, token, used, expires_at)
		VALUES (?, 'magic:expired-token', 0, datetime('now', '-1 hour'))`, user.ID)

	rr := doRequest(server.handleMagicLinkVerify, "GET",
		"/auth/email/magic-verify?token=expired-token", nil, nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "expired-link") {
		t.Errorf("expected redirect to expired-link, got %s", loc)
	}
}

func TestMagicLinkVerify_UsedToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := upsertUser(server.db, "5559990007")
	server.db.Exec(`INSERT INTO password_resets (user_id, token, used, expires_at)
		VALUES (?, 'magic:used-token', 1, datetime('now', '+1 hour'))`, user.ID)

	rr := doRequest(server.handleMagicLinkVerify, "GET",
		"/auth/email/magic-verify?token=used-token", nil, nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "expired-link") {
		t.Errorf("expected redirect to expired-link for used token, got %s", loc)
	}
}

func TestMagicLinkVerify_EmptyToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleMagicLinkVerify, "GET",
		"/auth/email/magic-verify?token=", nil, nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "invalid-link") {
		t.Errorf("expected redirect to invalid-link, got %s", loc)
	}
}

func TestMagicLinkVerify_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleMagicLinkVerify, "POST",
		"/auth/email/magic-verify", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

// =============================================================================
// Brand Transactions
// =============================================================================


// =============================================================================
// Session CleanExpired and DeleteUserSessions TESTS (50%/57% → target ~80%)
// =============================================================================

func TestCleanExpired_RemovesExpiredSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554470001")

	// Insert an expired session directly with SQLite datetime format
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone_number, auth_method, created_at, expires_at) VALUES ('expired_token', ?, '5554470001', 'phone', datetime('now', '-2 days'), datetime('now', '-1 day'))",
		userID,
	)

	// Verify it was inserted
	var before int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&before)
	if before < 2 {
		t.Fatalf("Should have at least 2 sessions (login + expired), got %d", before)
	}

	server.sessions.CleanExpired()

	var after int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&after)
	if after >= before {
		t.Errorf("CleanExpired should remove expired session: before=%d, after=%d", before, after)
	}
}

func TestCleanExpired_KeepsValidSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554470002")

	// loginUser creates a valid session (30-day expiry)
	var before int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&before)

	server.sessions.CleanExpired()

	var after int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&after)
	if after != before {
		t.Errorf("Valid sessions should be preserved: before=%d, after=%d", before, after)
	}
}

func TestDeleteUserSessions_RemovesAll(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554470003")

	// Create additional sessions
	server.sessions.CreateSession(userID, "5554470003", "phone")
	server.sessions.CreateSession(userID, "5554470003", "phone")

	var before int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&before)
	if before < 2 {
		t.Fatalf("Should have at least 2 sessions, got %d", before)
	}

	server.sessions.DeleteUserSessions(userID)

	var after int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&after)
	if after != 0 {
		t.Errorf("All sessions should be deleted, got %d remaining", after)
	}
}

func TestDeleteUserSessions_DoesNotAffectOtherUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5554470004")
	_, userID2 := loginUser(t, server, "5554470005")

	server.sessions.DeleteUserSessions(userID1)

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID2).Scan(&count)
	if count == 0 {
		t.Error("Other user's sessions should not be affected")
	}
}


// =============================================================================
// Batch 7: Google Auth, PennyInsights, ReferralRedeem, BrandRegister, LogoProxy
// =============================================================================

func TestGoogleAuth_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleGoogleAuth, "GET", "/auth/google", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestGoogleAuth_MissingCredential(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google", map[string]string{}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGoogleAuth_AudienceMismatch(t *testing.T) {
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub": "sub1", "email": "test@example.com", "email_verified": "true",
		"name": "Test", "aud": "wrong-client-id",
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = "correct-client-id"
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid token audience" {
		t.Errorf("expected audience error, got %v", body["error"])
	}
}

func TestGoogleAuth_EmailNotVerified(t *testing.T) {
	testClientID := "test-client-id"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub": "sub2", "email": "test@example.com", "email_verified": "false",
		"name": "Test", "aud": testClientID,
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "test-token"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Email not verified" {
		t.Errorf("expected email not verified error, got %v", body["error"])
	}
}

func TestGoogleAuth_InvalidToken(t *testing.T) {
	// Token server returns non-200
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid_token"}`))
	}))
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "bad-token"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestGoogleAuth_SuccessfulLogin(t *testing.T) {
	testClientID := "test-client-id-ok"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub": "google-sub-ok", "email": "ok@example.com", "email_verified": "true",
		"name": "OK User", "aud": testClientID,
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "valid-token"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	if body["user_id"] == nil {
		t.Error("expected user_id in response")
	}
}

func TestGoogleAuth_MergedAccount(t *testing.T) {
	testClientID := "test-client-merge"
	tokenServer := newMockGoogleTokenServer(t, map[string]string{
		"sub": "google-sub-merge", "email": "merged@example.com", "email_verified": "true",
		"name": "Merge User", "aud": testClientID,
	})
	defer tokenServer.Close()

	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = testClientID
	server.googleTokenInfoURL = tokenServer.URL

	// Pre-create user with same email (via email/password)
	server.db.Exec("INSERT INTO users (phone_number, email) VALUES ('', 'merged@example.com')")

	rr := doRequest(server.handleGoogleAuth, "POST", "/auth/google",
		map[string]string{"credential": "merge-token"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["account_merged"] != true {
		t.Errorf("expected account_merged=true, got %v", body["account_merged"])
	}
}
