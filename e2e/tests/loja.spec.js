// Store page: prices/checkout links, client "login" through the phone
// registration flow (POST /api/store/register, categoryIds:[]) and proof
// that a newly registered client appears in the admin user list.

const { test, expect } = require('playwright/test')
const {
  trackConsole,
  expectClean,
  randomPhone,
  apiStoreRegister,
  apiListUsers,
  apiDeleteUser,
} = require('./helpers')

test.describe('Loja', () => {
  test('categorias com preços e links de checkout', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.goto('/loja.html')
    await expect(page.locator('#catGrid .card')).toHaveCount(1)
    await expect(page.locator('#catGrid .price').first()).toHaveText('R$ 9,90')
    const hrefs = await page.locator('#catGrid a.buy-btn').evaluateAll((as) => as.map((a) => a.href))
    expect(hrefs.length).toBeGreaterThan(0)
    expect(hrefs[0]).toContain('checkout.exemplo.com')
    // Pack card.
    await expect(page.locator('#packsGrid .card', { hasText: 'Pacote Completo' })).toBeVisible()
    const packHref = await page.locator('#packsGrid a.buy-btn').first().getAttribute('href')
    expect(packHref).toContain('checkout.exemplo.com/pacote-completo')
    expectClean(testInfo, clean)
  })

  test('login de cliente na loja (registro por telefone)', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.goto('/loja.html')
    await page.locator('#phoneInput').fill('(99) 99999-9999')
    await page.locator('#loginBtn').click()
    await expect(page.locator('.user-chip')).toContainText('Cliente Teste')
    expectClean(testInfo, clean)
  })

  test('registro de NOVO cliente via loja → 200 + token + existe no admin', async ({ request }) => {
    const testInfo = test.info()
    const phone = randomPhone()
    const body = await apiStoreRegister(request, phone)
    expect(body.token).toBeTruthy()
    expect(body.user.phone).toBe(phone)

    // The new client must be visible in the admin user list.
    const users = await apiListUsers(request)
    const found = users.find((u) => u.phone === phone)
    expect(found).toBeTruthy()
    expect(found.isAdmin).toBe(false)

    // The account logs in via phone.
    const res = await request.post('http://localhost:4533/auth/login', { data: { phone } })
    expect(res.ok()).toBe(true)
    const login = await res.json()
    expect(login.name).toBe(found.name)

    try {
      await apiDeleteUser(request, found.id)
    } catch {
      /* cleanup best-effort */
    }
    testInfo.annotations.push({ type: 'api-only', description: 'sem navegação de página — nada a rastrear no console' })
  })
})
