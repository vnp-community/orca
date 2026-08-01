# BUG-WF-001 [BACKEND]: `StepExecutors.getRelay()` — `server:<devServerId>` spec type ném error `SERVER_SPEC_NOT_SUPPORTED`

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WF-001,002  
**Note:** WorkflowOrchestrator.ts complete implementation  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/main/workflow/StepExecutors.ts:197-204`:
```typescript
if (specType === 'server') {
  // Direct server access — use internal router method if available
  // Fallback: create a synthetic project access via projectService.get
  throw new Error(`SERVER_SPEC_NOT_SUPPORTED: direct server specs require router.getRelayForServer (not yet implemented)`)
}
```

**`server:<devServerId>` target type trong workflow templates luôn ném error.**

Flow BL-WF-02 mô tả:
```
"server:dev-02"    → direct mapping
"fleet:tag:backend" → all servers với tag='backend'
```

Cả hai đều không được implement:
- `server:` → throws NotImplemented
- `fleet:tag:` → không có case xử lý (fall through to `UNKNOWN_SERVER_SPEC_TYPE`)

## Root Cause

`ProjectServerRouter` chỉ có `getRelayForProject(projectId, userId)` — không có `getRelayForServer(devServerId)`.

Workflow steps với `serverSpec: 'server:dev-02'` hoặc `serverSpec: 'fleet:tag:backend'` sẽ **fail mọi lúc**.

## Ảnh hưởng

1. Multi-server workflow execution chỉ hoạt động với `project:<projectId>` spec
2. Tất cả `server:` và `fleet:tag:` specs ném error → step fails
3. `fleet:tag:` không có trong dispatch → UNKNOWN_SERVER_SPEC_TYPE error
4. BL-WF-02 "fleet:tag:backend" use case hoàn toàn không hoạt động

## Fix đề xuất

Thêm `getRelayForServer` vào `ProjectServerRouter`:
```typescript
async getRelayForServer(devServerId: string): Promise<DevServerRelayBridge> {
  const server = this.devServerManager.get(devServerId)
  if (!server) throw new Error(`DEV_SERVER_NOT_FOUND: ${devServerId}`)
  return this.relayPool.getOrConnect(devServerId, server)
}
```

Thêm fleet tag resolution:
```typescript
// StepExecutors.getRelay():
if (specType === 'fleet') {
  const [, tagKey, tagValue] = step.serverSpec.split(':')
  const servers = await this.router.getServersByTag(tagKey, tagValue)
  // Return relay for first available server
  return this.router.getRelayForServer(servers[0]?.id)
}
```

## Files liên quan

- `src/main/workflow/StepExecutors.ts:197-204`: throws not implemented
- `src/main/project/ProjectServerRouter.ts`: thiếu getRelayForServer
