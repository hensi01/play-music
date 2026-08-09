// Admin song upload validation (backend fix): junk bytes renamed to .mp3
// must be rejected with 400 (they used to be accepted with 201 and indexed
// with duration 0), and a valid WAV must be accepted with 201 echoing a
// REAL createdAt (the upsert struct used to serialize the zero-value) and a
// duration > 0. API-level tests via the request fixture.
//
// NOTE: there is no DELETE /api/admin/songs, so each full run leaves one
// test WAV in the catalog/bucket per project. They are harmless (titles
// start with pw-e2e-upload-) but must be purged for a clean seed:
//   1) stop the server, then run (as a temporary maintenance test):
//      DELETE FROM songs WHERE title LIKE 'pw-e2e%';
//      plus removal of the orphaned uploads/* bucket objects (the scanner
//      re-indexes them otherwise). See the run report for the exact steps.

const { test, expect } = require('playwright/test')
const fs = require('fs')
const {
  apiAuthHeaders,
  apiAdminSongs,
  ensureSilenceWav,
} = require('./helpers')

const BASE = 'http://localhost:4533'

test.describe('Upload admin', () => {
  test('lixo .mp3 → 400 e não entra no catálogo', async ({ request }) => {
    const testInfo = test.info()
    const headers = await apiAuthHeaders(request)
    const before = (await apiAdminSongs(request)).songs.length

    const junk = Buffer.alloc(1024, 'x')
    const res = await request.post(`${BASE}/api/admin/songs`, {
      headers,
      multipart: {
        song: { name: 'fake.mp3', mimeType: 'audio/mpeg', buffer: junk },
      },
    })
    expect(res.status()).toBe(400)
    const body = await res.json()
    expect(body.error).toContain('áudio inválido')

    const after = (await apiAdminSongs(request)).songs.length
    expect(after).toBe(before)
    testInfo.annotations.push({ type: 'api-only', description: 'sem navegação de página — nada a rastrear no console' })
  })

  test('WAV válido → 201 com createdAt real e duration > 0', async ({ request }) => {
    const testInfo = test.info()
    const headers = await apiAuthHeaders(request)

    const wav = fs.readFileSync(ensureSilenceWav(30))
    // Unique file name per run so catalog titles never collide with the
    // player spec's expectations (desktop + mobile projects both upload).
    const name = `pw-e2e-upload-${Date.now()}.wav`
    const res = await request.post(`${BASE}/api/admin/songs`, {
      headers,
      multipart: {
        song: { name, mimeType: 'audio/wav', buffer: wav },
      },
    })
    expect(res.status()).toBe(201)
    const song = await res.json()
    expect(song.id).toBeTruthy()
    expect(song.createdAt).toBeTruthy()
    expect(song.createdAt).not.toContain('0001-01-01')
    expect(song.duration).toBeGreaterThan(0)
    expect(Math.round(song.duration)).toBe(30)

    // The song is indexed in the catalog with the real duration (not 0).
    const list = await apiAdminSongs(request)
    const found = list.songs.find((s) => s.id === song.id)
    expect(found).toBeTruthy()
    expect(Math.round(found.duration)).toBe(30)
    testInfo.annotations.push({ type: 'api-only', description: 'sem navegação de página — nada a rastrear no console' })
  })
})
