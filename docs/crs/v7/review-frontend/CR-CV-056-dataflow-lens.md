# CR-CV-056 — Lens Luồng dữ liệu: sơ đồ tuần tự Mermaid, danh sách bước, tô bước đã đổi, sao chép và xuất

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-056 |
| **Tên** | Lens Luồng dữ liệu: danh sách luồng, sơ đồ tuần tự Mermaid dựng từ `DataFlow` (tái dùng `MermaidBlock` và `mermaid-config`), danh sách bước làm điểm tương tác, tô bước đã đổi, sao chép nguồn Mermaid và xuất `.mmd`/`.svg`, giới hạn kích thước sơ đồ |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050, 051, 052 (hook phím danh sách), 053 (mã hoá lớp phủ, chi tiết symbol, điều hướng diff); backend: CR-CV-034 (`DataFlow`), CR-CV-032 (nối proto client↔server), CR-CV-040 |
| **Mở khoá** | (độc lập) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (file mới), `hooks/useIsDarkTheme.ts` (mới), `store/slices/review-ui.ts` (thêm `dataFlowId`), `review-lens-registry.ts`, `i18n/locales/*.json`; không sửa `MermaidBlock.tsx` |

---

## 1. Bối cảnh và vấn đề

Người review cần biết một thay đổi nằm ở **bước nào** của luồng ("UI → gateway → service → kho"): sơ đồ tuần tự cho một luồng, bước đã đổi nổi bật, bấm bước thì xem symbol, và có thể sao chép hoặc xuất ([10 §5](../../../research/view-code/10-frontend-review-ux.md): "Mermaid sequence (tái dùng `MermaidBlock`/`mermaid-config`) + danh sách bước; có thể sao chép/xuất"; [08 §4](../../../research/view-code/08-views-and-review-models.md)).

Hiện trạng đã đọc (2026-10-05):

- `components/editor/MermaidBlock.tsx`: export mặc định, props `{content: string, isDark: boolean, htmlLabels?: boolean}`; tải lười `mermaid` (~650 KB, theo chú thích trong file); **tuần tự hoá mọi lệnh render** qua hàng đợi cấp module; `securityLevel: 'strict'` ở `mermaid-config.ts`; SVG đi qua `DOMPurify.sanitize(svg, {USE_PROFILES:{svg:true}})`; lỗi cú pháp hiển thị khung lỗi kèm mã nguồn thô (không phá trang). **Không phơi SVG hay hàm xuất** ra ngoài; chỉ có `containerRef` nội bộ.
- `mermaid` 11.15.0 có `MAX_TEXTLENGTH = 5e4` (50 000 ký tự) trong `dist/mermaid.core.mjs:951` (đã đọc); vượt thì Mermaid báo lỗi kích thước. Giới hạn cạnh/nút riêng của loại sequence chưa kiểm chứng.
- `isDark` hiện được tính lặp ở ba nơi (`MarkdownPreview.tsx:588`, `MermaidViewer.tsx`, `sidebar/CommentMermaidBlock.tsx`): `settings?.theme === 'dark' || (settings?.theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)`. Chưa có hook dùng chung.
- Sao chép văn bản: `window.api.ui.writeClipboardText(text)` (`preload/api-types.ts:3216`, namespace `ui` bắt đầu :3041; mẫu dùng ở `lib/markdown-review-note-copy.ts`). Tải tệp: `components/settings/mcp/mcp-audit-csv.ts` (`URL.createObjectURL(new Blob(...))` + thẻ `<a download>`; tên tệp không chứa `:` để hợp lệ trên Windows) là tiền lệ duy nhất trong renderer; hành vi tải tệp trong Electron đóng gói chưa kiểm chứng ngoài tiền lệ này.
- **Dữ liệu**: `DataFlow {id, label, trigger:{kind:'ws-channel|http|grpc|event|cron', name}, steps: FlowStep[], stores: StoreAccess[]}`, `FlowStep {n, from: ComponentRef, to: ComponentRef, kind:'call|rpc|event|db-read|db-write|ws-push', method?, symbol?: SymbolRef, sync: bool}`, `StoreAccess {step, store: StoreRef, table?, op:'read|write'}` (08 §4). `ComponentRef` và `StoreRef` chưa định nghĩa; kênh `codeIntel.dataFlows` (danh sách) và `codeIntel.dataFlow` (một luồng) có tên trong README 3.7 nhưng **kiểu danh sách chưa có**. GitNexus có ~300 `Process` cho Orca (README mục 1), danh sách phân trang ≤ 100 mỗi trang (README 3.2).
- Luồng GitNexus thường dừng ở stub gRPC; CR-CV-032/034 nối client↔server (08 §4). UI không được trình bày luồng như đầy đủ khi chưa nối.

## 2. Giải pháp đề xuất

### 2.1 Cây component và file (`components/review-map/`, mới)

```
DataFlowLens                         (ReviewLensProps)
├─ DataFlowListPane                  (trái: tìm kiếm, "Chỉ luồng chạm thay đổi", danh sách DataFlowSummary)
└─ DataFlowDetailPane
     ├─ DataFlowToolbar              (tên luồng, trigger, [Sao chép Mermaid] [Xuất ▾ .mmd .svg] [Thu phóng − 100% +] ReviewOverlayLegend)
     ├─ DataFlowDiagram              (MermaidBlock bọc; 2.3)
     └─ DataFlowStepList             (danh sách bước; 2.4)
data-flow-mermaid.ts  data-flow-overlay.ts  data-flow-export.ts  hooks/useIsDarkTheme.ts
```

```
┌ Luồng ─────────────┬ agent.execPrompt · trigger: ws-channel ──── [Sao chép][Xuất ▾][− 100% +] ┐
│ 🔍 tìm…            │ ┌ sơ đồ tuần tự (Mermaid) ───────────────────────────────────────────┐ │
│ ☐ Chỉ chạm thay đổi│ │  UI ──▶ gateway ──▶ infra-fleet ──▶ agent                          │ │
│ ▌agent.execPrompt 7│ │   1       2            3 [đổi]                                  │ │
│  task.create      5│ └────────────────────────────────────────────────────────────────────┘ │
│  …  [Tải thêm]     │ Bước │ Từ → Tới                │ Loại │ Nhãn            │ Kho          │
│                    │ ▌ 3  │ gateway → infra-fleet   │ rpc  │ RelayByDevServer│ ● [đổi]      │
└────────────────────┴──────────────────────────────────────────────────────────────────────────┘
```

Dưới 720 px bề rộng: danh sách luồng thu thành `Select` ở thanh công cụ.

### 2.2 Dữ liệu (đề xuất, cần CR-CV-034/040)

- `DataFlowSummary = {id, label, trigger, stepCount, changedStepCount?}` cho `codeIntel.dataFlows {limit: 100, offset}`; `useCodeIntelQuery('dataFlows', …)`, nút "Tải thêm" khi còn trang. Tìm kiếm theo nhãn **ở phía client** trên các trang đã tải (và gợi ý khi còn trang chưa tải). "Chỉ luồng chạm thay đổi" dùng `changedStepCount > 0`; nếu backend không trả thì dùng giao với `ChangeOverlay.affectedFlows` theo `id` (chỉ đúng khi cùng không gian id: 7.2).
- Chọn luồng: `codeIntel.dataFlow {flowId}` → `DataFlow`; cache theo `(flowId)`; hủy khi đổi lựa chọn nhanh.
- Vào lens từ chỗ khác: chip `flows` (CR-CV-051) mở lens với bộ lọc "chạm thay đổi"; "Luồng liên quan" ở panel symbol (CR-CV-053) đặt `ReviewUiState.dataFlowId`. Id không có trong `DataFlow` (vì `FlowSummary` của GitNexus ≠ `DataFlow`) → hiển thị "Chưa có luồng dữ liệu tương ứng với «{nhãn}»" kèm danh sách, không báo lỗi.
- `ComponentRef`/`StoreRef` tạm hiểu `{id: string; name: string}`; thiếu `name` thì dùng `id`. Parser `parseDataFlow` (`code-intel-wire-parsers.ts`): enum lạ → `'unknown'`; `n` thiếu → chỉ số vị trí; bỏ bước không có `from`/`to`.

### 2.3 Sơ đồ Mermaid (`data-flow-mermaid.ts`, thuần, có test)

`buildSequenceDiagram(flow, {changedSteps: ReadonlySet<number>, maxSteps = 60, maxParticipants = 14, maxChars = 40000}) → {ok: true, source, renderedSteps, totalSteps} | {ok: false, reason: 'too-many-participants'|'too-long'|'empty'}`.

Quy tắc:

- Dòng đầu `sequenceDiagram` rồi `autonumber`. Người tham gia theo thứ tự xuất hiện đầu tiên; `id` an toàn `p1…pn` (không dùng chuỗi từ dữ liệu làm định danh), dòng `participant p1 as <nhãn đã thoát>`. Kho (`StoreRef`) là một người tham gia như các thành phần khác (không dùng cú pháp biểu tượng riêng của Mermaid 11 vì chưa kiểm chứng).
- Thông điệp theo `kind` và `sync`: `call` → `->>`; `rpc` → `->>` kèm tiền tố `rpc: `; `event` → `-)`; `ws-push` → `--)`; `db-read`/`db-write` → `->>` tới kho với nhãn "đọc <bảng>" hoặc "ghi <bảng>"; `sync:false` ép dùng `-)`. Nhãn = `method ?? symbol.name ?? ''` cắt 48 ký tự.
- **Bước đã đổi**: nhãn thêm tiền tố `[đổi] ` (ký tự chữ, không phụ thuộc màu hay cú pháp `rect rgb(...)`); lý do không dùng `rect`/màu: `rect` cần chuỗi màu cụ thể, token `var(--review-changed)` không dùng được trong văn bản Mermaid và chưa kiểm chứng Mermaid đọc `oklch()` của Tailwind. Tô nổi chính nằm ở danh sách bước (2.4).
- **Thoát văn bản** `escapeMermaidLabel(text)`: bỏ ký tự điều khiển, chuyển xuống dòng thành dấu cách, thay `;`→`#59;`, `#`→`#35;`, `%`→`#37;` (chặn chỉ thị `%%{init}%%`), `<`→`#lt;`, `>`→`#gt;`, `"`→`#quot;`, dấu huyền → `#96;`. Dữ liệu đến từ repo nên coi là không đáng tin; thêm lớp bảo vệ có sẵn của `MermaidBlock` (`securityLevel: 'strict'` + DOMPurify). Không sinh `click`, không sinh HTML.
- **Giới hạn** (README mục 6, kiểm soát kích thước): ≤ 60 bước đầu được vẽ (`renderedSteps < totalSteps` thì dòng thông báo "Sơ đồ hiển thị {r}/{t} bước đầu; xem danh sách bên dưới"); > 14 người tham gia hoặc `source.length > 40000` (dưới `MAX_TEXTLENGTH` 50 000 của Mermaid 11.15.0, chừa biên) thì **không vẽ**, hiển thị lý do và chỉ dùng danh sách bước. Số tự đánh của Mermaid (`autonumber`) bằng `n` khi `n` liên tục từ 1 (vì chỉ cắt phần đuôi); nếu `n` không liên tục, danh sách vẫn hiển thị `n` thật và sơ đồ ghi chú "Số thứ tự trong sơ đồ có thể khác cột Bước".
- **Vẽ**: `<MermaidBlock content={source} isDark={isDark} htmlLabels={false} />` (`hooks/useIsDarkTheme.ts` mới gom công thức đang lặp; các nơi cũ giữ nguyên, không sửa trong CR này). Dùng đúng mặc định `securityLevel`, không `initialize` riêng. Lỗi cú pháp rơi vào khung lỗi của `MermaidBlock` (không phá lens); đoạn lỗi cộng nút "Sao chép nguồn" vẫn hoạt động.
- **Thu phóng**: ba nút `−`, `100%`, `+` (bước 25%, 50 đến 200%) dùng `transform: scale()` trên vùng bọc có `overflow: auto`; không thêm thư viện kéo-thả-phóng. Sơ đồ dài cuộn ngang. Chưa kiểm chứng chất lượng nét khi phóng bằng CSS (6).
- Trợ năng: vùng sơ đồ `role="img"` và `aria-label` "Sơ đồ tuần tự {nhãn}: {r}/{t} bước"; **danh sách bước là bản tương đương đầy đủ** cho trình đọc màn hình.
- Sao chép/xuất (`data-flow-export.ts`): "Sao chép Mermaid" gọi `window.api.ui.writeClipboardText(source)` (kết quả báo bằng trạng thái trên nút "Đã sao chép" 2 s, không toast); "Xuất .mmd" tải `source`; "Xuất .svg" lấy `outerHTML` của `svg` trong vùng bọc (`ref.querySelector('svg')`), vốn đã qua DOMPurify; vô hiệu khi chưa vẽ xong hoặc lỗi. Tên tệp `dataflow-<slug nhãn>-<YYYYMMDD-HHmm>.<đuôi>` không chứa `:` (mẫu `mcpAuditCsvFilename`), `slug` chỉ gồm `[a-z0-9-]` tối đa 40 ký tự. Phần tải tệp theo mẫu `downloadMcpAuditCsv`.

### 2.4 Danh sách bước (`DataFlowStepList`, tương đương đầy đủ của sơ đồ)

`ui/table.tsx`: cột Bước (`n`), Từ → Tới, Loại (icon `lucide-react` + chữ), Nhãn (`method`/`symbol.name`), Đồng bộ/Bất đồng bộ, Kho (bảng và `op` từ `stores` theo `step`), Cờ.

- **Bước đã đổi** (CR-CV-053 2.5): `computeOverlayFlags(step.symbol?.key, overlay)` cho `changed`/`untested`; thêm `changed` khi `StoreAccess.table` thuộc `ChangeOverlay.touchedTables`. Hàng `changed` có chấm `CircleDot` và nhãn chữ "đã đổi" cùng viền trái dày bằng `border-l-2 border-[color:var(--review-changed)]`; `untested` thêm nét đứt ở viền trái. Không dùng màu duy nhất.
- Bấm hàng: có `step.symbol` thì `onSelectSymbol(symbol)` (drawer `SymbolDetailPanel`, nút "Xem diff" theo CR-CV-053); không có thì drawer `DataFlowStepDetail` (từ, tới, loại, `method`, kho/bảng/`op`, ghi chú "Không có symbol gắn với bước này"). Phím `j/k/↑/↓/Enter/Home/End` bằng `useRovingListKeys` (cùng hook với Thứ tự đọc, CR-CV-052; bỏ qua khi `isEditableTarget`), nhãn chip phím ở chân bảng bằng `ShortcutKeyCombo`.
- Hàng "bước đầu bị cắt khỏi sơ đồ" (`n > renderedSteps`) mờ nhẹ và có chú thích "Không có trong sơ đồ".
- Danh sách dùng `useVirtualizer` khi > 150 bước (một luồng có thể dài); ≤ 150 vẽ thẳng.

### 2.5 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải danh sách | `Skeleton` danh sách + `DataFlowLoadingStage` theo thang CR-CV-051 |
| Danh sách rỗng | "Chưa dựng được luồng dữ liệu nào cho repo này" (không ghi nguyên nhân khi chưa biết); nếu `IndexStatus` cho thấy thiếu dữ liệu proto (CR-CV-032) thì gợi ý nêu số liệu có thật |
| Không chọn luồng | "Chọn một luồng ở bên trái"; gợi ý chọn luồng đầu tiên có bước đã đổi |
| Đang tải một luồng | `Skeleton` sơ đồ (vùng giữ chỗ cố định chiều cao) |
| Luồng rỗng (0 bước) | "Luồng này chưa có bước" |
| Quá lớn (`too-many-participants`, `too-long`) | Thay sơ đồ bằng lý do + chỉ danh sách |
| Cắt bớt bước | Dòng "{r}/{t} bước đầu" (không cắt im lặng) |
| Lỗi truy vấn (`timeout`, `too-large`, `tool-failed`) | Lỗi inline có Thử lại; lỗi cấp khung do CR-CV-051 |
| Mermaid lỗi cú pháp | Khung lỗi của `MermaidBlock` (có sẵn) kèm "Sao chép nguồn" |

## 3. Quyết định thiết kế

- **Tái dùng `MermaidBlock` nguyên trạng**, không sửa; lấy SVG từ DOM của vùng bọc để xuất, tránh đụng hàng đợi render toàn cục.
- **Bước đã đổi nổi bật ở danh sách, không ở sơ đồ**: Mermaid không có cách gắn màu theo token; tiền tố `[đổi]` là dấu hiệu không phụ thuộc màu có trong sơ đồ.
- **Danh sách bước là bản đầy đủ**; sơ đồ là trực quan có giới hạn 60 bước/14 người tham gia/40 000 ký tự.
- **Thoát văn bản ở một hàm, kiểm thử bơm chỉ thị**: dữ liệu từ repo không đáng tin.
- **Không thêm thư viện** (thu phóng bằng CSS, xuất bằng Blob).
- Không liên kết DOM giữa thông điệp trong SVG và hàng danh sách (chưa kiểm chứng id/class ổn định của Mermaid); chỉ liên kết qua số bước.

## 4. Tiêu chí chấp nhận

- [ ] Chọn một luồng vẽ sơ đồ tuần tự đúng người tham gia và thứ tự bước; thông điệp đúng mũi tên theo `kind`/`sync`.
- [ ] Nhãn chứa `;`, `#`, `%%{init: …}%%`, xuống dòng, `<script>`, dấu ngoặc kép không làm đổi cấu trúc sơ đồ (không sinh thêm người tham gia, chỉ thị hay thẻ); SVG sinh ra không chứa `<script>` hay `foreignObject` thừa.
- [ ] Luồng > 60 bước chỉ vẽ 60 bước đầu kèm "{r}/{t}"; > 14 người tham gia hoặc nguồn > 40 000 ký tự không vẽ, hiển thị lý do, danh sách vẫn đủ.
- [ ] Bước chạm symbol đã đổi hoặc bảng đã đổi có chấm, nhãn chữ "đã đổi" và viền trái dày trong danh sách, và tiền tố `[đổi]` trong sơ đồ; bước chưa test có viền trái nét đứt.
- [ ] Bấm hàng có `symbol` mở `SymbolDetailPanel`; không có `symbol` mở chi tiết bước.
- [ ] "Sao chép Mermaid" đặt đúng nguồn vào clipboard và báo trạng thái trên nút; "Xuất .mmd/.svg" tải tệp có tên hợp lệ ở Windows; SVG xuất ra không chứa `<script>`.
- [ ] Đổi giao diện sáng/tối vẽ lại sơ đồ theo chủ đề; lỗi cú pháp không phá lens.
- [ ] Từ chip `flows` và từ "Luồng liên quan" mở được lens; id lạ hiển thị thông báo, không lỗi.
- [ ] Danh sách có bàn phím `j/k/↑/↓/Enter`, bỏ qua khi tiêu điểm ở ô nhập; > 150 bước ảo hoá.
- [ ] Không hex, không lớp màu Tailwind thô trong file mới; chuỗi qua `translate()` đủ 5 locale; chạy ở Electron và web; không sửa `MermaidBlock.tsx`; không thêm thư viện.

## 5. Kiểm thử

- `data-flow-mermaid.test.ts`: từng `kind`/`sync` → mũi tên; thứ tự và id người tham gia; `[đổi]`; cắt 60 bước; > 14 người tham gia; > 40 000 ký tự (nhãn dài); `n` không liên tục; **bơm chỉ thị** (`%%{init:{…}}%%`, `---`, `;`, `participant evil`, xuống dòng) không đổi số dòng cấu trúc; kết quả xác định.
- `data-flow-export.test.ts`: tên tệp (không `:`, `slug`), nội dung `.mmd`, lấy `outerHTML` từ DOM giả, vô hiệu khi lỗi.
- `data-flow-overlay.test.ts`: bước đổi theo symbol, theo `touchedTables`, bước cắt khỏi sơ đồ.
- `hooks/useIsDarkTheme.test.ts`: `theme` `dark`/`light`/`system` với `matchMedia` giả.
- `DataFlowLens.test.tsx` (`// @vitest-environment happy-dom`; `vi.mock('@/components/editor/MermaidBlock')` trả một `div` ghi lại `content`, vì Mermaid cần DOM thật): chọn luồng, danh sách bước, sao chép (mock `window.api.ui.writeClipboardText`), id lạ, danh sách rỗng, quá lớn.
- `DataFlowStepList.test.tsx`: phím, ảo hoá > 150, hàng bị cắt khỏi sơ đồ.
- Kiểm tay bắt buộc (không có test ảnh chụp): sơ đồ 60 bước ở sáng/tối, độ rõ khi phóng 200%, tải `.svg` trong Electron đóng gói.
- `i18n/code-intel-locale-coverage.test.ts` mở rộng.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Kiểu danh sách luồng, `ComponentRef`, `StoreRef`** chưa có; CR phụ thuộc đề xuất ở 2.2.
- Mermaid sequence có giới hạn riêng (số thông điệp, độ rộng) ngoài 50 000 ký tự chưa kiểm chứng; ngưỡng 60/14/40 000 là ước lượng an toàn, chưa thử trên Mermaid 11.15.
- Cú pháp thoát bằng thực thể `#59;`/`#35;` của Mermaid dựa trên hành vi đã biết của tài liệu Mermaid, chưa chạy trên bản cài; test chuỗi không chứng minh Mermaid hiển thị đúng (cần kiểm tay).
- Hàng đợi render là toàn cục: nhiều sơ đồ cùng lúc xếp hàng; một luồng đổi nhanh có thể hiển thị kết quả cũ trong chốc lát.
- Tải tệp bằng `<a download>` trong Electron đóng gói chưa kiểm chứng ngoài tiền lệ MCP CSV.
- Thu phóng bằng CSS làm mờ chữ ở SVG lớn; có thể cần chế độ "vừa chiều rộng" thay thế.
- Danh sách luồng GitNexus (~300) trộn lẫn luồng không liên quan UI; chất lượng gợi ý của "Chỉ luồng chạm thay đổi" phụ thuộc backend.

## 7. Câu hỏi mở

1. Hình dạng `DataFlowSummary`, `ComponentRef`, `StoreRef` và trường `changedStepCount` (hoặc tính phía client).
2. `FlowSummary.id` (GitNexus) và `DataFlow.id` có chung không gian id không?
3. Có cần hỗ trợ DFD (UI → gateway → service → kho; 08 §4 nêu cả hai) ngoài sơ đồ tuần tự? Đề xuất không ở MVP.
4. Mermaid 11 có cú pháp loại người tham gia `database` ổn định không (để vẽ kho khác thành phần)? Hiện dùng người tham gia thường.
5. Có cần giới hạn riêng cho luồng nhiều dịch vụ (ví dụ cột nhóm theo container) không?
6. Có xuất PNG không? Không ở MVP.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.2, 3.7, 3.9, mục 6), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5, §6), `08-views-and-review-models.md` (§4)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (Empty and error states; Color roles; Animation)
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/{MermaidBlock.tsx,mermaid-config.ts,MermaidViewer.tsx,MarkdownPreview.tsx}` (:588 `isDark`), `components/sidebar/CommentMermaidBlock.tsx`
- `/opt/repos/orca/node_modules/.pnpm/mermaid@11.15.0/node_modules/mermaid/dist/mermaid.core.mjs` (:951 `MAX_TEXTLENGTH`), `/opt/repos/orca/frontend/package.json` (`mermaid`)
- `/opt/repos/orca/frontend/src/preload/api-types.ts` (:3041 `ui`, :3216 `writeClipboardText`), `renderer/src/lib/markdown-review-note-copy.ts`, `components/settings/mcp/mcp-audit-csv.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/editable-target.ts`, `components/ShortcutKeyCombo.tsx`, `components/ui/{table,select,skeleton}.tsx`
- CR-CV-050 (client, cache), CR-CV-051 (khung), CR-CV-052 (`useRovingListKeys`), CR-CV-053 (mã hoá lớp phủ, chi tiết symbol)
- Mới: `components/review-map/{DataFlowLens,DataFlowListPane,DataFlowDetailPane,DataFlowToolbar,DataFlowDiagram,DataFlowStepList,DataFlowStepDetail}.tsx`, `data-flow-mermaid.ts`, `data-flow-overlay.ts`, `data-flow-export.ts`, `hooks/useIsDarkTheme.ts`
