// PWA: service worker registration, manifest link and native installation prompt.

const { test, expect } = require('playwright/test')
const { trackConsole, expectClean } = require('./helpers')

test.describe('PWA', () => {
  test('sw.js registra e manifest link existe', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.goto('/')
    const manifest = page.locator('link[rel="manifest"]')
    await expect(manifest).toHaveCount(1)
    await expect(manifest).toHaveAttribute('href', /manifest/)

    // Registration happens on window load; the controller may not be set on
    // the very first visit.
    await expect
      .poll(
        () =>
          page.evaluate(async () => {
            if (!('serviceWorker' in navigator)) return false
            const regs = await navigator.serviceWorker.getRegistrations()
            return regs.length > 0 && regs.some((r) => r.active || r.installing || r.waiting)
          }),
        { timeout: 15000 },
      )
      .toBe(true)
    expectClean(testInfo, clean)
  })

  test('popup mostra somente Instalar PWA e abre a confirmação nativa', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.addInitScript(() => {
      window.__nativePwaPromptCalls = 0
      window.addEventListener('load', () => setTimeout(() => {
        const event = new Event('beforeinstallprompt', { cancelable: true })
        event.prompt = () => {
          window.__nativePwaPromptCalls++
          return Promise.resolve()
        }
        event.userChoice = Promise.resolve({ outcome: 'accepted' })
        window.dispatchEvent(event)
      }, 0))
    })
    await page.goto('/')
    const overlay = page.locator('.pwa-overlay')
    await expect(overlay).toBeVisible({ timeout: 7000 })
    await expect(overlay.getByRole('button')).toHaveCount(1)
    await expect(overlay.getByRole('button')).toHaveText('Instalar PWA')
    expect(await overlay.locator('.pwa-card').evaluate((card) => card.children.length)).toBe(1)
    await overlay.getByRole('button', { name: 'Instalar PWA' }).click()
    await expect.poll(() => page.evaluate(() => window.__nativePwaPromptCalls)).toBe(1)
    await expect(overlay).toHaveCount(0)
    expectClean(testInfo, clean)
  })
})
