// Admin page: users, categories (songs per category) and song uploads.

import { endpoints, artworkUrl, applyPhoneMask } from './api.js'
import { t, plural } from './i18n.js'

let adminTab = 'users'
let adminState = { users: [], categories: [], songs: [], karaokes: [], songCategoryIds: {}, categoryNames: {} }
// Race guard do refresh: cada chamada incrementa o seq; respostas de
// requisições antigas (troca rápida de abas) são descartadas.
let adminRefreshSeq = 0

function isAdminRefreshCurrent(seq) {
  return seq === adminRefreshSeq
}

export function renderAdmin() {
  const page = document.createElement('div')
  page.className = 'page-padding'

  const tabs = el(
    'div',
    { class: 'tabs' },
    el('button', { class: `tab-btn ${adminTab === 'users' ? 'active' : ''}`, onclick: () => { adminTab = 'users'; refresh() } }, t('admin.users')),
    el('button', { class: `tab-btn ${adminTab === 'categories' ? 'active' : ''}`, onclick: () => { adminTab = 'categories'; refresh() } }, t('admin.categories')),
    el('button', { class: `tab-btn ${adminTab === 'songs' ? 'active' : ''}`, onclick: () => { adminTab = 'songs'; refresh() } }, t('admin.songs')),
    el('button', { class: `tab-btn ${adminTab === 'karaokes' ? 'active' : ''}`, onclick: () => { adminTab = 'karaokes'; refresh() } }, t('admin.karaokes')),
  )
  page.append(el('h1', { class: 'page-title' }, t('admin.title')), tabs)

  const wrap = document.createElement('div')
  page.append(wrap)

  async function refresh() {
    const seq = ++adminRefreshSeq
    wrap.innerHTML = ''
    if (adminTab === 'users') await renderUsers(wrap, seq)
    else if (adminTab === 'categories') await renderCategories(wrap, seq)
    else if (adminTab === 'songs') await renderSongs(wrap, seq)
    else await renderKaraokes(wrap, seq)
  }

  refresh()
  return page
}

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue
    if (k === 'class') node.className = v
    else if (k === 'style') node.setAttribute('style', v)
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2), v)
    else node.setAttribute(k, v === true ? '' : String(v))
  }
  for (const c of children.flat()) {
    if (c == null) continue
    node.append(c instanceof Node ? c : document.createTextNode(String(c)))
  }
  return node
}

function pluralCount(n, singular, pluralForm) {
  return `${n} ${n === 1 ? singular : pluralForm}`
}

// Renders the whole app (defined in app.js, not reachable from this module).
function refreshApp() {
  window.dispatchEvent(new Event('pm:rerender'))
}

// ---------- Users ----------

async function renderUsers(wrap, seq) {
  const [users, categories] = await Promise.all([
    endpoints.admin.users().catch(() => []),
    endpoints.admin.categories().catch(() => []),
  ])
  // Resposta obsoleta (outra aba já disparou um refresh mais novo): descarta
  // antes de tocar no estado compartilhado ou no DOM.
  if (!isAdminRefreshCurrent(seq)) return
  adminState.users = users
  adminState.categories = categories

  wrap.append(
    el(
      'div',
      { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: () => userForm() }, t('admin.newUser')),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (users.length === 0) table.append(el('p', { class: 'empty-state' }, t('admin.noUsers')))
  for (const u of users) {
    const chips = (u.categories ?? []).map((c) => el('span', { class: 'chip' }, c.name))
    table.append(
      el(
        'div',
        { class: 'admin-row' },
        el('div', { class: 'admin-row-main' },
          el('p', { class: 'admin-row-title' }, u.name, u.isAdmin ? el('span', { class: 'badge' }, t('admin.admin')) : null),
          el('p', { class: 'admin-row-sub' }, u.isAdmin ? `@${u.username}` : (u.phone || '')),
          el('div', { class: 'admin-chips' }, ...chips),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'icon-btn', 'aria-label': t('admin.edit'), onclick: () => userForm(u) }, '✎'),
          el('button', { class: 'icon-btn', 'aria-label': t('admin.delete'), onclick: () => deleteUser(u) }, '✕'),
        ),
      ),
    )
  }
  wrap.append(table)
}

function userForm(existing) {
  const isEdit = !!existing
  let isAdmin = !!existing?.isAdmin
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.name'), value: existing?.name ?? '', autocomplete: 'name' })
  const phoneInput = el('input', { class: 'form-input', type: 'tel', inputmode: 'numeric', placeholder: t('admin.phone'), value: existing && !existing.isAdmin ? existing.phone : '', autocomplete: 'tel' })
  phoneInput.addEventListener('input', () => applyPhoneMask(phoneInput))
  const usernameInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.username'), value: existing?.username ?? '', autocomplete: 'username' })
  const emailInput = el('input', { class: 'form-input', type: 'email', placeholder: t('admin.email'), value: existing?.email ?? '', autocomplete: 'email' })
  const passInput = el('input', { class: 'form-input', type: 'password', placeholder: isEdit ? t('admin.newPassword') : t('admin.password'), autocomplete: 'new-password' })
  const errorEl = el('p', { class: 'login-error' })

  const adminFields = el('div', {}, usernameInput, emailInput, passInput)
  const clientFields = el('div', {}, phoneInput)

  const catLabel = el('div', { class: 'modal-section-label' }, t('admin.grantedCategories'))
  const catsWrap = el('div', {},
    adminState.categories.map((c) => {
      const box = el('input', { type: 'checkbox', id: `cat-${c.id}` })
      if ((existing?.categories ?? []).some((cc) => cc.id === c.id)) box.checked = true
      return el('label', { class: 'modal-check', style: 'display:block' }, box, el('span', {}, c.name))
    }),
  )

  const adminBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': isAdmin ? 'true' : 'false', onclick: () => { isAdmin = true; sync() } }, t('admin.admin'))
  const clientBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': !isAdmin ? 'true' : 'false', onclick: () => { isAdmin = false; sync() } }, t('admin.client'))

  function sync() {
    adminBtn.classList.toggle('active', isAdmin)
    clientBtn.classList.toggle('active', !isAdmin)
    // aria-pressed acompanha o toggle (antes ficava preso ao estado inicial).
    adminBtn.setAttribute('aria-pressed', isAdmin ? 'true' : 'false')
    clientBtn.setAttribute('aria-pressed', !isAdmin ? 'true' : 'false')
    adminFields.style.display = isAdmin ? '' : 'none'
    clientFields.style.display = isAdmin ? 'none' : ''
    catLabel.style.display = isAdmin ? 'none' : ''
    catsWrap.style.display = isAdmin ? 'none' : ''
  }

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal' },
      el('h3', {}, isEdit ? t('admin.editUser') : t('admin.newUserTitle')),
      el('div', { class: 'login-toggle' }, clientBtn, adminBtn),
      nameInput,
      clientFields,
      adminFields,
      catLabel,
      catsWrap,
      errorEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, t('admin.save')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)
  sync()

  async function save() {
    errorEl.textContent = ''
    const payload = { name: nameInput.value.trim(), isAdmin }
    if (isAdmin) {
      // Nota (contrato): o backend LIMPA o telefone ao virar admin
      // (handleAdminUpdateUser faz patch.Phone = "" no ramo admin) — enviar
      // phone aqui seria ignorado. O telefone só entra no payload do ramo
      // cliente, onde o backend o exige.
      payload.username = usernameInput.value.trim()
      payload.email = emailInput.value.trim()
      if (passInput.value) payload.password = passInput.value
    } else {
      payload.phone = phoneInput.value.trim()
      payload.categoryIds = [...overlay.querySelectorAll('input[type=checkbox]:checked')].map((b) => b.id.slice(4))
    }
    try {
      if (isEdit) await endpoints.admin.updateUser(existing.id, payload)
      else await endpoints.admin.createUser(payload)
      overlay.remove()
      refreshApp()
    } catch (err) {
      errorEl.textContent = err.message
    }
  }
}

async function deleteUser(u) {
  if (!window.confirm(t('admin.confirmDeleteUser', { name: u.name }))) return
  try {
    await endpoints.admin.deleteUser(u.id)
    refreshApp()
  } catch (err) {
    alert(err.message)
  }
}

// ---------- Categories ----------

// newCategoryForm: modal with name + checkout link + photo (used for creation).
function newCategoryForm() {
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.categoryName'), autofocus: true })
  const urlInput = el('input', { class: 'form-input', type: 'url', placeholder: 'https://checkout.exemplo.com/...', autocomplete: 'off' })
  const errorEl = el('p', { class: 'login-error' })

  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, t('admin.categoryPhoto')),
  )
  photoDrop.addEventListener('click', () => photoInput.click())
  photoInput.addEventListener('change', () => {
    const f = photoInput.files[0]
    if (f) photoDrop.querySelector('span:last-child').textContent = f.name
  })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal' },
      el('h3', {}, t('admin.newCategory')),
      nameInput,
      el('label', { class: 'upload-field' },
        el('span', { class: 'upload-label' }, t('admin.checkout')),
        urlInput,
      ),
      el('div', { class: 'modal-section-label' }, t('admin.categoryPhoto')),
      photoDrop,
      photoInput,
      errorEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, t('admin.create')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function save() {
    errorEl.textContent = ''
    const name = nameInput.value.trim()
    if (!name) { errorEl.textContent = t('admin.categoryNameRequired'); return }
    const btn = overlay.querySelector('.btn-accent')
    btn.disabled = true
    btn.textContent = t('admin.saving')
    try {
      const cat = await endpoints.admin.createCategory(name, urlInput.value.trim())
      // A categoria precisa existir antes do upload da foto (id gerado no create).
      if (photoInput.files[0]) {
        await endpoints.admin.uploadCategoryPhoto(cat.id, photoInput.files[0])
      }
      overlay.remove()
      refreshApp()
    } catch (err) {
      errorEl.textContent = err.message
      btn.disabled = false
      btn.textContent = t('admin.create')
    }
  }
}

async function renderCategories(wrap, seq) {
  const categories = await endpoints.admin.categories().catch(() => [])
  if (!isAdminRefreshCurrent(seq)) return
  adminState.categories = categories

  wrap.append(
    el('div', { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: newCategoryForm }, t('admin.newCategory')),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (categories.length === 0) table.append(el('p', { class: 'empty-state' }, t('admin.noCategories')))
  for (const c of categories) {
    table.append(
      el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main', style: 'cursor:pointer' },
          el('p', { class: 'admin-row-title', onclick: () => categoryForm(c) }, c.name),
          el('p', { class: 'admin-row-sub' }, [
            plural('count.songs', c.songCount ?? 0),
            (c.karaokeCount ?? 0) > 0 ? plural('count.karaokes', c.karaokeCount) : null,
          ].filter(Boolean).join(' • ')),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => categoryForm(c) }, t('admin.manage')),
          el('button', { class: 'icon-btn', 'aria-label': t('admin.delete'), onclick: () => deleteCategory(c) }, '✕'),
        ),
      ),
    )
  }
  wrap.append(table)
}

function categoryForm(cat) {
  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-wide' },
      el('h3', {}, `${t('admin.categoryName')}: ${cat.name}`),
      el('input', { class: 'form-input', id: 'cat-name', type: 'text', value: cat.name, placeholder: t('admin.categoryName') }),
      el('input', { class: 'form-input', id: 'cat-checkout', type: 'url', value: cat.checkoutUrl || '', placeholder: t('admin.checkoutHint'), autocomplete: 'off' }),
      el('div', { class: 'modal-section-label' }, t('admin.categoryPhoto')),
      el('div', { style: 'display:flex;align-items:center;gap:12px' },
        el('img', { id: 'cat-photo-preview', class: 'track-art', src: artworkUrl(cat.id, 96), alt: '', style: 'width:56px;height:56px;border-radius:8px;display:block;object-fit:cover' }),
        el('div', { style: 'display:flex;flex-direction:column;gap:6px' },
          el('button', { class: 'btn-secondary', onclick: () => uploadCatPhoto() }, t('admin.uploadPhoto')),
          el('button', { class: 'btn-secondary', onclick: () => removeCatPhoto() }, t('admin.removePhoto')),
        ),
      ),
      el('div', { class: 'modal-section-label' }, t('admin.songs')),
      el('input', { class: 'form-input', id: 'cat-song-filter', type: 'text', placeholder: t('admin.filterSongs') }),
      el('div', { class: 'modal-scroll', id: 'cat-songs' }),
      el('div', { class: 'modal-section-label' }, t('admin.karaokes')),
      el('input', { class: 'form-input', id: 'cat-karaoke-filter', type: 'text', placeholder: t('admin.filterKaraokes') }),
      el('div', { class: 'modal-scroll', id: 'cat-karaokes' }),
      el('p', { class: 'login-error', id: 'cat-error' }),
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, t('admin.save')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  const songsBox = overlay.querySelector('#cat-songs')
  const karaokesBox = overlay.querySelector('#cat-karaokes')
  let assigned = { songIds: [], karaokeIds: [] }

  endpoints.admin.category(cat.id).then((d) => {
    assigned = { songIds: d.songIds ?? [], karaokeIds: d.karaokeIds ?? [] }
    buildList('')
    buildKaraokeList('')
  }).catch(() => { buildList(''); buildKaraokeList('') })

  function buildList(filter) {
    songsBox.innerHTML = ''
    const needSongs = adminState.songs.length === 0
    ;(needSongs ? endpoints.admin.songs() : Promise.resolve({ songs: adminState.songs, categoryIds: adminState.songCategoryIds }))
      .then((data) => {
        const songs = data.songs ?? []
        adminState.songs = songs
        adminState.songCategoryIds = data.categoryIds ?? {}
        const f = filter.toLowerCase()
        for (const s of songs) {
          if (f && !s.title.toLowerCase().includes(f) && !(s.artist || '').toLowerCase().includes(f)) continue
          const box = el('input', { type: 'checkbox', id: `song-${s.id}` })
          box.checked = assigned.songIds.includes(s.id)
          box.addEventListener('change', () => {
            if (box.checked) assigned.songIds.push(s.id)
            else assigned.songIds = assigned.songIds.filter((x) => x !== s.id)
          })
          songsBox.append(
            el('div', { class: 'admin-row' },
              el('div', { class: 'admin-row-main', style: 'display:flex;align-items:center;gap:8px' },
                el('img', { class: 'track-art', src: artworkUrl(s.id, 48), alt: '' }),
                box,
                el('span', {}, `${s.title}${s.artist ? ` — ${s.artist}` : ''}`),
              ),
              el('div', { class: 'admin-row-actions' },
                el('button', { class: 'btn-secondary', onclick: () => uploadSongPhoto(s) }, t('admin.sendPhoto')),
                el('button', { class: 'btn-secondary', onclick: () => removeSongPhoto(s) }, t('admin.removePhoto')),
              ),
            ),
          )
        }
        if (songsBox.children.length === 0) songsBox.append(el('p', { class: 'modal-empty' }, t('admin.noSongMatch')))
      })
  }

  function buildKaraokeList(filter) {
    karaokesBox.innerHTML = ''
    // Filtra em memória quando a lista já foi carregada; busca no servidor
    // apenas na primeira vez (evita N requisições por tecla digitada).
    if (adminState.karaokes.length === 0) {
      endpoints.admin.karaokes()
        .then((data) => {
          adminState.karaokes = data.karaokes ?? []
          renderKaraokeRows(filter)
        })
        .catch(() => {
          karaokesBox.append(el('p', { class: 'modal-empty' }, t('admin.noKaraokesAvailable')))
        })
      return
    }
    renderKaraokeRows(filter)
  }

  function renderKaraokeRows(filter) {
    const list = adminState.karaokes
    const f = filter.toLowerCase()
    for (const k of list) {
      if (f && !k.title.toLowerCase().includes(f) && !(k.artist || '').toLowerCase().includes(f)) continue
      const box = el('input', { type: 'checkbox', id: `karaoke-${k.id}` })
      box.checked = assigned.karaokeIds.includes(k.id)
      box.addEventListener('change', () => {
        if (box.checked) assigned.karaokeIds.push(k.id)
        else assigned.karaokeIds = assigned.karaokeIds.filter((x) => x !== k.id)
      })
      karaokesBox.append(
        el('div', { class: 'admin-row' },
          el('div', { class: 'admin-row-main', style: 'display:flex;align-items:center;gap:8px' },
            el('img', { class: 'track-art', src: artworkUrl(k.id, 48), alt: '' }),
            box,
            el('span', {}, `${k.title}${k.artist ? ` — ${k.artist}` : ''}`),
          ),
        ),
      )
    }
    if (karaokesBox.children.length === 0) karaokesBox.append(el('p', { class: 'modal-empty' }, t('admin.noKaraokeMatch')))
  }

  overlay.querySelector('#cat-song-filter').addEventListener('input', (e) => buildList(e.target.value))
  overlay.querySelector('#cat-karaoke-filter').addEventListener('input', (e) => buildKaraokeList(e.target.value))

  function uploadSongPhoto(s) {
    const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
    input.addEventListener('change', async () => {
      const file = input.files[0]
      if (!file) return
      try {
        await endpoints.admin.uploadSongPhoto(s.id, file)
        alert(t('admin.photoUpdated'))
      } catch (err) {
        alert(err.message)
      }
    })
    document.body.append(input)
    input.click()
  }

  async function removeSongPhoto(s) {
    try {
      await endpoints.admin.deleteSongPhoto(s.id)
      alert(t('admin.photoRemoved'))
    } catch (err) {
      alert(err.message)
    }
  }

  function refreshCatPhotoPreview() {
    const preview = overlay.querySelector('#cat-photo-preview')
    if (preview) preview.src = artworkUrl(cat.id, 96) + '&t=' + Date.now()
  }

  function uploadCatPhoto() {
    const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
    input.addEventListener('change', async () => {
      const file = input.files[0]
      if (!file) return
      try {
        await endpoints.admin.uploadCategoryPhoto(cat.id, file)
        refreshCatPhotoPreview()
      } catch (err) {
        alert(err.message)
      }
    })
    document.body.append(input)
    input.click()
  }

  async function removeCatPhoto() {
    if (!window.confirm(t('admin.confirmDeleteCategory', { name: cat.name }))) return
    try {
      await endpoints.admin.deleteCategoryPhoto(cat.id)
      refreshCatPhotoPreview()
    } catch (err) {
      alert(err.message)
    }
  }

  async function save() {
    const errEl = overlay.querySelector('#cat-error')
    errEl.textContent = ''
    const name = overlay.querySelector('#cat-name').value.trim()
    const checkoutUrl = overlay.querySelector('#cat-checkout').value.trim()
    const saveBtn = overlay.querySelector('.btn-accent')
    // Desabilita durante o save: evita duplo submit (cliques repetidos).
    saveBtn.disabled = true
    try {
      await endpoints.admin.updateCategory(cat.id, { name, checkoutUrl, songIds: assigned.songIds, karaokeIds: assigned.karaokeIds })
      overlay.remove()
      refreshApp()
    } catch (err) {
      errEl.textContent = err.message
    } finally {
      saveBtn.disabled = false
    }
  }
}

async function deleteCategory(c) {
  if (!window.confirm(t('admin.confirmDeleteCategory', { name: c.name }))) return
  try {
    await endpoints.admin.deleteCategory(c.id)
    refreshApp()
  } catch (err) {
    alert(err.message)
  }
}

// ---------- Songs (upload + list) ----------

async function renderSongs(wrap, seq) {
  const data = await endpoints.admin.songs().catch(() => ({ songs: [], categoryIds: {}, categoryList: [] }))
  if (!isAdminRefreshCurrent(seq)) return
  const songs = data.songs ?? []
  adminState.songs = songs
  adminState.songCategoryIds = data.categoryIds ?? {}
  adminState.categoryNames = Object.fromEntries((data.categoryList ?? []).map((c) => [c.id, c.name]))
  adminState.categories = data.categoryList ?? []

  wrap.append(
    el('div', { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: uploadForm }, t('admin.uploadSong')),
      el('button', { class: 'btn-secondary', onclick: uploadFolderForm }, t('admin.uploadFolder')),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (songs.length === 0) {
    table.append(el('p', { class: 'empty-state' }, t('admin.noSongs')))
  }
  for (const s of songs) {
    const cats = (adminState.songCategoryIds[s.id] ?? []).map((cid) => adminState.categoryNames[cid]).filter(Boolean)
    const chips = cats.map((n) => el('span', { class: 'chip' }, n))
    table.append(
      el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main', style: 'display:flex;align-items:center;gap:8px' },
          el('img', { class: 'track-art', src: artworkUrl(s.id, 48), alt: '' }),
          el('div', { style: 'min-width:0' },
            el('p', { class: 'admin-row-title' }, s.title),
            el('p', { class: 'admin-row-sub' }, [s.artist || t('common.unknown'), s.format, fmtDur(s.duration)].filter(Boolean).join(' • ')),
            el('div', { class: 'admin-chips' }, ...chips),
          ),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => uploadSongPhoto(s) }, t('admin.sendPhoto')),
          el('button', { class: 'btn-secondary', onclick: () => removeSongPhoto(s) }, t('admin.removePhoto')),
        ),
      ),
    )
  }
  wrap.append(table)
}

function fmtDur(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return ''
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

function uploadForm() {
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'audio/*' })
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#9835;' }),
    el('p', { class: 'upload-dropzone-title' }, t('admin.uploadAudioTitle')),
    el('p', { class: 'upload-dropzone-hint' }, 'mp3, m4a, flac, ogg, wav…'),
  )
  dropzone.addEventListener('click', () => fileInput.click())
  dropzone.addEventListener('dragover', (e) => { e.preventDefault(); dropzone.classList.add('drag') })
  dropzone.addEventListener('dragleave', () => dropzone.classList.remove('drag'))
  dropzone.addEventListener('drop', (e) => {
    e.preventDefault()
    dropzone.classList.remove('drag')
    if (e.dataTransfer.files.length) fileInput.files = e.dataTransfer.files
    updateFile()
  })
  fileInput.addEventListener('change', updateFile)
  function updateFile() {
    const f = fileInput.files[0]
    if (!f) return
    dropzone.classList.add('has-file')
    dropzone.querySelector('.upload-dropzone-title').textContent = f.name
    dropzone.querySelector('.upload-dropzone-hint').textContent = `${(f.size / 1024 / 1024).toFixed(2)} MB`
  }

  const titleInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.titleOptional'), autocomplete: 'off' })
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.artistOptional'), autocomplete: 'off' })
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, t('admin.noCategory')),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, t('admin.songPhoto')),
  )
  photoDrop.addEventListener('click', () => photoInput.click())
  photoInput.addEventListener('change', () => {
    const f = photoInput.files[0]
    if (f) photoDrop.querySelector('span:last-child').textContent = f.name
  })
  const statusEl = el('p', { class: 'login-error' })

  const field = (label, control) =>
    el('label', { class: 'upload-field' }, el('span', { class: 'upload-label' }, label), control)

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-upload' },
      el('h3', {}, t('admin.uploadAudioTitle')),
      el('div', { class: 'modal-section-label' }, t('admin.fileAudio')),
      dropzone,
      fileInput,
      el('div', { class: 'upload-grid' },
        field(t('admin.titleOptional'), titleInput),
        field(t('admin.artistOptional'), artistInput),
      ),
      field(t('admin.category'), catSelect),
      el('div', { class: 'modal-section-label' }, t('admin.songPhoto')),
      photoDrop,
      photoInput,
      el('p', { class: 'upload-info' }, t('admin.photoHint')),
      statusEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: submit }, t('admin.uploadSong')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function submit() {
    const file = fileInput.files[0]
    if (!file) {
      statusEl.textContent = t('admin.selectAudio')
      dropzone.classList.add('error')
      return
    }
    const fd = new FormData()
    fd.append('song', file)
    if (titleInput.value.trim()) fd.append('title', titleInput.value.trim())
    if (artistInput.value.trim()) fd.append('artist', artistInput.value.trim())
    if (catSelect.value) fd.append('categoryId', catSelect.value)
    if (photoInput.files[0]) fd.append('photo', photoInput.files[0])

    const btn = overlay.querySelector('.btn-accent')
    btn.disabled = true
    btn.textContent = t('admin.uploading')
    statusEl.textContent = t('admin.uploadingIndex')
    statusEl.classList.remove('login-error')
    statusEl.classList.add('upload-info')
    try {
      await endpoints.admin.uploadSong(fd)
      overlay.remove()
      alert(t('admin.uploaded'))
      refreshApp()
    } catch (err) {
      statusEl.textContent = err.message
      statusEl.classList.remove('upload-info')
      statusEl.classList.add('login-error')
      btn.disabled = false
      btn.textContent = t('admin.uploadSong')
    }
  }
}

// Audio extensions accepted by the folder batch (same list as the scanner).
const FOLDER_AUDIO_EXTS = ['.mp3', '.flac', '.m4a', '.aac', '.ogg', '.opus', '.wav', '.wma', '.aiff', '.aif', '.wv', '.tak', '.ape']

function folderAudioExt(name) {
  const i = name.lastIndexOf('.')
  if (i < 0) return ''
  return name.slice(i).toLowerCase()
}

function fmtMB(bytes) {
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function uploadFolderForm() {
  // webkitdirectory lets the user pick a whole folder; each File keeps its
  // relative path but only the audio files matter here.
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', webkitdirectory: true, multiple: true })
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#128193;' }),
    el('p', { class: 'upload-dropzone-title' }, t('admin.selectFolder')),
    el('p', { class: 'upload-dropzone-hint' }, t('admin.folderHint')),
  )
  dropzone.addEventListener('click', () => fileInput.click())
  dropzone.addEventListener('dragover', (e) => { e.preventDefault(); dropzone.classList.add('drag') })
  dropzone.addEventListener('dragleave', () => dropzone.classList.remove('drag'))
  dropzone.addEventListener('drop', (e) => {
    e.preventDefault()
    dropzone.classList.remove('drag')
    if (e.dataTransfer.files.length) fileInput.files = e.dataTransfer.files
    updateFiles()
  })
  fileInput.addEventListener('change', updateFiles)

  let selected = []
  function updateFiles() {
    selected = Array.from(fileInput.files || []).filter((f) => FOLDER_AUDIO_EXTS.includes(folderAudioExt(f.name)))
    const total = selected.reduce((acc, f) => acc + f.size, 0)
    dropzone.classList.toggle('has-file', selected.length > 0)
    if (selected.length === 0) {
      dropzone.querySelector('.upload-dropzone-title').textContent = t('admin.noAudioFound')
      dropzone.querySelector('.upload-dropzone-hint').textContent = t('admin.chooseOtherFolder')
    } else {
      dropzone.querySelector('.upload-dropzone-title').textContent = plural('admin.songsInFolder', selected.length)
      dropzone.querySelector('.upload-dropzone-hint').textContent = t('admin.totalSize', { n: fmtMB(total) })
    }
    updateStartBtn()
  }

  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, t('admin.selectCategory')),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  catSelect.addEventListener('change', updateStartBtn)

  const statusEl = el('p', { class: 'login-error' })
  const progressWrap = el('div', { class: 'upload-progress', style: 'display:none' },
    el('div', { class: 'upload-progress-bar' }, el('div', { class: 'upload-progress-fill', style: 'width:0%' })),
    el('p', { class: 'upload-progress-text' }, ''),
  )
  const failsEl = el('div', { class: 'upload-fails', style: 'display:none' })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-upload' },
      el('h3', {}, t('admin.uploadFolderTitle')),
      dropzone,
      fileInput,
      el('div', { class: 'modal-section-label' }, t('admin.folderCategory')),
      catSelect,
      el('p', { class: 'upload-info' }, t('admin.folderInfo')),
      statusEl,
      progressWrap,
      failsEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', id: 'folder-upload-start', onclick: start }, t('admin.uploadFolder')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  const startBtn = overlay.querySelector('#folder-upload-start')
  const fillEl = progressWrap.querySelector('.upload-progress-fill')
  const textEl = progressWrap.querySelector('.upload-progress-text')

  function updateStartBtn() {
    startBtn.disabled = selected.length === 0 || !catSelect.value
  }

  async function start() {
    if (selected.length === 0) {
      statusEl.textContent = t('admin.selectFolder')
      statusEl.classList.add('login-error')
      return
    }
    if (!catSelect.value) {
      statusEl.textContent = t('admin.chooseCategory')
      statusEl.classList.add('login-error')
      return
    }
    statusEl.textContent = ''
    statusEl.classList.remove('login-error', 'upload-info')
    progressWrap.style.display = ''
    failsEl.style.display = 'none'
    failsEl.innerHTML = ''
    startBtn.disabled = true
    const cancelBtn = overlay.querySelector('.btn-secondary')
    cancelBtn.disabled = true
    cancelBtn.textContent = t('login.waiting')

    let ok = 0
    const fails = []
    for (let i = 0; i < selected.length; i++) {
      const f = selected[i]
      textEl.textContent = t('admin.sending', { i: i + 1, total: selected.length, name: f.name, size: fmtMB(f.size) })
      fillEl.style.width = `${Math.round((i / selected.length) * 100)}%`
      const fd = new FormData()
      fd.append('song', f)
      fd.append('categoryId', catSelect.value)
      try {
        await endpoints.admin.uploadSong(fd)
        ok++
      } catch (err) {
        fails.push({ name: f.name, msg: err.message })
        failsEl.style.display = ''
        const row = el('p', { class: 'upload-fail' }, '✕ ', el('strong', {}, f.name), ` — ${err.message}`)
        failsEl.append(row)
      }
    }
    fillEl.style.width = '100%'
    textEl.textContent = t('admin.done', { ok, fails: fails.length ? `, ${fails.length} ${fails.length === 1 ? t('admin.fail') : t('admin.fails')}` : '' })
    startBtn.disabled = false
    startBtn.textContent = t('common.close')
    startBtn.onclick = () => overlay.remove()
    cancelBtn.style.display = 'none'
    refreshApp()
  }
}

function uploadSongPhoto(s) {
  const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
  input.addEventListener('change', async () => {
    const file = input.files[0]
    if (!file) return
    try {
      await endpoints.admin.uploadSongPhoto(s.id, file)
      alert(t('admin.photoUpdated'))
    } catch (err) {
      alert(err.message)
    }
  })
  document.body.append(input)
  input.click()
}

async function removeSongPhoto(s) {
  try {
    await endpoints.admin.deleteSongPhoto(s.id)
    alert(t('admin.photoRemoved'))
  } catch (err) {
    alert(err.message)
  }
}

// ---------- Karaokes (upload + list) ----------

async function renderKaraokes(wrap, seq) {
  const data = await endpoints.admin.karaokes().catch(() => ({ karaokes: [], categoryIds: {}, categoryList: [] }))
  if (!isAdminRefreshCurrent(seq)) return
  const list = data.karaokes ?? []
  adminState.karaokes = list
  adminState.karaokeCategoryIds = data.categoryIds ?? {}
  adminState.categoryNames = Object.fromEntries((data.categoryList ?? []).map((c) => [c.id, c.name]))
  adminState.categories = data.categoryList ?? []

  wrap.append(
    el('div', { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: uploadKaraokeForm }, t('admin.uploadVideo')),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (list.length === 0) {
    table.append(el('p', { class: 'empty-state' }, t('admin.noKaraokes')))
  }
  for (const k of list) {
    const cats = (adminState.karaokeCategoryIds[k.id] ?? []).map((cid) => adminState.categoryNames[cid]).filter(Boolean)
    const chips = cats.map((n) => el('span', { class: 'chip' }, n))
    table.append(
      el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main', style: 'display:flex;align-items:center;gap:8px' },
          el('img', { class: 'track-art', src: artworkUrl(k.id, 48), alt: '', style: 'border-radius:6px;object-fit:cover' }),
          el('div', { style: 'min-width:0' },
            el('p', { class: 'admin-row-title' }, k.title),
            el('p', { class: 'admin-row-sub' }, [k.artist || t('common.unknown'), k.format, fmtDur(k.duration)].filter(Boolean).join(' • ')),
            el('div', { class: 'admin-chips' }, ...chips),
          ),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => uploadKaraokePhoto(k) }, t('admin.sendPhoto')),
          el('button', { class: 'btn-secondary', onclick: () => removeKaraokePhoto(k) }, t('admin.removePhoto')),
        ),
      ),
    )
  }
  wrap.append(table)
}

function uploadKaraokeForm() {
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'video/mp4,video/webm,video/x-matroska,.mp4,.webm,.mkv' })
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#127909;' }),
    el('p', { class: 'upload-dropzone-title' }, t('admin.uploadVideoTitle')),
    el('p', { class: 'upload-dropzone-hint' }, 'mp4, webm, mkv…'),
  )
  dropzone.addEventListener('click', () => fileInput.click())
  dropzone.addEventListener('dragover', (e) => { e.preventDefault(); dropzone.classList.add('drag') })
  dropzone.addEventListener('dragleave', () => dropzone.classList.remove('drag'))
  dropzone.addEventListener('drop', (e) => {
    e.preventDefault()
    dropzone.classList.remove('drag')
    if (e.dataTransfer.files.length) fileInput.files = e.dataTransfer.files
    updateFile()
  })
  fileInput.addEventListener('change', updateFile)
  function updateFile() {
    const f = fileInput.files[0]
    if (!f) return
    dropzone.classList.add('has-file')
    dropzone.querySelector('.upload-dropzone-title').textContent = f.name
    dropzone.querySelector('.upload-dropzone-hint').textContent = `${(f.size / 1024 / 1024).toFixed(2)} MB`
  }

  const titleInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.titleOptional'), autocomplete: 'off' })
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: t('admin.artistOptional'), autocomplete: 'off' })
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, t('admin.noCategory')),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, t('admin.videoPhoto')),
  )
  photoDrop.addEventListener('click', () => photoInput.click())
  photoInput.addEventListener('change', () => {
    const f = photoInput.files[0]
    if (f) photoDrop.querySelector('span:last-child').textContent = f.name
  })
  const statusEl = el('p', { class: 'login-error' })

  const field = (label, control) =>
    el('label', { class: 'upload-field' }, el('span', { class: 'upload-label' }, label), control)

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-upload' },
      el('h3', {}, t('admin.uploadVideoTitle')),
      el('div', { class: 'modal-section-label' }, t('admin.fileVideo')),
      dropzone,
      fileInput,
      el('div', { class: 'upload-grid' },
        field(t('admin.titleOptional'), titleInput),
        field(t('admin.artistOptional'), artistInput),
      ),
      field(t('admin.category'), catSelect),
      el('div', { class: 'modal-section-label' }, t('admin.videoPhoto')),
      photoDrop,
      photoInput,
      el('p', { class: 'upload-info' }, t('admin.thumbHint')),
      statusEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: submit }, t('admin.uploadVideo')),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, t('admin.cancel')),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function submit() {
    const file = fileInput.files[0]
    if (!file) {
      statusEl.textContent = t('admin.selectVideo')
      dropzone.classList.add('error')
      return
    }
    const fd = new FormData()
    fd.append('video', file)
    if (titleInput.value.trim()) fd.append('title', titleInput.value.trim())
    if (artistInput.value.trim()) fd.append('artist', artistInput.value.trim())
    if (catSelect.value) fd.append('categoryId', catSelect.value)
    if (photoInput.files[0]) fd.append('photo', photoInput.files[0])

    const btn = overlay.querySelector('.btn-accent')
    btn.disabled = true
    btn.textContent = t('admin.uploading')
    statusEl.textContent = t('admin.uploadingIndex')
    statusEl.classList.remove('login-error')
    statusEl.classList.add('upload-info')
    try {
      await endpoints.admin.uploadKaraoke(fd)
      overlay.remove()
      alert(t('admin.videoUploaded'))
      refreshApp()
    } catch (err) {
      statusEl.textContent = err.message
      statusEl.classList.remove('upload-info')
      statusEl.classList.add('login-error')
      btn.disabled = false
      btn.textContent = t('admin.uploadVideo')
    }
  }
}

function uploadKaraokePhoto(k) {
  const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
  input.addEventListener('change', async () => {
    const file = input.files[0]
    if (!file) return
    try {
      await endpoints.admin.uploadKaraokePhoto(k.id, file)
      alert(t('admin.photoUpdated'))
    } catch (err) {
      alert(err.message)
    }
  })
  document.body.append(input)
  input.click()
}

async function removeKaraokePhoto(k) {
  try {
    await endpoints.admin.deleteKaraokePhoto(k.id)
    alert(t('admin.photoRemovedThumb'))
  } catch (err) {
    alert(err.message)
  }
}
