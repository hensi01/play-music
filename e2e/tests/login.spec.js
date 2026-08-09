// Login flows: admin (toggle + credentials), client (phone), logout,
// wrong password (no crash) and the B1 regression: a real expired token
// must produce zero console errors on the login screen.

const { test, expect } = require('playwright/test')
const {
  CLIENT_PHONE,
  dismissPwa,
  trackConsole,
  expectClean,
  loginAdmin,
  loginCliente,
  logout,
  apiAdminLogin,
} = require('./helpers')

test.describe('Login', () => {
  test('admin login via UI — app shell + nav Administração', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await expect(page.locator('.app-shell')).toBeVisible()
    // DOM presence (the sidebar itself is hidden on mobile, the nav item
    // must still exist for admins).
    await expect(page.locator('.sidebar-nav .nav-link', { hasText: 'Administração' })).toHaveCount(1)
    await expect(page.locator('.sidebar-user-name')).toHaveText('admin')
    expectClean(testInfo, clean)
  })

  test('cliente login via UI (telefone)', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginCliente(page, CLIENT_PHONE)
    await expect(page.locator('.app-shell')).toBeVisible()
    await expect(page.locator('.sidebar-user-name')).toHaveText('Cliente Teste')
    expectClean(testInfo, clean)
  })

  test('logout volta para a tela de login', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await logout(page)
    await expect(page.locator('.login-screen')).toBeVisible()
    expectClean(testInfo, clean)
  })

  test('senha errada mostra erro sem crash', async ({ page }) => {
    const testInfo = test.info()
    // A wrong password is *supposed* to 401 — the browser logs the failed
    // resource load as a console error. That is correct product behaviour,
    // so we allow this specific 401 while still failing on any other error.
    const clean = trackConsole(page, { ignore: (t) => t.includes('status of 401') })
    await page.goto('/')
    await dismissPwa(page)
    await page.getByRole('button', { name: 'Administrador' }).click()
    await page.getByPlaceholder('Usuário ou e-mail').fill('admin')
    await page.getByPlaceholder('Senha').fill('senha-errada-123')
    await page.getByRole('button', { name: 'Entrar' }).click()
    await expect(page.locator('.login-error')).toContainText('usuário ou senha inválidos')
    await expect(page.locator('.app-shell')).toHaveCount(0)
    // Still on the login screen, form usable again.
    await expect(page.getByRole('button', { name: 'Entrar' })).toBeEnabled()
    expectClean(testInfo, clean)
  })

  test('token expirado REAL → 0 console errors (B1 regressão)', async ({ request, page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    const token = await apiAdminLogin(request)
    // Re-encode the real token's payload with an expired exp (same digit
    // count as the server's exp: 10 digits), keeping header/signature.
    const expired = await page.evaluate((raw) => {
      const pad = (s) => s + '='.repeat((4 - (s.length % 4)) % 4)
      const parts = raw.split('.')
      const json = JSON.parse(atob(pad(parts[1]).replace(/-/g, '+').replace(/_/g, '/')))
      json.exp = 1000000000 // past (2026-09-01)
      const b64 = btoa(JSON.stringify(json)).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_')
      return [parts[0], b64, parts[2]].join('.')
    }, token)

    await page.goto('/')
    await page.evaluate((t) => localStorage.setItem('pm_token', t), expired)
    await page.reload()
    await dismissPwa(page)

    await expect(page.locator('.login-screen')).toBeVisible()
    await expect(page.locator('.app-shell')).toHaveCount(0)
    // refreshAuth cleared the expired token silently (no 401 fetch).
    const stored = await page.evaluate(() => localStorage.getItem('pm_token'))
    expect(stored).toBeNull()
    expectClean(testInfo, clean, 'expired token')
  })
})
