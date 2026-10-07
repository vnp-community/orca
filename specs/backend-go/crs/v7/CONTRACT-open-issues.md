# Điểm hợp đồng thiếu, lệch hoặc chưa chốt do các solution báo (series v7)

> **Trạng thái: cập nhật 2026-10-06 — P0 và P1 đã được áp dụng vào ba file `CONTRACT-codeintel-*` trong một PR hợp đồng.**
>
> Các vấn đề **đã giải quyết** (đã sửa hợp đồng): H1, H2, H4, H9 (§A); B1, B3, B4, B5, B6, B9 (§B); C1, C2, C6 (chỉ mục + outbox seq + retention), C8, C11 (cảnh báo bảo mật) (§C); D1, D2, D3, D4 (một phần), D5 (§D).
>
> Các vấn đề **còn mở** (chưa có quyết định thiết kế hoặc ngoài phạm vi hợp đồng): H3, H5, H6, H7, H8, H10, H11; B2, B7, B8, B10; C3, C4, C5, C7, C9, C10, C12; D2 (phần còn lại: `payload_json` key convention, `quality.waive` revoke fields); D5 (phần còn lại: `c4.yaml` schema, admin UI `aiReviewLevel`).
>
> Mọi khẳng định là từ báo cáo của agent soạn, chưa được kiểm chứng lại từng điểm.

Cột "Báo bởi" dùng: BE-foundation, BE-graph, BE-sources A/B, BE-gateway, BE-signals, BE-gate, BE-rollout, AG-codeintel, AG-signals, AG-rollout, FE-reviewA/B, FE-gate, FE-vis. Các điểm được báo từ ≥2 nơi là chỗ hổng thật và nên ưu tiên.


## A. Cần quyết định của con người (ảnh hưởng thiết kế, không chỉ chữ nghĩa)

| # | Điểm | Tạm theo | Báo bởi |
|---|---|---|---|
| H1 | Env của tiến trình con `codeintel.*`: hợp đồng agent §2.4 để `toolEnv` nguyên văn (chứa toàn bộ `process.env`, gồm `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`) nhưng §10 và README v7 điểm 22 yêu cầu lọc | Lọc theo danh sách cho phép (task AG 001-03) | AG-codeintel, AG-rollout |
| H2 | Cờ tắt: `CODEINTEL_DISABLED` (PQ-01) vs CR-090 Q4 ("`quality.report` vẫn xuất được khi chỉ code-intel bật"); tên biến/trường `CODE_INTEL_ENABLED`/`settings.set{enabled}` (CR-073) vs `CODEINTEL_ENABLED`/`codeIntelEnabled` (hợp đồng) | Theo hợp đồng; `ExportReviewReport` thất bại khi cờ chất lượng tắt | BE-gate, BE-rollout, FE-reviewB, FE-gate |
| H3 | Frontend có `projectId` của mỗi worktree không (O-1); desktop main có tới được gateway không (O-4, mobile); nâng `SetReadLimit` `/ws` lên 320 KiB (O-2) | Giả định có; mobile `unavailable`; nâng | FE-reviewA/B, BE-gateway |
| H4 | Có hỗ trợ `relay-ssh` (Part B) không (O-5): §5 nói không có `quality.*` trong khi §1.1 nói Go chạy `--stdio` = Part A | Chỉ `direct-websocket`/Part A ở MVP | AG-signals, AG-codeintel |
| H5 | Chiến lược index cho worktree liên kết (O-6): `OVERLAY` hay index riêng (~4 GB/worktree); phép thử M1–M7 của CR-080 chưa chạy | `OVERLAY` | BE-signals, AG-signals |
| H6 | Thư viện parse SQL/proto (tự viết hay thư viện, O-7); ngưỡng điểm rủi ro CR-036; ngưỡng cổng chất lượng — đều là giá trị khởi điểm chưa hiệu chỉnh | Tự viết, mặc định `info` | BE-sources A/B, BE-gate |
| H7 | Duyệt công cụ/phụ thuộc mới (O-9): `@vitest/coverage-v8`, `govulncheck`, `osv-scanner`, `gitleaks`, `elkjs`/`dagre`, `d3-*` | Không thêm | AG-signals, BE-signals, FE-vis |
| H8 | Bộ chạy mặc định của agent có thể chứa profile bảo mật nên lách cờ tenant (`quality_security_scan_enabled`) | Cần chặn ở CR-081/091 | BE-signals |
| H9 | `ORCA_CODEINTEL_DISABLED` có tắt luôn `quality.*` không; hành vi `codeintel.status` trên Windows (§1.1 báo lỗi vs §4.1 "luôn thành công") | Chưa chốt | AG-codeintel, AG-rollout |
| H10 | Nguồn lượt agent từ hook (O-8): `agent.hook` không có ở Part A và Go chỉ giải mã `providerSession`; ai điền `claims` của `quality.turn.record` | Chỉ nguồn renderer; spike ghi envelope thật | BE-gate, FE-gate, FE-reviewB |
| H11 | `request-service` và `FindTaskBySource` chưa tồn tại → nhánh Request của truy vết (CR-092) chỉ có cổng + fake | Chờ v6 | BE-gate |

## B. Thiếu trong `CONTRACT-codeintel-agent-rpc.md`

| # | Thiếu / lệch | Báo bởi |
|---|---|---|
| B1 | `codeintel.status` thiếu `compatibility` (verified/untested/incompatible) và `warnings[]` ổn định (`tool_version_untested`, `index_built_with_old_extraction`, `codeintel_disabled`); thứ tự giao `indexScope`/`freshness` (CR-080) chưa nói | BE-rollout, AG-rollout, AG-codeintel |
| B2 | `TOOL_FAILED.data` thiếu `command`; tập `perf.command` thiếu `check`, `trace`, `list`, `analyze`, `sync`; `perf` khi cache hit/lỗi | BE-rollout, AG-rollout |
| B3 | Mã lỗi thiếu: `CODEINTEL_QUALITY_RUN_INTERRUPTED`, `CODEINTEL_RUN_NOT_FOUND` (README v7 điểm 23), lý do cho `ORCA_CODEINTEL_DISABLED`; hàng đợi đầy và tên env thời gian chờ hàng đợi chưa nêu | BE-signals, AG-rollout |
| B4 | Agent chỉ gửi thông báo sau khi backend gọi một method `codeintel.*`: cần `Watch` sau mỗi lần nối lại | BE-graph |
| B5 | Trạng thái: `quality.finished.status` có `interrupted` nhưng `QualityRun.status` không; `percent:16` ở ví dụ `quality.progress` lệch định nghĩa tiến độ; `skipReason` không có; PQ-26 `ruleResults[]` không rõ nằm đâu; `results`/`coverage` của run `interrupted` hoặc hết TTL | AG-signals, FE-vis |
| B6 | Mẫu `argv` thiếu `{gitCommonDir}` (cho `buf breaking`) và `scopeArgv` (vitest `related`); `network_policy` không có nguồn quyết định ở agent (đề xuất `ORCA_QUALITY_NETWORK`); capability `quality` "≥ 1 profile ready" không xác định được lúc handshake (không có `workspaceRoot`) | AG-signals |
| B7 | CodeGraph luôn `commit:null` nhưng `classifyIndexBasis` so sánh commit; `longest.name` trong `fileSizes` chưa có truy vấn chắc; `check-max-lines-ratchet --init|--prune` ghi baseline (argv profile phải cấm) | AG-signals |
| B8 | Whitelist lệnh §9 mục 1 rộng hơn mức dùng (`query`, `trace`, `list`, `status`, `explore`); registry GitNexus trùng tên (hợp đồng chọn `indexedAt` mới nhất, CR-072 từ chối); từ cấm Cypher trong literal có thể chặn nhầm; nơi xuất `CODEINTEL_METHODS`/`QUALITY_METHODS` và schema `validate` | AG-codeintel, AG-rollout |
| B9 | Không có method agent liệt kê thông điệp commit (CR-092 trailer) nhưng §8.3(5) cấm dùng method ngoài §4–5; `QualityProfileDefinition` thiếu `whenChangedPaths` (CR-085) và `ciMappings` (CR-086) | BE-gate, BE-signals |
| B10 | §11 yêu cầu sửa `branchCompare` nhưng SOL-005 không sửa (khác hành vi HEAD unborn, không có ở `desktop/`); `detectChanges` cần chỉ mục nên CR-036 §2.8 chưa thoả; điểm cắm Part B ở `desktop/src/relay/relay.ts` chưa kiểm cấu trúc `main()`; vị trí module (§11 file phẳng vs fixture `codeintel/__fixtures__`) | AG-codeintel, BE-sources B, AG-rollout |

## C. Thiếu trong `CONTRACT-codeintel-proto-and-data-map.md` (proto, RPC, DB, env)

| # | Thiếu / lệch | Báo bởi |
|---|---|---|
| C1 | Enum `ViewKind` không khớp 19+ chuỗi `view` ở §4; `SymbolDetail` thiếu ở §2.1; `ReindexJob` và request/response của `RequestReindex`/`GetReindexJob` chưa có; `Freshness` chưa có enum; `CodeIntelEvent` thiếu `state` cho `reindexProgress`; ánh xạ `reason` agent → push chưa có | BE-graph |
| C2 | `IndexStatus.index_basis=5` cần `codeintel_index_basis.proto` (đợt 7) nhưng `codeintel_binding.proto` ở đợt 2; `GetIndexStatus` có `selector` không; "`codeintel.proto` rỗng-có-health" mơ hồ | BE-foundation |
| C3 | Chưa có số field cho `GetArchitecture`, `Get/SaveC4Overrides`, `ListDataFlows`, `GetDataFlow`, `ErdChange`; `GetErdResponse` thiếu `services[]`, `warnings` đặt hai nơi; kiểu `version` của c4 không nhất quán (bigint/chuỗi/`expectedVersion`); `GetStructure` trả `ModuleGraph` nhưng agent `subgraph` chỉ nhận `center` symbol|file|cluster; `ARCHITECTURE` = C4 (PQ-10) vs CR-021 ánh xạ sang `overview` | BE-sources A, BE-graph |
| C4 | `StorageMap`: thiếu `Binding.database`, `StorageMap.warnings`, nguồn cho `change`; `CODEINTEL_SECRET_LEAK_BLOCKED` chưa có gRPC status (tạm `Internal`); `ListFindings` thiếu `detectors[]`; PQ-05 `reason` tuỳ chọn nhưng CR-037/038 bắt buộc; thiếu enum `false_positive` ở `finding_dismissals`/`quality_waivers`; chủ sở hữu repository `finding_dismissals` (SOL-011 vs 037) | BE-sources B, BE-rollout |
| C5 | Lỗi `fs.*`/`git.*` không ra khỏi infra-fleet (§3.4 chỉ ánh xạ `codeintel.*`/`quality.*`; chưa giao chủ port `AgentRelay`); thiếu cổng đọc binding xuyên tenant cho supervisor; chưa có subject/audit cho `c4.saved` | BE-sources A, BE-graph |
| C6 | DB: `outbox_events` (T0) không có cột `seq` (hai sự kiện cùng transaction cùng `created_at`); `trigger` (T7) có thể là từ dành riêng MySQL 8; `dirty_fingerprint` (15) trùng nghĩa `tree_fingerprint` (20); `scope_key` của `coverage_reports` không có nguồn module; thiếu chỉ mục `repo_bindings(tenant_id, worktree_id)` và `reindex_jobs(tenant_id, created_at)`; `reindex_jobs` chưa có retention; khoá `quality_trend_points` thêm `source` (O-10) | BE-foundation, BE-signals, BE-gate |
| C7 | RPC thiếu/mơ hồ: `ListCommitChecks` thiếu `tenant_id`, `include_steps`, `RateLimitInfo.limited`; §3.3 thiếu `ListMergeRequests` (GitLab); `GetCoverage` thiếu `reason`, `language=mixed`; `RunnableProfileLister` chưa có chủ; kiểu `before` của `ListAgentTurns` và `delta` của `GetQualityTrend`; `ReviewReportModel` thiếu `include_people`, `include_waiver_reasons`; hotspot không có RPC/kiểu và không có nguồn độ phức tạp | BE-signals, BE-gate, FE-vis |
| C8 | Env §6.2 thiếu: `SCM_INTEGRATION_SERVICE_ADDR`, `CODEINTEL_CI_*`, `CODEINTEL_AUTOREFRESH_*`, `CODEINTEL_SQL_TENANT_RULE_MODE`, trần impact overlay (đề xuất `CODEINTEL_OVERLAY_IMPACT_MAX`), `CODEINTEL_REINDEX_POLL_AFTER` và trần 16 luồng, `CODEINTEL_INTERNAL_CALLER_TOKEN`, bốn biến của quality-gate | BE-signals, BE-sources B, BE-graph, BE-gateway, BE-gate |
| C9 | Bảng vai trò × action cho 49 RPC nằm ngoài ba hợp đồng (ở CR-013/085); ranh giới `feature_gate.go`/bộ đọc cờ giữa SOL-013 và 073; chủ file workflow `code-intel-contract.yml`; `tenant_settings` do CR-011 tạo (không phải CR-073) | BE-rollout |
| C10 | Lệch số liệu CR so với hợp đồng/code: 25 vs 49 RPC, 26 vs 46 kênh; `DO $$` ở 7 file Postgres (CR ghi 4); `logical FK` 16 Postgres/11 MySQL; `_ usecase.` MySQL 23 dòng (CR ghi 29); `go.work` 21 mục (CR-081 ghi 22); telemetry `shared/` 6 bản sao (CR ghi 4) | BE-sources A, AG-signals, FE-gate |
| C11 | Lệch code thật: CR-013 nói không có `rate.NewLimiter` nhưng `api-gateway/internal/usecase/rate_limit.go:46` có; `internalcaller.Guard` chỉ chặn method liệt kê (danh sách guard phải sinh từ `ServiceDesc`); cờ `app.maintenance` chưa có; CR-010/013 "không sửa `common/apperrors`" nhưng PQ-03(8) giao CR-010 thêm 2 Kind; hai file `init-databases.sh` khác nhau (`backend-go/deploy` và `deploy/dev`); `task-service.GetTask` không kiểm grant, `project-service.GetWorktree` không lọc tenant | BE-foundation, BE-rollout, BE-gate |
| C12 | CR-035 tự mâu thuẫn ở khoá `DSN` và ngưỡng entropy 3,5 (che nhầm mọi `*_ADDR`); `affectedFlows` không có `processType` nên bỏ điểm `cross_community` (agent không nhận Cypher); đổi timeout `ai.complete` ảnh hưởng cả `GenerateCommitMessage` của git-gateway | BE-sources B, BE-gate |

## D. Thiếu trong `CONTRACT-codeintel-ui-api.md`

| # | Thiếu / lệch | Báo bởi |
|---|---|---|
| D1 | Cách chuyển proto → JSON (enum chữ thường, int64, optional, nullable) chưa nêu; đề xuất encoder `protoreflect` ở gateway | BE-gateway |
| D2 | `quality.summary` timeout: bảng 120 s vs chú thích ≤ 24 s (tạm 24 s); khung resync toàn luồng không rõ `projectId`/`worktreeId`; quy ước khoá `payload_json`; kiểu proto của `readingProgress`/`notes`/profile definition; trần `structure.limit`, độ dài id/`promptExcerpt`; `quality.waive` thu hồi có cần `reason`/`expiresAt`; `subscribe` cần registry theo kết nối + `Header()` handshake; gateway chưa có stub proto codeintel | BE-gateway |
| D3 | MCP: §9 không có `project_id`; pack 1 bật mặc định nên cần cổng `MCP_CODEINTEL_TOOLS_ENABLED`; danh sách loại trừ CR-041 thiếu `codeIntel.quality.*`; `KeepKeys` phải true; tên tool đúng `codeIntel_*` | BE-gateway |
| D4 | Kiểu thiếu: `manifest`/`cache`/`labels` của `quality.summary`; `generatedFor` của `quality.report`; `scope` của `quality.trace.confirm`; `ContractChange` không có before/after (tạm `details.before|after`); mã preset của `dismissFinding.reason` (tạm đặt trong `reason`, giải thích trong `note`); `ChangeOverlay.limits.totalCounts` chưa chốt tên; `settings.get` không trả vai trò; `ImpactGraph` không có cạnh; `incoming`/`outgoing` của `SymbolDetail` thiếu `key` và dòng | FE-gate, FE-reviewB, FE-reviewA |
| D5 | `reviewState.save`: bỏ `notes`/`turnMarkers` thì giữ hay xoá; cặp commit làm khoá chưa rõ; không có push báo đổi cờ (chỉ làm mới 60 s hoặc lỗi `CODEINTEL_DISABLED`); `QualityFinding.column` 0/1-based; định dạng `turnKey`, `observed/threshold`; `RunnableProfile.id` vs `QualityGate.profile`; không có capability quyền chạy kiểm tra; đơn vị coverage (0..1 hay 0..100); Id luồng GitNexus vs `DataFlow`; schema `c4.yaml` chưa chốt; không rõ màn quản trị nào đổi `aiReviewLevel` | FE-reviewA/B, FE-gate, FE-vis |

## E. Chồng lấn giữa các solution (đã phối hợp, cần giữ nhất quán)

| Chủ đề | Quyết định tạm |
|---|---|
| Khe composer của Create PR/hosted review | Tên `qualityNotice`, thuộc `FE-CV-SOL-085-source-control-quality-notice`; điểm gọi thật `SourceControl.tsx:5247`, `ChecksPanel.tsx:3611`; `FE-CV-SOL-087` không làm `preSubmitNotice` |
| Fake backend `code-intel-fake-backend.ts` | Chủ là `FE-CV-TASK-073-02`; `FE-CV-TASK-050-08` dùng/mở rộng, không tạo bản hai |
| `maskSensitiveText` | Tạo ở `FE-CV-TASK-057-01`; 058/059 dùng lại |
| Hook cờ | `useQualityFeatureFlags` (`FE-CV-TASK-085-01`) và `useCodeIntelSupport` (CR-050) là hai hook có chủ; `FE-CV-TASK-073-01` chỉ kiểm bằng test ma trận |
| `Finding` (CR-059) vs `QualityFinding` | Giữ tách biệt; v1 một dock, hai nguồn, không trộn danh sách |
| `useDiffCommentDecorator` | Được gọi ở 3 nơi (`DiffViewer`, `DiffSectionItem`, `MonacoEditor`): task chú thích diff của `FE-CV-SOL-087-quality-diff-annotations` phải phủ cả ba |
| Treemap | Đã có `status-bar/workspace-space-layout.ts` (trái CR-054 "không có treemap"); `FE-CV-SOL-054` cần xét tái dùng |
| Electron | `desktop/src/renderer` là bản riêng; Electron có thể chưa nhận Review cho tới khi desktop duyệt (task chạm `desktop/` ghi "ngoài `frontend/`") |
