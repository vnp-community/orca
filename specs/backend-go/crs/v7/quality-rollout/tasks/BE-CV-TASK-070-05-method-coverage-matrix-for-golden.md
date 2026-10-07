# BE-CV-TASK-070-05: Ma trận bao phủ method và mã lỗi (`TestEveryAgentMethodHasGolden`)

**From Solution:** BE-CV-SOL-070
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/contracttest/agent_method_coverage_test.go` (mới), `.../internal/contracttest/agent_method_table.go` (mới, bảng dữ liệu, tên theo nội dung)
**Depends on:** BE-CV-TASK-070-01, BE-CV-TASK-070-04, BE-CV-SOL-021 (bảng method của collector)
**Status:** `[x] DONE`

---

## Context

- Bộ method công khai (PQ-21, agent-rpc §4–5): `codeintel.status|overview|processes|process|subgraph|impact|symbol|routes|detectChanges|reindex|reindexStatus|reindexCancel|watch|structuralFacts|codegraphSearch|files`, `quality.listProfiles|run|runStatus|cancel|results|coverage`. `codeintel.node` **không công khai** (§4.15).
- Mẫu "thêm mục mà quên thì đỏ": v6 CR-REQ-025 D5 (`TestEveryRequestTypeHasScenario`).
- `reindex*`, `watch` không dùng phong bì chung (agent-rpc §2.2); `quality.*` có kết quả riêng (§5): tệp vàng của `quality.*` thuộc CR-082/083 (BE-CV-SOL-082, 083); task này chỉ yêu cầu bảng phản ánh trạng thái "chưa có tệp, ghi lý do".

## Việc cần làm

1. `agent_method_table.go`: bảng `{method, envelope bool, goldenFiles []string, owner string, waivedReason string}` cho 16 method `codeintel.*` đọc và 6 `quality.*`; `waivedReason` không rỗng chỉ được dùng cho method chưa phát hành (ví dụ `watch`, `reindexCancel` vì không có RPC/kênh, O-17) và phải trỏ mục hợp đồng.
2. `TestEveryAgentMethodHasGolden`: với mỗi method không waived, mọi tệp trong `goldenFiles` tồn tại và có trong `MANIFEST.json`; mỗi `data.code` ở §3.2 có ít nhất một tệp `errors/`.
3. `TestAgentMethodTableMatchesCollector`: tập method của bảng khớp tập method mà collector (CR-021) có khả năng gọi (đọc từ hằng của collector); lệch hai chiều đều đỏ.
4. `TestNoPrivateMethodInGolden`: không tệp nào tham chiếu `codeintel.node`.

## Kiểm thử

- Ba test trên; thử tay: xoá `impact-found.json` thì đỏ, thêm hằng method vào collector mà không có dòng bảng thì đỏ.
- Không DB.

## Tiêu chí hoàn thành

- [x] 22 method được liệt kê; mỗi dòng waived có lý do và trỏ hợp đồng.
- [x] Thêm method ở một phía mà quên phía kia thì CI đỏ.

## Rủi ro và lưu ý

- Danh sách method đếm theo hợp đồng ngày 2026-10-06; nếu hợp đồng đổi, sửa bảng cùng PR hợp đồng (§8.1).
