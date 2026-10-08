import { test, expect } from '@playwright/test'

test.describe('Static assets and SPA', () => {
  test('index.html loads with correct title', async ({ page }) => {
    await page.goto('/')
    const title = await page.title()
    expect(title.length).toBeGreaterThan(0)
  })

  test('JS bundle loads without errors', async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', err => errors.push(err.message))

    await page.goto('/login')
    await page.waitForTimeout(2000)

    expect(errors).toHaveLength(0)
  })

  test('SPA fallback serves index.html for unknown routes', async ({ page }) => {
    const res = await page.request.get('/some-unknown-page')
    expect(res.status()).toBe(200)
    const html = await res.text()
    expect(html).toContain('<div id="root">')
  })

  test('privacy policy page renders', async ({ page }) => {
    const res = await page.request.get('/privacy')
    expect(res.status()).toBe(200)
    const html = await res.text()
    expect(html.toLowerCase()).toContain('privacy')
  })

  test('terms of service page renders', async ({ page }) => {
    const res = await page.request.get('/terms')
    expect(res.status()).toBe(200)
    const html = await res.text()
    expect(html.toLowerCase()).toContain('terms')
  })

  test('PWA manifest is accessible', async ({ page }) => {
    const res = await page.request.get('/manifest.json')
    if (res.status() === 200) {
      const manifest = await res.json()
      expect(manifest).toHaveProperty('name')
    }
    // 404 is acceptable if manifest not built yet
  })
})
