const { test, expect } = require('playwright/test')
const { markPwaSeen } = require('./client-keys')
const { registerCliente, expectNoOverflow } = require('./helpers')
test.use({ serviceWorkers: 'block' })

test('cadastro gratuito e recomendações sem loja', async ({ page }) => {
  await markPwaSeen(page)
  let registerPhone = ''
  const song = { id: 'song-1', title: 'Música recomendada', artist: 'Banda teste', artistId: 'artist-1', genre: 'Rock', format: 'mp3', duration: 90 }
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/register') { registerPhone = route.request().postDataJSON().phone; return route.fulfill({ json: { token: 'test-token', id: 'u1', name: 'Cliente', isAdmin: false } }) }
    if (path === '/api/home') return route.fulfill({ json: { sections: [] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [song], artists: [{ songId: 'song-1', name: 'Banda teste' }], tastes: ['Rock'], personalized: true } })
    if (path === '/api/songs') return route.fulfill({ json: [song] })
    return route.fulfill({ json: [] })
  })
  await registerCliente(page)
  expect(registerPhone.replace(/\D/g, '')).toBe('11987654321')
  await expect(page.locator('.sidebar-nav')).not.toContainText('Loja')
  await page.evaluate(() => { window.location.hash = '#/recommendations' })
  await expect(page.getByText('Seus gostos: Rock')).toBeVisible()
  await expect(page.getByText('Música recomendada')).toBeVisible()
  await expect(page.getByText('Banda teste', { exact: true }).first()).toBeVisible()
  await page.getByText('Banda teste', { exact: true }).last().click()
  await expect(page.locator('.page-title')).toHaveText('Banda teste')
  await expectNoOverflow(page)
})
