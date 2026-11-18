// Login flows: admin (/admin/login credentials), client (phone), logout and
// wrong password (no crash). An expired token must produce zero console errors
// on the login screen.

const { test, expect } = require('playwright/test')
const {
  ADMIN_USER,
  CLIENT_PHONE,
  CLIENT_NAME,
  USER_PLACEHOLDER,
  dismissPwa,
  trackConsole,
  expectClean,
  loginAdmin,
  loginCliente,
  logout,
  apiAdminLogin,
} = require('./helpers')
const { TOKEN_KEY } = require('./client-keys')

test.describe('Login', () => {
  test('admin login via UI — app shell + nav Administração', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    // DOM presence (the sidebar itself is hidden on mobile, the nav item
    // must still exist for admins).
    await expect(page.locator('.sidebar-nav .nav-link', { hasText: 'Administração' })).toHaveCount(1)
    await expect(page.locator('.sidebar-user-name')).toHaveText(ADMIN_USER)
    expectClean(testInfo, clean)
  })

  test('cliente login via UI (telefone)', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginCliente(page, CLIENT_PHONE)
    await expect(page.locator('.sidebar-user-name')).toHaveText(CLIENT_NAME)
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
    // The admin form lives only at /admin/login; the client login screen has no
    // admin entry, so the wrong-password case starts there.
    await page.goto('/admin/login')
    await dismissPwa(page)
    await page.getByPlaceholder(USER_PLACEHOLDER).fill(ADMIN_USER)
    await page.getByPlaceholder('Senha').fill('senha-errada-123')
    await page.getByRole('button', { name: 'Entrar no painel' }).click()
    await expect(page.locator('.login-error')).toContainText('usuário ou senha inválidos')
    await expect(page.locator('.app-shell')).toHaveCount(0)
    // Still on the login screen, form usable again.
    await expect(page.getByRole('button', { name: 'Entrar no painel' })).toBeEnabled()
    expectClean(testInfo, clean)
  })

  test('token expirado REAL → 0 console errors', async ({ request, page }) => {
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
    await page.evaluate(([key, t]) => localStorage.setItem(key, t), [TOKEN_KEY, expired])
    await page.reload()
    await dismissPwa(page)

    await expect(page.locator('.login-screen')).toBeVisible()
    await expect(page.locator('.app-shell')).toHaveCount(0)
    // refreshAuth cleared the expired token silently (no 401 fetch).
    const stored = await page.evaluate((key) => localStorage.getItem(key), TOKEN_KEY)
    expect(stored).toBeNull()
    expectClean(testInfo, clean, 'expired token')
  })
})
