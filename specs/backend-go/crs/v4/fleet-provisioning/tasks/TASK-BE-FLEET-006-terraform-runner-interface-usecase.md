# TASK-BE-FLEET-006: `TerraformRunner` interface + usecase `ApplyTerraformPlan`

**Solution:** BE-FLEET-SOL-002 | **CR:** CR-FLEET-002
**Service:** `infra-fleet-service`
**Depends on:** TASK-BE-FLEET-007 (agent RPC `terraform.apply` phải tồn tại trước khi `TerraformRunner`'s implementation thật gọi được nó — nhưng usecase/interface có thể code song song, chỉ implementation cụ thể của `TerraformRunner` phụ thuộc)
**Status:** ✅ DONE (2026-09-09) — interface + usecase only (implementation thật của `TerraformRunner` gọi RPC
agent chưa được viết trong bộ 14 task này — không task nào trong bộ liệt kê 1 file adapter cụ thể cho nó;
TASK-BE-FLEET-014 sẽ cần 1 implementer thật hoặc fake khi wiring `main.go`)

> **Kết quả thực tế:** `TerraformRunner` (ports.go) + `ApplyTerraformPlan`/`parseTerraformOutput`
> (apply_terraform_plan.go) implement đúng nội dung task, không sai khác so với sketch. `impact()` không chạy
> được trước khi tạo (symbol mới, task tự ghi "không cần impact trước khi tạo — chưa tồn tại để impact").
>
> **Không tự thêm credential injection** — đúng "Phạm vi" của task: `TerraformRunner.Apply` không có tham số
> credential nào; usecase không đọc bất kỳ config/env cố định nào. Doc comment ghi rõ tham chiếu
> TASK-BE-FLEET-009 (bị skip, cần security review).
>
> **Build/test thật đã chạy**: `go build ./...` sạch. `go test ./internal/usecase/... -run
> 'ApplyTerraformPlan|ParseTerraformOutput' -v` — 7/7 PASS. `go test ./...` (toàn service) — PASS, không
> regress. `gofmt -l` sạch (1 lần lệch alignment struct field do gofmt tự canh lại — đã chạy `gofmt -w`, xác
> nhận sạch lại).

---

## Phạm vi — KHÔNG bao gồm credential storage

Task này chỉ làm phần **orchestration** (gọi `terraform apply` qua agent, parse output) — **không** chạm vào
việc lưu trữ/truyền cloud provider credential. Nếu trong lúc code phát hiện cần credential cho
`terraform apply` chạy được thật (gần như chắc chắn cần), **dừng lại và tham chiếu TASK-BE-FLEET-009**
(credential storage — cần security review) thay vì tự ý thêm 1 cơ chế lưu credential tạm bợ vào đây.

## Mục tiêu

Thêm interface `TerraformRunner` (port) + usecase `ApplyTerraformPlan` — gọi `terraform apply` trên 1 control
dev server đã đăng ký (qua agent), đọc `terraform output -json`, map sang danh sách instance tối thiểu
(`Host`) để `DeployFleetDefinition` (TASK-BE-FLEET-014) dùng.

## Files cần sửa

1. `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (MODIFY — thêm interface `TerraformRunner`)
2. `backend-go/services/infra-fleet-service/internal/usecase/apply_terraform_plan.go` (MỚI)
3. `backend-go/services/infra-fleet-service/internal/usecase/apply_terraform_plan_test.go` (MỚI)

## `ports.go` — thêm `TerraformRunner`

```go
// TerraformRunner runs `terraform apply` on a registered control dev
// server (via its agent) and returns `terraform output -json`'s raw
// content — CR-FLEET-002's Hướng A. Orca never runs terraform itself;
// it orchestrates the agent that does, mirroring how EphemeralVmRelay
// dispatches vm.provision instead of exec'ing locally.
type TerraformRunner interface {
	Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (outputJSON string, err error)
}
```

## `apply_terraform_plan.go`

```go
package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type ApplyTerraformPlanInput struct {
	ControlDevServerID string
	WorkingDir          string
	VarsFile            string
}

type TerraformInstance struct {
	Host string
}

type ApplyTerraformPlanResult struct {
	OutputJSON string
	Instances  []TerraformInstance
}

type ApplyTerraformPlan struct {
	devServers DevServerRepository
	runner     TerraformRunner
}

func NewApplyTerraformPlan(devServers DevServerRepository, runner TerraformRunner) *ApplyTerraformPlan {
	return &ApplyTerraformPlan{devServers: devServers, runner: runner}
}

func (uc *ApplyTerraformPlan) Execute(ctx context.Context, in ApplyTerraformPlanInput) (ApplyTerraformPlanResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	controlDevServer, err := uc.devServers.Get(ctx, tenantID, in.ControlDevServerID)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_UNKNOWN_CONTROL_HOST", "control dev server not found", err)
	}

	outputJSON, err := uc.runner.Apply(ctx, controlDevServer, in.WorkingDir, in.VarsFile)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInternal, "INFRA_TERRAFORM_APPLY_FAILED", "terraform apply failed", err)
	}

	instances, err := parseTerraformOutput(outputJSON)
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInternal, "INFRA_TERRAFORM_OUTPUT_PARSE_FAILED", "failed to parse terraform output", err)
	}
	return ApplyTerraformPlanResult{OutputJSON: outputJSON, Instances: instances}, nil
}

// parseTerraformOutput expects a `terraform output -json` document with a
// top-level output named "instance_hosts" of type list(string) — this is
// an Orca-imposed convention on the user-authored .tf module (CR-FLEET-002
// §"Không thuộc phạm vi": Orca does not generate HCL, but it must agree
// with the user on ONE output name to consume). Document this convention
// wherever the fleet YAML's provision.workingDir field is documented for
// end users — a .tf module missing this output fails ApplyTerraformPlan
// with INFRA_TERRAFORM_OUTPUT_PARSE_FAILED, not a silent empty result.
func parseTerraformOutput(outputJSON string) ([]TerraformInstance, error) {
	var raw struct {
		InstanceHosts struct {
			Value []string `json:"value"`
		} `json:"instance_hosts"`
	}
	if err := json.Unmarshal([]byte(outputJSON), &raw); err != nil {
		return nil, err
	}
	instances := make([]TerraformInstance, 0, len(raw.InstanceHosts.Value))
	for _, host := range raw.InstanceHosts.Value {
		instances = append(instances, TerraformInstance{Host: host})
	}
	return instances, nil
}
```

**Quyết định thiết kế bắt buộc trong task này (không để mở):** convention output tên `instance_hosts`
(`list(string)`) được chọn làm chuẩn — nếu team quyết định tên khác khi thực thi, đổi hằng số này ở 1 chỗ,
nhưng phải chọn 1 tên cụ thể trước khi merge, không để `parseTerraformOutput` là hàm rỗng/TODO.

## Test cases cần cover

- `TestApplyTerraformPlan_RequiresTenantContext`
- `TestApplyTerraformPlan_UnknownControlDevServer_ReturnsInvalidArgument`
- `TestApplyTerraformPlan_RunnerError_WrappedAsInternal`
- `TestParseTerraformOutput_ValidInstanceHosts_ReturnsInstances`
- `TestParseTerraformOutput_MissingInstanceHostsKey_ReturnsEmptyNotError` (thiếu key = 0 instance, không phải
  lỗi parse — chỉ JSON malformed mới là lỗi)
- `TestParseTerraformOutput_MalformedJSON_ReturnsError`

## Verify

```bash
cd backend-go/services/infra-fleet-service && go build ./... && go test ./internal/usecase/... -run ApplyTerraformPlan -v
gofmt -l internal/usecase/apply_terraform_plan.go internal/usecase/ports.go
```

## gitnexus

`TerraformRunner`/`ApplyTerraformPlan` là symbol mới — chạy `impact()` ngay sau khi tạo, trước khi
TASK-BE-FLEET-008 (RPC) hay TASK-BE-FLEET-014 (`DeployFleetDefinition`) thêm caller mới.

## Blocking

TASK-BE-FLEET-008 (RPC) và TASK-BE-FLEET-014 (`DeployFleetDefinition`) phụ thuộc cứng usecase này.
