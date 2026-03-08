package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// brandLoginIPLimiter rate-limits brand login attempts per IP.
var brandLoginIPLimiter sync.Map

// =============================================================================
// BRAND AUTH HELPERS
// =============================================================================

// BrandSession represents an authenticated brand portal session.
type BrandSession struct {
	BrandUserID int
	CreatedAt   time.Time
}

func (s *Server) createBrandSession(brandUserID int) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := s.db.Exec(
		"INSERT INTO brand_sessions (token, brand_user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, brandUserID, time.Now().UTC(), time.Now().UTC().Add(sessionDuration),
	)
	return token, err
}

func (s *Server) getBrandSession(token string) (BrandSession, bool) {
	var bs BrandSession
	err := s.db.QueryRow(
		"SELECT brand_user_id, created_at FROM brand_sessions WHERE token = ? AND expires_at > ?",
		token, time.Now().UTC(),
	).Scan(&bs.BrandUserID, &bs.CreatedAt)
	if err != nil {
		return BrandSession{}, false
	}
	return bs, true
}

func setBrandSessionCookie(w http.ResponseWriter, cfg Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "brand_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func (s *Server) requireBrandSession(w http.ResponseWriter, r *http.Request) (BrandSession, bool) {
	cookie, err := r.Cookie("brand_session")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not logged in"})
		return BrandSession{}, false
	}
	bs, ok := s.getBrandSession(cookie.Value)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Session expired"})
		return BrandSession{}, false
	}
	return bs, true
}

// =============================================================================
// BRAND REGISTRATION
// =============================================================================

func (s *Server) handleBrandRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		CompanyName string `json:"company_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email address"})
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password must be 8-72 characters"})
		return
	}
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	if req.CompanyName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Company name is required"})
		return
	}

	// Check if email already registered
	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_users WHERE email = ?", req.Email).Scan(&exists)
	if exists > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with this email already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	// Generate verify token
	vb := make([]byte, 16)
	rand.Read(vb)
	verifyToken := hex.EncodeToString(vb)

	// Check if this is a "Founding Brand" (first 10)
	var brandCount int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_users").Scan(&brandCount)
	badge := ""
	if brandCount < 10 {
		badge = "founding"
	}

	result, err := s.db.Exec(
		`INSERT INTO brand_users (email, password_hash, company_name, verified, verify_token, badge)
		 VALUES (?, ?, ?, 1, ?, ?)`,
		req.Email, string(hash), req.CompanyName, verifyToken, badge,
	)
	if err != nil {
		dbError(w, err, "Failed to create account")
		return
	}

	brandUserID, _ := result.LastInsertId()

	// Initialize balance
	s.db.Exec("INSERT INTO brand_balance (brand_user_id, balance_cents) VALUES (?, 0)", brandUserID)

	// Create session
	token, err := s.createBrandSession(int(brandUserID))
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setBrandSessionCookie(w, s.config, token)
	log.Printf("[brand-auth] brand user %d registered (%s, %s)", brandUserID, req.Email, req.CompanyName)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"brand_user_id": brandUserID,
		"badge":         badge,
	})
}

// =============================================================================
// BRAND LOGIN
// =============================================================================

func (s *Server) handleBrandLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	clientIP := getClientIP(r)
	if checkRateLimit(&brandLoginIPLimiter, clientIP, 10*time.Minute, 10) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many login attempts"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var brandUserID int
	var passwordHash string
	err := s.db.QueryRow(
		"SELECT id, password_hash FROM brand_users WHERE email = ?",
		req.Email,
	).Scan(&brandUserID, &passwordHash)

	if err != nil {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfortimingattttttttttttttttttttttttttttttttttttt"), []byte(req.Password))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
		return
	}

	token, err := s.createBrandSession(brandUserID)
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}

	setBrandSessionCookie(w, s.config, token)
	log.Printf("[brand-auth] brand user %d logged in (%s)", brandUserID, req.Email)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"brand_user_id": brandUserID,
	})
}

// =============================================================================
// BRAND ME / LOGOUT
// =============================================================================

func (s *Server) handleBrandMe(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	var email, companyName, badge string
	var verified int
	var createdAt string
	err := s.db.QueryRow(
		"SELECT email, company_name, verified, badge, created_at FROM brand_users WHERE id = ?",
		bs.BrandUserID,
	).Scan(&email, &companyName, &verified, &badge, &createdAt)
	if err != nil {
		dbError(w, err, "Failed to load profile")
		return
	}

	var balanceCents int
	s.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", bs.BrandUserID).Scan(&balanceCents)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"brand_user_id": bs.BrandUserID,
		"email":         email,
		"company_name":  companyName,
		"verified":      verified == 1,
		"badge":         badge,
		"balance_cents": balanceCents,
		"created_at":    createdAt,
	})
}

func (s *Server) handleBrandLogout(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	cookie, err := r.Cookie("brand_session")
	if err == nil {
		s.db.Exec("DELETE FROM brand_sessions WHERE token = ?", cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "brand_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"success": "true"})
}

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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid deal ID"})
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
			   created_at
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
		); err != nil {
			dbError(w, err, "Failed to scan deal")
			return
		}
		deals = append(deals, dealWithMetrics{BrandDeal: d})
	}
	rows.Close()

	// Enrich with swap event metrics (must be done after rows.Close to avoid SQLite deadlock)
	for i, dm := range deals {
		s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'offered'", dm.ID).Scan(&deals[i].Offered)
		s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'accepted'", dm.ID).Scan(&deals[i].Accepted)
		s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'declined'", dm.ID).Scan(&deals[i].Declined)
		if deals[i].Offered > 0 {
			deals[i].AcceptRate = (deals[i].Accepted * 100) / deals[i].Offered
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
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.CompetitorProduct == "" || req.OfferBrand == "" || req.OfferProduct == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "competitor_product, offer_brand, and offer_product are required"})
		return
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

	result, err := s.db.Exec(`
		INSERT INTO brand_deals (
			competitor_product, offer_brand, offer_product, discount_cents,
			total_budget_cents, status, brand_user_id,
			offer_price_cents, competitor_price_cents,
			offer_size, competitor_size, offer_unit_price, competitor_unit_price,
			unit_label, why_bullets, allergen_notes,
			deal_type, quantity_offered, trial_product_name, trial_size, campaign_tag,
			category_id, product_description, key_benefits, brand_story
		) VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.CompetitorProduct, req.OfferBrand, req.OfferProduct, req.DiscountCents,
		req.TotalBudgetCents, bs.BrandUserID,
		req.OfferPriceCents, req.CompetitorPriceCents,
		req.OfferSize, req.CompetitorSize, req.OfferUnitPrice, req.CompetitorUnitPrice,
		req.UnitLabel, req.WhyBullets, req.AllergenNotes,
		req.DealType, req.QuantityOffered, req.TrialProductName, req.TrialSize, req.CampaignTag,
		categoryID, req.ProductDescription, req.KeyBenefits, req.BrandStory,
	)
	if err != nil {
		dbError(w, err, "Failed to create deal")
		return
	}

	dealID, _ := result.LastInsertId()
	log.Printf("[brand-portal] brand user %d created deal %d", bs.BrandUserID, dealID)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      dealID,
		"message": "Deal created",
	})
}

func (s *Server) updateBrandDeal(w http.ResponseWriter, r *http.Request, bs BrandSession, dealID int) {
	// Verify ownership
	var ownerID sql.NullInt64
	err := s.db.QueryRow("SELECT brand_user_id FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil || !ownerID.Valid || int(ownerID.Int64) != bs.BrandUserID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Deal not found"})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No fields to update"})
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
	var ownerID sql.NullInt64
	err := s.db.QueryRow("SELECT brand_user_id FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil || !ownerID.Valid || int(ownerID.Int64) != bs.BrandUserID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Deal not found"})
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

	var totalDeals, activeDeals int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE brand_user_id = ? AND status != 'deleted'", bs.BrandUserID).Scan(&totalDeals)
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE brand_user_id = ? AND status = 'active'", bs.BrandUserID).Scan(&activeDeals)

	var totalSpentCents int
	s.db.QueryRow("SELECT COALESCE(SUM(budget_spent_cents), 0) FROM brand_deals WHERE brand_user_id = ?", bs.BrandUserID).Scan(&totalSpentCents)

	var totalSwaps, totalAccepted, totalDeclined int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'offered'`, bs.BrandUserID).Scan(&totalSwaps)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'accepted'`, bs.BrandUserID).Scan(&totalAccepted)
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.action = 'declined'`, bs.BrandUserID).Scan(&totalDeclined)

	acceptRate := 0
	if totalSwaps > 0 {
		acceptRate = (totalAccepted * 100) / totalSwaps
	}

	var balanceCents int
	s.db.QueryRow("SELECT balance_cents FROM brand_balance WHERE brand_user_id = ?", bs.BrandUserID).Scan(&balanceCents)

	// Swaps per day for the last 30 days
	type dayData struct {
		Date     string `json:"date"`
		Offered  int    `json:"offered"`
		Accepted int    `json:"accepted"`
	}
	dailyRows, _ := s.db.Query(`
		SELECT DATE(se.created_at) as day, se.action, COUNT(*) as cnt
		FROM swap_events se
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND se.created_at > datetime('now', '-30 days')
		  AND se.action IN ('offered', 'accepted')
		GROUP BY day, se.action
		ORDER BY day`, bs.BrandUserID)
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
	var verifiedPurchases int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_claims sc
		JOIN swap_events se ON sc.swap_event_id = se.id
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND sc.status = 'verified'`, bs.BrandUserID).Scan(&verifiedPurchases)

	var pendingClaims int
	s.db.QueryRow(`SELECT COUNT(*) FROM swap_claims sc
		JOIN swap_events se ON sc.swap_event_id = se.id
		JOIN brand_deals bd ON se.deal_id = bd.id
		WHERE bd.brand_user_id = ? AND sc.status = 'pending'`, bs.BrandUserID).Scan(&pendingClaims)

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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Category required"})
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
// BRAND TRANSACTIONS
// =============================================================================

func (s *Server) handleBrandTransactions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT id, type, amount_cents, description, COALESCE(stripe_id, ''), created_at
		FROM brand_transactions WHERE brand_user_id = ?
		ORDER BY created_at DESC LIMIT 100`, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to load transactions")
		return
	}
	defer rows.Close()

	type txn struct {
		ID          int    `json:"id"`
		Type        string `json:"type"`
		AmountCents int    `json:"amount_cents"`
		Description string `json:"description"`
		StripeID    string `json:"stripe_id,omitempty"`
		CreatedAt   string `json:"created_at"`
	}
	txns := []txn{}
	for rows.Next() {
		var t txn
		rows.Scan(&t.ID, &t.Type, &t.AmountCents, &t.Description, &t.StripeID, &t.CreatedAt)
		txns = append(txns, t)
	}

	writeJSON(w, http.StatusOK, txns)
}

// =============================================================================
// CSV EXPORT (M9)
// =============================================================================

func (s *Server) handleBrandExport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	exportType := r.URL.Query().Get("type")
	if exportType == "" {
		exportType = "swaps"
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_export_%s.csv", exportType, time.Now().Format("2006-01-02")))

	switch exportType {
	case "swaps":
		w.Write([]byte("date,deal_id,competitor_product,offer_product,action,user_id\n"))
		rows, err := s.db.Query(`
			SELECT COALESCE(se.created_at,''), se.deal_id, COALESCE(bd.competitor_product,''),
				   COALESCE(bd.offer_product,''), se.action, se.user_id
			FROM swap_events se
			JOIN brand_deals bd ON se.deal_id = bd.id
			WHERE bd.brand_user_id = ?
			ORDER BY se.created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var date, comp, offer, action string
			var dealID, userID int
			rows.Scan(&date, &dealID, &comp, &offer, &action, &userID)
			fmt.Fprintf(w, "%s,%d,%q,%q,%s,%d\n", date, dealID, comp, offer, action, userID)
		}

	case "transactions":
		w.Write([]byte("date,type,amount_cents,description,stripe_id\n"))
		rows, err := s.db.Query(`
			SELECT COALESCE(created_at,''), type, amount_cents, COALESCE(description,''), COALESCE(stripe_id,'')
			FROM brand_transactions WHERE brand_user_id = ?
			ORDER BY created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var date, txType, desc, stripeID string
			var amount int
			rows.Scan(&date, &txType, &amount, &desc, &stripeID)
			fmt.Fprintf(w, "%s,%s,%d,%q,%s\n", date, txType, amount, desc, stripeID)
		}

	case "deals":
		w.Write([]byte("id,competitor_product,offer_brand,offer_product,discount_cents,budget_cents,spent_cents,status,deal_type,created_at\n"))
		rows, err := s.db.Query(`
			SELECT id, COALESCE(competitor_product,''), COALESCE(offer_brand,''), COALESCE(offer_product,''),
				   discount_cents, total_budget_cents, budget_spent_cents, status,
				   COALESCE(deal_type,'sku_swap'), COALESCE(created_at,'')
			FROM brand_deals WHERE brand_user_id = ? AND status != 'deleted'
			ORDER BY created_at DESC`, bs.BrandUserID)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var comp, brand, offer, status, dealType, created string
			var id, discount, budget, spent int
			rows.Scan(&id, &comp, &brand, &offer, &discount, &budget, &spent, &status, &dealType, &created)
			fmt.Fprintf(w, "%d,%q,%q,%q,%d,%d,%d,%s,%s,%s\n", id, comp, brand, offer, discount, budget, spent, status, dealType, created)
		}

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid export type. Use: swaps, transactions, deals"})
	}
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Description is required"})
		return
	}
	if len(req.Description) > 5000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Description too long (max 5000 characters)"})
		return
	}

	_, err := s.db.Exec("INSERT INTO bug_reports (user_id, description) VALUES (?, ?)", session.UserID, req.Description)
	if err != nil {
		dbError(w, err, "Failed to submit report")
		return
	}

	log.Printf("[bug-report] user %d submitted bug report", session.UserID)
	writeJSON(w, http.StatusCreated, map[string]string{"message": "Thank you! Your report has been submitted."})
}

// =============================================================================
// STRIPE DEPOSIT (M5)
// =============================================================================

func (s *Server) handleBrandDeposit(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	var req struct {
		AmountCents int    `json:"amount_cents"`
		StripeToken string `json:"stripe_token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.AmountCents < 5000 { // $50 minimum
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Minimum deposit is $50.00"})
		return
	}

	// In production, this would create a Stripe Checkout Session.
	// For now, we directly credit the balance (Stripe integration placeholder).
	stripeID := ""
	if req.StripeToken != "" {
		stripeID = "pi_placeholder_" + req.StripeToken
	}

	// Credit balance
	_, err := s.db.Exec(`
		UPDATE brand_balance SET balance_cents = balance_cents + ?, last_deposit_at = CURRENT_TIMESTAMP
		WHERE brand_user_id = ?`, req.AmountCents, bs.BrandUserID)
	if err != nil {
		dbError(w, err, "Failed to process deposit")
		return
	}

	// Record transaction
	s.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
		VALUES (?, 'deposit', ?, 'Balance deposit', ?)`, bs.BrandUserID, req.AmountCents, stripeID)

	log.Printf("[brand-portal] brand user %d deposited %d cents", bs.BrandUserID, req.AmountCents)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Deposit processed",
		"amount_cents": req.AmountCents,
	})
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid deal ID"})
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
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Deal not found"})
		return
	}

	// Collect performance metrics
	var offered, accepted, declined int
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'offered'", dealID).Scan(&offered)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'accepted'", dealID).Scan(&accepted)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE deal_id = ? AND action = 'declined'", dealID).Scan(&declined)

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
		SELECT CASE WHEN SUM(CASE WHEN se.action = 'offered' THEN 1 ELSE 0 END) > 0
			THEN CAST(SUM(CASE WHEN se.action = 'accepted' THEN 1 ELSE 0 END) AS REAL) /
				 SUM(CASE WHEN se.action = 'offered' THEN 1 ELSE 0 END) * 100
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

// =============================================================================
// STRIPE WEBHOOK (M5)
// =============================================================================

func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Read raw body for signature verification
	body := make([]byte, 0)
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
		var err error
		body, err = json.RawMessage(nil), nil
		buf := make([]byte, 1024)
		for {
			n, readErr := r.Body.Read(buf)
			body = append(body, buf[:n]...)
			if readErr != nil {
				_ = err
				break
			}
		}
	}

	// Parse the event
	var event struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		// Parse payment intent and credit brand balance
		var pi struct {
			Object struct {
				ID       string `json:"id"`
				Amount   int    `json:"amount"`
				Metadata struct {
					BrandUserID string `json:"brand_user_id"`
				} `json:"metadata"`
			} `json:"object"`
		}
		if err := json.Unmarshal(event.Data, &pi); err != nil {
			log.Printf("[stripe] failed to parse payment_intent: %v", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
			return
		}

		brandUserID, _ := strconv.Atoi(pi.Object.Metadata.BrandUserID)
		if brandUserID == 0 {
			log.Printf("[stripe] payment_intent missing brand_user_id metadata")
			w.WriteHeader(http.StatusOK) // Acknowledge to prevent retries
			return
		}

		// Idempotency: check if we've already processed this payment
		var existing int
		s.db.QueryRow("SELECT COUNT(*) FROM brand_transactions WHERE stripe_id = ?", pi.Object.ID).Scan(&existing)
		if existing > 0 {
			log.Printf("[stripe] duplicate payment_intent %s, skipping", pi.Object.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Credit balance
		s.db.Exec(`UPDATE brand_balance SET balance_cents = balance_cents + ?, last_deposit_at = CURRENT_TIMESTAMP
			WHERE brand_user_id = ?`, pi.Object.Amount, brandUserID)
		s.db.Exec(`INSERT INTO brand_transactions (brand_user_id, type, amount_cents, description, stripe_id)
			VALUES (?, 'deposit', ?, 'Stripe deposit', ?)`, brandUserID, pi.Object.Amount, pi.Object.ID)

		log.Printf("[stripe] credited %d cents to brand user %d (pi: %s)", pi.Object.Amount, brandUserID, pi.Object.ID)
	}

	w.WriteHeader(http.StatusOK)
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Category required"})
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Tier must be 'free' or 'premium'"})
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
