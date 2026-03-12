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
	"strings"
	"sync"
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
	SMSCount       int

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
	m.SMSCount++
	return m.SMSErr
}

// GetSMSCount returns the total number of SMS messages sent.
func (m *MockTwilioClient) GetSMSCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SMSCount
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

	server := &Server{
		config:   Config{Port: "8080", AnthropicAPIKey: "test-key", TwilioPhoneNumber: "+10000000000", AdminPhone: "3104861833", FeedbackPhones: []string{"5551234567"}},
		db:       db,
		sessions: NewSessionStore(db),
		twilio:   mock,
		parseItems: func(apiKey, text string, images []string) ([]ParsedItem, error) {
			// Default mock parser: returns nothing. Tests can override.
			return nil, nil
		},
		smsBuffer: newMessageBuffer(50 * time.Millisecond),
	}
	// Default parseCommand wraps parseItems so existing tests work
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		parsed, err := server.parseItems(apiKey, text, nil)
		if err != nil {
			return nil, err
		}
		return &ParsedCommand{Action: "add", Items: parsed}, nil
	}
	return server
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

// newTestSessionStore creates a SessionStore backed by an in-memory SQLite DB.
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

// loginUser creates a user in the DB and returns a session cookie for that user.
func loginUser(t *testing.T, server *Server, phone string) (*http.Cookie, int) {
	t.Helper()
	user, err := upsertUser(server.db, phone)
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}
	token, err := server.sessions.CreateSession(user.ID, phone, "phone")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token}, user.ID
}

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

// doEmailRequest sends a multipart form request mimicking SendGrid Inbound Parse.
func doEmailRequest(handler http.HandlerFunc, from, text string) *httptest.ResponseRecorder {
	body := fmt.Sprintf("from=%s&text=%s", url.QueryEscape(from), url.QueryEscape(text))
	req := httptest.NewRequest("POST", "/api/email", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
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

// makeJPEG returns minimal valid JPEG bytes (FF D8 header + padding).
func makeJPEG(size int) []byte {
	data := make([]byte, size)
	data[0] = 0xFF
	data[1] = 0xD8
	return data
}

// makePNG returns minimal valid PNG bytes (89 50 4E 47 header + padding).
func makePNG(size int) []byte {
	data := make([]byte, size)
	data[0] = 0x89
	data[1] = 0x50
	data[2] = 0x4E
	data[3] = 0x47
	return data
}

// brandRegister is a helper that registers a brand user and returns the session cookie and brand_user_id.
func brandRegister(t *testing.T, server *Server, email, password, companyName string) (*http.Cookie, int) {
	t.Helper()
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        email,
		"password":     password,
		"company_name": companyName,
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("brandRegister: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "brand_session" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("brandRegister: no brand_session cookie returned")
	}
	body := parseJSON(rr)
	brandUserID := int(body["brand_user_id"].(float64))
	return cookie, brandUserID
}

// createTestDeal is a helper that creates a brand deal and returns the deal ID.
func createTestDeal(t *testing.T, server *Server, cookie *http.Cookie) int {
	t.Helper()
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product":     "Competitor Cola 12pk",
		"offer_brand":            "TestBrand",
		"offer_product":          "TestBrand Cola 12pk",
		"discount_cents":         150,
		"daily_budget_cents":     10000,
		"offer_price_cents":      499,
		"competitor_price_cents": 599,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("createTestDeal: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	return int(body["id"].(float64))
}

// setupEarningsTestData creates a user with brand_deals, swap_events, and swap_rebates
// for earnings tests. Returns (cookie, userID, dealID, rebateID).
func setupEarningsTestData(t *testing.T, server *Server) (*http.Cookie, int, int, int) {
	t.Helper()
	cookie, userID := loginUser(t, server, "5559991234")

	// Create a brand deal
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES ('Heinz Ketchup', 'Hunts', 'Tomato Ketchup', 150, 5000, 'active')`)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'Heinz Ketchup'").Scan(&dealID)

	// Create a list item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'ketchup', 'Condiments')", userID)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ?", userID).Scan(&itemID)

	// Create a swap event
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'accept')", userID, dealID, itemID)
	var swapEventID int
	server.db.QueryRow("SELECT id FROM swap_events WHERE user_id = ?", userID).Scan(&swapEventID)

	// Create a swap rebate
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, 150, 'pending')", userID, swapEventID, dealID)
	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE user_id = ?", userID).Scan(&rebateID)

	return cookie, userID, dealID, rebateID
}

// newMockGoogleTokenServer creates a mock Google tokeninfo endpoint that returns
// the provided tokenInfo JSON for any request.
func newMockGoogleTokenServer(t *testing.T, tokenInfoJSON map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenInfoJSON)
	}))
}
