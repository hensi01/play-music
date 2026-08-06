// Player singleton: queue, playback, seek, shuffle/repeat, transcoding
// fallback and the Media Session API integration.

import { endpoints, streamUrl, artworkUrl } from './api.js'

const audio = new Audio()
audio.preload = 'auto'

let fallbackUsed = false
// True while the audio element is being handed a new track (src swap). The
// browser fires a `pause` event during the media load algorithm, so those
// events must not overwrite the real play state.
let switching = false

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
  if (Number.isFinite(audio.currentTime) && audio.currentTime >= 0) {
    set({ progress: audio.currentTime })
  }
})

const updateDuration = () => set({ duration: resolvedTrackDuration() })
audio.addEventListener('loadedmetadata', updateDuration)
audio.addEventListener('durationchange', updateDuration)
audio.addEventListener('play', () => {
  switching = false
  set({ playing: true })
})
audio.addEventListener('pause', () => {
  if (switching) {
    // Consumed the pause fired by the media load algorithm during a src swap.
    // Do not let it flip the UI to paused; the next `play` event re-syncs.
    switching = false
    return
  }
  set({ playing: false })
})
audio.addEventListener('ended', () => {
  if (state.repeat) {
    audio.currentTime = 0
    void audio.play()
  } else {
    next()
  }
})

audio.addEventListener('error', () => {
  switching = false
  if (!state.current) return
  if (!fallbackUsed) {
    fallbackUsed = true
    audio.src = streamUrl(state.current, true)
    void audio.play()
  } else {
    fallbackUsed = false
    set({ playing: false })
  }
})

// Media Session API: system media controls + notifications.
function updateMediaSession() {
  if (!('mediaSession' in navigator) || !state.current) return
  navigator.mediaSession.metadata = new MediaMetadata({
    title: state.current.title,
    artist: state.current.artist,
    album: state.current.album,
    artwork: [{ src: artworkUrl(state.current.id, 512), sizes: '512x512' }],
  })
  navigator.mediaSession.setActionHandler('play', () => togglePlay())
  navigator.mediaSession.setActionHandler('pause', () => togglePlay())
  navigator.mediaSession.setActionHandler('previoustrack', () => prev())
  navigator.mediaSession.setActionHandler('nexttrack', () => next())
  navigator.mediaSession.setActionHandler('seekto', (d) => {
    if (d.seekTime != null) seek(d.seekTime)
  })
}

// Keeps the system media controls in sync with the audio element, clamping the
// position so it can never exceed the duration (avoids setPositionState errors).
function updateMediaSessionPosition() {
  if (!('mediaSession' in navigator) || !state.current) return
  const duration = resolveDuration(audio.duration, state.current.duration ?? 0)
  if (duration <= 0 || !Number.isFinite(audio.currentTime)) return
  const position = Math.min(Math.max(audio.currentTime, 0), duration)
  try {
    navigator.mediaSession.setPositionState({
      duration,
      position,
      playbackRate: audio.playbackRate,
    })
  } catch {
    /* position state not supported */
  }
}

function loadAndPlay(song) {
  fallbackUsed = false
  switching = true
  set({ progress: 0, duration: resolveDuration(song.duration), playing: true })
  audio.src = streamUrl(song)
  void audio.play()
  // Report the play so play counts and history update.
  endpoints.registerPlay(song.id).catch(() => undefined)
}

export function playContext(songs, index) {
  const song = songs[index]
  if (!song) return
  set({ queue: songs, currentIndex: index, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function playSong(song) {
  set({ queue: [song], currentIndex: 0, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function togglePlay() {
  if (!state.current) return
  if (state.playing) audio.pause()
  else void audio.play()
}

export function next() {
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
      switching = false
      set({ playing: false, currentIndex: queue.length, progress: 0 })
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
  const { queue, currentIndex, progress } = state
  if (queue.length === 0) return
  // Restart the current track if it has been playing for a while.
  if (progress > 3) {
    audio.currentTime = 0
    return
  }
  const idx = currentIndex > 0 ? currentIndex - 1 : 0
  const song = queue[idx]
  set({ currentIndex: idx, current: song, progress: 0 })
  loadAndPlay(song)
  updateMediaSession()
}

export function seek(seconds) {
  const duration = resolveDuration(audio.duration, state.duration, state.current?.duration ?? 0)
  if (duration === 0 || !Number.isFinite(seconds)) return
  const target = Math.min(Math.max(seconds, 0), duration)
  audio.currentTime = target
  set({ progress: target })
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
