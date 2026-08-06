// Admin page: users and categories management (admin only).

import { endpoints, artworkUrl, applyPhoneMask } from './api.js'

let adminTab = 'users'
let adminState = { users: [], categories: [], albums: [], artists: [] }

export function renderAdmin() {
  const page = document.createElement('div')
  page.className = 'page-padding'

  const tabs = el(
    'div',
    { class: 'tabs' },
    el('button', { class: `tab-btn ${adminTab === 'users' ? 'active' : ''}`, onclick: () => { adminTab = 'users'; refresh() } }, 'Usuários'),
    el('button', { class: `tab-btn ${adminTab === 'categories' ? 'active' : ''}`, onclick: () => { adminTab = 'categories'; refresh() } }, 'Categorias'),
  )
  page.append(el('h1', { class: 'page-title' }, 'Administração'), tabs)

  const wrap = document.createElement('div')
  page.append(wrap)

  async function refresh() {
    wrap.innerHTML = ''
    if (adminTab === 'users') await renderUsers(wrap)
    else await renderCategories(wrap)
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
      render()
    } catch (err) {
      errorEl.textContent = err.message
    }
  }
}

async function deleteUser(u) {
  if (!window.confirm(`Excluir o usuário “${u.name}”?`)) return
  try {
    await endpoints.admin.deleteUser(u.id)
    render()
  } catch (err) {
    alert(err.message)
  }
}

// ---------- Categories ----------

async function renderCategories(wrap) {
  const categories = await endpoints.admin.categories().catch(() => [])
  adminState.categories = categories

  wrap.append(
    el('div', { class: 'admin-toolbar' },
      el('button', { class: 'btn-accent', onclick: async () => {
        const name = window.prompt('Nome da nova categoria:')
        if (!name?.trim()) return
        try {
          await endpoints.admin.createCategory(name.trim())
          render()
        } catch (err) { alert(err.message) }
      } }, 'Nova categoria'),
    ),
  )

  const table = el('div', { class: 'admin-table' })
  if (categories.length === 0) table.append(el('p', { class: 'empty-state' }, 'Nenhuma categoria ainda.'))
  for (const c of categories) {
    table.append(
      el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main', style: 'cursor:pointer' },
          el('p', { class: 'admin-row-title', onclick: () => categoryForm(c) }, c.name),
          el('p', { class: 'admin-row-sub' }, `${plural(c.albumCount, 'álbum', 'álbuns')} • ${plural(c.artistCount, 'artista', 'artistas')}`),
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
      el('div', { class: 'modal-section-label' }, 'Álbuns'),
      el('input', { class: 'form-input', id: 'cat-album-filter', type: 'text', placeholder: 'Filtrar álbuns…' }),
      el('div', { class: 'modal-scroll', id: 'cat-albums' }),
      el('div', { class: 'modal-section-label' }, 'Artistas'),
      el('div', { class: 'modal-scroll', id: 'cat-artists' }),
      el('p', { class: 'login-error', id: 'cat-error' }),
      el('div', { class: 'modal-actions' },
        el('button', { class: 'btn-accent', onclick: save }, 'Salvar'),
        el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
      ),
    ),
  )
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)

  const albumsBox = overlay.querySelector('#cat-albums')
  const artistsBox = overlay.querySelector('#cat-artists')
  let assigned = { albumIds: [], artistIds: [] }
  let photoBusy = new Set()

  endpoints.admin.category(cat.id).then((d) => {
    assigned = { albumIds: d.albumIds ?? [], artistIds: d.artistIds ?? [] }
    buildLists('')
  }).catch(() => buildLists(''))

  function buildLists(filter) {
    albumsBox.innerHTML = ''
    artistsBox.innerHTML = ''
    const needAlbums = adminState.albums.length === 0
    const needArtists = adminState.artists.length === 0
    Promise.all([
      needAlbums ? endpoints.admin.albums() : Promise.resolve(adminState.albums),
      needArtists ? endpoints.admin.artists() : Promise.resolve(adminState.artists),
    ]).then(([albums, artists]) => {
      adminState.albums = albums
      adminState.artists = artists
      const f = filter.toLowerCase()
      for (const a of albums) {
        if (f && !a.name.toLowerCase().includes(f)) continue
        const box = el('input', { type: 'checkbox', id: `alb-${a.id}` })
        box.checked = assigned.albumIds.includes(a.id)
        box.addEventListener('change', () => {
          if (box.checked) assigned.albumIds.push(a.id)
          else assigned.albumIds = assigned.albumIds.filter((x) => x !== a.id)
        })
        albumsBox.append(
          el('div', { class: 'admin-row' },
            el('div', { class: 'admin-row-main', style: 'display:flex;align-items:center;gap:8px' },
              el('img', { class: 'track-art', src: artworkUrl(a.id, 48), alt: '' }),
              box,
              el('span', {}, `${a.name} — ${a.artist}`),
            ),
            el('div', { class: 'admin-row-actions' },
              el('button', { class: 'btn-secondary', onclick: () => uploadPhoto(a) }, 'Enviar foto'),
              el('button', { class: 'btn-secondary', onclick: () => removePhoto(a) }, 'Remover foto'),
            ),
          ),
        )
      }
      if (albumsBox.children.length === 0) albumsBox.append(el('p', { class: 'modal-empty' }, 'Nenhum álbum.'))
      for (const ar of artists) {
        const box = el('input', { type: 'checkbox', id: `art-${ar.id}` })
        box.checked = assigned.artistIds.includes(ar.id)
        box.addEventListener('change', () => {
          if (box.checked) assigned.artistIds.push(ar.id)
          else assigned.artistIds = assigned.artistIds.filter((x) => x !== ar.id)
        })
        artistsBox.append(
          el('label', { class: 'modal-check', style: 'display:flex;align-items:center;gap:8px' },
            box, el('span', {}, `${ar.name} (${plural(ar.albumCount, 'álbum', 'álbuns')})`)),
        )
      }
      if (artistsBox.children.length === 0) artistsBox.append(el('p', { class: 'modal-empty' }, 'Nenhum artista.'))
    })
  }

  overlay.querySelector('#cat-album-filter').addEventListener('input', (e) => buildLists(e.target.value))

  function uploadPhoto(a) {
    const input = el('input', { type: 'file', accept: 'image/*', style: 'display:none' })
    input.addEventListener('change', async () => {
      const file = input.files[0]
      if (!file) return
      try {
        await endpoints.admin.uploadPhoto(a.id, file)
        alert('Foto atualizada.')
      } catch (err) {
        alert(err.message)
      }
    })
    document.body.append(input)
    input.click()
  }

  async function removePhoto(a) {
    try {
      await endpoints.admin.deletePhoto(a.id)
      alert('Foto removida (capa original restaurada).')
    } catch (err) {
      alert(err.message)
    }
  }

  async function save() {
    const errEl = overlay.querySelector('#cat-error')
    errEl.textContent = ''
    const name = overlay.querySelector('#cat-name').value.trim()
    try {
      await endpoints.admin.updateCategory(cat.id, { name, albumIds: assigned.albumIds, artistIds: assigned.artistIds })
      overlay.remove()
      render()
    } catch (err) {
      errEl.textContent = err.message
    }
  }
}

async function deleteCategory(c) {
  if (!window.confirm(`Excluir a categoria “${c.name}”? Os clientes perdem o acesso imediatamente.`)) return
  try {
    await endpoints.admin.deleteCategory(c.id)
    render()
  } catch (err) {
    alert(err.message)
  }
}
