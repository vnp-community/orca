# TASK-E2E-012: Migration vá quyền `owner` cho task đã tạo trước bản sửa

**From Solution:** SOL-E2E-002 · **Resolves:** BUG-E2E-006
**Priority:** P1 · **Service/Area:** task-service
**Files:**
- backend-go/services/task-service/migrations/postgres/NNNN_backfill_owner_grants.{up,down}.sql (mới)
- backend-go/services/task-service/migrations/mysql/NNNN_backfill_owner_grants.{up,down}.sql (mới)
- docs/guides/ (runbook ngắn)
**Depends on:** TASK-E2E-011
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Task tạo qua gateway trước bản sửa không có grant `owner`; sau bản sửa cây con của chúng vẫn rỗng.

## Việc cần làm

- [-] Xác định nguồn chủ sở hữu: `tasks.owner_id` (có trong `Task`) hoặc người tạo nếu có cột. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_
- [-] `INSERT … SELECT` grant `owner` cho task chưa có, `ON CONFLICT DO NOTHING` (MySQL: `INSERT IGNORE`/`NOT EXISTS`) để idempotent; phân lô. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_
- [-] Runbook: đếm số task bị ảnh hưởng trước, đếm lại sau. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_
- [-] `down`: không xoá grant (không phân biệt được grant do người dùng cấp); ghi rõ trong file. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_

## Tiêu chí hoàn thành

- [-] Task tạo trước migration có `getSubtree` không rỗng sau migration. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_
- [-] Chạy hai lần cho cùng kết quả. — _không cần: thay bằng sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm_

## Kiểm thử

Test migration hai dialect với dữ liệu mẫu.

## Kết quả thực thi

Không cần: bị thay thế bởi sửa `GetSubtree` coi `OwnerID` là chủ sở hữu ngầm (test có). Không viết migration backfill.
