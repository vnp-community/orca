# BUG-FE-PW-006 — Nút "Connect" (Jira) bị disable khi để trống Email, chặn đúng luồng cần cho self-hosted PAT

## Mức độ: 🔴 HIGH

## Trạng thái: ✅ Fixed & deployed (2026-09-15), có test.

## Tóm tắt

Sau khi fix CR-JIRA-001/BUG-013 (backend hỗ trợ Jira self-hosted qua Bearer PAT khi `Email` rỗng), user report: form Connect Jira **không cho bấm nút "Connect"** khi để trống Email — dù đây chính xác là cách dùng đúng cho site self-hosted.

## Root Cause — CONFIRMED

2 nơi implement cùng 1 form Connect Jira (`jira-connect-dialog.tsx`'s doc comment tự ghi: "mirrors the inline Jira connect dialog in TaskPage"), **cả 2 đều bắt buộc Email non-empty**:

1. `jira-connect-dialog.tsx`:
   ```ts
   const canSubmit =
     Boolean(siteUrl.trim()) &&
     Boolean(email.trim()) &&   // ← chặn email rỗng
     Boolean(apiToken.trim()) &&
     connectState !== 'connecting'
   ```
   và `handleConnect`'s early-return guard lặp lại y hệt.

2. `TaskPage.tsx`'s inline copy: cùng điều kiện trong `handleJiraConnect`, key-down handler, và nút Connect's `disabled` prop.

Cả 2 chỗ được viết TRƯỚC KHI CR-JIRA-001 tồn tại (lúc đó Email luôn bắt buộc vì chỉ hỗ trợ Jira Cloud) — chưa được cập nhật khi thêm hỗ trợ self-hosted.

## Fix

Bỏ điều kiện `Boolean(email.trim())`/`!email` ở cả 4 vị trí (2 file × 2 chỗ check mỗi file: validation submit + disabled prop). Cập nhật label/placeholder ghi rõ "optional — leave blank for a Server/Data Center Personal Access Token".

## Testing

`jira-connect-dialog.test.tsx` (mới) — 3 test, xác nhận:
- Nút Connect **enable** được khi chỉ có Site URL + Token, Email để trống.
- `connectJira` được gọi với `email: ''` (không phải bị chặn từ đầu).
- Vẫn đúng: thiếu Site URL hoặc Token thì nút vẫn disable.

Đã confirm 2/3 test fail trên code trước fix (`git stash` tạm file, chạy lại, restore).

`TaskPage.tsx`'s bản sao: không có test riêng (file quá lớn, chưa có test suite) — sửa tương tự, đã verify bằng `tsc --noEmit` không lỗi type.

## Liên quan

- [BUG-013](../../../backend-go/bugs/missing-v2/BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) / [CR-JIRA-001](../../../../docs/crs/v4/jira-integration/CR-JIRA-001-support-self-hosted-jira-server-data-center.md) — backend fix mà bug UI này chặn không cho dùng tới
