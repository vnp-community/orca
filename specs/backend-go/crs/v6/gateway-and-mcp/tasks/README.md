# gateway-and-mcp: tasks (backend-go, v6)

> 📋 Proposed. Mọi task `Status: [ ] TODO`; chưa task nào được làm hay chạy. Mỗi task đã đối chiếu với file thật của `api-gateway`; chỗ khác CR ghi ở mục Context của task.
> Hợp đồng: [`../CONTRACT-request-ui-api.md`](../CONTRACT-request-ui-api.md).

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| [BE-REQ-SOL-016](../solutions/BE-REQ-SOL-016-api-gateway-request-channels.md) | [TASK-REQ-016-01](./TASK-REQ-016-01-gateway-request-client-wiring.md) | Nối client `request-service` (config, dial, `ChannelDeps`) | P0 | CR-REQ-001 |
| | [TASK-REQ-016-02](./TASK-REQ-016-02-request-channel-errors-views-source.md) | Lỗi kênh, view camelCase, luật nguồn | P0 | 016-01 |
| | [TASK-REQ-016-03](./TASK-REQ-016-03-request-lifecycle-channels.md) | Kênh `request.*` (+ cờ) | P0 | 016-02, RPC CR-003..006, 012, 013 |
| | [TASK-REQ-016-04](./TASK-REQ-016-04-solution-and-approval-channels.md) | Kênh `solution.*`, `approval.*` | P0 | 016-02, CR-007, 009 |
| | [TASK-REQ-016-05](./TASK-REQ-016-05-backlog-channels.md) | Kênh `backlog.*` | P1 | 016-02, CR-015 |
| | [TASK-REQ-016-06](./TASK-REQ-016-06-request-subscribe-stream.md) | Stream `request.subscribe`, `requestEventRegistry` | P1 | 016-01, 016-02 |
| | [TASK-REQ-016-07](./TASK-REQ-016-07-http-routes-and-docs.md) | 5 route HTTP, README gateway | P1 | 016-03, 016-04 |
| | [TASK-REQ-016-08](./TASK-REQ-016-08-supplementary-request-channels.md) | Kênh bổ sung `request.links`, `request.flow`, `request.checks` | P2 | 016-03, CONTRACT Q1 |
| [BE-REQ-SOL-017](../solutions/BE-REQ-SOL-017-mcp-request-tools-and-source.md) | [TASK-REQ-017-01](./TASK-REQ-017-01-request-flow-toolspec-pack.md) | `ToolSpec` 17 tool + loại trừ | P1 | 016-03, 016-04, 016-05 |
| | [TASK-REQ-017-02](./TASK-REQ-017-02-tool-origin-in-executor.md) | `ToolOrigin` trong executor, test schema | P1 | 017-01, 016-02 |
| | [TASK-REQ-017-03](./TASK-REQ-017-03-request-create-rate-limiter.md) | Hạn mức `MCP_REQUEST_CREATE_PER_HOUR` | P1 | 017-02 |
| | [TASK-REQ-017-04](./TASK-REQ-017-04-untrusted-output-and-golden.md) | Không tin cậy, che, golden | P1 | 017-01, 017-02 |
| | [TASK-REQ-017-05](./TASK-REQ-017-05-mcp-e2e-and-guides.md) | e2e MCP và hướng dẫn | P2 | 017-01..03 |

## Thứ tự phụ thuộc

```
CR-REQ-001 (proto) ─▶ 016-01 ─▶ 016-02 ─┬─▶ 016-03 ─┬─▶ 016-07
                                         ├─▶ 016-04 ─┤        
                                         ├─▶ 016-05  │  (CR-015)
                                         └─▶ 016-06  └─▶ 016-08 (tuỳ chọn)
                       016-03 + 016-04 + 016-05 ─▶ 017-01 ─▶ 017-02 ─┬─▶ 017-03 ─▶ 017-05
                                                                      └─▶ 017-04
```

## Ghi chú

- Mỗi task thêm kênh cũng thêm dòng vào `excluded_channels.yaml` để parity test xanh ở từng PR; 017-01 gỡ dòng tạm của kênh có tool.
- 016-03 và 016-06 phụ thuộc quyết định "RPC trả sớm" của CR-REQ-005, 012 (CONTRACT Q2); nếu chưa chốt, hai kênh AI vẫn làm được với deadline 24 giây và chịu lỗi `REQUEST_AI_COMPLETE_TIMEOUT`.
- Lệnh kiểm chung: `cd backend-go/services/api-gateway && go build ./... && go test ./internal/adapter/wscompat/... ./internal/adapter/httpgateway/... ./internal/adapter/mcpserver/...`. Chưa chạy.
- Trước khi sửa symbol hiện có (`updateIssueStatus`, `runGuarded`, `RegisterProductionChannels`) chạy `gitnexus_impact` theo quy ước dự án.
