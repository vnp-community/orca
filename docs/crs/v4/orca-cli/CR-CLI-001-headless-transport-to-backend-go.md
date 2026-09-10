# CR-CLI-001 — Transport backend-go-native cho `RuntimeClient`, thay `orca serve` khỏi việc tự respawn Electron

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CLI-001 |
| **Tên** | Thêm transport thứ 3 (backend-go WS) cho `RuntimeClient.call()`; định nghĩa lại headless mode để không phụ thuộc Electron |
| **Loại** | Feature + Architectural Decision |
| **Priority** | 🟡 P1 (giữ nguyên priority gốc của F09) |
| **Effort** | Large (thiết kế transport mới + wiring xuyên `desktop/`, `frontend/`, `backend-go/`) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Chưa triển khai — cần chốt kiến trúc (mục "Giải pháp đề xuất") trước khi code |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "thực thi đầy đủ F09 ở lớp backend-go", đối chiếu `docs/roadmap/feature-completion-matrix.md` gap #6 |
| **Tác động HLD** | C2 (theo `docs/features/F09-orca-cli.md`) |
| **Tác động Features** | F09 (Orca CLI), liên quan F22 (Web Server Mode), F23 (Multi-User Auth) |
| **Phụ thuộc** | CR-CLI-002 (credential headless) cho path CI/CD không tương tác; không phụ thuộc CR-CLI-003 |

---

## Bối cảnh & Vấn đề

### 1. CLI thật **không nằm ở đâu cả trong "backend-go/frontend/agent"** — nó nằm ở `desktop/`, gói thứ 4 mà audit 3-way (`frontend/`, `backend-go/`, `agent/`) của `feature-completion-matrix.md` chưa từng quét

`docs/roadmap/feature-completion-matrix.md` tự khai phạm vi ở §0: bảng "Codebase" liệt kê `backend/` (legacy, ngoài phạm vi), `frontend/`, `backend-go/`, `agent/`, `emulator/`, `packages/dev-agent-transport/` — **không có `desktop/`**. Đây chính xác là gap #6 nghi ngờ ("CLI binary có thể ở package khác chưa audit"): xác minh trực tiếp cho thấy:

- `desktop/package.json`: `"description": "Orca Desktop (Electron main + preload, isolated copy, split from monorepo)"`, có `"bin": {"orca": "./out/cli/index.js", "orca-dev": "./config/scripts/orca-dev.mjs"}` — trùng khớp với root `package.json:7-10`.
- `desktop/src/cli/` tồn tại đầy đủ: `dispatch.ts`, `handlers/` (worktree, orchestration, automations, terminal, browser, emulator, computer, linear, file...), `runtime/` (`client.ts`, `transport.ts`, `websocket-transport.ts`, `launch.ts`, `environments.ts`, `status.ts`) — **119 file**, có test đi kèm hầu hết.
- `skills/orca-cli/SKILL.md` (287 dòng) tài liệu hoá đầy đủ bộ lệnh `orca worktree/terminal/automations/repo/browser/emulator ...` — đây chính là bộ lệnh F09 mô tả, **đang được dùng hàng ngày** (chính môi trường chạy audit này cũng có skill `orca-cli` sẵn sàng).
- `docs/crs/v2/full-flow-tracing/CR-TRACE-010-cli-headless.md` (2026-08-01) đã trace chi tiết transport thật, khớp 100% với code hiện tại (`serveOrcaApp` vẫn ở đúng dòng 66 của `desktop/src/cli/runtime/launch.ts`).

→ Kết luận: **F09 KHÔNG phải "chưa làm"** như `frontend/backend-go/agent` gợi ý — nó đã hoàn thành và đang chạy, chỉ là ở gói `desktop/` (Electron main tách ra từ `backend/` cũ), thứ audit trước bỏ sót hoàn toàn. Việc sửa `feature-completion-matrix.md` được xử lý riêng ở CR-CLI-003.

### 2. Vấn đề thật của F09 ở "lớp backend-go": CLI có 2 transport, **cả 2 đều không đụng tới backend-go**

`RuntimeClient.call()` (`desktop/src/cli/runtime/client.ts:47-84`) chỉ rẽ nhánh giữa:

1. **Local** — `sendRequest()` (`desktop/src/cli/runtime/transport.ts:7`): mở Unix domain socket / named pipe (`transport.ts:24` `findTransport(metadata, 'unix', 'named-pipe')`), gửi frame `{ id, authToken, method, params }` (`transport.ts:177` `authToken: metadata.authToken`) tới `OrcaRuntimeRpcServer` **trong cùng tiến trình Electron main** (`desktop/src/main/runtime/runtime-rpc.ts`). `authToken` ở đây là secret cục bộ đọc từ file metadata trên đĩa (`readMetadata()`), **không liên quan `auth-service`**.
2. **Remote pairing** — `sendWebSocketRequest()` (`desktop/src/cli/runtime/websocket-transport.ts`): WebSocket + mã hoá E2EE (tweetnacl), dùng khi có `ORCA_PAIRING_CODE`/`ORCA_ENVIRONMENT` — đây là cơ chế pairing kiểu Mobile Companion (F03), **vẫn phải trỏ tới một tiến trình Electron main đang chạy ở đâu đó**, không phải backend-go.

`orca serve` (`serveOrcaApp()`, `desktop/src/cli/runtime/launch.ts:66-149`) — "headless mode" theo đúng lời F09 hứa ("Phù hợp cho Linux server không có display", "Dùng cho CI/CD pipeline") — thực chất chỉ `spawnProcess(executable, [...appArgs, '--serve', ...])` (`launch.ts:104`), tức là **respawn lại chính app Electron** ở chế độ nền, rồi CLI vẫn nói chuyện với nó qua Unix socket (nhánh 1). Điều này nghĩa là:

- Máy Linux "không có display" vẫn phải có **toàn bộ Electron runtime + app bundle** cài sẵn — không đúng tinh thần "server không có display" như spec F09 §"Headless Mode" mô tả, và không tương thích tốt với kịch bản SSH-only/CI runner tối giản mà AGENTS.md yêu cầu ("All changes must consider the SSH use case").
- **backend-go hoàn toàn không tham gia** vào bất kỳ lệnh CLI cốt lõi nào (`worktree.*`, `orchestration.*`, `terminal.*`, `automations.*`, browser/computer-use) — không có transport nào của CLI gọi tới `api-gateway`. Xác nhận bằng grep trực tiếp: không có tham chiếu `api-gateway`/`apiGateway`/`:8080` nào trong toàn bộ `desktop/src/cli/`.

### 3. Phát hiện quan trọng: backend-go **đã có sẵn** đúng bộ API mà CLI cần — được xây cho F22 (Web Server Mode), CLI chỉ chưa nói chuyện với nó

`backend-go/services/api-gateway/internal/adapter/wscompat/envelope.go:1-9` tự ghi chú: đây là "legacy channel-based WebSocket RPC protocol frontend/'s `WebSocketRpcClient` speaks... over a WS transport at `/ws`". Điều bất ngờ: **1 trong 2 dialect** mà `envelope.go` hỗ trợ (dòng 25-38, xử lý bởi `normalizeInboundMessage`/`session_dialect.go`) là format `WebSessionClient.call()` gửi:

```
{ id, authToken: 'cookie-auth', method, params }
```

— **cùng hệt shape** `{ id, authToken, method, params }` mà CLI's `sendRequest()` (`transport.ts:174-179`) đã dùng cho Unix socket từ trước. Tên channel cũng trùng khớp 1:1 với tên RPC method CLI gọi:

| Method CLI dùng | Channel đã có ở wscompat | File |
|---|---|---|
| `worktree.create`, `worktree.list`, `worktree.set`, `worktree.rm`, ... | ✅ `worktree.create` (dòng 40), `worktree.list` (213), `worktree.set` (263), `worktree.rm` (120), ... | `channels_worktree.go` |
| `terminal.create`, `terminal.send`, `terminal.resize`, `terminal.close`, `terminal.list`, ... | ✅ đầy đủ (`channels_terminal.go:228-516`) | `channels_terminal.go` |
| `git.push` (dùng bởi `worktree`/automation flows) | ✅ `git.commit`, `git.push`, `git.pull`, `git.diff`, ... (`channels_git.go:60-400`) | `channels_git.go` |
| browser (`goto`, `click`, `snapshot`, ...) | ✅ đăng ký động theo tên channel (`channels_browser.go:40`) | `channels_browser.go` |
| `orchestration.run` / `orchestration.dispatch` / `orchestration.send` (theo CR-TRACE-010) | ❌ **chưa có** — chỉ có `orchestration.dispatchShow` (24), `agentSession.listActive` (67) | `channels_orchestration.go` |
| `automation.runNow` (theo CR-TRACE-010) | ❌ **chưa có** — chỉ có `automation.create/runs/list/update/delete` (113-230), không có "run now" | `channels_automation_task.go` |

→ Với phần lớn domain (worktree/terminal/git/browser), **backend-go đã là 1 server đủ khả năng phục vụ CLI** — vì đây chính là API mà `frontend/`'s web-mode UI đang dùng khi Orca chạy ở "Web Server Mode" (F22, đã ✅ theo matrix). CLI chỉ đơn giản là chưa từng được viết để nói chuyện với nó. Domain orchestration/automation "run" vẫn thiếu ở backend-go — đây là gap có sẵn, không phải gap của CR này (xem "Không thuộc phạm vi").

## Giải pháp đề xuất

### A. Quyết định kiến trúc: thêm transport thứ 3, không thay thế 2 transport hiện có

`RuntimeClient.call()` (`client.ts:47`) rẽ thêm nhánh **`backend-go`** (bên cạnh `unix/named-pipe` và `ws-remote` E2EE hiện có), kích hoạt khi có biến môi trường mới, ví dụ `ORCA_SERVER_URL` (trỏ tới `api-gateway`, ví dụ `https://orca.example.com`) — theo đúng cách `ORCA_PAIRING_CODE`/`ORCA_ENVIRONMENT` đã kích hoạt nhánh remote hiện tại. Thứ tự ưu tiên đề xuất: `ORCA_SERVER_URL` (backend-go) > `ORCA_PAIRING_CODE` (E2EE remote) > Unix socket cục bộ mặc định — vì đây là nhánh duy nhất không cần một tiến trình Electron đang chạy ở đâu đó.

Transport mới (`desktop/src/cli/runtime/backend-go-transport.ts`, đặt cạnh `transport.ts`/`websocket-transport.ts`):

- Mở kết nối WS tới `${ORCA_SERVER_URL}/ws` (dùng lại đúng path `envelope.go` đã document).
- Gửi cùng shape frame `{ id, authToken, method, params }` CLI đã dùng cho Unix socket — **tái dùng nguyên schema/logic build request đã có trong `sendRequest()`**, chỉ đổi lớp transport vật lý (giống cách `sendWebSocketRequest()` đã tái dùng cùng convention). Không cần đổi gì phía backend-go (`normalizeInboundMessage` đã tự nhận dạng dialect này).
- `authToken` **không** dùng secret cục bộ (`metadata.authToken`) hay E2EE pairing code — dùng bearer JWT thật lấy từ `auth-service` (xem CR-CLI-002 cho cách CLI lấy token khi chạy headless/CI, không có phiên đăng nhập trình duyệt). Đây là điểm bắt buộc phải làm đúng ngay từ đầu: **không tự chế cơ chế auth mới** — verify path phía server là `SessionValidator.ValidateToken` (`backend-go/services/api-gateway/internal/adapter/authclient/session_validator.go:51`, JWKS qua `JWKSClient`, `jwks_client.go:37`) — cùng đường bearer-JWT mà CR-RBAC-002 đang chuẩn hoá cho toàn bộ api-gateway.

### B. Định nghĩa lại `orca serve` / headless mode khi có `ORCA_SERVER_URL`

Khi CLI được cấu hình `ORCA_SERVER_URL`, `orca serve` **không** gọi `serveOrcaApp()`/`spawnProcess` nữa (không cần Electron binary trên máy headless) — coi server backend-go từ xa là "runtime" luôn sẵn sàng, `orca serve` chỉ cần xác thực + in trạng thái kết nối. Giữ nguyên hành vi `serveOrcaApp()` hiện tại làm fallback khi không có `ORCA_SERVER_URL` (dev cục bộ, máy có Electron cài sẵn) — không phá vỡ use case hiện tại.

### C. Cross-platform & SSH

Transport mới là HTTPS/WSS thuần — không có khái niệm Unix socket/named-pipe khác nhau giữa macOS/Linux/Windows nữa, nên tự động thoả yêu cầu "CLI hoạt động trên macOS, Linux, Windows" của F09 tốt hơn 2 transport hiện tại. Vì chỉ cần outbound HTTPS/WSS, hoạt động xuyên SSH port-forward hoặc kết nối trực tiếp tới `api-gateway` public endpoint — đúng "SSH Use Case" của AGENTS.md mà không cần thêm logic đặc biệt.

## Changes Required

| File | Thay đổi |
|------|---------|
| `desktop/src/cli/runtime/backend-go-transport.ts` (mới) | Transport WS mới, tái dùng frame shape của `transport.ts` |
| `desktop/src/cli/runtime/client.ts` | `RuntimeClient.call()` thêm nhánh thứ 3 theo `ORCA_SERVER_URL`; `isRemote`/`isBackendGo` getter |
| `desktop/src/cli/runtime/launch.ts` (`serveOrcaApp`, dòng 66) | Khi có `ORCA_SERVER_URL`, bỏ qua `spawnProcess`, chỉ xác thực + báo trạng thái |
| `desktop/src/cli/runtime/environments.ts` | Model hoá `ORCA_SERVER_URL` như 1 loại "environment" thứ 3, song song `pairing`/`local` hiện có |
| `backend-go/services/api-gateway/internal/adapter/wscompat/envelope.go`, `session_dialect.go` | Không đổi logic — chỉ xác nhận bằng test tích hợp rằng dialect CLI gửi được `normalizeInboundMessage` nhận đúng khi `authToken` là JWT thật thay vì literal `'cookie-auth'` |
| `docs/features/F09-orca-cli.md` | Cập nhật bảng "Yêu cầu kỹ thuật": headless mode ghi rõ 2 chế độ (Electron respawn cục bộ vs backend-go transport từ xa) |

## Không thuộc phạm vi CR này

- **Thêm channel `orchestration.run`/`orchestration.dispatch`/`orchestration.send` và `automation.runNow` vào wscompat** — đây là gap có sẵn ở chính backend-go (không phải do CLI), thuộc track F14/F36 đang 🚧; cho tới khi có, CLI qua backend-go transport phải trả lỗi rõ ràng (`method_not_supported_on_backend_go`) thay vì treo, KHÔNG tự ý implement tạm trong CR này.
- **Credential headless (API token/service account) cho `authToken`** — xem CR-CLI-002; CR này giả định đã có 1 cách hợp lệ để lấy bearer JWT khi viết code, nhưng không tự chế cơ chế đó.
- Đổi hành vi transport `unix`/`named-pipe`/E2EE-remote hiện có — giữ nguyên 100%, chỉ thêm nhánh mới.
- Computer Use (`orca click`/`orca fill` lên desktop UI ngoài Orca) qua backend-go — nằm ngoài mô hình wscompat (không có "desktop UI" khi chạy headless qua backend-go); giữ nguyên giới hạn chỉ khả dụng khi có Electron cục bộ.

## Tiêu chí chấp nhận

- [ ] `RuntimeClient.call()` chọn đúng transport theo thứ tự ưu tiên `ORCA_SERVER_URL` > `ORCA_PAIRING_CODE` > Unix socket mặc định.
- [ ] `orca worktree create/list/set/rm`, `orca terminal *`, browser commands (`goto/snapshot/click/...`) chạy được end-to-end qua `ORCA_SERVER_URL` trỏ tới 1 `api-gateway` thật, không cần Electron cài trên máy chạy CLI.
- [ ] `orca orchestration run`/`orca automations run` qua backend-go transport trả lỗi rõ ràng, không treo, kèm thông báo "chưa hỗ trợ ở backend-go, dùng transport cục bộ".
- [ ] Test thủ công qua SSH: chạy CLI trên 1 host Linux không cài Electron, chỉ có Node + biến `ORCA_SERVER_URL`, xác nhận `orca worktree create --json` thành công.
- [ ] Không có thay đổi hành vi cho user hiện tại không set `ORCA_SERVER_URL` (regression test cho 2 transport cũ).

## Impact analysis (gitnexus)

Chạy qua MCP `impact()` (repo `orca`, callgraph mode, upstream):

| Symbol | File | Risk | Impacted | Ghi chú |
|---|---|---|---|---|
| `sendRequest` | `desktop/src/cli/runtime/transport.ts` | **HIGH** | 32 (2 direct, 3 execution flow: `worktree create`, `automations edit`, `automations create`) | Không sửa hàm này — chỉ thêm hàm chị em `sendBackendGoRequest` cạnh nó; nếu sau này refactor `sendRequest` để dùng chung logic build-frame, phải chạy lại `impact()` trước khi sửa (bắt buộc theo CLAUDE.md) |
| `RuntimeClient` (class) | `desktop/src/cli/runtime/client.ts:23` | LOW | 5 (3 direct) | Đây là symbol **sẽ sửa trực tiếp** (`call()` method) — impact thấp vì phần lớn caller đi qua `client.call()` gián tiếp, không gọi thẳng transport |
| `serveOrcaApp` | `desktop/src/cli/runtime/launch.ts:66` | LOW | 1 direct, module `Handlers` | Sẽ sửa trực tiếp — blast radius nhỏ, nhưng vẫn phải chạy lại `impact()` ngay trước khi implement theo đúng CLAUDE.md |

`registerCliChannels`/`envelope.go`/`session_dialect.go` phía backend-go **không bị sửa** trong CR này (chỉ verify bằng test) nên không cần impact() cho các symbol đó ở đây — xem CR-CLI-003 cho phần liên quan tới `registerCliChannels`.

## Liên quan

- `docs/features/F09-orca-cli.md`
- `docs/crs/v2/full-flow-tracing/CR-TRACE-010-cli-headless.md` (trace transport gốc, vẫn khớp code hiện tại)
- `docs/roadmap/feature-completion-matrix.md` (gap #6)
- `skills/orca-cli/SKILL.md`
- `backend-go/services/api-gateway/internal/adapter/wscompat/envelope.go`, `session_dialect.go`, `channels_worktree.go`, `channels_terminal.go`, `channels_git.go`, `channels_browser.go`, `channels_orchestration.go`, `channels_automation_task.go`
- CR-CLI-002 (credential headless — phụ thuộc)
- CR-CLI-003 (đổi tên `channels_cli.go`, cập nhật matrix — độc lập)
- CR-RBAC-002 (chuẩn hoá bearer-JWT ở api-gateway — dùng chung path xác thực)
