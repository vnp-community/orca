# BE-CV-TASK-040-15: Bộ kiểm hợp đồng cho 15 kênh đọc (shape, tenant, dialect, hai kiểu lỗi)

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_conformance_test.go` (mới), `testdata/codeintel_view_golden/*.json` (mới)
**Depends on:** TASK-040-10..14
**Status:** [ ] TODO

---

## Context

Chốt chất lượng chung cho nhóm đọc: không khoá snake_case, mảng không `null`, tiền tố lỗi, cô lập tenant, hai dialect, trần phản hồi, tham số cấm (UI-API U1–U9; CR-040 mục 4).

## Việc cần làm

1. Bảng test duyệt **15 kênh** (liệt kê cứng) với fake client trả proto mẫu tối thiểu: gọi qua `Registry.Dispatch`, khẳng định kết quả là `json.RawMessage` hợp lệ, không khoá khớp `^[a-z0-9]+_[a-z0-9_]+$`, không `null` ở vị trí mảng của hợp đồng (danh sách đường dẫn mảng lấy từ golden).
2. Golden JSON (một file/kênh) tạo bằng message proto mẫu; cập nhật có chủ đích bằng cờ `-update` (mẫu `catalog_test.go` `update`).
3. Tham số cấm: mỗi kênh với `{"tenantId":"x"}`, `{"workspaceRoot":"/x"}`, `{"repo":"r"}`, `{"cypher":"c"}`, `{"args":[]}`, `{"command":"c"}`, `{"devServerId":"d"}`, `{"role":"admin"}`, `{"userId":"u"}`, `{"deviceId":"d"}` => `INVALID_PARAMS` nêu tên trường, fake **không** được gọi.
4. Cô lập tenant: fake ghi `metadata` đầu vào; hai `Identity` khác tenant => metadata khác nhau; không có đường nào lấy tenant từ args.
5. Hai dialect cho 3 kênh đại diện (`status`, `structure`, `symbol`): native `{type:"result"}` và session-client `{ok:true}`; lỗi `CODEINTEL_X` ở `message`/`error.message`.
6. Trần phản hồi mỗi kênh (2 MiB; `symbol` 320 KiB) và deadline (`status` 8 s, còn lại 20 s).
7. `ifNoneMatch` 81 ký tự bị từ chối ở mọi kênh trừ `status`.

## Kiểm thử

`go test ./internal/adapter/wscompat/ -run 'CodeIntelViewConformance'`; chạy lại `go test ./internal/adapter/mcpserver/tools/...` (parity vẫn xanh).

## Tiêu chí hoàn thành

- [x] 15 kênh pass cả sáu nhóm kiểm.
- [x] Golden được review (diff có chủ đích).

## Rủi ro và lưu ý

- Golden phụ thuộc proto; đổi proto là đổi golden: dùng `-update` + review, không sửa tay.
- Kênh chưa có RPC (placeholder) được loại khỏi bảng bằng danh sách `skipUntilProto` có chú thích CR chặn; xoá khi nối.
