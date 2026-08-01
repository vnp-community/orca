# BUG-AT-003: Remote Host Automation bị block bởi safety guard — BL-AT-02 chỉ chạy được trên local

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-AT-003  
**Note:** index.ts: allowRemoteHostScheduling:isServeMode — already correctly gated  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`src/main/automations/run-target-resolution.ts:40-50`:
```typescript
const parsedHost = parseExecutionHostId(context.hostId)
if (
  parsedHost?.kind === 'runtime' &&
  (!options.allowRemoteHostScheduling || automation.schedulerOwner !== 'remote_host_service')
) {
  return {
    ok: false,
    error: 'Remote-server automation scheduling is not available from this Orca client yet.'
  }
}
```

Remote host automations bị **block** trừ khi:
1. `options.allowRemoteHostScheduling === true` (cần pass explicitly)
2. `automation.schedulerOwner === 'remote_host_service'`

HLD BL-AT-02 mô tả automation chạy agent trên Dev Server (`relay.call('agent.spawn')`). Nhưng code guard này ngăn remote host automations chạy.

## Kiểm tra AutomationService

`src/main/automations/service.ts:51`:
```typescript
this.allowRemoteHostScheduling = opts.allowRemoteHostScheduling ?? false
```

Default = `false` → remote scheduling bị disabled by default.

## Ảnh hưởng

1. Automation với target = remote Dev Server sẽ fail với error message thay vì execute
2. BL-AT-02 (multi-server workflow via agent spawn) không hoạt động
3. Feature bị gated sau một flag chưa được bật

## Fix đề xuất

Enable remote scheduling khi server có valid relay connections:
```typescript
// server-bootstrap.ts
const automationService = new AutomationService(store, {
  allowRemoteHostScheduling: relayPool.getActiveConnections().length > 0,
  headlessDispatcher: new HeadlessAutomationDispatcher(relayPool)
})
```

## Files liên quan

- `src/main/automations/run-target-resolution.ts:40-50`: guard block
- `src/main/automations/service.ts:51`: default false flag
- `src/main/index.ts`: wiring location
