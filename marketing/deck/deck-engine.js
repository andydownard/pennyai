'use strict'

const { createDatabase } = require('./lib/db')
const { createAuth } = require('./lib/auth')
const { createFeedback } = require('./lib/feedback')
const { createAnalytics } = require('./lib/analytics')
const { createAudit } = require('./lib/audit')
const { createRateLimiter } = require('./lib/rate-limit')
const { createCors } = require('./lib/cors')
const { setSecurityHeaders, serveStatic } = require('./lib/security')
const { json, parseBody, getIP, normalizePhone, MIME } = require('./lib/http')

/**
 * Create a fully initialized deck engine instance.
 *
 * @param {Object} config
 * @param {string} config.dbPath - Path to SQLite database file
 * @param {Object} [config.twilio] - { accountSid, authToken, verifyServiceSid }
 * @param {string[]} [config.allowedPhones] - Phone whitelist (empty = allow all)
 * @param {string} [config.ghPat] - GitHub PAT for issue creation
 * @param {string} [config.ghRepo] - GitHub repo (e.g., 'andydownard/thrift.ly')
 * @param {string} [config.ghUserAgent] - GitHub API User-Agent
 * @param {string} [config.adminApiKey] - Admin API key
 * @param {RegExp[]} [config.corsOrigins] - CORS allowed origin patterns
 * @param {string} [config.extraSchema] - Additional SQL for instance-specific tables
 * @returns {Object} Initialized engine with all modules
 */
function createDeckEngine(config) {
  const {
    dbPath,
    twilio = {},
    allowedPhones = [],
    ghPat = '',
    ghRepo = '',
    ghUserAgent = 'deck-engine',
    adminApiKey = '',
    corsOrigins,
    extraSchema,
  } = config

  const rateLimit = createRateLimiter()
  const { db, stmts } = createDatabase(dbPath, { extraSchema })

  const auth = createAuth({ stmts, rateLimit, twilio, allowedPhones })
  const feedback = createFeedback({ stmts, rateLimit, ghPat, ghRepo, ghUserAgent, adminApiKey })
  const analytics = createAnalytics({ stmts })
  const audit = createAudit({ stmts })
  const cors = createCors({ allowedOrigins: corsOrigins })

  return {
    // Core modules
    db,
    stmts,
    auth,
    feedback,
    analytics,
    audit,
    cors,
    rateLimit,

    // Utilities
    json,
    parseBody,
    getIP,
    normalizePhone,
    MIME,
    setSecurityHeaders,
    serveStatic,
  }
}

module.exports = { createDeckEngine }

// Also re-export individual modules for granular usage
module.exports.createDatabase = createDatabase
module.exports.createAuth = createAuth
module.exports.createFeedback = createFeedback
module.exports.createAnalytics = createAnalytics
module.exports.createAudit = createAudit
module.exports.createRateLimiter = createRateLimiter
module.exports.createCors = createCors
module.exports.setSecurityHeaders = setSecurityHeaders
module.exports.serveStatic = serveStatic
module.exports.json = json
module.exports.parseBody = parseBody
module.exports.getIP = getIP
module.exports.normalizePhone = normalizePhone
module.exports.MIME = MIME
