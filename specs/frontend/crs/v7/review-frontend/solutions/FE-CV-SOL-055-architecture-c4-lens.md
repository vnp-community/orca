# FE-CV-SOL-055-architecture-c4-lens: Lens Kiến trúc C4 mức 3 và chỉnh `c4.yaml`

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06; chưa chạy test hay ứng dụng.

**CR:** [CR-CV-055](../../../../../../docs/crs/v7/review-frontend/CR-CV-055-architecture-c4-lens.md)
**Area:** frontend (`components/review-map`, `shared/c4-override-document.ts`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (§3.1 `architecture`, `c4.get`, `c4.save`; §4.4 `ContainerRef`, `C4Component`, `C4Relation`, `C4External`, `C4ComponentView`; §2.3 `VERSION_CONFLICT`, `PAYLOAD_TOO_LARGE`, `INVALID_PARAMS`, `NOT_AUTHORIZED`; §2.4 `c4.save` ≤ 96 KiB, `document` ≤ 64 KiB), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-10, PQ-14(5), PQ-29; O-12).
**TDD tham chiếu:** [v5/15-task-graph-ui §1 TaskGraph](../../../../tdd/v5/15-task-graph-ui.md) (mẫu xyflow), [v5/05-ui-components §6 UI Library](../../../../tdd/v5/05-ui-components.md); storage: [backend-persistence](../../../../storage/backend-persistence.md) (ghi đè C4 ở backend).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `frontend/package.json` (`yaml` ^2.8.4, `zod` ~4.4.3, `@xyflow/react` ^12.11.2), `components/ui/*` (`textarea, sheet, dialog, select, command, table, badge` có), `components/confirmation-dialog.tsx` (tên; CR nêu `useConfirmationDialog` :120, **chưa đọc lại**), `lib/screen-submit-shortcut.ts` (tên), `components/task/TaskDAGView.tsx` (hex, không dùng). Chưa kiểm chứng: `yaml` đã nằm trong bundle renderer chưa (hiện `shared/orca-yaml.ts`, `fleet-config-parser.ts` dùng).

**Correction relative to CR-CV-055:**

| # | CR-055 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `architecture` trả `containers` và `view` (đề xuất) | PQ-10, §3.1: **đúng** `Env<{containers: ContainerRef[]; view: C4ComponentView \| null}>`, params `container?`, `includeHidden?` | Xác nhận; thêm công tắc "Hiện thành phần ẩn" (`includeHidden`) |
| 2 | `Component.origin 'inferred'\|'override'` (mong muốn) | `C4Component.origin: 'derived'\|'merged'\|'declared'`, `descriptionSource: 'c4.yaml'\|'package-doc'\|'none'`, `hidden`, `packagePaths`; `C4Relation.origin 'derived'\|'declared'`, `confidence`, `violatesLayering`, `label?`; `C4External.origin` | Nhãn "Suy luận" cho `derived`, "Đã chỉnh một phần" cho `merged`, "Khai báo" cho `declared`; `violatesLayering` ⇒ cờ `violation` trên cạnh |
| 3 | `C4Override` kiểu riêng | `c4.get ⇒ {container, document, version, updatedBy, updatedAt, seedSource?}`; `c4.save ⇒ {version, warnings: {code,message}[]}` | Dùng đúng hai dạng; `warnings` hiển thị sau khi lưu; `seedSource` hiển thị nguồn mẫu nếu có |
| 4 | Mã xung đột/xác thực chưa có | `CODEINTEL_VERSION_CONFLICT {currentVersion?}`; `INVALID_PARAMS {field, reason?}`; `PAYLOAD_TOO_LARGE {limit}`; quyền `c4_write` ⇒ `NOT_AUTHORIZED` | Ánh xạ trực tiếp (xem 4.5) |
| 5 | Giới hạn 64 KiB giả định | PQ-14(5): `c4.yaml` ≤ 64 KiB ở mọi tầng; args ≤ 96 KiB | Hằng cố định, kiểm theo byte UTF-8 |
| 6 | Schema `c4.yaml` chờ CR-033 | O-12 vẫn mở | Giữ: chỉ kiểm cú pháp/kích thước/gốc mapping; ngữ nghĩa do máy chủ (`warnings`/`INVALID_PARAMS`) |
| 7 | `ReviewUiState.c4Drafts` ≤ 8 | — | Giữ; dọn theo hai đường xoá worktree |
| 8 | `GetArchitecture` mơ hồ | PQ-10: là C4; đồ thị cụm có RPC riêng, **không có kênh WS** | Không dùng đồ thị cụm |

## 2. Hợp đồng áp dụng

`architecture` (20 s, `read`), `c4.get` (8 s, `read`), `c4.save` (8 s, `c4_write`, `expectedVersion`, `0` = tạo). Chuỗi tự do (tên, mô tả, `label`) coi là văn bản thuần (U9). `ContainerRef.kind`: `service|agent|frontend|desktop|other`.

## 3. Lệch giữa CR và hợp đồng

Bảng mục 1 (8 dòng). Lệch lớn nhất là dòng 2 (tên giá trị `origin`).

## 4. Giải pháp

### 4.1 Cây file

```
components/review-map/ArchitectureLens.tsx ArchitectureToolbar.tsx C4ContainerPicker.tsx C4InferredNotice.tsx C4DiagramCanvas.tsx
  C4ComponentNode.tsx C4ExternalNode.tsx C4LayerBand.tsx C4RelationsTable.tsx C4ComponentDetail.tsx C4OverrideEditor.tsx
  c4-layer-layout.ts c4-overlay-model.ts c4-container-default.ts c4-edge-style.ts
shared/c4-override-document.ts   (validate: yaml + zod)
```

### 4.2 Dữ liệu và bố cục

Lần đầu `architecture {}` ⇒ `containers`; chọn container ⇒ `architecture {container}`; cache theo `(scopeKey, container, includeHidden)`. Container mặc định = chứa nhiều `changedFiles` nhất theo tiền tố `ContainerRef.path`. Bố cục cố định theo hexagonal: cột 0 `grpc-server`, 1 `usecase`, 2 `domain`, 3 `adapter`+`grpc-client`, 4 externals, dưới cột 1 `config`+`other`; `C4LayerBand` nền không chọn. Cạnh: `strokeWidth = clamp(1+log2(count),1,5)`; loại quan hệ (7) bằng kiểu nét + tooltip/bảng; `confidence < 0.8` thêm nét đứt nhạt (kèm tooltip); cạnh chạm `changedSymbols` viền `--review-changed`; `violatesLayering` icon `TriangleAlert`. Tối đa 150 nút + 500 quan hệ ⇒ mặc định Danh sách "Đang hiển thị {n}/{total}". `view.warnings[]` hiển thị inline.

### 4.3 Nhãn "suy luận"

`C4InferredNotice` luôn có khi `hasOverrides=false`: "Sơ đồ được suy ra từ cấu trúc thư mục và quy tắc hexagonal; có thể chưa đúng ý thiết kế."; khi có: "Suy luận kèm ghi đè của {updatedBy} lúc {thời gian}" (từ `c4.get`). Mức phần tử theo `origin`. Component thiếu mô tả (`descriptionSource:'none'`) ⇒ "Chưa có mô tả" + "Thêm mô tả".

### 4.4 Lớp phủ

`changed` nếu `changedFiles` nằm dưới `packagePaths`/`path` (chuẩn hoá phân cách); `untested` theo `uncoveredSymbols`; `violation` theo `violations`+`violatesLayering`; `affected` chỉ khi có dữ liệu `impact` đã tải.

### 4.5 Soạn, kiểm tra, lưu

`Sheet` rộng tối thiểu 560 px, `Textarea` đơn cách, danh sách vấn đề. `c4.get {container}` tải bản ghi (không có: `version:0`, nháp rỗng + khối chú thích; mẫu khởi tạo chỉ khi O-12 chốt, hoặc dùng `seedSource` gợi ý). `validateC4OverrideDocument(text)` (debounce 300 ms): (1) ≤ 64 KiB byte UTF-8; (2) `parseDocument(text, {uniqueKeys:true, prettyErrors:true, maxAliasCount:100})` với `linePos`; (3) gốc là mapping; (4) schema `zod` khi O-12 chốt. Lưu: `c4.save {container, document, expectedVersion}`; nút khoá ngay; thành công ⇒ `version` mới, hiển thị `warnings`, vô hiệu cache `architecture` và tải lại. `VERSION_CONFLICT` ⇒ ba hành động (Tải bản mới có xác nhận; Ghi đè bản của tôi — tải lại `version` rồi lưu, `confirmVariant:'destructive'`; Sao chép bản của tôi); `INVALID_PARAMS {field}` ⇒ vào danh sách vấn đề; `PAYLOAD_TOO_LARGE` ⇒ `validation`; `NOT_AUTHORIZED` ⇒ chỉ đọc; `offline/timeout` ⇒ giữ nháp "Chưa lưu". Nháp `c4Drafts` ≤ 8 container, ≤ 64 KiB; đóng có thay đổi hỏi xác nhận.

## 5. Quyết định thiết kế

Bố cục cố định theo lớp (xác định); luôn nhãn suy luận; không tính trước kết quả ghi đè ở client (tải lại sơ đồ do server dựng); kiểm tra cục bộ chỉ cú pháp/kích thước/gốc; ghi đè xung đột phải xác nhận `destructive`; danh sách thay thế luôn có; không thêm thư viện (`yaml`, `zod` có sẵn).

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng |
|---|---|---|
| `architecture` | `BE-CV-SOL-033-c4-component-view`; `BE-CV-SOL-040-codeintel-view-channels` | G3 (đợt 4; fake G4 trước) |
| `c4.get/save`, bảng `c4_overrides` | `BE-CV-SOL-033-c4-overrides-yaml`; `BE-CV-SOL-040-codeintel-write-and-stream-channels`; `BE-CV-SOL-011-data-model-and-migrations` | G3 (đợt 4-5) |
| Schema `c4.yaml` | O-12 / `BE-CV-SOL-033-c4-overrides-yaml` | chưa chốt |
| Lớp phủ, điều hướng diff | SOL-053 | trước 055-03 |

## 7. Tiêu chí chấp nhận

- [ ] Chọn container đổi sơ đồ; mặc định đúng; giữ khi đổi lens.
- [ ] Component đúng cột theo `kind`; hai lần vẽ cùng dữ liệu ⇒ cùng toạ độ.
- [ ] Độ dày cạnh theo `count`; loại quan hệ phân biệt bằng nét + chú giải dựng từ cùng bảng.
- [ ] Nhãn `origin` đúng ba giá trị; banner suy luận theo `hasOverrides`.
- [ ] > 150 nút/> 500 quan hệ mặc định Danh sách và "{shown}/{total}".
- [ ] Trình soạn: lỗi cú pháp `Dòng:Cột` đặt con trỏ; > 64 KiB và gốc không phải mapping bị chặn; "Lưu" khoá ngay.
- [ ] Lưu: cập nhật `version`, hiển thị `warnings`; `VERSION_CONFLICT` ba hành động; `NOT_AUTHORIZED` chỉ đọc; offline giữ nháp.
- [ ] Không hex; `fitView` không hoạt ảnh khi reduced-motion; `translate()` 5 locale.

## 8. Kiểm thử

`c4-layer-layout.test.ts`, `c4-overlay-model.test.ts`, `c4-container-default.test.ts`, `c4-edge-style.test.ts`, `shared/c4-override-document.test.ts`, `C4OverrideEditor.test.tsx`, `ArchitectureLens.test.tsx`, `C4ComponentDetail.test.tsx`, `C4RelationsTable.test.tsx` (mock `@xyflow/react`; fake backend 073-02), `review-ui.test.ts` (`c4Drafts` giới hạn 8 và dọn), `code-intel-locale-coverage` mở rộng. Chưa chạy.

## 9. Rủi ro và điểm chưa kiểm chứng

Schema `c4.yaml` chưa có (O-12); `yaml` trong renderer chưa đo bundle (tải lười cùng lens); bố cục hexagonal xấu với container TS (`frontend`, `agent`) — cần đo; ngưỡng 500 quan hệ chưa kiểm; `var()` trong SVG xyflow; JSON hay YAML phía backend (hợp đồng nói YAML text).

## 10. Câu hỏi mở

1. Schema `c4.yaml` và vị trí seed (O-12).
2. Có `viewerCan`/trường quyền để ẩn nút sửa trước khi `forbidden`?
3. Container `agent/frontend/desktop` có cần bố cục khác?
4. `includeHidden` ảnh hưởng gì tới `overridesVersion` (cache)?

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-055-01](../tasks/FE-CV-TASK-055-01-c4-layout-edge-style-and-overlay-model.md) | Bố cục tầng, kiểu cạnh, lớp phủ C4 | P1 |
| [FE-CV-TASK-055-02](../tasks/FE-CV-TASK-055-02-c4-container-default-and-architecture-query.md) | Container mặc định và truy vấn `architecture` | P1 |
| [FE-CV-TASK-055-03](../tasks/FE-CV-TASK-055-03-c4-diagram-canvas-and-relations-table.md) | Canvas C4 và bảng quan hệ | P1 |
| [FE-CV-TASK-055-04](../tasks/FE-CV-TASK-055-04-c4-component-detail-and-origin-labels.md) | Chi tiết component và nhãn nguồn | P1 |
| [FE-CV-TASK-055-05](../tasks/FE-CV-TASK-055-05-c4-override-document-validator.md) | Kiểm tra tài liệu `c4.yaml` | P1 |
| [FE-CV-TASK-055-06](../tasks/FE-CV-TASK-055-06-c4-override-editor-save-conflict.md) | Trình soạn, lưu, xung đột | P1 |
| [FE-CV-TASK-055-07](../tasks/FE-CV-TASK-055-07-architecture-lens-registration-i18n-e2e.md) | Đăng ký lens, i18n, e2e | P1 |

Thứ tự: 055-01, 055-02, 055-05 song song → 055-03 → 055-04, 055-06 → 055-07.

## 12. Tham chiếu

`/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-055-architecture-c4-lens.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/frontend/src/shared/orca-yaml.ts`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
