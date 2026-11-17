// PWA install popup: the card contains a single install button. Browsers that
// expose beforeinstallprompt open their native install confirmation on click;
// other browsers get a short fallback on the same button.

(function () {
  const isInstalled = () =>
    window.matchMedia('(display-mode: standalone)').matches ||
    window.navigator.standalone === true

  const isIOS = () =>
    /iP(hone|od|ad)/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)

  let deferredPrompt = null
  let activeOverlay = null

  // Ao dispensar o aviso, não mostrá-lo novamente na mesma sessão.
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
    return isIOS()
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
      '<span class="notification-gate-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="2" width="14" height="20" rx="3"/><path d="M12 6v9m-3-3 3 3 3-3M10 19h4"/></svg></span>' +
      '<p class="notification-gate-eyebrow">TENHA O MI RITMO SEMPRE COM VOCÊ</p>' +
      '<h1>Instale o Mi Ritmo</h1>' +
      '<p class="notification-gate-description">Instale o aplicativo para acessar suas músicas diretamente pela tela inicial.</p>' +
      '<button type="button" class="pwa-btn primary" data-act="install">Instalar aplicativo</button>'

    card.querySelector('[data-act="install"]').addEventListener('click', async (event) => {
      const button = event.currentTarget
      const prompt = deferredPrompt
      if (!prompt) {
        button.textContent = fallbackLabel()
        return
      }

      // prompt() precisa ser chamado diretamente na ação do usuário. Cada
      // evento beforeinstallprompt pode ser usado uma única vez.
      deferredPrompt = null
      button.disabled = true
      button.textContent = 'Abrindo instalação…'
      await requestInstall(prompt, card.closest('.pwa-overlay'))
    })
    return card
  }

  function show(force = false) {
    const onAdminLogin = window.location?.pathname?.replace(/\/+$/, '') === '/admin/login'
    if (
      isInstalled() || activeOverlay ||
      (!force && (window.__pmAdmin || onAdminLogin || pwaSeen() || document.querySelector('.notification-gate')))
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

  window.installPlayMusic = function () {
    if (isInstalled()) return
    if (deferredPrompt) {
      const prompt = deferredPrompt
      deferredPrompt = null
      requestInstall(prompt, activeOverlay)
      return
    }
    show(true)
  }
  window.isPlayMusicInstalled = isInstalled

  // App aberto no navegador (não instalado): mostra uma vez por sessão.
  window.addEventListener('load', () => {
    setTimeout(show, 1500)
  })
})()
