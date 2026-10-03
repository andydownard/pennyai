package main

// handlers_clerk_test.go — STEP 6: POST /auth/clerk/exchange, the one mutating
// endpoint in the Rally sign-in path, plus the upsert beneath it.
//
// Per CLAUDE.md every mutation needs positive, negative, idempotency and
// auth/tenant-isolation coverage. The isolation suite is the important one here:
// the exchange can ADOPT an existing legacy account, so the tests pin the
// mapping order (clerk → google_sub → phone → email), prove a row already bound
// to one Clerk identity is never rebound to another, and prove no cross-user
// data becomes visible.
//
// Every test asserts the resulting row counts, not just the status code: a
// handler that returns 200 while writing two users rows has still failed.

import (
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// -----------------------------------------------------------------------
// harness
// -----------------------------------------------------------------------

type clerkExchangeHarness struct {
	server *Server
	priv   *rsa.PrivateKey
	stub   *stubTimpager
	jwks   *stubJWKSSource
}

// newClerkExchangeHarness wires a test server to a stub timpager and a stub
// JWKS, with the kill switch ON for this test only. Nothing reaches a network.
func newClerkExchangeHarness(t *testing.T, resolveBody string, resolveStatus int) *clerkExchangeHarness {
	t.Helper()
	server := newTestServer(t, &MockTwilioClient{})
	priv := newTestKeypair(t)
	jwks := withClerkSatelliteEnv(t, priv, nil)
	stub := newStubTimpagerResolve(t, resolveBody, resolveStatus)
	useStubTimpager(server, stub, "svc_test_key")
	return &clerkExchangeHarness{server: server, priv: priv, stub: stub, jwks: jwks}
}

// post sends a POST to the exchange with the given bearer token (empty = none).
func (h *clerkExchangeHarness) post(bearer string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	return h.request(http.MethodPost, bearer, cookies...)
}

func (h *clerkExchangeHarness) request(method, bearer string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/auth/clerk/exchange", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.server.handleClerkExchange(rr, req)
	return rr
}

func (h *clerkExchangeHarness) count(t *testing.T, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := h.server.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("counting (%s): %v", query, err)
	}
	return n
}

// assertNothingMinted is the guard every negative test shares: a refused
// exchange must leave the database exactly as it was.
func (h *clerkExchangeHarness) assertNothingMinted(t *testing.T, expectedUsers int) {
	t.Helper()
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != expectedUsers {
		t.Errorf("users rows: got %d want %d", got, expectedUsers)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'"); got != 0 {
		t.Errorf("clerk auth_methods rows: got %d want 0", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM sessions"); got != 0 {
		t.Errorf("sessions rows: got %d want 0", got)
	}
}

func clerkUserIDOf(t *testing.T, s *Server, userID string) string {
	t.Helper()
	var v string
	if err := s.db.QueryRow("SELECT COALESCE(clerk_user_id,'') FROM users WHERE id = ?", userID).Scan(&v); err != nil {
		t.Fatalf("reading clerk_user_id for %s: %v", userID, err)
	}
	return v
}

// -----------------------------------------------------------------------
// positive
// -----------------------------------------------------------------------

func TestClerkExchange_Positive_NewRallyUser(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-user-R", "user_sub_1", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_sub_1")

	rr := h.post(jwt)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("success: got %v", body["success"])
	}
	if body["user_id"] != "rally-user-R" {
		t.Errorf("user_id: got %v want rally-user-R — users.id must BE the rally_user_id", body["user_id"])
	}
	if _, ok := body["account_merged"]; ok {
		t.Errorf("a brand-new account is not a merge; got %#v", body)
	}

	// Session cookie: the normal pennyai session, HttpOnly.
	cookie := getSessionCookie(rr)
	if cookie == nil {
		t.Fatal(`no "session" cookie set — the exchange must mint the normal pennyai session`)
	}
	if !cookie.HttpOnly {
		t.Error("the session cookie must be HttpOnly")
	}

	// Rows.
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows: got %d want 1", got)
	}
	if got := clerkUserIDOf(t, h.server, "rally-user-R"); got != "user_sub_1" {
		t.Errorf("users.clerk_user_id: got %q want user_sub_1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk' AND provider_id = ?", "user_sub_1"); got != 1 {
		t.Errorf("clerk auth_methods rows: got %d want 1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM households"); got != 1 {
		t.Errorf("households: got %d want 1 (every new user gets a solo household)", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM household_members WHERE user_id = ?", "rally-user-R"); got != 1 {
		t.Errorf("household_members: got %d want 1", got)
	}

	// The minted cookie works on the existing surfaces.
	me := doRequest(h.server.handleMe, http.MethodGet, "/auth/me", nil, []*http.Cookie{cookie})
	if me.Code != http.StatusOK {
		t.Fatalf("/auth/me with the exchange cookie: got %d want 200", me.Code)
	}
	meBody := parseJSON(me)
	if meBody["logged_in"] != true {
		t.Errorf("/auth/me logged_in: got %v", meBody["logged_in"])
	}
	// `auth_methods` is the array built from the auth_methods table, so "clerk"
	// has to appear there. (The scalar `auth_method` field reads sessions.role,
	// which CreateSession has always hardcoded to "user" for every auth method
	// including Google — pre-existing, and not this change's business to alter.)
	methods, _ := meBody["auth_methods"].([]interface{})
	found := false
	for _, m := range methods {
		if m == "clerk" {
			found = true
		}
	}
	if !found {
		t.Errorf(`/auth/me auth_methods should include "clerk"; got %#v`, methods)
	}

	// The authority was called with the right header discipline.
	reqs := h.stub.Requests()
	if len(reqs) != 1 {
		t.Fatalf("timpager calls: got %d want 1", len(reqs))
	}
	if reqs[0].APIKeyHeader != "svc_test_key" {
		t.Errorf("X-API-Key: got %q", reqs[0].APIKeyHeader)
	}
	if reqs[0].Authorization != "Bearer "+jwt {
		t.Error("Authorization must forward the same end-user Clerk JWT the client presented")
	}
}

func TestClerkExchange_Positive_PhoneFromAuthorityIsStored(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-user-P", "user_sub_p", "", "+15557778888"), http.StatusOK)

	if rr := h.post(mintValidClerkJWT(t, h.priv, "user_sub_p")); rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	var phone string
	h.server.db.QueryRow("SELECT COALESCE(phone,'') FROM users WHERE id = ?", "rally-user-P").Scan(&phone)
	if phone != "5557778888" {
		t.Errorf("users.phone: got %q want 5557778888 (10-digit storage form)", phone)
	}
}

// -----------------------------------------------------------------------
// negative — every case must mint nothing
// -----------------------------------------------------------------------

func TestClerkExchange_Negative_KillSwitchOffIs404(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_x")
	t.Setenv("CLERK_ENABLED", "0")

	rr := h.post(jwt)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("a dark endpoint must 404, not 401 or 503: got %d", rr.Code)
	}
	if h.stub.Calls() != 0 {
		t.Errorf("the authority must not be called when disabled; calls=%d", h.stub.Calls())
	}
	if h.jwks.Lookups() != 0 {
		t.Errorf("no JWKS lookup when disabled; lookups=%d", h.jwks.Lookups())
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_WrongMethodIs405(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	rr := h.request(http.MethodGet, mintValidClerkJWT(t, h.priv, "user_x"))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", rr.Code)
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_NoBearerIs401(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	rr := h.post("")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_SessionCookieAloneIs401(t *testing.T) {
	// pennyai is cookie-blind: a __session cookie for the apex must not
	// authenticate an exchange here.
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_x")

	rr := h.post("", &http.Cookie{Name: "__session", Value: jwt})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	if h.jwks.Lookups() != 0 {
		t.Errorf("the JWKS must never be consulted for a cookie; lookups=%d", h.jwks.Lookups())
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_BadTokensAreRefusedBeforeAnyCall(t *testing.T) {
	cases := []struct {
		name string
		opts func(o *clerkTokenOpts)
	}{
		{"HS256", func(o *clerkTokenOpts) { o.alg = "HS256" }},
		{"alg none", func(o *clerkTokenOpts) { o.alg = "none" }},
		{"wrong issuer", func(o *clerkTokenOpts) { o.iss = "https://clerk.evil.invalid" }},
		{"bad azp", func(o *clerkTokenOpts) { o.azp = "https://attacker.example" }},
		{"expired", func(o *clerkTokenOpts) { o.expDelta = -10 * 60 * 1000000000 }},
		{"missing exp", func(o *clerkTokenOpts) { o.omitExp = true }},
		{"missing sub", func(o *clerkTokenOpts) { o.omitSub = true }},
		{"missing kid", func(o *clerkTokenOpts) { o.omitKID = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
			o := defaultClerkTokenOpts()
			c.opts(&o)

			rr := h.post(mintClerkTestJWT(t, h.priv, o))
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d want 401 (%s)", rr.Code, rr.Body.String())
			}
			if h.stub.Calls() != 0 {
				t.Errorf("the authority must not be called for an unverified token; calls=%d", h.stub.Calls())
			}
			h.assertNothingMinted(t, 0)
			if strings.Contains(rr.Body.String(), "jwt") || strings.Contains(rr.Body.String(), "JWT") {
				t.Errorf("the error must not describe token internals to the client: %s", rr.Body.String())
			}
		})
	}
}

func TestClerkExchange_Negative_AuthorityRejectionIs401(t *testing.T) {
	h := newClerkExchangeHarness(t, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_x"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_AuthorityUnavailableIs503(t *testing.T) {
	h := newClerkExchangeHarness(t, `{"error":"boom"}`, http.StatusBadGateway)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_x"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unreachable authority is a 503, not a 401 or a 500: got %d", rr.Code)
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_AuthorityNotConfiguredIs503(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	h.server.config.TimpagerAPIKey = "" // no service key on this deploy
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_x"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", rr.Code)
	}
	if h.stub.Calls() != 0 {
		t.Errorf("no HTTP call without the service key; calls=%d", h.stub.Calls())
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_NoRallyIdentityAndNoLocalMatchIs409(t *testing.T) {
	// rally_user_id null and nothing local to adopt: we must NOT mint a locally
	// generated UUID, because users.id has to equal the rally_user_id.
	h := newClerkExchangeHarness(t, resolveOKBody("", "user_orphan", "", ""), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_orphan"))
	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d want 409 (%s)", rr.Code, rr.Body.String())
	}
	msg, _ := parseJSON(rr)["error"].(string)
	for _, banned := range []string{"SSO", "bot", "AI", "assistant", "rally_user_id", "null"} {
		if strings.Contains(msg, banned) {
			t.Errorf("user-facing copy must not contain %q: %q", banned, msg)
		}
	}
	h.assertNothingMinted(t, 0)
}

func TestClerkExchange_Negative_RateLimitedBeforeVerification(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-x", "user_x", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_x")

	// 20 per 10 minutes per IP. The 21st is refused.
	for i := 0; i < 20; i++ {
		h.post(jwt)
	}
	lookupsBefore := h.jwks.Lookups()
	callsBefore := h.stub.Calls()

	rr := h.post(jwt)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("the 21st request in the window must be 429: got %d", rr.Code)
	}
	if h.jwks.Lookups() != lookupsBefore {
		t.Error("the rate limit must be enforced before verification runs")
	}
	if h.stub.Calls() != callsBefore {
		t.Error("the rate limit must be enforced before the authority is called")
	}
}

// -----------------------------------------------------------------------
// idempotency
// -----------------------------------------------------------------------

func TestClerkExchange_Idempotent_SameTokenTwice(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-user-I", "user_sub_i", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_sub_i")

	first := h.post(jwt)
	second := h.post(jwt)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("both exchanges must succeed: %d, %d", first.Code, second.Code)
	}
	if parseJSON(first)["user_id"] != parseJSON(second)["user_id"] {
		t.Error("a repeat sign-in must resolve to the same user")
	}

	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows: got %d want 1 — a second sign-in must not create a second account", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'"); got != 1 {
		t.Errorf("clerk auth_methods rows: got %d want 1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM households"); got != 1 {
		t.Errorf("households: got %d want 1", got)
	}
	// Two sign-ins legitimately mean two sessions — that is not a duplicate.
	if got := h.count(t, "SELECT COUNT(*) FROM sessions WHERE user_id = ?", "rally-user-I"); got != 2 {
		t.Errorf("sessions: got %d want 2 (one per sign-in)", got)
	}
}

func TestClerkExchange_Idempotent_NewSessionSameSubject(t *testing.T) {
	h := newClerkExchangeHarness(t, resolveOKBody("rally-user-I2", "user_sub_i2", "", ""), http.StatusOK)

	first := h.post(mintValidClerkJWT(t, h.priv, "user_sub_i2"))

	// A fresh Clerk session for the same person: new sid, same sub.
	o := defaultClerkTokenOpts()
	o.sub = "user_sub_i2"
	o.sid = "sess_rotated_2"
	second := h.post(mintClerkTestJWT(t, h.priv, o))

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses: %d, %d", first.Code, second.Code)
	}
	if parseJSON(first)["user_id"] != parseJSON(second)["user_id"] {
		t.Error("a new Clerk session for the same subject must resolve to the same user")
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows: got %d want 1", got)
	}
}

func TestClerkExchange_Idempotent_ConcurrentFirstSignIn(t *testing.T) {
	// NOTE: the Go suite runs on a single-connection in-memory SQLite through
	// pgcompat, so these goroutines are serialized at the driver and this cannot
	// prove true concurrency. On Postgres the partial unique index
	// idx_users_clerk_user_id is the real backstop; this test pins the contract
	// (every response is a success or a clean conflict, never a second row) and
	// exercises the conflict-retry path.
	h := newClerkExchangeHarness(t, resolveOKBody("rally-user-C", "user_sub_c", "", ""), http.StatusOK)
	jwt := mintValidClerkJWT(t, h.priv, "user_sub_c")

	const n = 10
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			codes[i] = h.post(jwt).Code
		}(i)
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK && code != http.StatusConflict {
			t.Errorf("goroutine %d: got %d — want 200 or 409, never a 500 or a duplicate", i, code)
		}
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows after %d concurrent first sign-ins: got %d want 1", n, got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'"); got != 1 {
		t.Errorf("clerk auth_methods rows: got %d want 1", got)
	}
}

// -----------------------------------------------------------------------
// auth / tenant isolation — the mapping order and the takeover guard
// -----------------------------------------------------------------------

// A. A legacy phone user is adopted, not duplicated.
func TestClerkExchange_Isolation_A_AdoptsLegacyPhoneUser(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	legacy, err := upsertUser(h.server.db, "5551110000")
	if err != nil {
		t.Fatalf("seeding legacy phone user: %v", err)
	}
	h.stub.SetResponse(resolveOKBody("rally-new-id", "user_S1", "", "+15551110000"), http.StatusOK)

	rr := h.post(mintValidClerkJWT(t, h.priv, "user_S1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["user_id"] != legacy.ID {
		t.Errorf("user_id: got %v want the legacy user %s — a phone match must adopt, not mint", body["user_id"], legacy.ID)
	}
	if body["account_merged"] != true {
		t.Errorf("account_merged should be true when an existing account was adopted; got %#v", body)
	}
	if msg, _ := body["merge_message"].(string); msg == "" {
		t.Error("a merge should carry a message the UI can show")
	}
	if got := clerkUserIDOf(t, h.server, legacy.ID); got != "user_S1" {
		t.Errorf("legacy row clerk_user_id: got %q want user_S1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows: got %d want 1 (no second account)", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users WHERE id = ?", "rally-new-id"); got != 0 {
		t.Error("no row should be created under the rally_user_id when a legacy row was adopted")
	}
}

// B. The takeover guard: a row bound to one Clerk identity is never rebound.
func TestClerkExchange_Isolation_B_NoTakeoverOfBoundAccount(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	legacy, err := upsertUser(h.server.db, "5552220000")
	if err != nil {
		t.Fatalf("seeding legacy user: %v", err)
	}

	// First sign-in binds S1.
	h.stub.SetResponse(resolveOKBody("rally-b1", "user_S1", "", "+15552220000"), http.StatusOK)
	if rr := h.post(mintValidClerkJWT(t, h.priv, "user_S1")); rr.Code != http.StatusOK {
		t.Fatalf("first sign-in: got %d (%s)", rr.Code, rr.Body.String())
	}
	sessionsAfterFirst := h.count(t, "SELECT COUNT(*) FROM sessions")

	// A DIFFERENT Clerk identity now claims the same phone.
	h.stub.SetResponse(resolveOKBody("rally-b2", "user_S2", "", "+15552220000"), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_S2"))
	if rr.Code != http.StatusConflict {
		t.Fatalf("a second Clerk identity claiming a bound account must be 409: got %d (%s)", rr.Code, rr.Body.String())
	}

	if got := clerkUserIDOf(t, h.server, legacy.ID); got != "user_S1" {
		t.Errorf("the binding was overwritten: clerk_user_id is now %q, want user_S1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows: got %d want 1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM sessions"); got != sessionsAfterFirst {
		t.Errorf("a refused exchange must not mint a session: got %d want %d", got, sessionsAfterFirst)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk' AND provider_id = ?", "user_S2"); got != 0 {
		t.Errorf("no auth_methods row for the refused identity; got %d", got)
	}
	if getSessionCookie(rr) != nil {
		t.Error("a 409 must not set a session cookie")
	}
}

// C. google_sub beats phone.
func TestClerkExchange_Isolation_C_GoogleSubBeatsPhone(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	google, err := resolveOrCreateUser(h.server.db, "google", "g1", "e@x.com", "Google User")
	if err != nil {
		t.Fatalf("seeding google user: %v", err)
	}
	phoneUser, err := upsertUser(h.server.db, "5553330000")
	if err != nil {
		t.Fatalf("seeding phone user: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-c", "user_S3", "", "+15553330000"), http.StatusOK)
	o := defaultClerkTokenOpts()
	o.sub = "user_S3"
	o.extra = map[string]any{"google_sub": "g1"}

	rr := h.post(mintClerkTestJWT(t, h.priv, o))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != google.ID {
		t.Errorf("user_id: got %v want the google user %s — google_sub must outrank phone", parseJSON(rr)["user_id"], google.ID)
	}
	if got := clerkUserIDOf(t, h.server, phoneUser.ID); got != "" {
		t.Errorf("the phone user must be untouched; its clerk_user_id is %q", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 2 {
		t.Errorf("users rows: got %d want 2 (no new account)", got)
	}
}

// D. phone beats email.
func TestClerkExchange_Isolation_D_PhoneBeatsEmail(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	phoneUser, err := upsertUser(h.server.db, "5554440000")
	if err != nil {
		t.Fatalf("seeding phone user: %v", err)
	}
	emailUser, err := resolveOrCreateUser(h.server.db, "email", "e@x.com", "e@x.com", "Email User")
	if err != nil {
		t.Fatalf("seeding email user: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-d", "user_S4", "e@x.com", "+15554440000"), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_S4"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != phoneUser.ID {
		t.Errorf("user_id: got %v want the phone user %s — phone must outrank email", parseJSON(rr)["user_id"], phoneUser.ID)
	}
	if got := clerkUserIDOf(t, h.server, emailUser.ID); got != "" {
		t.Errorf("the email user must be untouched; its clerk_user_id is %q", got)
	}
}

// E. A Clerk account with neither phone nor email still works.
func TestClerkExchange_Isolation_E_NoPhoneNoEmailMintsCleanly(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	legacy, err := upsertUser(h.server.db, "5555550000")
	if err != nil {
		t.Fatalf("seeding legacy user: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-e-new", "user_S5", "", ""), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_S5"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != "rally-e-new" {
		t.Errorf("user_id: got %v want rally-e-new", parseJSON(rr)["user_id"])
	}
	if got := clerkUserIDOf(t, h.server, legacy.ID); got != "" {
		t.Errorf("the unrelated legacy user must be untouched; its clerk_user_id is %q", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 2 {
		t.Errorf("users rows: got %d want 2", got)
	}
}

// F. Adoption does not open a cross-user data path.
func TestClerkExchange_Isolation_F_NoCrossUserDataAccess(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	userX, err := upsertUser(h.server.db, "5556660000")
	if err != nil {
		t.Fatalf("seeding user X: %v", err)
	}
	cookieY, userYID := loginUser(t, h.server, "5557770000")
	createListItem(t, h.server.db, userX.ID, "X milk", "Dairy")
	createListItem(t, h.server.db, userYID, "Y bread", "Bakery")

	h.stub.SetResponse(resolveOKBody("rally-f", "user_SF", "", "+15556660000"), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_SF"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	cookieX := getSessionCookie(rr)
	if cookieX == nil {
		t.Fatal("no session cookie")
	}

	listX := doRequest(h.server.handleList, http.MethodGet, "/api/list", nil, []*http.Cookie{cookieX})
	if listX.Code != http.StatusOK {
		t.Fatalf("X's list: got %d", listX.Code)
	}
	if strings.Contains(listX.Body.String(), "Y bread") {
		t.Error("X can see Y's item — the Clerk session must be scoped exactly like any other")
	}
	if !strings.Contains(listX.Body.String(), "X milk") {
		t.Error("X cannot see their own item")
	}

	listY := doRequest(h.server.handleList, http.MethodGet, "/api/list", nil, []*http.Cookie{cookieY})
	if strings.Contains(listY.Body.String(), "X milk") {
		t.Error("Y can see X's item")
	}
}

// G. Soft-deleted rows are never adopted by an email match.
func TestClerkExchange_Isolation_G_SoftDeletedRowNotAdopted(t *testing.T) {
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	if _, err := h.server.db.Exec(
		"INSERT INTO users (id, email, name) VALUES (?, ?, ?)",
		"soft-deleted-1", "ghost@x.com", "[merged into user abcdef]",
	); err != nil {
		t.Fatalf("seeding soft-deleted row: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-g-new", "user_SG", "ghost@x.com", ""), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_SG"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != "rally-g-new" {
		t.Errorf("user_id: got %v want a fresh row — a soft-deleted account must never be adopted", parseJSON(rr)["user_id"])
	}
	if got := clerkUserIDOf(t, h.server, "soft-deleted-1"); got != "" {
		t.Errorf("the soft-deleted row was bound: clerk_user_id %q", got)
	}
}

func TestClerkExchange_Isolation_G2_SoftDeletedAuthMethodNotAdopted(t *testing.T) {
	// The same exclusion must hold for the auth_methods email fallback, not just
	// the users lookup — otherwise a soft-deleted account could be adopted via
	// its auth_methods row one query later.
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	ghost, err := resolveOrCreateUser(h.server.db, "email", "ghost2@x.com", "ghost2@x.com", "Ghost")
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	// Soft-delete it the way mergeUserData does, leaving auth_methods in place.
	if _, err := h.server.db.Exec(
		"UPDATE users SET name = ?, email = '' WHERE id = ?",
		"[merged into user other-id]", ghost.ID,
	); err != nil {
		t.Fatalf("soft-deleting: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-g2-new", "user_SG2", "ghost2@x.com", ""), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_SG2"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != "rally-g2-new" {
		t.Errorf("user_id: got %v want a fresh row", parseJSON(rr)["user_id"])
	}
	if got := clerkUserIDOf(t, h.server, ghost.ID); got != "" {
		t.Errorf("the soft-deleted account was adopted via auth_methods: clerk_user_id %q", got)
	}
}

func TestClerkExchange_Isolation_EmailAdoptionViaAuthMethods(t *testing.T) {
	// The email fallback also looks at auth_methods, so an account whose email
	// lives only there is still recognized rather than duplicated.
	h := newClerkExchangeHarness(t, "", http.StatusOK)
	existing, err := resolveOrCreateUser(h.server.db, "email", "only-in-auth@x.com", "", "No Users Email")
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	h.stub.SetResponse(resolveOKBody("rally-h", "user_SH", "ONLY-IN-AUTH@x.com", ""), http.StatusOK)
	rr := h.post(mintValidClerkJWT(t, h.priv, "user_SH"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != existing.ID {
		t.Errorf("user_id: got %v want %s — email matching must be case-insensitive and cover auth_methods", parseJSON(rr)["user_id"], existing.ID)
	}
}

// -----------------------------------------------------------------------
// resolveClerkSession — the lookup-only per-request resolver
// -----------------------------------------------------------------------

func TestResolveClerkSession(t *testing.T) {
	t.Run("unknown clerk identity is anonymous", func(t *testing.T) {
		server := newTestServer(t, &MockTwilioClient{})
		sess, err := server.resolveClerkSession(ClerkIdentity{ClerkUserID: "user_nobody"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sess != nil {
			t.Errorf("want nil session for an unknown Clerk identity, got %+v", sess)
		}
	})

	t.Run("known identity yields a clerk session", func(t *testing.T) {
		server := newTestServer(t, &MockTwilioClient{})
		if _, err := server.db.Exec(
			"INSERT INTO users (id, phone, clerk_user_id) VALUES (?, ?, ?)",
			"local-1", "5551234567", "user_known",
		); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		sess, err := server.resolveClerkSession(ClerkIdentity{ClerkUserID: "user_known"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sess == nil {
			t.Fatal("want a session")
		}
		if sess.UserID != "local-1" || sess.PhoneNumber != "5551234567" || sess.AuthMethod != "clerk" {
			t.Errorf("session: got %+v", sess)
		}
	})

	t.Run("it never provisions", func(t *testing.T) {
		// The per-request resolver must be read-only: provisioning happens only
		// at the exchange, which has the authority's answer in hand.
		server := newTestServer(t, &MockTwilioClient{})
		var before int
		server.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&before)
		server.resolveClerkSession(ClerkIdentity{ClerkUserID: "user_nobody", Email: "a@b.com", Phone: "5551234567"})
		var after int
		server.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&after)
		if before != after {
			t.Errorf("resolveClerkSession created rows: %d → %d", before, after)
		}
	})

	t.Run("empty clerk id is anonymous", func(t *testing.T) {
		server := newTestServer(t, &MockTwilioClient{})
		sess, err := server.resolveClerkSession(ClerkIdentity{})
		if err != nil || sess != nil {
			t.Fatalf("want (nil, nil); got %+v, %v", sess, err)
		}
	})
}
