# BUG-BE-HLD-016 — Migration 0004 và 0007 đụng độ tên bảng `orca_projects`, buộc phải đổi tên `orca_v5_projects` — nợ kỹ thuật + tài liệu sai theo

**Mức độ:** 🟡 MEDIUM (Tech debt + Documentation drift)
**Status:** 🔴 Open
**Module:** `backend/src/main/db/migrations/0004_orca_app_tables.ts`, `0007_projects.ts`
**Phát hiện:** 2026-08-08/09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §2.7, §0)

---

## Mô tả

`docs/hld/backend-server-architecture.md §10` và `docs/adrs/v2/ADR-016` (proposal) đều mô tả migration 0007 tạo bảng `orca_projects`/`orca_project_members`. Nhưng migration 0004 (`orca_app_tables`) **đã chiếm tên `orca_projects`** từ trước (một bảng project ở thế hệ schema cũ hơn, dùng cho single-user/desktop mode).

Khi migration 0007 (Project↔DevServer binding, F34) được viết sau đó, đội ngũ phải đổi tên thành `orca_v5_projects`/`orca_v5_project_members` để tránh đụng độ — comment trong `0007_projects.ts:5-9` giải thích rõ lý do. Không tài liệu nào (`backend-server-architecture.md`, ADR-016, F34) được cập nhật để phản ánh tên bảng thật này.

## Hậu quả

- Bất kỳ ai đọc tài liệu rồi viết SQL query trực tiếp (debug, migration script, backup/restore) sẽ query nhầm bảng `orca_projects` (bảng cũ, khác schema) thay vì `orca_v5_projects` (bảng thật của F34/Project-DevServer binding).
- Tên bảng có tiền tố `v5` gây khó hiểu về mặt versioning lâu dài (không có `orca_v6_projects` tương ứng, dễ nhầm là "phiên bản 5" của hệ thống thay vì chỉ là disambiguation).
- Là dấu hiệu nợ kỹ thuật: 2 bảng "project" cùng tồn tại trong 1 DB, không rõ ràng bảng nào là nguồn sự thật cho tính năng nào.

## Bằng chứng

- `backend/src/main/db/migrations/0004_orca_app_tables.ts:20-61` — tạo `orca_projects` (schema cũ).
- `backend/src/main/db/migrations/0007_projects.ts:5-9,23,43` — comment giải thích đổi tên `orca_v5_projects` để tránh đụng độ.
- `backend/src/main/project/ProjectService.ts:102-118,176,219-224` — dùng đúng `orca_v5_projects`/`orca_v5_project_members` trong SQL.
- `docs/hld/backend-server-architecture.md §10`, `docs/adrs/v2/ADR-016` — cả 2 đều ghi `orca_projects`/`orca_project_members` không có tiền tố.

## Đề xuất fix

1. **Ngắn hạn (rẻ, không đụng code):** cập nhật `docs/hld/backend-server-architecture.md §10` và toàn bộ tài liệu liên quan để ghi đúng tên bảng thật `orca_v5_projects`/`orca_v5_project_members`.
2. **Dài hạn (cần kế hoạch migration riêng):** làm rõ vai trò của bảng `orca_projects` (0004) — nếu không còn dùng (dead table), viết migration dọn dẹp/rename để bỏ tiền tố `v5` gây hiểu nhầm; nếu vẫn dùng cho mục đích khác, tài liệu hoá rõ ràng ranh giới giữa 2 bảng.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §2.7, §0
- Doc gốc: `docs/hld/backend-server-architecture.md §10`, `docs/adrs/v2/ADR-016-db-migrations-0006-0010-schema.md`
