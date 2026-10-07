# FE-CV-TASK-073-06: Thẻ cài đặt admin bật/tắt code-intel và cổng chất lượng

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.3
**Priority:** P1
**Area:** frontend / renderer components + i18n
**File:** `frontend/src/renderer/src/components/settings/code-intel/CodeIntelSettingsCard.tsx` (mới) + test; `components/settings/Settings.tsx` hoặc registry pane (sửa nhỏ, vị trí chưa kiểm chứng); `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (thêm `KEYS`)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks (`settings.get`, làm mới); FE-CV-TASK-073-02
**Status:** [x] DONE

## Context

- Mẫu: `components/settings/mcp/McpTenantSettingsForm.tsx`, `McpPane.tsx`; admin: `useAppStore((s) => s.currentUser?.role === 'admin')` (đã xác minh ở `Settings.tsx:286`). **Chạy `gitnexus_impact` trên `Settings` trước khi sửa.**
- `settings.get` không trả vai trò ⇒ UI theo `currentUser`; service quyết định cuối (`admin`).

## Việc cần làm

1. Hiển thị `effective` (4 cờ) và `tenant`; chênh lệch ⇒ "công tắc của máy chủ đang tắt" (không đổ lỗi người dùng).
2. Hai công tắc `codeIntelEnabled`, `qualityGateEnabled` (admin); `settings.set` chỉ gửi trường thay đổi (≥ 1); khoá nút ngay, hiển thị trạng thái sau ~200 ms; lỗi `CODEINTEL_NOT_AUTHORIZED` inline; thành công ⇒ làm mới `settings.get`.
3. Người thường: thẻ chỉ đọc, không nút. Cờ tổng tắt ⇒ `qualityGateEnabled` vô hiệu có lý do.
4. Điểm mở rộng cho cờ khác (AI, quét bảo mật, `indexPolicy`…) thuộc solution của CR sở hữu; không thêm ở đây.
5. i18n 5 locale; token CSS; không hex.

## Kiểm thử

- Testing Library: admin vs người thường; chỉ gửi trường đổi; lỗi quyền; chênh lệch; khoá nút. 
- `pnpm --filter orca-frontend test -- src/renderer/src/components/settings/code-intel src/renderer/src/i18n`.

## Tiêu chí hoàn thành

- [ ] Khớp tiêu chí thẻ admin ở SOL-073 mục 5.
- [ ] `no-top-level-translate.test.ts` xanh.

## Rủi ro

- Vị trí pane chưa xác định (câu hỏi mở 2); tắt cờ chặn cả đường quay lại nếu `settings.set` bị chặn (service vẫn cho `settings.set` khi tắt, PQ-24).
