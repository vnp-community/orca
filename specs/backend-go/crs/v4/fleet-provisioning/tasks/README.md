# backend-go Tasks — Fleet Provisioning (v4, F31)

**Solutions:** [../solutions/](../solutions/README.md) | **CRs:** [docs/crs/v4/fleet-provisioning/](../../../../../../docs/crs/v4/fleet-provisioning/README.md)

**Cập nhật 2026-09-09 (lượt thực thi thực tế):** 12/14 task ✅ DONE (code + test thật đã chạy), 1
🟡 DESIGN DRAFTED (TASK-BE-FLEET-009 — vẫn là task thiết kế/security review, không phải code; câu hỏi #1 đã
chốt (Vault, container dùng chung), 5/6 câu còn lại nay có đề xuất cụ thể trong BE-FLEET-SOL-004 dựa trên tái
sử dụng `credential-broker-service` có sẵn — chờ security review sign-off trước khi mở task code), 1 MOVED
(TASK-BE-FLEET-005 — sửa `frontend/`, đã chuyển toàn bộ nội dung sang
[specs/frontend/crs/v4/fleet-provisioning/](../../../../../frontend/crs/v4/fleet-provisioning/tasks/README.md)
— xem file gốc để biết lý do và lịch sử). Xem từng file task's "Kết quả thực tế" cho chi tiết, sai khác so
với sketch, và các gap thật phát hiện lúc làm (đặc biệt: TASK-BE-FLEET-013's round-trip YAML phụ thuộc
FE-TASK-FLEET-001 chưa làm; TASK-BE-FLEET-014 phát hiện + sửa 1 race condition thật trong
TASK-BE-FLEET-003's RPC handler).

~~Tất cả 14 task dưới đây là **🔲 TODO — chưa implement**.~~ Không solution nào trong bộ 3 solution mà bộ task
này bám theo đã sửa code sản xuất — mọi task bắt đầu từ cùng baseline chưa xây. Mỗi task được viết để 1 AI
agent thực thi **trong 1 lượt**, không cần đọc lại solution gốc — file path, tên function/struct, và code
mẫu được copy gần như nguyên văn từ solution đã scope chúng.

## Task ↔ Solution ↔ CR ↔ Depends on ↔ Status

| Task | Solution | CR | Depends on | Status |
|---|---|---|---|---|
| [TASK-BE-FLEET-001](./TASK-BE-FLEET-001-bulk-provision-fleet-usecase.md) — usecase `BulkProvisionFleet` | BE-FLEET-SOL-001 | CR-FLEET-001 | TASK-BE-FLEET-002 | ✅ DONE (main.go wiring completed together with 003) |
| [TASK-BE-FLEET-002](./TASK-BE-FLEET-002-delete-ssh-target-usecase.md) — usecase `DeleteSshTarget` (compensating rollback) | BE-FLEET-SOL-001 | CR-FLEET-001 | — | ✅ DONE |
| [TASK-BE-FLEET-003](./TASK-BE-FLEET-003-bulk-provision-fleet-rpc-proto.md) — RPC/proto `BulkProvisionFleet` | BE-FLEET-SOL-001 | CR-FLEET-001 | TASK-BE-FLEET-001 | ✅ DONE |
| [TASK-BE-FLEET-004](./TASK-BE-FLEET-004-ssh-targets-host-unique-migration.md) — migration `0017` idempotency constraint | BE-FLEET-SOL-001 | CR-FLEET-001 | — | ✅ DONE |
| [TASK-BE-FLEET-005](./TASK-BE-FLEET-005-yaml-schema-vault-ssh-role.md) — YAML schema `vaultSshRole` (frontend) | BE-FLEET-SOL-001 | CR-FLEET-001 | — | ➡️ MOVED — xem [FE-TASK-FLEET-001](../../../../../frontend/crs/v4/fleet-provisioning/tasks/FE-TASK-FLEET-001-yaml-schema-vault-ssh-role.md) |
| [TASK-BE-FLEET-006](./TASK-BE-FLEET-006-terraform-runner-interface-usecase.md) — `TerraformRunner` interface + usecase `ApplyTerraformPlan` | BE-FLEET-SOL-002 | CR-FLEET-002 | — (implementation thật phụ thuộc TASK-BE-FLEET-007) | ✅ DONE |
| [TASK-BE-FLEET-007](./TASK-BE-FLEET-007-agent-terraform-apply-rpc.md) — agent RPC `terraform.apply` | BE-FLEET-SOL-002 | CR-FLEET-002 | — | ✅ DONE |
| [TASK-BE-FLEET-008](./TASK-BE-FLEET-008-apply-terraform-plan-rpc-proto.md) — RPC/proto `ApplyTerraformPlan` | BE-FLEET-SOL-002 | CR-FLEET-002 | TASK-BE-FLEET-006 | ✅ DONE |
| [TASK-BE-FLEET-009](./TASK-BE-FLEET-009-cloud-credential-storage-design-security-review.md) — **thiết kế + security review** credential storage | BE-FLEET-SOL-002 | CR-FLEET-002 | — | 🟡 DESIGN DRAFTED — xem [BE-FLEET-SOL-004](../solutions/BE-FLEET-SOL-004-cloud-credential-storage-design.md), chờ security review sign-off |
| [TASK-BE-FLEET-010](./TASK-BE-FLEET-010-fleet-definition-domain.md) — domain `FleetDefinition` (+ di chuyển `FleetSpecServer`) | BE-FLEET-SOL-003 | CR-FLEET-003 | TASK-BE-FLEET-001, TASK-BE-FLEET-006 | ✅ DONE |
| [TASK-BE-FLEET-011](./TASK-BE-FLEET-011-fleet-definitions-migration.md) — migration `0018 fleet_definitions` | BE-FLEET-SOL-003 | CR-FLEET-003 | TASK-BE-FLEET-004 | ✅ DONE |
| [TASK-BE-FLEET-012](./TASK-BE-FLEET-012-fleet-definition-crud-usecase-rpc.md) — CRUD usecase + RPC | BE-FLEET-SOL-003 | CR-FLEET-003 | TASK-BE-FLEET-010, TASK-BE-FLEET-011 | ✅ DONE |
| [TASK-BE-FLEET-013](./TASK-BE-FLEET-013-export-fleet-definition-yaml.md) — usecase `ExportFleetDefinitionYaml` | BE-FLEET-SOL-003 | CR-FLEET-003 | TASK-BE-FLEET-012 | ✅ DONE (gap: round-trip phụ thuộc [FE-TASK-FLEET-001](../../../../../frontend/crs/v4/fleet-provisioning/tasks/FE-TASK-FLEET-001-yaml-schema-vault-ssh-role.md) chưa làm) |
| [TASK-BE-FLEET-014](./TASK-BE-FLEET-014-deploy-fleet-definition.md) — usecase `DeployFleetDefinition` (điều phối 001/006) | BE-FLEET-SOL-003 | CR-FLEET-003 | TASK-BE-FLEET-001, TASK-BE-FLEET-006, TASK-BE-FLEET-012 | ✅ DONE (phát hiện + sửa race condition thật trong TASK-BE-FLEET-003's handler) |

14 task trên 3 solution/3 CR. TASK-BE-FLEET-009 là ngoại lệ có chủ đích — không phải task code, xem cảnh báo
riêng trong chính file đó.

## Sơ đồ thứ tự thực thi — xuyên suốt cả 3 CR

```
Wave 1 (độc lập hoàn toàn — chạy song song ngay)
  TASK-BE-FLEET-002  DeleteSshTarget usecase              (SOL-001, CR-FLEET-001)
  TASK-BE-FLEET-004  migration 0017 ssh_targets unique     (SOL-001, CR-FLEET-001)
  TASK-BE-FLEET-005  YAML schema vaultSshRole (frontend)   (SOL-001, CR-FLEET-001)
  TASK-BE-FLEET-007  agent RPC terraform.apply             (SOL-002, CR-FLEET-002)
  TASK-BE-FLEET-009  credential storage — design/security review (SOL-002, CR-FLEET-002, chạy song song, không chặn ai)

Wave 2
  TASK-BE-FLEET-001  BulkProvisionFleet usecase   ← cần 002
  TASK-BE-FLEET-006  TerraformRunner + ApplyTerraformPlan usecase ← interface độc lập, implementation thật cần 007

Wave 3
  TASK-BE-FLEET-003  RPC/proto BulkProvisionFleet          ← cần 001
  TASK-BE-FLEET-008  RPC/proto ApplyTerraformPlan           ← cần 006
  TASK-BE-FLEET-010  domain FleetDefinition (+ di chuyển FleetSpecServer) ← cần 001, 006
  TASK-BE-FLEET-011  migration 0018 fleet_definitions        ← cần 004 (số thứ tự tiếp theo)

Wave 4
  TASK-BE-FLEET-012  FleetDefinitionRepository + CRUD usecase + RPC ← cần 010, 011

Wave 5
  TASK-BE-FLEET-013  ExportFleetDefinitionYaml usecase + RPC ← cần 012
  TASK-BE-FLEET-014  DeployFleetDefinition usecase + RPC      ← cần 001, 006, 012 (điều phối cả 2)
```

Ghi chú thứ tự:

- **Wave 1** là 5 task thực sự không phụ thuộc nhau — verify chéo file/symbol trước khi giao song song cho
  nhiều AI/dev khác nhau, tránh xung đột merge (TASK-BE-FLEET-004/011 cùng thư mục `migrations/`, tuần tự về
  SỐ THỨ TỰ dù không phụ thuộc code — xem cảnh báo `ls migrations/` ở mỗi task).
- **TASK-BE-FLEET-003/008 độc lập với nhau** — RPC BulkProvisionFleet (CR-FLEET-001) và ApplyTerraformPlan
  (CR-FLEET-002) chạm các message/rpc khác nhau trong cùng 1 file `infrafleet.proto` — sequencing để tránh
  merge conflict trên cùng file, không phải phụ thuộc logic.
- **TASK-BE-FLEET-009 không chặn wave nào** — nó chạy song song từ đầu, nhưng **kết quả của nó (tài liệu
  design đã duyệt) là điều kiện để bất kỳ ai coi TASK-BE-FLEET-006/007/008 là "sẵn sàng production"** — về
  mặt code, các task Terraform vẫn build/test được độc lập với 009 (dùng fake/mock).
- **TASK-BE-FLEET-010 phải chạy sau CẢ 001 và 006** — nó di chuyển `FleetSpecServer` (định nghĩa ở 001) và
  cần hình dạng `ProvisionConfig`-tương-đương (định hình ở 006's solution, dù chưa có struct cùng tên) đã ổn
  định trước khi tạo `domain.FleetDefinition`.
- **TASK-BE-FLEET-014 là điểm cuối** — điều phối cả `BulkProvisionFleet` (001) và `ApplyTerraformPlan` (006),
  đồng thời cần `FleetDefinitionRepository` (012) để đọc definition trước khi điều phối.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore` trước khi sửa symbol đã tồn tại** — mỗi task đã ghi rõ symbol
  cần kiểm tra ở mục "gitnexus", nhưng đây là yêu cầu bắt buộc chung theo `CLAUDE.md`/`AGENTS.md`, không chỉ
  khi task nhắc tới. Số liệu `impact()` đã re-verify tại thời điểm viết bộ task này (2026-09-09, không đổi so
  với CR-FLEET-001 gốc): `RegisterDevServer`/`CreateSshTarget` — **LOW**, impactedCount 3 mỗi cái;
  `sshrelay.Provisioner` — **HIGH**, impactedCount 4 (modules `Sshrelay`/`Backendrelaysshprovisioner`/`Usecase`),
  chỉ tham chiếu, không CR/task nào trong bộ này sửa trực tiếp struct đó. Dùng lại số liệu này nếu implement
  ngay sau khi bộ task này được viết; **chạy lại nếu nghi ngờ code đã đổi** (khoảng cách thời gian lớn, hoặc 1
  CR khác đã merge chạm `sshrelay`/`register_dev_server.go`/`create_ssh_target.go`).
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê; nếu phát hiện gap khác trong lúc
  làm (như TASK-BE-FLEET-014's câu hỏi mở §"UserName/VaultSSHRole cho instance Terraform"), ghi nhận lại theo
  đúng khuôn task đã có, không tự chọn hướng khác với đã ghi nếu task đã chốt 1 hướng cụ thể.
- **Vault-only SSH invariant bất di bất dịch** — không task nào trong bộ này (kể cả TASK-BE-FLEET-009's thiết
  kế credential cloud) được phép làm `backend-go` chấp nhận `identityFile`/raw SSH key material. Cloud
  provider credential (TASK-BE-FLEET-009) là loại secret HOÀN TOÀN KHÁC — không bao giờ trộn lẫn cơ chế lưu
  trữ với Vault SSH role.
- **`tenantID` luôn từ `tenant.RequireTenantID(ctx)`** — không bao giờ từ request field. Mọi usecase mới
  trong bộ 14 task này tuân thủ đúng pattern đã verify ở `RegisterDevServer`/`CreateSshTarget`.
- **`buf generate` sau bất kỳ thay đổi `.proto` nào** (TASK-BE-FLEET-003, 008, 012, 013, 014) — kiểm tra
  bằng `git diff --stat proto/gen/go/` rằng thay đổi CHỈ chạm `infrafleet.pb.go`/`infrafleet_grpc.pb.go`,
  không phá `proto/gen/go` dùng chung với service khác trước khi commit.
- **Test trước, không giả định pass** — mọi lệnh trong mục "Verify" của từng task phải thực sự chạy và thấy
  kết quả, không suy đoán (mirror `specs/backend-go/crs/v3/storage/tasks/README.md`'s nguyên tắc).
- **Số thứ tự migration luôn `ls migrations/` lại ngay trước khi tạo file** (TASK-BE-FLEET-004, 011) — bộ CR
  gốc từng phát hiện 1 lần lệch số giữa lúc viết CR và lúc implement; bộ task này đã verify số thật tại thời
  điểm viết (`0016` là file mới nhất → `0017` cho TASK-004, `0018` cho TASK-011 SAU KHI 004 merge), nhưng
  khoảng cách thời gian tới lúc thực thi có thể khiến số này lệch nếu 1 CR khác chen migration vào trước.
- **`detect_changes({scope:"compare", base_ref:"main"})` trước khi commit mỗi task** — xác nhận thay đổi chỉ
  chạm `infra-fleet-service`/`agent` (TASK-BE-FLEET-007)/`frontend` (TASK-BE-FLEET-005) như task đã khai báo,
  không lan sang service khác ngoài dự kiến.

## Known gap: bộ task này KHÔNG cover cutover Admin UI/Wizard sang gọi `backend-go`

Đúng như CR-FLEET-001/003 tự ghi nhận, việc `FleetProvisionWizard.tsx`/`fleet-import-dialog.tsx` chuyển từ
gọi `desktop/`'s Electron IPC sang gọi các RPC mới (`BulkProvisionFleet`, `CreateFleetDefinition`,
`DeployFleetDefinition`, ...) là quyết định kiến trúc rộng hơn F31 — thuộc phạm vi
`docs/crs/v4/team-rbac/CR-RBAC-001-consolidate-admin-surface-to-backend-go.md`, không nằm trong 14 task ở
đây. Bộ task này chỉ đảm bảo `backend-go` **CÓ khả năng** — cutover UI là 1 bộ task riêng, viết khi
CR-RBAC-001's cutover thực sự bắt đầu.

## Other things intentionally not in this task set

- Materialize `identityFile` → Vault SSH role tự động (CR-FLEET-001's "Không thuộc phạm vi" mục 1).
- `BulkApproveDevServers` cho N dev server `PendingApproval` (mục 2).
- `orca fleet sync` (resync/diff tự động khi fleet YAML đổi) — cả CR-FLEET-001 và CR-FLEET-003 đều loại trừ.
- Multi-cloud abstraction, tự sinh HCL, `terraform destroy`/lifecycle đầy đủ — CR-FLEET-002's "Không thuộc
  phạm vi" mục 1-3.
- Full audit trail/version history đầy đủ cho `FleetDefinition` (chỉ tăng `version` counter, không có bảng
  lịch sử riêng) — CR-FLEET-003's "Không thuộc phạm vi" mục 1.
- Permission chi tiết theo từng `FleetDefinition` — dùng chung RBAC hiện có (CR-RBAC-004), không làm riêng.
