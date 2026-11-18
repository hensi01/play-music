// Browser facts and PWA entry points shared by the ES module app (app.js) and
// the classic script (pwa.js). A classic script cannot import a module, so
// everything they share lives on this one object instead of ad-hoc globals.
;(function () {
  function isIOS() {
    return /iP(hone|od|ad)/.test(navigator.userAgent) ||
      (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  }

  function isInstalled() {
    return window.matchMedia('(display-mode: standalone)').matches ||
      window.navigator.standalone === true
  }

  // The admin login page is a real path (not a hash route) and shows neither
  // the install overlay nor the notification gate.
  function onAdminLogin() {
    return window.location.pathname.replace(/\/+$/, '') === '/admin/login'
  }

  window.__pmPlatform = {
    isIOS,
    isInstalled,
    onAdminLogin,
    // Written by pwa.js once it wires the install flow.
    install: null,
    dismissInstall: null,
    // Written by app.js: whether the admin UI is showing, and whether the
    // notification gate currently owns the screen.
    admin: false,
    gateVisible: false,
  }
})()
