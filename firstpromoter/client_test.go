package firstpromoter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =====================================================================
// Test scaffolding
// =====================================================================

// withEnv installs the standard "kill switch on + valid API key" env block
// for the duration of one subtest, redirects FIRSTPROMOTER_API_BASE to the
// test server, and registers a cleanup hook that restores prior state. Each
// test must own its env mutations to keep `go test -race` honest.
func withEnv(t *testing.T, ts *httptest.Server, extra map[string]string) {
	t.Helper()
	defaults := map[string]string{
		"FIRSTPROMOTER_ENABLED":     "1",
		"FIRSTPROMOTER_API_KEY":     "fp_test_key_abc123",
		"FIRSTPROMOTER_API_BASE":    ts.URL,
		"FIRSTPROMOTER_CAMPAIGN_ID": "camp_test_42",
	}
	for k, v := range extra {
		defaults[k] = v
	}
	for k, v := range defaults {
		t.Setenv(k, v)
	}
}

// recordingHandler captures the most recent request the handler received so
// individual tests can assert on path, headers, and body. Concurrency-safe.
type recordingHandler struct {
	mu          sync.Mutex
	method      string
	path        string
	rawQuery    string
	authHeader  string
	contentType string
	body        []byte
	calls       int32

	// Configurable response.
	statusCode int
	respBody   string
	respDelay  time.Duration
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.calls, 1)
	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	h.method = r.Method
	h.path = r.URL.Path
	h.rawQuery = r.URL.RawQuery
	h.authHeader = r.Header.Get("Authorization")
	h.contentType = r.Header.Get("Content-Type")
	h.body = body
	h.mu.Unlock()

	if h.respDelay > 0 {
		time.Sleep(h.respDelay)
	}
	// Set content-type BEFORE WriteHeader — once WriteHeader is called the
	// header map is frozen for the response. Without this the parseBody path
	// in the client treats the body as a string instead of JSON.
	w.Header().Set("Content-Type", "application/json")
	if h.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(h.statusCode)
	}
	if h.respBody != "" {
		_, _ = w.Write([]byte(h.respBody))
	}
}

func (h *recordingHandler) snapshot() (string, string, string, string, []byte, int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.method, h.path, h.authHeader, h.contentType, append([]byte(nil), h.body...), atomic.LoadInt32(&h.calls)
}

// multiRouteHandler dispatches based on (method, path). Used when one test
// needs the test server to answer multiple endpoints (e.g., 409 → find).
type multiRouteHandler struct {
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	calls  []string // ordered list of "METHOD PATH" strings, useful for ordering assertions
}

func newMultiRouteHandler() *multiRouteHandler {
	return &multiRouteHandler{routes: map[string]http.HandlerFunc{}}
}

func (h *multiRouteHandler) handle(method, path string, fn http.HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.routes[method+" "+path] = fn
}

func (h *multiRouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
	fn, ok := h.routes[r.Method+" "+r.URL.Path]
	h.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, "no handler for %s %s", r.Method, r.URL.Path)
		return
	}
	fn(w, r)
}

func (h *multiRouteHandler) callOrder() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.calls))
	copy(out, h.calls)
	return out
}

// =====================================================================
// Kill switch
// =====================================================================

func TestProvisionPromoter_KillSwitchOff(t *testing.T) {
	t.Setenv("FIRSTPROMOTER_ENABLED", "0")
	t.Setenv("FIRSTPROMOTER_API_KEY", "irrelevant")

	_, err := ProvisionPromoter(context.Background(), "user-1", "u@example.com", "", "")
	if !errors.Is(err, ErrFirstPromoterDisabled) {
		t.Fatalf("expected ErrFirstPromoterDisabled, got: %v", err)
	}
}

func TestAllPublicFunctions_KillSwitchOff(t *testing.T) {
	t.Setenv("FIRSTPROMOTER_ENABLED", "")
	t.Setenv("FIRSTPROMOTER_API_KEY", "irrelevant")

	cases := []struct {
		name string
		fn   func() error
	}{
		{"MintIframeToken", func() error {
			_, err := MintIframeToken(context.Background(), "u", "")
			return err
		}},
		{"GetPromoter", func() error {
			_, err := GetPromoter(context.Background(), "u")
			return err
		}},
		{"TrackSignup", func() error {
			return TrackSignup(context.Background(), "ref", "u@example.com", "")
		}},
		{"ListPromoters", func() error {
			_, _, err := ListPromoters(context.Background(), 1, 10)
			return err
		}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := c.fn()
			if !errors.Is(err, ErrFirstPromoterDisabled) {
				t.Fatalf("%s: expected ErrFirstPromoterDisabled, got: %v", c.name, err)
			}
		})
	}
}

// =====================================================================
// API key requirements
// =====================================================================

func TestProvisionPromoter_MissingAPIKeyReturnsError(t *testing.T) {
	t.Setenv("FIRSTPROMOTER_ENABLED", "1")
	t.Setenv("FIRSTPROMOTER_API_KEY", "")

	_, err := ProvisionPromoter(context.Background(), "user-1", "u@example.com", "", "")
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "FIRSTPROMOTER_API_KEY") {
		t.Fatalf("error must name the missing var: %v", err)
	}
	// MUST NOT be the kill-switch sentinel — that would mislead ops.
	if errors.Is(err, ErrFirstPromoterDisabled) {
		t.Fatal("missing-key error must not collapse to ErrFirstPromoterDisabled")
	}
}

// =====================================================================
// ProvisionPromoter — happy path + 409 fallback
// =====================================================================

func TestProvisionPromoter_HappyPath(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusCreated,
		respBody: `{
			"id": 12345,
			"cust_id": "user-1",
			"email": "u@example.com",
			"default_ref_id": "abc-token",
			"default_promotion": {
				"referral_link": "https://affiliates.rallytasks.com/r/abc-token",
				"recruitment_link": "https://affiliates.rallytasks.com/recruit/abc-token"
			}
		}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	prom, err := ProvisionPromoter(context.Background(), "user-1", "u@example.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prom == nil {
		t.Fatal("expected promoter, got nil")
	}
	if prom.PromoterID != "12345" {
		t.Errorf("PromoterID: got %q want %q", prom.PromoterID, "12345")
	}
	if prom.RefToken != "abc-token" {
		t.Errorf("RefToken: got %q want %q", prom.RefToken, "abc-token")
	}
	if prom.ReferralLink == "" {
		t.Error("ReferralLink should be populated")
	}
	if prom.RecruitmentLink == "" {
		t.Error("RecruitmentLink should be populated")
	}
	if prom.AlreadyExisted {
		t.Error("AlreadyExisted should be false on fresh provision")
	}

	// Verify outgoing request shape.
	method, path, auth, ct, body, calls := h.snapshot()
	if calls != 1 {
		t.Errorf("calls: got %d want 1", calls)
	}
	if method != http.MethodPost {
		t.Errorf("method: got %q want POST", method)
	}
	if path != EndpointCreatePromoter {
		t.Errorf("path: got %q want %q", path, EndpointCreatePromoter)
	}
	if auth != "Bearer fp_test_key_abc123" {
		t.Errorf("auth header: got %q", auth)
	}
	if ct != "application/json" {
		t.Errorf("content-type: got %q", ct)
	}

	var sent map[string]interface{}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sent["cust_id"] != "user-1" {
		t.Errorf("cust_id: got %v", sent["cust_id"])
	}
	if sent["email"] != "u@example.com" {
		t.Errorf("email: got %v", sent["email"])
	}
	if sent["initial_campaign_id"] != "camp_test_42" {
		t.Errorf("initial_campaign_id should default from env, got %v", sent["initial_campaign_id"])
	}
}

func TestProvisionPromoter_PassesRefIDAndCampaignIDOverrides(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusCreated,
		respBody:   `{"id": 1, "default_ref_id": "x"}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	_, err := ProvisionPromoter(context.Background(), "user-9", "x@example.com", "my-slug", "camp_override_99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, _, _, body, _ := h.snapshot()
	var sent map[string]interface{}
	_ = json.Unmarshal(body, &sent)
	if sent["ref_id"] != "my-slug" {
		t.Errorf("ref_id: got %v want my-slug", sent["ref_id"])
	}
	if sent["initial_campaign_id"] != "camp_override_99" {
		t.Errorf("campaign override should win over env, got %v", sent["initial_campaign_id"])
	}
}

func TestProvisionPromoter_409FallbackToFind(t *testing.T) {
	router := newMultiRouteHandler()
	router.handle(http.MethodPost, EndpointCreatePromoter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error": "Promoter with this cust_id already exists"}`))
	})
	router.handle(http.MethodGet, EndpointFindPromoter, func(w http.ResponseWriter, r *http.Request) {
		// Confirm find used the right cust_id in the query.
		if r.URL.Query().Get("cust_id") != "user-2" {
			t.Errorf("find: cust_id query missing/wrong, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 555,
			"cust_id": "user-2",
			"default_ref_id": "existing-token",
			"default_promotion": {"referral_link": "https://x/r/existing-token"}
		}`))
	})
	ts := httptest.NewServer(router)
	defer ts.Close()
	withEnv(t, ts, nil)

	prom, err := ProvisionPromoter(context.Background(), "user-2", "u@example.com", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prom == nil {
		t.Fatal("expected promoter, got nil")
	}
	if !prom.AlreadyExisted {
		t.Error("AlreadyExisted should be true after 409 fallback")
	}
	if prom.PromoterID != "555" {
		t.Errorf("PromoterID: got %q want 555", prom.PromoterID)
	}
	if prom.RefToken != "existing-token" {
		t.Errorf("RefToken: got %q want existing-token", prom.RefToken)
	}

	calls := router.callOrder()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (POST + GET find), got %d: %v", len(calls), calls)
	}
	if calls[0] != "POST "+EndpointCreatePromoter {
		t.Errorf("first call: got %q", calls[0])
	}
	if calls[1] != "GET "+EndpointFindPromoter {
		t.Errorf("second call: got %q", calls[1])
	}
}

func TestProvisionPromoter_409ButFindReturns404(t *testing.T) {
	// FP returns 409 (already exists) but the subsequent find returns 404 —
	// inconsistent state. Caller must learn about it instead of getting nil.
	router := newMultiRouteHandler()
	router.handle(http.MethodPost, EndpointCreatePromoter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{}`))
	})
	router.handle(http.MethodGet, EndpointFindPromoter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(router)
	defer ts.Close()
	withEnv(t, ts, nil)

	_, err := ProvisionPromoter(context.Background(), "user-3", "u@example.com", "", "")
	if err == nil {
		t.Fatal("expected error when FP says exists but find returns 404, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T: %v", err, err)
	}
	if fpErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode: got %d want 409", fpErr.StatusCode)
	}
}

func TestProvisionPromoter_ValidationErrors(t *testing.T) {
	t.Setenv("FIRSTPROMOTER_ENABLED", "1")
	t.Setenv("FIRSTPROMOTER_API_KEY", "k")

	if _, err := ProvisionPromoter(context.Background(), "", "u@example.com", "", ""); err == nil {
		t.Error("expected error for empty custID")
	}
	if _, err := ProvisionPromoter(context.Background(), "u", "", "", ""); err == nil {
		t.Error("expected error for empty email")
	}
}

// =====================================================================
// MintIframeToken
// =====================================================================

func TestMintIframeToken_HappyPath(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody:   `{"access_token": "iframe-token-xyz", "expires_in": 600}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, map[string]string{"FIRSTPROMOTER_PORTAL_DOMAIN": "test-affiliates.example.com"})

	tok, err := MintIframeToken(context.Background(), "user-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == nil {
		t.Fatal("expected token, got nil")
	}
	if tok.AccessToken != "iframe-token-xyz" {
		t.Errorf("AccessToken: got %q", tok.AccessToken)
	}
	if tok.ExpiresInSeconds != 600 {
		t.Errorf("ExpiresInSeconds: got %d want 600", tok.ExpiresInSeconds)
	}
	if tok.PortalDomain != "test-affiliates.example.com" {
		t.Errorf("PortalDomain: got %q want test-affiliates.example.com", tok.PortalDomain)
	}

	_, path, _, _, body, _ := h.snapshot()
	if path != EndpointSSOToken {
		t.Errorf("path: got %q want %q", path, EndpointSSOToken)
	}
	var sent map[string]interface{}
	_ = json.Unmarshal(body, &sent)
	if sent["cust_id"] != "user-1" {
		t.Errorf("cust_id: got %v", sent["cust_id"])
	}
	if sent["campaign_id"] != "camp_test_42" {
		t.Errorf("campaign_id should default from env, got %v", sent["campaign_id"])
	}
}

func TestMintIframeToken_NonOKReturnsError(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusInternalServerError,
		respBody:   `{"error": "internal"}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	_, err := MintIframeToken(context.Background(), "user-1", "")
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T", err)
	}
	if fpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode: got %d want 500", fpErr.StatusCode)
	}
	if fpErr.Endpoint != EndpointSSOToken {
		t.Errorf("Endpoint: got %q want %q", fpErr.Endpoint, EndpointSSOToken)
	}
}

// =====================================================================
// GetPromoter — 404 returns nil/nil
// =====================================================================

func TestGetPromoter_HappyPath(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody:   `{"id": 9, "cust_id": "user-1", "default_ref_id": "tk"}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	prom, err := GetPromoter(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prom == nil || prom.PromoterID != "9" {
		t.Fatalf("unexpected promoter: %+v", prom)
	}
	method, _, _, _, _, _ := h.snapshot()
	if method != http.MethodGet {
		t.Errorf("method: got %q want GET", method)
	}
}

func TestGetPromoter_404ReturnsNilNil(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusNotFound,
		respBody:   `{"error": "not found"}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	prom, err := GetPromoter(context.Background(), "ghost-user")
	if err != nil {
		t.Fatalf("404 must NOT produce an error, got: %v", err)
	}
	if prom != nil {
		t.Errorf("404 must return nil promoter, got: %+v", prom)
	}
}

func TestGetPromoter_5xxReturnsError(t *testing.T) {
	h := &recordingHandler{statusCode: http.StatusBadGateway, respBody: "upstream gone"}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	_, err := GetPromoter(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error on 502, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T", err)
	}
	if fpErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode: got %d", fpErr.StatusCode)
	}
}

// =====================================================================
// TrackSignup
// =====================================================================

func TestTrackSignup_HappyPath(t *testing.T) {
	h := &recordingHandler{statusCode: http.StatusOK, respBody: `{"ok": true}`}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	err := TrackSignup(context.Background(), "ref-token-abc", "new@example.com", "user-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, path, _, _, body, _ := h.snapshot()
	if path != EndpointTrackSignup {
		t.Errorf("path: got %q want %q", path, EndpointTrackSignup)
	}
	var sent map[string]interface{}
	_ = json.Unmarshal(body, &sent)
	if sent["ref_token"] != "ref-token-abc" {
		t.Errorf("ref_token: got %v", sent["ref_token"])
	}
	if sent["email"] != "new@example.com" {
		t.Errorf("email: got %v", sent["email"])
	}
	if sent["cust_id"] != "user-99" {
		t.Errorf("cust_id: got %v", sent["cust_id"])
	}
}

func TestTrackSignup_4xxSurfaces(t *testing.T) {
	h := &recordingHandler{statusCode: http.StatusUnprocessableEntity, respBody: `{"error": "bad ref_token"}`}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	err := TrackSignup(context.Background(), "bad-token", "x@example.com", "")
	if err == nil {
		t.Fatal("expected error on 422, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T", err)
	}
	if fpErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode: got %d", fpErr.StatusCode)
	}
}

// =====================================================================
// ListPromoters — pagination
// =====================================================================

func TestListPromoters_HappyPath(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody: `[
			{"id": 1, "cust_id": "u1", "email": "u1@example.com", "default_ref_id": "t1"},
			{"id": 2, "cust_id": "u2", "email": "u2@example.com", "default_ref_id": "t2"}
		]`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	promoters, hasMore, err := ListPromoters(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(promoters) != 2 {
		t.Fatalf("promoters: got %d want 2", len(promoters))
	}
	if hasMore {
		t.Error("hasMore should be false (returned 2 < 100 perPage)")
	}
	if promoters[0].PromoterID != "1" || promoters[0].CustID != "u1" {
		t.Errorf("promoters[0]: %+v", promoters[0])
	}
	if promoters[1].PromoterID != "2" || promoters[1].Email != "u2@example.com" {
		t.Errorf("promoters[1]: %+v", promoters[1])
	}
	_, path, _, _, _, _ := h.snapshot()
	if path != EndpointListPromoters {
		t.Errorf("path: got %q want %q", path, EndpointListPromoters)
	}
	if !strings.Contains(h.rawQuery, "page=1") || !strings.Contains(h.rawQuery, "per_page=100") {
		t.Errorf("query: got %q", h.rawQuery)
	}
}

func TestListPromoters_HasMoreWhenFull(t *testing.T) {
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody: `[
			{"id": 1, "cust_id": "u1"},
			{"id": 2, "cust_id": "u2"}
		]`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	_, hasMore, err := ListPromoters(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMore {
		t.Error("hasMore should be true when len == perPage")
	}
}

func TestListPromoters_AcceptsPromotersEnvelope(t *testing.T) {
	// FP wraps the array in {"promoters": [...]} on some endpoints.
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody:   `{"promoters": [{"id": 7, "cust_id": "u7"}]}`,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	promoters, _, err := ListPromoters(context.Background(), 0, 0) // 0 → defaults
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(promoters) != 1 || promoters[0].PromoterID != "7" {
		t.Fatalf("expected 1 promoter id=7, got %+v", promoters)
	}
}

// =====================================================================
// API-key redaction in error bodies and logs
// =====================================================================

func TestRedaction_401ResponseScrubsAPIKey(t *testing.T) {
	const key = "fp_secret_should_not_leak"
	// Echo the key in the response body to simulate a buggy FP that leaks
	// our bearer back to us.
	h := &recordingHandler{
		statusCode: http.StatusUnauthorized,
		respBody:   fmt.Sprintf(`{"error": "invalid token: %s"}`, key),
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, map[string]string{"FIRSTPROMOTER_API_KEY": key})

	_, err := GetPromoter(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T", err)
	}
	if strings.Contains(fpErr.Body, key) {
		t.Errorf("API key leaked into FirstPromoterError.Body: %q", fpErr.Body)
	}
	if !strings.Contains(fpErr.Body, "[redacted]") {
		t.Errorf("Body should contain [redacted] marker, got: %q", fpErr.Body)
	}
	if strings.Contains(fpErr.Error(), key) {
		t.Errorf("API key leaked into Error() string: %q", fpErr.Error())
	}
}

// =====================================================================
// Network timeout
// =====================================================================

func TestNetworkTimeout_ProducesFPErrorWithTimeoutCode(t *testing.T) {
	// Server sleeps long enough that the per-call deadline trips first.
	h := &recordingHandler{
		statusCode: http.StatusOK,
		respBody:   `{}`,
		respDelay:  500 * time.Millisecond,
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	withEnv(t, ts, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := GetPromoter(ctx, "user-1")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T: %v", err, err)
	}
	if fpErr.StatusCode != 0 {
		t.Errorf("StatusCode should be 0 for transport failure, got %d", fpErr.StatusCode)
	}
	if !fpErr.IsTimeout() {
		t.Errorf("IsTimeout should be true, got Code=%q", fpErr.Code)
	}
}

func TestNetworkFailure_UnreachableServer(t *testing.T) {
	// Spin up + immediately tear down so the URL points to nothing alive.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	dead := ts.URL
	ts.Close()

	t.Setenv("FIRSTPROMOTER_ENABLED", "1")
	t.Setenv("FIRSTPROMOTER_API_KEY", "k")
	t.Setenv("FIRSTPROMOTER_API_BASE", dead)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := GetPromoter(ctx, "user-1")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	var fpErr *FirstPromoterError
	if !errors.As(err, &fpErr) {
		t.Fatalf("expected *FirstPromoterError, got %T", err)
	}
	if fpErr.StatusCode != 0 {
		t.Errorf("StatusCode should be 0 for transport failure, got %d", fpErr.StatusCode)
	}
}
