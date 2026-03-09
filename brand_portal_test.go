package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"
)


func TestBrandPortal_RegisterAndLogin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "brand@example.com",
		"password":     "securepassword123",
		"company_name": "Acme Brands",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Register: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	if body["brand_user_id"] == nil || body["brand_user_id"].(float64) == 0 {
		t.Error("expected non-zero brand_user_id")
	}

	// Verify brand_session cookie was set
	var regCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "brand_session" {
			regCookie = c
			break
		}
	}
	if regCookie == nil {
		t.Fatal("no brand_session cookie set on register")
	}

	// Login with same credentials
	rr = doRequest(server.handleBrandLogin, "POST", "/brand/login", map[string]string{
		"email":    "brand@example.com",
		"password": "securepassword123",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Login: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body = parseJSON(rr)
	if body["success"] != true {
		t.Errorf("Login: expected success=true, got %v", body["success"])
	}

	// Login with wrong password should fail
	rr = doRequest(server.handleBrandLogin, "POST", "/brand/login", map[string]string{
		"email":    "brand@example.com",
		"password": "wrongpassword",
	}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Login bad password: expected 401, got %d", rr.Code)
	}
}

func TestBrandPortal_Me(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "me@brand.com", "password1234", "MyCo Inc")

	rr := doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Me: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if int(body["brand_user_id"].(float64)) != brandUserID {
		t.Errorf("brand_user_id: got %v, want %d", body["brand_user_id"], brandUserID)
	}
	if body["email"] != "me@brand.com" {
		t.Errorf("email: got %v, want me@brand.com", body["email"])
	}
	if body["company_name"] != "MyCo Inc" {
		t.Errorf("company_name: got %v, want MyCo Inc", body["company_name"])
	}
	if body["verified"] != true {
		t.Errorf("verified: got %v, want true", body["verified"])
	}
	// balance_cents should be 0 for a new user
	if body["balance_cents"].(float64) != 0 {
		t.Errorf("balance_cents: got %v, want 0", body["balance_cents"])
	}
}

func TestBrandPortal_Logout(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "logout@brand.com", "password1234", "LogoutCo")

	// Logout
	rr := doRequest(server.handleBrandLogout, "POST", "/brand/logout", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Logout: expected 200, got %d", rr.Code)
	}

	// /brand/me should now return 401
	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Me after logout: expected 401, got %d", rr.Code)
	}
}

func TestBrandPortal_DuplicateEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// First registration succeeds
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "dup@brand.com",
		"password":     "password1234",
		"company_name": "First Co",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("First register: expected 200, got %d", rr.Code)
	}

	// Second registration with same email should fail with 409
	rr = doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "dup@brand.com",
		"password":     "anotherpass123",
		"company_name": "Second Co",
	}, nil)
	if rr.Code != http.StatusConflict {
		t.Errorf("Duplicate register: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["error"] == nil || body["error"] == "" {
		t.Error("expected error message in duplicate register response")
	}
}

func TestBrandPortal_CreateDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "deals@brand.com", "password1234", "DealCo")

	// Create a deal
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product":     "Store Brand Oats 32oz",
		"offer_brand":            "TestOats",
		"offer_product":          "TestOats Organic 32oz",
		"discount_cents":         100,
		"daily_budget_cents":     20000,
		"offer_price_cents":      399,
		"competitor_price_cents": 299,
		"why_bullets":            `["Organic certified","Non-GMO"]`,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create deal: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	dealID := body["id"].(float64)
	if dealID == 0 {
		t.Error("expected non-zero deal ID")
	}

	// GET deals should return the created deal
	rr = doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Get deals: expected 200, got %d", rr.Code)
	}

	var deals []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if deals[0]["competitor_product"] != "Store Brand Oats 32oz" {
		t.Errorf("competitor_product: got %v, want 'Store Brand Oats 32oz'", deals[0]["competitor_product"])
	}
	if deals[0]["offer_brand"] != "TestOats" {
		t.Errorf("offer_brand: got %v, want 'TestOats'", deals[0]["offer_brand"])
	}
	if deals[0]["status"] != "active" {
		t.Errorf("status: got %v, want 'active'", deals[0]["status"])
	}

	// Create deal with missing required fields should fail
	rr = doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"offer_brand": "TestOats",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Create deal missing fields: expected 400, got %d", rr.Code)
	}
}

func TestBrandPortal_DealPauseResume(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "pause@brand.com", "password1234", "PauseCo")
	dealID := createTestDeal(t, server, cookie)

	// Pause the deal
	pauseStatus := "paused"
	rr := doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID),
		map[string]interface{}{"status": &pauseStatus},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Pause deal: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify status is paused
	rr = doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	var deals []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if deals[0]["status"] != "paused" {
		t.Errorf("status after pause: got %v, want 'paused'", deals[0]["status"])
	}

	// Resume the deal
	activeStatus := "active"
	rr = doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID),
		map[string]interface{}{"status": &activeStatus},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Resume deal: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify status is active again
	rr = doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if deals[0]["status"] != "active" {
		t.Errorf("status after resume: got %v, want 'active'", deals[0]["status"])
	}
}

func TestBrandPortal_DeleteDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "delete@brand.com", "password1234", "DeleteCo")
	dealID := createTestDeal(t, server, cookie)

	// Delete the deal
	rr := doRequest(server.handleBrandDeals, "DELETE", fmt.Sprintf("/brand/deals/%d", dealID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Delete deal: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// GET deals should no longer return the deleted deal (soft delete filters by status != 'deleted')
	rr = doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	var deals []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 0 {
		t.Errorf("expected 0 deals after delete, got %d", len(deals))
	}

	// Verify the deal still exists in the DB (soft delete, not hard delete)
	var status string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&status)
	if status != "deleted" {
		t.Errorf("expected deal status='deleted' in DB, got '%s'", status)
	}
}

func TestBrandPortal_Dashboard(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "dash@brand.com", "password1234", "DashCo")

	// Create 2 deals
	createTestDeal(t, server, cookie)
	createTestDeal(t, server, cookie)

	// Check dashboard
	rr := doRequest(server.handleBrandDashboard, "GET", "/brand/dashboard", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Dashboard: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["total_deals"].(float64) != 2 {
		t.Errorf("total_deals: got %v, want 2", body["total_deals"])
	}
	if body["active_deals"].(float64) != 2 {
		t.Errorf("active_deals: got %v, want 2", body["active_deals"])
	}
	if body["balance_cents"].(float64) != 0 {
		t.Errorf("balance_cents: got %v, want 0", body["balance_cents"])
	}
	if body["total_swaps"].(float64) != 0 {
		t.Errorf("total_swaps: got %v, want 0 (no swap events yet)", body["total_swaps"])
	}
	if body["accept_rate"].(float64) != 0 {
		t.Errorf("accept_rate: got %v, want 0 (no swaps)", body["accept_rate"])
	}
	// daily should be an array (possibly empty)
	if body["daily"] == nil {
		t.Error("expected 'daily' key in dashboard response")
	}
}

func TestBrandPortal_DashboardDaysFilter(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "filter@brand.com", "password1234", "FilterCo")

	// Create a deal owned by this brand user
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status, brand_user_id)
		VALUES ('old brand', 'FilterBrand', 'Product', 100, 1000, 'active', ?)`, brandUserID)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE brand_user_id = ?", brandUserID).Scan(&dealID)

	// Insert swap events: 1 recent (today) and 1 old (60 days ago)
	server.db.Exec("INSERT INTO users (id, phone_number) VALUES (999, '5559999999')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name) VALUES (999, 999, 'Test Item')")
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action, created_at)
		VALUES (999, ?, 999, 'offered', datetime('now'))`, dealID)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action, created_at)
		VALUES (999, ?, 999, 'offered', datetime('now', '-60 days'))`, dealID)

	// Without filter: should see both
	rr := doRequest(server.handleBrandDashboard, "GET", "/brand/dashboard", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	if body["total_swaps"].(float64) != 2 {
		t.Errorf("no filter: expected 2 total_swaps, got %v", body["total_swaps"])
	}

	// With ?days=7: should see only the recent one
	req := httptest.NewRequest("GET", "/brand/dashboard?days=7", nil)
	req.AddCookie(cookie)
	rrFiltered := httptest.NewRecorder()
	server.handleBrandDashboard(rrFiltered, req)
	filteredBody := parseJSON(rrFiltered)
	if filteredBody["total_swaps"].(float64) != 1 {
		t.Errorf("days=7 filter: expected 1 total_swaps, got %v", filteredBody["total_swaps"])
	}
}

func TestBrandPortal_Deposit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "deposit@brand.com", "password1234", "DepositCo")

	// Deposit $100 (10000 cents)
	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]interface{}{
		"amount_cents": 10000,
		"stripe_token": "tok_test_abc123",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Deposit: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["amount_cents"].(float64) != 10000 {
		t.Errorf("amount_cents: got %v, want 10000", body["amount_cents"])
	}

	// Verify balance increased via /brand/me
	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["balance_cents"].(float64) != 10000 {
		t.Errorf("balance after deposit: got %v, want 10000", body["balance_cents"])
	}

	// Deposit another $50 (5000 cents) and verify cumulative
	doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]interface{}{
		"amount_cents": 5000,
		"stripe_token": "tok_test_def456",
	}, []*http.Cookie{cookie})

	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["balance_cents"].(float64) != 15000 {
		t.Errorf("balance after two deposits: got %v, want 15000", body["balance_cents"])
	}
}

func TestBrandPortal_DepositMinimum(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "mindepo@brand.com", "password1234", "MinDepoCo")

	// Try to deposit $49.99 (4999 cents) - below $50 minimum
	rr := doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]interface{}{
		"amount_cents": 4999,
		"stripe_token": "tok_test_small",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Deposit below minimum: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["error"] == nil || body["error"] == "" {
		t.Error("expected error message for below-minimum deposit")
	}

	// Verify balance is still 0
	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["balance_cents"].(float64) != 0 {
		t.Errorf("balance should be 0 after rejected deposit, got %v", body["balance_cents"])
	}

	// Deposit exactly $50 (5000 cents) should succeed
	rr = doRequest(server.handleBrandDeposit, "POST", "/brand/deposit", map[string]interface{}{
		"amount_cents": 5000,
		"stripe_token": "tok_test_exact",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Deposit at minimum: expected 200, got %d", rr.Code)
	}
}

func TestBrandPortal_StripeWebhookIdempotent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, brandUserID := brandRegister(t, server, "stripe@brand.com", "password1234", "StripeCo")

	// Build a payment_intent.succeeded webhook payload
	payload := fmt.Sprintf(`{
		"type": "payment_intent.succeeded",
		"data": {
			"object": {
				"id": "pi_test_idempotent_123",
				"amount": 25000,
				"metadata": {
					"brand_user_id": "%d"
				}
			}
		}
	}`, brandUserID)

	// First webhook delivery
	req1 := httptest.NewRequest("POST", "/brand/stripe-webhook", strings.NewReader(payload))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	server.handleStripeWebhook(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("First webhook: expected 200, got %d", rr1.Code)
	}

	// Check balance after first delivery
	var balance1 int
	server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&balance1)
	if balance1 != 25000 {
		t.Fatalf("Balance after first webhook: got %d, want 25000", balance1)
	}

	// Second webhook delivery (duplicate/retry) with same payment_intent ID
	req2 := httptest.NewRequest("POST", "/brand/stripe-webhook", strings.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	server.handleStripeWebhook(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("Second webhook: expected 200, got %d", rr2.Code)
	}

	// Balance should NOT have doubled
	var balance2 int
	server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&balance2)
	if balance2 != 25000 {
		t.Errorf("Balance after duplicate webhook: got %d, want 25000 (idempotency failed!)", balance2)
	}

	// Verify only one transaction record exists
	var txnCount int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_transactions WHERE stripe_id = 'pi_test_idempotent_123'").Scan(&txnCount)
	if txnCount != 1 {
		t.Errorf("Expected 1 transaction for this stripe_id, got %d", txnCount)
	}
}

func TestBrandPortal_CategoryIntel(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "intel@brand.com", "password1234", "IntelCo")

	// Create a deal with "cola" in the competitor product
	doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Generic Cola 12pk",
		"offer_brand":        "BrandCola",
		"offer_product":      "BrandCola Premium 12pk",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{cookie})

	// Query category intel for "cola"
	rr := doRequest(server.handleCategoryIntel, "GET", "/brand/category-intel/cola", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Category intel: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["category"] != "cola" {
		t.Errorf("category: got %v, want 'cola'", body["category"])
	}
	// total_swap_outs should be 0 (no swap events yet)
	if body["total_swap_outs"].(float64) != 0 {
		t.Errorf("total_swap_outs: got %v, want 0", body["total_swap_outs"])
	}
	// details should contain the deal we created
	details, ok := body["details"].([]interface{})
	if !ok {
		t.Fatal("expected 'details' to be an array")
	}
	if len(details) != 1 {
		t.Errorf("expected 1 detail row, got %d", len(details))
	}

	// Query empty category should still return valid response
	rr = doRequest(server.handleCategoryIntel, "GET", "/brand/category-intel/nonexistent", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Category intel (empty): expected 200, got %d", rr.Code)
	}
}

func TestBugReport_Submit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991234")

	rr := doRequest(server.handleBugReport, "POST", "/api/bug-report", map[string]string{
		"description": "The app crashes when I add more than 50 items to my list.",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Bug report: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["message"] == nil || body["message"] == "" {
		t.Error("expected message in bug report response")
	}

	// Verify it was saved in the database
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM bug_reports").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 bug report in DB, got %d", count)
	}
}

func TestBugReport_EmptyDescription(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991235")

	// Empty description
	rr := doRequest(server.handleBugReport, "POST", "/api/bug-report", map[string]string{
		"description": "",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Empty bug report: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// Whitespace-only description
	rr = doRequest(server.handleBugReport, "POST", "/api/bug-report", map[string]string{
		"description": "   \t\n  ",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Whitespace-only bug report: expected 400, got %d", rr.Code)
	}

	// No bug reports should have been saved
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM bug_reports").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 bug reports in DB after empty submissions, got %d", count)
	}
}

func TestBrandPortal_FoundingBadge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register the first 10 brand users — all should get "founding" badge
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("founding%d@brand.com", i)
		rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
			"email":        email,
			"password":     "password1234",
			"company_name": fmt.Sprintf("FoundingCo %d", i),
		}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("Register founding user %d: expected 200, got %d", i, rr.Code)
		}
		body := parseJSON(rr)
		if body["badge"] != "founding" {
			t.Errorf("User %d badge: got %v, want 'founding'", i, body["badge"])
		}
	}

	// The 11th user should NOT get the founding badge
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "notfounding@brand.com",
		"password":     "password1234",
		"company_name": "LateCo",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Register 11th user: expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["badge"] != "" {
		t.Errorf("11th user badge: got %v, want empty string", body["badge"])
	}

	// Verify via /brand/me that the badge persists
	var cookie11 *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "brand_session" {
			cookie11 = c
			break
		}
	}
	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie11})
	body = parseJSON(rr)
	if body["badge"] != "" {
		t.Errorf("11th user badge via /brand/me: got %v, want empty", body["badge"])
	}
}

// =============================================================================
// RECEIPT ENDPOINT TESTS
// =============================================================================

func TestEarningsGet_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991111")

	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	txns := body["transactions"].([]interface{})
	if len(txns) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(txns))
	}
	if body["total_cents"].(float64) != 0 {
		t.Errorf("expected total_cents=0, got %v", body["total_cents"])
	}
	if body["display_amount"] != "$0.00" {
		t.Errorf("expected display_amount=$0.00, got %v", body["display_amount"])
	}
}

func TestEarningsGet_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _, _, _ := setupEarningsTestData(t, server)

	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	txns := body["transactions"].([]interface{})
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	tx := txns[0].(map[string]interface{})
	if tx["offer_brand"] != "Hunts" {
		t.Errorf("offer_brand: got %v, want Hunts", tx["offer_brand"])
	}
	if tx["amount_cents"].(float64) != 150 {
		t.Errorf("amount_cents: got %v, want 150", tx["amount_cents"])
	}
	if body["total_cents"].(float64) != 150 {
		t.Errorf("total_cents: got %v, want 150", body["total_cents"])
	}
	if body["pending_cents"].(float64) != 150 {
		t.Errorf("pending_cents: got %v, want 150", body["pending_cents"])
	}
	if body["display_amount"] != "$1.50" {
		t.Errorf("display_amount: got %v, want $1.50", body["display_amount"])
	}
}

func TestEarningsGet_FilterByDays(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID, _, _ := setupEarningsTestData(t, server)

	// Insert an old rebate (100 days ago)
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES ('Pepsi', 'Coca-Cola', 'Classic Coke', 200, 5000, 'active')`)
	var dealID2 int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'Pepsi'").Scan(&dealID2)
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, created_at) VALUES (?, ?, NULL, 'accept', datetime('now', '-100 days'))", userID, dealID2)
	var seID2 int
	server.db.QueryRow("SELECT id FROM swap_events WHERE deal_id = ?", dealID2).Scan(&seID2)
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status, created_at) VALUES (?, ?, ?, 200, 'paid', datetime('now', '-100 days'))", userID, seID2, dealID2)

	// All: should see 2
	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	txns := body["transactions"].([]interface{})
	if len(txns) != 2 {
		t.Errorf("all: expected 2 transactions, got %d", len(txns))
	}

	// 7 days: should see 1
	req := httptest.NewRequest("GET", "/api/earnings?days=7", nil)
	req.AddCookie(cookie)
	rr2 := httptest.NewRecorder()
	server.handleEarnings(rr2, req)
	body2 := parseJSON(rr2)
	txns2 := body2["transactions"].([]interface{})
	if len(txns2) != 1 {
		t.Errorf("7d filter: expected 1 transaction, got %d", len(txns2))
	}
}

func TestEarningsGet_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestEarningsGet_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991112")

	rr := doRequest(server.handleEarnings, "POST", "/api/earnings", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestEarningsUpload_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	// Build multipart request with explicit Content-Type
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="receipt.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, _ := w.CreatePart(h)
	part.Write([]byte("fake-jpeg-data"))
	w.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/earnings/%d/upload", rebateID), body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["success"] != true {
		t.Errorf("expected success=true, got %v", result["success"])
	}

	// Verify attachment_path was saved
	var path string
	server.db.QueryRow("SELECT attachment_path FROM swap_rebates WHERE id = ?", rebateID).Scan(&path)
	if path == "" {
		t.Error("attachment_path should not be empty after upload")
	}

	// Cleanup uploaded file
	os.Remove(path)
	os.Remove("data/earnings-attachments")
	os.Remove("data")
}

func TestEarningsUpload_WrongUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, _, _, rebateID := setupEarningsTestData(t, server)

	// Login as a different user
	otherCookie, _ := loginUser(t, server, "5559992222")

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "receipt.jpg")
	part.Write([]byte("fake-jpeg-data"))
	w.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/earnings/%d/upload", rebateID), body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(otherCookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEarningsUpload_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "receipt.jpg")
	part.Write([]byte("fake-jpeg-data"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/earnings/1/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestEarningsUpload_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991113")

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "receipt.jpg")
	part.Write([]byte("fake-jpeg-data"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/earnings/abc/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEarningsUpload_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559991114")

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "receipt.jpg")
	part.Write([]byte("fake-jpeg-data"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/earnings/99999/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEarningsUpload_InvalidType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="malware.exe"`)
	h.Set("Content-Type", "application/x-executable")
	part, _ := w.CreatePart(h)
	part.Write([]byte("bad-data"))
	w.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/earnings/%d/upload", rebateID), body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEarningsUpload_NoFile(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	w.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/earnings/%d/upload", rebateID), body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEarningsGet_TransactionDetail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _, _, _ := setupEarningsTestData(t, server)

	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, []*http.Cookie{cookie})
	body := parseJSON(rr)
	txns := body["transactions"].([]interface{})
	tx := txns[0].(map[string]interface{})

	// Verify all expected fields are present
	expectedFields := []string{"id", "offer_brand", "offer_product", "competitor_product",
		"original_item_name", "amount_cents", "status", "created_at", "attachment_path"}
	for _, field := range expectedFields {
		if _, ok := tx[field]; !ok {
			t.Errorf("missing field: %s", field)
		}
	}

	if tx["competitor_product"] != "Heinz Ketchup" {
		t.Errorf("competitor_product: got %v, want 'Heinz Ketchup'", tx["competitor_product"])
	}
	if tx["original_item_name"] != "ketchup" {
		t.Errorf("original_item_name: got %v, want 'ketchup'", tx["original_item_name"])
	}
}

// =============================================================================
// GOOGLE AUTH PHOTO AUTO-POPULATION TESTS
// =============================================================================

// newMockGoogleTokenServer creates a mock Google tokeninfo endpoint that returns
// the provided tokenInfo JSON for any request.


// =============================================================================
// BRAND DIGEST TESTS
// =============================================================================

func TestBrandDigest_NoPendingDigests(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// No brand users — should not panic
	server.sendPendingBrandDigests()
}

func TestBrandDigest_OptedOut(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.SendGridAPIKey = "test-key"

	// Create a brand user who opted out
	server.db.Exec(`INSERT INTO brand_users (id, email, password_hash, company_name, digest_opt_out)
		VALUES (1, 'brand@test.com', '$2a$10$test', 'Test Brand', 1)`)

	server.sendPendingBrandDigests()

	// Verify no digest was sent (last_digest_at should still be null)
	var lastDigest *string
	server.db.QueryRow("SELECT last_digest_at FROM brand_users WHERE id = 1").Scan(&lastDigest)
	if lastDigest != nil {
		t.Errorf("Expected nil last_digest_at for opted-out user, got %v", *lastDigest)
	}
}

func TestBrandDigest_NoSendGridKey(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.SendGridAPIKey = "" // no API key

	server.db.Exec(`INSERT INTO brand_users (id, email, password_hash, company_name)
		VALUES (1, 'brand@test.com', '$2a$10$test', 'Test Brand')`)

	// Should silently skip
	server.sendPendingBrandDigests()

	var lastDigest *string
	server.db.QueryRow("SELECT last_digest_at FROM brand_users WHERE id = 1").Scan(&lastDigest)
	if lastDigest != nil {
		t.Errorf("Expected nil last_digest_at when no SendGrid key, got %v", *lastDigest)
	}
}

func TestBrandDigest_HTMLGeneration(t *testing.T) {
	data := brandDigestData{
		CompanyName:        "Hunts",
		WeekLabel:          "Mar 1 – Mar 8, 2026",
		TotalOffered:       50,
		TotalAccepted:      20,
		AcceptRate:         40,
		SpentCentsThisWeek: 4000,
		VerifiedPurchases:  10,
		PendingClaims:      5,
		BalanceCents:       100000,
		ActiveDeals:        3,
		DashboardURL:       "https://brands.thrift.ly",
	}

	html := buildBrandDigestHTML(data)

	if !strings.Contains(html, "Hunts") {
		t.Error("HTML should contain company name")
	}
	if !strings.Contains(html, "Mar 1") {
		t.Error("HTML should contain week label")
	}
	if !strings.Contains(html, "50") {
		t.Error("HTML should contain total offered count")
	}
	if !strings.Contains(html, "40%") {
		t.Error("HTML should contain accept rate")
	}
	if !strings.Contains(html, "$40.00") {
		t.Error("HTML should contain spent amount")
	}
	if !strings.Contains(html, "$1000.00") {
		t.Error("HTML should contain balance")
	}
	if !strings.Contains(html, "brands.thrift.ly") {
		t.Error("HTML should contain dashboard URL")
	}
}

func TestBrandDigest_RecentDigestSkipped(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.SendGridAPIKey = "test-key"

	// Create a brand user with recent digest (2 days ago)
	server.db.Exec(`INSERT INTO brand_users (id, email, password_hash, company_name, last_digest_at)
		VALUES (1, 'brand@test.com', '$2a$10$test', 'Test Brand', datetime('now', '-2 days'))`)

	server.sendPendingBrandDigests()

	// The last_digest_at should NOT have been updated (still ~2 days ago, not now)
	var lastDigest string
	server.db.QueryRow("SELECT last_digest_at FROM brand_users WHERE id = 1").Scan(&lastDigest)
	// Parse and check — it should be more than 1 day old
	parsed, _ := time.Parse("2006-01-02 15:04:05", lastDigest)
	if time.Since(parsed) < 1*24*time.Hour {
		t.Errorf("Expected old last_digest_at (recent digest should be skipped), got %s", lastDigest)
	}
}

func TestBrandUnsubscribe_RequiresLogin(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandUnsubscribe, "GET", "/brand/unsubscribe", nil, nil)
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200 (login prompt)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "log in") {
		t.Error("Should contain login prompt")
	}
}

// =============================================================================
// LEARNING INFRASTRUCTURE TESTS
// =============================================================================

func TestDealPerformance_Unauthenticated(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// No session -> 401
	rr := doRequest(server.handleBrandDealPerformance, "GET", "/brand/deal-performance/1", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without brand session, got %d", rr.Code)
	}
}

func TestDealPerformance_WithBrandAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register a brand
	brandCookie, brandUserID := brandRegister(t, server, "perf@test.com", "password123", "PerfCo")

	// Create a deal owned by this brand
	dealID := createTestDeal(t, server, brandCookie)

	// Update deal to have the correct brand_user_id (createTestDeal may or may not set it)
	server.db.Exec("UPDATE brand_deals SET brand_user_id = ? WHERE id = ?", brandUserID, dealID)

	// Insert deal performance data
	today := time.Now().Format("2006-01-02")
	server.db.Exec(`INSERT INTO deal_performance_daily
		(deal_id, date, impressions, accepts, declines, revenue_cents, verified_count)
		VALUES (?, ?, 50, 20, 10, 1500, 5)`, dealID, today)

	rr := doRequest(server.handleBrandDealPerformance, "GET",
		fmt.Sprintf("/brand/deal-performance/%d", dealID), nil, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if int(body["deal_id"].(float64)) != dealID {
		t.Errorf("Expected deal_id=%d, got %v", dealID, body["deal_id"])
	}

	perf, ok := body["performance"].([]interface{})
	if !ok {
		t.Fatal("Expected performance to be an array")
	}
	if len(perf) != 1 {
		t.Fatalf("Expected 1 performance row, got %d", len(perf))
	}

	row := perf[0].(map[string]interface{})
	if row["impressions"].(float64) != 50 {
		t.Errorf("Expected impressions=50, got %v", row["impressions"])
	}
	if row["accepts"].(float64) != 20 {
		t.Errorf("Expected accepts=20, got %v", row["accepts"])
	}
	if row["revenue_cents"].(float64) != 1500 {
		t.Errorf("Expected revenue_cents=1500, got %v", row["revenue_cents"])
	}
}

func TestDealPerformance_WrongBrand(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register two brands
	_, brandUserID1 := brandRegister(t, server, "brand1@test.com", "password123", "Brand1")
	brandCookie2, _ := brandRegister(t, server, "brand2@test.com", "password123", "Brand2")

	// Create a deal owned by brand 1
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status, brand_user_id)
		VALUES (99, 'competitor', 'Brand1', 'Product', 100, 5000, 'active', ?)`, brandUserID1)

	// Brand 2 tries to access brand 1's deal -> 403
	rr := doRequest(server.handleBrandDealPerformance, "GET",
		"/brand/deal-performance/99", nil, []*http.Cookie{brandCookie2})
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 when accessing another brand's deal, got %d", rr.Code)
	}
}

func TestDealPerformance_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	brandCookie, _ := brandRegister(t, server, "test@brand.com", "password123", "TestBrand")

	// Non-numeric ID -> 400
	rr := doRequest(server.handleBrandDealPerformance, "GET",
		"/brand/deal-performance/abc", nil, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-numeric ID, got %d", rr.Code)
	}

	// Non-existent deal -> 404
	rr = doRequest(server.handleBrandDealPerformance, "GET",
		"/brand/deal-performance/99999", nil, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent deal, got %d", rr.Code)
	}
}

// =============================================================================
// CHURN PREDICTION + COHORT TESTS
// =============================================================================

// --- Category-Tiered Pricing Tests ---

func TestCategoryPricingTiers_TableSeeded(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	var count int
	db.QueryRow("SELECT COUNT(*) FROM category_pricing_tiers").Scan(&count)
	if count < 10 {
		t.Errorf("expected at least 10 seeded pricing tiers, got %d", count)
	}

	// Verify premium tier
	var minCents int
	var tier string
	db.QueryRow("SELECT min_price_cents, tier_label FROM category_pricing_tiers WHERE parent_category = 'Dairy'").Scan(&minCents, &tier)
	if minCents != 250 {
		t.Errorf("Dairy min_price_cents: expected 250, got %d", minCents)
	}
	if tier != "premium" {
		t.Errorf("Dairy tier: expected 'premium', got %q", tier)
	}

	// Verify economy tier
	db.QueryRow("SELECT min_price_cents, tier_label FROM category_pricing_tiers WHERE parent_category = 'Cleaning'").Scan(&minCents, &tier)
	if minCents != 100 {
		t.Errorf("Cleaning min_price_cents: expected 100, got %d", minCents)
	}
	if tier != "economy" {
		t.Errorf("Cleaning tier: expected 'economy', got %q", tier)
	}
}

func TestGetCategoryPricingTier_ReturnsCorrectTier(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Get milk's category ID (Dairy → premium)
	var milkCatID int
	server.db.QueryRow("SELECT id FROM categories WHERE name = 'Milk'").Scan(&milkCatID)
	if milkCatID == 0 {
		t.Fatal("Milk category not found")
	}

	minCents, sugCents, tier := getCategoryPricingTier(server.db, milkCatID)
	if minCents != 250 {
		t.Errorf("Milk min_price: expected 250, got %d", minCents)
	}
	if sugCents != 300 {
		t.Errorf("Milk suggested: expected 300, got %d", sugCents)
	}
	if tier != "premium" {
		t.Errorf("Milk tier: expected 'premium', got %q", tier)
	}
}

func TestGetCategoryPricingTier_UnknownCategory(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Unknown category falls back to economy
	minCents, _, tier := getCategoryPricingTier(server.db, 99999)
	if minCents != 100 {
		t.Errorf("unknown category min: expected 100, got %d", minCents)
	}
	if tier != "economy" {
		t.Errorf("unknown category tier: expected 'economy', got %q", tier)
	}
}

func TestBrandDealCreation_EnforcesMinPricing(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "pricing@brand.com", "password1234", "PricingCo")

	// Create deal with Milk product (Dairy → premium, min 250 cents)
	// Attempt with too-low discount
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Brand Milk 1gal",
		"offer_brand":        "PremiumDairy",
		"offer_product":      "PremiumDairy Milk 1gal",
		"discount_cents":     100,
		"daily_budget_cents": 10000,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for below-minimum discount, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["tier"] != "premium" {
		t.Errorf("expected tier 'premium' in error, got %v", body["tier"])
	}

	// Now create with sufficient discount
	rr = doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Brand Milk 1gal",
		"offer_brand":        "PremiumDairy",
		"offer_product":      "PremiumDairy Milk 1gal",
		"discount_cents":     300,
		"daily_budget_cents": 10000,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 with sufficient discount, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify category_id was auto-assigned
	body = parseJSON(rr)
	dealID := int(body["id"].(float64))
	var catID int
	server.db.QueryRow("SELECT category_id FROM brand_deals WHERE id = ?", dealID).Scan(&catID)
	if catID == 0 {
		t.Errorf("expected auto-categorization to assign category_id, got 0")
	}
}

func TestBrandDealCreation_EconomyTierAcceptsLowDiscount(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "econ@brand.com", "password1234", "EconCo")

	// Uncategorized product → economy tier, min 100 cents
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Generic Widget",
		"offer_brand":        "WidgetBrand",
		"offer_product":      "WidgetBrand Premium",
		"discount_cents":     100,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("economy tier with 100 cents: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Below economy minimum
	rr = doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Generic Widget 2",
		"offer_brand":        "WidgetBrand",
		"offer_product":      "WidgetBrand Deluxe",
		"discount_cents":     50,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("below economy minimum: expected 400, got %d", rr.Code)
	}
}

func TestPricingTiers_Endpoint(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "tiers@brand.com", "password1234", "TiersCo")

	rr := doRequest(server.handlePricingTiers, "GET", "/brand/pricing-tiers", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var tiers []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &tiers)
	if len(tiers) < 10 {
		t.Errorf("expected at least 10 pricing tiers, got %d", len(tiers))
	}

	// Verify sorted by min_price_cents DESC
	if len(tiers) >= 2 {
		first := int(tiers[0]["min_price_cents"].(float64))
		last := int(tiers[len(tiers)-1]["min_price_cents"].(float64))
		if first < last {
			t.Errorf("expected descending order: first=%d, last=%d", first, last)
		}
	}
}

func TestPricingTiers_RequiresBrandAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// No auth
	rr := doRequest(server.handlePricingTiers, "GET", "/brand/pricing-tiers", nil, nil)
	if rr.Code == http.StatusOK {
		t.Errorf("expected non-200 without auth, got %d", rr.Code)
	}
}

// =============================================================================
// COMPETITIVE INTEL TESTS
// =============================================================================

func TestCompetitiveIntel_RequiresBrandAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// No auth cookie — should get 401
	rr := doRequest(server.handleCompetitiveIntel, "GET", "/brand/competitive-intel", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d: %s", rr.Code, rr.Body.String())
	}

	// Wrong method — should get 405
	cookie, _ := brandRegister(t, server, "ci-method@brand.com", "password1234", "MethodCo")
	rr = doRequest(server.handleCompetitiveIntel, "POST", "/brand/competitive-intel", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

func TestCompetitiveIntel_EmptyDeals(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "ci-empty@brand.com", "password1234", "EmptyCo")

	rr := doRequest(server.handleCompetitiveIntel, "GET", "/brand/competitive-intel", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	deals, ok := body["deals"].([]interface{})
	if !ok {
		t.Fatal("expected 'deals' to be an array")
	}
	if len(deals) != 0 {
		t.Errorf("expected empty deals array, got %d items", len(deals))
	}

	catSummary, ok := body["category_summary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'category_summary' to be an object")
	}
	if len(catSummary) != 0 {
		t.Errorf("expected empty category_summary, got %d entries", len(catSummary))
	}
}

func TestCompetitiveIntel_ReturnsData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "ci-data@brand.com", "password1234", "DataCo")

	// Create a category so we can link deals to it
	var catID int
	err := server.db.QueryRow("SELECT id FROM categories WHERE name = 'Milk'").Scan(&catID)
	if err != nil {
		// Insert a test category if Milk doesn't exist
		res, err := server.db.Exec("INSERT INTO categories (name, parent_category) VALUES ('Milk', 'Dairy')")
		if err != nil {
			t.Fatalf("failed to create category: %v", err)
		}
		id, _ := res.LastInsertId()
		catID = int(id)
	}

	// Insert deals directly so we can control category_id
	_, err = server.db.Exec(`
		INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents,
			total_budget_cents, status, brand_user_id, category_id)
		VALUES ('Store Brand Milk 1gal', 'DataCo', 'DataCo Milk 1gal', 150, 10000, 'active', ?, ?)`,
		brandUserID, catID)
	if err != nil {
		t.Fatalf("failed to insert deal: %v", err)
	}
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE brand_user_id = ? ORDER BY id DESC LIMIT 1", brandUserID).Scan(&dealID)

	// Insert a competing deal from another brand (different brand_user_id)
	_, err = server.db.Exec(`
		INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents,
			total_budget_cents, status, brand_user_id, category_id)
		VALUES ('Store Brand Milk 2%', 'CompetitorCo', 'CompetitorCo Milk 2%', 200, 15000, 'active', 99999, ?)`, catID)
	if err != nil {
		t.Fatalf("failed to insert competing deal: %v", err)
	}
	var competitorDealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE brand_user_id = 99999 ORDER BY id DESC LIMIT 1").Scan(&competitorDealID)

	// Create a test user so swap_events have valid user_id
	server.db.Exec("INSERT OR IGNORE INTO users (id, name) VALUES (1, 'TestUser')")

	// Insert swap events for our deal: 10 offered, 4 accepted
	for i := 0; i < 10; i++ {
		action := "offered"
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (1, ?, ?)", dealID, action)
	}
	for i := 0; i < 4; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (1, ?, 'accepted')", dealID)
	}

	// Insert swap events for competitor deal: 20 offered, 4 accepted
	for i := 0; i < 20; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (1, ?, 'offered')", competitorDealID)
	}
	for i := 0; i < 4; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (1, ?, 'accepted')", competitorDealID)
	}

	// Now call the endpoint
	rr := doRequest(server.handleCompetitiveIntel, "GET", "/brand/competitive-intel", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)

	// Check deals array
	deals, ok := body["deals"].([]interface{})
	if !ok || len(deals) == 0 {
		t.Fatal("expected non-empty deals array")
	}

	deal := deals[0].(map[string]interface{})

	// Verify deal fields
	if int(deal["deal_id"].(float64)) != dealID {
		t.Errorf("expected deal_id %d, got %v", dealID, deal["deal_id"])
	}
	if deal["offer_product"] != "DataCo Milk 1gal" {
		t.Errorf("expected offer_product 'DataCo Milk 1gal', got %v", deal["offer_product"])
	}
	if deal["category_name"] != "Milk" {
		t.Errorf("expected category_name 'Milk', got %v", deal["category_name"])
	}
	if deal["parent_category"] != "Dairy" {
		t.Errorf("expected parent_category 'Dairy', got %v", deal["parent_category"])
	}

	// Our deal: 10 offered, 4 accepted → acceptance rate 0.4
	if int(deal["total_offered"].(float64)) != 10 {
		t.Errorf("expected total_offered 10, got %v", deal["total_offered"])
	}
	if int(deal["total_accepted"].(float64)) != 4 {
		t.Errorf("expected total_accepted 4, got %v", deal["total_accepted"])
	}
	yourRate := deal["your_acceptance_rate"].(float64)
	if yourRate < 0.39 || yourRate > 0.41 {
		t.Errorf("expected your_acceptance_rate ~0.4, got %v", yourRate)
	}

	// Category avg: (4+4)/(10+20) = 8/30 ≈ 0.267
	catAvg := deal["category_avg_rate"].(float64)
	if catAvg < 0.26 || catAvg > 0.28 {
		t.Errorf("expected category_avg_rate ~0.267, got %v", catAvg)
	}

	// Competing deals: 2 total in category - 1 (self) = 1
	if int(deal["competing_deals"].(float64)) != 1 {
		t.Errorf("expected competing_deals 1, got %v", deal["competing_deals"])
	}

	// Our deal has 0.4 rate vs competitor's 0.2, so we should be rank 1
	if int(deal["category_rank"].(float64)) != 1 {
		t.Errorf("expected category_rank 1 (best), got %v", deal["category_rank"])
	}

	// Check category_summary
	catSummary, ok := body["category_summary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'category_summary' to be an object")
	}
	dairy, ok := catSummary["Dairy"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'Dairy' in category_summary")
	}
	if int(dairy["total_deals"].(float64)) != 2 {
		t.Errorf("expected Dairy total_deals 2, got %v", dairy["total_deals"])
	}
	if int(dairy["your_deals"].(float64)) != 1 {
		t.Errorf("expected Dairy your_deals 1, got %v", dairy["your_deals"])
	}
}

// =============================================================================
// Swap Propagation Tests
// =============================================================================
// CATEGORY INTELLIGENCE SUBSCRIPTIONS
// =============================================================================

func TestCategoryReport_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleCategoryReport, "GET", "/brand/category-report/Dairy", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCategoryReport_FreeTier(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "cat-free@example.com", "password123", "FreeCo")

	// Create a category and a deal linked to it
	var catID int
	server.db.QueryRow("SELECT id FROM categories WHERE parent_category = 'Dairy' LIMIT 1").Scan(&catID)
	if catID == 0 {
		// Insert a subcategory under Dairy if none exists
		res, _ := server.db.Exec("INSERT INTO categories (name, parent_category) VALUES ('Milk', 'Dairy')")
		id, _ := res.LastInsertId()
		catID = int(id)
	}

	// Create a brand deal in the Dairy category
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, category_id, status)
		VALUES ('Competitor Milk', 'TestBrand', 'TestBrand Milk', 100, 5000, ?, 'active')`, catID)

	rr := doRequest(server.handleCategoryReport, "GET", "/brand/category-report/Dairy", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["tier"] != "free" {
		t.Errorf("expected tier=free, got %v", body["tier"])
	}
	if body["category"] != "Dairy" {
		t.Errorf("expected category=Dairy, got %v", body["category"])
	}
	if body["total_deals"] == nil {
		t.Error("expected total_deals in response")
	}
	if body["total_swaps"] == nil {
		t.Error("expected total_swaps in response")
	}
	if body["upgrade_prompt"] == nil {
		t.Error("expected upgrade_prompt in free tier response")
	}
	// Free tier should NOT include premium fields
	if body["avg_acceptance_rate"] != nil {
		t.Error("free tier should not include avg_acceptance_rate")
	}
	if body["subcategories"] != nil {
		t.Error("free tier should not include subcategories")
	}
}

func TestCategoryReport_PremiumTier(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "cat-prem@example.com", "password123", "PremCo")

	// Set subscription to premium
	server.db.Exec(`INSERT INTO brand_subscriptions (brand_user_id, tier) VALUES (?, 'premium')`, brandUserID)

	// Create a category and deals
	var catID int
	server.db.QueryRow("SELECT id FROM categories WHERE parent_category = 'Dairy' LIMIT 1").Scan(&catID)
	if catID == 0 {
		res, _ := server.db.Exec("INSERT INTO categories (name, parent_category) VALUES ('Milk', 'Dairy')")
		id, _ := res.LastInsertId()
		catID = int(id)
	}

	// Create two deals in the Dairy category
	res1, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, category_id, status)
		VALUES ('Competitor Milk', 'TestBrand', 'TestBrand Milk', 150, 5000, ?, 'active')`, catID)
	dealID1, _ := res1.LastInsertId()

	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, category_id, status)
		VALUES ('Competitor Yogurt', 'TestBrand', 'TestBrand Yogurt', 200, 5000, ?, 'active')`, catID)

	// Create some swap events
	server.db.Exec(`INSERT INTO users (id, phone_number) VALUES (9990, '+15559990000')`)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES (9990, ?, 'accepted', '', 2500)`, dealID1)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES (9990, ?, 'declined', 'price', 4000)`, dealID1)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES (9990, ?, 'declined', 'brand_loyal', 3500)`, dealID1)

	rr := doRequest(server.handleCategoryReport, "GET", "/brand/category-report/Dairy", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["tier"] != "premium" {
		t.Errorf("expected tier=premium, got %v", body["tier"])
	}
	if body["category"] != "Dairy" {
		t.Errorf("expected category=Dairy, got %v", body["category"])
	}
	if body["total_deals"] == nil {
		t.Error("expected total_deals in response")
	}
	if body["total_swaps"] == nil {
		t.Error("expected total_swaps in response")
	}
	if body["avg_acceptance_rate"] == nil {
		t.Error("expected avg_acceptance_rate in premium response")
	}
	if body["top_decline_reasons"] == nil {
		t.Error("expected top_decline_reasons in premium response")
	}
	if body["avg_discount_cents"] == nil {
		t.Error("expected avg_discount_cents in premium response")
	}
	if body["avg_time_to_action_ms"] == nil {
		t.Error("expected avg_time_to_action_ms in premium response")
	}
	if body["propagation_rate"] == nil {
		t.Error("expected propagation_rate in premium response")
	}
	if body["subcategories"] == nil {
		t.Error("expected subcategories in premium response")
	}
	// Should NOT have upgrade_prompt
	if body["upgrade_prompt"] != nil {
		t.Error("premium tier should not include upgrade_prompt")
	}

	// Validate decline reasons contain our test data
	reasons, ok := body["top_decline_reasons"].([]interface{})
	if !ok {
		t.Fatal("top_decline_reasons should be an array")
	}
	if len(reasons) < 1 {
		t.Error("expected at least one decline reason")
	}

	// Validate subcategories is an array
	subs, ok := body["subcategories"].([]interface{})
	if !ok {
		t.Fatal("subcategories should be an array")
	}
	if len(subs) < 1 {
		t.Error("expected at least one subcategory")
	}
}

func TestBrandSubscription_GetAndSet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "sub@example.com", "password123", "SubCo")

	// GET — default tier should be free
	rr := doRequest(server.handleBrandSubscription, "GET", "/brand/subscription", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET subscription: expected 200, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["tier"] != "free" {
		t.Errorf("expected default tier=free, got %v", body["tier"])
	}

	// POST — upgrade to premium
	rr = doRequest(server.handleBrandSubscription, "POST", "/brand/subscription",
		map[string]string{"tier": "premium"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST subscription: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body = parseJSON(rr)
	if body["tier"] != "premium" {
		t.Errorf("expected tier=premium after upgrade, got %v", body["tier"])
	}

	// GET — should now be premium
	rr = doRequest(server.handleBrandSubscription, "GET", "/brand/subscription", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET subscription after upgrade: expected 200, got %d", rr.Code)
	}
	body = parseJSON(rr)
	if body["tier"] != "premium" {
		t.Errorf("expected tier=premium, got %v", body["tier"])
	}

	// POST — downgrade back to free
	rr = doRequest(server.handleBrandSubscription, "POST", "/brand/subscription",
		map[string]string{"tier": "free"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST subscription downgrade: expected 200, got %d", rr.Code)
	}
	body = parseJSON(rr)
	if body["tier"] != "free" {
		t.Errorf("expected tier=free after downgrade, got %v", body["tier"])
	}

	// POST — invalid tier
	rr = doRequest(server.handleBrandSubscription, "POST", "/brand/subscription",
		map[string]string{"tier": "gold"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid tier, got %d", rr.Code)
	}

	// No auth — should be 401
	rr = doRequest(server.handleBrandSubscription, "GET", "/brand/subscription", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

// ── Dynamic Auction Pricing Tests ─────────────────────────────────────

func TestBrandDealCreation_WithProductDescription(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Register brand user
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "brand@example.com",
		"password":     "securepassword123",
		"company_name": "Test Co",
	}, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("register: expected 200, got %d", rr.Code)
	}
	var brandCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "brand_session" {
			brandCookie = c
			break
		}
	}

	// Create deal with rich description fields
	rr = doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product":    "Old Brand Cola",
		"offer_brand":          "NewCola",
		"offer_product":        "NewCola Zero Sugar",
		"discount_cents":       150,
		"daily_budget_cents":   10000,
		"product_description":  "Refreshing zero-sugar cola with natural flavors.",
		"key_benefits":         `["Zero sugar","Natural caffeine","No artificial sweeteners"]`,
		"brand_story":          "NewCola was founded to prove that healthy can taste great.",
	}, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create deal: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	dealID := int(body["id"].(float64))

	// Verify stored correctly
	var desc, benefits, story string
	server.db.QueryRow("SELECT product_description, key_benefits, brand_story FROM brand_deals WHERE id = ?", dealID).
		Scan(&desc, &benefits, &story)
	if desc != "Refreshing zero-sugar cola with natural flavors." {
		t.Errorf("product_description: got %q", desc)
	}
	if benefits != `["Zero sugar","Natural caffeine","No artificial sweeteners"]` {
		t.Errorf("key_benefits: got %q", benefits)
	}
	if story != "NewCola was founded to prove that healthy can taste great." {
		t.Errorf("brand_story: got %q", story)
	}
}

// =============================================================================
// Brand Transactions
// =============================================================================

func TestBrandTransactions_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandTransactions, "GET", "/brand/transactions", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestBrandTransactions_EmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "txn@brand.com", "password1234", "TxnCo")

	rr := doRequest(server.handleBrandTransactions, "GET", "/brand/transactions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var txns []interface{}
	json.Unmarshal(rr.Body.Bytes(), &txns)
	if len(txns) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(txns))
	}
}

func TestBrandTransactions_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "txn2@brand.com", "password1234", "TxnCo2")

	// Insert a transaction
	server.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
		VALUES (?, 'deposit', 10000, 'Initial deposit', 'pi_test_123')`, brandUserID)

	rr := doRequest(server.handleBrandTransactions, "GET", "/brand/transactions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var txns []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &txns)
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	if txns[0]["type"] != "deposit" {
		t.Errorf("transaction type: got %v, want deposit", txns[0]["type"])
	}
	if txns[0]["amount_cents"].(float64) != 10000 {
		t.Errorf("amount_cents: got %v, want 10000", txns[0]["amount_cents"])
	}
	if txns[0]["stripe_id"] != "pi_test_123" {
		t.Errorf("stripe_id: got %v, want pi_test_123", txns[0]["stripe_id"])
	}
}

func TestBrandTransactions_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandTransactions, "POST", "/brand/transactions", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

// =============================================================================
// Brand Export
// =============================================================================

func TestBrandExport_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandExport, "GET", "/brand/export", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestBrandExport_CSVHeaders(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "export@brand.com", "password1234", "ExportCo")

	rr := doRequest(server.handleBrandExport, "GET", "/brand/export?type=swaps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "text/csv" {
		t.Errorf("Content-Type: got %s, want text/csv", ct)
	}

	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Errorf("Content-Disposition missing attachment/csv: got %s", cd)
	}

	// Should have CSV header row
	body := rr.Body.String()
	if !strings.Contains(body, "date,deal_id") {
		t.Errorf("CSV missing header row, body: %s", body)
	}
}

func TestBrandExport_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandExport, "POST", "/brand/export", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

// =============================================================================
// Places Endpoint Tests
// =============================================================================
