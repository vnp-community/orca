# TASK-010: Viết Unit Tests — DevServerStore + DevServerManager

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §10  
**Depends on:** TASK-003, TASK-004  
**Blocks:** (không — verification task)

---

## Mục tiêu

Viết unit tests cho `DevServerStore` và `DevServerManager`, đảm bảo logic CRUD và state management hoạt động đúng.

---

## Files cần tạo

1. `src/main/dev-server/__tests__/dev-server-store.test.ts`
2. `src/main/dev-server/__tests__/dev-server-manager.test.ts`

---

## Test cases cần implement

### `dev-server-store.test.ts`

```typescript
import { DevServerStore } from '../dev-server-store'

describe('DevServerStore', () => {
  let store: DevServerStore
  let mockPersistStore: MockStore  // Dùng mock/stub theo pattern test hiện tại

  beforeEach(() => {
    mockPersistStore = createMockStore({ devServers: [] })
    store = new DevServerStore(mockPersistStore)
  })

  it('list() trả về [] khi chưa có devServers')
  it('list() trả về [] khi state.devServers = undefined')
  it('add() tạo id với prefix "ds-"')
  it('add() set workspaceDir = null và addedAt gần Date.now()')
  it('add() persist vào store')
  it('update() sửa đúng field của đúng record')
  it('update() không ảnh hưởng record khác')
  it('remove() xóa đúng record theo id')
  it('remove() không ảnh hưởng record khác')
})
```

### `dev-server-manager.test.ts`

```typescript
import { DevServerManager } from '../dev-server-manager'

describe('DevServerManager', () => {
  it('add() persists devServer với id và addedAt')
  it('add() set runtime status = "disconnected"')
  it('add() emit "devServer:added" event')
  it('testConnection() relay-ssh success → return { ok: true, platform, nodeVersion }')
  it('testConnection() relay-ssh failure → return { ok: false, error }')
  it('connect() relay-ssh → status: "connecting" → "connected"')
  it('connect() relay-ssh → emit statusChanged "connecting" rồi "connected"')
  it('connect() relay-ssh failure → status: "error" + lastError set')
  it('connect() relay-ssh failure → emit statusChanged "error"')
  it('disconnect() → relay.close() được gọi, status: "disconnected"')
  it('disconnect() → emit statusChanged "disconnected"')
  it('remove() → disconnect() được gọi trước')
  it('remove() → xóa khỏi store')
  it('remove() → emit "devServer:removed"')
  it('getRelay() trả về bridge khi connected')
  it('getRelay() trả về null khi không connected')
  it('list() merge persisted + runtime state')
  it('get() trả về null khi không tìm thấy id')
  it('constructor() restore runtime state với status = "disconnected" cho tất cả persisted servers')
})
```

---

## Acceptance Criteria

- [x] Tất cả test cases được implement (không chỉ `it()` không có body)
- [x] Tests sử dụng mock/stub theo pattern hiện tại trong project (không dùng module mock mới)
- [x] `DevServerRelayBridge` được mock để test không cần SSH thật
- [x] Tất cả tests pass: 28/28 passed
- [x] Không có `console.error` unhandled trong test output

---

## Lưu ý cho AI

1. Tìm hiểu test setup hiện tại: xem `jest.config.*` và các test file mẫu trong `src/main/`
2. Tìm helper `createMockStore` hoặc tương đương trong test utils
3. Mock `DevServerRelayBridge` để isolate Manager tests
4. Mock `SshConnectionManager` để isolate Manager + Bridge tests
