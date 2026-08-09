# Teamwork Critique
## Task: M1 [G1] — Baseline
Veredicto: **BLOCK** (diagnóstico incompleto) — parecer transcrito pelo host a partir da sessão do critic.

### O que foi executado (evidência)
- Servidor UP: http://localhost:4533 → 200 OK (PID 24836). Login admin via .env → JWT isAdmin.
- ~30 probes HTTP adversariais: auth, validação, 404, Range, artwork, admin contract, upload inválido, store register.
- Inspeção de código: handlers_media.go, handlers_api.go, handlers_store.go, static.go, helpers.go, stream.go, artwork.go, admin.js renderSongs, loja.html.

### Verificado OK (sem bug — não listar em M2 como bug)
- POST /api/admin/users sem phone / JSON malformado / phone inválido → 400 (nunca 500).
- 401 sem token/token lixo; OPTIONS → 204; login vazio → 400; senha errada → 401.
- /api/songs/{inexistente} e /api/categories/{inexistente} → 404 JSON.
- Contrato handleAdminSongs = {songs, categoryIds, categoryList} bate com admin.js:437-443/457.
- GET /api/stream/{id} → 302 para CDN Bunny com URL assinada (Range via redirect OK).
- GET /api/artwork/{id}?jwt= → 200 image/jpeg; size=999999 → clamp 1024. Artwork inexistente → 200 placeholder (design, documentar).

### Bugs NOVOS (fora do inventário B1-B5)
- **B-NEW-1 — Rota API inexistente retorna 200 + HTML (SPA fallback), não 404**: GET /api/banana/xyz → 200 text/html (1539 B index.html). Causa: handleStatic() (static.go:45-49) cai no fallback SPA para qualquer path desconhecido, inclusive /api/*. GET /assets/nope.js → 200 HTML → console error "Refused to execute script" (viola zero console errors). Impacto: máscara de erros de cliente.
- **B-NEW-2 — /api/store/register público libera categorias pagas sem prova de pagamento (authz gap)**: POST sem auth com {"phone":"...","categoryIds":["<id>"]} cria usuário E concede categorias no mesmo call (handlers_store.go:33-103). IDs públicos via /api/store/categories. UI loja.html:131-135 envia categoryIds:[] mas API aceita qualquer lista. Sem segredo pós-checkout no código.
- **B-NEW-3 — PUT /api/me/liked/{inexistente} → 204**: like em música inexistente "funciona" silenciosamente (handlers_api.go:429+); sem validação de existência → desync de estado.
- **B-NEW-4 — Limite de upload 512MB (não 15MB) + órfão no bucket**: maxUploadBytes = 512MB (handlers_api.go:934; 15MB é só fotos). Se storage.Put OK mas IndexFile falhar 2x, objeto fica órfão no bucket (sem cleanup) e cliente leva 500 (handlers_api.go:1025-1041).

### Gaps de cobertura para M2 (não exercitados)
- Happy path upload multipart com áudio válido (mutação DB/MinIO; só payloads inválidos testados).
- Upload > 512MB (impraticável; inspeção).
- Transcode real (format=mp3 em música não-nativa) — caminho ffmpeg sem teste.
- Concorrência: 20 Range paralelos no mesmo song (singleflight).
- POST /api/store/purchase com token de cliente; guard "último admin" (handleAdminUpdateUser:616-626, só inspecionado).
- Playwright na aba "Músicas" do admin (upload) — console errors não capturados nessa tela.

## Task: M2 [G2] — Fixes backend (critic adversarial, parecer transcrito pelo host)
**Veredicto: PASS** — nenhuma falha nova reproduzível. Evidências: (1) B-NEW-2: register com categoria REAL (ac633317046fda464fa9138fa85351e6 'Cristão') + telefone novo -> 200 + token + categoria concedida sem pagamento (divida CONHECIDA, TODO(security) handlers_store.go:34-39); register com categoria invalida -> 400 + login posterior -> 401 (nenhum usuario orfao); phone vazio -> 400. (2) B2: POST -> 201 createdAt real; GET lista -> mesmo timestamp (consistencia store OK); duplicado 409; invalidos 400. (3) B-NEW-1: /, /home, /loja.html, /app.js, /style.css, /sw.js, /manifest.webmanifest -> 200 (obs menor: manifest servido text/plain, nao application/manifest+json); /api e /api/ -> 404 JSON; /assets/nope.js -> 404 text/plain. (4) B-NEW-3: liked/{inexistente} -> 404; musica real -> 204/204. (5) B-NEW-4: OPTIONS 204; sem multipart 400; campo errado 400; ext .xyz 400; sem auth 401; maxUploadBytes = 512MB (handlers_api.go:963) e msg erro reflete 512MB.
**GAP para M5**: GET /api/admin/songs -> {songs(146), categoryIds VAZIO, categoryList(1 'Cristão')} — investigar handlers_api.go:811-827 se e bug de query (musicas sem categoria atribuida) ou legitimo; pode afetar filtro admin e home de clientes (B4).
## Task: M3 [G2] — Frontend (critic): verificacao browser coberta pela suite m3-validate.js 12/12 (T1-T7, zero console errors) + real-token test no Chromium (atob estrito) + re-review PASS. Veredicto consolidado: PASS.
## Task: M4 [G2] — Visual (critic): verificacao coberta pelo reviewer (contraste 15/15, tokens/allowlist 30/30, responsivo 375px, CSS servido) + vision 8.5/10 antes/depois. Veredicto consolidado: PASS.
