# Loop State — My Project

Last run: 2026-08-09 — teamwork `fix-bugs-design-20260808` (bugs frontend+backend, redesign profissional, suíte Playwright) — COMMITTED + pushed

## High Priority (loop is acting or waiting on human)

0. **Correções de bugs frontend/backend + redesign + suíte Playwright** → **DONE + COMMITTED + pushed** (run teamwork fix-bugs-design-20260808, 5/5 tasks, auditoria CLEAN). Bugs corrigidos (12): B1 (401 /api/me no console — jwtExpiry com padding base64url, token expirado limpo sem fetch), B2 (createdAt zero-value em users — RETURNING created_at), B-NEW-1 (rota API inexistente → 404 JSON, assets → 404), B-NEW-3 (liked/{inexistente} → 404), B-NEW-4 (cleanup órfão no bucket), B-NEW-2 (validação categoryIds + TODO security), upload de não-áudio → 400 (validação pré-storage), createdAt real no 201 do upload, a11y (aria-valuemax/now sync), data-icon do volume, removePlaylistTrack/toggleLike/teclado/race admin/aria-pressed/duplo submit. Design: bloco aditivo M4 em style.css (vision 8.5/10, contraste AA 15/15, responsivo 375px). Suíte: e2e/ com 9 specs — 53 passed / 1 skipped / 0 failed, zero console errors. Evidência: teamwork-state/fix-bugs-design-20260808/ (REPORT/REVIEW/CRITIQUE/AUDIT/MEMORY + screenshots antes/depois em artifacts/).

1. **Dívidas documentadas (não bloqueantes, decisão de produto)** — B-NEW-2 (register público concede categorias sem prova de pagamento — integrar gateway), categoryIds vazio em /api/admin/songs (category_songs sem backfill; scanner não reindexa; cliente novo vê tudo vazio — B4), overlay PWA re-exibe a cada load (persistir dismiss?), suíte deixa 2 WAVs de teste por rodada no catálogo/bucket (sem API de delete de songs — cleanup manual documentado em e2e/tests/upload.spec.js), transcode format=mp3 sem fixture (seed 100% mp3), ND_FFMPEGPATH não setado no .env.

## Watch List

1. **Servidor local** — play-music.exe rodando em :4533 (iniciado via teamwork-state/fix-bugs-design-20260808/m1-start.ps1, que carrega .env — o app NÃO auto-carrega .env). Logs em m1-srv.log / m1-srv-err.log (gitignored). Catalogo em 146 músicas.
2. **Upload de músicas via e2e** — a suíte (upload.spec) insere WAVs de teste no catálogo; rodadas futuras precisam do cleanup documentado (DELETE LIKE 'pw-e2e%' + objetos órfãos do bucket, senão o scanner reindexa).
3. **Postgres remoto (72.62.11.235)** — kill -Force do servidor deixa transações abertas que seguram locks (DELETEs travam); usar pg_terminate_backend se ocorrer.
4. **Front-end regression risk** — redesign + fixes JS commitados; suíte e2e (53 testes) cobre as rotas principais como rede de segurança.

## Recent Noise (ignored this run)

- Deleções pré-existentes de .opencode/agents/knowledge-sources/* e skills/loop-triage/ (anteriores ao run; não commitadas — verificar se são intencionais).
- B5 overlay PWA: comportamento intencional (pwa.js "mostra sempre em toda visita").
- Manifest webmanifest servido como text/plain (deveria ser application/manifest+json) — cosmético.
- .genre-card é dead code no CSS (pré-existente).

---
Run log: 2026-08-09 — teamwork fix-bugs-design-20260808: 5/5 tasks (baseline, fixes backend, fixes frontend, polimento visual, suíte Playwright), auditoria CLEAN, push concluído. Ver teamwork-state/fix-bugs-design-20260808/REPORT.md para detalhes.
