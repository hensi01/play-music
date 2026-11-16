const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const vm = require('node:vm')

function setup(delayed = false) {
  let sound, queries = 0, shown = 0, release
  class AudioMock {
    constructor() { sound = this; this.handlers = {}; this.currentTime = 0; this.duration = 20; this.paused = false }
    addEventListener(name, fn) { this.handlers[name] = fn }
    play() { return Promise.resolve() }
    pause() {}
    dispatch(name) { this.handlers[name]?.() }
  }
  const element = () => ({ append() {}, remove() {}, setAttribute() {}, className: '', textContent: '', hidden: false })
  const sandbox = { Audio: AudioMock, window: {}, navigator: {}, document: { body: { append() {} }, createElement: element }, performance: { now: () => 0 }, crypto: { randomUUID: () => 'id' }, setInterval() {}, URL, AbortSignal,
    api: { fetch: async () => { queries++; if (delayed) await new Promise((resolve) => { release = resolve }); return { frequency: 1, ads: [{ embedUrl: 'https://ads.nuvexaia.online/embed/test', name: 'Teste' }] } } },
    endpoints: { registerPlay: async () => {}, qualifiedListen: async () => {} }, streamUrl: () => 'song.mp3', artworkUrl: () => '', getToken: () => '',
    createListenTracker: () => ({}), sampleListen: () => false, showAd: () => { shown++; return {} } }
  const source = fs.readFileSync('web/assets/player.js', 'utf8').replace(/^import .*$/gm, '').replace(/^export /gm, '') + '\nthis.player = { playContext, toggleShuffle, togglePlay }'
  vm.runInNewContext(source, sandbox)
  return { sandbox, sound: () => sound, queries: () => queries, shown: () => shown, release: () => release() }
}

test('a shuffled queue checks for ads after its last indexed song', async () => {
  const h = setup(); const { sandbox } = h
  sandbox.player.playContext([{ id: '1', duration: 20 }, { id: '2', duration: 20 }], 1)
  sandbox.player.toggleShuffle(); h.sound().dispatch('ended'); await new Promise(setImmediate)
  assert.equal(h.queries(), 1); assert.equal(h.shown(), 1)
})

test('pause during configuration fetch waits for resume before starting ad', async () => {
  const h = setup(true); const { sandbox } = h
  sandbox.player.playContext([{ id: '1' }, { id: '2' }], 0)
  h.sound().dispatch('ended'); sandbox.player.togglePlay(); h.release(); await new Promise(setImmediate)
  assert.equal(h.shown(), 0); sandbox.player.togglePlay(); assert.equal(h.shown(), 1)
})
