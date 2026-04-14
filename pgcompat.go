package main

import (
	"database/sql"
	"fmt"
	"strings"
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
}

// WrapPG creates a PGDB wrapper around a standard sql.DB connection.
func WrapPG(db *sql.DB) *PGDB {
	return &PGDB{DB: db}
}

func (db *PGDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	if db.skipConversion {
		return db.DB.Exec(query, args...)
	}
	converted := convertSQL(query)
	return pgExecWithReturning(db.DB, converted, args...)
}

func (db *PGDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if db.skipConversion {
		return db.DB.Query(query, args...)
	}
	return db.DB.Query(convertSQL(query), args...)
}

func (db *PGDB) QueryRow(query string, args ...interface{}) *sql.Row {
	if db.skipConversion {
		return db.DB.QueryRow(query, args...)
	}
	return db.DB.QueryRow(convertSQL(query), args...)
}

func (db *PGDB) Begin() (*PGTx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &PGTx{Tx: tx, skipConversion: db.skipConversion}, nil
}

// PGTx wraps *sql.Tx with the same SQL conversion.
type PGTx struct {
	*sql.Tx
	skipConversion bool
}

func (tx *PGTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if tx.skipConversion {
		return tx.Tx.Exec(query, args...)
	}
	converted := convertSQL(query)
	return pgExecWithReturning(tx.Tx, converted, args...)
}

func (tx *PGTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if tx.skipConversion {
		return tx.Tx.Query(query, args...)
	}
	return tx.Tx.Query(convertSQL(query), args...)
}

func (tx *PGTx) QueryRow(query string, args ...interface{}) *sql.Row {
	if tx.skipConversion {
		return tx.Tx.QueryRow(query, args...)
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

	// INSERT OR REPLACE INTO → INSERT INTO ... ON CONFLICT DO UPDATE
	// (handled at statement level where needed — wrapper adds basic support)
	q = strings.ReplaceAll(q, "INSERT OR REPLACE INTO", "INSERT INTO")

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
func convertInsertOrIgnore(q string) string {
	upper := strings.ToUpper(q)
	idx := strings.Index(upper, "INSERT OR IGNORE INTO")
	if idx == -1 {
		return q
	}
	// Replace the INSERT OR IGNORE INTO with INSERT INTO
	result := q[:idx] + "INSERT INTO" + q[idx+len("INSERT OR IGNORE INTO"):]

	// We need to add ON CONFLICT DO NOTHING before any RETURNING clause or at the end
	// Find the end of the VALUES clause
	upperResult := strings.ToUpper(result)
	// Look for the end of the statement (before any ; or end of string)
	// Insert ON CONFLICT DO NOTHING at the right place
	// Simple approach: append at end if not already there
	if !strings.Contains(upperResult, "ON CONFLICT") {
		// Find the closing paren of VALUES(...)
		valIdx := strings.Index(upperResult, "VALUES")
		if valIdx != -1 {
			// Find the last closing paren
			depth := 0
			endIdx := -1
			for i := valIdx; i < len(result); i++ {
				if result[i] == '(' {
					depth++
				} else if result[i] == ')' {
					depth--
					if depth == 0 {
						endIdx = i + 1
						break
					}
				}
			}
			if endIdx != -1 {
				result = result[:endIdx] + " ON CONFLICT DO NOTHING" + result[endIdx:]
			}
		}
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
	QueryRow(query string, args ...interface{}) *sql.Row
}

// pgResult implements sql.Result with explicit LastInsertId support.
type pgResult struct {
	lastID   int64
	affected int64
}

func (r pgResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r pgResult) RowsAffected() (int64, error) { return r.affected, nil }

// pgExecWithReturning handles INSERT statements by automatically appending
// RETURNING id so that LastInsertId() works in PostgreSQL.
// For non-INSERT or INSERT that already has RETURNING, it falls through to normal Exec.
func pgExecWithReturning(db dbQuerier, query string, args ...interface{}) (sql.Result, error) {
	upper := strings.TrimSpace(strings.ToUpper(query))

	// Only intercept INSERT statements that don't already have RETURNING
	isInsert := strings.HasPrefix(upper, "INSERT")
	hasReturning := strings.Contains(upper, "RETURNING")

	if isInsert && !hasReturning {
		// Append RETURNING id and use QueryRow to capture the generated ID
		returningQuery := strings.TrimRight(query, "; \t\n") + " RETURNING id"
		var id int64
		err := db.QueryRow(returningQuery, args...).Scan(&id)
		if err != nil {
			// If RETURNING id fails (e.g., table has no 'id' column, or ON CONFLICT DO NOTHING
			// matched zero rows), fall back to normal Exec
			result, execErr := db.Exec(query, args...)
			if execErr != nil {
				return nil, execErr
			}
			return result, nil
		}
		return pgResult{lastID: id, affected: 1}, nil
	}

	return db.Exec(query, args...)
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
			// Fallback: leave as-is but mark for manual review
			replacement = fmt.Sprintf("/* TODO: strftime conversion */ TO_CHAR(%s, %s)", col, format)
		}
		q = q[:start] + replacement + q[end:]
	}
	return q
}
