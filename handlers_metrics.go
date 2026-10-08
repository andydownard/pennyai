package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// =============================================================================
// ADMIN BUSINESS METRICS HANDLER
// =============================================================================

// handleBusinessMetrics handles GET /api/admin/business-metrics
// Returns the last 30 days (or ?days=N) of business_metrics_daily rows.
// Requires admin auth.
func (s *Server) handleBusinessMetrics(w http.ResponseWriter, r *http.Request) {
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

	rows, err := s.db.Query(`
		SELECT date, dau, wau, mau, new_users,
			swaps_offered, swaps_accepted, swaps_declined, acceptance_rate,
			gross_revenue_cents, active_deals, active_brands, computed_at
		FROM business_metrics_daily
		WHERE date >= DATE('now', ?)
		ORDER BY date DESC
	`, modifier)
	if err != nil {
		dbError(w, err, "Failed to query business metrics")
		return
	}

	type metricsRow struct {
		Date              string  `json:"date"`
		DAU               int     `json:"dau"`
		WAU               int     `json:"wau"`
		MAU               int     `json:"mau"`
		NewUsers          int     `json:"new_users"`
		SwapsOffered      int     `json:"swaps_offered"`
		SwapsAccepted     int     `json:"swaps_accepted"`
		SwapsDeclined     int     `json:"swaps_declined"`
		AcceptanceRate    float64 `json:"acceptance_rate"`
		GrossRevenueCents int     `json:"gross_revenue_cents"`
		ActiveDeals       int     `json:"active_deals"`
		ActiveBrands      int     `json:"active_brands"`
		ComputedAt        string  `json:"computed_at"`
	}

	var results []metricsRow
	for rows.Next() {
		var m metricsRow
		if err := rows.Scan(&m.Date, &m.DAU, &m.WAU, &m.MAU, &m.NewUsers,
			&m.SwapsOffered, &m.SwapsAccepted, &m.SwapsDeclined, &m.AcceptanceRate,
			&m.GrossRevenueCents, &m.ActiveDeals, &m.ActiveBrands, &m.ComputedAt); err != nil {
			log.Printf("[metrics] scan business_metrics_daily: %v", err)
			continue
		}
		results = append(results, m)
	}
	rows.Close()

	if results == nil {
		results = []metricsRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"days":    days,
		"metrics": results,
	})
}

// =============================================================================
// BRAND DEAL PERFORMANCE HANDLER
// =============================================================================

// handleBrandDealPerformance handles GET /brand/deal-performance/{id}
// Returns deal_performance_daily rows for the given deal_id (last 30 days).
// Requires brand session auth, and the deal must belong to the authenticated brand.
func (s *Server) handleBrandDealPerformance(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	bs, ok := s.requireBrandSession(w, r)
	if !ok {
		return
	}

	// Extract deal ID from path: /brand/deal-performance/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/brand/deal-performance/")
	dealID, err := strconv.Atoi(idStr)
	if err != nil || dealID <= 0 {
		writeBadRequest(w, "Invalid deal ID")
		return
	}

	// Verify the deal belongs to this brand user
	var ownerID string
	err = s.db.QueryRow("SELECT COALESCE(brand_user_id, 0) FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil {
		writeNotFound(w, "Deal not found")
		return
	}
	if ownerID != bs.BrandUserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Not your deal"})
		return
	}

	rows, err := s.db.Query(`
		SELECT date, impressions, accepts, declines, revenue_cents, verified_count
		FROM deal_performance_daily
		WHERE deal_id = ? AND date >= DATE('now', '-30 days')
		ORDER BY date DESC
	`, dealID)
	if err != nil {
		dbError(w, err, "Failed to query deal performance")
		return
	}

	type perfRow struct {
		Date          string `json:"date"`
		Impressions   int    `json:"impressions"`
		Accepts       int    `json:"accepts"`
		Declines      int    `json:"declines"`
		RevenueCents  int    `json:"revenue_cents"`
		VerifiedCount int    `json:"verified_count"`
	}

	var results []perfRow
	for rows.Next() {
		var p perfRow
		if err := rows.Scan(&p.Date, &p.Impressions, &p.Accepts, &p.Declines, &p.RevenueCents, &p.VerifiedCount); err != nil {
			log.Printf("[metrics] scan deal_performance_daily: %v", err)
			continue
		}
		results = append(results, p)
	}
	rows.Close()

	if results == nil {
		results = []perfRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deal_id":     dealID,
		"performance": results,
	})
}

// =============================================================================
// CHURN RISK HANDLER
// =============================================================================

// handleChurnRisk handles GET /api/admin/churn-risk
// Returns users with risk_score > 0.5 on any signal, grouped by user.
// Requires admin auth.
func (s *Server) handleChurnRisk(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	type signalInfo struct {
		SignalType  string  `json:"signal_type"`
		SignalValue float64 `json:"signal_value"`
		RiskScore   float64 `json:"risk_score"`
		ComputedAt  string  `json:"computed_at"`
	}
	type userRisk struct {
		UserID         string       `json:"user_id"`
		Name           string       `json:"name"`
		Email          string       `json:"email"`
		CompositeRisk  float64      `json:"composite_risk"`
		Signals        []signalInfo `json:"signals"`
	}

	rows, err := s.db.Query(`
		SELECT cs.user_id, COALESCE(u.name, ''), COALESCE(u.email, ''),
			cs.signal_type, cs.signal_value, cs.risk_score, cs.computed_at
		FROM churn_signals cs
		JOIN users u ON u.id = cs.user_id
		WHERE cs.risk_score > 0.5
		ORDER BY cs.user_id, cs.risk_score DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query churn signals")
		return
	}

	userMap := map[string]*userRisk{}
	var order []string
	for rows.Next() {
		var userID string
		var name, email string
		var si signalInfo
		if err := rows.Scan(&userID, &name, &email, &si.SignalType, &si.SignalValue, &si.RiskScore, &si.ComputedAt); err != nil {
			log.Printf("[churn-risk] scan: %v", err)
			continue
		}
		ur, exists := userMap[userID]
		if !exists {
			ur = &userRisk{
				UserID: userID,
				Name:   name,
				Email:  email,
			}
			userMap[userID] = ur
			order = append(order, userID)
		}
		ur.Signals = append(ur.Signals, si)
	}
	rows.Close()

	// Compute composite risk: average of all signal risk scores per user
	var results []userRisk
	for _, uid := range order {
		ur := userMap[uid]
		total := 0.0
		for _, sig := range ur.Signals {
			total += sig.RiskScore
		}
		ur.CompositeRisk = total / float64(len(ur.Signals))
		results = append(results, *ur)
	}

	if results == nil {
		results = []userRisk{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"at_risk_users": results,
		"count":         len(results),
	})
}

// =============================================================================
// COHORT ANALYSIS HANDLER
// =============================================================================

// handleCohortAnalysis handles GET /api/admin/cohort-analysis
// Returns cohort retention data grouped by signup week.
// Requires admin auth.
func (s *Server) handleCohortAnalysis(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	type cohortSummary struct {
		CohortWeek    string  `json:"cohort_week"`
		UserCount     int     `json:"user_count"`
		RetainedCount int     `json:"retained_count"`
		AvgSwaps      float64 `json:"avg_swaps"`
		AvgSavings    float64 `json:"avg_savings_cents"`
	}

	rows, err := s.db.Query(`
		SELECT
			cohort_week,
			COUNT(*) as user_count,
			COALESCE(SUM(CASE WHEN last_active >= datetime('now', '-7 days') THEN 1 ELSE 0 END), 0) as retained_count,
			COALESCE(AVG(total_swaps), 0) as avg_swaps,
			COALESCE(AVG(total_savings_cents), 0) as avg_savings
		FROM user_cohorts
		GROUP BY cohort_week
		ORDER BY cohort_week DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query cohort analysis")
		return
	}

	var results []cohortSummary
	for rows.Next() {
		var cs cohortSummary
		if err := rows.Scan(&cs.CohortWeek, &cs.UserCount, &cs.RetainedCount, &cs.AvgSwaps, &cs.AvgSavings); err != nil {
			log.Printf("[cohort-analysis] scan: %v", err)
			continue
		}
		results = append(results, cs)
	}
	rows.Close()

	if results == nil {
		results = []cohortSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cohorts": results,
	})
}

// handleMarketplaceHealth handles GET /api/admin/marketplace-health
// Returns last 30 days of marketplace health data.
func (s *Server) handleMarketplaceHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT date, supply_score, demand_score, balance_score, revenue_concentration, health_index
		FROM marketplace_health
		WHERE date >= DATE('now', '-30 days')
		ORDER BY date DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query marketplace health")
		return
	}

	type healthRow struct {
		Date                string  `json:"date"`
		SupplyScore         float64 `json:"supply_score"`
		DemandScore         float64 `json:"demand_score"`
		BalanceScore        float64 `json:"balance_score"`
		RevenueConcentration float64 `json:"revenue_concentration"`
		HealthIndex         float64 `json:"health_index"`
	}
	var results []healthRow
	for rows.Next() {
		var h healthRow
		if err := rows.Scan(&h.Date, &h.SupplyScore, &h.DemandScore, &h.BalanceScore, &h.RevenueConcentration, &h.HealthIndex); err != nil {
			continue
		}
		results = append(results, h)
	}
	rows.Close()

	if results == nil {
		results = []healthRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"health": results,
	})
}

// handleSegmentAnalysis handles GET /api/admin/segment-analysis
// Returns segment acceptance rates and avg discounts from category_swap_insights
// where category starts with "segment:". Requires admin auth.
func (s *Server) handleSegmentAnalysis(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT category, avg_acceptance_rate, avg_discount_pct, total_swaps, last_computed
		FROM category_swap_insights
		WHERE category LIKE 'segment:%'
		ORDER BY category
	`)
	if err != nil {
		dbError(w, err, "Failed to query segment analysis")
		return
	}

	type segmentRow struct {
		Segment         string  `json:"segment"`
		AcceptanceRate  float64 `json:"acceptance_rate"`
		AvgDiscountCents float64 `json:"avg_discount_cents"`
		TotalOffers     int     `json:"total_offers"`
		LastComputed    string  `json:"last_computed"`
	}

	var results []segmentRow
	for rows.Next() {
		var category string
		var sr segmentRow
		if err := rows.Scan(&category, &sr.AcceptanceRate, &sr.AvgDiscountCents, &sr.TotalOffers, &sr.LastComputed); err != nil {
			log.Printf("[segment-analysis] scan: %v", err)
			continue
		}
		sr.Segment = strings.TrimPrefix(category, "segment:")
		results = append(results, sr)
	}
	rows.Close()

	if results == nil {
		results = []segmentRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"segments": results,
	})
}

// =============================================================================
// CATEGORY-LEVEL INSIGHTS
// =============================================================================

// handleCategoryInsights handles GET /api/admin/category-insights
// Returns all category_swap_insights rows from last 30 days (non-segment rows).
// Requires admin auth.
func (s *Server) handleCategoryInsights(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	rows, err := s.db.Query(`
		SELECT category, avg_acceptance_rate, avg_discount_pct, total_swaps, top_decline_reason, last_computed
		FROM category_swap_insights
		WHERE category NOT LIKE 'segment:%'
			AND last_computed >= datetime('now', '-30 days')
		ORDER BY total_swaps DESC
	`)
	if err != nil {
		dbError(w, err, "Failed to query category insights")
		return
	}

	type categoryRow struct {
		Category         string  `json:"category"`
		AcceptanceRate   float64 `json:"acceptance_rate"`
		AvgDiscountCents float64 `json:"avg_discount_cents"`
		TotalSwaps       int     `json:"total_swaps"`
		TopDeclineReason string  `json:"top_decline_reason"`
		LastComputed     string  `json:"last_computed"`
	}

	var results []categoryRow
	for rows.Next() {
		var cr categoryRow
		if err := rows.Scan(&cr.Category, &cr.AcceptanceRate, &cr.AvgDiscountCents, &cr.TotalSwaps, &cr.TopDeclineReason, &cr.LastComputed); err != nil {
			log.Printf("[category-insights] scan: %v", err)
			continue
		}
		results = append(results, cr)
	}
	rows.Close()

	if results == nil {
		results = []categoryRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": results,
	})
}

// handleCategoryAlerts returns recent category alerts.
// GET /api/admin/category-alerts — admin only.
// Also available as GET /brand/category-alerts — brand portal (shows only their categories).
func (s *Server) handleCategoryAlerts(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Determine if admin or brand request
	isBrandRequest := strings.HasPrefix(r.URL.Path, "/brand/")
	var brandUserID string

	if isBrandRequest {
		bs, ok := s.requireBrandSession(w, r)
		if !ok {
			return
		}
		brandUserID = bs.BrandUserID
	} else {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
	}

	var queryArgs []interface{}
	query := `
		SELECT ca.id, ca.alert_type, ca.brand, ca.category_id,
		       COALESCE(c.name, ''), COALESCE(c.parent_category, ''),
		       ca.severity, ca.message, ca.data_json, ca.created_at
		FROM category_alerts ca
		LEFT JOIN categories c ON c.id = ca.category_id
	`

	if isBrandRequest && brandUserID != "" {
		// Filter to categories where this brand has deals
		query += ` WHERE ca.category_id IN (SELECT DISTINCT category_id FROM brand_deals WHERE brand_user_id = ?)`
		queryArgs = append(queryArgs, brandUserID)
	}

	query += ` ORDER BY ca.created_at DESC LIMIT 50`

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load alerts"})
		return
	}

	type alertRow struct {
		ID             int    `json:"id"`
		AlertType      string `json:"alert_type"`
		Brand          string `json:"brand"`
		CategoryID     int    `json:"category_id"`
		CategoryName   string `json:"category_name"`
		ParentCategory string `json:"parent_category"`
		Severity       string `json:"severity"`
		Message        string `json:"message"`
		DataJSON       string `json:"data_json"`
		CreatedAt      string `json:"created_at"`
	}
	var results []alertRow
	for rows.Next() {
		var a alertRow
		if err := rows.Scan(&a.ID, &a.AlertType, &a.Brand, &a.CategoryID, &a.CategoryName, &a.ParentCategory, &a.Severity, &a.Message, &a.DataJSON, &a.CreatedAt); err != nil {
			continue
		}
		results = append(results, a)
	}
	rows.Close()

	if results == nil {
		results = []alertRow{}
	}
	writeJSON(w, http.StatusOK, results)
}
