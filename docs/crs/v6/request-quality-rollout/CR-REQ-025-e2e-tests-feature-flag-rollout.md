# CR-REQ-025 — Kiểm thử đầu cuối, feature flag `request_flow_enabled`, rollout và tài liệu

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-025 |
| **Tên** | Bộ e2e cho 11 loại Request trên hai DB, cờ `request_flow_enabled` theo tenant, kế hoạch rollout và rollback, tài liệu và runbook |
| **Loại** | Chất lượng / Vận hành |
| **Priority** | 🔴 P0 (gate trước khi bật cho người dùng) |
| **Effort** | Medium (6 đến 8 ngày, rải theo các CR khác; phần cờ làm sớm) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001 đến 024 (đủ cho phần được bật); phần cờ chỉ cần CR-REQ-001, 002 |
| **Mở khoá** | GA của luồng Request |
| **Tác động** | `backend-go/services/request-service` (`e2e/`, bảng và RPC cờ, migration), `backend-go/ci/` (script mới), `.github/workflows/`, `deploy/dev/docker-compose.yml`, `tests/request/` (mới), `docs/guides/request/` (mới), README các service |

---

## 1. Bối cảnh và vấn đề

1. Luồng Request chạm 7 service và 2 DB (Postgres, MySQL). Kiểm thử từng CR không bắt được lỗi ở đường nối: Request → Approval → Plan/Phase (Task) → thực thi → phản hồi ngược → Jira.
2. README v6 yêu cầu toàn bộ luồng sau cờ `request_flow_enabled`, mặc định tắt (mục 6), nhưng chưa nói cờ ở đâu, ai đọc, bật theo tenant thế nào. CR-REQ-001 chỉ khai báo biến `REQUEST_FLOW_ENABLED` và log.
3. Đăng ký service mới dễ sót: `deploy/dev/scripts/migrate.sh` từng quên `issuetracking` nên DB không được migrate trên mọi môi trường. Cần kiểm tự động.
4. Mẫu rollout đã có ở CR-MCP-015 (giai đoạn, tách e2e xác định khỏi e2e LLM thật, rollback bằng cờ). CR này dùng lại.

## 2. Giải pháp đề xuất

### 2.1 Ba tầng kiểm thử

| Tầng | Chạy ở đâu | Phụ thuộc | Chặn PR |
|---|---|---|---|
| T1 e2e trong tiến trình | `backend-go/services/request-service/e2e/` (mới), build tag `e2e`, testcontainers | AI giả xác định, `task-service` giả trong bộ nhớ, outbox thật, DB thật | Có, ma trận `postgres`, `mysql` |
| T2 e2e trên stack dev | `tests/request/` (mới, Python theo khung `tests/mcp/mcp_check_framework.py`), chạy qua `backend-go/ci/request-e2e/run-request-e2e.sh` (mới) | `deploy/dev/docker-compose.yml` dựng service thật; Jira giả (issue-tracking trỏ stub) | Không; chạy hằng đêm và `workflow_dispatch` (`.github/workflows/request-e2e.yml`, mới) |
| T3 LLM thật | cùng T2, `ORCA_REQUEST_E2E_REAL_AI=true` | provider AI thật, dogfood | Không, báo cáo; không đánh giá chất lượng quyết định của LLM |

Lý do tách: AI thật không ổn định, không đưa vào cổng chặn (cùng nguyên tắc CR-MCP-015). Chưa kiểm chứng `ai-provider-service` có chế độ giả sẵn; nếu không, T1 dùng fake ở cổng `ai.complete` của `request-service`, T2 cần stub provider (Q2).

### 2.2 Cờ `request_flow_enabled`

| Hạng mục | Quyết định |
|---|---|
| Hai tầng | Tổng: biến `REQUEST_FLOW_ENABLED` (CR-REQ-001, mặc định `false`). Theo tenant: bảng `request.tenant_settings(tenant_id, request_flow_enabled, updated_by, updated_at)` (mới, migration cho cả Postgres và MySQL, số thứ tự chốt khi triển khai) |
| Hiệu lực | `effective = REQUEST_FLOW_ENABLED && tenant_settings.request_flow_enabled`; không có dòng nghĩa là `false`; lỗi đọc nghĩa là `false` (fail closed) |
| Ai thi hành | Chỉ `request-service`, ở một interceptor gRPC (`internal/adapter/grpc/flow_gate.go`, mới) |
| RPC | `GetRequestFlowSettings`, `SetRequestFlowSettings` trên `RequestService` (mới; README v6 3.6 chưa có). `Set` chỉ cho `role=admin` và ghi audit `request.flow.set` (CR-REQ-024) |
| Ai đọc | Frontend qua kênh `request.flowStatus` (CR-REQ-016) để hiện hoặc ẩn màn hình Request; MCP qua tool `request_flowStatus` (CR-REQ-017); `issue-status-sync` chỉ qua `LookupRequestBySource` (chỉ trả `found` khi cờ bật, CR-REQ-024) |
| Lỗi | `REQUEST_FLOW_DISABLED` (`FailedPrecondition`) |
| Bật cho tenant | admin tenant dùng `request.flowSet`; ops có thể đặt `REQUEST_FLOW_ENABLED=true` ở `deploy/dev/docker-compose.yml` cho `request-service` (`${REQUEST_FLOW_ENABLED:-false}`, theo mẫu `MCP_ENABLED`) |

Hành vi khi cờ tắt (cờ có thể tắt lúc đang có Request dở dang):

| Nhóm RPC | Khi tắt |
|---|---|
| Đọc (`GetRequest`, `ListRequests`, `ListSolutions`, `ListApprovals`, `ListPendingForUser`, `ListBacklog`, `GetRequestFlowSettings`) | cho phép, để người dùng xem lịch sử |
| Thoát an toàn (`CancelRequest`, `ReturnToBacklog`, `Reject`, `Cancel` Approval) | cho phép |
| Đi tiếp luồng (`CreateRequest`, `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `GenerateSolution`, `ChooseSolutionOption`, `GeneratePlan`, `StartPhase`, `Approve`, `SpawnChildRequest`, `ReopenRequest`) | `REQUEST_FLOW_DISABLED` |
| Nội bộ (`ReportTaskOutcome`, consumer outbox, hết hạn Approval) | vẫn chạy, để việc đang thực thi không mất kết quả |

`task-service` không đọc cờ: `type` `plan` và `phase` là một phần schema (CR-REQ-011), frontend lọc theo `type`. Cờ không ẩn hay xoá dữ liệu.

### 2.3 Bộ kịch bản e2e

Mỗi kịch bản kiểm: chuỗi trạng thái Request, Approval sinh ra (`subject_type`), `Solution.kind`, Plan/Phase/Task tạo ra, sự kiện outbox, dòng audit (CR-REQ-024).

| ID | Nhóm luồng | Loại | Kịch bản |
|---|---|---|---|
| E01 | Đủ | `change_request` | tạo → phân loại → xác nhận → Solution 2 phương án → chọn → duyệt → Plan → duyệt → 2 Phase, duyệt từng Phase → task chạy xong → Request `completed` |
| E02 | Đủ | `bug` size L | Chẩn đoán → Fix plan → có Phase |
| E03 | Đủ | `refactor` size L | Solution → Plan → Phase |
| E04 | Rút gọn | `bug` size S | không có Phase |
| E05 | Rút gọn | `task` | `task_list` → duyệt → chạy |
| E06 | Rút gọn | `docs` | như E05 |
| E07 | Rút gọn | `performance` | Chẩn đoán (baseline) → Plan |
| E08 | Rút gọn | `security` | xác nhận mức độ bởi người, `pre_deploy` |
| E09 | Rút gọn | `ops_request` | Plan có runbook và rollback, `pre_deploy` trước bước không đảo ngược |
| E10 | Hotfix | `hotfix` | bắt buộc người xác nhận loại; Chẩn đoán nhanh không cổng; một task; `pre_deploy`; sinh Request theo dõi `followup_hotfix` |
| E11 | Spike, question | `spike` | Findings → duyệt → đóng; sinh Request con `spawned_by_spike`; không có Task |
| E12 | Spike, question | `question` | Answer → người dùng chấp nhận; không worktree, không Task |
| E13 | Chéo | bất kỳ | từ chối Plan → `request_backlog` (`returned_from_stage=plan`) → mở lại → phân loại lại |
| E14 | Chéo | `bug` | đổi loại sang `change_request` giữa chừng; Chẩn đoán, Task đã có giữ làm tham chiếu; lịch sử loại ghi đủ |
| E15 | Chéo | `task` | task lỗi → Request về backlog (`returned_from_stage=task`) kèm lý do |
| E16 | Chéo | `task` | callback `ReportTaskOutcome` giao lặp và đảo thứ tự không đổi kết quả |
| E17 | Chéo | MCP | `request_create` dedupe theo `client_request_id`; vượt hạn mức bị chặn (CR-REQ-017) |
| E18 | Chéo | Jira | Request nguồn Jira: `executing` → "In Progress", `completed` → "Done"; PR merge trong lúc Request chưa xong không chuyển "Done" (CR-REQ-024) |
| E19 | Chéo | quyền | người không được duyệt bị `APPROVAL_NOT_APPROVER`; audit có dòng `denied` |
| E20 | Chéo | cờ | tắt, bật, tắt giữa chừng: đúng bảng 2.2; tenant A bật không ảnh hưởng tenant B |

Test ma trận loại (`TestEveryRequestTypeHasScenario`): lấy 11 loại từ registry của CR-REQ-003 (`FlowFor`) và khẳng định mỗi loại xuất hiện ở ít nhất một kịch bản thành công; thêm loại thứ 12 mà thiếu kịch bản thì đỏ.

### 2.4 Hai DB

- T1 chạy `matrix.dialect: [postgres, mysql]` trong `.github/workflows/backend-go-request-service.yml` (CR-REQ-001 tạo, sao `backend-go-task-service.yml`); thêm bước `go test -tags=e2e ./e2e/...`.
- Cùng bộ kịch bản, cùng khẳng định; chỉ khác DSN. Kiểm riêng cho hai dialect: khoá lặp `request_idempotency`, `SELECT ... FOR UPDATE` của Approval, claim outbox `SKIP LOCKED` (MySQL ≥ 8.0.1, CR-DB-002), `JSON` so với `JSONB`, thứ tự sắp xếp phân trang.
- Migration `task-service` thêm `plan`, `phase` (CR-REQ-011) chạy trong workflow `backend-go-task-service.yml` hiện có, đã có ma trận hai dialect.
- `issue-status-sync` migration `0002` (CR-REQ-024) chạy trong `backend-go-issue-status-sync.yml`, đã có ma trận.

### 2.5 Kiểm tra đăng ký đầy đủ

`backend-go/ci/check-request-service-wiring.sh` (mới, mẫu `check-opa-bundle-in-images.sh`) thất bại nếu thiếu `request` ở: `backend-go/go.work`, `SERVICES` trong `backend-go/Makefile`, `DATABASES` trong `backend-go/deploy/postgres-init-databases.sh`, service `request-service` và `migrate-request` trong `deploy/dev/docker-compose.yml`, biến `REQUEST_SERVICE_ADDR` cho `api-gateway` và `issue-status-sync`, danh sách trong `deploy/dev/scripts/migrate.sh`. Chạy trong workflow `backend-go-request-service.yml`.

### 2.6 Rollout

| Giai đoạn | Phạm vi | Điều kiện vào | Điều kiện chuyển tiếp |
|---|---|---|---|
| 0. Nội bộ | `REQUEST_FLOW_ENABLED=true` ở dev; một tenant thử | T1 xanh hai dialect; script 2.5 xanh | Không có alert `RequestOutboxLag`, `RequestStuck` trong 3 ngày |
| 1. Dogfood | Tenant đội Orca, mọi loại | T2 xanh 5 đêm liên tiếp; Jira thật: một issue chạy hết E18 bằng tay | 2 tuần không sự cố mất dữ liệu; `orca_request_stuck` về 0 sau mỗi lần xử lý |
| 2. Beta | Danh sách tenant opt-in (admin bật `request.flowSet`); loại rủi ro thấp trước: `task`, `docs`, `question`, `spike` | Review quyền duyệt (CR-REQ-010); runbook được diễn tập | Tỷ lệ `orca_request_returned_total` không tăng bất thường; không có Jira bị chuyển sai |
| 3. Mở rộng loại | Thêm `bug`, `refactor`, `change_request`; sau cùng `security`, `performance`, `ops_request`, `hotfix` | Các loại này cần năng lực agent còn chưa kiểm chứng (README v6 mục 7) | Mỗi loại có ít nhất 5 Request thật hoàn tất |
| 4. GA | Cờ mặc định theo tenant có thể bật (không đổi mặc định `false` của biến tổng cho tới quyết định riêng) | Review bảo mật độc lập; kết quả ghi lại | |

Số ngày, số Request là đề xuất ban đầu, chưa có số đo thực để hiệu chỉnh. "Bật theo loại" ở giai đoạn 2 và 3 chưa có cơ chế trong CR nào: cờ này theo tenant, không theo loại (Q1).

### 2.7 Kế hoạch rollback

1. **Tắt cờ** tenant (`request.flowSet enabled=false`) hoặc biến tổng `REQUEST_FLOW_ENABLED=false` rồi khởi động lại `request-service`. Có hiệu lực ngay cho ghi mới theo bảng 2.2.
2. Request đang `executing`: cho chạy xong (callback vẫn xử lý) hoặc dùng `ReturnToBacklog` ở bước thoát an toàn.
3. Approval đang `pending`: `Reject` hoặc `Cancel` được; sau khi bật lại, người duyệt xử lý tiếp.
4. **Không chạy `down` migration** của `task-service` (`CHECK` loại `plan`, `phase`) khi đã có dòng loại đó; `down` của CR-REQ-011 phải từ chối chạy nếu còn dòng. Dữ liệu giữ nguyên.
5. Jira: trạng thái đã chuyển không tự lùi. Người dùng đổi tay nếu cần.
6. Kill switch riêng cho tool MCP: tắt `request_*` bằng chính sách tenant của `mcp-service` (CR-MCP-012) hoặc kill switch (CR-MCP-013).
7. Diễn tập rollback là điều kiện của giai đoạn 2; có test T1 cho thứ tự bật, tắt, bật (E20).

### 2.8 Tài liệu và runbook (`docs/guides/request/`, mới, tiếng Việt)

| File | Nội dung |
|---|---|
| `README.md` | tổng quan luồng, 11 loại, trạng thái, bảng ba view backlog |
| `creating-and-classifying-requests.md` | tạo từ Jira, GitHub, thủ công; xác nhận hoặc đổi loại |
| `approving-requests.md` | các cổng duyệt, hộp duyệt, ai được duyệt |
| `request-and-task-backlogs.md` | Request backlog, Task backlog, Execute backlog |
| `requests-from-agents-mcp.md` | tool `request_*`, hạn mức, vì sao agent không duyệt được |
| `admin-enable-request-flow.md` | cờ theo tenant, rollout, rollback |
| `runbook-request-flow.md` | xem bảng dưới |

Cập nhật: `backend-go/README.md` (hàng `request-service`: RPC nào thật, nào chưa), `backend-go/services/api-gateway/README.md` (kênh mới), `docs/guides/mcp/README.md` và `task-worktree-tools.md` (tool Request), `docs/guides/jira/jira-orca-mapping.md` (Request sở hữu issue, đồng bộ theo Request), và cột "Trạng thái" của mọi CR v6 khi triển khai.

Runbook (triệu chứng → kiểm tra → xử lý):

| Triệu chứng | Kiểm | Xử lý |
|---|---|---|
| Request kẹt `classifying` | `orca_request_stuck`, `orca_request_ai_generation_seconds`, log lỗi AI | kiểm `ai-provider-service`; `request.classify` chạy lại (tối đa 5 lần, CR-REQ-005) |
| Sự kiện không tới `issue-status-sync` | `orca_request_outbox_pending`, durable consumer, `orca_issuesync_request_events_total{result}` | kiểm NATS; `result=skipped_*` cho biết lý do |
| Jira không chuyển | `result=skipped_no_actor`, `skipped_transition_unavailable`, `skipped_request_owned` | kết nối Jira của người dùng; workflow Jira có tên trạng thái đúng không |
| Approval chất đống | `orca_request_approvals_pending` theo `subject_type` | báo người duyệt; hết hạn xử lý theo CR-REQ-010 |
| Phase không chạy | `orca_request_task_outcome_total`, lease của `task-service` | kiểm `execution_link` và recovery (CR-TG-008) |
| Khẩn cấp | | tắt cờ (mục 2.7) |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Cờ thi hành ở `request-service`, không ở gateway | Một điểm, WS, HTTP, MCP đều theo |
| D2 | Tắt cờ vẫn cho đọc và thoát an toàn, vẫn xử lý callback | Không mất kết quả việc đang chạy, không giam dữ liệu |
| D3 | Cờ fail closed | Thiếu dòng hay lỗi đọc không được vô tình bật |
| D4 | T1 chặn PR, T2 và T3 không chặn | AI thật và stack đầy đủ không ổn định |
| D5 | Kịch bản sinh từ registry loại | Thêm loại mà quên e2e thì CI đỏ |
| D6 | Rollback không dùng `down` migration | Tránh mất dữ liệu `plan`, `phase` |
| D7 | Cờ theo tenant, không theo loại | Đơn giản; bật dần theo loại cần quyết định riêng (Q1) |

## 4. Tiêu chí chấp nhận

- [ ] Bảng `tenant_settings` và hai RPC cờ có trên Postgres và MySQL; không có dòng thì `Get` trả `enabled=false`.
- [ ] Với `REQUEST_FLOW_ENABLED=false`, mọi RPC "đi tiếp luồng" trả `REQUEST_FLOW_DISABLED`; với biến bật và tenant chưa bật cũng vậy; bật cả hai thì chạy.
- [ ] Tắt cờ giữa chừng: đọc, `CancelRequest`, `ReturnToBacklog`, `Reject` còn hoạt động; `ReportTaskOutcome` vẫn ghi kết quả.
- [ ] `SetRequestFlowSettings` từ chối người không phải admin và ghi audit.
- [ ] E01 đến E20 chạy xanh trên Postgres và MySQL ở T1; `TestEveryRequestTypeHasScenario` xanh.
- [ ] Mỗi nhóm luồng (đủ, rút gọn, hotfix, spike hoặc question) có ít nhất một kịch bản xanh.
- [ ] `check-request-service-wiring.sh` xanh và nằm trong CI; cố ý xoá `request` khỏi `migrate.sh` thì script đỏ.
- [ ] Workflow `request-e2e.yml` chạy được qua `workflow_dispatch` (chưa kiểm chứng stack dev chạy trong CI).
- [ ] Tài liệu ở 2.8 có đủ; README các service không ghi "Real" cho RPC còn `Unimplemented`.
- [ ] Rollback được diễn tập một lần ở dev, kết quả ghi vào runbook.

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `e2e/request_flow_test.go` (mới) | E01 đến E16, E19; bảng kịch bản dựa trên `FlowFor` |
| `e2e/type_matrix_test.go` (mới) | `TestEveryRequestTypeHasScenario` |
| `e2e/feature_flag_test.go` (mới) | E20, bảng 2.2, cô lập tenant |
| `internal/adapter/grpc/flow_gate_test.go` (mới) | interceptor theo từng RPC |
| `tenant_settings` repo test hai dialect | đọc, ghi, mặc định |
| `tests/request/check_request_flow_types.py` (mới) | T2: vòng đời qua `/ws` cho 11 loại |
| `tests/mcp/check_mcp_request_flow.py` (mới, CR-REQ-017) | E17 |
| `tests/request/check_request_jira_sync.py` (mới) | E18 với Jira stub |
| Frontend (khi CR-REQ-018 đến 023 xong) | Playwright smoke của `tests/e2e/` cho màn hình Request, chỉ khi cờ bật |

Chưa chạy: toàn bộ. Số liệu thời gian, tỷ lệ là giả định.

## 6. Rủi ro và điểm chưa kiểm chứng

- T1 dùng `task-service` giả nên có thể che lỗi ở đường nối thật; T2 bù, nhưng T2 chưa chặn PR.
- Stack `deploy/dev` có chạy được trong runner CI hay không chưa kiểm chứng (nhiều service, Vault, NATS).
- Jira thật chưa từng chạy với luồng này (`task_sources` trên server dev có 0 dòng); E18 ở T2 dùng Jira giả.
- Các loại `performance`, `ops_request`, `spike`, `question` cần năng lực agent chưa có (README v6 mục 7); e2e T1 chỉ kiểm khung luồng, không kiểm chất lượng việc agent làm.
- Người duyệt đầu tiên của tenant: chính sách mặc định ở CR-REQ-009 (admin hoặc người báo); tenant một người dùng không bị kẹt.
- SSH và remote: e2e không giả định thực thi cục bộ; việc chạy agent đi qua `task.execute` nên cùng đường với SSH hiện có. Kiểm thử SSH thật nằm ngoài CR này.

## 7. Câu hỏi mở

1. Bật theo loại ở giai đoạn 2 và 3: cần cờ theo loại (ví dụ `request_flow_types`) hay chỉ hướng dẫn tenant chọn loại khi xác nhận? CR này chỉ làm cờ theo tenant.
2. `ai-provider-service` có provider giả cho T2 không? Nếu không, cần CR nhỏ cho stub.
3. README v6 cần thêm bảng `tenant_settings` và hai RPC cờ vào mục 3.5 và 3.6.
4. Mặc định cờ cho tenant mới ở GA: giữ `false` hay `true` như `MCP_TENANT_DEFAULT_ENABLED`? Cần quyết định sản phẩm.
5. Có đưa cờ vào `tenant-service` (settings phân lớp) để frontend đọc chung với các cài đặt khác thay vì kênh riêng không? CR này chọn bảng ở `request-service` để một chủ sở hữu.

## 8. Tham chiếu

- `backend-go/Makefile`, `backend-go/go.work`, `backend-go/deploy/postgres-init-databases.sh`, `backend-go/docker-compose.yml`
- `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/ensure-mcp-env.sh`
- `.github/workflows/backend-go-task-service.yml`, `backend-go-usage-service.yml`, `backend-go-issue-status-sync.yml`, `backend-go-mcp-conformance.yml`
- `backend-go/ci/check-opa-bundle-in-images.sh`, `backend-go/ci/mcp-conformance/run-go-conformance.sh`
- `tests/mcp/mcp_check_framework.py`, `tests/mcp/check_mcp_task_worktree_flow.py`, `tests/e2e/`
- `backend-go/services/mcp-service/internal/domain/tenant_settings.go`, `backend-go/services/api-gateway/internal/config/config_mcp.go` (mẫu cờ tổng và theo tenant)
- `docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md`
- `docs/crs/v6/README.md` mục 6 và 7; `docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md`
- `docs/guides/mcp/README.md`, `docs/guides/jira/jira-orca-mapping.md`
