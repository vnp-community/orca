# BE-FLEET-SOL-003: `FleetDefinition` — lưu DB, export YAML, deploy lặp lại

> **🔲 Designed — chưa implement.** Phụ thuộc CỨNG [BE-FLEET-SOL-001](./BE-FLEET-SOL-001-bulk-provision-from-yaml.md) và [BE-FLEET-SOL-002](./BE-FLEET-SOL-002-iac-terraform-orchestration.md).

**CR:** [CR-FLEET-003](../../../../../../docs/crs/v4/fleet-provisioning/CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md)
**Service:** `infra-fleet-service`
**TDD tham chiếu:** [`infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md)

---

## 1. Re-verify code thật trước khi thiết kế

- **Migration số thứ tự** — `ls backend-go/services/infra-fleet-service/migrations/` (2026-09-09) xác nhận
  file mới nhất hiện có là `0016_ephemeral_vm_ssh_target_host_key.up/down.sql`. Nếu
  [BE-FLEET-SOL-001](./BE-FLEET-SOL-001-bulk-provision-from-yaml.md)'s `0017_ssh_targets_host_unique` được
  merge trước (đúng thứ tự phụ thuộc), file migration của solution này là **`0018`** — khớp đúng đề xuất của
  CR-FLEET-003. **Bắt buộc `ls migrations/` lại lần nữa ngay trước khi tạo file thật lúc implement** — CR-FLEET-003
  chính nó cũng phụ thuộc CR-FLEET-001/002 merge trước, và giữa lúc solution này được viết và lúc implement có
  thể có migration khác chen vào từ 1 CR song song khác (đúng như CR-FLEET-003 tự cảnh báo, và đúng như
  `TASK-BE-STORAGE-003` từng gặp — 1 tiến trình song song có thể đổi state working directory).
- `docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md` — file **có tồn tại thật**
  (xác nhận qua `find`) — cảnh báo chéo của CR-FLEET-003 về việc kiểm tra `infra-fleet-service` có nằm trong
  danh sách pilot multi-dialect hay không **trước khi** hard-code `JSONB` là hợp lệ, không phải suy đoán vô
  căn cứ. Solution này giữ nguyên `JSONB` (Postgres-only) làm baseline **nhưng ghi rõ điều kiện huỷ bỏ** —
  xem §3.
- `FleetSpecServer` (từ BE-FLEET-SOL-001 §2) và `ApplyTerraformPlan`/`ProvisionConfig`-tương-đương (từ
  BE-FLEET-SOL-002 §3, §7) là 2 phụ thuộc cứng — solution này **tái dùng nguyên trạng**, không định nghĩa lại.

## 2. Domain — `FleetDefinition`

```go
// backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go
type ProvisionConfig struct {
	IaC        string // "terraform" — khớp BE-FLEET-SOL-002's FleetConfigSchema.provision.iac
	WorkingDir string
	VarsFile   string
}

type FleetDefinition struct {
	ID        string
	TenantID  string
	Name      string // unique theo (tenant_id, name)
	Version   int
	Servers   []usecase.FleetSpecServer // TÁI DÙNG struct đã định nghĩa ở BE-FLEET-SOL-001 — không định nghĩa lại field-for-field
	Provision *ProvisionConfig          // nil nếu definition chỉ đăng ký host có sẵn
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

**Lưu ý kiến trúc:** `domain` package thường không import `usecase` (ngược hướng phụ thuộc chuẩn — domain là
tầng thấp nhất). Nếu `FleetSpecServer` được định nghĩa ở `usecase/bulk_provision_fleet.go` (như
BE-FLEET-SOL-001 đề xuất), `domain.FleetDefinition` **không thể** import nó mà không đảo ngược hướng phụ
thuộc. Hai lựa chọn khi implement (quyết định khi task-hoá, không đoán ở đây):

1. Di chuyển `FleetSpecServer` xuống `domain` package (từ `usecase/bulk_provision_fleet.go` sang
   `domain/ssh_target.go` hoặc file domain mới) — `usecase.BulkProvisionFleet` dùng lại `domain.FleetSpecServer`.
   Đây là hướng **khuyến nghị** vì giữ đúng nguyên tắc hướng phụ thuộc, và cả `BulkProvisionFleet` lẫn
   `FleetDefinition` đều là usecase-level construct dùng chung 1 domain concept.
2. Giữ `FleetSpecServer` ở `usecase`, và `FleetDefinition` cũng định nghĩa ở `usecase` package (không phải
   `domain`) — khác nhẹ so với CR gốc's line `backend-go/.../internal/domain/fleet_definition.go`, nhưng
   tránh vi phạm hướng phụ thuộc.

Solution này chọn **hướng 1** (di chuyển `FleetSpecServer` xuống domain) — nhất quán với vị trí file CR gốc
đề xuất (`internal/domain/fleet_definition.go`) và không cần usecase-import-usecase. Task tương ứng phải cập
nhật lại `BE-FLEET-SOL-001`'s `bulk_provision_fleet.go` để import `domain.FleetSpecServer` thay vì định nghĩa
tại chỗ — ghi rõ trong tasks/README.md như một điều chỉnh nhỏ so với BE-FLEET-SOL-001 gốc.

```go
// backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go
type FleetSpecServer struct {
	Host, UserName, VaultSSHRole string
	Kind                         AgentKind
}

type FleetDefinition struct {
	ID, TenantID, Name string
	Version             int
	Servers             []FleetSpecServer
	Provision           *ProvisionConfig
	CreatedBy           string
	CreatedAt, UpdatedAt time.Time
}
```

## 3. Migration `0018_fleet_definitions`

```sql
-- backend-go/services/infra-fleet-service/migrations/0018_fleet_definitions.up.sql
-- Số thứ tự tiếp theo BE-FLEET-SOL-001's 0017 (verify lại ls migrations/
-- ngay trước khi tạo file — xem §1).
CREATE TABLE infra.fleet_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    servers JSONB NOT NULL,   -- []FleetSpecServer, serialize trực tiếp — xem điều kiện huỷ bỏ dưới
    provision JSONB,          -- ProvisionConfig, nullable
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
```

```sql
-- 0018_fleet_definitions.down.sql
DROP TABLE infra.fleet_definitions;
```

**Điều kiện huỷ bỏ `JSONB`:** trước khi chạy migration này thật, kiểm tra
`docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md`'s danh sách service pilot —
nếu `infra-fleet-service` đã/sắp nằm trong danh sách đó tại thời điểm implement, đổi `JSONB` → kiểu
dialect-safe theo capability layer CR-DB-002 cung cấp (không tự chọn `JSON`/`TEXT` tuỳ tiện — dùng đúng
helper/kiểu mà CR-DB-002 đã chuẩn hoá, nếu đã có).

## 4. Usecase CRUD + Export + Deploy

```go
// backend-go/services/infra-fleet-service/internal/usecase/create_fleet_definition.go
type CreateFleetDefinitionInput struct {
	Name      string
	Servers   []domain.FleetSpecServer
	Provision *domain.ProvisionConfig
}

type CreateFleetDefinition struct {
	repo FleetDefinitionRepository
}

func NewCreateFleetDefinition(repo FleetDefinitionRepository) *CreateFleetDefinition {
	return &CreateFleetDefinition{repo: repo}
}

func (uc *CreateFleetDefinition) Execute(ctx context.Context, in CreateFleetDefinitionInput) (domain.FleetDefinition, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def := domain.FleetDefinition{
		ID: uuid.NewString(), TenantID: tenantID, Name: in.Name,
		Version: 1, Servers: in.Servers, Provision: in.Provision,
		CreatedBy: mustUserIDFromContext(ctx), // xác nhận helper thật lấy userID từ ctx (mirror pattern tenantID) khi implement — không đoán tên hàm
	}
	return uc.repo.Create(ctx, def)
}
```

`UpdateFleetDefinition`/`GetFleetDefinition`/`ListFleetDefinitions` theo cùng khuôn ngắn gọn — gọi thẳng
repository, không business logic khác (đúng "usecase mỏng khi không có gì để quyết định",
`03-clean-architecture-guidelines.md`). `UpdateFleetDefinition` tăng `Version` (`current.Version + 1`), không
xoá bản cũ (đúng CR §"Không thuộc phạm vi" mục 1 — không có bảng lịch sử riêng).

```go
// export_fleet_definition_yaml.go
type ExportFleetDefinitionYaml struct {
	repo FleetDefinitionRepository
}

func (uc *ExportFleetDefinitionYaml) Execute(ctx context.Context, id string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, id)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
	}
	return serializeFleetYaml(def) // mapping ngược sang orca-fleet.yaml's schema — field-for-field với
	                                 // FleetConfigSchema (frontend), bao gồm vaultSshRole (SOL-001) + provision (SOL-002)
}
```

`serializeFleetYaml` phải sinh đúng field name/case mà `frontend/src/shared/fleet-config-parser.ts`'s
`FleetConfigSchema` mong đợi khi parse lại (round-trip test — xem §"Test cases" ở tasks) — đọc lại schema
YAML thật (camelCase field name: `vaultSshRole`, không phải `vault_ssh_role`) trước khi khoá serializer.

```go
// deploy_fleet_definition.go
type DeployFleetDefinition struct {
	repo               FleetDefinitionRepository
	applyTerraformPlan *ApplyTerraformPlan     // BE-FLEET-SOL-002 — gọi lại nguyên trạng, KHÔNG viết lại
	bulkProvisionFleet *BulkProvisionFleet     // BE-FLEET-SOL-001 — gọi lại nguyên trạng, KHÔNG viết lại
}

func (uc *DeployFleetDefinition) Execute(ctx context.Context, id string, controlDevServerID string, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return BulkProvisionResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, id)
	if err != nil {
		return BulkProvisionResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
	}

	servers := def.Servers
	if def.Provision != nil {
		// Provision != nil: tạo host mới trước qua Terraform (SOL-002),
		// gộp kết quả (instance mới) vào servers hiện có, rồi mới
		// bulk-register toàn bộ — thứ tự bắt buộc theo CR §"Giải pháp".
		tfResult, err := uc.applyTerraformPlan.Execute(ctx, ApplyTerraformPlanInput{
			ControlDevServerID: controlDevServerID,
			WorkingDir:         def.Provision.WorkingDir,
			VarsFile:           def.Provision.VarsFile,
		})
		if err != nil {
			return BulkProvisionResult{}, err
		}
		for _, inst := range tfResult.Instances {
			servers = append(servers, domain.FleetSpecServer{Host: inst.Host /* UserName/VaultSSHRole từ đâu? xem ghi chú dưới */})
		}
	}

	return uc.bulkProvisionFleet.Execute(ctx, FleetSpec{Servers: servers}, 0, emit)
}
```

**Câu hỏi mở cần trả lời khi task-hoá (không đoán ở solution này):** `TerraformInstance` (BE-FLEET-SOL-002)
chỉ trả `Host` — nhưng `FleetSpecServer` cần cả `UserName`/`VaultSSHRole` để `CreateSshTarget` chạy được.
Terraform tạo instance mới thì `VaultSSHRole` chưa thể tồn tại từ trước (đó là 1 role trong Vault, phải được
tạo/gán riêng, ngoài phạm vi Terraform apply). Có 2 hướng, **chưa chọn, cần quyết định khi implement**:

1. `ProvisionConfig` thêm field `defaultUserName`/`defaultVaultSshRole` áp dụng cho MỌI instance Terraform
   tạo ra trong definition đó (đơn giản, nhưng giả định tất cả instance dùng chung 1 Vault role).
2. `terraform output` bắt buộc trả thêm `vault_ssh_role` per-instance (user tự cấu hình `.tf` để output field
   này) — linh hoạt hơn nhưng đặt thêm ràng buộc lên `.tf` của user.

Solution này **không tự chọn** — đây là 1 gap thật giữa BE-FLEET-SOL-001/002 mà chỉ lộ ra khi ghép 2 solution
lại ở CR-FLEET-003, cần Product Owner/tech lead quyết định trước khi `DeployFleetDefinition`'s nhánh
`Provision != nil` được code đầy đủ.

## 5. RPC

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto — cạnh BulkProvisionFleet (SOL-001), ApplyTerraformPlan (SOL-002)
message FleetDefinitionProto {
  string id = 1;
  string name = 2;
  int32 version = 3;
  repeated FleetSpecServerProto servers = 4; // TÁI DÙNG message đã định nghĩa ở SOL-001
  ProvisionConfigProto provision = 5;
  string created_by = 6;
  string created_at = 7;
  string updated_at = 8;
}

message ProvisionConfigProto {
  string iac = 1;
  string working_dir = 2;
  string vars_file = 3;
}

rpc CreateFleetDefinition(CreateFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc UpdateFleetDefinition(UpdateFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc GetFleetDefinition(GetFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc ListFleetDefinitions(ListFleetDefinitionsRequest) returns (ListFleetDefinitionsResponse);
rpc ExportFleetDefinitionYaml(ExportFleetDefinitionYamlRequest) returns (ExportFleetDefinitionYamlResponse); // { string yaml_content = 1; }
rpc DeployFleetDefinition(DeployFleetDefinitionRequest) returns (stream BulkProvisionFleetEvent); // TÁI DÙNG message đã có ở SOL-001, KHÔNG định nghĩa event type mới
```

`adapter/grpc/server.go` — 6 handler mới, cùng khuôn `companyID`/`tenantID` từ context (không từ request
field) như mọi handler khác trong file — mirror `TASK-BE-STORAGE-003`'s pattern.

## 6. Postgres repository

```go
// backend-go/services/infra-fleet-service/internal/adapter/postgres/fleet_definition_repository.go
type FleetDefinitionRepository struct{ db *sql.DB }

func (r *FleetDefinitionRepository) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, _ := json.Marshal(def.Servers)
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, _ = json.Marshal(def.Provision)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO infra.fleet_definitions (id, tenant_id, name, version, servers, provision, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		def.ID, def.TenantID, def.Name, def.Version, serversJSON, provisionJSON, def.CreatedBy)
	if err != nil {
		return domain.FleetDefinition{}, err
	}
	return def, nil
}
// Update/Get/List theo cùng khuôn — Update dùng UPDATE ... SET version = version + 1, ... WHERE id = $1 AND tenant_id = $2
```

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `FleetSpecServer` phải di chuyển từ `usecase` sang `domain` | Trung bình | Thay đổi nhỏ so với BE-FLEET-SOL-001 gốc — cần đồng bộ 2 solution, ghi rõ ở tasks/README |
| `UserName`/`VaultSSHRole` cho instance Terraform tạo mới | Chưa quyết định | Gap thật giữa SOL-001/002, cần Product Owner/tech lead chốt trước khi code nhánh `Provision != nil` |
| `JSONB` vs multi-dialect (CR-DB-002) | Thấp hiện tại | Chỉ áp dụng nếu `infra-fleet-service` vào danh sách pilot — kiểm tra lại tại thời điểm migration chạy thật |
| Round-trip YAML export/import | Trung bình | Cần test tự động so dữ liệu đã parse, không chỉ so string YAML (đúng AC gốc) |
| Số thứ tự migration `0018` | Phụ thuộc SOL-001's `0017` merge trước | Verify lại `ls migrations/` ngay trước khi tạo file thật |

## Không thuộc phạm vi solution này

- Full audit trail/version history đầy đủ (mục 1, CR gốc).
- Resync/diff tự động khi deploy lại (mục 2).
- Permission chi tiết theo từng fleet definition — dùng chung RBAC hiện có (mục 3).
- Terraform state file storage (mục 4) — vẫn là quyết định vận hành của BE-FLEET-SOL-002.
- Cutover Admin UI hiện tại (mục 5, CR-RBAC-001).
- Chọn cụ thể hướng 1/2 cho câu hỏi mở ở §4 — cần quyết định trước khi task tương ứng được thực thi đầy đủ.

## Liên quan

- [BE-FLEET-SOL-001](./BE-FLEET-SOL-001-bulk-provision-from-yaml.md) — phụ thuộc cứng (`FleetSpecServer`, `BulkProvisionFleet`)
- [BE-FLEET-SOL-002](./BE-FLEET-SOL-002-iac-terraform-orchestration.md) — phụ thuộc cứng (`ApplyTerraformPlan`, `ProvisionConfig`)
- `frontend/src/shared/fleet-config-parser.ts` (`FleetConfigSchema`) — schema round-trip
- `docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md` — kiểm tra chéo trước khi migration `JSONB` chạy thật
- `docs/crs/v4/team-rbac/CR-RBAC-004-project-team-scoped-server-visibility.md` — access control tái dùng, không làm riêng
- `specs/backend-go/crs/v3/storage/tasks/TASK-BE-STORAGE-003-grpc-proto-and-usecases.md` — khuôn mẫu proto+usecase+handler đã dùng lại ở đây
