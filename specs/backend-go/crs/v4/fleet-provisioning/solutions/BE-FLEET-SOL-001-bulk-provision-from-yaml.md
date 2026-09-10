# BE-FLEET-SOL-001: `BulkProvisionFleet` — bulk-register N host từ 1 fleet YAML

> **🔲 Designed — chưa implement.** Không phụ thuộc CR nào khác trong bộ F31 (làm trước).

**CR:** [CR-FLEET-001](../../../../../../docs/crs/v4/fleet-provisioning/CR-FLEET-001-bulk-provision-from-yaml.md)
**Service:** `infra-fleet-service`
**TDD tham chiếu:** [`infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) §9 (Vault-only SSH invariant)

---

## 1. Re-verify code thật trước khi thiết kế

Đọc lại trực tiếp (không suy đoán từ CR) — mọi khẳng định của CR-FLEET-001 vẫn đúng tại
thời điểm viết solution này:

- `internal/usecase/register_dev_server.go:38` — `RegisterDevServer.Execute(ctx, RegisterDevServerInput)` nhận
  **1** input (`Host, Mode, SSHTargetID, Kind`), `tenantID` lấy qua `tenant.RequireTenantID(ctx)` bên trong
  Execute — không phải field request. Không có biến thể batch.
- `internal/usecase/create_ssh_target.go:32` — `CreateSshTarget.Execute(ctx, CreateSshTargetInput)` tương tự,
  1 input (`Host, UserName, VaultSSHRole`), cùng pattern `tenant.RequireTenantID(ctx)`.
- `internal/usecase/ports.go:108-117` — `SshTargetRepository` **chỉ có** `Create`, `Get`, `List`. Không
  `FindByHost`/`Upsert` — xác nhận đúng CR §5 ("Không có idempotency ở tầng SSH target").
- `internal/usecase/ports.go:21-47` — `DevServerRepository` đã có sẵn `FindBySshTarget` **và**
  `FindByHostAndMode(ctx, tenantID, host string, mode domain.ConnectionMode) (ds domain.DevServer, found bool, err error)`
  — CR trích dẫn đúng.
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` — `grep -c "^  rpc "` = **58** RPC hiện có, không RPC
  nào tên `Bulk*`/`Import*`/`Provision*` theo nghĩa N-host — khớp CR. `StreamVmProvision` (dòng 196) là tiền lệ
  server-streaming RPC đúng như CR trích.
- `frontend/src/shared/fleet-config-parser.ts` — `FleetServerSchema` (dòng 36) có `identityFile: z.string().optional()`
  (dòng 42), chưa có `vaultSshRole` — khớp CR.
- `impact()` chạy lại (2026-09-09, số liệu KHÔNG đổi so với CR-FLEET-001's bảng gốc):
  - `RegisterDevServer` (struct, `internal/usecase/register_dev_server.go`) upstream → **LOW**, impactedCount 3,
    direct 1 (`cmd/server/main.go`'s `run`).
  - `CreateSshTarget` (struct, `internal/usecase/create_ssh_target.go`) upstream → **LOW**, impactedCount 3,
    direct 1, cùng dạng.
  - `Provisioner` (struct, `internal/adapter/sshrelay/provisioner.go`) upstream → **HIGH**, impactedCount 4,
    modules `Sshrelay` (direct), `Backendrelaysshprovisioner`/`Usecase` (indirect) — solution này **không sửa**
    struct này, chỉ gọi gián tiếp qua `RegisterDevServer` đã có. Rủi ro HIGH chỉ áp dụng nếu 1 PR tương lai
    đụng trực tiếp vào `sshrelay.Provisioner` — nhắc lại cảnh báo của CR gốc.

**Kết luận:** CR-FLEET-001 mô tả đúng thực trạng, không có gì lệch cần điều chỉnh thiết kế. Solution này giữ
nguyên hướng tiếp cận của CR — chỉ cụ thể hoá thành code Go.

## 2. Domain — `FleetSpec`/`FleetSpecServer`

```go
// backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go
package usecase

type FleetSpecServer struct {
	Host         string
	UserName     string
	VaultSSHRole string // KHÔNG identityFile — Vault-only invariant, domain.NewSshTarget đã enforce
	Kind         domain.AgentKind
}

type FleetSpec struct {
	Version string
	Servers []FleetSpecServer
}

type BulkProvisionServerResult struct {
	Host        string
	Status      string // "SUCCEEDED" | "FAILED"
	DevServerID string
	Error       string
}

type BulkProvisionResult struct {
	Results []BulkProvisionServerResult
}
```

## 3. Usecase `BulkProvisionFleet` — bounded concurrency + compensating rollback

```go
type BulkProvisionFleet struct {
	createSshTarget   *CreateSshTarget
	registerDevServer *RegisterDevServer
	deleteSshTarget   *DeleteSshTarget // usecase mới, xem §4
	sshTargets        SshTargetRepository
	concurrencyDefault int // 5, mirror CR-003 (bản TS cũ)'s --concurrency mặc định
}

func NewBulkProvisionFleet(
	createSshTarget *CreateSshTarget,
	registerDevServer *RegisterDevServer,
	deleteSshTarget *DeleteSshTarget,
	sshTargets SshTargetRepository,
) *BulkProvisionFleet {
	return &BulkProvisionFleet{
		createSshTarget:    createSshTarget,
		registerDevServer:  registerDevServer,
		deleteSshTarget:    deleteSshTarget,
		sshTargets:         sshTargets,
		concurrencyDefault: 5,
	}
}

// Execute chạy N server với concurrency bị chặn (semaphore, không phải
// worker-pool cố định — mirror CR-003's `--concurrency N` flag). emit
// dùng để đẩy BulkProvisionFleetEvent ra RPC stream ngay khi từng server
// xong, KHÔNG đợi cả batch (theo đúng convention StreamVmProvision).
func (uc *BulkProvisionFleet) Execute(ctx context.Context, spec FleetSpec, concurrency int, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	if concurrency <= 0 {
		concurrency = uc.concurrencyDefault
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := BulkProvisionResult{Results: make([]BulkProvisionServerResult, 0, len(spec.Servers))}

	for _, server := range spec.Servers {
		server := server
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			r := uc.provisionOne(ctx, server)
			mu.Lock()
			result.Results = append(result.Results, r)
			mu.Unlock()
			emit(r)
		}()
	}
	wg.Wait()
	return result, nil
}

// provisionOne — 1 server là 1 đơn vị tất-cả-hoặc-không độc lập, đúng
// tinh thần CR-FLEET-001 §"Rollback semantics": không rollback server
// khác trong cùng batch nếu server này fail.
func (uc *BulkProvisionFleet) provisionOne(ctx context.Context, server FleetSpecServer) BulkProvisionServerResult {
	sshTarget, err := uc.createSshTarget.Execute(ctx, CreateSshTargetInput{
		Host:         server.Host,
		UserName:     server.UserName,
		VaultSSHRole: server.VaultSSHRole,
	})
	if err != nil {
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	devServer, err := uc.registerDevServer.Execute(ctx, RegisterDevServerInput{
		Host:        server.Host,
		Mode:        domain.ConnectionModeRelaySSH,
		SSHTargetID: sshTarget.ID,
		Kind:        server.Kind,
	})
	if err != nil {
		// Compensating action — SshTarget vừa tạo đã mồ côi, xoá lại để
		// tránh rác infra.ssh_targets không gắn dev_server nào (CR §"Rollback
		// semantics"). Lỗi xoá KHÔNG che lỗi gốc — log riêng, trả về lỗi
		// RegisterDevServer cho caller.
		if delErr := uc.deleteSshTarget.Execute(ctx, sshTarget.ID); delErr != nil {
			return BulkProvisionServerResult{
				Host: server.Host, Status: "FAILED",
				Error: fmt.Sprintf("register failed: %v; cleanup also failed: %v", err, delErr),
			}
		}
		return BulkProvisionServerResult{Host: server.Host, Status: "FAILED", Error: err.Error()}
	}

	return BulkProvisionServerResult{Host: server.Host, Status: "SUCCEEDED", DevServerID: devServer.ID}
}
```

**Vì sao không DB transaction xuyên 2 usecase:** `CreateSshTarget`/`RegisterDevServer` mỗi cái tự gọi
`repo.Create`/`repo.Register` và commit ngay — constructor chỉ nhận `repo`, không nhận `*sql.Tx` truyền qua
được. Thêm cơ chế transaction xuyên-usecase là thay đổi kiến trúc lớn hơn phạm vi CR này (đã ghi rõ trong CR
gốc) — dùng compensating action (Saga đơn giản) thay vì transaction.

## 4. `DeleteSshTarget` — usecase mới, dọn record mồ côi

```go
// backend-go/services/infra-fleet-service/internal/usecase/delete_ssh_target.go
type DeleteSshTarget struct {
	repo SshTargetRepository
}

func NewDeleteSshTarget(repo SshTargetRepository) *DeleteSshTarget {
	return &DeleteSshTarget{repo: repo}
}

func (uc *DeleteSshTarget) Execute(ctx context.Context, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.Delete(ctx, tenantID, id); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DELETE_SSH_TARGET_FAILED", "failed to delete ssh target", err)
	}
	return nil
}
```

`SshTargetRepository` (`ports.go:108`) thêm method mới:

```go
type SshTargetRepository interface {
	Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
	List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
	Delete(ctx context.Context, tenantID, id string) error // MỚI
}
```

Postgres adapter (`internal/adapter/postgres/repository.go`, cạnh `INSERT` hiện có dòng 203):

```go
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM infra.ssh_targets WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}
```

## 5. Idempotency — unique constraint (chọn hướng DB constraint, không `FindByHost` usecase-level)

**Quyết định thiết kế:** CR gốc để mở 2 hướng ("unique constraint hoặc `FindByHost`, chọn 1"). Solution này
chọn **unique constraint DB** vì: (a) đơn giản hơn — không cần sửa `SshTargetRepository`/usecase để thêm 1
lookup trước insert trong mọi call site; (b) an toàn hơn dưới concurrency — `BulkProvisionFleet` chạy N
goroutine song song, 1 check-rồi-insert ở tầng usecase có race window giữa 2 goroutine cùng host (dù thực tế
hiếm vì YAML thường không có host trùng trong cùng request) — DB constraint loại bỏ hoàn toàn race đó bằng 1
lỗi `unique_violation` xử lý được ở `provisionOne`.

**Số thứ tự migration đã verify thật (2026-09-09):** `ls backend-go/services/infra-fleet-service/migrations/`
xác nhận file mới nhất là `0016_ephemeral_vm_ssh_target_host_key.up/down.sql` — CR-FLEET-001's đề xuất
`0017` là **đúng**, không lệch (khác lần trước — CR ghi chú "verify lại", lần verify này xác nhận số liệu
CR đề xuất khớp thực tế tại thời điểm viết solution).

```sql
-- backend-go/services/infra-fleet-service/migrations/0017_ssh_targets_host_unique.up.sql
ALTER TABLE infra.ssh_targets ADD CONSTRAINT ssh_targets_tenant_host_unique UNIQUE (tenant_id, host);
```

```sql
-- 0017_ssh_targets_host_unique.down.sql
ALTER TABLE infra.ssh_targets DROP CONSTRAINT ssh_targets_tenant_host_unique;
```

`provisionOne` xử lý `unique_violation` (Postgres error code `23505`) như một dạng "đã tồn tại" —
KHÔNG coi là fail cứng cho batch, trả `Status: "SUCCEEDED"` với `DevServerID` rỗng và ghi chú
`already_exists` trong `Error` field (dùng field này cho cả lỗi lẫn ghi chú không phải lỗi — hoặc, nếu cần
phân biệt rõ ràng hơn khi task-hoá, thêm 1 field `AlreadyExists bool` — xem TASK tương ứng).

## 6. RPC `BulkProvisionFleet` — server-streaming, tái dùng convention `StreamVmProvision`

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto — thêm cạnh RegisterDevServer/CreateSshTarget
rpc BulkProvisionFleet(BulkProvisionFleetRequest) returns (stream BulkProvisionFleetEvent);

message FleetSpecServerProto {
  string host = 1;
  string user_name = 2;
  string vault_ssh_role = 3;
  string kind = 4; // domain.AgentKind — string trên wire, mirror RegisterDevServerRequest's kind field hiện có
}

message BulkProvisionFleetRequest {
  string tenant_id = 1; // KHÔNG dùng để lấy tenantID thật — chỉ tenant.RequireTenantID(ctx) mới hợp lệ, xem §7
  repeated FleetSpecServerProto servers = 2;
  int32 concurrency = 3; // 0 = dùng default 5
}

message BulkProvisionFleetEvent {
  string host = 1;
  enum Status { PENDING = 0; SUCCEEDED = 1; FAILED = 2; }
  Status status = 2;
  string dev_server_id = 3;
  string error = 4;
}
```

`adapter/grpc/server.go` — `Server.BulkProvisionFleet` (server-streaming handler, mirror
`Server.StreamVmProvision` đã có cho cùng convention):

```go
func (s *Server) BulkProvisionFleet(req *infrafleetv1.BulkProvisionFleetRequest, stream infrafleetv1.InfraFleetService_BulkProvisionFleetServer) error {
	spec := usecase.FleetSpec{Servers: toFleetSpecServers(req.GetServers())}
	_, err := s.bulkProvisionFleet.Execute(stream.Context(), spec, int(req.GetConcurrency()), func(r usecase.BulkProvisionServerResult) {
		_ = stream.Send(toBulkProvisionFleetEvent(r)) // lỗi send không chặn goroutine khác — log, không return
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	return nil
}
```

**Bảo mật — bắt buộc:** `req.GetTenantId()` **KHÔNG bao giờ** được dùng trực tiếp — `tenantID` luôn lấy qua
`tenant.RequireTenantID(ctx)` bên trong từng usecase (`CreateSshTarget`/`RegisterDevServer` đã tự làm việc
này, `BulkProvisionFleet` chỉ truyền `ctx` xuống, không tự đọc `req.TenantId`). Field `tenant_id` trên request
proto (nếu giữ, để tương thích convention message khác trong file) chỉ mang tính tài liệu/logging, không bao
giờ là nguồn sự thật cho tenant scoping.

## 7. Wiring `cmd/server/main.go`

```go
deleteSshTarget := usecase.NewDeleteSshTarget(sshTargetRepo)
bulkProvisionFleet := usecase.NewBulkProvisionFleet(createSshTarget, registerDevServer, deleteSshTarget, sshTargetRepo)
// Server.New(...) nhận thêm bulkProvisionFleet — mirror TASK-BE-STORAGE-003's pattern thêm tham số usecase mới
```

## 8. YAML schema — `vaultSshRole` (frontend, không phải backend-go, ghi lại để solution đầy đủ)

```ts
// frontend/src/shared/fleet-config-parser.ts — FleetServerSchema
const FleetServerSchema = z.object({
  // ...trường đã có...
  identityFile: z.string().optional(), // GIỮ — dùng cho path desktop/ legacy, @deprecated cho path backend-go
  vaultSshRole: z.string().optional(), // MỚI — bắt buộc khi target là backend-go's BulkProvisionFleet
})
```

`infra-fleet-service` (Go) không parse YAML — chỉ nhận `FleetSpec` đã parse ở tầng gọi RPC, đúng quyết định
đã chốt ở CR-FLEET-001 (tránh 2 nơi định nghĩa cùng 1 schema).

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Concurrency với DB unique constraint | Thấp | `23505` xử lý được ở usecase layer, không crash goroutine khác |
| Compensating rollback không phải transaction thật | Trung bình | Nếu `DeleteSshTarget` cũng fail (DB down giữa 2 lần gọi), record mồ côi vẫn còn — chấp nhận được theo đúng CR gốc (ghi log, không block batch) |
| `sshrelay.Provisioner` HIGH risk | Cảnh báo, không áp dụng trực tiếp | Solution này không sửa struct đó — chỉ cảnh báo cho PR tương lai |
| Số thứ tự migration `0017` | Đã verify đúng (2026-09-09) | Verify lại 1 lần nữa ngay trước khi tạo file thật lúc implement — `migrations/` có thể tiến thêm giữa lúc viết solution và lúc code |

## Không thuộc phạm vi solution này

- Materialize `identityFile` → Vault SSH role tự động (CR §"Không thuộc phạm vi" mục 1).
- `BulkApproveDevServers` cho N dev server `PendingApproval` (mục 2).
- Cutover Admin UI/Wizard sang gọi `backend-go` (mục 3, thuộc CR-RBAC-001).
- IaC thật (tạo host mới) — xem [BE-FLEET-SOL-002](./BE-FLEET-SOL-002-iac-terraform-orchestration.md).

## Liên quan

- `backend-go/services/infra-fleet-service/internal/usecase/register_dev_server.go:38`
- `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go:32`
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:21-117` (`DevServerRepository`, `SshTargetRepository`)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:66` (tham chiếu, không sửa)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:196` (`StreamVmProvision`, convention tái dùng)
- `frontend/src/shared/fleet-config-parser.ts:36` (`FleetServerSchema`)
- [BE-FLEET-SOL-002](./BE-FLEET-SOL-002-iac-terraform-orchestration.md) — mở rộng thêm bước tạo host mới phía trước
- [BE-FLEET-SOL-003](./BE-FLEET-SOL-003-fleet-definition-persistence.md) — persist `FleetSpecServer` này vào DB
