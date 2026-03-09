package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)


// =============================================================================
// RECEIPT TESTS
// =============================================================================

func TestReceipt_ProcessAndCheckOff(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	server.db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Add items to list
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 1, 'Whole Milk', 'Dairy & Eggs')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (2, 1, 'Eggs', 'Dairy & Eggs')")
	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (3, 1, 'Bread', 'Bakery & Deli')")

	// Mock receipt parser to return matched items
	server.parseReceipt = func(apiKey string, images []string, currentItems []ListItem) ([]string, error) {
		return []string{"Whole Milk", "Eggs"}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]interface{}{
			"text":       "",
			"is_receipt": true,
			"images":     []string{"base64fakeimagadata"},
		}, cookies)

	if rr.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Action != "receipt_processed" {
		t.Errorf("expected action receipt_processed, got %s", resp.Action)
	}
	if len(resp.Checked) != 2 {
		t.Errorf("expected 2 checked items, got %d", len(resp.Checked))
	}

	// Verify items are checked in DB
	var checkedCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE checked = 1").Scan(&checkedCount)
	if checkedCount != 2 {
		t.Errorf("expected 2 checked items in DB, got %d", checkedCount)
	}

	// Verify receipt record created
	var receiptCount int
	server.db.QueryRow("SELECT COUNT(*) FROM receipts WHERE user_id = 1").Scan(&receiptCount)
	if receiptCount != 1 {
		t.Errorf("expected 1 receipt record, got %d", receiptCount)
	}
}

func TestReceipt_DuplicateDedup(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	server.db.Exec("INSERT INTO users (id, phone_number) VALUES (1, '5551234567')")
	token, _ := server.sessions.CreateSession(1, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	server.db.Exec("INSERT INTO list_items (id, user_id, name, department) VALUES (1, 1, 'Milk', 'Dairy & Eggs')")

	server.parseReceipt = func(apiKey string, images []string, currentItems []ListItem) ([]string, error) {
		return []string{"Milk"}, nil
	}

	body := map[string]interface{}{
		"text":       "",
		"is_receipt": true,
		"images":     []string{"sameimagedata"},
	}

	// First submission
	rr1 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse", body, cookies)
	if rr1.Code != 201 {
		t.Fatalf("first submit: expected 201, got %d", rr1.Code)
	}

	// Second submission with same image — should be deduped
	rr2 := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse", body, cookies)
	if rr2.Code != 200 {
		t.Fatalf("second submit: expected 200, got %d", rr2.Code)
	}

	var resp ParseResponse
	json.NewDecoder(rr2.Body).Decode(&resp)
	if resp.Action != "receipt_duplicate" {
		t.Errorf("expected action receipt_duplicate, got %s", resp.Action)
	}
}

// =============================================================================
// ADMIN DEALS TESTS
// =============================================================================
// RECEIPT ENDPOINT TESTS
// =============================================================================

func TestGetReceipts_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleReceipts, "GET", "/api/receipts", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/receipts status: got %d, want 200", rr.Code)
	}
	var receipts []interface{}
	json.Unmarshal(rr.Body.Bytes(), &receipts)
	if len(receipts) != 0 {
		t.Errorf("expected 0 receipts, got %d", len(receipts))
	}
}

func TestGetReceipts_ReturnsUserReceipts(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert a receipt directly (receipts table has no raw_text column)
	_, err := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status, store_name, items_found, items_matched)
		VALUES (?, 'testhash123', 'processed', 'Test Store', 3, 2)`, userID)
	if err != nil {
		t.Fatalf("insert receipt: %v", err)
	}

	rr := doRequest(server.handleReceipts, "GET", "/api/receipts", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/receipts status: got %d, want 200", rr.Code)
	}
	var receipts []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &receipts)
	if len(receipts) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(receipts))
	}
	if receipts[0]["status"] != "processed" {
		t.Errorf("status: got %v, want processed", receipts[0]["status"])
	}
	if receipts[0]["store_name"] != "Test Store" {
		t.Errorf("store_name: got %v, want Test Store", receipts[0]["store_name"])
	}
}

func TestGetReceipts_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleReceipts, "GET", "/api/receipts", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

func TestGetReceiptItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert receipt and line items
	res, err := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status)
		VALUES (?, 'hash1', 'processed')`, userID)
	if err != nil {
		t.Fatalf("insert receipt: %v", err)
	}
	receiptID, _ := res.LastInsertId()

	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'MILK 2%', 399, 1, 'Milk', 'high')`, receiptID)
	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'EGGS LG 12CT', 349, 2, 'Eggs', 'medium')`, receiptID)

	rr := doRequest(server.handleReceiptRouting, "GET",
		fmt.Sprintf("/api/receipts/%d/items", receiptID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET receipt items status: got %d, want 200", rr.Code)
	}
	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["raw_text"] != "MILK 2%" {
		t.Errorf("first item raw_text: got %v, want MILK 2%%", items[0]["raw_text"])
	}
	if items[0]["confidence"] != "high" {
		t.Errorf("first item confidence: got %v, want high", items[0]["confidence"])
	}
	// Check price_cents (JSON numbers come back as float64)
	if pc, ok := items[0]["price_cents"].(float64); !ok || int(pc) != 399 {
		t.Errorf("first item price_cents: got %v, want 399", items[0]["price_cents"])
	}
}

func TestGetReceiptItems_OtherUserCantAccess(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551234567")
	cookie2, _ := loginUser(t, server, "5559876543")

	// Insert receipt owned by user1
	res, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status)
		VALUES (?, 'hash1', 'processed')`, userID1)
	receiptID, _ := res.LastInsertId()

	// User2 tries to access user1's receipt items
	rr := doRequest(server.handleReceiptRouting, "GET",
		fmt.Sprintf("/api/receipts/%d/items", receiptID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for other user's receipt, got %d", rr.Code)
	}
}

func TestReceiptConfirm(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert receipt and line item
	res, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status)
		VALUES (?, 'hash2', 'processed')`, userID)
	receiptID, _ := res.LastInsertId()

	liRes, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'ORG MILK', 599, 1, 'Organic Milk', 'low')`, receiptID)
	lineItemID, _ := liRes.LastInsertId()

	// Confirm the line item
	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": lineItemID, "confirmed": true, "matched_name": "Organic Milk"},
		},
	}
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST confirm status: got %d, want 200", rr.Code)
	}
	result := parseJSON(rr)
	if c, ok := result["confirmed"].(float64); !ok || int(c) != 1 {
		t.Errorf("confirmed count: got %v, want 1", result["confirmed"])
	}

	// Verify the line item's confidence was updated
	var conf string
	server.db.QueryRow("SELECT confidence FROM receipt_line_items WHERE id = ?", lineItemID).Scan(&conf)
	if conf != "high" {
		t.Errorf("line item confidence after confirm: got %s, want high", conf)
	}
}

func TestReceiptConfirm_Reject(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	res, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status)
		VALUES (?, 'hash3', 'processed')`, userID)
	receiptID, _ := res.LastInsertId()

	liRes, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'MISC ITEM', 199, 1, 'Something', 'low')`, receiptID)
	lineItemID, _ := liRes.LastInsertId()

	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": lineItemID, "confirmed": false},
		},
	}
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST confirm (reject) status: got %d, want 200", rr.Code)
	}

	// Verify the line item was marked as rejected
	var conf string
	server.db.QueryRow("SELECT confidence FROM receipt_line_items WHERE id = ?", lineItemID).Scan(&conf)
	if conf != "rejected" {
		t.Errorf("line item confidence after reject: got %s, want rejected", conf)
	}
}

func TestGetSwapClaims_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleReceiptRouting, "GET", "/api/receipts/claims", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/receipts/claims status: got %d, want 200", rr.Code)
	}
	var claims []interface{}
	json.Unmarshal(rr.Body.Bytes(), &claims)
	if len(claims) != 0 {
		t.Errorf("expected 0 claims, got %d", len(claims))
	}
}

func TestReceiptRouting_InvalidPath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleReceiptRouting, "GET", "/api/receipts/999/unknown", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown subroute, got %d", rr.Code)
	}
}

// =============================================================================
// RECEIPT CROSS-USER & MULTI-RECEIPT TESTS
// =============================================================================

func TestGetReceipts_CrossUserIsolation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'hash_x', 'processed')`, userID1)

	// User 2 should see 0 receipts
	rr := doRequest(server.handleReceipts, "GET", "/api/receipts", nil, []*http.Cookie{cookie2})
	var receipts []interface{}
	json.Unmarshal(rr.Body.Bytes(), &receipts)
	if len(receipts) != 0 {
		t.Errorf("user 2 should see 0 receipts from user 1, got %d", len(receipts))
	}
	// User 1 should see 1
	rr = doRequest(server.handleReceipts, "GET", "/api/receipts", nil, []*http.Cookie{cookie1})
	json.Unmarshal(rr.Body.Bytes(), &receipts)
	if len(receipts) != 1 {
		t.Errorf("user 1 should see 1 receipt, got %d", len(receipts))
	}
}

func TestGetReceiptItems_CrossUser(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	res, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'hash_cross', 'processed')`, userID1)
	receiptID, _ := res.LastInsertId()

	// User 2 should not see user 1's receipt items
	rr := doRequest(server.handleReceiptRouting, "GET",
		fmt.Sprintf("/api/receipts/%d/items", receiptID), nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("cross-user receipt item access: got %d, want 404", rr.Code)
	}
}

func TestGetReceipts_MultipleReturned(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(`INSERT INTO receipts (user_id, image_hash, items_found, items_matched, store_name, total_cents, status, swaps_verified)
		VALUES (?, 'hash_a', 5, 3, 'Walmart', 4599, 'processed', 1)`, userID)
	server.db.Exec(`INSERT INTO receipts (user_id, image_hash, items_found, items_matched, store_name, total_cents, status, swaps_verified)
		VALUES (?, 'hash_b', 8, 6, 'Kroger', 7250, 'processed', 0)`, userID)

	rr := doRequest(server.handleReceipts, "GET", "/api/receipts", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/receipts status: got %d, want 200", rr.Code)
	}
	var receipts []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &receipts)
	if len(receipts) != 2 {
		t.Fatalf("expected 2 receipts, got %d", len(receipts))
	}
	// Verify both stores present
	stores := map[string]bool{}
	for _, r := range receipts {
		stores[r["store_name"].(string)] = true
	}
	if !stores["Walmart"] || !stores["Kroger"] {
		t.Errorf("expected both Walmart and Kroger in results, got %v", stores)
	}
}

func TestReceiptConfirm_FeedsAbbreviationDB(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	res, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status, store_name)
		VALUES (?, 'hash_abbrev', 'processed', 'Walmart')`, userID)
	receiptID, _ := res.LastInsertId()

	liRes, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'ORG WHL MLK', 599, 1, 'Organic Whole Milk', 'low')`, receiptID)
	lineItemID, _ := liRes.LastInsertId()

	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": lineItemID, "confirmed": true, "matched_name": "Organic Whole Milk"},
		},
	}
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status: got %d, want 200", rr.Code)
	}

	// Verify abbreviation was saved
	var abbrevName string
	var confirmations int
	err := server.db.QueryRow("SELECT full_name, confirmations FROM retailer_abbreviations WHERE abbreviation = 'ORG WHL MLK'").Scan(&abbrevName, &confirmations)
	if err != nil {
		t.Fatalf("abbreviation not saved: %v", err)
	}
	if abbrevName != "Organic Whole Milk" {
		t.Errorf("abbreviation full_name: got %s, want Organic Whole Milk", abbrevName)
	}
	if confirmations != 1 {
		t.Errorf("abbreviation confirmations: got %d, want 1", confirmations)
	}
}

// =============================================================================
// SETTINGS PHONE UPDATE TESTS
// =============================================================================

func TestReceipts_StoreIDColumn(t *testing.T) {
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("SELECT store_id FROM receipts LIMIT 0")
	if err != nil {
		t.Errorf("receipts.store_id column missing: %v", err)
	}

	_, err = db.Exec("SELECT store_id FROM shopping_trips LIMIT 0")
	if err != nil {
		t.Errorf("shopping_trips.store_id column missing: %v", err)
	}
}

// --- Category Alerts Tests ---

// =============================================================================
// Receipt: Swap Claims Endpoint with Data
// =============================================================================

func TestGetSwapClaims_WithData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "+15550030005")

	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Generic Cereal', 'Cheerios', 'Honey Nut Cheerios', 100, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	// Create referenced rows so JOIN succeeds
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'claims_hash', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()
	swapRes, _ := server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)
	swapEventID, _ := swapRes.LastInsertId()

	server.db.Exec(`INSERT INTO swap_claims (receipt_id, swap_event_id, deal_id, user_id, status, verified_at, payout_held_until)
		VALUES (?, ?, ?, ?, 'verified', CURRENT_TIMESTAMP, datetime('now', '+30 days'))`, receiptID, swapEventID, dealID, userID)

	rr := doRequest(server.handleReceiptRouting, "GET", "/api/receipts/claims", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET claims status: got %d, want 200", rr.Code)
	}

	var claims []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &claims)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0]["status"] != "verified" {
		t.Errorf("claim status: got %v, want verified", claims[0]["status"])
	}
	if claims[0]["offer_brand"] != "Cheerios" {
		t.Errorf("claim offer_brand: got %v, want Cheerios", claims[0]["offer_brand"])
	}
	if claims[0]["discount_cents"].(float64) != 100 {
		t.Errorf("claim discount_cents: got %v, want 100", claims[0]["discount_cents"])
	}
}

func TestGetSwapClaims_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleReceiptRouting, "GET", "/api/receipts/claims", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET claims without auth: got %d, want 401", rr.Code)
	}
}

func TestGetSwapClaims_CrossUserIsolation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550030006")
	cookieB, _ := loginUser(t, server, "+15550030007")

	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Brand X', 'Brand Y', 'Product Y', 200, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	// Claim belongs to user A
	server.db.Exec(`INSERT INTO swap_claims (receipt_id, swap_event_id, deal_id, user_id, status, verified_at, payout_held_until)
		VALUES (1, 1, ?, ?, 'verified', CURRENT_TIMESTAMP, datetime('now', '+30 days'))`, dealID, userA)

	// User B should see 0 claims
	rr := doRequest(server.handleReceiptRouting, "GET", "/api/receipts/claims", nil, []*http.Cookie{cookieB})
	if rr.Code != http.StatusOK {
		t.Fatalf("GET claims status: got %d, want 200", rr.Code)
	}
	var claims []interface{}
	json.Unmarshal(rr.Body.Bytes(), &claims)
	if len(claims) != 0 {
		t.Errorf("user B should see 0 claims from user A, got %d", len(claims))
	}
}

// =============================================================================
// Receipt: Confirm cross-user blocked
// =============================================================================

func TestReceiptConfirm_CrossUserBlocked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "+15550030008")
	cookieB, _ := loginUser(t, server, "+15550030009")

	// Receipt belongs to user A
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'cross_hash', 'processed')`, userA)
	receiptID, _ := receiptRes.LastInsertId()

	server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'SOME ITEM', 199, 1, 'Some Item', 'low')`, receiptID)

	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": 1, "confirmed": true, "matched_name": "Some Item"},
		},
	}

	// User B tries to confirm user A's receipt
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookieB})
	if rr.Code != http.StatusNotFound {
		t.Errorf("cross-user confirm should return 404, got %d", rr.Code)
	}
}

// =============================================================================
// Receipt: Confirm updates items_matched count
// =============================================================================

func TestReceiptConfirm_UpdatesMatchedCount(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "+15550030010")

	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status, items_matched) VALUES (?, 'count_hash', 'processed', 0)`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	// Two low-confidence items
	li1Res, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'ITEM1', 199, 1, 'Item One', 'low')`, receiptID)
	li1ID, _ := li1Res.LastInsertId()

	li2Res, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'ITEM2', 299, 1, 'Item Two', 'low')`, receiptID)
	li2ID, _ := li2Res.LastInsertId()

	// Confirm one, reject the other
	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": li1ID, "confirmed": true, "matched_name": "Item One"},
			{"line_item_id": li2ID, "confirmed": false},
		},
	}
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status: got %d, want 200", rr.Code)
	}

	result := parseJSON(rr)
	if result["confirmed"].(float64) != 1 {
		t.Errorf("confirmed: got %v, want 1", result["confirmed"])
	}
	if result["total_matched"].(float64) != 1 {
		t.Errorf("total_matched: got %v, want 1", result["total_matched"])
	}

	// Verify receipt items_matched was updated
	var matched int
	server.db.QueryRow("SELECT items_matched FROM receipts WHERE id = ?", receiptID).Scan(&matched)
	if matched != 1 {
		t.Errorf("receipts.items_matched: got %d, want 1", matched)
	}
}

// =============================================================================
// Receipt: Confirm triggers swap verification
// =============================================================================

func TestReceiptConfirm_TriggersSwapVerification(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "+15550030011")

	// Deal + accepted swap
	dealRes, _ := server.db.Exec(`INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents, daily_budget_cents, status)
		VALUES ('Generic Mayo', 'Hellmanns', 'Light Mayo', 100, 10000, 'active')`)
	dealID, _ := dealRes.LastInsertId()

	server.db.Exec(`INSERT INTO swap_events (user_id, deal_id, item_id, action) VALUES (?, ?, 1, 'accepted')`, userID, dealID)

	// Receipt with a line item matching the swapped product
	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'verify_confirm_hash', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	liRes, _ := server.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_name, confidence)
		VALUES (?, 'HELLMANNS LT MAYO', 549, 1, 'Light Mayo', 'low')`, receiptID)
	liID, _ := liRes.LastInsertId()

	// Confirm the line item — this should trigger swap verification
	body := map[string]interface{}{
		"confirmations": []map[string]interface{}{
			{"line_item_id": liID, "confirmed": true, "matched_name": "Light Mayo"},
		},
	}
	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/confirm", receiptID), body, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm status: got %d, want 200", rr.Code)
	}

	result := parseJSON(rr)
	if sv, ok := result["swaps_verified"].(float64); !ok || int(sv) != 1 {
		t.Errorf("swaps_verified: got %v, want 1", result["swaps_verified"])
	}

	// Verify claim was created in DB
	var claimCount int
	server.db.QueryRow("SELECT COUNT(*) FROM swap_claims WHERE user_id = ? AND deal_id = ?", userID, dealID).Scan(&claimCount)
	if claimCount != 1 {
		t.Errorf("expected 1 swap_claim, got %d", claimCount)
	}
}

// =============================================================================
// Receipt: Rate Limiting
// =============================================================================

func TestReceiptRateLimit_DailyLimit(t *testing.T) {
	// checkReceiptRateLimit uses global sync.Maps, so we test the function directly
	// Reset the limiters for this test
	receiptDayLimiter = sync.Map{}
	receiptWeekLimiter = sync.Map{}

	userID := 99999 // Use a high ID to avoid collisions

	// First 3 should pass
	for i := 0; i < 3; i++ {
		if checkReceiptRateLimit(userID) {
			t.Fatalf("rate limit triggered prematurely on attempt %d", i+1)
		}
	}

	// 4th should be rate-limited
	if !checkReceiptRateLimit(userID) {
		t.Error("expected rate limit after 3 receipts/day, but was not limited")
	}
}

func TestReceiptRateLimit_DifferentUsersIndependent(t *testing.T) {
	receiptDayLimiter = sync.Map{}
	receiptWeekLimiter = sync.Map{}

	// User A uses 3 (max daily)
	for i := 0; i < 3; i++ {
		checkReceiptRateLimit(88881)
	}

	// User B should still be allowed
	if checkReceiptRateLimit(88882) {
		t.Error("user B was rate-limited by user A's usage")
	}
}

// =============================================================================
// Receipt: Method not allowed checks
// =============================================================================

func TestReceiptItems_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "+15550030012")

	receiptRes, _ := server.db.Exec(`INSERT INTO receipts (user_id, image_hash, status) VALUES (?, 'method_hash', 'processed')`, userID)
	receiptID, _ := receiptRes.LastInsertId()

	rr := doRequest(server.handleReceiptRouting, "POST",
		fmt.Sprintf("/api/receipts/%d/items", receiptID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST to /items should return 405, got %d", rr.Code)
	}
}

func TestSwapClaims_MethodNotAllowed(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "+15550030013")

	rr := doRequest(server.handleReceiptRouting, "POST", "/api/receipts/claims", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST to /claims should return 405, got %d", rr.Code)
	}
}

// =============================================================================
// Admin Analytics
// =============================================================================
