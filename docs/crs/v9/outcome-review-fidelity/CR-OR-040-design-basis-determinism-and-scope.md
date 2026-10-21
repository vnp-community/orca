# CR-OR-040 — Cơ sở thiết kế đơn điệu, nguồn flow tất định và hình kiến trúc theo Request

**Series:** `outcome-review-fidelity` · **Trạng thái:** 📝 Đề xuất (2026-10-21) · **Giải pháp:** BE-OR-SOL-005 §2, §3 · **Checklist:** `phase-6-design-basis.md`, FE `phase-4-coverage-ui-and-e2e.md` (T402, T403, T407)

## 1. Vấn đề

Cơ sở thiết kế (container, cạnh, flow) là thứ mọi so sánh sau code dựa vào. Hiện nó **do agent dựng lại mỗi lần chạy có `Force`**, nên:

1. Trích lại làm **thiết kế co lại**: 17 container, 40 cạnh, 1 flow (06:43) → 5 container, 3 cạnh, 1 flow (07:14) cho cùng một Request, cùng snapshot `snap-000005` (README E1).
2. Regenerate "đọc lại code" **thay luôn** thiết kế đã khoá (E4): người dùng không yêu cầu đọc lại KG nhưng KG vẫn bị đọc lại.
3. Flow thiết kế phụ thuộc agent có chép khối mermaid hay không (E3); nhưng ngay cả khi chép đủ, **SAD thật chỉ có 3 `sequenceDiagram`** và hai flow của #2318 chỉ có dạng chữ (E2): phải có đường cho flow dạng chữ, có kiểm soát.
4. Hình kiến trúc thiết kế **không biết Request chạm vào container nào**, cắt theo thứ tự ID thành vài hộp và "+6 more" (E5).

## 2. Phạm vi

**Có:** mô hình `DesignBasis` có phiên bản; quy tắc không thu hẹp; tách phạm vi regenerate; Orca đọc tài liệu thiết kế; `flowRef`; nguồn flow `mermaid|text`; lát cắt kiến trúc theo Request; hiển thị cơ sở thiết kế trên header.

**Không:** đổi cách agent đọc KG; đổi quyết định "Orca không đọc KG" (D3) — Orca chỉ đọc **tệp tài liệu** trong repo của project để đếm khối mermaid, không gọi MCP KG; đổi hợp đồng vẽ (CR-OR-010).

## 3. Yêu cầu

| # | Yêu cầu |
|---|---|
| R1 | Thiết kế đã trích được lưu thành **phiên bản** gắn `(snapshot, docSha256, promptVersion, schema)` và `extractedAt`; xem được lịch sử. |
| R2 | Khi trích lại cùng `(snapshot, docSha256)` mà số container, cạnh hoặc flow **giảm**, hệ thống **giữ bản cũ**, ghi cảnh báo `design_shrank` kèm số liệu trước/sau, và hiện ở header. Chỉ thay khi người dùng chọn rõ "thay bằng bản mới". |
| R3 | Phạm vi regenerate tách thành `code` (chỉ đọc code, **giữ lock**, không gọi `kg_*`), `design` (đọc lại KG/SAD) và `all`. Lý do `code_changed`, `looks_wrong` ở pha sau code mặc định `code`; `kg_changed` mặc định `design`. |
| R4 | Orca đọc tệp tài liệu thiết kế (đường dẫn lấy từ khoá `docSha256` agent khai), đếm khối `sequenceDiagram`/`flowchart`; **thiếu khối nào agent không chép thì Orca tự parse và bổ sung**; participant không ánh xạ được thì ghi `design_flow_participant_unmapped:<mục>`. |
| R5 | Mỗi `DesignFlow` có `flowRef` ổn định (mã mục SAD như `8.1`, hoặc mã yêu cầu như `FR-NTF-05` cho flow dạng chữ) và `origin` ∈ `mermaid|text`. Flow `text` **bắt buộc** có bằng chứng `source` (tài liệu, mục, dòng); thiếu thì bị từ chối ở vòng kiểm. |
| R6 | Hình kiến trúc thiết kế dựng từ **lát cắt của Request**: container chạm tới (theo file đổi và theo FR/AC), cộng hàng xóm một bước; còn lại gom "+N" có drilldown. Thứ tự giữ nút theo mức liên quan, không theo ID. |
| R7 | Header review hiện **cơ sở thiết kế**: snapshot, số container/cạnh/flow, bao nhiêu flow là `mermaid` và bao nhiêu là `text`, thời điểm trích, cảnh báo `design_shrank` nếu có, và cảnh báo nếu khác bản thiết kế mà người dùng đã duyệt ở pha thiết kế. |

## 4. Tiêu chí chấp nhận

| # | Kiểm | Cách đo |
|---|---|---|
| AC1 | Trích lại với agent giả trả bản nhỏ hơn ⇒ lock **không đổi**, có `design_shrank` | test usecase |
| AC2 | Regenerate `code` ⇒ 0 lời gọi `kg_*` của agent giả, `design_digest` trước = sau | test usecase + e2e thật |
| AC3 | SAD mẫu 3 sequence: agent chỉ chép 1 ⇒ Orca bổ sung 2, có cảnh báo `design_blocks_added` | test domain với fixture `SAD.md` của `vnp-qa-platform` |
| AC4 | Flow `text` không bằng chứng ⇒ vi phạm; có bằng chứng ⇒ vào thiết kế với `origin=text` | test domain |
| AC5 | Với #2318: hình kiến trúc thiết kế chứa `ms-integration`, `ms-bug-management`, MySQL, Kafka, Google Workspace **vì Request chạm tới**, không vì thứ tự ID | test domain + ảnh chụp e2e |
| AC6 | Header hiện "17 container · 40 cạnh · 3 flow (1 mermaid, 2 text)" cho dữ liệu mẫu | vitest |

## 5. Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Project không đặt SAD ở `docs/SAD.md` | Lấy đường dẫn từ `docSha256` (O5, chưa kiểm chứng); không có thì giữ hành vi hiện tại và ghi `design_docs_unreadable`. |
| Quy tắc không thu hẹp giữ mãi một thiết kế sai | Nút "thay bằng bản mới" có xác nhận; lịch sử phiên bản; TTL của lock vẫn áp dụng. |
| Flow `text` do agent bịa | Bằng chứng bắt buộc và được kiểm với tài liệu đã khai `docSha256`; thiếu ⇒ từ chối. |
| Đổi hợp đồng proto/gateway cho enum phạm vi regenerate | Thêm giá trị mới, giữ giá trị cũ `context` làm bí danh của `all` trong một phiên bản; kiểm bằng test parity kênh CLI/MCP. |
