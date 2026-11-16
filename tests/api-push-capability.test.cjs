const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const vm = require('node:vm')

const source = fs.readFileSync('web/assets/api.js', 'utf8')
  .replace(/\bexport\s+(?=(?:async\s+)?(?:const|function)\b)/g, '') +
  '\nthis.testAPI = { apiFetch, artworkUrl, streamUrl }'

function client(supportsPush, userAgent = 'Chrome', search = '') {
  const requests = []
  const window = { isSecureContext: true, location: { search }, __APP_CONFIG__: { baseURL: '' } }
  const navigator = supportsPush ? { userAgent, serviceWorker: {}, locks: {} } : { userAgent }
  if (supportsPush) {
    window.PushManager = function PushManager() {}
    window.Notification = function Notification() {}
  }
  const ctx = {
    window,
    navigator,
    localStorage: { getItem: () => null, setItem() {}, removeItem() {} },
    Headers,
    URLSearchParams,
    fetch: async (url, options) => {
      requests.push({ url, options })
      return { headers: { get: () => null }, status: 200, ok: true, json: async () => ({}) }
    },
  }
  vm.runInNewContext(source, ctx)
  return { api: ctx.testAPI, requests, window }
}

test('browser sem Push continua sem marcador de exceção', async () => {
  const c = client(false)
  await c.api.apiFetch('/api/songs')
  assert.equal(c.requests[0].options.headers.has('X-Mi-Ritmo-Push-Unavailable'), false)
  assert.equal(new URL(c.api.artworkUrl('song-1'), 'https://miritmo.test').searchParams.has('pushUnavailable'), false)
  assert.equal(new URL(c.api.streamUrl({ id: 'song-1', format: 'mp3' }), 'https://miritmo.test').searchParams.has('pushUnavailable'), false)
})

test('browser com Push não envia a isenção', async () => {
  const c = client(true)
  await c.api.apiFetch('/api/songs')
  assert.equal(c.requests[0].options.headers.has('X-Mi-Ritmo-Push-Unavailable'), false)
  assert.equal(new URL(c.api.artworkUrl('song-1'), 'https://miritmo.test').searchParams.has('pushUnavailable'), false)
  assert.equal(new URL(c.api.streamUrl({ id: 'song-1', format: 'mp3' }), 'https://miritmo.test').searchParams.has('pushUnavailable'), false)
})

test('ChatGPT browser envia isenção mesmo quando anuncia as APIs', async () => {
  const c = client(true, 'ChatGPT/1.0')
  await c.api.apiFetch('/api/songs')
  assert.equal(c.requests[0].options.headers.get('X-Mi-Ritmo-Push-Unavailable'), '1')
  assert.equal(new URL(c.api.streamUrl({ id: 'song-1', format: 'mp3' }), 'https://miritmo.test').searchParams.get('pushUnavailable'), '1')
})

test('link do modo ChatGPT envia a isenção mesmo com User-Agent comum', async () => {
  const c = client(true, 'Chrome', '?chatgpt=1')
  await c.api.apiFetch('/api/songs')
  assert.equal(c.requests[0].options.headers.get('X-Mi-Ritmo-Push-Unavailable'), '1')
  assert.equal(new URL(c.api.artworkUrl('song-1'), 'https://miritmo.test').searchParams.get('pushUnavailable'), '1')
})
