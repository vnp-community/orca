# TASK-BE-DB-019: Multi-database rollout cho `auth-service`

**Solution:** [BE-DB-SOL-014](../solutions/BE-DB-SOL-014-auth-service-mysql-tidb-adapter.md) | **CR:** CR-DB-002, CR-DB-003
**Service:** `auth-service`
**Depends on:** TASK-BE-DB-002 (`dbcapability`, dùng lại nguyên vẹn), pattern TASK-BE-DB-004~007 (`usage-service` pilot), tham chiếu gần nhất BE-DB-SOL-006 (`credential-broker-service`, service nhạy cảm bảo mật) + BE-DB-SOL-005 (`annotation-service`, pitfall `RowsAffected()`)
**Status:** ✅ DONE (2026-09-11)

> **Kết quả thực tế:**
>
> **1. `impact()` chạy thật trước khi sửa** (bắt buộc theo AGENTS.md/CLAUDE.md,
> không suy đoán), repo `orca`, `direction: upstream`:
>
> | Symbol | Risk | impactedCount | Ghi chú |
> |---|---|---|---|
> | `run` (`cmd/server/main.go`, Function) | LOW | 1 | chỉ `main` gọi |
> | `UserRepository` (Interface) | MEDIUM | 13 | fan-out IMPORTS toàn bộ file cùng package, không phải caller thật bị phá — signature không đổi |
> | `SessionRepository` (Interface) | MEDIUM | 13 | như trên |
> | `ServiceTokenRepository` (Interface) | MEDIUM | 13 | như trên |
> | `AccessPolicyRepository` (Interface) | MEDIUM | 13 | như trên |
> | `AuditRepository` (Interface) | MEDIUM | 16 | như trên + 2 hit cross-service vào `credential-broker-service` (`resolveMetadata`/`ResolveCredential.Execute`/`ResolveCredentialByOwner.Execute` qua quan hệ `ACCESSES`) — xác nhận đây là trùng tên method `Query`/đồng dạng, KHÔNG phải phụ thuộc thật giữa 2 service (2 service độc lập database-per-service, không import lẫn nhau) — không hành động gì, chỉ ghi nhận false-positive graph edge |
> | `SsoIdentityRepository` (Interface) | MEDIUM | 13 | như trên |
> | `SsoGroupRoleMappingRepository` (Interface) | MEDIUM | 13 | như trên |
> | `PairingSessionRepository` (Interface) | MEDIUM | 13 | như trên |
> | `PairedDeviceRepository` (Interface) | MEDIUM | 13 | như trên |
>
> Không HIGH/CRITICAL nào ở cả 9 interface — an toàn tiếp tục theo đúng
> ngưỡng CLAUDE.md quy định. Rủi ro MEDIUM ở cả 9 chỉ phản ánh số file
> trong cùng package `internal/usecase`/`internal/adapter/*` IMPORT
> `ports.go` (13-16 file), KHÔNG phải rủi ro thật vì không interface nào
> bị đổi signature — mọi implementation mới (MySQL) chỉ THÊM method set
> giống hệt, không sửa/xoá gì ở `ports.go` hay ở bất kỳ file Postgres nào.
>
> **2. Audit đầy đủ trước khi viết code** — đếm lại chính xác: **9 interface
> SQL-backed** (không phải 10 như `ROLLOUT-TRACKING.md`'s con số đếm FILE
> thô — `repository.go` chỉ là struct `Repository{pool}` + `New()`, không
> phải 1 interface riêng), 10 file adapter Postgres, 20 file migration (10
> cặp up/down). Chi tiết đầy đủ ở BE-DB-SOL-014 §1.
>
> **3. Migration split** — `git mv` 20 file nguyên văn vào
> `migrations/postgres/`. `migrations/mysql/` mới, 20 file dialect-safe:
> `UUID`→`CHAR(36)`, hash cố định độ dài (`token_hash`,
> `pairing_sessions.id`, `issued_service_tokens.jti`,
> `refresh_token_hash`)→`VARCHAR(N)` theo độ dài THẬT đã xác nhận qua audit
> code (SHA-256 hex 64 ký tự / base64url(32 byte) 43 ký tự), `TIMESTAMPTZ`→
> `TIMESTAMP(6)`, `INET`→`VARCHAR(45)`, `BYTEA`→`BLOB`, `JSONB`→`JSON`,
> `BRIN`→B-tree thường, partial index→index đầy đủ (2 chỗ:
> `paired_devices`'s `WHERE status='active'`,
> `sessions.refresh_token_hash`'s `WHERE ... IS NOT NULL` — chỗ thứ 2
> thực ra không cần workaround gì vì MySQL/InnoDB's UNIQUE index đã coi
> mỗi NULL riêng biệt sẵn, giống ngữ nghĩa Postgres), RLS bỏ + comment
> giải thích trên cả 5 bảng có RLS thật (`users`, `sessions`, `audit_log`,
> `sso_group_role_mapping`, `sso_identities` — nhiều nhất trong rollout
> tới nay). `ListLatestPolicies`'s `DISTINCT ON` không có tương đương MySQL
> → dịch sang `ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC)`
> (MySQL 8.0+/TiDB window function — LẦN ĐẦU pattern này xuất hiện trong
> rollout). Chi tiết đầy đủ BE-DB-SOL-014 §5.
>
> **4. `internal/adapter/mysql/` — 10 file mới**, implement lại đúng cả 9
> interface, KHÔNG đổi signature nào. Phát hiện quan trọng nhất (khác mọi
> service trước, vì đây là service đầu tiên trong rollout có nhiều đường
> revoke bảo mật): **3 method có pitfall `RowsAffected()==0`-là-not-found
> kiểu BE-DB-SOL-005 §3.1, cả 3 đều trên đường bảo mật** —
> `SessionRepository.RevokeSession`, `ServiceTokenRepository.Revoke`
> (CLI-token revocation), `PairedDeviceRepository.RevokeAndWipeSecret`
> (BR-MB-04). Một retry hợp lệ (revoke 2 lần cùng giá trị — network retry,
> hoặc double-click UI) trên MySQL mặc định sẽ báo sai
> `ErrSessionNotFound`/`ErrServiceTokenNotFound`/`ErrDeviceNotFound` dù
> dòng chắc chắn tồn tại — sửa bằng UPDATE vô điều kiện + SELECT lại xác
> nhận tồn tại, không dựa `RowsAffected()`. Cộng 2 method dịch từ
> `RETURNING` sang UPDATE-rồi-SELECT-lại (`UpdateUserRole`, `UpdateUser`).
> Ngược lại, `PairingSessionRepository.GetAndConsume` XÁC NHẬN AN TOÀN dùng
> `RowsAffected()==0` (giữ nguyên, không sửa) vì `consumed_at` chỉ
> transition NULL→non-NULL đúng 1 lần, không có ca no-op — ghi rõ trong
> code tại sao đây KHÔNG phải một trường hợp bị bỏ sót. Chi tiết đầy đủ
> BE-DB-SOL-014 §3.
>
> **5. Wiring `cmd/server/main.go`** — khác mọi service rollout trước:
> `DatabaseCredentialsFile`/`secrets.DatabaseCredentialsFromFile` ĐÃ có sẵn
> từ trước (không phải thêm mới) — chỉ chèn
> `dbcapability.DetectDialectFromDSN` + `switch caps.Dialect` bọc quanh
> `pgxpool.New` hiện có. `repo` khai báo qua 1 interface ẩn danh gộp 7 port
> (`UserRepository` qua `SsoGroupRoleMappingRepository`, implement bởi
> cùng 1 struct `Repository` ở cả 2 dialect); `pairingSessions`/
> `pairedDevices` qua 2 interface port riêng
> (`PairingSessionRepository`/`PairedDeviceRepository`), gán trong cùng
> switch. `healthSrv := health.New()` di chuyển lên trước switch (dùng ở
> cả 2 nhánh, xoá đăng ký `"postgres"` cứng riêng lẻ trước đó).
> `toMySQLDriverDSN` copy nguyên văn từ `usage-service`. `go build ./...`
> + `go vet ./...` sạch ngay sau khi wiring xong (xem mục 7).
>
> **6. Test MySQL adapter** — 3 file mới build tag `integration`:
> `repository_test.go` (setup helper `setupMySQLDB`/`setupRepository` +
> round-trip cho User/Session/ServiceToken/AccessPolicy/Audit/SsoIdentity),
> `pairing_repository_test.go` (PairingSessionStore/PairedDeviceStore —
> **Postgres KHÔNG có integration test nào cho 2 port này, xác nhận qua
> `ls` trước khi viết, không giả định** — nên đây là coverage MỚI, không
> phải 1:1 port), `tenant_isolation_test.go` (TASK-BE-DB-003 pattern:
> `TestUserRepository_ListUsers_DoesNotLeakAcrossTenants`,
> `TestSessionRepository_ListForTenant_DoesNotLeakAcrossTenants`,
> `TestAuditRepository_Query_DoesNotLeakAcrossTenants`,
> `TestSsoGroupRoleMappingRepository_ListForProvider_DoesNotLeakAcrossTenants`
> — 4/5 bảng có RLS; `sso_identities` không có method tenant-scoped nào để
> viết test loại này, ghi nhận rõ trong file thay vì giả vờ có coverage)
> + regression test riêng cho cả 3 pitfall `RowsAffected()` ở mục 4
> (`TestUserRepository_UpdateUserRole_NoopRetryStillSucceeds`,
> `TestSessionRepository_RevokeSession_NoopRetryStillSucceeds`,
> `TestServiceTokenRepository_Revoke_NoopRetryStillSucceeds`,
> `TestPairedDeviceStore_RevokeAndWipeSecret_NoopRetryStillSucceeds`).
>
> **7. Build/test thật đã chạy** (không giả định pass):
> ```
> $ go build ./...                                          # sạch
> $ go vet ./...                                             # sạch
> $ go build -tags=integration ./...                         # sạch
> $ go vet -tags=integration ./...                           # sạch
> $ gofmt -l $(find . -name '*.go')                          # rỗng
> $ go test ./...                                             # PASS (domain, usecase, adapter/bcrypt,grpc,nacl,natsconsumer,oauth,oauthstate,policypublisher — cached, không đổi)
> ```
> `go test -tags=integration ./internal/adapter/postgres/... -v` (Docker +
> testcontainers thật, Postgres 16-alpine, xác nhận di chuyển thư mục
> migration không phá gì) → **12/12 PASS thật**, 177.7s tổng — bao gồm cả
> `TestRepository_AuditLog_QueryFiltersByActorActionOutcome`'s 5 subtest.
>
> `go test -tags=integration ./internal/adapter/mysql/... -v` (Docker +
> testcontainers thật, MySQL 8, `migrate` CLI thật chạy 10 file migration
> MySQL) → **23/23 PASS thật**, chạy foreground theo từng batch nhỏ.
>
> **8. `go mod tidy`** → thêm `github.com/go-sql-driver/mysql v1.10.1` vào
> `go.mod` như dependency **direct** (không `// indirect`).
>
> **9. `detect_changes()` chạy trước khi kết thúc** — môi trường có nhiều
> agent khác chạy song song trên các service khác của cùng đợt rollout
> gộp batch 3+4+5 (`tenant-service`, `automation-service`,
> `workflow-service`, `task-service`, `project-service`,
> `infra-fleet-service`) — kết quả lọc thủ công theo scope
> `backend-go/services/auth-service/` + 1 file CI workflow mới + 2 doc mới
> + 1 dòng `ROLLOUT-TRACKING.md`, xem chi tiết trong báo cáo cuối của
> agent.
>
> **Không có bug nào phát hiện ở `common/dbcapability`/`common/testutil`**
> trong quá trình này — dùng lại y nguyên, không sửa, đúng "flag don't
> fix" (không có gì cần flag).
>
> **Không đụng vào**: `internal/adapter/bcrypt` (băm mật khẩu),
> `internal/adapter/vault` (ký JWT qua Transit, seal/unseal shared secret),
> `internal/adapter/nacl` (NaCl box keypair), `internal/adapter/oauth*`
> (SSO token exchange) — đúng ràng buộc "chỉ retarget SQL storage
> mechanics, never security logic" của nhiệm vụ gốc.
>
> **Trạng thái cuối: ✅ DONE** — re-verify độc lập (agent này, không copy số
> cũ) ngày 2026-09-11: `go build ./...`/`go vet ./...` sạch (xác nhận lại
> trước khi chạy test). `go test -tags=integration ./internal/adapter/postgres/... -v`
> → **12/12 PASS thật**, 145.075s, không `--- FAIL` nào. `go test
> -tags=integration ./internal/adapter/mysql/... -v` → **23/23 PASS thật**
> (23 hàm test top-level xác nhận qua đếm trực tiếp `grep '^func Test'`,
> khớp đúng "23/23" ghi ở mục 7; không `t.Skip` nào trong 3 file test
> MySQL) — chạy đúng **2 lần độc lập** trong phiên này (570.982s và
> 869.805s do tranh chấp Docker/CPU với các agent sibling khác đang chạy
> song song test MySQL cho `automation-service`/`orchestration-service`
> cùng lúc), cả 2 lần đều `PASS`/`ok`, exit code 0, **0 dòng `--- FAIL`**
> trong cả 2 log. Không phải copy lại số "23/23" cũ — số này được tái tạo
> thật bằng lệnh `go test` chạy trực tiếp trong phiên xác minh này.

---

## Mục tiêu

Áp dụng đúng pattern BE-DB-SOL-001/002 (đã implement cho `usage-service`)
ra `auth-service` — service identity core, lớn nhất/nhạy cảm bảo mật nhất
đã rollout tới nay trong batch gộp 3+4+5 của `ROLLOUT-TRACKING.md`. Yêu
cầu đặc biệt: **không được làm yếu bất kỳ logic băm mật khẩu (bcrypt), ký
JWT (Vault Transit), hay sinh session-token nào** — chỉ retarget SQL
storage/retrieval mechanics.

## Files đã sửa/thêm

1. `backend-go/services/auth-service/migrations/postgres/000{1..10}_*.{up,down}.sql` (MOVE, `git mv`, nội dung không đổi)
2. `backend-go/services/auth-service/migrations/mysql/000{1..10}_*.{up,down}.sql` (MỚI, 20 file)
3. `backend-go/services/auth-service/internal/adapter/mysql/repository.go` (MỚI — struct + `New()` + `dbtx`/`rowScanner`)
4. `backend-go/services/auth-service/internal/adapter/mysql/user_repository.go` (MỚI)
5. `backend-go/services/auth-service/internal/adapter/mysql/session_repository.go` (MỚI)
6. `backend-go/services/auth-service/internal/adapter/mysql/service_token_repository.go` (MỚI)
7. `backend-go/services/auth-service/internal/adapter/mysql/access_policy_repository.go` (MỚI)
8. `backend-go/services/auth-service/internal/adapter/mysql/audit_repository.go` (MỚI)
9. `backend-go/services/auth-service/internal/adapter/mysql/sso_identity_repository.go` (MỚI)
10. `backend-go/services/auth-service/internal/adapter/mysql/sso_group_role_mapping_repository.go` (MỚI)
11. `backend-go/services/auth-service/internal/adapter/mysql/pairing_session_repository.go` (MỚI)
12. `backend-go/services/auth-service/internal/adapter/mysql/paired_device_repository.go` (MỚI)
13. `backend-go/services/auth-service/internal/adapter/mysql/repository_test.go` (MỚI, build tag `integration`)
14. `backend-go/services/auth-service/internal/adapter/mysql/pairing_repository_test.go` (MỚI, build tag `integration`)
15. `backend-go/services/auth-service/internal/adapter/mysql/tenant_isolation_test.go` (MỚI, build tag `integration`)
16. `backend-go/services/auth-service/internal/adapter/postgres/repository_test.go` (MODIFY — `filepath.Abs` path thêm `/postgres`)
17. `backend-go/services/auth-service/cmd/server/main.go` (MODIFY — `switch caps.Dialect` + `toMySQLDriverDSN`, `healthSrv` di chuyển lên trước switch)
18. `backend-go/services/auth-service/go.mod`/`go.sum` (MODIFY — `go mod tidy` thêm `go-sql-driver/mysql` direct)
19. `backend-go/services/auth-service/README.md` (MODIFY — migration path, hướng dẫn chạy MySQL)
20. `.github/workflows/backend-go-auth-service.yml` (MỚI)
21. `specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-014-auth-service-mysql-tidb-adapter.md` (MỚI)
22. `specs/backend-go/crs/v4/multi-database/tasks/ROLLOUT-TRACKING.md` (MODIFY — chỉ dòng `auth-service`)

## Verify

```bash
cd backend-go/services/auth-service
go build ./... && go vet ./...
go build -tags=integration ./... && go vet -tags=integration ./...
gofmt -l $(find . -name '*.go')
go test ./...
go test -tags=integration ./internal/adapter/postgres/... -v
go test -tags=integration ./internal/adapter/mysql/... -v
python3 -c "import yaml; yaml.safe_load(open('../../../.github/workflows/backend-go-auth-service.yml'))"
```

## gitnexus

`impact()` chạy trên `run` + tất cả 9 interface trước khi sửa — `run` LOW,
9 interface đều MEDIUM (fan-out import trong cùng package, không phải rủi
ro thật, không HIGH/CRITICAL nào — xem "Kết quả thực tế" mục 1 cho bảng
đầy đủ, bao gồm 1 false-positive cross-service edge trên `AuditRepository`
đã điều tra và loại trừ). `detect_changes()` chạy trước khi coi task DONE
— xem "Kết quả thực tế" mục 9.
