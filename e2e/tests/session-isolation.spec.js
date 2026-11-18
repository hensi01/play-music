const { test, expect } = require('playwright/test')
const { markPwaSeen, AUTH_HEADER, TOKEN_KEY } = require('./client-keys')
const { logout, PHONE_PLACEHOLDER } = require('./helpers')
test.use({ serviceWorkers: 'block' })

// The slow, stale responses below take this long.
const STALE_DELAY_MS = 1600

// The leak these assertions guard against is a race, so the test waits for the
// stale responses to actually be served instead of sleeping past them: two
// delayed token-A answers (history 401 + playlists) must land before the
// "nothing repopulated" checks are meaningful.
const STALE_RESPONSES = 2

test('troca de conta limpa player e descarta playlists antigas', async ({ page }) => {
  await markPwaSeen(page)
  let staleServed = 0
  let staleLanded = () => {}
  const staleSeen = new Promise((resolve) => {
    staleLanded = () => { if (++staleServed === STALE_RESPONSES) resolve() }
  })
  await page.route('**/auth/login', async route => {
    const phone = route.request().postDataJSON().phone.replace(/\D/g, '')
    const user = phone.endsWith('1') ? 'A' : 'B'
    await route.fulfill({ json: { token: `token-${user}`, id: `user-${user}`, name: `Cliente ${user}`, isAdmin: false } })
  })
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname
    const token = route.request().headers()[AUTH_HEADER.toLowerCase()] || ''
    if (path === '/api/me/history' && token.endsWith('token-A')) {
      await new Promise(resolve => setTimeout(resolve, STALE_DELAY_MS))
      await route.fulfill({ status: 401, json: { error: 'Sessão antiga' } })
      staleLanded()
      return
    }
    if (path === '/api/playlists') {
      if (token.endsWith('token-A')) await new Promise(resolve => setTimeout(resolve, STALE_DELAY_MS))
      await route.fulfill({ json: [{ id: token.endsWith('token-A') ? 'playlist-a' : 'playlist-b', name: token.endsWith('token-A') ? 'Lista privada A' : 'Lista privada B', songCount: 0 }] })
      if (token.endsWith('token-A')) staleLanded()
      return
    }
    if (path === '/api/home') return route.fulfill({ json: { sections: [] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [], artists: [], tastes: [] } })
    if (path === '/api/settings') return route.fulfill({ json: { appName: 'Teste', version: '1', libraryName: 'Teste' } })
    return route.fulfill({ json: [] })
  })
  const login = async phone => {
    await page.getByPlaceholder(PHONE_PLACEHOLDER).fill(phone)
    await page.locator('.login-submit').click()
    await expect(page.locator('.app-shell')).toBeVisible()
  }
  await page.goto('/')
  await login('11987654321')
  await page.evaluate(async () => { const p = await import('/player.js'); p.playContext([{ id: 'a-song', title: 'Pessoal A', artist: 'A', format: 'mp3', duration: 30 }], 0) })
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('a-song')
  await page.evaluate(() => { void import('/api.js').then(m => m.api.get('/api/me/history')).catch(() => undefined) })
  await logout(page)
  await login('11987654322')
  await expect.poll(() => page.evaluate(() => window.__player.getState().current)).toBeNull()
  await expect(page.locator('.sidebar-playlists')).toContainText('Lista privada B')
  // Both stale token-A responses have been served by now; neither may have
  // repopulated the new session.
  await staleSeen
  await expect(page.locator('.sidebar-playlists')).not.toContainText('Lista privada A')
  expect(await page.evaluate(() => window.__player.getState().queue.length)).toBe(0)
  expect(await page.evaluate((key) => localStorage.getItem(key), TOKEN_KEY)).toBe('token-B')
  await expect(page.locator('.app-shell')).toBeVisible()
})
