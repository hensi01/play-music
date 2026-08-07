# Loop Memory
Updated continuously by all agents as they discover things.
## Learnings
### Windows shell & git gotchas (T1 + T2, 2026-08-07)
- PS `>` writes UTF-16LE — NEVER redirect git output to a file with it; capture LF diffs via `cmd /c "git diff web/assets/player.js > file"` (verified from both main tree and worktree).
- `git worktree add <path> master` fails with "already used by worktree" when master is checked out — always add with `--detach`. Remove with `git worktree remove --force`.
- `git apply` inside the worktree applies LF→CRLF cleanly (autocrlf=true); `git -C ..\play-music-loop-wt diff` works from the main tree — no need to cd.
- `git merge-tree --write-tree origin/master <branch>` prints conflict info on stderr — capture with `2>&1`; exit 1 = conflict, empty stdout = clean merge.
- `go mod tidy -diff` exits 1 when a delta exists — record "exit 1 (diff found — expected)", not "failure".
- Worktree path from the main tree is `..\play-music-loop-wt`; loop-reports lives at `..\play-music\loop-reports\` when inside the worktree (task brief's `../loop-reports/` is wrong).

### player.js desync root cause + fix (T2)
- Root cause: during src-swap the AbortError rejection fires while `switching = true`; WIP's unconditional `switching = false` disarmed the pause-event consumption guard → UI flips to paused mid-playback (live desync).
- Fix (verified): restore `if (!switching)` guard in playAudio() catch (player.js:117-123), drop the WIP's `switching = false`, add 4-line AbortError comment. Final diff +18/-7. Other 6 WIP hunks (volume 0.8, mediaSession direct handlers, end-of-queue Math.max clamp, prev/seek readyState guards, optimistic pending-seek) validated safe — KEEP.
- Validation procedure: `node --check web/assets/player.js` (exit 0), verify diff stats match claims, check `git status --porcelain` for unintended main-tree changes.

### Dependabot stale-PR root cause (T1)
- Commit b2e353f8 (2026-08-05) slimmed go.mod (removed goose/jwx/go-sqlite3/hashstructure), rewrote Dockerfile (no osxcross stage) and deleted .github/ → all 6 open PRs (#1 #2 #5 #6 #9 #19) base off old commits and conflict with origin/master; verdict CLOSE all. Target absent from master = close criterion.
- No gh CLI: map PRs→branches via `git ls-remote origin` `refs/pull/<n>/head`; unreviewed refs/pull/10-18 exist (npm_and_yarn/ui/) outside scope.

### QA hooks & environment
- `window.__player = { getState: () => state, audio }` exposed at web/assets/player.js:324 — assert player state programmatically via `playwright_browser_evaluate`.
- Server live on http://localhost:4533/ (HTTP 200, title "Play Music", 0 console errors) — healthy at HEAD; do NOT restart it, but see T3 section: it serves HEAD, so QA of fixed JS needs a rebuild.
- vision subagent (openrouter/qwen) exists for screenshot/OCR (`task vision: "analyze <path>: <question>"`, read-only) but is NOT callable from loop agent contexts — see T3 section.

### T4 v1.16.0 feature regression QA — consolidated (2026-08-07)
- Admin creds were provided in the T4 brief (transient browser use only, NEVER written to files/reports — the report says "admin credentials" without values).
- Seed admin DOES have an email in the live DB: `admin@playmusic.com` (GET /api/admin/users) — corrects the research note "seed admin without email" (auth.go:70-75 is the bootstrap default; live row has email). Email login is fully live-testable.
- Guard behavior live-verified: self-demote attempt (PUT own id, isAdmin:false) → 400 "Não é possível editar a própria conta aqui" (self-edit guard handlers_api.go:597-601 fires FIRST, before the count-guard 616-626); self-delete → 400 "Não é possível excluir a própria conta" (675-686). The count-guard (616-626) is NOT live-triggerable: with 2 admins a demotion would SUCCEED (forbidden), with 1 admin the self-edit guard blocks first. Verify statically.
- BUG confirmed live: duplicate email → 500 "Erro interno" on BOTH create (POST /api/admin/users) and edit (PUT) — handleStoreError (helpers.go:78-89) has no unique-violation→4xx mapping. Repro + evidence in loop-reports/qa-v116.md. Not fixed (record-only per brief).
- Thumbnails live structure: home grid = `.card img.card-image` (48px src `size=48`, naturalWidth 300 grid variant), list rows = `img.track-art` (40px, size=48 request), now-playing = `.now-playing-art` (size=64). All `loading="lazy"` — 0 broken across home/search/history/liked/library(146)/category(145)/now-playing at desktop + 375px. `webkit-line-clamp: 2` on .track-title confirmed live. Playlist list itself is empty for all accounts ("Nenhuma playlist ainda") — playlist-row path proven via search/history rows (same renderer).
- Category covers: admin "Nova categoria" form has `.upload-photo-drop` + file input; "Gerenciar" form has `#cat-photo-preview` IMG (Cristão cover = artwork ac633317…, 74x96 loaded) + Enviar/Remover foto buttons; client category cards + `.detail-art` (640px) show covers. Photo upload/remove NOT executed (mutation-averse).
- Console pattern: all 7 session errors were browser "Failed to load resource" logs from deliberate negative tests (401/400/500/403) — zero JS exceptions. Treat such fetch-status logs as expected in negative QA.
- SPA modal quirk: an open admin modal (e.g. Novo usuário with "Erro interno" showing) SURVIVES `page.goto()` to the same origin/hash and intercepts pointer events on the page behind it — close modals with Cancelar before navigating, or clicks time out.
- T4 cleanup: 3 test users deleted (204 x3); users list back to seed admin + T3-flagged 11999990001 (2bb42464… still for human cleanup). Player history got a few entries on the seed admin account (benign).
- T3 C4 admin item-render gap: RESOLVED — admin users list + forms verified live this session (report section 9); checkbox remains legitimately ticked.
- QA client bootstrap recipe: `POST /api/store/register {"phone":"<throwaway>"}` (public, no secrets) → 200 + user id + token → `localStorage.setItem('pm_token', token)` → reload. Idempotent per phone; re-calling with `categoryIds:[<id>]` grants categories to the SAME user (used to grant Cristão to the throwaway client). Cleanup after QA: delete client user `2bb424644f4c0f0102d8e2faacf6b9dd` (phone 11999990001).
- LIVE SERVER SERVES HEAD, NOT THE T2 FIX: assets are go:embed-embedded into the binary at build time (web/embed.go) — served player.js is byte-identical to git HEAD (SHA256 fc). Browser QA of the fixed JS is IMPOSSIBLE until the human applies player-wip-fix.diff + rebuilds + restarts (AGENTS.md approval). Live-observed HEAD behaviors of the 6 fix hunks (volume desync 0.8/1.0 at load, togglePlay()-based mediaSession handlers, end-of-queue currentIndex=queue.length out-of-bounds, no readyState guards, pendingSeek always null) are expected — record as NOT-DEPLOYED, not bugs.
- QA evidence pattern: before/after state via `playwright_browser_evaluate` on `window.__player.getState()` + `__player.audio` (14-row table in qa-r1r3.md); re-assert `playwright_browser_console_messages` level=error → 0 after EVERY action; save screenshots with explicit paths under the loop dir (default lands in `.playwright-mcp/`).
- PowerShell mangles `-d '{"phone":...}'` in curl.exe (JSON decode fails server-side, "Requisição inválida") — write the body to a temp file and use `--data-binary "@file"` (verified fix).
- Browser profile retains localStorage across Playwright sessions — a leftover admin token (isAdmin:true, name "admin") was present; QA replaced it with the client token. Human may need to re-login as admin (O5).
- Playwright MCP gotchas: refs (f6eXXXX) go stale after UI re-renders — re-run find/snapshot before click; PWA overlay reappears after every navigation — dismiss via "Agora não" (O3).
- C4 admin item-render NOT verified: client session correctly redirects /#/admin → #/ (app.js:371-378), admin.js loads clean (0 boot errors) — needs human admin creds, .env off-limits; re-check in T4 → RESOLVED in T4 (2026-08-07): admin users list + forms verified live with real creds, checkbox legitimately ticked — see T4 section.
- Observations O1-O5 recorded un-fixed (full text in loop-reports/qa-r1r3.md): O1 .card-play blue rgb(97,141,255)+white = 3.11:1 (passes 3:1 UI-component, fails 4.5:1 normal-text AA); O2 track-card "Tocar" needs explicit main-play press (HEAD behavior, optimistic pending-seek in fix may change perception); O3 PWA overlay; O4 vision not run in-session; O5 leftover admin token.
