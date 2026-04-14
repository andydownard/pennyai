'use strict'

/**
 * In-memory rate limiter with sliding time windows.
 * Returns a rateLimit function bound to an internal Map.
 *
 * Usage:
 *   const { rateLimit } = require('@thriftly/deck-engine/rate-limit')
 *   const limiter = rateLimit()
 *   if (limiter('auth-' + ip, 10, 900000)) return json(res, { error: 'Too many attempts' }, 429)
 */

function createRateLimiter() {
  const entries = new Map()

  // Cleanup stale entries every 5 minutes
  setInterval(() => {
    const now = Date.now()
    for (const [key, entry] of entries) {
      if (now - entry.start > entry.windowMs * 2) entries.delete(key)
    }
  }, 300000).unref()

  /**
   * @param {string} key - Unique identifier (e.g., 'auth-127.0.0.1')
   * @param {number} maxHits - Max requests allowed in window
   * @param {number} windowMs - Window duration in milliseconds
   * @returns {boolean} true if rate limit exceeded
   */
  function check(key, maxHits, windowMs) {
    const now = Date.now()
    const entry = entries.get(key) || { hits: 0, start: now, windowMs }
    if (now - entry.start > windowMs) { entry.hits = 0; entry.start = now }
    entry.hits++
    entry.windowMs = windowMs
    entries.set(key, entry)
    return entry.hits > maxHits
  }

  return check
}

module.exports = { createRateLimiter }
