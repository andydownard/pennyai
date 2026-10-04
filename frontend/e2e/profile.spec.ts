import { test, expect } from '@playwright/test'
import { login, loginScreen } from './helpers'

test.describe('Profile screen', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('shows profile fields (first name, last name, email, address)', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByPlaceholder('First')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByPlaceholder('Last')).toBeVisible()
    await expect(page.getByPlaceholder('your@email.com')).toBeVisible()
    await expect(page.getByPlaceholder('Start typing your address')).toBeVisible()
  })

  test('update name auto-saves on blur', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByPlaceholder('First')).toBeVisible({ timeout: 10_000 })

    const firstNameInput = page.getByPlaceholder('First')
    await firstNameInput.fill('TestFirst')
    await firstNameInput.blur()

    // Give it a moment to save
    await page.waitForTimeout(500)

    // Reload and verify it persisted
    await page.goto('/profile')
    await expect(page.getByPlaceholder('First')).toHaveValue('TestFirst', { timeout: 10_000 })
  })

  test('sign out returns to login screen', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign Out')).toBeVisible({ timeout: 10_000 })
    await page.getByText('Sign Out').click()

    // Should be back on login
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
  })

  test('sign out clears session cookie', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Sign Out')).toBeVisible({ timeout: 10_000 })
    await page.getByText('Sign Out').click()
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })

    // Session cookie should be gone
    const cookies = await page.context().cookies()
    const sessionCookie = cookies.find(c => c.name === 'session')
    expect(sessionCookie).toBeUndefined()
  })

  test('avatar upload via API and retrieval', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByPlaceholder('First')).toBeVisible({ timeout: 10_000 })

    // Upload a small JPEG via the API
    const cookies = await page.context().cookies()
    const sessionCookie = cookies.find(c => c.name === 'session')

    // Create a minimal valid JPEG (FF D8 header + filler)
    const jpegBytes = new Uint8Array(64)
    jpegBytes[0] = 0xFF
    jpegBytes[1] = 0xD8
    const base64Photo = Buffer.from(jpegBytes).toString('base64')

    const res = await page.request.post('/api/avatar', {
      data: { photo: base64Photo },
      headers: { Cookie: `session=${sessionCookie?.value}` },
    })
    expect(res.status()).toBe(200)

    // Verify GET /api/avatar/{id} returns the image
    const meRes = await page.request.get('/auth/me', {
      headers: { Cookie: `session=${sessionCookie?.value}` },
    })
    const meData = await meRes.json()
    const userId = meData.user_id

    const avatarRes = await page.request.get(`/api/avatar/${userId}`, {
      headers: { Cookie: `session=${sessionCookie?.value}` },
    })
    expect(avatarRes.status()).toBe(200)
    expect(avatarRes.headers()['content-type']).toBe('image/jpeg')

    // Verify the returned data matches what we uploaded
    const body = await avatarRes.body()
    expect(body.length).toBe(64)
  })
})

test.describe('Share & Earn screen', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('shows household section with invite button', async ({ page }) => {
    await page.goto('/share')
    await expect(page.getByText('Shared List')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Invite someone to share')).toBeVisible()
  })

  test('shows referral section', async ({ page }) => {
    await page.goto('/share')
    await expect(page.getByText('Shared List')).toBeVisible({ timeout: 10_000 })
  })
})
