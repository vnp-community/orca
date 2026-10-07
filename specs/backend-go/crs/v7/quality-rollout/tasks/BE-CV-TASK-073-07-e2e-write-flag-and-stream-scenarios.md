# BE-CV-TASK-073-07: Kịch bản e2e ghi, quyền, cờ, stream (E05, E11–E14, E18) và ma trận RPC

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/e2e/reindex_test.go`, `e2e/permissions_test.go`, `e2e/feature_flag_test.go`, `e2e/review_state_test.go`, `e2e/dev_server_offline_test.go`, `e2e/stream_test.go`, `e2e/rpc_scenario_matrix_test.go` (mới, tag `e2e`)
**Depends on:** BE-CV-TASK-073-05, 073-06, BE-CV-TASK-072-06, BE-CV-SOL-024, 013-*, 011-*
**Status:** `[x] DONE`

---

## Context

- E05 `RequestReindex`: job → `reindexProgress` → `index_changed`; lần hai ⇒ `CODEINTEL_REINDEX_IN_PROGRESS`; khẳng định agent giả nhận argv có `--index-only` (kiểm `ReindexArgv` của agent giả) và (T3) `git status --porcelain` không đổi.
- E11 quyền (task 072-06); E12 cờ: tắt ⇒ `CODEINTEL_DISABLED`; bật/tắt/bật giữa job; tenant A không ảnh hưởng B; `GetSettings`/`SetSettings`/`GetReindexJob` luôn chạy; quality tắt ⇒ `QUALITY_GATE_DISABLED`.
- E13 `review_states`: `expected_version` cũ ⇒ `CODEINTEL_VERSION_CONFLICT`; ghi chú còn nguyên sau tắt rồi bật cờ.
- E14 dev server mất kết nối: `CODEINTEL_DEV_SERVER_OFFLINE` sau 20 s chờ (đồng hồ giả), cache cũ trả `stale`.
- E18 `StreamCodeIntelEvents`: nhận `changed`, `reindexProgress`; luồng đứt ⇒ `resync`; lọc theo quyền từng sự kiện.
- O-17: không có `CancelReindex`.

## Việc cần làm

1. Cài các kịch bản trên bằng harness task 05.
2. `rpc_scenario_matrix_test.go`: `TestEveryRPCHasScenario` đọc 49 RPC từ `ServiceDesc`; mỗi RPC xuất hiện trong ≥ 1 kịch bản (đánh dấu bằng bảng `scenarioCoverage`) hoặc nằm trong `waivedScenario` có `reason` và `ownerSolution`.
3. Chu trình bật-tắt-bật có kiểm dữ liệu giữ nguyên và job đang chạy hoàn tất khi tắt (đồng hồ giả để cache 5 s).
4. Kiểm migration `down` từ chối khi còn dữ liệu **nếu** CR-011 đã làm (`t.Skip` có tham chiếu nếu chưa).

## Kiểm thử

- `go test -tags=e2e ./e2e/...` hai dialect. Chưa chạy.

## Tiêu chí hoàn thành

- [x] E05, E11–E14, E18 xanh hai dialect; `TestEveryRPCHasScenario` xanh.
- [x] Mỗi waived có lý do và solution sở hữu.

## Rủi ro và lưu ý

- E15, E16 chỉ T3; E17 chỉ khi CR-041. Hạn "xếp hàng ~20 s" là của collector (README v7 §8 điểm 3).
