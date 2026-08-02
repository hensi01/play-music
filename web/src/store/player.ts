import { create } from 'zustand'
import { endpoints, streamUrl } from '../api'
import type { Song } from '../api/types'

export interface PlayerState {
  queue: Song[]
  currentIndex: number
  current: Song | null
  playing: boolean
  progress: number
  duration: number
  volume: number
  shuffle: boolean
  repeat: boolean
  fullScreen: boolean

  playContext: (songs: Song[], index: number) => void
  playSong: (song: Song) => void
  togglePlay: () => void
  next: () => void
  prev: () => void
  seek: (seconds: number) => void
  setVolume: (v: number) => void
  toggleShuffle: () => void
  toggleRepeat: () => void
  setFullScreen: (v: boolean) => void
  setLiked: (liked: boolean) => void
}

const audio = new Audio()
audio.preload = 'auto'

let fallbackUsed = false

function bindAudio() {
  audio.addEventListener('timeupdate', () => {
    usePlayer.setState({ progress: audio.currentTime })
  })
  audio.addEventListener('durationchange', () => {
    usePlayer.setState({ duration: audio.duration || 0 })
  })
  audio.addEventListener('play', () => usePlayer.setState({ playing: true }))
  audio.addEventListener('pause', () => usePlayer.setState({ playing: false }))
  audio.addEventListener('ended', () => {
    const { repeat } = usePlayer.getState()
    if (repeat) {
      audio.currentTime = 0
      void audio.play()
    } else {
      usePlayer.getState().next()
    }
  })
  audio.addEventListener('error', () => {
    const { current } = usePlayer.getState()
    if (!current) return
    if (!fallbackUsed) {
      fallbackUsed = true
      audio.src = streamUrl(current, true)
      void audio.play()
    } else {
      fallbackUsed = false
      usePlayer.setState({ playing: false })
    }
  })
}
bindAudio()

function loadAndPlay(song: Song) {
  fallbackUsed = false
  audio.src = streamUrl(song)
  void audio.play()
  // Report the play so play counts and history update.
  endpoints.registerPlay(song.id).catch(() => undefined)
}

export const usePlayer = create<PlayerState>((set, get) => ({
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

  playContext: (songs, index) => {
    const song = songs[index]
    if (!song) return
    set({ queue: songs, currentIndex: index, current: song, progress: 0 })
    loadAndPlay(song)
  },

  playSong: (song) => {
    set({ queue: [song], currentIndex: 0, current: song, progress: 0 })
    loadAndPlay(song)
  },

  togglePlay: () => {
    if (!get().current) return
    if (get().playing) audio.pause()
    else void audio.play()
  },

  next: () => {
    const { queue, currentIndex, shuffle } = get()
    if (queue.length === 0) return
    let idx = currentIndex + 1
    if (shuffle && queue.length > 1) {
      do {
        idx = Math.floor(Math.random() * queue.length)
      } while (idx === currentIndex)
    }
    if (idx >= queue.length) {
      if (!get().repeat) {
        set({ playing: false, currentIndex: queue.length })
        return
      }
      idx = 0
    }
    const song = queue[idx]
    set({ currentIndex: idx, current: song, progress: 0 })
    loadAndPlay(song)
  },

  prev: () => {
    const { queue, currentIndex, progress } = get()
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
  },

  seek: (seconds) => {
    audio.currentTime = seconds
    set({ progress: seconds })
  },

  setVolume: (v) => {
    audio.volume = v
    set({ volume: v })
  },

  toggleShuffle: () => set((s) => ({ shuffle: !s.shuffle })),
  toggleRepeat: () => set((s) => ({ repeat: !s.repeat })),
  setFullScreen: (v) => set({ fullScreen: v }),

  setLiked: (liked) => {
    const { current, queue, currentIndex } = get()
    if (!current) return
    const updated = { ...current, liked }
    const newQueue = queue.map((s, i) => (i === currentIndex ? updated : s))
    set({ current: updated, queue: newQueue })
  },
}))
