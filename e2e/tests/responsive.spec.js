// Responsive: at 375px no route may overflow horizontally
// (documentElement.scrollWidth <= innerWidth). Runs on the mobile project.

const { test, expect } = require('playwright/test')
const { trackConsole, expectClean, loginAdmin, expectNoOverflow } = require('./helpers')

test.describe('Responsivo 375px', () => {
  test('sem overflow horizontal em todas as rotas', async ({ page }) => {
    const testInfo = test.info()
    test.skip(testInfo.project.name === 'desktop', 'apenas no projeto mobile')
    const clean = trackConsole(page)

    // Login screen renders at 375px.
    await page.goto('/')
    await expect(page.locator('.login-screen')).toBeVisible()
    await expectNoOverflow(page)

    await loginAdmin(page)
    await expectNoOverflow(page) // home

    await page.goto('/#/search')
    await expect(page.locator('.search-input')).toBeVisible()
    await expectNoOverflow(page)

    await page.goto('/#/library')
    await expect(page.locator('.page-title', { hasText: 'Sua Biblioteca' })).toBeVisible()
    await expectNoOverflow(page)

    await page.goto('/#/admin')
    await expect(page.locator('.page-title', { hasText: 'Administração' })).toBeVisible()
    await expectNoOverflow(page)

    await page.goto('/#/recommendations')
    await expect(page.locator('.page-title', { hasText: 'Para você' })).toBeVisible()
    await expectNoOverflow(page)

    expectClean(testInfo, clean)
  })
})
