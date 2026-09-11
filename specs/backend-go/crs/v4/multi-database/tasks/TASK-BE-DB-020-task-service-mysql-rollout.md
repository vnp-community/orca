# TASK-BE-DB-020: Multi-database rollout cho `task-service`

**Solution:** [BE-DB-SOL-015](../solutions/BE-DB-SOL-015-task-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `task-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot), BE-DB-SOL-005's RowsAffected checklist
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa/thêm bất kỳ symbol nào** (bắt
> buộc theo AGENTS.md/CLAUDE.md) — 13 lần chạy trên repo `orca`
> (`TaskRepository`/`EdgeRepository`/`GrantRepository`/`ShareLinkRepository`/
> `CommentRepository`/`ExecutionLinkRepository`/`OutboxWriter`/`TxRunner`/
> `VelocityResolver` — interface trong `ports.go` — cộng `New`/`run`/`Load`/`Config`),
> chi tiết đầy đủ + bảng kết quả ở BE-DB-SOL-015 §2. Không HIGH/CRITICAL
> nào — MEDIUM/10 cho mọi interface là hiện tượng file-level `IMPORTS` đã
> ghi nhận từ BE-DB-SOL-004/005/006, không phải phụ thuộc chữ ký thật.
> `OutboxWriter`/`TxRunner` cần disambiguate qua `file_path` (trùng tên
> với symbol ở service/package khác). Không đổi chữ ký interface nào.
>
> **2. Audit thật xác nhận đúng số liệu ROLLOUT-TRACKING.md** (10
> repo/file adapter, 22 migration) — chi tiết đầy đủ ở BE-DB-SOL-015 §1.
> Điểm khác biệt lớn nhất so với mọi service trước trong rollout: **có cột
> mảng Postgres thật đang dùng** (`labels TEXT[]`) và **có sequence
> Postgres thật đang dùng** (`task.task_number_seq` qua `nextval()` inline
> trong `Create`) — cả 2 chưa từng gặp ở usage-service/issue-tracking-service/
> annotation-service/credential-broker-service.
>
> **3. Migration tách `migrations/postgres/` (11 cặp, `git mv` nguyên
> văn) + `migrations/mysql/` (11 cặp mới)** — dịch đầy đủ, chi tiết ở
> BE-DB-SOL-015 §4. Phát hiện thật quan trọng nhất: `labels TEXT[]` →
> cột `JSON`; `task_number_seq` → bảng `AUTO_INCREMENT` (MySQL không có
> `CREATE SEQUENCE`); và **1 lỗi InnoDB thật phát hiện khi chạy migration
> lần đầu trên container `mysql:8`** — `ERROR 1215: Cannot add foreign key
> constraint` khi `task_edges`'s cột generated `single_parent_key`
> (emulate partial unique index của Postgres) cùng tồn tại với
> `fk_task_edges_to ... ON DELETE CASCADE` trên CÙNG cột nguồn
> (`to_task_id`) — InnoDB không cho phép CASCADE/SET NULL trên 1 FK khi
> cột con của nó là input của 1 generated column STORED cùng bảng. Xác
> nhận nguyên nhân bằng bisect trực tiếp trên container `mysql:8.4.11`
> thật (không suy đoán từ tài liệu), sửa bằng cách bỏ `ON DELETE CASCADE`
> khỏi FK đó + thêm 1 `TRIGGER BEFORE DELETE ON tasks` khôi phục đúng
> hành vi cascade cho `to_task_id`. Verify cả `migrate up` LẪN `migrate
> down -all` chạy sạch qua CLI `migrate` thật (11/11 lên, 11/11 xuống) —
> xem mục 6.
>
> **4. `internal/adapter/mysql/` — 9 file mới** (+ `ids.go` cho
> `newUUID()` dùng chung), implement lại đúng toàn bộ 10 "repository"
> Postgres có, KHÔNG đổi chữ ký interface nào. Chi tiết đầy đủ (RETURNING/
> gen_random_uuid() cho bảng phụ, checklist RowsAffected theo từng hàm,
> `IN (?,...)` động) ở BE-DB-SOL-015 §3.
>
> **5. Wiring `cmd/server/main.go`** — khác pilot (`repo` chỉ cần thoả 1-3
> interface): `task-service`'s `repo` cần thoả 9 interface DB-backed khác
> nhau xuyên suốt ~30 dòng wiring usecase hiện có. Giải pháp: 1 interface
> cục bộ `repoAll` (union toàn bộ interface `Repository` implement) khai
> báo ngay trong `run()`, gán 1 lần trong `switch caps.Dialect` — không
> sửa 1 dòng nào trong các lời gọi usecase constructor hiện có. `go build`
> tự bắt thiếu `taskeventbus.OutboxWriter` (interface thứ 2, hẹp hơn,
> trùng tên `OutboxWriter` nhưng khác package) lúc build lần đầu — thêm
> vào `repoAll`, build sạch ngay sau đó. `Config` thêm `DatabaseCredentialsFile`
> (đóng gap README's "Vault not wired"), `toMySQLDriverDSN` copy nguyên
> vẹn từ `usage-service`. Chi tiết đầy đủ ở BE-DB-SOL-015 §5.
>
> **6. Build/test thật đã chạy** (Docker + testcontainers thật, `migrate`
> CLI thật với cả 2 driver `postgres`/`mysql`, không giả định):
> ```
> go build ./...          → sạch
> go vet ./...             → sạch
> gofmt -l .                → sạch (toàn bộ file .go trong service)
> go mod tidy               → +github.com/go-sql-driver/mysql v1.10.1
> GOWORK=off go build ./... → sạch (sau go mod tidy; trước đó thiếu vài
>                              go.sum entry TIỀN TỒN TẠI không liên quan
>                              mysql, go mod tidy đã đóng luôn)
> go test ./... (unit)      → PASS toàn bộ (domain/usecase/grpc/grpcclient
>                              không đổi, cached)
> migrate up (mysql, CLI thật, 11 migration) → 11/11 PASS
> migrate down -all (mysql, CLI thật, 11 migration) → 11/11 PASS
> go test -tags=integration ./internal/adapter/mysql/... -v
>                          → **41/41 PASS thật** (859.7s tổng, MySQL 8 qua
>                            testcontainers-go thật, không giả định)
> go test -tags=integration ./internal/adapter/postgres/... -v
>                          → **28/28 PASS thật** (sau khi vá mục 7 —
>                            trước khi vá: 10/28 FAIL, xem mục 7)
> ```
>
> **7. Phát hiện phụ NGHIÊM TRỌNG, NGOÀI PHẠM VI ban đầu — đã báo người
> dùng và ĐƯỢC YÊU CẦU SỬA NGAY, nên đã vá** (lệch có chủ đích khỏi
> nguyên tắc "chỉ SQL adapter layer" của task này, theo chỉ dẫn trực
> tiếp): chạy `go test -tags=integration ./internal/adapter/postgres/...
> -v` (đối chứng việc tách migration không phá vỡ gì) lộ ra **3 bug thật
> độc lập**, không liên quan tới multi-database rollout:
>
> - **Bug A — CreateTask fail 100% trên Postgres thật**: `domain.NewTask`
>   không khởi tạo `Labels` (nil); `postgres.Repository.Create` INSERT
>   thẳng `nil` vào cột `labels TEXT[] NOT NULL` (migration 0011) → vi
>   phạm NOT NULL (DEFAULT `'{}'` chỉ áp dụng khi cột bị bỏ khỏi INSERT,
>   không áp dụng khi truyền NULL tường minh) — chặn TOÀN BỘ `CreateTask`
>   production thật (`usecase.CreateTask` không set `Labels`, xác nhận
>   qua đọc trực tiếp code). Chưa từng bị phát hiện vì
>   `create_task_test.go` dùng `fakeTaskRepository` (không có ràng buộc
>   NOT NULL thật). **Đã vá 2 lớp (defense-in-depth)**: `domain.NewTask`
>   (`internal/domain/task.go`) khởi tạo `Labels: []string{}`, VÀ
>   `postgres.Repository.Create` (`internal/adapter/postgres/repository.go`)
>   coalesce `nil` → `[]string{}` trước khi bind — lớp thứ 2 cần thiết vì
>   `TestRepository_Create_PersistsAllFields` tự dựng `domain.Task{}` trực
>   tiếp (bỏ qua `NewTask`), chứng minh bất kỳ caller nào không qua
>   `NewTask` cũng dính lỗi tương tự. `impact()` xác nhận LOW risk (1
>   caller) trước khi sửa `NewTask`. MySQL adapter (`marshalLabels`)
>   **đã tự có** cùng pattern coalesce này từ trước — không cần vá.
> - **Bug B — `GetSubtree`/`GetSubtreeWithChildPercents` fail 100%**
>   (`internal/adapter/postgres/subtree.go`): `WITH RECURSIVE subtree AS
>   (...)` không đặt tên cột tường minh cho CTE — Postgres tự đặt tên cột
>   theo biểu thức (`COALESCE(parent_id::text, '')` → cột tên `coalesce`,
>   KHÔNG phải `parent_id`); câu SELECT cuối cùng (`SELECT taskColumns
>   FROM subtree`) lại tái sử dụng `taskColumns` — tham chiếu tên cột gốc
>   (`parent_id`, `project_id`,...) vào CTE `subtree`, nơi các tên đó
>   không tồn tại → `ERROR: column "parent_id" does not exist
>   (SQLSTATE 42703)`. Xác nhận độc lập bằng `psql` thủ công (tách khỏi Go)
>   trước khi sửa. **Đã vá**: thêm hằng `subtreeColumnNames`
>   (`internal/adapter/postgres/repository.go`) đặt tên tường minh cho
>   CTE khớp tên cột thật, dùng ở cả 2 hàm — double-COALESCE/cast phát
>   sinh là no-op an toàn (giá trị trong CTE đã non-null/đã ép kiểu text
>   từ vòng lặp gốc). `impact()` xác nhận LOW risk (0 caller nội bộ khác)
>   trước khi sửa cả 2 hàm.
> - **Bug C — lỗi TEST, không phải bug production**:
>   `TestAddEdge_AutoBlock_PersistsAgainstRealDB` assert sai task — theo
>   doc comment của `TaskEdge` (`internal/domain/task_edge.go`), "from
>   depends on to" = "from phải đợi to", nên task bị auto-block đúng là
>   `from` (khớp code thật, `addEdgeWithinTx` gọi
>   `UpdateStatus(edge.FromTaskID, StatusBlocked)`) — nhưng test lại check
>   status của `to.ID`. Business logic ĐÚNG, chỉ vá assertion của test
>   (`internal/adapter/postgres/add_edge_integration_test.go`) sang check
>   `from.ID`.
>
> Cả 3 đã vá + verify lại **28/28 Postgres PASS thật** (trước khi vá:
> 10/28 FAIL — 8 do Bug A + 1 do Bug B + 1 do Bug C). Chi tiết thiết kế ở
> BE-DB-SOL-015 §6 (ghi nhận Bug A như phát hiện ban đầu trước khi vá).
>
> **8. CI workflow** — `.github/workflows/backend-go-task-service.yml`
> mới, copy khung `backend-go-usage-service.yml`, đổi path/service name.
> `python3 -c "import yaml; yaml.safe_load(...)"` xác nhận YAML hợp lệ.
> Cả 2 lane `postgres`/`mysql` xanh thật (sau khi vá mục 7) — không còn
> bug tiền tồn tại nào chặn CI.
>
> **Kết luận trạng thái: ✅ DONE** — build/vet/gofmt sạch, 28/28 Postgres
> + 41/41 MySQL integration test PASS thật (69/69 tổng), 3 bug thật phát
> hiện ngoài phạm vi ban đầu (2 bug production nghiêm trọng + 1 bug test)
> đều đã vá theo yêu cầu trực tiếp, không còn FAIL/placeholder nào.

---

## Mục tiêu

Nhân rộng pattern CR-DB-002/CR-DB-003 (`dbcapability` + adapter MySQL/TiDB
song song + migration dialect-safe + test tenant-isolation-without-RLS +
CI matrix) từ pilot `usage-service` sang `task-service` — 1 trong 15
service của rollout (`ROLLOUT-TRACKING.md`, batch 3+4+5 gộp), lớn nhất đã
làm tính tới nay (10 repo, 22 migration).

## Files đã sửa/thêm

1. `backend-go/services/task-service/migrations/postgres/*.sql` (MOVED, `git mv`, 22 file, nội dung không đổi)
2. `backend-go/services/task-service/migrations/mysql/*.sql` (MỚI, 22 file)
3. `backend-go/services/task-service/internal/adapter/mysql/*.go` (MỚI, 9 file: `repository.go`, `edges.go`, `grants.go`, `comments.go`, `share_link.go`, `share_links.go`, `execution_links.go`, `outbox.go`, `subtree.go`, `velocity.go`, `ids.go`)
4. `backend-go/services/task-service/internal/adapter/mysql/*_test.go` (MỚI, 4 file: `repository_test.go`, `execution_links_test.go`, `share_links_test.go`, `subtree_test.go`)
5. `backend-go/services/task-service/internal/adapter/postgres/repository_test.go` (MODIFY — `migrationsPath` trỏ `migrations/postgres`)
6. `backend-go/services/task-service/internal/adapter/postgres/share_links_test.go` (MODIFY — `migrationsPath` trỏ `migrations/postgres`)
7. `backend-go/services/task-service/internal/config/config.go` (MODIFY — `+DatabaseCredentialsFile`)
8. `backend-go/services/task-service/cmd/server/main.go` (MODIFY — dialect factory qua `repoAll` union interface + `toMySQLDriverDSN`)
9. `backend-go/services/task-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go mod tidy`)
10. `backend-go/services/task-service/README.md` (MODIFY — migration paths, MySQL run instructions, đóng "Vault not wired" known gap)
11. `.github/workflows/backend-go-task-service.yml` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-015-task-service-mysql-tidb-adapter.md` (MỚI)
13. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-020-task-service-mysql-rollout.md` (MỚI, doc này)
14. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — hàng `task-service` only)
15. `backend-go/services/task-service/internal/domain/task.go` (MODIFY — Bug A: `NewTask` khởi tạo `Labels: []string{}`)
16. `backend-go/services/task-service/internal/adapter/postgres/repository.go` (MODIFY — Bug A: `Create` coalesce `Labels` nil→`[]string{}`; Bug B: thêm hằng `subtreeColumnNames`)
17. `backend-go/services/task-service/internal/adapter/postgres/subtree.go` (MODIFY — Bug B: đặt tên cột CTE tường minh ở `GetSubtree`/`GetSubtreeWithChildPercents`)
18. `backend-go/services/task-service/internal/adapter/postgres/add_edge_integration_test.go` (MODIFY — Bug C: sửa assertion sai task)

## Verify

```bash
cd backend-go/services/task-service
go build ./...
go vet ./...
gofmt -l .
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v   # requires Docker — lane ĐỎ, bug tiền tồn tại (mục 7)
go test -tags=integration ./internal/adapter/mysql/... -v      # requires Docker

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-task-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trước khi sửa 9 interface (`ports.go`) + `New`/`run`/
`Load`/`Config` — tất cả LOW hoặc MEDIUM (file-level IMPORTS, không phải
phụ thuộc chữ ký thật), không HIGH/CRITICAL nào — chi tiết đầy đủ ở
BE-DB-SOL-015 §2. `detect_changes()` chạy trước khi coi task DONE — xem
kết quả trong báo cáo cuối của agent.
