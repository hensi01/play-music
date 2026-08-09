# Review Log
## Task Reviews
## Task: M1 [G1] — Baseline (servidor UP, go build/vet/test, diagnóstico Playwright, inventário em REPORT.md)

**Veredicto: PASS**

### Verificações executadas (pelo reviewer, 2026-08-08)

| Check | Resultado |
|---|---|
| Servidor `http://localhost:4533/` | ✅ 200 OK (1539 bytes) |
| `go build ./...` | ✅ exit 0 |
| `go vet ./...` | ✅ exit 0 |
| `go test ./...` | ✅ exit 0 (phone, stream ok) |
| Inventário B1-B5 vs. código | ✅ referências precisas (detalhes abaixo) |
| Suspeitos STRATEGY vs. código | ✅ conferidos (1 desmentido, 1 baixo risco) |
| Estado (STATUS/REPORT/MEMORY) | ✅ coerentes |

### Confirmações por inspeção de código

- **B2 confirmado (real)**: `handlers_api.go:586` responde `writeJSON(201, u)` com `u.CreatedAt` nunca preenchido (o INSERT em `store/users.go:36-39` usa `now()` no SQL mas não retorna o valor). `model.go:27` tem `json:"createdAt,omitempty"`, mas `time.Time` zero nunca é omitido por `omitempty` → `"0001-01-01T00:00:00Z"` no JSON. Correção em M2: re-ler o usuário após o insert (GetUser) ou `RETURNING created_at`.
- **B3 confirmado**: nenhuma migration seeda clientes (0001-0008 não têm INSERT INTO users); só admin via bootstrap Go.
- **B5 confirmado (comportamento, não bug)**: `pwa.js:112-115` — comentário explícito "mostra sempre, em toda visita"; dismiss não persiste. Decisão de produto pendente.
- **B1 — ajuste de rótulo**: `refreshAuth` (app.js:123-134) NÃO chama `/api/me` sem token (guarda `getToken()` em :124). O 401 observado vem de token inválido/expirado no localStorage (ou de `loja.html:97`). Evidência real, mas o mecanismo no inventário ("fetch sem token") é impreciso — ver sugestão M3.

### Suspeitos conferidos no código

- ✅ **Reais**: removePlaylistTrack sem try/catch (app.js:777-780); desync toggleLike — `player.setLiked(!wasLiked)` em :298-300 roda também no catch; toggleLikeCurrent sem catch (app.js:1311-1319); teclado sem exclusão de BUTTON/A (app.js:1586 só exclui INPUT/TEXTAREA/SELECT); race no refresh admin (admin.js:24-29); aria-pressed nunca atualizado no sync() (admin.js:127-134 só mexe em classList); save() de categoria sem disable (admin.js:410-422).
- ⚠️ **Desmentido — remover da lista M3**: `player.js:221-224` — o do/while só roda com guarda `shuffle && queue.length > 1` (linha 221); com queue.length===1 o caminho é o de fim de fila (idx=1 ≥ 1). **Não há loop infinito possível.** Não "corrigir" em M3; no máximo teste de regressão.
- ⚠️ **Baixo risco — validar com teste, não mudar**: `loja.html:93-103` (arquivo é `web/assets/loja.html`, não `web/loja.html`) — `renderUser()` sempre faz `innerHTML` antes de `bindLogin()`, descartando nós antigos; duplicação de listeners é improvável.

### Problemas não bloqueantes (para M2/M3)

1. **M2 — superfície não exercitada pelo baseline**: upload de música (limite 15MB do multipart?), `handleStream` com Range (esperar 206 parcial), `handleArtwork` com `?jwt=`, contrato de `handleAdminSongs` ({songs, categoryIds, categoryList}) vs. `admin.js renderSongs`, e `handleStoreRegister` público (handlers_store.go:33-103). Adicionar ao checklist de auditoria M2.
2. **M3 — B1**: para zerar console errors, `refreshAuth` deve pular a chamada quando o JWT estiver expirado (decodificar `exp` do token client-side) — o 401 de rede sempre loga erro no console.
3. **M5 — fixtures**: B4 exige decisão de produto (cliente sem categorias ver "Nada encontrado" é intencional?) e fixture de teste (criar cliente com categorias via admin API). A overlay PWA (B5) também precisa de decisão (persistir dismiss em localStorage) ou dismiss explícito no setup dos testes.
4. **Evidência**: REPORT.md é textual; recomenda-se salvar logs de console Playwright como artefato (ex.: `teamwork-state/.../artifacts/console-*.log`) para regressão no M5.
5. **Edição admin→cliente**: o form admin.js:105 só pré-preenche phone para clientes existentes; converter admin→cliente exige digitar o telefone do zero (UX ok, mas cobrir no teste M5).

### Veredito final
M1 entregue por inteiro, evidências verificáveis e inventário de alta qualidade (referências precisas, 2 ressalvas: rótulo B1 e suspeito player.js desmentido). Estado dos arquivos coerente. Seguir para M2.

**Sugestões para M2/M3**: priorizar B2 (contrato API), B1 (rótulo + fix), race admin, aria-pressed; retirar player.js:221-224 da lista M3; incluir testes Go para B2; capturar artefatos de console no M5.

## Task: M4 [G2] — Polimento visual profissional (style.css, aditivo)

**Veredicto: PASS** (2026-08-09, reviewer)

### Verificações executadas

| Check | Resultado |
|---|---|
| Bloco "M4 — Polimento" | ✅ linhas 2246-2569, ADITIVO puro (git diff: 326 insertions, 0 deletions) |
| prefers-reduced-motion última regra | ✅ linhas 2571-2581 (única occurrence; `animation: none !important` mata modal-in e demais) |
| Tokens --bg/--surface/--grid/--faint/--accent/--accent-fill/--outline/--focus/--danger/--warn | ✅ todos definidos em :root (l.8-33) e usados corretamente; fallbacks `var(--x, #...)` preservados |
| [data-theme="light"] | ✅ intacto (l.55-66), com `--accent-2:#15803d` novo na l.62 + comentário |
| body.loja | ✅ intacto (35 regras pré-M4 + aditivas no bloco M4) |
| Transições ≤0.25s | ✅ máx. 0.2s (dur-fast 0.15s / dur 0.2s); animação modal-in 0.2s |
| Allowlist seletores JS | ✅ 30/30 seletores do M4 existem no DOM (app.js/admin.js/player.js/loja.html) ou CSS-base; JS diff (M3) não renomeou nada (única adição: querySelector('.btn-accent') em admin.js, legítima) |
| CSS servido | ✅ servidor restartado pelo reviewer (PID 27280, health 200); /style.css = 57951 bytes com marker M4, M4 ANTES de reduced-motion, --accent-2:#15803d, :has(.login-error), @keyframes modal-in |
| HTML intacto | ✅ nenhum .html modificado (git status: só style.css + JS do M3 + backend M2) |
| Contrastes novos | ✅ --accent-2 #15803d/white 5.02:1 PASS; #1db954 no dark 6.6:1 PASS; branco sobre --accent-2-fill 5.02:1 PASS; :has() OK (Chrome 105+/Safari 15.4+/FF 121+, uso progressivo — só borda) |

### Observações não bloqueantes

1. `.genre-card` (l.2384-2388) não existe no DOM (0 refs em JS/HTML — dead code pré-existente da l.504). Sem quebra; sugerido limpar ou usar no M5.
2. Light theme nunca é aplicado (0 refs a `data-theme` em JS/HTML). O fix do --accent-2 é defensivo e correto; badge âmbar #ffcf6b falharia AA em light (pré-existente) — irrelevante enquanto o light não for ativado.
3. `body.loja .card` base tinha `transition:none; transform:none` — o M4 sobrescreve corretamente pela ordem de cascata (mesma especificidade, bloco posterior). Verificado.
4. Nenhum `!important` novo no M4; `:has(.login-error:not(:empty))` tem fallback seguro (sem a regra, borda default).

### Sugestões para M5 (validação Playwright visual)
- Screenshot diff por rota: login (glow + tile logo + ring focus-within + pill de erro), home (card hover-lift, card-play scale, scrollbar fina), player (sombra superior, capa, progress 6px no drag via classe `.dragging`), sidebar (pill ativo), admin (tabs, badge, chip, btn-accent glow), loja (buy-btn verde, focus verde no input), empty-states tracejados.
- Validar hover/active via força de estado Playwright (hover não sai em screenshot estático).
- 375px sem overflow em todas as rotas (scrollWidth <= innerWidth), incluindo loja pós-polimento.
- prefers-reduced-motion: rodar 1 spec com emulação reduzida e conferir que modal-in/transições não animam (via screenshot A/B).
- Contraste programático: re-aplicar cálculo WCAG das combinações-chave (accent/2, on-accent, faint, grid, subtext) sobre screenshots reais.
- Dead code: decidir manter/remover .genre-card (ou exercitá-lo).

## Task: M2 [G2] — Fixes backend Go (parecer transcrito pelo host da sessão do reviewer)

**Veredicto: PASS** (2026-08-09, reviewer — parecer transcrito por limite de steps)

### Verificações executadas
- go build ./... exit 0; go vet ./... exit 0; `go test ./... -count=1 -v` → 12/12 PASS (integração NÃO pulou: 0.57–0.99s por teste contra Postgres real via DATABASE_URL do .env).
- git diff dos 4 arquivos + 2 novos: diffs limpos e cirúrgicos, sem mudanças fora de escopo.
- Servidor live GET / → 200.

### Verificações por arquivo
1. store/users.go (B2): INSERT ... RETURNING created_at com Scan(&u.CreatedAt) (l.39-43); transação preservada (dbTx envolve INSERT + setUserCategories com rollback); ErrDuplicate intacto; ambos os handlers que ecoam u ganham timestamp real.
2. static.go (B-NEW-1): /api/* → 404 JSON via writeError; /assets/* → 404 http.NotFound (text/plain); fallback SPA preservado. Edge cases: /api e /api/ → JSON 404; query string não interfere (path.Clean); assets conhecidos com ?v= continuam servidos; métodos errados em rotas registradas → 405 do ServeMux (Go 1.22+), nunca chegam ao static. Compatibilidade: api.js só chama endpoints registrados; NENHUMA referência a /assets/ no frontend (usa ./style.css, ./app.js na raiz) → 404 novo não quebra nada.
3. handlers_api.go (B-NEW-3/B-NEW-4): SongExists antes de SetLike (query EXISTS sobre PK indexada, ~1 roundtrip); id vazio → 404. Upload: storage.Remove(key) best-effort quando os DOIS IndexFile falham, 500 mantido, slog.Warn se Remove falhar; nenhum outro caminho pós-Put deixa órfão.
4. handlers_store.go (B-NEW-2): validação ANTES da criação do usuário (400 + invalidCategoryIds vence o 409 de duplicado — ordem correta); categoryIds:[] → 200 com token preservado; GrantUserCategories recebe lista validada; TODO(security) documenta dívida.
5. Testes: static_test.go (4 unit, sem DB, assertam status + Content-Type + body real); handlers_bugs_test.go (4 integração gated por DB, skip limpo, telefones/IDs aleatórios com retry, t.Cleanup(DeleteUser), unlike ao final do teste 204, register rejeitado NÃO criou usuário, createdAt contra valor real — não hardcoded). Todos PASS no Postgres real.

### Sugestões não bloqueantes para M5
1. Teste de integração do happy path do register (categoryIds:[] → 200 + token) — hoje só o 400 está coberto.
2. Teste de regressão para /api/ com trailing slash → JSON 404.
3. m1-srv.pid sumiu apesar do servidor rodando — o script de start do M5 deve regenerá-lo para o webServer do Playwright.
4. B-NEW-2 segue como dívida documentada — decidir produto/fixture no M5 sem depender dele para conceder categorias pagas.

## Task: M3 [G2] — Fixes frontend JS (parecer transcrito pelo host da sessão do reviewer)

**Veredicto: BLOCK (1 item de severidade ALTA)** (2026-08-09, reviewer)

### Problema bloqueante — jwtExpiry sem padding base64url → B1 NÃO corrigido em navegador
- web/assets/app.js:127-136: `atob(payload.replace(/-/g,'+').replace(/_/g,'/'))` não repõe o padding `=` do base64url. JWTs (RFC 7515) são base64url SEM padding; token real do servidor tem payload 222 chars (222 % 4 = 2).
- Em navegador, atob é ESTRITO → lança InvalidCharacterError → catch → jwtExpiry retorna null para TODOS os tokens reais → refreshAuth faz fetch com token expirado → 401 no console persiste (B1 intacto). O teste do worker passou por causa do atob leniente do Node (não representa o browser).
- Correção: adicionar padding antes do atob: `payload.replace(/-/g,'+').replace(/_/g,'/') + '='.repeat((4 - (payload.length % 4)) % 4)`. Edge cases (token sem exp, payload não-JSON, malformado, exp string) já cobertos pelo catch → null sem crash — corretos.

### Itens PASS confirmados (não precisam de retrabalho)
- removePlaylistTrack (app.js:806-815): try/catch + alert; sem render em falha (lista intacta).
- toggleLike (app.js:313-330): catch reverte classList E player.setLiked(wasLiked); sucesso reavalia isCurrent() pós-await (correto mesmo se música atual mudou).
- toggleLikeCurrent (app.js:1346-1362): async + catch reverte setLiked + refreshPlayerBar().
- Guard de teclado (app.js:1621-1657): exclui INPUT/TEXTAREA/SELECT/BUTTON/A/isContentEditable; preventDefault em todos os keys; cobre Space + ArrowUp/Down/Left/Right (Enter não é atalho).
- adminRefreshSeq (admin.js:9-13, 31-37, 70-77, 271-274, 461-463): seq incrementado no início do refresh() antes de limpar DOM/estado; guard checado nas 3 renders antes de tocar adminState/DOM. ⚠️ Menor: buildList()/categoryForm (admin.js:331-369) escrevem adminState.songs em path assíncrono sem guard — modal cobre UI, risco baixo.
- aria-pressed (admin.js:138-148): sync() atualiza ambos botões em todos estados, chamado na init (:168) e em cada toggle; phone só no ramo cliente (consistente com backend que limpa phone ao virar admin).
- save categoria (admin.js:429-446): saveBtn.disabled=true + finally reabilita (cobre erro).
- Integração: loja.html autocontido, selectors de app.js consistentes com syncPagePlayerState, player.setLiked existe. node --check 6/6 exit 0. Servidor serve código novo (app.js com jwtExpiry, admin.js com adminRefreshSeq/aria-pressed/saveBtn.disabled).

### Sugestões não bloqueantes para M5
1. Teste Playwright com token real expirado (payload unpadded do servidor) na tela de login, assertando zero console errors — hoje passaria só por atob leniente do Node.
2. Rapid double-click no like (like+unlike concorrentes fora de ordem) — pré-existente, avaliar no M5.
3. buildList sem seq guard — aceitável, cobrir com teste de troca de aba com modal aberto.

## Task: M3 [G2] — RE-REVIEW retry 1/3 (correção jwtExpiry / padding base64url)

**Veredicto: PASS** (2026-08-09, reviewer)

### Item bloqueante do BLOCK original — CONFIRMADO CORRIGIDO

web/assets/app.js:127-140 (jwtExpiry):
- **Padding presente e correto** (l.134): const b64 = payload.replace(/-/g,'+').replace(/_/g,'/') + '='.repeat((4 - (payload.length % 4)) % 4) — fórmula exata recomendada no BLOCK; cobre os 4 casos de módulo (0→'', 1→'===', 2→'==', 3→'=').
- **try/catch mantido** (l.137-139): token sem exp, payload não-JSON, malformado → null sem crash.
- **Edge cases OK**: !payload → null (l.130); exp string → null (l.136 	ypeof json.exp === 'number'); exp numérico → exp * 1000 ms.
- **refreshAuth** (l.142-156) usa jwtExpiry e, com token expirado, limpa o token e NÃO faz fetch — caminho do B1 (zero 401 no console).

### Verificações executadas (pelo reviewer)

| Check | Resultado |
|---|---|
| Fonte web/assets/app.js:134 | ✅ padding presente com comentário RFC 7515 |
| node --check web/assets/app.js | ✅ exit 0 |
| Servidor http://localhost:4533/ | ✅ 200 OK (1541 bytes) |
| GET /app.js servido | ✅ contém o .repeat do padding (Invoke-WebRequest + curl.exe, UTF-8 íntegro) |
| Matemática do fix (caso crítico %4==2) | ✅ payload 194 chars %4==2 → fórmula adiciona ==, JSON.parse(atob) OK, exp*1000 correto |
| Validação browser | ⏭ confiado na evidência do worker (12/12 + real-token test em Chromium headless: token real 303 chars, payload 222 chars %4==2, console level=error=0) — script m3-validate.js não versionado no repo; o teste do worker usou atob ESTRITO do browser, exatamente o modo de falha do BLOCK |

### Conclusão
O único item bloqueante foi corrigido na fonte E no arquivo servido. Demais itens da task já tinham PASS no review anterior e não foram tocados. M3 considerada satisfeita.

**Sugestão não bloqueante (M5)**: versionar o m3-validate.js (ex.: e2e/) para regressão automática — hoje o único artefato de evidência é o relato no STATUS.md.

## Task: M5 [G3] — Suíte Playwright completa em e2e/ + verificação final

**Veredicto: PASS** (2026-08-09, reviewer — suíte re-executada de primeira mão)

### Evidência de primeira mão (executada pelo reviewer)

| Check | Resultado |
|---|---|
| Servidor :4533 | ✅ 200 OK antes do run (reuseExistingServer — server já estava UP) |
| Suíte Playwright completa | ✅ **49 passed, 1 skipped, 0 failed** (3.0m, workers 1) — desktop 24 passed + mobile 26 passed; skip intencional = responsive.spec no projeto desktop |
| `go build ./...` | ✅ exit 0 (re-executado) |
| `go vet ./...` | ✅ exit 0 (re-executado) |
| `go test ./... -count=1` | ✅ exit 0 fresco (phone 0.8s, server 6.5s integração Postgres real, stream 1.1s) |
| `node --check` 6 JS | ✅ app/player/admin/api/pwa/sw — 6/6 OK |
| Escopo git | ✅ apenas internal/server/*, internal/store/users.go, web/assets/{app.js,admin.js,style.css} + e2e/ novo + testes M2 (já revisados). Deleções .opencode/skills pré-existentes (documentadas desde M1); m1-srv.* = artefatos runtime do start script |

### Estrutura da suíte (revisada por inspeção)

- **playwright.config.js**: reuseExistingServer:true com webServer start-server.ps1 (sobe só se :4533 cair, health-check + boot via m1-start.ps1); 2 projetos (desktop chromium 1280x720, mobile chromium 375x812 isMobile+hasTouch — sem webkit, corretamente evitado por não estar instalado); workers 1 + fullyParallel:false (estado DB compartilhado); retries 0; trace/screenshot on-failure; --autoplay-policy=no-user-gesture-required para áudio headless.
- **helpers.js**: envFromFile lê credenciais do .env em runtime (nunca commitadas); dismissPwa para a overlay B5 (~1.5s pós-load); trackConsole com predicado `ignore` (o 401 de senha errada é comportamento correto do produto); expectClean falha o teste com a lista de erros; loginAdmin/loginCliente/logout; API helpers (CRUD usuário/categoria via request fixture, store register, admin songs); ensureSilenceWav gera WAV PCM de 30s determinístico (44-byte header correto: RIFF/fmt/data, rate 44100, mono 16-bit).
- **8 specs**: login (admin, cliente, logout, senha errada sem crash + form reutilizável, **B1 regressão com token real expirado re-encodado → 0 console errors + pm_token limpo**), home/search/library (seções/cards admin, busca com waitForFunction rows|empty, biblioteca/categorias/playlists/liked/history, home cliente vazia sem crash — B4), player (play→barra, play/pause por aria-label, **seek que exercita o fix a11y** via poll de aria-valuemax ≠ 0 + tempo muda, volume 0.35→0 com assert em window.__player.getState() + data-icon="volumeX" no vol-icon, next/prev), loja (preços R$ 9,90 + href checkout, login cliente via phone, registro de novo cliente → token + visível no admin + login por phone + cleanup via API), admin (CRUD usuário UI com regex âncora p/ old-name; CRUD categoria nunca tocando "Cristão"; 145+ músicas; aria-pressed sync; PUT atrasado 1500ms → saveBtn disabled), responsive (scrollWidth <= innerWidth em login/home/search/library/admin/loja), pwa (sw registra, manifest link, overlay "Instale o Play Music" + "Agora não" fecha).
- **Não-falsos**: asserções sobre comportamento real (aria attrs, textos, contagens, console), fixtures throwaway com nome/telefone aleatórios, cleanup try/finally + best-effort via API, stubbing de stream com WAV local (não depende de CDN externa).

### Fixes de produto verificados na fonte (web/assets/app.js)

- **(a) a11y updateBarInPlace**: `refs.track` setado em bottomBar (l.1235); updateBarInPlace (l.1530-1544) atualiza `aria-valuemax`/`aria-valuenow` em cada tick (l.1541-1544) — o seek spec polla `aria-valuemax != 0`, exercitando exatamente o fix. ✅
- **(b) data-icon consistente**: `icon()` seta `dataset.icon` no svg na criação (l.110); updateBarInPlace (l.1545-1554) compara `firstElementChild.dataset.icon` com o alvo, troca innerHTML e **seta dataset.icon no novo svg** (l.1553) — os dois caminhos de render ficam consistentes; `icons.volumeX` existe (l.96). O teste `[data-icon="volumeX"]` no mobile (onde o slider fica display:none) valida via DOM, agnóstico de layout. ✅

### Problemas encontrados

Nenhum bloqueante. Observações não bloqueantes:

1. **login.spec.js:77** — comentário "past (2026-09-01)" para `exp=1000000000` é impreciso (epoch = 2001-09-09); o valor continua no passado e o teste passa. Cosmético.
2. **login.spec.js:53** — ignore por substring `'status of 401'` pode mascarar um 401 não relacionado à senha errada naquele único teste; risco aceitável e escopado (demais testes não têm ignore).
3. **loja.spec test 3** é 100% API (sem page) — anotado corretamente; sem cobertura de console para esse fluxo (legítimo, não há navegação).
4. **e2e/fixtures/ e test-results/ gitignored** — silence-30s.wav regenerado sob demanda por ensureSilenceWav; depende do write ser permitido (OK no Windows).
5. categoryIds vazio em /api/admin/songs (gap do critic M2) segue sem teste específico de UI — não é bug da suíte; decisão de produto pendente documentada.

### Conclusão

M5 entregue por inteiro: suíte completa, não-falsa, com fixtures auto-limpas, zero console errors assertado por teste, e os dois bugs de produto (a11y da barra + consistência do ícone de volume) corrigidos na fonte e cobertos por testes que os exercitam. Verificação final verde de primeira mão (build/vet/test frescos + node --check 6/6). Escopo respeitado. **PASS — goal satisfeito.**

## Task: M5 [G3] — RE-REVIEW retry 1/3 (correcoes do BLOCK do critic)
**Veredicto: PASS** (inspecao de fonte pelo reviewer; suite 53/1/0 e builds verdes executados pelo host como evidencia de primeira mao).
- scanner.go: ErrInvalidAudio sentinel (l.26); metadata.Read falha e Duration<=0 propagam com %w (l.230-233); fluxo normal intacto.
- handlers_api.go: validacao PRE-STORAGE (400 antes do Put, zero orfao); pos-Put errors.Is(ErrInvalidAudio) -> 400 + storage.Remove; 201 re-le via GetSong (created_at real, songCols inclui s.created_at).
- Testes Go novos nao-falsos (junk->400 catalogo inalterado; WAV 1s -> 201 duration>0 createdAt real). DIVIDA: TestAdminUploadValidWavHasDurationAndCreatedAt nao remove objeto do bucket (scanner reindexa como titulo=UUID); documentar ou storage.Remove no cleanup.
- upload.spec.js: nomes unicos pw-e2e-upload-{ts}, asserts completos, comentario de manutencao atualizado. player.spec next/prev: comparacao por id robusta.
- Ambiente: catalogo 146, zero pw-e2e*/pw-test* no DB; scan e manual (POST /api/admin/scan) — estado limpo.
- Ressalva nao bloqueante: ND_FFMPEGPATH nao setado no .env — mp3 valido sem duracao em tags seria 400 (tradeoff da politica 'sem duracao -> invalido').
