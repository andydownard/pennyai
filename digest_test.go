package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// DAILY DIGEST TESTS
// =============================================================================

func TestBuildDigestMessage_SingleItem(t *testing.T) {
	msg := buildDigestMessage("Alice Smith", []string{"milk"})
	if !strings.Contains(msg, "Hey Alice") {
		t.Errorf("Expected 'Hey Alice' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "1 item was") {
		t.Errorf("Expected '1 item was' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "milk") {
		t.Errorf("Expected 'milk' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "https://app.thrift.ly") {
		t.Errorf("Expected link in message, got: %s", msg)
	}
}

func TestBuildDigestMessage_MultipleItems(t *testing.T) {
	msg := buildDigestMessage("Bob", []string{"milk", "eggs", "bread"})
	if !strings.Contains(msg, "Hey Bob") {
		t.Errorf("Expected 'Hey Bob' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "3 items were") {
		t.Errorf("Expected '3 items were' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "milk, eggs, bread") {
		t.Errorf("Expected item list in message, got: %s", msg)
	}
}

func TestBuildDigestMessage_TruncatesAfter5(t *testing.T) {
	items := []string{"milk", "eggs", "bread", "butter", "cheese", "yogurt", "apples"}
	msg := buildDigestMessage("Carol", items)
	if !strings.Contains(msg, "7 items were") {
		t.Errorf("Expected '7 items were' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "and 2 more") {
		t.Errorf("Expected 'and 2 more' in message, got: %s", msg)
	}
	// Should NOT contain items beyond first 5
	if strings.Contains(msg, "yogurt") || strings.Contains(msg, "apples") {
		t.Errorf("Should not contain items beyond first 5, got: %s", msg)
	}
}

func TestBuildDigestMessage_EmptyName(t *testing.T) {
	msg := buildDigestMessage("", []string{"milk"})
	if !strings.HasPrefix(msg, "Hey,") {
		t.Errorf("Expected message to start with 'Hey,' for empty name, got: %s", msg)
	}
}

func TestProcessDailyDigest_OptsOutSkips(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create user with phone, NOT opted in (default)
	_, err := server.db.Exec("INSERT INTO users (id, phone_number, name) VALUES (1, '+15551234567', 'Test')")
	if err != nil {
		t.Fatal(err)
	}

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'milk')")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send digest when user has not opted in")
	}
}

func TestProcessDailyDigest_OptedInSends(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create user with phone AND opted in
	_, err := server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Alice', 1)")
	if err != nil {
		t.Fatal(err)
	}

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'milk')")

	server.processDailyDigest()

	smsTo, smsBody := mock.GetLastSMS()
	if smsTo != "+15551234567" {
		t.Errorf("Expected SMS to +15551234567, got %q", smsTo)
	}
	if !strings.Contains(smsBody, "milk") {
		t.Errorf("Expected 'milk' in SMS body, got: %s", smsBody)
	}
	if !strings.Contains(smsBody, "Hey Alice") {
		t.Errorf("Expected 'Hey Alice' in SMS body, got: %s", smsBody)
	}
}

func TestProcessDailyDigest_NoItems_NoSMS(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Opted-in user with phone but no list items
	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Bob', 1)")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send digest when there are no list items")
	}
}

func TestProcessDailyDigest_DeduplicatesWithin20Hours(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create opted-in user whose last digest was 10 hours ago
	tenHoursAgo := time.Now().UTC().Add(-10 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled, daily_digest_last_sent) VALUES (1, '+15551234567', 'Carol', 1, ?)", tenHoursAgo)

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'eggs')")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send digest within 20-hour dedup window")
	}
}

func TestProcessDailyDigest_SendsAfter20Hours(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create opted-in user whose last digest was 25 hours ago
	twentyFiveHoursAgo := time.Now().UTC().Add(-25 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled, daily_digest_last_sent) VALUES (1, '+15551234567', 'Dave', 1, ?)", twentyFiveHoursAgo)

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'bread')")

	server.processDailyDigest()

	smsTo, smsBody := mock.GetLastSMS()
	if smsTo != "+15551234567" {
		t.Errorf("Expected SMS to be sent after 25 hours, got to=%q", smsTo)
	}
	if !strings.Contains(smsBody, "bread") {
		t.Errorf("Expected 'bread' in message, got: %s", smsBody)
	}
}

func TestProcessDailyDigest_NoPhoneSkips(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Opted-in user WITHOUT phone (email-only)
	server.db.Exec("INSERT INTO users (id, name, daily_digest_enabled, email) VALUES (1, 'Eve', 1, 'eve@test.com')")

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'juice')")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send digest to user without phone number")
	}
}

func TestProcessDailyDigest_UpdatesLastSent(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Frank', 1)")
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'milk')")

	server.processDailyDigest()

	// Verify last_sent was updated
	var lastSent string
	server.db.QueryRow("SELECT COALESCE(daily_digest_last_sent, '') FROM users WHERE id = 1").Scan(&lastSent)
	if lastSent == "" {
		t.Error("daily_digest_last_sent should be updated after sending")
	}
}

func TestProcessDailyDigest_NoTwilioPhone_Skips(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.config.TwilioPhoneNumber = ""

	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Grace', 1)")
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'milk')")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send when TWILIO_PHONE_NUMBER is not configured")
	}
}

func TestProcessDailyDigest_CheckedItemsExcluded(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Hank', 1)")
	// Only add checked items
	server.db.Exec("INSERT INTO list_items (user_id, name, checked) VALUES (1, 'milk', 1)")

	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("Should NOT send digest when all items are checked off")
	}
}

func TestSettingsDailyDigestEnabled_GetAndPatch(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie, userID := loginUser(t, server, "5551234567")

	// GET should default to false
	data := digestGetSettings(t, server, cookie)
	if data["daily_digest_enabled"] != false {
		t.Errorf("Expected daily_digest_enabled=false by default, got: %v", data["daily_digest_enabled"])
	}

	// PATCH to enable
	digestPatchSettings(t, server, cookie, `{"daily_digest_enabled": true}`)

	// GET should now return true
	data = digestGetSettings(t, server, cookie)
	if data["daily_digest_enabled"] != true {
		t.Errorf("Expected daily_digest_enabled=true after enable, got: %v", data["daily_digest_enabled"])
	}

	// Verify opted_in_at was set
	var optedInAt string
	server.db.QueryRow("SELECT COALESCE(daily_digest_opted_in_at, '') FROM users WHERE id = ?", userID).Scan(&optedInAt)
	if optedInAt == "" {
		t.Error("Expected daily_digest_opted_in_at to be set on first enable")
	}

	// PATCH to disable
	digestPatchSettings(t, server, cookie, `{"daily_digest_enabled": false}`)

	data = digestGetSettings(t, server, cookie)
	if data["daily_digest_enabled"] != false {
		t.Errorf("Expected daily_digest_enabled=false after disable, got: %v", data["daily_digest_enabled"])
	}
}

func TestSettingsDailyDigestEnabled_AfterOptOut_DoesNotSend(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	cookie, _ := loginUser(t, server, "5551234567")

	// Enable
	digestPatchSettings(t, server, cookie, `{"daily_digest_enabled": true}`)

	// Add items using the known userID
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES ((SELECT id FROM users WHERE phone_number = '5551234567'), 'milk')")

	// Disable (opt-out)
	digestPatchSettings(t, server, cookie, `{"daily_digest_enabled": false}`)

	// Clear any SMS from login flow
	mock.mu.Lock()
	mock.LastSMSTo = ""
	mock.LastSMSBody = ""
	mock.mu.Unlock()

	// Run digest
	server.processDailyDigest()

	smsTo, _ := mock.GetLastSMS()
	if smsTo != "" {
		t.Error("After opt-out, digest should NOT send")
	}
}

func TestProcessDailyDigest_RapidCallsDoNotDuplicate(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Create opted-in user with no prior digest (NULL last_sent)
	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Rapid', 1)")
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'milk')")

	// Call processDailyDigest twice in rapid succession
	server.processDailyDigest()
	countAfterFirst := mock.GetSMSCount()
	if countAfterFirst != 1 {
		t.Fatalf("Expected 1 SMS after first call, got %d", countAfterFirst)
	}

	server.processDailyDigest()
	countAfterSecond := mock.GetSMSCount()
	if countAfterSecond != 1 {
		t.Errorf("Rapid second call should NOT send duplicate SMS (sent %d total, expected 1)", countAfterSecond)
	}
}

func TestProcessDailyDigest_DedupFlagSetBeforeSMS(t *testing.T) {
	// Verify that even if SMS fails, the dedup flag was already set
	// (miss one digest is better than risk duplicate)
	mock := &MockTwilioClient{SMSErr: fmt.Errorf("twilio error")}
	server := newTestServer(t, mock)

	server.db.Exec("INSERT INTO users (id, phone_number, name, daily_digest_enabled) VALUES (1, '+15551234567', 'Guard', 1)")
	server.db.Exec("INSERT INTO list_items (user_id, name) VALUES (1, 'eggs')")

	server.processDailyDigest()

	// Dedup flag should still be set even though SMS failed
	var lastSent string
	server.db.QueryRow("SELECT COALESCE(daily_digest_last_sent, '') FROM users WHERE id = 1").Scan(&lastSent)
	if lastSent == "" {
		t.Error("daily_digest_last_sent should be set BEFORE SMS attempt (dedup-first pattern)")
	}
}

// digestGetSettings is a test helper that GETs /api/settings and returns the JSON map.
func digestGetSettings(t *testing.T, server *Server, cookie *http.Cookie) map[string]interface{} {
	t.Helper()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleSettings(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /api/settings returned %d", rr.Code)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("Failed to parse settings JSON: %v", err)
	}
	return data
}

// digestPatchSettings is a test helper that PATCHes /api/settings with the given JSON body.
func digestPatchSettings(t *testing.T, server *Server, cookie *http.Cookie, body string) {
	t.Helper()
	req, _ := http.NewRequest("PATCH", "/api/settings", strings.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleSettings(rr, req)
	if rr.Code != 200 {
		t.Fatalf("PATCH /api/settings returned %d: %s", rr.Code, rr.Body.String())
	}
}


// --- BrandDigest ---

func TestSendPendingBrandDigests_NoBrandUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// Should not panic with no brand users
	server.sendPendingBrandDigests()
}

func TestSendPendingBrandDigests_UserWithNoDeals(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	brandRegister(t, server, "digest@brand.com", "password1234", "DigestCo")
	// Should not panic or error — just no digests to send
	server.sendPendingBrandDigests()
}
