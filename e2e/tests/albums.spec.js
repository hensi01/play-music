const { test, expect } = require('playwright/test')
const { mockActivatedPush, tokenFor } = require('./push-fixture.cjs')
test.use({ serviceWorkers: 'block' })

function silentWav(seconds = 30) {
  const sampleRate = 22050
  const dataSize = sampleRate * seconds * 2
  const buffer = Buffer.alloc(44 + dataSize)
  buffer.write('RIFF', 0)
  buffer.writeUInt32LE(36 + dataSize, 4)
  buffer.write('WAVE', 8)
  buffer.write('fmt ', 12)
  buffer.writeUInt32LE(16, 16)
  buffer.writeUInt16LE(1, 20)
  buffer.writeUInt16LE(1, 22)
  buffer.writeUInt32LE(sampleRate, 24)
  buffer.writeUInt32LE(sampleRate * 2, 28)
  buffer.writeUInt16LE(2, 32)
  buffer.writeUInt16LE(16, 34)
  buffer.write('data', 36)
  buffer.writeUInt32LE(dataSize, 40)
  return buffer
}

const songs = [
  { id: 'album-song-1', title: 'Abertura', artist: 'Banda Horizonte', album: 'Luz do Dia', duration: 30, format: 'wav', liked: false, trackNumber: 1 },
  { id: 'album-song-2', title: 'Caminho', artist: 'Banda Horizonte', album: 'Luz do Dia', duration: 30, format: 'wav', liked: false, trackNumber: 2 },
]
const album = {
  id: 'album-1',
  name: 'Luz do Dia',
  artist: 'Banda Horizonte',
  year: 2025,
  songCount: songs.length,
  duration: 60,
  songs,
}

test('cliente encontra álbuns, escolhe faixas e controla a fila', async ({ page }, testInfo) => {
  let albumSaved = false
  await mockActivatedPush(page, 'u1')
  await page.addInitScript(() => sessionStorage.setItem('pm_pwa_seen', '1'))
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    if (path === '/api/register') {
      return route.fulfill({ json: { token: tokenFor('u1'), id: 'u1', name: 'Cliente', isAdmin: false } })
    }
    if (path === '/api/me') return route.fulfill({ json: { id: 'u1', name: 'Cliente', isAdmin: false } })
    if (path === '/api/push/status') return route.fulfill({ json: { active: true } })
    if (path === '/api/home') return route.fulfill({ json: { sections: [] } })
    if (path === '/api/recommendations') return route.fulfill({ json: { songs: [], artists: [], tastes: [] } })
    if (path === '/api/albums') return route.fulfill({ json: [album] })
    if (path === '/api/albums/album-1') return route.fulfill({ json: { ...album, liked: albumSaved } })
    if (path === '/api/me/liked/albums' && route.request().method() === 'GET') {
      return route.fulfill({ json: albumSaved ? [{ ...album, liked: true }] : [] })
    }
    if (path === '/api/me/liked/albums/album-1') {
      albumSaved = route.request().method() === 'PUT'
      return route.fulfill({ status: 204 })
    }
    if (path === '/api/me/liked') return route.fulfill({ json: [] })
    if (path === '/api/search') {
      return route.fulfill({
        json: url.searchParams.get('type') === 'albums' ? { songs: [], albums: [album], artists: [], categories: [], playlists: [] } : { songs: [], albums: [], artists: [], categories: [], playlists: [] },
      })
    }
    if (path.startsWith('/api/stream/')) {
      return route.fulfill({ status: 200, contentType: 'audio/wav', headers: { 'Accept-Ranges': 'bytes' }, body: silentWav() })
    }
    if (path.startsWith('/api/artwork/')) {
      const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=', 'base64')
      return route.fulfill({ status: 200, contentType: 'image/png', body: png })
    }
    return route.fulfill({ json: [] })
  })

  await page.goto('/')
  await page.locator('.login-toggle').getByRole('button', { name: 'Criar conta' }).click()
  await page.getByPlaceholder('Telefone (99) 99999-9999').fill('11987654321')
  await page.locator('.login-submit').click()
  await expect(page.locator('.app-shell')).toBeVisible()
  await expect(page.locator('.section').filter({ hasText: 'Álbuns' })).toBeVisible()
  await expect(page.getByText('Sua biblioteca está vazia.')).toBeHidden()

  await page.evaluate(() => { window.location.hash = '#/library' })
  await page.getByRole('button', { name: 'Álbuns' }).click()
  await expect(page.locator('.card').filter({ hasText: 'Luz do Dia' })).toBeVisible()
  await page.locator('.card').filter({ hasText: 'Luz do Dia' }).click()
  await expect(page.getByRole('heading', { name: 'Luz do Dia' })).toBeVisible()
  await expect(page.getByText('Banda Horizonte • 2025 • 2 músicas • 1 min')).toBeVisible()
  await expect(page.locator('.album-track-list .track-row')).toHaveCount(2)

  const saveAlbumButton = page.getByRole('button', { name: 'Salvar álbum' })
  await expect(saveAlbumButton).toHaveAttribute('aria-pressed', 'false')
  await saveAlbumButton.click()
  await expect(page.getByRole('button', { name: 'Remover álbum dos salvos' })).toHaveAttribute('aria-pressed', 'true')
  await page.evaluate(() => { window.location.hash = '#/liked' })
  await expect(page.locator('.page').getByRole('heading', { name: 'Curtidas' })).toBeVisible()
  await expect(page.locator('.card').filter({ hasText: 'Luz do Dia' })).toBeVisible()
  await page.locator('.card').filter({ hasText: 'Luz do Dia' }).click()
  await expect(page.getByRole('button', { name: 'Remover álbum dos salvos' })).toBeVisible()
  await page.getByRole('button', { name: 'Remover álbum dos salvos' }).click()
  await expect(page.getByRole('button', { name: 'Salvar álbum' })).toHaveAttribute('aria-pressed', 'false')
  await page.evaluate(() => { window.location.hash = '#/library' })
  await page.getByRole('button', { name: 'Álbuns' }).click()
  await page.locator('.card').filter({ hasText: 'Luz do Dia' }).click()

  const dimensions = await page.evaluate(() => ({ width: document.documentElement.scrollWidth, viewport: innerWidth }))
  expect(dimensions.width).toBeLessThanOrEqual(dimensions.viewport)

  if (testInfo.project.name !== 'mobile') await page.locator('.album-track-list .track-row').nth(1).hover()
  await page.getByRole('button', { name: 'Adicionar Caminho à fila' }).click()
  await expect(page.getByRole('status')).toHaveText('“Caminho” adicionada à fila')
  expect(await page.evaluate(() => window.__player.getState().queue.map((song) => song.id))).toEqual(['album-song-2'])

  await page.getByRole('button', { name: 'Tocar álbum', exact: true }).click()
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('album-song-1')
  expect(await page.evaluate(() => window.__player.getState().queue.map((song) => song.id))).toEqual(['album-song-1', 'album-song-2'])

  await page.locator('.album-track-list .track-main').nth(1).click()
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('album-song-2')
  await page.evaluate(() => {
    window.__restoreTestRandom = Math.random
    Math.random = () => 0
  })
  await page.getByRole('button', { name: 'Tocar álbum aleatoriamente' }).click()
  await page.evaluate(() => {
    Math.random = window.__restoreTestRandom
    delete window.__restoreTestRandom
  })
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('album-song-2')

  await page.evaluate(() => { window.location.hash = '#/search' })
  await page.locator('.search-type-select').selectOption('albums')
  await page.locator('.search-input').fill('Luz do Dia')
  await expect(page.locator('.card').filter({ hasText: 'Luz do Dia' })).toBeVisible()
  await page.locator('.card').filter({ hasText: 'Luz do Dia' }).click()
  await expect(page.getByRole('heading', { name: 'Luz do Dia' })).toBeVisible()
  await page.getByRole('button', { name: 'Voltar', exact: true }).click()
  await expect(page.locator('.search-type-select')).toHaveValue('albums')
  await expect(page.locator('.search-input')).toHaveValue('Luz do Dia')
  await page.evaluate(() => window.__player.audio.pause())
})
