# backend-go Solutions — Fleet Provisioning (v4, F31)

**CRs:** [docs/crs/v4/fleet-provisioning/](../../../../../../docs/crs/v4/fleet-provisioning/README.md)
**TDD tham chiếu:** [`infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md)

Cả 3 solution dưới đây là **thiết kế đặc tả — không có code thật nào được sửa** trong lúc viết. Mỗi symbol
đã tồn tại mà solution tham chiếu (`RegisterDevServer`, `CreateSshTarget`, `SshTargetRepository`,
`DevServerRepository`, `sshrelay.Provisioner`, `runRecipeCommand`, ...) được re-verify bằng Read/Grep/`ls`
trực tiếp trên code thật khi viết solution (2026-09-09), không suy đoán từ CR gốc — chi tiết ở mỗi solution's
§1 "Re-verify code thật".

## Solutions

| Solution | CR | Service | Status |
|---|---|---|---|
| [BE-FLEET-SOL-001](./BE-FLEET-SOL-001-bulk-provision-from-yaml.md) | CR-FLEET-001 | `infra-fleet-service` | 🔲 Designed — chưa implement |
| [BE-FLEET-SOL-002](./BE-FLEET-SOL-002-iac-terraform-orchestration.md) | CR-FLEET-002 | `infra-fleet-service`, `agent` | 🔲 Designed — chưa implement |
| [BE-FLEET-SOL-003](./BE-FLEET-SOL-003-fleet-definition-persistence.md) | CR-FLEET-003 | `infra-fleet-service` | 🔲 Designed — chưa implement |
| [BE-FLEET-SOL-004](./BE-FLEET-SOL-004-cloud-credential-storage-design.md) | CR-FLEET-002 §4 | `credential-broker-service` (đề xuất tái dùng) | 🟡 Design drafted — chờ security review (TASK-BE-FLEET-009) |

## Thứ tự implement

```
BE-FLEET-SOL-001 (bulk-register N host có sẵn từ YAML)
        │  không phụ thuộc CR nào khác — làm trước, effort vừa phải
        ▼
BE-FLEET-SOL-002 (IaC thật — Terraform CLI orchestration, Hướng A đã chốt)
        │  phụ thuộc SOL-001 theo LUỒNG DỮ LIỆU (map terraform output → FleetSpec
        │  → gọi BulkProvisionFleet), KHÔNG bắt buộc merge trước — có thể code
        │  song song với SOL-001 nếu team đủ người, review độc lập
        ▼
BE-FLEET-SOL-003 (FleetDefinition — lưu DB, export YAML, deploy lặp lại)
        │  phụ thuộc CỨNG cả 2 solution trên — persist FleetSpecServer (SOL-001)
        │  và điều phối ApplyTerraformPlan (SOL-002); chỉ nên bắt đầu code sau khi
        │  2 solution trên đã có hình hài tương đối ổn định (không nhất thiết đã
        │  merge, nhưng interface/struct không nên còn đổi liên tục)
```

## Nguyên tắc chung xuyên suốt cả 3 solution

- **Vault-only SSH invariant — bất di bất dịch.** `backend-go` **không bao giờ** chấp nhận `identityFile`
  hay bất kỳ raw SSH key material nào ở bất kỳ layer nào của cả 3 solution — chỉ `vaultSshRole` (pointer vào
  Vault SSH secrets engine role, cấp chứng chỉ ngắn hạn per-connection). Đã enforce ở `domain.NewSshTarget`
  (`internal/domain/ssh_target.go:32`) từ trước — không solution nào trong bộ này nới lỏng invariant đó.
  BE-FLEET-SOL-002's cloud provider credential (§4 của solution đó) là **1 loại secret khác hẳn** — không
  bao giờ trộn lẫn hay tái dùng cùng cơ chế lưu trữ với Vault SSH role.
- **`tenantID` luôn từ `tenant.RequireTenantID(ctx)`, không bao giờ từ request field.** Mọi usecase mới ở cả
  3 solution (`BulkProvisionFleet`, `DeleteSshTarget`, `ApplyTerraformPlan`, `CreateFleetDefinition`, ...)
  tuân thủ đúng pattern `RegisterDevServer`/`CreateSshTarget` đã có — request proto có thể mang 1 field
  `tenant_id` cho mục đích tài liệu/logging, nhưng **không bao giờ** là nguồn sự thật cho tenant scoping.
  Đúng nguyên tắc đã kiểm chứng ở `BE-SOL-STORAGE-001/002`, `BE-SOL-EVM-*`, `tenant-service.md` §9.
- **Tái sử dụng, không viết lại.** Cả 3 solution cố tình tái dùng usecase/adapter đã có
  (`sshrelay.Provisioner`, `runRecipeCommand`, `StreamVmProvision`'s convention, `BulkProvisionFleet`,
  `ApplyTerraformPlan`) thay vì viết lại logic dial/deploy/handshake/process-run — chỉ thêm lớp điều phối
  hoặc bước mới phía trước/phía trên.
- **`impact()` bắt buộc trước khi sửa symbol đã tồn tại, và ngay sau khi tạo symbol mới trước khi mở rộng
  thêm** (CLAUDE.md). Cả 3 solution đã chạy `impact()` thật cho `RegisterDevServer`/`CreateSshTarget` (LOW,
  3 impacted mỗi cái) và `sshrelay.Provisioner` (HIGH, 4 impacted, tham chiếu không sửa) — số liệu đã
  re-verify khớp CR-FLEET-001 gốc, không có gì đổi tại thời điểm viết (2026-09-09).
- **Rủi ro bảo mật "Cao" của BE-FLEET-SOL-002's credential storage KHÔNG được hạ cấp.** Bất kỳ ai thực thi
  phần đó phải đi qua security review trước khi khoá schema lưu trữ — xem solution's §0/§4 và
  `tasks/README.md`'s ghi chú tương ứng.
