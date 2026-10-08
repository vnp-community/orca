# Tasks: review-frontend (frontend, v7)


> CR-050 xác minh 2026-10-07: 16 DONE, 4 PARTIAL (06, 08, 17, 18), 0 BLOCKED. Trạng thái từng task nằm ở dòng `**Status:**` trong file task.
> CR-057/058 xác minh 2026-10-07: 8 DONE (057-01..05, 058-01, 02, 04), 3 PARTIAL (057-06 và 058-05 thiếu e2e; 058-03 chưa ẩn tab khi `unsupported`), 0 BLOCKED.

> 🚧 **In Progress.** Rà soát 2026-10-07: SOL-050-types 6/8 done, SOL-050-store 5/6 done, SOL-050-tab 1/6 done, SOL-051 5/7 done (+2 partial), SOL-052 5/6 done (+1 partial). SOL-053..062 (lens UI) ❌ chưa bắt đầu. Tổng ~16/100 tasks (~16%). Fake backend (073-02) chưa tồn tại — blocker cho e2e. Soạn 2026-10-06. Task chạm `desktop/` ghi "ngoài `frontend/`, cần chủ sở hữu desktop duyệt".

Bảng ánh xạ kênh/mô hình hợp đồng → solution: xem [solutions/README.md](../solutions/README.md).

## Quy ước chạy

- Test: `pnpm --dir frontend test -- <đường dẫn>` (Vitest; component `// @vitest-environment happy-dom`). Chưa chạy bất kỳ test nào.
- Cổng: **G4** (fake backend FE-CV-TASK-073-02) làm trước; **G3** (kênh thật BE-CV-SOL-040-*) sau; không task nào của 050-01..04 cần backend.
- Dùng chung (không tạo bản hai): `code-intel-fake-backend.ts` (073-02), `maskSensitiveText` (057-01), `useCodeIntelSupport` (050-12), `useQualityFeatureFlags` (085-01), `review-overlay-model` (053-01), `useRovingListKeys` (052-04), writer `reviewState.save` (052-03).
- Mỗi task sửa symbol hiện có: GitNexus `impact` trước, `detect_changes` trước commit (chưa chạy).

## Thứ tự tổng quát

```
050-01 → 02,03 → 04 → 05,06 ; 050-09 → 10 → 11,12,13 → 14 ; 050-07 (cần 03,04,09) ; 050-08 (cần 073-02)
050-15 → 16 → 17 → 18 ; 050-19, 050-20 độc lập
051-01 → 02 → 03,04 → 05 → 06 → 07
052-01,02,04 → 03 → 05 → 06       053-01 → 02 → 03 ; 04 ; 05 → 06 → 07 → 08
054-01,02,03 → 04,05 → 06         055-01,02,05 → 03 → 04,06 → 07      056-01..04 → 05 → 06 → 07
057..062: xem Depends on từng task
```

## Bảng task

### CR-CV-050 (20 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-050-01](./FE-CV-TASK-050-01-shared-code-intel-types.md) | Kiểu mirror `code-intel-types.ts` theo hợp đồng | FE-CV-SOL-050-types-and-runtime-bridge | P0 | không |
| [FE-CV-TASK-050-02](./FE-CV-TASK-050-02-rpc-method-constants-and-contract-conformance.md) | Hằng 46 kênh, 5 sự kiện push và test đối chiếu hợp đồng | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 050-01 |
| [FE-CV-TASK-050-03](./FE-CV-TASK-050-03-error-codes-and-envelope-parsers.md) | Mã lỗi, tách tiền tố `message`, parser phong bì và push | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 050-01 |
| [FE-CV-TASK-050-04](./FE-CV-TASK-050-04-shared-bridge-factory-and-preload-types.md) | `createCodeIntelBridge` và kiểu `PreloadApi.codeIntel` | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 050-02, 050-03 |
| [FE-CV-TASK-050-05](./FE-CV-TASK-050-05-web-code-intel-api.md) | Cài đặt web `web-code-intel-api.ts` | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 050-04 |
| [FE-CV-TASK-050-06](./FE-CV-TASK-050-06-electron-preload-code-intel.md) | Preload Electron `codeIntel` (ngoài `frontend/`, cần chủ sở hữu desktop duyệt) (ngoài `frontend/`) | FE-CV-SOL-050-types-and-runtime-bridge | P1 | 050-04 |
| [FE-CV-TASK-050-07](./FE-CV-TASK-050-07-renderer-client-and-error-classification.md) | `codeIntelClient` và `classifyCodeIntelError` | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 050-03, 050-04, 050-09 |
| [FE-CV-TASK-050-08](./FE-CV-TASK-050-08-code-intel-fake-backend.md) | Dùng và mở rộng fixture của fake backend 073-02 (cổng G4) | FE-CV-SOL-050-types-and-runtime-bridge | P0 | 073-02, 050-01, 050-03 |
| [FE-CV-TASK-050-09](./FE-CV-TASK-050-09-worktree-selector-resolution.md) | `resolveCodeIntelSelector` (O-1: `projectId` của worktree) | FE-CV-SOL-050-store-and-query-hooks | P0 | không |
| [FE-CV-TASK-050-10](./FE-CV-TASK-050-10-code-intel-slice-cache-and-worktree-purge.md) | Slice `code-intel`, cache LRU, dọn rò rỉ hai đường | FE-CV-SOL-050-store-and-query-hooks | P0 | 050-01, 050-07 |
| [FE-CV-TASK-050-11](./FE-CV-TASK-050-11-event-stream-reconnect-bus-and-polling.md) | Luồng push `codeIntel.subscribe`, backoff, event bus, polling dự phòng | FE-CV-SOL-050-store-and-query-hooks | P0 | 050-10, 050-04 |
| [FE-CV-TASK-050-12](./FE-CV-TASK-050-12-use-code-intel-support-and-settings-polling.md) | `useCodeIntelSupport` qua `settings.get` | FE-CV-SOL-050-store-and-query-hooks | P0 | 050-07, 050-10 |
| [FE-CV-TASK-050-13](./FE-CV-TASK-050-13-use-code-intel-query-and-paged-query.md) | `useCodeIntelQuery` và `useCodeIntelPagedQuery` | FE-CV-SOL-050-store-and-query-hooks | P0 | 050-07, 050-10, 050-12 |
| [FE-CV-TASK-050-14](./FE-CV-TASK-050-14-use-code-intel-index-status-and-reindex.md) | `useCodeIntelIndexStatus` và `useCodeIntelReindex` | FE-CV-SOL-050-store-and-query-hooks | P0 | 050-10, 050-11, 050-13 |
| [FE-CV-TASK-050-15](./FE-CV-TASK-050-15-tab-type-union-and-session-schema.md) | Union `review`, schema phiên, `toVisibleTabType`, `isRenderableTab`, selectors | FE-CV-SOL-050-review-tab-wiring | P0 | không |
| [FE-CV-TASK-050-16](./FE-CV-TASK-050-16-ensure-review-tab.md) | `ensureReviewTab` | FE-CV-SOL-050-review-tab-wiring | P0 | 050-15, 050-10, 050-09 |
| [FE-CV-TASK-050-17](./FE-CV-TASK-050-17-tab-group-panel-and-tab-bar-wiring.md) | `TabGroupPanel`, `TabBar`, kéo-thả, `ReviewTabHost` | FE-CV-SOL-050-review-tab-wiring | P0 | 050-16, 050-12, 050-20 |
| [FE-CV-TASK-050-18](./FE-CV-TASK-050-18-tab-shortcuts-focus-palette-wiring.md) | Chuyển tab, tiêu điểm, palette, zoom; loại khỏi sync mobile | FE-CV-SOL-050-review-tab-wiring | P1 | 050-17 |
| [FE-CV-TASK-050-19](./FE-CV-TASK-050-19-review-tokens-in-main-css.md) | Token `--review-*` trong `main.css` và kiểm tương phản | FE-CV-SOL-050-review-tab-wiring | P0 | không |
| [FE-CV-TASK-050-20](./FE-CV-TASK-050-20-i18n-keys-and-locale-coverage-test.md) | i18n và test phủ khoá `code-intel-locale-coverage` | FE-CV-SOL-050-review-tab-wiring | P0 | không |

### CR-CV-051 (7 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-051-01](./FE-CV-TASK-051-01-review-ui-slice-and-scope-model.md) | Slice `review-ui` và mô hình phạm vi `review-scope-model` | FE-CV-SOL-051-review-workspace-shell | P0 | 050-10 |
| [FE-CV-TASK-051-02](./FE-CV-TASK-051-02-view-state-and-perceived-loading.md) | `review-view-state` (13 trạng thái) và `usePerceivedLoadingStage` | FE-CV-SOL-051-review-workspace-shell | P0 | 051-01, 050-14 |
| [FE-CV-TASK-051-03](./FE-CV-TASK-051-03-index-freshness-chip-and-reindex-button.md) | `IndexFreshnessChip` (9 `overall`) và `ReindexButton` | FE-CV-SOL-051-review-workspace-shell | P0 | 050-14, 051-02 |
| [FE-CV-TASK-051-04](./FE-CV-TASK-051-04-summary-bar-risk-chip-and-chip-filter.md) | `ReviewSummaryBar`, `ReviewRiskChip`, `review-chip-filter` | FE-CV-SOL-051-review-workspace-shell | P0 | 051-01, 050-13 |
| [FE-CV-TASK-051-05](./FE-CV-TASK-051-05-workspace-layout-lens-registry-drawer.md) | Bố cục ba cột, registry lens, drawer và `ReviewWorkspace` | FE-CV-SOL-051-review-workspace-shell | P0 | 051-02, 051-03, 051-04, 050-17 |
| [FE-CV-TASK-051-06](./FE-CV-TASK-051-06-state-screens-banners-ambiguous-dialog.md) | Màn trạng thái, banner và `AmbiguousSymbolDialog` | FE-CV-SOL-051-review-workspace-shell | P0 | 051-02, 051-03 |
| [FE-CV-TASK-051-07](./FE-CV-TASK-051-07-shell-i18n-storage-catalog-and-e2e.md) | i18n khung, catalog lưu trữ và e2e | FE-CV-SOL-051-review-workspace-shell | P1 | 051-05, 051-06, 050-20 |

### CR-CV-052 (6 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-052-01](./FE-CV-TASK-052-01-reading-order-row-model.md) | Mô hình hàng thứ tự đọc (`reading-order-model`) | FE-CV-SOL-052-reading-order-and-progress | P0 | 050-01, 051-04 |
| [FE-CV-TASK-052-02](./FE-CV-TASK-052-02-reading-progress-merge-and-size-guard.md) | `mergeReadingProgress` và chặn kích thước 64 KiB | FE-CV-SOL-052-reading-order-and-progress | P0 | 050-01 |
| [FE-CV-TASK-052-03](./FE-CV-TASK-052-03-review-progress-slice-load-save-queue.md) | Slice `review-progress`: tải `reviewState.get`, ghi tuần tự `reviewState.save` | FE-CV-SOL-052-reading-order-and-progress | P0 | 052-02, 050-10, 050-13, 051-01 |
| [FE-CV-TASK-052-04](./FE-CV-TASK-052-04-use-roving-list-keys.md) | `useRovingListKeys` (phím danh sách dùng chung) | FE-CV-SOL-052-reading-order-and-progress | P0 | không |
| [FE-CV-TASK-052-05](./FE-CV-TASK-052-05-reading-order-list-components.md) | `ReadingOrderList`, hàng, nhóm, thanh tiến độ | FE-CV-SOL-052-reading-order-and-progress | P0 | 052-01, 052-03, 052-04, 051-05, 053-01 |
| [FE-CV-TASK-052-06](./FE-CV-TASK-052-06-reading-order-states-i18n-and-matrix.md) | Trạng thái rỗng/lỗi, i18n, ma trận lưu trữ | FE-CV-SOL-052-reading-order-and-progress | P1 | 052-05 |

### CR-CV-053 (8 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-053-01](./FE-CV-TASK-053-01-review-overlay-model-and-legend.md) | `review-overlay-model` và `ReviewOverlayLegend` | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | 050-19, 050-01 |
| [FE-CV-TASK-053-02](./FE-CV-TASK-053-02-impact-column-layout.md) | `layoutImpactColumns` (cột theo tầng, không cạnh) | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | 050-01 |
| [FE-CV-TASK-053-03](./FE-CV-TASK-053-03-impact-lens-canvas-list-toolbar.md) | `ImpactLens`, canvas xyflow, danh sách, thanh công cụ | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | 053-01, 053-02, 051-05, 052-04, 050-13 |
| [FE-CV-TASK-053-04](./FE-CV-TASK-053-04-symbol-detail-data-and-panel.md) | Dữ liệu và `SymbolDetailPanel` | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | 050-13, 053-01, 051-05 |
| [FE-CV-TASK-053-05](./FE-CV-TASK-053-05-pending-diff-reveal-in-diff-viewer.md) | `pendingDiffReveal` và cuộn dòng ở `DiffViewer` | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | không |
| [FE-CV-TASK-053-06](./FE-CV-TASK-053-06-review-diff-navigation.md) | `openReviewDiffAtSymbol` và mở trong editor | FE-CV-SOL-053-impact-lens-and-symbol-detail | P0 | 053-05, 051-01 |
| [FE-CV-TASK-053-07](./FE-CV-TASK-053-07-diff-cursor-bus-and-symbol-line-index.md) | Diff → đồ thị: `diff-cursor-line-bus`, `symbol-line-index` | FE-CV-SOL-053-impact-lens-and-symbol-detail | P1 | 053-05, 053-06, 051-05 |
| [FE-CV-TASK-053-08](./FE-CV-TASK-053-08-impact-lens-registration-i18n-e2e.md) | Đăng ký lens Ảnh hưởng, i18n, e2e | FE-CV-SOL-053-impact-lens-and-symbol-detail | P1 | 053-03, 053-04, 053-06 |

### CR-CV-054 (6 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-054-01](./FE-CV-TASK-054-01-structure-treemap-layout-squarified.md) | `layoutSquarifiedTreemap` | FE-CV-SOL-054-structure-lens | P1 | không |
| [FE-CV-TASK-054-02](./FE-CV-TASK-054-02-structure-area-model-and-changed-index.md) | `structure-area-model` và `structure-changed-index` | FE-CV-SOL-054-structure-lens | P1 | 050-01, 053-01 |
| [FE-CV-TASK-054-03](./FE-CV-TASK-054-03-structure-tree-model-and-use-structure-tree.md) | `structure-tree-model` và `useStructureTree` | FE-CV-SOL-054-structure-lens | P1 | 050-13, 050-01 |
| [FE-CV-TASK-054-04](./FE-CV-TASK-054-04-structure-tree-component-aria.md) | `StructureTree` (WAI-ARIA Tree, ảo hoá) | FE-CV-SOL-054-structure-lens | P1 | 054-02, 054-03 |
| [FE-CV-TASK-054-05](./FE-CV-TASK-054-05-structure-treemap-component-and-toolbar.md) | `StructureTreemap` và `StructureToolbar` | FE-CV-SOL-054-structure-lens | P1 | 054-01, 054-02, 054-03, 053-01 |
| [FE-CV-TASK-054-06](./FE-CV-TASK-054-06-structure-lens-file-detail-registration-i18n.md) | `StructureLens`, chi tiết file, đăng ký lens, i18n | FE-CV-SOL-054-structure-lens | P1 | 054-04, 054-05, 053-06, 051-05 |

### CR-CV-055 (7 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-055-01](./FE-CV-TASK-055-01-c4-layout-edge-style-and-overlay-model.md) | `c4-layer-layout`, `c4-edge-style`, `c4-overlay-model` | FE-CV-SOL-055-architecture-c4-lens | P1 | 050-01, 053-01 |
| [FE-CV-TASK-055-02](./FE-CV-TASK-055-02-c4-container-default-and-architecture-query.md) | Container mặc định và truy vấn `architecture` | FE-CV-SOL-055-architecture-c4-lens | P1 | 050-13, 051-01 |
| [FE-CV-TASK-055-03](./FE-CV-TASK-055-03-c4-diagram-canvas-and-relations-table.md) | `C4DiagramCanvas` và `C4RelationsTable` | FE-CV-SOL-055-architecture-c4-lens | P1 | 055-01, 055-02, 053-01 |
| [FE-CV-TASK-055-04](./FE-CV-TASK-055-04-c4-component-detail-and-origin-labels.md) | `C4ComponentDetail`, `C4InferredNotice`, nhãn nguồn | FE-CV-SOL-055-architecture-c4-lens | P1 | 055-03, 053-06 |
| [FE-CV-TASK-055-05](./FE-CV-TASK-055-05-c4-override-document-validator.md) | `validateC4OverrideDocument` | FE-CV-SOL-055-architecture-c4-lens | P1 | không |
| [FE-CV-TASK-055-06](./FE-CV-TASK-055-06-c4-override-editor-save-conflict.md) | `C4OverrideEditor` (soạn, lưu, xung đột) | FE-CV-SOL-055-architecture-c4-lens | P1 | 055-05, 055-02, 050-07 |
| [FE-CV-TASK-055-07](./FE-CV-TASK-055-07-architecture-lens-registration-i18n-e2e.md) | Đăng ký lens Kiến trúc, i18n, e2e | FE-CV-SOL-055-architecture-c4-lens | P1 | 055-03, 055-04, 055-06 |

### CR-CV-056 (7 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-056-01](./FE-CV-TASK-056-01-use-is-dark-theme-hook.md) | Hook `useIsDarkTheme` | FE-CV-SOL-056-dataflow-lens | P1 | không |
| [FE-CV-TASK-056-02](./FE-CV-TASK-056-02-data-flow-mermaid-from-sequence-model.md) | `buildSequenceDiagram` từ `SequenceModel` và `escapeMermaidLabel` | FE-CV-SOL-056-dataflow-lens | P1 | 050-01 |
| [FE-CV-TASK-056-03](./FE-CV-TASK-056-03-data-flow-overlay-and-step-model.md) | Mô hình bước và lớp phủ (`data-flow-overlay`) | FE-CV-SOL-056-dataflow-lens | P1 | 053-01 |
| [FE-CV-TASK-056-04](./FE-CV-TASK-056-04-data-flow-export-and-clipboard.md) | Sao chép Mermaid và xuất `.mmd`/`.svg` | FE-CV-SOL-056-dataflow-lens | P1 | không |
| [FE-CV-TASK-056-05](./FE-CV-TASK-056-05-data-flow-list-pane-and-queries.md) | Danh sách luồng, `useDataFlows`, `useDataFlow` | FE-CV-SOL-056-dataflow-lens | P1 | 050-13, 056-03, 051-01 |
| [FE-CV-TASK-056-06](./FE-CV-TASK-056-06-data-flow-diagram-and-step-list.md) | `DataFlowDiagram`, `DataFlowStepList`, `DataFlowToolbar` | FE-CV-SOL-056-dataflow-lens | P1 | 056-01, 056-02, 056-03, 056-04, 056-05, 052-04, 053-04 |
| [FE-CV-TASK-056-07](./FE-CV-TASK-056-07-data-flow-lens-registration-i18n-e2e.md) | Đăng ký lens Luồng, i18n, e2e | FE-CV-SOL-056-dataflow-lens | P1 | 056-06 |

### CR-CV-057 (nhóm B) (6 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-057-01](./FE-CV-TASK-057-01-sensitive-text-masking.md) | Mô-đun che chuỗi nhạy cảm dùng chung (`maskSensitiveText`) | FE-CV-SOL-057 | P0 | không |
| [FE-CV-TASK-057-02](./FE-CV-TASK-057-02-erd-view-model-and-column-changes.md) | Mô hình hiển thị ERD và gộp thay đổi cột (hàm thuần) | FE-CV-SOL-057 | P0 | FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ErdModel`, `ErdChange`, `ChangeOverlay` ở `s |
| [FE-CV-TASK-057-03](./FE-CV-TASK-057-03-erd-layout-and-table-filter.md) | Bố cục tất định, lọc và tìm kiếm bảng ERD (không thêm thư viện) | FE-CV-SOL-057 | P0 | 057-02 |
| [FE-CV-TASK-057-04](./FE-CV-TASK-057-04-use-code-intel-erd-hook-and-slice-keys.md) | Hook `useCodeIntelErd` và khoá slice ERD | FE-CV-SOL-057 | P0 | FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelQuery`, `CODE_INTEL_WORKTREE_KEYED_STATE |
| [FE-CV-TASK-057-05](./FE-CV-TASK-057-05-erd-lens-canvas-toolbar-list.md) | `ErdLens`: toolbar, canvas xyflow, nút bảng, danh sách ảo hoá | FE-CV-SOL-057 | P0 | 057-02, 057-03, 057-04; FE-CV-SOL-051-review-workspace-shell (`ReviewLensProps`, `ReviewVi |
| [FE-CV-TASK-057-06](./FE-CV-TASK-057-06-erd-table-detail-i18n-and-e2e.md) | `ErdTableDetail`, i18n 5 locale và e2e web lens ERD | FE-CV-SOL-057 | P1 | 057-05; FE-CV-SOL-053-impact-lens-and-symbol-detail (`SymbolDetailPanel`, mở symbol/diff); |

### CR-CV-058 (nhóm B) (5 task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-058-01](./FE-CV-TASK-058-01-storage-view-model-and-change-marks.md) | Mô hình hiển thị StorageMap và đánh dấu thành phần bị đổi (hàm thuần) | FE-CV-SOL-058 | P2 | FE-CV-SOL-050-types-and-runtime-bridge (kiểu `StorageMap`, `Store`, `SourceRef`, `ChangedF |
| [FE-CV-TASK-058-02](./FE-CV-TASK-058-02-storage-lane-layout.md) | Bố cục bốn làn cố định (service, kho, topic, secret) | FE-CV-SOL-058 | P2 | 058-01 |
| [FE-CV-TASK-058-03](./FE-CV-TASK-058-03-use-code-intel-storage-hook.md) | Hook `useCodeIntelStorage`, khoá slice và điều kiện hiện lens | FE-CV-SOL-058 | P2 | FE-CV-SOL-050-store-and-query-hooks; 058-01; G4 fake backend (073-02) |
| [FE-CV-TASK-058-04](./FE-CV-TASK-058-04-storage-lens-canvas-and-secret-node.md) | `StorageLens`, canvas chỉ đọc, nút secret không lộ giá trị | FE-CV-SOL-058 | P2 | 058-01, 058-02, 058-03; FE-CV-SOL-051-review-workspace-shell |
| [FE-CV-TASK-058-05](./FE-CV-TASK-058-05-storage-node-detail-i18n-and-e2e.md) | Chi tiết nút Lưu trữ, liên kết ERD, i18n và e2e web | FE-CV-SOL-058 | P2 | 058-04; 057-04 (`setErdService`); FE-CV-SOL-053-impact-lens-and-symbol-detail; 073-02, 073 |

### CR-CV-059 (nhóm B) (7 task) — 2026-10-07: 059-01/02/03/04/06 DONE, 059-05/07 PARTIAL (xem Status từng task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-059-01](./FE-CV-TASK-059-01-contract-grouping-and-signature-diff.md) | Nhóm thay đổi hợp đồng, hàng `details` và diff chữ ký (hàm thuần) | FE-CV-SOL-059 | P1 | FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ContractChange`, `ContractDiff`); 057-01 (`m |
| [FE-CV-TASK-059-02](./FE-CV-TASK-059-02-finding-view-model-filter-sort.md) | Mô hình hiển thị `Finding`, lọc và sắp xếp (hàm thuần) | FE-CV-SOL-059 | P1 | FE-CV-SOL-050-types-and-runtime-bridge (kiểu `Finding`, `Owner`, `IndexFreshness`); 057-01 |
| [FE-CV-TASK-059-03](./FE-CV-TASK-059-03-findings-and-contract-hooks-and-dismissal.md) | Hook `useCodeIntelFindings`, `useCodeIntelContractDiff`, `useFindingDismissal` và selector | FE-CV-SOL-059 | P1 | FE-CV-SOL-050-store-and-query-hooks; 059-02; G4 fake backend (073-02) |
| [FE-CV-TASK-059-04](./FE-CV-TASK-059-04-contract-diff-lens-components.md) | Lens Hợp đồng: bảng trước/sau, huy hiệu tương thích, chi tiết, nhóm migration | FE-CV-SOL-059 | P1 | 059-01, 059-03; FE-CV-SOL-051-review-workspace-shell; FE-CV-SOL-053-impact-lens-and-symbol |
| [FE-CV-TASK-059-05](./FE-CV-TASK-059-05-findings-panel-row-dismiss-popover.md) | `FindingsPanel`: danh sách, hàng, popover Bỏ qua / Đã xử lý / Mở lại | FE-CV-SOL-059 | P1 | 059-02, 059-03; FE-CV-SOL-051-review-workspace-shell (dock đáy, `react-resizable-panels`); |
| [FE-CV-TASK-059-06](./FE-CV-TASK-059-06-finding-graph-indicators-and-erd-links.md) | Chỉ báo phát hiện trên đồ thị và liên kết "Xem trong đồ thị"/ERD | FE-CV-SOL-059 | P2 | 059-03, 059-05; FE-CV-SOL-053-impact-lens-and-symbol-detail; FE-CV-SOL-054-structure-lens; |
| [FE-CV-TASK-059-07](./FE-CV-TASK-059-07-contract-findings-i18n-and-e2e.md) | i18n 5 locale, test phủ khoá và e2e web Hợp đồng + Phát hiện | FE-CV-SOL-059 | P1 | 059-04, 059-05; 073-02, 073-03 |

### CR-CV-060 (nhóm B) (8 task) — 2026-10-07: 060-01/02/04/05/06 DONE, 060-03/07/08 PARTIAL (xem Status từng task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-060-01](./FE-CV-TASK-060-01-review-note-anchor-and-body.md) | Neo ghi chú đồ thị và dựng nội dung (hàm thuần) | FE-CV-SOL-060 | P1 | FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ReviewNoteAnchor`); 057-01 (`maskSensitiveTe |
| [FE-CV-TASK-060-02](./FE-CV-TASK-060-02-extract-annotation-mark-sent.md) | Trích `markAnnotationsSentBestEffort` ra module dùng chung | FE-CV-SOL-060 | P1 | không (độc lập; làm trước 060-04) |
| [FE-CV-TASK-060-03](./FE-CV-TASK-060-03-review-note-composer-panel-badge.md) | Composer, huy hiệu và `ReviewNotesPanel` | FE-CV-SOL-060 | P1 | 060-01; FE-CV-SOL-051-review-workspace-shell; FE-CV-SOL-053-impact-lens-and-symbol-detail  |
| [FE-CV-TASK-060-04](./FE-CV-TASK-060-04-review-notes-send-menu-and-sent-batch.md) | `ReviewNotesSendMenu`, bản ghi lô đã gửi, xem trước lời nhắc | FE-CV-SOL-060 | P1 | 060-01, 060-02, 060-03 |
| [FE-CV-TASK-060-05](./FE-CV-TASK-060-05-review-state-notes-persistence.md) | Lưu notes/sentBatches/turnMarkers qua `reviewState.save` | FE-CV-SOL-060 | P1 | FE-CV-SOL-052-reading-order-and-progress (bộ ghi `reviewState.save`); FE-CV-SOL-050-store- |
| [FE-CV-TASK-060-06](./FE-CV-TASK-060-06-turn-file-identity-and-compare-model.md) | Dấu vân tay tệp theo lượt và mô hình so sánh lượt (hàm thuần) | FE-CV-SOL-060 | P1 | FE-CV-SOL-050-types-and-runtime-bridge (`ReviewTurnMarker`) |
| [FE-CV-TASK-060-07](./FE-CV-TASK-060-07-review-turn-recorder-hook.md) | Hook ghi mốc lượt `useReviewTurnRecorder` | FE-CV-SOL-060 | P1 | 061-01 (`AgentTurnCompletion`); 060-05, 060-06; FE-CV-SOL-089-agent-turn-recorder (dùng ch |
| [FE-CV-TASK-060-08](./FE-CV-TASK-060-08-review-turn-switcher-i18n-and-e2e.md) | `ReviewTurnSwitcher`, lớp phủ lượt, i18n và e2e | FE-CV-SOL-060 | P1 | 060-04, 060-06, 060-07; FE-CV-SOL-052/053 (lớp phủ `review-overlay-model.ts`, lọc Thứ tự đ |

### CR-CV-061 (nhóm B) (7 task)

> CR-061 xác minh 2026-10-07: 6 DONE (061-01..06), 1 PARTIAL (061-07 thiếu e2e web), 0 BLOCKED. Trạng thái từng task ở dòng `**Status:**` trong file task.

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-061-01](./FE-CV-TASK-061-01-agent-turn-completion-selector.md) | Selector sự kiện "agent xong" và hook `useAgentTurnCompletions` | FE-CV-SOL-061 | P0 | không (đọc `agent-status` slice có sẵn) |
| [FE-CV-TASK-061-02](./FE-CV-TASK-061-02-open-review-entry-and-availability.md) | `openReviewFromEntryPoint` và `useReviewEntryAvailability` | FE-CV-SOL-061 | P0 | FE-CV-SOL-050-review-tab-wiring (`ensureReviewTab`); FE-CV-SOL-050-store-and-query-hooks ( |
| [FE-CV-TASK-061-03](./FE-CV-TASK-061-03-agent-row-review-button.md) | Nút Review ở hàng agent đã xong | FE-CV-SOL-061 | P0 | 061-01, 061-02 |
| [FE-CV-TASK-061-04](./FE-CV-TASK-061-04-source-control-review-entry.md) | Điểm vào ở Source Control | FE-CV-SOL-061 | P0 | 061-01, 061-02 |
| [FE-CV-TASK-061-05](./FE-CV-TASK-061-05-cmd-k-review-actions.md) | Ba hành động Cmd+K (Review, kiến trúc, ERD) | FE-CV-SOL-061 | P1 | 061-02; lens đã phát hành (`reviewLensAvailable`) |
| [FE-CV-TASK-061-06](./FE-CV-TASK-061-06-right-sidebar-review-tab-and-summary-panel.md) | Tab Review ở right sidebar và `ReviewSummaryPanel` | FE-CV-SOL-061 | P1 | 061-01, 061-02; FE-CV-SOL-051-review-workspace-shell (`IndexFreshnessChip`, trạng thái chu |
| [FE-CV-TASK-061-07](./FE-CV-TASK-061-07-entry-points-i18n-and-e2e.md) | i18n 5 locale và e2e web cho điểm vào Review | FE-CV-SOL-061 | P1 | 061-03..061-06; 073-02, 073-03 |

### CR-CV-062 (nhóm B) (6 task) — 2026-10-07: 062-01/02/05/06 DONE, 062-03/04 PARTIAL (xem Status từng task)

| Task | Tên | Solution mẹ | P | Depends on |
|---|---|---|---|---|
| [FE-CV-TASK-062-01](./FE-CV-TASK-062-01-mobile-review-summary-parser.md) | Parser thủ công `MobileReviewSummary` (mobile) | FE-CV-SOL-062 | P2 | không |
| [FE-CV-TASK-062-02](./FE-CV-TASK-062-02-mobile-review-summary-loader-and-model.md) | Loader, ánh xạ `unavailable` và mô hình lọc/sắp xếp | FE-CV-SOL-062 | P2 | 062-01 |
| [FE-CV-TASK-062-03](./FE-CV-TASK-062-03-mobile-review-summary-controller.md) | Controller `useMobileReviewSummaryController` | FE-CV-SOL-062 | P2 | 062-02 |
| [FE-CV-TASK-062-04](./FE-CV-TASK-062-04-mobile-review-summary-screen-route-and-entry.md) | Màn hình, route và điểm vào mobile | FE-CV-SOL-062 | P2 | 062-03 |
| [FE-CV-TASK-062-05](./FE-CV-TASK-062-05-desktop-host-review-summary-method.md) | Host method desktop `codeIntel.reviewSummary` (ngoài `frontend/`, cần chủ sở hữu desktop duyệt) (ngoài `frontend/`) | FE-CV-SOL-062 | P2 | 062-01 (shape); O-4 cho port thật; hợp đồng UI-API §8 |
| [FE-CV-TASK-062-06](./FE-CV-TASK-062-06-mobile-allowlist-and-contract-guard.md) | Allowlist mobile cho `codeIntel.reviewSummary` và test bảo vệ (ngoài `frontend/`, cần chủ sở hữu desktop duyệt) (ngoài `frontend/`) | FE-CV-SOL-062 | P2 | 062-05 |

