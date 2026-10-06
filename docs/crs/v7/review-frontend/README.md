# Feature: review-frontend — Giao diện Review (Code Intelligence & Review View)

> Thuộc series [Change Requests v7](../README.md). Trạng thái: 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-05; chưa chạy test hay ứng dụng.

Phạm vi: `frontend/src/renderer/src`, `frontend/src/shared`, `frontend/src/preload/api-types.ts` (một thay đổi ngoài `frontend/` được nêu rõ ở CR-CV-050 2.2: `desktop/src/preload/index.ts`). Backend thuộc CR-CV-001 đến 041, chỉ tham chiếu bằng mã CR. Nguyên tắc "UI sau cùng": bắt đầu khi RPC của đợt 2 (CR-CV-040 và các RPC `codeIntel.*` của service) chạy được; CR-CV-050 có thể bắt đầu sớm hơn bằng backend giả (`test-support/code-intel-fake-backend.ts`).

## Danh sách CR (13)

| CR | Tên | Priority | Effort | Phụ thuộc chính |
|---|---|---|---|---|
| [CR-CV-050](./CR-CV-050-review-frontend-foundation.md) | Nền frontend: kiểu, `window.api.codeIntel`, slice, push, i18n, cờ, loại tab `review` | 🔴 P0 | Large | CR-CV-040, 020 |
| [CR-CV-051](./CR-CV-051-review-workspace-shell.md) | Khung màn Review: ba cột, thanh tóm tắt, phạm vi, chip index, trạng thái/lỗi | 🔴 P0 | Large | 050 |
| [CR-CV-052](./CR-CV-052-reading-order-and-review-progress.md) | Thứ tự đọc, tiến độ review, phím tắt, lưu trạng thái | 🔴 P0 | Medium | 050, 051 |
| [CR-CV-053](./CR-CV-053-impact-lens-and-symbol-detail.md) | Lens Ảnh hưởng + panel chi tiết symbol + liên kết hai chiều với diff + chú giải lớp phủ | 🔴 P0 | Large | 050 đến 052 |
| [CR-CV-054](./CR-CV-054-structure-lens.md) | Lens Cấu trúc (treemap SVG + cây ảo hoá) | 🟠 P1 | Medium | 050, 051, 053 |
| [CR-CV-055](./CR-CV-055-architecture-c4-lens.md) | Lens Kiến trúc C4 (+ chỉnh `c4.yaml`) | 🟠 P1 | Large | 050, 051, 053 |
| [CR-CV-056](./CR-CV-056-dataflow-lens.md) | Lens Luồng dữ liệu (Mermaid) | 🟠 P1 | Medium | 050 đến 053 |
| [CR-CV-057](./CR-CV-057-erd-lens.md) | Lens ERD | 🔴 P0 | Large | 050, 051, 053 (soạn bởi agent khác) |
| [CR-CV-058](./CR-CV-058-storage-lens.md) | Lens Lưu trữ | ⚪ P2 | Medium | 050, 051, 053 (soạn bởi agent khác) |
| [CR-CV-059](./CR-CV-059-contract-lens-and-findings.md) | Lens Hợp đồng + danh sách Phát hiện | 🟠 P1 | Large | 050, 051, 053 (soạn bởi agent khác) |
| [CR-CV-060](./CR-CV-060-review-notes-send-to-agent-and-turn-compare.md) | Ghi chú → gửi lại agent, so sánh với lượt trước | 🟠 P1 | Large | 050 đến 053 (soạn bởi agent khác) |
| [CR-CV-061](./CR-CV-061-review-entry-points.md) | Điểm vào: dashboard, Source Control, Cmd+K, tab right sidebar | 🔴 P0 | Medium | 050, 051 (soạn bởi agent khác) |
| [CR-CV-062](./CR-CV-062-mobile-review-summary.md) | Mobile: màn tóm tắt chỉ đọc | ⚪ P2 | Medium | 050 (kiểu), backend (soạn bởi agent khác) |

CR-CV-057 đến 062 do agent khác soạn đồng thời; bảng chỉ lấy tiêu đề và priority theo README v7 mục 4. Khi có mâu thuẫn giữa README này và nội dung của CR đó, theo CR.

## Thứ tự thực thi

```
CR-CV-050 ─▶ CR-CV-051 ─┬▶ CR-CV-052 ─┐
                        └▶ CR-CV-053 ─┼▶ CR-CV-054   (Cấu trúc)
                                      ├▶ CR-CV-055   (C4)
                                      ├▶ CR-CV-056   (Luồng)  [cần thêm 052: phím danh sách]
                                      ├▶ CR-CV-057   (ERD)
                                      ├▶ CR-CV-058   (Lưu trữ)
                                      ├▶ CR-CV-059   (Hợp đồng, Phát hiện)
                                      └▶ CR-CV-060   (Ghi chú, gửi agent)
CR-CV-051 ──────────────────────────────────────────▶ CR-CV-061 (Điểm vào) ─▶ CR-CV-062 (Mobile)
```

Phụ thuộc thật: 052 và 053 chạy song song sau 051 (053 cần 052 cho nút "Xem diff" dùng chung trong danh sách và cần `selectedSymbolKey`; có thể làm với danh sách tạm). Các lens 054 đến 059 độc lập nhau, chỉ cần khung (051) và chú giải lớp phủ (053 mục 2.5); lens chưa xong không làm hỏng khung vì lens đăng ký qua `review-lens-registry.ts`. 061 chỉ cần `ensureReviewTab` của 050 và khung của 051 (nút Review mở được một khung có thể rỗng). Đợt gợi ý theo README v7 mục 5: đợt 3 gồm 050, 051, 052, 053, 057, 061; đợt 4 gồm 054, 055, 056; đợt 5 gồm 059, 060; đợt 6 gồm 058, 062.

## Quyết định chung cho cả nhóm

- **Loại tab `review` mới**, mẫu `simulator` (không bản ghi nền), **một tab mỗi worktree**, `entityId = worktreeId`, vẽ `ReviewWorkspace` thay `EditorPanel` trong `TabGroupPanel`; right sidebar chỉ làm lối vào (D7). Sở hữu: CR-CV-050 mục 2.8, kèm danh sách vị trí cần sửa đã đọc từ mã.
- **Chủ sở hữu kiểu, cầu nối, client, slice, cache:** `frontend/src/shared/code-intel-{types,rpc-methods,errors,bridge,wire-parsers}.ts`, `window.api.codeIntel`, `runtime-code-intel-client.ts`, `store/slices/code-intel.ts` và các hook `useCodeIntel*` thuộc CR-CV-050; các CR sau chỉ dùng, không định nghĩa lại. Kiểu `Finding`/`ContractDiff` thêm bởi CR-CV-059 vào cùng `code-intel-types.ts`.
- **Cầu nối trả phong bì, không ném chuỗi:** mã `CODEINTEL_*` phải sống sót tới UI (khác `window.api.mcp`). Phát hiện tính năng bằng capability hoặc thăm dò `codeIntel.status`, **không** bằng `typeof window.api.codeIntel` (web bọc `window.api` bằng Proxy `withFallback`).
- **Một nguồn cho khung và trạng thái:** `ReviewWorkspace` (051) quyết định thân màn (11 trạng thái, thứ tự ưu tiên) bằng `review-view-state.ts`; lens chỉ xử lý lỗi riêng của truy vấn mình. Lỗi và trạng thái chưa sẵn sàng là **persistent inline, không toast** (STYLEGUIDE).
- **Slice theo worktree và chống rò rỉ:** mọi khoá theo `worktreeId` (cache kết quả, trạng thái index, UI, tiến độ) nằm trong hằng `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (050), được dọn ở **cả hai** đường xoá worktree (`removeWorktree` và `buildWorktreePurgeState`); mỗi CR thêm khoá phải thêm test rò rỉ theo mẫu `generation-records-worktree-removal-leak.test.ts` và `bulk-worktree-purge-terminal-maps-leak.test.ts`, và thêm slice vào `store-test-helpers.ts`.
- **`ReviewUiState` dùng chung** (051): `scope`, `lens`, `chipFilter`, `selectedSymbolKey`, cộng trường do lens thêm (`impactFocusKey` 053, `c4ContainerId` và `c4Drafts` 055, `dataFlowId` 056).
- **Mã hoá lớp phủ duy nhất** (053 mục 2.5): đã đổi, bị ảnh hưởng, chưa test, vi phạm; mỗi cờ có **màu (token `--review-*`) và dấu hình học** (viền, nét đứt, icon). Mọi lens và cột trái dùng `review-overlay-model.ts`; chú giải dựng từ cùng bảng.
- **Token và UI:** chỉ token từ `main.css` (thêm `--review-changed|affected|untested|violation` và `--review-area-1..6` ở CR-CV-050), primitive `components/ui/*`, `lucide-react`; không hex, không lớp màu Tailwind thô, không emoji. Không dùng `components/code-review/*` (code chết), không sao chép `DAGPreview.tsx`/`TaskDAGView.tsx` (hardcode hex).
- **Không thêm thư viện** (O5): bố cục tầng tự viết, treemap squarified tự viết, Mermaid có sẵn; `yaml` và `zod` đã nằm trong `frontend/package.json`.
- **Hiển thị theo thời lượng** (STYLEGUIDE): `usePerceivedLoadingStage` (051): không đổi < 100 ms, vô hiệu hoá 100 ms đến 1 s, spinner ≥ 1 s, nhãn giai đoạn ≥ 3 s; đích từ xa trì hoãn phần hiển thị 200 ms; trạng thái vô hiệu áp ngay.
- **Phím tắt:** chỉ phím cục bộ trong tab Review (`j`/`k`/`Enter`/`Space`, `[`, `]`, `Esc`), bỏ qua khi `isEditableTarget`; không đăng ký vào `KEYBINDING_DEFINITIONS` ở nhóm này. Nếu CR sau cần phím có phím bổ trợ thì `metaKey` trên Mac, `ctrlKey` nơi khác, nhãn bằng `ShortcutKeyCombo`; chỉ hiển thị chip phím cho phím đã cài đặt.
- **i18n:** khoá đọc theo tên, tiền tố `auto.components.reviewMap.` và `auto.hooks.codeIntel.`, đủ `en, es, ja, ko, zh`, test phủ `i18n/code-intel-locale-coverage.test.ts`; không gọi `translate()` ở cấp module. Văn bản không khẳng định điều chưa xác thực.
- **Lưu trạng thái (O6):** trạng thái đã xem và ghi đè C4 ở backend; `localStorage` chỉ cho hình học panel (`orca.review.layout.v1`), bọc `try/catch`. Không nhân bản trạng thái bền vào `localStorage` (bị xoá khi đăng xuất; xem `specs/frontend/storage/dev-server-agent-impact.md`).
- **Hai render target:** mọi tính năng chạy cả web và Electron qua cùng `createCodeIntelBridge`; mỗi lời gọi mang `environmentId` của môi trường **sở hữu worktree**, không phải môi trường toàn cục.
- **SSH và remote:** không giả định thực thi cục bộ; mọi trạng thái tải chịu 50 đến 200 ms thêm; lệnh git chỉ dùng API đã có (baseline Git 2.25 theo AGENTS.md).
- **Nhà cung cấp:** gọi "Review đã gửi" (hosted review), không "PR", cho GitLab và các nhà cung cấp khác.
- **Kiểm thử:** Vitest, môi trường mặc định `node`; component dùng `// @vitest-environment happy-dom` + Testing Library; mock `@xyflow/react` như `TaskDAGView.test.tsx`; backend giả dùng chung `code-intel-fake-backend.ts`. Chạy bằng `pnpm --dir frontend test`.
- **Không thêm `max-lines` disable** (AGENTS.md); `web-preload-api.ts` đã có từ trước, chỉ thêm một dòng gắn namespace.

## Phạm vi ngoài

- Backend, agent, gateway, schema DB (CR-CV-001 đến 041); proto.
- Tool MCP `codeintel_*` (CR-CV-041).
- `CombinedDiffViewer` cuộn tới dòng (CR-CV-053 chỉ mở diff một file; mở rộng để sau).
- Chạy `analyze`/`reindex` tự động theo lịch (O3: người dùng bấm).
- Bố cục bằng `elkjs`/`dagre` và thư viện treemap/`d3-*` (O5).
- Cài đặt `window.api.codeIntel` trong gói `desktop/` (CR-CV-050 nêu thay đổi cần chủ sở hữu desktop duyệt; nằm ngoài `frontend/`).
- Màn mobile đầy đủ (CR-CV-062 chỉ tóm tắt chỉ đọc); đồng bộ tab Review sang mobile (đã loại ở `sync-runtime-graph.ts`, CR-CV-050 2.8).
- Sửa `MermaidBlock.tsx`, `ConnectionStatusBanner.tsx`, `DAGPreview.tsx`, `TaskDAGView.tsx` (chỉ ghi nhận lệch).

## Điểm lệch giữa hợp đồng/nghiên cứu và code (cần chốt khi duyệt; chưa sửa README v7)

Đã đối chiếu với code thật ngày 2026-10-05.

1. **Nghiên cứu 10 §7 gợi ý dùng `ConnectionStatusBanner`**: file chỉ có ở web, là lớp phủ `position: fixed` góc dưới phải, hardcode hex. Dùng `store/slices/connectivity-status.ts` (`connections`, `isConnectivityLikeRpcError`) và banner inline (CR-CV-051).
2. **README v7 3.7 liệt kê kênh nhưng không có tham số**, không có kênh đăng ký push, không có mã lỗi `CODEINTEL_DISABLED`, mã xung đột phiên bản, hay capability báo cờ `code_intel_enabled` (O8 chỉ nói "theo tenant"). Frontend đề xuất: `worktreeId` ở mọi lời gọi, `codeIntel.events.subscribe` (luồng khung `{channel, data}`), capability `code-intel.v1` trong `status.get` (CR-CV-050 2.1, 2.4).
3. **Phong bì 3.2 khác 05 §1** (`repo`, `worktreeId`, `devServerId`, `view` chỉ có ở 05); mirror theo hợp, trường thêm là tuỳ chọn.
4. **Triển khai preload Electron nằm ngoài `frontend/`** (`desktop/src/preload/index.ts`) và bản desktop đang lệch frontend (không có `mcp`). README v7 mục "Phạm vi" chỉ nêu `frontend/src/renderer/src`.
5. **`window.api` ở web là Proxy có đường lui**, nên không thể phát hiện tính năng bằng `typeof`; khác `mcpClient.isBridgeAvailable`.
6. **Lỗi qua `callRuntimeResult` ở web mất `error.code`/`data`**; cầu nối `codeIntel` phải trả phong bì (khác `mcp`).
7. **Kiểu thiếu trong 05/08**: `ReadingOrderItem` (`readingOrder[]`), phần tử `violations[]`, `riskReasons[]`, `ContainerRef` và kênh liệt kê container, `Component.origin`, `ModuleNode.area`/`symbolCount` tổng cho thư mục, `DataFlowSummary`, `ComponentRef`/`StoreRef`, cạnh trong `ImpactGraph` (`from` hoặc `edges`), kiểu `ReindexJob` và giá trị `mode`, dạng trả khi chưa có `review_states`.
8. **`ImpactGraph` (05 §2.5) không có cạnh** và `codeintel.impact` chỉ nhận một `target`; lens Ảnh hưởng hiển thị theo tầng nếu thiếu cạnh và không suy diễn (CR-CV-053).
9. **Trình xem diff không có API cuộn tới dòng**: nghiên cứu 10 §6.4 coi `startLine/endLine` của `SymbolRef` là đủ cho "mở diff ở dòng"; thực tế `pendingEditorReveal` chỉ dùng cho chế độ sửa. CR-CV-053 thêm `pendingDiffReveal` và phát vị trí con trỏ.
10. **Tab Review chạm ≥ 18 chỗ** (`TabContentType`, `WorkspaceVisibleTabType`, schema phiên, `tabs.ts`, `worktrees.ts`, `TabGroupPanel`, `TabBar`, …) chứ không chỉ hai union như nghiên cứu 10 §3 nêu.
11. **`AGENTS.md` trỏ `docs/STYLEGUIDE.md`** (không tồn tại); file thật là `guides/STYLEGUIDE.md` (đã dùng đường này trong series).
12. **Nghiên cứu 10 §1 nhắc `DAGPreview.tsx` hardcode hex**; `TaskDAGView.tsx` cũng hardcode (`STATUS_COLORS`), và v6 CR-REQ-018 sẽ sửa phần `backlog` ở file này; không dùng làm mẫu.
13. **Nghiên cứu 10 §9 đề xuất tên hook `use-code-intel-*.ts` (kebab)**; repo trộn `useXxx.ts` (đa số hook) và `use-xxx.ts`; nhóm này chọn `useCodeIntel*.ts`.

## Câu hỏi mở gộp (chi tiết ở mục 7 từng CR)

1. Desktop (Electron) có thuộc phạm vi MVP không, hay chỉ web? (CR-CV-050 7.6)
2. Hình dạng `readingOrder[]`, `violations[]`, `riskReasons[]` và quy tắc mang theo trạng thái đã xem khi `head` đổi. (CR-CV-051/052 7)
3. `ImpactGraph` có cạnh; `codeIntel.impact` có nhận nhiều `target` không. (CR-CV-053 7.1, 7.2)
4. `codeIntel.structure` có hỗ trợ `path`/`depth` và tổng cây con; `codeIntel.architecture` có trả `containers[]`; schema `c4.yaml`. (CR-CV-054/055 7)
5. `FlowSummary.id` (GitNexus) và `DataFlow.id` có chung không gian id không. (CR-CV-053 7.3, 056 7.2)
