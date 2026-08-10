// Karaoke video player — dedicated fullscreen <video> player, fully
// independent from the audio player (player.js). Plays MP4 karaoke videos
// from the library with play/pause, seek, volume, playback speed, previous /
// next within its own queue, and keyboard shortcuts.

import { karaokeStreamUrl, endpoints } from './api.js'

const video = document.createElement('video')
video.playsInline = true
video.preload = 'auto'
video.volume = 0.8

const state = {
  list: [],
  currentIndex: -1,
  current: null,
  playing: false,
  progress: 0,
  duration: 0,
  volume: 0.8,
  rate: 1,
}

// ---------- mini helpers (self-contained: no import cycle with app.js) ----------

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue
    if (k === 'class') node.className = v
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

const icons = {
  play: '<svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor"><polygon points="6 3 20 12 6 21 6 3"/></svg>',
  pause: '<svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>',
  prev: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="19 20 9 12 19 4 19 20"/><rect x="5" y="4" width="2.5" height="16" rx="1"/></svg>',
  next: '<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 4 15 12 5 20 5 4"/><rect x="16.5" y="4" width="2.5" height="16" rx="1"/></svg>',
  rewind5: '<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M11.99 2C7.5 2 3.85 4.86 2.55 8.86H5.1c.98-2.68 3.57-4.61 6.55-4.61 3.87 0 7 3.13 7 7s-3.13 7-7 7c-2.98 0-5.57-1.93-6.55-4.61H2.55C3.85 19.14 7.5 22 11.99 22c5.52 0 10-4.48 10-10s-4.48-10-10-10z"/><text x="12" y="15.5" font-size="11" font-weight="700" text-anchor="middle" fill="currentColor" stroke="none">5</text></svg>',
  forward5: '<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><g transform="scale(-1,1) translate(-24,0)"><path d="M11.99 2C7.5 2 3.85 4.86 2.55 8.86H5.1c.98-2.68 3.57-4.61 6.55-4.61 3.87 0 7 3.13 7 7s-3.13 7-7 7c-2.98 0-5.57-1.93-6.55-4.61H2.55C3.85 19.14 7.5 22 11.99 22c5.52 0 10-4.48 10-10s-4.48-10-10-10z"/><text x="12" y="15.5" font-size="11" font-weight="700" text-anchor="middle" fill="currentColor" stroke="none">5</text></g></svg>',
  volume: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/></svg>',
  volumeX: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><line x1="22" x2="16" y1="9" y2="15"/><line x1="16" x2="22" y1="9" y2="15"/></svg>',
  chevronDown: '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>',
}

function icon(name) {
  const wrap = document.createElement('span')
  wrap.innerHTML = icons[name] || ''
  return wrap.firstChild
}

function fmtDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00'
  const s = Math.floor(seconds % 60)
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}:${String(s).padStart(2, '0')}`
  const h = Math.floor(m / 60)
  return `${h}:${String(m % 60).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

// ---------- playback ----------

function playVideo() {
  const p = video.play()
  if (p && typeof p.catch === 'function') {
    p.catch(() => {
      // Autoplay-policy rejection: keep the UI honest (paused).
      state.playing = false
      sync()
    })
  }
}

function loadKaraoke(k, index) {
  state.currentIndex = index
  state.current = k
  state.progress = 0
  state.duration = Number.isFinite(k.duration) ? k.duration : 0
  video.src = karaokeStreamUrl(k)
  video.playbackRate = state.rate
  playVideo()
  // Report the play (play counter).
  endpoints.registerKaraokePlay(k.id).catch(() => undefined)
  updateTopTitle()
  sync()
}

// Shows the current karaoke title in the player's top bar.
function updateTopTitle() {
  if (!overlayEl) return
  const titleEl = overlayEl.querySelector('#karaoke-top-title')
  if (titleEl && state.current) {
    titleEl.textContent = state.current.title + (state.current.artist ? ` — ${state.current.artist}` : '')
  }
}

export function playKaraokeContext(list, index) {
  const k = list[index]
  if (!k) return
  state.list = list
  open()
  loadKaraoke(k, index)
}

export function playKaraoke(k) {
  playKaraokeContext([k], 0)
}

export function getKaraokeState() {
  return state
}

export function isOpen() {
  return !!overlayEl
}

export function next() {
  const { list, currentIndex } = state
  if (list.length === 0) return
  const idx = currentIndex + 1
  if (idx >= list.length) return // end of queue: stop here
  loadKaraoke(list[idx], idx)
}

export function prev() {
  const { list, currentIndex } = state
  if (list.length === 0) return
  if (state.progress > 3) {
    video.currentTime = 0
    state.progress = 0
    sync()
    return
  }
  const idx = currentIndex > 0 ? currentIndex - 1 : 0
  loadKaraoke(list[idx], idx)
}

export function togglePlay() {
  if (!state.current) return
  if (video.paused) {
    // Restart from the beginning when the video is at (or past) its end:
    // play() alone is a no-op in some browsers once currentTime reaches
    // the duration (either after a natural 'ended' or a manual seek).
    const dur = Number.isFinite(video.duration) ? video.duration : state.duration
    if (video.ended || (dur > 0 && video.currentTime >= dur - 0.05)) video.currentTime = 0
    playVideo()
  } else {
    video.pause()
  }
}

export function seek(seconds) {
  const total = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : state.duration
  if (total <= 0) return
  const target = Math.min(Math.max(seconds, 0), total)
  if (video.readyState > 0) video.currentTime = target
  state.progress = target
  sync()
}

export function seekBy(delta) {
  if (Number.isFinite(video.currentTime)) seek(video.currentTime + delta)
}

export function setVolume(v) {
  video.volume = v
  state.volume = v
  sync()
}

export function toggleMute() {
  video.volume = video.volume === 0 ? 0.8 : 0
  state.volume = video.volume
  sync()
}

export function setRate(rate) {
  video.playbackRate = rate
  state.rate = rate
  sync()
}

export function close() {
  video.pause()
  video.removeAttribute('src')
  video.load()
  state.list = []
  state.currentIndex = -1
  state.current = null
  state.playing = false
  if (overlayEl) {
    overlayEl.remove()
    overlayEl = null
  }
}

// ---------- overlay ----------

let overlayEl = null
let barRefs = {}

function open() {
  if (overlayEl) return
  overlayEl = el(
    'div',
    { class: 'karaoke-player' },
    el(
      'div',
      { class: 'karaoke-top' },
      el('button', { class: 'icon-btn', 'aria-label': 'Fechar', onclick: () => close() }, icon('chevronDown')),
      el('div', { class: 'karaoke-top-title', id: 'karaoke-top-title' }, ''),
    ),
    el('div', { class: 'karaoke-stage' }, video),
    el('div', { class: 'karaoke-controls' }, buildControls()),
  )
  document.body.append(overlayEl)
  renderControls()
}

// Rebuilds the control bar on structural changes; progress ticks update in
// place via sync().
function buildControls() {
  const fill = el('div', { class: 'progress-fill' })
  const cur = el('span', { class: 'progress-time' }, '0:00')
  const dur = el('span', { class: 'progress-time' }, '0:00')
  barRefs = { fill, cur, dur, track: null, rateBtn: null }

  const progressTrack = el(
    'div',
    { class: 'progress-track', role: 'slider', 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-valuenow': 0 },
    fill,
  )
  barRefs.track = progressTrack
  attachSeek(progressTrack)

  const rateBtn = el(
    'button',
    { class: 'player-btn karaoke-rate', 'aria-label': 'Velocidade', onclick: cycleRate },
    `${state.rate.toFixed(2).replace(/0$/, '')}x`,
  )
  barRefs.rateBtn = rateBtn

  const volIcon = el('span', { class: 'vol-icon' }, state.volume === 0 ? icon('volumeX') : icon('volume'))
  barRefs.volIcon = volIcon
  const volInput = el('input', {
    class: 'volume-slider',
    type: 'range',
    min: 0,
    max: 1,
    step: 0.01,
    value: state.volume,
    'aria-label': 'Volume',
    oninput: (e) => setVolume(parseFloat(e.target.value)),
  })
  barRefs.volInput = volInput

  return el(
    'div',
    { class: 'karaoke-controls-inner' },
    el(
      'div',
      { class: 'player-buttons' },
      el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: prev }, icon('prev')),
      el('button', { class: 'player-btn', 'aria-label': 'Retroceder 5 segundos', onclick: () => seekBy(-5) }, icon('rewind5')),
      el('button', { class: 'player-btn-main', 'aria-label': 'Tocar', 'aria-pressed': 'false', onclick: togglePlay }, icon('play')),
      el('button', { class: 'player-btn', 'aria-label': 'Avançar 5 segundos', onclick: () => seekBy(5) }, icon('forward5')),
      el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: next }, icon('next')),
      rateBtn,
    ),
    el(
      'div',
      { class: 'player-progress' },
      cur,
      progressTrack,
      dur,
    ),
    el(
      'div',
      { class: 'player-volume' },
      el('button', { class: 'icon-btn', 'aria-label': 'Mudo', onclick: toggleMute }, volIcon),
      volInput,
    ),
  )
}

const rates = [1, 1.25, 1.5, 2, 0.75, 0.5]
function cycleRate() {
  const cur = state.rate
  const idx = rates.indexOf(cur)
  setRate(rates[(idx + 1) % rates.length])
}

function attachSeek(track) {
  let dragging = false
  const clientXToSeek = (clientX) => {
    const rect = track.getBoundingClientRect()
    if (rect.width === 0) return
    const total = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : state.duration
    if (total <= 0) return
    const ratio = Math.min(Math.max((clientX - rect.left) / rect.width, 0), 1)
    seek(ratio * total)
  }
  track.addEventListener('pointerdown', (e) => {
    if (e.pointerType === 'mouse' && e.button !== 0) return
    dragging = true
    track.classList.add('dragging')
    clientXToSeek(e.clientX)
    e.preventDefault()
  })
  window.addEventListener('pointermove', (e) => {
    if (dragging) clientXToSeek(e.clientX)
  })
  window.addEventListener('pointerup', () => {
    dragging = false
    track.classList.remove('dragging')
  })
}

// Rebuilds the control bar (structural changes: play state, volume icon).
function renderControls() {
  if (!overlayEl) return
  const controls = overlayEl.querySelector('.karaoke-controls')
  controls.innerHTML = ''
  controls.append(buildControls())
  updateTopTitle()
}

// In-place updates for progress/volume while playing (no rebuild).
function sync() {
  if (!overlayEl) return
  const total = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : state.duration
  const prog = Number.isFinite(video.currentTime) ? video.currentTime : state.progress
  const pct = total > 0 ? (prog / total) * 100 : 0
  if (barRefs.fill) barRefs.fill.style.width = `${pct}%`
  if (barRefs.cur) barRefs.cur.textContent = fmtDuration(prog)
  if (barRefs.dur) barRefs.dur.textContent = fmtDuration(total)
  if (barRefs.track) {
    barRefs.track.setAttribute('aria-valuenow', String(Math.round(prog)))
    barRefs.track.setAttribute('aria-valuemax', String(Math.round(total)))
  }
  if (barRefs.volIcon) {
    const want = state.volume === 0 ? 'volumeX' : 'volume'
    const cur = barRefs.volIcon.firstElementChild?.dataset?.icon ?? ''
    if (cur !== want) {
      barRefs.volIcon.innerHTML = icons[want]
      const svg = barRefs.volIcon.firstElementChild
      if (svg) svg.dataset.icon = want
    }
  }
  if (barRefs.volInput && Math.abs(parseFloat(barRefs.volInput.value) - state.volume) > 0.005) {
    barRefs.volInput.value = String(state.volume)
  }
  // Speed label follows the current rate.
  if (barRefs.rateBtn) {
    barRefs.rateBtn.textContent = `${state.rate.toFixed(2).replace(/0$/, '')}x`
  }
  // Play/pause button icon.
  const mainBtn = overlayEl.querySelector('.player-btn-main')
  if (mainBtn) {
    const want = state.playing ? 'pause' : 'play'
    mainBtn.innerHTML = icons[want]
    mainBtn.setAttribute('aria-label', state.playing ? 'Pausar' : 'Tocar')
    mainBtn.setAttribute('aria-pressed', String(state.playing))
  }
}

// ---------- video events ----------

video.addEventListener('timeupdate', () => {
  if (Number.isFinite(video.currentTime) && video.currentTime >= 0) {
    state.progress = video.currentTime
  }
  sync()
})
video.addEventListener('loadedmetadata', () => {
  state.duration = video.duration
  state.progress = 0
  sync()
})
video.addEventListener('play', () => {
  state.playing = true
  sync()
})
video.addEventListener('pause', () => {
  state.playing = false
  sync()
})
video.addEventListener('ended', () => {
  if (state.currentIndex + 1 < state.list.length) next()
  else sync()
})

// ---------- keyboard shortcuts (only while open) ----------

document.addEventListener('keydown', (e) => {
  if (!overlayEl) return
  if (e.altKey || e.ctrlKey || e.metaKey) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.tagName === 'BUTTON')) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    return
  }
  if (!state.current) return
  switch (e.key) {
    case ' ':
      e.preventDefault()
      togglePlay()
      break
    case 'ArrowLeft':
      e.preventDefault()
      seekBy(-5)
      break
    case 'ArrowRight':
      e.preventDefault()
      seekBy(5)
      break
    case 'ArrowUp':
      e.preventDefault()
      setVolume(Math.min(1, state.volume + 0.1))
      break
    case 'ArrowDown':
      e.preventDefault()
      setVolume(Math.max(0, state.volume - 0.1))
      break
  }
})

// Debug/test hook.
window.__karaoke = { getState: () => state, isOpen, close }
