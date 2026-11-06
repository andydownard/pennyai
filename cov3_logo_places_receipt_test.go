package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubDefaultTransport replaces http.DefaultTransport (used by code that builds
// its own http.Client) so nothing reaches the network. Restored on cleanup.
func stubDefaultTransport(t *testing.T, fn roundTripFunc) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = old })
}

// =============================================================================
// handleBrands
// =============================================================================

// clearSeededBrands removes the brands seeded by initDB so a test controls the
// whole table (warmLogoCache sleeps 500ms per brand that lacks a logo).
func clearSeededBrands(t *testing.T, server *Server) {
	t.Helper()
	if _, err := server.db.Exec("DELETE FROM brands"); err != nil {
		t.Fatalf("clearing brands: %v", err)
	}
}

func TestHandleBrands(t *testing.T) {
	server := newTestServer(t)
	clearSeededBrands(t, server)

	// Empty table: an empty JSON object, not null.
	rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != "{}" {
		t.Fatalf("empty: %d %q", rr.Code, rr.Body.String())
	}

	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "Cheerios", "cheerios.com")
	server.db.Exec("INSERT INTO brands (name, domain) VALUES (?, ?)", "TIDE", "tide.com")

	for i := 0; i < 2; i++ { // idempotent read
		rr = doRequest(server.handleBrands, "GET", "/api/brands", nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("got %d", rr.Code)
		}
		var got map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got["cheerios"] != "cheerios.com" || got["tide"] != "tide.com" {
			t.Errorf("names must be lower-cased: %v", got)
		}
		if rr.Header().Get("Cache-Control") != "public, max-age=3600" || rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("headers: %v", rr.Header())
		}
	}

	for _, m := range []string{"POST", "DELETE", "PUT"} {
		if rr := doRequest(server.handleBrands, m, "/api/brands", nil, nil); rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", m, rr.Code)
		}
	}

	server.db.Exec("DROP TABLE brand_logos")
	server.db.Exec("DROP TABLE brands CASCADE")
	if rr := doRequest(server.handleBrands, "GET", "/api/brands", nil, nil); rr.Code != http.StatusInternalServerError {
		t.Errorf("db error: got %d, want 500", rr.Code)
	}
}

// =============================================================================
// warmLogoCache
// =============================================================================

func pngBytes(n int) []byte {
	b := make([]byte, n)
	copy(b, []byte{0x89, 'P', 'N', 'G'})
	return b
}

func TestWarmLogoCache_LoadsStoredLogosWithoutNetworkWhenNoToken(t *testing.T) {
	server := newTestServer(t)
	clearSeededBrands(t, server)
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected outbound request to %s", r.URL)
		return nil, fmt.Errorf("blocked")
	})
	var calls int
	old := logoDevImageBaseURL
	logoDevImageBaseURL = "http://127.0.0.1:1/" // would fail; must not be hit
	t.Cleanup(func() { logoDevImageBaseURL = old })

	res, err := server.db.Exec("INSERT INTO brands (name, domain) VALUES ('Stored', 'Stored.COM')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO brand_logos (brand_id, logo_data, content_type) VALUES (?, ?, 'image/png')", id, pngBytes(300))
	// A brand with no logo and no token configured: stays missing, no fetch.
	server.db.Exec("INSERT INTO brands (name, domain) VALUES ('Missing', 'missing.com')")
	// A brand with an empty domain is skipped.
	server.db.Exec("INSERT INTO brands (name, domain) VALUES ('NoDomain', '')")
	_ = calls

	server.warmLogoCache()
	server.warmLogoCache() // idempotent

	v, ok := server.logoCache.Load("stored.com")
	if !ok {
		t.Fatal("stored logo not cached under lower-cased domain")
	}
	if cl := v.(*cachedLogo); len(cl.data) != 300 || cl.contentType != "image/png" {
		t.Errorf("cached logo = %d bytes %q", len(cl.data), cl.contentType)
	}
	if _, ok := server.logoCache.Load("missing.com"); ok {
		t.Error("missing logo should not be cached without a token")
	}
	var n int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_logos").Scan(&n)
	if n != 1 {
		t.Errorf("brand_logos rows = %d, want 1", n)
	}
}

func TestWarmLogoCache_FetchesMissingOnceAndPersists(t *testing.T) {
	server := newTestServer(t)
	clearSeededBrands(t, server)
	server.config.LogoDevToken = "pk_test"

	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[strings.TrimPrefix(r.URL.Path, "/")]++
		if r.URL.Query().Get("token") != "pk_test" {
			t.Errorf("token not sent: %s", r.URL.RawQuery)
		}
		if strings.Contains(r.URL.Path, "broken") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "tiny") {
			w.Write([]byte("xx")) // under the 200-byte floor: treated as no logo
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Write(pngBytes(400))
	}))
	defer srv.Close()
	old := logoDevImageBaseURL
	logoDevImageBaseURL = srv.URL + "/"
	t.Cleanup(func() { logoDevImageBaseURL = old })

	server.db.Exec("INSERT INTO brands (name, domain) VALUES ('Good', 'good.com')")
	server.db.Exec("INSERT INTO brands (name, domain) VALUES ('Broken', 'broken.com')")
	server.db.Exec("INSERT INTO brands (name, domain) VALUES ('Tiny', 'tiny.com')")

	server.warmLogoCache()

	v, ok := server.logoCache.Load("good.com")
	if !ok || v.(*cachedLogo).contentType != "image/webp" {
		t.Fatalf("good.com not cached: %v %v", v, ok)
	}
	for _, d := range []string{"broken.com", "tiny.com"} {
		if _, ok := server.logoCache.Load(d); ok {
			t.Errorf("%s should not be cached", d)
		}
	}
	var n int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_logos").Scan(&n)
	if n != 1 {
		t.Errorf("brand_logos rows = %d, want 1", n)
	}

	// Second run: the stored logo is not fetched again (no duplicate rows, no
	// repeat request); only the still-missing ones are retried.
	server.warmLogoCache()
	if hits["good.com"] != 1 {
		t.Errorf("good.com fetched %d times, want 1", hits["good.com"])
	}
	server.db.QueryRow("SELECT COUNT(*) FROM brand_logos").Scan(&n)
	if n != 1 {
		t.Errorf("after rerun brand_logos rows = %d, want 1", n)
	}
}

// =============================================================================
// Places proxy (Google outbound stubbed)
// =============================================================================

func TestPlacesAutocomplete(t *testing.T) {
	server := newTestServer(t)
	server.config.GooglePlacesAPIKey = "places-key"
	cookie, _ := loginUser(t, server, "5550010001")

	calls := 0
	var gotKey, gotBody, gotURL string
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		calls++
		gotURL = r.URL.String()
		gotKey = r.Header.Get("X-Goog-Api-Key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return stubResponse(200, `{"suggestions":[{"placePrediction":{"placeId":"abc","text":{"text":"1 Main St, Town"}}},{"placePrediction":{"placeId":"def","text":{"text":"2 Main St, Town"}}}]}`), nil
	})

	// Auth and method.
	if rr := doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=main+street", nil, nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("no session: got %d, want 401", rr.Code)
	}
	if rr := doRequest(server.handlePlacesAutocomplete, "POST", "/api/places/autocomplete?input=main", nil, []*http.Cookie{cookie}); rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rr.Code)
	}
	if calls != 0 {
		t.Fatalf("outbound call made before auth/method checks passed: %d", calls)
	}

	// Short input: empty list, no outbound call.
	rr := doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=ab", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != "[]" || calls != 0 {
		t.Errorf("short input: %d %q calls=%d", rr.Code, rr.Body.String(), calls)
	}

	// Happy path.
	rr = doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=main+street", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	var items []map[string]string
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 || items[0]["placeId"] != "abc" || items[0]["description"] != "1 Main St, Town" {
		t.Errorf("items = %v", items)
	}
	if calls != 1 || gotURL != "https://places.googleapis.com/v1/places:autocomplete" || gotKey != "places-key" {
		t.Errorf("calls=%d url=%q key=%q", calls, gotURL, gotKey)
	}
	if !strings.Contains(gotBody, `"input":"main street"`) {
		t.Errorf("body = %s", gotBody)
	}
	// The key never reaches the browser.
	if strings.Contains(rr.Body.String(), "places-key") || rr.Header().Get("X-Goog-Api-Key") != "" {
		t.Error("API key leaked to client")
	}

	// Quotes in input stay inside the JSON string.
	doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input="+`a%22%2C%22x%22%3A1`, nil, []*http.Cookie{cookie})
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil || parsed["x"] != nil {
		t.Errorf("input broke out of JSON: %v %s", err, gotBody)
	}

	// Upstream garbage: empty list.
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) { return stubResponse(200, "<html>"), nil })
	rr = doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=main+street", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Errorf("garbage: %d %q", rr.Code, rr.Body.String())
	}

	// Upstream down: 502.
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("down") })
	rr = doRequest(server.handlePlacesAutocomplete, "GET", "/api/places/autocomplete?input=main+street", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadGateway {
		t.Errorf("down: got %d, want 502", rr.Code)
	}
}

func TestPlacesDetail(t *testing.T) {
	server := newTestServer(t)
	server.config.GooglePlacesAPIKey = "places-key"
	cookie, _ := loginUser(t, server, "5550010002")

	calls := 0
	var gotURL, gotMask string
	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) {
		calls++
		gotURL = r.URL.String()
		gotMask = r.Header.Get("X-Goog-FieldMask")
		return stubResponse(200, `{"formattedAddress":"1 Main St, Town, ST 12345, USA"}`), nil
	})

	if rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=abc", nil, nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("no session: got %d, want 401", rr.Code)
	}
	if rr := doRequest(server.handlePlacesDetail, "POST", "/api/places/detail?place_id=abc", nil, []*http.Cookie{cookie}); rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rr.Code)
	}
	if rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail", nil, []*http.Cookie{cookie}); rr.Code != http.StatusBadRequest {
		t.Errorf("missing place_id: got %d, want 400", rr.Code)
	}
	if calls != 0 {
		t.Fatalf("outbound call before validation passed: %d", calls)
	}

	rr := doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK || parseJSON(rr)["address"] != "1 Main St, Town, ST 12345, USA" {
		t.Errorf("happy: %d %s", rr.Code, rr.Body.String())
	}
	if gotURL != "https://places.googleapis.com/v1/places/abc" || gotMask != "formattedAddress" {
		t.Errorf("url=%q mask=%q", gotURL, gotMask)
	}

	// Empty address or garbage: blank address, still 200.
	for _, body := range []string{`{"formattedAddress":""}`, `nope`} {
		b := body
		stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) { return stubResponse(200, b), nil })
		rr = doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=abc", nil, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK || parseJSON(rr)["address"] != "" {
			t.Errorf("body %q: %d %s", b, rr.Code, rr.Body.String())
		}
	}

	stubDefaultTransport(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("down") })
	rr = doRequest(server.handlePlacesDetail, "GET", "/api/places/detail?place_id=abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadGateway {
		t.Errorf("down: got %d, want 502", rr.Code)
	}
}

// =============================================================================
// extractReceiptStructure (Anthropic stubbed)
// =============================================================================

func stubAnthropic(t *testing.T, status int, text string) (calls *int, lastBody *string) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	n := 0
	body := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		if status != 200 {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"stub"}`))
			return
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"stop_reason": "end_turn",
			"content":     []map[string]string{{"type": "text", "text": text}},
		})
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	old := anthropicBaseURL
	anthropicBaseURL = srv.URL
	t.Cleanup(func() { anthropicBaseURL = old })
	return &n, &body
}

func TestExtractReceiptStructure_ParsesAndSanitizes(t *testing.T) {
	server := newTestServer(t)
	long := strings.Repeat("x", 600)
	payload := fmt.Sprintf("```json\n"+`{"store_name":"<b>Kroger</b>","store_address":"%s","transaction_date":"2026-03-07","subtotal_cents":2499,"tax_cents":200,"total_cents":2699,`+
		`"line_items":[{"raw_text":"<script>x</script>ORG MLK","price_cents":599,"quantity":1},{"raw_text":"%s","price_cents":349,"quantity":2}]}`+"\n```", long, long)
	calls, body := stubAnthropic(t, 200, payload)

	got := server.extractReceiptStructure([]string{"data:image/png;base64,AAAA", "BBBB"})
	if got == nil {
		t.Fatal("nil result")
	}
	if *calls != 1 {
		t.Errorf("anthropic calls = %d, want 1", *calls)
	}
	if got.StoreName != "Kroger" {
		t.Errorf("store name not stripped: %q", got.StoreName)
	}
	if len(got.StoreAddress) != 500 {
		t.Errorf("address length = %d, want 500", len(got.StoreAddress))
	}
	if got.TotalCents != 2699 || got.SubtotalCents != 2499 || got.TaxCents != 200 || got.TransactionDate != "2026-03-07" {
		t.Errorf("numbers wrong: %+v", got)
	}
	if len(got.LineItems) != 2 || got.LineItems[0].RawText != "xORG MLK" {
		t.Errorf("line items = %+v", got.LineItems)
	}
	if strings.Contains(got.LineItems[0].RawText, "<") {
		t.Errorf("html survived: %q", got.LineItems[0].RawText)
	}
	if len(got.LineItems[1].RawText) != 200 {
		t.Errorf("raw_text length = %d, want 200", len(got.LineItems[1].RawText))
	}

	// Request carried both images with correct media types.
	if !strings.Contains(*body, `"media_type":"image/png"`) || !strings.Contains(*body, `"media_type":"image/jpeg"`) ||
		!strings.Contains(*body, `"data":"AAAA"`) || !strings.Contains(*body, `"data":"BBBB"`) {
		t.Errorf("request body missing images: %.400s", *body)
	}
}

func TestExtractReceiptStructure_FailuresReturnNil(t *testing.T) {
	server := newTestServer(t)

	t.Run("provider error", func(t *testing.T) {
		calls, _ := stubAnthropic(t, 500, "")
		if got := server.extractReceiptStructure([]string{"AAAA"}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
		if *calls == 0 {
			t.Error("provider was never called")
		}
	})
	t.Run("not json", func(t *testing.T) {
		stubAnthropic(t, 200, "Sorry, I cannot read this receipt.")
		if got := server.extractReceiptStructure([]string{"AAAA"}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
	t.Run("no key and no providers", func(t *testing.T) {
		stubAnthropic(t, 200, "{}")
		saved := server.config.AnthropicAPIKey
		server.config.AnthropicAPIKey = ""
		defer func() { server.config.AnthropicAPIKey = saved }()
		if got := server.extractReceiptStructure([]string{"AAAA"}); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
}
