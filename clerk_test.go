package main

// clerk_test.go — the Rally sign-in (Clerk satellite) verifier.
//
// Architecture under test (see docs/superpowers/specs/*clerk*):
//   - pennyai is a SATELLITE. It holds NO Clerk secret and never calls Clerk's
//     Backend API. It verifies session JWTs against the primary instance's
//     PUBLIC JWKS.
//   - pennyai is COOKIE-BLIND. Clerk's own __session cookie is never read;
//     only an Authorization: Bearer header counts. A browser that happens to
//     carry __session for the apex domain must not authenticate here.
//   - The SDK's jwt.Verify is necessary but not sufficient. Its issuer check is
//     a pattern ("https://clerk." prefix / contains ".clerk.accounts"), it does
//     not require `exp` to be present, and it does not require `sub`. pennyai
//     adds: exact issuer match, exp-present, sub-present, an RS256-only
//     pre-check before any key lookup, and JWK kty/use/alg pinning.
//
// Every test here is networkless: keys are generated in-process and the JWKS
// seam is a stub (see clerk_testkit_test.go).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	clerk "github.com/clerk/clerk-sdk-go/v2"
)

// =====================================================================
// clerkConfig — fail-closed configuration
// =====================================================================

func TestClerkConfig_RequiresIssuerAndAuthorizedParties(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		withClerkSatelliteEnv(t, nil, nil)
		cfg, err := clerkConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Issuer != testClerkIssuer {
			t.Errorf("Issuer: got %q want %q", cfg.Issuer, testClerkIssuer)
		}
		if len(cfg.AuthorizedParties) != 2 || cfg.AuthorizedParties[0] != "https://app.groflo.io" || cfg.AuthorizedParties[1] != "https://groflo.io" {
			t.Errorf("AuthorizedParties: got %#v", cfg.AuthorizedParties)
		}
	})

	t.Run("missing issuer is refused", func(t *testing.T) {
		withClerkSatelliteEnv(t, nil, map[string]string{"CLERK_JWT_ISSUER": ""})
		if _, err := clerkConfig(); err == nil {
			t.Fatal("empty CLERK_JWT_ISSUER must be a configuration error — a satellite cannot verify without an exact issuer")
		}
	})

	t.Run("empty allowlist is refused", func(t *testing.T) {
		withClerkSatelliteEnv(t, nil, map[string]string{"CLERK_AUTHORIZED_PARTIES": ""})
		if _, err := clerkConfig(); err == nil {
			t.Fatal("empty CLERK_AUTHORIZED_PARTIES must be a configuration error — without an azp allowlist any Clerk app on the instance could mint usable tokens")
		}
	})

	t.Run("allowlist is trimmed and blanks dropped", func(t *testing.T) {
		withClerkSatelliteEnv(t, nil, map[string]string{
			"CLERK_AUTHORIZED_PARTIES": " https://app.groflo.io , , https://groflo.io ",
		})
		cfg, err := clerkConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.AuthorizedParties) != 2 {
			t.Errorf("AuthorizedParties: got %#v want 2 entries", cfg.AuthorizedParties)
		}
	})

	t.Run("read at call time not module load", func(t *testing.T) {
		// Regression #21: config must be hot-read so a flag flip needs no
		// restart and so t.Setenv inside a subtest actually takes effect.
		withClerkSatelliteEnv(t, nil, nil)
		if _, err := clerkConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Setenv("CLERK_JWT_ISSUER", "https://clerk.other.invalid")
		cfg, err := clerkConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Issuer != "https://clerk.other.invalid" {
			t.Errorf("issuer was cached at load time; got %q", cfg.Issuer)
		}
	})
}

// =====================================================================
// VerifyClerkToken — positive
// =====================================================================

func TestVerifyClerkToken_ValidTokenYieldsIdentity(t *testing.T) {
	priv := newTestKeypair(t)
	stub := withClerkSatelliteEnv(t, priv, nil)

	token := mintValidClerkJWT(t, priv, "user_abc")
	id, err := VerifyClerkToken(context.Background(), token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if id.ClerkUserID != "user_abc" {
		t.Errorf("ClerkUserID: got %q want user_abc", id.ClerkUserID)
	}
	if id.SessionID != "sess_test_1" {
		t.Errorf("SessionID: got %q want sess_test_1", id.SessionID)
	}
	if id.AuthorizedParty != testClerkAZP {
		t.Errorf("AuthorizedParty: got %q want %q", id.AuthorizedParty, testClerkAZP)
	}
	if id.Email != "" || id.Phone != "" || id.PhoneVerified || id.Name != "" {
		t.Errorf("default Clerk session token carries no custom claims; got %+v", id)
	}
	if id.Claims == nil {
		t.Error("Claims should be retained for callers that need a field we did not project")
	}
	if stub.Lookups() != 1 {
		t.Errorf("expected exactly one JWKS lookup, got %d", stub.Lookups())
	}
	if iss := stub.Issuers(); len(iss) != 1 || iss[0] != testClerkIssuer {
		t.Errorf("key lookup must be scoped to the configured issuer; got %#v", iss)
	}
}

func TestVerifyClerkToken_OptionalCustomClaimsSurface(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	o := defaultClerkTokenOpts()
	o.sub = "user_with_claims"
	o.extra = map[string]any{
		"email":          "Tamika@Example.com",
		"phone":          "+15551234567",
		"phone_verified": true,
		"name":           "Tamika",
	}
	id, err := VerifyClerkToken(context.Background(), mintClerkTestJWT(t, priv, o))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !id.PhoneVerified {
		t.Error("PhoneVerified: got false want true")
	}
	if id.Name != "Tamika" {
		t.Errorf("Name: got %q want Tamika", id.Name)
	}
	if id.Email != "Tamika@Example.com" {
		t.Errorf("Email: got %q (normalization happens at the resolve/upsert layer)", id.Email)
	}
	if id.Phone != "+15551234567" {
		t.Errorf("Phone: got %q", id.Phone)
	}
}

func TestVerifyClerkToken_ExpiredWithinLeewayAccepted(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	o := defaultClerkTokenOpts()
	o.expDelta = -5 * time.Second // inside the 10s clock-skew leeway
	if _, err := VerifyClerkToken(context.Background(), mintClerkTestJWT(t, priv, o)); err != nil {
		t.Fatalf("token 5s past expiry must be accepted within the 10s skew leeway: %v", err)
	}
}

func TestVerifyClerkToken_SecondAuthorizedPartyAccepted(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	o := defaultClerkTokenOpts()
	o.azp = "https://groflo.io" // the second entry in the allowlist
	if _, err := VerifyClerkToken(context.Background(), mintClerkTestJWT(t, priv, o)); err != nil {
		t.Fatalf("every allowlisted origin must be accepted: %v", err)
	}
}

// =====================================================================
// VerifyClerkToken — negative. Each subtest breaks exactly one thing and
// asserts the failure happened, plus (where observable) that no later
// stage ran.
// =====================================================================

func TestVerifyClerkToken_Rejections(t *testing.T) {
	type tc struct {
		name string
		// mutate the token options
		opts func(o *clerkTokenOpts)
		// env overrides applied on top of the satellite defaults
		env map[string]string
		// when set, sign with this key instead of the one in the JWKS
		wrongKey bool
		// expect zero JWKS lookups (failure before any key lookup)
		noLookup bool
		// substring the error must mention
		wantErr string
	}

	cases := []tc{
		{
			name:     "HS256 token",
			opts:     func(o *clerkTokenOpts) { o.alg = "HS256" },
			noLookup: true,
			wantErr:  "RS256",
		},
		{
			name:     "alg none",
			opts:     func(o *clerkTokenOpts) { o.alg = "none" },
			noLookup: true,
			wantErr:  "RS256",
		},
		{
			name:     "missing kid",
			opts:     func(o *clerkTokenOpts) { o.omitKID = true },
			noLookup: true,
			wantErr:  "kid",
		},
		{
			name:    "unknown kid",
			opts:    func(o *clerkTokenOpts) { o.kid = "ins_rotated_key_9" },
			wantErr: "no key for kid",
		},
		{
			name:     "wrong signing key",
			wrongKey: true,
			wantErr:  "", // signature verification failure; message is library-owned
		},
		{
			name:    "expired beyond leeway",
			opts:    func(o *clerkTokenOpts) { o.expDelta = -60 * time.Second },
			wantErr: "",
		},
		{
			name:    "not yet valid",
			opts:    func(o *clerkTokenOpts) { o.nbfDelta = 60 * time.Second },
			wantErr: "",
		},
		{
			name:    "exp missing",
			opts:    func(o *clerkTokenOpts) { o.omitExp = true },
			wantErr: "exp",
		},
		{
			name:    "sub missing",
			opts:    func(o *clerkTokenOpts) { o.omitSub = true },
			wantErr: "sub",
		},
		{
			name:    "iss missing",
			opts:    func(o *clerkTokenOpts) { o.omitIss = true },
			wantErr: "issuer",
		},
		{
			// This is the test that earns the exact-match post-check: the SDK's
			// own issuer validation is only a PATTERN, and
			// "https://clerk.evil.invalid" satisfies it.
			name:    "issuer matches the SDK pattern but is not ours",
			opts:    func(o *clerkTokenOpts) { o.iss = "https://clerk.evil.invalid" },
			wantErr: "issuer",
		},
		{
			name:     "issuer unset in env",
			env:      map[string]string{"CLERK_JWT_ISSUER": ""},
			noLookup: true,
			wantErr:  "CLERK_JWT_ISSUER",
		},
		{
			name:    "azp missing",
			opts:    func(o *clerkTokenOpts) { o.omitAZP = true },
			wantErr: "authorized party",
		},
		{
			name:    "azp not allowlisted",
			opts:    func(o *clerkTokenOpts) { o.azp = "https://attacker.example" },
			wantErr: "authorized party",
		},
		{
			name:     "allowlist unset in env",
			env:      map[string]string{"CLERK_AUTHORIZED_PARTIES": ""},
			noLookup: true,
			wantErr:  "CLERK_AUTHORIZED_PARTIES",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			priv := newTestKeypair(t)
			stub := withClerkSatelliteEnv(t, priv, c.env)

			signer := priv
			if c.wrongKey {
				signer = newTestKeypair(t)
			}
			o := defaultClerkTokenOpts()
			if c.opts != nil {
				c.opts(&o)
			}
			token := mintClerkTestJWT(t, signer, o)

			id, err := VerifyClerkToken(context.Background(), token)
			if err == nil {
				t.Fatalf("expected rejection, got identity %+v", id)
			}
			if id != nil {
				t.Errorf("a rejected token must yield a nil identity, got %+v", id)
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error should mention %q; got: %v", c.wantErr, err)
			}
			if c.noLookup && stub.Lookups() != 0 {
				t.Errorf("verification must fail before any JWKS lookup; lookups=%d", stub.Lookups())
			}
			// Never leak the token itself in the error text.
			if strings.Contains(err.Error(), token) {
				t.Error("error message contains the raw JWT — tokens must never be logged or surfaced")
			}
		})
	}
}

func TestVerifyClerkToken_MalformedTokenShapes(t *testing.T) {
	priv := newTestKeypair(t)
	for _, tok := range []string{"", "not-a-jwt", "a.b", "a.b.c.d", "!!!.###.$$$"} {
		t.Run("shape "+tok, func(t *testing.T) {
			stub := withClerkSatelliteEnv(t, priv, nil)
			if _, err := VerifyClerkToken(context.Background(), tok); err == nil {
				t.Fatalf("malformed token %q accepted", tok)
			}
			if stub.Lookups() != 0 {
				t.Errorf("malformed token must be refused before any key lookup; lookups=%d", stub.Lookups())
			}
		})
	}
}

func TestVerifyClerkToken_JWKSSourceErrorFailsClosed(t *testing.T) {
	priv := newTestKeypair(t)
	stub := withClerkSatelliteEnv(t, priv, nil)
	stub.SetError(errors.New("jwks unreachable"))

	if _, err := VerifyClerkToken(context.Background(), mintValidClerkJWT(t, priv, "user_x")); err == nil {
		t.Fatal("a JWKS failure must fail closed — never verify a token without a key")
	}
}

func TestVerifyClerkToken_PinsKeyTypeAndUse(t *testing.T) {
	t.Run("non-RS256 key refused", func(t *testing.T) {
		priv := newTestKeypair(t)
		stub := withClerkSatelliteEnv(t, priv, nil)
		stub.SetKey(testClerkKID, &clerk.JSONWebKey{Key: &priv.PublicKey, KeyID: testClerkKID, Algorithm: "RS512", Use: "sig"})
		if _, err := VerifyClerkToken(context.Background(), mintValidClerkJWT(t, priv, "user_x")); err == nil {
			t.Fatal("a key whose alg is not RS256 must be refused")
		}
	})

	t.Run("encryption key refused", func(t *testing.T) {
		priv := newTestKeypair(t)
		stub := withClerkSatelliteEnv(t, priv, nil)
		stub.SetKey(testClerkKID, &clerk.JSONWebKey{Key: &priv.PublicKey, KeyID: testClerkKID, Algorithm: "RS256", Use: "enc"})
		if _, err := VerifyClerkToken(context.Background(), mintValidClerkJWT(t, priv, "user_x")); err == nil {
			t.Fatal(`a key with use "enc" must never be used to verify a signature`)
		}
	})

	t.Run("non-RSA key material refused", func(t *testing.T) {
		priv := newTestKeypair(t)
		stub := withClerkSatelliteEnv(t, priv, nil)
		stub.SetKey(testClerkKID, &clerk.JSONWebKey{Key: "not-a-key", KeyID: testClerkKID, Algorithm: "RS256", Use: "sig"})
		if _, err := VerifyClerkToken(context.Background(), mintValidClerkJWT(t, priv, "user_x")); err == nil {
			t.Fatal("non-RSA key material must be refused")
		}
	})
}

// =====================================================================
// httpJWKSSource — the production source: fetch, cache, refetch throttle,
// load-time key pinning. Served by an httptest server, never the internet.
// =====================================================================

func TestHTTPJWKSSource_FetchesCachesAndThrottlesRefetch(t *testing.T) {
	priv := newTestKeypair(t)
	srv := newJWKSTestServer(t, rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "sig", "RS256"))
	src := newHTTPJWKSSource()

	k, err := src.keyForKID(context.Background(), srv.URL, testClerkKID)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if k == nil || k.KeyID != testClerkKID {
		t.Fatalf("decoded key: %+v", k)
	}
	if srv.Hits() != 1 {
		t.Fatalf("hits after first fetch: got %d want 1", srv.Hits())
	}

	// Cached: a second lookup of a known kid does not hit the network.
	if _, err := src.keyForKID(context.Background(), srv.URL, testClerkKID); err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if srv.Hits() != 1 {
		t.Errorf("a known kid must be served from cache; hits=%d want 1", srv.Hits())
	}

	// Unknown kid: refetch once (keys rotate), then throttle so an attacker
	// cannot drive a fetch storm with random kids.
	if _, err := src.keyForKID(context.Background(), srv.URL, "ins_unknown"); err == nil {
		t.Fatal("unknown kid must be an error")
	}
	if srv.Hits() != 2 {
		t.Errorf("unknown kid should trigger exactly one refetch; hits=%d want 2", srv.Hits())
	}
	if _, err := src.keyForKID(context.Background(), srv.URL, "ins_unknown_2"); err == nil {
		t.Fatal("unknown kid must be an error")
	}
	if srv.Hits() != 2 {
		t.Errorf("a second unknown kid within the throttle window must not refetch; hits=%d want 2", srv.Hits())
	}
}

func TestHTTPJWKSSource_RefetchPicksUpRotatedKey(t *testing.T) {
	old := newTestKeypair(t)
	rotated := newTestKeypair(t)
	srv := newJWKSTestServer(t, rsaJWKSJSON(&old.PublicKey, "kid_old", "RSA", "sig", "RS256"))
	src := newHTTPJWKSSource()

	if _, err := src.keyForKID(context.Background(), srv.URL, "kid_old"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	srv.SetBody(rsaJWKSJSON(&rotated.PublicKey, "kid_new", "RSA", "sig", "RS256"), http.StatusOK)

	k, err := src.keyForKID(context.Background(), srv.URL, "kid_new")
	if err != nil {
		t.Fatalf("rotated key must be picked up by the one allowed refetch: %v", err)
	}
	if k.KeyID != "kid_new" {
		t.Errorf("KeyID: got %q want kid_new", k.KeyID)
	}
}

func TestHTTPJWKSSource_FailsClosed(t *testing.T) {
	priv := newTestKeypair(t)

	t.Run("http error", func(t *testing.T) {
		srv := newJWKSTestServer(t, "")
		srv.SetBody("boom", http.StatusInternalServerError)
		if _, err := newHTTPJWKSSource().keyForKID(context.Background(), srv.URL, testClerkKID); err == nil {
			t.Fatal("a non-200 JWKS response must be an error")
		}
	})

	t.Run("malformed body", func(t *testing.T) {
		srv := newJWKSTestServer(t, "{not json")
		if _, err := newHTTPJWKSSource().keyForKID(context.Background(), srv.URL, testClerkKID); err == nil {
			t.Fatal("an undecodable JWKS must be an error")
		}
	})

	t.Run("unreachable host", func(t *testing.T) {
		srv := newJWKSTestServer(t, rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "sig", "RS256"))
		url := srv.URL
		srv.Close() // nothing listening any more
		if _, err := newHTTPJWKSSource().keyForKID(context.Background(), url, testClerkKID); err == nil {
			t.Fatal("an unreachable JWKS endpoint must be an error, never a silent pass")
		}
	})

	t.Run("empty issuer", func(t *testing.T) {
		if _, err := newHTTPJWKSSource().keyForKID(context.Background(), "", testClerkKID); err == nil {
			t.Fatal("an empty issuer must be refused before any request")
		}
	})

	t.Run("empty kid", func(t *testing.T) {
		srv := newJWKSTestServer(t, rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "sig", "RS256"))
		if _, err := newHTTPJWKSSource().keyForKID(context.Background(), srv.URL, ""); err == nil {
			t.Fatal("an empty kid must be refused before any request")
		}
		if srv.Hits() != 0 {
			t.Errorf("no request should be made for an empty kid; hits=%d", srv.Hits())
		}
	})

	t.Run("failure is negatively cached briefly", func(t *testing.T) {
		// During a JWKS outage every Bearer request would otherwise pay the full
		// 5s timeout. One failed attempt suppresses retries for a short window.
		srv := newJWKSTestServer(t, "")
		srv.SetBody("boom", http.StatusInternalServerError)
		src := newHTTPJWKSSource()

		if _, err := src.keyForKID(context.Background(), srv.URL, testClerkKID); err == nil {
			t.Fatal("expected error")
		}
		if _, err := src.keyForKID(context.Background(), srv.URL, testClerkKID); err == nil {
			t.Fatal("expected error")
		}
		if srv.Hits() != 1 {
			t.Errorf("a repeat lookup inside the failure window must not refetch; hits=%d want 1", srv.Hits())
		}
	})
}

func TestHTTPJWKSSource_RejectsUnsuitableKeysAtLoadTime(t *testing.T) {
	priv := newTestKeypair(t)

	cases := []struct{ name, body string }{
		{"EC key", ecJWKSJSON(testClerkKID, "sig")},
		{`use "enc"`, rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "enc", "RS256")},
		{"alg not RS256", rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "sig", "RS512")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newJWKSTestServer(t, c.body)
			if _, err := newHTTPJWKSSource().keyForKID(context.Background(), srv.URL, testClerkKID); err == nil {
				t.Fatalf("%s must be filtered out at load time, not handed to the verifier", c.name)
			}
		})
	}
}

func TestHTTPJWKSSource_RequestsWellKnownPath(t *testing.T) {
	priv := newTestKeypair(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(rsaJWKSJSON(&priv.PublicKey, testClerkKID, "RSA", "sig", "RS256")))
	}))
	defer srv.Close()

	if _, err := newHTTPJWKSSource().keyForKID(context.Background(), srv.URL+"/", testClerkKID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/.well-known/jwks.json" {
		t.Errorf("JWKS path: got %q want /.well-known/jwks.json (and a trailing slash on the issuer must not double up)", gotPath)
	}
}

// =====================================================================
// extractClerkToken — Bearer only. Cookie-blind.
// =====================================================================

func TestExtractClerkToken(t *testing.T) {
	t.Run("bearer header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer my-jwt-1.2.3")
		tok, err := extractClerkToken(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok != "my-jwt-1.2.3" {
			t.Errorf("got %q", tok)
		}
	})

	t.Run("empty bearer is no token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer ")
		if _, err := extractClerkToken(r); !errors.Is(err, errNoClerkToken) {
			t.Fatalf("expected errNoClerkToken, got %v", err)
		}
	})

	t.Run("no header is no token", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, err := extractClerkToken(r); !errors.Is(err, errNoClerkToken) {
			t.Fatalf("expected errNoClerkToken, got %v", err)
		}
	})

	t.Run("__session cookie is never read", func(t *testing.T) {
		// Replaces the old CookieFallback test. pennyai shares an apex with
		// other Rally apps; a __session cookie present on the request must
		// NOT authenticate anyone here.
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: "__session", Value: "cookie-token-abc"})
		if _, err := extractClerkToken(r); !errors.Is(err, errNoClerkToken) {
			t.Fatalf("the __session cookie must be ignored entirely; got %v", err)
		}
	})

	t.Run("bearer wins and cookie is not consulted", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer header-token-xyz")
		r.AddCookie(&http.Cookie{Name: "__session", Value: "cookie-token-abc"})
		tok, err := extractClerkToken(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok != "header-token-xyz" {
			t.Errorf("got %q want the Bearer token", tok)
		}
	})
}

// =====================================================================
// VerifyClerkRequest — the per-request seam. Fails open to the legacy
// session path (nil, nil), never 500s a legacy user.
// =====================================================================

func TestVerifyClerkRequest_KillSwitchOff(t *testing.T) {
	priv := newTestKeypair(t)
	stub := withClerkSatelliteEnv(t, priv, map[string]string{"CLERK_ENABLED": "0"})

	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_x"))

	sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) {
		t.Fatal("resolver must not run when the kill switch is off")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("kill-switch-off must not error: %v", err)
	}
	if sess != nil {
		t.Errorf("kill-switch-off must return a nil session, got %+v", sess)
	}
	if stub.Lookups() != 0 {
		t.Errorf("no JWKS lookup when disabled; lookups=%d", stub.Lookups())
	}
}

func TestVerifyClerkRequest_HappyPath(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_happy"))

	want := &Session{UserID: "local-1", PhoneNumber: "5551234567", AuthMethod: "clerk"}
	var calls int32
	var gotID ClerkIdentity
	sess, err := VerifyClerkRequest(r, func(id ClerkIdentity) (*Session, error) {
		atomic.AddInt32(&calls, 1)
		gotID = id
		return want, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess != want {
		t.Errorf("session: got %+v want %+v", sess, want)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("resolver calls: got %d want 1", calls)
	}
	if gotID.ClerkUserID != "user_happy" {
		t.Errorf("resolver received ClerkUserID %q", gotID.ClerkUserID)
	}
}

func TestVerifyClerkRequest_NoBearerIsAnonymous(t *testing.T) {
	priv := newTestKeypair(t)
	stub := withClerkSatelliteEnv(t, priv, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) {
		t.Fatal("resolver must not run without a Bearer token")
		return nil, nil
	})
	if err != nil || sess != nil {
		t.Fatalf("no-token must be (nil, nil); got %+v, %v", sess, err)
	}
	if stub.Lookups() != 0 {
		t.Errorf("no JWKS lookup on the no-token fast path; lookups=%d", stub.Lookups())
	}
}

func TestVerifyClerkRequest_CookieOnlyIsAnonymous(t *testing.T) {
	priv := newTestKeypair(t)
	stub := withClerkSatelliteEnv(t, priv, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.AddCookie(&http.Cookie{Name: "__session", Value: mintValidClerkJWT(t, priv, "user_x")})

	sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) {
		t.Fatal("a __session cookie must never authenticate a request here")
		return nil, nil
	})
	if err != nil || sess != nil {
		t.Fatalf("cookie-only must be (nil, nil); got %+v, %v", sess, err)
	}
	if stub.Lookups() != 0 {
		t.Errorf("the JWKS must never be consulted for a cookie; lookups=%d", stub.Lookups())
	}
}

func TestVerifyClerkRequest_BearerWinsCookieIgnored(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_bearer"))
	r.AddCookie(&http.Cookie{Name: "__session", Value: mintValidClerkJWT(t, priv, "user_cookie")})

	var got string
	if _, err := VerifyClerkRequest(r, func(id ClerkIdentity) (*Session, error) {
		got = id.ClerkUserID
		return &Session{UserID: "local-1"}, nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "user_bearer" {
		t.Errorf("identity came from the cookie, not the Bearer header: %q", got)
	}
}

func TestVerifyClerkRequest_InvalidTokenFallsThroughToLegacy(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	o := defaultClerkTokenOpts()
	o.azp = "https://attacker.example"
	r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
	r.Header.Set("Authorization", "Bearer "+mintClerkTestJWT(t, priv, o))

	called := false
	sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) {
		called = true
		return &Session{UserID: "local-1"}, nil
	})
	if err != nil {
		t.Fatalf("an invalid Clerk token must not error the request — the legacy cookie path still has to run; got %v", err)
	}
	if sess != nil {
		t.Errorf("invalid token must yield a nil session, got %+v", sess)
	}
	if called {
		t.Error("resolver must not run for an unverified token")
	}
}

func TestVerifyClerkRequest_ResolverEdges(t *testing.T) {
	priv := newTestKeypair(t)

	t.Run("nil resolver", func(t *testing.T) {
		withClerkSatelliteEnv(t, priv, nil)
		r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
		r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_x"))
		sess, err := VerifyClerkRequest(r, nil)
		if err != nil || sess != nil {
			t.Fatalf("nil resolver must be (nil, nil); got %+v, %v", sess, err)
		}
	})

	t.Run("no local row yet", func(t *testing.T) {
		withClerkSatelliteEnv(t, priv, nil)
		r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
		r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_x"))
		sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) { return nil, nil })
		if err != nil || sess != nil {
			t.Fatalf("a verified Clerk user with no local row is anonymous, not an error; got %+v, %v", sess, err)
		}
	})

	t.Run("resolver error propagates", func(t *testing.T) {
		withClerkSatelliteEnv(t, priv, nil)
		dbErr := errors.New("db connection refused")
		r := httptest.NewRequest(http.MethodGet, "/api/list", nil)
		r.Header.Set("Authorization", "Bearer "+mintValidClerkJWT(t, priv, "user_x"))
		sess, err := VerifyClerkRequest(r, func(ClerkIdentity) (*Session, error) { return nil, dbErr })
		if !errors.Is(err, dbErr) {
			t.Fatalf("resolver error must propagate; got %v", err)
		}
		if sess != nil {
			t.Errorf("error path must not return a session, got %+v", sess)
		}
	})
}

// =====================================================================
// clerkForbiddenSecrets — pennyai must never hold a Clerk secret
// =====================================================================

func TestClerkForbiddenSecrets(t *testing.T) {
	t.Run("none set", func(t *testing.T) {
		t.Setenv("CLERK_SECRET_KEY", "")
		t.Setenv("CLERK_WEBHOOK_SIGNING_SECRET", "")
		if got := clerkForbiddenSecrets(); len(got) != 0 {
			t.Errorf("got %#v want none", got)
		}
	})

	t.Run("secret key set", func(t *testing.T) {
		t.Setenv("CLERK_SECRET_KEY", "sk_test_whatever")
		t.Setenv("CLERK_WEBHOOK_SIGNING_SECRET", "")
		got := clerkForbiddenSecrets()
		if len(got) != 1 || got[0] != "CLERK_SECRET_KEY" {
			t.Errorf("got %#v want [CLERK_SECRET_KEY]", got)
		}
	})

	t.Run("webhook secret set", func(t *testing.T) {
		t.Setenv("CLERK_SECRET_KEY", "")
		t.Setenv("CLERK_WEBHOOK_SIGNING_SECRET", "whsec_whatever")
		got := clerkForbiddenSecrets()
		if len(got) != 1 || got[0] != "CLERK_WEBHOOK_SIGNING_SECRET" {
			t.Errorf("got %#v want [CLERK_WEBHOOK_SIGNING_SECRET]", got)
		}
	})

	t.Run("both set", func(t *testing.T) {
		t.Setenv("CLERK_SECRET_KEY", "sk_test_whatever")
		t.Setenv("CLERK_WEBHOOK_SIGNING_SECRET", "whsec_whatever")
		got := clerkForbiddenSecrets()
		if len(got) != 2 || got[0] != "CLERK_SECRET_KEY" || got[1] != "CLERK_WEBHOOK_SIGNING_SECRET" {
			t.Errorf("got %#v want both, secret key first", got)
		}
	})

	t.Run("refused even when the kill switch is off", func(t *testing.T) {
		// Holding the secret IS the misconfiguration; the flag is irrelevant.
		t.Setenv("CLERK_ENABLED", "0")
		t.Setenv("CLERK_SECRET_KEY", "sk_test_whatever")
		t.Setenv("CLERK_WEBHOOK_SIGNING_SECRET", "")
		if got := clerkForbiddenSecrets(); len(got) != 1 {
			t.Errorf("got %#v — the check must not depend on CLERK_ENABLED", got)
		}
	})
}

// =====================================================================
// Kill switch hot-read
// =====================================================================

func TestClerkIsEnabled(t *testing.T) {
	for _, c := range []struct {
		val  string
		want bool
	}{{"1", true}, {"0", false}, {"", false}, {"true", false}, {"yes", false}} {
		t.Run("CLERK_ENABLED="+c.val, func(t *testing.T) {
			t.Setenv("CLERK_ENABLED", c.val)
			if got := clerkIsEnabled(); got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestVerifyClerkToken_PhoneVerifiedAcceptsBoolOrString(t *testing.T) {
	priv := newTestKeypair(t)
	withClerkSatelliteEnv(t, priv, nil)

	cases := []struct {
		name string
		val  any
		want bool
	}{
		{"bool true", true, true},
		{"string true", "true", true},
		{"bool false", false, false},
		{"string false", "false", false},
		{"unexpected string", "yes", false},
		{"number", 1, false},
		{"null", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := defaultClerkTokenOpts()
			o.extra = map[string]any{"phone": "+15551234567", "phone_verified": c.val}
			id, err := VerifyClerkToken(context.Background(), mintClerkTestJWT(t, priv, o))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id.PhoneVerified != c.want {
				t.Errorf("PhoneVerified: got %v want %v", id.PhoneVerified, c.want)
			}
		})
	}
}
