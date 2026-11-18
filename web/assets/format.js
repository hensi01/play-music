// fmtDuration renders m:ss, or h:mm:ss past one hour. `unknown` is what a
// missing or zero duration becomes: the player wants "0:00", table cells want
// an empty string.
export function fmtDuration(seconds, { unknown = '0:00' } = {}) {
  if (!Number.isFinite(seconds) || seconds <= 0) return unknown
  const s = Math.floor(seconds % 60)
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}:${String(s).padStart(2, '0')}`
  const h = Math.floor(m / 60)
  return `${h}:${String(m % 60).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function fmtDurationLong(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0 min'
  const m = Math.max(1, Math.round(seconds / 60))
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  return `${h} h ${m % 60} min`
}

export function fmtMB(bytes) {
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

// pluralize renders "1 música" / "3 músicas".
export function pluralize(n, singular, plural) {
  return `${n} ${n === 1 ? singular : plural}`
}
