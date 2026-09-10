# TASK-BE-FLEET-010: Domain `FleetDefinition`/`ProvisionConfig` — di chuyển `FleetSpecServer` xuống `domain`

**Solution:** BE-FLEET-SOL-003 §2 | **CR:** CR-FLEET-003
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-001 (`FleetSpecServer` phải tồn tại ở `usecase` trước để di chuyển), TASK-BE-FLEET-006 (`ProvisionConfig`-tương-đương cần tồn tại theo hình dạng đã biết)
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** `domain/fleet_definition.go` implement đúng `FleetSpecServer`/`ProvisionConfig`/
> `FleetDefinition`/`NewFleetDefinition` theo sketch. **Sửa 1 lỗi trong code mẫu của task:** sketch dùng
> `errors.New` nhưng chỉ import `"time"` (thiếu `"errors"`) — thêm import đúng.
>
> **`impact({target: "FleetSpecServer", ...})` không chạy được** — GitNexus chưa index các file mới tạo trong
> phiên này (đã gặp lại giới hạn này ở TASK-BE-FLEET-001); dùng `grep -rln "FleetSpecServer" --include=*.go .`
> thay thế (đúng như task's Verify mục cuối cũng dùng grep) — tìm thấy đúng 4 file tham chiếu:
> `bulk_provision_fleet.go`, `bulk_provision_fleet_test.go` (TASK-BE-FLEET-001), `server.go`,
> `server_test.go` (TASK-BE-FLEET-003) — `server_test.go`'s hits là `infrafleetv1.FleetSpecServerProto` (proto
> message, KHÔNG liên quan `usecase.FleetSpecServer`), bỏ qua đúng như dự đoán. Cập nhật 3 file còn lại: xoá
> định nghĩa tại chỗ trong `bulk_provision_fleet.go`, đổi `usecase.FleetSpecServer` → `domain.FleetSpecServer`
> trong `server.go`, đổi mọi `FleetSpecServer{`/`[]FleetSpecServer` trần trong
> `bulk_provision_fleet_test.go` sang `domain.FleetSpecServer`.
>
> **Verify grep cuối task (xác nhận không còn sót):**
> `grep -rn "usecase.FleetSpecServer\|[^.]FleetSpecServer{" internal/ | grep -v "domain.FleetSpecServer\|domain/fleet_definition\|FleetSpecServerProto"`
> → 0 kết quả, sạch.
>
> **Build/test thật đã chạy**: `go build ./...` sạch. `go test ./... -v` (TOÀN BỘ package, đúng yêu cầu "chạy
> lại TOÀN BỘ test suite của usecase" của task) — PASS, không regress ở bất kỳ package nào (usecase/domain/
> adapter/grpc/...). `gofmt -l` sạch trên mọi file đã sửa/thêm.

---

## Mục tiêu

Thêm domain entity `FleetDefinition`/`ProvisionConfig`, và **di chuyển** `FleetSpecServer` từ
`internal/usecase/bulk_provision_fleet.go` (TASK-BE-FLEET-001) xuống `internal/domain/` — lý do: `domain`
package không nên import `usecase` (hướng phụ thuộc ngược), nhưng `domain.FleetDefinition.Servers` cần kiểu
`[]FleetSpecServer`. Xem BE-FLEET-SOL-003 §2 cho phân tích đầy đủ.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go` (MỚI — `FleetDefinition`, `ProvisionConfig`, `FleetSpecServer`)
2. `backend-go/services/infra-fleet-service/internal/domain/fleet_definition_test.go` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` (MODIFY — xoá định nghĩa `FleetSpecServer` tại chỗ, import `domain.FleetSpecServer` thay thế)
4. Mọi file khác đang tham chiếu `usecase.FleetSpecServer` (TASK-BE-FLEET-003's `adapter/grpc/server.go`, TASK-BE-FLEET-001's test file) — cập nhật import, đổi `usecase.FleetSpecServer` → `domain.FleetSpecServer`

## `fleet_definition.go`

```go
// backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go
package domain

import "time"

// FleetSpecServer describes one server entry in a fleet spec/definition —
// moved here from usecase.FleetSpecServer (CR-FLEET-001's original
// location) so domain.FleetDefinition can reference it without usecase
// importing usecase (see BE-FLEET-SOL-003 §2 for the dependency-direction
// rationale).
type FleetSpecServer struct {
	Host, UserName, VaultSSHRole string
	Kind                         AgentKind
}

// ProvisionConfig mirrors CR-FLEET-002's FleetConfigSchema.provision —
// nil means the definition only registers pre-existing hosts, no new
// infrastructure is created.
type ProvisionConfig struct {
	IaC        string // "terraform"
	WorkingDir string
	VarsFile   string
}

// FleetDefinition is CR-FLEET-003's persisted source of truth for a fleet
// — Servers/Provision are the durable record of what BulkProvisionFleet
// (CR-FLEET-001) / ApplyTerraformPlan (CR-FLEET-002) were last asked to do.
type FleetDefinition struct {
	ID        string
	TenantID  string
	Name      string
	Version   int
	Servers   []FleetSpecServer
	Provision *ProvisionConfig
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewFleetDefinition validates required fields and returns a version-1
// FleetDefinition — mirror NewSshTarget/NewDevServer's validate-then-construct
// pattern already used throughout this package.
func NewFleetDefinition(id, tenantID, name string, servers []FleetSpecServer, provision *ProvisionConfig, createdBy string) (FleetDefinition, error) {
	if tenantID == "" {
		return FleetDefinition{}, ErrEmptyFleetDefinitionTenant
	}
	if name == "" {
		return FleetDefinition{}, ErrEmptyFleetDefinitionName
	}
	if len(servers) == 0 {
		return FleetDefinition{}, ErrEmptyFleetDefinitionServers
	}
	return FleetDefinition{
		ID: id, TenantID: tenantID, Name: name, Version: 1,
		Servers: servers, Provision: provision, CreatedBy: createdBy,
	}, nil
}

var (
	ErrEmptyFleetDefinitionTenant  = errors.New("domain: tenant_id is required")
	ErrEmptyFleetDefinitionName    = errors.New("domain: name is required")
	ErrEmptyFleetDefinitionServers = errors.New("domain: at least one server is required")
)
```

**Convention lỗi đã verify thật** (`internal/domain/ssh_target.go:5-16`): mỗi entity domain tự định nghĩa
biến lỗi riêng theo dạng `ErrEmpty<Entity><Field> = errors.New("domain: <field> is required")`
(vd. `ErrEmptySshTargetTenant`, `ErrEmptySshTargetHost`) — **không có** 1 biến lỗi `tenant_id` dùng chung
giữa các entity. `FleetDefinition` theo đúng convention này, không tái dùng biến của `SshTarget`.

## Cập nhật `bulk_provision_fleet.go` (TASK-BE-FLEET-001's file)

```go
// XOÁ định nghĩa tại chỗ:
// type FleetSpecServer struct { ... }

// THÊM import, dùng domain.FleetSpecServer xuyên suốt file:
import "github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"

type FleetSpec struct {
	Version string
	Servers []domain.FleetSpecServer
}
```

Mọi chỗ dùng `FleetSpecServer` (không có prefix) trong `bulk_provision_fleet.go`/test file đổi thành
`domain.FleetSpecServer`.

## Test cases cần cover

- `TestNewFleetDefinition_RequiresTenantID`
- `TestNewFleetDefinition_RequiresName`
- `TestNewFleetDefinition_RequiresAtLeastOneServer`
- `TestNewFleetDefinition_ValidInput_SetsVersion1`
- Regression: `go test ./internal/usecase/...` (toàn bộ package) vẫn PASS sau khi đổi `FleetSpecServer` sang
  `domain.FleetSpecServer` — không chỉ test file mới, chạy lại TOÀN BỘ test suite của `usecase` package để
  bắt lỗi compile/reference sót lại.

## Verify

```bash
cd backend-go/services/infra-fleet-service && go build ./... && go test ./... -v
gofmt -l internal/domain/fleet_definition.go internal/usecase/bulk_provision_fleet.go
grep -rn "usecase.FleetSpecServer\|[^.]FleetSpecServer{" internal/ | grep -v "domain.FleetSpecServer\|domain/fleet_definition"  # phải KHÔNG còn kết quả nào ngoài domain package
```

## gitnexus

`impact({target: "FleetSpecServer", direction: "upstream"})` **bắt buộc trước khi di chuyển** — xác nhận
toàn bộ call site hiện có (TASK-BE-FLEET-001/003 nếu đã merge) để không sót chỗ nào khi đổi package.
`FleetDefinition` là symbol mới — `impact()` sau khi tạo, trước khi TASK-BE-FLEET-012/013/014 thêm caller.

## Blocking

TASK-BE-FLEET-011 (migration), TASK-BE-FLEET-012 (CRUD usecase+RPC), TASK-BE-FLEET-013 (Export), TASK-BE-FLEET-014
(Deploy) đều phụ thuộc cứng `domain.FleetDefinition`/`domain.FleetSpecServer` đã tồn tại ở đây.
