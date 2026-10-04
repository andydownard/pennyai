// api.ts gained one thing: an optional Clerk Bearer header.
//
// The load-bearing assertion is the first one. With VITE_CLERK_PUBLISHABLE_KEY
// unset, fetch must receive the caller's exact options object — not a copy, not
// a Headers-normalized version — so every existing request is byte-for-byte what
// it is today and the 401 path is untouched.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { fetchAPI, fetchAction, postJSON } from './api'
import { setClerkTokenGetter, _resetClerkTokenRegistry } from './auth/clerkToken'

function okResponse(body: unknown = { ok: true }) {
  return {
    ok: true,
    status: 200,
    json: async () => body,
  } as unknown as Response
}

function authHeaderOf(init?: RequestInit): string | null {
  return new Headers(init?.headers).get('Authorization')
}

describe('api auth header', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    _resetClerkTokenRegistry()
    fetchMock = vi.fn(async () => okResponse())
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  describe('with Clerk off (the shipping default)', () => {
    beforeEach(() => vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', ''))

    it('passes the caller options through by identity — no header added, nothing rewritten', async () => {
      const options: RequestInit = { method: 'POST', headers: { 'Content-Type': 'application/json' } }
      await fetchAPI('/api/list', options)
      expect(fetchMock).toHaveBeenCalledTimes(1)
      expect(fetchMock.mock.calls[0][0]).toBe('/api/list')
      // Identity, not deep-equality: proves no copy and no normalization.
      expect(fetchMock.mock.calls[0][1]).toBe(options)
    })

    it('passes undefined options through as undefined', async () => {
      await fetchAPI('/api/list')
      expect(fetchMock.mock.calls[0][1]).toBeUndefined()
    })

    it('never consults the token getter', async () => {
      const getter = vi.fn(async () => 'tok')
      setClerkTokenGetter(getter)
      await fetchAction('/api/list/clear', { method: 'POST' })
      expect(getter).not.toHaveBeenCalled()
    })

    it('leaves postJSON exactly as it was', async () => {
      await postJSON('/api/nps', { score: 9 })
      const init = fetchMock.mock.calls[0][1] as RequestInit
      expect(init.method).toBe('POST')
      expect(authHeaderOf(init)).toBeNull()
      expect(init.body).toBe(JSON.stringify({ score: 9 }))
    })
  })

  describe('with Clerk on', () => {
    beforeEach(() => vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc'))

    it('adds the Bearer header from the registered getter', async () => {
      setClerkTokenGetter(async () => 't')
      await fetchAPI('/api/list')
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBe('Bearer t')
    })

    it('adds it on fetchAction too', async () => {
      setClerkTokenGetter(async () => 't2')
      await fetchAction('/auth/clerk/exchange', { method: 'POST' })
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBe('Bearer t2')
    })

    it('preserves the caller\'s other headers and options', async () => {
      setClerkTokenGetter(async () => 't')
      await fetchAPI('/api/list', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: '{}' })
      const init = fetchMock.mock.calls[0][1] as RequestInit
      expect(init.method).toBe('PATCH')
      expect(init.body).toBe('{}')
      expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    })

    it('never overwrites a caller-supplied Authorization header', async () => {
      setClerkTokenGetter(async () => 'clerk-token')
      await fetchAPI('/api/list', { headers: { Authorization: 'Bearer caller-owns-this' } })
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBe('Bearer caller-owns-this')
    })

    it('adds no header when there is no getter', async () => {
      await fetchAPI('/api/list')
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBeNull()
    })

    it('adds no header when the getter returns null', async () => {
      setClerkTokenGetter(async () => null)
      await fetchAPI('/api/list')
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBeNull()
    })

    it('still sends the request when the getter throws', async () => {
      setClerkTokenGetter(async () => {
        throw new Error('clerk down')
      })
      await expect(fetchAPI('/api/list')).resolves.toEqual({ ok: true })
      expect(authHeaderOf(fetchMock.mock.calls[0][1] as RequestInit)).toBeNull()
    })
  })

  describe('401 handling is unchanged', () => {
    beforeEach(() => vi.stubEnv('VITE_CLERK_PUBLISHABLE_KEY', 'pk_test_abc'))

    it('reports the server error text from fetchAction', async () => {
      fetchMock.mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => ({ error: 'This account is already connected to a different Rally sign-in.' }),
      } as unknown as Response)
      const res = await fetchAction('/auth/clerk/exchange', { method: 'POST' })
      expect(res.ok).toBe(false)
      expect(res.status).toBe(409)
      expect(res.error).toBe('This account is already connected to a different Rally sign-in.')
    })
  })
})
