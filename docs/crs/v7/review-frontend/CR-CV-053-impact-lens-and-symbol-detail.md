# CR-CV-053 — Lens Ảnh hưởng, panel chi tiết symbol, liên kết hai chiều với diff, chú giải lớp phủ thống nhất

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-053 |
| **Tên** | Lens Ảnh hưởng (`@xyflow/react`, bố cục tầng, không hex), panel chi tiết symbol (gọi bởi/gọi tới, test phủ, luồng), liên kết hai chiều đồ thị ↔ diff (`DiffViewer`/diff tab), chú giải và mã hoá lớp phủ dùng chung cho mọi lens |
| **Loại** | Feature + thay đổi nhỏ ở trình xem diff |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050, 051, 052; backend: CR-CV-002 (`impact`, `symbol`, `subgraph`), CR-CV-036 (`changeOverlay`), CR-CV-040 |
| **Mở khoá** | CR-CV-054, 055, 056 (dùng chú giải và liên kết diff), 057, 059, 060 (ghi chú gắn vào nút/dòng) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (mới), `components/editor/{DiffViewer,EditorContent,diff-viewer-props}` (thêm reveal và phát vị trí con trỏ), `store/slices/editor.ts` (trường `pendingDiffReveal`), `lib/review-diff-navigation.ts`, `lib/diff-cursor-line-bus.ts` (mới), `store/slices/review-ui.ts` (thêm `impactFocusKey`), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Lens Ảnh hưởng là lens mặc định: sau khi chọn một symbol đã đổi (từ Thứ tự đọc, CR-CV-052), người dùng thấy ai gọi nó, nó gọi ai, test nào phủ, luồng nào liên quan, rồi nhảy sang diff đúng dòng và ngược lại ([10 §2 nguyên tắc 2-3](../../../research/view-code/10-frontend-review-ux.md), [10 §5](../../../research/view-code/10-frontend-review-ux.md)).

Hiện trạng đã đọc (2026-10-05):

- **`@xyflow/react` 12 đã dùng** ở `components/workflow/DAGPreview.tsx` và `components/task/TaskDAGView.tsx` (bố cục "wave", `import '@xyflow/react/dist/style.css'`, test mock `ReactFlow` ở `components/task/__tests__/TaskDAGView.test.tsx`). **Cả hai hardcode màu hex** (`STATUS_COLORS` ở `TaskDAGView.tsx:19-27`), trái STYLEGUIDE; không sao chép.
- **Dữ liệu Impact theo 05 §2.5**: `ImpactGraph {target, direction, risk, levels:[{depth, symbols: ImpactSymbol[]}], affectedFlows, affectedClusters, testsCovering}`, `ImpactSymbol {symbol, via: EdgeKind, direct}`. **Không có danh sách cạnh** và không biết nút ở độ sâu 2 được tới từ nút nào ở độ sâu 1. `SymbolGraph` (05 §2.3) có cạnh `SymbolEdge {from, to, kind, confidence?, reason?, line?}`. Agent chỉ nhận **một** `target` mỗi lần `codeintel.impact` (README 3.2, ≤ 300 nút).
- **Diff viewer không có API cuộn tới dòng tuỳ ý.** `pendingEditorReveal {filePath, fileId?, line, column, matchLength}` chỉ được tiêu thụ ở chế độ sửa (`EditorContent.tsx:336-348` truyền `revealLine/Column/MatchLength` cho `MonacoEditor`; `MonacoEditor.tsx:769-779`). `DiffViewer` (`components/editor/DiffViewer.tsx`, 488 dòng) chỉ có cơ chế cuộn-tới-ghi-chú (`scrollToDiffCommentId` → `pendingScrollForThisViewer`, :95-133, hiện cuộn tới vùng bình luận) và tự cuộn tới thay đổi đầu tiên (:166-245). `DiffViewerProps` (`diff-viewer-props.ts`) không có thuộc tính reveal. `CombinedDiffViewer.tsx` dài 2 146 dòng, mỗi mục là `DiffSectionItem`.
- **Cách mở diff của Source Control** (dùng lại, `store/slices/editor.ts`): `openBranchDiff(worktreeId, worktreePath, entry: GitBranchChangeEntry, compare: BranchCompareLike, language, options)` (:536, :2627), `openDiff(worktreeId, filePath, relativePath, language, staged, options)` (:528), `openBranchAllDiffs`/`openAllDiffs` (:552, :592), id tab diff nhánh `${worktreeId}::diff::branch::${baseRef}::${compareVersion}::${path}`. `gitBranchChangesByWorktree` và `gitStatusByWorktree` cho biết file thuộc phần commit hay chưa commit.
- **Mẫu mở file tại dòng** đã kiểm chứng: `components/editor/check-annotation-open.ts` (`openAnnotationLocation`): `findWorktreeById` → `resolveAnnotationPathInsideWorktree(worktree.path, path)` (chặn thoát khỏi worktree) → `activateAndRevealWorktree` → `store.openFile({mode:'edit'}, {forceContentReload:true})` → hai khung `requestAnimationFrame` rồi `setPendingEditorReveal`. Ghi chú trong chính file: Monaco mount bất đồng bộ nên phải đợi hai khung.
- Nối đường dẫn đa nền tảng: `shared/cross-platform-path.ts` (`resolveRuntimePath`, `relativePathInsideRoot`). `SymbolRef.filePath` là đường dẫn **tương đối gốc repo trên dev server** (README 3.4); chuyển sang đường dẫn worktree ở UI.
- Token: chưa có `--review-*` (thêm ở CR-CV-050 2.10).
- `components/ui/`: `ToggleGroup`, `ContextMenu`, `Tooltip`, `Collapsible`, `ScrollArea`, `Button`, `Badge`, `Skeleton`.

## 2. Giải pháp đề xuất

### 2.1 Cây component và file (`components/review-map/`, mới)

```
ImpactLens                         (ReviewLensProps, CR-CV-051 2.6)
├─ ImpactToolbar                   (hướng: Ngược | Xuôi | Cả hai; độ sâu 1-3; Đồ thị | Danh sách; ReviewOverlayLegend)
├─ ImpactGraphCanvas               (@xyflow/react; lazy)
│    └─ ImpactSymbolNode           (nút tuỳ biến)
├─ ImpactColumnsList               (cùng dữ liệu dạng danh sách; trợ năng và đồ thị lớn)
└─ ImpactEmptyState / ImpactErrorState / ImpactLoadingStage
SymbolDetailPanel                  (drawer phải; 2.4)
├─ SymbolDetailHeader · SymbolDetailActions · SymbolRelationList (gọi bởi, gọi tới)
├─ SymbolCoveringTests · SymbolRelatedFlows · SymbolSourcePreview
ReviewOverlayLegend                (2.5)
review-overlay-model.ts  impact-column-layout.ts  review-diff-navigation.ts (lib/)  symbol-line-index.ts
```

### 2.2 Dữ liệu và hợp đồng (đề xuất)

- **Tâm của đồ thị** = `impactFocusKey` (trường mới của `ReviewUiState`, CR-CV-051 2.8): mặc định bằng `selectedSymbolKey` khi mục đó thuộc `changedSymbols`; "Đặt làm trung tâm" (nút trong panel và menu ngữ cảnh của nút) đặt lại. Chưa có tâm: trạng thái rỗng "Chọn một symbol đã đổi ở cột trái" (kèm gợi ý, không đoán).
- **Gọi mạng**: hai truy vấn song song `useCodeIntelQuery('impact', {target: key, direction:'upstream'|'downstream', depth})` (độ sâu mặc định 2, tối đa 3, giới hạn ≤ 300 nút mỗi hướng theo README 3.2). Chỉ gọi hướng đang bật. Một lần chọn tâm tốn tối đa hai lời gọi (~1,8 s mỗi lệnh CLI trên dev server, README mục 1); vì vậy hiển thị theo thang thời lượng (`usePerceivedLoadingStage`, CR-CV-051) và **không tự động gọi lại cho mọi symbol đã đổi**. Tổng quan toàn bộ thay đổi lấy từ `ChangeOverlay` (thanh tóm tắt), không từ `impact`.
- **Cạnh**: lens cần `ImpactSymbol.from?: string` (khoá của nút mà từ đó tới nút này) hoặc một mảng `edges: SymbolEdge[]` trong `ImpactGraph` (7.1). Nếu **không có**: vẽ cột không có đường nối, **không suy diễn** cạnh từ `depth` hay `via`, và hiển thị dòng "Dữ liệu chưa có thông tin cạnh; chỉ hiển thị theo tầng" ở thanh công cụ. Cạnh nối thẳng nút độ sâu 1 với tâm vẫn vẽ được vì `direct=true` (đã nói rõ trong 05 là "trực tiếp").
- **Lớp phủ**: `computeOverlayFlags(symbolKey, overlay, impact)` ở `review-overlay-model.ts` (2.5) dùng `ChangeOverlay.changedSymbols` (đã đổi), `uncoveredSymbols` (chưa test), `violations` (vi phạm), và "bị ảnh hưởng" = nút thuộc kết quả `impact` mà không thuộc `changedSymbols`.
- **Ambiguous**: `CODEINTEL_AMBIGUOUS_SYMBOL` mở `AmbiguousSymbolDialog` (CR-CV-051); chọn xong gọi lại với `uid` ứng viên.

### 2.3 Bố cục tầng và vẽ (`impact-column-layout.ts`, thuần, có test)

```
 gọi ngược (upstream)            tâm              gọi xuôi (downstream)
 d=-2        d=-1                d=0               d=+1         d=+2
 ┌────┐     ┌────┐             ┌══════┐           ┌────┐       ┌────┐
 │ f  │────▶│ g  │────────────▶║ SYM  ║──────────▶│ h  │──────▶│ i  │
 └────┘     └────┘             └══════┘           └────┘       └────┘
  ┄ nét đứt = chưa có test  ·  viền đậm = đã đổi  ·  viền vừa = bị ảnh hưởng
```

- `layoutImpactColumns(upstream, downstream, {rowHeight: 56, columnWidth: 260, columnGap: 80, maxPerColumn: 60})` trả `{nodes, edges, columns, hiddenCountByColumn}`: cột `-d` cho tầng `d` của hướng ngược, `0` cho tâm, `+d` cho hướng xuôi; trong cột sắp theo thư mục chứa file (hai đoạn cuối của `filePath`) rồi theo tên (ổn định, xác định); cột có hơn 60 nút hiển thị 60 và một nút "+N khác" mở rộng cột khi bấm (không cắt im lặng). Tổng ≤ 1 500 nút (README 3.2); vượt thì nhắc banner `truncated` (CR-CV-051) và dùng danh sách.
- `ImpactGraphCanvas`: `ReactFlow` với `nodesDraggable={false}`, `nodesConnectable={false}`, `onlyRenderVisibleElements`, `minZoom 0.2`, `fitView` khi đổi tâm, `colorMode` theo chủ đề ('light'/'dark'/'system' từ `settings.theme`), `Controls`, và `MiniMap` chỉ khi > 80 nút. Không bật `proOptions.hideAttribution`. Tải lười bằng `React.lazy`, nhập `@xyflow/react/dist/style.css` trong chính chunk lens (mẫu `TaskDAGView.tsx`).
- `ImpactSymbolNode`: tên (cắt giữa, đầy đủ ở tooltip), icon theo `kind` (`lucide-react`), `file:dòng` rút gọn, cờ lớp phủ (2.5), dấu "trực tiếp" nếu `direct`. Lớp: `border` + token; không hex. Chọn nút gọi `onSelectSymbol(ref)` (cập nhật `selectedSymbolKey`, drawer mở); nhấp đúp hoặc `Enter` trên nút gọi `onOpenDiff(ref)`. Menu ngữ cảnh (`ui/context-menu.tsx`): "Xem diff", "Đặt làm trung tâm", "Mở trong editor", "Sao chép khoá symbol".
- **Cạnh**: `smoothstep`, màu `var(--muted-foreground)`, độ dày 1; cạnh có `confidence < 0.8` nét đứt (05 §5); cạnh nối với nút đang chọn đậm hơn (`var(--foreground)`). Màu truyền dưới dạng chuỗi `var(--…)` trong `style` của cạnh/`MiniMap.nodeColor`; việc xyflow áp dụng `var()` đúng ở SVG chưa kiểm chứng (6).
- **Danh sách thay thế** (`ImpactColumnsList`): cùng dữ liệu, mỗi tầng là một nhóm `Collapsible` với hàng `button` (tên, `file:dòng`, cờ). Là đường truy cập cho trình đọc màn hình và là mặc định khi > 300 nút hiển thị (hoặc khi người dùng chọn). Bàn phím trong danh sách như CR-CV-052 (mũi tên, `Enter`).

### 2.4 Panel chi tiết symbol (`SymbolDetailPanel`)

Hiển thị trong `ReviewDetailDrawer` khi có `selectedSymbolKey`.

```
┌ ƒ CreateRequest · function ───────── [×] ┐
│ backend-go/…/usecase/create.go:42-97      │  ← bấm để sao chép; nút "Mở trong editor"
│ func (u *UseCase) Create(ctx, in) (…)     │  ← chữ ký (monospace, nhiều dòng cuộn)
│ ● đã đổi   ┄ chưa có test                 │
│ [Xem diff] [Đặt làm trung tâm]  (Ghi chú, Gửi cho agent: CR-CV-060)
├ Gọi bởi (4) ▾ ─────────────────────────── │
│  HandleCreate        grpc/handler.go:31   │
├ Gọi tới (6) ▸                              │
├ Test phủ (0) ▸  "Chưa tìm thấy test phủ trong index"
├ Luồng liên quan (2) ▸                      │
└ Mã nguồn ▸ (tải khi mở, ≤ 200 KiB)         ┘
```

- **Nguồn dữ liệu**: `codeIntel.symbol {uid | name+file}` → `SymbolDetail` (tên, `kind`, `qualifiedName`, `filePath`, `startLine`, `endLine`, `signature?`, `isExported`, `source?`, `language`); `codeIntel.subgraph {center, depth:1, kinds:['CALLS'], limit}` → `SymbolGraph` cho "Gọi bởi"/"Gọi tới" (tách theo hướng cạnh); `ImpactGraph.testsCovering` và `affectedFlows` từ kết quả `impact` của tâm nếu symbol đang chọn chính là tâm, nếu không từ một truy vấn `impact` riêng hướng ngược độ sâu 1 (tải lười khi mở mục "Test phủ"/"Luồng liên quan"). Mã nguồn **chỉ tải khi người dùng mở mục "Mã nguồn"** (README mục 6: mã chỉ trả khi mở một symbol cụ thể; không đưa vào cache dài hạn: bỏ khỏi `codeIntelResultsByWorktree` khi đóng panel).
- Mỗi danh sách tối đa 20 hàng rồi "Xem thêm" (mở rộng tại chỗ); mỗi hàng bấm để chọn symbol đó (đổi `selectedSymbolKey`, không đổi tâm).
- "Luồng liên quan": nhãn `FlowSummary.label` + `stepCount`; bấm gọi `setReviewLens('dataflow')` kèm `flowId` (CR-CV-056 xử lý id lạ bằng cách hiển thị danh sách; id của `FlowSummary` (GitNexus `Process`) và `DataFlow` (CR-CV-034) có khớp không chưa kiểm chứng, 7.3).
- "Mã nguồn": `<pre><code>` chỉ đọc có số dòng, tối đa 200 dòng hiển thị, ghi "Hiển thị 200/{n} dòng" và nút "Mở trong editor"; **không tô màu cú pháp** (không thêm thư viện; Monaco chỉ cho chế độ sửa/diff). Dữ liệu có thể chứa bí mật: phần che giá trị do backend (README mục 6), UI không tự giải mã/ghi log nội dung.
- Trạng thái: tải (theo thang), lỗi theo `kind` (CR-CV-050 bảng 2.3), symbol không có trong index ("Không tìm thấy symbol này trong index; có thể index cũ"), `ambiguous` (hộp chọn). Panel không đóng khi lỗi từng mục; mỗi mục có lỗi riêng và nút thử lại.
- `Esc` đóng drawer; chuỗi qua `translate()`; hành động "Ghi chú" và "Gửi cho agent" là **khe** (`SymbolDetailActionsSlot`) do CR-CV-060 điền, không có nút giả ở CR này.

### 2.5 Chú giải và mã hoá lớp phủ thống nhất (`review-overlay-model.ts`, `ReviewOverlayLegend`)

Một nguồn duy nhất cho mọi lens (Ảnh hưởng, Cấu trúc, Kiến trúc, Luồng, ERD; cột trái CR-CV-052), để người dùng học một lần ([10 §6.3](../../../research/view-code/10-frontend-review-ux.md)):

| Cờ | Ý nghĩa | Màu (token, CR-CV-050 2.10) | Dấu **không phải màu** |
|---|---|---|---|
| `changed` | Đã đổi trong phạm vi | `--review-changed` | Viền đặc 2 px + chấm `CircleDot` góc trên trái |
| `affected` | Bị ảnh hưởng, chưa đổi | `--review-affected` | Viền đặc 1 px |
| `untested` | Chưa tìm thấy test phủ | `--review-untested` | **Nét đứt** (giữ màu/độ dày của cờ đổi hoặc ảnh hưởng; nút không có cờ nào khác dùng viền `border`) |
| `violation` | Vi phạm lớp | `--review-violation` | Icon `TriangleAlert` ở góc dưới phải |

- `OVERLAY_ENCODING: Record<OverlayFlag, {labelKey; tokenVar; icon; strokeWidth; dashed}>` là bảng duy nhất; `ReviewOverlayLegend` được **dựng từ chính bảng này** (không thể lệch). `overlayClassNames(flags)` trả lớp Tailwind dạng `border-[color:var(--review-changed)]` cho HTML; `overlaySvgProps(flags)` trả `{stroke:'var(--review-changed)', strokeWidth, strokeDasharray}` cho SVG (treemap ở CR-CV-054). Tổ hợp: màu/độ dày theo ưu tiên `changed` > `affected`; `untested` chỉ thêm nét đứt; `violation` thêm icon.
- Chú giải là một hàng gọn ở thanh công cụ của lens, thu gọn được; swatch `aria-hidden`, nhãn là chữ. Mỗi cờ có `Tooltip` giải nghĩa. Hiển thị chỉ các cờ có dữ liệu thật (không có `violations` thì không vẽ mục vi phạm).
- Không khẳng định điều chưa biết: "Chưa tìm thấy test phủ trong index" thay vì "không có test".

### 2.6 Liên kết hai chiều với diff

**Đồ thị → diff** (`lib/review-diff-navigation.ts`, mới): `openReviewDiffAtSymbol(worktreeId, ref: SymbolRef, scope)`.

1. `worktree = findWorktreeById(...)`; `resolveAnnotationPathInsideWorktree(worktree.path, ref.filePath)` (chặn đường dẫn thoát worktree; nếu null thì thôi, thông báo lỗi inline). Dùng `resolveRuntimePath` cho nối đường dẫn.
2. Chọn bộ mở **theo nơi thay đổi nằm**: file có trong `gitBranchChangesByWorktree[worktreeId]` → `openBranchDiff` (dùng `GitBranchCompareSummary` của phạm vi `branch`); nếu chỉ có trong `gitStatusByWorktree` (chưa commit) → `openDiff(…, staged:false)`; phạm vi `range`/`hostedReview` → `openBranchDiff` với `compare` tổng hợp `{baseRef: baseCommit, baseOid: baseCommit, headOid: headCommit, mergeBase: baseCommit}` (**chưa kiểm chứng** `git.branchDiff` chấp nhận compare tổng hợp, 7.4). File không có trong cả hai: mở bản `openFile` tại dòng theo mẫu `openAnnotationLocation` và ghi "File không nằm trong thay đổi; đang mở bản hiện tại".
3. Sau khi mở, đọc `activeFileId` mới của store rồi `setPendingDiffReveal({fileId, line: ref.startLine ?? 1, side:'modified', nonce})` sau hai khung `requestAnimationFrame` (cùng lý do với `openAnnotationLocation`). Mở diff **trong nhóm tab đang hoạt động** như Source Control; có nhóm tách bên phải đã tồn tại thì dùng nó (`findReusableRightSplitGroupId`, như `ensure-simulator-tab.ts`) để Review vẫn hiển thị; không tự tạo split ở CR này (7.5).
4. "Mở trong editor" (chế độ sửa) dùng đúng mẫu `openAnnotationLocation` (`setPendingEditorReveal`).
5. Số dòng của `SymbolRef` là theo commit mà index đã quét; khi `IndexStatus.stale` hoặc `indexedCommit !== headCommit`, dòng có thể lệch. Panel hiển thị (không dùng toast) dòng nhỏ "Số dòng theo index tại {commit}; có thể lệch" cạnh nút "Xem diff" (thông tin, không chặn).

**Thay đổi nhỏ ở trình xem diff** (CR này sở hữu):

- `store/slices/editor.ts`: thêm `pendingDiffReveal: {fileId: string; line: number; side: 'original'|'modified'; nonce: number} | null` và `setPendingDiffReveal`, mô phỏng `pendingEditorReveal` (:740-741). Giá trị này **không** thuộc `OpenFile`, không lưu phiên.
- `diff-viewer-props.ts`: thêm `revealLine?: number`, `revealSide?: 'original'|'modified'`, `revealNonce?: number`. `EditorContent.tsx` (:963-981) truyền khi `pendingDiffReveal.fileId === activeFile.id`, rồi xoá `pendingDiffReveal` sau khi đã áp dụng.
- `DiffViewer.tsx`: logic trong hook mới `components/editor/use-diff-line-reveal.ts`: khi `revealNonce` đổi và Monaco đã gắn (`modifiedEditor` có), gọi `editor.revealLineInCenter(line)` (+ `setPosition`) trên editor của `side`; nếu chưa gắn thì giữ lại tới khi gắn (`handleMount` :318-383). **Tương tác với cuộn tự động tới thay đổi đầu tiên** (:166-245): thêm `revealNonce` vào điều kiện "một lần mỗi mount" để cuộn tới dòng có ưu tiên, giống cách `pendingScrollForThisViewer` nhường chỗ.
- `CombinedDiffViewer` **ngoài phạm vi** CR này (2 146 dòng, ảo hoá theo mục): Review luôn mở diff **một file** (`openBranchDiff`/`openDiff`); nút "Xem tất cả diff" gọi `openBranchAllDiffs` không kèm dòng. Ghi lại để mở rộng sau (7.6).

**Diff → đồ thị**: `lib/diff-cursor-line-bus.ts` (mới, mẫu `mcp-event-bus.ts`): `subscribeDiffCursorLine(listener)`, `emitDiffCursorLine({worktreeId, relativePath, line, side})`, `hasDiffCursorListeners()`. `DiffViewer` đăng ký `modifiedEditor.onDidChangeCursorPosition` và phát sự kiện (debounce 150 ms) **chỉ khi** `hasDiffCursorListeners()` (không tốn khi không có Review mở); hook `useDiffCursorEmitter` đặt cạnh `use-diff-line-reveal.ts`. `ReviewWorkspace` đăng ký khi đang gắn: `findInnermostSymbolAtLine(index, relativePath, line)` (`symbol-line-index.ts`, thuần: chỉ mục khoảng `[startLine,endLine]` theo file từ `changedSymbols` và nút `impact`; chọn symbol có khoảng hẹp nhất chứa dòng) → `setReviewSelectedSymbol(key)` kèm nguồn `'diff'`: lens làm nổi nút và `fitView`/cuộn vào khung nhìn, **không** mở drawer và **không** giành tiêu điểm. Chống vòng: sau khi `openReviewDiffAtSymbol` đặt reveal, bỏ qua sự kiện con trỏ 500 ms (`suppressDiffCursorUntil`). Không có symbol chứa dòng thì giữ nguyên lựa chọn (không xoá).

### 2.7 Trạng thái và lỗi của lens

| Tình huống | Hiển thị |
|---|---|
| Chưa có tâm | Rỗng: "Chọn một symbol đã đổi ở cột trái" |
| Đang tải | `ImpactLoadingStage` theo thang thời lượng (nhãn "Đang tính ảnh hưởng…" ≥ 3 s) |
| Kết quả rỗng | "Không tìm thấy phụ thuộc cho symbol này trong index" (không "không ảnh hưởng gì") |
| `truncated` (≤ 300 nút/hướng) | Banner CR-CV-051 "{shown}/{total}" + gợi ý giảm độ sâu |
| `index-missing`, `tool-unavailable`, `offline`... | Do `ReviewViewStateScreen` (CR-CV-051) vì ở cấp khung; lens chỉ xử lý lỗi riêng của truy vấn `impact` (`timeout`, `too-large`, `tool-failed`) bằng trạng thái lỗi inline có "Thử lại" |
| Symbol là file/kind không có trong đồ thị | "Symbol loại này không có trong đồ thị gọi" |

### 2.8 Hiệu năng

- Tính bố cục bằng `useMemo` theo `(upstream, downstream, expandedColumns)`; hàm thuần, độ phức tạp O(n log n) do sắp xếp cột.
- Ngân sách: ≤ 1 500 nút; khung hình mượt chưa đo. `onlyRenderVisibleElements` chỉ vẽ phần trong khung nhìn. Dùng `memo` cho `ImpactSymbolNode`; mọi hàm `onSelect` ổn định.
- Hủy truy vấn khi đổi tâm (CR-CV-050 hook huỷ khi gỡ, bỏ kết quả cũ).

## 3. Quyết định thiết kế

- **Một tâm mỗi lần**, không tổng hợp mọi symbol đã đổi vào một đồ thị: `codeintel.impact` chỉ nhận một `target` và chi phí ~1,8 s mỗi lệnh. Tổng quan do `ChangeOverlay`.
- **Không suy diễn cạnh**: thiếu `from`/`edges` thì vẽ theo tầng và nói rõ.
- **Danh sách thay thế luôn có** (đồ thị khó cho trình đọc màn hình và quá lớn thì không đọc nổi).
- **Mã hoá lớp phủ = màu + dấu hình học**, nguồn duy nhất, dùng chung mọi lens.
- **Diff một file, không combined**, để giảm phạm vi sửa vào `CombinedDiffViewer`.
- **`pendingDiffReveal` mô phỏng `pendingEditorReveal`** (mẫu đã chạy) thay vì gọi trực tiếp Monaco từ Review.
- **Bus con trỏ chỉ phát khi có người nghe** để không tốn ở diff thông thường.
- Không nhúng Monaco vào panel chi tiết; không thêm thư viện tô cú pháp.

## 4. Tiêu chí chấp nhận

- [ ] Chọn một symbol đã đổi ở Thứ tự đọc: lens gọi `impact` đúng hướng/độ sâu và vẽ tâm ở giữa, hướng ngược bên trái, xuôi bên phải; đổi hướng/độ sâu gọi lại.
- [ ] `ImpactGraph` không có cạnh: hiển thị theo tầng, không có đường nối giả, có dòng giải thích; có `from`/`edges`: vẽ đúng cạnh, cạnh `confidence < 0.8` nét đứt.
- [ ] Cột > 60 nút hiển thị 60 + nút "+N khác" mở rộng được; > 300 nút mặc định danh sách.
- [ ] Mã hoá lớp phủ đúng bảng 2.5 ở nút, hàng danh sách và chú giải; chú giải dựng từ cùng bảng; không dùng màu làm dấu hiệu duy nhất.
- [ ] Panel chi tiết hiển thị tên, `file:dòng`, chữ ký, gọi bởi/gọi tới (≤ 20 + xem thêm), test phủ, luồng; mã nguồn chỉ tải khi mở mục và không giữ lại khi đóng; `Esc` đóng.
- [ ] "Xem diff" ở file nằm trong thay đổi nhánh mở `openBranchDiff`; file chỉ chưa commit mở `openDiff`; diff mở xong cuộn tới đúng dòng `startLine` (kể cả khi Monaco mount sau).
- [ ] "Mở trong editor" mở file ở chế độ sửa tại dòng, không thoát khỏi worktree (đường dẫn như `../x` bị chặn).
- [ ] Con trỏ trong diff đổi sang dòng thuộc symbol đã biết thì nút tương ứng được làm nổi và đưa vào khung nhìn mà không mở drawer, không giành tiêu điểm; không tạo vòng với reveal vừa phát.
- [ ] Không có Review mở thì `DiffViewer` không phát sự kiện con trỏ (không listener, không debounce timer).
- [ ] Index cũ: hiển thị dòng "Số dòng theo index tại {commit}; có thể lệch".
- [ ] Trạng thái rỗng/lỗi/tải theo 2.7; `ambiguous` mở hộp chọn.
- [ ] Không hex, không lớp màu Tailwind thô trong file mới; `prefers-reduced-motion`: không `fitView` có hoạt ảnh (`duration: 0`); chuỗi qua `translate()` đủ 5 locale; chạy ở Electron và web.
- [ ] Không thêm thư viện; `DiffViewer`/`EditorContent` thay đổi không làm hỏng test hiện có.

## 5. Kiểm thử

- `impact-column-layout.test.ts`: chỉ số cột, sắp ổn định, giới hạn 60/cột, thiếu cạnh, trùng nút giữa hai hướng, 1 500 nút không ném.
- `review-overlay-model.test.ts`: tổ hợp cờ, `overlayClassNames`/`overlaySvgProps` không chứa hex (so khớp `/#[0-9a-f]{3,8}/i` trả rỗng), chú giải sinh từ cùng bảng.
- `symbol-line-index.test.ts`: symbol hẹp nhất chứa dòng, dòng ngoài mọi khoảng, nhiều file.
- `review-diff-navigation.test.ts` (store giả như `store-test-helpers.ts`): chọn `openBranchDiff` so với `openDiff` so với `openFile`; chặn đường dẫn thoát; compare tổng hợp cho `range`; `pendingDiffReveal` được đặt sau hai khung (`requestAnimationFrame` giả).
- `use-diff-line-reveal.test.ts`: editor giả `{revealLineInCenter, setPosition}`; hoãn tới khi gắn; nhường cuộn tự động; `nonce` mới mới áp dụng lại.
- `diff-cursor-line-bus.test.ts`: không phát khi không listener; debounce; `suppressDiffCursorUntil`.
- `ImpactLens.test.tsx`, `SymbolDetailPanel.test.tsx`, `ReviewOverlayLegend.test.tsx` (`// @vitest-environment happy-dom`; mock `@xyflow/react` như `TaskDAGView.test.tsx` vì cần kích thước thật và `ResizeObserver`): hiển thị tâm và cột, chọn nút gọi `onSelectSymbol`, lỗi từng mục không đóng panel, mã nguồn chỉ tải khi mở.
- `i18n/code-intel-locale-coverage.test.ts` mở rộng. Dùng `code-intel-fake-backend.ts`.
- Không có test ảnh chụp trong repo; nên kiểm tay `var()` trong SVG xyflow ở sáng/tối trước khi merge.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Không có cạnh trong `ImpactGraph`** (05 §2.5): đồ thị có thể hiển thị kém hữu ích nếu backend không bổ sung. Đây là điểm chặn chính của lens.
- Xyflow có áp dụng đúng chuỗi `var(--token)` cho `stroke` cạnh và `MiniMap.nodeColor` không (thuộc tính SVG/`style`) chưa kiểm chứng; mẫu hiện tại trong repo dùng hex.
- Sửa `DiffViewer`/`EditorContent` (nhiều người dùng chung; có Monaco, `diffViewStateCache`, `didAutoScrollFirstDiffRef`) dễ hồi quy; cần chạy đủ test diff hiện có.
- Số dòng index với HEAD/cây làm việc có thể lệch; diff modified side là cây làm việc hay HEAD tuỳ bộ mở.
- Hành vi `git.branchDiff` với compare tổng hợp cho khoảng commit/review đã gửi chưa kiểm chứng.
- Hai lời gọi `impact` song song qua SSH có thể chậm; chưa đo ngân sách.
- Nút `Space`/`Enter` trong xyflow (nút có thể focus) và xung đột với phím danh sách chưa kiểm chứng.

## 7. Câu hỏi mở

1. `ImpactGraph` có thể thêm `edges: SymbolEdge[]` (hoặc `ImpactSymbol.from`) không? Quyết định chất lượng lens (CR-CV-002/036).
2. `codeIntel.impact` có nhận nhiều `target` hoặc tập thay đổi (để vẽ tổng quan) không?
3. `FlowSummary.id` (GitNexus `Process`) và `DataFlow.id` (CR-CV-034) có chung không gian id không (liên kết "Luồng liên quan" → lens Luồng)?
4. `git.branchDiff`/`commitDiff` chấp nhận compare tổng hợp cho khoảng commit và review đã gửi?
5. Có tự tạo split bên phải khi mở diff từ Review (như `placement:'rightSplit'` của simulator) không? Mặc định: không.
6. Có mở rộng `CombinedDiffViewer` để cuộn tới dòng không? Đề xuất sau MVP.
7. Có cần phím tắt "Xem diff" trong lens (ví dụ `d`) ngoài `Enter`/nhấp đúp?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.2, 3.4, mục 6), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§2, §5, §6.3-6.4, §9), `05-graph-schemas.md` (§2.3, §2.5, §2.7, §3, §5), `08-views-and-review-models.md` (R1, R5)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (Color roles, Git decoration colors, "Don't invent color", Animation, Scrollbars)
- `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskDAGView.tsx` (:19-27 hex không dùng lại), `components/workflow/DAGPreview.tsx`, `components/task/__tests__/TaskDAGView.test.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/{DiffViewer.tsx,diff-viewer-props.ts,EditorContent.tsx,MonacoEditor.tsx,check-annotation-open.ts,check-annotation-path.ts,CombinedDiffViewer.tsx}`
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/editor.ts` (:528-552, :592, :696, :740-741, :2627), `store/slices/worktree-helpers.ts`, `lib/ensure-simulator-tab.ts`, `lib/emulator-right-split-target.ts`, `lib/mcp-event-bus.ts`, `lib/worktree-activation`
- `/opt/repos/orca/frontend/src/shared/cross-platform-path.ts`, `shared/types.ts` (`GitBranchCompareSummary`)
- Mới: `components/review-map/{ImpactLens,ImpactGraphCanvas,ImpactSymbolNode,ImpactColumnsList,SymbolDetailPanel,ReviewOverlayLegend}.tsx`, `impact-column-layout.ts`, `review-overlay-model.ts`, `symbol-line-index.ts`, `lib/review-diff-navigation.ts`, `lib/diff-cursor-line-bus.ts`, `components/editor/{use-diff-line-reveal,useDiffCursorEmitter}.ts`
