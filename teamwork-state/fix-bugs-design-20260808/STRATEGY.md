# Strategy — fix-bugs-design-20260808

## Contexto

**Stack:** Backend Go 1.26 (main.go + internal/), Postgres via pgxpool com 8 migrations SQL embutidas (internal/db/migrations/0001..0008), storage MinIO/S3 (internal/storage/), JWT HS256 (internal/auth/), servidor HTTP em `0.0.0.0:4533` servindo API + UI embutida via go:embed (web/embed.go, web/assets/).

**Frontend:** vanilla JS em módulos ES: app.js (1623 ln, router hash SPA), admin.js (760 ln), player.js (329 ln, singleton de áudio com Media Session), api.js (client fetch + JWT em X-ND-Authorization ou ?jwt=), pwa.js + sw.js (PWA). style.css (2255 ln) já tem design system em tokens de 2 camadas (:root primitivas + semânticas, [data-theme="light"], clamp typography, :focus-visible, prefers-reduced-motion, transições ≤0.25s, contraste WCAG AA) — entregue pelo loop frontend-design-overhaul (commit 152fc66) e validado em a92001b3.

**Testes existentes:** apenas 2 unit tests Go (internal/phone/phone_test.go, internal/stream/stream_test.go). **Zero testes de frontend/E2E.**

**Infra p/ testes:** docker-compose.yml expõe 4533, Dockerfile usa ffmpeg+alpine. Ambiente local: Go 1.26, Node 24, ffmpeg, Docker. Playwright disponível via MCP (npx @playwright/mcp).

**⚠️ Estado crítico:** servidor Go NÃO está rodando (netstat :4533 vazio; srv.log de 2026-08-07). Toda verificação Playwright exige subir o servidor antes (`go build -o play-music . && ./play-music`). Git: working tree tem apenas deleções não relacionadas (.opencode/agents/knowledge-sources/*, skills/loop-triage/) + teamwork-state/ untracked.

## Bugs / Áreas de Risco Identificadas

**Frontend (JS):**
- app.js:777-780 — removePlaylistTrack() sem try/catch → unhandled promise rejection se DELETE falhar.
- app.js:288-301 — toggleLike() reverte o botão no catch mas NÃO reverte player.setLiked() (desync barra vs lista); toggleLikeCurrent :1311-1319 sem catch → unhandled rejection.
- app.js:1392-1452 — render() destrói/recria árvore a cada evento; risco de perda de foco/gesto de seek em re-renders concorrentes.
- app.js:1578-1611 — atalhos de teclado não excluem BUTTON/A do target; espaço/arrows em botão focado pode duplo-disparar.
- admin.js:24-32 — refresh() sem guard de corrida: troca rápida de abas pode renderizar resposta obsoleta.
- admin.js:101-176 — userForm(): aria-pressed fica dessincronizado ao alternar; edição de admin não inclui phone no payload (validar).
- admin.js:410-422 — save() de categoria não desabilita botão (duplo submit possível).
- loja.html:97-103 — renderUser() chama bindLogin() após innerHTML sem verificar nós existentes (listeners duplicados).
- player.js — audio singleton global; next() com queue vazia e repeat/shuffle com queue.length===1: verificar do/while em :221-224 (loop infinito Math.random).

**Backend (Go):**
- handlers_store.go:33-103 — handleStoreRegister público cria conta com token automaticamente; validar superfície.
- handlers_api.go (986 ln) — auditoria focada: handleAdminSongs ({songs, categoryIds, categoryList}), handleHome/handleSearch, handleUploadSong (multipart 15MB?), handleStream (Range/206), handleArtwork.
- internal/store/* — sem testes além de phone/stream; migrations exigem go test ./... completo.
- main.go:99-105 — erro no ListenAndServe só loga e chama stop() (ok).

**Regressões de design:** style.css reescrito e commitado — qualquer mudança deve preservar tokens/aliases (--bg, --surface, --grid, --faint), body.loja, [data-theme="light"], e o allowlist de seletores usados pelos JS.

**Ambiente:** servidor parado; .env tem credenciais reais (MinIO/Postgres/Redis) — **não editar**; seed existe (categorias, músicas, admin único).

## Estratégia Recomendada

1. **Diagnóstico (M1)**: subir servidor, go build/vet/test, navegar Playwright em todas as rotas (login cliente (99) 99999-9999 + admin via .env), capturar console errors, inventário de bugs reproduzidos.
2. **Correções backend (M2)**: validação/status codes, 500→4xx, garantir contratos JSON do admin.js.
3. **Correções frontend (M3)**: try/catch removePlaylistTrack/toggleLike, guard de teclado, race no admin refresh, dessync aria-pressed, bugs do player.
4. **Refinamento visual profissional (M4)**: incremental em cima dos tokens atuais — não reinventar; manter contraste AA.
5. **Suíte Playwright (M5)**: login (admin+cliente), home/busca/biblioteca, player (play/pause/seek/volume/next/prev), loja (checkout URL), admin CRUD usuário/categoria, responsivo 375px, zero console errors, PWA/sw. Recomendado: suíte Node própria em e2e/ com webServer no playwright.config.

## Milestones

- **M1 [G1]** — Baseline: subir server, go build/vet/test verdes, diagnóstico Playwright completo, inventário de bugs em REPORT.md.
- **M2 [G2]** — Fixes backend (status codes, contratos, validações) + testes Go novos onde houver bug.
- **M3 [G3]** — Fixes frontend JS (todos os itens acima), node --check limpo, zero console errors.
- **M4 [G3]** — Polimento visual profissional (tokens existentes), sem regressão de contraste/responsivo.
- **M5 [G4]** — Suíte Playwright completa passando; verificação final go test ./... + console limpo.

## Critérios de Verificação

- `go build ./...` e `go vet ./...` → exit 0
- `go test ./...` → ok (phone, stream, + novos)
- `node --check web/assets/app.js` (+ player.js, admin.js, api.js, pwa.js, sw.js) → sem erros
- Servidor: escutando em http://localhost:4533; Invoke-WebRequest → 200
- Playwright: zero erros de console em todas as rotas; fluxos login cliente+admin, play/seek/volume, CRUD admin, loja, 375px sem overflow (scrollWidth <= innerWidth)

## Riscos

- Servidor precisa estar rodando para qualquer teste; não há processo ativo.
- Regressão do redesign commitado: preservar aliases e contrastes AA.
- Auth no Playwright: JWT em localStorage (pm_token); login de cliente por telefone; admin via .env (nunca commitar credenciais).
- Dados de seed: não deletar categorias/usuários reais; clientes de teste throwaway.
- Audio/stream em headless: Audio.play() pode ser bloqueado por autoplay policy — usar --autoplay-policy=no-user-gesture-required ou dismiss da PWA overlay (aparece aos 1.5s — fechar antes de asserts).
- SW/PWA cache: testes devem limpar storage ou usar contexto novo.
