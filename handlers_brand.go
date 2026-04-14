package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)


// =============================================================================
// BRAND DEALS (CRUD for brand portal users)
// =============================================================================

func (s *Server) handleBrandDeals(w http.ResponseWriter, r *http.Request) {
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getBrandDeals(w, r, bs)
	case http.MethodPost:
		s.createBrandDeal(w, r, bs)
	default:
		// Check for deal ID in path for PUT/DELETE
		path := strings.TrimPrefix(r.URL.Path, "/brand/deals/")
		if path == "" || path == r.URL.Path {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		dealID, err := strconv.Atoi(path)
		if err != nil {
			writeBadRequest(w, "Invalid deal ID")
			return
		}
		switch r.Method {
		case http.MethodPut:
			s.updateBrandDeal(w, r, bs, dealID)
		case http.MethodDelete:
			s.deleteBrandDeal(w, bs, dealID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		}
	}
}

func (s *Server) getBrandDeals(w http.ResponseWriter, r *http.Request, bs BrandSession) {
	rows, err := s.db.Query(`
		SELECT id, competitor_product, offer_brand, offer_product, discount_cents,
			   total_budget_cents, budget_spent_cents, status, COALESCE(starts_at,''), COALESCE(ends_at,''),
			   offer_price_cents, offer_size, offer_unit_price,
			   competitor_price_cents, competitor_size, competitor_unit_price,
			   unit_label, why_bullets, allergen_notes,
			   deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag,
			   created_at,
			   COALESCE(review_status,''), COALESCE(review_note,'')
		FROM brand_deals WHERE brand_user_id = ? AND status != 'deleted'
		ORDER BY created_at DESC`, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to load deals")
		return
	}
	defer rows.Close()

	type dealWithMetrics struct {
		BrandDeal
		Offered    int `json:"offered"`
		Accepted   int `json:"accepted"`
		Declined   int `json:"declined"`
		AcceptRate int `json:"accept_rate"`
	}
	deals := []dealWithMetrics{}
	for rows.Next() {
		var d BrandDeal
		if err := rows.Scan(
			&d.ID, &d.CompetitorProduct, &d.OfferBrand, &d.OfferProduct, &d.DiscountCents,
			&d.TotalBudgetCents, &d.BudgetSpentCents, &d.Status, &d.StartsAt, &d.EndsAt,
			&d.OfferPriceCents, &d.OfferSize, &d.OfferUnitPrice,
			&d.CompetitorPriceCents, &d.CompetitorSize, &d.CompetitorUnitPrice,
			&d.UnitLabel, &d.WhyBullets, &d.AllergenNotes,
			&d.DealType, &d.QuantityOffered, &d.TrialProductName, &d.TrialSize, &d.CampaignTag,
			&d.CreatedAt,
			&d.ReviewStatus, &d.ReviewNote,
		); err != nil {
			dbError(w, err, "Failed to scan deal")
			return
		}
		deals = append(deals, dealWithMetrics{BrandDeal: d})
	}
	rows.Close()

	// Enrich with swap event metrics — single query instead of 3N queries
	if len(deals) > 0 {
		dealIDs := make([]interface{}, len(deals))
		placeholders := make([]string, len(deals))
		dealIdx := map[int]int{} // deal_id -> index in deals slice
		for i, dm := range deals {
			dealIDs[i] = dm.ID
			placeholders[i] = "?"
			dealIdx[dm.ID] = i
		}
		swapRows, err := s.db.Query(
			"SELECT deal_id, action, COUNT(*) FROM swap_events WHERE deal_id IN ("+strings.Join(placeholders, ",")+") GROUP BY deal_id, action",
			dealIDs...,
		)
		if err == nil {
			defer swapRows.Close()
			for swapRows.Next() {
				var dID, cnt int
				var action string
				if swapRows.Scan(&dID, &action, &cnt) == nil {
					if idx, ok := dealIdx[dID]; ok {
						switch action {
						case "offered":
							deals[idx].Offered = cnt
						case "accepted":
							deals[idx].Accepted = cnt
						case "declined":
							deals[idx].Declined = cnt
						}
					}
				}
			}
		}
		for i := range deals {
			if deals[i].Offered > 0 {
				deals[i].AcceptRate = (deals[i].Accepted * 100) / deals[i].Offered
			}
		}
	}

	writeJSON(w, http.StatusOK, deals)
}

func (s *Server) createBrandDeal(w http.ResponseWriter, r *http.Request, bs BrandSession) {
	var req struct {
		CompetitorProduct    string `json:"competitor_product"`
		OfferBrand           string `json:"offer_brand"`
		OfferProduct         string `json:"offer_product"`
		DiscountCents        int    `json:"discount_cents"`
		TotalBudgetCents     int    `json:"daily_budget_cents"`
		OfferPriceCents      int    `json:"offer_price_cents"`
		CompetitorPriceCents int    `json:"competitor_price_cents"`
		OfferSize            string `json:"offer_size"`
		CompetitorSize       string `json:"competitor_size"`
		OfferUnitPrice       string `json:"offer_unit_price"`
		CompetitorUnitPrice  string `json:"competitor_unit_price"`
		UnitLabel            string `json:"unit_label"`
		WhyBullets           string `json:"why_bullets"`
		AllergenNotes        string `json:"allergen_notes"`
		DealType             string `json:"deal_type"`
		QuantityOffered      int    `json:"quantity_offered"`
		TrialProductName     string `json:"trial_product_name"`
		TrialSize            string `json:"trial_size"`
		CampaignTag          string `json:"campaign_tag"`
		ProductDescription   string `json:"product_description"`
		KeyBenefits          string `json:"key_benefits"`
		BrandStory           string `json:"brand_story"`
		// Fully Free Trial fields
		MaxRebateCents         int    `json:"max_rebate_cents"`
		PlatformFeeCents       int    `json:"platform_fee_cents"`
		RequireVerifiedIdentity *int  `json:"require_verified_identity"`
		ReceiptGraceDays       int    `json:"receipt_grace_days"`
		UPC                    string `json:"upc"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.CompetitorProduct == "" || req.OfferBrand == "" || req.OfferProduct == "" {
		writeBadRequest(w, "competitor_product, offer_brand, and offer_product are required")
		return
	}

	// Fully free validation
	if req.DealType == "fully_free" {
		if req.MaxRebateCents <= 0 {
			writeBadRequest(w, "max_rebate_cents is required for fully_free deals")
			return
		}
		if req.UPC == "" {
			writeBadRequest(w, "upc is required for fully_free deals")
			return
		}
		// Default platform fee to $1 (100 cents)
		if req.PlatformFeeCents <= 0 {
			req.PlatformFeeCents = 100
		}
		if req.ReceiptGraceDays <= 0 {
			req.ReceiptGraceDays = 7
		}
		// For fully_free, discount_cents = max_rebate_cents (used for display)
		if req.DiscountCents == 0 {
			req.DiscountCents = req.MaxRebateCents
		}
	}

	// Check balance — brand must have enough to cover at least one swap
	var balanceCents int
	s.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", bs.BrandUserID).Scan(&balanceCents)
	if balanceCents < req.DiscountCents && req.TotalBudgetCents > 0 {
		// Allow creating deals even without balance (they'll auto-pause at $0)
	}

	if req.DealType == "" {
		req.DealType = "sku_swap"
	}
	if req.QuantityOffered == 0 {
		req.QuantityOffered = 1
	}

	// Auto-categorize the deal based on product name
	categoryID := categorizeProduct(s.db, req.OfferProduct)
	if categoryID == 0 {
		categoryID = categorizeProduct(s.db, req.CompetitorProduct)
	}

	// Enforce category-tiered minimum pricing
	minCents, _, tierLabel := getCategoryPricingTier(s.db, categoryID)
	if req.DiscountCents < minCents {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":            fmt.Sprintf("Minimum discount for %s tier categories is $%.2f", tierLabel, float64(minCents)/100),
			"min_discount_cents": minCents,
			"tier":             tierLabel,
			"category_id":     categoryID,
		})
		return
	}

	// Resolve require_verified_identity: default to 1 if not specified
	requireVerified := 1
	if req.RequireVerifiedIdentity != nil {
		requireVerified = *req.RequireVerifiedIdentity
	}

	result, err := s.db.Exec(`
		INSERT INTO brand_deals (
			competitor_product, offer_brand, offer_product, discount_cents,
			total_budget_cents, status, brand_user_id,
			offer_price_cents, competitor_price_cents,
			offer_size, competitor_size, offer_unit_price, competitor_unit_price,
			unit_label, why_bullets, allergen_notes,
			deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag,
			category_id, product_description, key_benefits, brand_story,
			max_rebate_cents, platform_fee_cents, require_verified_identity,
			receipt_grace_days, upc,
			review_status, submitted_at
		) VALUES (?, ?, ?, ?, ?, 'pending_review', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)`,
		req.CompetitorProduct, req.OfferBrand, req.OfferProduct, req.DiscountCents,
		req.TotalBudgetCents, bs.BrandUserID,
		req.OfferPriceCents, req.CompetitorPriceCents,
		req.OfferSize, req.CompetitorSize, req.OfferUnitPrice, req.CompetitorUnitPrice,
		req.UnitLabel, req.WhyBullets, req.AllergenNotes,
		req.DealType, req.QuantityOffered, req.TrialProductName, req.TrialSize, req.CampaignTag,
		categoryID, req.ProductDescription, req.KeyBenefits, req.BrandStory,
		req.MaxRebateCents, req.PlatformFeeCents, requireVerified,
		req.ReceiptGraceDays, req.UPC,
	)
	if err != nil {
		dbError(w, err, "Failed to create deal")
		return
	}

	dealID, _ := result.LastInsertId()
	log.Printf("[brand-portal] brand user %s created deal %d (pending review)", bs.BrandUserID, dealID)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      dealID,
		"status":  "pending_review",
		"message": "Deal created and submitted for review. It will go live once approved.",
	})
}

func (s *Server) updateBrandDeal(w http.ResponseWriter, r *http.Request, bs BrandSession, dealID int) {
	// Verify ownership
	var ownerID sql.NullString
	err := s.db.QueryRow("SELECT brand_user_id FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil || !ownerID.Valid || ownerID.String != bs.BrandUserID {
		writeNotFound(w, "Deal not found")
		return
	}

	var req struct {
		Status           *string `json:"status"`
		DiscountCents    *int    `json:"discount_cents"`
		TotalBudgetCents *int    `json:"daily_budget_cents"`
		WhyBullets       *string `json:"why_bullets"`
		AllergenNotes    *string `json:"allergen_notes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	sets := []string{}
	args := []interface{}{}
	if req.Status != nil {
		// Brands cannot self-approve: only admin review can set status to 'active'
		if *req.Status == "active" {
			// Check current review_status — only allow resume if already approved
			var reviewStatus string
			s.db.QueryRow("SELECT COALESCE(review_status,'') FROM brand_deals WHERE id = ?", dealID).Scan(&reviewStatus)
			if reviewStatus != "approved" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "Deal must be approved by admin before it can go live"})
				return
			}
		}
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
	}
	if req.DiscountCents != nil {
		sets = append(sets, "discount_cents = ?")
		args = append(args, *req.DiscountCents)
	}
	if req.TotalBudgetCents != nil {
		sets = append(sets, "total_budget_cents = ?")
		args = append(args, *req.TotalBudgetCents)
	}
	if req.WhyBullets != nil {
		sets = append(sets, "why_bullets = ?")
		args = append(args, *req.WhyBullets)
	}
	if req.AllergenNotes != nil {
		sets = append(sets, "allergen_notes = ?")
		args = append(args, *req.AllergenNotes)
	}

	if len(sets) == 0 {
		writeBadRequest(w, "No fields to update")
		return
	}

	args = append(args, dealID)
	_, err = s.db.Exec(fmt.Sprintf("UPDATE brand_deals SET %s WHERE id = ?", strings.Join(sets, ", ")), args...)
	if err != nil {
		dbError(w, err, "Failed to update deal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Deal updated"})
}

func (s *Server) deleteBrandDeal(w http.ResponseWriter, bs BrandSession, dealID int) {
	var ownerID sql.NullString
	err := s.db.QueryRow("SELECT brand_user_id FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil || !ownerID.Valid || ownerID.String != bs.BrandUserID {
		writeNotFound(w, "Deal not found")
		return
	}

	// Soft-delete (preserve FK references from swap_events)
	s.db.Exec("UPDATE brand_deals SET status = 'deleted' WHERE id = ?", dealID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Deal deleted"})
}

// =============================================================================
// BRAND DASHBOARD
// =============================================================================

func (s *Server) handleBrandDashboard(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	// Optional date range filter: ?days=7 (default: all time)
	daysParam := r.URL.Query().Get("days")
	days := 0
	if daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
			days = d
		}
	}
	dateFilter := ""
	if days > 0 {
		dateFilter = fmt.Sprintf(" AND se.created_at >= datetime('now', '-%d days')", days)
	}

	var totalDeals, activeDeals int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE brand_user_id = ? AND status != 'deleted'", bs.BrandUserID).Scan(&totalDeals)
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE brand_user_id = ? AND status = 'active'", bs.BrandUserID).Scan(&activeDeals)

	var totalSpentCents int
	s.db.QueryRow("SELECT COALESCE(SUM(budget_spent_cents), 0) FROM brand_deals WHERE brand_user_id = ?", bs.BrandUserID).Scan(&totalSpentCents)

	var totalSwaps, totalAccepted, totalDeclined int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'offered'`+dateFilter, bs.BrandUserID).Scan(&totalSwaps)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'accepted'`+dateFilter, bs.BrandUserID).Scan(&totalAccepted)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'declined'`+dateFilter, bs.BrandUserID).Scan(&totalDeclined)

	acceptRate := 0
	if totalSwaps > 0 {
		acceptRate = (totalAccepted * 100) / totalSwaps
	}

	var balanceCents int
	s.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", bs.BrandUserID).Scan(&balanceCents)

	// Swaps per day (default last 30 days, respects ?days= filter)
	chartDays := 30
	if days > 0 {
		chartDays = days
	}
	type dayData struct {
		Date     string `json:"date"`
		Offered  int    `json:"offered"`
		Accepted int    `json:"accepted"`
	}
	dailyRows, _ := s.db.Query(fmt.Sprintf(`
		SELECT DATE(se.created_at) as day, se.action, COUNT(*) as cnt
		FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.created_at > datetime('now', '-%d days')
		  AND se.action IN ('offered', 'accepted')
		GROUP BY day, se.action
		ORDER BY day`, chartDays), bs.BrandUserID)
	dailyMap := map[string]*dayData{}
	if dailyRows != nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var day, action string
			var cnt int
			dailyRows.Scan(&day, &action, &cnt)
			if _, ok := dailyMap[day]; !ok {
				dailyMap[day] = &dayData{Date: day}
			}
			if action == "offered" {
				dailyMap[day].Offered = cnt
			} else {
				dailyMap[day].Accepted = cnt
			}
		}
	}
	daily := []dayData{}
	for _, v := range dailyMap {
		daily = append(daily, *v)
	}

	// Purchase verification funnel (M9)
	claimDateFilter := ""
	if days > 0 {
		claimDateFilter = fmt.Sprintf(" AND sc.created_at >= datetime('now', '-%d days')", days)
	}
	var verifiedPurchases int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_claims sc
		JOIN swap_events se ON sc.swap_event_id = se.id
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND sc.status = 'verified'`+claimDateFilter, bs.BrandUserID).Scan(&verifiedPurchases)

	var pendingClaims int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_claims sc
		JOIN swap_events se ON sc.swap_event_id = se.id
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND sc.status = 'pending'`+claimDateFilter, bs.BrandUserID).Scan(&pendingClaims)

	purchaseRate := 0
	if totalAccepted > 0 {
		purchaseRate = (verifiedPurchases * 100) / totalAccepted
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_deals":         totalDeals,
		"active_deals":        activeDeals,
		"total_swaps":         totalSwaps,
		"total_accepted":      totalAccepted,
		"total_declined":      totalDeclined,
		"accept_rate":         acceptRate,
		"total_spent_cents":   totalSpentCents,
		"balance_cents":       balanceCents,
		"daily":               daily,
		"verified_purchases":  verifiedPurchases,
		"pending_claims":      pendingClaims,
		"purchase_rate":       purchaseRate,
	})
}

// =============================================================================
// COMPETITIVE SWAP INTEL
// =============================================================================

func (s *Server) handleCategoryIntel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	_, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	category := strings.TrimPrefix(r.URL.Path, "/brand/category-intel/")
	if category == "" {
		writeBadRequest(w, "Category required")
		return
	}

	// Find deals in this category (by competitor_product containing category keyword)
	rows, err := s.db.Query(`
		SELECT bd.competitor_product, bd.offer_brand, bd.offer_product,
			   COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			   COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered
		FROM brand_deals bd
		LEFT JOIN swap_events se ON se.deal_id = bd.id
		WHERE LOWER(bd.competitor_product) LIKE ? AND bd.status != 'deleted'
		GROUP BY bd.id
		ORDER BY accepted DESC`, "%"+strings.ToLower(category)+"%")
	if err != nil {
		dbError(w, err, "Failed to query category intel")
		return
	}
	defer rows.Close()

	type intelRow struct {
		CompetitorProduct string `json:"competitor_product"`
		OfferBrand        string `json:"offer_brand"`
		OfferProduct      string `json:"offer_product"`
		Accepted          int    `json:"accepted"`
		Offered           int    `json:"offered"`
	}
	results := []intelRow{}
	for rows.Next() {
		var r intelRow
		rows.Scan(&r.CompetitorProduct, &r.OfferBrand, &r.OfferProduct, &r.Accepted, &r.Offered)
		results = append(results, r)
	}

	// Aggregate: total swap-outs, top replacements
	totalSwapOuts := 0
	brandCounts := map[string]int{}
	for _, r := range results {
		totalSwapOuts += r.Accepted
		brandCounts[r.OfferBrand] += r.Accepted
	}

	type topBrand struct {
		Brand string `json:"brand"`
		Count int    `json:"count"`
	}
	topBrands := []topBrand{}
	for brand, count := range brandCounts {
		topBrands = append(topBrands, topBrand{Brand: brand, Count: count})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"category":        category,
		"total_swap_outs": totalSwapOuts,
		"top_brands":      topBrands,
		"details":         results,
	})
}


// =============================================================================
// BUG REPORTS (consumer app endpoint)
// =============================================================================

func (s *Server) handleBugReport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Description = strings.TrimSpace(req.Description)
	if req.Description == "" {
		writeBadRequest(w, "Description is required")
		return
	}
	if len(req.Description) > 5000 {
		writeBadRequest(w, "Description too long (max 5000 characters)")
		return
	}

	_, err := s.db.Exec("INSERT INTO bug_reports (user_id, description) VALUES (?, ?)", session.UserID, req.Description)
	if err != nil {
		dbError(w, err, "Failed to submit report")
		return
	}

	log.Printf("[bug-report] user %s submitted bug report", session.UserID)
	writeJSON(w, http.StatusCreated, map[string]string{"message": "Thank you! Your report has been submitted."})
}


// =============================================================================
// OPTIMIZE THIS DEAL — AI-powered recommendations
// =============================================================================

// handleOptimizeDeal handles GET /brand/optimize/{id}
// Returns actionable recommendations for improving a deal's performance.
// Requires brand session auth and deal ownership.
func (s *Server) handleOptimizeDeal(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/brand/optimize/")
	dealID, err := strconv.Atoi(idStr)
	if err != nil || dealID <= 0 {
		writeBadRequest(w, "Invalid deal ID")
		return
	}

	// Verify ownership and load deal details
	var competitorProduct, offerBrand, offerProduct, status, whyBullets, allergenNotes, dealType string
	var discountCents, budgetCents, spentCents, offerPriceCents, competitorPriceCents int
	err = s.db.QueryRow(`
		SELECT competitor_product, offer_brand, offer_product, status, discount_cents,
			total_budget_cents, budget_spent_cents, offer_price_cents, competitor_price_cents,
			COALESCE(why_bullets, '[]'), COALESCE(allergen_notes, ''), COALESCE(deal_type, 'sku_swap')
		FROM brand_deals WHERE id = ? AND brand_user_id = ? AND status != 'deleted'`,
		dealID, bs.BrandUserID).Scan(
		&competitorProduct, &offerBrand, &offerProduct, &status, &discountCents,
		&budgetCents, &spentCents, &offerPriceCents, &competitorPriceCents,
		&whyBullets, &allergenNotes, &dealType)
	if err != nil {
		writeNotFound(w, "Deal not found")
		return
	}

	// Collect performance metrics — single query
	var offered, accepted, declined int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN action='offered' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='accepted' THEN 1 END), 0),
		       COALESCE(SUM(CASE WHEN action='declined' THEN 1 END), 0)
		FROM swap_events WHERE deal_id = ?`, dealID).Scan(&offered, &accepted, &declined)

	// Top decline reasons
	type reasonCount struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}
	var declineReasons []reasonCount
	dRows, dErr := s.db.Query(`
		SELECT COALESCE(NULLIF(decline_reason, ''), 'unspecified') as reason, COUNT(*) as cnt
		FROM swap_events
		WHERE deal_id = ? AND action = 'declined'
		GROUP BY reason ORDER BY cnt DESC LIMIT 5`, dealID)
	if dErr == nil {
		for dRows.Next() {
			var rc reasonCount
			dRows.Scan(&rc.Reason, &rc.Count)
			declineReasons = append(declineReasons, rc)
		}
		dRows.Close()
	}

	// Category benchmark — how does this deal compare to others in same category?
	var categoryAvgAcceptRate float64
	s.db.QueryRow(`
		SELECT CASE WHEN COALESCE(SUM(CASE WHEN se.action = 'offered' THEN 1 ELSE 0 END), 0) > 0
			THEN CAST(COALESCE(SUM(CASE WHEN se.action = 'accepted' THEN 1 ELSE 0 END), 0) AS REAL) /
				 COALESCE(SUM(CASE WHEN se.action = 'offered' THEN 1 ELSE 0 END), 0) * 100
			ELSE 0 END
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE LOWER(bd.competitor_product) = LOWER(?) AND bd.status != 'deleted'`,
		competitorProduct).Scan(&categoryAvgAcceptRate)

	// Generate recommendations
	acceptRate := 0.0
	if offered > 0 {
		acceptRate = float64(accepted) / float64(offered) * 100
	}
	budgetUtilization := 0.0
	if budgetCents > 0 {
		budgetUtilization = float64(spentCents) / float64(budgetCents) * 100
	}

	type recommendation struct {
		Priority    string `json:"priority"` // high, medium, low
		Category    string `json:"category"` // discount, messaging, targeting, budget, timing
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	var recs []recommendation

	// Rule 1: Low acceptance rate
	if offered >= 10 && acceptRate < 20 {
		recs = append(recs, recommendation{
			Priority:    "high",
			Category:    "discount",
			Title:       "Increase discount to boost acceptance",
			Description: fmt.Sprintf("Your acceptance rate is %.0f%%, well below the 30%% target. Consider increasing the discount from $%.2f to $%.2f to make the swap more compelling.", acceptRate, float64(discountCents)/100, float64(discountCents)*1.5/100),
		})
	} else if offered >= 10 && acceptRate < 35 {
		recs = append(recs, recommendation{
			Priority:    "medium",
			Category:    "discount",
			Title:       "Fine-tune your discount",
			Description: fmt.Sprintf("Your acceptance rate is %.0f%%. A small discount increase could push it above 35%% and improve ROI.", acceptRate),
		})
	}

	// Rule 2: Price comparison disadvantage
	if offerPriceCents > 0 && competitorPriceCents > 0 && offerPriceCents > competitorPriceCents {
		diff := offerPriceCents - competitorPriceCents
		if diff > discountCents {
			recs = append(recs, recommendation{
				Priority:    "high",
				Category:    "discount",
				Title:       "Discount doesn't cover price difference",
				Description: fmt.Sprintf("Your product is $%.2f more expensive than the competitor, but the discount is only $%.2f. Users still pay more after the swap — increase discount to at least cover the gap.", float64(diff)/100, float64(discountCents)/100),
			})
		}
	}

	// Rule 3: Top decline reason analysis
	if len(declineReasons) > 0 && declineReasons[0].Count >= 3 {
		topReason := declineReasons[0].Reason
		switch topReason {
		case "price":
			recs = append(recs, recommendation{
				Priority:    "high",
				Category:    "discount",
				Title:       "Users cite price as top decline reason",
				Description: "The most common decline reason is price sensitivity. Consider a larger discount or a BOGO offer to overcome price resistance.",
			})
		case "brand_loyal":
			recs = append(recs, recommendation{
				Priority:    "medium",
				Category:    "messaging",
				Title:       "Brand loyalty is your biggest competitor",
				Description: "Users decline because they're loyal to their current brand. Strengthen your 'why switch' bullets with taste tests, awards, or unique ingredients.",
			})
		case "dietary":
			recs = append(recs, recommendation{
				Priority:    "high",
				Category:    "targeting",
				Title:       "Dietary concerns causing declines",
				Description: "Users with dietary restrictions are being shown this deal. Add allergen notes to improve targeting and avoid showing the deal to incompatible users.",
			})
		case "tried_before":
			recs = append(recs, recommendation{
				Priority:    "low",
				Category:    "targeting",
				Title:       "Users already tried your product",
				Description: "Some users decline because they've tried this product before. Consider a different product variant or a loyalty-based offer for repeat purchases.",
			})
		}
	}

	// Rule 4: Missing why_bullets
	if whyBullets == "" || whyBullets == "[]" {
		recs = append(recs, recommendation{
			Priority:    "medium",
			Category:    "messaging",
			Title:       "Add 'why switch' reasons",
			Description: "Deals with compelling reasons to switch see 40% higher acceptance. Add 2-3 bullet points: price advantage, ratings, unique features.",
		})
	}

	// Rule 5: Missing allergen info
	if allergenNotes == "" && dealType == "sku_swap" {
		recs = append(recs, recommendation{
			Priority:    "low",
			Category:    "targeting",
			Title:       "Add allergen information",
			Description: "Adding allergen notes helps Penny avoid showing your deal to users with dietary restrictions, reducing wasted impressions and improving acceptance rate.",
		})
	}

	// Rule 6: Budget nearly exhausted
	if budgetUtilization > 80 && status == "active" {
		recs = append(recs, recommendation{
			Priority:    "medium",
			Category:    "budget",
			Title:       "Budget almost exhausted",
			Description: fmt.Sprintf("You've used %.0f%% of your budget. Consider topping up to maintain momentum — paused deals lose ranking.", budgetUtilization),
		})
	}

	// Rule 7: Below category average
	if offered >= 10 && categoryAvgAcceptRate > 0 && acceptRate < categoryAvgAcceptRate-10 {
		recs = append(recs, recommendation{
			Priority:    "medium",
			Category:    "targeting",
			Title:       "Below category average",
			Description: fmt.Sprintf("Your deal's acceptance rate (%.0f%%) is below the category average (%.0f%%). Review competitor deals in this category for positioning insights.", acceptRate, categoryAvgAcceptRate),
		})
	}

	// Rule 8: Low impression volume
	if offered < 10 {
		recs = append(recs, recommendation{
			Priority:    "low",
			Category:    "budget",
			Title:       "Not enough data yet",
			Description: fmt.Sprintf("Only %d impressions so far. Most recommendations need 10+ impressions to be meaningful. Consider increasing budget to gather more data.", offered),
		})
	}

	if recs == nil {
		recs = []recommendation{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deal_id":       dealID,
		"offer_product": offerProduct,
		"offer_brand":   offerBrand,
		"metrics": map[string]interface{}{
			"offered":              offered,
			"accepted":             accepted,
			"declined":             declined,
			"accept_rate":          acceptRate,
			"budget_utilization":   budgetUtilization,
			"category_avg_rate":    categoryAvgAcceptRate,
		},
		"decline_reasons":    declineReasons,
		"recommendations":    recs,
		"recommendation_count": len(recs),
	})
}


// handlePricingTiers returns category pricing tiers so brands know the minimum
// discount required per category. GET /brand/pricing-tiers — brand session required.
func (s *Server) handlePricingTiers(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireBrandSession(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT parent_category, min_price_cents, suggested_cents, tier_label
		FROM category_pricing_tiers
		ORDER BY min_price_cents DESC, parent_category
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load pricing tiers"})
		return
	}

	type pricingTier struct {
		ParentCategory string `json:"parent_category"`
		MinPriceCents  int    `json:"min_price_cents"`
		SuggestedCents int    `json:"suggested_cents"`
		TierLabel      string `json:"tier_label"`
	}
	var tiers []pricingTier
	for rows.Next() {
		var t pricingTier
		if err := rows.Scan(&t.ParentCategory, &t.MinPriceCents, &t.SuggestedCents, &t.TierLabel); err != nil {
			continue
		}
		tiers = append(tiers, t)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, tiers)
}

// =============================================================================
// COMPETITIVE SWAP INTEL
// =============================================================================

func (s *Server) handleCompetitiveIntel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	// Step 1: Collect all active deals for this brand (collect-then-write for SQLite)
	type dealRow struct {
		DealID       int
		OfferProduct string
		CategoryID   int
	}
	rows, err := s.db.Query(`
		SELECT id, offer_product, category_id
		FROM brand_deals
		WHERE brand_user_id = ? AND status != 'deleted'
		ORDER BY id`, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to query brand deals")
		return
	}
	var deals []dealRow
	for rows.Next() {
		var d dealRow
		rows.Scan(&d.DealID, &d.OfferProduct, &d.CategoryID)
		deals = append(deals, d)
	}
	rows.Close()

	if len(deals) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deals":            []interface{}{},
			"category_summary": map[string]interface{}{},
		})
		return
	}

	// Step 2: For each deal, get swap event counts (offered/accepted)
	type dealIntel struct {
		DealID             int     `json:"deal_id"`
		OfferProduct       string  `json:"offer_product"`
		CategoryName       string  `json:"category_name"`
		ParentCategory     string  `json:"parent_category"`
		YourAcceptanceRate float64 `json:"your_acceptance_rate"`
		CategoryAvgRate    float64 `json:"category_avg_rate"`
		CompetingDeals     int     `json:"competing_deals"`
		CategoryRank       int     `json:"category_rank"`
		TotalOffered       int     `json:"total_offered"`
		TotalAccepted      int     `json:"total_accepted"`
	}

	// Build a set of category IDs we care about
	catIDs := map[int]bool{}
	for _, d := range deals {
		if d.CategoryID > 0 {
			catIDs[d.CategoryID] = true
		}
	}

	// Step 3: Load category names and parent categories
	catNames := map[int]string{}
	catParents := map[int]string{}
	for catID := range catIDs {
		var name, parent string
		err := s.db.QueryRow("SELECT name, parent_category FROM categories WHERE id = ?", catID).Scan(&name, &parent)
		if err == nil {
			catNames[catID] = name
			catParents[catID] = parent
		}
	}

	// Step 4: For each category, get the count of competing deals and aggregate acceptance rate
	type catStats struct {
		TotalDeals     int
		TotalOffered   int
		TotalAccepted  int
		YourDeals      int
	}
	catStatsMap := map[int]*catStats{}

	for catID := range catIDs {
		var totalDeals, totalOffered, totalAccepted int
		s.db.QueryRow(`
			SELECT COUNT(DISTINCT bd.id)
			FROM brand_deals bd
			WHERE bd.category_id = ? AND bd.status != 'deleted'`, catID).Scan(&totalDeals)
		s.db.QueryRow(`
			SELECT COUNT(CASE WHEN se.action = 'offered' THEN 1 END),
			       COUNT(CASE WHEN se.action = 'accepted' THEN 1 END)
			FROM swap_events se
			JOIN brand_deals bd ON bd.id = se.deal_id
			WHERE bd.category_id = ? AND bd.status != 'deleted'`, catID).Scan(&totalOffered, &totalAccepted)
		catStatsMap[catID] = &catStats{
			TotalDeals:    totalDeals,
			TotalOffered:  totalOffered,
			TotalAccepted: totalAccepted,
		}
	}

	// Step 5: Build results for each deal
	var results []dealIntel
	// Track per-category rank: collect all deal acceptance rates per category
	type dealRate struct {
		DealID int
		Rate   float64
	}
	catDealRates := map[int][]dealRate{}

	// Pre-compute acceptance rates for all deals in our categories
	for catID := range catIDs {
		rateRows, err := s.db.Query(`
			SELECT bd.id,
				COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered,
				COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted
			FROM brand_deals bd
			LEFT JOIN swap_events se ON se.deal_id = bd.id
			WHERE bd.category_id = ? AND bd.status != 'deleted'
			GROUP BY bd.id`, catID)
		if err != nil {
			continue
		}
		for rateRows.Next() {
			var id, offered, accepted int
			rateRows.Scan(&id, &offered, &accepted)
			rate := 0.0
			if offered > 0 {
				rate = float64(accepted) / float64(offered)
			}
			catDealRates[catID] = append(catDealRates[catID], dealRate{DealID: id, Rate: rate})
		}
		rateRows.Close()
	}

	for _, d := range deals {
		var offered, accepted int
		s.db.QueryRow(`
			SELECT COUNT(CASE WHEN action = 'offered' THEN 1 END),
			       COUNT(CASE WHEN action = 'accepted' THEN 1 END)
			FROM swap_events WHERE deal_id = ?`, d.DealID).Scan(&offered, &accepted)

		yourRate := 0.0
		if offered > 0 {
			yourRate = float64(accepted) / float64(offered)
		}

		catName := catNames[d.CategoryID]
		parentCat := catParents[d.CategoryID]
		catAvg := 0.0
		competingDeals := 0
		rank := 1

		if cs, ok := catStatsMap[d.CategoryID]; ok {
			if cs.TotalOffered > 0 {
				catAvg = float64(cs.TotalAccepted) / float64(cs.TotalOffered)
			}
			competingDeals = cs.TotalDeals - 1 // exclude self
			if competingDeals < 0 {
				competingDeals = 0
			}
			cs.YourDeals++
		}

		// Compute rank within category (1 = best)
		if rates, ok := catDealRates[d.CategoryID]; ok {
			rank = 1
			for _, dr := range rates {
				if dr.DealID != d.DealID && dr.Rate > yourRate {
					rank++
				}
			}
		}

		results = append(results, dealIntel{
			DealID:             d.DealID,
			OfferProduct:       d.OfferProduct,
			CategoryName:       catName,
			ParentCategory:     parentCat,
			YourAcceptanceRate: yourRate,
			CategoryAvgRate:    catAvg,
			CompetingDeals:     competingDeals,
			CategoryRank:       rank,
			TotalOffered:       offered,
			TotalAccepted:      accepted,
		})
	}

	// Step 6: Build category summary keyed by parent_category
	type catSummary struct {
		TotalDeals       int     `json:"total_deals"`
		AvgAcceptanceRate float64 `json:"avg_acceptance_rate"`
		YourDeals        int     `json:"your_deals"`
	}
	categorySummary := map[string]*catSummary{}
	for catID, cs := range catStatsMap {
		parent := catParents[catID]
		if parent == "" {
			parent = catNames[catID]
		}
		if parent == "" {
			continue
		}
		if _, exists := categorySummary[parent]; !exists {
			categorySummary[parent] = &catSummary{}
		}
		categorySummary[parent].TotalDeals += cs.TotalDeals
		categorySummary[parent].YourDeals += cs.YourDeals
		if cs.TotalOffered > 0 {
			// Weighted average: accumulate and average across categories with same parent
			existingRate := categorySummary[parent].AvgAcceptanceRate
			if existingRate == 0 {
				categorySummary[parent].AvgAcceptanceRate = float64(cs.TotalAccepted) / float64(cs.TotalOffered)
			} else {
				categorySummary[parent].AvgAcceptanceRate = (existingRate + float64(cs.TotalAccepted)/float64(cs.TotalOffered)) / 2
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deals":            results,
		"category_summary": categorySummary,
	})
}

// =============================================================================
// CATEGORY INTELLIGENCE SUBSCRIPTIONS
// =============================================================================

// handleCategoryReport serves tiered category intelligence.
// GET /brand/category-report/{parent_category}
func (s *Server) handleCategoryReport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	parentCategory := strings.TrimPrefix(r.URL.Path, "/brand/category-report/")
	if parentCategory == "" {
		writeBadRequest(w, "Category required")
		return
	}

	// Determine subscription tier (default to free)
	var tier string
	err := s.db.QueryRow(
		"SELECT tier FROM brand_subscriptions WHERE brand_user_id = ?", bs.BrandUserID,
	).Scan(&tier)
	if err != nil {
		tier = "free"
	}

	// --- Basic counts (free + premium) ---
	var totalDeals int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM brand_deals bd
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ? AND bd.status != 'deleted'
	`, parentCategory).Scan(&totalDeals)
	if err != nil {
		dbError(w, err, "Failed to count deals")
		return
	}

	var totalSwaps int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ?
	`, parentCategory).Scan(&totalSwaps)
	if err != nil {
		dbError(w, err, "Failed to count swaps")
		return
	}

	// Free tier: return basic counts only
	if tier != "premium" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"category":       parentCategory,
			"tier":           "free",
			"total_deals":    totalDeals,
			"total_swaps":    totalSwaps,
			"upgrade_prompt": "Upgrade to premium for acceptance rates, decline analysis, and price intelligence",
		})
		return
	}

	// --- Premium tier: full report ---

	// Acceptance rate
	var totalOffered, totalAccepted int
	err = s.db.QueryRow(`
		SELECT
			COUNT(*) as offered,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ?
	`, parentCategory).Scan(&totalOffered, &totalAccepted)
	if err != nil {
		dbError(w, err, "Failed to compute acceptance rate")
		return
	}
	var avgAcceptanceRate float64
	if totalOffered > 0 {
		avgAcceptanceRate = float64(totalAccepted) / float64(totalOffered)
	}

	// Top decline reasons
	type declineReason struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}
	declineRows, err := s.db.Query(`
		SELECT se.decline_reason, COUNT(*) as cnt
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ? AND se.action = 'declined' AND se.decline_reason != ''
		GROUP BY se.decline_reason
		ORDER BY cnt DESC
		LIMIT 10
	`, parentCategory)
	if err != nil {
		dbError(w, err, "Failed to query decline reasons")
		return
	}
	defer declineRows.Close()
	topDeclineReasons := []declineReason{}
	for declineRows.Next() {
		var dr declineReason
		declineRows.Scan(&dr.Reason, &dr.Count)
		topDeclineReasons = append(topDeclineReasons, dr)
	}

	// Average discount
	var avgDiscountCents float64
	err = s.db.QueryRow(`
		SELECT COALESCE(AVG(bd.discount_cents), 0)
		FROM brand_deals bd
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ? AND bd.status != 'deleted'
	`, parentCategory).Scan(&avgDiscountCents)
	if err != nil {
		dbError(w, err, "Failed to compute avg discount")
		return
	}

	// Average time to action
	var avgTimeToActionMs float64
	err = s.db.QueryRow(`
		SELECT COALESCE(AVG(se.time_to_action_ms), 0)
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ? AND se.time_to_action_ms > 0
	`, parentCategory).Scan(&avgTimeToActionMs)
	if err != nil {
		dbError(w, err, "Failed to compute time to action")
		return
	}

	// Propagation rate
	var propagationCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM swap_propagations sp
		JOIN brand_deals bd ON bd.id = sp.deal_id
		JOIN categories c ON c.id = bd.category_id
		WHERE c.parent_category = ?
	`, parentCategory).Scan(&propagationCount)
	if err != nil {
		dbError(w, err, "Failed to compute propagation rate")
		return
	}
	var propagationRate float64
	if totalSwaps > 0 {
		propagationRate = float64(propagationCount) / float64(totalSwaps)
	}

	// Subcategory breakdown
	type subcategory struct {
		Name           string  `json:"name"`
		Deals          int     `json:"deals"`
		AcceptanceRate float64 `json:"acceptance_rate"`
	}
	subRows, err := s.db.Query(`
		SELECT c.name,
			COUNT(DISTINCT bd.id) as deals,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			COUNT(se.id) as total
		FROM categories c
		JOIN brand_deals bd ON bd.category_id = c.id
		LEFT JOIN swap_events se ON se.deal_id = bd.id
		WHERE c.parent_category = ? AND bd.status != 'deleted'
		GROUP BY c.name
		ORDER BY deals DESC
	`, parentCategory)
	if err != nil {
		dbError(w, err, "Failed to query subcategories")
		return
	}
	defer subRows.Close()
	subcategories := []subcategory{}
	for subRows.Next() {
		var sc subcategory
		var accepted, total int
		subRows.Scan(&sc.Name, &sc.Deals, &accepted, &total)
		if total > 0 {
			sc.AcceptanceRate = float64(accepted) / float64(total)
		}
		subcategories = append(subcategories, sc)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"category":              parentCategory,
		"tier":                  "premium",
		"total_deals":           totalDeals,
		"total_swaps":           totalSwaps,
		"avg_acceptance_rate":   avgAcceptanceRate,
		"top_decline_reasons":   topDeclineReasons,
		"avg_discount_cents":    avgDiscountCents,
		"avg_time_to_action_ms": avgTimeToActionMs,
		"propagation_rate":      propagationRate,
		"subcategories":         subcategories,
	})
}

// handleBrandSubscription manages subscription tier.
// GET  /brand/subscription — returns current tier
// POST /brand/subscription — set tier (body: {"tier": "premium"})
func (s *Server) handleBrandSubscription(w http.ResponseWriter, r *http.Request) {
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		var tier string
		err := s.db.QueryRow(
			"SELECT tier FROM brand_subscriptions WHERE brand_user_id = ?", bs.BrandUserID,
		).Scan(&tier)
		if err != nil {
			tier = "free"
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"brand_user_id": bs.BrandUserID,
			"tier":          tier,
		})

	case http.MethodPost:
		var req struct {
			Tier string `json:"tier"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Tier != "free" && req.Tier != "premium" {
			writeBadRequest(w, "Tier must be 'free' or 'premium'")
			return
		}
		_, err := s.db.Exec(`
			INSERT INTO brand_subscriptions (brand_user_id, tier) VALUES (?, ?)
			ON CONFLICT(brand_user_id) DO UPDATE SET tier = excluded.tier
		`, bs.BrandUserID, req.Tier)
		if err != nil {
			dbError(w, err, "Failed to update subscription")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"brand_user_id": bs.BrandUserID,
			"tier":          req.Tier,
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// =============================================================================
// BRAND FEEDBACK
// =============================================================================

// handleBrandFeedback handles POST /brand/feedback (submit) and GET /brand/feedback (list own).
func (s *Server) handleBrandFeedback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleBrandFeedbackSubmit(w, r)
	case http.MethodGet:
		s.handleBrandFeedbackList(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleBrandFeedbackSubmit handles POST /brand/feedback — submit feedback from brand portal.
func (s *Server) handleBrandFeedbackSubmit(w http.ResponseWriter, r *http.Request) {
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	// Rate limit: max 10 submissions per user per hour
	var recentCount int
	s.db.QueryRow(
		"SELECT COUNT(*) FROM brand_feedback_requests WHERE brand_user_id = ? AND created_at > datetime('now', '-1 hour')",
		bs.BrandUserID,
	).Scan(&recentCount)
	if recentCount >= 10 {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Max 10 submissions per hour."})
		return
	}

	var req struct {
		Body    string `json:"body"`
		Type    string `json:"type"`
		Urgency string `json:"urgency"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeBadRequest(w, "Feedback body cannot be empty.")
		return
	}
	if len(req.Body) > 10000 {
		writeBadRequest(w, "Please keep feedback under 10,000 characters.")
		return
	}

	if req.Type == "" || !validFeedbackTypes[req.Type] {
		req.Type = "feature"
	}
	if req.Urgency == "" || !validFeedbackUrgency[req.Urgency] {
		req.Urgency = "normal"
	}

	result, err := s.db.Exec(
		"INSERT INTO brand_feedback_requests (brand_user_id, body, type, urgency) VALUES (?, ?, ?, ?)",
		bs.BrandUserID, req.Body, req.Type, req.Urgency,
	)
	if err != nil {
		dbError(w, err, "Failed to save feedback")
		return
	}

	id, _ := result.LastInsertId()
	log.Printf("[brand-feedback] brand_user %s submitted %s (%s): %.60s...", bs.BrandUserID, req.Type, req.Urgency, req.Body)

	// Create GitHub issue asynchronously (tagged as brand portal)
	go func() {
		if s.config.GitHubPAT == "" || s.config.GitHubRepo == "" {
			return
		}
		title := fmt.Sprintf("[brand-portal][%s] %s", req.Type, truncateRunes(req.Body, 80))
		labels := []string{"feedback", "brand-portal", req.Type}
		if req.Urgency == "critical" || req.Urgency == "high" {
			labels = append(labels, "priority:"+req.Urgency)
		}
		issueBody := fmt.Sprintf("**Source:** Brand Portal\n**Type:** %s\n**Urgency:** %s\n**Feedback ID:** %d\n\n---\n\n%s",
			req.Type, req.Urgency, id, req.Body)
		ghResult, ok := s.postGitHubIssue(title, issueBody, labels)
		if !ok {
			ghResult, ok = s.postGitHubIssue(title, issueBody, nil)
		}
		if ok && ghResult.HTMLURL != "" {
			s.db.Exec("UPDATE brand_feedback_requests SET github_issue_url = ? WHERE id = ?", ghResult.HTMLURL, id)
		}
	}()

	// Write to repo markdown files for high-urgency items
	if req.Urgency == "critical" || req.Urgency == "high" {
		go s.appendToRepoFile("MUST_HAVES.md", "[Brand Portal] "+req.Body, req.Urgency, "Must-Have Requirements",
			"Items below were submitted via \"Instructions for Product Team\" with high urgency.\nClaude Code MUST implement these. If they conflict with existing code or CLAUDE.md, these override.")
	} else {
		go s.appendToRepoFile("CONSIDER_FOR_ROADMAP.md", "[Brand Portal] "+req.Body, req.Urgency, "Consider for Roadmap",
			"Items below were submitted via \"Instructions for Product Team\" for consideration.\nClaude Code should evaluate these when planning future work.")
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"success": true,
		"message": fmt.Sprintf("Feedback #%d submitted.", id),
	})
}

// handleBrandFeedbackList handles GET /brand/feedback — list own feedback.
func (s *Server) handleBrandFeedbackList(w http.ResponseWriter, r *http.Request) {
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT id, brand_user_id, body, type, urgency, status, admin_notes, github_issue_url, created_at, COALESCE(updated_at, '')
		FROM brand_feedback_requests
		WHERE brand_user_id = ?
		ORDER BY id DESC
	`, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to fetch feedback")
		return
	}
	defer rows.Close()

	items := make([]FeedbackRequest, 0)
	for rows.Next() {
		var f FeedbackRequest
		if err := rows.Scan(&f.ID, &f.UserID, &f.Body, &f.Type, &f.Urgency, &f.Status, &f.AdminNotes, &f.GitHubIssueURL, &f.CreatedAt, &f.UpdatedAt); err != nil {
			log.Printf("[warn] scanning brand feedback: %v", err)
			continue
		}
		items = append(items, f)
	}

	writeJSON(w, http.StatusOK, items)
}
