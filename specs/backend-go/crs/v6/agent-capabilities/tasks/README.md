# agent-capabilities: tasks backend (TASK-REQ-033-01 đến 06)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Mỗi task làm được trong 0,5 đến 2 ngày.

Solution: [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md). Danh sách solution: [solutions/README](../solutions/README.md). Phần agent: [`specs/agent/crs/v6/agent-capabilities/`](../../../../../agent/crs/v6/agent-capabilities/tasks/) (`AG-REQ-TASK-033-*`).

## Bảng Solution → Task

| Task | Tên | Priority | Service | Phụ thuộc |
|---|---|---|---|---|
| [TASK-REQ-033-01](./TASK-REQ-033-01-capability-profile-migration-and-repository.md) | Migration `0039`, domain `CapabilityProfile`, repository hai dialect | P1 | `infra-fleet-service` | không |
| [TASK-REQ-033-02](./TASK-REQ-033-02-handshake-info-protocol-and-features.md) | `HandshakeInfo` mang `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion` ở ba đường kết nối | P1 | `infra-fleet-service` | không |
| [TASK-REQ-033-03](./TASK-REQ-033-03-refresh-get-capabilities-usecases-and-rpc.md) | `RefreshDevServerCapabilities`, `GetDevServerCapabilities`, RPC proto, kích hoạt sau handshake | P1 | `infra-fleet-service`, `proto` | 01, 02 |
| [TASK-REQ-033-04](./TASK-REQ-033-04-request-service-capability-reader.md) | `DevServerCapabilityReader` và `SelectReadonlyRoute` ở `request-service` | P1 | `request-service` (mới) | 03, TASK-REQ-001-01..05, TASK-REQ-007-04 |
| [TASK-REQ-033-05](./TASK-REQ-033-05-agent-exec-prompt-go-client.md) | Client Go của `agent.execPrompt` mới (`accessMode`, `workspaceKind`, `reportChanges`, `resultBlock`, `parsed`, `changes`) | P1 | `request-service` (mới) | 04, TASK-REQ-008-02 |
| [TASK-REQ-033-06](./TASK-REQ-033-06-contract-golden-and-degradation-tests.md) | Golden hợp đồng với agent, kịch bản suy giảm | P1 | `infra-fleet-service`, `request-service` | 03, 04, 05 |

## Sơ đồ thứ tự

```
        ┌── TASK-033-01 (migration, domain, repo) ──┐
 start ─┤                                            ├─▶ TASK-033-03 ─▶ TASK-033-04 ─▶ TASK-033-05 ─▶ TASK-033-06
        └── TASK-033-02 (HandshakeInfo) ────────────┘                    ▲               ▲
                                                          TASK-REQ-001-01..05     TASK-REQ-008-02
```

01 và 02 song song. 04 và 05 ở `request-service` chờ module của feature request-service-foundation; nếu chưa có thì vẫn viết được ở nhánh riêng nhưng chỉ chạy test sau khi `go.work` có module.

## Ghi chú

- **Thứ tự triển khai chung với agent:** agent trước (build `agent/out/agent.js`, tăng `AGENT_VERSION`), backend sau. Backend suy giảm về hồ sơ `handshake_only` nếu agent chưa nâng cấp, nên các task 01 đến 03 không bị chặn bởi agent.
- **Hợp đồng không được đổi tên** so với CR-REQ-033: `accessMode`, `workspaceKind`, `reportChanges`, `resultBlock{nonce}`, `maxOutputBytes`, kết quả `changes`, `parsed`, `warnings`, `truncated`; handshake `protocolVersion`, `buildVersion`, `features`; RPC `agent.capabilities`.
- **Phụ thuộc tiến:** TASK-REQ-029-03 (task-service gửi `resultBlock`/`reportChanges` bằng struct riêng) và TASK-REQ-029-06 (`ReadinessGate` dùng `DevServerCapabilityReader`) dựa vào hợp đồng đã khoá ở đây.
- **Chưa kiểm chứng:** hành vi `claude --print --permission-mode plan --tools`, tên trường `claude auth status --json`, độ trễ `agent.capabilities`, `Relay` có chuyển `error.data` hay không. Mỗi task nêu rõ chỗ này ở mục Rủi ro.
- Mọi migration và repository hai dialect; không `max-lines` disable; không tên file `helpers`/`utils`/`common`/`misc`.
