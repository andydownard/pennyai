package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)


// =============================================================================
// HOUSEHOLD TESTS
// =============================================================================

func TestHouseholdAutoCreation(t *testing.T) {
	_, db := openTestDB(t)

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
	if !strings.Contains(body, "penny@list.groflo.io") {
		t.Error("Body should contain penny@list.groflo.io")
	}
	if !strings.Contains(body, "N:AI;Penny") {
		t.Error("Body should contain N:AI;Penny")
	}
	if !strings.Contains(body, "ORG:Groflo") {
		t.Error("Body should contain ORG:Groflo")
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
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-invite-1', ?)", "5551234567")
	var userID string
	server.db.QueryRow("SELECT id FROM users WHERE phone = '5551234567'").Scan(&userID)

	token, _ := server.sessions.CreateSession(userID, "5551234567", "phone")
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
// SETTINGS — METHOD DISPATCH
// =============================================================================
// CREATE SOLO HOUSEHOLD HELPER
// =============================================================================
// VCARD — METHOD NOT ALLOWED
// =============================================================================
// SETTINGS — METHOD DISPATCH
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

	// Join own household — should be rejected
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400 (self-join rejected)", rr.Code)
	}
}

// --- SMS form parse error ---

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
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
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

// TestReferral_ConcurrentClaimCreditsOnce fires simultaneous claims of the same
// referral code. The "already claimed" SELECT ran outside the transaction, and
// the INSERT INTO referrals error was logged and stepped past, so the points
// UPDATE ran regardless of whether the referral row was actually created.
// referral_points convert to a $10 Venmo payout, so this is money.
func TestReferral_ConcurrentClaimCreditsOnce(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cookieA, referrerID := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	referralCode := parseJSON(rr)["code"].(string)

	cookieB, referredID := loginUser(t, server, "5552222222")

	const n = 8
	codes := make([]int, n)
	hammer(t, n, func(i int) {
		rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
			map[string]string{"code": referralCode}, []*http.Cookie{cookieB})
		codes[i] = rr.Code
	})

	// Every claim is idempotent, so every one reports success. A 500 here is the
	// Postgres failure mode: the swallowed INSERT error aborts the transaction
	// and the Commit then fails.
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("claim %d: expected 200 (idempotent), got %d", i, c)
		}
	}

	var referralRows int
	server.db.QueryRow("SELECT COUNT(*) FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, referredID).Scan(&referralRows)
	if referralRows != 1 {
		t.Errorf("referrals rows: got %d, want 1", referralRows)
	}

	var points int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", referrerID).Scan(&points)
	if points != referralPointsPerInvite {
		t.Errorf("referral_points: got %d, want %d (double credit converts to a real payout)", points, referralPointsPerInvite)
	}
}

// TestReferral_ClaimWithPreSeededRowCreditsNothing is the deterministic partner
// to the hammer test: the referral row already exists, so the claim must report
// success and credit no further points.
func TestReferral_ClaimWithPreSeededRowCreditsNothing(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cookieA, referrerID := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookieA})
	referralCode := parseJSON(rr)["code"].(string)

	cookieB, referredID := loginUser(t, server, "5552222222")

	// A prior claim already landed the row and the points.
	if _, err := server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, referredID); err != nil {
		t.Fatalf("seeding referral row: %v", err)
	}
	server.db.Exec("UPDATE users SET referral_points = ? WHERE id = ?", referralPointsPerInvite, referrerID)

	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": referralCode}, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("repeat claim: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var points int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", referrerID).Scan(&points)
	if points != referralPointsPerInvite {
		t.Errorf("referral_points after a repeat claim: got %d, want %d", points, referralPointsPerInvite)
	}
	var referralRows int
	server.db.QueryRow("SELECT COUNT(*) FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, referredID).Scan(&referralRows)
	if referralRows != 1 {
		t.Errorf("referrals rows: got %d, want 1", referralRows)
	}
}

// TestReferral_UniqueIndexInTestSchema pins idx_referrals_unique into the test
// schema. Production has had it since schema.go:582, but the hand-copied SQLite
// test schema did not, so the referral idempotency tests were passing against
// code that would double-credit.
func TestReferral_UniqueIndexInTestSchema(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('ref-a', '5551110000')")
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('ref-b', '5552220000')")

	if _, err := server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES ('ref-a', 'ref-b')"); err != nil {
		t.Fatalf("first referral insert: %v", err)
	}
	if _, err := server.db.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES ('ref-a', 'ref-b')"); err == nil {
		t.Error("duplicate (referrer_id, referred_id) accepted — idx_referrals_unique missing from the test schema")
	}
}

// =============================================================================
// INVITE SMS GUARDS
// =============================================================================

// TestInvite_PerInviterRateLimit: an authenticated user could POST
// /api/settings/invite in a loop with arbitrary phone numbers, and every call
// fired a Twilio SMS carrying the inviter's name. Unwanted outbound SMS is
// CLAUDE.md damage risk #1, and this is also a harassment vector and an A2P
// 10DLC reputation risk. Every other send site in the codebase is guarded.
func TestInvite_PerInviterRateLimit(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	// A different destination each time, so the per-destination guard is not
	// what is under test here.
	sent := 0
	for i := 0; i < inviteSMSPerInviterPerDay; i++ {
		rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
			map[string]string{"phone": fmt.Sprintf("555200%04d", i)}, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Fatalf("invite %d: expected 200, got %d: %s", i, rr.Code, rr.Body.String())
		}
		sent++
	}
	if got := mock.GetSMSCount(); got != sent {
		t.Fatalf("SMS sent within the limit: got %d, want %d", got, sent)
	}

	// One past the limit must be refused, and must send nothing.
	before := mock.GetSMSCount()
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5552009999"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("invite past the daily limit: expected 429, got %d: %s", rr.Code, rr.Body.String())
	}
	if after := mock.GetSMSCount(); after != before {
		t.Errorf("SMS sent past the limit: count went %d -> %d, want no change", before, after)
	}
}

// TestInvite_PerDestinationDedup: inviting the same phone twice from one
// household must text that person once. Nothing stopped a loop against a single
// number before.
func TestInvite_PerDestinationDedup(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5553334444"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("first invite: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := mock.GetSMSCount(); got != 1 {
		t.Fatalf("after first invite: got %d SMS, want 1", got)
	}

	// Same destination again: the invite is still created (the caller gets a
	// fresh link to share by other means) but no second SMS goes out.
	rr = doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": "5553334444"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("repeat invite: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := mock.GetSMSCount(); got != 1 {
		t.Errorf("after repeat invite to the same phone: got %d SMS, want 1", got)
	}
	if parseJSON(rr)["code"] == nil {
		t.Error("repeat invite should still return a usable invite code")
	}
}

// TestInvite_ConcurrentSameDestinationSendsOnce hammers one destination from one
// inviter: still exactly one SMS.
func TestInvite_ConcurrentSameDestinationSendsOnce(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	hammer(t, 8, func(int) {
		doRequest(server.handleInvite, "POST", "/api/settings/invite",
			map[string]string{"phone": "5557778888"}, []*http.Cookie{cookie})
	})

	if got := mock.GetSMSCount(); got != 1 {
		t.Errorf("concurrent invites to one phone: got %d SMS, want 1", got)
	}
}

// TestInvite_WithoutPhoneIsNotRateLimited: link-only invites send no SMS, so
// they must not consume the SMS quota.
func TestInvite_WithoutPhoneIsNotRateLimited(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	for i := 0; i < inviteSMSPerInviterPerDay+5; i++ {
		rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
			map[string]string{"phone": ""}, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Fatalf("link-only invite %d: expected 200, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}
	if got := mock.GetSMSCount(); got != 0 {
		t.Errorf("link-only invites sent %d SMS, want 0", got)
	}
}

func TestReferral_CannotSelfRefer(t *testing.T) {
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	cookie, _ := loginUser(t, server, "5551111111")
	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "nonexistent"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Invalid code status: got %d, want 404", rr.Code)
	}
}

func TestReferral_RequiresAuth(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth, got %d", rr.Code)
	}
}

// =============================================================================
// ITEM MODIFICATION HISTORY TESTS
// =============================================================================

func TestReferralClaim_FullFlow(t *testing.T) {
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551111111")

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty code, got %d", rr.Code)
	}
}

func TestLeaveHousehold_FullFlow(t *testing.T) {
	mock := &MockTwilioClient{}
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

func TestSettingsUpdate_EmailConflict(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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
	server := newTestServer(t, &MockTwilioClient{})
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

func TestCreateSoloHousehold_Success(t *testing.T) {
	_, db := openTestDB(t)

	// Create user directly (no upsertUser which auto-creates household)
	userID := "test-solo-user"
	_, err := db.Exec("INSERT INTO users (id, phone) VALUES (?, '5559999999')", userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

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

func TestSettingsGet_WithHouseholdMembers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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

func TestReferral_GetWithFriends(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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
	mock := &MockTwilioClient{}
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
	mock := &MockTwilioClient{}
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

func TestSettingsUpdate_NameOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	email := "test@example.com"
	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]interface{}{"email": &email}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestReferralClaim_SelfRefer(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551111111")

	server.db.Exec("UPDATE users SET referral_code = 'mycode' WHERE id = ?", userID)

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "mycode"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for self-refer, got %d", rr.Code)
	}
}

func TestReferralClaim_Idempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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

func TestSettingsUpdate_EmailUniqueCheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
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

func TestCreateSoloHousehold_VerifyMembership(t *testing.T) {
	_, db := openTestDB(t)

	// Insert user without upsertUser (which auto-creates household)
	db.Exec("INSERT INTO users (id, phone) VALUES ('user-solo-1', '5551234567')")
	var userID string
	db.QueryRow("SELECT id FROM users WHERE phone = '5551234567'").Scan(&userID)

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
	_, db := openTestDB(t)

	user, _ := upsertUser(db, "5551234567")
	ids, err := getHouseholdUserIDs(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	// User with no household should get back just their own ID
	if len(ids) != 1 || ids[0] != user.ID {
		t.Errorf("ids = %v, want [%s]", ids, user.ID)
	}
}

func TestGetHouseholdUserIDs_WithHousehold(t *testing.T) {
	_, db := openTestDB(t)

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
	_, db := openTestDB(t)

	ids, err := getHouseholdUserIDs(db, "user-99999")
	if err != nil {
		t.Fatal(err)
	}
	// Should fallback to returning just the provided ID
	if len(ids) != 1 || ids[0] != "user-99999" {
		t.Errorf("ids = %v, want [user-99999]", ids)
	}
}

func TestHouseholdItemsWhereClause_Solo(t *testing.T) {
	_, db := openTestDB(t)

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
	_, db := openTestDB(t)

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

// =============================================================================
// SETTINGS HANDLER TESTS
// =============================================================================

func TestSettingsGet_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleSettingsGet, "GET", "/api/settings", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestSettingsGet_ReturnsData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990001")

	rr := doRequest(server.handleSettingsGet, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	// Should return valid JSON
	data := parseJSON(rr)
	if data == nil {
		t.Error("Expected JSON response, got nil")
	}
}

func TestSettingsUpdate_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]string{"name": "Alice"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestSettingsUpdate_Name(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559990002")

	rr := doRequest(server.handleSettingsUpdate, "PATCH", "/api/settings",
		map[string]string{"name": "Alice"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify name was saved in DB
	var name string
	server.db.QueryRow("SELECT COALESCE(name,'') FROM users WHERE id = ?", userID).Scan(&name)
	if name != "Alice" {
		t.Errorf("Name in DB: got %q, want %q", name, "Alice")
	}
}

func TestSettings_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990003")
	rr := doRequest(server.handleSettings, "DELETE", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// TRIPS HANDLER TESTS
// =============================================================================

func TestTrips_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleTrips, "GET", "/api/trips", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestTrips_GetEmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990010")

	rr := doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var trips []interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &trips); err != nil {
		t.Errorf("Expected JSON array, got: %s", rr.Body.String())
	}
	if len(trips) != 0 {
		t.Errorf("Expected empty trips list, got %d trips", len(trips))
	}
}

func TestTrips_PostAndRetrieve(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990011")

	// POST a new trip
	body := map[string]string{
		"trip_date":  "2099-01-15",
		"trip_time":  "10:00",
		"time_label": "Morning",
	}
	rr := doRequest(server.handleTrips, "POST", "/api/trips", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("POST Status: got %d, want 200", rr.Code)
	}

	// GET should now include the trip
	rr = doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("GET Status: got %d, want 200", rr.Code)
	}

	var trips []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &trips)
	if len(trips) == 0 {
		t.Error("Expected at least one trip after POST")
	}
}

func TestTrips_Delete(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990012")

	// Create a trip
	doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": "2099-02-01", "trip_time": "09:00", "time_label": "Morning"},
		[]*http.Cookie{cookie})

	// Delete it
	rr := doRequest(server.handleTrips, "DELETE", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE Status: got %d, want 200", rr.Code)
	}
}

// =============================================================================
// NPS HANDLER TESTS
// =============================================================================
// VCARD HANDLER TESTS
// =============================================================================

func TestVCard_ReturnsVCardContentType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleVCard, "GET", "/api/penny.vcf", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "text/vcard" {
		t.Errorf("Content-Type: got %q, want %q", ct, "text/vcard")
	}
	if !strings.Contains(rr.Body.String(), "BEGIN:VCARD") {
		t.Error("Expected vCard body to contain BEGIN:VCARD")
	}
}

func TestVCard_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleVCard, "POST", "/api/penny.vcf", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// VALIDATE TWILIO SIGNATURE TESTS
// =============================================================================
// REFERRAL HANDLER TESTS
// =============================================================================

func TestReferralClaim_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestReferral_GetCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990040")

	rr := doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	data := parseJSON(rr)
	if data["code"] == nil {
		t.Error("Expected 'code' field in referral response")
	}
}

// =============================================================================
// ROUND 3 — Constants, Restore, Cross-Household, NPS, Referral Tests
// =============================================================================

func TestReferralConstants_RedeemThreshold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("UPDATE users SET referral_points = ? WHERE id = ?", referralRedeemThreshold-1, userID)
	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 with %d points, got %d", referralRedeemThreshold-1, rr.Code)
	}

	server.db.Exec("UPDATE users SET referral_points = ? WHERE id = ?", referralRedeemThreshold, userID)
	rr = doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with %d points, got %d; body=%s", referralRedeemThreshold, rr.Code, rr.Body.String())
	}
}

func TestReferralConstants_PointsAwardedPerInvite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cookie1, inviterID := loginUser(t, server, "5551111111")

	inviteReq := map[string]interface{}{"phone": ""}
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite", inviteReq, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("invite create failed: %d", rr.Code)
	}
	data := parseJSON(rr)
	link, _ := data["link"].(string)
	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	code := parts[len(parts)-1]

	cookie2, _ := loginUser(t, server, "5552222222")
	joinReq := map[string]interface{}{"code": code}
	rr = doRequest(server.handleJoin, "POST", "/api/settings/join", joinReq, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("join failed: %d; body=%s", rr.Code, rr.Body.String())
	}

	var points int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", inviterID).Scan(&points)
	if points != referralPointsPerInvite {
		t.Errorf("expected inviter to receive %d points, got %d", referralPointsPerInvite, points)
	}
}

func TestSettingsUpdate_AddressField(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	update := map[string]interface{}{
		"name":    "Jane Doe",
		"email":   "jane@example.com",
		"address": "123 Main St, Springfield, IL 62701",
	}
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings", update, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH settings: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	rr = doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["address"] != "123 Main St, Springfield, IL 62701" {
		t.Errorf("expected address from GET, got %v", result["address"])
	}
}

func TestSettingsUpdate_AddressClearedByEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	doRequest(server.handleSettings, "PATCH", "/api/settings", map[string]interface{}{"address": "123 Main St"}, []*http.Cookie{cookie})
	doRequest(server.handleSettings, "PATCH", "/api/settings", map[string]interface{}{"address": ""}, []*http.Cookie{cookie})

	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["address"] != "" {
		t.Errorf("expected empty address after clear, got %v", result["address"])
	}
}

// =============================================================================
// SETTINGS EDGE CASES
// =============================================================================

func TestSettings_EdgeCase_PennySliderNegative(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Negative slider value should be clamped to 0
	negVal := -50
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_slider": negVal}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH with negative slider: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var stored int
	server.db.QueryRow("SELECT penny_slider FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 0 {
		t.Errorf("Expected penny_slider clamped to 0, got %d", stored)
	}
}

func TestSettings_EdgeCase_PennySliderOver100(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Slider > 100 should be clamped to 100
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_slider": 999}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH with slider=999: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var stored int
	server.db.QueryRow("SELECT penny_slider FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 100 {
		t.Errorf("Expected penny_slider clamped to 100, got %d", stored)
	}
}

func TestSettings_EdgeCase_PennySliderBoundary0(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Exactly 0 should be accepted as-is
	zero := 0
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_slider": zero}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH with slider=0: got %d, want 200", rr.Code)
	}

	var stored int
	server.db.QueryRow("SELECT penny_slider FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 0 {
		t.Errorf("Expected penny_slider=0, got %d", stored)
	}
}

func TestSettings_EdgeCase_PennySliderBoundary100(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_slider": 100}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH with slider=100: got %d, want 200", rr.Code)
	}

	var stored int
	server.db.QueryRow("SELECT penny_slider FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 100 {
		t.Errorf("Expected penny_slider=100, got %d", stored)
	}
}

func TestSettings_EdgeCase_InvalidBrandMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Invalid brand mode should return 400
	badMode := "invalid_mode"
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_brand_mode": badMode}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PATCH with invalid brand mode: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	if result["error"] != "Invalid penny_brand_mode" {
		t.Errorf("Expected 'Invalid penny_brand_mode' error, got %v", result["error"])
	}
}

func TestSettings_EdgeCase_ValidBrandModes(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	validModes := []string{"auto", "ask", "deals"}
	for _, mode := range validModes {
		rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
			map[string]interface{}{"penny_brand_mode": mode}, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Errorf("PATCH brand_mode=%q: got %d, want 200. Body: %s", mode, rr.Code, rr.Body.String())
		}

		var stored string
		server.db.QueryRow("SELECT penny_brand_mode FROM users WHERE id = ?", userID).Scan(&stored)
		if stored != mode {
			t.Errorf("Expected brand_mode=%q, got %q", mode, stored)
		}
	}
}

func TestSettings_EdgeCase_EmptyBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Empty JSON object — no fields to update, should succeed as no-op
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("PATCH with empty body: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSettings_EdgeCase_PennyAutotuneToggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Set autotune to false
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_autotune": false}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH autotune=false: got %d, want 200", rr.Code)
	}

	var stored int
	server.db.QueryRow("SELECT penny_autotune FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 0 {
		t.Errorf("Expected penny_autotune=0, got %d", stored)
	}

	// Set autotune back to true
	rr = doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_autotune": true}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH autotune=true: got %d, want 200", rr.Code)
	}

	server.db.QueryRow("SELECT penny_autotune FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 1 {
		t.Errorf("Expected penny_autotune=1, got %d", stored)
	}
}

func TestSettings_EdgeCase_PennyDealsToggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Set deals to true
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_deals": true}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH penny_deals=true: got %d, want 200", rr.Code)
	}

	var stored int
	server.db.QueryRow("SELECT penny_deals FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 1 {
		t.Errorf("Expected penny_deals=1, got %d", stored)
	}

	// Set deals back to false
	rr = doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_deals": false}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH penny_deals=false: got %d, want 200", rr.Code)
	}

	server.db.QueryRow("SELECT penny_deals FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != 0 {
		t.Errorf("Expected penny_deals=0, got %d", stored)
	}
}

func TestSettings_EdgeCase_MultipleFieldsAtOnce(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Update slider, autotune, brand mode, and name all at once
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{
			"name":             "Alice",
			"penny_slider":     75,
			"penny_autotune":   false,
			"penny_brand_mode": "auto",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH multiple fields: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var name, mode string
	var slider, autotune int
	server.db.QueryRow("SELECT name, penny_slider, penny_autotune, penny_brand_mode FROM users WHERE id = ?", userID).Scan(&name, &slider, &autotune, &mode)
	if name != "Alice" {
		t.Errorf("Expected name=Alice, got %q", name)
	}
	if slider != 75 {
		t.Errorf("Expected slider=75, got %d", slider)
	}
	if autotune != 0 {
		t.Errorf("Expected autotune=0, got %d", autotune)
	}
	if mode != "auto" {
		t.Errorf("Expected mode=auto, got %q", mode)
	}
}

func TestSettings_EdgeCase_WhitespaceNameTrimmed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"name": "  Bob  "}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH whitespace name: got %d, want 200", rr.Code)
	}

	var stored string
	server.db.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != "Bob" {
		t.Errorf("Expected trimmed name 'Bob', got %q", stored)
	}
}

func TestSettings_EdgeCase_WhitespaceBrandModeTrimmed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Whitespace-padded but valid mode should be trimmed and accepted
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_brand_mode": "  ask  "}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH whitespace brand mode: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var stored string
	server.db.QueryRow("SELECT penny_brand_mode FROM users WHERE id = ?", userID).Scan(&stored)
	if stored != "ask" {
		t.Errorf("Expected trimmed brand_mode='ask', got %q", stored)
	}
}

func TestSettings_EdgeCase_GetReflectsClampedSlider(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Set slider to a clamped value
	doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]interface{}{"penny_slider": -10}, []*http.Cookie{cookie})

	// GET should reflect clamped value
	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET settings: got %d, want 200", rr.Code)
	}
	result := parseJSON(rr)
	if result["penny_slider"] != float64(0) {
		t.Errorf("Expected penny_slider=0 in GET response, got %v", result["penny_slider"])
	}
}

// =============================================================================
// SAVED LISTS EDGE CASES
// =============================================================================
// TRIPS EDGE CASES
// =============================================================================

func TestTrips_EdgeCase_InvalidDateFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	badDates := []struct {
		name string
		date string
	}{
		{"month-day-year", "01-15-2099"},
		{"slash format", "01/15/2099"},
		{"text format", "January 15, 2099"},
		{"no separators", "20990115"},
		{"partial date", "2099-01"},
		{"garbage", "not-a-date"},
	}

	for _, tt := range badDates {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(server.handleTrips, "POST", "/api/trips",
				map[string]string{"trip_date": tt.date}, []*http.Cookie{cookie})
			if rr.Code != http.StatusBadRequest {
				t.Errorf("POST with date=%q: got %d, want 400. Body: %s", tt.date, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestTrips_EdgeCase_MissingTripDate(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// No trip_date field at all
	rr := doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_time": "15:00"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST without trip_date: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["error"] != "trip_date is required" {
		t.Errorf("Expected 'trip_date is required' error, got %v", result["error"])
	}
}

func TestTrips_EdgeCase_EmptyBodyPost(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Empty JSON body — trip_date missing
	rr := doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with empty body: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestTrips_EdgeCase_DeleteWhenNoTrips(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Delete with no trips should still succeed (idempotent)
	rr := doRequest(server.handleTrips, "DELETE", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE with no trips: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["ok"] != true {
		t.Errorf("Expected ok=true, got %v", result["ok"])
	}
}

func TestTrips_EdgeCase_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleTrips, "PATCH", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PATCH trips: got %d, want 405", rr.Code)
	}
}

func TestTrips_EdgeCase_WithTimeLabel(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create trip with time_label
	rr := doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{
			"trip_date":  "2099-06-15",
			"trip_time":  "10:00",
			"time_label": "Morning",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Create trip with time_label: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify time_label is stored and returned
	rr = doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	var trips []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &trips)
	if len(trips) != 1 {
		t.Fatalf("Expected 1 trip, got %d", len(trips))
	}
	if trips[0]["time_label"] != "Morning" {
		t.Errorf("Expected time_label='Morning', got %v", trips[0]["time_label"])
	}
	if trips[0]["trip_time"] != "10:00" {
		t.Errorf("Expected trip_time='10:00', got %v", trips[0]["trip_time"])
	}
}

func TestTrips_EdgeCase_EmptyTripDateString(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Explicit empty string for trip_date
	rr := doRequest(server.handleTrips, "POST", "/api/trips",
		map[string]string{"trip_date": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with empty trip_date: got %d, want 400", rr.Code)
	}
}

func TestTrips_EdgeCase_NilBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// nil body should fail gracefully
	rr := doRequest(server.handleTrips, "POST", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with nil body: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestTrips_EdgeCase_GetEmptyReturnsArray(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleTrips, "GET", "/api/trips", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET empty trips: got %d, want 200", rr.Code)
	}
	// Should be an empty array, not null
	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" {
		t.Errorf("Expected empty array '[]', got %q", body)
	}
}

// =============================================================================
// NPS EDGE CASES
// =============================================================================
// SETTINGS PHONE UPDATE TESTS
// =============================================================================

// =============================================================================
// EARNINGS ENDPOINT TESTS
// =============================================================================

// setupEarningsTestData creates a user with brand_deals, swap_events, and swap_rebates
// for earnings tests. Returns (cookie, userID, dealID, rebateID).


// =============================================================================
// DIETARY PROFILE TESTS
// =============================================================================

func TestDietaryProfile_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleDietaryPreferences, "GET", "/api/preferences", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestDietaryProfile_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleDietaryPreferences, "GET", "/api/preferences", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	allergens, ok := body["allergens"].([]interface{})
	if !ok {
		t.Fatal("Expected allergens to be an array")
	}
	if len(allergens) != 0 {
		t.Errorf("allergens: got %v, want empty", allergens)
	}
	diets, ok := body["diets"].([]interface{})
	if !ok {
		t.Fatal("Expected diets to be an array")
	}
	if len(diets) != 0 {
		t.Errorf("diets: got %v, want empty", diets)
	}
	avoidBrands, ok := body["avoid_brands"].([]interface{})
	if !ok {
		t.Fatal("Expected avoid_brands to be an array")
	}
	if len(avoidBrands) != 0 {
		t.Errorf("avoid_brands: got %v, want empty", avoidBrands)
	}
}

func TestDietaryProfile_SetAndGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// PATCH to set allergens and diets
	rr := doRequest(server.handleDietaryPreferences, "PATCH", "/api/preferences",
		map[string]interface{}{
			"allergens": []string{"peanuts", "shellfish"},
			"diets":     []string{"vegetarian"},
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH status: got %d, want 200", rr.Code)
	}
	patchBody := parseJSON(rr)
	if patchBody["ok"] != true {
		t.Errorf("PATCH response: got %v, want ok:true", patchBody)
	}

	// GET to verify
	rr = doRequest(server.handleDietaryPreferences, "GET", "/api/preferences", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	allergens := body["allergens"].([]interface{})
	if len(allergens) != 2 {
		t.Fatalf("allergens: got %d items, want 2", len(allergens))
	}
	if allergens[0] != "peanuts" || allergens[1] != "shellfish" {
		t.Errorf("allergens: got %v, want [peanuts shellfish]", allergens)
	}
	diets := body["diets"].([]interface{})
	if len(diets) != 1 || diets[0] != "vegetarian" {
		t.Errorf("diets: got %v, want [vegetarian]", diets)
	}
	avoidBrands := body["avoid_brands"].([]interface{})
	if len(avoidBrands) != 0 {
		t.Errorf("avoid_brands: got %v, want empty", avoidBrands)
	}
}

func TestDietaryProfile_UpdatePartial(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Set initial profile with allergens and diets
	doRequest(server.handleDietaryPreferences, "PATCH", "/api/preferences",
		map[string]interface{}{
			"allergens": []string{"peanuts"},
			"diets":     []string{"keto"},
		},
		[]*http.Cookie{cookie})

	// Partial update: only update allergens, leave diets unchanged
	rr := doRequest(server.handleDietaryPreferences, "PATCH", "/api/preferences",
		map[string]interface{}{
			"allergens": []string{"peanuts", "gluten"},
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH status: got %d, want 200", rr.Code)
	}

	// GET to verify diets are preserved
	rr = doRequest(server.handleDietaryPreferences, "GET", "/api/preferences", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)

	allergens := body["allergens"].([]interface{})
	if len(allergens) != 2 {
		t.Fatalf("allergens: got %d items, want 2", len(allergens))
	}
	if allergens[0] != "peanuts" || allergens[1] != "gluten" {
		t.Errorf("allergens: got %v, want [peanuts gluten]", allergens)
	}

	diets := body["diets"].([]interface{})
	if len(diets) != 1 || diets[0] != "keto" {
		t.Errorf("diets: got %v, want [keto] (should be preserved from initial set)", diets)
	}
}

func TestDietaryProfile_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Send invalid JSON body
	req := httptest.NewRequest("PATCH", "/api/preferences", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleDietaryPreferences(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestGetHouseholdAllergens_Union(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create 2 users in the same household
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-100', '+15551111111', 'Alice', 1)")
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-101', '+15552222222', 'Bob', 1)")
	server.db.Exec("INSERT INTO households (id, name) VALUES (1, 'Test Family')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-100')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-101')")

	// Alice has peanut allergy, Bob has dairy allergy
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ('user-100', '["peanuts"]', '[]')`)
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ('user-101', '["dairy"]', '[]')`)

	// Alice's household allergens should include both peanuts and dairy
	allergens := server.getHouseholdAllergens("user-100")
	if len(allergens) != 2 {
		t.Fatalf("Expected 2 household allergens, got %d: %v", len(allergens), allergens)
	}
	has := map[string]bool{}
	for _, a := range allergens {
		has[a] = true
	}
	if !has["peanuts"] || !has["dairy"] {
		t.Errorf("Expected peanuts and dairy, got %v", allergens)
	}
}

func TestGetHouseholdAllergens_NoHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-100', '+15551234567')")
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ('user-100', '["gluten"]', '[]')`)

	allergens := server.getHouseholdAllergens("user-100")
	if len(allergens) != 1 || allergens[0] != "gluten" {
		t.Errorf("Expected [gluten] for solo user, got %v", allergens)
	}
}

func TestHouseholdDietary_Endpoint(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create household
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-100', '+15551111111', 'Alice', 1)")
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-101', '+15552222222', 'Bob', 1)")
	server.db.Exec("INSERT INTO households (id, name) VALUES (1, 'Family')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-100')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-101')")
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ('user-100', '["peanuts"]', '["vegan"]')`)

	token, _ := server.sessions.CreateSession("user-100", "+15551111111", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleHouseholdDietary, "GET", "/api/household/dietary", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["household"] != true {
		t.Error("Expected household: true")
	}
	allergens := body["all_allergens"].([]interface{})
	if len(allergens) != 1 || allergens[0] != "peanuts" {
		t.Errorf("Expected [peanuts], got %v", allergens)
	}
}

func TestHouseholdDietary_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleHouseholdDietary, "GET", "/api/household/dietary", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestHouseholdDuplicateDetection(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create 2 users in the same household
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-100', '+15551111111', 'Alice', 1)")
	server.db.Exec("INSERT INTO users (id, phone, name, household_id) VALUES ('user-101', '+15552222222', 'Bob', 1)")
	server.db.Exec("INSERT INTO households (id, name) VALUES (1, 'Test Family')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-100')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (1, 'user-101')")

	// Alice adds milk
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, 'user-100', 'Milk', 0)")
	server.db.Exec("INSERT INTO item_modifications (item_id, user_id, action, source, created_at) VALUES (1, 'user-100', 'added', 'app', datetime('now', '-1 hour'))")

	// Bob adds milk — should trigger duplicate warning
	bobItems := []ListItem{{ID: 2, Name: "Milk", Brand: "", Department: "Dairy & Eggs"}}
	warnings := server.checkHouseholdDuplicates("user-101", bobItems)
	if len(warnings) != 1 {
		t.Fatalf("Expected 1 duplicate warning, got %d", len(warnings))
	}
	if warnings[0].AddedBy != "Alice" {
		t.Errorf("Expected added_by 'Alice', got '%s'", warnings[0].AddedBy)
	}

	// No warning for non-household user
	server.db.Exec("INSERT INTO users (id, phone, name) VALUES ('user-102', '+15553333333', 'Charlie')")
	noWarnings := server.checkHouseholdDuplicates("user-102", bobItems)
	if len(noWarnings) != 0 {
		t.Errorf("Expected no warnings for non-household user, got %d", len(noWarnings))
	}
}

// =============================================================================
// SHOPPING ROLE DETECTION TESTS (L2 Household Learning)
// =============================================================================

func TestShoppingRole_PrimaryShopper(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a household with two users
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-100', '+15551111111', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-101', '+15552222222', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-100', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-101', 'member')")

	// User 100: 20 items, 15 checked — check_ratio = 0.75 (> 0.5), total > median => primary_shopper
	for i := 1; i <= 20; i++ {
		checked := 0
		if i <= 15 {
			checked = 1
		}
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-100', ?, ?)",
			fmt.Sprintf("Item %d", i), checked)
	}

	// User 101: 3 items, 0 checked — total <= 5 => occasional
	for i := 1; i <= 3; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-101', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var role100 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-100' AND key = 'shopping_role'").Scan(&role100)
	if role100 != "primary_shopper" {
		t.Errorf("User 100 role: got %q, want 'primary_shopper'", role100)
	}

	var role101 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-101' AND key = 'shopping_role'").Scan(&role101)
	if role101 != "occasional" {
		t.Errorf("User 101 role: got %q, want 'occasional'", role101)
	}
}

func TestShoppingRole_ListBuilder(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a household with two users
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-200', '+15553333333', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-201', '+15554444444', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-200', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-201', 'member')")

	// User 200: 15 items, 1 checked — check_ratio = 0.067 (< 0.3), total > 5 => list_builder
	for i := 1; i <= 15; i++ {
		checked := 0
		if i == 1 {
			checked = 1
		}
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-200', ?, ?)",
			fmt.Sprintf("Item %d", i), checked)
	}

	// User 201: 10 items, 0 checked — check_ratio = 0.0 (< 0.3), total > 5
	for i := 1; i <= 10; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-201', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var role200 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-200' AND key = 'shopping_role'").Scan(&role200)
	if role200 != "list_builder" {
		t.Errorf("User 200 role: got %q, want 'list_builder'", role200)
	}

	var role201 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-201' AND key = 'shopping_role'").Scan(&role201)
	if role201 != "list_builder" {
		t.Errorf("User 201 role: got %q, want 'list_builder'", role201)
	}
}

func TestShoppingRole_Occasional(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a household with two users
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-300', '+15555555555', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-301', '+15556666666', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-300', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-301', 'member')")

	// User 300: 5 items, 3 checked — total <= 5 => occasional
	for i := 1; i <= 5; i++ {
		checked := 0
		if i <= 3 {
			checked = 1
		}
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-300', ?, ?)",
			fmt.Sprintf("Item %d", i), checked)
	}

	// User 301: 2 items, 0 checked — total <= 5 => occasional
	for i := 1; i <= 2; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-301', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var role300 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-300' AND key = 'shopping_role'").Scan(&role300)
	if role300 != "occasional" {
		t.Errorf("User 300 role: got %q, want 'occasional'", role300)
	}

	var role301 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-301' AND key = 'shopping_role'").Scan(&role301)
	if role301 != "occasional" {
		t.Errorf("User 301 role: got %q, want 'occasional'", role301)
	}
}

func TestShoppingRole_Confidence(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create household
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-400', '+15557777777', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-401', '+15558888888', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-400', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-401', 'member')")

	// User 400: 4 items => confidence = 4/20 = 0.2
	for i := 1; i <= 4; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-400', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}
	// User 401: 30 items => confidence = 30/20 = 1.5 clamped to 1.0
	for i := 1; i <= 30; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-401', ?, 1)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var conf400 float64
	server.db.QueryRow("SELECT confidence FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-400' AND key = 'shopping_role'").Scan(&conf400)
	if conf400 < 0.15 || conf400 > 0.25 {
		t.Errorf("User 400 confidence: got %f, want ~0.2", conf400)
	}

	var conf401 float64
	server.db.QueryRow("SELECT confidence FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-401' AND key = 'shopping_role'").Scan(&conf401)
	if conf401 != 1.0 {
		t.Errorf("User 401 confidence: got %f, want 1.0", conf401)
	}
}

func TestShoppingRole_NoHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// User without household — should NOT get a shopping role
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-500', '+15559999999')")
	for i := 1; i <= 10; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-500', ?, 1)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var role string
	err := server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-500' AND key = 'shopping_role'").Scan(&role)
	if err == nil && role != "" {
		t.Errorf("User without household should not have shopping_role, got %q", role)
	}
}

func TestShoppingRole_InLearningContext(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create user with autotune enabled
	server.db.Exec("INSERT INTO users (id, phone, penny_autotune, household_id) VALUES ('user-600', '+15550001111', 1, 1)")
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-600', 'owner')")

	// Set the shopping_role learning directly
	server.updateLearning("user", "user-600", "shopping_role", "primary_shopper", 0.8)

	ctx := server.buildLearningContext("user-600")
	if !strings.Contains(ctx, "primary shopper") {
		t.Errorf("Expected 'primary shopper' in learning context, got: %s", ctx)
	}
	if !strings.Contains(ctx, "adds and checks off most items") {
		t.Errorf("Expected role description in learning context, got: %s", ctx)
	}
}

func TestShoppingRole_ListBuilderInContext(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO users (id, phone, penny_autotune, household_id) VALUES ('user-700', '+15550002222', 1, 1)")
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-700', 'owner')")

	server.updateLearning("user", "user-700", "shopping_role", "list_builder", 0.6)

	ctx := server.buildLearningContext("user-700")
	if !strings.Contains(ctx, "list builder") {
		t.Errorf("Expected 'list builder' in learning context, got: %s", ctx)
	}
}

func TestShoppingRole_OccasionalInContext(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO users (id, phone, penny_autotune, household_id) VALUES ('user-800', '+15550003333', 1, 1)")
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-800', 'owner')")

	server.updateLearning("user", "user-800", "shopping_role", "occasional", 0.4)

	ctx := server.buildLearningContext("user-800")
	if !strings.Contains(ctx, "occasional contributor") {
		t.Errorf("Expected 'occasional contributor' in learning context, got: %s", ctx)
	}
}

func TestShoppingRole_ThreePersonHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a 3-person household
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-900', '+15550004444', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-901', '+15550005555', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-902', '+15550006666', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-900', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-901', 'member')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-902', 'member')")

	// User 900: 30 items, 25 checked — heavy shopper, high check ratio => primary_shopper
	for i := 1; i <= 30; i++ {
		checked := 0
		if i <= 25 {
			checked = 1
		}
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-900', ?, ?)",
			fmt.Sprintf("Item %d", i), checked)
	}

	// User 901: 15 items, 1 checked — above median, low check ratio => list_builder
	for i := 1; i <= 15; i++ {
		checked := 0
		if i == 1 {
			checked = 1
		}
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-901', ?, ?)",
			fmt.Sprintf("Item %d", i), checked)
	}

	// User 902: 3 items, 0 checked — <= 5 => occasional
	for i := 1; i <= 3; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-902', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}

	server.processLearnings()

	var role900, role901, role902 string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-900' AND key = 'shopping_role'").Scan(&role900)
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-901' AND key = 'shopping_role'").Scan(&role901)
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-902' AND key = 'shopping_role'").Scan(&role902)

	if role900 != "primary_shopper" {
		t.Errorf("User 900 (heavy shopper): got %q, want 'primary_shopper'", role900)
	}
	if role901 != "list_builder" {
		t.Errorf("User 901 (adds but doesn't check): got %q, want 'list_builder'", role901)
	}
	if role902 != "occasional" {
		t.Errorf("User 902 (few items): got %q, want 'occasional'", role902)
	}
}

func TestShoppingRole_Idempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-1000', '+15550007777', 1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id) VALUES ('user-1001', '+15550008888', 1)")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-1000', 'owner')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, 'user-1001', 'member')")

	for i := 1; i <= 10; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-1000', ?, 1)",
			fmt.Sprintf("Item %d", i))
	}
	for i := 1; i <= 2; i++ {
		server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES ('user-1001', ?, 0)",
			fmt.Sprintf("Item %d", i))
	}

	// Run twice — should produce same result (upsert)
	server.processLearnings()
	server.processLearnings()

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM penny_learnings WHERE scope = 'user' AND scope_id = 'user-1000' AND key = 'shopping_role'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected exactly 1 shopping_role row for user 1000, got %d", count)
	}
}

// =============================================================================
// REPURCHASE NUDGE TESTS (L2 Household Learning)
// =============================================================================

func TestSettingsUpdate_SetsNameSourceManual(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "+15550001234")

	// Verify initial name_source is 'auto'
	var src string
	server.db.QueryRow("SELECT COALESCE(name_source,'auto') FROM users WHERE id = ?", userID).Scan(&src)
	if src != "auto" {
		t.Errorf("expected initial name_source=auto, got %s", src)
	}

	// Update name via settings
	rr := doRequest(server.handleSettings, "PATCH", "/api/settings",
		map[string]string{"name": "Manually Set"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// name_source should now be 'manual'
	server.db.QueryRow("SELECT name_source FROM users WHERE id = ?", userID).Scan(&src)
	if src != "manual" {
		t.Errorf("expected name_source=manual after settings update, got %s", src)
	}
}

// =============================================================================
// CROSS-USER SECURITY TESTS
// =============================================================================

func TestCrossUser_CannotToggleOtherUsersItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550001000")
	cookieB, _ := loginUser(t, server, "+15550002000")

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity, checked) VALUES (?, 'Secret Item', 1, 0)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'Secret Item'", userA).Scan(&itemID)

	// User B tries to toggle it via handleList (router)
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID), nil, []*http.Cookie{cookieB})

	var checked bool
	server.db.QueryRow("SELECT checked FROM list_items WHERE id = ?", itemID).Scan(&checked)
	if checked {
		t.Errorf("user B was able to toggle user A's item — cross-user access violation (status %d)", rr.Code)
	}
}

func TestCrossUser_CannotDeleteOtherUsersItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550003000")
	cookieB, _ := loginUser(t, server, "+15550004000")

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Private Eggs', 1)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'Private Eggs'", userA).Scan(&itemID)

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID), nil, []*http.Cookie{cookieB})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 1 {
		t.Errorf("user B was able to delete user A's item — cross-user access violation (status %d)", rr.Code)
	}
}

func TestCrossUser_CannotEditOtherUsersItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550005000")
	cookieB, _ := loginUser(t, server, "+15550006000")

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'Original Name', 1)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'Original Name'", userA).Scan(&itemID)

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]string{"name": "Hacked Name"}, []*http.Cookie{cookieB})

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE id = ?", itemID).Scan(&name)
	if name != "Original Name" {
		t.Errorf("user B was able to edit user A's item to '%s' — cross-user access violation (status %d)", name, rr.Code)
	}
}

func TestCrossUser_CannotClaimOtherUsersItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550007000")
	cookieB, _ := loginUser(t, server, "+15550008000")

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity) VALUES (?, 'My Milk', 1)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'My Milk'", userA).Scan(&itemID)

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookieB})

	var claimedBy sql.NullInt64
	server.db.QueryRow("SELECT claimed_by FROM list_items WHERE id = ?", itemID).Scan(&claimedBy)
	if claimedBy.Valid {
		t.Errorf("user B was able to claim user A's item — cross-user access violation (claimed_by=%d, status %d)", claimedBy.Int64, rr.Code)
	}
}

func TestCrossUser_CannotMarkOtherUsersItemUrgent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550009000")
	cookieB, _ := loginUser(t, server, "+15550010000")

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity, urgent) VALUES (?, 'Calm Item', 1, 0)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'Calm Item'", userA).Scan(&itemID)

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID), nil, []*http.Cookie{cookieB})

	var urgent bool
	server.db.QueryRow("SELECT urgent FROM list_items WHERE id = ?", itemID).Scan(&urgent)
	if urgent {
		t.Errorf("user B was able to mark user A's item urgent — cross-user access violation (status %d)", rr.Code)
	}
}

func TestCrossUser_HouseholdMembersCanSeeSharedItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookieA, userA := loginUser(t, server, "+15550011000")
	cookieB, userB := loginUser(t, server, "+15550012000")

	// Put both users in the same household
	server.db.Exec("UPDATE users SET household_id = (SELECT household_id FROM users WHERE id = ?) WHERE id = ?", userA, userB)
	server.db.Exec("INSERT OR IGNORE INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = ?), ?)", userA, userB)

	// User A adds an item
	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Shared Milk"}, []*http.Cookie{cookieA})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("user A add failed: %d %s", rr.Code, rr.Body.String())
	}

	// User B should be able to see it
	rr = doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("user B list fetch failed: %d", rr.Code)
	}

	var listItems []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &listItems)
	found := false
	for _, item := range listItems {
		if item["name"] == "Shared Milk" {
			found = true
			break
		}
	}
	if !found {
		t.Error("household member B could not see item added by member A")
	}
}

func TestCrossUser_HouseholdMemberCanToggleSharedItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550013000")
	cookieB, userB := loginUser(t, server, "+15550014000")

	// Same household
	server.db.Exec("UPDATE users SET household_id = (SELECT household_id FROM users WHERE id = ?) WHERE id = ?", userA, userB)
	server.db.Exec("INSERT OR IGNORE INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = ?), ?)", userA, userB)

	server.db.Exec("INSERT INTO list_items (user_id, name, quantity, checked) VALUES (?, 'HH Bread', 1, 0)", userA)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'HH Bread'", userA).Scan(&itemID)

	// User B should be able to toggle it (they're in the same household)
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID), nil, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("household member could not toggle shared item: %d %s", rr.Code, rr.Body.String())
	}

	var checked bool
	server.db.QueryRow("SELECT checked FROM list_items WHERE id = ?", itemID).Scan(&checked)
	if !checked {
		t.Error("household member's toggle did not take effect on shared item")
	}
}

// =============================================================================
// Receipt: Swap Purchase Verification
// =============================================================================
// Places Endpoint Tests
// =============================================================================

func TestPlacesAutocomplete_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=123+Main", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session, got %d", rr.Code)
	}
}

func TestPlacesAutocomplete_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePlacesAutocomplete, "POST", "/api/places/autocomplete", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

func TestPlacesAutocomplete_ShortInput(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Input shorter than 3 chars returns empty array
	rr := doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=ab", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for short input, got %d", rr.Code)
	}
	if rr.Body.String() != "[]\n" {
		t.Errorf("expected empty array, got %s", rr.Body.String())
	}
}

func TestPlacesAutocomplete_EmptyInput(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for empty input, got %d", rr.Code)
	}
}

func TestPlacesDetail_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=abc", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session, got %d", rr.Code)
	}
}

func TestPlacesDetail_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePlacesDetail, "POST", "/api/places/detail", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

func TestPlacesDetail_MissingPlaceID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing place_id, got %d", rr.Code)
	}
}

// =============================================================================
// A/B Testing Endpoint Tests
// =============================================================================


// =============================================================================
// handleLeaveHousehold ADDITIONAL TESTS (56.5% → target ~75%)
// =============================================================================

func TestLeaveHousehold_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5553330001")

	rr := doRequest(server.handleLeaveHousehold, "GET", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", rr.Code)
	}
}

func TestLeaveHousehold_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestLeaveHousehold_VerifiesNewHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5553330002")

	// Get current household ID
	var origHouseholdID int
	server.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&origHouseholdID)

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify user has a new household
	var newHouseholdID int
	server.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&newHouseholdID)
	if newHouseholdID == 0 {
		t.Error("User should have a new household after leaving")
	}
	if newHouseholdID == origHouseholdID && origHouseholdID != 0 {
		t.Error("User should have a DIFFERENT household after leaving")
	}

	// Verify user is owner of the new household
	var role string
	server.db.QueryRow("SELECT role FROM household_members WHERE user_id = ? AND household_id = ?", userID, newHouseholdID).Scan(&role)
	if role != "owner" {
		t.Errorf("User should be owner of new household, got role %q", role)
	}
}

// =============================================================================
// handleAvatarUpload TESTS
// =============================================================================

func TestAvatarUpload_Success_JPEG(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	jpegData := makeJPEG(100)
	b64 := base64.StdEncoding.EncodeToString(jpegData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{
		"photo": b64,
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	if success, _ := resp["success"].(bool); !success {
		t.Error("Expected success=true")
	}
}

func TestAvatarUpload_Success_PNG(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	pngData := makePNG(100)
	b64 := base64.StdEncoding.EncodeToString(pngData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{
		"photo": b64,
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestAvatarUpload_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{
		"photo": "dGVzdA==",
	}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestAvatarUpload_EmptyPhoto(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{
		"photo": "",
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestAvatarUpload_InvalidFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// GIF-like data (not JPEG or PNG)
	gifData := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00}
	b64 := base64.StdEncoding.EncodeToString(gifData)

	rr := doRequest(server.handleAvatar, "POST", "/api/avatar", map[string]string{
		"photo": b64,
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// handleAvatarGet TESTS
// =============================================================================

func TestAvatarGet_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleAvatarGet, "GET", "/api/avatar/99999", nil, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestAvatarGet_EmptyID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleAvatarGet, "GET", "/api/avatar/", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// handleAvatarDelete TESTS
// =============================================================================

func TestAvatarDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert a photo first
	server.db.Exec("INSERT INTO profile_photos (user_id, photo_data, content_type) VALUES (?, ?, ?)",
		userID, makeJPEG(50), "image/jpeg")

	rr := doRequest(server.handleAvatar, "DELETE", "/api/avatar", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify photo is deleted
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM profile_photos WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Error("Expected photo to be deleted")
	}
}

func TestAvatarDelete_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleAvatar, "DELETE", "/api/avatar", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestAvatar_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAvatar, "PUT", "/api/avatar", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}
