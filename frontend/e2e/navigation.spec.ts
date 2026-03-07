import { test, expect } from '@playwright/test'
import { login } from './helpers'

async function openHamburger(page: import('@playwright/test').Page) {
  // Find the hamburger by its unique SVG path content (three horizontal lines)
  await page.evaluate(() => {
    const paths = document.querySelectorAll('svg path')
    for (const p of paths) {
      const d = p.getAttribute('d')
      if (d && d.includes('M3 5h14') && d.includes('M3 10h14')) {
        const btn = p.closest('button')
        if (btn) { btn.click(); return }
      }
    }
    throw new Error('Hamburger button not found')
  })
}

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('navigate to settings from hamburger menu', async ({ page }) => {
    await expect(page.getByText('Your list is empty.')).toBeVisible()
    await openHamburger(page)
    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page.getByText('Profile')).toBeVisible()
  })

  test('navigate to share page', async ({ page }) => {
    await expect(page.getByText('Your list is empty.')).toBeVisible()
    await openHamburger(page)
    await page.getByRole('button', { name: /Share/ }).click()
    await expect(page.getByText('Share & Earn')).toBeVisible()
  })

  test('browser back button works', async ({ page }) => {
    await expect(page.getByText('Your list is empty.')).toBeVisible()
    await openHamburger(page)
    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page.getByText('Profile')).toBeVisible()

    await page.goBack()
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()
  })

  test('deep link to /settings/profile works when logged in', async ({ page }) => {
    await page.goto('/settings/profile')
    await expect(page.getByPlaceholder('First')).toBeVisible({ timeout: 10_000 })
  })
})
