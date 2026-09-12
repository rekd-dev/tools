import { expect, test } from '@playwright/test'
import { expectNoPageError, waitForViewerReady } from './helpers'

test.describe('Architecture Viewer smoke', () => {
  test('home architecture graph renders the chrome and some boxes', async ({ page }) => {
    await page.goto('/')
    await waitForViewerReady(page)

    await expect(page.getByRole('button', { name: 'Architecture' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Focus' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Flow' })).toBeVisible()
    await expect(page.getByRole('status')).toContainText(/apps that share a name sit together/i)

    await expect(page.locator('.react-flow')).toBeVisible()
    await expect(page.locator('.react-flow__node').first()).toBeVisible({ timeout: 90_000 })
    await expectNoPageError(page)
  })

  test('hash restore opens matrix view', async ({ page }) => {
    await page.goto('/#lens=architecture&view=matrix')
    await waitForViewerReady(page)

    await expect(page.getByRole('button', { name: 'Matrix' })).toBeVisible()
    await expect(page.getByRole('status')).toContainText(/each row depends on the columns/i)
    await expect(page.getByRole('checkbox', { name: 'Churn' })).toHaveCount(0)
    await expect(page.locator('table')).toBeVisible({ timeout: 90_000 })

    await expectNoPageError(page)
  })

  test('flow lens without a selection tells you to pick a route', async ({ page }) => {
    await page.goto('/')
    await waitForViewerReady(page)

    await page.getByRole('button', { name: 'Flow' }).click()
    await waitForViewerReady(page)

    await expect(page.getByRole('status')).toContainText(/HTTP route|pick|search/i)
    await expectNoPageError(page)
  })

  test('architecture churn overlay does not crash', async ({ page }) => {
    await page.goto('/')
    await waitForViewerReady(page)

    const churn = page.getByRole('checkbox', { name: 'Churn' })
    await expect(churn).toBeVisible()
    await churn.check()
    await waitForViewerReady(page)

    await expect(page.locator('.react-flow')).toBeVisible()
    await expectNoPageError(page)
  })
})
