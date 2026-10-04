// clerk_testkit_test.go — test-only scaffolding for the Clerk satellite path.
//
// Mirrors timpager's app/tests/clerk_testkit.py: every Clerk test in this
// package runs with ZERO network. There is no real JWKS fetch, no real Clerk
// instance, and no real timpager. Keys are generated in-process, tokens are
// minted by hand (so a test can omit a claim or forge an algorithm, which no
// signing library will let you do), and the JWKS seam is swapped for a stub.
//
// The `_test.go` suffix keeps all of this out of the production binary.
package main

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	clerk "github.com/clerk/clerk-sdk-go/v2"
)

// -----------------------------------------------------------------------
// Fixed test values. None of these is a real host, key, or instance.
// -----------------------------------------------------------------------

const (
	testClerkIssuer = "https://clerk.test.invalid"
	testClerkAZP    = "https://app.groflo.io"
	testClerkKID    = "ins_test_key_1"
	testClerkAZPCSV = "https://app.groflo.io,https://groflo.io"
)

// -----------------------------------------------------------------------
// Keys
// -----------------------------------------------------------------------

// newTestKeypair returns a throwaway RSA key. 2048 bits: big enough to be a
// realistic RS256 key, small enough that per-test generation stays cheap.
func newTestKeypair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}
	return key
}

// testJWK wraps an in-process public key as the SDK's JWK type, exactly as a
// fetched-and-decoded JWKS entry would look.
func testJWK(pub *rsa.PublicKey, kid string) *clerk.JSONWebKey {
	return &clerk.JSONWebKey{Key: pub, KeyID: kid, Algorithm: "RS256", Use: "sig"}
}

// -----------------------------------------------------------------------
// Token minting
// -----------------------------------------------------------------------

// clerkTokenOpts describes a token to mint. The zero value plus
// defaultClerkTokenOpts() is a valid Clerk session JWT for this test
// instance; each field exists so one test can break exactly one thing.
type clerkTokenOpts struct {
	sub      string
	omitSub  bool
	iss      string
	omitIss  bool
	azp      string
	omitAZP  bool
	kid      string
	omitKID  bool
	alg      string // "RS256" (default), "HS256", or "none"
	sid      string
	expDelta time.Duration
	omitExp  bool
	nbfDelta time.Duration
	omitNbf  bool
	iatDelta time.Duration
	omitIat  bool
	// extra carries custom session-token claims (google_sub / email / phone),
	// which Clerk only emits if the primary instance is configured to add them.
	extra map[string]any
}

func defaultClerkTokenOpts() clerkTokenOpts {
	return clerkTokenOpts{
		sub:      "user_test_sub_1",
		iss:      testClerkIssuer,
		azp:      testClerkAZP,
		kid:      testClerkKID,
		alg:      "RS256",
		sid:      "sess_test_1",
		expDelta: 5 * time.Minute,
		nbfDelta: -10 * time.Second,
		iatDelta: -10 * time.Second,
	}
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// mintClerkTestJWT builds a JWT by hand. Hand-rolling (rather than using a
// signing library) is deliberate: a library will not let a test omit `exp`,
// claim `alg: none`, or sign RS256 bytes with an HMAC key, and those are
// exactly the attacks the verifier has to refuse.
func mintClerkTestJWT(t *testing.T, priv *rsa.PrivateKey, opts clerkTokenOpts) string {
	t.Helper()

	header := map[string]any{"typ": "JWT", "alg": opts.alg}
	if opts.alg == "" {
		header["alg"] = "RS256"
	}
	if !opts.omitKID && opts.kid != "" {
		header["kid"] = opts.kid
	}

	now := time.Now()
	claims := map[string]any{}
	if !opts.omitSub {
		claims["sub"] = opts.sub
	}
	if !opts.omitIss {
		claims["iss"] = opts.iss
	}
	if !opts.omitAZP {
		claims["azp"] = opts.azp
	}
	if opts.sid != "" {
		claims["sid"] = opts.sid
	}
	if !opts.omitExp {
		claims["exp"] = now.Add(opts.expDelta).Unix()
	}
	if !opts.omitNbf {
		claims["nbf"] = now.Add(opts.nbfDelta).Unix()
	}
	if !opts.omitIat {
		claims["iat"] = now.Add(opts.iatDelta).Unix()
	}
	for k, v := range opts.extra {
		claims[k] = v
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshalling test JWT header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshalling test JWT claims: %v", err)
	}
	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)

	var sig []byte
	switch header["alg"] {
	case "none":
		sig = nil
	case "HS256":
		mac := hmac.New(sha256.New, []byte("a-symmetric-secret-must-never-verify"))
		mac.Write([]byte(signingInput))
		sig = mac.Sum(nil)
	default: // RS256
		digest := sha256.Sum256([]byte(signingInput))
		sig, err = rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("signing test JWT: %v", err)
		}
	}
	return signingInput + "." + b64url(sig)
}

// mintValidClerkJWT is the common case: a token this instance should accept.
func mintValidClerkJWT(t *testing.T, priv *rsa.PrivateKey, sub string) string {
	t.Helper()
	o := defaultClerkTokenOpts()
	o.sub = sub
	return mintClerkTestJWT(t, priv, o)
}

// -----------------------------------------------------------------------
// JWKS stub — swaps the production seam
// -----------------------------------------------------------------------

// stubJWKSSource implements the same interface as the production JWKS source.
// Tests install it so the verifier never reaches the network; the counters let
// a test assert that a stage did NOT run.
type stubJWKSSource struct {
	mu      sync.Mutex
	keys    map[string]*clerk.JSONWebKey
	err     error
	lookups int
	issuers []string
}

func newStubJWKS(keys map[string]*clerk.JSONWebKey) *stubJWKSSource {
	return &stubJWKSSource{keys: keys}
}

func (s *stubJWKSSource) keyForKID(_ context.Context, issuer, kid string) (*clerk.JSONWebKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups++
	s.issuers = append(s.issuers, issuer)
	if s.err != nil {
		return nil, s.err
	}
	k, ok := s.keys[kid]
	if !ok {
		return nil, fmt.Errorf("no key for kid %q", kid)
	}
	return k, nil
}

func (s *stubJWKSSource) Lookups() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lookups
}

func (s *stubJWKSSource) Issuers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.issuers...)
}

// SetError makes every subsequent lookup fail, to exercise fail-closed paths.
func (s *stubJWKSSource) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// SetKey installs or replaces one key (used for EC / "enc" key pinning tests).
func (s *stubJWKSSource) SetKey(kid string, k *clerk.JSONWebKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys == nil {
		s.keys = map[string]*clerk.JSONWebKey{}
	}
	s.keys[kid] = k
}

// -----------------------------------------------------------------------
// Environment
// -----------------------------------------------------------------------

// withClerkSatelliteEnv installs the satellite configuration for one test and
// installs a stub JWKS source holding `pub` under the default kid.
//
// It sets CLERK_SECRET_KEY and CLERK_WEBHOOK_SIGNING_SECRET to "" explicitly:
// pennyai is a satellite and must never hold a Clerk secret, and a test must
// never pass because an operator's shell happened to export one.
func withClerkSatelliteEnv(t *testing.T, priv *rsa.PrivateKey, overrides map[string]string) *stubJWKSSource {
	t.Helper()

	env := map[string]string{
		"CLERK_ENABLED":                 "1",
		"CLERK_JWT_ISSUER":              testClerkIssuer,
		"CLERK_AUTHORIZED_PARTIES":      testClerkAZPCSV,
		"CLERK_SECRET_KEY":              "",
		"CLERK_WEBHOOK_SIGNING_SECRET":  "",
		"CLERK_FRONTEND_API_ORIGIN":     "",
	}
	for k, v := range overrides {
		env[k] = v
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	var keys map[string]*clerk.JSONWebKey
	if priv != nil {
		keys = map[string]*clerk.JSONWebKey{testClerkKID: testJWK(&priv.PublicKey, testClerkKID)}
	}
	stub := newStubJWKS(keys)
	prev := clerkJWKS
	clerkJWKS = stub
	t.Cleanup(func() { clerkJWKS = prev; _resetClerkState() })
	return stub
}

// -----------------------------------------------------------------------
// Real JWKS JSON (for exercising the production httpJWKSSource)
// -----------------------------------------------------------------------

// rsaJWKSJSON renders a public RSA key as a JWKS document, the way Clerk's
// /.well-known/jwks.json does. kty/use/alg are parameters so a test can serve
// a key the loader must refuse.
func rsaJWKSJSON(pub *rsa.PublicKey, kid, kty, use, alg string) string {
	n := b64url(pub.N.Bytes())
	e := b64url(big.NewInt(int64(pub.E)).Bytes())
	return fmt.Sprintf(
		`{"keys":[{"kty":%q,"kid":%q,"use":%q,"alg":%q,"n":%q,"e":%q}]}`,
		kty, kid, use, alg, n, e,
	)
}

// ecJWKSJSON renders a P-256 key — a valid JWK of the wrong family. The
// verifier must refuse it rather than attempt an RS256 verification with it.
func ecJWKSJSON(kid, use string) string {
	// Fixed, published P-256 test vector coordinates (RFC 7515 Appendix A.3.1).
	return fmt.Sprintf(
		`{"keys":[{"kty":"EC","crv":"P-256","kid":%q,"use":%q,"alg":"ES256",`+
			`"x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",`+
			`"y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}]}`,
		kid, use,
	)
}

// jwksTestServer serves a fixed JWKS body at /.well-known/jwks.json and counts
// hits, so cache and refetch-throttle behaviour can be asserted without a
// network. Returns the server and a function reading the hit count.
type jwksTestServer struct {
	*httptest.Server
	mu     sync.Mutex
	hits   int
	body   string
	status int
}

func newJWKSTestServer(t *testing.T, body string) *jwksTestServer {
	t.Helper()
	s := &jwksTestServer{body: body, status: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits++
		body, status := s.body, s.status
		s.mu.Unlock()
		if r.URL.Path != "/.well-known/jwks.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *jwksTestServer) Hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func (s *jwksTestServer) SetBody(body string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body, s.status = body, status
}

// -----------------------------------------------------------------------
// timpager identity-resolve stub
// -----------------------------------------------------------------------

// recordedResolveRequest is everything the resolve client sent, so a test can
// assert the service key travelled as X-API-Key and the end-user Clerk JWT as
// Authorization — never the other way round.
type recordedResolveRequest struct {
	Method        string
	Path          string
	APIKeyHeader  string
	Authorization string
	ContentType   string
	Body          map[string]any
	RawBody       string
}

type stubTimpager struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recordedResolveRequest
	status   int
	body     string
	// followUp, when set, answers every request after the first — so a test can
	// make the first attempt fail and the retry succeed without a spin loop.
	followUpBody   string
	followUpStatus int
	hasFollowUp    bool
}

// newStubTimpagerResolve serves POST /api/v1/identity/resolve. `response` may
// be a map/struct (marshalled) or a string (served verbatim, for malformed
// JSON tests).
func newStubTimpagerResolve(t *testing.T, response any, status int) *stubTimpager {
	t.Helper()
	s := &stubTimpager{status: status}
	switch v := response.(type) {
	case nil:
		s.body = "{}"
	case string:
		s.body = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshalling stub timpager response: %v", err)
		}
		s.body = string(b)
	}

	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec := recordedResolveRequest{
			Method:        r.Method,
			Path:          r.URL.Path,
			APIKeyHeader:  r.Header.Get("X-API-Key"),
			Authorization: r.Header.Get("Authorization"),
			ContentType:   r.Header.Get("Content-Type"),
			RawBody:       string(raw),
		}
		json.Unmarshal(raw, &rec.Body)
		s.mu.Lock()
		s.requests = append(s.requests, rec)
		status, body := s.status, s.body
		if s.hasFollowUp && len(s.requests) > 1 {
			status, body = s.followUpStatus, s.followUpBody
		}
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *stubTimpager) Requests() []recordedResolveRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedResolveRequest(nil), s.requests...)
}

func (s *stubTimpager) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *stubTimpager) SetResponse(body string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body, s.status = body, status
}

// SetFollowUp makes every request after the first answer with this instead.
func (s *stubTimpager) SetFollowUp(body string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.followUpBody, s.followUpStatus, s.hasFollowUp = body, status, true
}

// resolveOKBody builds a 200 response from timpager's identity resolve.
// rallyUserID "" means "no Rally user matched" (rally_user_id: null).
func resolveOKBody(rallyUserID, clerkUserID, email, phone string) string {
	m := map[string]any{
		"user":               nil,
		"rally_user_id":      nil,
		"clerk_user_id":      clerkUserID,
		"email":              nil,
		"phone":              nil,
		"is_superadmin":      false,
		"version":            "2026-06-17",
		"cache_ttl_seconds":  300,
	}
	if rallyUserID != "" {
		m["rally_user_id"] = rallyUserID
		m["user"] = map[string]any{"id": rallyUserID}
	}
	if email != "" {
		m["email"] = email
	}
	if phone != "" {
		m["phone"] = phone
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// useStubTimpager points the server's existing TIMPAGER_* config at the stub.
// No new Config fields: the exchange reuses TimpagerURL / TimpagerAPIKey.
func useStubTimpager(s *Server, stub *stubTimpager, apiKey string) {
	s.config.TimpagerURL = strings.TrimSuffix(stub.URL, "/")
	s.config.TimpagerAPIKey = apiKey
}
