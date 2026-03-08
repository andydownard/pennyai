package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// =============================================================================
// SAVED LISTS HANDLERS
// =============================================================================

// handleSavedLists routes requests under /api/saved-lists:
//
//	GET    /api/saved-lists          → list all saved lists
//	POST   /api/saved-lists          → save current unchecked items as a new saved list
//	PUT    /api/saved-lists/{id}     → update saved list items from current unchecked items
//	DELETE /api/saved-lists/{id}     → delete a saved list
//	POST   /api/saved-lists/{id}/load → load saved list items into active list
func (s *Server) handleSavedLists(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/saved-lists")
	tail = strings.Trim(tail, "/")

	if tail == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleSavedListsGet(w, r)
		case http.MethodPost:
			s.handleSavedListsCreate(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
		return
	}

	parts := strings.Split(tail, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		return
	}

	if len(parts) == 2 && parts[1] == "load" && r.Method == http.MethodPost {
		s.handleSavedListLoad(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPut {
		s.handleSavedListUpdate(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleSavedListDelete(w, r, id)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
}

// handleSavedListsGet handles GET /api/saved-lists
func (s *Server) handleSavedListsGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	rows, err := s.db.Query(
		"SELECT sl.id, sl.name, COUNT(sli.id), sl.created_at FROM saved_lists sl LEFT JOIN saved_list_items sli ON sli.saved_list_id = sl.id WHERE sl."+whereClause+" GROUP BY sl.id ORDER BY sl.created_at DESC",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your saved lists. Please try again.")
		return
	}
	defer rows.Close()

	type SavedListSummary struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		ItemCount int    `json:"item_count"`
		CreatedAt string `json:"created_at"`
	}

	lists := []SavedListSummary{}
	for rows.Next() {
		var sl SavedListSummary
		if err := rows.Scan(&sl.ID, &sl.Name, &sl.ItemCount, &sl.CreatedAt); err != nil {
			log.Printf("Error scanning saved list row: %v", err)
			continue
		}
		lists = append(lists, sl)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[saved-lists] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, lists)
}

// handleSavedListsCreate handles POST /api/saved-lists
// Saves the current unchecked list items as a new saved list.
func (s *Server) handleSavedListsCreate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	itemsToSave, err := collectListItems(s.db, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to read list")
		return
	}

	if len(itemsToSave) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No items to save"})
		return
	}

	// Create saved list in a transaction so items are all-or-nothing
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "Failed to save list")
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO saved_lists (user_id, name) VALUES (?, ?)",
		session.UserID, req.Name,
	)
	if err != nil {
		dbError(w, err, "Failed to save list")
		return
	}
	savedListID, _ := result.LastInsertId()

	if err := insertSavedListItems(tx, savedListID, itemsToSave); err != nil {
		dbError(w, err, "Failed to save list items")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "Failed to save list")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         savedListID,
		"name":       req.Name,
		"item_count": len(itemsToSave),
	})
}

// handleSavedListUpdate handles PUT /api/saved-lists/{id}
// Replaces saved list items with current unchecked list items.
func (s *Server) handleSavedListUpdate(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	if !s.requireSavedListAccess(w, session.UserID, id) {
		return
	}

	itemsToSave, err := collectListItems(s.db, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to read list")
		return
	}

	if len(itemsToSave) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No items to save"})
		return
	}

	// Delete old items and insert new ones in a transaction
	tx, err := s.db.Begin()
	if err != nil {
		dbError(w, err, "Failed to update saved list")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM saved_list_items WHERE saved_list_id = ?", id); err != nil {
		dbError(w, err, "Failed to update saved list")
		return
	}
	if err := insertSavedListItems(tx, int64(id), itemsToSave); err != nil {
		dbError(w, err, "Failed to update saved list items")
		return
	}

	if err := tx.Commit(); err != nil {
		dbError(w, err, "Failed to update saved list")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"item_count": len(itemsToSave),
	})
}

// handleSavedListDelete handles DELETE /api/saved-lists/{id}
func (s *Server) handleSavedListDelete(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Only delete if owned by someone in the household
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	deleteArgs := append([]interface{}{id}, whereArgs...)
	result, err := s.db.Exec("DELETE FROM saved_lists WHERE id = ? AND "+whereClause, deleteArgs...)
	if err != nil {
		dbError(w, err, "We couldn't delete that list. Please try again.")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved list not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleSavedListLoad handles POST /api/saved-lists/{id}/load
// Upserts saved list items into the active list.
func (s *Server) handleSavedListLoad(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	if !s.requireSavedListAccess(w, session.UserID, id) {
		return
	}

	// If mode=overwrite, clear household's active list first
	if r.URL.Query().Get("mode") == "overwrite" {
		whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
		if _, err := s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...); err != nil {
			log.Printf("Error clearing list for overwrite: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to clear list"})
			return
		}
	}

	// Get saved list items — collect all before upserting to avoid holding
	// the rows cursor open while doing writes (deadlocks with single-conn pools).
	rows, err := s.db.Query(
		"SELECT name, brand, department, quantity, size, notes FROM saved_list_items WHERE saved_list_id = ?",
		id,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load saved list"})
		return
	}

	var savedItems []itemData
	for rows.Next() {
		var si itemData
		if err := rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes); err != nil {
			continue
		}
		savedItems = append(savedItems, si)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[saved-lists] load row iteration error: %v", err)
	}
	rows.Close()

	added := 0
	for _, si := range savedItems {
		_, _, err := upsertListItem(s.db, session.UserID, si.Name, si.Brand, si.Department, si.Size, si.Quantity, "app", "")
		if err != nil {
			log.Printf("Error upserting saved list item %q: %v", si.Name, err)
			continue
		}
		added++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"added":   added,
	})
}

// =============================================================================
