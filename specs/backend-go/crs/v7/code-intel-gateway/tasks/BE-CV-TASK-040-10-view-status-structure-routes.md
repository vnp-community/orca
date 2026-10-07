# BE-CV-TASK-040-10: Kênh `codeIntel.status`, `codeIntel.structure`, `codeIntel.routes`

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_status.go` (mới), `channels_codeintel_view_graph.go` (mới), `channels_codeintel_view_status_test.go` (mới), `channels_codeintel_view_graph_test.go` (mới), `channels_codeintel_register.go`
**Depends on:** TASK-040-07; stub `GetIndexStatus` (BE-CV-SOL-012-index-status-aggregation), `GetStructure`, `GetRouteMap` (BE-CV-SOL-020/021)
**Status:** [x] COMPLETED

---

## Context

Kênh đầu tiên là mẫu cho 14 kênh còn lại (SOL-040-view-channels 2.1, 2.2). `status` trả `IndexStatus` **phẳng**: `{overall, tools[], scopeMismatch, activeJob?, indexBasis[], lastStatusAt, errorCode?, binding}` (UI-API 3.1, 4.1; PQ-08), `overall` chữ HOA (PQ-32). `structure`, `routes` trả `Env<ModuleGraph>` / `Env<RouteMap>` (+ `nextPageToken`).

## Việc cần làm

1. `statusArgs{sel, Refresh bool}`; gọi `Core.GetIndexStatus`; encoder trực tiếp `resp.GetStatus()` (không `encodeEnvelope`); `activeJob.percent` vắng => `null` (nullable table); `binding.indexScope` ∈ `exact|repo_root|unresolved`.
2. `structureArgs` và `routesArgs` theo bảng 2.2 (path an toàn, `depth` 1..3, `limit` 1..500 cho routes, `pageToken ≤ 512`, `ifNoneMatch ≤ 80`); `Core.GetStructure`, `Core.GetRouteMap`; `nextPageToken` nguyên văn.
3. Đăng ký ba kênh trong `registerCodeIntelViewChannels` (hàm mới, thêm vào `registerCodeIntelChannels`); timeout: `status` 8 s, hai kênh còn lại 20 s (từ catalog).
4. Lấy tên trường request/response từ proto thật; ghi chênh lệch với SOL vào PR.

## Kiểm thử

- Bảng validate: `depth` 0/4, `path` `../x`, `limit` 0/501 (routes), `ifNoneMatch` 81 ký tự, khoá cấm.
- Fake client: kiểm `Selector.ProjectId/WorktreeRef`, `Refresh`; `status` trả `overall=OVERLAY` giữ HOA; `activeJob` nil => khoá vắng; `tools: []` khi rỗng.
- `Unimplemented` => `CODEINTEL_UNAVAILABLE`; `DeviceID != ""` => `NOT_AUTHORIZED` (kể cả `status`).
- Session-client: `params` vắng cho `status` => lỗi `INVALID_PARAMS` nêu `projectId` (sel bắt buộc).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelView(Status|Graph)'`.

## Tiêu chí hoàn thành

- [x] Ba kênh trả đúng shape UI-API 4.1/4.2; `status` không có `data`.
- [x] Tham số ngoài khoảng bị từ chối.
- [x] Placeholder của ba kênh được thay.

## Rủi ro và lưu ý

- `GetIndexStatus` có thể gọi agent (`refresh:true`): deadline 8 s ngắn hơn 20 s đọc; service trả trạng thái cache khi hết hạn (theo CR-012; chưa kiểm chứng).
- Không có `ifNoneMatch` ở `status` (D3).
