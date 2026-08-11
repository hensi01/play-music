// Player interactions on the admin home: play from a card, play/pause
// toggle, seek on the progress bar, volume slider, next/prev — all with
// the audio stream stubbed to a local silent WAV so playback is
// deterministic in headless (no dependency on the external CDN).

const { test, expect } = require('playwright/test')
const { trackConsole, expectClean, loginAdmin, ensureSilenceWav } = require('./helpers')

// Logs in as admin and stubs every /api/stream/* request with a silent WAV.
// Must be called BEFORE clicking play.
async function setupPlayer(page) {
  const clean = trackConsole(page)
  await loginAdmin(page)
  await page.route('**/api/stream/**', (route) =>
    route.fulfill({
      path: ensureSilenceWav(30),
      contentType: 'audio/wav',
      headers: { 'Accept-Ranges': 'bytes' },
    }),
  )
  return clean
}

async function playFirstCard(page) {
  await page.locator('.card-play').first().click()
  await expect(page.locator('.bottom-bar')).toBeVisible({ timeout: 10000 })
}

test.describe('Player', () => {
  test('play → player bar aparece com título', async ({ page }) => {
    const testInfo = test.info()
    const clean = await setupPlayer(page)
    await playFirstCard(page)
    await expect(page.locator('.now-playing-title')).not.toHaveText('')
    const title = (await page.locator('.now-playing-title').textContent()).trim()
    expect(title.length).toBeGreaterThan(0)
    expectClean(testInfo, clean)
  })

  test('play/pause toggle', async ({ page }) => {
    const testInfo = test.info()
    const clean = await setupPlayer(page)
    await playFirstCard(page)
    const main = page.locator('.player-btn-main')
    await expect(main).toHaveAttribute('aria-label', 'Pausar')
    await main.click()
    await expect(main).toHaveAttribute('aria-label', 'Tocar')
    await main.click()
    await expect(main).toHaveAttribute('aria-label', 'Pausar')
    expectClean(testInfo, clean)
  })

  test('seek clicando na progress bar', async ({ page }) => {
    const testInfo = test.info()
    const clean = await setupPlayer(page)
    await playFirstCard(page)
    const track = page.locator('.progress-track')
    await expect(track).toBeVisible()
    // Wait for the duration (30s WAV) to be known before clicking at 60%.
    await expect.poll(() => track.getAttribute('aria-valuemax')).not.toBe('0')
    const box = await track.boundingBox()
    await page.mouse.click(box.x + box.width * 0.6, box.y + box.height / 2)
    await expect.poll(() => page.locator('.progress-time').first().textContent(), { timeout: 10000 }).not.toBe('0:00')
    expectClean(testInfo, clean)
  })

  test('volume muda pelo slider', async ({ page }) => {
    const testInfo = test.info()
    const clean = await setupPlayer(page)
    await playFirstCard(page)
    const vol = page.locator('.volume-slider')
    // The volume controls are hidden (display:none) on the mobile layout —
    // interact via the DOM instead of requiring visibility.
    await expect(vol).toHaveCount(1)
    await vol.evaluate((el) => {
      el.value = '0.35'
      el.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await expect.poll(() => page.evaluate(() => window.__player.getState().volume)).toBe(0.35)
    // Mute → icon switches to volumeX.
    await vol.evaluate((el) => {
      el.value = '0'
      el.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await expect.poll(() => page.evaluate(() => window.__player.getState().volume)).toBe(0)
    // Icon present in the DOM with the correct data-icon on every layout.
    await expect(page.locator('.vol-icon [data-icon="volumeX"]')).toHaveCount(1)
    expectClean(testInfo, clean)
  })

  test('next/prev trocam a música', async ({ page }) => {
    const testInfo = test.info()
    const clean = await setupPlayer(page)
    await playFirstCard(page)
    // Compare by song id, not title: upload fixtures may insert two catalog
    // entries with identical titles (e.g. "pw-e2e-silence-30s" from the
    // upload spec), which would make a title-based assertion flaky.
    const currentId = () => page.evaluate(() => window.__player.getState().current?.id ?? null)
    const id1 = await currentId()
    expect(id1).toBeTruthy()

    await page.getByRole('button', { name: 'Próxima' }).click()
    await expect.poll(currentId, { timeout: 10000 }).not.toBe(id1)
    const id2 = await currentId()
    expect(id2).toBeTruthy()

    await page.getByRole('button', { name: 'Anterior' }).click()
    await expect.poll(currentId, { timeout: 10000 }).toBe(id1)
    expect(id2).not.toBe(id1)
    expectClean(testInfo, clean)
  })

  test('CDN fora do ar → fallback local (?nocdn=1) mantém playback', async ({ page }) => {
    const testInfo = test.info()
    // The aborted first attempt (dead CDN) logs a browser network error —
    // expected noise, ignored; any other console error fails the test.
    const clean = trackConsole(page, { ignore: (text) => text.includes('ERR_FAILED') })
    await loginAdmin(page)
    let nocdnRequests = 0
    await page.route('**/api/stream/**', (route) => {
      const url = route.request().url()
      if (url.includes('nocdn')) {
        // Client-side fallback: server proxies the native bytes locally.
        nocdnRequests += 1
        return route.fulfill({
          path: ensureSilenceWav(30),
          contentType: 'audio/wav',
          headers: { 'Accept-Ranges': 'bytes' },
        })
      }
      // First attempt would 302 to the Bunny CDN; simulate a dead CDN by
      // failing the request so the media element errors out.
      return route.abort('failed')
    })
    await playFirstCard(page)
    // The ladder (CDN → local) must have retried and kept playing.
    await expect(page.locator('.player-btn-main')).toHaveAttribute('aria-label', 'Pausar', { timeout: 10000 })
    expect(nocdnRequests).toBeGreaterThanOrEqual(1)
    expectClean(testInfo, clean)
  })
})
