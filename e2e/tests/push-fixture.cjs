// Mocked Web Push registration for specs that need an activated device.
const { PUSH_OWNER_KEY, pushStateKey } = require('./client-keys')

exports.mockActivatedPush = async (page, userId) => {
  await page.addInitScript(({ id, ownerKey, stateKey }) => {
    const endpoint = 'https://push.example.test/device'
    localStorage.setItem(stateKey, JSON.stringify({ enabled: true, endpoint }))
    localStorage.setItem(ownerKey, id)
    const subscription = { endpoint, toJSON: () => ({ endpoint, keys: { p256dh: 'key', auth: 'auth' } }) }
    const registration = { pushManager: { getSubscription: async () => subscription } }
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: { ready: Promise.resolve(registration), addEventListener() {}, register: async () => registration } })
    Object.defineProperty(window, 'PushManager', { configurable: true, value: function PushManager() {} })
    Object.defineProperty(window, 'Notification', { configurable: true, value: { permission: 'granted' } })
  }, { id: userId, ownerKey: PUSH_OWNER_KEY, stateKey: pushStateKey(userId) })
}
exports.tokenFor = (id) => `x.${Buffer.from(JSON.stringify({ uid: id })).toString('base64url')}.x`
