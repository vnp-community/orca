# FE-CV-TASK-073-04: Spec `flag.web.e2e.ts` (cờ tắt/bật, ba mức, tắt giữa chừng)

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.2, 2.5
**Priority:** P0
**Area:** tests / Playwright web
**File:** `tests/e2e/code-intel-web/flag.web.e2e.ts` (mới)
**Depends on:** FE-CV-TASK-073-02, 073-03; FE-CV-SOL-061 (lối vào); FE-CV-TASK-073-06 (ca admin)
**Status:** [ ] TODO

## Context

- Mẫu `tests/e2e/mcp-web/rollout.web.e2e.ts` (MCP disabled: không mục MCP + `streamCount()==0`; kill switch trực tiếp).
- Hợp đồng: không có push cờ; tắt giữa chừng qua refresh `settings.get` mỗi 60 s (khi tab Review mở) hoặc `CODEINTEL_DISABLED` ở lần gọi kế.

## Việc cần làm

1. Cờ tắt: không lối vào ở Source Control/Cmd+K/right sidebar/hàng agent; không `pageerror` liên quan; `streamCount()==0`; `calls` chỉ `settings.get`.
2. Cờ bật: có lối vào; `subscribe` đúng một lần.
3. Ba mức: `qualityGateEnabled=false` ẩn phần chất lượng nhưng giữ Review; `aiReviewEnabled=false` chỉ ẩn AI; `PROFILE_UNKNOWN` không ẩn gì.
4. Tắt giữa chừng: (a) `page.clock.install()` + `fastForward(60_000)` sau `setSettings({codeIntelEnabled:false})` ⇒ lối vào/tab biến mất; (b) `failNext('codeIntel.status','CODEINTEL_DISABLED: …')` ⇒ ẩn không toast.
5. `CODEINTEL_UNAVAILABLE` ⇒ ẩn (unsupported), không lỗi đỏ.
6. Admin vs người thường cho thẻ cài đặt (khi 073-06 xong).

## Kiểm thử

- Chính spec; chạy `pnpm run test:e2e:code-intel-web -- flag.web.e2e.ts` (**chưa chạy**).

## Tiêu chí hoàn thành

- [ ] Các ca 1–5 xanh trên fake backend.
- [ ] Khẳng định trên DOM, không trên store.

## Rủi ro

- `page.clock` tương tác với timer ứng dụng chưa thử.
