package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// openTestDB gives the test its own PostgreSQL database, cloned from a template
// built by the production initDB. Returns the raw *sql.DB (which accepts
// legacy-dialect `?` placeholders, test fixtures only) and the *PGDB wrapper.
func openTestDB(t *testing.T) (*sql.DB, *PGDB) {
	t.Helper()
	return openPostgresTestDB(t, postgresTestURL())
}

// =============================================================================
// CONCURRENCY TEST HELPER
// =============================================================================

// hammer runs fn concurrently n times and waits for all of them to finish,
// releasing them as close to simultaneously as a start barrier allows. Use it
// to exercise TOCTOU windows in money-handling paths.
//
// IMPORTANT: whether a check-then-act interleaving actually occurs depends on
// goroutine scheduling. Every hammer test MUST therefore be paired with a deterministic negative test that pre-seeds
// the conflicting row and asserts the guard fires, so correctness never
// depends on scheduler luck.
func hammer(t *testing.T, n int, fn func(i int)) {
	t.Helper()
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			ready.Done()
			<-start
			fn(i)
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// newTestServer creates a Server with its own PostgreSQL database. Each test gets a fresh database -- no leaking state between
// tests.
func newTestServer(t *testing.T) *Server {
	t.Helper() // marks this as a helper so errors show the caller's line number

	_, pgdb := openTestDB(t)

	server := &Server{
		config:   Config{Port: "8080", AppBaseURL: "https://groflo.rallytasks.com", AnthropicAPIKey: "test-key", AdminClerkUserIDs: []string{testAdminClerkID}, BcryptCost: 4},
		db:       pgdb,
		sessions: NewSessionStore(pgdb),
		parseItems: func(apiKey, text string, images []string) ([]ParsedItem, error) {
			// Default mock parser: returns nothing. Tests can override.
			return nil, nil
		},
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
func createListItem(t *testing.T, db *PGDB, userID string, name, department string) int64 {
	t.Helper()
	result, err := db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, name, department)
	if err != nil {
		t.Fatalf("createListItem: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

// newTestSessionStore creates a SessionStore backed by a fresh PostgreSQL test database.
func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	rawDB, pgdb := openTestDB(t)
	// Create users so FK constraints are satisfied
	rawDB.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567'), ('user-2', '5552222222')")
	return NewSessionStore(pgdb)
}

// upsertUser is a TEST FIXTURE: it creates (or finds) a user keyed by the legacy
// users.phone column and returns it. Production code has no phone-keyed
// upsert; the column survives only so existing rows and tests keep a stable
// per-user key.
func upsertUser(db *PGDB, phone string) (User, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	newID := uuid.New().String()
	if _, err = tx.Exec("INSERT OR IGNORE INTO users (id, phone) VALUES (?, ?)", newID, phone); err != nil {
		return User{}, fmt.Errorf("upserting user: %w", err)
	}

	if phone == testAdminPhone {
		// The admin test user is admin by Clerk id, never by phone.
		if _, err := tx.Exec("UPDATE users SET clerk_user_id = ? WHERE phone = ? AND clerk_user_id IS NULL", testAdminClerkID, phone); err != nil {
			return User{}, fmt.Errorf("binding admin clerk id: %w", err)
		}
	}

	var user User
	var householdID sql.NullInt64
	var name, email sql.NullString
	err = tx.QueryRow(
		"SELECT id, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE phone = ?",
		phone,
	).Scan(&user.ID, &name, &email, &householdID, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user: %w", err)
	}
	user.Name = name.String
	user.Email = email.String
	if householdID.Valid {
		user.HouseholdID = int(householdID.Int64)
	}
	if _, err := tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", user.ID); err != nil {
		log.Printf("[warn] failed to update login count for user %s: %v", user.ID, err)
	}
	if !householdID.Valid {
		hID, err := createSoloHouseholdTx(tx, user.ID)
		if err != nil {
			return User{}, err
		}
		user.HouseholdID = hID
	}
	tx.Exec("INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)", user.ID, phone)
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("committing upsert: %w", err)
	}
	return user, nil
}

// loginUser creates a user in the DB and returns a session cookie for that user.
func loginUser(t *testing.T, server *Server, phone string) (*http.Cookie, string) {
	t.Helper()
	user, err := upsertUser(server.db, phone)
	if err != nil {
		t.Fatalf("upsertUser: %v", err)
	}
	token, err := server.sessions.CreateSession(user.ID, phone, "test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token}, user.ID
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
	var brandUserID int
	switch v := body["brand_user_id"].(type) {
	case float64:
		brandUserID = int(v)
	case string:
		fmt.Sscanf(v, "%d", &brandUserID)
	}
	return cookie, brandUserID
}

// createTestDeal is a helper that creates a brand deal and returns the deal ID.
// Deals are created as pending_review and auto-approved so existing tests work.
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
	var dealID int
	switch v := body["id"].(type) {
	case float64:
		dealID = int(v)
	case string:
		fmt.Sscanf(v, "%d", &dealID)
	}
	// Auto-approve the deal so tests that expect active deals work
	server.db.Exec("UPDATE brand_deals SET status = 'active', review_status = 'approved' WHERE id = ?", dealID)
	return dealID
}

// testAdminClerkID is the Clerk user id that newTestServer lists in
// ADMIN_CLERK_USER_IDS. loginAdmin binds it to the admin test user.
const testAdminClerkID = "user_test_admin"

// testAdminPhone is the phone the admin test user is created under. The phone
// grants nothing: admin is decided by the Clerk id alone.
const testAdminPhone = "3104861833"

// loginAdmin logs in the admin user: the user whose Clerk id is in
// config.AdminClerkUserIDs.
func loginAdmin(t *testing.T, server *Server) *http.Cookie {
	t.Helper()
	cookie, _ := loginUser(t, server, testAdminPhone) // upsertUser binds the admin Clerk id
	return cookie
}

// doRequestMux sends a request through a custom http.ServeMux (for sub-route testing).
func doRequestMux(mux *http.ServeMux, method, path string, body interface{}, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// setupEarningsTestData creates a user with brand_deals, swap_events, and swap_rebates
// for earnings tests. Returns (cookie, userID, dealID, rebateID).
func setupEarningsTestData(t *testing.T, server *Server) (*http.Cookie, string, int, int) {
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

// captureLog redirects the standard logger to a buffer for the duration of fn
// and returns everything written.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	fn()
	return buf.String()
}
