# TASK-016: Viết Unit Tests — `onboarding-ipc` detectAgents

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §8  
**Depends on:** TASK-014  
**Blocks:** (không — verification task)

---

## Mục tiêu

Viết unit tests đầy đủ cho các IPC handlers `onboarding.detectAgents` và `onboarding.detectAgentsAllServers`.

---

## File cần tạo

**Path:** `src/main/ipc/__tests__/onboarding-ipc.test.ts`

---

## Test cases cần implement

```typescript
describe('onboarding.detectAgents', () => {
  it('devServerId = null → trả về { agents: [], platform: null, devServerId: null }')
  it('devServerId của server không tồn tại (relay null) → throw Error')
  it('dev server connected → forward detectAgents đến relay với đúng commands')
  it('relay trả về agents và platform → return đúng format')
  it('cache hit trong 60s → không gọi relay lần 2')
  it('cache miss sau 60s → gọi relay lại')
  it('relay timeout 15s → throw timeout error (error không được cache)')
  it('relay throw error → không lưu vào cache')
})

describe('onboarding.detectAgentsAllServers', () => {
  it('0 connected servers → trả về {} rỗng')
  it('2 connected servers → map { dsId: { agents, platform } }')
  it('1 thành công, 1 lỗi → { ds1: { agents }, ds2: { agents: [], error } }')
  it('chạy song song (Promise.allSettled) — kiểm tra bằng spy/timing')
})
```

---

## Acceptance Criteria

- [x] Tất cả 12 test cases được implement
- [x] `DevServerManager` và `DevServerRelayBridge` được mock
- [x] Timer/cache được test với jest fake timers
- [x] Tất cả tests pass: `npm test -- --testPathPattern=onboarding-ipc`
- [x] No unhandled rejections hoặc console errors

---

## Lưu ý cho AI

1. Dùng `jest.useFakeTimers()` để kiểm soát cache TTL
2. Mock `buildAgentDetectionCommands()` để isolate test
3. Đặt lại cache state trước mỗi test (module re-require hoặc export cache để clear)
