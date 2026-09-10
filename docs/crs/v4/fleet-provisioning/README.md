# Fleet Provisioning (F31) — Change Requests (v4)

> **Bối cảnh:** Yêu cầu "thực thi đầy đủ F31 — Fleet Provisioning ở lớp `backend-go`
> (và `agent` nếu liên quan)". Audit trước (`docs/roadmap/feature-completion-matrix.md`,
> dòng F31) ghi: *"FE có wizard + YAML parser đầy đủ; BG/AG có registration/preflight
> nhưng thiếu bulk-provision/IaC thực sự"*. Khảo sát trực tiếp mã nguồn (GitNexus +
> đọc file thật) xác nhận đúng nhận định này, và làm rõ thêm 1 lớp gap kiến trúc mà
> audit trước chưa nói tới: **"bulk-provision" F31 gốc mô tả (CR-003, v1) đã
> "Implemented" — nhưng hoàn toàn nằm ở tầng `desktop/` (Electron/Node cũ), không
> phải `backend-go`**. Wizard/YAML-import mà FE gọi hôm nay (`window.api.ssh.
> importFleetConfig`, `window.api.ssh.provisionFleetServers`) đi thẳng vào IPC của
> `desktop/`, không đụng tới `backend-go/services/infra-fleet-service` — đúng dạng
> gap "2 hệ song song, chưa nói chuyện với nhau" mà `docs/crs/v4/team-rbac/README.md`
> đã ghi nhận cho toàn bộ Admin UI (CR-RBAC-001), chỉ khác domain (fleet, không phải
> RBAC).
>
> **✅ Cập nhật 2026-09-09 — Quyết định Product Owner:** (1) CR-FLEET-002's Hướng A
> (Terraform CLI orchestration — `backend-go` thực sự khởi tạo hạ tầng mới) được xác
> nhận, không còn "chờ business"; (2) yêu cầu bổ sung: kết quả/định nghĩa fleet phải
> **lưu vào DB** của `backend-go` làm nguồn sự thật, từ đó hỗ trợ **xuất lại YAML** và
> **triển khai (deploy) lặp lại** — xử lý ở CR mới
> [CR-FLEET-003](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md).

## Tổng quan gap đã xác nhận

| Gap | Trạng thái thật | CR |
|-----|-----------------|-----|
| YAML fleet config parser + Zod schema | ✅ Đầy đủ, ở `frontend/src/shared/fleet-config-parser.ts` — nhưng dùng `identityFile` (raw key path), không tương thích Vault-only invariant của `backend-go` | CR-FLEET-001 (§ Bối cảnh 3) |
| Wizard UI 4 bước (select → confirm → provision → done) | ✅ Đầy đủ, `FleetProvisionWizard.tsx` — nhưng gọi thẳng `desktop/`'s IPC, không phải `backend-go` | CR-FLEET-001 |
| Bulk provision N host cùng lúc từ 1 YAML, có concurrency + rollback | ❌ Không tồn tại ở `backend-go` — chỉ có `RegisterDevServer`/`CreateSshTarget` xử lý **1 host/lần gọi** | CR-FLEET-001 |
| Idempotency khi chạy lại cùng fleet YAML | ❌ `SshTargetRepository` không có `FindByHost`/`Upsert`, chỉ có `Create` | CR-FLEET-001 |
| IaC thật — tạo hạ tầng mới (VM/instance), không chỉ đăng ký host có sẵn | ❌ 0% — cơ chế gần nhất (Ephemeral VM, F18) chỉ chạy 1 shell command tuỳ ý do recipe author viết, không có Terraform/Ansible/cloud-init có cấu trúc | CR-FLEET-002 (✅ Hướng A đã quyết định 2026-09-09) |
| `Provisioner`/`SshProvisioner`/`EphemeralVmSshProvisioner` ở `backend-go` | ✅ Có thật, nhưng làm việc khác hẳn "bulk/IaC": đây là "dial + SFTP-deploy + handshake cho **1** SSH target/dev server **đã được đăng ký sẵn**" | Nền tảng CR-FLEET-001 tái sử dụng, không sửa |
| Lưu định nghĩa fleet/kết quả tạo hạ tầng vào DB, export lại YAML, deploy lặp lại | ❌ 0% — mọi definition hiện ephemeral qua request, không persist | CR-FLEET-003 (mới, theo yêu cầu Product Owner 2026-09-09) |

| CR | Vấn đề | Priority | Effort | Status |
|----|--------|----------|--------|--------|
| [CR-FLEET-001](./CR-FLEET-001-bulk-provision-from-yaml.md) | Bulk-provision N host từ 1 YAML ở `backend-go` — chưa tồn tại, chỉ có ở `desktop/` legacy | 🟠 P1 | Medium–Large | 🔲 Chưa triển khai |
| [CR-FLEET-002](./CR-FLEET-002-iac-real-infrastructure-creation.md) | IaC thật (tạo hạ tầng mới qua Terraform, Hướng A) — 0% code | 🟠 P1 | Large | ✅ Quyết định: Hướng A (2026-09-09), sẵn sàng triển khai |
| [CR-FLEET-003](./CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md) | Lưu fleet/infra definition vào DB làm nguồn sự thật — export YAML + deploy lặp lại | 🟠 P1 | Large | 🔲 Chưa triển khai — phụ thuộc CR-FLEET-001/002 |

## 2 khái niệm "Provisioner" dễ nhầm lẫn — làm rõ 1 lần cho cả 2 CR

Codebase có **3 struct tên/hậu tố "Provisioner"** ở `backend-go/services/infra-fleet-service`, cả 3 đều **KHÔNG** phải "bulk provisioning" hay "IaC" theo nghĩa F31 dùng — chúng giải quyết bài toán khác:

| Symbol | File | Làm gì thật sự |
|---|---|---|
| `sshrelay.Provisioner` | `internal/adapter/sshrelay/provisioner.go:66` | Dial **1** `domain.SshTarget` đã có trong DB, SFTP-deploy `agent.js --stdio`, chờ `agent.handshake`. Đây là "kết nối + cài agent lên 1 host đã biết trước", không tạo record mới. |
| `backendrelaysshprovisioner.Provisioner` | `internal/adapter/backendrelaysshprovisioner/provisioner.go:41` | Implement `EphemeralVmSshProvisioner` — "Hướng B" của việc dial **1** ephemeral VM SSH target (khi recipe kết quả là `ssh`-type), tái dùng `sshrelay.Provisioner`'s deploy pipeline nhưng auth khác. |
| `AgentOutboundSshProvisioner` | `internal/usecase/agent_outbound_ssh_provisioner.go:36` | "Hướng A" của cùng bài toán trên — agent tự dial ra ngoài thay vì Orca dial vào, cùng interface `EphemeralVmSshProvisioner` (`internal/usecase/ports.go:466`). |

Cả 3 đều là **"đơn vị nhỏ nhất: 1 kết nối"**. F31's gap thật là **thiếu lớp điều phối N-lần** phía trên các đơn vị này (CR-FLEET-001) và **thiếu bước "tạo mới" phía trước** khi host chưa tồn tại (CR-FLEET-002) — 2 CR này không sửa, không thay thế bất kỳ `Provisioner` nào ở trên, chỉ gọi chúng nhiều lần / thêm bước trước chúng.

## Thứ tự thực thi

```
CR-FLEET-001 (bulk-register N host có sẵn từ YAML) ── làm trước, effort vừa phải
                    │
                    ▼
CR-FLEET-002 (IaC thật — tạo host mới qua Terraform, Hướng A đã chốt) ── sẵn sàng triển khai
                    │
                    ▼
CR-FLEET-003 (lưu fleet definition vào DB, export YAML, deploy lặp lại) ── phụ thuộc cứng
                                                                             cả 2 CR trên
```

CR-FLEET-001 và CR-FLEET-002 vẫn có thể review/code song song (CR-FLEET-002 chỉ phụ
thuộc CR-FLEET-001 theo luồng dữ liệu, không theo thứ tự merge bắt buộc). CR-FLEET-003
thì phụ thuộc cứng cả hai — nó persist `FleetSpecServer` (CR-FLEET-001) và điều phối
`ApplyTerraformPlan` (CR-FLEET-002), nên chỉ nên bắt đầu code sau khi 2 CR đó đã có
hình hài tương đối ổn định (không nhất thiết đã merge, nhưng interface/struct không nên
còn đổi liên tục).

## Impact analysis (gitnexus, chạy trước khi sửa — bắt buộc theo CLAUDE.md)

| Symbol | Direction | Risk | Impacted | CR |
|---|---|---|---|---|
| `RegisterDevServer` (struct, `internal/usecase/register_dev_server.go`) | upstream | LOW | 3 (direct 1) | CR-FLEET-001 |
| `CreateSshTarget` (struct, `internal/usecase/create_ssh_target.go`) | upstream | LOW | 3 (direct 1) | CR-FLEET-001 |
| `sshrelay.Provisioner` (struct, `internal/adapter/sshrelay/provisioner.go`) | upstream | **HIGH** | 4 (3 module ảnh hưởng) | CR-FLEET-001 (tham chiếu, không sửa — cảnh báo cho PR tương lai nếu có ai đụng trực tiếp) |
| `EphemeralVmSshProvisioner` (interface, `internal/usecase/ports.go:466`) | upstream | MEDIUM | 11 (8 direct) | Tham chiếu — không đổi ở cả 2 CR |

Symbol **mới** của cả 3 CR (`BulkProvisionFleet`, `DeleteSshTarget`, `ApplyTerraformPlan`,
`TerraformRunner`, `FleetDefinition`, `DeployFleetDefinition`, ...) chưa tồn tại nên chưa
chạy được `impact()` — mỗi CR đã ghi rõ yêu cầu chạy `impact()` ngay khi symbol được tạo,
trước khi mở rộng thêm, theo đúng quy tắc bắt buộc của repo. CR-FLEET-003 còn yêu cầu chạy
lại `impact()` cho `BulkProvisionFleet`/`ApplyTerraformPlan` **tại thời điểm nó bắt đầu**
(không dùng số liệu ước tính của CR-FLEET-001/002 vì code thật lúc đó có thể đã khác). Không
CR nào trong bộ 3 CR này được thực thi (code) trong lần khảo sát/cập nhật này — cả ba file
trên là tài liệu đặc tả, chưa có thay đổi code nào.

## Việc chưa làm ngoài bộ CR này

- Cutover Admin UI/Wizard (`FleetProvisionWizard.tsx`, `fleet-import-dialog.tsx`) sang
  gọi `backend-go` thay vì `desktop/`'s Electron IPC — đây là quyết định kiến trúc rộng
  hơn F31 (toàn bộ Admin UI), xem `docs/crs/v4/team-rbac/CR-RBAC-001-consolidate-admin-surface-to-backend-go.md`.
  CR-FLEET-001/003 chỉ đảm bảo `backend-go` CÓ khả năng bulk-provision + lưu/export/deploy
  khi cutover đó xảy ra, không tự chuyển UI.
- Cập nhật `docs/features/F31-fleet-provisioning.md` để phản ánh đúng 2 tầng kiến trúc
  (desktop legacy vs backend-go) + mô hình mới "backend-go là nguồn sự thật cho fleet
  definition" (CR-FLEET-003) — cả 3 CR đề xuất việc này như một phần "Changes Required"
  của chính CR đó, chưa sửa trước khi CR được duyệt/triển khai.
- Cập nhật `docs/roadmap/feature-completion-matrix.md` dòng F31 — gap "thiếu bulk-provision/IaC
  thực sự" nay có hướng giải quyết đã quyết định (không còn "chờ business"), nên đổi mô tả
  từ "gap chưa rõ hướng" sang "đang triển khai theo CR-FLEET-001/002/003".
