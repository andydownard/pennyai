package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

// PGDB wraps *sql.DB to automatically convert SQLite SQL syntax to PostgreSQL.
// This allows the entire codebase to continue using ? placeholders and SQLite
// functions while actually running against PostgreSQL.
//
// Conversions applied:
//   - ? placeholders → $1, $2, $3, ...
//   - datetime('now') → NOW()
//   - datetime('now', '+N days') → NOW() + INTERVAL 'N days'
//   - datetime('now', '-N days') → NOW() - INTERVAL 'N days'
//   - datetime('now', '-N hours') → NOW() - INTERVAL 'N hours'
//   - datetime('now', '-N seconds') → NOW() - INTERVAL 'N seconds'
//   - datetime('now', ?) → NOW() + CAST(? AS INTERVAL) [parameterized]
//   - INSERT OR IGNORE INTO → INSERT INTO ... ON CONFLICT DO NOTHING
//   - INSERT OR REPLACE INTO → handled via ON CONFLICT DO UPDATE
//   - IFNULL(a, b) → COALESCE(a, b)
//   - GROUP_CONCAT(x) → STRING_AGG(x::TEXT, ',')
//   - strftime('%w', col) → EXTRACT(DOW FROM col)
//   - strftime('%Y', col) → EXTRACT(YEAR FROM col)
//   - strftime('%W', col) → EXTRACT(WEEK FROM col)
//   - julianday('now') - julianday(col) → EXTRACT(EPOCH FROM NOW() - col) / 86400
//   - AUTOINCREMENT → (removed, SERIAL handles this)
//   - COLLATE NOCASE → (removed)
//   - INTEGER PRIMARY KEY auto-creates SERIAL
type PGDB struct {
	*sql.DB
	skipConversion bool // when true, pass SQL through without conversion (for SQLite tests)

	// serialTables names every table whose `id` column is an integer, i.e. the
	// tables where LastInsertId() means something. Loaded once from the catalog.
	serialOnce   sync.Once
	serialTables map[string]bool
}

// WrapPG creates a PGDB wrapper around a standard sql.DB connection.
func WrapPG(db *sql.DB) *PGDB {
	return &PGDB{DB: db}
}

// sqliteCompat rewrites Postgres-only syntax to SQLite-compatible equivalents
// when running under skipConversion (in-memory SQLite tests). SQLite's LIKE is
// case-insensitive by default, matching Postgres ILIKE semantics.
func sqliteCompat(query string) string {
	return strings.ReplaceAll(query, "ILIKE", "LIKE")
}

func (db *PGDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	if db.skipConversion {
		return db.DB.Exec(sqliteCompat(query), args...)
	}
	converted := convertSQL(query)
	return pgExecWithReturning(db.DB, db.hasSerialID, converted, args...)
}

func (db *PGDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if db.skipConversion {
		return db.DB.Query(sqliteCompat(query), args...)
	}
	return db.DB.Query(convertSQL(query), args...)
}

func (db *PGDB) QueryRow(query string, args ...interface{}) *sql.Row {
	if db.skipConversion {
		return db.DB.QueryRow(sqliteCompat(query), args...)
	}
	return db.DB.QueryRow(convertSQL(query), args...)
}

func (db *PGDB) Begin() (*PGTx, error) {
	// Read the catalog before the transaction takes a connection. Loading it
	// lazily from inside a transaction needs a second connection while holding
	// the first, which deadlocks once concurrent callers exhaust the pool.
	if !db.skipConversion {
		db.loadSerialTables()
	}
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &PGTx{Tx: tx, skipConversion: db.skipConversion, parent: db}, nil
}

// PGTx wraps *sql.Tx with the same SQL conversion.
type PGTx struct {
	*sql.Tx
	skipConversion bool
	parent         *PGDB
}

func (tx *PGTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if tx.skipConversion {
		return tx.Tx.Exec(sqliteCompat(query), args...)
	}
	converted := convertSQL(query)
	return pgExecWithReturning(tx.Tx, tx.parent.hasSerialID, converted, args...)
}

func (tx *PGTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if tx.skipConversion {
		return tx.Tx.Query(sqliteCompat(query), args...)
	}
	return tx.Tx.Query(convertSQL(query), args...)
}

func (tx *PGTx) QueryRow(query string, args ...interface{}) *sql.Row {
	if tx.skipConversion {
		return tx.Tx.QueryRow(sqliteCompat(query), args...)
	}
	return tx.Tx.QueryRow(convertSQL(query), args...)
}

// Commit delegates to the underlying transaction.
func (tx *PGTx) Commit() error {
	return tx.Tx.Commit()
}

// Rollback delegates to the underlying transaction.
func (tx *PGTx) Rollback() error {
	return tx.Tx.Rollback()
}

// convertSQL applies all SQLite → PostgreSQL transformations to a SQL string.
func convertSQL(query string) string {
	q := query

	// datetime('now') → NOW()
	q = strings.ReplaceAll(q, "datetime('now')", "NOW()")
	q = strings.ReplaceAll(q, `datetime("now")`, "NOW()")

	// datetime('now', '+N days/hours/minutes/seconds') → NOW() + INTERVAL 'N unit'
	// datetime('now', '-N days/hours/minutes/seconds') → NOW() - INTERVAL 'N unit'
	q = convertDatetimeIntervals(q)

	// INSERT OR IGNORE INTO → INSERT INTO ... ON CONFLICT DO NOTHING
	q = convertInsertOrIgnore(q)

	// INSERT OR REPLACE INTO → panic loudly. Silently stripping the OR REPLACE
	// qualifier would turn an upsert into a plain INSERT that crashes on
	// duplicate-key. A real conversion requires per-table ON CONFLICT knowledge
	// the wrapper doesn't have. No active callers as of 2026-04-16; if one is
	// added, the panic fires at first use instead of in production. Rewrite the
	// caller to use explicit ON CONFLICT (...) DO UPDATE SET ... instead.
	if strings.Contains(strings.ToUpper(q), "INSERT OR REPLACE INTO") {
		panic("pgcompat: INSERT OR REPLACE INTO is not supported; rewrite to explicit ON CONFLICT (...) DO UPDATE SET ... — offending query: " + q)
	}

	// IFNULL → COALESCE
	q = strings.ReplaceAll(q, "IFNULL(", "COALESCE(")
	q = strings.ReplaceAll(q, "ifnull(", "COALESCE(")

	// GROUP_CONCAT → STRING_AGG
	q = convertGroupConcat(q)

	// Remove COLLATE NOCASE
	q = strings.ReplaceAll(q, " COLLATE NOCASE", "")

	// Remove AUTOINCREMENT (PostgreSQL SERIAL handles this)
	q = strings.ReplaceAll(q, " AUTOINCREMENT", "")

	// julianday conversions
	q = convertJulianday(q)

	// strftime conversions
	q = convertStrftime(q)

	// date(x), date('now', modifier), date(?, modifier)
	q = convertDateFunc(q)

	// COALESCE(x, '') → COALESCE((x)::TEXT, '')
	q = convertCoalesceEmpty(q)

	// Convert ? placeholders to $1, $2, $3, ...
	q = convertPlaceholders(q)

	return q
}

// convertPlaceholders replaces ? with $1, $2, $3, etc.
// Skips ? inside single-quoted strings.
func convertPlaceholders(query string) string {
	var result strings.Builder
	result.Grow(len(query) + 32)
	n := 1
	inString := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			if inString && i+1 < len(query) && query[i+1] == '\'' {
				// Escaped single quote inside string
				result.WriteByte(ch)
				result.WriteByte(query[i+1])
				i++
				continue
			}
			inString = !inString
			result.WriteByte(ch)
		} else if ch == '?' && !inString {
			result.WriteString(fmt.Sprintf("$%d", n))
			n++
		} else {
			result.WriteByte(ch)
		}
	}
	return result.String()
}

// convertDatetimeIntervals handles patterns like:
//
//	datetime('now', '+7 days')  → NOW() + INTERVAL '7 days'
//	datetime('now', '-30 days') → NOW() - INTERVAL '30 days'
//	datetime('now', '-24 hours') → NOW() - INTERVAL '24 hours'
func convertDatetimeIntervals(q string) string {
	// Pattern: datetime('now', '[+-]N unit')
	for {
		idx := strings.Index(q, "datetime('now',")
		if idx == -1 {
			idx = strings.Index(q, `datetime("now",`)
		}
		if idx == -1 {
			break
		}
		// Find the closing paren
		start := idx
		rest := q[idx:]
		closeIdx := strings.Index(rest, ")")
		if closeIdx == -1 {
			break
		}
		full := rest[:closeIdx+1]

		// Extract the interval part: datetime('now', '+7 days') or datetime('now', '-30 days')
		// Also handle datetime('now', ?) where interval is parameterized
		commaIdx := strings.Index(full, ",")
		if commaIdx == -1 {
			break
		}
		intervalPart := strings.TrimSpace(full[commaIdx+1 : len(full)-1])
		intervalPart = strings.Trim(intervalPart, "'\"")
		intervalPart = strings.TrimSpace(intervalPart)

		if intervalPart == "?" {
			// Parameterized interval — use NOW() + CAST(? AS INTERVAL)
			q = q[:start] + "NOW() + CAST(? AS INTERVAL)" + q[start+len(full):]
			continue
		}

		var op string
		if strings.HasPrefix(intervalPart, "+") {
			op = "+"
			intervalPart = strings.TrimPrefix(intervalPart, "+")
		} else if strings.HasPrefix(intervalPart, "-") {
			op = "-"
			intervalPart = strings.TrimPrefix(intervalPart, "-")
		} else {
			op = "+"
		}

		replacement := fmt.Sprintf("NOW() %s INTERVAL '%s'", op, intervalPart)
		q = q[:start] + replacement + q[start+len(full):]
	}
	return q
}

// convertInsertOrIgnore converts INSERT OR IGNORE INTO → INSERT INTO ... ON CONFLICT DO NOTHING
// Handles both INSERT ... VALUES (...) and INSERT ... SELECT ... forms.
func convertInsertOrIgnore(q string) string {
	upper := strings.ToUpper(q)
	idx := strings.Index(upper, "INSERT OR IGNORE INTO")
	if idx == -1 {
		return q
	}
	// Replace the INSERT OR IGNORE INTO with INSERT INTO
	result := q[:idx] + "INSERT INTO" + q[idx+len("INSERT OR IGNORE INTO"):]

	upperResult := strings.ToUpper(result)
	if strings.Contains(upperResult, "ON CONFLICT") {
		return result
	}

	// Find insertion point: before RETURNING if present, before trailing ;, or at end.
	insertAt := len(result)
	// Strip trailing whitespace for stable placement
	for insertAt > 0 && (result[insertAt-1] == ' ' || result[insertAt-1] == '\n' || result[insertAt-1] == '\t') {
		insertAt--
	}
	// Strip trailing semicolon — we'll re-add it after ON CONFLICT
	hadSemi := false
	if insertAt > 0 && result[insertAt-1] == ';' {
		hadSemi = true
		insertAt--
		for insertAt > 0 && (result[insertAt-1] == ' ' || result[insertAt-1] == '\n' || result[insertAt-1] == '\t') {
			insertAt--
		}
	}
	if retIdx := strings.LastIndex(upperResult[:insertAt], "RETURNING"); retIdx != -1 {
		insertAt = retIdx
		for insertAt > 0 && (result[insertAt-1] == ' ' || result[insertAt-1] == '\n' || result[insertAt-1] == '\t') {
			insertAt--
		}
	}
	conflict := " ON CONFLICT DO NOTHING"
	if hadSemi {
		result = result[:insertAt] + conflict + " " + result[insertAt:]
	} else {
		result = result[:insertAt] + conflict + result[insertAt:]
	}
	return result
}

// convertGroupConcat handles GROUP_CONCAT(expr) → STRING_AGG(expr::TEXT, ',')
func convertGroupConcat(q string) string {
	for {
		lower := strings.ToLower(q)
		idx := strings.Index(lower, "group_concat(")
		if idx == -1 {
			break
		}
		// Find matching closing paren
		start := idx + len("group_concat(")
		depth := 1
		end := start
		for end < len(q) && depth > 0 {
			if q[end] == '(' {
				depth++
			} else if q[end] == ')' {
				depth--
			}
			end++
		}
		inner := q[start : end-1]
		// Check if there's already a separator specified
		if strings.Contains(inner, ",") {
			// GROUP_CONCAT(expr, sep) → STRING_AGG(expr::TEXT, sep)
			parts := strings.SplitN(inner, ",", 2)
			replacement := fmt.Sprintf("STRING_AGG(%s::TEXT, %s)", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			q = q[:idx] + replacement + q[end:]
		} else {
			replacement := fmt.Sprintf("STRING_AGG(%s::TEXT, ',')", inner)
			q = q[:idx] + replacement + q[end:]
		}
	}
	return q
}

// convertJulianday handles julianday('now') - julianday(col) patterns.
func convertJulianday(q string) string {
	// Simple pattern: julianday('now') - julianday(X)
	for {
		lower := strings.ToLower(q)
		idx := strings.Index(lower, "julianday(")
		if idx == -1 {
			break
		}
		// Find the full expression: julianday(...) - julianday(...)
		// or just julianday(...)
		start := idx
		parenStart := idx + len("julianday(")
		depth := 1
		end := parenStart
		for end < len(q) && depth > 0 {
			if q[end] == '(' {
				depth++
			} else if q[end] == ')' {
				depth--
			}
			end++
		}
		arg := q[parenStart : end-1]

		if strings.TrimSpace(strings.ToLower(arg)) == "'now'" {
			// julianday('now') → EXTRACT(EPOCH FROM NOW()) / 86400
			q = q[:start] + "EXTRACT(EPOCH FROM NOW()) / 86400" + q[end:]
		} else {
			// julianday(col) → EXTRACT(EPOCH FROM col) / 86400
			q = q[:start] + fmt.Sprintf("EXTRACT(EPOCH FROM %s) / 86400", arg) + q[end:]
		}
	}
	return q
}

// dbQuerier abstracts *sql.DB and *sql.Tx for pgExecWithReturning.
type dbQuerier interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// pgResult implements sql.Result with explicit LastInsertId support.
type pgResult struct {
	lastID   int64
	affected int64
}

func (r pgResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r pgResult) RowsAffected() (int64, error) { return r.affected, nil }

var insertTableRe = regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\s+"?([a-z_][a-z0-9_]*)"?`)

// loadSerialTables reads, once, which tables have an integer `id` column.
// It runs on the pool, never on a caller's transaction. main() calls it after
// initDB so the first INSERT does not pay for it.
func (db *PGDB) loadSerialTables() {
	db.serialOnce.Do(func() {
		tables := map[string]bool{}
		rows, err := db.DB.Query(
			`SELECT table_name FROM information_schema.columns
			 WHERE table_schema = current_schema()
			   AND column_name = 'id'
			   AND data_type IN ('integer', 'bigint', 'smallint')`)
		if err != nil {
			log.Printf("[pgcompat] could not read the catalog for integer id columns: %v", err)
			db.serialTables = tables
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				tables[strings.ToLower(name)] = true
			}
		}
		db.serialTables = tables
	})
}

// hasSerialID reports whether INSERTs into table can return a generated id.
func (db *PGDB) hasSerialID(table string) bool {
	db.loadSerialTables()
	return db.serialTables[strings.ToLower(table)]
}

// pgExecWithReturning makes LastInsertId() work on PostgreSQL by appending
// RETURNING id — but ONLY for tables known to have an integer id column.
//
// The statement always runs exactly once. The earlier version appended
// RETURNING id to every INSERT and re-ran the INSERT when that failed, which
// (a) inserted twice into tables with a TEXT id, such as users, and (b)
// aborted the enclosing transaction for tables with no id column at all.
//
// A conflict swallowed by ON CONFLICT DO NOTHING returns no rows; that is
// reported as RowsAffected() == 0, which the idempotency guards depend on.
func pgExecWithReturning(db dbQuerier, hasSerialID func(string) bool, query string, args ...interface{}) (sql.Result, error) {
	m := insertTableRe.FindStringSubmatch(query)
	if m == nil || strings.Contains(strings.ToUpper(query), "RETURNING") || !hasSerialID(m[1]) {
		return db.Exec(query, args...)
	}

	rows, err := db.Query(strings.TrimRight(query, "; \t\n")+" RETURNING id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lastID, affected int64
	for rows.Next() {
		if err := rows.Scan(&lastID); err != nil {
			return nil, err
		}
		affected++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pgResult{lastID: lastID, affected: affected}, nil
}

// convertStrftime handles common strftime patterns used in the codebase.
func convertStrftime(q string) string {
	for {
		lower := strings.ToLower(q)
		idx := strings.Index(lower, "strftime(")
		if idx == -1 {
			break
		}
		start := idx
		parenStart := idx + len("strftime(")
		depth := 1
		end := parenStart
		for end < len(q) && depth > 0 {
			if q[end] == '(' {
				depth++
			} else if q[end] == ')' {
				depth--
			}
			end++
		}
		inner := q[parenStart : end-1]
		parts := strings.SplitN(inner, ",", 2)
		if len(parts) != 2 {
			break
		}
		format := strings.TrimSpace(parts[0])
		col := strings.TrimSpace(parts[1])

		var replacement string
		switch format {
		case "'%w'":
			replacement = fmt.Sprintf("EXTRACT(DOW FROM %s)", col)
		case "'%Y'":
			replacement = fmt.Sprintf("EXTRACT(YEAR FROM %s)", col)
		case "'%W'":
			replacement = fmt.Sprintf("EXTRACT(WEEK FROM %s)", col)
		case "'%Y-%m-%d'":
			replacement = fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD')", col)
		case "'%Y-%m'":
			replacement = fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM')", col)
		case "'%H'":
			replacement = fmt.Sprintf("EXTRACT(HOUR FROM %s)", col)
		default:
			// An unrecognised format used to become a TODO comment plus
			// TO_CHAR(col, '%x'), which Postgres executes without error and which
			// yields the literal format string instead of a date part — silently
			// wrong analytics, with nothing in the logs. The conversion stays
			// best-effort (refusing here would break the query outright), but it
			// is now loud, and pgcompat_test.go fails if any format literal in
			// non-test sources reaches this branch at all.
			log.Printf("[pgcompat] UNSUPPORTED strftime format %q — emitting TO_CHAR(%s, %s), which will NOT "+
				"produce a date part. Add a case to convertStrftime.", format, col, format)
			replacement = fmt.Sprintf("TO_CHAR(%s, %s)", col, format)
		}
		q = q[:start] + replacement + q[end:]
	}
	return q
}

// convertCoalesceEmpty rewrites COALESCE(expr, '') to COALESCE((expr)::TEXT, '').
//
// SQLite lets any column fall back to an empty string. PostgreSQL resolves the
// two arguments to one type, so COALESCE(some_timestamp, '') fails with
// "invalid input syntax for type timestamp". The callers all scan the result
// into a string, so casting the first argument to TEXT preserves their intent.
// Only the exact two-argument form with an empty-string fallback is touched.
func convertCoalesceEmpty(q string) string {
	const kw = "COALESCE("
	var out strings.Builder
	out.Grow(len(q) + 16)
	inString := false
	for i := 0; i < len(q); {
		ch := q[i]
		if ch == '\'' {
			inString = !inString
			out.WriteByte(ch)
			i++
			continue
		}
		if inString || i+len(kw) > len(q) || !strings.EqualFold(q[i:i+len(kw)], kw) ||
			(i > 0 && isIdentByte(q[i-1])) {
			out.WriteByte(ch)
			i++
			continue
		}

		args, end, ok := splitCallArgs(q, i+len(kw))
		if !ok {
			out.WriteByte(ch)
			i++
			continue
		}
		for j := range args {
			args[j] = convertCoalesceEmpty(args[j])
		}
		if len(args) == 2 && strings.TrimSpace(args[1]) == "''" {
			first := strings.TrimSpace(args[0])
			if !strings.HasSuffix(strings.ToUpper(first), "::TEXT") {
				lead := args[0][:len(args[0])-len(strings.TrimLeft(args[0], " \t\n"))]
				args[0] = lead + "(" + first + ")::TEXT"
			}
		}
		out.WriteString(q[i : i+len(kw)])
		out.WriteString(strings.Join(args, ","))
		out.WriteByte(')')
		i = end + 1
	}
	return out.String()
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// splitCallArgs splits the arguments of a function call whose opening paren
// has just been consumed (start is the first byte after it). It returns the
// top-level arguments and the index of the matching close paren.
func splitCallArgs(q string, start int) ([]string, int, bool) {
	var args []string
	depth := 0
	inString := false
	argStart := start
	for i := start; i < len(q); i++ {
		ch := q[i]
		switch {
		case ch == '\'':
			inString = !inString
		case inString:
		case ch == '(':
			depth++
		case ch == ')':
			if depth == 0 {
				return append(args, q[argStart:i]), i, true
			}
			depth--
		case ch == ',' && depth == 0:
			args = append(args, q[argStart:i])
			argStart = i + 1
		}
	}
	return nil, 0, false
}

// convertDateFunc translates SQLite's date() function.
//
// SQLite's date() returns 'YYYY-MM-DD' text in UTC, and the schema stores
// calendar dates (trip_date, the *_daily.date columns) as TEXT in that shape,
// so the one- and 'now'-based forms are translated to TO_CHAR(...) text:
//
//	date(x)                 → TO_CHAR((x) AT TIME ZONE 'UTC', 'YYYY-MM-DD')
//	date('now')             → TO_CHAR((NOW() AT TIME ZONE 'UTC'), 'YYYY-MM-DD')
//	date('now', '-2 days')  → TO_CHAR((NOW() AT TIME ZONE 'UTC') + INTERVAL '-2 days', 'YYYY-MM-DD')
//	date('now', ?)          → TO_CHAR((NOW() AT TIME ZONE 'UTC') + CAST(? AS INTERVAL), 'YYYY-MM-DD')
//
// A non-'now' base with a modifier is only ever compared against a timestamp
// column, so it stays a timestamp:
//
//	date(?, '-6 days')      → (CAST(? AS DATE) + INTERVAL '-6 days')
func convertDateFunc(q string) string {
	const kw = "DATE("
	const utcNow = "(NOW() AT TIME ZONE 'UTC')"
	var out strings.Builder
	out.Grow(len(q) + 32)
	inString := false
	for i := 0; i < len(q); {
		ch := q[i]
		if ch == '\'' {
			inString = !inString
			out.WriteByte(ch)
			i++
			continue
		}
		if inString || i+len(kw) > len(q) || !strings.EqualFold(q[i:i+len(kw)], kw) ||
			(i > 0 && isIdentByte(q[i-1])) {
			out.WriteByte(ch)
			i++
			continue
		}
		args, end, ok := splitCallArgs(q, i+len(kw))
		if !ok || len(args) == 0 || len(args) > 2 {
			out.WriteByte(ch)
			i++
			continue
		}
		base := strings.TrimSpace(args[0])
		isNow := strings.EqualFold(base, "'now'")
		modifier := ""
		if len(args) == 2 {
			m := strings.TrimSpace(args[1])
			if m == "?" {
				modifier = " + CAST(? AS INTERVAL)"
			} else {
				modifier = " + INTERVAL " + m
			}
		}
		switch {
		case isNow:
			out.WriteString("TO_CHAR(" + utcNow + modifier + ", 'YYYY-MM-DD')")
		case len(args) == 1:
			out.WriteString("TO_CHAR((" + convertDateFunc(base) + ") AT TIME ZONE 'UTC', 'YYYY-MM-DD')")
		default:
			out.WriteString("(CAST(" + base + " AS DATE)" + modifier + ")")
		}
		i = end + 1
	}
	return out.String()
}
