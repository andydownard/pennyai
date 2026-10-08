import { test, expect } from '@playwright/test'
import { login, loginScreen, homeInput } from './helpers'

// Rally (Clerk) is the only way in. The Rally primary cannot be driven from a
// local test, so these cover what Groflo owns: the login screen, the session,
// sign-out, and the retirement of every older sign-in endpoint.

const CREDENTIAL_INPUTS = 'input[type="tel"], input[type="email"], input[type="password"], input[type="text"]'

test.describe('Login screen', () => {
  test('signed-out visitor sees the login screen', async ({ page }) => {
    await page.goto('/')
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
    await expect(page).toHaveURL(/\/login$/)
    await expect(page.getByRole('img', { name: 'Penny' })).toBeVisible()
  })

  test('login screen collects no credentials', async ({ page }) => {
    await page.goto('/login')
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
    await expect(page.locator(CREDENTIAL_INPUTS)).toHaveCount(0)
    await expect(page.locator('#google-signin-btn')).toHaveCount(0)
    await expect(page.getByText(/Use email instead|Forgot password/)).toHaveCount(0)
  })

  test('login screen offers Rally, or says sign-in is unavailable — never blank', async ({ page }) => {
    await page.goto('/login')
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
    const rally = page.getByRole('button', { name: 'Continue with Rally' })
    const unavailable = page.getByText('Sign-in is briefly unavailable.')
    await expect(rally.or(unavailable)).toBeVisible({ timeout: 10_000 })
  })

  test('login screen shows the Rally button', async ({ page }) => {
    // The button needs a build with VITE_CLERK_PUBLISHABLE_KEY, VITE_CLERK_DOMAIN
    // and VITE_CLERK_SIGN_IN_URL, and a server reporting clerk_enabled: true.
    const config = await (await page.request.get('/api/config')).json()
    test.skip(
      !process.env.VITE_CLERK_PUBLISHABLE_KEY || !config.clerk_enabled,
      'Rally sign-in is not configured for this run',
    )
    await page.goto('/login')
    await expect(page.getByRole('button', { name: 'Continue with Rally' })).toBeVisible({ timeout: 10_000 })
  })

  test('login screen shows legal links', async ({ page }) => {
    await page.goto('/login')
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('link', { name: 'Privacy', exact: true })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Terms', exact: true }).first()).toBeVisible()
  })

  test('retired sign-in routes land on the login screen', async ({ page }) => {
    for (const path of ['/verify', '/login/email', '/login/email/register', '/login/forgot', '/reset-password?token=abc']) {
      await page.goto(path)
      await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
      await expect(page.locator(CREDENTIAL_INPUTS)).toHaveCount(0)
    }
  })

  test('deep link while signed out shows the login screen', async ({ page }) => {
    await page.goto('/profile')
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
  })
})

test.describe('Session', () => {
  test('a test-login session lands on the list', async ({ page }) => {
    await login(page)
    await expect(homeInput(page)).toBeVisible()
    await expect(loginScreen(page)).toHaveCount(0)
  })

  test('session persists on page reload', async ({ page }) => {
    await login(page)
    await page.reload()
    await expect(homeInput(page)).toBeVisible({ timeout: 10_000 })
  })

  test('/auth/me reports the signed-in user', async ({ page }) => {
    await login(page)
    const res = await page.request.get('/auth/me')
    expect(res.status()).toBe(200)
    const me = await res.json()
    expect(me.logged_in).toBe(true)
    expect(me.user_id).toBeTruthy()
    expect(me).not.toHaveProperty('phone_number')
    expect(me).not.toHaveProperty('needs_sms_consent')
  })

  test('sign out returns to the login screen and stays there', async ({ page }) => {
    await login(page)
    await page.goto('/profile')
    await expect(page.getByText('Sign Out')).toBeVisible({ timeout: 10_000 })
    await page.getByText('Sign Out').click()

    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })

    // The session is gone, and a reload does not sign the user back in.
    const cookies = await page.context().cookies()
    expect(cookies.find(c => c.name === 'session')).toBeUndefined()
    await page.reload()
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
    await expect(homeInput(page)).toHaveCount(0)
    expect((await page.request.get('/auth/me')).status()).toBe(401)
  })

  test('test-login ignores a phone number', async ({ page }) => {
    const res = await page.request.post('/auth/test-login', { data: { phone_number: '123' } })
    expect(res.status()).toBe(200)
    const me = await (await page.request.get('/auth/me')).json()
    expect(me).not.toHaveProperty('phone_number')
  })
})

test.describe('Retired sign-in endpoints', () => {
  const RETIRED: Array<{ method: 'post' | 'get'; path: string }> = [
    { method: 'post', path: '/auth/send-code' },
    { method: 'post', path: '/auth/verify-code' },
    { method: 'post', path: '/auth/google' },
    { method: 'post', path: '/auth/email/register' },
    { method: 'post', path: '/auth/email/login' },
    { method: 'post', path: '/auth/email/forgot' },
    { method: 'post', path: '/auth/email/reset' },
    { method: 'post', path: '/auth/email/magic-link' },
    { method: 'get', path: '/auth/email/magic-verify?token=abc' },
    { method: 'post', path: '/api/auth/link' },
  ]

  for (const { method, path } of RETIRED) {
    test(`${method.toUpperCase()} ${path} answers 410 Gone`, async ({ page }) => {
      const res = method === 'get'
        ? await page.request.get(path, { maxRedirects: 0 })
        : await page.request.post(path, { data: {} })
      expect(res.status()).toBe(410)
    })
  }

  test('a retired endpoint sets no session', async ({ page }) => {
    await page.request.post('/auth/verify-code', { data: { phone_number: '2125550101', code: '000000' } })
    const cookies = await page.context().cookies()
    expect(cookies.find(c => c.name === 'session')).toBeUndefined()
    expect((await page.request.get('/auth/me')).status()).toBe(401)
  })
})

test.describe('Retired texting endpoints', () => {
  for (const path of ['/api/sms', '/api/me/sms-consent', '/api/penny.vcf']) {
    for (const method of ['get', 'post'] as const) {
      test(`${method.toUpperCase()} ${path} answers 410 Gone`, async ({ page }) => {
        const res = method === 'get' ? await page.request.get(path) : await page.request.post(path, { data: {} })
        expect(res.status()).toBe(410)
      })
    }
  }
})
