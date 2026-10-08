package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := newTestSessionStore(t)
	token, err := store.CreateSession("user-1", "5551234567", "phone")

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
	if session.UserID != "user-1" {
		t.Errorf("UserID: got %s, want user-1", session.UserID)
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
	token, _ := store.CreateSession("user-1", "5551234567", "phone")
	store.DeleteSession(token)

	_, ok := store.GetSession(token)
	if ok {
		t.Error("Session still exists after deletion")
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	store := newTestSessionStore(t)
	token1, _ := store.CreateSession("user-1", "5551111111", "phone")
	token2, _ := store.CreateSession("user-2", "5552222222", "phone")

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
	token1, _ := store.CreateSession("user-1", "5551111111", "phone")
	token2, _ := store.CreateSession("user-2", "5552222222", "phone")

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

// =============================================================================
// /auth/me CARRIES NO PHONE FIELDS
// =============================================================================

func TestMe_HasNoPhoneOrSmsConsentFields(t *testing.T) {
	server := newTestServer(t)
	cookie, userID := loginUser(t, server, "5551234567")
	// Even a user row with a legacy phone and a sessions row carrying one must
	// not surface either in /auth/me.
	server.db.Exec(`UPDATE users SET sms_consent_at = NULL WHERE id = ?`, userID)

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	for _, k := range []string{"phone_number", "needs_sms_consent", "phone"} {
		if _, present := body[k]; present {
			t.Errorf("/auth/me must not carry %q, got %v", k, body[k])
		}
	}
	if body["logged_in"] != true {
		t.Error("expected logged_in=true")
	}
}

// =============================================================================
// VERIFY CODE HANDLER TESTS
// =============================================================================

// Regression: the users table has column `phone`, not
// `phone_number`. If any penny SQL queries a nonexistent `phone_number`
// column on users, staging /verify breaks with "We couldn't set up your
// account." Catch that column-drift in tests.
func TestUsersSchema_HasPhoneNotPhoneNumber(t *testing.T) {
	_, pgdb := openTestDB(t)

	colQuery := "SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'users'"
	rows, err := pgdb.Query(colQuery)
	if err != nil {
		t.Fatalf("listing users columns: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			cols[name] = true
		}
	}
	if !cols["phone"] {
		t.Error("users table must have `phone` column")
	}
	if cols["phone_number"] {
		t.Error("users table must NOT have `phone_number` column — production uses `phone`; the dual column masked staging /verify failures")
	}
}

// =============================================================================
// ME HANDLER TESTS
// =============================================================================

func TestMe_LoggedIn(t *testing.T) {
	server := newTestServer(t)
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
	if _, present := body["phone_number"]; present {
		t.Errorf("phone_number must not be in /auth/me, got %v", body["phone_number"])
	}
	if uid, ok := body["user_id"].(string); !ok || uid != user.ID {
		t.Errorf("user_id: got %v, want %s", body["user_id"], user.ID)
	}
}

func TestMe_NotLoggedIn(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMe_ExpiredSession(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil,
		[]*http.Cookie{{Name: "session", Value: "fake-expired-token"}})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMe_WrongMethod(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleMe, "POST", "/auth/me", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// LOGOUT HANDLER TESTS
// =============================================================================

func TestLogout_ClearsSession(t *testing.T) {
	server := newTestServer(t)
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")

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
	server := newTestServer(t)
	rr := doRequest(server.handleLogout, "POST", "/auth/logout", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestLogout_WrongMethod(t *testing.T) {
	server := newTestServer(t)
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

// =============================================================================
// LIST HANDLER TESTS
// =============================================================================

// loginUser creates a user in the DB and returns a session cookie for that user.

// --- requireSession expired token ---

func TestRequireSession_ExpiredToken(t *testing.T) {
	server := newTestServer(t)

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

// --- Email webhook form parse error ---

// =============================================================================
// REQUIRE ADMIN HELPER
// =============================================================================

func TestRequireAdmin_NonAdminUser(t *testing.T) {
	server := newTestServer(t)

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
	server := newTestServer(t)

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
	rawDB, pgdb := openTestDB(t)
	store := NewSessionStore(pgdb)

	// Create user for FK
	rawDB.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")

	// Insert an already-expired session directly
	rawDB.Exec("INSERT INTO sessions (token, user_id, phone, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"expired-token", "user-1", "5551234567", time.Now().UTC().Add(-48*time.Hour), time.Now().UTC().Add(-24*time.Hour))

	// Insert a valid session
	rawDB.Exec("INSERT INTO sessions (token, user_id, phone, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"valid-token", "user-1", "5551234567", time.Now().UTC(), time.Now().UTC().Add(24*time.Hour))

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
	rawDB, pgdb := openTestDB(t)
	store := NewSessionStore(pgdb)

	// Create user for FK
	rawDB.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")

	// Insert expired session
	rawDB.Exec("INSERT INTO sessions (token, user_id, phone, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		"old-token", "user-1", "5551234567", time.Now().UTC().Add(-48*time.Hour), time.Now().UTC().Add(-1*time.Hour))

	_, ok := store.GetSession("old-token")
	if ok {
		t.Error("Expired session should not be returned by GetSession")
	}
}

// =============================================================================
// BRANDS ENDPOINT TESTS
// =============================================================================

func TestSecureCookie_EnvAware(t *testing.T) {
	// Without SecureCookies, cookie should not be Secure
	rr := httptest.NewRecorder()
	setSessionCookie(rr, Config{SecureCookies: false}, "tok")
	cookie := getSessionCookie(rr)
	if cookie == nil || cookie.Secure {
		t.Error("Expected non-secure cookie when SecureCookies=false")
	}

	// With SecureCookies, cookie should be Secure and HttpOnly
	rr = httptest.NewRecorder()
	setSessionCookie(rr, Config{SecureCookies: true}, "tok")
	cookie = getSessionCookie(rr)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}
	if !cookie.Secure || !cookie.HttpOnly {
		t.Error("Expected secure, HttpOnly cookie when SecureCookies=true")
	}
}

// =============================================================================
// REFERRAL REDEEM TESTS
// =============================================================================

func TestLogout_CookieHasSecurityFlags(t *testing.T) {
	server := newTestServer(t)
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

// =============================================================================
// EMAIL REGISTER HANDLER TESTS
// =============================================================================

// =============================================================================
// EMAIL LOGIN HANDLER TESTS
// =============================================================================

// =============================================================================
// EMAIL FORGOT / RESET HANDLER TESTS
// =============================================================================

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================

// =============================================================================
// REQUIRE SAVED LIST ACCESS TESTS
// =============================================================================
// ONBOARDING TESTS
// =============================================================================

func TestOnboarding_MeReturnsLoginCount(t *testing.T) {
	server := newTestServer(t)
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
	server := newTestServer(t)

	// Simulate a phone login which increments login_count
	server.db.Exec("INSERT INTO users (id, phone, login_count) VALUES ('user-1', '5551234567', 0)")
	server.db.Exec("INSERT INTO auth_methods (user_id, provider, provider_id) VALUES ('user-1', 'phone', '5551234567')")

	// Manually increment login_count as would happen during auth
	server.db.Exec("UPDATE users SET login_count = 3 WHERE id = 'user-1'")

	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	body := parseJSON(rr)
	lc := body["login_count"].(float64)
	if int(lc) != 3 {
		t.Errorf("expected login_count=3, got %d", int(lc))
	}
}

func TestOnboarding_MeReturnsName(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone, name) VALUES ('user-1', '5551234567', 'Andy')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, cookies)
	body := parseJSON(rr)

	name, ok := body["name"].(string)
	if !ok || name != "Andy" {
		t.Errorf("expected name='Andy', got %v", body["name"])
	}
}

func TestOnboarding_OnboardingFlagsColumnExists(t *testing.T) {
	server := newTestServer(t)

	// Verify onboarding_flags column exists with default value
	var flags string
	err := server.db.QueryRow("SELECT onboarding_flags FROM users WHERE id = 'nonexistent'").Scan(&flags)
	if err == nil && flags != "{}" {
		t.Errorf("expected default onboarding_flags='{}', got '%s'", flags)
	}

	// Insert a user and verify default
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-99', '5559999999')")
	err = server.db.QueryRow("SELECT onboarding_flags FROM users WHERE id = 'user-99'").Scan(&flags)
	if err != nil {
		t.Fatalf("failed to read onboarding_flags: %v", err)
	}
	if flags != "{}" {
		t.Errorf("expected default '{}', got '%s'", flags)
	}
}

func TestOnboarding_NameSavedViaSettings(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Save name via PATCH /api/settings
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"name": "Penny Tester"}, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH settings: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify name persisted
	var name string
	server.db.QueryRow("SELECT name FROM users WHERE id = 'user-1'").Scan(&name)
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

// =============================================================================
// AUTH LINK TESTS (Sprint 8 WS1: Multi-Method Linking)
// =============================================================================

func TestAuthMe_ReturnsAuthMethods(t *testing.T) {
	server := newTestServer(t)

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
	server := newTestServer(t)

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

// --- Account Merge: Household Dedup ---

// --- Account Merge: Conflict Resolution ---

// =============================================================================
// Magic Link Send Tests
// =============================================================================

// --- Email Webhook Tests ---

// =============================================================================
// Magic Link Verify
// =============================================================================

// =============================================================================
// Brand Transactions
// =============================================================================

// =============================================================================
// Session CleanExpired and DeleteUserSessions TESTS (50%/57% → target ~80%)
// =============================================================================

func TestCleanExpired_RemovesExpiredSessions(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "5554470001")

	// Insert an expired session directly with a plain datetime string
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('expired_token', ?, '5554470001', 'user', datetime('now', '-2 days'), datetime('now', '-1 day'))",
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
	server := newTestServer(t)
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
	server := newTestServer(t)
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
	server := newTestServer(t)
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

// =============================================================================
// GOOGLE TOKENINFO TRANSPORT + LOG HYGIENE
// =============================================================================
