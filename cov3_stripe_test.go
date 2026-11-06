package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func stripeSig(secret string, ts int64, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts, payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyStripeSignature(t *testing.T) {
	const secret = "whsec_test"
	const payload = `{"id":"evt_1","type":"checkout.session.completed"}`
	now := time.Now().Unix()
	good := stripeSig(secret, now, payload)

	cases := []struct {
		name    string
		payload string
		header  string
		secret  string
		want    bool
	}{
		{"valid", payload, fmt.Sprintf("t=%d,v1=%s", now, good), secret, true},
		{"valid, reordered parts", payload, fmt.Sprintf("v1=%s,t=%d", good, now), secret, true},
		{"wrong secret", payload, fmt.Sprintf("t=%d,v1=%s", now, good), "whsec_other", false},
		{"tampered body", payload + " ", fmt.Sprintf("t=%d,v1=%s", now, good), secret, false},
		{"stale timestamp, validly signed", payload, fmt.Sprintf("t=%d,v1=%s", now-361, stripeSig(secret, now-361, payload)), secret, false},
		{"far-future timestamp, validly signed", payload, fmt.Sprintf("t=%d,v1=%s", now+600, stripeSig(secret, now+600, payload)), secret, false},
		{"within window (4 min old)", payload, fmt.Sprintf("t=%d,v1=%s", now-240, stripeSig(secret, now-240, payload)), secret, true},
		{"timestamp altered after signing", payload, fmt.Sprintf("t=%d,v1=%s", now-1, good), secret, false},
		{"empty header", payload, "", secret, false},
		{"garbage header", payload, "not-a-signature", secret, false},
		{"missing timestamp", payload, "v1=" + good, secret, false},
		{"missing v1", payload, fmt.Sprintf("t=%d", now), secret, false},
		{"only v0 scheme", payload, fmt.Sprintf("t=%d,v0=%s", now, good), secret, false},
		{"non-numeric timestamp", payload, "t=abc,v1=" + good, secret, false},
		{"empty v1 value", payload, fmt.Sprintf("t=%d,v1=", now), secret, false},
		{"truncated signature", payload, fmt.Sprintf("t=%d,v1=%s", now, good[:10]), secret, false},
		{"multiple v1, bad first good last", payload, fmt.Sprintf("t=%d,v1=deadbeef,v1=%s", now, good), secret, true},
		{"multiple v1, good first bad last", payload, fmt.Sprintf("t=%d,v1=%s,v1=deadbeef", now, good), secret, true},
		{"multiple v1, all bad", payload, fmt.Sprintf("t=%d,v1=deadbeef,v1=cafebabe", now), secret, false},
		{"uppercase hex rejected", payload, fmt.Sprintf("t=%d,v1=%s", now, strings.ToUpper(good)), secret, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifyStripeSignature([]byte(tc.payload), tc.header, tc.secret); got != tc.want {
				t.Errorf("verifyStripeSignature = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVerifyStripeSignature_EmptyKeySignatureNeverVerifiesAgainstRealSecret(t *testing.T) {
	now := time.Now().Unix()
	sig := stripeSig("", now, "x")
	if verifyStripeSignature([]byte("x"), fmt.Sprintf("t=%d,v1=%s", now, sig), "real") {
		t.Error("empty-key signature must not verify against a real secret")
	}
}

func TestStripeWebhook_RejectsBadSignatureWhenSecretConfigured(t *testing.T) {
	server := newTestServer(t)
	server.config.StripeWebhookSecret = "whsec_test"
	body := `{"id":"evt_x","type":"checkout.session.completed"}`

	rr := doRequest(server.handleStripeWebhook, "POST", "/stripe/webhook", nil, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("no signature: got %d, want 400", rr.Code)
	}

	now := time.Now().Unix()
	req := httptest.NewRequest("POST", "/stripe/webhook", strings.NewReader(body))
	req.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", now, stripeSig("wrong", now, body)))
	rec := httptest.NewRecorder()
	server.handleStripeWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("wrong secret: got %d, want 400", rec.Code)
	}
}

// =============================================================================
// handleBrandDeposit
// =============================================================================

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// stubDefaultClient replaces http.DefaultClient's transport so no test reaches
// the network. Restored on cleanup.
func stubDefaultClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = fn
	t.Cleanup(func() { http.DefaultClient.Transport = old })
}

func stubResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func brandBalance(t *testing.T, server *Server, brandUserID int) int {
	t.Helper()
	var bal int
	if err := server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&bal); err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal
}

func brandTxnCount(t *testing.T, server *Server, brandUserID int) int {
	t.Helper()
	var n int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_transactions WHERE brand_user_id = ?", brandUserID).Scan(&n)
	return n
}

func TestBrandDeposit_Auth(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 10000}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: got %d, want 401", rr.Code)
	}
	bad := &http.Cookie{Name: "brand_session", Value: "nope"}
	rr = doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 10000}, []*http.Cookie{bad})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("bad cookie: got %d, want 401", rr.Code)
	}
}

func TestBrandDeposit_MethodAndBadBody(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := brandRegister(t, server, "dep-method@brand.com", "password1234", "MethodCo")
	rr := doRequest(server.handleBrandDeposit, "GET", "/brand/deposit", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d, want 405", rr.Code)
	}
	rr = doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", "not an object", []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad body: got %d, want 400", rr.Code)
	}
}

func TestBrandDeposit_BelowMinimumAndNegativeDoNotMoveBalance(t *testing.T) {
	server := newTestServer(t)
	cookie, id := brandRegister(t, server, "dep-neg@brand.com", "password1234", "NegCo")
	for _, amt := range []int{0, -10000, 4999} {
		rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": amt}, []*http.Cookie{cookie})
		if rr.Code != http.StatusBadRequest {
			t.Errorf("amount %d: got %d, want 400", amt, rr.Code)
		}
	}
	if b := brandBalance(t, server, id); b != 0 {
		t.Errorf("balance moved to %d", b)
	}
	if n := brandTxnCount(t, server, id); n != 0 {
		t.Errorf("%d ledger rows written", n)
	}
	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 5000}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("minimum: got %d, want 200", rr.Code)
	}
}

func TestBrandDeposit_TestModeLedgerAndCrossBrandIsolation(t *testing.T) {
	server := newTestServer(t)
	cookieA, idA := brandRegister(t, server, "dep-a@brand.com", "password1234", "A Co")
	_, idB := brandRegister(t, server, "dep-b@brand.com", "password1234", "B Co")

	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 7500}, []*http.Cookie{cookieA})
	if rr.Code != http.StatusOK {
		t.Fatalf("deposit: %d %s", rr.Code, rr.Body.String())
	}
	if b := brandBalance(t, server, idA); b != 7500 {
		t.Errorf("A balance = %d, want 7500", b)
	}
	if n := brandTxnCount(t, server, idA); n != 1 {
		t.Errorf("A ledger rows = %d, want 1", n)
	}
	if b := brandBalance(t, server, idB); b != 0 {
		t.Errorf("B balance = %d, want 0", b)
	}
	if n := brandTxnCount(t, server, idB); n != 0 {
		t.Errorf("B ledger rows = %d, want 0", n)
	}
	// A body brand_user_id is ignored; the session decides who is credited.
	doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]interface{}{"amount_cents": 5000, "brand_user_id": idB}, []*http.Cookie{cookieA})
	if b := brandBalance(t, server, idB); b != 0 {
		t.Errorf("B credited via body field: %d", b)
	}
}

func TestBrandDeposit_StripeCheckoutRedirect(t *testing.T) {
	server := newTestServer(t)
	server.config.StripeSecretKey = "sk_test_stub"
	cookie, id := brandRegister(t, server, "dep-stripe@brand.com", "password1234", "StripeCo")

	var gotURL, gotBody, gotAuth string
	calls := 0
	stubDefaultClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		gotURL = r.URL.String()
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotAuth = r.Header.Get("Authorization")
		return stubResponse(200, `{"id":"cs_test_1","url":"https://checkout.stripe.com/c/pay/cs_test_1"}`), nil
	})

	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 12345}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	if parseJSON(rr)["checkout_url"] != "https://checkout.stripe.com/c/pay/cs_test_1" {
		t.Errorf("body = %s", rr.Body.String())
	}
	if gotURL != "https://api.stripe.com/v1/checkout/sessions" || calls != 1 {
		t.Errorf("url=%q calls=%d", gotURL, calls)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("auth header = %q", gotAuth)
	}
	form, _ := url.ParseQuery(gotBody)
	if form.Get("payment_intent_data[metadata][source]") != "pennyai" {
		t.Errorf("metadata source missing: %v", form)
	}
	if form.Get("payment_intent_data[metadata][brand_user_id]") != fmt.Sprint(id) {
		t.Errorf("metadata brand_user_id = %q, want %d", form.Get("payment_intent_data[metadata][brand_user_id]"), id)
	}
	if form.Get("line_items[0][price_data][unit_amount]") != "12345" {
		t.Errorf("unit_amount = %q", form.Get("line_items[0][price_data][unit_amount]"))
	}
	// Redirect flow must not credit anything: only the verified webhook does.
	if b := brandBalance(t, server, id); b != 0 {
		t.Errorf("balance credited before payment: %d", b)
	}
	if n := brandTxnCount(t, server, id); n != 0 {
		t.Errorf("ledger rows before payment: %d", n)
	}
}

func TestBrandDeposit_StripeFailures(t *testing.T) {
	server := newTestServer(t)
	server.config.StripeSecretKey = "sk_test_stub"
	cookie, id := brandRegister(t, server, "dep-stripefail@brand.com", "password1234", "FailCo")

	t.Run("transport error", func(t *testing.T) {
		stubDefaultClient(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("boom") })
		rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 6000}, []*http.Cookie{cookie})
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got %d, want 500", rr.Code)
		}
	})
	t.Run("stripe error body has no url", func(t *testing.T) {
		stubDefaultClient(t, func(r *http.Request) (*http.Response, error) {
			return stubResponse(401, `{"error":{"message":"Invalid API Key"}}`), nil
		})
		rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 6000}, []*http.Cookie{cookie})
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got %d, want 500", rr.Code)
		}
		if strings.Contains(rr.Body.String(), "Invalid API Key") {
			t.Error("Stripe error text leaked to client")
		}
	})
	t.Run("unparseable body", func(t *testing.T) {
		stubDefaultClient(t, func(r *http.Request) (*http.Response, error) { return stubResponse(200, "<html>"), nil })
		rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]int{"amount_cents": 6000}, []*http.Cookie{cookie})
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got %d, want 500", rr.Code)
		}
	})
	if b := brandBalance(t, server, id); b != 0 {
		t.Errorf("balance moved on failure: %d", b)
	}
	if n := brandTxnCount(t, server, id); n != 0 {
		t.Errorf("ledger rows on failure: %d", n)
	}
}
