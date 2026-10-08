package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (Session, bool) {
	// Cookie path first — a usable cookie always wins and costs nothing extra.
	staleCookie := false
	if cookie, err := r.Cookie("session"); err == nil {
		if session, ok := s.sessions.GetSession(cookie.Value); ok {
			return session, true
		}
		log.Printf("[auth] invalid/expired session on %s %s", r.Method, r.URL.Path)
		staleCookie = true
	} else {
		log.Printf("[auth] no session cookie on %s %s: %v", r.Method, r.URL.Path, err)
	}

	// No usable cookie: try an Authorization: Bearer Clerk session JWT. With
	// CLERK_ENABLED unset this is one env read and nothing else, so the dark
	// build is identical to today's.
	clerkSession, err := VerifyClerkRequest(r, s.resolveClerkSession)
	if err != nil {
		dbError(w, err, "Failed to verify sign-in")
		return Session{}, false
	}
	if clerkSession != nil {
		return *clerkSession, true
	}

	// Same two messages, same conditions, as before this change.
	msg := "Not logged in"
	if staleCookie {
		msg = "Session expired"
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": msg})
	return Session{}, false
}

// isAdmin reports whether the session belongs to an owner. Server-side only:
// either the session phone is ADMIN_PHONE, or the user's Clerk id is in the
// ADMIN_CLERK_USER_IDS allowlist (so an owner without a phone can be admin).
func (s *Server) isAdmin(session Session) bool {
	if s.config.AdminPhone != "" && session.PhoneNumber != "" && session.PhoneNumber == s.config.AdminPhone {
		return true
	}
	if len(s.config.AdminClerkUserIDs) == 0 || session.UserID == "" {
		return false
	}
	var clerkID sql.NullString
	if err := s.db.QueryRow("SELECT clerk_user_id FROM users WHERE id = ?", session.UserID).Scan(&clerkID); err != nil || !clerkID.Valid || clerkID.String == "" {
		return false
	}
	for _, id := range s.config.AdminClerkUserIDs {
		if id == clerkID.String {
			return true
		}
	}
	return false
}

// requireAdmin checks for a valid session that isAdmin accepts.
// Returns the session and true if authorized, or writes an error and returns false.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return Session{}, false
	}
	if !s.isAdmin(session) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return Session{}, false
	}
	return session, true
}

// handleAdminDashboard handles GET /api/admin/dashboard?days=30
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	modifier := fmt.Sprintf("-%d days", days)

	// Use reporting DB if available to avoid hitting production
	db := s.adminDB()

	var activeUsers, totalUsers, newUsers int
	if err := db.QueryRow("SELECT COUNT(DISTINCT id) FROM users WHERE last_active >= datetime('now', ?)", modifier).Scan(&activeUsers); err != nil {
		log.Printf("[admin] failed to count active users: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers); err != nil {
		log.Printf("[admin] failed to count total users: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE created_at >= datetime('now', ?)", modifier).Scan(&newUsers); err != nil {
		log.Printf("[admin] failed to count new users: %v", err)
	}

	var referralsInPeriod int
	if err := db.QueryRow("SELECT COUNT(*) FROM referrals WHERE created_at >= datetime('now', ?)", modifier).Scan(&referralsInPeriod); err != nil {
		log.Printf("[admin] failed to count referrals: %v", err)
	}

	var kFactor float64
	if newUsers > 0 {
		kFactor = float64(referralsInPeriod) / float64(newUsers)
	}

	type leaderEntry struct {
		Phone    string `json:"phone"`
		Name     string `json:"name"`
		Points   int    `json:"points"`
		Referrals int   `json:"referrals"`
	}

	rows, err := db.Query(`
		SELECT COALESCE(u.phone,''), COALESCE(u.name,''), COALESCE(u.referral_points,0), COUNT(r.id) as ref_count
		FROM users u
		JOIN referrals r ON r.referrer_id = u.id
		GROUP BY u.id
		ORDER BY COALESCE(u.referral_points,0) DESC
		LIMIT 20
	`)
	var leaderboard []leaderEntry
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e leaderEntry
			if err := rows.Scan(&e.Phone, &e.Name, &e.Points, &e.Referrals); err == nil {
				e.Phone = maskPhone(e.Phone)
				leaderboard = append(leaderboard, e)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[admin] leaderboard row iteration error: %v", err)
		}
	}
	if leaderboard == nil {
		leaderboard = []leaderEntry{}
	}

	// NPS data
	type npsEntry struct {
		Score     int    `json:"score"`
		Comment   string `json:"comment"`
		UserName  string `json:"user_name"`
		UserPhone string `json:"user_phone"`
		CreatedAt string `json:"created_at"`
	}

	var npsScore float64
	var npsTotal, npsPromoters, npsDetractors int
	npsRow := db.QueryRow(`
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN score >= 9 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN score <= 6 THEN 1 ELSE 0 END), 0)
		FROM nps_responses WHERE created_at >= datetime('now', ?)`, modifier)
	if err := npsRow.Scan(&npsTotal, &npsPromoters, &npsDetractors); err != nil {
		log.Printf("[admin] failed to calculate NPS: %v", err)
	}
	if npsTotal > 0 {
		npsScore = (float64(npsPromoters) - float64(npsDetractors)) / float64(npsTotal) * 100
	}

	npsRows, err := db.Query(`
		SELECT n.score, n.comment, COALESCE(u.name,''), COALESCE(u.phone,''), n.created_at
		FROM nps_responses n
		JOIN users u ON u.id = n.user_id
		WHERE n.created_at >= datetime('now', ?)
		ORDER BY n.created_at DESC
		LIMIT 100
	`, modifier)
	var npsResponses []npsEntry
	if err == nil {
		defer npsRows.Close()
		for npsRows.Next() {
			var e npsEntry
			if err := npsRows.Scan(&e.Score, &e.Comment, &e.UserName, &e.UserPhone, &e.CreatedAt); err == nil {
				e.UserPhone = maskPhone(e.UserPhone)
				npsResponses = append(npsResponses, e)
			}
		}
		if err := npsRows.Err(); err != nil {
			log.Printf("[admin] NPS row iteration error: %v", err)
		}
	}
	if npsResponses == nil {
		npsResponses = []npsEntry{}
	}

	// Swap deal stats — single query
	var swapOffered, swapAccepted, swapDeclined int
	db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN action='offered' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='accepted' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='declined' THEN 1 END), 0)
		FROM swap_events WHERE created_at >= datetime('now', ?)`, modifier).Scan(&swapOffered, &swapAccepted, &swapDeclined)
	var swapAcceptRate float64
	if swapOffered > 0 {
		swapAcceptRate = float64(swapAccepted) / float64(swapOffered) * 100
	}

	// Rebate ledger stats
	var rebatesPendingCents, rebatesIssuedCents int
	db.QueryRow("SELECT COALESCE(SUM(amount_cents),0) FROM swap_rebates WHERE status = 'pending'").Scan(&rebatesPendingCents)
	db.QueryRow("SELECT COALESCE(SUM(amount_cents),0) FROM swap_rebates WHERE status IN ('issued','paid')").Scan(&rebatesIssuedCents)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_users":           activeUsers,
		"total_users":            totalUsers,
		"new_users":              newUsers,
		"k_factor":               kFactor,
		"leaderboard":            leaderboard,
		"days":                   days,
		"nps_responses":          npsResponses,
		"nps_score":              npsScore,
		"nps_total":              npsTotal,
		"nps_promoters":          npsPromoters,
		"nps_detractors":         npsDetractors,
		"swap_offered":           swapOffered,
		"swap_accepted":          swapAccepted,
		"swap_declined":          swapDeclined,
		"swap_accept_rate":       swapAcceptRate,
		"rebates_pending_cents":  rebatesPendingCents,
		"rebates_issued_cents":   rebatesIssuedCents,
	})
}

// handleAdminAnalytics handles GET /api/admin/analytics?days=30
// Returns anonymized/aggregated item event data from item_modifications.
func (s *Server) handleAdminAnalytics(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}
	modifier := fmt.Sprintf("-%d days", days)

	// Use reporting DB if available
	db := s.adminDB()

	// 1. Items created
	var itemsCreated int
	db.QueryRow("SELECT COUNT(*) FROM item_modifications WHERE action = 'added' AND created_at >= datetime('now', ?)", modifier).Scan(&itemsCreated)

	// 2. Items by source (only 'added' actions)
	itemsBySource := map[string]int{}
	srcRows, err := db.Query("SELECT COALESCE(source,'unknown'), COUNT(*) FROM item_modifications WHERE action = 'added' AND created_at >= datetime('now', ?) GROUP BY COALESCE(source,'unknown')", modifier)
	if err == nil {
		defer srcRows.Close()
		for srcRows.Next() {
			var src string
			var cnt int
			if srcRows.Scan(&src, &cnt) == nil {
				itemsBySource[src] = cnt
			}
		}
	}

	// 3. Items by department (only 'added' actions)
	itemsByDept := map[string]int{}
	deptRows, err := db.Query(`
		SELECT COALESCE(li.department,'Other'), COUNT(*)
		FROM item_modifications im
		JOIN list_items li ON li.id = im.item_id
		WHERE im.action = 'added' AND im.created_at >= datetime('now', ?)
		GROUP BY COALESCE(li.department,'Other')
		ORDER BY COUNT(*) DESC
	`, modifier)
	if err == nil {
		defer deptRows.Close()
		for deptRows.Next() {
			var dept string
			var cnt int
			if deptRows.Scan(&dept, &cnt) == nil {
				itemsByDept[dept] = cnt
			}
		}
	}

	// 4. Events by action (all actions)
	eventsByAction := map[string]int{}
	actRows, err := db.Query("SELECT action, COUNT(*) FROM item_modifications WHERE created_at >= datetime('now', ?) GROUP BY action", modifier)
	if err == nil {
		defer actRows.Close()
		for actRows.Next() {
			var action string
			var cnt int
			if actRows.Scan(&action, &cnt) == nil {
				eventsByAction[action] = cnt
			}
		}
	}

	// 5. Events by day (all actions)
	type dayCount struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	var eventsByDay []dayCount
	dayRows, err := db.Query("SELECT date(created_at), COUNT(*) FROM item_modifications WHERE created_at >= datetime('now', ?) GROUP BY date(created_at) ORDER BY date(created_at)", modifier)
	if err == nil {
		defer dayRows.Close()
		for dayRows.Next() {
			var dc dayCount
			if dayRows.Scan(&dc.Date, &dc.Count) == nil {
				eventsByDay = append(eventsByDay, dc)
			}
		}
	}
	if eventsByDay == nil {
		eventsByDay = []dayCount{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"days":              days,
		"items_created":     itemsCreated,
		"items_by_source":   itemsBySource,
		"items_by_department": itemsByDept,
		"events_by_action":  eventsByAction,
		"events_by_day":     eventsByDay,
	})
}

// handleAdminDeals handles GET/POST/PUT/DELETE /api/admin/deals
func (s *Server) handleAdminDeals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`
			SELECT id, competitor_product, offer_brand, offer_product, discount_cents,
			       total_budget_cents, budget_spent_cents, status,
			       starts_at, COALESCE(ends_at,''), created_at,
			       offer_price_cents, competitor_price_cents,
			       offer_size, competitor_size,
			       offer_unit_price, competitor_unit_price,
			       unit_label, why_bullets, allergen_notes,
			       deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag,
			       COALESCE(review_status,''), COALESCE(review_note,''),
			       COALESCE(reviewed_by,''), COALESCE(reviewed_at,''), COALESCE(submitted_at,'')
			FROM brand_deals WHERE status != 'deleted'
			ORDER BY CASE WHEN status = 'pending_review' THEN 0 ELSE 1 END, created_at DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load deals"})
			return
		}
		defer rows.Close()

		var deals []BrandDeal
		for rows.Next() {
			var d BrandDeal
			if err := rows.Scan(&d.ID, &d.CompetitorProduct, &d.OfferBrand, &d.OfferProduct,
				&d.DiscountCents, &d.TotalBudgetCents, &d.BudgetSpentCents, &d.Status,
				&d.StartsAt, &d.EndsAt, &d.CreatedAt,
				&d.OfferPriceCents, &d.CompetitorPriceCents,
				&d.OfferSize, &d.CompetitorSize,
				&d.OfferUnitPrice, &d.CompetitorUnitPrice,
				&d.UnitLabel, &d.WhyBullets, &d.AllergenNotes,
				&d.DealType, &d.QuantityOffered, &d.TrialProductName, &d.TrialSize, &d.CampaignTag,
				&d.ReviewStatus, &d.ReviewNote, &d.ReviewedBy, &d.ReviewedAt, &d.SubmittedAt); err != nil {
				log.Printf("[admin-deals] scan error: %v", err)
				continue
			}
			deals = append(deals, d)
		}
		if deals == nil {
			deals = []BrandDeal{}
		}
		writeJSON(w, http.StatusOK, deals)

	case http.MethodPost:
		var deal BrandDeal
		if err := json.NewDecoder(r.Body).Decode(&deal); err != nil {
			writeBadRequest(w, "Invalid request body")
			return
		}
		deal.CompetitorProduct = strings.TrimSpace(deal.CompetitorProduct)
		deal.OfferBrand = strings.TrimSpace(deal.OfferBrand)
		deal.OfferProduct = strings.TrimSpace(deal.OfferProduct)
		if deal.CompetitorProduct == "" || deal.OfferBrand == "" || deal.OfferProduct == "" {
			writeBadRequest(w, "competitor_product, offer_brand, and offer_product are required")
			return
		}
		if deal.DiscountCents < 0 || deal.TotalBudgetCents < 0 || deal.OfferPriceCents < 0 || deal.CompetitorPriceCents < 0 {
			writeBadRequest(w, "Price and budget values must not be negative")
			return
		}
		if deal.Status == "" {
			deal.Status = "active"
		}
		if deal.DealType == "" {
			deal.DealType = "sku_swap"
		}
		if deal.QuantityOffered == 0 {
			deal.QuantityOffered = 1
		}

		result, err := s.db.Exec(`
			INSERT INTO brand_deals (competitor_product, offer_brand, offer_product, discount_cents,
			                         total_budget_cents, status, ends_at,
			                         offer_price_cents, competitor_price_cents,
			                         offer_size, competitor_size,
			                         offer_unit_price, competitor_unit_price,
			                         unit_label, why_bullets, allergen_notes,
			                         deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			deal.CompetitorProduct, deal.OfferBrand, deal.OfferProduct, deal.DiscountCents,
			deal.TotalBudgetCents, deal.Status, nullIfEmpty(deal.EndsAt),
			deal.OfferPriceCents, deal.CompetitorPriceCents,
			deal.OfferSize, deal.CompetitorSize,
			deal.OfferUnitPrice, deal.CompetitorUnitPrice,
			deal.UnitLabel, jsonArrayText(deal.WhyBullets), deal.AllergenNotes,
			deal.DealType, deal.QuantityOffered, deal.TrialProductName, deal.TrialSize, deal.CampaignTag,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create deal"})
			return
		}
		id, _ := result.LastInsertId()
		deal.ID = int(id)
		writeJSON(w, http.StatusCreated, deal)

	case http.MethodPut:
		var deal BrandDeal
		if err := json.NewDecoder(r.Body).Decode(&deal); err != nil {
			writeBadRequest(w, "Invalid request body")
			return
		}
		if deal.ID == 0 {
			writeBadRequest(w, "id is required")
			return
		}
		_, err := s.db.Exec(`
			UPDATE brand_deals SET competitor_product = ?, offer_brand = ?, offer_product = ?,
			       discount_cents = ?, total_budget_cents = ?, status = ?, ends_at = ?,
			       offer_price_cents = ?, competitor_price_cents = ?,
			       offer_size = ?, competitor_size = ?,
			       offer_unit_price = ?, competitor_unit_price = ?,
			       unit_label = ?, why_bullets = ?, allergen_notes = ?,
			       deal_type = ?, quantity_offered = ?, trial_product_name = ?, trial_size = ?, campaign_tag = ?
			WHERE id = ?`,
			deal.CompetitorProduct, deal.OfferBrand, deal.OfferProduct,
			deal.DiscountCents, deal.TotalBudgetCents, deal.Status, nullIfEmpty(deal.EndsAt),
			deal.OfferPriceCents, deal.CompetitorPriceCents,
			deal.OfferSize, deal.CompetitorSize,
			deal.OfferUnitPrice, deal.CompetitorUnitPrice,
			deal.UnitLabel, jsonArrayText(deal.WhyBullets), deal.AllergenNotes,
			deal.DealType, deal.QuantityOffered, deal.TrialProductName, deal.TrialSize, deal.CampaignTag,
			deal.ID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update deal"})
			return
		}
		writeJSON(w, http.StatusOK, deal)

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
			writeBadRequest(w, "id is required")
			return
		}
		// Soft-delete: set status='deleted' instead of hard DELETE to preserve
		// FK references from swap_events and swap_rebates (B12 fix)
		result, err := s.db.Exec("UPDATE brand_deals SET status = 'deleted' WHERE id = ?", req.ID)
		if err != nil {
			log.Printf("[admin] failed to soft-delete deal %d: %v", req.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete deal"})
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			writeNotFound(w, "Deal not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleAdminDealReview handles POST /api/admin/deals/{id}/review
// Allows admins to approve or reject brand campaigns before they go live.
func (s *Server) handleAdminDealReview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	// Extract deal ID from path: /api/admin/deals/{id}/review
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/deals/")
	path = strings.TrimSuffix(path, "/review")
	dealID, err := strconv.Atoi(path)
	if err != nil || dealID == 0 {
		writeBadRequest(w, "Invalid deal ID")
		return
	}

	var req struct {
		Action string `json:"action"` // "approve" or "reject"
		Note   string `json:"note"`   // optional review note
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Action != "approve" && req.Action != "reject" {
		writeBadRequest(w, "action must be 'approve' or 'reject'")
		return
	}

	// Verify the deal exists and is pending review
	var currentStatus, currentReviewStatus string
	err = s.db.QueryRow("SELECT status, COALESCE(review_status,'') FROM brand_deals WHERE id = ?", dealID).Scan(&currentStatus, &currentReviewStatus)
	if err != nil {
		writeNotFound(w, "Deal not found")
		return
	}
	if currentStatus != "pending_review" {
		writeBadRequest(w, fmt.Sprintf("Deal is not pending review (current status: %s)", currentStatus))
		return
	}

	// Get admin identifier for audit trail
	adminIdentifier := admin.PhoneNumber
	if adminIdentifier == "" {
		adminIdentifier = fmt.Sprintf("user_%s", admin.UserID)
	}

	if req.Action == "approve" {
		_, err = s.db.Exec(`UPDATE brand_deals SET status = 'active', review_status = 'approved', review_note = ?, reviewed_by = ?, reviewed_at = CURRENT_TIMESTAMP WHERE id = ?`,
			req.Note, adminIdentifier, dealID)
		if err != nil {
			dbError(w, err, "Failed to approve deal")
			return
		}
		log.Printf("[admin] deal %d approved by %s", dealID, adminIdentifier)
		writeJSON(w, http.StatusOK, map[string]string{"message": "Deal approved and now live", "status": "active"})
	} else {
		_, err = s.db.Exec(`UPDATE brand_deals SET status = 'rejected', review_status = 'rejected', review_note = ?, reviewed_by = ?, reviewed_at = CURRENT_TIMESTAMP WHERE id = ?`,
			req.Note, adminIdentifier, dealID)
		if err != nil {
			dbError(w, err, "Failed to reject deal")
			return
		}
		log.Printf("[admin] deal %d rejected by %s: %s", dealID, adminIdentifier, req.Note)
		writeJSON(w, http.StatusOK, map[string]string{"message": "Deal rejected", "status": "rejected"})
	}
}

// handleAdminPayouts handles GET/POST /api/admin/payouts
// GET: returns users with pending swap rebates, their contact info, and amounts due.
// POST: marks a payout as issued (updates rebate status from pending → issued).
func (s *Server) handleAdminPayouts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`
			SELECT u.id, COALESCE(u.name,''), COALESCE(u.email,''), COALESCE(u.phone,''),
			       SUM(sr.amount_cents) as total_cents, COUNT(sr.id) as swap_count,
			       MIN(sr.created_at) as oldest
			FROM swap_rebates sr
			JOIN users u ON u.id = sr.user_id
			WHERE sr.status = 'pending'
			GROUP BY u.id
			ORDER BY total_cents DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load payouts"})
			return
		}
		defer rows.Close()

		type payoutEntry struct {
			UserID     string `json:"user_id"`
			Name       string `json:"name"`
			Email      string `json:"email"`
			Phone      string `json:"phone"`
			TotalCents int    `json:"total_cents"`
			SwapCount  int    `json:"swap_count"`
			OldestSwap string `json:"oldest_swap"`
		}
		payouts := []payoutEntry{}
		for rows.Next() {
			var p payoutEntry
			if err := rows.Scan(&p.UserID, &p.Name, &p.Email, &p.Phone, &p.TotalCents, &p.SwapCount, &p.OldestSwap); err != nil {
				log.Printf("[admin-payouts] scan error: %v", err)
				continue
			}
			payouts = append(payouts, p)
		}
		writeJSON(w, http.StatusOK, payouts)

	case http.MethodPost:
		var req struct {
			UserID string `json:"user_id"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.UserID == "" {
			writeBadRequest(w, "user_id is required")
			return
		}

		result, err := s.db.Exec(
			"UPDATE swap_rebates SET status = 'issued' WHERE user_id = ? AND status = 'pending'",
			req.UserID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update payouts"})
			return
		}
		affected, _ := result.RowsAffected()
		log.Printf("[admin-payouts] marked %d rebates as issued for user %s", affected, req.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "updated": affected})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// legalPageHTML wraps legal page content in a simple styled HTML shell.
func legalPageHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — Penny by Groflo</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Avenir','Avenir Next',-apple-system,'Segoe UI',sans-serif;color:#2A2A2A;background:#F7F6F3;line-height:1.6}
.wrap{max-width:640px;margin:0 auto;padding:40px 20px}
h1{font-size:24px;font-weight:700;margin-bottom:8px}
.updated{font-size:13px;color:#525252;margin-bottom:32px}
h2{font-size:18px;font-weight:600;margin:28px 0 8px}
p,ul{font-size:15px;margin-bottom:12px}
ul{padding-left:20px}
li{margin-bottom:6px}
a{color:#35644F}
.back{display:inline-block;margin-bottom:24px;font-size:14px;color:#35644F;text-decoration:none}
</style></head><body><div class="wrap">
<a class="back" href="/">← Back to Penny</a>
%s
</div></body></html>`, title, body)
}

func handlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, legalPageHTML("Privacy Policy", `
<h1>Privacy Policy</h1>
<p class="updated">Effective: April 7, 2026</p>

<p>Groflo, Inc. ("Groflo," "we," "us," or "our") makes a shared shopping list app you control by texting Penny, our AI assistant. This policy tells you what we collect, how we use it, and what you control. We wrote it in plain English and we mean it.</p>

<h2>1. Account and the Rally ecosystem</h2>
<p>Your Groflo account is part of the <strong>Rally ecosystem</strong>, a shared identity layer that lets one Rally login work across Groflo and other Rally apps. <strong>Your account is shared. Your shopping list is not.</strong> Other Rally apps cannot read your list, your dietary preferences, your household, or your messages with Penny.</p>
<p>When you create a Groflo account we record your phone number, optional email, optional display name, and the timestamp of your consent. If you sign in with Google, we also receive your Google email and name.</p>

<h2>2. SMS messages and your consent</h2>
<p>Penny is an SMS-native assistant. If you give us your phone number, you are choosing to receive text messages from Penny: replies to messages you send her, list updates from your household, and occasional reminders about items you have asked her to track.</p>
<p><strong>Message frequency varies</strong> based on your usage and your household's activity. <strong>Message and data rates may apply</strong> from your wireless carrier.</p>
<p>You can change how often Penny texts you in Settings. You can stop texts at any time by replying <strong>STOP</strong> to any Penny message. After you reply STOP we send one confirmation text and no more. Reply <strong>HELP</strong> to any Penny message for support.</p>
<p><strong>We will not share your mobile information with third parties or affiliates for marketing or promotional purposes.</strong> Information you share with Penny stays inside Groflo and the service providers listed in section 4.</p>

<h2>3. Information we collect</h2>
<ul>
<li><strong>Phone number</strong> — required for SMS, used to verify your account and to send and receive texts with Penny.</li>
<li><strong>Email and name</strong> — optional, used to recover access and to address you in messages.</li>
<li><strong>Shopping list data</strong> — items you add by app, text, or email, your edits, and your check-offs.</li>
<li><strong>Household relationships</strong> — who you share lists with.</li>
<li><strong>Brand preferences and dietary attributes</strong> — saved when you correct Penny or set them in Settings.</li>
<li><strong>Receipts you upload</strong> — to verify swap rebates. We extract the line items and store the parsed data; the original image is deleted within 30 days.</li>
<li><strong>Usage data</strong> — page views, feature usage, and error reports, used to keep Penny working.</li>
</ul>

<h2>4. Service providers</h2>
<p>We share the minimum data needed with the providers who run the service. Each provider sees only the slice of data they need.</p>
<ul>
<li><strong>Twilio</strong> — sends and receives your SMS messages, and sends one-time verification codes.</li>
<li><strong>Anthropic</strong> — runs the AI that turns your texts into list items. Your texts are sent for processing. Anthropic does not train its models on your data.</li>
<li><strong>Stripe</strong> — processes brand swap rebate payouts to you, when applicable. Stripe is also the payments backbone for the Rally ecosystem.</li>
<li><strong>Railway</strong> — hosts the Groflo backend.</li>
<li><strong>Google</strong> — only if you sign in with Google.</li>
<li><strong>SendGrid</strong> — sends transactional emails (password resets, magic links, optional digests).</li>
</ul>

<h2>5. How we use your data</h2>
<ul>
<li>To verify your identity and keep your account secure.</li>
<li>To send and receive SMS messages with Penny and your household.</li>
<li>To organize your list, learn your brand preferences, and respect your dietary attributes.</li>
<li>To pay you brand swap rebates you have earned.</li>
<li>To improve Penny — but only with anonymized, aggregated data, never your individual list.</li>
</ul>

<h2>6. Brand partners and rebates</h2>
<p>Groflo is free for shoppers because brands pay to suggest products on your list. Suggestions are always labeled. Brand partners <strong>never</strong> see your name, your phone number, your email, your address, or your individual list. They see aggregated, anonymized statistics about how often their products are chosen across the whole Groflo population.</p>

<h2>7. Data retention</h2>
<p>We keep your account data while your account is active. You can delete individual list items at any time in the app. You can delete your account by emailing <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>. After we receive your request we delete your personal data within 30 days, except records we are legally required to keep (for example, financial records of rebate payouts).</p>

<h2>8. Security</h2>
<p>We protect your data with encrypted connections (TLS), HttpOnly session cookies, hashed passwords (bcrypt), short-lived authentication codes, and industry-standard server hardening. We do not store payment card numbers; Stripe does.</p>

<h2>9. Your rights</h2>
<p>You have the right to access, correct, export, or delete your personal data at any time. California residents have additional rights under the CCPA, including the right not to be discriminated against for exercising those rights. We do not sell or share personal information as those terms are defined under the CCPA. To exercise any of these rights, email <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>.</p>

<h2>10. Children</h2>
<p>Groflo is not for children under 13 and we do not knowingly collect data from children under 13.</p>

<h2>11. Changes</h2>
<p>When we make material changes to this policy we will notify you by email or by an in-app notice before the changes take effect. The "Effective" date at the top of this page always reflects the current version.</p>

<h2>12. Contact</h2>
<p>Questions or requests: <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>.</p>
`))
}

func handleTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, legalPageHTML("Terms of Service", `
<h1>Terms of Service</h1>
<p class="updated">Effective: April 7, 2026</p>

<p>These terms cover your use of <strong>Groflo</strong>, the shared shopping list service operated by Groflo, Inc., and your conversations with <strong>Penny</strong>, our AI assistant. Read them. By using Groflo you accept them.</p>

<h2>1. The service</h2>
<p>Groflo lets you and your household share a single shopping list. You can add items by typing in the app, by texting Penny, or by emailing her. Penny uses AI to organize your list, learn your brand preferences, and surface savings.</p>

<h2>2. Your account and the Rally ecosystem</h2>
<p>Your Groflo account is part of the Rally ecosystem. One Rally login works across Groflo and other Rally apps. Your shopping list, household, dietary preferences, and messages with Penny are private to Groflo and are not shared with other Rally apps.</p>
<p>You are responsible for keeping your account credentials secure. Tell us right away if you think someone else has access to your account.</p>

<h2>3. SMS messaging program</h2>
<p>When you give Groflo your phone number you are enrolling in Penny's SMS program. By enrolling, you agree to receive recurring text messages from Penny: replies, list updates from your household, and reminders.</p>
<ul>
<li><strong>Message frequency varies</strong> based on your usage and your household's activity.</li>
<li><strong>Message and data rates may apply.</strong></li>
<li>To stop messages, reply <strong>STOP</strong> to any Penny message. To get help, reply <strong>HELP</strong> to any Penny message or email <a href="mailto:support@groflo.io">support@groflo.io</a>.</li>
<li>Carriers are not liable for delayed or undelivered messages.</li>
<li>You can change how often Penny texts you at any time in Settings.</li>
</ul>
<p>For details on what we collect and how we use it, see our <a href="/privacy">Privacy Policy</a>.</p>

<h2>4. Your content</h2>
<p>You own your shopping list, your household, and the messages you send Penny. You give Groflo a limited license to store and display that content as needed to run the service. We will never sell your individual content.</p>

<h2>5. Brand suggestions and swap rebates</h2>
<p>Groflo is free because brands pay to suggest products on your list. Brand suggestions are always labeled. You are never required to accept a suggestion.</p>
<p>When you accept a swap that includes a rebate, the rebate is paid to your in-app wallet. You can cash out your wallet balance through Stripe. Cash-outs are subject to Stripe's terms and may require identity verification. Rebate amounts and availability may change at any time.</p>

<h2>6. Acceptable use</h2>
<p>Use Groflo to manage your household's shopping. Do not use it to harass other users, to send unsolicited commercial messages, to scrape brand data, or to break the law. We may suspend accounts that abuse the service.</p>

<h2>7. Eligibility</h2>
<p>You must be at least 13 years old and have a valid US mobile phone number to use Groflo.</p>

<h2>8. Termination</h2>
<p>You can delete your account at any time by emailing <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>. We may suspend or terminate your account if you violate these terms or if we are required to by law. After termination, your data is deleted as described in the Privacy Policy.</p>

<h2>9. Disclaimers</h2>
<p>Groflo and Penny are provided "as is" and "as available." We do not guarantee that the service will be uninterrupted, error-free, or that brand suggestions will be accurate or available. Penny is an AI assistant and may make mistakes. Always check labels for allergens.</p>

<h2>10. Limitation of liability</h2>
<p>To the fullest extent permitted by law, Groflo, Inc., its officers, employees, and partners are not liable for any indirect, incidental, special, consequential, or punitive damages, or any loss of profits or revenues, arising from your use of the service. Our total liability for any claim is limited to one hundred US dollars ($100).</p>

<h2>11. Governing law and disputes</h2>
<p>These terms are governed by the laws of the State of Delaware, without regard to conflict-of-laws rules. Disputes will be resolved in the state or federal courts located in Delaware, except where you have a non-waivable right to bring claims in your local jurisdiction.</p>

<h2>12. Changes</h2>
<p>We may update these terms. When we do, we will post the updated terms here and update the Effective date. If the changes are material, we will notify you in the app or by email before they take effect. Continued use of Groflo after the changes take effect means you accept them.</p>

<h2>13. Contact</h2>
<p>Questions about these terms: <a href="mailto:legal@groflo.io">legal@groflo.io</a>. Support: <a href="mailto:support@groflo.io">support@groflo.io</a>.</p>
`))
}

// handleList routes requests under /api/list:
//
//	GET  /api/list              → list all items for the user
//	POST /api/list              → add a single item
//	POST /api/list/parse        → parse free-form text with Claude and bulk-add items
//	PATCH /api/list/{id}/toggle → toggle checked
//	DELETE /api/list/{id}       → remove an item
