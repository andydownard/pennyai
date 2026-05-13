// Package firstpromoter is a Go port of timpager's lib/firstpromoter.js.
//
// FirstPromoter v2 REST API client. Pure functions a route handler can call.
// No HTTP framework, no middleware. Wave 1 / PR-B of the Clerk + FirstPromoter
// integration. See timpager/plans/clerk-firstpromoter-phase-1-findings.md
// section 8c step 11 for the broader context.
//
// Public surface mirrors the Node version one-for-one:
//   - ProvisionPromoter(ctx, custID, email, refID, campaignID) → *Promoter
//   - MintIframeToken(ctx, custID, campaignID)                  → *IframeToken
//   - GetPromoter(ctx, custID)                                  → *Promoter (nil on 404)
//   - TrackSignup(ctx, refToken, email, custID)                 → error
//   - ListPromoters(ctx, page, perPage)                         → ([]*Promoter, hasMore, error)
//   - ErrFirstPromoterDisabled (sentinel)
//   - FirstPromoterError (rich error type)
//
// Kill switch: FIRSTPROMOTER_ENABLED must equal "1" or every public function
// returns ErrFirstPromoterDisabled immediately. Mirrors Node's
// FirstPromoterDisabledError. Hot-read on every call so ops can flip the
// flag without a deploy.
//
// Per timpager CLAUDE.md regression #21: no fallback chains for
// security-critical config. FIRSTPROMOTER_API_KEY MUST be set when enabled —
// we return a clear error otherwise rather than silently use a default.
// Defaults are allowed for non-secret config (api base, portal domain,
// campaign id).
package firstpromoter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// =====================================================================
// Constants
// =====================================================================

const (
	defaultAPIBase       = "https://firstpromoter.com"
	defaultPortalDomain  = "affiliates.rallytasks.com"
	defaultTimeout       = 10 * time.Second
	userAgent            = "pennyai-firstpromoter/1.0"
	defaultListPerPage   = 100
	defaultListPageStart = 1
)

// FirstPromoter v2 endpoint paths. TODO(route author): verify against the
// live FirstPromoter v2 docs before wiring the route. Shapes match FP's
// published v2 REST surface as of 2026-Q2 — `cust_id` is the customer key
// both sides use to identify a promoter without exposing FP's internal
// numeric `id`. If FP renames an endpoint, these are the only places to
// update.
const (
	EndpointCreatePromoter = "/api/v2/company/promoters"
	EndpointFindPromoter   = "/api/v2/company/promoters/find"
	EndpointListPromoters  = "/api/v2/company/promoters"
	EndpointSSOToken       = "/api/v2/sso/access-token"
	EndpointTrackSignup    = "/api/v2/track/signup"
)

// =====================================================================
// Errors
// =====================================================================

// ErrFirstPromoterDisabled is the sentinel returned when FIRSTPROMOTER_ENABLED
// is not exactly "1". The kill switch lets ops disable the FP integration
// without a code deploy.
var ErrFirstPromoterDisabled = errors.New("firstpromoter disabled (FIRSTPROMOTER_ENABLED != 1)")

// FirstPromoterError carries non-2xx FirstPromoter responses (except the two
// documented "expected" non-2xx cases: 409 in ProvisionPromoter, 404 in
// GetPromoter, both of which return nil/recovery values without an error).
//
// StatusCode == 0 means the request never produced an HTTP response — typically
// a context cancellation, a network error, or a timeout. In that case Code is
// set to "timeout" or "network".
type FirstPromoterError struct {
	StatusCode int    // HTTP status code returned by FP, or 0 for transport failures
	Endpoint   string // FP endpoint path that was called
	Body       string // Response body (redacted via maskAPIKey before storage)
	Code       string // "timeout" | "network" | "" — populated for transport failures
	message    string // human-readable message; built once at construction
}

// Error implements the error interface.
func (e *FirstPromoterError) Error() string {
	return e.message
}

// IsTimeout reports whether the error originated from a context deadline or
// AbortController-style timeout.
func (e *FirstPromoterError) IsTimeout() bool {
	return e != nil && e.Code == "timeout"
}

// =====================================================================
// Public types
// =====================================================================

// Promoter is the consumer-facing shape of a FirstPromoter promoter record.
// Mirrors Node's _shapePromoter output. PromoterID is coerced to a string so
// callers don't have to worry about FP's int-vs-string id quirks.
type Promoter struct {
	PromoterID      string `json:"promoter_id"`
	CustID          string `json:"cust_id,omitempty"`
	Email           string `json:"email,omitempty"`
	RefToken        string `json:"ref_token,omitempty"`
	ReferralLink    string `json:"referral_link,omitempty"`
	RecruitmentLink string `json:"recruitment_link,omitempty"`
	AlreadyExisted  bool   `json:"already_existed,omitempty"`
}

// IframeToken is returned by MintIframeToken. AccessToken is short-lived;
// callers MUST NOT log it or persist it. Render the iframe at
// `https://{PortalDomain}/iframe?tk={AccessToken}`.
type IframeToken struct {
	AccessToken      string `json:"access_token"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	PortalDomain     string `json:"portal_domain"`
}

// =====================================================================
// Internal: env-var readers + masking
// =====================================================================

// isEnabled hot-reads the kill switch on every call. Tests mutate os.Setenv
// in place; production ops can flip the flag without restart.
func isEnabled() bool {
	return os.Getenv("FIRSTPROMOTER_ENABLED") == "1"
}

// requireAPIKey returns the API key or a clear, operator-friendly error if
// the key is missing — never silently fall back. Per timpager CLAUDE.md
// regression #21, security secrets do not get fallback chains.
func requireAPIKey() (string, error) {
	key := os.Getenv("FIRSTPROMOTER_API_KEY")
	if key == "" {
		return "", errors.New("FIRSTPROMOTER_API_KEY is not set. Configure the env var in Railway before calling firstpromoter.")
	}
	return key, nil
}

func apiBase() string {
	if v := os.Getenv("FIRSTPROMOTER_API_BASE"); v != "" {
		return v
	}
	return defaultAPIBase
}

// PortalDomain is exported so callers (e.g., a /refer route) can build the
// iframe URL without re-reading the env var.
func PortalDomain() string {
	if v := os.Getenv("FIRSTPROMOTER_PORTAL_DOMAIN"); v != "" {
		return v
	}
	return defaultPortalDomain
}

// resolveCampaignID returns the caller override if set, otherwise the
// environment default. May return "" if neither is set — caller decides
// whether to require it.
func resolveCampaignID(callerOverride string) string {
	if callerOverride != "" {
		return callerOverride
	}
	return os.Getenv("FIRSTPROMOTER_CAMPAIGN_ID")
}

// maskAPIKey scrubs the live API key out of any string that might end up in a
// log line or error message — defense in depth even though we own the
// codepath.
func maskAPIKey(s string) string {
	key := os.Getenv("FIRSTPROMOTER_API_KEY")
	if key == "" || s == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[redacted]")
}

// newFPError builds a FirstPromoterError with a redacted body and a
// pre-computed Error() string. Centralizing construction keeps the logging
// + redaction contract honest.
func newFPError(statusCode int, endpoint, body, code string) *FirstPromoterError {
	redacted := maskAPIKey(body)
	var msg string
	switch {
	case code == "timeout":
		msg = fmt.Sprintf("FirstPromoter request to %s timed out", endpoint)
	case code == "network":
		msg = fmt.Sprintf("FirstPromoter request to %s failed: %s", endpoint, redacted)
	default:
		msg = fmt.Sprintf("FirstPromoter %s returned HTTP %d", endpoint, statusCode)
	}
	return &FirstPromoterError{
		StatusCode: statusCode,
		Endpoint:   endpoint,
		Body:       redacted,
		Code:       code,
		message:    msg,
	}
}

// =====================================================================
// Internal: HTTP transport
// =====================================================================

// requestOpts bundles per-call knobs. Timeout overrides the package default
// when non-zero. Only used internally; public callers pass context.Context.
type requestOpts struct {
	query   url.Values
	body    interface{}
	timeout time.Duration
}

// httpResponse is the parsed result of one FP API call. Body is decoded into
// either a map[string]interface{} (JSON object), []interface{} (JSON array),
// or a string (non-JSON response). The caller decides which statuses count
// as errors — _request never returns an error for a non-2xx; that's the
// caller's call (e.g., 409 in ProvisionPromoter recovers, 404 in GetPromoter
// recovers).
type httpResponse struct {
	OK     bool
	Status int
	Body   interface{}
}

// httpClient is the package's transport. Exposed as a var so tests can swap
// in a transport that targets httptest.NewServer — but the standard pattern
// is to set FIRSTPROMOTER_API_BASE to the test server URL, which avoids
// touching this var at all.
var httpClient = &http.Client{
	Timeout: 0, // we manage per-call timeouts via context.WithTimeout
}

// doRequest is the single transport primitive. Every public function calls
// this. Handles bearer auth, per-call timeout via ctx, JSON marshaling,
// structured logging on success and failure, and safe response parsing
// (FP sometimes returns text or HTML on errors).
//
// Returns (*httpResponse, error). The error is non-nil ONLY for transport
// failures (network, timeout). Non-2xx HTTP statuses are surfaced via
// httpResponse.OK == false + httpResponse.Status — never as a Go error.
func doRequest(ctx context.Context, method, endpoint string, opts requestOpts) (*httpResponse, error) {
	apiKey, err := requireAPIKey()
	if err != nil {
		return nil, err
	}

	// Build URL with optional query string.
	target := apiBase() + endpoint
	if len(opts.query) > 0 {
		target += "?" + opts.query.Encode()
	}

	// Marshal body if present.
	var bodyReader io.Reader
	if opts.body != nil {
		buf, mErr := json.Marshal(opts.body)
		if mErr != nil {
			return nil, fmt.Errorf("firstpromoter: marshal body for %s: %w", endpoint, mErr)
		}
		bodyReader = bytes.NewReader(buf)
	}

	// Per-call timeout. Defaults to 10s — caller can shorten via opts.timeout
	// or extend via the parent ctx.
	timeout := opts.timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, target, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("firstpromoter: build request for %s: %w", endpoint, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if opts.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	res, err := httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		// Distinguish timeout from generic network failure. context.DeadlineExceeded
		// fires when our per-call timeout elapses; net.Errors implement Timeout().
		code := "network"
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutErr(err) {
			code = "timeout"
		}
		fpErr := newFPError(0, endpoint, err.Error(), code)
		log.Printf("[firstpromoter] %s %s error: %s (latency=%s)", method, endpoint, fpErr.Error(), latency)
		return nil, fpErr
	}
	defer res.Body.Close()

	// Read body defensively — FP usually returns JSON but error pages can be
	// HTML or text.
	raw, _ := io.ReadAll(res.Body)
	parsed := parseBody(res.Header.Get("Content-Type"), raw)

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		log.Printf("[firstpromoter] %s %s ok (status=%d, latency=%s)", method, endpoint, res.StatusCode, latency)
	} else {
		// Mask the API key before logging the body — even a buggy FP response
		// that echoes the bearer header is sanitized.
		log.Printf("[firstpromoter] %s %s error (status=%d, latency=%s, body=%s)",
			method, endpoint, res.StatusCode, latency, maskAPIKey(string(raw)))
	}

	return &httpResponse{
		OK:     res.StatusCode >= 200 && res.StatusCode < 300,
		Status: res.StatusCode,
		Body:   parsed,
	}, nil
}

// isTimeoutErr returns true when err is a net-level timeout. Pulled out so
// the doRequest path doesn't import net/url just for the type assertion.
func isTimeoutErr(err error) bool {
	type timeoutAware interface{ Timeout() bool }
	if t, ok := err.(timeoutAware); ok && t.Timeout() {
		return true
	}
	// url.Error wraps the underlying transport error.
	if uerr, ok := err.(*url.Error); ok {
		if t, ok := uerr.Err.(timeoutAware); ok && t.Timeout() {
			return true
		}
	}
	return false
}

// parseBody returns the most useful representation of the response body:
// a JSON value (map / slice) when content-type is JSON and parsing succeeds,
// otherwise the raw text. Never returns an error — failed parses degrade to
// the raw string.
func parseBody(contentType string, raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}
	if strings.Contains(contentType, "application/json") {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err == nil {
			return v
		}
	}
	return string(raw)
}

// extractStringField is a defensive accessor for nested JSON fields. Returns
// "" when the value is missing or the wrong type — never panics.
func extractStringField(body interface{}, keys ...string) string {
	cur := body
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = m[k]
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		// JSON numbers decode as float64; FP's promoter `id` arrives this way.
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// extractIntField is the int-typed sibling of extractStringField. Returns 0
// when the value is missing or the wrong type.
func extractIntField(body interface{}, keys ...string) int {
	cur := body
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return 0
		}
		cur = m[k]
	}
	if f, ok := cur.(float64); ok {
		return int(f)
	}
	return 0
}

// shapePromoter maps the FirstPromoter promoter object to the consumer
// Promoter type. FP uses `id` (numeric), `default_ref_id` (the public ref
// token), and a `default_promotion` sub-object holding the referral_link
// and recruitment_link.
func shapePromoter(body interface{}, alreadyExisted bool) *Promoter {
	if body == nil {
		return nil
	}
	return &Promoter{
		PromoterID:      extractStringField(body, "id"),
		CustID:          extractStringField(body, "cust_id"),
		Email:           extractStringField(body, "email"),
		RefToken:        extractStringField(body, "default_ref_id"),
		ReferralLink:    extractStringField(body, "default_promotion", "referral_link"),
		RecruitmentLink: extractStringField(body, "default_promotion", "recruitment_link"),
		AlreadyExisted:  alreadyExisted,
	}
}

// =====================================================================
// Public surface
// =====================================================================

// ProvisionPromoter creates a FirstPromoter promoter for the given customer.
// Idempotent on FP's side via cust_id — if a promoter with this cust_id
// already exists FP returns 409 and we transparently GET the existing record
// and return it with AlreadyExisted=true.
//
// custID and email are required; refID and campaignID are optional (pass
// "" to omit / accept the env-var default for campaignID).
func ProvisionPromoter(ctx context.Context, custID, email, refID, campaignID string) (*Promoter, error) {
	if !isEnabled() {
		return nil, ErrFirstPromoterDisabled
	}
	if custID == "" {
		return nil, errors.New("firstpromoter: ProvisionPromoter: custID is required")
	}
	if email == "" {
		return nil, errors.New("firstpromoter: ProvisionPromoter: email is required")
	}

	body := map[string]interface{}{
		"cust_id": custID,
		"email":   email,
	}
	if refID != "" {
		body["ref_id"] = refID
	}
	if camp := resolveCampaignID(campaignID); camp != "" {
		body["initial_campaign_id"] = camp
	}

	res, err := doRequest(ctx, http.MethodPost, EndpointCreatePromoter, requestOpts{body: body})
	if err != nil {
		return nil, err
	}
	if res.OK {
		return shapePromoter(res.Body, false), nil
	}
	if res.Status == http.StatusConflict {
		// 409 = already exists. Look up the existing record and return it.
		existing, lookupErr := GetPromoter(ctx, custID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing != nil {
			existing.AlreadyExisted = true
			return existing, nil
		}
		// FP said "already exists" but find returned null — surface as error
		// so the caller doesn't silently lose data.
		return nil, newFPError(http.StatusConflict, EndpointCreatePromoter,
			fmt.Sprintf("FirstPromoter said cust_id=%s already exists but find returned no record", custID), "")
	}
	return nil, newFPError(res.Status, EndpointCreatePromoter, bodyToString(res.Body), "")
}

// MintIframeToken mints a one-time SSO access token for embedding the FP
// affiliate portal in an iframe. The returned token is short-lived; never
// log it, never store it. Caller renders
// `https://{PortalDomain}/iframe?tk={AccessToken}`.
func MintIframeToken(ctx context.Context, custID, campaignID string) (*IframeToken, error) {
	if !isEnabled() {
		return nil, ErrFirstPromoterDisabled
	}
	if custID == "" {
		return nil, errors.New("firstpromoter: MintIframeToken: custID is required")
	}

	body := map[string]interface{}{
		"cust_id": custID,
	}
	if camp := resolveCampaignID(campaignID); camp != "" {
		body["campaign_id"] = camp
	}

	res, err := doRequest(ctx, http.MethodPost, EndpointSSOToken, requestOpts{body: body})
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, newFPError(res.Status, EndpointSSOToken, bodyToString(res.Body), "")
	}
	return &IframeToken{
		AccessToken:      extractStringField(res.Body, "access_token"),
		ExpiresInSeconds: extractIntField(res.Body, "expires_in"),
		PortalDomain:     PortalDomain(),
	}, nil
}

// GetPromoter looks up an existing FirstPromoter promoter by app-side
// cust_id. Returns (nil, nil) on 404 so callers can branch cleanly on
// "exists vs. not". Returns a *FirstPromoterError on any other non-2xx.
func GetPromoter(ctx context.Context, custID string) (*Promoter, error) {
	if !isEnabled() {
		return nil, ErrFirstPromoterDisabled
	}
	if custID == "" {
		return nil, errors.New("firstpromoter: GetPromoter: custID is required")
	}

	q := url.Values{}
	q.Set("cust_id", custID)

	res, err := doRequest(ctx, http.MethodGet, EndpointFindPromoter, requestOpts{query: q})
	if err != nil {
		return nil, err
	}
	if res.OK {
		return shapePromoter(res.Body, false), nil
	}
	if res.Status == http.StatusNotFound {
		return nil, nil
	}
	return nil, newFPError(res.Status, EndpointFindPromoter, bodyToString(res.Body), "")
}

// TrackSignup registers a first-touch signup attribution. Used by the
// migration script (PR sequence 8c step 12) to backfill `referred_by` chains
// into FP without requiring users to click a fresh tracking link.
//
// refToken and email are required; custID is optional (pass "" to omit).
// Returns a *FirstPromoterError on non-2xx so the migration script can
// collect the failures.
func TrackSignup(ctx context.Context, refToken, email, custID string) error {
	if !isEnabled() {
		return ErrFirstPromoterDisabled
	}
	if refToken == "" {
		return errors.New("firstpromoter: TrackSignup: refToken is required")
	}
	if email == "" {
		return errors.New("firstpromoter: TrackSignup: email is required")
	}

	body := map[string]interface{}{
		"ref_token": refToken,
		"email":     email,
	}
	if custID != "" {
		body["cust_id"] = custID
	}

	res, err := doRequest(ctx, http.MethodPost, EndpointTrackSignup, requestOpts{body: body})
	if err != nil {
		return err
	}
	if !res.OK {
		return newFPError(res.Status, EndpointTrackSignup, bodyToString(res.Body), "")
	}
	return nil
}

// ListPromoters pages through the FirstPromoter promoter list. Used by the
// nightly reconciliation worker (Wave 3 / PR-B) to compare FP's promoter
// universe against the local affiliate_partners table — READ-ONLY on both
// sides.
//
// Returns (promoters, hasMore, err). The worker walks through `hasMore` and
// is agnostic to FP's specific pagination scheme — if FP later switches to
// cursor-based pagination, this function is the only place to update.
//
// Pass page <= 0 to use the default (1). Pass perPage <= 0 to use the
// default (100). FP caps perPage on its end.
//
// TODO(reviewer): verify the endpoint path + pagination parameter names
// (`page`, `per_page`) against the live FirstPromoter v2 docs before this
// worker enables in production.
func ListPromoters(ctx context.Context, page, perPage int) ([]*Promoter, bool, error) {
	if !isEnabled() {
		return nil, false, ErrFirstPromoterDisabled
	}
	if page <= 0 {
		page = defaultListPageStart
	}
	if perPage <= 0 {
		perPage = defaultListPerPage
	}

	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))

	res, err := doRequest(ctx, http.MethodGet, EndpointListPromoters, requestOpts{query: q})
	if err != nil {
		return nil, false, err
	}
	if !res.OK {
		return nil, false, newFPError(res.Status, EndpointListPromoters, bodyToString(res.Body), "")
	}

	// FP v2 returns either a bare array or {promoters: [...]} or {data: [...]}
	// depending on the endpoint. Accept all three.
	var raw []interface{}
	switch v := res.Body.(type) {
	case []interface{}:
		raw = v
	case map[string]interface{}:
		if list, ok := v["promoters"].([]interface{}); ok {
			raw = list
		} else if list, ok := v["data"].([]interface{}); ok {
			raw = list
		}
	}

	promoters := make([]*Promoter, 0, len(raw))
	for _, item := range raw {
		// shapePromoter handles nil safely. List endpoint omits
		// `default_promotion` for non-active promoters — that's expected.
		shaped := shapePromoter(item, false)
		if shaped == nil {
			continue
		}
		promoters = append(promoters, shaped)
	}

	// Less-than-perPage results = FP returned the tail. The reconciliation
	// worker uses hasMore as its loop sentinel; we never depend on a
	// separate "total" count.
	hasMore := len(promoters) >= perPage
	return promoters, hasMore, nil
}

// bodyToString renders a parsed JSON body as a string, suitable for
// embedding in error messages. Strings pass through; everything else is
// re-marshaled. Never returns an error — falls back to fmt.Sprintf on
// marshal failure (which can't really happen for parsed JSON).
func bodyToString(body interface{}) string {
	if body == nil {
		return ""
	}
	if s, ok := body.(string); ok {
		return s
	}
	if buf, err := json.Marshal(body); err == nil {
		return string(buf)
	}
	return fmt.Sprintf("%v", body)
}
