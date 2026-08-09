// ─── web-push-manager.test.ts ─────────────────────────────────────────────────
// Unit tests for WebPushManager — TASK-041.
// Mocks web-push library, uses a fake Store.

import { describe, it, expect, vi, beforeEach } from 'vitest'

// ── Mock web-push ──────────────────────────────────────────────────────────────
vi.mock('web-push', () => {
  const mockVapidKeys = { publicKey: 'pk_test', privateKey: 'sk_test' }
  return {
    default: {
      setVapidDetails: vi.fn(),
      generateVAPIDKeys: vi.fn(() => mockVapidKeys),
      sendNotification: vi.fn(async () => ({ statusCode: 201 }))
    }
  }
})

import webPush from 'web-push'
import { WebPushManager } from '../../notifications/web-push-manager'
import type { WebPushSubscription } from '../../../shared/types'

// ── Fake Store ─────────────────────────────────────────────────────────────────

function makeFakeStore(initial?: {
  vapidKeys?: { publicKey: string; privateKey: string } | null
  webPushSubscriptions?: WebPushSubscription[]
}) {
  let vapidKeys = initial?.vapidKeys ?? null
  let subscriptions: WebPushSubscription[] = initial?.webPushSubscriptions ?? []

  return {
    getVapidKeys: vi.fn(() => vapidKeys),
    setVapidKeys: vi.fn((keys: { publicKey: string; privateKey: string }) => {
      vapidKeys = keys
    }),
    getWebPushSubscriptions: vi.fn(() => subscriptions),
    setWebPushSubscriptions: vi.fn((subs: WebPushSubscription[]) => {
      subscriptions = subs
    })
  }
}

type FakeStore = ReturnType<typeof makeFakeStore>

// ── Tests ──────────────────────────────────────────────────────────────────────

describe('WebPushManager', () => {
  let mockWebPush: typeof webPush
  let store: FakeStore
  let manager: WebPushManager

  beforeEach(() => {
    vi.clearAllMocks()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    mockWebPush = (webPush as any).default ?? webPush
  })

  describe('loadOrCreateVapidKeys()', () => {
    it('tạo keys mới nếu store rỗng', () => {
      store = makeFakeStore({ vapidKeys: null })
      manager = new WebPushManager(store as never)

      expect(mockWebPush.generateVAPIDKeys).toHaveBeenCalledOnce()
    })

    it('persist keys vào store khi tạo mới', () => {
      store = makeFakeStore({ vapidKeys: null })
      manager = new WebPushManager(store as never)

      expect(store.setVapidKeys).toHaveBeenCalledWith(
        expect.objectContaining({ publicKey: 'pk_test', privateKey: 'sk_test' })
      )
    })

    it('reuse keys đã có — không tạo lại', () => {
      const existingKeys = { publicKey: 'existing_pk', privateKey: 'existing_sk' }
      store = makeFakeStore({ vapidKeys: existingKeys })
      manager = new WebPushManager(store as never)

      expect(mockWebPush.generateVAPIDKeys).not.toHaveBeenCalled()
      expect(store.setVapidKeys).not.toHaveBeenCalled()
      expect(manager.getPublicKey()).toBe('existing_pk')
    })
  })

  describe('getPublicKey()', () => {
    it('trả về VAPID public key', () => {
      store = makeFakeStore({ vapidKeys: { publicKey: 'pub', privateKey: 'priv' } })
      manager = new WebPushManager(store as never)

      expect(manager.getPublicKey()).toBe('pub')
    })
  })

  describe('saveSubscription()', () => {
    beforeEach(() => {
      store = makeFakeStore()
      manager = new WebPushManager(store as never)
    })

    it('tạo record với id mới', () => {
      const sub = { endpoint: 'https://ep1.test', keys: { auth: 'a', p256dh: 'b' } }
      const record = manager.saveSubscription(sub as PushSubscriptionJSON)

      expect(record.id).toBeTruthy()
      expect(record.endpoint).toBe('https://ep1.test')
      expect(record.addedAt).toBeGreaterThan(0)
    })

    it('deduplicate theo endpoint (upsert)', () => {
      const sub = { endpoint: 'https://ep1.test', keys: { auth: 'a', p256dh: 'b' } }
      manager.saveSubscription(sub as PushSubscriptionJSON, { userAgent: 'chrome-v1' })
      manager.saveSubscription(sub as PushSubscriptionJSON, { userAgent: 'chrome-v2' })

      // setWebPushSubscriptions called twice but second upsert replaces the first
      const lastCall = store.setWebPushSubscriptions.mock.lastCall![0]
      const filtered = lastCall.filter((s: WebPushSubscription) => s.endpoint === 'https://ep1.test')
      expect(filtered).toHaveLength(1)
      expect(filtered[0].userAgent).toBe('chrome-v2')
    })
  })

  describe('removeSubscription()', () => {
    it('xóa đúng subscription theo endpoint', () => {
      const initial: WebPushSubscription[] = [
        { id: '1', endpoint: 'https://ep1.test', keys: { auth: 'a', p256dh: 'b' }, addedAt: 1 },
        { id: '2', endpoint: 'https://ep2.test', keys: { auth: 'c', p256dh: 'd' }, addedAt: 2 }
      ]
      store = makeFakeStore({ webPushSubscriptions: initial })
      manager = new WebPushManager(store as never)

      manager.removeSubscription('https://ep1.test')
      const lastCall = store.setWebPushSubscriptions.mock.lastCall![0]
      expect(lastCall).toHaveLength(1)
      expect(lastCall[0].endpoint).toBe('https://ep2.test')
    })
  })

  describe('sendToAll()', () => {
    beforeEach(() => {
      const subs: WebPushSubscription[] = [
        { id: '1', endpoint: 'https://ep1.test', keys: { auth: 'a', p256dh: 'b' }, addedAt: 1 },
        { id: '2', endpoint: 'https://ep2.test', keys: { auth: 'c', p256dh: 'd' }, addedAt: 2 }
      ]
      store = makeFakeStore({ webPushSubscriptions: subs })
      manager = new WebPushManager(store as never)
    })

    it('gửi đến tất cả subscriptions', async () => {
      await manager.sendToAll({ title: 'Test', body: 'body' })
      expect(mockWebPush.sendNotification).toHaveBeenCalledTimes(2)
    })

    it('tự xóa subscription bị 410 Gone', async () => {
      ;(mockWebPush.sendNotification as ReturnType<typeof vi.fn>)
        .mockRejectedValueOnce({ statusCode: 410 })
        .mockResolvedValueOnce({ statusCode: 201 })

      await manager.sendToAll({ title: 'Test', body: 'body' })

      // One subscription should have been removed
      expect(store.setWebPushSubscriptions).toHaveBeenCalled()
      const lastCall = store.setWebPushSubscriptions.mock.lastCall![0]
      expect(lastCall).toHaveLength(1)
    })

    it('tiếp tục gửi subscriptions khác khi 1 lỗi', async () => {
      ;(mockWebPush.sendNotification as ReturnType<typeof vi.fn>)
        .mockRejectedValueOnce(new Error('network error'))
        .mockResolvedValueOnce({ statusCode: 201 })

      // Should not throw
      await expect(manager.sendToAll({ title: 'Test', body: 'body' })).resolves.toBeUndefined()
      expect(mockWebPush.sendNotification).toHaveBeenCalledTimes(2)
    })
  })
})
