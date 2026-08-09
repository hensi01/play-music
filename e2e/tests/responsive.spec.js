// Responsive: at 375px no route may overflow horizontally
// (documentElement.scrollWidth <= innerWidth). Runs on the mobile project.

const { test, expect } = require('playwright/test')
const { trackConsole, expectClean, loginAdmin } = require('./helpers')

async function assertNoOverflow(page) {
  const r = await page.evaluate(() => ({
    sw: document.documentElement.scrollWidth,
    iw: window.innerWidth,
    bodySw: document.body.scrollWidth,
  }))
  expect(r.sw, `scrollWidth ${r.sw} > innerWidth ${r.iw}`).toBeLessThanOrEqual(r.iw)
  expect(r.bodySw, `body scrollWidth ${r.bodySw} > innerWidth ${r.iw}`).toBeLessThanOrEqual(r.iw)
}

test.describe('Responsivo 375px', () => {
  test('sem overflow horizontal em todas as rotas', async ({ page }) => {
    const testInfo = test.info()
    test.skip(testInfo.project.name === 'desktop', 'apenas no projeto mobile')
    const clean = trackConsole(page)

    // Login screen renders at 375px.
    await page.goto('/')
    await expect(page.locator('.login-screen')).toBeVisible()
    await assertNoOverflow(page)

    await loginAdmin(page)
    await assertNoOverflow(page) // home

    await page.goto('/#/search')
    await expect(page.locator('.search-input')).toBeVisible()
    await assertNoOverflow(page)

    await page.goto('/#/library')
    await expect(page.locator('.page-title', { hasText: 'Sua Biblioteca' })).toBeVisible()
    await assertNoOverflow(page)

    await page.goto('/#/admin')
    await expect(page.locator('.page-title', { hasText: 'Administração' })).toBeVisible()
    await assertNoOverflow(page)

    await page.goto('/loja.html')
    await expect(page.locator('#catGrid .card')).toHaveCount(1)
    await assertNoOverflow(page)

    expectClean(testInfo, clean)
  })
})
