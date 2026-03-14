package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
)

// handleReferral handles GET /api/referral — returns referral code, points, and friends list.
func (s *Server) handleReferral(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	// Get or generate referral code
	var referralCode sql.NullString
	var points int
	err := s.db.QueryRow("SELECT referral_code, COALESCE(referral_points, 0) FROM users WHERE id = ?", session.UserID).Scan(&referralCode, &points)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't load your referral info. Please try again."})
		return
	}

	code := referralCode.String
	if !referralCode.Valid || code == "" {
		// Generate a new referral code
		codeBytes := make([]byte, 16)
		if _, err := rand.Read(codeBytes); err != nil {
			log.Printf("[referral] failed to generate code: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate referral code"})
			return
		}
		code = hex.EncodeToString(codeBytes)
		result, err := s.db.Exec("UPDATE users SET referral_code = ? WHERE id = ? AND (referral_code IS NULL OR referral_code = '')", code, session.UserID)
		if err != nil {
			log.Printf("[referral] failed to save referral code: %v", err)
		} else if n, _ := result.RowsAffected(); n == 0 {
			// Another request already set the code — re-read it
			s.db.QueryRow("SELECT referral_code FROM users WHERE id = ?", session.UserID).Scan(&code)
		}
	}

	// Get referred friends
	type referredFriend struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	var friends []referredFriend
	rows, err := s.db.Query(
		"SELECT COALESCE(u.name, ''), r.created_at FROM referrals r JOIN users u ON u.id = r.referred_id WHERE r.referrer_id = ? ORDER BY r.created_at DESC",
		session.UserID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f referredFriend
			if rows.Scan(&f.Name, &f.CreatedAt) == nil {
				friends = append(friends, f)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[referral] friends row iteration error: %v", err)
		}
	}
	if friends == nil {
		friends = []referredFriend{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":           code,
		"points":         points,
		"referral_count": len(friends),
		"friends":        friends,
	})
}

// handleReferralRedeem handles POST /api/referral/redeem — redeem 100 points for $10.
func (s *Server) handleReferralRedeem(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}
	defer tx.Rollback()

	// Atomic check-and-deduct to prevent TOCTOU race on point balance
	result, err := tx.Exec("UPDATE users SET referral_points = referral_points - ? WHERE id = ? AND COALESCE(referral_points, 0) >= ?", referralRedeemThreshold, session.UserID, referralRedeemThreshold)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		writeBadRequest(w, "You need at least 100 points to redeem.")
		return
	}
	if _, err := tx.Exec("INSERT INTO redemptions (user_id, points, amount_cents) VALUES (?, ?, ?)", session.UserID, referralRedeemThreshold, referralRedeemCents); err != nil {
		log.Printf("[referral] failed to insert redemption: %v", err)
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your redemption. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "$10 will be Venmo'd to your number!"})
}

// handleReferralClaim handles POST /api/referral/claim — called on signup with a referral code.
func (s *Server) handleReferralClaim(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Code == "" {
		writeBadRequest(w, "Referral code is required.")
		return
	}

	// Look up referrer by code
	var referrerID int
	err := s.db.QueryRow("SELECT id FROM users WHERE referral_code = ?", req.Code).Scan(&referrerID)
	if err != nil {
		writeNotFound(w, "Invalid referral code.")
		return
	}

	// Don't let users refer themselves
	if referrerID == session.UserID {
		writeBadRequest(w, "You can't use your own referral code.")
		return
	}

	// Idempotent — check if already claimed
	var existingID int
	err = s.db.QueryRow("SELECT id FROM referrals WHERE referrer_id = ? AND referred_id = ?", referrerID, session.UserID).Scan(&existingID)
	if err == nil {
		// Already claimed — return success without double-crediting
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT INTO referrals (referrer_id, referred_id) VALUES (?, ?)", referrerID, session.UserID); err != nil {
		log.Printf("[referral] failed to insert referral: %v", err)
	}
	if _, err := tx.Exec("UPDATE users SET referral_points = COALESCE(referral_points, 0) + ? WHERE id = ?", referralPointsPerInvite, referrerID); err != nil {
		log.Printf("[referral] failed to update points: %v", err)
	}
	// M2d: Track referral source
	tx.Exec("UPDATE users SET referral_source = 'referral' WHERE id = ? AND referral_source = ''", session.UserID)

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "We couldn't process your referral. Please try again."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// REBATE HISTORY
// =============================================================================

// handleRebates handles GET /api/rebates — returns user's rebate history and summary.
func (s *Server) handleRebates(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	type rebateEntry struct {
		ID           int    `json:"id"`
		OfferBrand   string `json:"offer_brand"`
		OfferProduct string `json:"offer_product"`
		AmountCents  int    `json:"amount_cents"`
		Status       string `json:"status"`
		CreatedAt    string `json:"created_at"`
	}

	rows, err := s.db.Query(`
		SELECT sr.id, bd.offer_brand, bd.offer_product, sr.amount_cents, sr.status, sr.created_at
		FROM swap_rebates sr
		JOIN brand_deals bd ON bd.id = sr.deal_id
		WHERE sr.user_id = ?
		ORDER BY sr.created_at DESC
		LIMIT 100
	`, session.UserID)
	if err != nil {
		log.Printf("[rebates] query error for user %d: %v", session.UserID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load rebates"})
		return
	}
	defer rows.Close()

	rebates := []rebateEntry{}
	for rows.Next() {
		var e rebateEntry
		if err := rows.Scan(&e.ID, &e.OfferBrand, &e.OfferProduct, &e.AmountCents, &e.Status, &e.CreatedAt); err != nil {
			log.Printf("[rebates] scan error: %v", err)
			continue
		}
		rebates = append(rebates, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[rebates] row iteration error: %v", err)
	}

	// Summary totals
	var pendingCents, paidCents, totalCents int
	for _, r := range rebates {
		totalCents += r.AmountCents
		switch r.Status {
		case "pending":
			pendingCents += r.AmountCents
		case "issued", "paid":
			paidCents += r.AmountCents
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rebates":       rebates,
		"total_cents":   totalCents,
		"pending_cents": pendingCents,
		"paid_cents":    paidCents,
	})
}
