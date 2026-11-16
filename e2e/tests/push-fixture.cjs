exports.mockActivatedPush = async (page, userId) => {
  await page.addInitScript((id) => {
    const endpoint = 'https://push.example.test/device'
    localStorage.setItem(`pm_push_state:${id}`, JSON.stringify({ enabled: true, endpoint }))
    localStorage.setItem('pm_push_owner', id)
    const subscription = { endpoint, toJSON: () => ({ endpoint, keys: { p256dh: 'key', auth: 'auth' } }) }
    const registration = { pushManager: { getSubscription: async () => subscription } }
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: { ready: Promise.resolve(registration), addEventListener() {}, register: async () => registration } })
    Object.defineProperty(window, 'PushManager', { configurable: true, value: function PushManager() {} })
    Object.defineProperty(window, 'Notification', { configurable: true, value: { permission: 'granted' } })
  }, userId)
}
exports.tokenFor = (id) => `x.${Buffer.from(JSON.stringify({ uid: id })).toString('base64url')}.x`
