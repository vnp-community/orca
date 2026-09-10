# BE-FLEET-SOL-002: `ApplyTerraformPlan` — IaC thật qua Terraform CLI orchestration (Hướng A)

> **🔲 Designed — chưa implement.** Phụ thuộc CR-FLEET-001 theo luồng dữ liệu (không bắt buộc merge trước).

**CR:** [CR-FLEET-002](../../../../../../docs/crs/v4/fleet-provisioning/CR-FLEET-002-iac-real-infrastructure-creation.md) — **Hướng A đã được Product Owner chốt 2026-09-09**, không còn "chờ business".
**Service:** `infra-fleet-service`, `agent` (RPC mới)
**TDD tham chiếu:** [`infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md)

---

## 0. Cảnh báo bảo mật — đọc trước khi thiết kế bất kỳ phần nào của solution này

CR gốc tự đánh giá rủi ro "Cao" cho phần **credential lưu trữ cho cloud provider** (AWS/GCP key) — đây là bề
mặt bảo mật hoàn toàn mới so với mô hình hiện tại của `backend-go` (chỉ cầm Vault SSH cert ngắn hạn, không
bao giờ cầm raw credential dài hạn của bất kỳ hệ thống nào). Solution này **giữ nguyên đánh giá rủi ro đó** —
phần credential storage ở §4 dưới đây được thiết kế ở mức tối thiểu (interface + luồng dữ liệu), **không đưa
ra schema lưu trữ cụ thể sẵn sàng code ngay** — cần security review riêng trước khi bất kỳ AI/dev nào bắt
đầu implement phần đó. Phần `TerraformRunner`/`ApplyTerraformPlan` (§2, §3 — orchestration, không cầm
credential) **an toàn để task-hoá bình thường**.

## 1. Re-verify code thật

- `internal/usecase/ephemeral_vm_relay.go` (`EphemeralVmRelay`) — xác nhận đây là cơ chế "tạo" gần nhất, dùng
  `VmProvisionParams{RepoPath, Command, RecipeID, RuntimeID}` (`ports.go:401-406`) — `Command` là 1 chuỗi shell
  tuỳ ý, không có khái niệm plan/apply/state như Terraform. Khớp CR §2.
- `agent/src/relay/agent-ephemeral-vm-handler.ts` — có thật, dòng 34 import `runRecipeCommand` từ
  `agent/src/shared/ephemeral-vm-recipe-process.ts` (dòng 23, `export async function runRecipeCommand(args: {...`)
  — `handleVmProvision` (dòng 147) gọi hàm này (dòng 163). Khớp CR — runner có thể tái dùng cho
  `terraform.apply`.
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:196` — `StreamVmProvision` xác nhận là RPC
  server-streaming duy nhất cho "N bước tuần tự cần progress" hiện có — convention đúng để tái dùng cho
  `ApplyTerraformPlan`.
- Không tìm thấy `*.tf`, Terraform provider Go SDK import, hay cloud-init template nào trong `backend-go/`
  hay `agent/` liên quan tới fleet provisioning — xác nhận đúng CR §3 ("0% code hiện có").
- **`agent-terraform-handler.ts` CHƯA tồn tại** — đúng như CR ghi "Changes Required" (file `[NEW]`).

**Kết luận:** không có gì lệch — CR-FLEET-002's Hướng A vẫn đúng thực trạng codebase, cụ thể hoá thành thiết
kế bên dưới.

## 2. `TerraformRunner` — port interface, agent thực thi qua `terraform.apply`

```go
// backend-go/services/infra-fleet-service/internal/usecase/ports.go — thêm cạnh EphemeralVmSshProvisioner
// TerraformRunner chạy `terraform apply` trên 1 control host đã đăng ký
// (dev server có agent, KHÔNG phải Orca server tự exec — giữ nguyên
// nguyên tắc "Orca server không exec lệnh tuỳ ý" hiện có toàn hệ thống,
// mirror cách EphemeralVmRelay gọi vm.provision qua agent thay vì tự
// chạy shell trên chính nó).
type TerraformRunner interface {
	Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (outputJSON string, err error)
}
```

Implement (`internal/adapter/agentrelay/terraform_runner.go`, tên package minh hoạ — chọn đúng theo convention
adapter hiện có khi implement thật, ví dụ cạnh `sshrelay`/`backendrelaysshprovisioner`) gọi RPC agent mới qua
kênh agent<->Orca đã có (cùng cơ chế `vm.exec`/`vm.provision` dùng, KHÔNG mở kênh mới):

```go
func (r *AgentTerraformRunner) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	resp, err := r.agentClient.Call(ctx, controlDevServer, "terraform.apply", map[string]any{
		"workingDir": workingDir,
		"varsFile":   varsFile,
	})
	if err != nil {
		return "", err
	}
	return resp.OutputJSON, nil
}
```

**`r.agentClient.Call`'s tên method/shape thật phải xác nhận lại tại thời điểm implement** — solution này
không giả định tên chính xác của client hiện có (`vm.exec` dùng qua kênh nào, ví dụ có thể là
`DevServerAgentClient.Exec` như `TASK-BE-STORAGE-012` đã dùng cho best-effort notify) — đọc lại code thật
trước khi khoá tên method.

## 3. Usecase `ApplyTerraformPlan`

```go
// backend-go/services/infra-fleet-service/internal/usecase/apply_terraform_plan.go
type ApplyTerraformPlanInput struct {
	ControlDevServerID string // dev server đã đăng ký, có agent — chạy terraform trên đó
	WorkingDir          string
	VarsFile            string
}

type TerraformInstance struct {
	Host string
	// Các field khác (region, instance_type...) tuỳ `terraform output` thật
	// của module user viết — KHÔNG giả định shape cố định ở solution này,
	// vì user tự viết .tf (CR §"Không thuộc phạm vi" mục 2: Orca không
	// sinh HCL, không kiểm soát output schema).
}

type ApplyTerraformPlanResult struct {
	OutputJSON string               // raw `terraform output -json`, giữ lại để audit/debug
	Instances  []TerraformInstance  // parse tối thiểu (chỉ field cần cho FleetSpecServer.Host)
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

	instances, err := parseTerraformOutput(outputJSON) // helper riêng — mapping cụ thể phụ thuộc convention output user chọn, xem "Không thuộc phạm vi"
	if err != nil {
		return ApplyTerraformPlanResult{}, apperrors.New(apperrors.KindInternal, "INFRA_TERRAFORM_OUTPUT_PARSE_FAILED", "failed to parse terraform output", err)
	}
	return ApplyTerraformPlanResult{OutputJSON: outputJSON, Instances: instances}, nil
}
```

`parseTerraformOutput` cần 1 convention rõ ràng cho `output` block trong `.tf` của user (ví dụ bắt buộc 1
output tên `instance_hosts` kiểu `list(string)`) — **đây là 1 quyết định thiết kế cần chốt khi task-hoá**,
không đoán ở solution này (rủi ro nếu đoán sai: `ApplyTerraformPlan` không parse được output của mọi user).

## 4. Credential storage cho cloud provider — CHỈ thiết kế luồng, KHÔNG khoá schema (an ninh Cao)

Theo cảnh báo ở §0: phần này **không đủ cụ thể để task-hoá thành code ngay** — task tương ứng
(xem `tasks/README.md`) phải là "thiết kế + security review", không phải "code theo schema có sẵn".

Ràng buộc thiết kế đã biết (từ CR):

1. **Không tái dùng `domain.SshTarget.VaultSSHRole`** — đây là loại secret khác hẳn (Vault SSH secrets engine
   role vs. cloud provider IAM key/service-account JSON).
2. Terraform CLI cần credential trong **environment của tiến trình `terraform apply`** chạy trên control host
   (qua agent) — nghĩa là secret phải đi từ nơi lưu trữ (chưa quyết định) → agent → biến môi trường tiến
   trình `terraform`, và **không được log/persist ở agent** (mirror nguyên tắc đã áp dụng cho
   `ReadCredentialFile`'s "never log contentPEM", `ports.go:389`).
3. Ứng viên lưu trữ (liệt kê để cân nhắc, **chưa chọn**): (a) Vault KV engine riêng (khác SSH secrets engine)
   — nhất quán với việc hệ thống đã có Vault client; (b) 1 bảng Postgres mới với encryption-at-rest
   (cần xác nhận cơ chế encryption hiện có của `infra-fleet-service`, nếu có, trước khi tự chọn — không tự
   sinh 1 cơ chế mã hoá mới).
4. **Bắt buộc security review trước khi chọn 1 trong các ứng viên trên** — đây là quyết định vượt phạm vi
   1 solution kỹ thuật thông thường.

## 5. RPC `ApplyTerraformPlan` — stream, tái dùng convention `StreamVmProvision`

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto
rpc ApplyTerraformPlan(ApplyTerraformPlanRequest) returns (stream ApplyTerraformPlanEvent);

message ApplyTerraformPlanRequest {
  string control_dev_server_id = 1;
  string working_dir = 2;
  string vars_file = 3;
}

message ApplyTerraformPlanEvent {
  string type = 1; // "stdout" | "stderr" | "result" | "error" — mirror VmProvisionEvent's Type convention
  string chunk = 2;
  string output_json = 3; // populated khi type == "result"
  string error_msg = 4;
}
```

## 6. Agent RPC `terraform.apply` — tái dùng `runRecipeCommand`, không viết process runner mới

```ts
// agent/src/relay/agent-terraform-handler.ts [NEW]
// Reuses runRecipeCommand (ephemeral-vm-recipe-process.ts) — same process
// runner agent-ephemeral-vm-handler.ts's handleVmProvision already uses,
// vì "chạy 1 binary bên ngoài, capture stdout/stderr/exit code" là đúng
// hình dạng bài toán terraform apply cần — không có lý do viết runner mới.
import { runRecipeCommand } from '../shared/ephemeral-vm-recipe-process'

export async function handleTerraformApply(params: { workingDir: string; varsFile: string }) {
  const result = await runRecipeCommand({
    command: `terraform apply -var-file=${params.varsFile} -auto-approve`,
    cwd: params.workingDir,
    // ...các tham số khác runRecipeCommand yêu cầu — đọc lại chữ ký hàm thật
    // (dòng 23 của ephemeral-vm-recipe-process.ts) trước khi khoá call site,
    // KHÔNG đoán tham số.
  })
  // Sau apply thành công: chạy `terraform output -json`, trả cả 2 kết quả
  // về Orca (hoặc để usecase gọi apply/output như 2 lệnh riêng — quyết định
  // khi task-hoá, xem tasks/TASK-BE-FLEET-*).
}
```

**Version của binary `terraform`/`tofu` trên control host không kiểm soát được** — theo đúng nguyên tắc
`docs/reference/git-compatibility.md` áp dụng tương tự cho binary bên thứ 3 chạy trên host SSH remote: cần 1
capability probe tối thiểu (`terraform version`) trước lần `apply` đầu, cache theo host — KHÔNG giả định 1
version cố định.

## 7. Backward-compat schema YAML

```ts
// frontend/src/shared/fleet-config-parser.ts — FleetConfigSchema, thêm optional
const FleetConfigSchema = z.object({
  // ...trường đã có...
  provision: z.object({
    iac: z.literal('terraform'),
    workingDir: z.string(),
    varsFile: z.string().optional(),
  }).optional(),
})
```

Optional-thêm-mới — `orca-fleet.yaml` không có `provision` vẫn hoạt động y hệt trước solution này.

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Cloud provider credential storage | **Cao** | Chưa thiết kế xong (đúng ý — cần security review, xem §4) |
| Terraform state file | Cao | Vận hành (S3/GCS backend/Terraform Cloud) — ngoài phạm vi code, cần tài liệu vận hành riêng trước khi dùng thật |
| `terraform`/`tofu` version trên control host | Trung bình | Cần capability probe, không giả định version cố định |
| `parseTerraformOutput`'s convention output | Trung bình | Cần chốt 1 convention output tên cụ thể trước khi code — nếu không chốt, mỗi user `.tf` khác nhau sẽ parse sai |
| Trùng lặp UX với Ephemeral VM (F18) | Trung bình | Cần tài liệu phân biệt rõ (dài hạn/fleet vs ngắn hạn/per-worktree) |

## Không thuộc phạm vi solution này

- Multi-cloud abstraction (mục 1, CR gốc).
- Tự sinh HCL từ YAML (mục 2) — user tự viết `.tf`.
- `terraform destroy`/lifecycle đầy đủ, `plan`-review trước `apply` (mục 3).
- Sửa `BulkProvisionFleet` (CR-FLEET-001) — solution này chỉ thêm 1 bước phía trước, gọi lại usecase đó
  nguyên trạng.
- Lưu `FleetDefinition`/kết quả vào DB, export YAML, deploy lặp lại — xem
  [BE-FLEET-SOL-003](./BE-FLEET-SOL-003-fleet-definition-persistence.md).
- **Chọn schema/nơi lưu credential cụ thể** (§4) — cần security review riêng, không tự quyết ở solution kỹ
  thuật này.

## Liên quan

- `backend-go/services/infra-fleet-service/internal/usecase/ephemeral_vm_relay.go` (tham khảo cấu trúc, không sửa)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:401-468` (`VmProvisionParams`, `EphemeralVmSshProvisioner`)
- `agent/src/relay/agent-ephemeral-vm-handler.ts:34,147,163` (`runRecipeCommand`, `handleVmProvision`)
- `agent/src/shared/ephemeral-vm-recipe-process.ts:23` (`runRecipeCommand`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:196` (`StreamVmProvision`, convention tái dùng)
- `docs/reference/git-compatibility.md` (nguyên tắc capability-cache áp dụng tương tự cho `terraform` binary)
- [BE-FLEET-SOL-001](./BE-FLEET-SOL-001-bulk-provision-from-yaml.md) — phụ thuộc theo luồng dữ liệu (`FleetSpecServer`, `BulkProvisionFleet`)
- [BE-FLEET-SOL-003](./BE-FLEET-SOL-003-fleet-definition-persistence.md) — phụ thuộc ngược (dùng kết quả CR này)
