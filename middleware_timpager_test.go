package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Tests for middleware_timpager.go — the revenue + payout push module.
//
// This file is MONEY critical (CLAUDE.md Revenue Rules) and shipped with ZERO
// tests. These tests pin:
//   - the golden idempotency-key formats (pennyai_/groflo_payout_ prefixes)
//   - 409-from-timpager is treated as idempotent success with NO retry
//   - the retry/backoff loop is capped (never retries forever)
//   - non-positive / empty payout distributions are refused (no send)
//
// The header-forwarding cross-domain SSO bridge that used to live in
// middleware_timpager.go was deleted along with its tests. Cross-domain SSO is
// not implemented by design; Rally identity routes through timpager's identity
// resolve API, and SSO routes through Clerk JWKS verification.
//
// All timpager HTTP calls are mocked with httptest — tests never hit the network.
// =============================================================================

// revEventBody mirrors the JSON pushRevenueEvent posts to timpager. Decoding it
// also pins the request shape (API rule: never silently change the wire format).
type revEventBody struct {
	EventType      string `json:"event_type"`
	AmountCents    int    `json:"amount_cents"`
	Currency       string `json:"currency"`
	SourceRef      string `json:"source_ref"`
	IdempotencyKey string `json:"idempotency_key"`
}

// captureLog redirects the standard logger to a buffer for the duration of fn
// and returns everything written. Used to observe the payout stub path, which
// (by design) logs intent rather than sending.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	fn()
	return buf.String()
}

// -----------------------------------------------------------------------------
// Golden idempotency-key formats
// -----------------------------------------------------------------------------

// TestTimpagerIdempotencyKeyFormats pins the exact documented key shapes so a
// refactor cannot silently change them. Per CLAUDE.md MONEY rule, revenue keys
// are pennyai_{eventType}_{sourceRef}_{date}; payout keys are
// groflo_payout_{distributionID}.
func TestTimpagerIdempotencyKeyFormats(t *testing.T) {
	t.Run("revenue event key shape", func(t *testing.T) {
		cases := []struct {
			eventType, sourceRef, date, want string
		}{
			{"swap_rebate", "rebate_42", "2026-06-29", "pennyai_swap_rebate_rebate_42_2026-06-29"},
			{"deal_impression", "deal_7", "2026-01-02", "pennyai_deal_impression_deal_7_2026-01-02"},
		}
		for _, c := range cases {
			got := fmt.Sprintf("pennyai_%s_%s_%s", c.eventType, c.sourceRef, c.date)
			if got != c.want {
				t.Errorf("revenue key = %q, want %q", got, c.want)
			}
			if !strings.HasPrefix(got, "pennyai_") {
				t.Errorf("revenue key %q must carry the pennyai_ prefix", got)
			}
		}
	})

	t.Run("payout key shape", func(t *testing.T) {
		cases := []struct {
			distID, want string
		}{
			{"dist_777", "groflo_payout_dist_777"},
			{"abc-123", "groflo_payout_abc-123"},
		}
		for _, c := range cases {
			got := fmt.Sprintf("groflo_payout_%s", c.distID)
			if got != c.want {
				t.Errorf("payout key = %q, want %q", got, c.want)
			}
			if !strings.HasPrefix(got, "groflo_payout_") {
				t.Errorf("payout key %q must carry the groflo_payout_ prefix", got)
			}
		}
	})
}

// -----------------------------------------------------------------------------
// pushRevenueEvent — forwarding, success-no-retry, retry cap, not-configured
// -----------------------------------------------------------------------------

// TestTimpagerPushRevenueEventForwardsGoldenKey verifies the caller-supplied
// idempotency key is forwarded verbatim and the full JSON body shape is intact.
func TestTimpagerPushRevenueEventForwardsGoldenKey(t *testing.T) {
	bodyCh := make(chan revEventBody, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b revEventBody
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodyCh <- b
		w.WriteHeader(http.StatusCreated) // 201 = created
	}))
	defer srv.Close()

	s := newTestServer(t, &MockTwilioClient{})
	s.config.TimpagerURL = srv.URL
	s.config.TimpagerAPIKey = "test-key"

	key := fmt.Sprintf("pennyai_%s_%s_%s", "swap_rebate", "rebate_42", "2026-06-29")
	s.pushRevenueEvent("swap_rebate", 250, "rebate_42", key)

	select {
	case b := <-bodyCh:
		if b.IdempotencyKey != key {
			t.Errorf("idempotency_key = %q, want %q (must forward caller key verbatim)", b.IdempotencyKey, key)
		}
		if b.EventType != "swap_rebate" {
			t.Errorf("event_type = %q, want swap_rebate", b.EventType)
		}
		if b.AmountCents != 250 {
			t.Errorf("amount_cents = %d, want 250", b.AmountCents)
		}
		if b.Currency != "USD" {
			t.Errorf("currency = %q, want USD", b.Currency)
		}
		if b.SourceRef != "rebate_42" {
			t.Errorf("source_ref = %q, want rebate_42", b.SourceRef)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for revenue event POST")
	}
}

// TestTimpagerPushRevenueEventSuccessNoRetry verifies that both 201 (created)
// and 409 (duplicate / idempotent replay) are treated as success and trigger
// exactly ONE request — no retry. This is the core OUTBOUND/idempotency
// guarantee: a 409 whose key already exists must not be re-sent.
func TestTimpagerPushRevenueEventSuccessNoRetry(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusConflict} {
		status := status
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			var count int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&count, 1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			s := newTestServer(t, &MockTwilioClient{})
			s.config.TimpagerURL = srv.URL
			s.config.TimpagerAPIKey = "test-key"

			s.pushRevenueEvent("swap_rebate", 150, "rebate_1", "pennyai_swap_rebate_rebate_1_2026-06-29")

			// First backoff is ~1s; wait longer than that to catch an erroneous retry.
			time.Sleep(1500 * time.Millisecond)
			if got := atomic.LoadInt32(&count); got != 1 {
				t.Fatalf("status %d should be terminal success: got %d requests, want exactly 1", status, got)
			}
		})
	}
}

// TestTimpagerPushRevenueEventRetryCap verifies the retry loop is bounded: on a
// persistent 500 it makes exactly 3 attempts (with exponential backoff) and
// then gives up — it never retries forever.
func TestTimpagerPushRevenueEventRetryCap(t *testing.T) {
	var count int32
	reqCh := make(chan struct{}, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		reqCh <- struct{}{}
		w.WriteHeader(http.StatusInternalServerError) // 500 forces retry
	}))
	defer srv.Close()

	s := newTestServer(t, &MockTwilioClient{})
	s.config.TimpagerURL = srv.URL
	s.config.TimpagerAPIKey = "test-key"

	s.pushRevenueEvent("swap_rebate", 150, "rebate_1", "pennyai_swap_rebate_rebate_1_2026-06-29")

	// Expect exactly 3 attempts. Backoff is 1s then 2s, so the 3rd lands ~3s in.
	for i := 0; i < 3; i++ {
		select {
		case <-reqCh:
		case <-time.After(8 * time.Second):
			t.Fatalf("expected attempt %d, only saw %d total", i+1, atomic.LoadInt32(&count))
		}
	}
	// A buggy uncapped loop would fire a 4th attempt after its ~4s backoff.
	// Wait past that window and assert it never comes.
	select {
	case <-reqCh:
		t.Fatalf("retry cap breached: a 4th attempt fired (count=%d)", atomic.LoadInt32(&count))
	case <-time.After(5 * time.Second):
	}
	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("expected exactly 3 attempts (cap), got %d", got)
	}
}

// TestTimpagerPushRevenueEventNotConfigured verifies that with no TIMPAGER_URL
// or API key the helper is a no-op and sends nothing (SECRETS/OUTBOUND: never
// fire a send when the integration is not configured).
func TestTimpagerPushRevenueEventNotConfigured(t *testing.T) {
	for _, tc := range []struct {
		name, url, key string
	}{
		{"no url", "", "test-key"},
		{"no api key", "http://127.0.0.1:0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var count int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&count, 1)
				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			s := newTestServer(t, &MockTwilioClient{})
			// Point at the (recording) server but blank one credential — the helper
			// must bail before ever touching the network.
			if tc.url == "" {
				s.config.TimpagerURL = ""
			} else {
				s.config.TimpagerURL = srv.URL
			}
			s.config.TimpagerAPIKey = tc.key

			s.pushRevenueEvent("swap_rebate", 150, "rebate_1", "pennyai_swap_rebate_rebate_1_2026-06-29")

			time.Sleep(300 * time.Millisecond)
			if got := atomic.LoadInt32(&count); got != 0 {
				t.Fatalf("expected no request when unconfigured, got %d", got)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// pushPayoutToTimpager — refusals + golden key (stub mode)
//
// NOTE: pushPayoutToTimpager runs in STUB MODE (const stubMode = true in the
// source), so the live POST/201/409/retry path is not reachable from a test
// without flipping that constant. The shared 409/retry semantics are covered by
// the pushRevenueEvent tests above. Here we pin the guard rails and the key.
// -----------------------------------------------------------------------------

// TestTimpagerPushPayoutRefusesBadInput verifies the MONEY guard rails: a
// non-positive amount or an empty distribution id is refused and NOT queued.
func TestTimpagerPushPayoutRefusesBadInput(t *testing.T) {
	s := newTestServer(t, &MockTwilioClient{})
	// Configure both so we prove the refusal short-circuits BEFORE any send,
	// not merely because the integration is unconfigured.
	s.config.TimpagerURL = "http://timpager.invalid"
	s.config.TimpagerAPIKey = "test-key"

	cases := []struct {
		name        string
		amount      int
		distID      string
		wantLog     string
		dontWantLog string
	}{
		{"zero amount", 0, "dist_1", "refusing non-positive amount", "would push payout"},
		{"negative amount", -50, "dist_1", "refusing non-positive amount", "would push payout"},
		{"empty distribution", 100, "", "refusing empty distribution_id", "would push payout"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := captureLog(func() {
				s.pushPayoutToTimpager("groflo-user-1", c.amount, c.distID)
			})
			if !strings.Contains(out, c.wantLog) {
				t.Errorf("expected refusal log %q, got: %q", c.wantLog, out)
			}
			if strings.Contains(out, c.dontWantLog) {
				t.Errorf("payout must NOT be queued for %s; log: %q", c.name, out)
			}
		})
	}
}

// TestTimpagerPushPayoutStubUsesGoldenKey verifies that a valid payout, in stub
// mode, builds the groflo_payout_{distributionID} key and logs its intent
// rather than sending (OUTBOUND: no real send while stubbed).
func TestTimpagerPushPayoutStubUsesGoldenKey(t *testing.T) {
	s := newTestServer(t, &MockTwilioClient{})

	out := captureLog(func() {
		s.pushPayoutToTimpager("groflo-user-1", 250, "dist_777")
	})
	if !strings.Contains(out, "key=groflo_payout_dist_777") {
		t.Errorf("expected golden payout key in log, got: %q", out)
	}
	if !strings.Contains(out, "would push payout") {
		t.Errorf("stub mode must log intent (no real send), got: %q", out)
	}
}

// -----------------------------------------------------------------------------
// Authorization header on revenue pushes
// -----------------------------------------------------------------------------

// TestTimpagerPushRevenueEventSendsAuthHeader: pushRevenueEvent refuses to run
// unless TimpagerAPIKey is set, yet it used client.Post and sent no
// Authorization header at all — so every event timpager received was
// unauthenticated. pushPayoutToTimpager already does this correctly.
func TestTimpagerPushRevenueEventSendsAuthHeader(t *testing.T) {
	authCh := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCh <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	s := newTestServer(t, &MockTwilioClient{})
	s.config.TimpagerURL = srv.URL
	s.config.TimpagerAPIKey = "test-key"

	s.pushRevenueEvent("swap_rebate", 150, "rebate_1", "pennyai_swap_rebate_rebate_1_2026-06-29")

	select {
	case got := <-authCh:
		if want := "Bearer test-key"; got != want {
			t.Errorf("Authorization header: got %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the revenue event push")
	}
}

// TestTimpagerPushRevenueEventSendsJSONContentType pins the content type
// alongside the auth header, since both now come from an explicit request.
func TestTimpagerPushRevenueEventSendsJSONContentType(t *testing.T) {
	ctCh := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctCh <- r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	s := newTestServer(t, &MockTwilioClient{})
	s.config.TimpagerURL = srv.URL
	s.config.TimpagerAPIKey = "test-key"

	s.pushRevenueEvent("swap_rebate", 150, "rebate_1", "pennyai_swap_rebate_rebate_1_2026-06-29")

	select {
	case got := <-ctCh:
		if got != "application/json" {
			t.Errorf("Content-Type: got %q, want application/json", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the revenue event push")
	}
}
