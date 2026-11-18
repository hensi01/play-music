const { test, expect } = require('playwright/test')
const { mockActivatedPush, tokenFor } = require('./push-fixture.cjs')
const { markPwaSeen } = require('./client-keys')
const { registerCliente, stubStream, stubArtwork, expectNoOverflow } = require('./helpers')
test.use({ serviceWorkers: 'block' })

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
  await markPwaSeen(page)
  await stubStream(page)
  await stubArtwork(page)
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    if (path === '/api/register') {
      return route.fulfill({ json: { token: tokenFor('u1'), id: 'u1', name: 'Cliente', isAdmin: false } })
    }
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
    return route.fulfill({ json: [] })
  })

  await registerCliente(page)
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

  await expectNoOverflow(page)

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

  // The queue page renders its own rows and each row plays from that same queue.
  // Clicking a row that is not the current song must make that row current.
  const queue = await page.evaluate(() => window.__player.getState().queue.map((song) => song.id))
  const current = await page.evaluate(() => window.__player.getState().current?.id)
  expect(queue.length).toBeGreaterThan(1)
  const targetIndex = queue.findIndex((id) => id !== current)
  expect(targetIndex).toBeGreaterThan(-1)
  await page.evaluate(() => { window.location.hash = '#/queue' })
  await expect(page.locator('.page-title', { hasText: 'Fila' })).toBeVisible()
  await page.locator('.track-list .track-main').nth(targetIndex).click()
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe(queue[targetIndex])

  await page.evaluate(() => window.__player.audio.pause())
})
