# CR-CV-055 — Lens Kiến trúc C4: sơ đồ component theo container và chỉnh `c4.yaml`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-055 |
| **Tên** | Lens Kiến trúc C4 mức 3: `@xyflow/react`, chọn container, nhóm component theo lớp hexagonal, cạnh có trọng số, nhãn "suy luận", và giao diện soạn/kiểm tra/lưu ghi đè `c4.yaml` qua `codeIntel.c4.get/save` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050, 051, 053 (mã hoá lớp phủ, điều hướng diff); backend: CR-CV-033 (C4 mức 3 và schema `c4.yaml`), CR-CV-011 (bảng `c4_overrides`), CR-CV-040 |
| **Mở khoá** | (độc lập) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (file mới), `shared/c4-override-document.ts` (mới), `store/slices/review-ui.ts` (thêm `c4ContainerId`, `c4Drafts`), `review-lens-registry.ts`, `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Component của C4 không có sẵn trong GitNexus/CodeGraph; sơ đồ là **suy luận** từ cấu trúc thư mục hexagonal (`usecase`, `domain`, `adapter/*`), cạnh `implements`/`calls` và lời gọi gRPC ([08 §3](../../../research/view-code/08-views-and-review-models.md)), cho phép người dùng ghi đè bằng `c4.yaml` (đổi tên, gộp, mô tả, bỏ package nhiễu). Mỗi sơ đồ chỉ vẽ **một container**. Người review cần thấy "thay đổi chạm component nào, quan hệ nào" và sửa ghi đè khi sơ đồ sai.

Hiện trạng đã đọc (2026-10-05):

- `@xyflow/react` 12 đã dùng nhưng bố cục phải tự viết (không `elkjs`/`dagre`, O5); `DAGPreview.tsx`/`TaskDAGView.tsx` hardcode hex nên không sao chép (CR-CV-053 mục 1).
- **Đã có trong `frontend/package.json`**: `yaml` ^2.8.4 (dùng ở `shared/orca-yaml.ts`, `shared/fleet-config-parser.ts`) và `zod` ~4.4.3 (dùng ở `shared/workspace-session-schema.ts`, `shared/runtime-rpc-envelope.ts`). Chưa kiểm chứng `yaml` đã nằm trong gói renderer (hiện chỉ `shared/` dùng) và kích thước thêm vào bundle.
- Primitive: `ui/textarea.tsx`, `ui/sheet.tsx`, `ui/dialog.tsx`, `ui/select.tsx`, `ui/command.tsx`, `ui/table.tsx`, `ui/badge.tsx`; `useConfirmationDialog()` (`components/confirmation-dialog.tsx:120`) trả hàm xác nhận bất đồng bộ.
- **Hợp đồng dữ liệu**: `C4ComponentView {container: ContainerRef, components[], relations[], externals[]}`, `Component {id, name, kind:'usecase|domain|adapter|grpc-server|grpc-client|config|other', path, description?, symbolCount, techHint?}`, `ComponentRelation {from, to, kind:'uses|implements|calls-rpc|reads|writes|publishes|subscribes', evidence: SymbolRef[], count}`, `ExternalRef {id, name, kind:'service|database|queue|vault|external-api'}` (08 §3). **`ContainerRef` chưa được định nghĩa** và không có kênh liệt kê container (README 3.6 chỉ có `GetArchitecture`; 3.7 có `codeIntel.architecture`, `codeIntel.c4.get/save`). Bảng `c4_overrides`: `container, document (JSON/YAML text), updated_by, updated_at, version` (README 3.5): **bản ghi nằm ở backend**, không phải file trong repo, nên "lưu `c4.yaml`" là ghi vào dịch vụ, không ghi vào worktree.
- Chưa có mã lỗi riêng cho xung đột phiên bản và cho lỗi xác thực nội dung YAML (README 3.3 chỉ có `CODEINTEL_INVALID_PARAMS`).
- Không có cơ chế quyền sửa phía frontend; chỉ có lỗi `forbidden` khi ghi (`runtime-rpc-result.ts`, `isRuntimeScopeForbiddenError`).

## 2. Giải pháp đề xuất

### 2.1 Cây component và file (`components/review-map/`, mới)

```
ArchitectureLens                      (ReviewLensProps)
├─ ArchitectureToolbar                (C4ContainerPicker · Đồ thị | Danh sách · [Chỉnh c4.yaml] · ReviewOverlayLegend · C4EdgeLegend)
├─ C4InferredNotice                   (banner "suy luận" ở cấp sơ đồ; 2.4)
├─ C4DiagramCanvas                    (@xyflow/react; lazy)
│    ├─ C4ComponentNode · C4ExternalNode · C4LayerBand
├─ C4RelationsTable                   (Danh sách thay thế: từ, loại, tới, số lần)
├─ C4ComponentDetail                  (nội dung drawer; 2.5)
└─ C4OverrideEditor                   (Sheet bên phải; 2.6)
c4-layer-layout.ts  c4-overlay-model.ts  c4-container-default.ts  c4-edge-style.ts
shared/c4-override-document.ts        (validate: yaml + zod)
```

```
┌ Container: [infra-fleet-service ▾]  [Đồ thị|Danh sách]  [Chỉnh c4.yaml]  ───── Suy luận từ cấu trúc thư mục ┐
│  GỌI VÀO      ĐIỀU PHỐI        MIỀN         ADAPTER RA            NGOÀI                                  │
│ ┌─────────┐  ┌─────────┐    ┌────────┐   ┌────────────┐       ╭────────────╮                          │
│ │grpc     │─▶│usecase  │──▶ │domain  │   │postgres    │──────▶│ Postgres   │                          │
│ │ server  │  │ relay ● │    └────────┘   │eventbus  ● │──────▶│ event bus  │                          │
│ └─────────┘  └─────────┘ ◁ ─ implements ─│grpcclient  │──────▶│ git-gw svc │                          │
│                  (● = có đổi, nét đứt = chưa test, đường dày = nhiều lời gọi)                        │
└──────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Dữ liệu và hợp đồng (đề xuất; cần CR-CV-033/040 xác nhận)

- `codeIntel.architecture {container?}` → `{containers: ContainerRef[]; view: C4ComponentView | null}`. Không truyền `container` → `containers` đầy đủ và `view` cho container mặc định phía server hoặc `null`. Frontend gọi một lần không tham số để lấy danh sách, rồi gọi lại với `container` được chọn (`useCodeIntelQuery('architecture', {container})`, cache theo `(scopeKey, container)`).
- `ContainerRef = {id, name, kind: 'service'|'agent'|'frontend'|'desktop'|'other', path}`; `path` (tương đối gốc repo) dùng để chọn container mặc định: `c4-container-default.ts` chọn container chứa nhiều `changedFiles` nhất theo tiền tố `path` (hòa thì theo tên); không có thay đổi hoặc không khớp thì container đầu tiên; lựa chọn lưu ở `ReviewUiState.c4ContainerId`.
- Trường tuỳ chọn mong muốn: `Component.origin` và `ComponentRelation.origin` = `'inferred'|'override'` (để gắn nhãn "suy luận" từng phần tử); nếu vắng, nhãn chỉ ở cấp sơ đồ (2.4).
- Giới hạn: tối đa 150 component + external và 500 quan hệ được vẽ; vượt thì mặc định chế độ Danh sách kèm dòng "Đang hiển thị {n}/{total}" (README mục 6: không trả/hiển thị quá lớn, banner `truncated` ở CR-CV-051).

### 2.3 Bố cục và vẽ (`c4-layer-layout.ts`, thuần, có test)

Bố cục tầng theo lớp hexagonal, xác định: cột ánh xạ từ `Component.kind`:

| Cột | Kind | Nhãn dải |
|---|---|---|
| 0 | `grpc-server` | Gọi vào |
| 1 | `usecase` | Điều phối |
| 2 | `domain` | Miền |
| 3 | `adapter`, `grpc-client` | Adapter ra |
| 4 | `externals` (mọi `ExternalRef`) | Ngoài |
| dưới cột 1 | `config`, `other` | Khác |

- Trong cột sắp theo tên; `C4LayerBand` là nút nền không chọn/không kéo (`selectable:false`, `draggable:false`, `zIndex` thấp) có nhãn lớp, cao theo số hàng; dải trống bị bỏ.
- Cạnh: `strokeWidth = clamp(1 + log2(count), 1, 5)` (`c4-edge-style.ts`); nhãn hiện số `count` khi > 1. **Loại quan hệ** mã hoá bằng kiểu nét (`strokeDasharray`): `uses` đặc, `implements` `6 4`, `calls-rpc` đặc kèm icon `Waypoints` trên nhãn, `reads` `2 4`, `writes` `10 3 2 3`, `publishes` `4 2`, `subscribes` `1 3`. Bảy kiểu nét khó phân biệt bằng mắt nên nét chỉ là gợi ý: loại luôn có trong tooltip, drawer và bảng danh sách. `C4EdgeLegend` dựng từ cùng bảng ánh xạ (không lệch). Cạnh chạm thay đổi (giao giữa `evidence[].key` và `changedSymbols`) viền `var(--review-changed)`; cạnh còn lại `var(--muted-foreground)`; cạnh đang chọn `var(--foreground)`.
- Nút component: tên, icon theo `kind` (`lucide-react`), `symbolCount`, `techHint?`; cờ lớp phủ theo CR-CV-053 2.5: `changed` nếu có `changedFiles` nằm dưới `Component.path` (so tiền tố, chuẩn hoá `normalizeRuntimePathSeparators`), `untested` nếu `uncoveredSymbols` có `filePath` dưới `path`, `violation` nếu `violations` liên quan; cờ `affected` chỉ khi có dữ liệu `impact` đã tải; không có thì không hiển thị cờ đó. `c4-overlay-model.ts` thuần, có test.
- `ReactFlow`: `nodesDraggable={false}`, `nodesConnectable={false}`, `onlyRenderVisibleElements`, `fitView` (`duration: 0` khi `prefers-reduced-motion`), `colorMode` theo chủ đề, `Controls`; tải lười cùng `@xyflow/react/dist/style.css`. Màu chỉ qua token (`var(--…)`); chưa kiểm chứng áp dụng `var()` ở SVG xyflow (CR-CV-053 mục 6).
- **Danh sách thay thế** `C4RelationsTable` (`ui/table.tsx`): cột "Từ", "Loại", "Tới", "Số lần", cờ lớp phủ; mặc định khi > 150 nút hoặc người dùng chọn; là đường truy cập cho trình đọc màn hình.

### 2.4 Nhãn "suy luận"

- **Cấp sơ đồ**: `C4InferredNotice` luôn hiển thị phía trên sơ đồ khi **không có** ghi đè (`c4.get` trả `document` rỗng/không có bản ghi): "Sơ đồ được suy ra từ cấu trúc thư mục và quy tắc hexagonal; có thể chưa đúng ý thiết kế." Khi có ghi đè: "Suy luận kèm ghi đè của {updatedBy} lúc {thời gian}". Không bao giờ trình bày sơ đồ như sự thật đã xác minh (STYLEGUIDE "UI copy must not overclaim").
- **Cấp phần tử** (khi có `origin`): `Badge variant="outline"` "Suy luận" cho phần tử `inferred`; phần tử `override` có nhãn "Đã chỉnh". Component không có `description` hiển thị mờ "Chưa có mô tả" và nút "Thêm mô tả" mở `C4OverrideEditor`.

### 2.5 Drawer chi tiết component (`C4ComponentDetail`)

Chọn nút hoặc cạnh. Component: tên, `kind`, `path` (sao chép), `description`, `techHint`, `symbolCount`, nhãn nguồn, cờ lớp phủ, danh sách quan hệ vào/ra (đếm `count`, bấm để chọn cạnh). Cạnh: loại, hai đầu, `count`, **Bằng chứng** (`evidence: SymbolRef[]`, tối đa 20 + xem thêm; bấm chọn symbol, nút "Xem diff"/"Mở trong editor" theo CR-CV-053 2.6). Hành động "Chỉnh trong c4.yaml" mở trình soạn.

### 2.6 Soạn, kiểm tra, lưu `c4.yaml` (`C4OverrideEditor`, `shared/c4-override-document.ts`)

`Sheet` bên phải rộng tối thiểu 560 px (biểu mẫu có văn bản nhiều dòng cần chỗ rộng: STYLEGUIDE "Dialogs and overlays"), gồm `Textarea` đơn cách (`font-mono`, `spellCheck={false}`), danh sách vấn đề, và chân trang.

- **Tải**: `codeIntel.c4.get {worktreeId, container}` → `C4Override {container, document, version, updatedBy, updatedAt}`. Không có bản ghi: soạn từ rỗng với một khối chú thích giải thích; **mẫu khởi tạo theo schema chỉ có khi CR-CV-033 công bố schema** (7.1) vì UI không được tự bịa khoá.
- **Kiểm tra cục bộ** (`validateC4OverrideDocument(text)`, debounce 300 ms, thuần, có test), theo thứ tự:
  1. Kích thước ≤ 64 KiB tính bằng byte UTF-8 (`TextEncoder`); hằng số tạm, phải khớp backend (7.2).
  2. Cú pháp YAML: `parseDocument(text, {uniqueKeys: true, prettyErrors: true, maxAliasCount: 100})`; `errors[]`/`warnings[]` dùng `linePos` để có `{line, column, message, severity}`; không dùng thẻ tuỳ chỉnh (lược đồ `core` mặc định).
  3. Gốc là một ánh xạ (mapping), không phải mảng hay chuỗi.
  4. Theo schema `zod` (`c4OverrideDocumentSchema`) **khi CR-CV-033 công bố**; trước đó bỏ qua bước này và ghi rõ trong giao diện "Chưa kiểm tra ngữ nghĩa; máy chủ sẽ kiểm tra khi lưu".
  Danh sách vấn đề hiển thị `Dòng:Cột lỗi`; bấm một dòng đặt vùng chọn của `Textarea` tại vị trí (tính từ độ lệch ký tự của dòng).
- **Lưu**: `codeIntel.c4.save {worktreeId, container, document, expectedVersion}`. Nút "Lưu" bị vô hiệu khi có lỗi (không phải cảnh báo), khi không đổi, hoặc đang lưu; khoá **ngay** khi bấm, nhãn đổi sau 1 s, giai đoạn sau 3 s (thang STYLEGUIDE; từ xa trì hoãn phần hiển thị 200 ms). Thành công: cập nhật `version`, hiển thị "Đã lưu lúc {giờ}" trong chân trang (không toast), vô hiệu hoá cache `architecture` của container và tải lại sơ đồ; lỗi: lỗi persistent inline.
  - `conflict` (CR-CV-050 7.3): băng "Bản trên máy chủ đã đổi bởi {ai} lúc {giờ}" với ba hành động: "Tải bản mới" (thay thế nháp, có xác nhận bằng `useConfirmationDialog`), "Ghi đè bằng bản của tôi" (tải lại `version` rồi lưu; có xác nhận, `confirmVariant:'destructive'` vì làm mất thay đổi của người kia), "Sao chép bản của tôi".
  - `validation` từ máy chủ: hiển thị thông điệp (và `fields[]` nếu có) trong danh sách vấn đề.
  - `forbidden`: chuyển trình soạn sang **chỉ đọc**, ghi "Bạn không có quyền sửa c4.yaml"; nháp vẫn sao chép được.
  - `offline`/`timeout`: giữ nháp, hiển thị "Chưa lưu" và Thử lại; không khẳng định đã lưu.
- **Nháp**: `ReviewUiState.c4Drafts: Record<containerId, string>` (≤ 8 container, mỗi nháp ≤ 64 KiB) để đổi container/lens không mất; đóng Sheet khi có thay đổi chưa lưu hỏi bằng `useConfirmationDialog()` ("Bỏ thay đổi?"). `Esc` đóng (qua xác nhận nếu cần); tiêu điểm vào `Textarea` khi mở; không có phím tắt lưu riêng ở CR này (nút "Lưu" là hành động chính; `isScreenSubmitShortcut` (`lib/screen-submit-shortcut.ts`) có thể thêm sau: `metaKey` Mac, `ctrlKey` nơi khác, nhãn bằng `ShortcutKeyCombo`).
- Không chặn phím `Tab` trong `Textarea` (giữ trợ năng bàn phím); thụt lề bằng dấu cách.

### 2.7 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải danh sách container/sơ đồ | `Skeleton` + `ArchitectureLoadingStage` theo thang (CR-CV-051) |
| Không có container nào | "Chưa suy ra container nào; kiểm tra quy ước thư mục `internal/`" (nêu điều quan sát được, không đoán) |
| `view === null` cho container | "Chưa có sơ đồ cho container này" + nút Thử lại |
| `truncated` | "{shown}/{total}" + chế độ Danh sách |
| Lỗi truy vấn (`timeout`, `too-large`, `tool-failed`) | Lỗi inline có Thử lại |
| `c4.get` lỗi | Sơ đồ vẫn hiển thị; nút "Chỉnh c4.yaml" hiện lỗi nhỏ và Thử lại |
| Lỗi cấp khung (`index-missing`, `offline`…) | Do `ReviewViewStateScreen` (CR-CV-051) |

## 3. Quyết định thiết kế

- **Bố cục theo lớp hexagonal cố định** (xác định, không thư viện bố cục): đọc được và so sánh được giữa các container.
- **Sơ đồ luôn có nhãn "suy luận"**; ghi đè được nêu rõ người sửa và thời điểm.
- **Không tính trước kết quả ghi đè ở client**: sau khi lưu, tải lại sơ đồ do server dựng (tránh hai bản logic gộp).
- **Kiểm tra cục bộ chỉ cú pháp/kích thước/gốc**, ngữ nghĩa theo schema do CR-CV-033 sở hữu; không bịa khoá.
- **Một nháp mỗi container** trong store (giới hạn 8) thay vì mất khi đổi lens.
- **Ghi đè xung đột phải xác nhận kiểu `destructive`** vì làm mất thay đổi của người khác.
- **Danh sách luôn có** cho trợ năng và đồ thị lớn.

## 4. Tiêu chí chấp nhận

- [ ] Chọn container đổi sơ đồ; mặc định là container chứa nhiều file đã đổi nhất; lựa chọn giữ khi đổi lens trong cùng tab.
- [ ] Bố cục đặt component đúng cột theo `kind` (bảng 2.3), external ở cột 4, `config`/`other` bên dưới; hai lần vẽ cùng dữ liệu cho cùng toạ độ.
- [ ] Độ dày cạnh tăng theo `count`, nhãn số hiện khi > 1; loại quan hệ phân biệt bằng nét (không chỉ màu); chú giải dựng từ cùng bảng.
- [ ] Lớp phủ: component dưới `path` có file đổi có cờ `changed`; cạnh có `evidence` giao với `changedSymbols` viền `changed`; không hiển thị cờ `affected` khi không có dữ liệu.
- [ ] Banner "suy luận" luôn có khi chưa có ghi đè; có ghi đè thì ghi người sửa và thời gian; từng phần tử có nhãn nếu `origin` có mặt.
- [ ] > 150 nút/> 500 quan hệ mặc định Danh sách và hiển thị "{shown}/{total}".
- [ ] Trình soạn: lỗi cú pháp hiển thị `Dòng:Cột` và bấm đặt con trỏ đúng; > 64 KiB và gốc không phải mapping bị chặn; "Lưu" bị vô hiệu khi có lỗi; khoá ngay khi bấm.
- [ ] Lưu thành công cập nhật `version`, tải lại sơ đồ, hiển thị "Đã lưu" bằng trạng thái (không toast); `conflict` hiển thị ba hành động, ghi đè cần xác nhận; `forbidden` chuyển chỉ đọc; `offline` giữ nháp và "Chưa lưu".
- [ ] Đóng Sheet có thay đổi chưa lưu hỏi xác nhận; nháp giữ qua đổi lens/container (≤ 8) và bị dọn khi xoá worktree.
- [ ] Không hex, không lớp màu Tailwind thô trong file mới; `fitView` không hoạt ảnh khi `prefers-reduced-motion`; chuỗi qua `translate()` đủ 5 locale; chạy ở Electron và web; không thêm thư viện (chỉ `yaml`, `zod` có sẵn).

## 5. Kiểm thử

- `c4-layer-layout.test.ts`: ánh xạ `kind` → cột, dải trống bỏ, ổn định, 150 nút, external, quan hệ tới nút không tồn tại bị bỏ (không ném).
- `c4-overlay-model.test.ts`: so tiền tố đường dẫn (POSIX/Windows), `evidence` giao `changedSymbols`, thiếu dữ liệu `affected`.
- `c4-container-default.test.ts`: chọn theo số file đổi, hòa, không có thay đổi.
- `c4-edge-style.test.ts`: `count` 1/2/8/1000, bảng loại → nét đủ 7 loại.
- `shared/c4-override-document.test.ts`: cú pháp lỗi (dòng/cột), khoá trùng, gốc là mảng, vượt 64 KiB (ký tự nhiều byte), bí danh quá nhiều, rỗng hợp lệ.
- `C4OverrideEditor.test.tsx` (`// @vitest-environment happy-dom`, client giả): nạp, gõ → vấn đề, lưu thành công, `conflict` ba hành động và xác nhận, `forbidden` chỉ đọc, `offline` giữ nháp, đóng có thay đổi hỏi xác nhận.
- `ArchitectureLens.test.tsx`, `C4ComponentDetail.test.tsx`, `C4RelationsTable.test.tsx` (mock `@xyflow/react` như `TaskDAGView.test.tsx`).
- `store/slices/review-ui.test.ts` mở rộng: `c4Drafts` giới hạn 8 và dọn khi xoá worktree (test rò rỉ có sẵn của CR-CV-051 chạy trên khoá này).
- `i18n/code-intel-locale-coverage.test.ts` mở rộng.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Schema `c4.yaml`**, `ContainerRef`, kênh liệt kê container, trường `origin`, và lỗi xác thực nội dung đều chưa được định nghĩa ở README v7; toàn bộ phần soạn phụ thuộc CR-CV-033. Nếu backend chỉ nhận JSON thay vì YAML (README 3.5: "JSON/YAML text") cần đổi bước kiểm tra.
- `yaml` trong gói renderer chưa dùng; kích thước bundle chưa đo (tải lười cùng lens).
- Bố cục cố định theo `kind` có thể xấu với container không theo hexagonal (`frontend`, `agent`): phần lớn component rơi vào `other`; cần đo trên ít nhất một container TypeScript.
- Cạnh nhiều làm rối; chưa kiểm chứng ngưỡng 500.
- Hành vi xyflow với `var()` trong SVG và nút nền không chọn chưa kiểm chứng.
- Xung đột phiên bản và `forbidden` ở `c4.save` phụ thuộc mã lỗi chưa chốt.

## 7. Câu hỏi mở

1. Schema `c4.yaml` (khoá cho đổi tên/gộp/mô tả/ẩn) và dạng lỗi xác thực nội dung: CR-CV-033.
2. Giới hạn kích thước `document` ở server (đang giả định 64 KiB).
3. `codeIntel.architecture` trả `containers[]` và `origin` như đề xuất ở 2.2?
4. Quyền sửa: có `viewerCan` (hoặc lỗi trước khi soạn) để ẩn nút thay vì đợi `forbidden` khi lưu không?
5. `ContainerRef.kind` có đủ cho `agent`, `frontend`, `desktop` (không hexagonal) không, hay cần bố cục khác?
6. Có cần xuất sơ đồ (SVG/PNG) không? Không ở MVP.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O5, 3.2, 3.5, 3.6, 3.7, 3.9), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5), `08-views-and-review-models.md` (§3), `07-architecture-decisions.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md` (UX rules: copy không overclaim; Dialogs and overlays; Empty and error states; Animation)
- `/opt/repos/orca/frontend/package.json` (`yaml`, `zod`, `@xyflow/react`), `frontend/src/shared/orca-yaml.ts`, `shared/fleet-config-parser.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/confirmation-dialog.tsx` (:120), `components/ui/{textarea,sheet,dialog,select,command,table,badge}.tsx`, `lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/__tests__/TaskDAGView.test.tsx`, `components/task/TaskDAGView.tsx` (hex không dùng lại)
- CR-CV-050 (client, lỗi, token), CR-CV-051 (khung), CR-CV-053 (mã hoá lớp phủ, điều hướng diff)
- Mới: `components/review-map/{ArchitectureLens,C4DiagramCanvas,C4ComponentNode,C4ExternalNode,C4LayerBand,C4RelationsTable,C4ComponentDetail,C4OverrideEditor,C4ContainerPicker,C4InferredNotice}.tsx`, `c4-layer-layout.ts`, `c4-overlay-model.ts`, `c4-container-default.ts`, `c4-edge-style.ts`, `shared/c4-override-document.ts`
