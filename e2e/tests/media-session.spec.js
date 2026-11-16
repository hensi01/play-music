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
  { id: 'media-song-1', title: 'Faixa de teste 1', artist: 'Artista de teste', album: 'Álbum de teste', duration: 30, format: 'wav', liked: false },
  { id: 'media-song-2', title: 'Faixa de teste 2', artist: 'Artista de teste', album: 'Álbum de teste', duration: 30, format: 'wav', liked: false },
]

test('player móvel e comandos Media Session mantêm faixa e posição sincronizadas', async ({ page }) => {
  await mockActivatedPush(page, 'media-admin')
  const consoleErrors = []
  page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(message.text()) })

  await page.addInitScript(() => {
    const state = { handlers: {}, metadata: null, position: null, playbackState: 'none' }
    class TestMediaMetadata {
      constructor(values) { Object.assign(this, values) }
    }
    Object.defineProperty(globalThis, 'MediaMetadata', { configurable: true, value: TestMediaMetadata })
    const session = {
      setActionHandler(action, handler) {
        if (action === 'seekbackward') throw new Error('unsupported in this browser')
        if (handler == null) delete state.handlers[action]
        else state.handlers[action] = handler
      },
      setPositionState(position) { state.position = { ...position } },
      get metadata() { return state.metadata },
      set metadata(value) { state.metadata = value },
      get playbackState() { return state.playbackState },
      set playbackState(value) { state.playbackState = value },
    }
    Object.defineProperty(navigator, 'mediaSession', { configurable: true, value: session })
    window.__testMediaSession = state
  })

  const token = tokenFor('media-admin')
  await page.route('**/auth/admin/login', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ token, id: 'media-admin', username: 'teste', name: 'Admin de teste', isAdmin: true }),
  }))
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/push/status') return route.fulfill({ json: { active: true } })
    if (path.startsWith('/api/stream/')) {
      return route.fulfill({ status: 200, contentType: 'audio/wav', headers: { 'Accept-Ranges': 'bytes' }, body: silentWav() })
    }
    if (path.startsWith('/api/artwork/')) {
      const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=', 'base64')
      return route.fulfill({ status: 200, contentType: 'image/png', body: png })
    }
    if (path === '/api/me') {
      return route.fulfill({ json: { id: 'media-admin', username: 'teste', name: 'Admin de teste', isAdmin: true } })
    }
    if (path === '/api/home') {
      return route.fulfill({ json: { sections: [{ title: 'Músicas de teste', songs }] } })
    }
    if (path === '/api/recommendations') {
      return route.fulfill({ json: { songs: [], artists: [], tastes: [] } })
    }
    return route.fulfill({ json: [] })
  })

  await page.setViewportSize({ width: 375, height: 812 })
  await page.goto('/')
  const installOverlay = page.locator('.pwa-overlay')
  await installOverlay.waitFor({ state: 'visible', timeout: 7000 })
  await installOverlay.getByRole('button', { name: 'Agora não' }).click()
  await page.getByRole('link', { name: 'Acesso administrativo' }).click()
  await page.getByPlaceholder('Usuário ou e-mail').fill('teste')
  await page.getByPlaceholder('Senha').fill('teste')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15000 })
  await page.locator('.card-play').first().click()
  await expect(page.locator('.bottom-bar')).toBeVisible()
  await expect(page.locator('.now-playing-open')).toHaveAttribute('aria-label', 'Abrir player: Faixa de teste 1')
  await expect.poll(() => page.evaluate(() => window.__player.getState().playing)).toBe(true)

  const dialog = page.getByRole('dialog', { name: 'Player de música' })
  await page.locator('.now-playing-open').click()
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Fechar' })).toBeFocused()
  const artwork = await dialog.locator('.fullscreen-art').boundingBox()
  expect(artwork.width).toBeLessThanOrEqual(375)
  await page.keyboard.press('Shift+Tab')
  await expect(dialog.getByRole('button', { name: 'Repetir' })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Fechar' })).toBeFocused()
  await dialog.getByRole('button', { name: 'Pausar' }).click()
  await expect(dialog.getByRole('button', { name: 'Tocar' })).toBeFocused()
  await dialog.getByRole('button', { name: 'Tocar' }).click()
  await expect(dialog.getByRole('button', { name: 'Pausar' })).toBeFocused()
  await dialog.getByRole('button', { name: 'Fechar' }).click()
  await expect(page.locator('.now-playing-open')).toBeFocused()
  await page.locator('.now-playing-open').click()
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Fechar' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  await expect(page.locator('.now-playing-open')).toBeFocused()

  await expect.poll(() => page.evaluate(() => window.__testMediaSession.metadata?.title)).toBe('Faixa de teste 1')
  const actions = await page.evaluate(() => Object.keys(window.__testMediaSession.handlers))
  expect(actions).toEqual(expect.arrayContaining(['play', 'pause', 'previoustrack', 'nexttrack', 'seekforward', 'seekto']))
  expect(actions).not.toContain('seekbackward')

  await page.evaluate(() => window.__testMediaSession.handlers.pause())
  await expect.poll(() => page.evaluate(() => window.__player.getState().playing)).toBe(false)
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.playbackState)).toBe('paused')
  await page.evaluate(() => window.__testMediaSession.handlers.play())
  await expect.poll(() => page.evaluate(() => window.__player.getState().playing)).toBe(true)
  await page.waitForFunction(() => window.__player.audio.readyState >= 1)

  await page.evaluate(() => window.__testMediaSession.handlers.seekto({ seekTime: 8 }))
  await expect.poll(() => page.evaluate(() => window.__player.getState().progress)).toBeGreaterThan(7)
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.position?.position ?? 0)).toBeGreaterThan(7)
  expect(await page.evaluate(() => window.__testMediaSession.position.duration)).toBeGreaterThanOrEqual(30)

  await page.evaluate(() => window.__testMediaSession.handlers.nexttrack())
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('media-song-2')
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.metadata?.title)).toBe('Faixa de teste 2')
  await page.evaluate(() => window.__testMediaSession.handlers.seekto({ seekTime: 1 }))
  await page.evaluate(() => window.__testMediaSession.handlers.previoustrack())
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('media-song-1')
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.metadata?.title)).toBe('Faixa de teste 1')

  await page.evaluate(() => window.__testMediaSession.handlers.nexttrack())
  await expect.poll(() => page.evaluate(() => window.__player.getState().current?.id)).toBe('media-song-2')
  await page.evaluate(() => window.__testMediaSession.handlers.seekto({ seekTime: 8 }))
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.position?.position ?? 0)).toBeGreaterThan(7)
  await page.evaluate(() => window.__testMediaSession.handlers.nexttrack())
  await expect.poll(() => page.evaluate(() => window.__player.getState().playing)).toBe(false)
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.playbackState)).toBe('paused')
  await expect.poll(() => page.evaluate(() => window.__testMediaSession.position?.position)).toBe(0)

  expect(consoleErrors).toEqual([])
})
