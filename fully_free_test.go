package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// FULLY FREE TRIAL CAMPAIGN TESTS
// =============================================================================

// createFullyFreeDeal is a test helper that inserts a fully_free deal directly into the DB.
func createFullyFreeDeal(t *testing.T, server *Server, offerBrand, offerProduct, competitor string, maxRebateCents, budgetCents int) int {
	t.Helper()
	result, err := server.db.Exec(`
		INSERT INTO brand_deals (
			competitor_product, offer_brand, offer_product, discount_cents,
			total_budget_cents, status, deal_type, max_rebate_cents,
			platform_fee_cents, require_verified_identity, receipt_grace_days, upc
		) VALUES (?, ?, ?, ?, ?, 'active', 'fully_free', ?, 100, 1, 7, '012345678901')`,
		competitor, offerBrand, offerProduct, maxRebateCents,
		budgetCents, maxRebateCents,
	)
	if err != nil {
		t.Fatalf("createFullyFreeDeal: %v", err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func TestFullyFree_AcceptNoBudgetDebit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Create user with phone (verified identity) and old enough account (>72h)
	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create a fully_free deal with $5 max rebate and $50 budget
	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Organic Milk", "milk", 500, 5000)

	// Create a list item that matches
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Milk', 'Dairy')")

	// Accept the deal
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Action != "swap_accepted" {
		t.Errorf("expected action swap_accepted, got %s", resp.Action)
	}

	// Key assertion: budget should NOT be debited on accept for fully_free
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 0 {
		t.Errorf("expected budget_spent_cents 0 (no debit on accept for fully_free), got %d", spent)
	}

	// Verify rebate was created with 'pending_purchase' status
	var rebateStatus string
	var rebateAmount int
	server.db.QueryRow("SELECT status, amount_cents FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateStatus, &rebateAmount)
	if rebateStatus != "pending_purchase" {
		t.Errorf("expected rebate status 'pending_purchase', got %q", rebateStatus)
	}
	if rebateAmount != 500 {
		t.Errorf("expected rebate amount 500 (max_rebate_cents), got %d", rebateAmount)
	}

	// Verify the message mentions "FREE"
	if resp.Message == "" {
		t.Error("expected non-empty response message")
	}
}

func TestFullyFree_ReceiptVerify_BudgetDebit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Organic Milk", "milk", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Milk', 'Dairy')")

	// Accept the deal first
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr.Code != 201 {
		t.Fatalf("accept: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get the rebate ID
	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)

	// Verify the receipt — submit with receipt price matching max rebate
	rr = doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 499}, cookies)

	if rr.Code != 200 {
		t.Fatalf("verify: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["success"] != true {
		t.Errorf("expected success=true, got %v", body["success"])
	}

	// Verified amount should be MIN(receipt_price, max_rebate) = 499
	verifiedCents := int(body["verified_amount_cents"].(float64))
	if verifiedCents != 499 {
		t.Errorf("expected verified_amount_cents 499, got %d", verifiedCents)
	}

	// Budget should now be debited: 499 (rebate) + 100 (platform fee) = 599
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 599 {
		t.Errorf("expected budget_spent_cents 599 (499 rebate + 100 fee), got %d", spent)
	}

	// Rebate status should be 'verified'
	var status string
	server.db.QueryRow("SELECT status FROM swap_rebates WHERE id = ?", rebateID).Scan(&status)
	if status != "verified" {
		t.Errorf("expected rebate status 'verified', got %q", status)
	}
}

// acceptedFullyFreeRebate sets a user up with one accepted fully_free deal and
// returns the deal id, the pending_purchase rebate id, and the user's cookies.
func acceptedFullyFreeRebate(t *testing.T, server *Server, maxRebateCents, budgetCents int) (int, int, []*http.Cookie) {
	t.Helper()
	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Organic Milk", "milk", maxRebateCents, budgetCents)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Milk', 'Dairy')")

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr.Code != 201 {
		t.Fatalf("accept: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)
	return dealID, rebateID, cookies
}

// TestFullyFree_ConcurrentReceiptVerifyDebitsOnce fires simultaneous receipt
// verifications for one rebate. The status read happened before tx.Begin and the
// final UPDATE swap_rebates had no status guard, so each concurrent verify ran
// the budget debit — the debit is individually atomic, so they all succeeded and
// the brand was charged rebate + platform fee more than once for one rebate.
func TestFullyFree_ConcurrentReceiptVerifyDebitsOnce(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	dealID, rebateID, cookies := acceptedFullyFreeRebate(t, server, 500, 50000)

	const n = 4 // the handler allows 5 verifications per user per day
	codes := make([]int, n)
	hammer(t, n, func(i int) {
		rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
			map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 499}, cookies)
		codes[i] = rr.Code
	})

	wins := 0
	for i, c := range codes {
		switch c {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
		default:
			t.Errorf("verify %d: unexpected status %d (want 200 for the winner, 409 for losers)", i, c)
		}
	}
	if wins != 1 {
		t.Errorf("verifications that succeeded: got %d, want exactly 1", wins)
	}

	// 499 rebate + 100 platform fee, charged once.
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 599 {
		t.Errorf("budget_spent_cents: got %d, want 599 (brand overcharged)", spent)
	}
}

// TestFullyFree_ReceiptVerifyAlreadyVerified is the deterministic partner to the
// hammer test: the rebate is already verified, so the handler must 409 and leave
// the brand's budget alone.
func TestFullyFree_ReceiptVerifyAlreadyVerified(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	dealID, rebateID, cookies := acceptedFullyFreeRebate(t, server, 500, 50000)

	// A prior verification already settled this rebate and debited the brand.
	server.db.Exec("UPDATE swap_rebates SET status = 'verified', verified_amount_cents = 499 WHERE id = ?", rebateID)
	server.db.Exec("UPDATE brand_deals SET budget_spent_cents = 599 WHERE id = ?", dealID)

	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 499}, cookies)
	if rr.Code != http.StatusConflict {
		t.Fatalf("re-verify: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 599 {
		t.Errorf("budget_spent_cents moved on a blocked re-verify: got %d, want 599", spent)
	}
}

// TestFullyFree_ReceiptVerifyRejectionDoesNotBurnRateLimit: a 403 or 404 must
// not consume one of the user's 5 daily verification attempts.
func TestFullyFree_ReceiptVerifyRejectionDoesNotBurnRateLimit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	dealID, rebateID, cookies := acceptedFullyFreeRebate(t, server, 500, 50000)

	// Someone else's rebate: 403, and must cost nothing.
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-2', '5559998888')")
	server.db.Exec("UPDATE swap_rebates SET user_id = 'user-2' WHERE id = ?", rebateID)
	for i := 0; i < 6; i++ {
		rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
			map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 499}, cookies)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("attempt %d on another user's rebate: expected 403, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// A nonexistent rebate: 404, likewise free.
	for i := 0; i < 6; i++ {
		rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
			map[string]interface{}{"rebate_id": 999999, "receipt_price_cents": 499}, cookies)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("attempt %d on a missing rebate: expected 404, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// The user's own genuine verification must still go through.
	server.db.Exec("UPDATE swap_rebates SET user_id = 'user-1' WHERE id = ?", rebateID)
	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 499}, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("genuine verify after 12 rejected attempts: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 599 {
		t.Errorf("budget_spent_cents: got %d, want 599", spent)
	}
}

func TestFullyFree_DynamicRebate_ReceiptPriceLessThanMax(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Max rebate is $8.00 but receipt price will be $5.99
	dealID := createFullyFreeDeal(t, server, "Fancy", "Fancy Oat Milk", "oat milk", 800, 10000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Oat Milk', 'Dairy')")

	// Accept
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)

	// Verify with lower receipt price
	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 599}, cookies)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	verifiedCents := int(body["verified_amount_cents"].(float64))
	if verifiedCents != 599 {
		t.Errorf("expected verified_amount_cents 599 (MIN of receipt price 599 and max 800), got %d", verifiedCents)
	}

	// Budget debit: 599 + 100 (platform fee) = 699
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 699 {
		t.Errorf("expected budget_spent_cents 699, got %d", spent)
	}
}

func TestFullyFree_DynamicRebate_ReceiptPriceHigherThanMax(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Max rebate is $5.00 but receipt price will be $7.99
	dealID := createFullyFreeDeal(t, server, "Fancy", "Fancy Organic Milk", "organic milk", 500, 10000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Organic Milk', 'Dairy')")

	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)

	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 799}, cookies)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	verifiedCents := int(body["verified_amount_cents"].(float64))
	// Should cap at max_rebate_cents (500), not use the higher receipt price
	if verifiedCents != 500 {
		t.Errorf("expected verified_amount_cents 500 (capped at max), got %d", verifiedCents)
	}
}

func TestFullyFree_FraudGate_72hCoolingPeriod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Create user whose account is only 1 hour old
	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-1 hours'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Free Product", "product", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")

	// Set up parseCommand to return the item
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Product", Department: "Other", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	// The deal should not be offered to a new account (< 72h)
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "product"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Check that no pending swap for the fully_free deal is offered
	if resp.PendingSwap != nil && resp.PendingSwap.DealID == dealID {
		t.Errorf("fully_free deal should NOT be offered to accounts < 72h old")
	}
}

func TestFullyFree_FraudGate_UnverifiedIdentity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Create user WITHOUT phone (no verified identity), account > 72h old
	server.db.Exec("INSERT INTO users (id, created_at) VALUES ('user-1', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "", "email")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Free Product", "product", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Product", Department: "Other", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "product"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.PendingSwap != nil && resp.PendingSwap.DealID == dealID {
		t.Errorf("fully_free deal should NOT be offered to unverified identity (no phone)")
	}
}

func TestFullyFree_FraudGate_HouseholdDedup(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	// Create household with two users
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO users (id, phone, household_id, created_at) VALUES ('user-1', '5551234567', 1, datetime('now', '-4 days'))")
	server.db.Exec("INSERT INTO users (id, phone, household_id, created_at) VALUES ('user-2', '5559876543', 1, datetime('now', '-4 days'))")

	token1, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	token2, _ := server.sessions.CreateSession("user-2", "5559876543", "phone")
	cookies1 := []*http.Cookie{{Name: "session", Value: token1}}
	cookies2 := []*http.Cookie{{Name: "session", Value: token2}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Free Product", "product", 500, 5000)

	// User 1 accepts the deal
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies1)
	if rr.Code != 201 {
		t.Fatalf("user1 accept: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// User 2 should NOT be offered the same fully_free deal (household dedup)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (2, 'user-2', 'Product', 'Other')")
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Product", Department: "Other", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr = doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "product"}, cookies2)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.PendingSwap != nil && resp.PendingSwap.DealID == dealID {
		t.Errorf("fully_free deal should NOT be offered to another household member (dedup)")
	}
}

func TestFullyFree_VerifyWrongUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-2', '5559876543', datetime('now', '-4 days'))")
	token1, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	token2, _ := server.sessions.CreateSession("user-2", "5559876543", "phone")
	cookies1 := []*http.Cookie{{Name: "session", Value: token1}}
	cookies2 := []*http.Cookie{{Name: "session", Value: token2}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Organic Milk", "milk", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Milk', 'Dairy')")

	// User 1 accepts
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies1)

	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)

	// User 2 tries to verify user 1's rebate — should be forbidden
	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 500}, cookies2)

	if rr.Code != 403 {
		t.Errorf("expected 403 for wrong user, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFullyFree_VerifyAlreadyVerified(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Organic Milk", "milk", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Milk', 'Dairy')")

	// Accept
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	var rebateID int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ?", dealID).Scan(&rebateID)

	// First verification succeeds
	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 500}, cookies)
	if rr.Code != 200 {
		t.Fatalf("first verify: expected 200, got %d", rr.Code)
	}

	// Second verification should fail (already verified). 409, not 400: the
	// request conflicts with the rebate's current state, and this matches the
	// swap-accept path's "already accepted" conflict.
	rr = doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID, "receipt_price_cents": 500}, cookies)
	if rr.Code != http.StatusConflict {
		t.Errorf("double verify: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFullyFree_BudgetExhaustedOnVerify(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-2', '5559876543', datetime('now', '-4 days'))")
	token1, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	token2, _ := server.sessions.CreateSession("user-2", "5559876543", "phone")
	cookies1 := []*http.Cookie{{Name: "session", Value: token1}}
	cookies2 := []*http.Cookie{{Name: "session", Value: token2}}

	// Create deal with tiny budget: only enough for 1 verification ($5 rebate + $1 fee = $6)
	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Product", "product", 500, 600)

	// User 1 accepts and verifies
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies1)

	var rebateID1 int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ? AND user_id = 'user-1'", dealID).Scan(&rebateID1)

	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID1, "receipt_price_cents": 500}, cookies1)
	if rr.Code != 200 {
		t.Fatalf("user1 verify: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Deal should be auto-paused now
	var dealStatus string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&dealStatus)
	if dealStatus != "paused" {
		t.Errorf("expected deal status 'paused' after budget exhausted, got %q", dealStatus)
	}

	// User 2 accepts (deal should already be paused, but let's force the accept to test verify)
	server.db.Exec("UPDATE brand_deals SET status = 'active' WHERE id = ?", dealID) // Un-pause for test
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (2, 'user-2', 'Product', 'Other')")
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 2}, cookies2)

	var rebateID2 int
	server.db.QueryRow("SELECT id FROM swap_rebates WHERE deal_id = ? AND user_id = 'user-2'", dealID).Scan(&rebateID2)

	// User 2's verification should fail because budget is exhausted
	rr = doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": rebateID2, "receipt_price_cents": 500}, cookies2)
	if rr.Code != 400 {
		t.Errorf("user2 verify should fail with budget exhausted, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFullyFree_PendingRebatesEndpoint(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Product", "product", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")

	// Accept
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	// Get pending rebates
	rr := doRequest(server.handleFullyFreeRebates, "GET", "/api/rebates/pending", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	rebates := body["rebates"].([]interface{})
	if len(rebates) != 1 {
		t.Fatalf("expected 1 rebate, got %d", len(rebates))
	}
	rebate := rebates[0].(map[string]interface{})
	if rebate["status"] != "pending_purchase" {
		t.Errorf("expected status pending_purchase, got %v", rebate["status"])
	}
	if rebate["offer_brand"] != "Acme" {
		t.Errorf("expected offer_brand Acme, got %v", rebate["offer_brand"])
	}
}

func TestFullyFree_EarningsShowNewStatuses(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	dealID := createFullyFreeDeal(t, server, "Acme", "Acme Product", "product", 500, 5000)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 'user-1', 'Product', 'Other')")

	// Accept
	doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	// Earnings should show the pending_purchase rebate
	rr := doRequest(server.handleEarnings, "GET", "/api/earnings", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var earnings EarningsResponse
	json.NewDecoder(rr.Body).Decode(&earnings)
	if len(earnings.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(earnings.Transactions))
	}
	if earnings.Transactions[0].Status != "pending_purchase" {
		t.Errorf("expected status pending_purchase, got %q", earnings.Transactions[0].Status)
	}
	if earnings.Transactions[0].DealType != "fully_free" {
		t.Errorf("expected deal_type fully_free, got %q", earnings.Transactions[0].DealType)
	}
	// pending_purchase should count toward pending
	if earnings.PendingCents != 500 {
		t.Errorf("expected pending_cents 500, got %d", earnings.PendingCents)
	}
}

func TestFullyFree_BrandPortalCreate(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "brand@example.com", "password123", "FreeBrand Co")

	// Create a fully_free deal via brand portal
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "generic milk",
		"offer_brand":        "FreeBrand",
		"offer_product":      "FreeBrand Organic Milk",
		"deal_type":          "fully_free",
		"max_rebate_cents":   599,
		"daily_budget_cents": 10000,
		"upc":                "012345678901",
		"why_bullets":        `["100% organic", "Farm fresh"]`,
	}, []*http.Cookie{cookie})

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	dealID := int(body["id"].(float64))
	if dealID == 0 {
		t.Error("expected non-zero deal ID")
	}
	// Verify response indicates pending review
	if body["status"] != "pending_review" {
		t.Errorf("expected response status 'pending_review', got %q", body["status"])
	}

	// Verify the deal was stored correctly
	var dealType, upc, dealStatus, reviewStatus string
	var maxRebate, platformFee, requireVerified, graceDays int
	server.db.QueryRow(
		"SELECT deal_type, max_rebate_cents, platform_fee_cents, require_verified_identity, receipt_grace_days, upc, status, COALESCE(review_status,'') FROM brand_deals WHERE id = ?",
		dealID,
	).Scan(&dealType, &maxRebate, &platformFee, &requireVerified, &graceDays, &upc, &dealStatus, &reviewStatus)

	if dealStatus != "pending_review" {
		t.Errorf("expected deal status 'pending_review', got %q", dealStatus)
	}
	if reviewStatus != "pending" {
		t.Errorf("expected review_status 'pending', got %q", reviewStatus)
	}

	if dealType != "fully_free" {
		t.Errorf("expected deal_type fully_free, got %q", dealType)
	}
	if maxRebate != 599 {
		t.Errorf("expected max_rebate_cents 599, got %d", maxRebate)
	}
	if platformFee != 100 {
		t.Errorf("expected platform_fee_cents 100 (default), got %d", platformFee)
	}
	if requireVerified != 1 {
		t.Errorf("expected require_verified_identity 1, got %d", requireVerified)
	}
	if graceDays != 7 {
		t.Errorf("expected receipt_grace_days 7, got %d", graceDays)
	}
	if upc != "012345678901" {
		t.Errorf("expected upc '012345678901', got %q", upc)
	}
}

func TestFullyFree_BrandPortalValidation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "brand2@example.com", "password123", "Brand2 Co")

	// Missing max_rebate_cents — should fail
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "generic milk",
		"offer_brand":        "Brand2",
		"offer_product":      "Brand2 Milk",
		"deal_type":          "fully_free",
		"daily_budget_cents": 10000,
		"upc":                "012345678901",
	}, []*http.Cookie{cookie})
	if rr.Code != 400 {
		t.Errorf("expected 400 for missing max_rebate_cents, got %d: %s", rr.Code, rr.Body.String())
	}

	// Missing UPC — should fail
	rr = doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "generic milk",
		"offer_brand":        "Brand2",
		"offer_product":      "Brand2 Milk",
		"deal_type":          "fully_free",
		"max_rebate_cents":   500,
		"daily_budget_cents": 10000,
	}, []*http.Cookie{cookie})
	if rr.Code != 400 {
		t.Errorf("expected 400 for missing UPC, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFullyFree_VerifyMissingRebateID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"receipt_price_cents": 500}, cookies)
	if rr.Code != 400 {
		t.Errorf("expected 400 for missing rebate_id, got %d", rr.Code)
	}
}

func TestFullyFree_VerifyNonexistentRebate(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})

	server.db.Exec("INSERT INTO users (id, phone, created_at) VALUES ('user-1', '5551234567', datetime('now', '-4 days'))")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleFullyFreeReceiptVerify, "POST", "/api/receipt/verify",
		map[string]interface{}{"rebate_id": 99999, "receipt_price_cents": 500}, cookies)
	if rr.Code != 404 {
		t.Errorf("expected 404 for nonexistent rebate, got %d", rr.Code)
	}
}

// =============================================================================
// CAMPAIGN REVIEW TESTS
// =============================================================================

func TestCampaignReview_DealStartsPendingReview(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "review@brand.com", "password123", "ReviewCo")

	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "ReviewBrand",
		"offer_product":      "ReviewBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{cookie})

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["status"] != "pending_review" {
		t.Errorf("expected response status 'pending_review', got %v", body["status"])
	}

	// Verify DB state
	dealID := int(body["id"].(float64))
	var status, reviewStatus string
	server.db.QueryRow("SELECT status, COALESCE(review_status,'') FROM brand_deals WHERE id = ?", dealID).Scan(&status, &reviewStatus)
	if status != "pending_review" {
		t.Errorf("expected DB status 'pending_review', got %q", status)
	}
	if reviewStatus != "pending" {
		t.Errorf("expected review_status 'pending', got %q", reviewStatus)
	}
}

func TestCampaignReview_AdminApprove(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	brandCookie, _ := brandRegister(t, server, "approve@brand.com", "password123", "ApproveCo")

	// Create deal (starts as pending_review)
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "ApproveBrand",
		"offer_product":      "ApproveBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{brandCookie})
	dealID := int(parseJSON(rr)["id"].(float64))

	// Admin approves via mux-based routing
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/deals/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/review") {
			server.handleAdminDealReview(w, r)
			return
		}
		server.handleAdminDeals(w, r)
	})

	adminCookie := loginAdmin(t, server)
	rr = doRequestMux(mux, "POST", fmt.Sprintf("/api/admin/deals/%d/review", dealID),
		map[string]interface{}{"action": "approve", "note": "Looks good"},
		[]*http.Cookie{adminCookie})

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deal is now active
	var status, reviewStatus, reviewNote, reviewedBy string
	server.db.QueryRow("SELECT status, COALESCE(review_status,''), COALESCE(review_note,''), COALESCE(reviewed_by,'') FROM brand_deals WHERE id = ?", dealID).Scan(&status, &reviewStatus, &reviewNote, &reviewedBy)
	if status != "active" {
		t.Errorf("expected status 'active' after approval, got %q", status)
	}
	if reviewStatus != "approved" {
		t.Errorf("expected review_status 'approved', got %q", reviewStatus)
	}
	if reviewNote != "Looks good" {
		t.Errorf("expected review_note 'Looks good', got %q", reviewNote)
	}
}

func TestCampaignReview_AdminReject(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	brandCookie, _ := brandRegister(t, server, "reject@brand.com", "password123", "RejectCo")

	// Create deal
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "RejectBrand",
		"offer_product":      "RejectBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{brandCookie})
	dealID := int(parseJSON(rr)["id"].(float64))

	// Admin rejects
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/deals/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/review") {
			server.handleAdminDealReview(w, r)
			return
		}
		server.handleAdminDeals(w, r)
	})

	adminCookie := loginAdmin(t, server)
	rr = doRequestMux(mux, "POST", fmt.Sprintf("/api/admin/deals/%d/review", dealID),
		map[string]interface{}{"action": "reject", "note": "Violates ad policy"},
		[]*http.Cookie{adminCookie})

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status, reviewStatus, reviewNote string
	server.db.QueryRow("SELECT status, COALESCE(review_status,''), COALESCE(review_note,'') FROM brand_deals WHERE id = ?", dealID).Scan(&status, &reviewStatus, &reviewNote)
	if status != "rejected" {
		t.Errorf("expected status 'rejected', got %q", status)
	}
	if reviewNote != "Violates ad policy" {
		t.Errorf("expected review_note 'Violates ad policy', got %q", reviewNote)
	}
}

func TestCampaignReview_BrandCannotSelfApprove(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "selfapprove@brand.com", "password123", "SelfCo")

	// Create deal
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "SelfBrand",
		"offer_product":      "SelfBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{cookie})
	dealID := int(parseJSON(rr)["id"].(float64))

	// Brand tries to set status to 'active' — should be blocked
	activeStatus := "active"
	rr = doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID),
		map[string]interface{}{"status": &activeStatus},
		[]*http.Cookie{cookie})

	if rr.Code != 403 {
		t.Errorf("expected 403 for self-approval, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCampaignReview_ApprovedDealCanPauseResume(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := brandRegister(t, server, "pauseresume@brand.com", "password123", "PauseCo")

	// Create and auto-approve a deal via helper
	dealID := createTestDeal(t, server, cookie)

	// Pause should work
	pauseStatus := "paused"
	rr := doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID),
		map[string]interface{}{"status": &pauseStatus},
		[]*http.Cookie{cookie})
	if rr.Code != 200 {
		t.Fatalf("expected 200 for pause, got %d: %s", rr.Code, rr.Body.String())
	}

	// Resume should work because deal is approved
	activeStatus := "active"
	rr = doRequest(server.handleBrandDeals, "PUT", fmt.Sprintf("/brand/deals/%d", dealID),
		map[string]interface{}{"status": &activeStatus},
		[]*http.Cookie{cookie})
	if rr.Code != 200 {
		t.Fatalf("expected 200 for resume of approved deal, got %d: %s", rr.Code, rr.Body.String())
	}

	var status string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&status)
	if status != "active" {
		t.Errorf("expected status 'active' after resume, got %q", status)
	}
}

func TestCampaignReview_InvalidAction(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	brandCookie, _ := brandRegister(t, server, "invalid@brand.com", "password123", "InvalidCo")

	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "InvalidBrand",
		"offer_product":      "InvalidBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{brandCookie})
	dealID := int(parseJSON(rr)["id"].(float64))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/deals/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/review") {
			server.handleAdminDealReview(w, r)
			return
		}
		server.handleAdminDeals(w, r)
	})

	adminCookie := loginAdmin(t, server)
	rr = doRequestMux(mux, "POST", fmt.Sprintf("/api/admin/deals/%d/review", dealID),
		map[string]interface{}{"action": "maybe"},
		[]*http.Cookie{adminCookie})

	if rr.Code != 400 {
		t.Errorf("expected 400 for invalid action, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCampaignReview_CannotReviewActiveDeal(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	brandCookie, _ := brandRegister(t, server, "alreadyactive@brand.com", "password123", "ActiveCo")

	// Create and auto-approve a deal
	dealID := createTestDeal(t, server, brandCookie)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/deals/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/review") {
			server.handleAdminDealReview(w, r)
			return
		}
		server.handleAdminDeals(w, r)
	})

	adminCookie := loginAdmin(t, server)
	rr := doRequestMux(mux, "POST", fmt.Sprintf("/api/admin/deals/%d/review", dealID),
		map[string]interface{}{"action": "approve"},
		[]*http.Cookie{adminCookie})

	if rr.Code != 400 {
		t.Errorf("expected 400 for reviewing already-active deal, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCampaignReview_PendingReviewDealNotMatchedToShoppers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	brandCookie, _ := brandRegister(t, server, "nomatch@brand.com", "password123", "NoMatchCo")

	// Create deal — stays as pending_review (DO NOT auto-approve)
	rr := doRequest(server.handleBrandDeals, "POST", "/brand/deals", map[string]interface{}{
		"competitor_product": "Store Cola",
		"offer_brand":        "NoMatchBrand",
		"offer_product":      "NoMatchBrand Cola",
		"discount_cents":     150,
		"daily_budget_cents": 5000,
	}, []*http.Cookie{brandCookie})
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	// Verify deal is pending_review in DB
	dealID := int(parseJSON(rr)["id"].(float64))
	var status string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&status)
	if status != "pending_review" {
		t.Fatalf("expected pending_review, got %q", status)
	}

	// Verify deal does NOT appear in active deal matching query
	var activeCount int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE id = ? AND status = 'active'", dealID).Scan(&activeCount)
	if activeCount != 0 {
		t.Error("pending_review deal should not match as active")
	}
}

// Suppress unused import warnings
var _ = fmt.Sprintf
var _ = time.Now
