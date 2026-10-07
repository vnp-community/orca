# FE-CV-TASK-073-01: Ma trận kiểm thử gating cờ × lối vào (không gọi kênh khi cờ tắt)

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.2
**Priority:** P0
**Area:** frontend / test-support (Vitest, happy-dom)
**File:** `frontend/src/renderer/src/test-support/code-intel-integration/code-intel-flag-gating.integration.test.tsx` (mới)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelSupport`); FE-CV-TASK-085-01 (`useQualityFeatureFlags`); FE-CV-TASK-073-02 (hoặc mock `codeIntelClient.call`); lối vào của FE-CV-SOL-061
**Status:** [x] DONE

## Context

- Hợp đồng §6: nguồn cờ là `settings.get`; `effective.codeIntelEnabled=false` ⇒ không gọi kênh nào khác, không `subscribe`; `qualityGateEnabled=false` ⇒ ẩn phần chất lượng. PQ-01: `PROFILE_UNKNOWN` không ẩn tính năng.
- Mẫu: `test-support/mcp-integration/mcp-pane-states.integration.test.tsx` (đã xác minh tồn tại thư mục `mcp-integration`).
- **Không tạo hook cờ thứ hai.**

## Việc cần làm

1. Ma trận: cờ ∈ {unknown, tắt, code-intel bật/quality tắt, quality bật/AI tắt, tất cả bật} × lối vào {nút hàng agent, Source Control, Cmd+K, tab sidebar, tab `review`, thông báo chất lượng (085)} ⇒ hiển thị/ẩn đúng; spy `codeIntelClient.call`: cờ tắt/unknown chỉ `settings.get`.
2. Ca lỗi: `CODEINTEL_DISABLED` giữa chừng ⇒ ẩn không toast; `CODEINTEL_UNAVAILABLE` ⇒ ẩn; `QUALITY_GATE_DISABLED`/`AI_REVIEW_DISABLED` ⇒ chỉ ẩn phần đó; `PROFILE_UNKNOWN` không ẩn.
3. Ca `settings.get` lỗi mạng: giữ trạng thái trước; chưa từng có ⇒ fail closed.
4. Ca hồi quy cấm `typeof window.api.codeIntel`: giả lập `window.api` là Proxy `withFallback` luôn có hàm ⇒ vẫn ẩn khi `settings.get` báo tắt.

## Kiểm thử

- Chính file này là kiểm thử. Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/test-support/code-intel-integration`.

## Tiêu chí hoàn thành

- [ ] Mọi lối vào có ca ẩn khi cờ tắt.
- [ ] Không có ca nào gọi kênh khác `settings.get` khi cờ tắt.
- [ ] Test xanh.

## Rủi ro

- Phụ thuộc các lối vào đã tồn tại; ca nào chưa có component thì `it.todo` có tên solution chủ.
