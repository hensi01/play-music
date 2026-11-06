const { test, expect } = require('playwright/test')
test.use({ serviceWorkers: 'block' })
test('escuta qualificada ignora pausas e saltos e registra uma vez', async ({ page }) => {
  await page.goto('/')
  const result = await page.evaluate(async () => {
    const { createListenTracker, sampleListen } = await import('/listening.js')
    const t = createListenTracker('s', 'play-1234567890123456', 'u')
    const sample = (position, wall, playing = true, seeking = false, duration = 120) => sampleListen(t, { position, wall, playing, seeking, rate: 1, duration })
    const states = [sample(0,0), sample(10,10000), sample(10,20000,false), sample(100,21000,true,true), sample(100,22000), sample(110,32000), sample(120,42000), sample(121,43000)]
    return { states, elapsed: t.elapsed }
  })
  expect(result.states.filter(Boolean)).toHaveLength(1)
  expect(result.states[6]).toBe(true)
  expect(result.elapsed).toBeCloseTo(30)
})
test('faixas curtas qualificam após metade e repetição usa nova sessão', async ({ page }) => {
  await page.goto('/')
  const result = await page.evaluate(async () => {
    const { createListenTracker, sampleListen } = await import('/listening.js')
    const t = createListenTracker('s', 'play-1234567890123456', 'u')
    const sample = (position, wall) => sampleListen(t, { position, wall, playing: true, seeking: false, rate: 1, duration: 8 })
    return [sample(0,0), sample(3,3000), sample(4,4000), sample(5,5000)]
  })
  expect(result).toEqual([false, false, true, false])
})

test('velocidade diferente respeita 30 segundos reais ou metade da faixa', async ({ page }) => {
  await page.goto('/')
  const states = await page.evaluate(async () => {
    const { createListenTracker, sampleListen } = await import('/listening.js')
    const slow = createListenTracker('s','slow-1234567890123456','u')
    const fast = createListenTracker('s','fast-1234567890123456','u')
    const run = (t, position, wall, rate) => sampleListen(t, { position, wall, playing:true, seeking:false, rate, duration:40 })
    return [run(slow,0,0,0.5),run(slow,15,30000,0.5),run(fast,0,0,2),run(fast,20,10000,2)]
  })
  expect(states).toEqual([false,true,false,true])
})

test('buffering sem avanço não conta como escuta', async ({ page }) => {
  await page.goto('/')
  const values = await page.evaluate(async () => {
    const { createListenTracker, sampleListen } = await import('/listening.js')
    const t = createListenTracker('s','buffer-1234567890123456','u')
    const run = (position, wall) => sampleListen(t, { position, wall, playing:true, seeking:false, rate:1, duration:120 })
    return [run(0,0),run(0,30000),t.elapsed,t.media]
  })
  expect(values).toEqual([false,false,0,0])
})
