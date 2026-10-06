# Solutions: review-frontend (frontend, v7)

> 📋 Proposed. Chưa triển khai. Soạn ngày 2026-10-06 từ [docs/crs/v7/review-frontend](../../../../../../docs/crs/v7/review-frontend/README.md). Phần A (050-056) do nhóm A soạn; phần B (057-062) do nhóm B soạn, README này chỉ liệt kê. Khi CR và hợp đồng khác nhau, **hợp đồng thắng**: [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (chính), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-01..37, §7 thứ tự, §8.2 tên solution, §9 điểm mở), `CONTRACT-codeintel-agent-rpc.md` (frontend không gọi agent).

## Bảng CR → Solution (15 solution)

| CR | Solution | Nội dung | Số task | Trạng thái |
|---|---|---|---|---|
| CR-CV-050 | [FE-CV-SOL-050-types-and-runtime-bridge](./FE-CV-SOL-050-types-and-runtime-bridge.md) | Kiểu mirror, 46 kênh, mã lỗi, bridge `window.api.codeIntel`, client, dùng fake backend 073-02 | 8 (050-01..08) | 📋 |
| CR-CV-050 | [FE-CV-SOL-050-store-and-query-hooks](./FE-CV-SOL-050-store-and-query-hooks.md) | Selector `{projectId, worktreeId}` (O-1), slice, push, hook cờ/truy vấn/index/reindex | 6 (050-09..14) | 📋 |
| CR-CV-050 | [FE-CV-SOL-050-review-tab-wiring](./FE-CV-SOL-050-review-tab-wiring.md) | Loại tab `review`, `ensureReviewTab`, token `--review-*`, i18n | 6 (050-15..20) | 📋 |
| CR-CV-051 | [FE-CV-SOL-051-review-workspace-shell](./FE-CV-SOL-051-review-workspace-shell.md) | Khung ba cột, phạm vi, chip index (9 `overall`), tóm tắt, 13 trạng thái | 7 | 📋 |
| CR-CV-052 | [FE-CV-SOL-052-reading-order-and-progress](./FE-CV-SOL-052-reading-order-and-progress.md) | Thứ tự đọc theo `ReadingStep`, tiến độ theo `stepKey`, lưu `reviewState` | 6 | 📋 |
| CR-CV-053 | [FE-CV-SOL-053-impact-lens-and-symbol-detail](./FE-CV-SOL-053-impact-lens-and-symbol-detail.md) | Lens Ảnh hưởng (không cạnh), chi tiết symbol, liên kết diff, mã hoá lớp phủ | 8 | 📋 |
| CR-CV-054 | [FE-CV-SOL-054-structure-lens](./FE-CV-SOL-054-structure-lens.md) | Treemap squarified + cây ảo hoá | 6 | 📋 |
| CR-CV-055 | [FE-CV-SOL-055-architecture-c4-lens](./FE-CV-SOL-055-architecture-c4-lens.md) | Lens C4 + chỉnh `c4.yaml` | 7 | 📋 |
| CR-CV-056 | [FE-CV-SOL-056-dataflow-lens](./FE-CV-SOL-056-dataflow-lens.md) | Lens Luồng (Mermaid từ `SequenceModel`) | 7 | 📋 |
| CR-CV-057 | [FE-CV-SOL-057-erd-lens](./FE-CV-SOL-057-erd-lens.md) | Lens ERD; tạo `maskSensitiveText` (057-01) | 6 (do nhóm B soạn) | 📋 có trên đĩa |
| CR-CV-058 | [FE-CV-SOL-058-storage-lens](./FE-CV-SOL-058-storage-lens.md) | Lens Lưu trữ | 5 (do nhóm B soạn) | 📋 có trên đĩa |
| CR-CV-059 | [FE-CV-SOL-059-contract-lens-and-findings](./FE-CV-SOL-059-contract-lens-and-findings.md) | Lens Hợp đồng và Phát hiện | 7 (do nhóm B soạn) | 📋 có trên đĩa |
| CR-CV-060 | [FE-CV-SOL-060-review-notes-and-turn-compare](./FE-CV-SOL-060-review-notes-and-turn-compare.md) | Ghi chú, gửi agent, so sánh lượt | 8 (do nhóm B soạn) | 📋 có trên đĩa |
| CR-CV-061 | [FE-CV-SOL-061-review-entry-points](./FE-CV-SOL-061-review-entry-points.md) | Điểm vào | 7 (do nhóm B soạn) | 📋 có trên đĩa |
| CR-CV-062 | [FE-CV-SOL-062-mobile-review-summary](./FE-CV-SOL-062-mobile-review-summary.md) | Mobile + host method desktop | 6 (do nhóm B soạn) | 📋 có trên đĩa |

Tổng: 61 task phần A + 39 task phần B = 100 task (đếm từ đĩa lúc soạn).

## Thứ tự phụ thuộc

```
050-types ─▶ 050-store ─▶ 050-tab-wiring ─▶ 051 ─┬▶ 052 ─┐
(fake backend 073-02: G4)                         └▶ 053 ─┼▶ 054, 055, 056, 057, 058, 059, 060
                                                          └▶ 061 ─▶ 062
```

Trong 050: 050-01 → 050-02/03 → 050-04 → 050-05/06; 050-09 chặn 050-07; 050-15 → 16 → 17 → 18; 050-19, 050-20 độc lập. 052 và 053 song song sau 051 (053 cần `useRovingListKeys` của 052-04 cho danh sách; 052-05 cần `review-overlay-model` của 053-01). Các lens 054-059 độc lập, chỉ cần khung (051) và mã hoá lớp phủ (053-01); lens thêm bằng một mục vào `REVIEW_LENS_DEFINITIONS`.

## Cổng backend (hợp đồng §7.1) và quy tắc "fake trước, thật sau"

- **G4**: fake backend `code-intel-fake-backend.ts` do [FE-CV-TASK-073-02](../../quality-rollout/tasks/FE-CV-TASK-073-02-code-intel-fake-backend.md) (feature quality-rollout) **sở hữu**; solution 050 chỉ dùng/mở rộng fixture (050-08), **không tạo bản thứ hai**. Mọi lens bắt đầu bằng fake.
- **G3**: kênh thật (`BE-CV-SOL-040-*` đăng ký 46 kênh, giải mã chặt, `codeIntelChannelError`, `SetReadLimit`). Chuyển từ fake sang thật không đổi mã lens; chạy lại cùng test.
- Đợt (hợp đồng §7.3): 3 = 050, 051, 052, 053, 057, 061; 4 = 054, 055, 056; 5 = 059, 060; 6 = 058, 062.

## Dùng chung giữa các nhóm (không tạo bản thứ hai)

| Thứ | Chủ | Ai dùng |
|---|---|---|
| `code-intel-fake-backend.ts` | FE-CV-TASK-073-02 | tất cả |
| `maskSensitiveText` | FE-CV-TASK-057-01 | 053-04 (nguồn symbol), 058, các nơi hiển thị chuỗi tự do |
| `useCodeIntelSupport` (cờ `codeIntel`) | FE-CV-TASK-050-12 | tất cả lens, 061 |
| `useQualityFeatureFlags` (cờ chất lượng) | FE-CV-TASK-085-01 | các solution quality; **không** tạo hook cờ thứ ba |
| `review-overlay-model`, `ReviewOverlayLegend` | FE-CV-TASK-053-01 | 052, 054-059 |
| `useRovingListKeys` | FE-CV-TASK-052-04 | 053, 056, 059 |
| Writer `reviewState.save` (một hàng chờ) | FE-CV-TASK-052-03 | 060 (ghi chú) |
| Khoá `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` | FE-CV-TASK-050-10 | mọi slice theo worktree |

## Bảng "kênh / mô hình hợp đồng → solution FE"

### Kênh `codeIntel.*` (26; UI-API §3.1)

| Kênh | Solution FE dùng | Ghi chú |
|---|---|---|
| `status` | 050-store (050-14), 051 (chip, màn trạng thái) | trả `IndexStatus` phẳng, không phong bì |
| `reindex`, `reindexStatus` | 050-store (050-14), 051 (nút làm mới) | cooldown 5 phút; `percent: null` |
| `structure` | 054 | `path`, `depth` 1..3, `pageToken` |
| `architecture` | 055 | `{containers, view}` |
| `dataFlows`, `dataFlow` | 056 | `includeSequence`, tìm kiếm phía server |
| `erd` | 057 (nhóm B) | |
| `storage` | 058 (nhóm B) | |
| `subgraph` | **không dùng ở MVP**; hằng số ở 050-02; đề xuất làm giàu cạnh cho 053 (câu hỏi mở 053) | |
| `impact`, `symbol` | 053 | `includeSource:false` mặc định phía client |
| `routes` | **chưa có lens FE** trong 050-062; hằng số ở 050-02; nhóm B (059/061) xác nhận nếu dùng | |
| `changeOverlay` | 051 (khung, thanh tóm tắt, rủi ro), dùng bởi 052-059 | `mode`, `detail` |
| `readingOrder` | 052 | dự phòng khi overlay `summary` |
| `findings`, `dismissFinding`, `contractDiff` | 059 (nhóm B) | |
| `reviewState.get`, `reviewState.save` | 052 (tiến độ); 060 (ghi chú, `turnMarkers`, nhóm B) | chung writer 052-03 |
| `c4.get`, `c4.save` | 055 | |
| `bindRepo` | 051 (màn `no-binding`) | |
| `settings.get` | 050-store (050-12) | màn admin `settings.set` thuộc FE-CV-SOL-073 (quality-rollout) |
| `settings.set` | FE-CV-SOL-073 | ngoài review-frontend |
| `subscribe` (stream) + 5 push | 050-store (050-11), 050-types (bridge 050-04) | `quality.*` push chỉ phát lên bus |

### Kênh `codeIntel.quality.*` (20; UI-API §3.2) — ngoài review-frontend, theo §8.2

Hằng số khai báo một lần ở 050-02; hành vi do các feature `quality-gate`/`quality-visualization`: `quality.start/cancel/run/runs/findings/waive/gate/profile.get/profile.save/trend/coverage/ci` ⇒ FE-CV-SOL-085-source-control-quality-notice và FE-CV-SOL-087-quality-* (theo tên trong §8.2; nhóm sở hữu xác nhận từng kênh); `quality.trace/trace.confirm/trace.link` ⇒ FE-CV-SOL-092; `quality.summary` ⇒ FE-CV-SOL-093; `quality.report` ⇒ FE-CV-SOL-090; `quality.turn.record/turns/turn` ⇒ FE-CV-SOL-089 (so sánh lượt ở 060 dùng `reviewState.turnMarkers`, không phải `quality.turns`).

### Mô hình hợp đồng (UI-API §4) → solution

| Mô hình | Định nghĩa | Dùng |
|---|---|---|
| `CodeIntelEnvelope<T>`, `WorktreeSel`, `Settings`, `IndexStatus/ToolIndexStatus/IndexBasis/ReindexJob` (§2, §4.1) | 050-01 | 050-types, 050-store, 051 |
| `SymbolRef/SourceRef`, `ModuleGraph`, `ImpactGraph`, `SymbolDetail`, `RouteMap` (§4.2) | 050-01 | 053, 054 |
| `ChangeOverlay`, `ReadingStep`, `ComponentGroup`, `RiskAssessment`, `IndexFreshness`, `TouchedTable/TouchedContract/ViolationRef` (§4.3) | 050-01 | 051, 052, 053, 057, 058, 059 |
| `ContainerRef`, `C4*` (§4.4) | 050-01 | 055 |
| `DataFlow*`, `SequenceModel`, `DfdModel` (§4.4) | 050-01 | 056 (`DfdModel` chưa dùng) |
| `Erd*`, `Store`, `StorageMap` (§4.4) | 050-01 | 057, 058 (nhóm B) |
| `Finding`, `ContractDiff` (§4.5) | 050-01 | 059 (nhóm B) |
| `ReadingProgress`, `ReviewNoteAnchor`, `ReviewSentBatch`, `ReviewTurnMarker`, `ReviewState` (§4.6) | 050-01 | 052, 060 |
| Kiểu chất lượng (§4.7) | `code-intel-quality-types.ts` (khung 050-01, nội dung 085/087) | quality-* |
| Push `PushChanged/PushReindexProgress/...` (§5) | 050-01 | 050-store (050-11) |
| `MobileReviewSummary` (UI-API §8) | 062 | 062 (nhóm B) |
| Mã lỗi (§2.3), `kind` client | 050-03 | tất cả |

## Điểm lệch giữa CR/README feature và hợp đồng (đã xử lý trong từng solution)

1. `window.api.codeIntel`: `runtimeEnvironments.call` nhận `selector` (không `environmentId`); đích local chỉ `unsupported`.
2. Mọi kênh nhận `{projectId, worktreeId}`; `projectId` tuỳ chọn trong `Worktree`/`Repo` (O-1) ⇒ thiếu thì `unsupported`.
3. `status` trả `IndexStatus` đơn (9 `overall`), không `IndexStatus[]`.
4. Mã lỗi ở tiền tố `message` (~45 mã), không phải "10 mã"/`error.data.code`.
5. Push: `codeIntel.subscribe`, object có `event`; `changed` thường không tự tải lại; backoff 1-30 s.
6. Cờ: `settings.get`, không capability/thăm dò.
7. Tiến độ đọc khoá `stepKey`; hàng thứ tự đọc là bước (file), không phải symbol.
8. `ImpactGraph` không cạnh (hạn chế đã biết); `symbol` có `includeSource` mặc định `true`.
9. `dataFlow` trả `SequenceModel`; `dataFlows` lọc/tìm kiếm phía server.
10. Tab `review` chạm 30 file có `'simulator'` (không phải 18).
11. Electron: `desktop/src/renderer` là bản riêng (74 slice, không `mcp`); Review có thể chỉ chạy ở web tới khi đồng bộ.
12. Có treemap sẵn ở `status-bar/workspace-space-layout.ts` (CR-054 nói không có).

## Hợp đồng thiếu/mâu thuẫn cần người quyết (không tự sửa hợp đồng)

- `reviewState.save`: ngữ nghĩa khi bỏ `notes`/`turnMarkers`; cặp `(baseCommit, headCommit)` khoá dòng khi gồm chưa commit; giới hạn 64 KiB so với số mục `stepKey`.
- `ImpactGraph` cạnh; `SymbolDetail.incoming/outgoing` không có `key`/dòng.
- Id `FlowSummary`/process GitNexus ↔ `DataFlow.id`; `DataFlowSummary` không có `changedStepCount`.
- Schema `c4.yaml` (O-12); `symbolCount` thư mục (054).
- O-1 (`projectId`), O-2 (`SetReadLimit` 320 KiB), O-13 (`head` vắng/compare tổng hợp), O-4 (mobile tới gateway).

## Cách chạy kiểm thử (chung)

- Vitest: `pnpm --dir frontend test -- <đường dẫn>` (`vitest run --config config/vitest.config.ts`; môi trường `node`, component dùng `// @vitest-environment happy-dom`). Typecheck và `lint:switch-exhaustiveness`: chưa kiểm chứng chạy được (xem SOL-050-review-tab-wiring).
- E2E: `tests/e2e/review-*.spec.ts` (mới), mẫu `tests/e2e/tasks-page.spec.ts`; cách tiêm fake backend vào web build chưa kiểm chứng (FE-CV-TASK-073-03).
- Trước khi sửa symbol hiện có: GitNexus `impact`; trước commit: `detect_changes` (CLAUDE.md; chưa chạy).
