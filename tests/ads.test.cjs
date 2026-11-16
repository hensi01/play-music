const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const vm = require('node:vm')

function harness(kind, delayed = false) {
  let now = 0, tick, lastMedia, timeout, release
  const events = []
  class Element {
    constructor(tag) { this.tagName = tag.toUpperCase(); this.children = []; this.listeners = {}; this.disabled = false; this.hidden = false; this.attributes = {}; this.playCount = 0; this.pauseCount = 0; if (['img', 'video', 'audio'].includes(tag)) lastMedia = this }
    setAttribute(name, value) { this.attributes[name] = value }
    removeAttribute() {}
    append(...children) { this.children.push(...children) }
    replaceChildren(...children) { this.children = children }
    remove() { this.removed = true }
    addEventListener(name, fn) { (this.listeners[name] ||= []).push(fn) }
    dispatch(name) { for (const fn of this.listeners[name] || []) fn() }
    click() { if (!this.disabled) this.onclick?.() }
    play() { this.playCount++; return Promise.resolve() }
    pause() { this.pauseCount++; this.dispatch('pause') }
    load() {}
  }
  const document = { body: new Element('body'), createElement: (tag) => new Element(tag) }
  const fetch = async (url, opts) => { if (url.endsWith('/events')) { events.push(JSON.parse(opts.body)); return {} } if (delayed) await new Promise((resolve) => { release = resolve }); return { ok: true, json: async () => ({ id: 'abc', kind, mediaUrl: 'https://cdn.example/media', destinationUrl: 'https://example.com' }) } }
  const sandbox = { document, fetch, URL, AbortSignal, crypto: { randomUUID: () => 'instance' }, performance: { now: () => now }, setInterval: (fn) => { tick = fn; return 1 }, clearInterval: () => { tick = null }, setTimeout: (fn) => { timeout = fn; return 1 }, clearTimeout: () => { timeout = null } }
  const source = fs.readFileSync('web/assets/ads.js', 'utf8').replace('export function showAd', 'function showAd') + '\nthis.showAd = showAd'
  vm.runInNewContext(source, sandbox)
  let finished = 0
  const states = []
  const player = sandbox.showAd({ embedUrl: 'https://ads.nuvexaia.online/embed/abc', duration: 10 }, () => finished++, (v) => states.push(v))
  return { document, player, states, events, media: () => lastMedia, button: () => document.body.children[0].children.find((node) => node.tagName === 'BUTTON'), release: () => release(), advance: (seconds) => { now += seconds * 1000; tick?.() }, expire: () => timeout?.(), done: () => finished }
}

test('ad overlay has no pause control and labels the destination Saiba Mais', async () => {
  const h = harness('image'); await new Promise(setImmediate)
  const overlay = h.document.body.children[0]
  const destination = overlay.children.find((node) => node.className === 'ad-destination')
  assert.equal(overlay.attributes['aria-label'], 'Anúncio patrocinado')
  assert.equal(overlay.children.some((node) => node.tagName === 'H2'), false)
  assert.equal(h.player.pause, undefined)
  assert.equal(h.player.toggle, undefined)
  assert.equal(h.button().hidden, true)
  assert.equal(destination.children[0].textContent, 'Saiba Mais')
  h.media().dispatch('load')
  assert.equal(h.button().hidden, true)
  assert.equal(overlay.children.find((node) => node.attributes.role === 'status').textContent, '10 s')
})

test('image does not count until loaded and then advances automatically', async () => {
  const h = harness('image'); await new Promise(setImmediate)
  h.advance(20); assert.equal(h.done(), 0); assert.equal(h.events.length, 0)
  h.media().dispatch('load'); h.advance(10); assert.equal(h.done(), 1); assert.equal(h.events[0].type, 'impression')
})

test('video buffering stops the clock until playback resumes', async () => {
  const h = harness('video'); await new Promise(setImmediate)
  h.media().dispatch('playing'); h.advance(4); h.media().dispatch('waiting'); h.advance(30); assert.equal(h.done(), 0)
  h.media().dispatch('playing'); h.advance(6); assert.equal(h.done(), 1); assert.equal(h.events.length, 1)
  assert.ok(h.states.includes(false)); assert.ok(h.states.includes(true))
})

test('a media loading failure offers retry without a pause control', async () => {
  const h = harness('image'); await new Promise(setImmediate)
  h.expire()
  const button = h.button()
  assert.equal(button.disabled, false); assert.equal(button.textContent, 'Tentar novamente')
  assert.equal(button.hidden, false)
})

test('audio starts automatically, records one impression and finishes the slot', async () => {
  const h = harness('audio'); await new Promise(setImmediate)
  const audio = h.media(); assert.equal(audio.tagName, 'AUDIO'); assert.equal(audio.playCount, 1)
  assert.equal(audio.controls, false)
  assert.equal(audio.disablePictureInPicture, true)
  assert.equal(audio.disableRemotePlayback, true)
  audio.dispatch('playing'); h.advance(10); assert.equal(h.done(), 1)
  assert.equal(h.events.filter((e) => e.type === 'impression').length, 1)
})

test('audio buffering and failure wait for user action', async () => {
  const h = harness('audio'); await new Promise(setImmediate)
  const audio = h.media(); audio.dispatch('playing'); h.advance(3); audio.dispatch('waiting'); h.advance(20); assert.equal(h.done(), 0)
  h.expire(); assert.equal(h.button().textContent, 'Tentar novamente')
  assert.ok(audio.pauseCount > 0)
})

test('audio starts automatically when the ad request finishes', async () => {
  const h = harness('audio', true); h.release(); await new Promise(setImmediate)
  const audio = h.media(); assert.equal(audio.playCount, 1)
  assert.equal(h.player.pause, undefined)
})

test('media ending continues the ad slot timer until the configured duration', async () => {
  const h = harness('audio'); await new Promise(setImmediate)
  h.media().dispatch('playing'); h.advance(4); h.media().dispatch('ended')
  h.advance(6); assert.equal(h.done(), 1)
})
