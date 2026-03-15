package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/api/list")
	tail = strings.Trim(tail, "/")

	if tail == "parse" && r.Method == http.MethodPost {
		s.handleParseAndAdd(w, r)
		return
	}

	if tail == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListGet(w, r)
		case http.MethodPost:
			s.handleListAdd(w, r)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
		return
	}

	parts := strings.Split(tail, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeBadRequest(w, "Invalid ID")
		return
	}

	if len(parts) == 2 && parts[1] == "toggle" && r.Method == http.MethodPatch {
		s.handleListToggle(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "urgent" && r.Method == http.MethodPatch {
		s.handleListUrgent(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "claim" && r.Method == http.MethodPatch {
		s.handleListClaim(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
		s.handleItemHistory(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.handleListEdit(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleListDelete(w, r, id)
		return
	}

	writeNotFound(w, "Not found")
}

// handleParseAndAdd handles POST /api/list/parse
// Request:  { "text": "milk eggs sourdough chicken", "image": "data:image/jpeg;base64,..." }
// Response: ParseResponse with items, added count, and updated count
func (s *Server) handleParseAndAdd(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10 MB limit

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Per-user rate limit: 30 parse requests per hour
	if checkRateLimitBucket(&s.parseLimiter, session.UserID, time.Hour, 30) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests — please wait a few minutes"})
		return
	}

	var req ParseAndAddRequest
	// Decode directly — don't use decodeJSON which would override our 10MB limit with 1MB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid request body"
		if err.Error() == "http: request body too large" {
			msg = "Request too large"
		}
		writeBadRequest(w, msg)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	req.Image = strings.TrimSpace(req.Image)
	// Merge: support both single image and images array
	images := req.Images
	if len(images) == 0 && req.Image != "" {
		images = []string{req.Image}
	}
	// Cap images to prevent memory exhaustion
	if len(images) > 5 {
		images = images[:5]
	}

	// Branch 1: Swap accept/decline — short-circuit before Claude call
	if req.SwapDealID > 0 {
		s.handleSwapAction(w, req, session)
		return
	}

	// Branch 2: Receipt photo — cross-reference receipt against list
	if req.IsReceipt && len(images) > 0 {
		s.handleReceiptScan(w, session, images)
		return
	}

	if req.Text == "" && len(images) == 0 {
		log.Printf("[parse] empty text and images")
		writeBadRequest(w, "Text or image is required")
		return
	}
	if s.config.AnthropicAPIKey == "" {
		log.Printf("[parse] no API key configured")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI parsing not configured"})
		return
	}

	log.Printf("Parsing grocery text for user %d: len=%d (images: %d)", session.UserID, len(req.Text), len(images))

	// Cache household IDs once for this request to avoid repeated queries
	hhIDs, _ := getHouseholdUserIDs(s.db, session.UserID)

	// Branch 3: Text-only — command parser with natural language operations
	if len(images) == 0 && req.Text != "" {
		s.handleTextCommand(w, r, session, req.Text, hhIDs)
		return
	}

	// Branch 4: Image-based item parsing
	s.handleImageParse(w, session, req.Text, images)
}

// handleListGet handles GET /api/list
func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	rows, err := s.db.Query(
		"SELECT "+listItemColumns+" FROM "+listItemJoins+" WHERE li."+whereClause+" ORDER BY li.name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't load your list. Please try again.")
		return
	}
	defer rows.Close()

	items := []ListItem{}
	for rows.Next() {
		item, err := scanListItem(rows)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[list] row iteration error: %v", err)
	}

	writeJSON(w, http.StatusOK, items)
}

// handleListSwaps returns pending (un-actioned) swap offers for the user's list items.
// A swap is pending if it was offered but not yet accepted or declined.
func (s *Server) handleListSwaps(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT se.item_id, se.deal_id,
		       bd.offer_brand, bd.offer_product, bd.discount_cents,
		       bd.offer_price_cents, bd.competitor_price_cents,
		       bd.offer_size, bd.competitor_size,
		       bd.offer_unit_price, bd.competitor_unit_price,
		       bd.why_bullets, bd.allergen_notes,
		       bd.deal_type, bd.quantity_offered, bd.trial_product_name, bd.trial_size, bd.campaign_tag,
		       COALESCE(bd.ends_at,'')
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE se.user_id = ? AND se.action = 'offered'
		  AND bd.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM swap_events se2
		    WHERE se2.user_id = se.user_id AND se2.deal_id = se.deal_id AND se2.item_id = se.item_id
		      AND se2.action IN ('accepted', 'declined')
		      AND se2.id > se.id
		  )
		  AND se.item_id IN (SELECT id FROM list_items WHERE checked = 0)
		ORDER BY se.created_at DESC
	`, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load swaps"})
		return
	}
	defer rows.Close()

	swaps := []PendingSwap{}
	for rows.Next() {
		var ps PendingSwap
		var whyJSON, allergen string
		if err := rows.Scan(&ps.ItemID, &ps.DealID,
			&ps.OfferBrand, &ps.OfferProduct, &ps.DiscountCents,
			&ps.OfferPriceCents, &ps.OriginalPriceCents,
			&ps.OfferSize, &ps.OriginalSize,
			&ps.OfferUnitPrice, &ps.OriginalUnitPrice,
			&whyJSON, &allergen,
			&ps.DealType, &ps.QuantityOffered, &ps.TrialProductName, &ps.TrialSize, &ps.CampaignTag,
			&ps.EndsAt,
		); err != nil {
			log.Printf("[swaps] scan error: %v", err)
			continue
		}
		if whyJSON != "" && whyJSON != "[]" {
			json.Unmarshal([]byte(whyJSON), &ps.WhyBullets)
		}
		ps.AllergenWarning = allergen
		if ps.OriginalPriceCents > 0 && ps.OfferPriceCents > 0 {
			ps.SavingsPercent = int(float64(ps.OriginalPriceCents-ps.OfferPriceCents) / float64(ps.OriginalPriceCents) * 100)
		}
		swaps = append(swaps, ps)
	}

	writeJSON(w, http.StatusOK, swaps)
}

// handleListAdd handles POST /api/list
// Request: { "name": "Milk", "department": "Dairy & Eggs" }
func (s *Server) handleListAdd(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req ListAddRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeBadRequest(w, "Name is required")
		return
	}
	if len(req.Name) > 200 {
		writeBadRequest(w, "Item name is too long")
		return
	}
	if req.Department == "" {
		req.Department = "Other"
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}

	emoji := inferItemEmoji(req.Name, req.Department)
	item, _, err := upsertListItem(s.db, session.UserID, req.Name, "", req.Department, "", req.Quantity, "app", "", emoji)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	writeJSON(w, http.StatusCreated, item)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleListToggle handles PATCH /api/list/{id}/toggle
func (s *Server) handleListToggle(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	toggleArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET checked = NOT checked WHERE id = ? AND "+whereClause,
		toggleArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}

	action := "checked"
	if !item.Checked {
		action = "unchecked"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	// F3: Record department order event when item is checked (fire-and-forget)
	if item.Checked && item.Department != "" {
		go func(db *sql.DB, userID int, department string) {
			// Determine store from active shopping trip (if any)
			var storeName string
			var storeID sql.NullInt64
			db.QueryRow(
				"SELECT COALESCE(store_id, 0) FROM shopping_trips WHERE user_id = ? AND trip_date >= date('now', '-1 day') ORDER BY created_at DESC LIMIT 1",
				userID,
			).Scan(&storeID)
			if storeID.Valid && storeID.Int64 > 0 {
				db.QueryRow("SELECT COALESCE(name, '') FROM stores WHERE id = ?", storeID.Int64).Scan(&storeName)
			}

			tripDate := time.Now().Format("2006-01-02")
			_, err := db.Exec(
				"INSERT INTO department_order_events (user_id, store_name, department, checked_at, trip_date) VALUES (?, ?, ?, datetime('now'), ?)",
				userID, storeName, department, tripDate,
			)
			if err != nil {
				log.Printf("[dept-order] failed to record event for user %d: %v", userID, err)
			}
		}(s.db, session.UserID, item.Department)
	}

	writeJSON(w, http.StatusOK, item)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleListUrgent handles PATCH /api/list/{id}/urgent
func (s *Server) handleListUrgent(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	urgentArgs := append([]interface{}{id}, whereArgs...)
	_, err := s.db.Exec(
		"UPDATE list_items SET urgent = NOT urgent WHERE id = ? AND "+whereClause,
		urgentArgs...,
	)
	if err != nil {
		dbError(w, err, "We couldn't save your item right now. Please try again.")
		return
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}

	action := "marked urgent"
	if !item.Urgent {
		action = "unmarked urgent"
	}
	recordModification(s.db, id, session.UserID, action, "", "app")

	writeJSON(w, http.StatusOK, item)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleListClaim handles PATCH /api/list/{id}/claim
func (s *Server) handleListClaim(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Check item exists in household
	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}

	if item.ClaimedByID != nil && *item.ClaimedByID == session.UserID {
		// Unclaim
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = NULL, claimed_at = NULL WHERE id = ?", id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "unclaimed", "", "app")
	} else {
		// Claim (or override)
		_, err = s.db.Exec("UPDATE list_items SET claimed_by = ?, claimed_at = CURRENT_TIMESTAMP WHERE id = ?", session.UserID, id)
		if err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "claimed", "", "app")
	}

	updated, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleItemHistory handles GET /api/list/{id}/history
func (s *Server) handleItemHistory(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Verify item belongs to household
	_, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}

	rows, err := s.db.Query(
		"SELECT m.action, m.detail, m.source, m.created_at, COALESCE(u.name, u.phone_number, '') FROM item_modifications m LEFT JOIN users u ON u.id = m.user_id WHERE m.item_id = ? ORDER BY m.created_at DESC",
		id,
	)
	if err != nil {
		dbError(w, err, "We couldn't load item history. Please try again.")
		return
	}
	defer rows.Close()

	type modification struct {
		Action    string `json:"action"`
		Detail    string `json:"detail"`
		Source    string `json:"source"`
		CreatedAt string `json:"created_at"`
		UserName  string `json:"user_name"`
	}
	var mods []modification
	for rows.Next() {
		var m modification
		var detail, source, userName sql.NullString
		if err := rows.Scan(&m.Action, &detail, &source, &m.CreatedAt, &userName); err != nil {
			continue
		}
		m.Detail = detail.String
		m.Source = source.String
		m.UserName = userName.String
		mods = append(mods, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[history] row iteration error: %v", err)
	}
	if mods == nil {
		mods = []modification{}
	}

	writeJSON(w, http.StatusOK, mods)
}

// handleListEdit handles PATCH /api/list/{id}
// Request: { "name": "...", "brand": "...", "department": "...", "quantity": 1, "size": "..." } (all optional)
func (s *Server) handleListEdit(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req ListEditRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)

	// Build a single UPDATE with only the fields that were provided
	var setClauses []string
	var setArgs []interface{}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeBadRequest(w, "Name cannot be empty")
			return
		}
		setClauses = append(setClauses, "name = ?")
		setArgs = append(setArgs, name)
	}
	if req.Brand != nil {
		setClauses = append(setClauses, "brand = ?")
		setArgs = append(setArgs, *req.Brand)
	}
	if req.Department != nil {
		setClauses = append(setClauses, "department = ?")
		setArgs = append(setArgs, *req.Department)
	}
	if req.Quantity != nil {
		qty := *req.Quantity
		if qty < 1 {
			qty = 1
		}
		setClauses = append(setClauses, "quantity = ?")
		setArgs = append(setArgs, qty)
	}
	if req.Size != nil {
		setClauses = append(setClauses, "size = ?")
		setArgs = append(setArgs, *req.Size)
	}
	if req.Notes != nil {
		setClauses = append(setClauses, "notes = ?")
		setArgs = append(setArgs, *req.Notes)
	}

	if len(setClauses) > 0 {
		query := "UPDATE list_items SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND " + whereClause
		args := append(setArgs, id)
		args = append(args, whereArgs...)
		if _, err := s.db.Exec(query, args...); err != nil {
			dbError(w, err, "We couldn't save your item right now. Please try again.")
			return
		}
		recordModification(s.db, id, session.UserID, "updated", strings.Join(setClauses, ", "), "app")
	}

	item, err := getItemByID(s.db, id, session.UserID)
	if err != nil {
		writeNotFound(w, "Item not found")
		return
	}

	writeJSON(w, http.StatusOK, item)
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleListDelete handles DELETE /api/list/{id}
func (s *Server) handleListDelete(w http.ResponseWriter, r *http.Request, id int) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	// ON DELETE CASCADE handles item_modifications cleanup
	args := append([]interface{}{id}, whereArgs...)
	result, err := s.db.Exec(
		"DELETE FROM list_items WHERE id = ? AND "+whereClause,
		args...,
	)
	if err != nil {
		dbError(w, err, "We couldn't remove that item. Please try again.")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeNotFound(w, "Item not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// =============================================================================
// DEPARTMENT ORDER
// =============================================================================

// getLearnedDepartmentOrder computes the learned department order for a user
// at a specific store based on their check-off history. Returns the sorted
// department list and true if enough data exists (>= 3 distinct trip dates),
// otherwise returns nil and false.
func getLearnedDepartmentOrder(db *sql.DB, userID int, storeName string) ([]string, int, bool) {
	// Count distinct trip dates for this user+store
	var tripCount int
	err := db.QueryRow(
		"SELECT COUNT(DISTINCT trip_date) FROM department_order_events WHERE user_id = ? AND store_name = ? AND trip_date != ''",
		userID, storeName,
	).Scan(&tripCount)
	if err != nil || tripCount < 3 {
		return nil, tripCount, false
	}

	// For each department, compute average position within each trip.
	// Within a trip (same trip_date), items are ordered by checked_at.
	// We use a window function to assign position within each trip, then average across trips.
	rows, err := db.Query(`
		WITH ranked AS (
			SELECT department, trip_date,
				ROW_NUMBER() OVER (PARTITION BY trip_date ORDER BY checked_at) as pos
			FROM department_order_events
			WHERE user_id = ? AND store_name = ? AND trip_date != ''
		),
		first_seen AS (
			SELECT department, trip_date, MIN(pos) as first_pos
			FROM ranked
			GROUP BY department, trip_date
		)
		SELECT department, AVG(first_pos) as avg_pos
		FROM first_seen
		GROUP BY department
		ORDER BY avg_pos
	`, userID, storeName)
	if err != nil {
		log.Printf("[dept-order] query error for user %d store %q: %v", userID, storeName, err)
		return nil, tripCount, false
	}
	defer rows.Close()

	var departments []string
	for rows.Next() {
		var dept string
		var avgPos float64
		if err := rows.Scan(&dept, &avgPos); err != nil {
			log.Printf("[dept-order] scan error: %v", err)
			continue
		}
		departments = append(departments, dept)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[dept-order] row iteration error: %v", err)
	}

	if len(departments) == 0 {
		return nil, tripCount, false
	}

	return departments, tripCount, true
}

// handleDepartmentOrder handles GET /api/list/department-order?store=StoreName
func (s *Server) handleDepartmentOrder(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	storeName := r.URL.Query().Get("store")

	// Check if store_order_enabled is on for this user
	var storeOrderEnabled int
	if err := s.db.QueryRow(
		"SELECT COALESCE(store_order_enabled, 1) FROM users WHERE id = ?",
		session.UserID,
	).Scan(&storeOrderEnabled); err != nil {
		log.Printf("[dept-order] failed to read store_order_enabled for user %d: %v", session.UserID, err)
		storeOrderEnabled = 1
	}

	// If disabled, return alphabetical order of departments from user's list
	if storeOrderEnabled == 0 {
		departments := s.getAlphabeticalDepartments(session.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"departments": departments,
			"learned":     false,
			"store":       storeName,
			"trips":       0,
		})
		return
	}

	departments, tripCount, learned := getLearnedDepartmentOrder(s.db, session.UserID, storeName)
	if !learned {
		departments = s.getAlphabeticalDepartments(session.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"departments": departments,
			"learned":     false,
			"store":       storeName,
			"trips":       tripCount,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"departments": departments,
		"learned":     true,
		"store":       storeName,
		"trips":       tripCount,
	})
}

// getAlphabeticalDepartments returns distinct departments from the user's active
// list items, sorted alphabetically. Used as fallback when learned order is unavailable.
func (s *Server) getAlphabeticalDepartments(userID int) []string {
	whereClause, whereArgs := householdItemsWhereClause(s.db, userID)
	rows, err := s.db.Query(
		"SELECT DISTINCT department FROM list_items WHERE "+whereClause+" AND checked = 0 ORDER BY department",
		whereArgs...,
	)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var departments []string
	for rows.Next() {
		var dept string
		if rows.Scan(&dept) == nil && dept != "" {
			departments = append(departments, dept)
		}
	}
	if departments == nil {
		departments = []string{}
	}
	return departments
}

// handleListClear handles POST /api/list/clear
func (s *Server) handleListClear(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)

	// Stash checked items before deleting so Penny can restore them
	rows, err := s.db.Query("SELECT name, brand, department, quantity, size, notes FROM list_items WHERE checked = 1 AND "+whereClause, whereArgs...)
	if err == nil {
		type stashedItem struct {
			Name       string `json:"name"`
			Brand      string `json:"brand"`
			Department string `json:"department"`
			Quantity   int    `json:"quantity"`
			Size       string `json:"size"`
			Notes      string `json:"notes"`
		}
		var stashed []stashedItem
		for rows.Next() {
			var si stashedItem
			if rows.Scan(&si.Name, &si.Brand, &si.Department, &si.Quantity, &si.Size, &si.Notes) == nil {
				stashed = append(stashed, si)
			}
		}
		rows.Close()
		if stashJSON, e := json.Marshal(stashed); e == nil {
			s.db.Exec("UPDATE users SET last_cleared_items = ? WHERE id = ?", string(stashJSON), session.UserID)
		}
	}

	// If ?all=true, clear entire list (used by Penny chat "clear list" command)
	clearAll := r.URL.Query().Get("all") == "true"
	if clearAll {
		_, err = s.db.Exec("DELETE FROM list_items WHERE "+whereClause, whereArgs...)
	} else {
		_, err = s.db.Exec("DELETE FROM list_items WHERE checked = 1 AND "+whereClause, whereArgs...)
	}
	if err != nil {
		dbError(w, err, "We couldn't clear your list. Please try again.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// handleListRestore handles POST /api/list/restore — bulk re-insert items (used by undo)
func (s *Server) handleListRestore(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var items []struct {
		Name       string `json:"name"`
		Brand      string `json:"brand"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
		Size       string `json:"size"`
		Notes      string `json:"notes"`
		Checked    bool   `json:"checked"`
		Urgent     bool   `json:"urgent"`
	}
	if !decodeJSON(w, r, &items) {
		return
	}

	// Cap restore size to prevent abuse
	if len(items) > 500 {
		writeBadRequest(w, "Too many items to restore.")
		return
	}

	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		qty := item.Quantity
		if qty < 1 {
			qty = 1
		}
		dept := item.Department
		if dept == "" {
			dept = "Other"
		}
		checked := 0
		if item.Checked {
			checked = 1
		}
		urgent := 0
		if item.Urgent {
			urgent = 1
		}
		_, err := s.db.Exec(
			`INSERT INTO list_items (user_id, name, brand, department, quantity, size, notes, checked, urgent, source)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'app')`,
			session.UserID, strings.TrimSpace(item.Name), strings.TrimSpace(item.Brand),
			dept, qty, strings.TrimSpace(item.Size), strings.TrimSpace(item.Notes), checked, urgent,
		)
		if err != nil {
			dbError(w, err, "We couldn't restore your list. Please try again.")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	go s.broadcastListUpdate(getUserHouseholdID(s.db, session.UserID), session.UserID)
}

// =============================================================================
// SSE — Real-time list synchronization
// =============================================================================

// broadcastListUpdate sends an SSE event to all connected clients in the given
// household (or to all clients of a specific user if householdID is 0).
// This is fire-and-forget — slow or disconnected clients are skipped.
func (s *Server) broadcastListUpdate(householdID int64, userID int) {
	msg := []byte("event: list-update\ndata: {}\n\n")
	s.sseClients.Range(func(key, _ any) bool {
		client := key.(*sseClient)
		// Match by household if the user is in one, otherwise match by user ID
		if householdID > 0 && client.householdID == householdID {
			select {
			case client.ch <- msg:
			default:
				// Channel full — skip this client (it will catch up on next event)
			}
		} else if householdID == 0 && client.userID == userID {
			select {
			case client.ch <- msg:
			default:
			}
		}
		return true
	})
}

// broadcastCommandResult sends a command-result SSE event to a specific user.
// Used for async command processing — the result of a slow Claude parse is pushed
// to the client after the HTTP response has already been sent.
func (s *Server) broadcastCommandResult(userID int, result interface{}) {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[sse] failed to marshal command result: %v", err)
		return
	}
	msg := []byte("event: command-result\ndata: " + string(data) + "\n\n")
	s.sseClients.Range(func(key, _ any) bool {
		client := key.(*sseClient)
		if client.userID == userID {
			select {
			case client.ch <- msg:
			default:
			}
		}
		return true
	})
}

// handleListStream handles GET /api/list/stream — SSE endpoint for real-time list updates.
func (s *Server) handleListStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Check that the ResponseWriter supports flushing (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Streaming not supported"})
		return
	}

	householdID := getUserHouseholdID(s.db, session.UserID)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if behind proxy
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Register this client
	client := &sseClient{
		householdID: householdID,
		userID:      session.UserID,
		ch:          make(chan []byte, 8),
	}
	s.sseClients.Store(client, struct{}{})
	defer s.sseClients.Delete(client)

	log.Printf("[sse] client connected: user=%d household=%d", session.UserID, householdID)

	// Use request context to detect client disconnection
	ctx := r.Context()

	// Heartbeat ticker — keeps the connection alive (30s)
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[sse] client disconnected: user=%d", session.UserID)
			return
		case msg := <-client.ch:
			_, err := w.Write(msg)
			if err != nil {
				log.Printf("[sse] write error for user=%d: %v", session.UserID, err)
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			_, err := w.Write([]byte(": heartbeat\n\n"))
			if err != nil {
				log.Printf("[sse] heartbeat error for user=%d: %v", session.UserID, err)
				return
			}
			flusher.Flush()
		}
	}
}

// =============================================================================

func (s *Server) handleBrandResolve(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req BrandResolveRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.ItemID == 0 || req.Brand == "" {
		writeBadRequest(w, "item_id and brand are required")
		return
	}

	// Update the item's brand — scoped to household ownership
	whereClause, whereArgs := householdItemsWhereClause(s.db, session.UserID)
	updateArgs := append([]interface{}{req.Brand, req.ItemID}, whereArgs...)
	result, err := s.db.Exec("UPDATE list_items SET brand = ? WHERE id = ? AND "+whereClause, updateArgs...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeNotFound(w, "Item not found")
		return
	}

	// Record the brand choice for preference learning
	category := req.ProductCategory
	if category == "" {
		var itemName string
		s.db.QueryRow("SELECT name FROM list_items WHERE id = ?", req.ItemID).Scan(&itemName)
		category = itemName
	}
	recordBrandChoice(s.db, session.UserID, category, req.Brand, "disambiguation")
	recordModification(s.db, req.ItemID, session.UserID, "updated", "brand resolved to "+req.Brand, "app")

	// Return the updated item
	item, err := getItemByID(s.db, req.ItemID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load item"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleShareList handles POST /api/list/share — creates a shared link for the user's list.
func (s *Server) handleShareList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Generate 16-byte hex code
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate share code"})
		return
	}
	code := hex.EncodeToString(codeBytes)

	// Check if user has a household
	shareHID := getUserHouseholdID(s.db, session.UserID)

	// Generate an invite code if user has a household
	var inviteCode string
	if shareHID != 0 {
		inviteBytes := make([]byte, 16)
		if _, err := rand.Read(inviteBytes); err == nil {
			inviteCode = hex.EncodeToString(inviteBytes)
			expiresAt := time.Now().Add(inviteExpiry)
			s.db.Exec(
				"INSERT INTO invites (household_id, code, created_by, expires_at) VALUES (?, ?, ?, ?)",
				shareHID, inviteCode, session.UserID, expiresAt,
			)
		}
	}

	// Insert shared link with 7-day expiry
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err := s.db.Exec(
		"INSERT INTO shared_links (code, user_id, household_id, invite_code, expires_at) VALUES (?, ?, ?, ?, ?)",
		code, session.UserID, shareHID, inviteCode, expiresAt,
	)
	if err != nil {
		dbError(w, err, "Failed to create share link")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"url":        "/shared/" + code,
		"code":       code,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// handleSharedView handles GET /api/shared/{code} — returns read-only list data (no auth required).
func (s *Server) handleSharedView(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Extract code from URL path
	code := strings.TrimPrefix(r.URL.Path, "/api/shared/")
	if code == "" {
		writeBadRequest(w, "Missing share code")
		return
	}

	// Look up the shared link
	var userID int
	var householdID sql.NullInt64
	var inviteCode sql.NullString
	var expiresAt string
	var viewCount int
	err := s.db.QueryRow(
		"SELECT user_id, household_id, invite_code, expires_at, view_count FROM shared_links WHERE code = ?",
		code,
	).Scan(&userID, &householdID, &inviteCode, &expiresAt, &viewCount)
	if err != nil {
		writeNotFound(w, "Share link not found")
		return
	}

	// Check expiry
	expiry, err := time.Parse("2006-01-02 15:04:05", expiresAt)
	if err != nil {
		// Try RFC3339 format
		expiry, err = time.Parse(time.RFC3339, expiresAt)
	}
	if err != nil || time.Now().After(expiry) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "This share link has expired"})
		return
	}

	// Increment view count
	s.db.Exec("UPDATE shared_links SET view_count = view_count + 1 WHERE code = ?", code)

	// Get sharer's name
	var sharerName string
	s.db.QueryRow("SELECT COALESCE(name, '') FROM users WHERE id = ?", userID).Scan(&sharerName)

	// Get list items (name, brand, department, quantity, checked — no IDs or sensitive data)
	whereClause, whereArgs := householdItemsWhereClause(s.db, userID)
	rows, err := s.db.Query(
		"SELECT name, COALESCE(brand, ''), COALESCE(department, ''), COALESCE(quantity, 1), checked FROM list_items WHERE "+whereClause+" ORDER BY name",
		whereArgs...,
	)
	if err != nil {
		dbError(w, err, "Failed to load shared list")
		return
	}
	defer rows.Close()

	type sharedItem struct {
		Name       string `json:"name"`
		Brand      string `json:"brand,omitempty"`
		Department string `json:"department,omitempty"`
		Quantity   int    `json:"quantity"`
		Checked    bool   `json:"checked"`
	}
	items := []sharedItem{}
	for rows.Next() {
		var item sharedItem
		if rows.Scan(&item.Name, &item.Brand, &item.Department, &item.Quantity, &item.Checked) == nil {
			items = append(items, item)
		}
	}

	resp := map[string]interface{}{
		"items":       items,
		"sharer_name": sharerName,
		"expires_at":  expiry.Format(time.RFC3339),
	}
	if inviteCode.Valid && inviteCode.String != "" {
		resp["invite_code"] = inviteCode.String
	}

	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
