import { test, expect } from '@playwright/test'
import { login, uniquePhone } from './helpers'

test.describe('Phone auth flow', () => {
  test('phone screen renders with input and continue button', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByPlaceholder('(555) 123-4567')).toBeVisible()
    await expect(page.getByRole('button', { name: /Continue/ })).toBeVisible()
  })

  test('Penny avatar visible on login', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('img', { name: 'Penny' })).toBeVisible()
  })

  test('enter valid phone navigates to code screen', async ({ page }) => {
    await page.goto('/login')
    const phoneInput = page.getByPlaceholder('(555) 123-4567')
    await phoneInput.click()
    await phoneInput.pressSequentially('5551234567')

    // Ensure checkbox is checked
    const checkbox = page.locator('input[type="checkbox"]').first()
    if (!(await checkbox.isChecked())) {
      await checkbox.check()
    }

    await page.getByRole('button', { name: /Continue/ }).click()

    await expect(page.getByText('Enter the code sent to')).toBeVisible()
  })

  test('code entry lands on home screen', async ({ page }) => {
    await login(page)
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()
  })

  test('session persists on page reload', async ({ page }) => {
    await login(page)
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible()

    await page.reload()
    // Should still be on home screen, not redirected to login
    await expect(page.locator('textarea[placeholder*="Tell Penny"]')).toBeVisible({ timeout: 10_000 })
  })

  test('unauthenticated user is redirected to login', async ({ page }) => {
    await page.goto('/')
    // Should see the phone login screen
    await expect(page.getByPlaceholder('(555) 123-4567')).toBeVisible({ timeout: 10_000 })
  })

  test('phone input formats digits correctly', async ({ page }) => {
    await page.goto('/login')
    const phoneInput = page.getByPlaceholder('(555) 123-4567')
    await phoneInput.click()
    await phoneInput.pressSequentially('555')
    // Should show formatted phone
    await expect(phoneInput).toHaveValue('(555')
  })
})

test.describe('Email auth flow', () => {
  test('email login screen renders', async ({ page }) => {
    await page.goto('/login/email')
    await expect(page.getByText('Email', { exact: true })).toBeVisible()
    await expect(page.getByText('Password', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible()
  })

  test('email register screen renders', async ({ page }) => {
    await page.goto('/login/email/register')
    await expect(page.getByText('Email', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: /Create/ })).toBeVisible()
  })

  test('navigate from phone to email login', async ({ page }) => {
    await page.goto('/login')
    await page.getByText('Use email instead').click()
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible()
  })

  test('navigate from email login to register', async ({ page }) => {
    await page.goto('/login/email')
    await page.getByText('Create an account').click()
    await expect(page.getByRole('button', { name: /Create/ })).toBeVisible()
  })

  test('navigate from email login to forgot password', async ({ page }) => {
    await page.goto('/login/email')
    await page.getByText('Forgot password?').click()
    await expect(page.getByText('Reset your password')).toBeVisible()
  })

  test('navigate from email login back to phone', async ({ page }) => {
    await page.goto('/login/email')
    await page.getByText('Use phone number instead').click()
    await expect(page.getByPlaceholder('(555) 123-4567')).toBeVisible()
  })
})
