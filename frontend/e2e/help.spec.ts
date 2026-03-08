import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Help & FAQ screen', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('navigates to help from profile', async ({ page }) => {
    await page.goto('/profile')
    await expect(page.getByText('Need Help?')).toBeVisible({ timeout: 10_000 })
    await page.getByText('Need Help?').click()
    await expect(page.getByText('Frequently Asked Questions')).toBeVisible({ timeout: 5_000 })
  })

  test('FAQ accordion expands and collapses', async ({ page }) => {
    await page.goto('/profile')
    await page.getByText('Need Help?').click()
    await expect(page.getByText('Frequently Asked Questions')).toBeVisible({ timeout: 5_000 })

    // Click a FAQ question
    await page.getByText('How do I share my list with someone?').click()

    // Answer should be visible
    await expect(page.getByText('Invite to Household')).toBeVisible()

    // Click again to collapse
    await page.getByText('How do I share my list with someone?').click()

    // Wait for collapse animation
    await page.waitForTimeout(400)
  })

  test('bug report form renders', async ({ page }) => {
    await page.goto('/profile')
    await page.getByText('Need Help?').click()
    await expect(page.getByText('Report a Bug')).toBeVisible({ timeout: 5_000 })
    await expect(page.getByPlaceholder('Describe the issue you encountered...')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Submit Report' })).toBeVisible()
  })

  test('bug report submit button disabled when empty', async ({ page }) => {
    await page.goto('/profile')
    await page.getByText('Need Help?').click()
    await expect(page.getByText('Report a Bug')).toBeVisible({ timeout: 5_000 })

    const submitBtn = page.getByRole('button', { name: 'Submit Report' })
    await expect(submitBtn).toBeDisabled()
  })
})
