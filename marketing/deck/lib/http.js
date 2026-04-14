'use strict'

/**
 * Shared HTTP helpers for deck servers.
 * No dependencies — pure utility functions.
 */

function json(res, data, status = 200) {
  res.writeHead(status, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify(data))
}

function parseBody(req, maxBytes = 1e6) {
  return new Promise((resolve, reject) => {
    let body = ''
    req.on('data', c => {
      body += c
      if (body.length > maxBytes) { req.destroy(); reject(new Error('Too large')) }
    })
    req.on('end', () => {
      try { resolve(JSON.parse(body)) } catch { reject(new Error('Invalid JSON')) }
    })
  })
}

function getIP(req) {
  return req.headers['x-forwarded-for']?.split(',')[0]?.trim() || req.socket.remoteAddress || ''
}

function normalizePhone(p) {
  return (p || '').replace(/\D/g, '').replace(/^1/, '')
}

const MIME = {
  '.html': 'text/html', '.css': 'text/css', '.js': 'application/javascript',
  '.json': 'application/json', '.png': 'image/png', '.jpg': 'image/jpeg',
  '.svg': 'image/svg+xml', '.ico': 'image/x-icon', '.woff2': 'font/woff2',
  '.woff': 'font/woff', '.ttf': 'font/ttf', '.eot': 'application/vnd.ms-fontobject',
  '.map': 'application/json', '.txt': 'text/plain', '.xml': 'application/xml',
}

module.exports = { json, parseBody, getIP, normalizePhone, MIME }
