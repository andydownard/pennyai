/**
 * Groflo Marketing Server
 *
 * One Node process serves:
 *   - static dist/ (Vite build) with SPA client-side routing fallback
 *   - POST /.netlify/functions/campaign-lead  (brand campaign brief capture)
 *   - POST /.netlify/functions/feedback       (internal product feedback)
 *   - GET  /.netlify/functions/feedback       (list history by phone)
 *   - GET  /health                            (Railway healthcheck)
 *
 * Routes mirror the legacy Netlify function paths so the React callsites in
 * CampaignSignup.tsx and FeedbackWidget.tsx continue to work unchanged.
 *
 * Backing store: Postgres (Rally's Postgres-mpxU, pennyai-owned instance).
 * Tables are created on startup via CREATE TABLE IF NOT EXISTS, matching the
 * rest of this repo's additive-only migration pattern (see CLAUDE.md
 * "Database Rules"). ALTER TABLE ADD COLUMN for later changes.
 *
 * Rate limiting: in-memory sliding-window Map per (route, ip) and per
 * (route, phone). CLAUDE.md requires server-side rate limits that are not
 * bypassable by the client. Single-instance is fine for Railway's default
 * deploy; horizontal scale-out would need a shared store.
 */

import express from 'express'
import type { Request, Response, NextFunction } from 'express'
import helmet from 'helmet'
import cors from 'cors'
import { createHash, randomUUID } from 'node:crypto'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import fs from 'node:fs'
import { Pool } from 'pg'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// ──────────────────────────────────────────────────────────────────
// Config
// ──────────────────────────────────────────────────────────────────

const PORT = Number(process.env.PORT || 8080)
const DIST_DIR = path.resolve(__dirname, 'dist')
// Env vars read lazily (on first use) so tests can set them before route calls.
const getDatabaseUrl = () => process.env.DATABASE_URL || ''
const getIpHashSalt = () => process.env.IP_HASH_SALT || 'groflo-default-salt'
const getBrandPortalUrl = () => process.env.BRAND_PORTAL_URL || 'https://brands.groflo.io'

// Campaign-lead constants (match legacy Netlify function)
const MAX_BODY_BYTES = 16_384
const MAX_TEXT_LEN = 280
const MAX_DESC_LEN = 140
const CAMPAIGN_RATE_LIMIT_PER_IP_PER_HOUR = 5
const ALLOWED_AD_TYPES = new Set([
  'free-trial',
  'targeted-swap',
  'promoted-suggestion',
  'sponsored-brand',
  'list-placement',
])
const ALLOWED_GEOGRAPHIES = new Set([
  'national',
  'northeast',
  'southeast',
  'midwest',
  'southwest',
  'west',
])
const ALLOWED_BUDGET_TYPES = new Set(['daily', 'total'])

// Feedback constants (match legacy Netlify function)
const FEEDBACK_ALLOWED_PHONES = ['3104861833', '6264043580']
const FEEDBACK_MAX_BODY_LENGTH = 10_000
const FEEDBACK_RATE_LIMIT_PER_PHONE_PER_HOUR = 10

const ALLOWED_ORIGINS = [
  'https://groflo.io',
  'https://www.groflo.io',
  'https://thrift.ly', // transition window — see marketing/CLAUDE.md
  'https://www.thrift.ly',
  'http://localhost:5173',
  'http://localhost:8888',
  'http://localhost:8080',
]

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// ──────────────────────────────────────────────────────────────────
// Postgres pool (lazy — created on first use so the server can start
// and answer /health even if DATABASE_URL is unset)
// ──────────────────────────────────────────────────────────────────

let pool: Pool | null = null
let migratedOnce = false

export function getPool(): Pool {
  if (!pool) {
    const url = getDatabaseUrl()
    if (!url) throw new Error('DATABASE_URL not configured')
    pool = new Pool({
      connectionString: url,
      max: 5,
      idleTimeoutMillis: 30_000,
      connectionTimeoutMillis: 10_000,
    })
    pool.on('error', (err) => {
      // Never log secrets — just the error class.
      console.error('pg pool error:', err.message)
    })
  }
  return pool
}

/** Test helper — DO NOT call from production code. Clears the cached pool. */
export function __resetPoolForTest(): void {
  pool = null
}

export async function runMigrations(p: Pool = getPool()): Promise<void> {
  if (migratedOnce) return
  // Additive-only. Matches the Go backend pattern (CLAUDE.md Database Rules).
  await p.query(`
    CREATE TABLE IF NOT EXISTS campaign_leads (
      id BIGSERIAL PRIMARY KEY,
      idempotency_key TEXT NOT NULL,
      full_name TEXT NOT NULL,
      company_name TEXT NOT NULL,
      email TEXT NOT NULL,
      website TEXT NOT NULL DEFAULT '',
      ad_type TEXT NOT NULL,
      categories TEXT NOT NULL,
      geography TEXT NOT NULL,
      budget_type TEXT NOT NULL,
      budget_amount INTEGER NOT NULL,
      start_date DATE NOT NULL,
      end_date DATE NOT NULL,
      creative_name TEXT NOT NULL,
      creative_description TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'new',
      source TEXT NOT NULL DEFAULT 'campaign_signup',
      ip_hash TEXT NOT NULL DEFAULT '',
      user_agent TEXT NOT NULL DEFAULT '',
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      CONSTRAINT campaign_leads_idempotency_key_unique UNIQUE (idempotency_key)
    )
  `)
  await p.query(`CREATE INDEX IF NOT EXISTS idx_campaign_leads_email ON campaign_leads (lower(email))`)
  await p.query(`CREATE INDEX IF NOT EXISTS idx_campaign_leads_status_created ON campaign_leads (status, created_at DESC)`)
  await p.query(`CREATE INDEX IF NOT EXISTS idx_campaign_leads_ip_created ON campaign_leads (ip_hash, created_at DESC)`)

  await p.query(`
    CREATE TABLE IF NOT EXISTS marketing_feedback_requests (
      id BIGSERIAL PRIMARY KEY,
      phone TEXT NOT NULL,
      body TEXT NOT NULL,
      urgency TEXT NOT NULL DEFAULT 'normal',
      status TEXT NOT NULL DEFAULT 'new',
      admin_notes TEXT NOT NULL DEFAULT '',
      idempotency_key TEXT NOT NULL DEFAULT '',
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )
  `)
  await p.query(`CREATE INDEX IF NOT EXISTS idx_marketing_feedback_phone ON marketing_feedback_requests (phone)`)
  await p.query(`CREATE INDEX IF NOT EXISTS idx_marketing_feedback_status_created ON marketing_feedback_requests (status, created_at DESC)`)
  // Conditional unique — only enforced when key is non-empty, so legacy rows with
  // empty keys (e.g. from pre-idempotency-era callers) don't collide.
  await p.query(`
    CREATE UNIQUE INDEX IF NOT EXISTS idx_marketing_feedback_idempotency_key
      ON marketing_feedback_requests (idempotency_key)
      WHERE idempotency_key <> ''
  `)
  migratedOnce = true
}

/** Test helper — DO NOT call from production code. Resets the migration-once flag. */
export function __resetMigrationsForTest(): void {
  migratedOnce = false
}

/**
 * pg returns BIGINT columns as strings to preserve precision. The React
 * callsites expect numeric IDs (CampaignSignup.tsx types leadId as number;
 * FeedbackWidget.tsx compares with item.id numerically). Coerce here —
 * safe because row IDs for this app will not exceed Number.MAX_SAFE_INTEGER.
 */
function toNum(v: unknown): number {
  if (typeof v === 'number') return v
  if (typeof v === 'bigint') return Number(v)
  return Number(v)
}

// ──────────────────────────────────────────────────────────────────
// Pure helpers (validation, rate limiting, etc.)
// ──────────────────────────────────────────────────────────────────

export function hashIp(ip: string): string {
  return createHash('sha256').update(`${getIpHashSalt()}:${ip}`).digest('hex')
}

export function normalizePhone(input: string): string {
  return input.replace(/\D/g, '').replace(/^1(\d{10})$/, '$1')
}

export function getClientIp(req: Request): string {
  const fwd = (req.headers['x-forwarded-for'] as string) || ''
  const first = fwd.split(',')[0]?.trim()
  return first || (req.ip || 'unknown')
}

interface RawLead {
  idempotencyKey?: unknown
  fullName?: unknown
  companyName?: unknown
  email?: unknown
  website?: unknown
  adType?: unknown
  categories?: unknown
  geography?: unknown
  budgetType?: unknown
  budgetAmount?: unknown
  startDate?: unknown
  endDate?: unknown
  creativeName?: unknown
  creativeDescription?: unknown
}

export interface ValidLead {
  idempotency_key: string
  full_name: string
  company_name: string
  email: string
  website: string
  ad_type: string
  categories: string
  geography: string
  budget_type: 'daily' | 'total'
  budget_amount: number
  start_date: string
  end_date: string
  creative_name: string
  creative_description: string
}

function str(v: unknown, max: number): string | null {
  if (typeof v !== 'string') return null
  const trimmed = v.trim()
  if (trimmed.length === 0 || trimmed.length > max) return null
  return trimmed
}

export function validateLead(
  input: RawLead,
  idempotencyKeyHeader: string | null
): ValidLead | { error: string } {
  const key = idempotencyKeyHeader || (typeof input.idempotencyKey === 'string' ? input.idempotencyKey : '')
  if (!UUID_RE.test(key)) return { error: 'Missing or invalid idempotency key.' }

  const fullName = str(input.fullName, MAX_TEXT_LEN)
  if (!fullName) return { error: 'Full name is required.' }

  const companyName = str(input.companyName, MAX_TEXT_LEN)
  if (!companyName) return { error: 'Company name is required.' }

  const email = str(input.email, MAX_TEXT_LEN)?.toLowerCase()
  if (!email || !EMAIL_RE.test(email)) return { error: 'Valid email is required.' }

  const website = typeof input.website === 'string' ? input.website.trim().slice(0, MAX_TEXT_LEN) : ''

  const adType = typeof input.adType === 'string' ? input.adType : ''
  if (!ALLOWED_AD_TYPES.has(adType)) return { error: 'Invalid campaign type.' }

  if (!Array.isArray(input.categories) || input.categories.length === 0) {
    return { error: 'At least one category is required.' }
  }
  const categories = (input.categories as unknown[])
    .filter((c): c is string => typeof c === 'string' && c.length > 0 && c.length <= 64)
    .slice(0, 20)
    .join(',')
  if (categories.length === 0) return { error: 'At least one valid category is required.' }

  const geography = typeof input.geography === 'string' ? input.geography : ''
  if (!ALLOWED_GEOGRAPHIES.has(geography)) return { error: 'Invalid geography.' }

  const budgetType = typeof input.budgetType === 'string' ? input.budgetType : ''
  if (!ALLOWED_BUDGET_TYPES.has(budgetType)) return { error: 'Invalid budget type.' }

  const budgetAmount = Number(input.budgetAmount)
  if (!Number.isFinite(budgetAmount) || budgetAmount <= 0) {
    return { error: 'Budget amount must be a positive number.' }
  }
  const minBudget = budgetType === 'daily' ? 25 : 100
  if (budgetAmount < minBudget) return { error: `Minimum ${budgetType} budget is $${minBudget}.` }
  if (budgetAmount > 1_000_000) return { error: 'Budget exceeds maximum.' }

  const startDate = typeof input.startDate === 'string' ? input.startDate : ''
  const endDate = typeof input.endDate === 'string' ? input.endDate : ''
  if (!DATE_RE.test(startDate) || !DATE_RE.test(endDate)) {
    return { error: 'Valid start and end dates are required.' }
  }
  if (endDate <= startDate) return { error: 'End date must be after start date.' }

  const creativeName = str(input.creativeName, MAX_TEXT_LEN)
  if (!creativeName) return { error: 'Product/brand name is required.' }

  const creativeDescription = str(input.creativeDescription, MAX_DESC_LEN)
  if (!creativeDescription) return { error: 'Product description is required.' }

  return {
    idempotency_key: key,
    full_name: fullName,
    company_name: companyName,
    email,
    website,
    ad_type: adType,
    categories,
    geography,
    budget_type: budgetType as 'daily' | 'total',
    budget_amount: Math.round(budgetAmount),
    start_date: startDate,
    end_date: endDate,
    creative_name: creativeName,
    creative_description: creativeDescription,
  }
}

// ──────────────────────────────────────────────────────────────────
// In-memory sliding-window rate limiter (per key, last-hour count)
// ──────────────────────────────────────────────────────────────────

const ONE_HOUR_MS = 60 * 60 * 1000
const rateBuckets = new Map<string, number[]>()

export function hitRateLimit(key: string, limitPerHour: number, now: number = Date.now()): boolean {
  const cutoff = now - ONE_HOUR_MS
  const arr = (rateBuckets.get(key) || []).filter((t) => t > cutoff)
  if (arr.length >= limitPerHour) {
    rateBuckets.set(key, arr)
    return true
  }
  arr.push(now)
  rateBuckets.set(key, arr)
  return false
}

/** Test helper. Clears rate buckets between tests. */
export function __resetRateBucketsForTest(): void {
  rateBuckets.clear()
}

// ──────────────────────────────────────────────────────────────────
// App factory
// ──────────────────────────────────────────────────────────────────

export function createApp(): express.Express {
  const app = express()

  app.set('trust proxy', true) // Railway sits behind a proxy
  app.use(helmet({ contentSecurityPolicy: false })) // SPA sets its own CSP via <meta> if needed
  app.use(
    cors({
      origin: (origin, cb) => {
        if (!origin) return cb(null, true) // non-browser clients, same-origin
        if (ALLOWED_ORIGINS.includes(origin)) return cb(null, true)
        // Do not throw — let the request through without CORS headers so the
        // browser blocks it, but the server still processes (legacy behavior
        // from the Netlify function: CORS header falls back to canonical).
        return cb(null, true)
      },
      methods: ['GET', 'POST', 'OPTIONS'],
      allowedHeaders: ['Content-Type', 'Idempotency-Key'],
    })
  )
  app.use(express.json({ limit: MAX_BODY_BYTES }))

  // ── Health ──
  app.get('/health', (_req, res) => {
    res.json({ status: 'ok' })
  })

  // ── Campaign-lead ──
  app.post('/.netlify/functions/campaign-lead', async (req: Request, res: Response) => {
    // Body size guard (express.json already enforces the JSON byte limit;
    // this covers the case where Content-Length is present but parser didn't run)
    const contentLength = Number(req.headers['content-length'] || 0)
    if (contentLength > MAX_BODY_BYTES) {
      return res.status(413).json({ error: 'Payload too large.' })
    }

    const idempotencyKey = (req.headers['idempotency-key'] as string) || null
    const validated = validateLead(req.body || {}, idempotencyKey)
    if ('error' in validated) {
      return res.status(400).json({ error: validated.error })
    }

    let p: Pool
    try {
      p = getPool()
      await runMigrations(p)
    } catch (err) {
      console.error('campaign-lead: db config error:', (err as Error).message)
      return res.status(500).json({ error: 'Service unavailable.' })
    }

    try {
      // Idempotency lookup — same key returns same row without re-inserting.
      const existing = await p.query<{ id: number }>(
        'SELECT id FROM campaign_leads WHERE idempotency_key = $1 LIMIT 1',
        [validated.idempotency_key]
      )
      if (existing.rows.length > 0) {
        const leadId = toNum(existing.rows[0].id)
        return res.status(200).json({
          success: true,
          leadId,
          redirectUrl: `${getBrandPortalUrl()}?from=campaign_signup&lead=${leadId}`,
          replay: true,
        })
      }

      // Per-IP rate limit — defense in depth.
      const clientIp = getClientIp(req)
      const ipHash = hashIp(clientIp)

      const rlKey = `campaign-lead:ip:${ipHash}`
      if (hitRateLimit(rlKey, CAMPAIGN_RATE_LIMIT_PER_IP_PER_HOUR)) {
        return res.status(429).json({ error: 'Too many submissions from this IP. Try again later.' })
      }

      const userAgent = ((req.headers['user-agent'] as string) || '').slice(0, 512)

      const insert = await p.query<{ id: number }>(
        `INSERT INTO campaign_leads (
          idempotency_key, full_name, company_name, email, website,
          ad_type, categories, geography, budget_type, budget_amount,
          start_date, end_date, creative_name, creative_description,
          ip_hash, user_agent
        ) VALUES (
          $1, $2, $3, $4, $5,
          $6, $7, $8, $9, $10,
          $11, $12, $13, $14,
          $15, $16
        )
        ON CONFLICT (idempotency_key) DO NOTHING
        RETURNING id`,
        [
          validated.idempotency_key,
          validated.full_name,
          validated.company_name,
          validated.email,
          validated.website,
          validated.ad_type,
          validated.categories,
          validated.geography,
          validated.budget_type,
          validated.budget_amount,
          validated.start_date,
          validated.end_date,
          validated.creative_name,
          validated.creative_description,
          ipHash,
          userAgent,
        ]
      )

      if (insert.rows.length === 0) {
        // Race: another concurrent request inserted the same idempotency key
        // between our lookup and our insert. Recover by re-reading.
        const replay = await p.query<{ id: number }>(
          'SELECT id FROM campaign_leads WHERE idempotency_key = $1 LIMIT 1',
          [validated.idempotency_key]
        )
        if (replay.rows.length > 0) {
          const leadId = toNum(replay.rows[0].id)
          return res.status(200).json({
            success: true,
            leadId,
            redirectUrl: `${getBrandPortalUrl()}?from=campaign_signup&lead=${leadId}`,
            replay: true,
          })
        }
        console.error('campaign-lead: insert race: no row after ON CONFLICT DO NOTHING')
        return res.status(500).json({ error: 'Failed to save lead.' })
      }

      const leadId = toNum(insert.rows[0].id)
      return res.status(201).json({
        success: true,
        leadId,
        redirectUrl: `${getBrandPortalUrl()}?from=campaign_signup&lead=${leadId}`,
        replay: false,
      })
    } catch (err) {
      console.error('campaign-lead: insert error:', (err as Error).message)
      return res.status(500).json({ error: 'Failed to save lead.' })
    }
  })

  // ── Feedback POST ──
  app.post('/.netlify/functions/feedback', async (req: Request, res: Response) => {
    try {
      const body = (req.body || {}) as { phone?: unknown; body?: unknown; urgency?: unknown }

      const normalized = normalizePhone(typeof body.phone === 'string' ? body.phone : '')
      if (!FEEDBACK_ALLOWED_PHONES.includes(normalized)) {
        return res.status(403).json({ error: 'Not authorized.' })
      }

      const text = (typeof body.body === 'string' ? body.body : '').trim()
      if (!text) {
        return res.status(400).json({ error: 'Feedback cannot be empty.' })
      }
      if (text.length > FEEDBACK_MAX_BODY_LENGTH) {
        return res.status(400).json({ error: 'Feedback is too long.' })
      }

      const urg = body.urgency === 'critical' ? 'critical' : 'normal'

      // Idempotency header is OPTIONAL (FeedbackWidget.tsx doesn't send one).
      // When sent, duplicate POSTs return the existing row instead of inserting.
      const idempotencyKey =
        typeof req.headers['idempotency-key'] === 'string' ? (req.headers['idempotency-key'] as string) : ''
      const idemKey = UUID_RE.test(idempotencyKey) ? idempotencyKey : ''

      let p: Pool
      try {
        p = getPool()
        await runMigrations(p)
      } catch (err) {
        console.error('feedback: db config error:', (err as Error).message)
        return res.status(500).json({ error: 'Something went wrong.' })
      }

      // Idempotency replay
      if (idemKey) {
        const existing = await p.query<{ id: number }>(
          `SELECT id FROM marketing_feedback_requests WHERE idempotency_key = $1 LIMIT 1`,
          [idemKey]
        )
        if (existing.rows.length > 0) {
          return res.status(200).json({ success: true, id: toNum(existing.rows[0].id), replay: true })
        }
      }

      // Per-phone rate limit
      const rlKey = `feedback:phone:${normalized}`
      if (hitRateLimit(rlKey, FEEDBACK_RATE_LIMIT_PER_PHONE_PER_HOUR)) {
        return res.status(429).json({ error: 'Easy there — max 10 submissions per hour. Take a breather.' })
      }

      const insert = await p.query<{ id: number }>(
        `INSERT INTO marketing_feedback_requests (phone, body, urgency, idempotency_key)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (idempotency_key) WHERE idempotency_key <> '' DO NOTHING
         RETURNING id`,
        [normalized, text, urg, idemKey]
      )

      if (insert.rows.length === 0 && idemKey) {
        // Race on idempotency key — re-read.
        const replay = await p.query<{ id: number }>(
          `SELECT id FROM marketing_feedback_requests WHERE idempotency_key = $1 LIMIT 1`,
          [idemKey]
        )
        if (replay.rows.length > 0) {
          return res.status(200).json({ success: true, id: toNum(replay.rows[0].id), replay: true })
        }
      }

      if (insert.rows.length === 0) {
        console.error('feedback: insert returned no rows without idempotency key')
        return res.status(500).json({ error: 'Failed to save feedback.' })
      }

      return res.status(201).json({ success: true, id: toNum(insert.rows[0].id) })
    } catch (err) {
      console.error('feedback POST error:', (err as Error).message)
      return res.status(500).json({ error: 'Something went wrong.' })
    }
  })

  // ── Feedback GET (list history by phone) ──
  app.get('/.netlify/functions/feedback', async (req: Request, res: Response) => {
    try {
      const phoneParam = typeof req.query.phone === 'string' ? (req.query.phone as string) : ''
      const normalized = normalizePhone(phoneParam)
      if (!FEEDBACK_ALLOWED_PHONES.includes(normalized)) {
        return res.status(403).json({ error: 'Not authorized.' })
      }

      let p: Pool
      try {
        p = getPool()
        await runMigrations(p)
      } catch (err) {
        console.error('feedback GET: db config error:', (err as Error).message)
        return res.status(500).json({ error: 'Failed to load feedback.' })
      }

      const rows = await p.query(
        `SELECT id, body, urgency, status, admin_notes, created_at
         FROM marketing_feedback_requests
         WHERE phone = $1
         ORDER BY created_at DESC
         LIMIT 50`,
        [normalized]
      )
      return res.status(200).json(rows.rows.map((r: Record<string, unknown>) => ({ ...r, id: toNum(r.id) })))
    } catch (err) {
      console.error('feedback GET error:', (err as Error).message)
      return res.status(500).json({ error: 'Failed to load feedback.' })
    }
  })

  // ── Static + SPA fallback ──
  // Only mount static middleware if dist/ exists (so tests don't require a build)
  if (fs.existsSync(DIST_DIR)) {
    app.use(
      express.static(DIST_DIR, {
        index: false,
        setHeaders: (res, filePath) => {
          if (filePath.includes(`${path.sep}assets${path.sep}`)) {
            res.setHeader('Cache-Control', 'public, max-age=31536000, immutable')
          } else if (filePath.endsWith('index.html')) {
            res.setHeader('Cache-Control', 'public, max-age=0, must-revalidate')
          }
        },
      })
    )

    // SPA client-side routing: every non-API GET falls back to index.html.
    app.get(/.*/, (req: Request, res: Response, next: NextFunction) => {
      if (req.path.startsWith('/.netlify/') || req.path === '/health') return next()
      const indexPath = path.join(DIST_DIR, 'index.html')
      if (!fs.existsSync(indexPath)) return next()
      res.setHeader('Cache-Control', 'public, max-age=0, must-revalidate')
      res.sendFile(indexPath)
    })
  }

  // Default 404 JSON for everything else (API routes with wrong method, etc.)
  app.use((req: Request, res: Response) => {
    res.status(404).json({ error: 'Not Found', path: req.path })
  })

  // Final error handler — never leak stack traces / secrets.
  app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
    console.error('unhandled error:', err.message)
    res.status(500).json({ error: 'Internal server error' })
  })

  return app
}

// Idempotency-key helper exported for tests / callers.
export function newIdempotencyKey(): string {
  return randomUUID()
}

// ──────────────────────────────────────────────────────────────────
// Bootstrap
// ──────────────────────────────────────────────────────────────────

// Only start the server when this file is run directly (not when imported by tests).
const isMain = (() => {
  try {
    return process.argv[1] && path.resolve(process.argv[1]) === __filename
  } catch {
    return false
  }
})()

if (isMain) {
  const app = createApp()
  // Best-effort migration at startup — don't block listen on failure.
  if (getDatabaseUrl()) {
    runMigrations().catch((err) => {
      console.error('startup migration failed:', (err as Error).message)
    })
  } else {
    console.warn('DATABASE_URL not set — API routes will return 500 until configured')
  }
  app.listen(PORT, () => {
    console.log(`groflo marketing server listening on :${PORT}`)
  })
}
