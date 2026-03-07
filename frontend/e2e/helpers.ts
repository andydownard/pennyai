import { type Page, expect } from '@playwright/test'

let phoneCounter = 0

/**
 * Generate a unique 10-digit phone number for each test to avoid session conflicts.
 */
function uniquePhone(): string {
  phoneCounter++
  return `555${String(phoneCounter).padStart(7, '0')}`
}

/**
 * Log in via the phone/code flow using the TEST_MODE bypass (code "000000").
 */
export async function login(page: Page, phone?: string) {
  const phoneNumber = phone ?? uniquePhone()
  await page.goto('/login')

  // Type the phone number digit-by-digit (React onChange strips non-digits)
  const phoneInput = page.getByPlaceholder('(555) 123-4567')
  await phoneInput.click()
  await phoneInput.pressSequentially(phoneNumber)

  // Check the SMS opt-in checkbox
  await page.locator('label:has-text("I want to text Penny AI") input[type="checkbox"]').check()

  // Submit
  await page.getByRole('button', { name: 'Send Code' }).click()

  // Wait for code screen
  await expect(page.getByText('Enter the code sent to')).toBeVisible()

  // Type "000000" into the code inputs by focusing the first and typing sequentially.
  // Each digit triggers handleCodeChange which advances focus to the next box.
  const firstCodeInput = page.locator('input[inputmode="numeric"][maxlength="1"]').first()
  await firstCodeInput.click()
  await page.keyboard.press('0')
  await page.keyboard.press('0')
  await page.keyboard.press('0')
  await page.keyboard.press('0')
  await page.keyboard.press('0')
  await page.keyboard.press('0')

  // Wait for home screen — look for the chat input that only exists on the home screen
  await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })
}
