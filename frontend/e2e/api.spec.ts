import { test, expect } from '@playwright/test'
import { login, getSessionCookie } from './helpers'

test.describe('API endpoints via browser', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('GET /health returns ok', async ({ page }) => {
    const res = await page.request.get('/health')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body.status).toBe('ok')
  })

  test('GET /api/config returns config', async ({ page }) => {
    const res = await page.request.get('/api/config')
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(typeof body.clerk_enabled).toBe('boolean')
    expect(body).not.toHaveProperty('google_client_id')
  })

  test('GET /api/list returns array', async ({ page }) => {
    const session = await getSessionCookie(page)
    const res = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
  })

  test('GET /api/settings returns user settings', async ({ page }) => {
    const session = await getSessionCookie(page)
    const res = await page.request.get('/api/settings', {
      headers: { Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('penny_slider')
  })

  test('GET /api/saved-lists returns array', async ({ page }) => {
    const session = await getSessionCookie(page)
    const res = await page.request.get('/api/saved-lists', {
      headers: { Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(Array.isArray(body)).toBe(true)
  })

  test('GET /api/referral returns referral info', async ({ page }) => {
    const session = await getSessionCookie(page)
    const res = await page.request.get('/api/referral', {
      headers: { Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('code')
  })

  test('GET /api/nps/check returns eligibility', async ({ page }) => {
    const session = await getSessionCookie(page)
    const res = await page.request.get('/api/nps/check', {
      headers: { Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)
    const body = await res.json()
    expect(body).toHaveProperty('show_nps')
  })

  test('unknown API route returns 404 JSON', async ({ page }) => {
    const res = await page.request.get('/api/nonexistent')
    expect(res.status()).toBe(404)
    const body = await res.json()
    expect(body.error).toBe('Not found')
  })

  test('unauthenticated API calls return 401', async ({ page }) => {
    // Clear cookies
    await page.context().clearCookies()
    const res = await page.request.get('/api/list')
    expect(res.status()).toBe(401)
  })
})

test.describe('Security headers', () => {
  test('responses include security headers', async ({ page }) => {
    const res = await page.request.get('/health')
    const headers = res.headers()
    expect(headers['x-content-type-options']).toBe('nosniff')
    expect(headers['x-frame-options']).toBe('DENY')
    expect(headers['referrer-policy']).toBe('strict-origin-when-cross-origin')
  })
})
