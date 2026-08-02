export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00'
  const s = Math.floor(seconds % 60)
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}:${s.toString().padStart(2, '0')}`
  const h = Math.floor(m / 60)
  return `${h}:${(m % 60).toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

export function formatDurationLong(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0 min'
  const m = Math.max(1, Math.round(seconds / 60))
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  return `${h} h ${m % 60} min`
}

export function musicas(n: number): string {
  return `${n} ${n === 1 ? 'música' : 'músicas'}`
}

export function albuns(n: number): string {
  return `${n} ${n === 1 ? 'álbum' : 'álbuns'}`
}

export function greeting(): string {
  const h = new Date().getHours()
  if (h < 6) return 'Boa madrugada'
  if (h < 12) return 'Bom dia'
  if (h < 18) return 'Boa tarde'
  return 'Boa noite'
}
