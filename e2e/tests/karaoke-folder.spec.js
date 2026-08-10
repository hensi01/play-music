// Karaoke folder upload (admin UI): the "Enviar pasta" button picks a whole
// folder (webkitdirectory) and uploads every video sequentially through the
// single-video endpoint (POST /api/admin/karaokes), assigning the chosen
// category to all of them and applying the optional artist (all) + title
// prefix (prefix + file name). Invalid files are filtered out client-side.
//
// NOTE: there is no DELETE /api/admin/karaokes, so each run leaves two test
// karaokes (titles starting with pw-e2e-kf-) in the DB plus their uploads/*
// bucket objects. Purge for a clean seed:
//   1) stop the server, then run:
//      DELETE FROM karaokes WHERE title LIKE 'pw-e2e-kf%';
//   2) remove the orphaned uploads/<id>.mp4 objects from the bucket (the
//      scanner never re-indexes videos, so they do not re-appear on their
//      own).

const { test, expect } = require('playwright/test')
const fs = require('fs')
const os = require('os')
const path = require('path')
const {
  trackConsole,
  expectClean,
  loginAdmin,
  randomName,
  apiCreateCategory,
  apiDeleteCategory,
  apiListCategories,
  apiAdminKaraokes,
  ensureTestMp4,
} = require('./helpers')

test.describe('Karaoke folder upload', () => {
  test('pasta de vídeos → categoria + artista + prefixo aplicados', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)

    const catName = randomName('PW-KF-Cat')
    const artist = 'PW Artista KF'
    // Project-scoped prefix: desktop and mobile run against the same server/
    // DB, so titles must not collide between projects. Still starts with
    // pw-e2e-kf- so the documented cleanup (LIKE 'pw-e2e-kf%') covers it.
    const prefix = `pw-e2e-kf-${testInfo.project.name}-`
    const titles = ['001-faixa', '002-faixa']

    // Throwaway folder with two tiny decodable MP4s.
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'pw-kf-'))
    for (const t of titles) {
      fs.copyFileSync(ensureTestMp4(`${t}.mp4`, 3), path.join(tmpDir, `${t}.mp4`))
    }

    try {
      // Category must exist before the Karaokês tab renders (the modal select
      // is populated from the tab's category list).
      const cat = await apiCreateCategory(page.request, catName)

      await loginAdmin(page)
      await page.goto('/#/admin')

      // --- Open the folder upload modal ---
      await page.getByRole('button', { name: 'Karaokês' }).click()
      await expect(page.getByRole('button', { name: 'Enviar pasta' })).toBeVisible({ timeout: 10000 })
      await page.getByRole('button', { name: 'Enviar pasta' }).click()
      const modal = page.locator('.modal-overlay')

      // --- Pick the folder ---
      await modal.locator('input[webkitdirectory]').setInputFiles(tmpDir)
      await expect(modal.locator('.upload-dropzone-title')).toHaveText('2 vídeos na pasta')

      // Category is required: start is disabled until one is chosen.
      const startBtn = modal.locator('#folder-upload-start')
      await expect(startBtn).toBeDisabled()
      await modal.locator('select.form-input').selectOption({ label: catName })
      await expect(startBtn).toBeEnabled()

      // --- Optional artist + title prefix ---
      await modal.getByPlaceholder(/Artista/).fill(artist)
      await modal.getByPlaceholder(/Prefixo do título/).fill(prefix)

      // --- Upload all videos in sequence ---
      // Baseline for the no-re-upload guard (this project's karaokes only —
      // the other project may have left its own rows in the shared DB).
      const baseline = await thisProjectCount(page, prefix)
      await startBtn.click()
      await expect(modal.locator('.upload-progress-text')).toHaveText('Concluído: 2 vídeos enviados.', { timeout: 45000 })

      // --- Close and check the persisted rows ---
      // The start button is replaced by a "Fechar" button (no id) once the
      // upload finishes — clicking it must NOT re-run the upload.
      await modal.getByRole('button', { name: 'Fechar' }).click()
      await expect(modal).toBeHidden({ timeout: 5000 })

      // Guard against the Fechar button re-triggering the upload: this
      // project's karaokes must go from `baseline` to exactly baseline + 2
      // after the modal closes, and stay there.
      await expect.poll(() => thisProjectCount(page, prefix), { timeout: 10000 }).toBe(baseline + 2)
      await page.waitForTimeout(1500)
      expect(await thisProjectCount(page, prefix)).toBe(baseline + 2)

      const api = await apiAdminKaraokes(page.request)
      for (const t of titles) {
        const row = page.locator('.admin-row', { hasText: prefix + t })
        await expect(row).toHaveCount(1, { timeout: 10000 })
        await expect(row.locator('.admin-row-sub')).toContainText(artist)
        await expect(row.locator('.chip')).toContainText(catName)
        // Also confirmed server-side (title = prefix + file name).
        expect(api.karaokes.some((k) => k.title === prefix + t)).toBe(true)
      }
    } finally {
      // Best-effort cleanup: the category can go, the karaokes cannot (no
      // delete API — see the header note).
      try {
        const cats = await apiListCategories(page.request)
        const leftover = cats.find((c) => c.name === catName)
        if (leftover) await apiDeleteCategory(page.request, leftover.id)
      } catch {
        /* best-effort */
      }
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
    expectClean(testInfo, clean)
  })
})

// Counts karaokes whose title starts with the given prefix (this run's own).
async function thisProjectCount(page, prefix) {
  const api = await apiAdminKaraokes(page.request)
  return api.karaokes.filter((k) => k.title.startsWith(prefix)).length
}
