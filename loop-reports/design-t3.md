# Design T3 — Layouts & components redesign (style.css + loja.html unification)

Task [G3] T3 of the frontend-design-overhaul loop. Executor pass, 2026-08-07.
Scope: `web/assets/style.css` + `web/assets/loja.html` only. No commits.

## What changed

### 1. Contrast flips (FAIL-group fixes + fill consistency)

| Selector | BEFORE | AFTER | Ratio |
|---|---|---|---|
| `.btn-accent` bg | var(--accent) #618dff → #4d7dff | `var(--accent-fill, #3865f8)` | #fff: 3.11 → **4.79** |
| `.login-submit` bg | var(--accent) | `var(--accent-fill, #3865f8)` | #fff: 3.11 → **4.79** |
| `.card-play` bg | var(--accent) | `var(--accent-fill, #3865f8)` | icon 3.69 → 4.79 (consistency) |
| `.pwa-icon` bg / color | var(--accent) / #0b0b0f | accent-fill / `var(--on-accent, #fff)` | 4.79 |
| `.pwa-btn.primary` bg / color | var(--accent) / #0b0b0f | accent-fill / `var(--on-accent, #fff)` | 4.79 |
| `.upload-progress-fill` bg | var(--accent) | `var(--accent-fill, #3865f8)` | 4.79 |
| `.spinner` border-top-color | var(--accent) | `var(--accent-fill, #3865f8)` | 4.79 |
| `.volume-slider` accent-color | #fff | `var(--accent-fill, #3865f8)` | 4.79 |
| `.progress-fill` + `::after` bg | #fff | `var(--accent-fill, #3865f8)` (hover/drag state unified too) | 4.79 |
| loja `.buy-btn` bg | var(--accent) #1db954 | `var(--accent-2-fill, #15803d)` | #fff: 2.59 → **5.02** |
| loja `.login-box button` bg | var(--accent) | `var(--accent-2-fill, #15803d)` | 2.59 → **5.02** |

**KEPT white (21:1 PASS, per research slip correction):** `.player-btn-main` (#fff/#000) and `.fullscreen-btn-main` (#fff/#000) — NOT flipped. Live-verified rgb(255,255,255)/rgb(0,0,0).

### 2. rgba() wash tokenization (11 tokens added to `:root`)

`--border-soft` rgba(42,42,42,.5) · `--scrim` rgba(0,0,0,.6) · `--overlay-scrim` rgba(8,8,12,.72) · `--pwa-scrim` rgba(0,0,0,.55) · `--topbar-glass` rgba(11,11,15,.95) · `--wash` rgba(255,255,255,.05) · `--wash-strong` rgba(255,255,255,.1) · `--wash-soft` rgba(255,255,255,.04) · `--accent-wash` rgba(29,185,84,.14) · `--accent-wash-soft` rgba(29,185,84,.06) · `--warn-wash` rgba(255,200,60,.16).

Converted 16 sites: sidebar border (161), sidebar-footer (272), track-list-header (875), bottom-bar (1116) → `--border-soft`; mobile-topbar (327) → `--topbar-glass`; mobile-overlay-backdrop (357) → `--scrim`; modal-overlay (1812) → `--overlay-scrim`; pwa-overlay (1959) → `--pwa-scrim`; modal-item (1841) → `--wash`; modal-item:hover (1846) → `--wash-strong`; admin-row (1878) → `--wash-soft`; track-add:hover + modal-close:hover (1807/1857, rgba .06) → `--wash-soft`; chip (1893) → `--accent-wash`; upload-dropzone hover (1535) → `--accent-wash-soft`; badge (1904) → `--warn-wash`.

Shadows tokenized: card-play → `--shadow-2`; mobile-sidebar, detail-art, detail-art-icon, fullscreen-art → `--shadow-3` (5 sites).

Light-theme block: added wash-token overrides (light glass/borders) so the additive `[data-theme="light"]` stays coherent. Decorative gradients (`rgba(26,26,31,.6)`, genre-card `rgba(97,141,255,.4)`, detail-header, login-screen, track-list `rgba(18,18,22,.3)`, genre-count `rgba(255,255,255,.7)`) kept as literal rgba — gate A.1 permits non-hex rgba, values are gradients not washes.

### 3. Elevation ramp + border tokens (component polish per proposal §6)

- `.sidebar` → `var(--surface-2, #1b1b20)`; `.mobile-sidebar` → surface-2 + `--shadow-3`
- `.card` → surface-2 bg, `--radius-md 10px`, 1px `var(--outline)` border, hover lift `translateY(-2px)` (0.2s, transform/background/border-color only — `transition: all` REMOVED from `.card-play`)
- `.genre-card` → inset `var(--outline)` ring, hover 0.2s
- `.login-form` → surface-2 + outline border; `.form-input` → surface-3 + outline ring
- `.search-type-select` / `.search-input` / options → surface-3 + outline ring
- `.bottom-bar` → surface-2; `.progress-track` outline ✓ (was grid, same value)
- `.settings-card` → surface-2 + outline border
- `.upload-dropzone` / `.upload-photo-drop` → `--outline` dashed borders; `.error` → `var(--danger, #f87171)`
- `.modal` → `var(--surface-5, #1c1c24)` + `var(--outline, #2c2c36)` border; `.modal-scroll` → outline border; overlay → scrim token
- `.pwa-card` → `var(--surface-5, #121218)`; `.pwa-icon`/`.pwa-btn.primary` → accent-fill + on-accent
- `.admin-row` → `var(--outline, #26262f)` border (1.25 → 3.34:1), wash-soft bg
- `.chip` → **12px floor**, `var(--accent-2, #1db954)` text on accent-wash; `.badge` → **12px floor**, `var(--warn, #ffcf6b)` on warn-wash (NOT fills — text tokens per research slip correction)
- `.login-toggle-btn.active` → `var(--accent-fill, #3865f8)` + on-accent (was legacy green fallback)
- `.track-add:hover` / `.modal-item.new` / `.modal-check input` → repointed legacy green `var(--accent, #1db954)` → `var(--accent-2, #1db954)`
- `.vol-icon` → optional polish: `display:inline-flex`, `color: var(--subtext)` (additive, zero JS impact; verified live flex + #b0b0b8)

### 4. loja.html unification (STANDALONE → tokenized)

- `<link rel="stylesheet" href="./style.css">` (PLAIN — verified static.go: `__ASSET_VERSION__` substitution is index.html-only; raw placeholder would ship otherwise)
- `<body class="loja">` — scoping class resolving the 2 class collisions (`.card` width 144px→auto, `.login-box` 384px→flex) via specificity (0,2,1) vs (0,1,0)
- Embedded `<style>` block (lines 8-112) DELETED; all rules ported as one contiguous `body.loja` scoped block at style.css end
- h1 → `clamp(26px, 2.5vw + 0.9rem, 42px)` + `--lh-title` (was static 26px)
- Green identity kept via `--accent-2` (text/icons #1db954) + `--accent-2-fill` (fills #15803d); `.pack-badge` 11px → 12px floor; thumb gradient hexes #24242e/#101018 tokenized into `var(--surface-4, …)`/`var(--surface-1, …)` fallbacks (gate A.1)
- All 7 JS-coupled IDs untouched (loginBox/loginMsg/phoneInput/loginBtn/catGrid/packsGrid/packsSection), loja JS byte-identical

## Contrast BEFORE → AFTER (WCAG 2.1, exact)

| Pair | BEFORE | AFTER | Verdict |
|---|---|---|---|
| #fff on accent fill | 3.11 (btn-accent) | 4.79 (#3865f8) | FAIL → PASS |
| #fff on loja green | 2.59 (buy-btn) | 5.02 (#15803d) | FAIL → PASS |
| faint #6b6b73 labels | 3.22–3.72 | 4.61–5.45 (#8a8a93 on ramp) | FAIL → PASS |
| borders #26262f/#2c2c36/#2a2a2a | 1.18–1.74 | 3.08–3.64 (#6d6d75 on ramp) | FAIL → PASS |
| white player buttons | 21.0 | 21.0 (kept) | PASS → PASS |

## Verification results

### Static (worktree, gate A.1-A.5)
- Hex grep: every remaining raw hex is inside `:root`/`[data-theme]` token defs, `var(--x, fallback)` fallbacks, or pre-existing kept values (tab-btn.active/player-btn-main/fullscreen-btn-main #fff/#000). `#26262f`/`#2c2c36`/`#1c1c24` only in fallbacks. Braces 361/361. `.chip`/`.badge` 12px.
- loja.html: link present, NO `__ASSET_VERSION__`, `<style>` gone, `<body class="loja">`, all 7 IDs in markup+JS.
- Allowlist: every JS-coupled selector resolves; T3 delta (209+/67-) contains ZERO renamed selector lines (only value changes + additive body.loja/.vol-icon rules).
- Contrast recomputed (node, exact WCAG 2.1): 4.79 / 5.02 / 4.61–5.45 / 3.08–3.64 — all ≥ threshold.
- `go build -o play-music.exe .` PASS (worktree AND main tree).
- Diff artifact: LF via cmd /c (31915 B, 1045 lines).

### Live (:4533, gates B.7-B.11; SW unregistered + caches deleted + reload first)
- Freshness gate: served style.css SHA256 == main-tree SHA256 (byte-identical).
- getPropertyValue: `--accent-fill` #3865f8, `--accent-2-fill` #15803d, `--outline` #6d6d75, `--border-soft` rgba(42,42,42,0.5).
- Home: `.btn-accent` rgb(56,101,248)/#fff; `.sidebar`/`.card`/`.bottom-bar` rgb(27,27,32); `.card` border rgb(109,109,117), radius 10px; `.card-play` rgb(56,101,248); faint labels rgb(138,138,147); player-btn-main rgb(255,255,255)/rgb(0,0,0); `.progress-fill` rgb(56,101,248); `.volume-slider` accent rgb(56,101,248); `.vol-icon` flex #b0b0b8.
- Admin: `.btn-accent` rgb(56,101,248); `.admin-row` border rgb(109,109,117); `.chip` 12px rgb(29,185,84); `.modal` bg rgb(18,18,24) + border rgb(109,109,117), radius 14px; overlay rgba(8,8,12,0.72); `.modal-check input` accent rgb(29,185,84).
- Loja: `.buy-btn` rgb(21,128,61)/#fff; `.price` rgb(29,185,84); `.login-box button` rgb(21,128,61)/#fff; `.card` width 246px (grid-stretched, NOT 144px) + border rgb(109,109,117); `.login-box` max-width none; h1 42px @1440 / 26px @375 (clamp bounds respected); body rgb(245,245,247); topbar stacks column @375; 0 horizontal overflow @375.
- Console: 0 errors on home + loja + admin (2 benign Chrome hints: PWA beforeinstallprompt, password-not-in-form).
- Screenshots (project-root loop-reports/): after-home-1440.png (431 KB), after-loja-1440.png (63 KB), after-admin-1440.png (56 KB), after-player-1440.png (447 KB), after-home-375.png (128 KB), after-loja-375.png (29 KB).

## Notes / gotchas

- SW stale-cache gotcha re-confirmed: persisted playwright context served OLD CSS until SW unregister + caches.delete + reload; first hash comparison (text-based) showed a false mismatch — PS text decoding mangles bytes; use Get-FileHash on curl-saved file for byte-exact checks.
- Admin creds entered via browser form only; never written to files.
- Throwaway client 11999998888 still present (from T2 QA) — cleanup candidate for the next admin session.
