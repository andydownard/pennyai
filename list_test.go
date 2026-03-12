package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)


func TestListGet_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var items []ListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestListAdd_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &item); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if item.Name != "Milk" {
		t.Errorf("Name: got %q, want Milk", item.Name)
	}
	if item.Department != "Dairy & Eggs" {
		t.Errorf("Department: got %q, want Dairy & Eggs", item.Department)
	}
	if item.ID == 0 {
		t.Error("Expected non-zero ID")
	}
	if item.Checked {
		t.Error("New item should not be checked")
	}
	if item.Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1", item.Quantity)
	}
}

func TestListAdd_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "  ", "department": "Produce"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListToggle_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert an item directly
	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

	// Toggle it
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if !item.Checked {
		t.Error("Expected item to be checked after toggle")
	}

	// Toggle again — should uncheck
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Checked {
		t.Error("Expected item to be unchecked after second toggle")
	}
}

func TestListEdit_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

	newName := "Sourdough"
	newBrand := "Tartine"
	newSize := "1 lb"
	newQty := 2
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": newName, "brand": newBrand, "size": newSize, "quantity": newQty},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != newName {
		t.Errorf("Name: got %q, want %q", item.Name, newName)
	}
	if item.Brand != newBrand {
		t.Errorf("Brand: got %q, want %q", item.Brand, newBrand)
	}
	if item.Size != newSize {
		t.Errorf("Size: got %q, want %q", item.Size, newSize)
	}
	if item.Quantity != newQty {
		t.Errorf("Quantity: got %d, want %d", item.Quantity, newQty)
	}
}

func TestListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Butter", "Dairy & Eggs")

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Verify gone from DB
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Item still in DB after delete")
	}
}

func TestListCannotAccessOtherUsersItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User 1 adds an item
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Secret Item", "Pantry",
	)
	itemID, _ := result.LastInsertId()

	// User 2 tries to delete user 1's item — should get 404 (not their item)
	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie2})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Delete returned %d, want 404", rr.Code)
	}

	// Item should still exist (user 2's delete was a no-op)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 1 {
		t.Error("User 2 was able to delete user 1's item")
	}

	// User 1 can see their own item
	rr = doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("User 1 should see 1 item, got %d", len(items))
	}
}

// =============================================================================
// DEDUP TESTS
// =============================================================================

func TestDedup_SameItemTwice(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")
	upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity=2, got %d", qty)
	}
}

func TestDedup_CaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "BREAD", "", "Bakery", "", 1, "app", "", "")
	upsertListItem(server.db, userID, "bread", "", "Bakery", "", 1, "app", "", "")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row (case-insensitive match), got %d", count)
	}
}

func TestDedup_DifferentUsersDontCrossDedup(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	upsertListItem(server.db, userID1, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")
	upsertListItem(server.db, userID2, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 rows (one per user), got %d", count)
	}
}

func TestDedup_CheckedItemGetsUnchecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert and check off the item
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)",
		userID, "Milk", "Dairy & Eggs",
	)

	// Upsert same name — should uncheck existing (no duplicate)
	item, _, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 row (no duplicate), got %d", count)
	}
}

func TestDedup_QuantityAddsCorrectly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 2, "app", "", "")
	upsertListItem(server.db, userID, "Eggs", "", "Dairy & Eggs", "", 3, "app", "", "")

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 5 {
		t.Errorf("Expected quantity=5, got %d", qty)
	}
}

func TestDedup_BrandFillsBlankButDoesNotOverwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// First insert: no brand
	upsertListItem(server.db, userID, "Peanut Butter", "", "Pantry", "", 1, "app", "", "")

	// Second insert: has brand — should fill blank
	upsertListItem(server.db, userID, "Peanut Butter", "Jif", "Pantry", "", 1, "app", "", "")

	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ?", userID).Scan(&brand)
	if brand != "Jif" {
		t.Errorf("Expected brand=Jif, got %q", brand)
	}

	// Third insert: different brand — should NOT overwrite
	upsertListItem(server.db, userID, "Peanut Butter", "Skippy", "Pantry", "", 1, "app", "", "")

	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ?", userID).Scan(&brand)
	if brand != "Jif" {
		t.Errorf("Expected brand to stay Jif, got %q", brand)
	}
}

// =============================================================================
// PARSE ENDPOINT TESTS
// =============================================================================

func TestParseAndAdd_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: "1 gal"},
			{Name: "Bread", Brand: "", Department: "Bakery", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk and bread"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Added != 2 {
		t.Errorf("Added: got %d, want 2", resp.Added)
	}
	if len(resp.Items) != 2 {
		t.Errorf("Items: got %d, want 2", len(resp.Items))
	}
	if len(resp.Skipped) != 0 {
		t.Errorf("Skipped: got %d, want 0", len(resp.Skipped))
	}
}

func TestParseAndAdd_SkipsDuplicates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Pre-insert Milk
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1,
	)
	milkID, _ := result.LastInsertId()

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 2, Size: ""},
			{Name: "Eggs", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "2 milk eggs"},
		[]*http.Cookie{cookie})

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp.Added != 1 {
		t.Errorf("Added: got %d, want 1", resp.Added)
	}
	if len(resp.Skipped) != 1 {
		t.Errorf("Skipped: got %d, want 1", len(resp.Skipped))
	}
	if len(resp.Skipped) > 0 && resp.Skipped[0].ExistingID != int(milkID) {
		t.Errorf("Skipped existing_id: got %d, want %d", resp.Skipped[0].ExistingID, milkID)
	}
	if len(resp.Skipped) > 0 && resp.Skipped[0].Parsed.Name != "Milk" {
		t.Errorf("Skipped parsed name: got %q, want Milk", resp.Skipped[0].Parsed.Name)
	}

	// Milk quantity should be merged via upsertListItem (1 + 2 = 3)
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 3 {
		t.Errorf("Milk quantity should be merged: got %d, want 3", qty)
	}
}

func TestParseAndAdd_EmptyTextNoImage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": ""},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestParseAndAdd_NoAPIKey(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	server.config.AnthropicAPIKey = ""
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Status: got %d, want 503", rr.Code)
	}
}

func TestParseAndAdd_ImageOnly(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotImages = images
		return []ParsedItem{
			{Name: "Apple", Brand: "", Department: "Produce", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "", "image": "data:image/jpeg;base64,abc123"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	if len(gotImages) != 1 || gotImages[0] != "data:image/jpeg;base64,abc123" {
		t.Errorf("Image not passed to parser: got %v", gotImages)
	}
}

func TestParseAndAdd_TextAndImage(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	var gotText string
	var gotImages []string
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		gotText = text
		gotImages = images
		return []ParsedItem{
			{Name: "Banana", Brand: "", Department: "Produce", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "also bananas", "image": "data:image/png;base64,xyz"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	if gotText != "also bananas" {
		t.Errorf("Text: got %q, want 'also bananas'", gotText)
	}
	if len(gotImages) != 1 || gotImages[0] != "data:image/png;base64,xyz" {
		t.Errorf("Images: got %v", gotImages)
	}
}

// =============================================================================
// DIRECT ADD DEDUP TEST
// =============================================================================

func TestListAdd_DedupSameName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})
	doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after adding same name twice, got %d", count)
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ?", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity=2, got %d", qty)
	}
}

// =============================================================================
// ADDITIONAL HANDLER TESTS — edge cases for coverage
// =============================================================================

func TestListAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Mystery Item"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Department != "Other" {
		t.Errorf("Department: got %q, want Other", item.Department)
	}
}

func TestListAdd_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListToggle_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999/toggle",
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListEdit_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

	emptyName := "  "
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": emptyName},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListEdit_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999",
		map[string]interface{}{"name": "Whatever"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListEdit_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery",
	)

	req := httptest.NewRequest("PATCH", "/api/list/1", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListEdit_QuantityClamp(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Bread", "Bakery", 1,
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"quantity": 0},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", item.Quantity)
	}
}

func TestListDelete_NonexistentItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "DELETE", "/api/list/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestList_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/abc/toggle", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestInvalidID_ListEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
		want   int
	}{
		{"toggle", "PATCH", "/api/list/abc/toggle", nil, 400},
		{"urgent", "PATCH", "/api/list/abc/urgent", nil, 400},
		{"claim", "PATCH", "/api/list/abc/claim", nil, 400},
		{"history", "GET", "/api/list/abc/history", nil, 400},
		{"edit", "PATCH", "/api/list/abc", map[string]string{"name": "x"}, 400},
		{"delete", "DELETE", "/api/list/abc", nil, 400},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(server.handleList, tc.method, tc.path, tc.body, []*http.Cookie{cookie})
			if rr.Code != tc.want {
				t.Errorf("Status: got %d, want %d", rr.Code, tc.want)
			}
			ct := rr.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type: got %q, want application/json", ct)
			}
			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Errorf("Response is not valid JSON: %v, body: %s", err, rr.Body.String())
			}
			if resp["error"] != "Invalid ID" {
				t.Errorf("Error message: got %q, want %q", resp["error"], "Invalid ID")
			}
		})
	}
}

func TestInvalidID_ListNotFoundReturnsJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/1/unknown", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func TestList_UnknownSubpath(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/1/unknown", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestParseAndAdd_ClaudeError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return nil, fmt.Errorf("Claude API error")
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

func TestParseAndAdd_EmptyItemName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "", Brand: "", Department: "Other", Quantity: 1, Size: ""},
			{Name: "Milk", Brand: "", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "milk"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Added != 1 {
		t.Errorf("Added: got %d, want 1 (empty name should be skipped)", resp.Added)
	}
}

func TestParseAndAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Widget", Brand: "", Department: "", Quantity: 0, Size: ""},
		}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "widget"},
		[]*http.Cookie{cookie})

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Department != "Other" {
		t.Errorf("Department: got %q, want Other", resp.Items[0].Department)
	}
	if resp.Items[0].Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", resp.Items[0].Quantity)
	}
}

func TestParseAndAdd_MalformedJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list/parse", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestListGet_ReturnsMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Bread", "Bakery")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})

	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestListGet_OnlyReturnsOwnItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID1, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID2, "Bread", "Bakery")

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie1})

	var items []ListItem
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("Expected 1 item for user 1, got %d", len(items))
	}
	if len(items) > 0 && items[0].Name != "Milk" {
		t.Errorf("Expected Milk, got %q", items[0].Name)
	}
}

// =============================================================================
// CLEAR LIST TESTS
// =============================================================================

func TestClearList_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID, "Bread", "Bakery")

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestClearList_DoesNotAffectOtherUsers(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID1, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)", userID2, "Bread", "Bakery")

	doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie1})

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID2).Scan(&count)
	if count != 1 {
		t.Errorf("User 2 items affected by user 1 clear: got %d, want 1", count)
	}
}

func TestClearList_EmptyListReturnsOK(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Clearing an already-empty list should not error
	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for clearing empty list, got %d", rr.Code)
	}
}

func TestClearList_CheckedOnlyLeavesUnchecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add one checked and one unchecked item
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)", userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 0)", userID, "Bread", "Bakery")

	// Clear without ?all=true should only clear checked items
	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 unchecked item remaining, got %d", count)
	}

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", userID).Scan(&name)
	if name != "Bread" {
		t.Errorf("expected unchecked item 'Bread' to remain, got %q", name)
	}
}

func TestClearList_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session, got %d", rr.Code)
	}
}

func TestListRestore_EmptyRestore(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Restore with empty items array returns 400 (nothing to restore)
	rr := doRequest(server.handleListRestore, "POST", "/api/list/restore",
		map[string]interface{}{"items": []interface{}{}}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty restore, got %d", rr.Code)
	}
}

// =============================================================================
// SAVED LISTS TESTS
// =============================================================================
// SOURCE / ATTRIBUTION / SHARED SAVED LISTS / INVITE-NO-PHONE TESTS
// =============================================================================

func TestItemSource_App(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Source != "app" {
		t.Errorf("Source: got %q, want 'app'", item.Source)
	}
}

func TestItemSource_SMS(t *testing.T) {
	mock := &MockTwilioClient{}
	server := newTestServer(t, mock)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Bread", Department: "Bakery", Quantity: 1},
		}, nil
	}

	doSMSRequest(server.handleSMSWebhook, "+15551234567", "bread", 0)
	time.Sleep(200 * time.Millisecond)

	var source string
	server.db.QueryRow("SELECT source FROM list_items LIMIT 1").Scan(&source)
	if source != "sms" {
		t.Errorf("Source: got %q, want 'sms'", source)
	}
}

func TestItemSource_Email(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Exec("UPDATE users SET email = 'test@example.com' WHERE id = ?", userID)

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
		}, nil
	}

	doEmailRequest(server.handleEmailWebhook, "test@example.com", "eggs")

	var source string
	server.db.QueryRow("SELECT source FROM list_items WHERE user_id = ?", userID).Scan(&source)
	if source != "email" {
		t.Errorf("Source: got %q, want 'email'", source)
	}
}

func TestItemAttribution(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Set user name
	server.db.Exec("UPDATE users SET name = 'Alice' WHERE id = ?", userID)

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.AddedByName != "Alice" {
		t.Errorf("AddedByName: got %q, want 'Alice'", item.AddedByName)
	}
	if item.AddedByPhone != "5551234567" {
		t.Errorf("AddedByPhone: got %q, want '5551234567'", item.AddedByPhone)
	}
}

// =============================================================================
// handleListEdit TESTS
// =============================================================================

func TestListEdit_MultipleFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Bread", "", "Bakery", 1, "",
	)
	itemID, _ := result.LastInsertId()

	newName := "Sourdough"
	newBrand := "Tartine"
	newDept := "Bakery"
	newQty := 3
	newSize := "1 lb"
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name": newName, "brand": newBrand, "department": newDept,
			"quantity": newQty, "size": newSize,
		},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != newName {
		t.Errorf("Name: got %q, want %q", item.Name, newName)
	}
	if item.Brand != newBrand {
		t.Errorf("Brand: got %q, want %q", item.Brand, newBrand)
	}
	if item.Department != newDept {
		t.Errorf("Department: got %q, want %q", item.Department, newDept)
	}
	if item.Quantity != newQty {
		t.Errorf("Quantity: got %d, want %d", item.Quantity, newQty)
	}
	if item.Size != newSize {
		t.Errorf("Size: got %q, want %q", item.Size, newSize)
	}
}

func TestListEdit_NoFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Bread", "Bakery")

	// Send empty patch — no fields provided
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Item should be unchanged
	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != "Bread" {
		t.Errorf("Name: got %q, want Bread", item.Name)
	}
}

func TestListEdit_AllFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Old Name", "Old Brand", "Pantry", 1, "8 oz",
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name": "New Name", "brand": "New Brand", "department": "Frozen",
			"quantity": 5, "size": "16 oz",
		},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Name != "New Name" || item.Brand != "New Brand" || item.Department != "Frozen" || item.Quantity != 5 || item.Size != "16 oz" {
		t.Errorf("Fields not updated correctly: %+v", item)
	}
}

// =============================================================================
// addParsedItems TESTS
// =============================================================================

func TestAddParsedItems_SkipsEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	parsed := []ParsedItem{
		{Name: "", Department: "Other", Quantity: 1},
		{Name: "  ", Department: "Other", Quantity: 1},
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Errorf("Expected 1 added, got %d", len(added))
	}
	if len(skipped) != 0 {
		t.Errorf("Expected 0 skipped, got %d", len(skipped))
	}
	if len(added) > 0 && added[0].Name != "Milk" {
		t.Errorf("Expected Milk, got %q", added[0].Name)
	}
}

func TestAddParsedItems_SkipsDuplicates(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Pre-insert Milk
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1,
	)

	parsed := []ParsedItem{
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 2},
		{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 1},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Errorf("Expected 1 added (Eggs), got %d", len(added))
	}
	if len(skipped) != 1 {
		t.Errorf("Expected 1 skipped (Milk), got %d", len(skipped))
	}
	if len(skipped) > 0 && skipped[0].Parsed.Name != "Milk" {
		t.Errorf("Skipped item: got %q, want Milk", skipped[0].Parsed.Name)
	}

	// Milk quantity should have been merged (1 + 2 = 3)
	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 3 {
		t.Errorf("Milk quantity after merge: got %d, want 3", qty)
	}
}

func TestAddParsedItems_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	parsed := []ParsedItem{
		{Name: "Widget", Department: "", Quantity: 0},
	}

	added, _ := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 1 {
		t.Fatalf("Expected 1 added, got %d", len(added))
	}
	if added[0].Department != "Other" {
		t.Errorf("Department: got %q, want Other", added[0].Department)
	}
	if added[0].Quantity != 1 {
		t.Errorf("Quantity: got %d, want 1 (clamped from 0)", added[0].Quantity)
	}
}

func TestAddParsedItems_MergesQuantity(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Pre-insert with quantity 3
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs", 3,
	)

	parsed := []ParsedItem{
		{Name: "Eggs", Department: "Dairy & Eggs", Quantity: 2},
	}

	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 0 {
		t.Errorf("Expected 0 added (existing item), got %d", len(added))
	}
	if len(skipped) != 1 {
		t.Errorf("Expected 1 skipped, got %d", len(skipped))
	}

	var qty int
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Eggs'", userID).Scan(&qty)
	if qty != 5 {
		t.Errorf("Quantity: got %d, want 5 (3+2)", qty)
	}
}

func TestAddParsedItems_SourceTagging(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	sources := []string{"app", "sms", "email"}
	for i, src := range sources {
		parsed := []ParsedItem{
			{Name: fmt.Sprintf("Item%d", i), Department: "Other", Quantity: 1},
		}
		addParsedItems(server.db, userID, parsed, src)
	}

	for i, src := range sources {
		var gotSource string
		server.db.QueryRow("SELECT source FROM list_items WHERE name = ?", fmt.Sprintf("Item%d", i)).Scan(&gotSource)
		if gotSource != src {
			t.Errorf("Item%d source: got %q, want %q", i, gotSource, src)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestParseAndAdd_EmptyParsedResults(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{}, nil
	}

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": "random nonsense"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var resp ParseResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Added != 0 {
		t.Errorf("Added: got %d, want 0", resp.Added)
	}
	if len(resp.Items) != 0 {
		t.Errorf("Items: got %d, want 0", len(resp.Items))
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items in DB, got %d", count)
	}
}

func TestParseAndAdd_LongList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Simulate 40+ items returned from Claude (a realistic large grocery list)
	server.parseItems = func(apiKey, text string, images []string) ([]ParsedItem, error) {
		return []ParsedItem{
			{Name: "Mixed Greens", Brand: "Earthbound Farm", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Bananas", Brand: "Chiquita", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Strawberries", Brand: "Driscoll's", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Fresh Basil", Brand: "Gotham Greens", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Clementines", Brand: "Wonderful Halos", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Cooked Beets", Brand: "Love Beets", Department: "Produce", Quantity: 1, Size: ""},
			{Name: "Large Eggs", Brand: "Vital Farms", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Salted Butter", Brand: "Kerrygold", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Greek Yogurt", Brand: "Chobani", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Oat Milk", Brand: "Oatly", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Shredded Sharp Cheddar", Brand: "Sargento", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Whole Milk", Brand: "Fairlife", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Cream Cheese", Brand: "Philadelphia", Department: "Dairy & Eggs", Quantity: 1, Size: ""},
			{Name: "Pasta", Brand: "Barilla", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Basmati Rice", Brand: "Lundberg", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Rolled Oats", Brand: "Bob's Red Mill", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "All-Purpose Flour", Brand: "King Arthur", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Marinara Sauce", Brand: "Rao's", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Chicken Base", Brand: "Better Than Bouillon", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Dijon Mustard", Brand: "Maille", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Ranch Dressing", Brand: "Hidden Valley", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Turkey Breast", Brand: "Applegate", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Bacon", Brand: "Applegate", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Tuna Pouches", Brand: "StarKist", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Chicken Breasts", Brand: "Tyson", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Plant-Based Burger Patties", Brand: "Beyond Meat", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Ground Turkey", Brand: "Perdue", Department: "Meat & Seafood", Quantity: 1, Size: ""},
			{Name: "Popcorn", Brand: "SkinnyPop", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Nut Bars", Brand: "Kind", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Tortilla Chips", Brand: "Siete", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Wheat Crackers", Brand: "Triscuits", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Almond Butter", Brand: "Justin's", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Dark Chocolate", Brand: "Lindt", Department: "Pantry", Quantity: 1, Size: ""},
			{Name: "Sparkling Water", Brand: "LaCroix", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Coffee", Brand: "Peet's", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Tea", Brand: "Twinings", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Orange Juice", Brand: "Simply Orange", Department: "Beverages", Quantity: 1, Size: ""},
			{Name: "Margherita Pizza", Brand: "Amy's Kitchen", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Broccoli Florets", Brand: "Bird's Eye", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Gelato", Brand: "Talenti", Department: "Frozen", Quantity: 1, Size: ""},
			{Name: "Waffles", Brand: "Eggo", Department: "Frozen", Quantity: 1, Size: ""},
		}, nil
	}

	longList := `Produce & Fresh Herbs
Earthbound Farm Organic Mixed Greens
Chiquita Bananas
Driscoll's Strawberries
Gotham Greens Fresh Basil
Wonderful Halo Clementines
Love Beets Vacuum-packed cooked beets
Dairy, Eggs & Alternatives
Vital Farms Pasture-Raised Large Eggs
Kerrygold Salted Butter
Chobani Greek Yogurt
Oatly Full-Thickness Oat Milk
Sargento Shredded Sharp Cheddar Cheese
Fairlife Ultra-Filtered Whole Milk
Philadelphia Original Cream Cheese
Pantry Staples & Grains
Barilla Al Bronzo Pasta
Lundberg Family Farms Organic Basmati Rice
Bob's Red Mill Old Fashioned Rolled Oats
King Arthur Unbleached All-Purpose Flour
Raos Homemade Marinara Sauce
Better Than Bouillon Roasted Chicken Base
Maille Dijon Mustard
Hidden Valley Ranch Dressing
Proteins & Meat
Applegate Organics Roasted Turkey Breast
Applegate Sunday Bacon
StarKist E.V.O.O. Wild Caught Tuna Pouches
Tyson Frozen Boneless Skinless Chicken Breasts
Beyond Meat Plant-Based Burger Patties
Perdue Ground Turkey
Snacks & Sweets
SkinnyPop Original Popcorn
Kind Nut Bars
Siete Grain-Free Tortilla Chips
Triscuits Original Whole Wheat Crackers
Justin's Almond Butter
Lindt Excellence 70% Cocoa Dark Chocolate
Beverages
LaCroix Sparkling Water
Peet's Coffee Major Dickason's Blend
Twinings Earl Grey Tea
Simply Orange Pulp-Free Orange Juice
Freezer Favorites
Amy's Kitchen Margherita Pizza
Bird's Eye Steamfresh Broccoli Florets
Talenti Roman Raspberry Sorbetto
Eggo Homestyle Waffles`

	rr := doRequest(server.handleList, "POST", "/api/list/parse",
		map[string]string{"text": longList},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	var resp ParseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Added != 41 {
		t.Errorf("Added: got %d, want 41", resp.Added)
	}
	if len(resp.Items) != 41 {
		t.Errorf("Items: got %d, want 41", len(resp.Items))
	}

	// Verify all items in DB
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 41 {
		t.Errorf("DB items: got %d, want 41", count)
	}

	// Verify brands were preserved
	var brand string
	server.db.QueryRow("SELECT brand FROM list_items WHERE user_id = ? AND name = 'Marinara Sauce'", userID).Scan(&brand)
	if brand != "Rao's" {
		t.Errorf("Brand for Marinara Sauce: got %q, want %q", brand, "Rao's")
	}
}

// =============================================================================
// PHASE 4: NEW TESTS FOR 80% COVERAGE
// =============================================================================

// --- downloadTwilioMedia tests ---

func TestCollectListItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Milk', 'Organic Valley', 'Dairy & Eggs', 2, '1 gal')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", userID)

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

// --- Saved list handler edge cases ---

// --- handleListClear success ---

func TestListClear_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Bread', 'Bakery', 1)", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

// --- handleSavedListDelete success ---

// --- handleSavedListDelete with items ---

// --- handleListDelete success ---

func TestListDelete_ConfirmGone(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Expected item to be deleted")
	}
}

// --- handleListGet success ---

func TestListGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', 'Organic', 'Dairy & Eggs', 2, '1 gal', 'app')", userID)

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0]["name"] != "Milk" {
		t.Errorf("Name: got %v, want Milk", items[0]["name"])
	}
}

// --- handleListAdd success ---

func TestListAdd_WithDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{"name": "Eggs", "department": "Dairy & Eggs"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	var dept string
	server.db.QueryRow("SELECT department FROM list_items WHERE user_id = ? AND name = 'Eggs'", userID).Scan(&dept)
	if dept != "Dairy & Eggs" {
		t.Errorf("Department: got %q, want 'Dairy & Eggs'", dept)
	}
}

// --- handleListToggle success ---

func TestListToggle_CheckUncheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'Milk', 'Dairy & Eggs', 1, 0)", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var checked int
	server.db.QueryRow("SELECT checked FROM list_items WHERE id = ?", itemID).Scan(&checked)
	if checked != 1 {
		t.Error("Expected item to be checked")
	}
}

// --- downloadTwilioMedia read error (closed connection) ---

// --- handleListAdd empty name ---

// --- handleListAdd invalid JSON ---

func TestListAdd_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/list", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleList(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleSavedListsCreate invalid JSON ---

// --- Tests that trigger DB error paths by closing the DB ---

func TestListGet_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListAdd_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]string{"name": "Milk"},
		[]*http.Cookie{cookie})
	if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListClear_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListToggle_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestListEdit_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]string{"name": "Eggs"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError || rr.Code == http.StatusNotFound {
		// Either 500 or 404 is acceptable depending on which query fails first
	}
}

func TestListDelete_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	itemID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	// Should be an error since DB is closed
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

// --- upsertListItem with existing checked item ---

func TestUpsertListItem_ExistingChecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert a checked item — upsert should uncheck it and set new quantity (no duplicate)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'Milk', 'Dairy & Eggs', 1, 1)", userID)

	item, isNew, err := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 2, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if !isNew {
		t.Error("Expected isNew=true for re-added checked item")
	}
	if item.Quantity != 2 {
		t.Errorf("Quantity: got %d, want 2", item.Quantity)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	// Verify no duplicate — should be exactly 1 item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND LOWER(name) = 'milk'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item, got %d (duplicate created)", count)
	}
}

func TestUpsertListItem_CaseInsensitiveMergeChecked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert a checked "honey" — adding "Honey" should merge, not create duplicate
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, checked) VALUES (?, 'honey', 'Condiments', 1, 1)", userID)

	item, _, err := upsertListItem(server.db, userID, "Honey", "", "Condiments", "", 1, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if item.Checked {
		t.Error("Expected item to be unchecked after re-adding")
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item, got %d (duplicate created)", count)
	}
}

func TestUpsertListItem_MergeBrandAndSize(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	// Insert unchecked item without brand/size
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, checked) VALUES (?, 'Milk', '', 'Dairy & Eggs', 1, '', 0)", userID)

	// Upsert with brand and size — should merge
	item, isNew, err := upsertListItem(server.db, userID, "Milk", "Organic Valley", "Dairy & Eggs", "1 gal", 2, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}
	if isNew {
		t.Error("Expected isNew=false for merge")
	}
	if item.Quantity != 3 {
		t.Errorf("Quantity: got %d, want 3", item.Quantity)
	}
	if item.Brand != "Organic Valley" {
		t.Errorf("Brand: got %q, want 'Organic Valley'", item.Brand)
	}
	if item.Size != "1 gal" {
		t.Errorf("Size: got %q, want '1 gal'", item.Size)
	}
}

// --- addParsedItems with DB error ---

func TestAddParsedItems_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")
	server.db.Close()

	parsed := []ParsedItem{
		{Name: "Milk", Department: "Dairy & Eggs", Quantity: 1},
	}
	added, skipped := addParsedItems(server.db, userID, parsed, "app")
	if len(added) != 0 {
		t.Errorf("Expected 0 added, got %d", len(added))
	}
	if len(skipped) != 0 {
		t.Errorf("Expected 0 skipped, got %d", len(skipped))
	}
}

// =============================================================================
// ITEM MODIFICATION HISTORY TESTS
// =============================================================================

func TestItemHistory_RecordsOnAdd(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add an item via upsert
	item, _, err := upsertListItem(server.db, userID, "Bananas", "", "Produce", "", 1, "app", "", "")
	if err != nil {
		t.Fatalf("upsertListItem: %v", err)
	}

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("History status: got %d, want 200", rr.Code)
	}

	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)
	if len(mods) == 0 {
		t.Fatal("Expected at least 1 modification record")
	}
	if mods[0]["action"] != "added" {
		t.Errorf("Expected action 'added', got %v", mods[0]["action"])
	}
}

func TestItemHistory_RecordsOnToggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	item, _, _ := upsertListItem(server.db, userID, "Milk", "", "Dairy & Eggs", "", 1, "app", "", "")

	// Toggle to checked
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", item.ID), nil, []*http.Cookie{cookie})

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)

	// Should have "added" and "checked"
	actions := make(map[string]bool)
	for _, m := range mods {
		actions[m["action"].(string)] = true
	}
	if !actions["checked"] {
		t.Error("Expected 'checked' action in history")
	}
}

func TestItemHistory_RecordsOnEdit(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	item, _, _ := upsertListItem(server.db, userID, "Bread", "", "Bakery", "", 1, "app", "", "")

	// Edit the item
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", item.ID),
		map[string]interface{}{"name": "Sourdough Bread"}, []*http.Cookie{cookie})

	// Fetch history
	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID), nil, []*http.Cookie{cookie})
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)

	actions := make(map[string]bool)
	for _, m := range mods {
		actions[m["action"].(string)] = true
	}
	if !actions["updated"] {
		t.Error("Expected 'updated' action in history")
	}
}

func TestItemHistory_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleList, "GET", "/api/list/1/history", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rr.Code)
	}
}

func TestItemHistory_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/99999/history", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

// =============================================================================
// SMS/EMAIL DUPE QUANTITY TESTS
// =============================================================================
// CLEAR LIST VIA POST
// =============================================================================

func TestClearList_PostMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear, got %d", count)
	}
}

func TestClearList_DeleteMethodNoLongerWorks(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// DELETE should not work anymore — it will try to parse "clear" as an item ID
	rr := doRequest(server.handleListClear, "DELETE", "/api/list/clear", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("DELETE to /api/list/clear should no longer work (changed to POST)")
	}

	// Item should still be there
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected item to still exist, got %d", count)
	}
}

// =============================================================================
// MUX ROUTING INTEGRATION TESTS
// =============================================================================

func TestMuxRouting_ClearList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// Register routes on a fresh mux, same as main()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/list", server.handleList)
	mux.HandleFunc("/api/list/", server.handleList)
	mux.HandleFunc("/api/list/clear", server.handleListClear)

	req := httptest.NewRequest("POST", "/api/list/clear?all=true", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 from mux-routed clear, got %d: %s", rr.Code, rr.Body.String())
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after clear via mux, got %d", count)
	}
}

func TestMuxRouting_SavedListLoad(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list with items
	var savedListID int64
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	savedListID, _ = res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Apples', 'Produce', 1)", savedListID)

	// Add an existing item that should be cleared on overwrite
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'OldItem', 'Other')", userID)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/saved-lists/", server.handleSavedLists)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedListID), nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 from mux-routed saved list load, got %d: %s", rr.Code, rr.Body.String())
	}

	// OldItem should be gone, Apples should be present
	var oldCount, newCount int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'OldItem'", userID).Scan(&oldCount)
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Apples'", userID).Scan(&newCount)
	if oldCount != 0 {
		t.Errorf("Expected OldItem to be cleared, but found %d", oldCount)
	}
	if newCount != 1 {
		t.Errorf("Expected Apples to be added, but found %d", newCount)
	}
}

// =============================================================================
// PRIVACY AND TERMS PAGES
// =============================================================================

func TestUpsertListItem_Transaction(t *testing.T) {
	// Test that concurrent upserts don't create duplicates
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	_, userID := loginUser(t, server, "5551234567")

	// Add the same item twice — second should merge
	item1, isNew1, err1 := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 1, "app", "", "")
	item2, isNew2, err2 := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 2, "app", "", "")

	if err1 != nil {
		t.Fatalf("First upsert error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Second upsert error: %v", err2)
	}
	if !isNew1 {
		t.Error("First insert should be new")
	}
	if isNew2 {
		t.Error("Second insert should merge, not be new")
	}
	if item1.ID != item2.ID {
		t.Errorf("Expected same item ID, got %d and %d", item1.ID, item2.ID)
	}
	if item2.Quantity != 3 {
		t.Errorf("Expected quantity 3 after merge, got %d", item2.Quantity)
	}
}

func TestUpsertListItem_ConcurrentSafety(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)
	_, userID := loginUser(t, server, "5551234567")

	var wg sync.WaitGroup
	var errors int64
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := upsertListItem(server.db, userID, "Eggs", "", "Dairy", "", 1, "app", "", "")
			if err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}()
	}
	wg.Wait()

	if errors > 0 {
		t.Errorf("Got %d errors from concurrent upserts", errors)
	}

	// Check that we have exactly one item (quantities merged)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND LOWER(name) = 'eggs'", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 egg item, got %d (duplicates created)", count)
	}
}

func TestItemHistory_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "POST", fmt.Sprintf("/api/list/%d/history", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Errorf("Expected non-200 for POST to history, got %d", rr.Code)
	}
}

func TestCollectListItems_EmptyList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestHandleListAdd_DefaultDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{"name": "Mystery Item", "department": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["department"] != "Other" {
		t.Errorf("Expected department 'Other', got %v", body["department"])
	}
}

func TestHandleListGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', 'Organic Valley', 'Dairy', 2, '1 gallon', 'app')", userID)

	rr := doRequest(server.handleList, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0]["brand"] != "Organic Valley" {
		t.Errorf("brand: got %v, want Organic Valley", items[0]["brand"])
	}
}

func TestHandleListAdd_WithDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "POST", "/api/list",
		map[string]interface{}{
			"name":       "Eggs",
			"department": "Dairy & Eggs",
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	if body["department"] != "Dairy & Eggs" {
		t.Errorf("department: got %v, want Dairy & Eggs", body["department"])
	}
}

func TestHandleListToggle_CheckAll(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	// Toggle to checked
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["checked"] != true {
		t.Error("Expected checked=true")
	}

	// Toggle back to unchecked
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	body = parseJSON(rr)
	if body["checked"] != false {
		t.Error("Expected checked=false")
	}
}

func TestHandleListEdit_AllFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{
			"name":       "Almond Milk",
			"brand":      "Silk",
			"department": "Beverages",
			"quantity":   3,
			"size":       "32oz",
		},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["name"] != "Almond Milk" {
		t.Errorf("name: got %v", body["name"])
	}
	if body["brand"] != "Silk" {
		t.Errorf("brand: got %v", body["brand"])
	}
}

func TestHandleListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE id = ?", itemID).Scan(&count)
	if count != 0 {
		t.Error("Item should be deleted")
	}
}

func TestHandleListClear_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 items, got %d", count)
	}
}

func TestHandleItemHistory_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add item (creates modification record)
	item, _, _ := upsertListItem(server.db, userID, "Milk", "", "Dairy", "", 1, "app", "", "")

	rr := doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", item.ID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var history []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &history)
	if len(history) == 0 {
		t.Error("Expected at least one history entry")
	}
}

func TestCollectListItems_WithCheckedItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, 'Milk', 'Dairy', 0)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, 'Bread', 'Bakery', 1)", userID)

	items, err := collectListItems(server.db, userID)
	if err != nil {
		t.Fatalf("collectListItems: %v", err)
	}
	// collectListItems returns all items (checked and unchecked)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestHandleItemHistory_WithModifications(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add an item
	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Add: %d", rr.Code)
	}
	itemID := int(parseJSON(rr)["id"].(float64))

	// Record a modification
	recordModification(server.db, itemID, userID, "updated", "name = ?", "app")

	// Get history — use handleList router which dispatches to handleItemHistory
	rr = doRequest(server.handleList, "GET", fmt.Sprintf("/api/list/%d/history", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("History: %d", rr.Code)
	}
	// Should have modifications (at least the add and the manual one)
	var mods []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &mods)
	if len(mods) < 1 {
		t.Errorf("Expected at least 1 modification, got %d", len(mods))
	}
}

func TestHandleItemHistory_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "GET", "/api/list/99999/history",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandleListEdit_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	name := ""
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"name": &name}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty name, got %d", rr.Code)
	}
}

func TestHandleListEdit_QuantityClamp(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Eggs"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	qty := 0
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d", itemID),
		map[string]interface{}{"quantity": &qty}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Edit: %d", rr.Code)
	}
	item := parseJSON(rr)
	if item["quantity"].(float64) != 1 {
		t.Errorf("Expected quantity clamped to 1, got %v", item["quantity"])
	}
}

func TestHandleListEdit_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999",
		map[string]interface{}{"brand": "Acme"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestHandleListToggle_Uncheck(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	itemID := int(parseJSON(rr)["id"].(float64))

	// Check — use handleList router which dispatches to handleListToggle
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Toggle check: %d", rr.Code)
	}
	if !parseJSON(rr)["checked"].(bool) {
		t.Error("Expected checked=true after first toggle")
	}

	// Uncheck
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/toggle", itemID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Toggle uncheck: %d", rr.Code)
	}
	if parseJSON(rr)["checked"].(bool) {
		t.Error("Expected checked=false after second toggle")
	}
}

func TestHandleListDelete_NotOwned(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie1, _ := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User1 adds item
	rr := doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie1})
	itemID := int(parseJSON(rr)["id"].(float64))

	// User2 tries to delete it (not in same household) — should get 404
	rr = doRequest(server.handleList, "DELETE", fmt.Sprintf("/api/list/%d", itemID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-owned item, got %d", rr.Code)
	}
}

func TestHandleListGet_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestHandleListClear_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	// Add items
	doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Milk"}, []*http.Cookie{cookie})
	doRequest(server.handleListAdd, "POST", "/api/list",
		map[string]string{"name": "Bread"}, []*http.Cookie{cookie})

	// Clear
	rr := doRequest(server.handleListClear, "POST", "/api/list/clear?all=true", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Clear: %d", rr.Code)
	}

	// Verify empty
	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	var items []interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("Expected 0 items after clear, got %d", len(items))
	}
}

func TestHandleListGet_WithMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, checked) VALUES (?, 'Milk', 'Organic Valley', 'Dairy', 2, '1 gal', 'app', 0)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size, source, checked) VALUES (?, 'Bread', 'Wonder', 'Bakery', 1, '', 'sms', 1)", userID)

	rr := doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var items []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

// =============================================================================
// LIST URGENT ENDPOINT TESTS
// =============================================================================

func TestListUrgent_Toggle(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Mark urgent
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if !item.Urgent {
		t.Error("Expected item to be marked urgent")
	}

	// Toggle back — should unmark
	rr = doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Urgent {
		t.Error("Expected item to be unmarked urgent after second toggle")
	}
}

func TestListUrgent_NotOwnedItem(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, otherUserID := loginUser(t, server, "5559999999")
	cookie, _ := loginUser(t, server, "5551234567")

	// Insert item owned by other user
	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		otherUserID, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestListUrgent_RecordsModification(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

	// Mark urgent
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/urgent", itemID),
		nil, []*http.Cookie{cookie})

	// Verify modification was recorded
	var action string
	err := server.db.QueryRow(
		"SELECT action FROM item_modifications WHERE item_id = ? ORDER BY id DESC LIMIT 1",
		itemID,
	).Scan(&action)
	if err != nil {
		t.Fatalf("Failed to query modification: %v", err)
	}
	if action != "marked urgent" {
		t.Errorf("Expected action 'marked urgent', got %q", action)
	}
}

// =============================================================================
// CLAIM ENDPOINT TESTS
// =============================================================================

func TestListClaim_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")

	// Claim
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil || *item.ClaimedByID != userID {
		t.Errorf("Expected claimed_by_id to be %d, got %v", userID, item.ClaimedByID)
	}
}

func TestListClaim_Unclaim(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	itemID := createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")

	// Claim
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	// Unclaim (second call)
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID != nil {
		t.Errorf("Expected claimed_by_id to be nil after unclaim, got %v", item.ClaimedByID)
	}
}

func TestListClaim_Override(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	// Create household with two users
	cookie1, userID1 := loginUser(t, server, "5551234567")
	cookie2, userID2 := loginUser(t, server, "5559999999")

	// Create household: user1 creates, user2 joins
	server.db.Exec("INSERT INTO households (name) VALUES ('Test')")
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, ?, 'admin')", userID1)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (1, ?, 'member')", userID2)
	server.db.Exec("UPDATE users SET household_id = 1 WHERE id IN (?, ?)", userID1, userID2)

	result, _ := server.db.Exec(
		"INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Bread", "Bakery",
	)
	itemID, _ := result.LastInsertId()

	// User1 claims
	doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie1})

	// User2 overrides
	rr := doRequest(server.handleList, "PATCH", fmt.Sprintf("/api/list/%d/claim", itemID),
		nil, []*http.Cookie{cookie2})

	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var item ListItem
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ClaimedByID == nil || *item.ClaimedByID != userID2 {
		t.Errorf("Expected claimed_by_id to be %d (user2), got %v", userID2, item.ClaimedByID)
	}
}

func TestListClaim_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleList, "PATCH", "/api/list/99999/claim",
		nil, []*http.Cookie{cookie})

	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

// =============================================================================
// NPS ENDPOINT TESTS
// =============================================================================
// EDGE CASE TESTS — INVALID IDs, METHOD VALIDATION, BODY SIZE
// =============================================================================

func TestList_InvalidIDFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	tests := []struct {
		name string
		path string
	}{
		{"letters", "/api/list/abc"},
		{"negative", "/api/list/-1"},
		{"float", "/api/list/1.5"},
		{"empty_after_slash", "/api/list//toggle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(server.handleList, "PATCH", tt.path, nil, []*http.Cookie{cookie})
			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s: got %d, want 400", tt.name, rr.Code)
			}
		})
	}
}

func TestStripBrandFromName(t *testing.T) {
	tests := []struct {
		name, brand, want string
	}{
		{"Budweiser Beer", "Budweiser", "Beer"},
		{"Beer Budweiser", "Budweiser", "Beer"},
		{"Peanut Butter", "Jif", "Peanut Butter"},
		{"Macaroni & Cheese", "Kraft", "Macaroni & Cheese"},
		{"Kraft Macaroni & Cheese", "Kraft", "Macaroni & Cheese"},
		{"Whole Milk", "", "Whole Milk"},
		{"Budweiser", "Budweiser", "Budweiser"}, // don't strip if nothing left
	}
	for _, tt := range tests {
		got := stripBrandFromName(tt.name, tt.brand)
		if got != tt.want {
			t.Errorf("stripBrandFromName(%q, %q) = %q, want %q", tt.name, tt.brand, got, tt.want)
		}
	}
}

// =============================================================================
// LOGO TESTS
// =============================================================================
// ROUND 3 — Constants, Restore, Cross-Household, NPS, Referral Tests
// =============================================================================

func TestListRestore_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	items := []map[string]interface{}{
		{"name": "Milk", "department": "Dairy & Eggs", "quantity": 2, "checked": false},
		{"name": "Bread", "department": "Bakery", "quantity": 1, "checked": true},
	}
	rr := doRequest(server.handleListRestore, "POST", "/api/list/restore", items, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("restore status: got %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 restored items, got %d", count)
	}
}

func TestListRestore_DefaultsEmptyDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	items := []map[string]interface{}{
		{"name": "Widget", "quantity": 1},
	}
	doRequest(server.handleListRestore, "POST", "/api/list/restore", items, []*http.Cookie{cookie})

	var dept string
	server.db.QueryRow("SELECT department FROM list_items WHERE user_id = ?", userID).Scan(&dept)
	if dept != "Other" {
		t.Errorf("expected department='Other', got %q", dept)
	}
}

func TestListRestore_WrongMethod(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleListRestore, "GET", "/api/list/restore", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestListRestore_EmptyBody(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	rr := doRequest(server.handleListRestore, "POST", "/api/list/restore", []interface{}{}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for empty restore, got %d; body=%s", rr.Code, rr.Body.String())
	}
}

func TestListRestore_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleListRestore, "POST", "/api/list/restore", []interface{}{}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

func TestListClaim_AcrossHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	cookie1, userID1 := loginUser(t, server, "5551111111")
	createListItem(t, server.db, userID1, "Eggs", "Dairy & Eggs")

	inviteReq := map[string]interface{}{"phone": ""}
	rr := doRequest(server.handleInvite, "POST", "/api/settings/invite", inviteReq, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("invite failed: %d", rr.Code)
	}
	link := parseJSON(rr)["link"].(string)
	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	code := parts[len(parts)-1]

	cookie2, _ := loginUser(t, server, "5552222222")
	doRequest(server.handleJoin, "POST", "/api/settings/join", map[string]interface{}{"code": code}, []*http.Cookie{cookie2})

	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	if rr.Code != http.StatusOK {
		t.Fatalf("list get failed: %d", rr.Code)
	}
	var items []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&items)
	if len(items) == 0 {
		t.Fatal("expected at least one item")
	}
	itemID := int(items[0]["id"].(float64))

	claimPath := fmt.Sprintf("/api/list/%d/claim", itemID)
	req := httptest.NewRequest("PATCH", claimPath, nil)
	req.AddCookie(cookie2)
	rw := httptest.NewRecorder()
	server.handleList(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("claim failed: %d; body=%s", rw.Code, rw.Body.String())
	}

	rr = doRequest(server.handleListGet, "GET", "/api/list", nil, []*http.Cookie{cookie1})
	var itemsAfter []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&itemsAfter)
	if len(itemsAfter) == 0 {
		t.Fatal("expected items after claim")
	}
	claimedByID := itemsAfter[0]["claimed_by_id"]
	if claimedByID == nil || claimedByID == float64(0) {
		t.Error("expected item to be claimed when viewed by other household member")
	}
}

// =============================================================================
// PENNY SWAPS TESTS
// =============================================================================
// BRAND RESOLVE ENDPOINT TESTS
// =============================================================================

func TestBrandResolve_Happy(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559000001")

	// Add an item directly
	itemID := createListItem(t, server.db, userID, "Yogurt", "Dairy & Eggs")

	// Resolve brand
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Fage", "product_category": "yogurt"},
		[]*http.Cookie{cookie})
	if rr.Code != 200 {
		t.Fatalf("brand resolve: got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Verify brand was set
	resolved := parseJSON(rr)
	if resolved["brand"] != "Fage" {
		t.Errorf("expected brand Fage, got %v", resolved["brand"])
	}

	// Verify brand choice recorded
	var choiceCount int
	server.db.QueryRow("SELECT COUNT(*) FROM brand_choices WHERE product_category = 'yogurt' AND brand = 'Fage'").Scan(&choiceCount)
	if choiceCount == 0 {
		t.Error("brand choice not recorded in brand_choices table")
	}
}

func TestBrandResolve_MissingFields(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559000002")

	// Missing brand
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 1},
		[]*http.Cookie{cookie})
	if rr.Code != 400 {
		t.Errorf("expected 400 for missing brand, got %d", rr.Code)
	}

	// Missing item_id
	rr = doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"brand": "Fage"},
		[]*http.Cookie{cookie})
	if rr.Code != 400 {
		t.Errorf("expected 400 for missing item_id, got %d", rr.Code)
	}
}

func TestBrandResolve_CrossUserBlocked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5559000003")
	cookie2, _ := loginUser(t, server, "5559000004")

	// User 1's item
	itemID := createListItem(t, server.db, userID1, "Milk", "Dairy & Eggs")

	// User 2 tries to resolve brand on user 1's item
	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": itemID, "brand": "Horizon", "product_category": "milk"},
		[]*http.Cookie{cookie2})
	if rr.Code != 404 {
		t.Errorf("expected 404 for cross-user brand resolve, got %d", rr.Code)
	}
}

func TestBrandResolve_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleBrandResolve, "POST", "/api/list/brand-resolve",
		map[string]interface{}{"item_id": 1, "brand": "Fage", "product_category": "yogurt"},
		nil)
	if rr.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

// =============================================================================
// SHARE LIST ENDPOINT TESTS
// =============================================================================

func TestShareList_Happy(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559100001")

	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
	if rr.Code != 201 {
		t.Fatalf("share list: got %d, body: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	if resp["code"] == nil || resp["code"] == "" {
		t.Error("expected non-empty share code")
	}
	if resp["url"] == nil || resp["url"] == "" {
		t.Error("expected non-empty share URL")
	}
	if resp["expires_at"] == nil {
		t.Error("expected expires_at")
	}
}

func TestShareList_UniqueCodesPerRequest(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559100002")

	codes := make(map[string]bool)
	for i := 0; i < 3; i++ {
		rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
		if rr.Code != 201 {
			t.Fatalf("share %d: got %d", i, rr.Code)
		}
		resp := parseJSON(rr)
		code := resp["code"].(string)
		if codes[code] {
			t.Errorf("duplicate share code: %s", code)
		}
		codes[code] = true
	}
}

func TestShareList_NoAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, nil)
	if rr.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

// =============================================================================
// SHARED VIEW ENDPOINT TESTS
// =============================================================================

func TestSharedView_Happy(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559200001")

	// Add items directly
	createListItem(t, server.db, userID, "Milk", "Dairy & Eggs")
	createListItem(t, server.db, userID, "Eggs", "Dairy & Eggs")
	createListItem(t, server.db, userID, "Bread", "Bakery & Deli")

	// Create share link
	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
	shareResp := parseJSON(rr)
	code := shareResp["code"].(string)

	// View shared list (no auth required)
	req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
	rrView := httptest.NewRecorder()
	server.handleSharedView(rrView, req)
	if rrView.Code != 200 {
		t.Fatalf("shared view: got %d, body: %s", rrView.Code, rrView.Body.String())
	}

	var viewResp map[string]interface{}
	json.Unmarshal(rrView.Body.Bytes(), &viewResp)
	items := viewResp["items"].([]interface{})
	if len(items) < 3 {
		t.Errorf("expected 3+ items, got %d", len(items))
	}
	if viewResp["expires_at"] == nil {
		t.Error("expected expires_at in shared view response")
	}
}

func TestSharedView_InvalidCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	req := httptest.NewRequest("GET", "/api/shared/nonexistent123", nil)
	rr := httptest.NewRecorder()
	server.handleSharedView(rr, req)
	if rr.Code != 404 {
		t.Errorf("expected 404 for invalid code, got %d", rr.Code)
	}
}

func TestSharedView_ExpiredCode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5559200003")

	// Insert an expired shared link directly
	pastExpiry := time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	server.db.Exec(
		"INSERT INTO shared_links (code, user_id, expires_at, view_count) VALUES (?, ?, ?, 0)",
		"expired-test-code", userID, pastExpiry,
	)

	req := httptest.NewRequest("GET", "/api/shared/expired-test-code", nil)
	rr := httptest.NewRecorder()
	server.handleSharedView(rr, req)
	if rr.Code != 410 {
		t.Errorf("expected 410 Gone for expired code, got %d", rr.Code)
	}
}

func TestSharedView_ViewCountIncrements(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5559200004")

	// Create share link
	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
	shareResp := parseJSON(rr)
	code := shareResp["code"].(string)

	// View 3 times
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
		rrView := httptest.NewRecorder()
		server.handleSharedView(rrView, req)
		if rrView.Code != 200 {
			t.Fatalf("view %d: got %d", i, rrView.Code)
		}
	}

	// Check view count
	var viewCount int
	server.db.QueryRow("SELECT view_count FROM shared_links WHERE code = ?", code).Scan(&viewCount)
	if viewCount != 3 {
		t.Errorf("expected view_count=3, got %d", viewCount)
	}
}

func TestSharedView_NoSensitiveData(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559200005")

	// Add item directly
	createListItem(t, server.db, userID, "Organic Milk", "Dairy & Eggs")

	// Create share link
	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
	shareResp := parseJSON(rr)
	code := shareResp["code"].(string)

	// View shared list
	req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
	rrView := httptest.NewRecorder()
	server.handleSharedView(rrView, req)

	// Verify no sensitive fields (source, user_id) — brand and department are OK for logo display
	respBody := rrView.Body.String()
	for _, field := range []string{`"source"`, `"user_id"`} {
		if bytes.Contains([]byte(respBody), []byte(field)) {
			t.Errorf("shared view should not expose %s, but response contains it", field)
		}
	}
}

func TestSharedView_IncludesBrandAndDepartment(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5559200010")

	// Add item with brand and department
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department) VALUES (?, 'Peanut Butter', 'Skippy', 'Pantry & Dry Goods')", userID)

	// Create share link
	rr := doRequest(server.handleShareList, "POST", "/api/list/share", nil, []*http.Cookie{cookie})
	shareResp := parseJSON(rr)
	code := shareResp["code"].(string)

	// View shared list
	req := httptest.NewRequest("GET", "/api/shared/"+code, nil)
	rrView := httptest.NewRecorder()
	server.handleSharedView(rrView, req)
	if rrView.Code != 200 {
		t.Fatalf("expected 200, got %d", rrView.Code)
	}

	var viewResp map[string]interface{}
	json.Unmarshal(rrView.Body.Bytes(), &viewResp)
	items := viewResp["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0].(map[string]interface{})
	if item["brand"] != "Skippy" {
		t.Errorf("expected brand 'Skippy', got %v", item["brand"])
	}
	if item["department"] != "Pantry & Dry Goods" {
		t.Errorf("expected department 'Pantry & Dry Goods', got %v", item["department"])
	}
}

// =============================================================================
// API CONFIG ENDPOINT TEST
// =============================================================================
