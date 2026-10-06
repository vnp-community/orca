# 01 — Agent chạy trên dev server và kết nối tới backend

Nguồn: `specs/agent/api/*` (đã đối chiếu với code `agent/src/relay/*` và `backend-go/services/infra-fleet-service`). Spec gốc mô tả backend TypeScript (`backend/src/...`); bản chạy thật là **Go**, trong `infra-fleet-service`. Các ý dưới đây ghi theo code hiện tại.

## 1. Agent là gì

Một process Node chạy trên dev server (thường dưới systemd), điểm vào `agent/src/relay/agent-entry.ts`. Nó:
- giữ một kết nối duy nhất tới backend;
- phục vụ JSON-RPC cho backend: `git.*`, `fs.*`, `pty.*`, `agent.*`, `ports.*`, `preflight.*`, `shell.*`, `tools/list`, `tools/call`…;
- đẩy sự kiện không cần hỏi: `pty.data`, `fs.changed`, `agent.hook`…

Agent có **hai bề mặt RPC độc lập** (spec `README.md`):

| | Part A "Dev Server Agent" | Part B "Orca Relay" |
|---|---|---|
| Router | `agent-rpc-dispatch.ts` (switch phẳng) | `dispatcher.ts` (`RelayDispatcher`, Map handler) |
| Mode | `direct-websocket`, `relay-websocket` | `relay-ssh` |
| `tools/list`, `tools/call` | **Có** (`agent-rpc-dispatch-misc.ts`) | **Không** (grep `relay.ts` không có) |

Hệ quả cho tính năng này: tool `gitnexus`/`codegraph` có sẵn ở Part A, không có ở Part B.

## 2. Ba chế độ kết nối

| Mode | Ai khởi tạo | Đường dẫn | Xác thực |
|---|---|---|---|
| `direct-websocket` (mặc định) | Agent gọi ra backend | `wss://<gateway>/agent` | token một lần, lấy qua `POST /api/agent-token`, gửi trong `agent.handshake` |
| `relay-websocket` | Backend gọi vào agent (cổng 6799) | `ws://host:6799/orca-relay` | `ORCA_AGENT_TOKEN` dài hạn (bearer hoặc `?token=`) |
| `relay-ssh` | Backend, qua SSH exec channel | stdin/stdout của `relay.js` | tin cậy SSH; có handshake phiên bản |

Phía Go tương ứng:
- `adapter/agentwsserver/` — nhận WS inbound ở `/agent`, chạy handshake phía nhận, kiểm tra token (lưu SHA-256, không lưu plaintext), rồi giao kết nối cho `devserveragent.Client.AttachInboundSession`.
- `adapter/devserveragent/` — client/session: gọi `Exec`, `ExecStream`, `StreamPty`, `StreamFileChanges`, `StreamExecOutput`…, giữ keepalive/reconnect.
- `adapter/sshrelay/` — dựng relay cho `relay-ssh`.

## 3. Wire

- Khung nhị phân 13 byte: `[type u8][seq u32][ack u32][len u32][payload]`; `Regular=1`, `KeepAlive=9`, tối đa 16 MiB/message.
- Payload là JSON-RPC 2.0, hai chiều (cả hai bên đều đăng ký handler và gọi được).
- Keepalive 5 s, timeout 20 s. Agent reconnect backoff `1,2,5,15,30 s`; token một lần nên mỗi lần reconnect lấy token mới.
- `agent.handshake` agent gửi: `{agentVersion, platform, arch, nodeVersion, capabilities[], agentToken?, devServerId, tools[]}`. Backend trả `{ok, orcaVersion, sessionId}`.
- `tools[]` trong handshake là tên các tool agent phát hiện được (`discoverTools`: tool có `binary` chỉ được liệt kê nếu binary nằm trong `toolPath`). **Backend biết dev server có `gitnexus`/`codegraph` hay không ngay từ handshake**; Go lưu qua `LastHandshakeInfo`.

## 4. Đường đi từ backend ra agent (đã có)

```
frontend --WS /ws (wscompat)--> api-gateway --gRPC--> infra-fleet-service
   --Client.Exec(devServer, method, params)--> session.call --JSON-RPC--> agent
```

Điểm quan trọng:
- `InfraFleetService.Relay / RelayByDevServer / RelayStream` là passthrough **không dịch method**: `connectionId|devServerId + method + params_json` → `result_json`. `git-gateway-service`, `workflow-service`, `task-service`, `project-service`, `ai-provider-service` đều đi qua đây (xem `git-gateway-service/.../relay_executor.go`).
- Giới hạn thời gian: mặc định 30 s (`cfg.RequestTimeout`); chỉ `agent.execPrompt` có ngoại lệ (`execTimeoutForMethod`).
- Frontend nói qua `api-gateway/internal/adapter/wscompat` (envelope `invoke/send/result/error/push`), đăng ký kênh bằng `Registry.Register(channel, handler)`; kênh stream dùng `StreamHandler` → `PushEvent`.
- Agent → backend là notification (`pty.data`, `fs.changed`, `agent.hook`…); backend chuyển thành server-streaming gRPC (`StreamPty`, `StreamFileChanges`, `StreamExecOutput`).

## 5. Sự cố/khoảng trống đã biết có ảnh hưởng

(`specs/agent/api/gaps-and-findings.md`, và đối chiếu code)
- Hai Part khác nhau về tên method/shape (`pty.create` vs `pty.spawn`…); tính năng mới phải nói rõ chạy trên Part nào.
- `agent.output/agent.exited` không có consumer phía backend — đừng dựa vào stream này cho dữ liệu mới.
- Backend Go **chưa gọi `tools/call` của agent ở đâu cả** (grep `tools/call` chỉ ra `mcpprober`, `mcpserver` — đó là MCP server của Orca, không phải agent). Tool `gitnexus`/`codegraph` hiện chưa có đường sử dụng.
