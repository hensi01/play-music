const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')

function setup(gateActive = false, isAdmin = false) {
  const listeners = {}, buttons = {}, steps = { textContent: '', scrollIntoView() {} }
  let overlay
  const card = {
    querySelector(selector) { return selector.includes('dismiss') ? buttons.dismiss : selector.includes('install') ? buttons.install : steps },
    closest() { return overlay },
    set innerHTML(value) { this.html = value },
  }
  for (const name of ['install', 'dismiss']) buttons[name] = { addEventListener(_event, fn) { this.click = fn } }
  const document = {
    createElement(tag) {
      if (!overlay) { overlay = { className: '', appendChild(c) { this.card = c }, addEventListener() {}, remove() { this.removed = true } }; return overlay }
      return card
    },
    querySelector(selector) { if (selector === '.notification-gate') return gateActive ? {} : null; return selector.includes('install') ? (overlay && !overlay.removed ? buttons.install : null) : (overlay && !overlay.removed ? overlay : null) },
    body: { appendChild() {} },
  }
  const navigator = { userAgent: 'Android', platform: '', maxTouchPoints: 0 }
  const window = { navigator, __pmAdmin: isAdmin, matchMedia: () => ({ matches: false }), addEventListener(name, fn) { listeners[name] = fn } }
  const saved = new Map()
  const sessionStorage = { getItem: (k) => saved.get(k) || null, setItem(k,v) { saved.set(k,v) } }
  vm.runInNewContext(fs.readFileSync('web/assets/pwa.js', 'utf8'), { window, document, navigator, sessionStorage, setTimeout: (fn) => fn() })
  listeners.load()
  return { listeners, buttons, steps, overlay, window }
}

test('evento tardio abre o diálogo e remove o aviso após aceitação', async () => {
  const s = setup(); let prompts = 0
  s.listeners.beforeinstallprompt({ preventDefault() {}, prompt() { prompts++; return Promise.resolve() }, userChoice: Promise.resolve({ outcome: 'accepted' }) })
  await s.buttons.install.click()
  assert.equal(prompts, 1); assert.equal(s.overlay.removed, true)
})

test('sem diálogo nativo ou após recusa mantém instruções acessíveis', async () => {
  const s = setup()
  await s.buttons.install.click()
  assert.match(s.steps.textContent, /etapas/)
  s.listeners.beforeinstallprompt({ preventDefault() {}, prompt() { return Promise.resolve() }, userChoice: Promise.resolve({ outcome: 'dismissed' }) })
  await s.buttons.install.click()
  assert.match(s.steps.textContent, /menu/); assert.notEqual(s.overlay.removed, true)
})


test('botão do cabeçalho abre instalação após dispensar aviso', async () => {
  const s = setup(); let calls = 0
  s.buttons.dismiss.click()
  s.listeners.beforeinstallprompt({ preventDefault() {}, prompt() { calls++; return Promise.resolve() }, userChoice: Promise.resolve({ outcome: 'accepted' }) })
  s.window.installPlayMusic()
  assert.equal(calls, 1)
})


test('aviso automático de instalação não cobre a ativação obrigatória', () => {
  const s=setup(true); assert.equal(s.overlay, undefined)
})

test('administrador não recebe aviso automático, mas pode solicitar instalação', () => {
  const s=setup(false, true); assert.equal(s.overlay, undefined)
  s.window.installPlayMusic(); assert.equal(typeof s.buttons.install.click, 'function')
})
