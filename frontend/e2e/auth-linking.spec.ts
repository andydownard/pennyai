import { test, expect } from '@playwright/test'
import { login, getSessionCookie } from './helpers'

test.describe('Auth linking on profile', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('sign-in methods section visible on profile', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign-in methods')).toBeVisible({ timeout: 10_000 })
  })

  test('phone method shows as linked for phone-login users', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Phone / SMS')).toBeVisible({ timeout: 10_000 })

    // The phone row should not have an "Add" button — user logged in with phone
    // Check that within the sign-in methods section, phone doesn't show "Add"
    const signinSection = page.locator('text=Sign-in methods').locator('..')
    const phoneAddBtn = signinSection.locator('button', { hasText: 'Add' }).filter({ has: page.locator('text=Phone') })
    await expect(phoneAddBtn).not.toBeVisible()
  })

  test('email Add button visible for phone-only users', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign-in methods')).toBeVisible({ timeout: 10_000 })

    // There should be at least one "Add" button in the sign-in methods section
    // (for email, since user logged in with phone only)
    const addButtons = page.getByRole('button', { name: 'Add' })
    await expect(addButtons.first()).toBeVisible()
  })

  test('clicking Add opens inline email linking form', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign-in methods')).toBeVisible({ timeout: 10_000 })

    // Click the first "Add" button (should be email for phone-login users)
    await page.getByRole('button', { name: 'Add' }).first().click()

    // Inline email form should appear with email input, Link and Cancel buttons
    await expect(page.getByPlaceholder('your@email.com').last()).toBeVisible()
    await expect(page.getByRole('button', { name: 'Link' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Cancel' })).toBeVisible()
  })

  test('cancel closes the email linking form', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign-in methods')).toBeVisible({ timeout: 10_000 })

    // Open email linking form
    await page.getByRole('button', { name: 'Add' }).first().click()
    await expect(page.getByRole('button', { name: 'Link' })).toBeVisible()

    // Cancel
    await page.getByRole('button', { name: 'Cancel' }).click()

    // Form should be gone
    await expect(page.getByRole('button', { name: 'Link' })).not.toBeVisible()
  })

  test('link button disabled when email input empty', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign-in methods')).toBeVisible({ timeout: 10_000 })

    // Open email linking form
    await page.getByRole('button', { name: 'Add' }).first().click()

    // Link button should be disabled when no email entered
    const linkBtn = page.getByRole('button', { name: 'Link' })
    await expect(linkBtn).toBeDisabled()
  })

  test('link email via API contract', async ({ page }) => {
    const session = await getSessionCookie(page)
    const testEmail = `test-${Date.now()}@example.com`

    // Link email via API
    const res = await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: testEmail },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })
    expect(res.status()).toBe(200)

    // Verify /auth/me now shows email in auth_methods
    const meRes = await page.request.get('/auth/me', {
      headers: { Cookie: `session=${session}` },
    })
    const meData = await meRes.json()
    expect(meData.auth_methods).toContain('email')
  })

  test('linking duplicate email returns conflict', async ({ page }) => {
    const session = await getSessionCookie(page)
    const testEmail = `dup-${Date.now()}@example.com`

    // Link email first time
    await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: testEmail },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })

    // Login as a different user
    await page.context().clearCookies()
    await login(page)
    const session2 = await getSessionCookie(page)

    // Try to link the same email — should be 409 conflict
    const res = await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: testEmail },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session2}` },
    })
    expect(res.status()).toBe(409)
  })

  test('link email requires authentication', async ({ page }) => {
    await page.context().clearCookies()

    const res = await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: 'test@example.com' },
      headers: { 'Content-Type': 'application/json' },
    })
    expect(res.status()).toBe(401)
  })

  test('re-linking same method is idempotent', async ({ page }) => {
    const session = await getSessionCookie(page)
    const testEmail = `idem-${Date.now()}@example.com`

    // Link email
    const res1 = await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: testEmail },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })
    expect(res1.status()).toBe(200)

    // Link again — should still succeed (idempotent)
    const res2 = await page.request.post('/api/auth/link', {
      data: { provider: 'email', email: testEmail },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })
    expect(res2.status()).toBe(200)
  })
})
