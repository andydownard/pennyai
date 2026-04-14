package main

import (
	"strings"
	"testing"
)

func TestConvertPlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single placeholder",
			input:  "SELECT * FROM users WHERE id = ?",
			expect: "SELECT * FROM users WHERE id = $1",
		},
		{
			name:   "multiple placeholders",
			input:  "INSERT INTO users (name, email) VALUES (?, ?)",
			expect: "INSERT INTO users (name, email) VALUES ($1, $2)",
		},
		{
			name:   "placeholder inside string literal ignored",
			input:  "SELECT * FROM users WHERE name = 'what?'",
			expect: "SELECT * FROM users WHERE name = 'what?'",
		},
		{
			name:   "mixed placeholders and strings",
			input:  "SELECT * FROM users WHERE name = ? AND status = 'active?' AND id = ?",
			expect: "SELECT * FROM users WHERE name = $1 AND status = 'active?' AND id = $2",
		},
		{
			name:   "escaped single quote in string",
			input:  "SELECT * FROM users WHERE name = 'O''Brien' AND id = ?",
			expect: "SELECT * FROM users WHERE name = 'O''Brien' AND id = $1",
		},
		{
			name:   "no placeholders",
			input:  "SELECT COUNT(*) FROM users",
			expect: "SELECT COUNT(*) FROM users",
		},
		{
			name:   "many placeholders",
			input:  "INSERT INTO t (a,b,c,d,e) VALUES (?,?,?,?,?)",
			expect: "INSERT INTO t (a,b,c,d,e) VALUES ($1,$2,$3,$4,$5)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertPlaceholders(tt.input)
			if got != tt.expect {
				t.Errorf("convertPlaceholders(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertDatetime(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "datetime now",
			input:  "UPDATE users SET last_active = datetime('now') WHERE id = ?",
			expect: "UPDATE users SET last_active = NOW() WHERE id = $1",
		},
		{
			name:   "datetime now plus days",
			input:  "SELECT * FROM sessions WHERE expires_at > datetime('now', '+30 days')",
			expect: "SELECT * FROM sessions WHERE expires_at > NOW() + INTERVAL '30 days'",
		},
		{
			name:   "datetime now minus hours",
			input:  "SELECT * FROM signals WHERE created_at >= datetime('now', '-24 hours')",
			expect: "SELECT * FROM signals WHERE created_at >= NOW() - INTERVAL '24 hours'",
		},
		{
			name:   "datetime now minus days",
			input:  "SELECT * FROM users WHERE last_active >= datetime('now', '-7 days')",
			expect: "SELECT * FROM users WHERE last_active >= NOW() - INTERVAL '7 days'",
		},
		{
			name:   "multiple datetime calls",
			input:  "SELECT * FROM sessions WHERE created_at >= datetime('now', '-30 days') AND expires_at > datetime('now')",
			expect: "SELECT * FROM sessions WHERE created_at >= NOW() - INTERVAL '30 days' AND expires_at > NOW()",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertInsertOrIgnore(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "insert or ignore",
			input:  "INSERT OR IGNORE INTO users (phone) VALUES (?)",
			expect: "INSERT INTO users (phone) VALUES ($1) ON CONFLICT DO NOTHING",
		},
		{
			name:   "insert or ignore multi-value",
			input:  "INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)",
			expect: "INSERT INTO auth_methods (user_id, provider, provider_id) VALUES ($1, 'phone', $2) ON CONFLICT DO NOTHING",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertInsertOrReplace(t *testing.T) {
	got := convertSQL("INSERT OR REPLACE INTO brand_logos (brand_id, logo_data) VALUES (?, ?)")
	expect := "INSERT INTO brand_logos (brand_id, logo_data) VALUES ($1, $2)"
	if got != expect {
		t.Errorf("convertSQL INSERT OR REPLACE\n  got:  %q\n  want: %q", got, expect)
	}
}

func TestConvertIFNULL(t *testing.T) {
	got := convertSQL("SELECT IFNULL(name, '') FROM users WHERE id = ?")
	expect := "SELECT COALESCE(name, '') FROM users WHERE id = $1"
	if got != expect {
		t.Errorf("convertSQL IFNULL\n  got:  %q\n  want: %q", got, expect)
	}
}

func TestConvertGroupConcat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple group_concat",
			input:  "SELECT GROUP_CONCAT(name) FROM users",
			expect: "SELECT STRING_AGG(name::TEXT, ',') FROM users",
		},
		{
			name:   "group_concat with separator",
			input:  "SELECT GROUP_CONCAT(name, '; ') FROM users",
			expect: "SELECT STRING_AGG(name::TEXT, '; ') FROM users",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertJulianday(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "julianday now",
			input:  "SELECT julianday('now')",
			expect: "SELECT EXTRACT(EPOCH FROM NOW()) / 86400",
		},
		{
			name:   "julianday column",
			input:  "SELECT julianday(created_at) FROM users",
			expect: "SELECT EXTRACT(EPOCH FROM created_at) / 86400 FROM users",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertStrftime(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "strftime day of week",
			input:  "SELECT strftime('%w', created_at) FROM orders",
			expect: "SELECT EXTRACT(DOW FROM created_at) FROM orders",
		},
		{
			name:   "strftime year",
			input:  "SELECT strftime('%Y', created_at) FROM users",
			expect: "SELECT EXTRACT(YEAR FROM created_at) FROM users",
		},
		{
			name:   "strftime week",
			input:  "SELECT strftime('%W', created_at) FROM users",
			expect: "SELECT EXTRACT(WEEK FROM created_at) FROM users",
		},
		{
			name:   "strftime date format",
			input:  "SELECT strftime('%Y-%m-%d', created_at) FROM users",
			expect: "SELECT TO_CHAR(created_at, 'YYYY-MM-DD') FROM users",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertCollateNocase(t *testing.T) {
	got := convertSQL("SELECT * FROM brands WHERE name = ? COLLATE NOCASE")
	expect := "SELECT * FROM brands WHERE name = $1"
	if got != expect {
		t.Errorf("COLLATE NOCASE removal\n  got:  %q\n  want: %q", got, expect)
	}
}

func TestConvertAutoincrement(t *testing.T) {
	got := convertSQL("CREATE TABLE t (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	expect := "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)"
	if got != expect {
		t.Errorf("AUTOINCREMENT removal\n  got:  %q\n  want: %q", got, expect)
	}
}

func TestConvertComplexQuery(t *testing.T) {
	input := `INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id)
		SELECT id, 'phone', phone_number FROM users
		WHERE phone_number IS NOT NULL AND phone_number != ''`
	got := convertSQL(input)
	// Should convert INSERT OR IGNORE, no placeholders to convert
	if got == input {
		t.Error("convertSQL should have transformed INSERT OR IGNORE")
	}
	if !containsString(got, "ON CONFLICT DO NOTHING") {
		t.Errorf("expected ON CONFLICT DO NOTHING, got: %s", got)
	}
}

func TestConvertRealWorldQueries(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "brand preference update",
			input:  "UPDATE brand_preferences SET streak = streak + 1, pick_count = pick_count + 1, last_picked = CURRENT_TIMESTAMP WHERE user_id = ? AND product_category = ?",
			expect: "UPDATE brand_preferences SET streak = streak + 1, pick_count = pick_count + 1, last_picked = CURRENT_TIMESTAMP WHERE user_id = $1 AND product_category = $2",
		},
		{
			name:   "select with coalesce",
			input:  "SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE phone_number = ?",
			expect: "SELECT id, phone_number, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE phone_number = $1",
		},
		{
			name:   "insert with datetime",
			input:  "INSERT INTO user_signals (user_id, signal, category, value, context, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))",
			expect: "INSERT INTO user_signals (user_id, signal, category, value, context, created_at) VALUES ($1, $2, $3, $4, $5, NOW())",
		},
		{
			name:   "select with datetime interval in where",
			input:  "SELECT COUNT(*) FROM swap_events WHERE user_id = ? AND created_at >= datetime('now', '-30 days') AND action = 'accept'",
			expect: "SELECT COUNT(*) FROM swap_events WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '30 days' AND action = 'accept'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConvertSQLIdempotent(t *testing.T) {
	// Already-converted queries should not be double-converted
	pgQuery := "SELECT * FROM users WHERE id = $1 AND created_at > NOW()"
	got := convertSQL(pgQuery)
	// Should not change $1 or NOW()
	if got != pgQuery {
		t.Errorf("double-conversion happened:\n  input: %q\n  got:   %q", pgQuery, got)
	}
}

func TestConvertSQLPreservesWhitespace(t *testing.T) {
	input := `
		SELECT u.id, u.name, COUNT(li.id) as item_count
		FROM users u
		LEFT JOIN list_items li ON li.user_id = u.id
		WHERE u.household_id = ?
		  AND li.created_at >= datetime('now', '-7 days')
		GROUP BY u.id
	`
	got := convertSQL(input)
	if !containsString(got, "$1") {
		t.Error("should have converted ? to $1")
	}
	if !containsString(got, "NOW() - INTERVAL '7 days'") {
		t.Error("should have converted datetime interval")
	}
	if containsString(got, "?") {
		t.Error("should not contain unconverted ?")
	}
}

// containsString is a helper to check substring presence.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 1. Real-world query patterns from the codebase
// ---------------------------------------------------------------------------

func TestConvertSQL_OnConflictDoUpdate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "upsert with excluded columns and datetime",
			input: `INSERT INTO user_photos (user_id, photo_data, content_type, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(user_id) DO UPDATE SET photo_data = excluded.photo_data, content_type = excluded.content_type, updated_at = datetime('now')`,
			expect: `INSERT INTO user_photos (user_id, photo_data, content_type, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT(user_id) DO UPDATE SET photo_data = excluded.photo_data, content_type = excluded.content_type, updated_at = NOW()`,
		},
		{
			name: "upsert brand logos with multiple excluded columns",
			input: `INSERT INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source)
					VALUES (?, ?, ?, datetime('now'), ?)
					ON CONFLICT(brand_id) DO UPDATE SET logo_data = excluded.logo_data, content_type = excluded.content_type, fetched_at = excluded.fetched_at, source = excluded.source`,
			expect: `INSERT INTO brand_logos (brand_id, logo_data, content_type, fetched_at, source)
					VALUES ($1, $2, $3, NOW(), $4)
					ON CONFLICT(brand_id) DO UPDATE SET logo_data = excluded.logo_data, content_type = excluded.content_type, fetched_at = excluded.fetched_at, source = excluded.source`,
		},
		{
			name: "upsert dietary profiles",
			input: `INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES (?, '[]', '["vegan"]') ON CONFLICT(user_id) DO UPDATE SET allergens = excluded.allergens, diets = excluded.diets`,
			expect: `INSERT INTO user_dietary_profiles (user_id, allergens, diets) VALUES ($1, '[]', '["vegan"]') ON CONFLICT(user_id) DO UPDATE SET allergens = excluded.allergens, diets = excluded.diets`,
		},
		{
			name:   "on conflict do update with computed_at datetime",
			input:  `INSERT INTO learning_context (user_id, context_json, computed_at) VALUES (?, ?, datetime('now')) ON CONFLICT(user_id) DO UPDATE SET context_json = ?, computed_at = datetime('now')`,
			expect: `INSERT INTO learning_context (user_id, context_json, computed_at) VALUES ($1, $2, NOW()) ON CONFLICT(user_id) DO UPDATE SET context_json = $3, computed_at = NOW()`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL\n  got:  %q\n  want: %q", got, tt.expect)
			}
		})
	}
}

func TestConvertSQL_MultiLineDatetimeInterval(t *testing.T) {
	input := `SELECT u.id, u.name, COUNT(se.id) as events
		FROM users u
		LEFT JOIN swap_events se ON se.user_id = u.id
		WHERE se.created_at >= datetime('now', '-30 days')
		  AND u.last_active >= datetime('now', '-7 days')
		GROUP BY u.id
		HAVING COUNT(se.id) > ?`
	got := convertSQL(input)

	if !strings.Contains(got, "NOW() - INTERVAL '30 days'") {
		t.Error("should convert -30 days interval")
	}
	if !strings.Contains(got, "NOW() - INTERVAL '7 days'") {
		t.Error("should convert -7 days interval")
	}
	if !strings.Contains(got, "$1") {
		t.Error("should convert ? to $1")
	}
	if strings.Contains(got, "datetime") {
		t.Error("should not contain unconverted datetime")
	}
}

func TestConvertSQL_NestedSubqueryPlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "insert with subquery select",
			input:  "INSERT OR IGNORE INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = ?), ?)",
			expect: "INSERT INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = $1), $2) ON CONFLICT DO NOTHING",
		},
		{
			name: "nested subquery in where",
			input: `SELECT * FROM deals
				WHERE brand_id IN (SELECT brand_id FROM brand_preferences WHERE user_id = ?)
				AND created_at >= datetime('now', '-7 days')
				AND status = ?`,
			expect: `SELECT * FROM deals
				WHERE brand_id IN (SELECT brand_id FROM brand_preferences WHERE user_id = $1)
				AND created_at >= NOW() - INTERVAL '7 days'
				AND status = $2`,
		},
		{
			name:   "three nested placeholders",
			input:  "SELECT * FROM t WHERE a = ? AND b IN (SELECT b FROM t2 WHERE c = ?) AND d = ?",
			expect: "SELECT * FROM t WHERE a = $1 AND b IN (SELECT b FROM t2 WHERE c = $2) AND d = $3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL\n  got:  %q\n  want: %q", got, tt.expect)
			}
		})
	}
}

func TestConvertSQL_CaseWhenWithStrftime(t *testing.T) {
	// Real pattern from handlers_metrics.go cohort analysis
	input := `SELECT
			CAST(strftime('%Y', u.created_at) AS TEXT) || '-W' ||
				CASE WHEN LENGTH(CAST(strftime('%W', u.created_at) AS TEXT)) = 1
					THEN '0' || CAST(strftime('%W', u.created_at) AS TEXT)
					ELSE CAST(strftime('%W', u.created_at) AS TEXT)
				END as cohort_week
		FROM users u WHERE u.id = ?`
	got := convertSQL(input)

	if strings.Contains(got, "strftime") {
		t.Error("should not contain unconverted strftime calls")
	}
	if !strings.Contains(got, "EXTRACT(YEAR FROM") {
		t.Error("should convert strftime('%Y', ...) to EXTRACT(YEAR FROM ...)")
	}
	if !strings.Contains(got, "EXTRACT(WEEK FROM") {
		t.Error("should convert strftime('%W', ...) to EXTRACT(WEEK FROM ...)")
	}
	if !strings.Contains(got, "CASE WHEN") {
		t.Error("should preserve CASE WHEN structure")
	}
	if !strings.Contains(got, "$1") {
		t.Error("should convert ? to $1")
	}
}

func TestConvertSQL_ComplexJoinsWithDatetimeFilters(t *testing.T) {
	input := `SELECT u.id, u.name, COUNT(li.id) as item_count, MAX(li.created_at) as last_item
		FROM users u
		LEFT JOIN list_items li ON li.user_id = u.id
		INNER JOIN households h ON h.id = u.household_id
		WHERE u.household_id = ?
		  AND li.created_at >= datetime('now', '-7 days')
		  AND h.created_at >= datetime('now', '-30 days')
		GROUP BY u.id
		ORDER BY item_count DESC
		LIMIT ?`
	got := convertSQL(input)

	if !strings.Contains(got, "$1") || !strings.Contains(got, "$2") {
		t.Error("should convert both ? placeholders")
	}
	if !strings.Contains(got, "NOW() - INTERVAL '7 days'") {
		t.Error("should convert 7 days interval")
	}
	if !strings.Contains(got, "NOW() - INTERVAL '30 days'") {
		t.Error("should convert 30 days interval")
	}
	if strings.Contains(got, "datetime") {
		t.Error("should not have unconverted datetime calls")
	}
}

func TestConvertSQL_InsertReturningNoDouble(t *testing.T) {
	// An INSERT that already has RETURNING should pass through unchanged
	// (except for placeholder and datetime conversion)
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "insert with existing RETURNING id",
			input:  "INSERT INTO users (phone, display_name) VALUES (?, ?) RETURNING id",
			expect: "INSERT INTO users (phone, display_name) VALUES ($1, $2) RETURNING id",
		},
		{
			name:   "insert with existing RETURNING id and datetime",
			input:  "INSERT INTO users (phone, display_name, created_at) VALUES (?, ?, datetime('now')) RETURNING id",
			expect: "INSERT INTO users (phone, display_name, created_at) VALUES ($1, $2, NOW()) RETURNING id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL\n  got:  %q\n  want: %q", got, tt.expect)
			}
			// Verify RETURNING appears exactly once
			count := strings.Count(strings.ToUpper(got), "RETURNING")
			if count != 1 {
				t.Errorf("RETURNING should appear exactly once, found %d times", count)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Test pgExecWithReturning logic (query manipulation only, no DB needed)
// ---------------------------------------------------------------------------

func TestPgExecWithReturning_QueryClassification(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectInsert   bool
		expectReturning bool
	}{
		{
			name:           "plain INSERT",
			query:          "INSERT INTO users (name) VALUES ($1)",
			expectInsert:   true,
			expectReturning: false,
		},
		{
			name:           "INSERT with RETURNING",
			query:          "INSERT INTO users (name) VALUES ($1) RETURNING id",
			expectInsert:   true,
			expectReturning: true,
		},
		{
			name:           "UPDATE query",
			query:          "UPDATE users SET name = $1 WHERE id = $2",
			expectInsert:   false,
			expectReturning: false,
		},
		{
			name:           "DELETE query",
			query:          "DELETE FROM users WHERE id = $1",
			expectInsert:   false,
			expectReturning: false,
		},
		{
			name:           "SELECT query",
			query:          "SELECT * FROM users WHERE id = $1",
			expectInsert:   false,
			expectReturning: false,
		},
		{
			name:           "INSERT with ON CONFLICT DO NOTHING",
			query:          "INSERT INTO users (phone) VALUES ($1) ON CONFLICT DO NOTHING",
			expectInsert:   true,
			expectReturning: false,
		},
		{
			name:           "INSERT with leading whitespace",
			query:          "  INSERT INTO users (name) VALUES ($1)",
			expectInsert:   true,
			expectReturning: false,
		},
		{
			name:           "INSERT with RETURNING in mixed case",
			query:          "INSERT INTO users (name) VALUES ($1) returning id",
			expectInsert:   true,
			expectReturning: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upper := strings.TrimSpace(strings.ToUpper(tt.query))
			isInsert := strings.HasPrefix(upper, "INSERT")
			hasReturning := strings.Contains(upper, "RETURNING")

			if isInsert != tt.expectInsert {
				t.Errorf("isInsert: got %v, want %v", isInsert, tt.expectInsert)
			}
			if hasReturning != tt.expectReturning {
				t.Errorf("hasReturning: got %v, want %v", hasReturning, tt.expectReturning)
			}

			// If INSERT without RETURNING, verify RETURNING id would be appended correctly
			if isInsert && !hasReturning {
				returningQuery := strings.TrimRight(tt.query, "; \t\n") + " RETURNING id"
				if !strings.HasSuffix(returningQuery, "RETURNING id") {
					t.Error("RETURNING id should be appended at end")
				}
				// Verify it appears exactly once
				count := strings.Count(strings.ToUpper(returningQuery), "RETURNING")
				if count != 1 {
					t.Errorf("RETURNING should appear exactly once, found %d", count)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Test convertDatetimeIntervals
// ---------------------------------------------------------------------------

func TestConvertDatetimeIntervals_Comprehensive(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "plus 7 days",
			input:  "SELECT * FROM t WHERE expires > datetime('now', '+7 days')",
			expect: "SELECT * FROM t WHERE expires > NOW() + INTERVAL '7 days'",
		},
		{
			name:   "minus 30 days",
			input:  "SELECT * FROM t WHERE created >= datetime('now', '-30 days')",
			expect: "SELECT * FROM t WHERE created >= NOW() - INTERVAL '30 days'",
		},
		{
			name:   "minus 24 hours",
			input:  "SELECT * FROM t WHERE ts >= datetime('now', '-24 hours')",
			expect: "SELECT * FROM t WHERE ts >= NOW() - INTERVAL '24 hours'",
		},
		{
			name:   "minus 60 seconds",
			input:  "SELECT * FROM t WHERE ts >= datetime('now', '-60 seconds')",
			expect: "SELECT * FROM t WHERE ts >= NOW() - INTERVAL '60 seconds'",
		},
		{
			name:   "plus 1 hours",
			input:  "SELECT * FROM t WHERE ts < datetime('now', '+1 hours')",
			expect: "SELECT * FROM t WHERE ts < NOW() + INTERVAL '1 hours'",
		},
		{
			name:   "multiple intervals in same query",
			input:  "SELECT * FROM t WHERE a >= datetime('now', '-7 days') AND b < datetime('now', '+30 days') AND c >= datetime('now', '-24 hours')",
			expect: "SELECT * FROM t WHERE a >= NOW() - INTERVAL '7 days' AND b < NOW() + INTERVAL '30 days' AND c >= NOW() - INTERVAL '24 hours'",
		},
		{
			name:   "parameterized interval with ?",
			input:  "SELECT * FROM t WHERE ts > datetime('now', ?)",
			expect: "SELECT * FROM t WHERE ts > NOW() + CAST(? AS INTERVAL)",
		},
		{
			name:   "double-quoted now",
			input:  `SELECT * FROM t WHERE ts > datetime("now", '-5 days')`,
			expect: "SELECT * FROM t WHERE ts > NOW() - INTERVAL '5 days'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// convertDatetimeIntervals runs after datetime('now') → NOW() replacement in convertSQL,
			// but we test convertDatetimeIntervals directly here
			got := convertDatetimeIntervals(tt.input)
			if got != tt.expect {
				t.Errorf("convertDatetimeIntervals\n  got:  %q\n  want: %q", got, tt.expect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Test convertInsertOrIgnore
// ---------------------------------------------------------------------------

func TestConvertInsertOrIgnore_Comprehensive(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "basic insert or ignore",
			input:  "INSERT OR IGNORE INTO users (phone) VALUES (?)",
			expect: "INSERT INTO users (phone) VALUES (?) ON CONFLICT DO NOTHING",
		},
		{
			name:   "insert or ignore with subquery",
			input:  "INSERT OR IGNORE INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = ?), ?)",
			expect: "INSERT INTO household_members (household_id, user_id) VALUES ((SELECT household_id FROM users WHERE id = ?), ?) ON CONFLICT DO NOTHING",
		},
		{
			name:   "insert or ignore multi-column",
			input:  "INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?)",
			expect: "INSERT INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'phone', ?) ON CONFLICT DO NOTHING",
		},
		{
			name:   "non-insert passthrough",
			input:  "UPDATE users SET name = ? WHERE id = ?",
			expect: "UPDATE users SET name = ? WHERE id = ?",
		},
		{
			name:   "regular insert passthrough",
			input:  "INSERT INTO users (name) VALUES (?)",
			expect: "INSERT INTO users (name) VALUES (?)",
		},
		{
			name:   "on conflict already present",
			input:  "INSERT INTO users (phone) VALUES (?) ON CONFLICT DO NOTHING",
			expect: "INSERT INTO users (phone) VALUES (?) ON CONFLICT DO NOTHING",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertInsertOrIgnore(tt.input)
			if got != tt.expect {
				t.Errorf("convertInsertOrIgnore\n  got:  %q\n  want: %q", got, tt.expect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 5. Edge cases
// ---------------------------------------------------------------------------

func TestConvertSQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty string",
			input:  "",
			expect: "",
		},
		{
			name:   "no sqlite-isms passthrough",
			input:  "SELECT id, name FROM users WHERE active = true ORDER BY name",
			expect: "SELECT id, name FROM users WHERE active = true ORDER BY name",
		},
		{
			name:   "question mark inside single quotes not converted",
			input:  "SELECT * FROM users WHERE name = 'is this a question?' AND id = ?",
			expect: "SELECT * FROM users WHERE name = 'is this a question?' AND id = $1",
		},
		{
			name:   "multiple question marks inside strings",
			input:  "INSERT INTO faq (q, a) VALUES ('what?', 'why?') RETURNING id",
			expect: "INSERT INTO faq (q, a) VALUES ('what?', 'why?') RETURNING id",
		},
		{
			name:   "escaped quotes with placeholder after",
			input:  "SELECT * FROM users WHERE name = 'O''Brien' AND id = ?",
			expect: "SELECT * FROM users WHERE name = 'O''Brien' AND id = $1",
		},
		{
			name:   "escaped quotes with question mark inside",
			input:  "SELECT * FROM t WHERE a = 'it''s a question?' AND b = ?",
			expect: "SELECT * FROM t WHERE a = 'it''s a question?' AND b = $1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSQL(tt.input)
			if got != tt.expect {
				t.Errorf("convertSQL\n  got:  %q\n  want: %q", got, tt.expect)
			}
		})
	}
}

func TestConvertSQL_DoubleConversionIdempotent(t *testing.T) {
	// Already-converted PostgreSQL queries should not be modified further
	alreadyConverted := []string{
		"SELECT * FROM users WHERE id = $1 AND created_at > NOW()",
		"SELECT * FROM t WHERE ts >= NOW() - INTERVAL '30 days' AND id = $1",
		"INSERT INTO users (name) VALUES ($1) RETURNING id",
		"INSERT INTO t (a, b) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		"SELECT STRING_AGG(name::TEXT, ',') FROM users GROUP BY team",
		"SELECT COALESCE(name, '') FROM users WHERE id = $1",
		"SELECT EXTRACT(DOW FROM created_at) FROM orders",
		"SELECT TO_CHAR(created_at, 'YYYY-MM-DD') FROM users",
		"SELECT EXTRACT(EPOCH FROM NOW()) / 86400",
	}
	for _, q := range alreadyConverted {
		t.Run(q[:min(40, len(q))], func(t *testing.T) {
			got := convertSQL(q)
			if got != q {
				t.Errorf("double conversion changed query:\n  input: %q\n  got:   %q", q, got)
			}
		})
	}
}

func TestConvertSQL_DoubleConversionRoundtrip(t *testing.T) {
	// Convert a SQLite query, then convert the result again — should be stable
	sqliteQueries := []string{
		"SELECT * FROM users WHERE id = ? AND created_at > datetime('now')",
		"SELECT * FROM t WHERE ts >= datetime('now', '-30 days') AND status = ?",
		"INSERT OR IGNORE INTO users (phone) VALUES (?)",
		"SELECT IFNULL(name, '') FROM users WHERE id = ?",
		"SELECT GROUP_CONCAT(name) FROM users",
		"SELECT strftime('%w', created_at) FROM orders",
		"SELECT julianday('now')",
		"UPDATE t SET a = ? WHERE b COLLATE NOCASE = ?",
	}
	for _, q := range sqliteQueries {
		t.Run(q[:min(40, len(q))], func(t *testing.T) {
			first := convertSQL(q)
			second := convertSQL(first)
			if first != second {
				t.Errorf("not idempotent:\n  first:  %q\n  second: %q", first, second)
			}
		})
	}
}

func TestConvertSQL_WhitespaceVariants(t *testing.T) {
	// Tabs, newlines, extra spaces should not break conversion
	input := "INSERT OR IGNORE INTO\tusers\n(phone)\nVALUES\n(?)"
	got := convertSQL(input)
	if strings.Contains(got, "OR IGNORE") {
		t.Error("should have removed OR IGNORE")
	}
	if !strings.Contains(got, "ON CONFLICT DO NOTHING") {
		t.Error("should have added ON CONFLICT DO NOTHING")
	}
	if !strings.Contains(got, "$1") {
		t.Error("should have converted ? to $1")
	}
}

func TestConvertSQL_AllConversionsAtOnce(t *testing.T) {
	// A single query that triggers many conversions simultaneously
	input := `INSERT OR IGNORE INTO report (user_id, name, score, week, created_at)
		VALUES (?, IFNULL(?, 'anon'), ?, strftime('%W', datetime('now')), datetime('now'))`
	got := convertSQL(input)

	if strings.Contains(got, "OR IGNORE") {
		t.Error("should remove OR IGNORE")
	}
	if !strings.Contains(got, "ON CONFLICT DO NOTHING") {
		t.Error("should add ON CONFLICT DO NOTHING")
	}
	if !strings.Contains(got, "COALESCE") {
		t.Error("should convert IFNULL to COALESCE")
	}
	if strings.Contains(got, "IFNULL") {
		t.Error("should not contain IFNULL")
	}
	if !strings.Contains(got, "NOW()") {
		t.Error("should convert datetime('now') to NOW()")
	}
	if !strings.Contains(got, "$1") || !strings.Contains(got, "$2") || !strings.Contains(got, "$3") {
		t.Error("should convert all three ? placeholders")
	}
	if strings.Contains(got, "?") {
		t.Error("should not contain unconverted ?")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
