const { test, expect } = require('playwright/test')
const { loginAdmin, ensureSilenceWav } = require('./helpers')
test.use({ serviceWorkers: 'block' })

test('player registra uma escuta por reprodução sem contar salto ou pausa', async ({ page }) => {
  const plays = []
  const token = `x.${Buffer.from(JSON.stringify({ uid: 'test-user', exp: Math.floor(Date.now()/1000)+3600 })).toString('base64url')}.x`
  const song = { id: 'song-1', title: 'Faixa teste', artist: 'Artista', duration: 30, format: 'wav' }
  await page.route('**/auth/login', route => route.fulfill({ json: { token, id: 'test-user', name: 'Teste', isAdmin: true } }))
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/home') return route.fulfill({ json: { sections: [{ title: 'Teste', songs: [song] }] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [song], artists: [], tastes: [] } })
    return route.fulfill({ json: [] })
  })
  await loginAdmin(page)
  await page.route('**/api/stream/**', route => route.fulfill({ path: ensureSilenceWav(30), contentType: 'audio/wav', headers: { 'Accept-Ranges': 'bytes' } }))
  await page.route('**/api/me/listens', route => { plays.push(route.request().postDataJSON()); return route.fulfill({ status: 204 }) })
  await page.locator('.card-play').first().click()
  await page.waitForFunction(() => window.__player?.audio.currentTime > 0.2)
  await page.evaluate(async () => {
    const p = await import('/player.js')
    if (!p.getPlayerState().repeat) p.toggleRepeat()
    const audio = window.__player.audio
    audio.pause()
    audio.currentTime = 10
    audio.playbackRate = 4
  })
  await page.waitForTimeout(700)
  expect(plays).toHaveLength(0)
  await page.evaluate(() => window.__player.audio.play())
  await expect.poll(() => plays.length, { timeout: 14000 }).toBeGreaterThanOrEqual(1)
  await expect.poll(() => plays.length, { timeout: 14000 }).toBeGreaterThanOrEqual(2)
  expect(plays).toHaveLength(2)
  expect(plays[0].songId).toBe(plays[1].songId)
  expect(plays[0].playId).not.toBe(plays[1].playId)
})
