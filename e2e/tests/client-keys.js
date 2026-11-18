// The browser facts the specs must reproduce: localStorage/sessionStorage keys
// and the auth header name.
//
// These values are owned by the web app (`web/assets/api.js` TOKEN_KEY and
// AUTH_HEADER, `web/assets/pwa.js` SEEN_KEY, `web/assets/app.js` PUSH_OWNER_KEY
// and pushStateKey) and by the server (`internal/auth` HeaderName). The specs
// cannot import the ESM modules, so every copy lives here instead of being
// retyped per spec: renaming a key is now one edit plus these constants.

const TOKEN_KEY = 'pm_token'
const PWA_SEEN_KEY = 'pm_pwa_seen'
const PUSH_OWNER_KEY = 'pm_push_owner'
const AUTH_HEADER = 'X-ND-Authorization'

// pushStateKey mirrors app.js's per-user push state entry.
function pushStateKey(userId) {
  return `pm_push_state:${userId}`
}

// markPwaSeen suppresses the install overlay: most specs are not about it.
async function markPwaSeen(page) {
  await page.addInitScript((key) => sessionStorage.setItem(key, '1'), PWA_SEEN_KEY)
}

module.exports = { TOKEN_KEY, PUSH_OWNER_KEY, AUTH_HEADER, pushStateKey, markPwaSeen }
