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

export async function apiFetch(path, options = {}) {
  const token = getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set(AUTH_HEADER, `Bearer ${token}`)

  const res = await fetch(path, { ...options, headers })

  const refreshed = res.headers.get(AUTH_HEADER)
  if (refreshed) setToken(refreshed)

  if (res.status === 401) {
    setToken(null)
    window.dispatchEvent(new Event('pm:unauthorized'))
    throw new Error('Não autenticado')
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
}

export const endpoints = {
  me: () => api.get('/api/me'),
  settings: () => api.get('/api/settings'),
  home: () => api.get('/api/home'),
  search: (q) => api.get(`/api/search?q=${encodeURIComponent(q)}`),

  albums: (params = '') => api.get(`/api/albums${params}`),
  album: (id) => api.get(`/api/albums/${id}`),
  artists: () => api.get('/api/artists'),
  artist: (id) => api.get(`/api/artists/${id}`),
  song: (id) => api.get(`/api/songs/${id}`),

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

  lyrics: (id) => api.get(`/api/lyrics/${id}`),
}

export function artworkUrl(id, size = 300) {
  // <img> tags cannot send the Authorization header, so the JWT travels as a
  // query parameter (the server accepts ?jwt= via jwtauth.TokenFromQuery).
  const q = new URLSearchParams({ size: String(size) })
  const token = getToken()
  if (token) q.set('jwt', token)
  return `/api/artwork/${id}?${q.toString()}`
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
  return `/api/stream/${song.id}${qs ? `?${qs}` : ''}`
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
