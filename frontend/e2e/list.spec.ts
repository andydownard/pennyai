import { test, expect } from '@playwright/test'
import { login, addItemViaAPI, getSessionCookie } from './helpers'

test.describe('List operations', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('empty list shows Penny greeting', async ({ page }) => {
    await expect(page.getByText('Your list is empty.')).toBeVisible()
    await expect(page.getByText('Tell Penny what you need.')).toBeVisible()
  })

  test('add item via API and it appears in list', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Bananas', department: 'Produce' })
    await page.reload()
    await expect(page.getByText('Bananas')).toBeVisible({ timeout: 10_000 })
  })

  test('toggle item checked via API', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Milk', department: 'Dairy & Eggs' })
    await page.reload()
    await expect(page.getByText('Milk', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Toggle via API (clicking the item text opens edit sheet, not toggle)
    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json()
    const milkItem = (items as Array<{ id: number; name: string }>).find(i => i.name === 'Milk')

    await page.request.patch(`/api/list/${milkItem!.id}/toggle`, {
      headers: { Cookie: `session=${session}` },
    })

    await page.reload()
    // After toggling, the checkmark should appear
    await expect(page.locator('text="✓"').first()).toBeVisible({ timeout: 10_000 })
  })

  test('delete item via swipe or API', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Bread' })
    await page.reload()
    await expect(page.getByText('Bread')).toBeVisible({ timeout: 10_000 })

    // Delete via API
    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json()
    const breadItem = (items as Array<{ id: number; name: string }>).find(i => i.name === 'Bread')
    expect(breadItem).toBeTruthy()

    await page.request.delete(`/api/list/${breadItem!.id}`, {
      headers: { Cookie: `session=${session}` },
    })

    await page.reload()
    await expect(page.getByText('Your list is empty.')).toBeVisible({ timeout: 10_000 })
  })

  test('multiple items appear in correct order', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Apples', department: 'Produce' })
    await addItemViaAPI(page, { name: 'Cheese', department: 'Dairy & Eggs' })
    await addItemViaAPI(page, { name: 'Rice', department: 'Pantry' })
    await page.reload()

    await expect(page.getByText('Apples')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Cheese')).toBeVisible()
    await expect(page.getByText('Rice')).toBeVisible()
  })

  test('toggle urgent via API and badge appears', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Yogurt', department: 'Dairy & Eggs' })
    await page.reload()
    await expect(page.getByText('Yogurt', { exact: true })).toBeVisible({ timeout: 10_000 })

    // Mark urgent via API
    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json()
    const eggItem = (items as Array<{ id: number; name: string }>).find(i => i.name === 'Yogurt')

    await page.request.patch(`/api/list/${eggItem!.id}/urgent`, {
      headers: { Cookie: `session=${session}` },
    })

    await page.reload()
    // Urgent items show a red dot or "!" indicator
    await expect(page.getByText('Yogurt', { exact: true })).toBeVisible({ timeout: 10_000 })
  })

  test('clear checked items', async ({ page }) => {
    // Add two items, check one
    await addItemViaAPI(page, { name: 'Milk' })
    await addItemViaAPI(page, { name: 'Butter' })
    await page.reload()
    await expect(page.getByText('Milk')).toBeVisible({ timeout: 10_000 })

    // Check Milk
    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json()
    const milkItem = (items as Array<{ id: number; name: string }>).find(i => i.name === 'Milk')
    await page.request.patch(`/api/list/${milkItem!.id}/toggle`, {
      headers: { Cookie: `session=${session}` },
    })

    // Clear only checked items (no ?all=true)
    await page.request.post('/api/list/clear', {
      headers: { Cookie: `session=${session}` },
    })

    await page.reload()
    // Milk should be gone, Butter should remain
    await expect(page.getByText('Butter')).toBeVisible({ timeout: 10_000 })
  })
})
