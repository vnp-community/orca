# TASK-BE-FLEET-014: Usecase `DeployFleetDefinition` — điều phối CR-FLEET-001/002 + RPC

**Solution:** BE-FLEET-SOL-003 §4, §5 | **CR:** CR-FLEET-003
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-001 (`BulkProvisionFleet`), TASK-BE-FLEET-006 (`ApplyTerraformPlan`), TASK-BE-FLEET-012 (`FleetDefinitionRepository`/`GetFleetDefinition`)
**Status:** ✅ DONE (2026-09-09)

> **Câu hỏi mở đã chốt:** chọn ĐÚNG "hướng 1" (khuyến nghị của task) — `domain.ProvisionConfig` thêm
> `DefaultUserName`/`DefaultVaultSSHRole`, áp dụng cho mọi instance Terraform tạo ra trong 1 definition.
> `deploy_fleet_definition.go` implement đúng sketch, doc comment giải thích rõ lý do chọn hướng 1 thay vì
> hướng 2.
>
> **🔴 BUG THẬT PHÁT HIỆN VÀ ĐÃ SỬA — race condition trong `BulkProvisionFleet`'s handler (TASK-BE-FLEET-003):**
> khi viết `DeployFleetDefinition`'s RPC handler (cùng pattern `sendErr`/`stream.Send` trong `emit` callback,
> TÁI DÙNG nguyên văn từ `BulkProvisionFleet`'s handler theo đúng chỉ dẫn của task), chạy `go test ./... -race`
> lần đầu tiên trên package `adapter/grpc` trong bộ task này → phát hiện **data race thật**: `emit` callback
> của `BulkProvisionFleet.Execute` chạy từ N goroutine đồng thời (đúng thiết kế semaphore concurrency của
> chính usecase, TASK-BE-FLEET-001), nhưng handler gốc (TASK-BE-FLEET-003) đọc/ghi biến `sendErr` VÀ gọi
> `stream.Send` KHÔNG có khoá — gRPC stream không an toàn khi `Send` đồng thời từ nhiều goroutine. Đây là bug
> **THẬT, đã tồn tại từ TASK-BE-FLEET-003**, không phải bug mới của riêng `DeployFleetDefinition`. Đã sửa CẢ
> HAI handler (`BulkProvisionFleet` VÀ `DeployFleetDefinition`) bằng `sync.Mutex` bọc quanh
> `sendErr`/`stream.Send`. Race thứ 2 (nhỏ hơn) cũng phát hiện: `bulkFakeSshTargetRepositoryForServerTest`/
> `bulkFakeDevServerRepositoryForServerTest` (2 fake test double ở `server_test.go`, cũng tự viết ở
> TASK-BE-FLEET-003) thiếu `sync.Mutex` bảo vệ slice `created`/`registered` — sửa thêm mutex, mirror
> `bulkFakeSshTargetRepository`/`bulkFakeDevServerRepository` (usecase package) vốn đã có mutex đúng ngay từ
> đầu (TASK-BE-FLEET-001). Đã cập nhật addendum vào TASK-BE-FLEET-003's "Kết quả thực tế".
>
> **`impact()` cho `BulkProvisionFleet`/`ApplyTerraformPlan` trước khi thêm caller mới:** không chạy được qua
> GitNexus (giới hạn index lặp lại xuyên suốt phiên này) — dùng `grep -rln` xác nhận caller thật hiện có
> (chỉ `cmd/server/main.go` và `adapter/grpc/server.go`, cả 2 đã biết rõ), thêm `DeployFleetDefinition` làm
> caller thứ 3 là thay đổi an toàn, thuần cộng thêm, không sửa chữ ký 2 usecase đó.
>
> **Proto:** TÁI DÙNG `BulkProvisionFleetEvent` đúng như task chỉ định, không định nghĩa event type mới.
>
> **Build/test thật đã chạy**: `buf generate --path orca/infrafleet/v1/infrafleet.proto` (từ `backend-go/proto/`)
> sạch, chỉ đổi `infrafleet.pb.go`/`infrafleet_grpc.pb.go`. `go build ./...` sạch (service này + 6 service
> tiêu thụ khác). `go test ./internal/usecase/... -run DeployFleetDefinition -v -race` — 7/7 PASS. `go test
> ./internal/adapter/grpc/... -v` — 14/14 PASS. **`go test ./... -race -count=1` (TOÀN service, LẦN ĐẦU TIÊN
> chạy `-race` trong bộ 14 task này) — PASS sạch, chạy lặp lại 3 lần liên tiếp đều PASS**, xác nhận race đã
> sửa triệt để, không phải may mắn 1 lần. `gofmt -l` sạch. Không regress ở service nào khác.

---

## ⚠️ Câu hỏi mở CHƯA có câu trả lời — đọc trước khi implement

BE-FLEET-SOL-003 §4 xác định 1 gap thật: `ApplyTerraformPlan`'s `TerraformInstance` (TASK-BE-FLEET-006) chỉ
trả `Host` — nhưng `domain.FleetSpecServer` cần cả `UserName`/`VaultSSHRole` để `CreateSshTarget` chạy được.
Terraform tạo instance mới thì `VaultSSHRole` chưa thể tồn tại từ trước. **Task này PHẢI chốt 1 trong 2
hướng dưới đây trước khi code nhánh `Provision != nil`** (không được để TODO mơ hồ trong code merge):

1. `domain.ProvisionConfig` thêm field `DefaultUserName`/`DefaultVaultSSHRole` áp dụng cho MỌI instance
   Terraform tạo ra trong definition đó.
2. `terraform output` bắt buộc trả thêm `vault_ssh_role` per-instance (`parseTerraformOutput`,
   TASK-BE-FLEET-006, mở rộng convention output đã chọn để bao gồm field này).

**Khuyến nghị của task này: chọn hướng 1** (đơn giản hơn, đủ dùng cho MVP 1 cloud provider mà CR-FLEET-002
nhắm tới — mọi instance trong 1 lần `apply` thường dùng chung 1 Vault role vì cùng mục đích/team). Nếu chọn
hướng 2, cần quay lại sửa `TerraformInstance`/`parseTerraformOutput` (TASK-BE-FLEET-006) trước.

## Mục tiêu

Thêm usecase `DeployFleetDefinition` — đọc `FleetDefinition` theo `id`, nếu `Provision != nil` gọi
`ApplyTerraformPlan` trước rồi gộp kết quả vào `Servers`, cuối cùng gọi `BulkProvisionFleet` cho toàn bộ danh
sách. **Không viết lại** logic 2 usecase đó — chỉ điều phối đúng thứ tự.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go` (MODIFY — nếu chọn hướng 1, thêm `DefaultUserName`/`DefaultVaultSSHRole` vào `ProvisionConfig`)
2. `backend-go/services/infra-fleet-service/internal/usecase/deploy_fleet_definition.go` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/deploy_fleet_definition_test.go` (MỚI)
4. `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (MODIFY — `rpc DeployFleetDefinition`, TÁI DÙNG `BulkProvisionFleetEvent` đã có, không định nghĩa message mới)
5. `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (MODIFY — 1 handler mới)

## `ProvisionConfig` mở rộng (hướng 1 đã chọn)

```go
// internal/domain/fleet_definition.go
type ProvisionConfig struct {
	IaC        string
	WorkingDir string
	VarsFile   string
	// DefaultUserName/DefaultVaultSSHRole — MỚI, TASK-BE-FLEET-014's quyết
	// định "hướng 1": mọi instance Terraform tạo ra trong definition này
	// dùng chung 1 user/Vault role, vì ApplyTerraformPlan's output chỉ
	// cung cấp Host (BE-FLEET-SOL-003 §4's câu hỏi mở, đã chốt).
	DefaultUserName     string
	DefaultVaultSSHRole string
}
```

## `deploy_fleet_definition.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type DeployFleetDefinitionInput struct {
	FleetDefinitionID  string
	ControlDevServerID string // bắt buộc chỉ khi Provision != nil — validate trong Execute
}

type DeployFleetDefinition struct {
	repo               FleetDefinitionRepository
	applyTerraformPlan *ApplyTerraformPlan
	bulkProvisionFleet *BulkProvisionFleet
}

func NewDeployFleetDefinition(repo FleetDefinitionRepository, applyTerraformPlan *ApplyTerraformPlan, bulkProvisionFleet *BulkProvisionFleet) *DeployFleetDefinition {
	return &DeployFleetDefinition{repo: repo, applyTerraformPlan: applyTerraformPlan, bulkProvisionFleet: bulkProvisionFleet}
}

func (uc *DeployFleetDefinition) Execute(ctx context.Context, in DeployFleetDefinitionInput, emit func(BulkProvisionServerResult)) (BulkProvisionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return BulkProvisionResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, in.FleetDefinitionID)
	if err != nil {
		return BulkProvisionResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
	}

	servers := append([]domain.FleetSpecServer{}, def.Servers...) // copy — không mutate def.Servers gốc

	if def.Provision != nil {
		if in.ControlDevServerID == "" {
			return BulkProvisionResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_MISSING_CONTROL_DEV_SERVER", "control_dev_server_id is required when definition has a provision config", nil)
		}
		tfResult, err := uc.applyTerraformPlan.Execute(ctx, ApplyTerraformPlanInput{
			ControlDevServerID: in.ControlDevServerID,
			WorkingDir:         def.Provision.WorkingDir,
			VarsFile:           def.Provision.VarsFile,
		})
		if err != nil {
			return BulkProvisionResult{}, err // đã là apperrors từ ApplyTerraformPlan, không bọc lại 2 lần
		}
		for _, inst := range tfResult.Instances {
			servers = append(servers, domain.FleetSpecServer{
				Host:         inst.Host,
				UserName:     def.Provision.DefaultUserName,
				VaultSSHRole: def.Provision.DefaultVaultSSHRole,
			})
		}
	}

	return uc.bulkProvisionFleet.Execute(ctx, FleetSpec{Servers: servers}, 0, emit)
}
```

## RPC

```protobuf
message DeployFleetDefinitionRequest {
  string fleet_definition_id = 1;
  string control_dev_server_id = 2; // optional — chỉ cần nếu definition có provision config
}

rpc DeployFleetDefinition(DeployFleetDefinitionRequest) returns (stream BulkProvisionFleetEvent); // TÁI DÙNG message đã có ở TASK-BE-FLEET-003, không định nghĩa event type mới
```

```go
func (s *Server) DeployFleetDefinition(req *infrafleetv1.DeployFleetDefinitionRequest, stream infrafleetv1.InfraFleetService_DeployFleetDefinitionServer) error {
	var sendErr error
	_, err := s.deployFleetDefinition.Execute(stream.Context(), usecase.DeployFleetDefinitionInput{
		FleetDefinitionID:  req.GetFleetDefinitionId(),
		ControlDevServerID: req.GetControlDevServerId(),
	}, func(r usecase.BulkProvisionServerResult) {
		if sendErr != nil {
			return
		}
		status := infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED
		if r.Status == "FAILED" {
			status = infrafleetv1.BulkProvisionFleetEvent_FAILED
		}
		sendErr = stream.Send(&infrafleetv1.BulkProvisionFleetEvent{
			Host: r.Host, Status: status, DevServerId: r.DevServerID, Error: r.Error,
		})
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	if sendErr != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_DEPLOY_STREAM_SEND_FAILED", "failed to stream event", sendErr))
	}
	return nil
}
```

## Test cases cần cover (khớp CR-FLEET-003's AC)

- `TestDeployFleetDefinition_ProvisionNil_BehavesLikeBulkProvisionFleetDirectly` — `Provision == nil` → bỏ
  qua hoàn toàn bước Terraform, kết quả giống hệt gọi `BulkProvisionFleet` trực tiếp với `def.Servers` —
  **không regression**, đúng AC gốc.
- `TestDeployFleetDefinition_ProvisionNotNil_MissingControlDevServerID_ReturnsInvalidArgument`
- `TestDeployFleetDefinition_ProvisionNotNil_CallsApplyTerraformPlanBeforeBulkProvisionFleet` — xác nhận thứ
  tự gọi (dùng fake ghi lại call order).
- `TestDeployFleetDefinition_MixedServers_ExistingAndTerraformCreated` — 1 definition có cả server "đã tồn
  tại" (trong `def.Servers`) và Terraform trả về thêm N instance mới → `BulkProvisionFleet` nhận đủ
  `len(def.Servers) + N` server, cả 2 loại xử lý đúng (khớp AC "test tái hiện 1 definition có cả 2 loại
  server").
- `TestDeployFleetDefinition_ApplyTerraformPlanFails_DoesNotCallBulkProvisionFleet` — lỗi ở bước Terraform →
  dừng lại, không gọi tiếp `BulkProvisionFleet` (tránh provision N server rác nếu Terraform fail giữa
  chừng).
- `TestDeployFleetDefinition_NotFound_ReturnsNotFoundError`
- `TestDeployFleetDefinition_RunTwice_SameID_NoIdempotencyRegression` — thừa hưởng unique constraint
  (TASK-BE-FLEET-004) qua `BulkProvisionFleet`, không cần cơ chế mới — chạy 2 lần liên tiếp không tạo
  `SshTarget` trùng.

## Verify

```bash
cd backend-go && buf generate --path orca/infrafleet/v1/infrafleet.proto
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/usecase/... -run DeployFleetDefinition -v
gofmt -l internal/domain/fleet_definition.go internal/usecase/deploy_fleet_definition.go internal/adapter/grpc/server.go
```

## gitnexus

**Bắt buộc chạy lại `impact()` cho `BulkProvisionFleet`/`ApplyTerraformPlan` NGAY TRƯỚC khi task này thêm
caller mới vào 2 usecase đó** — CR-FLEET-003 tự ghi rõ yêu cầu này (không dùng số liệu ước tính cũ của
CR-FLEET-001/002, code thật tại thời điểm task này bắt đầu có thể đã khác):

```
impact({target: "BulkProvisionFleet", direction: "upstream"})
impact({target: "ApplyTerraformPlan", direction: "upstream"})
```

`DeployFleetDefinition` là symbol mới — `impact()` sau khi tạo, trước khi mở rộng thêm.

## Blocking

Không task nào trong bộ này phụ thuộc `DeployFleetDefinition` — đây là điểm cuối của chuỗi phụ thuộc xuyên
suốt cả 3 CR trong bộ task này.
