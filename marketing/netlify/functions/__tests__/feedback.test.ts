import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@supabase/supabase-js', () => import('./_supabase-mock'))

import { __mockState, resetMockState } from './_supabase-mock'
import feedback from '../feedback.mts'

const ALLOWED_PHONE = '3104861833'
const ALLOWED_PHONE_E164 = '+13104861833'
const ALLOWED_PHONE_FORMATTED = '(310) 486-1833'
const UNAUTH_PHONE = '5550001234'

function postBody(body: unknown): Request {
  return new Request('https://groflo.io/.netlify/functions/feedback', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: typeof body === 'string' ? body : JSON.stringify(body),
  })
}

function getReq(qs: string): Request {
  return new Request(`https://groflo.io/.netlify/functions/feedback${qs}`, {
    method: 'GET',
  })
}

const ctx = {} as Parameters<typeof feedback>[1]

describe('feedback Netlify function', () => {
  beforeEach(() => {
    resetMockState()
    process.env.SUPABASE_URL = 'https://test.supabase.co'
    process.env.SUPABASE_SERVICE_ROLE_KEY = 'service-role-test-key'
  })

  describe('CORS + method handling', () => {
    it('responds 204 to OPTIONS preflight with CORS headers', async () => {
      const req = new Request('https://groflo.io/.netlify/functions/feedback', { method: 'OPTIONS' })
      const res = await feedback(req, ctx)
      expect(res.status).toBe(204)
      expect(res.headers.get('Access-Control-Allow-Origin')).toBe('*')
      expect(res.headers.get('Access-Control-Allow-Methods')).toContain('POST')
    })

    it('rejects DELETE with 405', async () => {
      const req = new Request('https://groflo.io/.netlify/functions/feedback', { method: 'DELETE' })
      const res = await feedback(req, ctx)
      expect(res.status).toBe(405)
    })

    it('rejects PUT with 405', async () => {
      const req = new Request('https://groflo.io/.netlify/functions/feedback', { method: 'PUT' })
      const res = await feedback(req, ctx)
      expect(res.status).toBe(405)
    })

    it('always sets CORS headers on success', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hello' }), ctx)
      expect(res.headers.get('Access-Control-Allow-Origin')).toBe('*')
    })

    it('always sets CORS headers on error', async () => {
      const res = await feedback(postBody({ phone: UNAUTH_PHONE, body: 'hello' }), ctx)
      expect(res.headers.get('Access-Control-Allow-Origin')).toBe('*')
    })
  })

  describe('POST authorization (phone allowlist)', () => {
    it('accepts a raw 10-digit allowlisted phone', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(201)
    })

    it('accepts an E.164 (+1...) allowlisted phone via normalization', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE_E164, body: 'hi' }), ctx)
      expect(res.status).toBe(201)
    })

    it('accepts a formatted "(310) 486-1833" allowlisted phone via normalization', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE_FORMATTED, body: 'hi' }), ctx)
      expect(res.status).toBe(201)
    })

    it('rejects an unauthorized phone with 403', async () => {
      const res = await feedback(postBody({ phone: UNAUTH_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(403)
    })

    it('rejects missing phone with 403', async () => {
      const res = await feedback(postBody({ body: 'hi' }), ctx)
      expect(res.status).toBe(403)
    })

    it('does not write anything to Supabase when phone is unauthorized', async () => {
      await feedback(postBody({ phone: UNAUTH_PHONE, body: 'hi' }), ctx)
      expect(__mockState.tables.feedback_requests.rows.length).toBe(0)
      expect(__mockState.insertCalls.length).toBe(0)
    })
  })

  describe('POST body validation', () => {
    it('rejects empty body with 400', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: '' }), ctx)
      expect(res.status).toBe(400)
    })

    it('rejects whitespace-only body with 400', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: '   \n\t  ' }), ctx)
      expect(res.status).toBe(400)
    })

    it('rejects body over 10,000 chars with 400', async () => {
      const long = 'a'.repeat(10_001)
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: long }), ctx)
      expect(res.status).toBe(400)
    })

    it('accepts body at exactly 10,000 chars', async () => {
      const long = 'a'.repeat(10_000)
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: long }), ctx)
      expect(res.status).toBe(201)
    })

    it('defaults urgency to "normal" when not provided', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      const row = __mockState.tables.feedback_requests.rows[0] as Record<string, unknown>
      expect(row.urgency).toBe('normal')
    })

    it('accepts urgency "critical" when explicitly set', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi', urgency: 'critical' }), ctx)
      const row = __mockState.tables.feedback_requests.rows[0] as Record<string, unknown>
      expect(row.urgency).toBe('critical')
    })

    it('coerces unknown urgency value to "normal"', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi', urgency: 'panic' }), ctx)
      const row = __mockState.tables.feedback_requests.rows[0] as Record<string, unknown>
      expect(row.urgency).toBe('normal')
    })

    it('returns 500 with safe message on malformed JSON', async () => {
      const req = new Request('https://groflo.io/.netlify/functions/feedback', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: 'not json',
      })
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await feedback(req, ctx)
      expect(res.status).toBe(500)
      const json = await res.json()
      expect(json.error).toMatch(/something went wrong/i)
      errSpy.mockRestore()
    })
  })

  describe('POST rate limiting', () => {
    it('allows 10 submissions per phone per hour', async () => {
      for (let i = 0; i < 10; i++) {
        const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: `msg ${i}` }), ctx)
        expect(res.status).toBe(201)
      }
      expect(__mockState.tables.feedback_requests.rows.length).toBe(10)
    })

    it('rejects 11th submission within an hour with 429', async () => {
      for (let i = 0; i < 10; i++) {
        await feedback(postBody({ phone: ALLOWED_PHONE, body: `msg ${i}` }), ctx)
      }
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'one too many' }), ctx)
      expect(res.status).toBe(429)
      const json = await res.json()
      expect(json.error).toMatch(/max 10/i)
    })

    it('does not insert when rate limit triggers', async () => {
      for (let i = 0; i < 10; i++) {
        await feedback(postBody({ phone: ALLOWED_PHONE, body: `msg ${i}` }), ctx)
      }
      const beforeCount = __mockState.insertCalls.length
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'blocked' }), ctx)
      expect(__mockState.insertCalls.length).toBe(beforeCount)
    })

    it('rate limit is per-phone, not global', async () => {
      // Both allowed phones can each submit 10 independently
      const phones = ['3104861833', '6264043580']
      for (const p of phones) {
        for (let i = 0; i < 10; i++) {
          const res = await feedback(postBody({ phone: p, body: `${p}-${i}` }), ctx)
          expect(res.status).toBe(201)
        }
      }
      expect(__mockState.tables.feedback_requests.rows.length).toBe(20)
    })
  })

  describe('POST persistence', () => {
    it('stores normalized phone (digits only) in the row', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE_E164, body: 'hi' }), ctx)
      const row = __mockState.tables.feedback_requests.rows[0] as Record<string, unknown>
      expect(row.phone).toBe(ALLOWED_PHONE)
    })

    it('stores trimmed body', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: '   important note   ' }), ctx)
      const row = __mockState.tables.feedback_requests.rows[0] as Record<string, unknown>
      expect(row.body).toBe('important note')
    })

    it('returns 201 with success + id on insert', async () => {
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(201)
      const json = await res.json()
      expect(json.success).toBe(true)
      expect(json.id).toBeGreaterThan(0)
    })

    it('returns 500 on Supabase insert error', async () => {
      __mockState.forceError = { op: 'insert', error: { message: 'pg unreachable' } }
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(500)
      const json = await res.json()
      expect(json.error).toMatch(/failed to save/i)
      errSpy.mockRestore()
    })
  })

  describe('GET (list by phone)', () => {
    it('returns 403 when phone is not on allowlist', async () => {
      const res = await feedback(getReq(`?phone=${UNAUTH_PHONE}`), ctx)
      expect(res.status).toBe(403)
    })

    it('returns 403 when phone query param is missing', async () => {
      const res = await feedback(getReq(''), ctx)
      expect(res.status).toBe(403)
    })

    it('returns 200 with empty array when no rows for phone', async () => {
      const res = await feedback(getReq(`?phone=${ALLOWED_PHONE}`), ctx)
      expect(res.status).toBe(200)
      const json = await res.json()
      expect(Array.isArray(json)).toBe(true)
      expect(json.length).toBe(0)
    })

    it('returns rows previously inserted for that phone', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'first' }), ctx)
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'second' }), ctx)
      const res = await feedback(getReq(`?phone=${ALLOWED_PHONE}`), ctx)
      const json = await res.json()
      expect(json.length).toBe(2)
    })

    it('does not return rows for other phones', async () => {
      await feedback(postBody({ phone: '3104861833', body: 'mine' }), ctx)
      await feedback(postBody({ phone: '6264043580', body: 'theirs' }), ctx)
      const res = await feedback(getReq(`?phone=3104861833`), ctx)
      const json = await res.json()
      expect(json.length).toBe(1)
      expect(json[0].body).toBe('mine')
    })

    it('accepts E.164 phone format in query string', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hello' }), ctx)
      const res = await feedback(getReq(`?phone=%2B1${ALLOWED_PHONE}`), ctx)
      expect(res.status).toBe(200)
      const json = await res.json()
      expect(json.length).toBe(1)
    })

    it('returns 500 on Supabase select error', async () => {
      __mockState.forceError = { op: 'select', error: { message: 'pg unreachable' } }
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await feedback(getReq(`?phone=${ALLOWED_PHONE}`), ctx)
      expect(res.status).toBe(500)
      errSpy.mockRestore()
    })
  })

  describe('Supabase config', () => {
    it('returns 500 if SUPABASE_URL is missing', async () => {
      delete process.env.SUPABASE_URL
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(500)
      errSpy.mockRestore()
    })

    it('returns 500 if SUPABASE_SERVICE_ROLE_KEY is missing', async () => {
      delete process.env.SUPABASE_SERVICE_ROLE_KEY
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const res = await feedback(postBody({ phone: ALLOWED_PHONE, body: 'hi' }), ctx)
      expect(res.status).toBe(500)
      errSpy.mockRestore()
    })
  })

  describe('idempotency gap (CLAUDE.md compliance)', () => {
    // Per CLAUDE.md "Idempotency & Mutation Safety" the POST handler must be
    // idempotent. It currently is not — a double-click creates two rows. The
    // rate limit caps abuse at 10/hr but does not solve the duplication
    // problem. This test documents the gap so it's visible to anyone running
    // the suite. When idempotency is added (e.g. Idempotency-Key header), this
    // assertion flips to expect 1 row, not 2.
    it('KNOWN GAP: duplicate POST currently creates two rows', async () => {
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'same message' }), ctx)
      await feedback(postBody({ phone: ALLOWED_PHONE, body: 'same message' }), ctx)
      const rows = __mockState.tables.feedback_requests.rows
      expect(rows.length).toBe(2)
    })
  })
})
