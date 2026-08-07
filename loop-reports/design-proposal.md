# Design Proposal — Play Music Frontend Overhaul

Loop: `frontend-design-overhaul` · Task: [G1] T1 — Design audit + proposal (report-only, no code changes)
Date: 2026-08-07 · Evidence: live playwright against http://localhost:4533/ (served HEAD, 0 console errors in session)
Basis: `loop-stack/frontend-design-overhaul/RESEARCH.md` pass 5 (contrast values re-computed exactly this task, see §10)

---

## 1. Current-State Findings + Audit Summary

### 1.1 Screenshots captured (8, all under `loop-reports/before-*.png`)

| # | File | View | Viewport |
|---|---|---|---|
| 1 | `before-home-1440.png` | Home logged-OUT (pm_token cleared, login screen) | 1440×900 |
| 2 | `before-home-375.png` | Home logged-OUT | 375×667 |
| 3 | `before-loja-1440.png` | loja.html (catGrid rendered, .buy-btn visible) | 1440×900 |
| 4 | `before-loja-375.png` | loja.html | 375×667 |
| 5 | `before-admin-1440.png` | #/admin (logged in via form; .btn-accent "Novo usuário", .admin-row) | 1440×900 |
| 6 | `before-admin-375.png` | #/admin | 375×667 |
| 7 | `before-player-1440.png` | Player bar, track playing (`progress: 16.6s` of 275.3s at shot) | 1440×900 |
| 8 | `before-player-375.png` | Player bar, track playing (`progress: 30s`) | 375×667 |

All shots taken AFTER dismissing the PWA overlay ("Agora não", 1.5s+ after load; overlay presence verified false via evaluate before every shot). Player state asserted programmatically via `window.__player.getState()` (`playing: true`, progress ticking). Session console errors: 0.

### 1.2 Computed-style evidence (playwright evaluate + getComputedStyle, exact values)

| Element | Computed | Meaning |
|---|---|---|
| `.btn-accent` (admin "Novo usuário") | `color: rgb(255,255,255)` · `background: rgb(97,141,255)` · 14px/700 | #fff on #618dff → 3.11:1 FAIL |
| `.buy-btn` (loja "Comprar") | `color: rgb(255,255,255)` · `background: rgb(29,185,84)` · 14px/700 | #fff on #1db954 → 2.59:1 FAIL |
| `.sidebar-user-handle` / `.playlists-empty` | `color: rgb(107,107,115)` · 12px/400 | --faint #6b6b73 → 3.22–3.72:1 FAIL |
| `.bottom-bar` | `background: rgb(18,18,22)` = #121216 (wrapper `#player-bar` is transparent) | var(--surface) resolved; proof target is `.bottom-bar` |
| `.admin-row` | `border: 0.8px solid rgb(38,38,47)` = #26262f; `.admin-row-sub` color rgb(160,160,168) | 1.25:1 FAIL (border) |
| `::-webkit-scrollbar-thumb` (cssRules scan) | `background: var(--grid)` → #2a2a2a | 1.30:1 FAIL |
| `:root` tokens (resolved) | `--accent: #618dff` · `--grid: #2a2a2a` · `--faint: #6b6b73` · `--surface: #121216` · `--subtext: #a0a0a8` | all resolve; loja overrides `--accent:#1db954 --surface:#1c1c24 --faint:#6b6b76` |
| loja `.price` | `color: rgb(29,185,84)` · 22px/800 | green as large text: PASS (7.60:1) |
| loja `.badge` (admin) | `color: rgb(255,207,107)` on rgba(255,200,60,.16) · 10px | 12.80:1 PASS |

### 1.3 Contrast audit (exact WCAG 2.1, computed this task — full table in §10)

4 failing groups, all confirmed live:

1. **#fff on --accent #618dff** (`.btn-accent` fill) — **3.11:1**, FAIL AA normal (large/UI 3:1 only).
2. **#fff on loja green #1db954** (`.buy-btn`, `.login-box button` fills) — **2.59:1**, FAIL even large-text 3:1.
3. **--faint #6b6b73 labels** (12px: sidebar-nav-label, playlists-empty, sidebar-user-handle, track-list-header, placeholders) — **3.28–3.72:1**; loja --faint #6b6b76 — **3.22:1**. FAIL AA normal.
4. **Borders/scrollbar/progress < 3:1** (--grid #2a2a2a 1.18–1.37, .admin-row 1.25, .modal 1.23, scrollbar hover 1.74). FAIL 1.4.11 UI 3:1. (`--hover #24242b` 1.21:1 is EXEMPT — pointer indicator, 1.4.11.)

Passing (no action): #fff/#0b0b0f 19.64 · #fff/#121216 18.69 · --subtext #a0a0a8 on all surfaces 6.52–7.57 · #618dff as TEXT 6.02–6.33 · #1db954 as TEXT 6.54–7.60 · danger #f87171 6.76 · warn #ffcf6b 12.80 · chip text #3ddc84 on composite #14291f 8.61.

### 1.4 Other current-state observations (evidence-based)

- **Typographic scale is flat**: headings/buttons/rows mostly 14–16px; display sizes (`.page-title` ~30px, detail title ~36px, loja h1 ~26px, login brand ~30px) are static px — no fluid clamp.
- **No spacing/radius/shadow tokens**: literal px (2–40px; page-bottom padding 96/196px); 5 distinct shadow recipes; radii inline.
- **Loja diverges from the app**: its own `:root` (`--surface #1c1c24` ≠ app `#121216`, `--accent #1db954` green vs blue, `--faint #6b6b76`) + an embedded `<style>` block (T3 unifies).
- **Undefined tokens**: `--text` and `--danger` are only used as `var(--x, fallback)` (never defined in :root). `--accent-hover #7ba0ff` is defined but never used.
- **Focus states exist only on form inputs/search/select**; buttons/cards/nav rows have no focus-visible affordance.
- **No `:focus-visible` global rule, no `prefers-reduced-motion`, no `outline` management** on interactive elements.
- **No transitions on most interactive elements**; player knob/volume slider rely on native input styling (native `<input type="range">` inside bar — good a11y baseline).
- **No light theme** (ND_DEFAULTTHEME=Dark; `theme_color #0B0B0F` in manifest + index + loja).

---

## 2. Token Architecture

### 2.1 Layers in `:root` (two-layer: primitives + semantic), additive only

```css
:root {
  /* ── LAYER 1 · PRIMITIVES ─────────────────────────────── */
  --blue-400:#7ba0ff;  --blue-500:#618dff;  --blue-600:#4d7dff;  --blue-700:#3865f8;  --blue-800:#2f5fff;
  --gray-900:#0b0b0f;  --gray-850:#121218;  --gray-800:#1b1b20;  --gray-750:#22222a;
  --gray-700:#6d6d75;  --gray-600:#8a8a93;  --gray-500:#b0b0b8;  --gray-100:#f5f5f7;
  --green-500:#1db954; --green-700:#15803d;
  --red-400:#f87171;   --amber-400:#ffcf6b; --focus-tone:#8ab0ff;
  /* ── LAYER 2 · SEMANTIC (dark default) ─────────────────── */
  --bg:            var(--gray-900, #0b0b0f);     /* legacy alias: body/page bg            */
  --surface:       var(--gray-850, #121216);     /* legacy alias: sidebar/bottom-bar/cards */
  --surface2:      var(--gray-800, #1a1a1f);     /* legacy alias: inputs/secondary        */
  --surface-1:     var(--gray-900, #0b0b0f);     /* NEW: page bg                          */
  --surface-2:     var(--gray-850, #121216);     /* NEW: cards/sidebar/bottom bar         */
  --surface-3:     var(--gray-800, #1b1b20);     /* NEW: inputs/buttons/track rows        */
  --surface-4:     var(--gray-750, #22222a);     /* NEW: hovered rows/player-knob host    */
  --surface-5:     var(--gray-850, #121216);     /* NEW: modals/player bar/fullscreen     */
  --text:          var(--gray-100, #f5f5f7);     /* NEW (previously undefined!)           */
  --on-surface:    var(--gray-100, #f5f5f7);     /* NEW: primary text                     */
  --on-surface-sub: var(--gray-500, #b0b0b8);    /* NEW: secondary text (was --subtext)   */
  --on-surface-faint: var(--gray-600, #8a8a93);  /* NEW: 12px labels (was --faint)        */
  --subtext:       var(--on-surface-sub);        /* legacy alias                          */
  --faint:         var(--on-surface-faint);      /* legacy alias                           */
  --grid:          var(--gray-700, #6d6d75);     /* legacy alias: borders/scrollbar       */
  --hover:         var(--gray-800, #24242b);     /* legacy alias: hover washes            */
  --accent:        var(--blue-600, #4d7dff);     /* legacy alias: TEXT/links/icons only   */
  --accent-hover:  var(--blue-400, #7ba0ff);     /* legacy alias (currently unused)       */
  --accent-fill:   var(--blue-700, #3865f8);     /* NEW: filled-button background         */
  --on-accent:     #fff;                          /* NEW: text on accent fills (4.79:1)    */
  --accent-2:      var(--green-500, #1db954);    /* NEW: green TEXT/icons (loja unified)  */
  --accent-2-fill: var(--green-700, #15803d);    /* NEW: green button fill (5.02:1)       */
  --outline:       var(--gray-700, #6d6d75);     /* NEW: control borders ≥3:1 (also --grid) */
  --focus:         var(--focus-tone, #8ab0ff);   /* NEW: focus ring (7.32–8.66:1)         */
  --danger:        var(--red-400, #f87171);      /* NEW (previously undefined!)           */
  --warn:          var(--amber-400, #ffcf6b);    /* NEW                                  */
  /* typography / space / radius / shadow / motion tokens — §4 §5 §8 */
}
```

### 2.2 Rules that keep the architecture safe

- **Every legacy token name is kept as an alias** (`--bg --surface --surface2 --grid --hover --accent --accent-hover --subtext --faint --font-sans`) → all existing `var()` refs and any JS-set inline styles keep resolving. No selector, no `var()` usage changes in T2/T3.
- **`var(--x, fallback)` everywhere**: every new rule uses the existing fallback hex so a missing token can never produce an invalid value (browsers drop the whole declaration otherwise).
- **`--text` and `--danger` become DEFINED in `:root`** (currently fallback-only, e.g. `var(--danger, #f87171)` sites).
- **Light theme = additive `[data-theme="light"] { … }` block** (default stays dark — ND_DEFAULTTHEME=Dark). The block re-defines ONLY semantic tokens (primitives unchanged): `--bg:#f5f5f7 --surface:#ffffff --surface2:#e9e9ee --surface-1:#f5f5f7 --surface-2:#ffffff --surface-3:#e9e9ee --surface-4:#e0e0e6 --surface-5:#ffffff --on-surface:#1a1a1f --on-surface-sub:#4a4a55 --on-surface-faint:#666670 --text:#1a1a1f --subtext:#4a4a55 --faint:#666670 --grid:#6d6d75 --hover:rgba(0,0,0,0.05) --accent:#2f5fff --accent-fill:#2f5fff --on-accent:#fff --outline:#6d6d75 --focus:#2a5df2`. No dark rule is overridden outside the block; light text/border contrast verified in §10.
- **`theme-color` meta + manifest stay `#0B0B0F`** (dark default; only revisit if a theme toggle ships).

---

## 3. Corrected Target Palette (dark default)

> **CORRECTION (research pass 5, re-verified this task):** `#4d7dff` as a white-text fill = **3.69:1 — FAILS 4.5:1**. The plan's original target is only valid as accent TEXT. Fills must use `#3865f8` (4.79:1 ✓).

| Token | Value | Used for | Contrast proof |
|---|---|---|---|
| `--surface-1` | `#121218` | page bg (was #0b0b0f; elevation ramp start) | text stack below |
| `--surface-2` | `#1b1b20` | cards, sidebar, bottom bar (was #121216) | text stack below |
| `--surface-3` | `#22222a` | inputs, buttons, track-row hover host (was #1a1a1f) | text stack below |
| `--on-surface` | `#f5f5f7` | primary text | 18.04 / 17.16 / 14.50 ✓ |
| `--on-surface-sub` | `#b0b0b8` | secondary text (was #a0a0a8) | 9.12 / 8.67 / 7.33 ✓ |
| `--on-surface-faint` | `#8a8a93` | 12px labels (was #6b6b73) | 5.74 / 5.46 / 5.01 / 4.61 ✓ |
| `--accent` (text) | `#4d7dff` | links, icons, active text | 5.33 / 5.07 / 4.65 ✓ (⚠ 4.28 on surface-3 — accent-as-text only on surfaces 1–2) |
| `--accent-fill` | `#3865f8` | `.btn-accent` fill | #fff on it = **4.79 ✓** (alt `#2f5fff` = 5.00) |
| `--accent-2` (green text) | `#1db954` | `.price`, `.ok`, pack badge, `.chip` text | 7.60 / 6.54 / 6.10 ✓ |
| `--accent-2-fill` (green fill) | `#15803d` | `.buy-btn`, `.login-box button` | #fff on it = **5.02 ✓** (⚠ as TEXT 3.37–3.92 — never use the fill tone for text) |
| `--outline` / `--grid` | `#6d6d75` | borders, scrollbar, control outlines | 3.83 / 3.64 / **3.08** ✓ (all ramps; #6a6a70 fails ramp-3 at 2.94 — do NOT use) |
| `--focus` | `#8ab0ff` | focus rings | 8.66 / 7.32 ✓ |
| `--danger` | `#f87171` | error text (unchanged) | 6.76 ✓ |
| `--warn` | `#ffcf6b` | badge text (unchanged) | 12.80 ✓ |
| `--hover` | `#24242b` (mapped to `--surface-3` ramp tone) | hover washes | 1.21 — EXEMPT (1.4.11 pointer indicator) |
| `theme-color` | `#0B0B0F` | meta + manifest | unchanged |

Light theme (§2.2) verified: on-surface #1a1a1f/#f5f5f7 15.92, sub #4a4a55 8.03, faint #666670 5.21, accent text #2f5fff 4.59, accent fill #2f5fff w/ #fff 5.00, outline #6d6d75 4.71 — all ≥4.5/≥3 ✓.

---

## 4. Typography Plan

New tokens: `--fs-xs 12px · --fs-sm 13px · --fs-md 14px · --fs-base 16px · --fs-lg 18px · --fs-xl 20px · --fs-2xl 24px · --fs-3xl 30px`; `--lh-ui 1.45` (default body/UI 1.35–1.5) · `--lh-title 1.2`; `--font-sans` stays.

**Fluid display scale — `clamp()` on the 4 display sizes** (min ≥ 2× below, per MDN 200%-zoom guideline — all have max ≥ 2× min in px terms at 320px→1920px):

| Site | Current | Target |
|---|---|---|
| `.page-title` (page header) | ~30px static | `clamp(30px, 3vw + 1rem, 48px)` |
| `.detail-title` (detail header) | ~36px static | `clamp(36px, 3.5vw + 1rem, 48px)` |
| loja `h1` (loja header) | ~26px static | `clamp(26px, 2.5vw + 0.9rem, 42px)` |
| `.login-brand h1` (login brand) | ~30px static | `clamp(30px, 2.5vw + 1rem, 40px)` |

- Line-height: UI text 1.35–1.5 (`--lh-ui`), titles 1.2 (`--lh-title`).
- **12px label floor**: no text below 12px anywhere (sidebar-nav-label, playlists-empty, sidebar-user-handle, track-list-header, placeholders, admin badge 10px→12px).
- **`font-variant-numeric: tabular-nums`** on all duration/seek/time text (track-row duration, player-bar time, fullscreen time).
- Font stacks unchanged (`--font-sans` preserved — no dependency changes).

---

## 5. Spacing / Radius / Shadow Scales

### Spacing
`--space-1 4px · --space-2 8px · --space-3 12px · --space-4 16px · --space-5 20px · --space-6 24px · --space-7 32px · --space-8 40px`; `--space-page-bottom 96px` (desktop) / `196px` (<768px, mobile topbar overlap). Existing literal px (2–40) map onto these; no layout value changes in T2 (pure token substitution), redesign in T3.

### Radius
`--radius-sm 6px` (inputs, chips) · `--radius-md 10px` (cards, buttons) · `--radius-lg 14px` (modals, upload, pwa-card) · `--radius-pill 999px` (tabs, search, tags) · `--radius-round 50%` (artwork, icon buttons). Current values (e.g. cards 10px, pills) preserved so this is a unification, not a visual jump.

### Shadows
`--shadow-1: 0 1px 3px rgba(0,0,0,0.4)` (resting cards) · `--shadow-2: 0 10px 15px -3px rgba(0,0,0,0.4)` (hover cards, current card-play) · `--shadow-3: 0 25px 50px -12px rgba(0,0,0,0.8)` (detail art, fullscreen art, mobile sidebar, login form, pwa-card — all existing recipes collapse into 3).

---

## 6. Component-by-Component Redesign List (~29)

Each row: current issue → proposed change. ALL class/id names are allowlist-safe (no renames).

| # | Component | Current issue → Proposed change |
|---|---|---|
| 1 | App shell | flat #0b0b0f bg → gradient header accents using `--surface-1`→`--surface-2` ramp + existing radial gradients kept, tokenized |
| 2 | Sidebar | `--surface` bg, faint 12px labels 3.5:1 → `--surface-2`, labels → `--on-surface-faint` (#8a8a93), nav hover → `--hover` + ≤0.2s transition |
| 3 | Mobile topbar + overlay/backdrop | 288px slide-in, no transition → slide+fade ≤0.25s transform/opacity, backdrop blur kept, `--surface-3` with `--outline` edge |
| 4 | Cards (`.card`) | 10px radius, hover play btn `--shadow-2` → `--surface-2` bg, `--outline` 1px border, hover lift via transform translateY(-2px) ≤0.2s |
| 5 | Genre cards | 135deg gradient text contrast OK → keep gradients, add `--outline` inset ring for definition, hover scale 1.03 ≤0.2s |
| 6 | Empty state | `.playlists-empty` faint 3.5:1 → `--on-surface-faint` (4.5:1+), icon + `--on-surface-sub` copy |
| 7 | Tabs (`.tab-btn`) | white-bg active w/ black text (21:1, fine) → keep, add `:focus-visible` ring, pill radius `--radius-pill` |
| 8 | Track rows (`.track-row`) | no focus affordance, row separators `--grid` 1.3:1 → `--outline`, hover `--hover`, `:focus-visible` ring, `--fs-sm` metadata |
| 9 | Detail header | static 36px title → clamp 36→48, art shadow `--shadow-3`, back-link focus ring |
| 10 | Track-list-header | 12px faint 3.5:1 → `--on-surface-faint`, tabular-nums duration column |
| 11 | Section-block | flat headings → `--fs-lg`/`--lh-title`, `--space-6` gaps tokenized |
| 12 | Back-link | icon-only, no label → keep icon, add `aria-label`, `:focus-visible` |
| 13 | Playlist rows + remove-track-btn | `--grid` separators 1.3:1, danger 6.76:1 OK → `--outline` separators, danger as `--danger` token, hover state |
| 14 | Login screen | brand 30px static → clamp 30→40, form card `--surface-2`/`--shadow-3`, `.login-box button` fill → `--accent-2-fill` #15803d (5.02:1, fixes FAIL group 2), error → `--danger` |
| 15 | Search bar | pill `--radius-pill`, custom chevron kept → `--surface-3` bg, `--outline` border (3:1+), focus ring `--focus`, placeholder → `--on-surface-faint` |
| 16 | Bottom player bar (`.bottom-bar`) | `--surface` 18,18,22 → `--surface-2`, controls `:focus-visible`, progress track `--outline`, knob `--accent-fill`, duration tabular-nums, `env(safe-area-inset-bottom)` (T4) |
| 17 | Fullscreen player | art `min(72vw,384px,40vh)` kept → title clamp 24→32, 64px controls with `:focus-visible`, time tabular-nums, `--shadow-3` |
| 18 | Settings | card + actions → `--surface-2` cards, `--outline` separators, focus rings |
| 19 | Upload form | inputs `--surface-3`/`--outline`, dropzone dashed border ≥3:1 (`--outline`), fail msg `--danger`, progress fill `--accent-fill` |
| 20 | Modals (overlay z100/1000) | 420/620px, `--surface` → `--surface-5` + `--outline` border (1.23→3:1+), header h3 `--fs-xl`, scrollbar `--outline` |
| 21 | Admin toolbar + table | toolbar `--surface-2` → `--surface-3`, `.btn-accent` fill → `--accent-fill` #3865f8 (4.79:1, fixes FAIL group 1) |
| 22 | Admin rows | 0.8px `#26262f` border 1.25:1 → 1px `--outline` #6d6d75 (3:1+, fixes FAIL group 4), row hover `--hover`, sub text `--on-surface-sub` |
| 23 | Admin chips (`.chip`) | green-tinted bg + `#3ddc84` text (8.61:1 composite, PASS) → tokenized `--accent-2` text on rgba(29,185,84,.14) |
| 24 | Badge (`.badge`) | 10px `#ffcf6b` on rgba(255,200,60,.16), 12.80:1 PASS → 12px floor, `--warn` text |
| 25 | PWA popup | z1000 blur card → `--surface-5`, `--shadow-3`, `--outline`, `.pwa-btn` primary = `.btn-accent` treatment |
| 26 | Queue (`.queue-track-wrap`) | separators `--grid` → `--outline`, current marker `--accent` text, row hover `--hover` |
| 27 | Spinner + loading-screen | keep geometry → `--accent-fill` color, add `prefers-reduced-motion` kill (T4) |
| 28 | Login toggle (`.login-toggle-btn`) | two-tab switcher, active = `.tab-btn` white/black → focus rings, `aria-pressed` (T4) |
| 29 | Track-add button (`.track-add`) | icon-only → `aria-label`, `:focus-visible`, hover `--accent-2-fill` bg |
| 30 | (bonus) loja.html unify | embedded `<style>` block removed (T3); tokens from style.css via existing classes; `--surface #1c1c24`→`--surface-2`, `--accent` → `--accent-2` (green) with `--accent-2-fill` on buttons; `.price` stays `--accent-2` #1db954 (7.60:1 large text) |

---

## 7. Responsive Plan

- **320px guard**: no horizontal scroll at 320px — hard gate C5 (`document.documentElement.scrollWidth <= innerWidth` at 320px). Review fixed widths (player-bar art 48→40 @<360, track-row grid 2.5rem/1fr/auto min, fullscreen art `min(72vw,…)` already fluid).
- **Safe-area**: `padding-bottom: env(safe-area-inset-bottom)` on `.bottom-bar` and `#player-full` (iOS PWA notch).
- **Breakpoints (keep, verify-only)**: `640px` (loja topbar stacking, max-width 560px), `767px` (sidebar hidden → mobile-topbar, page padding 196px), `768px` (card/queue density). Existing MQs are sound (research pass 3 verdict) — T4 re-verifies each, adjusts only if a gate fails. No new breakpoints planned; `@media (hover:none)` paths preserved.
- Touch targets ≥44px on coarse pointers (audit icon buttons in player bar + like buttons).

---

## 8. Animation Plan

- **Dur/ease tokens**: `--dur-fast 0.15s · --dur 0.2s · --dur-slow 0.25s`; `--ease: cubic-bezier(0.4,0,0.2,1)`.
- Transitions ≤0.25s, **transform/opacity/background-color only — never `transition: all`**; hover lifts via transform (no layout shift); overlays fade+slide (opacity/transform).
- **`@media (prefers-reduced-motion: reduce)` block at the END of style.css** (after base rules, per PLAN.md): kills spinner rotation, loading shimmer, card-play pop, all transitions/animations; keeps opacity-based state changes instant.
- Existing keyframes (spin, playing indicator) tokenized but geometry unchanged.

---

## 9. Accessibility Plan

- **Global focus**: `:focus-visible { outline: 2px solid var(--focus, #8ab0ff); outline-offset: 2px; }` + `@supports not selector(:focus-visible)` fallback (`:focus` rule). **Never `outline: none` on interactive elements.** Removes the current gap: buttons/cards/nav/rows have no focus affordance today.
- **`aria-pressed`** on play/pause + like toggles (static markup in index.html/loja.html; app.js `el()` templates get aria attributes ONLY — no JS logic changes, player.js untouched).
- **`aria-label`** on icon-only controls (play, like, track-add, back-link, player buttons).
- **Contrast**: all 4 failing groups fixed (§3, §10) — ≥4.5:1 text, ≥3:1 UI.
- **Semantics preserved**: native `<input type="range">` sliders stay native; `.tab-btn` role stays as-is (no role changes).
- Reduced-motion + 200%-zoom safe clamp typography (§4, §8).

---

## 10. BEFORE / AFTER Contrast Table

WCAG 2.1 relative luminance, exact (no rounding): R' = ((c/255+0.055)/1.055)^2.4 for c/255 > 0.03928 else c/255/12.92; L = 0.2126R'+0.7152G'+0.0722B'; ratio = (L1+0.05)/(L2+0.05). All values computed this task with the reference script; pairs are identical fg/bg so T3 can re-measure.

### 10.1 The 4 failing groups → fixes

| Pair (fg on bg) | BEFORE | Verdict | AFTER (target) | AFTER ratio | Verdict |
|---|---|---|---|---|---|
| #fff on .btn-accent fill (#618dff → #3865f8) | 3.11 | FAIL 4.5 (ok 3:1) | #fff on #3865f8 | **4.79** | PASS |
| #fff on .buy-btn/.login-box fill (#1db954 → #15803d) | 2.59 | FAIL even 3:1 | #fff on #15803d | **5.02** | PASS |
| --faint #6b6b73 on #0b0b0f | 3.72 | FAIL | #8a8a93 on #121218 | 5.74 | PASS |
| --faint #6b6b73 on #121216 | 3.54 | FAIL | #8a8a93 on #1b1b20 | 5.46 | PASS |
| --faint #6b6b73 on #1a1a1f | 3.28 | FAIL | #8a8a93 on #22222a | 4.61 | PASS |
| loja --faint #6b6b76 on #1c1c24 | 3.22 | FAIL | #8a8a93 on #1b1b20 | 5.46 | PASS |
| --grid #2a2a2a vs #0b0b0f | 1.37 | FAIL 3:1 | #6d6d75 vs #121218 | 3.83 | PASS |
| --grid #2a2a2a vs #121216 | 1.30 | FAIL 3:1 | #6d6d75 vs #1b1b20 | 3.64 | PASS |
| --grid #2a2a2a vs #1c1c24 | 1.18 | FAIL 3:1 | #6d6d75 vs #1b1b20 | 3.64 | PASS |
| .admin-row border #26262f vs #121216 | 1.25 | FAIL 3:1 | #6d6d75 vs #1b1b20 | 3.64 | PASS |
| .modal border #2c2c36 vs #1c1c24 | 1.23 | FAIL 3:1 | #6d6d75 vs #22222a | 3.08 | PASS |
| scrollbar hover #3a3a42 vs #0b0b0f | 1.74 | FAIL 3:1 | #6d6d75 vs #121218 | 3.83 | PASS |
| --hover #24242b vs #121216 | 1.21 | EXEMPT (1.4.11) | #24242b (kept) | 1.21 | EXEMPT |

### 10.2 Passing pairs — must NOT regress

| Pair | Ratio | Verdict |
|---|---|---|
| #fff on #0b0b0f | 19.64 | PASS |
| #fff on #121216 | 18.69 | PASS |
| #f0f0f4 on #121216 | 16.44 | PASS |
| --subtext #a0a0a8 on #0b0b0f / #121216 / #1a1a1f / #1c1c24 | 7.57 / 7.20 / 6.68 / 6.52 | PASS |
| #618dff as TEXT on #0b0b0f / #121216 | 6.33 / 6.02 | PASS |
| #1db954 as TEXT on #0b0b0f / #1c1c24 | 7.60 / 6.54 | PASS |
| danger #f87171 on #121216 | 6.76 | PASS |
| warn #ffcf6b on #121216 | 12.80 | PASS |
| #000 on #fff | 21.00 | PASS |
| chip text #3ddc84 on composite #14291f | 8.61 | PASS |

### 10.3 Target-palette proof rows (all new tokens)

| Pair | Ratio | Verdict |
|---|---|---|
| #fff on #3865f8 (accent fill) | 4.79 | PASS ≥4.5 |
| #fff on #2f5fff (accent fill alt) | 5.00 | PASS |
| ⚠ #fff on #4d7dff (plan's WRONG fill) | 3.69 | **FAIL — do not use as fill** |
| #4d7dff (accent text) on #121218 / #1b1b20 / #22222a | 5.33 / 4.65 / 4.28 | PASS surfaces 1–2; ⚠ text-only on surface-3 |
| #fff on #15803d (green fill) | 5.02 | PASS |
| ⚠ #15803d as TEXT on #0b0b0f / #1c1c24 | 3.92 / 3.37 | FAIL — fill tone never used as text |
| #1db954 (green text) on #121218 / #1b1b20 / #22222a | 7.60 / 6.54 / 6.10 | PASS |
| #8a8a93 (faint) on ramp #121218/#1b1b20/#22222a | 5.74 / 5.01 / 4.61 | PASS |
| ⚠ #6a6a70 vs #22222a | 2.94 | FAIL — use #6d6d75+ |
| #6d6d75 (outline) vs #121218 / #1b1b20 / #22222a | 3.83 / 3.64 / 3.08 | PASS |
| #707078 (outline safer) vs ramps | 4.00 / 3.81 / 3.22 | PASS (extra margin) |
| #8ab0ff (focus) on #1b1b20 / #22222a | 8.66 / 7.32 | PASS |
| #b0b0b8 (sub) on ramps | 9.12 / 8.67 / 7.33 | PASS |
| #f5f5f7 (on-surface) on ramps | 18.04 / 17.16 / 14.50 | PASS |
| Light: #1a1a1f/#f5f5f7, #4a4a55/#f5f5f7, #666670/#f5f5f7, #2f5fff/#f5f5f7, #6d6d75/#f5f5f7 | 15.92 / 8.03 / 5.21 / 4.59 / 4.71 | PASS |

---

*Report-only deliverable. No source files under `web/` were modified. Admin credentials were entered only through the browser login form, never written to any file. Proposal supersedes any earlier `#4d7dff`-as-fill target in PLAN.md/research pass 2 (3.69:1 fails; corrected fill #3865f8 = 4.79:1).*
