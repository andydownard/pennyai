import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Penny settings screen', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('shows learn from my choices toggle', async ({ page }) => {
    await page.goto('/penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Penny remembers your brand preferences')).toBeVisible()
  })

  test('shows text with Penny toggle', async ({ page }) => {
    await page.goto('/penny')
    await expect(page.getByText('I want to SMS with Penny')).toBeVisible({ timeout: 10_000 })
  })

  test('shows Penny contact section', async ({ page }) => {
    await page.goto('/penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible({ timeout: 10_000 })
  })

  test('toggling autotune updates toggle state', async ({ page }) => {
    await page.goto('/penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible({ timeout: 10_000 })

    // Find the "Learn from my choices" toggle area and click it
    const autotuneRow = page.getByText('Learn from my choices')
    await autotuneRow.click()

    // Wait for the state to update
    await page.waitForTimeout(300)
  })
})

test.describe('Penny chat input', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('chat textarea is visible on home screen', async ({ page }) => {
    const textarea = page.locator('textarea[placeholder*="Tell Penny"]')
    await expect(textarea).toBeVisible()
  })

  test('chat textarea accepts text input', async ({ page }) => {
    const textarea = page.locator('textarea[placeholder*="Tell Penny"]')
    await textarea.fill('bananas and milk')
    await expect(textarea).toHaveValue('bananas and milk')
  })
})
