# BE-CV-TASK-023-04: Bảng timeout theo method cho `codeintel.*` và `quality.*`

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/exec_timeouts.go` (mới), `.../devserveragent/client.go` (xoá thân `execTimeoutForMethod` cũ, dòng ~412–418), `.../devserveragent/exec_timeouts_test.go` (mới), `client_test.go` (mở rộng)
**Depends on:** TASK-023-01
**Status:** [x] DONE

---

## Context

Hiện `execTimeoutForMethod` chỉ có `agent.execPrompt` = 15 phút; mọi method khác dùng `cfg.RequestTimeout` = 30 s (`config.go:57`, `callWithTimeout` bỏ qua timeout ≤ 0 → `RequestTimeout`, `session.go` ~1160–1170). PQ-13 đưa bảng cuối cùng.

## Việc cần làm

1. Tạo `exec_timeouts.go` với `execTimeoutOverrides` (map) và `codeIntelReadTimeout = 90 * time.Second` như SOL-023 mục 2.C; chuyển hằng `execPromptTimeout` vào bảng.
2. Hàm `execTimeoutForMethod` giữ **tên và chữ ký** cũ (`func(string) time.Duration`) để `Exec` không đổi; trả `0` cho `codeintel.status|reindex|reindexStatus|reindexCancel|watch` và mọi `quality.*` trừ `listProfiles` (45 s).
3. Ghi chú 2 dòng ở đầu file: agent luôn tự hết hạn trước Go; `ai.complete` = 120 s sẽ được thêm bởi BE-CV-SOL-093 (không thêm ở đây).
4. Không đổi `RequestTimeout` toàn cục và không đổi `invokeTimeout` 25 s của WS.

## Kiểm thử

- `cd backend-go/services/infra-fleet-service && go test ./internal/adapter/devserveragent/ -run 'ExecTimeout|ClientExec' -race`.
- Test bảng 12 ca: `codeintel.subgraph`, `overview`, `processes`, `process`, `impact`, `symbol`, `routes`, `detectChanges`, `structuralFacts` = 90 s; `codeintel.status|reindex|reindexStatus|reindexCancel|watch` = 0; `quality.listProfiles` = 45 s; `quality.run|runStatus|cancel|results|coverage` = 0; `agent.execPrompt` = 15 phút; `ports.scan` = 0.
- Mở rộng `TestClientExec_AgentExecPromptSurvivesLongerThanRequestTimeout` (`client_test.go:310`): ca `codeintel.subgraph` với `responseDelay` > `RequestTimeout` thành công; ca `codeintel.status` vẫn lỗi `timed out` ở `RequestTimeout`.

## Tiêu chí hoàn thành

- [x] Bảng 12 ca xanh.
- [x] Test hiện có của `agent.execPrompt` xanh không sửa.
- [x] Không còn hằng timeout rải rác ở `client.go` (grep `15 \* time.Minute` chỉ còn ở bảng).

## Rủi ro và lưu ý

- Chọn 90 s là hợp đồng, **chưa đo**; `codeintel.overview` lạnh ~5–7 s theo CR-002 (một lần đo).
- Khi Go cắt trước agent, lệnh vẫn chạy trên agent (`dropPending` không huỷ); điều này do agent tự hết hạn trước (agent contract §2.5).
