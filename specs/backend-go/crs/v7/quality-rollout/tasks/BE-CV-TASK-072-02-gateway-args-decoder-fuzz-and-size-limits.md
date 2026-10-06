# BE-CV-TASK-072-02: Fuzz và giới hạn của `decodeCodeIntelArgs` ở api-gateway

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/codeintel_args_security_test.go` (mới), `.../wscompat/codeintel_args_fuzz_test.go` (mới)
**Depends on:** BE-CV-SOL-040-codeintel-channel-foundation (`decodeCodeIntelArgs`, `codeIntelChannelError`, `SetReadLimit`), BE-CV-TASK-072-01
**Status:** `[ ] TODO`

---

## Context

- PQ-14/ui-api §2.4: `SetReadLimit(320<<10)`; `reviewState.save` ≤ 256 KiB, `c4.save` ≤ 96 KiB, `quality.profile.save` ≤ 96 KiB, `quality.trace.confirm|link` ≤ 8 KiB, còn lại ≤ 16 KiB; `depth` 1..3 ngoài khoảng bị từ chối; `kinds` ≤ 32.
- Hiện gateway chưa gọi `SetReadLimit` (đã grep, không thấy). Mẫu fuzz: `mcpserver/conformance_flow_test.go:194` `FuzzJSONRPCDecode`.
- U8: phiên thiết bị bị từ chối `CODEINTEL_NOT_AUTHORIZED` trừ `settings.get`.

## Việc cần làm

1. Test bảng: khoá lạ, >1 phần tử `args`, kiểu sai, `projectId` > 64, `worktreeId` > 512, `pageToken` > 512, `depth` 0/4, `kinds` 33, vượt kích thước theo từng nhóm kênh.
2. Chạy `path-attack-vectors.json` trên mọi tham số đường dẫn.
3. Test `SetReadLimit`: khung > 320 KiB ngắt kết nối đúng cách (dùng server thử với `coder/websocket`).
4. Test phiên thiết bị cho cả 46 kênh (danh sách từ registry).
5. `FuzzDecodeCodeIntelArgs`: bất biến không panic, khoá lạ bị từ chối, vượt giới hạn bị từ chối; hạt giống từ vector.

## Kiểm thử

- `go test ./internal/adapter/wscompat/... -run CodeIntel`; fuzz `-fuzz FuzzDecodeCodeIntelArgs -fuzztime 10s`.

## Tiêu chí hoàn thành

- [ ] Mọi giới hạn nhóm kênh có test; không clamp ngầm.
- [ ] Fuzz 10 s không panic.

## Rủi ro và lưu ý

- Đổi read limit chạm toàn `/ws` (O-2): cần người duyệt; chạy `gitnexus_impact` trước khi sửa `handler.go`.
