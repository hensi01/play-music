// Home (admin sections/cards), search and the library routes — all with
// zero console errors.

const { test, expect } = require('playwright/test')
const {
  trackConsole,
  expectClean,
  loginAdmin,
  loginCliente,
  CLIENT_PHONE,
} = require('./helpers')

test.describe('Home / Busca / Biblioteca', () => {
  test('home admin: seções e cards renderizam', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    // Admin home = "Adicionadas recentemente" (+ "Mais ouvidas") sections.
    await expect(page.locator('.section')).not.toHaveCount(0)
    const titles = await page.locator('.section-title').allTextContents()
    expect(titles.some((t) => t.includes('Adicionadas recentemente') || t.includes('Mais ouvidas'))).toBe(true)
    await expect(page.locator('.card')).not.toHaveCount(0)
    await expect(page.locator('.card-play').first()).toBeVisible()
    expectClean(testInfo, clean)
  })

  test('busca "Deus" → resultados sem crash', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await page.goto('/#/search')
    const input = page.locator('.search-input')
    await input.fill('Deus')
    // Debounce (300ms) + fetch: wait for either rows or the empty state.
    await page.waitForFunction(
      () => {
        const el = document.getElementById('search-results')
        if (!el) return false
        return el.querySelectorAll('.track-row').length > 0 || !!el.querySelector('.empty-state')
      },
      undefined,
      { timeout: 15000 },
    )
    const rows = await page.locator('#search-results .track-row').count()
    const empty = await page.locator('#search-results .empty-state').count()
    expect(rows > 0 || empty > 0).toBe(true)
    if (rows > 0) {
      await expect(page.locator('#search-results .track-row').first().locator('.track-title')).not.toHaveText('')
    }
    // Input still alive (no crash / no re-render destroying the field).
    await expect(input).toBeVisible()
    expectClean(testInfo, clean)
  })

  test('library / liked / history renderizam sem erro', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)

    await page.goto('/#/library')
    await expect(page.locator('.page-title', { hasText: 'Sua Biblioteca' })).toBeVisible()
    await expect(page.locator('.tab-btn', { hasText: 'Músicas' })).toBeVisible()
    await expect(page.locator('.track-list .track-row').first()).toBeVisible()

    await page.getByRole('button', { name: 'Categorias' }).click()
    await expect(page.locator('.card', { hasText: 'Cristão' })).toBeVisible()

    await page.getByRole('button', { name: 'Playlists' }).click()

    await page.goto('/#/liked')
    await expect(page.locator('.detail-title', { hasText: 'Curtidas' })).toBeVisible()

    await page.goto('/#/history')
    await expect(page.locator('.page-title', { hasText: 'Histórico' })).toBeVisible()

    expectClean(testInfo, clean)
  })

  test('home cliente (telefone) renderiza sem erro', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    // Client has no granted categories: home shows the empty state — must
    // still render without crashing or console errors (B4 behaviour).
    await loginCliente(page, CLIENT_PHONE)
    await expect(page.locator('.app-shell')).toBeVisible()
    await expect(page.locator('.page-title').first()).toBeVisible()
    expectClean(testInfo, clean)
  })
})
