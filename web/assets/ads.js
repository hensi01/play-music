// Advertisement playback is owned by the app, not by a cross-origin iframe.
const origin = 'https://ads.nuvexaia.online'

export function showAd(ad, onDone, onPlaybackState = () => {}) {
  const id = new URL(ad.embedUrl).pathname.split('/').pop()
  const instanceId = crypto.randomUUID()
  const overlay = document.createElement('section')
  overlay.className = 'ad-overlay'
  overlay.setAttribute('aria-label', 'Publicidade')
  const title = document.createElement('h2'); title.textContent = 'Publicidade'
  const slot = document.createElement('div'); slot.className = 'ad-media'
  const status = document.createElement('p'); status.setAttribute('role', 'status')
  const button = document.createElement('button'); button.className = 'btn-primary'
  const destination = document.createElement('div')
  overlay.append(title, slot, status, button, destination)
  document.body.append(overlay)
  let media, timer, mediaTimeout, running = false, desiredPlaying = true, ended = false, elapsed = 0, anchor = 0, destroyed = false, loading = false, ready = false, impressed = false, mediaGeneration = 0
  const event = (type) => fetch(`${origin}/api/v1/events`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ adId: id, type, instanceId }), keepalive: true }).catch(() => {})
  const impression = () => { if (!impressed) { impressed = true; event('impression') } }
  const update = () => {
    const time = elapsed + (running ? (performance.now() - anchor) / 1000 : 0)
    status.textContent = `Publicidade · ${Math.max(0, Math.ceil(ad.duration - time))} s`
    if (time >= ad.duration) { cleanup(); onDone() }
  }
  const startClock = () => { if (running || destroyed) return; running = true; onPlaybackState(true); anchor = performance.now(); button.textContent = 'Pausar'; update(); if (!destroyed) timer = setInterval(update, 250) }
  const stopClock = () => { if (running) elapsed += (performance.now() - anchor) / 1000; running = false; onPlaybackState(false); clearInterval(timer); button.textContent = desiredPlaying ? 'Pausar' : 'Continuar' }
  const fail = () => { if (destroyed) return; desiredPlaying = false; clearTimeout(mediaTimeout); stopClock(); status.textContent = 'Não foi possível reproduzir o anúncio. Toque para tentar novamente.'; button.textContent = 'Tentar novamente'; button.disabled = false }
  const isPlayable = (node) => node?.tagName === 'VIDEO' || node?.tagName === 'AUDIO'
  const cleanup = () => { destroyed = true; mediaGeneration++; clearTimeout(mediaTimeout); clearInterval(timer); if (isPlayable(media)) { media.pause(); media.removeAttribute('src'); media.load() }; overlay.remove() }
  const resume = () => {
    desiredPlaying = true
    if (!media) { load(); return }
    if (!ready && !isPlayable(media)) return
    if (isPlayable(media) && !ended) { const generation = mediaGeneration; media.play().catch(() => { if (!destroyed && generation === mediaGeneration) fail() }) }
    else startClock()
  }
  const pause = () => { desiredPlaying = false; if (ready) clearTimeout(mediaTimeout); stopClock(); if (isPlayable(media)) media.pause() }
  button.onclick = () => { if (desiredPlaying) pause(); else resume() }
  async function load() {
    if (loading || destroyed) return
    loading = true; button.disabled = true; status.textContent = 'Carregando publicidade…'
    try {
      const response = await fetch(`${origin}/api/v1/ads/${encodeURIComponent(id)}`, { signal: AbortSignal.timeout(15000) })
      if (!response.ok) throw new Error('Anúncio indisponível')
      const data = await response.json()
      if (destroyed) return
      if (!['image', 'video', 'audio'].includes(data.kind) || !data.mediaUrl || data.id !== id) throw new Error('Anúncio inválido')
      const mediaURL = new URL(data.mediaUrl); if (mediaURL.protocol !== 'https:') throw new Error('Mídia inválida')
      destination.replaceChildren()
      if (data.destinationUrl) {
        const target = new URL(data.destinationUrl)
        if (target.protocol === 'https:') {
          const link = document.createElement('a'); link.href = target.href; link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = 'Visitar anunciante'; link.onclick = () => event('click'); destination.append(link)
        }
      }
      media = document.createElement(data.kind === 'image' ? 'img' : data.kind)
      const current = media, generation = ++mediaGeneration
      const mediaFail = () => { if (generation !== mediaGeneration || destroyed) return; mediaGeneration++; if (isPlayable(current)) { current.pause(); current.removeAttribute('src'); current.load() }; media = null; ready = false; fail() }
      media.alt = data.altText || 'Publicidade'
      current.addEventListener('error', mediaFail)
      mediaTimeout = setTimeout(mediaFail, 15000)
      if (data.kind === 'image') current.addEventListener('load', () => { if (generation !== mediaGeneration || destroyed) return; clearTimeout(mediaTimeout); ready = true; button.disabled = false; impression(); if (desiredPlaying) startClock() }, { once: true })
      else {
        current.playsInline = true; current.preload = 'auto'
        current.addEventListener('playing', () => { if (generation !== mediaGeneration || destroyed) return; clearTimeout(mediaTimeout); ready = true; button.disabled = false; if (!desiredPlaying) { current.pause(); return }; impression(); startClock() })
        const waitForMedia = () => { if (generation !== mediaGeneration || destroyed) return; stopClock(); clearTimeout(mediaTimeout); mediaTimeout = setTimeout(mediaFail, 15000) }
        current.addEventListener('waiting', waitForMedia)
        current.addEventListener('stalled', () => { if (current.readyState < 3) waitForMedia() })
        current.addEventListener('pause', () => { if (generation === mediaGeneration && !ended) stopClock() })
        current.addEventListener('ended', () => { if (generation !== mediaGeneration || destroyed) return; ended = true; startClock() })
      }
      slot.replaceChildren(current)
      current.src = mediaURL.href
      if (isPlayable(current)) {
        if (!desiredPlaying) { button.disabled = false; button.textContent = 'Continuar'; status.textContent = 'Publicidade pausada.' }
        else current.play().catch(() => { if (!destroyed && generation === mediaGeneration) { clearTimeout(mediaTimeout); ready = true; fail() } })
      }
    } catch { fail() } finally { loading = false }
  }
  button.textContent = 'Aguarde'; load()
  return { cleanup, toggle: () => button.click(), pause, resume }
}
