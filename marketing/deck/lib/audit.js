'use strict'

/**
 * Audit log helpers.
 *
 * @param {Object} config
 * @param {Object} config.stmts - Prepared statements from db.js
 */
function createAudit(config) {
  const { stmts } = config

  function log(action, detail, ip) {
    stmts.insertAudit.run(action, detail || '', ip || '')
  }

  function getRecent() {
    return stmts.getAudit.all()
  }

  return { log, getRecent }
}

module.exports = { createAudit }
