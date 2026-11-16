const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')

function setup({ gateActive = false, isAdmin = false, userAgent = 'Android' } = {}) {
  const listeners = {}, saved = new Map()
  let overlay, card
  const button = {
    disabled: false,
    textContent: '',
    addEventListener(_event, fn) { this.onClick = fn },
  }
  card = {
    className: '',
    html: '',
    querySelector(selector) { return selector.includes('install') ? button : null },
    closest() { return overlay },
    setAttribute() {},
    set innerHTML(value) {
      this.html = value
      const text = value.match(/data-act="install">([^<]*)<\/button>/)
      if (text) button.textContent = text[1]
    },
    get innerHTML() { return this.html },
  }
  const document = {
    createElement(tag) {
      if (tag === 'div' && !overlay) {
        overlay = {
          className: '',
          handlers: {},
          appendChild(child) { this.card = child },
          querySelector(selector) { return this.card?.querySelector(selector) || null },
          addEventListener(name, fn) { this.handlers[name] = fn },
          remove() { this.removed = true },
        }
        return overlay
      }
      return card
    },
    querySelector(selector) {
      if (selector === '.notification-gate') return gateActive ? {} : null
      if (selector === '.pwa-overlay') return overlay && !overlay.removed ? overlay : null
      return null
    },
    body: { appendChild() {} },
  }
  const navigator = { userAgent, platform: '', maxTouchPoints: 0 }
  const window = {
    navigator,
    __pmAdmin: isAdmin,
    location: { pathname: '/' },
    matchMedia: () => ({ matches: false }),
    addEventListener(name, fn) { listeners[name] = fn },
    removeEventListener(name) { delete listeners[name] },
  }
  const sessionStorage = {
    getItem: (key) => saved.get(key) || null,
    setItem(key, value) { saved.set(key, value) },
  }
  vm.runInNewContext(fs.readFileSync('web/assets/pwa.js', 'utf8'), {
    window, document, navigator, sessionStorage, setTimeout: (fn) => fn(),
  })
  listeners.load()
  return {
    listeners,
    button,
    document,
    get overlay() { return overlay },
    card,
    window,
    saved,
  }
}

function click(s) {
  return s.button.onClick({ currentTarget: s.button })
}

function installEvent(prompt, outcome = 'accepted') {
  return {
    prevented: false,
    preventDefault() { this.prevented = true },
    prompt,
    userChoice: Promise.resolve({ outcome }),
  }
}

test('popup contém somente o botão Instalar PWA', () => {
  const s = setup()
  assert.equal(s.button.textContent, 'Instalar PWA')
  assert.match(s.card.innerHTML, /^<button[^>]*>Instalar PWA<\/button>$/)
  assert.equal(s.card.innerHTML.includes('<h2'), false)
  assert.equal(s.card.innerHTML.includes('Agora não'), false)
})

test('clique chama o prompt nativo e fecha após aceitar', async () => {
  const s = setup(); let prompts = 0
  const event = installEvent(() => { prompts++; return Promise.resolve() })
  s.listeners.beforeinstallprompt(event)
  assert.equal(event.prevented, true)
  await click(s)
  assert.equal(prompts, 1)
  assert.equal(s.overlay.removed, true)
})

test('após recusar a confirmação nativa fecha e não reaparece na sessão', async () => {
  const s = setup()
  s.listeners.beforeinstallprompt(installEvent(() => Promise.resolve(), 'dismissed'))
  await click(s)
  assert.equal(s.overlay.removed, true)
  assert.equal(s.saved.get('pm_pwa_seen'), '1')
})

test('sem prompt nativo, o mesmo botão informa o caminho manual', async () => {
  const s = setup()
  await click(s)
  assert.match(s.button.textContent, /menu do navegador/)
  assert.equal(s.overlay.removed, undefined)
  s.listeners.beforeinstallprompt(installEvent(() => Promise.resolve()))
  assert.equal(s.button.textContent, 'Instalar PWA')
})

test('botão do cabeçalho também aciona diretamente o prompt nativo', () => {
  const s = setup(); let prompts = 0
  s.listeners.beforeinstallprompt(installEvent(() => { prompts++; return Promise.resolve() }))
  s.window.installPlayMusic()
  assert.equal(prompts, 1)
})

test('popup automático não cobre ativação obrigatória nem aparece para admin', () => {
  assert.equal(setup({ gateActive: true }).overlay, undefined)
  assert.equal(setup({ isAdmin: true }).overlay, undefined)
})

test('tecla Escape dispensa o popup', () => {
  const s = setup()
  s.listeners.keydown({ key: 'Escape' })
  assert.equal(s.overlay.removed, true)
  assert.equal(s.saved.get('pm_pwa_seen'), '1')
})
