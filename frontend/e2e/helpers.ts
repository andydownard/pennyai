import { type Page, expect } from '@playwright/test'

/** The home screen's chat box — present only when signed in. */
export function homeInput(page: Page) {
  return page.getByRole('textbox', { name: 'Add items or tell Penny anything' })
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
 * Every call mints a new user, so tests never share a list.
 */
export async function login(page: Page, name?: string): Promise<void> {
  const res = await page.request.post('/auth/test-login', { data: name ? { name } : {} })
  expect(res.status(), 'POST /auth/test-login (is the server running with TEST_MODE=true?)').toBe(200)

  // 'domcontentloaded', not 'load': index.html pulls gtag from googletagmanager.com, and a slow
  // or blocked third-party request would otherwise stall 'load' for the whole test timeout.
  await page.goto('/', { waitUntil: 'domcontentloaded' })
  await expect(homeInput(page)).toBeVisible({ timeout: 10_000 })
}

/** Open the slide-in navigation menu (the avatar button in the header). */
export async function openNavMenu(page: Page) {
  await page.getByRole('button', { name: 'Open menu' }).click()
  const nav = page.getByRole('navigation', { name: 'Main navigation' })
  await expect(nav).toBeVisible()
  return nav
}

/** Sign out from the bottom of the avatar menu. */
export async function signOutViaMenu(page: Page): Promise<void> {
  const nav = await openNavMenu(page)
  await nav.getByRole('button', { name: 'Sign out' }).click()
}

/**
 * Navigate to a screen via the avatar menu. Pass the menu item's label:
 * 'Shopping List', 'My Lists', 'Penny', 'Share & Earn' or 'Profile'.
 * The menu closes itself after a selection.
 */
export async function navigateTo(page: Page, menuItemText: string | RegExp): Promise<void> {
  const nav = await openNavMenu(page)
  await nav.getByRole('button', { name: menuItemText }).click()
  await expect(nav).toBeHidden()
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
