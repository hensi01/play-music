// Karaoke video player — YouTube-style: controls overlaid ON the video
// (progress bar + buttons with gradient, auto-hide while playing), native
// fullscreen on open, fully independent from the audio player (player.js).

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
  // Modo de exibição atual: 'landscape' (paisagem) ou 'portrait' (vertical).
  orientation: 'landscape',
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
  max: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>',
  minimize: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3v3a2 2 0 0 1-2 2H3"/><path d="M21 8h-3a2 2 0 0 1-2-2V3"/><path d="M3 16h3a2 2 0 0 1 2 2v3"/><path d="M16 21v-3a2 2 0 0 1 2-2h3"/></svg>',
  // Ícone = a AÇÃO: smartphone deitado (rotateLandscape) indica "ir para
  // paisagem"; smartphone em pé (rotatePortrait) indica "ir para vertical".
  rotateLandscape: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="10" rx="2"/><path d="M6 12h.01"/></svg>',
  rotatePortrait: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="7" y="2" width="10" height="20" rx="2"/><path d="M12 18h.01"/></svg>',
}

function icon(name) {
  const wrap = document.createElement('span')
  wrap.innerHTML = icons[name] || ''
  const node = wrap.firstChild
  if (node) node.dataset.icon = name
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
  showControls()
  sync()
}

// Shows the current karaoke title in the player's top overlay.
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

// ---------- orientation (modo paisagem / vertical) ----------

// Trava a orientação da tela quando a API existe (Android Chrome). iOS e
// desktop não têm screen.orientation.lock: o botão alterna o estado visual
// normalmente, mas a rotação real fica a cargo do usuário/navegador — o
// mesmo comportamento silencioso do fullscreen nessas plataformas.
function lockOrientation(orientation) {
  const so = screen.orientation
  if (!so || typeof so.lock !== 'function') return
  const p = so.lock(orientation)
  if (p && typeof p.catch === 'function') {
    p.catch(() => {
      // Alguns navegadores recusam o lock (ex.: 'portrait' em tablets):
      // libera a trava para a UI nunca ficar presa num estado impossível.
      if (so.unlock) so.unlock()
    })
  }
}

export function toggleOrientation() {
  state.orientation = state.orientation === 'landscape' ? 'portrait' : 'landscape'
  lockOrientation(state.orientation)
  sync()
  showControls()
}

// Auto-orientação: quando o vídeo é horizontal (w > h), a tela gira para
// paisagem no Android (onde screen.orientation.lock existe); vídeo vertical
// gira para portrait. iOS/desktop: no-op silencioso (a API não existe). O
// botão manual continua prevalecendo até a próxima carga/mudança de
// resolução do vídeo.
function autoOrientFromVideo() {
  if (!video.videoWidth || !video.videoHeight) return
  const want = video.videoWidth > video.videoHeight ? 'landscape' : 'portrait'
  state.orientation = want
  lockOrientation(want)
  sync()
}

// ---------- native fullscreen ----------

function isFullscreen() {
  // iOS: document.webkitFullscreenElement is unreliable while the video is in
  // its native fullscreen; webkitDisplayingFullscreen is the source of truth.
  if (isIOS()) return !!video.webkitDisplayingFullscreen
  return !!(document.fullscreenElement || document.webkitFullscreenElement)
}

// Detects iOS Safari / iPadOS (which report as MacIntel with touch support).
function isIOS() {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
}

// requestFullscreen on Android Chrome can reject when called in the same tick
// the element is appended to the DOM (no rendered frame yet), failing
// silently — the video then stays inline. These attempts are staggered, all
// inside the transient user-activation window (~5s), so the first call that
// runs against a rendered element succeeds. Failures are logged so real
// device issues are diagnosable instead of swallowed.
function requestContainerFullscreen(el, attempt) {
  const req = el.requestFullscreen || el.webkitRequestFullscreen
  if (!req) return Promise.reject(new Error('container fullscreen API unavailable'))
  const p = req.call(el)
  if (p && typeof p.catch === 'function') {
    return p.catch((err) => {
      if (attempt >= 3) throw err
      return new Promise((resolve, reject) => {
        const step = attempt === 1 ? requestAnimationFrame : (attempt === 2 ? requestAnimationFrame : setTimeout)
        const delay = attempt === 3 ? 120 : 0
        const run = () => {
          requestContainerFullscreen(el, attempt + 1).then(resolve, reject)
        }
        if (delay) setTimeout(run, delay)
        else step(run)
      })
    })
  }
  return Promise.resolve(p)
}

// Enters the native browser fullscreen (F11-like). iOS Safari has no
// container fullscreen API for <div>; the video's own fullscreen (native iOS
// player — the same behaviour as YouTube on iPhone) is used, synchronously
// inside the user gesture so the activation is still valid.
function enterFullscreen(el) {
  if (isIOS()) {
    if (video.webkitEnterFullscreen) video.webkitEnterFullscreen()
    return
  }
  requestContainerFullscreen(el, 1).catch((err) => {
    // Rejected after all staggered attempts (policy, unsupported container…):
    // report it so real-device issues are not invisible, then try the video's
    // own fullscreen as a last resort.
    console.warn('[karaoke] fullscreen falhou:', err && err.message ? err.message : err)
    if (video.webkitEnterFullscreen) video.webkitEnterFullscreen()
  })
}

export function toggleFullscreen() {
  if (isFullscreen()) {
    const exit = document.exitFullscreen || document.webkitExitFullscreen
    if (exit) {
      const p = exit.call(document)
      if (p && typeof p.catch === 'function') p.catch(() => {})
    }
    return
  }
  if (!overlayEl) return
  enterFullscreen(overlayEl)
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
  // Best-effort: when closing from our own controls while still in native
  // fullscreen, leave it.
  if (document.fullscreenElement) {
    const exit = document.exitFullscreen || document.webkitExitFullscreen
    if (exit) {
      const p = exit.call(document)
      if (p && typeof p.catch === 'function') p.catch(() => {})
    }
  }
}

// ---------- player DOM (YouTube-style overlays) ----------

let overlayEl = null
let barRefs = {}
let hideTimer = null
let seekDragging = false

// Shows the overlays and schedules auto-hide (only while playing and not
// dragging the seek bar).
function showControls() {
  if (!overlayEl) return
  overlayEl.classList.remove('controls-hidden')
  clearTimeout(hideTimer)
  hideTimer = setTimeout(() => {
    if (overlayEl && state.playing && !seekDragging) {
      overlayEl.classList.add('controls-hidden')
    }
  }, 2500)
}

function open() {
  if (overlayEl) return
  overlayEl = el(
    'div',
    { class: 'karaoke-player' },
    el(
      'div',
      { class: 'karaoke-stage' },
      video,
      el(
        'div',
        { class: 'karaoke-overlay' },
        el(
          'div',
          { class: 'karaoke-top' },
          el('button', { class: 'icon-btn', 'aria-label': 'Fechar', onclick: close }, icon('chevronDown')),
          el('div', { class: 'karaoke-top-title', id: 'karaoke-top-title' }, ''),
        ),
        el('div', { class: 'karaoke-controls' }, buildControls()),
      ),
    ),
  )
  document.body.append(overlayEl)
  updateTopTitle()
  attachStageEvents()
  showControls()
  enterFullscreen(overlayEl)
}

// Click on the video (outside the control overlays) toggles play/pause;
// any pointer activity shows the overlays and restarts the auto-hide timer.
function attachStageEvents() {
  if (!overlayEl) return
  overlayEl.addEventListener('click', (e) => {
    if (e.target.closest('.karaoke-overlay')) return
    togglePlay()
  })
  const poke = () => showControls()
  overlayEl.addEventListener('pointermove', poke)
  overlayEl.addEventListener('pointerdown', poke)
  overlayEl.addEventListener('touchstart', poke)
}

// Formats the playback rate label: 1 -> "1x", 1.25 -> "1.25x", 2 -> "2x".
function rateLabel(rate) {
  return `${Number(rate.toFixed(2))}x`
}

function buildControls() {
  const fill = el('div', { class: 'progress-fill' })
  const cur = el('span', { class: 'progress-time' }, '0:00')
  const dur = el('span', { class: 'progress-time' }, '0:00')
  barRefs = { fill, cur, dur, track: null, rateBtn: null, fsBtn: null, rotateBtn: null, volIcon: null, volInput: null }

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
    rateLabel(state.rate),
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

  const fsBtn = el(
    'button',
    { class: 'icon-btn karaoke-fs', 'aria-label': isFullscreen() ? 'Sair da tela cheia' : 'Tela cheia', onclick: toggleFullscreen },
    icon(isFullscreen() ? 'minimize' : 'max'),
  )
  barRefs.fsBtn = fsBtn

  const rotateBtn = el(
    'button',
    { class: 'icon-btn karaoke-rotate', 'aria-label': state.orientation === 'landscape' ? 'Alternar para modo vertical' : 'Alternar para modo paisagem', onclick: toggleOrientation },
    icon(state.orientation === 'landscape' ? 'rotatePortrait' : 'rotateLandscape'),
  )
  barRefs.rotateBtn = rotateBtn

  return el(
    'div',
    { class: 'karaoke-controls-inner' },
    el(
      'div',
      { class: 'player-progress' },
      cur,
      progressTrack,
      dur,
    ),
    el(
      'div',
      { class: 'player-buttons-row' },
      el(
        'div',
        { class: 'player-buttons' },
        el('button', { class: 'player-btn', 'aria-label': 'Anterior', onclick: prev }, icon('prev')),
        el('button', { class: 'player-btn', 'aria-label': 'Retroceder 5 segundos', onclick: () => seekBy(-5) }, icon('rewind5')),
        el('button', { class: 'player-btn-main', 'aria-label': 'Tocar', 'aria-pressed': 'false', onclick: togglePlay }, icon('play')),
        el('button', { class: 'player-btn', 'aria-label': 'Avançar 5 segundos', onclick: () => seekBy(5) }, icon('forward5')),
        el('button', { class: 'player-btn', 'aria-label': 'Próxima', onclick: next }, icon('next')),
      ),
      el(
        'div',
        { class: 'player-side' },
        rateBtn,
        el('button', { class: 'icon-btn', 'aria-label': 'Mudo', onclick: toggleMute }, volIcon),
        volInput,
        rotateBtn,
        fsBtn,
      ),
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
    seekDragging = true
    track.classList.add('dragging')
    clientXToSeek(e.clientX)
    e.preventDefault()
    e.stopPropagation()
  })
  window.addEventListener('pointermove', (e) => {
    if (seekDragging) clientXToSeek(e.clientX)
  })
  window.addEventListener('pointerup', () => {
    if (!seekDragging) return
    seekDragging = false
    track.classList.remove('dragging')
    showControls()
  })
}

// In-place updates for progress/volume/icons while playing (no rebuild).
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
    barRefs.rateBtn.textContent = rateLabel(state.rate)
  }
  // Fullscreen button icon follows the actual fullscreen state.
  if (barRefs.fsBtn) {
    const inFs = isFullscreen()
    barRefs.fsBtn.innerHTML = icons[inFs ? 'minimize' : 'max']
    const svg = barRefs.fsBtn.firstElementChild
    if (svg) svg.dataset.icon = inFs ? 'minimize' : 'max'
    barRefs.fsBtn.setAttribute('aria-label', inFs ? 'Sair da tela cheia' : 'Tela cheia')
  }
  // Orientation button icon follows the current orientation (icon = action).
  if (barRefs.rotateBtn) {
    const inLandscape = state.orientation === 'landscape'
    const want = inLandscape ? 'rotatePortrait' : 'rotateLandscape'
    const cur = barRefs.rotateBtn.firstElementChild?.dataset?.icon ?? ''
    if (cur !== want) {
      barRefs.rotateBtn.innerHTML = icons[want]
      const svg = barRefs.rotateBtn.firstElementChild
      if (svg) svg.dataset.icon = want
    }
    barRefs.rotateBtn.setAttribute('aria-label', inLandscape ? 'Alternar para modo vertical' : 'Alternar para modo paisagem')
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
  autoOrientFromVideo()
  sync()
})
// Alguns streams (adaptativos/HLS) mudam de resolução durante a reprodução:
// re-aplica a auto-orientação quando as dimensões intrínsecas mudam.
video.addEventListener('resize', autoOrientFromVideo)
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

// Native fullscreen state changes: leaving fullscreen NEVER closes the player
// (only the close button does); we just resync the button icon. iOS Safari
// fires webkitfullscreenchange on the video element instead of the document.
function onFullscreenChange() {
  sync()
}
document.addEventListener('fullscreenchange', onFullscreenChange)
document.addEventListener('webkitfullscreenchange', onFullscreenChange)
video.addEventListener('webkitfullscreenchange', onFullscreenChange)

// ---------- keyboard shortcuts (only while open) ----------

document.addEventListener('keydown', (e) => {
  if (!overlayEl) return
  if (e.altKey || e.ctrlKey || e.metaKey) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.tagName === 'BUTTON')) return
  if (e.key === 'Escape') {
    // Esc only exits native fullscreen (handled by the browser); the player
    // stays open — only the close button (or close()) closes it.
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
window.__karaoke = { getState: () => state, isOpen, close, toggleFullscreen, toggleOrientation }
