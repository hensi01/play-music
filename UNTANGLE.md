# Untangling play-music

Two phases, as the workflow demands: first *every* problem is listed with a
location and nothing is changed; then the redesign is written above the
problems it removes; only then does the code move.

## Phase 1 — the problems

### Hidden links (a fact lives in one place, the code reaches for it in another)

| # | Where | What is hidden |
|---|-------|----------------|
| H1 | `server/helpers.go:117` `filterUser` -> `store/access.go` | `filterUser` always returns `""`, so the whole access-filter machinery (`HasAccessFilter`, `visibleSongSet`, `visibleAlbumSet`, `CanAccess*`, `GrantedCategoryIDs`) never filters anything. ~25 call sites pass an "access user" that is always `""`. |
| H2 | `server/handlers_push.go:78,97,142,174,252,292`, `push_campaigns.go:72,80,102,144,146,152,195,205,213,220,228`, `api_keys.go:33,68,85,108`, `handlers_downloads.go:56,66`, `handlers_api.go:708` | The server builds SQL against tables owned by `internal/store`; `store.Pool()` is the door. Table shapes, column names and indexes are checked in two packages. |
| H3 | `artwork/artwork.go:268` | `invalidate` type-asserts its own `cache` interface to `*redisCache` to read the unexported `client` field, so the interface does not describe what callers need. |
| H4 | `server/server.go:53` vs `stream/stream.go:20` vs `web/assets/api.js:169` | The native-format list exists three times; nothing keeps them equal. |
| H5 | `server/handlers_api.go:1054` vs `scanner/scanner.go:30` | Audio extension knowledge exists twice; the upload path and the scan path disagree about `.tak`. |
| H6 | `metadata/metadata.go:189` `ffmpegConfigured` + `SetFFmpegPath` | Package-level mutable global holds config; `stream.Service` resolves ffmpeg a different way from the same config field, and `main.go` must remember to call a setter. |
| H7 | `server/handlers_ads.go:34` | The ad host is hardcoded next to request validation; the fact belongs to the ad provider contract. |
| H8 | `main.go:117` | Logs `"1.0.0"` while `version.Version` is the real version. |

### Needless links (a link that buys nothing)

| # | Where | What |
|---|-------|------|
| N1 | `store/store.go:19` `Pool()` | Exists only so the server can run SQL (H2) and so three tests can make fixtures. |
| N2 | `store/store.go:22,31` | `NewID` is a one-line wrapper over `newID`; two names, one function. |
| N3 | `server/helpers.go:61` `parseID` | Renames `r.PathValue("id")` at 20 call sites. |
| N4 | `server/static.go:83` `quoteJSON` | One-line wrapper over `json.Marshal` at one call site. |
| N5 | `server/handlers_api.go:898` `mustCategories` | Swallows an error and re-runs `GetCategories` a second time for the same response. |
| N6 | `server/handlers_api.go:23` `userFromContext` | Used once, inline-able. |
| N7 | `db/db.go:90` `InTx`, `metadata/metadata.go:132` `ExtractThumb`, `metadata/metadata.go:196` `ConfiguredFFmpegPath`, `storage/storage.go:161` `keyFromURL` | No callers. |
| N8 | `store/albums.go:73,93,226`, `store/store.go:129`, `store/songs.go:52`, `store/users.go:55`, `store/access.go:117` | No callers (`RecentlyAddedAlbums`, `MostPlayedAlbums`, `Genres`, `AlbumsWithCoverInFolder`, `GetSongByPath`, `GetUserByUsername`, `GrantedCategoryIDs`). |
| N9 | `server/handlers_push.go:296` `handlePushSend` | No route; only a test calls it. A second, parallel copy of the campaign validation. |
| N10 | `server/handlers_push.go:262-264` | Reads `management_token` in `handlePushUnsubscribe` and never uses it. |

### Surplus (one conceptual change requires several edits)

| # | Where | Copies |
|---|-------|--------|
| S1 | `server/handlers_api.go:910,949,988,1175` | The 15 MB multipart photo block, four times. |
| S2 | `server/handlers_api.go:490,511` and `:454,482,486` | `handleLike`/`handleUnlike` differ only by a boolean; same for album likes. |
| S3 | `artwork/artwork.go:356` and `stream/stream.go:148` | Byte-identical cache-trim loops in two packages. |
| S4 | `store/songs.go:26` vs `:362`, `albums.go:253` vs `:264`, `artists.go:85` vs `:94` | Each row of a result is scanned by two identical bodies. |
| S5 | `store/playlists.go:87-99` | `GetPlaylist` retypes the `songCols` destination list by hand. |
| S6 | `store/categories.go:15` vs `:43` | The category upsert CTE, twice. |
| S7 | `store/*.go` (12 loops) | Hand-rolled "close, iterate, scan, collect" loops; they disagree on whether an empty result is `nil` or `[]`. |
| S8 | `store/songs.go:201` | Two `INSERT INTO history` branches for the NULL owner. |
| S9 | `store/albums.go:114` | `strings.Replace(albumCols, "FALSE AS liked", ...)` patches a constant at runtime. |
| S10 | `server/handlers_push.go:191,205` + `push_campaigns.go:237,246` + `:318` | Relay response shapes (enrollment ticket, subscription, management token) written three times. |
| S11 | `server/handlers_push.go:323` + `push_campaigns.go:152` | The recipient query, twice, with two different row loops. |
| S12 | `server/handlers_push.go:353` + `push_campaigns.go:175` | The 100-per-batch relay loop, twice. |
| S13 | `server/handlers_push.go:312` + `push_campaigns.go:34` | Title/body/URL/size validation and payload construction, twice. |
| S14 | `server/handlers_push.go:190` + `push_campaigns.go:230` | `"/api/apps/" + url.PathEscape(...)`, twice. |
| S15 | `server/handlers_push.go:196,213` + `push_campaigns.go:242,253` | The "ticket, then subscription" relay dance, twice. |
| S16 | `server/handlers_push.go:148,180,258` | The same advisory-lock statement three times. |
| S17 | `server/api_keys.go:77` `apiKeyInfo`, `handlers_downloads.go:72` `entry`, `push_campaigns.go:18` `pushCampaign` | JSON shapes declared in the handler that the store also builds. |
| S18 | `server/handlers_api.go:626,718` | Admin/client identity validation in create and update. |
| S19 | `web/assets/app.js:15` + `admin.js:203` | `el()`, twice — and the admin copy has no `html` branch, so four `el(..., { html: '&#128247;' })` icons never render. |
| S20 | `web/assets/app.js:32,41` + `admin.js:643` | Duration formatting, three ways. |
| S21 | `web/assets/app.js:165,1369`, `player.js:15`, `admin.js:101`, `app.js:176` | JWT payload decoding, five times. |
| S22 | `server/push_campaigns.go:116` | Two tickers with two intervals inside one blocking function. |
| S23 | `server/handlers_push.go:165` | `"pending-"+store.NewID()` for a relay id next to the separate operation id — one row, two ids, no explanation of which is which. |

### Special cases (a rule with an exception the code must remember)

| # | Where | Exception |
|---|-------|-----------|
| X1 | `server/handlers_push.go:72` | The push gate can be switched off by a request header. |
| X2 | `server/handlers_auth.go:23` | `POST /auth/login` routes to admin login when `phone` is empty. |
| X3 | `server/static.go:51` | `loja.html` is special-cased so a removed page still 404s. |
| X4 | `store/store.go:39,84`, `store/user.go:33` | "No rows" detected by comparing `err.Error()` strings. |
| X5 | `server/api_keys.go:35` | `err == pgx.ErrNoRows` instead of `errors.Is`. |
| X6 | `scanner/scanner.go:316` | Genre `"music"` is skipped by name. |
| X7 | `store/albums.go:113` | `LikedAlbums` takes two user ids; the other methods take one or none. |
| X8 | `store/playlists.go:323` | `validateSongAccess` also enforces "the ids exist" by counting rows. |
| X9 | `server/handlers_push.go:142,174` | A DB transaction is held open across two HTTPS calls. |

### Tests

| # | Where | Problem |
|---|-------|---------|
| T1 | `server/handlers_push_test.go:60` | Exercises `handlePushSend`, a handler with no route (N9). |
| T2 | `server/removed_karaoke_test.go` | Asserts 404 for four routes that do not exist; `static_test.go` already covers "unknown `/api/*` is a JSON 404". Cannot fail. |
| T3 | `server/api_keys_test.go:??` `TestOpenAPICoversRegisteredAPIRoutes` | Greps `server.go` for `mux.Handle(Func)?`, a pattern that cannot match the `mux.Handle("POST /api/…", …)` form used by almost every route, so coverage is far weaker than the name claims. |
| T4 | `server/handlers_bugs_test.go:287` | Duplicated mojibake assertion; one variant (`"Áudio inválido"`) can never match. |
| T5 | `server/handlers_bugs_test.go:225,305,339` | Reach through `store.Pool()` (H2/N1) for fixtures. |
| T6 | `server/static_test.go:38`, `handlers_bugs_test.go:26,196`, `handlers_bugs_test.go:145` | Comments cite ticket ids (B-NEW-1/3/4, B2) instead of the external fact worth keeping. |

## Phase 2 — the redesign

1. **One access model.** Everybody sees the whole catalogue. Delete
   `filterUser`, `access.go`, and every `userID`/`accessUserID` parameter that
   only fed the filter. Keep owner scoping (`Likes`, `History`, `Playlists`,
   `Download`) because that decides whose rows you read, not what is visible.
   *Removes H1, and the argument lists that existed only for it.*
2. **The store owns the database.** Add `store/push.go`, `store/api_keys.go`,
   `store/downloads.go`, `store.CountAdmins`; delete `Store.Pool()`. The
   server never writes SQL again. *Removes H2, N1, T5.*
3. **One row-scanning path.** `column → destination` lists per model, one
   `collect[T]` for every list query, and `scanSong` accepts anything with
   `Scan(...any) error`. *Removes S4, S5, S7, and the nil/empty-slice
   disagreement that forced `writeJSON`'s reflection hack.*
4. **One push pipeline.** `pushMessage` (validate + payload), `enrollPush`
   (ticket → subscription → management token), `relayPath`, `relaySend`
   (batching), named relay response types, one recipient query, one store
   method per state change, and one `PushCampaign` shape in `model`.
   Delete `handlePushSend`. *Removes N9, S10–S16, S23, X9.*
5. **One photo path.** `readPhoto` for the shared multipart block, one
   `UploadPhoto`/`DeletePhoto` pair in `artwork` keyed by entity type.
   *Removes S1.*
6. **One like path.** `setLike` for songs and albums. *Removes S2.*
7. **One cache trim.** `internal/fscache.EnforceLimit`, used by both caches;
   `cache.Invalidate` becomes part of the interface. *Removes S3, H3.*
8. **One audio-format table.** `stream` exposes the format facts (native or
   not, MIME type); the scanner and the upload handler read it. *Removes H4,
   H5.*
9. **Config is passed, not stored globally. ** `metadata.Read` takes the
   configured ffmpeg path; one resolver. *Removes H6.*
10. **Routes are data.** A `routes()` slice registers the mux and lets the
    OpenAPI test compare real routes with the spec. *Removes T3.*
11. **Front end: one `dom.js`, one `format.js`, one JWT decoder in `api.js`.**
    *Removes S19–S21.*
12. **Tests follow behaviour**: drop the tests for deleted code, make the
    OpenAPI test read the route table, fix the duplicated assertion, delete
    the karaoke 404 tautology, and stop reaching for `Pool()`.

Behaviour is unchanged except where the old behaviour was a defect:
inert access parameters still admit everyone, the admin icon slots start
rendering the icons they were written for, and `stream` picks up the same
ffmpeg stat-check the metadata path already had.

## Phase 3 — the hunter rounds

Re-read the code as if it were written by someone else, looking for the four
categories again. Each round's findings and what became of them:

### Round 1 — front end

| Finding | Fix |
|---|---|
| `admin.js` had `uploadSongPhoto`/`removeSongPhoto` twice, the inner copy nested in the category modal | the modal calls the module-level pair |
| the single-file and folder uploaders each wired click/dragover/dragleave/drop by hand | one `wireDropzone(dropzone, fileInput, onChange)` |
| `admin.js` computed `${(f.size/1024/1024).toFixed(2)} MB` beside an imported `fmtMB` | `fmtMB(f.size)` |
| `FOLDER_AUDIO_EXTS` copied `stream/formats.go`; the uploader hid `.tak`/`.ape`/`.wv` the server accepts | `GET /api/settings` now carries `audioExtensions` from the format table; the uploader filters on that |
| `adminState.users` was assigned and never read | field deleted |
| `cat-name`, `cat-songs`, `cat-song-filter`, `cat-photo-preview`, `cat-error`, `folder-upload-start`, `song-*` and the dropzone classes existed only so the creating function could re-find its own nodes | the functions keep direct references; the lookups and ids are gone |
| the library tabs rendered `el('p', {style:'color:var(--subtext)'})` instead of `emptyState()` | `emptyState(msg, hint?)`; `emptyState2` folded into it |
| `appConfig` was assigned and never read | deleted |
| the `video` and `cart` icons had no `icon()` call anywhere | deleted |
| `musicas()` in `app.js` and `plural()` in `admin.js` were the same function under two names | one `pluralize(n, singular, plural)` in `format.js` |
| `isChatGPTBrowser()` in `app.js` re-implemented `chatGPTNotificationBypass()` in `api.js` | one exported `isChatGPTBrowser()` in `api.js` |
| the iOS regex lived in `app.js` and `pwa.js`; `isIOS()` was not exported | `platform.js` publishes `window.__pmPlatform` for both script kinds |
| `typeof verifyNotificationGate === 'function'` guarded a hoisted same-module function | guards removed |
| `showToast` looked up `#app-toast`, a node it had itself created | module-level `toastEl` |
| `pm_token`, `pm_push_owner`, `pm:rerender`/`pm:unauthorized`/`pm:playlists-changed` were typed in several modules | `TOKEN_KEY`, `PUSH_OWNER_KEY`, `RERENDER_EVENT`, `UNAUTHORIZED_EVENT`, `PLAYLISTS_CHANGED_EVENT` |
| `app.js` removed `.pwa-overlay` and `pwa.js` checked for `.notification-gate` — each reaching into the other's markup | `__pmPlatform.dismissInstall()` and `__pmPlatform.gateVisible` |
| the install card borrowed `notification-gate-icon`/`-eyebrow`/`-description` | renamed `promo-*`, the two promo screens now share them |
| `ads.js` hardcoded the partner origin that `handlers_ads.go` also owns | derived from the normalized `embedUrl` |
| eleven `endpoints.*` wrappers had no caller while the server still served the routes | deleted from `api.js`; the routes stay in the API and the spec |

### Round 2 — backend

| Finding | Fix |
|---|---|
| the six `store` sentinels were six bare `var` declarations | one `var (...)` block that reads as the failure vocabulary |
| `static_test.go` had a stray brace-level indent | fixed |
| `testStaticServer` kept `/pwa.js` in the known-assets list but not the new `platform.js` | added |

### Round 2 — tests

The workflow puts tests last, so the questions were only asked once the code
had settled:

| Question | Finding |
|---|---|
| does the test repeat the implementation? | `TestAudioExtensionsMatchesAcceptedSet` compares `AudioExtensions()` against `formats` itself — it is the one place that *must* agree with the table, and the assertion is the contract, not a copy of the loop body. `TestIsNativeMatchesFormatTable` and `TestMIMEForExtIsCaseInsensitive` pin the semantics, not the map. |
| does it depend on internals? | every integration test goes through `s.Handler()` and an HTTP call; `handlers_bugs_test.go` still drives handlers directly, which is the only way to assert a 404 for an unregistered method without a route. |
| can it fail? | `TestStaticKnownAssets` was extended to `platform.js`, so a missing embed now fails the suite. `TestSettingsListsUploadableAudioExtensions` fails if the server stops sending the list the uploader reads. |
| is it testing deleted code? | yes — `removed_karaoke_test.go` and the `handlePushSend` test were deleted earlier in the run. |

### Round 3 — backend

| Finding | Fix |
|---|---|
| `"Desconhecido"` existed in four writers and two lowercased comparisons in `recommendations.go` | `store.UnknownArtist`; the comparisons use `strings.ToLower`/`strings.EqualFold` on it |
| `os.CreateTemp("", "pm-scan-*…")` and the `"pm-scan-"` title check in the scanner were two copies of the same prefix | `scanTempPrefix` |
| `UserCategories` re-declared the category columns and scan loop | reuses `categoryCols` + `collectCategories` |
| `GetOrCreateCategory` had an `errors.Is(err, ErrNotFound)` branch that `upsertCategoryID` cannot return | branch deleted |
| `handleRegister` built its own login map, and it was missing `username` | one `writeLoginResponse` + `session` struct used by register and both logins |
| the phone display format was applied inline at four call sites and missing at `handleMe` | `formatPhone(*model.User)` in `helpers.go`, called by every response that carries a phone |
| `touchAPIKey` selected `k.id` and scanned it into an unused variable | returns only the owner |
| `requireAdmin`'s malformed-key test hardcoded `pmk_` + 43 chars | `apiKeyHeader`/`apiKeyPrefix`/`apiKeyBody`/`apiKeyLen` now own the key shape |
| `relayFailure.Queued` was set and never read | field deleted |
| `scanner.Result.Error` was assigned in three places and never read | field deleted; the failures were already logged |
| `processAudio` passed `prev` into `indexAudio`, which ignored it | parameter deleted |
| `(*Server).transcode` was a one-line pass-through to `stream.Transcode` | inlined |
| `handleAdminListCategories` was `handleCategories` under a second name | route points at `handleCategories` |
| `statusRecorder.Write` set a status the recorder never needs | method deleted |
| `writeJSON` had a `v == nil` early return no caller can hit | branch deleted |
| `artistDetail.AlbumCount` was `len(albums)` shadowing the SQL count already on the embedded `model.Artist` | field deleted; `albumCount` comes from the embedded struct |
| `entityExists` ended in an unreachable fall-through | `default:` panics, naming the type |
| `randomSecret` returned the literal `"play-music"` on a `crypto/rand` failure, while `store.NewID` panics | panics with the same reason |
| `ND_S3_SECURE` was ignored when `ND_MUSICFOLDER` had already set `Secure` | an explicit `ND_S3_SECURE` now always wins |
| `CacheDirs` was a struct wrapping one string | `Service.dir` |
| `artwork.Service` stored `cfg` and `dir` and read neither | fields deleted |
| the artwork default size `300` was written in the handler and again in the service | `artwork.DefaultSize` |
| `Art.Mime` and the `mime` column were never read — every cover is re-encoded to JPEG | `GetArt` no longer selects it; `model.Art` is data-only |
| `phone.Mask` had no caller outside its own test | deleted with `TestMask` |
| `metadata.Read`, `ProbePath` and `Probe` were exported with no external caller | unexported |
| `mimeForStatic` was a switch over two extensions whose owners were the two `versioned` entries | the content type lives in the `versionedAsset` entry |
| `"Mi Ritmo"` was written twice in one settings payload | `appName` const |
| an unknown `categoryId` on upload was silently ignored | 400 before the song is assigned |
| the 2-minute transcode cache window was an unexplained literal | `cacheSkew` with the clock-skew reason |
| the model comment claimed migration 0008 dropped the genre column; it deleted a category | comment corrected to what `handleHome` actually does |

### Round 3 — front end

| Finding | Fix |
|---|---|
| `readAppConfig` handled `window.__APP_CONFIG__` being a JSON string; the only producer writes an object | returns the object, with a `??` fallback |
| `__APP_CONFIG__.firstTime` was written and never read | removed from `index.html` and the fallback |
| `else if (!navigator.locks)` in the notification card was unreachable — the branch above requires `navigator.locks` | deleted |
| the push-capability predicate was written three times with slightly different shapes | `supportsPushNotifications()` used by all three |
| `renderSearch` read a `genre` query parameter nothing produces | deleted, with the now-unused `parseHash()` call |
| `album-shuffle` was a class with no CSS rule | deleted |
| `card()`'s `square` parameter was never passed; the ternary was constant | parameter deleted, class literal |
| `trackRow`'s `compactActions = false` default encoded a two-mode flag used by one caller | default dropped |
| the queue marker markup was built twice | `currentMarker()` + `queueRow()` |
| three identical `container.innerHTML = ''; container.append(emptyState(err.message))` handlers | `renderError(container, err)` |
| the error-body parse block in `api.js` was written twice | `errorMessage(res, fallback)` |
| `ads.js`'s `if (button.disabled) return` guarded a state the same handler cannot observe | guard deleted |
| `playAudio` type-tested a promise it had just created | `audio.play().catch(...)` |
| `admin.js` wrapped one array in `assigned = { songIds: [] }` | `assignedSongIds` |
| `endpoints.albums(params)` interpolated a parameter no caller passed | `albums()` |
| `.empty-state` was declared three times, the middle one fully undone by the last | one rule; `padding`/`text-align`/`color` and `margin` merged |
| `sw.js`'s `CACHE_NAME` was a hand-bumped `pm-shell-v20` | `pm-shell-__ASSET_VERSION__`, substituted like the rest of the shell list |

### Round 3 — e2e

| Finding | Fix |
|---|---|
| `pm_token`, `pm_pwa_seen`, `pm_push_owner`, `pm_push_state:<id>` and the auth header name were re-typed across six specs | `e2e/tests/client-keys.js`, required by every spec that needs them |
| `markPwaSeen` was four copies of `sessionStorage.setItem('pm_pwa_seen','1')` | one helper in `client-keys.js` |
| `loginAdmin` clicked an `Administrador` button that `renderLogin` no longer renders | goes to `/admin/login` directly |
| the wrong-password spec drove the client login screen and asserted an admin error | drives `/admin/login` and `Entrar no painel` |
| `song-share.spec.js` logged in through the same missing admin button | uses the client phone login the screen offers |
| `pwa.spec.js` asserted `Instalar PWA`; the card has said `Instalar aplicativo` since before HEAD | asserts the button by role, not by a copy of its label |
| `silentWav` was duplicated byte-for-byte in two specs beside an existing shared builder | one `silenceWavBuffer()` in `helpers.js` |
| `apiCreateUser`, `apiUpdateUser` and the `uniqueSuffix` export had no caller | deleted |
| `helpers.js` posted admin credentials to `/auth/login` (the legacy branch) rather than the route the UI uses | `/auth/admin/login` |



### Round 4 — backend

| Finding | Fix |
|---|---|
| `handlers_media.go` spelled the transcode target `"mp3"` while `stream` actually invoked `libmp3lame`/`-f mp3` | `stream.TranscodeFormat`/`TranscodeCodec`; the `?format=` check and the ffmpeg args both read them |
| every SQL `entity_type='song'|album'|artist'` literal bypassed the `Entity*` constants that claim to own them | the queries concatenate the constants |
| `albumIDs`, `ApplySongLikes` and `ApplyArtistLikes` each hand-rolled the same ID projection | one generic `idsOf` |
| `CategorySongIDs` and `recommendations.songCategories` were the same (song_id, value) fold over different columns | `groupSongCategories(rows)` |
| `Apply*Likes` wrapped a single entity in a slice to read element 0 back | `Store.LikedSet` for the three detail handlers |
| `entityGuard` and `entityExists` were two `switch entityType` tables over the same set | one `probeFor` table of (exists, notFound) |
| the upload's optional `photo` part re-implemented the 15 MB cap and read inline beside `readPhoto` | `photoField(r)`; required vs optional differs only in the missing-part answer |
| `waitForObject` polled inside `ObjectExists`, which already retries 4x inside `Stat` | `storage.WaitForObject` owns the polling and calls `statOnce` once per poll |
| `ImportPlaylist` used `ON CONFLICT DO NOTHING` on a table with no such constraint, so its fallback branch was the only path and "replace" never replaced | explicit select-then-insert |
| `ReorderPlaylistTracks` and `renumberPlaylist` wrote the same renumber loop | `renumberPlaylist(ctx, q, ids)` takes the ordered ids |
| `scanner.Result` was built, incremented under a mutex and returned, and no caller read it | local counters feeding the one log line |
| the push gate hand-copied `/api/me` and `/api/push/` as path prefixes beside the route table | `pushGateExempt` reads the matched `r.Pattern`, so a new push route is exempt without an edit |
| genre `"music"` was a bare literal in the scan loop, introduced by migration 0008 | `store.IgnoredGenre`, documented with the migration |
| `handleLikedAlbums` wrote `Liked` as it came from `LikedAlbums`, whose columns carry no liked flag, so every saved album said `liked:false` | the loop marks them true; the query already selects only liked rows |
| `handleAdminAlbums`/`handleAdminArtists` were byte-identical to the public handlers | the admin routes reuse `handleAlbums`/`handleArtists` |
| `handleHome` and `handleSearch` marshalled `null` for empty results while the published schema says array | both start from empty slices |
| the upload's post-write `GetSong` re-read swallowed its error silently | logged; the response keeps the indexed song |
| `Claims.Subject` was written into every token and no consumer read it | field dropped from the claims literal |
| `stream.flights` kept one mutex per song ever transcoded | `flight{mu, waiters}`, deleted when the last waiter leaves |
| the three 401 write sites for a malformed API key were one fact, and the owner resolution was inline | `apiKeyOwner` + `errNotOurKey`, keeping the 401-vs-503 split |
| `newTestServerStore` and `newFullTestServer` connected to the **app** database, contradicting the disposable-DB rule | both take `testDisposableDB`; `newFullTestServer` keeps `config.Load` only for S3 |
| `testDBURL` survived as a second DB resolver with no caller | deleted; `appDBURL` remains only for the "must not be the app DB" guard |

### Round 4 — comments

Removed the comments that restated their own declaration (`likedEntityIDs returns every id in ids that userID has liked`, `firstBool returns the parsed value of ...`). Kept every comment that states a fact the code cannot show: migration history, the caching proxy, why `?jwt=`, why the shell is substituted by version, why each e2e wait exists. Ticket ids (`B1`, `B4`, `B5`) became the behaviour they named, since the ticket itself is not reachable from the tree.


### Round 5 — front end

| Finding | Fix |
|---|---|
| a refactor of this session had lifted the queue row into `queueRow` and left `queue` out of scope, so every row click on `/queue` threw `ReferenceError` (`/queue` has no spec, so nothing caught it) | `queueRow` takes the queue list as a parameter |
| the library "Playlists" tab was clicked with no assertion after it | the spec asserts the tab is active and the track list is gone |
| two photo dropzones hand-wired click/change, so they never accepted a dropped file while `wireDropzone` existed | both go through `wireDropzone` |
| the modal shell (backdrop + panel + backdrop dismissal) was spelled out five times byte-identically | `mountModal(panelClass, ...children)` |
| `albumCard`'s `from` default and `renderAlbum`'s accepted-return list were two copies of the same set | `albumReturnPaths` holds it; the default is gone and every caller names its screen |
| the track-more menu loop carried `if (!button) continue` because the option table could hold `null` keys, and the two `? null` buttons made that possible | the table is filtered before the loop, so the menu only holds buttons (`compactActions` itself stays: album rows deliberately hide share/add) |
| `renderHome` was the one renderer that appended the error itself instead of calling `renderError`, leaving the greeting above the failure | it calls `renderError` |
| `.detail-art.round` and `.detail-meta .link` were dead rules that a class sweep called used because of `icon('link')` and `stroke-linecap="round"` | deleted |
| `.fullscreen-btn-main.is-playing` re-declared the base background/ink with identical values | deleted (the glow and spinner remain) |
| the endpoint-purge loop was copy-pasted three times, and two bare `endpoints` bindings shadowed the imported API map | `removePushEndpoints(userId, ...endpoints)` |
| two singular/plural ternaries sat on lines that already call `pluralize` | `pluralize` |
| `jwtClaims` and `apiFetch` were exported with no importers anywhere | module-private |
