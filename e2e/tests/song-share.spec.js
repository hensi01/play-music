const { test, expect } = require('playwright/test')

test.use({ serviceWorkers: 'block' })

async function prepare(page, allowed) {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: async (value) => { window.__copiedSongLink = value } } })
    window.alert = () => {}
  })
  await page.route('**/auth/login', (route) => route.fulfill({ json: { token: 'test-token', id: 'u1', name: 'Teste', isAdmin: true } }))
  await page.route('**/api/**', (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/songs/song-123') {
      return allowed
        ? route.fulfill({ json: { id: 'song-123', title: 'Faixa de teste', artist: 'Artista', format: 'mp3', duration: 30 } })
        : route.fulfill({ status: 403, json: { error: 'Sem permissão' } })
    }
    if (path === '/api/songs') return route.fulfill({ json: [{ id: 'song-123', title: 'Faixa de teste', artist: 'Artista', format: 'mp3', duration: 30 }] })
    return route.fulfill({ json: path === '/api/home' ? { sections: [] } : [] })
  })
  await page.goto('/#/song/song-123')
  const close = page.locator('.pwa-overlay [data-act="dismiss"]')
  if (await close.isVisible()) await close.click()
  await page.getByRole('button', { name: 'Administrador' }).click()
  await page.getByPlaceholder('Usuário ou e-mail').fill('teste')
  await page.getByPlaceholder('Senha').fill('teste')
  await page.getByRole('button', { name: 'Entrar' }).click()
}

test('o link fixo continua na música após login e pode ser copiado', async ({ page }) => {
  await prepare(page, true)
  await expect(page.locator('.detail-title')).toHaveText('Faixa de teste')
  await expect(page).toHaveURL(/#\/song\/song-123$/)
  await page.getByRole('button', { name: 'Copiar link' }).click()
  const link = await page.evaluate(() => window.__copiedSongLink)
  expect(link).toMatch(/#\/song\/song-123$/)
  expect(link).not.toContain('test-token')
})

test('o link não mostra uma música sem permissão', async ({ page }) => {
  await prepare(page, false)
  await expect(page.locator('.empty-state')).toContainText('Sem permissão')
  await expect(page.getByRole('button', { name: 'Tocar' })).toHaveCount(0)
})


test('o botão de copiar na biblioteca é acessível pelo teclado', async ({ page }) => {
  await prepare(page, true)
  await page.evaluate(() => { window.location.hash = '#/library' })
  const row = page.locator('.track-row').first()
  await expect(row).toBeVisible()
  await row.locator('.track-like').focus()
  const copy = row.getByRole('button', { name: 'Copiar link da música' })
  for (let i = 0; i < 3 && !(await copy.evaluate((button) => button === document.activeElement)); i++) await page.keyboard.press('Tab')
  await expect(copy).toBeFocused()
  await page.keyboard.press('Enter')
  expect(await page.evaluate(() => window.__copiedSongLink)).toMatch(/#\/song\/song-123$/)
})
