---
cr_id: CR-OR-040, CR-OR-041, CR-OR-042
solution_id: FE-OR-SOL-004
product: orca
layer: frontend
status: draft
priority: P0
effort: 8–10 ngày
depends_on: BE-OR-SOL-005 (designBasis, flowCoverage, flowRef, errorDetail)
created: 2026-10-21
---

# FE-OR-SOL-004 — Cơ sở thiết kế, thanh phủ flow, thẻ ghép Design | After code, trạng thái suy giảm

Checklist: `tasks/phase-4-coverage-ui-and-e2e.md`. CR: `docs/crs/v9/outcome-review-fidelity/`.

## 1. Phân tích (đã đọc code, 2026-10-21)

- `OutcomeSystemDiff.tsx`: hai `DiagramColumn` (Design, After code) lấy `diagramsOf(review, section, variant)` rồi xếp mỗi hình thành một `ReviewArtifactFrame`. Không có khái niệm "một flow"; hai cột không thẳng hàng theo flow (README E7).
- Cột trống đã nói rõ khi section có hình thiết kế mà không có hình sau code (`noCodeDrawingText`, commit `64c258d29`); các trường hợp còn lại vẫn "Nothing here.".
- Huy hiệu tab: `subTabsOf` dùng `verdict.architecture.unplannedEdges` và `verdict.flow.unplannedCalls` (đếm **cạnh** của hình delta), còn hàng trong danh sách đếm **mục** (`items`) mang nhãn "Not in design". Hai cơ sở đếm khác nhau nên số trên huy hiệu có thể khác số hàng (README E12). Ảnh chụp 06:5x: huy hiệu "0 not in design" trong khi hàng đầu có nhãn đó; dữ liệu mô hình hiện tại là 1 (kiến trúc) và 4 (flow); chưa kiểm lại trên UI.
- Khi context lỗi, `OutcomeReviewStates` hiện banner "agent run failed" nhưng Architecture/Flow trống (README E11).
- `OutcomeAgentRunFacts` liệt kê số lượt và chi phí nhưng không có lý do lỗi (README E10).

## 2. Thiết kế

### 2.1 Kiểu dữ liệu (tuỳ chọn để đọc được review cũ)
`shared/outcome-review-types.ts` thêm:
```ts
designBasis?: { version: number; digest: string; snapshot: string
  counts: { containers: number; edges: number; flows: number; mermaid: number; text: number }
  extractedAt: string; warnings: string[] }
flowCoverage?: { total: number; drawn: number
  items: { flowRef: string; name: string; reason: 'code_touches' | 'requirement_link' | 'code_only'
           status: 'drawn' | 'unchanged' | 'not_applicable' | 'undrawable' | 'capped' | 'missing'
           hasDesign: boolean; hasDelta: boolean }[] }
DiagramRef.flowRef?: string
OutcomeReviewRun.errorDetail?: string[]; OutcomeReviewRun.diagramCount?: number
```

### 2.2 Header: cơ sở thiết kế
`OutcomeDesignControl` (đã có "Checked against design (snapshot …)") hiện thêm chip "17 container · 40 cạnh · 3 flow" (tooltip: bao nhiêu `mermaid`/`text`, thời điểm trích). Có `warnings` chứa `design_shrank` ⇒ dải cảnh báo "Thiết kế mới trích nhỏ hơn bản đang dùng; đang giữ bản cũ" kèm nút **"Thay bằng bản mới"** (chỉ người có quyền duyệt; xác nhận) gọi `request.review.designBasis.adopt`. Khác bản thiết kế đã duyệt ở pha thiết kế ⇒ dải cảnh báo riêng.

### 2.3 Tab Flow: thanh phủ và thẻ ghép
- **Thanh phủ** trên đầu tab Flow: "Đã vẽ 1/2 flow" (thanh + số). Dưới đó **danh sách flow chưa vẽ**: tên, lý do ứng viên ("chạm ms-integration"), trạng thái (`undrawable`/`capped`/`missing`) và lý do của agent; nút **"Vẽ flow này"** (hiện ước tính chi phí từ `request.review.estimate`, ẩn với người không có quyền sinh lại).
- **Thẻ ghép:** mỗi `flowRef` là một thẻ có tiêu đề (tên flow, mục SAD, nhãn `mới, chưa có trong thiết kế` nếu `hasDesign=false`) và hai khung **Design | After code** cạnh nhau (≥ 900 px; xếp dọc dưới đó). Thẻ thu gọn được; mặc định mở thẻ có `hasDelta` và thu gọn thẻ `unchanged`.
- Chế độ `Design` và `After code` vẫn dùng được: lọc theo chế độ nhưng giữ cùng thẻ.
- `DiagramColumn` vẫn dùng cho các section khác (Architecture, Data, Behavior); riêng Flow dùng `FlowPairCard`.

### 2.4 Tab Architecture: lát cắt
Hình thiết kế dựng từ lát cắt (BE); UI hiển thị nhãn "Lát cắt của Request: N/M container" và liên kết **"Hiện toàn bộ"** mở danh sách container đầy đủ (tái dùng `OutcomeContainerDrilldown`); nút "+N" của hình dẫn tới danh sách đó.

### 2.5 Huy hiệu bằng số hàng
`subTabsOf` bỏ `verdict.*.unplanned*` làm nguồn; đếm **hàng** có cờ `notInDesign` của section (cùng hàm `rowsOf`/`flagsOf` mà danh sách dùng) để huy hiệu luôn bằng số hàng. Verdict chỉ còn dùng cho thẻ kết luận. Test: với mọi section, huy hiệu = số hàng mang nhãn.

### 2.6 Trạng thái suy giảm
Khi `review.model.warnings` có `code_compare_failed`: chế độ `After code` của Architecture/Flow hiện dải "Lượt đọc code lỗi nên chưa so với code" thay vì "Nothing here.", kèm lý do ngắn từ `errorDetail` của run mới nhất (tối đa 3 dòng) và nút **"Thử lại"** (sinh lại phạm vi `code`). Chế độ `Design` vẫn hiện hình thiết kế. Approve vẫn bị khoá (BE đã chặn; UI thêm trạng thái `disabled` với lý do `contextFailed`).

### 2.7 Khung "Lượt chạy của agent"
`OutcomeAgentRunFacts` thêm: số hình đã vẽ và chi phí mỗi hình; dòng "lỗi: …" khi `errorDetail` có (tối đa 3 dòng, "xem thêm" mở danh sách đủ); số cảnh báo `coerced:*`/`dropped:*` được nhận.

### 2.8 i18n
Mọi chuỗi mới qua `translate(...)` với khoá `auto.components.request.outcomeReview.*`, bổ sung đủ `en, es, ja, ko, zh` theo thứ tự khoá; chạy kiểm tra catalog (script `verify:localization-catalog` hiện **không tồn tại** trong `config/scripts`, nên kiểm bằng JSON hợp lệ và thứ tự khoá cho tới khi script được khôi phục).

## 3. Hợp đồng với backend

| Trường | Nguồn | Ghi chú |
|---|---|---|
| `model.designBasis`, `model.flowCoverage`, `DiagramRef.flowRef` | BE-OR-T606, T704, T707 | tuỳ chọn; vắng ⇒ UI quay về hiển thị theo `variant` như hiện nay |
| `Run.errorDetail`, `Run.diagramCount` | BE-OR-T712, T715 | vắng ⇒ không hiện dòng lỗi |
| `request.review.designBasis.adopt` | BE-OR-T605 | cần quyền duyệt |
| `request.review.estimate` cho `flowRef` | BE-OR-T715 | chi phí của đúng số hình |

## 4. Kiểm chứng

- **vitest:** thanh phủ (0/N, X/N, N/N), danh sách chưa vẽ, nút "Vẽ flow này" theo quyền, ghép theo `flowRef` khi tên khác, thẻ `hasDesign=false`, huy hiệu = số hàng (cả 7 section), dải `design_shrank`, trạng thái `code_compare_failed`, tương thích review cũ không có trường mới.
- **a11y:** thẻ thu gọn có `aria-expanded`; thanh phủ có `role="progressbar"` và nhãn chữ; trạng thái không chỉ bằng màu.
- **E2E thật BL-E05** (`ci-cd-kit-review`): tab Flow báo "Đã vẽ 2/2"; mỗi thẻ có khung Design và After code; huy hiệu = số hàng; kiểm sau regenerate phạm vi `code` thiết kế không nhỏ đi.
- **E2E không AI (agent giả):** flow ứng viên 3, agent giả vẽ 1 ⇒ danh sách chưa vẽ có 2 dòng với lý do; bấm "Vẽ flow này" ⇒ ước tính hiện và hình xuất hiện.

## 5. Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Thẻ ghép làm tab Flow dài khi có nhiều flow | Thu gọn mặc định cho thẻ `unchanged`; danh sách chưa vẽ gọn; trần 8 flow ở BE. |
| Đổi cách đếm huy hiệu làm lệch với ảnh chụp/tài liệu cũ | Ghi trong checklist; cập nhật test hiện có của `outcome-diff-view-model`. |
| Nút "Thay bằng bản mới" bị dùng nhầm | Xác nhận có nêu số liệu trước/sau; audit phía BE. |
