# CR-REQ-030: Đánh giá tác động và chấm điểm rủi ro cho Solution, Plan và thực thi
| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-030 |
| **Tên** | Thực thể `ImpactAssessment`, `RiskAcceptance`, `RiskPolicy`; điểm rủi ro bằng quy tắc có phiên bản; bộ thu thập chạy công cụ trên dev server; ánh xạ mức rủi ro sang cổng duyệt; lệch kế hoạch khi thực thi; dữ liệu đồ thị cho frontend; chạy bóng trước khi chặn cổng |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-007 (Solution, `options[].affected_areas`), CR-REQ-009 (Approval, `UpdatePendingDigest`), CR-REQ-010 (người duyệt theo `team:<id>`), CR-REQ-012 (cây Plan, nhãn), CR-REQ-013 (`AdvanceExecution`), CR-REQ-014 (`TypePolicy`, cổng `pre_deploy`), CR-REQ-029 (`files_changed` đã kiểm chứng, `base_sha`, `agent.exec` qua `Relay`), CR-REQ-032 (`GraphPayload`) và CR-REQ-036 (tên kênh, UI chấp nhận rủi ro), CR-REQ-034 (cổng AI cho phần diễn giải), CR-REQ-035 (`AppendDetailed`, `request.rego`) |
| **Mở khoá** | Màn tác động ở CR-REQ-020, 021, 022, 032, 036 (frontend); CR-REQ-031 (danh mục service làm nền đồ thị Kiến trúc) |
| **Tác động** | `request-service` (domain, usecase, adapter, migration hai dialect, proto); không đổi `task-service`, `agent`; sửa nhỏ CR-REQ-009, 012, 013, 014, 016 (mục 9) |
## 1. Bối cảnh và vấn đề
Solution và Plan khi thực thi có thể đổi kiến trúc, hợp đồng, dữ liệu của nhiều service. Hiện chỉ có `affected_areas` do AI nêu trong `options[]` (CR-REQ-007 mục 2.3) và `irreversible` do AI gắn (CR-REQ-012). Người duyệt không có số liệu đo. Đã kiểm tra ngày 2026-10-06:

- Mẫu thủ công có sẵn: `docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md` (ví dụ "gitnexus impact: 34 direct caller, CRITICAL"). Cần tự động hoá đúng kiểu đó.
- GitNexus CLI có `impact`, `detect-changes --scope compare --base-ref <ref>`, `status` (`node .gitnexus/run.cjs --help`). CLI không có cờ `--json` trong phần `--help` đã đọc, nên định dạng đầu ra để phân tích bằng chương trình chưa kiểm chứng. `status` hiện báo đúng tình huống rủi ro: indexed commit `d819812`, current commit `1b0c760`, "stale". `.gitnexus/gitnexus.json` chứa `lastCommit`, `indexedAt`, `stats` nên tuổi index đọc được bằng `fs.readFile`.
- `backend-go/proto/buf.yaml`: `lint: STANDARD`, `breaking: FILE`. `backend-go/Makefile` dòng 81: `cd proto && buf lint && buf breaking --against '.git#branch=main' || true`. Do `A && B || true`, mọi lỗi của `lint` lẫn `breaking` đều bị nuốt; `make proto-lint` luôn thành công. CI chỉ có `buf lint --path orca/mcp` (`.github/workflows/backend-go-mcp-service.yml`), không có `buf breaking` ở workflow nào.
- Đã chạy thử trên máy này (không phải dev server): `cd backend-go/proto && buf breaking --against '../../.git#ref=HEAD~3,subdir=backend-go/proto' --error-format=json` trả exit 0 sau 7,6 giây. Cú pháp `.git#...` phải trỏ tới thư mục `.git`, không phải thư mục làm việc. Chưa thử trong worktree `task/<id>` hay trên host SSH.
- `backend-go/ci/` chỉ có kiểm tra định tuyến nginx, MCP, bundle OPA; không có kiểm tra migration hay độ phủ. `backend-go/policy/orca-authz/` có `task_grant.rego`, `admin.rego`, `tenant.rego` (kiểm tra chạm chính sách).
- Agent đã có `agent.exec` (tối đa 5 phút mỗi lệnh), `fs.*`, `git.*` (`agent/src/relay/`). Bộ thu thập chỉ cần các RPC này; không sửa agent.
- Hai kênh đã có test bắt lệch: `parity_test.go` của `mcpserver/tools` (README v6 mục 8, điều chỉnh 13) và `config/max-lines-baseline.txt` (cấm thêm `max-lines` disable). Cả hai là tín hiệu đo được cho chiều Hợp đồng và Chất lượng.

Nghiên cứu nền: `docs/research/receive-request/impact-assessment-and-risk-scoring.md`; hợp đồng đồ thị: `frontend-visualization-and-ux.md` mục 4.
## 2. Giải pháp đề xuất
Nguyên tắc: dữ liệu do công cụ chạy xác định; điểm do quy tắc có phiên bản (cùng đầu vào cho cùng điểm); AI chỉ diễn giải và gợi ý giảm rủi ro, không được đổi điểm (lý do: điểm AI không tái lập, khó kiểm toán, dễ bị nội dung Request lèo lái).

### 2.1 Thực thể (request-service, hai dialect, mọi bảng có `tenant_id`)
**`impact_assessments`** (append-only theo `revision`)

| Cột | Postgres | MySQL | Ghi chú |
|---|---|---|---|
| `id`, `tenant_id`, `request_id` | `UUID` | `CHAR(36)` | không FK chéo service; FK nội bộ tới `requests(id)` |
| `subject_type` | `TEXT CHECK IN ('solution_option','plan','phase','task','actual_task','actual_phase')` | `VARCHAR(20)` + CHECK | ba thời điểm, mục 2.2 |
| `subject_id` | `TEXT NOT NULL` | `VARCHAR(80)` | `<solution_id>:<option_id>` hoặc id task |
| `subject_digest` | `CHAR(64)` | `CHAR(64)` | digest nội dung đã đánh giá (Option, cây Plan, `base_sha..head_sha`) |
| `revision` | `INT` | `INT` | UNIQUE `(tenant_id, subject_type, subject_id, revision)` |
| `rules_version`, `policy_id` | `TEXT`, `UUID NULL` | `VARCHAR(16)`, `CHAR(36)` | `rp/1` là bản mặc định trong mã; `policy_id` khi tenant có `RiskPolicy` riêng |
| `mode` | `TEXT CHECK IN ('shadow','enforce')` | | mục 2.9 |
| `status` | `TEXT CHECK IN ('collecting','ready','partial','failed','superseded')` | | `partial`: có công cụ lỗi, điểm tính từ phần còn lại, chiều thiếu ghi bất định |
| `level` | `TEXT CHECK IN ('low','medium','high','critical') NULL` | | |
| `score` | `SMALLINT NULL` | | 0 đến 100 |
| `dimensions` | `JSONB` | `JSON` | 9 chiều: `{score, measured, signals[]}` |
| `triggers` | `JSONB` | `JSON` | luật cứng đã kích hoạt |
| `findings` | `JSONB` | `JSON` | `[{finding_id, dimension, code, severity, path?, message, evidence_run_id}]` |
| `confidence` | `TEXT CHECK IN ('high','medium','low')` | | từ tuổi index, công cụ lỗi, số chiều không đo được |
| `digest` | `CHAR(64)` | | SHA-256 JSON chuẩn tắc của `dimensions`, `triggers`, `findings`, `level`, `rules_version`; **không** gồm `narrative` |
| `narrative` | `JSONB NULL` | `JSON NULL` | diễn giải của AI, `generated_by='ai'`; không nằm trong `digest` |
| `lease_owner`, `lease_expires_at` | | | mẫu `analysis_runs` của CR-REQ-007 (lease 90 giây, quét 30 giây) |
| `created_by`, `created_at`, `finished_at` | | | |

Chỉ một `collecting` cho mỗi `(subject_type, subject_id)`: Postgres partial unique index; MySQL cột sinh `active_key` + `UNIQUE` (cùng thủ thuật `project_key` của `task_sources` 0012).

**`impact_tool_runs`**: `id`, `tenant_id`, `assessment_id`, `tool` (`gitnexus_impact`, `gitnexus_detect_changes`, `gitnexus_status`, `codegraph`, `buf_breaking`, `migration_scan`, `go_cover`, `contract_scan`, `policy_scan`), `tool_version`, `command_digest`, `exit_code`, `duration_ms`, `index_commit`, `index_age_seconds`, `status` (`ok|timeout|error|skipped`), `output` (TEXT, cắt 256 KB), `output_digest`, `created_at`. Là bằng chứng cho từng `finding_id`.

**`risk_acceptances`**: `id`, `tenant_id`, `request_id`, `assessment_id`, `assessment_digest`, `finding_id`, `level` (mức của phát hiện), `accepted_by`, `rationale` (≥ 10 ký tự, khớp CR-REQ-036), `approval_id` NULL, `created_at`. UNIQUE `(tenant_id, assessment_id, finding_id, accepted_by)`. Hiệu lực chỉ khi `assessment_digest` trùng bản đánh giá hiện hành; digest đổi thì phải chấp nhận lại.

**`risk_policies`**: `id`, `tenant_id`, `version` INT, `status` (`draft|shadow|active|retired`), `thresholds`, `weights`, `hard_rules`, `gate_mapping` (JSON), `created_by`, `activated_at`. Không có dòng nghĩa là dùng bản mặc định `rp/1` biên dịch trong mã (`internal/domain/risk_rules_v1.go`, mới). Mỗi tenant tối đa một `active` và một `shadow` (chỉ mục duy nhất như trên).

**`risk_outcomes`** (dữ liệu hiệu chỉnh): `request_id`, `predicted_level`, `actual_level` (từ đánh giá `actual_*`), `incident` BOOL, `rolled_back` BOOL, `override_count`, `recorded_at`. Điền khi Request `completed` hoặc về backlog (người khai `incident`, `rolled_back` tay ở v1).

### 2.2 Ba thời điểm tính
| Thời điểm | `subject_type` | Kích hoạt | Đầu vào | Độ tin cậy |
|---|---|---|---|---|
| Option của Solution | `solution_option` | sự kiện `orca.request.solution.proposed` (CR-REQ-007), mỗi Option | `affected_areas[]` do AI nêu, `breaking_change`, `rollback`; truy vấn công cụ trên tên khu vực | thấp hơn: dựa vào khai báo của AI |
| Plan, Phase, Task | `plan`, `phase`, `task` | sau `GeneratePlan` COMMIT (CR-REQ-012 bước 3) | `scope.include/create` của `TaskSpec` (CR-REQ-029), đồ thị `depends_on`, nhãn, số task | trung bình |
| Thực tế | `actual_task`, `actual_phase` | sau `VerifyExecution` đạt (CR-REQ-029 mục 2.6) và khi Phase `done` | `git diff` thật `base_sha..HEAD`, `gitnexus detect-changes --scope compare --base-ref <base_sha>` | cao nhất; dùng cho lệch kế hoạch (2.8) và hiệu chỉnh |

Bản đánh giá của Phase lấy chiều cao nhất của các task con cộng chiều Quy mô tính trên cả Phase; Plan tương tự trên các Phase. Không tính trung bình đơn giản.

### 2.3 Chín chiều: tín hiệu đo được và bảng ngưỡng
Mỗi chiều cho điểm 0 đến 100 theo bảng ngưỡng cố định (bản `rp/1`; tenant ghi đè qua `RiskPolicy.thresholds`). Chiều không đo được (công cụ lỗi, thiếu index) có `measured=false`, mang điểm trung tính 50 khi gộp (không bao giờ tính là thấp) và tăng bất định.

| Chiều | Tín hiệu (nguồn) | Ngưỡng điểm (mỗi dòng cộng dồn, trần 100) |
|---|---|---|
| Kiến trúc | thư mục mới dưới `backend-go/services/` hoặc mục mới ở `go.work` (diff); cạnh gRPC giữa service thêm mới (import `proto/gen/go/orca/<svc>` mới trong `adapter/grpcclient`); chạm ranh giới `api-gateway`, `wscompat`, `proto/` | service mới +60; mỗi cạnh liên service mới +15 (tối đa 45); chạm ranh giới +20 |
| Tương thích hợp đồng | `buf breaking` (số vi phạm); kênh WS thêm, xoá, đổi tên (diff `wscompat/channels_*.go`, heuristic); `ToolSpec` thiếu (kết quả `parity_test.go`); đổi payload sự kiện outbox (diff `*payload*.go`, heuristic) | breaking ≥1: 80, mỗi vi phạm thêm +5; kênh xoá hoặc đổi tên +60, thêm +10; ToolSpec thiếu +30; payload outbox đổi +40 |
| Dữ liệu | quét migration (2.5): có migration; thiếu `.down.sql`; chỉ một dialect; `DROP`/`ALTER ... TYPE`/`TRUNCATE`/`DELETE`/`UPDATE` không `WHERE`; thêm `NOT NULL` không `DEFAULT`; backfill | có migration +20; thiếu down +50; một dialect +50; thao tác phá huỷ +60; `NOT NULL` không default +30 |
| Phạm vi ảnh hưởng | `gitnexus impact` mỗi symbol đổi: số người gọi trực tiếp và gián tiếp, số luồng thực thi, số service chạm | gọi trực tiếp <5: 10; 5 đến 20: 35; 21 đến 50: 60; >50: 85; mỗi luồng thực thi bị chạm +5; ≥3 service +20 |
| Bảo mật và quyền | đường dẫn chạm `auth-service`, `tenant-service`, `credential-broker-service`, `common/tenant`, `policy/**/*.rego`, `SecretRedactor`; ingress mới (webhook, route công khai) | mỗi vùng nhạy cảm +70 (không cộng dồn quá 100); ingress mới +40 |
| Vận hành | proto đổi giữa nhiều service (thứ tự deploy); hành vi mới không sau cờ; không có `rollback`; chạm `agent/`, `infra-fleet-service`, `relay`, đường dẫn Windows (SSH, remote, đa nền tảng theo AGENTS.md) | thứ tự deploy +30; không cờ +30; không đường quay lui +60; chạm relay/agent +25; chạm mã theo nền tảng +15 |
| Chất lượng | `go test -cover` theo package có file đổi (độ phủ theo package, không theo symbol); trạng thái build, lint, test nền (CR-REQ-029 Check nền); `max-lines` disable mới (`config/scripts/check-max-lines-ratchet.mjs`) | phủ <30%: 60, 30 đến 60%: 30; không có test ở package đổi +40; nền đỏ 80; `max-lines` disable mới 100 |
| Bất định | số `open_questions` và giả định của Solution; tuổi index; số công cụ lỗi; chiều không đo được; độ tin cậy `ai` của Option | mỗi `open_question` +8; index lỗi thời +40; mỗi công cụ lỗi +15; mỗi chiều không đo được +10 |
| Quy mô | số file, số dòng, số service, số task | file <5: 5; 5 đến 15: 25; 16 đến 40: 50; 41 đến 100: 75; >100: 95; ≥3 service +15 |

### 2.4 Cách tính điểm và mức
`internal/domain/risk_scoring.go` (mới), hàm thuần `Score(dims [9]DimensionResult, p RulePolicy) Assessment` không đọc đồng hồ, môi trường, không gọi AI:

```
base   = round(0.6 * max(s_d) + 0.4 * Σ(w_d * s_d) / Σ(w_d))     // s_d của chiều không đo được = 50
level  = Thấp (<25) | Trung bình (25-49) | Cao (50-74) | Nghiêm trọng (>=75)
level  = max(level, mức tối thiểu của các luật cứng 2.4.1)
```

Trọng số `rp/1`: bảo mật 1,2; kiến trúc, hợp đồng, dữ liệu, phạm vi ảnh hưởng 1,0; vận hành, chất lượng 0,8; bất định, quy mô 0,6. Chiều cao nhất chiếm 60% để một chiều nguy hiểm không bị các chiều thấp che. Mọi hằng số nằm trong `RulePolicy`; đổi hằng số thì tăng `rules_version` (golden test bắt buộc cập nhật). Cùng `dims` cho cùng `score`, `level`, `digest` (test thuộc tính chạy 1000 lần).

#### 2.4.1 Luật cứng (nâng mức tối thiểu, bất kể điểm)
| Điều kiện | Mức tối thiểu |
|---|---|
| `buf breaking` ≥ 1 vi phạm, hoặc kênh WS/tool MCP bị xoá hoặc đổi tên | Cao |
| Migration không đảo ngược (không có `.down.sql`, hoặc thao tác phá huỷ) | Cao |
| Chạm xác thực, cách ly tenant, credential, OPA | Cao |
| Không có đường quay lui, hoặc có bước `irreversible` mà không sau cờ tính năng | Cao |
| Chạm hơn N service (N = 3, cấu hình) | Cao |
| `max-lines` disable mới (AGENTS.md cấm) | Cao |
| Index phân tích lỗi thời hoặc không có kết quả | tăng một bậc, `confidence=low`, ghi "chưa đánh giá được" |
| ≥ 3 chiều không đo được | Trung bình, `confidence=low` |

Index lỗi thời: đọc `.gitnexus/gitnexus.json` (`lastCommit`); `git rev-list --count <lastCommit>..<base_sha>` lớn hơn `REQUEST_IMPACT_INDEX_MAX_COMMITS_BEHIND` (mặc định 30, đề xuất) hoặc `lastCommit` không tới được. Bộ thu thập không tự chạy `analyze` (tốn thời gian, chưa đo trên repo 248 nghìn symbol); chỉ báo. Cờ `REQUEST_IMPACT_REINDEX` (mặc định `false`) dành cho phiên bản sau.

#### 2.4.2 AI chỉ diễn giải
Sau khi điểm đã chốt, `ImpactNarrator` gọi AI qua cổng của CR-REQ-034 (`PromptRegistry` bước `impact_narrative`, ngân sách, ghi sổ; cùng đường `ai.complete` của CR-REQ-007) với **chỉ** JSON của bản đánh giá, trong khối rào vì `path` và `message` có thể mang nội dung từ repo. Đầu ra bắt buộc `{summary, top_reasons[3], mitigations[]}`; bị loại nếu chứa trường `score`, `level` hoặc lệch tên chiều. Ghi vào `narrative`, đánh dấu `generated_by=ai`. Lỗi AI không ảnh hưởng điểm (bản đánh giá vẫn `ready`). Gợi ý giảm rủi ro (chia Phase, thêm cờ, thêm test, đổi thứ tự) chỉ là văn bản; không tự sửa Plan.

### 2.5 Bộ thu thập (`ImpactCollector`) chạy công cụ trên dev server qua `agent.exec`
`internal/usecase/collect_impact.go` (mới), chạy bất đồng bộ bền (lease như `analysis_runs`), gọi `Relay` tới dev server của project (cùng đường, SSH và remote đều chạy được). Mỗi lệnh có `timeout ≤ 300 giây` (giới hạn `agent.exec`), tổng ≤ `REQUEST_IMPACT_BUDGET` (mặc định 10 phút, đề xuất). Công cụ nào lỗi thì `impact_tool_runs.status` ≠ `ok`, bản đánh giá `partial`, chiều tương ứng `measured=false`.

| Công cụ | Lệnh (qua `agent.exec`, `cwd` = repo hoặc worktree) | Chiều | Ghi chú |
|---|---|---|---|
| Năng lực | `command -v node buf go`; `fs.readFile .gitnexus/gitnexus.json` | tất cả | thiếu công cụ thì chiều liên quan `measured=false`; báo cáo năng lực đầy đủ thuộc CR-REQ-031 |
| GitNexus | `node .gitnexus/run.cjs impact <symbol> --direction upstream` mỗi symbol đổi (tối đa 20 symbol, `--depth 3`); `detect-changes --scope compare --base-ref <base_sha>` cho đánh giá thực tế | Phạm vi, Kiến trúc | định dạng đầu ra chưa kiểm chứng; viết `GitNexusOutputParser` có golden test từ mẫu thật, lỗi parse thì `status=error` |
| `buf breaking` | `buf breaking --against '<repo>/.git#ref=<base_sha>,subdir=backend-go/proto' --error-format=json` trong `backend-go/proto`, **không** `|| true`, không dùng `make proto-lint` | Hợp đồng | exit ≠ 0 và có dòng JSON là vi phạm; exit ≠ 0 không có JSON là `error`. Đã chạy được trên máy này (7,6 giây); chưa thử trên dev server |
| Quét migration | `git diff --name-only -z <base_sha>` rồi đọc file `**/migrations/{postgres,mysql}/*.sql` bằng `fs.readFile`; `MigrationScanner` (Go, `internal/adapter/migrationscan/`, mới) kiểm cặp up/down, hai dialect, từ khoá phá huỷ | Dữ liệu | thuần Go, không cần công cụ trên dev server ngoài `git` |
| Quét hợp đồng và chính sách | cùng diff; `ContractScanner` đối chiếu `wscompat/channels_*.go`, `tools/excluded_channels.yaml`, payload outbox; khớp đường dẫn với vùng nhạy cảm và `policy/**/*.rego` | Hợp đồng, Bảo mật | heuristic bằng regex; `severity` thấp hơn cho kết quả suy đoán. CodeGraph (`codegraph explore`) chỉ đính kèm làm bằng chứng, không vào điểm ở `rp/1` |
| Độ phủ | `go test -cover ./<pkg>/...` cho từng package có file đổi (dòng `coverage: X% of statements`) | Chất lượng | theo package; chạy chậm nên chỉ ở thời điểm Plan và Thực tế, tối đa 5 package |
| Check nền | kết quả `task_readiness_reports` (CR-REQ-029) | Chất lượng | dùng lại, không chạy lại |

Với thời điểm `solution_option` chưa có diff: `affected_areas[].name` ánh xạ sang đường dẫn bằng bảng `AreaResolver` (service thì `backend-go/services/<name>`, module, `proto/orca/<name>`); khu vực không ánh xạ được thì `finding.code=AREA_UNRESOLVED` và chiều Bất định tăng. Độ tin cậy của đánh giá Option luôn tối đa `medium`.

### 2.6 Ánh xạ mức rủi ro sang cổng duyệt
`RiskGate` (`internal/usecase/risk_gate.go`, mới) là một `ApprovalGuard` ghép vào `Approve` (CR-REQ-009) và một điều kiện của `AdvanceExecution` (CR-REQ-013). Chỉ `mode=enforce` mới chặn (2.9).

| Mức | Hệ quả (CR-REQ-009, 010, 014) |
|---|---|
| Thấp | luồng bình thường theo loại Request |
| Trung bình | `Approve` bắt buộc đã mở phần tác động (cờ `viewed_impact_digest` trong `DecideApprovalRequest`); `ListPendingForUser` đánh dấu không cho duyệt hàng loạt |
| Cao | người duyệt thuộc `team:<risk_approver_team>` (CR-REQ-010 `ApproverPolicy`); mỗi phát hiện từ Cao trở lên cần `RiskAcceptance` (`REQUEST_RISK_ACCEPTANCE_REQUIRED`); Plan phải có cờ tính năng (nhãn `gate:feature_flag`, mới) và task `rollback` (CR-REQ-014 2.5); thêm cổng `pre_deploy` (CR-REQ-014) cho mọi loại; `AdvanceExecution` chỉ cho task thuộc Phase đã duyệt |
| Nghiêm trọng | mọi điều của mức Cao; Plan phải chia Phase sao cho mỗi Phase tối đa mức Cao (`REQUEST_PLAN_RISK_TOO_HIGH` ở `PlanPreconditions`, CR-REQ-012/014), mỗi Phase có Approval `phase` riêng; cần hai người duyệt khác nhau (`required_approvals=2`, mục 9 đề xuất CR-REQ-009); có task nhãn `check:rollback_rehearsal` (mới) hoàn tất trước Phase đầu |

Giữ nguyên nguyên tắc CR-REQ-009: Approval gắn `subject_digest`; đưa `assessment.digest` vào digest của chủ thể `solution` (digest Option đã chọn) và `plan` (digest cây). Đánh giá xong sau khi Approval đã mở thì gọi `UpdatePendingDigest` (CR-REQ-009); `Approve` với `expected_digest` cũ bị từ chối như thường. Đang `collecting`: `Approve` trả `REQUEST_RISK_ASSESSMENT_PENDING` (chỉ ở `enforce`).

**Ghi đè (override):** `OverrideRiskGate{request_id, gate, reason}` cho người thuộc `team:<risk_override_team>` hoặc admin; bắt buộc `reason` ≥ 20 ký tự; ghi `audit` (CR-REQ-024, `AppendDetailed`) và `risk_outcomes.override_count`; không bỏ được luật bất biến "người xác nhận từng phát hiện Cao" trừ khi chính override nêu `finding_id`.

### 2.7 Kiểm soát sau khi bắt đầu thực thi: lệch kế hoạch
Sau mỗi task `VerifyExecution` đạt, `AssessActual` tạo `actual_task`; khi Phase `done` tạo `actual_phase`. So với bản đánh giá dự kiến của cùng chủ thể:

| Điều kiện lệch | Hành động |
|---|---|
| tập service thực tế khác tập dự kiến (thêm service) | `drift` |
| `level` thực tế cao hơn dự kiến một bậc trở lên | `drift` |
| `score` thực tế − dự kiến > `REQUEST_RISK_DRIFT_DELTA` (mặc định 15, đề xuất) | `drift` |
| tín hiệu tuyệt đối mới (`buf breaking` ≥ 1 mà dự kiến 0, migration không đảo ngược) | `drift` luôn |

`drift` ở `enforce`: ghi sự kiện `orca.request.impact.drift_detected`, mở Approval `subject_type=phase`, `stage=drift_review` (CR-REQ-009 giữ nguyên bảng `subject_type`, dùng `stage`) cho Phase đang chạy; `RiskGate` chặn `AdvanceExecution` cho đến khi có quyết định. `OnApproved`: tiếp tục, ghi bản đánh giá mới làm baseline; `OnRejected`: `ReturnToBacklog(stage=phase, category=rejected)` (CR-REQ-006). Task đang chạy vẫn chạy tới cùng (không có RPC dừng run, CR-REQ-013 mục 6). Ở `shadow`: chỉ ghi và hiển thị.

### 2.8 Hợp đồng dữ liệu đồ thị cho frontend và API đọc
Hợp đồng là `GraphPayload` do CR-REQ-032 (frontend, `shared/graph-types.ts`) định nghĩa; CR này trả đúng dạng đó (JSON của proto, camelCase khi qua gateway). Node `{id, kind, label, group, risk, status}`, cạnh `{from, to, kind, change}`:

```json
{ "lens": "architecture", "nodes": [{"id": "svc:task-service", "kind": "service", "label": "task-service", "group": "backend-go",
    "risk": "high", "status": "modified"}],
  "edges": [{"from": "svc:request-service", "to": "svc:task-service", "kind": "calls", "change": "added"}],
  "totalNodes": 63, "truncated": true, "assessedAt": "2026-10-06T10:00:00Z", "tool": "gitnexus", "stale": false }
```
- `risk` ∈ `low|medium|high|critical|unknown`: `unknown` là chiều không đo được hoặc index lỗi thời, **không bao giờ** trả `low` thay cho nó. `change` ∈ `added|removed|unchanged`, độc lập với `risk`. `status` là chuỗi theo lens (`modified`, `breaking`, `irreversible`, trạng thái task). `id` ổn định giữa các lần gọi (`<kind>:<tên>`). Server cắt ở `max_nodes` (mặc định 50, `truncated=true`, `totalNodes` là số trước khi cắt); gom nhóm hiển thị là việc của client (`graph-grouping.ts`, CR-REQ-032). `meta.finding_ids[]` tuỳ chọn để mở bằng chứng.
- Lens (tên theo CR-REQ-032): `architecture` (nền: danh mục service của CR-REQ-031, lớp phủ: diff), `contract` (`buf breaking`, `ContractScanner`: proto, kênh WS, sự kiện, tool MCP), `data` (`MigrationScanner`: bảng, migration, cờ không đảo ngược, hai dialect), `impact` (GitNexus `impact`: symbol, luồng theo bước nhảy), `plan` và `execution` (client dựng từ cây Plan và `task_run_outcomes`, chỉ lấy `risk` và `drift` từ CR này), `flow` hoàn toàn phía client. Backend chỉ dựng bốn lens đầu.

RPC đọc và ghi (`RequestService`; tên kênh theo CR-REQ-036 và CR-REQ-032, CR-REQ-016 chốt; mỗi kênh cần `ToolSpec` hoặc dòng loại trừ, README v6 mục 8 điều 13):

| RPC | Kênh WS | Trả |
|---|---|---|
| `RequestImpactAssessment{subject_type, subject_id}` | `impact.request` | `assessment_id` (khởi chạy, hoặc trả bản `ready` còn hợp lệ) |
| `GetImpactAssessment{subject_type, subject_id}` | `impact.get` | tóm tắt: mức, điểm, 3 lý do chính, độ tin cậy, tuổi index, `narrative` |
| `GetImpactGraph{request_id, subject_type, subject_id, lens, base?, max_nodes}` | `impact.graph` | `GraphPayload` |
| `CompareImpact{solution_id}` | `impact.compare` | ma trận Option × 9 chiều (bảng so sánh ở CR-REQ-020) |
| `ListImpactFindings{assessment_id, dimension?, min_level?}`, `GetImpactEvidence{finding_id}` | `impact.findings`, `impact.evidence` | phát hiện xếp theo phạm vi ảnh hưởng kèm độ phủ; bằng chứng (`impact_tool_runs.output` đã cắt) |
| `GetPlanRiskHeatmap{plan_task_id}` | `impact.heatmap` | mức rủi ro theo Phase và Task |
| `AcceptRisk{assessment_id, finding_id, rationale, assessment_digest}` | `impact.accept` | `RiskAcceptance` (CR-REQ-036) |
| `GetImpactDrift{phase_id}` | `impact.drift` | dự kiến so với thực tế (CR-REQ-036 2.7) |
| `OverrideRiskGate{request_id, gate, reason}` | `risk.override` | mục 2.6 |
| `GetRiskPolicy`, `SetRiskPolicy` | `risk.policy.get|set` | chỉ admin tenant |

### 2.9 Chạy bóng (shadow) trước khi dùng điểm để chặn cổng
Vì ngưỡng và trọng số là ước lượng, chặn cổng bằng điểm chưa hiệu chỉnh sẽ chặn sai. Ba giai đoạn:

0. **Thử ngoại tuyến (trước khi viết code sản xuất):** lấy 3 đến 5 thay đổi đã có trong lịch sử repo (ví dụ các mục trong `IMPACT-ASSESSMENT-2026-09-15-...md`: CR-PW-007 mức thấp, 3 symbol; CR-PW-010 `ConnectionResolver` mức Nghiêm trọng, 34 caller), chạy bộ thu thập bằng CLI trên máy dev, so mức tính ra với mức thật. Tiêu chí qua: khớp mức hoặc lệch tối đa một bậc ở ≥ 4/5 trường hợp; nếu không thì chỉnh bảng 2.3 trước.
1. **`shadow`:** mọi đánh giá có `mode=shadow`, hiển thị với nhãn "tham khảo", không chặn gì, không đòi `RiskAcceptance`. `risk_outcomes` được điền. Mặc định cho mọi tenant (`risk_policies.status=shadow` hoặc bản `rp/1` mặc định).
2. **`enforce`:** admin tenant bật (`SetRiskPolicy` đặt `active`) khi đạt: ≥ 30 đánh giá có `risk_outcomes`; tỉ lệ khớp mức dự kiến với mức thực tế ≥ 70%; tỉ lệ đánh giá `partial` hoặc `confidence=low` ≤ 30%; không có phát hiện sai loại "Nghiêm trọng" cho thay đổi đã hoàn tất không sự cố ở > 10% trường hợp. Các ngưỡng này là đề xuất, chưa kiểm chứng. Có thể đặt `enforce` theo mức: chỉ chặn từ Cao trở lên.

Hai dialect: toàn bộ bảng ở 2.1 có migration cho Postgres và MySQL (≥ 8.0.1 cho `SKIP LOCKED` ở vòng quét lease); CHECK như ở CR-REQ-009; JSON dùng `JSONB`/`JSON` không DEFAULT literal.

### 2.10 Sự kiện, mã lỗi, cấu hình
Sự kiện (outbox `orca.request.<entity>.<event>`): `orca.request.impact.assessed` (khớp CR-REQ-036) `{assessment_id, request_id, subject_type, subject_id, level, score, confidence, mode}`; `orca.request.impact.drift_detected`; `orca.request.risk.accepted` `{assessment_id, finding_id, accepted_by}` (không chứa `reason`); `orca.request.risk_policy.changed`.

Lỗi: `REQUEST_RISK_ASSESSMENT_PENDING`, `REQUEST_RISK_ACCEPTANCE_REQUIRED`, `REQUEST_RISK_ASSESSMENT_STALE` (digest không khớp), `REQUEST_RISK_APPROVER_NOT_ALLOWED`, `REQUEST_PLAN_RISK_TOO_HIGH`, `REQUEST_RISK_OVERRIDE_REASON_REQUIRED`, `REQUEST_RISK_POLICY_INVALID` (InvalidArgument), `REQUEST_IMPACT_NO_CONNECTION` (cùng tinh thần `REQUEST_SOLUTION_NO_CONNECTION`). FailedPrecondition trừ khi ghi khác.

Cấu hình (đề xuất, chưa đo): `REQUEST_IMPACT_ENABLED=false`, `REQUEST_IMPACT_BUDGET=10m`, `REQUEST_IMPACT_INDEX_MAX_COMMITS_BEHIND=30`, `REQUEST_RISK_DRIFT_DELTA=15`, `REQUEST_IMPACT_MAX_SYMBOLS=20`.
## 3. Quyết định thiết kế
| Quyết định | Lý do |
|---|---|
| Điểm do quy tắc có phiên bản, AI chỉ diễn giải, `narrative` ngoài `digest` | Tái lập, kiểm toán; đổi lời diễn giải không làm mất hiệu lực Approval |
| Mức tổng = 60% chiều cao nhất + 40% trung bình có trọng số | Chiều nguy hiểm không bị che; không phải trung bình đơn giản |
| Chiều không đo được tính 50 và tăng bất định | "Không đo được" không được hiểu là "thấp" |
| Chạy `buf breaking` trực tiếp, không qua `make proto-lint` | `A && B || true` nuốt mọi lỗi; kết quả phải phân tích được |
| Bộ thu thập không tự `analyze` lại index | Chưa đo chi phí; báo lỗi thời thay vì ngầm tốn thời gian |
| `RiskAcceptance` gắn `assessment_digest` | Đánh giá đổi thì chấp nhận cũ mất hiệu lực, cùng cơ chế `subject_digest` của Approval |
| Lệch kế hoạch dùng Approval `phase` với `stage=drift_review` | Không thêm `subject_type` mới vào CHECK của CR-REQ-009 |
| Shadow trước, enforce sau | Ngưỡng là ước lượng; chặn sai làm hỏng niềm tin vào luồng |
| Độ phủ theo package | Công cụ hiện có không cho độ phủ theo symbol; ghi rõ giới hạn |
## 4. Tiêu chí chấp nhận
- [ ] Migration hai dialect up/down sạch; CHECK từ chối giá trị lạ; chỉ một `collecting` cho mỗi chủ thể; `UNIQUE` của `risk_acceptances` hoạt động.
- [ ] `Score` cùng đầu vào cho cùng `score`, `level`, `digest` qua 1000 lần; golden test cho `rp/1` với ≥ 6 ca (từng luật cứng, chiều không đo được, index lỗi thời).
- [ ] Mỗi luật cứng 2.4.1 nâng đúng mức tối thiểu kể cả khi điểm thấp.
- [ ] `buf breaking` thật phát hiện một thay đổi phá vỡ tạo cho test (xoá một trường proto mẫu) và đánh `Cao`; kết quả không bị `|| true` che.
- [ ] `MigrationScanner` phát hiện: thiếu down, một dialect, `DROP COLUMN`, `UPDATE` không `WHERE`, `NOT NULL` không default (mẫu dương và âm).
- [ ] Công cụ lỗi hoặc hết thời gian cho `status=partial`, chiều liên quan `measured=false`, `confidence` hạ; không có bản đánh giá `ready` nào im lặng bỏ chiều.
- [ ] Index lỗi thời được phát hiện từ `gitnexus.json` và tăng một bậc kèm nhãn "chưa đánh giá được".
- [ ] Đánh giá `solution_option` chạy sau `solution.proposed`; `plan`/`phase`/`task` sau COMMIT; `actual_*` sau `VerifyExecution`; mỗi bản có `impact_tool_runs` làm bằng chứng.
- [ ] `enforce`: `Approve` thiếu `RiskAcceptance` cho phát hiện Cao bị `REQUEST_RISK_ACCEPTANCE_REQUIRED`; đánh giá đổi digest làm chấp nhận cũ mất hiệu lực; `collecting` thì `REQUEST_RISK_ASSESSMENT_PENDING`; người ngoài `team` yêu cầu bị `REQUEST_RISK_APPROVER_NOT_ALLOWED`.
- [ ] `shadow`: không chặn gì, không đòi chấp nhận, nhãn "tham khảo" có trong dữ liệu trả về (`mode`).
- [ ] Nghiêm trọng: Plan có Phase mức Nghiêm trọng bị `REQUEST_PLAN_RISK_TOO_HIGH`; thiếu người duyệt thứ hai không qua.
- [ ] Lệch vượt `REQUEST_RISK_DRIFT_DELTA` mở Approval `stage=drift_review`, chặn `AdvanceExecution`; `OnRejected` đưa Request về backlog stage `phase`.
- [ ] `GetImpactGraph` mọi lens trả đúng hợp đồng node, cạnh; `id` ổn định giữa hai lần gọi; vượt `max_nodes` thì có `collapsed` và `truncated=true`.
- [ ] Narrative chứa `score` hoặc `level` bị loại; lỗi AI không làm bản đánh giá `failed`.
- [ ] `parity_test.go` xanh sau khi thêm kênh (có `ToolSpec` hoặc dòng loại trừ); không file tên `helpers`, `utils`, `common`, `misc`; không `max-lines` disable.
## 5. Kiểm thử
- **Unit:** `Score` và luật cứng (bảng, thuộc tính, golden); `AreaResolver`; `MigrationScanner`, `ContractScanner`, `GitNexusOutputParser` (mẫu thật lưu làm fixture); so lệch kế hoạch; ghép đồ thị từng lens; kiểm narrative.
- **Integration, cả hai dialect:** repository `impact_assessments` (append-only, lease, một `collecting`), `risk_acceptances` (hiệu lực theo digest), `risk_policies` (một `active`); thu thập với fake `AgentRelay` trả đầu ra công cụ (thành công, timeout, lỗi parse); `RiskGate` ghép với `Approve` của CR-REQ-009.
- **Hợp đồng:** `buf breaking`/`lint` cho proto mới; JSON Schema của đồ thị có mẫu dùng chung với frontend (CR-REQ-018 kiểu TypeScript).
- **Thủ công, có dev server (chưa kiểm chứng):** chạy `impact`, `detect-changes`, `buf breaking` thật trong worktree `task/<id>` và qua SSH; đo thời gian trên repo 248 nghìn symbol; giai đoạn 0 của mục 2.9.
- Chưa chạy test nào ở thời điểm viết CR.
## 6. Rủi ro và điểm chưa kiểm chứng
- GitNexus và CodeGraph có thể không thấy tác động xuyên ranh giới gRPC, kênh WS, sự kiện outbox; chiều Phạm vi ảnh hưởng có thể bị đánh giá thấp. `ContractScanner` chỉ bù một phần bằng regex.
- Định dạng đầu ra của GitNexus CLI chưa kiểm chứng (không thấy `--json`); `GitNexusOutputParser` phụ thuộc định dạng văn bản, dễ gãy khi đổi phiên bản. Có thể phải dùng MCP của GitNexus (`mcp__gitnexus__impact`) thay CLI qua CR-REQ-031.
- Index nằm ở thư mục gốc repo; worktree `task/<id>` có dùng chung index hay không chưa kiểm chứng. `detect-changes --base-ref` ánh xạ hunk sang symbol của index cũ nên có thể bỏ sót symbol mới.
- Thời gian truy vấn trên repo cỡ 248 nghìn symbol và giới hạn `agent.exec` 5 phút chưa đo; 20 symbol × `impact` có thể vượt ngân sách.
- Ngưỡng, trọng số, `N=3` service, độ lệch 15 điểm đều là ước lượng; chấm sai ở giai đoạn đầu là dự kiến, vì vậy phải qua `shadow`.
- Đánh giá Option dựa vào `affected_areas` do AI nêu; chính nó có thể sai. Đánh giá thực tế sau chạy đáng tin hơn nhưng đến muộn.
- `buf breaking --against '<.git>#ref=<sha>'` cần `ref` có trong clone của dev server; clone nông (`depth`) làm hỏng. Chưa thử SSH.
- Tăng tải: mỗi Option và mỗi task có một lần thu thập; Plan 100 task có thể tạo hàng trăm lần chạy công cụ. Cần dùng lại kết quả theo `(base_sha, path_set)`; chưa thiết kế bộ nhớ đệm.
- Đồ thị Kiến trúc cần danh mục service (CR-REQ-031 mục dữ liệu còn thiếu); khi chưa có thì lens này chỉ hiện phần diff, không có nền.
## 7. Câu hỏi mở
- **Q1.** Hai người duyệt cho mức Nghiêm trọng cần `required_approvals` và bảng quyết định ở CR-REQ-009 (hiện một `decided_by`). Đề xuất mở rộng CR-REQ-009, hay tạo hai Approval nối tiếp cùng chủ thể (cần nới chỉ mục "một `pending` mỗi chủ thể")?
- **Q2.** `risk_approver_team` và `risk_override_team` lấy từ đâu (CR-REQ-010 chỉ có `team:<id>` và `admin|user`)? Đề xuất cấu hình trong `RiskPolicy.gate_mapping`.
- **Q3.** Nhãn mới `gate:feature_flag` và `check:rollback_rehearsal` cần vào bảng nhãn của CR-REQ-012 (`plan_labels.go`); Orca không có bước deploy nên "thử quay lui" chỉ là task agent chạy trong môi trường người dùng chỉ định. Có chấp nhận?
- **Q4.** Ai khai `incident` và `rolled_back` cho `risk_outcomes` ở v1 (người dùng tay)? Có nguồn tự động từ CR-REQ-024 không?
## 8. Tham chiếu
- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.5, 3.7, 6, 8 (điều chỉnh 13, 14)
- `/opt/repos/orca/docs/research/receive-request/impact-assessment-and-risk-scoring.md`, `frontend-visualization-and-ux.md` (mục 4, 5), `artifact-formats-ontology-and-execution-readiness.md`
- `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md` (`affected_areas`, `analysis_runs`), `approval/CR-REQ-009-generic-approval-domain-and-api.md` (`subject_digest`, `UpdatePendingDigest`), `approval/CR-REQ-010-approval-authorization-notification-expiry.md`, `plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md`, `CR-REQ-013-phase-execution-and-feedback-loop.md`, `CR-REQ-014-type-specific-execution-policies.md`, `execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md`
- `/opt/repos/orca/docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md`
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/backend-go/Makefile` (dòng 80 đến 81), `/opt/repos/orca/.github/workflows/backend-go-mcp-service.yml`, `/opt/repos/orca/backend-go/ci/`, `/opt/repos/orca/backend-go/policy/orca-authz/`
- `/opt/repos/orca/.gitnexus/gitnexus.json`, `node .gitnexus/run.cjs --help|status|impact --help|detect-changes --help`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`, `parity_test.go`; `wscompat/channels_*.go`
- `/opt/repos/orca/AGENTS.md` (SSH, đa nền tảng, `max-lines`, Git compatibility), `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `request-service/internal/domain/impact_assessment.go`, `risk_acceptance.go`, `risk_policy.go`, `risk_rules_v1.go`, `risk_scoring.go`, `impact_graph.go`; `internal/usecase/collect_impact.go`, `risk_gate.go`, `assess_actual_impact.go`, `get_impact_graph.go`, `accept_risk.go`, `impact_narrator.go`; `internal/adapter/migrationscan/`, `contractscan/`, `gitnexusparse/`; `internal/adapter/{postgres,mysql}/impact_repository.go`, `risk_policy_repository.go`; migration `impact_risk` hai dialect
## 9. Tác động tới CR hiện có (không sửa trong CR này; người duyệt series áp dụng)
| CR | Mục | Cần sửa gì |
|---|---|---|
| CR-REQ-007 | 2.3 schema `options` | `affected_areas[].name` dùng từ vựng ánh xạ được (`service|module|api|schema|ui|infra` kèm đường dẫn hoặc tên service chuẩn); sau `solution.proposed` kích đánh giá Option; thêm khối `impact` tóm tắt (`level`, `score`, `assessment_id`) vào dữ liệu trả về, không ghi vào `options` JSON |
| CR-REQ-009 | 2.4 `Approve`, bảng `approvals` | Thêm cổng `ApprovalGuard` có thể ghép (cho `RiskGate`); `DecideApprovalRequest` thêm `viewed_impact_digest`, `accepted_finding_ids[]`; đề xuất `required_approvals` và bảng `approval_decisions` cho hai người duyệt (Q1); digest chủ thể `solution` và `plan` gồm `assessment.digest`; `stage=drift_review` hợp lệ cho `subject_type=phase` |
| CR-REQ-010 | `ApproverPolicy` | Thêm khoá chính sách theo mức rủi ro: `min_level → approver team`, `required_approvals`; không đổi bảng vai trò `admin|user` |
| CR-REQ-012 | 2.1 bảng nhãn, 2.3 COMMIT, 2.4 | Thêm nhãn `gate:feature_flag`, `check:rollback_rehearsal`; COMMIT bước 3 phát sự kiện kích đánh giá Plan; `PlanPreconditions` thêm `REQUEST_PLAN_RISK_TOO_HIGH`; digest Plan gồm `assessment.digest` |
| CR-REQ-013 | 2.4 `AdvanceExecution` | Thêm kiểm `RiskGate` (đánh giá `drift` chưa giải quyết) cạnh `PreExecutionGate`; 2.5 bước sau `VerifyExecution`: gọi `AssessActual`; bảng `task_run_outcomes` thêm `actual_assessment_id` NULL |
| CR-REQ-014 | 2.1, 2.2 | `PreExecutionGate` và `PlanPreconditions` nhận thêm điều kiện theo mức rủi ro (mục 2.6); cổng `pre_deploy` bắt buộc cho mọi loại từ mức Cao |
| CR-REQ-016 | danh sách kênh | Thêm các kênh `impact.*`, `risk.*` ở 2.8; kèm `ToolSpec` hoặc dòng trong `excluded_channels.yaml` |
| CR-REQ-020, 021, 022 | UI | Thẻ tóm tắt rủi ro, bảng so sánh Option theo chiều (`impact.compare`), bản đồ thay đổi và danh sách phát hiện (`impact.graph`, `impact.findings`), hộp xác nhận chấp nhận từng phát hiện Cao (`risk.accept`), biểu đồ nhiệt Phase (`impact.heatmap`), dải cảnh báo lệch; dùng token màu hiện có, kiểm token cho Cao và Nghiêm trọng (chưa đọc `main.css`) |
| CR-REQ-024, 035 | audit, quyền | `AppendDetailed` nhận `target_type=assessment|policy`; ghi override và đổi `RiskPolicy`; `request.rego` thêm hành động `impact.*`, `risk.*` (đọc theo quyền đọc Request, `risk.policy.set` chỉ admin) |
| CR-REQ-032 | 2.1 `GraphPayload` | Xác nhận khớp mục 2.8 của CR này; `impact.graph` nhận `{requestId, subjectType, subjectId, lens, base?}`; lens backend `architecture|contract|data|impact` |
| CR-REQ-034 | bước AI | Thêm bước `impact_narrative` vào `PromptRegistry` và chính sách ngân sách |
| CR-REQ-036 | 2.5, 2.7, kênh | Xác nhận kênh `impact.get|graph|accept|drift`, trường `rationale`, sự kiện `impact.assessed`, `impact.drift_detected` khớp mục 2.8 và 2.10 |
| CR-REQ-025 | rollout, e2e | Cờ `REQUEST_IMPACT_ENABLED`; giai đoạn `shadow` là mặc định ở rollout; e2e: mỗi mức rủi ro một Request |
| CR-REQ-029 | 2.6 | `VerifyExecution` phát sự kiện hoặc gọi trực tiếp `AssessActual` sau khi đạt |
| `Makefile` (backend-go) | dòng 81 | Ngoài phạm vi series này, đề nghị riêng: bỏ `|| true`, chạy `buf breaking` thật trong CI PR (hiện không có) |
