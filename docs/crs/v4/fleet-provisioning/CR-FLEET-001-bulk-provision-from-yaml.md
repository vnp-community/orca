# CR-FLEET-001 — Bulk provision N dev server từ 1 fleet YAML ở `backend-go`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FLEET-001 |
| **Tên** | `infra-fleet-service` — usecase bulk-provision từ fleet YAML spec (lặp lại `Provisioner`/`CreateSshTarget`/`RegisterDevServer` có transaction/rollback semantics) |
| **Loại** | Feature (Gap Fix) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium–Large (4–6 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu hoàn thiện F31 ở `backend-go`/`agent` |
| **Tác động HLD** | C3.5 (Fleet Provisioning), C3.13 (Dev Server Agent) |
| **Tác động Features** | F31 (Fleet Inventory & Bulk Provisioning) |
| **Phụ thuộc** | Không bắt buộc — nên làm sau/song song CR-RBAC-002 (role claim) vì bulk-provision RPC cần global-admin override thật để chạy qua RPC theo đúng ngữ cảnh multi-user (xem `docs/crs/v4/team-rbac/CR-RBAC-002-unify-role-model-and-propagate-claims.md`) |

---

## Bối cảnh & Vấn đề

### 1. F31 mô tả "Bulk Server Provisioning" (CR-003) nhưng đó là code TS Electron cũ, không phải `backend-go`

`docs/features/F31-fleet-provisioning.md` trỏ CRs về `../crs/v1/remote-server/CR-001~004` — tài liệu đó (`docs/crs/v1/remote-server/CR-003-bulk-provisioning.md`) mô tả và đánh dấu "Implemented" một luồng `orca fleet import|provision|status|list` **hoàn toàn nằm trong `desktop/`** (Electron/Node cũ):

- `desktop/src/main/ssh/fleet-bootstrap-service.ts` — hàm `bootstrapServer()`
- `desktop/src/main/ssh/fleet-remote-commands.ts` — `installNodeJs`, `ensureGitInstalled`, `cloneOrUpdateRepo`
- `desktop/src/main/ssh/dev-server-provisioner.ts` — `class DevServerProvisioner` (tạo unix account, khác nghiệp vụ)
- `desktop/src/main/ipc/ssh.ts` — IPC handler `ssh.importFleetConfig`, `fleet:getStatus`
- `desktop/src/cli/handlers/fleet.ts` — CLI `orca fleet import|provision|status|list`

Đây chính là chuỗi module mà `docs/crs/v2/full-flow-tracing/CR-TRACE-012-fleet.md` đã audit và xác nhận **không có `class FleetManager`/`FleetProvisioner`** — logic phân mảnh thành hàm rời rạc, và toàn bộ vẫn sống ở tầng `desktop/` TS cũ, KHÔNG đi qua `backend-go`.

Frontend hiện tại (`frontend/`) gọi thẳng layer cũ này qua Electron IPC, xác nhận bằng chính source:

- `frontend/src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx:60` — `await window.api.ssh.importFleetConfig({ yamlContent, configFilePath: file.name })`
- `frontend/src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx:56` — `await window.api.ssh.provisionFleetServers?.({ serverIds: ids, concurrency: 3 })`
- `frontend/src/preload/api-types.ts:3335,3352` — khai báo 2 IPC method trên, implement tại `desktop/src/preload/index.ts`, `desktop/src/main/ipc/ssh.ts`, `desktop/src/main/runtime/rpc/methods/ssh.ts` (xác nhận qua grep, không phải backend-go)

Điều này khớp đúng bức tranh mà `docs/crs/v4/team-rbac/README.md` đã ghi cho Admin UI nói chung: **"Admin UI/RBAC surface mà F32's acceptance criteria trỏ tới vẫn là một hệ song song, chưa nói chuyện với backend-go thật"** (CR-RBAC-001) — F31's wizard/YAML-import UI rơi vào đúng gap kiến trúc này, chỉ khác domain (fleet thay vì RBAC).

### 2. `backend-go/services/infra-fleet-service` chỉ có đăng ký **1 host tại 1 thời điểm**, không có bulk, không có YAML

Khảo sát trực tiếp `backend-go/services/infra-fleet-service`:

- `internal/usecase/register_dev_server.go:30-57` — `RegisterDevServer.Execute()` nhận 1 `RegisterDevServerInput{Host, Mode, SSHTargetID, Kind}`, tạo **1** `domain.DevServer` — không có input dạng slice/batch.
- `internal/usecase/create_ssh_target.go:24-48` — `CreateSshTarget.Execute()` tương tự, tạo **1** `domain.SshTarget`.
- `internal/adapter/grpc/server.go:249-259` (RPC `RegisterDevServer`) và dòng tương ứng cho `CreateSshTarget` — cả hai RPC method chỉ nhận 1 request/1 response, xác nhận qua `grep -c "^  rpc " backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (58 RPC method, không method nào tên `Bulk*`, `Import*`, hay `Provision*` theo nghĩa "N host cùng lúc từ YAML").
- 3 `Provisioner` có thật trong service (xác nhận bằng GitNexus + đọc trực tiếp):
  - `internal/adapter/sshrelay/provisioner.go:66-115` — `Provisioner.Provision(ctx, devServer) (Transport, HandshakeInfo, error)`: dial **1** SSH target đã tồn tại trong DB, SFTP-deploy `agent.js --stdio`, chờ `agent.handshake`. Đây là "connect + hand-shake 1 dev server đã đăng ký", **không** tạo record mới, không lặp N lần.
  - `internal/adapter/backendrelaysshprovisioner/provisioner.go:41-75` (implements `EphemeralVmSshProvisioner`) và `internal/usecase/agent_outbound_ssh_provisioner.go:36-58` (`AgentOutboundSshProvisioner`, cùng interface) — cả hai chỉ giải quyết bài toán "dial reachability cho **1** ephemeral VM SSH target theo recipe kết quả `ssh`-type" (`internal/usecase/ports.go:466-468`), không liên quan gì tới "đọc 1 file YAML rồi tạo N host".
- Không có bước "preflight" nào ở backend-go làm việc N-lần-song-song với concurrency control + partial-failure report như CR-003's `orca fleet provision --concurrency N` đặc tả — cái gần nhất là `get_host_capabilities.go` (`GetHostCapabilities`, dòng 20-40), nhưng đó là probe WSL/pwsh/git-bash cho **1** connection đã tồn tại, không phải bulk provisioning.

Kết luận: audit trước ("BG/AG có registration/preflight nhưng thiếu bulk-provision/IaC thực sự") là chính xác — `RegisterDevServer`/`CreateSshTarget`/`Provisioner` là các viên gạch "1-host" đúng đắn, nhưng **chưa có usecase/RPC nào lặp N lần từ 1 YAML spec với transaction/rollback semantics**.

### 3. Gap schema: YAML fleet spec hiện tại KHÔNG tương thích với mô hình bảo mật SSH của `backend-go`

`frontend/src/shared/fleet-config-parser.ts:36-53` (`FleetServerSchema`) cho phép `identityFile: z.string().optional()` — một **đường dẫn private key thô** trên máy chạy Orca. Nhưng `backend-go`'s `domain.NewSshTarget` (`internal/domain/ssh_target.go:32-47`) enforce invariant ngược lại hoàn toàn:

```go
// ErrEmptyVaultSSHRole enforces this service's security invariant (see
// specs/backend-go/services/infra-fleet-service.md §9): no raw SSH key
// material is ever stored here, only a pointer into Vault's SSH secrets
// engine role used to issue a short-lived certificate per connection.
```

`SshTarget{ID, TenantID, Host, UserName, VaultSSHRole}` — không có field `identityFile`/`port`/`jumpHost`. Nghĩa là: **không thể ánh xạ trực tiếp 1 record `orca-fleet.yaml`'s `servers[]` (dùng `identityFile`) sang `backend-go`'s `CreateSshTargetInput{Host, UserName, VaultSSHRole}`** — phải đổi format YAML (thêm field `vaultSshRole`, bỏ/deprecate `identityFile` cho path này) hoặc thêm 1 bước "materialize identityFile vào 1 Vault SSH role" trước khi gọi `CreateSshTarget`, việc mà agent/backend-go hiện chưa có usecase nào làm.

### 4. `DevServerStatus` mặc định `PendingApproval` — bulk-register N host nghĩa là N lần chờ duyệt thủ công

`internal/domain/dev_server.go:164-184` (`NewDevServer`) luôn set `Status: DevServerStatusPendingApproval`. Có usecase `ApproveDevServer`/`RejectDevServer` (`internal/usecase/approve_dev_server.go`, `reject_dev_server.go`) nhưng cả hai chỉ nhận **1 ID**. Import 1 fleet YAML có 20 server hôm nay (nếu build theo kiến trúc mới) sẽ tạo 20 dòng `pending_approval` phải duyệt tay từng cái — không có `BulkApprove`.

### 5. Không có idempotency ở tầng SSH target

`SshTargetRepository` (`internal/usecase/ports.go:108-117`) chỉ có `Create`, `Get`, `List` — không có `FindByHost` hay `Upsert` (khác với `DevServerRepository.FindByHostAndMode`, dòng 39-41, đã có sẵn cho dev server). Chạy lại cùng 1 fleet YAML hôm nay (nếu có bulk usecase) sẽ tạo `SshTarget` trùng lặp — vi phạm chính acceptance criteria "Idempotent" mà CR-003 (bản TS cũ) đã đặt ra và tự nhận đã làm được (`ssh-connection-store.ts`'s `importFromFleetConfig()` có upsert theo `fleetId`, nhưng đó là store SQLite của `desktop/`, không map sang Postgres schema của `backend-go`).

---

## Giải pháp đề xuất

Xây 1 usecase mới `BulkProvisionFleet` trong `infra-fleet-service`, **tái sử dụng nguyên vẹn** `CreateSshTarget`, `RegisterDevServer`, và `sshrelay.Provisioner` đã có — không viết lại logic dial/deploy/handshake — chỉ thêm lớp điều phối N-lần + concurrency + rollback ở trên:

```go
// backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go
type FleetSpec struct {
    Version string
    Servers []FleetSpecServer
}

type FleetSpecServer struct {
    Host, UserName, VaultSSHRole string // KHÔNG identityFile — xem "Không thuộc phạm vi" §1
    Kind                         domain.AgentKind
}

type BulkProvisionFleet struct {
    createSshTarget   *CreateSshTarget
    registerDevServer *RegisterDevServer
    devServers        DevServerRepository // FindByHostAndMode cho idempotency dev-server side
    sshTargets        SshTargetRepository // List() + lọc theo host cho idempotency ssh-target side (§ "Không thuộc phạm vi" nếu cần FindByHost thật)
    limiter           chan struct{}       // concurrency gate, mirror CR-003's `--concurrency N`
}

// Execute chạy N server tuần tự theo lô (bounded concurrency), mỗi server là
// 1 đơn vị "tất cả-hoặc-không" độc lập: CreateSshTarget rồi RegisterDevServer
// — nếu RegisterDevServer fail SAU KHI CreateSshTarget đã thành công, xoá lại
// SshTarget vừa tạo (compensating action, KHÔNG dùng DB transaction xuyên
// service call vì CreateSshTarget/RegisterDevServer đã là 2 usecase riêng,
// mỗi usecase tự commit) — xem "Rollback semantics" bên dưới.
func (uc *BulkProvisionFleet) Execute(ctx context.Context, spec FleetSpec) (BulkProvisionResult, error)
```

### Rollback semantics (không có DB transaction xuyên 2 usecase — dùng compensating action)

`CreateSshTarget` và `RegisterDevServer` là 2 usecase độc lập, mỗi usecase tự gọi `repo.Create`/`repo.Register` và trả về ngay — không có API "mở 1 transaction rồi truyền qua 2 usecase" trong codebase hiện tại (mỗi usecase constructor chỉ nhận repository, không nhận `*sql.Tx`). Thêm 1 transaction xuyên-usecase là thay đổi kiến trúc lớn, ngoài phạm vi CR này. Thay vào đó, theo đúng nguyên tắc Saga/compensating-action đơn giản:

1. `CreateSshTarget.Execute()` cho server N → thành công → có `sshTargetID`.
2. `RegisterDevServer.Execute()` cho server N (dùng `sshTargetID` vừa tạo) → **fail**.
3. `BulkProvisionFleet` gọi 1 usecase mới, nhỏ — `DeleteSshTarget` (chưa tồn tại, cần thêm — xem bảng Changes Required) — để dọn `sshTargetID` mồ côi, tránh rác `infra.ssh_targets` không gắn `dev_server` nào.
4. Ghi rõ trong `BulkProvisionResult` server N là `failed`, kèm lý do — **không rollback các server khác đã thành công trước đó trong cùng batch** (mỗi server là 1 đơn vị độc lập, đúng tinh thần CR-003's "Report: N/M servers provisioned successfully", không phải all-or-nothing cho cả batch).

### RPC mới

```protobuf
// backend-go/proto/orca/infrafleet/v1/infrafleet.proto — thêm cạnh RegisterDevServer/CreateSshTarget hiện có
rpc BulkProvisionFleet(BulkProvisionFleetRequest) returns (stream BulkProvisionFleetEvent);

message BulkProvisionFleetRequest {
  string tenant_id = 1;
  repeated FleetSpecServer servers = 2;
  int32 concurrency = 3; // default 5, mirror CR-003's flag mặc định
}

message BulkProvisionFleetEvent {
  string host = 1;
  enum Status { PENDING = 0; SUCCEEDED = 1; FAILED = 2; }
  Status status = 2;
  string dev_server_id = 3;
  string error = 4;
}
```

Stream (không phải unary trả về sau khi xong hết) — theo đúng tiền lệ đã có trong cùng file cho việc "N bước tuần tự cần progress" (`StreamVmProvision`, dòng 196 của `infrafleet.proto`, `internal/usecase/ports.go:355`) — tái dùng convention stream đã có, không phát minh transport mới.

### YAML → `FleetSpec` mapping — sửa schema, không giữ `identityFile`

Vì `backend-go` không bao giờ chấp nhận raw key material (§3 ở trên), field `vaultSshRole` phải **thêm mới** vào `orca-fleet.yaml`'s schema cho phần dùng để bulk-provision qua `backend-go` (khác với `identityFile` cũ chỉ dùng cho `desktop/` legacy path):

```yaml
servers:
  - host: dev-alpha.vnpblc.internal
    username: dev
    vaultSshRole: dev-alpha-role   # MỚI — bắt buộc cho backend-go path, thay thế identityFile
```

Việc parse YAML → `FleetSpec` nằm ở tầng gọi RPC (frontend hoặc CLI `orca fleet provision`, tuỳ CR-RBAC-001 quyết định Admin UI cutover sang backend-go dùng entrypoint nào) — **không** parse YAML trong `infra-fleet-service` (Go) để tránh 2 nơi cùng định nghĩa 1 schema; `infra-fleet-service` chỉ nhận `FleetSpec` đã parse+validate.

---

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` | [NEW] `rpc BulkProvisionFleet`, `message BulkProvisionFleetRequest/Event/FleetSpecServer` |
| `backend-go/services/infra-fleet-service/internal/usecase/bulk_provision_fleet.go` | [NEW] `BulkProvisionFleet` usecase — lặp `CreateSshTarget`+`RegisterDevServer` theo `FleetSpec`, bounded concurrency, compensating rollback |
| `backend-go/services/infra-fleet-service/internal/usecase/delete_ssh_target.go` | [NEW] `DeleteSshTarget` usecase — dọn `SshTarget` mồ côi khi `RegisterDevServer` fail sau `CreateSshTarget` thành công |
| `backend-go/services/infra-fleet-service/internal/usecase/ports.go` | `SshTargetRepository` thêm `Delete(ctx, tenantID, id) error`; cân nhắc thêm `FindByHost` cho idempotency (xem AC) |
| `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go` | Thêm `DELETE FROM infra.ssh_targets WHERE id = $1 AND tenant_id = $2` (cạnh `INSERT` đã có ở dòng 203) |
| `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` | [NEW] `Server.BulkProvisionFleet` — server-streaming, gọi usecase, emit `BulkProvisionFleetEvent` mỗi server xong |
| `backend-go/services/infra-fleet-service/migrations/0017_ssh_targets_host_unique.up.sql` (số thứ tự tiếp theo `0016_ephemeral_vm_ssh_target_host_key` — verify lại tại thời điểm implement, `migrations/` có thể đã tiến thêm) | [NEW] Unique constraint `(tenant_id, host)` trên `infra.ssh_targets` nếu chọn hướng "DB constraint chặn trùng" thay vì `FindByHost` ở tầng usecase (một trong hai, không cần cả hai — xem AC) |
| `frontend/src/shared/fleet-config-parser.ts` | `FleetServerSchema` thêm `vaultSshRole: z.string().optional()`; giữ `identityFile` cho path `desktop/` legacy, đánh dấu `@deprecated` khi dùng cho path `backend-go` |
| `frontend/src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx` | Bước "confirm" hiển thị rõ server nào thiếu `vaultSshRole` → sẽ bị skip nếu target backend là `backend-go` (khác hẳn hành vi hiện tại gọi `window.api.ssh.provisionFleetServers` thẳng vào `desktop/`) |
| `docs/features/F31-fleet-provisioning.md` | Cập nhật bảng "Yêu cầu kỹ thuật" — bổ sung cột "Tầng" (desktop legacy / backend-go) cho từng file, vì hiện bảng chỉ liệt kê file `src/main/ssh/*` như thể đó là kiến trúc duy nhất |

---

## Không thuộc phạm vi CR này

1. **Materialize `identityFile` → Vault SSH role tự động.** CR này yêu cầu YAML cung cấp `vaultSshRole` có sẵn (do admin tự tạo role trong Vault trước) — không tự động "upload private key rồi tạo Vault role hộ user". Đây là 1 CR bảo mật riêng nếu business cần giữ UX "chỉ cần identityFile" cho backend-go path.
2. **`BulkApprove` cho `DevServerStatusPendingApproval`.** N dev server mới vẫn cần duyệt từng cái qua `ApproveDevServer` hiện có, hoặc 1 CR riêng thêm `BulkApproveDevServers` nếu UX yêu cầu duyệt cả lô.
3. **Cutover Admin UI/Wizard sang gọi `backend-go` thay vì `desktop/`'s IPC.** Đó là quyết định kiến trúc rộng hơn F31 (toàn bộ Admin UI, không riêng fleet) — xem `docs/crs/v4/team-rbac/CR-RBAC-001-consolidate-admin-surface-to-backend-go.md`. CR này chỉ đảm bảo `backend-go` CÓ khả năng bulk-provision khi cutover đó xảy ra.
4. **`FindByHost` đầy đủ + tự động resync khi fleet YAML thay đổi (`orca fleet sync`).** CR-003 (bản cũ) có `fleet sync` — CR này chỉ làm `provision` (thêm mới), không làm `sync` (thêm/xoá theo diff).
5. **Tự tạo hạ tầng mới (VM/instance) từ YAML** — đây là "IaC thật", xem CR-FLEET-002 (CR riêng vì độ phức tạp và quyết định phạm vi khác hẳn).

---

## Tiêu chí chấp nhận

- [ ] `BulkProvisionFleet` usecase nhận `FleetSpec` với N server, gọi `CreateSshTarget`+`RegisterDevServer` cho từng server với concurrency bị chặn (default 5, cấu hình được qua request)
- [ ] 1 server fail ở bước `RegisterDevServer` sau khi `CreateSshTarget` thành công → `SshTarget` mồ côi bị xoá (compensating action), không rò rỉ record
- [ ] 1 server fail không chặn các server khác trong cùng batch tiếp tục chạy — kết quả cuối cùng là danh sách per-server `SUCCEEDED`/`FAILED`, đúng tinh thần "N/M servers provisioned successfully"
- [ ] Chạy lại cùng `FleetSpec` (cùng `host`) không tạo `SshTarget` trùng lặp — có cơ chế idempotency thật (unique constraint hoặc `FindByHost`, chọn 1 trong Changes Required)
- [ ] `BulkProvisionFleetRequest` không chấp nhận field kiểu `identityFile`/raw key material — chỉ `vaultSshRole`, đúng invariant `domain.NewSshTarget`
- [ ] Test tái hiện đúng "N server, 1 server host rỗng/invalid" → usecase trả về đúng 1 `FAILED` cho server đó, N-1 server còn lại vẫn `SUCCEEDED`
- [ ] `detect_changes({scope:"compare", base_ref:"main"})` xác nhận thay đổi chỉ chạm `infra-fleet-service`'s usecase/adapter/grpc/proto layer, không lan sang service khác ngoài dự kiến

---

## Impact analysis (gitnexus)

Chạy trực tiếp qua MCP `impact()` (repo `orca`), không phải số liệu suy đoán:

| Symbol | Direction | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `RegisterDevServer` (struct, `internal/usecase/register_dev_server.go`) | upstream | LOW | 3 (direct: `NewRegisterDevServer` → `cmd/server/main.go`'s `run`/`main`) | An toàn để gọi lại (không sửa) từ `BulkProvisionFleet` — usecase này không đổi, chỉ có thêm 1 caller mới |
| `CreateSshTarget` (struct, `internal/usecase/create_ssh_target.go`) | upstream | LOW | 3 (direct: `NewCreateSshTarget`) | Tương tự — tái sử dụng nguyên trạng |
| `Provisioner` (struct, `internal/adapter/sshrelay/provisioner.go`) | upstream | **HIGH** | 4 (3 module: `Sshrelay`, `Backendrelaysshprovisioner`, `Usecase`; process `run`/`main` bị "broken" theo graph — thực ra là "phụ thuộc", không phải lỗi) | CR này **không sửa** `sshrelay.Provisioner` — chỉ gọi nó gián tiếp qua `RegisterDevServer`/kết nối có sẵn. Rủi ro HIGH chỉ áp dụng nếu có CR khác sửa trực tiếp struct này; ghi nhận ở đây để cảnh báo nếu 1 PR tương lai đụng vào `sshrelay.Provisioner` khi triển khai CR-FLEET-001 — PHẢI chạy lại `impact()` ngay trước khi sửa, đúng quy tắc bắt buộc của repo |
| `EphemeralVmSshProvisioner` (interface, `internal/usecase/ports.go:466`) | upstream | MEDIUM | 11 (8 direct qua `IMPORTS`) | Không đổi trong CR này — liệt kê để phân biệt rõ với `BulkProvisionFleet` (2 khái niệm "Provisioner" khác nhau dễ nhầm, xem Bối cảnh §2) |

Chưa chạy `impact()` cho các symbol **mới** (`BulkProvisionFleet`, `DeleteSshTarget`) vì chưa tồn tại — bắt buộc chạy `impact()` lần nữa ngay khi các symbol này được tạo, trước khi mở rộng thêm (theo CLAUDE.md).

---

## Liên quan

- [F31-fleet-provisioning.md](../../../features/F31-fleet-provisioning.md)
- [CR-003 (v1, đã Implemented cho `desktop/` TS)](../../v1/remote-server/CR-003-bulk-provisioning.md)
- [CR-TRACE-012-fleet.md](../../v2/full-flow-tracing/CR-TRACE-012-fleet.md) — audit kiến trúc thật của luồng fleet cũ, xác nhận không có `FleetManager`/`FleetProvisioner`
- [CR-DS-001-dev-server-agent-architecture.md](../../v2/dev-server/CR-DS-001-dev-server-agent-architecture.md) — F31 xếp vào Gateway's "Fleet Management" trong kiến trúc v6.0
- [CR-DS-003-feature-delegation-matrix.md](../../v2/dev-server/CR-DS-003-feature-delegation-matrix.md) — dòng F31: "GW Role: Provisioning wizard, RBAC / Agent Role: Agent auto-config on first boot"
- [CR-RBAC-001](../../v4/team-rbac/CR-RBAC-001-consolidate-admin-surface-to-backend-go.md) — cutover Admin UI, phụ thuộc chéo với việc bulk-provision RPC này được UI nào gọi
- [CR-RBAC-002](../../v4/team-rbac/CR-RBAC-002-unify-role-model-and-propagate-claims.md) — global-admin override cần thiết để 1 admin bulk-provision fleet không phải member của mọi project
- [CR-FLEET-002-iac-real-infrastructure-creation.md](./CR-FLEET-002-iac-real-infrastructure-creation.md) — gap còn lại (IaC thật, tạo hạ tầng mới), không nằm trong CR này
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:466` (`EphemeralVmSshProvisioner`)
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:66` (`Provisioner.Provision`)
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:32` (`NewSshTarget`'s Vault-only invariant)
- `frontend/src/shared/fleet-config-parser.ts:36` (`FleetServerSchema`)
