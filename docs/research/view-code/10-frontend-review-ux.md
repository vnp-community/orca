# 10 — Đề xuất giao diện để review kết quả sau khi agent code

Mục tiêu: sau khi agent code xong, người dùng nhìn **thay đổi → ảnh hưởng → có ổn không** trong 1–2 thao tác, rồi đi sâu vào code hoặc phản hồi lại cho agent. Dùng dữ liệu của [05](./05-graph-schemas.md) và [08](./08-views-and-review-models.md).

Trạng thái: đề xuất (2026-10-05). Phần "Hiện trạng" đã đối chiếu `frontend/src/renderer/src/**`, `specs/frontend/tdd/v5/*` và `guides/STYLEGUIDE.md`.

## 1. Hiện trạng frontend (đã kiểm tra)

**Stack** (`frontend/package.json`): React 19, Zustand 5, Tailwind 4, shadcn/Radix, lucide-react, `@xyflow/react` 12, `mermaid` 11, `monaco-editor`, `@tanstack/react-virtual`, `react-resizable-panels`. Hai render target dùng chung `App.tsx`: Electron (`window.api` từ preload) và web (`web-preload-api`, nói WS `/ws` với `api-gateway`). Mọi tính năng mới phải chạy được cả hai.

**Bố cục** (`specs/frontend/tdd/v5/05-ui-components.md`, `App.tsx`): titlebar, left sidebar (workspace/worktree), khu vực chính có **tab group** (terminal, editor, browser, simulator; PR/task/native-chat), right sidebar có activity bar, các lớp phủ toàn cục (Quick Open, `WorktreeJumpPalette` Cmd+K).

**Right sidebar** (`store/right-sidebar-route.ts`): các tab `explorer`, `vault`, `workspaces`, `pr-checks`, `source-control`, `checks`, `ports`. Thêm tab mới cần sửa `ActiveRightSidebarTab` (`shared/types.ts`), `normalizeRightSidebarRoute` và `activity-bar-buttons.tsx`.

**Đã có cho review code** (nên tái dùng):
- `components/editor/`: `CombinedDiffViewer` (diff nhiều file liên tục) + `CombinedDiffFileTree`, `DiffViewer` (Monaco), `ChangesModeView`, `CheckRunDetailsPanel`.
- `components/diff-comments/`: bình luận theo dòng + `DiffNotesSendMenu` (gửi ghi chú cho agent).
- `components/right-sidebar/`: `SourceControl`, `ChecksPanel`; `PullRequestPage`.
- `components/dashboard/`: danh sách agent và trạng thái (`DashboardAgentRow`), `agent-status` slice.
- `components/workflow/DAGPreview.tsx`, `components/task/TaskDAGView.tsx`: đã dùng `@xyflow/react` (layout theo "wave").
- `components/editor/MermaidBlock.tsx`: render Mermaid (tải lazy).

**Không dùng**: `components/code-review/*` — file ghi rõ là code chết, không có nơi gọi, đã bị thay bằng `diff-comments`.

**Cần lưu ý**: `DAGPreview.tsx` hardcode mã màu hex, trái [STYLEGUIDE](../../../guides/STYLEGUIDE.md) ("never hardcode a hex"); không sao chép. Spec ghi React 18, `package.json` ghi React 19 (spec cũ). `AGENTS.md` trỏ `docs/STYLEGUIDE.md` nhưng file thật nằm ở `guides/STYLEGUIDE.md`.

## 2. Nguyên tắc thiết kế

1. **Bắt đầu từ thay đổi, không từ đồ thị.** Màn đầu là tóm tắt thay đổi và rủi ro; đồ thị là công cụ để trả lời, không phải điểm bắt đầu.
2. **Bốn cấp, mỗi cấp một cú bấm**: Tóm tắt → Bản đồ (đồ thị) → Chi tiết symbol → Mã (diff).
3. **Hai chiều giữa đồ thị và diff**: bấm nút → mở diff đúng dòng; bấm dòng diff → làm nổi nút.
4. **Review phải khép vòng**: phát hiện → ghi chú → gửi lại agent (dùng luồng `diff-comments`/`DiffNotesSendMenu`).
5. **Tuân [STYLEGUIDE](../../../guides/STYLEGUIDE.md)**: chỉ dùng token `main.css`, primitive shadcn, icon lucide, màu chỉ để biểu thị trạng thái; sáng/tối; reduced-motion; mọi chuỗi qua `translate()` (i18n).

## 3. Điểm vào và vị trí

| Điểm vào | Hành vi |
|---|---|
| Agent báo xong (hàng trong `DashboardAgentRow`, thông báo) | Nút **Review** → mở tab Review của đúng worktree |
| `SourceControl` (cạnh các nút commit/PR) | Nút/menu **Review changes** |
| Cmd+K (`WorktreeJumpPalette`/command surface) | Lệnh "Review changes", "Open architecture map", "Open ERD" |
| Right sidebar | Tab **Review** hiển thị tóm tắt gọn (số file/symbol/luồng/bảng, rủi ro) + nút mở đầy đủ |

**Vị trí chính: một tab trong tab group** (loại mới `review`, như `browser`/`editor`), không nhét hết vào right sidebar vì đồ thị cần chỗ rộng và có thể chia cột cạnh terminal của agent. Right sidebar chỉ làm lối vào và tóm tắt. Thêm loại tab mới đụng `WorkspaceVisibleTabType` (`shared/types.ts`), tab-bar và nhiều chỗ khác (khảo sát lúc soạn CR v7 ghi ít nhất 18 chỗ, xem `docs/crs/v7/review-frontend/CR-CV-050`); cần chốt (xem §10).

## 4. Bố cục màn hình Review

```
┌ Review · worktree feat/x · so với main ─ [Phạm vi ▾] [Index: d819812 · 2 phút trước ✓] [Rủi ro: MEDIUM] [↻] ┐
├──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Tóm tắt:  12 file · 38 symbol · 3 luồng · 2 bảng · 1 hợp đồng · 5 chưa có test        (mỗi chip = bộ lọc) │
├──────────────┬──────────────────────────────────────────────────────────────┬────────────────────────────┤
│ Thứ tự đọc   │ [Ảnh hưởng] Kiến trúc  Luồng  ERD  Lưu trữ  Cấu trúc  Hợp đồng │ Chi tiết nút đang chọn     │
│ ☐ 1 usecase… │                                                              │  tên · file:dòng · chữ ký  │
│ ☐ 2 adapter… │            (đồ thị / sơ đồ của "lens" đang chọn)             │  gọi bởi / gọi tới         │
│ ☑ 3 proto…   │                                                              │  test phủ · luồng liên quan│
│ …            │   chú giải: ▣ đã đổi  ◻ bị ảnh hưởng  ┄ chưa có test        │  [Xem diff] [Ghi chú]      │
│ 4/12 đã xem  │                                                              │  [Gửi cho agent]           │
└──────────────┴──────────────────────────────────────────────────────────────┴────────────────────────────┘
```

- **Khung ba cột** dùng `react-resizable-panels` (đã có); cột phải có thể thu gọn thành drawer. Cột trái và phải mở/đóng bằng phím tắt.
- **Phạm vi**: thay đổi chưa commit so với base · khoảng commit · PR. Mặc định: toàn bộ thay đổi của worktree so với nhánh gốc.
- **Chip chỉ báo index** (`IndexStatus` [05 §2.8]): hiển thị `indexedAt`, `stale`; nút làm mới (cần quyền, có tiến trình).
- Mở diff dùng lại `CombinedDiffViewer`/`DiffViewer`, không viết mới.

## 5. Các lens (cách vẽ)

| Lens | Dữ liệu | Cách vẽ | Ghi chú |
|---|---|---|---|
| **Ảnh hưởng** (mặc định) | `ImpactGraph` + `ChangeOverlay` | `@xyflow/react`, bố cục theo tầng độ sâu (trái → phải = gọi ngược → xuôi) | Nút đã đổi nhấn mạnh bằng viền accent; nút bị ảnh hưởng viền muted; nét đứt = chưa có test |
| **Kiến trúc** (C4 L3) | `C4ComponentView` ([08 §3]) | xyflow, nút = component, nhóm theo container; cạnh có trọng số | Chọn container ở bộ chọn đầu trang; một diagram mỗi container |
| **Luồng** | `DataFlow` ([08 §4]) | Mermaid sequence (tái dùng `MermaidBlock`/`mermaid-config`) + danh sách bước; có thể sao chép/xuất | Bước đã đổi tô nổi; bấm bước → symbol |
| **ERD** | `ErdModel` ([08 §5]) | xyflow, nút = bảng (cột, PK/FK); nét đứt = liên kết logic giữa service | Cột thêm/xoá/đổi kiểu tô theo diff migration |
| **Lưu trữ** | `StorageMap` ([08 §6]) | xyflow: service → kho → topic → secret | Chỉ đọc; đánh dấu thành phần bị đổi |
| **Cấu trúc** | `ModuleGraph` ([05 §2.2]) | Treemap/sunburst bằng SVG tự viết (không có thư viện sẵn); cây thư mục ảo hoá (`react-virtual`) | Kích thước = số symbol; màu = khu vực |
| **Hợp đồng** | proto/route/kênh/schema diff ([08 R4]) | Bảng hai cột trước/sau, đánh dấu phá vỡ tương thích | Mỗi dòng liên kết tới diff |

Giới hạn hiển thị: đồ thị ≤ ~1 500 nút ([04 §2](./04-raw-data-and-pipeline.md)); `onlyRenderVisibleElements` của xyflow; khi cắt, hiển thị "đang hiển thị 1 500 / 9 039 — thu hẹp phạm vi" kèm bộ lọc, không cắt im lặng.

## 6. Các tính năng giúp review nhanh

1. **Thẻ tóm tắt thay đổi + rủi ro** (đầu màn): mỗi chip là bộ lọc; bấm "5 chưa có test" lọc ngay danh sách và đồ thị. Nhãn rủi ro phải kèm lý do bấm xem được (ví dụ "chạm 3 luồng cross-community").
2. **Thứ tự đọc** (cột trái): từ `readingOrder[]` ([08 R6]), nhóm theo component, đánh dấu đã xem, tiến độ `4/12`. Phím: `j/k` chuyển mục, `Enter` mở diff, `Space` đánh dấu đã xem.
3. **Chú giải và lớp phủ nhất quán** giữa mọi lens (đổi / bị ảnh hưởng / chưa test / vi phạm lớp), để người dùng học một lần.
4. **Hai chiều đồ thị ↔ diff**: bấm nút → mở diff ở dòng; con trỏ ở dòng diff → nút được làm nổi (qua `startLine/endLine` của `SymbolRef`).
5. **Ghi chú và gửi lại agent**: ghi chú gắn vào nút hoặc dòng diff, dùng `diff-comments` hiện có; menu "Gửi cho agent" dùng `DiffNotesSendMenu`. Cho phép gửi một lô ghi chú.
6. **So sánh theo phiên bản**: chuyển giữa "agent lượt trước" và "lượt này" (cùng worktree), để thấy phần agent vừa sửa theo phản hồi.
7. **Cảnh báo trực tiếp trên đồ thị**: vi phạm lớp (`usecase` import `adapter`), vòng phụ thuộc, truy vấn thiếu `tenant_id`, hotspot ([08 R2/R3/R7]) — hiển thị như danh sách "Phát hiện" ở đáy, mỗi dòng có "Bỏ qua/Đã xử lý".
8. **Trạng thái review lưu theo worktree + commit** (đã xem, ghi chú) để quay lại không mất; nơi lưu chọn theo `specs/frontend/storage/*` (chưa đọc, xem §10).

## 7. Trạng thái và lỗi (theo rubric của STYLEGUIDE)

| Tình huống | Hiển thị |
|---|---|
| Đang lấy dữ liệu | <100 ms không đổi; 100 ms–1 s chỉ vô hiệu hoá; ≥1 s spinner/nhãn; ≥3 s hiển thị giai đoạn ("Đang truy vấn index…"). Dev server qua SSH: trì hoãn hiển thị tải ~200 ms, vẫn khoá nút ngay |
| Chưa có index | Trạng thái rỗng có hành động trực tiếp: "Lập chỉ mục" (nếu có quyền) kèm liên kết hướng dẫn cài GitNexus/CodeGraph |
| Index cũ (`stale`) | Banner inline, vẫn hiển thị dữ liệu, ghi `indexedCommit` và `headCommit`; nút làm mới |
| Công cụ thiếu trên dev server (`TOOL_UNAVAILABLE`) | Lỗi persistent inline có hướng xử lý, không dùng toast |
| Dev server mất kết nối | Banner/inline riêng trong màn Review (`ConnectionStatusBanner` chỉ có ở web và dùng mã hex nên không dùng lại được); giữ dữ liệu cache, đánh dấu cũ |
| Symbol mơ hồ (`ambiguous`) | Danh sách ứng viên để chọn (command surface có tìm kiếm) |
| Bị cắt (`truncated`) | Thông báo số lượng + gợi ý lọc |

Văn bản giao diện không được khẳng định điều chưa xác thực (ví dụ không ghi "an toàn" khi chỉ mới "không phát hiện vấn đề trong phạm vi index").

## 8. Phím tắt và tốc độ

- Từ khi agent báo xong tới thấy tác động: **1 cú bấm** (Review) → lens Ảnh hưởng đã mở với thay đổi được chọn sẵn.
- Command surface (Cmd+K) có tìm kiếm: nhảy tới symbol, lens, component, bảng.
- Phím theo nền tảng (`metaKey` trên Mac, `ctrlKey` nơi khác); chỉ hiển thị chip phím khi đã cài đặt thật.
- Focus mặc định: danh sách "Thứ tự đọc"; `Esc` đóng drawer chi tiết, không thêm trang trí cho nút Hủy.

## 9. Kỹ thuật phía frontend

- **Truy cập backend**: thêm namespace `window.api.codeIntel` ở cả preload desktop và `web-preload-api`, gọi các kênh `codeIntel.*` ([03 §4](./03-command-and-data-flow.md)); nhận push `codeIntel.changed`, `codeIntel.reindexProgress`.
- **State**: một slice Zustand riêng (theo mẫu `store/slices/*`), khoá theo `(worktreeId, commit, lens)`; có bộ test chống rò rỉ khi xoá worktree như các slice khác.
- **Layout đồ thị**: xyflow không tự bố cục; MVP dùng bố cục tầng đơn giản như `DAGPreview` (nhưng dùng token màu, không hex). Nếu cần bố cục tốt hơn thì thêm `elkjs`/`dagre` (phụ thuộc mới, cần duyệt).
- **Màu node/edge**: ánh xạ sang biến CSS trong `main.css`; nếu thiếu token (ví dụ trạng thái "bị ảnh hưởng"), thêm vào `:root` và `.dark` rồi bind trong `@theme inline`.
- **Tên file**: không dùng tên mơ hồ (`helpers`, `utils`…). Đề xuất thư mục `components/review-map/`:
  `ReviewWorkspace.tsx`, `ReviewSummaryBar.tsx`, `ReadingOrderList.tsx`, `ReviewLensTabs.tsx`, `ImpactLens.tsx`, `ArchitectureLens.tsx`, `DataFlowLens.tsx`, `ErdLens.tsx`, `StorageLens.tsx`, `StructureLens.tsx`, `ContractDiffLens.tsx`, `SymbolDetailPanel.tsx`, `IndexFreshnessChip.tsx`; hook `use-code-intel-*.ts`; slice `store/slices/code-intel.ts`.
- **Mobile**: đã có `MobileDiffReviewScreenView`; giai đoạn đầu chỉ cần màn tóm tắt chỉ-đọc (số liệu + danh sách phát hiện), không vẽ đồ thị.

## 10. Phân kỳ và câu hỏi mở

**Phân kỳ (khớp [08 §8](./08-views-and-review-models.md))**
1. Khung Review + tóm tắt + Thứ tự đọc + lens Ảnh hưởng + Cấu trúc; liên kết diff hai chiều; chip index.
2. Ghi chú → gửi agent; lưu trạng thái đã xem; lens ERD.
3. Lens Luồng (Mermaid) và Kiến trúc (C4) cho 2–3 service đầu tiên.
4. Lens Lưu trữ, Hợp đồng, danh sách Phát hiện (R2/R3/R7).

**Cần chốt**
1. Loại tab chính `review` mới hay mở như panel trong Source Control? (khuyến nghị: tab mới; tác động `WorkspaceVisibleTabType` và tab-bar.)
2. Lưu trạng thái review ở đâu (localStorage hay backend, theo `specs/frontend/storage/backend-persistence.md` và `browser-storage-catalog.md`): chưa đọc.
3. Cho phép thêm `elkjs`/`dagre` và thư viện treemap, hay chỉ dùng thứ đã có?
4. Phạm vi mặc định ("thay đổi chưa commit" hay "so với nhánh gốc"), và khi nào kích hoạt tự động sau khi agent xong.
5. Chế độ web: kiểm tra `web-preload-api` có đủ cho push `codeIntel.*`.
6. Bao nhiêu ngôn ngữ i18n phải có chuỗi ngay từ đầu.
