const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')

function setup() {
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
    querySelector(selector) { return selector.includes('install') ? (overlay && !overlay.removed ? buttons.install : null) : (overlay && !overlay.removed ? overlay : null) },
    body: { appendChild() {} },
  }
  const navigator = { userAgent: 'Android', platform: '', maxTouchPoints: 0 }
  const window = { navigator, matchMedia: () => ({ matches: false }), addEventListener(name, fn) { listeners[name] = fn } }
  const sessionStorage = { getItem: () => null, setItem() {} }
  vm.runInNewContext(fs.readFileSync('web/assets/pwa.js', 'utf8'), { window, document, navigator, sessionStorage, setTimeout: (fn) => fn() })
  listeners.load()
  return { listeners, buttons, steps, overlay }
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
