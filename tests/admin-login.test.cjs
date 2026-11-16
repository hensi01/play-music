const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')

const source = fs.readFileSync('web/assets/app.js', 'utf8')
const loginUI = source.slice(source.indexOf("let loginMode ="), source.indexOf('\n// ---------- Sidebar ----------'))

function fixture() {
  const calls = []
  const window = { location: { pathname: '/admin/login', replace: (url) => calls.push(['replace', url]) } }
  const context = {
    calls,
    window,
    loginMode: undefined,
    el(tag, attrs = {}, ...children) {
      const node = { tag, attrs: {}, children: children.flat(), listeners: {}, value: '', disabled: false, _textContent: '' }
      Object.assign(node.attrs, attrs)
      for (const [key, value] of Object.entries(attrs)) {
        if (key.startsWith('on') && typeof value === 'function') node.listeners[key.slice(2)] = value
      }
      node.addEventListener = (name, handler) => { node.listeners[name] = handler }
      Object.defineProperty(node, 'textContent', { get() { return this._textContent }, set(value) { this._textContent = String(value) } })
      return node
    },
    icon: (name) => ({ tag: 'icon', attrs: { name }, children: [] }),
    applyPhoneMask() {},
    doLogin: async (...args) => calls.push(['login', ...args]),
    setToken() {},
    setAuth() {},
    endpoints: { async register(phone) { calls.push(['register', phone]); return { token: 'client-token' } } },
    parseHash: () => ({ path: '/' }),
    navigate: (path) => calls.push(['navigate', path]),
    render() {},
    Error,
  }
  vm.createContext(context)
  vm.runInContext(loginUI, context)
  const root = { innerHTML: '', children: [], append(...children) { this.children.push(...children) } }
  context.root = root
  return { context, root, calls }
}

function walk(node, predicate, found = []) {
  if (!node || typeof node !== 'object') return found
  if (predicate(node)) found.push(node)
  for (const child of node.children || []) walk(child, predicate, found)
  return found
}

test('o login comum mostra telefone sem links administrativos ou de documentação', () => {
  const f = fixture()
  vm.runInContext('renderLogin(root)', f.context)
  const inputs = walk(f.root, (node) => node.tag === 'input')
  const links = walk(f.root, (node) => node.tag === 'a')
  const buttons = walk(f.root, (node) => node.tag === 'button')

  assert.equal(inputs.length, 1)
  assert.equal(inputs[0].attrs.autocomplete, 'tel')
  assert.equal(buttons.some((button) => button.children.includes('Administrador')), false)
  assert.equal(links.some((link) => link.attrs.href === '/admin/login'), false)
  assert.equal(links.some((link) => link.attrs.href === '/docs/'), false)
})

test('a página administrativa tem formulário próprio e leva ao painel após login', async () => {
  const f = fixture()
  vm.runInContext('renderAdminLogin(root)', f.context)
  const inputs = walk(f.root, (node) => node.tag === 'input')
  const form = walk(f.root, (node) => node.tag === 'form')[0]

  assert.equal(inputs.length, 2)
  inputs.find((input) => input.attrs.autocomplete === 'username').value = 'admin@example.com'
  inputs.find((input) => input.attrs.autocomplete === 'current-password').value = 'correct-horse'
  await form.listeners.submit({ preventDefault() {} })

  assert.deepEqual(f.calls[0], ['login', 'admin', 'admin@example.com', 'correct-horse'])
  assert.deepEqual(f.calls[1], ['replace', '/#/admin'])
})

test('o cliente administrativo usa endpoint separado do login de clientes', async () => {
  const loginFn = source.slice(source.indexOf('async function doLogin('), source.indexOf('\nfunction doLogout()'))
  const calls = []
  const context = {
    api: { async fetch(path, options) { calls.push([path, JSON.parse(options.body)]); return { token: 't', id: '1', isAdmin: true } } },
    setToken() {}, setAuth() {},
  }
  vm.createContext(context)
  vm.runInContext(loginFn, context)

  await vm.runInContext("doLogin('admin', 'admin', 'secret')", context)
  await vm.runInContext("doLogin('client', '11999999999')", context)

  assert.deepEqual(calls, [
    ['/auth/admin/login', { username: 'admin', password: 'secret' }],
    ['/auth/login', { phone: '11999999999' }],
  ])
})
