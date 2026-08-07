# Research Log

## Context & Prior Work

Domain: full inventory of the CURRENT frontend design (v1.16.0 era, HEAD cb26fb1b). Everything below was read from the actual files in `web/assets/` on 2026-08-07.

### Architecture (critical context)

- **SPA shell**: `web/assets/index.html` (39 lines) is a bare shell — `<div id="app">`, `__APP_CONFIG__`, loads `app.js` (module) + `pwa.js` + `sw.js`. ALL views are rendered by vanilla JS (hash router) inside `#app`.
- **No admin.html exists** (confirmed). Admin UI is `admin.js` (693 lines), dynamically imported by app.js and rendered as an in-app page + modals.
- **loja.html** (310 lines) is a STANDALONE page with its OWN inline `<style>` block — its design tokens are duplicated and DIVERGENT from style.css (see palette below). It reuses token NAMES but different values.
- **player.js** (296 lines) is fully UI-agnostic — zero `getElementById/querySelector/classList`. It exposes state functions (`playContext`, `togglePlay`, `getPlayerState`, `subscribe`, ...) and app.js renders the bar into `#player-bar` and fullscreen into `#player-full`.
- App renders via `el(tag, attrs, ...)` helper (app.js:15-30) — `class`/`id` keys are set via `node.className`/`setAttribute`. Nearly every class in CSS is produced by this helper (see allowlist).
- Prior design evolution (git log on style.css): v1.14 (PWA install popup, queue markers), v1.15 (thumbnails everywhere, 2-line titles), v1.16 (admin users, login toggle, checkout links). No prior full-design-overhaul attempts — this is the first.

### Token inventory — style.css `:root` (lines 1-12, ONLY 11 tokens)

| Token | Value | Used for |
|---|---|---|
| `--bg` | `#0b0b0f` | page background, gradients |
| `--surface` | `#121216` | sidebar, bottom bar, cards, modals, login form |
| `--surface2` | `#1a1a1f` | inputs, dropzones, secondary buttons, fullscreen bg |
| `--grid` | `#2a2a2a` | borders (via `0 0 0 1px` box-shadow), scrollbar, progress track |
| `--hover` | `#24242b` | hover backgrounds (nav, cards, rows) |
| `--accent` | `#618dff` | primary blue — buttons, active states, icons |
| `--accent-hover` | `#7ba0ff` | **defined but NEVER used anywhere in CSS** |
| `--subtext` | `#a0a0a8` | secondary text |
| `--faint` | `#6b6b73` | tertiary text, labels |
| `--font-sans` | `system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif` | body font |

**Undefined tokens referenced (fallbacks save them):** `var(--danger, #f87171)` (upload-fail), `var(--text, #f0f0f4)` (login-toggle/modal/admin rows), `var(--accent, #1db954)` (admin section — legacy GREEN accent fallback). `--text`, `--danger` do NOT exist in `:root`.

**loja.html has its OWN token block (lines 9-17):** `--bg #0b0b0f`, `--surface #1c1c24` (≠ main), `--surface2 #1a1a1f`, `--grid #2a2a2a`, `--accent #1db954` (GREEN ≠ main blue), `--subtext #a0a0a8`, `--faint #6b6b76`.

### Color palette (every distinct value, style.css)

- **Brand blue (app accent):** `#618dff` (--accent), `#7ba0ff` (--accent-hover, unused), `rgba(97,141,255,...)` gradients (0.4 alpha).
- **Legacy green (admin section + loja):** `#1db954` (accent fallback, loja accent, chip bg rgba(29,185,84,0.14)), `#3ddc84` (chip text), `rgba(29,185,84,0.06)` dropzone drag bg.
- **Backgrounds:** `#0b0b0f`, `#121216`, `#1a1a1f`, `#1a1a21` (login-toggle fallback), `#1c1c24` (modal), `#26262f` (admin-row border), `#2a2a2a`, `#2c2c36` (modal border), `#3a3a42` (scrollbar hover), `#101018`/`#24242e` (loja thumb gradient).
- **Text:** `#ffffff`/`#fff` (primary), `#000` (on-white buttons, tab-btn.active, player-btn-main), `#f0f0f4` (--text fallback).
- **Semantic:** danger `#f87171` (login-error, remove-track-btn, upload-fail, loja .msg); warning `#ffcf6b` + bg `rgba(255,200,60,0.16)` (admin badge); white-on-accent buttons use `#fff`.
- **Overlays/shadows:** `rgba(0,0,0,0.55)`/`0.6`/`0.72` (overlays), `rgba(8,8,12,0.72)` (modal), `rgba(255,255,255,0.04-0.1)` (hover washes), `rgba(42,42,42,0.5)` (borders).
- **loja.html only:** `#101018`, `#24242e` (thumb gradient), `#6b6b76` (--faint).

### Typography

- **One font stack:** `--font-sans` (system-ui stack) everywhere; `monospace` only for `.settings-dd.mono`. NO custom/webfont.
- **Sizes (all hard-coded px, no tokens):** 10 (badge), 11 (progress, pack-badge, chips), 12 (labels, subtitles, faint), 13 (modal-sub, pwa), 14 (body default — most components), 15 (admin-row-title), 16 (fullscreen-artist, loja h3, touch inputs forced 16px), 18 (brand, section titles, settings h2, modal h3), 20 (section-title, section-block h2), 22 (loja price), 24 (fullscreen-title), 26 (loja h1), 30 (page-title, login-brand h1), 36 (detail-title), 48 (detail-title desktop), 40 (loja thumb icon), 60 (detail-art-icon).
- **Weights:** 400, 500, 600, 700, 800 (loja price). letter-spacing: -0.02em (titles), 0.05-0.1em (uppercase labels). line-clamp 2 used for titles.

### Spacing system

- **NO spacing tokens.** Literal px values: 2, 4, 6, 8, 10, 12, 14, 16, 20, 24, 32, 40 (empty-state), 96/196 (page bottom padding for player bar). Gaps: 2-24px. Standard paddings: 8/12/16/20/24.

### Border-radius (literal, no tokens)

- 4 (track-art), 6 (card-image-wrap), 8 (nav-links, cards, inputs, rows, buttons), 10 (now-playing, admin-row, login-toggle), 12 (track-list, detail-art, photo-drop), 14 (modal, dropzone), 16 (login-form, settings-card, pwa-card, loja card), 9999/999px pills (all buttons, search input, chips, badges), 50% (spinner).

### Shadows (literal)

- `0 25px 50px -12px rgba(0,0,0,0.8)` (detail-art/fullscreen-art), `0 25px 50px -12px rgba(0,0,0,0.6)` (mobile-sidebar), `0 10px 15px -3px rgba(0,0,0,0.4)` (card-play), `0 20px 25px -5px rgba(0,0,0,0.4)` (login-form), `0 20px 60px rgba(0,0,0,0.5)` (pwa-card).

### Breakpoints

- `max-width: 480px` — upload-grid single column.
- `max-width: 560px` — loja topbar stacks (inline, loja.html only).
- `min-width: 640px` — track rows get 5-col grid, album col visible, detail-header horizontal, detail-title 48px, track-list-header grid.
- `max-width: 767px` — sidebar hidden, mobile-topbar shown, sidebar-close shown, .page padding 196px, fullscreen-buttons gap 10px.
- `min-width: 768px` — bottom-bar 3-col grid h96px, now-playing-art 56px, player-controls order, volume visible, mobile-overlay hidden !important.
- `(hover: hover)` — now-playing hover bg. `(hover: none)` — touch: always-visible row actions, bigger tap targets (44px main btn), card-play always shown, track-num-text hidden. `(hover: none) and (pointer: coarse)` — inputs forced 16px (iOS zoom).

### Components styled (style.css, 2028 lines, 15 commented sections)

app-shell layout (sidebar 240px / main-area gradient / page), sidebar (brand, close, nav, playlists label/list, footer, user), mobile topbar + overlay/backdrop/sidebar (288px), cards (card 144px, card-image-wrap square, card-play hover reveal, card-title/subtitle, genre-card gradient 135deg, genre-count, empty-state, card-row, card-wrap), tabs (pill tab-btn, active = white bg black text), track rows (grid 2.5rem/1fr/auto, number/play-btn swap, playing state with ▶, art 40px, 2-line title, artist, album, like, duration), detail header (art 176→224px, round, icon, type/title/meta/link, actions, btn-accent, btn-icon-lg, liked), track-list-header, section-block, back-link, playlist-track-row + remove-track-btn, login screen/box/brand/form/error/submit, search bar/input/select (pill w/ custom chevron SVG), bottom player bar (now-playing w/ art 48→56, controls, buttons, progress track/fill/knob, dragging state, volume slider accent-color), fullscreen player (art min(72vw,384px,40vh), title 24, buttons 64px), settings (page/card/text/actions/playlists), upload form (modal-upload, fields, grid, hidden file input, dropzone w/ drag/error states, photo-drop, info, progress bar, fails), queue (current marker, track-wrap), spinner + spin keyframes, loading-screen, mobile-only, login-toggle (admin/client switcher), track-add button, modal system (overlay z100, modal 420px, modal-wide 620px, h3, sub, empty, item/new, close, actions, section-label, check, scroll), admin (toolbar, table, row, row-main, title, sub, actions, chips, chip green, badge yellow), PWA popup (overlay z1000 blur, card, header, icon, desc, tips, steps, actions, pwa-btn + primary).

### JS-coupled class/ID allowlist (changes MUST keep these working)

IDs (getElementById): `app`, `search-results`, `page-content`, `player-bar`, `player-full` (app.js); `cat-songs`, `cat-song-filter`, `cat-photo-preview`, `cat-error`, `cat-name`, `cat-checkout`, `folder-upload-start` (admin.js); `loginBox`, `loginMsg`, `phoneInput`, `loginBtn`, `catGrid`, `packsGrid`, `packsSection` (loja.html). TOTAL: **18 IDs**.

Classes queried by JS: `.track-list`, `.queue-track-wrap`, `.track-row` (with `[data-song-id]`), `.track-like`, `.btn-accent`, `.btn-secondary`, `.upload-dropzone-title`, `.upload-dropzone-hint`, `.upload-progress-fill`, `.upload-progress-text`, `.pwa-overlay`, `.pwa-card`; structural: `input[type=checkbox]:checked` (ids `cat-*` sliced), `span:last-child` (photo-drop label). TOTAL: **12 class queries + 2 structural**.

State classes toggled by JS (classList): `liked`, `playing`, `dragging`, `active`, `drag`, `has-file`, `error`, `login-error`, `upload-info`, `ok`, `open`; set via className: `page-padding`, `pwa-card`, `pwa-overlay`. TOTAL: **14 state classes**.

Data attributes: `data-song-id` (rows), `data-act="dismiss"/"install"` (PWA popup buttons), `data-icon` (icon spans, set by app.js:110).

JS classes rendered but NOT styled in CSS (dead rules — safe to style): `vol-icon` (app.js:1493-1495), `modal-empty` IS styled; `has-file` styled only as class (no .has-file rule — used purely as JS state).

### PWA / manifest (web/assets/manifest.webmanifest)

- `theme_color` + `background_color`: **`#0B0B0F`** (matches index.html meta theme-color `#0B0B0F`, apple status-bar `black-translucent`).
- Icons: `icon-192.png` (192x192 any), `icon-512.png` (512x512 any + maskable), `apple-touch-icon.png`. display standalone, orientation portrait, scope `/`.
- sw.js: `pm-shell-v1` caches shell (`./`, manifest, 3 icons); network-first navigations, stale-while-revalidate static, NEVER intercepts `/api/` or `/auth/` (critical for streaming).

### Existing visual evidence — `.playwright-mcp/` (root)

- **ZERO image screenshots** (no .png/.jpg/.jpeg/.webp). Contains **45 `console-*.log`** + **168 `page-*.yml` accessibility snapshots** from prior QA (2026-08-05 → 2026-08-07 15:21Z; biggest batches at 14:38-15:21Z 2026-08-07 = latest QA run). The .yml files are accessibility trees (usable to reconstruct current DOM structure per view); the logs hold console errors. No visual/design evidence exists — design review must be done live via playwright + vision subagent (per .global/TOOLS.md, vision = `task vision:`).

## Existing Tools & Resources

- **playwright MCP** (CONFIRMED, .global/TOOLS.md): navigate/snapshot/click/type/evaluate/console/network — available for live visual QA against `http://localhost:4533/`. Note MEMORY.md T3: server serves HEAD build (go:embed) — JS/CSS changes are NOT visible live until human rebuilds; design QA must target a locally-built asset or accept served-HEAD visuals.
- **vision subagent** (`openrouter/qwen/qwen3.7-flash`): screenshot analysis — but MEMORY.md T3 warns vision is NOT callable inside loop agent contexts (no task tool / image input in executor sessions); researcher confirms same limitation here (no task tool available). Screenshots should be saved for human/orchestrator review.
- **.playwright-mcp/** accessibility snapshots: 168 .yml files — free DOM-structure reference for current render, no setup needed.
- **No CSS framework, no design system, no build step** — style.css is plain CSS hand-maintained; sw.js cache-busting uses `?v=__ASSET_VERSION__` (server-injected; CSS/JS edits need version bump for SW clients, see embed.go).

## Requirements & Constraints

Researcher pass 2 (2026-08-07) — Requirements & Constraints domain. Sources: ORIGINAL_REQUEST.md (verbatim), live palette computed (WCAG 2.1), AGENTS.md + loop-constraints.md (verbatim rules), git status (repo state), .env key list + manifest (design direction).

### R2 — verbatim from ORIGINAL_REQUEST.md (2026-08-07T16:36:41Z)
> R2. Refinamento Visual e Modernização de UI/UX — Redesenhar e aprimorar a estilização visual em `web/assets/style.css`, `index.html` e `loja.html`, aplicando estética moderna (paleta de cores harmoniosa em modo escuro/gradientes, animações e transições suaves, tipografia fluida e navegação responsiva em telas mobile e desktop).

### Acceptance criteria C5 + C6 — verbatim from ORIGINAL_REQUEST.md
> C5. O CSS global em `web/assets/style.css` utiliza layout responsivo (Flexbox/Grid) adaptado para dispositivos móveis (<768px) e desktop sem sobreposição de texto ou quebra de elementos.
> C6. O tema e componentes visuais utilizam transições de hover suaves, contraste adequado e elementos de UI refinados.

Note: R1 (bug fixes) and R3 (no console errors / Go compatibility) are ALREADY ticked [x] in the request — this loop owns R2 only; C5/C6 are its acceptance gates. "Modo escuro/gradientes" is explicitly in R2's scope: current palette is already dark+gradients, so R2 = evolve/polish it, not introduce light-on-dark ambiguity (light theme optional, see direction signals below).

### WCAG contrast audit of CURRENT palette (computed 2026-08-07, WCAG 2.1 relative luminance)

| Pair | Ratio | Verdict |
|---|---|---|
| #fff on #0b0b0f (primary text/bg) | 19.64 | PASS |
| #f0f0f4 on #121216 (--text fallback/surface) | 16.44 | PASS |
| #fff on #121216 (cards/nav) | 18.69 | PASS |
| --subtext #a0a0a8 on #0b0b0f / #121216 / #1a1a1f | 7.57 / 7.20 / 6.68 | PASS |
| --accent #618dff on #0b0b0f / #121216 | 6.33 / 6.02 | PASS (blue as TEXT is fine) |
| #1db954 (loja green) on #0b0b0f / #1c1c24 | 7.60 / 6.54 | PASS |
| danger #f87171 on #121216 | 6.76 | PASS |
| warning #ffcf6b on #121216 | 12.80 | PASS |
| #000 on #fff (tab-btn.active) | 21.0 | PASS |
| **#fff on #618dff (.btn-accent, app buttons, style.css:759-768)** | **3.11** | **FAIL AA normal** (ok only as large/UI 3:1) |
| **#fff on #1db954 (.buy-btn/.login-box button, loja.html:60,88-89)** | **2.59** | **FAIL (even 3:1 large fails)** |
| **--faint #6b6b73 on #0b0b0f / #121216 / #1a1a1f** (12-14px labels, style.css 8 uses + app.js:920 + loja .empty/footer) | 3.72 / 3.54 / 3.28 | **FAIL AA normal** (large-only) |
| loja --faint #6b6b76 on #1c1c24 | 3.22 | **FAIL AA normal** |
| borders/dividers: #2a2a2a vs #0b0b0f / #121216, #26262f row, #2c2c36 modal, #3a3a42 scrollbar, #24242b --hover | 1.23–1.66 | **FAIL 3:1 UI-component contrast (WCAG 1.4.11)** |
| --subtext on loja surface #1c1c24 | 6.52 | PASS |
| chip text #3ddc84 vs composite chip bg (rgba(29,185,84,0.14) over #121216 = #14291f) | 8.61 | PASS |

**Correction to prior researcher note** (RESEARCH.md "Verification Criteria" said #618dff on #0b0b0f fails AA): measured 6.33:1 — PASSES. The real accent problem is INVERTED: white text on accent fills.

**Concrete AA fix targets:** (1) darken accent for button fills OR bump btn text to ≥18.66px bold (btn-accent labels are 14px normal → need 4.5:1; e.g. accent fill #4d7dff would give ~4.6:1 with #fff); (2) loja green buttons need a darker green fill (e.g. #16a34a → 3.4:1, still short; ~#15803d ≈ 5:1) or dark text; (3) --faint must go ≥4.5:1 (needs ≥#8a8a93 on #0b0b0f / ≥#9a9aa2 on #121216); (4) all 1px borders/--hover/scrollbar need ≥3:1 vs their bg (requires ≥#6a6a70 on #0b0b0f, ≥#66666e on #121216 — or accept as decorative where not essential UI). Verify with the audit script pattern (contrast formula above; recompute after token changes).

### JS-coupled selector allowlist (MUST keep working — from "Context & Prior Work" above)
- **18 IDs** (`getElementById`): app, search-results, page-content, player-bar, player-full, cat-songs, cat-song-filter, cat-photo-preview, cat-error, cat-name, cat-checkout, folder-upload-start, loginBox, loginMsg, phoneInput, loginBtn, catGrid, packsGrid, packsSection.
- **12 class queries** (.track-list, .queue-track-wrap, .track-row[data-song-id], .track-like, .btn-accent, .btn-secondary, .upload-dropzone-title, .upload-dropzone-hint, .upload-progress-fill, .upload-progress-text, .pwa-overlay, .pwa-card) + 2 structural (input[type=checkbox]:checked, span:last-child).
- **14 state classes** (liked, playing, dragging, active, drag, has-file, error, login-error, upload-info, ok, open + className-set: page-padding, pwa-card, pwa-overlay) + data-attrs data-song-id / data-act / data-icon.
- Any rename = paired JS update (el() helper in app.js:15-30 generates class/id strings). `.btn-accent` is the most contrast-critical ALLOWLIST member (fix via CSS only — no rename).

### Constraint rules (binding — AGENTS.md + loop-constraints.md, verbatim spirit)
- Start in L1 report-only; **no source edits until human enables L2**.
- Never edit `.env`, `.env.*`, `auth/`, `payments/`, `secrets/`, `credentials/`.
- Use a **git worktree** for every code-changing attempt; **max 3 fix attempts per item** (log to loop-ledger.json, escalate after); dispatch **verifier sub-agent after L2+ changes**; record test evidence in STATE.md.
- Never push/merge without human approval; no auto-commit; no gate.yaml; run documented tests before proposing a fix.

### Current repo state (git status 2026-08-07)
- **Uncommitted (M):** `internal/server/helpers.go`, `internal/store/common.go`, `internal/store/users.go`, `web/assets/player.js` — previous loop's validated fixes (BUG-1 409 mapping, player switching guard). Design loop must NOT revert or commit them (or include them in any commit); patch on top only.
- Server live on **:4533** (HTTP 200 verified now) running new binary. MEMORY.md T3: assets are go:embed — CSS/JS edits NOT visible live until human rebuilds; SW shell `pm-shell-v1` caches shell until asset-version bump (see embed.go `__ASSET_VERSION__`).

### Design direction signals
- `.env` key **ND_DEFAULTTHEME=Dark** exists (server-side theme default = DARK; frontend has no theme toggle today). Any light-theme work must respect this default and is additive-only.
- manifest.webmanifest + index.html + loja.html all `theme_color: #0B0B0F` — keep in sync on any bg change.
- Plan-mode note: PLAN.md adds light/dark theme + AA + focus states + aria to R2's scope — C5/C6 (verbatim above) are the hard gates; theme toggle is enhancement, not acceptance.

### Suggested Approach (requirements-shaped)
Token-first patch: extend `:root` (add --text/--danger/radius/shadow/scale tokens), fix accent fills for AA (btn-accent + loja green), lift --faint and border contrast to AA thresholds, unify loja tokens (green vs blue) — then per-component polish (hover transitions ≤0.25s, :focus-visible everywhere, touch targets), always via worktree, allowlist untouched.

### Verification Criteria
- PASS: computed contrast ≥4.5:1 (normal text) / ≥3:1 (large/UI) for all pairs above after patch (re-run contrast script); the 4 failing pairs in the table are all fixed.
- PASS: allowlist (18 IDs + 12 classes + 2 structural + 14 state classes + 3 data-attrs) still resolves; `grep` for each selector in patched CSS+JS.
- PASS: loja.html and style.css accent tokens unified (no `var(--accent, #1db954)` legacy fallbacks; --text/--danger defined in :root).
- PASS: theme-color meta + manifest stay `#0B0B0F` (or sync'd to new bg); hover transitions present; no var() references missing tokens.
- FAIL: any allowlist selector renamed without JS pair-update; white-on-green/white-on-blue button text still <4.5:1; uncommitted player.js/helpers.go/common.go/users.go touched or committed.

## Suggested Approach

Use the allowlist above as the contract: refactor tokens first (extend `:root` with text/danger/radius/shadow/spacing tokens, unify accent to the single blue, fix undefined-token fallbacks and `.vol-icon`), then patch components per view using `.playwright-mcp` accessibility snapshots + live playwright (served HEAD) to verify each view; then loja.html (either link style.css or mirror tokens); then light/dark via `color-scheme` + `[data-theme]` class toggled by app.js (new state — safe, additive). Keep all transitions ≤0.25s; ensure `:focus-visible` states exist (currently only form-input/search/select have focus shadows — buttons/cards/nav have NONE).

## Verification Criteria

- PASS: every selector in the allowlist above resolves in the patched DOM and JS behaviors (playing/liked/dragging/open/active states) visibly work after patch; live app (after human rebuild) shows 0 console errors on home/library/search/queue/settings/admin views.
- PASS: no `var(--x)` references a token missing from `:root` (grep), no hard-coded hex remains where a token exists (grep), theme-color meta + manifest in sync.
- PASS: contrast AA check on accent/`#fff` text pairs (blue #618dff on #0b0b0f fails AA for normal text — must verify/handle) and subtext #a0a0a8 on #121216 (4.5:1+).
- FAIL: any `getElementById`/`querySelector` string in JS no longer matching rendered markup; touch `@media (hover:none)` paths broken; SW-cached stale CSS served (must bump asset version).

## Quality Standards

- Done right: single source of truth tokens; light/dark themes via tokens only; consistent radius/shadow/spacing scales; every interactive element has hover + focus-visible + active affordance; touch targets ≥44px on coarse pointers (already partially done); no layout shift in player bar re-render (it's mounted once — don't re-mount).
- Anti-patterns: rewriting style.css wholesale (breaks patch mode + SW diffing), renaming classes in the allowlist, mixing new hex values outside tokens, adding frameworks/build steps, inline-styling JS templates (keep `el()` class-based).

## Prior Attempt Analysis

No prior executor attempts in this loop (planning in progress, 0 attempts). No known failures to learn from. Related loop learnings (validate-play-music-v116) apply: server serves HEAD only (rebuild needed for live QA), QA hook `window.__player.getState()` exists for programmatic checks (player.js:324), vision not callable in agent context.

## Environment & Integration

Researcher pass 4 (2026-08-07) — how the frontend is served/cached, version mechanics, QA toolchain, live-QA constraints, rebuild flow. Every claim below verified first-hand this pass (served-vs-tree SHA256 comparison, served HTML grep, `go build` run, .playwright-mcp inspection) or taken verbatim from the validated prior loop (validate-play-music-v116_DONE/RESEARCH.md "Web Asset Pipeline & Versioning").

### Context & Prior Work — serving pipeline (verified live 2026-08-07)

- **go:embed at build time**: `web/embed.go` embeds ALL of `web/assets/` (`//go:embed assets`) into the binary. `internal/server/static.go` `handleStatic()` serves it via `http.FileServerFS` (web/assets → `http://localhost:4533/`). **A design change in `web/` files is invisible until rebuild + restart** — nothing is read from disk at runtime (this loop's core constraint, matches global MEMORY.md T3).
- **Version injection**: index.html is read from the embed FS once at boot; `__ASSET_VERSION__` → `version.Version` via `bytes.ReplaceAll` (static.go:28) and the in-memory index is served forever after. `Cache-Control: no-cache` on every static response; unknown routes = SPA fallback to the same in-memory index.
- **Live binary == main tree (verified this pass)**: served `/style.css` SHA256 `4E63DE62...` == main-tree `web/assets/style.css` (exact match); served `/sw.js` == main tree; served index.html carries `?v=1.16.0` on style.css/app.js/pwa.js/sw.js registration. The :4533 server was rebuilt from the CURRENT main tree (post-fix rebuild confirmed — it includes the 4 uncommitted fixes). Design QA against :4533 right now shows the pre-design baseline; after a patch, :4533 shows new CSS **only after the human rebuilds from the main tree**.

### Cache-busting & version mechanics

- **5 `__ASSET_VERSION__` usages, all in `web/assets/index.html`** (verified by grep): line 15 `style.css?v=`, line 18 `version: '__ASSET_VERSION__'` JS constant (surfaces as `window.__APP_CONFIG__.version` via `readAppConfig()` in api.js:190-201 — this is ALSO what admin.js uses to bust its own dynamic import), line 29 `app.js?v=`, line 30 `pwa.js?v=`, line 34 `sw.js?v=` registration. **sw.js itself is version-cache-busted** (its register URL changes when the version changes → guaranteed SW update).
- **Bump mechanics**: `internal/version/version.go` — `const Version = "1.16.0"` (hardcoded, NOT ldflags; prior-loop RESEARCH.md confirmed). Bump = source edit + rebuild; repo convention is one version bump per release commit (v1.14/1.15/1.16 in git log). `/api/settings` also reports it.
- **SW rules (sw.js, read in full)**: `CACHE_NAME='pm-shell-v1'`; precache `SHELL` list = `./`, manifest, 3 icons — **style.css NOT precached**; navigations = network-first (fallback cache); static assets = **stale-while-revalidate** (cache-first, background fetch refreshes); NEVER touches `/api/*` or `/auth/*`; only same-origin GET. `skipWaiting` + `clients.claim` on install/activate; old caches purged.
- **How a CSS change goes live in a PWA**: (1) rebuild+restart from main tree; (2) the `?v=` is UNCHANGED (same `version.Version` unless bumped) so the SW cache key `style.css?v=1.16.0` collides with the old entry — a controlled client shows the stale CSS once, then the background revalidate fixes it from the second navigation; (3) version bump (`1.16.0`→`1.16.1` etc.) makes every `?v=` URL new → cache miss → fresh immediately, and forces the SW itself to update. For loop QA: use fresh playwright contexts after rebuild (SW has no entry yet → always network) or, in a persistent context, clear SW via evaluate (`caches.keys()` + `unregister()` + reload) before asserting visuals.

### Existing Tools & Resources — QA toolchain

- **playwright MCP** (CONFIRMED, .global/TOOLS.md): navigate/snapshot/find/click/type/fill_form/evaluate/console_messages/network_requests/take_screenshot/wait_for/hover/resize — full capability for computed-style verification via `playwright_browser_evaluate` `getComputedStyle(...)` and layout checks (`scrollWidth <= innerWidth`, element rects).
- **Vision subagent verdict — CONFIRMED NOT AVAILABLE in loop subagent contexts**: researcher's own tool set this pass includes NO `task` tool (tools available: bash/edit/glob/grep/playwright*/question/read/skill/todowrite/webfetch/write) — identical to the prior loop's finding (global MEMORY.md T3: "vision subagent is NOT callable inside loop agent contexts"). **Workaround (documented, to reuse)**: programmatic computed-style assertions (`getComputedStyle` on tokens/buttons/rows) + screenshots saved under `loop-stack/frontend-design-overhaul/` with explicit filenames for human/orchestrator review. Screenshots default to `.playwright-mcp/` in the workdir — always pass an explicit filename.
- **`.playwright-mcp/` (root) — REUSABLE**: 168 `page-*.yml` accessibility snapshots + 45 `console-*.log` (2026-08-05 → 2026-08-07T15:21Z), zero images. The .yml files are full accessibility trees per view (free DOM-structure reference for the current baseline); the logs hold console errors. Verified this pass: the PWA install overlay IS captured in one snapshot (page-2026-08-07T06-52-29Z.yml:226-238 — heading "Instale o Play Music", buttons "Instalar" [ref] and "Agora não").
- **PWA install overlay mechanics (pwa.js, read in full)**: shows 1.5s after `load` (`setTimeout(show, 1500)`), EVERY visit in a browser (no localStorage dismissal flag — "mostra sempre, em toda visita"); suppressed only when `display-mode: standalone` or already-rendered. **Dismissal recipe**: wait ≥1.5s, then click the "Agora não" button (`data-act="dismiss"`, removes `.pwa-overlay`) or click the overlay backdrop (handler removes when `e.target === overlay`, pwa.js:106-108). Overlay is z1000 blurred — must be dismissed before snapshots/screenshots of the app UI.

### Requirements & Constraints — live QA and worktree interplay

- **Live-QA constraint**: :4533 serves the embedded build = main tree at last rebuild. Executor design edits (web/ only) are NOT visible until human rebuild+restart. `go build ./...` is unaffected by web/ edits (verified: PASS this pass with current uncommitted state; go:embed does not parse CSS/HTML) — web-only changes can NEVER break the Go build, and `go test ./...` (only internal/phone + internal/stream have tests, confirmed by prior loop) is equally untouched. **Frontend tests: none exist** (no package.json/Node tooling anywhere — RESEARCH.md pass 3; prior loop confirmed `ui/`/node_modules absent) — verification is playwright-based, not unit-test based.
- **Worktree vs main tree decision (verified reasoning)**: AGENTS.md requires a git worktree for every code-changing attempt. BUT web/ files are inert plain text until embedded — editing them in a worktree neither helps nor hurts the live server; the human can only rebuild from the MAIN tree. **Recommended executor flow**: do the design edits in the worktree (AGENTS.md compliance), produce reviewable diffs via `cmd /c "git diff -- web/ > diff.txt"` (LF-safe, per global MEMORY.md — PS `>` writes UTF-16LE and must NOT be used), and the HUMAN applies the worktree diff to the main tree (or the executor copies the changed web/ files over — a plain-file copy, still AGENTS-compliant since the actual editing happened in the worktree). Either way the diff file IS the review artifact (PLAN.md: no auto-commit, human reviews changes).
- **Login flows for QA sessions**:
  - Admin: login UI (app.js login view, app.js:1007-1055) — toggle to "Administrador" → `POST /auth/login { username, password }` (app.js:138) → token saved to `localStorage pm_token` (api.js TOKEN_KEY) → `setAuth` re-renders. Admin creds exist in prior-loop memory (seed admin lives as `admin@playmusic.com`; global MEMORY.md T4) — **NEVER write creds to files**; pull from prior loop context.
  - Throwaway client (global MEMORY.md recipe, endpoints verified in server.go:67 + api.js): `POST /api/store/register {"phone":"<throwaway>"}` → token → `localStorage.setItem('pm_token', token)` → reload; idempotent per phone; re-call with `categoryIds:[<id>]` grants store access; delete the user afterwards.
  - Token header is `X-ND-Authorization: Bearer <token>` (api.js:3); JWT also travels as `?jwt=` for `<img>`/`<audio>` (api.js artworkUrl/streamUrl).
- **Repo state (verified git status this pass)**: 4 uncommitted fixes (`internal/server/helpers.go`, `internal/store/common.go`, `internal/store/users.go`, `web/assets/player.js`) — the design loop must NOT touch/commit/revert them; all web/ DESIGN files (style.css, index.html, loja.html, app.js, admin.js, pwa.js, sw.js) are clean at HEAD cb26fb1b era + uncommitted player.js. Untracked: loop files, `play-music.exe~` (stray exe backup — ignore, do not commit). A local rebuild produces `play-music.exe` in the repo root.

### Suggested Approach

Executors patch web/ files in a git worktree; every change set produces `cmd /c`-written git diffs as review artifacts for the human; human applies to main tree + `go build` + restart :4533; QA then runs playwright against :4533 with a FRESH context (or SW-cleared context), dismissing the PWA overlay (wait 1.5s → click "Agora não") before snapshots; visual assertions are `getComputedStyle`/layout-evaluate based (vision unavailable), with screenshots saved under the loop dir for human review.

### Verification Criteria

- PASS: served style.css SHA256 == main tree after rebuild (curl + Get-FileHash — evidence pattern used this pass); served index.html still carries `?v=1.16.0` (no raw `__ASSET_VERSION__`).
- PASS: `go build ./...` exit 0 after web-only changes (go:embed doesn't validate CSS/HTML — build stays green by construction; run to prove).
- PASS: QA screenshots/snapshots show NO `.pwa-overlay` covering the UI (dismissed); console errors 0.
- PASS: computed-style spot checks return the NEW token values after rebuild (e.g. `getComputedStyle(document.body).getPropertyValue('--accent')`); 320px viewport check `document.documentElement.scrollWidth <= 320`.
- FAIL: any claim of live-visible design change without a rebuild evidence line; `.playwright-mcp` screenshot with no explicit filename; any UI assertion from a SW-stale context; `__ASSET_VERSION__` left raw in served HTML; version const bumped without human approval.

### Quality Standards

- Ground every live assertion in a rebuild + hash-compare (the served==tree SHA pattern above is the standard).
- Diff artifacts always LF (cmd /c redirection) — never PowerShell `>`.
- Design QA = fresh playwright context per build; programmatic style/layout checks primary, screenshots secondary (human review); vision subagent NEVER relied on inside the loop.
- Never write admin creds, tokens, or any secret to files; worktree for all edits; uncommitted Go/player.js fixes preserved untouched.

### Prior Attempt Analysis

No prior executor attempts in this loop. Prior-loop validated facts reused: served==HEAD behavior, QA hook `window.__player` (player.js:324, global MEMORY.md T2) for programmatic player state checks, throwaway-client register recipe, worktree/LF-diff discipline.

## External Knowledge & Resources

Researcher pass 3 (2026-08-07) — External Knowledge domain. Every claim below was read from a real fetched source on 2026-08-07. **Local tooling check (new this pass):** glob confirmed NO `package.json` and NO postcss/sass/stylelint/tailwind/vite/webpack config anywhere in the repo — zero build step, zero design tooling, pure vanilla CSS hand-maintained. Confirms the token architecture must be plain `:root` custom properties with no compile step.

### Sources (fetched 2026-08-07)
1. MDN `clamp()` — Baseline widely available since Jul 2020
2. MDN `prefers-reduced-motion` — Baseline since Jan 2020
3. MDN `:focus-visible` — Baseline since Mar 2022
4. MDN "CSS container queries" guide (container-type/@container/cqw-cqi units, fallback pattern)
5. MDN "OpenType font features" guide (kerning/liga defaults, tabular numerals, font-variant > font-feature-settings)
6. W3C Understanding SC 1.4.11 Non-text Contrast (3:1 UI/focus/graphics; hover+disabled+logos exempt; 2.999:1 fails; F78 focus-hidden failure)
7. W3C Understanding SC 1.4.12 Text Spacing (content must survive line-height 1.5 / letter-spacing 0.12em / word-spacing 0.16em without loss — F104 clipping failure)
8. MDC-Android `docs/theming/Color.md` (Material 3 color roles: dark tonal surface scale, on-role pairing, literal-vs-role naming, luminance-stability advice)
9. W3C ARIA Authoring Practices Guide (APG) patterns index — **confirmed NO media-player pattern exists**; the applicable patterns are **Button** (aria-pressed for toggles) and **Slider** (seek/volume)

### Dark-theme palette & elevation (the Spotify/Apple-Music-ish pattern, vanilla-friendly)

M3's dark scheme (source 8) is the canonical "dark surfaces go LIGHTER with elevation" model — the exact opposite of the app's current flat `#121216` everywhere. Dark baseline tonal ramp: Surface Container **Lowest ≈ neutral4**, **Low ≈ neutral10**, **Surface/Background ≈ neutral6**, **High ≈ neutral17**, **Highest ≈ neutral22**; **On-Surface ≈ neutral90** (light text). Two implementation options, both token-only:
- **Tonal ramp (recommended, simpler):** `--surface-1..-5` stepping ~#121218 → #1b1b20 → #22222a, mapped onto existing components (sidebar/bg = 1, cards/rows = 2, inputs/buttons = 3, modals/player-bar/fullscreen = 4-5). Keep the text stack: on-surface 90-tone ≈ #f5f5f7, secondary ≈ #b0b0b8, tertiary ≥ #8a8a93 (pass-2 AA fix).
- Elevation overlay (legacy M2/M3 mechanism, source 8): lighten surfaces by blending `elevationOverlayColor` (primary) at alpha ∝ elevation — equivalent visual effect, more moving parts; skip.

**Key correctives from pass 2 that this pattern reinforces:** (a) white-on-accent fills still need the darker fill (~#4d7dff ≈ 4.6:1, per pass-2 math) or dark-on-light "container" treatment (M3: dark theme uses light primary-containers + DARK on-container text — a valid alternative to darkening the fill); (b) **borders in dark themes are brighter than the app's #2a2a2a** — M3 dark Outline ≈ neutral_variant60 (~#938F99), which is why the app's 1px borders all fail 3:1 (1.4.11). Raise `--grid`/border tokens to ≥3:1 only where the border is the identifying indicator of a control (checkbox, input, slider track); decorative dividers between content areas may stay subtle BUT the 1.4.11 note says borders are only exempted when the control has another identifying indicator (text/icon) — safest is one `--outline` token ≥3:1 on dark.
- Hover states do NOT need 3:1 (source 6: "pointer position is the indicator") — `--hover` #24242b may stay subtle; focus states DO.
- Naming (source 8): role names for roles (`--surface-2`, `--on-surface`), literal names for palette primitives (`--blue-500`), never name a custom color after a role.

### CSS custom property token architecture (vanilla, zero build)

- **Two layers in `:root` (style.css):** primitives (`--blue-500: #4d7dff` etc.) → semantic (`--surface-1..-5`, `--on-surface`, `--on-surface-sub`, `--on-surface-faint`, `--accent`, `--on-accent`, `--danger`, `--warn`, `--outline`, `--radius-sm/md/lg/pill`, `--space-1..8`, `--shadow-1..3`, `--fs-*`, `--lh-*`, `--dur-fast: 0.15s`, `--dur: 0.2s`, `--ease: cubic-bezier(...)`).
- Theme switching needs NO JS toggle logic beyond one class: redefine the semantic block under a root selector, e.g. `[data-theme="light"] { --surface-1: #fafafa; ... }` — custom properties inherit down the tree (spec: CSS Custom Properties for Cascading Variables, drafts.csswg.org/css-variables). All component CSS stays untouched. `.env` ND_DEFAULTTHEME=Dark means default = current dark block; light is additive-only.
- `var(--x, fallback)` fallback syntax is the safety net — keep fallbacks during migration, remove once defined.
- Map existing literals: radius 4/6/8/10/12/14/16/9999/50% → `--radius-sm/md/lg/pill/round`; 5 literal shadows → `--shadow-1..3`; px sizes 10-48 → `--fs-xs..--fs-3xl`; spacing 2-196 → `--space-1..8` + the 96/196 page-bottom paddings as `--space-page-bottom`.
- loja.html owns a divergent duplicate token block (green accent, #1c1c24) — the token-first fix unifies it (either `<link>` style.css or mirror the new tokens; pass-2 already flags `var(--accent, #1db954)`).

### Typography

- **Fluid type (source 1):** use `clamp()` for display sizes only — `page-title` (30→48px), `detail-title` (36→48), loja h1 (26): e.g. `font-size: clamp(1.875rem, 2.5vw, 3rem)`. Rule from MDN a11y note: **max must be ≥2× min in relative units** (or text can't reach 200% zoom); `rem` bounds, `vw` middle. Body/UI text stays fixed rem (fluid body is not worth it at this scale).
- **Line-height:** 1.35–1.5 for UI text (titles 1.2). Hard requirement (source 7, 1.4.12): content must not clip/overlap when user forces **line-height 1.5, letter-spacing 0.12em, word-spacing 0.16em** (F104 failure). ⚠ This is a real risk in current fixed-height track rows (grid rows + line-clamp 2 + overflow hidden) — executor should prefer `min-height`/auto-height rows or verify clamp handles 1.5 line-height; the 96px bottom bar and fixed card titles are secondary risks. Keep 12px as the floor for labels (11/10px badges are below practical readability; --faint is already an AA fail).
- **Font features (source 5):** kerning + common ligatures are ON by default in OpenType fonts — do nothing. One high-value feature: `font-variant-numeric: tabular-nums` on duration/seek-time text (progress `0:42 / 3:15` etc.) — stops jitter when digits change. Prefer `font-variant-*` over `font-feature-settings` (settings require re-declaring the whole string on change); wrap in `@supports` if used.

### Accessibility (focus / contrast / reduced-motion / ARIA)

- **Focus-visible (sources 3+6):** Baseline 2022 — safe in all target browsers. Add a global rule (currently ONLY inputs/selects have focus styles; buttons/cards/nav/rows have NONE):
  `:focus-visible { outline: 2px solid var(--focus, #8ab0ff); outline-offset: 2px; }` — the indicator must be ≥3:1 against adjacent bg (1.4.11; on #0b0b0f, #8ab0ff ≈ 9:1 ✓). Never `outline: none` on interactive elements (F78). Fallback for exotic browsers: `@supports not selector(:focus-visible) { :focus { ... } }`.
- **Contrast (source 6):** text 4.5:1 / large-text & UI-components & focus-indicators & icons 3:1 / 2.999:1 FAILS (no rounding) / hover exempt / disabled exempt / logos exempt. Pass-2's failing set is the executor's checklist (white-on-accent 3.11, white-on-green 2.59, --faint 3.2-3.7, borders 1.2-1.7).
- **Reduced motion (source 2):** `@media (prefers-reduced-motion: reduce)` placed AFTER base rules (same specificity, later wins): kill the spinner animation, loading-screen, card-play transitions → `animation: none; transition: none;` or ~0.01ms. Baseline 2020.
- **ARIA (source 9):** APG has NO media-player pattern — don't invent one. Player controls = **Button pattern** (`aria-pressed` on play/pause and like toggles; `aria-label` on every icon-only button — prev/play/next/like/volume/shuffle currently are bare `<button>` with SVGs), **Slider pattern** for seek/volume. Best pragmatic move: the progress bar is a native `<input type="range">` already? — verify; native range gives slider semantics + arrow-key support free (app.js renders it; if it's a div, it needs role=slider + aria-valuenow/min/max + keyboard). These are additive JS template changes (safe vs allowlist).

### Responsive / breakpoints

- **Container queries (source 4):** fully supported in target browsers (Chrome 105+, Safari 16+, Firefox 110+; Baseline 2023). BUT this app's layout is viewport-driven (240px sidebar + main, fixed bottom bar) — container queries earn their keep only for card-grid density or track-row column collapse inside narrower pages. **Verdict: keep viewport MQs as primary (existing 640/767/768 are sound); optionally add `container-type: inline-size` on `.page-content` + one `@container` for card wrap — low cost, no rewrite.** Not required for C5.
- Keep the mobile-bottom-player-bar approach; add `padding-bottom: env(safe-area-inset-bottom)` on the bar (PWA standalone on iOS notch) and a `min-width: 320px` guard so nothing breaks below that (C5 gate: no text overlap, no horizontal scroll).
- (hover:none) and (hover:hover) MQs already present are correct patterns — preserve them.

### Webfont verdict

**Keep `system-ui` (current `--font-sans`).** Justification: (1) the app is a self-hosted PWA that must run offline — sw.js caches same-origin only, so Google-Fonts CDN = guaranteed failure offline and FOUT/FOIT risk; (2) zero build step means no font subsetting pipeline; (3) system-ui on Win/Chrome is Segoe UI Variable — visually close to "modern music player" without shipping bytes. IF the human insists on a brand typeface: self-host a single WOFF2 variable font (Inter/Geist latin subset, ~100-300KB), `@font-face` + `font-display: swap`, add the file to sw.js `pm-shell-v1` cache list AND bump `__ASSET_VERSION__` (embed.go). Treat as optional polish, not part of R2 acceptance.

### Suggested Approach (external-knowledge-shaped)

Token-first, in one worktree patch: (1) extend `:root` with primitive+semantic tokens (surface ramp 1-5, on-* pairs, outline, radius/space/shadow/fs/lh/dur scales, --focus); (2) swap flat hexes → tokens across style.css + loja tokens unified; (3) AA fixes per pass-2 list (accent fill #4d7dff or on-accent dark text, --faint ≥ #8a8a93, --outline ≥3:1); (4) global :focus-visible + reduced-motion block at file end; (5) clamp() on the 4 display sizes; tabular-nums on durations; (6) aria-label/aria-pressed additions in app.js templates; (7) safe-area + 320px guard. No new files, no build step, allowlist untouched.

### Verification Criteria (additions for verifier/auditor)

- PASS: contrast recompute (pass-2 script) — all text pairs ≥4.5:1, UI/focus pairs ≥3:1; accent fill pair now ≥4.5:1.
- PASS: grep `:focus-visible` present for buttons/cards/nav/rows (≥1 global rule + no `outline: none` on interactive); `prefers-reduced-motion` block exists AFTER base rules; `var(--focus)` defined in :root.
- PASS: 1.4.12 spot-check via `playwright_browser_evaluate` — inject `line-height:1.5; letter-spacing:.12em; word-spacing:.16em` on a track list: no clipped/overlapping text, no horizontal scroll at 320px.
- PASS: no hard-coded hex outside `:root` primitives (grep hex in style.css body); loja.html no longer defines its own token block; every `var(--x)` resolves (grep fallbacks gone).
- PASS: player controls expose names (aria-label) and toggle state (aria-pressed) — snapshot via playwright accessibility tree.
- FAIL: any of the above missing; new dependency (package.json) introduced; allowlist selector renamed; player bar re-mounted.

### Quality Standards

- Done right = "one source of truth, zero new tooling": every color/radius/shadow/size flows from `:root` tokens; themes = pure token redefinition; focus visible everywhere but unobtrusive; motion capped ≤0.25s and killable by reduced-motion; the app still looks like a modern music player (elevation, not flatness).
- Anti-patterns: sprinkling new hexes inline; white-on-bright-accent text; `transition: all` (perf + motion); animating layout properties (transform/opacity only); `outline: none` without replacement; Google-Fonts CDN link; introducing a build step to "solve" tokens.

## Task-Specific Research — [G1] T1 — Design audit + proposal

Researcher pass 5 (2026-08-07). Goal of this pass: give the executor everything needed to (a) capture before-state evidence (screenshots + computed styles) from live :4533, and (b) write `loop-reports/design-proposal.md` with a full redesign plan. **Critical correction found: the plan's accent target `#4d7dff` FAILS 4.5:1 with white text (3.69:1) — corrected targets below.**

### Context & Prior Work

Everything below re-verified first-hand this pass from `web/assets/style.css`, `web/assets/loja.html`, and live playwright against :4533 (HTTP 200, healthy). Source-verified color values and where each is applied:

| Value | Token/rule | Applies to (source-verified) |
|---|---|---|
| `#0b0b0f` | `--bg` (style.css:2) | body bg, page gradients |
| `#121216` | `--surface` (style.css:3) | sidebar, `.bottom-bar` (style.css:1069 `background: var(--surface)`), cards, modals, login form |
| `#1a1a1f` | `--surface2` (style.css:4) | inputs, dropzones, secondary buttons, fullscreen bg |
| `#2a2a2a` | `--grid` (style.css:5) | borders via `0 0 0 1px` box-shadow, `::-webkit-scrollbar-thumb` (style.css:51), progress track |
| `#24242b` | `--hover` (style.css:6) | hover washes (nav/cards/rows) — **exempt from 3:1** (1.4.11: pointer is the indicator) |
| `#618dff` | `--accent` (style.css:7) | `.btn-accent` fill (style.css:759-768), active states, focus shadows (style.css:963), icons |
| `#7ba0ff` | `--accent-hover` (style.css:8) | **never used in CSS** |
| `#a0a0a8` | `--subtext` (style.css:9) | 43 `color: var(--subtext)` sites (secondary text) |
| `#6b6b73` | `--faint` (style.css:10) | 12px labels only: `.sidebar-nav-label` (195), `.playlists-empty` (229), `.sidebar-user-handle` (261), `.track-list-header` (842), `.form-input::placeholder` (967), + 4 more sites |
| `#1db954` | loja `--accent` (loja.html:14) | `.buy-btn` + `.login-box button` fill w/ #fff text (loja.html:60,89), `.price` (22px/800 text), `.pack-badge`, `.ok`; also admin legacy fallback `var(--accent, #1db954)` (style.css:1750,1766,1806,1827) |
| `#1c1c24` | loja `--surface` (loja.html:11) | loja topbar, `.card` bg — DIVERGENT from app's #121216 |
| `#6b6b76` | loja `--faint` (loja.html:16) | loja `.empty`, `footer` |
| `#26262f` | `.admin-row` border (style.css:1838) | 0.8px solid — measured live `rgb(38, 38, 47)` |
| `#2c2c36` | `.modal` border (style.css:1780,1828) | 1px borders |
| `#3a3a42` | scrollbar hover (style.css:56) | `::-webkit-scrollbar-thumb:hover` |
| `#f87171` | danger fallback `var(--danger, #f87171)` | upload-fail, login-error, remove-track-btn, loja `.msg` |
| `#ffcf6b` | warn badge | admin badge + rgba(255,200,60,0.16) bg |

**Live computed-style evidence captured this pass (playwright evaluate, exact values — reuse this recipe):**
- `.btn-accent` (admin view, "Novo usuário"): `color: rgb(255,255,255)`, `background: rgb(97,141,255)` = #618dff, `font-size: 14px`, `font-weight: 700` → the 3.11:1 pair, live.
- `.bottom-bar`: `background: var(--surface)` → computed `rgb(18,18,22)` (wrapper `#player-bar` itself is transparent — proof the `getComputedStyle` target is `.bottom-bar`, not `#player-bar`).
- `.admin-row`: border `0.8px solid rgb(38,38,47)` = #26262f; `.admin-row-sub` = `rgb(160,160,168)` = #a0a0a8.
- `::-webkit-scrollbar-thumb` background resolves to `var(--grid)` via cssRules scan (getComputedStyle cannot read ::-webkit-scrollbar pseudo — use `document.styleSheets` cssRules scan; confirmed working).
- `:root` vars resolve: `--accent` → `#618dff`, `--grid` → `#2a2a2a`.
- `window.__player.getState()` live: `playing: true`, `progress` ticks, `currentIndex`, native `<input type="range">` inside the bar (slider semantics already native — good for a11y plan).

**Screenshot session state gotcha (verified live):** the playwright context carries `localStorage pm_token` from prior QA → the app opens LOGGED IN as admin (`admin / @admin` in sidebar footer). For the "home logged-out" shot the executor MUST `evaluate localStorage.removeItem('pm_token')` + reload first. Admin route = `#/admin` (hash), reachable directly with the persisted session.

**PWA overlay (re-verified live):** appears 1.5s after load; dismiss = click "Agora não" (button `data-act="dismiss"`, ref via `getByRole('button', { name: 'Agora não' })`) or click overlay backdrop (`e.target === overlay`). Overlay is z1000 blurred — MUST be dismissed before any screenshot.

### Requirements & Constraints

**WCAG 2.1 contrast — CURRENT palette (recomputed this pass with exact formula: R'=((c/255+0.055)/1.055)^2.4 for c/255>0.03928 else c/255/12.92; L=0.2126R'+0.7152G'+0.0722B'; ratio=(L1+0.05)/(L2+0.05); no rounding — 2.999 fails):**

| Pair | Ratio | Verdict |
|---|---|---|
| #fff on #618dff (`.btn-accent`, 14px/700) | **3.11** | FAIL AA normal (large/UI-ok) |
| #fff on #1db954 (`.buy-btn`/`.login-box button`, 14px/700) | **2.59** | FAIL even large-text 3:1 |
| #6b6b73 on #0b0b0f / #121216 / #1a1a1f (12px labels) | 3.72 / 3.54 / 3.28 | FAIL AA normal |
| #6b6b76 on #1c1c24 (loja faint) | 3.22 | FAIL AA normal |
| #2a2a2a vs #0b0b0f / #121216 / #1c1c24 (borders, scrollbar, progress) | 1.37 / 1.30 / 1.18 | FAIL 3:1 UI |
| #26262f vs #121216 (admin-row border) | 1.25 | FAIL 3:1 UI |
| #2c2c36 vs #1c1c24 (modal border) | 1.23 | FAIL 3:1 UI |
| #3a3a42 vs #0b0b0f (scrollbar hover) | 1.74 | FAIL 3:1 UI |
| #24242b vs #121216 (--hover) | 1.21 | EXEMPT (hover = pointer indicator, 1.4.11) |
| PASSING (no action): #fff/#0b0b0f 19.64; #fff/#121216 18.69; #f0f0f4/#121216 16.44; #a0a0a8 7.57/7.20/6.68 (subtext everywhere PASS); #a0a0a8/#1c1c24 6.52; #618dff as TEXT 6.33/6.02; #1db954 as TEXT 7.60/6.54; #f87171 6.76; #ffcf6b 12.80; #000/#fff 21.0; chip text #3ddc84 on composite #14291f 8.61 |

**CORRECTED target palette (this pass's key finding — do NOT use the plan's raw numbers):**
- **#fff on #4d7dff = 3.69:1 — STILL FAILS 4.5:1.** (PLAN.md and RESEARCH pass-2 both claimed ~4.6:1 — wrong.) Correct accent fill: **`#3865f8` (4.79:1 ✓)** or `#2f5fff` (5.00:1 ✓). Architecture: keep `--accent #4d7dff` for TEXT/links/icons (5.33 on #0b0b0f / 5.07 on #121216 / 4.65 on #1b1b20 ✓; ⚠ 4.28 on #22222a — accent-as-text only on surfaces 1–2) and add **`--accent-fill: #3865f8`** for filled buttons; `.btn-accent { background: var(--accent) }` → `var(--accent-fill, var(--accent))`.
- **Green: split text vs fill.** Fill `#15803d` = 5.02:1 white ✓ (plan target CORRECT for buttons). But as TEXT `#15803d` = 3.92 on #0b0b0f / 3.37 on #1c1c24 → FAILS normal text. Keep `#1db954` as green TEXT/icon token (`--accent-2`): 7.60 on bg / 6.10 on ramp-3 ✓ (`.price` 22px/800 = large text, needs 3:1 only ✓; `.ok`/`.pack-badge` at 11–13px need the brighter green). Buy-button fill = `--accent-2-fill: #15803d` (5.02 ✓).
- **Faint `#8a8a93`: PASSES on every ramp** — 5.74 (#0b0b0f) / 5.46 (#121216) / 5.01 (#1b1b20) / 4.61 (#22222a). Plan target correct.
- **Outline: `#6a6a70` FAILS on ramp-3** (2.94 vs #22222a). Use **`#6d6d75`** (3.08/3.34/3.64 — passes all ramps) or `#707078` (3.22/3.49/3.80, safer margin). Plan's "~#6a6a70" only works on surfaces 1–2.
- Focus `#8ab0ff`: 9.11–7.32 ✓ everywhere. Sub `#b0b0b8`: 8.67–7.96 ✓. On-surface `#f5f5f7`: 18.04–14.50 ✓. Elevation ramp #121218→#1b1b20→#22222a all verified compatible with the full text stack.
- Non-negotiables (binding): allowlist 18 IDs + 12 classes + 2 structural + 14 state + 3 data-attrs untouched; NO source edits in this task (report-only); screenshots ONLY under `loop-stack/frontend-design-overhaul/loop-reports/before-*.png`; never write admin creds to files; no vision subagent available (programmatic proof only).

**Screenshot list for the executor (live :4533, fresh context, dismiss overlay first):**

| # | File (relative, under loop-reports/) | View | State prep |
|---|---|---|---|
| 1 | `before-home-1440.png` | Home logged-OUT, 1440×900 | `localStorage.removeItem('pm_token')` → reload → wait 1.5s → click "Agora não" |
| 2 | `before-home-375.png` | Home logged-OUT, 375×667 | same + `resize 375x667` |
| 3 | `before-loja-1440.png` | loja.html, 1440 | navigate `http://localhost:4533/loja.html` (needs store access token — recipe below) |
| 4 | `before-loja-375.png` | loja.html, 375×667 | same |
| 5 | `before-admin-1440.png` | #/admin, 1440 | logged-in session persists; direct hash nav; shot includes `.btn-accent` "Novo usuário" + admin-row borders |
| 6 | `before-admin-375.png` | #/admin, 375×667 | same |
| 7 | `before-playerbar-1440.png` | bottom bar + fullscreen, 1440 | click first "Tocar" (`getByRole('button',{name:'Tocar'}).first()`), wait `window.__player.getState().progress > 0`, shot; then click bar art → fullscreen → second shot optional |
| 8 | `before-playerbar-375.png` | bottom bar, 375×667 | same |

Note: playwright MCP saves relative to server CWD (project root) — pass explicit relative filename `loop-stack/frontend-design-overhaul/loop-reports/before-<name>.png`; if the subdir write fails, fall back to `.playwright-mcp/before-<name>.png` and move/copy after (create `loop-reports/` if missing). Loja store access: re-call `POST /api/store/register {"phone":"<throwaway>"}` then `categoryIds:[<id>]` grant (global MEMORY.md recipe) if the current token lacks categories — if `packsSection`/`catGrid` render empty, that is ALSO valid before-evidence for the proposal (empty state).

**Computed-style proof recipe (VALIDATED live this pass — paste into playwright evaluate):**
```js
() => {
  const cs = sel => { const el = document.querySelector(sel); return el ? (() => { const s = getComputedStyle(el); return { color: s.color, bg: s.backgroundColor, fontSize: s.fontSize, fontWeight: s.fontWeight, border: s.border }; })() : null; };
  const scrollbar = (() => { for (const ss of document.styleSheets) { try { for (const r of ss.cssRules) { if (r.selectorText && r.selectorText.includes('webkit-scrollbar-thumb')) return r.style.background; } } catch(e){} } return 'n/a'; })();
  return {
    accentBtn: cs('.btn-accent'),            // expect rgb(97,141,255)/rgb(255,255,255)
    buyBtn: cs('.buy-btn'),                  // loja page: rgb(29,185,84)/rgb(255,255,255)
    faintLabel: cs('.sidebar-nav-label'),    // 12px labels, rgb(107,107,115)
    bottomBar: cs('.bottom-bar'),            // rgb(18,18,22)
    adminRow: cs('.admin-row'),              // border rgb(38,38,47)
    scrollbarThumbCSS: scrollbar,            // var(--grid)
    vars: { accent: getComputedStyle(document.documentElement).getPropertyValue('--accent').trim(),
            grid: getComputedStyle(document.documentElement).getPropertyValue('--grid').trim(),
            faint: getComputedStyle(document.documentElement).getPropertyValue('--faint').trim() }
  };
}
```
Record these exact rgb values + the ratio math in the proposal's current-state section. Contrast math helper (Node, exact WCAG 2.1 — same formulas used for every ratio in this file): relative luminance per channel as stated above; paste results into a BEFORE table.

### Existing Tools & Resources

- playwright MCP — the screenshot + evaluate workhorse (re-verified live this pass: navigate/snapshot/click/evaluate/resize/wait_for all worked; console 0 errors).
- `window.__player` QA hook (player.js:324) — `getState()` returns full state incl. progress/currentIndex; used to confirm a track is actually playing before the bar screenshot.
- `.playwright-mcp/` accessibility snapshots — free DOM reference for the current baseline (168 .yml files).
- No contrast tooling installed (no npm deps in repo) — the Node one-liner above IS the audit tool; paste it into a scratch file (temp dir, not the repo) if a reusable script is wanted.

### Requirements & Constraints (output shape of design-proposal.md)

The proposal MUST contain all of these sections (per PLAN.md T1): (1) current-state findings incl. the contrast audit table above + computed-style evidence + screenshot references; (2) token architecture — primitives + semantic two-layer in `:root`, `[data-theme="light"]` additive block, every legacy token kept as alias (`--bg/--surface/--surface2/--grid/--hover/--accent/--accent-hover/--subtext/--faint/--font-sans`), `var(--x, fallback)` safety net, define `--text` + `--danger` in :root, NEW tokens `--accent-fill`, `--accent-2`, `--accent-2-fill`, `--outline`, `--focus`, `--on-surface*`, `--radius-*`, `--space-*`, `--shadow-*`, `--fs-*`, `--lh-*`, `--dur-*`; theme-color meta + manifest stay `#0B0B0F`; (3) target palette with the CORRECTED values (ramp #121218→#1b1b20→#22222a, on-surface #f5f5f7, sub #b0b0b8, faint #8a8a93, accent-text #4d7dff, accent-fill #3865f8, green-text #1db954, green-fill #15803d, outline #6d6d75, focus #8ab0ff, danger/warn unchanged); (4) typography — `--fs-xs..3xl` tokens, `clamp()` on the 4 display sizes (page-title 30→48, detail-title 36→48, loja h1 26, login-brand h1 30; max ≥2× min in rem/vw per MDN 200%-zoom rule), lh 1.35–1.5 UI / 1.2 titles, 12px label floor, `font-variant-numeric: tabular-nums`; (5) spacing `--space-1..8` + `--space-page-bottom` (96/196), radius sm/md/lg/pill/round, shadow 1..3; (6) ~29-component redesign list (from RESEARCH.md inventory: sidebar, mobile topbar/overlay, cards, genre-cards, empty-state, tabs, track rows, detail header, track-list-header, section-block, back-link, playlist rows, login screen, search bar, bottom player bar, fullscreen player, settings, upload form, modals, admin rows/chips/badge, PWA popup, queue, spinner, loading-screen, login-toggle, track-add, chip, badge) each with current-bug/contrast note + proposed change; (7) responsive plan (320px guard, safe-area-inset-bottom, 640/767/768 MQs kept); (8) animation plan (≤0.25s, transform/opacity/bg only, `prefers-reduced-motion` block at file end); (9) a11y plan (global `:focus-visible` outline 2px var(--focus) + offset 2px, `@supports not selector(:focus-visible)` fallback, aria-pressed on toggles, aria-label on icon-only buttons, tabular-nums, 1.4.12 line-height resilience); (10) BEFORE/AFTER contrast table — every pair above with target values + verdict.

### Suggested Approach

One playwright session: (1) fresh context → dismiss overlay → clear pm_token → capture home logged-out at 1440 and 375; (2) navigate loja.html → capture 1440/375 (+ ensure store access via throwaway-client recipe if empty); (3) click "Administração" (or hash #/admin) → capture 1440/375; (4) click first "Tocar" → wait progress>0 → capture bar 1440/375 (+ optional fullscreen); (5) run the computed-style proof evaluate on home + loja + admin, copy raw JSON into the proposal; (6) write `loop-reports/design-proposal.md` (structure in "Requirements & Constraints" above) with the exact ratio table from this file. No source edits, no file outside loop-reports/ + RESEARCH/STATUS.

### Verification Criteria

- PASS: 8 screenshots exist under `loop-stack/frontend-design-overhaul/loop-reports/before-*.png` (4 views × 1440/375 incl. player bar with a playing track; 0 console errors across the session; no `.pwa-overlay` in any shot).
- PASS: proposal contains the current-state contrast table with the exact numbers from this file (3.11 / 2.59 / 3.72-3.22 / 1.18-1.74 for the failing 4 groups), computed-style rgb evidence (btn-accent rgb(97,141,255) etc.), and a BEFORE/AFTER table.
- PASS: proposal's target palette uses the CORRECTED values — accent FILL `#3865f8` (or darker, e.g. #2f5fff) NOT `#4d7dff`; outline `#6d6d75`+ NOT `#6a6a70`-as-is on ramp-3; green split text #1db954 / fill #15803d; faint #8a8a93; every target row shows ≥4.5:1 text / ≥3:1 UI.
- PASS: all 10 required proposal sections present (token architecture, palette, typography, spacing/radius/shadow scales, ~29-component list, responsive, animation, a11y, before/after table, current-state findings); legacy tokens preserved as aliases; theme-color/manifest sync note.
- FAIL: `#4d7dff` quoted as an AA-OK white-text fill anywhere (it is 3.69:1); screenshots written to repo root or without explicit filenames; any source file under `web/` modified; admin creds written to any file; contrast table numbers that disagree with this file.

### Quality Standards

- Done right: proposal reads as a build-ready spec — every token has an exact hex + a contrast proof line; every one of the 4 failing groups has a concrete fix with ratio; the component list maps each item to its current CSS section; the BEFORE/AFTER table uses identical pairs (same fg/bg) so the verifier can re-measure in T3.
- Anti-patterns: repeating the plan's uncorrected #4d7dff target; inventing new token names that collide with the allowlist/legacy aliases; proposing component renames (JS-coupled); promising light theme as anything but additive `[data-theme="light"]` (ND_DEFAULTTHEME=Dark); any recommendation to bump `__ASSET_VERSION__` or add dependencies.

### Prior Attempt Analysis

No prior executor attempts in this loop (T1 is first). Prior passes (2-4) contributed the inventory, contrast audit, and environment facts this pass re-verified — with the single correction to the accent-fill target (#4d7dff → #3865f8+) and the outline value (#6a6a70 fails ramp-3; use #6d6d75+). The pass-2 note "accent fill #4d7dff would give ~4.6:1" is superseded by the exact 3.69:1 computation.

## Task-Specific Research — [G2] T2 — Visual system implementation

Researcher pass 6 (2026-08-07). Goal of this pass: give the executor everything needed to implement the token layer + hex→var conversion + clamp typography in `web/assets/style.css` (worktree `../play-music-design-wt`), and give the verifier exact testable criteria. All line numbers below verified against the working-tree file (git-clean). **Three proposal defects found and resolved here: (1) §2.1's :root CSS block is shifted one step vs §3's palette table (surface-1/2/3 conflict) — §3 + loop MEMORY.md win; (2) §2.1 `--hover: var(--gray-800, #24242b)` resolves to #1b1b20, contradicting §3/§10 which keep #24242b — keep #24242b literal; (3) the 4th clamp target (loja h1) lives in `web/assets/loja.html` embedded `<style>`, NOT in style.css — out of T2's scope, defer to T3.**

### Context & Prior Work

- `web/assets/style.css` (2028 lines, git-clean, LF in tree; embedded CRLF into the binary at build — compare content, not bytes). `:root` at lines 1–12 defines the 11 existing tokens (values in `Existing Tools` table above): `--bg #0b0b0f (2)`, `--surface #121216 (3)`, `--surface2 #1a1a1f (4)`, `--grid #2a2a2a (5)`, `--hover #24242b (6)`, `--accent #618dff (7)`, `--accent-hover #7ba0ff (8)`, `--subtext #a0a0a8 (9)`, `--faint #6b6b73 (10)`, `--font-sans (11)`.
- **Undefined-token sites (verified: no `--text:`/`--danger:`/`data-theme` anywhere in style.css):** `var(--text, #f0f0f4)` at 1753 (login-toggle-btn:hover), 1801 (.modal-item), 1816 (.modal-close:hover), 1826 (.modal-check); `var(--danger, #f87171)` at 1576 (.upload-fail). All sit inside the admin/modal block — T2 only needs to DEFINE the tokens in `:root`; the 4 use sites keep their fallback behavior untouched.
- `--accent-hover` (line 8) is defined but never used — keep as alias per proposal (no behavior change).
- **No JS sets inline colors** (grep of app.js/player.js/admin.js for `style.color`/`style.background`/`setProperty`: zero hits) — alias preservation is purely about the 43 `var(--subtext)` + other `var()` refs in CSS.
- loja.html (`web/assets/loja.html`, NOT `web/loja.html`) is fully self-contained: its own `:root` (lines 9–17, `--surface #1c1c24`, `--accent #1db954`, `--faint #6b6b76`) + embedded `<style>` (8–112) and **does NOT link style.css**. All its hexes (L22/48/60/80/89/103/105) are out of T2 scope; the whole block is removed in T3 (unify).

### Existing Tools & Resources

- Verifier workhorse: playwright MCP + `getComputedStyle`/`getPropertyValue` (loop TOOLS.md). Scrollbar pseudo-element must be read via `document.styleSheets` cssRules scan (getComputedStyle can't see `::-webkit-scrollbar-thumb`).
- After rebuild the served CSS is the go:embed copy of the MAIN tree file — the browser never sees worktree edits until the human applies the diff (deploy per SHARED RULES below IS the approved mechanism).
- No CSS linter in repo; the static gates are: `go build -o play-music.exe .` PASS (no Go touched, trivially true) + hex grep + live computed-style checks. Do NOT introduce any build tooling (proposal §4: font stacks unchanged, no deps).

### Requirements & Constraints

- CODE CHANGE ONLY in `web/assets/style.css` (single file). No edits to loja.html/index.html/app.js/player.js; never touch the 4 uncommitted fixes (player.js, helpers.go, common.go, users.go); never `.env`/`auth/`/`payments/`/`secrets/`.
- Hard contract: no selector renamed, none added for components, no rule deleted, no media query altered EXCEPT reconciling the `.detail-title` ≥640px override with clamp (see Clamp mapping). JS-coupled allowlist (18 IDs + 12 classes + 2 structural + 14 state + 3 data-attrs) stays 100% intact.
- All new rules/values use `var(--x, fallback)` with the fallback = the ORIGINAL value at each conversion site (a missing token must never drop a declaration).
- Every legacy token name stays as an alias (11 names above) so every existing `var()` ref resolves.
- `[data-theme="light"]` block is ADDITIVE; default stays dark (ND_DEFAULTTHEME=Dark); `theme-color` meta + manifest stay `#0B0B0F`.
- Deliverables: `loop-reports/design-t2.diff` (via `cmd /c`, never PS `>`), `loop-reports/design-t2.md` (what changed, why, contrast numbers). Max 3 fix attempts per item, then escalate to human.

### Suggested Approach

In the worktree, single pass over style.css: (1) replace the 11-line `:root` with the two-layer block below (aliases FIRST so every downstream ref keeps working; fallback hexes verbatim from current values); (2) append the `[data-theme="light"]` semantic block after `:root`; (3) convert the 26 T2-eligible hex rules to `var(--x, fallback)`; (4) apply clamp + lh tokens to the 3 style.css display sizes; (5) leave everything else byte-identical; then deploy per SHARED RULES (copy style.css to main tree → `go build -o play-music.exe .` → stop :4533 PID → reload .env → start → curl 200), write the .diff/.md deliverables, dispatch verifier.

#### DEPLOY RECIPE (verbatim; worktree + rebuild + restart mechanics)

```powershell
# 1. worktree (from repo root; --detach REQUIRED, non-detached fails when branch is checked out elsewhere)
git worktree add --detach ../play-music-design-wt HEAD
#    ...edit ../play-music-design-wt/web/assets/style.css only...
# 2. copy ONLY the changed file back to the main tree (never player.js/helpers.go/common.go/users.go)
Copy-Item "..\play-music-design-wt\web\assets\style.css" "web\assets\style.css" -Force
# 3. rebuild (workdir = project root)
go build -o play-music.exe .
# 4. stop the running server on :4533
$c = Get-NetTCPConnection -LocalPort 4533 -State Listen | Select-Object -First 1
Stop-Process -Id $c.OwningProcess -Force
# 5. load .env into the NEW process env WITHOUT printing any value
Get-Content .env | ForEach-Object {
  if ($_ -match '^\s*([^#;=][^=]*)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), 'Process')
  }
}
# 6. start + wait for 200 (retry up to ~30s)
Start-Process -FilePath ".\play-music.exe" -WorkingDirectory (Get-Location)
Start-Sleep -Seconds 2
curl.exe -s -o NUL -w "%{http_code}" --max-time 5 http://localhost:4533/   # expect 200
# 7. LF-safe diff artifacts (PS `>` writes UTF-16LE — NEVER use it for git output)
cmd /c "git diff -- web/assets/style.css > loop-reports/design-t2.diff"
# 8. write loop-reports/design-t2.md (what/why/contrast numbers), then dispatch verifier
```
NEVER echo or log `.env` contents or values; the file itself stays untouched (read-only pass). Worktree cleanup when done: `git worktree remove --force ../play-music-design-wt`.

#### EXACT :root to write (primitives + semantic; resolved values in `→`)

```css
:root {
  /* LAYER 1 - PRIMITIVES */
  --blue-400:#7ba0ff;  --blue-500:#618dff;  --blue-600:#4d7dff;  --blue-700:#3865f8;  --blue-800:#2f5fff;  --gray-900:#0b0b0f;  --gray-850:#121218;  --gray-800:#1b1b20;  --gray-750:#22222a;
  --gray-700:#6d6d75;  --gray-600:#8a8a93;  --gray-500:#b0b0b8;  --gray-100:#f5f5f7;
  --green-500:#1db954; --green-700:#15803d;
  --red-400:#f87171;   --amber-400:#ffcf6b; --focus-tone:#8ab0ff;
  /* LAYER 2 - SEMANTIC (dark default) */
  --bg: var(--gray-900, #0b0b0f);                      /* alias -> #0b0b0f (unchanged)          */
  --surface: var(--gray-850, #121216);                 /* alias -> #121218 (was #121216)         */
  --surface2: var(--gray-800, #1a1a1f);                /* alias -> #1b1b20 (was #1a1a1f)         */
  --surface-1: var(--gray-850, #121218);               /* NEW page bg -> #121218                  */
  --surface-2: var(--gray-800, #1b1b20);               /* NEW cards/sidebar/bottom bar -> #1b1b20 */
  --surface-3: var(--gray-750, #22222a);               /* NEW inputs/buttons/rows -> #22222a      */
  --surface-4: var(--gray-750, #22222a);               /* NEW hovered rows/knob host (unused T2)  */
  --surface-5: var(--gray-850, #121218);               /* NEW modals/fullscreen (unused T2)       */
  --text: var(--gray-100, #f5f5f7);                    /* NEW (was UNDEFINED; 4 fallback sites)   */
  --on-surface: var(--gray-100, #f5f5f7);              /* NEW primary text                        */
  --on-surface-sub: var(--gray-500, #b0b0b8);          /* NEW secondary text                      */
  --on-surface-faint: var(--gray-600, #8a8a93);        /* NEW 12px labels                         */
  --subtext: var(--on-surface-sub);                    /* alias -> #b0b0b8 (was #a0a0a8)          */
  --faint: var(--on-surface-faint);                    /* alias -> #8a8a93 (was #6b6b73) FIX 4.5:1*/
  --grid: var(--gray-700, #6d6d75);                    /* alias -> #6d6d75 (was #2a2a2a) FIX 3:1  */
  --hover: #24242b;                                    /* alias, LITERAL (proposal slip: gray-800 would resolve #1b1b20) */
  --accent: var(--blue-600, #4d7dff);                  /* alias -> #4d7dff TEXT-only token        */
  --accent-hover: var(--blue-400, #7ba0ff);            /* alias -> #7ba0ff (unused, kept)         */
  --accent-fill: var(--blue-700, #3865f8);             /* NEW filled-button bg (unused T2)        */
  --on-accent: #fff;                                   /* NEW                                    */
  --accent-2: var(--green-500, #1db954);               /* NEW green text (unused T2)              */
  --accent-2-fill: var(--green-700, #15803d);          /* NEW green fill (unused T2)              */
  --outline: var(--gray-700, #6d6d75);                 /* NEW control borders (unused T2)         */
  --focus: var(--focus-tone, #8ab0ff);                 /* NEW focus ring (unused T2)              */
  --danger: var(--red-400, #f87171);                   /* NEW (was UNDEFINED; 1 fallback site)    */
  --warn: var(--amber-400, #ffcf6b);                   /* NEW                                    */
  /* typography (§4) */ --fs-xs:12px; --fs-sm:13px; --fs-md:14px; --fs-base:16px; --fs-lg:18px; --fs-xl:20px; --fs-2xl:24px; --fs-3xl:30px;
  /* line-height (§4) */ --lh-ui:1.45; --lh-title:1.2;
  /* spacing (§5) */ --space-1:4px; --space-2:8px; --space-3:12px; --space-4:16px; --space-5:20px; --space-6:24px; --space-7:32px; --space-8:40px; --space-page-bottom:96px;
  /* radius (§5) */ --radius-sm:6px; --radius-md:10px; --radius-lg:14px; --radius-pill:999px; --radius-round:50%;
  /* shadow (§5) */ --shadow-1:0 1px 3px rgba(0,0,0,0.4); --shadow-2:0 10px 15px -3px rgba(0,0,0,0.4); --shadow-3:0 25px 50px -12px rgba(0,0,0,0.8);
  /* motion (§8) */ --dur-fast:0.15s; --dur:0.2s; --dur-slow:0.25s; --ease:cubic-bezier(0.4,0,0.2,1);
  --font-sans: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;  /* alias, verbatim */
}
```

NOTES (do not deviate): (a) surface-1..3 use the §3/MEMORY ramp #121218→#1b1b20→#22222a — proposal §2.1's `--surface-1: var(--gray-900, #0b0b0f)` / `--surface-2: gray-850` / `--surface-3: gray-800` is the shifted-by-one slip; §3 contrast rows (4.61/5.01/5.74, 3.08/3.64/3.83) are keyed to this ramp; (b) `--hover` stays literal #24242b (1.21:1, EXEMPT per 1.4.11); (c) `--grid`'s fallback is the ORIGINAL #6d6d75 of the alias's new semantic value — wait, no: fallback = old value where a rule's fallback matters. For legacy alias `--grid` the resolved value legitimately CHANGES to #6d6d75 (that IS the token-layer fix). Fallback hexes only protect against an undefined token — since we define it, fallback is cosmetic; still write `#6d6d75` (new value) as fallback per proposal §2.1 so all layers agree. (d) `--bg` unchanged #0b0b0f; page bg does NOT move to #121218 in T2 (surface-1 is defined but unused until T3).

#### [data-theme="light"] block (additive, semantic-only redefinition; verbatim from proposal §2.2)

```css
[data-theme="light"] {
  --bg:#f5f5f7; --surface:#ffffff; --surface2:#e9e9ee;
  --surface-1:#f5f5f7; --surface-2:#ffffff; --surface-3:#e9e9ee; --surface-4:#e0e0e6; --surface-5:#ffffff;
  --on-surface:#1a1a1f; --on-surface-sub:#4a4a55; --on-surface-faint:#666670;
  --text:#1a1a1f; --subtext:#4a4a55; --faint:#666670;
  --grid:#6d6d75; --hover:rgba(0,0,0,0.05);
  --accent:#2f5fff; --accent-fill:#2f5fff; --on-accent:#fff; --outline:#6d6d75; --focus:#2a5df2;
}
```
Light ratios verified in proposal §10.3 (on-surface 15.92, sub 8.03, faint 5.21, accent-text 4.59, outline 4.71 — all PASS).

#### Rules to convert in T2 — 26 target hexes (main surface/text/border; GLOBAL only)

All → `var(--X, <original-hex>)`. `#fff`→`var(--text, #fff)`; `#f87171`→`var(--danger, #f87171)`; `#3a3a42`→`var(--outline, #3a3a42)`; body → `var(--text, #ffffff)`.

| # | Line | Selector | Hex | Token |
|---|---|---|---|---|
| 1 | 36 | `body` | `#ffffff` | `--text` |
| 2 | 56 | `::-webkit-scrollbar-thumb:hover` | `#3a3a42` | `--outline` |
| 3 | 154 | `.sidebar-close:hover` | `#fff` | `--text` |
| 4 | 178 | `.nav-link:hover` | `#fff` | `--text` |
| 5 | 182 | `.nav-link.active` | `#fff` | `--text` |
| 6 | 218 | `.playlist-link:hover` | `#fff` | `--text` |
| 7 | 222 | `.playlist-link.active` | `#fff` | `--text` |
| 8 | 275 | `.icon-btn:hover` | `#fff` | `--text` |
| 9 | 491 | `.tab-btn:hover` | `#fff` | `--text` |
| 10 | 744 | `.detail-meta .link` | `#fff` | `--text` |
| 11 | 872 | `.back-link:hover` | `#fff` | `--text` |
| 12 | 897 | `.remove-track-btn:hover` | `#f87171` | `--danger` |
| 13 | 956 | `.form-input` | `#fff` | `--text` |
| 14 | 973 | `.login-error` | `#f87171` | `--danger` |
| 15 | 1020 | `.search-type-select` | `#fff` | `--text` |
| 16 | 1035 | `.search-type-select option` | `#fff` | `--text` |
| 17 | 1046 | `.search-input` | `#fff` | `--text` |
| 18 | 1146 | `.now-playing-artist:hover` | `#fff` | `--text` |
| 19 | 1176 | `.player-btn:hover` | `#fff` | `--text` |
| 20 | 1351 | `.fullscreen-artist:hover` | `#fff` | `--text` |
| 21 | 1404 | `.settings-text strong` | `#fff` | `--text` |
| 22 | 1431 | `.settings-playlist-item` | `#fff` | `--text` |
| 23 | 1510 | `.upload-dropzone-title` | `#fff` | `--text` |
| 24 | 1581 | `.upload-fail strong` | `#fff` | `--text` |
| 25 | 1597 | `.btn-secondary:hover` | `#fff` | `--text` |
| 26 | 2022 | `.pwa-btn:hover` | `#fff` | `--text` |

**DO NOT TOUCH (component-specific — T3's job, incl. all 4 contrast-fix fills):** L399 `.card-play` color #fff; L495-496 `.tab-btn.active` #fff/#000; L538 `.track-play-btn`; L768 `.btn-accent` #fff; L983 `.login-submit` #fff; L1190-1191 `.player-btn-main` #fff/#000; L1240+L1253 `.progress-fill` #fff; L1283 `.volume-slider` accent-color #fff; L1368-1369 `.fullscreen-btn-main` #fff/#000; L1779-1780+L1828 `.modal` #1c1c24/#2c2c36; L1838 `.admin-row` #26262f; L1853 `.chip` #3ddc84; L1864 `.badge` #ffcf6b; L1945 `.pwa-icon` #0b0b0f; L2027 `.pwa-btn.primary` #0b0b0f. **rgba() values (not hex) also deferred:** L101/908 gradients, L122/233/1075 rgba(42,42,42,.5) borders, L288 mobile-topbar rgba(11,11,15,.95), L509 track-list rgba(18,18,22,.3), L1494/1766/1800/1816 rgba washes, L1771 overlay, L1918 pwa-overlay. **Admin/modal block var(--x, fallback) sites (1733-1864) already tokenized — leave as-is.** loja.html hexes all deferred to T3.

#### Clamp mapping (4 display sizes; 3 in style.css + 1 deferred)

| Site | Current | Target clamp | Where |
|---|---|---|---|
| `.page-title` (style.css:335-339, `font-size:30px`) | 30px | `clamp(30px, 3vw + 1rem, 48px)` | style.css |
| `.detail-title` (style.css:727-733, `font-size:36px; line-height:1.1`) | 36px | `clamp(36px, 3.5vw + 1rem, 48px)`; `line-height: var(--lh-title, 1.2)` | style.css |
| `.detail-title` @media(min-width:640px) (style.css:818-820, `font-size:48px`) | 48px | same clamp as base (max is already 48px — replace the literal with the clamp so it can't exceed max; or delete the override) | style.css |
| `.login-brand h1` (style.css:928-933, `font-size:30px`) | 30px | `clamp(30px, 2.5vw + 1rem, 40px)` | style.css |
| loja `h1` (loja.html:64, embedded `<style>` `h1 { font-size:26px }`) | 26px | `clamp(26px, 2.5vw + 0.9rem, 42px)` — **DEFERRED to T3** (loja unify removes the embedded block; style.css-only scope forbids touching loja.html in T2). State this explicitly in design-t2.md. | loja.html (T3) |

Apply `--lh-title` to the 3 style.css display titles and `--lh-ui` to `body` (line 34-40; currently no line-height). Font-weight literals (700/500/600) may stay literal or move to optional `--fw-*` tokens (PLAN says "size/weight tokens"; proposal only specifies size — adding `--fw-regular/medium/semibold/bold` is additive and safe, but NOT required for PASS).

### Verification Criteria

**A. Static gates (verifier, on the diff + tree):**
1. Grep `#(?:[0-9a-f]{3}|[0-9a-f]{6})\b` in style.css (case-insensitive): remaining matches MUST all be inside (i) the `:root`/`[data-theme="light"]` token definitions, (ii) the T3-deferred component list above (L399/495/496/538/768/983/1190/1191/1240/1253/1283/1368/1369/1751/1853/1864/1945/2027), (iii) `var(--x, fallback)` fallbacks. No match outside these. (The 26 T2 rows above must all be tokenized.)
2. All 11 legacy names present in `:root` as aliases: `--bg --surface --surface2 --grid --hover --accent --accent-hover --subtext --faint --font-sans`.
3. `--text:` and `--danger:` now defined in `:root`; `[data-theme="light"] {` block exists AFTER `:root`; it redefines only semantic tokens (no primitives).
4. No selector renamed/added/removed, no media query altered except `.detail-title` ≥640px; `git diff --stat` shows exactly `web/assets/style.css` changed (plus loop artifacts).
5. clamp() present on `.page-title`, `.detail-title` (base AND the ≥640 MQ line), `.login-brand h1`; loja h1 clamp correctly ABSENT (deferred — documented in design-t2.md).
6. `go build -o play-music.exe .` PASS; `node --check` not applicable (no JS touched — trivially unchanged); `git diff` (cmd /c) has no unrelated hunks.

**B. Live gates (after deploy; fresh playwright context; dismiss PWA overlay 1.5s → "Agora não"):**
7. `getPropertyValue` on `document.documentElement`: `--accent`→`#4d7dff`, `--surface-2`→`#1b1b20`, `--grid`→`#6d6d75`, `--faint`→`#8a8a93`, `--text`→`#f5f5f7`, `--danger`→`#f87171`, `--surface`→`#121218`.
8. `getComputedStyle(document.querySelector('.bottom-bar')).backgroundColor` → `rgb(18, 18, 24)` (= #121218 ramp token). `getComputedStyle(document.body).color` → `rgb(245, 245, 247)` (= --text). `getComputedStyle(document.querySelector('.page-title')).fontSize` → a clamp-computed px ≤48px and ≥30px; `.detail-title` ≤48px ≥36px; `.login-brand h1` ≤40px ≥30px (values at viewport ≥1000px).
9. Scrollbar: cssRules scan finds `::-webkit-scrollbar-thumb` with `background: var(--grid)`.
10. `playwright_browser_console_messages` level=error → 0 entries.
11. Contrast spot checks (computed pairs, node or manual): faint #8a8a93 on #121218 = 5.45 ≥4.5 ✓; grid/outline #6d6d75 vs #121218 = 3.83 ≥3 ✓ — these two fixes LAND IN T2 via alias values.
12. **INTERIM ACCEPTANCE (do NOT fail T2 on this):** `.btn-accent`/`.login-submit`/`.card-play` fills resolve to `--accent` = #4d7dff → 3.69:1 STILL FAILS white text in T2; `.buy-btn` untouched (loja). The T3 task converts fills to `--accent-fill` #3865f8 (4.79) / `--accent-2-fill` #15803d (5.02). T2's PASS = tokens defined + surfaces/text/borders tokenized; the 4 contrast FIXES belong to T3. Any verifier report must state this expectation explicitly.

FAIL conditions: any raw hex outside the allowlist above; missing legacy alias; `--text`/`--danger` still undefined; `data-theme` block missing or overriding primitives; clamp applied to loja h1 in style.css; any selector rename; `.detail-title` MQ left as conflicting literal 48px while base is clamp (cascade keeps 48px at ≥640 — acceptable only if deliberate, but the research-specified reconciliation is the same clamp); console errors; served CSS ≠ main-tree style.css after rebuild.

### Quality Standards

- Done right: style.css reads as "tokens at top, everything else a pure consumer" — the :root block groups primitives/semantic with short section markers mirroring proposal §2.1; zero layout/geometry/selector changes; every converted rule keeps its original hex as the `var()` fallback; clamp expressions are the proposal's exact recipes (min ≥ 2× under max in px, per MDN 200%-zoom guideline); light theme exists but changes NOTHING until a toggle ships.
- Anti-patterns: converting component fills (they flip in T3 — converting them twice churns the diff); touching loja.html; renumbering/shifting rules; wrapping `rgba()` values in tokens via color-mix (no CSS4 tricks); using `!important`; adding comments beyond the section markers; bumping `__ASSET_VERSION__` (version stays 1.16.0 — the SW revalidate path covers the refresh; fresh playwright contexts sidestep caching).
- Interim-visual note for the human reviewer: after T2 the app will look SLIGHTLY different even without T3 (borders/scrollbar lighter #6d6d75, faint labels lighter #8a8a93, subtext #b0b0b8, hovered-rows/knob fill #4d7dff on accent fills). That is the token layer working as designed; T3 completes the component polish.

### Prior Attempt Analysis

No prior executor attempt for T2 (T1 was report-only; no failures). Risk register from this pass: (1) proposal §2.1/§3 ramp conflict — resolved above (surface-1..3 = #121218/#1b1b20/#22222a); (2) `.detail-title` dual font-size (base 36 + ≥640 MQ 48) — must reconcile both or the MQ overrides the clamp; (3) loja h1 clamp out of scope — flagged, deferred to T3, must be documented in design-t2.md so the deliverable doesn't look incomplete; (4) PS `>` writes UTF-16LE — all diff artifacts via `cmd /c`; (5) the playwright context's persisted pm_token/PWA overlay gotchas from T1 apply to all live checks.

## Task-Specific Research — [G3] T3 — Layouts & components redesign

Researcher pass 7 (2026-08-07). Goal: give the executor the full component→selector map for the ~29-component proposal pass + the 4 contrast fixes + loja.html unification, with exact CURRENT line numbers (post-T2 file, 2069 lines, verified by direct read this pass) and the verifier's testable criteria. **Two live verifications done this pass: `.vol-icon` is VISIBLE (invisible-icon bug DISPROVEN) and loja.html CAN link style.css — but `?v=__ASSET_VERSION__` will NOT be substituted there (verified in static.go).**

### Context & Prior Work

**Current style.css layout (2069 lines, post-T2, tokens at 1-41, `[data-theme="light"]` at 43-50):** component sections — Layout 122-152, Sidebar 154-315, Mobile topbar 317-366, Cards 368-510, Tabs 512-543, Track rows 545-710, Detail headers 712-899, Back-link/playlist rows 901-938, Login 940-1034, Search 1036-1106, Bottom bar 1108-1325, Fullscreen 1327-1415, Settings 1417-1478, Upload 1480-1676, Queue 1678-1691, Spinner 1693-1717, Misc 1719-1766, Admin/modals/login-toggle 1769-1907, Touch MQ 1909-1947, PWA popup 1949-2069.

**T3-deferred inventory (from loop MEMORY.md T2-exec, must land in T3):** component fills → `--accent-fill #3865f8` (white text 4.79:1), loja green buttons → `--accent-2-fill #15803d` (5.02:1), rgba() washes tokenized, admin/modal inline hexes + var() fallbacks polished, loja.html embedded-style hexes removed + its h1 → `clamp(26px, 2.5vw + 0.9rem, 42px)`.

**Component → selector map (30 rows, current line refs + current value → target per proposal §6; allowlist-safe, no renames):**

| # | Component | Selector (style.css lines) | Current → Target |
|---|---|---|---|
| 1 | App shell | `.main-area` 136-141 (gradient `rgba(26,26,31,0.6)`→`var(--bg)`), `.page` 143-152 (96/196px bottom — keep) | tokenize gradient washes (rgba list below); body stays `var(--bg)` (row 1: gradient header accents, keep base) |
| 2 | Sidebar | `.sidebar` 156-163 (`var(--surface)` → `var(--surface-2, #121218)`; border-right `rgba(42,42,42,.5)` → `var(--outline, ...)` soft border token); `.sidebar-footer` 271-274 same border; `.sidebar-playlists-label` 225-235, `.playlists-empty` 265-269, `.sidebar-user-handle` 295-301 (12px `var(--faint)` — ALREADY #8a8a93 ✓ via T2 alias, no-op or explicit); `.nav-link` 203-214 + hover 216 (bg `var(--hover)` ✓), `.icon-btn` 303-315 | surface-2 bg; border soft token; faint labels already pass (4.5:1) |
| 3 | Mobile topbar + overlay | `.mobile-topbar` 319-329 (`rgba(11,11,15,0.95)` bg), `.mobile-overlay-backdrop` 354-358 (`rgba(0,0,0,0.6)`), `.mobile-sidebar` 360-366 (`var(--surface)` → `--surface-2`, shadow `0 25px 50px -12px rgba(0,0,0,0.6)` → `--shadow-3`) | scrim token; surface-2; `--shadow-3`; ≤0.25s transform/opacity transitions (slide+fade) |
| 4 | Cards | `.card` 398-410 (`var(--surface)` → `--surface-2`; radius 8 → `--radius-md 10px`? proposal: 10px radius + `--outline` 1px border + hover lift translateY(-2px)); `.card-play` 428-444 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`**; color #fff stays; shadow → `--shadow-2`; `transition: all 0.15s` at 443 — `all` is an anti-pattern, split to bg/transform) | flip + tokens |
| 5 | Genre cards | `.genre-card` 469-488 (gradient `rgba(97,141,255,0.4)`→`var(--grid)` ✓ T2), hover scale 1.05 | keep gradients; add `--outline` inset ring; hover scale ≤0.2s |
| 6 | Empty state | `.empty-state` 496-510 (`var(--subtext)` — fine; pass) | faint copy → `--on-surface-sub`; (`.playlists-empty` is the 12px faint one — already fixed via alias) |
| 7 | Tabs | `.tab-btn` 520-537 (active `#fff/#000` 21:1 ✓ keep) | focus ring `--focus` (T4-ish but ring rule may land here); radius `--radius-pill` (already 9999) |
| 8 | Track rows | `.track-row` 553-561 (separators? none inline — hover `var(--hover)` ✓); `.track-play-btn` 576-579 (`#fff` icon on hover — keep); `.track-duration` 678-684 (tabular-nums ✓); MQ 640 686-710 | `:focus-visible` ring; `--fs-sm` metadata (optional) |
| 9 | Detail header | `.detail-header` 714-720 (gradient `rgba(42,42,42,0.4)` soft token); `.detail-art` 727-734, `.detail-art-icon` 740-751 (shadow `0 25px 50px -12px rgba(0,0,0,0.8)` → `--shadow-3`; icon gradient `var(--accent)`→`rgba(97,141,255,0.4)` keep+token); `.detail-title` 767-773 clamp ✓ done | shadow token; back-link focus ring |
| 10 | Track-list-header | `.track-list-header` 871-883 (12px `var(--faint)` ✓; border-bottom `rgba(42,42,42,.5)` soft token) | faint ✓; tabular-nums already on `.track-duration` |
| 11 | Section-block | `.section-block h2` 895-899 (20px) | `--fs-lg`/`--lh-title` tokens (optional) |
| 12 | Back-link | `.back-link` 901-913 | keep icon; focus ring (T4 aria-label) |
| 13 | Playlist rows | `.playlist-track-row` 915-934; `.remove-track-btn` 926-938 (danger ✓ via `var(--danger, #f87171)`) | separators `--outline`; danger token ✓ |
| 14 | Login screen | `.login-screen` 942-949 (gradient soft); `.login-brand h1` 968-974 clamp ✓; `.login-form` 982-990 (`var(--surface)` → `--surface-2`, shadow → `--shadow-2`-style or keep recipe); `.form-input` 992-1001 (`var(--surface2)` → `--surface-3`?, shadow `0 0 0 1px var(--grid)` → `--outline`); `.login-submit` 1017-1034 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`**, color #fff → 4.79 ✓) | flip + tokens |
| 15 | Search bar | `.search-type-select` 1052-1068, `.search-input` 1079-1089 (`var(--surface)` bg; `0 0 0 1px var(--grid)` → `--outline`), placeholders 1095-1097 + `.search-icon` 1099-1106 (`var(--faint)` ✓) | `--surface-3` bg per proposal row 15; `--outline` border |
| 16 | Bottom player bar | `.bottom-bar` 1110-1119 (**bg `var(--surface)` → `var(--surface-2, #121218)`**; border-top `rgba(42,42,42,.5)` soft token; safe-area ✓ already present 1118); `.progress-track` 1263-1273 (`var(--grid)` ✓ = #6d6d75 ≥3:1); `.progress-fill` 1275-1284 + `::after` 1286-1297 (**bg `#fff` — flip to `var(--accent-fill, #3865f8)`? hover/drag state 1299-1302 already `var(--accent)` → unify to `--accent-fill`**; knob = the 12px ::after); `.volume-slider` 1322-1325 (**`accent-color: #fff` → `var(--accent-fill, #3865f8)`** — listed in deferred rgba/wash item); `.player-btn-main` 1224-1233 `#fff/#000` — **21:1 PASS, KEEP WHITE** (flip-list mention is a categorization slip — see note) | surface-2 + fill tokens |
| 17 | Fullscreen player | `.fullscreen-player` 1329-1337 (gradient `var(--surface2)`→`var(--bg)` → keep/tokens); `.fullscreen-art` 1355-1362 (shadow → `--shadow-3` ✓ recipe matches); `.fullscreen-title` 1372-1379 (24px → clamp 24→32 per row 17, optional); `.fullscreen-btn-main` 1402-1415 `#fff/#000` — **21:1 PASS, KEEP WHITE** (same slip note) | shadow token; title clamp optional |
| 18 | Settings | `.settings-card` 1425-1430 (`var(--surface)` → `--surface-2`), `.settings-playlist-item` 1462-1478 (`var(--surface2)`; hover `box-shadow 0 0 0 1px var(--grid)` ✓) | surface-2 + outline separators |
| 19 | Upload form | `.upload-dropzone` 1516-1530 (`1.5px dashed var(--grid)` → `var(--outline)`; `var(--surface2)`), hover/drag 1532-1536 (bg `rgba(29,185,84,0.06)` green wash → token), `.upload-dropzone.error` 1538-1540 (`rgb(248,113,113)` → `var(--danger, #f87171)`), `.upload-photo-drop` 1560-1575 (same border), `.upload-progress-fill` 1594-1599 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`**), `.upload-fail` 1614-1619 (danger ✓), `.btn-secondary` 1625-1639 (`var(--surface2)`) | outline borders + accent-fill progress |
| 20 | Modals | `.modal-overlay` 1809-1818 (`rgba(8,8,12,0.72)` → scrim token); `.modal` 1819-1831 (**bg `#1c1c24` → `var(--surface-5, #1c1c24)`; border `1px solid #2c2c36` → `var(--outline, #2c2c36)`**; radius 14 ✓ `--radius-lg`); `.modal-scroll` 1869 (**border `#2c2c36` → `var(--outline, #2c2c36)`**); `.modal-item` 1836-1846 (washes `rgba(255,255,255,0.05/0.1)` → wash tokens); `.modal-close:hover` 1857 same | surface-5 + outline |
| 21 | Admin toolbar | `.admin-toolbar` 1871; `.btn-accent` 799-819 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`**, color #fff → 4.79 ✓ — FAIL group 1 FIX) | flip |
| 22 | Admin rows | `.admin-row` 1873-1882 (**border `1px solid #26262f` → `var(--outline, #26262f)`** — FAIL group 4 FIX; bg `rgba(255,255,255,0.04)` wash token); `.admin-row-sub` 1885 (`var(--subtext, #9a9aa5)` — keep fallback) | outline border |
| 23 | Chips | `.chip` 1888-1895 (**11px → 12px floor**; bg `rgba(29,185,84,0.14)` → green-wash token; color `#3ddc84` → `var(--accent-2, #1db954)` — PASSES 8.6:1 either way; NOT a fill — no --accent-2-fill flip) | text token + 12px |
| 24 | Badge | `.badge` 1896-1907 (**10px → 12px floor**; bg `rgba(255,200,60,0.16)` → warn-wash token; color `#ffcf6b` → `var(--warn, #ffcf6b)`) | text token + 12px |
| 25 | PWA popup | `.pwa-overlay` 1951-1961 (`rgba(0,0,0,0.55)` → scrim token, blur keep); `.pwa-card` 1963-1973 (`var(--surface)` → `var(--surface-5, #121218)`; border `var(--grid)` ✓; shadow `0 20px 60px rgba(0,0,0,0.5)` → keep/`--shadow-3`); `.pwa-icon` 1981-1991 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`; color `#0b0b0f` → `var(--on-accent, #fff)`**); `.pwa-btn.primary` 2066-2068 (**bg `var(--accent)` → `var(--accent-fill, #3865f8)`; color `#0b0b0f` → `var(--on-accent, #fff)`**) | surface-5 + accent-fill |
| 26 | Queue | `.queue-current-marker` 1680-1686 (`var(--accent)` ✓), `.queue-track-wrap` 1688-1691 | separators `--outline`; marker keep |
| 27 | Spinner | `.spinner` 1703-1711 (**border-top-color `var(--accent)` → `var(--accent-fill, #3865f8)`**; track `var(--grid)` ✓) | accent-fill |
| 28 | Login toggle | `.login-toggle` 1771-1778 (`var(--surface2, #1a1a21)` fallback), `.login-toggle-btn.active` 1790-1793 (bg `var(--accent, #1db954)`, color `#0b0b0f` — note: **#0b0b0f on #4d7dff = 5.34:1 PASSES today**; on fallback #1db954 = 7.8:1 PASSES; NOT a contrast failure) | proposal row 28 wants tab-btn treatment (`#fff/#000`); OR bg `var(--accent-fill)` + color `var(--on-accent)`. Pick ONE — both AA-clean |
| 29 | Track-add | `.track-add` 1796-1807 (hover color `var(--accent, #1db954)` → `var(--accent-2, #1db954)`; hover bg `rgba(255,255,255,0.06)` wash token) | green hover text token (proposal's "--accent-2-fill bg" = solid green hover wash — recommend text-color + wash bg instead; flag to executor) |
| 30 | loja unify | see loja section below | full port |

**Accent-fill flip list (selectors with WHITE text/icons on accent bg — the real 4.5/3:1 fixes):** `.btn-accent` (799-810, 14px/700 #fff → 3.11 FAIL), `.login-submit` (1017-1026, #fff → 3.11 FAIL), `.card-play` (428-444, white icon 36px — 3.69 ≥3:1 UI passes but flips for system consistency), `.pwa-btn.primary` (2066-2068), `.pwa-icon` (1981-1991), `.upload-progress-fill` (1594-1599), `.spinner` border-top (1708), `.volume-slider` accent-color (1324), `.progress-fill` (+`::after`) (1281/1294) — all → `var(--accent-fill, #3865f8)` with text/icons → `var(--on-accent, #fff)`. **DO NOT flip: `.player-btn-main` (1224-1233) and `.fullscreen-btn-main` (1402-1411) — white-fill/black-icon 21:1 PASS, keep (T2 MEMORY's flip-list mention of player-btn-main is a categorization slip; verified against proposal §6 which only flips white-on-accent FAILs).**

**Green split (loja + admin fallbacks):** fills → `var(--accent-2-fill, #15803d)`; green TEXT/icons stay `var(--accent-2, #1db954)` (7.6:1). Sites: loja `.login-box button` (loja.html:60) + `.buy-btn` (loja.html:89) — white text on green 2.59 FAIL → fill flip = 5.02 ✓ (FAIL group 2 FIX). Admin fallback sites already use `var(--accent, #1db954)` — semantically green → repoint to `var(--accent-2, #1db954)`: `.modal-item.new` (1847), `.modal-check input` accent-color (1868), `.track-add:hover` (1807). **`.chip`/`.badge` get TEXT tokens + 12px floor — NOT fills** (proposal rows 23-24; MEMORY's "--accent-2-fill on .chip/.badge" is a lumping slip).

**rgba()/rgb() wash inventory (MEMORY: "rgba() washes → tokenized"):** borders/soft `rgba(42,42,42,0.5)` at 161 (sidebar), 272 (sidebar-footer), 875 (track-list-header), 1116 (bottom-bar); gradients 140 (main-area `rgba(26,26,31,0.6)`), 477 (genre-card), 718 (detail-header), 948 (login-screen), 1335 (fullscreen-player); scrims 357 (`rgba(0,0,0,0.6)`), 1812 (`rgba(8,8,12,0.72)`), 1959 (`rgba(0,0,0,0.55)`); washes 327 (`rgba(11,11,15,0.95)` topbar), 549 (track-list `rgba(18,18,22,0.3)`), 1535 (dropzone green `rgba(29,185,84,0.06)`), 1807/1841/1846/1857 (`rgba(255,255,255,0.04-0.1)`), 1878 (admin-row 0.04), 1893 (chip 0.14), 1904 (badge 0.16); 493 genre-count `rgba(255,255,255,0.7)`; shadows 440/733/750/989/1361/1972 → `--shadow-1..3`. Recommended NEW tokens (all in :root, primitives-adjacent): `--border-soft: rgba(42,42,42,0.5)`, `--scrim: rgba(0,0,0,0.6)`, `--overlay-scrim: rgba(8,8,12,0.72)`, `--pwa-scrim: rgba(0,0,0,0.55)`, `--topbar-glass: rgba(11,11,15,0.95)`, `--wash: rgba(255,255,255,0.05)`, `--wash-strong: rgba(255,255,255,0.1)`, `--wash-soft: rgba(255,255,255,0.04)`, `--accent-wash: rgba(29,185,84,0.14)`, `--accent-wash-soft: rgba(29,185,84,0.06)`, `--warn-wash: rgba(255,200,60,0.16)`. (Shadows: `.card-play` 440 → `--shadow-2`; detail-art 733, detail-art-icon 750, mobile-sidebar 365, fullscreen-art 1361 → `--shadow-3`; login-form 989 keep recipe or `--shadow-2`.) Falls back to keeping literals ONLY if verifier accepts rgba-as-decorative — but MEMORY deferred them explicitly, so tokenize.

**loja.html unification (web/assets/loja.html, 310 lines):**
- Embedded `<style>` block = lines 8-112 (with its own `:root` override 9-17: `--bg #0b0b0f`, `--surface #1c1c24`, `--surface2 #1a1a1f`, `--grid #2a2a2a`, `--accent #1db954`, `--subtext #a0a0a8`, `--faint #6b6b76`). h1 at line 64 (`font-size: 26px`). Theme-color meta line 6 = `#0B0B0F` (keep in sync).
- Classes used: `.topbar`, `.brand` (svg accent-colored), `.login-box` (input/button), `.user-chip` (+`strong`), `.msg`/`.ok`, `.buy-btn`, `.back-btn`, `.card` (+`.thumb`, h3, `.desc`), `.price`, `.pack-badge`, `.empty`, `.grid`, `.subtitle`, bare `h1/h2/main/footer/body`. JS-coupled IDs (loja.html script): loginBox/loginMsg/phoneInput/loginBtn/catGrid/packsGrid/packsSection — 7 IDs (6 + loginMsg; the allowlist row says 6 IDs but loginMsg IS queried at line 213 `$('#loginMsg')` — count it).
- **Class collisions with style.css (verified by grep): exactly 2 — `.card` (style.css 398: width 144px, flex-shrink 0, cursor pointer — would break loja grid cards) and `.login-box` (style.css 951: width 100%, max-width 384px — would squeeze the topbar login controls).** All other loja classes are unique. Fix: add `class="loja"` to loja.html `<body>` (line 114) and port every loja rule scoped `body.loja .x` (specificity (0,2,1) beats `.card`/`.login-box` (0,1,0) and `.card:hover` (0,2,0)); loja's JS template strings (categoryCard/packCard) and 7 IDs stay untouched.
- **CAN loja.html link style.css? YES — verified in internal/server/static.go this pass: `__ASSET_VERSION__` substitution happens ONLY on the in-memory index (static.go:28 reads index.html + ReplaceAll; lines 40-43 serve it for index/SPA-fallback). loja.html EXISTS in the embed FS → served raw by FileServerFS (static.go:45-51) → **a `?v=__ASSET_VERSION__` link in loja.html would ship the RAW placeholder into served HTML** (audit FAIL). Options: (A) plain `<link rel="stylesheet" href="./style.css">` — RECOMMENDED: no version needed (server sends `Cache-Control: no-cache`; sw.js does NOT precache loja.html — SHELL = `./`, manifest, icons only — so loja navigations are network-first; style.css SW cache-key "style.css?v=1.16.0" vs "style.css" are separate entries, both revalidate fresh); (B) literal `?v=1.16.0` — shares the app-shell cache key but must be hand-synced with internal/version/version.go on every bump (trap). Never (C) raw `__ASSET_VERSION__`.
- Porting spec (targets, loja keeps GREEN identity via --accent-2/--accent-2-fill): body → `var(--bg)`/`var(--text)`/`var(--font-sans)`/`var(--lh-ui)` (style.css body rule covers it once linked); `.topbar` → bg `var(--surface-2, #1b1b20)`, border-bottom `var(--border-soft)`; `.brand svg` → `var(--accent-2, #1db954)`; `.login-box input` → bg `var(--surface-3, #22222a)`, border `var(--outline, #6d6d75)`; `.login-box button` → **bg `var(--accent-2-fill, #15803d)` + `color: var(--on-accent, #fff)` = 5.02 ✓**; `.buy-btn` same flip; `.back-btn` → bg `var(--surface-3)`/`var(--surface2)` keep + `var(--subtext)`; `.card` → bg `var(--surface-2)`, border `0.8px solid var(--outline)` → 1px `var(--outline)`, radius 16 keep (`--radius-lg`-ish), `cursor: default`; `.thumb` gradient keep; `.price` → `var(--accent-2, #1db954)`; `.pack-badge` → bg `var(--accent-wash, rgba(29,185,84,0.15))`, color `var(--accent-2)`; `.empty`/`footer` → `var(--faint)` (auto-fixed: #8a8a93, 3.22 → 5.45 ✓); `.msg` → `var(--danger, #f87171)`; `.ok` → `var(--accent-2)`; `h1` → **`clamp(26px, 2.5vw + 0.9rem, 42px)`** + `--lh-title`; MQ 560px ported as `body.loja`-scoped. Then DELETE the entire `<style>` block (8-112) + `<link>` in head.
- Note the plan says "`web/loja.html`" — actual path is `web/assets/loja.html` (MEMORY T2 correction; only style.css + loja.html may change in T3).

**`.vol-icon` — VERIFIED LIVE this pass (playing track, 1440×900):** present, `display:block`, 16×16 svg, `color: rgb(245,245,247)` (inherits body `--text`), visible (offsetWidth 16), flex-parent `.player-volume` centers it (`align-items:center`). **NOT invisible — no bug.** Zero CSS rules exist for `.vol-icon` (grep: only app.js:1203 `el('span', { class: 'vol-icon' }, ...)` inside bottomBar). Optional polish (additive, safe): `.vol-icon { display:inline-flex; color:var(--subtext); }` to match `.player-btn` treatment. Do NOT treat as a fix item.

### Existing Tools & Resources

- playwright MCP + `getComputedStyle`/`getPropertyValue` recipe (loop TOOLS.md); scrollbar check must use `document.styleSheets` cssRules scan (can't read `::-webkit-scrollbar-thumb` via getComputedStyle). Live session this pass confirmed: overlay dismiss → click "Agora não"; track start → `getByRole('button', { name: 'Tocar' }).first()`; `.vol-icon` check recipe above.
- No new external resources needed (pure CSS patch; no build tooling allowed).
- SW stale-cache gotcha (MEMORY T2-exec): after rebuild+restart, persisted contexts may serve OLD CSS — always unregister SWs + `caches.delete('*')` + reload before live QA; freshness gate = served style.css SHA256 == main-tree SHA256 (byte-identical); fresh contexts do NOT always sidestep it.

### Requirements & Constraints

- CODE CHANGE ONLY in `web/assets/style.css` + `web/assets/loja.html` (NOT `web/loja.html`). Never touch the 4 uncommitted fixes (player.js/helpers.go/common.go/users.go); never `.env`/`auth/`/`payments/`/`secrets/`; no version bump (`__ASSET_VERSION__` stays 1.16.0).
- Hard contract: NO selector renames/deletions on the allowlist (18 IDs + 12 class queries + 2 structural + 14 state + 3 data-attrs); loja 7 IDs (incl. loginMsg) untouched; loja JS behavior untouched. New selectors are ADDITIVE (`body.loja .x`).
- Every new value: `var(--token, fallback)` with fallback = original value at that site. All flips must keep the original hex as fallback (`var(--accent-fill, #3865f8)` — the FALLBACK is the new token value per T2 precedent where tokens are defined in :root; for rules the fallback convention = original hex — for fills use the new fill hex since the old value was the bug).
- Keep `theme-color` meta + manifest `#0B0B0F`; loja theme-color stays (line 6).
- Deliverables: `loop-reports/design-t3.diff` (cmd /c, LF) + `loop-reports/design-t3.md` (what/why, contrast BEFORE→AFTER table). Deploy: worktree `--detach` → copy BOTH style.css + loja.html → `go build -o play-music.exe .` → stop :4533 PID → .env→Process env (never print) → Start-Process → curl 200 → cmd /c diff. Worktree cleanup `git worktree remove --force`.

### Suggested Approach

One pass in the worktree, in this order: (1) contrast flips first (btn-accent/login-submit/card-play/pwa/loja buttons — the 4 FAIL-group fixes + fill consistency), (2) elevation + border/scrollbar remainder (sidebar/bottom-bar/cards → `--surface-2`; modal/pwa-card → `--surface-5`; admin-row/modal/scrollbar borders → `--outline`; upload/search outlines), (3) admin/modal var() fallback polish + chip/badge 12px floor + rgba wash tokens, (4) `body.loja` port block + loja.html link + `<style>` deletion + h1 clamp, (5) `.vol-icon` optional polish. Deploy, then verify per criteria.

### Verification Criteria

**A. Static gates (verifier, on diff + tree):**
1. Hex grep of style.css: remaining raw hexes ONLY inside `:root`/`[data-theme]` token defs, `var(--x, fallback)` fallbacks, or pre-existing kept values (tab-btn.active #fff/#000, player-btn-main/fullscreen-btn-main #fff/#000, genre-card gradient rgba not hex). Specifically ABSENT: `#26262f`, `#2c2c36`, `#1c1c24` outside fallbacks; `.btn-accent`/`.login-submit`/`.card-play`/`.pwa-btn.primary`/`.pwa-icon` background now `var(--accent-fill, #3865f8)`; `.buy-btn`/loja `.login-box button` background `var(--accent-2-fill, #15803d)`; `.admin-row`/`.modal`/`.modal-scroll` borders `var(--outline, ...)`; `.chip`/`.badge` font-size 12px.
2. loja.html: `<link rel="stylesheet" href="./style.css">` present (no `__ASSET_VERSION__` anywhere in loja.html), `<style>` block GONE (lines 8-112 removed), `<body class="loja">`, all 7 IDs (loginBox/loginMsg/phoneInput/loginBtn/catGrid/packsGrid/packsSection) still in markup+JS, `h1` rule exists in style.css as `body.loja h1 { font-size: clamp(26px, 2.5vw + 0.9rem, 42px); ... }`.
3. Allowlist grep: every allowlist selector still resolves in patched style.css; ZERO renamed selector lines in diff (only property-value/added-rule changes; new `body.loja` rules are additive).
4. Contrast recompute (node, WCAG 2.1 exact): #fff/#3865f8 = 4.79 ✓; #fff/#15803d = 5.02 ✓; #8a8a93 on #121218/#1b1b20/#22222a = 5.74/5.01/4.61 ✓; #6d6d75 vs #22222a = 3.08 ✓ (min). All §10.1 pairs re-measured and ≥ threshold.
5. `go build -o play-music.exe .` PASS; diff artifacts LF (cmd /c); diff touches ONLY style.css + loja.html; no version bump.
6. Served-CSS freshness gate: served style.css SHA256 == main-tree SHA256 (byte-identical) after rebuild.

**B. Live gates (:4533, fresh/SW-cleared context, dismiss overlay, play a track):**
7. `getPropertyValue` spot: `--accent-fill` → `#3865f8`, `--accent-2-fill` → `#15803d` (already in :root — sanity).
8. getComputedStyle: `.btn-accent` bg `rgb(56,101,248)` + color `rgb(255,255,255)` (was rgb(97,141,255)); `.sidebar`/`.bottom-bar`/`.card` bg `rgb(27,27,32)` (#1b1b20) if surface-2 applied; `.modal` bg `rgb(18,18,24)` (#121218 surface-5) + border `rgb(109,109,117)` (#6d6d75) on admin view; `.admin-row` border `rgb(109,109,117)`; `.chip` font-size 12px; `.progress-fill`/`.volume-slider` accent-color `rgb(56,101,248)`.
9. loja (`/loja.html`, no token needed for catGrid): `.buy-btn` bg `rgb(21,128,61)` + color #fff (was rgb(29,185,84)); `.price` color `rgb(29,185,84)`; `body.loja .card` width auto (grid-stretched, NOT 144px) + border `rgb(109,109,117)`; `.login-box` not constrained to 384px (max-width none); `h1` computed font-size at 1440px = 42px (clamp max) and ≥26px at 375px; body color `rgb(245,245,247)`; 0 console errors; no horizontal overflow at 375px.
10. Console errors 0 on home + loja + admin; `.pwa-overlay` dismissed before any assertion.
11. FAIL conditions: any §10.1 pair still failing; `.buy-btn` still rgb(29,185,84); loja layout broken by `.card`/`.login-box` collisions; raw `__ASSET_VERSION__` in served loja.html; any allowlist rename; served CSS ≠ tree; console errors.

### Quality Standards

- Done right: every visual property flows from tokens; elevation reads as ramp (surface-2 cards/sidebar/bar, surface-3 inputs, surface-5 modals/pwa); the 4 contrast groups each get a BEFORE→AFTER row in design-t3.md (3.11→4.79, 2.59→5.02, faint 3.22-3.72→4.61-5.74, borders 1.18-1.74→3.08+); loja looks like the app (same surfaces/typography) while keeping its green buy identity via `--accent-2`/`--accent-2-fill`; diff is reviewable (values-first, additive rules, loja port as one contiguous scoped block).
- Anti-patterns: flipping the white player buttons (player-btn-main/fullscreen-btn-main — keep white); renaming anything in the allowlist; `transition: all` (card-play:443 — split into background/transform); raw `__ASSET_VERSION__` in loja.html; keeping loja's divergent `:root` override; new hexes outside tokens; touching the 4 uncommitted fixes; PS `>` for diff files.

### Prior Attempt Analysis

No prior T3 executor attempt (T2 deployed cleanly, audit CLEAN). Carry-forward risks from T2: (1) SW stale-cache after rebuild (unregister + caches.delete + reload; SHA gate); (2) persisted pm_token/overlay in playwright context; (3) PS `>` UTF-16LE — always `cmd /c` diffs; (4) `.detail-title` MQ reconciliation precedent — any dual-size rule must reconcile to one clamp; (5) loja link version-query trap (this pass's finding). Two MEMORY lumping slips corrected here for the executor: `.player-btn-main` in the accent-fill flip list (keep white) and `.chip/.badge` "flip to --accent-2-fill" (they're text tokens + 12px floor, not fills).
## Task-Specific Research — [G4] T4 — Responsive + animations + accessibility polish

Researcher pass 8 (2026-08-07). Goal: map what T4 must change in style.css (2211 lines, post-T3) + app.js (additive aria attrs only) — 320px guard, safe-area, breakpoint review, transitions, prefers-reduced-motion, :focus-visible, aria-pressed, tabular-nums, contrast re-audit. **Six live probes done this pass on :4533 (served HEAD = T3 baseline, 0 console errors): full 320px route sweep, 640/767/768 breakpoint boundary scan, keyboard focus-visible outline check, aria-pressed presence on all play/pause/like/shuffle/repeat buttons, computed tabular-nums + safe-area resolution, and `emulateMedia({reducedMotion:'reduce'})` feasibility.**

### Context & Prior Work

**Proposal requirements (design-proposal.md §7/§8/§9):** 320px no-horizontal-scroll hard gate C5 (`document.documentElement.scrollWidth <= innerWidth`); `env(safe-area-inset-bottom)` on `.bottom-bar` + `#player-full`; keep 640/767/768 MQs (re-verify, adjust only if a gate fails); transitions ≤0.25s transform/opacity/background-color only (never `transition: all`); `@media (prefers-reduced-motion: reduce)` block at END of file (kills spinner/loading/card-play/transitions); global `:focus-visible { outline: 2px solid var(--focus, #8ab0ff); outline-offset: 2px; }` + `@supports not selector(:focus-visible)` fallback, never `outline:none` on interactive; aria-pressed on play/pause/like toggles + aria-label on icon-only controls (static markup index.html/loja.html; app.js el() templates get aria attrs ONLY — no JS logic, player.js untouched); `font-variant-numeric: tabular-nums` on durations; contrast re-audit (all 4 groups fixed in T3 — re-verify, no regressions).

**Motion tokens ALREADY in :root (style.css:51):** `--dur-fast:0.15s; --dur:0.2s; --dur-slow:0.25s; --ease:cubic-bezier(0.4,0,0.2,1);` — executor should use them for new transitions.

**Breakpoint map (current style.css, all VERIFIED live this pass — none broken, keep all):**

| Line | MQ | What it does |
|---|---|---|
| 163 | `max-width: 767px` | `.page` padding-bottom 96→196px (mobile bottom-bar clearance) — VERIFIED 196px @320 |
| 704/861/903 | `min-width: 640px` | track-row 3col→5col + show album/num; detail-header horizontal + art 224px; track-list-header grid |
| 1140 | `min-width: 768px` | `.bottom-bar` 1col→3col + height 96px (padding `0 16px` — safe-area dropped, desktop-correct) |
| 1159/1174/1218/1335 | `hover:hover` / `min-width: 768px` | now-playing hover; art 56px; player-controls order 0; player-volume flex |
| 1533 | `max-width: 480px` | `.upload-grid` 2col→1col |
| 1760 | `max-width: 767px` | sidebar hidden → mobile-topbar flex + sidebar-close shown + fullscreen-buttons gap 10px |
| 1778 | `min-width: 768px` | `.mobile-overlay` display:none !important |
| 1787 | `hover:none + pointer:coarse` | inputs 16px (iOS zoom) |
| 1939 | `hover:none` | touch: play-btn/like/track-add revealed, card-play visible, player-btn padding 8px, main-btn 44px |
| 2208 | `max-width: 560px` | loja topbar column + input 100% |

**Transition inventory (11 decls today):** nav-link color .15s (228), .card bg/border/transform (421), .card-play opacity/transform/bg (460), .genre-card transform .2s (501), .tab-btn color .15s (545), .track-row bg .15s (578), .btn-accent transform .15s (827), .login-submit opacity .15s (1044), .upload-dropzone border/bg .15s (1556), .upload-progress-fill width .25s (1625), body.loja .card none (2167). **Hover/focus-state elements LACKING transitions (add ≤0.25s, transform/opacity/background-color only):** `.icon-btn` (318-330 hover bg+color), `.btn-icon-lg` (843-859), `.playlist-link` (259-278), `.track-like` opacity (681-694), `.player-btn` hover color (1230-1241), `.player-btn-main` hover scale (1243-1256 — pops), `.fullscreen-btn-main` hover scale (1428-1441 — pops), `.login-toggle-btn` (1806-1821), `.modal-item` hover bg (1863-1873), `.modal-close` (1875-1884), `.pwa-btn` (2078-2096), `body.loja .buy-btn` filter hover (2187-2188), `.back-link` (919-931), `.now-playing` hover bg (1149-1163), `.btn-secondary` (1652-1666), `.settings-playlist-item` hover shadow (1489-1505), `.remove-track-btn` (944-956), `.track-add` (1823-1834). Anti-pattern guard: NO `transition: all` ever (T3 already split .card-play's).

**Focus state today:** ONLY `.track-add:focus-visible` (1833, a display rule — not an outline). Inputs use `outline:none` + box-shadow focus (`.form-input` 1018/1022, `.search-type-select` 1077/1089, `.search-input` 1103/1110, `body.loja .login-box input` 2127) — these are fine (visible focus via 2px --accent box-shadow), KEEP them, do NOT let the global rule double-outline them (global `:focus-visible` on inputs + box-shadow is acceptable but redundant — simplest: global rule applies to all; inputs keep their box-shadow too; harmless duplication, or scope global rule to non-input as executor prefers — no gate either way). **Verified live: Tab onto `.icon-btn` → `:focus-visible` matches with DEFAULT outline `0.8px auto rgb(16,16,16)` (near-invisible black on dark) — the exact gap the global rule fixes.** `--focus` token = #8ab0ff = rgb(138,176,255), 7.32-8.66:1 — defined at :root (line 31) + light override #2a5df2 (61).

**Safe-area status:** `.mobile-topbar` ALREADY `calc(10px + env(safe-area-inset-top))` (341, computed 10px 12px live); `.bottom-bar` ALREADY `calc(8px + env(safe-area-inset-bottom))` (1137, computed 8px live; overridden to `0 16px` at ≥768, correct for desktop). **ONLY GAP: `.fullscreen-player` (1355-1363) `padding: 24px` — no env() (computed 24px/24px live).** Fix = `padding: 24px 24px calc(24px + env(safe-area-inset-bottom))` (or env on both via calc). `#player-full` is the transparent host div (app.js:1429) — the target is its child `.fullscreen-player`. Loja topbar is a regular web page (no PWA fullscreen) — no safe-area needed.

**tabular-nums: ALREADY COMPLETE** — `.track-number` (590), `.track-duration` (700), `.player-progress` (1265; covers `.progress-time` in BOTH bottom bar app.js:1248 and fullscreen app.js:1370 wrappers + queue rows via trackRow). Live-computed "tabular-nums" on both. Executor: verify-only, no additions (unless a future duration element appears). `.track-list-header` is a column LABEL, not numeric — no need.

**Contrast re-audit:** T3 flipped everything (audit CLEAN: 4.79 accent-fill / 5.02 green / faint 4.61-5.45 / outline 3.08+). T4 = re-verify live only (computed rgb on .btn-accent/.buy-btn/.chip/.admin-row + no new hexes below threshold). The only T4-introduced color is `--focus` #8ab0ff (7.32+ PASS).

**Allowlist contract (from RESEARCH.md pass 1, hard):** no selector renames; new rules additive. All line refs below are CURRENT (post-T3).

### Existing Tools & Resources

- playwright MCP — all needed tools verified THIS pass: `resize`, `evaluate` (scrollWidth/overflow/getComputedStyle), `run_code_unsafe` with `page.emulateMedia({ reducedMotion: 'reduce' })` **WORKS** (matchMedia flips true — VERIFIED; reset `{ reducedMotion: null }`); keyboard via `page.keyboard.press('Tab')` inside run_code_unsafe (or press_key tool).
- `window.__player.getState()` QA hook (player.js:324) — play state assertions (VERIFIED this pass).
- Served CSS = HEAD = tree (T3): grep served style.css confirms NO prefers-reduced-motion, 11 transitions, 2 safe-area decls, only .track-add:focus-visible.
- No external libs needed (pure CSS + attr-only JS additions; no build tooling).

### Requirements & Constraints

- CODE CHANGE: `web/assets/style.css` (+ `web/assets/app.js` aria attrs ONLY). **index.html and loja.html need NO changes** (verified below). Never touch player.js or the 4 uncommitted fixes. Worktree `../play-music-design-wt --detach` → copy changed files → `go build -o play-music.exe .` → restart :4533 → curl 200.
- **Aria target inventory (app.js line refs, all `el()` template calls — attr additions ONLY, ZERO logic changes):**
  - STATIC markup: **index.html has NO buttons at all** (pure shell, `<div id="app">` only, line 28) → nothing to add. **loja.html: only `<button id="loginBtn">Entrar</button>` (line 19, has text — no aria needed); `.buy-btn`/`.back-btn` are `<a>` links with text** → nothing to add. Proposal's "(index.html/loja.html static markup)" resolves to EMPTY — document this.
  - DYNAMIC gaps (aria-pressed MISSING, verified live): app.js **1241** `.player-btn-main` (play/pause), **1381** `.fullscreen-btn-main`, **1199** bottom-bar like `.icon-btn`, **1364** fullscreen like `.btn-icon-lg`, **1238+1378** shuffle, **1244+1384** repeat, **1008-1009** login-toggle (Cliente/Administrador), admin.js **124-125** login-toggle (login-page variant).
  - ALREADY covered (do NOT touch): `.track-like` rows (app.js:200 has aria-pressed + syncPagePlayerState setAttribute String(isLiked) at 1566-1570 — live-verified `aria-pressed="false"` on rows); aria-labels on track-play-btn (188), card-play (316), track-add (208), remove-track-btn (753), all transport buttons, vol slider (1212), progress-track role=slider+aria-valuemin/max/now (1193/1372).
  - **Sync proof (why no logic change is needed):** `structuralKey()` (app.js:1463-1466) includes playing/shuffle/repeat/liked → `refreshPlayerBar` (1504-1516) rebuilds the bar from templates (50ms debounce) on any toggle → template `aria-pressed` stays current forever. Verified: after clicking bar-like, likedNow flipped true and bar re-rendered.
  - **el() helper gotcha (app.js:15-30, CRITICAL):** line 18 `if (v === undefined || v === null || v === false) continue` → boolean `false` is SKIPPED (attr absent) and line 23 maps `true` → `''` (EMPTY string attr). So `'aria-pressed': playing` is WRONG (''/absent). **MUST pass STRING: `'aria-pressed': playing ? 'true' : 'false'`** (same ternary pattern as the existing aria-labels). Do NOT "fix" line 200's boolean usage (covered by 1566-1570 sync).
- Every new value uses `var(--token, fallback)`; `--focus` #8ab0ff is the ring color. No new breakpoints. Keep touch-target rules (hover:none 1939) untouched.
- Deliverables: `loop-reports/design-t4.diff` (cmd /c, LF) + `loop-reports/design-t4.md` (what/why + final contrast table). No version bump.

### Suggested Approach

One worktree pass in this order: (1) safe-area on `.fullscreen-player` + 320px additive guard tweaks if any (baseline already passes — see below; optional art 48→40 at ≤360, optional ≤360 button-gap shrink — both cosmetic, gate passes without them); (2) transitions on the 19 gap selectors (≤0.25s, tokens --dur*/--ease, transform/opacity/background-color/border-color/color only — never `all`); (3) global `:focus-visible` rule + `@supports not selector(:focus-visible)` fallback placed BEFORE component rules (after :root/light block ~line 62), leaving input box-shadow focus intact; (4) `@media (prefers-reduced-motion: reduce)` block at FILE END (after body.loja 2211): `animation: none !important; transition: none !important;` on spinner/loading/card-play + blanket; (5) aria-pressed string-attrs in the 9 app.js template sites (string ternaries); (6) contrast re-audit = verify-only (no CSS color changes expected). Deploy, then verifier runs the matrix.

### Verification Criteria

**A. Static gates (verifier, diff + tree):**
1. style.css: global `:focus-visible` rule present with `outline: 2px solid var(--focus, #8ab0ff); outline-offset: 2px;` + `@supports not selector(:focus-visible)` fallback; NO new `outline: none` anywhere; the 4 pre-existing `outline:none` sites (1018/1077/1103/2127) untouched.
2. `@media (prefers-reduced-motion: reduce)` block is the LAST rule in the file (after line 2211 body.loja block); inside it: `.spinner` animation killed, `.card-play`/`.card`/all transition decls → none (blanket `* { transition: none !important; animation: none !important; }` or per-selector — either acceptable, blanket preferred per proposal "kills spinner/loading/card-play/transitions").
3. `.fullscreen-player` padding-bottom includes `env(safe-area-inset-bottom)`; `.bottom-bar` (1137) + `.mobile-topbar` (341) byte-unchanged (already safe).
4. Grep `transition:` — no `transition: all`; every new transition ≤0.25s; new decls on ≥10 of the 19 gap selectors (list above) using --dur*/--ease tokens.
5. tabular-nums: unchanged 3 sites (590/700/1265) — no new duration element without it.
6. app.js diff: ONLY added `'aria-pressed': <x> ? 'true' : 'false'` attrs in the 9 el() template calls (1241/1381/1199/1364/1238/1378/1244/1384/1008-1009 + admin.js 124-125); ZERO logic lines changed; player.js NOT in diff; index.html/loja.html NOT in diff (expect no changes — or loja.html only if executor adds aria, which is unnecessary).
7. Braces balanced; `go build -o play-music.exe .` PASS; diff = cmd /c LF; served CSS SHA256 == main-tree SHA256; git status = 4 pre-existing fixes + style.css + app.js only.

**B. Live gates (:4533, SW unregistered + caches cleared + reload, dismiss PWA overlay):**
8. **320px C5 gate** — `document.documentElement.scrollWidth <= window.innerWidth` at 320x568 on: home, library, category detail (145-track view verified), queue, admin + admin modal, fullscreen player, loja.html. (Baseline measured PASSING today at all of them — executor must not regress; scrollWidth == 320 everywhere.)
9. **Resize matrix** 320/375/640/767/768/1024/1440: no horizontal overflow at any width on home+category+admin. **Breakpoint assertions must use `matchMedia` (NOT innerWidth):** this environment has a ~1-3px media-viewport drift at exact boundaries (measured: at innerWidth 767, `matchMedia('(max-width:767px)')` = false, sidebar still desktop; flip happens between innerW 766 and 770) — assert `matchMedia('(max-width: 767px)').matches` for mobile mode and `matchMedia('(min-width: 768px)').matches` for desktop mode, or probe 760/800 instead of the exact boundary. Expected: 640-767 = 5-col track rows + mobile bottom bar; ≥768 = sidebar + 3-col bottom bar + volume visible.
10. **Focus-visible:** `page.keyboard.press('Tab')` repeatedly onto `.btn-accent`, `.icon-btn`, `.player-btn-main`, `.nav-link`, `.back-link`, `.card` → `document.activeElement.matches(':focus-visible')` true AND `getComputedStyle(activeElement).outline` = `2px solid rgb(138, 176, 255)` (or rgb(42,93,242) if light theme). NOT the default `0.8px auto rgb(16,16,16)`.
11. **Reduced-motion** (run_code_unsafe): `page.emulateMedia({ reducedMotion: 'reduce' })` → `matchMedia('(prefers-reduced-motion: reduce)').matches` === true; `.spinner` computed `animationName === 'none'`; `.card-play` + `.card` computed `transitionDuration === '0s'`; `.btn-accent` transitionDuration 0s. Reset `{ reducedMotion: null }`. (emulateMedia VERIFIED working in this MCP.)
12. **aria-pressed:** bottom bar + fullscreen: play button `aria-pressed` = "true" when playing / "false" when paused (click to toggle, assert after re-render); like button toggles "true"/"false"; shuffle/repeat "true" when active (click to toggle); login-toggle buttons on login screen; track-row like still "false"/"true" via existing sync; all values STRING "true"/"false", never "" or absent. Playwright accessibility snapshot shows pressed state on the play button.
13. tabular-nums computed `font-variant-numeric: tabular-nums` on `.track-duration` + `.progress-time` (both bars) — re-assert.
14. Console errors 0 across home/category/admin/fullscreen/loja; FAIL conditions: any 320px overflow; outline still default; reduced-motion block ineffective; aria-pressed empty/missing after toggle; `transition: all`; any app.js logic change; player.js/index.html/loja.html modified; served CSS ≠ tree.

### Quality Standards

- Done right: the file ends with the reduced-motion block (last word); focus ring is the SAME 2px --focus everywhere (visual consistency) and visible on every keyboard-reachable interactive element; durations never jitter (tabular-nums everywhere, unchanged); toggles are truthful to assistive tech (pressed state tracks actual state via the existing structuralKey re-render — no bespoke JS); transitions are subtle (≤0.25s, transform/opacity/background only) and honor reduced-motion; 320px gate evidence recorded per-view; design-t4.md has the final 4-group contrast table (all PASS, values from T3 audit: 4.79/5.02/4.61-5.45/3.08+).
- Anti-patterns: `transition: all`; `outline: none` on any interactive (global or new); adding aria-pressed with BOOLEAN values (el() maps true→'' and skips false); touching player.js or the 4 uncommitted fixes; editing index.html/loja.html for aria (nothing to add there); new breakpoints; JS logic changes (adding the attr to the el() template IS the allowed path); forgetting the `@supports not selector(:focus-visible)` fallback; moving the reduced-motion block anywhere but the file end.

### Prior Attempt Analysis

No prior T4 attempt (T3 deployed cleanly, audit CLEAN). Carry-forward risks: (1) SW stale-cache after rebuild — unregister + caches.delete + reload; SHA256 freshness gate; (2) PS `>` UTF-16LE — `cmd /c` diffs; (3) the ~1-3px media-boundary drift at exactly 767/768 in this env — use matchMedia, not innerWidth, in live assertions (verifier B.9); (4) el() boolean-attr gotcha (A.6/B.12) — string ternaries only; (5) worktree copy: this time ALSO copy app.js (new since T3 — T3 copied only style.css + loja.html); app.js has 4 uncommitted-fix adjacency risk? NO — app.js is CLEAN in git (the 4 fixes are player.js/helpers.go/common.go/users.go) — safe to edit/copy app.js.
