# Teamwork Report
## Summary
Run fix-bugs-design-20260808 COMPLETO (5/5 tasks, auditoria CLEAN). Bugs de frontend e backend corrigidos (10 bugs de produto: B1-B5 + B-NEW-1..4 + 2 descobertos pelos testes: a11y aria-valuemax e upload de não-áudio/createdAt), design elevado a nível profissional (vision 8.5/10, contraste WCAG AA 15/15, responsivo 375px) e suíte Playwright completa criada e verde: 53 passed / 1 skipped (intencional) / 0 failed, com zero console errors em todas as rotas. go build/vet/test verdes (10 testes Go, integração com Postgres real), node --check 15/15. Servidor rodando em :4533, catálogo em 146.
## Tasks Final Status
- [x] M1 [G1] Baseline — CONCLUÍDA (servidor UP, build/vet/test verdes, diagnóstico Playwright, inventário B1-B5 + B-NEW-1..4; REVIEW PASS, CRITIC BLOCK → correção documental aplicada)
- [x] M2 [G2] Fixes backend — CONCLUÍDA (B2 RETURNING created_at; B-NEW-1 404 API/assets; B-NEW-3 liked 404; B-NEW-4 cleanup upload; B-NEW-2 validação categoryIds + TODO security; 8 testes Go novos; REVIEW PASS + CRITIC PASS)
- [x] M3 [G2] Fixes frontend — CONCLUÍDA (7 fixes: removePlaylistTrack, toggleLike sync, toggleLikeCurrent, guard teclado, race admin, aria-pressed, disable save + jwtExpiry com padding — retry 1/3; suíte 12/12; RE-REVIEW PASS)
- [x] M4 [G2] Polimento visual — CONCLUÍDA (bloco aditivo em style.css: login, player, cards, sidebar, admin, loja, micro-interações; fix contraste --accent-2 no light; vision 8.5/10; REVIEW PASS)
- [x] M5 [G3] Suíte Playwright — CONCLUÍDA (e2e/ com 9 specs + config + helpers; bugs revelados e corrigidos: a11y aria-valuemax/now, data-icon volume, upload não-áudio→400, createdAt do upload; 53 passed / 1 skipped / 0 failed; verificação final completa; REVIEW PASS + CRITIC BLOCK → retry 1/3 → RE-REVIEW PASS)
## Final Audit
CLEAN — ver AUDIT.md. Nenhum cheating; testes não-falsos (Postgres real, asserts genuínos); .env intocado; nenhuma mudança fora de escopo.
## Learnings
Ver MEMORY.md (resumo: .env não auto-carregado → m1-start.ps1; atob do browser é estrito vs Node leniente (padding base64url obrigatório); Refs do Playwright invalidam após re-render; overlay PWA re-exibe; clientes não são seedados; refs de teste Playwright: comparar por id quando títulos podem duplicar; upload.spec polui catálogo/bucket (sem API de delete) — cleanup manual documentado; locks do Postgres com kill -Force → pg_terminate_backend; kill -Force do servidor deixa transações abertas; scanner reindexa órfãos do bucket).
## Baseline (M1) — inventário de bugs
### Confirmados
- **B1 (frontend/UX)** — console error `401 @ /api/me` na tela de login (fetch de auth sem token). Evidência: console error em `/` sem sessão. Local provável: app.js (auth.loading) / api.js. Viola meta "zero console errors".
- **B2 (backend)** — `POST /api/admin/users` retorna `"createdAt":"0001-01-01T00:00:00Z"` (zero-value). Evidência: body 201 capturado. Local: handlers_api.go handleAdminCreateUser (~525+).
- **B3 (produto/seed)** — cliente `(99) 99999-9999` NÃO existe no seed; login 401 "Telefone não cadastrado" até admin criar. Evidência: 401 /auth/login + msg UI. STRATEGY.md estava incorreto ao assumir seed de clientes.
- **B4 (produto)** — cliente sem categorias atribuídas: home/busca/biblioteca vazias ("Nada encontrado") mesmo com 145+ músicas no catálogo. Modelo de acesso por categoria — confirmar se intencional.
- **B5 (observação UX)** — overlay PWA re-exibe a cada page load (~1.5s), fecha só com clique em "Agora não". Verificar persistência de dismiss (pwa.js). Reviewer: comportamento INTENCIONAL (pwa.js:112-115), decisão de produto.
### Novos (descobertos pelo critic adversarial — adicionados após BLOCK)
- **B-NEW-1 (backend/contrato)** — Rota API inexistente retorna 200 + HTML (SPA fallback), não 404: GET /api/banana/xyz → 200 text/html. Causa: handleStatic() (static.go:45-49) aplica fallback SPA a qualquer path desconhecido, inclusive /api/*. GET /assets/nope.js → 200 HTML → "Refused to execute script" (viola zero console errors). Prioridade M2.
- **B-NEW-2 (backend/segurança)** — /api/store/register público libera categorias pagas sem prova de pagamento: POST sem auth com categoryIds concede categorias no mesmo call (handlers_store.go:33-103). IDs públicos via /api/store/categories; sem segredo pós-checkout. Prioridade M2 alta.
- **B-NEW-3 (backend)** — PUT /api/me/liked/{inexistente} → 204 sem validar existência da música (handlers_api.go:429+) → desync de estado.
- **B-NEW-4 (backend)** — maxUploadBytes = 512MB (handlers_api.go:934; 15MB é só fotos — PLAN.md incorreto); storage.Put OK + IndexFile falhar 2x → objeto órfão no bucket sem cleanup (handlers_api.go:1025-1041).
### Desmentidos (não corrigir)
- player.js:221-224 — guarda `shuffle && queue.length > 1` (linha 221) impede o loop infinito (reviewer).
- loja.html:93-103 — renderUser() sempre faz innerHTML antes de bindLogin() → duplicação improvável; validar com teste, não mudar. Arquivo real: web/assets/loja.html.
### Suspeitos (de STRATEGY.md, não exercitados — exigem player/queue e interações admin)
- app.js:777-780 — removePlaylistTrack() sem try/catch → unhandled rejection se DELETE falhar.
- app.js:288-301 — toggleLike() reverte botão no catch mas não player.setLiked() (desync barra vs lista); toggleLikeCurrent :1311-1319 sem catch.
- app.js:1578-1611 — atalhos de teclado não excluem BUTTON/A do target (duplo-disparo espaço/arrows).
- admin.js:24-32 — refresh() sem guard de corrida (stale render em troca rápida de abas).
- admin.js:101-176 — aria-pressed dessincronizado ao alternar isAdmin; edição admin sem phone no payload (validar).
- admin.js:410-422 — save() categoria sem disable (duplo submit).
- loja.html:97-103 — renderUser() chama bindLogin() sem verificar nós existentes (listeners duplicados).
- player.js:221-224 — do/while shuffle com queue.length===1 pode loop infinito de Math.random.
## Tasks Final Status
- [x] M1 [G1] Baseline — CONCLUÍDA (servidor UP, build/vet/test verdes, diagnóstico Playwright, inventário B1-B5 + B-NEW-1..4; REVIEW PASS, CRITIC BLOCK → correção documental aplicada)
- [x] M2 [G2] Fixes backend — CONCLUÍDA (B2 RETURNING created_at; B-NEW-1 404 API/assets; B-NEW-3 liked 404; B-NEW-4 cleanup upload; B-NEW-2 validação categoryIds + TODO security; 8 testes Go novos; REVIEW PASS + CRITIC PASS)
- [x] M3 [G2] Fixes frontend — CONCLUÍDA (7 fixes: removePlaylistTrack, toggleLike sync, toggleLikeCurrent, guard teclado, race admin, aria-pressed, disable save + jwtExpiry com padding — retry 1/3; suíte 12/12; RE-REVIEW PASS)
- [x] M4 [G2] Polimento visual — CONCLUÍDA (bloco aditivo em style.css: login, player, cards, sidebar, admin, loja, micro-interações; fix contraste --accent-2 no light; vision 8.5/10; REVIEW PASS)
- [x] M5 [G3] Suíte Playwright — CONCLUÍDA (e2e/ com 9 specs + config + helpers; bugs revelados e corrigidos: a11y aria-valuemax/now, data-icon volume, upload não-áudio→400, createdAt do upload; 53 passed / 1 skipped / 0 failed; verificação final completa; REVIEW PASS + CRITIC BLOCK → retry 1/3 → RE-REVIEW PASS)
## Final Audit
CLEAN — ver AUDIT.md. Nenhum cheating; testes não-falsos (Postgres real, asserts genuínos); .env intocado; nenhuma mudança fora de escopo.
## Learnings
(pending)
