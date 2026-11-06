const { test, expect } = require('playwright/test')
test.use({ serviceWorkers: 'block' })

test('troca de conta limpa player e descarta playlists antigas', async ({ page }) => {
  await page.addInitScript(() => sessionStorage.setItem('pm_pwa_seen', '1'))
  await page.route('**/auth/login', async route => {
    const phone = route.request().postDataJSON().phone.replace(/\D/g, '')
    const user = phone.endsWith('1') ? 'A' : 'B'
    await route.fulfill({ json: { token: `token-${user}`, id: `user-${user}`, name: `Cliente ${user}`, isAdmin: false } })
  })
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname
    const token = route.request().headers()['x-nd-authorization'] || ''
    if (path === '/api/me/history' && token.endsWith('token-A')) {
      await new Promise(resolve => setTimeout(resolve, 1600))
      return route.fulfill({ status: 401, json: { error: 'Sessão antiga' } })
    }
    if (path === '/api/playlists') {
      if (token.endsWith('token-A')) await new Promise(resolve => setTimeout(resolve, 1600))
      return route.fulfill({ json: [{ id: token.endsWith('token-A') ? 'playlist-a' : 'playlist-b', name: token.endsWith('token-A') ? 'Lista privada A' : 'Lista privada B', songCount: 0 }] })
    }
    if (path === '/api/home') return route.fulfill({ json: { sections: [] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [], artists: [], tastes: [] } })
    if (path === '/api/settings') return route.fulfill({ json: { appName: 'Teste', version: '1', libraryName: 'Teste' } })
    return route.fulfill({ json: [] })
  })
  const login = async phone => {
    await page.getByPlaceholder('Telefone (99) 99999-9999').fill(phone)
    await page.locator('.login-submit').click()
    await expect(page.locator('.app-shell')).toBeVisible()
  }
  await page.goto('/')
  await login('11987654321')
  await page.evaluate(async () => { const p = await import('/player.js'); p.playSong({ id: 'a-song', title: 'Pessoal A', artist: 'A', format: 'mp3', duration: 30 }) })
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('a-song')
  await page.evaluate(() => { void import('/api.js').then(m => m.api.get('/api/me/history')).catch(() => undefined) })
  await page.evaluate(() => { window.location.hash = '#/settings' })
  await page.locator('.settings-actions').getByRole('button', { name: 'Sair' }).click()
  await login('11987654322')
  await expect.poll(() => page.evaluate(() => window.__player.getState().current)).toBeNull()
  await expect(page.locator('.sidebar-playlists')).toContainText('Lista privada B')
  await page.waitForTimeout(1900)
  await expect(page.locator('.sidebar-playlists')).not.toContainText('Lista privada A')
  expect(await page.evaluate(() => window.__player.getState().queue.length)).toBe(0)
  expect(await page.evaluate(() => localStorage.getItem('pm_token'))).toBe('token-B')
  await expect(page.locator('.app-shell')).toBeVisible()
})
