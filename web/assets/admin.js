// Admin page: users, categories (songs per category) and song uploads.

import { endpoints, artworkUrl, applyPhoneMask } from './api.js'

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
    el('button', { class: `tab-btn ${adminTab === 'users' ? 'active' : ''}`, onclick: () => { adminTab = 'users'; refresh() } }, 'Usuários'),
    el('button', { class: `tab-btn ${adminTab === 'categories' ? 'active' : ''}`, onclick: () => { adminTab = 'categories'; refresh() } }, 'Categorias'),
    el('button', { class: `tab-btn ${adminTab === 'songs' ? 'active' : ''}`, onclick: () => { adminTab = 'songs'; refresh() } }, 'Músicas'),
    el('button', { class: `tab-btn ${adminTab === 'karaokes' ? 'active' : ''}`, onclick: () => { adminTab = 'karaokes'; refresh() } }, 'Karaokês'),
  )
  page.append(el('h1', { class: 'page-title' }, 'Administração'), tabs)

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

function plural(n, singular, pluralForm) {
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
      el('button', { class: 'btn-accent', onclick: () => userForm() }, 'Novo usuário'),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (users.length === 0) table.append(el('p', { class: 'empty-state' }, 'Nenhum usuário ainda.'))
  for (const u of users) {
    const chips = (u.categories ?? []).map((c) => el('span', { class: 'chip' }, c.name))
    table.append(
      el(
        'div',
        { class: 'admin-row' },
        el('div', { class: 'admin-row-main' },
          el('p', { class: 'admin-row-title' }, u.name, u.isAdmin ? el('span', { class: 'badge' }, 'Admin') : null),
          el('p', { class: 'admin-row-sub' }, u.isAdmin ? `@${u.username}` : (u.phone || '')),
          el('div', { class: 'admin-chips' }, ...chips),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'icon-btn', 'aria-label': 'Editar', onclick: () => userForm(u) }, '✎'),
          el('button', { class: 'icon-btn', 'aria-label': 'Excluir', onclick: () => deleteUser(u) }, '✕'),
        ),
      ),
    )
  }
  wrap.append(table)
}

function userForm(existing) {
  const isEdit = !!existing
  let isAdmin = !!existing?.isAdmin
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Nome', value: existing?.name ?? '', autocomplete: 'name' })
  const phoneInput = el('input', { class: 'form-input', type: 'tel', inputmode: 'numeric', placeholder: 'Telefone (99) 99999-9999', value: existing && !existing.isAdmin ? existing.phone : '', autocomplete: 'tel' })
  phoneInput.addEventListener('input', () => applyPhoneMask(phoneInput))
  const usernameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Usuário', value: existing?.username ?? '', autocomplete: 'username' })
  const emailInput = el('input', { class: 'form-input', type: 'email', placeholder: 'E-mail', value: existing?.email ?? '', autocomplete: 'email' })
  const passInput = el('input', { class: 'form-input', type: 'password', placeholder: isEdit ? 'Nova senha (opcional)' : 'Senha', autocomplete: 'new-password' })
  const errorEl = el('p', { class: 'login-error' })

  const adminFields = el('div', {}, usernameInput, emailInput, passInput)
  const clientFields = el('div', {}, phoneInput)

  const catLabel = el('div', { class: 'modal-section-label' }, 'Categorias liberadas')
  const catsWrap = el('div', {},
    adminState.categories.map((c) => {
      const box = el('input', { type: 'checkbox', id: `cat-${c.id}` })
      if ((existing?.categories ?? []).some((cc) => cc.id === c.id)) box.checked = true
      return el('label', { class: 'modal-check', style: 'display:block' }, box, el('span', {}, c.name))
    }),
  )

  const adminBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': isAdmin ? 'true' : 'false', onclick: () => { isAdmin = true; sync() } }, 'Administrador')
  const clientBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': !isAdmin ? 'true' : 'false', onclick: () => { isAdmin = false; sync() } }, 'Cliente')

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
      el('h3', {}, isEdit ? 'Editar usuário' : 'Novo usuário'),
      el('div', { class: 'login-toggle' }, clientBtn, adminBtn),
      nameInput,
      clientFields,
      adminFields,
      catLabel,
      catsWrap,
      errorEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, 'Salvar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
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
  if (!window.confirm(`Excluir o usuário “${u.name}”?`)) return
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
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Nome da categoria', autofocus: true })
  const urlInput = el('input', { class: 'form-input', type: 'url', placeholder: 'https://checkout.exemplo.com/...', autocomplete: 'off' })
  const errorEl = el('p', { class: 'login-error' })

  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, 'Foto da categoria (opcional)'),
  )
  photoDrop.addEventListener('click', () => photoInput.click())
  photoInput.addEventListener('change', () => {
    const f = photoInput.files[0]
    if (f) photoDrop.querySelector('span:last-child').textContent = f.name
  })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal' },
      el('h3', {}, 'Nova categoria'),
      nameInput,
      el('label', { class: 'upload-field' },
        el('span', { class: 'upload-label' }, 'Link do checkout (loja)'),
        urlInput,
      ),
      el('div', { class: 'modal-section-label' }, 'Foto da categoria (opcional)'),
      photoDrop,
      photoInput,
      errorEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, 'Criar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function save() {
    errorEl.textContent = ''
    const name = nameInput.value.trim()
    if (!name) { errorEl.textContent = 'Informe o nome da categoria.'; return }
    const btn = overlay.querySelector('.btn-accent')
    btn.disabled = true
    btn.textContent = 'Salvando…'
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
      btn.textContent = 'Criar'
    }
  }
}

async function renderCategories(wrap, seq) {
  const categories = await endpoints.admin.categories().catch(() => [])
  if (!isAdminRefreshCurrent(seq)) return
  adminState.categories = categories

  wrap.append(
    el('div', { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: newCategoryForm }, 'Nova categoria'),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (categories.length === 0) table.append(el('p', { class: 'empty-state' }, 'Nenhuma categoria ainda.'))
  for (const c of categories) {
    table.append(
      el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main', style: 'cursor:pointer' },
          el('p', { class: 'admin-row-title', onclick: () => categoryForm(c) }, c.name),
          el('p', { class: 'admin-row-sub' }, [
            plural(c.songCount ?? 0, 'música', 'músicas'),
            (c.karaokeCount ?? 0) > 0 ? plural(c.karaokeCount, 'karaokê', 'karaokês') : null,
          ].filter(Boolean).join(' • ')),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => categoryForm(c) }, 'Gerenciar'),
          el('button', { class: 'icon-btn', 'aria-label': 'Excluir', onclick: () => deleteCategory(c) }, '✕'),
        ),
      ),
    )
  }
  wrap.append(table)
}

function categoryForm(cat) {
  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-wide' },
      el('h3', {}, `Categoria: ${cat.name}`),
      el('input', { class: 'form-input', id: 'cat-name', type: 'text', value: cat.name, placeholder: 'Nome da categoria' }),
      el('input', { class: 'form-input', id: 'cat-checkout', type: 'url', value: cat.checkoutUrl || '', placeholder: 'Link do checkout (loja) — ex.: https://checkout.exemplo.com/cristao', autocomplete: 'off' }),
      el('div', { class: 'modal-section-label' }, 'Foto da categoria'),
      el('div', { style: 'display:flex;align-items:center;gap:12px' },
        el('img', { id: 'cat-photo-preview', class: 'track-art', src: artworkUrl(cat.id, 96), alt: '', style: 'width:56px;height:56px;border-radius:8px;display:block;object-fit:cover' }),
        el('div', { style: 'display:flex;flex-direction:column;gap:6px' },
          el('button', { class: 'btn-secondary', onclick: () => uploadCatPhoto() }, 'Enviar foto'),
          el('button', { class: 'btn-secondary', onclick: () => removeCatPhoto() }, 'Remover foto'),
        ),
      ),
      el('div', { class: 'modal-section-label' }, 'Músicas'),
      el('input', { class: 'form-input', id: 'cat-song-filter', type: 'text', placeholder: 'Filtrar músicas…' }),
      el('div', { class: 'modal-scroll', id: 'cat-songs' }),
      el('div', { class: 'modal-section-label' }, 'Karaokês'),
      el('input', { class: 'form-input', id: 'cat-karaoke-filter', type: 'text', placeholder: 'Filtrar karaokês…' }),
      el('div', { class: 'modal-scroll', id: 'cat-karaokes' }),
      el('p', { class: 'login-error', id: 'cat-error' }),
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, 'Salvar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
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
                el('button', { class: 'btn-secondary', onclick: () => uploadSongPhoto(s) }, 'Enviar foto'),
                el('button', { class: 'btn-secondary', onclick: () => removeSongPhoto(s) }, 'Remover foto'),
              ),
            ),
          )
        }
        if (songsBox.children.length === 0) songsBox.append(el('p', { class: 'modal-empty' }, 'Nenhuma música. Envie músicas na aba "Músicas".'))
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
          karaokesBox.append(el('p', { class: 'modal-empty' }, 'Nenhum karaokê disponível.'))
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
    if (karaokesBox.children.length === 0) karaokesBox.append(el('p', { class: 'modal-empty' }, 'Nenhum karaokê. Envie vídeos na aba "Karaokês".'))
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
        alert('Foto atualizada.')
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
      alert('Foto removida.')
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
    if (!window.confirm('Remover a foto desta categoria?')) return
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
  if (!window.confirm(`Excluir a categoria “${c.name}”? Os clientes perdem o acesso imediatamente.`)) return
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
      el('button', { class: 'btn-accent', onclick: uploadForm }, 'Enviar música'),
      el('button', { class: 'btn-secondary', onclick: uploadFolderForm }, 'Enviar pasta'),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (songs.length === 0) {
    table.append(el('p', { class: 'empty-state' }, 'Nenhuma música no sistema ainda. Use "Enviar música".'))
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
            el('p', { class: 'admin-row-sub' }, [s.artist || 'Desconhecido', s.format, fmtDur(s.duration)].filter(Boolean).join(' • ')),
            el('div', { class: 'admin-chips' }, ...chips),
          ),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => uploadSongPhoto(s) }, 'Enviar foto'),
          el('button', { class: 'btn-secondary', onclick: () => removeSongPhoto(s) }, 'Remover foto'),
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
    el('p', { class: 'upload-dropzone-title' }, 'Arraste o arquivo de áudio ou clique para escolher'),
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

  const titleInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Ex.: Louvor da Manhã', autocomplete: 'off' })
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Ex.: Ministério Coral', autocomplete: 'off' })
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Sem categoria'),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, 'Adicionar foto da música'),
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
      el('h3', {}, 'Enviar música'),
      el('div', { class: 'modal-section-label' }, 'Arquivo de áudio'),
      dropzone,
      fileInput,
      el('div', { class: 'upload-grid' },
        field('Título (opcional)', titleInput),
        field('Artista (opcional)', artistInput),
      ),
      field('Categoria', catSelect),
      el('div', { class: 'modal-section-label' }, 'Foto da música (opcional)'),
      photoDrop,
      photoInput,
      el('p', { class: 'upload-info' }, 'Se o arquivo tiver capa embutida, ela é usada automaticamente. Uma foto enviada aqui substitui a embutida.'),
      statusEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: submit }, 'Enviar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function submit() {
    const file = fileInput.files[0]
    if (!file) {
      statusEl.textContent = 'Selecione um arquivo de áudio.'
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
    btn.textContent = 'Enviando…'
    statusEl.textContent = 'Enviando e indexando…'
    statusEl.classList.remove('login-error')
    statusEl.classList.add('upload-info')
    try {
      await endpoints.admin.uploadSong(fd)
      overlay.remove()
      alert('Música enviada com sucesso.')
      refreshApp()
    } catch (err) {
      statusEl.textContent = err.message
      statusEl.classList.remove('upload-info')
      statusEl.classList.add('login-error')
      btn.disabled = false
      btn.textContent = 'Enviar'
    }
  }
}

// Audio extensions accepted by the folder batch (same list as the scanner).
const FOLDER_AUDIO_EXTS = ['.mp3', '.flac', '.m4a', '.aac', '.ogg', '.opus', '.wav', '.wma', '.aiff', '.aif', '.wv', '.tak', '.ape']

// Video extensions accepted by the karaoke folder batch (same list as the
// upload endpoint's videoMimeByExt, plus .m4v which the single upload's
// accept attribute omits).
const FOLDER_VIDEO_EXTS = ['.mp4', '.m4v', '.webm', '.mkv']

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
    el('p', { class: 'upload-dropzone-title' }, 'Selecione a pasta de músicas ou arraste aqui'),
    el('p', { class: 'upload-dropzone-hint' }, 'Todas as músicas (mp3, flac, m4a, ogg, wav…) serão enviadas em sequência'),
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
      dropzone.querySelector('.upload-dropzone-title').textContent = 'Nenhum arquivo de áudio encontrado'
      dropzone.querySelector('.upload-dropzone-hint').textContent = 'Escolha outra pasta'
    } else {
      dropzone.querySelector('.upload-dropzone-title').textContent = `${selected.length} ${selected.length === 1 ? 'música' : 'músicas'} na pasta`
      dropzone.querySelector('.upload-dropzone-hint').textContent = `Total de ${fmtMB(total)}`
    }
    updateStartBtn()
  }

  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Selecione a categoria…'),
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
      el('h3', {}, 'Enviar pasta'),
      dropzone,
      fileInput,
      el('div', { class: 'modal-section-label' }, 'Categoria das músicas'),
      catSelect,
      el('p', { class: 'upload-info' }, 'Todas as músicas da pasta entram nesta categoria. Título, artista e capa embutida vêm das tags do arquivo.'),
      statusEl,
      progressWrap,
      failsEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', id: 'folder-upload-start', onclick: start }, 'Enviar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
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
      statusEl.textContent = 'Selecione uma pasta com músicas.'
      statusEl.classList.add('login-error')
      return
    }
    if (!catSelect.value) {
      statusEl.textContent = 'Escolha uma categoria para as músicas.'
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
    cancelBtn.textContent = 'Aguarde…'

    let ok = 0
    const fails = []
    for (let i = 0; i < selected.length; i++) {
      const f = selected[i]
      textEl.textContent = `Enviando ${i + 1} de ${selected.length}: ${f.name} (${fmtMB(f.size)})`
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
    textEl.textContent = `Concluído: ${ok} ${ok === 1 ? 'música enviada' : 'músicas enviadas'}${fails.length ? `, ${fails.length} ${fails.length === 1 ? 'falha' : 'falhas'}` : ''}.`
    // Replace the start button instead of reassigning onclick: el() binds the
    // original 'click'→start listener via addEventListener, so a plain
    // `startBtn.onclick = ...` would leave BOTH handlers — clicking "Fechar"
    // would re-run the whole folder upload.
    startBtn.replaceWith(el('button', { class: 'btn-accent', onclick: () => overlay.remove() }, 'Fechar'))
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
      alert('Foto atualizada.')
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
    alert('Foto removida.')
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
      el('button', { class: 'btn-accent', onclick: uploadKaraokeForm }, 'Enviar vídeo'),
      el('button', { class: 'btn-secondary', onclick: uploadKaraokeFolderForm }, 'Enviar pasta'),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (list.length === 0) {
    table.append(el('p', { class: 'empty-state' }, 'Nenhum karaokê no sistema ainda. Use "Enviar vídeo" ou "Enviar pasta".'))
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
            el('p', { class: 'admin-row-sub' }, [k.artist || 'Desconhecido', k.format, fmtDur(k.duration)].filter(Boolean).join(' • ')),
            el('div', { class: 'admin-chips' }, ...chips),
          ),
        ),
        el('div', { class: 'admin-row-actions' },
          el('button', { class: 'btn-secondary', onclick: () => uploadKaraokePhoto(k) }, 'Enviar foto'),
          el('button', { class: 'btn-secondary', onclick: () => removeKaraokePhoto(k) }, 'Remover foto'),
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
    el('p', { class: 'upload-dropzone-title' }, 'Arraste o vídeo ou clique para escolher'),
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

  const titleInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Título (opcional)', autocomplete: 'off' })
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Artista (opcional)', autocomplete: 'off' })
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Sem categoria'),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    el('span', {}, 'Adicionar foto do vídeo (opcional)'),
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
      el('h3', {}, 'Enviar vídeo de karaokê'),
      el('div', { class: 'modal-section-label' }, 'Arquivo de vídeo'),
      dropzone,
      fileInput,
      el('div', { class: 'upload-grid' },
        field('Título (opcional)', titleInput),
        field('Artista (opcional)', artistInput),
      ),
      field('Categoria', catSelect),
      el('div', { class: 'modal-section-label' }, 'Foto do vídeo (opcional)'),
      photoDrop,
      photoInput,
      el('p', { class: 'upload-info' }, 'Uma miniatura é gerada automaticamente do vídeo. Uma foto enviada aqui substitui a miniatura.'),
      statusEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: submit }, 'Enviar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function submit() {
    const file = fileInput.files[0]
    if (!file) {
      statusEl.textContent = 'Selecione um arquivo de vídeo.'
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
    btn.textContent = 'Enviando…'
    statusEl.textContent = 'Enviando e indexando…'
    statusEl.classList.remove('login-error')
    statusEl.classList.add('upload-info')
    try {
      await endpoints.admin.uploadKaraoke(fd)
      overlay.remove()
      alert('Vídeo enviado com sucesso.')
      refreshApp()
    } catch (err) {
      statusEl.textContent = err.message
      statusEl.classList.remove('upload-info')
      statusEl.classList.add('login-error')
      btn.disabled = false
      btn.textContent = 'Enviar'
    }
  }
}

function uploadKaraokeFolderForm() {
  // Same webkitdirectory pattern as the song folder upload: pick a whole
  // folder, keep only the video files, upload each one sequentially through
  // the single-video endpoint (per-file ffprobe validation + thumbnail).
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', webkitdirectory: true, multiple: true })
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#128193;' }),
    el('p', { class: 'upload-dropzone-title' }, 'Selecione a pasta de vídeos ou arraste aqui'),
    el('p', { class: 'upload-dropzone-hint' }, 'Todos os vídeos (mp4, webm, mkv…) serão enviados em sequência'),
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
    selected = Array.from(fileInput.files || []).filter((f) => FOLDER_VIDEO_EXTS.includes(folderAudioExt(f.name)))
    const total = selected.reduce((acc, f) => acc + f.size, 0)
    dropzone.classList.toggle('has-file', selected.length > 0)
    if (selected.length === 0) {
      dropzone.querySelector('.upload-dropzone-title').textContent = 'Nenhum vídeo encontrado'
      dropzone.querySelector('.upload-dropzone-hint').textContent = 'Escolha outra pasta'
    } else {
      dropzone.querySelector('.upload-dropzone-title').textContent = `${selected.length} ${selected.length === 1 ? 'vídeo' : 'vídeos'} na pasta`
      dropzone.querySelector('.upload-dropzone-hint').textContent = `Total de ${fmtMB(total)}`
    }
    updateStartBtn()
  }

  // Category is required (every karaoke in the folder gets assigned to it).
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Selecione a categoria…'),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  catSelect.addEventListener('change', updateStartBtn)

  // Optional overrides applied to every video in the folder: artist applies
  // as-is; title is used as a PREFIX before the file name (without extension).
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Artista (opcional, vale para todos)', autocomplete: 'off' })
  const titlePrefixInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Prefixo do título (opcional, ex.: "Harpa - ")', autocomplete: 'off' })
  const field = (label, control) =>
    el('label', { class: 'upload-field' }, el('span', { class: 'upload-label' }, label), control)

  const statusEl = el('p', { class: 'login-error' })
  const progressWrap = el('div', { class: 'upload-progress', style: 'display:none' },
    el('div', { class: 'upload-progress-bar' }, el('div', { class: 'upload-progress-fill', style: 'width:0%' })),
    el('p', { class: 'upload-progress-text' }, ''),
  )
  const failsEl = el('div', { class: 'upload-fails', style: 'display:none' })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal modal-upload' },
      el('h3', {}, 'Enviar pasta de karaokês'),
      dropzone,
      fileInput,
      el('div', { class: 'modal-section-label' }, 'Categoria dos vídeos'),
      catSelect,
      el('div', { class: 'upload-grid' },
        field('Artista (opcional)', artistInput),
        field('Prefixo do título (opcional)', titlePrefixInput),
      ),
      el('p', { class: 'upload-info' }, 'Todos os vídeos da pasta entram na categoria escolhida. O título vem do nome do arquivo — com o prefixo digitado, vira "prefixo + nome". O artista se aplica a todos.'),
      statusEl,
      progressWrap,
      failsEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', id: 'folder-upload-start', onclick: start }, 'Enviar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
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
      statusEl.textContent = 'Selecione uma pasta com vídeos.'
      statusEl.classList.add('login-error')
      return
    }
    if (!catSelect.value) {
      statusEl.textContent = 'Escolha uma categoria para os vídeos.'
      statusEl.classList.add('login-error')
      return
    }
    const artist = artistInput.value.trim()
    const prefix = titlePrefixInput.value.trim()
    statusEl.textContent = ''
    statusEl.classList.remove('login-error', 'upload-info')
    progressWrap.style.display = ''
    failsEl.style.display = 'none'
    failsEl.innerHTML = ''
    startBtn.disabled = true
    const cancelBtn = overlay.querySelector('.btn-secondary')
    cancelBtn.disabled = true
    cancelBtn.textContent = 'Aguarde…'

    let ok = 0
    const fails = []
    for (let i = 0; i < selected.length; i++) {
      const f = selected[i]
      textEl.textContent = `Enviando ${i + 1} de ${selected.length}: ${f.name} (${fmtMB(f.size)})`
      fillEl.style.width = `${Math.round((i / selected.length) * 100)}%`
      const fd = new FormData()
      fd.append('video', f)
      fd.append('categoryId', catSelect.value)
      if (prefix) {
        const base = f.name.slice(0, f.name.lastIndexOf('.')) || f.name
        fd.append('title', prefix + base)
      }
      if (artist) fd.append('artist', artist)
      try {
        await endpoints.admin.uploadKaraoke(fd)
        ok++
      } catch (err) {
        fails.push({ name: f.name, msg: err.message })
        failsEl.style.display = ''
        const row = el('p', { class: 'upload-fail' }, '✕ ', el('strong', {}, f.name), ` — ${err.message}`)
        failsEl.append(row)
      }
    }
    fillEl.style.width = '100%'
    textEl.textContent = `Concluído: ${ok} ${ok === 1 ? 'vídeo enviado' : 'vídeos enviados'}${fails.length ? `, ${fails.length} ${fails.length === 1 ? 'falha' : 'falhas'}` : ''}.`
    // Replace the start button instead of reassigning onclick: el() binds the
    // original 'click'→start listener via addEventListener, so a plain
    // `startBtn.onclick = ...` would leave BOTH handlers — clicking "Fechar"
    // would re-run the whole folder upload.
    startBtn.replaceWith(el('button', { class: 'btn-accent', onclick: () => overlay.remove() }, 'Fechar'))
    cancelBtn.style.display = 'none'
    refreshApp()
  }
}

function uploadKaraokePhoto(k) {
  const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
  input.addEventListener('change', async () => {
    const file = input.files[0]
    if (!file) return
    try {
      await endpoints.admin.uploadKaraokePhoto(k.id, file)
      alert('Foto atualizada.')
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
    alert('Foto removida (volta à miniatura automática).')
  } catch (err) {
    alert(err.message)
  }
}
