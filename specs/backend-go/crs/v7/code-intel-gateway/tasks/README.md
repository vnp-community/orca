# code-intel-gateway: tasks (backend-go, v7)

> Proposed. Mọi task `Status: [ ] TODO`; chưa task nào được làm hay chạy. Mỗi task đã đối chiếu với file thật của `api-gateway` (2026-10-06); chỗ khác CR ghi ở mục Context.
> Hợp đồng: [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md), [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md). Solution: [`../solutions/README.md`](../solutions/README.md).

## Solution -> Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| [BE-CV-SOL-040-codeintel-channel-foundation](../solutions/BE-CV-SOL-040-codeintel-channel-foundation.md) | [040-01](./BE-CV-TASK-040-01-codeintel-config-and-client-wiring.md) | Config, dial, health, `ChannelDeps` | P0 | G0 (stub) |
| | [040-02](./BE-CV-TASK-040-02-pathsafety-package-extraction.md) | Gói `pathsafety` | P0 | — |
| | [040-03](./BE-CV-TASK-040-03-strict-args-decoding-and-selector-validation.md) | Giải mã chặt, selector, bộ kiểm | P0 | 040-02 |
| | [040-04](./BE-CV-TASK-040-04-codeintel-channel-error-mapping.md) | `codeIntelChannelError` | P0 | — |
| | [040-05](./BE-CV-TASK-040-05-proto-to-wire-json-encoder.md) | Encoder `proto -> JSON` | P0 | G0 (test thật) |
| | [040-06](./BE-CV-TASK-040-06-result-envelope-and-unary-runner.md) | Phong bì + runner | P0 | 040-03, 04, 05 |
| | [040-07](./BE-CV-TASK-040-07-channel-catalog-registration-and-placeholders.md) | Catalog 46 kênh, placeholder | P0 | 040-01, 06 |
| | [040-08](./BE-CV-TASK-040-08-ws-read-limit-and-timeout-invariants.md) | `SetReadLimit`, bất biến timeout | P0 | 040-07, O-2 |
| | [040-09](./BE-CV-TASK-040-09-parity-exclusion-docs-and-compose.md) | Loại trừ MCP, README, compose | P0 | 040-07 |
| [BE-CV-SOL-040-codeintel-view-channels](../solutions/BE-CV-SOL-040-codeintel-view-channels.md) | [040-10](./BE-CV-TASK-040-10-view-status-structure-routes.md) | `status`, `structure`, `routes` | P0 | 040-07 |
| | [040-11](./BE-CV-TASK-040-11-view-architecture-and-data-flow-channels.md) | `architecture`, `dataFlows`, `dataFlow` | P1 | 040-10, CR-033/034 |
| | [040-12](./BE-CV-TASK-040-12-view-erd-storage-contract-diff-channels.md) | `erd`, `storage`, `contractDiff` | P1 | 040-11, CR-031/035/038 |
| | [040-13](./BE-CV-TASK-040-13-view-subgraph-impact-symbol-channels.md) | `subgraph`, `impact`, `symbol` | P0 | 040-10 |
| | [040-14](./BE-CV-TASK-040-14-view-change-overlay-reading-order-findings-channels.md) | `changeOverlay`, `readingOrder`, `findings` | P0 | 040-12, CR-036/037 |
| | [040-15](./BE-CV-TASK-040-15-view-channels-conformance-tests.md) | Kiểm hợp đồng nhóm đọc | P1 | 040-10..14 |
| [BE-CV-SOL-040-codeintel-write-and-stream-channels](../solutions/BE-CV-SOL-040-codeintel-write-and-stream-channels.md) | [040-16](./BE-CV-TASK-040-16-state-reindex-and-reindex-status-channels.md) | `reindex`, `reindexStatus` | P0 | 040-07, CR-021 |
| | [040-17](./BE-CV-TASK-040-17-state-dismiss-and-review-state-channels.md) | `dismissFinding`, `reviewState.get/save` | P0 | 040-07, 040-08 |
| | [040-18](./BE-CV-TASK-040-18-state-c4-and-bind-repo-channels.md) | `c4.get/save`, `bindRepo` | P1 | 040-17 |
| | [040-19](./BE-CV-TASK-040-19-state-settings-channels.md) | `settings.get/set` | P0 | 040-07, CR-073 |
| | [040-20](./BE-CV-TASK-040-20-subscribe-stream-open-registry-and-limits.md) | `subscribe`: mở, registry, giới hạn | P0 | 040-07, 04, CR-024 |
| | [040-21](./BE-CV-TASK-040-21-subscribe-event-translation-and-resync.md) | Dịch 5 khung push, `resync` | P0 | 040-20 |
| | [040-22](./BE-CV-TASK-040-22-state-and-stream-conformance-tests.md) | Kiểm hợp đồng ghi + stream | P1 | 040-16..21 |
| [BE-CV-SOL-040-codeintel-quality-channels](../solutions/BE-CV-SOL-040-codeintel-quality-channels.md) | [040-23](./BE-CV-TASK-040-23-quality-run-lifecycle-channels.md) | `quality.start/cancel/run/runs` | P1 | 040-07, CR-082/085 |
| | [040-24](./BE-CV-TASK-040-24-quality-findings-and-waive-channels.md) | `quality.findings/waive` | P1 | 040-23 |
| | [040-25](./BE-CV-TASK-040-25-quality-gate-and-profile-channels.md) | `quality.gate`, `profile.get/save` | P1 | 040-23, CR-085 |
| | [040-26](./BE-CV-TASK-040-26-quality-trend-coverage-ci-channels.md) | `quality.trend/coverage/ci` | P2 | 040-23, CR-083/086 |
| | [040-27](./BE-CV-TASK-040-27-quality-requirement-trace-channels.md) | `quality.trace`, `trace.confirm/link` | P2 | 040-23, CR-092 |
| | [040-28](./BE-CV-TASK-040-28-quality-summary-and-report-channels.md) | `quality.summary` (24 s), `quality.report` | P2 | 040-23, 040-08, CR-090/093 |
| | [040-29](./BE-CV-TASK-040-29-quality-agent-turn-channels.md) | `quality.turn.record`, `turns`, `turn` | P2 | 040-23, CR-089 |
| | [040-30](./BE-CV-TASK-040-30-quality-channels-conformance-and-inventory-gate.md) | Kiểm hợp đồng + cổng 46 kênh | P1 | 040-23..29, 15, 22 |
| [BE-CV-SOL-041-mcp-codeintel-tools](../solutions/BE-CV-SOL-041-mcp-codeintel-tools.md) | [041-01](./BE-CV-TASK-041-01-codeintel-tools-deployment-gate.md) | Cổng `MCP_CODEINTEL_TOOLS_ENABLED` | P2 | O2 = Có, 040 |
| | [041-02](./BE-CV-TASK-041-02-pack1-codeintel-toolspecs.md) | 9 `ToolSpec` | P2 | 041-01, 040-10/11/12/13/14 |
| | [041-03](./BE-CV-TASK-041-03-codeintel-result-shaping-and-sensitive-paths.md) | Kết quả cho LLM, đường dẫn nhạy cảm | P2 | 041-02 |
| | [041-04](./BE-CV-TASK-041-04-exclusion-list-parity-and-golden.md) | Danh sách loại trừ, golden | P2 | 041-02 |
| | [041-05](./BE-CV-TASK-041-05-codeintel-tools-security-and-untrusted-tests.md) | An toàn, taint, red-team | P2 | 041-02..04 |
| | [041-06](./BE-CV-TASK-041-06-mcp-e2e-script-and-guide.md) | e2e MCP, hướng dẫn | P2 | 041-01..05 |

Tổng: 36 task (30 cho CR-CV-040, 6 cho CR-CV-041).

## Kênh -> solution -> task (46 kênh, UI-API §3)

Mọi kênh đăng ký ở catalog (040-07) từ đầu (placeholder), được thay bởi task "nối" dưới đây; 040-15, 040-22, 040-30 là bộ kiểm hợp đồng. Cột "Tool MCP": có tool khi CR-CV-041 được duyệt; còn lại loại trừ (041-04).

### 3.1 Kênh `codeIntel.*` (26)

| # | Kênh | Loại | T/o | Solution | Task nối | Tool MCP |
|---|---|---|---|---|---|---|
| 1 | `codeIntel.status` | unary | 8 s | view-channels | 040-10 | `codeIntel_status` |
| 2 | `codeIntel.reindex` | unary | 8 s | write-and-stream | 040-16 | không |
| 3 | `codeIntel.reindexStatus` | unary | 8 s | write-and-stream | 040-16 | không |
| 4 | `codeIntel.structure` | unary | 20 s | view-channels | 040-10 | không |
| 5 | `codeIntel.architecture` | unary | 20 s | view-channels | 040-11 | không |
| 6 | `codeIntel.dataFlows` | unary | 20 s | view-channels | 040-11 | `codeIntel_dataFlows` |
| 7 | `codeIntel.dataFlow` | unary | 20 s | view-channels | 040-11 | không |
| 8 | `codeIntel.erd` | unary | 20 s | view-channels | 040-12 | `codeIntel_erd` |
| 9 | `codeIntel.storage` | unary | 20 s | view-channels | 040-12 | không |
| 10 | `codeIntel.subgraph` | unary | 20 s | view-channels | 040-13 | không |
| 11 | `codeIntel.impact` | unary | 20 s | view-channels | 040-13 | `codeIntel_impact` |
| 12 | `codeIntel.symbol` | unary | 20 s | view-channels | 040-13 | `codeIntel_symbol` |
| 13 | `codeIntel.routes` | unary | 20 s | view-channels | 040-10 | `codeIntel_routes` |
| 14 | `codeIntel.changeOverlay` | unary | 20 s | view-channels | 040-14 | `codeIntel_changeOverlay` |
| 15 | `codeIntel.readingOrder` | unary | 20 s | view-channels | 040-14 | `codeIntel_readingOrder` |
| 16 | `codeIntel.findings` | unary | 20 s | view-channels | 040-14 | `codeIntel_findings` |
| 17 | `codeIntel.dismissFinding` | unary | 8 s | write-and-stream | 040-17 | không |
| 18 | `codeIntel.contractDiff` | unary | 20 s | view-channels | 040-12 | không |
| 19 | `codeIntel.reviewState.get` | unary | 8 s | write-and-stream | 040-17 | không |
| 20 | `codeIntel.reviewState.save` | unary | 8 s | write-and-stream | 040-17 | không |
| 21 | `codeIntel.c4.get` | unary | 8 s | write-and-stream | 040-18 | không |
| 22 | `codeIntel.c4.save` | unary | 8 s | write-and-stream | 040-18 | không |
| 23 | `codeIntel.bindRepo` | unary | 8 s | write-and-stream | 040-18 | không |
| 24 | `codeIntel.settings.get` | unary | 8 s | write-and-stream | 040-19 | không |
| 25 | `codeIntel.settings.set` | unary | 8 s | write-and-stream | 040-19 | không |
| 26 | `codeIntel.subscribe` | **stream** | — | write-and-stream | 040-20, 040-21 | không |

### 3.2 Kênh `codeIntel.quality.*` (20)

| # | Kênh | Loại | T/o | Solution | Task nối | Tool MCP |
|---|---|---|---|---|---|---|
| 27 | `codeIntel.quality.start` | unary | 8 s | quality-channels | 040-23 | không |
| 28 | `codeIntel.quality.cancel` | unary | 8 s | quality-channels | 040-23 | không |
| 29 | `codeIntel.quality.run` | unary | 8 s | quality-channels | 040-23 | không |
| 30 | `codeIntel.quality.runs` | unary | 8 s | quality-channels | 040-23 | không |
| 31 | `codeIntel.quality.findings` | unary | 20 s | quality-channels | 040-24 | không |
| 32 | `codeIntel.quality.waive` | unary | 8 s | quality-channels | 040-24 | không |
| 33 | `codeIntel.quality.gate` | unary | 8 s | quality-channels | 040-25 | không |
| 34 | `codeIntel.quality.profile.get` | unary | 8 s | quality-channels | 040-25 | không |
| 35 | `codeIntel.quality.profile.save` | unary | 8 s | quality-channels | 040-25 | không |
| 36 | `codeIntel.quality.trend` | unary | 8 s | quality-channels | 040-26 | không |
| 37 | `codeIntel.quality.coverage` | unary | 20 s | quality-channels | 040-26 | không |
| 38 | `codeIntel.quality.trace` | unary | 20 s | quality-channels | 040-27 | không |
| 39 | `codeIntel.quality.trace.confirm` | unary | 8 s | quality-channels | 040-27 | không |
| 40 | `codeIntel.quality.trace.link` | unary | 8 s | quality-channels | 040-27 | không |
| 41 | `codeIntel.quality.summary` | unary | 24 s (gateway) | quality-channels | 040-28 | không |
| 42 | `codeIntel.quality.report` | unary | 20 s | quality-channels | 040-28 | không |
| 43 | `codeIntel.quality.ci` | unary | 20 s | quality-channels | 040-26 | không |
| 44 | `codeIntel.quality.turn.record` | unary | 8 s | quality-channels | 040-29 | không |
| 45 | `codeIntel.quality.turns` | unary | 8 s | quality-channels | 040-29 | không |
| 46 | `codeIntel.quality.turn` | unary | 8 s | quality-channels | 040-29 | không |

Kiểm tổng: 26 + 20 = 46 (45 unary + 1 stream). Task nối: 040-10 (3), 040-11 (3), 040-12 (3), 040-13 (3), 040-14 (3), 040-16 (2), 040-17 (3), 040-18 (3), 040-19 (2), 040-20/21 (1), 040-23 (4), 040-24 (2), 040-25 (3), 040-26 (3), 040-27 (3), 040-28 (2), 040-29 (3) = 15 + 10 + 1 + 20 = 46. Push (5 kênh native qua `subscribe`): `codeIntel.changed`, `.reindexProgress`, `.quality.progress`, `.quality.finished`, `.quality.gateChanged` (040-21).

Không có kênh: `codeIntel.bindings`, `codeIntel.events.subscribe`, `codeIntel.quality.listProfiles`, `codeIntel.hintAgentTurnFinished` (UI-API 3.3).

## Kiểm tra bắt buộc (hợp đồng 8.3 điểm 2)

- `TestChannelInventory`, `TestToolParity` (`mcpserver/tools/parity_test.go`): xanh nhờ một dòng `codeIntel.*` (040-09), thay bằng danh sách tường minh + `codeIntel.quality.*` ở 041-04; `excluded_channels.yaml`.
- `TestCodeIntelChannelInventory` / `TestCodeIntelCatalog_MatchesContract` (040-07): liệt kê cứng 46 tên; 45 unary + 1 stream.
- `TestCodeIntelAllChannelsWired` (040-30): không còn placeholder.
- `TestCodeIntelTimeouts_ShorterThanInvokeTimeout` (040-08): 8/20/24 s < `invokeTimeout` 25 s.
- `SetReadLimit(320 KiB)` (PQ-14, O-2): 040-08.

## Thứ tự phụ thuộc

```
G0 (CR-010/020 stub) ─▶ 040-01 ─┐
040-02 ─▶ 040-03 ─┐             ├─▶ 040-07 ─┬─▶ 040-08 ─▶ 040-09
040-04 ───────────┼─▶ 040-06 ───┘           │
040-05 ───────────┘                          ├─▶ 040-10 ─▶ 040-11 ─▶ 040-12 ─▶ 040-14 ─▶ 040-15
                                             │       └──▶ 040-13 ──────────────────────────┘
                                             ├─▶ 040-16, 040-17 ─▶ 040-18, 040-19 ; 040-20 ─▶ 040-21 ─▶ 040-22
                                             └─▶ 040-23 ─▶ 040-24..029 ─▶ 040-30 (cũng cần 15, 22)
040-10/11/12/13/14 + 040-04 ─▶ 041-01 ─▶ 041-02 ─┬─▶ 041-03 ─┐
                                                  └─▶ 041-04 ─┴─▶ 041-05 ─▶ 041-06
```

Làm song song được: 040-02 với 040-01; 040-03/04/05; 040-10..14 sau 040-07 (mỗi task chỉ nối khi stub RPC có); 040-16/17/19/20; 040-23..29.

## Ghi chú

- Cổng **G3** (§7.1) đạt sau 040-09: 46 kênh đã đăng ký, trả `CODEINTEL_UNAVAILABLE`; FE (CR-050) dùng kênh thật từ đây. Kênh thật tới dần theo RPC của các CR service.
- Trước khi sửa symbol hiện có (`RegisterProductionChannels`, `handleSubscribe`/`ServeHTTP`, `NewCatalog`, `CleanWorktreePath`, `Config.ApplyEnv`) chạy `gitnexus_impact` và ghi vào PR; trước commit chạy `detect_changes()`.
- Lệnh kiểm chung: `cd backend-go/services/api-gateway && go build ./... && go test ./internal/adapter/wscompat/... ./internal/adapter/pathsafety/... ./internal/adapter/mcpserver/... ./internal/config/... ./cmd/server/...`. Chưa chạy.
- Không tạo file tên `helpers/utils/common/misc`; không `max-lines` disable (AGENTS.md); SSH/GitLab/Git 2.25: gateway không chạm dev server hay lệnh Git (ref chỉ được kiểm ký tự).
