package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// WATT ADMIN ENDPOINT TESTS
//
// The Watt endpoints (handlers_watt.go) return person PII (CPG brand-manager
// contacts) and call the external Watt Data API. These tests pin three things
// for each endpoint: (1) only admins reach them, (2) a missing API key returns
// 503 instead of attempting an outbound call, and (3) empty input is rejected
// with 400 before any outbound call. The Watt Data API is mocked with httptest
// so no test ever touches the network.
//
// wattHTTPClient (handlers_watt.go) has a nil Transport, so it falls back to
// http.DefaultTransport. Overriding DefaultTransport with testTransport (see
// test_helpers_test.go) routes every Watt request to the local mock server.
// =============================================================================

// mockWattServer stands up an httptest server that speaks the Watt Data MCP
// envelope and redirects wattHTTPClient to it. It dispatches on the MCP tool
// name so a single server can back entity_find, entity_enrich, trait_search,
// and entity_resolve. The returned closure restores the original transport.
func mockWattServer(t *testing.T, byTool map[string]string) func() {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req wattMCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		text, ok := byTool[req.Params.Name]
		if !ok {
			http.Error(w, "unexpected tool: "+req.Params.Name, http.StatusBadRequest)
			return
		}
		body, _ := json.Marshal(wattMCPResponse{
			Content: []wattContent{{Type: "text", Text: text}},
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	origTransport := http.DefaultTransport
	http.DefaultTransport = &testTransport{targetURL: ts.URL, real: origTransport}
	return func() {
		http.DefaultTransport = origTransport
		ts.Close()
	}
}

// wattEndpoint pairs a handler with a minimal valid request body for the
// table-driven auth/config tests.
type wattEndpoint struct {
	name    string
	handler func(*Server) http.HandlerFunc
	path    string
	body    map[string]interface{}
}

func wattEndpoints() []wattEndpoint {
	return []wattEndpoint{
		{
			name:    "brand-search",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattBrandSearch },
			path:    "/api/admin/watt/brand-search",
			body:    map[string]interface{}{"expression": "T123 AND T456"},
		},
		{
			name:    "trait-search",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattTraitSearch },
			path:    "/api/admin/watt/trait-search",
			body:    map[string]interface{}{"query": "brand manager"},
		},
		{
			name:    "resolve",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattEntityResolve },
			path:    "/api/admin/watt/resolve",
			body: map[string]interface{}{
				"identifiers": []map[string]string{{"id_type": "email", "id_value": "x@y.com"}},
			},
		},
	}
}

// --- Auth-negative: a non-admin session must never reach a PII endpoint ---

func TestWattEndpoints_NonAdminForbidden(t *testing.T) {
	for _, ep := range wattEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			server := newTestServer(t, &MockTwilioClient{})
			// Key is set so the 403 proves auth runs before the config check.
			server.config.WattAPIKey = "watt_test_key"

			cookie, _ := loginUser(t, server, "5551234567") // not the admin phone
			rr := doRequest(ep.handler(server), "POST", ep.path, ep.body, []*http.Cookie{cookie})

			if rr.Code != http.StatusForbidden {
				t.Errorf("non-admin: got %d, want 403; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// --- Auth-negative: no session at all must return 401, not leak PII ---

func TestWattEndpoints_NoSessionUnauthorized(t *testing.T) {
	for _, ep := range wattEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			server := newTestServer(t, &MockTwilioClient{})
			server.config.WattAPIKey = "watt_test_key"

			rr := doRequest(ep.handler(server), "POST", ep.path, ep.body, nil)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("no session: got %d, want 401; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// --- Missing config: admin + unset Watt key returns 503, no outbound call ---

func TestWattEndpoints_MissingConfig(t *testing.T) {
	for _, ep := range wattEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			server := newTestServer(t, &MockTwilioClient{})
			server.config.WattAPIKey = "" // default, but assert it explicitly

			cookie := loginAdmin(t, server)
			rr := doRequest(ep.handler(server), "POST", ep.path, ep.body, []*http.Cookie{cookie})

			if rr.Code != http.StatusServiceUnavailable {
				t.Errorf("missing config: got %d, want 503; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// --- Validation: admin + key set + empty input returns 400 before any call ---

func TestWattEndpoints_EmptyInputBadRequest(t *testing.T) {
	cases := []struct {
		name    string
		handler func(*Server) http.HandlerFunc
		path    string
		body    map[string]interface{}
	}{
		{
			name:    "brand-search empty expression",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattBrandSearch },
			path:    "/api/admin/watt/brand-search",
			body:    map[string]interface{}{"expression": ""},
		},
		{
			name:    "trait-search blank query",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattTraitSearch },
			path:    "/api/admin/watt/trait-search",
			body:    map[string]interface{}{"query": "   "},
		},
		{
			name:    "resolve no identifiers",
			handler: func(s *Server) http.HandlerFunc { return s.handleWattEntityResolve },
			path:    "/api/admin/watt/resolve",
			body:    map[string]interface{}{"identifiers": []map[string]string{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t, &MockTwilioClient{})
			server.config.WattAPIKey = "watt_test_key"

			cookie := loginAdmin(t, server)
			rr := doRequest(tc.handler(server), "POST", tc.path, tc.body, []*http.Cookie{cookie})

			if rr.Code != http.StatusBadRequest {
				t.Errorf("empty input: got %d, want 400; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// --- Method guard: a non-POST request is rejected before auth ---

func TestWattEndpoints_WrongMethod(t *testing.T) {
	for _, ep := range wattEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			server := newTestServer(t, &MockTwilioClient{})
			server.config.WattAPIKey = "watt_test_key"

			rr := doRequest(ep.handler(server), "GET", ep.path, nil, nil)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET: got %d, want 405; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// --- Happy paths: prove the mocked outbound flow returns 200 for an admin ---

func TestWattTraitSearch_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WattAPIKey = "watt_test_key"

	restore := mockWattServer(t, map[string]string{
		"trait_search": `{"traits":[{"trait_id":"T123","name":"Brand Manager"}]}`,
	})
	defer restore()

	cookie := loginAdmin(t, server)
	rr := doRequest(server.handleWattTraitSearch, "POST", "/api/admin/watt/trait-search",
		map[string]interface{}{"query": "brand manager"}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("trait-search: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "T123") {
		t.Errorf("trait-search body missing trait id: %s", rr.Body.String())
	}
}

func TestWattEntityResolve_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WattAPIKey = "watt_test_key"

	restore := mockWattServer(t, map[string]string{
		"entity_resolve": `{"matches":[{"entity_id":"E1"}]}`,
	})
	defer restore()

	cookie := loginAdmin(t, server)
	rr := doRequest(server.handleWattEntityResolve, "POST", "/api/admin/watt/resolve",
		map[string]interface{}{
			"identifiers": []map[string]string{{"id_type": "email", "id_value": "x@y.com"}},
		}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("resolve: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "E1") {
		t.Errorf("resolve body missing entity id: %s", rr.Body.String())
	}
}

func TestWattBrandSearch_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WattAPIKey = "watt_test_key"

	restore := mockWattServer(t, map[string]string{
		"entity_find":   `{"total":2,"returned_count":1,"has_more":false,"workflow_id":"wf1","sample":[{"entity_id":"E1","email":"x@y.com"}]}`,
		"entity_enrich": `{"profiles":[{"entity_id":"E1","name":"Jane"}]}`,
	})
	defer restore()

	cookie := loginAdmin(t, server)
	rr := doRequest(server.handleWattBrandSearch, "POST", "/api/admin/watt/brand-search",
		map[string]interface{}{"expression": "T123 AND T456"}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("brand-search: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if got, ok := body["returned"].(float64); !ok || got != 1 {
		t.Errorf("brand-search returned: got %v, want 1; body=%s", body["returned"], rr.Body.String())
	}
	if _, ok := body["profiles"]; !ok {
		t.Errorf("brand-search missing profiles: %s", rr.Body.String())
	}
}

// --- Empty-result path: entity_find returns nothing, no enrich call needed ---

func TestWattBrandSearch_NoMatches(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.WattAPIKey = "watt_test_key"

	restore := mockWattServer(t, map[string]string{
		"entity_find": `{"total":0,"returned_count":0,"has_more":false,"sample":[]}`,
	})
	defer restore()

	cookie := loginAdmin(t, server)
	rr := doRequest(server.handleWattBrandSearch, "POST", "/api/admin/watt/brand-search",
		map[string]interface{}{"expression": "T999"}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("brand-search no matches: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if got, ok := body["total"].(float64); !ok || got != 0 {
		t.Errorf("brand-search no matches total: got %v, want 0; body=%s", body["total"], rr.Body.String())
	}
}
