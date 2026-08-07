# Loop State — My Project

Last run: 2026-08-07 (delete categoria "music" — COMMITTED f9e2dcc + pushed)

## High Priority (loop is acting or waiting on human)

0. **DELETE categoria "music"** → **DONE + COMMITTED (f9e2dcc, pushed)**: migration `0008_delete_music_category.sql` (`DELETE FROM categories WHERE lower(name)='music'`, CASCADE limpa category_songs/user_categories) + scanner.go ignora gênero "music" (case-insensitive) na criação automática de categorias. Evidência: `go build`/`go vet` OK, `go test ./...` OK (phone, stream), verifier APROVADO. Gap não bloqueante: admin ainda pode recriar/renomear para "music" via painel (decisão humana pendente).

1. ~~**Uncommitted WIP in `web/assets/player.js`**~~ → **COMMITTED (152fc66, pushed)**: playAudio() catch desync fixed (guard `if (!switching)` restored); volume 0.8, direct mediaSession handlers, readyState guards, end-of-queue clamp, optimistic pending-seek all validated and live.
2. ~~**ORIGINAL_REQUEST.md acceptance criteria unverified**~~ → **RESOLVED + COMMITTED**: all 6 checkboxes ticked with evidence (loop-reports/qa-r1r3.md). C4 admin render fully covered by T4 (loop-reports/qa-v116.md).
3. ~~**BUG-1 (duplicate email → 500)**~~ → **FIXED + COMMITTED (152fc66, pushed)**: SQLSTATE 23505 → ErrDuplicate → 409 "Já existe um usuário com esse e-mail, usuário ou telefone". Live-verified (409).
4. **Frontend design overhaul** → **DONE + COMMITTED (152fc66, pushed)**: loop (frontend-design-overhaul, 4/4 VERIFIED_PASS) — design system em tokens 2 camadas + `[data-theme="light"]`, contraste WCAG AA (accent 4.79, green 5.02, faint ≥4.5, outline ≥3.1), ~30 componentes, loja unificada no style.css (body.loja), tipografia clamp, 30 transições ≤0.25s, prefers-reduced-motion, :focus-visible, 12 aria-pressed aditivos. Diffs: loop-reports/design-t{2,3,4}.diff.

## Watch List

1. **Dependabot PRs #1, #2, #5, #6, #9, #19** → verdict CLOSE (all stale, deps removed by b2e353f8). Report: loop-reports/dependabot-review.md. Needs human: `gh pr close 1 2 5 6 9 19` (gh CLI not installed locally).
2. **Front-end regression risk** — design overhaul landed (big CSS change + additive aria in app.js/admin.js). Watch for visual/behavioral regressions in player controls, admin forms, loja checkout next 48h. Zero front-end tests exist.
3. **Server logs healthy** — restarted post-overhaul (HTTP 200), all 200s, srv-err empty. CSS/JS served byte-identical to committed tree.

## Recent Noise (ignored this run)

- v1.16.0 feature QA: no regressions (login user/email/phone, admin/cliente forms, tipo cleanup, guards 400, thumbnails 0 broken, covers OK).
- Throwaway clients deleted via admin API (11999990001, 11999998888) — only seed admin remains.
- Optional: enable CI (.github/) so future Dependabot PRs validate.
- Optional: self-hosted WOFF2 webfont (offline-safe) — decided against in design-proposal, revisitable.

---
Run log: 2026-08-07 — delete categoria "music": commit f9e2dcc (migration 0008 + guard do scanner) merged em master e pushed to origin/master; working tree clean.
