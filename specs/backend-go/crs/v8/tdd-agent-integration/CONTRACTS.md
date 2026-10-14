# Hợp đồng đã đổi bởi SOL-TDD-002 (Giai đoạn 2, 2026-10-14)

Tài liệu này ghi **những gì mã đã làm** cho tích hợp tdd-agent như knowledge hub: tham số và kết quả `agent.execPrompt`, schema Solution, hợp đồng `tasks.md`, thực thi task, kênh/REST/CLI, cơ sở dữ liệu, cấu hình, metric và mã lỗi kèm gợi ý sửa. Hướng dẫn vận hành (cài đặt, bật cờ): [docs/guides/request/knowledge-hub-setup.md](../../../../../docs/guides/request/knowledge-hub-setup.md). Bằng chứng chạy thật: [evidence/](evidence/). Điều **chưa** làm nằm ở mục 11.

Nguyên tắc không đổi: Orca không gọi MCP hay đồ thị tri thức (KG). Agent mà Orca chạy trên dev server gọi, qua MCP `tdd-agent` và bốn skill. KG chỉ là tham chiếu, không bao giờ là cổng. Mặc định mọi cờ tắt; tắt thì hành vi như trước (T2.13).

## 1. `agent.execPrompt` (agent TypeScript, `agent/src/relay`)

Tham số mới (đều tuỳ chọn; agent cũ bỏ qua khoá lạ, nên Orca chỉ gửi khi capability cho phép):

| Tham số | Ý nghĩa |
|---|---|
| `knowledgeRoot` | Đường dẫn tuyệt đối của checkout có `.tdd-agent/config.toml`. Agent kiểm (realpath, thư mục, `lstat` để chặn symlink), tự dựng cấu hình MCP tạm (file 0600 trong `<workDir>/tmp/mcp-*`, xoá sau run) chạy cố định `tdd-agent -C <realpath> mcp serve` và truyền `--strict-mcp-config`, `--allowedTools` (bảy tool đọc `kg_*`/`source_get`, thêm `changeset_propose` chỉ khi được xin) và `--disallowedTools` (`run_start`, `run_resume`, `approval_submit`, `proposals_list`, và `changeset_propose` khi không xin). Không chuỗi nào do người gọi quyết định thành tham số tiến trình. Lỗi: `KNOWLEDGE_ROOT_NOT_ABSOLUTE`, `_MISSING`, `_NOT_DIRECTORY`, `KNOWLEDGE_CONFIG_MISSING`, `KNOWLEDGE_HUB_UNSUPPORTED` (InvalidParams). |
| `proposeChangesets` | Cho phép `changeset_propose`; đòi `knowledgeRoot`. Chưa dùng ở Orca (T3.1: chỉ đề xuất, người áp tay). |
| `permissionProfile` | Tên hồ sơ quyền; hiện chỉ `openspec-author` (`acceptEdits`, tool `Read,Glob,Grep,Write,Edit,Bash` và `Bash(openspec:*)`, `Bash(tdd-agent -C * status:*)`; không bao giờ toàn quyền). Agent chọn mọi cờ từ tên, không cờ nào đi theo yêu cầu. |
| `resultBlock{nonce}`, `accessMode`, `workspaceKind`, `reportChanges` | đã có từ CR-REQ-029/protocol 2; engine `openspec` dùng `write`/`worktree` + `permissionProfile`. |

Kết quả mới (chỉ chạy `stream-json`; chế độ văn bản không có): `toolCalls` (đếm theo tên tool, tối đa 64 tên, phần dư vào `(other)`; **chỉ tên**, không tham số hay kết quả) và `usage` (`turns`, `durationMs`, `costUsd`, `inputTokens`, `outputTokens`, `cacheReadTokens`, `cacheCreationTokens`; chỉ có những số `claude` báo). Capability: feature `agent.execPrompt.knowledgeHub`, `agent.execPrompt.permissionProfile`; công cụ `tdd-agent` trong `tools`; `skills[{name,installed,sha256}]` cho bốn skill.

## 2. Solution bằng agent chỉ đọc có KG (T2.9)

- Cờ project `solution_agent` (mặc định tắt). Bật mà dev server không tải được hub, không có repo path hay không đọc được capability: Solution đi `ai.complete` như cũ, ghi log lý do. Người gọi không thể ép chế độ agent khi cờ tắt (`REQUEST_SOLUTION_MODE_NOT_ALLOWED`).
- Prompt (`usecase/solution_prompt.go`, golden `usecase/testdata/solution_prompt/agent_solution_{knowledge,no_knowledge}.golden`; golden `solution`/`diagnosis`/`retry` của `ai.complete` không đổi): preamble chỉ đọc, dùng skill `tdd-knowledge`, xung đột Decision/Constraint vào `cons` của từng phương án, chỗ thiếu thành câu hỏi mở, **AC chỉ lấy từ `<request>`** (id trong KG không phải AC của Request này), giữ `approach` dưới 3500 ký tự.
- Schema: thành viên tuỳ chọn `design_references{available, snapshot, read[], gaps[], conflicts[]}` trong `SolutionOptions` (không đổi trường bắt buộc nào). Giới hạn: `snapshot` ≤120 ký tự; `read` ≤80 mục ≤200 ký tự; `gaps`, `conflicts` ≤30 mục ≤1500 ký tự; khử bí mật như mọi tài liệu. Worker `settleDesignReferences`: chạy không có KG thì buộc `available=false`, bỏ `read`/`conflicts`/`snapshot` (agent không tự nhận đã đọc); KG vắng không làm run thất bại. KG chỉ để người duyệt đọc, không chặn gì.

## 3. Plan do agent viết (engine `openspec`, T2.3 đến T2.8)

- `GeneratePlan` cho Request ghim `openspec` chạy `agent_proposal`: worktree riêng trên nhánh cục bộ `request/<số>-proposal` (không bao giờ đẩy), agent viết change OpenSpec dưới `openspec/changes/<id>/`, Orca kiểm phạm vi ghi (chỉ trong thư mục change), đọc `tasks.md`, kiểm cấu trúc rồi dựng Plan từ đó. Thử lại một lần có lý do lỗi.
- `tasks.md`: vùng `<!-- orca:begin plan request=<ref> schema=1 -->`…`<!-- orca:end plan -->`; `## PH-n`/`## TASKS`; id `T<p>.<q>` theo vị trí; mỗi task bốn dòng `Serves:`, `Code:`, `Verify:`, `Done when:`; `[depends=…]` tương đối trong phase, phụ thuộc phase trước thành `DependsOnPhaseIndices`. Mã vi phạm cú pháp `TASKSMD_*`; mã kiểm bundle `REQUEST_SPEC_{TASK_COUNT,TASK_INCOMPLETE,CODE_TARGET_INVALID,AC_NOT_COVERED,AC_UNKNOWN,AC_NO_TASK,VALIDATION_FAILED,SPECS_MISSING}` (`openspec validate --strict` chạy qua `agent.exec`, hạn 25 giây).
- Lưu: `openspec_changes` (bundle ≤1 MiB, tệp quá cỡ chỉ giữ kích thước và băm; điểm tri thức mà agent khai; `skill_hash`; `workspace_id`); digest cổng Plan gộp digest bundle (sửa gói sau khi mở cổng làm quyết định cũ mất hiệu lực). Người duyệt đọc bằng `orca plan bundle`.
- Thời gian: lượt agent ≤15 phút (`REQUEST_SPEC_TIMEOUT_MS`), cả run kèm thử lại `REQUEST_SPEC_TOTAL_TIMEOUT_MS` (mặc định 2×lượt + 2 phút); relay Go 16 phút; lease gia hạn 30 giây/90 giây. CLI/UI chờ 4 phút (`native`) hay 18 phút (`openspec`).

## 4. Thực thi task (T2.11)

Task của Request ghim `openspec` đi đường hợp đồng của task-service: `TaskServiceExecuteRequest` thêm `use_knowledge_hub` (gốc repo lấy từ kết nối của project, không nhận đường dẫn từ người gọi) và `require_passing_check` (khi mô tả có dòng `Verify:`). Khối kết quả có nonce, **chỉ các khoá** `schema_version, status, summary, files_changed, checks_run[{id,exit}], outputs, questions, notes, knowledge{available,snapshot,read,gaps,conflicts}, writeback{documents,proposal_id,proposal_ok,touched}`; khoá lạ ⇒ `RESULT_SCHEMA_INVALID`. `done` mà không có kiểm tra `exit=0` khi cần ⇒ lần thất bại `agent_defect VERIFY_NOT_PASSED`. `knowledge`/`writeback` được lưu (cắt gọn, khử bí mật khi hiển thị) và không quyết định gì. `ai_context` mỗi task mang nguyên văn AC nó phục vụ (≤2000 ký tự). Vẫn `trustPreset: full` (rủi ro B4).

## 5. Dọn dẹp (T2.12)

`ProposalJanitor`: huỷ ⇒ gỡ worktree + gói `abandoned`; hoàn thành ⇒ sau `REQUEST_PROPOSAL_RETENTION` (mặc định 7 ngày) hoặc khi `archived`; run lỗi ⇒ sau cùng thời hạn (gói giữ `preparing`); không đụng worktree của run đang chạy; chỉ gỡ worktree đúng nhánh của Request. Gỡ tay: `orca request cleanup`. Báo mồ côi: `tests/stages/e2e/orca-flows/report_orphan_worktrees.py`.

## 5b. Đồng bộ `tasks.md` và archive (T3.3)

`TasksMdSyncer` (quét 30 s, bắt đầu sau 45 s; `ReportTaskOutcome` chỉ báo "có task xong" để quét sớm, mất thông báo chỉ làm chậm). Mỗi task của Plan openspec mang nhãn `spec:t<p>.<q>` (đặt lúc dựng Plan; task-service chuyển thường). Task xong = có run `succeeded` hoặc người đã đặt `done`. Bộ đồng bộ **tick** dòng `- [ ] T<p>.<q>` trong vùng `orca:begin/end plan` của `openspec/changes/<id>/tasks.md` trên nhánh proposal, chỉ commit `tasks.md`, **không bao giờ bỏ tick**, không đụng `tasks_md_digest`/bundle (cổng duyệt Plan vẫn khớp). Vùng bị sửa tay (digest vùng khác `tasks_sync_digest`, hoặc khác vùng trong bundle khi chưa đồng bộ lần nào) ⇒ khôi phục vùng đã lưu rồi tick lại, tăng `request_engine_drift_total`; văn bản ngoài vùng giữ nguyên từng byte. Bỏ qua khi có run plan đang chạy.

Khi Request `completed` và gói còn `ready`: chạy `openspec archive <id> --yes` với `cwd` = worktree proposal (qua `agent.exec`), **kiểm vùng ghi** (chỉ `openspec/changes/**` và `openspec/specs/**`, vi phạm ⇒ huỷ sạch và coi là lỗi), commit, `MarkArchived(sha)`. Không push, không mở PR. Lỗi (kể cả worktree đã gỡ, Ensure lại) ⇒ thử lại sau 30 s, 2 phút, 10 phút, 30 phút, 2 giờ (`tasks_sync_attempts`, `tasks_sync_error`, `tasks_sync_next_retry_at`); lần thứ 5 ⇒ `tasks_sync_state=failed`, dừng thử, **Request vẫn `completed`**. Thay thế `OpenSpecArchiveConsumer` và `RegisterOpenSpecConsumers` (đã xoá): archive do vòng quét nhận ra Request `completed`, nên không cần khử trùng `processed_events` mà vẫn lặp lại an toàn.

## 6. Giao diện (gRPC, kênh WS, REST, CLI)

| RPC | Kênh | REST | CLI |
|---|---|---|---|
| `Get/SetProjectEngineSettings` | `request.engineSettings`, `request.engineSettingsSet` (chỉ admin) | `GET/PUT /v1/projects/{id}/request-engine?check=true` | `orca engine show [--check]`, `engine set` |
| `GetPlanProposal` (thêm `engine`, `bundle` kèm `status`, `sync_state`, `sync_error`, `sync_attempts`, `checks`, `knowledge`, `stats`, `elapsed_ms`) | `request.planProposal` | `GET /v1/requests/{id}/plan[/bundle]` | `orca plan proposal`, `plan bundle [--file]` |
| `GetPlanBundleFile` | `request.planBundleFile` | `GET /v1/requests/{id}/plan/bundle/file?path=` | `plan bundle --file` |
| `CleanupRequestProposal` | `request.cleanupProposal` | `POST /v1/requests/{id}/proposal/cleanup` | `orca request cleanup` |
| `ListSolutions` (run thêm `engine`, `stats`) | `solution.list` | `GET /v1/requests/{id}/solutions` | `solution list`, `request show` |
| (task-service) kết quả lượt chạy | `execution.get` (thêm `knowledge`, `writeback`) | `GET /v1/tasks/{id}/execution` | `orca plan result <task-id>` |

Kênh mới đều **ngoài** công cụ MCP (`excluded_channels.yaml`). `orca engine set` cần token có `role=admin`; token của `orca login` hiện chưa mang role, dùng phiên web (xem README orca-cli).

**Bàn giao (T3.4).** `orca request show` tính ở phía CLI, chỉ khi Request `completed`, từ `GET /v1/tasks/{plan}/subtree` (worktree của task), `GetPlanProposal.bundle` (nhánh, trạng thái archive/đồng bộ), `GET /v1/tasks/{plan}/writebacks` và `design_references.conflicts` của Solution `approved`; `--json` mang `handover{items[{kind,text,command}],nothingLeft}`. Không có hành động tự động; `command` chỉ là gợi ý. Sau archive `bundle.commit` là commit cuối trên nhánh proposal (của archive), `bundle.status=archived`; file của gói vẫn đọc được từ bản lưu.

## 7. Cơ sở dữ liệu (cả Postgres và MySQL)

`0094_openspec_change_provenance` (cột tri thức, bundle, `workspace_id` của `openspec_changes`), `0095_project_solution_agent` (`project_engine_settings.solution_agent`), `0096_analysis_run_stats` (`analysis_runs.run_stats`: tên tool và số lần, số liệu usage; xếp loại "không có nội dung"), `0097_openspec_change_sync_progress` (`openspec_changes.tasks_sync_attempts`). Nền: `0007_solution_engines`.

## 8. Cấu hình

`REQUEST_OPENSPEC_MIN_VERSION`, `REQUEST_SKILLS_REPOSITORY_URL`, `REQUEST_PROPOSAL_RETENTION`, `REQUEST_SPEC_TIMEOUT_MS`, `REQUEST_SPEC_TOTAL_TIMEOUT_MS`. Cờ theo project qua `engine set`, không qua biến môi trường.

## 9. Metric (Prometheus, request-service)

`request_engine_runs_total{engine,kind,result}`, `request_engine_preflight_failures_total{code}`, `request_knowledge_tool_calls_total{tool}` (tên ngoài danh sách đã biết gộp vào `other`), `request_spec_check_violations_total{code}`, `request_engine_drift_total`, `request_engine_sync_total{step,result}` (step `tasks|archive`, result `done|failed|gave_up`). Không nhãn nào mang tenant, Request hay nội dung.

## 10. Mã lỗi và gợi ý sửa

| Mã | Khi nào | Gợi ý |
|---|---|---|
| `REQUEST_ENGINE_NO_CONNECTION` / `_REPO_NOT_RESOLVED` | không có dev server hoặc repo không có đường dẫn trên dev server | nối dev server và thêm repo với đường dẫn mà dev server mở được |
| `REQUEST_ENGINE_OPENSPEC_MISSING` / `_TIMEOUT` / `_VERSION` | `openspec` thiếu, không trả lời, hay cũ hơn mức tối thiểu | cài hoặc cập nhật OpenSpec CLI trên dev server |
| `REQUEST_ENGINE_AGENT_UNKNOWN` / `_AGENT_TOO_OLD` | không đọc được khả năng của agent / agent không viết được change dưới hồ sơ quyền | kiểm kết nối; cập nhật agent Orca |
| `REQUEST_KNOWLEDGE_AGENT_UNKNOWN` / `_AGENT_TOO_OLD` | (chỉ cảnh báo) không xác nhận được hub / agent không tải được hub | như trên; engine và Solution vẫn chạy không có KG |
| `REQUEST_KNOWLEDGE_TDD_AGENT_MISSING` | (cảnh báo) `tdd-agent` không có trong PATH | cài `tdd-agent` |
| `REQUEST_KNOWLEDGE_SKILLS_MISSING` | (cảnh báo) thiếu skill | `npx skills add <repo> --skill tdd-knowledge tdd-knowledge-writeback tdd-openspec-author tdd-orca-task --global` |
| `REQUEST_KNOWLEDGE_NOT_INITIALIZED` | (cảnh báo) repo không có `.tdd-agent/config.toml` | thiết lập tdd-agent trong repo |
| `REQUEST_ENGINE_UNAVAILABLE` | server chưa có worker cho engine của project | hoàn tất cấu hình dịch vụ (git-gateway, project-service, infra-fleet) |
| `REQUEST_ENGINE_SETTINGS_INVALID` / `_VERSION_CONFLICT` | cài đặt sai / bản cũ | sửa giá trị; đọc lại `version` rồi ghi |
| `REQUEST_OPENSPEC_{AGENT_FAILED,TIMEOUT,INVALID_OUTPUT,OUT_OF_SCOPE_CHANGE}`, `REQUEST_SPEC_RUN_OUT_OF_SCOPE` | run viết change lỗi, quá giờ, sai định dạng hay ghi ngoài thư mục change | xem thông điệp của run; worktree được giữ để điều tra (`orca request cleanup` khi xong) |
| `REQUEST_PLAN_BUNDLE_{NOT_FOUND,PATH_INVALID,FILE_NOT_FOUND}` | đọc tệp của gói | dùng đường dẫn tương đối trong change |
| `REQUEST_PROPOSAL_RUN_ACTIVE` | gỡ tay khi run đang viết | đợi run xong |
| `REQUEST_SOLUTION_MODE_NOT_ALLOWED` | xin chế độ agent cho Solution khi cờ tắt | bật `solution_agent` (admin) |

## 11. Chưa làm

Giai đoạn 4 (e2e ba bước trên pilot, cài skill toàn cục, bật cờ cho pilot); web UI (nhãn engine, `design_references`, thống kê run: T5.4); ghi `skill_hash` vào run của Solution bằng agent (chỉ có ở change OpenSpec); token CLI mang `role`.
