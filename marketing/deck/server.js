/**
 * Thriftly Deck Server — Investor pitch deck with feedback, analytics, and auth.
 *
 * Built on deck-engine for shared auth, feedback, analytics, and security.
 * This file contains only deck-specific routing and configuration.
 */
'use strict'

const http = require('http')
const path = require('path')
const { createDeckEngine } = require('./deck-engine')

// ─── Configuration ───
const PORT = process.env.PORT || 3000

const engine = createDeckEngine({
  dbPath: process.env.DB_PATH || 'thriftly.db',
  twilio: {
    accountSid: process.env.TWILIO_ACCOUNT_SID || '',
    authToken: process.env.TWILIO_AUTH_TOKEN || '',
    verifyServiceSid: process.env.TWILIO_VERIFY_SERVICE_SID || '',
  },
  allowedPhones: (process.env.FEEDBACK_PHONES || '3104861833,8054332540,6156796015').split(',').map(s => s.trim()),
  ghPat: process.env.GH_PAT || '',
  ghRepo: process.env.GH_REPO || 'andydownard/thrift.ly',
  ghUserAgent: 'thriftly-deck',
  adminApiKey: process.env.ADMIN_API_KEY || '',
  corsOrigins: [
    /^https?:\/\/(.*\.)?thrift\.ly(:\d+)?$/,
    /^https?:\/\/localhost(:\d+)?$/,
  ],
})

// ─── Server ───
const server = http.createServer(async (req, res) => {
  const ip = engine.getIP(req)
  const url = new URL(req.url, `http://${req.headers.host}`)
  const pathname = url.pathname

  engine.setSecurityHeaders(res)

  try {
    // ── Auth endpoints ──
    if (pathname === '/api/auth/send-code' && req.method === 'POST') {
      const body = await engine.parseBody(req)
      const result = await engine.auth.sendCode(body, ip)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/auth/verify' && req.method === 'POST') {
      const body = await engine.parseBody(req)
      const result = await engine.auth.verifyCode(body, ip)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/auth/validate' && req.method === 'GET') {
      const result = engine.auth.validateSession(req)
      return engine.json(res, result.data, result.status)
    }

    // ── Feedback endpoints ──
    if (pathname === '/api/feedback' && req.method === 'POST') {
      const session = engine.auth.requireAuth(req)
      if (!session) return engine.json(res, { error: 'Not authenticated' }, 401)
      const body = await engine.parseBody(req)
      const result = await engine.feedback.submit(session, body, ip)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/feedback' && req.method === 'GET') {
      const session = engine.auth.requireAuth(req)
      const result = engine.feedback.getAll(req, session)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/feedback/update' && req.method === 'POST') {
      const body = await engine.parseBody(req)
      const result = engine.feedback.updateStatus(req, body, ip)
      return engine.json(res, result.data, result.status)
    }

    // ── Analytics endpoints ──
    if (pathname === '/api/view' && req.method === 'POST') {
      const body = await engine.parseBody(req)
      const session = engine.auth.getSessionFromHeader(req)
      const sessionId = session ? session.phone : 'anon'
      const result = engine.analytics.trackView(sessionId, body)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/analytics' && req.method === 'GET') {
      const isAdmin = engine.feedback.isAdmin(req)
      const session = !isAdmin ? engine.auth.requireAuth(req) : null
      if (!isAdmin && !session) return engine.json(res, { error: 'Not authenticated' }, 401)
      const result = engine.analytics.getReport()
      return engine.json(res, result.data, result.status)
    }

    // ── Audit endpoint ──
    if (pathname === '/api/audit' && req.method === 'GET') {
      const isAdmin = engine.feedback.isAdmin(req)
      const session = !isAdmin ? engine.auth.requireAuth(req) : null
      if (!isAdmin && !session) return engine.json(res, { error: 'Not authenticated' }, 401)
      return engine.json(res, { items: engine.audit.getRecent() })
    }

    // ── Widget API (CORS-enabled) ──
    if (pathname.startsWith('/api/widget') && req.method === 'OPTIONS') {
      return engine.cors.handlePreflight(req, res)
    }

    if (pathname === '/api/widget/send-code' && req.method === 'POST') {
      engine.cors.setCorsHeaders(req, res)
      const body = await engine.parseBody(req)
      const result = await engine.auth.sendCode(body, ip)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/widget/verify' && req.method === 'POST') {
      engine.cors.setCorsHeaders(req, res)
      const body = await engine.parseBody(req)
      const result = await engine.auth.verifyCode(body, ip)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/widget/validate' && req.method === 'GET') {
      engine.cors.setCorsHeaders(req, res)
      const result = engine.auth.validateSession(req)
      return engine.json(res, result.data, result.status)
    }

    if (pathname === '/api/widget/feedback' && req.method === 'POST') {
      engine.cors.setCorsHeaders(req, res)
      const session = engine.auth.requireAuth(req)
      if (!session) return engine.json(res, { error: 'Not authenticated' }, 401)
      const body = await engine.parseBody(req)
      const result = await engine.feedback.submit(session, { ...body, feedback: body.body, source_url: body.source_url, page_title: body.page_title }, ip)
      return engine.json(res, result.data, result.status)
    }

    // ── Static files ──
    if (req.method === 'GET') {
      const filePath = pathname === '/' ? '/deck.html' : pathname
      const served = engine.serveStatic(res, filePath, __dirname, engine.MIME, {
        fallbackFile: 'deck.html',
        corsFiles: { 'widget.js': '*' },
      })
      if (served) return
    }
  } catch (e) {
    console.error('Request error:', e)
    return engine.json(res, { error: 'Internal server error' }, 500)
  }

  res.writeHead(404)
  res.end('Not Found')
})

server.listen(PORT, () => console.log(`Thrift.ly deck server running on :${PORT}`))
