'use strict'

const Database = require('better-sqlite3')

/**
 * Creates and initializes a deck database with core tables.
 *
 * Core tables (always created):
 *   - sessions: phone-based auth tokens
 *   - feedback: viewer feedback with urgency and status
 *   - audit_log: event tracking
 *   - slide_views: per-slide dwell analytics
 *
 * @param {string} dbPath - Path to SQLite file
 * @param {Object} [options]
 * @param {string} [options.extraSchema] - Additional CREATE TABLE / ALTER TABLE SQL to run after core tables
 * @returns {{ db: Database, stmts: Object }}
 */
function createDatabase(dbPath, options = {}) {
  const db = new Database(dbPath, { wal: true })
  db.pragma('journal_mode = WAL')

  // Core tables — shared across all deck instances
  db.exec(`
    CREATE TABLE IF NOT EXISTS sessions (
      token TEXT PRIMARY KEY,
      phone TEXT NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      last_used DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS feedback (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      phone TEXT NOT NULL,
      slide_number INTEGER NOT NULL,
      slide_title TEXT NOT NULL,
      body TEXT NOT NULL,
      urgency TEXT NOT NULL DEFAULT 'normal',
      status TEXT NOT NULL DEFAULT 'new',
      admin_notes TEXT NOT NULL DEFAULT '',
      github_issue_url TEXT NOT NULL DEFAULT '',
      source_url TEXT NOT NULL DEFAULT '',
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
      updated_at DATETIME
    );

    CREATE TABLE IF NOT EXISTS audit_log (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      action TEXT NOT NULL,
      detail TEXT NOT NULL DEFAULT '',
      ip TEXT NOT NULL DEFAULT '',
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS slide_views (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      session_id TEXT NOT NULL,
      slide_number INTEGER NOT NULL,
      slide_title TEXT NOT NULL,
      dwell_ms INTEGER NOT NULL DEFAULT 0,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
  `)

  // Instance-specific schema
  if (options.extraSchema) {
    db.exec(options.extraSchema)
  }

  // Core prepared statements
  const stmts = {
    // Sessions
    insertSession: db.prepare('INSERT INTO sessions (token, phone) VALUES (?, ?)'),
    getSession: db.prepare('SELECT * FROM sessions WHERE token = ?'),
    touchSession: db.prepare('UPDATE sessions SET last_used = CURRENT_TIMESTAMP WHERE token = ?'),

    // Feedback
    insertFeedback: db.prepare('INSERT INTO feedback (phone, slide_number, slide_title, body, urgency) VALUES (?, ?, ?, ?, ?)'),
    getAllFeedback: db.prepare('SELECT * FROM feedback ORDER BY created_at DESC'),
    updateFeedbackStatus: db.prepare('UPDATE feedback SET status = ?, admin_notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?'),
    updateFeedbackGH: db.prepare('UPDATE feedback SET github_issue_url = ? WHERE id = ?'),
    updateFeedbackSource: db.prepare('UPDATE feedback SET source_url = ? WHERE id = ?'),

    // Audit
    insertAudit: db.prepare('INSERT INTO audit_log (action, detail, ip) VALUES (?, ?, ?)'),
    getAudit: db.prepare('SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 100'),

    // Analytics
    insertView: db.prepare('INSERT INTO slide_views (session_id, slide_number, slide_title, dwell_ms) VALUES (?, ?, ?, ?)'),
    getAnalytics: db.prepare(`
      SELECT slide_number, slide_title, COUNT(*) as views, AVG(dwell_ms) as avg_dwell_ms,
             SUM(dwell_ms) as total_dwell_ms, COUNT(DISTINCT session_id) as unique_sessions
      FROM slide_views GROUP BY slide_number ORDER BY slide_number
    `),
    getTotalViews: db.prepare('SELECT COUNT(*) as total FROM slide_views'),
    getUniqueSessions: db.prepare('SELECT COUNT(DISTINCT session_id) as total FROM slide_views'),
    getSessionDetails: db.prepare(`
      SELECT session_id, COUNT(*) as views, SUM(dwell_ms) as total_dwell_ms,
             MIN(created_at) as first_view, MAX(created_at) as last_view,
             COUNT(DISTINCT slide_number) as slides_seen
      FROM slide_views GROUP BY session_id ORDER BY last_view DESC LIMIT 50
    `),
  }

  return { db, stmts }
}

module.exports = { createDatabase }
