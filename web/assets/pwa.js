// PWA install popup: shows an install card with platform-specific
// instructions whenever the app is opened in the browser (not installed).
// Chrome (Android/desktop) can install directly through beforeinstallprompt;
// iOS and browsers without it get step-by-step instructions.

(function () {
  const isInstalled = () =>
    window.matchMedia('(display-mode: standalone)').matches ||
    window.navigator.standalone === true

  const isIOS = () =>
    /iP(hone|od|ad)/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)

  const isAndroid = () => /Android/i.test(navigator.userAgent)

  // Safari desktop (macOS) — sem beforeinstallprompt; instala pelo menu.
  const isSafariDesktop = () => {
    const ua = navigator.userAgent
    return /Safari\//.test(ua) && !/Chrome\//.test(ua) && !/CriOS\//.test(ua) && !isIOS()
  }

  let deferredPrompt = null

  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault()
    deferredPrompt = e
  })

  // O aviso de instalação aparece UMA vez por sessão (sessionStorage): ele é
  // gravado no primeiro show, então recarregar/navegar na mesma aba não
  // re-exibe o card. Fechar a aba/janela inicia uma nova sessão (o aviso
  // volta). try/catch: se o storage não estiver disponível, mantém o
  // comportamento antigo (mostrar em toda visita).
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

  const musicIcon =
    '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>'

  // Tradução via window.__i18n (o i18n.js é um módulo ES; o pwa.js é um
  // script clássico — as funções só usam t() no evento load, já com o
  // i18n carregado). Fallback: devolve a própria chave.
  const tt = (key) => (window.__i18n ? window.__i18n.t(key) : key)

  function stepsFor() {
    if (isIOS()) {
      return [tt('pwa.ios.1'), tt('pwa.ios.2'), tt('pwa.ios.3')]
    }
    if (isAndroid()) {
      return [tt('pwa.android.1'), tt('pwa.android.2'), tt('pwa.android.3')]
    }
    if (isSafariDesktop()) {
      return [tt('pwa.safari.1'), tt('pwa.safari.2')]
    }
    return [tt('pwa.other.1'), tt('pwa.other.2')]
  }

  function buildCard() {
    const tips = [tt('pwa.tip1'), tt('pwa.tip2'), tt('pwa.tip3')]
    const canInstall = deferredPrompt != null
    const steps = stepsFor()

    const card = document.createElement('div')
    card.className = 'pwa-card'
    card.innerHTML =
      '<div class="pwa-header">' +
        '<span class="pwa-icon">' + musicIcon + '</span>' +
        '<h2>' + tt('pwa.install') + '</h2>' +
      '</div>' +
      '<p class="pwa-desc">' + tt('pwa.desc') + '</p>' +
      '<ul class="pwa-tips">' + tips.map((t) => '<li>' + t + '</li>').join('') + '</ul>' +
      '<p class="pwa-steps-title">' + tt('pwa.howTo') + '</p>' +
      '<ol class="pwa-steps">' + steps.map((s) => '<li>' + s + '</li>').join('') + '</ol>' +
      '<div class="pwa-actions">' +
        (canInstall ? '<button type="button" class="pwa-btn primary" data-act="install">' + tt('pwa.installBtn') + '</button>' : '') +
        '<button type="button" class="pwa-btn" data-act="dismiss">' + tt('pwa.dismiss') + '</button>' +
      '</div>'

    card.querySelector('[data-act="dismiss"]').addEventListener('click', () => {
      const overlay = card.closest('.pwa-overlay')
      if (overlay) overlay.remove()
    })
    card.querySelectorAll('[data-act="install"]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        if (!deferredPrompt) return
        deferredPrompt.prompt()
        await deferredPrompt.userChoice.catch(() => undefined)
        deferredPrompt = null
        card.remove()
      })
    })
    return card
  }

  function show() {
    if (isInstalled() || document.querySelector('.pwa-overlay') || pwaSeen()) return
    markPwaSeen()
    const overlay = document.createElement('div')
    overlay.className = 'pwa-overlay'
    overlay.appendChild(buildCard())
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) overlay.remove()
    })
    document.body.appendChild(overlay)
  }

  // App aberto no navegador (não instalado): mostra uma única vez por sessão.
  window.addEventListener('load', () => {
    setTimeout(show, 1500)
  })
})()
