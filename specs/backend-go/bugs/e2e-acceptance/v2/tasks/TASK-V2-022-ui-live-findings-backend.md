# TASK-V2-022: Lỗi backend tìm thấy khi chạy e2e qua giao diện thật (gateway panic, task.list cho tab Plan, Solution không tự bắt đầu)

**From Solution:** (phát hiện khi chạy Playwright `request-live` trên stack thật, xem `specs/frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-008-live-stack-playwright-smoke.md`, mục "Kết quả thực thi") · **Resolves:** `GATEWAY-CONNECTION-DROPPED` / `UI-STALE-CLASSIFY` (panic), `PLAN-TAB-EMPTY` (phần backend), `SOLUTION-NOT-STARTED` (phần backend)
**Priority:** P0 (panic gateway), P1 (hai lỗi còn lại) · **Service/Area:** api-gateway, common/eventbus, request-service
**Files:**
- backend-go/common/eventbus/eventbus.go (`consumeUntilDone`) + eventbus/consume_stop_test.go
- backend-go/services/api-gateway/internal/adapter/wscompat/push_fanin_sink.go (+ test), channels_request_stream.go, channels_task_activity.go, channels_agent.go
- backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go, channels.go (`taskView.RequestID`) (+ test)
- backend-go/services/request-service/internal/usecase/generate_solution.go (+ test), internal/adapter/eventbus/clarification_resume_consumer.go
- specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md (mục 2.7)
**Depends on:** TASK-V2-018 (cùng lớp lỗi "ngữ cảnh nền chỉ có tenant")
**Status:** `[x]` XONG về mã và test (cập nhật 2026-10-10, chưa commit); **chưa chạy trên stack thật** (cấm redeploy trong đợt này)

---

## Context

Lượt e2e qua UI thật tìm ra bốn điều ở backend. Mục 1 đến 3 đã sửa; mục 4 chỉ điều tra.

## Lỗi 1: `panic: send on closed channel` ở api-gateway (đã sửa)

**Nguyên nhân gốc (có hai tầng):**

1. `common/eventbus.consumeUntilDone` (dùng chung bởi `Subscribe` và `SubscribeEphemeral`) gọi `cons.Consume(...)` rồi `<-ctx.Done()` và **trả về mà không dừng vòng consume**. Consumer JetStream cứ sống và cứ gọi callback sau khi `SubscribeEphemeral` đã trả về (rò rỉ consumer, callback chạy với `ctx` đã huỷ).
2. Ở gateway, `request.subscribe` (và `task.activity.subscribe`, `agent.subscribeStatus`) cho nhiều goroutine NATS gửi vào cùng một `out`, còn `defer close(out)` chạy khi `SubscribeEphemeral` trả về. `select { case out <- x: case <-ctx.Done(): }` trên kênh đã đóng vẫn panic khi nhánh gửi được chọn, dù `ctx.Done()` đã sẵn sàng. Tải lại trang hoặc đổi Request đang xem lúc có sự kiện là đủ để kích hoạt.

**Sửa:**
- `consumeUntilDone` dừng consume (`Stop`, chờ `Closed()` tối đa 2 giây) trước khi trả về, và NAK tin đã nằm trong bộ đệm khi `ctx` đã huỷ (để giao lại, không chạy handler sau khi huỷ). `Stop` không chờ callback đang chạy, nên tầng 2 vẫn cần.
- `wscompat/push_fanin_sink.go`: `pushFanInSink` là mô hình "một chủ sở hữu đóng": `send` giữ khoá đọc, `close` đóng kênh `done` trước (để người gửi đang chặn nhả ra), rồi lấy khoá ghi, đặt cờ và mới đóng `out`. Gửi sau khi đóng bị bỏ, không panic. Dùng cho ba kênh có mẫu "nhiều callback NATS vào một kênh". `agent.subscribeStatus` còn một lỗi phụ: hai goroutine, một cái `defer close(events)`, cái kia vẫn gửi; nay đóng sau khi cả hai kết thúc.

**Rà mọi chỗ có mẫu tương tự (grep `defer close(` và `SubscribeEphemeral` trong api-gateway):**
- Đã sửa: `channels_request_stream.go`, `channels_task_activity.go`, `channels_agent.go` (`agent.subscribeStatus`).
- An toàn, không đổi: `notifications.subscribe`, `workspacePorts.subscribe`, `starNag.subscribe`, `mcp.events.subscribe`, `accounts.subscribe`, `session.tabs.subscribeAll`, `mobile.statusSubscribe`, `terminal.*` (`drainAttachPtyOutput`), `ephemeralVm` (`drainVmProvisionOutput`), `files.watch`, `pipeGitProgressStream`, `terminal.subscribe`: mỗi kênh chỉ có **một** goroutine vừa gửi vừa đóng.
- An toàn: `ClientEventBus` và `WorkspaceEventBus` (gửi và đóng cùng giữ `mu`, gửi không chặn), `TraceBroadcast` (không đóng kênh), `mcpserver/resources.BusTaskEvents`, `watchToolsChanged`, `watchPromptChanges` (callback không ghi vào kênh bị đóng).

## Lỗi 2: `task.list` không phục vụ tab Plan (đã sửa)

Proto `ListTasksRequest` **đã có** `task_types`, `request_ids`, `parent_id` và task-service đã lọc đúng (kèm test hai dialect); thiếu là gateway không chuyển. Không đổi proto, không đổi task-service.

**Hợp đồng `task.list` cuối cùng** (CONTRACT mục 2.7), mọi trường mới đều tuỳ chọn, vắng thì hành vi như cũ (task-service ẩn `plan`/`phase`):

| Tham số | Kiểu | Ghi chú |
|---|---|---|
| `projectId`, `pageToken`, `pageSize` | như cũ | |
| `taskTypes` | `string[]` | ví dụ `["plan","phase"]`; giá trị sai trả `TASK_INVALID_TYPE` |
| `requestId` | `string` | gộp với `requestIds` |
| `requestIds` | `string[]` | tối đa 100 (`TASK_LIST_TOO_MANY_REQUEST_IDS`) |
| `parentId` | `string` | chỉ con trực tiếp |

Kết quả giữ `{tasks, nextPageToken}`; mỗi task thêm trường `requestId` (`omitempty`), để tab Plan nhóm theo Request.

**Phân quyền:** `ListTasks` ở task-service chỉ lọc theo tenant (không theo grant), trước và sau thay đổi này; không đổi. `CreatePlanTree` **đã** cấp grant owner (`ApplyTree`) cho `CreatorID` sau commit (`create_plan_tree.go`), và request-service truyền `CreatorID` là người commit (`commit_plan.go`, `ensure_hotfix_fix_task.go`); nếu cấp grant lỗi thì người tạo vẫn là `Task.OwnerID` (đã ghi log ERROR). Không cần sửa nghiệp vụ.

Tệp vàng `channel_inventory.json` **không đổi** (danh mục chỉ ghi tên và loại kênh, `task.list` đã có).

## Lỗi 3: Solution không tự bắt đầu sau Clarification (đã sửa; quyết định: TỰ ĐỘNG)

**Quyết định theo CR:** CR-REQ-028 mục 2.4 bước 5: "`to=analyzing` thì tự gọi `GenerateSolution` nội bộ với `feedback="clarification:<id>"`"; tiêu chí hoàn thành: "một run `GenerateSolution` mới khi `analyzing` (consumer dedup khi giao lặp)"; Q3 đề xuất "có" cho `GenerateSolution` và "không" cho `GeneratePlan`. Mã đã đúng thiết kế (`ResumeAfterClarification`, mặc định `REQUEST_CLARIFICATION_AUTO_REGENERATE=true`, đã được nối ở `wire_artifact.go`); **lỗi là một khiếm khuyết cài đặt**, không phải thiếu nối dây.

**Bằng chứng (log thật `docker logs orca-go-request`, chỉ đọc):** consumer có chạy, nhưng chỉ sinh các dòng `WARN resume skipped ... REQUEST_SOLUTION_WRONG_STATE: request in status planning|cancelled cannot start analysis` đúng lúc Request đã rời `analyzing` (khi test chọn option sang `planning`, hoặc khi dọn bằng huỷ), không bao giờ một run mới. Nghĩa là mọi lần giao trước đó trả lỗi **không** thuộc nhóm "bỏ qua", JetStream NAK rồi giao lại ngay (vòng lặp nóng, im lặng vì không có log), cho tới khi trạng thái đổi và lỗi chuyển thành `WRONG_STATE` (được coi là bỏ qua).

**Nguyên nhân gốc:** ngữ cảnh của consumer chỉ có tenant, không có người dùng. `GenerateSolution.start` (đường `system`) gọi `AIConnectionResolver.ResolveForProject`, mà bước dự phòng gọi `project.ListRepos` qua `withIdentityMetadata` đòi người dùng, nên trả `REQUEST_REPORTER_REQUIRED`. Lỗi này không nằm trong `skipWithoutConnection` nên bị coi là tạm thời. Cùng lớp lỗi với TASK-V2-018. Test đơn vị tái hiện đúng mã lỗi đó trước khi sửa.

**Sửa:**
- `generate_solution.go`: với đường hệ thống không có người gọi, hành động nhân danh người báo cáo (`tenant.WithUserID(ctx, req.ReporterID)`), cùng người mà run vốn đã được gán (`ActorID`), và giống `propose_request_classification.go` đã làm. Đường người dùng không đổi (vẫn từ chối khi thiếu người dùng).
- `clarification_resume_consumer.go`: ghi `WARN "clarification resume failed; will be redelivered"` khi `Handle` trả lỗi, để vòng giao lại vĩnh viễn không còn im lặng.

**Không phá luồng:** idempotent như cũ (`processed_events` cùng transaction, khoá `clr-<id>`); Request không còn ở `analyzing` thì vẫn là `ErrResumeSkipped`; `planning` không bao giờ tự chạy; `executing` vẫn gọi `ExecutionAdvancer` (chưa được nối ở `wire_artifact.go`, chỉ log, không đổi).

**Chỗ CR để người bấm (ghi cho frontend):** Request vào `analyzing` **không qua Clarification** (xác nhận loại mà kiểm tra sẵn sàng đạt) thì CR-REQ-007 mục 2.5 để người gọi `GenerateSolution` rõ ràng, backend không tự khởi động và mã cũng không (xác nhận loại không gọi sinh Solution). Do đó UI cần nút "Generate analysis" khi `analyzing` mà chưa có run; đó là việc của frontend, không đổi backend.

**Tự phục hồi sau redeploy:** các Request đang kẹt ở `analyzing` với sự kiện `information_provided` chưa ack sẽ được giao lại cho consumer mới và tự sinh Solution; các Request đã `planning`/`cancelled` bị bỏ qua và ack.

## Lỗi 4: agent dev-server rớt khi gateway panic (chỉ điều tra, không cần sửa mã)

Agent kết nối `ws://<gateway>/agent` (`MODE=direct-websocket`, `start-local-agent.sh`), nên tiến trình api-gateway chết thì WebSocket của agent đứt bất thường. `agent-connection-direct.ts` nối lại có lùi (1, 2, 5, 15, 30 giây), và token agent dùng một lần nên nó lấy token mới trước khi nối; trong lúc đó dev server `unreachable` và phân loại trả `REQUEST_AI_AGENT_UNAVAILABLE`. Mỗi lần panic tạo một khoảng rớt như vậy (chuỗi 15 lần khởi động lại làm backoff lên tới 30 giây). Hết panic thì hết nguyên nhân; hành vi nối lại bản thân là thiết kế. Không sửa. Nên cân nhắc về sau: đưa gateway khởi động lại mà không đứt agent chỉ có thể làm bằng cách tách kênh agent khỏi tiến trình gateway (ngoài phạm vi).

## Việc cần làm

- [x] Dừng consume khi `ctx` huỷ ở `common/eventbus` + test.
- [x] `pushFanInSink` và dùng cho ba kênh; test tái hiện đồng thời với `-race`.
- [x] Rà các kênh khác có mẫu tương tự (kết quả ở trên).
- [x] `task.list`: chuyển `taskTypes`, `requestId(s)`, `parentId`; `taskView.requestId`; CONTRACT mục 2.7; test.
- [x] Resume sau Clarification: hành động nhân danh người báo cáo + log lỗi; test.
- [x] Ghi quyết định mục 3.
- [ ] Redeploy và chạy lại UI e2e (người điều phối).
- [ ] Commit.

## Tiêu chí hoàn thành

- [x] Phát sự kiện đồng thời lúc huỷ subscription không panic, kênh luôn được đóng, chạy lặp `-count=20` với `-race` (test sau khi bỏ khoá trong sink thì FAIL với data race, đã thử).
- [x] `task.list {projectId, taskTypes:["plan","phase"], requestId}` tới `TaskService.ListTasks` đúng trường; vắng trường thì yêu cầu không đổi.
- [x] Consumer resume nhân danh người báo cáo khi tra cứu dev server (test dùng bộ phân giải đòi người dùng, thất bại với `REQUEST_REPORTER_REQUIRED` khi bỏ bản sửa).
- [ ] Trên stack thật: hết `panic` trong `docker logs orca-go-api-gateway`; Request sau khi trả lời Clarification có run Solution trong vài giây; tab Plan thấy cây Plan.

## Kiểm thử

Chạy với `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`:

- `cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./...`: PASS.
- `go test -race -count=20 -run 'CancelWhileEventsFlow|PushFanInSink' ./internal/adapter/wscompat/`: PASS (35 giây).
- `go test -race -skip TestBrowserScreencastChannel ./...` (api-gateway): PASS. Ba test `TestBrowserScreencastChannel_*` có **data race có sẵn** trong test double (`channels_browser_screencast_test.go`, tệp không đổi) khi chạy với `-race`; không liên quan.
- `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./...`: PASS.
- `cd backend-go/services/task-service && go build ./... && go vet ./... && go test ./...`: PASS (không đổi mã task-service; test lọc `TaskTypes`/`RequestIDs`/`ParentID` hai dialect đã có sẵn).
- `cd backend-go/common && go test ./eventbus/`: PASS.
- `go test -tags e2e ./e2e/...` (request-service) với `E2E_DIALECT=postgres` và `E2E_DIALECT=mysql`: PASS cả hai (32 giây và 57 giây).

## Kết quả thực thi

Xong về mã và test; **chưa chạy trên stack thật**.

**Blast radius đã rà tay (không có gitnexus/codegraph trong phiên):**
- `common/eventbus.consumeUntilDone`: gọi bởi `Consumer.Subscribe` và `Consumer.SubscribeEphemeral` (grep: không nơi nào khác). Mọi service dùng hai hàm này (api-gateway, notification, request, task, workflow, tenant, v.v.) nay có consumer dừng thật khi `ctx` huỷ lúc tắt hoặc huỷ subscription; trước đó consumer rò rỉ. Hợp đồng `Handler` không đổi. Điểm khác: tin đã nằm trong bộ đệm khi `ctx` huỷ bị NAK (giao lại sau, dành cho durable) thay vì chạy handler; `Subscribe` bền vững không mất tin vì chưa ack.
- `registerRequestStream`, `RegisterTaskActivityStreamChannel`, `registerAgentStatusSubscribeChannel`: đăng ký qua `registerRequestChannels`, `RegisterPushChannels` và test; chữ ký và kênh không đổi, chỉ cách ghi vào kênh đầu ra.
- `task.list`: chỉ frontend `useTasks`/`usePlanTree` gọi; trường mới tuỳ chọn. `taskView` thêm `requestId` (`omitempty`), dùng bởi `task.list`, `task.get`, `task.update`, `task.create*` và các kênh trả `toTaskView`; thêm khoá JSON, không bỏ khoá nào.
- `GenerateSolution.start`: chỉ đường `system` (`ExecuteForSystem`, `PrepareForSystem`, gọi bởi `SolutionRegenerator`) đổi; `Execute` của RPC không đổi.
- `ClarificationResumeConsumer.Handle`: chỉ thêm dòng log.

**Cần redeploy (người điều phối):** `api-gateway` (panic, `task.list`) và `request-service` (resume). Không migration, không đổi proto. `task-service` không đổi mã. Các service khác chỉ nhận thay đổi gián tiếp qua `common/eventbus` khi được build lại (không bắt buộc ngay, nhưng nên build lại để hết rò rỉ consumer).

**Chưa kiểm chứng / rủi ro còn lại:**
- Chưa chạy trên stack thật, nên chưa xác nhận hết panic và Solution tự sinh.
- Vòng giao lại nóng khi `Handle` lỗi vĩnh viễn là hành vi chung của `consumeUntilDone` (NAK không có độ trễ); chưa đổi vì ảnh hưởng mọi service. Nên cân nhắc `NakWithDelay` với backoff.
- `ExecutionAdvancer` của resume chưa được nối (chỉ log): Clarification trả lời khi `resume_status=executing` không tự chạy tiếp; ngoài phạm vi.
- Resume nhân danh người báo cáo giả định người báo cáo còn là thành viên project; nếu không, `ListRepos` vẫn từ chối và sự kiện giao lại (có log mới để thấy).
