import { test, expect } from '@playwright/test'
import { login, addItemViaAPI } from './helpers'

test.describe('Edit item short form', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await addItemViaAPI(page, { name: 'Whole milk', department: 'Dairy & Eggs' })
    await page.reload()
    await page.getByText('Whole milk', { exact: true }).click()
  })

  test('shows the short form and hides Notes until More details', async ({ page }) => {
    const sheet = page.getByRole('dialog', { name: 'Edit item' })
    await expect(sheet).toBeVisible()
    await expect(sheet.getByRole('combobox', { name: 'Department' })).toBeVisible()
    await expect(sheet.getByRole('combobox', { name: 'Priority' })).toHaveValue('normal')
    await expect(sheet.getByLabel('Notes')).toHaveCount(0)
    await sheet.getByRole('button', { name: /more details/i }).click()
    await expect(sheet.getByLabel('Notes')).toBeVisible()
    // A just-added item has one history entry, so History shows.
    await expect(sheet.getByText('History')).toBeVisible()
  })

  test('priority select makes the item urgent', async ({ page }) => {
    const sheet = page.getByRole('dialog', { name: 'Edit item' })
    await sheet.getByRole('combobox', { name: 'Priority' }).selectOption('urgent')
    await expect(sheet.getByRole('combobox', { name: 'Priority' })).toHaveValue('urgent')
    await sheet.getByRole('button', { name: 'Close edit item' }).click()
    await expect(page.getByText('Urgent').first()).toBeVisible()
  })

  test('editing the name and saving keeps the item', async ({ page }) => {
    const sheet = page.getByRole('dialog', { name: 'Edit item' })
    await sheet.getByLabel('Name').fill('Oat milk')
    await sheet.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('Oat milk', { exact: true })).toBeVisible()
  })
})

test.describe('Balance (Earnings) screen', () => {
  test('empty state has no 30% promise and no Penny face on the list', async ({ page }) => {
    await login(page)
    await page.goto('/earnings', { waitUntil: 'domcontentloaded' })
    await expect(page.getByText('Try a deal and get paid.')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Total earned')).toBeVisible()
    await expect(page.getByText('Coming', { exact: true })).toBeVisible()
    await expect(page.getByText('Paid monthly by Venmo.', { exact: false })).toBeVisible()
    await expect(page.getByText('30%')).toHaveCount(0)
    await expect(page.getByText('Pending', { exact: true })).toHaveCount(0)
  })
})
