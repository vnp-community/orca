# FE-CV-TASK-050-11: Luồng push `codeIntel.subscribe`, backoff, event bus, polling dự phòng

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.4
**Priority:** P0
**Area:** frontend / store + lib + hooks
**File:** `store/slices/code-intel.ts` (thêm), `store/slices/code-intel-stream-reconnect.ts` (mới), `lib/code-intel-event-bus.ts` (mới), `hooks/useCodeIntelEvents.ts` (mới), `App.tsx` (sửa một dòng), tests
**Depends on:** FE-CV-TASK-050-10, FE-CV-TASK-050-04
**Status:** [x] DONE

## Context

- UI-API §5: ack `null`; mỗi khung có `event`; `changed{resync:true}` rồi gateway đóng, client mở lại backoff 1 s → 30 s và tải lại view; `changed` thường chỉ huỷ cache + chip "Có dữ liệu mới"; polling: `status` 30 s, `reindexStatus` 2 s khi `running`.
- Mẫu: `startMcpEvents` (`mcp-slice.ts:231`), `mcp-event-bus.ts`, `window-visibility-interval.ts`. `subscribe` lần hai thay lần đầu (PQ-11).

## Việc cần làm

1. `startCodeIntelEvents(environmentId)` ref-count; chỉ mở khi `flags.codeIntel`; không gửi `selectors`.
2. `applyCodeIntelEvent`: `changed` (`reason`/`resync`) / `reindexProgress` (`percent:null` giữ null) / `quality.*` (chỉ phát bus).
3. Backoff 1 s → 30 s, khoẻ sau 30 s, tăng `codeIntelResyncCounter` khi nối lại thành công hoặc `resync`.
4. `onUnsupported`/`RATE_LIMITED` ⇒ `polling`; dọn timer khi `reset`/hết ref.
5. Bus: nuốt lỗi từng listener; `useCodeIntelEvents()` gắn ở `App.tsx` (chạy `impact` trước khi sửa).

## Kiểm thử

- Đồng hồ giả: backoff; ref-count một luồng; `changed` không tăng counter; `resync` tăng; polling bật/tắt theo `visibility`; bus cô lập lỗi.

## Tiêu chí hoàn thành

- [ ] Khung lạ bị bỏ; không rò timer sau `reset`; cờ tắt ⇒ không mở luồng.

## Rủi ro

- Giới hạn `CODE_INTEL_MAX_STREAMS` 500/replica: nhiều tab web có thể chạm; chưa đo.
