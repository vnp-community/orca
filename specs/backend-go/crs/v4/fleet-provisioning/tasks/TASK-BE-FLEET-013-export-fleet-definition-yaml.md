# TASK-BE-FLEET-013: Usecase `ExportFleetDefinitionYaml` + RPC

**Solution:** BE-FLEET-SOL-003 §4, §5 | **CR:** CR-FLEET-003
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-012 (`FleetDefinitionRepository`/`GetFleetDefinition` phải tồn tại)
**Status:** ✅ DONE (2026-09-09) — code hoàn chỉnh, NHƯNG phát hiện 1 gap thật cần ghi nhận (xem bên dưới)

> **Kết quả thực tế:** `ExportFleetDefinitionYaml` + `serializeFleetYaml` implement đúng field mapping của
> task (`id`/`label` synthesize từ host, `vaultSshRole` camelCase). `gopkg.in/yaml.v3 v3.0.1` xác nhận đã có
> sẵn trong `go.mod` dạng `// indirect` — sau khi thêm `import "gopkg.in/yaml.v3"`, chạy `go mod tidy` gỡ đúng
> `// indirect`, KHÔNG đổi version (`go.sum` chỉ thêm hash còn thiếu cho `nats-io` — dependency có sẵn trong
> `go.mod` nhưng thiếu sum entry, không liên quan gì tới thay đổi của task này).
>
> **⚠️ GAP THẬT phát hiện khi làm — ghi nhận đúng khuôn README's "không tự chọn hướng khác":** đọc lại
> `frontend/src/shared/fleet-config-parser.ts`'s `FleetServerSchema` (dòng 36-53) THẬT trước khi khoá code —
> field `vaultSshRole` **KHÔNG TỒN TẠI** trong schema hiện tại (chỉ có `id, label, host, port, username,
> identityFile, jumpHost, proxyCommand, relayGracePeriodSeconds, project, team, environment, tags, repos,
> portForwards, bootstrap`). Lý do: TASK-BE-FLEET-005 (task duy nhất định nghĩa thêm field `vaultSshRole` vào
> schema này) đã bị **SKIP** trong lượt thực thi này (ngoài phạm vi thư mục `frontend/` được phép sửa). Hệ
> quả: usecase này VẪN emit `vaultSshRole` đúng theo spec của CHÍNH task 013 (không tự ý bỏ field), nhưng
> round-trip AC ("export rồi import lại phải cho ra dữ liệu tương đương") **CHƯA đúng hoàn toàn end-to-end**
> — Zod's `.object()` mặc định strip field lạ, nên `vaultSshRole` sẽ bị mất khi parse lại qua
> `FleetConfigSchema` THẬT hôm nay, cho tới khi TASK-BE-FLEET-005 được làm bởi 1 phiên có quyền sửa
> `frontend/`. Đã ghi rõ trong doc comment của `export_fleet_definition_yaml.go` VÀ trong test
> `TestExportFleetDefinitionYaml_RoundTrip_MatchesFleetConfigSchema` (chỉ assert những field THẬT SỰ đã tồn
> tại trong schema hôm nay — `id`/`label`/`host` — không giả vờ `vaultSshRole` đã round-trip được).
>
> **Build/test thật đã chạy**: `buf generate --path orca/infrafleet/v1/infrafleet.proto` (từ `backend-go/proto/`)
> sạch, chỉ đổi `infrafleet.pb.go`/`infrafleet_grpc.pb.go`. `go mod tidy` chạy sạch, gỡ đúng `// indirect` cho
> `yaml.v3`. `go build ./...` sạch (service này + 6 service tiêu thụ khác). `go test
> ./internal/usecase/... -run ExportFleetDefinitionYaml -v` — 5/5 PASS. `go test ./internal/adapter/grpc/...
> -v` — 12/12 PASS (toàn bộ RPC test, không chỉ Export). `go test ./...` — PASS, không regress. `gofmt -l`
> sạch.

---

## Mục tiêu

Serialize 1 `FleetDefinition` đã lưu → YAML đúng schema `orca-fleet.yaml`
(`frontend/src/shared/fleet-config-parser.ts`'s `FleetConfigSchema`) — round-trip được: export rồi import
lại (parse bằng `FleetConfigSchema`) phải cho ra dữ liệu tương đương bản gốc.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/usecase/export_fleet_definition_yaml.go` (MỚI)
2. `backend-go/services/infra-fleet-service/internal/usecase/export_fleet_definition_yaml_test.go` (MỚI)
3. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (MODIFY — `rpc ExportFleetDefinitionYaml`)
4. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (MODIFY — 1 handler mới)

## Field mapping chính xác — đã verify với `fleet-config-parser.ts`

`FleetServerSchema` (dòng 36-53 của `fleet-config-parser.ts`) yêu cầu **`id`** và **`label`** là bắt buộc
(không `.optional()`), nhưng `domain.FleetSpecServer` (TASK-BE-FLEET-010) chỉ có `Host, UserName,
VaultSSHRole, Kind` — **không có `id`/`label`**. Đây là 1 gap thật giữa 2 schema, phải xử lý khi serialize:

```go
// export_fleet_definition_yaml.go
package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type ExportFleetDefinitionYaml struct {
	repo FleetDefinitionRepository
}

func NewExportFleetDefinitionYaml(repo FleetDefinitionRepository) *ExportFleetDefinitionYaml {
	return &ExportFleetDefinitionYaml{repo: repo}
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
	return serializeFleetYaml(def)
}

// serializeFleetYaml maps domain.FleetDefinition -> orca-fleet.yaml's
// FleetConfigSchema (frontend/src/shared/fleet-config-parser.ts:36-92).
// FleetServerSchema requires `id`/`label` (not present on
// domain.FleetSpecServer) — synthesized here as host-derived values so
// the exported YAML still validates against FleetConfigSchema on
// re-import. This is a lossy synthesis (id/label are invented, not
// round-tripped from anything the user originally typed) — acceptable
// per CR-FLEET-003's AC ("round-trip: parse lại ra FleetSpec tương
// đương", not "byte-identical id/label").
func serializeFleetYaml(def domain.FleetDefinition) (string, error) {
	type yamlServer struct {
		ID           string `yaml:"id"`
		Label        string `yaml:"label"`
		Host         string `yaml:"host"`
		Username     string `yaml:"username,omitempty"`
		VaultSSHRole string `yaml:"vaultSshRole,omitempty"`
	}
	type yamlProvision struct {
		IaC        string `yaml:"iac"`
		WorkingDir string `yaml:"workingDir"`
		VarsFile   string `yaml:"varsFile,omitempty"`
	}
	type yamlConfig struct {
		Version   string         `yaml:"version"`
		Servers   []yamlServer   `yaml:"servers"`
		Provision *yamlProvision `yaml:"provision,omitempty"`
	}

	cfg := yamlConfig{Version: "1"}
	for i, s := range def.Servers {
		cfg.Servers = append(cfg.Servers, yamlServer{
			ID:           fmt.Sprintf("%s-%d", def.ID, i), // synthesized — xem doc comment ở trên
			Label:        s.Host,                          // dùng Host làm label mặc định — không có field label riêng trong domain
			Host:         s.Host,
			Username:     s.UserName,
			VaultSSHRole: s.VaultSSHRole,
		})
	}
	if def.Provision != nil {
		cfg.Provision = &yamlProvision{
			IaC: def.Provision.IaC, WorkingDir: def.Provision.WorkingDir, VarsFile: def.Provision.VarsFile,
		}
	}

	out, err := yaml.Marshal(cfg) // xác nhận thư viện YAML dùng ở Go — "gopkg.in/yaml.v3" hay tương đương,
	                                // grep go.mod của infra-fleet-service trước khi khoá import, KHÔNG đoán
	if err != nil {
		return "", err
	}
	return string(out), nil
}
```

**Đã verify:** (1) tên field YAML chính xác (`vaultSshRole` camelCase, không `vault_ssh_role` — xác nhận đúng
ở TASK-BE-FLEET-005); (2) `gopkg.in/yaml.v3 v3.0.1` **đã có trong**
`backend-go/services/infra-fleet-service/go.mod` (dòng 96) nhưng đánh dấu `// indirect` — nghĩa là chưa có
file nào trong service này import trực tiếp nó. Task này là import trực tiếp đầu tiên; sau khi thêm
`import "gopkg.in/yaml.v3"`, chạy `go mod tidy` để dòng `// indirect` được gỡ (go.mod tự cập nhật đúng), xác
nhận `go.sum` không đổi version.

## RPC

```protobuf
message ExportFleetDefinitionYamlRequest { string id = 1; }
message ExportFleetDefinitionYamlResponse { string yaml_content = 1; }
rpc ExportFleetDefinitionYaml(ExportFleetDefinitionYamlRequest) returns (ExportFleetDefinitionYamlResponse);
```

```go
func (s *Server) ExportFleetDefinitionYaml(ctx context.Context, req *infrafleetv1.ExportFleetDefinitionYamlRequest) (*infrafleetv1.ExportFleetDefinitionYamlResponse, error) {
	yamlContent, err := s.exportFleetDefinitionYaml.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.ExportFleetDefinitionYamlResponse{YamlContent: yamlContent}, nil
}
```

## Test cases cần cover — round-trip là AC bắt buộc của CR

- `TestExportFleetDefinitionYaml_NotFound_ReturnsNotFoundError`
- `TestExportFleetDefinitionYaml_ProducesValidYamlStructure` — parse output bằng 1 YAML parser Go generic,
  xác nhận đúng field name (`vaultSshRole`, không phải `vault_ssh_role`).
- `TestExportFleetDefinitionYaml_RoundTrip_MatchesFleetConfigSchema` — **quan trọng nhất**: test này PHẢI
  chạy được cả 2 phía (Go export + TS parse) hoặc ít nhất xác nhận field name/shape khớp tay bằng cách so
  sánh trực tiếp với `FleetServerSchema`'s field list (`id, label, host, username, vaultSshRole, ...`) —
  nếu CI không chạy được cross-language test thật, viết 1 fixture YAML cố định mà cả Go test (so string) và
  TS test (`fleet-config-parser.test.ts`, nếu tồn tại) cùng dùng, đảm bảo đồng bộ thủ công.
- `TestExportFleetDefinitionYaml_NoProvision_OmitsProvisionField` — `def.Provision == nil` → output YAML
  không có key `provision` (không phải `provision: null`).

## Verify

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/usecase/... -run ExportFleetDefinitionYaml -v
gofmt -l internal/usecase/export_fleet_definition_yaml.go internal/adapter/grpc/server.go
```

## gitnexus

`ExportFleetDefinitionYaml` là symbol mới — `impact()` sau khi tạo. `Server` struct sửa constructor — chạy
`impact({target: "Server", direction: "upstream", file_path: ".../adapter/grpc/server.go"})` trước.

## Blocking

Không task nào trong bộ này phụ thuộc cứng usecase này (độc lập với TASK-BE-FLEET-014).
