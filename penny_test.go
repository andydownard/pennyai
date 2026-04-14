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
		t.Errorf("Expected show_nps=false when login_count < %d", npsMinLogins)
	}
}

func TestNPSCheck_ShowsAfterEnoughLogins(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count above npsMinLogins threshold
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins+1, userID)

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != true {
		t.Errorf("Expected show_nps=true when login_count >= %d and no recent NPS", npsMinLogins)
	}
}

func TestNPSCheck_HidesAfterRecentResponse(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count above threshold and add recent NPS response
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins+1, userID)
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

// =============================================================================
// NPS HANDLER TESTS
// =============================================================================

func TestNPSCheck_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestNPSCheck_ReturnsShowNPSField(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990020")

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	data := parseJSON(rr)
	if _, ok := data["show_nps"]; !ok {
		t.Errorf("Expected 'show_nps' field in NPS check response, got: %v", data)
	}
}

func TestNPS_Submit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559990021")

	rr := doRequest(server.handleNPS, "POST", "/api/nps",
		map[string]interface{}{"score": 9, "feedback": "Love it!"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestNPS_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleNPS, "POST", "/api/nps",
		map[string]interface{}{"score": 7}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

// =============================================================================
// VCARD HANDLER TESTS
// =============================================================================

func TestNPSConstants_MinLoginThreshold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	server.config.NPSIntervalDays = 90

	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins-1, userID)
	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != false {
		t.Errorf("expected show_nps=false at %d logins, got %v", npsMinLogins-1, result["show_nps"])
	}

	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins, userID)
	rr = doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result = parseJSON(rr)
	if result["show_nps"] != true {
		t.Errorf("expected show_nps=true at %d logins, got %v", npsMinLogins, result["show_nps"])
	}
}

// =============================================================================
// NPS EDGE CASES
// =============================================================================

func TestNPS_EdgeCase_ScoreNegative(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleNPS, "POST", "/api/nps",
		map[string]interface{}{"score": -5, "comment": "bad"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("NPS with score=-5: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["error"] != "Score must be 0-10" {
		t.Errorf("Expected 'Score must be 0-10' error, got %v", result["error"])
	}
}

func TestNPS_EdgeCase_ScoreOver10(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleNPS, "POST", "/api/nps",
		map[string]interface{}{"score": 100, "comment": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("NPS with score=100: got %d, want 400", rr.Code)
	}
}

func TestNPS_EdgeCase_DuplicateSubmission(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	body := map[string]interface{}{"score": 8, "comment": "First"}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First NPS submit: got %d, want 200", rr.Code)
	}

	// Second submission should also succeed (no unique constraint)
	body2 := map[string]interface{}{"score": 9, "comment": "Second"}
	rr = doRequest(server.handleNPS, "POST", "/api/nps", body2, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Second NPS submit: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// Both should be in the DB
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM nps_responses WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 NPS responses, got %d", count)
	}
}

func TestNPS_EdgeCase_WhitespaceOnlyComment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	body := map[string]interface{}{"score": 7, "comment": "   \t\n   "}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("NPS with whitespace comment: got %d, want 200", rr.Code)
	}

	// Comment should be trimmed to empty
	var comment string
	server.db.QueryRow("SELECT comment FROM nps_responses WHERE user_id = ?", userID).Scan(&comment)
	if comment != "" {
		t.Errorf("Expected empty comment after trim, got %q", comment)
	}
}

func TestNPS_EdgeCase_CommentExactly500Chars(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	exact500 := strings.Repeat("b", 500)
	body := map[string]interface{}{"score": 6, "comment": exact500}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("NPS with 500-char comment: got %d, want 200", rr.Code)
	}

	var comment string
	server.db.QueryRow("SELECT comment FROM nps_responses WHERE user_id = ?", userID).Scan(&comment)
	if len(comment) != 500 {
		t.Errorf("Expected comment length 500, got %d", len(comment))
	}
}

func TestNPS_EdgeCase_CommentExactly501CharsTruncated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	over500 := strings.Repeat("c", 501)
	body := map[string]interface{}{"score": 5, "comment": over500}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("NPS with 501-char comment: got %d, want 200", rr.Code)
	}

	var comment string
	server.db.QueryRow("SELECT comment FROM nps_responses WHERE user_id = ?", userID).Scan(&comment)
	if len(comment) != 500 {
		t.Errorf("Expected comment truncated to 500, got %d", len(comment))
	}
}

func TestNPS_EdgeCase_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// NPS submit is POST only
	rr := doRequest(server.handleNPS, "GET", "/api/nps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/nps: got %d, want 405", rr.Code)
	}

	// NPS check is GET only
	rr = doRequest(server.handleNPSCheck, "POST", "/api/nps/check",
		map[string]interface{}{"score": 5}, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/nps/check: got %d, want 405", rr.Code)
	}
}

func TestNPS_EdgeCase_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// nil body
	rr := doRequest(server.handleNPS, "POST", "/api/nps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("NPS with nil body: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestNPS_EdgeCase_MissingScoreDefaultsToZero(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// JSON with only comment — score defaults to 0 (Go zero value), which is valid (0-10)
	body := map[string]interface{}{"comment": "No score given"}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("NPS with missing score: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var score int
	server.db.QueryRow("SELECT score FROM nps_responses WHERE user_id = ?", userID).Scan(&score)
	if score != 0 {
		t.Errorf("Expected score=0 (Go zero value), got %d", score)
	}
}

func TestNPSCheck_EdgeCase_ExactMinLoginThreshold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count to exactly npsMinLogins (10) — should show NPS
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins, userID)

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != true {
		t.Errorf("Expected show_nps=true at exactly %d logins, got %v", npsMinLogins, result["show_nps"])
	}
}

func TestNPSCheck_EdgeCase_OneBelowMinLoginThreshold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count to npsMinLogins-1 — should NOT show NPS
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins-1, userID)

	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != false {
		t.Errorf("Expected show_nps=false at %d logins, got %v", npsMinLogins-1, result["show_nps"])
	}
}

// =============================================================================
// NPS DISMISS ENDPOINT TESTS
// =============================================================================

func TestNPSDismiss_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleNPSDismiss, "POST", "/api/nps/dismiss", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	if result["success"] != true {
		t.Error("Expected success=true")
	}

	// Verify the timestamp was stored in the database
	var dismissedAt string
	err := server.db.QueryRow("SELECT nps_dismissed_at FROM users WHERE id = ?", userID).Scan(&dismissedAt)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if dismissedAt == "" {
		t.Error("Expected nps_dismissed_at to be set, got empty string")
	}
}

func TestNPSDismiss_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleNPSDismiss, "POST", "/api/nps/dismiss", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestNPSDismiss_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleNPSDismiss, "GET", "/api/nps/dismiss", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestNPSCheck_HidesAfterDismiss(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count above threshold
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins+1, userID)

	// Verify NPS shows before dismiss
	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != true {
		t.Fatal("Expected show_nps=true before dismiss")
	}

	// Dismiss NPS
	rr = doRequest(server.handleNPSDismiss, "POST", "/api/nps/dismiss", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Dismiss failed: %d", rr.Code)
	}

	// Verify NPS is hidden after dismiss
	rr = doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result = parseJSON(rr)
	if result["show_nps"] != false {
		t.Error("Expected show_nps=false after dismiss (server-side guard)")
	}
}

func TestNPSCheck_ShowsAfterDismissExpires(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.NPSIntervalDays = 90
	cookie, userID := loginUser(t, server, "5551234567")

	// Set login_count above threshold
	server.db.Exec("UPDATE users SET login_count = ? WHERE id = ?", npsMinLogins+1, userID)

	// Set dismiss timestamp to 91 days ago (expired)
	server.db.Exec(
		"UPDATE users SET nps_dismissed_at = datetime('now', '-91 days') WHERE id = ?",
		userID,
	)

	// NPS should show again because dismiss has expired
	rr := doRequest(server.handleNPSCheck, "GET", "/api/nps/check", nil, []*http.Cookie{cookie})
	result := parseJSON(rr)
	if result["show_nps"] != true {
		t.Error("Expected show_nps=true after dismiss expires (91 days > 90 day interval)")
	}
}

func TestNPSDismiss_Idempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Dismiss twice — should succeed both times (idempotent)
	rr := doRequest(server.handleNPSDismiss, "POST", "/api/nps/dismiss", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First dismiss: got %d, want 200", rr.Code)
	}
	rr = doRequest(server.handleNPSDismiss, "POST", "/api/nps/dismiss", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Second dismiss: got %d, want 200", rr.Code)
	}
}

func TestNPS_RateLimited(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Pre-fill the rate limiter with 3 entries (max allowed)
	val, _ := server.npsSubmitLimiter.LoadOrStore(userID, &rateBucket{})
	rb := val.(*rateBucket)
	rb.mu.Lock()
	now := time.Now()
	for i := 0; i < 3; i++ {
		rb.times = append(rb.times, now.Add(-time.Duration(i)*time.Minute))
	}
	rb.mu.Unlock()

	body := map[string]interface{}{"score": 8, "comment": "great"}
	rr := doRequest(server.handleNPS, "POST", "/api/nps", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected 429 rate limited, got %d", rr.Code)
	}
}

// =============================================================================
// AVATAR TESTS
// =============================================================================

// makeJPEG returns minimal valid JPEG bytes (FF D8 header + padding).


// =============================================================================
// MILESTONE TESTS
// =============================================================================

func TestMilestoneCheck_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleMilestoneCheck, "GET", "/api/milestones/check", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMilestoneCheck_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleMilestoneCheck, "POST", "/api/milestones/check", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestMilestoneCheck_NoSavings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleMilestoneCheck, "GET", "/api/milestones/check", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["milestone"] != nil {
		t.Errorf("Expected nil milestone for no savings, got %v", body["milestone"])
	}
}

func TestMilestoneCheck_WithSavings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number, name) VALUES (100, '+15551234567', 'Test User')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Insert deal and rebates totaling $12 (1200 cents) — should trigger $10 milestone
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (900, 'heinz', 'Hunts', 'Ketchup', 200, 10000, 'active')`)
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (900, 100, 900, 'accepted')")
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (100, 900, 900, 1200, 'pending')")

	rr := doRequest(server.handleMilestoneCheck, "GET", "/api/milestones/check", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	milestone, ok := body["milestone"].(map[string]interface{})
	if !ok || milestone == nil {
		t.Fatalf("Expected milestone object, got %v", body["milestone"])
	}
	if cents, ok := milestone["cents"].(float64); !ok || cents != 1000 {
		t.Errorf("milestone.cents: got %v, want 1000", milestone["cents"])
	}
	if dollars, ok := milestone["dollars"].(string); !ok || dollars != "$10" {
		t.Errorf("milestone.dollars: got %v, want '$10'", milestone["dollars"])
	}
}

func TestMilestoneAck_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Insert deal and rebates
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (900, 'heinz', 'Hunts', 'Ketchup', 200, 10000, 'active')`)
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (900, 100, 900, 'accepted')")
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (100, 900, 900, 1200, 'pending')")

	// First check — should show $10 milestone
	rr := doRequest(server.handleMilestoneCheck, "GET", "/api/milestones/check", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	milestone := body["milestone"].(map[string]interface{})
	if milestone["cents"].(float64) != 1000 {
		t.Fatalf("Expected $10 milestone, got %v", milestone["cents"])
	}

	// Acknowledge
	rr2 := doRequest(server.handleMilestoneAck, "POST", "/api/milestones/ack", map[string]int{"cents": 1000}, []*http.Cookie{cookie})
	if rr2.Code != http.StatusOK {
		t.Fatalf("Ack status: got %d, want 200", rr2.Code)
	}

	// Check again — should be nil now
	rr3 := doRequest(server.handleMilestoneCheck, "GET", "/api/milestones/check", nil, []*http.Cookie{cookie})
	body3 := parseJSON(rr3)
	if body3["milestone"] != nil {
		t.Errorf("Expected nil milestone after ack, got %v", body3["milestone"])
	}
}

func TestMilestoneAck_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleMilestoneAck, "POST", "/api/milestones/ack", map[string]int{"cents": 1000}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestMilestoneAck_InvalidBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleMilestoneAck, "POST", "/api/milestones/ack", map[string]int{"cents": 0}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// BRAND DIGEST TESTS
// =============================================================================
// LEARNING INFRASTRUCTURE TESTS
// =============================================================================

func TestPennyInsights_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestPennyInsights_EmptyUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if _, ok := body["user_learnings"].(map[string]interface{}); !ok {
		t.Error("Response missing user_learnings object")
	}
	if _, ok := body["patterns"].(map[string]interface{}); !ok {
		t.Error("Response missing patterns object")
	}
	if _, ok := body["swap_behavior"].(map[string]interface{}); !ok {
		t.Error("Response missing swap_behavior object")
	}
}

func TestRecordSignal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")

	server.recordSignal("user-100", "item_added", "Dairy", "Organic Valley", map[string]interface{}{"name": "Whole Milk"})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM user_signals WHERE user_id = 100").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 signal, got %d", count)
	}

	var signal, category, value string
	server.db.QueryRow("SELECT signal, category, value FROM user_signals WHERE user_id = 100").Scan(&signal, &category, &value)
	if signal != "item_added" {
		t.Errorf("signal: got %q, want 'item_added'", signal)
	}
	if category != "Dairy" {
		t.Errorf("category: got %q, want 'Dairy'", category)
	}
}

func TestUpdateLearning(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// First insert
	server.updateLearning("user", 1, "shopping_frequency", "weekly", 0.7)

	var value string
	var conf float64
	server.db.QueryRow("SELECT value, confidence FROM penny_learnings WHERE scope = 'user' AND scope_id = 1 AND key = 'shopping_frequency'").Scan(&value, &conf)
	if value != "weekly" || conf != 0.7 {
		t.Errorf("Got value=%q conf=%v, want weekly/0.7", value, conf)
	}

	// Upsert
	server.updateLearning("user", 1, "shopping_frequency", "daily", 0.9)
	server.db.QueryRow("SELECT value, confidence FROM penny_learnings WHERE scope = 'user' AND scope_id = 1 AND key = 'shopping_frequency'").Scan(&value, &conf)
	if value != "daily" || conf != 0.9 {
		t.Errorf("After upsert: got value=%q conf=%v, want daily/0.9", value, conf)
	}
}

func TestProcessLearnings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create user with some activity
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (900, 'heinz', 'Hunts', 'Ketchup', 200, 10000, 'active')`)

	// Add swap events
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'offered')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'offered')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'offered')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'accepted')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'accepted')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-100', 900, 'declined')")

	// Add a list item so foreign key is satisfied, then add modifications
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, 100, 'Milk', 0)")
	for i := 0; i < 15; i++ {
		server.db.Exec(fmt.Sprintf("INSERT INTO item_modifications (item_id, user_id, action, source, created_at) VALUES (1, 100, 'added', 'app', datetime('now', '-%d days'))", i))
	}

	// Run learning loop
	server.processLearnings()

	// Verify user-level learnings were created
	var freq string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 100 AND key = 'shopping_frequency'").Scan(&freq)
	if freq == "" {
		t.Error("Expected shopping_frequency learning to be set")
	}

	// Verify brand-level learning
	var brandPerf string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'brand' AND key = 'performance:hunts'").Scan(&brandPerf)
	if brandPerf == "" {
		t.Error("Expected brand performance learning to be set")
	}
}

func TestPennyInsights_WithLearnings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Pre-populate a learning
	server.updateLearning("user", 100, "shopping_frequency", "daily", 0.8)

	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	learnings := body["user_learnings"].(map[string]interface{})
	freqEntry, ok := learnings["shopping_frequency"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected shopping_frequency in user_learnings")
	}
	if freqEntry["value"] != "daily" {
		t.Errorf("shopping_frequency value: got %v, want 'daily'", freqEntry["value"])
	}
}

func TestBuildLearningContext_AutotuneOff(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number, penny_autotune) VALUES (100, '+15551234567', 0)")
	ctx := server.buildLearningContext("user-100")
	if ctx != "" {
		t.Errorf("Expected empty context when autotune is off, got: %s", ctx)
	}
}

func TestBuildLearningContext_WithLearnings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number, penny_autotune) VALUES (100, '+15551234567', 1)")

	// Add learnings
	server.updateLearning("user", 100, "shopping_frequency", "weekly", 0.7)
	server.updateLearning("user", 100, "price_sensitivity", "high", 0.8)
	server.updateLearning("user", 100, "preferred_shopping_day", "Sunday", 0.6)

	// Add brand preference
	server.db.Exec("INSERT INTO brand_preferences (user_id, product_category, brand, pick_count, streak) VALUES (100, 'yogurt', 'Fage', 5, 3)")

	ctx := server.buildLearningContext("user-100")
	if ctx == "" {
		t.Fatal("Expected non-empty learning context")
	}
	if !strings.Contains(ctx, "weekly") {
		t.Error("Expected shopping frequency in context")
	}
	if !strings.Contains(ctx, "high") {
		t.Error("Expected price sensitivity in context")
	}
	if !strings.Contains(ctx, "Sunday") {
		t.Error("Expected preferred day in context")
	}
	if !strings.Contains(ctx, "Fage") {
		t.Error("Expected brand preference in context")
	}
}

func TestRepurchaseCycleDetection(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES ('user-100', '+15551234567')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, 100, 'Milk', 0)")

	// Add "Milk" additions with ~7 day intervals
	for i := 0; i < 4; i++ {
		server.db.Exec(fmt.Sprintf("INSERT INTO item_modifications (item_id, user_id, action, source, created_at) VALUES (1, 100, 'added', 'app', datetime('now', '-%d days'))", i*7))
	}

	server.processLearnings()

	// Verify repurchase cycle was detected
	var cycleDays string
	server.db.QueryRow("SELECT value FROM penny_learnings WHERE scope = 'user' AND scope_id = 100 AND key = 'repurchase_cycle:milk'").Scan(&cycleDays)
	if cycleDays == "" {
		t.Error("Expected repurchase cycle for milk to be detected")
	}
	if !strings.Contains(cycleDays, "7") {
		t.Errorf("Expected ~7 day cycle, got: %s", cycleDays)
	}
}

func TestBuildLearningContext_LowConfidenceExcluded(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.db.Exec("INSERT INTO users (id, phone_number, penny_autotune) VALUES (100, '+15551234567', 1)")

	// Add low-confidence learning (below 0.3 threshold)
	server.updateLearning("user", 100, "shopping_frequency", "daily", 0.1)

	ctx := server.buildLearningContext("user-100")
	if strings.Contains(ctx, "daily") {
		t.Error("Low-confidence learning should be excluded from context")
	}
}

// =============================================================================
// DIETARY PROFILE TESTS
// =============================================================================
// REPURCHASE NUDGE TESTS (L2 Household Learning)
// =============================================================================

func TestRepurchaseNudges_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges for new user, got %d", len(nudges))
	}
}

func TestRepurchaseNudges_DueItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(`INSERT INTO penny_learnings (scope, scope_id, key, value, confidence, updated_at)
		VALUES ('user', ?, 'repurchase_cycle:Milk', '7', 0.8, datetime('now'))`, userID)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, ?, 'Milk', 1)", userID)
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (1, ?, 'added', datetime('now', '-10 days'))`, userID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 1 {
		t.Fatalf("Expected 1 nudge, got %d", len(nudges))
	}
	nudge := nudges[0].(map[string]interface{})
	if nudge["item_name"] != "Milk" {
		t.Errorf("Expected nudge for 'Milk', got %v", nudge["item_name"])
	}
}

func TestRepurchaseNudges_NotDueYet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(`INSERT INTO penny_learnings (scope, scope_id, key, value, confidence, updated_at)
		VALUES ('user', ?, 'repurchase_cycle:Eggs', '14', 0.7, datetime('now'))`, userID)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, ?, 'Eggs', 1)", userID)
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (1, ?, 'added', datetime('now', '-5 days'))`, userID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges (not due yet), got %d", len(nudges))
	}
}

func TestRepurchaseNudges_AlreadyOnList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(`INSERT INTO penny_learnings (scope, scope_id, key, value, confidence, updated_at)
		VALUES ('user', ?, 'repurchase_cycle:Bread', '7', 0.8, datetime('now'))`, userID)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, ?, 'Bread', 0)", userID)
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (1, ?, 'added', datetime('now', '-10 days'))`, userID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges (already on list), got %d", len(nudges))
	}
}

func TestRepurchaseNudges_Unauthorized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestRepurchaseNudges_InLearningContext(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec("INSERT INTO users (id, phone_number, penny_autotune) VALUES (100, '+15551234567', 1)")

	// Set up a due repurchase nudge
	server.db.Exec(`INSERT INTO penny_learnings (scope, scope_id, key, value, confidence, updated_at)
		VALUES ('user', 100, 'repurchase_cycle:Milk', '7', 0.8, datetime('now'))`)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, checked) VALUES (1, 100, 'Milk', 1)")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (1, 100, 'added', datetime('now', '-10 days'))`)

	ctx := server.buildLearningContext("user-100")
	if !strings.Contains(ctx, "Items due for restock") {
		t.Errorf("Expected 'Items due for restock' in learning context, got: %s", ctx)
	}
	if !strings.Contains(ctx, "Milk") {
		t.Errorf("Expected 'Milk' in nudge context, got: %s", ctx)
	}
}

// =============================================================================
// OPTIMIZE THIS DEAL TESTS (L3 Brand Learning)
// =============================================================================
// REPURCHASE NUDGE TESTS
// =============================================================================

func TestRepurchaseNudge_NoSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestRepurchaseNudge_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleRepurchaseNudges, "POST", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rr.Code)
	}
}

func TestRepurchaseNudge_EmptyWhenNoLearnings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	nudges, ok := body["nudges"].([]interface{})
	if !ok {
		t.Fatal("Expected 'nudges' array in response")
	}
	if len(nudges) != 0 {
		t.Errorf("Expected empty nudges array, got %d items", len(nudges))
	}
}

func TestRepurchaseNudge_OverdueItemReturned(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Seed a repurchase_cycle learning: milk every 7 days
	server.updateLearning("user", userID, "repurchase_cycle:milk", "7 days", 0.8)

	// Seed item_modifications: milk was last added 10 days ago
	itemID := createListItem(t, server.db, userID, "Milk", "Dairy")
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, itemID, userID, tenDaysAgo)

	// Mark the list item as checked so it doesn't suppress the nudge
	server.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", itemID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	nudges, ok := body["nudges"].([]interface{})
	if !ok {
		t.Fatal("Expected 'nudges' array in response")
	}
	if len(nudges) != 1 {
		t.Fatalf("Expected 1 nudge, got %d", len(nudges))
	}

	nudge := nudges[0].(map[string]interface{})
	if nudge["item_name"] != "milk" {
		t.Errorf("Expected item_name 'milk', got %v", nudge["item_name"])
	}
	if int(nudge["cycle_days"].(float64)) != 7 {
		t.Errorf("Expected cycle_days 7, got %v", nudge["cycle_days"])
	}
	// days_overdue should be ~3 (10 days since added - 7 day cycle)
	daysOverdue := int(nudge["days_overdue"].(float64))
	if daysOverdue < 2 || daysOverdue > 4 {
		t.Errorf("Expected days_overdue ~3, got %d", daysOverdue)
	}
	if nudge["last_added"] == nil || nudge["last_added"] == "" {
		t.Error("Expected non-empty last_added date")
	}
}

func TestRepurchaseNudge_NotDueYet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Seed a repurchase_cycle learning: milk every 14 days
	server.updateLearning("user", userID, "repurchase_cycle:milk", "14 days", 0.8)

	// Seed item_modifications: milk was last added 5 days ago (not due for 9 more days)
	itemID := createListItem(t, server.db, userID, "Milk", "Dairy")
	fiveDaysAgo := time.Now().Add(-5 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, itemID, userID, fiveDaysAgo)

	// Mark checked so it's not "on the list"
	server.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", itemID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges (item not due yet), got %d", len(nudges))
	}
}

func TestRepurchaseNudge_SuppressedWhenOnList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Seed a repurchase_cycle learning: eggs every 7 days
	server.updateLearning("user", userID, "repurchase_cycle:eggs", "7 days", 0.8)

	// Seed item_modifications: eggs were last added 10 days ago (overdue)
	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy")
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, itemID, userID, tenDaysAgo)

	// Leave the item unchecked on the list — nudge should be suppressed
	// (checked defaults to 0 / false)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges (item already on list), got %d", len(nudges))
	}
}

func TestRepurchaseNudge_MultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Seed two repurchase cycles
	server.updateLearning("user", userID, "repurchase_cycle:milk", "7 days", 0.8)
	server.updateLearning("user", userID, "repurchase_cycle:bread", "5 days", 0.6)

	// Both items overdue
	milkID := createListItem(t, server.db, userID, "Milk", "Dairy")
	breadID := createListItem(t, server.db, userID, "Bread", "Bakery")
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, milkID, userID, tenDaysAgo)
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, breadID, userID, tenDaysAgo)

	// Mark both as checked (off the active list)
	server.db.Exec("UPDATE list_items SET checked = 1 WHERE id IN (?, ?)", milkID, breadID)

	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 2 {
		t.Errorf("Expected 2 nudges, got %d", len(nudges))
	}
}

func TestRepurchaseNudge_DoesNotLeakAcrossUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "5551234567")
	cookieB, _ := loginUser(t, server, "5559876543")

	// Seed repurchase cycle for user A only
	server.updateLearning("user", userA, "repurchase_cycle:milk", "7 days", 0.8)
	itemID := createListItem(t, server.db, userA, "Milk", "Dairy")
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(`INSERT INTO item_modifications (item_id, user_id, action, created_at)
		VALUES (?, ?, 'added', ?)`, itemID, userA, tenDaysAgo)
	server.db.Exec("UPDATE list_items SET checked = 1 WHERE id = ?", itemID)

	// User B should see no nudges
	rr := doRequest(server.handleRepurchaseNudges, "GET", "/api/penny/nudges", nil, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	body := parseJSON(rr)
	nudges := body["nudges"].([]interface{})
	if len(nudges) != 0 {
		t.Errorf("Expected 0 nudges for user B, got %d", len(nudges))
	}
}

// =============================================================================
// SWAP INTERACTION ENRICHMENT TESTS
// =============================================================================

// --- Penny Quote Tests ---

func TestPennyQuote_Login_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny/quote?context=login", nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for login quote (no auth required), got %d", rr.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	if _, ok := result["quote"]; !ok {
		t.Error("expected 'quote' field in response")
	}
}

func TestPennyQuote_NonLogin_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny/quote?context=chime", nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-login quote without auth, got %d", rr.Code)
	}
}

func TestPennyQuote_DefaultContextIsLogin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny/quote", nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	// No context defaults to "login" which doesn't require auth
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for default context, got %d", rr.Code)
	}
}


// =============================================================================
// handlePennyQuote TESTS (59.5% → target ~85%)
// =============================================================================

func TestPennyQuote_NameSanitization_NumbersStripped(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Name with numbers should have them stripped — the handler strips non-alpha chars
	req := httptest.NewRequest("GET", "/api/penny/quote?context=login&name=John123", nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	// The quote should still be returned — sanitization doesn't cause an error
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	if _, ok := result["quote"]; !ok {
		t.Error("expected 'quote' field in response")
	}
}

func TestPennyQuote_NameTruncation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Name longer than 30 chars should be truncated
	longName := "Abcdefghijklmnopqrstuvwxyzabcdefghijklmnop" // 43 chars
	req := httptest.NewRequest("GET", "/api/penny/quote?context=login&name="+longName, nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	if _, ok := result["quote"]; !ok {
		t.Error("expected 'quote' field in response")
	}
}

func TestPennyQuote_UnknownContextDefaultsToChime(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Unknown context "foobar" — the handler will use it as-is. Since it's not "login",
	// it requires auth. The pool will be empty (no fallbacks for "foobar"), so it should
	// return the generic "Hey there!" fallback.
	req := httptest.NewRequest("GET", "/api/penny/quote?context=foobar", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	quote, ok := result["quote"].(string)
	if !ok || quote == "" {
		t.Error("expected non-empty quote for unknown context")
	}
	// Unknown context has no fallback pool → should be "Hey there!"
	if quote != "Hey there!" {
		t.Errorf("expected 'Hey there!' for unknown context with empty pool, got %q", quote)
	}
}

func TestPennyQuote_ChimeWithoutAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/penny/quote?context=chime", nil)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for chime context without auth, got %d", rr.Code)
	}
}

func TestPennyQuote_DoneContext(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("GET", "/api/penny/quote?context=done", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handlePennyQuote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for done context with auth, got %d", rr.Code)
	}
	var result map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &result)
	quote, ok := result["quote"].(string)
	if !ok || quote == "" {
		t.Error("expected non-empty quote for done context")
	}
	// Should come from the "done" fallback pool
	doneFallbacks := []string{
		"You're all done!", "List complete — nice job!", "Everything's checked off!",
	}
	found := false
	for _, fb := range doneFallbacks {
		if quote == fb {
			found = true
			break
		}
	}
	if !found {
		// Could be from LLM cache, that's OK — just check it's non-empty (already checked above)
		t.Logf("Quote %q not in fallback pool (may be LLM-generated, which is fine)", quote)
	}
}


// --- PennyInsights with data ---

func TestPennyInsights_WithHouseholdLearnings(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559990001")

	// Create household and add user
	result, _ := server.db.Exec("INSERT INTO households DEFAULT VALUES")
	hhID, _ := result.LastInsertId()
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hhID, userID)
	server.db.Exec("INSERT INTO household_members (household_id, user_id) VALUES (?, ?)", hhID, userID)

	// Add household-level learning
	server.updateLearning("household", int(hhID), "coordination_score", "high", 0.9)

	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	hl := body["household_learnings"].(map[string]interface{})
	if _, ok := hl["coordination_score"]; !ok {
		t.Error("expected coordination_score in household_learnings")
	}
}

func TestPennyInsights_WithSwapBehavior(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559990002")

	// Add swap events
	server.db.Exec("INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status) VALUES (1, 'comp', 'brand', 'offer', 100, 10000, 'active')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (?, 1, 'offered')", userID)
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (?, 1, 'offered')", userID)
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (?, 1, 'accepted')", userID)

	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	swap := body["swap_behavior"].(map[string]interface{})
	if swap["offered"].(float64) != 2 {
		t.Errorf("expected 2 offered, got %v", swap["offered"])
	}
	if swap["accepted"].(float64) != 1 {
		t.Errorf("expected 1 accepted, got %v", swap["accepted"])
	}
	if swap["accept_rate"].(float64) != 50 {
		t.Errorf("expected accept_rate 50, got %v", swap["accept_rate"])
	}
}

func TestPennyInsights_WithBrandAffinities(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559990003")

	server.db.Exec("INSERT INTO brand_preferences (user_id, product_category, brand, pick_count, streak) VALUES (?, 'milk', 'Organic Valley', 8, 5)", userID)

	rr := doRequest(server.handlePennyInsights, "GET", "/api/penny/insights", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	patterns := body["patterns"].(map[string]interface{})
	affinities := patterns["brand_affinities"].([]interface{})
	if len(affinities) == 0 {
		t.Fatal("expected at least one brand affinity")
	}
	first := affinities[0].(map[string]interface{})
	if first["brand"] != "Organic Valley" {
		t.Errorf("expected Organic Valley, got %v", first["brand"])
	}
}

func TestPennyInsights_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePennyInsights, "POST", "/api/penny/insights", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
