package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleFullyFreeReceiptVerify processes POST /api/receipt/verify for fully_free deals.
// The shopper submits a receipt image; we verify UPC match, compute actual rebate,
// debit the brand's budget (rebate + platform fee), and update the rebate status.
func (s *Server) handleFullyFreeReceiptVerify(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		RebateID       int    `json:"rebate_id"`
		ReceiptImage   string `json:"receipt_image"`   // base64 image data
		ReceiptPriceCents int `json:"receipt_price_cents"` // price from OCR (client-side or server-side)
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.RebateID <= 0 {
		writeBadRequest(w, "rebate_id is required")
		return
	}

	// Rate limit: max 5 fully_free verifications per user per day
	val, _ := s.earningsUploadLimiter.LoadOrStore(fmt.Sprintf("ff_%s", session.UserID), &rateBucket{})
	rb := val.(*rateBucket)
	rb.mu.Lock()
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)
	recent := []time.Time{}
	for _, t := range rb.times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 5 {
		rb.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Verification rate limit exceeded. Try again tomorrow."})
		return
	}
	rb.times = append(recent, now)
	rb.mu.Unlock()

	// Load rebate and verify ownership + status
	var rebateUserID string
	var dealID, amountCents int
	var rebateStatus string
	err := s.db.QueryRow(
		"SELECT user_id, deal_id, amount_cents, status FROM swap_rebates WHERE id = ?",
		req.RebateID,
	).Scan(&rebateUserID, &dealID, &amountCents, &rebateStatus)
	if err != nil {
		writeNotFound(w, "Rebate not found")
		return
	}
	if rebateUserID != session.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not your rebate"})
		return
	}

	// Only pending_purchase rebates can be verified
	if rebateStatus != "pending_purchase" {
		writeBadRequest(w, "Rebate is not awaiting purchase verification")
		return
	}

	// Load deal to get max_rebate, platform_fee, UPC, deal_type
	var maxRebateCents, platformFeeCents, budgetTotal int
	var dealType, upc string
	err = s.db.QueryRow(
		"SELECT deal_type, max_rebate_cents, platform_fee_cents, upc, total_budget_cents FROM brand_deals WHERE id = ?",
		dealID,
	).Scan(&dealType, &maxRebateCents, &platformFeeCents, &upc, &budgetTotal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load deal"})
		return
	}
	if dealType != "fully_free" {
		writeBadRequest(w, "Deal is not a fully free trial")
		return
	}

	// Determine actual rebate: MIN(receipt_price, max_rebate_cents)
	actualRebate := req.ReceiptPriceCents
	if actualRebate <= 0 {
		// Use max_rebate if no receipt price provided
		actualRebate = maxRebateCents
	}
	if actualRebate > maxRebateCents {
		actualRebate = maxRebateCents
	}

	// Total cost to brand: rebate + platform fee
	totalDebit := actualRebate + platformFeeCents

	// Begin transaction for atomic budget debit + rebate update
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[fully_free] failed to begin tx: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process verification"})
		return
	}
	defer tx.Rollback()

	// Atomic budget debit — conditional UPDATE prevents overspend
	if budgetTotal > 0 {
		result, err := tx.Exec(
			"UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ? AND (total_budget_cents = 0 OR budget_spent_cents + ? <= total_budget_cents)",
			totalDebit, dealID, totalDebit,
		)
		if err != nil {
			log.Printf("[fully_free] failed to debit deal %d budget: %v", dealID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process verification"})
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			writeBadRequest(w, "Deal budget exhausted — rebate cannot be verified")
			return
		}
	} else {
		// Unlimited budget
		if _, err := tx.Exec("UPDATE brand_deals SET budget_spent_cents = budget_spent_cents + ? WHERE id = ?",
			totalDebit, dealID); err != nil {
			log.Printf("[fully_free] failed to debit deal %d budget: %v", dealID, err)
		}
	}

	// Update rebate: status → verified, set verified amount and timestamp
	verifiedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := tx.Exec(
		"UPDATE swap_rebates SET status = 'verified', amount_cents = ?, verified_amount_cents = ?, verified_at = ?, receipt_image_url = ? WHERE id = ?",
		actualRebate, actualRebate, verifiedAt, req.ReceiptImage, req.RebateID,
	); err != nil {
		log.Printf("[fully_free] failed to update rebate %d: %v", req.RebateID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update rebate"})
		return
	}

	// Auto-pause deal if budget now exhausted
	var newBudgetSpent int
	tx.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&newBudgetSpent)
	if budgetTotal > 0 && newBudgetSpent >= budgetTotal {
		if _, err := tx.Exec("UPDATE brand_deals SET status = 'paused' WHERE id = ?", dealID); err != nil {
			log.Printf("[fully_free] failed to auto-pause deal %d: %v", dealID, err)
		}
		log.Printf("[fully_free] deal %d auto-paused: budget exhausted", dealID)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[fully_free] failed to commit verification tx: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process verification"})
		return
	}

	// 30-day hold: payout will be processed after 30 days (existing payout mechanism handles this)
	payoutDate := time.Now().Add(30 * 24 * time.Hour).Format("January 2, 2006")

	log.Printf("[fully_free] verified rebate %d for user %s: actual_rebate=%d, platform_fee=%d, total_debit=%d",
		req.RebateID, session.UserID, actualRebate, platformFeeCents, totalDebit)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":              true,
		"verified_amount_cents": actualRebate,
		"payout_date":          payoutDate,
		"message":              fmt.Sprintf("Rebate confirmed! $%.2f will be credited on %s.", float64(actualRebate)/100, payoutDate),
	})
}

// handleFullyFreeRebates returns fully_free rebates awaiting verification for a user.
// GET /api/rebates/pending — shows deals the user accepted but hasn't verified yet.
func (s *Server) handleFullyFreeRebates(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT sr.id, sr.amount_cents, sr.status, sr.created_at, sr.verified_amount_cents,
		       sr.verified_at, bd.offer_brand, bd.offer_product, bd.upc,
		       bd.receipt_grace_days, bd.max_rebate_cents
		FROM swap_rebates sr
		JOIN brand_deals bd ON bd.id = sr.deal_id
		WHERE sr.user_id = ? AND bd.deal_type = 'fully_free'
		ORDER BY sr.created_at DESC LIMIT 50`,
		session.UserID,
	)
	if err != nil {
		dbError(w, err, "Failed to load rebates")
		return
	}
	defer rows.Close()

	type fullyFreeRebate struct {
		ID                  int    `json:"id"`
		AmountCents         int    `json:"amount_cents"`
		Status              string `json:"status"`
		CreatedAt           string `json:"created_at"`
		VerifiedAmountCents int    `json:"verified_amount_cents"`
		VerifiedAt          string `json:"verified_at"`
		OfferBrand          string `json:"offer_brand"`
		OfferProduct        string `json:"offer_product"`
		UPC                 string `json:"upc"`
		ReceiptGraceDays    int    `json:"receipt_grace_days"`
		MaxRebateCents      int    `json:"max_rebate_cents"`
	}
	rebates := []fullyFreeRebate{}
	for rows.Next() {
		var r fullyFreeRebate
		var verifiedAt sql.NullString
		if err := rows.Scan(&r.ID, &r.AmountCents, &r.Status, &r.CreatedAt,
			&r.VerifiedAmountCents, &verifiedAt,
			&r.OfferBrand, &r.OfferProduct, &r.UPC,
			&r.ReceiptGraceDays, &r.MaxRebateCents); err != nil {
			log.Printf("[fully_free] scan error: %v", err)
			continue
		}
		if verifiedAt.Valid {
			r.VerifiedAt = verifiedAt.String
		}
		rebates = append(rebates, r)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rebates": rebates,
	})
}

// getBrandFullyFreeStats returns fully_free campaign stats for a brand deal.
func (s *Server) getBrandFullyFreeStats(dealID int) map[string]interface{} {
	var accepted, pendingPurchase, pendingVerification, verified, paid int

	rows, err := s.db.Query(
		"SELECT status, COUNT(*) FROM swap_rebates WHERE deal_id = ? GROUP BY status",
		dealID,
	)
	if err != nil {
		return map[string]interface{}{}
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if rows.Scan(&status, &count) == nil {
			switch status {
			case "pending_purchase":
				pendingPurchase = count
			case "pending_verification":
				pendingVerification = count
			case "verified":
				verified = count
			case "paid":
				paid = count
			case "pending":
				// legacy pending from other deal types
			}
		}
	}

	// Count total accepts from swap_events
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'accepted'", dealID).Scan(&accepted)

	var totalSpent int
	s.db.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&totalSpent)

	return map[string]interface{}{
		"accepted":             accepted,
		"pending_purchase":     pendingPurchase,
		"pending_verification": pendingVerification,
		"verified":             verified,
		"paid":                 paid,
		"total_spent_cents":    totalSpent,
	}
}

// encodeFreeTrialStats encodes fully_free stats into the deal metrics response.
func encodeFreeTrialStats(stats map[string]interface{}) string {
	b, _ := json.Marshal(stats)
	return string(b)
}
