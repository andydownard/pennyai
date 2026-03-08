import { test, expect } from '@playwright/test'
import { login, navigateTo, openMenu } from './helpers'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('hamburger menu opens and shows all nav items', async ({ page }) => {
    await openMenu(page)
    await expect(page.getByRole('button', { name: 'Shopping List' })).toBeVisible()
    await expect(page.getByRole('button', { name: /Share/ })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Penny' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Profile' })).toBeVisible()
  })

  test('navigate to Penny settings', async ({ page }) => {
    await navigateTo(page, 'Penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible()
  })

  test('navigate to Share & Earn', async ({ page }) => {
    await navigateTo(page, /Share/)
    await expect(page.getByText('Shared List')).toBeVisible()
  })

  test('navigate to Profile', async ({ page }) => {
    await navigateTo(page, 'Profile')
    await expect(page.getByPlaceholder('First')).toBeVisible()
    await expect(page.getByText('Sign Out')).toBeVisible()
  })

  test('browser back button works', async ({ page }) => {
    await navigateTo(page, 'Profile')
    await expect(page.getByPlaceholder('First')).toBeVisible()

    await page.goBack()
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()
  })

  test('deep link to /profile works when logged in', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByPlaceholder('First')).toBeVisible({ timeout: 10_000 })
  })

  test('deep link to /penny works when logged in', async ({ page }) => {
    await page.goto('/penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible({ timeout: 10_000 })
  })

  test('deep link to /share works when logged in', async ({ page }) => {
    await page.goto('/share')
    await expect(page.getByText('Shared List')).toBeVisible({ timeout: 10_000 })
  })

  test('unknown route redirects to home', async ({ page }) => {
    await page.goto('/nonexistent-page')
    // Should end up on home screen
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })
  })

  test('clicking wordmark returns to home', async ({ page }) => {
    await navigateTo(page, 'Profile')
    await expect(page.getByPlaceholder('First')).toBeVisible()

    // Click the "thriftly" wordmark
    await page.getByText('thriftly').click()
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()
  })
})
