# CR-REQ-033 backend: ghi chú triển khai

Ngày 2026-10-07, đợt I1 (`infra-fleet-service`). Task 033-01, 02, 03 DONE; 033-04, 05 và phần `request-service` của 033-06 để đợt R2.

## Đã triển khai và kiểm chứng

- Migration `0039_dev_server_capability_profiles` hai dialect, có down. Postgres 16 và MySQL 8 thật (container): up, down 1, up 1, CHECK `source`, cascade, tenant, đồng thời, unicode. Postgres kiểm RLS thật bằng role không phải superuser.
- Repository Postgres và MySQL (`CapabilityProfileStore`), use case `RefreshDevServerCapabilities` và `GetDevServerCapabilities`, handler gRPC `GetDevServerCapabilities` (trước đó trả `Unimplemented`), nối trong `cmd/server/main.go`, hook sau handshake `WithOnSessionAttached`, sự kiện `capabilities_changed`, config TTL và khoảng tối thiểu.
- Golden hợp đồng (bản sao fixture của agent), bốn kịch bản suy giảm, script so hash `backend-go/ci/check-agent-capability-golden.sh`.

## Quyết định lệch so với task

| Điểm | Quyết định | Lý do |
|---|---|---|
| Subject sự kiện | `orca.infrafleet.dev_server.capabilities_changed` | Cùng họ `orca.infrafleet.*` của service; solution/CR vẫn ghi `orca.infra.`, chưa sửa tài liệu |
| Thêm RPC vào server | Option `WithDevServerCapabilities`, không đổi `grpc.New` | 70+ tham số vị trí, mọi test gọi `New` |
| Lỗi probe | Chỉ `-32601` ra `handshake_only`; lỗi khác giữ hồ sơ cũ | Không hạ cấp agent khoẻ vì timeout |
| Gộp probe | Tự cài, không `x/sync` | `x/sync` là phụ thuộc gián tiếp trong go.mod |
| `NewRefreshDevServerCapabilities` | Bỏ tham số `resolver` | Không dùng |
| `Upsert` | Khoá hàng `dev_servers` trước, không `INSERT ... SELECT` | Hai probe đồng thời phải phân biệt được; tenant lạ phải ra `ErrNotFound` |
| RLS | `FORCE` + `NULLIF` + `WITH CHECK`, store đặt `app.tenant_id` mọi giao dịch | Theo quy ước RLS thật của đợt này; các bảng cũ của infra-fleet vẫn chưa đặt GUC |
| Golden | Một file hai khoá (`full`, `partial`) lấy nguyên từ agent | Một nguồn hợp đồng; không tạo `unknown_schema` (request-service suy giảm schema lạ) |

## Đối chiếu tên trường với agent

Khớp: handshake (`protocolVersion`, `buildVersion`, `features`, `agentVersion`, `platform`, `arch`, `nodeVersion`, `capabilities`, `tools`); 8 tên feature; `agent.capabilities` (`schemaVersion`, `probedAt`, `partial`, `agent{buildVersion,protocolVersion}`, `host{platform,arch,nodeVersion,cpuCount,memTotalMb,memFreeMb,diskFreeMb,loadAvg1}`, `tools[{id,installed,version}]`, `claude{installed,version,auth,flags}`, `env[{name,present}]`, `unknownTools`, `rejectedEnvNames`); tham số `refresh` (boolean). Không sửa agent. Lưu ý cho R2: `tools[].installed` và `claude.installed` có thể là `null` (đo quá ngân sách), và `tools` không chứa `claude` (nằm ở khoá `claude`).

## Chưa kiểm chứng

- Dev server và agent thật; độ trễ `agent.capabilities` qua SSH và relay-websocket; `grpcurl` thủ công.
- `buf lint` đỏ sẵn trên toàn repo (infrafleet.proto có 82 dòng); RPC mới thêm một dòng đặt tên response (`DevServerCapabilityProfile`). `buf breaking` không báo gì về infrafleet.
- CI chưa gọi script so golden (không có workflow backend-go).
- RLS của bảng mới đã kiểm bằng role thường trong test; chưa kiểm với role ứng dụng thật của môi trường triển khai (cần quyền `SELECT` trên `infra.dev_servers` kèm GUC, đã đặt trong cùng giao dịch).

## Câu hỏi mở / việc cho R2

- 033-04: client `request-service` phải gắn metadata tenant; `profile_json` có thể có `schemaVersion != 1` (lưu nguyên, suy giảm ở reader).
- 033-05: thay stub `Exec`/`ExecPrompt` ở `request-service/.../dev_server_executor.go`; `RelayByDevServer` chuyển tham số nguyên vẹn.
- 033-06: thêm bản sao golden vào `request-service/internal/adapter/grpcclient/testdata/agent_capabilities_v1.golden.json` (script đã sẵn sàng so hash), ba golden `execprompt`.
- Giới hạn `refresh=true` theo `lastAttempt` cho mọi caller; chưa có kiểm admin (quyết định thuộc người duyệt).
- Đã chạm file ngoài task: `.gitattributes` gốc (hai dòng LF cho golden), `backend-go/Makefile` (target `check-agent-capability-golden`), `gofmt` lại vài file của infra-fleet chưa sạch từ trước.
