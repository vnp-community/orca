# BUG-PW-002 [BACKEND]: `ProjectServerRouter.getRelayForProject()` không check project membership — chỉ gọi `assertAccess` nhưng missing user context

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-PW-002  
**Note:** Known design: relay is shared per DevServer; auth at Orca Server level via assertAccess()  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`src/main/project/ProjectServerRouter.ts:32-39`:
```typescript
async getRelayForProject(projectId: string, userId: string): Promise<DevServerRelayBridge> {
  await this.projectService.assertAccess(projectId, userId)
  const project = await this.projectService.get(projectId)
  if (!project) throw new Error('PROJECT_NOT_FOUND')
  const server = this.devServerManager.get(project.devServerId)
  if (!server) throw new Error('DEV_SERVER_NOT_FOUND')
  return this.relayPool.getOrConnect(project.devServerId, server)
}
```

Vấn đề: `assertAccess()` được gọi với `userId` để check quyền. Nhưng sau đó relay bridge được tạo/lấy từ pool **không có userId context**.

`RelayConnectionPool.getOrConnect(devServerId, server)` — relay bridge là **per devServer**, không phải per user.

Nếu:
1. User A và User B đều có project trên `dev-server-01`
2. User A xóa file critical (relay.call('fs.writeFile'))
3. User B dùng cùng relay bridge → không có isolation

**Relay bridge bị share giữa users trên cùng Dev Server** → không có per-user isolation tại relay level.

## Root Cause

`DevServerRelayBridge` là single connection từ Orca Server đến Dev Server. Tất cả users share cùng connection. User context không được gửi xuống relay per-call.

## Thực tế

Đây là architectural decision đã biết: Orca Server trust Dev Server, và authorization là tại Orca Server level. Tuy nhiên, nếu relay commands không carry userId, Dev Server không thể log/audit per-user actions.

## Fix đề xuất

Thêm `userId` vào relay call context:
```typescript
// relay.call với user context
relay.callAs(userId, 'git.exec', { args: [...] })
// → Dev Server nhận { userId, ...params } → log audit
```

Ít nhất cần audit log: Dev Server relay handler nên log userId từ relay call context.

## Files liên quan

- `src/main/project/ProjectServerRouter.ts:32-39`: shared relay bridge
- `src/main/dev-server/relay-connection-pool.ts`: per-server pool
