import { type Page, expect } from '@playwright/test'

/**
 * Generate a unique, well-formed 10-digit US number for each test so sessions
 * never collide. Area code 212; the exchange starts 2-9, as real ones do.
 */
export function uniquePhone(): string {
  const exchangeLead = 2 + Math.floor(Math.random() * 8)
  const ts = String(Date.now() % 1000).padStart(3, '0')
  const rand = String(Math.floor(Math.random() * 1000)).padStart(3, '0')
  return `212${exchangeLead}${ts}${rand}`
}

/** The home screen's chat box — present only when signed in. */
export function homeInput(page: Page) {
  return page.locator('textarea[placeholder*="Tell Penny"]')
}

/** The signed-out login screen. */
export function loginScreen(page: Page) {
  return page.getByTestId('login-screen')
}

/**
 * Sign in without the UI.
 *
 * Real sign-in happens on the Rally primary, which a local test cannot drive.
 * With TEST_MODE=true the server exposes POST /auth/test-login, which sets the
 * session cookie on this browser context. The endpoint does not exist in
 * production and the app has no UI for it.
 *
 * Returns the phone number used.
 */
export async function login(page: Page, phone?: string): Promise<string> {
  const phoneNumber = phone ?? uniquePhone()
  const res = await page.request.post('/auth/test-login', { data: { phone_number: phoneNumber } })
  expect(res.status(), 'POST /auth/test-login (is the server running with TEST_MODE=true?)').toBe(200)

  await page.goto('/')
  await expect(homeInput(page)).toBeVisible({ timeout: 10_000 })

  return phoneNumber
}

/**
 * Navigate to a specific screen via the bottom tab bar.
 * Maps common menu item names to tab labels.
 */
export async function navigateTo(page: Page, menuItemText: string | RegExp): Promise<void> {
  // Map old menu names to bottom tab labels
  const labelMap: Record<string, string> = {
    'Shopping List': 'List',
    'Penny': 'Penny',
    'Profile': 'Profile',
  }

  const text = typeof menuItemText === 'string' ? menuItemText : ''
  // Handle Share & Earn / Share tab
  if (typeof menuItemText !== 'string' || menuItemText.includes('Share')) {
    await page.getByRole('button', { name: 'Share' }).click()
    return
  }

  const tabLabel = labelMap[text] || text
  await page.getByRole('button', { name: tabLabel }).click()
}

/**
 * Add a list item via the API (no LLM needed).
 */
export async function addItemViaAPI(page: Page, item: { name: string; department?: string; brand?: string }): Promise<void> {
  const cookies = await page.context().cookies()
  const sessionCookie = cookies.find(c => c.name === 'session')
  await page.request.post('/api/list', {
    data: {
      name: item.name,
      department: item.department || 'Other',
      brand: item.brand || '',
    },
    headers: { Cookie: `session=${sessionCookie?.value}` },
  })
}

/**
 * Get the session cookie value.
 */
export async function getSessionCookie(page: Page): Promise<string> {
  const cookies = await page.context().cookies()
  const sessionCookie = cookies.find(c => c.name === 'session')
  return sessionCookie?.value || ''
}
