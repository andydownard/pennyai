package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// DAILY METRICS AGGREGATION
// =============================================================================

// processDailyMetrics computes deal_performance_daily, category_swap_insights,
// and business_metrics_daily for the given date string (YYYY-MM-DD format).
// Uses the collect-then-write pattern: read all data into slices, close rows,
// then write. This is CRITICAL for SQLite with MaxOpenConns=1.
func (s *Server) processDailyMetrics(dateStr string) {
	start := time.Now()

	// =========================================================================
	// 1. Deal performance daily — aggregate swap_events grouped by deal_id
	// =========================================================================
	type dealPerf struct {
		dealID      int
		impressions int
		accepts     int
		declines    int
	}
	var dealPerfData []dealPerf

	rows, err := s.db.Query(`
		SELECT deal_id,
			COUNT(CASE WHEN action = 'offered' THEN 1 END) as impressions,
			COUNT(CASE WHEN action = 'accepted' THEN 1 END) as accepts,
			COUNT(CASE WHEN action = 'declined' THEN 1 END) as declines
		FROM swap_events
		WHERE DATE(created_at) = ?
		GROUP BY deal_id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query swap_events for %s: %v", dateStr, err)
	} else {
		for rows.Next() {
			var dp dealPerf
			if err := rows.Scan(&dp.dealID, &dp.impressions, &dp.accepts, &dp.declines); err != nil {
				log.Printf("[metrics] scan deal perf: %v", err)
				continue
			}
			dealPerfData = append(dealPerfData, dp)
		}
		rows.Close()
	}

	// Collect revenue per deal from brand_transactions (spend events for that date)
	type dealRevenue struct {
		dealID      int
		revenueCents int
	}
	revenueMap := map[int]int{}
	revRows, err := s.db.Query(`
		SELECT bd.id, COALESCE(SUM(bt.amount_cents), 0)
		FROM brand_deals bd
		JOIN brand_transactions bt ON bt.brand_user_id = bd.brand_user_id
		WHERE DATE(bt.created_at) = ? AND bt.type = 'spend'
		GROUP BY bd.id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query brand_transactions for revenue: %v", err)
	} else {
		for revRows.Next() {
			var dealID, cents int
			if revRows.Scan(&dealID, &cents) == nil {
				revenueMap[dealID] = cents
			}
		}
		revRows.Close()
	}

	// Collect verified purchase counts from swap_claims
	verifiedMap := map[int]int{}
	verRows, err := s.db.Query(`
		SELECT deal_id, COUNT(*)
		FROM swap_claims
		WHERE status = 'verified' AND DATE(verified_at) = ?
		GROUP BY deal_id
	`, dateStr)
	if err != nil {
		log.Printf("[metrics] failed to query swap_claims: %v", err)
	} else {
		for verRows.Next() {
			var dealID, cnt int
			if verRows.Scan(&dealID, &cnt) == nil {
				verifiedMap[dealID] = cnt
			}
		}
		verRows.Close()
	}

	// Write deal_performance_daily rows
	for _, dp := range dealPerfData {
		_, err := s.db.Exec(`
			INSERT INTO deal_performance_daily (deal_id, date, impressions, accepts, declines, revenue_cents, verified_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(deal_id, date) DO UPDATE SET
				impressions = excluded.impressions,
				accepts = excluded.accepts,
				declines = excluded.declines,
				revenue_cents = excluded.revenue_cents,
				verified_count = excluded.verified_count
		`, dp.dealID, dateStr, dp.impressions, dp.accepts, dp.declines, revenueMap[dp.dealID], verifiedMap[dp.dealID])
		if err != nil {
			log.Printf("[metrics] failed to upsert deal_performance_daily for deal %d on %s: %v", dp.dealID, dateStr, err)
		}
	}

	// =========================================================================
	// 2. Category swap insights — aggregate across deals by competitor_product
	// =========================================================================
	type catInsight struct {
		category  string
		offered   int
		accepted  int
		avgDisc   float64
	}
	var catData []catInsight

	catRows, err := s.db.Query(`
		SELECT LOWER(bd.competitor_product) as category,
			COUNT(CASE WHEN se.action = 'offered' THEN 1 END) as offered,
			COUNT(CASE WHEN se.action = 'accepted' THEN 1 END) as accepted,
			AVG(bd.discount_cents) as avg_discount
		FROM swap_events se
		JOIN brand_deals bd ON bd.id = se.deal_id
		WHERE bd.status != 'deleted'
		GROUP BY LOWER(bd.competitor_product)
	`)
	if err != nil {
		log.Printf("[metrics] failed to query category insights: %v", err)
	} else {
		for catRows.Next() {
			var ci catInsight
			if err := catRows.Scan(&ci.category, &ci.offered, &ci.accepted, &ci.avgDisc); err != nil {
				log.Printf("[metrics] scan category insight: %v", err)
				continue
			}
			catData = append(catData, ci)
		}
		catRows.Close()
	}

	// Write category_swap_insights
	for _, ci := range catData {
		acceptRate := 0.0
		if ci.offered > 0 {
			acceptRate = float64(ci.accepted) / float64(ci.offered) * 100.0
		}
		totalSwaps := ci.offered
		_, err := s.db.Exec(`
			INSERT INTO category_swap_insights (category, avg_acceptance_rate, avg_discount_pct, total_swaps, last_computed)
			VALUES (?, ?, ?, ?, datetime('now'))
			ON CONFLICT(category) DO UPDATE SET
				avg_acceptance_rate = excluded.avg_acceptance_rate,
				avg_discount_pct = excluded.avg_discount_pct,
				total_swaps = excluded.total_swaps,
				last_computed = datetime('now')
		`, ci.category, acceptRate, ci.avgDisc, totalSwaps)
		if err != nil {
			log.Printf("[metrics] failed to upsert category_swap_insights for %s: %v", ci.category, err)
		}
	}

	// =========================================================================
	// 3. Business metrics daily — aggregate DAU/WAU/MAU, new users, swap counts
	// =========================================================================

	// DAU: distinct sessions active on dateStr
	var dau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE DATE(created_at) <= ? AND DATE(expires_at) >= ?
	`, dateStr, dateStr).Scan(&dau)

	// WAU: distinct sessions active in the 7 days ending on dateStr
	var wau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= DATE(?, '-6 days') AND DATE(created_at) <= ?
	`, dateStr, dateStr).Scan(&wau)

	// MAU: distinct sessions active in the 30 days ending on dateStr
	var mau int
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM sessions
		WHERE created_at >= DATE(?, '-29 days') AND DATE(created_at) <= ?
	`, dateStr, dateStr).Scan(&mau)

	// New users created on dateStr
	var newUsers int
	s.db.QueryRow("SELECT COUNT(*) FROM users WHERE DATE(created_at) = ?", dateStr).Scan(&newUsers)

	// Swap counts for dateStr
	var swapsOffered, swapsAccepted, swapsDeclined int
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'offered' AND DATE(created_at) = ?", dateStr).Scan(&swapsOffered)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'accepted' AND DATE(created_at) = ?", dateStr).Scan(&swapsAccepted)
	s.db.QueryRow("SELECT COUNT(*) FROM swap_events WHERE action = 'declined' AND DATE(created_at) = ?", dateStr).Scan(&swapsDeclined)

	acceptanceRate := 0.0
	if swapsOffered > 0 {
		acceptanceRate = float64(swapsAccepted) / float64(swapsOffered) * 100.0
	}

	// Gross revenue from brand_transactions on dateStr
	var grossRevenueCents int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(amount_cents), 0) FROM brand_transactions
		WHERE type = 'spend' AND DATE(created_at) = ?
	`, dateStr).Scan(&grossRevenueCents)

	// Active deals and brands
	var activeDeals, activeBrands int
	s.db.QueryRow("SELECT COUNT(*) FROM brand_deals WHERE status = 'active'").Scan(&activeDeals)
	s.db.QueryRow("SELECT COUNT(DISTINCT offer_brand) FROM brand_deals WHERE status = 'active'").Scan(&activeBrands)

	// Write business_metrics_daily
	_, err = s.db.Exec(`
		INSERT INTO business_metrics_daily (date, dau, wau, mau, new_users,
			swaps_offered, swaps_accepted, swaps_declined, acceptance_rate,
			gross_revenue_cents, active_deals, active_brands, computed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(date) DO UPDATE SET
			dau = excluded.dau,
			wau = excluded.wau,
			mau = excluded.mau,
			new_users = excluded.new_users,
			swaps_offered = excluded.swaps_offered,
			swaps_accepted = excluded.swaps_accepted,
			swaps_declined = excluded.swaps_declined,
			acceptance_rate = excluded.acceptance_rate,
			gross_revenue_cents = excluded.gross_revenue_cents,
			active_deals = excluded.active_deals,
			active_brands = excluded.active_brands,
			computed_at = datetime('now')
	`, dateStr, dau, wau, mau, newUsers,
		swapsOffered, swapsAccepted, swapsDeclined, acceptanceRate,
		grossRevenueCents, activeDeals, activeBrands)
	if err != nil {
		log.Printf("[metrics] failed to upsert business_metrics_daily for %s: %v", dateStr, err)
	}

	elapsed := time.Since(start)
	log.Printf("[metrics] processed daily metrics for %s in %v (deals=%d, categories=%d)",
		dateStr, elapsed, len(dealPerfData), len(catData))
}

// runDailyMetricsLoop runs processDailyMetrics after an initial 10-minute delay,
// then every 24 hours. Computes metrics for yesterday's date.
func (s *Server) runDailyMetricsLoop() {
	// Initial delay: let the server warm up
	time.Sleep(10 * time.Minute)

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	s.processDailyMetrics(yesterday)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		yesterday = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		s.processDailyMetrics(yesterday)
	}
}

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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid deal ID"})
		return
	}

	// Verify the deal belongs to this brand user
	var ownerID int
	err = s.db.QueryRow("SELECT COALESCE(brand_user_id, 0) FROM brand_deals WHERE id = ?", dealID).Scan(&ownerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Deal not found"})
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
