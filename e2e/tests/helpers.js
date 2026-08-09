// Shared helpers for the Play Music e2e suite.
// The Go server must be running on :4533 (webServer reuse in the config).
// The PWA overlay re-appears ~1.5s after every real page load (B5) — every
// test must call dismissPwa() after a full goto/reload before interacting.

const fs = require('fs')
const path = require('path')

const BASE = 'http://localhost:4533'

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

const ADMIN_USER = process.env.ND_ADMINUSERNAME || envFromFile('ND_ADMINUSERNAME') || 'admin'
const ADMIN_PASS = process.env.ND_ADMINPASSWORD || envFromFile('ND_ADMINPASSWORD') || '123456'

const CLIENT_PHONE = '(99) 99999-9999' // seeded "Cliente Teste"

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

// ---------- PWA overlay (B5) ----------

// Closes the PWA install overlay. Returns true if it was present and closed.
async function dismissPwa(page, { timeout = 6000 } = {}) {
  const overlay = page.locator('.pwa-overlay')
  try {
    await overlay.waitFor({ state: 'visible', timeout })
  } catch {
    return false
  }
  const btn = overlay.locator('[data-act="dismiss"]')
  if (await btn.count()) {
    await btn.click()
    await overlay.waitFor({ state: 'detached', timeout: 5000 }).catch(() => {})
    return true
  }
  return false
}

// ---------- console error tracking (asserted zero per test) ----------

// `ignore` is an optional predicate (text) => boolean to allow expected
// console noise (e.g. the 401 a wrong-password login is *supposed* to get).
function trackConsole(page, { ignore } = {}) {
  const errors = []
  const all = []
  const skip = (t) => (ignore ? ignore(t) : false)
  page.on('console', (m) => {
    all.push(`[${m.type()}] ${m.text()}`)
    if (m.type() === 'error' && !skip(m.text())) errors.push(`console.error: ${m.text()}`)
  })
  page.on('pageerror', (e) => { if (!skip(e.message)) errors.push(`pageerror: ${e.message}`) })
  return { errors, all }
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
  await page.goto('/')
  await dismissPwa(page)
  await page.getByRole('button', { name: 'Administrador' }).click()
  await page.getByPlaceholder('Usuário ou e-mail').fill(ADMIN_USER)
  await page.getByPlaceholder('Senha').fill(ADMIN_PASS)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await page.locator('.app-shell').waitFor({ state: 'visible', timeout: 15000 })
}

async function loginCliente(page, phone = CLIENT_PHONE) {
  await page.goto('/')
  await dismissPwa(page)
  await page.getByPlaceholder('Telefone (99) 99999-9999').fill(phone)
  await page.getByRole('button', { name: 'Entrar' }).click()
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
  const res = await request.post(`${BASE}/auth/login`, {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  })
  if (!res.ok()) throw new Error(`admin API login failed: ${res.status()}`)
  return (await res.json()).token
}

async function apiAuthHeaders(request) {
  const token = await apiAdminLogin(request)
  return { 'X-ND-Authorization': `Bearer ${token}` }
}

async function apiCreateUser(request, { name, phone }) {
  const headers = await apiAuthHeaders(request)
  const res = await request.post(`${BASE}/api/admin/users`, {
    headers,
    data: { name, phone, isAdmin: false, categoryIds: [] },
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok()) throw new Error(`create user failed ${res.status()}: ${JSON.stringify(body)}`)
  return body
}

async function apiUpdateUser(request, id, payload) {
  const headers = await apiAuthHeaders(request)
  const res = await request.put(`${BASE}/api/admin/users/${id}`, { headers, data: payload })
  if (!res.ok()) throw new Error(`update user failed ${res.status()}`)
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

async function apiCreateCategory(request, name, checkoutUrl = '') {
  const headers = await apiAuthHeaders(request)
  const res = await request.post(`${BASE}/api/admin/categories`, {
    headers,
    data: { name, checkoutUrl },
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

async function apiStoreRegister(request, phone) {
  const res = await request.post(`${BASE}/api/store/register`, {
    data: { phone, categoryIds: [] },
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok()) throw new Error(`store register failed ${res.status()}: ${JSON.stringify(body)}`)
  return body
}

async function apiAdminSongs(request) {
  const headers = await apiAuthHeaders(request)
  const res = await request.get(`${BASE}/api/admin/songs`, { headers })
  if (!res.ok()) throw new Error(`admin songs failed ${res.status()}`)
  return res.json()
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

module.exports = {
  BASE,
  ADMIN_USER,
  ADMIN_PASS,
  CLIENT_PHONE,
  uniqueSuffix,
  randomName,
  randomPhone,
  dismissPwa,
  trackConsole,
  expectClean,
  loginAdmin,
  loginCliente,
  logout,
  apiAdminLogin,
  apiAuthHeaders,
  apiCreateUser,
  apiUpdateUser,
  apiDeleteUser,
  apiListUsers,
  apiCreateCategory,
  apiDeleteCategory,
  apiListCategories,
  apiStoreRegister,
  apiAdminSongs,
  ensureSilenceWav,
}
