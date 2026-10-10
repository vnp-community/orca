# TASK-V2-030: infra-fleet nâng giới hạn đọc websocket; `browseDir` có trần mục, `truncated`, không rớt agent

**From Solution:** SOL-V2-002 (mục 1.3, 3.6) · **Resolves:** `message too big: read limited at 32769 bytes` và rớt agent ~30 giây khi duyệt thư mục lớn
**Priority:** P0 (bước 1) / P1 (phần còn lại) · **Service/Area:** infra-fleet-service, api-gateway (wscompat), agent
**Files:**
- backend-go/services/infra-fleet-service/internal/adapter/devserveragent/transport.go (48 `newWSTransport`: `conn.SetReadLimit(MaxMessageSize + HeaderLength)`), session.go (241 Dial; 308 đọc bắt tay), client.go (371), frame.go (24, 73-74)
- backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go (132, 143 Accept; 167 đọc bắt tay)
- backend-go/services/api-gateway/internal/adapter/wscompat/channels.go (`devServer.browseDir` 984-1045; `rpcTimeout` 105), channels_files.go (`files.browseServerDir` 46-111): thêm `limit`, `directoriesOnly`, `query`; kết quả thêm `truncated`, `totalEntries`
- agent/src/relay/fs-agent-extensions.ts (`handleFsReadDir` 31-69, `readDirRecursive` 71-91: tham số `maxEntries`, `directoriesOnly`, `nameContains`, `includeSize:false`, sắp thư mục trước)
- specs/agent/api/agent-rpc-catalog-git-fs.md (dòng 92: ghi tham số mới)
**Depends on:** không (bước 1 độc lập, nên làm đầu tiên)
**Status:** `[x]` XONG qua TASK-V2-035 (2026-10-11), đã kiểm live; các mục dự kiến bị đơn giản hoá theo quyết định của người dùng (không có chế độ warn/legacy) ghi ở TASK-V2-035.
**Triển khai thực tế (2026-10-10, chưa commit, chưa redeploy):** [TASK-V2-035](./TASK-V2-035-repo-host-binding-and-no-silent-local.md) — bước đọc websocket và trần danh sách ở gateway (`limit`, `directoriesOnly`, `query`, `truncated`, `totalEntries`); chưa sửa `agent/**`

---

## Context

`github.com/coder/websocket v1.8.15` mặc định giới hạn đọc 32768 byte; không nơi nào gọi `SetReadLimit` dù trần khung là 16 MiB. Mỗi mục `fs.readDir` Phần A mang `path,name,type,size` và agent `stat` từng file; gateway không giới hạn mục và `rpcTimeout` = 8 giây.

## Việc cần làm

- [ ] Bước 1 (infra-fleet): `SetReadLimit` ở `newWSTransport` và hai nơi đọc bắt tay; log rõ khi khung vượt 16 MiB. Test đỏ trước: phản hồi giả 200 KiB không làm rớt phiên (hiện đóng `StatusMessageTooBig`).
- [ ] Bước 2 (gateway): `limit` mặc định 500, tối đa 2000; `directoriesOnly` mặc định false; `query` không phân biệt hoa thường; `truncated/totalEntries`; truyền tham số mới xuống agent và tự cắt nếu agent cũ bỏ qua. `~` giữ hành vi hiện tại (ghi chú kênh).
- [ ] Bước 3 (agent): xử lý tham số mới, bỏ `stat` khi `includeSize:false`; tương thích ngược (tham số lạ bị bỏ qua).
- [ ] Lỗi có mã đi nguyên (`INFRA_DEV_SERVER_NOT_CONNECTED`, `INFRA_AGENT_RESPONSE_TOO_LARGE` khi vượt trần).
- [ ] Đo: liệt kê thư mục 20 000 mục (fixture) xong < 8 giây và không rớt agent.

## Tiêu chí hoàn thành

- Gõ `/private/var/folders/.../T` trong Browse folder trên stack thật: phản hồi bị cắt kèm `truncated:true`, agent KHÔNG rớt (`orca-go-infra-fleet` không có `dev server disconnected`).
- `go test ./...` xanh ở `infra-fleet-service` (trừ hai test socket unix có sẵn), `api-gateway`; vitest của `agent` xanh cho `fs-agent-extensions`.

## Kiểm thử

Go: `agentwsserver/server_test.go`, `devserveragent/session_test.go` (khung lớn); wscompat `channels_files_test.go`, `channels_test.go` (giới hạn, `truncated`, agent cũ không hỗ trợ). Agent: test `handleFsReadDir` với 5000 file. E2E Python `suite_dev_server_agent.py` thêm ca duyệt thư mục lớn (trong thư mục tạm của suite).

## Rủi ro · Ước lượng

LOW (bước 1) / MEDIUM (bước 2-3, đổi hợp đồng JSON theo hướng cộng). Redeploy: infra-fleet-service (bước 1); api-gateway + agent (bước 2-3). 1 đến 1.5 ngày.
