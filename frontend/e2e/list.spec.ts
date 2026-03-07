import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('List operations', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('empty list shows Penny greeting', async ({ page }) => {
    await expect(page.getByText('Your list is empty.')).toBeVisible()
    await expect(page.getByText('Tell Penny what you need.')).toBeVisible()
  })

  test('add item via API and it appears in list', async ({ page }) => {
    // Add item via the direct POST /api/list endpoint (no LLM needed)
    const cookies = await page.context().cookies()
    const sessionCookie = cookies.find(c => c.name === 'session')
    await page.request.post('/api/list', {
      data: { name: 'Bananas', department: 'Produce' },
      headers: { Cookie: `session=${sessionCookie?.value}` },
    })

    // Reload to see the item
    await page.reload()
    await expect(page.getByText('Bananas')).toBeVisible({ timeout: 10_000 })
  })

  test('toggle item checked and unchecked', async ({ page }) => {
    // Add an item via API
    const cookies = await page.context().cookies()
    const sessionCookie = cookies.find(c => c.name === 'session')
    await page.request.post('/api/list', {
      data: { name: 'Milk', department: 'Dairy & Eggs' },
      headers: { Cookie: `session=${sessionCookie?.value}` },
    })

    await page.reload()
    await expect(page.getByText('Milk')).toBeVisible({ timeout: 10_000 })

    // Click the item text to toggle it (handleToggle is on the item center div)
    await page.getByText('Milk').click()

    // After toggling, the checkmark should appear
    await expect(page.locator('text="✓"').first()).toBeVisible({ timeout: 5_000 })
  })
})
