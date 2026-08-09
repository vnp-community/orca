# Tasks — CR series `frontend-e2ee`

**Nguồn:** [solutions/](../solutions/)
**Mục tiêu:** Chia 5 solution thành các tác vụ độc lập, AI có thể thực thi từng cái mà không cần context từ cái khác.
**Trạng thái:** ✅ **Đã thực thi — 2026-08-09** — 11/11 task DONE. Xem `NOTES.md` (nếu cần tạo) hoặc mục "Kết quả thực thi" trong [TASK-FE2E-011](./TASK-FE2E-011-full-test-matrix-and-doc-updates.md) cho tổng kết đầy đủ.

---

## Danh sách Tasks

| ID | Solution | Tiêu đề | File mục tiêu | Phụ thuộc | Kết quả |
|----|----------|---------|----------------|-----------|---------|
| [TASK-FE2E-001](./TASK-FE2E-001-reverify-discovery-audit.md) | SOL-001 | Re-verify audit trước khi implement | — (chỉ verify) | — | ✅ Khớp 100%, không cần cập nhật gì |
| [TASK-FE2E-002](./TASK-FE2E-002-remove-paircodefallback-loginpage.md) | SOL-002 | Bỏ `PairCodeFallback` khỏi `LoginPage.tsx` | `web/login/LoginPage.tsx` | 001 | ✅ DONE, 8/8 test pass |
| [TASK-FE2E-003](./TASK-FE2E-003-update-bootstrap-comment.md) | SOL-002 | Cập nhật comment `installAuthFailedRedirect` | `web/main-web-bootstrap.tsx` | 001 | ✅ DONE |
| [TASK-FE2E-004](./TASK-FE2E-004-loginpage-test.md) | SOL-002 | Test `LoginPage` không còn pairing fallback | `web/login/__tests__/LoginPage.test.tsx` | 002 | ✅ DONE |
| [TASK-FE2E-005](./TASK-FE2E-005-verify-no-dangling-references.md) | SOL-002 | Xác nhận không còn tham chiếu treo, đóng CR-002 | — (verify) | 002, 003, 004 | ✅ DONE + phát hiện/fix 1 comment lỗi thời ngoài kế hoạch |
| [TASK-FE2E-006](./TASK-FE2E-006-create-pair-code-app-entry.md) | SOL-003 | Tạo `pair-code-app-entry.tsx` | `web/pair-code-app-entry.tsx` (mới) | 005 | ✅ DONE |
| [TASK-FE2E-007](./TASK-FE2E-007-main-dynamic-import.md) | SOL-003 | `main.tsx` → dynamic import | `web/main.tsx` | 006 | ✅ DONE, ~110→25 dòng |
| [TASK-FE2E-008](./TASK-FE2E-008-tests-and-bundle-measurement.md) | SOL-003 | Test + đo bundle size | `web/__tests__/main-web.test.ts` (mới) | 007 | ⚠️ DONE — AC-1 chỉ đạt 1 phần (giới hạn đã biết trước, xem file) |
| [TASK-FE2E-009](./TASK-FE2E-009-settings-share-link-comment.md) | SOL-004 | Comment giải thích `canGeneratePairingUrl` | `components/settings/Settings.tsx` | 001 | ✅ DONE |
| [TASK-FE2E-010](./TASK-FE2E-010-settings-share-link-regression-test.md) | SOL-005 | Test bảo vệ share-link ẩn ở web client | `components/settings/RuntimeEnvironmentsPane.share-link.test.tsx` (đổi target so với kế hoạch) | 001 | ✅ DONE, 2/2 test pass |
| [TASK-FE2E-011](./TASK-FE2E-011-full-test-matrix-and-doc-updates.md) | SOL-005 | Test matrix đầy đủ + cập nhật HLD/TDD | `docs/hld/...`, `specs/frontend/tdd/v4/06-web-entry.md` | 002–010 | ✅ DONE — 140/144 test pass toàn bộ series |

---

## Thứ Tự Thực Hiện

```
Sprint 1 — Nền tảng (không phụ thuộc nhau, chạy song song):
  TASK-FE2E-001   re-verify audit
  TASK-FE2E-009   Settings.tsx comment (độc lập với nhánh LoginPage/main.tsx)
  TASK-FE2E-010   Settings.tsx regression test (độc lập)

Sprint 2 — CR-FE2E-002 (bỏ PairCodeFallback):
  TASK-FE2E-002   (sau 001) sửa LoginPage.tsx
  TASK-FE2E-003   (sau 001) sửa comment bootstrap — chạy song song với 002
  TASK-FE2E-004   (sau 002) test LoginPage
  TASK-FE2E-005   (sau 002, 003, 004) verify tổng hợp, đóng CR-002

Sprint 3 — CR-FE2E-003 (code-split), CHỈ bắt đầu sau khi CR-002 đóng để tránh conflict:
  TASK-FE2E-006   (sau 005) tạo pair-code-app-entry.tsx
  TASK-FE2E-007   (sau 006) sửa main.tsx
  TASK-FE2E-008   (sau 007) test + đo bundle

Sprint 4 — Đóng series:
  TASK-FE2E-011   (sau tất cả) test matrix đầy đủ + cập nhật docs
```

**Lưu ý khác với series `hld-v1`:** CR-FE2E-001 (audit) và CR-FE2E-004 (share-link decision) đã có kết luận đầy đủ trong solutions — [TASK-FE2E-001](./TASK-FE2E-001-reverify-discovery-audit.md) chỉ re-verify chứ không lặp lại investigation từ đầu; không có task nào cho CR-FE2E-001/004 riêng lẻ vì bản thân chúng không sinh ra thay đổi code (chỉ TASK-FE2E-009/010 là hệ quả tài liệu hoá/test của kết luận CR-004).

---

## Format Mỗi Task File

Mỗi TASK file có cấu trúc: **Context** (file cần đọc trước) → **Thay đổi cần thực hiện** (diff/code cụ thể, copy-paste ready) → **Verify** (lệnh kiểm tra) → **Definition of Done** (checklist).
