package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


func TestInvalidID_SavedListEndpoints(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
		want   int
	}{
		{"update", "PUT", "/api/saved-lists/abc", map[string]string{"name": "x"}, 400},
		{"delete", "DELETE", "/api/saved-lists/abc", nil, 400},
		{"load", "POST", "/api/saved-lists/abc/load", nil, 400},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(server.handleSavedLists, tc.method, tc.path, tc.body, []*http.Cookie{cookie})
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

// =============================================================================
// SAVED LISTS TESTS
// =============================================================================

func TestSavedLists_GetEmpty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("Expected 0 saved lists, got %d", len(lists))
	}
}

func TestSavedLists_CreateAndGet(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add some items to the list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Milk", "Organic Valley", "Dairy & Eggs", 2, "1 gal")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")

	// Save the list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly Groceries"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Create status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["name"] != "Weekly Groceries" {
		t.Errorf("Name: got %v, want Weekly Groceries", body["name"])
	}
	if body["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v, want 2", body["item_count"])
	}

	// Get saved lists
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Fatalf("Expected 1 saved list, got %d", len(lists))
	}
	if lists[0]["name"] != "Weekly Groceries" {
		t.Errorf("Name: got %v", lists[0]["name"])
	}
	if lists[0]["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v", lists[0]["item_count"])
	}
}

func TestSavedLists_CreateEmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "  "},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_CreateNoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// No items at all on the list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Empty List"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_CreateSavesAllItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 0)",
		userID, "Milk", "Dairy & Eggs")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, checked) VALUES (?, ?, ?, 1)",
		userID, "Bread", "Bakery")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Full List"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}

	body := parseJSON(rr)
	if body["item_count"].(float64) != 2 {
		t.Errorf("Item count: got %v, want 2 (all items)", body["item_count"])
	}
}

func TestSavedLists_Delete(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	// Create
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "To Delete"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Delete
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Delete status: got %d, want 200", rr.Code)
	}

	// Verify items are also gone (cascade)
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 saved list items after cascade delete, got %d", count)
	}

	// Verify list is gone
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("Expected 0 saved lists after delete, got %d", len(lists))
	}
}

func TestSavedLists_DeleteNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists/99999",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_Update(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Change the active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs")

	// Update saved list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Update status: got %d, want 200", rr.Code)
	}

	updateBody := parseJSON(rr)
	if updateBody["item_count"].(float64) != 2 {
		t.Errorf("Item count after update: got %v, want 2", updateBody["item_count"])
	}

	// Verify old items replaced
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", savedID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 saved list items after update, got %d", count)
	}
}

func TestSavedLists_UpdateNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/99999",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_Load(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, ?, ?, ?, ?, ?)",
		userID, "Milk", "Organic Valley", "Dairy & Eggs", 1, "1 gal")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bread", "Bakery")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Clear active list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)

	// Load saved list
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	loadBody := parseJSON(rr)
	if loadBody["added"].(float64) != 2 {
		t.Errorf("Added: got %v, want 2", loadBody["added"])
	}

	// Verify items are in active list
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 active list items after load, got %d", count)
	}
}

func TestSavedLists_LoadNonexistent(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/99999/load",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedLists_CrossUserIsolation(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	// User 1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "User1 List"},
		[]*http.Cookie{cookie1})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// User 2 tries to load user 1's list
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Load other user's list: got %d, want 404", rr.Code)
	}

	// User 2 tries to delete user 1's list
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Delete other user's list: got %d, want 404", rr.Code)
	}

	// User 2 tries to update user 1's list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Update other user's list: got %d, want 404", rr.Code)
	}

	// User 2 sees no saved lists
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie2})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 0 {
		t.Errorf("User 2 should see 0 saved lists, got %d", len(lists))
	}
}

func TestSavedLists_InvalidID(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedLists_LoadOverwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Add a different item to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Eggs", "Dairy & Eggs")

	// Load with overwrite — should clear Eggs and Milk, then add back Milk from saved
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	// Should have only the saved list item (Milk), not Eggs
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite load, got %d", count)
	}

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", userID).Scan(&name)
	if name != "Milk" {
		t.Errorf("Expected Milk, got %q", name)
	}
}

func TestSavedLists_LoadMergesWithExisting(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		userID, "Milk", "Dairy & Eggs", 1)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// Load into list that already has Milk — should merge (upsert)
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Load status: got %d, want 200", rr.Code)
	}

	// Should still be 1 row but with quantity 2
	var count, qty int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 list item (merged), got %d", count)
	}
	server.db.QueryRow("SELECT quantity FROM list_items WHERE user_id = ? AND name = 'Milk'", userID).Scan(&qty)
	if qty != 2 {
		t.Errorf("Expected quantity 2 after merge, got %d", qty)
	}
}

// =============================================================================
// SMS WEBHOOK TESTS
// =============================================================================

// doSMSRequest sends a Twilio-style form POST to the SMS webhook handler.


func TestSavedListSharedAcrossHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// Put user2 in user1's household
	var hID int
	server.db.QueryRow("SELECT household_id FROM users WHERE id = ?", userID1).Scan(&hID)
	server.db.Exec("UPDATE users SET household_id = ? WHERE id = ?", hID, userID2)
	server.db.Exec("DELETE FROM household_members WHERE user_id = ?", userID2)
	server.db.Exec("INSERT INTO household_members (household_id, user_id, role) VALUES (?, ?, 'member')", hID, userID2)

	// User1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID1, "Milk", "Dairy & Eggs")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Shared List"},
		[]*http.Cookie{cookie1})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	savedID := int(body["id"].(float64))

	// User2 should see the saved list
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie2})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Errorf("User2 should see 1 saved list, got %d", len(lists))
	}

	// User2 should be able to load it
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("User2 load: got %d, want 200", rr.Code)
	}

	// User2 should be able to delete it
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusOK {
		t.Errorf("User2 delete: got %d, want 200", rr.Code)
	}
}

func TestRequireSavedListAccess_NotFound(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5551234567")

	rr := httptest.NewRecorder()
	ok := server.requireSavedListAccess(rr, userID, 99999)
	if ok {
		t.Error("Expected false for non-existent list")
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestRequireSavedListAccess_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	_, userID2 := loginUser(t, server, "5552222222")

	// User1 creates a saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID1)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID1)
	listID, _ := result.LastInsertId()

	// User2 tries to access user1's list
	rr := httptest.NewRecorder()
	ok := server.requireSavedListAccess(rr, userID2, int(listID))
	if ok {
		t.Error("Expected false for wrong household")
	}
}

// --- Saved list handler edge cases ---

func TestSavedListsCreate_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Empty List"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListsCreate_EmptyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": ""},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListUpdate_NoItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list with items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", listID)

	// Clear list items so update has no items to save
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

func TestSavedListUpdate_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy & Eggs', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Butter', 'Dairy & Eggs', 1)", userID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["item_count"] != float64(2) {
		t.Errorf("item_count: got %v, want 2", body["item_count"])
	}
}

func TestSavedListUpdate_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, userID2 := loginUser(t, server, "5552222222")

	// User1 creates a saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	// User2 adds items and tries to update user1's list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy & Eggs', 1)", userID2)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedListLoad_Overwrite(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Eggs', '', 'Dairy & Eggs', 12, '')", listID)

	// Add existing items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Old Item', 'Other', 1)", userID)

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}
	body := parseJSON(rr)
	if body["added"] != float64(1) {
		t.Errorf("added: got %v, want 1", body["added"])
	}

	// Old items should be gone
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Old Item'", userID).Scan(&count)
	if count != 0 {
		t.Error("Old items should have been cleared")
	}
}

func TestSavedListLoad_Merge(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Eggs', '', 'Dairy & Eggs', 12, '')", listID)

	// Add existing items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Old Item', 'Other', 1)", userID)

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=merge", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	// Both old and new items should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items (old + new), got %d", count)
	}
}

func TestSavedListLoad_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestSavedListDelete_WrongHousehold(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID1 := loginUser(t, server, "5551111111")
	cookie2, _ := loginUser(t, server, "5552222222")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Private')", userID1)
	listID, _ := result.LastInsertId()

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

// --- handleInvite edge cases ---

// --- handleListClear tests ---

// --- handleSavedListsGet tests ---

func TestSavedListsGet_Empty(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}
	// Should return empty array
	if !strings.Contains(rr.Body.String(), "[]") {
		t.Errorf("Expected empty array, got %q", rr.Body.String())
	}
}

func TestSavedListsGet_WithLists(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Monthly')", userID)

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Errorf("Expected 2 lists, got %d", len(lists))
	}
}

// --- handleSavedLists routing tests ---

func TestSavedLists_Unauthorized(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status: got %d, want 401", rr.Code)
	}
}

// --- processSMSMessage with skipped items ---

// --- handleSavedListLoad error paths ---

func TestSavedListLoad_QueryError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()

	// Drop the saved_list_items table to cause a query error
	server.db.Exec("DROP TABLE saved_list_items")

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Status: got %d, want 500", rr.Code)
	}
}

// --- handleSavedListsCreate with items ---

func TestSavedListsCreate_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to list
	server.db.Exec("INSERT INTO list_items (user_id, name, brand, department, quantity, size) VALUES (?, 'Milk', 'Organic Valley', 'Dairy & Eggs', 1, '1 gal')", userID)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Errorf("Status: got %d, want 201", rr.Code)
	}
	body := parseJSON(rr)
	if body["name"] != "Weekly" {
		t.Errorf("name: got %v, want Weekly", body["name"])
	}
	if body["item_count"] != float64(1) {
		t.Errorf("item_count: got %v, want 1", body["item_count"])
	}
}

// --- handleListClear success ---

// --- handleSavedListDelete success ---

func TestSavedListDelete_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'ToDelete')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", listID)

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Errorf("Status: got %d, want 200", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_lists WHERE id = ?", listID).Scan(&count)
	if count != 0 {
		t.Error("Expected saved list to be deleted")
	}
}

// --- handleSettingsUpdate name and email together ---

// --- handleSavedListsCreate invalid JSON ---

func TestSavedListsCreate_InvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	req := httptest.NewRequest("POST", "/api/saved-lists", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	server.handleSavedLists(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status: got %d, want 400", rr.Code)
	}
}

// --- handleVCard success ---

func TestSavedListsGet_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListsCreate_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Test"},
		[]*http.Cookie{cookie})
	// Could be 400 (collectListItems error) or 500
	if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
		t.Errorf("Expected error status, got %d", rr.Code)
	}
}

func TestSavedListUpdate_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListDelete_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListLoad_DBError(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test')", userID)
	listID, _ := result.LastInsertId()
	server.db.Close()

	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("Expected error status when DB is closed")
	}
}

func TestSavedListLoad_OverwriteClearsOldItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Bread', 'Bakery')", userID)

	// Create a saved list with different items
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test List')", userID)
	savedID, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department) VALUES (?, 'Eggs', 'Dairy & Eggs')", savedID)

	// Load with overwrite
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Old items should be gone, only new item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite, got %d", count)
	}

	var name string
	server.db.QueryRow("SELECT name FROM list_items WHERE user_id = ?", userID).Scan(&name)
	if name != "Eggs" {
		t.Errorf("Expected 'Eggs', got %q", name)
	}
}

func TestSavedListLoad_MergeKeepsBothItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to active list
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy & Eggs')", userID)

	// Create a saved list
	res, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Test List')", userID)
	savedID, _ := res.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department) VALUES (?, 'Eggs', 'Dairy & Eggs')", savedID)

	// Load with merge (default)
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedID), nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both items should exist
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items after merge, got %d", count)
	}
}

// =============================================================================
// CLEAR LIST VIA POST
// =============================================================================

func TestSavedListsGet_Sorted(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create two saved lists
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Beta List"}, []*http.Cookie{cookie})
	doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Alpha List"}, []*http.Cookie{cookie})

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Fatalf("Expected 2 lists, got %d", len(lists))
	}
}

func TestSavedListLoad_MergeMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add current items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)

	// Create saved list with different items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", listID)

	// Load in merge mode (default)
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	// Should have both items
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 items after merge, got %d", count)
	}
}

func TestSavedListLoad_OverwriteMode(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add current items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)

	// Create saved list with different items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 1, '')", listID)

	// Load in overwrite mode
	rr := doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	// Should have only the saved list item
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 item after overwrite, got %d", count)
	}
}

func TestSavedListsCreate_WithMultipleItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add multiple items
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, brand, size) VALUES (?, 'Milk', 'Dairy', 2, 'Organic Valley', '1 gallon')", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity, brand, size) VALUES (?, 'Bread', 'Bakery', 1, '', 'loaf')", userID)

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly Shopping"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: %d, body: %s", rr.Code, rr.Body.String())
	}

	body := parseJSON(rr)
	if body["item_count"].(float64) != 2 {
		t.Errorf("Expected 2 items, got %v", body["item_count"])
	}
}

func TestSavedListUpdate_WithNewItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create saved list with one item
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Old List')", userID)
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity, brand, size) VALUES (?, 'OldItem', 'Other', 1, '', '')", listID)

	rr := doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	body := parseJSON(rr)
	if body["item_count"].(float64) != 1 {
		t.Errorf("Expected 1 item, got %v", body["item_count"])
	}
}

func TestSavedListDelete_Success_Cascade(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, 'Milk', 'Dairy')", userID)
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Deletable"},
		[]*http.Cookie{cookie})
	body := parseJSON(rr)
	id := int(body["id"].(float64))

	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", id),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}

	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", id).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 saved list items after delete, got %d", count)
	}
}

func TestSavedLists_NotFoundRoute(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, _ := loginUser(t, server, "5551234567")

	// PATCH on a saved list ID is not a valid route
	rr := doRequest(server.handleSavedLists, "PATCH", "/api/saved-lists/1", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

func TestSavedListUpdate_FullFlow(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to current list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Bread', 'Bakery', 2)", userID)

	// Create saved list
	rr := doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create: %d", rr.Code)
	}
	listID := int(parseJSON(rr)["id"].(float64))

	// Add a new item to current list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy', 12)", userID)

	// Update the saved list
	rr = doRequest(server.handleSavedLists, "PUT", fmt.Sprintf("/api/saved-lists/%d", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Update: %d", rr.Code)
	}

	// Verify updated count
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", listID).Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 saved items after update, got %d", count)
	}
}

func TestSavedListsGet_WithItems(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add item and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "My List"}, []*http.Cookie{cookie})

	// Get saved lists
	rr := doRequest(server.handleSavedListsGet, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: %d", rr.Code)
	}
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Errorf("Expected 1 list, got %d", len(lists))
	}
}

func TestSavedListLoad_OverwriteWithExisting(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{CheckApproved: true})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items and create saved list
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Milk', 'Dairy', 1)", userID)
	rr := doRequest(server.handleSavedListsCreate, "POST", "/api/saved-lists",
		map[string]string{"name": "Weekly"}, []*http.Cookie{cookie})
	listID := int(parseJSON(rr)["id"].(float64))

	// Add different items to current list
	server.db.Exec("DELETE FROM list_items WHERE user_id = ?", userID)
	server.db.Exec("INSERT INTO list_items (user_id, name, department, quantity) VALUES (?, 'Eggs', 'Dairy', 12)", userID)

	// Load saved list in overwrite mode
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", listID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Load: %d, body: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["added"].(float64) != 1 {
		t.Errorf("Expected 1 item added, got %v", body["added"])
	}
}

func TestSavedLists_InvalidIDFormat(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Non-numeric IDs should return 400
	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists/abc", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("letters: got %d, want 400", rr.Code)
	}

	// Negative IDs parse fine but return 404 (not found in DB)
	rr = doRequest(server.handleSavedLists, "GET", "/api/saved-lists/-1", nil, []*http.Cookie{cookie})
	if rr.Code == http.StatusOK {
		t.Error("negative ID should not return 200")
	}
}

// =============================================================================
// SAVED LISTS METHOD VALIDATION TESTS
// =============================================================================

func TestSavedLists_RootPUT_Returns405(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists",
		map[string]string{"name": "Test"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/saved-lists: got %d, want 405", rr.Code)
	}
}

func TestSavedLists_RootDELETE_Returns405(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists",
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/saved-lists: got %d, want 405", rr.Code)
	}
}

// =============================================================================
// SETTINGS EDGE CASE TESTS
// =============================================================================

func TestSavedListLoad_OverwriteClearsExisting(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Add an existing item to the active list
	server.db.Exec(
		"INSERT INTO list_items (user_id, name, brand, department, quantity, size, source) VALUES (?, 'Milk', '', 'Dairy', 1, '', 'app')",
		user.ID,
	)

	// Create a saved list with a different item
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Weekly')", user.ID)
	savedID, _ := result.LastInsertId()
	server.db.Exec(
		"INSERT INTO saved_list_items (saved_list_id, name, brand, department, quantity, size) VALUES (?, 'Bread', '', 'Bakery', 2, '')",
		savedID,
	)

	// Load with overwrite
	rr := doRequest(
		server.handleSavedLists,
		"POST",
		fmt.Sprintf("/api/saved-lists/%d/load?mode=overwrite", savedID),
		nil,
		cookies,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	body := parseJSON(rr)
	if body["added"] != float64(1) {
		t.Errorf("added = %v, want 1", body["added"])
	}

	// The old item (Milk) should be deleted
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items WHERE user_id = ? AND name = 'Milk'", user.ID).Scan(&count)
	if count != 0 {
		t.Error("expected Milk to be deleted on overwrite")
	}
}

func TestSavedListUpdate_EmptyList(t *testing.T) {
	mock := &MockTwilioClient{CheckApproved: true}
	server := newTestServer(t, mock)

	user, _ := upsertUser(server.db, "5551234567")
	token, _ := server.sessions.CreateSession(user.ID, "5551234567", "phone")
	cookies := []*http.Cookie{{Name: "session", Value: token}}

	// Create an empty saved list
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, 'Empty')", user.ID)
	savedID, _ := result.LastInsertId()

	// Try to update with empty active list → should get 400
	rr := doRequest(
		server.handleSavedLists,
		"PUT",
		fmt.Sprintf("/api/saved-lists/%d", savedID),
		nil,
		cookies,
	)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// =============================================================================
// SHOPPING TRIPS
// =============================================================================
// REQUIRE SAVED LIST ACCESS TESTS
// =============================================================================

func TestRequireSavedListAccess_MissingList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5559990030")

	rr := httptest.NewRecorder()

	ok := server.requireSavedListAccess(rr, userID, 99999)
	if ok {
		t.Error("Expected false for non-existent list ID")
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", rr.Code)
	}
}

func TestRequireSavedListAccess_OtherUsersListDenied(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, user1ID := loginUser(t, server, "5559990031")
	_, user2ID := loginUser(t, server, "5559990032")

	// Create a saved list belonging to user1
	res, err := server.db.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)", user1ID, "User1 List",
	)
	if err != nil {
		t.Fatalf("insert saved_list: %v", err)
	}
	listID, _ := res.LastInsertId()

	// user2 should not be able to access it
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	_ = req

	ok := server.requireSavedListAccess(rr, user2ID, int(listID))
	if ok {
		t.Error("Expected false when accessing another user's saved list")
	}
}

func TestRequireSavedListAccess_OwnerCanAccess(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	_, userID := loginUser(t, server, "5559990033")

	// Create a saved list belonging to the user
	res, err := server.db.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)", userID, "My List",
	)
	if err != nil {
		t.Fatalf("insert saved_list: %v", err)
	}
	listID, _ := res.LastInsertId()

	rr := httptest.NewRecorder()
	ok := server.requireSavedListAccess(rr, userID, int(listID))
	if !ok {
		t.Error("Expected true when owner accesses their own saved list")
	}
}

// =============================================================================
// REFERRAL HANDLER TESTS
// =============================================================================
// SAVED LISTS EDGE CASES
// =============================================================================

func TestSavedLists_EdgeCase_WhitespaceOnlyName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Whitespace-only name should be rejected (trimmed to empty)
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "   "}, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with whitespace name: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["error"] != "Name is required" {
		t.Errorf("Expected 'Name is required' error, got %v", result["error"])
	}
}

func TestSavedLists_EdgeCase_VeryLongName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add an item so save can succeed
	createListItem(t, server.db, userID, "Milk", "Dairy")

	longName := strings.Repeat("a", 500)
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": longName}, []*http.Cookie{cookie})
	// Should succeed — SQLite TEXT has no length limit
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST with long name: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	if result["name"] != longName {
		t.Errorf("Expected long name to be stored verbatim, got length %d", len(result["name"].(string)))
	}
}

func TestSavedLists_EdgeCase_LoadNonexistentList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Loading a saved list that doesn't exist should return 404
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/99999/load", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Load nonexistent saved list: got %d, want 404. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSavedLists_EdgeCase_DeleteNonexistentList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Deleting a saved list that doesn't exist should return 404
	rr := doRequest(server.handleSavedLists, "DELETE", "/api/saved-lists/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Delete nonexistent saved list: got %d, want 404. Body: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["error"] != "Saved list not found" {
		t.Errorf("Expected 'Saved list not found' error, got %v", result["error"])
	}
}

func TestSavedLists_EdgeCase_UpdateNonexistentList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Updating a saved list that doesn't exist should return 404
	rr := doRequest(server.handleSavedLists, "PUT", "/api/saved-lists/99999", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Update nonexistent saved list: got %d, want 404. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSavedLists_EdgeCase_LoadEmptySavedList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create an item so saved list can be created
	createListItem(t, server.db, userID, "Bread", "Bakery")

	// Create a saved list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Test List"}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create saved list: got %d, want 201", rr.Code)
	}
	result := parseJSON(rr)
	savedListID := int(result["id"].(float64))

	// Manually delete all items from the saved list to simulate an empty one
	server.db.Exec("DELETE FROM saved_list_items WHERE saved_list_id = ?", savedListID)

	// Load the now-empty saved list — should succeed with 0 items added
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/load", savedListID),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Load empty saved list: got %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	result = parseJSON(rr)
	if result["added"] != float64(0) {
		t.Errorf("Expected 0 items added from empty saved list, got %v", result["added"])
	}
}

func TestSavedLists_EdgeCase_SpecialCharsInName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	createListItem(t, server.db, userID, "Eggs", "Dairy")

	specialName := "My List <script>alert('xss')</script> & \"quotes\""
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": specialName}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST with special chars name: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify stored as-is (no mutation of the name)
	result := parseJSON(rr)
	if result["name"] != specialName {
		t.Errorf("Expected name stored verbatim, got %q", result["name"])
	}
}

func TestSavedLists_EdgeCase_CreateWithInvalidJSON(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Send nil body — decodeJSON should reject it
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST with nil body: got %d, want 400. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSavedLists_EdgeCase_DoubleDelete(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	createListItem(t, server.db, userID, "Butter", "Dairy")

	// Create
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]string{"name": "Temp"}, []*http.Cookie{cookie})
	result := parseJSON(rr)
	id := int(result["id"].(float64))

	// First delete
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", id),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("First delete: got %d, want 200", rr.Code)
	}

	// Second delete — should be 404
	rr = doRequest(server.handleSavedLists, "DELETE", fmt.Sprintf("/api/saved-lists/%d", id),
		nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusNotFound {
		t.Errorf("Second delete of same list: got %d, want 404", rr.Code)
	}
}

func TestSavedListMerge_Success(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// Create a saved list with one item
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]interface{}{"name": "My Groceries", "items": []map[string]interface{}{
			{"name": "Milk", "brand": "Organic Valley", "department": "Dairy & Eggs"},
		}}, []*http.Cookie{cookie})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rr.Code)
	}
	id := int(parseJSON(rr)["id"].(float64))

	// Merge in new items (one dup, one new)
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/merge", id),
		map[string]interface{}{"items": []map[string]interface{}{
			{"name": "Milk", "brand": "Organic Valley", "department": "Dairy & Eggs"},
			{"name": "Eggs", "brand": "", "department": "Dairy & Eggs"},
		}}, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("merge: got %d, want 200: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	if result["added"].(float64) != 1 {
		t.Errorf("expected 1 item added (dup skipped), got %v", result["added"])
	}

	// Verify total items
	var count int
	server.db.QueryRow("SELECT COUNT(*) FROM saved_list_items WHERE saved_list_id = ?", id).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 total items, got %d", count)
	}
}

func TestSavedListMerge_RequiresAuth(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists/1/merge",
		map[string]interface{}{"items": []map[string]interface{}{}}, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestSavedListMerge_CrossUserBlocked(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie1, _ := loginUser(t, server, "5551234567")
	cookie2, _ := loginUser(t, server, "5559999999")

	// User 1 creates a list
	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]interface{}{"name": "Private", "items": []map[string]interface{}{
			{"name": "Butter", "department": "Dairy & Eggs"},
		}}, []*http.Cookie{cookie1})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201: %s", rr.Code, rr.Body.String())
	}
	result := parseJSON(rr)
	idVal, ok := result["id"].(float64)
	if !ok {
		t.Fatalf("expected id in response, got %v", result)
	}
	id := int(idVal)

	// User 2 tries to merge into it
	rr = doRequest(server.handleSavedLists, "POST", fmt.Sprintf("/api/saved-lists/%d/merge", id),
		map[string]interface{}{"items": []map[string]interface{}{
			{"name": "Cheese", "department": "Dairy & Eggs"},
		}}, []*http.Cookie{cookie2})
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-user merge, got %d", rr.Code)
	}
}

// =============================================================================
// PER-STORE MASTER LISTS
// =============================================================================

func TestSavedLists_CreateWithStoreName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Rotisserie Chicken", "Deli")
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Bulk Rice", "Grains")

	rr := doRequest(server.handleSavedLists, "POST", "/api/saved-lists",
		map[string]interface{}{"name": "Weekly Costco", "store_name": "Costco"},
		[]*http.Cookie{cookie})

	if rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}
	body := parseJSON(rr)
	if body["store_name"] != "Costco" {
		t.Errorf("store_name: got %v, want Costco", body["store_name"])
	}
	if body["item_count"].(float64) != 2 {
		t.Errorf("item_count: got %v, want 2", body["item_count"])
	}
}

func TestSavedLists_GetReturnsStoreNameAndIsMaster(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Insert a saved list with store_name directly
	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name, is_master) VALUES (?, ?, ?, ?)",
		userID, "Costco Run", "Costco", 1)
	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "WF Weekly", "Whole Foods")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Fatalf("Expected 2 saved lists, got %d", len(lists))
	}

	// Find the Costco list — should have is_master=true
	var costcoList, wfList map[string]interface{}
	for _, l := range lists {
		if l["name"] == "Costco Run" {
			costcoList = l
		}
		if l["name"] == "WF Weekly" {
			wfList = l
		}
	}
	if costcoList == nil {
		t.Fatal("Costco list not found in response")
	}
	if costcoList["store_name"] != "Costco" {
		t.Errorf("store_name: got %v, want Costco", costcoList["store_name"])
	}
	if costcoList["is_master"] != true {
		t.Errorf("is_master: got %v, want true", costcoList["is_master"])
	}
	if wfList == nil {
		t.Fatal("WF list not found in response")
	}
	if wfList["store_name"] != "Whole Foods" {
		t.Errorf("store_name: got %v, want Whole Foods", wfList["store_name"])
	}
}

func TestSavedLists_GetFilterByStore(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "Costco Run", "Costco")
	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "WF Weekly", "Whole Foods")
	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, ?)",
		userID, "General List")

	// Filter by Costco
	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists?store=Costco", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusOK {
		t.Fatalf("Status: got %d, want 200", rr.Code)
	}

	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Fatalf("Expected 1 list for Costco filter, got %d", len(lists))
	}
	if lists[0]["name"] != "Costco Run" {
		t.Errorf("Name: got %v, want Costco Run", lists[0]["name"])
	}
}

func TestSavedLists_GetFilterByStoreCaseInsensitive(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "Costco Run", "Costco")

	// Filter with lowercase
	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists?store=costco", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 1 {
		t.Fatalf("Expected 1 list for case-insensitive store filter, got %d", len(lists))
	}
}

func TestSavedLists_GetNoStoreFilter_ReturnsAll(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "Costco Run", "Costco")
	server.db.Exec("INSERT INTO saved_lists (user_id, name) VALUES (?, ?)",
		userID, "General List")

	rr := doRequest(server.handleSavedLists, "GET", "/api/saved-lists", nil, []*http.Cookie{cookie})
	var lists []map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &lists)
	if len(lists) != 2 {
		t.Fatalf("Expected 2 lists (no filter), got %d", len(lists))
	}
}

func TestParseAndAdd_SaveWithStoreName(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Add items to save
	server.db.Exec("INSERT INTO list_items (user_id, name, department) VALUES (?, ?, ?)",
		userID, "Organic Milk", "Dairy & Eggs")

	// Mock parseCommand to return a save command with store_name
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:    "save",
			SaveName:  "Whole Foods List",
			StoreName: "Whole Foods",
			Message:   "Saved your list for Whole Foods!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "save this as my Whole Foods list"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 200/201. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify the saved list has store_name set
	var storeName string
	err := server.db.QueryRow("SELECT store_name FROM saved_lists WHERE user_id = ? AND name = ?",
		userID, "Whole Foods List").Scan(&storeName)
	if err != nil {
		t.Fatalf("saved list not found: %v", err)
	}
	if storeName != "Whole Foods" {
		t.Errorf("store_name: got %q, want %q", storeName, "Whole Foods")
	}
}

func TestParseAndAdd_LoadSavedListByStore(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a saved list with store_name and items
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "Costco Weekly", "Costco")
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		listID, "Paper Towels", "Household", 1)
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		listID, "Olive Oil", "Pantry", 1)

	// Mock parseCommand to return load with store filter
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:        "load_saved_list",
			SavedListName: "Costco Weekly",
			StoreName:     "Costco",
			Message:       "Loading your Costco list!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "load my costco list"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 200/201. Body: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	addedVal, _ := resp["added"].(float64)
	if int(addedVal) != 2 {
		t.Errorf("added: got %v, want 2", resp["added"])
	}
}

func TestParseAndAdd_LoadSavedListByStoreOnly_SingleList(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, userID := loginUser(t, server, "5551234567")

	// Create a single saved list for Costco
	result, _ := server.db.Exec("INSERT INTO saved_lists (user_id, name, store_name) VALUES (?, ?, ?)",
		userID, "My Costco Stuff", "Costco")
	listID, _ := result.LastInsertId()
	server.db.Exec("INSERT INTO saved_list_items (saved_list_id, name, department, quantity) VALUES (?, ?, ?, ?)",
		listID, "Toilet Paper", "Household", 1)

	// Mock parseCommand: store_name set but no saved_list_name — should auto-load the single list
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:    "load_saved_list",
			StoreName: "Costco",
			Message:   "Loading your Costco list!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "load my costco list"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 200/201. Body: %s", rr.Code, rr.Body.String())
	}

	resp := parseJSON(rr)
	addedVal, _ := resp["added"].(float64)
	if int(addedVal) != 1 {
		t.Errorf("added: got %v, want 1", resp["added"])
	}
}

func TestParseAndAdd_LoadSavedListByStore_NoLists(t *testing.T) {
	server := newTestServer(t, &MockTwilioClient{})
	cookie, _ := loginUser(t, server, "5551234567")

	// No saved lists at all
	server.parseCommand = func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error) {
		return &ParsedCommand{
			Action:    "load_saved_list",
			StoreName: "Costco",
			Message:   "Loading your Costco list!",
		}, nil
	}

	rr := doRequest(server.handleParseAndAdd, "POST", "/api/list/parse",
		map[string]string{"text": "load my costco list"},
		[]*http.Cookie{cookie})
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("Status: got %d, want 200/201", rr.Code)
	}

	resp := parseJSON(rr)
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "Costco") {
		t.Errorf("Expected message to mention Costco, got: %s", msg)
	}
}

// =============================================================================
// TRIPS EDGE CASES
// =============================================================================
