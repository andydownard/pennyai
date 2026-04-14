import type { Context } from '@netlify/functions'
import { createClient, type SupabaseClient } from '@supabase/supabase-js'
import { createHash } from 'node:crypto'

const MAX_BODY_BYTES = 16_384
const MAX_TEXT_LEN = 280
const MAX_DESC_LEN = 140
const RATE_LIMIT_PER_IP_PER_HOUR = 5
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

const ALLOWED_ORIGINS = [
  'https://groflo.io',
  'https://www.groflo.io',
  'https://thrift.ly',          // transition window — see CLAUDE.md
  'https://www.thrift.ly',
  'http://localhost:5173',
  'http://localhost:8888',
]

function brandPortalUrl(): string {
  return process.env.BRAND_PORTAL_URL || 'https://brands.groflo.io'
}

function corsHeaders(origin: string | null): Record<string, string> {
  const allow = origin && ALLOWED_ORIGINS.includes(origin) ? origin : ALLOWED_ORIGINS[0]
  return {
    'Access-Control-Allow-Origin': allow,
    'Access-Control-Allow-Headers': 'Content-Type, Idempotency-Key',
    'Access-Control-Allow-Methods': 'POST, OPTIONS',
    'Vary': 'Origin',
  }
}

function getSupabase(): SupabaseClient {
  const url = process.env.SUPABASE_URL
  const key = process.env.SUPABASE_SERVICE_ROLE_KEY
  if (!url || !key) throw new Error('Missing Supabase config')
  return createClient(url, key)
}

function hashIp(ip: string): string {
  const salt = process.env.IP_HASH_SALT || 'groflo-default-salt'
  return createHash('sha256').update(`${salt}:${ip}`).digest('hex')
}

function getClientIp(req: Request): string {
  const fwd = req.headers.get('x-forwarded-for') || ''
  const first = fwd.split(',')[0]?.trim()
  return first || req.headers.get('x-nf-client-connection-ip') || 'unknown'
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

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

interface ValidLead {
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

export function validateLead(input: RawLead, idempotencyKeyHeader: string | null): ValidLead | { error: string } {
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

export default async (req: Request, _context: Context) => {
  const origin = req.headers.get('Origin')
  const cors = corsHeaders(origin)

  if (req.method === 'OPTIONS') {
    return new Response(null, { status: 204, headers: cors })
  }

  const headers = { ...cors, 'Content-Type': 'application/json' }

  if (req.method !== 'POST') {
    return new Response(JSON.stringify({ error: 'Method not allowed' }), { status: 405, headers })
  }

  // Body size guard before parsing JSON
  const contentLength = Number(req.headers.get('content-length') || 0)
  if (contentLength > MAX_BODY_BYTES) {
    return new Response(JSON.stringify({ error: 'Payload too large.' }), { status: 413, headers })
  }

  let body: RawLead
  try {
    const raw = await req.text()
    if (raw.length > MAX_BODY_BYTES) {
      return new Response(JSON.stringify({ error: 'Payload too large.' }), { status: 413, headers })
    }
    body = JSON.parse(raw)
  } catch {
    return new Response(JSON.stringify({ error: 'Invalid JSON.' }), { status: 400, headers })
  }

  const validated = validateLead(body, req.headers.get('Idempotency-Key'))
  if ('error' in validated) {
    return new Response(JSON.stringify({ error: validated.error }), { status: 400, headers })
  }

  let supabase: SupabaseClient
  try {
    supabase = getSupabase()
  } catch (err) {
    console.error('campaign-lead: supabase config error', err)
    return new Response(JSON.stringify({ error: 'Service unavailable.' }), { status: 500, headers })
  }

  // Idempotency: if a row with this key already exists, return it without
  // re-inserting. This makes safe retries / double-clicks cheap.
  const { data: existing, error: lookupErr } = await supabase
    .from('campaign_leads')
    .select('id')
    .eq('idempotency_key', validated.idempotency_key)
    .maybeSingle()

  if (lookupErr) {
    console.error('campaign-lead: lookup error', lookupErr)
    return new Response(JSON.stringify({ error: 'Failed to save lead.' }), { status: 500, headers })
  }

  if (existing) {
    return new Response(
      JSON.stringify({
        success: true,
        leadId: existing.id,
        redirectUrl: `${brandPortalUrl()}?from=campaign_signup&lead=${existing.id}`,
        replay: true,
      }),
      { status: 200, headers }
    )
  }

  // Per-IP rate limit (defense in depth — Netlify already throttles)
  const clientIp = getClientIp(req)
  const ipHash = hashIp(clientIp)
  const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString()
  const { count: recentCount, error: countErr } = await supabase
    .from('campaign_leads')
    .select('id', { count: 'exact', head: true })
    .eq('ip_hash', ipHash)
    .gte('created_at', oneHourAgo)

  if (countErr) {
    console.error('campaign-lead: rate-limit count error', countErr)
    return new Response(JSON.stringify({ error: 'Failed to save lead.' }), { status: 500, headers })
  }

  if ((recentCount ?? 0) >= RATE_LIMIT_PER_IP_PER_HOUR) {
    return new Response(
      JSON.stringify({ error: 'Too many submissions from this IP. Try again later.' }),
      { status: 429, headers }
    )
  }

  const userAgent = (req.headers.get('user-agent') || '').slice(0, 512)
  const { data, error } = await supabase
    .from('campaign_leads')
    .insert({ ...validated, ip_hash: ipHash, user_agent: userAgent })
    .select('id')
    .single()

  if (error) {
    // Race: another concurrent request inserted the same idempotency key.
    // Recover by re-reading the existing row.
    if (error.code === '23505') {
      const { data: replay } = await supabase
        .from('campaign_leads')
        .select('id')
        .eq('idempotency_key', validated.idempotency_key)
        .maybeSingle()
      if (replay) {
        return new Response(
          JSON.stringify({
            success: true,
            leadId: replay.id,
            redirectUrl: `${brandPortalUrl()}?from=campaign_signup&lead=${replay.id}`,
            replay: true,
          }),
          { status: 200, headers }
        )
      }
    }
    console.error('campaign-lead: insert error', error)
    return new Response(JSON.stringify({ error: 'Failed to save lead.' }), { status: 500, headers })
  }

  return new Response(
    JSON.stringify({
      success: true,
      leadId: data.id,
      redirectUrl: `${brandPortalUrl()}?from=campaign_signup&lead=${data.id}`,
      replay: false,
    }),
    { status: 201, headers }
  )
}
