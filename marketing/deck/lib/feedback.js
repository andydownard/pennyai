'use strict'

/**
 * Feedback submission, retrieval, status updates, and GitHub issue creation.
 *
 * @param {Object} config
 * @param {Object} config.stmts - Prepared statements from db.js
 * @param {Function} config.rateLimit - Rate limiter function
 * @param {string} config.ghPat - GitHub personal access token
 * @param {string} config.ghRepo - GitHub repo (e.g., 'andydownard/thrift.ly')
 * @param {string} [config.ghUserAgent] - User-Agent for GitHub API (default: 'deck-engine')
 * @param {string} [config.adminApiKey] - Admin API key for elevated access
 */
function createFeedback(config) {
  const { stmts, rateLimit, ghPat = '', ghRepo = '', ghUserAgent = 'deck-engine', adminApiKey = '' } = config

  function isAdmin(req) {
    const authHeader = (req.headers['authorization'] || '').replace(/^Bearer\s+/i, '')
    return adminApiKey && authHeader === adminApiKey
  }

  /**
   * Create a GitHub issue for feedback.
   * @param {Object} fb - { body, slideNumber, slideTitle, urgency, phone, source_url?, page_title? }
   * @returns {Promise<string|null>} GitHub issue URL or null
   */
  async function createGitHubIssue(fb) {
    if (!ghPat) return null
    try {
      const maskedPhone = `***-***-${(fb.phone || '').slice(-4)}`
      let title, body, labels

      if (fb.source_url) {
        // Widget feedback (cross-domain)
        const domain = new URL(fb.source_url).hostname
        const snippet = fb.body.length > 60 ? fb.body.slice(0, 57) + '...' : fb.body
        title = `[Feedback] ${fb.page_title || 'Page'}: ${snippet}`
        body = `**Source:** ${fb.source_url}\n**Page:** ${fb.page_title || 'N/A'}\n**Urgency:** ${fb.urgency}\n**Phone:** ${maskedPhone}\n\n---\n\n${fb.body}`
        labels = ['product-feedback', fb.urgency || 'normal', `source:${domain}`]
      } else {
        // Direct deck feedback
        title = `[Deck Feedback] Slide ${fb.slideNumber}: ${fb.slideTitle}`
        body = `**Slide:** ${fb.slideNumber} — ${fb.slideTitle}\n**Urgency:** ${fb.urgency}\n**Phone:** ${maskedPhone}\n\n---\n\n${fb.body}`
        labels = ['deck-feedback', fb.urgency || 'normal']
      }

      const res = await fetch(`https://api.github.com/repos/${ghRepo}/issues`, {
        method: 'POST',
        headers: {
          'Authorization': `token ${ghPat}`,
          'Content-Type': 'application/json',
          'User-Agent': ghUserAgent,
        },
        body: JSON.stringify({ title, body, labels }),
      })
      if (res.ok) return (await res.json()).html_url
    } catch (e) {
      console.error('GitHub issue creation failed:', e.message)
    }
    return null
  }

  /**
   * Submit feedback from an authenticated session.
   * @param {Object} session - { phone, token }
   * @param {Object} body - { feedback, slideNumber, slideTitle, urgency }
   * @param {string} ip
   * @returns {{ status: number, data: Object }}
   */
  async function submit(session, body, ip) {
    if (rateLimit(`fb-${session.phone}`, 10, 3600000)) {
      return { status: 429, data: { error: 'Rate limited' } }
    }

    const text = body.feedback || body.body
    if (!text || typeof text !== 'string' || text.length > 5000) {
      return { status: 400, data: { error: 'Invalid feedback' } }
    }

    stmts.touchSession.run(session.token)
    const result = stmts.insertFeedback.run(
      session.phone, body.slideNumber || 0, body.slideTitle || '', text, body.urgency || 'normal'
    )
    stmts.insertAudit.run(
      'feedback_submitted',
      `Phone: ***${session.phone.slice(-4)} | Slide ${body.slideNumber}: ${body.slideTitle}`,
      ip
    )

    // Update source_url if present (widget feedback)
    if (body.source_url) {
      stmts.updateFeedbackSource.run(body.source_url, result.lastInsertRowid)
    }

    // GitHub issue (async, non-blocking)
    const ghUrl = await createGitHubIssue({
      body: text,
      slideNumber: body.slideNumber,
      slideTitle: body.slideTitle || body.page_title || '',
      urgency: body.urgency || 'normal',
      phone: session.phone,
      source_url: body.source_url || '',
      page_title: body.page_title || '',
    })
    if (ghUrl) {
      stmts.updateFeedbackGH.run(ghUrl, result.lastInsertRowid)
    }

    return { status: 200, data: { success: true, id: Number(result.lastInsertRowid), github_issue_url: ghUrl || '' } }
  }

  /**
   * Get all feedback (admin or authenticated user).
   * @param {Object} req - HTTP request
   * @param {Object|null} session - Authenticated session or null
   * @returns {{ status: number, data: Object }}
   */
  function getAll(req, session) {
    if (!isAdmin(req) && !session) return { status: 401, data: { error: 'Not authenticated' } }
    const items = stmts.getAllFeedback.all()
    return { status: 200, data: { items, phone: session ? session.phone : 'admin' } }
  }

  /**
   * Update feedback status (admin or authenticated user).
   * @param {Object} req - HTTP request
   * @param {Object} body - { id, status, admin_notes }
   * @param {string} ip
   * @returns {{ status: number, data: Object }}
   */
  function updateStatus(req, body, ip) {
    if (!isAdmin(req)) return { status: 401, data: { error: 'Not authenticated' } }
    stmts.updateFeedbackStatus.run(body.status || 'new', body.admin_notes || '', body.id)
    stmts.insertAudit.run('feedback_updated', `ID ${body.id} → ${body.status}`, ip)
    return { status: 200, data: { success: true } }
  }

  return { submit, getAll, updateStatus, createGitHubIssue, isAdmin }
}

module.exports = { createFeedback }
