# TASK-BE-DB-021: MySQL/TiDB rollout cho `project-service`

**Solution:** [BE-DB-SOL-016](../solutions/BE-DB-SOL-016-project-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `project-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại. Tham khảo BE-DB-SOL-005/TASK-BE-DB-010 (annotation-service, RowsAffected pitfall) và BE-DB-SOL-006/TASK-BE-DB-011 (credential-broker-service, RLS-drop + UUID translation) làm ví dụ đã hoàn thành.
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy trước khi sửa mọi symbol hiện có** (bắt buộc theo
> CLAUDE.md/AGENTS.md, repo `orca`) — `run`, `Load`, và cả 8 interface
> DB-backed trong `ports.go` (`ProjectRepository`, `RepoRepository`,
> `WorktreeRepository`, `ProjectGroupRepository`, `SparsePresetRepository`,
> `SourceProjectRepository`, `HostSetupRepository`,
> `FolderWorkspaceRepository`) — **tất cả LOW risk, impactedCount 1-3,
> không HIGH/CRITICAL nào**. Xem BE-DB-SOL-016 §2 cho bảng đầy đủ. An toàn
> tiến hành không cần cảnh báo thêm.
>
> **2. Audit thật xác nhận đúng số liệu ROLLOUT-TRACKING.md** (9 repository
> — 8 interface DB-backed + `OutboxRepository`/`common/outbox.Store` không
> có interface riêng trong `ports.go`; 48 file migration = 24 cặp up/down,
> đánh số `0001-0008` + `0015-0030`, khoảng trống `0009-0014` không tồn
> tại từ trước — không phải thiếu file do rollout này). Chi tiết đầy đủ ở
> BE-DB-SOL-016 §1: 11/11 bảng CÓ RLS (chưa từng thật enforce, đúng phát
> hiện BE-DB-SOL-001 §4), 2 cột JSONB, 1 cột `TEXT[]` (Postgres-only array,
> không tương đương MySQL trực tiếp), nhiều điểm `RETURNING`, 1 multi-table
> `UPDATE ... FROM`, 4 partial index, 2 cột `TEXT` tham gia UNIQUE
> constraint (gap thật, chưa gặp ở service nào trước trong rollout).
>
> **3. Migration tách `migrations/postgres/` (`git mv`, 48 file, nội dung y
> hệt) + `migrations/mysql/` (mới, 48 file, dialect-safe)** — điểm dịch
> không tầm thường liệt kê đầy đủ ở BE-DB-SOL-016 §1/§5: RLS bỏ trên cả 11
> bảng (comment giải thích), `JSONB`→`JSON`, `TEXT[]`→`JSON` (mảng, dịch ở
> tầng Go), `UPDATE ... FROM`→`UPDATE ... JOIN`, 4 partial index → plain
> UNIQUE index (2 chỗ, ngữ nghĩa NULL-distinct của MySQL giữ nguyên ý định)
> hoặc full index (2 chỗ), `folder_workspaces.path`→`VARCHAR(1024)` và
> `worktrees.idempotency_key`→`VARCHAR(255)` (nới rộng có chủ đích để
> UNIQUE index hoạt động trên MySQL/InnoDB, ghi rõ trong migration
> comment), `0022`'s backfill thêm `WHERE created_by IS NOT NULL` tường
> minh (cải thiện nhỏ so với bản gốc, an toàn hơn trên cả 2 ý định).
>
> **4. `internal/adapter/mysql/` mới — 9 file, implement đúng cả 9
> repository**, không đổi signature nào. Phát hiện quan trọng nhất (áp
> dụng lại BE-DB-SOL-005 §3.1's RowsAffected()-counts-CHANGED-not-MATCHED
> pitfall trên diện rộng — 6/9 repository có method UPDATE-without-DELETE):
> không method UPDATE nào trong 9 file dựa vào `RowsAffected()` để quyết
> định not-found — hoặc check tồn tại riêng bằng `SELECT EXISTS(...)`
> trước, hoặc UPDATE-rồi-SELECT-lại. Test trực tiếp:
> `TestRepository_UpdateProject_NoopPatchStillSucceeds`,
> `TestRepository_UpdateDevServerID_NoopStillSucceeds`, no-op-rename case
> trong `TestProjectGroupRepository_CreateGetUpdateDelete_RoundTrip`. Mọi
> `DELETE` vẫn dùng `RowsAffected()` trực tiếp — không ambiguity ở dialect
> nào (đúng BE-DB-SOL-005 §4's phát hiện, xem BE-DB-SOL-016 §3.1).
>
> **5. Điểm dịch riêng của service này, chưa gặp ở service nào trước trong
> rollout** — chi tiết đầy đủ BE-DB-SOL-016 §3.2-§3.6:
> - `sparse_presets.directories` (Postgres `TEXT[]`) → cột `JSON` +
>   `json.Marshal`/`Unmarshal` ở tầng Go, domain type `[]string` không đổi.
> - `worktrees.metadata`'s merge-patch: Postgres jsonb `||` →
>   `JSON_MERGE_PATCH` (RFC 7396) — khác biệt hành vi thật với `null` tường
>   minh (xoá key thay vì lưu null), coi là tương đương chức năng cho mọi
>   caller hiện có, ghi rõ trong code comment.
> - `RepoRepository.ReassignProject`'s vị trí subquery trên chính bảng
>   đang UPDATE — MySQL cấm trực tiếp (`ERROR 1093`), khắc phục bằng
>   derived-table wrapper.
> - `FolderWorkspaceRepository.Create`'s lỗi UNIQUE/FK map sang mã lỗi
>   MySQL `1062`/`1452` (thay SQLSTATE `23505`/`23503` của Postgres).
> - `ProjectGroupRepository.ImportNested`'s 3 điểm `gen_random_uuid()` dịch
>   sang `uuid.NewString()` sinh ở Go (nhất quán với mọi ID khác của
>   service).
>
> **Bug tiền tồn tại phát hiện + XÁC NHẬN THẬT bằng test chạy trên
> Postgres** (không phải suy đoán từ đọc code — xem mục 8 dưới): bản
> Postgres gốc's `ImportNested` (`internal/adapter/postgres/
> project_group_repository.go`) scan 1 `INSERT ... RETURNING` 12 cột vào
> chỉ 10 đích — pgx lỗi thật "number of field descriptions must equal
> number of destinations, got 12 and 10" mỗi lần `ImportNested` chạy trên
> Postgres. **Flag, không fix** (ngoài phạm vi rollout multi-database,
> cùng posture "flag don't fix" như TASK-BE-DB-009's UUID-column finding).
> Bản MySQL viết mới KHÔNG tái tạo lỗi này.
>
> **6. Wiring `cmd/server/main.go`** — 9 biến port khai báo kiểu interface
> (`usecase.ProjectRepository`, ...) trong `switch caps.Dialect`, thay vì
> struct cụ thể — khác `usage-service`'s 2-biến vì service này có 9 port
> DB-backed. `internal/config/config.go`'s `DatabaseCredentialsFile` +
> Vault wiring **đã có sẵn từ trước** (khác annotation-service/
> credential-broker-service, vốn phải thêm mới) — "Project Workspace
> unification" work trước đó đã đóng gap này, xác nhận tươi qua Read trực
> tiếp, không giả định. `toMySQLDriverDSN` copy nguyên vẹn từ
> `usage-service` (thuần DSN-plumbing).
>
> **7. `.github/workflows/backend-go-project-service.yml` mới** — copy
> khung `backend-go-usage-service.yml`, đổi path/service name. YAML hợp lệ
> xác nhận qua `python3 -c "import yaml; yaml.safe_load(...)"` — PASS.
>
> **8. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS (phiên xác minh 2026-09-11, chạy lại từ đầu bởi
> agent verification, không kế thừa số liệu cũ):**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ go vet -tags=integration ./...                     # sạch, không lỗi
> $ gofmt -l .                                         # rỗng
> $ go test ./...                                      # PASS (domain, usecase — không đổi)
> ```
>
> `go test -tags=integration ./internal/adapter/postgres/... -v -timeout=20m`:
> **26 PASS / 14 FAIL / 40 test** (455s). **Cả 14 FAIL đều là bug tiền tồn
> tại KHÔNG liên quan tới rollout này** — xác nhận qua `git diff` (không
> file nào trong 2 nhóm dưới bị sửa bởi rollout, chỉ có
> `repository_test.go`'s đổi path migration):
> - **1 FAIL** (`TestProjectGroupRepository_ImportNested_StampsDevServerIDOnRepoToo`):
>   đúng bug đã flag ở mục 5/BE-DB-SOL-016 §1 — `ImportNested`'s
>   `INSERT ... RETURNING` 12 cột / `Scan` 10 đích. Lỗi thật quan sát được:
>   `insert imported project: number of field descriptions must equal
>   number of destinations, got 12 and 10`. Không fix (ngoài phạm vi, đã
>   ghi nhận từ trước).
> - **13 FAIL** (mọi test dùng `RecordWorktreeCreated`/`CreateWorktreeWithEvent`
>   trong `worktree_repository_test.go` — trực tiếp hoặc qua setup helper
>   cascade): **bug tiền tồn tại MỚI PHÁT HIỆN trong phiên xác minh này**,
>   KHÔNG do rollout — helper `newTestOutboxEvent()` (file test, không đổi
>   bởi rollout) không set `TenantID`, trong khi
>   `internal/usecase/record_worktree_created.go` (không đổi bởi rollout)
>   set đúng `event.TenantID = tenantID` từ `tenant.RequireTenantID(ctx)` ở
>   production code — chỉ riêng test helper gọi thẳng
>   `WorktreeRepository.RecordWorktreeCreated` bỏ qua usecase layer nên
>   thiếu field này. Lỗi thật quan sát được:
>   `insert outbox event: ERROR: invalid input syntax for type uuid: ""
>   (SQLSTATE 22P02)` (cột `outbox_events.tenant_id` kiểu `uuid`, nhận
>   chuỗi rỗng). **Flag, không fix** — cùng posture với `ImportNested`, cả 2
>   file liên quan (`worktree_repository_test.go`,
>   `record_worktree_created.go`) không nằm trong "Files đã sửa/thêm" của
>   task này, tiền tồn tại độc lập với multi-dialect rollout, ngoài phạm vi
>   CR-DB-002/003.
>
> `go test -tags=integration ./internal/adapter/mysql/... -v -timeout=25m`:
> **27 PASS / 0 FAIL / 27 test** (978s) — **100% PASS**, không lỗi nào,
> bao gồm mọi test đặc thù rollout này liệt kê ở mục 3/5 (no-op UPDATE,
> JSON array round-trip, `JSON_MERGE_PATCH`, derived-table workaround,
> mã lỗi 1062/1452, `ImportNested` bản MySQL — không tái tạo bug Postgres).
> ("unexpected EOF" trong log là noise từ driver khi container dừng sau
> khi test đã PASS, không phải lỗi test.)
>
> **9. `.github/workflows/backend-go-project-service.yml`**: YAML hợp lệ
> xác nhận lại qua `python3 -c "import yaml; yaml.safe_load(...)"` — PASS.
> Không thể verify "Actions tab chạy thật" (không có quyền mở PR/push
> trong phiên này).
>
> **Final status: ✅ DONE.** Toàn bộ scope của rollout này (migration
> `mysql/`, 9 adapter MySQL, wiring `cmd/server/main.go`, workflow CI) xác
> minh PASS thật 100% (build/vet/gofmt sạch, MySQL integration 27/27). 14
> FAIL trên Postgres đều là bug tiền tồn tại xác nhận KHÔNG do rollout này
> gây ra (file liên quan không nằm trong "Files đã sửa/thêm", xem mục 8) —
> 1 đã biết từ trước khi bắt đầu (`ImportNested`), 13 mới phát hiện trong
> phiên xác minh này (`newTestOutboxEvent` thiếu `TenantID`) — cả 2 flag
> không fix, đúng posture "flag don't fix" nhất quán với toàn bộ rollout.
> Không còn placeholder/số liệu chưa điền nào trong tài liệu này.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `project-service` — service lớn thứ 2 trong 15-service
rollout còn lại (xem `ROLLOUT-TRACKING.md`, batch 3+4+5 gộp), 9 repository /
48 file migration (24 cặp).

## Files đã sửa/thêm

1. `backend-go/services/project-service/migrations/postgres/*.sql` (MOVED, `git mv`, nội dung không đổi, 48 file)
2. `backend-go/services/project-service/migrations/mysql/*.sql` (MỚI, 48 file)
3. `backend-go/services/project-service/internal/adapter/mysql/{repository,repo_repository,worktree_repository,project_group_repository,sparse_preset_repository,source_project_repository,host_setup_repository,folder_workspace_repository,outbox_repository}.go` (MỚI, 9 file)
4. `backend-go/services/project-service/internal/adapter/mysql/*_test.go` (MỚI, build tag `integration`, 9 file test)
5. `backend-go/services/project-service/internal/adapter/postgres/repository_test.go` (MODIFY — migrations path `../../../migrations` → `../../../migrations/postgres`)
6. `backend-go/services/project-service/cmd/server/main.go` (MODIFY — dialect factory + `toMySQLDriverDSN`, 9 port interface-typed vars)
7. `backend-go/services/project-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql` direct)
8. `backend-go/services/project-service/README.md` (MODIFY — migration paths, MySQL run instructions, closes stale Vault "Known gap" note)
9. `.github/workflows/backend-go-project-service.yml` (MỚI)
10. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-016-project-service-mysql-tidb-adapter.md` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-021-project-service-mysql-rollout.md` (MỚI, doc này)
12. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ thêm `project-service` vào hàng batch 3+4+5)

`internal/config/config.go` — KHÔNG sửa (đã có sẵn `DatabaseCredentialsFile`
từ trước, xác nhận qua Read trực tiếp).

## Verify

```bash
cd backend-go/services/project-service
go build ./... && go vet ./... && go vet -tags=integration ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration -timeout=30m ./internal/adapter/postgres/... -v
go test -tags=integration -timeout=30m ./internal/adapter/mysql/... -v

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-project-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trước khi sửa `run`/`Load`/8 interface DB-backed — tất cả
**LOW** (chi tiết ở "Kết quả thực tế" mục 1, bảng đầy đủ ở BE-DB-SOL-016
§2). `detect_changes()` chạy trước khi coi task DONE — xem báo cáo cuối
của agent (không lặp lại ở đây để tránh lệch nếu chạy lại), scope theo chỉ
file của `project-service` (môi trường có nhiều agent song song sửa các
service khác cùng batch).
