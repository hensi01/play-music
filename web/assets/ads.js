// Advertisement playback is owned by the app, not by a cross-origin iframe.
const origin = 'https://ads.nuvexaia.online'

export function showAd(ad, onDone, onPlaybackState = () => {}) {
  const id = new URL(ad.embedUrl).pathname.split('/').pop()
  const instanceId = crypto.randomUUID()
  const overlay = document.createElement('section')
  overlay.className = 'ad-overlay'
  overlay.setAttribute('aria-label', 'Anúncio patrocinado')
  const slot = document.createElement('div'); slot.className = 'ad-media'
  const status = document.createElement('p'); status.setAttribute('role', 'status')
  const button = document.createElement('button'); button.className = 'btn-primary'; button.type = 'button'; button.hidden = true; button.disabled = true
  const destination = document.createElement('div'); destination.className = 'ad-destination'
  overlay.append(slot, status, button, destination)
  document.body.append(overlay)
  let media, timer, mediaTimeout, running = false, ended = false, elapsed = 0, anchor = 0, destroyed = false, loading = false, ready = false, impressed = false, mediaGeneration = 0
  const event = (type) => fetch(`${origin}/api/v1/events`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ adId: id, type, instanceId }), keepalive: true }).catch(() => {})
  const impression = () => { if (!impressed) { impressed = true; event('impression') } }
  const update = () => {
    const time = elapsed + (running ? (performance.now() - anchor) / 1000 : 0)
    status.textContent = `${Math.max(0, Math.ceil(ad.duration - time))} s`
    if (time >= ad.duration) { cleanup(); onDone() }
  }
  const startClock = () => { if (running || destroyed) return; running = true; button.hidden = true; onPlaybackState(true); anchor = performance.now(); update(); if (!destroyed) timer = setInterval(update, 250) }
  const stopClock = () => { if (running) elapsed += (performance.now() - anchor) / 1000; running = false; onPlaybackState(false); clearInterval(timer) }
  const fail = () => { if (destroyed) return; clearTimeout(mediaTimeout); stopClock(); status.textContent = 'Não foi possível reproduzir o anúncio. Toque para tentar novamente.'; button.textContent = 'Tentar novamente'; button.disabled = false; button.hidden = false }
  const isPlayable = (node) => node?.tagName === 'VIDEO' || node?.tagName === 'AUDIO'
  const cleanup = () => { destroyed = true; mediaGeneration++; clearTimeout(mediaTimeout); clearInterval(timer); if (isPlayable(media)) { media.pause(); media.removeAttribute('src'); media.load() }; overlay.remove() }
  const retryPlayback = () => {
    if (!media) { load(); return }
    if (!ready && !isPlayable(media)) return
    if (isPlayable(media) && !ended) { const generation = mediaGeneration; media.play().catch(() => { if (!destroyed && generation === mediaGeneration) fail() }) }
    else startClock()
  }
  button.onclick = () => { if (button.disabled) return; button.hidden = true; retryPlayback() }
  async function load() {
    if (loading || destroyed) return
    loading = true; button.disabled = true; button.hidden = true; status.textContent = 'Carregando anúncio…'
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
          const link = document.createElement('a'); link.href = target.href; link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = 'Saiba Mais'; link.onclick = () => event('click'); destination.append(link)
        }
      }
      media = document.createElement(data.kind === 'image' ? 'img' : data.kind)
      const current = media, generation = ++mediaGeneration
      const mediaFail = () => { if (generation !== mediaGeneration || destroyed) return; mediaGeneration++; if (isPlayable(current)) { current.pause(); current.removeAttribute('src'); current.load() }; media = null; ready = false; fail() }
      media.alt = data.altText || 'Anúncio'
      current.addEventListener('error', mediaFail)
      mediaTimeout = setTimeout(mediaFail, 15000)
      if (data.kind === 'image') current.addEventListener('load', () => { if (generation !== mediaGeneration || destroyed) return; clearTimeout(mediaTimeout); ready = true; button.disabled = false; button.hidden = true; impression(); startClock() }, { once: true })
      else {
        current.controls = false; current.disablePictureInPicture = true; current.disableRemotePlayback = true; current.playsInline = true; current.preload = 'auto'
        current.addEventListener('playing', () => { if (generation !== mediaGeneration || destroyed) return; clearTimeout(mediaTimeout); ready = true; button.disabled = false; button.hidden = true; impression(); startClock() })
        const waitForMedia = () => { if (generation !== mediaGeneration || destroyed) return; stopClock(); clearTimeout(mediaTimeout); mediaTimeout = setTimeout(mediaFail, 15000) }
        current.addEventListener('waiting', waitForMedia)
        current.addEventListener('stalled', () => { if (current.readyState < 3) waitForMedia() })
        current.addEventListener('ended', () => { if (generation !== mediaGeneration || destroyed) return; ended = true; startClock() })
      }
      slot.replaceChildren(current)
      current.src = mediaURL.href
      if (isPlayable(current)) {
        current.play().catch(() => { if (!destroyed && generation === mediaGeneration) { clearTimeout(mediaTimeout); ready = true; fail() } })
      }
    } catch { fail() } finally { loading = false }
  }
  load()
  return { cleanup }
}
