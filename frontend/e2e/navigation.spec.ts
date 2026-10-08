import { test, expect } from '@playwright/test'
import { login, loginScreen, navigateTo, openNavMenu } from './helpers'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('navigation menu lists every screen', async ({ page }) => {
    const nav = await openNavMenu(page)
    for (const label of ['Shopping List', 'My Lists', 'Penny', 'Profile']) {
      await expect(nav.getByRole('button', { name: label })).toBeVisible()
    }
    await expect(nav.getByRole('button', { name: 'Share & Earn' })).toHaveCount(0)
    await expect(nav.getByText('Groflo', { exact: true })).toHaveCount(0)
  })

  test('navigate to Penny settings', async ({ page }) => {
    await navigateTo(page, 'Penny')
    await expect(page.getByText('Learn from my choices')).toBeVisible()
  })

  test('the share screen is still reachable by its address', async ({ page }) => {
    await page.goto('/share')
    await expect(page.getByText('Shared List')).toBeVisible({ timeout: 10_000 })
  })

  test('navigate to Profile', async ({ page }) => {
    await navigateTo(page, 'Profile')
    await expect(page.getByPlaceholder('First')).toBeVisible()
  })

  test('menu opens from the avatar: Profile first, Sign out last, Sign out leaves', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await page.screenshot({ path: 'test-results/avatar-menu-header.png', clip: { x: 0, y: 0, width: 390, height: 120 } })
    const nav = await openNavMenu(page)
    const buttons = nav.getByRole('button')
    await expect(buttons.first()).toContainText('Profile')
    await expect(buttons.last()).toHaveText('Sign out')
    await page.waitForTimeout(400) // let the slide-in finish
    await page.screenshot({ path: 'test-results/avatar-menu-open.png' })
    await buttons.last().click()
    await expect(loginScreen(page)).toBeVisible({ timeout: 10_000 })
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

test.describe('Menu name row', () => {
  test('a typical full name fits beside the Profile arrow', async ({ page }) => {
    // Regression: the arrow added in #130 took 32px from the name, so with
    // Chrome's "Large" text setting (20px base; Groflo sizes text in rem) a
    // 12-letter name like this one ended in "..." in the 260px menu.
    await login(page, 'Andy Downard')
    await page.addStyleTag({ content: 'html { font-size: 20px }' })
    const nav = await openNavMenu(page)
    const name = nav.getByRole('button', { name: 'Profile, Andy Downard' }).getByText('Andy Downard')
    const fit = await name.evaluate(el => ({ needs: el.scrollWidth, has: el.clientWidth }))
    expect(fit.needs, `name needs ${fit.needs}px but has ${fit.has}px`).toBeLessThanOrEqual(fit.has)
  })
})
