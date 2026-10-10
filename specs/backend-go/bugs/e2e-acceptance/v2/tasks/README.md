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
- [x] TASK-V2-011 · Hotfix `single_task`: `EnsureHotfixFixTask` tạo task sửa duy nhất khi chẩn đoán ghi xong (`AnalysisResultWriter.Persist`), chuyển `analysis_ready` và mở Approval `pre_deploy` trên task trong cùng giao dịch; idempotent, bật cùng Approval. Test đơn vị/domain, hợp đồng Postgres+MySQL (`HotfixFixTaskAndPreDeployApproval`), e2e Go E10. Kết quả chi tiết ở mục 3 của [SOL-V2-001](./../solutions/SOL-V2-001-plan-stage.md). Chưa kiểm với Claude/agent thật; hotfix tạo trước bản sửa đang kẹt ở `awaiting_plan_approval` phải huỷ hoặc trả về backlog rồi mở lại

## Lỗi tìm thêm trong đợt hoàn thiện (đã sửa)

- [x] TASK-V2-018 · [Thông báo duyệt làm kẹt outbox: `ListUsers` không có actor](./TASK-V2-018-approval-notification-directory-without-actor.md). RPC nội bộ `AuthService.ListActiveTenantAdmins` (token `AUTH_DIRECTORY_CALLER_TOKEN`, fail-closed), lỗi vĩnh viễn không chặn sự kiện khác. Sau redeploy: 0 dòng `AUTH_NO_ACTOR` trong log request-service
- [x] TASK-V2-019 · Với `change_request` có cổng Phase, Approval của Phase đầu không bao giờ được mở nên Request kẹt ở `executing`: `StartExecution` gọi `EvaluateExecution` và `completePhases` mở Approval Phase kế (idempotent). Test `TestStatusConsumer_GatedFlowOpensTheFirstPhaseApprovalOnce`, e2e Go E01 cả hai dialect (ghi trong TASK-E2E-028)
- [x] TASK-V2-020 · `AST-05`: AI đề xuất `hotfix`/`security` trong tenant một người dùng → `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` huỷ giao dịch phân loại, Request kẹt ở `classifying`. `ProposeRequestClassification` chấp nhận lỗi này (người báo xác nhận loại trực tiếp), lỗi khác vẫn huỷ. Test `TestPropose_NoEligibleApprover_DoesNotStrandTheRequest`, `AST-05` PASS (ghi trong TASK-E2E-026)
- [x] TASK-V2-021 · Con trỏ nil `*ProjectAccessLookup` gắn vào interface làm panic `GetRequest` khi thiếu `PROJECT_INTERNAL_CALLER_TOKEN`: `projectAccess.existence()` + `wire_project_access_test.go`
- [x] TASK-V2-022 · [Lỗi backend từ UI live: panic gateway, `task.list` tab Plan, Solution không tự bắt đầu](./TASK-V2-022-ui-live-findings-backend.md)
- [x] TASK-V2-023 · [Loại chỉ đọc thiếu `repo_path`; câu hỏi chặn của Solution không có đường trả lời](./TASK-V2-023-readonly-analysis-repo-path-and-blocking-questions.md). Resolver lấy đường dẫn từ repo mặc định (không đoán), kênh `clarification.request`, câu trả lời vào prompt phân tích lại
- [x] TASK-V2-024 · [Đường thực thi sau duyệt Plan: cổng Phase không mở vì thiếu danh tính ở ngữ cảnh nền; task Plan có `depends_on` bị giao cho engine orchestration và kẹt `in_progress`](./TASK-V2-024-execution-path-identity-and-engine-selection.md). Xong về mã và test (hợp đồng + e2e Go hai dialect); chưa chạy trên stack thật, cần redeploy request-service và task-service
- [x] TASK-V2-025 · [Danh tính thực thi khi người duyệt khác người commit Plan (grant `user` tự cấp); xem lại kết quả chạy của task (`execution.get`, RPC `GetTaskExecutionOutput`); dev server bận (`busy`, `devServer.statusChanged`) và dữ liệu handshake](./TASK-V2-025-executor-identity-run-history-and-busy-state.md). Xong về mã và test (hợp đồng + e2e Go hai dialect, tích hợp task-service); chưa chạy trên stack thật, cần redeploy request-service, task-service, api-gateway, infra-fleet-service
- [x] TASK-V2-027 · [`git.status` của worktree task trả rỗng vì gateway giải sai đáp án của agent; `CheckWorktreeDeleteSafety` luôn thấy worktree relay là sạch](./TASK-V2-027-task-worktree-git-status-empty.md)
- [x] TASK-V2-026 · [Output của agent không tới UI (stream `StreamExecOutput` theo `connectionId` rỗng khi project gắn dev server qua `devServerId`); `platform`/`arch`/`nodeVersion` luôn null (lỗi đọc cột ở repository hai dialect, không phải ở agent)](./TASK-V2-026-agent-output-stream-and-dev-server-platform.md). Xong về mã và test (NATS thật cho đường publish và subscribe, tích hợp hai dialect); chưa chạy trên stack thật, cần redeploy infra-fleet-service, task-service, api-gateway

## SOL-V2-002: repo luôn gắn host, không chạy git "cục bộ" một cách im lặng ([solution](../solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md); frontend: [SOL-FE-E2E-004](../../../../../frontend/bugs/e2e-acceptance-v1/solutions/SOL-FE-E2E-004-web-repo-host-binding.md))

Gốc: [TASK-FE-E2E-015](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-015-web-add-project-drops-dev-server.md) (`WORKTREE_CREATE_FAILED ... chdir ... no such file or directory`). Quyết định đã chọn: repo luôn gắn dev server; project tạo gắn dev server và repo thừa hưởng khi request không nêu; local chỉ qua `ORCA_LOCAL_REPO_ROOT` (mặc định tắt); repo cũ không tự migrate (công cụ dry-run); Browse folder có giới hạn.

Thứ tự và phụ thuộc:

```
V2-030 (bước 1: SetReadLimit) ─► [FE-016/017/018 lên] ─► V2-028 (warn) ─► V2-031 (báo cáo repo mồ côi)
                                                              │                    │
                                                              ▼                    ▼
                                                        V2-028 (enforce) ◄── V2-029 (strict) ─► V2-032 ─► V2-033 ─► V2-034
V2-030 (bước 2-3: giới hạn browse) ─► FE-020
```

- [x] TASK-V2-028 · [project-service: chính sách host của repo (thừa hưởng từ project), `PROJECT_REPO_HOST_REQUIRED`, project gắn dev server không cần repo, `GetRepo` host hiệu lực](./TASK-V2-028-project-repo-host-policy.md)
- [x] TASK-V2-029 · [git-gateway: một điểm quyết định host, bỏ fallback im lặng về local, `GITGATEWAY_REPO_HOST_UNBOUND`/`GITGATEWAY_DEV_SERVER_UNREACHABLE`](./TASK-V2-029-git-gateway-no-silent-local-dispatch.md)
- [x] TASK-V2-030 · [infra-fleet nâng giới hạn đọc websocket (32 KiB); `browseDir` có trần mục và `truncated`](./TASK-V2-030-browse-limits-and-websocket-read-limit.md)
- [x] TASK-V2-031 · [Công cụ báo cáo repo không host và gắn lại dry-run; cải thiện `RebindRepoDevServer`](./TASK-V2-031-unbound-repos-report-and-rebind-tool.md)
- [~] TASK-V2-032 · [Test hợp đồng và tích hợp hai dialect cho chính sách host](./TASK-V2-032-repo-host-contract-and-two-dialect-tests.md)
- [~] TASK-V2-033 · [E2E: spec `--repro add-project-worktree` xanh + host ngắt giữa chừng, kênh tắt không host, duyệt thư mục lớn](./TASK-V2-033-e2e-add-project-host-binding-scenarios.md)
- [~] TASK-V2-034 · [Cấu hình triển khai, quan sát, tài liệu, rollout `warn`→`enforce`, `legacy`→`strict`](./TASK-V2-034-rollout-config-observability-docs.md)

Định nghĩa xong của SOL-V2-002:

- [ ] Spec repro `run_ui_flow.py --repro add-project-worktree` XANH (cùng frontend 016-018); kịch bản mở rộng TASK-V2-033 xanh.
- [ ] `go test ./...` xanh ở `project-service`, `git-gateway-service`, `api-gateway`, `infra-fleet-service` (trừ lỗi có sẵn đã ghi), tích hợp Postgres và MySQL xanh, `buf lint`/`buf breaking` xanh; vitest agent cho `fs.readDir`.
- [ ] Không hồi quy: `run_ui_flow.py --real all` 6/6, `run_acceptance.py` như trước.
- [ ] Tài liệu và cấu hình (TASK-V2-034); rollout đã ghi ngày từng bước.
- [ ] Dọn dữ liệu: `report_orphan_requests.py` và `report_unbound_repos.py` báo 0 mục thăm dò còn lại; repo thật của người dùng không bị đổi.

## Kết quả thực thi (2026-10-09)

- e2e đầy đủ: **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP** (460 giây, Claude thật). BLOCKED/SKIP: MCP tắt (2), AIS-01/03 (không dựng được đường "không có AI" khi AI sẵn có), TSK-08 tuỳ chọn.
- `go test ./...` xanh ở `request-service` (kể cả `-race` cho test Plan), `api-gateway`, `project-service`, `task-service`, `workflow-service`; `infra-fleet-service` xanh trừ hai test socket unix có sẵn.
- Tích hợp Postgres và MySQL: hợp đồng Solution (gồm run `plan`) PASS.
- Chưa kiểm chứng (bản đầu vòng 2): việc thực thi sau khi duyệt Plan và `StartPhase` thật với agent AI; xem mục "Đợt hoàn thiện" bên dưới cho phần đã được kiểm lại (`buf`, `migrate`).
- Dữ liệu: 27 Request mồ côi (toàn dữ liệu thăm dò của các lượt e2e cũ) đã được huỷ bằng `report_orphan_requests.py --cancel` theo quyết định của người dùng; chạy lại cho `0 orphaned`.

### Đợt hoàn thiện (cùng ngày, sau redeploy mọi service đã sửa)

- e2e đầy đủ với Claude thật: **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP**; `--strict --ai-stub --allow-blocked MCP-01,MCP-02`: **110 PASS, 0 FAIL, 2 BLOCKED, 1 SKIP**.
- e2e Go `-tags e2e`: xanh Postgres (33 giây) và MySQL (39 giây), 0 skip; `-tags integration` request-service xanh cả hai dialect.
- `buf breaking` xanh, `buf lint` không tăng cảnh báo, mã sinh khớp; test tích hợp `project-service` Postgres chạy được với CLI `migrate` (not-found PASS; 14 test worktree fail giống hệt trên `main`).
- Lỗi có sẵn, không do đợt này: `common/tracing` (1), `infra-fleet-service` (test ssh agent/socket), `git-gateway-service/localgit` (symlink macOS).
- Chưa kiểm: workflow CI trên runner GitHub thật; thực thi sau khi duyệt Plan với agent AI thật (`ORCA_E2E_EXECUTE_AGENT=1`); metric `orca_request_orphan_total` trên stack thật (mới test đơn vị); healthcheck âm (tắt token → unhealthy trong ~50 giây) chỉ kiểm bằng test đơn vị, healthcheck dương (`healthy` sau redeploy) đã thấy trên stack thật.

## Định nghĩa "xong" của vòng

- [x] `run_acceptance.py` không còn FAIL (BLOCKED chỉ còn MCP và hai đường lỗi AI).
- [x] `go test ./...` xanh ở các service sửa; còn lại là các test lỗi có sẵn trên `main` (xem trên).
