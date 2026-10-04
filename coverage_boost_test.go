package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// NEW TESTS: handleAdminDeals — uncovered branches
// =============================================================================

func TestNewTests_AdminDeals_PostInvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833") // admin phone

	// Send invalid JSON body
	req := httptest.NewRequest("POST", "/api/admin/deals", bytes.NewReader([]byte("{bad json")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAdminDeals(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON should return 400, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid request body" {
		t.Errorf("Expected 'Invalid request body' error, got %v", body["error"])
	}
}

func TestNewTests_AdminDeals_PostMissingRequiredFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Missing competitor_product
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"offer_brand":   "Brand B",
			"offer_product": "Product B",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Missing competitor_product: got %d, want 400", rr.Code)
	}

	// Missing offer_brand
	rr = doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "Brand A Milk",
			"offer_product":     "Product B",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Missing offer_brand: got %d, want 400", rr.Code)
	}

	// Missing offer_product
	rr = doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "Brand A Milk",
			"offer_brand":       "Brand B",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Missing offer_product: got %d, want 400", rr.Code)
	}

	// Whitespace-only fields should also fail
	rr = doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "   ",
			"offer_brand":       "  ",
			"offer_product":     "  ",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Whitespace-only fields: got %d, want 400", rr.Code)
	}
}

func TestNewTests_AdminDeals_PostDefaultValues(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create deal without status, deal_type, or quantity_offered
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A Milk",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Milk",
			"discount_cents":     100,
			"daily_budget_cents": 5000,
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create deal: got %d, want 201: %s", rr.Code, rr.Body.String())
	}

	var deal BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &deal)

	if deal.Status != "active" {
		t.Errorf("Default status: got %q, want 'active'", deal.Status)
	}
	if deal.DealType != "sku_swap" {
		t.Errorf("Default deal_type: got %q, want 'sku_swap'", deal.DealType)
	}
	if deal.QuantityOffered != 1 {
		t.Errorf("Default quantity_offered: got %d, want 1", deal.QuantityOffered)
	}
}

func TestNewTests_AdminDeals_PostCustomDealType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create deal with explicit deal_type, status, and quantity_offered
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A Cola",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Cola",
			"discount_cents":     50,
			"daily_budget_cents": 2000,
			"status":             "paused",
			"deal_type":          "free_trial",
			"quantity_offered":   3,
			"trial_product_name": "Brand B Trial Pack",
			"trial_size":         "6oz",
			"campaign_tag":       "summer_2026",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create deal: got %d, want 201: %s", rr.Code, rr.Body.String())
	}

	var deal BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &deal)

	if deal.Status != "paused" {
		t.Errorf("Status: got %q, want 'paused'", deal.Status)
	}
	if deal.DealType != "free_trial" {
		t.Errorf("DealType: got %q, want 'free_trial'", deal.DealType)
	}
	if deal.QuantityOffered != 3 {
		t.Errorf("QuantityOffered: got %d, want 3", deal.QuantityOffered)
	}
	if deal.TrialProductName != "Brand B Trial Pack" {
		t.Errorf("TrialProductName: got %q, want 'Brand B Trial Pack'", deal.TrialProductName)
	}
}

func TestNewTests_AdminDeals_PutInvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	req := httptest.NewRequest("PUT", "/api/admin/deals", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAdminDeals(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON PUT: got %d, want 400", rr.Code)
	}
}

func TestNewTests_AdminDeals_PutMissingID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	rr := doRequest(server.handleAdminDeals, "PUT", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "Brand A",
			"offer_brand":       "Brand B",
			"offer_product":     "Brand B Milk",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PUT without ID: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "id is required" {
		t.Errorf("Expected 'id is required', got %v", body["error"])
	}
}

func TestNewTests_AdminDeals_DeleteInvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	req := httptest.NewRequest("DELETE", "/api/admin/deals", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAdminDeals(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON DELETE: got %d, want 400", rr.Code)
	}
}

func TestNewTests_AdminDeals_DeleteMissingID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	rr := doRequest(server.handleAdminDeals, "DELETE", "/api/admin/deals",
		map[string]interface{}{"id": 0}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DELETE with id=0: got %d, want 400", rr.Code)
	}
}

func TestNewTests_AdminDeals_DeleteNonexistentDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	rr := doRequest(server.handleAdminDeals, "DELETE", "/api/admin/deals",
		map[string]interface{}{"id": 99999}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("DELETE nonexistent deal: got %d, want 404", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Deal not found" {
		t.Errorf("Expected 'Deal not found', got %v", body["error"])
	}
}

func TestNewTests_AdminDeals_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	rr := doRequest(server.handleAdminDeals, "PATCH", "/api/admin/deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PATCH should return 405, got %d", rr.Code)
	}
}

func TestNewTests_AdminDeals_PostNegativePrices(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Negative offer_price_cents
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Milk",
			"offer_price_cents":  -100,
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Negative offer_price_cents: got %d, want 400", rr.Code)
	}

	// Negative competitor_price_cents
	rr = doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":      "Brand A",
			"offer_brand":            "Brand B",
			"offer_product":          "Brand B Milk",
			"competitor_price_cents": -200,
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Negative competitor_price_cents: got %d, want 400", rr.Code)
	}
}

// =============================================================================
// NEW TESTS: handleResolveABTest — uncovered branches
// =============================================================================

func TestNewTests_ABResolveTest_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create a test to resolve
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('resolve_me', 'Test to resolve', '["control","variant_a"]', 'conversion', 100, 7, 100, 'active')`)

	// Use ServeMux so r.PathValue("name") works
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/ab-tests/{name}/resolve", handleResolveABTest(server))

	body, _ := json.Marshal(map[string]string{"winner": "variant_a"})
	req := httptest.NewRequest("POST", "/api/admin/ab-tests/resolve_me/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Resolve: got %d, want 200: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	if result["status"] != "resolved" {
		t.Errorf("Status: got %v, want 'resolved'", result["status"])
	}
	if result["winner"] != "variant_a" {
		t.Errorf("Winner: got %v, want 'variant_a'", result["winner"])
	}

	// Verify the test is now resolved in DB
	var dbStatus, dbWinner string
	server.db.QueryRow("SELECT status, COALESCE(winner,'') FROM ab_tests WHERE name = 'resolve_me'").Scan(&dbStatus, &dbWinner)
	if dbStatus != "resolved" {
		t.Errorf("DB status: got %q, want 'resolved'", dbStatus)
	}
	if dbWinner != "variant_a" {
		t.Errorf("DB winner: got %q, want 'variant_a'", dbWinner)
	}
}

func TestNewTests_ABResolveTest_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/ab-tests/{name}/resolve", handleResolveABTest(server))

	req := httptest.NewRequest("POST", "/api/admin/ab-tests/test_name/resolve", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON resolve: got %d, want 400", rr.Code)
	}
}

func TestNewTests_ABResolveTest_NonAdmin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999001") // non-admin

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/ab-tests/{name}/resolve", handleResolveABTest(server))

	body, _ := json.Marshal(map[string]string{"winner": "control"})
	req := httptest.NewRequest("POST", "/api/admin/ab-tests/test_name/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Non-admin resolve: got %d, want 403", rr.Code)
	}
}

// =============================================================================
// NEW TESTS: handleAdminPayouts — uncovered branches
// =============================================================================

func TestNewTests_AdminPayouts_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	rr := doRequest(server.handleAdminPayouts, "DELETE", "/api/admin/payouts", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE should return 405, got %d", rr.Code)
	}

	rr = doRequest(server.handleAdminPayouts, "PUT", "/api/admin/payouts", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT should return 405, got %d", rr.Code)
	}
}

func TestNewTests_AdminPayouts_PostNoNeedingRebates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// POST for a user with no pending rebates — should succeed with 0 updated
	rr := doRequest(server.handleAdminPayouts, "POST", "/api/admin/payouts",
		map[string]interface{}{"user_id": "user-99999"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST for non-existing user: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["updated"].(float64) != 0 {
		t.Errorf("Updated should be 0 for user with no pending rebates, got %v", body["updated"])
	}
}

func TestNewTests_AdminPayouts_MultiplePendingRebates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	adminUser, _ := upsertUser(server.db, "3104861833")
	adminToken, _ := server.sessions.CreateSession(adminUser.ID, "3104861833", "phone")
	adminCookie := &http.Cookie{Name: "session", Value: adminToken}

	// Create a user with multiple pending rebates
	regularUser, _ := upsertUser(server.db, "5559990099")
	server.db.Exec("UPDATE users SET name = 'Multi Rebate User', email = 'multi@test.com' WHERE id = ?", regularUser.ID)

	// Create 3 swap events and rebates, one campaign each: a user gets a single
	// accept per campaign (idx_swap_events_one_accept), so three pending rebates
	// for one user means three different campaigns.
	for i := 0; i < 3; i++ {
		dealRes, err := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
			VALUES ('X', 'Y', 'Z', 100, 5000, 'active')`)
		if err != nil {
			t.Fatalf("inserting deal %d: %v", i, err)
		}
		dealID, _ := dealRes.LastInsertId()

		swapRes, err := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'accepted')`, regularUser.ID, dealID, i+1)
		if err != nil {
			t.Fatalf("inserting swap event %d: %v", i, err)
		}
		swapEventID, _ := swapRes.LastInsertId()
		server.db.Exec(`INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, ?, 'pending')`,
			regularUser.ID, swapEventID, dealID, 50*(i+1))
	}

	// GET to verify aggregation
	rr := doRequest(server.handleAdminPayouts, "GET", "/api/admin/payouts", nil, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET payouts: got %d, want 200", rr.Code)
	}

	var payouts []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &payouts)
	if len(payouts) != 1 {
		t.Fatalf("Expected 1 payout entry, got %d", len(payouts))
	}
	// Total should be 50+100+150 = 300
	if payouts[0]["total_cents"].(float64) != 300 {
		t.Errorf("Total cents: got %v, want 300", payouts[0]["total_cents"])
	}
	if payouts[0]["swap_count"].(float64) != 3 {
		t.Errorf("Swap count: got %v, want 3", payouts[0]["swap_count"])
	}

	// POST to mark all as issued
	rr = doRequest(server.handleAdminPayouts, "POST", "/api/admin/payouts",
		map[string]interface{}{"user_id": regularUser.ID}, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST payouts: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["updated"].(float64) != 3 {
		t.Errorf("Should mark 3 rebates as issued, got %v", body["updated"])
	}

	// GET again should show 0 pending
	rr = doRequest(server.handleAdminPayouts, "GET", "/api/admin/payouts", nil, []*http.Cookie{adminCookie})
	var afterPayouts []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &afterPayouts)
	if len(afterPayouts) != 0 {
		t.Errorf("After marking issued, expected 0 payouts, got %d", len(afterPayouts))
	}
}

func TestNewTests_AdminPayouts_PostInvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Send request with wrong content type (decodeJSON checks Content-Type)
	req := httptest.NewRequest("POST", "/api/admin/payouts", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAdminPayouts(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Wrong content-type: got %d, want 415", rr.Code)
	}
}

func TestNewTests_AdminPayouts_MultipleUsersAggregation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	adminUser, _ := upsertUser(server.db, "3104861833")
	adminToken, _ := server.sessions.CreateSession(adminUser.ID, "3104861833", "phone")
	adminCookie := &http.Cookie{Name: "session", Value: adminToken}

	// Create two users with pending rebates
	user1, _ := upsertUser(server.db, "5559991111")
	user2, _ := upsertUser(server.db, "5559992222")
	server.db.Exec("UPDATE users SET name = 'User One' WHERE id = ?", user1.ID)
	server.db.Exec("UPDATE users SET name = 'User Two' WHERE id = ?", user2.ID)

	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('A', 'B', 'C', 100, 5000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	// User 1: 200 cents pending
	swapRes1, _ := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, user1.ID, dealID)
	swapEventID1, _ := swapRes1.LastInsertId()
	server.db.Exec(`INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, 200, 'pending')`, user1.ID, swapEventID1, dealID)

	// User 2: 100 cents pending
	swapRes2, _ := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 2, 'accepted')`, user2.ID, dealID)
	swapEventID2, _ := swapRes2.LastInsertId()
	server.db.Exec(`INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES (?, ?, ?, 100, 'pending')`, user2.ID, swapEventID2, dealID)

	rr := doRequest(server.handleAdminPayouts, "GET", "/api/admin/payouts", nil, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET payouts: got %d, want 200", rr.Code)
	}

	var payouts []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &payouts)
	if len(payouts) != 2 {
		t.Fatalf("Expected 2 payout entries (two users), got %d", len(payouts))
	}

	// They should be ordered by total_cents DESC (user1=200 first, user2=100 second)
	if payouts[0]["total_cents"].(float64) != 200 {
		t.Errorf("First payout total_cents: got %v, want 200", payouts[0]["total_cents"])
	}
	if payouts[1]["total_cents"].(float64) != 100 {
		t.Errorf("Second payout total_cents: got %v, want 100", payouts[1]["total_cents"])
	}
}

// =============================================================================
// NEW TESTS: handleABTestResults — uncovered branches
// =============================================================================

func TestNewTests_ABResults_MissingTestName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Call handler directly without path routing — PathValue("name") returns ""
	rr := doRequest(handleABTestResults(server), "GET", "/api/admin/ab-tests//results", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Empty test name: got %d, want 400", rr.Code)
	}
}

func TestNewTests_ABResults_NonexistentTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/admin/ab-tests/{name}/results", handleABTestResults(server))

	req := httptest.NewRequest("GET", "/api/admin/ab-tests/totally_bogus_test/results", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Nonexistent test: got %d, want 500", rr.Code)
	}
}

func TestNewTests_ABResults_NonAdmin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999002") // non-admin

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/admin/ab-tests/{name}/results", handleABTestResults(server))

	req := httptest.NewRequest("GET", "/api/admin/ab-tests/some_test/results", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Non-admin results: got %d, want 403", rr.Code)
	}
}

// =============================================================================
// NEW TESTS: trackABEvent — uncovered branches
// =============================================================================

func TestNewTests_TrackABEvent_NonexistentTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Track event for a test that doesn't exist — should silently succeed (no error)
	err := trackABEvent(server.db, "nonexistent_test", "user-1", "conversion", 1.0)
	if err != nil {
		t.Errorf("Tracking event for nonexistent test should not error, got: %v", err)
	}

	// Verify no event was inserted
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM ab_events WHERE test_name = 'nonexistent_test'").Scan(&count)
	if count != 0 {
		t.Errorf("No event should be inserted for nonexistent test, got %d", count)
	}
}

func TestNewTests_TrackABEvent_InactiveTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a resolved (inactive) test
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('inactive_test', 'desc', '["control","variant"]', 'conversion', 100, 7, 100, 'resolved')`)

	err := trackABEvent(server.db, "inactive_test", "user-1", "conversion", 1.0)
	if err != nil {
		t.Errorf("Tracking event for inactive test should not error, got: %v", err)
	}

	// Verify no event was inserted
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM ab_events WHERE test_name = 'inactive_test'").Scan(&count)
	if count != 0 {
		t.Errorf("No event should be inserted for inactive test, got %d", count)
	}
}

func TestNewTests_TrackABEvent_SuccessfulInsert(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create an active test with 100% traffic
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('active_track', 'desc', '["control","variant"]', 'click', 100, 7, 100, 'active')`)

	err := trackABEvent(server.db, "active_track", "user-42", "click", 2.5)
	if err != nil {
		t.Fatalf("Tracking event: %v", err)
	}

	// Verify event was inserted
	var count int
	var value float64
	server.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(value),0) FROM ab_events WHERE test_name = 'active_track' AND user_id = 'user-42'").Scan(&count, &value)
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
	if value != 2.5 {
		t.Errorf("Expected value 2.5, got %f", value)
	}
}

func TestNewTests_TrackABEvent_MultipleEvents(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('multi_track', 'desc', '["control","variant"]', 'click', 100, 7, 100, 'active')`)

	// Track multiple events for same user — should all be inserted
	for i := 0; i < 5; i++ {
		err := trackABEvent(server.db, "multi_track", "user-10", "click", 1.0)
		if err != nil {
			t.Fatalf("Track event %d: %v", i, err)
		}
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM ab_events WHERE test_name = 'multi_track' AND user_id = 'user-10'").Scan(&count)
	if count != 5 {
		t.Errorf("Expected 5 events, got %d", count)
	}
}

// =============================================================================
// NEW TESTS: handleGetVariant — uncovered branches
// =============================================================================

func TestNewTests_GetVariant_NonexistentTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999003")

	rr := doRequest(handleGetVariant(server), "GET", "/api/ab/variant?test=does_not_exist", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Nonexistent test: got %d, want 200", rr.Code)
	}
	result := parseJSON(rr)
	if result["variant"] != "" {
		t.Errorf("Expected empty variant for nonexistent test, got %v", result["variant"])
	}
}

func TestNewTests_GetVariant_InactiveTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999004")

	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('inactive_var', 'desc', '["control","variant"]', 'conversion', 100, 7, 100, 'resolved')`)

	rr := doRequest(handleGetVariant(server), "GET", "/api/ab/variant?test=inactive_var", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Inactive test: got %d, want 200", rr.Code)
	}
	result := parseJSON(rr)
	if result["variant"] != "" {
		t.Errorf("Expected empty variant for inactive test, got %v", result["variant"])
	}
}

func TestNewTests_GetVariant_TrafficPercentage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create test with only 10% traffic — users with bucket >= 10 get control
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('low_traffic', 'desc', '["control","variant"]', 'metric', 100, 7, 10, 'active')`)

	// User with ID where ID % 100 >= 10 should get control (outside traffic %)
	// Create user — loginUser creates with upsertUser which auto-assigns IDs
	// We need a user whose ID % 100 >= 10
	// Insert directly to control the ID
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-50', '5559999050')")
	token, _ := server.sessions.CreateSession("user-50", "5559999050", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(handleGetVariant(server), "GET", "/api/ab/variant?test=low_traffic", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Get variant: got %d, want 200", rr.Code)
	}
	result := parseJSON(rr)
	// User ID 50: bucket = 50 % 100 = 50, which is >= 10, so should get control
	if result["variant"] != "control" {
		t.Errorf("User outside traffic %% should get control, got %v", result["variant"])
	}
}

func TestNewTests_GetVariant_ThreeVariants(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create test with 3 variants and 100% traffic
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('three_vars', 'desc', '["a","b","c"]', 'metric', 100, 7, 100, 'active')`)

	// Test deterministic assignment across different user IDs
	variants := map[string]bool{}
	for _, uid := range []string{"user-1", "user-100", "user-200", "user-300", "user-400", "user-500"} {
		v, err := getVariant(server.db, "three_vars", uid)
		if err != nil {
			t.Fatalf("getVariant uid=%s: %v", uid, err)
		}
		if v != "a" && v != "b" && v != "c" {
			t.Errorf("Unexpected variant %q for uid %s", v, uid)
		}
		variants[v] = true
	}
	// With 6 different user IDs we should see at least 2 of the 3 variants
	if len(variants) < 2 {
		t.Errorf("Expected at least 2 distinct variants across 6 users, got %d: %v", len(variants), variants)
	}
}

// =============================================================================
// NEW TESTS: getVariant edge cases
// =============================================================================

func TestNewTests_GetVariant_FunctionLevel_FewerThanTwoVariants(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a test with only 1 variant (invalid but tests the error path)
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('one_var', 'desc', '["only_one"]', 'metric', 100, 7, 100, 'active')`)

	_, err := getVariant(server.db, "one_var", "user-1")
	if err == nil {
		t.Error("Expected error for test with fewer than 2 variants")
	}
	if !strings.Contains(err.Error(), "fewer than 2 variants") {
		t.Errorf("Error should mention fewer than 2 variants, got: %v", err)
	}
}

func TestNewTests_GetVariant_FunctionLevel_InvalidVariantsJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a test with invalid JSON in variants column
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('bad_json_var', 'desc', '"not-json"', 'metric', 100, 7, 100, 'active')`)

	_, err := getVariant(server.db, "bad_json_var", "user-1")
	if err == nil {
		t.Error("Expected error for invalid variants JSON")
	}
}

// =============================================================================
// NEW TESTS: analyzeABTest — additional branch coverage
// =============================================================================

func TestNewTests_AnalyzeABTest_BothZeroEvents(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	server.db.Exec(
		`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now', '-10 days'))`,
		"both_zero", "Both zero", `["control","variant_a"]`, "purchases", 2, 7, 100, "active",
	)

	// Both variants have events with value=0 (mean=0 for both)
	for i := 1; i <= 3; i++ {
		server.db.Exec(
			"INSERT INTO ab_events (test_name, variant, user_id, event, value) VALUES (?, ?, ?, ?, ?)",
			"both_zero", "control", fmt.Sprint(i), "purchases", 0.0,
		)
	}
	for i := 4; i <= 6; i++ {
		server.db.Exec(
			"INSERT INTO ab_events (test_name, variant, user_id, event, value) VALUES (?, ?, ?, ?, ?)",
			"both_zero", "variant_a", fmt.Sprint(i), "purchases", 0.0,
		)
	}

	analysis, err := analyzeABTest(server.db, "both_zero")
	if err != nil {
		t.Fatalf("analyzeABTest error: %v", err)
	}
	if !analysis.SufficientData {
		t.Error("Should have sufficient data")
	}
	// control.Mean == 0, bestMean == 0: should hit "No events recorded" path
	if !strings.Contains(analysis.Recommendation, "No events recorded") {
		t.Errorf("Should recommend checking instrumentation, got %q", analysis.Recommendation)
	}
}

func TestNewTests_AnalyzeABTest_ResolvedStatus(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a resolved test
	server.db.Exec(
		`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status, winner, resolved_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '-20 days'))`,
		"resolved_test", "Resolved", `["control","variant_a"]`, "metric", 2, 7, 100, "resolved", "variant_a",
	)

	analysis, err := analyzeABTest(server.db, "resolved_test")
	if err != nil {
		t.Fatalf("analyzeABTest error: %v", err)
	}
	if analysis.Status != "resolved" {
		t.Errorf("Status: got %q, want 'resolved'", analysis.Status)
	}
}

// =============================================================================
// NEW TESTS: handleTrackABEvent handler — uncovered branches
// =============================================================================

func TestNewTests_TrackABEventHandler_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999010")

	req := httptest.NewRequest("POST", "/api/ab/event", bytes.NewReader([]byte("{bad")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handleTrackABEvent(server)(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "invalid JSON" {
		t.Errorf("Error: got %v, want 'invalid JSON'", body["error"])
	}
}

func TestNewTests_TrackABEventHandler_MissingTest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999011")

	rr := doRequest(handleTrackABEvent(server), "POST", "/api/ab/event",
		map[string]interface{}{"test": "", "event": "click"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Missing test: got %d, want 400", rr.Code)
	}
}

func TestNewTests_TrackABEventHandler_MissingEvent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559999012")

	rr := doRequest(handleTrackABEvent(server), "POST", "/api/ab/event",
		map[string]interface{}{"test": "t", "event": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Missing event: got %d, want 400", rr.Code)
	}
}

func TestNewTests_TrackABEventHandler_ActiveTestRecords(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559999013")

	// Create an active test
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('handler_track', 'desc', '["control","variant"]', 'click', 100, 7, 100, 'active')`)

	rr := doRequest(handleTrackABEvent(server), "POST", "/api/ab/event",
		map[string]interface{}{"test": "handler_track", "event": "click", "value": 3.5}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Track event: got %d, want 200", rr.Code)
	}

	// Verify event was recorded with correct value
	var value float64
	server.db.QueryRow("SELECT value FROM ab_events WHERE test_name = 'handler_track' AND user_id = ?", userID).Scan(&value)
	if value != 3.5 {
		t.Errorf("Event value: got %f, want 3.5", value)
	}
}

// =============================================================================
// NEW TESTS: handleCreateABTest — additional branches
// =============================================================================

func TestNewTests_ABCreateTest_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	req := httptest.NewRequest("POST", "/api/admin/ab-tests", bytes.NewReader([]byte("{bad json")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handleCreateABTest(server)(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON: got %d, want 400", rr.Code)
	}
}

func TestNewTests_ABCreateTest_DuplicateName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create first test
	rr := doRequest(handleCreateABTest(server), "POST", "/api/admin/ab-tests",
		map[string]interface{}{"name": "dupe_test", "key_metric": "clicks"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("First create: got %d, want 201", rr.Code)
	}

	// Try to create same name again
	rr = doRequest(handleCreateABTest(server), "POST", "/api/admin/ab-tests",
		map[string]interface{}{"name": "dupe_test", "key_metric": "clicks"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Duplicate name: got %d, want 500", rr.Code)
	}
}

func TestNewTests_ABCreateTest_DefaultsApplied(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create with only required fields — other defaults should apply
	rr := doRequest(handleCreateABTest(server), "POST", "/api/admin/ab-tests",
		map[string]interface{}{
			"name":       "defaults_test",
			"key_metric": "metric",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create: got %d, want 201: %s", rr.Code, rr.Body.String())
	}

	// Verify defaults in DB
	var minSample, days, pct int
	var variantsJSON string
	server.db.QueryRow("SELECT variants, min_sample_per_variant, measurement_days, traffic_pct FROM ab_tests WHERE name = 'defaults_test'").
		Scan(&variantsJSON, &minSample, &days, &pct)

	// JSONB re-serialises with spaces, so compare the decoded value.
	var gotVariants []string
	if err := json.Unmarshal([]byte(variantsJSON), &gotVariants); err != nil || len(gotVariants) != 2 || gotVariants[0] != "control" || gotVariants[1] != "variant" {
		t.Errorf("Default variants: got %q, want [\"control\",\"variant\"]", variantsJSON)
	}
	if minSample != 100 {
		t.Errorf("Default min_sample: got %d, want 100", minSample)
	}
	if days != 7 {
		t.Errorf("Default measurement_days: got %d, want 7", days)
	}
	if pct != 50 {
		t.Errorf("Default traffic_pct: got %d, want 50", pct)
	}
}

// =============================================================================
// NEW TESTS: handleListABTests — additional branches
// =============================================================================

func TestNewTests_ABListTests_FilterByStatus(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create tests with different statuses
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status)
		VALUES ('active1', 'desc', '["a","b"]', 'metric', 100, 7, 50, 'active')`)
	server.db.Exec(`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct, status, winner)
		VALUES ('resolved1', 'desc', '["a","b"]', 'metric', 100, 7, 50, 'resolved', 'a')`)

	// Filter for active only
	rr := doRequest(handleListABTests(server), "GET", "/api/admin/ab-tests?status=active", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("List active: got %d, want 200", rr.Code)
	}

	var tests []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &tests)
	// Should include seeded active tests + active1 but not resolved1
	for _, test := range tests {
		if test["status"] != "active" {
			t.Errorf("Expected only active tests, got status %v for test %v", test["status"], test["name"])
		}
	}

	// Filter for resolved only
	rr = doRequest(handleListABTests(server), "GET", "/api/admin/ab-tests?status=resolved", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("List resolved: got %d, want 200", rr.Code)
	}

	json.Unmarshal(rr.Body.Bytes(), &tests)
	for _, test := range tests {
		if test["status"] != "resolved" {
			t.Errorf("Expected only resolved tests, got status %v for test %v", test["status"], test["name"])
		}
	}
}

func TestNewTests_ABListTests_EmptyResult(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Filter for a status that won't match any test
	rr := doRequest(handleListABTests(server), "GET", "/api/admin/ab-tests?status=nonexistent_status", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("List empty: got %d, want 200", rr.Code)
	}

	var tests []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &tests)
	if len(tests) != 0 {
		t.Errorf("Expected 0 tests for nonexistent status, got %d", len(tests))
	}
}

// =============================================================================
// NEW TESTS: AdminDeals — GET with no deals returns empty array
// =============================================================================

func TestNewTests_AdminDeals_GetEmptyAfterDeletingAll(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Soft-delete all seeded deals
	server.db.Exec("UPDATE brand_deals SET status = 'deleted'")

	rr := doRequest(server.handleAdminDeals, "GET", "/api/admin/deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET deals: got %d, want 200", rr.Code)
	}

	var deals []BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 0 {
		t.Errorf("Expected empty deals array after deleting all, got %d", len(deals))
	}
}

func TestNewTests_AdminDeals_PostWithEndsAt(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create deal with ends_at set
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A Milk",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Milk",
			"discount_cents":     100,
			"daily_budget_cents": 5000,
			"ends_at":            "2026-12-31",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create deal with ends_at: got %d, want 201: %s", rr.Code, rr.Body.String())
	}

	var deal BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &deal)

	// Verify ends_at was stored
	var endsAt string
	server.db.QueryRow("SELECT COALESCE(ends_at,'') FROM brand_deals WHERE id = ?", deal.ID).Scan(&endsAt)
	if endsAt != "2026-12-31" {
		t.Errorf("ends_at: got %q, want '2026-12-31'", endsAt)
	}
}

func TestNewTests_AdminDeals_PostWithEmptyEndsAt(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create deal with empty ends_at — should be stored as NULL (via NULLIF)
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A Bread",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Bread",
			"discount_cents":     75,
			"daily_budget_cents": 3000,
			"ends_at":            "",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create deal with empty ends_at: got %d, want 201", rr.Code)
	}

	var deal BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &deal)

	var endsAt *string
	server.db.QueryRow("SELECT ends_at FROM brand_deals WHERE id = ?", deal.ID).Scan(&endsAt)
	if endsAt != nil {
		t.Errorf("ends_at should be NULL for empty string, got %v", *endsAt)
	}
}

func TestNewTests_AdminDeals_PutUpdatesAllFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "3104861833")

	// Create a deal first
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product":  "Brand A Milk",
			"offer_brand":        "Brand B",
			"offer_product":      "Brand B Milk",
			"discount_cents":     100,
			"daily_budget_cents": 5000,
		}, []*http.Cookie{cookie})
	var created BrandDeal
	json.Unmarshal(rr.Body.Bytes(), &created)

	// Update all fields
	rr = doRequest(server.handleAdminDeals, "PUT", "/api/admin/deals",
		map[string]interface{}{
			"id":                      created.ID,
			"competitor_product":      "Updated Competitor",
			"offer_brand":             "Updated Brand",
			"offer_product":           "Updated Product",
			"discount_cents":          300,
			"daily_budget_cents":      8000,
			"status":                  "paused",
			"ends_at":                 "2027-01-01",
			"offer_price_cents":       499,
			"competitor_price_cents":  699,
			"offer_size":              "32oz",
			"competitor_size":         "28oz",
			"offer_unit_price":        "$0.016/oz",
			"competitor_unit_price":   "$0.025/oz",
			"unit_label":              "per oz",
			"why_bullets":             "Better value;Larger size",
			"allergen_notes":          "Contains dairy",
			"deal_type":               "buy_more",
			"quantity_offered":        2,
			"trial_product_name":      "Trial Pack",
			"trial_size":              "8oz",
			"campaign_tag":            "winter_promo",
		}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT all fields: got %d, want 200: %s", rr.Code, rr.Body.String())
	}

	// Verify all fields in DB
	var d BrandDeal
	server.db.QueryRow(`SELECT competitor_product, offer_brand, offer_product, discount_cents,
		total_budget_cents, status, COALESCE(ends_at,''),
		offer_price_cents, competitor_price_cents, offer_size, competitor_size,
		unit_label, why_bullets, allergen_notes, deal_type, quantity_offered, campaign_tag
		FROM brand_deals WHERE id = ?`, created.ID).
		Scan(&d.CompetitorProduct, &d.OfferBrand, &d.OfferProduct, &d.DiscountCents,
			&d.TotalBudgetCents, &d.Status, &d.EndsAt,
			&d.OfferPriceCents, &d.CompetitorPriceCents, &d.OfferSize, &d.CompetitorSize,
			&d.UnitLabel, &d.WhyBullets, &d.AllergenNotes, &d.DealType, &d.QuantityOffered, &d.CampaignTag)

	if d.CompetitorProduct != "Updated Competitor" {
		t.Errorf("CompetitorProduct: got %q, want 'Updated Competitor'", d.CompetitorProduct)
	}
	if d.Status != "paused" {
		t.Errorf("Status: got %q, want 'paused'", d.Status)
	}
	if d.DealType != "buy_more" {
		t.Errorf("DealType: got %q, want 'buy_more'", d.DealType)
	}
	if d.CampaignTag != "winter_promo" {
		t.Errorf("CampaignTag: got %q, want 'winter_promo'", d.CampaignTag)
	}
}

// =============================================================================
// TestCov_ — Coverage boost tests for auth handlers
// =============================================================================

// --- handleAPIConfig ---

func TestCov_APIConfig_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleAPIConfig, "POST", "/api/config", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestCov_APIConfig_ReturnsGoogleClientID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = "test-client-id-12345.apps.googleusercontent.com"

	rr := doRequest(server.handleAPIConfig, "GET", "/api/config", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["google_client_id"] != "test-client-id-12345.apps.googleusercontent.com" {
		t.Errorf("google_client_id: got %v, want test-client-id-12345.apps.googleusercontent.com", body["google_client_id"])
	}
}

func TestCov_APIConfig_EmptyGoogleClientID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.GoogleClientID = ""

	rr := doRequest(server.handleAPIConfig, "GET", "/api/config", nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["google_client_id"] != "" {
		t.Errorf("google_client_id: got %v, want empty string", body["google_client_id"])
	}
}

// --- handleSendCode rate limiting ---

func TestCov_SendCode_PerPhoneRateLimit(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	// Disable test mode so rate limiting is active
	server.config.TestMode = false

	// First request should succeed
	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5551111111", "sms_opt_in": true}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("First request: got %d, want 200", rr.Code)
	}

	// Second request within 60s to the same phone should be rate limited
	rr = doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5551111111", "sms_opt_in": true}, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: got %d, want 429", rr.Code)
	}
	body := parseJSON(rr)
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "60 seconds") {
		t.Errorf("Error should mention 60 seconds, got %q", errMsg)
	}
}

func TestCov_SendCode_PerIPRateLimit(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.config.TestMode = false

	// Send 5 requests (the max per IP per 10 minutes), each to a different phone
	for i := 0; i < 5; i++ {
		phone := fmt.Sprintf("555200%04d", i)
		rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
			map[string]interface{}{"phone_number": phone, "sms_opt_in": true}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("Request %d: got %d, want 200", i+1, rr.Code)
		}
	}

	// 6th request from the same IP should be rate limited
	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5553000000", "sms_opt_in": true}, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: got %d, want 429", rr.Code)
	}
}

func TestCov_SendCode_TestModeSkipsTwilio(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	server.config.TestMode = true

	rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5559999999", "sms_opt_in": true}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	// In test mode, Twilio should NOT have been called
	if mock.LastSendPhone != "" {
		t.Errorf("Twilio should not be called in test mode, but got phone %s", mock.LastSendPhone)
	}
}

func TestCov_SendCode_ContentTypeNotJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/send-code", strings.NewReader("phone_number=5551234567"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleSendCode(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Status: got %d, want 415", rr.Code)
	}
}

// --- linkEmail ---

func TestCov_LinkEmail_EmptyEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Email is required" {
		t.Errorf("Error: got %v, want 'Email is required'", body["error"])
	}
}

func TestCov_LinkEmail_InvalidFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "not-valid-email"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid email address" {
		t.Errorf("Error: got %v, want 'Invalid email address'", body["error"])
	}
}

func TestCov_LinkEmail_WhitespaceOnlyEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "   "},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_LinkEmail_CaseNormalization(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]string{"provider": "email", "email": "  TestUser@Example.COM  "},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify stored normalized
	var storedEmail string
	server.db.QueryRow("SELECT provider_id FROM auth_methods WHERE user_id = ? AND provider = 'email'", userID).Scan(&storedEmail)
	if storedEmail != "testuser@example.com" {
		t.Errorf("Email should be normalized, got %q", storedEmail)
	}
}

// --- linkPhone ---

func TestCov_LinkPhone_EmptyPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TestMode = true

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user@test.com", "user@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "", "code": "000000"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid phone number" {
		t.Errorf("Error: got %v, want 'Invalid phone number'", body["error"])
	}
}

func TestCov_LinkPhone_EmptyCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TestMode = true

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user2@test.com", "user2@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "5551234567", "code": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Verification code is required" {
		t.Errorf("Error: got %v, want 'Verification code is required'", body["error"])
	}
}

func TestCov_LinkPhone_InvalidPhone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TestMode = true

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user3@test.com", "user3@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "123", "code": "000000"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_LinkPhone_WrongCode(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: false}
	server := newTestServer(t, mock)
	// Not in test mode — so it will call Twilio for code verification
	server.config.TestMode = false

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user4@test.com", "user4@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "5551234567", "code": "999999"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid verification code" {
		t.Errorf("Error: got %v, want 'Invalid verification code'", body["error"])
	}
}

func TestCov_LinkPhone_TwilioCheckError(t *testing.T) {
	mock := &MockTwilioClient{CheckErr: fmt.Errorf("twilio down")}
	server := newTestServer(t, mock)
	server.config.TestMode = false

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user5@test.com", "user5@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link",
		map[string]interface{}{"provider": "phone", "phone": "5551234567", "code": "123456"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

func TestCov_LinkPhone_IdempotentSameUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	server.config.TestMode = true

	emailUser, _ := resolveOrCreateUser(server.db, "email", "user6@test.com", "user6@test.com", "Test")
	token, _ := server.sessions.CreateSession(emailUser.ID, "", "email")
	cookie := &http.Cookie{Name: "session", Value: token}

	body := map[string]interface{}{"provider": "phone", "phone": "5557776666", "code": "000000"}

	// First link
	rr := doRequest(server.handleAuthLink, "POST", "/api/auth/link", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First link: got %d, want 200", rr.Code)
	}

	// Second link (idempotent)
	rr = doRequest(server.handleAuthLink, "POST", "/api/auth/link", body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Idempotent link: got %d, want 200", rr.Code)
	}

	// Verify only one auth_methods row
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE user_id = ? AND provider = 'phone'", emailUser.ID).Scan(&count)
	if count != 1 {
		t.Errorf("auth_methods phone rows: got %d, want 1", count)
	}
}

// --- handleEmailRegister ---

func TestCov_EmailRegister_PasswordTooLong(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	longPassword := strings.Repeat("a", 73)
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "toolong@example.com", "password": longPassword}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "8-72 characters") {
		t.Errorf("Error should mention 8-72 characters, got %q", errMsg)
	}
}

func TestCov_EmailRegister_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/register", bytes.NewBufferString("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleEmailRegister(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_EmailRegister_EmptyPassword(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "valid@example.com", "password": ""}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_EmailRegister_NameStored(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "named@example.com", "password": "securepassword1", "name": "Alice Smith"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	userID := body["user_id"].(string)

	var name string
	server.db.QueryRow("SELECT COALESCE(name,'') FROM users WHERE id = ?", userID).Scan(&name)
	if name != "Alice Smith" {
		t.Errorf("Name: got %q, want 'Alice Smith'", name)
	}
}

func TestCov_EmailRegister_EmailCaseNormalization(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "  UPPER@EXAMPLE.COM  ", "password": "securepassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify stored as lowercase
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM auth_methods WHERE provider = 'email' AND provider_id = 'upper@example.com'").Scan(&count)
	if count != 1 {
		t.Errorf("Email should be stored lowercase, got count %d", count)
	}
}

func TestCov_EmailRegister_SessionCookieProperties(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "cookie@example.com", "password": "securepassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	cookie := getSessionCookie(rr)
	if cookie == nil {
		t.Fatal("No session cookie")
	}
	if !cookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}
	if len(cookie.Value) != 64 {
		t.Errorf("Cookie value length: got %d, want 64", len(cookie.Value))
	}
}

// --- handleEmailLogin ---

func TestCov_EmailLogin_RateLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register a user first
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "ratelimit@example.com", "password": "securepassword1"}, nil)

	// Send 10 failed login attempts (max per IP per 10 min)
	for i := 0; i < 10; i++ {
		doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
			map[string]string{"email": "ratelimit@example.com", "password": "wrongpassword"}, nil)
	}

	// 11th should be rate limited
	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "ratelimit@example.com", "password": "wrongpassword"}, nil)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Status: got %d, want 429", rr.Code)
	}
}

func TestCov_EmailLogin_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/login", bytes.NewBufferString("}{"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleEmailLogin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_EmailLogin_NoPasswordHash(t *testing.T) {
	// When email auth method exists but has no password hash (e.g., Google auth user)
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := resolveOrCreateUser(server.db, "email", "nohash@example.com", "nohash@example.com", "No Hash")
	// auth_methods row exists but password_hash is NULL
	server.db.Exec("UPDATE auth_methods SET password_hash = NULL WHERE user_id = ? AND provider = 'email'", user.ID)

	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "nohash@example.com", "password": "anypassword1"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestCov_EmailLogin_EmptyPasswordHash(t *testing.T) {
	// When email auth method exists but password_hash is empty string
	server := newTestServer(t, &MockTwilioClient{})

	user, _ := resolveOrCreateUser(server.db, "email", "emptyhash@example.com", "emptyhash@example.com", "Empty Hash")
	server.db.Exec("UPDATE auth_methods SET password_hash = '' WHERE user_id = ? AND provider = 'email'", user.ID)

	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "emptyhash@example.com", "password": "anypassword1"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestCov_EmailLogin_UpdatesLoginTracking(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "track@example.com", "password": "securepassword1"}, nil)

	// Get user ID
	var userID string
	server.db.QueryRow("SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'track@example.com'").Scan(&userID)

	// Get initial login count (register counts as 0 login_count, since it doesn't call the login tracking)
	var initialCount int
	server.db.QueryRow("SELECT login_count FROM users WHERE id = ?", userID).Scan(&initialCount)

	// Login
	doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "track@example.com", "password": "securepassword1"}, nil)

	// Check login count increased
	var newCount int
	server.db.QueryRow("SELECT login_count FROM users WHERE id = ?", userID).Scan(&newCount)
	if newCount != initialCount+1 {
		t.Errorf("login_count: got %d, want %d", newCount, initialCount+1)
	}
}

func TestCov_EmailLogin_EmailCaseInsensitiveInput(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "case@example.com", "password": "securepassword1"}, nil)

	// Login with uppercase email
	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "  CASE@EXAMPLE.COM  ", "password": "securepassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200 (case insensitive login)", rr.Code)
	}
}

// --- handleEmailForgot ---

func TestCov_EmailForgot_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/forgot", bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleEmailForgot(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_EmailForgot_ContentTypeNotJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/forgot", strings.NewReader("email=test@example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	server.handleEmailForgot(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Status: got %d, want 415", rr.Code)
	}
}

func TestCov_EmailForgot_TokenCreated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// No SendGrid key, so email won't actually send

	// Register a user
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "forgot@example.com", "password": "securepassword1"}, nil)

	// Request password reset
	rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "forgot@example.com"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify token was created in password_resets table
	var tokenCount int
	server.db.QueryRow("SELECT COUNT(*) FROM password_resets WHERE user_id = (SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'forgot@example.com')").Scan(&tokenCount)
	if tokenCount < 1 {
		t.Error("Password reset token should be created")
	}
}

func TestCov_EmailForgot_EmailCaseNormalized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "forgotcase@example.com", "password": "securepassword1"}, nil)

	// Request with uppercase email
	rr := doRequest(server.handleEmailForgot, "POST", "/auth/email/forgot",
		map[string]string{"email": "  FORGOTCASE@EXAMPLE.COM  "}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Should find the user and create a token
	var tokenCount int
	server.db.QueryRow("SELECT COUNT(*) FROM password_resets WHERE user_id = (SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'forgotcase@example.com')").Scan(&tokenCount)
	if tokenCount < 1 {
		t.Error("Should find user with case-normalized email")
	}
}

// --- handleMagicLink ---

func TestCov_MagicLink_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/magic-link", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleMagicLink(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_MagicLink_ContentTypeNotJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/magic-link", strings.NewReader("email=test@example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	server.handleMagicLink(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Status: got %d, want 415", rr.Code)
	}
}

func TestCov_MagicLink_TokenCreatedWithMagicPrefix(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "magic@example.com", "password": "securepassword1"}, nil)

	rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": "magic@example.com"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify token with "magic:" prefix in password_resets table
	var token string
	server.db.QueryRow(`SELECT token FROM password_resets WHERE user_id =
		(SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'magic@example.com')
		AND token LIKE 'magic:%'`).Scan(&token)
	if !strings.HasPrefix(token, "magic:") {
		t.Errorf("Token should have magic: prefix, got %q", token)
	}
}

func TestCov_MagicLink_EmailCaseNormalized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "magiccase@example.com", "password": "securepassword1"}, nil)

	rr := doRequest(server.handleMagicLink, "POST", "/auth/email/magic-link",
		map[string]string{"email": "  MAGICCASE@EXAMPLE.COM  "}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Token should be created for the right user
	var tokenCount int
	server.db.QueryRow(`SELECT COUNT(*) FROM password_resets WHERE user_id =
		(SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'magiccase@example.com')`).Scan(&tokenCount)
	if tokenCount < 1 {
		t.Error("Should find user with case-normalized email")
	}
}

// --- handleEmailReset ---

func TestCov_EmailReset_PasswordTooLong(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	longPassword := strings.Repeat("b", 73)
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": "sometoken", "password": longPassword}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
	body := parseJSON(rr)
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "8-72 characters") {
		t.Errorf("Error should mention password length, got %q", errMsg)
	}
}

func TestCov_EmailReset_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/reset", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleEmailReset(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_EmailReset_ContentTypeNotJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("POST", "/auth/email/reset", strings.NewReader("token=x&password=12345678"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	server.handleEmailReset(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Status: got %d, want 415", rr.Code)
	}
}

func TestCov_EmailReset_InvalidatesSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register user
	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "sessions@example.com", "password": "originalpass1"}, nil)

	// Get user ID
	var userID string
	server.db.QueryRow("SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'sessions@example.com'").Scan(&userID)

	// Create additional sessions for this user
	tok1, _ := server.sessions.CreateSession(userID, "", "email")
	tok2, _ := server.sessions.CreateSession(userID, "", "email")

	// Create a password reset token
	resetToken := "test-reset-token-sessions"
	server.db.Exec("INSERT INTO password_resets (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, resetToken, time.Now().UTC().Add(1*time.Hour))

	// Reset password
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": resetToken, "password": "newpassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify all old sessions are invalidated
	_, ok1 := server.sessions.GetSession(tok1)
	_, ok2 := server.sessions.GetSession(tok2)
	if ok1 || ok2 {
		t.Error("All sessions should be invalidated after password reset")
	}
}

func TestCov_EmailReset_TokenMarkedUsed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "usedtoken@example.com", "password": "originalpass1"}, nil)

	var userID string
	server.db.QueryRow("SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'usedtoken@example.com'").Scan(&userID)

	resetToken := "test-used-token-check"
	server.db.Exec("INSERT INTO password_resets (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, resetToken, time.Now().UTC().Add(1*time.Hour))

	// First reset should work
	rr := doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": resetToken, "password": "newpassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("First reset: got %d, want 200", rr.Code)
	}

	// Second reset with same token should fail (token marked as used)
	rr = doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": resetToken, "password": "anotherpassword1"}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Second reset: got %d, want 400 (token already used)", rr.Code)
	}
}

func TestCov_EmailReset_PasswordActuallyChanged(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	doRequest(server.handleEmailRegister, "POST", "/auth/email/register",
		map[string]string{"email": "changed@example.com", "password": "originalpass1"}, nil)

	var userID string
	server.db.QueryRow("SELECT user_id FROM auth_methods WHERE provider = 'email' AND provider_id = 'changed@example.com'").Scan(&userID)

	resetToken := "test-changed-password"
	server.db.Exec("INSERT INTO password_resets (user_id, token, expires_at) VALUES (?, ?, ?)",
		userID, resetToken, time.Now().UTC().Add(1*time.Hour))

	doRequest(server.handleEmailReset, "POST", "/auth/email/reset",
		map[string]string{"token": resetToken, "password": "brandnewpassword1"}, nil)

	// Old password should no longer work
	rr := doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "changed@example.com", "password": "originalpass1"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Old password should fail: got %d, want 401", rr.Code)
	}

	// New password should work
	rr = doRequest(server.handleEmailLogin, "POST", "/auth/email/login",
		map[string]string{"email": "changed@example.com", "password": "brandnewpassword1"}, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("New password should work: got %d, want 200", rr.Code)
	}
}

// --- handleAuthLink edge cases ---

func TestCov_AuthLink_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/auth/link", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAuthLink(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestCov_AuthLink_ContentTypeNotJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/auth/link", strings.NewReader("provider=email"))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleAuthLink(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Status: got %d, want 415", rr.Code)
	}
}

// --- handleSendCode edge cases ---

func TestCov_SendCode_TestModeNoRateLimit(t *testing.T) {
	// In test mode, per-phone rate limiting is skipped
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TestMode = true

	// Send multiple requests to the same phone — all should succeed in test mode
	for i := 0; i < 3; i++ {
		rr := doRequest(server.handleSendCode, "POST", "/auth/send-code",
			map[string]interface{}{"phone_number": "5551234567", "sms_opt_in": true}, nil)
		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: got %d, want 200 (test mode should skip rate limiting)", i+1, rr.Code)
		}
	}
}

func TestCov_SendCode_DifferentPhonesNoPerPhoneRateLimit(t *testing.T) {
	// Different phones should each be allowed even with rate limiting enabled
	server := newTestServer(t, &MockTwilioClient{})
	server.config.TestMode = false

	rr1 := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5551000001", "sms_opt_in": true}, nil)
	if rr1.Code != http.StatusOK {
		t.Errorf("Phone 1: got %d, want 200", rr1.Code)
	}

	rr2 := doRequest(server.handleSendCode, "POST", "/auth/send-code",
		map[string]interface{}{"phone_number": "5551000002", "sms_opt_in": true}, nil)
	if rr2.Code != http.StatusOK {
		t.Errorf("Phone 2: got %d, want 200", rr2.Code)
	}
}

// =============================================================================
// TestCov2_ — COVERAGE BOOST ROUND 2
// =============================================================================

// --- handleLeaveHousehold (56.5%) ---
// Already tested: wrong method, unauthenticated, happy path (solo & shared).
// Uncovered: user with NULL household_id (no household row), verifies old
// household_members cleaned up, double-leave idempotency.

func TestCov2_LeaveHousehold_NullHouseholdID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557770001")

	// Force household_id to NULL to simulate a legacy user
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID)
	server.db.Exec("UPDATE users SET household_id = NULL WHERE id = ?", userID)

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	// Should have a new household now
	var newHID int
	server.db.QueryRow("SELECT COALESCE(household_id, 0) FROM users WHERE id = ?", userID).Scan(&newHID)
	if newHID == 0 {
		t.Error("User should have a new household after leave even from NULL state")
	}
}

func TestCov2_LeaveHousehold_DoubleLeaveSameUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557770002")

	// Leave once
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First leave: got %d, want 200", rr.Code)
	}
	var hid1 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid1)

	// Leave again — idempotent, should succeed with a NEW household
	rr = doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Second leave: got %d, want 200", rr.Code)
	}
	var hid2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hid2)
	if hid2 == 0 {
		t.Error("Should have a household after double leave")
	}
	if hid2 == hid1 {
		t.Error("Double leave should create a different household each time")
	}
}

func TestCov2_LeaveHousehold_OldMembersCleanedUp(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5557770003")
	cookie2, userID2 := loginUser(t, server, "5557770004")

	// Put user2 into user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// Count members in old household before leave
	var oldCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE household_id = ? AND user_id = ?", hID, userID2).Scan(&oldCount)
	if oldCount != 1 {
		t.Fatalf("Setup: expected 1 member row for user2 in old household, got %d", oldCount)
	}

	// User2 leaves
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Old membership should be gone
	var afterCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE household_id = ? AND user_id = ?", hID, userID2).Scan(&afterCount)
	if afterCount != 0 {
		t.Errorf("Old household_members should be cleaned up, got %d rows", afterCount)
	}
}

func TestCov2_LeaveHousehold_DBClosed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557770005")
	server.db.Close()

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Should fail when DB is closed")
	}
}

// --- handleReferralRedeem (69.6%) ---
// Already tested: insufficient points, exact threshold success.
// Uncovered: unauthenticated, wrong method, zero points, redeem twice in a row,
// points deducted correctly after redeem, redemption record created.

func TestCov2_ReferralRedeem_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557780001")

	rr := doRequest(server.handleReferralRedeem, "GET", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", rr.Code)
	}
}

func TestCov2_ReferralRedeem_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_ReferralRedeem_ZeroPoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557780002")

	// Ensure zero points
	server.db.Exec("UPDATE users SET referral_points = 0 WHERE id = ?", userID)

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Zero points should return 400, got %d", rr.Code)
	}
}

func TestCov2_ReferralRedeem_PointsDeductedCorrectly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557780003")

	// Give user 150 points (more than threshold)
	server.db.Exec("UPDATE users SET referral_points = 150 WHERE id = ?", userID)

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	// Points should be 150 - 100 = 50
	var remaining int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", userID).Scan(&remaining)
	if remaining != 50 {
		t.Errorf("Expected 50 points remaining, got %d", remaining)
	}

	// Verify redemption record was created
	var redemptionCount int
	server.db.QueryRow("SELECT COUNT(*) FROM redemptions WHERE user_id = ?", userID).Scan(&redemptionCount)
	if redemptionCount != 1 {
		t.Errorf("Expected 1 redemption record, got %d", redemptionCount)
	}
}

func TestCov2_ReferralRedeem_DoubleRedeem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557780004")

	// Give exactly threshold points
	server.db.Exec("UPDATE users SET referral_points = ? WHERE id = ?", referralRedeemThreshold, userID)

	// First redeem succeeds
	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First redeem: got %d, want 200", rr.Code)
	}

	// Second redeem should fail (0 points now)
	rr = doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Second redeem: got %d, want 400 (no points left)", rr.Code)
	}
}

func TestCov2_ReferralRedeem_DBClosed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557780005")
	server.db.Close()

	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Should fail when DB is closed")
	}
}

// --- handleStorePrices (67.9%) ---
// Already tested via subroute dispatch.
// Uncovered: wrong method, unauthenticated, invalid store ID in path,
// store with price data, store with no prices (empty result).

func TestCov2_StorePrices_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557790001")

	rr := doRequest(server.handleStorePrices, "POST", "/api/stores/1/prices", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should return 405, got %d", rr.Code)
	}
}

func TestCov2_StorePrices_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleStorePrices, "GET", "/api/stores/1/prices", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_StorePrices_InvalidStoreID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557790002")

	rr := doRequest(server.handleStorePrices, "GET", "/api/stores/abc/prices", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid store ID should return 400, got %d", rr.Code)
	}
}

func TestCov2_StorePrices_InvalidPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557790003")

	rr := doRequest(server.handleStorePrices, "GET", "/api/stores/1/notprices", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid path should return 400, got %d", rr.Code)
	}
}

func TestCov2_StorePrices_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557790004")

	storeID, err := server.upsertStore("PriceTest Store", "TestChain", "123 Main", "City", "ST", "00000", "", nil, nil)
	if err != nil {
		t.Fatalf("upsertStore: %v", err)
	}

	server.recordStorePrice(storeID, "Milk", "Brand A", 399, 0, "receipt")
	server.recordStorePrice(storeID, "Bread", "Brand B", 299, 0, "manual")

	rr := doRequest(server.handleStorePrices, "GET",
		fmt.Sprintf("/api/stores/%d/prices", storeID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var prices []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&prices)
	if len(prices) != 2 {
		t.Errorf("Expected 2 prices, got %d", len(prices))
	}
}

func TestCov2_StorePrices_EmptyStore(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557790005")

	storeID, _ := server.upsertStore("Empty Store", "TestChain", "456 Oak", "City", "ST", "00000", "", nil, nil)

	rr := doRequest(server.handleStorePrices, "GET",
		fmt.Sprintf("/api/stores/%d/prices", storeID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var prices []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&prices)
	if len(prices) != 0 {
		t.Errorf("Expected 0 prices for empty store, got %d", len(prices))
	}
}

// --- handleStorePriceAverages (73.2%) ---
// Already tested: basic query, product filter.
// Uncovered: brand filter, invalid store ID, wrong method, invalid path, both filters.

func TestCov2_StorePriceAverages_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800001")

	rr := doRequest(server.handleStorePriceAverages, "POST", "/api/stores/1/price-averages", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should return 405, got %d", rr.Code)
	}
}

func TestCov2_StorePriceAverages_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleStorePriceAverages, "GET", "/api/stores/1/price-averages", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_StorePriceAverages_InvalidStoreID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800002")

	rr := doRequest(server.handleStorePriceAverages, "GET", "/api/stores/xyz/price-averages", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid store ID should return 400, got %d", rr.Code)
	}
}

func TestCov2_StorePriceAverages_InvalidPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800003")

	rr := doRequest(server.handleStorePriceAverages, "GET", "/api/stores/1/not-price-averages", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid path should return 400, got %d", rr.Code)
	}
}

func TestCov2_StorePriceAverages_BrandFilter(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800004")

	storeID, _ := server.upsertStore("Avg Store", "AvgChain", "789 Pine", "City", "ST", "00000", "", nil, nil)
	server.recordStorePrice(storeID, "Ketchup", "Heinz", 349, 0, "receipt")
	server.recordStorePrice(storeID, "Ketchup", "Hunts", 249, 0, "receipt")
	server.recordStorePrice(storeID, "Mustard", "French's", 199, 0, "receipt")

	rr := doRequest(server.handleStorePriceAverages, "GET",
		fmt.Sprintf("/api/stores/%d/price-averages?brand=heinz", storeID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result with brand filter 'heinz', got %d", len(results))
	}
}

func TestCov2_StorePriceAverages_ProductAndBrandFilter(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800005")

	storeID, _ := server.upsertStore("Combo Store", "ComboChain", "999 Elm", "City", "ST", "00000", "", nil, nil)
	server.recordStorePrice(storeID, "Ketchup", "Heinz", 349, 0, "receipt")
	server.recordStorePrice(storeID, "Ketchup", "Hunts", 249, 0, "receipt")
	server.recordStorePrice(storeID, "Mustard", "Heinz", 199, 0, "receipt")

	rr := doRequest(server.handleStorePriceAverages, "GET",
		fmt.Sprintf("/api/stores/%d/price-averages?product=ketchup&brand=heinz", storeID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("Expected 1 result with both filters, got %d", len(results))
	}
}

func TestCov2_StorePriceAverages_EmptyResults(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557800006")

	storeID, _ := server.upsertStore("EmptyAvg Store", "EmptyChain", "100 Nowhere", "City", "ST", "00000", "", nil, nil)

	rr := doRequest(server.handleStorePriceAverages, "GET",
		fmt.Sprintf("/api/stores/%d/price-averages", storeID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty store, got %d", len(results))
	}
}

// --- handlePriceComparison (77.5%) ---
// Already tested: across stores, requires product, with brand filter.
// Uncovered: wrong method, unauthenticated, product only (no brand),
// user with shopping trips linking to stores, no matching prices.

func TestCov2_PriceComparison_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557810001")

	rr := doRequest(server.handlePriceComparison, "POST", "/api/price-compare?product=milk", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should return 405, got %d", rr.Code)
	}
}

func TestCov2_PriceComparison_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handlePriceComparison, "GET", "/api/price-compare?product=milk", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_PriceComparison_NoMatchingPrices(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557810002")

	rr := doRequest(server.handlePriceComparison, "GET",
		"/api/price-compare?product=nonexistentproduct12345", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	stores := result["stores"].([]interface{})
	if len(stores) != 0 {
		t.Errorf("Expected 0 stores for nonexistent product, got %d", len(stores))
	}
}

func TestCov2_PriceComparison_WithUserTrips(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557810003")

	// Create stores
	store1, _ := server.upsertStore("UserTrip Kroger", "Kroger", "1 A St", "City", "OH", "43215", "", nil, nil)
	store2, _ := server.upsertStore("UserTrip Walmart", "Walmart", "2 B St", "City", "OH", "43215", "", nil, nil)

	// Record prices
	server.recordStorePrice(store1, "Eggs Dozen", "Farm Fresh", 299, 0, "receipt")
	server.recordStorePrice(store2, "Eggs Dozen", "Farm Fresh", 349, 0, "receipt")

	// Create a shopping trip for the user linked to store1
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID).Scan(&hID)
	server.db.Exec("INSERT INTO shopping_trips (user_id, household_id, trip_date, store_id) VALUES (?, ?, '2026-03-08', ?)",
		userID, hID, store1)

	rr := doRequest(server.handlePriceComparison, "GET",
		"/api/price-compare?product=eggs", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	stores := result["stores"].([]interface{})
	if len(stores) < 2 {
		t.Errorf("Expected at least 2 stores, got %d", len(stores))
	}
	// Verify user_store_ids includes the user's store
	userStores := result["user_store_ids"]
	if userStores == nil {
		t.Fatal("Expected user_store_ids in response")
	}
	userStoreList := userStores.([]interface{})
	if len(userStoreList) != 1 {
		t.Errorf("Expected 1 user store, got %d", len(userStoreList))
	}
}

func TestCov2_PriceComparison_ProductOnlyNoBrand(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557810004")

	store1, _ := server.upsertStore("NoBrand Store1", "Chain1", "10 A", "City", "ST", "00000", "", nil, nil)
	server.recordStorePrice(store1, "Butter", "Land O Lakes", 499, 0, "receipt")

	rr := doRequest(server.handlePriceComparison, "GET",
		"/api/price-compare?product=butter", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	if result["product"] != "butter" {
		t.Errorf("Expected product='butter', got %q", result["product"])
	}
}

// --- handleRegionalSwapPerformance (76.2%) ---
// Already tested: admin-only, returns data with store-linked swaps.
// Uncovered: wrong method, empty data, unknown region (no store link).

func TestCov2_RegionalSwapPerformance_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleRegionalSwapPerformance, "POST", "/api/admin/regional-swaps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should return 405, got %d", rr.Code)
	}
}

func TestCov2_RegionalSwapPerformance_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleRegionalSwapPerformance, "GET", "/api/admin/regional-swaps", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_RegionalSwapPerformance_EmptyData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	rr := doRequest(server.handleRegionalSwapPerformance, "GET", "/api/admin/regional-swaps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestCov2_RegionalSwapPerformance_UnknownRegion(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	admin, _ := upsertUser(server.db, "3104861833")
	token, _ := server.sessions.CreateSession(admin.ID, "3104861833", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Create swap events with no store link (should appear as "Unknown" region)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-999', '5559999999')")
	server.db.Exec("INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status) VALUES (1, 'X', 'Y', 'Z', 100, 10000, 'active')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-999', 1, 'offered')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-999', 1, 'declined')")

	rr := doRequest(server.handleRegionalSwapPerformance, "GET", "/api/admin/regional-swaps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) == 0 {
		t.Fatal("Expected at least 1 result for unknown region")
	}

	// Check that accept_rate is computed correctly (0 accepted / 1 offered = 0)
	found := false
	for _, r := range results {
		if r["state"] == "Unknown" {
			found = true
			offered := int(r["total_offered"].(float64))
			declined := int(r["total_declined"].(float64))
			if offered != 1 {
				t.Errorf("Expected 1 offered, got %d", offered)
			}
			if declined != 1 {
				t.Errorf("Expected 1 declined, got %d", declined)
			}
			rate := r["accept_rate"].(float64)
			if rate != 0 {
				t.Errorf("Expected 0 accept rate, got %f", rate)
			}
		}
	}
	if !found {
		t.Error("Expected 'Unknown' region in results")
	}
}

// --- adminDB (66.7%) ---
// Only 2 branches: reportDB is nil vs non-nil.

func TestCov2_AdminDB_NilReportDB(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// reportDB removed — adminDB() always returns the primary DB now
	db := server.adminDB()
	if db != server.db.DB {
		t.Error("adminDB() should return primary DB")
	}
}

func TestCov2_AdminDB_WithReportDB(t *testing.T) {
	t.Skip("reportDB removed — reporting now uses the primary database")
}

// --- handleStores (78.4%) ---
// Uncovered: chain filter, search query filter, custom limit, stores with lat/lng.

func TestCov2_Stores_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820001")

	rr := doRequest(server.handleStores, "POST", "/api/stores", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should return 405, got %d", rr.Code)
	}
}

func TestCov2_Stores_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleStores, "GET", "/api/stores", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_Stores_EmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820002")

	rr := doRequest(server.handleStores, "GET", "/api/stores", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 0 {
		t.Errorf("Expected 0 stores, got %d", len(stores))
	}
}

func TestCov2_Stores_ChainFilter(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820003")

	server.upsertStore("Kroger #1", "Kroger", "1 A St", "City", "OH", "43215", "", nil, nil)
	server.upsertStore("Walmart #1", "Walmart", "2 B St", "City", "OH", "43215", "", nil, nil)
	server.upsertStore("Kroger #2", "Kroger", "3 C St", "City", "OH", "43215", "", nil, nil)

	rr := doRequest(server.handleStores, "GET", "/api/stores?chain=Kroger", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 2 {
		t.Errorf("Expected 2 Kroger stores, got %d", len(stores))
	}
}

func TestCov2_Stores_SearchQuery(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820004")

	server.upsertStore("Target Downtown", "Target", "100 Main", "Springfield", "IL", "62701", "", nil, nil)
	server.upsertStore("Walmart Supercenter", "Walmart", "200 Oak", "Springfield", "IL", "62701", "", nil, nil)

	rr := doRequest(server.handleStores, "GET", "/api/stores?q=target", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 1 {
		t.Errorf("Expected 1 store matching 'target', got %d", len(stores))
	}
}

func TestCov2_Stores_CustomLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820005")

	for i := 0; i < 5; i++ {
		server.upsertStore(fmt.Sprintf("Store %d", i), "TestChain", fmt.Sprintf("%d Z St", i), "City", "ST", "00000", "", nil, nil)
	}

	rr := doRequest(server.handleStores, "GET", "/api/stores?limit=2", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 2 {
		t.Errorf("Expected 2 stores with limit=2, got %d", len(stores))
	}
}

func TestCov2_Stores_InvalidLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820006")

	server.upsertStore("Only Store", "OnlyChain", "1 Only St", "City", "ST", "00000", "", nil, nil)

	// Non-numeric limit should fall back to default 50
	rr := doRequest(server.handleStores, "GET", "/api/stores?limit=abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 1 {
		t.Errorf("Expected 1 store, got %d", len(stores))
	}
}

func TestCov2_Stores_WithLatLng(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820007")

	lat := 39.9612
	lng := -82.9988
	server.upsertStore("GeoStore", "GeoChain", "777 Geo Rd", "Columbus", "OH", "43215", "place123", &lat, &lng)

	rr := doRequest(server.handleStores, "GET", "/api/stores", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 1 {
		t.Fatalf("Expected 1 store, got %d", len(stores))
	}
	if stores[0]["latitude"] == nil {
		t.Error("Expected non-nil latitude")
	}
	if stores[0]["longitude"] == nil {
		t.Error("Expected non-nil longitude")
	}
}

func TestCov2_Stores_SearchByAddress(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557820008")

	server.upsertStore("Some Store", "SomeChain", "742 Evergreen Terrace", "Springfield", "IL", "62701", "", nil, nil)

	rr := doRequest(server.handleStores, "GET", "/api/stores?q=evergreen", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var stores []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&stores)
	if len(stores) != 1 {
		t.Errorf("Expected 1 store matching address 'evergreen', got %d", len(stores))
	}
}

// --- handleReferralClaim (81.6%) ---
// Already tested: basic claim flow with referral code.
// Uncovered: wrong method, unauthenticated, empty code, invalid code,
// self-referral, idempotent claim (already claimed), referral_source tracking.

func TestCov2_ReferralClaim_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557830001")

	rr := doRequest(server.handleReferralClaim, "GET", "/api/referral/claim", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", rr.Code)
	}
}

func TestCov2_ReferralClaim_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "abc123"}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov2_ReferralClaim_EmptyCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557830002")

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": ""}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Empty code should return 400, got %d", rr.Code)
	}
}

func TestCov2_ReferralClaim_InvalidCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5557830003")

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": "nonexistent_code_xyz"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Invalid code should return 404, got %d", rr.Code)
	}
}

func TestCov2_ReferralClaim_SelfReferral(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5557830004")

	// Generate referral code by calling GET /api/referral
	doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie})
	var code string
	server.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", userID).Scan(&code)
	if code == "" {
		t.Fatal("User should have a referral code after GET /api/referral")
	}

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Self-referral should return 400, got %d", rr.Code)
	}
}

func TestCov2_ReferralClaim_IdempotentClaim(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, referrerID := loginUser(t, server, "5557830005")
	cookie2, _ := loginUser(t, server, "5557830006")

	// Generate referral code by calling GET /api/referral
	doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie1})
	var code string
	server.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", referrerID).Scan(&code)
	if code == "" {
		t.Fatal("Referrer should have a referral code")
	}

	// First claim
	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("First claim: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	// Get referrer points after first claim
	var pointsAfterFirst int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", referrerID).Scan(&pointsAfterFirst)

	// Second claim — idempotent, should succeed without double-crediting
	rr = doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Second claim (idempotent): got %d, want 200", rr.Code)
	}

	// Points should not have increased
	var pointsAfterSecond int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", referrerID).Scan(&pointsAfterSecond)
	if pointsAfterSecond != pointsAfterFirst {
		t.Errorf("Idempotent claim should not double-credit: first=%d, second=%d", pointsAfterFirst, pointsAfterSecond)
	}
}

func TestCov2_ReferralClaim_SetsReferralSource(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, referrerID := loginUser(t, server, "5557830007")
	cookie2, userID2 := loginUser(t, server, "5557830008")

	// Generate referral code
	doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie1})
	var code string
	server.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", referrerID).Scan(&code)

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	// Verify referral_source was set
	var source string
	server.db.QueryRow("SELECT COALESCE(referral_source, '') FROM users WHERE id = ?", userID2).Scan(&source)
	if source != "referral" {
		t.Errorf("Expected referral_source='referral', got %q", source)
	}

	// Verify referral record exists
	var refCount int
	server.db.QueryRow("SELECT COUNT(*) FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, userID2).Scan(&refCount)
	if refCount != 1 {
		t.Errorf("Expected 1 referral record, got %d", refCount)
	}
}

func TestCov2_ReferralClaim_PointsAwarded(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, referrerID := loginUser(t, server, "5557830009")
	cookie2, _ := loginUser(t, server, "5557830010")

	// Reset referrer points to 0
	server.db.Exec("UPDATE users SET referral_points = 0 WHERE id = ?", referrerID)

	// Generate referral code
	doRequest(server.handleReferral, "GET", "/api/referral", nil, []*http.Cookie{cookie1})
	var code string
	server.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", referrerID).Scan(&code)

	rr := doRequest(server.handleReferralClaim, "POST", "/api/referral/claim",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var points int
	server.db.QueryRow("SELECT COALESCE(referral_points, 0) FROM users WHERE id = ?", referrerID).Scan(&points)
	if points != referralPointsPerInvite {
		t.Errorf("Referrer should have %d points, got %d", referralPointsPerInvite, points)
	}
}

// --- handleJoin (79.8%) ---
// Already tested: invalid code, empty code, already used, expired, self-join,
// happy path with inviter points. Uncovered: already in target household (B7),
// leaving previous household to join new one (B3 warning), joiner with name
// vs without name (penny message branch), SMS notification to inviter.

func TestCov2_Join_AlreadyInTargetHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5557840001")
	cookie2, userID2 := loginUser(t, server, "5557840002")

	// Put user2 into user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// Create an invite from user1
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 tries to join — already in target household (B7)
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Already in household should return 200, got %d; body=%s", rr.Code, rr.Body.String())
	}
	body = parseJSON(rr)
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "already") {
		t.Errorf("Expected 'already in this household' message, got %q", msg)
	}

	// Invite should NOT be consumed
	var usedBy interface{}
	server.db.QueryRow("SELECT used_by FROM invites WHERE code = ?", code).Scan(&usedBy)
	if usedBy != nil {
		t.Error("Invite should not be consumed when user is already in household")
	}
}

func TestCov2_Join_LeavePreviousHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5557840003")
	cookie2, userID2 := loginUser(t, server, "5557840004")
	_, userID3 := loginUser(t, server, "5557840005")

	// Put user2 and user3 in a shared household
	var hID2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&hID2)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID3)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID2, userID3)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID2, userID3)

	// User1 creates invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 joins user1's household (leaving their own)
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Join: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body = parseJSON(rr)
	warning, _ := body["warning"].(string)
	if !strings.Contains(warning, "previous household") {
		t.Errorf("Expected warning about leaving previous household, got %q", warning)
	}

	// Verify user2 is in user1's household now
	var newHID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&newHID)
	var hID1 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&hID1)
	if newHID == hID2 {
		t.Error("User2 should have moved to a new household")
	}
}

func TestCov2_Join_JoinerWithName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5557840006")
	cookie2, userID2 := loginUser(t, server, "5557840007")

	// Set user2's name
	server.db.Exec("UPDATE users SET name = 'Alice' WHERE id = ?", userID2)

	// Create invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 joins
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body = parseJSON(rr)
	pennyMsg, _ := body["penny_message"].(string)
	if !strings.Contains(pennyMsg, "Alice") {
		t.Errorf("Penny message should include joiner name 'Alice', got %q", pennyMsg)
	}
}

func TestCov2_Join_JoinerWithoutName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5557840008")
	cookie2, userID2 := loginUser(t, server, "5557840009")

	// Ensure user2 has no name
	server.db.Exec("UPDATE users SET name = '' WHERE id = ?", userID2)

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body = parseJSON(rr)
	pennyMsg, _ := body["penny_message"].(string)
	if !strings.Contains(pennyMsg, "Welcome to the family") {
		t.Errorf("Penny message without name should be generic, got %q", pennyMsg)
	}
}

func TestCov2_Join_InviterSMSNotification(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie1, userID1 := loginUser(t, server, "5557840010")
	cookie2, userID2 := loginUser(t, server, "5557840011")

	// Set joiner's name
	server.db.Exec("UPDATE users SET name = 'Bob' WHERE id = ?", userID2)

	// Create invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 joins — inviter should receive SMS notification
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	// Check that SMS was sent to inviter
	smsTo, smsBody := mock.GetLastSMS()
	if smsTo == "" {
		t.Error("Expected SMS notification to inviter")
	}
	if !strings.Contains(smsBody, "Bob") {
		t.Errorf("SMS should mention joiner name 'Bob', got %q", smsBody)
	}
	if !strings.Contains(smsBody, "joined") {
		t.Errorf("SMS should mention joining, got %q", smsBody)
	}

	_ = userID1
}

func TestCov2_Join_InviterNotificationDedup(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)
	cookie1, _ := loginUser(t, server, "5557840012")
	cookie2, _ := loginUser(t, server, "5557840013")

	// Create invite
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	// User2 joins
	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	// Verify notified flag is set (preventing duplicate SMS)
	var notified int
	server.db.QueryRow("SELECT notified FROM invites WHERE code = ?", code).Scan(&notified)
	if notified != 1 {
		t.Errorf("Expected notified=1, got %d", notified)
	}
}

func TestCov2_Join_SetsReferralSourceInvite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5557840014")
	cookie2, userID2 := loginUser(t, server, "5557840015")

	// Ensure referral_source is empty initially
	server.db.Exec("UPDATE users SET referral_source = '' WHERE id = ?", userID2)

	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite",
		map[string]string{"phone": ""}, []*http.Cookie{cookie1})
	body := parseJSON(rr)
	code := body["code"].(string)

	rr = doRequest(server.handleJoin, "POST", "/api/join",
		map[string]string{"code": code}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var source string
	server.db.QueryRow("SELECT COALESCE(referral_source, '') FROM users WHERE id = ?", userID2).Scan(&source)
	if source != "invite" {
		t.Errorf("Expected referral_source='invite', got %q", source)
	}
}

// =============================================================================
// BATCH 5: Utility Functions Coverage
// =============================================================================

func TestParseFloat_ValidCases(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"3.14", 3.14},
		{"0", 0},
		{"100.5", 100.5},
		{"-2.5", -2.5},
		{"", 0},
		{"not_a_number", 0},
	}
	for _, tc := range tests {
		got := parseFloat(tc.input)
		if got != tc.want {
			t.Errorf("parseFloat(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestPruneTimeLimiter(t *testing.T) {
	var store sync.Map

	// Add some entries: one fresh, one stale
	store.Store("fresh", []time.Time{time.Now()})
	store.Store("stale", []time.Time{time.Now().Add(-2 * time.Hour)})
	store.Store("mixed", []time.Time{
		time.Now().Add(-2 * time.Hour), // stale
		time.Now(),                      // fresh
	})

	pruneTimeLimiter(&store, 1*time.Hour)

	// Fresh should remain
	if _, ok := store.Load("fresh"); !ok {
		t.Error("Expected 'fresh' to remain after pruning")
	}
	// Stale should be removed
	if _, ok := store.Load("stale"); ok {
		t.Error("Expected 'stale' to be removed after pruning")
	}
	// Mixed should remain (has at least one fresh entry)
	if _, ok := store.Load("mixed"); !ok {
		t.Error("Expected 'mixed' to remain after pruning (has fresh entries)")
	}
}

func TestPruneBucketLimiter(t *testing.T) {
	var store sync.Map

	fresh := &rateBucket{times: []time.Time{time.Now()}}
	stale := &rateBucket{times: []time.Time{time.Now().Add(-2 * time.Hour)}}

	store.Store("fresh", fresh)
	store.Store("stale", stale)

	pruneBucketLimiter(&store, 1*time.Hour)

	if _, ok := store.Load("fresh"); !ok {
		t.Error("Expected 'fresh' bucket to remain")
	}
	if _, ok := store.Load("stale"); ok {
		t.Error("Expected 'stale' bucket to be removed")
	}
}

func TestCachePriceAndGetCachedPrice(t *testing.T) {
	server := newTestServer(t, nil)

	// Ensure price_cache table exists
	server.db.Exec(`CREATE TABLE IF NOT EXISTS price_cache (
		brand TEXT, category TEXT, zip TEXT, price_cents INTEGER,
		unit_price TEXT, unit_label TEXT, size TEXT,
		product_name TEXT, store_name TEXT, fetched_at DATETIME,
		PRIMARY KEY (brand, category, zip)
	)`)

	p := &BlueCartPrice{
		Brand:       "TestBrand",
		Category:    "Ketchup",
		Zip:         "90210",
		PriceCents:  499,
		UnitPrice:   "0.25",
		UnitLabel:   "per oz",
		Size:        "20 oz",
		ProductName: "TestBrand Ketchup 20oz",
		StoreName:   "TestStore",
	}

	server.cachePrice(p)

	cached, err := server.getCachedPrice("TestBrand", "Ketchup", "90210")
	if err != nil {
		t.Fatalf("getCachedPrice error: %v", err)
	}
	if cached.PriceCents != 499 {
		t.Errorf("PriceCents: got %d, want 499", cached.PriceCents)
	}
	if cached.ProductName != "TestBrand Ketchup 20oz" {
		t.Errorf("ProductName: got %q, want 'TestBrand Ketchup 20oz'", cached.ProductName)
	}

	// Case insensitive lookup
	cached2, err := server.getCachedPrice("testbrand", "ketchup", "90210")
	if err != nil {
		t.Fatalf("Case-insensitive lookup error: %v", err)
	}
	if cached2.PriceCents != 499 {
		t.Errorf("Case-insensitive: got %d, want 499", cached2.PriceCents)
	}

	// Non-existent
	_, err = server.getCachedPrice("NoSuchBrand", "Nothing", "00000")
	if err == nil {
		t.Error("Expected error for non-existent cache entry")
	}
}

func TestGetUserZip(t *testing.T) {
	server := newTestServer(t, nil)
	cookie, _ := loginUser(t, server, "5551110001")

	// Get user ID
	rr := doRequest(server.handleMe, "GET", "/auth/me", nil, []*http.Cookie{cookie})
	var me map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &me)
	userID := me["user_id"].(string)

	// No address set — should return empty
	zip := server.getUserZip(userID)
	if zip != "" {
		t.Errorf("Expected empty zip, got %q", zip)
	}

	// Set address with zip
	server.db.Exec("UPDATE users SET address = ? WHERE id = ?", "123 Main St, Beverly Hills, CA 90210", userID)
	zip = server.getUserZip(userID)
	if zip != "90210" {
		t.Errorf("Expected '90210', got %q", zip)
	}

	// Address without zip
	server.db.Exec("UPDATE users SET address = ? WHERE id = ?", "Some Place, California", userID)
	zip = server.getUserZip(userID)
	if zip != "" {
		t.Errorf("Expected empty zip for address without 5-digit number, got %q", zip)
	}
}

// --- ReferralRedeem ---

func TestReferralRedeem_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleReferralRedeem, "GET", "/api/referral/redeem", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestReferralRedeem_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleReferralRedeem, "POST", "/api/referral/redeem", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// =============================================================================
// COVERAGE BOOST: handleLeaveHousehold (56.5% → target 85%+)
// =============================================================================

func TestCov3_LeaveHousehold_ResponseBodyContainsSuccess(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558880001")

	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("Expected success=true in response, got %v", body["success"])
	}
}

func TestCov3_LeaveHousehold_MultiUserHousehold_LeaverGetsNewHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5558880002")
	cookie2, userID2 := loginUser(t, server, "5558880003")

	// Put user2 into user1's household via invite flow
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite", map[string]interface{}{"phone": ""}, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("invite: got %d", rr.Code)
	}
	link := parseJSON(rr)["link"].(string)
	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	code := parts[len(parts)-1]

	doRequest(server.handleJoin, "POST", "/api/join", map[string]interface{}{"code": code}, []*http.Cookie{cookie2})

	// Both should be in the same household now
	var hid1, hid2 int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hid1)
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&hid2)
	if hid1 != hid2 {
		t.Fatalf("Setup: users should share household, got %d vs %d", hid1, hid2)
	}
	sharedHID := hid1

	// User2 leaves
	rr = doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("leave: got %d, body=%s", rr.Code, rr.Body.String())
	}

	// User2 should have a different household
	var newHID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID2).Scan(&newHID)
	if newHID == sharedHID {
		t.Error("User2 should have a new household after leaving")
	}
	if newHID == 0 {
		t.Error("User2 should have a valid household_id")
	}

	// User1 should still be in the original household
	var user1HID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&user1HID)
	if user1HID != sharedHID {
		t.Errorf("User1 should still be in original household %d, got %d", sharedHID, user1HID)
	}

	// User2 should be the owner of their new household
	var role string
	server.db.QueryRow("SELECT role FROM household_members WHERE user_id = ? AND household_id = ?", userID2, newHID).Scan(&role)
	if role != "owner" {
		t.Errorf("User2 should be owner of new household, got %q", role)
	}

	// User2 should have exactly 1 household_members entry (new one only)
	var memberCount int
	server.db.QueryRow("SELECT COUNT(*) FROM household_members WHERE user_id = ?", userID2).Scan(&memberCount)
	if memberCount != 1 {
		t.Errorf("User2 should have exactly 1 household_members entry, got %d", memberCount)
	}
}

func TestCov3_LeaveHousehold_PUTMethodRejected(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558880004")

	rr := doRequest(server.handleLeaveHousehold, "PUT", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT should return 405, got %d", rr.Code)
	}
}

func TestCov3_LeaveHousehold_DELETEMethodRejected(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558880005")

	rr := doRequest(server.handleLeaveHousehold, "DELETE", "/api/settings/leave-household", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE should return 405, got %d", rr.Code)
	}
}

func TestCov3_LeaveHousehold_InvalidSessionToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	fakeCookie := &http.Cookie{Name: "session", Value: "invalid_token_12345"}
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{fakeCookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Invalid session should return 401, got %d", rr.Code)
	}
}

func TestCov3_LeaveHousehold_ExpiredSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5558880006")

	// Create an expired session
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('expired_leave_tok', ?, '5558880006', 'user', datetime('now', '-2 days'), datetime('now', '-1 day'))",
		userID,
	)

	expiredCookie := &http.Cookie{Name: "session", Value: "expired_leave_tok"}
	rr := doRequest(server.handleLeaveHousehold, "POST", "/api/settings/leave-household", nil, []*http.Cookie{expiredCookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expired session should return 401, got %d", rr.Code)
	}
}

// =============================================================================
// COVERAGE BOOST: handleSavedListUpdate (63% → target 85%+)
// =============================================================================

func TestCov3_SavedListUpdate_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/1", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov3_SavedListUpdate_NonexistentID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558890001")

	// Add items to the active list so "no items" is not the issue
	createListItem(t, server.db, userID, "Apples", "Produce")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Nonexistent saved list should return 404, got %d", rr.Code)
	}
}

func TestCov3_SavedListUpdate_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558890002")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid ID should return 400, got %d", rr.Code)
	}
}

func TestCov3_SavedListUpdate_EmptyActiveList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558890003")

	// Create a saved list with items
	createListItem(t, server.db, userID, "Bananas", "Produce")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Test List"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create saved list: got %d", rr.Code)
	}
	savedListID := int(parseJSON(rr)["id"].(float64))

	// Clear the active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)

	// Try to update — should fail because active list is empty
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedListID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Update with empty active list should return 400, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "No items to save" {
		t.Errorf("Error message: got %q, want 'No items to save'", body["error"])
	}
}

func TestCov3_SavedListUpdate_ReplacesOldItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558890004")

	// Create a saved list with initial items
	createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")
	createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Dairy Essentials"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create: got %d", rr.Code)
	}
	savedListID := int(parseJSON(rr)["id"].(float64))

	// Verify initial item count
	var initialCount int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedListID).Scan(&initialCount)
	if initialCount != 2 {
		t.Fatalf("Expected 2 initial saved items, got %d", initialCount)
	}

	// Clear and add different items to active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)
	createListItem(t, server.db, userID, "Bread", "Bakery")
	createListItem(t, server.db, userID, "Butter", "Dairy & Eggs")
	createListItem(t, server.db, userID, "Jam", "Condiments")

	// Update the saved list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedListID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Update: got %d, body=%s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["item_count"] != float64(3) {
		t.Errorf("Expected item_count=3, got %v", body["item_count"])
	}

	// Verify old items were replaced
	var newCount int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedListID).Scan(&newCount)
	if newCount != 3 {
		t.Errorf("Expected 3 saved items after update, got %d", newCount)
	}

	// Verify the old items are gone and new ones are present
	var milkCount int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ? AND name = 'Milk'", savedListID).Scan(&milkCount)
	if milkCount != 0 {
		t.Error("Old item 'Milk' should be removed after update")
	}

	var breadCount int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ? AND name = 'Bread'", savedListID).Scan(&breadCount)
	if breadCount != 1 {
		t.Error("New item 'Bread' should be present after update")
	}
}

func TestCov3_SavedListUpdate_OtherUsersListBlocked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5558890005")
	cookie2, userID2 := loginUser(t, server, "5558890006")

	// User1 creates a saved list
	createListItem(t, server.db, userID1, "Apples", "Produce")
	res, err := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'User1 List')", userID1)
	if err != nil {
		t.Fatalf("insert saved_list: %v", err)
	}
	savedListID, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Apples', 'Produce', 1)", savedListID)

	// User2 adds items to their active list
	createListItem(t, server.db, userID2, "Oranges", "Produce")

	// User2 tries to update User1's saved list
	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedListID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Cross-user update should return 404, got %d", rr.Code)
	}
}

func TestCov3_SavedListUpdate_ResponseShape(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558890007")

	// Create saved list
	createListItem(t, server.db, userID, "Rice", "Grains")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Staples"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create: got %d", rr.Code)
	}
	savedListID := int(parseJSON(rr)["id"].(float64))

	// Update
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedListID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Update: got %d, body=%s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["id"] != float64(savedListID) {
		t.Errorf("Response should contain id=%d, got %v", savedListID, body["id"])
	}
	if body["item_count"] == nil {
		t.Error("Response should contain item_count")
	}
}

func TestCov3_SavedListUpdate_InvalidSessionToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	fakeCookie := &http.Cookie{Name: "session", Value: "bogus_token_xyz"}
	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/1", nil, []*http.Cookie{fakeCookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Invalid session should return 401, got %d", rr.Code)
	}
}

// =============================================================================
// COVERAGE BOOST: handleListClaim (73.9% → target 85%+)
// =============================================================================

func TestCov3_ListClaim_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleList, "PATCH", "/api/list/1/claim", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("No session should return 401, got %d", rr.Code)
	}
}

func TestCov3_ListClaim_InvalidSessionToken(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	fakeCookie := &http.Cookie{Name: "session", Value: "not_a_real_token"}
	rr := doRequest(server.handleList, "PATCH", "/api/list/1/claim", nil, []*http.Cookie{fakeCookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Invalid session should return 401, got %d", rr.Code)
	}
}

func TestCov3_ListClaim_ClaimAndVerifyModificationRecord(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558900001")

	itemID := createListItem(t, server.db, userID, "Cheese", "Dairy & Eggs")

	// Claim
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Claim: got %d", rr.Code)
	}

	// Verify modification recorded
	var action string
	err := server.db.QueryRow("SELECT action FROM item_modifications WHERE item_id = ? AND user_id = ? ORDER BY created_at DESC LIMIT 1", itemID, userID).Scan(&action)
	if err != nil {
		t.Fatalf("Query modification: %v", err)
	}
	if action != "claimed" {
		t.Errorf("Expected modification action 'claimed', got %q", action)
	}
}

func TestCov3_ListClaim_UnclaimVerifyModificationRecord(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558900002")

	itemID := createListItem(t, server.db, userID, "Yogurt", "Dairy & Eggs")

	// Claim then unclaim
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Unclaim: got %d", rr.Code)
	}

	// Verify unclaimed modification recorded (use id DESC since created_at may tie)
	var action string
	server.db.QueryRow("SELECT action FROM item_modifications WHERE item_id = ? AND user_id = ? ORDER BY id DESC LIMIT 1", itemID, userID).Scan(&action)
	if action != "unclaimed" {
		t.Errorf("Expected modification action 'unclaimed', got %q", action)
	}
}

func TestCov3_ListClaim_ClaimResponseContainsClaimedByID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558900003")

	itemID := createListItem(t, server.db, userID, "Butter", "Dairy & Eggs")

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Claim: got %d", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil {
		t.Fatal("ClaimedByID should not be nil after claim")
	}
	if *item.ClaimedByID != userID {
		t.Errorf("ClaimedByID: got %s, want %s", *item.ClaimedByID, userID)
	}
	if item.ID != int(itemID) {
		t.Errorf("Item ID: got %d, want %d", item.ID, int(itemID))
	}
}

func TestCov3_ListClaim_UnclaimResponseClaimedByIDNil(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558900004")

	itemID := createListItem(t, server.db, userID, "Cream", "Dairy & Eggs")

	// Claim then unclaim
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Unclaim: got %d", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID != nil {
		t.Errorf("ClaimedByID should be nil after unclaim, got %v", *item.ClaimedByID)
	}
}

func TestCov3_ListClaim_NonexistentItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558900005")

	rr := doRequest(server.handleList, "PATCH", "/api/list/999999/claim", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Nonexistent item should return 404, got %d", rr.Code)
	}
}

func TestCov3_ListClaim_CrossUserItemBlocked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5558900006")
	cookie2, _ := loginUser(t, server, "5558900007")

	// User1's item
	itemID := createListItem(t, server.db, userID1, "Bananas", "Produce")

	// User2 tries to claim User1's item (different household)
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Cross-user claim should return 404, got %d", rr.Code)
	}
}

func TestCov3_ListClaim_OverrideRecordsClaimedModification(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a household with two users
	cookie1, userID1 := loginUser(t, server, "5558900008")
	cookie2, userID2 := loginUser(t, server, "5558900009")

	// Put them in the same household
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite", map[string]interface{}{"phone": ""}, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("invite: %d", rr.Code)
	}
	link := parseJSON(rr)["link"].(string)
	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	code := parts[len(parts)-1]
	doRequest(server.handleJoin, "POST", "/api/join", map[string]interface{}{"code": code}, []*http.Cookie{cookie2})

	// User1 creates an item and claims it
	itemID := createListItem(t, server.db, userID1, "Avocados", "Produce")
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie1})

	// User2 overrides the claim
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("Override claim: got %d", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil || *item.ClaimedByID != userID2 {
		t.Errorf("Override claim: expected claimed_by=%s, got %v", userID2, item.ClaimedByID)
	}

	// Verify modification records — should have "claimed" entries for both users
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE item_id = ? AND action = 'claimed'", itemID).Scan(&count)
	if count < 2 {
		t.Errorf("Expected at least 2 'claimed' modification records, got %d", count)
	}
}

// =============================================================================
// COVERAGE BOOST: CleanExpired (50% → target 85%+)
// =============================================================================

func TestCov3_CleanExpired_NoSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Delete all sessions so the table is empty
	server.db.Exec("DELETE FROM sessions")

	// Should not panic or error
	server.sessions.CleanExpired()

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 sessions after clean, got %d", count)
	}
}

func TestCov3_CleanExpired_MultipleExpiredSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5558910001")

	// Insert multiple expired sessions
	for i := 0; i < 5; i++ {
		server.db.Exec(
			"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES (?, ?, '5558910001', 'user', datetime('now', '-3 days'), datetime('now', '-1 day'))",
			fmt.Sprintf("expired_multi_%d", i), userID,
		)
	}

	var before int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&before)
	if before < 6 {
		t.Fatalf("Expected at least 6 sessions (1 valid + 5 expired), got %d", before)
	}

	server.sessions.CleanExpired()

	// Only the valid login session should remain
	var after int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&after)
	if after != 1 {
		t.Errorf("Expected 1 valid session after cleanup, got %d", after)
	}
}

func TestCov3_CleanExpired_MixedUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5558910002")
	_, userID2 := loginUser(t, server, "5558910003")

	// Add expired sessions for both users
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('exp_u1', ?, '5558910002', 'user', datetime('now', '-2 days'), datetime('now', '-1 day'))",
		userID1,
	)
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('exp_u2', ?, '5558910003', 'user', datetime('now', '-2 days'), datetime('now', '-1 day'))",
		userID2,
	)

	server.sessions.CleanExpired()

	// Both users should still have their valid sessions
	var count1, count2 int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID1).Scan(&count1)
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID2).Scan(&count2)
	if count1 != 1 {
		t.Errorf("User1 should have 1 valid session, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("User2 should have 1 valid session, got %d", count2)
	}
}

func TestCov3_CleanExpired_JustExpiredSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5558910004")

	// Insert a session that expired 1 second ago (boundary case)
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('just_expired', ?, '5558910004', 'user', datetime('now', '-31 days'), datetime('now', '-1 second'))",
		userID,
	)

	server.sessions.CleanExpired()

	// The just-expired session should be cleaned
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = 'just_expired'").Scan(&count)
	if count != 0 {
		t.Error("Just-expired session should be cleaned up")
	}
}

func TestCov3_CleanExpired_DoesNotRemoveFutureSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5558910005")

	// Insert a session that expires far in the future
	server.db.Exec(
		"INSERT INTO sessions (token, user_id, phone, role, created_at, expires_at) VALUES ('future_tok', ?, '5558910005', 'user', datetime('now'), datetime('now', '+60 days'))",
		userID,
	)

	server.sessions.CleanExpired()

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = 'future_tok'").Scan(&count)
	if count != 1 {
		t.Error("Future session should not be cleaned up")
	}
}

// =============================================================================
// COVERAGE BOOST: DeleteUserSessions (57.1% → target 85%+)
// =============================================================================

func TestCov3_DeleteUserSessions_NoSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a user without any sessions
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-9999', '5558920001')")

	// Should not panic or error
	server.sessions.DeleteUserSessions("user-9999")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = 'user-9999'").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 sessions, got %d", count)
	}
}

func TestCov3_DeleteUserSessions_CacheInvalidated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558920002")

	// Verify session works before deletion
	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Session should work before delete: got %d", rr.Code)
	}

	// Delete all sessions
	server.sessions.DeleteUserSessions(userID)

	// Session should no longer work (cache should be cleared too)
	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Session should be invalid after delete: got %d, want 401", rr.Code)
	}
}

func TestCov3_DeleteUserSessions_MultipleSessions(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5558920003")

	// Create additional sessions (different auth methods)
	server.sessions.CreateSession(userID, "5558920003", "phone")
	server.sessions.CreateSession(userID, "5558920003", "email")
	server.sessions.CreateSession(userID, "5558920003", "google")

	var before int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&before)
	if before < 4 {
		t.Fatalf("Expected at least 4 sessions, got %d", before)
	}

	server.sessions.DeleteUserSessions(userID)

	var after int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID).Scan(&after)
	if after != 0 {
		t.Errorf("Expected 0 sessions after delete, got %d", after)
	}
}

func TestCov3_DeleteUserSessions_PreservesOtherUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5558920004")
	cookie2, userID2 := loginUser(t, server, "5558920005")

	// Create extra sessions for both
	server.sessions.CreateSession(userID1, "5558920004", "phone")
	server.sessions.CreateSession(userID2, "5558920005", "phone")

	// Delete user1's sessions
	server.sessions.DeleteUserSessions(userID1)

	// User1's sessions should be gone
	var count1 int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID1).Scan(&count1)
	if count1 != 0 {
		t.Errorf("User1 should have 0 sessions, got %d", count1)
	}

	// User2's sessions should be intact
	var count2 int
	server.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = ?", userID2).Scan(&count2)
	if count2 < 2 {
		t.Errorf("User2 should still have sessions, got %d", count2)
	}

	// User2's session should still work
	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("User2 session should still work: got %d", rr.Code)
	}

	// User1's session should be invalid
	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("User1 session should be invalid: got %d", rr.Code)
	}
}

func TestCov3_DeleteUserSessions_NonexistentUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Should not panic or error for a user that doesn't exist
	server.sessions.DeleteUserSessions("user-999999")
}

// =============================================================================
// COVERAGE BOOST: handleBrandResolve (71.9% → target 85%+)
// =============================================================================

func TestCov3_BrandResolve_WrongMethod_GET(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930001")

	rr := doRequest(server.handleBrandResolve, "GET", "/api/list/brand-resolve", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_WrongMethod_PUT(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930002")

	rr := doRequest(server.handleBrandResolve, "PUT", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 1, "brand": "Fage"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT should return 405, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_WrongMethod_DELETE(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930003")

	rr := doRequest(server.handleBrandResolve, "DELETE", "/api/list/brand-resolve", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE should return 405, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_NonexistentItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930004")

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 999999, "brand": "Horizon"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Nonexistent item should return 404, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_EmptyBrand(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930005")

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 1, "brand": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Empty brand should return 400, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_ZeroItemID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930006")

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 0, "brand": "Horizon"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Zero item_id should return 400, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_WithoutProductCategory(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558930007")

	itemID := createListItem(t, server.db, userID, "Whole Milk", "Dairy & Eggs")

	// Resolve without product_category — should derive from item name
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Organic Valley"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Brand resolve without category: got %d, body=%s", rr.Code, rr.Body.String())
	}

	// Verify brand was set
	var brand string
	server.db.QueryRow("SELECT COALESCE(brand,'') FROM list_items WHERE id = ?", itemID).Scan(&brand)
	if brand != "Organic Valley" {
		t.Errorf("Expected brand 'Organic Valley', got %q", brand)
	}

	// Verify brand choice was recorded with item name as category
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_choices WHERE user_id = ? AND product_category = 'whole milk' AND brand = 'Organic Valley'", userID).Scan(&count)
	if count == 0 {
		t.Error("Brand choice should be recorded with item name as category when product_category not provided")
	}
}

func TestCov3_BrandResolve_ModificationRecorded(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558930008")

	itemID := createListItem(t, server.db, userID, "Greek Yogurt", "Dairy & Eggs")

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Chobani", "product_category": "yogurt"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Brand resolve: got %d", rr.Code)
	}

	// Verify modification recorded
	var detail string
	err := server.db.QueryRow(
		"SELECT detail FROM item_modifications WHERE item_id = ? AND user_id = ? AND action = 'updated' ORDER BY created_at DESC LIMIT 1",
		itemID, userID,
	).Scan(&detail)
	if err != nil {
		t.Fatalf("Query modification: %v", err)
	}
	if !strings.Contains(detail, "Chobani") {
		t.Errorf("Modification detail should mention Chobani, got %q", detail)
	}
}

func TestCov3_BrandResolve_InvalidSession(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	fakeCookie := &http.Cookie{Name: "session", Value: "totally_fake_session"}
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 1, "brand": "Fage"},
		[]*http.Cookie{fakeCookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Invalid session should return 401, got %d", rr.Code)
	}
}

func TestCov3_BrandResolve_UpdatesExistingBrand(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558930009")

	itemID := createListItem(t, server.db, userID, "Ketchup", "Condiments")

	// Set initial brand
	server.db.Exec("UPDATE list_items SET brand = 'Heinz' WHERE id = ?", itemID)

	// Resolve to a different brand
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Hunt's", "product_category": "ketchup"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Brand update: got %d", rr.Code)
	}

	// Verify brand was changed
	body := parseJSON(rr)
	if body["brand"] != "Hunt's" {
		t.Errorf("Expected brand 'Hunt's', got %v", body["brand"])
	}

	// Verify in DB
	var dbBrand string
	server.db.QueryRow("SELECT COALESCE(brand,'') FROM list_items WHERE id = ?", itemID).Scan(&dbBrand)
	if dbBrand != "Hunt's" {
		t.Errorf("DB brand: got %q, want 'Hunt's'", dbBrand)
	}
}

func TestCov3_BrandResolve_BrandPreferenceRecorded(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5558930010")

	itemID := createListItem(t, server.db, userID, "Pasta Sauce", "Condiments")

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Rao's", "product_category": "pasta sauce"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Brand resolve: got %d", rr.Code)
	}

	// Verify brand_choices entry
	var choiceBrand string
	err := server.db.QueryRow("SELECT brand FROM brand_choices WHERE user_id = ? AND product_category = 'pasta sauce'", userID).Scan(&choiceBrand)
	if err != nil {
		t.Fatalf("brand_choices query: %v", err)
	}
	if choiceBrand != "Rao's" {
		t.Errorf("Expected brand choice 'Rao's', got %q", choiceBrand)
	}

	// Verify brand_preferences entry
	var prefBrand string
	err = server.db.QueryRow("SELECT brand FROM brand_preferences WHERE user_id = ? AND product_category = 'pasta sauce'", userID).Scan(&prefBrand)
	if err != nil {
		t.Fatalf("brand_preferences query: %v", err)
	}
	if prefBrand != "Rao's" {
		t.Errorf("Expected brand preference 'Rao's', got %q", prefBrand)
	}
}

func TestCov3_BrandResolve_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5558930011")

	req := httptest.NewRequest("POST", "/api/list/brand-resolve", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleBrandResolve(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON should return 400, got %d", rr.Code)
	}
}
