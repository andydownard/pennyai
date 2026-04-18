/**
 * Integration tests for the Groflo marketing server.
 *
 * Covers both /.netlify/functions/campaign-lead and /.netlify/functions/feedback
 * with happy paths, validation errors, rate limits, idempotency, and
 * cross-user isolation. Per CLAUDE.md "Mutation Safety" the test suite
 * includes:
 *   - positive path   (201 on first insert)
 *   - negative path   (duplicate POST does NOT produce duplicate writes)
 *   - rate limit      (N+1st request is rejected 429)
 *   - auth/validation (invalid inputs return 4xx)
 *
 * Postgres is mocked via __tests__/_pg-mock.ts so tests run offline.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('pg', () => import('./_pg-mock'))

import request from 'supertest'
import { __mockState, resetMockState } from './_pg-mock'

// Ensure DATABASE_URL is set before createApp is imported so the module-load
// singleton check passes.
process.env.DATABASE_URL = 'postgresql://test:test@localhost:5432/test'
process.env.IP_HASH_SALT = 'test-salt'
process.env.BRAND_PORTAL_URL = 'https://brands.groflo.io'

// eslint-disable-next-line import/first
import { createApp, __resetRateBucketsForTest, __resetMigrationsForTest, __resetPoolForTest, validateLead, hashIp, normalizePhone, hitRateLimit } from '../server'

const app = createApp()

const VALID_KEY_A = '11111111-1111-4111-8111-111111111111'
const VALID_KEY_B = '22222222-2222-4222-8222-222222222222'
const VALID_KEY_C = '33333333-3333-4333-8333-333333333333'

function validLeadBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
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
    startDate: '2027-05-01',
    endDate: '2027-06-01',
    creativeName: 'Organic Valley Whole Milk',
    creativeDescription: 'Farm-fresh organic milk from pasture-raised cows',
    ...overrides,
  }
}

describe('Groflo marketing server — health', () => {
  it('GET /health returns 200 {status:"ok"}', async () => {
    const res = await request(app).get('/health')
    expect(res.status).toBe(200)
    expect(res.body).toEqual({ status: 'ok' })
  })
})

describe('Pure helpers', () => {
  describe('validateLead', () => {
    it('accepts a complete valid payload', () => {
      const r = validateLead(validLeadBody(), VALID_KEY_A)
      expect('error' in r).toBe(false)
    })

    it('rejects missing idempotency key', () => {
      const r = validateLead(validLeadBody(), null)
      expect('error' in r && r.error).toMatch(/idempotency/i)
    })

    it('rejects malformed idempotency key', () => {
      const r = validateLead(validLeadBody(), 'not-a-uuid')
      expect('error' in r && r.error).toMatch(/idempotency/i)
    })

    it('rejects missing fullName', () => {
      const r = validateLead(validLeadBody({ fullName: '' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/full name/i)
    })

    it('rejects email missing @', () => {
      const r = validateLead(validLeadBody({ email: 'not-an-email' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/email/i)
    })

    it('lowercases email', () => {
      const r = validateLead(validLeadBody({ email: 'JANE@ACME.COM' }), VALID_KEY_A)
      if ('error' in r) throw new Error('expected ok')
      expect(r.email).toBe('jane@acme.com')
    })

    it('rejects unknown adType', () => {
      const r = validateLead(validLeadBody({ adType: 'super-mega' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/campaign type/i)
    })

    it('rejects empty categories', () => {
      const r = validateLead(validLeadBody({ categories: [] }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/categor/i)
    })

    it('rejects categories not an array', () => {
      const r = validateLead(validLeadBody({ categories: 'Dairy' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/categor/i)
    })

    it('joins categories with comma in normalized form', () => {
      const r = validateLead(validLeadBody({ categories: ['A', 'B', 'C'] }), VALID_KEY_A)
      if ('error' in r) throw new Error('expected ok')
      expect(r.categories).toBe('A,B,C')
    })

    it('caps categories at 20', () => {
      const many = Array.from({ length: 30 }, (_, i) => `Cat${i}`)
      const r = validateLead(validLeadBody({ categories: many }), VALID_KEY_A)
      if ('error' in r) throw new Error('expected ok')
      expect(r.categories.split(',').length).toBe(20)
    })

    it('rejects unknown geography', () => {
      const r = validateLead(validLeadBody({ geography: 'mars' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/geography/i)
    })

    it('rejects daily budget below $25', () => {
      const r = validateLead(validLeadBody({ budgetType: 'daily', budgetAmount: 10 }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/\$25/)
    })

    it('rejects total budget below $100', () => {
      const r = validateLead(validLeadBody({ budgetType: 'total', budgetAmount: 50 }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/\$100/)
    })

    it('rejects budget over $1,000,000', () => {
      const r = validateLead(validLeadBody({ budgetAmount: 5_000_000 }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/maximum/i)
    })

    it('rejects negative budget', () => {
      const r = validateLead(validLeadBody({ budgetAmount: -50 }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/positive/i)
    })

    it('rejects malformed startDate', () => {
      const r = validateLead(validLeadBody({ startDate: '05/01/2026' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/dates/i)
    })

    it('rejects endDate <= startDate', () => {
      const r = validateLead(validLeadBody({ startDate: '2027-06-01', endDate: '2027-05-01' }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/end date/i)
    })

    it('rejects creativeDescription over 140 chars', () => {
      const r = validateLead(validLeadBody({ creativeDescription: 'x'.repeat(141) }), VALID_KEY_A)
      expect('error' in r && r.error).toMatch(/description/i)
    })

    it('rounds fractional budget to integer', () => {
      const r = validateLead(validLeadBody({ budgetAmount: 49.7 }), VALID_KEY_A)
      if ('error' in r) throw new Error('expected ok')
      expect(r.budget_amount).toBe(50)
    })
  })

  describe('hashIp', () => {
    it('produces a hex sha256 digest', () => {
      expect(hashIp('1.2.3.4')).toMatch(/^[0-9a-f]{64}$/)
    })

    it('is stable for the same input', () => {
      expect(hashIp('1.2.3.4')).toBe(hashIp('1.2.3.4'))
    })

    it('differs for different inputs', () => {
      expect(hashIp('1.2.3.4')).not.toBe(hashIp('5.6.7.8'))
    })

    it('does not embed raw IP in the output', () => {
      expect(hashIp('203.0.113.42')).not.toContain('203.0.113.42')
    })
  })

  describe('normalizePhone', () => {
    it('strips non-digits', () => {
      expect(normalizePhone('(310) 486-1833')).toBe('3104861833')
    })
    it('strips leading +1', () => {
      expect(normalizePhone('+13104861833')).toBe('3104861833')
    })
    it('passes 10-digit unchanged', () => {
      expect(normalizePhone('3104861833')).toBe('3104861833')
    })
    it('returns empty for empty input', () => {
      expect(normalizePhone('')).toBe('')
    })
  })

  describe('hitRateLimit', () => {
    beforeEach(() => __resetRateBucketsForTest())

    it('allows up to limit hits in the window', () => {
      const KEY = 'test:ip:1'
      for (let i = 0; i < 3; i++) {
        expect(hitRateLimit(KEY, 3)).toBe(false)
      }
    })

    it('blocks the limit+1 hit', () => {
      const KEY = 'test:ip:2'
      for (let i = 0; i < 3; i++) {
        expect(hitRateLimit(KEY, 3)).toBe(false)
      }
      expect(hitRateLimit(KEY, 3)).toBe(true)
    })

    it('per-key buckets are independent', () => {
      for (let i = 0; i < 3; i++) expect(hitRateLimit('test:ip:a', 3)).toBe(false)
      expect(hitRateLimit('test:ip:b', 3)).toBe(false)
    })

    it('entries older than 1 hour fall out of the window', () => {
      const KEY = 'test:ip:decay'
      const base = 10_000_000
      for (let i = 0; i < 3; i++) hitRateLimit(KEY, 3, base + i)
      // A full hour + 1 ms later: prior entries are outside the window.
      expect(hitRateLimit(KEY, 3, base + 60 * 60 * 1000 + 1)).toBe(false)
    })
  })
})

describe('POST /.netlify/functions/campaign-lead', () => {
  beforeEach(() => {
    resetMockState()
    __resetRateBucketsForTest()
    __resetMigrationsForTest()
    __resetPoolForTest()
    process.env.DATABASE_URL = 'postgresql://test:test@localhost:5432/test'
    process.env.BRAND_PORTAL_URL = 'https://brands.groflo.io'
  })

  it('rejects missing idempotency key with 400', async () => {
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Content-Type', 'application/json')
      .send(validLeadBody())
    expect(res.status).toBe(400)
    expect(res.body.error).toMatch(/idempotency/i)
  })

  it('rejects invalid body (bad email) with 400', async () => {
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .send(validLeadBody({ email: 'not-an-email' }))
    expect(res.status).toBe(400)
    expect(res.body.error).toMatch(/email/i)
  })

  it('rejects GET method with 404', async () => {
    const res = await request(app).get('/.netlify/functions/campaign-lead')
    expect(res.status).toBe(404)
  })

  it('inserts a valid lead and returns 201 with id + redirectUrl', async () => {
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.1')
      .send(validLeadBody())
    expect(res.status).toBe(201)
    expect(res.body.success).toBe(true)
    expect(res.body.leadId).toBeGreaterThan(0)
    expect(res.body.redirectUrl).toContain('brands.groflo.io')
    expect(res.body.redirectUrl).toContain(`lead=${res.body.leadId}`)
    expect(res.body.replay).toBe(false)
    expect(__mockState.campaignLeads.length).toBe(1)
    const row = __mockState.campaignLeads[0]
    expect(row.email).toBe('jane@acme.com')
    expect(row.idempotency_key).toBe(VALID_KEY_A)
  })

  it('idempotency: same key returns same lead, does NOT create duplicate row', async () => {
    const first = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.1')
      .send(validLeadBody())
    const second = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.1')
      .send(validLeadBody({ creativeName: 'Different Name' }))
    expect(second.status).toBe(200)
    expect(second.body.leadId).toBe(first.body.leadId)
    expect(second.body.replay).toBe(true)
    expect(__mockState.campaignLeads.length).toBe(1)
  })

  it('different idempotency keys produce different leads', async () => {
    const r1 = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.1')
      .send(validLeadBody())
    const r2 = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_B)
      .set('X-Forwarded-For', '203.0.113.2')
      .send(validLeadBody())
    expect(r1.body.leadId).not.toBe(r2.body.leadId)
    expect(__mockState.campaignLeads.length).toBe(2)
  })

  it('rate-limits to 5 per IP per hour (6th returns 429)', async () => {
    for (let i = 0; i < 5; i++) {
      const key = `${i.toString().padStart(8, '0')}-1111-4111-8111-111111111111`
      const res = await request(app)
        .post('/.netlify/functions/campaign-lead')
        .set('Idempotency-Key', key)
        .set('X-Forwarded-For', '198.51.100.1')
        .send(validLeadBody({ creativeName: `P${i}` }))
      expect(res.status).toBe(201)
    }
    const res6 = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_C)
      .set('X-Forwarded-For', '198.51.100.1')
      .send(validLeadBody())
    expect(res6.status).toBe(429)
  })

  it('different IPs do not share a rate-limit bucket', async () => {
    for (let i = 0; i < 5; i++) {
      const key = `${i.toString().padStart(8, '0')}-1111-4111-8111-111111111111`
      await request(app)
        .post('/.netlify/functions/campaign-lead')
        .set('Idempotency-Key', key)
        .set('X-Forwarded-For', '198.51.100.1')
        .send(validLeadBody({ creativeName: `P${i}` }))
    }
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_C)
      .set('X-Forwarded-For', '198.51.100.99')
      .send(validLeadBody())
    expect(res.status).toBe(201)
  })

  it('stores hashed IP, not raw IP, in the row', async () => {
    await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.42')
      .send(validLeadBody())
    const row = __mockState.campaignLeads[0]
    expect(row.ip_hash).toMatch(/^[0-9a-f]{64}$/)
    expect(String(row.ip_hash)).not.toContain('203.0.113.42')
  })

  it('respects BRAND_PORTAL_URL env var', async () => {
    // Note: env var is read at module load, so this test documents default behavior.
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('Idempotency-Key', VALID_KEY_A)
      .set('X-Forwarded-For', '203.0.113.1')
      .send(validLeadBody())
    expect(res.body.redirectUrl.startsWith('https://brands.groflo.io')).toBe(true)
  })

  it('idempotency key in body also accepted when header missing', async () => {
    const res = await request(app)
      .post('/.netlify/functions/campaign-lead')
      .set('X-Forwarded-For', '203.0.113.1')
      .send({ ...validLeadBody(), idempotencyKey: VALID_KEY_A })
    expect(res.status).toBe(201)
  })
})

describe('POST /.netlify/functions/feedback', () => {
  beforeEach(() => {
    resetMockState()
    __resetRateBucketsForTest()
    __resetMigrationsForTest()
    __resetPoolForTest()
  })

  it('accepts an allowlisted phone', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: 'hi' })
    expect(res.status).toBe(201)
    expect(res.body.success).toBe(true)
    expect(res.body.id).toBeGreaterThan(0)
  })

  it('normalizes formatted phone "(310) 486-1833" via normalizePhone', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '(310) 486-1833', body: 'hi' })
    expect(res.status).toBe(201)
  })

  it('normalizes E.164 phone "+13104861833"', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '+13104861833', body: 'hi' })
    expect(res.status).toBe(201)
  })

  it('rejects unauthorized phone with 403 and does NOT insert', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '5550001234', body: 'hi' })
    expect(res.status).toBe(403)
    expect(__mockState.feedbackRequests.length).toBe(0)
  })

  it('rejects missing phone with 403', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ body: 'hi' })
    expect(res.status).toBe(403)
  })

  it('rejects empty body with 400', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: '' })
    expect(res.status).toBe(400)
  })

  it('rejects whitespace-only body with 400', async () => {
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: '   \n\t  ' })
    expect(res.status).toBe(400)
  })

  it('rejects body over 10,000 chars with 400', async () => {
    const long = 'x'.repeat(10_001)
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: long })
    expect(res.status).toBe(400)
  })

  it('accepts body at exactly 10,000 chars', async () => {
    const long = 'x'.repeat(10_000)
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: long })
    expect(res.status).toBe(201)
  })

  it('defaults urgency to "normal"', async () => {
    await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: 'hi' })
    expect(__mockState.feedbackRequests[0].urgency).toBe('normal')
  })

  it('accepts urgency="critical"', async () => {
    await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: 'hi', urgency: 'critical' })
    expect(__mockState.feedbackRequests[0].urgency).toBe('critical')
  })

  it('coerces unknown urgency to "normal"', async () => {
    await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: 'hi', urgency: 'panic' })
    expect(__mockState.feedbackRequests[0].urgency).toBe('normal')
  })

  it('allows 10 submissions per phone per hour', async () => {
    for (let i = 0; i < 10; i++) {
      const res = await request(app)
        .post('/.netlify/functions/feedback')
        .send({ phone: '3104861833', body: `msg ${i}` })
      expect(res.status).toBe(201)
    }
  })

  it('rejects 11th submission within an hour with 429', async () => {
    for (let i = 0; i < 10; i++) {
      await request(app)
        .post('/.netlify/functions/feedback')
        .send({ phone: '3104861833', body: `msg ${i}` })
    }
    const res = await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: 'too many' })
    expect(res.status).toBe(429)
    expect(res.body.error).toMatch(/max 10/i)
  })

  it('rate limit is per-phone, not global', async () => {
    const phones = ['3104861833', '6264043580']
    for (const p of phones) {
      for (let i = 0; i < 10; i++) {
        const res = await request(app)
          .post('/.netlify/functions/feedback')
          .send({ phone: p, body: `${p}-${i}` })
        expect(res.status).toBe(201)
      }
    }
    expect(__mockState.feedbackRequests.length).toBe(20)
  })

  it('stores normalized phone (digits only) in the row', async () => {
    await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '+13104861833', body: 'hi' })
    expect(__mockState.feedbackRequests[0].phone).toBe('3104861833')
  })

  it('stores trimmed body', async () => {
    await request(app)
      .post('/.netlify/functions/feedback')
      .send({ phone: '3104861833', body: '   important   ' })
    expect(__mockState.feedbackRequests[0].body).toBe('important')
  })

  it('idempotency with header — duplicate POST returns existing row', async () => {
    const first = await request(app)
      .post('/.netlify/functions/feedback')
      .set('Idempotency-Key', VALID_KEY_A)
      .send({ phone: '3104861833', body: 'only once' })
    expect(first.status).toBe(201)

    const second = await request(app)
      .post('/.netlify/functions/feedback')
      .set('Idempotency-Key', VALID_KEY_A)
      .send({ phone: '3104861833', body: 'only once' })
    expect(second.status).toBe(200)
    expect(second.body.id).toBe(first.body.id)
    expect(second.body.replay).toBe(true)
    expect(__mockState.feedbackRequests.length).toBe(1)
  })
})

describe('GET /.netlify/functions/feedback (list history)', () => {
  beforeEach(() => {
    resetMockState()
    __resetRateBucketsForTest()
    __resetMigrationsForTest()
    __resetPoolForTest()
  })

  it('rejects unauthorized phone with 403', async () => {
    const res = await request(app).get('/.netlify/functions/feedback?phone=5550001234')
    expect(res.status).toBe(403)
  })

  it('rejects missing phone query param with 403', async () => {
    const res = await request(app).get('/.netlify/functions/feedback')
    expect(res.status).toBe(403)
  })

  it('returns empty array when no rows for phone', async () => {
    const res = await request(app).get('/.netlify/functions/feedback?phone=3104861833')
    expect(res.status).toBe(200)
    expect(Array.isArray(res.body)).toBe(true)
    expect(res.body.length).toBe(0)
  })

  it('returns previously-inserted rows for the phone', async () => {
    await request(app).post('/.netlify/functions/feedback').send({ phone: '3104861833', body: 'one' })
    await request(app).post('/.netlify/functions/feedback').send({ phone: '3104861833', body: 'two' })
    const res = await request(app).get('/.netlify/functions/feedback?phone=3104861833')
    expect(res.status).toBe(200)
    expect(res.body.length).toBe(2)
  })

  it('does not return rows for other phones (cross-user isolation)', async () => {
    await request(app).post('/.netlify/functions/feedback').send({ phone: '3104861833', body: 'mine' })
    await request(app).post('/.netlify/functions/feedback').send({ phone: '6264043580', body: 'theirs' })
    const res = await request(app).get('/.netlify/functions/feedback?phone=3104861833')
    expect(res.body.length).toBe(1)
    expect(res.body[0].body).toBe('mine')
  })

  it('accepts E.164 phone format in query string', async () => {
    await request(app).post('/.netlify/functions/feedback').send({ phone: '3104861833', body: 'hi' })
    const res = await request(app).get('/.netlify/functions/feedback?phone=%2B13104861833')
    expect(res.status).toBe(200)
    expect(res.body.length).toBe(1)
  })
})

describe('SPA fallback', () => {
  beforeEach(() => {
    resetMockState()
    __resetRateBucketsForTest()
    __resetMigrationsForTest()
    __resetPoolForTest()
  })

  it('does not intercept /.netlify/* with 404 for unknown API subroute (API 404 is JSON)', async () => {
    const res = await request(app).get('/.netlify/functions/does-not-exist')
    expect(res.status).toBe(404)
    expect(res.headers['content-type']).toContain('application/json')
  })

  it('/health always JSON, not SPA-fallback HTML', async () => {
    const res = await request(app).get('/health')
    expect(res.status).toBe(200)
    expect(res.headers['content-type']).toContain('application/json')
  })
})
