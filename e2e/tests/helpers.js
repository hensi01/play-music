// Shared helpers for the Play Music e2e suite.
// The Go server must be running on :4533 (webServer reuse in the config).
// The PWA overlay re-appears ~1.5s after every real page load, so a spec
// either calls markPwaSeen() (client-keys) or dismissPwa() before interacting.

const fs = require('fs')
const path = require('path')
const { AUTH_HEADER } = require('./client-keys')

// API base for direct request-fixture calls (UI calls use the page origin).
// PM_E2E_BASEURL overrides the target (validation servers on other ports).
const BASE = process.env.PM_E2E_BASEURL || 'http://localhost:4533'

// ---------- .env (runtime, never committed) ----------

function envFromFile(key) {
  try {
    const p = path.join(__dirname, '..', '..', '.env')
    const raw = fs.readFileSync(p, 'utf8')
    for (const line of raw.split(/\r?\n/)) {
      const m = line.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$/)
      if (m && m[1] === key) {
        let v = m[2].trim()
        if (v.startsWith('"') && v.endsWith('"')) v = v.slice(1, -1)
        return v
      }
    }
  } catch { /* no .env */ }
  return undefined
}

// Credenciais vêm SOMENTE do ambiente ou do .env (nunca hardcoded no código).
function requireEnv(key) {
  const v = process.env[key] || envFromFile(key)
  if (!v) throw new Error(`${key} não configurada — defina no .env (raiz do projeto) ou no ambiente`)
  return v
}

const ADMIN_USER = requireEnv('ND_ADMINUSERNAME')
const ADMIN_PASS = requireEnv('ND_ADMINPASSWORD')

// Seed facts: the client the suite logs in as, and the category the seed
// catalog must always contain. Specs target them through here.
const CLIENT_PHONE = '(99) 99999-9999'
const CLIENT_NAME = 'Cliente Teste'
const SEEDED_CATEGORY = 'Cristão'

// Placeholders of the login and user forms (labels, not values).
const PHONE_PLACEHOLDER = 'Telefone (99) 99999-9999'
const USER_PLACEHOLDER = 'Usuário ou e-mail'

// ---------- random fixtures ----------

let seq = 0
function uniqueSuffix() {
  seq += 1
  return `${Date.now().toString(36)}${seq}${Math.floor(Math.random() * 1e4).toString(36)}`
}

function randomName(prefix = 'PW') {
  return `${prefix} ${uniqueSuffix()}`
}

// 11 digits, first digit != 0, formatted as (XX) XXXXX-XXXX.
function randomPhone() {
  const ddd = 11 + Math.floor(Math.random() * 88)
  let n = 9 // mobile prefix
  for (let i = 0; i < 8; i++) n = n * 10 + Math.floor(Math.random() * 10)
  const s = String(n)
  return `(${ddd}) ${s.slice(0, 5)}-${s.slice(5)}`
}

// ---------- PWA overlay ----------

// Simulates a browser that supports the native install prompt, then closes the
// one-button overlay by clicking its backdrop.
async function dismissPwa(page) {
  await page.evaluate(() => {
    const event = new Event('beforeinstallprompt', { cancelable: true })
    event.prompt = () => Promise.resolve()
    event.userChoice = Promise.resolve({ outcome: 'dismissed' })
    window.dispatchEvent(event)
  }).catch(() => {})
  const overlay = page.locator('.pwa-overlay')
  try {
    await overlay.waitFor({ state: 'visible', timeout: 6000 })
  } catch {
    return
  }
  await overlay.click({ position: { x: 1, y: 1 } })
  await overlay.waitFor({ state: 'detached', timeout: 5000 }).catch(() => {})
}

// ---------- console error tracking (asserted zero per test) ----------

// `ignore` is an optional predicate (text) => boolean to allow expected
// console noise (e.g. the 401 a wrong-password login is *supposed* to get).
function trackConsole(page, { ignore } = {}) {
  const errors = []
  const skip = (t) => (ignore ? ignore(t) : false)
  page.on('console', (m) => {
    if (m.type() === 'error' && !skip(m.text())) errors.push(`console.error: ${m.text()}`)
  })
  page.on('pageerror', (e) => { if (!skip(e.message)) errors.push(`pageerror: ${e.message}`) })
  return { errors }
}

// Fails the current test if any console/page error was recorded.
function expectClean(testInfo, tracker, label = '') {
  if (tracker.errors.length > 0) {
    const head = label ? `${label} — ` : ''
    const msg = `${head}${tracker.errors.length} console error(s):\n` + tracker.errors.map((e) => `  ${e}`).join('\n')
    testInfo.annotations.push({ type: 'console-errors', description: msg })
    throw new Error(msg)
  }
}

// ---------- UI flows ----------

async function loginAdmin(page) {
  await page.goto('/admin/login')
  await dismissPwa(page)
  await page.getByPlaceholder(USER_PLACEHOLDER).fill(ADMIN_USER)
  await page.getByPlaceholder('Senha').fill(ADMIN_PASS)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15000 })
}

async function loginCliente(page, phone = CLIENT_PHONE) {
  await page.goto('/')
  await dismissPwa(page)
  await page.getByPlaceholder(PHONE_PLACEHOLDER).fill(phone)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15000 })
}

// Registers a brand-new client from the login screen (the "Criar conta" mode).
async function registerCliente(page) {
  await page.goto('/')
  await page.getByRole('button', { name: 'Criar conta' }).click()
  await page.getByPlaceholder(PHONE_PLACEHOLDER).fill('11987654321')
  await page.locator('.login-submit').click()
  await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15000 })
}

// Logs out through the Settings page (its "Sair" button is visible on both
// desktop and mobile; the sidebar "Sair" is hidden on mobile).
async function logout(page) {
  await page.evaluate(() => { window.location.hash = '#/settings' })
  await page.locator('.settings-actions').getByRole('button', { name: 'Sair' }).click()
  await page.locator('.login-screen').waitFor({ state: 'visible', timeout: 10000 })
}

// ---------- API helpers (request fixture) ----------

async function apiAdminLogin(request) {
  const res = await request.post(`${BASE}/auth/admin/login`, {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  })
  if (!res.ok()) throw new Error(`admin API login failed: ${res.status()}`)
  return (await res.json()).token
}

async function apiAuthHeaders(request) {
  const token = await apiAdminLogin(request)
  return { [AUTH_HEADER]: `Bearer ${token}` }
}

async function apiDeleteUser(request, id) {
  const headers = await apiAuthHeaders(request)
  const res = await request.delete(`${BASE}/api/admin/users/${id}`, { headers })
  if (!res.ok()) throw new Error(`delete user failed ${res.status()}`)
}

async function apiListUsers(request) {
  const headers = await apiAuthHeaders(request)
  const res = await request.get(`${BASE}/api/admin/users`, { headers })
  if (!res.ok()) throw new Error(`list users failed ${res.status()}`)
  return res.json()
}

async function apiCreateCategory(request, name) {
  const headers = await apiAuthHeaders(request)
  const res = await request.post(`${BASE}/api/admin/categories`, {
    headers,
    data: { name },
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok()) throw new Error(`create category failed ${res.status()}: ${JSON.stringify(body)}`)
  return body
}

async function apiDeleteCategory(request, id) {
  const headers = await apiAuthHeaders(request)
  const res = await request.delete(`${BASE}/api/admin/categories/${id}`, { headers })
  if (!res.ok()) throw new Error(`delete category failed ${res.status()}`)
}

async function apiListCategories(request) {
  const headers = await apiAuthHeaders(request)
  const res = await request.get(`${BASE}/api/admin/categories`, { headers })
  if (!res.ok()) throw new Error(`list categories failed ${res.status()}`)
  return res.json()
}

async function apiAdminSongs(request) {
  const headers = await apiAuthHeaders(request)
  const res = await request.get(`${BASE}/api/admin/songs`, { headers })
  if (!res.ok()) throw new Error(`admin songs failed ${res.status()}`)
  return res.json()
}

// Uploads a category cover via API. Uses the app's own PNG (icon-512.png,
// served at the root) so no binary fixture has to live in the repo. With
// `invalid: true` it uploads the WAV fixture instead (validation must 400).
async function apiUploadCategoryPhoto(request, id, { invalid = false } = {}) {
  const headers = await apiAuthHeaders(request)
  let buffer, name, mimeType
  if (invalid) {
    buffer = fs.readFileSync(ensureSilenceWav(30))
    name = 'silence.wav'
    mimeType = 'audio/wav'
  } else {
    const img = await request.get(`${BASE}/icon-512.png`)
    if (!img.ok()) throw new Error(`fixture png unavailable: ${img.status()}`)
    buffer = await img.body()
    name = 'icon-512.png'
    mimeType = 'image/png'
  }
  const res = await request.post(`${BASE}/api/admin/categories/${id}/photo`, {
    headers,
    multipart: { photo: { name, mimeType, buffer } },
  })
  if (!res.ok()) throw new Error(`category photo upload failed ${res.status()}`)
  return res
}

async function apiDeleteCategoryPhoto(request, id) {
  const headers = await apiAuthHeaders(request)
  const res = await request.delete(`${BASE}/api/admin/categories/${id}/photo`, { headers })
  if (!res.ok()) throw new Error(`category photo delete failed ${res.status()}`)
}

// Fetches the artwork byte length for an entity as an authenticated client.
// Used to prove a category cover changed (upload ≠ placeholder) and reverted
// (delete = placeholder again).
async function apiArtworkBytes(request, id) {
  const headers = await apiAuthHeaders(request)
  const res = await request.get(`${BASE}/api/artwork/${id}`, { headers })
  if (!res.ok()) throw new Error(`artwork fetch failed ${res.status()}`)
  return (await res.body()).length
}

// ---------- media fixtures ----------

// A 1x1 transparent PNG, for specs that answer /api/artwork themselves.
const TINY_PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=', 'base64')

// Answers every /api/stream/* request with a silent WAV that supports ranges,
// so playback and seeking are deterministic in headless.
async function stubStream(page) {
  await page.route('**/api/stream/**', (route) =>
    route.fulfill({ status: 200, contentType: 'audio/wav', headers: { 'Accept-Ranges': 'bytes' }, body: silenceWavBuffer() }),
  )
}

async function stubArtwork(page) {
  await page.route('**/api/artwork/**', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: TINY_PNG }),
  )
}

// ---------- layout ----------

// Fails when the page scrolls horizontally: no route may exceed the viewport.
async function expectNoOverflow(page) {
  const r = await page.evaluate(() => ({
    sw: document.documentElement.scrollWidth,
    iw: window.innerWidth,
    bodySw: document.body.scrollWidth,
  }))
  expect(r.sw, `scrollWidth ${r.sw} > innerWidth ${r.iw}`).toBeLessThanOrEqual(r.iw)
  expect(r.bodySw, `body scrollWidth ${r.bodySw} > innerWidth ${r.iw}`).toBeLessThanOrEqual(r.iw)
}

// ---------- audio fixture (deterministic player) ----------

// A silent WAV so the audio element can actually play/seek in headless.
function ensureSilenceWav(seconds = 30) {
  const dir = path.join(__dirname, '..', 'fixtures')
  const file = path.join(dir, `silence-${seconds}s.wav`)
  if (fs.existsSync(file)) return file
  fs.mkdirSync(dir, { recursive: true })
  const rate = 44100
  const samples = rate * seconds
  const dataSize = samples * 2
  const buf = Buffer.alloc(44 + dataSize)
  buf.write('RIFF', 0)
  buf.writeUInt32LE(36 + dataSize, 4)
  buf.write('WAVE', 8)
  buf.write('fmt ', 12)
  buf.writeUInt32LE(16, 16)
  buf.writeUInt16LE(1, 20) // PCM
  buf.writeUInt16LE(1, 22) // mono
  buf.writeUInt32LE(rate, 24)
  buf.writeUInt32LE(rate * 2, 28) // byte rate
  buf.writeUInt16LE(2, 32) // block align
  buf.writeUInt16LE(16, 34) // bits per sample
  buf.write('data', 36)
  buf.writeUInt32LE(dataSize, 40)
  fs.writeFileSync(file, buf)
  return file
}

// A silent WAV as a Buffer, for specs that answer /api/stream themselves.
function silenceWavBuffer(seconds = 30) {
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

module.exports = {
  BASE,
  ADMIN_USER,
  CLIENT_PHONE,
  CLIENT_NAME,
  SEEDED_CATEGORY,
  PHONE_PLACEHOLDER,
  USER_PLACEHOLDER,
  randomName,
  randomPhone,
  dismissPwa,
  trackConsole,
  expectClean,
  loginAdmin,
  loginCliente,
  registerCliente,
  logout,
  apiAdminLogin,
  apiAuthHeaders,
  apiDeleteUser,
  apiListUsers,
  apiCreateCategory,
  apiDeleteCategory,
  apiListCategories,
  apiAdminSongs,
  apiUploadCategoryPhoto,
  apiDeleteCategoryPhoto,
  apiArtworkBytes,
  ensureSilenceWav,
  stubStream,
  stubArtwork,
  expectNoOverflow,
}
