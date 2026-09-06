# FE-TASK-007: `useWorkflowExecution` polling interim (CR-PW-006 Phase A)

**Domain:** project-workspace
**Solution Ref:** FE-SOL-005
**Priority:** 🟡 P1
**Estimated:** 40 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch. 5 test case polling mới (dùng `vi.useFakeTimers()`) +
5 test case cũ không đổi 1 dòng nào. `vitest run` trên file: **10/10 pass**.

---

## Mục tiêu

Thay `window.api.on(...)` (Electron-only, silent no-op ở Web mode) bằng polling
`workflow.getExecution` cross-platform, dừng khi execution không còn `running`.

## Files cần sửa

1. `frontend/src/renderer/src/hooks/useWorkflowExecution.ts` (MODIFY — code mẫu đầy đủ ở
   FE-SOL-005)
2. `frontend/src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts` (MODIFY — thêm
   `describe` block mới, không đổi test cũ)

## Test cases cần cover

- Poll gọi `workflow.getExecution` đúng interval khi status = `running` (dùng fake timers, assert
  không gọi trước 4s, gọi đúng 1 lần sau 4s, 2 lần sau 8s).
- Kết quả poll cập nhật store qua `updateExecutionStatus`.
- KHÔNG poll khi execution đã ở status terminal (`completed`).
- Dừng poll sau unmount (advance timer sau unmount, RPC không được gọi thêm).
- RPC reject không throw ra ngoài hook, không crash, không gọi `updateExecutionStatus`.

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts
```

**Kết quả thật:** `Test Files 1 passed (1)`, `Tests 10 passed (10)`.

Sweep rộng hơn để đảm bảo không phá test nào khác đang dùng `useWorkflowExecution`/store workflow
slice:

```bash
cd frontend && npx vitest run src/renderer/src/components/workflow/ src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts src/renderer/src/store/slices/
```

**Kết quả thật:** `Test Files 113 passed (113)`, `Tests 1761 passed (1761)`.

## gitnexus

- `impact({target:"useWorkflowExecution", direction:"upstream", repo:"orca"})`: risk **LOW**,
  impactedCount 1 (`ExecutionMonitor.tsx`), 0 execution flow bị ảnh hưởng.

## Depends on
[BE-SOL-001](../../../../backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md)
(CR-PW-005) — `workflow.getExecution` phải nối dây xong ở Web mode để polling này có tác dụng
thật ngoài Electron mode.

## Blocking
Không có
