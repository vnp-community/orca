# CR-FLEET-003 — Fleet/Infra definition làm nguồn sự thật trong `backend-go`: lưu DB, export YAML, triển khai

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FLEET-003 |
| **Tên** | Lưu định nghĩa fleet/infra vào Postgres (`infra-fleet-service`) làm nguồn sự thật — export YAML + deploy lặp lại từ bản đã lưu |
| **Loại** | Feature |
| **Priority** | 🟠 P1 (yêu cầu trực tiếp Product Owner, 2026-09-09) |
| **Effort** | Large (1.5–2 tuần, không tính effort CR-FLEET-001/002 mà CR này phụ thuộc) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai (Proposed) |
| **Tác giả** | Theo yêu cầu trực tiếp Product Owner ("F31 hỗ trợ khởi tạo hạ tầng ở backend-go và lưu DB → từ đó hỗ trợ xuất YAML và triển khai"), mở rộng CR-FLEET-001/002 — 2026-09-09 |
| **Tác động HLD** | C3.5 (Fleet Provisioning) |
| **Tác động Features** | F31 (Fleet Inventory & Bulk Provisioning) |
| **Phụ thuộc** | **Cứng vào [CR-FLEET-001](./CR-FLEET-001-bulk-provision-from-yaml.md)** (`FleetSpecServer`, `BulkProvisionFleet`) và **[CR-FLEET-002](./CR-FLEET-002-iac-real-infrastructure-creation.md)** (`ApplyTerraformPlan`, Terraform output) — cả hai phải tồn tại trước, CR này chỉ persist + điều phối lại, không viết logic provisioning mới |

---

## Bối cảnh & Vấn đề

### 1. Sau CR-FLEET-001/002, fleet definition vẫn hoàn toàn ephemeral

Theo đúng thiết kế đã đề xuất ở CR-FLEET-001 (§"YAML → `FleetSpec` mapping"): *"Việc parse YAML → `FleetSpec` nằm ở tầng gọi RPC (frontend hoặc CLI)... `infra-fleet-service` chỉ nhận `FleetSpec` đã parse+validate"* — nghĩa là mỗi lần gọi `BulkProvisionFleet`/`ApplyTerraformPlan` (CR-FLEET-002), backend-go dùng dữ liệu request 1 lần rồi quên, không lưu lại. Hệ quả trực tiếp:

- Không có nơi tra cứu "lần provision trước dùng definition nào" ngoài log RPC (nếu có bật, và không phải data model truy vấn được).
- Không "deploy lại" được 1 fleet definition đã dùng nếu client (trình duyệt/CLI) không còn giữ file YAML gốc.
- Không có 1 bản ghi để review/approve trước khi apply — đặc biệt quan trọng với CR-FLEET-002 (tạo hạ tầng thật, tốn tiền, khó hoàn tác) so với CR-FLEET-001 (chỉ đăng ký host có sẵn).
- Không hỗ trợ luồng GitOps 2 chiều: "sửa trên UI Orca → xuất YAML → commit vào 1 repo riêng để review" hoặc ngược lại "sửa YAML trong git → import lại vào Orca để deploy".

### 2. Quyết định Product Owner (2026-09-09)

Đi kèm quyết định chọn Hướng A ở [CR-FLEET-002](./CR-FLEET-002-iac-real-infrastructure-creation.md) (Terraform CLI orchestration — "khởi tạo hạ tầng thật ở `backend-go`"), Product Owner yêu cầu thêm: **kết quả khởi tạo hạ tầng phải lưu vào DB**, từ đó hệ thống hỗ trợ **xuất lại YAML** và **triển khai (deploy)** lặp lại từ bản đã lưu — không dừng ở "gọi 1 lần rồi quên". CR này hiện thực hoá đúng yêu cầu đó, xây trên nền `FleetSpec`/`BulkProvisionFleet` (CR-FLEET-001) và `ApplyTerraformPlan` (CR-FLEET-002) — không thay thế, không viết lại 2 cơ chế đó.

---

## Giải pháp đề xuất

### 1. Domain entity mới: `FleetDefinition`

```go
// backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go
type FleetDefinition struct {
    ID        string
    TenantID  string
    Name      string            // định danh do user đặt, unique theo tenant
    Version   int               // tăng dần mỗi lần Update — lịch sử tối thiểu (xem "Không thuộc phạm vi" §1 nếu cần audit trail đầy đủ)
    Servers   []FleetSpecServer // TÁI DÙNG struct đã định nghĩa ở CR-FLEET-001's bulk_provision_fleet.go — KHÔNG định nghĩa lại
    Provision *ProvisionConfig  // optional — mirror CR-FLEET-002's `provision.iac` config; nil nếu definition chỉ đăng ký host có sẵn, không tạo hạ tầng mới
    CreatedBy string
    CreatedAt, UpdatedAt time.Time
}

type ProvisionConfig struct {
    IaC        string // "terraform" — khớp CR-FLEET-002's FleetConfigSchema.provision.iac
    WorkingDir string
    VarsFile   string
}
```

`FleetDefinition.Servers` chỉ là **bản lưu trữ** của `FleetSpecServer` (CR-FLEET-001) — tránh 2 định nghĩa song song cho cùng khái niệm "1 server trong fleet".

### 2. Migration mới

```sql
-- backend-go/services/infra-fleet-service/migrations/0018_fleet_definitions.up.sql
-- Số thứ tự tiếp theo CR-FLEET-001's đề xuất 0017 (bản thân số đó cũng cần verify lại
-- tại thời điểm implement — repo thật đã có 0016_ephemeral_vm_ssh_target_host_key,
-- không phải 0015 như CR-FLEET-001 giả định lúc viết; xem migrations/ thư mục thật
-- ngay trước khi thêm file mới, tránh trùng số lần nữa).
CREATE TABLE infra.fleet_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    servers JSONB NOT NULL,   -- []FleetSpecServer, serialize trực tiếp
    provision JSONB,          -- ProvisionConfig, nullable
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);
```

**Lưu ý chéo với CR-DB-002/003 (F26, multi-dialect đã được Product Owner kích hoạt cùng ngày):** nếu `infra-fleet-service` nằm trong danh sách service pilot multi-dialect tại thời điểm migration này chạy thật, cột `JSONB` phải đổi sang kiểu dialect-safe (`JSON`/`TEXT`) theo capability layer của CR-DB-002 — không hard-code `JSONB` mà không kiểm tra lại, tránh tạo thêm 1 điểm lock-in Postgres mới đúng lúc hệ thống đang gỡ bỏ lock-in đó.

### 3. Usecase mới

| Usecase | Việc làm |
|---|---|
| `CreateFleetDefinition` | Validate + insert 1 `FleetDefinition` mới (`version = 1`) |
| `UpdateFleetDefinition` | Ghi lại `servers`/`provision`, tăng `version` — không xoá bản cũ (xem "Không thuộc phạm vi" §1) |
| `GetFleetDefinition` / `ListFleetDefinitions` | Đọc theo `tenant_id` (+ `id`/`name`) |
| `ExportFleetDefinitionYaml` | Serialize `FleetDefinition` → YAML đúng schema `orca-fleet.yaml` hiện có (`frontend/src/shared/fleet-config-parser.ts`'s `FleetConfigSchema`, mở rộng bởi CR-FLEET-001's `vaultSshRole` + CR-FLEET-002's `provision`) |
| `DeployFleetDefinition` | Đọc `FleetDefinition` theo `id` → nếu `Provision != nil`: gọi `ApplyTerraformPlan` (CR-FLEET-002) trước, map kết quả instance mới → gộp với `Servers` còn lại (host có sẵn) → gọi `BulkProvisionFleet` (CR-FLEET-001). **Không viết lại** logic 2 usecase đó — chỉ điều phối đúng thứ tự |

### 4. RPC mới

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto — cạnh BulkProvisionFleet (CR-FLEET-001), ApplyTerraformPlan (CR-FLEET-002)
rpc CreateFleetDefinition(CreateFleetDefinitionRequest) returns (FleetDefinition);
rpc UpdateFleetDefinition(UpdateFleetDefinitionRequest) returns (FleetDefinition);
rpc GetFleetDefinition(GetFleetDefinitionRequest) returns (FleetDefinition);
rpc ListFleetDefinitions(ListFleetDefinitionsRequest) returns (ListFleetDefinitionsResponse);
rpc ExportFleetDefinitionYaml(ExportFleetDefinitionYamlRequest) returns (ExportFleetDefinitionYamlResponse); // { yaml_content: string }
rpc DeployFleetDefinition(DeployFleetDefinitionRequest) returns (stream BulkProvisionFleetEvent); // TÁI DÙNG message đã có ở CR-FLEET-001, không định nghĩa event type mới
```

### 5. Frontend — 2 luồng nhập liệu, cùng 1 nguồn sự thật ở backend-go

- **Import YAML** (luồng đã có, `fleet-import-dialog.tsx`): sau khi parse client-side, gọi `CreateFleetDefinition`/`UpdateFleetDefinition` thay vì chỉ giữ trong state cục bộ rồi gọi provision thẳng.
- **Edit trực tiếp trên UI** (mới): `FleetProvisionWizard.tsx` thêm bước "chỉnh sửa danh sách server" trước khi deploy, ghi qua `UpdateFleetDefinition`.
- **Export YAML** (mới): nút "Export" trong Wizard/Admin fleet list — gọi `ExportFleetDefinitionYaml`, tải xuống `.yaml`.
- **Deploy** (mới): nút "Deploy"/"Provision" gọi `DeployFleetDefinition(id)` thay vì client tự gọi `BulkProvisionFleet` với `FleetSpec` parse tại chỗ — client không cần giữ bản YAML sau khi đã import.

---

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go` | [NEW] `FleetDefinition`, `ProvisionConfig` |
| `backend-go/services/infra-fleet-service/migrations/0018_fleet_definitions.{up,down}.sql` | [NEW] bảng `infra.fleet_definitions` (verify số thứ tự thật trước khi tạo file) |
| `backend-go/services/infra-fleet-service/internal/usecase/{create,update,get,list}_fleet_definition.go` | [NEW] CRUD usecase |
| `backend-go/services/infra-fleet-service/internal/usecase/export_fleet_definition_yaml.go` | [NEW] serialize → YAML |
| `backend-go/services/infra-fleet-service/internal/usecase/deploy_fleet_definition.go` | [NEW] điều phối `ApplyTerraformPlan` (nếu có) + `BulkProvisionFleet` — gọi lại, không viết mới logic provisioning |
| `backend-go/services/infra-fleet-service/internal/usecase/ports.go` | [NEW] `FleetDefinitionRepository` interface (`Create`, `Update`, `Get`, `List`) |
| `backend-go/services/infra-fleet-service/internal/adapter/postgres/fleet_definition_repository.go` | [NEW] implement interface trên |
| `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` | [NEW] 6 RPC handler tương ứng |
| `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` | [NEW] message/RPC ở mục 4 |
| `frontend/src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx` | Thêm bước edit + nút Export/Deploy gọi RPC mới thay vì gọi thẳng `BulkProvisionFleet` với spec ephemeral |
| `frontend/src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx` | Sau parse, gọi `CreateFleetDefinition`/`UpdateFleetDefinition` thay vì chỉ giữ state cục bộ |
| `docs/features/F31-fleet-provisioning.md` | Ghi nhận `backend-go` nay là nguồn sự thật cho fleet definition, không chỉ nhận YAML transient qua 1 lần gọi |

---

## Không thuộc phạm vi CR này

1. **Full audit trail / version history đầy đủ** (xem lại từng version cũ, diff giữa các version) — CR này chỉ tăng `version` counter trên chính record, không có bảng lịch sử riêng (`infra.fleet_definition_history`). Thêm sau nếu compliance/audit cần.
2. **Resync/diff tự động** (CR-FLEET-001 đã loại trừ `orca fleet sync`) — `DeployFleetDefinition` chỉ tạo/đăng ký thêm theo definition hiện tại, không tự phát hiện/xoá server đã bị bỏ khỏi definition so với lần deploy trước.
3. **Permission chi tiết theo từng fleet definition** (ai được xem/sửa definition nào) — dùng chung RBAC hiện có của `infra-fleet-service` (project/team scoping, xem `docs/crs/v4/team-rbac/CR-RBAC-004-project-team-scoped-server-visibility.md`), không thêm access-control model riêng cho `FleetDefinition`.
4. **Terraform state file storage** — vẫn là rủi ro/quyết định vận hành đã ghi nhận ở CR-FLEET-002 (S3/GCS backend hay Terraform Cloud); CR này chỉ lưu *định nghĩa đầu vào* (`workingDir`/`varsFile` dạng tham chiếu), không lưu state Terraform.
5. **Cutover Admin UI hiện tại sang gọi `backend-go`** — thuộc phạm vi CR-RBAC-001 (đã ghi nhận tương tự ở CR-FLEET-001); CR này chỉ đảm bảo `backend-go` CÓ API để cutover gọi vào.

---

## Tiêu chí chấp nhận

- [ ] `CreateFleetDefinition`/`UpdateFleetDefinition` lưu đúng `servers[]`/`provision` vào `infra.fleet_definitions`, tăng `version` đúng mỗi lần update
- [ ] `ExportFleetDefinitionYaml` round-trip: Import 1 YAML → Export lại → parse lại bằng `FleetConfigSchema` (frontend) → ra `FleetSpec` tương đương bản gốc (test tự động so sánh dữ liệu đã parse, không chỉ so string YAML)
- [ ] `DeployFleetDefinition` với `Provision != nil`: gọi đúng thứ tự `ApplyTerraformPlan` → map kết quả → `BulkProvisionFleet`; test tái hiện 1 definition có cả server "đã tồn tại" và server "cần tạo qua Terraform" trong cùng 1 definition, cả 2 loại được xử lý đúng
- [ ] `DeployFleetDefinition` với `Provision == nil` bỏ qua bước Terraform hoàn toàn, hành vi giống hệt gọi `BulkProvisionFleet` trực tiếp (CR-FLEET-001) — không regression
- [ ] Chạy lại `DeployFleetDefinition` cùng 1 `id` không tạo trùng lặp `SshTarget`/`DevServer` (thừa hưởng idempotency đã yêu cầu ở CR-FLEET-001, không cần cơ chế mới)
- [ ] `detect_changes({scope:"compare", base_ref:"main"})` xác nhận thay đổi chỉ chạm `infra-fleet-service` + phần frontend liên quan, không lan sang service khác ngoài dự kiến

---

## Impact analysis (gitnexus)

**Chưa chạy** — mọi symbol trong CR này (`FleetDefinition`, `CreateFleetDefinition`, `DeployFleetDefinition`, ...) là mới, chưa tồn tại trong codebase nên không có gì để `impact()`. Bắt buộc khi bắt đầu triển khai:

- Chạy `impact({target: "BulkProvisionFleet", direction: "upstream"})` và `impact({target: "ApplyTerraformPlan", direction: "upstream"})` **ngay sau khi CR-FLEET-001/002 thực sự merge** — trước khi `DeployFleetDefinition` (CR này) thêm 1 caller mới vào 2 usecase đó. Không giả định lại số liệu risk mà CR-FLEET-001/002 ước tính lúc đề xuất (`RegisterDevServer`/`CreateSshTarget`: LOW; `sshrelay.Provisioner`: HIGH nhưng chỉ tham chiếu) — code thật tại thời điểm CR này bắt đầu có thể đã khác, phải đo lại.
- Chạy `impact()` cho mọi usecase/RPC mới của chính CR này ngay khi tạo, trước khi mở rộng thêm, theo đúng CLAUDE.md.

---

## Liên quan

- [F31-fleet-provisioning.md](../../../features/F31-fleet-provisioning.md)
- [CR-FLEET-001-bulk-provision-from-yaml.md](./CR-FLEET-001-bulk-provision-from-yaml.md) — phụ thuộc cứng (`FleetSpecServer`, `BulkProvisionFleet`)
- [CR-FLEET-002-iac-real-infrastructure-creation.md](./CR-FLEET-002-iac-real-infrastructure-creation.md) — phụ thuộc cứng (`ApplyTerraformPlan`, `provision.iac` config)
- `frontend/src/shared/fleet-config-parser.ts` (`FleetConfigSchema`) — schema YAML cần giữ tương thích cho round-trip export
- [CR-DB-002-dialect-capability-layer-foundation.md](../multi-database/CR-DB-002-dialect-capability-layer-foundation.md) — kiểm tra chéo nếu `infra-fleet-service` vào danh sách pilot multi-dialect trước khi migration `JSONB` của CR này chạy thật
- `docs/crs/v4/team-rbac/CR-RBAC-004-project-team-scoped-server-visibility.md` — access control tái dùng, không làm riêng
