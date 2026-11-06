const { test, expect } = require('playwright/test')
test.use({ serviceWorkers: 'block' })

test('cadastro gratuito e recomendações sem loja', async ({ page }) => {
  await page.addInitScript(() => sessionStorage.setItem('pm_pwa_seen', '1'))
  let registerPhone = ''
  const song = { id: 'song-1', title: 'Música recomendada', artist: 'Banda teste', artistId: 'artist-1', genre: 'Rock', format: 'mp3', duration: 90 }
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/register') { registerPhone = route.request().postDataJSON().phone; return route.fulfill({ json: { token: 'test-token', id: 'u1', name: 'Cliente', isAdmin: false } }) }
    if (path === '/api/me') return route.fulfill({ json: { id: 'u1', name: 'Cliente', isAdmin: false } })
    if (path === '/api/home') return route.fulfill({ json: { sections: [] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [song], artists: [{ songId: 'song-1', name: 'Banda teste' }], tastes: ['Rock'], personalized: true } })
    if (path === '/api/artists/artist-1') return route.fulfill({ json: { id: 'artist-1', name: 'Banda teste', topSongs: [song], albums: [{}] } })
    if (path === '/api/songs') return route.fulfill({ json: [song] })
    return route.fulfill({ json: [] })
  })
  await page.goto('/')
  await page.locator('.login-toggle').getByRole('button', { name: 'Criar conta' }).click()
  await page.getByPlaceholder('Telefone (99) 99999-9999').fill('11987654321')
  await page.locator('.login-submit').click()
  await expect(page.locator('.app-shell')).toBeVisible()
  expect(registerPhone.replace(/\D/g, '')).toBe('11987654321')
  await expect(page.locator('.sidebar-nav')).not.toContainText('Loja')
  await page.evaluate(() => { window.location.hash = '#/recommendations' })
  await expect(page.getByText('Seus gostos: Rock')).toBeVisible()
  await expect(page.getByText('Música recomendada')).toBeVisible()
  await expect(page.getByText('Banda teste', { exact: true }).first()).toBeVisible()
  await page.getByText('Banda teste', { exact: true }).last().click()
  await expect(page.locator('.page-title')).toHaveText('Banda teste')
  const dimensions = await page.evaluate(() => ({ screen: window.innerWidth, body: document.documentElement.scrollWidth }))
  expect(dimensions.body).toBeLessThanOrEqual(dimensions.screen)
})
