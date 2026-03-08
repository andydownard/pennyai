import { type Page, expect } from '@playwright/test'

let phoneCounter = 0

/**
 * Generate a unique 10-digit phone number for each test to avoid session conflicts.
 */
export function uniquePhone(): string {
  phoneCounter++
  return `555${String(phoneCounter).padStart(7, '0')}`
}

/**
 * Log in via the phone/code flow using the TEST_MODE bypass (code "000000").
 * Returns the phone number used.
 */
export async function login(page: Page, phone?: string): Promise<string> {
  const phoneNumber = phone ?? uniquePhone()
  await page.goto('/login')

  // Type the phone number digit-by-digit (React onChange strips non-digits)
  const phoneInput = page.getByPlaceholder('(555) 123-4567')
  await phoneInput.click()
  await phoneInput.pressSequentially(phoneNumber)

  // The checkbox for SMS opt-in may already be checked; ensure it is
  const checkbox = page.locator('input[type="checkbox"]').first()
  if (!(await checkbox.isChecked())) {
    await checkbox.check()
  }

  // Submit — button text is "Continue →"
  await page.getByRole('button', { name: /Continue/ }).click()

  // Wait for code screen
  await expect(page.getByText('Enter the code sent to')).toBeVisible({ timeout: 10_000 })

  // Type "000000" into the 6 code inputs.
  // Each input auto-advances focus via React's handleCodeChange.
  // We use pressSequentially on the first input — React onChange fires per keystroke
  // and auto-focuses the next input.
  const firstCodeInput = page.locator('input[maxlength="1"]').first()
  await firstCodeInput.waitFor({ state: 'visible' })
  await firstCodeInput.click()
  await page.waitForTimeout(200)
  await firstCodeInput.pressSequentially('000000', { delay: 80 })

  // Wait for home screen — look for the chat input that only exists on the home screen
  await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })

  return phoneNumber
}

/**
 * Log in via email/password flow using TEST_MODE (skips bcrypt & auto-creates account).
 */
export async function loginWithEmail(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login/email/register')

  // Fill registration form — FloatingLabelField inputs don't have aria labels,
  // so use the textbox role by index
  const inputs = page.locator('input[type="email"], input[type="text"], input[type="password"]')
  const emailInput = inputs.first()
  await emailInput.fill(email)

  const nameInput = page.getByPlaceholder('First')
  if (await nameInput.isVisible()) {
    await nameInput.fill('Test')
  }

  const passwordInput = page.locator('input[type="password"]').first()
  await passwordInput.fill(password)
  await page.getByRole('button', { name: /Create|Sign/ }).click()

  // Wait for home screen
  await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })
}

/**
 * Open the hamburger/avatar menu.
 * The menu button can be intercepted by the Penny chime-in overlay or loading state.
 * We wait for stability, dismiss any chime-in, then force-click.
 */
export async function openMenu(page: Page): Promise<void> {
  // Wait for any animations/loading to settle
  await page.waitForTimeout(1000)

  // Dismiss any Penny chime-in overlay by clicking it if present
  const chimeIn = page.locator('[data-testid="penny-chime"]')
  if (await chimeIn.isVisible({ timeout: 200 }).catch(() => false)) {
    await chimeIn.click()
    await page.waitForTimeout(300)
  }

  const menuBtn = page.getByLabel('Navigation menu')
  await menuBtn.waitFor({ state: 'visible', timeout: 10_000 })
  // Use dispatchEvent to bypass pointer interception issues
  await menuBtn.dispatchEvent('click')
}

/**
 * Navigate to a specific screen via the hamburger menu.
 */
export async function navigateTo(page: Page, menuItemText: string): Promise<void> {
  await openMenu(page)
  await page.getByRole('button', { name: menuItemText }).click()
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
