// PWA: service worker registration, manifest link in the HTML and the
// install overlay (appears ~1.5s after load) closed with "Agora não".

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

    // Registration happens on window load; wait for an active/installing
    // registration. The controller may not be set on the very first visit.
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

  test('overlay PWA aparece e "Agora não" fecha', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.goto('/')
    const overlay = page.locator('.pwa-overlay')
    await expect(overlay).toBeVisible({ timeout: 7000 })
    await expect(overlay.locator('h2')).toHaveText('Instale o Play Music')
    await overlay.getByRole('button', { name: 'Agora não' }).click()
    await expect(overlay).toBeHidden({ timeout: 5000 })
    // The app underneath still works after the dismiss.
    await expect(page.locator('.login-screen')).toBeVisible()
    expectClean(testInfo, clean)
  })
})
