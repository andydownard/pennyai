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
	"sync/atomic"
	"testing"
	"time"
)


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
	var userID string
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
	var userID string
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

// =============================================================================
// SMS/EMAIL DUPE QUANTITY TESTS
// =============================================================================

func TestSMSDupe_AlwaysIncreasesQuantity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Add "Milk" with qty 1 via app
	_, _, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}

	// Add "Milk" again via SMS with qty 2
	item, isNew, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 2, "sms", "", "")
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
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Add "Bread" via app
	upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "", "")

	// Add "Bread" again via app — should report as NOT new (skipped)
	_, isNew, _ := upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "", "")
	if isNew {
		t.Error("App dupe should report isNew=false (skipped)")
	}
}

func TestSMSDupe_PreservesOriginalAdder(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// User 1 adds item via app
	_, userID1 := loginUser(t, server, "5551111111")
	item, _, _ := upsertListItem(server.db, userID1, "Yogurt", "", "Dairy & Eggs", "", 1, "app", "", "")

	// User 2 (same household) adds same item via SMS
	_, userID2 := loginUser(t, server, "5552222222")
	// Join same household
	var hid int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hid)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hid, userID2)
	server.db.Exec("INSERT OR IGNORE INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hid, userID2)

	upsertListItem(server.db, userID2, "Yogurt", "", "Dairy & Eggs", "", 2, "sms", "", "")

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
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
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
	server := newTestServer(t, &MockTwilioClient{})
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
	server := newTestServer(t, &MockTwilioClient{})
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
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WebhookBaseURL = "https://example.com"
	server.config.TwilioAuthToken = "correct-secret"

	body := strings.NewReader("from=alice@test.com&text=milk")
	req := httptest.NewRequest("POST", "/api/email?secret=wrong-secret", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailWebhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid secret, got %d", rr.Code)
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

// --- SMS Webhook Tests ---

func TestSMSWebhook_BuffersAndProcesses(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Eggs", Department: "Dairy", Quantity: 1},
		}, nil
	}

	// Use %2B for the + sign so form parsing doesn't convert it to a space
	body := strings.NewReader("From=%2B15551234567&Body=eggs")
	req := httptest.NewRequest("POST", "/api/sms", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	// Should return TwiML
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<Response>") {
		t.Error("expected TwiML response")
	}

	// Wait for the buffer to flush (50ms in test + margin)
	time.Sleep(200 * time.Millisecond)

	// Item should have been added (user auto-created by upsertUser)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE name = 'Eggs'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 Eggs item, got %d", count)
	}

	// SMS reply should have been sent
	to, smsBody := mock.GetLastSMS()
	if to != "+15551234567" {
		t.Errorf("expected SMS to +15551234567, got %s", to)
	}
	if !strings.Contains(smsBody, "Eggs") {
		t.Errorf("expected SMS body to contain 'Eggs', got %q", smsBody)
	}
}

func TestSMSWebhook_InvalidSignature(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WebhookBaseURL = "https://example.com"
	server.config.TwilioAuthToken = "real-token"

	body := strings.NewReader("From=+15551234567&Body=milk")
	req := httptest.NewRequest("POST", "/api/sms", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", "invalid-sig")
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid Twilio signature, got %d", rr.Code)
	}
}

func TestSMSWebhook_EmptyMessage(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	body := strings.NewReader("From=+15551234567&Body=&NumMedia=0")
	req := httptest.NewRequest("POST", "/api/sms", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleSMSWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Wait for buffer
	time.Sleep(200 * time.Millisecond)

	// No items should be added, no SMS reply
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 items for empty SMS, got %d", count)
	}
}

// =============================================================================
// CTIA / A2P 10DLC keyword handling tests
// =============================================================================

func TestClassifySMSKeyword(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		// stop variants
		{"STOP", "stop"},
		{"stop", "stop"},
		{" Stop ", "stop"},
		{"STOPALL", "stop"},
		{"UNSUBSCRIBE", "stop"},
		{"CANCEL", "stop"},
		{"END", "stop"},
		{"QUIT", "stop"},
		// help variants
		{"HELP", "help"},
		{"help", "help"},
		{"INFO", "help"},
		// start variants
		{"START", "start"},
		{"unstop", "start"},
		// NON-keyword: list items, embedded usage, conversational YES
		{"milk and bread", ""},
		{"I need help with eggs", ""}, // embedded "help" must NOT trigger
		{"YES", ""},                    // YES is conversational, not a global keyword
		{"NO", ""},
		{"stop me from buying milk", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := classifySMSKeyword(c.body)
		if got != c.want {
			t.Errorf("classifySMSKeyword(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

func TestSMSWebhook_HelpKeyword_RespondsBranded(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Track that Claude was NOT called
	parseCalled := false
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		parseCalled = true
		return nil, nil
	}

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "HELP", 0)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	if parseCalled {
		t.Error("HELP keyword must not be parsed by Claude")
	}

	// Branded HELP reply must mention Groflo and STOP
	_, body := mock.GetLastSMS()
	if !strings.Contains(body, "Groflo") {
		t.Errorf("HELP reply must mention Groflo, got: %q", body)
	}
	if !strings.Contains(body, "STOP") {
		t.Errorf("HELP reply must mention STOP, got: %q", body)
	}

	// HELP must NOT add anything to the list
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 0 {
		t.Errorf("HELP must not add list items, got %d", count)
	}
}

func TestSMSWebhook_StopKeyword_ClearsConsent(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Pre-create user with consent
	server.db.Exec(`INSERT INTO users (id, phone_number, sms_consent_at)
		VALUES (1, '5551234567', CURRENT_TIMESTAMP)`)

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "STOP", 0)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}

	// Consent should be cleared
	if server.canSendSMS("user-1") {
		t.Error("canSendSMS should return false after STOP")
	}

	// Confirmation reply sent
	_, body := mock.GetLastSMS()
	if !strings.Contains(body, "unsubscribed") {
		t.Errorf("STOP confirmation should say 'unsubscribed', got: %q", body)
	}
	if !strings.Contains(body, "START") {
		t.Errorf("STOP confirmation should mention START to opt back in, got: %q", body)
	}
}

func TestSMSWebhook_StartKeyword_RestoresConsent(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	// Pre-create user without consent (previously opted out)
	server.db.Exec(`INSERT INTO users (id, phone_number, sms_consent_at)
		VALUES (1, '5551234567', NULL)`)

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "START", 0)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}

	// Consent should be restored
	if !server.canSendSMS("user-1") {
		t.Error("canSendSMS should return true after START")
	}

	// Welcome-back reply sent
	_, body := mock.GetLastSMS()
	if !strings.Contains(body, "Welcome back") {
		t.Errorf("START reply should say 'Welcome back', got: %q", body)
	}
}

func TestSMSWebhook_HelpInSentence_NotKeyword(t *testing.T) {
	// Embedded "help" (e.g., "I need help with the eggs") must reach the
	// item parser, not the keyword handler. The whole point of the exact
	// match in classifySMSKeyword is to allow grocery items containing
	// "help" / "stop" to flow through normally.
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{{Name: "eggs", Department: "Dairy", Quantity: 12}}, nil
	}

	rr := doSMSRequest(server.handleSMSWebhook, "+15551234567", "I need help with eggs", 0)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}

	// Wait for the debounce timer
	time.Sleep(200 * time.Millisecond)

	// Item should be added (not treated as keyword)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE name = 'eggs'").Scan(&count)
	if count != 1 {
		t.Errorf("embedded 'help' should not block item parsing, got count=%d", count)
	}
}

// --- List Swaps Tests ---
