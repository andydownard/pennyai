package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		log.Printf("[auth] no session cookie on %s %s: %v", r.Method, r.URL.Path, err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not logged in"})
		return Session{}, false
	}
	session, ok := s.sessions.GetSession(cookie.Value)
	if !ok {
		log.Printf("[auth] invalid/expired session on %s %s", r.Method, r.URL.Path)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Session expired"})
		return Session{}, false
	}
	return session, true
}

// requireAdmin checks for a valid session and that the phone is the admin phone.
// Returns the session and true if authorized, or writes an error and returns false.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return Session{}, false
	}
	if s.config.AdminPhone == "" || session.PhoneNumber != s.config.AdminPhone {
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
		SELECT COALESCE(u.phone_number,''), COALESCE(u.name,''), COALESCE(u.referral_points,0), COUNT(r.id) as ref_count
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
		SELECT n.score, n.comment, COALESCE(u.name,''), COALESCE(u.phone_number,''), n.created_at
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
			VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			deal.CompetitorProduct, deal.OfferBrand, deal.OfferProduct, deal.DiscountCents,
			deal.TotalBudgetCents, deal.Status, deal.EndsAt,
			deal.OfferPriceCents, deal.CompetitorPriceCents,
			deal.OfferSize, deal.CompetitorSize,
			deal.OfferUnitPrice, deal.CompetitorUnitPrice,
			deal.UnitLabel, deal.WhyBullets, deal.AllergenNotes,
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
			       discount_cents = ?, total_budget_cents = ?, status = ?, ends_at = NULLIF(?, ''),
			       offer_price_cents = ?, competitor_price_cents = ?,
			       offer_size = ?, competitor_size = ?,
			       offer_unit_price = ?, competitor_unit_price = ?,
			       unit_label = ?, why_bullets = ?, allergen_notes = ?,
			       deal_type = ?, quantity_offered = ?, trial_product_name = ?, trial_size = ?, campaign_tag = ?
			WHERE id = ?`,
			deal.CompetitorProduct, deal.OfferBrand, deal.OfferProduct,
			deal.DiscountCents, deal.TotalBudgetCents, deal.Status, deal.EndsAt,
			deal.OfferPriceCents, deal.CompetitorPriceCents,
			deal.OfferSize, deal.CompetitorSize,
			deal.OfferUnitPrice, deal.CompetitorUnitPrice,
			deal.UnitLabel, deal.WhyBullets, deal.AllergenNotes,
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
		adminIdentifier = fmt.Sprintf("user_%d", admin.UserID)
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
			SELECT u.id, COALESCE(u.name,''), COALESCE(u.email,''), COALESCE(u.phone_number,''),
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
			UserID     int    `json:"user_id"`
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
			UserID int `json:"user_id"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.UserID == 0 {
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
		log.Printf("[admin-payouts] marked %d rebates as issued for user %d", affected, req.UserID)
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "updated": affected})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// legalPageHTML wraps legal page content in a simple styled HTML shell.
func legalPageHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — Penny by Thriftly</title>
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
<p class="updated">Last updated: March 4, 2026</p>

<p>Penny by Thriftly ("Penny," "we," "us," or "our") operates the Penny shopping list application
and related SMS messaging service. This Privacy Policy describes what information we collect,
how we use it, and your choices.</p>

<h2>1. Information We Collect</h2>
<ul>
<li><strong>Phone number</strong> — provided when you sign up, used for account verification and to send/receive SMS messages.</li>
<li><strong>Name and email</strong> — optionally provided in your profile settings.</li>
<li><strong>Shopping list data</strong> — items you add via the app, text message, or email.</li>
<li><strong>Usage data</strong> — basic analytics such as page views and feature usage to improve the service.</li>
</ul>

<h2>2. How We Use Your Information</h2>
<ul>
<li>To verify your identity and manage your account.</li>
<li>To send and receive SMS messages related to your shopping list (e.g., adding items, notifications).</li>
<li>To operate, maintain, and improve the Penny service.</li>
<li>To respond to your support requests.</li>
</ul>

<h2>3. Information Sharing</h2>
<p>We do <strong>not</strong> sell, rent, or share your personal information with third parties for their
marketing purposes. We share data only in these limited circumstances:</p>
<ul>
<li><strong>Service providers</strong> — We use Twilio to deliver SMS messages and Anthropic to process natural-language list items. These providers access only the data necessary to perform their services.</li>
<li><strong>Household members</strong> — If you join a household, other members can see the shared shopping list and your display name.</li>
<li><strong>Anonymized &amp; aggregated data</strong> — We may share anonymized, aggregated shopping data with brands, retailers, and advertising partners to enable product rebates and improve the shopping experience. This data does not identify you personally — it contains no names, phone numbers, or other personal identifiers.</li>
<li><strong>Legal requirements</strong> — We may disclose information if required by law or to protect our rights.</li>
</ul>

<h2>4. Data Retention</h2>
<p>We retain your account data for as long as your account is active. You can delete your list data
at any time through the app. To delete your account entirely, contact us at the address below.</p>

<h2>5. Security</h2>
<p>We use industry-standard measures to protect your data, including encrypted connections (HTTPS),
HttpOnly session cookies, and secure credential storage.</p>

<h2>6. Children's Privacy</h2>
<p>Penny is not directed at children under 13 and we do not knowingly collect information from children under 13.</p>

<h2>7. Changes</h2>
<p>We may update this policy from time to time. We will notify you of material changes by posting the updated policy on this page.</p>

<h2>8. Contact Us</h2>
<p>If you have questions about this Privacy Policy, contact us at
<a href="mailto:privacy@thrift.ly">privacy@thrift.ly</a>.</p>
`))
}

func handleTerms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, legalPageHTML("Terms & Conditions", `
<h1>Terms &amp; Conditions</h1>
<p class="updated">Last updated: March 4, 2026</p>

<h2>1. Program Name</h2>
<p><strong>Penny by Thriftly</strong> — an AI-powered shared shopping list service.</p>

<h2>2. Service Description</h2>
<p>Penny lets you create and manage a shared grocery shopping list. You can add items by typing in the app,
sending a text message (SMS), or forwarding an email. Penny uses AI to parse your input into organized list items.</p>

<h2>3. Messaging Terms</h2>
<ul>
<li><strong>Message frequency:</strong> Message frequency varies based on your usage. You will receive SMS messages
when household members add items, and in response to messages you send to Penny.</li>
<li><strong>Message and data rates may apply.</strong> Your carrier's standard messaging and data rates apply to all
SMS messages sent to and received from Penny.</li>
</ul>

<h2>4. Opt-Out</h2>
<p>You can opt out of SMS messages at any time by texting <strong>STOP</strong> to the Penny phone number.
After opting out, you will receive one final confirmation message and no further SMS messages.</p>

<h2>5. Help</h2>
<p>For support, text <strong>HELP</strong> to the Penny phone number, or email us at
<a href="mailto:support@thrift.ly">support@thrift.ly</a>.</p>

<h2>6. Eligibility</h2>
<p>You must be at least 13 years old and have a valid US mobile phone number to use Penny.</p>

<h2>7. User Conduct</h2>
<p>You agree to use Penny only for lawful purposes related to managing shopping lists. You are
responsible for all activity under your account.</p>

<h2>8. Intellectual Property</h2>
<p>All content, branding, and technology associated with Penny are owned by Thriftly. You retain
ownership of your shopping list data.</p>

<h2>8a. Anonymized Data &amp; Brand Partnerships</h2>
<p>By using Penny, you agree that we may anonymize and aggregate your shopping list data and share
it with brand and retail partners. This data is stripped of all personally identifying information
(such as your name, phone number, and household details) and is used solely to enable product
rebates, improve product recommendations, and support market research. You will never be personally
identified to any partner.</p>

<h2>9. Disclaimer of Warranties</h2>
<p>Penny is provided "as is" without warranties of any kind. We do not guarantee uninterrupted
or error-free service.</p>

<h2>10. Limitation of Liability</h2>
<p>To the fullest extent permitted by law, Thriftly shall not be liable for any indirect, incidental,
or consequential damages arising from your use of Penny.</p>

<h2>11. Changes</h2>
<p>We may update these terms at any time. Continued use of Penny after changes constitutes acceptance
of the updated terms.</p>

<h2>12. Contact</h2>
<p>Questions? Email us at <a href="mailto:support@thrift.ly">support@thrift.ly</a>.</p>
`))
}

// handleList routes requests under /api/list:
//
//	GET  /api/list              → list all items for the user
//	POST /api/list              → add a single item
//	POST /api/list/parse        → parse free-form text with Claude and bulk-add items
//	PATCH /api/list/{id}/toggle → toggle checked
//	DELETE /api/list/{id}       → remove an item
