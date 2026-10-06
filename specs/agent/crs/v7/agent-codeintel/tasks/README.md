# agent-codeintel (v7) tasks: index (agent)

**Solutions:** [../solutions/](../solutions/README.md)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mỗi task viết sau khi đọc code thật ở `agent/src/relay/` (và `desktop/src/relay/` cho CR-006) và hợp đồng; chưa chạy gì.

Tất cả task chạy trong `agent/` (gói `orca-agent`, TypeScript, vitest), trừ 006-05/006-06 chạm `desktop/` (cần chủ sở hữu desktop duyệt). Lệnh test: trong `/opt/repos/orca/agent`, `pnpm exec vitest run <đường dẫn test>`; toàn gói `pnpm test`; `vitest.config.ts` gom `src/**/*.test.ts`. Kiểm kiểu: `npx tsc --noEmit` chỉ để so sánh trước/sau. **CI hiện không chạy test `agent/`**: job `code-intel-contract` thuộc AG-CV-SOL-070.

Trước khi sửa symbol sẵn có (`route`, `extractTraceFields`, `runToolCommand`, `buildCapabilities`, `stop`, `handleRequest`) phải chạy `gitnexus impact -r /opt/repos/orca <symbol>` và ghi blast radius vào PR (task liên quan đã ghi bước này).

## Solution → Task

| Solution | Task | Tên | Ưu tiên |
|---|---|---|---|
| [001](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) | [001-01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md) | Mã lỗi và tham số chặt | P0 |
| | [001-02](./AG-CV-TASK-001-02-codeintel-limits-and-concurrency-gate.md) | Giới hạn và cổng đồng thời | P0 |
| | [001-03](./AG-CV-TASK-001-03-codeintel-command-whitelist-and-child-env.md) | Whitelist lệnh, env con | P0 |
| | [001-04](./AG-CV-TASK-001-04-run-tool-command-options-and-legacy-tool-guard.md) | `runToolCommand` mở rộng, chặn động từ ghi | P0 |
| | [001-05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md) | `runCodeIntelTool`, tệp tạm, phân loại | P0 |
| | [001-06](./AG-CV-TASK-001-06-gitnexus-registry-and-repo-resolution.md) | Registry và phân giải repo | P0 |
| | [001-07](./AG-CV-TASK-001-07-codeintel-tool-detection-and-capabilities.md) | Phát hiện công cụ, capability | P0 |
| | [001-08](./AG-CV-TASK-001-08-result-envelope-and-head-commit.md) | Phong bì, `headCommit`, `perf` | P0 |
| | [001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md) | `status`, bảng method, dispatcher | P0 |
| [002](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) | [002-01](./AG-CV-TASK-002-01-gitnexus-cypher-literal-and-guard.md) | Mã hoá Cypher và guard | P0 |
| | [002-02](./AG-CV-TASK-002-02-gitnexus-cypher-markdown-parser.md) | Parser markdown | P0 |
| | [002-03](./AG-CV-TASK-002-03-gitnexus-cypher-templates-and-runner.md) | Mẫu và runner | P0 |
| | [002-04](./AG-CV-TASK-002-04-verify-cypher-templates-against-real-gitnexus.md) | Re-verify mẫu + fixture | P0 |
| | [002-05](./AG-CV-TASK-002-05-codeintel-symbol-ref.md) | `SymbolRef` | P0 |
| | [002-06](./AG-CV-TASK-002-06-codeintel-short-lived-cache.md) | Cache ngắn hạn | P0 |
| | [002-07](./AG-CV-TASK-002-07-gitnexus-index-probe.md) | Probe chỉ mục | P0 |
| | [002-08](./AG-CV-TASK-002-08-gitnexus-overview-processes-process-routes.md) | `overview/processes/process/routes` | P0 |
| | [002-09](./AG-CV-TASK-002-09-gitnexus-subgraph-impact-symbol.md) | `subgraph/impact/symbol` | P0 |
| [003](../solutions/AG-CV-SOL-003-codegraph-extraction.md) | [003-01](./AG-CV-TASK-003-01-codegraph-cli-output-parsing.md) | Parse CLI CodeGraph | P1 |
| | [003-02](./AG-CV-TASK-003-02-codegraph-index-probe.md) | Probe chỉ mục | P1 |
| | [003-03](./AG-CV-TASK-003-03-codeintel-symbol-ref-codegraph.md) | `SymbolRef` CodeGraph | P1 |
| | [003-04](./AG-CV-TASK-003-04-codegraph-sqlite-readonly-reader.md) | SQLite chỉ-đọc | P1 |
| | [003-05](./AG-CV-TASK-003-05-codegraph-search-and-files-methods.md) | `codegraphSearch`, `files` | P1 |
| | [003-06](./AG-CV-TASK-003-06-codegraph-affected-tests.md) | `affected` | P1 |
| | [003-07](./AG-CV-TASK-003-07-codegraph-enrichment-of-symbol-subgraph-impact.md) | Làm giàu `symbol/subgraph/impact` | P1 |
| | [003-08](./AG-CV-TASK-003-08-reverify-codegraph-assumptions.md) | Re-verify giả định | P1 |
| [004](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) | [004-01](./AG-CV-TASK-004-01-codeintel-notification-sink.md) | Sink thông báo | P1 |
| | [004-02](./AG-CV-TASK-004-02-reindex-commands-and-progress-parser.md) | Lệnh reindex, tiến độ | P1 |
| | [004-03](./AG-CV-TASK-004-03-reindex-journal.md) | Nhật ký job | P1 |
| | [004-04](./AG-CV-TASK-004-04-reindex-job-core.md) | Lõi job | P1 |
| | [004-05](./AG-CV-TASK-004-05-reindex-runner-cancel-and-verify.md) | Runner, huỷ, verify | P1 |
| | [004-06](./AG-CV-TASK-004-06-reindex-read-guard.md) | Chặn đọc khi làm mới | P1 |
| | [004-07](./AG-CV-TASK-004-07-index-watcher-polling.md) | Bộ thăm dò | P1 |
| | [004-08](./AG-CV-TASK-004-08-reindex-methods-and-session-cleanup.md) | Bốn method, nối `stop()` | P1 |
| | [004-09](./AG-CV-TASK-004-09-reindex-integration-on-repo-copy.md) | Kiểm chứng trên bản sao | P1 |
| [005](../solutions/AG-CV-SOL-005-detect-changes.md) | [005-01](./AG-CV-TASK-005-01-merge-base-resolution.md) | Merge-base | P0 |
| | [005-02](./AG-CV-TASK-005-02-diff-hunk-parser.md) | Parser diff | P0 |
| | [005-03](./AG-CV-TASK-005-03-diff-collection-untracked-and-fallback.md) | Thu thập diff | P0 |
| | [005-04](./AG-CV-TASK-005-04-hunk-to-symbol-mapping.md) | Hunk → symbol | P0 |
| | [005-05](./AG-CV-TASK-005-05-affected-flows-and-clusters.md) | Luồng, cụm | P0 |
| | [005-06](./AG-CV-TASK-005-06-gitnexus-detect-changes-header-crosscheck.md) | `crossCheck` | P2 |
| | [005-07](./AG-CV-TASK-005-07-detect-changes-method.md) | Method + ngân sách | P0 |
| | [005-08](./AG-CV-TASK-005-08-reverify-detect-changes-real-tools.md) | Re-verify | P1 |
| [006](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) | [006-01](./AG-CV-TASK-006-01-transport-neutral-core-guard.md) | Test lõi trung lập | P2 |
| | [006-02](./AG-CV-TASK-006-02-relay-handlers-and-throwable-mapping.md) | `registerCodeIntelHandlers` | P2 |
| | [006-03](./AG-CV-TASK-006-03-dispatcher-forward-error-data-agent-tree.md) | `error.data` (`agent/`) | P2 |
| | [006-04](./AG-CV-TASK-006-04-wire-into-agent-relay-main.md) | Nối `relay.ts` (`agent/`) | P2 |
| | [006-05](./AG-CV-TASK-006-05-desktop-relay-tree-port.md) | Cây `desktop/` | P2 |
| | [006-06](./AG-CV-TASK-006-06-relay-parity-script-and-wire-parity-test.md) | Parity | P2 |
| | [006-07](./AG-CV-TASK-006-07-stdio-and-detach-codeintel-tests.md) | Test stdio/detach | P1 |

Trạng thái mọi task: `[ ] TODO`. Tổng 50 task.

## Thứ tự phụ thuộc (toàn feature)

```
001-01..04 (song song) ─► 001-05 ─┐
001-01 ─► 001-06 ─► 001-08 ───────┼─► 001-09 ─► (mọi CR khác)
001-01 ─► 001-07 ─────────────────┘
001-09 ─► 002-01..03 ─► 002-04 ─► 002-08 ─► 002-09 ─► 005-04…07
          002-05, 002-06, 002-07 ─┘
001-05 ─► 003-01 ─► 003-02/05/06 ; 002-05 ─► 003-03 ; 003-04 ; 003-* ─► 003-07 ─► 003-08
001-09 ─► 004-01 ; 004-02/03 ─► 004-04 ─► 004-05 ─► 004-06 ─► 004-08 ◄─ 004-07 ; 004-08 ─► 004-09
001-06 ─► 005-01 ; 005-02 ; 005-01,02 ─► 005-03 ─► 005-04 ─► 005-07 ; 005-05, 005-06 ─► 005-07 ─► 005-08
006-01 ─► 02 ─► 03 ─► 04 ─► 05 ─► 06 (cần 001–005) ; 006-07 độc lập
```
Làm đầu tiên, song song: 001-01…04, 002-02, 002-05, 002-06, 003-03, 004-03, 005-02, 006-07.

## Ghi chú chung

- Mọi file mới đặt tên theo khái niệm cụ thể (AGENTS.md); không `max-lines` disable (`dispatcher.ts` hai cây đã có sẵn một cái: không thêm); mọi lệnh Git ≤ 2.24; SSH/WSL: agent chạy trên chính host.
- Tên method, tham số, mã lỗi, `reason` trùng nguyên văn hợp đồng agent; backend chịu `-32601` cho method phát hành muộn (`codegraphSearch`, `files`, `reindex*`, `watch`).
- Fixture đặt ở `agent/src/relay/codeintel/__fixtures__/` (chủ AG-CV-SOL-070); chụp thủ công lần đầu ở 002-04, 003-08, 004-09, 005-08.
- TDD cần cập nhật SAU khi triển khai (người điều phối): v5/04 (capabilities), v5/07 (method `codeintel.*`), v5/05 (tool cũ bị chặn động từ ghi), `specs/agent/api/agent-rpc-catalog-runtime.md` (cột Part A/B).
- Mẫu: `specs/agent/crs/v6/agent-capabilities/tasks/README.md`.
