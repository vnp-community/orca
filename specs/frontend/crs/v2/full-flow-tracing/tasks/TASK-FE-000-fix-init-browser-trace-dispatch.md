# TASK-FE-000: Fix no-op `initBrowserTrace()` dispatch (uncomment `addTraceEvent()`)

**Phase:** 0
**SOL Ref:** [solutions/00-index.md §"Blocker chung"](../solutions/00-index.md)
**CR Ref:** [CR-TRACE-000](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-000-tracing-rollout-overview.md)
**Prerequisite:** Không có (task nền tảng đầu tiên, chặn TOÀN BỘ 10 CR frontend)
**Status:** ✅ Done (2026-08-03) — Uncommented `useAppStore.getState().addTraceEvent(event)` in the `initBrowserTrace` dispatch callback in `main-web-bootstrap.tsx`; `useAppStore` import and `addTraceEvent` action already existed, no other changes needed; `pnpm tsc --noEmit` clean, no dedicated test file exists for this bootstrap module.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "initBrowserTrace"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "initBrowserTrace", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

`src/shared/trace/00-index.md` của solutions ghi nhận: dù 10 solution frontend có instrument đầy đủ RPC call site, TracePanel vẫn sẽ **không hiển thị gì** cho tới khi bug này được sửa. Đã Read trực tiếp file `src/renderer/src/web/main-web-bootstrap.tsx` (2026-08-02) để xác nhận lại nội dung/số dòng hiện tại (không dựa vào solution doc, vì solution doc trích dẫn dòng 294-296 nhưng có thể lệch sau các commit gần đây).

Xác nhận qua `grep -n` trực tiếp: dòng 294 gọi `initBrowserTrace((event) => {`, dòng 295 là `// useAppStore.getState().addTraceEvent(event)` — **đang bị comment out**. Đây chính là điểm dispatch callback của SSE client (`startSseClient()` trong `src/shared/trace/browser.ts`) — nếu callback này là no-op, mọi `TraceEvent` (kể cả event phát từ 10 CR sẽ được instrument sau task này) chỉ trôi qua và bị bỏ, không bao giờ tới Zustand store `trace.ts`, do đó không bao giờ hiển thị trên `TracePanel.tsx`.

Đây là bug fix đơn giản (uncomment 1 dòng), KHÔNG phải instrumentation mới — nhưng là precondition cứng, phải làm TRƯỚC Phase 1-3.

## File: `src/renderer/src/web/main-web-bootstrap.tsx` [MODIFY]

Đọc lại context quanh dòng 294-296 trước khi sửa (dùng Read, không dùng giả định từ solution doc vì số dòng có thể đã lệch). Nội dung hiện tại xác nhận qua grep:

```typescript
  initBrowserTrace((event) => {
    // useAppStore.getState().addTraceEvent(event)
```

Sửa thành (uncomment, giữ nguyên phần còn lại của callback không đổi):

```typescript
  initBrowserTrace((event) => {
    useAppStore.getState().addTraceEvent(event)
```

**Yêu cầu bắt buộc trước khi sửa:**
1. Xác nhận `useAppStore` đã có action `addTraceEvent` trong store slice `trace.ts` (`src/renderer/src/store/slices/trace.ts`) — nếu action này chưa tồn tại hoặc có tên khác, KHÔNG tự đổi tên action, chỉ báo cáo mismatch và dừng lại (ngoài phạm vi task này để tự bịa 1 action mới).
2. Xác nhận `useAppStore` đã được import trong `main-web-bootstrap.tsx` ở đầu file — nếu chưa, thêm import (additive).
3. Đây là thay đổi **additive/bugfix duy nhất** — không sửa logic nào khác trong `initBrowserTrace()` hay phần còn lại của file.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/web/__tests__/main-web-bootstrap.test.ts
```

Verify thủ công (khuyến nghị, không bắt buộc để merge task): chạy app ở web mode, mở TracePanel, thực hiện 1 hành động có RPC (vd. mở Settings), xác nhận có ít nhất 1 `TraceEvent` xuất hiện trong panel — trước khi fix, panel phải trống dù backend có phát event qua SSE.

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Dòng `// useAppStore.getState().addTraceEvent(event)` trong `main-web-bootstrap.tsx` được uncomment thành `useAppStore.getState().addTraceEvent(event)`
- [ ] Không có thay đổi nào khác trong file ngoài dòng này (và import `useAppStore` nếu thiếu)
- [ ] `pnpm tsc --noEmit` pass — xác nhận `addTraceEvent` tồn tại đúng chữ ký trên store type
- [ ] Test suite hiện có của `main-web-bootstrap.tsx` (nếu có) không bị break
- [ ] Ghi trong PR/commit rằng đây là bug fix precondition, không phải instrumentation mới — tham chiếu tới finding "Blocker chung" trong `solutions/00-index.md`
- [ ] Task này PHẢI merge/hoàn thành trước khi bất kỳ task Phase 1/2/3 nào được coi là "verify được bằng mắt qua TracePanel" (các task khác vẫn có thể code song song, nhưng không thể end-to-end verify tới khi task này xong)
