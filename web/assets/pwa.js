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

  const musicIcon =
    '<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>'

  function stepsFor() {
    if (isIOS()) {
      return [
        'Abra no Safari e toque no botão Compartilhar (ícone de seta para cima) na barra inferior',
        'Role a lista e toque em "Adicionar à Tela de Início"',
        'Toque em "Adicionar" no canto superior direito',
      ]
    }
    if (isAndroid()) {
      return [
        'Abra no Chrome e toque no menu de três pontos (⋮) no canto superior direito',
        'Toque em "Adicionar à tela inicial" (ou "Instalar aplicativo")',
        'Confirme tocando em "Instalar"',
      ]
    }
    if (isSafariDesktop()) {
      return [
        'No Safari, abra o menu "Arquivo" e escolha "Adicionar ao Dock"',
        'Confirme tocando em "Adicionar"',
      ]
    }
    return [
      'No Chrome, toque no ícone de instalação (monitor com seta) no fim da barra de endereço',
      'Confirme tocando em "Instalar"',
    ]
  }

  function buildCard() {
    const tips = [
      'Acesso em 1 toque pela tela de início',
      'Abre em tela cheia, sem a barra do navegador',
      'Ícone próprio do Play Music no seu dispositivo',
    ]
    const canInstall = deferredPrompt != null
    const steps = stepsFor()

    const card = document.createElement('div')
    card.className = 'pwa-card'
    card.innerHTML =
      '<div class="pwa-header">' +
        '<span class="pwa-icon">' + musicIcon + '</span>' +
        '<h2>Instale o Play Music</h2>' +
      '</div>' +
      '<p class="pwa-desc">Ouça suas músicas como um aplicativo:</p>' +
      '<ul class="pwa-tips">' + tips.map((t) => '<li>' + t + '</li>').join('') + '</ul>' +
      '<p class="pwa-steps-title">Como instalar:</p>' +
      '<ol class="pwa-steps">' + steps.map((s) => '<li>' + s + '</li>').join('') + '</ol>' +
      '<div class="pwa-actions">' +
        (canInstall ? '<button type="button" class="pwa-btn primary" data-act="install">Instalar</button>' : '') +
        '<button type="button" class="pwa-btn" data-act="dismiss">Agora não</button>' +
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
    if (isInstalled() || document.querySelector('.pwa-overlay')) return
    const overlay = document.createElement('div')
    overlay.className = 'pwa-overlay'
    overlay.appendChild(buildCard())
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) overlay.remove()
    })
    document.body.appendChild(overlay)
  }

  // App aberto no navegador (não instalado): mostra sempre, em toda visita.
  window.addEventListener('load', () => {
    setTimeout(show, 1500)
  })
})()
