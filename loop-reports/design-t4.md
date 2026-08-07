# T4 — Responsive + animations + accessibility polish — execution report

Task: [G4] T4 — CODE CHANGE in `web/assets/style.css` + `web/assets/app.js` + `web/assets/admin.js` (additive aria only).
Deployed live on http://localhost:4533/ (build + restart done). No commits.

## What changed

### style.css (T4-only delta = 24 additions / 1 modification, cumulative diff artifact vs HEAD includes T2+T3)

1. **Global `:focus-visible` ring** (inserted after `[data-theme="light"]` block, before component rules):
   - `:focus-visible { outline: 2px solid var(--focus, #8ab0ff); outline-offset: 2px; }`
   - `@supports not selector(:focus-visible) { :focus { ... } }` fallback for old engines
   - The 4 pre-existing input `outline:none` + box-shadow focus rules (`.form-input` 1039, `.search-type-select` 1098, `.search-input` 1124, `body.loja .login-box input` 2159) left byte-untouched — inputs keep their 2px `--accent` box-shadow focus (box-shadow wins because input rules come later in the cascade; verified live).
2. **Transitions on all 19 gap selectors** (tokens `var(--dur-fast, 0.15s)` / `var(--dur, 0.2s)` + `var(--ease, cubic-bezier(0.4,0,0.2,1))`, all ≤0.25s, property-specific — NEVER `transition: all`):
   - color: `.playlist-link`, `.player-btn`, `.back-link`, `.btn-secondary`, `.pwa-btn`
   - background+color: `.icon-btn`, `.btn-icon-lg`, `.login-toggle-btn`, `.modal-close`, `.track-add`
   - background: `.now-playing`, `.modal-item`
   - opacity+color: `.track-like`, `.remove-track-btn`
   - transform: `.player-btn-main`, `.fullscreen-btn-main` (hover scale 1.05 — no more pop)
   - box-shadow: `.settings-playlist-item` (hover ring)
   - filter: `body.loja .buy-btn` (hover brightness — filter is the ONLY changing property; property-specific, allowed)
   - Post-T4 transition inventory: 30 decls (11 pre-existing + 19 new), 0 `transition: all`.
3. **`@media (prefers-reduced-motion: reduce)`** block as the LAST rule in the file (after the final `body.loja` 560px MQ): blanket `* , *::before, *::after { animation: none !important; transition: none !important; scroll-behavior: auto !important; }` — kills spinner spin, card-play reveal, all transitions. Verified as the final cssRule in the served stylesheet (cssRules scan).
4. **Safe-area**: `.fullscreen-player` `padding: 24px` → `padding: 24px 24px calc(24px + env(safe-area-inset-bottom))` (verified in served cssRule: `"24px 24px calc(24px + env(safe-area-inset-bottom))"`). `.bottom-bar` + `.mobile-topbar` already had env() — byte-unchanged.
5. **320px cosmetics: SKIPPED by design** — the C5 gate already passes on all 7 views at baseline; the only suggested tweaks (now-playing-art 48→40 @<360, fullscreen button-gap shrink) would require NEW breakpoints, which the research explicitly forbids ("No new breakpoints"). Gate passes without them.
6. No new breakpoints, no selector renames, no color changes (hex grep: 151 matches = T3's 149 + 2 `#8ab0ff` fallbacks in the new focus rules; all in token defs/fallbacks/kept values).

### app.js (10 el() template lines — aria attrs ADDED ONLY, zero logic changes)

`'aria-pressed': <cond> ? 'true' : 'false'` — STRING values (el() gotcha: boolean `false` is skipped, `true` renders `''`). Sync is free via the existing `structuralKey()` re-render (no JS changes needed):

| Site | Selector | Expression |
|---|---|---|
| 1008 | login-toggle Cliente | `loginMode === 'client'` |
| 1009 | login-toggle Administrador | `loginMode === 'admin'` |
| 1199 | bottom-bar like `.icon-btn` | `current.liked` |
| 1238 | bottom-bar shuffle | `shuffle` |
| 1241 | bottom-bar play/pause `.player-btn-main` | `playing` |
| 1244 | bottom-bar repeat | `repeat` |
| 1364 | fullscreen like `.btn-icon-lg` | `current.liked` |
| 1378 | fullscreen shuffle | `shuffle` |
| 1381 | fullscreen play `.fullscreen-btn-main` | `playing` |
| 1384 | fullscreen repeat | `repeat` |

`index.html`/`loja.html`: verified ZERO buttons to instrument (index.html is an empty shell; loja has 1 text button + `<a>` links) — nothing added, files untouched.

### admin.js (2 lines, additive only)

- 124: `adminBtn` → `'aria-pressed': isAdmin ? 'true' : 'false'`
- 125: `clientBtn` → `'aria-pressed': !isAdmin ? 'true' : 'false'`

NOTE (honest limitation): the admin user-form toggle's `sync()` only flips `.active` classes — aria-pressed reflects the INITIAL state (verified: Cliente `"true"`/Administrador `"false"` on open, correct for the client default). Live-flipping it would require a logic change in `sync()`, which the task forbids ("NO JS logic changes"). The login-screen toggle (app.js) re-renders fully on click, so its aria-pressed stays truthful (verified flips). The admin-modal toggle keeps truthful visual state; aria stays initial — documented for the verifier.

### tabular-nums — verify-only (already complete at T3)

3 sites unchanged: `.track-number` (590), `.track-duration` (700), `.player-progress` (1265). Verified computed `tabular-nums` on all 145 rows of the Cristão category + both player bars.

### Contrast re-audit — verify-only (no color changes this task)

| Pair | Ratio | Status |
|---|---|---|
| #fff on --accent-fill #3865f8 (.btn-accent/.login-submit/.card-play/.player-btn-main…) | 4.79:1 | PASS |
| #fff on --accent-2-fill #15803d (.buy-btn/.login-box button) | 5.02:1 | PASS |
| --faint #8a8a93 on surface ramp | 4.61–5.45:1 | PASS |
| --outline #6d6d75 vs ramp | 3.08–3.64:1 | PASS |
| NEW --focus #8ab0ff ring (only new color) | 7.32–8.66:1 vs bg | PASS |

## Verification matrix results (live on :4533)

| Gate | Result |
|---|---|
| **A.1** focus-visible global rule + @supports fallback in served CSS; no new `outline:none`; 4 pre-existing sites untouched | PASS |
| **A.2** reduced-motion block is the LAST cssRule in served stylesheet | PASS |
| **A.3** `.fullscreen-player` padding has env(safe-area-inset-bottom); bottom-bar/mobile-topbar byte-unchanged | PASS |
| **A.4** 30 transition decls, 0 `transition:all`, 19 new on gap selectors w/ tokens, all ≤0.25s | PASS |
| **A.5** tabular-nums 3 sites unchanged | PASS |
| **A.6** app.js diff = 10 aria-pressed-only lines; admin.js = 2; zero logic lines; player.js/index.html/loja.html NOT in diff | PASS |
| **A.7** braces 366/366; node --check app.js+admin.js PASS; go build PASS (worktree + main); diff LF (0 CRLF); served SHA == tree SHA (B5516DF3…); git status = 4 pre-existing fixes + loja.html(T3) + style.css + app.js + admin.js | PASS |
| **B.8** 320px C5 gate (`scrollWidth <= innerWidth`) on home (320/320), category 145-track (320/320), admin (320/320), admin modal (320/320), queue (320/320), fullscreen (320/320), loja (310/320) | PASS |
| **B.9** resize sweep 320/375/640/767/768/1024/1440 — zero horizontal overflow everywhere; matchMedia drift quirk reproduced (at innerWidth 767 `max-width:767px`=false, at 768 `min-width:768px`=true) — asserted via matchMedia, not innerWidth | PASS |
| **B.10** Tab → `:focus-visible` matches, outline COLOR exactly `rgb(138,176,255)` (#8ab0ff) — NOT the old default `0.8px auto rgb(16,16,16)`. NOTE: computed width reads `1.6px` because this playwright env applies ~0.8× browser page-zoom (proven: inline `outline:2px` probe also computes 1.6px); the CSS rule is 2px | PASS |
| **B.11** emulateMedia reduce → matchMedia true; `.card-play`/`.card`/`.icon-btn` transitionDuration `0s` (before: 0.15s/0.2s); restored after reset; `.spinner` animation killed by blanket rule (rule verified in cssText) | PASS |
| **B.12** aria-pressed STRING toggles live: bottom-bar play `"true"`→pause `"false"`→play `"true"`; shuffle `"false"`→`"true"`→`"false"`; repeat `"false"`→`"true"`; like `"true"`→`"false"`→`"true"`; fullscreen same; login-screen toggle client `"true"`/admin `"false"` → click admin → `"false"`/`"true"` → back; admin-modal initial `"true"`/`"false"` (static, documented above); a11y snapshot shows `[pressed]` on Pausar + Descurtir | PASS |
| **B.13** tabular-nums computed on .track-duration (all 145), .track-number, .progress-time (both bars) | PASS |
| **B.14** console errors 0 on clean loads (home/category/queue/loja/admin); only self-inflicted 401/404s from token-removal testing + invalid #/category/1 probe | PASS |

## Files

- `loop-reports/design-t4.diff` — LF, 38,016 B (cumulative vs HEAD, includes T2/T3 delta — consistent with T2/T3 convention; T4-only delta listed above)
- `loop-reports/after-final-1440.png` (473 KB), `loop-reports/after-final-375.png` (80 KB)
- `web/assets/style.css`, `web/assets/app.js`, `web/assets/admin.js` — modified in main tree (uncommitted)
