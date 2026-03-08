package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

// A/B Testing Infrastructure
//
// How it works:
// 1. Create a test:    POST /api/admin/ab-tests  { name, variants, metric, min_sample, measurement_days }
// 2. Assign variant:   GET  /api/ab/variant?test=swap_card_layout  (uses user_id % 100)
// 3. Track event:      POST /api/ab/event  { test, event, value }
// 4. Check results:    GET  /api/admin/ab-tests/:name/results
// 5. Resolve:          POST /api/admin/ab-tests/:name/resolve  { winner }
//
// Automated analysis runs via cron (see scheduled task) and resolves tests
// when sample size is sufficient and statistical significance is reached.

// initABTables creates the A/B testing tables. Call from initDB().
func initABTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ab_tests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			variants TEXT NOT NULL DEFAULT '["control","variant"]',
			key_metric TEXT NOT NULL,
			min_sample_per_variant INTEGER NOT NULL DEFAULT 100,
			measurement_days INTEGER NOT NULL DEFAULT 7,
			traffic_pct INTEGER NOT NULL DEFAULT 50,
			status TEXT NOT NULL DEFAULT 'active',
			winner TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		);

		CREATE TABLE IF NOT EXISTS ab_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			test_name TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			variant TEXT NOT NULL,
			event TEXT NOT NULL,
			value REAL DEFAULT 1.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_ab_events_test ON ab_events(test_name, event);
		CREATE INDEX IF NOT EXISTS idx_ab_events_user ON ab_events(test_name, user_id);
	`)
	if err != nil {
		return err
	}

	// Seed the onboarding A/B test if it doesn't exist
	db.Exec(`INSERT OR IGNORE INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct)
		VALUES ('onboarding_v2', 'New onboarding: brand defaults + item-based household invite',
		'["control","brand_defaults"]', 'items_added_day1', 50, 14, 50)`)

	return nil
}

// ABTest represents an active A/B test
type ABTest struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Variants            []string  `json:"variants"`
	KeyMetric           string    `json:"key_metric"`
	MinSamplePerVariant int       `json:"min_sample_per_variant"`
	MeasurementDays     int       `json:"measurement_days"`
	TrafficPct          int       `json:"traffic_pct"`
	Status              string    `json:"status"`
	Winner              *string   `json:"winner"`
	CreatedAt           time.Time `json:"created_at"`
	ResolvedAt          *time.Time `json:"resolved_at"`
}

// getVariant determines which variant a user sees for a given test.
// Uses user_id % 100 for deterministic assignment.
// Returns empty string if test is not active or user is not in the test.
func getVariant(db *sql.DB, testName string, userID int) (string, error) {
	var variantsJSON string
	var trafficPct int
	var status string
	err := db.QueryRow(
		`SELECT variants, traffic_pct, status FROM ab_tests WHERE name = ?`,
		testName,
	).Scan(&variantsJSON, &trafficPct, &status)
	if err == sql.ErrNoRows {
		return "", nil // Test doesn't exist — default behavior
	}
	if err != nil {
		return "", err
	}
	if status != "active" {
		return "", nil // Test not active
	}

	var variants []string
	if err := json.Unmarshal([]byte(variantsJSON), &variants); err != nil {
		return "", fmt.Errorf("invalid variants JSON: %w", err)
	}
	if len(variants) < 2 {
		return "", fmt.Errorf("test %s has fewer than 2 variants", testName)
	}

	bucket := userID % 100
	if bucket >= trafficPct {
		return variants[0], nil // Control (outside traffic %)
	}

	// Deterministic variant assignment within traffic %
	variantIdx := (userID / 100) % len(variants)
	return variants[variantIdx], nil
}

// trackABEvent records a metric event for an A/B test
func trackABEvent(db *sql.DB, testName string, userID int, event string, value float64) error {
	variant, err := getVariant(db, testName, userID)
	if err != nil || variant == "" {
		return err // Silently skip if test not active
	}

	_, err = db.Exec(
		`INSERT INTO ab_events (test_name, user_id, variant, event, value) VALUES (?, ?, ?, ?, ?)`,
		testName, userID, variant, event, value,
	)
	return err
}

// ABResult holds the analysis results for one variant
type ABResult struct {
	Variant    string  `json:"variant"`
	Count      int     `json:"count"`
	Sum        float64 `json:"sum"`
	Mean       float64 `json:"mean"`
	UniqueUsers int    `json:"unique_users"`
}

// ABAnalysis holds the full analysis for a test
type ABAnalysis struct {
	TestName       string     `json:"test_name"`
	Status         string     `json:"status"`
	Results        []ABResult `json:"results"`
	SufficientData bool       `json:"sufficient_data"`
	Significant    bool       `json:"significant"`
	Recommendation string     `json:"recommendation"`
	DaysSinceStart int        `json:"days_since_start"`
}

// analyzeABTest runs statistical analysis on a test
func analyzeABTest(db *sql.DB, testName string) (*ABAnalysis, error) {
	var test ABTest
	var variantsJSON string
	var winner sql.NullString
	var resolvedAt sql.NullTime
	err := db.QueryRow(
		`SELECT id, name, COALESCE(description,''), variants, key_metric,
		 min_sample_per_variant, measurement_days, traffic_pct, status,
		 winner, created_at, resolved_at
		 FROM ab_tests WHERE name = ?`,
		testName,
	).Scan(&test.ID, &test.Name, &test.Description, &variantsJSON,
		&test.KeyMetric, &test.MinSamplePerVariant, &test.MeasurementDays,
		&test.TrafficPct, &test.Status, &winner, &test.CreatedAt, &resolvedAt)
	if err != nil {
		return nil, err
	}

	var variants []string
	json.Unmarshal([]byte(variantsJSON), &variants)

	daysSinceStart := int(time.Since(test.CreatedAt).Hours() / 24)

	analysis := &ABAnalysis{
		TestName:       testName,
		Status:         test.Status,
		DaysSinceStart: daysSinceStart,
	}

	// Get results per variant for the key metric
	allSufficient := true
	for _, v := range variants {
		var count int
		var sum float64
		var uniqueUsers int
		db.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(value), 0), COUNT(DISTINCT user_id)
			 FROM ab_events WHERE test_name = ? AND variant = ? AND event = ?`,
			testName, v, test.KeyMetric,
		).Scan(&count, &sum, &uniqueUsers)

		mean := 0.0
		if count > 0 {
			mean = sum / float64(count)
		}

		analysis.Results = append(analysis.Results, ABResult{
			Variant:     v,
			Count:       count,
			Sum:         sum,
			Mean:        mean,
			UniqueUsers: uniqueUsers,
		})

		if uniqueUsers < test.MinSamplePerVariant {
			allSufficient = false
		}
	}

	analysis.SufficientData = allSufficient && daysSinceStart >= test.MeasurementDays

	// Simple significance check: if best variant is >10% better than control
	// and we have sufficient data, recommend it
	if analysis.SufficientData && len(analysis.Results) >= 2 {
		control := analysis.Results[0]
		bestIdx := 0
		bestMean := control.Mean
		for i, r := range analysis.Results {
			if r.Mean > bestMean {
				bestMean = r.Mean
				bestIdx = i
			}
		}

		if control.Mean > 0 {
			lift := (bestMean - control.Mean) / control.Mean
			if math.Abs(lift) > 0.10 { // >10% lift
				analysis.Significant = true
				if bestIdx == 0 {
					analysis.Recommendation = fmt.Sprintf("Control wins (%.1f%% better). Revert variant.", lift*100)
				} else {
					analysis.Recommendation = fmt.Sprintf("%s wins with %.1f%% lift. Ship to 100%%.", analysis.Results[bestIdx].Variant, lift*100)
				}
			} else {
				analysis.Recommendation = "No significant difference. Consider extending test or testing a bolder variant."
			}
		} else {
			if bestMean > 0 {
				analysis.Significant = true
				analysis.Recommendation = fmt.Sprintf("%s wins (control has zero events). Ship to 100%%.", analysis.Results[bestIdx].Variant)
			} else {
				analysis.Recommendation = "No events recorded for key metric. Check instrumentation."
			}
		}
	} else if !allSufficient {
		analysis.Recommendation = "Insufficient data. Continue collecting."
	} else {
		analysis.Recommendation = fmt.Sprintf("Measurement period not reached (%d/%d days). Continue.", daysSinceStart, test.MeasurementDays)
	}

	return analysis, nil
}

// --- HTTP Handlers ---

// handleGetVariant returns the variant for the current user
// GET /api/ab/variant?test=test_name
func handleGetVariant(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := s.requireSession(w, r)
		if !ok {
			return
		}

		testName := r.URL.Query().Get("test")
		if testName == "" {
			http.Error(w, "test parameter required", http.StatusBadRequest)
			return
		}

		variant, err := getVariant(s.db, testName, session.UserID)
		if err != nil {
			log.Printf("ab: getVariant error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"variant": variant})
	}
}

// handleTrackABEvent records a metric event
// POST /api/ab/event  { "test": "name", "event": "metric_name", "value": 1.0 }
func handleTrackABEvent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := s.requireSession(w, r)
		if !ok {
			return
		}
		_ = session // used below

		var req struct {
			Test  string  `json:"test"`
			Event string  `json:"event"`
			Value float64 `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Test == "" || req.Event == "" {
			http.Error(w, "test and event required", http.StatusBadRequest)
			return
		}
		if req.Value == 0 {
			req.Value = 1.0
		}

		if err := trackABEvent(s.db, req.Test, session.UserID, req.Event, req.Value); err != nil {
			log.Printf("ab: trackEvent error: %v", err)
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleCreateABTest creates a new A/B test (admin only)
// POST /api/admin/ab-tests
func handleCreateABTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		var req struct {
			Name            string   `json:"name"`
			Description     string   `json:"description"`
			Variants        []string `json:"variants"`
			KeyMetric       string   `json:"key_metric"`
			MinSample       int      `json:"min_sample"`
			MeasurementDays int      `json:"measurement_days"`
			TrafficPct      int      `json:"traffic_pct"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.KeyMetric == "" {
			http.Error(w, "name and key_metric required", http.StatusBadRequest)
			return
		}
		if len(req.Variants) < 2 {
			req.Variants = []string{"control", "variant"}
		}
		if req.MinSample == 0 {
			req.MinSample = 100
		}
		if req.MeasurementDays == 0 {
			req.MeasurementDays = 7
		}
		if req.TrafficPct == 0 {
			req.TrafficPct = 50
		}

		variantsJSON, _ := json.Marshal(req.Variants)
		_, err := s.db.Exec(
			`INSERT INTO ab_tests (name, description, variants, key_metric, min_sample_per_variant, measurement_days, traffic_pct)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			req.Name, req.Description, string(variantsJSON), req.KeyMetric, req.MinSample, req.MeasurementDays, req.TrafficPct,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create test: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": req.Name})
	}
}

// handleABTestResults returns analysis for a test (admin only)
// GET /api/admin/ab-tests/:name/results
func handleABTestResults(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		testName := r.PathValue("name")
		if testName == "" {
			http.Error(w, "test name required", http.StatusBadRequest)
			return
		}

		analysis, err := analyzeABTest(s.db, testName)
		if err != nil {
			http.Error(w, fmt.Sprintf("analysis failed: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, analysis)
	}
}

// handleResolveABTest marks a test as resolved with a winner (admin only)
// POST /api/admin/ab-tests/:name/resolve  { "winner": "variant" }
func handleResolveABTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		testName := r.PathValue("name")
		var req struct {
			Winner string `json:"winner"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Winner == "" {
			http.Error(w, "winner required", http.StatusBadRequest)
			return
		}

		_, err := s.db.Exec(
			`UPDATE ab_tests SET status = 'resolved', winner = ?, resolved_at = CURRENT_TIMESTAMP WHERE name = ?`,
			req.Winner, testName,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("resolve failed: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "resolved", "winner": req.Winner})
	}
}

// handleListABTests lists all A/B tests (admin only)
// GET /api/admin/ab-tests
func handleListABTests(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		status := r.URL.Query().Get("status")
		query := `SELECT id, name, COALESCE(description,''), variants, key_metric,
				  min_sample_per_variant, measurement_days, traffic_pct, status,
				  COALESCE(winner,''), created_at FROM ab_tests`
		args := []any{}
		if status != "" {
			query += " WHERE status = ?"
			args = append(args, status)
		}
		query += " ORDER BY created_at DESC"

		rows, err := s.db.Query(query, args...)
		if err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var tests []map[string]any
		for rows.Next() {
			var id int
			var name, desc, variantsJSON, metric, st, winner string
			var minSample, days, pct int
			var createdAt time.Time
			rows.Scan(&id, &name, &desc, &variantsJSON, &metric, &minSample, &days, &pct, &st, &winner, &createdAt)
			tests = append(tests, map[string]any{
				"id": id, "name": name, "description": desc, "variants": variantsJSON,
				"key_metric": metric, "min_sample": minSample, "measurement_days": days,
				"traffic_pct": pct, "status": st, "winner": winner, "created_at": createdAt,
			})
		}
		if tests == nil {
			tests = []map[string]any{}
		}
		writeJSON(w, http.StatusOK, tests)
	}
}
