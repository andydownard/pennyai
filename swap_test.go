package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)


// =============================================================================
// PENNY SWAPS TESTS
// =============================================================================

func TestSwaps_DealMatchingBasic(t *testing.T) {
	server := newTestServer(t)

	// Create user and session
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Use seeded skippy→jif deal: add a Skippy item and verify swap is offered
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added peanut butter!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap for skippy peanut butter")
	}
	if resp.PendingSwap.OfferBrand != "Jif" {
		t.Errorf("expected offer brand Jif, got %s", resp.PendingSwap.OfferBrand)
	}
	if resp.PendingSwap.DiscountCents != 125 {
		t.Errorf("expected 125 discount cents, got %d", resp.PendingSwap.DiscountCents)
	}

	// Verify swap_events logged
	var eventCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'offered'").Scan(&eventCount)
	if eventCount != 1 {
		t.Errorf("expected 1 offered event, got %d", eventCount)
	}
}

func TestSwaps_AcceptSwap(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create item matching seeded skippy→jif deal (deal ID 1)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	// Get the seeded deal ID for skippy
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

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

	// Verify item was updated to Jif
	var name, brand string
	server.db.QueryRow("SELECT name, brand FROM list_items WHERE id = 1").Scan(&name, &brand)
	if name != "Creamy Peanut Butter" || brand != "Jif" {
		t.Errorf("expected Jif Creamy Peanut Butter, got %s %s", brand, name)
	}

	// Verify accepted event logged
	var eventCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'accepted'").Scan(&eventCount)
	if eventCount != 1 {
		t.Errorf("expected 1 accepted event, got %d", eventCount)
	}

	// Verify budget decremented
	var spent int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&spent)
	if spent != 125 {
		t.Errorf("expected budget_spent_cents 125, got %d", spent)
	}
}

func TestSwaps_DeclineAndSuppression(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	// Get the seeded deal ID for skippy
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	// Decline 3 times
	for i := 0; i < 3; i++ {
		rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
			map[string]interface{}{"text": "decline_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
		if rr.Code != 201 {
			t.Fatalf("decline %d: expected 201, got %d", i+1, rr.Code)
		}
	}

	// Verify suppression count is 3
	var declineCount int
	server.db.QueryRow("SELECT decline_count FROM swap_suppressions WHERE user_id = 'user-1' AND deal_id = ?", dealID).Scan(&declineCount)
	if declineCount != 3 {
		t.Errorf("expected decline_count 3, got %d", declineCount)
	}

	// Now add a matching item — deal should NOT appear due to suppression
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added peanut butter!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap != nil {
		t.Error("expected no swap offer after 3 declines (suppressed)")
	}
}

func TestSwaps_SliderConvenienceBlocksDeals(t *testing.T) {
	server := newTestServer(t)

	// Create user with slider = 100 (convenience, never show deals)
	server.db.Exec("INSERT INTO users (id, phone, penny_slider, penny_autotune) VALUES ('user-1', '5551234567', 100, 0)")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Use seeded deals — add a skippy item with slider=100, should get no swap
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap != nil {
		t.Error("expected no swap when slider is at 100 (convenience)")
	}
}

func TestSwaps_BudgetExhaustedAutopauses(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create a deal with tight budget: discount = budget, so one accept exhausts it
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Ketchup', 'Heinz', 'Pantry & Dry Goods')")
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES ('heinz', 'Hunts', 'Tomato Ketchup', 200, 200, 'active')`)

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'heinz'").Scan(&dealID)

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	// Verify deal was auto-paused
	var status string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&status)
	if status != "paused" {
		t.Errorf("expected deal status 'paused', got %q", status)
	}
}

func TestSwaps_FrequencyCapLimitsPendingOffers(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Add 4 different items with matching deals via parse, each should trigger a swap offer
	// But with the frequency cap of 3, the 4th should NOT get a swap offer
	products := []struct{ name, brand string }{
		{"Peanut Butter", "Skippy"},
		{"Sparkling Water", "LaCroix"},
		{"Laundry Detergent", "Tide"},
		{"Marinara Sauce", "Ragu"},
	}

	swapCount := 0
	for i, p := range products {
		server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
			return &ParsedCommand{
				Action:  "add",
				Items:   []ParsedItem{{Name: p.name, Brand: p.brand, Department: "Pantry & Dry Goods", Quantity: 1}},
				Message: "Added!",
			}, nil
		}

		rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
			map[string]string{"text": p.name}, cookies)

		if rr.Code != 201 {
			t.Fatalf("item %d: expected 201, got %d: %s", i, rr.Code, rr.Body.String())
		}

		var resp ParseResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.PendingSwap != nil {
			swapCount++
		}
	}

	// Expect exactly 3 swap offers (capped), not 4
	if swapCount != 3 {
		t.Errorf("expected 3 swap offers (frequency cap), got %d", swapCount)
	}
}

func TestSwaps_FrequencyCapResetsAfterDecline(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Add 3 items to hit the cap
	products := []struct{ name, brand string }{
		{"Peanut Butter", "Skippy"},
		{"Sparkling Water", "LaCroix"},
		{"Laundry Detergent", "Tide"},
	}

	var lastDealID, lastItemID int
	for _, p := range products {
		server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
			return &ParsedCommand{
				Action: "add",
				Items:  []ParsedItem{{Name: p.name, Brand: p.brand, Department: "Pantry & Dry Goods", Quantity: 1}},
			}, nil
		}

		rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
			map[string]string{"text": p.name}, cookies)
		if rr.Code != 201 {
			t.Fatalf("expected 201 for %s, got %d", p.name, rr.Code)
		}
		var resp ParseResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.PendingSwap != nil {
			lastDealID = resp.PendingSwap.DealID
			lastItemID = resp.PendingSwap.ItemID
		}
	}

	// Decline one of the pending swaps to free up a slot
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "decline_swap", "swap_deal_id": lastDealID, "swap_item_id": lastItemID}, cookies)
	if rr.Code != 201 {
		t.Fatalf("decline expected 201, got %d", rr.Code)
	}

	// Now add a 4th item — should get a swap offer since we freed a slot
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action: "add",
			Items:  []ParsedItem{{Name: "Marinara Sauce", Brand: "Ragu", Department: "Pantry & Dry Goods", Quantity: 1}},
		}, nil
	}

	rr = doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "marinara sauce"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Error("expected a swap offer after declining one (freed a slot), got nil")
	}
}

// =============================================================================
// DONE SHOPPING TESTS
// =============================================================================

func TestDoneShopping_Action(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "done_shopping",
			Items:   []ParsedItem{},
			Message: "Nice trip! Snap your receipt and I'll check off what you got.",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "I'm done shopping"}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Action != "done_shopping" {
		t.Errorf("expected action done_shopping, got %s", resp.Action)
	}
	if resp.Message == "" {
		t.Error("expected a non-empty message")
	}
}

// =============================================================================
// RECEIPT TESTS
// =============================================================================
// ADMIN DEALS TESTS
// =============================================================================

func TestAdminDeals_CRUD(t *testing.T) {
	server := newTestServer(t)

	// Create admin user
	server.db.Exec("INSERT INTO users (id, phone, clerk_user_id) VALUES ('user-1', '3104861833', 'user_test_admin')")
	token, _ := server.sessions.CreateSession("user-1", "3104861833", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// POST — create deal
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "heinz ketchup",
			"offer_brand":       "Hunts",
			"offer_product":     "Tomato Ketchup",
			"discount_cents":    150,
			"total_budget_cents": 10000,
		}, cookies)
	if rr.Code != 201 {
		t.Fatalf("create deal: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var created BrandDeal
	json.NewDecoder(rr.Body).Decode(&created)
	if created.ID == 0 {
		t.Fatal("expected non-zero deal ID")
	}

	// GET — list deals (5 seeded + 1 created = 6)
	rr = doRequest(server.handleAdminDeals, "GET", "/api/admin/deals", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("list deals: expected 200, got %d", rr.Code)
	}
	var deals []BrandDeal
	json.NewDecoder(rr.Body).Decode(&deals)
	if len(deals) != 9 {
		t.Fatalf("expected 9 deals (5 sku_swap + 3 new types + 1 created), got %d", len(deals))
	}

	// PUT — update deal
	rr = doRequest(server.handleAdminDeals, "PUT", "/api/admin/deals",
		map[string]interface{}{
			"id":                 created.ID,
			"competitor_product": "heinz ketchup",
			"offer_brand":       "Hunts",
			"offer_product":     "Tomato Ketchup",
			"discount_cents":    200,
			"total_budget_cents": 10000,
			"status":            "paused",
		}, cookies)
	if rr.Code != 200 {
		t.Fatalf("update deal: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify update
	var discount int
	var status string
	server.db.QueryRow("SELECT discount_cents, status FROM brand_deals WHERE id = ?", created.ID).Scan(&discount, &status)
	if discount != 200 || status != "paused" {
		t.Errorf("expected discount 200 and status paused, got %d and %s", discount, status)
	}

	// DELETE — remove deal
	rr = doRequest(server.handleAdminDeals, "DELETE", "/api/admin/deals",
		map[string]interface{}{"id": created.ID}, cookies)
	if rr.Code != 200 {
		t.Fatalf("delete deal: expected 200, got %d", rr.Code)
	}

	// Soft-delete: deal still exists in DB but with status='deleted' (B12 fix)
	var deletedStatus string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", created.ID).Scan(&deletedStatus)
	if deletedStatus != "deleted" {
		t.Errorf("expected status='deleted' after soft-delete, got %q", deletedStatus)
	}

	// GET should not return deleted deals
	rr = doRequest(server.handleAdminDeals, "GET", "/api/admin/deals", nil, cookies)
	var remaining []BrandDeal
	json.NewDecoder(rr.Body).Decode(&remaining)
	if len(remaining) != 8 {
		t.Errorf("expected 8 deals in GET after soft-delete (8 seeded, 1 created then deleted), got %d", len(remaining))
	}
}

func TestAdminDeals_RequiresAdmin(t *testing.T) {
	server := newTestServer(t)

	// Non-admin user
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleAdminDeals, "GET", "/api/admin/deals", nil, cookies)
	if rr.Code != 403 {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}
}

func TestAdminDeals_RejectsNegativeValues(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone, clerk_user_id) VALUES ('user-1', '3104861833', 'user_test_admin')")
	token, _ := server.sessions.CreateSession("user-1", "3104861833", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Negative discount should be rejected
	rr := doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "Brand A Milk",
			"offer_brand":       "Brand B",
			"offer_product":     "Brand B Milk",
			"discount_cents":    -100,
		}, cookies)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("negative discount: got %d, want 400", rr.Code)
	}

	// Negative budget should be rejected
	rr = doRequest(server.handleAdminDeals, "POST", "/api/admin/deals",
		map[string]interface{}{
			"competitor_product": "Brand A Milk",
			"offer_brand":       "Brand B",
			"offer_product":     "Brand B Milk",
			"daily_budget_cents": -500,
		}, cookies)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("negative budget: got %d, want 400", rr.Code)
	}
}

func TestSwaps_RebateCreatedOnAccept(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify swap_rebates row exists with correct amount
	var rebateAmount int
	var rebateStatus string
	err := server.db.QueryRow("SELECT amount_cents, status FROM swap_rebates WHERE user_id = 'user-1' AND deal_id = ?", dealID).Scan(&rebateAmount, &rebateStatus)
	if err != nil {
		t.Fatalf("expected rebate row, got error: %v", err)
	}
	// Seeded skippy→jif deal has discount_cents=125
	if rebateAmount != 125 {
		t.Errorf("expected rebate amount 125, got %d", rebateAmount)
	}
	if rebateStatus != "pending" {
		t.Errorf("expected rebate status 'pending', got %q", rebateStatus)
	}

	// Verify user's swap_credits_cents was incremented
	var credits int
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	if credits != 125 {
		t.Errorf("expected swap_credits_cents 125, got %d", credits)
	}
}

func TestSwaps_RebateAppearsInSettings(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone, swap_credits_cents) VALUES ('user-1', '5551234567', 350)")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleSettings, "GET", "/api/settings", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	result := parseJSON(rr)
	if result["swap_credits_cents"] != float64(350) {
		t.Errorf("expected swap_credits_cents 350, got %v", result["swap_credits_cents"])
	}
}

func TestRebates_ReturnsHistory(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Insert swap events and rebates with different statuses. One campaign each:
	// a user gets a single accept per campaign (idx_swap_events_one_accept).
	// Deal ids come from the seeded deals (their numbering is not
	// guaranteed), so look them up rather than assuming 1 and 2.
	var jifDeal, otherDeal int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&jifDeal)
	server.db.QueryRow("SELECT id FROM brand_deals WHERE id != ? ORDER BY id LIMIT 1", jifDeal).Scan(&otherDeal)
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (1, 'user-1', ?, 'accepted')", jifDeal)
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (2, 'user-1', ?, 'accepted')", otherDeal)
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES ('user-1', 1, ?, 150, 'pending')", jifDeal)
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES ('user-1', 2, ?, 150, 'paid')", otherDeal)

	rr := doRequest(server.handleRebates, "GET", "/api/rebates", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	rebates := result["rebates"].([]interface{})
	if len(rebates) != 2 {
		t.Fatalf("expected 2 rebates, got %d", len(rebates))
	}

	// Verify summary totals
	if result["total_cents"] != float64(300) {
		t.Errorf("expected total_cents 300, got %v", result["total_cents"])
	}
	if result["pending_cents"] != float64(150) {
		t.Errorf("expected pending_cents 150, got %v", result["pending_cents"])
	}
	if result["paid_cents"] != float64(150) {
		t.Errorf("expected paid_cents 150, got %v", result["paid_cents"])
	}

	// Verify rebate entries have deal info
	// created_at ties at second resolution, so find the Jif entry
	// rather than assuming its position.
	var first map[string]interface{}
	for _, r := range rebates {
		if e := r.(map[string]interface{}); e["offer_brand"] == "Jif" {
			first = e
		}
	}
	if first == nil {
		t.Fatalf("expected a rebate with offer_brand Jif, got %v", rebates)
	}
	if first["amount_cents"] != float64(150) {
		t.Errorf("expected amount_cents 150, got %v", first["amount_cents"])
	}
}

func TestRebates_RequiresAuth(t *testing.T) {
	server := newTestServer(t)

	rr := doRequest(server.handleRebates, "GET", "/api/rebates", nil, nil)
	if rr.Code != 401 {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRebates_EmptyForNewUser(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleRebates, "GET", "/api/rebates", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	result := parseJSON(rr)
	rebates := result["rebates"].([]interface{})
	if len(rebates) != 0 {
		t.Errorf("expected 0 rebates, got %d", len(rebates))
	}
	if result["total_cents"] != float64(0) {
		t.Errorf("expected total_cents 0, got %v", result["total_cents"])
	}
}

func TestSwaps_AcceptIncludesCumulativeTotal(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone, swap_credits_cents) VALUES ('user-1', '5551234567', 500)")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// 500 existing + 125 from skippy deal = 625
	if resp.SwapCreditsCents != 625 {
		t.Errorf("expected swap_credits_cents 625, got %d", resp.SwapCreditsCents)
	}
	if !strings.Contains(resp.Message, "$6.25 total saved") {
		t.Errorf("expected message to include cumulative total, got: %s", resp.Message)
	}
}

func TestSwaps_PendingSwapIncludesPriceData(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added peanut butter!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	// Verify new price fields from seeded data (skippy→jif: 348 vs 473)
	if resp.PendingSwap.OfferPriceCents != 348 {
		t.Errorf("expected offer_price_cents 348, got %d", resp.PendingSwap.OfferPriceCents)
	}
	if resp.PendingSwap.OriginalPriceCents != 473 {
		t.Errorf("expected original_price_cents 473, got %d", resp.PendingSwap.OriginalPriceCents)
	}
	if resp.PendingSwap.OfferSize != "16 oz" {
		t.Errorf("expected offer_size '16 oz', got %q", resp.PendingSwap.OfferSize)
	}
	if resp.PendingSwap.OfferUnitPrice != "$0.22/oz" {
		t.Errorf("expected offer_unit_price '$0.22/oz', got %q", resp.PendingSwap.OfferUnitPrice)
	}
	// savings_percent uses per-oz comparison when sizes differ (16 oz vs 16.3 oz)
	// $0.22/oz vs $0.29/oz → (29-22)/29 = 24%
	if resp.PendingSwap.SavingsPercent != 24 {
		t.Errorf("expected savings_percent 24 (per-oz), got %d", resp.PendingSwap.SavingsPercent)
	}
}

func TestSwaps_WhyBulletsParsed(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added peanut butter!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	if len(resp.PendingSwap.WhyBullets) != 3 {
		t.Fatalf("expected 3 why bullets, got %d", len(resp.PendingSwap.WhyBullets))
	}
	if resp.PendingSwap.WhyBullets[0] != "No palm oil" {
		t.Errorf("expected first bullet 'No palm oil', got %q", resp.PendingSwap.WhyBullets[0])
	}
}

func TestSwaps_AllergenWarningIncluded(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Use the archer→chomps deal which has allergen_notes
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Beef Sticks", Brand: "Archer", Department: "Snacks & Candy", Quantity: 1}},
			Message: "Added beef sticks!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "archer beef sticks"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap for archer")
	}

	if resp.PendingSwap.AllergenWarning != "Contains: beef. Free of top 8 allergens." {
		t.Errorf("expected allergen warning, got %q", resp.PendingSwap.AllergenWarning)
	}
}

func TestSwaps_PendingSwapGracefulDegradation(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Insert a deal with no price/why data (only required fields)
	server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES ('heinz', 'HuntsTest', 'Tomato Ketchup', 50, 50000, 'active')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Ketchup", Brand: "Heinz", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added ketchup!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "heinz ketchup"}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap for heinz")
	}

	// All price fields should be zero/empty for graceful degradation
	if resp.PendingSwap.OfferPriceCents != 0 {
		t.Errorf("expected offer_price_cents 0, got %d", resp.PendingSwap.OfferPriceCents)
	}
	if resp.PendingSwap.SavingsPercent != 0 {
		t.Errorf("expected savings_percent 0, got %d", resp.PendingSwap.SavingsPercent)
	}
	if len(resp.PendingSwap.WhyBullets) != 0 {
		t.Errorf("expected empty why_bullets, got %v", resp.PendingSwap.WhyBullets)
	}
	if resp.PendingSwap.AllergenWarning != "" {
		t.Errorf("expected empty allergen_warning, got %q", resp.PendingSwap.AllergenWarning)
	}
	// Core fields still work
	if resp.PendingSwap.OfferBrand != "HuntsTest" {
		t.Errorf("expected offer brand HuntsTest, got %s", resp.PendingSwap.OfferBrand)
	}
	if resp.PendingSwap.DiscountCents != 50 {
		t.Errorf("expected 50 discount cents, got %d", resp.PendingSwap.DiscountCents)
	}
}

func TestSwaps_AutotuneThresholdBlocksDeals(t *testing.T) {
	server := newTestServer(t)

	// Create user with autotune ON, 20+ shown, <5% accept rate → should block deals
	server.db.Exec(`INSERT INTO users (id, phone, penny_autotune, deal_shown_count, deal_accept_count)
		VALUES ('user-1', '5551234567', 1, 25, 1)`)
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap != nil {
		t.Error("expected no swap when autotune ratio < 5% (1/25 = 4%)")
	}
}

func TestSwaps_SliderMidRangeFiltersSmallDiscounts(t *testing.T) {
	server := newTestServer(t)

	// Slider 50, autotune OFF — should block deals with discount <= $1
	server.db.Exec(`INSERT INTO users (id, phone, penny_slider, penny_autotune)
		VALUES ('user-1', '5551234567', 50, 0)`)
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Archer→Chomps has discount_cents=75 (< $1), should be filtered by mid-range slider
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Beef Sticks", Brand: "Archer", Department: "Snacks & Candy", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "archer beef sticks"}, cookies)

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap != nil {
		t.Error("expected no swap for $0.75 discount with slider=50 (threshold is $1)")
	}

	// Skippy→Jif has discount_cents=125 (> $1), should still show
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr2 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	var resp2 ParseResponse
	json.NewDecoder(rr2.Body).Decode(&resp2)
	if resp2.PendingSwap == nil {
		t.Error("expected swap for $1.50 discount with slider=50 (above threshold)")
	}
}

func TestSwaps_InvalidDealIDReturnsError(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Ketchup', 'Heinz', 'Pantry & Dry Goods')")

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": 99999, "swap_item_id": 1}, cookies)

	if rr.Code != 400 {
		t.Fatalf("expected 400 for nonexistent deal, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp["error"] != "Deal no longer available" {
		t.Errorf("expected 'Deal no longer available' error, got %q", errResp["error"])
	}
}

func TestSwaps_DoubleAcceptBlocked(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	// First accept — should succeed
	rr1 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr1.Code != 201 {
		t.Fatalf("first accept: expected 201, got %d: %s", rr1.Code, rr1.Body.String())
	}

	// Second accept — same deal+item within 1 hour → blocked by rebate dedup (B14)
	rr2 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr2.Code != 409 {
		t.Fatalf("second accept: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify only 1 credit award (125, not 250)
	var credits int
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	if credits != 125 {
		t.Errorf("expected 125 credits (single accept), got %d", credits)
	}

	// Verify only 1 rebate row
	var rebateCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1'").Scan(&rebateCount)
	if rebateCount != 1 {
		t.Errorf("expected 1 rebate row, got %d", rebateCount)
	}
}

// TestSwaps_ConcurrentAcceptBlocked fires many simultaneous accepts of the same
// deal. The prior-accepts COUNT ran before tx.Begin, so two concurrent accepts
// both passed it and each one incremented the brand's budget, inserted a
// swap_events row, inserted a swap_rebates row and credited swap_credits_cents:
// the brand pays twice and the user is paid twice for one swap. The
// one-accept-per-user-per-campaign guarantee is what makes swaps non-farmable.
func TestSwaps_ConcurrentAcceptBlocked(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	var dealID, budgetBefore int
	server.db.QueryRow("SELECT id, budget_spent_cents FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID, &budgetBefore)

	const n = 8
	codes := make([]int, n)
	hammer(t, n, func(i int) {
		rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
			map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
		codes[i] = rr.Code
	})

	wins := 0
	for i, c := range codes {
		switch c {
		case http.StatusCreated, http.StatusOK:
			wins++
		case http.StatusConflict:
		default:
			t.Errorf("accept %d: unexpected status %d (want 201 for the winner, 409 for losers)", i, c)
		}
	}
	if wins != 1 {
		t.Errorf("accepts that succeeded: got %d, want exactly 1", wins)
	}

	var acceptedEvents int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE user_id = 'user-1' AND deal_id = ? AND action = 'accepted'", dealID).Scan(&acceptedEvents)
	if acceptedEvents != 1 {
		t.Errorf("accepted swap_events rows: got %d, want 1", acceptedEvents)
	}

	var rebateCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1' AND deal_id = ?", dealID).Scan(&rebateCount)
	if rebateCount != 1 {
		t.Errorf("swap_rebates rows: got %d, want 1 (user paid more than once)", rebateCount)
	}

	var credits int
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	if credits != 125 {
		t.Errorf("swap_credits_cents: got %d, want 125 (single accept)", credits)
	}

	var budgetAfter int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&budgetAfter)
	if budgetAfter-budgetBefore != 125 {
		t.Errorf("brand budget debited by %d, want 125 (brand charged more than once)", budgetAfter-budgetBefore)
	}
}

// TestSwaps_AcceptWithPreSeededEventBlocked is the deterministic partner to the
// hammer test: the accepted swap_events row already exists, so the accept must
// 409 and leave the brand's budget and the user's credits untouched.
func TestSwaps_AcceptWithPreSeededEventBlocked(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	var dealID, budgetBefore int
	server.db.QueryRow("SELECT id, budget_spent_cents FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID, &budgetBefore)

	// A prior accept already landed.
	if _, err := server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES ('user-1', ?, 1, 'accepted')", dealID); err != nil {
		t.Fatalf("seeding accepted event: %v", err)
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr.Code != http.StatusConflict {
		t.Fatalf("second accept: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	var budgetAfter int
	server.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&budgetAfter)
	if budgetAfter != budgetBefore {
		t.Errorf("brand budget moved on a blocked accept: %d -> %d", budgetBefore, budgetAfter)
	}
	var credits int
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	if credits != 0 {
		t.Errorf("swap_credits_cents on a blocked accept: got %d, want 0", credits)
	}
	var rebateCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1'").Scan(&rebateCount)
	if rebateCount != 0 {
		t.Errorf("swap_rebates rows on a blocked accept: got %d, want 0", rebateCount)
	}
}

// TestSwaps_OneAcceptUniqueIndex pins the partial unique index that enforces
// one accepted swap_events row per (user, deal) at the storage layer. Declines
// are deliberately unconstrained — a user may decline the same deal repeatedly.
func TestSwaps_OneAcceptUniqueIndex(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	ins := func(action string) error {
		_, err := server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES ('user-1', ?, 1, ?)", dealID, action)
		return err
	}

	if err := ins("accepted"); err != nil {
		t.Fatalf("first accepted insert: %v", err)
	}
	if err := ins("accepted"); err == nil {
		t.Error("second accepted swap_events row was accepted — unique index missing")
	}

	if err := ins("declined"); err != nil {
		t.Fatalf("first declined insert: %v", err)
	}
	if err := ins("declined"); err != nil {
		t.Errorf("repeat declines must stay allowed, got: %v", err)
	}
	if err := ins("offered"); err != nil {
		t.Errorf("repeat offers must stay allowed, got: %v", err)
	}
}

func TestSwaps_CrossUserAcceptBlocked(t *testing.T) {
	server := newTestServer(t)

	// User 1 owns the item
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-2', '5559876543')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")

	// User 2 tries to accept swap on User 1's item (not in same household)
	token2, _ := server.sessions.CreateSession("user-2", "5559876543", "phone")
	cookies2 := []*http.Cookie{{Name: "session", Value: token2}}

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies2)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	// Verify User 1's item was NOT modified (brand should still be Skippy)
	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE id = 1").Scan(&brand)
	if brand != "Skippy" {
		t.Errorf("expected item brand to remain 'Skippy' (cross-user protection), got %q", brand)
	}
}

func TestSwaps_BudgetExhaustionBlocksAccept(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Set a deal budget that's barely enough for one accept (discount=125, budget=125)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)
	server.db.Exec("UPDATE brand_deals SET total_budget_cents = 125 WHERE id = ?", dealID)

	// First accept with item 1 — should succeed and exhaust the budget
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr.Code != 201 {
		t.Fatalf("first accept: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Deal should be auto-paused now
	var status string
	server.db.QueryRow("SELECT status FROM brand_deals WHERE id = ?", dealID).Scan(&status)
	if status != "paused" {
		t.Errorf("expected deal to be auto-paused, got status=%q", status)
	}

	// Second accept from a DIFFERENT user — should fail (deal paused/budget exhausted)
	// (Same user would be blocked by per-campaign limit; use user 2 to test budget enforcement)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-2', '5559998888')")
	token2, _ := server.sessions.CreateSession("user-2", "5559998888", "phone")
	cookies2 := []*http.Cookie{{Name: "session", Value: token2}}
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (2, 'user-2', 'Peanut Butter 2', 'Skippy', 'Pantry & Dry Goods')")
	rr2 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 2}, cookies2)
	if rr2.Code != 400 {
		t.Errorf("second accept after budget exhaustion: expected 400, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify only 125 credits awarded to user 1 (not 250)
	var credits int
	server.db.QueryRow("SELECT swap_credits_cents FROM users WHERE id = 'user-1'").Scan(&credits)
	if credits != 125 {
		t.Errorf("expected 125 credits, got %d", credits)
	}
}

func TestSwaps_OneSwapPerUserPerCampaign(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE competitor_product = 'skippy'").Scan(&dealID)

	// First accept — should succeed
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')")
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 1}, cookies)
	if rr.Code != 201 {
		t.Fatalf("first accept: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Second accept (different item, same deal) — should be blocked
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (2, 'user-1', 'Peanut Butter 2', 'Skippy', 'Pantry & Dry Goods')")
	rr2 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "accept_swap", "swap_deal_id": dealID, "swap_item_id": 2}, cookies)
	if rr2.Code != 409 {
		t.Errorf("second accept same campaign: expected 409, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify only 1 rebate created
	var rebateCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_rebates WHERE user_id = 'user-1' AND deal_id = ?", dealID).Scan(&rebateCount)
	if rebateCount != 1 {
		t.Errorf("expected 1 rebate, got %d", rebateCount)
	}
}

// =============================================================================
// SWAP TYPE EXPANSION (Sprint 3)
// =============================================================================

func TestSwaps_BOGOAcceptSetsQuantity(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Get the seeded BOGO deal (cheerios→Magic Spoon)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE deal_type = 'bogo'").Scan(&dealID)
	if dealID == 0 {
		t.Fatal("no BOGO deal found in seeded data")
	}

	// Create matching item
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Cheerios', 'General Mills', 'Cereal')")

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

	// Verify item has quantity=2 (BOGO)
	var quantity int
	var name, brand string
	server.db.QueryRow("SELECT name, brand, quantity FROM list_items WHERE id = 1").Scan(&name, &brand, &quantity)
	if quantity != 2 {
		t.Errorf("BOGO: expected quantity 2, got %d", quantity)
	}
	if brand != "Magic Spoon" {
		t.Errorf("BOGO: expected brand Magic Spoon, got %s", brand)
	}
}

func TestSwaps_TrialAcceptAddsItem(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Get the seeded Trial deal (folgers→Counter Culture)
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE deal_type = 'trial'").Scan(&dealID)
	if dealID == 0 {
		t.Fatal("no Trial deal found in seeded data")
	}

	// Create matching item
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Coffee', 'Folgers', 'Beverages')")

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

	// Verify original item is UNCHANGED (trial keeps original)
	var origName, origBrand string
	server.db.QueryRow("SELECT name, brand FROM list_items WHERE id = 1").Scan(&origName, &origBrand)
	if origName != "Coffee" || origBrand != "Folgers" {
		t.Errorf("Trial: original item should be unchanged, got %s %s", origBrand, origName)
	}

	// Verify trial item was ADDED as new item
	var trialCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = 'user-1' AND name = 'Counter Culture Coffee Sample'").Scan(&trialCount)
	if trialCount != 1 {
		t.Errorf("Trial: expected 1 trial item added, got %d", trialCount)
	}
}

func TestSwaps_SeasonalDealIncludesEndsAt(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create matching item for seasonal deal (oreo→Partake)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department) VALUES (1, 'user-1', 'Oreo Cookies', 'Nabisco', 'Snacks')")

	// Add the item via parse to trigger deal check
	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{"text": "oreo"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check the seasonal deal has ends_at and campaign_tag set
	var endsAt, campaignTag, dealType string
	server.db.QueryRow("SELECT COALESCE(ends_at,''), campaign_tag, deal_type FROM brand_deals WHERE deal_type = 'seasonal'").Scan(&endsAt, &campaignTag, &dealType)
	if dealType != "seasonal" {
		t.Errorf("expected deal_type 'seasonal', got %s", dealType)
	}
	if endsAt == "" {
		t.Error("seasonal deal should have ends_at set")
	}
	if campaignTag != "holiday_2026" {
		t.Errorf("expected campaign_tag 'holiday_2026', got %s", campaignTag)
	}
}

func TestSwaps_NewDealTypesInAdminGET(t *testing.T) {
	server := newTestServer(t)

	// Use the admin phone directly
	server.db.Exec("INSERT INTO users (id, phone, clerk_user_id) VALUES ('user-1', '3104861833', 'user_test_admin')")
	token, _ := server.sessions.CreateSession("user-1", "3104861833", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleAdminDeals, "GET", "/api/admin/deals", nil, cookies)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var deals []BrandDeal
	json.NewDecoder(rr.Body).Decode(&deals)

	// Check that we have deals with different types
	typeMap := make(map[string]bool)
	for _, d := range deals {
		typeMap[d.DealType] = true
	}
	for _, expected := range []string{"sku_swap", "bogo", "trial", "seasonal"} {
		if !typeMap[expected] {
			t.Errorf("expected deal_type %q in admin GET response", expected)
		}
	}
}

// =============================================================================
// SETTINGS EDGE CASES
// =============================================================================
// WRAPPED ENDPOINT TESTS
// =============================================================================

func TestWrapped_Unauthenticated(t *testing.T) {
	server := newTestServer(t)

	rr := doRequest(server.handleWrapped, "GET", "/api/wrapped", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

func TestWrapped_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleWrapped, "POST", "/api/wrapped", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status: got %d, want 405", rr.Code)
	}
}

func TestWrapped_NoActivity(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleWrapped, "GET", "/api/wrapped", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)

	// All counts should be zero
	if savings, ok := body["total_savings_cents"].(float64); !ok || savings != 0 {
		t.Errorf("total_savings_cents: got %v, want 0", body["total_savings_cents"])
	}
	if swaps, ok := body["swaps_count"].(float64); !ok || swaps != 0 {
		t.Errorf("swaps_count: got %v, want 0", body["swaps_count"])
	}
	if items, ok := body["items_added"].(float64); !ok || items != 0 {
		t.Errorf("items_added: got %v, want 0", body["items_added"])
	}
	if streak, ok := body["streak_days"].(float64); !ok || streak != 0 {
		t.Errorf("streak_days: got %v, want 0", body["streak_days"])
	}

	// top_swap_brand and top_category should be empty strings
	if brand, ok := body["top_swap_brand"].(string); ok && brand != "" {
		t.Errorf("top_swap_brand: got %q, want empty", brand)
	}

	// month should be present
	if _, ok := body["month"].(string); !ok {
		t.Errorf("month: expected a string, got %v", body["month"])
	}

	// share_text should be present
	if _, ok := body["share_text"].(string); !ok {
		t.Errorf("share_text: expected a string, got %v", body["share_text"])
	}
}

func TestWrapped_WithSwapRebates(t *testing.T) {
	server := newTestServer(t)

	// Create user directly
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-100', '+15551234567')")
	token, _ := server.sessions.CreateSession("user-100", "+15551234567", "phone")
	cookie := &http.Cookie{Name: "session", Value: token}

	// Two brand deals with high IDs to avoid conflict with seeded deals. Two
	// campaigns, because a user gets a single accept per campaign
	// (idx_swap_events_one_accept).
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (900, 'heinz ketchup', 'Hunts', 'Tomato Ketchup', 200, 10000, 'active')`)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (901, 'french''s mustard', 'Hunts', 'Yellow Mustard', 200, 10000, 'active')`)

	// Insert accepted swap events (this month)
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (900, 'user-100', 900, 'accepted')")
	server.db.Exec("INSERT INTO swap_events (id, user_id, deal_id, action) VALUES (901, 'user-100', 901, 'accepted')")

	// Insert swap rebates (this month)
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES ('user-100', 900, 900, 150, 'pending')")
	server.db.Exec("INSERT INTO swap_rebates (user_id, swap_event_id, deal_id, amount_cents, status) VALUES ('user-100', 901, 901, 250, 'pending')")

	rr := doRequest(server.handleWrapped, "GET", "/api/wrapped", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)

	// Total savings: 150 + 250 = 400 cents
	if savings, ok := body["total_savings_cents"].(float64); !ok || savings != 400 {
		t.Errorf("total_savings_cents: got %v, want 400", body["total_savings_cents"])
	}

	// Swaps accepted: 2
	if swaps, ok := body["swaps_count"].(float64); !ok || swaps != 2 {
		t.Errorf("swaps_count: got %v, want 2", body["swaps_count"])
	}

	// Top swap brand should be Hunts
	if brand, ok := body["top_swap_brand"].(string); !ok || brand != "Hunts" {
		t.Errorf("top_swap_brand: got %v, want 'Hunts'", body["top_swap_brand"])
	}

	// share_text should mention savings
	shareText, _ := body["share_text"].(string)
	if !strings.Contains(shareText, "$4.00") {
		t.Errorf("share_text should contain '$4.00', got %q", shareText)
	}
	if !strings.Contains(shareText, "2 smart swaps") {
		t.Errorf("share_text should contain '2 smart swaps', got %q", shareText)
	}
}

func TestWrapped_WithMonthParam(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551234567")

	// Query a specific month
	rr := doRequest(server.handleWrapped, "GET", "/api/wrapped?month=2025-12", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)

	// Month label should reflect the queried month
	if month, ok := body["month"].(string); !ok || month != "December 2025" {
		t.Errorf("month: got %v, want 'December 2025'", body["month"])
	}
}

func TestWrapped_ResponseShape(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleWrapped, "GET", "/api/wrapped", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	body := parseJSON(rr)

	// Verify all expected keys exist
	expectedKeys := []string{
		"month", "total_savings_cents", "swaps_count", "items_added",
		"top_swap_brand", "top_category", "streak_days", "share_text", "user_name",
	}
	for _, key := range expectedKeys {
		if _, exists := body[key]; !exists {
			t.Errorf("Response missing key %q", key)
		}
	}
}

// =============================================================================
// ADMIN BACKUP ENDPOINT TESTS
// =============================================================================

func TestDeclineReasonCaptured(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone, penny_slider) VALUES ('user-100', '+15551234567', 0)")

	// Create a deal
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status)
		VALUES (800, 'ketchup', 'Hunts', 'Ketchup', 50, 10000, 'active')`)

	// Insert a decline event with reason
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, decline_reason) VALUES ('user-100', 800, 1, 'declined', 'brand_loyal')")

	// Verify the reason is stored
	var reason string
	server.db.QueryRow("SELECT decline_reason FROM swap_events WHERE user_id = 'user-100' AND action = 'declined'").Scan(&reason)
	if reason != "brand_loyal" {
		t.Errorf("Expected decline_reason 'brand_loyal', got '%s'", reason)
	}
}

func TestPreferenceReset(t *testing.T) {
	server := newTestServer(t)
	cookie, userID := loginUser(t, server, "5551234567")

	// Set dietary profile
	doRequest(server.handleDietaryPreferences, "PATCH", "/api/preferences",
		map[string]interface{}{
			"allergens": []string{"dairy"},
			"diets":     []string{"vegan"},
		},
		[]*http.Cookie{cookie})

	// Insert learning rows
	server.updateLearning("user", userID, "shopping_frequency", "daily", 0.9)
	server.recordSignal(userID, "item_added", "produce", "apples", nil)

	// Insert context cache
	server.db.Exec("INSERT INTO penny_context_cache (user_id, context_json) VALUES (?, '{\"test\":true}')", userID)

	// Verify data exists before reset
	var profileCount, learningCount, signalCount, cacheCount int
	server.db.QueryRow("SELECT COUNT(*) FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&profileCount)
	server.db.QueryRow("SELECT COUNT(*) FROM penny_learnings WHERE scope = 'user' AND scope_id = ?", userID).Scan(&learningCount)
	server.db.QueryRow("SELECT COUNT(*) FROM user_signals WHERE user_id = ?", userID).Scan(&signalCount)
	server.db.QueryRow("SELECT COUNT(*) FROM penny_context_cache WHERE user_id = ?", userID).Scan(&cacheCount)
	if profileCount == 0 || learningCount == 0 || signalCount == 0 || cacheCount == 0 {
		t.Fatalf("Pre-reset data missing: profiles=%d, learnings=%d, signals=%d, cache=%d",
			profileCount, learningCount, signalCount, cacheCount)
	}

	// DELETE /api/preferences/reset
	rr := doRequest(server.handlePreferenceReset, "DELETE", "/api/preferences/reset", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["ok"] != true {
		t.Errorf("Response: got %v, want ok:true", body)
	}

	// Verify all data is gone
	server.db.QueryRow("SELECT COUNT(*) FROM user_dietary_profiles WHERE user_id = ?", userID).Scan(&profileCount)
	server.db.QueryRow("SELECT COUNT(*) FROM penny_learnings WHERE scope = 'user' AND scope_id = ?", userID).Scan(&learningCount)
	server.db.QueryRow("SELECT COUNT(*) FROM user_signals WHERE user_id = ?", userID).Scan(&signalCount)
	server.db.QueryRow("SELECT COUNT(*) FROM penny_context_cache WHERE user_id = ?", userID).Scan(&cacheCount)
	if profileCount != 0 {
		t.Errorf("user_dietary_profiles not cleared: %d rows remain", profileCount)
	}
	if learningCount != 0 {
		t.Errorf("penny_learnings not cleared: %d rows remain", learningCount)
	}
	if signalCount != 0 {
		t.Errorf("user_signals not cleared: %d rows remain", signalCount)
	}
	if cacheCount != 0 {
		t.Errorf("penny_context_cache not cleared: %d rows remain", cacheCount)
	}
}

func TestAllergenHardFilter_BlocksSwap(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone, penny_slider) VALUES ('user-100', '+15551234567', 0)")

	// Set up a dairy allergen
	server.db.Exec(`INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ('user-100', '["dairy"]', '[]')`)

	// Create a deal that has dairy in allergen_notes
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, allergen_notes)
		VALUES (800, 'milk', 'Oatly', 'Oat Milk', 50, 10000, 'active', 'Contains dairy proteins')`)

	// Create a deal that does NOT contain allergens
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, allergen_notes)
		VALUES (801, 'ketchup', 'Hunts', 'Ketchup', 50, 10000, 'active', '')`)

	// Add a milk item — should NOT trigger the dairy deal due to allergen filter
	milkItem := ListItem{ID: 1, Name: "Milk", Brand: "Kirkland", Department: "Dairy & Eggs"}
	swap := server.checkDealsForItems("user-100", []ListItem{milkItem})
	if swap != nil {
		t.Errorf("Expected no swap for dairy-allergic user with milk deal, got deal ID %d", swap.DealID)
	}

	// Add a ketchup item — should trigger the ketchup deal (no allergen match)
	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department, checked) VALUES (2, 'user-100', 'Ketchup', 'Heinz', 'Pantry & Dry Goods', 0)")
	ketchupItem := ListItem{ID: 2, Name: "Ketchup", Brand: "Heinz", Department: "Pantry & Dry Goods"}
	swap = server.checkDealsForItems("user-100", []ListItem{ketchupItem})
	if swap == nil {
		t.Error("Expected swap for ketchup deal (no allergen match)")
	}
}

func TestAllergenHardFilter_NoProfile(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone, penny_slider) VALUES ('user-100', '+15551234567', 0)")

	// No dietary profile — deals should work normally
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents, status, allergen_notes)
		VALUES (800, 'milk', 'Oatly', 'Oat Milk', 50, 10000, 'active', 'Contains dairy')`)

	server.db.Exec("INSERT INTO list_items (id, user_id, name, brand, department, checked) VALUES (1, 'user-100', 'Milk', 'Kirkland', 'Dairy & Eggs', 0)")
	milkItem := ListItem{ID: 1, Name: "Milk", Brand: "Kirkland", Department: "Dairy & Eggs"}
	swap := server.checkDealsForItems("user-100", []ListItem{milkItem})
	if swap == nil {
		t.Error("Expected swap for user without dietary profile")
	}
}

// =============================================================================
// DAILY METRICS AGGREGATION TESTS
// =============================================================================
// OPTIMIZE THIS DEAL TESTS (L3 Brand Learning)
// =============================================================================

func TestOptimizeDeal_Unauthorized(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleOptimizeDeal, "GET", "/brand/optimize/1", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestOptimizeDeal_NotFound(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := brandRegister(t, server, "opt@test.com", "password123", "OptCo")

	rr := doRequest(server.handleOptimizeDeal, "GET", "/brand/optimize/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestOptimizeDeal_InvalidID(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := brandRegister(t, server, "opt2@test.com", "password123", "OptCo2")

	rr := doRequest(server.handleOptimizeDeal, "GET", "/brand/optimize/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestOptimizeDeal_NotOwner(t *testing.T) {
	server := newTestServer(t)
	cookie1, _ := brandRegister(t, server, "owner@test.com", "password123", "OwnerCo")
	cookie2, _ := brandRegister(t, server, "other@test.com", "password123", "OtherCo")

	dealID := createTestDeal(t, server, cookie1)

	rr := doRequest(server.handleOptimizeDeal, "GET",
		fmt.Sprintf("/brand/optimize/%d", dealID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-owner, got %d", rr.Code)
	}
}

func TestOptimizeDeal_LowImpressions(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := brandRegister(t, server, "low@test.com", "password123", "LowCo")
	dealID := createTestDeal(t, server, cookie)

	rr := doRequest(server.handleOptimizeDeal, "GET",
		fmt.Sprintf("/brand/optimize/%d", dealID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	recs := body["recommendations"].([]interface{})
	// Should have "Not enough data yet" recommendation
	found := false
	for _, r := range recs {
		rec := r.(map[string]interface{})
		if rec["title"] == "Not enough data yet" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'Not enough data yet' recommendation for deal with 0 impressions")
	}
}

func TestOptimizeDeal_MissingBullets(t *testing.T) {
	server := newTestServer(t)
	cookie, brandUserID := brandRegister(t, server, "bullets@test.com", "password123", "BulletCo")

	// Create deal with empty why_bullets
	result, _ := server.db.Exec(`INSERT INTO brand_deals (
		competitor_product, offer_brand, offer_product, discount_cents, total_budget_cents,
		status, brand_user_id, offer_price_cents, competitor_price_cents,
		offer_size, competitor_size, offer_unit_price, competitor_unit_price,
		unit_label, why_bullets, allergen_notes, deal_type, quantity_offered)
		VALUES ('Cola', 'TestBrand', 'TestCola', 50, 5000, 'active', ?, 400, 500, '12pk', '12pk', '$0.33/can', '$0.42/can', 'can', '[]', '', 'sku_swap', 1)`,
		brandUserID)
	dealID64, _ := result.LastInsertId()

	// Add 15 swap events (offered + declined) to get past low-impression threshold
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '+15551234567')")
	for i := 0; i < 15; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-1', ?, 'offered')", dealID64)
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action, decline_reason) VALUES ('user-1', ?, 'declined', 'price')", dealID64)
	}

	rr := doRequest(server.handleOptimizeDeal, "GET",
		fmt.Sprintf("/brand/optimize/%d", dealID64), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	recs := body["recommendations"].([]interface{})
	foundBullets := false
	foundPrice := false
	for _, r := range recs {
		rec := r.(map[string]interface{})
		if rec["title"] == "Add 'why switch' reasons" {
			foundBullets = true
		}
		if rec["title"] == "Users cite price as top decline reason" {
			foundPrice = true
		}
	}
	if !foundBullets {
		t.Error("Expected 'Add why switch reasons' recommendation")
	}
	if !foundPrice {
		t.Error("Expected 'Users cite price as top decline reason' recommendation")
	}

	// Verify metrics in response
	metrics := body["metrics"].(map[string]interface{})
	if metrics["offered"].(float64) != 15 {
		t.Errorf("Expected 15 offered, got %v", metrics["offered"])
	}
	if metrics["declined"].(float64) != 15 {
		t.Errorf("Expected 15 declined, got %v", metrics["declined"])
	}
}

// =============================================================================
// REPURCHASE NUDGE TESTS
// =============================================================================
// SWAP INTERACTION ENRICHMENT TESTS
// =============================================================================

func TestSwapEvent_TimeToActionAndWhyTapped(t *testing.T) {
	server := newTestServer(t)

	// Verify the columns exist by inserting directly
	_, err := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, time_to_action_ms, why_tapped)
		VALUES ('user-1', 1, 'offered', 0, 0)`)
	if err != nil {
		// Need parent rows for foreign keys
		server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '+15551234567')")
		server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, status)
			VALUES (1, 'test', 'TestBrand', 'TestProduct', 50, 'active')`)
		_, err = server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, time_to_action_ms, why_tapped)
			VALUES ('user-1', 1, 'offered', 0, 0)`)
		if err != nil {
			t.Fatalf("Failed to insert swap_event with enrichment columns: %v", err)
		}
	}

	// Insert with enrichment values
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-2', '+15559999999')")
	_, err = server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, time_to_action_ms, why_tapped)
		VALUES ('user-2', 1, 'accepted', 4500, 1)`)
	if err != nil {
		t.Fatalf("Failed to insert enriched swap_event: %v", err)
	}

	// Verify values stored correctly
	var ttaMs int
	var whyTapped int
	err = server.db.QueryRow("SELECT time_to_action_ms, why_tapped FROM swap_events WHERE user_id = 'user-2' AND action = 'accepted'").Scan(&ttaMs, &whyTapped)
	if err != nil {
		t.Fatalf("Failed to query enriched swap_event: %v", err)
	}
	if ttaMs != 4500 {
		t.Errorf("Expected time_to_action_ms=4500, got %d", ttaMs)
	}
	if whyTapped != 1 {
		t.Errorf("Expected why_tapped=1, got %d", whyTapped)
	}
}

// Ensure sort import is used — this is a compile-time check
var _ = sort.Ints

// =============================================================================
// CATEGORY TAXONOMY TESTS
// =============================================================================

// --- User Swap Predictions Tests ---

func TestProcessSwapPredictions_Empty(t *testing.T) {
	server := newTestServer(t)
	// Should not panic or error with no swap events
	server.processSwapPredictions()

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM user_swap_predictions").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 predictions with no swap events, got %d", count)
	}
}

func TestProcessSwapPredictions_ComputesRates(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// Three deals in the same category and brand (ids 100-102 to avoid
	// conflicting with seeded deals). The prediction is per user, so the three
	// accepts stay on one user and spread across three campaigns — a user gets a
	// single accept per campaign (idx_swap_events_one_accept).
	for id := 100; id <= 102; id++ {
		server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
			discount_cents, status, deal_type, category_id)
			VALUES (?, 'Brand A Milk', 'Brand B', 'Brand B Milk', 50, 'active', 'sku_swap', 1)`, id)
	}

	// Create 10 offered, 3 accepted, 7 declined
	for i := 0; i < 10; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, 100, 1, 'offered')", uid)
	}
	for i := 0; i < 3; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, time_to_action_ms) VALUES (?, ?, 1, 'accepted', 2000)", uid, 100+i)
	}
	for i := 0; i < 7; i++ {
		server.db.Exec("INSERT INTO swap_events (user_id, deal_id, item_id, action, time_to_action_ms) VALUES (?, 100, 1, 'declined', 5000)", uid)
	}

	server.processSwapPredictions()

	// Check category prediction (category_id=1, brand="")
	var rate, conf float64
	var offers, accepts int
	err := server.db.QueryRow(
		"SELECT acceptance_rate, confidence, total_offers, total_accepts FROM user_swap_predictions WHERE user_id = ? AND category_id = 1 AND brand = ''",
		uid,
	).Scan(&rate, &conf, &offers, &accepts)
	if err != nil {
		t.Fatalf("category prediction not found: %v", err)
	}
	if offers != 10 {
		t.Errorf("expected 10 offers, got %d", offers)
	}
	if accepts != 3 {
		t.Errorf("expected 3 accepts, got %d", accepts)
	}
	expectedRate := 0.3
	if rate < expectedRate-0.01 || rate > expectedRate+0.01 {
		t.Errorf("expected acceptance rate ~0.3, got %f", rate)
	}
	if conf != 1.0 {
		t.Errorf("expected confidence 1.0 with 10 offers, got %f", conf)
	}

	// Check brand prediction (category_id=0, brand="Brand B")
	err = server.db.QueryRow(
		"SELECT acceptance_rate, confidence FROM user_swap_predictions WHERE user_id = ? AND category_id = 0 AND brand = 'Brand B'",
		uid,
	).Scan(&rate, &conf)
	if err != nil {
		t.Fatalf("brand prediction not found: %v", err)
	}
	if rate < 0.29 || rate > 0.31 {
		t.Errorf("expected brand acceptance rate ~0.3, got %f", rate)
	}
}

func TestGetUserSwapPrediction_NoPrediction(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// No predictions exist — should return neutral (0.5, 0)
	rate, conf := server.getUserSwapPrediction(uid, 1, "SomeBrand")
	if rate != 0.5 {
		t.Errorf("expected neutral rate 0.5, got %f", rate)
	}
	if conf != 0 {
		t.Errorf("expected zero confidence, got %f", conf)
	}
}

func TestGetUserSwapPrediction_CategoryOnly(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// Insert only category prediction
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, confidence)
		VALUES (?, 5, '', 0.7, 20, 14, 0.9)`, uid)

	rate, conf := server.getUserSwapPrediction(uid, 5, "UnknownBrand")
	if rate < 0.69 || rate > 0.71 {
		t.Errorf("expected rate ~0.7, got %f", rate)
	}
	if conf < 0.89 || conf > 0.91 {
		t.Errorf("expected confidence ~0.9, got %f", conf)
	}
}

func TestGetUserSwapPrediction_BrandOnly(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// Insert only brand prediction
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, confidence)
		VALUES (?, 0, 'Acme', 0.2, 15, 3, 0.8)`, uid)

	rate, conf := server.getUserSwapPrediction(uid, 99, "Acme")
	if rate < 0.19 || rate > 0.21 {
		t.Errorf("expected rate ~0.2, got %f", rate)
	}
	if conf < 0.79 || conf > 0.81 {
		t.Errorf("expected confidence ~0.8, got %f", conf)
	}
}

func TestGetUserSwapPrediction_BlendsRates(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// Insert both category and brand predictions with equal confidence
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, confidence)
		VALUES (?, 5, '', 0.8, 20, 16, 0.8)`, uid)
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, confidence)
		VALUES (?, 0, 'Acme', 0.4, 20, 8, 0.8)`, uid)

	rate, conf := server.getUserSwapPrediction(uid, 5, "Acme")
	// Blended: (0.8*0.8 + 0.4*0.8) / (0.8+0.8) = 0.96/1.6 = 0.6
	if rate < 0.59 || rate > 0.61 {
		t.Errorf("expected blended rate ~0.6, got %f", rate)
	}
	// Confidence: min((0.8+0.8)/2, 1.0) = 0.8
	if conf < 0.79 || conf > 0.81 {
		t.Errorf("expected blended confidence ~0.8, got %f", conf)
	}
}

func TestSwapPredictions_AdminOnly(t *testing.T) {
	server := newTestServer(t)

	// Non-admin user
	cookie, _ := loginUser(t, server, "5559999999")
	rr := doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}

	// Admin user
	adminCookie, _ := loginUser(t, server, "3104861833")
	rr = doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for admin, got %d", rr.Code)
	}
}

func TestSwapPredictions_PredictionFilter(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5551234567")

	// Enable autotune for the user
	server.db.Exec("UPDATE users SET penny_autotune = 1 WHERE id = ?", uid)

	// Insert a low-acceptance, high-confidence prediction
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, confidence)
		VALUES (?, 0, 'BadBrand', 0.05, 40, 2, 0.9)`, uid)

	// getUserSwapPrediction should return the low rate
	rate, conf := server.getUserSwapPrediction(uid, 0, "BadBrand")
	if rate > 0.1 {
		t.Errorf("expected low rate, got %f", rate)
	}
	if conf < 0.5 {
		t.Errorf("expected high confidence, got %f", conf)
	}
}

func TestUserSwapPredictions_TableExists(t *testing.T) {
	_, pgdb := openTestDB(t)

	var count int
	err := pgdb.QueryRow("SELECT COUNT(*) FROM user_swap_predictions").Scan(&count)
	if err != nil {
		t.Errorf("user_swap_predictions table doesn't exist: %v", err)
	}
}

// --- Category-Tiered Pricing Tests ---

// =============================================================================
// Swap Propagation Tests
// =============================================================================

func TestSwapPropagations_TableExists(t *testing.T) {
	_, pgdb := openTestDB(t)

	var count int
	err := pgdb.QueryRow("SELECT COUNT(*) FROM swap_propagations").Scan(&count)
	if err != nil {
		t.Fatalf("swap_propagations table should exist: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows in fresh table, got %d", count)
	}
}

func TestProcessSwapPropagations_DetectsAdoption(t *testing.T) {
	server := newTestServer(t)

	// Create two users in the same household
	cookieA, uidA := loginUser(t, server, "5550001111")
	_ = cookieA
	_, uidB := loginUser(t, server, "5550002222")

	// Put both users in the same household (users auto-get solo households on login,
	// so we update existing entries rather than inserting new ones)
	server.db.Exec("INSERT OR IGNORE INTO households (id) VALUES (901)")
	server.db.Exec("UPDATE household_members SET household_id = 901 WHERE user_id = ?", uidA)
	server.db.Exec("UPDATE household_members SET household_id = 901 WHERE user_id = ?", uidB)
	server.db.Exec("UPDATE users SET household_id = 901 WHERE id IN (?, ?)", uidA, uidB)

	// Create a brand deal (use high ID to avoid conflicts with seeded deals)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES (901, 'Old Brand Milk', 'NewBrand', 'NewBrand Milk', 50, 10000, 'active')`)

	// User A accepts the swap
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, created_at)
		VALUES (?, 901, 'accepted', datetime('now', '-1 day'))`, uidA)

	// User B manually adds an item with the same brand AFTER the swap
	server.db.Exec(`INSERT INTO list_items (user_id, name, brand, source, created_at)
		VALUES (?, 'NewBrand Milk', 'NewBrand', 'manual', datetime('now'))`, uidB)

	// Run propagation detection
	server.processSwapPropagations()

	// Verify propagation record exists
	var count int
	err := server.db.QueryRow("SELECT COUNT(*) FROM swap_propagations WHERE deal_id = 901 AND source_user_id = ? AND adopter_user_id = ?", uidA, uidB).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 propagation record, got %d", count)
	}

	// Verify idempotency — running again should not create duplicates
	server.processSwapPropagations()
	err = server.db.QueryRow("SELECT COUNT(*) FROM swap_propagations WHERE deal_id = 901").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected still 1 propagation record after re-run, got %d", count)
	}
}

func TestProcessSwapPropagations_IgnoresSwapSource(t *testing.T) {
	server := newTestServer(t)

	// Create two users in the same household
	cookieA, uidA := loginUser(t, server, "5550003333")
	_ = cookieA
	_, uidB := loginUser(t, server, "5550004444")

	// Put both users in the same household (users auto-get solo households on login,
	// so we update existing entries rather than inserting new ones)
	server.db.Exec("INSERT OR IGNORE INTO households (id) VALUES (902)")
	server.db.Exec("UPDATE household_members SET household_id = 902 WHERE user_id = ?", uidA)
	server.db.Exec("UPDATE household_members SET household_id = 902 WHERE user_id = ?", uidB)
	server.db.Exec("UPDATE users SET household_id = 902 WHERE id IN (?, ?)", uidA, uidB)

	// Create a brand deal (use high ID to avoid conflicts with seeded deals)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES (902, 'Old Brand Juice', 'FreshCo', 'FreshCo Juice', 30, 5000, 'active')`)

	// User A accepts the swap
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, action, created_at)
		VALUES (?, 902, 'accepted', datetime('now', '-1 day'))`, uidA)

	// User B adds an item with the same brand, but via 'swap' source (NOT organic)
	server.db.Exec(`INSERT INTO list_items (user_id, name, brand, source, created_at)
		VALUES (?, 'FreshCo Juice', 'FreshCo', 'swap', datetime('now'))`, uidB)

	// Run propagation detection
	server.processSwapPropagations()

	// Verify NO propagation record exists (swap source should be excluded)
	var count int
	err := server.db.QueryRow("SELECT COUNT(*) FROM swap_propagations WHERE deal_id = 902").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 propagation records for swap-source items, got %d", count)
	}
}

func TestSwapPropagations_AdminOnly(t *testing.T) {
	server := newTestServer(t)

	// Non-admin user
	cookie, _ := loginUser(t, server, "5550005555")
	rr := doRequest(server.handleSwapPropagations, "GET", "/api/admin/swap-propagations", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}

	// Admin user (phone matches AdminPhone "3104861833")
	adminCookie, _ := loginUser(t, server, "3104861833")
	rr = doRequest(server.handleSwapPropagations, "GET", "/api/admin/swap-propagations", nil, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for admin, got %d", rr.Code)
	}
}

// --- Store System Tests ---

// ── Dynamic Auction Pricing Tests ─────────────────────────────────────

func TestAuction_HighestBidScoreWins(t *testing.T) {
	server := newTestServer(t)

	// Create user
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Delete seeded deals to have a clean slate
	server.db.Exec("DELETE FROM brand_deals")

	// Create two competing deals for "skippy" — deal A has lower discount but will have higher bid score
	// Deal A: 200 cents discount (with default 0.5 prediction = bid score 100)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (100, 'skippy', 'BrandA', 'BrandA Peanut Butter', 200, 10000, 'active')`)

	// Deal B: 150 cents discount (with default 0.5 prediction = bid score 75)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (101, 'skippy', 'BrandB', 'BrandB Peanut Butter', 150, 10000, 'active')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added peanut butter!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	// BrandA should win (higher discount × same prediction = higher bid score)
	if resp.PendingSwap.OfferBrand != "BrandA" {
		t.Errorf("expected BrandA to win auction, got %s", resp.PendingSwap.OfferBrand)
	}
	if resp.PendingSwap.DiscountCents != 200 {
		t.Errorf("expected 200 discount cents, got %d", resp.PendingSwap.DiscountCents)
	}
}

func TestAuction_OutbidEventsLogged(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("DELETE FROM brand_deals")

	// Create 3 competing deals
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (100, 'skippy', 'Winner', 'Winner PB', 300, 10000, 'active')`)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (101, 'skippy', 'Loser1', 'Loser1 PB', 200, 10000, 'active')`)
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (102, 'skippy', 'Loser2', 'Loser2 PB', 100, 10000, 'active')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify winner gets 'offered'
	var offeredCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'offered'").Scan(&offeredCount)
	if offeredCount != 1 {
		t.Errorf("expected 1 offered event, got %d", offeredCount)
	}

	// Verify losers get 'outbid'
	var outbidCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'outbid'").Scan(&outbidCount)
	if outbidCount != 2 {
		t.Errorf("expected 2 outbid events, got %d", outbidCount)
	}

	// Verify winner's deal_id
	var winnerDealID int
	server.db.QueryRow("SELECT deal_id FROM swap_events WHERE action = 'offered'").Scan(&winnerDealID)
	if winnerDealID != 100 {
		t.Errorf("expected winner deal_id 100, got %d", winnerDealID)
	}
}

func TestAuction_SingleCandidate_NoOutbidEvents(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("DELETE FROM brand_deals")

	// Only one deal for skippy
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (100, 'skippy', 'OnlyBrand', 'OnlyBrand PB', 200, 10000, 'active')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}
	if resp.PendingSwap.OfferBrand != "OnlyBrand" {
		t.Errorf("expected OnlyBrand, got %s", resp.PendingSwap.OfferBrand)
	}

	// No outbid events should be logged
	var outbidCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'outbid'").Scan(&outbidCount)
	if outbidCount != 0 {
		t.Errorf("expected 0 outbid events for single candidate, got %d", outbidCount)
	}
}

func TestAuction_PredictionWeightedBidding(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Enable autotune so predictions are used
	server.db.Exec("UPDATE users SET penny_autotune = 1 WHERE id = 'user-1'")

	server.db.Exec("DELETE FROM brand_deals")

	// Deal A: high discount (300) but low predicted acceptance → bid score = 300 × 0.2 = 60
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status, category_id)
		VALUES (100, 'skippy', 'ExpensiveBrand', 'Expensive PB', 300, 10000, 'active', 0)`)

	// Deal B: lower discount (200) but high predicted acceptance → bid score = 200 × 0.8 = 160
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status, category_id)
		VALUES (101, 'skippy', 'PopularBrand', 'Popular PB', 200, 10000, 'active', 0)`)

	// Seed prediction data: user 1 has high acceptance for PopularBrand, low for ExpensiveBrand
	_, err1 := server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, confidence, total_offers, total_accepts)
		VALUES ('user-1', 0, 'ExpensiveBrand', 0.2, 0.8, 10, 2)`)
	if err1 != nil {
		t.Fatalf("failed to insert prediction for ExpensiveBrand: %v", err1)
	}
	_, err2 := server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, confidence, total_offers, total_accepts)
		VALUES ('user-1', 0, 'PopularBrand', 0.8, 0.8, 10, 8)`)
	if err2 != nil {
		t.Fatalf("failed to insert prediction for PopularBrand: %v", err2)
	}

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	// PopularBrand should win despite lower discount because bid score 160 > 60
	if resp.PendingSwap.OfferBrand != "PopularBrand" {
		t.Errorf("expected PopularBrand to win prediction-weighted auction, got %s", resp.PendingSwap.OfferBrand)
	}
}

// ── L4 Location Learning Tests ────────────────────────────────────────

func TestStorePriceAverages_Basic(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create a store and some price observations
	storeID, err := server.upsertStore("Kroger #123", "Kroger", "123 Main St", "Columbus", "OH", "43215", "", nil, nil)
	if err != nil {
		t.Fatalf("upsertStore: %v", err)
	}

	server.recordStorePrice(storeID, "Jif Peanut Butter", "Jif", 348, 0, "receipt")
	// Insert second price with different timestamp to avoid UNIQUE constraint
	server.db.Exec(`INSERT INTO store_prices (store_id, product_name, brand, price_cents, category_id, source, observed_at)
		VALUES (?, 'Jif Peanut Butter', 'Jif', 399, 0, 'receipt', datetime('now', '+1 hour'))`, storeID)
	server.recordStorePrice(storeID, "Skippy Peanut Butter", "Skippy", 473, 0, "receipt")

	rr := doRequest(server.handleStorePriceAverages, "GET",
		fmt.Sprintf("/api/stores/%d/price-averages", storeID), nil, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 products, got %d", len(results))
	}

	// Verify Jif has 2 observations with averaged price
	for _, r := range results {
		if r["product_name"] == "Jif Peanut Butter" {
			obs := int(r["observations"].(float64))
			if obs != 2 {
				t.Errorf("Jif: expected 2 observations, got %d", obs)
			}
			avgCents := int(r["avg_price_cents"].(float64))
			// (348+399)/2 = 373.5 → 373 or 374
			if avgCents < 370 || avgCents > 380 {
				t.Errorf("Jif avg: expected ~373, got %d", avgCents)
			}
		}
	}
}

func TestStorePriceAverages_FilterByProduct(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	storeID, _ := server.upsertStore("Target #456", "Target", "456 Oak Ave", "Austin", "TX", "78701", "", nil, nil)
	server.recordStorePrice(storeID, "Heinz Ketchup", "Heinz", 349, 0, "receipt")
	server.recordStorePrice(storeID, "French's Mustard", "French's", 199, 0, "receipt")

	rr := doRequest(server.handleStorePriceAverages, "GET",
		fmt.Sprintf("/api/stores/%d/price-averages?product=ketchup", storeID), nil, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("expected 1 result for ketchup filter, got %d", len(results))
	}
}

func TestPriceComparison_AcrossStores(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create 2 stores with different prices for same product
	store1, _ := server.upsertStore("Kroger #1", "Kroger", "100 First St", "Columbus", "OH", "43215", "", nil, nil)
	store2, _ := server.upsertStore("Walmart #2", "Walmart", "200 Second St", "Columbus", "OH", "43215", "", nil, nil)

	server.recordStorePrice(store1, "Milk 1 Gallon", "Great Value", 349, 0, "receipt")
	server.recordStorePrice(store2, "Milk 1 Gallon", "Great Value", 299, 0, "receipt")

	rr := doRequest(server.handlePriceComparison, "GET",
		"/api/price-compare?product=milk&brand=great+value", nil, cookies)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	stores := result["stores"].([]interface{})
	if len(stores) < 2 {
		t.Fatalf("expected at least 2 stores in comparison, got %d", len(stores))
	}

	// First store should be cheapest (sorted by price ASC)
	first := stores[0].(map[string]interface{})
	if int(first["avg_price_cents"].(float64)) != 299 {
		t.Errorf("expected cheapest store at 299 cents, got %v", first["avg_price_cents"])
	}
}

func TestPriceComparison_RequiresProduct(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handlePriceComparison, "GET", "/api/price-compare", nil, cookies)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without product param, got %d", rr.Code)
	}
}

func TestRegionalSwapPerformance_AdminOnly(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	rr := doRequest(server.handleRegionalSwapPerformance, "GET", "/api/admin/regional-swaps", nil, cookies)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}
}

func TestRegionalSwapPerformance_ReturnsData(t *testing.T) {
	server := newTestServer(t)
	admin, _ := upsertUser(server.db, "3104861833")
	adminToken, _ := server.sessions.CreateSession(admin.ID, "3104861833", "phone")
	adminCookie := &http.Cookie{Name: "session", Value: adminToken}

	// Create store with state
	storeID, _ := server.upsertStore("Kroger Columbus", "Kroger", "100 Main St", "Columbus", "OH", "43215", "", nil, nil)

	// Create user with trip linked to store
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-2', '5559876543')")
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("INSERT INTO shopping_trips (user_id, household_id, trip_date, store_id) VALUES ('user-2', 1, '2026-03-08', ?)", storeID)

	// Create swap events for that user
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-2', 1, 'offered')")
	server.db.Exec("INSERT INTO swap_events (user_id, deal_id, action) VALUES ('user-2', 1, 'accepted')")

	rr := doRequest(server.handleRegionalSwapPerformance, "GET", "/api/admin/regional-swaps",
		nil, []*http.Cookie{adminCookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) == 0 {
		t.Fatal("expected at least 1 regional result")
	}
}

func TestStoreAwarePricing_EnrichesSwapCard(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create a store
	storeID, _ := server.upsertStore("Kroger #42", "Kroger", "42 Elm St", "Columbus", "OH", "43215", "", nil, nil)

	// Link user to store via a shopping trip (requires household_id and trip_date)
	server.db.Exec("INSERT INTO households (id) VALUES (1)")
	server.db.Exec("UPDATE users SET household_id = 1 WHERE id = 'user-1'")
	server.db.Exec("INSERT INTO shopping_trips (user_id, household_id, trip_date, store_id) VALUES ('user-1', 1, '2026-03-08', ?)", storeID)

	// Record store-specific prices — use Jif brand to match the deal's offer
	server.recordStorePrice(storeID, "Creamy Peanut Butter", "Jif", 298, 0, "receipt")

	// Delete seeded deals and create one with known prices
	server.db.Exec("DELETE FROM brand_deals")
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status, offer_price_cents, competitor_price_cents)
		VALUES (100, 'skippy', 'Jif', 'Creamy Peanut Butter', 125, 10000, 'active', 348, 473)`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	// Store-aware price should override the deal's default offer price
	if resp.PendingSwap.OfferPriceCents != 298 {
		t.Errorf("expected store-aware offer price 298, got %d", resp.PendingSwap.OfferPriceCents)
	}

	// Store name should be set
	if resp.PendingSwap.StoreName != "Kroger #42" {
		t.Errorf("expected store_name 'Kroger #42', got %q", resp.PendingSwap.StoreName)
	}
}

func TestGetStorePrice_NotFound(t *testing.T) {
	server := newTestServer(t)

	// No store prices recorded — should return not found
	_, found := server.getStorePrice(1, "NonexistentProduct", "NoBrand")
	if found {
		t.Error("expected not found for nonexistent product")
	}

	// Zero store ID should return not found
	_, found = server.getStorePrice(0, "Milk", "")
	if found {
		t.Error("expected not found for store ID 0")
	}
}

// ── v1 Stretch: Rich Product Descriptions Tests ──────────────────────

func TestProductDescription_InSwapResponse(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("DELETE FROM brand_deals")
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status,
		product_description, key_benefits, brand_story)
		VALUES (100, 'skippy', 'Jif', 'Creamy Peanut Butter', 200, 10000, 'active',
		'Smooth, creamy peanut butter made from roasted peanuts.',
		'["No palm oil","4.6 star avg rating","24% less per ounce"]',
		'Jif has been Americas favorite peanut butter since 1958.')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	if resp.PendingSwap.ProductDescription != "Smooth, creamy peanut butter made from roasted peanuts." {
		t.Errorf("unexpected product_description: %q", resp.PendingSwap.ProductDescription)
	}
	if len(resp.PendingSwap.KeyBenefits) != 3 {
		t.Errorf("expected 3 key_benefits, got %d", len(resp.PendingSwap.KeyBenefits))
	}
	if resp.PendingSwap.BrandStory != "Jif has been Americas favorite peanut butter since 1958." {
		t.Errorf("unexpected brand_story: %q", resp.PendingSwap.BrandStory)
	}
}

func TestProductDescription_EmptyGracefulDegradation(t *testing.T) {
	server := newTestServer(t)

	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	token, _ := server.sessions.CreateSession("user-1", "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("DELETE FROM brand_deals")
	// No product_description, key_benefits, or brand_story
	server.db.Exec(`INSERT INTO brand_deals (id, competitor_product, offer_brand, offer_product,
		discount_cents, total_budget_cents, status)
		VALUES (100, 'skippy', 'Jif', 'Creamy Peanut Butter', 200, 10000, 'active')`)

	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:  "add",
			Items:   []ParsedItem{{Name: "Peanut Butter", Brand: "Skippy", Department: "Pantry & Dry Goods", Quantity: 1}},
			Message: "Added!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "skippy peanut butter"}, cookies)
	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.PendingSwap == nil {
		t.Fatal("expected a pending swap")
	}

	// Empty descriptions should not appear in JSON (omitempty)
	if resp.PendingSwap.ProductDescription != "" {
		t.Errorf("expected empty product_description, got %q", resp.PendingSwap.ProductDescription)
	}
	if resp.PendingSwap.KeyBenefits != nil {
		t.Errorf("expected nil key_benefits, got %v", resp.PendingSwap.KeyBenefits)
	}
}

func TestParseKeyBenefits_EmptyInput(t *testing.T) {
	if result := parseKeyBenefits(""); result != nil {
		t.Errorf("expected nil for empty string, got %v", result)
	}
	if result := parseKeyBenefits("[]"); result != nil {
		t.Errorf("expected nil for empty array, got %v", result)
	}
	if result := parseKeyBenefits("invalid json"); result != nil {
		t.Errorf("expected nil for invalid json, got %v", result)
	}
}

func TestParseKeyBenefits_ValidInput(t *testing.T) {
	result := parseKeyBenefits(`["benefit1","benefit2","benefit3"]`)
	if len(result) != 3 {
		t.Fatalf("expected 3 benefits, got %d", len(result))
	}
	if result[0] != "benefit1" || result[2] != "benefit3" {
		t.Errorf("unexpected benefits: %v", result)
	}
}

func TestConversationDepthColumn_Exists(t *testing.T) {
	server := newTestServer(t)

	// Insert a swap event with conversation_depth
	server.db.Exec("INSERT INTO users (id, phone) VALUES ('user-1', '5551234567')")
	_, err := server.db.Exec(
		"INSERT INTO swap_events (user_id, deal_id, action, conversation_depth) VALUES ('user-1', 1, 'offered', 3)")
	if err != nil {
		t.Fatalf("failed to insert swap_event with conversation_depth: %v", err)
	}

	var depth int
	server.db.QueryRow("SELECT conversation_depth FROM swap_events WHERE user_id = 'user-1'").Scan(&depth)
	if depth != 3 {
		t.Errorf("expected conversation_depth 3, got %d", depth)
	}
}

// =============================================================================
// FEEDBACK TESTS
// =============================================================================

// --- List Swaps Tests ---

func TestListSwaps_Happy(t *testing.T) {
	server := newTestServer(t)
	cookie, userID := loginUser(t, server, "+15550009999")

	// Add a list item
	server.db.Exec("INSERT INTO list_items (user_id, name, quantity, checked) VALUES (?, 'Heinz Ketchup', 1, 0)", userID)
	var itemID int
	server.db.QueryRow("SELECT id FROM list_items WHERE user_id = ? AND name = 'Heinz Ketchup'", userID).Scan(&itemID)

	// Get a deal ID
	var dealID int
	server.db.QueryRow("SELECT id FROM brand_deals WHERE status = 'active' LIMIT 1").Scan(&dealID)

	if dealID > 0 {
		// Create an offered swap event
		server.db.Exec(
			"INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, ?, 'offered')",
			userID, dealID, itemID,
		)

		rr := doRequest(server.handleListSwaps, "GET", "/api/list/swaps", nil, []*http.Cookie{cookie})
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var swaps []map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &swaps)
		if len(swaps) != 1 {
			t.Errorf("expected 1 swap, got %d", len(swaps))
		}
	}
}

func TestListSwaps_NoAuth(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleListSwaps, "GET", "/api/list/swaps", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListSwaps_EmptyWhenNoOffers(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "+15550008888")

	rr := doRequest(server.handleListSwaps, "GET", "/api/list/swaps", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var swaps []interface{}
	json.Unmarshal(rr.Body.Bytes(), &swaps)
	if len(swaps) != 0 {
		t.Errorf("expected 0 swaps, got %d", len(swaps))
	}
}

// --- Penny Quote Tests ---

// =============================================================================
// Receipt: Swap Purchase Verification
// =============================================================================

func TestVerifySwapPurchases_VerifiesMatchingItems(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "+15550030001")

	// Create a brand deal
	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Heinz Ketchup', 'Sir Kensingtons', 'Organic Ketchup', 150, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	// Create an accepted swap event
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)

	// Create a receipt with a matching line item
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'verify_hash1', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'ORG KETCHUP', 499, 1, 'Organic Ketchup', 'high')`, receiptID)

	// Verify swap purchases
	verified := server.verifySwapPurchases(userID, int(receiptID))
	if verified != 1 {
		t.Fatalf("expected 1 verified swap, got %d", verified)
	}

	// Check swap_claims was created
	var claimStatus string
	var payoutHeld string
	err := server.db.QueryRow("SELECT status, payout_held_until FROM swap_claims WHERE receipt_id = ?", receiptID).Scan(&claimStatus, &payoutHeld)
	if err != nil {
		t.Fatalf("swap_claims row not found: %v", err)
	}
	if claimStatus != "verified" {
		t.Errorf("claim status: got %s, want verified", claimStatus)
	}
	if payoutHeld == "" {
		t.Error("payout_held_until should be set (30-day hold)")
	}

	// Check receipts.swaps_verified was updated
	var swapsVerified int
	server.db.QueryRow("SELECT swaps_verified FROM receipts WHERE id = ?", receiptID).Scan(&swapsVerified)
	if swapsVerified != 1 {
		t.Errorf("receipts.swaps_verified: got %d, want 1", swapsVerified)
	}
}

func TestVerifySwapPurchases_NoMatchReturnsZero(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "+15550030002")

	// Create a brand deal
	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Heinz Ketchup', 'Sir Kensingtons', 'Organic Ketchup', 150, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	// Create an accepted swap event
	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)

	// Create receipt with NON-matching line items
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'verify_hash2', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'WHOLE MILK', 399, 1, 'Whole Milk', 'high')`, receiptID)

	verified := server.verifySwapPurchases(userID, int(receiptID))
	if verified != 0 {
		t.Errorf("expected 0 verified swaps for non-matching items, got %d", verified)
	}
}

func TestVerifySwapPurchases_AlreadyVerifiedSkipped(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "+15550030003")

	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Store Ketchup', 'Premium Brand', 'Premium Ketchup', 100, 5000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	swapRes, _ := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)
	swapEventID, _ := swapRes.LastInsertId()

	// Create a prior receipt so FK is satisfied
	priorReceiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'prior_hash', 'processed')`, userID)
	priorReceiptID, _ := priorReceiptRes.LastInsertId()

	// Pre-create a verified claim (simulating already verified)
	_, err := server.db.Exec(`INSERT INTO swap_claims (receipt_id, swap_event_id, deal_id, user_id, status, verified_at, payout_held_until)
		VALUES (?, ?, ?, ?, 'verified', CURRENT_TIMESTAMP, datetime('now', '+30 days'))`, priorReceiptID, swapEventID, dealID, userID)
	if err != nil {
		t.Fatalf("failed to insert prior claim: %v", err)
	}

	// New receipt with matching item — should NOT re-verify
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'verify_hash3', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'PREMIUM KETCHUP', 549, 1, 'Premium Ketchup', 'high')`, receiptID)

	verified := server.verifySwapPurchases(userID, int(receiptID))
	if verified != 0 {
		t.Errorf("expected 0 (already verified swap skipped), got %d", verified)
	}
}

func TestVerifySwapPurchases_MatchesByBrandName(t *testing.T) {
	server := newTestServer(t)
	_, userID := loginUser(t, server, "+15550030004")

	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Old Spice', 'Dr Bronners', 'Castile Soap', 200, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)

	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'verify_hash4', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	// Match by brand name "Dr Bronners" in the matched_name
	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'DR BRONNERS SOAP', 899, 1, 'Dr Bronners Peppermint Soap', 'high')`, receiptID)

	verified := server.verifySwapPurchases(userID, int(receiptID))
	if verified != 1 {
		t.Errorf("expected 1 (brand name match), got %d", verified)
	}
}

// =============================================================================
// Receipt: Swap Claims Endpoint with Data
// =============================================================================


// =============================================================================
// handleEarningsUpload TESTS (coverage boost)
// =============================================================================

func TestEarningsUpload_WrongMethod(t *testing.T) {
	server := newTestServer(t)
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	rr := doRequest(server.handleEarnings, "GET", path, nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rr.Code)
	}
}

func TestEarningsUpload_NoAuthSession(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleEarnings, "POST", "/api/earnings/1/upload", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for no auth, got %d", rr.Code)
	}
}

func TestEarningsUpload_InvalidRebateID(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5559998888")

	// Non-numeric ID
	req := httptest.NewRequest("POST", "/api/earnings/abc/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid rebate ID, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if msg, ok := body["error"].(string); !ok || msg != "Invalid rebate ID" {
		t.Errorf("expected 'Invalid rebate ID' error, got %v", body["error"])
	}
}

func TestEarningsUpload_RebateNotFound(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5559998877")

	req := httptest.NewRequest("POST", "/api/earnings/99999/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent rebate, got %d", rr.Code)
	}
}

func TestEarningsUpload_NotYourRebate(t *testing.T) {
	server := newTestServer(t)
	// Setup rebate owned by one user
	_, _, _, rebateID := setupEarningsTestData(t, server)
	// Login as a different user
	cookie2, _ := loginUser(t, server, "5559998866")

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	req := httptest.NewRequest("POST", path, strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req.AddCookie(cookie2)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong owner, got %d", rr.Code)
	}
}

func TestEarningsUpload_InvalidMultipart(t *testing.T) {
	server := newTestServer(t)
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	req := httptest.NewRequest("POST", path, strings.NewReader("this is not valid multipart data"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid multipart, got %d", rr.Code)
	}
}

func TestEarningsUpload_MissingFileField(t *testing.T) {
	server := newTestServer(t)
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	// Build a valid multipart form but with wrong field name
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// Add a field that is NOT "file"
	fw, err := w.CreateFormField("notfile")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("some data"))
	w.Close()

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file field, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if msg, ok := body["error"].(string); !ok || msg != "No file provided" {
		t.Errorf("expected 'No file provided' error, got %v", body["error"])
	}
}

func TestEarningsUpload_InvalidFileType(t *testing.T) {
	server := newTestServer(t)
	cookie, _, _, rebateID := setupEarningsTestData(t, server)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("plain text content"))
	w.Close()

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid file type, got %d", rr.Code)
	}
	body := parseJSON(rr)
	if msg, ok := body["error"].(string); !ok || !strings.Contains(msg, "Invalid file type") {
		t.Errorf("expected 'Invalid file type' error, got %v", body["error"])
	}
}

func TestEarningsUpload_RateLimit(t *testing.T) {
	server := newTestServer(t)
	cookie, userID, _, rebateID := setupEarningsTestData(t, server)

	// Pre-fill the rate limiter with 5 entries
	val, _ := server.earningsUploadLimiter.LoadOrStore(userID, &rateBucket{})
	rb := val.(*rateBucket)
	rb.mu.Lock()
	now := time.Now()
	for i := 0; i < 5; i++ {
		rb.times = append(rb.times, now.Add(-time.Duration(i)*time.Minute))
	}
	rb.mu.Unlock()

	path := fmt.Sprintf("/api/earnings/%d/upload", rebateID)
	req := httptest.NewRequest("POST", path, strings.NewReader("body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleEarnings(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for rate limit, got %d", rr.Code)
	}
}

// =============================================================================
// handleSwapPredictions TESTS (coverage boost)
// =============================================================================

func TestSwapPredictions_WrongMethod(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "3104861833") // admin phone
	rr := doRequest(server.handleSwapPredictions, "POST", "/api/admin/swap-predictions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

func TestSwapPredictions_NoAuth(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for no auth, got %d", rr.Code)
	}
}

func TestSwapPredictions_NonAdmin(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551112222") // non-admin
	rr := doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}
}

func TestSwapPredictions_EmptyResults(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "3104861833") // admin
	rr := doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// Response should be a JSON array (possibly null for empty, but valid JSON)
	var results []interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &results); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v", err)
	}
}

func TestSwapPredictions_WithData(t *testing.T) {
	server := newTestServer(t)
	cookie, userID := loginUser(t, server, "3104861833") // admin

	// Insert a prediction row with confidence >= 0.3
	server.db.Exec(`INSERT INTO user_swap_predictions (user_id, category_id, brand, acceptance_rate, total_offers, total_accepts, avg_time_to_action_ms, confidence, computed_at)
		VALUES (?, 1, 'TestBrand', 0.75, 10, 7, 5000, 0.8, datetime('now'))`, userID)

	rr := doRequest(server.handleSwapPredictions, "GET", "/api/admin/swap-predictions", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &results); err != nil {
		t.Fatalf("expected valid JSON array, got: %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 prediction result")
	}
	if results[0]["brand"] != "TestBrand" {
		t.Errorf("expected brand 'TestBrand', got %v", results[0]["brand"])
	}
}

// =============================================================================
// handleSwapPropagations TESTS (coverage boost)
// =============================================================================

func TestSwapPropagations_WrongMethod(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "3104861833")
	rr := doRequest(server.handleSwapPropagations, "POST", "/api/admin/swap-propagations", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", rr.Code)
	}
}

func TestSwapPropagations_NoAuth(t *testing.T) {
	server := newTestServer(t)
	rr := doRequest(server.handleSwapPropagations, "GET", "/api/admin/swap-propagations", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for no auth, got %d", rr.Code)
	}
}

func TestSwapPropagations_NonAdmin(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5551113333")
	rr := doRequest(server.handleSwapPropagations, "GET", "/api/admin/swap-propagations", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rr.Code)
	}
}

func TestSwapPropagations_EmptyResults(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "3104861833")
	rr := doRequest(server.handleSwapPropagations, "GET", "/api/admin/swap-propagations", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var results []interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &results); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d items", len(results))
	}
}
