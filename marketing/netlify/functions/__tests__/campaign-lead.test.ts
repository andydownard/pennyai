import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@supabase/supabase-js', () => import('./_supabase-mock'))

import { __mockState, resetMockState } from './_supabase-mock'
import campaignLead, { validateLead } from '../campaign-lead.mts'

const KEY_A = '11111111-1111-4111-8111-111111111111'
const KEY_B = '22222222-2222-4222-8222-222222222222'
const KEY_C = '33333333-3333-4333-8333-333333333333'

function validBody(overrides: Record<string, unknown> = {}) {
  return {
    fullName: 'Jane Smith',
    companyName: 'Acme Foods',
    email: 'jane@acme.com',
    website: 'acme.com',
    adType: 'free-trial',
    categories: ['Dairy', 'Snacks'],
    geography: 'national',
    budgetType: 'daily',
    budgetAmount: 50,
    startDate: '2026-05-01',
    endDate: '2026-06-01',
    creativeName: 'Organic Valley Whole Milk',
    creativeDescription: 'Farm-fresh organic milk from pasture-raised cows',
    ...overrides,
  }
}

function makeRequest(body: unknown, opts: { idempotencyKey?: string; ip?: string; method?: string; origin?: string } = {}): Request {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'x-forwarded-for': opts.ip ?? '203.0.113.42',
    'user-agent': 'Mozilla/5.0 (test)',
  }
  if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey
  if (opts.origin) headers.Origin = opts.origin
  const method = opts.method ?? 'POST'
  const init: RequestInit = { method, headers }
  if (method !== 'GET' && method !== 'HEAD') {
    init.body = typeof body === 'string' ? body : JSON.stringify(body)
  }
  return new Request('https://groflo.io/.netlify/functions/campaign-lead', init)
}

const ctx = {} as Parameters<typeof campaignLead>[1]

describe('campaign-lead Netlify function', () => {
  beforeEach(() => {
    resetMockState()
    process.env.SUPABASE_URL = 'https://test.supabase.co'
    process.env.SUPABASE_SERVICE_ROLE_KEY = 'service-role-test-key'
    process.env.IP_HASH_SALT = 'test-salt'
    delete process.env.BRAND_PORTAL_URL
  })

  describe('validateLead (pure)', () => {
    it('accepts a complete valid payload', () => {
      const result = validateLead(validBody(), KEY_A)
      expect('error' in result).toBe(false)
    })

    it('rejects missing idempotency key', () => {
      const result = validateLead(validBody(), null)
      expect('error' in result && result.error).toMatch(/idempotency/i)
    })

    it('rejects malformed idempotency key (not a uuid)', () => {
      const result = validateLead(validBody(), 'not-a-uuid')
      expect('error' in result && result.error).toMatch(/idempotency/i)
    })

    it('rejects missing fullName', () => {
      const result = validateLead(validBody({ fullName: '' }), KEY_A)
      expect('error' in result && result.error).toMatch(/full name/i)
    })

    it('rejects fullName over MAX_TEXT_LEN', () => {
      const result = validateLead(validBody({ fullName: 'a'.repeat(281) }), KEY_A)
      expect('error' in result && result.error).toMatch(/full name/i)
    })

    it('rejects malformed email', () => {
      const result = validateLead(validBody({ email: 'not-an-email' }), KEY_A)
      expect('error' in result && result.error).toMatch(/email/i)
    })

    it('lowercases email', () => {
      const result = validateLead(validBody({ email: 'JANE@ACME.COM' }), KEY_A)
      if ('error' in result) throw new Error('expected ok')
      expect(result.email).toBe('jane@acme.com')
    })

    it('rejects unknown adType', () => {
      const result = validateLead(validBody({ adType: 'super-mega-ad' }), KEY_A)
      expect('error' in result && result.error).toMatch(/campaign type/i)
    })

    it('rejects empty categories array', () => {
      const result = validateLead(validBody({ categories: [] }), KEY_A)
      expect('error' in result && result.error).toMatch(/categor/i)
    })

    it('rejects categories that is not an array', () => {
      const result = validateLead(validBody({ categories: 'Dairy' }), KEY_A)
      expect('error' in result && result.error).toMatch(/categor/i)
    })

    it('joins categories with comma in normalized form', () => {
      const result = validateLead(validBody({ categories: ['Dairy', 'Snacks', 'Produce'] }), KEY_A)
      if ('error' in result) throw new Error('expected ok')
      expect(result.categories).toBe('Dairy,Snacks,Produce')
    })

    it('caps categories at 20 entries', () => {
      const many = Array.from({ length: 30 }, (_, i) => `Cat${i}`)
      const result = validateLead(validBody({ categories: many }), KEY_A)
      if ('error' in result) throw new Error('expected ok')
      expect(result.categories.split(',').length).toBe(20)
    })

    it('rejects unknown geography', () => {
      const result = validateLead(validBody({ geography: 'mars' }), KEY_A)
      expect('error' in result && result.error).toMatch(/geography/i)
    })

    it('rejects daily budget below $25', () => {
      const result = validateLead(validBody({ budgetType: 'daily', budgetAmount: 10 }), KEY_A)
      expect('error' in result && result.error).toMatch(/\$25/)
    })

    it('rejects total budget below $100', () => {
      const result = validateLead(validBody({ budgetType: 'total', budgetAmount: 50 }), KEY_A)
      expect('error' in result && result.error).toMatch(/\$100/)
    })

    it('rejects budget over $1,000,000', () => {
      const result = validateLead(validBody({ budgetAmount: 5_000_000 }), KEY_A)
      expect('error' in result && result.error).toMatch(/maximum/i)
    })

    it('rejects negative budget', () => {
      const result = validateLead(validBody({ budgetAmount: -50 }), KEY_A)
      expect('error' in result && result.error).toMatch(/positive/i)
    })

    it('rejects non-numeric budget', () => {
      const result = validateLead(validBody({ budgetAmount: 'free' }), KEY_A)
      expect('error' in result && result.error).toMatch(/positive/i)
    })

    it('rejects malformed startDate', () => {
      const result = validateLead(validBody({ startDate: '05/01/2026' }), KEY_A)
      expect('error' in result && result.error).toMatch(/dates/i)
    })

    it('rejects endDate before startDate', () => {
      const result = validateLead(validBody({ startDate: '2026-06-01', endDate: '2026-05-01' }), KEY_A)
      expect('error' in result && result.error).toMatch(/end date/i)
    })

    it('rejects endDate equal to startDate', () => {
      const result = validateLead(validBody({ startDate: '2026-05-01', endDate: '2026-05-01' }), KEY_A)
      expect('error' in result && result.error).toMatch(/end date/i)
    })

    it('rejects creativeDescription over 140 chars', () => {
      const result = validateLead(validBody({ creativeDescription: 'x'.repeat(141) }), KEY_A)
      expect('error' in result && result.error).toMatch(/description/i)
    })

    it('rounds fractional budget to integer', () => {
      const result = validateLead(validBody({ budgetAmount: 49.7 }), KEY_A)
      if ('error' in result) throw new Error('expected ok')
      expect(result.budget_amount).toBe(50)
    })
  })

  describe('HTTP handler', () => {
    it('handles CORS preflight', async () => {
      const req = makeRequest('', { method: 'OPTIONS', origin: 'https://groflo.io' })
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(204)
      expect(res.headers.get('Access-Control-Allow-Origin')).toBe('https://groflo.io')
    })

    it('rejects non-POST methods', async () => {
      const req = makeRequest('', { method: 'GET' })
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(405)
    })

    it('rejects invalid JSON', async () => {
      const req = makeRequest('not json', { idempotencyKey: KEY_A })
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(400)
      const json = await res.json()
      expect(json.error).toMatch(/json/i)
    })

    it('rejects missing idempotency key header', async () => {
      const req = makeRequest(validBody())
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(400)
      const json = await res.json()
      expect(json.error).toMatch(/idempotency/i)
    })

    it('rejects oversized payload via Content-Length', async () => {
      const big = 'x'.repeat(20_000)
      const req = new Request('https://groflo.io/.netlify/functions/campaign-lead', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': '20000',
          'Idempotency-Key': KEY_A,
        },
        body: big,
      })
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(413)
    })

    it('inserts a valid lead and returns 201 with id + redirectUrl', async () => {
      const req = makeRequest(validBody(), { idempotencyKey: KEY_A })
      const res = await campaignLead(req, ctx)
      expect(res.status).toBe(201)
      const json = await res.json()
      expect(json.success).toBe(true)
      expect(json.leadId).toBeGreaterThan(0)
      expect(json.redirectUrl).toContain('brands.groflo.io')
      expect(json.redirectUrl).toContain(`lead=${json.leadId}`)
      expect(json.replay).toBe(false)
      expect(__mockState.tables.campaign_leads.rows.length).toBe(1)
      const row = __mockState.tables.campaign_leads.rows[0] as Record<string, unknown>
      expect(row.email).toBe('jane@acme.com')
      expect(row.idempotency_key).toBe(KEY_A)
    })

    it('returns the same row on repeat submission with same idempotency key (no duplicate)', async () => {
      const req1 = makeRequest(validBody(), { idempotencyKey: KEY_A })
      const res1 = await campaignLead(req1, ctx)
      const json1 = await res1.json()

      const req2 = makeRequest(validBody({ creativeName: 'Different Name' }), { idempotencyKey: KEY_A })
      const res2 = await campaignLead(req2, ctx)
      expect(res2.status).toBe(200)
      const json2 = await res2.json()

      expect(json2.leadId).toBe(json1.leadId)
      expect(json2.replay).toBe(true)
      expect(__mockState.tables.campaign_leads.rows.length).toBe(1)
    })

    it('different idempotency key produces a new row', async () => {
      const res1 = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A }), ctx)
      const res2 = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_B }), ctx)
      expect((await res1.json()).leadId).not.toBe((await res2.json()).leadId)
      expect(__mockState.tables.campaign_leads.rows.length).toBe(2)
    })

    it('rate-limits 5 submissions per IP per hour', async () => {
      for (let i = 0; i < 5; i++) {
        const key = `${i.toString().padStart(8, '0')}-1111-4111-8111-111111111111`
        const res = await campaignLead(
          makeRequest(validBody({ creativeName: `Product ${i}` }), { idempotencyKey: key, ip: '198.51.100.1' }),
          ctx
        )
        expect(res.status).toBe(201)
      }
      const res6 = await campaignLead(
        makeRequest(validBody(), { idempotencyKey: KEY_C, ip: '198.51.100.1' }),
        ctx
      )
      expect(res6.status).toBe(429)
    })

    it('different IPs do not share a rate-limit bucket', async () => {
      for (let i = 0; i < 5; i++) {
        const key = `${i.toString().padStart(8, '0')}-1111-4111-8111-111111111111`
        await campaignLead(
          makeRequest(validBody({ creativeName: `P${i}` }), { idempotencyKey: key, ip: '198.51.100.1' }),
          ctx
        )
      }
      const res = await campaignLead(
        makeRequest(validBody(), { idempotencyKey: KEY_C, ip: '198.51.100.99' }),
        ctx
      )
      expect(res.status).toBe(201)
    })

    it('hashes IP rather than storing it raw', async () => {
      await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A, ip: '203.0.113.42' }), ctx)
      const row = __mockState.tables.campaign_leads.rows[0] as Record<string, unknown>
      expect(row.ip_hash).toMatch(/^[0-9a-f]{64}$/)
      expect(row.ip_hash).not.toContain('203.0.113.42')
    })

    it('returns 500 if Supabase env vars are missing', async () => {
      delete process.env.SUPABASE_URL
      const res = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A }), ctx)
      expect(res.status).toBe(500)
    })

    it('returns 400 with field error when validation fails (forwarded from validateLead)', async () => {
      const res = await campaignLead(
        makeRequest(validBody({ email: 'nope' }), { idempotencyKey: KEY_A }),
        ctx
      )
      expect(res.status).toBe(400)
      const json = await res.json()
      expect(json.error).toMatch(/email/i)
    })

    it('respects BRAND_PORTAL_URL env override', async () => {
      process.env.BRAND_PORTAL_URL = 'https://staging-brands.groflo.io'
      const res = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A }), ctx)
      const json = await res.json()
      expect(json.redirectUrl).toContain('staging-brands.groflo.io')
    })

    it('reflects allowed origin in CORS header, falls back to canonical for unknown origin', async () => {
      const ok = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A, origin: 'https://groflo.io' }), ctx)
      expect(ok.headers.get('Access-Control-Allow-Origin')).toBe('https://groflo.io')

      resetMockState()
      const evil = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_B, origin: 'https://evil.example' }), ctx)
      expect(evil.headers.get('Access-Control-Allow-Origin')).toBe('https://groflo.io')
    })

    it('still allows thrift.ly origin during the rebrand transition window', async () => {
      const res = await campaignLead(
        makeRequest(validBody(), { idempotencyKey: KEY_A, origin: 'https://thrift.ly' }),
        ctx
      )
      expect(res.headers.get('Access-Control-Allow-Origin')).toBe('https://thrift.ly')
    })

    it('logs structured error and returns 500 on Supabase insert failure', async () => {
      __mockState.forceError = { op: 'insert', error: { message: 'pg down' } }
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await campaignLead(makeRequest(validBody(), { idempotencyKey: KEY_A }), ctx)
      expect(res.status).toBe(500)
      expect(errSpy).toHaveBeenCalled()
      errSpy.mockRestore()
    })
  })
})
