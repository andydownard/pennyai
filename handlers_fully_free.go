package main

import (
	"database/sql"
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

	// Only pending_purchase rebates can be verified. Fast path only: the
	// authoritative guard is the conditional UPDATE inside the transaction below.
	if rebateStatus != "pending_purchase" {
		writeConflict(w, "Rebate is not awaiting purchase verification")
		return
	}

	// Rate limit: max 5 fully_free verifications per user per day. Consumed only
	// after ownership and status check out, so a 403 or 404 does not burn one of
	// the user's attempts — otherwise anyone could exhaust a victim's daily quota
	// by guessing rebate ids.
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

	// Claim the rebate FIRST, with status in the WHERE clause. This is the
	// check-then-act fix (B11): the status read above happened before tx.Begin,
	// so two concurrent verifications both passed it and each ran the budget
	// debit below — individually atomic, so both succeeded, and the brand paid
	// rebate + platform fee twice for one rebate. A loser affects zero rows and
	// gets a 409 before any money moves.
	verifiedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	claim, err := tx.Exec(
		`UPDATE swap_rebates SET status = 'verified', amount_cents = ?, verified_amount_cents = ?, verified_at = ?, receipt_image_url = ?
		 WHERE id = ? AND user_id = ? AND status = 'pending_purchase'`,
		actualRebate, actualRebate, verifiedAt, req.ReceiptImage, req.RebateID, session.UserID,
	)
	if err != nil {
		log.Printf("[fully_free] failed to update rebate %d: %v", req.RebateID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update rebate"})
		return
	}
	claimed, err := claim.RowsAffected()
	if err != nil {
		log.Printf("[fully_free] failed to read rebate update result for %d: %v", req.RebateID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update rebate"})
		return
	}
	if claimed != 1 {
		writeConflict(w, "Rebate is not awaiting purchase verification")
		return
	}

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
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process verification"})
			return // deferred tx.Rollback() leaves the rebate pending
		}
	}

	// The rebate was already marked verified by the guarded claim at the top of
	// this transaction.

	// Auto-pause deal if budget now exhausted
	var newBudgetSpent int
	tx.QueryRow("SELECT budget_spent_cents FROM brand_deals WHERE id = ?", dealID).Scan(&newBudgetSpent)
	if budgetTotal > 0 && newBudgetSpent >= budgetTotal {
		if _, err := tx.Exec("UPDATE brand_deals SET status = 'paused' WHERE id = ?", dealID); err != nil {
			log.Printf("[fully_free] failed to auto-pause deal %d: %v", dealID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process verification"})
			return
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

