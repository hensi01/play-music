# Design T2 — Visual system implementation (style.css token layer)

Date: 2026-08-07 · Executor deliverable for loop `frontend-design-overhaul` [G2] T2
Scope: `web/assets/style.css` ONLY (single file). No component redesign (T3), no JS-coupled selector touched, loja.html untouched.

## What changed

1. **`:root` rewritten as a two-layer token block** (research "EXACT :root" verbatim):
   - Layer 1 primitives: `--blue-400..800`, `--gray-900..100` (ramp #0b0b0f → #f5f5f7), `--green-500/700`, `--red-400`, `--amber-400`, `--focus-tone`.
   - Layer 2 semantic (dark default): `--surface-1..5` (ramp #121218 → #1b1b20 → #22222a), `--text`, `--on-surface*`, `--accent-fill`, `--on-accent`, `--accent-2*`, `--outline`, `--focus`, `--danger`, `--warn` — plus typography (`--fs-xs..3xl`, `--lh-ui:1.45`, `--lh-title:1.2`), spacing (`--space-1..8`, `--space-page-bottom`), radius (`--radius-sm..round`), shadow (`--shadow-1..3`), motion (`--dur-fast/.dur/.dur-slow`, `--ease`).
   - **All 10 legacy names kept as aliases** (hard contract, every existing `var()` ref keeps resolving): `--bg #0b0b0f`, `--surface`→#121218 (was #121216), `--surface2`→#1b1b20 (was #1a1a1f), `--grid`→#6d6d75 (was #2a2a2a), `--hover #24242b` (kept LITERAL — the `var(--gray-800)` slip would resolve to #1b1b20), `--accent`→#4d7dff (TEXT-only token; was #618dff), `--accent-hover #7ba0ff`, `--subtext`→#b0b0b8, `--faint`→#8a8a93, `--font-sans` verbatim.
   - **`--text` and `--danger` now DEFINED in `:root`** (were undefined; 4 fallback sites in the admin/modal block + 1 in `.upload-fail` now resolve real tokens — sites untouched, fallbacks kept).

2. **`[data-theme="light"]` additive block** appended after `:root` (proposal §2.2 verbatim, semantic-only redefinition, no primitives). Default stays dark (`ND_DEFAULTTHEME=Dark`); no toggle ships in T2.

3. **27 hex → `var(--x, fallback)` conversions** (fallback = original hex at each site):
   - Research 26-rule table: body `#ffffff`→`var(--text, #ffffff)` (L36); scrollbar-thumb:hover `#3a3a42`→`var(--outline, #3a3a42)` (L56); 23 hover/active-text `#fff`→`var(--text, #fff)` (`.sidebar-close:hover`, `.nav-link:hover`, `.nav-link.active`, `.playlist-link:hover`, `.playlist-link.active`, `.icon-btn:hover`, `.tab-btn:hover`, `.detail-meta .link`, `.back-link:hover`, `.form-input`, `.search-type-select`, `.search-type-select option`, `.search-input`, `.now-playing-artist:hover`, `.player-btn:hover`, `.fullscreen-artist:hover`, `.settings-text strong`, `.settings-playlist-item`, `.upload-dropzone-title`, `.upload-fail strong`, `.btn-secondary:hover`, `.pwa-btn:hover`); danger `#f87171`→`var(--danger, #f87171)` (`.remove-track-btn:hover`, `.login-error`).
   - **DEV — L796 `.btn-icon-lg:hover` `#fff`→`var(--text, #fff)`**: same hover-text family, present in the file's hex inventory but MISSING from both the research 26-row table and the deferred list. Left raw it would trip research FAIL-condition A.1 ("raw hex outside allowlist"). Converted with the same pattern; documented so T3 knows it is already tokenized.
   - NOT converted (T3-deferred per research): component fills (`.card-play`, `.tab-btn.active`, `.track-play-btn`, `.btn-accent`, `.login-submit`, `.player-btn-main`, `.progress-fill`×2, `.volume-slider`, `.fullscreen-btn-main`, `.modal` #1c1c24/#2c2c36, `.modal-scroll`, `.admin-row`, `.chip`, `.badge`, `.pwa-icon`, `.pwa-btn.primary`, `.login-toggle-btn.active` #0b0b0f), all `rgba()` washes, and the admin/modal block `var(--x, fallback)` sites (already tokenized, left as-is). loja.html hexes untouched (T3 unify).

4. **Clamp typography (3 of 4 display sizes — style.css only):**
   | Site | Before | After |
   |---|---|---|
   | `.page-title` | 30px | `clamp(30px, 3vw + 1rem, 48px)` + `line-height: var(--lh-title, 1.2)` |
   | `.detail-title` base | 36px / lh 1.1 | `clamp(36px, 3.5vw + 1rem, 48px)` + `line-height: var(--lh-title, 1.2)` |
   | `.detail-title` @media(min-width:640px) | 48px literal | SAME clamp (reconciles the MQ — can no longer exceed max; base and MQ now identical) |
   | `.login-brand h1` | 30px | `clamp(30px, 2.5vw + 1rem, 40px)` + `line-height: var(--lh-title, 1.2)` |
   - **loja h1 clamp DEFERRED to T3**: it lives in `web/assets/loja.html` embedded `<style>` (loja.html:64), which is out of T2's style.css-only scope (loja unify in T3 removes that block entirely). Target clamp for T3: `clamp(26px, 2.5vw + 0.9rem, 42px)`.
   - `--lh-ui: 1.45` applied to `body` (had no line-height).
   - `--fs-*` tokens defined but NOT applied to rules (research: font-size conversions belong to T3).

## Token table (resolved dark values)

| Token | Resolved | Notes |
|---|---|---|
| --bg | #0b0b0f | unchanged (theme-color/meta stay #0B0B0F) |
| --surface / --surface-1 | #121218 | ramp-1 (alias was #121216) |
| --surface2 / --surface-2 | #1b1b20 | ramp-2 (alias was #1a1a1f) |
| --surface-3 / --surface-4 | #22222a | ramp-3 |
| --surface-5 | #121218 | modals/fullscreen, unused T2 |
| --text / --on-surface | #f5f5f7 | NEW (was undefined) |
| --on-surface-sub / --subtext | #b0b0b8 | alias lifted from #a0a0a8 |
| --on-surface-faint / --faint | #8a8a93 | alias lifted from #6b6b73 — FIX lands via alias (4.5:1) |
| --grid | #6d6d75 | alias lifted from #2a2a2a — FIX lands via alias (3:1 UI) |
| --hover | #24242b | LITERAL (1.21:1, WCAG 1.4.11 exempt) |
| --accent | #4d7dff | TEXT-only token (3.69:1 white-text fill FAIL — fills flip to --accent-fill in T3) |
| --accent-fill | #3865f8 | 4.79:1 with white ✓ (unused T2) |
| --accent-hover | #7ba0ff | unused, kept as alias |
| --outline | #6d6d75 | 3.08 min on ramp-3 ✓ |
| --danger | #f87171 | NEW (was undefined) |
| --warn / --accent-2 / --accent-2-fill | #ffcf6b / #1db954 / #15803d | defined, unused T2 |

## Verification results (live, :4533, fresh reload after SW-cache clear)

- Static: braces balanced 327/327; hex grep — every remaining raw hex is inside `:root`/`[data-theme="light"]` token defs, `var(--x, fallback)` fallbacks, or the T3-deferred list; all 10 legacy aliases present; `--text:`/`--danger:` defined; `[data-theme="light"]` present after `:root`; 3 clamp sites + MQ reconciled; `go build -o play-music.exe .` PASS; git diff stat = ONLY `web/assets/style.css` (83+/42-).
- Live getPropertyValue (document.documentElement): --accent `#4d7dff` ✓, --grid `#6d6d75` ✓, --faint `#8a8a93` ✓, --text `#f5f5f7` ✓, --danger `#f87171` ✓, --surface `#121218` ✓, --surface-2 `#1b1b20` ✓, --hover `#24242b` ✓.
- Computed: `body.color` = rgb(245,245,247) ✓ (--text); `body.background` = rgb(11,11,15) ✓ (--bg); `.bottom-bar` bg = rgb(18,18,24) ✓ (#121218); `.page-title` fontSize = 48px (clamp max, range 30–48 ✓); `.detail-title` = 48px (36–48 ✓, lh 57.6px = 1.2× ✓); `.login-brand h1` = 40px (30–40 ✓).
- Scrollbar cssRules scan: `::-webkit-scrollbar-thumb { background: var(--grid) }` ✓, hover `var(--outline, #3a3a42)` ✓.
- Console errors: 0 on all checked views.
- Served CSS SHA256 == main-tree style.css SHA256 (byte-identical) ✓.
- Contrast (computed pairs, fixed in T2 via alias values): faint #8a8a93 on #121218 = 5.45 ≥4.5 ✓; grid/outline #6d6d75 vs #121218 = 3.64 ≥3 ✓ (research §3 quoted 3.83 vs the adjacent-darker bg — audit-flagged pairing slip, verdict unchanged).
- **INTERIM ACCEPTANCE (expected T2 state, NOT a failure):** `.btn-accent`/`.login-submit`/`.card-play` fills resolve to `--accent` #4d7dff → 3.69:1 with white text still FAILS; `.buy-btn` untouched (loja). T3 converts fills to `--accent-fill` #3865f8 (4.79) / `--accent-2-fill` #15803d (5.02). T2's PASS = tokens defined + surfaces/text/borders tokenized.

## Gotcha found during deploy (executor learning)

Playwright context's service worker had cached the OLD style.css; after rebuild+restart the browser still showed pre-T2 values until `navigator.serviceWorker.getRegistrations()` unregister + `caches.keys()` delete + reload. The 401-on-login test artifact was a truncated pasted JWT (PS `Substring` cut the signature), NOT a token-layer issue — verified with a full-length token via in-page fetch.

## Deferred to T3 (explicit list)

- loja h1 clamp (loja.html:64 embedded `<style>`; target `clamp(26px, 2.5vw + 0.9rem, 42px)`) + full loja.html unify (its own `:root` + 6 hex sites).
- Component fills → `--accent-fill`/`--accent-2-fill` (4 contrast fixes), all deferred component hexes, rgba() washes, admin/modal visual polish.
- `--fs-*` application to rule font-sizes; fw tokens optional.
