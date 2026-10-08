package main

// handlers_clerk_test.go — POST /auth/clerk/exchange, the one mutating endpoint
// in the Rally sign-in path, plus the per-request resolver.
//
// Per CLAUDE.md every mutation needs positive, negative, idempotency and
// auth/tenant-isolation coverage. Every test asserts resulting row counts, not
// just the status code: a handler that returns 200 while writing two users rows
// has still failed.

import (
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// harness
// -----------------------------------------------------------------------

type clerkExchangeHarness struct {
	server *Server
	priv   *rsa.PrivateKey
	jwks   *stubJWKSSource
}

// newClerkExchangeHarness wires a test server to a stub JWKS, with the kill
// switch ON for this test only. Nothing reaches a network.
func newClerkExchangeHarness(t *testing.T) *clerkExchangeHarness {
	t.Helper()
	server := newTestServer(t, &MockTwilioClient{})
	priv := newTestKeypair(t)
	jwks := withClerkSatelliteEnv(t, priv, nil)
	return &clerkExchangeHarness{server: server, priv: priv, jwks: jwks}
}

// token mints a valid token for sub with the given custom claims.
func (h *clerkExchangeHarness) token(t *testing.T, sub string, extra map[string]any) string {
	t.Helper()
	o := defaultClerkTokenOpts()
	o.sub = sub
	o.extra = extra
	return mintClerkTestJWT(t, h.priv, o)
}

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

// assertCounts pins users / households / household_members / clerk
// auth_methods / sessions in one place.
func (h *clerkExchangeHarness) assertCounts(t *testing.T, users, households, members, authMethods, sessions int) {
	t.Helper()
	for _, c := range []struct {
		name  string
		query string
		want  int
	}{
		{"users", "SELECT COUNT(*) FROM users", users},
		{"households", "SELECT COUNT(*) FROM households", households},
		{"household_members", "SELECT COUNT(*) FROM household_members", members},
		{"clerk auth_methods", "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'", authMethods},
		{"sessions", "SELECT COUNT(*) FROM sessions", sessions},
	} {
		if got := h.count(t, c.query); got != c.want {
			t.Errorf("%s rows: got %d want %d", c.name, got, c.want)
		}
	}
}

// assertNothingMinted is the guard every negative test shares: a refused
// exchange must leave the database exactly as it was.
func (h *clerkExchangeHarness) assertNothingMinted(t *testing.T) {
	t.Helper()
	h.assertCounts(t, 0, 0, 0, 0, 0)
}

type userRow struct {
	ID, Phone, Email, Name, ClerkID string
}

func (h *clerkExchangeHarness) userByClerkID(t *testing.T, sub string) userRow {
	t.Helper()
	var u userRow
	err := h.server.db.QueryRow(
		"SELECT id, COALESCE(phone,''), COALESCE(email,''), COALESCE(name,''), COALESCE(clerk_user_id,'') FROM users WHERE clerk_user_id = ?", sub,
	).Scan(&u.ID, &u.Phone, &u.Email, &u.Name, &u.ClerkID)
	if err != nil {
		t.Fatalf("no users row for clerk id %s: %v", sub, err)
	}
	return u
}

// -----------------------------------------------------------------------
// positive
// -----------------------------------------------------------------------

func TestClerkExchange_Positive_NewClerkUser(t *testing.T) {
	h := newClerkExchangeHarness(t)
	rr := h.post(h.token(t, "user_sub_1", map[string]any{"email": "Tamika@Example.com", "name": "Tamika"}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("success: got %v", body["success"])
	}
	for _, gone := range []string{"account_merged", "merge_message"} {
		if _, ok := body[gone]; ok {
			t.Errorf("response must not carry %q: %#v", gone, body)
		}
	}

	h.assertCounts(t, 1, 1, 1, 1, 1)
	u := h.userByClerkID(t, "user_sub_1")
	if body["user_id"] != u.ID {
		t.Errorf("user_id: got %v want %s", body["user_id"], u.ID)
	}
	if len(u.ID) != 36 {
		t.Errorf("users.id must be a locally minted UUID, got %q", u.ID)
	}
	if u.Email != "tamika@example.com" {
		t.Errorf("email: got %q want lowercased claim", u.Email)
	}
	if u.Name != "Tamika" {
		t.Errorf("name: got %q want Tamika", u.Name)
	}
	if u.Phone != "" {
		t.Errorf("phone: got %q want none (no phone claim)", u.Phone)
	}

	var provider, providerID string
	if err := h.server.db.QueryRow("SELECT provider, provider_id FROM auth_methods WHERE user_id = ?", u.ID).Scan(&provider, &providerID); err != nil {
		t.Fatalf("auth_methods: %v", err)
	}
	if provider != "clerk" || providerID != "user_sub_1" {
		t.Errorf("auth_methods: got %s/%s", provider, providerID)
	}

	cookie := getSessionCookie(rr)
	if cookie == nil || !cookie.HttpOnly {
		t.Fatalf("expected an HttpOnly session cookie, got %+v", cookie)
	}
	if sess, ok := h.server.sessions.GetSession(cookie.Value); !ok || sess.UserID != u.ID {
		t.Errorf("cookie must resolve to the new user: %+v ok=%v", sess, ok)
	}
}

func TestClerkExchange_Positive_VerifiedPhoneIsStoredAsTenDigits(t *testing.T) {
	h := newClerkExchangeHarness(t)
	rr := h.post(h.token(t, "user_ph", map[string]any{"phone": "+15551234567", "phone_verified": true}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := h.userByClerkID(t, "user_ph").Phone; got != "5551234567" {
		t.Errorf("phone: got %q want 5551234567", got)
	}
	h.assertCounts(t, 1, 1, 1, 1, 1)
}

func TestClerkExchange_Positive_SessionCarriesCurrentPhone(t *testing.T) {
	h := newClerkExchangeHarness(t)
	rr := h.post(h.token(t, "user_ph2", map[string]any{"phone": "5559990000", "phone_verified": "true"}))
	cookie := getSessionCookie(rr)
	if cookie == nil {
		t.Fatalf("no cookie (status %d)", rr.Code)
	}
	sess, ok := h.server.sessions.GetSession(cookie.Value)
	if !ok || sess.PhoneNumber != "5559990000" {
		t.Errorf("session phone: got %+v ok=%v want 5559990000", sess, ok)
	}
}

func TestClerkExchange_Positive_PhoneNotStoredUnlessVerified(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"verified false", map[string]any{"phone": "+15551234567", "phone_verified": false}},
		{"verified absent", map[string]any{"phone": "+15551234567"}},
		{"verified string false", map[string]any{"phone": "+15551234567", "phone_verified": "false"}},
		{"verified but malformed", map[string]any{"phone": "12345", "phone_verified": true}},
		{"verified but empty", map[string]any{"phone": "", "phone_verified": true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newClerkExchangeHarness(t)
			rr := h.post(h.token(t, "user_np", c.extra))
			if rr.Code != http.StatusOK {
				t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
			}
			if got := h.userByClerkID(t, "user_np").Phone; got != "" {
				t.Errorf("phone must not be stored; got %q", got)
			}
			if got := h.count(t, "SELECT COUNT(*) FROM users WHERE phone IS NOT NULL AND phone != ''"); got != 0 {
				t.Errorf("rows holding a phone: got %d want 0", got)
			}
			h.assertCounts(t, 1, 1, 1, 1, 1)
		})
	}
}

// -----------------------------------------------------------------------
// negative — every case must mint nothing
// -----------------------------------------------------------------------

func TestClerkExchange_Negative_KillSwitchOffIs404(t *testing.T) {
	h := newClerkExchangeHarness(t)
	jwt := h.token(t, "user_x", nil)
	t.Setenv("CLERK_ENABLED", "0")

	rr := h.post(jwt)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("a dark endpoint must 404, not 401: got %d", rr.Code)
	}
	if h.jwks.Lookups() != 0 {
		t.Errorf("no JWKS lookup when disabled; lookups=%d", h.jwks.Lookups())
	}
	h.assertNothingMinted(t)
}

func TestClerkExchange_Negative_WrongMethodIs405(t *testing.T) {
	h := newClerkExchangeHarness(t)
	rr := h.request(http.MethodGet, h.token(t, "user_x", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", rr.Code)
	}
	h.assertNothingMinted(t)
}

func TestClerkExchange_Negative_NoBearerIs401(t *testing.T) {
	h := newClerkExchangeHarness(t)
	rr := h.post("")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	h.assertNothingMinted(t)
}

func TestClerkExchange_Negative_SessionCookieAloneIs401(t *testing.T) {
	// pennyai is cookie-blind: a __session cookie for the apex must not
	// authenticate an exchange here.
	h := newClerkExchangeHarness(t)
	jwt := h.token(t, "user_x", nil)

	rr := h.post("", &http.Cookie{Name: "__session", Value: jwt})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	if h.jwks.Lookups() != 0 {
		t.Errorf("the JWKS must never be consulted for a cookie; lookups=%d", h.jwks.Lookups())
	}
	h.assertNothingMinted(t)
}

func TestClerkExchange_Negative_BadTokensWriteNothing(t *testing.T) {
	cases := []struct {
		name string
		opts func(o *clerkTokenOpts)
	}{
		{"HS256", func(o *clerkTokenOpts) { o.alg = "HS256" }},
		{"alg none", func(o *clerkTokenOpts) { o.alg = "none" }},
		{"wrong issuer", func(o *clerkTokenOpts) { o.iss = "https://clerk.evil.invalid" }},
		{"bad azp", func(o *clerkTokenOpts) { o.azp = "https://attacker.example" }},
		{"expired", func(o *clerkTokenOpts) { o.expDelta = -10 * time.Minute }},
		{"missing exp", func(o *clerkTokenOpts) { o.omitExp = true }},
		{"missing sub", func(o *clerkTokenOpts) { o.omitSub = true }},
		{"missing kid", func(o *clerkTokenOpts) { o.omitKID = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newClerkExchangeHarness(t)
			o := defaultClerkTokenOpts()
			c.opts(&o)

			rr := h.post(mintClerkTestJWT(t, h.priv, o))
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status: got %d want 401 (%s)", rr.Code, rr.Body.String())
			}
			h.assertNothingMinted(t)
			if strings.Contains(strings.ToLower(rr.Body.String()), "jwt") {
				t.Errorf("the error must not describe token internals to the client: %s", rr.Body.String())
			}
		})
	}
}

func TestClerkExchange_Negative_BadSignatureWritesNothing(t *testing.T) {
	h := newClerkExchangeHarness(t)
	other := newTestKeypair(t) // signs with a key the JWKS does not hold
	o := defaultClerkTokenOpts()
	rr := h.post(mintClerkTestJWT(t, other, o))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rr.Code)
	}
	h.assertNothingMinted(t)
}

func TestClerkExchange_Negative_RateLimitedBeforeVerification(t *testing.T) {
	h := newClerkExchangeHarness(t)
	jwt := h.token(t, "user_x", nil)

	// 20 per 10 minutes per IP. The 21st is refused.
	for i := 0; i < 20; i++ {
		h.post(jwt)
	}
	lookupsBefore := h.jwks.Lookups()
	sessionsBefore := h.count(t, "SELECT COUNT(*) FROM sessions")

	rr := h.post(jwt)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("the 21st request in the window must be 429: got %d", rr.Code)
	}
	if h.jwks.Lookups() != lookupsBefore {
		t.Error("the rate limit must be enforced before verification runs")
	}
	if got := h.count(t, "SELECT COUNT(*) FROM sessions"); got != sessionsBefore {
		t.Errorf("a rate-limited request must create no session: %d -> %d", sessionsBefore, got)
	}
}

// -----------------------------------------------------------------------
// idempotency
// -----------------------------------------------------------------------

func TestClerkExchange_Idempotent_SameTokenTwice(t *testing.T) {
	h := newClerkExchangeHarness(t)
	jwt := h.token(t, "user_sub_i", nil)

	first, second := h.post(jwt), h.post(jwt)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses: %d, %d", first.Code, second.Code)
	}
	if parseJSON(first)["user_id"] != parseJSON(second)["user_id"] {
		t.Error("the same token twice must resolve to the same user")
	}
	// Two sign-ins legitimately mean two sessions — that is not a duplicate.
	h.assertCounts(t, 1, 1, 1, 1, 2)
}

func TestClerkExchange_Idempotent_NewTokenSameSubject(t *testing.T) {
	h := newClerkExchangeHarness(t)
	first := h.post(h.token(t, "user_sub_i2", nil))

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
	h.assertCounts(t, 1, 1, 1, 1, 2)
}

func TestClerkExchange_Idempotent_ConcurrentFirstSignIn(t *testing.T) {
	// On Postgres the goroutines truly overlap, and the partial unique index
	// idx_users_clerk_user_id is the backstop. Every response must be a success
	// or a clean, retryable conflict; never a 500, never a second row.
	h := newClerkExchangeHarness(t)
	jwt := h.token(t, "user_sub_c", map[string]any{"email": "c@example.com"})

	const n = 12
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = h.post(jwt).Code
		}(i)
	}
	close(start)
	wg.Wait()

	ok := 0
	for i, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusConflict, http.StatusTooManyRequests:
			// clean, retryable outcome
		default:
			t.Errorf("goroutine %d: got %d — want 200 or a clean retryable outcome, never a 500", i, code)
		}
	}
	if ok == 0 {
		t.Error("no request succeeded")
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 1 {
		t.Errorf("users rows after %d concurrent first sign-ins: got %d want 1", n, got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM households"); got != 1 {
		t.Errorf("households: got %d want 1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM household_members"); got != 1 {
		t.Errorf("household_members: got %d want 1", got)
	}
	if got := h.count(t, "SELECT COUNT(*) FROM auth_methods WHERE provider = 'clerk'"); got != 1 {
		t.Errorf("clerk auth_methods rows: got %d want 1", got)
	}
}

// -----------------------------------------------------------------------
// adoption and isolation
// -----------------------------------------------------------------------

// A phone-only row, as created by an inbound text, is adopted by a Clerk user
// presenting that VERIFIED phone.
func TestClerkExchange_Adoption_PhoneOnlyRowIsBound(t *testing.T) {
	h := newClerkExchangeHarness(t)
	texted, err := upsertUser(h.server.db, "5551110000")
	if err != nil {
		t.Fatalf("seeding phone-only user: %v", err)
	}
	createListItem(t, h.server.db, texted.ID, "texted milk", "Dairy")

	rr := h.post(h.token(t, "user_S1", map[string]any{"phone": "+15551110000", "phone_verified": true}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["user_id"] != texted.ID {
		t.Errorf("user_id: got %v want the texted user %s", parseJSON(rr)["user_id"], texted.ID)
	}
	h.assertCounts(t, 1, 1, 1, 1, 1)
	if got := h.userByClerkID(t, "user_S1").ID; got != texted.ID {
		t.Errorf("the existing row must be bound; got %s", got)
	}

	// The household's list is reachable through the new session.
	list := doRequest(h.server.handleList, http.MethodGet, "/api/list", nil, []*http.Cookie{getSessionCookie(rr)})
	if !strings.Contains(list.Body.String(), "texted milk") {
		t.Error("the adopted account must keep its list")
	}
}

func TestClerkExchange_Adoption_UnverifiedPhoneIsNotAdopted(t *testing.T) {
	h := newClerkExchangeHarness(t)
	texted, err := upsertUser(h.server.db, "5552220000")
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	rr := h.post(h.token(t, "user_S2", map[string]any{"phone": "+15552220000", "phone_verified": false}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	// 2 users, 2 households, 2 members; only the Clerk user has a clerk auth_methods row.
	h.assertCounts(t, 2, 2, 2, 1, 1)
	u := h.userByClerkID(t, "user_S2")
	if u.ID == texted.ID {
		t.Fatal("an unverified phone must never adopt an existing row")
	}
	if u.Phone != "" {
		t.Errorf("the new row must be phone-less; got %q", u.Phone)
	}
	if got := clerkIDOf(t, h.server, texted.ID); got != "" {
		t.Errorf("the texted row must stay unbound; got %q", got)
	}
}

// Phone row already bound to Clerk user A; Clerk user B presents the same
// verified phone. B gets their own phone-less row and A is untouched.
func TestClerkExchange_NoTakeover_BoundPhoneRowIsNeverRebound(t *testing.T) {
	h := newClerkExchangeHarness(t)
	claims := map[string]any{"phone": "+15553330000", "phone_verified": true}

	if rr := h.post(h.token(t, "user_A", claims)); rr.Code != http.StatusOK {
		t.Fatalf("A: status %d (%s)", rr.Code, rr.Body.String())
	}
	a := h.userByClerkID(t, "user_A")
	if a.Phone != "5553330000" {
		t.Fatalf("A should hold the phone; got %q", a.Phone)
	}

	rr := h.post(h.token(t, "user_B", claims))
	if rr.Code != http.StatusOK {
		t.Fatalf("B: status %d (%s)", rr.Code, rr.Body.String())
	}
	b := h.userByClerkID(t, "user_B")
	if b.ID == a.ID {
		t.Fatal("B must not be bound to A's row")
	}
	if b.Phone != "" {
		t.Errorf("B must not receive A's phone; got %q", b.Phone)
	}
	after := h.userByClerkID(t, "user_A")
	if after != a {
		t.Errorf("A's row changed: before %+v after %+v", a, after)
	}
	h.assertCounts(t, 2, 2, 2, 2, 2)
}

// Email is never a matching signal: an existing row with the same email is not
// adopted, whether or not the phone matches nothing.
func TestClerkExchange_EmailIsNeverAMatchingSignal(t *testing.T) {
	h := newClerkExchangeHarness(t)
	if _, err := h.server.db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)", "existing-email-user", "same@example.com", "Old"); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	rr := h.post(h.token(t, "user_E", map[string]any{"email": "same@example.com"}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := h.count(t, "SELECT COUNT(*) FROM users"); got != 2 {
		t.Errorf("users rows: got %d want 2 (a new row, no adoption)", got)
	}
	if got := clerkIDOf(t, h.server, "existing-email-user"); got != "" {
		t.Errorf("the email-matching row must stay unbound; got %q", got)
	}
	if h.userByClerkID(t, "user_E").ID == "existing-email-user" {
		t.Error("email must never adopt a row")
	}
}

// Adoption must not open a cross-user data path.
func TestClerkExchange_Isolation_NoCrossUserDataAccess(t *testing.T) {
	h := newClerkExchangeHarness(t)
	_, userAID := loginUser(t, h.server, "5556660000")
	createListItem(t, h.server.db, userAID, "A milk", "Dairy")

	rr := h.post(h.token(t, "user_B", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d (%s)", rr.Code, rr.Body.String())
	}
	cookieB := getSessionCookie(rr)
	if cookieB == nil {
		t.Fatal("no session cookie for B")
	}
	createListItem(t, h.server.db, h.userByClerkID(t, "user_B").ID, "B bread", "Bakery")

	listB := doRequest(h.server.handleList, http.MethodGet, "/api/list", nil, []*http.Cookie{cookieB})
	if listB.Code != http.StatusOK {
		t.Fatalf("B's list: got %d", listB.Code)
	}
	if strings.Contains(listB.Body.String(), "A milk") {
		t.Error("B can see A's item")
	}
	if !strings.Contains(listB.Body.String(), "B bread") {
		t.Error("B cannot see their own item")
	}
}

func clerkIDOf(t *testing.T, s *Server, userID string) string {
	t.Helper()
	var v string
	if err := s.db.QueryRow("SELECT COALESCE(clerk_user_id,'') FROM users WHERE id = ?", userID).Scan(&v); err != nil {
		t.Fatalf("reading clerk_user_id for %s: %v", userID, err)
	}
	return v
}

// -----------------------------------------------------------------------
// resolveClerkSession — the per-request, read-only resolver
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
		server := newTestServer(t, &MockTwilioClient{})
		var before int
		server.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&before)
		server.resolveClerkSession(ClerkIdentity{ClerkUserID: "user_nobody", Email: "a@b.com", Phone: "5551234567", PhoneVerified: true})
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
