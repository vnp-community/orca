# Tasks vòng 2 (v2)

Ký hiệu: `[x]` xong và đã kiểm chứng, `[~]` làm một phần, `[ ]` chưa làm. Các mục `[x]` chưa commit.

## Đã sửa trong vòng này (bug nhỏ, mỗi bug có test)

- [x] TASK-V2-002 · Làm mới sức khoẻ dev server ngay khi agent attach · `PollFleetHealth.PollDevServer`, `agentwsserver.Server.OnAttached`, test usecase và test server
- [x] TASK-V2-003 · Tắt infra-fleet đóng session bằng 1001 để agent tự nối lại · `session.shutdown`, test mã đóng 1001 và 1000
- [x] TASK-V2-004 · Chủ đề outbox `dev_server.health_degraded` nằm dưới stream NATS · đổi hằng, migration `0040` (postgres, mysql), test chặn chủ đề ngoài stream; `redeploy-local.sh` tự chạy migration của dịch vụ vừa dựng
- [x] TASK-V2-005 · Bộ lọc trạng thái của `request.list` nhận `awaiting_information` · test
- [x] TASK-V2-006 · Id project sai định dạng là NotFound ở repo Postgres · test tích hợp (chưa chạy được ở máy này vì thiếu CLI `migrate`; đã kiểm bằng e2e `NFC-03/08` trên stack thật)
- [x] TASK-V2-007 · Bỏ qua liên kết ephemeral VM khi id agent không phải UUID · test
- [x] TASK-V2-008 · Test AIS-02 chờ run bất đồng bộ; thăm dò RQL-20 đăng ký dọn Request; suite hợp đồng `notfound`; `report_orphan_requests.py`

## SOL-V2-001: giai đoạn Plan (BUG-V2-001) — XONG

- [x] TASK-V2-009 · Miền: `PlanShapeFor` (16 ca loại × size), `PlanLimitsFromEnv`, `NormalizePlanProposal`, `ValidatePlanProposal` (25 ca), mã lỗi `REQUEST_PLAN_*`, `RunKindPlan`, `domain.StartGateSubject` dùng chung
- [x] TASK-V2-010 · Hạ tầng run: migration `0093_analysis_runs_plan_kind` (postgres, mysql; đã chạy up/down trên MySQL 8 thật và cả hai dialect qua hợp đồng), `StartRun` không cần Solution nháp, kịch bản hợp đồng `PlanRunNeedsNoDraftSolution` PASS trên Postgres và MySQL; `solution.list` không còn lẫn run `plan`
- [x] TASK-V2-012 · `BuildPlanPrompt` (luật theo hình dạng và loại), `ParsePlanOutput`/`EncodePlanProposal` (che bí mật), worker `RunPlanGeneration` (thử lại một lần, lỗi do trạng thái project không thử lại), nối vào `AnalysisRunner.WithPlanWorker`
- [x] TASK-V2-013 · `GeneratePlan` (chạy nền, làm lại khi đang chờ duyệt kèm góp ý), `GetPlanProposal`, handler gRPC `server_plan.go` và ánh xạ proto, test ánh xạ hai chiều
- [x] TASK-V2-014 · `TaskClient.CreatePlanTree` (cổng `TaskPlanWriter`), `CommitPlan` (kiểm lại, gọi task-service trước giao dịch, ghi `plan_task_id`, sự kiện `orca.request.plan.generated`, chuyển `plan_ready`, mở Approval theo `StartGate`); chạy lại an toàn, làm lại Plan có `supersedes_plan_id`, tự rơi về tạo mới khi tham chiếu cũ lệch
- [x] TASK-V2-015 · `PlanArtifacts` cho `plan` và `task_list` (digest cây con), nối vào registry duyệt
- [x] TASK-V2-016 · `wire_plan.go`, `REQUEST_PLAN_MAX_*`, mã lỗi vào CONTRACT; ghi chú: CommitPlan tắt khi Approval tắt (có cảnh báo)
- [x] TASK-V2-017 · E2E với Claude thật: `AIS-04` PASS; suite `plan` PLN-01..08 PASS (task_list và change_request có Phase đi từ Solution đã duyệt; commit, lặp lại, từ chối, sai trạng thái, vòng phụ thuộc)
- [ ] TASK-V2-011 · (ngoài phạm vi, theo dõi) Hotfix `single_task`: tạo task sửa trong `analyzing`. Chưa làm: hotfix vẫn không có Plan

## Kết quả thực thi (2026-10-09)

- e2e đầy đủ: **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP** (460 giây, Claude thật). BLOCKED/SKIP: MCP tắt (2), AIS-01/03 (không dựng được đường "không có AI" khi AI sẵn có), TSK-08 tuỳ chọn.
- `go test ./...` xanh ở `request-service` (kể cả `-race` cho test Plan), `api-gateway`, `project-service`, `task-service`, `workflow-service`; `infra-fleet-service` xanh trừ hai test socket unix có sẵn.
- Tích hợp Postgres và MySQL: hợp đồng Solution (gồm run `plan`) PASS.
- Chưa kiểm chứng: test tích hợp `project-service` Postgres (thiếu CLI `migrate`); `buf lint/breaking`; việc thực thi sau khi duyệt Plan (cố ý không duyệt trong e2e để không chạy agent AI trên máy dev; PLN từ chối Approval thay vì duyệt); Plan có Phase chạy `StartPhase` thật.
- Dữ liệu: 27 Request mồ côi trên môi trường local do các lượt thử cũ; cần người quyết định có chạy `report_orphan_requests.py --cancel` không.

## Định nghĩa "xong" của vòng

- [x] `run_acceptance.py` không còn FAIL (BLOCKED chỉ còn MCP và hai đường lỗi AI).
- [~] `go test ./...` xanh ở các service sửa; `infra-fleet-service` còn 2 test socket unix và `common/tracing` còn 1 test lỗi có sẵn.
