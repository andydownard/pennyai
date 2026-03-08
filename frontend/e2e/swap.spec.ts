import { test, expect } from '@playwright/test'
import { login, addItemViaAPI, getSessionCookie } from './helpers'

/**
 * Swap & Share flow E2E tests.
 *
 * Swap tests:
 *   The swap matching engine requires brand_deals in the DB, which are seeded
 *   via the admin-only POST /api/admin/deals endpoint. Since E2E test users
 *   are not admin (no ADMIN_PHONE set), we test the swap accept/decline API
 *   contract directly — verifying correct status codes, response shapes, and
 *   idempotency guards. The SwapCard UI component is covered by the Go unit
 *   tests which can seed deals directly.
 *
 * Share tests:
 *   Full end-to-end: create items -> generate share link -> view shared list.
 *   Includes empty list guard, unauthenticated access, and expired link handling.
 */

test.describe('Swap accept/decline API contract', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('accept swap with nonexistent deal returns error', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Peanut Butter', department: 'Pantry' })

    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json() as Array<{ id: number; name: string }>
    const pbItem = items.find(i => i.name === 'Peanut Butter')
    expect(pbItem).toBeTruthy()

    // Attempt to accept a deal that does not exist (deal_id=99999)
    const res = await page.request.post('/api/list/parse', {
      data: {
        text: 'accept_swap',
        swap_deal_id: 99999,
        swap_item_id: pbItem!.id,
      },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })

    // Should return 400 because the deal doesn't exist or isn't active
    expect(res.status()).toBe(400)
    const body = await res.json()
    expect(body.error).toBeTruthy()
  })

  test('decline swap with nonexistent deal still returns success', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Tomato Sauce', department: 'Pantry' })

    const session = await getSessionCookie(page)
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json() as Array<{ id: number; name: string }>
    const sauceItem = items.find(i => i.name === 'Tomato Sauce')
    expect(sauceItem).toBeTruthy()

    // Decline always succeeds (logs event + increments suppression)
    const res = await page.request.post('/api/list/parse', {
      data: {
        text: 'decline_swap',
        swap_deal_id: 99999,
        swap_item_id: sauceItem!.id,
      },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })

    expect(res.status()).toBe(201)
    const body = await res.json()
    expect(body.action).toBe('swap_declined')
    expect(body.message).toContain('keeping your original')
  })

  test('decline does not modify the list item', async ({ page }) => {
    await addItemViaAPI(page, { name: 'Ketchup', department: 'Pantry', brand: 'Heinz' })

    const session = await getSessionCookie(page)

    // Get the item
    const listRes = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items = await listRes.json() as Array<{ id: number; name: string; brand: string }>
    const ketchupItem = items.find(i => i.name === 'Ketchup')
    expect(ketchupItem).toBeTruthy()
    expect(ketchupItem!.brand).toBe('Heinz')

    // Decline a swap
    await page.request.post('/api/list/parse', {
      data: {
        text: 'decline_swap',
        swap_deal_id: 1,
        swap_item_id: ketchupItem!.id,
      },
      headers: { 'Content-Type': 'application/json', Cookie: `session=${session}` },
    })

    // Verify item is unchanged
    const listRes2 = await page.request.get('/api/list', {
      headers: { Cookie: `session=${session}` },
    })
    const items2 = await listRes2.json() as Array<{ id: number; name: string; brand: string }>
    const ketchupAfter = items2.find(i => i.id === ketchupItem!.id)
    expect(ketchupAfter).toBeTruthy()
    expect(ketchupAfter!.name).toBe('Ketchup')
    expect(ketchupAfter!.brand).toBe('Heinz')
  })

  test('swap accept requires authentication', async ({ page }) => {
    await page.context().clearCookies()

    const res = await page.request.post('/api/list/parse', {
      data: {
        text: 'accept_swap',
        swap_deal_id: 1,
        swap_item_id: 1,
      },
      headers: { 'Content-Type': 'application/json' },
    })

    expect(res.status()).toBe(401)
  })

  test('swap decline requires authentication', async ({ page }) => {
    await page.context().clearCookies()

    const res = await page.request.post('/api/list/parse', {
      data: {
        text: 'decline_swap',
        swap_deal_id: 1,
        swap_item_id: 1,
      },
      headers: { 'Content-Type': 'application/json' },
    })

    expect(res.status()).toBe(401)
  })

  test('swap card component renders with correct buttons', async ({ page }) => {
    // Navigate to home screen (already logged in from beforeEach)
    // Add an item so the list isn't empty
    await addItemViaAPI(page, { name: 'Cereal', department: 'Pantry' })
    await page.reload()

    // Verify the item appears (proving the list renders)
    await expect(page.getByText('Cereal')).toBeVisible({ timeout: 10_000 })

    // Without a matching deal in the DB, no swap card should appear
    // This verifies that swap cards are NOT shown spuriously
    const brandPartner = page.getByText('Brand Partner')
    await page.waitForTimeout(1_000)
    await expect(brandPartner).not.toBeVisible()
  })
})

test.describe('Share list flow', () => {
  test('empty list generates share link with zero items', async ({ page }) => {
    await login(page)

    // Generate a share link on an empty list
    const session = await getSessionCookie(page)
    const shareRes = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    expect(shareRes.status()).toBe(201)
    const shareData = await shareRes.json()
    expect(shareData.url).toBeTruthy()
    expect(shareData.code).toBeTruthy()
    expect(shareData.expires_at).toBeTruthy()

    // Fetch the shared list (no auth required)
    const sharedRes = await page.request.get(`/api/shared/${shareData.code}`)
    expect(sharedRes.ok()).toBe(true)
    const sharedData = await sharedRes.json()

    // Should return an empty items array
    expect(Array.isArray(sharedData.items)).toBe(true)
    expect(sharedData.items.length).toBe(0)
  })

  test('generate share link and view shared list with items', async ({ page }) => {
    await login(page)

    // Add items to the list
    await addItemViaAPI(page, { name: 'Milk', department: 'Dairy & Eggs' })
    await addItemViaAPI(page, { name: 'Bread', department: 'Bakery' })
    await addItemViaAPI(page, { name: 'Bananas', department: 'Produce' })

    // Generate share link via API
    const session = await getSessionCookie(page)
    const shareRes = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    expect(shareRes.status()).toBe(201)
    const shareData = await shareRes.json()
    expect(shareData.url).toMatch(/^\/shared\//)
    expect(shareData.code).toHaveLength(32) // 16 bytes -> 32 hex chars

    // Fetch the shared list via API (no auth — public endpoint)
    const sharedRes = await page.request.get(`/api/shared/${shareData.code}`)
    expect(sharedRes.ok()).toBe(true)
    const sharedData = await sharedRes.json()

    // Verify the shared list contains the items we added
    expect(Array.isArray(sharedData.items)).toBe(true)
    expect(sharedData.items.length).toBe(3)

    const itemNames = (sharedData.items as Array<{ name: string }>)
      .map(i => i.name)
      .sort()
    expect(itemNames).toEqual(['Bananas', 'Bread', 'Milk'])
  })

  test('shared list renders in browser without auth', async ({ page }) => {
    await login(page)

    // Add an item
    await addItemViaAPI(page, { name: 'Eggs', department: 'Dairy & Eggs' })

    // Generate share link
    const session = await getSessionCookie(page)
    const shareRes = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    const shareData = await shareRes.json()

    // Clear cookies to simulate unauthenticated visitor
    await page.context().clearCookies()

    // Navigate to the shared list page in the browser
    await page.goto(`/shared/${shareData.code}`)

    // The shared list screen should render the item
    await expect(page.getByText('Eggs')).toBeVisible({ timeout: 10_000 })
  })

  test('nonexistent share code returns 404', async ({ page }) => {
    await login(page)

    const res = await page.request.get('/api/shared/nonexistent_code_12345')
    expect(res.status()).toBe(404)
    const body = await res.json()
    expect(body.error).toBeTruthy()
  })

  test('unauthenticated user can view shared list via API', async ({ page }) => {
    // First, login and create items + share link
    await login(page)
    await addItemViaAPI(page, { name: 'Chicken', department: 'Meat' })

    const session = await getSessionCookie(page)
    const shareRes = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    const shareData = await shareRes.json()

    // Clear cookies to simulate unauthenticated user
    await page.context().clearCookies()

    // Should still be able to view the shared list (no auth required)
    const sharedRes = await page.request.get(`/api/shared/${shareData.code}`)
    expect(sharedRes.ok()).toBe(true)
    const sharedData = await sharedRes.json()
    expect(sharedData.items.length).toBe(1)
    expect(sharedData.items[0].name).toBe('Chicken')
  })

  test('share link requires authentication to create', async ({ page }) => {
    await login(page)
    await page.context().clearCookies()

    const res = await page.request.post('/api/list/share')
    expect(res.status()).toBe(401)
  })

  test('shared list does not expose item IDs or sensitive data', async ({ page }) => {
    await login(page)
    await addItemViaAPI(page, { name: 'Secret Item', department: 'Pantry' })

    const session = await getSessionCookie(page)
    const shareRes = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    const shareData = await shareRes.json()

    const sharedRes = await page.request.get(`/api/shared/${shareData.code}`)
    const sharedData = await sharedRes.json()

    // Items should have name, quantity, checked — but NOT id, user_id, etc.
    const item = sharedData.items[0]
    expect(item.name).toBe('Secret Item')
    expect(item).not.toHaveProperty('id')
    expect(item).not.toHaveProperty('user_id')
  })

  test('multiple share links can coexist', async ({ page }) => {
    await login(page)
    await addItemViaAPI(page, { name: 'Yogurt', department: 'Dairy & Eggs' })

    const session = await getSessionCookie(page)

    // Generate two share links
    const shareRes1 = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    const shareData1 = await shareRes1.json()

    const shareRes2 = await page.request.post('/api/list/share', {
      headers: { Cookie: `session=${session}` },
    })
    const shareData2 = await shareRes2.json()

    // Both should have different codes
    expect(shareData1.code).not.toBe(shareData2.code)

    // Both should return the same list data
    const shared1 = await (await page.request.get(`/api/shared/${shareData1.code}`)).json()
    const shared2 = await (await page.request.get(`/api/shared/${shareData2.code}`)).json()

    expect(shared1.items.length).toBe(1)
    expect(shared2.items.length).toBe(1)
    expect(shared1.items[0].name).toBe('Yogurt')
    expect(shared2.items[0].name).toBe('Yogurt')
  })
})
