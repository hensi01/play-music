import { api, endpoints, streamUrl, artworkUrl, tokenUserId } from './api.js'
import { createListenTracker, sampleListen } from './listening.js'
import { showAd } from './ads.js'

const audio = new Audio()
audio.preload = 'auto'
audio.volume = 0.8

let listenTracker = null
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

// The media load algorithm fires `pause` while a new src is loading, so those
// events must not overwrite the real play state.
let switching = false
let adGate = false, adPlayer = null, adGeneration = 0, songsSinceAd = 0, adIndex = 0, gateOverlay = null

async function afterSong() {
  if (adGate) return
  const hasNext = state.repeat || state.currentIndex + 1 < state.queue.length || (state.shuffle && state.queue.length > 1)
  if (!hasNext) { next(); return }
  adGate = true
  const generation = ++adGeneration
  songsSinceAd++
  set({ playing: false })
  setMediaSessionPlaybackState(false)
  if ('mediaSession' in navigator) { try { navigator.mediaSession.setPositionState() } catch { /* unsupported */ } }
  const proceed = () => {
    if (generation !== adGeneration) return
    adGate = false; adPlayer = null; gateOverlay?.remove(); gateOverlay = null
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
      start()
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

function playAudio() {
  audio.play().catch(() => {
    // A src swap rejects the previous track's play() with AbortError; the
    // swap's own events settle state, and `switching` must stay set so the
    // load-algorithm pause is still consumed.
    if (!switching) {
      set({ playing: false })
      setMediaSessionPlaybackState(false)
    }
  })
}

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
  }
  setMediaSessionAction('play', () => { if (!adGate) playAudio() })
  setMediaSessionAction('pause', () => { if (!adGate) audio.pause() })
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

function setMediaSessionPlaybackState(playing) {
  const session = globalThis.navigator?.mediaSession
  if (!session) return
  try {
    session.playbackState = state.current ? (playing ? 'playing' : 'paused') : 'none'
  } catch {
    /* not supported */
  }
}

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

export function addToQueue(song) {
  if (!song?.id) return
  set({ queue: [...state.queue, song] })
}

export function clearSession() {
  adGeneration++; adGate = false; adPlayer?.cleanup(); adPlayer = null; gateOverlay?.remove(); gateOverlay = null; songsSinceAd = 0; adIndex = 0
  listenTracker = null
  switching = false
  audio.pause()
  audio.removeAttribute('src')
  audio.load()
  set({ queue: [], currentIndex: -1, current: null, playing: false, progress: 0, duration: 0, pendingSeek: null, fullScreen: false, shuffle: false, repeat: false })
  clearMediaSession()
}

export function togglePlay() {
  if (adGate) return
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
      // Stopping the element matters: pausing only the UI would leave it
      // playing the last track.
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

setInterval(() => {
  if (state.playing) updateMediaSessionPosition()
}, 1000)

// The e2e specs drive the player through this handle; product code never reads
// it. Keep the shape in sync with e2e/tests (getState + the audio element).
window.__player = { getState: () => state, audio }
