// Admin page: users, categories (songs per category) and song uploads.

import { endpoints, artworkUrl, applyPhoneMask } from './api.js'

let adminTab = 'users'
let adminState = { users: [], categories: [], songs: [], songCategoryIds: {}, categoryNames: {} }

export function renderAdmin() {
  const page = document.createElement('div')
  page.className = 'page-padding'

  const tabs = el(
    'div',
    { class: 'tabs' },
    el('button', { class: `tab-btn ${adminTab === 'users' ? 'active' : ''}`, onclick: () => { adminTab = 'users'; refresh() } }, 'Usuários'),
    el('button', { class: `tab-btn ${adminTab === 'categories' ? 'active' : ''}`, onclick: () => { adminTab = 'categories'; refresh() } }, 'Categorias'),
    el('button', { class: `tab-btn ${adminTab === 'songs' ? 'active' : ''}`, onclick: () => { adminTab = 'songs'; refresh() } }, 'Músicas'),
  )
  page.append(el('h1', { class: 'page-title' }, 'Administração'), tabs)

  const wrap = document.createElement('div')
  page.append(wrap)

  async function refresh() {
    wrap.innerHTML = ''
    if (adminTab === 'users') await renderUsers(wrap)
    else if (adminTab === 'categories') await renderCategories(wrap)
    else await renderSongs(wrap)
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

async function renderUsers(wrap) {
  const [users, categories] = await Promise.all([
    endpoints.admin.users().catch(() => []),
    endpoints.admin.categories().catch(() => []),
  ])
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
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Nome', value: existing?.name ?? '', autocomplete: 'name' })
  const phoneInput = el('input', { class: 'form-input', type: 'tel', inputmode: 'numeric', placeholder: 'Telefone (99) 99999-9999', value: existing && !existing.isAdmin ? existing.phone : '', autocomplete: 'tel' })
  phoneInput.addEventListener('input', () => applyPhoneMask(phoneInput))
  const passInput = el('input', { class: 'form-input', type: 'password', placeholder: isEdit ? 'Nova senha (opcional)' : 'Senha', autocomplete: 'new-password' })
  const errorEl = el('p', { class: 'login-error' })

  const catBoxes = adminState.categories.map((c) => {
    const box = el('input', { type: 'checkbox', id: `cat-${c.id}` })
    if ((existing?.categories ?? []).some((cc) => cc.id === c.id)) box.checked = true
    return el('label', { class: 'modal-check', style: 'display:block' }, box, el('span', {}, c.name))
  })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal' },
      el('h3', {}, isEdit ? 'Editar usuário' : 'Novo usuário'),
      nameInput,
      existing?.isAdmin ? null : phoneInput,
      passInput,
      el('div', { class: 'modal-section-label' }, 'Categorias liberadas'),
      ...catBoxes,
      errorEl,
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, 'Salvar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  async function save() {
    errorEl.textContent = ''
    const categoryIds = [...overlay.querySelectorAll('input[type=checkbox]:checked')].map((b) => b.id.slice(4))
    const payload = { name: nameInput.value.trim(), categoryIds }
    if (!existing?.isAdmin) payload.phone = phoneInput.value.trim()
    if (passInput.value) payload.password = passInput.value
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

// newCategoryForm: modal with name + checkout link (used for creation).
function newCategoryForm() {
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Nome da categoria', autofocus: true })
  const urlInput = el('input', { class: 'form-input', type: 'url', placeholder: 'https://checkout.exemplo.com/...', autocomplete: 'off' })
  const errorEl = el('p', { class: 'login-error' })

  const overlay = el('div', { class: 'modal-overlay' },
    el('div', { class: 'modal' },
      el('h3', {}, 'Nova categoria'),
      nameInput,
      el('label', { class: 'upload-field' },
        el('span', { class: 'upload-label' }, 'Link do checkout (loja)'),
        urlInput,
      ),
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
    try {
      await endpoints.admin.createCategory(name, urlInput.value.trim())
      overlay.remove()
      refreshApp()
    } catch (err) {
      errorEl.textContent = err.message
    }
  }
}

async function renderCategories(wrap) {
  const categories = await endpoints.admin.categories().catch(() => [])
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
          el('p', { class: 'admin-row-sub' }, plural(c.songCount ?? 0, 'música', 'músicas')),
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
      el('div', { class: 'modal-section-label' }, 'Músicas'),
      el('input', { class: 'form-input', id: 'cat-song-filter', type: 'text', placeholder: 'Filtrar músicas…' }),
      el('div', { class: 'modal-scroll', id: 'cat-songs' }),
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
  let assigned = { songIds: [] }

  endpoints.admin.category(cat.id).then((d) => {
    assigned = { songIds: d.songIds ?? [] }
    buildList('')
  }).catch(() => buildList(''))

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

  overlay.querySelector('#cat-song-filter').addEventListener('input', (e) => buildList(e.target.value))

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

  async function save() {
    const errEl = overlay.querySelector('#cat-error')
    errEl.textContent = ''
    const name = overlay.querySelector('#cat-name').value.trim()
    const checkoutUrl = overlay.querySelector('#cat-checkout').value.trim()
    try {
      await endpoints.admin.updateCategory(cat.id, { name, checkoutUrl, songIds: assigned.songIds })
      overlay.remove()
      refreshApp()
    } catch (err) {
      errEl.textContent = err.message
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

async function renderSongs(wrap) {
  const data = await endpoints.admin.songs().catch(() => ({ songs: [], categoryIds: {}, categoryList: [] }))
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
    startBtn.disabled = false
    startBtn.textContent = 'Fechar'
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
