import { test, expect } from '@playwright/test'
import { login, navigateTo, openNavMenu } from './helpers'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('navigation menu lists every screen', async ({ page }) => {
    const nav = await openNavMenu(page)
    for (const label of ['Shopping List', 'My Lists', 'Penny', 'Share & Earn', 'Profile']) {
      await expect(nav.getByRole('button', { name: label })).toBeVisible()
    }
  })

  test('navigate to Penny settings', async ({ page }) => {
    await navigateTo(page, 'Penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible()
  })

  test('navigate to Share & Earn', async ({ page }) => {
    await navigateTo(page, 'Share & Earn')
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
    await expect(page.getByRole('textbox', { name: 'Add items or tell Penny anything' })).toBeVisible()
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
    await expect(page.getByRole('textbox', { name: 'Add items or tell Penny anything' })).toBeVisible({ timeout: 10_000 })
  })

  test('Shopping List menu item returns to home', async ({ page }) => {
    await navigateTo(page, 'Profile')
    await expect(page.getByPlaceholder('First')).toBeVisible()

    await navigateTo(page, 'Shopping List')
    await expect(page.getByRole('textbox', { name: 'Add items or tell Penny anything' })).toBeVisible()
  })

  test('active menu item is highlighted', async ({ page }) => {
    let nav = await openNavMenu(page)
    await expect(nav.getByRole('button', { name: 'Shopping List' })).toHaveAttribute('aria-current', 'page')
    await nav.getByRole('button', { name: 'Shopping List' }).click() // selecting an item closes the menu
    await expect(nav).toBeHidden()

    await navigateTo(page, 'Profile')
    nav = await openNavMenu(page)
    await expect(nav.getByRole('button', { name: 'Profile' })).toHaveAttribute('aria-current', 'page')
    await expect(nav.getByRole('button', { name: 'Shopping List' })).not.toHaveAttribute('aria-current', 'page')
  })
})
