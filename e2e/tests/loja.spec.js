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

  test('card de categoria exibe a foto (capa pública, sem JWT)', async ({ page, request }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await page.goto('/loja.html')
    const card = page.locator('#catGrid .card').first()
    const img = card.locator('img.thumb-img')
    // A capa vem da rota pública da loja (não de /api/artwork que exige JWT).
    await expect(img).toBeVisible()
    const src = await img.getAttribute('src')
    expect(src).toMatch(/^\/api\/store\/categories\/[0-9a-f]+\/photo\?size=300$/)
    // A rota pública responde sem autenticação com a imagem (foto ou
    // placeholder) — nunca 401. `request.get` resolve contra a baseURL da
    // config (mesmo host do servidor sob teste).
    const res = await request.get(src)
    expect(res.status()).toBe(200)
    expect(res.headers()['content-type'] || '').toContain('image/jpeg')
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
