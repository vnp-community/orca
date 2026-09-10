# CR-FLEET-002 — IaC thật: tạo hạ tầng mới (không chỉ đăng ký host có sẵn)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FLEET-002 |
| **Tên** | Tích hợp Infrastructure-as-Code thật (Terraform/cloud-init) cho fleet provisioning — phân biệt rõ với "đăng ký host đã tồn tại" |
| **Loại** | Feature (Gap Fix) |
| **Priority** | 🟠 P1 — **kích hoạt**: Product Owner xác nhận nhu cầu + chọn **Hướng A** (Terraform CLI orchestration), 2026-09-09 |
| **Effort** | Large (2–3 tuần cho MVP 1 cloud provider; XL nếu multi-cloud) |
| **Phiên bản** | v1.1 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai (Proposed) — sẵn sàng bắt đầu, không còn chờ business |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu hoàn thiện F31 ở `backend-go`/`agent` |
| **Tác động HLD** | C3.5 (Fleet Provisioning), C3.13 (Dev Server Agent) |
| **Tác động Features** | F31 (Fleet Inventory & Bulk Provisioning), liên quan F18 (Ephemeral VM) |
| **Phụ thuộc** | SAU CR-FLEET-001 (bulk-register cần có trước khi "tạo rồi tự đăng ký" có ý nghĩa); nối tiếp bởi [CR-FLEET-003](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md) (lưu định nghĩa hạ tầng vào DB, export YAML, triển khai) |

---

## Bối cảnh & Vấn đề

### 1. "IaC" trong spec F31 nghĩa là gì, và tại sao chưa có

`docs/features/F31-fleet-provisioning.md` mở đầu bằng: *"Orca hỗ trợ quản lý fleet dev servers theo mô hình **Infrastructure-as-Code** — khai báo danh sách server trong file YAML..."*. Đọc kỹ toàn bộ file thì **"IaC" ở đây chỉ có nghĩa là "khai báo danh sách host đã tồn tại sẵn dưới dạng YAML"**, không có bất kỳ mục nào mô tả việc YAML định nghĩa loại instance, region, image, network — tức không có Terraform/CloudFormation/Ansible/cloud-init resource definition nào trong spec gốc. `orca-fleet.yaml`'s `servers[]` (dòng 46-63 của spec) chỉ có `host`, `project`, `team`, `environment`, `repos` — tất cả giả định server **đã chạy sẵn**, Orca chỉ SSH vào và bootstrap phần mềm.

Vì vậy gap "IaC thực sự" mà audit trước nêu ra là chính xác nhưng cần làm rõ ranh giới: đây không phải "F31 có 1 phần chưa xong", mà là **"F31 gốc chưa bao giờ đặc tả IaC theo nghĩa tạo hạ tầng mới — nếu muốn tính năng này, đó là mở rộng phạm vi F31, không phải hoàn thiện nó"**. CR này ghi nhận rõ điều đó và đề xuất phương án tối thiểu-rủi ro nếu business quyết định cần.

### 2. Cái gần nhất trong codebase với "tạo hạ tầng mới" là Ephemeral VM (F18) — nhưng đó KHÔNG phải Terraform/cloud-init có cấu trúc

`backend-go/services/infra-fleet-service/internal/usecase/ephemeral_vm_relay.go` (`EphemeralVmRelay`) + `agent/src/relay/agent-ephemeral-vm-handler.ts` là cơ chế thật duy nhất trong repo hiện "tạo" một máy/mục tiêu mới thay vì chỉ đăng ký máy có sẵn:

- `internal/usecase/ports.go:401-406` (`VmProvisionParams{RepoPath, Command, RecipeID, RuntimeID}`) — `Command` là **1 chuỗi shell tuỳ ý** do user/recipe author viết (vd. `docker run ...`, hoặc 1 CLI cloud provider như `aws ec2 run-instances`/`gcloud compute instances create`), chạy trên agent của **1 dev server nguồn đã có sẵn** (`sourceDevServer`, xem `EphemeralVmSshProvisioner`'s doc comment, dòng 449-459).
- `agent/src/relay/agent-ephemeral-vm-handler.ts:1-24` (doc comment) xác nhận: `vm.provision` chạy `runRecipeCommand` — tái dùng process runner của "desktop-local exec" — nghĩa là **thực thi lệnh shell thô, không parse/validate resource graph nào**. Không có khái niệm plan/apply/state-file như Terraform, không có idempotency-by-declaration như cloud-init.
- Kết quả `VmProvisionResult{Type: "orca-server"|"ssh", PairingCode, ProjectRoot, SshTarget}` (`ports.go:430-435`) chỉ mô tả "làm sao Orca nối vào máy vừa tạo", không mô tả **máy đó được tạo bằng công cụ gì** — hoàn toàn phụ thuộc vào nội dung `command` mà recipe author tự viết, ngoài tầm kiểm soát/validate của `infra-fleet-service`.

Đây là 1 cơ chế "bring-your-own-script", đủ dùng cho 1 VM tại 1 thời điểm (Ephemeral VM per-worktree), nhưng **không phải "IaC cho fleet"**: không khai báo N host cùng lúc, không có state reconciliation (biết được host nào đã tạo, host nào cần xoá khi YAML đổi), không multi-cloud abstraction có cấu trúc.

### 3. Không có Terraform/Ansible/cloud-init provider nào trong repo

Xác nhận qua khảo sát trực tiếp: không tìm thấy `*.tf`, HCL, Terraform provider Go SDK import, hay cloud-init template (`#cloud-config`) nào trong `backend-go/` hay `agent/` liên quan tới fleet provisioning. Duy nhất các file `.tf`/CI liên quan tới hạ tầng **triển khai chính Orca** (nếu có, ví dụ Docker/K8s manifest cho self-hosted Orca server) — khác hoàn toàn phạm vi "provision fleet dev servers cho end-user".

---

## Giải pháp đề xuất

### Quyết định kiến trúc bắt buộc trước khi code: KHÔNG tự viết provisioner cho từng cloud

Viết provisioner riêng cho AWS/GCP/Azure/DigitalOcean... (gọi SDK trực tiếp, tự quản lý credential, tự retry, tự đồng bộ trạng thái) là công việc tương đương xây lại một phần Terraform — effort rất lớn, rủi ro bảo trì cao, và trùng lặp với công cụ đã trưởng thành. Đề xuất 1 trong 2 hướng, **chọn 1** theo quyết định business:

#### Hướng A (khuyến nghị) — Orca gọi `terraform`/`tofu` CLI như 1 external process, không tự viết cloud SDK integration

```
orca-fleet.yaml (mở rộng, KHÔNG phá vỡ schema hiện có)
  ↓
provision:
  iac: terraform
  workingDir: deploy/terraform/dev-fleet/     # user tự viết .tf, Orca không sinh HCL hộ user
  varsFile: dev-fleet.tfvars.json             # sinh từ fleet YAML's servers[] (count, instance_type, region...)
  ↓
BulkProvisionFleet (CR-FLEET-001) mở rộng thêm 1 pre-step tuỳ chọn:
  1. Nếu spec.provision.iac == "terraform": chạy `terraform apply -var-file=... -auto-approve`
     trong workingDir, qua agent trên "control host" (không phải target host — target
     host CHƯA TỒN TẠI trước bước này) — hoặc qua 1 CI job ngoài Orca hoàn toàn, xem
     "Không thuộc phạm vi" §2 cho lựa chọn giữa 2 cách này.
  2. Đọc `terraform output -json` → lấy IP/hostname của các instance vừa tạo.
  3. Map kết quả sang FleetSpec (CR-FLEET-001) → gọi BulkProvisionFleet như bình thường
     (từ đây trở đi, luồng giống hệt "host đã tồn tại" — không phân biệt nữa).
```

**Vì sao chọn hướng này:** Orca không cần hiểu AWS/GCP/Azure API — Terraform provider (đã có sẵn, cộng đồng duy trì) làm việc đó. Orca chỉ là 1 orchestrator gọi CLI + đọc output — đúng tinh thần "không tự viết provisioner cho từng cloud" mà brief yêu cầu cân nhắc.

**Rủi ro cần warn business:** Orca (chạy trên máy nào đó — dev server nguồn, hoặc chính Orca server) cần quyền chạy `terraform`/`tofu` + credential cloud provider (AWS/GCP keys) — đây là bề mặt bảo mật hoàn toàn mới so với mô hình hiện tại (Orca chỉ cần SSH cert từ Vault, không bao giờ cầm cloud provider credential). Cần thiết kế lưu trữ credential riêng (không tái dùng Vault SSH role hiện có — khác loại secret).

#### Hướng B — cloud-init only, không dùng Terraform (nếu chỉ cần "khai báo cấu hình OS", không cần "tạo instance mới")

Nếu nhu cầu thật chỉ là "khi VM đã được tạo sẵn (qua bất kỳ cách nào, ngoài Orca), tự động cấu hình nó theo YAML khai báo" — thì không cần Terraform, chỉ cần render 1 `#cloud-config` từ `orca-fleet.yaml`'s `bootstrap` section (đã có schema sẵn: `FleetServerBootstrapSchema`, `frontend/src/shared/fleet-config-parser.ts:23-34` — `repos`, `setupScript`) và cấp nó cho cloud provider lúc tạo instance (ngoài phạm vi Orca). Đây **không phải "tạo hạ tầng"**, chỉ là "chuẩn hoá bootstrap script đã có thành cloud-init format chuẩn" — effort nhỏ hơn nhiều Hướng A, nhưng không giải quyết đúng nghĩa "IaC tạo N host mới" mà audit gốc nêu.

### Quyết định (2026-09-09)

**Hướng A được chọn** — Product Owner xác nhận nhu cầu "khởi tạo hạ tầng thật ở `backend-go`" (không chỉ chuẩn hoá bootstrap script cho máy có sẵn), khớp đúng ý nghĩa "IaC tạo hạ tầng mới" mà audit gốc đặt vấn đề. MVP làm cho **đúng 1 cloud provider** (khuyến nghị AWS vì Terraform AWS provider trưởng thành nhất) trước khi tính multi-cloud (xem "Không thuộc phạm vi" §1).

Quyết định này còn kéo theo yêu cầu mới từ Product Owner: **kết quả khởi tạo hạ tầng (và định nghĩa fleet dùng để khởi tạo) phải được lưu vào DB** của `backend-go` làm nguồn sự thật — không chỉ nhận YAML tạm thời qua 1 lần gọi RPC rồi quên. Từ đó hệ thống hỗ trợ xuất lại YAML (từ DB) và triển khai (deploy) lặp lại/từ bản đã lưu. Yêu cầu này **vượt phạm vi kỹ thuật của CR này** (CR-FLEET-002 chỉ lo việc "gọi `terraform apply` và đọc kết quả") — xem CR mới **[CR-FLEET-003](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md)**, xây dựng ngay trên kết quả `BulkProvisionFleetEvent`/`terraform output` mà CR này tạo ra, để lưu định nghĩa + kết quả vào bảng `infra.fleet_definitions` và cung cấp `ExportFleetDefinitionYaml`/`DeployFleetDefinition`.

---

## Changes Required (nếu Hướng A được chọn — MVP 1 provider)

| File | Thay đổi |
|------|---------|
| `backend-go/services/infra-fleet-service/internal/usecase/apply_terraform_plan.go` | [NEW] usecase gọi `terraform apply` qua 1 `CommandRunner` port (tương tự cách `EphemeralVmRelay` gọi `vm.provision` qua agent — KHÔNG chạy `terraform` trực tiếp trên Orca server, chạy qua agent trên 1 dev server đã đăng ký làm "control host", giữ đúng nguyên tắc "Orca server không exec lệnh tuỳ ý" hiện có toàn hệ thống) |
| `backend-go/services/infra-fleet-service/internal/usecase/ports.go` | [NEW] `TerraformRunner` interface — `Apply(ctx, workingDir, varsFile) (outputJSON string, err error)`, implement qua agent RPC mới (xem dòng dưới) |
| `agent/src/relay/agent-terraform-handler.ts` | [NEW] RPC `terraform.apply` — tái dùng đúng pattern `runRecipeCommand` (`agent/src/shared/ephemeral-vm-recipe-process.ts`) đã có cho `vm.provision`, KHÔNG viết process runner mới |
| `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` | [NEW] `rpc ApplyTerraformPlan` — stream tương tự `StreamVmProvision` (dòng 196), tái dùng convention stdout/stderr/result đã có |
| `frontend/src/shared/fleet-config-parser.ts` | `FleetConfigSchema` thêm field tuỳ chọn `provision: { iac: 'terraform', workingDir: string, varsFile?: string }` — optional, không phá schema hiện có (backward-compat với `orca-fleet.yaml` không có IaC) |
| Credential storage mới (chưa có adapter — cần 1 CR bảo mật riêng nếu Hướng A được chọn) | Lưu cloud provider credential (AWS/GCP key) — KHÔNG tái dùng Vault SSH role (`domain.SshTarget.VaultSSHRole`), đây là loại secret khác hẳn |
| `docs/features/F31-fleet-provisioning.md` | Làm rõ lại đoạn mở đầu: đổi "Infrastructure-as-Code" (dễ hiểu nhầm là tạo hạ tầng) thành mô tả chính xác hơn cho phần đã có ("Infrastructure Inventory as Code" hoặc tương tự) — tách riêng phần IaC thật (nếu CR này được duyệt) |

---

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Cloud provider credential mới trong hệ thống | Cao | Bề mặt bảo mật hoàn toàn khác Vault SSH cert hiện có — cần threat-model riêng trước khi code, không chỉ "thêm 1 field config" |
| Terraform state file quản lý ở đâu | Cao | Local state trên control host = mất state nếu host chết; cần S3/GCS backend hoặc Terraform Cloud — quyết định vận hành, không phải chi tiết code |
| Chạy `terraform apply` qua agent (SSH use case) | Trung bình | Theo AGENTS.md's "SSH Use Case" — control host có thể là 1 dev server SSH remote (không phải máy local Orca chạy) → phải test cả native/WSL/SSH host theo đúng ràng buộc Git Binary Compatibility tương tự (dù đây là `terraform`, không phải `git`, cùng nguyên tắc: version của `terraform` binary trên control host có thể khác version Orca test với — cần capability probe tương tự `GitCapabilityCache`, không giả định 1 version cố định) |
| Không tương thích ngược `orca-fleet.yaml` hiện có | Thấp | `provision` field là optional-thêm-mới, không đổi field cũ |
| Trùng lặp với Ephemeral VM (F18) | Trung bình | Cần làm rõ với user: fleet IaC (dài hạn, dev server cố định) khác ephemeral VM (ngắn hạn, per-worktree) — tránh 2 tính năng na ná nhau gây nhầm lẫn UX |

---

## Không thuộc phạm vi CR này

1. **Multi-cloud abstraction** (AWS + GCP + Azure cùng lúc qua 1 schema thống nhất) — MVP chỉ 1 provider, mở rộng là CR riêng sau khi có dữ liệu sử dụng thật.
2. **Tự sinh HCL Terraform config từ YAML.** User tự viết `.tf`, Orca chỉ truyền `.tfvars` + gọi `apply`/đọc `output` — không có "Terraform config generator" trong phạm vi CR này (rủi ro rất cao nếu tự sinh HCL sai và tạo/xoá nhầm hạ tầng thật).
3. **Terraform destroy / lifecycle quản lý đầy đủ (drift detection, `plan` review trước khi `apply`).** CR này chỉ làm `apply` (create) tối thiểu; `destroy`/`plan`-review UI là mở rộng sau khi Hướng A dùng thật.
4. **CR-FLEET-001's bulk-register logic** — CR này chỉ thêm 1 bước "tạo host mới" PHÍA TRƯỚC luồng bulk-register đã có, không đổi lại usecase đó.
5. **Lưu định nghĩa fleet/kết quả tạo hạ tầng vào DB, export YAML, deploy lặp lại từ bản đã lưu** — đây là yêu cầu tiếp theo của Product Owner, xử lý ở [CR-FLEET-003](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md) (phụ thuộc CR này đã merge trước).

---

## Tiêu chí chấp nhận

- [x] Product owner xác nhận nhu cầu + chọn **Hướng A** bằng văn bản (2026-09-09).
- [ ] `orca-fleet.yaml` với `provision.iac: terraform` chạy `terraform apply` qua agent trên 1 control host đã đăng ký, output N instance mới
- [ ] Kết quả `terraform output -json` map đúng sang `FleetSpec` (CR-FLEET-001) — instance mới tự động vào luồng bulk-register hiện có, không cần thêm bước thủ công
- [ ] Cloud provider credential được lưu trữ tách biệt khỏi Vault SSH role, có encryption-at-rest tương đương chuẩn hiện có cho secret khác trong hệ thống
- [ ] `orca-fleet.yaml` KHÔNG có field `provision` vẫn hoạt động y hệt trước CR này (backward-compat)
- [ ] Tài liệu vận hành ghi rõ Terraform state được lưu ở đâu, ai chịu trách nhiệm backup/lock state

---

## Impact analysis (gitnexus)

**Chưa chạy** — CR này đề xuất symbol hoàn toàn mới (`ApplyTerraformPlan`, `TerraformRunner`, `agent-terraform-handler.ts`), chưa tồn tại trong codebase nên không có gì để `impact()`. Symbol gần nhất có thể ảnh hưởng gián tiếp nếu Hướng A được chọn và implement:

| Symbol liên quan (đã tồn tại, KHÔNG sửa trong CR này) | Vì sao liên quan |
|---|---|
| `EphemeralVmRelay` (`internal/usecase/ephemeral_vm_relay.go`) | Cùng pattern "agent chạy 1 command tuỳ ý rồi map kết quả" — nếu implement, nên tham khảo cấu trúc usecase này, KHÔNG kế thừa/sửa trực tiếp |
| `runRecipeCommand` (`agent/src/shared/ephemeral-vm-recipe-process.ts`) | Process runner có thể tái dùng cho `terraform.apply` handler thay vì viết runner mới — cần `impact()` trước khi quyết định tái dùng hay tách riêng, vì đây là runner dùng chung nhiều recipe khác |

Khi CR này chuyển sang triển khai thật, bắt buộc chạy `impact()` cho `runRecipeCommand` (nếu tái dùng) và mọi symbol usecase mới ngay khi tạo, theo đúng CLAUDE.md.

---

## Liên quan

- [F31-fleet-provisioning.md](../../../features/F31-fleet-provisioning.md)
- [CR-FLEET-001-bulk-provision-from-yaml.md](./CR-FLEET-001-bulk-provision-from-yaml.md) — phụ thuộc thứ tự (làm trước)
- [CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md) — phụ thuộc thứ tự (làm sau CR này), lưu definition/kết quả vào DB + export YAML + deploy lặp lại
- `backend-go/services/infra-fleet-service/internal/usecase/ephemeral_vm_relay.go` (F18, cơ chế "tạo VM" gần nhất hiện có)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:401-468` (`VmProvisionParams`, `EphemeralVmSshProvisioner`)
- `agent/src/relay/agent-ephemeral-vm-handler.ts` (doc comment giải thích `vm.provision` là shell command tuỳ ý)
- `frontend/src/shared/fleet-config-parser.ts:23-34` (`FleetServerBootstrapSchema` — gần nhất với "cloud-init" hiện có, chỉ thiếu phần "tạo instance")
- `docs/reference/git-compatibility.md` — tham khảo nguyên tắc capability-cache/version-boundary cho binary bên thứ 3 chạy trên host không kiểm soát version (áp dụng tương tự cho `terraform` binary nếu Hướng A triển khai)
