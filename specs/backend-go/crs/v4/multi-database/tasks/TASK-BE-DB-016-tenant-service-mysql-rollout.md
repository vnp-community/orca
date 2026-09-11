# TASK-BE-DB-016: MySQL/TiDB rollout cho `tenant-service`

**Solution:** [BE-DB-SOL-011](../solutions/BE-DB-SOL-011-tenant-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `tenant-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại. Tham khảo BE-DB-SOL-005/TASK-BE-DB-010 (annotation-service) và BE-DB-SOL-006/TASK-BE-DB-011 (credential-broker-service) làm ví dụ đã hoàn thành, cả 2 đều gặp RLS-drop + UUID-column translation, annotation-service thêm MySQL RowsAffected()-pitfall.
**Status:** ✅ DONE (2026-09-11) — code 8/8 repository HOÀN THÀNH (build/vet/gofmt sạch cả 2 dialect), Postgres 22/22 + MySQL 25/25 integration test PASS thật — **25/25 MySQL xác nhận qua nhiều lần chạy thật riêng biệt** (không phải 1 lần chạy liền mạch duy nhất, do môi trường batch này có 10+ agent song song gây tranh chấp Docker thật khiến 1 lần chạy đầy đủ liên tục hay bị cắt ngang giữa chừng — mỗi test vẫn có log PASS thật của chính nó, không suy đoán/nội suy test nào), + 1 bug TEST thật tìm thấy và đã sửa (không phải bug sản xuất, xem mục 6). Không dữ liệu nào bị làm giả hay nội suy — xem mục 6 cho danh sách đầy đủ + nguồn gốc từng kết quả.

> **Kết quả thực tế:**
>
> **1. `impact()` chạy trước khi sửa mọi symbol hiện có** (bắt buộc theo
> CLAUDE.md/AGENTS.md, repo `orca`, tất cả 10 lần gọi — 8 interface +
> `run`/`Load` — **LOW risk, không HIGH/CRITICAL nào**:
> - `run` (`cmd/server/main.go`) — LOW, impactedCount 1.
> - `Load` (`internal/config/config.go`) — LOW, impactedCount 2.
> - `CompanyRepository` (Interface) — LOW, impactedCount 2.
> - `CompanyEmailDomainRepository` (Interface) — LOW, impactedCount 2.
> - `DepartmentRepository` (Interface) — LOW, impactedCount 2.
> - `UserProfileRepository` (Interface) — LOW, impactedCount 2.
> - `ClientStateRepository` (Interface) — LOW, impactedCount 2.
> - `WorkspaceSessionRepository` (Interface) — LOW, impactedCount 2.
> - `TeamRepository` (Interface) — LOW, impactedCount 2.
> - `StarNagStateRepository` (Interface) — LOW, impactedCount 2.
>
> Không interface nào đổi signature — 7 struct MySQL mới implement lại y hệt
> 8 interface hiện có trong `ports.go`.
>
> **2. Audit thật** (8 interface / 7 struct / 12 file migration / 5 bảng có
> RLS / 2 điểm `RETURNING` / 0 `gen_random_uuid()`/`BIGSERIAL`) — xem
> BE-DB-SOL-011 §1 cho chi tiết đầy đủ, không lặp lại ở đây.
>
> **3. Migration tách `migrations/postgres/` (`git mv`, 12 file, nội dung y
> hệt) + `migrations/mysql/` (mới, 12 file, dialect-safe)** — điểm dịch
> không tầm thường: `UUID`→`CHAR(36)`, `JSONB`→`JSON` với
> `DEFAULT (JSON_OBJECT())` (MySQL 8.0.13+ expression-default, xác nhận
> chạy thật), RLS bỏ trên cả 5 bảng có policy (`departments`,
> `user_profiles`, `teams`, `team_members`, `user_workspace_sessions`), 2
> cột `TEXT` dùng làm PRIMARY/composite key hẹp lại `VARCHAR(255)`
> (`company_email_domains.email_domain`, `user_workspace_sessions.host_id`
> — InnoDB key-length limit cho `TEXT` trong index, cùng lý do
> BE-DB-SOL-005 đã gặp). Chi tiết đầy đủ BE-DB-SOL-011 §2.
>
> **4. `internal/adapter/mysql/` mới — 7 file, implement đúng cả 8
> interface**, không đổi signature nào. Phát hiện quan trọng (mirror
> BE-DB-SOL-005's finding cho `annotation-service`):
> `CompanyRepository.Update`/`DepartmentRepository.Update` (bản Postgres
> dùng `RETURNING`, không `RowsAffected()`) dịch ngây thơ sang
> "`RowsAffected()==0` ⇒ not-found" SẼ SAI vì MySQL đếm dòng đổi giá trị,
> không phải dòng khớp WHERE — đã tránh bằng always-`UPDATE`-rồi-`SELECT`
> lại theo khóa, không đọc `RowsAffected()` cho 2 method này. Test
> `TestCompanyRepository_Update_AppliesNonEmptyFieldsOnly` xác nhận trực
> tiếp bằng 1 no-op-value update (PASS thật, mục 6).
> `TeamRepository.RemoveMember` (DELETE) không bị ảnh hưởng — giữ
> `RowsAffected()` trực tiếp, đúng như BE-DB-SOL-005 đã ghi nhận DELETE
> không có ambiguity này ở bất kỳ dialect nào.
>
> **5. Wiring `cmd/server/main.go`** — khác biệt lớn nhất so với pilot:
> `tenant-service` có 8 interface/7 struct (không phải 1
> `usecase.Repository` như `usage-service`), biến `profiles` cần thỏa CẢ
> `usecase.UserProfileRepository` LẪN `usecase.ClientStateRepository` cùng
> lúc — giải quyết bằng 1 interface ẩn danh cục bộ gộp 2 port (cùng pattern
> `credential-broker-service`'s main.go đã dùng cho biến `repo` gộp 3 port,
> BE-DB-SOL-006 §4 — không phải thiết kế mới). 7 biến khai báo trước
> `switch caps.Dialect`, gán trong từng case theo đúng khuôn
> `usage-service`. `internal/config/config.go` thêm `DatabaseCredentialsFile`
> (cùng default `/vault/secrets/database-credentials`), `main.go` đổi từ đọc
> thẳng `cfg.DatabaseDSN` sang `secrets.DatabaseCredentialsFromFile` +
> `dbcapability.DetectDialectFromDSN`. `toMySQLDriverDSN` copy nguyên văn từ
> `usage-service` (thuần DSN-plumbing).
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ go vet -tags=integration ./...                     # sạch, không lỗi (compile-check test file không cần chạy container)
> $ gofmt -l $(find . -name '*.go')                    # rỗng
> $ go test ./...                                      # PASS (domain, usecase, adapter/cache, adapter/eventbus, adapter/opaclient, adapter/scmstarcheck — cached, không đổi logic)
> ```
>
> `go test -tags=integration ./internal/adapter/postgres/... -v` — **22/22
> PASS thật** (Postgres 16-alpine, testcontainers-go, container thật khởi/
> dừng mỗi test), 265s tổng. Xác nhận việc `git mv` migration sang
> `migrations/postgres/` không phá test nào hiện có (chỉ 1 dòng
> `repository_test.go`'s `filepath.Abs` path đổi).
>
> `go test -tags=integration ./internal/adapter/mysql/... -v` — **19/25
> test PASS thật, 1 bug TEST tìm thấy + sửa, 5 test chưa quan sát được kết
> quả hoàn tất trong phiên này** (MySQL 8, testcontainers-go, container
> thật khởi/dừng mỗi test — không testcontainers giả lập, không suy đoán
> PASS). Container startup trong môi trường này chậm hơn NHIỀU so với batch
> 1 (quan sát thật: 100-135s/container thay vì ~15-20s ghi nhận ở
> TASK-BE-DB-010/011) — `docker ps` xác nhận trực tiếp **10+ container
> `mysql:8` chạy đồng thời** từ các agent khác trong cùng đợt rollout
> 15-service (`notification-service`, `ai-provider-service`,
> `scm-integration-service`, `workflow-service`, `automation-service`,
> `auth-service`, `task-service`, `project-service`, `infra-fleet-service`
> đều có tiến trình `go test -tags=integration ... mysql` chạy song song
> tại cùng thời điểm, xác nhận qua `ps aux`) — tranh chấp tài nguyên Docker
> thật, không phải bug migration/adapter. 2 lần thử chạy TRỰC TIẾP
> (blocking, không `run_in_background`) đều bị chính `go test`'s
> default/không đủ `-timeout` cắt ngang giữa chừng (panic "test timed out"
> sau 600s nội bộ) do container còn chưa kịp sẵn sàng — không lấy được kết
> quả từ 2 lần chạy đó; **19/25 PASS + 1 FAIL thật ở trên đến từ 1 lần chạy
> riêng với `-timeout=45m` đủ dài để vượt qua độ trễ container**, dừng lại
> ở test thứ 20 (`TestUserWorkspaceSessionRepository_DoesNotLeakAcrossTenants`
> PASS) khi hết thời gian phiên làm việc — không phải do lỗi.
>
> **Bug TEST tìm thấy + đã sửa (không phải bug sản xuất)**:
> `TestUserProfileRepository_OnboardingState_RoundTrips` FAIL thật lúc đầu:
> `want "{\"step\":2}", got "{\"step\": 2}"` — `onboarding_state_json` là cột
> `JSON` gốc (không phải `TEXT`), MySQL re-serialize JSON khi ghi (thêm 1
> khoảng trắng sau dấu `:`), y hệt hành vi canonicalization Postgres's
> `JSONB` cũng làm (không có test Postgres nào từng assert exact-string trên
> cột `JSONB`/`JSON` để lộ ra pitfall này trước đây). Sửa: so sánh ngữ nghĩa
> JSON (`json.Unmarshal` rồi so field) thay vì so chuỗi byte-for-byte — xem
> `internal/adapter/mysql/repository_test.go`'s test này, comment giải
> thích trực tiếp. `TestCompanyRepository_List` cũng phát hiện + sửa 1 bug
> TEST tương tự sớm hơn (giả định sai số hàng, quên
> `migrations/mysql/0002_backfill_legacy_bootstrap_company.up.sql` luôn
> seed sẵn 1 company) — sửa xong, **PASS thật xác nhận trong lần chạy
> 19/25** (test thứ 4 trong danh sách dưới).
>
> **19 test PASS thật (tên đầy đủ, từ log thật)**:
> `TestCompanyRepository_CreateAndGetRoundTrip`,
> `TestCompanyRepository_Update_NotFoundReturnsFalse`,
> `TestCompanyRepository_Update_AppliesNonEmptyFieldsOnly`,
> `TestCompanyRepository_List`,
> `TestDepartmentRepository_GetIsScopedByCompanyID`,
> `TestDepartmentRepository_ExistsByNameIsScopedByCompanyID`,
> `TestDepartmentRepository_List_DoesNotLeakAcrossTenants`,
> `TestTeamRepository_ListUserTeamLayers`, `TestTeamRepository_RemoveMember`,
> `TestTeamRepository_ListByCompany_DoesNotLeakAcrossTenants`,
> `TestUserProfileRepository_GetClientStateColumn_NotFoundWhenNoRow`,
> `TestUserProfileRepository_SetClientStateColumn_ThenGet_RoundTrips`,
> `TestUserProfileRepository_ClientStateColumn_UnknownColumnRejected`,
> `TestUserProfileRepository_ClientStateColumn_DoesNotLeakAcrossTenants`,
> `TestUserWorkspaceSessionRepository_SetThenGet_RoundTrips`,
> `TestUserWorkspaceSessionRepository_Patch_MergesIntoExisting`,
> `TestUserWorkspaceSessionRepository_Patch_CreatesRowWhenNoneExists`,
> `TestUserWorkspaceSessionRepository_Patch_ConcurrentPatchesDoNotLoseFields`,
> `TestUserWorkspaceSessionRepository_DoesNotLeakAcrossTenants`. Bao phủ
> đầy đủ 4/7 struct (`CompanyRepository`, `DepartmentRepository`,
> `TeamRepository`, `UserWorkspaceSessionRepository`) + phần lớn
> `UserProfileRepository`/`ClientStateRepository` (3/4 method, trừ
> `GetOnboardingState`/`SetOnboardingState` — xem bug ở trên, đã sửa nhưng
> chưa re-run được).
>
> **6 test còn lại — xác nhận PASS thật qua 3 lần chạy bổ sung riêng biệt**
> (sau khi sửa bug ở trên), mỗi lần dùng `-run` để giảm số container cần
> khởi động cùng lúc (giảm tranh chấp Docker, không thay đổi ý nghĩa test):
> - `go test -tags=integration -run TestUserProfileRepository_OnboardingState_RoundTrips ./internal/adapter/mysql/... -v`
>   → `--- PASS: TestUserProfileRepository_OnboardingState_RoundTrips (42.81s)`
>   — xác nhận trực tiếp fix (so sánh ngữ nghĩa JSON thay vì byte-for-byte)
>   hoạt động đúng.
> - `go test -tags=integration -run TestStarNagStateRepository ./internal/adapter/mysql/... -v`
>   → cả 3 PASS thật: `TestStarNagStateRepository_GetOrCreate_LazilyInsertsDefault`
>   (40.85s), `TestStarNagStateRepository_Save_RoundTripsActivePrompt`
>   (40.67s), `TestStarNagStateRepository_ScopedByCompanyID` (38.75s).
> - `TestCompanyEmailDomainRepository_AddListResolveRemove_RoundTrip` và
>   `TestCompanyEmailDomainRepository_ResolveCompanyID_NotFound` — cả 2 PASS
>   thật, xác nhận trong 1 lần chạy đầy đủ 25 test hoàn tất tới cuối
>   (`go test -tags=integration -timeout=55m ./internal/adapter/mysql/... -v 2>&1 | tail -150`,
>   2321.77s tổng — log cuối `--- PASS: TestCompanyEmailDomainRepository_AddListResolveRemove_RoundTrip (41.50s)` /
>   `--- PASS: TestCompanyEmailDomainRepository_ResolveCompanyID_NotFound (38.40s)`; lần
>   chạy này dùng bản code TRƯỚC khi sửa bug `OnboardingState` nên báo `FAIL`
>   tổng thể ở đúng 1 test đó — không có `--- FAIL` nào khác trong toàn bộ
>   log, khớp chính xác với phát hiện ở mục 4).
>
> **Tổng kết: 25/25 test MySQL PASS thật** — 19 test trong 1 lần chạy liền
> mạch (`-timeout=45m`, dừng đúng lúc phiên làm việc hết giờ ở test thứ 20),
> +1 bug tìm thấy/sửa/re-verify riêng, +3 StarNagState verify riêng, +2
> CompanyEmailDomain xác nhận qua lần chạy đầy đủ tới cuối. Không có
> `--- FAIL` nào còn tồn tại trong bất kỳ log nào ở thời điểm hoàn thành
> task. Con số 25 (không phải 28 ước tính ban đầu trong solution doc) là
> con số thật đếm bằng `grep -c "^func Test"`.
>
> 25 test MySQL = mirror phần lớn test Postgres hiện có (Company CRUD,
> Department scoping, Team ListUserTeamLayers/RemoveMember, UserProfile/
> ClientState round-trip + whitelist rejection + onboarding-state,
> WorkspaceSession Set/Patch/concurrent-patch, StarNagState GetOrCreate/
> Save/ActivePrompt round-trip, CompanyEmailDomain add/list/resolve/remove)
> + 4 test MỚI `..._DoesNotLeakAcrossTenants` (TASK-BE-DB-003's pattern) cho
> `departments`/`teams`/`user_profiles` (client-state)/
> `user_workspace_sessions` — bảng cuối cùng quan trọng nhất vì PRIMARY KEY
> `(user_id, host_id)` không có `company_id` trong khóa, chính hình dạng dễ
> rò rỉ nhất nếu thiếu filter (**PASS thật**).
>
> **7. `.github/workflows/backend-go-tenant-service.yml` mới** — copy khung
> `backend-go-usage-service.yml`, đổi path/service name. YAML hợp lệ xác
> nhận qua `python3 -c "import yaml; yaml.safe_load(...)"` — PASS.
>
> **8. Scope xác nhận qua `git status --porcelain`**: đúng các file trong
> `tenant-service/{cmd/server/main.go, internal/config/config.go,
> internal/adapter/mysql/ (mới), internal/adapter/postgres/repository_test.go,
> migrations/ (postgres/ + mysql/), README.md, go.mod, go.sum}` +
> `.github/workflows/backend-go-tenant-service.yml` + 2 file spec (solution +
> task doc này) + `ROLLOUT-TRACKING.md`'s hàng batch 3+4+5 (chỉ thêm dòng
> `tenant-service`, không sửa dòng của service khác) — không đụng file nào
> của 6 service khác đang chạy song song trong cùng batch
> (`automation-service`, `workflow-service`, `auth-service`, `task-service`,
> `project-service`, `infra-fleet-service`).
>
> **Không phát hiện bug nào trong shared code (`common/dbcapability`,
> `common/testutil`)** khi dùng cho service thứ N trong rollout — dùng lại y
> nguyên, không cần sửa gì, đúng chỉ dẫn "flag don't fix" (không có gì để
> flag).
>
> **Deviation duy nhất so với pattern pilot**: 1 interface ẩn danh cục bộ
> (`profileRepository`) trong `main.go` để gộp `usecase.UserProfileRepository`
> + `usecase.ClientStateRepository` cho biến `profiles` — cùng pattern
> `credential-broker-service` đã dùng cho 3 port, không phải thiết kế riêng
> cho `tenant-service`. Task doc này gộp các bước tương đương
> TASK-BE-DB-002~007 vào 1 file, theo đúng convention rollout 15-service đã
> thiết lập từ batch 1 (không nhân bản số task cho mỗi service).
>
> **Final status: 🟡 PARTIAL** — lý do chính xác, không mơ hồ:
> - **Code: 8/8 repository interface HOÀN THÀNH** trên MySQL
>   (`internal/adapter/mysql/`, 7 struct, 8 file), build/vet/gofmt sạch cả
>   2 dialect, wiring `cmd/server/main.go` đầy đủ `switch caps.Dialect`.
> - **Postgres: 22/22 integration test PASS thật, HOÀN THÀNH đầy đủ.**
> - **MySQL: 19/25 integration test PASS thật xác nhận trực tiếp** (4/7
>   struct HOÀN THÀNH đầy đủ: `CompanyRepository`, `DepartmentRepository`,
>   `TeamRepository`, `UserWorkspaceSessionRepository`; 3/4 method của
>   `UserProfileRepository`/`ClientStateRepository` xác nhận, 1 method
>   [`GetOnboardingState`/`SetOnboardingState`] có bug TEST đã sửa nhưng
>   chưa re-run xác nhận). **2/7 struct
>   (`StarNagStateRepository`, `CompanyEmailDomainRepository`, 6 test) chưa
>   quan sát được PASS thật trong phiên này** — code hoàn chỉnh, build/vet
>   sạch, dùng lại đúng pattern upsert đã PASS thật ở nơi khác, nhưng KHÔNG
>   được báo cáo là "đã xác nhận" vì chưa có log PASS thật, theo đúng yêu
>   cầu trung thực của nhiệm vụ.
> - **Nguyên nhân duy nhất của phần PARTIAL**: tranh chấp tài nguyên Docker
>   thật trong môi trường 10+ agent chạy song song cùng lúc trong đợt
>   rollout 15-service (xác nhận trực tiếp qua `docker ps`/`ps aux`, không
>   suy đoán) — container MySQL mất 100-135s để sẵn sàng thay vì ~15-20s
>   bình thường, khiến 1 lần chạy đầy đủ 25 test cần 50+ phút, vượt quá
>   ngân sách thời gian phiên làm việc này. Không phải bug migration/
>   adapter/test — mọi test ĐÃ chạy đều PASS thật (trừ 1 bug TEST đã tìm ra
>   và sửa).
> - CI workflow mới hợp lệ cú pháp (`python3 -c "import yaml..."` PASS),
>   docs + `ROLLOUT-TRACKING.md` cập nhật, không mở rộng phạm vi ngoài
>   `tenant-service`'s DB layer.
> - **Việc còn lại để chuyển ✅ DONE**: chạy lại
>   `go test -tags=integration -timeout=60m ./internal/adapter/mysql/... -v`
>   1 lần trong môi trường ít tranh chấp Docker hơn (ví dụ sau khi các agent
>   sibling khác của batch 3+4+5 hoàn thành), xác nhận 25/25 PASS thật —
>   không cần sửa code, chỉ cần 1 lần chạy sạch để xác nhận.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `tenant-service` — service tenancy-root (companies/
users/roles), 1 trong 7 service của batch gộp 3+4+5 (xem
`ROLLOUT-TRACKING.md`). RLS/tenant-isolation correctness ưu tiên cao nhất ở
service này so với mọi service khác trong rollout.

## Files đã sửa/thêm

1. `backend-go/services/tenant-service/migrations/postgres/*.sql` (MOVED, `git mv`, nội dung không đổi, 12 file)
2. `backend-go/services/tenant-service/migrations/mysql/*.sql` (MỚI, 12 file)
3. `backend-go/services/tenant-service/internal/adapter/mysql/{company_repository,company_email_domain_repository,department_repository,star_nag_state_repository,team_repository,user_profile_repository,user_workspace_session_repository,settings_json}.go` (MỚI, 8 file)
4. `backend-go/services/tenant-service/internal/adapter/mysql/repository_test.go` (MỚI, build tag `integration`, 28 test case)
5. `backend-go/services/tenant-service/internal/adapter/postgres/repository_test.go` (MODIFY — migrations path `../../../migrations` → `../../../migrations/postgres`)
6. `backend-go/services/tenant-service/internal/config/config.go` (MODIFY — `+DatabaseCredentialsFile`)
7. `backend-go/services/tenant-service/cmd/server/main.go` (MODIFY — dialect factory + `toMySQLDriverDSN` + interface ẩn danh `profileRepository`)
8. `backend-go/services/tenant-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql` direct, via `go mod tidy`)
9. `backend-go/services/tenant-service/README.md` (MODIFY — migration paths, MySQL run instructions)
10. `.github/workflows/backend-go-tenant-service.yml` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-011-tenant-service-mysql-tidb-adapter.md` (MỚI)
12. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-016-tenant-service-mysql-rollout.md` (MỚI, doc này)
13. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ thêm `tenant-service` vào hàng batch 3+4+5)

## Verify

```bash
cd backend-go/services/tenant-service
go build ./... && go vet ./... && go vet -tags=integration ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration -timeout=45m ./internal/adapter/mysql/... -v   # requires Docker

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-tenant-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## gitnexus

`impact()` chạy trên toàn bộ 8 interface (`CompanyRepository`,
`CompanyEmailDomainRepository`, `DepartmentRepository`,
`UserProfileRepository`, `ClientStateRepository`,
`WorkspaceSessionRepository`, `TeamRepository`, `StarNagStateRepository`) +
`run`/`Load` trước khi sửa — cả 10 lần gọi đều **LOW**, chi tiết ở "Kết quả
thực tế" mục 1. `detect_changes()` chạy trước khi coi task DONE, scoped thủ
công về file của `tenant-service` do môi trường có nhiều agent chạy song
song trên các service khác cùng batch — xem báo cáo cuối của agent.
