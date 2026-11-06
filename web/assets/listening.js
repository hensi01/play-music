// Count only time that actually advances during playback, excluding seeks.
export function createListenTracker(songId, playId, userId) {
  return { songId, playId, userId, elapsed: 0, media: 0, wall: null, position: null, sent: false }
}
export function sampleListen(tracker, sample) {
  if (!tracker || tracker.sent) return false
  const { position, wall, playing, seeking, rate, duration } = sample
  if (!playing || seeking || !Number.isFinite(position) || !Number.isFinite(wall)) {
    tracker.wall = null; tracker.position = null; return false
  }
  if (tracker.wall != null) {
    const seconds = Math.max(0, (wall - tracker.wall) / 1000)
    const moved = position - tracker.position
    // Ignore discontinuities even when the browser misses a seeking event.
    const playbackRate = Number.isFinite(rate) && rate > 0 ? rate : 1
    if (moved >= 0 && moved <= seconds * playbackRate + 0.5) {
      const heard = Math.min(moved, seconds * playbackRate)
      tracker.elapsed += Math.min(seconds, heard / playbackRate)
      tracker.media += heard
    }
  }
  tracker.wall = wall; tracker.position = position
  const halfReached = Number.isFinite(duration) && duration > 0 && tracker.media + 0.05 >= duration / 2
  if (tracker.elapsed + 0.05 >= 30 || halfReached) { tracker.sent = true; return true }
  return false
}
