'use strict'

const crypto = require('crypto')

/**
 * Creates auth handlers for phone-based SMS authentication via Twilio Verify.
 *
 * @param {Object} config
 * @param {Object} config.stmts - Prepared statements from db.js (insertSession, getSession, touchSession, insertAudit)
 * @param {Function} config.rateLimit - Rate limiter function from rate-limit.js
 * @param {Object} config.twilio - { accountSid, authToken, verifyServiceSid }
 * @param {string[]} [config.allowedPhones] - Optional phone whitelist (empty = allow all)
 */
function createAuth(config) {
  const { stmts, rateLimit, twilio = {}, allowedPhones = [] } = config

  async function twilioRequest(endpoint, body) {
    if (!twilio.accountSid || !twilio.verifyServiceSid) return null
    const auth = Buffer.from(`${twilio.accountSid}:${twilio.authToken}`).toString('base64')
    const res = await fetch(
      `https://verify.twilio.com/v2/Services/${twilio.verifyServiceSid}/${endpoint}`,
      {
        method: 'POST',
        headers: { 'Authorization': `Basic ${auth}`, 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams(body).toString(),
      }
    )
    return res.json()
  }

  function createSessionToken(phone, ip) {
    const token = crypto.randomBytes(32).toString('hex')
    stmts.insertSession.run(token, phone)
    stmts.insertAudit.run('session_created', `Phone: ***${phone.slice(-4)}`, ip)
    return token
  }

  function getSessionFromHeader(req) {
    const auth = req.headers['authorization'] || ''
    const token = auth.startsWith('Bearer ') ? auth.slice(7) : ''
    if (!token) return null
    return stmts.getSession.get(token)
  }

  function requireAuth(req) {
    const session = getSessionFromHeader(req)
    if (session) stmts.touchSession.run(session.token)
    return session
  }

  /**
   * Handle POST /api/auth/send-code
   * @param {Object} body - { phone }
   * @param {string} ip - Client IP
   * @returns {{ status: number, data: Object }}
   */
  async function sendCode(body, ip) {
    const { normalizePhone } = require('./http')
    const clean = normalizePhone(body.phone)
    if (!clean || clean.length < 10) return { status: 400, data: { error: 'Invalid phone number' } }

    if (allowedPhones.length > 0 && !allowedPhones.includes(clean)) {
      return { status: 403, data: { error: 'Phone not authorized' } }
    }

    if (rateLimit(`auth-${ip}`, 10, 900000)) {
      return { status: 429, data: { error: 'Too many attempts. Try again in 15 minutes.' } }
    }

    // Fallback: direct session if Twilio not configured
    if (!twilio.accountSid || !twilio.verifyServiceSid) {
      const token = createSessionToken(clean, ip)
      return { status: 200, data: { success: true, token, phone: clean, method: 'direct' } }
    }

    const result = await twilioRequest('Verifications', { To: `+1${clean}`, Channel: 'sms' })
    if (result && result.status === 'pending') {
      stmts.insertAudit.run('code_sent', `Phone: ***${clean.slice(-4)}`, ip)
      return { status: 200, data: { success: true, codeSent: true, phone: clean } }
    }
    return { status: 500, data: { error: 'Failed to send code. Try again.' } }
  }

  /**
   * Handle POST /api/auth/verify
   * @param {Object} body - { phone, code }
   * @param {string} ip - Client IP
   * @returns {{ status: number, data: Object }}
   */
  async function verifyCode(body, ip) {
    const { normalizePhone } = require('./http')
    const clean = normalizePhone(body.phone)
    if (!clean) return { status: 400, data: { error: 'Phone required' } }

    // Fallback: auto-approve if Twilio not configured
    if (!twilio.accountSid || !twilio.verifyServiceSid) {
      const token = createSessionToken(clean, ip)
      return { status: 200, data: { success: true, token, phone: clean } }
    }

    if (!body.code) return { status: 400, data: { error: 'Code required' } }
    if (rateLimit(`verify-${ip}`, 10, 900000)) {
      return { status: 429, data: { error: 'Too many attempts.' } }
    }

    const result = await twilioRequest('VerificationCheck', { To: `+1${clean}`, Code: body.code })
    if (result && result.status === 'approved') {
      const token = createSessionToken(clean, ip)
      stmts.insertAudit.run('session_verified', `Phone: ***${clean.slice(-4)}`, ip)
      return { status: 200, data: { success: true, token, phone: clean } }
    }
    return { status: 400, data: { error: 'Invalid code. Try again.' } }
  }

  /**
   * Handle GET /api/auth/validate
   * @param {Object} req - HTTP request
   * @returns {{ status: number, data: Object }}
   */
  function validateSession(req) {
    const session = getSessionFromHeader(req)
    if (!session) return { status: 401, data: { valid: false } }
    stmts.touchSession.run(session.token)
    return { status: 200, data: { valid: true, phone: session.phone } }
  }

  return { sendCode, verifyCode, validateSession, getSessionFromHeader, requireAuth, createSessionToken }
}

module.exports = { createAuth }
