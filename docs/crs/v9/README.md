# Change Requests v9 — Outcome Review (review mức kết quả cho Request)

> **Mục tiêu:** khi một Request đã chạy xong, người duyệt thấy **ở mức hệ thống** (không phải từng dòng code): yêu cầu nào đã giao, kiến trúc có lệch thiết kế không, hành vi/dữ liệu/hợp đồng đổi gì (dạng diff trước và sau), rủi ro phát hành, và những quyết định còn chờ họ. Ngữ cảnh thiết kế (SRS, SAD, TDD) lấy từ **tdd knowledge graph**; hình vẽ do skill [`diagram-design`](https://github.com/cathrynlavery/diagram-design) sinh, **lưu lại để không sinh lại** (tiết kiệm token) và **cho phép sinh lại** khi cần. Review có **hai pha**: *trước code* (kiến trúc, luồng, hành vi, dữ liệu đã thiết kế, xem được ngay cả khi Execution chưa chạy hay chưa xong) và *sau khi Execution xong* (thêm phần thay đổi sau code).
>
> **Quan hệ với v7:** v7 là review **mức code** (symbol, graph, diff dòng). v9 là cổng **mức kết quả** nằm trên v7; từ mỗi mục của v9 có liên kết "Mở trong review code" (CR-OR-022). v7 **đã có mã** (mục 1: `code-intel-service`, `components/review-map/`) dù tài liệu v7 vẫn ghi "đề xuất". v9 **không phụ thuộc** v7 để chạy và bật riêng bằng cờ của nó; khi v7 bật (cổng chất lượng CR-CV-085, overlay CR-CV-036) thì v9 dùng thêm làm dữ kiện tuỳ chọn và **không dựng song song** những gì v7 đã có (mục 2, D12).
>
> **Phạm vi:** `backend-go/services/request-service` (mô hình, kho artifact, RPC), `api-gateway` (kênh `request.review.*`), agent (một lượt chạy chỉ-đọc với hai skill), `frontend/src/renderer/src/components/request/outcome-review/` (mới), `skills/diagram-design/` (bản ghim). Không đổi máy trạng thái Request ở MVP.

> **Trạng thái toàn series:** 📝 Đề xuất, chưa triển khai (soạn 2026-10-20). Phần "Hiện trạng đã xác nhận" bên dưới là đọc code ngày 2026-10-20; mọi chỗ ghi "chưa kiểm chứng" phải được đọc lại trước khi sửa.

## 1. Hiện trạng đã xác nhận (đọc code 2026-10-20)

| Hạng mục | Thật sự có gì | Bằng chứng |
|---|---|---|
| Trạng thái Request | `new, classifying, awaiting_type_confirmation, analyzing, awaiting_analysis_approval, planning, awaiting_plan_approval, executing, completed, request_backlog, cancelled, awaiting_information`. **Không có trạng thái "chờ review kết quả"**; `completed` là cuối | `backend-go/services/request-service/internal/domain/request_status.go` |
| Tiêu chí chấp nhận và độ phủ | `GetRequestCoverage` trả hàng `ac_id → task_id → check_id` và `uncovered_ac_ids`; có `acceptance_criteria.go` | `proto/orca/request/v1/artifact.proto`, `domain/acceptance_criteria.go` |
| Mô hình artifact tài liệu | 8 loại tài liệu có JSON Schema (`request, solution, diagnosis, findings, answer, plan, phase, task`), trần kích thước theo loại, `ExportArtifactProjection` (Markdown, ≤ 1 MB). **Không phải** nơi chứa hình vẽ | `domain/artifact_kind.go`, `artifact.proto` |
| Đánh giá tác động và rủi ro | `ImpactAssessmentSummary` (level `low…critical`, `dimensions[]`, `digest`, `narrative_json`, `index_age_seconds`, mode `shadow|enforce`); ba thời điểm gồm "thực tế sau chạy" (v6 R2) | `proto/orca/request/v1/impact.proto`, `docs/crs/v6/impact-risk/README.md` |
| Tham chiếu thiết kế từ KG | `DesignReferences{available, snapshot, read[], gaps[], conflicts[]}`: dữ liệu để người xem, **không phải cổng**; thiếu KG là bình thường (`available=false`) | `domain/solution_design_references.go` |
| Orca không đọc KG | Chỉ agent chạy trên dev server gọi MCP `tdd-agent`; Orca chỉ kiểm tệp `.tdd-agent/config.toml` có mặt. Skill cài bằng `npx skills add https://github.com/vnp-community/orca --skill …` từ thư mục `skills/` | `docs/guides/request/knowledge-hub-setup.md` |
| Skill đã có trong repo | `skills/tdd-knowledge`, `tdd-knowledge-writeback` (và `tdd-openspec-author`, `tdd-orca-task` theo hướng dẫn); **chưa có `diagram-design`** | `ls skills/` |
| Tool đọc KG của skill | Chỉ-đọc: `kg_search`, `kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `source_get`; kết quả mang `snapshot`; id không có thì không được bịa | `skills/tdd-knowledge/SKILL.md` |
| Đường chạy agent có kết quả có cấu trúc | Task của Plan OpenSpec kết thúc bằng khối kết quả có nonce (`status`, `checks_run`, `knowledge`, `writeback`); task có `Verify:` chỉ xong khi có kiểm tra thoát mã 0 | `docs/guides/request/knowledge-hub-setup.md` |
| Màn hình Request hiện có | Thư mục `components/request/` với `execution/`, `activity/`, `plan/`, `approval/`, `RequestStageTimeline.tsx`, `NextActionBar.tsx`, `SpawnChildRequestDialog.tsx`; chưa có thư mục `review/` | `ls frontend/src/renderer/src/components/request` |
| Migration | Mới nhất `0104_plan_auto_jobs_revision_origin` ở `migrations/postgres` (handoff 2026-10-17 còn ghi 0098: đã lỗi thời) | `ls migrations/postgres` |
| v7 đã có mã (backend) | `backend-go/services/code-intel-service` (usecase `get_architecture`, `get_change_overlay`, `get_contract_diff`, `build_erd`, `bind_repo`, bộ đệm snapshot có ETag SHA-256, giới hạn 3 MiB mặc định mỗi payload / 64 MiB mỗi binding / 512 MiB mỗi tenant, janitor, `secret_redactor`); có cả `quality-service`. Khoá theo **worktree/repo binding**, không theo Request | `ls backend-go/services`, `code-intel-service/README.md` |
| v7 đã có mã (frontend) | `components/review-map/` với `impact`, `c4`, `dataflow`, `erd`, `findings`, `quality`, `requirements` (`RequirementTracePanel`), `reading-order`, `notes`, `report` (`ReviewReportMenu`, chèn báo cáo vào mô tả PR, bộ lọc Mermaid `review-report-diagram-guard.ts`), `ai-summary`, `entry` (`openReviewFromEntryPoint(worktreeId, source, {lens, filter})`, chỉ khi `codeIntelSupportState.state === 'enabled'`) | `ls frontend/src/renderer/src/components/review-map`, `entry/open-review-entry.ts` |
| Chồng chéo cần tránh | (a) lens **Requirements** của v7 nối thay đổi của worktree với Task/yêu cầu; (b) **báo cáo xuất** của v7 dựng Markdown + Mermaid cho mô tả PR; (c) bộ đệm snapshot của v7 đã có mẫu ETag/hạn mức. v9 khác ở chỗ: khoá theo **Request**, có tầng KG (SRS/SAD/TDD), và hình do skill vẽ lưu lại; không thay thế (a) hay (b) | so sánh mã nêu trên với CR-OR-020/021/022 |
| Skill `diagram-design` (bên ngoài) | MIT, phiên bản 2.6; sinh một tệp HTML + SVG tĩnh, không JS; màu theo vai trò (paper, ink, muted, accent, link); có 44–45 loại (architecture, architecture delta, sequence, ER, database schema, state machine, swimlane, treemap, dependency graph, data flow, …); có cổng "style guide" hỏi người dùng ở lần đầu; `scripts/self_check.py`; xuất PNG cần Playwright; chỉ Google Fonts là tài nguyên ngoài | trang repo và `SKILL.md` (đọc qua web 2026-10-20; chưa đọc `references/`) |

## 2. Quyết định đã đưa ra

| # | Quyết định | Lý do / hệ quả |
|---|---|---|
| D1 | **Review mức kết quả gắn với Request, không thêm trạng thái Request** ở MVP. Bản ghi `OutcomeReview` có hai pha (D17) và mở được từ khi Request có Plan (`awaiting_plan_approval`) đến `completed` | Tránh đổi máy trạng thái (rủi ro cao); pha thiết kế cho kiểm soát kiến trúc trước khi code. Cổng chặn "phải duyệt review mới completed" để CR sau |
| D2 | **Tách bốn tầng dữ liệu**: F (dữ kiện xác định, Orca tính), C (ngữ cảnh KG, do agent đọc, gắn nhãn), D (mô hình sơ đồ, sắp xếp xác định), A (hình vẽ, artifact) | Chỉ F mới được dùng để kết luận; C và A không bao giờ làm căn cứ chặn (đồng nhất v7 F7) |
| D3 | **Orca không đọc KG.** Một *review run* chỉ-đọc trên dev server dùng skill `tdd-knowledge` và trả `ReviewContext` có cấu trúc, mọi mục kèm bằng chứng | Khớp quyết định hiện có (hướng dẫn knowledge hub); thiếu KG thì review vẫn có tầng F |
| D4 | **Hình vẽ do skill `diagram-design` sinh trên agent**, một bản **ghim phiên bản** vendor vào `skills/diagram-design/` (kèm LICENSE MIT) | Tái lập; skill có script chạy được, không kéo bản mới tự động (chuỗi cung ứng) |
| D5 | **Artifact lưu theo nội dung**: khoá = `(section, input_digest của mô hình sơ đồ, preset_digest, skill_version)`. Không đổi mô hình sơ đồ thì **không vẽ lại**, kể cả khi head commit đổi hay review được dựng lại | Tiết kiệm token là yêu cầu chính |
| D6 | **Màu theo token lúc hiển thị**, không nướng hex vào artifact: skill dùng vai trò màu, trình xem điền giá trị token của Orca (sáng/tối) | Một artifact cho cả hai giao diện, không vẽ lại khi đổi theme (cần spike, CR-OR-010) |
| D7 | **Sinh lại là hành động của người dùng**, có phạm vi (một sơ đồ, một mục, cả review), có xác nhận chi phí, giữ bản cũ thành revision | Không tự sinh lại; so sánh được |
| D8 | **Tuỳ chỉnh skill vẽ qua `ReviewDrawingProfile`** (loại sơ đồ theo mục, biến thể, mức chi tiết, token). MVP chỉ có preset `orca-default` chỉ-đọc; tuỳ chỉnh đầy đủ ở CR-OR-013 (P1) | Mặc định tốt nhất trước, khoá lược đồ để sau mở |
| D9 | **Artifact do agent sinh là dữ liệu không tin cậy**: hiển thị trong iframe `sandbox` không script, CSP chặt, lọc trước khi lưu | HTML từ LLM có thể chứa script hoặc nội dung chèn từ repo |
| D10 | **"Gửi lại agent" = tạo Request con** (cơ chế `SpawnChildRequest` hiện có) mang theo các mục bị trả về; không mở lại Request đã `completed` | Không cần chuyển trạng thái ngược |
| D11 | **Cờ theo project**, mặc định tắt: `outcome_review = off|manual` (và `auto` ở P2); mặc định triển khai qua `REQUEST_DEFAULT_OUTCOME_REVIEW` (cùng mẫu `REQUEST_DEFAULT_ENGINE`) | Mẫu hiện có; chi phí AI nên không bật mặc định |
| D12 | **Tái dùng, không dựng lại những gì v7 đã có**: lối vào review code qua `openReviewFromEntryPoint`; bộ lọc/ý tưởng của `review-report-diagram-guard.ts` cho artifact; mẫu ETag + hạn mức của bộ đệm snapshot; lens Requirements của v7 vẫn là nơi xem truy vết mức worktree. v9 là cổng mức Request, khoá theo Request | Tránh hai hệ thống review song song; v7 đã có mã (mục 1) |
| D13 | **Review tập trung vào kiểm soát kiến trúc và kiểm soát luồng ở mức C4 level 2 (container).** Architecture là sơ đồ container; Flow là sơ đồ động (dynamic) giữa các container. Thay đổi bên trong một container (level 3) **không** thành nút ở v9: hiện bằng huy hiệu "n components changed" kèm lối vào lens `architecture` của v7 | Người duyệt cần thấy ranh giới giữa service, giao thức, hướng phụ thuộc và thứ tự gọi; chi tiết component đã có ở v7 |
| D14 | **Kiến trúc và luồng là hạng mục kiểm soát hạng nhất**: có thẻ kết luận riêng, được so với thiết kế (SAD/ADR trong KG) khi có, mỗi lệch là một mục `needsDecision` | Mục tiêu của người dùng; tránh chôn trong bảng chung |
| D15 | **Thành phần của hệ thống (container, cạnh phụ thuộc, luồng thiết kế) được đọc từ tdd knowledge, Orca không giữ danh mục riêng.** Nguồn gồm phần tử KG (`Component`, `Constraint` mã `CT-*`, `Decision`) **và** văn bản SAD qua `source_get` (AM-1). Mỗi nút/làn mang `source: kg\|sad-text\|code`; thành phần chỉ có trong văn bản SAD (datastore, hệ ngoài) là `sad-text`, chỉ thành phần không có ở KG lẫn SAD mới là `unmapped` và tự nó là một phát hiện ("Not in design") (AM-2) | Nguồn sự thật duy nhất là tài liệu thiết kế; KG vắng thì không đối chiếu được (nói rõ, không bịa); đo trên KG thật của `vnp-qa-platform` cho thấy KG không chứa datastore, hệ ngoài hay cạnh giữa container |
| D16 | **Đối chiếu trước code và sau code**: Architecture và Flow có hai góc nhìn, *Design* (đã thiết kế trong KG, trước khi code) và *After code* (thay đổi do code gây ra, so với base và với thiết kế); UI cho xem riêng từng góc nhìn hoặc cạnh nhau | Mục tiêu của người duyệt: kiểm soát kiến trúc và luồng bằng cách so thiết kế với kết quả |
| D17 | **Hai pha của review.** Pha `design` (trước code): có ngay khi Request có Plan, kể cả khi Execution chưa chạy hay chưa xong; chỉ hiện phần *Design* của Architecture, Flow, Behavior, Data và các yêu cầu/tiêu chí áp dụng, đọc từ KG, **không đọc diff**. Pha `delivered` (sau khi Execution xong): thêm phần *After code* (delta so với base và thiết kế), quyết định và duyệt. Hình `design` vẽ một lần ở pha thiết kế và **tái dùng** ở pha `delivered`; ngữ cảnh KG đã đọc cũng tái dùng | Yêu cầu của người dùng; phần Design rẻ vì không đọc code |
| D18 | **Tab trong màn Request tên `Outcome review`** (giá trị `outcome-review`), đứng **ngay sau Execution**; khác tab `Review` mức code của v7 trong tab group của workspace | Tránh nhầm hai tab cùng tên; vị trí cố định |

### Mặc định đề xuất cho điểm còn mở (cần xác nhận khi duyệt)

| # | Điểm mở | Mặc định trong series này |
|---|---|---|
| O1 | Nơi lưu blob artifact | **DB request-service** (cột văn bản, tách bảng blob theo digest) với trần nhỏ; chưa có xác nhận repo có object store nào. Đổi sau không phá hợp đồng vì truy cập qua `blob_digest` |
| O2 | Ai gọi vẽ | Một *draw run* tách khỏi *context run*; chỉ vẽ phần trượt cache; tối đa 1 lần sửa nếu `self_check` lỗi |
| O3 | Có tự chạy khi Request `completed` không | **Không** ở MVP (`manual`). `auto` là P2, cần hạn mức chi phí theo project |
| O4 | Biến thể skill | `minimal`, một tệp theo mục, không chuyển động, không kiểu phác thảo tay, không biểu tượng; `full editorial` không dùng ở MVP |
| O5 | Quyền | Đọc review: người xem Request; sinh/sinh lại: người có quyền chạy agent trên project; sửa profile: admin project. Tên hành động OPA chốt khi đọc `approval_authorization.go` |
| O6 | Ngôn ngữ nhãn trong hình | Tiếng Anh (nhãn UI theo quy ước dự án); tên thành phần lấy nguyên văn từ code/KG |

## 3. Hợp đồng chung (mọi CR phải dùng đúng tên này)

### 3.1 Kiến trúc

```
Frontend (tab Outcome review của Request) ─WS request.review.*─▶ api-gateway ─gRPC─▶ request-service
                                                                              │ F: dữ kiện xác định (DB + git diff qua agent.exec)
                                                                              │ C: context run ──agent.execPrompt──▶ agent (dev server, chỉ-đọc)
                                                                              │                                     └ skill tdd-knowledge ─MCP─▶ tdd-agent KG
                                                                              │ A: draw run ─────agent.execPrompt──▶ agent
                                                                              │                                     └ skill diagram-design (ghim)
                                                                              └ kho artifact (blob băm nội dung, theo tenant)
```

### 3.2 Mô hình `OutcomeReview` (JSON; proto `outcome_review.proto`, message tiền tố `OutcomeReview*`)

```jsonc
{ "schemaVersion": 1, "reviewId": "…", "requestId": "…",
  "phase": "design|delivered",                    // design: chưa có diff; subject.heads rỗng
  "subject": { "baseRef": "…", "baseCommit": "…", "headCommit": "… (head_key khi nhiều nhánh)", "heads": [ { "branch": "…", "commit": "…", "taskId": "…?" } ], "engine": "native|openspec", "executionMode": "continuous|step_by_step" },
  "inputsDigest": "<sha256 canonical JSON, không có thời gian>", "state": "draft|ready|stale|approved|superseded",
  "verdict": {
    "requirements": { "delivered": 0, "partial": 0, "notCovered": 0, "unknown": 0 },       // F (+C nếu có KG, gắn nhãn)
    "architecture": { "containersChanged": 0, "newEdges": 0, "removedEdges": 0, "unplannedEdges": 0,
                      "reportedDeviations": 0, "withEvidence": 0, "advisory": 0, "designChecked": false, "reviewedRatio": 0.0 },   // C, không phải F; designChecked=true khi có KG/SAD để đối chiếu; reviewedRatio = tỉ lệ phần tử thiết kế đã duyệt (AM-3); advisory = lệch dựa trên phần tử chưa duyệt
    "flow":         { "newCalls": 0, "removedCalls": 0, "reordered": 0, "unplannedCalls": 0,
                      "reportedDeviations": 0, "withEvidence": 0, "advisory": 0, "designChecked": false, "reviewedRatio": 0.0 },   // C, không phải F
    "quality":      { "gate": "pass|warn|fail|unknown|absent" },                              // v7 nếu có
    "release":      { "risk": "low|medium|high|critical|unknown", "flags": ["migration","breaking","config"] } },  // F
  "items": [ ReviewItem ],                  // mục theo từng tab; xem 3.3
  "context": { "kg": { "available": true, "snapshot": "…", "gaps": [], "conflicts": [] } },  // như DesignReferences
  "diagrams": [ { "section": "architecture|flow|behavior|data", "type": "architecture-delta|sequence|state-machine|database-schema", "variant": "design|delta",
                  "inputDigest": "…", "artifactId": "…|null", "status": "cached|missing|drawing|failed" } ],
  "warnings": [ "kg_unavailable", "index_stale", "draw_failed:flow", "partial_facts" ] }
```

### 3.3 `ReviewItem`

```jsonc
{ "key": "<sha256(section|kind|subject)[:16], ổn định giữa các lần dựng>",
  "section": "architecture|flow|behavior|data|contracts|requirements|config",
  "change": "added|modified|removed|unchanged",
  "containers": [ "COMP-12", "COMP-03" ],                 // id phần tử KG (`kgId`) của container mà mục nói tới, `sad:<mã>` (chỉ có trong văn bản SAD, ví dụ `sad:DS-01`) hoặc `code:<tên>` (unmapped); bắt buộc ở section architecture|flow
  "protocol": "grpc|ws|http|sql|nats|mcp|fs|…",                 // giao thức của cạnh/tin nhắn (architecture|flow)
  "source": "kg|sad-text|code",                                   // nguồn của thành phần/cạnh (AM-2); chỉ `code` mới là unmapped
  "reviewState": "approved|needs_review|unknown",                 // trạng thái rà soát của phần tử KG làm căn cứ thiết kế (AM-3)
  "advisory": false,                                             // true khi lệch chỉ dựa trên phần tử chưa duyệt: không thành needsDecision (AM-3)
  "expected": true,                                              // cạnh/lời gọi có trong thiết kế (SAD/ADR) không; null khi không có thiết kế để đối chiếu
  "origin": "fact|agent",                        // fact: Orca tính; agent: lời agent từ KG/đọc code
  "title": "…≤160", "detail": "…≤400",
  "severity": "info|warning|breaking|deviation|regression",
  "needsDecision": false,
  "evidence": [ { "kind": "kg|source|file|rpc|channel|migration|check", "ref": "ADR-007 | path | …", "snapshot": "…?", "doc": "SAD?", "section": "5.3?", "line": 0 } ],
  "before": "…?", "after": "…?" }                // dùng cho mục "yêu cầu" (trạng thái trên base và trên head)
```

Quy tắc: `origin=agent` mà `evidence[]` rỗng hoặc id không xác minh được → hiển thị "Chưa xác minh" và **không** tính vào `verdict.architecture.withEvidence`; Orca kiểm tra được phần `file` (tệp có trong diff) và định dạng id KG, không kiểm được nội dung KG.

Loại bằng chứng `source` (AM-1) trỏ vào văn bản tài liệu nguồn của KG: `{doc, section, line}`, dùng cho cạnh, luồng, datastore và hệ ngoài mà KG không có thành phần/cạnh tương ứng. Orca kiểm định dạng và việc tài liệu/mục đã được agent khai là đọc, không kiểm được nội dung.

### 3.4 Bảng (request-service, hai dialect, mọi bảng có `tenant_id`, không FK chéo service)

| Bảng | Trường chính |
|---|---|
| `outcome_reviews` | `id`, `tenant_id`, `request_id`, `head_commit`, `base_commit`, `inputs_digest`, `state`, `model` (JSON, có trần), `kg_snapshot`, `created_by`, `created_at`, `updated_at`, `version` |
| `outcome_review_decisions` | `id`, `tenant_id`, `review_id`, `item_key`, `decision` (`accept_exception|send_back|dismiss`), `note`, `decided_by`, `decided_at` |
| `outcome_review_runs` | `id`, `tenant_id`, `review_id`, `kind` (`context|draw`), `status`, `scope`, `agent_run_ref`, `cost_estimate`, `cost_actual`, `error_code`, `started_at`, `finished_at` |
| `review_artifacts` | `id`, `tenant_id`, `request_id`, `section`, `diagram_type`, `input_digest`, `preset_digest`, `skill_version`, `blob_digest`, `revision`, `state` (`active|superseded|failed`), `origin` (`auto|regenerated`), `reason`, `self_check` (JSON), `error_code`, `model` (JSON mô hình sơ đồ ≤ 32 KiB, để so sánh revision; theo CR-OR-011), `created_by`, `created_at` |
| `review_artifact_blobs` | `tenant_id`, `digest`, `media_type`, `size_bytes`, `content`, `created_at` — khoá chính `(tenant_id, digest)` |
| `review_drawing_profiles` | `id`, `tenant_id`, `project_id`, `name`, `spec` (JSON), `spec_digest`, `version`, `updated_by`, `updated_at` |

`digest` dùng cùng cách chuẩn hoá JSON của `domain/canonical_json_digest.go`. Số migration tiếp theo chốt khi triển khai (hiện mới nhất là 0104).

### 3.5 RPC (trong `RequestService`, theo quy ước `proto/orca/request/v1`)

`EstimateOutcomeReview`, `GenerateOutcomeReview`, `GetOutcomeReview`, `ListOutcomeReviews`, `GetReviewArtifact`, `ListReviewArtifactRevisions`, `RegenerateReviewArtifacts`, `RestoreReviewArtifactRevision`, `DecideOutcomeReviewItem`, `ApproveOutcomeReview`, `GetReviewDrawingProfile`, `SaveReviewDrawingProfile`. Message do CR sở hữu từng RPC định nghĩa; không khai báo RPC chưa có message.

Kênh gateway: `request.review.estimate|generate|get|list|artifact|revisions|regenerate|restore|decide|approve|profile.get|profile.save`; push: `request.review.changed`, `request.review.progress`. Tên chốt cuối ở CR-OR-020. Ghi vào danh sách kênh, MCP (`request_review_*`) và OPA `agent_rpcs` theo mẫu `request.activity`.

### 3.6 Mã lỗi (`REQUEST_REVIEW_*`, cùng kiểu `REQUEST_SPEC_*`, `REQUEST_ENGINE_*`)

`REQUEST_REVIEW_DISABLED`, `REQUEST_REVIEW_NOT_FOUND`, `REQUEST_REVIEW_NOT_READY`, `REQUEST_REVIEW_RUN_IN_PROGRESS`, `REQUEST_REVIEW_SKILL_MISSING`, `REQUEST_REVIEW_RESULT_INVALID`, `REQUEST_REVIEW_ARTIFACT_UNSAFE`, `REQUEST_REVIEW_ARTIFACT_TOO_LARGE`, `REQUEST_REVIEW_PROFILE_INVALID`, `REQUEST_REVIEW_BUDGET_EXCEEDED`, `REQUEST_REVIEW_STALE_REVISION`, `REQUEST_REVIEW_DECISIONS_OPEN`. `REQUEST_REVIEW_NOT_READY` khi Request ở trạng thái trước `awaiting_plan_approval` (chưa có Plan) hoặc đã `cancelled`. KG vắng **không** là lỗi: là cảnh báo `kg_unavailable`.

### 3.7 Sự kiện (outbox)

`orca.request.review.generated`, `orca.request.review.artifact_regenerated`, `orca.request.review.decided` (theo mẫu `domain/outbox_subjects.go`; chốt tên khi đọc).

### 3.8 Preset vẽ mặc định `orca-default` (MVP)

| Mục (tab) | Loại sơ đồ của skill | Ghi chú |
|---|---|---|
| Architecture | architecture delta, **mức container (C4 level 2)**, hai biến thể `design` (từ KG, trước code) và `delta` (sau code) | Nút = container lấy từ KG (`kgId`, `kind`, `tech`); cạnh = giao thức giữa container; thêm/sửa/xoá/lệch/thiếu; cạnh không có trong thiết kế tô bằng accent duy nhất; thay đổi bên trong container là huy hiệu, không phải nút |
| Flow | sequence dùng như **C4 dynamic mức container**, hai biến thể `design` và `delta` | Làn = container (≤ 6); tin nhắn = lời gọi giữa container, ghi giao thức; không có mũi tên tự gọi; `design` là luồng đã thiết kế, `delta` là luồng sau code đánh dấu theo base / head / thiết kế |
| Behavior | state machine, hai biến thể `design` và `delta` | Pha design: máy trạng thái đã thiết kế (nếu KG mô tả); pha delivered: chỉ khi hành vi đổi |
| Data | database schema, hai biến thể `design` (thực thể và thuộc tính từ KG) và `delta` (từ migration) | Cột thêm/xoá ở `delta` |
| Requirements, Contracts, Config | **không vẽ** (bảng) | Bảng dễ đọc và rẻ hơn. Pha design: Requirements hiện yêu cầu/tiêu chí áp dụng (trạng thái "Planned"); Contracts, Config chỉ có sau khi code |

Biến thể `minimal`, một accent cho 1–2 phần tử trọng tâm, ≤ 12 nút mỗi sơ đồ (vượt thì gộp theo cụm và ghi "+N"), ≤ 8 sơ đồ mỗi review (mỗi mục Architecture, Flow, Behavior, Data tối đa hai biến thể `design` và `delta`).

### 3.9 Mức C4 và nguồn thành phần

| Mức C4 | Trong v9 | Ở đâu |
|---|---|---|
| Level 1 (context) | Chỉ là dải hệ ngoài trong sơ đồ Architecture (`kind: external`: Jira, GitHub/GitLab, KG, nhà cung cấp AI); không có sơ đồ riêng ở MVP | v9 |
| **Level 2 (container)** | **Architecture và Flow**: ranh giới service, kho dữ liệu, ứng dụng, giao thức, hướng phụ thuộc, thứ tự gọi | **v9 (trọng tâm)** |
| Level 3 (component) | Huy hiệu "n components changed" trên container + lối vào lens `architecture` của v7 | v7 (CR-CV-033) |
| Level 4 (code) | Ngoài phạm vi; diff dòng ở Changes/review code | v7 và diff |

**Nguồn thành phần là tdd knowledge (KG và tài liệu nguồn của nó), không phải danh mục của Orca.** Orca không giữ danh mục container và không đọc KG (D3). Trong context run agent đọc, kèm `snapshot`: (1) phần tử KG lớp `Component` (ví dụ `SVC-01…08`), lớp `Constraint` mã `CT-*` (ví dụ gateway `CT-02`, SPA `CT-01`) và `Decision` (ADR); (2) văn bản tài liệu nguồn qua `source_get` (SAD: bảng container, bảng phụ thuộc, khối mermaid), nơi có datastore (`DS-*`), hệ ngoài (`EXT-*`), cạnh giữa container và luồng mà KG không có thành phần/cạnh tương ứng (AM-1). Quy tắc:

- **Nguồn nút/làn (AM-2):** `source = kg` (có phần tử KG, `kgId`), `sad-text` (chỉ có trong văn bản SAD: `kgId = null`, id `sad:<mã>`, hiển thị "In SAD text only"), `code` (chỉ thấy trong code: `unmapped`, id `code:<kebab(label)>`, hiển thị "Not in design"). `unmapped` **chỉ** dành cho `code`, tức không có ở cả KG lẫn SAD. Id nút theo `kgId` hoặc `sad:<mã>` nên ổn định giữa các lần sinh.
- **`kind` và mức C4 (AM-5)** do văn bản SAD/agent kết luận, không suy từ lớp KG: `SVC-BUG-09` và `SVC-JIR-13` cùng lớp `Component` với service nhưng là thành phần bên trong (level 3); `CT-02` thuộc lớp `Constraint` nhưng là container. Chưa có quy tắc cứng; kiểm trên dự án pilot (mục 8).
- **Trạng thái rà soát (AM-3):** mỗi phần tử KG đã đọc mang `reviewState`. Cạnh/luồng/lệch dựa trên phần tử chưa duyệt (`needs_review` hoặc `unknown`) là `advisory`: không thành `needsDecision`; UI hiện "Design not reviewed". `verdict.architecture|flow.reviewedRatio` = tỉ lệ phần tử thiết kế đã duyệt.
- **Khoá thiết kế (AM-4):** bản trích `design.*` được **khoá** theo `(kg.snapshot, sha256 tài liệu SAD)` và tái dùng giữa các lần sinh và giữa hai pha; khối mermaid trong SAD được **parse xác định ở Orca** (agent trả nguyên văn khối), agent chỉ bổ sung phần văn bản; khi hai bên lệch ưu tiên parser.
- KG vắng hoặc SAD không mô tả kiến trúc → chỉ có góc nhìn *After code* với mọi nút `code` (unmapped) và nhãn "Design not available"; không có góc nhìn *Design* và không có số `unplanned`.
- Brief chỉ gợi ý nhóm tệp đổi theo thư mục cấp một cho agent đọc; đó là gợi ý, không phải danh mục.

**Hai góc nhìn:** `design` (trước code, chỉ dựa vào KG, không phụ thuộc commit nên tái dùng giữa các lần sinh) và `delta` (sau code, đánh dấu base / head / thiết kế). Behavior và Data cũng có hai biến thể khi KG mô tả (máy trạng thái, thực thể); không có thì chỉ `delta`.

## 4. Danh sách feature và CR

| Feature (folder) | CR | Nội dung | Priority | Effort |
|---|---|---|---|---|
| [`outcome-review-core`](./outcome-review-core/README.md) | CR-OR-001 | Mô hình `OutcomeReview`, dữ kiện xác định (tầng F), bảng, RPC, trạng thái cũ/mới | 🔴 P0 | Large |
| | CR-OR-002 | Context run của agent: ngữ cảnh KG (tầng C), diff kiến trúc và luồng **mức container C4 level 2** so ba chiều base / head / thiết kế đọc từ KG, hai góc nhìn trước và sau code, bằng chứng, giới hạn | 🔴 P0 | Large |
| [`review-visuals`](./review-visuals/README.md) | CR-OR-010 | Pipeline vẽ bằng `diagram-design`: vendor ghim, preset mặc định, draw run, kiểm tra tự động | 🔴 P0 | Large |
| | CR-OR-011 | Kho artifact và cache theo nội dung, revision, dọn dẹp | 🔴 P0 | Medium |
| | CR-OR-012 | Sinh lại artifact: phạm vi, chi phí, bản cũ, so sánh | 🔴 P0 | Medium |
| | CR-OR-013 | Hồ sơ vẽ tuỳ chỉnh (`ReviewDrawingProfile`), kiểm hợp lệ, phạm vi tenant/project | 🟠 P1 | Medium |
| [`outcome-review-frontend`](./outcome-review-frontend/README.md) | CR-OR-020 | Tab **Outcome review** (ngay sau Execution) trong Request: **thẻ kiểm soát kiến trúc và luồng ở đầu tab**, thẻ kết luận, "cần bạn quyết định", yêu cầu giao so với hỏi, trạng thái | 🔴 P0 | Large |
| | CR-OR-021 | Tab diff mức hệ thống (Architecture, Flow, Behavior, Data, Contracts, Requirements, Config) và trình xem artifact an toàn | 🔴 P0 | Large |
| | CR-OR-022 | Quyết định, "gửi lại agent" bằng Request con, nối sang review code v7 | 🔴 P0 | Medium |
| [`outcome-review-rollout`](./outcome-review-rollout/README.md) | CR-OR-030 | Bảo mật artifact, kiểm thử, đo chi phí/cache, cờ và rollout | 🔴 P0 | Medium |

**Số lượng CR:** 10: `outcome-review-core` 2, `review-visuals` 4, `outcome-review-frontend` 3, `outcome-review-rollout` 1. Mốc MVP = CR-OR-001, 002, 010, 011, 012, 020, 021, 022, 030 (mọi P0); CR-OR-013 (tuỳ chỉnh hồ sơ vẽ) là P1 và chỉ mở sau khi preset mặc định chạy ổn và đã có số đo chi phí.

## 5. Thứ tự thực thi

```
CR-OR-001 ─▶ CR-OR-002 ─▶ CR-OR-020                                (MVP chỉ chữ: bảng, không hình)
CR-OR-011 ─▶ CR-OR-010 ─▶ CR-OR-021 ─▶ CR-OR-012                    (kho → vẽ → xem → sinh lại)
CR-OR-020 ─▶ CR-OR-022
CR-OR-010 ─▶ CR-OR-013                                              (tuỳ chỉnh sau khi mặc định chạy ổn)
CR-OR-030 chạy song song từ đợt 2 (bộ test hợp đồng với fixture vàng ngay khi 002 chốt định dạng)
```

| Đợt | CR | Kết quả có thể chạy được |
|---|---|---|
| 1 | 001, 002, 020 | Tab Outcome review (ngay sau Execution): khi Request có Plan thấy ngay phần Design (kiến trúc, luồng, hành vi, dữ liệu, yêu cầu áp dụng, đọc từ KG) dù Execution chưa xong; khi Execution xong thấy thêm phần After code, kết luận, mục lệch, bằng chứng; chưa có hình |
| 2 | 011, 010, 021, 030 (một phần) | Hình sinh một lần, lưu, hiển thị an toàn; lần mở sau không tốn token |
| 3 | 012, 022 | Sinh lại có kiểm soát; quyết định và "gửi lại agent" |
| 4 | 013, 030 (hoàn tất) | Tuỳ chỉnh hồ sơ vẽ; rollout theo project |

## 6. Quy ước chung

- **Hai DB** (Postgres và MySQL), `tenant_id` ở mọi bảng, không FK chéo service; migration theo mẫu hiện có của request-service.
- **Orca không tự đọc KG**; mọi nội dung từ KG đến qua context run, kèm `snapshot` và bằng chứng. Không điền id KG khi không có.
- **Không bao giờ dùng tầng C hoặc A làm căn cứ chặn.** Kết luận chặn (nếu có sau này) chỉ dựa trên tầng F.
- **Chữ không overclaim**: "Chưa xác minh", "Agent báo", "Không có dữ liệu"; không dùng "an toàn để phát hành", "đã đáp ứng yêu cầu" khi chỉ có bằng chứng gián tiếp. `unknown` là trạng thái thật.
- **Không đưa secret vào mô hình, hình vẽ hay log**: chỉ tên biến, đường dẫn tương đối; qua bộ che trước khi lưu.
- **Chi phí là ràng buộc cấp một**: mỗi lượt AI có ước lượng, hạn mức và số liệu thực; không vòng lặp tự sửa không giới hạn.
- **SSH và remote**: không giả định thực thi cục bộ; thao tác git/tệp chạy qua agent trên dev server; lệnh git theo [`git-compatibility.md`](../../../guides/reference/git-compatibility.md) (baseline 2.25).
- **Provider**: tên không gắn riêng GitHub (GitLab và nhà cung cấp khác dùng chung khái niệm).
- **UI**: theo [`STYLEGUIDE.md`](../../../guides/STYLEGUIDE.md), token từ `main.css`, shadcn primitives, nhãn tiếng Anh với khoá i18n đủ 5 locale (en, es, ja, ko, zh); tên file theo khái niệm cụ thể (không `utils/helpers/common`); không thêm `max-lines` disable; file mã quá ngưỡng dòng thì tách.
- **Hai render target**: Electron và web.
- **Idempotent**: consumer sự kiện chịu giao lặp; sinh review hai lần cùng đầu vào cho cùng `inputsDigest` và không tốn token lần hai.

## 7. Rủi ro toàn series

- **Chi phí AI**: một context run và một draw run đều là lượt agent có phí (một Plan OpenSpec thật được ghi 7 đến 13 USD, 13 đến 25 phút; review sẽ nhẹ hơn nhưng chưa đo). Bắt buộc ước lượng, hạn mức, và đo tỉ lệ trúng cache trước khi bật rộng.
- **KG thưa hoặc cũ** làm tầng C trống hoặc sai; mọi mục phải mang `snapshot` và bằng chứng; thiếu thì nói thiếu.
- **Skill bên thứ ba có script** (kể cả xuất PNG qua Playwright): chỉ cho phép đúng các script đã duyệt (`self_check.py`), ghim commit, không cài bản mới tự động.
- **Cổng "style guide" của skill hỏi người dùng** ở lần đầu: lượt chạy không tương tác phải được cấp hồ sơ cố định, nếu không lượt chạy treo hoặc hỏng (spike ở CR-OR-010).
- **HTML do LLM sinh**: coi là không tin cậy (CR-OR-030).
- **Phông**: skill mặc định dùng Google Fonts; app phải dùng phông của Orca (hồ sơ ghi đè) để artifact không cần mạng.
- **Hai hệ thống review** (v7 và v9) có thể chồng chéo trên cổng chất lượng và truy vết yêu cầu (CR-CV-085, 092); v9 chỉ đọc kết quả của chúng, không dựng song song.
- **Docs và code lệch** (đã gặp ở v6, v7): đọc lại code trước khi sửa.

## 8. Điểm lệch và điều cần kiểm chứng trước khi viết mã

1. **Không có trạng thái review** trong `RequestStatus`; thêm trạng thái là đổi máy trạng thái và `IsHumanWaiting`: ngoài MVP (D1).
2. `AGENTS.md` trỏ `docs/STYLEGUIDE.md` và `docs/reference/git-compatibility.md`; đường dẫn thật là `guides/...` (đã ghi ở README v7 mục 8 điểm 19).
3. Handoff 2026-10-17 ghi migration 0098; repo đã có tới 0104 (mục 1).
4. **Chưa đọc**: cách request-service chạy một lượt agent có kết quả có cấu trúc (các usecase của Solution agent và task run), `engine_settings.go` (chỗ thêm cờ), `approval_authorization.go` (tên hành động OPA), `outbox_subjects.go`, `ai_budget.go`, `dev_server_capability.go` (chỗ kiểm skill). Mỗi CR ghi rõ điều này ở mục rủi ro.
5. **Spike bắt buộc trước CR-OR-010**: (a) chạy `diagram-design` không tương tác với hồ sơ cố định và vẽ được container kèm công nghệ/giao thức (ký hiệu gần C4); (b) xác nhận tạo được artifact dùng biến vai trò màu để điền token lúc hiển thị; (c) đo token và thời gian một sơ đồ ≤ 12 nút; (d) đọc `references/` về loại sơ đồ, `self_check.py` và cách ghi đè phông.
6. **Spike trước CR-OR-002**: một lượt thật `tdd-knowledge` trên repo pilot `vnp-qa-platform` (cần `.tdd-agent/config.toml`) để xem hình dạng kết quả KG, tỉ lệ id xác minh được, và **SAD của dự án mô hình hoá container, cạnh phụ thuộc, luồng ra sao** (lớp `Component`/`Decision`); nếu không có thì Architecture và Flow chỉ còn góc nhìn After code với nút `code` (unmapped).
7. Cách phân phối skill `diagram-design` (vendor vào `skills/` và cài bằng `npx skills add … --skill diagram-design`): lệnh cài của repo gốc chưa tìm thấy trong trang đọc được; theo mẫu của bốn skill tdd.

## 9. Điểm chưa ai chốt

- Có cần cổng chặn "phải duyệt review mới được `completed`" (cần trạng thái mới), và với loại Request nào.
- Có hợp nhất `outcome_review_decisions` với `Approval` hiện có (subject `request`, stage `outcome_review`) hay giữ bảng riêng (CR-OR-022 Q1).
- Nơi lưu blob lâu dài khi số review lớn (O1).
- Có cho phép skill vẽ tuỳ chỉnh do tenant tự cung cấp (mã chạy trên dev server) hay chỉ các preset của Orca (CR-OR-013 Q1).
