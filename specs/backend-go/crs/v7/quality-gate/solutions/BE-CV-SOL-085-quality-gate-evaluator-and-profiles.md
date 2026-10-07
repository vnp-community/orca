# BE-CV-SOL-085-quality-gate-evaluator-and-profiles: Hàm kết luận cổng chất lượng, profile lưu được, RPC đọc cổng và hồ sơ

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/analyze nào. Mọi thứ ghi "(mới)" là đề xuất; `code-intel-service` **chưa tồn tại** trong `backend-go/services/` (đã `ls`). Solution này là nửa thứ nhất của CR-CV-085; nửa còn lại (miễn trừ, xu hướng, sự kiện `gate_changed`) ở [BE-CV-SOL-085-waivers-and-trend](./BE-CV-SOL-085-waivers-and-trend.md).

**CR:** [CR-CV-085](../../../../../../docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md)
**Service:** `code-intel-service` (mới, do BE-CV-SOL-010 dựng) · `proto/orca/codeintel/v1/codeintel_quality_gate.proto` (mới) · `policy/orca-authz`
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (§1, §2.1 dòng 19, §3.2, §4.2 T10–T12, §5, §6, §8), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§2.3, §3.2, §4.7)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (ranh giới service), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (§The dependency rule, §Layer contracts), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Multi-tenancy, §Migration conventions), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§AuthZ, §Audit logging), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§gRPC conventions, §Event conventions), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (§Resilience patterns)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ | Áp dụng thế nào |
|---|---|
| PQ-01 | `quality_gate_enabled` tắt (mọi RPC `QualityGateService`) → `CODEINTEL_QUALITY_GATE_DISABLED`; `code_intel_enabled` tắt → `CODEINTEL_DISABLED` (thứ tự kiểm: cờ lớn trước) |
| PQ-03 | `CODEINTEL_INVALID_PARAMS` (không có `INVALID_ARGUMENT`); id con không thuộc tenant → `CODEINTEL_NOT_FOUND`; không quyền dự án → `CODEINTEL_NOT_AUTHORIZED` |
| PQ-04 | Request nhận `selector{project_id, worktree_ref}`; **không** nhận `repo_binding_id` |
| PQ-05 | `finding_dismissals` khoá `repo_id`; `Dismiss` không bao giờ miễn cổng với `error`/`blockingSeverities` |
| PQ-06 | Phát hiện cấu trúc dùng `severity ∈ error|warning|info`, `origin ∈ introduced|touched|preexisting|unknown` |
| PQ-07 | File proto là `codeintel_quality_gate.proto`, service `QualityGateService` |
| PQ-24 | Cột cờ có sẵn trong `0002_code_intel_core` (T1); **không** migration thêm cột `tenant_settings` |
| PQ-32/33/34 | Enum chữ thường; `QualityGate.reasons[]` thêm `category`, `tool`; `profile = "<name>@<scope>/v<version>"`; `basedOn` có `headCommit, baseCommit, evaluatedAt, profileVersion` |
| H6, H7 | Backend trả `code`+`params`; `unknown` là trạng thái thật, không thành `pass` |
| H10 | Bảng có `tenant_id`, không FK, hai dialect (T10–T12; migration `0004_quality_gate`, §4.2) |

### 0.2 Lệch giữa CR và hợp đồng (theo hợp đồng)

| # | CR-CV-085 nói | Hợp đồng quyết | Xử lý |
|---|---|---|---|
| L1 | `CODEINTEL_INVALID_ARGUMENT` (`reason=block_mode_not_enabled`) | PQ-03: bỏ mã này | `CODEINTEL_INVALID_PARAMS` + hậu tố `{"field":"mode","reason":"block_mode_not_enabled"}` |
| L2 | `quality_gate.proto`; request `repo_binding_id` | PQ-07, PQ-04 | `codeintel_quality_gate.proto`; `selector` |
| L3 | Cột `quality_gate_enabled` thêm bằng migration của CR này; biến `CODE_INTEL_QUALITY_GATE_ENABLED` | PQ-24, PQ-23: cột có sẵn ở `0002`; biến `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED` | Không viết migration thêm cột; đọc cờ qua cổng cờ của BE-CV-SOL-013 |
| L4 | `introduced: yes|touched|unknown` | PQ-06 đổi tên `origin` 4 giá trị | Bảng quy tắc dùng `origin`; chỉ `introduced` tính vào `fail/warn` cấu trúc; `unknown` → check `structure` `unknown`; `touched`/`preexisting` không tính (giá trị khởi điểm chưa hiệu chỉnh) |
| L5 | `reasons[]` chỉ `check, observed, threshold, result, code, params, runId, waivedCount` | PQ-34 thêm `category`, `tool` | Thêm hai trường tuỳ chọn |
| L6 | Điểm xu hướng khoá không có `source` (Q7) | T12 thêm `source` (O-10 chờ duyệt) | Theo T12 (xem SOL-085-waivers-and-trend) |
| L7 | Payload `gate_changed` có `tenant_id` | §5: `tenant_id` nằm ở envelope `eventbus.Event`, payload không lặp | Theo §5 |
| L8 | `GetQualityGate` chỉ nhận `repo_binding_id`… trả `gate, waivers, evaluatedAt, profileDefinitionDigest` | §3.2 thêm `comparison[]` (CR-086) | Trả `comparison: []` cho tới khi BE-CV-SOL-086-ci-run-merge-and-comparison lắp port (task 085-06) |
| L9 | Định nghĩa profile chỉ có `checks[].required` (bool); CR 2.5 lại cần "bắt buộc **nếu** chạm `backend-go/**`/`.proto`" | Hợp đồng im | **Đề xuất thêm** `checks[].whenChangedPaths: string[]` (glob, ≤ 16, rỗng = luôn áp). Là thay đổi additive của `QualityProfileDefinition` do CR-085 sở hữu; ghi vào câu hỏi mở Q1 để người duyệt xác nhận |

### 0.3 Phụ thuộc chéo khu vực (§7)

| Hướng | Solution | Vì sao |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-085-source-control-quality-notice` (khối cảnh báo trước commit/Create PR; dùng kênh `codeIntel.quality.gate`, `codeIntel.settings.get`) · `FE-CV-SOL-087-quality-scorecard-and-state`, `FE-CV-SOL-087-quality-diff-annotations`, `FE-CV-SOL-087-quality-trend-coverage-hotspot` (đọc `QualityGate`, `QualityProfile`, `runnableProfiles`) | Fixture JSON của task 085-06 là đầu vào của họ; FE chỉ bắt đầu dùng kênh thật sau BE-CV-SOL-040-codeintel-quality-channels |
| AG | Không có solution AG cho CR-085 (§8.2). Tên profile chạy do `AG-CV-SOL-081-quality-profile-catalog-and-preflight` chốt; `fingerprint` do `AG-CV-SOL-082-quality-parsers-and-fingerprint` | Profile mặc định `orca-default` tham chiếu tên đó; sai tên → `warnings[]` khi lưu, `unknown` khi đánh giá |
| BE lân cận | BE-CV-SOL-010/011 (service, `0002`), 013 (cờ, OPA khung, audit, che bí mật), 036 (`ChangeOverlay`), 037 (`Finding`, `finding_dismissals`), 082 (`quality_runs`, `quality_findings`), 083 (coverage), 086 (`comparison`), 040-quality-channels (kênh) | Thứ tự §7.2: `082-BE → 085` (sau 011, 013, 036, 037) |

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/services/` (19 thư mục, **không** có `code-intel-service`), `backend-go/proto/orca/` (17 package, **không** có `codeintel`), `backend-go/policy/orca-authz/` (không có `code_intel.rego`), `common/apperrors/apperrors.go`, `common/dbcapability/capability.go`, `common/outbox/outbox.go`, `common/eventbus/eventbus.go` (`Consumer.Subscribe` :128, `SubscribeEphemeral` :196), `common/auditclient/client.go`, `common/policy/evaluator.go`, `services/mcp-service/internal/adapter/postgres/{tenant_tx.go, tenant_scope_guard_test.go}`, `docs/crs/v7/quality-gate/*`, ba file hợp đồng, TDD 02/03/05/07/08/09.

Xác nhận đúng: (a) `withTenantTx` = `set_config('app.tenant_id', $1, true)` trong transaction (mẫu bắt buộc), `withRelayTx` chỉ cho `outbox.Store`; (b) `dbcapability.Capabilities{SupportsRLS,SupportsJSONB,SupportsReturning}` — MySQL false cả ba, nên `RETURNING`/`->` không được dùng; (c) `auditclient.Client.Append(ctx, tenantID, actorID, action, target, outcome, ip)` không có `actor_type`/`target_type` (khớp ghi chú hợp đồng §3.3).

### Correction relative to CR-CV-085

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | Thêm hành động OPA vào `code_intel.rego` | Chưa có file (BE-CV-SOL-013 tạo); thư mục có `repo.rego`, `task_grant.rego`, `mcp.rego` làm mẫu bố cục | Task 085-07 **phụ thuộc** SOL-013; không tạo file song song |
| C2 | `finding_dismissals` có sẵn | Chưa có bảng nào của service (service chưa dựng) | Evaluator nhận `dismissals` qua **port**, adapter thật do SOL-037/011 |
| C3 | `ChangeOverlay` cung cấp `changedFiles`, hunks | CR-CV-036 còn là đề xuất | Port `ChangedFileSource`; thiếu overlay → check theo tệp `unknown` (CR mục 6) |
| C4 | Frontend `SourceControl.tsx:1792, 3008, 3483, 5247` | Chưa đọc lại ở phiên này (thuộc FE) | Không dùng làm căn cứ ở solution BE; FE-CV-SOL-085 tự re-verify |

## 2. Giải pháp chi tiết

### 2.1 Cây file (tất cả mới; tên theo khái niệm)

```
backend-go/proto/orca/codeintel/v1/codeintel_quality_gate.proto     # service QualityGateService + message của gate/profile/waiver/trend
backend-go/services/code-intel-service/
  internal/domain/quality_profile.go                # QualityProfileDefinition, validate, DisallowUnknownFields
  internal/domain/quality_profile_builtin.go        # orca-default (giá trị khởi điểm chưa hiệu chỉnh)
  internal/domain/quality_gate_evaluator.go         # EvaluateGate (hàm thuần)
  internal/domain/quality_gate_run_selection.go     # chọn run phù hợp cho từng check
  internal/domain/quality_gate_errors.go            # mã lỗi CODEINTEL_PROFILE_INVALID …
  internal/usecase/evaluate_quality_gate.go         # ghép đầu vào từ DB + port
  internal/usecase/get_quality_profile.go, save_quality_profile.go
  internal/usecase/quality_gate_ports.go            # RunReader, FindingReader, ChangedFileSource, StructureFindingSource, CoverageReader, IndexFreshnessReader, WaiverReader, DismissalReader, RunnableProfileLister
  internal/adapter/postgres/quality_profile_repository.go
  internal/adapter/mysql/quality_profile_repository.go
  internal/adapter/grpc/quality_gate_server.go      # struct QualityGateServer + GetQualityGate/GetQualityProfile/SaveQualityProfile
  migrations/{postgres,mysql}/0004_quality_gate.{up,down}.sql   # 3 bảng: T10, T11, T12 (SOL-085-waivers-and-trend dùng T11, T12)
backend-go/policy/orca-authz/code_intel.rego (+ _test.rego)     # thêm quality_read|quality_waive|quality_profile_write — file do SOL-013 tạo
```

`QualityGateServer` nhúng `codeintelv1.UnimplementedQualityGateServiceServer`; các solution 085-waivers, 089, 090, 092, 093 thêm phương thức vào **cùng struct** ở file riêng (không struct thứ hai), để một `Register` duy nhất ở `main.go`.

### 2.2 Chữ ký chính (Go; kiểu bắt nguồn từ proto, không chép lại ở đây)

```go
// domain — hàm thuần, không I/O, không import package nào của ai_review (kiểm bằng test cấu trúc, xem SOL-093)
type GateInput struct {
    HeadCommit, BaseCommit string
    ChangedFiles           ChangedFileSet          // nil = thiếu overlay
    Runs                   []RunView               // quality_runs của binding, đã lọc tenant
    Findings               map[string][]FindingView // theo run_id
    Structure              []StructureFindingView  // Finding (CR-037): rule, severity, origin
    Coverage               *CoverageView           // nil = không có báo cáo
    Waivers                []WaiverView            // đã lọc hiệu lực theo đồng hồ DB ở adapter
    Dismissals             map[string]DismissalView
    Index                  IndexFreshness          // {IndexCommit, Stale}
}
func EvaluateGate(def QualityProfileDefinition, ref ProfileRef, in GateInput, now time.Time) QualityGate

// usecase
func (uc *EvaluateQualityGate) Execute(ctx context.Context, in EvaluateInput) (GateResult, error)
```

`now` nhận từ đồng hồ DB (một lần `SELECT now()`/`CURRENT_TIMESTAMP(6)` ở adapter đọc waiver; không dùng giờ máy ứng dụng, F6 của CR-011).

### 2.3 Quy tắc kết luận (tham chiếu CR-085 §2.3, chỉ ghi phần bổ sung/chốt)

- Bảng kết quả từng kiểm tra và thứ tự `fail > unknown > warn > pass` **theo CR §2.3 nguyên văn**; mỗi `reason` mang `category`/`tool` (PQ-34).
- Chọn run (`quality_gate_run_selection.go`): cùng `repo_binding_id`, `profile` = `check.profile`, `source='local'` (run `ci` chỉ vào `comparison[]`, xem Q2), `head_commit` = HEAD; `succeeded|failed` dùng được; `queued|running` → `unknown` mã `run_running`; `cancelled` → `unknown`; `failed` có `error_code ≠ ''` hoặc không có finding parse được → `unknown` mã `env_not_ready`/`tool_failed`; lấy run `started_at` mới nhất; `scope` bao phủ (`worktree ⊇ changed`; `commitRange` cùng `base_commit`). `work_tree_changed=true` → `unknown` mã `tree_changed_during_run` (mới, vì T8 đã có cột; CR chưa có mã này). Run `dirty=true` vẫn dùng được nhưng thêm `params.dirty="true"`.
- `required` điều kiện theo `whenChangedPaths` (L9); `ChangedFiles == nil` và check có `whenChangedPaths` → `unknown` mã `no_overlay` (không bỏ qua).
- Coverage: `coverage.required=false` → không sinh lý do; `true` mà `Coverage == nil` → `unknown` mã `no_coverage`; `source='estimated'` **không** được dùng để `fail` (chỉ `warn`/`unknown`) — đề xuất, vì CR-083 phân biệt `measured|estimated` (PQ-33) mà CR-085 chưa nói; ghi Q3.
- Phát hiện cấu trúc: chỉ `origin=introduced` tính; `error` → `fail`, `warning` → `warn`; `origin=unknown` → `unknown` (check `structure`); `Dismissal` với mức `error`/`blockingSeverities` **không** bớt (mã `dismissed_not_waived`).
- `reasons[] ≤ 64`; vượt thì gộp theo `check` giữ lý do tệ nhất, thêm `params.merged="N"`.
- Mọi ngưỡng của `orca-default` là **giá trị khởi điểm chưa hiệu chỉnh** (CR §2.5); ghi vào `quality_profile_builtin.go` dưới hằng có chú thích "uncalibrated"; số dùng được vẫn do tenant sửa qua `SaveQualityProfile`.

### 2.4 SQL (hai dialect; chi tiết cột lấy từ hợp đồng §4.2 T10–T12, không chép lại)

Migration `0004_quality_gate` (số có thể dịch khi merge: chạy `ls migrations/postgres` trước). Postgres: schema `codeintel`, `ENABLE`+`FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` `USING`+`WITH CHECK` với `NULLIF(current_setting('app.tenant_id', true), '')::uuid`; MySQL: không RLS, `WHERE tenant_id = ?` ở mọi truy vấn. Khác biệt cần chú ý:

| Việc | Postgres | MySQL |
|---|---|---|
| `definition json` | `JSONB`, `CHECK (octet_length(definition::text) <= 65536)` | `JSON`; kích thước kiểm ở ứng dụng (MySQL không có CHECK tiện cho JSON, `CHECK` cần ≥ 8.0.16) |
| UNIQUE `(tenant_id, scope_key, name)` | UNIQUE | UNIQUE (có `scope_key varchar(160)` chính vì MySQL thiếu chỉ mục duy nhất từng phần) |
| CAS `version` | `UPDATE … WHERE id=$1 AND tenant_id=$2 AND version=$3` + `RowsAffected` | giống, `database/sql` |
| Upsert | `ON CONFLICT … DO UPDATE` | `ON DUPLICATE KEY UPDATE` |
| `RETURNING` | có (`SupportsReturning`) | không → `SELECT` lại cùng transaction |

### 2.5 `SaveQualityProfile` / `GetQualityProfile`

- Lưu: validate (`DisallowUnknownFields`, `schemaVersion==1`, `checks ≤ 32`, ngưỡng ∈ [0, 10 000], `diffCoverage* ∈ [0,1]`, `category` ∈ enum `QualityFinding.category`, `name` khớp `^[a-z0-9-]{1,64}$`), `mode=block` → `CODEINTEL_INVALID_PARAMS` (L1), `json.Valid` + UTF-8 + không `\u0000`; tên `checks[].profile` ∉ `runnable_profiles` → vẫn lưu, `warnings[]` (nguồn danh sách qua port `RunnableProfileLister`, adapter của BE-CV-SOL-082/021; **chưa chốt ai làm**, Q4). `expected_version=0` = tạo; lệch → `CODEINTEL_VERSION_CONFLICT` với `data.currentVersion`.
- Đọc: ưu tiên `repo:<repo_id>` > `tenant` > builtin; không trộn trường (CR §2.1); trả `origin`, `version` (builtin = 0).
- Ẩn profile bảo mật: `runnable_profiles[]` lọc `security-*`/`dependency-diff` khi `quality_security_scan_enabled=false` (PQ-01(4)); `checks[].profile` tham chiếu tên đó vẫn lưu được nhưng gate coi `unknown`.
- Audit `codeintel.quality.profile.save` (target `profile:<repo>:<name>`) qua `auditclient.Append`; `reason` KHÔNG có ở đây.

### 2.6 `GetQualityGate`

Chỉ đọc DB (không gọi agent, không qua `AgentCallGate`); gọi `EvaluateQualityGate`; trả `{gate, waivers[≤50], evaluated_at, profile_definition_digest, comparison[]}`. `profile_definition_digest` = sha256 JSON chuẩn hoá (khoá sắp). `record=true` và `turn_key` được xử lý ở solution waivers-and-trend (task 085-10); ở solution này, tham số `record` được **nhận nhưng bỏ qua** cho tới khi task đó xong.

### 2.7 Quyền và cờ

Thêm `quality_read`, `quality_waive`, `quality_profile_write` vào `code_intel.rego` theo bảng CR §2.9 và hợp đồng §6.3 (`quality_profile_write` chỉ owner/admin). Thứ tự kiểm theo hợp đồng §3: cờ → quyền OPA → phân giải `selector` → (không agent) → quyền trước cache → audit.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Hàm thuần `EvaluateGate`, tính khi đọc | F3; dễ kiểm bằng bảng ca; không kết luận cũ |
| D2 | `unknown` > `warn` | F2; PQ H7 |
| D3 | Chỉ run `source='local'` vào check; CI đi `comparison[]` | PQ-25 chưa nói ngược; tránh một nguồn ghi đè nguồn kia |
| D4 | Port hoá mọi nguồn dữ liệu của service khác | Solution làm được khi 036/037/082 chưa merge, test bằng fake |
| D5 | `whenChangedPaths` (L9) | Nếu không có, `orca-default` không biểu diễn được "go-vet nếu chạm `backend-go/**`" |
| D6 | `estimated` coverage không bao giờ `fail` | Số ước tính không đủ căn cứ chặn (đề xuất, Q3) |
| D7 | Không mã `pass` nếu bất kỳ check `required` thiếu dữ liệu | Tiêu chí CR §4 |

## 4. Tiêu chí chấp nhận

- [x] `EvaluateGate` có bảng ca phủ: thiếu run → `unknown`; run sai HEAD không dùng; `running` → `unknown`; `fail`+`unknown` → `fail` kèm cả hai lý do; chỉ `warn` → `warn`; đủ → `pass`; **không** ca nào cho `pass` khi check `required` thiếu dữ liệu.
- [x] `index_commit ≠ HEAD` (`freshness.indexMustMatchHead`) → check `structure` `unknown`, `basedOn.stale=true`; lint/typecheck/test không đổi.
- [x] Coverage tắt → không có lý do coverage; bật mà thiếu → `unknown`/`no_coverage`.
- [x] `SaveQualityProfile` từ chối `mode=block`, khoá lạ, ngưỡng ngoài khoảng, version lệch; hai dialect cùng kết quả.
- [x] `member` gọi `SaveQualityProfile` → `CODEINTEL_NOT_AUTHORIZED`; cờ tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; cờ tổng tắt → `CODEINTEL_DISABLED`.
- [x] Mọi truy vấn có `tenant_id`; test AST kiểu `tenant_scope_guard_test.go` phủ repository mới; tenant A không đọc/ghi profile tenant B ở cả hai dialect.
- [x] Fixture JSON `QualityGate` (có/không trường tuỳ chọn) khớp `CONTRACT-codeintel-ui-api.md` §4.7.
- [x] `go test` hai dialect (`-tags=integration`), `opa test policy/orca-authz/`, `buf lint` + `buf breaking` (gọi trực tiếp, không qua `make proto-lint` có `|| true`) xanh; không `max-lines` disable.

## 5. Kiểm thử

- **Unit (domain):** bảng ca `EvaluateGate` (≥ 40 ca: mỗi dòng bảng §2.3 của CR × có/không waiver), `Validate` profile (fuzz ngắn cho JSON), chọn run, cắt `reasons[]`, digest ổn định (thứ tự khoá).
- **Repository (integration, mỗi dialect):** CAS, unique, UTF-8/`\u0000`, cách ly tenant; role Postgres `NOSUPERUSER NOBYPASSRLS` (dev compose dùng superuser nên RLS không có hiệu lực — hợp đồng §4.1).
- **Use case:** cờ tắt, quyền, port giả trả thiếu từng nguồn (`ChangedFiles=nil`, `Coverage=nil`, run `running`).
- **Hợp đồng:** golden JSON dùng chung với FE (CR-CV-070).
- **Đo thật (chưa làm được):** chạy `orca-default` trên 10–20 worktree thật rồi mới gọi ngưỡng là "đã hiệu chỉnh".

## 6. Rủi ro và điểm chưa kiểm chứng

- Ngưỡng `orca-default` chỉ là phỏng đoán (giá trị khởi điểm chưa hiệu chỉnh); nguy cơ nhiễu.
- `QualityRun.error_code`/`dirty_fingerprint` (T8) chưa có hiện thực (BE-CV-SOL-082); thiếu thì mọi run `failed` không phân biệt được lỗi công cụ → coi `unknown`.
- Không phát hiện được sửa tiếp trên cùng commit sau khi chạy (backend chỉ biết `commit`); chỉ giảm nhẹ bằng `work_tree_changed` và `finishedAt` ở UI.
- Chưa chạy `buf`, `opa test`, `golang-migrate` nào; chưa kiểm chứng `golang-migrate` MySQL với database `codeintel`.
- Bộ che bí mật dùng chung chưa có chủ (hợp đồng O-16); solution này không lưu chuỗi tự do nên ít phụ thuộc.
- `RunnableProfileLister` chưa có chủ (Q4); thiếu thì `runnable_profiles=[]` và cảnh báo.

## 7. Câu hỏi mở

- **Q1.** Chấp nhận thêm `whenChangedPaths` vào `QualityProfileDefinition` (L9)? Nếu không, `orca-default` phải coi lint/typecheck/unit luôn bắt buộc và bỏ go-vet/proto có điều kiện.
- **Q2.** `CiComparison.relation=local_pass_ci_fail` có hạ cổng về `unknown/warn` không? CR-085 và hợp đồng chưa nói (PQ-25 chỉ cấm `pass` cho `comparison`); mặc định không đổi verdict.
- **Q3.** Coverage `estimated` có được tính vào `warn`? (D6).
- **Q4.** Ai cung cấp `RunnableProfileLister` (collector BE-021 hay BE-082) và cache bao lâu (đề xuất 60 s như preflight)?
- **Q5.** `GetQualityGate(record=true)` là ghi trong RPC đọc: giữ `quality_read` (hợp đồng §3.2) hay yêu cầu `review_write`? Đề xuất giữ vì chỉ chèn điểm idempotent.
- **Q6 (kế thừa CR).** Khi nào `mode=block`; xác nhận một lần cho chuỗi commit→push→PR (không thuộc BE).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md`, `README.md`; `/opt/repos/orca/docs/crs/v7/README.md` (O1, O8, O9, O11, O13, O14; 3.10; mục 6, 8)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md`, `CONTRACT-codeintel-ui-api.md`, `CONTRACT-codeintel-agent-rpc.md` (§5.1 `quality.listProfiles`)
- `/opt/repos/orca/backend-go/common/{apperrors,dbcapability,outbox,eventbus,auditclient,policy}/`, `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/{tenant_tx.go,tenant_scope_guard_test.go}`
- Mẫu định dạng: `/opt/repos/orca/specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-001-scaffold-request-service.md`
