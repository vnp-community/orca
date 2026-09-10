# CR-DS-009 — Mobile Emulator Agent: Tách riêng khỏi Dev Server Agent

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-009 |
| **Tên** | Mobile Emulator Agent — Tiến trình/gói riêng, đăng ký độc lập với Dev Server Agent |
| **Loại** | Architectural Change — New Agent Kind |
| **Priority** | P1 — High (tính năng Mobile Emulator hiện không thể thực thi trên `backend-go`) |
| **Phiên bản** | v6.2 |
| **Ngày tạo** | 2026-09-03 |
| **Trạng thái** | Phase 1-5 DONE (verify xanh, xem [specs/emulator/README.md](../../../specs/emulator/README.md)): `packages/dev-agent-transport/` tách xong; `emulator/` chạy direct-websocket thật + Android control (adb) thật, iOS honest-stub có chủ đích; backend-go `AgentKind` + project binding + `emulator.*` routing xong (`go build`/`vet`/`test` xanh mọi service liên quan); frontend chọn Mobile Emulator Agent cho project + đăng ký agent qua `AddDevServerDialog`'s kind selector + `emulator.*` calls đổi sang target động (`tsc`/`vitest`/`vite build` khớp baseline). **Còn lại**: end-to-end thật với `infra-fleet-service` chạy qua docker-compose chưa verify (cần DB thật); backend-go's `devServer.list`/`add` wscompat chưa đọc/trả `kind` (filter kind ở web mode là no-op an toàn); port điều khiển iOS thật cần máy macOS/Xcode |
| **Phụ thuộc** | CR-DS-001, CR-DS-002, CR-DS-003, CR-DS-004, CR-DS-006 |
| **Tác động HLD** | C2-containers.md, C3-components.md, C4-code.md, deployment.md |
| **Tác động Features** | Mobile Emulator (Settings › Mobile Emulator, `EmulatorPane`), F28 (mẫu cho luồng onboarding), F34 (project binding) |

---

## 1. Bối cảnh & Vấn đề

### 1.1 Hiện trạng: backend-go đã "xong" phần khung, nhưng vô hiệu

`infra-fleet-service` đã có đầy đủ hợp đồng cho `emulator.*` — proto (`ListEmulatorDevices`, `GetEmulatorAvailability`, `AttachEmulatorSession`, `SendEmulatorTap`, `SendEmulatorGesture`, `SendEmulatorButton`, `RotateEmulator`, `ShutdownEmulator` — [`infrafleet.proto:124-131`](../../../backend-go/proto/orca/infrafleet/v1/infrafleet.proto#L124-L131)), usecase (`EmulatorRelay` — [`emulator_relay.go`](../../../backend-go/services/infra-fleet-service/internal/usecase/emulator_relay.go)), và wscompat wiring (`registerEmulatorChannels` — [`channels_emulator_folderworkspace_host.go`](../../../backend-go/services/api-gateway/internal/adapter/wscompat/channels_emulator_folderworkspace_host.go)). Trạng thái được chính đội triển khai ghi lại trong [`TASK-048`](../../../specs/backend-go/bugs/missing-v1/tasks/TASK-048-emulator-relay-design-blocked-on-agent.md): **DONE, nhưng "honestly relay-inert"**.

Lý do: mọi RPC `emulator.*` đều relay xuống một Dev Server Agent qua `agent.Exec(ctx, devServer, "device.list"/"device.attach"/..., params)`. Thiết kế **cố tình không có nhánh chạy emulator ngay trên host `backend-go`** — container Linux headless dùng chung không có Android SDK/Xcode/GPU/hiển thị cho từng user, và điều này được loại trừ tường minh trong tài liệu phân rã service (*"driving emulators on the shared backend-go host is explicitly excluded"*). Nhưng `agent/src/relay/agent-rpc-dispatch.ts` — nơi Dev Server Agent xử lý các RPC từ Gateway — **chưa có bất kỳ `case 'device.*'` nào**. Kết quả: mọi thao tác Mobile Emulator trả về lỗi cố định `INFRA_EMULATOR_UNSUPPORTED`.

### 1.2 Vì sao không thể gộp `device.*` vào Dev Server Agent hiện có

Đề xuất ban đầu (thêm `device.*` handler thẳng vào `agent/src/relay/agent-rpc-dispatch.ts`) có 2 vấn đề kiến trúc:

1. **Sai lệch vị trí vật lý.** Dev Server Agent chạy trên máy chứa *code* (thường là server Linux remote, theo đúng mô hình CR-DS-001). Máy đó hiếm khi có Android Studio/Xcode. iOS Simulator **chỉ chạy được trên macOS**. Người dùng cần Mobile Emulator lại thường có Android Studio/Xcode trên **máy cá nhân** (laptop) — một máy khác hẳn dev server đang chạy code.
2. **Trộn ranh giới bảo mật/triển khai.** Dev Server Agent nắm toàn quyền git/fs/pty của project. Bắt một agent như vậy cũng phải cài trên máy cá nhân của dev (chỉ để có Android Studio) là không hợp lý — vừa thừa quyền, vừa không khớp mô hình triển khai (Dev Server Agent cài như systemd/Docker trên server, không phải trên laptop cá nhân).

→ Cần **một loại agent khác, độc lập vòng đời và độc lập tiến trình**: **Mobile Emulator Agent**.

---

## 2. Giải pháp: Mobile Emulator Agent là một loại agent riêng

### 2.1 Mô hình

```
┌──────────────────────────────────────────────────────────────┐
│  ORCA BACKEND SERVER (backend-go) — Control Plane            │
│  infra-fleet-service: Agent Registry (kind-aware)             │
└───────────────┬───────────────────────────────┬───────────────┘
                │ wss:// (git.*, fs.*, pty.*)    │ wss:// (device.*)
     ┌──────────▼──────────┐          ┌──────────▼──────────────┐
     │ DEV SERVER AGENT     │          │ MOBILE EMULATOR AGENT   │
     │ package: agent/      │          │ package: emulator/      │
     │ chạy trên: Dev Server│          │ chạy trên: máy có       │
     │ (Linux/macOS/Windows,│          │ Android Studio/Xcode    │
     │  thường remote)      │          │ (thường máy cá nhân)    │
     │                      │          │                         │
     │ git/fs/pty/browser/  │          │ device.list/attach/tap/ │
     │ cli/wsl/automation   │          │ gesture/button/rotate/  │
     │                      │          │ shutdown/capabilities   │
     └──────────────────────┘          └─────────────────────────┘
```

**Nguyên tắc:** hai agent là hai tiến trình, hai gói cài đặt, hai vòng đời đăng ký **độc lập hoàn toàn**. Một project có thể trỏ code đến Dev Server A và điều khiển emulator qua Mobile Emulator Agent B chạy trên một máy khác — hoặc cả hai trỏ về cùng một máy nếu đó là máy local của dev.

### 2.2 Không nhân bản package `agent/` — trích tầng transport dùng chung

`agent/src/relay/` có ~150 file, nhưng chỉ ~10 file là hạ tầng kết nối/đăng ký (không phụ thuộc nghiệp vụ git/fs/pty). Sao chép nguyên khối (`cp -r agent emulator`) sẽ tạo ra bảo trì kép cho toàn bộ logic reconnect/handshake — trái với nguyên tắc đặt tên/không trùng lặp của `AGENTS.md`. Thay vào đó:

| Lớp | Xử lý |
|---|---|
| **Transport & lifecycle dùng chung** (`agent-config.ts`, `agent-logger.ts`, `agent-wire.ts`, `agent-connection-direct/relay/stdio.ts`, `agent-token-manager.ts`, `agent-session.ts`, `dispatcher.ts`, `protocol.ts`, `relay-handshake.ts`, `rotating-log-writer.ts`) | Trích thành package dùng chung mới `packages/dev-agent-transport/`. `agent/` và `emulator/` cùng phụ thuộc — sửa bug reconnect/handshake một lần, cả hai agent đều hưởng. |
| **Handler nghiệp vụ Dev Server** (`git-*`, `fs-*`, `pty-*`, `browser-*`, `cli-*`, `*-cli-handler.ts`, `wsl-*`, `plugin-overlay*`, `automation*`, …) | Giữ nguyên trong `agent/`. Không đưa sang `emulator/`. |
| **Handler nghiệp vụ Mobile Emulator** (mới) | Package mới `emulator/`, chỉ có `device-handler.ts` + `device-capabilities-handler.ts`. |

### 2.3 Gói `emulator/` mới — cấu trúc

```
emulator/
├── package.json        # name: "orca-emulator-agent", bin: { "orca-emulator-agent": "./out/emulator.js" }
├── build.mjs            # song song với agent/build.mjs, nhưng bundle nhẹ hơn nhiều
└── src/
    └── relay/
        ├── emulator-entry.ts          # main() — dùng packages/dev-agent-transport để dial/handshake
        ├── emulator-rpc-dispatch.ts   # switch rút gọn: chỉ case 'device.*'
        ├── device-handler.ts          # port từ backend/src/main/emulator/emulator-bridge.ts + backends/*
        └── device-capabilities-handler.ts  # probe Android SDK / Xcode / simctl, trả cho GetHostCapabilities-tương-đương
```

**Nguồn logic điều khiển thiết bị không viết lại từ đầu.** `backend/src/main/emulator/**` (17/19 file) đã thuần Node.js, không phụ thuộc Electron — chỉ `serve-sim-execution.ts` và `android/scrcpy-server-download.ts` có `import 'electron'` (dùng `app.getPath` cho cache dir), cần thay bằng đường dẫn cấu hình qua `ORCA_EMULATOR_DATA_DIR`. Phần còn lại (`EmulatorBridge`, `backends/*`, `android/*`, `simctl-simulator-devices.ts`, `emulator-gesture-sender.ts`, …) port gần như nguyên trạng.

**Mapping RPC method → field JSON đã có sẵn** trong `emulator_relay.go` (`devices[]`, `available/reason`, `sessionId/deviceId/platform`) — bám đúng shape đó để không phải sửa gì ở `infra-fleet-service`.

---

## 3. Backend-go: thêm khái niệm "loại agent" vào registry

### 3.1 `AgentKind` — phân biệt Dev Server Agent vs Mobile Emulator Agent

`DevServer`/`RegisterDevServerRequest` hiện không phân biệt loại agent đăng ký ([`infrafleet.proto:150-178`](../../../backend-go/proto/orca/infrafleet/v1/infrafleet.proto#L150-L178)). Thêm:

```protobuf
enum AgentKind {
  AGENT_KIND_UNSPECIFIED = 0;
  AGENT_KIND_DEV_SERVER = 1;
  AGENT_KIND_MOBILE_EMULATOR = 2;
}

message DevServer {
  // ...existing fields...
  AgentKind kind = 8;  // mặc định AGENT_KIND_DEV_SERVER cho hàng cũ (migration backfill)
}

message RegisterDevServerRequest {
  // ...existing fields...
  AgentKind kind = 5;
}
```

Mobile Emulator Agent gọi **cùng** `RegisterDevServer` RPC với `kind = AGENT_KIND_MOBILE_EMULATOR` → thừa hưởng miễn phí toàn bộ hạ tầng registry đã có (approval workflow CR-DS-006, health check, `IsDevServerConnected`, reconnect). Không cần bảng/registry mới. `ListDevServers`/`ListDevServersForUser` lọc theo `kind` để UI hiển thị đúng danh sách ("Dev Servers" vs "Mobile Emulator Agents").

### 3.2 Project binding: thêm `mobileEmulatorAgentId`

Mở rộng F34 (Project–Dev Server Binding): project hiện chỉ có `devServerId`. Thêm field song song `mobileEmulatorAgentId` — trỏ đến một `DevServer` có `kind = AGENT_KIND_MOBILE_EMULATOR`. Hai field độc lập, có thể cùng trỏ 1 máy (dev local) hoặc khác máy (dev server remote + Mac cá nhân cho emulator).

### 3.3 Định tuyến `emulator.*` theo `mobileEmulatorAgentId`, không theo connectionId git

`registerEmulatorChannels` hiện nhận `connectionId` kiểu git/worktree (`infra.connections`, gắn với repo path). Mobile Emulator Agent không có khái niệm repo/worktree, nên sửa để dùng nhánh `ResolveConnectionRequest.dev_server_id` **đã có sẵn trong proto** ([`infrafleet.proto:186-195`](../../../backend-go/proto/orca/infrafleet/v1/infrafleet.proto#L186-L195), chỉ chưa ai gọi theo hướng này):

```go
resp, err := client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{
    DevServerId: project.MobileEmulatorAgentID,
})
```

`usecase.EmulatorRelay` ở `infra-fleet-service` **không cần sửa logic** — `resolveDevServer`/`callAgent(... "device.xxx" ...)` giữ nguyên, vì `Relay`/`RelayByDevServer` vốn generic theo `devServerId` + method string, không quan tâm agent phía kia thuộc loại nào.

### 3.4 Frontend: trạng thái "Agent control is ready" tách theo kind

`MobileEmulatorAgentSetupGuide`/`useMobileEmulatorAgentSetupState` hiện hỏi trạng thái agent chung; đổi sang gọi `IsDevServerConnected` với `mobileEmulatorAgentId`, độc lập với trạng thái kết nối của Dev Server Agent phục vụ code.

---

## 4. Đăng ký / pairing — tái dùng luồng F28, chọn `kind` lúc cài đặt

Không phát minh luồng mới. Script cài đặt riêng (`install-emulator-agent.sh` / `orca-emulator-agent.exe`) chạy y hệt luồng pairing token của F28 (Dev Server Onboarding), chỉ khác tham số `--kind=mobile-emulator`. Sau khi có token, `emulator/src/relay/emulator-entry.ts` dial outbound WS (mode `direct-websocket` — phù hợp máy cá nhân sau NAT) dùng `packages/dev-agent-transport`, gọi `RegisterDevServer{kind: AGENT_KIND_MOBILE_EMULATOR}`.

---

## 5. Kế hoạch triển khai

### Phase 1 — Tách transport dùng chung
- [x] Trích **phần thuần codec** — `relay-protocol.ts` + `agent-wire.ts` (~450 dòng: khung nhị phân 13-byte, seq/ack, keepalive) → `packages/dev-agent-transport/`. Phạm vi thu hẹp so với dự tính ban đầu (không trích `agent-config.ts`/`agent-connection-*.ts`/`agent-session.ts`/`dispatcher.ts`/`protocol.ts`/`relay-handshake.ts`/`agent-token-manager.ts`/`rotating-log-writer.ts`) — khảo sát thật cho thấy `agent-session.ts` hardcode dispatcher/capabilities/PTY cleanup của Dev Server Agent, trích nguyên nó sẽ tái tạo lỗ hổng bảo mật CR-DS-009 muốn triệt tiêu. Xem [TDD-EMU-03](../../../specs/emulator/tdd/v1/03-transport-reuse-analysis.md).
- [x] `agent/` chuyển sang import từ package mới (`orca-dev-agent-transport`), test suite `agent/` xanh — zero regression (336 file pass / 3816 test pass, giống hệt baseline trước/sau move)

### Phase 2 — Gói `emulator/`
- [x] Khởi tạo workspace `emulator/` (package.json, build.mjs, tsconfig, vitest.config.ts)
- [x] Port điều khiển Android (`adb shell input tap/swipe/keyevent`, `adb emu kill`) → `emulator/src/relay/device-android-control.ts` + `device-session-registry.ts` — bản tự viết gọn, không phải copy nguyên `backend/src/main/emulator/**` (xem SOL-EMU-004/TASK-EMU-010 cho lý do). iOS control vẫn honest-stub (`-32601`), 2 file dính Electron không cần tách vì chưa dùng tới.
- [x] `emulator-rpc-dispatch.ts`: `device.list/availability/attach/tap/gesture/button/rotate/shutdown/capabilities` — tất cả có handler thật (Android) hoặc honest-stub method-cụ thể (iOS), không còn honest-stub chung chung
- [x] `emulator-entry.ts` dùng `packages/dev-agent-transport` qua `emulator-session.ts`/`emulator-connection-direct.ts` (tự viết riêng, không copy `agent-session.ts` — xem Phase 1) khi `ORCA_BACKEND_URL` được set; giữ stdio debug mode khi không set

### Phase 3 — Backend-go: `AgentKind`
- [x] Proto: thêm `AgentKind` enum, field `kind` trên `DevServer`/`RegisterDevServerRequest`, regenerate stubs
- [x] Migration: backfill `kind = AGENT_KIND_DEV_SERVER` cho hàng hiện có
- [x] `ListDevServers`/`ListDevServersForUser` nhận filter `kind`

### Phase 4 — Project binding & định tuyến
- [x] `project-service`: thêm `mobileEmulatorAgentId`
- [x] `channels_emulator_folderworkspace_host.go`: đổi nguồn id sang `ResolveConnection{dev_server_id}` từ `mobileEmulatorAgentId`
- [x] Cập nhật lại test honest-stub (`TASK-046`/`TASK-048`) theo hướng relay thật — contract đổi từ `connectionId` sang `projectId`, honest-stub giữ nguyên hành vi khi chưa bind

### Phase 5 — Onboarding & UI
- [x] **Quyết định thay thế** cho script cài đặt riêng: tái dùng `AddDevServerDialog.tsx` có sẵn (thêm `Select` chọn `kind`), vì cả 2 loại agent đăng ký qua cùng registry và cùng luồng test-connection/add/connect. Không tạo installer `curl|bash` — repo chưa có hạ tầng phân phối binary đã build sẵn cho `emulator/`; hiển thị lệnh build-from-source thật thay vào đó. Xem TASK-EMU-011.
- [ ] `MobileEmulatorAgentSetupGuide`/`use-mobile-emulator-agent-setup-state.ts` (setup guide CLI/skill cho agent lập trình trong `EmulatorPane`) — **chưa đổi** sang trạng thái theo `mobileEmulatorAgentId`; đây là tính năng khác (agent AI điều khiển emulator qua Orca CLI, không phải Mobile Emulator Agent process) — nằm ngoài phạm vi Phase 5 đã làm, để nguyên hành vi cũ.
- [x] Settings: section "Mobile Emulator Agents" mới trong `MobileEmulatorSettingsPane.tsx` (danh sách + nút "+ Add Mobile Emulator Agent"), tách khỏi `DevServerList.tsx`'s "Dev Servers" — xem TASK-EMU-012c.
- [x] UI chọn Mobile Emulator Agent cho từng project (`ProjectMobileEmulatorAgentSection.tsx`) — không có trong checklist gốc nhưng cần thiết để field `mobileEmulatorAgentId` (Phase 4) thực sự dùng được — xem TASK-EMU-012b.
- [x] Nối `emulator.*` calls (`EmulatorPane`, `MobileEmulatorSettingsPane`) từ hard-code `{kind:'local'}` sang target động + `projectId` — không có trong checklist gốc nhưng là điều kiện để Phase 3/4 (routing backend-go) thực sự được UI gọi tới — xem TASK-EMU-013.

---

## 6. Acceptance Criteria

- [ ] `emulator/` build ra binary độc lập, không phụ thuộc `agent/`'s git/fs/pty code
- [ ] Sửa bug reconnect/handshake ở `packages/dev-agent-transport/` áp dụng cho cả `agent/` và `emulator/` mà không cần sửa 2 nơi
- [ ] Một project trỏ `devServerId` (Linux remote) và `mobileEmulatorAgentId` (Mac cá nhân) khác nhau, cả hai hoạt động độc lập
- [ ] Ngắt kết nối Mobile Emulator Agent không ảnh hưởng `git.*`/`fs.*`/`pty.*` của Dev Server Agent, và ngược lại
- [ ] `emulator.*` RPC từ UI không còn trả `INFRA_EMULATOR_UNSUPPORTED` khi Mobile Emulator Agent đã pair và kết nối
- [ ] Đăng ký Mobile Emulator Agent dùng cùng cơ chế token/approval với Dev Server Agent (CR-DS-006), không có luồng phê duyệt riêng
- [x] `go build`/`go vet`/`go test` sạch cho `infra-fleet-service` và `api-gateway` sau khi thêm `AgentKind` — xác nhận thêm sạch cho mọi service khác import `infrafleetv1`/`projectv1` (`ai-provider-service`, `git-gateway-service`, `project-service`, `task-service`, `workflow-service`); xem TASK-EMU-007/008/009 cho bằng chứng lệnh thật. Các mục còn lại của acceptance criteria này cần luồng runtime thật (Mobile Emulator Agent pair + kết nối thật) chưa verify được trong pass này.

---

## 7. Tham chiếu

- [CR-DS-001](./CR-DS-001-dev-server-agent-architecture.md) — Dev Server Agent Architecture (mô hình gốc được tái dùng)
- [CR-DS-003](./CR-DS-003-feature-delegation-matrix.md) — Feature Delegation Matrix
- [CR-DS-004](./CR-DS-004-agent-lifecycle-management.md) — Agent Lifecycle & Deployment (mẫu cho vòng đời Mobile Emulator Agent)
- [CR-DS-006](./CR-DS-006-dev-server-approval-and-grouping.md) — Approval & Grouping (tái dùng cho Mobile Emulator Agent)
- [TASK-048](../../../specs/backend-go/bugs/missing-v1/tasks/TASK-048-emulator-relay-design-blocked-on-agent.md) — trạng thái relay-inert hiện tại, điểm khởi đầu của CR này
- [F34 — Project-Dev Server Binding](../../features/F34-project-dev-server-binding.md) — mở rộng thêm `mobileEmulatorAgentId`
