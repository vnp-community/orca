# BE-CV-TASK-031-08: Quan hệ ERD (fk, logical: declared/comment/naming), cardinality, `tenant_scoped`, `DIALECT_DRIFT`

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/erd/erd_model.go` (mới), `.../domain/erd/relation_inference.go` (mới), `.../domain/erd/dialect_drift.go` (mới), `.../adapter/erdlinks/declared_links.go` (mới); sửa `.../adapter/sqlmigration/statement_splitter.go` (giữ chú thích cùng dòng); các `_test.go`
**Depends on:** BE-CV-TASK-031-06 (Catalog hoàn chỉnh), BE-CV-TASK-030-05
**Status:** [ ] TODO

---

## Context

SOL-031-erd mục 2.C. Nguồn E7 rẻ nhất là chú thích `logical FK` (16 file Postgres + 11 MySQL) với nhiều biến thể (`-> tenant-service.users; nullable`, `to mcp.grants (no cross-database FK)`, `-> task-service.tasks.id (different id space…)`). Tokenizer hiện bỏ comment trước khi tách; cần giữ chú thích gắn với dòng.

## Việc cần làm

1. Sửa splitter/tokenizer: `Statement.CommentsByLine map[int]string` (hoặc `Column.TrailingComment` do phân tích cột nhận từ comment cùng dòng cuối định nghĩa cột); không làm đổi hành vi tách (test BE-CV-TASK-031-02 vẫn xanh).
2. `erd_model.go`: dựng `ErdModel` miền từ `Catalog` (+ `externalRefs`, `changes` chỗ trống cho task 10); `tenant_scoped` = bảng có cột `tenant_id`; không tạo cạnh `tenant_id`.
3. `relation_inference.go`:
   - `fk` từ `ForeignKey` của Catalog (`source:"ddl"`, conf 1.0, `on_delete`, `on_update`);
   - `logical` từ chú thích: mẫu `logical FK\s*(->|to|into|back to)\s*([a-z_-]+)(\.([a-z_]+))?(\.([a-z_]+))?`; khử hậu tố sau `;`/`(`; chỉ tên service → cột đích `id`, bảng đích suy từ tên cột (`worktree_id`→`worktrees`) chỉ khi duy nhất trong ERD service đích (cần danh sách bảng của service đích: tham số `peerCatalogs`), ngược lại `to.table=""` + note "chưa xác định đích"; `source:"comment"`, conf 0.8;
   - `declared` từ `declared_links.go` (`docs/code-intel/erd-links.yaml` qua `RepoSourceReader`; `yaml.v3` `KnownFields(true)`, từ chối `AliasNode`, ≤ 64 KiB, ≤ 500 mục; thiếu tệp = không lỗi); `source:"declared"`, conf 1.0;
   - `naming` (`*_id` ↔ tên bảng số ít duy nhất của service khác) chỉ khi `includeInferred`; conf 0.4.
   - Khi cùng cặp có nhiều nguồn: giữ nguồn conf cao nhất, thêm `note` liệt kê nguồn còn lại.
4. `cardinality`: `one-to-one` nếu cột nguồn thuộc PK/UNIQUE đơn, còn lại `many-to-one`.
5. `dialect_drift.go`: so tập bảng và tập cột của hai Catalog cùng service → `ParseWarning{code:"DIALECT_DRIFT", message}` (một cho mỗi bảng/cột lệch, tối đa 200 + 1 cảnh báo tổng nếu cắt).
6. Ngân sách: ≤ 300 bảng, ≤ 2 000 cột, cắt theo thứ tự tên + `truncated=true`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/erd/... ./services/code-intel-service/internal/adapter/erdlinks/...` (chưa chạy).
- Bảng ca chú thích (đủ biến thể của BE-CV-TASK-031-01 INVENTORY), cả Postgres và MySQL; ca đích mơ hồ; `declared` ghi đè `comment` cùng cặp.
- YAML: có alias → từ chối; trường lạ → từ chối; 600 mục → vượt giới hạn.
- `DIALECT_DRIFT`: `mcp-service` (chỉ Postgres) và một ca tổng hợp có cột chỉ ở MySQL.
- Test hai dialect: suy quan hệ chạy trên Catalog của cả hai.

## Tiêu chí hoàn thành

- [ ] Mỗi chú thích `logical FK` trong corpus sinh quan hệ `source=comment` hoặc cảnh báo "đích chưa xác định".
- [ ] `naming` không xuất hiện khi `includeInferred=false`.
- [ ] YAML độc hại bị từ chối.

## Rủi ro và lưu ý

- Mức heuristic tên cột chưa thử; để tắt mặc định.
- `peerCatalogs` đòi dựng Catalog của service đích → tăng chi phí đọc; chỉ dựng khi có chú thích trỏ tới service đó (lazy, dùng cache).
