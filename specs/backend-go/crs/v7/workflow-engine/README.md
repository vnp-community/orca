# Workflow Engine v2 — Solution, Task và nghiệm thu (specs/backend-go/crs/v7/workflow-engine)

> 📋 Proposed (chưa triển khai). Bộ thiết kế thực thi cho [CR-WFD-001..011](../../../../../docs/crs/v7/workflow-engine/README.md). **Không phụ thuộc quy trình nghiệp vụ cụ thể nào**: mọi thứ ở đây là cơ chế chung để định nghĩa và chạy workflow tuỳ ý.
>
> **Vị trí:** theo quy ước repo (`specs/backend-go/crs/vN/<series>/{solutions,tasks}`), không phải `backend-go/specs/...` (đường dẫn này không tồn tại). Task phía agent (TypeScript) và frontend cũng nằm ở đây, gắn nhãn `Layer`, để một bảng thứ tự duy nhất điều khiển cả series.
>
> **Hợp đồng dùng chung:** [CONTRACT-workflow-api.md](./CONTRACT-workflow-api.md). Solution, task và test **phải khớp** hợp đồng; đổi hình dạng thì sửa hợp đồng trước.
>
> **Nghiệm thu:** bộ test nằm ở `tests/` (backend Python, client vitest, e2e Playwright), mỗi wave có cổng nghiệm thu riêng — xem mục 5 và [BE-WFD-SOL-012](./solutions/BE-WFD-SOL-012-acceptance-test-suites.md).

## 1. Định nghĩa "production-ready" (áp cho **mọi** task)

Một task chỉ `DONE` khi thoả **tất cả**:

1. **Không stub, không TODO, không "để sau"**: không hàm trả cố định, không `panic("not implemented")`, không nhánh `Unimplemented`, không dữ liệu giả trong đường chạy thật. Thiếu phần phụ thuộc thì **task đó không được xếp trước** phần phụ thuộc (bảng mục 3 đã bảo đảm); không được che bằng mã giả.
2. **Hai dialect**: mọi bảng, migration (`up` và `down`), repository có bản Postgres **và** MySQL, có test hai dialect (testcontainers hoặc job CI có sẵn của repo).
3. **Cách ly tenant**: Postgres `ENABLE` + `FORCE ROW LEVEL SECURITY`; MySQL lọc `tenant_id` mọi `WHERE`; có test đọc chéo tenant cho mọi bảng mới.
4. **Kiểm thử**: unit (bảng ca, biên, lỗi), integration có DB thật, test race (`go test -race`) cho mã đồng thời, test hợp đồng cho proto/kênh; TypeScript có vitest; không giảm độ phủ của package đã sửa.
5. **Quan sát**: metric theo CONTRACT mục 5, log có cấu trúc kèm `execution_id`/`step_id`/`root_trace_id`, **không** log secret.
6. **An toàn**: kiểm quyền ở backend, danh tính người chạy, che secret, giới hạn kích thước/thời gian cho mọi đầu vào; lỗi người dùng không bao giờ là 5xx.
7. **Tương thích**: chỉ thêm (proto số trường mới, chủ đề NATS, kênh, cột); `schemaVersion=1` chạy y như cũ (đường `legacydispatch`); sau cờ `workflow_engine_v2`, **fail closed**.
8. **Chất lượng mã theo repo**: `go vet`, `golangci-lint` (hoặc lint đang dùng của repo), `gofmt`; TS: `tsc`, `oxlint`/eslint, **không** thêm `max-lines` disable (xem `AGENTS.md`); tên tệp theo khái niệm, không `helpers/utils/common`; comment chỉ ghi "vì sao".
9. **Tài liệu**: cập nhật README service, mục lỗi, runbook nếu có hành vi vận hành; ghi lại điều chỉnh so với CR (nếu code thật khác CR).
10. **Re-verify**: mỗi solution mở đầu bằng "Trạng thái hiện tại (re-verify)" — đọc code thật, trích `file:dòng`; nếu CR lệch code thì ghi "Correction relative to CR" và theo code.
11. Chạy được **trên SSH và chế độ web** (không giả định thực thi cục bộ); đa nền tảng (macOS/Linux/Windows) cho phần agent/frontend; đường dẫn dùng `path.join`/tiện ích Node/Go, không cứng `/` hay `\`.
12. Git: nếu đụng lệnh git phải theo `docs/reference/git-compatibility.md` (Git 2.25 baseline, `GitCapabilityCache`).

## 2. Các wave và cổng nghiệm thu

Mỗi wave kết thúc bằng một hệ thống **chạy được và dùng được** (không có task nào của wave sau là điều kiện để wave trước chạy). Cổng nghiệm thu là bộ test ở mục 5, chạy với `ORCA_WORKFLOW_WAVE=<n>`.

| Wave | Tên | Hệ thống làm được gì khi xong | Ưu tiên |
|---|---|---|---|
| **W0** | Nền | Lưu, kiểm tra, ghi phiên bản, khôi phục template v2 qua REST/WS; mọi thứ v1 chạy **y như cũ**; cờ fail-closed | P0 |
| **W1** | Chạy v2 cơ bản | Chạy workflow v2 với input/output có kiểu, retry/timeout/`onError`, sự kiện từng bước trực tiếp, qua UI (form chạy + màn hình theo dõi); quyền và che secret | P0 |
| **W2** | Độ bền vòng đời | Tạm dừng/huỷ **thật**, phục hồi sau khởi động lại không lặp tác dụng phụ, chạy lại từ bước lỗi, hạn mức, retention, artifact, audit, metric | P0 |
| **W3** | Điều khiển luồng và executor | Rẽ nhánh `when`/`skipped`, workflow con, `action`/`mcp_call`, cấp worktree cho bước agent; trình soạn có form theo schema | P1 |
| **W4** | Cổng, trigger, tích hợp | Cổng của người, trigger lịch/sự kiện/webhook/workflow, tích hợp task-service, canvas chỉnh sửa, hộp thư cổng | P1 |
| **W5** | Hoàn thiện | Công cụ nâng cấp v1→v2, runbook và diễn tập quay lui, thư viện/trigger/artifact UI, a11y/perf, `foreach` | P1 (P2 cho `foreach`) |

## 3. Bảng task (số hiệu, wave, phụ thuộc cố định)

`Layer`: **BE** = backend-go, **AGENT** = `agent/` (TypeScript), **FE** = `frontend/`, **TEST** = `tests/`. Phụ thuộc **chỉ trỏ về wave ≤ wave của task** (đã kiểm: đồ thị không chu trình, không có cạnh ngược wave).

### W0 — Nền (P0)

| Task | Slug (tên file `TASK-WFD-<id>-<slug>.md`) | Layer | Phụ thuộc |
|---|---|---|---|
| 002-01 | `expression-engine` | BE | — |
| 005-01 | `step-executor-registry-and-legacy-adapters` | BE | — |
| 010-01 | `tenant-settings-and-feature-flags` | BE | — |
| 011-01 | `legacy-dispatch-isolation-and-routing` | BE | 010-01 |
| 001-01 | `template-versions-migration` | BE | — |
| 001-02 | `definition-v2-domain-and-validation` | BE | 002-01, 005-01 |
| 001-03 | `template-versions-repositories` | BE | 001-01, 001-02 |
| 001-04 | `version-usecases-proto-grpc` | BE | 001-03, 010-01 |
| 001-05 | `execute-pins-version-and-recovery` | BE | 001-03, 011-01 |
| 001-06 | `gateway-template-routes-and-capabilities` | BE | 001-04, 005-01 |
| 011-02 | `version-backfill` | BE | 001-01 |
| CI-01 | `ci-gates-dialects-race-lint` | TEST | — |
| ACC-01 | `acceptance-harness-and-wave0-suite` | TEST | 001-06 |

### W1 — Chạy v2 cơ bản (P0)

| Task | Slug | Layer | Phụ thuộc |
|---|---|---|---|
| 003-01 | `input-validation-and-gateway-inputs` | BE | 001-05 |
| 003-02 | `typed-outputs-and-context-builder` | BE | 002-01, 003-01 |
| 006-01 | `lifecycle-migration` | BE | 001-01 |
| 006-02 | `state-machine-and-error-classification` | BE | — |
| 006-03 | `dispatcher-v2-core-loop` | BE | 006-01, 006-02, 003-02, 001-05, 005-01, 011-01 |
| 006-04 | `retry-timeout-onerror` | BE | 006-03 |
| 006-06 | `agent-timeout-and-cancel-support` | AGENT | — |
| 008-01 | `execution-events-store` | BE | 006-01 |
| 008-02 | `proto-execution-detail-and-list-filters` | BE | 006-03, 008-01 |
| 008-03 | `stream-execution-events-server` | BE | 008-01, 006-03 |
| 008-04 | `gateway-execution-routes-and-stream-channels` | BE | 008-02, 008-03, 003-01 |
| 010-02 | `rego-policy-and-authorization-layer` | BE | 010-01, 001-04 |
| 010-03 | `secret-refs-and-redaction` | BE | 003-02 |
| 010-04 | `ssrf-guard` | BE | 005-01 |
| 009-01 | `fe-types-and-schema-driven-forms` | FE | 001-06 |
| 009-02 | `fe-run-form-and-live-monitor` | FE | 008-04, 009-01 |
| ACC-02 | `wave1-acceptance-suite` | TEST | 009-02, 008-04, 006-04 |

### W2 — Độ bền vòng đời (P0)

| Task | Slug | Layer | Phụ thuộc |
|---|---|---|---|
| 006-05 | `real-pause-resume-cancel` | BE | 006-03, 006-06 |
| 006-07 | `recovery-idempotency-and-rerun` | BE | 006-04 |
| 006-08 | `concurrency-quotas` | BE | 006-03, 010-01 |
| 003-03 | `artifacts-migration-domain-repository` | BE | 001-01 |
| 003-04 | `artifact-usecases-rpc-channels` | BE | 003-03, 003-02, 010-02 |
| 010-05 | `quota-settings-api-and-retention-worker` | BE | 010-01, 008-01 |
| 010-06 | `audit-events` | BE | 010-02 |
| 008-05 | `metrics-and-tracing` | BE | 006-03 |
| ACC-03 | `wave2-acceptance-suite` | TEST | 006-05, 006-07, 006-08, 003-04, 010-05 |

### W3 — Điều khiển luồng và executor (P0/P1)

| Task | Slug | Layer | Phụ thuộc | Ưu tiên |
|---|---|---|---|---|
| 002-02 | `guard-when-and-skipped` | BE | 006-03, 002-01, 003-02 | P0 |
| 002-03 | `expression-reference-validation` | BE | 002-02, 001-02 | P0 |
| 002-04 | `sub-workflow-executor` | BE | 002-02, 006-05, 005-01 | P1 |
| 005-02 | `action-registry-and-handler-framework` | BE | 005-01, 010-02 | P1 |
| 005-03 | `agent-step-workspace-provision` | BE | 005-01, 006-06, 003-02 | P1 |
| 005-04 | `mcp-call-executor` | BE | 005-01, 010-04, 010-03 | P1 |
| 005-05 | `relay-contract-tests-and-method-fixes` | BE | 005-01 | P1 |
| 009-03 | `fe-editor-properties-expressions-diagnostics-versions` | FE | 009-01, 002-03 | P1 |
| ACC-04 | `wave3-acceptance-suite` | TEST | 002-04, 005-03, 005-04, 009-03 | P1 |

### W4 — Cổng, trigger, tích hợp (P1)

| Task | Slug | Layer | Phụ thuộc |
|---|---|---|---|
| 004-01 | `gates-migration` | BE | 006-01 |
| 004-02 | `gate-domain-and-quorum` | BE | — |
| 004-03 | `gate-repositories` | BE | 004-01, 004-02 |
| 004-04 | `gate-step-executor-and-dispatcher-waiting` | BE | 004-03, 006-03, 002-02 |
| 004-05 | `decide-gate-authz-gatedecider-expiry` | BE | 004-04, 010-02, 006-05 |
| 004-06 | `gate-proto-gateway-notification-events` | BE | 004-05, 008-04 |
| 007-01 | `triggers-migration-domain-repository` | BE | 001-01 |
| 007-02 | `fire-trigger-mapping-dedupe-throttle` | BE | 007-01, 003-01, 002-01, 010-02 |
| 007-03 | `scheduler-with-leader-lease` | BE | 007-02 |
| 007-04 | `event-trigger-consumer` | BE | 007-02 |
| 007-05 | `webhook-trigger-endpoint` | BE | 007-02, 010-03 |
| 007-06 | `workflow-completed-trigger-and-loop-guard` | BE | 007-02, 008-01 |
| 007-07 | `trigger-gateway-and-automation-run-workflow` | BE | 007-03, 007-04, 007-05, 007-06 |
| 011-03 | `task-service-integration` | BE | 003-01, 006-03 |
| 009-04 | `fe-editable-canvas` | FE | 009-03 |
| 009-05 | `fe-gate-inbox-and-detail` | FE | 004-06, 009-01 |
| ACC-05 | `wave4-acceptance-suite` | TEST | 004-06, 007-07, 011-03, 009-05 |

### W5 — Hoàn thiện (P1; `foreach` P2)

| Task | Slug | Layer | Phụ thuộc | Ưu tiên |
|---|---|---|---|---|
| 011-04 | `upgrade-tool-and-capability-gating` | BE | 001-04, 002-03, 004-06 | P1 |
| 011-05 | `rollout-runbook-dashboards-rollback-drill` | BE | 008-05, 010-01 | P1 |
| 009-06 | `fe-triggers-library-versions-artifacts` | FE | 007-07, 003-04, 009-03 | P1 |
| 009-07 | `fe-accessibility-i18n-performance` | FE | 009-06, 009-05, 009-04 | P1 |
| 002-05 | `foreach-executor` | BE | 002-04 | P2 |
| ACC-06 | `wave5-acceptance-suite` | TEST | 011-04, 009-07, 002-05 | P1 |

## 4. Solution

| Solution | Tệp | CR | Task |
|---|---|---|---|
| BE-WFD-SOL-001 | [definition-schema-v2-and-versioning](./solutions/BE-WFD-SOL-001-definition-schema-v2-and-versioning.md) | 001 | 001-01…06, CI-01 |
| BE-WFD-SOL-002 | [control-flow-and-expressions](./solutions/BE-WFD-SOL-002-control-flow-and-expressions.md) | 002 | 002-01…05 |
| BE-WFD-SOL-003 | [data-inputs-outputs-artifacts](./solutions/BE-WFD-SOL-003-data-inputs-outputs-artifacts.md) | 003 | 003-01…04 |
| BE-WFD-SOL-004 | [human-gate](./solutions/BE-WFD-SOL-004-human-gate.md) | 004 | 004-01…06 |
| BE-WFD-SOL-005 | [step-executors-and-registry](./solutions/BE-WFD-SOL-005-step-executors-and-registry.md) | 005 | 005-01…05 |
| BE-WFD-SOL-006 | [execution-lifecycle](./solutions/BE-WFD-SOL-006-execution-lifecycle.md) | 006 | 006-01…08 |
| BE-WFD-SOL-007 | [triggers](./solutions/BE-WFD-SOL-007-triggers.md) | 007 | 007-01…07 |
| BE-WFD-SOL-008 | [observability-and-api](./solutions/BE-WFD-SOL-008-observability-and-api.md) | 008 | 008-01…05 |
| FE-WFD-SOL-009 | [editor-and-run-views](./solutions/FE-WFD-SOL-009-editor-and-run-views.md) | 009 | 009-01…07 |
| BE-WFD-SOL-010 | [security-and-governance](./solutions/BE-WFD-SOL-010-security-and-governance.md) | 010 | 010-01…06 |
| BE-WFD-SOL-011 | [migration-and-compatibility](./solutions/BE-WFD-SOL-011-migration-and-compatibility.md) | 011 | 011-01…05 |
| BE-WFD-SOL-012 | [acceptance-test-suites](./solutions/BE-WFD-SOL-012-acceptance-test-suites.md) | tất cả | ACC-01…06 |

Danh sách task chi tiết ở [tasks/README.md](./tasks/README.md).

## 5. Bộ test nghiệm thu (trong `tests/`)

Ba tầng, mỗi tầng chia theo wave bằng biến `ORCA_WORKFLOW_WAVE` (0…5; **không đặt = bỏ qua toàn bộ**, để CI hiện tại không đỏ trước khi triển khai; đặt `n` = chạy mọi ca thuộc wave ≤ n).

| Tầng | Tệp | Chạy |
|---|---|---|
| Backend API (Python, qua api-gateway thật) | `tests/backend/api_workflow_v2.py` (+ `workflow_v2_support.py`), đăng ký trong `run_all.py` suite `workflow-v2`, cập nhật `orca_route_catalog.py` | `ORCA_WORKFLOW_WAVE=1 python run_all.py workflow-v2` |
| Client / WebSocket (vitest) | `tests/client/workflow-v2-rpc.spec.ts`, `tests/client/workflow-v2-contract.spec.ts`, `tests/client/workflow-v2-support.ts` | `ORCA_WORKFLOW_WAVE=1 pnpm vitest run --config tests/client/vitest.config.ts workflow-v2` |
| E2E UI (Playwright web, stack thật) | `tests/e2e/workflow-web/*.web.e2e.ts`, `tests/playwright.workflow-web.config.ts` | `ORCA_WORKFLOW_WAVE=1 WORKFLOW_E2E_BASE_URL=http://localhost:28080 npx playwright test -c tests/playwright.workflow-web.config.ts` |

Cổng nghiệm thu của wave `n` đạt khi cả ba tầng xanh với `ORCA_WORKFLOW_WAVE=n` **và** bộ test của các wave trước vẫn xanh (không hồi quy).

## 6. Thứ tự triển khai khuyến nghị (đường găng)

```
W0:  002-01 ─┐                   010-01 ─▶ 011-01 ─┐
     005-01 ─┴▶ 001-02 ─▶ 001-03 ─▶ 001-04 ─▶ 001-05 ─▶ 001-06 ─▶ ACC-01
     001-01 ─▶ 011-02 ; CI-01 (song song từ đầu)
W1:  003-01 ─▶ 003-02 ─▶ 006-03 ─▶ 006-04 ─▶ 008-02/03 ─▶ 008-04 ─▶ 009-02 ─▶ ACC-02
     006-01, 006-02 song song trước 006-03; 006-06 (AGENT) song song; 010-02/03/04 song song sau 010-01
W2:  006-05 / 006-07 / 006-08 ; 003-03 ─▶ 003-04 ; 010-05 / 010-06 / 008-05 ─▶ ACC-03
W3:  002-02 ─▶ 002-03 ─▶ 009-03 ; 002-04 ; 005-02/03/04/05 song song ─▶ ACC-04
W4:  004-01..06 ; 007-01 ─▶ 007-02 ─▶ 007-03/04/05/06 ─▶ 007-07 ; 011-03 ; 009-04/05 ─▶ ACC-05
W5:  011-04, 011-05, 009-06 ─▶ 009-07, 002-05 ─▶ ACC-06
```

## 7. Quy tắc đánh dấu và vận hành tài liệu

- Trạng thái task: `[ ] TODO`, `[~] IN PROGRESS`, `[x] DONE`, `[!] BLOCKED` (kèm lý do và task chặn). Khi làm xong, tick **từng** mục "Tiêu chí hoàn thành".
- Không được đánh `DONE` nếu còn mục nào ở mục 1 chưa đạt; ghi bằng chứng (lệnh test, kết quả) vào phần "Bằng chứng hoàn thành" cuối task.
- Phát hiện CR hoặc hợp đồng sai so với code thật: ghi vào "Correction relative to CR" của solution tương ứng và **báo** để sửa CONTRACT; không tự lệch.
