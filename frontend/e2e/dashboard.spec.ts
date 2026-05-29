import { test, expect } from '@playwright/test'

test.describe('Dashboard smoke tests', () => {
  test('page loads and shows key sections', async ({ page }) => {
    await page.goto('/')

    await expect(page.locator('text=Viri Explorer')).toBeVisible()
    await expect(page.locator('text=Block Height')).toBeVisible()
    await expect(page.locator('text=Recent Blocks')).toBeVisible()
    await expect(page.locator('text=Recent Transactions')).toBeVisible()
  })

  test('search bar accepts input', async ({ page }) => {
    await page.goto('/')
    const searchInput = page.locator('input[placeholder*="Search" i]').first()
    await expect(searchInput).toBeVisible()
    await searchInput.fill('0xabcd')
    await expect(searchInput).toHaveValue('0xabcd')
  })

  test('navigation links are clickable', async ({ page }) => {
    await page.goto('/')
    const blocksLink = page.locator('a:has-text("Blocks")')
    if (await blocksLink.isVisible()) {
      await blocksLink.click()
      await expect(page).toHaveURL(/.*blocks.*/i)
    }
  })

  test('responsive layout - mobile viewport', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    await page.goto('/')
    await expect(page.locator('text=Viri Explorer')).toBeVisible()
  })
})
