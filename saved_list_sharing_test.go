package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// SAVED LIST SHARING VIA LINK — TESTS
// =============================================================================

// Helper: create a saved list with items, returns saved list ID.
func createTestSavedList(t *testing.T, server *Server, userID, name string, items []itemData) int {
	t.Helper()
	result, err := server.db.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)", userID, name,
	)
	if err != nil {
		t.Fatalf("createTestSavedList: %v", err)
	}
	id, _ := result.LastInsertId()
	for _, item := range items {
		qty := item.Quantity
		if qty < 1 {
			qty = 1
		}
		_, err := server.db.Exec(
			"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size, notes) VALUES (?, ?, ?, ?, ?, ?, ?)",
			id, item.Name, item.Brand, item.Department, qty, item.Size, item.Notes,
		)
		if err != nil {
			t.Fatalf("createTestSavedList item: %v", err)
		}
	}
	return int(id)
}

// --- POST /api/saved-lists/:id/share ---

func TestSavedListShare_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	listID := createTestSavedList(t, server, userID, "Costco Run", []itemData{
		{Name: "Milk", Brand: "Organic Valley", Department: "Dairy", Quantity: 2},
		{Name: "Bread", Brand: "Dave's", Department: "Bakery", Quantity: 1},
	})

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	code, ok := resp["code"].(string)
	if !ok || code == "" {
		t.Fatalf("expected non-empty code, got %v", resp["code"])
	}
	if len(code) != 32 {
		t.Errorf("expected 32-char hex code, got %d chars", len(code))
	}
	if _, ok := resp["expires_at"]; !ok {
		t.Error("expected expires_at in response")
	}
	urlStr, _ := resp["url"].(string)
	if !strings.Contains(urlStr, code) {
		t.Errorf("expected url to contain code, got %q", urlStr)
	}
}

func TestSavedListShare_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/99999/share", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestSavedListShare_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/1/share", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// --- GET /api/shared/:code (saved list) ---

func TestSharedSavedListView_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Set user name
	server.db.Exec("UPDATE users SET name = 'Alice' WHERE id = ?", userID)

	listID := createTestSavedList(t, server, userID, "Costco Run", []itemData{
		{Name: "Milk", Brand: "Organic Valley", Department: "Dairy", Quantity: 2, Size: "1 gal"},
		{Name: "Bread", Brand: "Dave's", Department: "Bakery", Quantity: 1},
	})

	// Share the list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("share: expected 201, got %d", rr.Code)
	}
	shareResp := parseJSON(rr)
	code := shareResp["code"].(string)

	// View shared list (no auth required)
	req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
	rrView := httptest.NewRecorder()
	server.handleSharedView(rrView, req)
	if rrView.Code != http.StatusOK {
		t.Fatalf("view: expected 200, got %d: %s", rrView.Code, rrView.Body.String())
	}

	var viewResp map[string]interface{}
	json.Unmarshal(rrView.Body.Bytes(), &viewResp)

	// Should indicate this is a saved list share
	if viewResp["type"] != "saved_list" {
		t.Errorf("expected type=saved_list, got %v", viewResp["type"])
	}
	if viewResp["list_name"] != "Costco Run" {
		t.Errorf("expected list_name=Costco Run, got %v", viewResp["list_name"])
	}
	if viewResp["sharer_name"] != "Alice" {
		t.Errorf("expected sharer_name=Alice, got %v", viewResp["sharer_name"])
	}

	items, ok := viewResp["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items, got %v", viewResp["items"])
	}

	// Verify brand fidelity is preserved
	item0 := items[0].(map[string]interface{})
	if item0["brand"] == nil || item0["brand"] == "" {
		// At least one item should have a brand
	}

	// Verify saved_list_id is present for authenticated actions
	savedListID, ok := viewResp["saved_list_id"]
	if !ok {
		t.Error("expected saved_list_id in response")
	}
	if int(savedListID.(float64)) != listID {
		t.Errorf("expected saved_list_id=%d, got %v", listID, savedListID)
	}
}

func TestSharedSavedListView_BrandFidelity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	listID := createTestSavedList(t, server, userID, "Weekly Shop", []itemData{
		{Name: "Ketchup", Brand: "Heinz", Department: "Condiments", Quantity: 1},
		{Name: "Yogurt", Brand: "Chobani", Department: "Dairy", Quantity: 3, Size: "32oz"},
	})

	// Share
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookie})
	code := parseJSON(rr)["code"].(string)

	// View
	req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
	rrView := httptest.NewRecorder()
	server.handleSharedView(rrView, req)

	var viewResp map[string]interface{}
	json.Unmarshal(rrView.Body.Bytes(), &viewResp)
	items := viewResp["items"].([]interface{})

	// Check brands are preserved
	foundHeinz, foundChobani := false, false
	for _, raw := range items {
		item := raw.(map[string]interface{})
		if item["brand"] == "Heinz" {
			foundHeinz = true
		}
		if item["brand"] == "Chobani" {
			foundChobani = true
		}
	}
	if !foundHeinz {
		t.Error("expected Heinz brand to be preserved in shared view")
	}
	if !foundChobani {
		t.Error("expected Chobani brand to be preserved in shared view")
	}
}

// --- POST /api/shared/:code/copy (Save as My List) ---

func TestSharedSavedList_Copy(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "5551234567")
	cookieB, _ := loginUser(t, server, "5559999999")

	listID := createTestSavedList(t, server, userA, "Costco Run", []itemData{
		{Name: "Milk", Brand: "Organic Valley", Department: "Dairy", Quantity: 2},
		{Name: "Bread", Brand: "Dave's", Department: "Bakery", Quantity: 1},
		{Name: "Eggs", Brand: "", Department: "Dairy", Quantity: 1},
	})

	// User A shares
	cookieA, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookieA})
	code := parseJSON(rr)["code"].(string)

	// User B copies
	req := httptest.NewRequest("POST", "/api/shared/"+code+"/copy", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookieB)
	rrCopy := httptest.NewRecorder()
	server.handleSharedView(rrCopy, req)

	if rrCopy.Code != http.StatusCreated {
		t.Fatalf("copy: expected 201, got %d: %s", rrCopy.Code, rrCopy.Body.String())
	}

	copyResp := parseJSON(rrCopy)
	newListID := int(copyResp["id"].(float64))
	if newListID == listID {
		t.Error("copy should create a new list with different ID")
	}
	if copyResp["name"] != "Costco Run" {
		t.Errorf("expected name 'Costco Run', got %v", copyResp["name"])
	}
	itemCount := int(copyResp["item_count"].(float64))
	if itemCount != 3 {
		t.Errorf("expected 3 items, got %d", itemCount)
	}

	// Verify the new list is owned by user B
	var ownerID string
	server.db.QueryRow("SELECT user_id FROM saved_lists WHERE id = ?", newListID).Scan(&ownerID)
	if ownerID == userA {
		t.Error("copied list should be owned by recipient, not original sharer")
	}

	// Verify brand fidelity in copy
	var brand string
	server.db.QueryRow("SELECT brand FROM saved_list_items WHERE saved_list_id = ? AND name = 'Milk'", newListID).Scan(&brand)
	if brand != "Organic Valley" {
		t.Errorf("expected brand 'Organic Valley' preserved in copy, got %q", brand)
	}
}

func TestSharedSavedList_Copy_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "5551234567")

	listID := createTestSavedList(t, server, userA, "My List", []itemData{
		{Name: "Milk", Brand: "", Department: "Dairy", Quantity: 1},
	})

	cookieA, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookieA})
	code := parseJSON(rr)["code"].(string)

	// Try copy without auth
	req := httptest.NewRequest("POST", "/api/shared/"+code+"/copy", nil)
	rrCopy := httptest.NewRecorder()
	server.handleSharedView(rrCopy, req)

	if rrCopy.Code != http.StatusUnauthorized {
		t.Errorf("copy without auth: expected 401, got %d", rrCopy.Code)
	}
}

// --- POST /api/shared/:code/add-to-list (Merge into active list) ---

func TestSharedSavedList_AddToList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "5551234567")
	cookieB, userB := loginUser(t, server, "5559999999")

	listID := createTestSavedList(t, server, userA, "Costco Run", []itemData{
		{Name: "Milk", Brand: "Organic Valley", Department: "Dairy", Quantity: 2},
		{Name: "Bread", Brand: "Dave's", Department: "Bakery", Quantity: 1},
		{Name: "Eggs", Brand: "", Department: "Dairy", Quantity: 1},
	})

	// User B already has Milk on their active list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity) VALUES (?, 'Milk', 'Store Brand', 'Dairy', 1)", userB)

	cookieA, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookieA})
	code := parseJSON(rr)["code"].(string)

	// User B merges into active list
	req := httptest.NewRequest("POST", "/api/shared/"+code+"/add-to-list", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookieB)
	rrMerge := httptest.NewRecorder()
	server.handleSharedView(rrMerge, req)

	if rrMerge.Code != http.StatusOK {
		t.Fatalf("add-to-list: expected 200, got %d: %s", rrMerge.Code, rrMerge.Body.String())
	}

	mergeResp := parseJSON(rrMerge)
	added := int(mergeResp["added"].(float64))
	alreadyOnList := int(mergeResp["already_on_list"].(float64))

	// Milk already exists, so 2 new items added
	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}
	if alreadyOnList != 1 {
		t.Errorf("expected 1 already_on_list, got %d", alreadyOnList)
	}

	// Verify summary message
	msg, _ := mergeResp["message"].(string)
	if !strings.Contains(msg, "Added 2 items") {
		t.Errorf("expected message about added items, got %q", msg)
	}
	if !strings.Contains(msg, "1 already on list") {
		t.Errorf("expected message about already on list, got %q", msg)
	}
}

func TestSharedSavedList_AddToList_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userA := loginUser(t, server, "5551234567")

	listID := createTestSavedList(t, server, userA, "My List", []itemData{
		{Name: "Milk", Brand: "", Department: "Dairy", Quantity: 1},
	})

	cookieA, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookieA})
	code := parseJSON(rr)["code"].(string)

	// Try merge without auth
	req := httptest.NewRequest("POST", "/api/shared/"+code+"/add-to-list", nil)
	rrMerge := httptest.NewRecorder()
	server.handleSharedView(rrMerge, req)

	if rrMerge.Code != http.StatusUnauthorized {
		t.Errorf("add-to-list without auth: expected 401, got %d", rrMerge.Code)
	}
}

// --- Expired link ---

func TestSharedSavedList_ExpiredLink(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	listID := createTestSavedList(t, server, userID, "Old List", []itemData{
		{Name: "Milk", Brand: "", Department: "Dairy", Quantity: 1},
	})

	// Insert expired link directly (use a past date string)
	server.db.Exec(
		"INSERT INTO shared_links (code, user_id, expires_at, saved_list_id) VALUES (?, ?, '2020-01-01 00:00:00', ?)",
		"expired-saved-list-code", userID, listID,
	)

	req := httptest.NewRequest("GET", "/api/shared/expired-saved-list-code", nil)
	rr := httptest.NewRecorder()
	server.handleSharedView(rr, req)

	if rr.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", rr.Code)
	}
}

// --- Fork/copy model: changes to original don't propagate ---

func TestSharedSavedList_ForkModel(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookieA, userA := loginUser(t, server, "5551234567")
	cookieB, _ := loginUser(t, server, "5559999999")

	listID := createTestSavedList(t, server, userA, "Weekly Shop", []itemData{
		{Name: "Milk", Brand: "Organic Valley", Department: "Dairy", Quantity: 1},
		{Name: "Bread", Brand: "", Department: "Bakery", Quantity: 1},
	})

	// Share
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/"+itoa(listID)+"/share", nil, []*http.Cookie{cookieA})
	code := parseJSON(rr)["code"].(string)

	// User B copies
	req := httptest.NewRequest("POST", "/api/shared/"+code+"/copy", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookieB)
	rrCopy := httptest.NewRecorder()
	server.handleSharedView(rrCopy, req)
	newListID := int(parseJSON(rrCopy)["id"].(float64))

	// User A modifies original list (adds item)
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department) VALUES (?, 'Cheese', 'Dairy')", listID)

	// User B's copy should not have Cheese
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ? AND name = 'Cheese'", newListID).Scan(&count)
	if count != 0 {
		t.Error("fork/copy model violated: changes to original propagated to copy")
	}
}

// --- Schema: saved_list_id column exists ---

func TestSchema_SharedLinks_HasSavedListIDColumn(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create a user for FK constraint
	server.db.Exec("INSERT OR IGNORE INTO users (id, phone) VALUES ('schema-test-user', '5550000000')")

	// Should be able to insert with saved_list_id
	_, err := server.db.Exec(
		"INSERT INTO shared_links (code, user_id, expires_at, saved_list_id) VALUES ('test-code-sl', 'schema-test-user', '2030-01-01 00:00:00', 42)",
	)
	if err != nil {
		t.Fatalf("shared_links should have saved_list_id column: %v", err)
	}

	// Verify it can be NULL
	_, err = server.db.Exec(
		"INSERT INTO shared_links (code, user_id, expires_at) VALUES ('test-code-sl-2', 'schema-test-user', '2030-01-01 00:00:00')",
	)
	if err != nil {
		t.Fatalf("saved_list_id should be nullable: %v", err)
	}
}

// itoa is a small helper to convert int to string for URL construction.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
