'use strict'

/**
 * Slide view tracking and analytics aggregation.
 *
 * @param {Object} config
 * @param {Object} config.stmts - Prepared statements from db.js
 */
function createAnalytics(config) {
  const { stmts } = config

  /**
   * Track a slide view.
   * @param {string} sessionId - Session phone or 'anon'
   * @param {Object} body - { slideNumber, slideTitle, dwellMs }
   * @returns {{ status: number, data: Object }}
   */
  function trackView(sessionId, body) {
    stmts.insertView.run(sessionId, body.slideNumber || 0, body.slideTitle || '', body.dwellMs || 0)
    return { status: 200, data: { success: true } }
  }

  /**
   * Get analytics report (by-slide breakdown + totals + session details).
   * @returns {{ status: number, data: Object }}
   */
  function getReport() {
    const bySlide = stmts.getAnalytics.all()
    const totalViews = stmts.getTotalViews.get().total
    const uniqueSessions = stmts.getUniqueSessions.get().total
    const sessionDetails = stmts.getSessionDetails.all()
    return { status: 200, data: { bySlide, totalViews, uniqueSessions, sessionDetails } }
  }

  return { trackView, getReport }
}

module.exports = { createAnalytics }
