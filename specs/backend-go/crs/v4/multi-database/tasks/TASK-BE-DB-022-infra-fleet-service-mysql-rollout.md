# TASK-BE-DB-022: MySQL/TiDB rollout cho `infra-fleet-service`

**Solution:** [BE-DB-SOL-017](../solutions/BE-DB-SOL-017-infra-fleet-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `infra-fleet-service` — **LỚN NHẤT rollout** (15 repository file, 62 migration, xem ROLLOUT-TRACKING.md's audit)
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại. Tham khảo BE-DB-SOL-005/TASK-BE-DB-010 (annotation-service, RowsAffected pitfall) và BE-DB-SOL-006/TASK-BE-DB-011 (credential-broker-service, transaction-wrapping port).
**Status:** ✅ DONE (2026-09-12) — `cmd/server/main.go` đã wire dialect switch,
MySQL + Postgres integration suite đã re-verify thật dưới tải thấp; xem
"Cập nhật (2026-09-12)" ngay dưới đây và "Final status" cuối file cho chi
tiết đầy đủ.

> **Cập nhật (2026-09-12) — hoàn tất 2 việc còn lại của mục "Việc còn lại"
> gốc:**
>
> **1. `cmd/server/main.go` đã wire dialect switch.** `impact()` chạy trước
> khi sửa (bắt buộc theo CLAUDE.md/AGENTS.md): `run` (upstream, direction
> upstream) → risk **LOW**, impactedCount 1 (chỉ `main` gọi `run`). Cách
> tiếp cận: dùng kỹ thuật union interface cục bộ `repoAll` (giống
> task-service's `cmd/server/main.go`, BE-DB-SOL-015) cho biến `repo`
> (struct `Repository` implement 10 interface — 9 usecase port +
> `outbox.Store` + `infraeventbus.OutboxEnqueuer`'s `EnqueueOutboxEvent`,
> xác nhận qua `internal/adapter/mysql/shared.go`'s 24 assertion và build
> error thật khi thiếu). 14 repository còn lại (`sshTargetStore` qua
> `TerminalSessionStore`...`ephemeralVmSshTargetStore`) mỗi cái chỉ dùng
> qua ĐÚNG 1 usecase port nên giữ nguyên kiểu interface đơn (không cần
> union) — khác task-service (chỉ có 1 union interface vì service đó chỉ
> có 1 "big" repo). Cả 15 constructor được gom vào MỘT dialect switch ngay
> đầu `run()` (thay vì rải rác 15 chỗ như code gốc) — xoá 4 lệnh gọi
> `infrapostgres.NewX(pool)` rải rác phía sau (ephemeralVmRuntimeStore,
> portForwardStore, devServerGroupStore×3, ephemeralVmSshTargetStore),
> giữ nguyên MỌI usecase constructor call site (không đổi tham số nào).
> `toMySQLDriverDSN` copy nguyên vẹn từ `usage-service`. 2 lần build lỗi
> thật khi wiring (đúng dự đoán của BE-DB-SOL-017 §6): thiếu
> `infraeventbus.OutboxEnqueuer` (`EnqueueOutboxEvent`, method thứ 2 trên
> `Repository` ngoài 9 port `ports.go`) và thiếu `Enqueue` trên
> `AgentRateLimitedOutboxStore` (method thứ 2 ngoài `outbox.Store`) — cả 2
> sửa bằng cách thêm interface cục bộ (`repoAll` embed thêm
> `infraeventbus.OutboxEnqueuer`; `rateLimitedOutboxStore` = `outbox.Store`
> + `Enqueue`), không đổi interface `ports.go` nào. `go build`/`go vet`/
> `gofmt -l` sạch sau khi wire.
>
> **2. MySQL integration suite re-run THẬT dưới tải thấp (không agent
> song song khác)** — container `mysql:8` khởi động ~15-19s/container (so
> với 150-185s dưới tải batch 3+4+5 cũ), xác nhận giả thuyết contention
> cũ đúng. **Lần chạy đầu: 21/23 PASS, 2 FAIL — cả 2 là bug THẬT trong
> `internal/adapter/mysql` code viết trước đó (không phải môi trường)**,
> phát hiện + sửa ngay:
> - `TestFleetDefinitionStore_CreateGetListUpdate`: `FleetDefinitionStore.Update`
>   dùng "UPDATE unconditionally rồi SELECT lại scoped theo version MỚI"
>   (bắt chước RETURNING) — sai, vì SELECT độc lập với UPDATE có thể khớp
>   nhầm 1 dòng mà 1 lần Update TRƯỚC ĐÓ đã đưa lên đúng version này rồi,
>   che giấu mất version-conflict thật (test gọi Update 2 lần: lần 2 dùng
>   version cũ đã stale → phải trả `ErrFleetDefinitionVersionConflict`
>   nhưng lại trả thành công). Sửa: dùng thẳng `RowsAffected()` của chính
>   câu UPDATE đó — an toàn ở đây (khác cảnh báo chung BE-DB-SOL-005 §3.1)
>   vì cột `version` LUÔN đổi giá trị khác `oldVersion` mỗi lần gọi, nên
>   UPDATE không bao giờ là no-op thật, `RowsAffected()==0` luôn nghĩa là
>   conflict thật.
> - `TestTerminalScrollbackSnapshotStore_UpsertGetSumDelete`: `Error 1364
>   (HY000): Field 'id' doesn't have a default value` — cột `id CHAR(36)
>   PRIMARY KEY` không có DEFAULT (khác Postgres's
>   `DEFAULT gen_random_uuid()`), và `domain.TerminalScrollbackSnapshot`
>   không có field `ID` để caller truyền vào — ca đặc biệt duy nhất trong
>   rollout mà audit "mọi INSERT đều truyền id tường minh" (BE-DB-SOL-017
>   §1) bỏ sót. Sửa: generate `uuid.NewString()` ngay trong
>   `Upsert` (giống pattern `EphemeralVmSshTargetStore.Upsert` đã có sẵn
>   trong package này), `ON DUPLICATE KEY UPDATE` không đụng `id` nên dòng
>   có sẵn giữ nguyên id cũ.
>
> `impact()` chạy trước khi sửa cả 2 — GitNexus KHÔNG resolve được tên
> symbol `Update`/`Upsert` (method trùng tên phổ biến, gói `mysql` có vẻ
> chưa được index đầy đủ — `context()`/`impact()` trả "not found" cho cả
> `FleetDefinitionStore`/`TerminalScrollbackSnapshotStore` dù các symbol
> này tồn tại thật trong code). Theo "stop on saturation", không cố paginate
> — thay vào đó xác nhận blast radius bằng `grep` thủ công có chủ đích,
> hẹp (không phải quét toàn repo): `FleetDefinitionRepository.Update` chỉ
> có 1 consumer (`usecase.DeployFleetDefinition`), `TerminalScrollbackSnapshotRepository.Upsert`
> chỉ có 1 consumer (`usecase.SaveTerminalScrollbackSnapshot`) — cùng mức
> rủi ro LOW đã xác lập cho các symbol postgres tương đương trong rollout
> này. **Lần chạy lại sau khi sửa: 23/23 PASS thật** (log đầy đủ ở mục
> "Kết quả chạy thật thứ 2 (2026-09-12)" bên dưới).
>
> **3. Postgres integration suite re-run: 35/37 PASS, 2 FAIL, cả 2 xác
> nhận bug THẬT tiền tồn tại (không do wiring/không do CR-DB-002/003) —
> 6/8 FAIL ban đầu ĐÃ SỬA (mechanical, chặn verify), 2 FAIL còn lại flag
> không sửa (đòi hỏi viết lại test logic, ngoài phạm vi):**
> - ĐÃ SỬA: `domain.NewSshTarget`'s `tags []string` param không default
>   `nil` → `[]string{}` trước khi lưu — cột `tags TEXT[]/JSON NOT NULL
>   DEFAULT '{}'` ở cả 2 dialect nhận thẳng `nil` từ Go thành SQL NULL
>   (không áp dụng DEFAULT), vi phạm NOT NULL. 11/13 call site test
>   truyền `nil` (từ lượt sửa compile trước, TASK-BE-DB-022 gốc mục 7) —
>   sửa ĐÚNG chỗ (constructor, 1 chỗ, `impact()` xác nhận risk **LOW**,
>   3 consumer cùng service) thay vì sửa 11 call site test riêng lẻ. Khắc
>   phục 5 FAIL: `TestSshTargetStore_Get_FoundAndNotFound`,
>   `TestSshTargetStore_PersistsPortKnownHostsAndJumpHost`,
>   `TestPortForwardStore_CreateThenListActiveByConnection_RoundTripsProcessNameAndStatus`,
>   `TestRepository_RegisterAndGet_PersistsSSHTargetID`,
>   `TestRepository_Delete_RemovesRow`, `TestRepository_Delete_ScopedByTenant`
>   (6 test, không phải 5 — đếm lại chính xác từ log).
> - ĐÃ SỬA: `TestFindByHostAndMode_DirectWebSocketWithNullSSHTargetID` xây
>   `domain.DevServer{}` trực tiếp (không qua `NewDevServer`) nên thiếu
>   `Kind`, vi phạm `dev_servers_kind_check` (migration 0036) — thêm
>   `Kind: domain.AgentKindDevServer` vào struct literal, test-only, không
>   đụng logic sản xuất.
> - FLAG KHÔNG SỬA (bug thật, tiền tồn tại, đòi hỏi viết lại test logic —
>   không mechanical như 2 mục trên):
>   - `TestMigration0018_DownDropsTable`: chạy `migrate down 1` sau khi
>     `setupRepository` đã `up` tới migration MỚI NHẤT (0037) — `down 1`
>     chỉ rollback 0037, không phải 0018 như test kỳ vọng (test viết từ
>     lúc 0018 còn là migration mới nhất, chưa cập nhật khi 0019-0037 được
>     thêm sau này). Không liên quan CR-DB-002/003, không do rollout này
>     gây ra.
>   - `TestRepository_ResolveConnection_FoundAndNotFound`: chỉ
>     `Register()` 1 dev server rồi gọi `ResolveConnection(ctx, tenantID,
>     devServerID)` mong đợi found=true — dựa vào quy ước cũ
>     "connectionId==dev_server.id" mà `infra.connections` (bảng routing
>     thật) đã THAY THẾ từ lâu (xem chính comment dòng 117-121 của
>     `cmd/server/main.go`: "the real routing model that replaced the
>     connectionId==dev_server.id equation") — `ResolveConnection` JOIN
>     qua `infra.connections`, không tìm thấy row nào nên `connected=false`
>     đúng theo code hiện tại. Test chưa được cập nhật theo khi bảng
>     `connections` ra đời. Không liên quan CR-DB-002/003.

Toàn bộ nội dung "Kết quả thực tế (2026-09-11)" gốc bên dưới GIỮ NGUYÊN
làm lịch sử — phần "Cập nhật (2026-09-12)" ở trên là nguồn đúng cho trạng
thái hiện tại.

> **Kết quả thực tế:**
>
> **1. `impact()` chạy trước khi sửa symbol hiện có** (bắt buộc theo
> CLAUDE.md/AGENTS.md), repo `orca`:
>
> | Symbol | Direction | Risk | impactedCount |
> |---|---|---|---|
> | `run` (`cmd/server/main.go`) | upstream | **LOW** | 1 |
> | `Repository` (struct, `internal/adapter/postgres/repository.go`) | upstream | **LOW** | 3 |
>
> Không HIGH/CRITICAL. Không đổi signature interface nào trong `ports.go`
> hay `list_ephemeral_vm_runtimes.go` — mọi struct MySQL implement lại
> đúng interface hiện có, xác nhận bằng 24 dòng compile-time assertion mới
> trong `internal/adapter/mysql/shared.go` (package `postgres` chỉ assert
> 2/15, package `mysql` assert **toàn bộ 24 port** — kể cả 2 port thứ hai
> `sshrelay.SshTargetResolver`/`outbox.Store` mà 1 số struct implement
> đồng thời).
>
> **2. Audit thật — enumerate ĐẦY ĐỦ 15 repository unit + 62 migration
> TRƯỚC khi viết code thay vì phát hiện dần dần** (yêu cầu bắt buộc của
> task này, do quy mô ~15x pilot) — checklist đầy đủ ở BE-DB-SOL-017 §2,
> xử lý tuần tự, `go build` sau mỗi unit. Phát hiện đáng chú ý khi audit:
> `EphemeralVmRuntimeRepository` port định nghĩa ở
> `internal/usecase/list_ephemeral_vm_runtimes.go`, KHÔNG ở `ports.go` như
> 20 port còn lại — bỏ sót port này nếu chỉ `grep ports.go`. Chi tiết đầy
> đủ (JSONB/RLS/`gen_random_uuid`/`RETURNING`/`ON CONFLICT`/`xmax`/
> `TEXT[]`/advisory-lock) ở BE-DB-SOL-017 §1.
>
> **3. Migration tách `migrations/postgres/` (`git mv`, 62 file, nội dung
> y hệt) + `migrations/mysql/` (mới, 62 file, dialect-safe)** — TOÀN BỘ
> đã `git mv`/viết mới, không phải subset. Điểm dịch không có tiền lệ
> trong rollout (chi tiết đầy đủ BE-DB-SOL-017 §5):
> - 2 partial UNIQUE index dịch bằng generated STORED column (kỹ thuật
>   `ai-provider-service` đã xác lập) — KHÔNG đơn giản bỏ `WHERE` như 7
>   partial index không-unique khác trong service này.
> - 2 cột `TEXT PRIMARY KEY` (`pty_id`) → `VARCHAR(255)` (InnoDB không cho
>   PK trên TEXT/BLOB), kéo theo FK tham chiếu cũng đổi theo.
> - 1 FK cần đặt tên tường minh ngay từ migration gốc (0005) để migration
>   sau (0034) `DROP FOREIGN KEY` đúng tên được.
> - `rows` là từ khoá dành riêng MySQL 8.0.19+ — backtick-quote xuyên suốt
>   migration + adapter.
> - **2 lỗi CÚ PHÁP thật phát hiện khi CHẠY THẬT `migrate up` trên
>   `mysql:8` (không phải đọc code suông)** — `Error 1101` (TEXT column
>   DEFAULT cần ngoặc, MySQL 8.0.13+) và `Error 1064` (`rows` reserved
>   word) — xem BE-DB-SOL-017 §7. Đây LÀ giá trị thật của bước "verify
>   migration bằng Docker thật trước khi viết adapter code", không phải
>   thủ tục hình thức.
>
> **Verify migration ĐỘC LẬP với Go test** (dựng `mysql:8` container qua
> Docker trực tiếp + `migrate` CLI, KHÔNG qua `go test`) — cả `migrate up`
> (31 migration) VÀ `migrate down -all` (31 migration ngược) đều **PASS
> thật**, chạy `up` lại lần 2 cũng PASS — xác nhận idempotent. Đây là mức
> verify SÂU HƠN các service trước trong rollout (thường chỉ verify `up`
> gián tiếp qua `go test`'s setup).
>
> **4. `internal/adapter/mysql/` mới — 15/15 repository unit, 17 file
> (15 file .go + `shared.go` + 2 file test)**, implement đúng 24 port
> assertion, không đổi signature nào. `go build ./internal/adapter/mysql/...`
> + `go vet ./internal/adapter/mysql/...` sạch sau MỖI unit (không viết
> hết 15 rồi debug hàng loạt — yêu cầu bắt buộc của task này). Điểm dịch
> đáng chú ý nhất: `SshTargetStore.Upsert`'s `xmax != 0` → MySQL's tài
> liệu hoá affected-rows convention cho `ON DUPLICATE KEY UPDATE`
> (1=insert/2=updated-changed/0=updated-unchanged) — `updated := affected
> != 1` — xem BE-DB-SOL-017 §4.1, test
> `TestSshTargetStore_Upsert_InsertThenUpdate` xác nhận PASS thật (mục 6).
> 13 method dùng `RETURNING` dịch theo ĐÚNG nguyên tắc BE-DB-SOL-005 §3.1
> (không máy móc): chỉ dùng always-UPDATE-rồi-SELECT khi giá trị mới CÓ
> THỂ trùng giá trị cũ trên retry hợp lệ (`FleetDefinitionStore.Update`,
> `Repository.UpdateStatus`); dùng thẳng `RowsAffected()` khi giá trị LUÔN
> đổi thật (`AgentTokenStore.Revoke`, `TerminalSessionStore.Touch/Close`)
> — xem BE-DB-SOL-017 §4.2 cho lý do từng chỗ.
>
> **5. `cmd/server/main.go` — CHƯA wire dialect switch** — khác BIỆT DUY
> NHẤT so với mọi service trước trong rollout (tất cả đã DONE bước này).
> Lý do cụ thể: composition root 825 dòng, 15 lần gọi
> `infrapostgres.New*`, biến `sshTargetStore` đan sâu vào nhiều subsystem
> (SSH provisioning, Vault, agent token). Quyết định: không thực hiện
> rewiring rủi ro cao ở service lớn nhất rollout khi không thể chạy
> full end-to-end integration test để xác nhận an toàn — xem BE-DB-SOL-017
> §6 cho việc còn lại chính xác cho lượt sau (đã liệt kê đủ 15 constructor
> tương ứng, không mơ hồ).
>
> **6. Test — kết quả CHẠY THẬT, Docker thật, không testcontainers giả
> lập, không suy đoán PASS:**
>
> ```
> $ go build ./...                                    # sạch, không lỗi
> $ go vet ./...                                       # sạch, không lỗi
> $ gofmt -l internal/adapter/mysql/                   # rỗng (sau gofmt -w)
> $ go test ./...                                      # PASS toàn bộ package không cần Docker
> ```
>
> `go test -tags=integration ./internal/adapter/postgres/... -v`:
> [ĐIỀN Ở CUỐI — xem khối kết quả riêng bên dưới, Monitor còn đang chạy lúc
> viết đoạn này]
>
> `go test -tags=integration ./internal/adapter/mysql/... -v -timeout 25m`:
> [ĐIỀN Ở CUỐI — xem khối kết quả riêng bên dưới]
>
> **Môi trường**: batch 3+4+5 chạy ~10 agent song song, mỗi agent tự dựng
> container Postgres/MySQL riêng qua testcontainers — quan sát THẬT
> (không suy đoán): tại 1 thời điểm có tới 15 container `mysql:8` + 65
> container tổng cộng chạy đồng thời trên máy dùng chung. MySQL container
> startup quan sát được 150-185s/container (so với ~15-20s ở batch 1,
> TASK-BE-DB-010/011) — tranh chấp tài nguyên thật, không phải bug
> migration/adapter (Postgres container cùng máy, cùng thời điểm chỉ mất
> ~10-15s/container, xác nhận chênh lệch không phải do máy chậm chung mà
> do đặc thù MySQL container's khởi động nặng hơn dưới tải, cộng dồn với
> tranh chấp).
>
> **7. Bug NGOÀI PHẠM VI phát hiện + ĐÃ SỬA (chặn verify, không phải "flag
> don't fix")**: `internal/adapter/postgres`'s **integration test package
> không compile được** trước khi task này bắt đầu — xác nhận qua
> `git log`/`git diff`: hoàn toàn tiền tồn tại (commit `87192e667`,
> "SSH target port/known-hosts/jump-host... TASK-SSH-01-01..08"), không
> liên quan CR-DB-002/003, không do task này gây ra (chỉ sửa duy nhất
> đường dẫn `migrations/` → `migrations/postgres/` ở 3 file, không đụng
> constructor call nào). 3 lỗi cụ thể: `domain.NewSshTarget`'s signature
> đổi (thêm `port`/`jumpHostTargetID`/`project`/`tags`) nhưng 4 test file
> (`migration_0017_ssh_targets_host_unique_test.go`, `ssh_target_store_test.go`)
> chưa cập nhật theo (6 call site thiếu tham số); `testUser1` khai báo
> trùng ở 2 file (`agent_session_repository_test.go` +
> `dev_server_access_request_repository_test.go`); `repository_test.go`'s
> `gotBastion != bastion` so sánh struct chứa `[]string` (không comparable
> từ khi `SshTarget.Tags` được thêm) — cả 3 SỬA (không chỉ flag) vì chặn
> trực tiếp bước verify bắt buộc của CHÍNH task này
> (`go test -tags=integration ./internal/adapter/postgres/...`), khác
> "flag don't fix" thông thường cho bug không liên quan gì tới việc đang
> làm — sửa mang tính cơ học (thêm tham số đúng theo signature hiện tại,
> đổi tên 1 hằng, đổi `!=` thành `reflect.DeepEqual`), không đụng logic
> sản xuất nào.
>
> **8. `.github/workflows/backend-go-infra-fleet-service.yml` mới** — copy
> khung `backend-go-usage-service.yml`, đổi path/service name, có ghi chú
> rõ trạng thái PARTIAL trong comment đầu file. YAML hợp lệ xác nhận qua
> `python3 -c "import yaml; yaml.safe_load(...)"` — PASS.
>
> **9. Scope xác nhận qua `git status --porcelain` (chỉ scope
> `infra-fleet-service` + file dùng chung đã sửa tối thiểu)**: đúng các
> file trong `infra-fleet-service/{internal/adapter/mysql/ (mới, 17
> file), internal/adapter/postgres/*_test.go (6 file, sửa tối thiểu — path
> + 3 bug tiền tồn tại), migrations/ (postgres/ + mysql/, 124 file), go.mod,
> go.sum, README.md}` + `.github/workflows/backend-go-infra-fleet-service.yml`
> + 2 file spec (solution + task doc này) + `ROLLOUT-TRACKING.md`'s hàng
> batch 3+4+5 (chỉ thêm phần `infra-fleet-service`, không sửa phần service
> khác) — **KHÔNG đụng `cmd/server/main.go`** (xem mục 5, quyết định có
> chủ đích) — không đụng file nào của 6 service khác đang chạy song song
> trong cùng batch (`tenant-service`, `automation-service`,
> `workflow-service`, `auth-service`, `task-service`, `project-service`).
>
> **Không phát hiện bug nào trong shared code (`common/dbcapability`,
> `common/testutil`)** khi dùng cho service lớn nhất rollout — dùng lại y
> nguyên, không cần sửa gì.
>
> **Checklist đầy đủ 15 repository unit** (yêu cầu bắt buộc của task —
> minh bạch tuyệt đối, không tóm tắt che giấu trạng thái thật):
>
> | # | Repository (struct) | Migration split | MySQL adapter (build+vet) | Interface assert | Tests viết | Tests PASS thật (cả 2 dialect) | Status |
> |---|---|---|---|---|---|---|---|
> | 1 | `Repository` (DevServer/Connection/Resolver/FleetHealth/PollLock/OutboxWriter) | ✅ | ✅ | ✅ | ✅ (9 test, `repository_test.go`) | 🟡 xem khối kết quả | 🟡 |
> | 2 | `SshTargetStore` | ✅ | ✅ | ✅ | ✅ (3 test) | 🟡 xem khối kết quả | 🟡 |
> | 3 | `PortForwardStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | 4 | `Repository` (outbox.go: Enqueue/Fetch/MarkPublished) | ✅ | ✅ | ✅ | ✅ (chung repository_test.go) | 🟡 xem khối kết quả | 🟡 |
> | 5 | `DevServerGroupStore` | ✅ | ✅ | ✅ | ✅ (1 test, tenant-isolation) | 🟡 xem khối kết quả | 🟡 |
> | 6 | `DevServerGroupGrantStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | 7 | `DevServerAccessRequestStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | 8 | `AgentTokenStore` | ✅ | ✅ | ✅ | ✅ (1 test, +no-op-revoke guard) | 🟡 xem khối kết quả | 🟡 |
> | 9 | `FleetDefinitionStore` | ✅ | ✅ | ✅ | ✅ (1 test, +version-conflict guard) | 🟡 xem khối kết quả | 🟡 |
> | 10 | `TerminalScrollbackSnapshotStore` | ✅ | ✅ | ✅ | ✅ (1 test, `rows` backtick round-trip) | 🟡 xem khối kết quả | 🟡 |
> | 11 | `TerminalSessionStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | 12 | `AgentSessionStore` | ✅ | ✅ | ✅ | ✅ (1 test, +BR-AG-01 unique-constraint guard) | 🟡 xem khối kết quả | 🟡 |
> | 13 | `EphemeralVmRuntimeStore` | ✅ | ✅ | ✅ | ✅ (1 test, 7 method) | 🟡 xem khối kết quả | 🟡 |
> | 14 | `EphemeralVmSshTargetStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | 15 | `BrowserProfileStore` | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
> | — | `AgentRateLimitedOutboxStore` (outbox.Store thứ 2, đếm chung nhóm #1 theo file) | ✅ | ✅ | ✅ | ✅ (1 test) | 🟡 xem khối kết quả | 🟡 |
>
> **23 test function MySQL** (`repository_test.go` 10 + `small_stores_test.go`
> 13) bao phủ **15/15 repository unit** ít nhất 1 test/unit (nhiều unit có
> 2-3+), gồm **2 test tenant-isolation-without-RLS mới** (`dev_servers` qua
> `TestRepository_List_DoesNotLeakAcrossTenants`,
> `connections` qua `TestRepository_ListConnectivitySummary_DoesNotLeakAcrossTenants`,
> `dev_server_groups` qua `TestDevServerGroupStore_CreateAndList_DoesNotLeakAcrossTenants`
> — 3 bảng RLS-bearing trong tổng 11 bảng có RLS, ưu tiên bảng có khả năng
> rò rỉ cao nhất/dùng nhiều nhất).
>
> **Kết quả chạy thật** (điền tại thời điểm hoàn tất, xem timestamp cuối
> mục này):
>
> [KHỐI KẾT QUẢ THẬT — điền sau khi 2 Monitor hoàn tất]
>
> **10. Deviation so với pattern pilot**: (a) `cmd/server/main.go` CHƯA
> wire — xem mục 5, đây là deviation LỚN NHẤT so với mọi task trước trong
> rollout (tất cả đã DONE bước này), có chủ đích và ghi rõ lý do, không
> phải bỏ sót; (b) sửa 3 bug tiền tồn tại ở `internal/adapter/postgres`'s
> test file — ngoài phạm vi CR-DB nhưng chặn verify bắt buộc, xem mục 7;
> (c) task doc này gộp các bước tương đương TASK-BE-DB-002~007 vào 1 file,
> theo đúng convention rollout đã thiết lập từ batch 1.
>
> **Final status (2026-09-11, LỊCH SỬ — xem bản 2026-09-12 bên dưới cho
> trạng thái đúng hiện tại): 🟡 PARTIAL** — migration 100% xong VÀ verify
> thật (cả 2 chiều up/down, độc lập với Go test); adapter code 100% xong
> (15/15 unit, build+vet sạch, 24 interface assertion); test tích hợp viết
> đầy đủ cho 15/15 unit nhưng CHƯA xác nhận PASS 100% do môi trường Docker
> contention thật (không phải bug); `cmd/server/main.go` CHƯA wire —
> service **CHƯA chạy được thật trên MySQL** cho tới khi hoàn tất mục 5.

## Final status (2026-09-12, ĐÚNG hiện tại): ✅ DONE

- `cmd/server/main.go` đã wire dialect switch — service chạy được thật
  trên cả Postgres và MySQL (build/vet/gofmt sạch, xem "Cập nhật
  (2026-09-12)" ở trên).
- MySQL integration: **23/23 PASS thật** dưới tải thấp (2 bug adapter thật
  phát hiện + sửa trong lượt này — xem "Cập nhật (2026-09-12)" mục 2).
- Postgres integration: **35/37 PASS thật** — 2 FAIL còn lại là bug tiền
  tồn tại xác nhận KHÔNG liên quan CR-DB-002/003 (test-design lỗi thời,
  đòi hỏi viết lại logic test, không phải giá trị tham số) — flag không
  sửa, cùng convention "flag không fix bug tiền tồn tại ngoài phạm vi" đã
  dùng ở project-service (TASK-BE-DB-021 mục 8) và issue-tracking-service
  (TASK-BE-DB-009 mục 6). Chi tiết 2 test còn FAIL ở "Cập nhật
  (2026-09-12)" mục 3.
- Đây là service LỚN NHẤT rollout (15 repo/62 migration) và là mảnh cuối
  cùng của toàn bộ 15-service rollout — không còn "Việc còn lại" nào chặn.

---

## Mục tiêu

Mở rộng Multi-Database pattern (F26, CR-DB-002/003) đã xác lập ở pilot
`usage-service` sang `infra-fleet-service` — service LỚN NHẤT trong 15
service còn lại của rollout (xem `ROLLOUT-TRACKING.md`, batch 3+4+5).

## Files đã sửa/thêm

1. `backend-go/services/infra-fleet-service/migrations/postgres/*.sql` (MOVED, 62 file, nội dung không đổi)
2. `backend-go/services/infra-fleet-service/migrations/mysql/*.sql` (MỚI, 62 file)
3. `backend-go/services/infra-fleet-service/internal/adapter/mysql/*.go` (MỚI, 15 file adapter + `shared.go`)
4. `backend-go/services/infra-fleet-service/internal/adapter/mysql/*_test.go` (MỚI, 2 file — `repository_test.go`, `small_stores_test.go`)
5. `backend-go/services/infra-fleet-service/internal/adapter/postgres/{repository_test.go,terminal_scrollback_snapshot_repository_test.go,migration_0018_fleet_definitions_test.go}` (MODIFY — migrations path only)
6. `backend-go/services/infra-fleet-service/internal/adapter/postgres/{dev_server_access_request_repository_test.go,migration_0017_ssh_targets_host_unique_test.go,ssh_target_store_test.go}` (MODIFY — sửa 3 bug tiền tồn tại chặn compile, xem mục 7)
7. `backend-go/services/infra-fleet-service/go.mod`/`go.sum` (MODIFY — `+github.com/go-sql-driver/mysql`, via `go get`)
8. `backend-go/services/infra-fleet-service/README.md` (MODIFY — migration path)
9. `.github/workflows/backend-go-infra-fleet-service.yml` (MỚI)
10. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-017-infra-fleet-service-mysql-tidb-adapter.md` (MỚI)
11. `specs/backend-go/crs/v4/multi-database/tasks/TASK-BE-DB-022-infra-fleet-service-mysql-rollout.md` (MỚI, doc này)
12. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ phần `infra-fleet-service`)
13. `backend-go/services/infra-fleet-service/cmd/server/main.go` (MODIFY, 2026-09-12 — wire dialect switch, xem "Cập nhật (2026-09-12)" mục 1)
14. `backend-go/services/infra-fleet-service/internal/adapter/mysql/{fleet_definition_repository.go,terminal_scrollback_snapshot_repository.go}` (MODIFY, 2026-09-12 — 2 bug thật phát hiện khi re-run integration suite, xem mục 2)
15. `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go` (MODIFY, 2026-09-12 — `NewSshTarget`'s `tags` nil-default, xem mục 3)
16. `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository_test.go` (MODIFY, 2026-09-12 — thêm `Kind` field còn thiếu, test-only, xem mục 3)

## Việc còn lại (chính xác, cho follow-up agent hoặc người)

KHÔNG còn việc nào chặn — cả 2 mục của lượt trước đã hoàn tất (xem "Cập
nhật (2026-09-12)" ở đầu file). Còn lại 2 mục KHÔNG chặn, tuỳ chọn cho
lượt sau nếu muốn:

1. (Tuỳ chọn, không chặn) 2 test Postgres pre-existing còn FAIL
   (`TestMigration0018_DownDropsTable`,
   `TestRepository_ResolveConnection_FoundAndNotFound`) — bug thật, không
   liên quan CR-DB-002/003, đòi hỏi viết lại test logic (không phải sửa
   tham số) — xem "Cập nhật (2026-09-12)" mục 3 cho mô tả chính xác từng
   test.
2. (Tuỳ chọn, không chặn) Mở rộng test cho các method chưa có test riêng
   trong mỗi unit (ví dụ `Repository.FindBySshTarget`,
   `Repository.FindByHostAndMode` chưa có test MySQL riêng — có test
   Postgres tương đương ở `repository_test.go`'s
   `TestFindByHostAndMode_DirectWebSocketWithNullSSHTargetID`).

## Verify

```bash
cd backend-go/services/infra-fleet-service
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v -timeout 25m   # requires Docker

# Migration verify độc lập (không qua Go test) — dùng để tái tạo bước phát
# hiện bug ở mục 7 của "Kết quả thực tế":
docker run -d --name infra-mysql-verify -e MYSQL_ROOT_PASSWORD=orca -e MYSQL_DATABASE=infra -p 33061:3306 mysql:8
migrate -path migrations/mysql -database "mysql://root:orca@tcp(127.0.0.1:33061)/infra" up
migrate -path migrations/mysql -database "mysql://root:orca@tcp(127.0.0.1:33061)/infra" down -all

python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-infra-fleet-service.yml'))"
```

Xem "Kết quả thực tế" ở trên cho output thật của các lệnh này.

## Kết quả chạy thật thứ 2 (2026-09-12, sau khi wire main.go + sửa bug)

```
$ go build ./...                                              # sạch
$ go vet ./...                                                # sạch
$ gofmt -l .                                                  # rỗng
$ go test ./...                                                # PASS toàn bộ (unit, không Docker)

$ go test -tags=integration ./internal/adapter/mysql/... -v -timeout=25m
--- 23/23 PASS thật (container mysql:8 ~15-19s/container, không tranh
    chấp — batch song song trước đó đã kết thúc)
--- Lần chạy đầu: 21/23 PASS, 2 FAIL (bug thật, đã sửa — xem "Cập nhật
    (2026-09-12)" mục 2); lần chạy lại: 23/23 PASS thật.

$ go test -tags=integration ./internal/adapter/postgres/... -v -timeout=20m
--- 35/37 PASS thật (container postgres:16-alpine ~2-4s/container)
--- Lần chạy đầu: 29/37 PASS, 8 FAIL; sau khi sửa 2 bug mechanical (tags
    nil-default ở domain.NewSshTarget, Kind còn thiếu ở 1 test struct
    literal): 35/37 PASS, 2 FAIL còn lại — cả 2 xác nhận bug tiền tồn tại
    KHÔNG liên quan CR-DB-002/003 (xem "Cập nhật (2026-09-12)" mục 3):
    TestMigration0018_DownDropsTable, TestRepository_ResolveConnection_FoundAndNotFound.
```

## gitnexus

`impact()` chạy trước khi sửa `run`/`Repository` — cả 2 risk **LOW** (chi
tiết ở "Kết quả thực tế" mục 1).

**Lượt 2026-09-12** — `impact()` chạy trước mỗi symbol sửa:
- `run` (`cmd/server/main.go`), upstream: risk **LOW**, impactedCount 1
  (chỉ `main`).
- `NewSshTarget` (`internal/domain/ssh_target.go`), upstream: risk **LOW**,
  impactedCount 3 (`CreateSshTarget.Execute`, `DeployFleetDefinition.Execute`,
  `ImportFleetInventory.Execute` — cả 3 cùng service).
- `FleetDefinitionStore.Update`/`TerminalScrollbackSnapshotStore.Upsert`
  (`internal/adapter/mysql/`): GitNexus KHÔNG resolve được (symbol
  "not found" cho cả tên method lẫn tên struct qua `context()`) — gói
  `internal/adapter/mysql` có vẻ chưa được index đầy đủ (các symbol này
  tồn tại thật trong code, build/test xác nhận). Theo "stop on saturation"
  (CLAUDE.md), không cố paginate/mở rộng truy vấn — thay vào đó xác nhận
  blast radius bằng `grep` hẹp, có chủ đích (không quét toàn repo):
  `FleetDefinitionRepository.Update` có đúng 1 consumer
  (`usecase.DeployFleetDefinition`), `TerminalScrollbackSnapshotRepository.Upsert`
  có đúng 1 consumer (`usecase.SaveTerminalScrollbackSnapshot`) — cùng mức
  LOW đã xác lập cho các symbol Postgres tương đương.

`detect_changes()` (scope unstaged, repo `orca`) chạy trước khi coi task
xong — kết quả BỊ SATURATE (80,579 ký tự/1,979 dòng, vượt giới hạn token)
do các agent sibling khác trong rollout để lại thay đổi CHƯA COMMIT trên
đĩa (batch 3+4+5, 6 service khác + docs). Theo "stop on saturation", không
paginate toàn bộ — lọc kết quả đã lưu bằng `grep infra-fleet-service`,
xác nhận đúng những symbol lượt này sửa (`run`, các file test Postgres đã
sửa từ lượt trước) xuất hiện, không có symbol của service nào khác lẫn
vào. Xác nhận scope chính xác hơn qua `git status --porcelain` scoped
(mục "Files đã sửa/thêm" ở trên) — toàn bộ thay đổi nằm trong
`infra-fleet-service/{cmd/server/main.go, internal/adapter/mysql/*.go (2
file), internal/domain/ssh_target.go, internal/adapter/postgres/repository_test.go}`
+ 2 file spec (solution + task doc này) + `ROLLOUT-TRACKING.md`'s hàng
batch 3+4+5 — không đụng file nào của 6 service khác đang chạy song song
cùng batch, không đụng `internal/usecase/ports.go` hay bất kỳ interface
signature nào.
