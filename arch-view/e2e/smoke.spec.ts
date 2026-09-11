import { expect, test } from '@playwright/test'
import { expectNoPageError, waitForViewerReady } from './helpers'

test.describe('Architecture Viewer smoke', () => {
  test('home architecture graph shows app boxes', async ({ page }) => {
    await page.goto('/')
    await waitForViewerReady(page)

    await expect(page.getByRole('button', { name: 'Architecture' })).toBeVisible()
    await expect(page.getByRole('status')).toContainText(/apps that share a name sit together/i)

    const appChip = page
      .getByRole('button', { name: 'schedule-web', exact: true })
      .or(page.getByRole('button', { name: 'auth', exact: true }))
    await expect(appChip.first()).toBeVisible({ timeout: 90_000 })

    await expect(page.locator('.react-flow')).toBeVisible()
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

  test('search clock-in and flow lens shows clock-in graph', async ({ page }) => {
    await page.goto('/')
    await waitForViewerReady(page)

    const search = page.getByRole('textbox')
    await search.fill('clock-in')
    await page.waitForLoadState('networkidle')

    const hit = page
      .getByRole('button')
      .filter({ hasText: /clock-in/i })
      .first()
    await expect(hit).toBeVisible({ timeout: 30_000 })
    await hit.click()

    await page.getByRole('button', { name: 'Flow' }).click()
    await waitForViewerReady(page)

    const dataCheckbox = page.getByRole('checkbox', { name: 'Data' })
    if (!(await dataCheckbox.isChecked())) {
      await dataCheckbox.check()
    }
    await waitForViewerReady(page)

    const graphNode = page
      .locator('.react-flow__node')
      .filter({ hasText: /ClockInUseCase|POST \/clock-in/i })
      .first()
    await expect(graphNode).toBeVisible({ timeout: 90_000 })

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

    const commitChip = page.locator('.react-flow__node').filter({ hasText: /\d+ commits/i }).first()
    const inspectorCommits = page.getByText(/\d+ commits in 90 days/i).first()

    if ((await commitChip.count()) > 0) {
      await expect(commitChip).toBeVisible()
    } else if ((await inspectorCommits.count()) > 0) {
      await expect(inspectorCommits).toBeVisible()
    }
  })
})
