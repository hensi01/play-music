import { api, endpoints, artworkUrl, applyPhoneMask, tokenUserId, RERENDER_EVENT } from './api.js'
import { el } from './dom.js'
import { fmtDuration, fmtMB, pluralize } from './format.js'

let pendingAPIKey = null
let apiKeyCreating = false
let apiKeyMessage = ''
let apiKeySession = 0
export function clearPendingAPIKey() { pendingAPIKey = null; apiKeyCreating = false; apiKeyMessage = ''; apiKeySession++ }

let adminTab = 'users'
let adminState = { categories: [], songs: [], songCategoryIds: {}, categoryNames: {} }
let adminRefreshSeq = 0
let adSaving = false
let adSavePending = null

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
    el('button', { class: `tab-btn ${adminTab === 'notifications' ? 'active' : ''}`, onclick: () => { adminTab = 'notifications'; refresh() } }, 'Notificações'),
    el('button', { class: `tab-btn ${adminTab === 'ads' ? 'active' : ''}`, onclick: () => { adminTab = 'ads'; refresh() } }, 'Anúncios'),
    el('button', { class: `tab-btn ${adminTab === 'api-keys' ? 'active' : ''}`, onclick: () => { adminTab = 'api-keys'; refresh() } }, 'Chaves API'),
  )
  page.append(el('h1', { class: 'page-title' }, 'Administração'), tabs)

  const wrap = document.createElement('div')
  page.append(wrap)

  async function refresh() {
    const names = ['users', 'categories', 'songs', 'notifications', 'ads', 'api-keys']
    tabs.querySelectorAll('button').forEach((tab, index) => {
      const selected = names[index] === adminTab
      tab.classList.toggle('active', selected)
      tab.setAttribute('aria-pressed', String(selected))
    })
    const seq = ++adminRefreshSeq
    wrap.innerHTML = ''
    wrap.className = ''
    if (adminTab === 'users') await renderUsers(wrap, seq)
    else if (adminTab === 'categories') await renderCategories(wrap, seq)
    else if (adminTab === 'songs') await renderSongs(wrap, seq)
    else if (adminTab === 'notifications') renderNotifications(wrap)
    else if (adminTab === 'ads') await renderAds(wrap, seq)
    else if (adminTab === 'api-keys') renderAPIKeys(wrap)
  }

  refresh()
  return page
}

async function renderAds(wrap, seq) {
  wrap.className = 'admin-section admin-ads'
  if (adSavePending) { try { await adSavePending } catch { /* show current saved configuration */ } }
  if (!isAdminRefreshCurrent(seq)) return
  let config
  try { config = await api.get('/api/admin/ads') } catch (err) { if (isAdminRefreshCurrent(seq)) wrap.textContent = err.message; return }
  if (!isAdminRefreshCurrent(seq)) return
  let editing = -1
  const status = el('p', { role: 'status' })
  const frequency = el('input', { class: 'form-input', type: 'number', min: '1', max: '100', value: String(config.frequency), 'aria-label': 'Músicas entre anúncios' })
  const name = el('input', { class: 'form-input', placeholder: 'Nome do anúncio', maxlength: '120', 'aria-label': 'Nome do anúncio' })
  const embed = el('textarea', { class: 'form-input', placeholder: 'Cole o código iframe ou o endereço do anúncio', 'aria-label': 'Código iframe ou endereço' })
  const duration = el('input', { class: 'form-input', type: 'number', min: '1', max: '3600', value: '30', 'aria-label': 'Duração em segundos' })
  const active = el('input', { type: 'checkbox' }); active.checked = true
  const list = el('div', { class: 'admin-table' })
  const save = async () => {
    if (adSaving) throw new Error('Aguarde a gravação anterior terminar.')
    adSaving = true
    const controls = [...wrap.querySelectorAll('button, input, textarea')]
    controls.forEach((control) => { control.disabled = true })
    try { config.frequency = Number(frequency.value); adSavePending = api.put('/api/admin/ads', config); await adSavePending }
    finally { adSavePending = null; adSaving = false; controls.forEach((control) => { control.disabled = false }) }
  }
  const reset = () => { editing = -1; name.value = ''; embed.value = ''; duration.value = '30'; active.checked = true; submit.textContent = 'Adicionar anúncio' }
  const draw = () => {
    list.replaceChildren()
    config.ads.forEach((ad, index) => {
      const edit = el('button', { class: 'btn-secondary', onclick: () => { editing = index; name.value = ad.name; embed.value = ad.embedUrl; duration.value = String(ad.duration); active.checked = ad.active; submit.textContent = 'Salvar anúncio' } }, 'Editar')
      const toggle = el('button', { class: 'btn-secondary', onclick: async () => { ad.active = !ad.active; try { await save(); draw(); status.textContent = 'Anúncio atualizado.' } catch (err) { ad.active = !ad.active; status.textContent = err.message } } }, ad.active ? 'Desativar' : 'Ativar')
      list.append(el('div', { class: 'admin-row' }, el('div', { class: 'admin-row-main' }, el('p', { class: 'admin-row-title' }, ad.name), el('p', { class: 'admin-row-sub' }, `${ad.duration} s · ${ad.active ? 'Ativo' : 'Inativo'}`)), el('div', { class: 'admin-row-actions' }, edit, toggle)))
    })
  }
  const submit = el('button', { class: 'btn-primary', onclick: async () => {
    let value = embed.value.trim()
    if (value.startsWith('<')) { const doc = new DOMParser().parseFromString(value, 'text/html'); value = doc.querySelector('iframe')?.getAttribute('src') || '' }
    const entry = { name: name.value.trim(), embedUrl: value, duration: Number(duration.value), active: active.checked }
    const previous = config.ads.slice()
    if (editing < 0) config.ads.push(entry); else config.ads[editing] = entry
    try { await save(); reset(); draw(); status.textContent = 'Anúncio salvo.' } catch (err) { config.ads = previous; status.textContent = err.message }
  } }, 'Adicionar anúncio')
  const saveFrequency = el('button', { class: 'btn-secondary', onclick: async () => { try { await save(); status.textContent = 'Frequência salva.' } catch (err) { status.textContent = err.message } } }, 'Salvar frequência')
  wrap.append(el('h2', {}, 'Anúncios entre músicas'), el('p', {}, 'Quantidade de músicas entre anúncios:'), frequency, saveFrequency, list, el('h3', {}, 'Adicionar ou editar anúncio'), name, embed, el('p', {}, 'Duração em segundos:'), duration, el('label', {}, active, ' Ativo'), submit, status)
  draw()
}

function renderAPIKeys(wrap) {
  wrap.className = 'admin-section admin-api-keys'
  const name = el('input', { class: 'form-input', placeholder: 'Nome da integração', maxlength: '80', required: true })
  const status = el('p', { role: 'status' }, apiKeyMessage)
  const secret = el('div', { class: 'settings-card' })
  const list = el('div', { class: 'admin-table' })
  const create = el('button', { class: 'btn-accent', type: 'submit' }, 'Criar chave')
  async function refreshKeys() {
    try {
      const keys = await api.get('/api/admin/api-keys')
      list.innerHTML = ''
      if (!keys.length) list.append(el('p', {}, 'Nenhuma chave ativa.'))
      for (const key of keys) list.append(el('div', { class: 'admin-row' },
        el('div', { class: 'admin-row-main' }, el('p', { class: 'admin-row-title' }, key.name), el('p', { class: 'admin-row-sub' }, 'Criada em ' + new Date(key.createdAt).toLocaleString())),
        el('button', { type: 'button', class: 'btn-secondary', onclick: async () => {
          if (!confirm('Revogar esta chave? A integração deixará de funcionar.')) return
          try { await api.del(`/api/admin/api-keys/${encodeURIComponent(key.id)}`); await refreshKeys() } catch (error) { status.textContent = error.message }
        } }, 'Revogar')))
    } catch (error) { status.textContent = error.message }
  }
  function showPendingKey() {
    secret.innerHTML = ''
    create.disabled = !!pendingAPIKey || apiKeyCreating
    if (!pendingAPIKey) { secret.hidden = true; return }
    secret.hidden = false
    const value = el('input', { class: 'form-input', readonly: true, value: pendingAPIKey, 'aria-label': 'Chave de API' })
    secret.append(el('strong', {}, 'Copie sua chave agora. Ela não será exibida após fechar esta sessão.'), value,
      el('button', { type: 'button', class: 'btn-secondary', onclick: async () => { try { await navigator.clipboard.writeText(pendingAPIKey); status.textContent = 'Chave copiada.' } catch { status.textContent = 'Selecione e copie a chave acima.' } } }, 'Copiar chave'),
      el('button', { type: 'button', class: 'btn-secondary', onclick: () => { pendingAPIKey = null; showPendingKey() } }, 'Já guardei a chave'))
  }
  const form = el('form', { onsubmit: async (event) => {
    event.preventDefault(); if (pendingAPIKey || apiKeyCreating) return
    apiKeyCreating = true; create.disabled = true; apiKeyMessage = ''; status.textContent = ''
    const session = apiKeySession
    const owner = tokenUserId()
    try {
      const result = await api.post('/api/admin/api-keys', { name: name.value })
      if (session !== apiKeySession || !owner || tokenUserId() !== owner) return
      pendingAPIKey = result.key
      showPendingKey()
      refreshApp()
      name.value = ''; await refreshKeys()
    } catch (error) { if (session === apiKeySession) { apiKeyMessage = error.message || 'Não foi possível criar a chave.'; status.textContent = apiKeyMessage } }
    finally { if (session === apiKeySession) { apiKeyCreating = false; create.disabled = !!pendingAPIKey; refreshApp() } }
  } }, el('h2', {}, 'Chaves de API'), el('p', { class: 'settings-text' }, 'Cada chave concede acesso às operações administrativas, inclusive criação e revogação de chaves. Guarde-a em um local seguro.'), name, create, status)
  showPendingKey()
  wrap.append(form, secret, el('h2', {}, 'Chaves ativas'), list, el('a', { href: '/docs/', target: '_blank', rel: 'noopener noreferrer' }, 'Abrir documentação da API'))
  void refreshKeys()
}

function renderNotifications(wrap) {
  wrap.className = 'admin-section admin-notifications'
  const title = el('input', { class: 'form-input', placeholder: 'Título', 'aria-label': 'Título da notificação', maxlength: '120', required: true })
  const body = el('textarea', { class: 'form-input', placeholder: 'Mensagem', 'aria-label': 'Mensagem da notificação', maxlength: '500', required: true })
  const link = el('input', { class: 'form-input', placeholder: 'Link HTTPS ou caminho do aplicativo (opcional)', 'aria-label': 'Link da notificação' })
  const date = el('input', { class: 'form-input', type: 'datetime-local', 'aria-label': 'Data e hora do envio' })
  const repeat = el('input', { class: 'form-input', type: 'number', min: '1', max: '8760', step: '1', placeholder: 'Repetir a cada quantas horas? (opcional)', 'aria-label': 'Intervalo de repetição em horas' })
  const status = el('p', { role: 'status' })
  const list = el('div', { class: 'admin-table' })
  const configStatus = el('p', { class: 'settings-text' }, 'Verificando serviço de notificações…')
  endpoints.pushConfig().then(config => { configStatus.textContent = config.enabled ? 'Serviço de notificações configurado.' : 'Serviço indisponível: configure o relay no servidor.' }).catch(() => { configStatus.textContent = 'Não foi possível verificar o serviço.' })
  const button = el('button', { class: 'btn-accent', type: 'submit' }, 'Programar notificação')
  async function load() {
    try {
      const campaigns = await api.get('/api/admin/notifications')
      list.innerHTML = ''
      const labels = { pending: 'Agendada', sending: 'Processando', completed: 'Enfileirada', partial: 'Parcial', unknown: 'Resultado incerto', cancelled: 'Cancelada' }
      for (const item of campaigns) {
        const row = el('div', { class: 'admin-row' }, el('div', { class: 'admin-row-main' },
          el('p', { class: 'admin-row-title' }, item.title),
          el('p', { class: 'admin-row-sub' }, `${new Date(item.scheduledAt).toLocaleString()} · ${labels[item.status] || item.status} · ${item.queued}/${item.selected} enfileiradas${item.repeatHours ? ` · Repete a cada ${item.repeatHours} h` : ''}`),
          item.detail ? el('p', { class: 'admin-row-sub' }, item.detail) : null))
        if (item.status === 'pending' || (item.status === 'sending' && item.repeatHours)) row.append(el('button', { class: 'btn-secondary', type: 'button', onclick: async () => {
          if (!confirm(item.repeatHours ? 'Interromper os próximos envios desta notificação?' : 'Cancelar esta notificação agendada?')) return
          try { await api.del(`/api/admin/notifications/${encodeURIComponent(item.id)}`); await load() } catch (err) { status.textContent = err.message }
        } }, item.repeatHours ? 'Parar repetição' : 'Cancelar'))
        list.append(row)
      }
      if (!campaigns.length) list.append(el('p', { class: 'settings-text' }, 'Nenhuma notificação programada.'))
    } catch (err) { status.textContent = err.message }
  }
  const form = el('form', { onsubmit: async (event) => {
    event.preventDefault()
    const scheduledAt = date.value ? new Date(date.value) : new Date()
    if (!title.value.trim() || !body.value.trim()) { status.textContent = 'Preencha o título e a mensagem.'; return }
    if (Number.isNaN(scheduledAt.getTime())) { status.textContent = 'Informe uma data válida.'; return }
    if (date.value && scheduledAt.getTime() < Date.now() - 60000) { status.textContent = 'Escolha uma data e hora futuras.'; return }
    const repeatHours = repeat.value === '' ? 0 : Number(repeat.value)
    if (!Number.isInteger(repeatHours) || repeatHours < 0 || repeatHours > 8760 || (repeat.value !== '' && repeatHours === 0)) { status.textContent = 'Informe um intervalo inteiro entre 1 e 8760 horas.'; return }
    if (button.disabled) return
    const confirmation = date.value ? `Programar envio para ${scheduledAt.toLocaleString()}?` : 'Enviar notificação agora?'
    if (!confirm(confirmation + (repeatHours ? ` A notificação será repetida a cada ${repeatHours} hora(s) até ser interrompida.` : ''))) return
    button.disabled = true
    try {
      await api.post('/api/admin/notifications', { title: title.value.trim(), body: body.value.trim(), url: link.value.trim(), scheduledAt: scheduledAt.toISOString(), repeatHours })
      status.textContent = 'Notificação registrada. Acompanhe o processamento no histórico; o enfileiramento não confirma a exibição no dispositivo.'
      form.reset(); await load()
    } catch (err) { status.textContent = err.message }
    finally { button.disabled = false }
  } }, el('h2', {}, 'Avisos e promoções'), el('p', { class: 'settings-text' }, 'Escolha a data e a hora local. Deixe em branco para enviar agora.'), title, body, link, date, el('label', { class: 'settings-text' }, 'Repetição em horas (opcional)', repeat), el('p', { class: 'settings-text' }, 'A repetição começa após cada envio concluído e pode ser interrompida no histórico.'), button, status)
  wrap.append(configStatus, form, el('h2', {}, 'Histórico e agendamentos'), list)
  void load()
}

function refreshApp() {
  window.dispatchEvent(new Event(RERENDER_EVENT))
}

async function renderUsers(wrap, seq) {
  const [users, categories] = await Promise.all([
    endpoints.admin.users().catch(() => []),
    endpoints.admin.categories().catch(() => []),
  ])
  if (!isAdminRefreshCurrent(seq)) return
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
    table.append(
      el(
        'div',
        { class: 'admin-row' },
        el('div', { class: 'admin-row-main' },
          el('p', { class: 'admin-row-title' }, u.name, u.isAdmin ? el('span', { class: 'badge' }, 'Admin') : null),
          el('p', { class: 'admin-row-sub' }, u.isAdmin ? `@${u.username}` : (u.phone || '')),
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

  const adminBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': isAdmin ? 'true' : 'false', onclick: () => { isAdmin = true; sync() } }, 'Administrador')
  const clientBtn = el('button', { class: 'login-toggle-btn', 'aria-pressed': !isAdmin ? 'true' : 'false', onclick: () => { isAdmin = false; sync() } }, 'Cliente')

  function sync() {
    adminBtn.classList.toggle('active', isAdmin)
    clientBtn.classList.toggle('active', !isAdmin)
    adminBtn.setAttribute('aria-pressed', isAdmin ? 'true' : 'false')
    clientBtn.setAttribute('aria-pressed', !isAdmin ? 'true' : 'false')
    adminFields.style.display = isAdmin ? '' : 'none'
    clientFields.style.display = isAdmin ? 'none' : ''
  }

  const overlay = mountModal('modal',
    el('h3', {}, isEdit ? 'Editar usuário' : 'Novo usuário'),
    el('div', { class: 'login-toggle' }, clientBtn, adminBtn),
    nameInput,
    clientFields,
    adminFields,
    errorEl,
    el('div', { class: 'modal-actions' },
      el('button', { class: 'btn-accent', onclick: save }, 'Salvar'),
      el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
    ),
  )
  sync()

  async function save() {
    errorEl.textContent = ''
    const payload = { name: nameInput.value.trim(), isAdmin }
    if (isAdmin) {
      // The backend clears the phone when a user becomes an admin, so only the
      // client branch sends it.
      payload.username = usernameInput.value.trim()
      payload.email = emailInput.value.trim()
      if (passInput.value) payload.password = passInput.value
    } else {
      payload.phone = phoneInput.value.trim()
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

function newCategoryForm() {
  const nameInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Nome da categoria', autofocus: true })
  const errorEl = el('p', { class: 'login-error' })

  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoLabel = el('span', {}, 'Foto da categoria (opcional)')
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    photoLabel,
  )
  wireDropzone(photoDrop, photoInput, () => {
    const f = photoInput.files[0]
    if (f) photoLabel.textContent = f.name
  })

  const saveBtn = el('button', { class: 'btn-accent', onclick: save }, 'Criar')
  const overlay = mountModal('modal',
    el('h3', {}, 'Nova categoria'),
    nameInput,
    el('div', { class: 'modal-section-label' }, 'Foto da categoria (opcional)'),
    photoDrop,
    photoInput,
    errorEl,
    el('div', { class: 'modal-actions' },
      saveBtn,
      el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
    ),
  )

  async function save() {
    errorEl.textContent = ''
    const name = nameInput.value.trim()
    if (!name) { errorEl.textContent = 'Informe o nome da categoria.'; return }
    saveBtn.disabled = true
    saveBtn.textContent = 'Salvando…'
    try {
      const cat = await endpoints.admin.createCategory(name)
      // The photo upload needs the id the create call returns.
      if (photoInput.files[0]) {
        await endpoints.admin.uploadCategoryPhoto(cat.id, photoInput.files[0])
      }
      overlay.remove()
      refreshApp()
    } catch (err) {
      errorEl.textContent = err.message
      saveBtn.disabled = false
      saveBtn.textContent = 'Criar'
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
            pluralize(c.songCount ?? 0, 'música', 'músicas'),
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
  const nameInput = el('input', { class: 'form-input', type: 'text', value: cat.name, placeholder: 'Nome da categoria' })
  const photoPreview = el('img', { class: 'track-art', src: artworkUrl(cat.id, 96), alt: '', style: 'width:56px;height:56px;border-radius:8px;display:block;object-fit:cover' })
  const filterInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Filtrar músicas…' })
  const songsBox = el('div', { class: 'modal-scroll' })
  const errEl = el('p', { class: 'login-error' })
  const saveBtn = el('button', { class: 'btn-accent', onclick: save }, 'Salvar')
  const overlay = mountModal('modal modal-wide',
    el('h3', {}, `Categoria: ${cat.name}`),
    nameInput,
    el('div', { class: 'modal-section-label' }, 'Foto da categoria'),
    el('div', { style: 'display:flex;align-items:center;gap:12px' },
      photoPreview,
      el('div', { style: 'display:flex;flex-direction:column;gap:6px' },
        el('button', { class: 'btn-secondary', onclick: () => uploadCatPhoto() }, 'Enviar foto'),
        el('button', { class: 'btn-secondary', onclick: () => removeCatPhoto() }, 'Remover foto'),
      ),
    ),
    el('div', { class: 'modal-section-label' }, 'Músicas'),
    filterInput,
    songsBox,
    errEl,
    el('div', { class: 'modal-actions' },
      saveBtn,
      el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
    ),
  )

  let assignedSongIds = []

  endpoints.admin.category(cat.id).then((d) => {
    assignedSongIds = d.songIds ?? []
    buildList('')
  }).catch(() => { buildList('') })

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
          box.checked = assignedSongIds.includes(s.id)
          box.addEventListener('change', () => {
            if (box.checked) assignedSongIds.push(s.id)
            else assignedSongIds = assignedSongIds.filter((x) => x !== s.id)
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

  filterInput.addEventListener('input', (e) => buildList(e.target.value))

  function refreshCatPhotoPreview() {
    photoPreview.src = artworkUrl(cat.id, 96) + '&t=' + Date.now()
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
    errEl.textContent = ''
    const name = nameInput.value.trim()
    saveBtn.disabled = true
    try {
      await endpoints.admin.updateCategory(cat.id, { name, songIds: assignedSongIds })
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
            el('p', { class: 'admin-row-sub' }, [s.artist || 'Desconhecido', s.format, fmtDuration(s.duration, { unknown: '' })].filter(Boolean).join(' • ')),
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

// mountModal builds a modal shell (backdrop plus centred panel) from its innards
// and attaches it. Clicking the backdrop dismisses it; the panel must stop the
// click only if it needs to, which none do. Returns the overlay so callers can
// remove it.
function mountModal(panelClass, ...children) {
  const overlay = el('div', { class: 'modal-overlay' }, el('div', { class: panelClass }, ...children))
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.body.append(overlay)
  return overlay
}

// wireDropzone makes a dropzone behave like its hidden file input: click opens
// the picker, a dropped file fills the input, and either path calls onChange.
function wireDropzone(dropzone, fileInput, onChange) {
  dropzone.addEventListener('click', () => fileInput.click())
  dropzone.addEventListener('dragover', (e) => { e.preventDefault(); dropzone.classList.add('drag') })
  dropzone.addEventListener('dragleave', () => dropzone.classList.remove('drag'))
  dropzone.addEventListener('drop', (e) => {
    e.preventDefault()
    dropzone.classList.remove('drag')
    if (e.dataTransfer.files.length) fileInput.files = e.dataTransfer.files
    onChange()
  })
  fileInput.addEventListener('change', onChange)
}

function uploadForm() {
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'audio/*' })
  const dropTitle = el('p', { class: 'upload-dropzone-title' }, 'Arraste o arquivo de áudio ou clique para escolher')
  const dropHint = el('p', { class: 'upload-dropzone-hint' }, 'mp3, m4a, flac, ogg, wav…')
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#9835;' }),
    dropTitle,
    dropHint,
  )
  function updateFile() {
    const f = fileInput.files[0]
    if (!f) return
    dropzone.classList.add('has-file')
    dropTitle.textContent = f.name
    dropHint.textContent = fmtMB(f.size)
  }
  wireDropzone(dropzone, fileInput, updateFile)

  const titleInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Ex.: Louvor da Manhã', autocomplete: 'off' })
  const artistInput = el('input', { class: 'form-input', type: 'text', placeholder: 'Ex.: Ministério Coral', autocomplete: 'off' })
  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Sem categoria'),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  const photoInput = el('input', { class: 'upload-file-input', type: 'file', accept: 'image/*' })
  const photoLabel = el('span', {}, 'Adicionar foto da música')
  const photoDrop = el(
    'div',
    { class: 'upload-photo-drop' },
    el('span', { html: '&#128247;' }),
    photoLabel,
  )
  wireDropzone(photoDrop, photoInput, () => {
    const f = photoInput.files[0]
    if (f) photoLabel.textContent = f.name
  })
  const statusEl = el('p', { class: 'login-error' })

  const field = (label, control) =>
    el('label', { class: 'upload-field' }, el('span', { class: 'upload-label' }, label), control)

  const submitBtn = el('button', { class: 'btn-accent', onclick: submit }, 'Enviar')
  const overlay = mountModal('modal modal-upload',
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
      submitBtn,
      el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar'),
    ),
  )

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

    submitBtn.disabled = true
    submitBtn.textContent = 'Enviando…'
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
      submitBtn.disabled = false
      submitBtn.textContent = 'Enviar'
    }
  }
}

// The accepted audio extensions come from /api/settings, which the server
// derives from the one format table the scanner and streamer also use.
let audioExtensions = null
async function isAudioFile(name) {
  const i = name.lastIndexOf('.')
  if (i < 0) return false
  if (audioExtensions === null) {
    try { audioExtensions = (await endpoints.settings()).audioExtensions ?? [] } catch { audioExtensions = [] }
  }
  return audioExtensions.includes(name.slice(i + 1).toLowerCase())
}

function uploadFolderForm() {
  // webkitdirectory lets the user pick a whole folder; each File keeps its
  // relative path but only the audio files matter here.
  const fileInput = el('input', { class: 'upload-file-input', type: 'file', webkitdirectory: true, multiple: true })
  const dropTitle = el('p', { class: 'upload-dropzone-title' }, 'Selecione a pasta de músicas ou arraste aqui')
  const dropHint = el('p', { class: 'upload-dropzone-hint' }, 'Todas as músicas da pasta serão enviadas em sequência')
  const dropzone = el(
    'div',
    { class: 'upload-dropzone' },
    el('span', { class: 'upload-dropzone-icon', html: '&#128193;' }),
    dropTitle,
    dropHint,
  )
  let selected = []
  async function updateFiles() {
    const candidates = Array.from(fileInput.files || [])
    const keep = await Promise.all(candidates.map((f) => isAudioFile(f.name)))
    selected = candidates.filter((_, i) => keep[i])
    const total = selected.reduce((acc, f) => acc + f.size, 0)
    dropzone.classList.toggle('has-file', selected.length > 0)
    if (selected.length === 0) {
      dropTitle.textContent = 'Nenhum arquivo de áudio encontrado'
      dropHint.textContent = 'Escolha outra pasta'
    } else {
      dropTitle.textContent = `${pluralize(selected.length, 'música', 'músicas')} na pasta`
      dropHint.textContent = `Total de ${fmtMB(total)}`
    }
    updateStartBtn()
  }
  wireDropzone(dropzone, fileInput, updateFiles)

  const catSelect = el('select', { class: 'form-input' },
    el('option', { value: '' }, 'Selecione a categoria…'),
    ...adminState.categories.map((c) => el('option', { value: c.id }, c.name)),
  )
  catSelect.addEventListener('change', updateStartBtn)

  const statusEl = el('p', { class: 'login-error' })
  const fillEl = el('div', { class: 'upload-progress-fill', style: 'width:0%' })
  const textEl = el('p', { class: 'upload-progress-text' })
  const progressWrap = el('div', { class: 'upload-progress', style: 'display:none' },
    el('div', { class: 'upload-progress-bar' }, fillEl),
    textEl,
  )
  const failsEl = el('div', { class: 'upload-fails', style: 'display:none' })

  const startBtn = el('button', { class: 'btn-accent', onclick: start }, 'Enviar')
  const cancelBtn = el('button', { class: 'btn-secondary', onclick: () => overlay.remove() }, 'Cancelar')
  const overlay = mountModal('modal modal-upload',
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
      startBtn,
      cancelBtn,
    ),
  )

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
    textEl.textContent = `Concluído: ${pluralize(ok, 'música enviada', 'músicas enviadas')}${fails.length ? `, ${pluralize(fails.length, 'falha', 'falhas')}` : ''}.`
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
