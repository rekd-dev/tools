import { expect, type Page } from '@playwright/test'

export async function waitForViewerReady(page: Page) {
  await expect(page.getByRole('heading', { name: 'Architecture Viewer' })).toBeVisible()
  await page.waitForLoadState('networkidle')
  await expect(page.getByText('Loading…')).toHaveCount(0, { timeout: 90_000 })
}

export async function expectNoPageError(page: Page) {
  await expect(page.getByText(/^Error:/)).toHaveCount(0)
  await expect(page.locator('p.text-red-400')).toHaveCount(0)
}
