// Play Music web UI — vanilla JS, hash-based router.

import { api, endpoints, artworkUrl, getToken, setToken, readAppConfig, applyPhoneMask, phoneMask } from './api.js'
import * as player from './player.js'
import { renderAdmin } from './admin.js'

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

// ---------- Icons (inline SVG) ----------

const icons = {
  home: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>',
  search: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>',
  library: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m16 6 4 14"/><path d="M12 6v14"/><path d="M8 8v12"/><path d="M4 4v16"/></svg>',
  heart: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/></svg>',
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
  return wrap.firstChild
}

// ---------- Auth state ----------

let auth = { user: null, loading: true }

function setAuth(patch) {
  auth = { ...auth, ...patch }
  render()
}

async function refreshAuth() {
  if (!getToken()) {
    setAuth({ loading: false, user: null })
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
  const res = await api.fetch('/auth/login', {
    method: 'POST',
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
  const { onPlay } = opts

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
      'aria-label': song.liked ? 'Descurtir' : 'Curtir',
      'aria-pressed': song.liked,
      onclick: () => toggleLike(song.id, likeBtn),
    },
    icon('heart'),
  )

  const addBtn = el(
    'button',
    { class: 'track-add', 'aria-label': 'Adicionar à playlist', onclick: (e) => { e.stopPropagation(); openPlaylistPicker(song) } },
    icon('plus'),
  )

  return el(
    'div',
    { class: 'track-row' },
    numCell,
    el(
      'div',
      { class: 'track-main' },
      el('img', { class: 'track-art', src: artworkUrl(song.id, 48), alt: '', loading: 'lazy' }),
      el(
        'div',
        { class: 'track-info' },
        el('p', { class: 'track-title' }, song.title),
        el('p', { class: 'track-artist' }, song.artist || 'Desconhecido'),
      ),
    ),
    el('div', { class: 'track-actions' }, likeBtn, addBtn, el('span', { class: 'track-duration' }, fmtDuration(song.duration))),
  )
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
  try {
    if (!wasLiked) await endpoints.like(songId)
    else await endpoints.unlike(songId)
  } catch {
    btn.classList.toggle('liked', wasLiked)
  }
}

// ---------- Card ----------

function card({ image, title, subtitle, onClick, onPlay, square = true }) {
  return el(
    'div',
    { class: 'card', onclick: onClick },
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

// ---------- Pages ----------

const pages = {
  '/': renderHome,
  '/search': renderSearch,
  '/library': renderLibrary,
  '/liked': renderLiked,
  '/history': renderHistory,
  '/queue': renderQueue,
  '/lyrics': renderLyrics,
  '/settings': renderSettings,
  '/admin': renderAdminPage,
  '/category/:id': renderCategory,
  '/playlist/:id': renderPlaylist,
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
    const home = await endpoints.home()
    const sectionsEl = []
    for (const s of home.sections ?? []) {
      const songs = s.songs ?? []
      if (songs.length === 0) continue
      const cards = songs.map((song) =>
        songCard(song, () => player.playContext(songs, songs.indexOf(song))),
      )
      sectionsEl.push(section(s.title, cards))
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
    if ((home.sections ?? []).length === 0) {
      container.append(
        emptyState2(
          'Sua biblioteca está vazia.',
          'O administrador precisa fazer upload de músicas e atribuí-las a uma categoria.',
        ),
      )
    }
  } catch (err) {
    container.append(emptyState(err.message))
  }
}

// ---------- Search ----------

let searchBusy = false
let searchResults = null
let searchQuery = ''
let searchType = 'all'

async function runSearch(q) {
  searchQuery = q
  if (!q.trim()) {
    searchResults = null
    searchBusy = false
    render()
    return
  }
  searchBusy = true
  render()
  try {
    searchResults = await endpoints.search(q, searchType)
  } catch {
    searchResults = null
  } finally {
    searchBusy = false
    render()
  }
}

const searchTypes = [
  { id: 'all', label: 'Tudo' },
  { id: 'songs', label: 'Músicas' },
  { id: 'categories', label: 'Categorias' },
  { id: 'playlists', label: 'Playlists' },
]

async function renderSearch(container) {
  const { params } = parseHash()
  container.innerHTML = ''
  const initial = params.get('genre') || ''

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
  container.append(page)

  if (initial && searchQuery !== initial) await runSearch(initial)
  if (searchBusy && !searchResults) {
    page.append(spinner())
    return
  }
  if (!searchResults) return

  const results = searchResults
  const songs = results.songs ?? []
  const cats = results.categories ?? []
  const pls = results.playlists ?? []
  const blocks = []
  const all = searchType === 'all'
  if ((all || searchType === 'songs') && songs.length > 0) {
    const rows = songs.map((s, i) =>
      trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }),
    )
    blocks.push(
      el('div', {},
        el('h2', { class: 'section-title' }, 'Músicas'),
        el('div', { class: 'track-list' }, ...rows),
      ),
    )
  }
  if ((all || searchType === 'categories') && cats.length > 0) {
    blocks.push(
      section(
        'Categorias',
        cats.map((c) => categoryCard(c)),
      ),
    )
  }
  if ((all || searchType === 'playlists') && pls.length > 0) {
    blocks.push(
      section(
        'Playlists',
        pls.map((p) =>
          card({ image: artworkUrl(p.id, 300), title: p.name, subtitle: p.owner, onClick: () => navigate(`/playlist/${p.id}`) }),
        ),
      ),
    )
  }
  const total =
    (all || searchType === 'songs' ? songs.length : 0) +
    (all || searchType === 'categories' ? cats.length : 0) +
    (all || searchType === 'playlists' ? pls.length : 0)
  if (total === 0) {
    blocks.push(el('p', { class: 'empty-state' }, `Nada encontrado para “${searchQuery}”.`))
  }
  page.append(...blocks)
}

// ---------- Library ----------

let libTab = 'songs'
let libData = null
let libLoading = false

async function loadLibrary() {
  if (libLoading) return
  libLoading = true
  const [s, p, c] = await Promise.allSettled([endpoints.songs(), endpoints.playlists(), endpoints.categories()])
  libData = {
    songs: s.status === 'fulfilled' ? s.value : [],
    playlists: p.status === 'fulfilled' ? p.value : [],
    categories: c.status === 'fulfilled' ? c.value : [],
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

function playShuffle(songs) {
  const shuffled = [...songs].sort(() => Math.random() - 0.5)
  player.playContext(shuffled, 0)
}

// ---------- Playlist ----------

async function renderPlaylist(container, params) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const playlist = await endpoints.playlist(params.id)
    const songs = playlist.songs.map((ps) => ps.song)

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
    if (playlist.songs.length === 0) {
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
  await endpoints.removePlaylistTrack(id, entryId)
  render()
}

// ---------- Liked ----------

async function renderLiked(container) {
  container.innerHTML = ''
  container.append(spinner())
  try {
    const songs = await endpoints.liked()
    const header = el(
      'div',
      { class: 'detail-header' },
      el('div', { class: 'detail-art-icon' }, '♥'),
      el(
        'div',
        { class: 'detail-info' },
        el('p', { class: 'detail-type' }, 'Playlist'),
        el('h1', { class: 'detail-title' }, 'Curtidas'),
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
        ),
      ),
    )
    const content = el('div', { class: 'detail-content' })
    if (songs.length === 0) {
      content.append(emptyState2('Nenhuma música curtida ainda.', 'Toque no coração de uma música para salvá-la aqui.'))
    } else {
      const rows = songs.map((s, i) => trackRow(s, i, { onPlay: (_song, idx) => player.playContext(songs, idx) }))
      content.append(el('div', { class: 'track-list' }, ...rows))
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

// ---------- Lyrics ----------

let lyricsData = null
let lyricsLoaded = false

async function renderLyrics(container) {
  const { current } = player.getPlayerState()
  container.innerHTML = ''
  if (!current) {
    container.append(emptyState('Nenhuma música tocando. Inicie uma reprodução para ver as letras.'))
    return
  }

  lyricsData = null
  lyricsLoaded = false

  const page = el(
    'div',
    { class: 'lyrics-page' },
    el('button', { class: 'back-link', onclick: () => history.back() }, icon('arrowLeft'), 'Voltar'),
    el(
      'div',
      { class: 'lyrics-header' },
      el('img', { class: 'lyrics-art', src: artworkUrl(current.id, 96), alt: '' }),
      el('div', {},
        el('h1', { class: 'lyrics-title' }, current.title),
        el('p', { class: 'lyrics-artist' }, current.artist),
      ),
    ),
  )
  container.append(el('div', { class: 'page' }, page))

  try {
    lyricsData = await endpoints.lyrics(current.id)
  } catch {
    lyricsData = null
  }
  lyricsLoaded = true

  const linesEl = el('div', { class: 'lyrics-lines' })
  if (!lyricsData || lyricsData.lines.length === 0) {
    page.append(emptyState('Nenhuma letra encontrada para esta música.'))
    return
  }
  lyricsData.lines.forEach((line, i) => {
    linesEl.append(el('p', { class: 'lyric-line', 'data-idx': i }, line.text || '♪'))
  })
  page.append(linesEl)
  updateLyricsHighlight()
}

function updateLyricsHighlight() {
  if (!lyricsData || !lyricsData.synced) return
  const { progress } = player.getPlayerState()
  const activeIndex = lyricsData.lines.findIndex((l, i) => {
    const next = lyricsData.lines[i + 1]
    if (l.start == null) return false
    if (next && next.start != null) return progress * 1000 >= l.start && progress * 1000 < next.start
    return progress * 1000 >= l.start
  })
  document.querySelectorAll('.lyric-line').forEach((n) => {
    n.classList.toggle('active', Number(n.dataset.idx) === activeIndex)
  })
}

// ---------- Settings ----------

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
        el('div', {}, el('dt', { class: 'settings-dt' }, 'Pasta de música'), el('dd', { class: 'settings-dd mono' }, settings.musicFolder)),
      ),
    )
  }

  container.innerHTML = ''
  container.append(
    el('div', { class: 'settings-page' },
      el('h1', { class: 'page-title' }, 'Configurações'),
      accountCard,
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

let loginMode = 'client' // 'client' | 'admin'

function renderLogin(container) {
  const toggle = el(
    'div',
    { class: 'login-toggle' },
    el('button', { class: `login-toggle-btn ${loginMode === 'client' ? 'active' : ''}`, onclick: () => { loginMode = 'client'; render() } }, 'Cliente'),
    el('button', { class: `login-toggle-btn ${loginMode === 'admin' ? 'active' : ''}`, onclick: () => { loginMode = 'admin'; render() } }, 'Administrador'),
  )

  const phoneInput = loginMode === 'client'
    ? el('input', { class: 'form-input', type: 'tel', inputmode: 'numeric', placeholder: 'Telefone (99) 99999-9999', autofocus: true, autocomplete: 'tel' })
    : null
  if (phoneInput) phoneInput.addEventListener('input', () => applyPhoneMask(phoneInput))
  const usernameInput = loginMode === 'admin'
    ? el('input', { class: 'form-input', type: 'text', placeholder: 'Usuário', autofocus: true, autocomplete: 'username' })
    : null
  const passwordInput = loginMode === 'admin'
    ? el('input', { class: 'form-input', type: 'password', placeholder: 'Senha', autocomplete: 'current-password' })
    : null
  const errorEl = el('p', { class: 'login-error' })
  const submitBtn = el('button', { class: 'login-submit', type: 'submit' }, 'Entrar')

  const form = el(
    'form',
    {
      class: 'login-form',
      onsubmit: async (e) => {
        e.preventDefault()
        errorEl.textContent = ''
        if (loginMode === 'client') {
          if (!phoneInput.value.trim()) {
            errorEl.textContent = 'Informe seu telefone.'
            return
          }
        } else if (!usernameInput.value.trim() || !passwordInput.value) {
          errorEl.textContent = 'Informe usuário e senha.'
          return
        }
        submitBtn.disabled = true
        submitBtn.textContent = 'Aguarde…'
        try {
          if (loginMode === 'client') await doLogin('client', phoneInput.value.trim())
          else await doLogin('admin', usernameInput.value.trim(), passwordInput.value)
          navigate('/')
        } catch (err) {
          errorEl.textContent = err instanceof Error ? err.message : 'Erro ao entrar'
        } finally {
          submitBtn.disabled = false
          submitBtn.textContent = 'Entrar'
        }
      },
    },
    phoneInput,
    usernameInput,
    passwordInput,
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
          el('h1', {}, 'Play Music'),
          el('p', {}, loginMode === 'client' ? 'Entre com seu telefone para ouvir' : 'Acesso administrativo'),
        ),
        form,
      ),
    ),
  )
}

// ---------- Sidebar ----------

let playlistsCache = []
let menuOpen = false

async function loadPlaylists() {
  if (!getToken()) {
    playlistsCache = []
    return
  }
  try {
    playlistsCache = (await endpoints.playlists()) ?? []
  } catch {
    playlistsCache = []
  }
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
      el('span', { class: 'sidebar-brand' }, icon('music'), 'Play Music'),
      el('button', { class: 'sidebar-close', 'aria-label': 'Fechar', onclick: onNavigate }, icon('x')),
    ),
    el(
      'nav',
      { class: 'sidebar-nav' },
      nav('/', 'Início', 'home'),
      nav('/search', 'Buscar', 'search'),
      nav('/library', 'Sua Biblioteca', 'library'),
      nav('/liked', 'Curtidas', 'heart'),
      nav('/history', 'Histórico', 'clock'),
      el(
        'a',
        { class: 'nav-link', href: './loja.html', onclick: () => { onNavigate?.() } },
        icon('cart'),
        'Loja',
      ),
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

// ---------- Bottom bar ----------

function bottomBar() {
  const { current, playing, progress, duration, volume, shuffle, repeat } = player.getPlayerState()
  if (!current) return null

  const totalDuration = player.resolveDuration(duration, current.duration)
  const safeProgress = Number.isFinite(progress) ? Math.min(Math.max(progress, 0), totalDuration) : 0
  const progressPercent = totalDuration > 0 ? (safeProgress / totalDuration) * 100 : 0

  const progressFill = el('div', { class: 'progress-fill', style: `width:${progressPercent}%` })
  const progressTrack = el(
    'div',
    { class: 'progress-track', role: 'slider', 'aria-valuemin': 0, 'aria-valuemax': totalDuration, 'aria-valuenow': safeProgress, onclick: seekFromEvent },
    progressFill,
  )

  const likeBtn = el(
    'button',
    { class: `icon-btn ${current.liked ? 'liked' : ''}`, style: current.liked ? 'color:var(--accent)' : '', 'aria-label': current.liked ? 'Descurtir' : 'Curtir', onclick: toggleLikeCurrent },
    icon('heart'),
  )

  const volumeIcon = volume === 0 ? icon('volumeX') : icon('volume')

  return el(
    'div',
    { class: 'bottom-bar' },
    el(
      'div',
      { class: 'now-playing' },
      el('img', { class: 'now-playing-art', src: artworkUrl(current.id, 64), alt: '' }),
      el('div', { class: 'now-playing-info' },
        el('p', { class: 'now-playing-title' }, current.title),
        el('p', { class: 'now-playing-artist' }, current.artist || 'Desconhecido'),
      ),
      likeBtn,
    ),
    el(
      'div',
      { class: 'player-controls' },
      el(
        'div',
        { class: 'player-buttons' },
        el('button', { class: `player-btn ${shuffle ? 'active' : ''}`, 'aria-label': 'Aleatório', onclick: player.toggleShuffle }, icon('shuffle')),
        el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: player.prev }, icon('prev')),
        el('button', { class: 'player-btn-main', 'aria-label': playing ? 'Pausar' : 'Tocar', onclick: player.togglePlay }, playing ? icon('pause') : icon('play')),
        el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: player.next }, icon('next')),
        el('button', { class: `player-btn ${repeat ? 'active' : ''}`, 'aria-label': 'Repetir', onclick: player.toggleRepeat }, icon('repeat')),
      ),
      el(
        'div',
        { class: 'player-progress' },
        el('span', { class: 'progress-time' }, fmtDuration(safeProgress)),
        progressTrack,
        el('span', { class: 'progress-time' }, fmtDuration(totalDuration)),
      ),
      el(
        'div',
        { class: 'player-extra' },
        el('button', { class: 'icon-btn', 'aria-label': 'Fila', onclick: () => navigate('/queue') }, icon('list')),
        el('button', { class: 'icon-btn', 'aria-label': 'Letras', onclick: () => navigate('/lyrics') }, el('span', { style: 'font-size:12px;font-weight:700' }, 'LYR')),
        el('button', { class: 'icon-btn', 'aria-label': 'Tela cheia', onclick: () => player.setFullScreen(true) }, icon('max')),
      ),
    ),
    el(
      'div',
      { class: 'player-volume' },
      volumeIcon,
      el('input', {
        class: 'volume-slider',
        type: 'range',
        min: 0,
        max: 1,
        step: 0.01,
        value: volume,
        'aria-label': 'Volume',
        oninput: (e) => player.setVolume(parseFloat(e.target.value)),
      }),
      el('button', { class: 'icon-btn', 'aria-label': 'Fila', onclick: () => navigate('/queue') }, icon('list')),
      el('button', { class: 'icon-btn', 'aria-label': 'Letras', onclick: () => navigate('/lyrics') }, el('span', { style: 'font-size:12px;font-weight:700;letter-spacing:0.05em' }, 'LYR')),
      el('button', { class: 'icon-btn', 'aria-label': 'Tela cheia', onclick: () => player.setFullScreen(true) }, icon('max')),
    ),
  )
}

function seekFromEvent(e) {
  const rect = e.currentTarget.getBoundingClientRect()
  if (rect.width === 0) return
  const state = player.getPlayerState()
  const totalDuration = player.resolveDuration(state.duration, state.current?.duration ?? 0)
  if (totalDuration === 0) return
  const ratio = (e.clientX - rect.left) / rect.width
  player.seek(ratio * totalDuration)
}

function toggleLikeCurrent() {
  const state = player.getPlayerState()
  if (!state.current) return
  const next = !state.current.liked
  player.setLiked(next)
  if (next) void endpoints.like(state.current.id)
  else void endpoints.unlike(state.current.id)
  refreshPlayerBar()
}

// ---------- Fullscreen player ----------

function fullscreenPlayer() {
  const { current, playing, progress, duration, shuffle, repeat } = player.getPlayerState()
  if (!current) return null

  const totalDuration = player.resolveDuration(duration, current.duration)
  const safeProgress = Number.isFinite(progress) ? Math.min(Math.max(progress, 0), totalDuration) : 0
  const progressPercent = totalDuration > 0 ? (safeProgress / totalDuration) * 100 : 0

  return el(
    'div',
    { class: 'fullscreen-player' },
    el(
      'div',
      { class: 'fullscreen-top' },
      el('button', { class: 'icon-btn', 'aria-label': 'Fechar', onclick: () => player.setFullScreen(false) }, icon('chevronDown')),
      el('button', { class: 'icon-btn', 'aria-label': 'Fila', onclick: () => navigate('/queue') }, icon('list')),
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
          { class: `btn-icon-lg ${current.liked ? 'liked' : ''}`, 'aria-label': current.liked ? 'Descurtir' : 'Curtir', onclick: toggleLikeCurrent },
          icon('heart'),
        ),
      ),
      el(
        'div',
        { class: 'player-progress', style: 'max-width:384px' },
        el('span', { class: 'progress-time' }, fmtDuration(safeProgress)),
        el('div', { class: 'progress-track', onclick: seekFromEvent }, el('div', { class: 'progress-fill', style: `width:${progressPercent}%` })),
        el('span', { class: 'progress-time' }, fmtDuration(totalDuration)),
      ),
      el(
        'div',
        { class: 'fullscreen-buttons' },
        el('button', { class: `player-btn ${shuffle ? 'active' : ''}`, 'aria-label': 'Aleatório', onclick: player.toggleShuffle }, icon('shuffle')),
        el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: player.prev }, icon('prev')),
        el('button', { class: 'fullscreen-btn-main', 'aria-label': playing ? 'Pausar' : 'Tocar', onclick: player.togglePlay }, playing ? icon('pause') : icon('play')),
        el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: player.next }, icon('next')),
        el('button', { class: `player-btn ${repeat ? 'active' : ''}`, 'aria-label': 'Repetir', onclick: player.toggleRepeat }, icon('repeat')),
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

  if (!auth.user) {
    renderLogin(root)
    return
  }

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
          el('button', { class: 'icon-btn', 'aria-label': 'Abrir menu', onclick: () => { menuOpen = true; render() } }, icon('menu')),
          el('span', {}, icon('music'), 'Play Music'),
        ),
        el('div', { class: 'page', id: 'page-content' }),
      ),
    ),
    el('div', { id: 'player-bar' }),
    el('div', { id: 'player-full' }),
  )

  root.innerHTML = ''
  root.append(app)

  const pageEl = document.getElementById('page-content')
  const { path, params } = parseHash()
  const { fn, params: routeParams } = matchRoute(path)
  void fn(pageEl, routeParams)

  renderPlayerBar()
}

function renderPlayerBar() {
  const barHost = document.getElementById('player-bar')
  const fullHost = document.getElementById('player-full')
  if (!barHost) return
  const { fullScreen } = player.getPlayerState()
  barHost.innerHTML = ''
  barHost.append(bottomBar() ?? '')
  fullHost.innerHTML = ''
  if (fullScreen) fullHost.append(fullscreenPlayer())
}

// ---------- Player UI updates (in-place, no full re-render) ----------

// Trailing-edge debounce: limits re-renders to one per 50ms window but always
// renders the final state, so the last event is never dropped.
let barRenderTimer = null
function refreshPlayerBar() {
  if (barRenderTimer) return
  barRenderTimer = setTimeout(() => {
    barRenderTimer = null
    renderPlayerBar()
    updateLyricsHighlight()
  }, 50)
}

player.subscribe(refreshPlayerBar)

// ---------- Init ----------

window.addEventListener('hashchange', render)
window.addEventListener('pm:rerender', render)
window.addEventListener('pm:playlists-changed', () => {
  if (auth.user) void loadPlaylists()
})

void refreshAuth().then(() => {
  if (auth.user) void loadPlaylists()
})
