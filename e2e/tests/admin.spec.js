// Admin panel: user CRUD via UI, category CRUD via UI (never touching
// "Cristão"), songs tab count, aria-pressed toggle sync and the save button
// disabling during a slow PUT. Throwaway fixtures cleaned up via API.

const { test, expect } = require('playwright/test')
const {
  trackConsole,
  expectClean,
  loginAdmin,
  randomPhone,
  randomName,
  apiCreateCategory,
  apiDeleteCategory,
  apiDeleteUser,
  apiListUsers,
  apiListCategories,
  apiUploadCategoryPhoto,
  apiDeleteCategoryPhoto,
  apiArtworkBytes,
} = require('./helpers')

test.describe('Admin', () => {
  test('CRUD usuário via UI', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await page.goto('/#/admin')

    const name = randomName('PW-User')
    const phone = randomPhone()

    try {
      // --- Create ---
      await page.getByRole('button', { name: 'Novo usuário' }).click()
      const modal = page.locator('.modal-overlay')
      await modal.getByPlaceholder('Nome').fill(name)
      await modal.getByPlaceholder('Telefone (99) 99999-9999').fill(phone)
      await modal.getByRole('button', { name: 'Salvar' }).click()
      await expect(modal).toBeHidden({ timeout: 10000 })

      // Visible in the list (client mode, phone shown as subtitle).
      let row = page.locator('.admin-row', { hasText: name })
      await expect(row).toHaveCount(1, { timeout: 10000 })
      await expect(row.locator('.admin-row-sub')).toContainText(phone)

      // --- Edit (rename) ---
      await row.getByRole('button', { name: 'Editar' }).click()
      const editModal = page.locator('.modal-overlay')
      const newName = name + '-editado'
      const nameInput = editModal.getByPlaceholder('Nome')
      await nameInput.fill(newName)
      await editModal.getByRole('button', { name: 'Salvar' }).click()
      await expect(editModal).toBeHidden({ timeout: 10000 })
      row = page.locator('.admin-row', { hasText: newName })
      await expect(row).toHaveCount(1, { timeout: 10000 })
      // hasText does substring matching, so the renamed row (name + "-editado")
      // would also match the old name — anchor the old-name check with a regex.
      await expect(page.locator('.admin-row', { hasText: new RegExp(`^${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`) })).toHaveCount(0)

      // --- Delete (confirm dialog) ---
      page.once('dialog', (d) => d.accept())
      await row.getByRole('button', { name: 'Excluir' }).click()
      await expect(page.locator('.admin-row', { hasText: newName })).toHaveCount(0, { timeout: 10000 })
    } finally {
      // Cleanup best-effort: if the delete above failed, remove via API.
      try {
        const users = await apiListUsers(page.request)
        const leftover = users.find((u) => u.name === name || u.name === name + '-editado')
        if (leftover) await apiDeleteUser(page.request, leftover.id)
      } catch {
        /* best-effort */
      }
    }
    expectClean(testInfo, clean)
  })

  test('CRUD categoria via UI (nunca toca "Cristão")', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await page.goto('/#/admin')

    const catName = randomName('PW-Cat')

    try {
      await page.getByRole('button', { name: 'Categorias' }).click()
      await expect(page.locator('.admin-row', { hasText: 'Cristão' })).toHaveCount(1, { timeout: 10000 })

      // --- Create ---
      await page.getByRole('button', { name: 'Nova categoria' }).click()
      const modal = page.locator('.modal-overlay')
      await modal.getByPlaceholder('Nome da categoria').fill(catName)
      await modal.getByRole('button', { name: 'Criar' }).click()
      await expect(modal).toBeHidden({ timeout: 10000 })

      let row = page.locator('.admin-row', { hasText: catName })
      await expect(row).toHaveCount(1, { timeout: 10000 })
      await expect(row.locator('.admin-row-sub')).toContainText('música')

      // --- Delete ---
      page.once('dialog', (d) => d.accept())
      await row.getByRole('button', { name: 'Excluir' }).click()
      await expect(page.locator('.admin-row', { hasText: catName })).toHaveCount(0, { timeout: 10000 })

      // "Cristão" untouched.
      await expect(page.locator('.admin-row', { hasText: 'Cristão' })).toHaveCount(1)
    } finally {
      // Best-effort cleanup through the API if the UI delete never ran.
      try {
        const cats = await apiListCategories(page.request)
        const leftover = cats.find((c) => c.name === catName)
        if (leftover) await apiDeleteCategory(page.request, leftover.id)
      } catch {
        /* best-effort */
      }
    }
    expectClean(testInfo, clean)
  })

  test('aba Músicas renderiza 145+', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await page.goto('/#/admin')
    await page.getByRole('button', { name: 'Músicas' }).click()
    await expect.poll(() => page.locator('.admin-row').count(), { timeout: 15000 }).toBeGreaterThanOrEqual(145)
    expectClean(testInfo, clean)
  })

  test('aria-pressed alterna no form de usuário', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    await loginAdmin(page)
    await page.goto('/#/admin')
    await page.getByRole('button', { name: 'Novo usuário' }).click()
    const modal = page.locator('.modal-overlay')
    const clientBtn = modal.getByRole('button', { name: 'Cliente' })
    const adminBtn = modal.getByRole('button', { name: 'Administrador' })

    await expect(clientBtn).toHaveAttribute('aria-pressed', 'true')
    await expect(adminBtn).toHaveAttribute('aria-pressed', 'false')

    await adminBtn.click()
    await expect(adminBtn).toHaveAttribute('aria-pressed', 'true')
    await expect(clientBtn).toHaveAttribute('aria-pressed', 'false')

    await clientBtn.click()
    await expect(clientBtn).toHaveAttribute('aria-pressed', 'true')
    await expect(adminBtn).toHaveAttribute('aria-pressed', 'false')

    await modal.getByRole('button', { name: 'Cancelar' }).click()
    await expect(modal).toBeHidden()
    expectClean(testInfo, clean)
  })

  test('upload e remoção de foto de categoria via API', async ({ request }) => {
    const testInfo = test.info()
    const catName = randomName('PW-CatFoto')
    let catId = null
    try {
      const cat = await apiCreateCategory(request, catName)
      catId = cat.id

      // Antes do upload: artwork = placeholder (mesmo tamanho de um id
      // inexistente).
      const ghost = await apiArtworkBytes(request, 'categoria-inexistente-' + Date.now())
      const before = await apiArtworkBytes(request, catId)
      expect(before).toBe(ghost)

      // Upload de imagem válida → 204 e o artwork muda (não é placeholder).
      await apiUploadCategoryPhoto(request, catId)
      const after = await apiArtworkBytes(request, catId)
      expect(after).not.toBe(ghost)

      // Upload de não-imagem → 400 (validação pré-storage).
      await expect(apiUploadCategoryPhoto(request, catId, { invalid: true })).rejects.toThrow()

      // DELETE → volta ao placeholder.
      await apiDeleteCategoryPhoto(request, catId)
      const reverted = await apiArtworkBytes(request, catId)
      expect(reverted).toBe(ghost)
    } finally {
      if (catId) {
        try {
          await apiDeleteCategory(request, catId)
        } catch {
          /* best-effort */
        }
      }
    }
    testInfo.annotations.push({ type: 'api-only', description: 'sem navegação de página — nada a rastrear no console' })
  })

  test('Salvar desabilita durante save (PUT atrasado)', async ({ page }) => {
    const testInfo = test.info()
    const clean = trackConsole(page)
    const catName = randomName('PW-Slow')
    let catId = null
    try {
      // Throwaway category via API so we never save on a real one.
      const cat = await apiCreateCategory(page.request, catName)
      catId = cat.id

      await loginAdmin(page)
      await page.goto('/#/admin')
      await page.getByRole('button', { name: 'Categorias' }).click()
      const row = page.locator('.admin-row', { hasText: catName })
      await expect(row).toHaveCount(1, { timeout: 10000 })

      // Delay PUT /api/admin/categories/{id} by 1500ms.
      await page.route('**/api/admin/categories/**', async (route) => {
        if (route.request().method() === 'PUT') {
          await new Promise((r) => setTimeout(r, 1500))
        }
        await route.continue()
      })

      await row.getByRole('button', { name: 'Gerenciar' }).click()
      const modal = page.locator('.modal-overlay')
      const saveBtn = modal.getByRole('button', { name: 'Salvar' })
      await saveBtn.click()
      // Disabled while the PUT is in flight…
      await expect(saveBtn).toBeDisabled()
      // …and the modal closes after the save completes.
      await expect(modal).toBeHidden({ timeout: 15000 })
    } finally {
      if (catId) {
        try {
          await apiDeleteCategory(page.request, catId)
        } catch {
          /* best-effort */
        }
      }
    }
    expectClean(testInfo, clean)
  })
})
