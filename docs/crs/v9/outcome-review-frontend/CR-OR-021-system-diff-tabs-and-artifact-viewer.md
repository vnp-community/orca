# CR-OR-021 — Tab diff mức hệ thống và trình xem artifact an toàn

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-021 |
| **Tên** | Bảy tab con (Architecture, Flow, Behavior, Data, Contracts, Requirements, Config) với hàng diff `+ ~ −`, chip bằng chứng, nhãn nguồn; `ReviewArtifactFrame` (iframe sandbox không script, CSP, điền token theo giao diện), phóng to, danh sách revision và so sánh; lùi về bảng khi không có hình |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-020 (khung, hook), CR-OR-011 (`GetReviewArtifact`), CR-OR-010 (hình), CR-OR-012 (sinh lại, revision) |
| **Mở khoá** | CR-OR-022 (mục cần quyết định nằm trong các tab này) |
| **Tác động** | `frontend/src/renderer/src/components/request/outcome-review/diff/` (mới), `components/request/outcome-review/artifact/` (mới), `frontend/src/shared/outcome-review-types.ts`, `hooks/useReviewArtifact.ts` (mới), `i18n/` |

---

## 1. Bối cảnh và vấn đề

1. Mockup 2026-10-20 cho thấy bố cục: mỗi tab con có (tuỳ mục) một hình, rồi danh sách hàng diff có ký hiệu `+`/`~`/`−`, cờ (Breaking, Deviates, Regression) và ô "đã xem". Tab **Flow** (sequence) tách khỏi **Behavior** (máy trạng thái): Flow cho thấy ai gọi ai giữa các service, Behavior cho thấy trạng thái.
2. Hình đến từ skill vẽ (CR-OR-010) dưới dạng **HTML/SVG do LLM sinh**, coi là không tin cậy (D9). Frontend có sẵn mẫu hiển thị SVG an toàn ở nơi khác (`MermaidBlock` dùng `securityLevel: 'strict'` và lọc `DOMPurify`, theo CR-CV-090) và bộ lọc nguồn Mermaid ở `review-report-diagram-guard.ts` (v7); đều dành cho Mermaid, không phải HTML tuỳ ý.
3. Màu: skill dùng vai trò màu; trình xem phải điền giá trị token của Orca theo giao diện sáng/tối mà **không** tốn token vẽ lại (D6, spike S2 của CR-OR-010). Màu diff trong UI: chỉ dùng token hiện có (`main.css`), không thêm màu.
4. iframe `sandbox` **không** có `allow-scripts` và `allow-same-origin` thì trang chủ không đọc được chiều cao nội dung; cần chiều cao suy từ `viewBox`.
5. Tab Requirements có `before → after` (trạng thái trên base và trên head). Phần "Regression" (yêu cầu từng thoả nay không còn bằng chứng) cần KG (tầng C): luôn mang nhãn "Agent reported".
6. Chi phí: Architecture và Flow có **hai hình mỗi mục** (`design` và `delta`) vì người duyệt cần đối chiếu kiến trúc/luồng trước và sau code. Hình `design` dựng từ KG nên **tái dùng** giữa các lần sinh (cache theo digest thiết kế); chỉ `delta` đổi theo commit. Behavior và Data cũng có hai hình khi KG mô tả.

## 2. Giải pháp đề xuất

### 2.1 Các tab con và nguồn

| Tab | Hình (MVP) | Hàng diff | Ghi chú |
|---|---|---|---|
| **Architecture** (tab mặc định, mức container C4 level 2) | architecture delta | container và **cạnh giữa container** (giao thức) thêm/xoá/lệch/thiếu so với thiết kế; huy hiệu "n components changed" trên container kèm nút mở lens `architecture` của v7 | so ba chiều base / head / thiết kế; cạnh `deviation` có nhãn "Deviates from design" kèm id ADR là chip; cạnh `missing` (thiết kế có, code không) có nhãn "Missing from code" |
| **Flow** (mức container, C4 dynamic) | sequence | lời gọi giữa container thêm/xoá/đổi thứ tự, lời gọi không có trong thiết kế (ví dụ bỏ qua gateway), lời gọi thiết kế có nhưng code không | ≤ 6 làn; mỗi tin nhắn ghi giao thức; không tự gọi; hiển thị luồng thiết kế bên cạnh khi có |
| Behavior | state machine (`design` + `delta`) | trạng thái/chuyển tiếp đổi | Pha design: máy trạng thái đã thiết kế nếu KG mô tả; `delta` ẩn khi không có thay đổi hành vi |
| Data | database schema (`design` + `delta`) | bảng/cột thêm, đổi, bỏ; migration cần triển khai trước | `design` từ thực thể và thuộc tính trong KG; `delta` từ migration |
| Contracts | — | RPC, kênh, tool MCP, OPA; cờ Breaking | bảng |
| Requirements | — | trạng thái trước → sau, bằng chứng | bảng; "Regression" có nhãn agent |
| Config | — | biến môi trường (tên, **không** giá trị), thứ tự khởi động lại | bảng |

**Đối chiếu trước/sau code (Architecture, Flow).** Công tắc ba chế độ: `Design` (đã thiết kế, trước code, từ KG), `After code` (thay đổi so với base và thiết kế, mặc định), `Side by side` (hai khung cạnh nhau để so). Trước khi Execution xong (pha `design`) **chỉ có `Design`**: `After code` và `Side by side` bị vô hiệu kèm "Available when execution completes"; pha `delivered` mặc định `After code`. `Design` bị vô hiệu kèm giải thích khi KG vắng. Mỗi hàng diff cho biết nó có trong thiết kế / base / head (ba chấm `Design`, `Base`, `Head`); nút/cạnh `unmapped` có nhãn "Not in design".

Architecture và Flow đứng đầu thanh tab con, hiển thị số `unplanned` thay vì chỉ số mục chưa xem. Tab hiển thị số mục chưa xem. Hàng diff: ký hiệu `+`/`~`/`−` (không chỉ màu), tiêu đề, chi tiết, chip bằng chứng, cờ. Chip `kg` hiển thị id và `snapshot` kèm nút sao chép (**không** liên kết: Orca không đọc KG); chip `file` mở diff của tệp trong cây làm việc; chip `rpc|channel|migration|check` hiển thị tên. Ô "đã xem" lưu **phía client** theo `reviewId` (mất khi đổi máy; Q1).

### 2.2 `ReviewArtifactFrame`

- Tải `GetReviewArtifact(section)` chỉ khi tab con đang mở (tối đa 6 hình trên trang); dùng `if_digest` để nhận `not_modified`.
- Dựng `srcdoc` = HTML đã lưu + khối `<style>:root{…}</style>` điền biến vai trò màu từ token đang áp dụng (đọc `getComputedStyle(document.documentElement)`), + `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">` chèn đầu `<head>`. Đổi sáng/tối dựng lại `srcdoc` từ nội dung đã có, không gọi mạng.
- `<iframe sandbox="" referrerpolicy="no-referrer" loading="lazy" title="…">`: **không** `allow-scripts`, **không** `allow-same-origin`, không `allow-popups`/`allow-forms`/`allow-top-navigation`. Tên truy cập (`title`) lấy từ `<title>/<desc>` mà máy chủ đã trích (meta của artifact).
- Chiều cao: máy chủ trả `viewBox` (rộng, cao) khi lưu (bộ kiểm của CR-OR-010 đọc được); iframe đặt `aspect-ratio` theo đó, rộng 100%, cao tối đa 70 vh, cuộn trong khung nếu vượt.
- "Open larger": hộp thoại shadcn chứa cùng iframe, điều khiển bằng bàn phím, Esc đóng; phóng/thu bằng CSS `transform` ngoài iframe (không script bên trong).
- Dải thông tin dưới hình: "Drawn by an agent from the model above. It may be wrong." + loại, `skill_version`, revision, thời điểm, `origin` (auto hay regenerated); nút "Regenerate…" (CR-OR-012), "Revisions" (menu), "Compare" (mở hộp thoại hai hình cạnh nhau + danh sách nút/cạnh thêm/bớt tính xác định từ `model`).
- Hình `failed`/thiếu: không có iframe; hiện bảng của mục kèm "Drawing failed" hoặc "No diagram for this change" và nút thử lại (cần quyền).
- Văn bản thay thế: dưới hình luôn có **danh sách hàng diff** (cùng dữ liệu) nên người dùng đọc bằng đọc màn hình không phụ thuộc hình.

### 2.3 Điều chỉnh hợp đồng

- Meta của `GetReviewArtifact` thêm `viewBox`, `title`, `desc` (CR-OR-011/010 trích khi lưu).
- `DiagramModel` và mục `diagrams[]` có `variant: "design"|"delta"` (README 3.2): cả bốn mục có hình (Architecture, Flow, Behavior, Data) đều có `design` + `delta`; Behavior và Data chỉ có `design` khi KG mô tả.

### 2.4 Hiệu năng và dung lượng

Mỗi hình ≤ 256 KiB; tải theo yêu cầu; cache trong bộ nhớ theo `blob_digest`; không giữ nhiều hơn 8 `srcdoc` cùng lúc.

### 2.5 Cấu trúc file

`diff/OutcomeSystemDiff.tsx`, `diff/OutcomeDiffRow.tsx`, `diff/OutcomeEvidenceChip.tsx`, `diff/OutcomeRequirementsTable.tsx`, `diff/outcome-diff-view-model.ts`; `artifact/ReviewArtifactFrame.tsx`, `artifact/ReviewArtifactInfoBar.tsx`, `artifact/ReviewArtifactCompareDialog.tsx`, `artifact/review-artifact-srcdoc.ts` (dựng `srcdoc`, hàm thuần), `artifact/review-artifact-theme-tokens.ts`, `hooks/useReviewArtifact.ts`. Không `helpers/utils`.

## 3. Quyết định thiết kế

1. **iframe sandbox thay vì chèn SVG vào DOM**: `<style>` trong hình do agent sinh không rò sang giao diện; chấp nhận mất chọn chữ trong hình và chiều cao cố định theo tỉ lệ (Q2).
2. **Danh sách hàng luôn có** để hình chỉ là phần hỗ trợ; đạt truy cập.
3. **Token điền lúc hiển thị**: đổi theme không tốn token.
4. **Không liên kết id KG**: Orca không đọc KG; chỉ hiển thị và sao chép.
5. **Hai hình cho Architecture và Flow** (`design` tái dùng được, `delta` đổi theo commit): đối chiếu trước/sau code là mục tiêu chính của review.

## 4. Tiêu chí chấp nhận

- [ ] Mở tab Flow lần hai không gọi sinh/vẽ; chỉ `GetReviewArtifact` (hoặc `not_modified`).
- [ ] Chế độ `Side by side` hiển thị đúng hai hình `design` và `delta`; KG vắng thì `Design` bị vô hiệu kèm lý do và không có hình `design`.
- [ ] iframe không có `allow-scripts` hay `allow-same-origin`; thử nạp artifact chứa `<script>` (giả lập bộ kiểm bị bỏ qua) không thực thi và không đổi DOM trang chủ.
- [ ] Đổi giao diện sáng ↔ tối đổi màu hình mà không gọi mạng; mọi chữ trong hình đủ tương phản (kiểm tay trên fixture).
- [ ] Hình `failed`/thiếu hiển thị bảng, không để khoảng trống; nút thử lại ẩn khi không đủ quyền.
- [ ] Mỗi hàng diff có ký hiệu `+`/`~`/`−` và bằng chứng; chip `kg` có nút sao chép, không có liên kết.
- [ ] Không có hex màu cứng trong mã mới; chỉ token từ `main.css`.
- [ ] "Compare" hiển thị hai revision cạnh nhau và danh sách thay đổi mô hình, không gọi AI.
- [ ] Điều khiển bằng bàn phím được: tab con, hình phóng to, đóng bằng Esc, ô "đã xem".
- [ ] Hai render target; hình không phụ thuộc phông mạng.

## 5. Kiểm thử

- **Unit**: `review-artifact-srcdoc` (CSP, chèn token, thoát), view model hàng diff, phép tính khác biệt mô hình.
- **Component (RTL)**: các trạng thái hình (đang tải, có, thiếu, lỗi), công tắc theme, ô "đã xem", bàn phím; kiểm thuộc tính `sandbox` chính xác.
- **Bảo mật**: bộ HTML độc hại (script, `onerror`, `<meta refresh>`, `<base>`, form tự gửi, `<style>` với `@import`/`url()`) không tạo lưu lượng hay ảnh hưởng trang chủ (kiểm trong trình duyệt thật của Playwright); chung với CR-OR-030.
- **Truy cập**: kiểm đọc màn hình cho danh sách hàng thay cho hình.

## 6. Rủi ro và điểm chưa kiểm chứng

- Phụ thuộc kết quả **S2** của CR-OR-010 (hình dùng vai trò màu có điền token được không); nếu không, phải vẽ lại theo giao diện hoặc cố định màu.
- iframe sandbox có thể hành xử khác giữa Electron và trình duyệt web (CSP trong `srcdoc`, `sandbox` rỗng); phải thử trên cả hai.
- Chiều cao theo `viewBox` giả định skill luôn xuất SVG có `viewBox`; chưa kiểm chứng.
- Nhãn trong hình là tiếng Anh theo hồ sơ; không đi qua `translate()` (hình là dữ liệu do agent vẽ, không phải chuỗi UI).
- Chưa đọc `DiffViewer` về khả năng mở tệp theo dòng; chip `file` chỉ mở theo tệp.

## 7. Câu hỏi mở

- **Q1.** Lưu tiến độ "đã xem" ở backend (mất khi đổi máy nếu để client)? Đề xuất: client ở MVP, thêm vào `outcome_review_decisions` kiểu `reviewed` nếu cần.
- **Q2.** Có chuyển sang SVG nội tuyến (lọc bằng DOMPurify hồ sơ svg) để chọn chữ và tương tác tốt hơn không, sau khi đánh giá rủi ro CSS rò.
- **Q3.** Có nên có công tắc ẩn các hàng `unchanged` (mặc định ẩn).

## 8. Tham chiếu

- [README v9](../README.md) D6, D9, mục 3.8
- [CR-OR-010](../review-visuals/CR-OR-010-diagram-design-drawing-pipeline.md), [CR-OR-011](../review-visuals/CR-OR-011-review-artifact-store-and-cache.md), [CR-OR-012](../review-visuals/CR-OR-012-regenerate-review-artifacts.md)
- `frontend/src/renderer/src/components/review-map/report/review-report-diagram-guard.ts`, `components/editor/MermaidBlock.tsx`
- [CR-CV-090](../../v7/quality-gate/CR-CV-090-exportable-review-report.md)
- [STYLEGUIDE](../../../../guides/STYLEGUIDE.md)
