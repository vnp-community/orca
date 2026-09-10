# TASK-BE-FLEET-012: `FleetDefinitionRepository` + CRUD usecase (Create/Update/Get/List) + RPC

**Solution:** BE-FLEET-SOL-003 §4, §5, §6 | **CR:** CR-FLEET-003
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-010 (domain), TASK-BE-FLEET-011 (migration)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `FleetDefinitionRepository` port + 4 usecase + `FleetDefinitionStore` (Postgres) + 4 RPC
> handler đúng theo sketch. **Quyết định optimistic locking:** CHỌN áp dụng (khuyến nghị của task) —
> `UpdateFleetDefinition` đọc row hiện tại trước (`repo.Get`), tăng `Version`, gọi `repo.Update` với
> `WHERE version = version_cũ`; 0 row affected → `domain.ErrFleetDefinitionVersionConflict` →
> `apperrors.KindFailedPrecondition` (không có `KindConflict` riêng trong `common/apperrors` — dùng
> `FailedPrecondition`, đúng ví dụ "rebind while active execution" đã có trong doc comment của chính Kind đó).
>
> **`tenant.UserID(ctx)` xác nhận đúng như task ghi** — không có `RequireUserID`, dùng `UserID(ctx) (string,
> bool)` + tự check `ok`.
>
> **`r.db.Exec` trong sketch sai driver** — thực tế `FleetDefinitionStore` dùng `pgxpool.Pool` (`s.pool.QueryRow`/
> `s.pool.Query`), giống hệt `SshTargetStore`/`Repository` khác trong package — đã điều chỉnh, dùng
> `QueryRow(...).Scan(&CreatedAt, &UpdatedAt)` để lấy `created_at`/`updated_at` server-generated thay vì
> `Exec` trần (task's sketch không đọc lại timestamp sau INSERT — bổ sung để `FleetDefinitionProto.created_at`/
> `updated_at` không rỗng).
>
> **Proto:** `FleetSpecServerProto` TÁI DÙNG đúng từ TASK-BE-FLEET-003, không định nghĩa lại — 4 message +
> `ProvisionConfigProto`/`FleetDefinitionProto` + 4 rpc thêm cạnh `ApplyTerraformPlan`. `created_at`/`updated_at`
> là `string` (RFC3339) theo đúng sketch — không phải `google.protobuf.Timestamp`.
>
> **`impact()` cho các symbol mới** không chạy được — GitNexus chưa index file mới tạo trong phiên này (giới
> hạn lặp lại từ các task trước). `impact({target: "Server", ...})` trước khi sửa constructor: đã áp dụng bài
> học từ TASK-BE-FLEET-003/008 (LOW risk, chỉ `cmd/server/main.go` gọi `Server.New`), không chạy lại riêng vì
> cùng file/constructor đã impact 2 lần trong phiên này, không có thay đổi nào khác chen vào.
>
> **Build/test thật đã chạy**: `buf generate --path orca/infrafleet/v1/infrafleet.proto` (từ `backend-go/proto/`)
> sạch, chỉ đổi `infrafleet.pb.go`/`infrafleet_grpc.pb.go` (auth files do agent khác, xác nhận qua mtime). `go
> build ./...` sạch (service này + 6 service tiêu thụ khác). `go test ./internal/usecase/... -run
> FleetDefinition -v` — 15/15 PASS. `go test ./internal/adapter/grpc/... -v` — 10/10 PASS (toàn bộ RPC test,
> không chỉ FleetDefinition). `go test ./...` — PASS, không regress. Integration test Postgres
> (testcontainers, Docker sẵn có): `TestFleetDefinitionRepository_Create_RoundTrips`,
> `TestFleetDefinitionRepository_UniqueNameConstraint`, `TestFleetDefinitionRepository_Update_OptimisticLockConflict`
> — PASS khi container khởi động đúng (gặp lại flake tiền tồn tại của testcontainers 1 lần, retry riêng lẻ
> PASS 3/3 — không liên quan logic repository, xem ghi chú tương tự ở TASK-BE-FLEET-004/011). `gofmt -l` sạch
> (1 lần lệch alignment ở `server.go` do gofmt tự canh lại struct field — đã `gofmt -w`).

---

## Mục tiêu

Thêm `FleetDefinitionRepository` port + Postgres implementation, 4 usecase CRUD
(`CreateFleetDefinition`/`UpdateFleetDefinition`/`GetFleetDefinition`/`ListFleetDefinitions`), 4 RPC handler
tương ứng.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (MODIFY — thêm `FleetDefinitionRepository`)
2. `backend-go/services/infra-fleet-service/internal/usecase/create_fleet_definition.go` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/update_fleet_definition.go` (MỚI)
4. `backend-go/services/infra-fleet-service/internal/usecase/get_fleet_definition.go` (MỚI)
5. `backend-go/services/infra-fleet-service/internal/usecase/list_fleet_definitions.go` (MỚI)
6. `backend-go/services/infra-fleet-service/internal/usecase/{create,update,get,list}_fleet_definition_test.go` (MỚI)
7. `backend-go/services/infra-fleet-service/internal/adapter/postgres/fleet_definition_repository.go` (MỚI)
8. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (MODIFY — 4 message + 4 rpc)
9. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (MODIFY — 4 handler)
10. `backend-go/services/infra-fleet-service/cmd/server/main.go` (MODIFY — wire 4 usecase mới)

## `ports.go` — `FleetDefinitionRepository`

```go
type FleetDefinitionRepository interface {
	Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error)
	Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) // def.Version là version MỚI (đã +1), WHERE version = version cũ để optimistic lock — xem ghi chú dưới
	Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error)
	List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error)
}
```

**Optimistic locking cho `Update` — quyết định cần chốt:** CR gốc không đặc tả concurrency control cho
`UpdateFleetDefinition`. Khuyến nghị: `UPDATE ... SET version = version + 1, ... WHERE id = $1 AND tenant_id
= $2 AND version = $3` (client gửi version đang đọc, mismatch → 0 row affected → lỗi `INFRA_FLEET_DEFINITION_VERSION_CONFLICT`)
— tránh 2 client cùng update chồng nhau âm thầm mất dữ liệu. Nếu quyết định không cần optimistic lock (MVP
đơn giản hơn), ghi rõ trong PR description lý do bỏ qua — không im lặng bỏ qua.

## Usecase mẫu — `create_fleet_definition.go`

```go
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

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
	userID, ok := tenant.UserID(ctx) // common/tenant.go:58 — verify thật: có UserID(ctx) (string, bool),
	                                   // KHÔNG có RequireUserID (chỉ RequireTenantID có bản Require*, dòng 79)
	if !ok {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_USER", "no user in request context", nil)
	}

	def, err := domain.NewFleetDefinition(uuid.NewString(), tenantID, in.Name, in.Servers, in.Provision, userID)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_FLEET_DEFINITION", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, def)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_FLEET_DEFINITION_FAILED", "failed to create fleet definition", err)
	}
	return saved, nil
}
```

**Đã verify:** `common/tenant/tenant.go` có `UserID(ctx) (string, bool)` (dòng 58) nhưng **không có**
`RequireUserID` — chỉ `TenantID`/`RequireTenantID` có cặp non-require/require (dòng 52, 79). Dùng
`tenant.UserID(ctx)` + tự kiểm tra `ok` như code trên, không gọi 1 hàm `RequireUserID` không tồn tại.

`UpdateFleetDefinition`/`GetFleetDefinition`/`ListFleetDefinitions` theo cùng khuôn ngắn gọn — gọi thẳng
repository qua port interface, không business logic khác (đúng "usecase mỏng khi không có gì để quyết định").

## Postgres repository

```go
func (r *FleetDefinitionRepository) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, err := json.Marshal(def.Servers)
	if err != nil {
		return domain.FleetDefinition{}, err
	}
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, err = json.Marshal(def.Provision)
		if err != nil {
			return domain.FleetDefinition{}, err
		}
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO infra.fleet_definitions (id, tenant_id, name, version, servers, provision, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		def.ID, def.TenantID, def.Name, def.Version, serversJSON, provisionJSON, def.CreatedBy)
	if err != nil {
		return domain.FleetDefinition{}, err
	}
	return def, nil
}
```

**`r.db.Exec` — xác nhận API thật của DB handle đang dùng** (`pgxpool.Pool.Exec(ctx, sql, args...)` theo
`pgx/v5` đã verify là driver thật của service này — chữ ký `Exec` của `pgxpool` không giống
`database/sql`'s `ExecContext`, xác nhận đúng chữ ký trước khi khoá code, mirror cách
`dev_server_group_grant_repository.go` (đã import `pgx/v5`/`pgxpool`) gọi).

## Proto

```protobuf
message FleetSpecServerProto { /* đã có ở BE-FLEET-SOL-001/TASK-BE-FLEET-003 — TÁI DÙNG, không định nghĩa lại */ }

message ProvisionConfigProto {
  string iac = 1;
  string working_dir = 2;
  string vars_file = 3;
}

message FleetDefinitionProto {
  string id = 1;
  string name = 2;
  int32 version = 3;
  repeated FleetSpecServerProto servers = 4;
  ProvisionConfigProto provision = 5;
  string created_by = 6;
  string created_at = 7;
  string updated_at = 8;
}

message CreateFleetDefinitionRequest {
  string name = 1;
  repeated FleetSpecServerProto servers = 2;
  ProvisionConfigProto provision = 3;
}
message UpdateFleetDefinitionRequest {
  string id = 1;
  repeated FleetSpecServerProto servers = 2;
  ProvisionConfigProto provision = 3;
}
message GetFleetDefinitionRequest { string id = 1; }
message ListFleetDefinitionsRequest {}
message ListFleetDefinitionsResponse { repeated FleetDefinitionProto definitions = 1; }

rpc CreateFleetDefinition(CreateFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc UpdateFleetDefinition(UpdateFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc GetFleetDefinition(GetFleetDefinitionRequest) returns (FleetDefinitionProto);
rpc ListFleetDefinitions(ListFleetDefinitionsRequest) returns (ListFleetDefinitionsResponse);
```

## Test cases cần cover

- `TestCreateFleetDefinition_RequiresTenantContext`
- `TestCreateFleetDefinition_ValidatesInput` (mirror `TestCreateSshTarget_ValidatesInput`)
- `TestCreateFleetDefinition_CreatesWithTenantFromContext`
- `TestUpdateFleetDefinition_IncrementsVersion`
- `TestUpdateFleetDefinition_VersionConflict_ReturnsConflictError` (nếu optimistic lock được chọn)
- `TestGetFleetDefinition_ScopedByTenant` — tenant A không đọc được definition của tenant B
- `TestListFleetDefinitions_ReturnsOnlyCallerTenant`
- Postgres adapter: `TestFleetDefinitionRepository_Create_RoundTrips` (servers/provision JSON serialize rồi
  đọc lại đúng), `TestFleetDefinitionRepository_UniqueNameConstraint`

## Verify

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
cd backend-go/services/infra-fleet-service && go build ./... && go test ./... -run FleetDefinition -v
gofmt -l internal/usecase/*fleet_definition*.go internal/adapter/postgres/fleet_definition_repository.go internal/adapter/grpc/server.go
```

## gitnexus

`FleetDefinitionRepository`/`CreateFleetDefinition`/`UpdateFleetDefinition`/`GetFleetDefinition`/
`ListFleetDefinitions` đều là symbol mới — chạy `impact()` cho mỗi symbol ngay sau khi tạo, trước khi
TASK-BE-FLEET-013/014 thêm caller. `impact({target: "Server", direction: "upstream", file_path: ".../adapter/grpc/server.go"})`
trước khi sửa constructor `Server.New(...)`.

## Blocking

TASK-BE-FLEET-013 (`ExportFleetDefinitionYaml`) và TASK-BE-FLEET-014 (`DeployFleetDefinition`) phụ thuộc cứng
`FleetDefinitionRepository`/`GetFleetDefinition` đã tồn tại ở đây.
