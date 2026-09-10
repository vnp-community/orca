# Ma trận hoàn thành Feature theo Codebase (Frontend / Backend-go / Agent / Emulator)

**Cập nhật:** 2026-09-09 | **Nguồn:** đối chiếu [`docs/features/`](../features/README.md) (42 feature specs) với code thật trong `frontend/`, `backend-go/`, `agent/` (quét bằng 3 sub-agent độc lập, codegraph/grep có định hướng, không đọc từng dòng) + `emulator/` (rà soát trực tiếp, 21 file, đối chiếu [CR-DS-009](../crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md))

---

## 0. Bối cảnh quan trọng — đây KHÔNG phải cùng một trục với `docs/features/README.md`

Repo hiện có **nhiều codebase song song**:

| Codebase | Vai trò |
|----------|---------|
| `backend/` | **Legacy** — Electron main process TypeScript, monolith cũ. Hầu hết feature spec trong `docs/features/*.md` mô tả kiến trúc này (đường dẫn `src/main/...`, `src/renderer/...`). **Không nằm trong phạm vi audit này.** |
| `frontend/` | UI mới (renderer đã tách khỏi Electron main) + platform abstraction layer (`IPlatformServices`, Electron/Node/Web adapters) |
| `backend-go/` | **Kiến trúc mới** — 17 Go microservices (`tenant-service`, `project-service`, `ai-provider-service`, `workflow-service`, `task-service`, `git-gateway-service`, `infra-fleet-service`, `auth-service`, v.v.), rõ ràng được xây chủ yếu cho nhóm v5.0 (F33–F39) cộng với việc viết lại một số phần server-side cũ |
| `agent/` | **Dev Server Agent** — binary chạy trên dev server/remote host: spawn PTY, chạy git, gọi AI provider CLI, nói chuyện qua Agent WebSocket Protocol về phía Orca |
| `emulator/` | **Mobile Emulator Agent** — package riêng (21 file), độc lập vòng đời với `agent/`: điều khiển Android/iOS simulator qua `device.*` (adb / xcrun simctl) cho tính năng "Mobile Emulator" (Settings › Mobile Emulator). **Chưa có ID trong `docs/features/`** — xem [CR-DS-009](../crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md) và §2 bên dưới |
| `packages/dev-agent-transport/` | Transport/lifecycle dùng chung (wire protocol, keepalive, handshake framing) mà cả `agent/` và `emulator/` cùng import — tách ra để sửa bug reconnect một lần, cả hai agent đều hưởng |
| `desktop/` | Electron main + preload — tách từ `backend/` cũ, chứa CLI thật `src/cli/` (F09 Orca CLI) + runtime RPC `src/main/runtime/`. **Không có trong phạm vi 3 sub-agent audit gốc** (§"Nguồn" ở đầu file) — xem CR-CLI-001/002/003 |

→ Ma trận dưới đây trả lời câu hỏi khác với Feature Registry: **"feature này đã được port sang kiến trúc mới (frontend + backend-go + agent) tới đâu?"** — không phải "feature này đã release cho người dùng chưa" (câu đó `docs/features/README.md` đã trả lời qua cột Trạng thái ✅/🚧/📋).

### Ký hiệu

| Ký hiệu | Ý nghĩa |
|---------|---------|
| ✅ | Có bằng chứng rõ ràng, logic/UI hoạt động cho concern của layer này |
| 🟡 | Có scaffolding/types/UI một phần nhưng chưa đầy đủ so với spec |
| ❌ | Không tìm thấy bằng chứng trong layer này (có thể chỉ còn ở `backend/` legacy, hoặc chưa build) |
| N/A | Feature không có concern ở layer này về mặt cấu trúc (vd. Desktop Pet Companion không có phần backend-go) |

> **Giới hạn phương pháp:** kết quả từ 3 sub-agent quét theo từ khoá/tên file/proto định nghĩa có định hướng (không đọc toàn bộ ~7.000 file). Đủ tin cậy để định hướng ưu tiên, nhưng **nên xác minh lại bằng review thủ công** trước khi dùng làm căn cứ duy nhất để cắt/giữ code hoặc estimate.

---

## 1. Ma trận đầy đủ

| ID | Feature | Priority | Frontend | Backend-go | Agent | Tổng hợp |
|----|---------|----------|:--:|:--:|:--:|---------|
| F01 | Parallel Worktrees | P0 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F02 | Terminal Splits | P0 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F03 | Mobile Companion | P0 | ✅ | 🟡 | 🟡 | **Cập nhật (2026-09-09)**: CR-MOBILE-001 Track 1 (`specs/backend-go/crs/v4/mobile-companion/`) hoàn tất 9/9 task — push delivery thật qua `notification-service` (APNs/FCM/WebPush RFC 8291+8292 VAPID, `DeliverMobilePush` wired vào `HandleIncomingEvent`). Track 2 (route pairing/subscribe `POST /api/mobile/push-subscribe` ở `api-gateway`) vẫn **BLOCKED** — quyết định kiến trúc A/B đã chốt (Phương án B: mobile SSO riêng) nhưng chờ 1 CR hạ tầng SSO mobile mới, chưa viết. AG/agent vẫn chưa có pairing/QR/E2E thật. *(Khác với "Mobile Emulator Agent" ở §2.)* |
| F04 | AI Agent Support | P0 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F05 | Design Mode | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F06 | GitHub/Linear Integration | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F07 | SSH Worktrees | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F08 | Annotate AI Diffs | P1 | ✅ | ✅ | 🟡 | AG chỉ có shared format type, chưa có logic thực thi phía relay |
| F09 | Orca CLI | P1 | 🟡 | 🟡 | 🟡 | CLI thật hoạt động ở `desktop/src/cli/` (ngoài 3 cột bảng này). **Cập nhật (2026-09-09)**: CR-CLI-001/002/003 (`specs/backend-go/crs/v4/orca-cli/`) hoàn tất 7/7 task — `backend-go` giờ có hạ tầng auth/credential thật cho CLI headless: `wscompat` bearer-JWT identity fallback, route mint/revoke/list/audit token credential, và **vá 1 lỗ hổng authorization thật** (`IssueServiceToken` không ép `user_id` = caller). Vẫn không phải logic thực thi lệnh CLI (đó vẫn ở `desktop/`) — nâng từ ❌ lên 🟡 vì auth/credential path CLI headless giờ có bằng chứng thật ở backend-go. |
| F10 | Quick Open | P1 | ✅ | N/A | ✅ | Đúng kỳ vọng — search UI ở FE, remote dir-walk hỗ trợ ở AG |
| F11 | Notifications | P1 | ✅ | ✅ | ✅ | **Cập nhật (2026-09-09)**: CR-NOTIF-001 (`specs/backend-go/crs/v4/notification/`) hoàn tất 11/11 task — unread state tường minh (`ListNotifications`/`MarkAsRead`/`MarkAllAsRead`/`GetUnreadCount` + REST `api-gateway`), WS read-receipt đa tab/thiết bị, và `DeliverPush` (Web Push RFC 8291 encrypt + RFC 8292 VAPID JWT qua Vault) — kèm **vá 1 bug thật**: `SignVapidPayload` gọi nhầm Vault `transit/encrypt` thay vì `transit/sign`, khiến mọi VAPID JWT trước đây không hợp lệ về mặt mật mã. |
| F12 | File Explorer & Editor | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F13 | Text Search | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F14 | Automations | P2 | ✅ | ✅ | 🟡 | AG chỉ có schedule/precheck type, thiếu handler thực thi trigger phía relay |
| F15 | Computer Use | P2 | 🟡 | ❌ | ❌ | Gần như chưa có ở kiến trúc mới — khớp với trạng thái P2/🚧 còn sơ khai trong spec |
| F16 | Rich Repo Previews | P2 | ✅ | N/A | ❌ | Đúng kỳ vọng phần lớn (render client-side); AG không cần có |
| F17 | Memory/AI Vault | P2 | ✅ | ❌ | ✅ | BG chưa có storage AI vault — chỉ FE (UI) + AG (transcript/path types) |
| F18 | Ephemeral VM | P2 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F19 | Localization | P2 | ✅ | N/A | N/A | Đúng kỳ vọng — thuần client-side |
| F20 | Speech Input | P3 | ✅ | N/A | ❌ | Đúng kỳ vọng phần lớn — STT chạy on-device tại FE; AG chỉ có type rỗng |
| F21 | Auto Update | P0 | 🟡 | N/A | ❌ | FE thiếu UI hiển thị changelog; AG không có self-update relay (có thể do Orca-server phía deploy đảm nhiệm) |
| F22 | Web Server Mode | P0 | ✅ | ✅ | 🟡 | api-gateway là hậu duệ rõ ràng của web-server cũ; AG chỉ tham chiếu gián tiếp |
| F23 | Multi-User Auth | P0 | ✅ | ✅ | 🟡 | AG chỉ có connection-token auth (đúng vai trò), không có full login/session (đúng — đó là việc của BG) |
| F24 | Per-User Sandbox | P0 | 🟡 | 🟡 | ✅ | AG có per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` isolation rõ ràng nhất; BG có access-grant nhưng chưa thấy process-sandbox layer tường minh |
| F25 | Admin Panel | P1 | ✅ | ✅ | N/A | Đúng kỳ vọng |
| F26 | Multi-Database | P1 | ❌ | 🟡 | N/A | **Cập nhật (2026-09-09)**: CR-DB-002/003 (`specs/backend-go/crs/v4/multi-database/`) 8/8 task DONE — `common/dbcapability` package + dialect-safe migration split + MySQL/TiDB adapter thật + CI matrix Postgres+MySQL, tất cả hoạt động cho **pilot `usage-service`** (1/17 service), factory wiring theo `DATABASE_DSN`'s scheme. Kèm vá 3 bug thật phát hiện khi chạy test trên Postgres thật (không phải testcontainers giả lập): `ListSessions`'s tham số SQL suy luận kiểu mâu thuẫn (`uuid` vs `text`), fixture test dùng ID không phải UUID hợp lệ, và scan `NULL` vào `time.Time` không nullable. Chưa rollout ra 16 service còn lại — vẫn 🟡 vì phạm vi mới là 1 pilot, không phải multi-database toàn hệ thống. |
| F27 | Fleet Health Monitoring | P1 | ✅ | ✅ | 🟡 | BG có polling nhưng thiếu Prometheus metrics/webhook alert; AG chỉ có capability check, thiếu CPU/RAM/disk metrics |
| F28 | Dev Server Onboarding | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F29 | Agent WebSocket Protocol | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F30 | Remote Integrations | P1 | ✅ | ✅ | ✅ | Đầy đủ 3 lớp |
| F31 | Fleet Provisioning | P1 | ✅ | ✅ | ✅ | **Cập nhật (2026-09-09)**: CR-FLEET-001/002/003 (`specs/backend-go/crs/v4/fleet-provisioning/`) 13/14 task DONE — `BulkProvisionFleet`+`DeleteSshTarget` rollback thật, Terraform apply thật qua agent RPC `terraform.apply` (`TerraformRunner`+`ApplyTerraformPlan` RPC), `FleetDefinition` domain/CRUD/migration/RPC + `ExportFleetDefinitionYaml` + `DeployFleetDefinition` orchestration (kèm vá 1 race condition thật ở TASK-BE-FLEET-003's handler). Còn 1 task **security-review-gated, chưa thực thi theo đúng chủ đích**: TASK-BE-FLEET-009 (thiết kế lưu trữ cloud credential) — design đã có, chờ sign-off bảo mật trước khi code. YAML round-trip đầy đủ còn phụ thuộc 1 task frontend (FE-TASK-FLEET-001) chưa làm. |
| F32 | Team RBAC | P2 | ✅ | ✅ | N/A | **Đã xác minh (2026-09-09)**: CR-RBAC-001→007 hoàn tất 59/59 task ([`docs/crs/v4/team-rbac/`](../crs/v4/team-rbac/README.md), [PR #9](https://github.com/vnp-community/orca/pull/9)) — SSO OIDC/GitHub + group→role mapping + session refresh end-to-end (Phase 2 cũ, trước đây "pending", nay đã xong), role model thống nhất (`callerGlobalRole` đã sửa — admin toàn cục trước đó không thực sự quản lý được mọi project/repo), audit log `outcome`/`ip_address` + phủ đủ project/task/annotation/infra-fleet-service (trước chỉ auth-service tự audit), OPA policy publish thật (thay `NoopPublisher`), Admin UI hợp nhất về backend-go (`AdminOrgConsole` — tab Policies/Teams/Sessions/Audit qua wscompat, SPA cũ đã retire phần trùng lặp). Agent **N/A theo thiết kế** (không phải thiếu sót) — đã xác minh trực tiếp: agent không tham gia bất kỳ quyết định authn/authz/audit nào, mọi quyết định RBAC xảy ra ở backend-go trước khi request được relay tới agent (xem `specs/agent/crs/v4/team-rbac/solutions/README.md`). SAML (CR-RBAC-007) vẫn Backlog P3, cố ý chưa build — không nằm trong phạm vi acceptance criteria chính của F32 |
| F33 | User Profile Hierarchy | P0 (🚧) | ✅ | ✅ | 🟡 | AG mới chỉ inject `ORCA_USER_ID`/account env, **chưa có company/department hierarchy** khi spawn agent |
| F34 | Project-Dev Server Binding | P0 (🚧) | ✅ | ✅ | ✅ | Đầy đủ 3 lớp — Track 1 gần hoàn thành. Đã mở rộng thêm field `mobileEmulatorAgentId` song song với `devServerId` (migration `0015_project_mobile_emulator_agent`) — xem §2 |
| F35 | AI Provider Account Management | P0 (🚧) | ✅ | ✅ | ✅ | Đầy đủ 3 lớp — Track 2 gần hoàn thành |
| F36 | Multi-Server Workflow Orchestration | P1 (🚧) | 🟡 | ✅ | ✅ | **FE thiếu template library/sharing UI** — backend đã có DAG + template inheritance, frontend đang là điểm nghẽn |
| F37 | Task Graph Management | P0 (🚧) | 🟡 | ✅ | ✅ | **FE thiếu Board view + Grant/Share modal** — backend đã có graph/grant/AI-decompose, frontend đang là điểm nghẽn |
| F38 | Project Workspace | P0 (🚧) | ✅ | ✅ | ✅ | Đầy đủ 3 lớp — Track 3 gần hoàn thành |
| F39 | Remote Git UI | P0 (🚧) | ✅ | ✅ | ✅ | Đầy đủ 3 lớp — Track 3 gần hoàn thành |
| F40 | Full-Flow Tracing | P1 | ✅ | ✅ | ✅ | **Đã xác minh (2026-09-09)**: CR-FFT-001/002/003 hoàn tất 11/11 task (`specs/backend-go/crs/v4/full-flow-tracing/tasks/`) — OTel span thật (server + outbound client), `trace_id`/`span_id` trong log, span→NATS TraceEvent bridge, `api-gateway`'s SSE endpoint forward event thật (không còn heartbeat-only) |
| F41 | Desktop Pet Companion | P3 | ✅ | N/A | N/A | Đúng kỳ vọng |
| F42 | Contextual Onboarding Tours | P2 | ✅ | N/A | N/A | Đúng kỳ vọng |

---

## 2. Bổ sung — `emulator/`: Mobile Emulator Agent (chưa có ID trong `docs/features/`)

`emulator/` là **package thứ 4** ngoài `frontend/`/`backend-go/`/`agent/` — điều khiển Android/iOS simulator (`device.*` qua adb/xcrun simctl) cho tính năng Settings › Mobile Emulator. Đây là feature **chưa có ID Fxx trong `docs/features/`** — không nên nhầm với **F03 Mobile Companion** (pairing app di động thật qua QR/E2E). Nguồn: [CR-DS-009](../crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md) (Phase 1–5, trạng thái do chính team ghi lại) + xác minh nhanh trực tiếp trên code.

| Layer | Trạng thái | Bằng chứng | Ghi chú |
|-------|:--:|---|---|
| Frontend | ✅ | `components/emulator-pane/` (15 file: `EmulatorPane.tsx`, `use-emulator-pane-session.ts`, ...), `components/settings/MobileEmulatorSettingsPane.tsx`, `components/project/ProjectMobileEmulatorAgentSection.tsx`, `components/dev-server/AddDevServerDialog.tsx` (kind selector) | UI chọn Mobile Emulator Agent theo project, `emulator.*` calls dùng target động thay vì hard-code |
| Backend-go | ✅ | `infra-fleet-service` (`AgentKind` enum, `adapter/devserveragent/methods.go`, `usecase/emulator_relay.go` + test), `project-service/migrations/0015_project_mobile_emulator_agent.{up,down}.sql`, `api-gateway/.../channels_emulator_folderworkspace_host.go` + test | `go build`/`vet`/`test` xanh cho mọi service liên quan; 2 gap còn lại — xem dưới |
| Dev Server Agent (`agent/`) | N/A *(chủ đích)* | — | Theo thiết kế CR-DS-009 §1.2: **không** gộp `device.*` vào `agent/` — sai vị trí vật lý (dev server hiếm khi có Android Studio/Xcode) + trộn ranh giới bảo mật (agent này đã nắm toàn quyền git/fs/pty) |
| Mobile Emulator Agent (`emulator/`) | ✅ Android / ⚠️ iOS honest-stub | `device-android-control.ts` (adb `input tap/swipe/keyevent`, `emu kill` — **thật**), `device-ios-discovery.ts` (chỉ probe `xcrun simctl` có tồn tại), `device-control-handler.ts` (`DEVICE_METHOD_NOT_FOUND_CODE = -32601`) | iOS control là stub **có chủ đích**, không phải thiếu sót — cần máy macOS/Xcode thật để làm tiếp, và team đã ghi rõ điều này |
| `packages/dev-agent-transport/` | ✅ | `src/agent-wire.ts`, `src/relay-protocol.ts` | Transport dùng chung; `agent/` và `emulator/` cùng import, sửa 1 nơi 2 agent đều hưởng |

**Gap còn lại (theo chính CR-DS-009 tự ghi nhận, chưa phải do audit này phát hiện):**
1. End-to-end thật qua `infra-fleet-service` chạy bằng docker-compose **chưa verify** (cần DB thật, không chạy được trong CI/audit hiện tại)
2. `devServer.list`/`devServer.add` wscompat ở backend-go **chưa đọc/trả field `kind`** — filter theo kind ở web mode hiện là no-op an toàn (không hỏng chức năng) nhưng UI chưa thực sự lọc đúng "Dev Servers" vs "Mobile Emulator Agents" ở nguồn dữ liệu
3. Điều khiển iOS thật cần máy macOS/Xcode — không thể verify trong môi trường Linux/CI hiện tại (không phải bug, là giới hạn môi trường)

---

## 3. Điểm sáng — đã đầy đủ cả 3 lớp (18/42)

F01, F02, F04, F05, F06, F07, F12, F13, F18, F28, F29, F30, F34, F35, F38, F39 — và F10/F19/F20/F25/F32/F41/F42 đạt "đúng kỳ vọng" dù có N/A hợp lý (F32's Agent = N/A theo thiết kế, xác minh trực tiếp chứ không phải suy luận từ thiếu bằng chứng — xem §1).

Đáng chú ý: **toàn bộ nhóm Core Desktop IDE (F01, F02, F04) và phần lớn Group 3 (F06, F07, F12, F13, F28–F30)** đã được port sang kiến trúc mới gần như hoàn chỉnh — việc migrate khỏi `backend/` legacy cho các feature này coi như xong về mặt code, dù `docs/features/README.md` vẫn còn ghi các đường dẫn cũ.

**Track 1–3 của roadmap** ([`README.md`](./README.md) §2) cũng đã gần cán đích ở tầng code: F34/F35/F38/F39 đều ✅ cả 3 lớp — phần còn thiếu chủ yếu là polish/tích hợp, không phải xây từ đầu.

---

## 4. Khoảng trống cần xử lý (ưu tiên theo mức độ ảnh hưởng)

| # | Gap | Ảnh hưởng | Đề xuất |
|---|-----|-----------|---------|
| 1 | **F26 Multi-Database**: backend-go chỉ hỗ trợ Postgres ở 16/17 service, dialect abstraction mới có ở 1 pilot | Regression so với feature spec đã ✅ release (P1) — khách hàng dùng MySQL/TiDB sẽ không chạy được trên hầu hết service của kiến trúc mới | **Cập nhật (2026-09-09)**: `common/dbcapability` + MySQL/TiDB adapter thật + CI matrix Postgres+MySQL đã chạy PASS cho `usage-service` (pilot, CR-DB-002/003, 8/8 task DONE — không còn PARTIAL). Còn thiếu: rollout dialect abstraction sang 16 service data-owning còn lại — xem specs/backend-go/crs/v4/multi-database/ |
| 2 | **F36/F37 (Track 4)**: backend-go + agent đã sẵn sàng, nhưng **frontend là điểm nghẽn** (thiếu Workflow Template Library/Sharing UI, thiếu Task Board View + Grant Modal) | Chặn demo end-to-end của Track 4 dù backend đã xong | Ưu tiên frontend cho 2 feature này trước khi đầu tư thêm backend |
| 3 | **F03 Mobile Companion**: pairing/QR/E2E chỉ có ở frontend, không có ở backend-go/agent | Không rõ mobile companion có hoạt động được với kiến trúc server mới hay không | **Cập nhật (2026-09-09)**: push delivery (1 phần riêng của F03, không phải pairing) nay đã thật ở backend-go (CR-MOBILE-001 Track 1, 9/9 task — APNs/FCM/WebPush). Pairing/QR/E2E CHÍNH nó vẫn chưa có ở backend-go/agent — Track 2 (route subscribe) đang BLOCKED chờ 1 CR hạ tầng SSO mobile mới (quyết định A/B đã chốt: Phương án B). Vẫn cần xác minh câu hỏi gốc: pairing có thực sự cần phía backend-go/agent, hay là P2P thuần frontend↔mobile |
| 4 | **F33**: agent chưa inject company/department profile khi spawn, chỉ có user-level | Track 1 (Foundation) chưa thực sự "xong" ở tầng agent dù backend-go đã có `GetResolvedProfile` | Nối `agent-spawn-env.ts` với kết quả `GetResolvedProfile` đầy đủ 3 tầng |
| ~~5~~ | ~~**F40 Full-Flow Tracing**: SSE endpoint ở backend-go mới là heartbeat, chưa forward trace event thật (có TODO trong code)~~ — **Đã xác minh (2026-09-09)**: hoàn tất qua CR-FFT-001/002/003 (11/11 task), build+test sạch cho toàn bộ 17 module | — | Xem `specs/backend-go/crs/v4/full-flow-tracing/` |
| ~~6~~ | ~~**F09 Orca CLI**: không có bằng chứng rõ ràng ở cả 3 layer~~ — **Đã xác minh (2026-09-09)**: CLI thật ở `desktop/src/cli/`, không ở `backend-go`; `wscompat`'s `cli.*` là installer channel không liên quan (đổi tên `channels_cli_installer.go` để tránh nhầm lẫn tiếp — CR-CLI-003) | — | Xem CR-CLI-001/002/003 |
| 7 | **F15 Computer Use, F17 Memory/AI Vault (backend-go)**: gần như trống ở backend-go | Đúng với priority P2/🚧 thấp — không khẩn cấp | Giữ nguyên track "Continuous P2" như đã định trong roadmap chính |
| 8 | **Mobile Emulator Agent** (§2): `devServer.list`/`add` wscompat chưa trả field `kind` | UI có thể lẫn "Dev Servers" và "Mobile Emulator Agents" trong cùng danh sách nếu chỗ khác dựa vào response này để lọc | Bổ sung `kind` vào response wscompat — filter phía client hiện là no-op an toàn nên không khẩn cấp bằng #1–#4 |

---

## 5. Cách đọc chung với `docs/roadmap/README.md`

- Bảng track theo thời gian ở [`README.md`](./README.md) trả lời **"làm gì tiếp theo, theo thứ tự nào"** — dựa trên priority + dependency giữa các feature.
- Ma trận này trả lời **"phần đã làm thực sự nằm ở đâu, còn thiếu chỗ nào trong các lớp code mới"** — dùng để giao việc cụ thể cho team frontend / backend-go / agent / emulator trong từng track.
- Khi một dòng gap ở §4 được xử lý, cập nhật lại ô tương ứng trong bảng §1 (hoặc §2 cho Mobile Emulator Agent) và xoá dòng khỏi §4.
