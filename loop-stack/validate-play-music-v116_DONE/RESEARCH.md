# Research Log

## Context & Prior Work
Domain: frontend architecture + player code (web/assets/player.js, app.js, api.js, admin.js, pwa.js, sw.js), tests, and codebase patterns.

### The uncommitted WIP diff in web/assets/player.js (+17/-11, 1 file)
Working tree has one modified file: `web/assets/player.js` (everything else untracked are loop files). `git diff` shows 7 logical changes:

1. **Default volume** — `audio.volume = 0.8` set at module init (line 8). Aligns element default with `state.volume = 0.8` (line 31); previously the element defaulted to 1.0 until first `setVolume`.
2. **`playAudio()` catch (lines 114-123)** — the `if (!switching)` guard was REMOVED; on `audio.play()` promise rejection it now unconditionally sets `switching = false`, `playing = false`, mediaSession paused. ⚠️ Risk (also flagged in root `STATE.md`): during a src-swap the previous track's play() promise rejects with `AbortError`; without the guard this can flip the UI to paused mid-swap. The subsequent `play` event usually re-syncs, but there is a desync window.
3. **mediaSession play/pause handlers (lines 134-135)** — now call `playAudio()` / `audio.pause()` directly instead of `togglePlay()`. Reasonable: `togglePlay()` decides on `state.playing` which can be stale; direct calls use the element's real state. Pause path relies on the `pause` event to update UI (safe: `switching` is false then).
4. **`next()` at end of queue (line 227)** — `currentIndex` clamped to `Math.max(0, queue.length - 1)` (was `queue.length`). Old value made the Queue page / `syncPagePlayerState` show no current row; this is a genuine fix.
5. **`prev()` restart (lines 243-247)** — guards `audio.currentTime = 0` with `readyState > 0` and explicitly `set({ progress: 0 })`.
6. **`seek()` (lines 256-286)** — metadata-not-loaded branch now also triggers on `audio.readyState === 0`; the seekable-range last resort additionally requires `readyState > 0`; deferred pending-seek now sets optimistic `progress: Math.max(seconds, 0)`; the main branch guards the `currentTime` write with `readyState > 0`. Consistent with the previous "seek before metadata" fix (commit 4c8755c9).
7. Minor: end-of-queue path also sets `progress: 0` (already present).

`node --check` passes on all 6 JS files (verified this run). The diff is behaviorally unverified — the play-catch change (item 2) is the one to exercise in a live browser.

### web/assets/player.js structure (324 lines)
- Singleton `Audio` element + module-level `state` object (queue, currentIndex, current, playing, progress, duration, volume, shuffle, repeat, fullScreen, pendingSeek). Pub-sub via `subscribe(fn)`/`emit()`.
- State is driven by element events: `play`/`pause` (with `switching` flag suppressing the pause fired by the media-load algorithm during src swaps), `timeupdate` → progress, `loadedmetadata`/`durationchange`/`canplay` → duration + applies `pendingSeek`, `ended` (repeat → restart, else `next()`), `error` (transcode fallback: one retry with `streamUrl(song, true)` → format=mp3; then stop).
- Exports: `resolveDuration` (first finite>0 of candidates), `playContext(songs, index)`, `playSong`, `togglePlay`, `next`, `prev`, `seek`, `seekBy`, `setVolume`, `toggleShuffle`, `toggleRepeat`, `setFullScreen`, `setLiked`, `subscribe`, `getPlayerState`.
- Media Session: metadata + action handlers (play, pause, previoustrack, nexttrack, seekbackward/forward with ±10s default, seekto), `playbackState` sync, `setPositionState` every 1s while playing (clamped position ≤ duration to avoid errors).
- **`window.__player = { getState: () => state, audio }` (line 324)** — debug/test hook the verifier can drive from console/Playwright.
- Queue is **in-memory only**: `endpoints.queue`/`saveQueue` exist in api.js but are unused by the front-end (dead front-end code).

### web/assets/app.js (1623 lines) — how player state connects to UI
- `player.subscribe(refreshPlayerBar)` (line 1574). Bar is **rebuilt only when the structural key changes** (`fullScreen|current.id|playing|shuffle|repeat|liked`); otherwise `updateBarInPlace()` mutates the DOM (progress fill, times, volume) so the seek track/volume slider are never destroyed mid-gesture (lines 1460-1501).
- `syncPagePlayerState()` (lines 1524-1572) moves the queue marker, toggles the `.playing` class on rows and syncs like-hearts across all visible track lists without re-rendering.
- Likes: row `toggleLike` and bar/fullscreen `toggleLikeCurrent` both end by calling `player.setLiked()`; optimistic UI with rollback on API failure (lines 288-301, 1311-1319).
- Seek UI: pointerdown/move/up **delegated on document** (`seekToClientX`, lines 1270-1309); ignores clicks when totalDuration is 0.
- Keyboard shortcuts: arrows (seek ±5s), space (play/pause), vol ±0.1; skipped when focus is in an input (lines 1578-1611).
- Router: hash-based, pages map; `render()` rebuilds shell; admin imported as `await import('./admin.js?v=' + readAppConfig().version)`.

### web/assets/api.js (201 lines) — fetch/state patterns
- `apiFetch`: JWT in `X-ND-Authorization`; token refresh from response header; 401 → clear token + dispatch `pm:unauthorized`; all non-OK → `throw new Error(server msg or HTTP n)`. `api` object wraps get/post/put/del/upload; `endpoints` groups all REST calls.
- `artworkUrl`/`streamUrl`: JWT passed as **`?jwt=` query param** (img/audio cannot send headers; server accepts `jwtauth.TokenFromQuery`). `streamUrl` adds `format=mp3` for non-native formats and when fallback=true.
- `readAppConfig()` parses `window.__APP_CONFIG__` (version, baseURL).

### Cache-busting patterns
- `web/assets/index.html` uses `?v=__ASSET_VERSION__` placeholders on CSS/JS/SW registration; replaced at serve time by `internal/server/static.go:28` from `version.Version`. admin.js additionally busted at import time with `readAppConfig().version`.
- `sw.js`: caches only same-origin GET static assets, **never** `/api/*` or `/auth/*` (audio must hit network); network-first navigations, stale-while-revalidate assets, stale caches purged on activate.

### Existing tests
- Go only: `internal/phone/phone_test.go` (Normalize/Format/Mask) and `internal/stream/stream_test.go` (CDN HS256/MD5 signing, URL encoding, path prefix). **No front-end/JS tests exist anywhere** (no `*.test.js`).
- `go test ./...` was green in today's triage (root STATE.md); all 6 JS files pass `node --check` (re-verified this run).
- Browser-session evidence exists: `.playwright-mcp/` holds console logs + page snapshots from earlier sessions. Latest (2026-08-07T15:37): **no JS runtime errors**; only INFO/VERBOSE noise (`beforeinstallprompt` banner suppressed; admin "Password field is not contained in a form") and HTTP 400/401/500 from API endpoints during admin testing (backend/validation responses, not JS errors).

### Patterns / conventions
- Vanilla ES modules; a local `el()` DOM helper duplicated in app.js and admin.js; inline SVG icon map; optimistic UI + rollback on error; `window.dispatchEvent(new Event('pm:...'))` for cross-module events (`pm:rerender`, `pm:unauthorized`, `pm:playlists-changed`); Portuguese UI strings throughout.

## Existing Tools & Resources
- **`window.__player` hook** (player.js:324) — exposes `getState()` and the live `audio` element; ideal for Playwright-driven verification of play/pause/seek state.
- **Playwright MCP** is wired in (playwright_browser_* tools) and `.playwright-mcp/` shows prior session artifacts (console logs, page snapshots) at `http://localhost:4533` — the app appears to run locally on port 4533.
- No `gh` CLI installed (PR review must use `git fetch`/`git diff` against `origin/dependabot/*` branches or the web UI). 18 dependabot branches exist on origin; root STATE.md lists 6 open PRs (#1,#2,#5,#6,#9,#19) — note several branches reference a `ui/` npm workspace that does NOT exist in this repo (possible stale/incorrect PRs).
- Go tests + `node --check` are the only runnable test tooling.

## Requirements & Constraints

### API Surface, Data Models & Runtime Environment (researcher pass 2 — server-side)

### API endpoints — full route map (internal/server/server.go:59-133)

**Public (no auth):**
- `POST /auth/login` — admin `{username, password}` (username OR e-mail) | client `{phone}` only → `{token, id, username, name, phone, isAdmin}` (handlers_auth.go:20-78; client phone normalized via `internal/phone`)
- `GET /api/store/categories` — public category list (store page, no JWT)
- `POST /api/store/register` — `{phone, categoryIds}` → creates/updates client + grants categories + auto-login token (post-checkout flow, handlers_store.go:33-103; only valid category ids granted)

**JWT required (header `X-ND-Authorization: Bearer` OR `Authorization: Bearer` OR `?jwt=` query):**
- `GET /api/me`, `GET /api/settings`, `GET /api/home`, `GET /api/search`
- `GET /api/categories`, `GET /api/categories/{id}`; `GET /api/albums`, `GET /api/albums/{id}`; `GET /api/artists`, `GET /api/artists/{id}`; `GET /api/songs`, `GET /api/songs/{id}`
- Playlists: `GET/POST /api/playlists`, `GET/PUT/DELETE /api/playlists/{id}`, `POST /api/playlists/{id}/tracks`, `DELETE /api/playlists/{id}/tracks/{entryId}`, `PUT /api/playlists/{id}/tracks` (reorder)
- Liked/history: `GET /api/me/liked`, `PUT/DELETE /api/me/liked/{id}`, `GET /api/me/history`, `POST /api/me/history/{id}`; Queue: `GET/PUT /api/queue`
- `POST /api/store/purchase` — client grants categories to self
- Media (via `<img>`/`<audio>` with `?jwt=`): `GET /api/artwork/{id}`, `GET /api/stream/{id}` (front-end adds `format=mp3` for non-native/fallback)

**Admin (JWT + `isAdmin` claim; non-admin → 403 "Sem permissão"):**
- Users: `GET/POST /api/admin/users`, `PUT/DELETE /api/admin/users/{id}` (handlers_api.go:503-686)
- Categories: `GET/POST /api/admin/categories`, `GET/PUT/DELETE /api/admin/categories/{id}` (handlers_api.go:690+)
- Content: `GET /api/admin/albums`, `/api/admin/artists`, `/api/admin/songs`, `POST /api/admin/songs` (upload); photo upload/delete: `POST/DELETE /api/admin/albums/{id}/photo`, `/api/admin/songs/{id}/photo`, `/api/admin/categories/{id}/photo`
- `POST /api/scan` (async scan trigger, 202)

**Middleware** (server.go:189-220): CORS `*`, `Access-Control-Allow-Headers: Content-Type, X-ND-Authorization, Authorization`, `Access-Control-Expose-Headers: X-ND-Authorization`, OPTIONS → 204, per-request slog log, panic recovery → 500.

### Data models (internal/model/model.go)

- **User**: `id`, `username` (admin only), `email` (admin only), `phone` (client only), `name`, `isAdmin`, `categories []Category`, `createdAt`. Admin = username+email+password; client = phone only (random password, never logs in with it).
- **Song**: `id, title, artist, artistId, album, albumId, year, genre, duration (float64), format, bitrate, sampleRate, trackNumber, discNumber, path, size, playCount, liked, createdAt, updatedAt`; `HasCover` is `json:"-"` (not serialized). UI depends on id/title/artist/artistId/album/albumId/duration/format/liked (model.go:6 comment).
- **Category**: `id, name, songCount, checkoutUrl, songs []Song` — the store/loja unit (paywall).
- **Playlist**: `id, name, comment, owner, songCount, duration, songs []PlaylistEntry`; `PlaylistEntry = {entryId, song}`; `UserID` is `json:"-"`.
- **Home**: `sections []{title, songs, albums}` (category → songs model), `genres []{name, songCount}`.

### Auth flow (internal/auth/auth.go)

- JWT HS256, TTL 24h (`tokenTTL`), signing secret from DB (`store.GetOrCreateSecret`), admin bootstrap from `ND_ADMINUSERNAME`/`ND_ADMINPASSWORD` only when no admin row exists (auth.go:55-81).
- Token extraction order: `X-ND-Authorization` header (with/without `Bearer `) → `Authorization` header → `?jwt=` query (auth.go:170-177). Query param exists for `<img>`/`<audio>` tags that cannot send headers.
- Refresh: past half TTL → response header `X-ND-Authorization` carries fresh token; web UI reads it (api.js apiFetch).
- Claims: `uid, username, phone, name, isAdmin` + reg claims. `LoginUsername` rejects non-admin users; `LoginPhone` rejects admin phones.

### Runtime state (srv.log / srv-err.log / port probe — verified this run)

- **Server IS running**: PID 31476 listening on `:4533` (all interfaces `::`), `GET /` → HTTP 200.
- `srv-err.log`: **empty** — zero errors.
- `srv.log`: all request logs 200; last traffic 12:49 (admin categories/users 200s from prior QA); scheduled scan 13:40:25 → `scan finished added=0 updated=0 skipped=146 deleted=0 duration=1.748s` (library healthy, 146 songs). No errors, no panics.

### Constraints — what must NOT change / rules

- **Denylisted paths** (root AGENTS.md): never edit `.env`, `.env.*`, `auth/`, `payments/`, `secrets/`, `credentials/` — the *credential-bearing* paths are protected; the Go package `internal/auth/` is application code (JWT logic, part of the domain) and was only read, not modified. Never log or print JWT tokens, admin passwords, or `DATABASE_URL`.
- **AGENTS.md rules**: L1 report-only until human enables L2; no source edits during research; code changes (L2+) only via git worktree; max 3 fix attempts per item then escalate; dispatch verifier sub-agent after any L2+ change.
- **No gate.yaml exists** (glob `**/gate*.y*ml` → none, also none in loop-stack/).
- **Report-only loop** (PLAN.md): no auto-commit, no auto-merge; PR merges and fixes are human-decided.
- **Postgres-only** — no SQLite anywhere; docker-compose has only the app service (Postgres/MinIO/Redis external).
- **QA safety traps**: (1) `DELETE /api/admin/users/{id}` has **no last-admin guard** — only the self-delete guard (handlers_api.go:675-686) and the demote guard (handlers_api.go:616-626); deleting the last admin would lock the app out (bootstrap only re-runs when no admin exists AND env vars set). QA must not delete/demote the only admin. (2) Client login is phone-only — a QA client must first be created via admin panel or `/api/store/register`. (3) Prefer read-only API checks in the prod DB; only perform CRUD tests (create/update/delete category or client user) if the plan explicitly sanctions it.

### QA session endpoints checklist (browser + API)

| Flow | Endpoint | Auth |
|---|---|---|
| Admin login (user or email) | `POST /auth/login` `{username,password}` | none |
| Client login (phone) | `POST /auth/login` `{phone}` | none |
| Home render | `GET /api/home` | JWT header |
| Search | `GET /api/search?q=` | JWT header |
| Stream (player) | `GET /api/stream/{id}?jwt=` (+`format=mp3` fallback) | `?jwt=` |
| Artwork | `GET /api/artwork/{id}?jwt=` | `?jwt=` |
| Admin categories CRUD | `GET/POST/PUT/DELETE /api/admin/categories[...]` | admin JWT |
| Admin users CRUD | `GET/POST/PUT/DELETE /api/admin/users[...]` | admin JWT |
| Store public | `GET /api/store/categories`, `POST /api/store/register` | none |
| Me/settings | `GET /api/me`, `GET /api/settings` | JWT header |

### Front-end-focused constraints (prior pass)
- Must preserve backend compatibility (Go `go 1.26`, JWT via header/query, endpoints per api.js).
- R1: player controls (play/pause/seek/volume/progress) must not throw uncaught exceptions; state sync between UI, element and mediaSession must be consistent.
- R3: browser console free of JS errors (VERBOSE DOM hints like the admin password-field warning are tolerable but noted; HTTP error statuses are API-side).
- v1.16.0 features (admin users vs clients, login by user OR email, last-admin guard) must not regress.

## Suggested Approach
Validate the WIP with a live-browser pass driving `window.__player` + the UI (play, pause, seek before/after metadata, prev-restart, next at queue end, mediaSession handlers, volume default 0.8, transcode fallback), then `git diff` a proposed minimal fix if the play-catch desync reproduces. Acceptance criteria R1-R3 verified from the same browser session (console errors captured, loja/admin rendered, responsive <768px layout).

## Verification Criteria
- Pass: all player controls work with no uncaught exceptions; `state.playing`/`state.progress` match `audio` element reality after each action (play, pause, seek to 0:10, prev within first 3s vs after, next past last track with repeat off/on, media keys); mediaSession.playbackState reflects element state; queue page shows last track as current after next-at-end; console free of JS errors; `node --check` on all 6 JS files.
- Fail: play() rejection mid-src-swap leaves UI paused while audio plays (the flagged desync), progress > duration after clamp, currentIndex out of range in queue page, unhandled promise rejections.

## Quality Standards
- Keep the existing event-driven design (state as projection of element events) — do not bolt on timers or polling for state beyond the existing 1s mediaSession position sync.
- Follow codebase conventions: Portuguese UI strings, `el()` helper, optimistic updates, `pm:` custom events, `?v=` cache-busting when new static assets are introduced.
- Anti-patterns: re-rendering the bottom bar on progress ticks (breaks seek drag — explicitly fixed in commit 4c8755c9), swallowing errors silently without UI feedback, adding JS frameworks, touching the SW's `/api` interception rules.

## Prior Attempt Analysis
- No prior loop attempts on this task (STATUS.md: planning in progress, 0 attempts). Root STATE.md triage noted the play-catch desync risk in the WIP and recommended manual player QA — same risk surfaced here in the diff analysis; treat it as the primary validation target.
- 18 dependabot branches on origin reference `ui/` npm deps that don't exist locally — verify before merging any PR (mismatched workspace).

## External Knowledge & Resources
Domain: goal artifacts (ORIGINAL_REQUEST.md), run/setup docs (README.md, docker-compose.yml, Dockerfile), and the 6 open Dependabot PRs (#1 #2 #5 #6 #9 #19) on https://github.com/hensi01/play-music. Sources: root files read directly; PR data from GitHub REST API + PR .diff URLs via WebFetch; staleness confirmed with local git (`merge-tree --write-tree origin/master <branch>`).

### ORIGINAL_REQUEST.md — requirements (verbatim) and acceptance criteria (verbatim checkboxes)

Requirements:
- **R1** (web/assets/app.js, player.js, admin.js, api.js, pwa.js, sw.js): fix runtime exceptions, player state-sync failures, API/request errors, SW registration issues.
- **R2** (web/assets/style.css, index.html, loja.html): modern visual redesign — harmonious dark/gradient palette, smooth transitions, fluid typography, responsive nav mobile+desktop.
- **R3**: keep Go-backend compatibility; browser console free of syntax errors / missing-resource errors on page load.

Acceptance criteria (6 checkboxes, all unchecked in source):
1. `[ ]` All JS in `web/assets/*.js` valid syntax, no compile/parse errors.
2. `[ ]` HTML files (`index.html`, `loja.html`) have valid references for all CSS/JS/image deps.
3. `[ ]` Player controls (play, pause, seek, volume, progress) work without uncaught JS exceptions.
4. `[ ]` Loja (`loja.html`) catalog and Admin panel (`admin.js`) render items correctly with clear visual feedback.
5. `[ ]` `web/assets/style.css` uses responsive layout (Flexbox/Grid) for mobile (<768px) and desktop, no text overlap or broken elements.
6. `[ ]` Theme/components use smooth hover transitions, adequate contrast, refined UI elements.

### README.md — how to run the server
- Requirements: Go 1.26+ (build) or Docker; **PostgreSQL only (no SQLite)**; S3/MinIO bucket; ffmpeg.
- Config: all env (`ND_*` prefix + `DATABASE_URL`): `ND_ADMINUSERNAME/ND_ADMINPASSWORD`, `DATABASE_URL`, `ND_MUSICFOLDER` (s3:// URL or ND_S3_*/MINIO_*), `ND_CDN_*` (Bunny), `ND_REDIS_ENABLED/ND_REDIS_URL` (optional artwork cache), `ND_SCANNER_SCHEDULE`, `ND_FFMPEGPATH`, `ND_PORT/ND_ADDRESS/ND_LOGLEVEL`, `ND_TRANSCODINGCACHESIZE/ND_IMAGECACHESIZE`.
- Run: local `go build -o play-music . && ./play-music`; docker `docker compose up -d --build`. First boot runs migrations + initial bucket scan. Listens on `ND_ADDRESS:ND_PORT` (**default 0.0.0.0:4533**), serving UI + API together.
- **No `.env.example` exists** (confirmed via glob; env vars are documented only in the README table).
- Auth: `POST /auth/login` (admin `{username,password}` | client `{phone}` only) → `{token,id,name,phone,isAdmin}`; JWT via `X-ND-Authorization: Bearer` or `?jwt=` for `<img>/<audio>`.

### Service topology (docker-compose.yml + Dockerfile)
- docker-compose.yml: **single service** `play-music` (build: ., env_file: .env, `4533:4533`, restart unless-stopped, volume `playmusic-var:/app/var`). **Postgres, MinIO, Redis are NOT compose services** — they are external infra configured via env vars (README: MinIO/S3 bucket + optional Redis). Do not expect a `docker compose up` to bring up a full stack.
- Dockerfile: `golang:1.26-alpine` build stage (CGO_ENABLED=0, `-ldflags="-s -w"`) → `alpine:3.21` runtime with ffmpeg+ca-certificates+tzdata; ENV `ND_PORT=4533 ND_ADDRESS=0.0.0.0`; EXPOSE 4533.

### Dependabot PRs — ALL 6 OPEN PRs ARE STALE / UNMERGABLE (major finding)
Evidence: every PR branch is diverged behind master (does not contain v1.16.0 commit `8df05211`); `git merge-tree --write-tree origin/master <branch>` exits **1 (conflict) for all 6**; the target dep/file no longer exists on master:

| PR | Dependency bump | Touches | Status vs master |
| --- | --- | --- | --- |
| #1 | crazy-max/osxcross 14.5-debian → 26.1-debian (docker) | root `Dockerfile` (+1/-1) | osxcross build stage removed from fork Dockerfile — stale |
| #2 | pressly/goose/v3 3.27.2 → 3.27.3 | go.mod/go.sum (+18/-18) | goose not in master go.mod — stale |
| #5 | lestrrat-go/jwx/v3 3.1.1 → 3.2.0 | go.mod/go.sum (+3/-3) | jwx not in master go.mod — stale |
| #6 | mattn/go-sqlite3 1.14.48 → 1.14.49 | go.mod/go.sum (+3/-3) | sqlite3 not in master go.mod nor imported in any .go — stale (README: Postgres only) |
| #9 | actions/stale 10 → 11 (GitHub Actions) | `.github/workflows/stale.yml` (+1/-1) | **`.github/` does not exist on master** (no workflows at all) — stale |
| #19 | gohugoio/hashstructure 0.6.0 → 1.0.0 | go.mod/go.sum | hashstructure not in master go.mod, not imported — stale |

Context: master `go.mod` has only 7 direct deps (dhowden/tag, golang-jwt/jwt/v5, jackc/pgx/v5, minio-go/v7, go-redis/v9, robfig/cron/v3, golang.org/x/image); local working-tree go.mod == origin/master go.mod. PRs were created 2026-08-02 against the pre-slimming (Navidrome-lineage) go.mod. Created_at 2026-08-02, updated 2026-08-05, all 1 commit, no review comments, `mergeable_state: unknown` on the API (GitHub hasn't computed merges; local merge-tree proves conflicts).

### Existing Tools & Resources
- GitHub REST API via WebFetch (unauthenticated; repo is public) — `pulls?state=open`, `/pulls/{n}`, `{n}.diff` all work; used for PR metadata above.
- Local git: all 6 PR branches already fetched at `remotes/origin/dependabot/...`; `git merge-tree --write-tree` used for conflict detection (no working-tree mutation, safe in report-only mode).

### Requirements & Constraints
- Report-only loop: no source edits/commits (PLAN.md/AGENTS.md); PR merges are human-decided.
- Acceptance criteria R1-R3 (6 checkboxes above) are the verifier's contract; verified against localhost:4533.
- Postgres-only; no SQLite — any PR re-adding sqlite deps (e.g. #6) is unwanted by design.

### Suggested Approach
Do not merge or rebase any of the 6 PRs — close all (deps and files they touch no longer exist on master; `@dependabot recreate`/`rebase` will keep failing against the slim go.mod). Optionally run `go mod tidy` + `go build` to confirm the slim go.mod is complete before closing, so no removed dep is silently needed. Focus loop effort on R1-R3 validation + player.js WIP, not on dependency work.

### Verification Criteria
- Pass: all 6 PRs confirmed stale (merge-tree exit 1; target dep/file absent from `origin/master` @ `8df05211`); `go build`/`go test ./...` green on master without goose/jwx/sqlite3/hashstructure; no open PR is dependency-required for the v1.16.0 features.
- Fail: any PR merges cleanly (reassess it), or master's go.mod actually contains one of the removed deps (checklist error), or closing a PR would remove a dependency still imported by `.go` files (verified: no imports found).

### Quality Standards
- Ground every PR claim in origin/master state (current head `8df05211`, v1.16.0) — the repo was slimmed since the PRs were created; never judge a Dependabot PR by its title alone.
- Record merge-evidence (merge-tree exit codes, missing files/deps) in the report so the human can close PRs with confidence; keep the R1-R3 acceptance criteria verbatim as the verifier contract.

### Prior Attempt Analysis
- Previous researcher flagged "verify before merging any PR (mismatched workspace)" only for the npm `ui/` branches; this research extends it: **all 6 open PRs** (Go + Docker + Actions) are stale, not just the npm ones. The 18 dependabot branches on origin include 12 more branches without open PRs (npm/ui + prometheus) — all reference removed `ui/` workspace or removed deps; no mergeable dependency work exists in this repo.

## Environment & Integration
Domain: build pipeline and dependency health — go.mod/go.sum, build/test evidence, Docker, CI/CD, web asset pipeline, version stamping, Node tooling, live server reachability. All evidence captured today (2026-08-07) against working tree at commit 8df05211 (v1.16.0).

### Context & Prior Work
- go.mod has 7 direct deps: dhowden/tag v0.0.0-20240417053706-3d75831295e8, golang-jwt/jwt/v5 v5.2.1, jackc/pgx/v5 v5.7.2, minio-go/v7 v7.0.83, go-redis/v9 v9.7.0, robfig/cron/v3 v3.0.1, golang.org/x/image v0.23.0 (go.mod:5-13). Indirect block includes x/crypto v0.31.0, x/net v0.33.0, x/sync v0.10.0, x/sys v0.28.0, x/text v0.21.0. `go 1.26` line present; **no `toolchain` directive** (fine — go.mod is clean and current).
- Prior researcher (this loop) already mapped Docker/service topology (RESEARCH.md lines 177-179) and the 6 stale Dependabot PRs. This research extends with first-hand build/test/verify evidence.

### Existing Tools & Resources
- Local Go toolchain (go.mod says 1.26; `go mod verify` and `go test` run locally, so Go 1.26 is installed).
- `go mod tidy -diff` (Go 1.26 built-in dry-run) — used to answer "what would tidy change" without touching go.mod.

### Requirements & Constraints
- go.mod/go.sum must stay consistent: `go mod verify` must pass; `go mod tidy -diff` should ideally be empty (currently shows ONE cosmetic change, see below — decide whether to apply it; no functional impact).
- Docker build must produce a single static binary with embedded web/ assets; runtime image must keep ffmpeg (streaming/transcoding) + ca-certificates + tzdata.
- No CI/CD exists on master: `.github/` confirmed ABSENT (Test-Path False). Dependabot PR #9 (actions/stale 10→11) is stale precisely because no workflows exist.
- No Node tooling on master: package.json, ui/, node_modules all absent — nothing on master needs npm; the `ui/` npm workspace lives only on 18 stale dependabot branches.
- Version stamping is a hardcoded const, not ldflags: bumping v1.16.0 requires editing internal/version/version.go and rebuilding the image.

### Build & Test Evidence (run 2026-08-07)
- `go mod verify` → **PASS**: "all modules verified". go.sum is present and consistent.
- `go mod tidy -diff` → **1-line delta**: would move `golang.org/x/crypto v0.31.0` from the `// indirect` block into the direct require block (go.mod:29). No version bumps, no go.sum changes, no removals. The slimmed go.mod (goose/jwx/sqlite3/hashstructure removed) is otherwise tidy-clean, confirming the 6 Dependabot PRs target deps that are genuinely gone.
- `go build ./...` → **PASS** (zero output, exit 0; root module play-music + 14 internal packages).
- `go test ./...` → **PASS**: 15 packages. Only `internal/phone` (ok, cached) and `internal/stream` (ok, cached) contain tests; the other 13 report "[no test files]". **Zero test failures; no test coverage elsewhere.**

### Docker & Compose
- Dockerfile (16 lines): 2 stages — build: golang:1.26-alpine, WORKDIR /src, `go mod download` cached layer, CGO_ENABLED=0, `-trimpath -ldflags="-s -w"` → /out/play-music; runtime: alpine:3.21 + `apk add ffmpeg ca-certificates tzdata`, COPY binary, ENV ND_PORT=4533 ND_ADDRESS=0.0.0.0, EXPOSE 4533, CMD play-music. Complete for the app; nothing missing. NOTE: no `-X main/...` ldflags — version comes from the const, so the image always reports what's baked into the source.
- docker-compose.yml (12 lines): single service play-music (build: ., env_file: .env, 4533:4533, restart: unless-stopped, named volume playmusic-var:/app/var). Postgres/MinIO/Redis are external infra (no compose services) — `docker compose up` alone won't boot a stack; `.env` must exist before compose will even parse (env_file is mandatory). No `.env.example` in repo.

### Web Asset Pipeline & Versioning
- Assets embedded via `//go:embed assets` in web/embed.go; served by internal/server/static.go handleStatic():
  - `__ASSET_VERSION__` placeholder replaced at boot with `version.Version` via bytes.ReplaceAll on index.html (static.go:28).
  - 5 usages in index.html: style.css?v=, `version: '__ASSET_VERSION__'` JS constant (line 18), app.js?v=, pwa.js?v=, sw.js registration?v= — so the service worker itself is cache-busted (matches commit history "fix: import do admin.js com ?v para cache-busting do service worker").
  - Every static response: `Cache-Control: no-cache`; unknown routes get SPA fallback to index.html (static.go:39-49). Requires a read from disk/embed per request (fs.Stat) — fine at this scale.
- internal/version/version.go: `const Version = "1.16.0"` — hardcoded, reported via /api/settings and used for ?v=. **Version bump = source edit + rebuild**; current value matches HEAD commit "feat: ... versao 1.16.0" (8df05211). Commits e1ea6cc5 (v1.14.0), cb26fb1b (v1.15.0), 8df05211 (v1.16.0) confirm the bump-per-release convention.
- Working tree state: `M web/assets/player.js` (the flagged WIP from PLAN.md) + untracked loop files (.opencode/, AGENTS.md, LOOP.md, ORIGINAL_REQUEST.md) — no other drift vs HEAD.

### CI/CD
- `.github/` does NOT exist on master (Test-Path False). No workflows, no stale action, no CI of any kind. Build/test correctness is enforced only locally/Docker. Dependabot PR #9 (actions/stale) is dead-on-arrival — closing it (and #1 #2 #5 #6 #19) is safe per prior research.

### Live Server (loop runtime env)
- `GET http://localhost:4533/` → **HTTP 200** (Invoke-WebRequest, 5s timeout). Server is up on the default port; prior researcher confirmed PID 31476, 146 songs, srv-err.log empty. UI + API reachable; verifier can run R1-R3 checks against localhost:4533 right now.

### Suggested Approach
No code changes needed for any of this: go.mod is verified-consistent (one cosmetic tidy delta — promote x/crypto to direct, or leave; no functional difference), build/tests green, Dockerfile/compose complete, server up. Before closing the Dependabot PRs, run `go mod tidy` + `go build ./...` as a final sanity check (prior researcher already recommended this) so no removed dep is silently needed — this research already proves tidy wants nothing new. Bump version const + ?v= flows together whenever a new release lands.

### Verification Criteria
- Pass: `go mod verify` prints "all modules verified"; `go build ./...` exits 0; `go test ./...` shows only `ok`/`[no test files]`; `go mod tidy -diff` output is empty or limited to the x/crypto direct/indirect promotion; http://localhost:4533/ returns 200; `.github/`, `package.json`, `ui/` absent; `__ASSET_VERSION__` resolves to `1.16.0` in served HTML.
- Fail: any package fails to build/test; tidy would add/remove/upgrade a dependency (would mean the slimmed go.mod is incomplete); version const differs from release tag; served HTML still contains the raw `__ASSET_VERSION__` token.

### Quality Standards
- Ground every claim in live command output (verified today), not assumptions — build/test/tidy evidence above is first-hand.
- Keep research-only discipline: no go.mod rewrite performed; the x/crypto promotion is reported, not applied.

### Prior Attempt Analysis
- No prior loop attempts on this domain (planning in progress). Prior researcher's "go build/go test green" assertion is now confirmed with concrete output; the "tidy will be clean" recommendation is confirmed to hold modulo the single cosmetic promotion.

## Task-Specific Research — [G1] T1 — Dependabot PR review

## Context
- All 6 PR branches exist on origin (verified `git branch -r`): #1 docker/crazy-max/osxcross, #2 pressly/goose/v3, #5 lestrrat-go/jwx/v3, #6 mattn/go-sqlite3, #9 github_actions stale (/.github), #19 gohugoio/hashstructure.
- GitHub API (api.github.com/repos/hensi01/play-music/pulls/<n>): all 6 state=open, base=master, titles = "chore(deps): bump <dep> from <old> to <new>".
- origin/master = 8df0521134829be7318ff409da5e8c6b1b821772 (feat v1.16.0). Master go.mod is the SLIMMED version: only dhowden/tag, golang-jwt/jwt/v5, jackc/pgx/v5, minio-go/v7, redis/go-redis/v9, robfig/cron/v3, x/image + indirects. goose/jwx/go-sqlite3/hashstructure are ALL ABSENT.
- Root cause of staleness: commit b2e353f8 "hensi" (2026-08-05 23:37, ancestor of master, verified `git merge-base --is-ancestor` exit 0) removed the 4 go deps from go.mod. Dockerfile was rewritten to a slim 2-stage alpine build — osxcross stage gone. `.github/` directory deleted from master entirely.
- Merge-bases: go-module PRs (#2 #5 #6 #19) all based on 54103a00 (2026-08-05, pre-slimming); #1 and #9 based on af5e6277 (2026-08-01). All branches = 1 single-commit bump. 16 commits on master since b2e353f8.

## git merge-tree --write-tree origin/master <branch> evidence (all exit=1 = conflict)
| PR | Branch | Conflict files | Conflict type | Target in master? |
|---|---|---|---|---|
| #1 | osxcross-26.1-debian | Dockerfile | content (osxcross stage removed) | NO (Dockerfile has no osxcross, no --platform/xx) |
| #2 | goose/v3-3.27.3 | go.mod, go.sum | content (goose block removed) | NO (absent from go.mod) |
| #5 | jwx/v3-3.2.0 | go.mod, go.sum | content (jwx block removed) | NO (absent from go.mod) |
| #6 | go-sqlite3-1.14.49 | go.mod, go.sum | content (go-sqlite3 block removed) | NO (absent from go.mod) |
| #9 | stale-11 | .github/workflows/stale.yml | modify/delete (deleted in master) | NO (.github/ absent, Test-Path False) |
| #19 | hashstructure-1.0.0 | go.mod, go.sum | content (hashstructure block removed) | NO (absent from go.mod) |

Note: merge-tree conflict outputs confirm the branch diffs reference the OLD fat go.mod (context lines show pocketbase/dbx, go-chi/jwtauth, gomega, etc.) — branches predate the Postgres/rebrand migration.

## Existing Tools & Resources
- Git 2.55.0 built-ins (no gh CLI): `git merge-tree --write-tree`, `git ls-remote`, `git merge-base --is-ancestor`, `git branch -r` — all verified working.
- GitHub REST API via `curl.exe` (repo `hensi01/play-music`, full_name verified from API) — no auth needed for public metadata.
- Prior researcher run (STATUS.md): go mod verify PASS, go build ./... clean, go test ./... PASS (15 pkgs), tidy -diff = 1 cosmetic delta only (x/crypto indirect→direct). Commands documented in loop TOOLS.md.

## Requirements & Constraints
- Report-only: NO source changes, no commits, no pushes. Deliverable: loop-reports/dependabot-review.md (per-PR verdict + go.mod evidence table + close-recommendation summary). loop-reports/ does NOT exist yet (Test-Path False) — executor must create it at project root.
- Verdict criteria (per PLAN.md): target dep/file absent from master go.mod/Dockerfile/.github → CLOSE; dep still present + conflict → REBASE; dep present + clean → rebase-and-merge. Evidence per PR must come from merge-tree exit codes + presence checks.
- Re-verify briefly after analysis: `go mod verify` (expect "all modules verified"), `go mod tidy -diff` (expect ≤1 cosmetic delta), `go build ./...` (exit 0), `go test ./...` (PASS).

## Suggested Approach
For each PR: (1) record GitHub title/base/state; (2) run merge-tree --write-tree origin/master <branch>, record exit + conflict files; (3) verify target dep/file presence in master (go.mod / Dockerfile / .github); (4) apply criteria → verdict. All 6 are expected CLOSE since every target was removed from master. Write loop-reports/dependabot-review.md with the evidence table + summary; no code changes.

## Verification Criteria
- PASS: loop-reports/dependabot-review.md exists at project root, covers all 6 PRs, each with merge-tree exit code + conflict-file evidence + presence check, verdict consistent with criteria (all 6 = close), go.mod table lists the 7 remaining master requires + 4 absent deps, re-verify commands output recorded (verify/tidy-diff/build/test).
- FAIL: any PR verdict "rebase" without evidence its dep still exists on master; report missing a PR; evidence not reproducible (commands missing from report).

## Quality Standards
- Evidence-first: every verdict cites its merge-tree exit code and the presence check (e.g. "goose absent from master go.mod — no go.sum lines").
- State the root cause once (b2e353f8 slimming + Dockerfile rewrite + .github deletion) and reference it per-PR instead of repeating.
- Keep the report human-actionable: a final close-all-6 recommendation summary with the exact `gh pr close <n>` command the human can run (gh CLI absent locally — note that).
- Anti-pattern: recommending "rebase" for a dep that no longer exists (would resurrect dead deps in the slimmed go.mod).

## Prior Attempt Analysis
None for this task (0 attempts). Prior researcher verified build/test green and confirmed .github/ absent — consistent with all-close expectation.

## Task-Specific Research — [G1] T2 — player.js WIP fix

## Context & Prior Work
Full player.js (324 lines, web/assets/player.js) read; state model: module-level `state` object + `set(patch)` (Object.assign + emit) + `subscribe(fn)`/`getPlayerState()`; `window.__player = { getState: () => state, audio }` (line 324) exists for T3 QA. `switching` lifecycle: SET true only in `loadAndPlay()` (line 182, right before `audio.src = ...`); CLEARED by the `play` event (75), the `pause` event when it fires mid-swap (83), the `error` handler (99), and end-of-queue stop in `next()` (226). WIP added a 4th clear inside the playAudio() catch (118). `playAudio()` (114-123) is called from: `loadAndPlay` (187), `ended`→repeat restart (92), `error` fallback retry (104), `togglePlay` (209), and mediaSession `play` handler (134). `togglePlay` is still used by app.js (1241, 1381, 1608) — mediaSession no longer uses it. app.js consumes `currentIndex` only for row highlight (856-868, `i === currentIndex`) and render keys (1526); it never indexes `queue[currentIndex]` directly, so the old out-of-bounds value was a highlight bug, not a crash.

## Per-Hunk Verdicts (WIP diff, 7 hunks)
1. **volume 0.8 (line 8)** — ENHANCEMENT, KEEP. Aligns element default with `state.volume = 0.8`; before, the element was 1.0 until first setVolume. Low risk.
2. **playAudio() catch guard removal (117-121)** — BUG, FIX. Original: `if (!switching) { set({playing:false}); setMediaSessionPlaybackState(false) }`. WIP: `switching = false; set({playing:false}); setMediaSessionPlaybackState(false)` unconditionally. Failure mode: src-swap (rapid prev/next/shuffle) — old play() promise rejects with AbortError while switching=true → WIP flips UI to paused mid-load, and worse, clears `switching` so the load-algorithm `pause` event is no longer consumed (pause handler checks the flag). If the rejection lands after the new track's `play` event (racy, common on fast swaps), `playing=false` sticks while audio actually plays — the live desync. The extra `switching = false` is wrong: it disarms pause-consumption before the load algorithm's pause fires. Original guard + no flag clear is self-consistent: rejection during swap does nothing; the swap's own pause/play events settle state; non-swap rejection (autoplay policy) still sets playing=false. RECOMMENDED FIX (minimal, codebase-style — file uses explanatory comments everywhere):
   p.catch(() => { if (!switching) { set({ playing: false }); setMediaSessionPlaybackState(false) } }) — restore guard, drop `switching = false`, add a short comment so the fix is visible in the final diff (a pure revert would make hunk 2 invisible in player-wip-fix.diff vs HEAD). Callers unaffected: mediaSession 'play' during a swap → rejection → guard skips → swap's play event syncs; non-swap lockscreen press → guard passes → honest paused. `togglePlay` reads accurate state.playing.
3. **mediaSession 'play'→playAudio() / 'pause'→audio.pause() (134-135)** — BUGFIX/ENHANCEMENT, KEEP. Unambiguous semantics vs the old double-toggle; togglePlay() decided on possibly-stale `state.playing` (the very desync this task fixes). Pause path relies on the `pause` event to update UI — safe since `switching` is false on user pause.
4. **end-of-queue `currentIndex: Math.max(0, queue.length-1)` (227)** — BUGFIX, KEEP. Old `queue.length` (out of bounds) left the queue page with no highlighted row; clamp matches `state.current` (unchanged last song). `queue.length === 0` early-return makes Math.max a harmless belt-and-suspenders. Also sets `progress: 0` + `audio.pause()` — correct stop semantics.
5. **prev() restart guard (243-247)** — DEFENSIVE ENHANCEMENT, KEEP. `readyState > 0` before `audio.currentTime = 0`; explicit `set({progress: 0})` covers the readyState===0 case where no timeupdate will fire. No regression: currentTime write only skipped when nothing is loaded.
6. **seek() metadata branch + readyState guards (259-285)** — MOSTLY SAFE, KEEP with one edge noted. `duration === 0 || readyState === 0` routes pre-load seeks to deferral (pendingSeek) or seekable last resort; seekable now also requires `readyState > 0`. Main branch (duration>0 but element not loaded — true during src swap with known song.duration): WIP skips `audio.currentTime = target` and does NOT set pendingSeek, so the seek is silently dropped (progress shows target briefly, then real playback position wins). Original implicitly deferred via default-playback-start-position. Pre-metadata window is tiny (preload='auto'); NOT a live risk — do not expand this task. Optional future hardening: set `state.pendingSeek` there too.
7. **optimistic pending-seek progress (277)** — ENHANCEMENT, KEEP. `set({progress: Math.max(seconds,0)})` in the defer branch makes the seekbar respond immediately; updateDuration re-applies the real seek on loadedmetadata (69).

## Requirements & Constraints
- CODE CHANGE restricted to new git worktree `../play-music-loop-wt` (AGENTS.md); main tree keeps ONLY the pre-existing `M web/assets/player.js` WIP. No commit/push. Deliverables: `loop-reports/wip.patch` (captured BEFORE any fix), `loop-reports/player-wip-fix.diff` (worktree diff vs HEAD = applied WIP + guard fix), `loop-reports/player-wip-fix.md` (changes + rationale + node --check result). loop-reports/ must be created (does not exist, confirmed T1).
- Fix = restore `if (!switching)` guard in playAudio() catch; do NOT touch the other 6 hunks (all validated safe above).
- Max 3 fix attempts, then escalate. Budget: node --check must pass in the worktree.

## Suggested Approach
Verified executable sequence (all tested this run, Windows PS 5.1):
1. Main tree: `New-Item -ItemType Directory -Force loop-reports`
2. Capture WIP: `cmd /c 'git diff web/assets/player.js > loop-reports\wip.patch'` — **PS `>` writes UTF-16LE (verified: bytes 255,254...) and breaks git apply; cmd /c redirect yields clean LF (verified: first bytes 100,105,102,102 = "diff")**.
3. Worktree: `git worktree add --detach ../play-music-loop-wt master` — **`git worktree add ... master` FAILS (verified exit 128: "fatal: 'master' is already used by worktree at ...") because master is checked out in the main tree. --detach verified working (exit 0)**.
4. In worktree (workdir=play-music-loop-wt): `cmd /c 'git apply ..\play-music\loop-reports\wip.patch'` — LF patch onto CRLF working copy (core.autocrlf=true global) applies cleanly (verified exit 0). NOTE the task brief's `../loop-reports/` is WRONG from the worktree (resolves to gits\loop-reports); must be `..\play-music\loop-reports\`.
5. Edit catch in worktree player.js (restore guard + comment, drop `switching = false`).
6. `node --check web/assets/player.js` (workdir=worktree) — verified working from any dir, exit 0 on current WIP file; Node 24 auto-detects ESM syntax (no package.json needed).
7. `cmd /c 'git diff web/assets/player.js > ..\play-music\loop-reports\player-wip-fix.diff'`
8. Write `..\play-music\loop-reports\player-wip-fix.md`. Optionally `git worktree remove ../play-music-loop-wt` when done (verify with `git worktree list` first) — or leave for T3 live QA.
PS quirk: git's stderr messages render as red "git :" errors in PS — check ``, not the red text.

## Verification Criteria
PASS:
- `node --check web/assets/player.js` in worktree → exit 0.
- Worktree player.js catch (playAudio): contains `if (!switching) {` and does NOT contain `switching = false` inside the catch; `set({ playing: false })` + `setMediaSessionPlaybackState(false)` nested inside the guard.
- Trace criterion: switching=true (src swap) + play() rejection (AbortError) → catch takes no action, playing stays true, load-algorithm pause consumed by pause handler, subsequent `play` event syncs playing=true. switching=false + rejection (autoplay policy) → playing=false set (guard passes).
- mediaSession wiring: `'play'` → `playAudio()`, `'pause'` → `audio.pause()`; `readyState` guards present in prev() and both seek() branches; `currentIndex: Math.max(0, queue.length - 1)` at end of queue; `audio.volume = 0.8` present.
- loop-reports/wip.patch + player-wip-fix.diff exist, first bytes are LF diff ("diff"), no UTF-16 BOM; player-wip-fix.md documents the fix + node --check result.
- Main tree: `git status --porcelain` shows only the pre-existing `M web/assets/player.js` + untracked loop dirs; nothing else modified.
FAIL:
- catch lacks the guard, or contains `switching = false`; any of the 6 other hunks altered/reverted; node --check nonzero; patch files UTF-16 (BOM bytes 255,254); worktree created non-detached or command produced "already used by worktree" error; any edit outside the worktree.

## Quality Standards
- Minimal diff: exactly one behavior line restored (guard) + a comment; nothing else changes. Follow the file's comment style (short prose above the changed block, like the existing `// Wraps audio.play()...` notes).
- Deliverables human-reviewable: the .md must explain the desync mechanism (AbortError during src-swap; WIP's `switching=false` disarms pause-consumption) and why the guard is the safe choice, plus the node --check evidence line.
- Anti-patterns: rewriting the catch to special-case error names (NotAllowedError vs AbortError) — out of scope, non-minimal; changing any of the 6 validated hunks "while we're at it"; leaving the worktree registered after the task without removing it (or without a note if kept for T3).

## Prior Attempt Analysis
None for this task (0 attempts). The WIP diff was previously flagged in this file ("Context & Prior Work" item 2) as the risk to exercise live — this research confirms the mechanism and prescribes the minimal revert. `STATE.md` (project root, untracked) also flagged the catch change; consistent.

## Task-Specific Research — [G2] T3 — R1-R3 browser QA

## Context & Prior Work
- T2 delivered `loop-reports/player-wip-fix.diff` (+18/-7) = applied WIP + restored `if (!switching)` guard in `playAudio()` catch (player.js:117-123 fixed) + 4-line AbortError comment. Main tree player.js is UNCHANGED since T2 (still the WIP, +17/-11 vs HEAD). **The fixed player.js exists ONLY in the diff — nowhere on disk.**
- **CRITICAL FINDING (verified byte-for-byte this session): the live server at localhost:4533 serves the ORIGINAL HEAD player.js, NOT the fixed version.** `curl /player.js` (9189 B, 318 lines) is byte-identical to `git show HEAD:web/assets/player.js` (fc: "nenhuma diferença encontrada"). The binary predates the WIP. Consequence: live QA exercises ORIGINAL behavior — `audio.volume` defaults to 1.0 (state says 0.8, mismatch), mediaSession handlers use `togglePlay()` (not direct), no `Math.max(0, queue.length-1)` clamp (sets `queue.length`), no readyState guards in prev()/seek(), no optimistic pendingSeek. The `if (!switching)` guard IS present in served code (it is the original HEAD code — the guard predates the WIP). So: **the 6 kept hunks cannot be proven against the live server as-is; only the guard behavior can.**
- Served `app.js` (61749 B) == main tree app.js (WIP touched only player.js); served style.css (34100 B) == main tree; admin.js/pwa.js/sw.js/manifest/loja.html all == main tree (sizes verified). Served index.html = 1541 B vs 1596 B main = the `__ASSET_VERSION__`→`1.16.0` substitution (static.go:28); served refs carry `?v=1.16.0`.
- Player fix validation from T2/auditor stands as the static evidence for the 6 hunks (diff + reconstructed file review + node --check exit 0). Live QA can only re-prove the guard and general control behavior.
- Data: catalog LIVE — `GET /api/store/categories` (PUBLIC, no auth) returns 2 categories (Cristão 145 músicas, Music 130). DB = PostgreSQL via pgx (internal/store/*.go); no sqlite.
- Login: SPA shows login screen when `!auth.user` (app.js:1399-1402) — **player controls require login**. Client = phone only (`POST /auth/login {phone}`); admin = username/email + password (`POST /auth/login {username,password}`, auth.go:84-103). Admin account seeded only on first boot from `ND_ADMINUSERNAME`/`ND_ADMINPASSWORD` (.env — OFF-LIMITS per AGENTS.md, do not read).
- `window.__player = { getState: () => state, audio }` — present in SERVED code too (served line 318; main line 324). Verified.

## Existing Tools & Resources
- Playwright MCP (verified working): navigate / snapshot / find / click(target,element) / type / fill_form / console_messages(level=error) / network_requests(static=false) / network_request(index,part) / take_screenshot(scale=css,filename) / wait_for(time|text) / evaluate(function=()=>...) / resize(width,height). Screenshots default to `.playwright-mcp/` in workdir — ALWAYS pass explicit filename under the loop dir for vision handoff.
- Vision subagent: `task vision: "OCR and analyze <loop-stack/validate-play-music-v116/shot.png>: <question>"` (read-only).
- `node --check` — Node v24.19.0 auto-detects ESM; **verified exit 0 on ALL 6 files this session** (app.js, player.js, admin.js, api.js, pwa.js, sw.js) → checkbox 1 provable.
- HTML refs: all 200 on live server (`/manifest.webmanifest`, `/apple-touch-icon.png`, `/icon-192.png`, `/icon-512.png`, `/style.css?v=1.16.0`, `/app.js?v=1.16.0`, `/player.js?v=1.16.0`, `/pwa.js?v=1.16.0`, `/sw.js?v=1.16.0`, `/admin.js?v=1.16.0`, `/loja.html`) → checkbox 2 provable. loja.html is fully self-contained (inline CSS/JS, no external deps).
- Client session bootstrap (no secrets, public API): `POST /api/store/register {"phone":"11999990000","categoryIds":[]}` → creates client user by phone (10-11 digits, no leading 0 — phone.go:14-29) + returns `{token, user}`. Then `localStorage.setItem('pm_token', token)` + reload = logged-in session with player bar. Reversible (one throwaway client row).

## Requirements & Constraints
- **6 acceptance checkboxes (VERBATIM from ORIGINAL_REQUEST.md):**
  1. `- [ ] Todos os arquivos JS em web/assets/*.js possuem sintaxe válida sem erros de compilação ou parse.`
  2. `- [ ] Os arquivos HTML (index.html, loja.html) contêm referências válidas para todas as dependências de CSS, JS e imagens.`
  3. `- [ ] Os controles do player (play, pause, buscar faixa, volume, progresso) funcionam sem lançar exceções não tratadas no JavaScript.`
  4. `- [ ] O catálogo de músicas na página da Loja (loja.html) e o painel Admin (admin.js) renderizam itens corretamente com feedbacks visuais claros para o usuário.`
  5. `- [ ] O CSS global em web/assets/style.css utiliza layout responsivo (Flexbox/Grid) adaptado para dispositivos móveis (<768px) e desktop sem sobreposição de texto ou quebra de elementos.`
  6. `- [ ] O tema e componentes visuais utilizam transições de hover suaves, contraste adequado e elementos de UI refinados.`
- Report-only: tick ONLY criteria proven in this session; record bugs in loop-reports/qa-r1r3.md (create dir — still absent); NO auto-fix, NO commit.
- Deploy-gap decision needed BEFORE player-behavior assertions: the fix is not live. Options: (a) human applies player-wip-fix.diff to main tree + rebuild + restart, then QA proves the 6 hunks; (b) QA the live original + rely on T2 diff/static evidence for the 6 hunks, explicitly recording "not deployed — fix-specific assertions cannot pass live" (their failure is EXPECTED, not a bug). AGENTS.md forbids touching .env and restarting without approval — executor must ask the human if (a).
- Admin panel render (checkbox 4, admin part): requires admin login (`/#/admin` redirects non-admins to `/`, app.js:371-378). Admin creds live in .env — **executor must ask the human for them**; do NOT read .env. Partial evidence without admin: admin.js is imported unconditionally at app boot (app.js:7 `await import('./admin.js?v=...')`) → zero console errors at boot already proves admin.js parses/loads; only its item rendering needs admin.
- Player QA (checkbox 3): needs a logged-in CLIENT session (phone-only path above). Assertions via `playwright_browser_evaluate` `() => { const s = window.__player.getState(); return { playing: s.playing, paused: window.__player.audio.paused, progress: s.progress, t: window.__player.audio.currentTime, volume: s.volume, avol: window.__player.audio.volume, idx: s.currentIndex, len: s.queue.length, pending: s.pendingSeek } }`.

## Suggested Approach
1. `node --check` on all 6 files (re-run, record exits) + curl all HTML refs (record 200s) → tick checkboxes 1-2.
2. Playwright: navigate to / (PWA overlay appears after ~1.5s — dismiss with "Agora não" before interacting; SW may serve cached assets — versioned URLs mitigate). Create client session via evaluate: `fetch('/api/store/register',{...})` or navigate loja.html login first; set token; reload.
3. Player QA on home: click a `.card-play`/`.track-play-btn` (Tocar) → assert `__player` state vs audio (playing===!paused, progress≈currentTime, volume 0.8 vs avol 1.0 — EXPECTED mismatch on live original, record as deploy-gap not bug); play/pause via `.player-btn-main` aria-label Tocar/Pausar; seek via `.progress-track` click or `player.seekBy` via ±5s buttons (aria-labels Retroceder/Avançar 5 segundos); volume via `.volume-slider`; prev/next via aria-label Anterior/Próxima; next-past-last (repeat off → `currentIndex: Math.max(0,len-1)` only in FIXED — live shows `queue.length`, record); console_messages(level=error) == 0 after each step.
4. Loja: navigate /loja.html → #catGrid/#packsGrid render cards (.card, .buy-btn) — PUBLIC, no login needed.
5. Admin: if human provides creds → login Admin tab → /#/admin renders tabs Usuários/Categorias/Músicas + rows. Without creds: assert boot console has no errors (admin.js imported) + document blocker.
6. Responsive: `playwright_browser_resize` 375x667 and 767x900 → snapshot + screenshot to loop dir → vision pass (no overlap/overflow).
7. Hover/contrast: style.css has `@media (max-width: 767px)` (lines 109, 1692), `(max-width: 480px)` (1465), `(hover: hover)` (1099) — verify transitions/computed styles + vision on screenshots.
8. Write loop-reports/qa-r1r3.md (evidence per step: actions, state snapshots, console output, network statuses, screenshot paths) → tick proven checkboxes.

## Verification Criteria
PASS (per checkbox, live-evidence based):
- C1: `node --check` exit 0 on all 6 files (recorded).
- C2: every ref in index.html + loja.html resolves HTTP 200 and file exists in web/assets (manifest, apple-touch-icon, icon-192/512, style.css, app.js, pwa.js, sw.js; loja self-contained).
- C3: play/pause/seek/volume/progress via UI with zero console errors (level=error == 0); `window.__player` state consistent with audio element after each action.
- C4: loja.html renders ≥1 category card + pack (public); admin panel renders tabs+items IF admin creds provided (else partial: boot console clean + blocker documented).
- C5: at 375x667 and <768px widths, snapshot shows no overlapping/broken elements; vision confirms (screenshot paths recorded).
- C6: CSS shows transition rules + hover media queries; vision confirms contrast/refinement.
- Fix-specific assertions (avol=0.8, clamp, readyState guards, pendingSeek): PASS only if the fix is deployed first; otherwise record as "fix not deployed — cannot verify live" (NOT a bug).
FAIL:
- Any console error at any step; checkbox ticked without its evidence; fix-specific assertions reported as bugs without the deploy-gap note; .env read; admin creds requested from files instead of the human.

## Quality Standards
- Evidence-first: every tick backed by recorded output (console text, exit codes, HTTP codes, evaluate snapshots, screenshot paths in qa-r1r3.md).
- Distinguish THREE code states: live server = HEAD/original; main tree = WIP (desync bug present!); fixed = diff only. Never conflate them — a "bug" found live may be the original behavior, not a WIP regression.
- Keep QA non-destructive: one throwaway client user via public register is acceptable and reversible; no admin deletes, no uploads.
- Dismiss the PWA install overlay before every interaction session; re-assert console after it.
- Anti-patterns: ticking a checkbox without session evidence; claiming the fix works live when it is not deployed; reading .env for admin creds; restarting the server without human approval.

## Prior Attempt Analysis
None (0 attempts on T3). T2 note "never rebuild/restart to QA; it's already healthy" applies to liveness checks — for T3 the fix is NOT on the live server, so if fix-specific proof is required the executor must get human approval to deploy+restart (or accept static-evidence-only).

## Task-Specific Research — [G3] T4 — v1.16.0 feature regression QA

## Context & Prior Work
- Commit `8df05211` (v1.16.0, "usuarios admin vs cliente ... guarda contra remover o ultimo admin") is HEAD; the commit diff was read in full this session. 8 files, +197/-61: `internal/server/handlers_api.go` (admin create/update/delete guards), `internal/store/users.go` (email column + `GetUserByUsernameOrEmail` + pointer-based `UserPatch`), `internal/auth/auth.go` (login by username-or-email), `internal/db/migrations/0007_user_email.sql` (new), `internal/model/model.go` (+Email), `web/assets/admin.js` (user form tipo toggle), `web/assets/app.js` (login placeholder "Usuário ou e-mail"), `internal/version/version.go` (1.16.0).
- **LIVE SERVER VERIFIED SERVING v1.16.0**: `http://localhost:4533/` → 200; served index.html has `style.css?v=1.16.0` and `app.js?v=1.16.0` (checked live this session). Unlike the T2 player fix, the v1.16.0 feature surface IS deployed — live browser QA of these features is possible.
- v1.15.0 (cb26fb1b) thumbnails: pure CSS change — `.track-art` `display: none` → `block` (style.css:577-581) + 2-line clamp on `.track-title` (style.css:586-592) and `.now-playing-title` (style.css:1124-1130). Thumbnails were already rendered in DOM before; 1.15.0 made them visible in ALL lists + 2-line titles.
- v1.14.0 (e1ea6cc5) category covers in forms: admin.js `newCategoryForm` (photo drop + upload after create, admin.js:190-251) and `categoryForm` (cat-photo-preview img + Enviar/Remover foto buttons, admin.js:282-404); backend `internal/artwork/artwork.go` `UploadCategoryPhoto`/`DeleteCategoryPhoto` (artwork.go:224-260) write BOTH Postgres `artworks` table AND MinIO `covers/<id>.jpg`; `resolveCategory` (artwork.go:146-167) falls back Postgres→MinIO. Endpoints: `POST/DELETE /api/admin/categories/{id}/photo`.
- Prior T3 QA memory: browser profile previously contained a leftover admin JWT (name "admin", isAdmin:true — observation O5) but T3 OVERWROTE it with the client token; the admin password is NOT known. Loop-reports/ exists (T1-T3 deliverables present) — executor appends `loop-reports/qa-v116.md`.

## Existing Tools & Resources
- Playwright MCP + `window.__player` hook (player.js:324, present in served code), vision subagent, curl.exe (use `--data-binary "@file"` for JSON bodies — PS mangles inline `-d`), node --check. All per loop TOOLS.md; no new tools needed.
- Reusable QA recipe (loop MEMORY.md T3): throwaway client = `POST /api/store/register {"phone":"<NEW throwaway>"}` → token → `localStorage.setItem('pm_token', token)` → reload; idempotent per phone; `categoryIds:[<id>]` grants categories; cleanup after QA. Use a FRESH phone (e.g. 11999990002) — 11999990001 is still flagged for cleanup from T3.
- Live data: 2 categories (Cristão 145, Music 130), 146+ songs (last scan). `GET /api/store/categories` is public (no auth) — useful pre-QA.

## Requirements & Constraints
- Deliverable: `loop-reports/qa-v116.md` — per-feature evidence (actions, state, console, API statuses, screenshot paths) + bug list with reproduction steps. **Record bugs only — do NOT auto-fix, do NOT modify source.** Verify-only for the last-admin guard: NEVER delete/demote the real admin.
- Admin creds: `.env` is DENYLISTED (AGENTS.md). No test admin with known password exists (prior loops only created clients via public register). **Executor must ask the human for admin creds** (or schedule a human manual pass) for admin-gated features; without them, document the blocker and rely on static code evidence.
- Guard touch rules: self-edit/self-delete attempts return 400 without mutation — these two ARE safe live tests of the self-guards. The count-based guard (400 "Não é possível remover o último administrador") must NOT be live-triggered against the seed admin (would require demoting/deleting the real admin) — verify statically only.

## Suggested Approach
Two-track QA: (A) client-only browser session (no creds needed) covering login-screen field mapping, client login variants, non-admin 403/redirect checks, thumbnails in all lists + now-playing bar (desktop + 375px), category covers in client category pages; (B) admin-gated flows (user form per tipo, tipo toggle + field cleanup, create/edit/delete of a THROWAWAY client user only, last-admin self-guards live, duplicate-email behavior) — pending human admin creds; record blocker if not provided. Write evidence to loop-reports/qa-v116.md.

## Verification Criteria
PASS (live-evidence based, per feature):
1. **Login screen fields** (no creds needed): client mode shows ONLY phone input; admin mode shows "Usuário ou e-mail" + "Senha" inputs (app.js:1012-1021); toggling re-renders with no stale fields; POST /auth/login payload inspected via network request (client → {phone}, admin → {username,password}).
2. **Client login variants** (no creds): fresh throwaway phone → 200 + isAdmin=false + token; wrong/unknown phone → 401 "telefone não cadastrado"; client credentials sent as {username,password} → 401 (clients have no username; LoginUsername requires isAdmin, auth.go:92-94).
3. **Admin login by username AND email**: `{username,password}` AND `{email,password}` both → 200 — ONLY if human provides creds; otherwise record blocker + static evidence (store/users.go:81-99 OR-query, auth.go:84-103).
4. **Admin users list / user form per tipo** (creds needed): user list rows show badge "Admin" + @username vs phone (admin.js:87-88); "Novo usuário" form: client default → toggle Cliente/Administrador (admin.js:124-125), admin mode shows Usuário+E-mail+Senha and hides Telefone/Categorias, client mode shows Telefone only and hides Usuário/E-mail/Senha (sync(), admin.js:127-134); switching tipo mid-form swaps fields (field cleanup). Create a THROWAWAY client (phone only) → row appears; edit it, toggle to admin with username+email+password → phone cleared; toggle back to client → username/email cleared. All reversible, no impact on real users.
5. **Last-admin guard** (verify-only): static — demote guard at handlers_api.go:616-626 (`SELECT count(*) FROM users WHERE is_admin`; count<=1 → 400 "Não é possível remover o último administrador"); self-edit 400 (597-601); self-delete 400 (675-686); NO client-side guard in admin.js (server-side only). Live (safe): with admin creds, attempt self-edit → 400, self-delete → 400, verify no mutation (list users unchanged). NEVER demote/delete the seed admin.
6. **Thumbnails in lists** (no creds, client session): every `.track-art` img in home/category/playlist/search/liked/history rows resolves (src `/api/artwork/<id>?size=48&jwt=...`), HTTP 200, `naturalWidth > 0` — at desktop AND 375x667 (resize); now-playing bar `.now-playing-art` (app.js:1225, size 64) shows after starting playback; titles clamp to 2 lines (`.track-title`, `.now-playing-title` webkit-line-clamp:2); 0 console errors after each action. NOTE: artwork.Serve NEVER 404s — missing covers render a generated gradient placeholder (artwork.go:90-94), so naturalWidth>0 does NOT prove a real cover; vision pass or byte-comparison vs placeholder needed to distinguish.
7. **Category covers in forms** (creds needed): `newCategoryForm` shows "Foto da categoria (opcional)" drop; `categoryForm` shows `#cat-photo-preview` (artworkUrl(cat.id,96)) + "Enviar foto"/"Remover foto" (admin.js:288-295); create category + upload photo → preview updates with `&t=Date.now()` cache-bust (admin.js:379-382); delete category cleans MinIO covers/<id>.jpg + Postgres artwork (artwork.go:248-260, handlers_api.go:752-757). WITHOUT creds: verify client-side category pages show covers (app.js:661 detail-art) and record blocker for the forms.
8. **Non-admin isolation** (no creds): client token on GET /api/admin/users → 403 "Sem permissão" (requireAdmin server.go:161-186); navigating /#/admin as client → redirect to #/ (app.js:371-378).
9. **Bug candidates to attempt (safe)**: (a) duplicate email on admin create/edit → currently pg unique violation → `handleStoreError` → 500 "Erro interno" (helpers.go:78-89 maps only ErrNotFound/ErrForbidden; no unique-violation → 4xx mapping) — expect 400-like message, likely 500 = BUG (needs admin creds; static evidence if not); (b) no unique index on username (only email partial unique idx_users_email, 0007_user_email.sql) → duplicate usernames possible, login-by-username returns arbitrary row — observation, not a regression; (c) bootstrapAdmin (auth.go:70-75) creates the env-seeded admin WITHOUT email → seed admin cannot log in by email until edited — feature gap note, not a regression.

FAIL:
- Any 401/403/500 during client-only flows where 2xx expected; console errors at any step; any feature claimed verified without its recorded evidence; any mutation of the real admin account; .env read; admin features reported PASS without human creds (record PARTIAL/BLOCKED instead).

## Quality Standards
- Evidence-first, mirroring qa-r1r3.md: per-step actions, state snapshots, console level=error re-asserted 0 after every action, HTTP statuses, screenshot paths under the loop dir (explicit filenames — default lands in .playwright-mcp/).
- Distinguish BLOCKED (no creds) from PASS/FAIL; static code evidence for admin guards is legit only as code-level verification (quote line numbers).
- Dismiss the PWA overlay ("Agora não") before every interaction session; MCP refs go stale after re-renders — re-run find/snapshot before clicks.
- Cleanup: delete the throwaway client (phone used for T4) via admin panel at the end IF creds were provided; otherwise leave it flagged for the human (like T3's 11999990001).
- Anti-patterns: live-triggering the last-admin count guard; creating a second REAL admin for testing (use clearly-throwaway username e.g. qa-admin-<ts> only if the human OKs); reporting placeholder-artwork as "missing thumbnail" (it's by design).

## Prior Attempt Analysis
- No prior attempts on T4 (0 attempts; T3 audit already flagged "admin item-render NOT verified — needs human creds; re-check in T4" as the known WARN). T3's O5 (leftover admin token in browser profile) is gone — overwritten by the client token; do not rely on it. No admin password ever appeared in loop artifacts (verified across RESEARCH.md/MEMORY.md/STATUS.md).
