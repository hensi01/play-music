// API client: fetch wrapper with JWT auth, token refresh and all endpoints.

const AUTH_HEADER = 'X-ND-Authorization'
export const TOKEN_KEY = 'pm_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

// Resolves a path against the configured API base URL (see window.__APP_CONFIG__).
function resolve(path) {
  const base = (readAppConfig().baseURL || '').replace(/\/+$/, '')
  return base + path
}

export async function apiFetch(path, options = {}) {
  const token = getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set(AUTH_HEADER, `Bearer ${token}`)

  const res = await fetch(resolve(path), { ...options, headers })

  const refreshed = res.headers.get(AUTH_HEADER)
  if (refreshed) setToken(refreshed)

  if (res.status === 401) {
    setToken(null)
    window.dispatchEvent(new Event('pm:unauthorized'))
    let msg = 'Não autenticado'
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* not json */
    }
    throw new Error(msg)
  }

  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* not json */
    }
    throw new Error(msg)
  }

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
  settings: () => api.get('/api/settings'),
  home: () => api.get('/api/home'),
  search: (q, type = 'all') => api.get(`/api/search?q=${encodeURIComponent(q)}&type=${encodeURIComponent(type)}`),
  categories: () => api.get('/api/categories'),
  category: (id) => api.get(`/api/categories/${id}`),

  albums: (params = '') => api.get(`/api/albums${params}`),
  album: (id) => api.get(`/api/albums/${id}`),
  artists: () => api.get('/api/artists'),
  artist: (id) => api.get(`/api/artists/${id}`),
  songs: () => api.get('/api/songs'),
  song: (id) => api.get(`/api/songs/${id}`),

  karaokes: () => api.get('/api/karaokes'),
  karaoke: (id) => api.get(`/api/karaokes/${id}`),
  registerKaraokePlay: (id) => api.post(`/api/me/karaoke/${id}`),

  playlists: () => api.get('/api/playlists'),
  playlist: (id) => api.get(`/api/playlists/${id}`),
  createPlaylist: (name, songIds) => api.post('/api/playlists', { name, songIds }),
  updatePlaylist: (id, body) => api.put(`/api/playlists/${id}`, body),
  deletePlaylist: (id) => api.del(`/api/playlists/${id}`),
  addPlaylistTracks: (id, songIds) => api.post(`/api/playlists/${id}/tracks`, { songIds }),
  removePlaylistTrack: (id, entryId) => api.del(`/api/playlists/${id}/tracks/${entryId}`),
  reorderPlaylistTracks: (id, from, to) => api.put(`/api/playlists/${id}/tracks`, { from, to }),

  liked: () => api.get('/api/me/liked'),
  like: (id) => api.put(`/api/me/liked/${id}`),
  unlike: (id) => api.del(`/api/me/liked/${id}`),
  history: () => api.get('/api/me/history'),
  registerPlay: (id) => api.post(`/api/me/history/${id}`),

  queue: () => api.get('/api/queue'),
  saveQueue: (q) => api.put('/api/queue', q),

  admin: {
    users: () => api.get('/api/admin/users'),
    createUser: (u) => api.post('/api/admin/users', u),
    updateUser: (id, u) => api.put(`/api/admin/users/${id}`, u),
    deleteUser: (id) => api.del(`/api/admin/users/${id}`),
    categories: () => api.get('/api/admin/categories'),
    category: (id) => api.get(`/api/admin/categories/${id}`),
    createCategory: (name, checkoutUrl) => api.post('/api/admin/categories', { name, checkoutUrl: checkoutUrl || '' }),
    updateCategory: (id, body) => api.put(`/api/admin/categories/${id}`, body),
    deleteCategory: (id) => api.del(`/api/admin/categories/${id}`),
    albums: () => api.get('/api/admin/albums'),
    artists: () => api.get('/api/admin/artists'),
    songs: () => api.get('/api/admin/songs'),
    uploadSong: (fd) => apiFetch('/api/admin/songs', { method: 'POST', body: fd }),
    uploadPhoto: (id, file) => api.upload(`/api/admin/albums/${id}/photo`, file),
    deletePhoto: (id) => api.del(`/api/admin/albums/${id}/photo`),
    uploadSongPhoto: (id, file) => api.upload(`/api/admin/songs/${id}/photo`, file),
    deleteSongPhoto: (id) => api.del(`/api/admin/songs/${id}/photo`),
    uploadCategoryPhoto: (id, file) => api.upload(`/api/admin/categories/${id}/photo`, file),
    deleteCategoryPhoto: (id) => api.del(`/api/admin/categories/${id}/photo`),
    karaokes: () => api.get('/api/admin/karaokes'),
    uploadKaraoke: (fd) => apiFetch('/api/admin/karaokes', { method: 'POST', body: fd }),
    uploadKaraokePhoto: (id, file) => api.upload(`/api/admin/karaokes/${id}/photo`, file),
    deleteKaraokePhoto: (id) => api.del(`/api/admin/karaokes/${id}/photo`),
  },
}

export function artworkUrl(id, size = 300) {
  // <img> tags cannot send the Authorization header, so the JWT travels as a
  // query parameter (the server accepts ?jwt= via jwtauth.TokenFromQuery).
  const q = new URLSearchParams({ size: String(size) })
  const token = getToken()
  if (token) q.set('jwt', token)
  return resolve(`/api/artwork/${id}?${q.toString()}`)
}

// Formats browsers can usually play natively; anything else is transcoded to mp3.
const nativeFormats = new Set(['mp3', 'm4a', 'aac', 'ogg', 'opus', 'wav', 'flac'])

export function streamUrl(song, fallback = false) {
  // <audio> cannot send the Authorization header, so the JWT travels as a query
  // parameter (the server accepts ?jwt= via jwtauth.TokenFromQuery).
  const q = new URLSearchParams()
  if (fallback || !nativeFormats.has(song.format.toLowerCase())) q.set('format', 'mp3')
  const token = getToken()
  if (token) q.set('jwt', token)
  const qs = q.toString()
  return resolve(`/api/stream/${song.id}${qs ? `?${qs}` : ''}`)
}

// karaokeStreamUrl builds the <video> src for a karaoke (JWT as ?jwt=).
export function karaokeStreamUrl(karaoke) {
  const q = new URLSearchParams()
  const token = getToken()
  if (token) q.set('jwt', token)
  const qs = q.toString()
  return resolve(`/api/karaoke/stream/${karaoke.id}${qs ? `?${qs}` : ''}`)
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

export function readAppConfig() {
  const raw = window.__APP_CONFIG__
  if (!raw) return { version: 'dev', firstTime: false, baseURL: '' }
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw)
    } catch {
      return { version: 'dev', firstTime: false, baseURL: '' }
    }
  }
  return raw
}
