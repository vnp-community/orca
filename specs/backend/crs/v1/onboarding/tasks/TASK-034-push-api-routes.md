# TASK-034: Tạo `src/server/push-api-routes.ts`

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B.5  
**Depends on:** TASK-032  
**Blocks:** TASK-035

---

## Mục tiêu

Tạo HTTP route handlers cho Web Push API: lấy VAPID public key, subscribe, và unsubscribe.

---

## File cần tạo

**Path:** `src/server/push-api-routes.ts`

---

## Nội dung cần implement

```typescript
import type { IncomingMessage, ServerResponse } from 'node:http'
import type { WebPushManager } from '../main/notifications/web-push-manager'

export function registerPushApiRoutes(
  server: import('node:http').Server,
  pushManager: WebPushManager
): void {
  server.on('request', (req: IncomingMessage, res: ServerResponse) => {
    const url = req.url ?? ''

    // GET /api/vapid-public-key
    if (req.method === 'GET' && url === '/api/vapid-public-key') {
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ publicKey: pushManager.getPublicKey() }))
      return
    }

    // POST /api/push-subscribe
    if (req.method === 'POST' && url === '/api/push-subscribe') {
      readBody(req).then(body => {
        const { subscription } = JSON.parse(body)
        const record = pushManager.saveSubscription(subscription, {
          userAgent: req.headers['user-agent']
        })
        res.writeHead(201, { 'Content-Type': 'application/json' })
        res.end(JSON.stringify({ id: record.id }))
      }).catch(() => {
        res.writeHead(400)
        res.end('Invalid body')
      })
      return
    }

    // POST /api/push-unsubscribe
    if (req.method === 'POST' && url === '/api/push-unsubscribe') {
      readBody(req).then(body => {
        const { endpoint } = JSON.parse(body)
        pushManager.removeSubscription(endpoint)
        res.writeHead(204)
        res.end()
      }).catch(() => {
        res.writeHead(400)
        res.end()
      })
      return
    }

    // Unknown route: không xử lý (pass-through cho các middleware khác)
  })
}

async function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', chunk => { data += chunk })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/server/push-api-routes.ts`
- [x] `registerPushApiRoutes()` được export
- [x] `GET /api/vapid-public-key` → `200 { publicKey: string }`
- [x] `POST /api/push-subscribe` → `201 { id: string }`
- [x] `POST /api/push-subscribe` deduplicate theo endpoint (qua `saveSubscription`)
- [x] `POST /api/push-unsubscribe` → `204` không có body
- [x] Routes không match → không xử lý (không gửi response), để route tiếp theo handle
- [x] Invalid JSON body → `400`
- [x] TypeScript compile thành công
