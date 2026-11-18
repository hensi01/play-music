// PWA install popup: the card carries a single install button. Browsers that
// expose beforeinstallprompt open their native install confirmation on click;
// other browsers get a short fallback label on the same button.
(function () {
  let deferredPrompt = null
  let activeOverlay = null

  const SEEN_KEY = 'pm_pwa_seen'

  function pwaSeen() {
    try {
      return sessionStorage.getItem(SEEN_KEY) === '1'
    } catch {
      return false
    }
  }

  function markPwaSeen() {
    try {
      sessionStorage.setItem(SEEN_KEY, '1')
    } catch {
      /* storage indisponível */
    }
  }

  function closeOverlay(markSeen = false) {
    if (markSeen) markPwaSeen()
    if (!activeOverlay) return
    activeOverlay.remove()
    activeOverlay = null
    window.removeEventListener('keydown', onOverlayKeydown)
  }

  function onOverlayKeydown(event) {
    if (event.key === 'Escape') closeOverlay(true)
  }

  function fallbackLabel() {
    return window.__pmPlatform.isIOS()
      ? 'Use Compartilhar para adicionar à tela inicial'
      : 'Instale pelo menu do navegador'
  }

  async function requestInstall(prompt, overlay) {
    try {
      await prompt.prompt()
      const choice = await prompt.userChoice
      if (choice.outcome === 'accepted') closeOverlay()
      else closeOverlay(true)
    } catch {
      const button = overlay?.querySelector('[data-act="install"]')
      if (button) {
        button.disabled = false
        button.textContent = fallbackLabel()
      }
    }
  }

  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault()
    deferredPrompt = event
    const button = activeOverlay?.querySelector('[data-act="install"]')
    if (button) {
      button.disabled = false
      button.textContent = 'Instalar aplicativo'
    }
  })

  window.addEventListener('appinstalled', () => {
    deferredPrompt = null
    closeOverlay()
  })

  function buildCard() {
    const card = document.createElement('div')
    card.className = 'pwa-card'
    card.setAttribute('role', 'dialog')
    card.setAttribute('aria-modal', 'true')
    card.setAttribute('aria-label', 'Instalar Mi Ritmo')
    card.innerHTML =
      '<span class="promo-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="2" width="14" height="20" rx="3"/><path d="M12 6v9m-3-3 3 3 3-3M10 19h4"/></svg></span>' +
      '<p class="promo-eyebrow">TENHA O MI RITMO SEMPRE COM VOCÊ</p>' +
      '<h1>Instale o Mi Ritmo</h1>' +
      '<p class="promo-description">Instale o aplicativo para acessar suas músicas diretamente pela tela inicial.</p>' +
      '<button type="button" class="pwa-btn primary" data-act="install">Instalar aplicativo</button>'

    card.querySelector('[data-act="install"]').addEventListener('click', async (event) => {
      const button = event.currentTarget
      const prompt = deferredPrompt
      if (!prompt) {
        button.textContent = fallbackLabel()
        return
      }

      // Each beforeinstallprompt event can only be used once, and the prompt
      // must be requested directly from the user's click.
      deferredPrompt = null
      button.disabled = true
      button.textContent = 'Abrindo instalação…'
      await requestInstall(prompt, card.closest('.pwa-overlay'))
    })
    return card
  }

  function show(force = false) {
    if (
      window.__pmPlatform.isInstalled() || activeOverlay ||
      (!force && (window.__pmPlatform.admin || window.__pmPlatform.onAdminLogin() || pwaSeen() || window.__pmPlatform.gateVisible))
    ) return

    const overlay = document.createElement('div')
    overlay.className = 'pwa-overlay'
    overlay.appendChild(buildCard())
    overlay.addEventListener('click', (event) => {
      if (event.target === overlay) closeOverlay(true)
    })
    activeOverlay = overlay
    document.body.appendChild(overlay)
    window.addEventListener('keydown', onOverlayKeydown)
  }

  window.__pmPlatform.dismissInstall = () => closeOverlay()

  window.__pmPlatform.install = function () {
    if (window.__pmPlatform.isInstalled()) return
    if (deferredPrompt) {
      const prompt = deferredPrompt
      deferredPrompt = null
      requestInstall(prompt, activeOverlay)
      return
    }
    show(true)
  }

  window.addEventListener('load', () => {
    setTimeout(show, 1500)
  })
})()
