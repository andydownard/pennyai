package main

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
)


// =============================================================================
// HOUSEHOLD TESTS
// =============================================================================

func TestEmailWebhook(t *testing.T) {
	server := newTestServer(t)
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
	server := newTestServer(t)
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
	server := newTestServer(t)
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
// EMAIL WEBHOOK — EMPTY BODY
// =============================================================================

func TestEmailWebhook_EmptyBody(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551234567")
	_ = cookie

	// Set up a user with email
	server.db.Exec("UPDATE users SET email = 'alice@example.com' WHERE phone = '5551234567'")

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
// MULTI-IMAGE TESTS
// =============================================================================

func TestEmailWebhook_MultipleImages(t *testing.T) {
	server := newTestServer(t)
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
	server := newTestServer(t)
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

// buildMultipartEmail creates a multipart form body mimicking SendGrid's format.


// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestEmailWebhook_NonImageAttachment(t *testing.T) {
	server := newTestServer(t)
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

// =============================================================================
// PHASE 4: NEW TESTS FOR 80% COVERAGE
// =============================================================================

// --- handleEmailWebhook tests ---

func TestEmailWebhook_NoFromAddress(t *testing.T) {
	server := newTestServer(t)

	rr := doEmailRequest(server.handleEmailWebhook, "", "milk and eggs")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestEmailWebhook_UnknownSender(t *testing.T) {
	server := newTestServer(t)

	rr := doEmailRequest(server.handleEmailWebhook, "nobody@example.com", "milk and eggs")
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
}

func TestEmailWebhook_ParseError(t *testing.T) {
	server := newTestServer(t)
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

// --- helper tests ---

// --- requireSession expired token ---

// --- Invite with null household (legacy user) ---

// --- handleJoin expired invite ---

// --- handleListAdd empty name ---

// --- handleListAdd invalid JSON ---

// --- Email webhook form parse error ---

func TestEmailWebhook_FormParseError(t *testing.T) {
	server := newTestServer(t)

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

// =============================================================================
// EMAIL DUPE QUANTITY TESTS
// =============================================================================

func TestEmailDupe_AlwaysIncreasesQuantity(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "5551234567")

	// Add "Eggs" with qty 1 via app
	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 1, "app", "", "")

	// Add "Eggs" again via email with qty 1
	item, isNew, _ := upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 1, "email", "", "")

	if !isNew {
		t.Error("Email dupe should report isNew=true")
	}
	if item.Quantity != 2 {
		t.Errorf("Expected quantity 2, got %d", item.Quantity)
	}
}

func TestAppDupe_StillReportsSkipped(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "5551234567")

	// Add "Bread" via app
	upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "", "")

	// Add "Bread" again via app — should report as NOT new (skipped)
	_, isNew, _ := upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "", "")
	if isNew {
		t.Error("App dupe should report isNew=false (skipped)")
	}
}

func TestEmailDupe_PreservesOriginalAdder(t *testing.T) {
	server := newTestServer(t)

	// User 1 adds item via app
	_, userID1 := loginUser(t, server, "5551111111")
	item, _, _ := upsertListItem(server.db, userID1, "Yogurt", "", "Dairy & Eggs", "", 1, "app", "", "")

	// User 2 (same household) adds same item via email
	_, userID2 := loginUser(t, server, "5552222222")
	// Join same household
	var hid int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hid)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hid, userID2)
	server.db.Exec("INSERT OR IGNORE INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hid, userID2)

	upsertListItem(server.db, userID2, "Yogurt", "", "Dairy & Eggs", "", 2, "email", "", "")

	// Original adder should still be user 1
	var ownerID string
	server.db.QueryRow("SELECT user_id FROM list_items WHERE id = ?", item.ID).Scan(&ownerID)
	if ownerID != userID1 {
		t.Errorf("Expected original adder %s, got %s", userID1, ownerID)
	}
}

// =============================================================================
// ADMIN DASHBOARD TESTS
// =============================================================================

func TestEmailWebhook_EmptyFrom(t *testing.T) {
	server := newTestServer(t)

	body := "from=&text=hello"
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
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

func TestEmailWebhook_KnownUserAddsItems(t *testing.T) {
	server := newTestServer(t)
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
	server := newTestServer(t)
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

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================

func TestExtractEmail_BareAddress(t *testing.T) {
	got := extractEmail("user@example.com")
	if got != "user@example.com" {
		t.Errorf("extractEmail bare: got %q, want %q", got, "user@example.com")
	}
}

func TestExtractEmail_NameAngleBracket(t *testing.T) {
	got := extractEmail("John Doe <john@example.com>")
	if got != "john@example.com" {
		t.Errorf("extractEmail angle-bracket: got %q, want %q", got, "john@example.com")
	}
}

func TestExtractEmail_TrimsWhitespace(t *testing.T) {
	got := extractEmail("  hello@example.com  ")
	if got != "hello@example.com" {
		t.Errorf("extractEmail trim: got %q, want %q", got, "hello@example.com")
	}
}

func TestExtractEmail_AngleBracketWithSpaces(t *testing.T) {
	got := extractEmail("My Name < spaced@example.com >")
	if got != "spaced@example.com" {
		t.Errorf("extractEmail spaced angle: got %q, want %q", got, "spaced@example.com")
	}
}

// --- Email Webhook Tests ---

func TestEmailWebhook_ParsesAndAddsItems(t *testing.T) {
	server := newTestServer(t)
	// Create user with email via loginUser, then set the email
	_, userID := loginUser(t, server, "+15550001111")
	server.db.Exec("UPDATE users SET email = 'alice@test.com' WHERE id = ?", userID)
	// Mock parser
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Department: "Dairy"},
			{Name: "Bread", Department: "Bakery"},
		}, nil
	}

	// Build form-encoded request like SendGrid would send
	body := strings.NewReader("from=alice@test.com&text=milk and bread&subject=groceries")
	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check items were added
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 items added, got %d", count)
	}
}

func TestEmailWebhook_UnknownSenderReturns200(t *testing.T) {
	server := newTestServer(t)
	body := strings.NewReader("from=unknown@test.com&text=milk")
	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	// Should still return 200 (don't make SendGrid retry)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown sender, got %d", rr.Code)
	}
}

func TestEmailWebhook_EmptyBodyNoItems(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "+15550002222")
	server.db.Exec("UPDATE users SET email = 'alice@test.com' WHERE id = ?", userID)

	body := strings.NewReader("from=alice@test.com&text=")
	req := httptest.NewRequest("POST", "/api/email", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty body, got %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 items for empty email, got %d", count)
	}
}

func TestEmailWebhook_InvalidSecret(t *testing.T) {
	server := newTestServer(t)
	server.config.WebhookBaseURL = "https://example.com"
	server.config.EmailWebhookSecret = "correct-secret"

	body := strings.NewReader("from=alice@test.com&text=milk")
	req := httptest.NewRequest("POST", "/api/email?secret=wrong-secret", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid secret, got %d", rr.Code)
	}
}

func TestEmailWebhook_MissingSecretRejected(t *testing.T) {
	server := newTestServer(t)
	server.config.WebhookBaseURL = "https://example.com"
	server.config.EmailWebhookSecret = "correct-secret"
	rr := doEmailRequest(server.handleEmailWebhook, "alice@test.com", "milk")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with no secret, got %d", rr.Code)
	}
}

func TestEmailWebhook_EmptyConfiguredSecretRejectsEverything(t *testing.T) {
	server := newTestServer(t)
	server.config.WebhookBaseURL = "https://example.com"
	req := httptest.NewRequest("POST", "/api/email?secret=", strings.NewReader("from=alice@test.com&text=milk"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when no secret is configured, got %d", rr.Code)
	}
}

func TestEmailWebhook_ValidSecretAccepted(t *testing.T) {
	server := newTestServer(t)
	server.config.WebhookBaseURL = "https://example.com"
	server.config.EmailWebhookSecret = "correct-secret"
	req := httptest.NewRequest("POST", "/api/email?secret=correct-secret", strings.NewReader("from=nobody@test.com&text=milk"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with the right secret, got %d", rr.Code)
	}
}

func TestEmailWebhook_ExtractEmail(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"alice@test.com", "alice@test.com"},
		{"Alice <alice@test.com>", "alice@test.com"},
		{"  Alice Smith <alice@test.com>  ", "alice@test.com"},
		{"UPPER@TEST.COM", "upper@test.com"},
	}
	for _, tc := range tests {
		got := extractEmail(tc.input)
		if got != tc.expected {
			t.Errorf("extractEmail(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// =============================================================================
// CTIA / A2P 10DLC keyword handling tests
// =============================================================================

// --- List Swaps Tests ---
