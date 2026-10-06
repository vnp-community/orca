# quality-rollout (v7) tasks: index (agent)

**Solutions:** [../solutions/](../solutions/README.md)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi task là kế hoạch; chưa chạy build/test/công cụ nào. Mã `codeintel`/`quality` của AG-CV-SOL-001/002/003/004/005/081 **chưa tồn tại**: task test import qua hằng đường dẫn ở đầu tệp và đổi một dòng khi tên thật được chốt.

Tất cả task chạy trong `agent/` (gói `orca-agent`, TypeScript, vitest). Lệnh test: trong `/opt/repos/orca/agent`, `pnpm exec vitest run <đường dẫn test>`; job CI dùng `vitest.code-intel-contract.config.ts` (task 070-09). NN liên tục theo từng CR (070: 01-09, 071: 01-07, 072: 01-09, 073: 01-06). Trạng thái mọi task: `[ ] TODO`.

## Solution → Task

| Solution | Task | Tên | Ưu tiên | Phụ thuộc |
|---|---|---|---|---|
| [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md) | [070-01](./AG-CV-TASK-070-01-mini-repo-source-tree.md) | Cây mã nguồn mẫu `mini-repo` có ca đối kháng | P0 | không (làm đầu tiên) |
|  | [070-02](./AG-CV-TASK-070-02-fixture-manifest-budget-and-leak-scan.md) | Schema `MANIFEST.json`, ngân sách kích thước, quét rò rỉ | P0 | 070-01 |
|  | [070-03](./AG-CV-TASK-070-03-capture-script-and-typescript-entry-runner.md) | Script chụp fixture `capture-codeintel-fixtures.mjs` và bộ chạy entry TS | P0 | 070-01, 070-02; AG-CV-SOL-001/002/003 (hằng argv whitelist) |
|  | [070-04](./AG-CV-TASK-070-04-gitnexus-cypher-output-golden-tests.md) | Test vàng parser `cypher` của GitNexus | P0 | 070-02, 070-03; AG-CV-SOL-002 (parser) |
|  | [070-05](./AG-CV-TASK-070-05-gitnexus-json-and-text-commands-golden-tests.md) | Test vàng `context`, `impact`, `query`, `detect-changes`, `status` của GitNexus | P0 | 070-02, 070-03; AG-CV-SOL-002, AG-CV-SOL-005 |
|  | [070-06](./AG-CV-TASK-070-06-codegraph-output-golden-tests.md) | Test vàng parser CodeGraph (JSON, ANSI "not found", SQLite schema) | P0 | 070-02, 070-03; AG-CV-SOL-003 |
|  | [070-07](./AG-CV-TASK-070-07-tool-compatibility-guard-and-version-fixtures.md) | Guard phiên bản/marker và `TestEverySupportedVersionHasFixtures` | P0 | 070-02, 070-04, 070-06; AG-CV-SOL-001 |
|  | [070-08](./AG-CV-TASK-070-08-agent-result-golden-c2-files.md) | Tệp vàng C2 `testdata/agent-results/` và `TestResultMatchesServiceGolden` | P0 | 070-04, 070-05, 070-06, 070-07; AG-CV-SOL-001 (`buildCodeIntelResult`), 002, 003, 005; BE-CV-TASK-070-01 |
|  | [070-09](./AG-CV-TASK-070-09-code-intel-contract-workflow-agent-job.md) | Workflow `code-intel-contract.yml` (job agent) và cấu hình vitest riêng | P0 | 070-02 đến 070-08; BE-CV-TASK-070-06 (cùng tệp) |
| [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md) | [071-01](./AG-CV-TASK-071-01-perf-block-types-and-recorder.md) | Kiểu `CodeIntelPerf` và `PerfRecorder` | P1 | không (AG-CV-SOL-001 cắm ở task 03) |
|  | [071-02](./AG-CV-TASK-071-02-child-process-rss-sampler.md) | Lấy mẫu RSS tiến trình con (Linux `/proc`, macOS `ps`) | P1 | không |
|  | [071-03](./AG-CV-TASK-071-03-wire-perf-into-result-envelope.md) | Cắm `PerfRecorder` vào runner/envelope; `perf` không vào cache và không vào tệp vàng | P1 | 071-01, 071-02; AG-CV-SOL-001 |
|  | [071-04](./AG-CV-TASK-071-04-tool-slot-limits-load-tests.md) | Test bất biến đồng thời và hàng đợi bằng CLI giả | P1 | 071-03; AG-CV-SOL-001 |
|  | [071-05](./AG-CV-TASK-071-05-budgets-file-and-budget-comparison.md) | `codeintel-budgets.json` và bộ so sánh báo cáo với ngân sách | P1 | 071-01 |
|  | [071-06](./AG-CV-TASK-071-06-bench-runner-and-scenarios.md) | Bộ chạy benchmark (lạnh/ấm, 10 phiên, 10 worktree) và báo cáo JSON | P1 | 071-03, 071-04, 071-05; AG-CV-SOL-070 task 03 (bộ chạy entry TS) |
|  | [071-07](./AG-CV-TASK-071-07-nightly-bench-workflow.md) | Workflow benchmark đêm `code-intel-bench.yml` | P1 | 071-05, 071-06 |
| [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md) | [072-01](./AG-CV-TASK-072-01-spawn-recorder-and-spawn-site-scan.md) | Bộ ghi `spawn` (test-only) và quét tĩnh điểm spawn | P0 | AG-CV-SOL-001 |
|  | [072-02](./AG-CV-TASK-072-02-command-whitelist-dash-and-repo-flag-tests.md) | Whitelist đóng, lệnh cấm, giá trị bắt đầu `-`, cờ `-r`/`-p`, reindex `--index-only` | P0 | 072-01; AG-CV-SOL-001, 002, 003, 004 |
|  | [072-03](./AG-CV-TASK-072-03-params-strict-and-schema-reflection-tests.md) | Tham số nghiêm ngặt và phản chiếu schema | P0 | 072-01; AG-CV-SOL-001, 081 |
|  | [072-04](./AG-CV-TASK-072-04-cypher-templates-and-injection-tests.md) | Mẫu Cypher chỉ-đọc, bộ mã hoá và vector tiêm | P0 | 072-01; AG-CV-SOL-002 |
|  | [072-05](./AG-CV-TASK-072-05-workspace-root-and-path-attack-vectors.md) | `workspaceRoot` và đường dẫn: bộ vector dùng chung, symlink, thứ tự kiểm | P0 | 072-01; AG-CV-SOL-001 |
|  | [072-06](./AG-CV-TASK-072-06-repo-resolution-tests.md) | Phân giải repo: worktree lồng, registry trùng, tên gần giống, stderr không rò | P0 | 072-01, 072-05; AG-CV-SOL-001 |
|  | [072-07](./AG-CV-TASK-072-07-process-limits-env-and-canary-tests.md) | Giới hạn stdout/timeout cây tiến trình, lọc env, canary secret | P0 | 072-01, 072-05; AG-CV-SOL-001, 081; AG-CV-TASK-071-04 (`fake-codeintel-cli`) |
|  | [072-08](./AG-CV-TASK-072-08-seeded-fuzz-invariants.md) | Fuzz xác định không thêm phụ thuộc | P0 | 072-02 đến 072-05 |
|  | [072-09](./AG-CV-TASK-072-09-code-intel-security-ci-job.md) | Job CI `code-intel-security` và bước khẳng định số test | P0 | 072-02 đến 072-08; AG-CV-TASK-070-09 |
| [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md) | [073-01](./AG-CV-TASK-073-01-runtime-switches-parser.md) | Bộ đọc `RuntimeSwitches` (`ORCA_CODEINTEL_DISABLED`, `ORCA_CODEINTEL_REINDEX`, `ORCA_QUALITY_RUN`) | P0 | không |
|  | [073-02](./AG-CV-TASK-073-02-disabled-gate-in-dispatchers.md) | Cổng tắt cứng đứng đầu dispatcher `codeintel.*` và `quality.*` | P0 | 073-01; AG-CV-SOL-001, 081 |
|  | [073-03](./AG-CV-TASK-073-03-disabled-capabilities-in-handshake.md) | Bỏ capability `codeintel*`/`quality` khỏi handshake khi tắt | P0 | 073-01; AG-CV-SOL-001, 081 |
|  | [073-04](./AG-CV-TASK-073-04-disabled-startup-no-side-effects.md) | Khởi động khi tắt: không watcher, không quét nhật ký, một dòng log | P0 | 073-01, 073-02; AG-CV-SOL-004 |
|  | [073-05](./AG-CV-TASK-073-05-runbook-and-env-example.md) | Runbook tắt/bật tại máy và `agent-runtime.env.example` | P0 | 073-01 |
|  | [073-06](./AG-CV-TASK-073-06-disabled-end-to-end-dispatch-test.md) | Test tích hợp dispatcher thật: tắt rồi bật | P0 | 073-02, 073-03, 073-04 |

## Thứ tự phụ thuộc

```
070: 01 ─► 02 ─► 03 ─┬─► 04 ┐
                     ├─► 05 ┼─► 07 ─► 08 ─► 09
                     └─► 06 ┘
071: 01, 02 ─► 03 ─► 04 ; 01 ─► 05 ; 03,04,05 ─► 06 ─► 07      (06 cần 070-03)
072: 01 ─► 02, 03, 04, 05 ─► 06 ; 05 ─► 07 ; 02-05 ─► 08 ─► 09   (07 dùng 071-04)
073: 01 ─► 02 ─► 03, 04 ─► 06 ; 01 ─► 05
```

Liên kết giữa các solution của feature: 071-06 dùng bộ chạy entry TS của 070-03; 071-04 (CLI giả) được 072-07 và 073-06 dùng; 070-09 dựng workflow/cấu hình vitest mà 072-09 mở rộng. Làm đầu tiên, song song: 070-01, 071-01, 071-02, 073-01.

## Việc ngoài `agent/` mà các task này cần (không nằm trong task)

1. BE-CV-SOL-070: tên và bố cục `testdata/agent-results/` (070-08), job Go cùng tệp workflow (070-09).
2. BE-CV-SOL-071: `codeintel-budgets.json` phần `views`, tiêu thụ `perf` (071-03, 071-05).
3. BE-CV-SOL-072: đọc `path-attack-vectors.json` từ cây `agent/` (072-05).
4. BE-CV-SOL-073 / FE-CV-SOL-073: runbook bước 3, ca `codeintel_disabled` (073-02, 073-06).
5. **Sửa hợp đồng (PR riêng, không tự sửa):** `compatibility` tri-state và cảnh báo `tool_version_untested`, `index_built_with_old_extraction`, `codeintel_disabled`; `reason="codeintel_disabled"` ở §3.2; `data.command` của `TOOL_FAILED`; tập `perf.command` thiếu `check, trace, list, analyze, sync`; env cho thời gian chờ hàng đợi; hành vi khi hàng đợi đầy; vị trí module (phẳng hay `codeintel/`); bảng `CODEINTEL_METHODS`/`QUALITY_METHODS` công khai; mâu thuẫn env con `toolEnv` (§2.4 so với §10).
