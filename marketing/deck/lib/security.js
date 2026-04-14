'use strict'

const path = require('path')
const fs = require('fs')

/**
 * HTTP security headers and static file serving.
 */

function setSecurityHeaders(res) {
  res.setHeader('X-Content-Type-Options', 'nosniff')
  res.setHeader('X-Frame-Options', 'DENY')
  res.setHeader('Strict-Transport-Security', 'max-age=31536000; includeSubDomains')
}

/**
 * Serve a static file safely (path traversal prevention).
 *
 * @param {Object} res - HTTP response
 * @param {string} requestPath - URL pathname (e.g., '/deck.html')
 * @param {string} baseDir - Root directory for static files
 * @param {Object} MIME - MIME type map from http.js
 * @param {Object} [options]
 * @param {string} [options.fallbackFile] - File to serve on 404 (e.g., 'deck.html' for SPA)
 * @param {Object} [options.corsFiles] - Map of filenames to CORS origins (e.g., { 'widget.js': '*' })
 * @returns {boolean} true if file was served
 */
function serveStatic(res, requestPath, baseDir, MIME, options = {}) {
  const safePath = path.normalize(requestPath).replace(/^(\.\.[/\\])+/, '')
  const fullPath = path.join(baseDir, safePath)

  // Path traversal check
  if (!fullPath.startsWith(baseDir)) {
    res.writeHead(403)
    res.end('Forbidden')
    return true
  }

  try {
    const data = fs.readFileSync(fullPath)
    const ext = path.extname(fullPath).toLowerCase()
    const headers = {
      'Content-Type': MIME[ext] || 'application/octet-stream',
      'Cache-Control': ext === '.html' ? 'no-cache' : 'public, max-age=86400',
    }

    // CORS for specific files (e.g., widget.js)
    const basename = path.basename(safePath)
    if (options.corsFiles && options.corsFiles[basename]) {
      headers['Access-Control-Allow-Origin'] = options.corsFiles[basename]
    }

    res.writeHead(200, headers)
    res.end(data)
    return true
  } catch {
    // Try fallback (SPA-style)
    if (options.fallbackFile) {
      try {
        const data = fs.readFileSync(path.join(baseDir, options.fallbackFile))
        res.writeHead(200, { 'Content-Type': 'text/html', 'Cache-Control': 'no-cache' })
        res.end(data)
        return true
      } catch { /* fall through */ }
    }
    return false
  }
}

module.exports = { setSecurityHeaders, serveStatic }
