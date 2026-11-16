// Player singleton: queue, playback, seek, shuffle/repeat and the Media
// Session API integration. Audio streams come exclusively from the Bunny CDN
// (the server 302s /api/stream to a signed CDN URL) — no retry ladder, no
// local/transcode fallback: a failed stream simply stops playback.

import { api, endpoints, streamUrl, artworkUrl, getToken } from './api.js'
import { createListenTracker, sampleListen } from './listening.js'
import { showAd } from './ads.js'

const audio = new Audio()
audio.preload = 'auto'
audio.volume = 0.8

let listenTracker = null
function tokenUserId() {
  try {
    const part = getToken().split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    return JSON.parse(atob(part.padEnd(Math.ceil(part.length / 4) * 4, '='))).uid || null
  } catch { return null }
}
function startListening(song) {
  const playId = globalThis.crypto?.randomUUID?.() || `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`
  listenTracker = createListenTracker(song.id, playId, tokenUserId())
}
function resetListenAnchor() { if (listenTracker) { listenTracker.wall = null; listenTracker.position = null } }
function sampleListening() {
  const duration = resolvedTrackDuration()
  if (sampleListen(listenTracker, { position: audio.currentTime, wall: performance.now(), playing: !audio.paused, seeking: audio.seeking, rate: audio.playbackRate, duration })) {
    if (listenTracker.userId && listenTracker.userId === tokenUserId()) endpoints.qualifiedListen(listenTracker.songId, listenTracker.playId).catch(() => undefined)
  }
}
audio.addEventListener('playing', sampleListening)
audio.addEventListener('seeking', resetListenAnchor)
audio.addEventListener('seeked', resetListenAnchor)

// True while the audio element is being handed a new track (src swap). The
// browser fires a `pause` event during the media load algorithm, so those
// events must not overwrite the real play state.
let switching = false
let adGate = false, adPlayer = null, adGeneration = 0, songsSinceAd = 0, adIndex = 0, gateOverlay = null, gatePaused = false, pendingGateStart = null

function pauseAdGate() { gatePaused = true; adPlayer?.pause() }
function resumeAdGate() { gatePaused = false; if (adPlayer) adPlayer.resume(); else if (pendingGateStart) { const start = pendingGateStart; pendingGateStart = null; start() } }

async function afterSong() {
  if (adGate) return
  const hasNext = state.repeat || state.currentIndex + 1 < state.queue.length || (state.shuffle && state.queue.length > 1)
  if (!hasNext) { next(); return }
  adGate = true
  gatePaused = false; pendingGateStart = null
  const generation = ++adGeneration
  songsSinceAd++
  set({ playing: false })
  setMediaSessionPlaybackState(false)
  if ('mediaSession' in navigator) { try { navigator.mediaSession.setPositionState() } catch { /* unsupported */ } }
  const proceed = () => {
    if (generation !== adGeneration) return
    adGate = false; adPlayer = null; pendingGateStart = null; gateOverlay?.remove(); gateOverlay = null
    if (state.repeat) { if (state.current) startListening(state.current); audio.currentTime = 0; playAudio() }
    else next()
    updateMediaSession()
  }
  gateOverlay = document.createElement('section'); gateOverlay.className = 'ad-overlay'
  gateOverlay.setAttribute('aria-label', 'Publicidade')
  const message = document.createElement('p'); message.setAttribute('role', 'status'); message.textContent = 'Preparando próxima música…'
  const retry = document.createElement('button'); retry.className = 'btn-primary'; retry.textContent = 'Tentar novamente'; retry.hidden = true
  gateOverlay.append(message, retry); document.body.append(gateOverlay)
  const load = async () => {
    retry.hidden = true; message.textContent = 'Preparando próxima música…'
    try {
      const config = await api.fetch('/api/ads', { signal: AbortSignal.timeout(15000) })
      if (generation !== adGeneration) return
      const start = () => {
        if (generation !== adGeneration) return
        if (!config.ads?.length || songsSinceAd < config.frequency) { proceed(); return }
        const ad = config.ads[adIndex++ % config.ads.length]; songsSinceAd = 0
        gateOverlay.remove(); gateOverlay = null
        adPlayer = showAd(ad, proceed, setMediaSessionPlaybackState)
        if ('mediaSession' in navigator && typeof globalThis.MediaMetadata === 'function') {
          try { navigator.mediaSession.metadata = new MediaMetadata({ title: 'Publicidade', artist: ad.name }) } catch { /* unsupported */ }
        }
      }
      if (gatePaused) { pendingGateStart = start; message.textContent = 'Reprodução pausada.'; retry.textContent = 'Continuar'; retry.hidden = false; retry.onclick = resumeAdGate }
      else start()
    } catch {
      if (generation !== adGeneration) return
      message.textContent = 'Não foi possível carregar a publicidade.'; retry.textContent = 'Tentar novamente'; retry.onclick = load; retry.hidden = false
    }
  }
  retry.onclick = load
  await load()
}

export function resolveDuration(...candidates) {
  return candidates.find((value) => Number.isFinite(value) && value > 0) ?? 0
}

function resolvedTrackDuration() {
  return resolveDuration(audio.duration, state.current?.duration ?? 0)
}

const state = {
  queue: [],
  currentIndex: -1,
  current: null,
  playing: false,
  progress: 0,
  duration: 0,
  volume: 0.8,
  shuffle: false,
  repeat: false,
  fullScreen: false,
  // Seek requested before the metadata (duration) was available; applied as
  // soon as the duration loads.
  pendingSeek: null,
}

const listeners = new Set()

function emit() {
  for (const fn of listeners) fn(state)
}

export function subscribe(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export function getPlayerState() {
  return state
}

function set(patch) {
  Object.assign(state, patch)
  emit()
}

audio.addEventListener('timeupdate', () => {
  sampleListening()
  if (Number.isFinite(audio.currentTime) && audio.currentTime >= 0) {
    set({ progress: audio.currentTime })
  }
})

const updateDuration = () => {
  set({ duration: resolvedTrackDuration() })
  // Apply a seek that arrived before the metadata was available.
  if (state.pendingSeek != null) seek(state.pendingSeek)
  updateMediaSessionPosition()
}
audio.addEventListener('loadedmetadata', updateDuration)
audio.addEventListener('durationchange', updateDuration)
audio.addEventListener('canplay', updateDuration)
audio.addEventListener('play', () => {
  if (!state.current) return
  switching = false
  set({ playing: true })
  setMediaSessionPlaybackState(true)
  updateMediaSessionPosition()
})
audio.addEventListener('pause', () => {
  resetListenAnchor()
  if (switching) {
    // Consumed the pause fired by the media load algorithm during a src swap.
    // Do not let it flip the UI to paused; the next `play` event re-syncs.
    switching = false
    return
  }
  set({ playing: false })
  setMediaSessionPlaybackState(false)
  updateMediaSessionPosition()
})
audio.addEventListener('ended', () => {
  afterSong()
})

audio.addEventListener('error', () => {
  switching = false
  if (!state.current) return
  // CDN-only: there is no fallback ladder. A failed stream stops playback;
  // the UI stays honest (paused) and the next track/user action retries.
  set({ playing: false })
  setMediaSessionPlaybackState(false)
})

// Wraps audio.play() so autoplay-policy rejections (iOS/Android) never turn
// into unhandled promise rejections; the UI state stays honest (paused).
function playAudio() {
  const p = audio.play()
  if (p && typeof p.catch === 'function') {
    p.catch(() => {
      // During a src swap the previous track's play() promise rejects with an
      // AbortError; ignore it (the swap's own pause/play events settle state),
      // and never clear `switching` here or the load-algorithm pause event
      // would stop being consumed — desyncing the UI from the audio element.
      if (!switching) {
        set({ playing: false })
        setMediaSessionPlaybackState(false)
      }
    })
  }
}

// Media Session API: system media controls + notifications.
function updateMediaSession() {
  const session = globalThis.navigator?.mediaSession
  if (!session || !state.current) return
  try {
    if (typeof globalThis.MediaMetadata === 'function') {
      session.metadata = new MediaMetadata({
        title: state.current.title,
        artist: state.current.artist,
        album: state.current.album,
        artwork: [{ src: artworkUrl(state.current.id, 512), sizes: '512x512' }],
      })
    }
  } catch {
    // Metadata support differs by browser; playback controls can still work.
  }
  setMediaSessionAction('play', () => { if (adGate) resumeAdGate(); else playAudio() })
  setMediaSessionAction('pause', () => { if (adGate) pauseAdGate(); else audio.pause() })
  setMediaSessionAction('previoustrack', () => prev())
  setMediaSessionAction('nexttrack', () => next())
  setMediaSessionAction('seekbackward', (d) => {
    const delta = (d && d.seekOffset) || 10
    seek(audio.currentTime - delta)
  })
  setMediaSessionAction('seekforward', (d) => {
    const delta = (d && d.seekOffset) || 10
    seek(audio.currentTime + delta)
  })
  setMediaSessionAction('seekto', (d) => {
    if (d.seekTime != null) seek(d.seekTime)
  })
  setMediaSessionPlaybackState(state.playing)
  updateMediaSessionPosition()
}

function setMediaSessionAction(action, handler) {
  const session = globalThis.navigator?.mediaSession
  if (!session || typeof session.setActionHandler !== 'function') return
  try {
    session.setActionHandler(action, handler)
  } catch {
    // Ignore individual actions that the current browser does not support.
  }
}

function clearMediaSession() {
  const session = globalThis.navigator?.mediaSession
  if (!session) return
  for (const action of ['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto']) {
    setMediaSessionAction(action, null)
  }
  try { session.metadata = null } catch { /* unsupported */ }
  try { session.playbackState = 'none' } catch { /* unsupported */ }
}

// Syncs the lock-screen/notification play state (iOS/Android).
function setMediaSessionPlaybackState(playing) {
  const session = globalThis.navigator?.mediaSession
  if (!session) return
  try {
    session.playbackState = state.current ? (playing ? 'playing' : 'paused') : 'none'
  } catch {
    /* not supported */
  }
}

// Keeps the system media controls in sync with the audio element, clamping the
// position so it can never exceed the duration (avoids setPositionState errors).
function updateMediaSessionPosition() {
  const session = globalThis.navigator?.mediaSession
  if (!session || typeof session.setPositionState !== 'function' || !state.current) return
  const duration = resolveDuration(audio.duration, state.duration, state.current.duration ?? 0)
  if (duration <= 0 || !Number.isFinite(audio.currentTime)) return
  const position = Math.min(Math.max(audio.currentTime, 0), duration)
  try {
    session.setPositionState({
      duration,
      position,
      playbackRate: audio.playbackRate,
    })
  } catch {
    /* position state not supported */
  }
}

function loadAndPlay(song) {
  startListening(song)
  switching = true
  state.pendingSeek = null
  set({ progress: 0, duration: resolveDuration(song.duration), playing: true })
  setMediaSessionPlaybackState(true)
  audio.src = streamUrl(song)
  playAudio()
  // Report the play so play counts and history update.
  endpoints.registerPlay(song.id).catch(() => undefined)
}

export function playContext(songs, index) {
  if (adGate) return
  const song = songs[index]
  if (!song) return
  set({ queue: songs, currentIndex: index, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function playSong(song) {
  if (adGate) return
  set({ queue: [song], currentIndex: 0, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

// Add a track after the current queue without interrupting playback.
export function addToQueue(song) {
  if (!song?.id) return
  set({ queue: [...state.queue, song] })
}

// Clear playback and listening state when the account changes.
export function clearSession() {
  adGeneration++; adGate = false; adPlayer?.cleanup(); adPlayer = null; gateOverlay?.remove(); gateOverlay = null; songsSinceAd = 0; adIndex = 0; gatePaused = false; pendingGateStart = null
  listenTracker = null
  switching = false
  audio.pause()
  audio.removeAttribute('src')
  audio.load()
  set({ queue: [], currentIndex: -1, current: null, playing: false, progress: 0, duration: 0, pendingSeek: null, fullScreen: false, shuffle: false, repeat: false })
  clearMediaSession()
}

export function togglePlay() {
  if (adGate) { if (adPlayer) adPlayer.toggle(); else if (gatePaused) resumeAdGate(); else pauseAdGate(); return }
  if (!state.current) return
  if (state.playing) audio.pause()
  else playAudio()
}

export function next() {
  if (adGate) return
  const { queue, currentIndex, shuffle } = state
  if (queue.length === 0) return
  let idx = currentIndex + 1
  if (shuffle && queue.length > 1) {
    do {
      idx = Math.floor(Math.random() * queue.length)
    } while (idx === currentIndex)
  }
  if (idx >= queue.length) {
    if (!state.repeat) {
      // End of queue: actually stop the audio, otherwise the element keeps
      // playing the last track while the UI state reports paused.
      audio.pause()
      if (audio.readyState > 0) {
        try { audio.currentTime = 0 } catch { /* duration may no longer be seekable */ }
      }
      switching = false
      set({ playing: false, currentIndex: Math.max(0, queue.length - 1), progress: 0 })
      setMediaSessionPlaybackState(false)
      updateMediaSessionPosition()
      return
    }
    idx = 0
  }
  const song = queue[idx]
  set({ currentIndex: idx, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function prev() {
  if (adGate) return
  const { queue, currentIndex, progress } = state
  if (queue.length === 0) return
  // Restart the current track if it has been playing for a while.
  if (progress > 3) {
    if (audio.readyState > 0) {
      audio.currentTime = 0
    }
    set({ progress: 0 })
    return
  }
  const idx = currentIndex > 0 ? currentIndex - 1 : 0
  const song = queue[idx]
  set({ currentIndex: idx, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function seek(seconds) {
  if (adGate) return
  if (!Number.isFinite(seconds)) return
  const duration = resolveDuration(audio.duration, state.duration, state.current?.duration ?? 0)
  if (duration === 0 || audio.readyState === 0) {
    // Metadata not loaded yet (or the stream reports no duration): use the
    // seekable range as a last resort, otherwise defer until metadata loads.
    let seekable = 0
    try {
      const sr = audio.seekable
      if (sr && sr.length > 0) seekable = sr.end(sr.length - 1)
    } catch {
      /* not available */
    }
    if (seekable > 0 && audio.readyState > 0) {
      const target = Math.min(Math.max(seconds, 0), seekable)
      audio.currentTime = target
      set({ progress: target, duration: seekable })
      state.pendingSeek = null
      updateMediaSessionPosition()
      return
    }
    state.pendingSeek = seconds
    set({ progress: Math.max(seconds, 0) })
    return
  }
  state.pendingSeek = null
  const target = Math.min(Math.max(seconds, 0), duration)
  if (audio.readyState > 0) {
    audio.currentTime = target
  }
  set({ progress: target })
  updateMediaSessionPosition()
}

// Nudge playback by delta seconds (e.g. the +5s / -5s buttons and keyboard).
export function seekBy(delta) {
  if (Number.isFinite(audio.currentTime)) seek(audio.currentTime + delta)
}

export function setVolume(v) {
  audio.volume = v
  set({ volume: v })
}

export function toggleShuffle() {
  set({ shuffle: !state.shuffle })
}

export function toggleRepeat() {
  set({ repeat: !state.repeat })
}

export function setFullScreen(v) {
  set({ fullScreen: v })
}

export function setLiked(liked) {
  const { current, queue, currentIndex } = state
  if (!current) return
  const updated = { ...current, liked }
  const newQueue = queue.map((s, i) => (i === currentIndex ? updated : s))
  set({ current: updated, queue: newQueue })
}

// Periodically sync the Media Session position state while playing.
setInterval(() => {
  if (state.playing) updateMediaSessionPosition()
}, 1000)

// Debug/test hook: exposes the player state and audio element.
window.__player = { getState: () => state, audio }
