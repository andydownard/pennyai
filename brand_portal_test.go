package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
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
	if body["brand_user_id"] == nil {
		t.Error("expected non-nil brand_user_id")
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
	cookie, _ := brandRegister(t, server, "me@brand.com", "password1234", "MyCo Inc")

	rr := doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Me: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	// brand_user_id type depends on whether PGDB RETURNING id worked
	if body["brand_user_id"] == nil {
		t.Error("expected non-nil brand_user_id in /brand/me response")
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
	if deals[0]["status"] != "pending_review" {
		t.Errorf("status: got %v, want 'pending_review'", deals[0]["status"])
	}
	if deals[0]["review_status"] != "pending" {
		t.Errorf("review_status: got %v, want 'pending'", deals[0]["review_status"])
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
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-999', '5559999999')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name) VALUES (999, 'user-999', 'Test Item')")
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action, created_at)
		VALUES ('user-999', ?, 999, 'offered', datetime('now'))`, dealID)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action, created_at)
		VALUES ('user-999', ?, 999, 'offered', datetime('now', '-60 days'))`, dealID)

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

// stripeWebhookPayload builds a payment_intent.succeeded event body.
func stripeWebhookPayload(piID string, amountCents, brandUserID int) string {
	return fmt.Sprintf(`{
		"type": "payment_intent.succeeded",
		"data": {
			"object": {
				"id": "%s",
				"amount": %d,
				"metadata": {
					"brand_user_id": "%d"
				}
			}
		}
	}`, piID, amountCents, brandUserID)
}

// TestBrandPortal_StripeWebhookConcurrentDelivery fires the same
// payment_intent.succeeded event from many goroutines at once. Stripe retries
// webhooks by design and can deliver the same event concurrently; the
// COUNT-then-UPDATE check-then-act in handleStripeWebhook double-credits real
// money when two deliveries interleave.
func TestBrandPortal_StripeWebhookConcurrentDelivery(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, brandUserID := brandRegister(t, server, "race@brand.com", "password1234", "RaceCo")

	payload := stripeWebhookPayload("pi_test_race_1", 25000, brandUserID)

	const n = 8
	codes := make([]int, n)
	hammer(t, n, func(i int) {
		req := httptest.NewRequest("POST", "/brand/stripe-webhook", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.handleStripeWebhook(rr, req)
		codes[i] = rr.Code
	})

	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("delivery %d: expected 200 (Stripe must not be told to retry), got %d", i, c)
		}
	}

	var txnCount int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_transactions WHERE stripe_id = 'pi_test_race_1'").Scan(&txnCount)
	if txnCount != 1 {
		t.Errorf("ledger rows for one payment_intent: got %d, want 1 (double-credit)", txnCount)
	}

	var balance int
	server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&balance)
	if balance != 25000 {
		t.Errorf("balance after %d concurrent deliveries: got %d, want 25000", n, balance)
	}
}

// TestBrandPortal_StripeWebhookPreSeededLedgerRow is the deterministic partner
// to the hammer test above: the ledger row already exists, so the handler must
// credit nothing and still return 200.
func TestBrandPortal_StripeWebhookPreSeededLedgerRow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, brandUserID := brandRegister(t, server, "seeded@brand.com", "password1234", "SeededCo")

	// A prior delivery already landed the ledger row and credited the balance.
	if _, err := server.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
		VALUES (?, 'deposit', 25000, 'Stripe deposit', 'pi_test_seeded_1')`, brandUserID); err != nil {
		t.Fatalf("seeding ledger row: %v", err)
	}
	if _, err := server.db.Exec(`UPDATE brand_balance SET balance_cents = 25000 WHERE brand_user_id = ?`, brandUserID); err != nil {
		t.Fatalf("seeding balance: %v", err)
	}

	req := httptest.NewRequest("POST", "/brand/stripe-webhook",
		strings.NewReader(stripeWebhookPayload("pi_test_seeded_1", 25000, brandUserID)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleStripeWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("duplicate delivery: expected 200, got %d", rr.Code)
	}

	var balance int
	server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&balance)
	if balance != 25000 {
		t.Errorf("balance must be unchanged on duplicate: got %d, want 25000", balance)
	}
	var txnCount int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_transactions WHERE stripe_id = 'pi_test_seeded_1'").Scan(&txnCount)
	if txnCount != 1 {
		t.Errorf("ledger rows: got %d, want 1", txnCount)
	}
}

// TestBrandPortal_StripeWebhookMissingBalanceRow proves a deposit is never
// silently swallowed when the brand_balance row is absent.
func TestBrandPortal_StripeWebhookMissingBalanceRow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, brandUserID := brandRegister(t, server, "nobal@brand.com", "password1234", "NoBalCo")

	if _, err := server.db.Exec("DELETE FROM brand_balance WHERE brand_user_id = ?", brandUserID); err != nil {
		t.Fatalf("removing balance row: %v", err)
	}

	req := httptest.NewRequest("POST", "/brand/stripe-webhook",
		strings.NewReader(stripeWebhookPayload("pi_test_nobal_1", 7700, brandUserID)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.handleStripeWebhook(rr, req)

	var balance int
	err := server.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", brandUserID).Scan(&balance)
	if err != nil {
		t.Fatalf("balance row must exist after a credited deposit, got: %v", err)
	}
	if balance != 7700 {
		t.Errorf("balance: got %d, want 7700 (deposit swallowed)", balance)
	}
}

// TestBrandPortal_StripeWebhookUniqueIndexExists pins the partial unique index
// that makes the double-credit impossible at the storage layer. It is partial
// because the dev/test deposit fallback inserts stripe_id = '' for every row.
func TestBrandPortal_StripeWebhookUniqueIndexExists(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, brandUserID := brandRegister(t, server, "idx@brand.com", "password1234", "IdxCo")

	ins := func(stripeID string) error {
		_, err := server.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
			VALUES (?, 'deposit', 100, 'x', ?)`, brandUserID, stripeID)
		return err
	}

	if err := ins("pi_unique_1"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := ins("pi_unique_1"); err == nil {
		t.Error("duplicate stripe_id was accepted — unique index missing from the test schema")
	}

	// The empty-string sentinel used by the dev deposit fallback must stay
	// insertable more than once.
	if err := ins(""); err != nil {
		t.Fatalf("first empty stripe_id insert: %v", err)
	}
	if err := ins(""); err != nil {
		t.Errorf("second empty stripe_id insert must be allowed (dev deposit fallback), got: %v", err)
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
		DashboardURL:       "https://brand.thrift.ly",
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
	if !strings.Contains(html, "brand.thrift.ly") {
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
	_, db := openTestDB(t)

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
	server.db.Exec("INSERT OR IGNORE INTO users (id, name) VALUES ('user-1', 'TestUser')")

	// A user accepts a campaign at most once (idx_swap_events_one_accept), so
	// the accepts in an acceptance rate come from different users. Offers may
	// repeat for one user.
	accepter := func(n int) string {
		id := fmt.Sprintf("ci-user-%d", n)
		server.db.Exec("INSERT OR IGNORE INTO users (id, name) VALUES (?, 'TestUser')", id)
		return id
	}

	// Insert swap events for our deal: 10 offered, 4 accepted
	for i := 0; i < 10; i++ {
		action := "offered"
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-1', ?, ?)", dealID, action)
	}
	for i := 0; i < 4; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (?, ?, 'accepted')", accepter(i), dealID)
	}

	// Insert swap events for competitor deal: 20 offered, 4 accepted
	for i := 0; i < 20; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-1', ?, 'offered')", competitorDealID)
	}
	for i := 0; i < 4; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES (?, ?, 'accepted')", accepter(i), competitorDealID)
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
	server.db.Exec(`INSERT INTO users (id, phone) VALUES ('user-9990', '+15559990000')`)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES ('user-9990', ?, 'accepted', '', 2500)`, dealID1)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES ('user-9990', ?, 'declined', 'price', 4000)`, dealID1)
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, decline_reason, time_to_action_ms) VALUES ('user-9990', ?, 'declined', 'brand_loyal', 3500)`, dealID1)

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
	// JSONB re-serialises (adds spaces), so compare the decoded array.
	var gotBenefits []string
	if err := json.Unmarshal([]byte(benefits), &gotBenefits); err != nil || len(gotBenefits) != 3 ||
		gotBenefits[0] != "Zero sugar" || gotBenefits[1] != "Natural caffeine" || gotBenefits[2] != "No artificial sweeteners" {
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

func TestBrandExport_TransactionsCSV(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "export@brand.com", "password1234", "ExportCo")

	// Get brand_user_id
	var brandUserID int
	server.db.QueryRow("SELECT id FROM brand_users WHERE email = 'export@brand.com'").Scan(&brandUserID)

	// Insert a transaction
	server.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id, created_at)
		VALUES (?, 'deposit', 10000, 'Initial deposit', 'pi_test123', datetime('now'))`, brandUserID)

	rr := doRequest(server.handleBrandExport, "GET", "/brand/export?type=transactions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "date,type,amount_cents,description,stripe_id") {
		t.Errorf("CSV missing transactions header row")
	}
	if !strings.Contains(body, "deposit") || !strings.Contains(body, "10000") {
		t.Errorf("CSV missing transaction data: %s", body)
	}
}

func TestBrandExport_DealsCSV(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "export@brand.com", "password1234", "ExportCo")

	var brandUserID int
	server.db.QueryRow("SELECT id FROM brand_users WHERE email = 'export@brand.com'").Scan(&brandUserID)

	server.db.Exec(`INSERT INTO brand_deals (brand_user_id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, budget_spent_cents, status, deal_type)
		VALUES (?, 'heinz ketchup', 'Primal Kitchen', 'Organic Ketchup', 150, 10000, 500, 'active', 'sku_swap')`, brandUserID)

	rr := doRequest(server.handleBrandExport, "GET", "/brand/export?type=deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "id,competitor_product") {
		t.Errorf("CSV missing deals header row")
	}
	if !strings.Contains(body, "Primal Kitchen") {
		t.Errorf("CSV missing deal data: %s", body)
	}
}

func TestBrandExport_InvalidType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "export@brand.com", "password1234", "ExportCo")

	rr := doRequest(server.handleBrandExport, "GET", "/brand/export?type=invalid", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid export type, got %d", rr.Code)
	}
}

func TestBrandExport_DefaultType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "export@brand.com", "password1234", "ExportCo")

	// No type param should default to "swaps"
	rr := doRequest(server.handleBrandExport, "GET", "/brand/export", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "date,deal_id") {
		t.Errorf("expected default swaps CSV header, got: %s", body)
	}
}

// =============================================================================
// Brand Feedback Tests
// =============================================================================

func TestBrandFeedback_SubmitHappyPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "fb@brand.com", "password1234", "FeedbackCo")

	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "Please add dark mode to the portal",
		"type":    "feature",
		"urgency": "normal",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}
	if body["id"] == nil || body["id"].(float64) == 0 {
		t.Error("expected non-zero feedback id")
	}
	if body["message"] == nil {
		t.Error("expected message in response")
	}
}

func TestBrandFeedback_EmptyBodyReturns400(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "empty@brand.com", "password1234", "EmptyCo")

	// Empty body string
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "",
		"type":    "bug",
		"urgency": "low",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d: %s", rr.Code, rr.Body.String())
	}

	// Whitespace-only body
	rr = doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body": "   \t\n  ",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for whitespace-only body, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBrandFeedback_NoAuthReturns401(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// POST with no cookie
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body": "Unauthorized feedback",
	}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("POST no auth: expected 401, got %d", rr.Code)
	}

	// GET with no cookie
	rr = doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET no auth: expected 401, got %d", rr.Code)
	}
}

func TestBrandFeedback_DefaultsInvalidTypeAndUrgency(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "defaults@brand.com", "password1234", "DefaultsCo")

	// Submit with invalid type and urgency — should default to "feature" and "normal"
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "Testing defaults",
		"type":    "nonexistent_type",
		"urgency": "super_urgent",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify defaults by fetching the feedback
	rr = doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET feedback: expected 200, got %d", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 feedback item, got %d", len(items))
	}
	if items[0]["type"] != "feature" {
		t.Errorf("expected type defaulted to 'feature', got %v", items[0]["type"])
	}
	if items[0]["urgency"] != "normal" {
		t.Errorf("expected urgency defaulted to 'normal', got %v", items[0]["urgency"])
	}
}

func TestBrandFeedback_ListEmptyInitially(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "listempty@brand.com", "password1234", "ListEmptyCo")

	rr := doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("expected 0 feedback items initially, got %d", len(items))
	}
}

func TestBrandFeedback_ListReturnsSubmittedItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "listfull@brand.com", "password1234", "ListFullCo")

	// Submit two feedback items
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "First feedback item",
		"type":    "bug",
		"urgency": "high",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("submit 1: expected 201, got %d", rr.Code)
	}

	rr = doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "Second feedback item",
		"type":    "other",
		"urgency": "low",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("submit 2: expected 201, got %d", rr.Code)
	}

	// Fetch feedback list
	rr = doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET feedback: expected 200, got %d", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Fatalf("expected 2 feedback items, got %d", len(items))
	}

	// Results should be in reverse order (ORDER BY id DESC)
	if items[0]["body"] != "Second feedback item" {
		t.Errorf("first item body: got %v, want 'Second feedback item'", items[0]["body"])
	}
	if items[0]["type"] != "other" {
		t.Errorf("first item type: got %v, want 'other'", items[0]["type"])
	}
	if items[0]["urgency"] != "low" {
		t.Errorf("first item urgency: got %v, want 'low'", items[0]["urgency"])
	}
	if items[0]["status"] != "new" {
		t.Errorf("first item status: got %v, want 'new'", items[0]["status"])
	}
	if items[1]["body"] != "First feedback item" {
		t.Errorf("second item body: got %v, want 'First feedback item'", items[1]["body"])
	}
	if items[1]["type"] != "bug" {
		t.Errorf("second item type: got %v, want 'bug'", items[1]["type"])
	}
	if items[1]["urgency"] != "high" {
		t.Errorf("second item urgency: got %v, want 'high'", items[1]["urgency"])
	}

	// Verify items belong to the correct user (another user should see nothing)
	cookie2, _ := brandRegister(t, server, "other@brand.com", "password1234", "OtherCo")
	rr = doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET other user feedback: expected 200, got %d", rr.Code)
	}
	var otherItems []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &otherItems)
	if len(otherItems) != 0 {
		t.Errorf("other user should see 0 items, got %d", len(otherItems))
	}
}

func TestBrandFeedback_BodyTooLongReturns400(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "long@brand.com", "password1234", "LongCo")

	longBody := strings.Repeat("a", 10001)
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body": longBody,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for body > 10000 chars, got %d: %s", rr.Code, rr.Body.String())
	}

	// Exactly 10000 should succeed
	exactBody := strings.Repeat("b", 10000)
	rr = doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body": exactBody,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201 for body of exactly 10000 chars, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBrandFeedback_RateLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "ratelimit@brand.com", "password1234", "RateLimitCo")

	// Submit 10 feedback items (the max per hour)
	for i := 0; i < 10; i++ {
		rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
			"body":    fmt.Sprintf("Feedback item %d", i+1),
			"type":    "feature",
			"urgency": "normal",
		}, []*http.Cookie{cookie})
		if rr.Code != http.StatusCreated {
			t.Fatalf("submit %d: expected 201, got %d: %s", i+1, rr.Code, rr.Body.String())
		}
	}

	// 11th should be rate limited
	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body":    "This should be rate limited",
		"type":    "bug",
		"urgency": "low",
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for 11th submission, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["error"] == nil || body["error"] == "" {
		t.Error("expected error message in rate limit response")
	}

	// A different user should NOT be rate limited
	cookie2, _ := brandRegister(t, server, "ratelimit2@brand.com", "password1234", "RateLimit2Co")
	rr = doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]interface{}{
		"body": "Different user feedback",
	}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusCreated {
		t.Errorf("different user should not be rate limited, got %d", rr.Code)
	}
}

// =============================================================================
// Places Endpoint Tests
// =============================================================================


// =============================================================================
// resolveBrandAlternatives TESTS (18.8% → target ~80%)
// =============================================================================

func TestResolveBrandAlternatives_NoAlternatives(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110001")

	items := []ListItem{{ID: 1, Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: nil}}

	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	if len(result) != 0 {
		t.Errorf("Expected 0 disambiguations for no alternatives, got %d", len(result))
	}
}

func TestResolveBrandAlternatives_ItemNotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110002")

	// items list doesn't include the parsed item
	items := []ListItem{{ID: 1, Name: "Bread"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley"}}}

	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	if len(result) != 0 {
		t.Errorf("Expected 0 disambiguations for unmatched item, got %d", len(result))
	}
}

func TestResolveBrandAlternatives_NoPref_AskMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110003")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")
	items := []ListItem{{ID: int(itemID), Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley", "Fairlife"}}}

	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	if len(result) != 1 {
		t.Fatalf("Expected 1 disambiguation, got %d", len(result))
	}
	if result[0].AutoAssigned {
		t.Error("Should not be auto-assigned without global defaults")
	}
	if result[0].CurrentBrand != "Horizon" {
		t.Errorf("CurrentBrand: got %q, want Horizon", result[0].CurrentBrand)
	}
	if len(result[0].Alternatives) != 3 {
		t.Errorf("Alternatives count: got %d, want 3", len(result[0].Alternatives))
	}
}

func TestResolveBrandAlternatives_StrongPreference(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110004")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Insert a strong preference (streak >= 3)
	server.db.Exec(
		"INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked) VALUES (?, ?, ?, ?, ?, datetime('now'))",
		userID, "milk", "Organic Valley", 5, 10,
	)

	items := []ListItem{{ID: int(itemID), Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley", "Fairlife"}}}

	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	if len(result) != 0 {
		t.Errorf("Strong preference should auto-apply without disambiguation, got %d", len(result))
	}

	// Verify the item's brand was updated
	var brand string
	server.db.QueryRow("SELECT COALESCE(brand,'') FROM list_items WHERE id = ?", itemID).Scan(&brand)
	if brand != "Organic Valley" {
		t.Errorf("Item brand should be Organic Valley, got %q", brand)
	}
}

func TestResolveBrandAlternatives_AutoMode_WeakPreference(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110005")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Insert a weak preference (streak < 3) in auto mode
	server.db.Exec(
		"INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked) VALUES (?, ?, ?, ?, ?, datetime('now'))",
		userID, "milk", "Fairlife", 1, 2,
	)

	items := []ListItem{{ID: int(itemID), Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley", "Fairlife"}}}

	result := resolveBrandAlternatives(server.db, userID, "auto", items, parsed)
	if len(result) != 0 {
		t.Errorf("Auto mode with any preference should auto-apply, got %d disambiguations", len(result))
	}

	var brand string
	server.db.QueryRow("SELECT COALESCE(brand,'') FROM list_items WHERE id = ?", itemID).Scan(&brand)
	if brand != "Fairlife" {
		t.Errorf("Item brand should be Fairlife, got %q", brand)
	}
}

func TestResolveBrandAlternatives_StalePreference(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110006")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Insert a stale preference (>90 days old, low streak)
	server.db.Exec(
		"INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked) VALUES (?, ?, ?, ?, ?, datetime('now', '-100 days'))",
		userID, "milk", "Fairlife", 2, 3,
	)

	items := []ListItem{{ID: int(itemID), Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley"}}}

	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	// Stale pref with streak < 5 should be treated as no preference → disambiguation
	if len(result) != 1 {
		t.Fatalf("Stale preference should require disambiguation, got %d", len(result))
	}
	if result[0].AutoAssigned {
		t.Error("Should not be auto-assigned")
	}
}

func TestResolveBrandAlternatives_GlobalDefaults(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110007")

	itemID := createListItem(t, server.db, userID, "Ketchup", "Condiments")

	// Seed global brand choices for "ketchup" from multiple users
	for i := 0; i < 5; i++ {
		otherUserID := fmt.Sprintf("other-user-%d", 1000+i)
		server.db.Exec("INSERT OR IGNORE INTO users (id, phone) VALUES (?, ?)", otherUserID, fmt.Sprintf("555000%04d", i))
		server.db.Exec(
			"INSERT INTO brand_choices (user_id, product_category, brand, source) VALUES (?, ?, ?, ?)",
			otherUserID, "ketchup", "Heinz", "manual",
		)
	}

	items := []ListItem{{ID: int(itemID), Name: "Ketchup"}}
	parsed := []ParsedItem{{Name: "Ketchup", Brand: "", BrandAlternatives: []string{"Heinz", "Hunt's", "Sir Kensington's"}}}

	result := resolveBrandAlternatives(server.db, userID, "auto", items, parsed, true)
	if len(result) != 1 {
		t.Fatalf("Expected 1 disambiguation with global default, got %d", len(result))
	}
	if !result[0].AutoAssigned {
		t.Error("Global default should set AutoAssigned = true")
	}
	if result[0].CurrentBrand != "Heinz" {
		t.Errorf("Global default brand should be Heinz, got %q", result[0].CurrentBrand)
	}
}

func TestResolveBrandAlternatives_PreferenceReordering(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551110008")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Insert a weak preference (streak=1, not enough for auto-apply in ask mode but still has preference)
	server.db.Exec(
		"INSERT INTO brand_preferences (user_id, product_category, brand, streak, pick_count, last_picked) VALUES (?, ?, ?, ?, ?, datetime('now'))",
		userID, "milk", "Fairlife", 1, 1,
	)

	items := []ListItem{{ID: int(itemID), Name: "Milk"}}
	parsed := []ParsedItem{{Name: "Milk", Brand: "Horizon", BrandAlternatives: []string{"Horizon", "Organic Valley", "Fairlife"}}}

	// ask mode, streak=1 → needs disambiguation, but preferred brand should be first
	result := resolveBrandAlternatives(server.db, userID, "ask", items, parsed)
	if len(result) != 1 {
		t.Fatalf("Expected 1 disambiguation, got %d", len(result))
	}
	if result[0].Alternatives[0] != "Fairlife" {
		t.Errorf("Preferred brand should be first in alternatives, got %q", result[0].Alternatives[0])
	}
}

// =============================================================================
// handleBrandUnsubscribe TESTS (40% → target ~90%)
// =============================================================================

func TestBrandUnsubscribe_AuthenticatedGET(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	brandCookie, _ := brandRegister(t, server, "unsub@test.com", "password123", "UnsubCo")

	rr := doRequest(server.handleBrandUnsubscribe, "GET", "/brand/unsubscribe", nil, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Unsubscribe") {
		t.Error("Should show unsubscribe form")
	}
	if strings.Contains(body, "log in") {
		t.Error("Authenticated user should not see login prompt")
	}
}

func TestBrandUnsubscribe_AuthenticatedPOST(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	brandCookie, brandUserID := brandRegister(t, server, "unsub2@test.com", "password123", "UnsubCo2")

	rr := doRequest(server.handleBrandUnsubscribe, "POST", "/brand/unsubscribe", nil, []*http.Cookie{brandCookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Unsubscribed") {
		t.Error("Should show unsubscribed confirmation")
	}

	// Verify the database was updated
	var optOut int
	server.db.QueryRow("SELECT digest_opt_out FROM brand_users WHERE id = ?", brandUserID).Scan(&optOut)
	if optOut != 1 {
		t.Errorf("digest_opt_out should be 1, got %d", optOut)
	}
}


// =============================================================================
// recordBrandChoice TESTS (55.6% → target ~90%)
// =============================================================================

func TestRecordBrandChoice_NewPreference(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440001")

	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")

	// Verify brand_choices log entry
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_choices WHERE user_id = ? AND product_category = 'milk'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("brand_choices should have 1 entry, got %d", count)
	}

	// Verify brand_preferences was created
	var brand string
	var streak, pickCount int
	server.db.QueryRow("SELECT brand, streak, pick_count FROM brand_preferences WHERE user_id = ? AND product_category = 'milk'", userID).Scan(&brand, &streak, &pickCount)
	if brand != "Horizon" {
		t.Errorf("brand: got %q, want Horizon", brand)
	}
	if streak != 1 {
		t.Errorf("streak: got %d, want 1", streak)
	}
	if pickCount != 1 {
		t.Errorf("pick_count: got %d, want 1", pickCount)
	}
}

func TestRecordBrandChoice_SameBrandIncrementsStreak(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440002")

	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")

	var streak, pickCount int
	server.db.QueryRow("SELECT streak, pick_count FROM brand_preferences WHERE user_id = ? AND product_category = 'milk'", userID).Scan(&streak, &pickCount)
	if streak != 3 {
		t.Errorf("streak: got %d, want 3", streak)
	}
	if pickCount != 3 {
		t.Errorf("pick_count: got %d, want 3", pickCount)
	}
}

func TestRecordBrandChoice_DifferentBrandResetsStreak(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440003")

	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "Organic Valley", "manual")

	var brand string
	var streak, pickCount int
	server.db.QueryRow("SELECT brand, streak, pick_count FROM brand_preferences WHERE user_id = ? AND product_category = 'milk'", userID).Scan(&brand, &streak, &pickCount)
	if brand != "Organic Valley" {
		t.Errorf("brand should switch to Organic Valley, got %q", brand)
	}
	if streak != 1 {
		t.Errorf("streak should reset to 1, got %d", streak)
	}
	if pickCount != 3 {
		t.Errorf("pick_count should be 3, got %d", pickCount)
	}
}

func TestRecordBrandChoice_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440004")

	recordBrandChoice(server.db, userID, "Milk", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "horizon", "manual") // lowercase

	var streak int
	server.db.QueryRow("SELECT streak FROM brand_preferences WHERE user_id = ? AND product_category = 'milk'", userID).Scan(&streak)
	if streak != 2 {
		t.Errorf("Case-insensitive match should increment streak, got %d", streak)
	}
}

func TestRecordBrandChoice_EmptyInputs(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440005")

	recordBrandChoice(server.db, userID, "", "Horizon", "manual")
	recordBrandChoice(server.db, userID, "Milk", "", "manual")
	recordBrandChoice(server.db, userID, "  ", "  ", "manual")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_choices WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Empty inputs should not create brand_choices, got %d", count)
	}
}

func TestRecordBrandChoice_NormalizesCategory(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5554440006")

	recordBrandChoice(server.db, userID, "  MILK  ", "Horizon", "manual")

	var category string
	server.db.QueryRow("SELECT product_category FROM brand_choices WHERE user_id = ?", userID).Scan(&category)
	if category != "milk" {
		t.Errorf("Category should be normalized to 'milk', got %q", category)
	}
}


// TestTruncateRunes already exists in admin_test.go

// =============================================================================
// handleBrandLogin TESTS (66.7% → target ~90%)
// =============================================================================

func TestBrandLogin_NonexistentEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandLogin, "POST", "/brand/login", map[string]string{
		"email":    "nobody@example.com",
		"password": "somepassword",
	}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for nonexistent email, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "Invalid email or password" {
		t.Errorf("unexpected error message: %v", body["error"])
	}
}

func TestBrandLogin_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandLogin, "GET", "/brand/login", nil, nil)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rr.Code)
	}
}

func TestBrandLogin_EmptyBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Send a POST with empty JSON object — email will be empty, should hit the DB query
	// which returns no rows → 401
	rr := doRequest(server.handleBrandLogin, "POST", "/brand/login", map[string]string{
		"email":    "",
		"password": "",
	}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty credentials, got %d", rr.Code)
	}
}

// =============================================================================
// updateBrandDeal TESTS (55.9% → target ~85%)
// =============================================================================

func TestUpdateBrandDeal_NonexistentDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "update-nonexist@test.com", "password123", "TestCo")

	rr := doRequest(server.handleBrandDeals, "PUT", "/brand/deals/99999", map[string]interface{}{
		"status": "paused",
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent deal, got %d", rr.Code)
	}
}

func TestUpdateBrandDeal_WrongOwner(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Owner creates the deal
	ownerCookie, _ := brandRegister(t, server, "deal-owner@test.com", "password123", "OwnerCo")
	dealID := createTestDeal(t, server, ownerCookie)

	// Different user tries to update it
	otherCookie, _ := brandRegister(t, server, "deal-other@test.com", "password123", "OtherCo")
	rr := doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID), map[string]interface{}{
		"status": "paused",
	}, []*http.Cookie{otherCookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrong owner, got %d", rr.Code)
	}
}

func TestUpdateBrandDeal_EmptyFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "update-empty@test.com", "password123", "TestCo")
	dealID := createTestDeal(t, server, cookie)

	// Send an empty JSON body — no updatable fields
	rr := doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID), map[string]interface{}{}, []*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty fields, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["error"] != "No fields to update" {
		t.Errorf("unexpected error: %v", body["error"])
	}
}

func TestUpdateBrandDeal_SuccessfulUpdate(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "update-success@test.com", "password123", "TestCo")
	dealID := createTestDeal(t, server, cookie)

	newStatus := "paused"
	newDiscount := 200
	newBudget := 50000
	newWhy := "Better taste, better price"
	newAllergen := "Contains soy"

	rr := doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID), map[string]interface{}{
		"status":           newStatus,
		"discount_cents":   newDiscount,
		"daily_budget_cents": newBudget,
		"why_bullets":      newWhy,
		"allergen_notes":   newAllergen,
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for successful update, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["message"] != "Deal updated" {
		t.Errorf("unexpected message: %v", body["message"])
	}

	// Verify the fields actually changed in DB
	var status, why, allergen string
	var discount, budget int
	err := server.db.QueryRow(
		"SELECT status, discount_cents, total_budget_cents, COALESCE(why_bullets,''), COALESCE(allergen_notes,'') FROM brand_deals WHERE id = ?", dealID,
	).Scan(&status, &discount, &budget, &why, &allergen)
	if err != nil {
		t.Fatalf("failed to query updated deal: %v", err)
	}
	if status != newStatus {
		t.Errorf("status: got %q, want %q", status, newStatus)
	}
	if discount != newDiscount {
		t.Errorf("discount_cents: got %d, want %d", discount, newDiscount)
	}
	if budget != newBudget {
		t.Errorf("total_budget_cents: got %d, want %d", budget, newBudget)
	}
	// why_bullets is a JSON array column: plain text is kept as one bullet.
	if wantWhy := jsonArrayText(newWhy); why != wantWhy {
		t.Errorf("why_bullets: got %q, want %q", why, wantWhy)
	}
	if allergen != newAllergen {
		t.Errorf("allergen_notes: got %q, want %q", allergen, newAllergen)
	}
}

// =============================================================================
// deleteBrandDeal TESTS (71.4% → target ~95%)
// =============================================================================

func TestDeleteBrandDeal_NonexistentDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "delete-nonexist@test.com", "password123", "TestCo")

	rr := doRequest(server.handleBrandDeals, "DELETE", "/brand/deals/99999", nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent deal, got %d", rr.Code)
	}
}

func TestDeleteBrandDeal_WrongOwner(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Owner creates the deal
	ownerCookie, _ := brandRegister(t, server, "del-owner@test.com", "password123", "OwnerCo")
	dealID := createTestDeal(t, server, ownerCookie)

	// Different user tries to delete it
	otherCookie, _ := brandRegister(t, server, "del-other@test.com", "password123", "OtherCo")
	rr := doRequest(server.handleBrandDeals, "DELETE", fmt.Sprintf("/brand/deals/%d", dealID), nil, []*http.Cookie{otherCookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrong owner, got %d", rr.Code)
	}
}


// --- BrandRegister ---

func TestBrandRegister_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleBrandRegister, "GET", "/brand/register", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestBrandRegister_MissingFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email": "test@brand.com",
	}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBrandRegister_ShortPassword(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "test@brand.com",
		"password":     "short",
		"company_name": "TestCo",
	}, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBrandRegister_DuplicateEmail(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	// First registration
	brandRegister(t, server, "dup@brand.com", "password1234", "FirstCo")
	// Second with same email
	rr := doRequest(server.handleBrandRegister, "POST", "/brand/register", map[string]string{
		"email":        "dup@brand.com",
		"password":     "password1234",
		"company_name": "SecondCo",
	}, nil)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- LogoProxy ---

func TestLogoProxy_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleLogoProxy, "POST", "/api/logo", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestLogoProxy_MissingURL(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("GET", "/api/logo", nil)
	rr := httptest.NewRecorder()
	server.handleLogoProxy(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLogoProxy_InvalidURL(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	req := httptest.NewRequest("GET", "/api/logo?url="+url.QueryEscape("not-a-url"), nil)
	rr := httptest.NewRecorder()
	server.handleLogoProxy(rr, req)
	// Should return 400 or fetch error
	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for invalid URL")
	}
}

// --- handleBrandDeals routing ---

func TestBrandDeals_InvalidDealID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "deals@brand.com", "password1234", "DealsCo")

	req := httptest.NewRequest("PUT", "/brand/deals/notanumber", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleBrandDeals(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric deal ID, got %d", rr.Code)
	}
}

func TestBrandDeals_UnsupportedMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "method@brand.com", "password1234", "MethodCo")

	req := httptest.NewRequest("PATCH", "/brand/deals", bytes.NewBuffer(nil))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleBrandDeals(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PATCH, got %d", rr.Code)
	}
}

func TestBrandDeals_GetEmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "empty@brand.com", "password1234", "EmptyCo")

	rr := doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var deals []interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 0 {
		t.Errorf("expected empty deals list, got %d", len(deals))
	}
}

// =============================================================================
// Batch 8: PricingTiers, CategoryReport, BrandDigest, BrandDeals routing
// =============================================================================

func TestPricingTiers_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePricingTiers, "POST", "/brand/pricing-tiers", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestPricingTiers_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handlePricingTiers, "GET", "/brand/pricing-tiers", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestPricingTiers_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "tiers@brand.com", "password1234", "TiersCo")

	rr := doRequest(server.handlePricingTiers, "GET", "/brand/pricing-tiers", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// Should return array (may be empty or seeded)
	var tiers []interface{}
	json.Unmarshal(rr.Body.Bytes(), &tiers)
	// Just verify it's a valid JSON array
	if tiers == nil {
		t.Error("expected non-nil tiers array")
	}
}

func TestCategoryReport_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleCategoryReport, "POST", "/brand/category-report/Dairy", nil, nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCategoryReport_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleCategoryReport, "GET", "/brand/category-report/Dairy", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCategoryReport_MissingCategory(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "cat@brand.com", "password1234", "CatCo")

	req := httptest.NewRequest("GET", "/brand/category-report/", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleCategoryReport(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCategoryReport_FreeTierBasic(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "free@brand.com", "password1234", "FreeCo")

	req := httptest.NewRequest("GET", "/brand/category-report/Dairy", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleCategoryReport(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["tier"] != "free" {
		t.Errorf("expected tier=free, got %v", body["tier"])
	}
	if body["upgrade_prompt"] == nil {
		t.Error("expected upgrade_prompt for free tier")
	}
}

func TestCategoryReport_PremiumTierFull(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "premium@brand.com", "password1234", "PremiumCo")

	// Give premium subscription
	server.db.Exec("INSERT INTO brand_subscriptions (brand_user_id, tier) VALUES (?, 'premium')", brandUserID)

	req := httptest.NewRequest("GET", "/brand/category-report/Beverages", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleCategoryReport(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["tier"] != "premium" {
		t.Errorf("expected tier=premium, got %v", body["tier"])
	}
	// Premium should have acceptance rate fields
	if _, ok := body["avg_acceptance_rate"]; !ok {
		t.Error("expected avg_acceptance_rate in premium report")
	}
}

// --- handleBrandDeals full CRUD ---

func TestBrandDeals_CreateAndGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "crud@brand.com", "password1234", "CrudCo")

	// Create a deal
	dealID := createTestDeal(t, server, cookie)
	if dealID == 0 {
		t.Fatal("expected non-zero deal ID")
	}

	// Get deals
	rr := doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var deals []interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 1 {
		t.Errorf("expected 1 deal, got %d", len(deals))
	}
}

func TestBrandDeals_DeleteAndVerify(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "del@brand.com", "password1234", "DelCo")

	dealID := createTestDeal(t, server, cookie)

	// Delete via PUT with status=deleted
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/brand/deals/%d", dealID), nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleBrandDeals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get deals - should be empty (deleted deals excluded)
	rr = doRequest(server.handleBrandDeals, "GET", "/brand/deals", nil, []*http.Cookie{cookie})
	var deals []interface{}
	json.Unmarshal(rr.Body.Bytes(), &deals)
	if len(deals) != 0 {
		t.Errorf("expected 0 deals after delete, got %d", len(deals))
	}
}

func TestBrandDeals_UpdateAndVerify(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "upd@brand.com", "password1234", "UpdCo")

	dealID := createTestDeal(t, server, cookie)

	// Update status to paused
	status := "paused"
	body, _ := json.Marshal(map[string]interface{}{"status": status})
	req := httptest.NewRequest("PUT", fmt.Sprintf("/brand/deals/%d", dealID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleBrandDeals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify status changed
	var dbStatus string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&dbStatus)
	if dbStatus != "paused" {
		t.Errorf("expected status=paused, got %s", dbStatus)
	}
}

func TestBrandPortal_LogoUploadAndServe(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "logo@brand.com", "password1234", "LogoCo")

	// Upload a logo (1x1 red PNG as base64 data URI)
	pngBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
	dataURI := "data:image/png;base64," + pngBase64
	rr := doRequest(server.handleBrandLogoUpload, "POST", "/brand/logo", map[string]string{
		"image": dataURI,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Logo upload: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify has_logo in /brand/me
	rr = doRequest(server.handleBrandMe, "GET", "/brand/me", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Brand me: expected 200, got %d", rr.Code)
	}
	meBody := parseJSON(rr)
	if meBody["has_logo"] != true {
		t.Errorf("expected has_logo=true, got %v", meBody["has_logo"])
	}

	// Serve the logo
	mux := http.NewServeMux()
	mux.HandleFunc("/brand/logo/", server.handleBrandLogoServe)
	req := httptest.NewRequest("GET", fmt.Sprintf("/brand/logo/%d", brandUserID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Logo serve: expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty logo body")
	}

	// Serve non-existent logo
	req = httptest.NewRequest("GET", "/brand/logo/99999", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Non-existent logo: expected 404, got %d", w.Code)
	}
}

func TestBrandPortal_LogoUpload_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandLogoUpload, "POST", "/brand/logo", map[string]string{
		"image": "data:image/png;base64,abc",
	}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Logo upload without auth: expected 401, got %d", rr.Code)
	}
}

func TestBrandPortal_LogoUpload_TooLarge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "big@brand.com", "password1234", "BigCo")

	// Create a large base64 string (>500KB decoded)
	bigData := strings.Repeat("A", 700*1024) // ~525KB when decoded
	rr := doRequest(server.handleBrandLogoUpload, "POST", "/brand/logo", map[string]string{
		"image": "data:image/png;base64," + bigData,
	}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Large logo: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// handleBrandFeedbackSubmit TESTS
// =============================================================================

func TestBrandFeedbackSubmit_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body":    "Please add dark mode to the brand portal",
		"type":    "feature",
		"urgency": "normal",
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	if success, _ := resp["success"].(bool); !success {
		t.Error("Expected success=true")
	}
	if id, _ := resp["id"].(float64); id == 0 {
		t.Error("Expected non-zero feedback id")
	}
}

func TestBrandFeedbackSubmit_EmptyBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body": "",
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestBrandFeedbackSubmit_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body": "test feedback",
	}, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestBrandFeedbackSubmit_DefaultsType(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, brandUserID := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	rr := doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body":    "Some feedback",
		"type":    "invalid_type",
		"urgency": "invalid_urgency",
	}, []*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify defaults were applied
	var feedbackType, urgency string
	server.db.QueryRow("SELECT type, urgency FROM brand_feedback_requests WHERE brand_user_id = ?", brandUserID).Scan(&feedbackType, &urgency)
	if feedbackType != "feature" {
		t.Errorf("Expected type 'feature', got %q", feedbackType)
	}
	if urgency != "normal" {
		t.Errorf("Expected urgency 'normal', got %q", urgency)
	}
}

// =============================================================================
// handleBrandFeedbackList TESTS
// =============================================================================

func TestBrandFeedbackList_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	rr := doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("Expected 0 feedback items, got %d", len(items))
	}
}

func TestBrandFeedbackList_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	// Submit two feedback items
	doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body": "Feedback 1",
	}, []*http.Cookie{cookie})
	doRequest(server.handleBrandFeedback, "POST", "/brand/feedback", map[string]string{
		"body": "Feedback 2",
	}, []*http.Cookie{cookie})

	// List them
	rr := doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("Expected 2 feedback items, got %d", len(items))
	}
}

func TestBrandFeedbackList_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleBrandFeedback, "GET", "/brand/feedback", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestBrandFeedback_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "test@brand.com", "password123!", "TestCo")

	rr := doRequest(server.handleBrandFeedback, "DELETE", "/brand/feedback", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

// TestBrandPortal_DepositCheckoutFormCarriesSourceMetadata: CLAUDE.md Revenue
// Rules require source: "pennyai" on all charges. The Checkout Session set only
// brand_user_id, so attribution was broken for every real deposit.
func TestBrandPortal_DepositCheckoutFormCarriesSourceMetadata(t *testing.T) {
	form := buildDepositCheckoutForm("42", 25000, "https://brands.groflo.io")

	if got := form.Get("payment_intent_data[metadata][source]"); got != "pennyai" {
		t.Errorf("payment_intent_data[metadata][source] = %q, want \"pennyai\"", got)
	}
	if got := form.Get("payment_intent_data[metadata][brand_user_id]"); got != "42" {
		t.Errorf("brand_user_id metadata = %q, want \"42\"", got)
	}

	// The encoded body is what Stripe actually receives.
	encoded := form.Encode()
	for _, want := range []string{
		"payment_intent_data%5Bmetadata%5D%5Bsource%5D=pennyai",
		"payment_intent_data%5Bmetadata%5D%5Bbrand_user_id%5D=42",
		"mode=payment",
		"line_items%5B0%5D%5Bprice_data%5D%5Bunit_amount%5D=25000",
	} {
		if !strings.Contains(encoded, want) {
			t.Errorf("encoded form body missing %q\ngot: %s", want, encoded)
		}
	}

	// Redirects must go back to the brand portal.
	if got := form.Get("success_url"); got != "https://brands.groflo.io?deposit=success" {
		t.Errorf("success_url = %q", got)
	}
	if got := form.Get("cancel_url"); got != "https://brands.groflo.io?deposit=cancel" {
		t.Errorf("cancel_url = %q", got)
	}
}
