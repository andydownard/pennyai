'use strict'

/**
 * CORS helpers for embeddable widget endpoints.
 *
 * @param {Object} [config]
 * @param {RegExp[]} [config.allowedOrigins] - Array of regex patterns for allowed origins
 */
function createCors(config = {}) {
  const { allowedOrigins = [
    /^https?:\/\/localhost(:\d+)?$/,
  ] } = config

  /**
   * Set CORS headers if origin matches allowed patterns.
   * @param {Object} req - HTTP request
   * @param {Object} res - HTTP response
   * @returns {boolean} true if origin was allowed
   */
  function setCorsHeaders(req, res) {
    const origin = req.headers['origin'] || ''
    const allowed = allowedOrigins.some(pattern => pattern.test(origin))
    if (allowed) {
      res.setHeader('Access-Control-Allow-Origin', origin)
      res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
      res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization')
      res.setHeader('Access-Control-Max-Age', '86400')
    }
    return allowed
  }

  /**
   * Handle OPTIONS preflight request.
   * @param {Object} req - HTTP request
   * @param {Object} res - HTTP response
   */
  function handlePreflight(req, res) {
    setCorsHeaders(req, res)
    res.writeHead(204)
    res.end()
  }

  return { setCorsHeaders, handlePreflight }
}

module.exports = { createCors }
