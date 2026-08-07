# player.js WIP fix — [G1] T2

Date: 2026-08-07
Scope: `web/assets/player.js` only. Changes made in git worktree `../play-music-loop-wt` (detached HEAD @ 8df05211), main working tree untouched.
Deliverables: `loop-reports/wip.patch` (raw WIP, captured before any fix), `loop-reports/player-wip-fix.diff` (worktree diff vs HEAD = WIP applied + fix).

## Per-hunk verdict (7 hunks)

| Hunk | Change | Verdict |
|---|---|---|
| 1 | `audio.volume = 0.8` at module init | **KEEP** — aligns element default with `state.volume = 0.8` (was 1.0 until first setVolume) |
| 2 | `playAudio()` catch: guard removed, `switching = false` added | **FIXED** — guard restored, `switching = false` dropped, explanatory comment added |
| 3 | mediaSession `'play'` → `playAudio()` / `'pause'` → `audio.pause()` | **KEEP** — direct calls use the element's real state instead of possibly-stale `state.playing` |
| 4 | End-of-queue `currentIndex: Math.max(0, queue.length - 1)` | **KEEP** — old `queue.length` was out of bounds, leaving no highlighted row in the queue page |
| 5 | `prev()` restart guarded by `readyState > 0` + explicit `progress: 0` | **KEEP** — defensive; currentTime write only skipped when nothing is loaded |
| 6 | `seek()` readyState guards (defer branch + seekable last resort + currentTime write) | **KEEP** — pre-metadata seeks defer via pendingSeek instead of dropping |
| 7 | Optimistic `progress: Math.max(seconds, 0)` in pending-seek branch | **KEEP** — seekbar responds immediately; real seek re-applied on loadedmetadata |

## The desync bug (hunk 2) and the fix

Original (pre-WIP) catch:

```js
p.catch(() => {
  if (!switching) {
    set({ playing: false })
    setMediaSessionPlaybackState(false)
  }
})
```

WIP removed the guard and added an unconditional `switching = false`:

```js
p.catch(() => {
  switching = false
  set({ playing: false })
  setMediaSessionPlaybackState(false)
})
```

**Failure mechanism:** during a src swap (rapid prev/next/shuffle), `loadAndPlay()` sets `switching = true` and hands the element a new `src`; the browser then aborts the old track's `play()` promise with `AbortError`. The WIP catch:

1. Flips the UI to paused mid-load (`playing: false`) even though the new track is starting — and if the rejection lands after the new track's `play` event (racy, common on fast swaps), the UI stays paused while audio actually plays;
2. Clears `switching`, disarming the `pause`-event consumption (the `pause` handler at player.js:80 checks the flag) so the load-algorithm `pause` event now also flips the UI.

**Fix applied (worktree player.js:117-123):**

```js
p.catch(() => {
  // During a src swap the previous track's play() promise rejects with an
  // AbortError; ignore it (the swap's own pause/play events settle state),
  // and never clear `switching` here or the load-algorithm pause event
  // would stop being consumed — desyncing the UI from the audio element.
  if (!switching) {
    set({ playing: false })
    setMediaSessionPlaybackState(false)
  }
})
```

Why this is safe: a rejection while `switching === true` is always the aborted old track's play() — the swap's own pause/play events settle the UI. A rejection while `switching === false` (e.g. autoplay-policy denial) still sets `playing = false` honestly. No error-name special-casing needed (minimal diff, per research).

## Verification

- `node --check web/assets/player.js` in the worktree → **exit 0** (Node v24.19.0, ESM auto-detected).
- All 6 kept hunks re-confirmed present in the worktree file (volume line 8, mediaSession handlers 134-135, end-of-queue clamp, prev/seek readyState guards, optimistic pending-seek).
- Final diff stat (worktree vs HEAD): `web/assets/player.js | 25 ++++++++++++++++++------- 1 file changed, 18 insertions(+), 7 deletions(-)` — saved to `loop-reports/player-wip-fix.diff` (LF, no UTF-16 BOM).
- Raw WIP saved to `loop-reports/wip.patch` (LF) before the fix.
- Main tree: `git diff --stat web/assets/player.js` still shows exactly **+17/-11** — the pre-existing WIP, unchanged.

## Manual QA checklist (for T3)

- Rapid prev/next/shuffle during playback: UI must stay playing, no pause flash, `state.playing` matches `audio.paused === false`.
- Autoplay-policy rejection (blocked autoplay): UI goes paused honestly.
- Queue page after next past the last track (repeat off): last row highlighted, progress 0.
- Seek before metadata loads and after: seekbar responds, position lands correctly after loadedmetadata.
- mediaSession play/pause from system controls match the UI; volume slider reflects 0.8 default.
- Console: zero uncaught exceptions / zero console errors.
