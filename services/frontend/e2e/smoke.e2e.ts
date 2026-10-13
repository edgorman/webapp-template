import { expect, test } from '@playwright/test'

const feedRows = (page: import('@playwright/test').Page) => page.locator('a.feed-row')

test('the feed loads', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Recent notes' })).toBeVisible()
  // A fresh deployment may hold no notes yet; either a row or the empty state proves the backend
  // answered through CORS and config.json.
  await expect(feedRows(page).first().or(page.getByText(/No notes yet/))).toBeVisible()
})

test('opening a note renders it', async ({ page }) => {
  await page.goto('/')
  const first = feedRows(page).first()
  test.skip(!(await first.isVisible()), 'no public notes on this deployment')
  const title = (await first.locator('.feed-title').textContent()) ?? ''
  await first.click()
  await expect(page).toHaveURL(/\/note\/[^/]+$/)
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
})

test('the sign-in button renders', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Account' }).click()
  const panel = page.getByRole('dialog', { name: 'Account' })
  await expect(panel.getByRole('alert')).toHaveCount(0)
  // Google Identity Services draws the button into an iframe of its own.
  await expect(panel.locator('iframe[src*="accounts.google.com"]')).toBeVisible()
})

test('/robots.txt and /sitemap.xml answer', async ({ page, request }) => {
  expect((await request.get('/robots.txt')).ok()).toBe(true)

  const response = await request.get('/sitemap.xml')
  expect(response.ok()).toBe(true)
  const xml = await response.text()
  const root = await page.evaluate((text) => {
    const doc = new DOMParser().parseFromString(text, 'application/xml')
    return doc.querySelector('parsererror') ? null : doc.documentElement.localName
  }, xml)
  expect(root).toBe('urlset')
  expect(xml).toContain('<loc>')
})
