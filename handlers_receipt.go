package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Receipt rate limiters: 3/day and 10/week per user
var (
	receiptDayLimiter  sync.Map
	receiptWeekLimiter sync.Map
)

// =============================================================================
// GET /api/receipts — list user's receipts
// =============================================================================

func (s *Server) handleReceipts(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getReceipts(w, session)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (s *Server) getReceipts(w http.ResponseWriter, session Session) {
	rows, err := s.db.Query(`
		SELECT id, image_hash, items_found, items_matched, store_name, transaction_date,
			   total_cents, status, swaps_verified, created_at
		FROM receipts WHERE user_id = ? ORDER BY created_at DESC LIMIT 50`,
		session.UserID)
	if err != nil {
		dbError(w, err, "Failed to load receipts")
		return
	}
	defer rows.Close()

	type receiptSummary struct {
		ID              int    `json:"id"`
		ImageHash       string `json:"image_hash"`
		ItemsFound      int    `json:"items_found"`
		ItemsMatched    int    `json:"items_matched"`
		StoreName       string `json:"store_name"`
		TransactionDate string `json:"transaction_date"`
		TotalCents      int    `json:"total_cents"`
		Status          string `json:"status"`
		SwapsVerified   int    `json:"swaps_verified"`
		CreatedAt       string `json:"created_at"`
	}
	receipts := []receiptSummary{}
	for rows.Next() {
		var r receiptSummary
		rows.Scan(&r.ID, &r.ImageHash, &r.ItemsFound, &r.ItemsMatched,
			&r.StoreName, &r.TransactionDate, &r.TotalCents, &r.Status,
			&r.SwapsVerified, &r.CreatedAt)
		receipts = append(receipts, r)
	}
	writeJSON(w, http.StatusOK, receipts)
}

// =============================================================================
// GET /api/receipts/{id}/items — line items for a receipt
// =============================================================================

func (s *Server) handleReceiptItems(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/receipts/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "items" {
		writeBadRequest(w, "Invalid path")
		return
	}
	receiptID, err := strconv.Atoi(parts[0])
	if err != nil {
		writeBadRequest(w, "Invalid receipt ID")
		return
	}

	// Verify ownership
	var ownerID int
	if err := s.db.QueryRow("SELECT user_id FROM receipts WHERE id = ?", receiptID).Scan(&ownerID); err != nil || ownerID != session.UserID {
		writeNotFound(w, "Receipt not found")
		return
	}

	rows, err := s.db.Query(`
		SELECT id, raw_text, price_cents, quantity, COALESCE(matched_item_id, 0),
			   matched_name, confidence
		FROM receipt_line_items WHERE receipt_id = ? ORDER BY id`,
		receiptID)
	if err != nil {
		dbError(w, err, "Failed to load items")
		return
	}
	defer rows.Close()

	type lineItem struct {
		ID            int    `json:"id"`
		RawText       string `json:"raw_text"`
		PriceCents    int    `json:"price_cents"`
		Quantity      int    `json:"quantity"`
		MatchedItemID int    `json:"matched_item_id,omitempty"`
		MatchedName   string `json:"matched_name,omitempty"`
		Confidence    string `json:"confidence"`
	}
	items := []lineItem{}
	for rows.Next() {
		var li lineItem
		rows.Scan(&li.ID, &li.RawText, &li.PriceCents, &li.Quantity,
			&li.MatchedItemID, &li.MatchedName, &li.Confidence)
		items = append(items, li)
	}
	writeJSON(w, http.StatusOK, items)
}

// =============================================================================
// POST /api/receipts/{id}/confirm — user confirms low-confidence matches
// =============================================================================

func (s *Server) handleReceiptConfirm(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/receipts/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "confirm" {
		writeBadRequest(w, "Invalid path")
		return
	}
	receiptID, err := strconv.Atoi(parts[0])
	if err != nil {
		writeBadRequest(w, "Invalid receipt ID")
		return
	}

	// Verify ownership
	var ownerID int
	if err := s.db.QueryRow("SELECT user_id FROM receipts WHERE id = ?", receiptID).Scan(&ownerID); err != nil || ownerID != session.UserID {
		writeNotFound(w, "Receipt not found")
		return
	}

	var req struct {
		Confirmations []struct {
			LineItemID  int    `json:"line_item_id"`
			Confirmed   bool   `json:"confirmed"`
			MatchedName string `json:"matched_name,omitempty"`
		} `json:"confirmations"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	confirmed := 0
	for _, c := range req.Confirmations {
		if c.Confirmed {
			// Update confidence to high
			s.db.Exec("UPDATE receipt_line_items SET confidence = 'high', matched_name = ? WHERE id = ? AND receipt_id = ?",
				c.MatchedName, c.LineItemID, receiptID)
			confirmed++

			// Feed the abbreviation database (the flywheel)
			var rawText string
			s.db.QueryRow("SELECT raw_text FROM receipt_line_items WHERE id = ?", c.LineItemID).Scan(&rawText)
			if rawText != "" && c.MatchedName != "" {
				var storeName string
				s.db.QueryRow("SELECT store_name FROM receipts WHERE id = ?", receiptID).Scan(&storeName)
				s.db.Exec(`INSERT INTO retailer_abbreviations (abbreviation, full_name, retailer, confirmations)
					VALUES (?, ?, ?, 1)
					ON CONFLICT(abbreviation, retailer) DO UPDATE SET confirmations = confirmations + 1`,
					strings.ToUpper(rawText), c.MatchedName, storeName)
			}
		} else {
			// Clear the match
			s.db.Exec("UPDATE receipt_line_items SET matched_item_id = NULL, matched_name = '', confidence = 'rejected' WHERE id = ? AND receipt_id = ?",
				c.LineItemID, receiptID)
		}
	}

	// Recount matched items
	var matchedCount int
	s.db.QueryRow("SELECT COUNT(*) FROM receipt_line_items WHERE receipt_id = ? AND confidence IN ('high', 'medium')", receiptID).Scan(&matchedCount)
	s.db.Exec("UPDATE receipts SET items_matched = ? WHERE id = ?", matchedCount, receiptID)

	// Now verify any swap purchases
	swapsVerified := s.verifySwapPurchases(session.UserID, receiptID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"confirmed":      confirmed,
		"total_matched":  matchedCount,
		"swaps_verified": swapsVerified,
	})
}

// =============================================================================
// SWAP PURCHASE VERIFICATION
// =============================================================================

func (s *Server) verifySwapPurchases(userID, receiptID int) int {
	// Find accepted swaps for this user that haven't been verified yet
	rows, err := s.db.Query(`
		SELECT se.id, se.deal_id, bd.offer_brand, bd.offer_product
		FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE se.user_id = ? AND se.action = 'accepted'
		AND NOT EXISTS (SELECT 1 FROM swap_claims sc WHERE sc.swap_event_id = se.id AND sc.status = 'verified')`,
		userID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	type pendingSwap struct {
		eventID      int
		dealID       int
		offerBrand   string
		offerProduct string
	}
	var pending []pendingSwap
	for rows.Next() {
		var ps pendingSwap
		rows.Scan(&ps.eventID, &ps.dealID, &ps.offerBrand, &ps.offerProduct)
		pending = append(pending, ps)
	}

	if len(pending) == 0 {
		return 0
	}

	// Load receipt line items that were confirmed
	lineRows, err := s.db.Query(`
		SELECT matched_name FROM receipt_line_items
		WHERE receipt_id = ? AND confidence IN ('high', 'medium') AND matched_name != ''`,
		receiptID)
	if err != nil {
		return 0
	}
	defer lineRows.Close()

	var matchedNames []string
	for lineRows.Next() {
		var name string
		lineRows.Scan(&name)
		matchedNames = append(matchedNames, strings.ToLower(name))
	}

	verified := 0
	for _, ps := range pending {
		// Check if the swapped product appears in the receipt
		productLower := strings.ToLower(ps.offerProduct)
		brandLower := strings.ToLower(ps.offerBrand)
		for _, name := range matchedNames {
			if strings.Contains(name, productLower) || strings.Contains(name, brandLower) {
				// Create a swap claim with 30-day payout hold
				payoutHeldUntil := time.Now().Add(30 * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
				s.db.Exec(`INSERT INTO swap_claims (receipt_id, swap_event_id, deal_id, user_id, status, verified_at, payout_held_until)
					VALUES (?, ?, ?, ?, 'verified', CURRENT_TIMESTAMP, ?)`,
					receiptID, ps.eventID, ps.dealID, userID, payoutHeldUntil)
				verified++
				break
			}
		}
	}

	if verified > 0 {
		s.db.Exec("UPDATE receipts SET swaps_verified = ? WHERE id = ?", verified, receiptID)
		log.Printf("[receipt] verified %d swap purchases for user %d via receipt %d", verified, userID, receiptID)
	}

	return verified
}

// =============================================================================
// GET /api/receipts/claims — user's swap claims and payout status
// =============================================================================

func (s *Server) handleSwapClaims(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT sc.id, sc.receipt_id, sc.deal_id, sc.status, sc.verified_at,
			   sc.payout_held_until, sc.created_at,
			   bd.offer_brand, bd.offer_product, bd.discount_cents
		FROM swap_claims sc
		JOIN brand_deals bd ON sc.deal_id = bd.id
		WHERE sc.user_id = ?
		ORDER BY sc.created_at DESC LIMIT 50`, session.UserID)
	if err != nil {
		dbError(w, err, "Failed to load claims")
		return
	}
	defer rows.Close()

	type claim struct {
		ID              int    `json:"id"`
		ReceiptID       int    `json:"receipt_id"`
		DealID          int    `json:"deal_id"`
		Status          string `json:"status"`
		VerifiedAt      string `json:"verified_at"`
		PayoutHeldUntil string `json:"payout_held_until"`
		CreatedAt       string `json:"created_at"`
		OfferBrand      string `json:"offer_brand"`
		OfferProduct    string `json:"offer_product"`
		DiscountCents   int    `json:"discount_cents"`
	}
	claims := []claim{}
	for rows.Next() {
		var c claim
		rows.Scan(&c.ID, &c.ReceiptID, &c.DealID, &c.Status, &c.VerifiedAt,
			&c.PayoutHeldUntil, &c.CreatedAt, &c.OfferBrand, &c.OfferProduct, &c.DiscountCents)
		claims = append(claims, c)
	}
	writeJSON(w, http.StatusOK, claims)
}

// =============================================================================
// ENHANCED RECEIPT PROCESSING (called from parse handler)
// =============================================================================

// processReceiptEnhanced is called after the basic receipt parsing. It performs
// structured extraction, stores line items, and triggers swap verification.
func (s *Server) processReceiptEnhanced(userID, receiptID int, matchedNames []string, images []string) {
	if s.config.AnthropicAPIKey == "" {
		return
	}

	// Structured extraction via Claude
	structured := s.extractReceiptStructure(images)
	if structured != nil {
		// Update receipt with structured data
		s.db.Exec(`UPDATE receipts SET store_name = ?, store_address = ?, transaction_date = ?,
			subtotal_cents = ?, tax_cents = ?, total_cents = ? WHERE id = ?`,
			structured.StoreName, structured.StoreAddress, structured.TransactionDate,
			structured.SubtotalCents, structured.TaxCents, structured.TotalCents, receiptID)

		// Store line items
		for _, li := range structured.LineItems {
			confidence := "low"
			var matchedName string
			var matchedItemID int

			// Try to match against abbreviation database first
			var abbrevName string
			err := s.db.QueryRow(
				"SELECT full_name FROM retailer_abbreviations WHERE abbreviation = ? AND confirmations >= 2 ORDER BY confirmations DESC LIMIT 1",
				strings.ToUpper(li.RawText)).Scan(&abbrevName)
			if err == nil && abbrevName != "" {
				matchedName = abbrevName
				confidence = "medium"
			}

			// Check against matched names from Claude
			for _, mn := range matchedNames {
				if strings.EqualFold(mn, li.RawText) || strings.Contains(strings.ToLower(li.RawText), strings.ToLower(mn)) {
					matchedName = mn
					confidence = "high"
					break
				}
			}

			s.db.Exec(`INSERT INTO receipt_line_items (receipt_id, raw_text, price_cents, quantity, matched_item_id, matched_name, confidence)
				VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?)`,
				receiptID, li.RawText, li.PriceCents, li.Quantity, matchedItemID, matchedName, confidence)
		}
	}

	// Auto-verify swap purchases for high-confidence matches
	s.verifySwapPurchases(userID, receiptID)
}

type receiptStructure struct {
	StoreName       string            `json:"store_name"`
	StoreAddress    string            `json:"store_address"`
	TransactionDate string            `json:"transaction_date"`
	SubtotalCents   int               `json:"subtotal_cents"`
	TaxCents        int               `json:"tax_cents"`
	TotalCents      int               `json:"total_cents"`
	LineItems       []receiptLineItem `json:"line_items"`
}

type receiptLineItem struct {
	RawText    string `json:"raw_text"`
	PriceCents int    `json:"price_cents"`
	Quantity   int    `json:"quantity"`
}

func (s *Server) extractReceiptStructure(images []string) *receiptStructure {
	systemPrompt := `You are a receipt parser. Extract structured data from this receipt photo.

Return ONLY a JSON object with this exact structure:
{
  "store_name": "Store Name",
  "store_address": "123 Main St, City, State",
  "transaction_date": "2026-03-07",
  "subtotal_cents": 2499,
  "tax_cents": 200,
  "total_cents": 2699,
  "line_items": [
    {"raw_text": "ORG WHOLE MLK", "price_cents": 599, "quantity": 1},
    {"raw_text": "EGGS LG 12CT", "price_cents": 349, "quantity": 2}
  ]
}

Rules:
- price_cents is the price in cents (e.g. $5.99 = 599)
- quantity defaults to 1 unless specified on the receipt
- raw_text should be EXACTLY as printed on the receipt
- transaction_date in YYYY-MM-DD format
- If you can't determine a value, use empty string or 0
- Return ONLY the JSON, no markdown, no code fences`

	var contentBlocks []map[string]interface{}
	for _, img := range images {
		mediaType := "image/jpeg"
		b64Data := img
		if strings.HasPrefix(img, "data:") {
			if idx := strings.Index(img, ";base64,"); idx > 0 {
				mediaType = img[5:idx]
				b64Data = img[idx+8:]
			}
		}
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": "image",
			"source": map[string]string{
				"type":       "base64",
				"media_type": mediaType,
				"data":       b64Data,
			},
		})
	}
	contentBlocks = append(contentBlocks, map[string]interface{}{
		"type": "text",
		"text": "Extract all structured data from this receipt.",
	})

	text, err := callClaude(s.config.AnthropicAPIKey, systemPrompt, contentBlocks, 4096)
	if err != nil {
		log.Printf("[receipt] structured extraction failed: %v", err)
		return nil
	}
	text = stripCodeFence(text)

	var result receiptStructure
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		log.Printf("[receipt] failed to parse structured extraction: %v (text: %.200s)", err, text)
		return nil
	}
	return &result
}

// handleReceiptRouting routes /api/receipts/{id}/items and /api/receipts/{id}/confirm
func (s *Server) handleReceiptRouting(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/receipts/")
	if path == "claims" {
		s.handleSwapClaims(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		switch parts[1] {
		case "items":
			s.handleReceiptItems(w, r)
			return
		case "confirm":
			s.handleReceiptConfirm(w, r)
			return
		}
	}
	writeNotFound(w, "Not found")
}

// checkReceiptRateLimit enforces 3/day and 10/week per user.
func checkReceiptRateLimit(userID int) bool {
	key := fmt.Sprintf("receipt_%d", userID)
	if checkRateLimit(&receiptDayLimiter, key, 24*time.Hour, 3) {
		return true
	}
	if checkRateLimit(&receiptWeekLimiter, key, 7*24*time.Hour, 10) {
		return true
	}
	return false
}
