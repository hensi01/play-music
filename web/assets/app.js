// Mi Ritmo web UI — vanilla JS, hash-based router.

import { api, endpoints, artworkUrl, getToken, setToken, readAppConfig, applyPhoneMask, phoneMask } from './api.js'
import * as player from './player.js'
// admin.js is imported with the build version so the service worker (which
// caches by URL) never serves a stale copy after a redeploy.
const { renderAdmin, clearPendingAPIKey } = await import(`./admin.js?v=${readAppConfig().version}`)

const appConfig = readAppConfig()

const root = document.getElementById('app')

// ---------- Helpers ----------

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue
    if (k === 'class') node.className = v
    else if (k === 'html') node.innerHTML = v
    else if (k === 'style') node.setAttribute('style', v)
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2), v)
    else node.setAttribute(k, v === true ? '' : String(v))
  }
  for (const c of children.flat()) {
    if (c == null) continue
    node.append(c instanceof Node ? c : document.createTextNode(String(c)))
  }
  return node
}

function fmtDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00'
  const s = Math.floor(seconds % 60)
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}:${String(s).padStart(2, '0')}`
  const h = Math.floor(m / 60)
  return `${h}:${String(m % 60).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function fmtDurationLong(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0 min'
  const m = Math.max(1, Math.round(seconds / 60))
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  return `${h} h ${m % 60} min`
}

function musicas(n) {
  return `${n} ${n === 1 ? 'música' : 'músicas'}`
}

function greeting() {
  const h = new Date().getHours()
  if (h < 6) return 'Boa madrugada'
  if (h < 12) return 'Bom dia'
  if (h < 18) return 'Boa tarde'
  return 'Boa noite'
}

function spinner(text = 'Carregando…') {
  return el('div', { class: 'spinner-wrap' }, el('div', { class: 'spinner' }), text)
}

function emptyState(msg) {
  return el('div', { class: 'empty-state' }, el('p', {}, msg))
}

function emptyState2(msg, hint) {
  return el('div', { class: 'empty-state' }, el('p', {}, msg), el('p', {}, hint))
}

let toastTimer
function showToast(message) {
  let toast = document.getElementById('app-toast')
  if (!toast) {
    toast = el('div', { id: 'app-toast', class: 'app-toast', role: 'status', 'aria-live': 'polite' })
    document.body.append(toast)
  }
  toast.textContent = message
  toast.classList.add('visible')
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => toast.classList.remove('visible'), 2200)
}

// ---------- Icons (inline SVG) ----------

const icons = {
  home: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>',
  search: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>',
  library: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m16 6 4 14"/><path d="M12 6v14"/><path d="M8 8v12"/><path d="M4 4v16"/></svg>',
  heart: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/></svg>',
  video: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 8-6 4 6 4V8Z"/><rect x="2" y="6" width="14" height="12" rx="2"/></svg>',
  link: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.1 0l3-3a5 5 0 0 0-7.1-7.1l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.1 0l-3 3a5 5 0 0 0 7.1 7.1l1.7-1.7"/></svg>',
  download: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12m-5-5 5 5 5-5M4 17v3h16v-3"/></svg>',
  clock: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
  music: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
  settings: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>',
  logout: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/></svg>',
  list: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6h13"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M3 6h.01"/><path d="M3 12h.01"/><path d="M3 18h.01"/></svg>',
  cart: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="21" r="1"/><circle cx="19" cy="21" r="1"/><path d="M2.05 2.05h2l2.66 12.42a2 2 0 0 0 2 1.58h9.78a2 2 0 0 0 1.95-1.57l1.65-7.43H5.12"/></svg>',
  play: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="6 3 20 12 6 21 6 3"/></svg>',
  pause: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>',
  playSmall: '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><polygon points="6 3 20 12 6 21 6 3"/></svg>',
  prev: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="19 20 9 12 19 4 19 20"/><rect x="5" y="4" width="2.5" height="16" rx="1"/></svg>',
  next: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 4 15 12 5 20 5 4"/><rect x="16.5" y="4" width="2.5" height="16" rx="1"/></svg>',
  rewind5: '<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M11.99 2C7.5 2 3.85 4.86 2.55 8.86H5.1c.98-2.68 3.57-4.61 6.55-4.61 3.87 0 7 3.13 7 7s-3.13 7-7 7c-2.98 0-5.57-1.93-6.55-4.61H2.55C3.85 19.14 7.5 22 11.99 22c5.52 0 10-4.48 10-10s-4.48-10-10-10z"/><text x="12" y="15.5" font-size="11" font-weight="700" text-anchor="middle" fill="currentColor" stroke="none">5</text></svg>',
  forward5: '<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><g transform="scale(-1,1) translate(-24,0)"><path d="M11.99 2C7.5 2 3.85 4.86 2.55 8.86H5.1c.98-2.68 3.57-4.61 6.55-4.61 3.87 0 7 3.13 7 7s-3.13 7-7 7c-2.98 0-5.57-1.93-6.55-4.61H2.55C3.85 19.14 7.5 22 11.99 22c5.52 0 10-4.48 10-10s-4.48-10-10-10z"/><text x="12" y="15.5" font-size="11" font-weight="700" text-anchor="middle" fill="currentColor" stroke="none">5</text></g></svg>',
  shuffle: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 18h1.4c1.3 0 2.5-.6 3.3-1.7l6.1-8.6c.8-1.1 2-1.7 3.3-1.7H22"/><path d="m18 2 4 4-4 4"/><path d="M2 6h1.9c1.5 0 2.9.9 3.6 2.2"/><path d="M22 18h-5.9c-1.3 0-2.6-.7-3.3-1.8l-.5-.8"/><path d="m18 14 4 4-4 4"/></svg>',
  repeat: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m17 2 4 4-4 4"/><path d="M3 11v-1a4 4 0 0 1 4-4h14"/><path d="m7 22-4-4 4-4"/><path d="M21 13v1a4 4 0 0 1-4 4H3"/></svg>',
  volume: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/></svg>',
  volumeX: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><line x1="22" x2="16" y1="9" y2="15"/><line x1="16" x2="22" y1="9" y2="15"/></svg>',
  max: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>',
  chevronDown: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>',
  arrowLeft: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 19-7-7 7-7"/><path d="M19 12H5"/></svg>',
  trash: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>',
  plus: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5v14"/></svg>',
  menu: '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>',
  x: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>',
}

function icon(name) {
  const wrap = document.createElement('span')
  wrap.innerHTML = icons[name] || ''
  const node = wrap.firstChild
  if (node) node.dataset.icon = name
  return node
}

// ---------- Auth state ----------

let auth = { user: null, loading: true }
let notificationGate = { userId: null, status: 'checking' }
let notificationGateNeedsRedirect = false
let notificationGateSeq = 0

let sessionEpoch = 0
function setAuth(patch) {
  const previousId = auth.user?.id || null
  const nextId = Object.hasOwn(patch, 'user') ? (patch.user?.id || null) : previousId
  if (previousId !== nextId) {
    clearPendingAPIKey()
    sessionEpoch++
    playlistsCache = []
    libData = null
    libLoading = false
    searchSeq++
    searchResults = null
    searchQuery = ''
    player.clearSession()
  }
  auth = { ...auth, ...patch }
  window.__pmAdmin = !!auth.user?.isAdmin
  if (window.__pmAdmin) document.querySelector('.pwa-overlay')?.remove()
  if (previousId !== nextId) { notificationGateSeq++; notificationGateNeedsRedirect = false; notificationGate = { userId: nextId, status: 'checking' } }
  render()
  if (previousId !== nextId && nextId) void reconcilePushSubscription(nextId).finally(() => { if (auth.user?.id === nextId) void verifyNotificationGate(nextId) })
}

// Sessions share localStorage across tabs. Clear sensitive admin UI when a
// different account signs in or the session is removed elsewhere.
window.addEventListener('storage', (event) => {
  if ((event.key !== 'pm_token' && event.key !== null) || !auth.user) return
  try {
    const part = (getToken() || '').split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const uid = JSON.parse(atob(part.padEnd(Math.ceil(part.length / 4) * 4, '='))).uid
    if (uid === auth.user.id) return
  } catch { /* missing or invalid token ends this tab's session */ }
  setAuth({ user: null, loading: false })
})

// Lê a claim `exp` de um JWT (header.payload.signature, base64url) sem
// validar a assinatura — suficiente para evitar o fetch de /api/me com token
// expirado (o 401 resultante vira ruído no console na tela de login).
// Retorna timestamp em ms, ou null se o token não tiver exp decodificável.
function jwtExpiry(token) {
  try {
    const payload = token.split('.')[1]
    if (!payload) return null
    // JWT (RFC 7515) é base64url SEM padding; atob() no browser é estrito e
    // lança InvalidCharacterError se o comprimento não for múltiplo de 4 —
    // repor o padding '=' antes de decodificar.
    const b64 = payload.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (payload.length % 4)) % 4)
    const json = JSON.parse(atob(b64))
    return typeof json.exp === 'number' ? json.exp * 1000 : null
  } catch {
    return null
  }
}

async function refreshAuth() {
  const token = getToken()
  if (!token) {
    setAuth({ loading: false, user: null })
    return
  }
  const exp = jwtExpiry(token)
  if (exp !== null && exp <= Date.now()) {
    // Token expirado: limpa silenciosamente e nem faz o fetch — sem 401 no
    // console. (Token sem exp decodificável segue o fluxo antigo: fetch e,
    // em 401, o api.js já limpa o token.)
    setToken(null)
    setAuth({ user: null, loading: false })
    return
  }
  try {
    const user = await endpoints.me()
    setAuth({ user, loading: false })
  } catch {
    setAuth({ user: null, loading: false })
  }
}

async function doLogin(mode, credential, password) {
  const body = mode === 'client' ? { phone: credential } : { username: credential, password }
  const endpoint = mode === 'admin' ? '/auth/admin/login' : '/auth/login'
  const res = await api.fetch(endpoint, {
    method: 'POST',
    anonymous: true,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  setToken(res.token)
  setAuth({
    user: { id: res.id, username: res.username, phone: res.phone, name: res.name, isAdmin: res.isAdmin },
    loading: false,
  })
}

function doLogout() {
  setToken(null)
  setAuth({ user: null })
  navigate('/')
}

window.addEventListener('pm:unauthorized', () => {
  // Sessão expirada/inválida durante o uso: desloga. Durante uma tentativa de
  // login falha (401), o usuário já está deslogado — preserva a mensagem de erro.
  if (auth.user) setAuth({ user: null })
})

// ---------- Router (hash-based) ----------

function parseHash() {
  const raw = window.location.hash.replace(/^#/, '') || '/'
  const [pathPart, queryPart] = raw.split('?')
  const params = new URLSearchParams(queryPart || '')
  return { path: pathPart || '/', params }
}

function navigate(to) {
  window.location.hash = to
}

// ---------- Track row ----------

function trackRow(song, index, opts = {}) {
  const { onPlay, onQueue, compactActions = false } = opts
  const isCurrent = player.getPlayerState().current?.id === song.id

  const numCell = el(
    'div',
    { class: 'track-number' },
    el('span', { class: 'track-num-text' }, String((index ?? 0) + 1)),
    onPlay
      ? el(
          'button',
          { class: 'track-play-btn', 'aria-label': 'Tocar', onclick: () => onPlay(song, index ?? 0) },
          icon('playSmall'),
        )
      : null,
  )

  const likeBtn = el(
    'button',
    {
      class: `track-like ${song.liked ? 'liked' : ''}`,
      'data-song-id': song.id,
      'aria-label': song.liked ? 'Descurtir' : 'Curtir',
      'aria-pressed': song.liked,
      onclick: () => toggleLike(song.id, likeBtn),
    },
    icon('heart'),
  )

  const shareBtn = compactActions ? null : el(
    'button',
    { class: 'track-add track-share', 'aria-label': 'Copiar link da música', onclick: (e) => { e.stopPropagation(); copySongLink(song.id) } },
    icon('link'),
  )

  const addBtn = compactActions ? null : el(
    'button',
    { class: 'track-add', 'aria-label': 'Adicionar à playlist', onclick: (e) => { e.stopPropagation(); openPlaylistPicker(song) } },
    icon('plus'),
  )

  const queueBtn = onQueue
    ? el(
        'button',
        {
          class: 'track-add track-queue',
          'aria-label': `Adicionar ${song.title} à fila`,
          title: 'Adicionar à fila',
          onclick: (e) => {
            e.stopPropagation()
            onQueue(song)
          },
        },
        icon('list'),
      )
    : null

  return el(
    'div',
    { class: `track-row${isCurrent ? ' playing' : ''}`, 'data-song-id': song.id },
    numCell,
      el(onPlay ? 'button' : 'div', {
        class: 'track-main',
        type: onPlay ? 'button' : undefined,
        'aria-label': onPlay ? `Tocar ${song.title}` : undefined,
        onclick: onPlay ? () => onPlay(song, index ?? 0) : undefined,
      },
      el('img', { class: 'track-art', src: artworkUrl(song.id, 48), alt: '', loading: 'lazy' }),
      el(
        'div',
        { class: 'track-info' },
        el('p', { class: 'track-title' }, song.title),
        el('p', { class: 'track-artist' }, song.artist || 'Desconhecido'),
      ),
    ),
    el('div', { class: 'track-actions' }, likeBtn, queueBtn, addBtn, shareBtn, downloadButton(song), el('span', { class: 'track-duration' }, fmtDuration(song.duration))),
  )
}

function downloadButton(song) {
  return el('button', { class: 'download-button', 'aria-label': 'Baixar música', title: 'Baixar música', onclick: async (e) => {
    e.stopPropagation()
    const btn = e.currentTarget
    btn.disabled = true
    try {
      const result = await endpoints.download(song.id)
      const link = el('a', { href: result.url, download: '', rel: 'noreferrer' })
      document.body.append(link); link.click(); link.remove()
    } catch (err) { alert(`Não foi possível baixar a música: ${err.message}`) }
    finally { btn.disabled = false }
  } }, icon('download'))
}

// The public-facing path stays stable; authentication is checked on opening.
async function copySongLink(songId) {
  const link = `${window.location.origin}${window.location.pathname}#/song/${encodeURIComponent(songId)}`
  try {
    await navigator.clipboard.writeText(link)
    alert('Link copiado.')
  } catch {
    window.prompt('Copie o link da música:', link)
  }
}

// ---------- Add to playlist ----------

function openPlaylistPicker(song) {
  const overlay = el(
    'div',
    { class: 'modal-overlay', onclick: (e) => { if (e.target === overlay) overlay.remove() } },
    el(
      'div',
      { class: 'modal' },
      el('h3', {}, 'Adicionar à playlist'),
      el('p', { class: 'modal-sub' }, `${song.title}${song.artist ? ` — ${song.artist}` : ''}`),
      playlistsCache.length === 0
        ? el('p', { class: 'modal-empty' }, 'Nenhuma playlist ainda.')
        : playlistsCache.map((pl) =>
            el(
              'button',
              {
                class: 'modal-item',
                onclick: async () => {
                  try {
                    await endpoints.addPlaylistTracks(pl.id, [song.id])
                    alert(`Adicionada à playlist “${pl.name}”.`)
                    void loadPlaylists()
                  } catch (err) {
                    alert(err.message)
                  }
                  overlay.remove()
                },
              },
              pl.name,
            ),
          ),
      el(
        'button',
        {
          class: 'modal-item new',
          onclick: async () => {
            overlay.remove()
            const name = window.prompt('Nome da nova playlist:')
            if (!name?.trim()) return
            try {
              await endpoints.createPlaylist(name.trim(), [song.id])
              window.dispatchEvent(new Event('pm:playlists-changed'))
              alert('Playlist criada com a música.')
            } catch (err) {
              alert(err.message)
            }
          },
        },
        'Nova playlist',
      ),
      el('button', { class: 'modal-close', onclick: () => overlay.remove() }, 'Cancelar'),
    ),
  )
  document.body.append(overlay)
}

async function toggleLike(songId, btn) {
  const wasLiked = btn.classList.contains('liked')
  btn.classList.toggle('liked', !wasLiked)
  const isCurrent = () => player.getPlayerState().current?.id === songId
  try {
    if (!wasLiked) await endpoints.like(songId)
    else await endpoints.unlike(songId)
  } catch (err) {
    // Falha: reverte o botão E o estado do player (barra inferior) — o
    // otimismo só vale em sucesso, senão a lista e a barra dessincronizam.
    btn.classList.toggle('liked', wasLiked)
    if (isCurrent()) player.setLiked(wasLiked)
    console.error('Falha ao alternar curtida:', err)
    return
  }
  // Sincroniza a barra inferior quando a música curtida é a atual.
  if (isCurrent()) player.setLiked(!wasLiked)
}

// ---------- Card ----------

function card({ image, title, subtitle, onClick, onPlay, square = true }) {
  return el(
    'div',
    {
      class: 'card',
      role: onClick && !onPlay ? 'button' : undefined,
      tabindex: onClick && !onPlay ? '0' : undefined,
      onclick: onClick,
      onkeydown: onClick && !onPlay ? (event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onClick(event)
        }
      } : undefined,
    },
    el(
      'div',
      { class: `card-image-wrap ${square ? 'square' : ''}` },
      el('img', { class: 'card-image', src: image, alt: '', loading: 'lazy' }),
      onPlay
        ? el(
            'button',
            { class: 'card-play', 'aria-label': 'Tocar', onclick: (e) => { e.stopPropagation(); onPlay() } },
            icon('playSmall'),
          )
        : null,
    ),
    el('p', { class: 'card-title' }, title),
    el('p', { class: 'card-subtitle' }, subtitle),
  )
}

function section(title, children) {
  return el(
    'section',
    { class: 'section' },
    el('h2', { class: 'section-title' }, title),
    el('div', { class: 'card-row' }, ...children),
  )
}

// songCard: a playable card for a song (photo, title, artist).
function songCard(song, onPlay) {
  return card({
    image: artworkUrl(song.id, 300),
    title: song.title,
    subtitle: song.artist || 'Desconhecido',
    onClick: onPlay,
    onPlay,
  })
}

// categoryCard: a card that opens a category page with its songs.
function categoryCard(c) {
  return card({
    image: artworkUrl(c.id, 300),
    title: c.name,
    subtitle: musicas(c.songCount ?? 0),
    onClick: () => navigate(`/category/${c.id}`),
  })
}

function albumCard(album, from = '/library') {
  const details = [album.artist, album.year > 0 ? album.year : null].filter(Boolean).join(' • ')
  return card({
    image: artworkUrl(album.id, 300),
    title: album.name,
    subtitle: [details, musicas(album.songCount ?? 0)].filter(Boolean).join(' • '),
    onClick: () => navigate(`/album/${encodeURIComponent(album.id)}?from=${encodeURIComponent(from)}`),
  })
}

async function toggleAlbumLike(album, button) {
  const next = !album.liked
  button.disabled = true
  try {
    if (next) await endpoints.likeAlbum(album.id)
    else await endpoints.unlikeAlbum(album.id)
    album.liked = next
    button.classList.toggle('liked', next)
    button.setAttribute('aria-pressed', String(next))
    button.setAttribute('aria-label', next ? 'Remover álbum dos salvos' : 'Salvar álbum')
    button.replaceChildren(icon('heart'), document.createTextNode(next ? 'Salvo' : 'Salvar álbum'))
    showToast(next ? 'Álbum salvo na biblioteca' : 'Álbum removido da biblioteca')
  } catch (err) {
    showToast(err.message || 'Não foi possível salvar o álbum')
  } finally {
    button.disabled = false
  }
}

// ---------- Pages ----------

const pages = {
  '/': renderHome,
  '/search': renderSearch,
  '/library': renderLibrary,
  '/recommendations': renderRecommendations,
  '/recommended-artist/:name': renderArtist,
  '/liked': renderLiked,
  '/history': renderHistory,
  '/downloads': renderDownloads,
  '/queue': renderQueue,
  '/settings': renderSettings,
  '/admin': renderAdminPage,
  '/category/:id': renderCategory,
  '/album/:id': renderAlbum,
  '/playlist/:id': renderPlaylist,
  '/song/:id': renderSong,
}

async function renderAdminPage(container) {
  if (!auth.user?.isAdmin) {
    navigate('/')
    return
  }
  container.innerHTML = ''
  container.append(renderAdmin())
}

function matchRoute(path) {
  for (const [pattern, fn] of Object.entries(pages)) {
    if (!pattern.includes(':')) {
      if (pattern === path) return { fn, params: {} }
      continue
    }
    const patParts = pattern.split('/')
    const pathParts = path.split('/')
    if (patParts.length !== pathParts.length) continue
    const params = {}
    let ok = true
    for (let i = 0; i < patParts.length; i++) {
      if (patParts[i].startsWith(':')) params[patParts[i].slice(1)] = decodeURIComponent(pathParts[i])
      else if (patParts[i] !== pathParts[i]) { ok = false; break }
    }
    if (ok) return { fn, params }
  }
  return { fn: renderHome, params: {} }
}

// ---------- Home ----------

async function renderHome(container) {
  container.innerHTML = ''
  container.append(el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, greeting())))
  try {
    const [home, rec, albums] = await Promise.all([
      endpoints.home(),
      endpoints.recommendations().catch(() => ({ songs: [], artists: [], tastes: [] })),
      endpoints.albums().catch(() => []),
    ])
    const sectionsEl = []
    if (rec.songs?.length) sectionsEl.push(section('Para você', rec.songs.slice(0, 10).map((song) => songCard(song, () => player.playContext(rec.songs, rec.songs.indexOf(song))))))
    if (rec.artists?.length) sectionsEl.push(section('Artistas para você', rec.artists.slice(0, 10).map((a) => card({ image: artworkUrl(a.songId, 300), title: a.name, subtitle: 'Artista', onClick: () => navigate(`/recommended-artist/${encodeURIComponent(a.name)}`) }))))
    if (albums.length) sectionsEl.push(section('Álbuns', albums.slice(0, 12).map((album) => albumCard(album, '/'))))
    for (const s of home.sections ?? []) {
      const songs = s.songs ?? []
      if (songs.length > 0) {
        const cards = songs.map((song) =>
          songCard(song, () => player.playContext(songs, songs.indexOf(song))),
        )
        sectionsEl.push(section(s.title, cards))
      }
    }
    const myPlaylists = playlistsCache ?? []
    if (myPlaylists.length > 0) {
      sectionsEl.push(
        section(
          'Playlists',
          myPlaylists.map((p) =>
            card({ image: artworkUrl(p.id, 300), title: p.name, subtitle: musicas(p.songCount), onClick: () => navigate(`/playlist/${p.id}`) }),
          ),
        ),
      )
    }
    container.append(...sectionsEl)
    if (sectionsEl.length === 0) {
      container.append(
        emptyState2(
          'Sua biblioteca está vazia.',
          'O administrador precisa adicionar músicas ao catálogo.',
        ),
      )
    }
  } catch (err) {
    container.append(emptyState(err.message))
  }
}

// ---------- Personalized recommendations ----------
async function renderRecommendations(container) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const rec = await endpoints.recommendations()
    const page = el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, 'Para você'))
    if (rec.tastes?.length) page.append(el('p', { class: 'detail-meta' }, `Seus gostos: ${rec.tastes.join(' • ')}`))
    if (rec.songs?.length) page.append(section(rec.personalized ? 'Músicas para você' : 'Comece a ouvir', rec.songs.map((song) => songCard(song, () => player.playContext(rec.songs, rec.songs.indexOf(song))))))
    if (rec.artists?.length) page.append(section('Artistas para você', rec.artists.map((artist) => card({ image: artworkUrl(artist.songId, 300), title: artist.name, subtitle: 'Artista', onClick: () => navigate(`/recommended-artist/${encodeURIComponent(artist.name)}`) }))))
    if (!rec.songs?.length) page.append(emptyState('Ainda não há músicas no catálogo.'))
    container.innerHTML = ''
    container.append(page)
  } catch (err) { container.innerHTML = ''; container.append(emptyState(err.message)) }
}

async function renderArtist(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const allSongs = await endpoints.songs()
    const songs = allSongs.filter((song) => (song.artist || '').trim().toLocaleLowerCase() === params.name.trim().toLocaleLowerCase())
    const page = el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, params.name))
    if (songs.length) page.append(el('div', { class: 'track-list' }, ...songs.map((song, i) => trackRow(song, i, { onPlay: (_song, index) => player.playContext(songs, index) }))))
    else page.append(emptyState('Nenhuma música disponível.'))
    container.innerHTML = ''
    container.append(page)
  } catch (err) { container.innerHTML = ''; container.append(emptyState(err.message)) }
}

// ---------- Search ----------

// searchSeq guards against stale responses: only the latest request may
// write results (typing fast can leave several requests in flight).
let searchSeq = 0
let searchResults = null
let searchQuery = ''
let searchType = 'all'

// runSearch updates ONLY the #search-results container. It must never call
// render() — rebuilding the page would destroy the input mid-typing and the
// next keystrokes would go nowhere (the "search freezes" bug).
async function runSearch(q) {
  searchQuery = q
  const seq = ++searchSeq
  const resultsEl = document.getElementById('search-results')
  if (!q.trim()) {
    searchResults = null
    if (resultsEl) resultsEl.innerHTML = ''
    return
  }
  if (resultsEl) {
    resultsEl.innerHTML = ''
    resultsEl.append(spinner())
  }
  let results = null
  try {
    results = await endpoints.search(q, searchType)
  } catch {
    results = null
  }
  if (seq !== searchSeq) return
  searchResults = results
  if (resultsEl) renderSearchResults(resultsEl, results)
}

function renderSearchResults(container, results) {
  container.innerHTML = ''
  if (!results) return
  const songs = results.songs ?? []
  const cats = results.categories ?? []
  const pls = results.playlists ?? []
  const all = searchType === 'all'
  if ((all || searchType === 'songs') && songs.length > 0) {
    const rows = songs.map((s, i) =>
      trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }),
    )
    container.append(
      el('div', {},
        el('h2', { class: 'section-title' }, 'Músicas'),
        el('div', { class: 'track-list' }, ...rows),
      ),
    )
  }
  if ((all || searchType === 'categories') && cats.length > 0) {
    container.append(
      section(
        'Categorias',
        cats.map((c) => categoryCard(c)),
      ),
    )
  }
  if ((all || searchType === 'playlists') && pls.length > 0) {
    container.append(
      section(
        'Playlists',
        pls.map((p) =>
          card({ image: artworkUrl(p.id, 300), title: p.name, subtitle: p.owner, onClick: () => navigate(`/playlist/${p.id}`) }),
        ),
      ),
    )
  }
  const albums = results.albums ?? []
  if ((all || searchType === 'albums') && albums.length > 0) {
    container.append(section('Álbuns', albums.map((album) => albumCard(album, '/search'))))
  }
  const total =
    (all || searchType === 'songs' ? songs.length : 0) +
    (all || searchType === 'categories' ? cats.length : 0) +
    (all || searchType === 'playlists' ? pls.length : 0) +
    (all || searchType === 'albums' ? albums.length : 0)
  if (total === 0) {
    container.append(el('p', { class: 'empty-state' }, `Nada encontrado para “${searchQuery}”.`))
  }
}

const searchTypes = [
  { id: 'all', label: 'Tudo' },
  { id: 'songs', label: 'Músicas' },
  { id: 'albums', label: 'Álbuns' },
  { id: 'categories', label: 'Categorias' },
  { id: 'playlists', label: 'Playlists' },
]

async function renderSearch(container) {
  const { params } = parseHash()
  container.innerHTML = ''
  // Preserves the typed text across re-renders (searchQuery) or the genre
  // filter coming from ?genre=.
  const initial = params.get('genre') || searchQuery || ''

  const input = el('input', { class: 'search-input', placeholder: 'O que você quer ouvir?', value: initial })
  let debounce
  input.addEventListener('input', (e) => {
    window.clearTimeout(debounce)
    debounce = window.setTimeout(() => runSearch(e.target.value), 300)
  })

  const typeSelect = el(
    'select',
    {
      class: 'search-type-select',
      'aria-label': 'Tipo de pesquisa',
      onchange: (e) => { searchType = e.target.value; runSearch(input.value) },
    },
    ...searchTypes.map((t) => el('option', { value: t.id }, t.label)),
  )
  typeSelect.value = searchTypes.some((t) => t.id === searchType) ? searchType : 'all'

  const page = el(
    'div',
    { class: 'page-padding' },
    el('div', { class: 'search-bar' },
      el('div', { class: 'search-input-wrap' }, el('span', { class: 'search-icon', html: icons.search }), input),
      typeSelect,
    ),
  )
  const resultsEl = el('div', { id: 'search-results' })
  page.append(resultsEl)
  container.append(page)

  // Render the cached results without a refetch when possible; otherwise
  // fire the search. Never await inside render: the container updates in
  // place and the input keeps focus.
  if (initial) {
    if (searchResults && searchQuery === initial) renderSearchResults(resultsEl, searchResults)
    else void runSearch(initial)
  } else {
    searchQuery = ''
    renderSearchResults(resultsEl, null)
  }
}

// ---------- Library ----------

let libTab = 'songs'
let libData = null
let libLoading = false

async function loadLibrary() {
  if (libLoading) return
  libLoading = true
  const epoch = sessionEpoch
  const [s, p, c, a] = await Promise.allSettled([endpoints.songs(), endpoints.playlists(), endpoints.categories(), endpoints.albums()])
  if (epoch !== sessionEpoch) return
  libData = {
    songs: s.status === 'fulfilled' ? s.value : [],
    playlists: p.status === 'fulfilled' ? p.value : [],
    categories: c.status === 'fulfilled' ? c.value : [],
    albums: a.status === 'fulfilled' ? a.value : [],
  }
  libLoading = false
  render()
}

async function renderLibrary(container) {
  container.innerHTML = ''
  if (!libData) {
    container.append(el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, 'Sua Biblioteca'), spinner()))
    void loadLibrary()
    return
  }
  const tabs = [
    { id: 'songs', label: 'Músicas' },
    { id: 'albums', label: 'Álbuns' },
    { id: 'categories', label: 'Categorias' },
    { id: 'playlists', label: 'Playlists' },
  ]
  const tabsEl = el(
    'div',
    { class: 'tabs' },
    ...tabs.map((t) =>
      el('button', { class: `tab-btn ${libTab === t.id ? 'active' : ''}`, onclick: () => { libTab = t.id; render() } }, t.label),
    ),
  )
  const page = el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, 'Sua Biblioteca'), tabsEl)
  const wrap = el('div', { class: 'card-wrap' })
  page.append(wrap)
  container.append(page)

  if (libTab === 'songs') {
    if (libData.songs.length === 0) {
      wrap.append(el('p', { style: 'color:var(--subtext)' }, 'Nenhuma música na biblioteca.'))
      return
    }
    const rows = libData.songs.map((s, i) =>
      trackRow(s, i, { onPlay: (_song, idx) => player.playContext(libData.songs, idx) }),
    )
    page.append(el('div', { class: 'track-list' }, ...rows))
  } else if (libTab === 'albums') {
    if (libData.albums.length === 0) {
      wrap.append(el('p', { style: 'color:var(--subtext)' }, 'Nenhum álbum disponível.'))
      return
    }
    for (const album of libData.albums) wrap.append(albumCard(album))
  } else if (libTab === 'categories') {
    if (libData.categories.length === 0) {
      wrap.append(el('p', { style: 'color:var(--subtext)' }, 'Nenhuma categoria ainda.'))
      return
    }
    for (const c of libData.categories) {
      wrap.append(categoryCard(c))
    }
  } else {
    if (libData.playlists.length === 0) {
      wrap.append(el('p', { style: 'color:var(--subtext)' }, 'Nenhuma playlist criada.'))
      return
    }
    for (const p of libData.playlists) {
      wrap.append(card({ image: artworkUrl(p.id, 300), title: p.name, subtitle: musicas(p.songCount), onClick: () => navigate(`/playlist/${p.id}`) }))
    }
  }
}

// Permanent, access-controlled share link for a song.
async function renderSong(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const song = await endpoints.song(params.id)
    const header = el('div', { class: 'detail-header horizontal' },
      el('img', { class: 'detail-art', src: artworkUrl(song.id, 480), alt: '', loading: 'lazy' }),
      el('div', { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Música'),
        el('h1', { class: 'detail-title' }, song.title),
        el('p', { class: 'detail-meta' }, song.artist || 'Desconhecido'),
        el('div', { class: 'detail-actions' },
          el('button', { class: 'btn-accent', onclick: () => player.playContext([song], 0) }, icon('play'), 'Tocar'),
          el('button', { class: 'btn-secondary', onclick: () => copySongLink(song.id) }, 'Copiar link'),
        ),
      ),
    )
    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, header))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

// ---------- Category ----------

async function renderCategory(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const cat = await endpoints.category(params.id)
    const songs = cat.songs ?? []

    const header = el(
      'div',
      { class: 'detail-header horizontal' },
      el('img', { class: 'detail-art', src: artworkUrl(cat.id, 480), alt: '', loading: 'lazy' }),
      el(
        'div',
        { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Categoria'),
        el('h1', { class: 'detail-title' }, cat.name),
        el('p', { class: 'detail-meta' }, musicas(songs.length)),
        el(
          'div',
          { class: 'detail-actions' },
          el(
            'button',
            { class: 'btn-accent', disabled: songs.length === 0, onclick: () => player.playContext(songs, 0) },
            icon('play'),
            'Tocar',
          ),
          el('button', { class: 'btn-icon-lg', 'aria-label': 'Aleatório', onclick: () => playShuffle(songs) }, icon('shuffle')),
        ),
      ),
    )

    const content = el('div', { class: 'detail-content' })
    if (songs.length === 0) {
      content.append(emptyState('Esta categoria ainda não tem músicas.'))
    } else {
      const rows = songs.map((s, i) =>
        trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }),
      )
      content.append(el('div', { class: 'track-list' }, ...rows))
    }
    content.append(
      el('button', { class: 'back-link', onclick: () => navigate('/library') }, icon('arrowLeft'), 'Voltar para a biblioteca'),
    )

    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, header, content))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

async function renderAlbum(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  const { params: query } = parseHash()
  const requestedReturn = query.get('from')
  const returnTo = ['/search', '/', '/liked'].includes(requestedReturn) ? requestedReturn : '/library'
  try {
    const album = await endpoints.album(params.id)
    const songs = album.songs ?? []
    const duration = album.duration || songs.reduce((total, song) => total + (Number(song.duration) || 0), 0)
    const meta = [
      album.artist || 'Artista desconhecido',
      album.year > 0 ? album.year : null,
      musicas(songs.length),
      fmtDurationLong(duration),
    ].filter(Boolean).join(' • ')
    const header = el(
      'div',
      { class: 'detail-header horizontal album-detail-header' },
      el('img', { class: 'detail-art', src: artworkUrl(album.id, 480), alt: `Capa do álbum ${album.name}`, loading: 'lazy' }),
      el(
        'div',
        { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Álbum'),
        el('h1', { class: 'detail-title' }, album.name),
        el('p', { class: 'detail-meta' }, meta),
        album.artist
          ? el('button', {
              class: 'album-artist-link',
              onclick: () => navigate(`/recommended-artist/${encodeURIComponent(album.artist)}`),
            }, `Ver artista: ${album.artist}`)
          : null,
        el(
          'div',
          { class: 'detail-actions' },
          el(
            'button',
            { class: 'btn-accent', disabled: songs.length === 0, onclick: () => player.playContext(songs, 0) },
            icon('play'),
            'Tocar álbum',
          ),
          el(
            'button',
            {
              class: 'btn-secondary album-shuffle',
              disabled: songs.length === 0,
              'aria-label': 'Tocar álbum aleatoriamente',
              onclick: () => playShuffle(songs),
            },
            icon('shuffle'),
            'Aleatório',
          ),
          el(
            'button',
            {
              class: `btn-secondary album-save${album.liked ? ' liked' : ''}`,
              'aria-label': album.liked ? 'Remover álbum dos salvos' : 'Salvar álbum',
              'aria-pressed': String(!!album.liked),
              onclick: (event) => toggleAlbumLike(album, event.currentTarget),
            },
            icon('heart'),
            album.liked ? 'Salvo' : 'Salvar álbum',
          ),
        ),
      ),
    )

    const content = el('div', { class: 'detail-content album-detail-content' })
    content.append(el('h2', { class: 'section-title album-tracks-title' }, 'Faixas'))
    if (songs.length === 0) {
      content.append(emptyState('Este álbum ainda não tem músicas disponíveis.'))
    } else {
      const rows = songs.map((song, index) => trackRow(song, index, {
        onPlay: (_selected, selectedIndex) => player.playContext(songs, selectedIndex),
        compactActions: true,
        onQueue: (selected) => {
          player.addToQueue(selected)
          showToast(`“${selected.title}” adicionada à fila`)
        },
      }))
      content.append(el('div', { class: 'track-list album-track-list' }, ...rows))
    }
    content.append(el('button', { class: 'back-link', onclick: () => navigate(returnTo) }, icon('arrowLeft'), 'Voltar'))
    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, header, content))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

function playShuffle(songs) {
  const shuffled = [...songs]
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    const current = shuffled[i]
    shuffled[i] = shuffled[j]
    shuffled[j] = current
  }
  player.playContext(shuffled, 0)
}

// ---------- Playlist ----------

async function renderPlaylist(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const playlist = await endpoints.playlist(params.id)
    const songs = (playlist.songs ?? []).map((ps) => ps.song)

    const header = el(
      'div',
      { class: 'detail-header horizontal' },
      el('img', { class: 'detail-art', src: artworkUrl(playlist.id, 320), alt: '', loading: 'lazy' }),
      el(
        'div',
        { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Playlist'),
        el('h1', { class: 'detail-title' }, playlist.name),
        playlist.comment ? el('p', { class: 'detail-meta' }, playlist.comment) : null,
        el('p', { class: 'detail-meta' }, `${playlist.owner} • ${musicas(playlist.songCount)} • ${fmtDurationLong(playlist.duration)}`),
        el(
          'div',
          { class: 'detail-actions' },
          el(
            'button',
            { class: 'btn-accent', disabled: songs.length === 0, onclick: () => player.playContext(songs, 0) },
            icon('play'),
            'Tocar',
          ),
        ),
      ),
    )

    const content = el('div', { class: 'detail-content' })
    if ((playlist.songs ?? []).length === 0) {
      content.append(
        emptyState2(
          'Esta playlist está vazia.',
          'Crie uma playlist pela página de Configurações e adicione músicas.',
        ),
      )
    } else {
      const rows = playlist.songs.map((ps, i) => {
        const removeBtn = el(
          'button',
          { class: 'remove-track-btn', 'aria-label': 'Remover da playlist', onclick: () => removePlaylistTrack(playlist.id, ps.entryId) },
          icon('trash'),
        )
        return el(
          'div',
          { class: 'playlist-track-row' },
          el('div', { style: 'flex:1;min-width:0' }, trackRow(ps.song, i, { onPlay: (_song, idx) => player.playContext(songs, idx) })),
          removeBtn,
        )
      })
      content.append(el('div', { class: 'track-list' }, ...rows))
    }
    content.append(
      el('button', { class: 'back-link', onclick: () => navigate('/library') }, icon('arrowLeft'), 'Voltar para a biblioteca'),
    )

    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, header, content))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

async function removePlaylistTrack(id, entryId) {
  try {
    await endpoints.removePlaylistTrack(id, entryId)
    render()
  } catch (err) {
    // Falha do DELETE: a lista permanece intacta (nenhum render) e o erro
    // vira feedback visível em vez de unhandled promise rejection.
    alert(err.message)
  }
}

// ---------- Liked ----------

async function renderLiked(container) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const [songs, albums] = await Promise.all([endpoints.liked(), endpoints.likedAlbums()])
    const header = el(
      'div',
      { class: 'detail-header' },
      el('div', { class: 'detail-art-icon' }, '♥'),
      el(
        'div',
        { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Playlist'),
        el('h1', { class: 'detail-title' }, 'Curtidas'),
        el('p', { class: 'detail-meta' }, `${musicas(songs.length)} • ${albums.length} ${albums.length === 1 ? 'álbum salvo' : 'álbuns salvos'}`),
        el(
          'div',
          { class: 'detail-actions' },
          el(
            'button',
            { class: 'btn-accent', disabled: songs.length === 0, onclick: () => player.playContext(songs, 0) },
            icon('play'),
            'Tocar',
          ),
        ),
      ),
    )
    const content = el('div', { class: 'detail-content' })
    if (songs.length === 0 && albums.length === 0) {
      content.append(emptyState2('Nenhuma música curtida ainda.', 'Toque no coração de uma música para salvá-la aqui.'))
    } else {
      if (songs.length) {
        content.append(el('h2', { class: 'section-title liked-section-title' }, 'Músicas'))
        const rows = songs.map((s, i) => trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }))
        content.append(el('div', { class: 'track-list' }, ...rows))
      }
      if (albums.length) {
        content.append(el('h2', { class: 'section-title liked-section-title' }, 'Álbuns'))
        content.append(el('div', { class: 'card-wrap' }, ...albums.map((album) => albumCard(album, '/liked'))))
      }
    }
    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, header, content))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

// ---------- History ----------

async function renderHistory(container) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const songs = await endpoints.history()
    const page = el(
      'div',
      { class: 'page-padding' },
      el('h1', { class: 'page-title' }, 'Histórico'),
      el('p', { class: 'detail-meta' }, 'Músicas que você tocou recentemente'),
    )
    if (songs.length === 0) {
      page.append(emptyState('Nenhuma música tocada ainda.'))
    } else {
      const rows = songs.map((s, i) => trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }))
      page.append(el('div', { class: 'track-list' }, ...rows))
    }
    container.innerHTML = ''
    container.append(el('div', { class: 'page' }, page))
  } catch (err) {
    container.innerHTML = ''
    container.append(emptyState(err.message))
  }
}

async function renderDownloads(container) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const entries = await endpoints.downloads()
    const page = el('div', { class: 'page-padding' }, el('h1', { class: 'page-title' }, 'Downloads'), el('p', { class: 'detail-meta' }, 'Downloads iniciados nesta conta'))
    if (!entries.length) page.append(emptyState('Nenhum download iniciado ainda.'))
    else page.append(el('div', { class: 'track-list' }, ...entries.map(entry => el('div', { class: 'download-history-row' },
      el('div', {}, el('p', { class: 'track-title' }, entry.title), el('p', { class: 'track-artist' }, `${entry.artist || 'Desconhecido'} · ${new Date(entry.downloadedAt).toLocaleString('pt-BR')}`)), entry.id ? downloadButton(entry) : el('span', { class: 'track-artist' }, 'Indisponível')))))
    container.innerHTML = ''; container.append(el('div', { class: 'page' }, page))
  } catch (err) { container.innerHTML = ''; container.append(emptyState(err.message)) }
}

// ---------- Queue ----------

function renderQueue(container) {
  const { queue, currentIndex } = player.getPlayerState()
  const page = el(
    'div',
    { class: 'page-padding' },
    el('h1', { class: 'page-title' }, 'Fila'),
    el('p', { class: 'detail-meta' }, 'Próximas músicas na reprodução'),
  )
  if (queue.length === 0) {
    page.append(emptyState('A fila está vazia. Toque uma música para começar.'))
  } else {
    const rows = queue.map((s, i) => {
      const row = trackRow(s, i, { onPlay: (_song, idx) => player.playContext(queue, idx) })
      if (i === currentIndex) {
        return el(
          'div',
          { class: 'queue-track-wrap' },
          el('span', { class: 'queue-current-marker' }, el('span', { style: 'display:block;width:4px;height:12px;border-radius:2px;background:var(--accent)' })),
          row,
        )
      }
      return row
    })
    page.append(el('div', { class: 'track-list' }, ...rows))
  }
  container.innerHTML = ''
  container.append(el('div', { class: 'page' }, page))
}

// ---------- Settings ----------

// Avoid leaving the activation screen stuck if registration fails.
async function pushServiceWorker() {
  let timer
  try {
    return await Promise.race([
      navigator.serviceWorker.ready,
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('O aplicativo não ficou pronto. Atualize a página e tente novamente.')), 8000) }),
    ])
  } finally { clearTimeout(timer) }
}

// Serialize device operations so an opt-out always runs after an in-flight update.
let pushOperation = Promise.resolve()
function queuePushOperation(action) {
  const next = pushOperation.catch(() => undefined).then(() =>
    navigator.locks ? navigator.locks.request('pm-push-device', action) : action())
  pushOperation = next.catch(() => undefined)
  return next
}
function pushStateKey(userId) { return `pm_push_state:${userId}` }
function readPushState(userId) {
  try { return JSON.parse(localStorage.getItem(pushStateKey(userId)) || 'null') || { enabled: false } }
  catch { return { enabled: false } }
}
function writePushState(userId, state) { localStorage.setItem(pushStateKey(userId), JSON.stringify(state)) }
function ensurePushAccount(userId) {
  let tokenUser = null
  try {
    const part = getToken().split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    tokenUser = JSON.parse(atob(part.padEnd(Math.ceil(part.length / 4) * 4, '='))).uid
  } catch { /* A missing or malformed token cannot authorize this operation. */ }
  if (auth.user?.id !== userId || tokenUser !== userId) throw new Error('A conta mudou. Entre novamente para concluir a operação.')
}

async function pushRequest(path, method = 'GET', body) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 8000)
  try {
    return await api.fetch(path, { method, signal: controller.signal, ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }) })
  } finally { clearTimeout(timer) }
}
async function removePushEndpoint(endpoint) {
  await pushRequest('/api/push/subscriptions', 'DELETE', { endpoint })
}
async function reconcilePushSubscription(userId) {
  return queuePushOperation(async () => {
    let state = readPushState(userId)
    const owner = localStorage.getItem('pm_push_owner')
    if (owner !== userId) return
    if (!navigator.locks && state.enabled) return
    if (!('serviceWorker' in navigator) || !('Notification' in window)) return
    try {
      const registration = await pushServiceWorker()
      const current = await registration.pushManager.getSubscription()
      ensurePushAccount(userId)
      state = readPushState(userId)
      if (!state.enabled) {
        if (localStorage.getItem('pm_push_owner') !== userId) return
        const endpoints = [...new Set([state.endpoint, state.pendingEndpoint].filter(Boolean))]
        for (const endpoint of endpoints) { await removePushEndpoint(endpoint); ensurePushAccount(userId) }
        if (current) await current.unsubscribe()
        ensurePushAccount(userId)
        writePushState(userId, { enabled: false })
        localStorage.removeItem('pm_push_owner')
        return
      }
      if (Notification.permission !== 'granted' || !current) {
        // Keep the old endpoint until the server confirms revocation.
        for (const endpoint of new Set([state.endpoint, state.pendingEndpoint].filter(Boolean))) { await removePushEndpoint(endpoint); ensurePushAccount(userId) }
        ensurePushAccount(userId)
        state = readPushState(userId)
        if (state.enabled) {
          writePushState(userId, { enabled: false })
          if (localStorage.getItem('pm_push_owner') === userId) localStorage.removeItem('pm_push_owner')
        }
        return
      }
      if (current.endpoint === state.endpoint && !state.pendingEndpoint) return
      // Save both endpoints before any request; failures are retried on next login.
      state.pendingEndpoint = current.endpoint
      writePushState(userId, state)
      if (state.endpoint && state.endpoint !== current.endpoint) {
        await removePushEndpoint(state.endpoint)
        ensurePushAccount(userId)
      }
      if (!readPushState(userId).enabled) return
      await pushRequest('/api/push/subscriptions', 'POST', current.toJSON())
      ensurePushAccount(userId)
      state = readPushState(userId)
      if (state.enabled) writePushState(userId, { enabled: true, endpoint: current.endpoint })
    } catch { /* Keep the pending state for retry; do not request permission. */ }
  })
}

async function verifyNotificationGate(userId) {
  if (auth.user?.id !== userId) return
  const seq = ++notificationGateSeq
  notificationGate = { userId, status: 'checking' }
  if (auth.user?.id === userId) render()
  let allowed = false
  try {
    ensurePushAccount(userId)
    if (!window.isSecureContext || !('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window) || !navigator.locks || Notification.permission !== 'granted') throw new Error('Permissão necessária')
    const state = readPushState(userId)
    if (!state.enabled || !state.endpoint || state.pendingEndpoint || localStorage.getItem('pm_push_owner') !== userId) throw new Error('Inscrição necessária')
    const registration = await pushServiceWorker()
    const subscription = await registration.pushManager.getSubscription()
    ensurePushAccount(userId)
    if (!subscription || subscription.endpoint !== state.endpoint) throw new Error('Inscrição alterada')
    const result = await pushRequest('/api/push/status', 'POST', { endpoint: subscription.endpoint })
    ensurePushAccount(userId)
    allowed = result.active === true && Notification.permission === 'granted' && readPushState(userId).enabled === true
  } catch { /* Stay on the activation screen until verification succeeds. */ }
  if (seq !== notificationGateSeq || auth.user?.id !== userId) return
  notificationGate = { userId, status: allowed ? 'allowed' : 'required' }
  if (!allowed && !auth.user.isAdmin) { notificationGateNeedsRedirect = true; player.clearSession() }
  if (allowed && notificationGateNeedsRedirect) {
    notificationGateNeedsRedirect = false
    if (window.location.hash !== '#/') navigate('/')
  }
  render()
  if (allowed) void loadPlaylists()
}
function installButton() {
  if (window.isPlayMusicInstalled?.()) return null
  return el('button', { class: 'install-app-btn', type: 'button', onclick: () => window.installPlayMusic?.() }, 'Instalar aplicativo')
}
function renderNotificationGate() {
  document.querySelector('.pwa-overlay')?.remove()
  root.innerHTML = ''
  const checking = notificationGate.status === 'checking'
  const bell = el('span', { class: 'notification-gate-icon', 'aria-hidden': 'true', html: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 20a2 2 0 0 0 4 0"/></svg>' })
  root.append(el('main', { class: 'notification-gate' },
    el('div', { class: 'notification-gate-brand' }, icon('music'), el('strong', {}, 'Mi Ritmo')),
    el('section', { class: 'notification-gate-content', 'aria-labelledby': 'notification-gate-title' },
      bell,
      el('p', { class: 'notification-gate-eyebrow' }, 'FIQUE POR DENTRO'),
      el('h1', { id: 'notification-gate-title' }, checking ? 'Verificando notificações' : 'Ative as notificações'),
      el('p', { class: 'notification-gate-description' }, checking ? 'Estamos preparando seu acesso.' : 'Permita as notificações para continuar e receber avisos e novidades do Mi Ritmo.'),
      checking ? spinner('Só um instante…') : notificationSettingsCard(true))))
}

document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && auth.user) void verifyNotificationGate(auth.user.id)
})
window.addEventListener('appinstalled', () => render())

function notificationSettingsCard(required = false) {
  const userId = auth.user?.id
  const pushState = readPushState(userId)
  const active = !required && notificationGate.userId === userId && notificationGate.status === 'allowed' && window.isSecureContext && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window && !!navigator.locks && Notification.permission === 'granted' && localStorage.getItem('pm_push_owner') === userId && pushState.enabled === true && !!pushState.endpoint && !pushState.pendingEndpoint
  const status = el('p', { class: required ? 'notification-gate-status' : 'settings-text', role: 'status', 'aria-live': 'polite' }, required ? '' : active ? 'As notificações já estão ativadas neste dispositivo.' : 'Ative para receber avisos e promoções neste dispositivo.')
  const enable = el('button', { class: 'btn-accent', type: 'button' }, 'Ativar notificações')
  const disable = el('button', { class: 'btn-secondary', type: 'button' }, 'Desativar notificações')
  if (!('serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window && window.isSecureContext)) {
    const ios = /iP(hone|od|ad)/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
    status.textContent = ios ? 'No iPhone ou iPad, abra no Safari, toque em Compartilhar e em Adicionar à Tela de Início. Depois, abra o aplicativo instalado.' : 'As notificações não estão disponíveis neste navegador. Abra o aplicativo em um navegador compatível.'
    enable.disabled = true; disable.disabled = true
  }
  else if (!navigator.locks) { status.textContent = 'Atualize o navegador para ativar notificações com segurança.'; enable.disabled = true }
  let config = null
  if (!enable.disabled && !active) {
    enable.disabled = true
    pushRequest('/api/push/config').then(value => { config = value; enable.disabled = false; if (!value.enabled) status.textContent = 'Notificações indisponíveis. Toque para tentar novamente.' }).catch(() => { enable.disabled = false; status.textContent = 'Não foi possível verificar as notificações. Toque para tentar novamente.' })
  }
  enable.addEventListener('click', async () => {
    const intent = `${Date.now()}-${Math.random()}`
    try {
      ensurePushAccount(userId)
      if (!config?.enabled) {
        config = await pushRequest('/api/push/config')
        ensurePushAccount(userId)
        status.textContent = config.enabled ? 'Tudo pronto. Toque novamente para permitir.' : 'Notificações indisponíveis. Tente novamente em instantes.'
        return
      }
      writePushState(userId, { ...readPushState(userId), enabled: true, intent })
      // This call must remain directly in the user's click for iOS.
      const permission = await Notification.requestPermission()
      ensurePushAccount(userId)
      if (readPushState(userId).intent !== intent || !readPushState(userId).enabled) return
      if (permission !== 'granted') { writePushState(userId, { ...readPushState(userId), enabled: false }); throw new Error('Permissão não concedida. Verifique as configurações do navegador.') }
      await queuePushOperation(async () => {
        ensurePushAccount(userId)
        if (!readPushState(userId).enabled || readPushState(userId).intent !== intent) return
        const owner = localStorage.getItem('pm_push_owner')
        if (owner && owner !== userId) throw new Error('Este dispositivo está vinculado a outra conta. Entre nela e desative as notificações primeiro.')
        localStorage.setItem('pm_push_owner', userId)
        const reg = await pushServiceWorker()
        let sub = await reg.pushManager.getSubscription()
        ensurePushAccount(userId)
        if (!readPushState(userId).enabled) return
        if (!sub) {
          const raw = atob(config.publicKey.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(config.publicKey.length / 4) * 4, '='))
          sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: Uint8Array.from(raw, c => c.charCodeAt(0)) })
        }
        ensurePushAccount(userId)
        let next = readPushState(userId)
        // Record the endpoint even if opt-out arrived while subscribe was in flight.
        next.pendingEndpoint = sub.endpoint
        writePushState(userId, next)
        if (!next.enabled) return
        if (next.endpoint && next.endpoint !== sub.endpoint) { await removePushEndpoint(next.endpoint); ensurePushAccount(userId) }
        if (!readPushState(userId).enabled) return
        await pushRequest('/api/push/subscriptions', 'POST', sub.toJSON())
        ensurePushAccount(userId)
        next = readPushState(userId)
        if (next.enabled) writePushState(userId, { enabled: true, endpoint: sub.endpoint })
      })
      status.textContent = readPushState(userId).enabled ? 'Notificações ativadas neste dispositivo.' : 'Cancelamento em andamento.'
      if (typeof verifyNotificationGate === 'function') void verifyNotificationGate(userId)
    } catch (err) { status.textContent = err.message || 'Não foi possível ativar notificações.' }
  })
  disable.addEventListener('click', async () => {
    try {
      ensurePushAccount(userId)
      const owner = localStorage.getItem('pm_push_owner')
      if (owner && owner !== userId) throw new Error('Este dispositivo está vinculado a outra conta. Entre nela para desativar as notificações.')
      // Record opt-out synchronously, before waiting for any earlier network call.
      writePushState(userId, { ...readPushState(userId), enabled: false, intent: '' })
      if (typeof verifyNotificationGate === 'function') void verifyNotificationGate(userId)
      await queuePushOperation(async () => {
        ensurePushAccount(userId)
        const owner = localStorage.getItem('pm_push_owner')
        if (owner && owner !== userId) throw new Error('Este dispositivo está vinculado a outra conta. Entre nela para desativar as notificações.')
        const state = readPushState(userId)
        if (owner !== userId && !state.endpoint && !state.pendingEndpoint) throw new Error('Não foi possível confirmar a conta desta inscrição. Entre na conta que ativou as notificações.')
        const reg = await pushServiceWorker()
        const sub = await reg.pushManager.getSubscription()
        ensurePushAccount(userId)
        const endpoints = [...new Set([state.endpoint, state.pendingEndpoint, sub?.endpoint].filter(Boolean))]
        for (const endpoint of endpoints) { await removePushEndpoint(endpoint); ensurePushAccount(userId) }
        if (sub) await sub.unsubscribe()
        ensurePushAccount(userId)
        writePushState(userId, { enabled: false })
        if (localStorage.getItem('pm_push_owner') === userId) localStorage.removeItem('pm_push_owner')
      })
      status.textContent = 'Notificações desativadas neste dispositivo.'
    } catch (err) { status.textContent = err.message || 'Não foi possível desativar notificações. Tente novamente.' }
  })
  if (required) return el('div', { class: 'notification-gate-action' }, enable, status)
  return el('section', { class: 'settings-card' }, el('h2', {}, 'Notificações'), status, el('div', { class: 'settings-actions' }, active ? null : enable, disable))
}

async function renderSettings(container) {
  container.innerHTML = ''
  container.append(spinner())
  let settings = null
  try {
    settings = await endpoints.settings()
  } catch {
    /* ignore */
  }

  // Clients: the account page only shows the phone and the playlists.
  if (!auth.user?.isAdmin) {
    const phoneCard = el(
      'section',
      { class: 'settings-card' },
      el('h2', {}, 'Conta'),
      el('p', { class: 'settings-text' },
        'Telefone: ',
        el('strong', {}, phoneMask(auth.user?.phone ?? '')),
      ),
    )
    const playlistsCard = el('section', { class: 'settings-card' }, el('h2', {}, 'Playlists'))
    const pl = playlistsCache ?? []
    if (pl.length === 0) {
      playlistsCard.append(el('p', { class: 'settings-text' }, 'Nenhuma playlist ainda.'))
    } else {
      const list = el('div', { class: 'settings-playlists' })
      for (const p of pl) {
        list.append(
          el(
            'button',
            { class: 'settings-playlist-item', onclick: () => navigate(`/playlist/${p.id}`) },
            icon('list'),
            el('span', { style: 'flex:1;min-width:0;text-align:left' }, p.name),
            el('span', { style: 'color:var(--faint);font-size:12px' }, musicas(p.songCount)),
          ),
        )
      }
      playlistsCard.append(list)
    }
    playlistsCard.append(
      el(
        'div',
        { class: 'settings-actions' },
        el('button', { class: 'btn-accent', onclick: () => createPlaylist() }, icon('plus'), 'Nova playlist'),
        el('button', { class: 'btn-secondary', onclick: doLogout }, icon('logout'), 'Sair'),
      ),
    )
    container.innerHTML = ''
    container.append(
      el('div', { class: 'settings-page' },
        el('h1', { class: 'page-title' }, 'Minha Conta'),
        phoneCard,
        notificationSettingsCard(),
        playlistsCard,
      ),
    )
    return
  }

  const accountCard = el(
    'section',
    { class: 'settings-card' },
    el('h2', {}, 'Conta'),
    el('p', { class: 'settings-text' },
      'Conectado como ',
      el('strong', {}, auth.user?.name ?? ''),
      ` (${auth.user?.username ? '@' + auth.user.username : (auth.user?.phone ? phoneMask(auth.user.phone) : '')})`,
    ),
    el(
      'div',
      { class: 'settings-actions' },
      el('button', { class: 'btn-accent', onclick: () => createPlaylist() }, icon('plus'), 'Nova playlist'),
      el('button', { class: 'btn-secondary', onclick: doLogout }, icon('logout'), 'Sair'),
    ),
  )

  const serverCard = el('section', { class: 'settings-card' }, el('h2', {}, 'Servidor'))
  if (!settings) {
    serverCard.append(spinner())
  } else {
    serverCard.append(
      el(
        'dl',
        { class: 'settings-dl' },
        el('div', {}, el('dt', { class: 'settings-dt' }, 'Servidor'), el('dd', { class: 'settings-dd' }, settings.appName)),
        el('div', {}, el('dt', { class: 'settings-dt' }, 'Versão'), el('dd', { class: 'settings-dd' }, settings.version)),
        el('div', {}, el('dt', { class: 'settings-dt' }, 'Biblioteca'), el('dd', { class: 'settings-dd' }, settings.libraryName)),
      ),
    )
  }

  container.innerHTML = ''
  container.append(
    el('div', { class: 'settings-page' },
      el('h1', { class: 'page-title' }, 'Configurações'),
      accountCard,
      notificationSettingsCard(),
      serverCard,
    ),
  )
}

async function createPlaylist() {
  const name = window.prompt('Nome da nova playlist:')
  if (!name?.trim()) return
  try {
    await endpoints.createPlaylist(name.trim(), [])
    window.dispatchEvent(new Event('pm:playlists-changed'))
    render()
  } catch (err) {
    alert(err.message)
  }
}

// ---------- Login ----------

let loginMode = 'client' // client, register

function renderLogin(container) {
  const toggle = el(
    'div',
    { class: 'login-toggle' },
    el('button', { class: `login-toggle-btn ${loginMode === 'client' ? 'active' : ''}`, 'aria-pressed': loginMode === 'client' ? 'true' : 'false', onclick: () => { loginMode = 'client'; render() } }, 'Cliente'),
    el('button', { class: `login-toggle-btn ${loginMode === 'register' ? 'active' : ''}`, 'aria-pressed': loginMode === 'register' ? 'true' : 'false', onclick: () => { loginMode = 'register'; render() } }, 'Criar conta'),
  )

  const phoneInput = el('input', { class: 'form-input', type: 'tel', inputmode: 'numeric', placeholder: 'Telefone (99) 99999-9999', autofocus: true, autocomplete: 'tel', required: true })
  phoneInput.addEventListener('input', () => applyPhoneMask(phoneInput))
  const errorEl = el('p', { class: 'login-error' })
  const submitBtn = el('button', { class: 'login-submit', type: 'submit' }, loginMode === 'register' ? 'Criar conta' : 'Entrar')

  const form = el(
    'form',
    {
      class: 'login-form',
      onsubmit: async (e) => {
        e.preventDefault()
        errorEl.textContent = ''
        if (!phoneInput.value.trim()) {
          errorEl.textContent = 'Informe seu telefone.'
          return
        }
        submitBtn.disabled = true
        submitBtn.textContent = 'Aguarde…'
        try {
          if (loginMode === 'client') await doLogin('client', phoneInput.value.trim())
          else if (loginMode === 'register') {
            const user = await endpoints.register(phoneInput.value.trim())
            setToken(user.token)
            setAuth({ user, loading: false })
          }
          if (!parseHash().path.startsWith('/song/')) navigate('/')
        } catch (err) {
          errorEl.textContent = err instanceof Error ? err.message : 'Erro ao entrar'
        } finally {
          submitBtn.disabled = false
          submitBtn.textContent = 'Entrar'
        }
      },
    },
    phoneInput,
    errorEl,
    submitBtn,
  )

  container.innerHTML = ''
  container.append(
    el(
      'div',
      { class: 'login-screen' },
      el(
        'div',
        { class: 'login-box' },
        toggle,
        el(
          'div',
          { class: 'login-brand' },
          icon('music'),
          el('h1', {}, 'Mi Ritmo'),
          el('p', {}, loginMode === 'register' ? 'Crie sua conta gratuita' : 'Entre com seu telefone para ouvir'),
        ),
        form,
        el('a', { class: 'login-admin-link', href: '/admin/login' }, 'Acesso administrativo'),
        el('a', { href: '/docs/', class: 'settings-text' }, 'Documentação da API'),
      ),
    ),
  )
}

function renderAdminLogin(container) {
  const usernameInput = el('input', { class: 'form-input', type: 'text', name: 'username', placeholder: 'Usuário ou e-mail', autofocus: true, autocomplete: 'username', required: true })
  const passwordInput = el('input', { class: 'form-input', type: 'password', name: 'password', placeholder: 'Senha', autocomplete: 'current-password', required: true })
  const errorEl = el('p', { class: 'login-error', role: 'alert', 'aria-live': 'polite' })
  const submitBtn = el('button', { class: 'login-submit', type: 'submit' }, 'Entrar no painel')
  const form = el('form', {
    class: 'login-form admin-login-form',
    onsubmit: async (event) => {
      event.preventDefault()
      errorEl.textContent = ''
      if (!usernameInput.value.trim() || !passwordInput.value) {
        errorEl.textContent = 'Informe seu usuário e senha.'
        return
      }
      submitBtn.disabled = true
      submitBtn.textContent = 'Verificando…'
      try {
        await doLogin('admin', usernameInput.value.trim(), passwordInput.value)
        window.location.replace('/#/admin')
      } catch (err) {
        errorEl.textContent = err instanceof Error ? err.message : 'Não foi possível entrar.'
      } finally {
        submitBtn.disabled = false
        submitBtn.textContent = 'Entrar no painel'
      }
    },
  }, usernameInput, passwordInput, errorEl, submitBtn)

  container.innerHTML = ''
  container.append(el('div', { class: 'login-screen admin-login-screen' },
    el('div', { class: 'login-box admin-login-box' },
      el('div', { class: 'login-brand' }, icon('settings'), el('h1', {}, 'Painel administrativo'), el('p', {}, 'Acesso restrito à equipe autorizada')),
      form,
      el('a', { class: 'login-admin-link admin-login-back', href: '/' }, 'Voltar ao acesso dos clientes'),
    ),
  ))
}

// ---------- Sidebar ----------

let playlistsCache = []
let menuOpen = false

async function loadPlaylists() {
  if (!getToken() || !auth.user) return
  const epoch = sessionEpoch
  let result = []
  try { result = (await endpoints.playlists()) ?? [] } catch { /* show an empty list */ }
  if (epoch !== sessionEpoch) return
  playlistsCache = result
  if (auth.user) render()
}

function sidebarContent(onNavigate) {
  const { path } = parseHash()
  const isActive = (p) => (p === '/' ? path === '/' : path.startsWith(p))

  const nav = (to, label, ic) =>
    el(
      'button',
      { class: `nav-link ${isActive(to) ? 'active' : ''}`, onclick: () => { onNavigate?.(); navigate(to) } },
      icon(ic),
      label,
    )

  return el(
    'div',
    { style: 'display:flex;flex-direction:column;height:100%' },
    el(
      'div',
      { class: 'sidebar-header' },
      el('span', { class: 'sidebar-brand' }, icon('music'), 'Mi Ritmo'),
      el('button', { class: 'sidebar-close', 'aria-label': 'Fechar', onclick: onNavigate }, icon('x')),
    ),
    el(
      'nav',
      { class: 'sidebar-nav' },
      nav('/', 'Início', 'home'),
      nav('/search', 'Buscar', 'search'),
      nav('/library', 'Sua Biblioteca', 'library'),
      nav('/recommendations', 'Para você', 'heart'),
      nav('/liked', 'Curtidas', 'heart'),
      nav('/history', 'Histórico', 'clock'),
      nav('/downloads', 'Downloads', 'download'),
      auth.user?.isAdmin ? nav('/admin', 'Administração', 'settings') : null,
    ),
    el('div', { class: 'sidebar-playlists-label' }, el('span', {}, 'Playlists'), el('span', { html: icons.list })),
    el(
      'div',
      { class: 'sidebar-playlists' },
      (playlistsCache ?? []).length === 0
        ? el('p', { class: 'playlists-empty' }, 'Nenhuma playlist ainda')
        : playlistsCache.map((pl) =>
            el(
              'button',
              { class: `playlist-link ${path === `/playlist/${pl.id}` ? 'active' : ''}`, onclick: () => { onNavigate?.(); navigate(`/playlist/${pl.id}`) } },
              pl.name,
            ),
          ),
    ),
    el(
      'div',
      { class: 'sidebar-footer' },
      el(
        'div',
        { class: 'sidebar-user' },
        el('div', { class: 'sidebar-user-info' },
          el('p', { class: 'sidebar-user-name' }, auth.user?.name ?? auth.user?.username ?? ''),
          el('p', { class: 'sidebar-user-handle' }, auth.user?.username ? `@${auth.user.username}` : (auth.user?.phone ? phoneMask(auth.user.phone) : '')),
        ),
        el('div', { style: 'display:flex;gap:4px' },
          el('button', { class: 'icon-btn', 'aria-label': 'Configurações', onclick: () => { onNavigate?.(); navigate('/settings') } }, icon('settings')),
          el('button', { class: 'icon-btn', 'aria-label': 'Sair', onclick: () => { onNavigate?.(); doLogout() } }, icon('logout')),
        ),
      ),
    ),
  )
}

// Compact navigation keeps the most used destinations one tap away on phones.
function mobileNavigation() {
  const { path } = parseHash()
  const destinations = [
    ['/', 'Início', 'home'], ['/search', 'Buscar', 'search'],
    ['/library', 'Biblioteca', 'library'], ['/recommendations', 'Para você', 'heart'],
  ]
  return el('nav', { class: 'mobile-nav', 'aria-label': 'Navegação principal' },
    ...destinations.map(([to, label, ic]) => el('button', {
      class: `mobile-nav-item ${path === to ? 'active' : ''}`,
      'aria-current': path === to ? 'page' : null,
      onclick: () => navigate(to),
    }, icon(ic), el('span', {}, label))),
  )
}

// ---------- Bottom bar ----------

// refs holds live element references so progress/volume updates can mutate
// the DOM in place instead of rebuilding the whole bar (rebuilding every
// 50ms was destroying the seek track mid-gesture and breaking clicks/drags).
function bottomBar(refs) {
  const { current, playing, progress, duration, volume, shuffle, repeat } = player.getPlayerState()
  if (!current) return null

  const totalDuration = player.resolveDuration(duration, current.duration)
  const safeProgress = Number.isFinite(progress) ? Math.min(Math.max(progress, 0), totalDuration) : 0
  const progressPercent = totalDuration > 0 ? (safeProgress / totalDuration) * 100 : 0

  const progressFill = el('div', { class: 'progress-fill', style: `width:${progressPercent}%` })
  const curTime = el('span', { class: 'progress-time' }, fmtDuration(safeProgress))
  const durTime = el('span', { class: 'progress-time' }, fmtDuration(totalDuration))
  refs.fill = progressFill
  refs.cur = curTime
  refs.dur = durTime
  const progressTrack = el(
    'div',
    { class: 'progress-track', role: 'slider', 'aria-valuemin': 0, 'aria-valuemax': totalDuration, 'aria-valuenow': safeProgress },
    progressFill,
  )
  refs.track = progressTrack

  const likeBtn = el(
    'button',
    { class: `icon-btn ${current.liked ? 'liked' : ''}`, style: current.liked ? 'color:var(--accent)' : '', 'aria-label': current.liked ? 'Descurtir' : 'Curtir', 'aria-pressed': current.liked ? 'true' : 'false', onclick: (e) => { e.stopPropagation(); toggleLikeCurrent() } },
    icon('heart'),
  )

  const volIcon = el('span', { class: 'vol-icon' }, volume === 0 ? icon('volumeX') : icon('volume'))
  refs.volIcon = volIcon
  const volInput = el('input', {
    class: 'volume-slider',
    type: 'range',
    min: 0,
    max: 1,
    step: 0.01,
    value: volume,
    'aria-label': 'Volume',
    oninput: (e) => player.setVolume(parseFloat(e.target.value)),
  })
  refs.volInput = volInput

  const openFullscreen = () => {
    fullscreenReturnTarget = document.activeElement
    player.setFullScreen(true)
    scheduleFullscreenFocus(true)
  }

  return el(
    'div',
    { class: 'bottom-bar' },
    el(
      'div',
      { class: 'now-playing' },
      el('button', { class: 'now-playing-open', type: 'button', 'aria-label': `Abrir player: ${current.title}`, onclick: openFullscreen },
        el('img', { class: 'now-playing-art', src: artworkUrl(current.id, 64), alt: '' }),
        el('div', { class: 'now-playing-info' },
          el('p', { class: 'now-playing-title' }, current.title),
          el('p', { class: 'now-playing-artist' }, current.artist || 'Desconhecido'),
        ),
      ),
      likeBtn,
      downloadButton(current),
    ),
    el(
      'div',
      { class: 'player-controls' },
      el(
        'div',
        { class: 'player-buttons' },
        el('button', { class: `player-btn ${shuffle ? 'active' : ''}`, 'aria-label': 'Aleatório', 'aria-pressed': shuffle ? 'true' : 'false', onclick: player.toggleShuffle }, icon('shuffle')),
        el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: player.prev }, icon('prev')),
        el('button', { class: 'player-btn', 'aria-label': 'Retroceder 5 segundos', onclick: () => player.seekBy(-5) }, icon('rewind5')),
        el('button', { class: 'player-btn-main', 'aria-label': playing ? 'Pausar' : 'Tocar', 'aria-pressed': playing ? 'true' : 'false', onclick: player.togglePlay }, playing ? icon('pause') : icon('play')),
        el('button', { class: 'player-btn', 'aria-label': 'Avançar 5 segundos', onclick: () => player.seekBy(5) }, icon('forward5')),
        el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: player.next }, icon('next')),
        el('button', { class: `player-btn ${repeat ? 'active' : ''}`, 'aria-label': 'Repetir', 'aria-pressed': repeat ? 'true' : 'false', onclick: player.toggleRepeat }, icon('repeat')),
      ),
      el(
        'div',
        { class: 'player-progress' },
        curTime,
        progressTrack,
        durTime,
      ),
    ),
    el(
      'div',
      { class: 'player-volume' },
      volIcon,
      volInput,
      el('button', { class: 'icon-btn', 'aria-label': 'Fila', onclick: () => navigate('/queue') }, icon('list')),
      el('button', { class: 'icon-btn', 'aria-label': 'Tela cheia', onclick: openFullscreen }, icon('max')),
    ),
  )
}

// Delegated seek: pointerdown/move/up on the document. Works on click, drag
// and touch, and survives the bar being rebuilt (listeners live on
// document, not on the ephemeral track element).
let seekDrag = null

function seekToClientX(clientX, track) {
  if (!track) return
  const rect = track.getBoundingClientRect()
  if (rect.width === 0) return
  const st = player.getPlayerState()
  const totalDuration = player.resolveDuration(st.duration, st.current?.duration ?? 0)
  if (totalDuration === 0) return
  const ratio = Math.min(Math.max((clientX - rect.left) / rect.width, 0), 1)
  player.seek(ratio * totalDuration)
  updateBarInPlace()
}

document.addEventListener('pointerdown', (e) => {
  if (e.pointerType === 'mouse' && e.button !== 0) return
  const track = e.target.closest && e.target.closest('.progress-track')
  if (!track) return
  seekDrag = { pointerId: e.pointerId, track }
  track.classList.add('dragging')
  seekToClientX(e.clientX, track)
  try {
    track.setPointerCapture(e.pointerId)
  } catch {
    /* not supported */
  }
  e.preventDefault()
})

document.addEventListener('pointermove', (e) => {
  if (seekDrag && e.pointerId === seekDrag.pointerId) seekToClientX(e.clientX, seekDrag.track)
})

function endSeekDrag(e) {
  if (!seekDrag || e.pointerId !== seekDrag.pointerId) return
  seekToClientX(e.clientX, seekDrag.track)
  seekDrag.track.classList.remove('dragging')
  seekDrag = null
}

document.addEventListener('pointerup', endSeekDrag)
document.addEventListener('pointercancel', endSeekDrag)

async function toggleLikeCurrent() {
  const state = player.getPlayerState()
  if (!state.current) return
  const wasLiked = state.current.liked
  const next = !wasLiked
  player.setLiked(next)
  try {
    if (next) await endpoints.like(state.current.id)
    else await endpoints.unlike(state.current.id)
  } catch (err) {
    // Reverte o estado do player (barra e listas) — sem desync nem
    // unhandled rejection.
    player.setLiked(wasLiked)
    console.error('Falha ao alternar curtida:', err)
  }
  refreshPlayerBar()
}

// ---------- Fullscreen player ----------

let fullscreenReturnTarget = null
let fullscreenFocusRequest = 0

function focusFullscreenDialog(preferredLabel) {
  const dialog = document.querySelector('.fullscreen-player[role="dialog"]')
  if (!dialog) return false
  const buttons = [...dialog.querySelectorAll('button:not([disabled])')]
  const target = (preferredLabel === 'fullscreen-main' && dialog.querySelector('.fullscreen-btn-main'))
    || buttons.find((button) => button.getAttribute('aria-label') === preferredLabel)
    || dialog.querySelector('button[aria-label="Fechar"]')
    || dialog
  target.focus()
  return true
}

function scheduleFullscreenFocus(opening, preferredLabel = null) {
  const request = ++fullscreenFocusRequest
  let frames = 30
  const attempt = () => {
    if (request !== fullscreenFocusRequest) return
    if (player.getPlayerState().fullScreen !== opening) return
    if (opening) {
      if (focusFullscreenDialog(preferredLabel || 'Fechar')) return
    } else {
      const target = (fullscreenReturnTarget?.isConnected && fullscreenReturnTarget)
        || document.querySelector('.now-playing-open')
      if (target) {
        target.focus()
        fullscreenReturnTarget = null
        return
      }
    }
    if (frames-- > 0) requestAnimationFrame(attempt)
  }
  requestAnimationFrame(attempt)
}

function closeFullscreen() {
  player.setFullScreen(false)
  scheduleFullscreenFocus(false)
}

function fullscreenPlayer(refs) {
  const { current, playing, progress, duration, shuffle, repeat } = player.getPlayerState()
  if (!current) return null

  const totalDuration = player.resolveDuration(duration, current.duration)
  const safeProgress = Number.isFinite(progress) ? Math.min(Math.max(progress, 0), totalDuration) : 0
  const progressPercent = totalDuration > 0 ? (safeProgress / totalDuration) * 100 : 0

  const progressFill = el('div', { class: 'progress-fill', style: `width:${progressPercent}%` })
  const curTime = el('span', { class: 'progress-time' }, fmtDuration(safeProgress))
  const durTime = el('span', { class: 'progress-time' }, fmtDuration(totalDuration))
  refs.fill = progressFill
  refs.cur = curTime
  refs.dur = durTime

  return el(
    'div',
    { class: 'fullscreen-player', role: 'dialog', 'aria-modal': 'true', 'aria-label': 'Player de música', tabindex: '-1' },
    el(
      'div',
      { class: 'fullscreen-top' },
        el('button', { class: 'icon-btn', 'aria-label': 'Fechar', onclick: closeFullscreen }, icon('chevronDown')),
        el(
          'div',
          { style: 'display:flex;gap:8px' },
          downloadButton(current),
          el('button', { class: 'icon-btn', 'aria-label': 'Fila', onclick: () => navigate('/queue') }, icon('list')),
        ),
    ),
    el(
      'div',
      { class: 'fullscreen-content' },
      el('img', { class: 'fullscreen-art', src: artworkUrl(current.id, 640), alt: 'Capa da música' }),
      el(
        'div',
        { class: 'fullscreen-track-info' },
        el('div', { style: 'min-width:0' },
          el('h2', { class: 'fullscreen-title' }, current.title),
          el('p', { class: 'fullscreen-artist' }, current.artist || 'Desconhecido'),
        ),
        el(
          'button',
          { class: `btn-icon-lg ${current.liked ? 'liked' : ''}`, 'aria-label': current.liked ? 'Descurtir' : 'Curtir', 'aria-pressed': current.liked ? 'true' : 'false', onclick: toggleLikeCurrent },
          icon('heart'),
        ),
      ),
      el(
        'div',
        { class: 'player-progress', style: 'max-width:384px' },
        curTime,
        el('div', { class: 'progress-track', role: 'slider', 'aria-valuemin': 0, 'aria-valuemax': totalDuration, 'aria-valuenow': safeProgress }, progressFill),
        durTime,
      ),
      el(
        'div',
        { class: 'fullscreen-buttons' },
        el('button', { class: `player-btn ${shuffle ? 'active' : ''}`, 'aria-label': 'Aleatório', 'aria-pressed': shuffle ? 'true' : 'false', onclick: player.toggleShuffle }, icon('shuffle')),
        el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: player.prev }, icon('prev')),
        el('button', { class: 'player-btn', 'aria-label': 'Retroceder 5 segundos', onclick: () => player.seekBy(-5) }, icon('rewind5')),
        el('button', { class: 'fullscreen-btn-main', 'aria-label': playing ? 'Pausar' : 'Tocar', 'aria-pressed': playing ? 'true' : 'false', onclick: player.togglePlay }, playing ? icon('pause') : icon('play')),
        el('button', { class: 'player-btn', 'aria-label': 'Avançar 5 segundos', onclick: () => player.seekBy(5) }, icon('forward5')),
        el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: player.next }, icon('next')),
        el('button', { class: `player-btn ${repeat ? 'active' : ''}`, 'aria-label': 'Repetir', 'aria-pressed': repeat ? 'true' : 'false', onclick: player.toggleRepeat }, icon('repeat')),
      ),
    ),
  )
}

// ---------- Render ----------

function render() {
  if (auth.loading) {
    root.innerHTML = ''
    root.append(el('div', { class: 'loading-screen' }, icon('music'), 'Carregando…'))
    return
  }

  if (window.location.pathname.replace(/\/+$/, '') === '/admin/login') {
    renderAdminLogin(root)
    return
  }

  if (!auth.user) {
    renderLogin(root)
    return
  }

  if (!auth.user.isAdmin && (notificationGate.userId !== auth.user.id || notificationGate.status !== 'allowed')) { renderNotificationGate(); return }

  const onNavigate = () => {
    menuOpen = false
    render()
  }

  const app = el(
    'div',
    { class: 'app-shell' },
    el(
      'div',
      { class: 'app-body' },
      el('aside', { class: 'sidebar' }, sidebarContent(onNavigate)),
      el(
        'main',
        { class: 'main-area' },
        el(
          'div',
          { class: 'mobile-topbar' },
          el('button', { class: 'icon-btn topbar-menu', 'aria-label': 'Abrir menu', onclick: () => { menuOpen = true; render() } }, icon('menu')),
          el('span', {}, icon('music'), 'Mi Ritmo'),
          installButton(),
        ),
        el('div', { class: 'page', id: 'page-content' }),
      ),
    ),
    el('div', { id: 'player-bar' }),
    mobileNavigation(),
    el('div', { id: 'player-full' }),
  )

  root.innerHTML = ''
  root.append(app)

  if (menuOpen) {
    root.append(
      el(
        'div',
        { class: 'mobile-overlay open' },
        el('div', { class: 'mobile-overlay-backdrop', onclick: onNavigate }),
        el('aside', { class: 'mobile-sidebar' }, sidebarContent(onNavigate)),
      ),
    )
  }

  const pageEl = document.getElementById('page-content')
  const { path, params } = parseHash()
  const { fn, params: routeParams } = matchRoute(path)
  void fn(pageEl, routeParams)

  renderPlayerBar()
}

// ---------- Player UI updates (structural rebuild vs in-place) ----------

// The bar is rebuilt only when something structural changes (track, play
// state, shuffle/repeat/like, fullscreen). Progress ticks update the existing
// DOM in place, so the seek track and volume slider are never destroyed under
// an active gesture.
let barKey = null
let barRefs = null

function structuralKey() {
  const s = player.getPlayerState()
  return [s.fullScreen, s.current?.id ?? '', s.playing, s.shuffle, s.repeat, s.current?.liked ?? false].join('|')
}

function renderPlayerBar() {
  const barHost = document.getElementById('player-bar')
  const fullHost = document.getElementById('player-full')
  if (!barHost) return
  const { fullScreen } = player.getPlayerState()
  barHost.innerHTML = ''
  fullHost.innerHTML = ''
  barRefs = { fill: null, cur: null, dur: null, volIcon: null, volInput: null }
  const bar = fullScreen ? fullscreenPlayer(barRefs) : bottomBar(barRefs)
  if (bar) (fullScreen ? fullHost : barHost).append(bar)
  barKey = structuralKey()
  updateBarInPlace()
}

function updateBarInPlace() {
  if (!barRefs) return
  const s = player.getPlayerState()
  const total = player.resolveDuration(s.duration, s.current?.duration ?? 0)
  const prog = Number.isFinite(s.progress) ? Math.min(Math.max(s.progress, 0), total) : 0
  const pct = total > 0 ? (prog / total) * 100 : 0
  if (barRefs.fill) barRefs.fill.style.width = `${pct}%`
  if (barRefs.cur) barRefs.cur.textContent = fmtDuration(prog)
  if (barRefs.dur) barRefs.dur.textContent = fmtDuration(total)
  // Keep the slider's ARIA values in sync with the live state (they are
  // baked at render time when duration may still be unknown — a11y bug fix).
  if (barRefs.track) {
    barRefs.track.setAttribute('aria-valuemax', String(Math.round(total)))
    barRefs.track.setAttribute('aria-valuenow', String(Math.round(prog)))
  }
  if (barRefs.volIcon) {
    const want = s.volume === 0 ? 'volumeX' : 'volume'
    const cur = barRefs.volIcon.firstElementChild?.dataset?.icon ?? ''
    if (cur !== want) {
      barRefs.volIcon.innerHTML = icons[want]
      const svg = barRefs.volIcon.firstElementChild
      // data-icon lives on the <svg> (same as the initial render) so both
      // render paths stay consistent for selectors/tests.
      if (svg) svg.dataset.icon = want
    }
  }
  if (barRefs.volInput && Math.abs(parseFloat(barRefs.volInput.value) - s.volume) > 0.005) {
    barRefs.volInput.value = String(s.volume)
  }
}

let barRenderTimer = null
function refreshPlayerBar() {
  const key = structuralKey()
  if (key !== barKey) {
    if (barRenderTimer) return
    const activeElement = document.activeElement
    const activeDialogLabel = player.getPlayerState().fullScreen && activeElement?.closest('.fullscreen-player')
      ? (activeElement.classList.contains('fullscreen-btn-main') ? 'fullscreen-main' : activeElement.getAttribute('aria-label'))
      : null
    barRenderTimer = setTimeout(() => {
      barRenderTimer = null
      renderPlayerBar()
      if (activeDialogLabel) focusFullscreenDialog(activeDialogLabel)
    }, 50)
    return
  }
  updateBarInPlace()
  syncPagePlayerState()
}

// Sincroniza a página aberta com o estado do player (marker da fila, linha
// em reprodução e corações) sem re-renderizar a página — assim a troca de
// música por next/prev/fim reflete na fila e nas playlists na hora, e o
// estado curtido fica consistente entre a barra e as listas.
let lastPageStateKey = null

function syncPagePlayerState() {
  const s = player.getPlayerState()
  const key = `${s.current?.id ?? ''}|${s.currentIndex}|${s.current?.liked ?? false}`
  if (key === lastPageStateKey) return
  lastPageStateKey = key

  const currentId = s.current?.id

  // Fila: move o marker para a linha do índice atual.
  const list = document.querySelector('.track-list')
  if (list && currentId) {
    const oldWrap = list.querySelector('.queue-track-wrap')
    if (oldWrap) {
      const row = oldWrap.querySelector('.track-row')
      if (row) oldWrap.replaceWith(row)
    }
    const target = list.querySelector(`.track-row[data-song-id="${CSS.escape(currentId)}"]`)
    if (target && !target.closest('.queue-track-wrap')) {
      const marker = el(
        'span',
        { class: 'queue-current-marker' },
        el('span', { style: 'display:block;width:4px;height:12px;border-radius:2px;background:var(--accent)' }),
      )
      // Substitui a linha pelo wrapper e move a linha para dentro depois
      // (anexar primeiro moveria a linha e o replaceWith falharia).
      const wrap = el('div', { class: 'queue-track-wrap' }, marker)
      target.replaceWith(wrap)
      wrap.append(target)
    }
  }

  // Todas as listas: linha em reprodução + corações sincronizados.
  const likedById = new Map()
  for (const q of s.queue) likedById.set(q.id, q.liked)
  if (s.current) likedById.set(s.current.id, s.current.liked)
  document.querySelectorAll('.track-row').forEach((row) => {
    const id = row.dataset.songId
    if (!id) return
    row.classList.toggle('playing', id === currentId)
    const liked = likedById.get(id)
    if (liked === undefined) return
    const isLiked = !!liked
    const likeBtn = row.querySelector('.track-like')
    if (!likeBtn) return
    likeBtn.classList.toggle('liked', isLiked)
    likeBtn.setAttribute('aria-label', isLiked ? 'Descurtir' : 'Curtir')
    likeBtn.setAttribute('aria-pressed', String(isLiked))
  })
}

player.subscribe(refreshPlayerBar)

// ---------- Keyboard shortcuts ----------

document.addEventListener('keydown', (e) => {
  const dialog = document.querySelector('.fullscreen-player[role="dialog"]')
  if (dialog && e.key === 'Escape') {
    e.preventDefault()
    closeFullscreen()
    return
  }
  if (dialog && e.key === 'Tab') {
    const focusable = [...dialog.querySelectorAll('button:not([disabled])')]
    if (focusable.length) {
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey && (document.activeElement === first || !dialog.contains(document.activeElement))) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && (document.activeElement === last || !dialog.contains(document.activeElement))) {
        e.preventDefault()
        first.focus()
      }
    } else {
      e.preventDefault()
      dialog.focus()
    }
    return
  }
  if (e.altKey || e.ctrlKey || e.metaKey) return
  if (e.key === 'Escape' && menuOpen) {
    menuOpen = false
    render()
    return
  }
  const t = e.target
  // Elementos interativos têm atalhos nativos próprios (espaço/arrows ativam
  // botões e links). Sem esta exclusão, Espaço num botão focado dispararia o
  // clique nativo E o toggle do player — duplo-disparo.
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.tagName === 'BUTTON' || t.tagName === 'A' || t.isContentEditable)) return
  const s = player.getPlayerState()
  if (!s.current) return
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault()
      player.seekBy(-5)
      break
    case 'ArrowRight':
      e.preventDefault()
      player.seekBy(5)
      break
    case 'ArrowUp':
      e.preventDefault()
      player.setVolume(Math.min(1, s.volume + 0.1))
      break
    case 'ArrowDown':
      e.preventDefault()
      player.setVolume(Math.max(0, s.volume - 0.1))
      break
    case ' ':
      e.preventDefault()
      player.togglePlay()
      break
  }
})

// ---------- Init ----------

window.addEventListener('hashchange', render)
window.addEventListener('pm:rerender', render)
window.addEventListener('pm:playlists-changed', () => {
  if (auth.user) void loadPlaylists()
})

void refreshAuth()
