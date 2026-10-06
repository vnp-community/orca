# BE-REQ-SOL-030: Đánh giá tác động (`ImpactAssessment`), `RiskAcceptance`, `RiskPolicy`, bộ thu thập, điểm bằng quy tắc, `GraphPayload` và chạy bóng

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Tài liệu ngày 2026-10-06. Toàn bộ nằm ở `request-service` (mới, chưa có thư mục: mọi đường dẫn là "(mới)"); không đổi `task-service`, `agent`. Có điểm cần các solution khác sửa (`ApprovalGuard` ở SOL-009, nhãn ở SOL-012): xem mục 4.

**CR:** [CR-REQ-030](../../../../../../docs/crs/v6/impact-risk/CR-REQ-030-impact-assessment-and-risk-scoring.md)
**Service:** `request-service` (mới) · `proto/orca/request/v1`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, port ở usecase), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB mỗi service, RLS, outbox, hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (dữ liệu không tin cậy, quyền), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (Relay, consumer bền), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (lease, phục hồi, chỉ số), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md), [`services/task-service.md`](../../../../tdd/services/task-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-06:
- `node .gitnexus/run.cjs impact --help`: tuỳ chọn `-d/--direction upstream|downstream`, `--depth <n>` (mặc định 3), `-l/--limit`, `--summary-only` ("counts and risk only"), `-f/--file`, `--kind`, `--include-tests`, `-r/--repo`, `--branch`; **không có `--json`**. `detect-changes --help`: `-s/--scope unstaged|staged|all|compare`, `-b/--base-ref`, `-l/--limit`. Định dạng đầu ra (văn bản) chưa kiểm chứng.
- `.gitnexus/gitnexus.json` có `repoPath`, `lastCommit` (`d8198127...`), `indexedAt`, `branch`, `stats{files,nodes,edges,processes}`; đọc bằng `fs.readFile`.
- `backend-go/Makefile` dòng 80 đến 81: `proto-lint: cd proto && buf lint && buf breaking --against '.git#branch=main' || true`. Do `A && B || true`, `make proto-lint` luôn thành công; CR chỉ có `buf lint --path orca/mcp` trong CI (`backend-go-mcp-service.yml`). `buf.yaml`: `lint: STANDARD`, `breaking: FILE`.
- `backend-go/ci/` chỉ có `check-mcp-protocol-version-lock.sh`, `check-nginx-*-routing.sh`, `check-opa-bundle-in-images.sh`, `mcp-conformance/`. `backend-go/policy/orca-authz/` có `admin.rego`, `annotation.rego`, `mcp*.rego`, `project.rego`, `repo.rego` (và `tenant.rego`, `task_grant.rego` theo CR; chưa liệt kê hết).
- `api-gateway/internal/adapter/mcpserver/tools/parity_test.go` và `excluded_channels.yaml` tồn tại; `wscompat/channels_*.go` có nhiều tệp (`channels_accounts.go`, `channels_admin_*.go`, ...). `config/scripts/check-max-lines-ratchet.mjs` tồn tại.
- `go.work` không có `services/request-service`; module thêm ở TASK-REQ-001-01.
- `agent.exec` nhận `{binary,args[],cwd,stdin,env,timeoutMs}` không qua shell, tối đa 5 phút (`agent-rpc-dispatch-agent-exec.ts`); `fs.readFile`, `fs.stat`. `git.exec` không cho `ls-files` (SOL-029 mục 1).
- Phụ thuộc cùng đợt (đã có solution): SOL-029 (`AgentRelay`, báo cáo sẵn sàng có `base_sha`, `ExecutionVerdict.FilesChanged`, sự kiện `execution.verified`), SOL-033 (`DevServerCapabilityReader`), SOL-007 (`analysis_runs` lease 90 giây/quét 30 giây, sự kiện `solution.proposed`), SOL-012 (sự kiện `orca.request.plan.generated {request_id, plan_task_id, phase_task_ids, task_ids, ...}`), SOL-009 (`Approval`, `SubjectHandler`, `UpdatePendingDigest`), SOL-013 (`AdvanceExecution`, `task_run_outcomes`).

### Correction relative to CR-REQ-030

1. **Plan, Phase, Task chưa có diff.** CR bảng 2.2 nói thời điểm Plan lấy đầu vào từ `scope.include/create` rồi chạy công cụ; nhưng `buf breaking`, `MigrationScanner` đọc nội dung, `gitnexus impact <symbol>` đều cần thay đổi thật hoặc danh sách symbol, mà lúc Plan chưa có. Quyết định: ở `plan|phase|task` chỉ các tín hiệu **dựa vào đường dẫn** (`basis=path`) và độ phủ `go test -cover` của package hiện có được đo; `Phạm vi ảnh hưởng` (impact theo symbol) và phần nội dung của `Hợp đồng`/`Dữ liệu` ghi `basis=declared|path`, tối đa `confidence=medium`. Công cụ thật (`basis=tool`) chạy ở `actual_*`. Mỗi `DimensionResult` có `basis` (CR không có trường này).
2. **Trigger có sẵn.** CR nói "sau `GeneratePlan` COMMIT"; sự kiện thật là `orca.request.plan.generated` (SOL-012). Dùng sự kiện này, không thêm sự kiện mới. `solution.proposed` (SOL-007) cho Option; `execution.verified` (SOL-029 task 08) cho `actual_task`; `phase.completed` (SOL-013) cho `actual_phase`.
3. **`ApprovalGuard` và `required_approvals` chưa có trong SOL-009.** `BE-REQ-SOL-009` có `SubjectHandler` và `UpdatePendingDigest` nhưng không có cổng ghép (`ApprovalGuard`), cũng không có `viewed_impact_digest`/`accepted_finding_ids` trong `DecideApprovalRequest`. Solution này định nghĩa giao diện cần thiết (mục 2.F) nhưng **không sửa** SOL-009; ghi thành phụ thuộc (mục 4) và câu hỏi mở Q1.
4. **Hai người duyệt cho mức Nghiêm trọng (CR Q1)** chưa chốt: v1 chỉ thi hành phần "Plan chia Phase, mỗi Phase ≤ Cao, task `check:rollback_rehearsal`"; cờ `REQUEST_RISK_REQUIRE_TWO_APPROVERS` mặc định `false` giữ chỗ.
5. **Hạn chế `--summary-only` của GitNexus.** CR gọi `impact <symbol> --direction upstream`; dùng thêm `--summary-only` cho vòng đếm (giảm đầu ra), và chỉ bỏ cờ này khi cần danh sách người gọi cho bằng chứng. Định dạng vẫn chưa kiểm chứng; `GitNexusOutputParser` có golden từ mẫu thật (task 04).
6. **`make proto-lint` không dùng.** Đúng như CR: chạy `buf breaking` trực tiếp. Sửa Makefile và thêm job CI là đề nghị riêng ngoài series (không làm ở đây).

## 2. Giải pháp

### 2.A Cây file

```
proto/orca/request/v1/request.proto                            (sửa)  RPC mục 2.H
request-service/
  migrations/{postgres,mysql}/NNNN_impact_risk.{up,down}.sql   (mới)  5 bảng
  internal/domain/
    impact_assessment.go      ImpactAssessment, DimensionResult, Finding, Trigger, enum
    risk_acceptance.go, risk_policy.go, risk_outcome.go
    risk_rules_v1.go          RulePolicy mặc định "rp/1" (hằng, không đọc môi trường)
    risk_scoring.go           Score(dims, policy) -> Scored; luật cứng; Digest
    impact_graph.go           GraphPayload, GraphNode, GraphEdge, dựng 4 lens
    area_resolver.go          AreaResolver (khu vực -> đường dẫn)
    impact_drift.go           CompareActual(expected, actual, delta) -> DriftResult
  internal/usecase/
    request_impact_assessment.go, collect_impact.go, collect_impact_steps.go,
    assess_actual_impact.go, risk_gate.go, accept_risk.go, override_risk_gate.go,
    get_impact_assessment.go, get_impact_graph.go, compare_impact.go, plan_risk_heatmap.go,
    list_impact_findings.go, impact_narrator.go, risk_policy_admin.go, record_risk_outcome.go,
    recover_impact_assessments.go, impact_ports.go
  internal/adapter/
    migrationscan/ contractscan/ gitnexusparse/                (mới)  bộ phân tích thuần Go, có fixture
    eventbus/impact_trigger_consumer.go                        (mới)  consumer bền
    postgres|mysql/impact_repository.go, risk_acceptance_repository.go,
                   risk_policy_repository.go, risk_outcome_repository.go
    grpcclient/impact_tool_runner.go                           (mới)  chạy công cụ qua AgentRelay (SOL-029)
```

### 2.B Dữ liệu (mọi bảng có `tenant_id`, hai dialect)

Cột đúng như CR 2.1; ghi lại các điểm cần chốt khi viết migration:
- `impact_assessments` (append-only theo `revision`): `subject_type` CHECK sáu giá trị; `mode` CHECK `shadow|enforce`; `status` CHECK `collecting|ready|partial|failed|superseded`; `level` CHECK nullable; `score SMALLINT` 0..100 (CHECK); `dimensions`, `triggers`, `findings` JSONB/JSON; `confidence` CHECK; `digest CHAR(64)`; `narrative` JSON NULL; lease: `lease_owner`, `lease_expires_at`. UNIQUE `(tenant_id, subject_type, subject_id, revision)`. "Một `collecting` mỗi chủ thể": Postgres partial unique index `WHERE status='collecting'`; MySQL cột sinh `active_key = IF(status='collecting', CONCAT(subject_type,':',subject_id), NULL)` + `UNIQUE (tenant_id, active_key)` (cùng thủ thuật `project_key` của `task_sources` `0012`).
- `impact_tool_runs`: `tool` CHECK chín giá trị, `status` CHECK `ok|timeout|error|skipped`, `output TEXT` cắt 256 KB (MySQL `MEDIUMTEXT`), `output_digest`. FK nội bộ tới `impact_assessments` `ON DELETE CASCADE`.
- `risk_acceptances`: UNIQUE `(tenant_id, assessment_id, finding_id, accepted_by)`; `rationale` CHECK độ dài ≥ 10 (kiểm ở domain, không dựa CHECK vì độ dài ký tự khác dialect).
- `risk_policies`: `status` CHECK `draft|shadow|active|retired`; tối đa một `active` và một `shadow` mỗi tenant (Postgres partial unique `WHERE status IN ('active')` và `('shadow')` hai chỉ mục; MySQL cột sinh `active_key`/`shadow_key`).
- `risk_outcomes`: PK `(tenant_id, request_id)`; `predicted_level`, `actual_level` nullable, `incident`, `rolled_back` BOOL, `override_count INT`, `recorded_at`.
- Không FK sang `task-service`; chỉ FK nội bộ tới `requests(id)` và giữa các bảng của feature.

### 2.C Domain: điểm, chiều, luật cứng

```go
type Dimension string // arch|contract|data|blast|security|ops|quality|uncertainty|scale
type DimensionResult struct {
    Dimension Dimension; Score int; Measured bool
    Basis     string // "tool" | "path" | "declared"
    Signals   []Signal // {Code, Value float64, Points int, Measured bool}
}
type RulePolicy struct { // mọi hằng ở đây; đổi hằng thì tăng RulesVersion
    RulesVersion string; Weights map[Dimension]float64; MaxWeight float64 // 0.6
    Thresholds map[Dimension][]Step; LevelCuts [3]int // 25, 50, 75
    ServiceCountHardRule int // 3
}
type Scored struct{ Score int; Level Level; Triggers []HardRule; Confidence Confidence; Digest string }
func Score(dims [9]DimensionResult, p RulePolicy) Scored
func DefaultRulePolicyV1() RulePolicy // "rp/1"
```
`Score`: `base = round(0.6*max(s_d) + 0.4*Σ(w_d*s_d)/Σ(w_d))`, chiều `Measured=false` mang điểm 50; mức theo cắt 25/50/75; luật cứng của CR 2.4.1 nâng mức tối thiểu (buf breaking ≥ 1 hoặc kênh/tool xoá-đổi tên thì Cao; migration không đảo ngược thì Cao; chạm xác thực/tenant/credential/OPA thì Cao; không đường quay lui hoặc `irreversible` không sau cờ thì Cao; chạm hơn `ServiceCountHardRule` service thì Cao; `max-lines` disable mới thì Cao; index lỗi thời hoặc không có kết quả thì tăng một bậc kèm `confidence=low`; ≥ 3 chiều không đo được thì Trung bình kèm `confidence=low`). `Digest` = SHA-256 JSON chuẩn tắc của `dimensions`, `triggers`, `findings`, `level`, `rules_version`; **không** gồm `narrative`. Hàm thuần: không đồng hồ, môi trường, AI. `confidence` thêm quy tắc: bất kỳ chiều `basis != tool` thì tối đa `medium`; `solution_option` luôn tối đa `medium` (CR 2.5).

### 2.D Bộ thu thập `ImpactCollector` (qua `AgentRelay` của SOL-029)

`RequestImpactAssessment.Execute(ctx, in{SubjectType, SubjectID, RequestID, Force bool})`: kiểm cờ `REQUEST_IMPACT_ENABLED`; nếu có bản `ready` còn hợp lệ (`subject_digest` trùng) thì trả; nếu có `collecting` thì trả nó; ngược lại chèn `impact_assessments(collecting, lease_owner, lease_expires_at = now_db + 90s)` trong một giao dịch (mẫu `analysis_runs`). `CollectImpact.Run(ctx, assessmentID)` (worker gia hạn lease 30 giây/lần, `RecoverInterruptedImpactAssessments` quét 30 giây, `FOR UPDATE SKIP LOCKED`, MySQL ≥ 8.0.1) chạy các bước, mỗi bước một `impact_tool_runs`; công cụ lỗi thì `status != ok`, bản đánh giá `partial`, chiều liên quan `Measured=false`; tổng ≤ `REQUEST_IMPACT_BUDGET` (10 phút); mỗi lệnh ≤ 300 giây (`agent.exec`).

| Bước | Lệnh (`AgentRelay.RunCommand`, `Binary`+`Args` tách) | Thời điểm | Chiều |
|---|---|---|---|
| Năng lực | `DevServerCapabilityReader.Get` (SOL-033); `ReadFile .gitnexus/gitnexus.json` | mọi | tất cả |
| Tuổi index | `git rev-list --count <lastCommit>..<base_sha>`; vượt `REQUEST_IMPACT_INDEX_MAX_COMMITS_BEHIND` (30) hoặc `lastCommit` không tới được thì `index stale` | `actual_*`, `solution_option`(nếu cần) | Bất định, `confidence` |
| GitNexus | `node .gitnexus/run.cjs impact <symbol> --direction upstream --depth 3 --summary-only` (tối đa `REQUEST_IMPACT_MAX_SYMBOLS` = 20) và `detect-changes --scope compare --base-ref <base_sha>` | `actual_*` | Phạm vi ảnh hưởng, Kiến trúc |
| `buf breaking` | `buf breaking --against '<repo>/.git#ref=<base_sha>,subdir=backend-go/proto' --error-format=json` với `Cwd=backend-go/proto`, **không** `|| true` | `actual_*` | Hợp đồng |
| Quét migration | `git diff --name-only -z <base_sha>` (qua `agent.exec binary=git`) rồi `ReadFile` từng `**/migrations/{postgres,mysql}/*.sql`; `migrationscan.Scan` (thuần Go) | `actual_*`; ở `plan` chỉ theo tên tệp trong `create` | Dữ liệu |
| Quét hợp đồng/chính sách | cùng diff; `contractscan.Scan` đối chiếu `wscompat/channels_*.go`, `tools/excluded_channels.yaml`, payload outbox, vùng nhạy cảm, `policy/**/*.rego` | `actual_*`; ở `plan` chỉ khớp đường dẫn | Hợp đồng, Bảo mật |
| Độ phủ | `go test -cover ./<pkg>/...` (tối đa 5 package có file đổi hoặc nằm trong `scope`) | `plan`, `actual_*` | Chất lượng |
| Check nền | `task_readiness_reports.baseline` (SOL-029), không chạy lại | `actual_*` | Chất lượng |
| `max-lines` mới | `git diff <base_sha> -- config/max-lines-baseline.txt` + tìm `max-lines` disable trong diff (`contractscan`) | `actual_*` | Chất lượng |

Dùng lại kết quả theo `(base_sha, path_set_digest, tool, command_digest)` (bộ nhớ đệm đọc `impact_tool_runs` gần nhất trong 30 phút) để Plan nhiều task không chạy lặp (CR rủi ro "tăng tải"). `AreaResolver` ánh xạ `affected_areas[].name` thành đường dẫn: `service` thì `backend-go/services/<tên>`, `module` thì `backend-go/<tên>`, `api` thì `backend-go/proto/orca/<tên>`; không ánh xạ được thì `finding AREA_UNRESOLVED` và Bất định tăng.

### 2.E Ba thời điểm và kích hoạt

Consumer bền `request-service-impact` (stream `REQUEST`, khử trùng bằng `processed_events(tenant_id, event_id)`, mẫu SOL-013 mục 2.5): `orca.request.solution.proposed` thì một đánh giá `solution_option` cho mỗi Option; `orca.request.plan.generated` thì `plan` rồi mỗi `phase`, mỗi `task` (một lần thu thập chung theo đường dẫn, nhiều bản đánh giá dẫn xuất); `orca.request.execution.verified` (`status=passed`) thì `actual_task` (`base_sha`..`HEAD` của task, từ `ExecutionVerdict`); `orca.request.phase.completed` thì `actual_phase`. Bản của Phase lấy chiều cao nhất của các task con cộng chiều Quy mô trên cả Phase; Plan tương tự trên các Phase (không trung bình đơn giản).

### 2.F Cổng rủi ro, chấp nhận, ghi đè

`RiskGate` là cổng ghép vào `Approve` (SOL-009) và điều kiện của `AdvanceExecution` (SOL-013). Giao diện cần SOL-009 cung cấp:

```go
// SOL-009 cần thêm (đề xuất): ApprovalGuard được gọi trong DecideApproval.Approve, sau CanDecide, trước handler.OnApproved
type ApprovalGuard interface {
    Check(ctx context.Context, in GuardInput) error // GuardInput{Approval, Decider, ViewedImpactDigest string, AcceptedFindingIDs []string}
}
```
`RiskGate.Check`: chỉ khi `mode=enforce` mới chặn; đang `collecting` thì `REQUEST_RISK_ASSESSMENT_PENDING`; `level` Trung bình thì cần `ViewedImpactDigest == assessment.digest`; Cao thì người duyệt thuộc `team:<RiskPolicy.gate_mapping.risk_approver_team>` (`REQUEST_RISK_APPROVER_NOT_ALLOWED`), mỗi phát hiện từ Cao trở lên có `RiskAcceptance` hợp lệ (`assessment_digest` trùng bản hiện hành; `REQUEST_RISK_ACCEPTANCE_REQUIRED`), Plan có nhãn `gate:feature_flag` và task `rollback`, cổng `pre_deploy` (SOL-014) cho mọi loại; Nghiêm trọng thì thêm `REQUEST_PLAN_RISK_TOO_HIGH` ở `PlanPreconditions` khi một Phase vượt Cao, và task `check:rollback_rehearsal` hoàn tất trước Phase đầu. Digest chủ thể `solution` (Option đã chọn) và `plan` gồm `assessment.digest`; đánh giá xong sau khi Approval đã mở thì gọi `UpdatePendingDigest`. `AcceptRisk.Execute(ctx, in{AssessmentID, FindingID, Rationale, AssessmentDigest})`: `rationale` ≥ 10 ký tự, `assessment_digest` phải trùng bản hiện hành (`REQUEST_RISK_ASSESSMENT_STALE`), ghi `risk_acceptances` và outbox `orca.request.risk.accepted {assessment_id, finding_id, accepted_by}` (không có `rationale`). `OverrideRiskGate.Execute(ctx, in{RequestID, Gate, Reason})`: `team:<risk_override_team>` hoặc admin, `reason` ≥ 20 ký tự (`REQUEST_RISK_OVERRIDE_REASON_REQUIRED`), ghi audit (`AppendDetailed`, CR-REQ-024, `target_type=assessment`), tăng `risk_outcomes.override_count`; không bỏ được luật "mỗi phát hiện Cao cần xác nhận" trừ khi override nêu `finding_id`.

### 2.G Lệch kế hoạch (drift) và hiệu chỉnh

`AssessActual` tạo `actual_task` sau `VerifyExecution` đạt và `actual_phase` khi Phase `done`; `domain.CompareActual(expected, actual, delta)` trả `drift` khi: tập service thực tế khác dự kiến; `level` thực tế cao hơn một bậc trở lên; `score` chênh > `REQUEST_RISK_DRIFT_DELTA` (15); hoặc tín hiệu tuyệt đối mới (`buf breaking` ≥ 1 mà dự kiến 0, migration không đảo ngược). `enforce`: outbox `orca.request.impact.drift_detected`, mở Approval `subject_type=phase`, `stage=drift_review` (SOL-009 giữ nguyên bảng `subject_type`); `RiskGate` chặn `AdvanceExecution` cho đến khi có quyết định; `OnApproved` ghi bản đánh giá mới làm baseline; `OnRejected` thì `ReturnToBacklog(stage=phase, category=rejected)`. Task đang chạy vẫn chạy tới cùng (không có RPC dừng run). `shadow`: chỉ ghi và hiển thị. `RecordRiskOutcome` điền `risk_outcomes` khi Request `completed` hoặc về backlog (`incident`, `rolled_back` do người khai tay ở v1, CR Q4).

### 2.H API, đồ thị, sự kiện, lỗi, cấu hình

RPC (`RequestService`; kênh WS do CR-REQ-016/036 chốt; mỗi kênh cần `ToolSpec` hoặc dòng trong `excluded_channels.yaml` để `parity_test.go` xanh, README v6 mục 8 điều 13): `RequestImpactAssessment`, `GetImpactAssessment`, `GetImpactGraph{request_id, subject_type, subject_id, lens, base?, max_nodes}`, `CompareImpact{solution_id}`, `ListImpactFindings`, `GetImpactEvidence`, `GetPlanRiskHeatmap`, `AcceptRisk`, `GetImpactDrift`, `OverrideRiskGate`, `GetRiskPolicy`, `SetRiskPolicy` (kênh `impact.request|get|graph|compare|findings|evidence|heatmap|accept|drift`, `risk.override`, `risk.policy.get|set`). Mọi RPC tự kiểm quyền (đọc theo quyền đọc Request; `risk.policy.set` chỉ admin tenant). `GraphPayload` theo CR-REQ-032: node `{id, kind, label, group, risk, status}`, cạnh `{from, to, kind, change}`, `totalNodes`, `truncated`, `assessedAt`, `tool`, `stale`; `risk ∈ low|medium|high|critical|unknown` với `unknown` cho chiều không đo được hoặc index lỗi thời (không bao giờ `low`); `id` ổn định `<kind>:<tên>`; cắt ở `max_nodes` (mặc định 50). Backend dựng bốn lens `architecture|contract|data|impact`; `plan`, `execution`, `flow` do client dựng.

Sự kiện: `orca.request.impact.assessed {assessment_id, request_id, subject_type, subject_id, level, score, confidence, mode}`, `orca.request.impact.drift_detected`, `orca.request.risk.accepted`, `orca.request.risk_policy.changed`. Lỗi (`FailedPrecondition` trừ khi ghi khác): `REQUEST_RISK_ASSESSMENT_PENDING`, `_ACCEPTANCE_REQUIRED`, `_ASSESSMENT_STALE`, `_APPROVER_NOT_ALLOWED`, `REQUEST_PLAN_RISK_TOO_HIGH`, `REQUEST_RISK_OVERRIDE_REASON_REQUIRED`, `REQUEST_RISK_POLICY_INVALID` (`InvalidArgument`), `REQUEST_IMPACT_NO_CONNECTION`. Cấu hình (đề xuất, chưa đo): `REQUEST_IMPACT_ENABLED=false`, `REQUEST_IMPACT_BUDGET=10m`, `REQUEST_IMPACT_INDEX_MAX_COMMITS_BEHIND=30`, `REQUEST_RISK_DRIFT_DELTA=15`, `REQUEST_IMPACT_MAX_SYMBOLS=20`, `REQUEST_RISK_REQUIRE_TWO_APPROVERS=false`.

### 2.I `ImpactNarrator` (AI chỉ diễn giải)

Sau khi điểm chốt, gọi AI qua cổng của CR-REQ-034 (`PromptRegistry` bước `impact_narrative`; nếu CR-034 chưa có thì qua `AICompleter` của SOL-007) với **chỉ** JSON của bản đánh giá trong khối rào (ranh giới suy ra từ nội dung như SOL-029 mục 2.C; `path`, `message` có thể mang nội dung từ repo). Đầu ra bắt buộc `{summary, top_reasons[3], mitigations[]}`; bị loại nếu có trường `score`/`level` hay lệch tên chiều. Ghi `narrative` (`generated_by=ai`), ngoài `digest`. Lỗi AI không làm bản đánh giá `failed`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Điểm bằng quy tắc có phiên bản, `narrative` ngoài `digest` | Tái lập, kiểm toán; đổi lời diễn giải không vô hiệu Approval |
| `basis` trên từng chiều (`tool|path|declared`) | Plan chưa có diff nên không đo bằng công cụ; không giả vờ độ tin cậy cao |
| Dùng consumer bền, không gọi trực tiếp từ use case khác | Tách khỏi đường chặn của `Approve`/`VerifyExecution`; at-least-once + `processed_events` |
| Chiều không đo được tính 50 và `risk=unknown` ở đồ thị | "Không đo được" khác "thấp" |
| `buf breaking` chạy trực tiếp, không `|| true` | `make proto-lint` nuốt mọi lỗi |
| Không tự `gitnexus analyze` | Chưa đo chi phí trên repo 247 nghìn symbol; chỉ báo lỗi thời |
| `RiskAcceptance` gắn `assessment_digest` | Đánh giá đổi thì chấp nhận cũ mất hiệu lực |
| Drift dùng Approval `phase` với `stage=drift_review` | Không thêm `subject_type` vào CHECK của SOL-009 |
| Shadow mặc định | Ngưỡng là ước lượng; chặn sai làm mất niềm tin |

## 4. Phụ thuộc và thứ tự

Cần SOL-029 task 06 (`AgentRelay`) và 08 (`execution.verified`), SOL-007 (`solution.proposed`, mẫu lease), SOL-012 (`plan.generated`), SOL-013 (`phase.completed`, `AdvanceExecution`), SOL-009 (`SubjectHandler`; **cần bổ sung** `ApprovalGuard`, `viewed_impact_digest`, `accepted_finding_ids`, `stage=drift_review`), SOL-010 (`team:<id>` người duyệt), SOL-014 (`pre_deploy`), SOL-012 (nhãn `gate:feature_flag`, `check:rollback_rehearsal`, `PlanPreconditions`), CR-REQ-024 (`AppendDetailed`), CR-REQ-032 (`GraphPayload`), CR-REQ-034, 035. Thứ tự: 01, 02, 04 song song; 03 sau 01; 05 sau 02, 03, 04; 06 sau 05 và SOL-009; 07 sau 05; 08 cuối. Giai đoạn 0 (thử ngoại tuyến) của CR chạy được ngay sau 02 và 04, trước khi viết 05 (tiêu chí qua: khớp mức hoặc lệch tối đa một bậc ở ≥ 4/5 ca).

## 5. Kiểm thử

- **Unit:** `Score` (bảng, thuộc tính 1000 lần, golden `rp/1` ≥ 6 ca: từng luật cứng, chiều không đo được, index lỗi thời); `AreaResolver`; `MigrationScanner`, `ContractScanner`, `GitNexusOutputParser` (mẫu thật làm fixture); `CompareActual`; dựng đồ thị từng lens; kiểm narrative.
- **Integration hai dialect (`-tags=integration`):** `impact_assessments` (append-only, lease, một `collecting`), `risk_acceptances` (hiệu lực theo digest, UNIQUE), `risk_policies` (một `active`, một `shadow`); thu thập với fake `AgentRelay` (thành công, timeout, lỗi parse); `RiskGate` ghép với `Approve`.
- **Hợp đồng:** `buf lint`/`buf breaking` chạy trực tiếp; JSON Schema đồ thị dùng chung với frontend; `parity_test.go` xanh sau khi thêm kênh.
- **Thủ công, có dev server (chưa kiểm chứng):** chạy `impact`, `detect-changes`, `buf breaking` thật trong worktree `task/<id>` và qua SSH; đo thời gian trên repo 247 nghìn symbol; giai đoạn 0.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/...` và bản `-tags=integration`.

## 6. Rủi ro và điểm chưa kiểm chứng

- GitNexus và CodeGraph có thể không thấy tác động xuyên ranh giới gRPC, kênh WS, sự kiện outbox; `ContractScanner` chỉ bù một phần bằng regex.
- Định dạng đầu ra CLI GitNexus chưa kiểm chứng, không có `--json`; parser dễ gãy khi đổi phiên bản; phương án thay là MCP của GitNexus (CR-REQ-031).
- Index nằm ở thư mục gốc repo; worktree `task/<id>` có dùng chung index hay không chưa kiểm chứng; `detect-changes --base-ref` ánh xạ hunk sang symbol của index cũ nên có thể bỏ sót symbol mới.
- 20 symbol × `impact` có thể vượt ngân sách 10 phút; chưa đo.
- `buf breaking --against '<.git>#ref=<sha>'` cần `ref` có trong clone của dev server; clone nông làm hỏng; chưa thử qua SSH.
- Ngưỡng, trọng số, `N=3` service, độ lệch 15 điểm đều là ước lượng: bắt buộc `shadow` trước `enforce`.
- Tăng tải: Plan 100 task có thể tạo hàng trăm lần chạy công cụ nếu bộ nhớ đệm theo `(base_sha, path_set)` không hiệu quả.
- Phụ thuộc chặt vào việc SOL-009 bổ sung `ApprovalGuard`; nếu không, chặn theo mức rủi ro không thi hành được (chỉ hiển thị).

## 7. Câu hỏi mở

- **Q1.** SOL-009 mở rộng `ApprovalGuard`, `viewed_impact_digest`, `required_approvals` (hay hai Approval nối tiếp cùng chủ thể, cần nới chỉ mục "một `pending` mỗi chủ thể")?
- **Q2.** `risk_approver_team` và `risk_override_team` lấy từ `RiskPolicy.gate_mapping`; xác nhận với SOL-010.
- **Q3.** Nhãn `gate:feature_flag` và `check:rollback_rehearsal` vào `plan_labels.go` (SOL-012); Orca không có bước deploy nên "thử quay lui" chỉ là task agent.
- **Q4.** Ai khai `incident`/`rolled_back` cho `risk_outcomes` ở v1 (tay)? Có nguồn tự động từ CR-REQ-024 không?
- **Q5.** Đo `Phạm vi ảnh hưởng` ở thời điểm Plan: có chấp nhận `basis=declared` (chỉ từ `affected_areas` và số service) cho tới khi có danh sách symbol từ TaskSpec?
- **Q6.** Chạy `go test -cover` ở thời điểm Plan trên checkout nào (`repo_path` thường có thay đổi chưa commit)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.7, 6, 8 (điều 13, 14); `docs/research/receive-request/impact-assessment-and-risk-scoring.md`; `docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md`
- `/opt/repos/orca/.gitnexus/gitnexus.json`; `node .gitnexus/run.cjs impact --help`, `detect-changes --help`
- `/opt/repos/orca/backend-go/Makefile` (dòng 80 đến 81), `proto/buf.yaml`, `backend-go/ci/`, `backend-go/policy/orca-authz/`, `config/scripts/check-max-lines-ratchet.mjs`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/mcpserver/tools/parity_test.go`, `excluded_channels.yaml`; `wscompat/channels_*.go`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql` (`project_key` thủ thuật cột sinh MySQL)
- `/opt/repos/orca/specs/backend-go/crs/v6/execution-contract/solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md`, `approval/solutions/BE-REQ-SOL-009-generic-approval-domain-and-api.md`, `solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md`, `plan-phase-task/solutions/BE-REQ-SOL-012-...`, `BE-REQ-SOL-013-...`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
