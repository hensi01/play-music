const { test, expect } = require('playwright/test')
test.use({ serviceWorkers: 'block' })

test('navegação responsiva mantém acesso ao catálogo e ao player', async ({ page }, testInfo) => {
  await page.addInitScript(() => sessionStorage.setItem('pm_pwa_seen', '1'))
  const song = { id: 'song-1', title: 'Música teste', artist: 'Banda', duration: 90 }
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/register') return route.fulfill({ json: { token: 'test-token', id: 'u1', name: 'Cliente', isAdmin: false } })
    if (path === '/api/me') return route.fulfill({ json: { id: 'u1', name: 'Cliente', isAdmin: false } })
    if (path === '/api/home') return route.fulfill({ json: { sections: [{ title: 'Músicas', songs: [song] }] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [song], artists: [], tastes: [] } })
    return route.fulfill({ json: [] })
  })
  await page.goto('/')
  await page.locator('.login-toggle').getByRole('button', { name: 'Criar conta' }).click()
  await page.getByPlaceholder('Telefone (99) 99999-9999').fill('11987654321')
  await page.locator('.login-submit').click()
  await expect(page.getByText('Música teste').first()).toBeVisible()
  if (testInfo.project.name === 'mobile') {
    await expect(page.getByRole('navigation', { name: 'Navegação principal' })).toBeVisible()
    await page.getByRole('navigation', { name: 'Navegação principal' }).getByRole('button', { name: 'Para você' }).click()
    await expect(page.locator('.page-title')).toHaveText('Para você')
    await page.getByText('Música teste').first().click()
    await expect(page.locator('.bottom-bar')).toBeVisible()
    for (const width of [812, 640]) {
      await page.setViewportSize({ width, height: 360 })
      await expect(page.getByRole('navigation', { name: 'Navegação principal' })).toBeVisible()
      const layout = await page.evaluate(() => {
        const bar = document.querySelector('.bottom-bar').getBoundingClientRect()
        const controls = document.querySelector('.player-controls').getBoundingClientRect()
        const nav = document.querySelector('.mobile-nav').getBoundingClientRect()
        return { bottom: bar.bottom, controlsBottom: controls.bottom, navTop: nav.top }
      })
      expect(layout.controlsBottom).toBeLessThanOrEqual(layout.navTop)
      expect(layout.bottom).toBeLessThanOrEqual(layout.navTop + 1)
    }
  } else {
    await expect(page.locator('.sidebar')).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Navegação principal' })).toBeHidden()
  }
  const metrics = await page.evaluate(() => ({ width: document.documentElement.scrollWidth, viewport: innerWidth, main: document.querySelector('.main-area').getBoundingClientRect().height }))
  expect(metrics.width).toBeLessThanOrEqual(metrics.viewport)
  expect(metrics.main).toBeGreaterThan(150)
})
