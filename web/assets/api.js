const AUTH_HEADER = 'X-ND-Authorization'
const PUSH_UNAVAILABLE_HEADER = 'X-Mi-Ritmo-Push-Unavailable'
export const TOKEN_KEY = 'pm_token'

// UI events other modules dispatch on window. They live here so a typo cannot
// silently stop a listener from firing.
export const RERENDER_EVENT = 'pm:rerender'
export const UNAUTHORIZED_EVENT = 'pm:unauthorized'
export const PLAYLISTS_CHANGED_EVENT = 'pm:playlists-changed'

// The ChatGPT/Codex preview browser cannot grant notification permission, so the
// server lets it skip the push gate.
export function isChatGPTBrowser() {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') return false
  return new URLSearchParams(window.location?.search || '').get('chatgpt') === '1' ||
    /ChatGPT|Codex Browser/i.test(navigator.userAgent || '')
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

// decodeBase64URL decodes the URL-safe base64 alphabet JWTs and VAPID keys use.
export function decodeBase64URL(value) {
  const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0))
}

// jwtClaims decodes a JWT payload without verifying it: the server verifies
// every request, this is only for local UI decisions.
function jwtClaims(token) {
  const payload = (token || '').split('.')[1]
  if (!payload) return null
  try {
    return JSON.parse(new TextDecoder().decode(decodeBase64URL(payload)))
  } catch {
    return null
  }
}

// tokenUserId is the `uid` claim of the current token, or null.
export function tokenUserId() {
  return jwtClaims(getToken())?.uid || null
}

// tokenExpiry is the `exp` claim in milliseconds, or null when absent.
export function tokenExpiry(token) {
  const exp = jwtClaims(token)?.exp
  return typeof exp === 'number' ? exp * 1000 : null
}

// Resolves a path against the configured API base URL (see window.__APP_CONFIG__).
function resolve(path) {
  const base = (readAppConfig().baseURL || '').replace(/\/+$/, '')
  return base + path
}

// errorMessage is the server's `error` field, or fallback when the body is not
// JSON. Every failed response reports through it.
async function errorMessage(res, fallback) {
  try {
    const body = await res.json()
    if (body?.error) return body.error
  } catch {
    /* not json */
  }
  return fallback
}

async function apiFetch(path, options = {}) {  const token = options.anonymous ? null : getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set(AUTH_HEADER, `Bearer ${token}`)
  if (isChatGPTBrowser()) headers.set(PUSH_UNAVAILABLE_HEADER, '1')

  const { anonymous, ...fetchOptions } = options
  const res = await fetch(resolve(path), { ...fetchOptions, headers })

  const refreshed = res.headers.get(AUTH_HEADER)
  if (refreshed && token === getToken()) setToken(refreshed)

  if (res.status === 401) {
    if (!anonymous && token === getToken()) {
      setToken(null)
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
    throw new Error(await errorMessage(res, 'Não autenticado'))
  }

  if (!res.ok) throw new Error(await errorMessage(res, `HTTP ${res.status}`))

  if (res.status === 204) return undefined
  return res.json()
}

export const api = {
  fetch: apiFetch,
  get: (path) => apiFetch(path),
  post: (path, body) =>
    apiFetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  put: (path, body) =>
    apiFetch(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  del: (path) => apiFetch(path, { method: 'DELETE' }),
  upload: (path, file) => {
    const fd = new FormData()
    fd.append('photo', file)
    return apiFetch(path, { method: 'POST', body: fd })
  },
}

export const endpoints = {
  me: () => api.get('/api/me'),
  register: (phone) => api.post('/api/register', { phone }),
  recommendations: () => api.get('/api/recommendations'),
  qualifiedListen: (songId, playId) => api.post('/api/me/listens', { songId, playId }),
  settings: () => api.get('/api/settings'),
  pushConfig: (options) => apiFetch('/api/push/config', options),
  home: () => api.get('/api/home'),
  search: (q, type = 'all') => api.get(`/api/search?q=${encodeURIComponent(q)}&type=${encodeURIComponent(type)}`),
  categories: () => api.get('/api/categories'),
  category: (id) => api.get(`/api/categories/${id}`),

  albums: () => api.get('/api/albums'),
  album: (id) => api.get(`/api/albums/${id}`),
  songs: () => api.get('/api/songs'),
  song: (id) => api.get(`/api/songs/${id}`),

  playlists: () => api.get('/api/playlists'),
  playlist: (id) => api.get(`/api/playlists/${id}`),
  createPlaylist: (name, songIds) => api.post('/api/playlists', { name, songIds }),
  addPlaylistTracks: (id, songIds) => api.post(`/api/playlists/${id}/tracks`, { songIds }),
  removePlaylistTrack: (id, entryId) => api.del(`/api/playlists/${id}/tracks/${entryId}`),

  liked: () => api.get('/api/me/liked'),
  likedAlbums: () => api.get('/api/me/liked/albums'),
  like: (id) => api.put(`/api/me/liked/${id}`),
  unlike: (id) => api.del(`/api/me/liked/${id}`),
  downloads: () => api.get('/api/me/downloads'),
  download: (id) => api.post(`/api/songs/${encodeURIComponent(id)}/download`),
  likeAlbum: (id) => api.put(`/api/me/liked/albums/${id}`),
  unlikeAlbum: (id) => api.del(`/api/me/liked/albums/${id}`),
  history: () => api.get('/api/me/history'),
  registerPlay: (id) => api.post(`/api/me/history/${id}`),

  admin: {
    users: () => api.get('/api/admin/users'),
    createUser: (u) => api.post('/api/admin/users', u),
    updateUser: (id, u) => api.put(`/api/admin/users/${id}`, u),
    deleteUser: (id) => api.del(`/api/admin/users/${id}`),
    categories: () => api.get('/api/admin/categories'),
    category: (id) => api.get(`/api/admin/categories/${id}`),
    createCategory: (name) => api.post('/api/admin/categories', { name }),
    updateCategory: (id, body) => api.put(`/api/admin/categories/${id}`, body),
    deleteCategory: (id) => api.del(`/api/admin/categories/${id}`),
    songs: () => api.get('/api/admin/songs'),
    uploadSong: (fd) => apiFetch('/api/admin/songs', { method: 'POST', body: fd }),
    uploadSongPhoto: (id, file) => api.upload(`/api/admin/songs/${id}/photo`, file),
    deleteSongPhoto: (id) => api.del(`/api/admin/songs/${id}/photo`),
    uploadCategoryPhoto: (id, file) => api.upload(`/api/admin/categories/${id}/photo`, file),
    deleteCategoryPhoto: (id) => api.del(`/api/admin/categories/${id}/photo`),
  },
}

export function artworkUrl(id, size = 300) {
  // <img> cannot send the Authorization header, so the JWT travels as a query
  // parameter (?jwt=).
  const q = new URLSearchParams({ size: String(size) })
  const token = getToken()
  if (token) q.set('jwt', token)
  if (isChatGPTBrowser()) q.set('pushUnavailable', '1')
  return resolve(`/api/artwork/${id}?${q.toString()}`)
}

export function streamUrl(song) {
  // <audio> cannot send the Authorization header either. The server picks
  // between the stored file and an mp3 transcode from the song's format.
  const q = new URLSearchParams()
  const token = getToken()
  if (token) q.set('jwt', token)
  if (isChatGPTBrowser()) q.set('pushUnavailable', '1')
  const qs = q.toString()
  return resolve(`/api/stream/${song.id}${qs ? `?${qs}` : ''}`)
}

// Phone mask: formats input as (99) 99999-9999 / (99) 9999-9999.
export function phoneMask(raw) {
  const digits = raw.replace(/\D/g, '').slice(0, 11)
  let out = ''
  for (let i = 0; i < digits.length; i++) {
    if (i === 0) out += '('
    else if (i === 2) out += ' '
    else if (i === 6 && digits.length === 10) out += '-'
    else if (i === 7 && digits.length > 10) out += '-'
    out += digits[i]
    if (i === 1) out += ')'
  }
  return out
}

// applyPhoneMask remasks an input on every keystroke, keeping the caret
// anchored to the same digit position (formatting chars shift it otherwise).
export function applyPhoneMask(input) {
  const caret = input.selectionStart ?? input.value.length
  const before = input.value.slice(0, caret).replace(/\D/g, '').length
  input.value = phoneMask(input.value)
  let pos = 0
  let digits = 0
  for (; pos < input.value.length && digits < before; pos++) {
    if (/\d/.test(input.value[pos])) digits++
  }
  input.setSelectionRange(pos, pos)
}

// readAppConfig is the app config the shell inlined into index.html.
export function readAppConfig() {
  return window.__APP_CONFIG__ ?? { version: 'dev', baseURL: '' }
}
