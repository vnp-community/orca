# TASK-V2-041: Cổng duyệt Phase mở cho Phase sẵn sàng (theo `depends_on`), không theo thứ tự liệt kê

**From:** e2e thật trên repo `deepseek-harness` (TASK-FE-E2E-028, 2026-10-11) · **Priority:** P0 · **Service/Area:** request-service (domain + usecase)
**Files:** `backend-go/services/request-service/internal/domain/execution_tree.go` (`NextReadyPhase`, `waitsOnUnfinishedPhase`), `internal/usecase/evaluate_execution.go` (`completePhases` dùng `nextReadyPhase`: một `GetSubtree` đọc cạnh `depends_on`), test `internal/domain/execution_tree_test.go` (mới), `internal/usecase/start_execution_test.go`
**Depends on:** không · **Status:** `[x]` XONG (2026-10-11): sửa, test đỏ khi bỏ sửa và xanh khi có, `go test ./...` của request-service xanh, đã redeploy; e2e thật `deepseek-execute` xanh trên Plan hai Phase

## Lỗi

`completePhases` mở Approval của `tree.NextPhase(started)`, tức Phase chưa xong và chưa bắt đầu **đầu tiên theo thứ tự task-service liệt kê**. Thứ tự đó không phải thứ tự phụ thuộc. Khi Phase phụ thuộc ("Add node:test suite", chờ "Implement slugify module") đứng trước trong danh sách:

1. Approval được mở cho Phase phụ thuộc;
2. `request.startPhase` bị `REQUEST_PHASE_PREDECESSOR_NOT_DONE` (đúng, theo `StartPhase.checkPredecessors`);
3. Phase mà nó chờ không bao giờ có cổng (chỉ mở một cổng một lúc, "next waits"): Request kẹt ở `executing`, không lối ra ngoài huỷ.

Tái hiện thật: hai lượt `change_request` hai Phase trên `deepseek-harness` (một lượt kẹt tới giới hạn 25 phút, một lượt bị từ chối khi Start). Các lượt một Phase không thấy.

## Việc cần làm

- [x] `ExecutionTree.NextReadyPhase(started, edges)`: bỏ qua Phase đang chờ một Phase chưa xong (cạnh `From` chờ `To`; Phase bị huỷ không giữ), còn lại như `NextPhase`.
- [x] `EvaluateExecution.nextReadyPhase`: đọc cạnh `depends_on` bằng `GetSubtree` của Plan (cùng cách `refsWithEdges` và `StartPhase`); lỗi đọc là lỗi (reconcile thử lại), không đoán, vì cổng sai thì không bao giờ mở được cổng đúng.
- [x] Test miền (6 ca) và test usecase `TestStatusConsumer_FirstPhaseApprovalSkipsAPhaseThatWaitsForAnother` (Phase phụ thuộc đứng trước trong danh sách: cổng đầu tiên phải là Phase còn lại).
- [ ] Chưa làm: cổng sau khi Phase đầu xong dùng cùng hàm (đã đúng vì cùng đường `completePhases`; chưa có test riêng cho chuỗi ba Phase).

## Rủi ro · Ước lượng

LOW: một lời gọi đọc thêm mỗi lần đánh giá khi flow có cổng Phase; hành vi không đổi khi không có cạnh. 0,5 ngày.
