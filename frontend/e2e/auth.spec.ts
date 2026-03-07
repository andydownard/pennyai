import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Authentication flow', () => {
  test('phone screen renders with input', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByPlaceholder('(555) 123-4567')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Send Code' })).toBeVisible()
  })

  test('enter valid phone navigates to code screen', async ({ page }) => {
    await page.goto('/login')
    const phoneInput = page.getByPlaceholder('(555) 123-4567')
    await phoneInput.click()
    await phoneInput.pressSequentially('5551234567')
    await page.locator('label:has-text("I want to text Penny AI") input[type="checkbox"]').check()
    await page.getByRole('button', { name: 'Send Code' }).click()

    await expect(page.getByText('Enter the code sent to')).toBeVisible()
  })

  test('enter code lands on home screen', async ({ page }) => {
    await login(page)
    // The chat textarea only exists on the home screen
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()
  })

  test('session persists on page reload', async ({ page }) => {
    await login(page)
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()

    await page.reload()
    // Should still be on home screen, not redirected to login
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })
  })
})
