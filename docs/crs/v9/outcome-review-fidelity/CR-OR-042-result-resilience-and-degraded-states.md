# CR-OR-042 — Kết quả agent bền hơn, chi phí chẩn đoán được và trạng thái suy giảm có ích

**Series:** `outcome-review-fidelity` · **Trạng thái:** 📝 Đề xuất (2026-10-21) · **Giải pháp:** BE-OR-SOL-005 §5, §6 · **Checklist:** `phase-7-coverage-and-resilience.md` (T709–T716), FE `phase-4-coverage-ui-and-e2e.md` (T409, T410)

**Đã làm trước CR này (không làm lại):** log `agent result rejected` (`3d8e998a8`); prompt nói `expected` là boolean và nút chỉ-code không vào `design.containers` (`55dc58d5c`); chặn Approve khi run context mới nhất lỗi (`c2e5052ad`); thông báo "agent không vẽ gì" cho tab sau code trống (`64c258d29`).

## 1. Vấn đề

1. **Một lỗi cục bộ làm hỏng cả kết quả** (README E10): `expected` là chuỗi, hoặc bốn container nguồn code nằm trong `design.containers` ⇒ `REQUEST_REVIEW_RESULT_INVALID`. Mỗi lần như vậy tốn 1,5–2 USD (#2318: 3,51 USD cho hai lần bị từ chối).
2. **Vòng thử lại chỉ một lần** và không có ngân sách tiền; lần hai cũng thất bại ở lỗi khác (container nguồn code) vì lỗi ấy chỉ lộ ra khi lần một đã sửa xong lỗi kiểu dữ liệu.
3. **Lý do từ chối không được lưu** với run; người dùng chỉ thấy "agent's answer could not be used". Muốn biết lý do phải đọc log container.
4. **Hỏng một bước thì mất cả thiết kế:** review sau code lỗi không còn Architecture/Flow để xem (E11), dù thiết kế đã khoá vẫn có trong kho.
5. **Chi phí vẽ khó dự đoán:** run vẽ 0,52–0,92 USD cho 4–5 hình, không có chỉ báo trước, không so được engine `agent` với `builtin`.

## 2. Phạm vi

**Có:** phân loại lỗi (cục bộ/nặng) và xử lý một phần; ép kiểu an toàn; ngân sách thử lại; lưu `error_detail`; hợp nhất thiết kế khoá vào review lỗi; đo và hiện chi phí vẽ; spike chọn engine vẽ.

**Không:** nới các quy tắc an toàn (bằng chứng, chống tự gọi, sandbox); cho Approve review đã lỗi; đổi mô hình giá.

## 3. Yêu cầu

| # | Yêu cầu |
|---|---|
| R1 | **Phân loại vi phạm.** *Cục bộ:* kiểu dữ liệu sai nhưng ép được (`expected` là `"true"/"false"` ⇒ boolean; số dạng chuỗi), container `source=code` trong `design.containers`, `expected`/`reviewState` ngoài tập giá trị. *Nặng:* không phải JSON, sai `schema`, bằng chứng tham chiếu thứ không có, `kgId` ngoài `elementsRead`, vượt kích thước. |
| R2 | Vi phạm **cục bộ** được **sửa hoặc bỏ phần đó** kèm cảnh báo (`coerced:<field>`, `design_container_code_only_dropped:<label>`); kết quả vẫn được nhận. Vi phạm **nặng** từ chối và vào vòng thử lại. |
| R3 | Vòng thử lại có **ngân sách tiền** `REQUEST_REVIEW_RETRY_BUDGET_USD` (mặc định bằng chi phí dự đoán của một run); hết ngân sách thì dừng, không thử tiếp. Số lần thử tối đa vẫn 2. |
| R4 | Lưu `error_detail` (danh sách vi phạm đã **khử bí mật**, tối đa 30 dòng như `dedupeViolations`) vào bản ghi run; trả qua `request.review.runs`; hiện trong khung "Lượt chạy của agent". |
| R5 | Khi context sau code hỏng mà có thiết kế đã khoá, review **vẫn chứa** thiết kế (hình design, verdict `designChecked`) và nhãn `codeCompared=false`; Architecture/Flow ở chế độ After code nói rõ "chưa so với code vì lượt đọc code lỗi". Approve vẫn bị chặn. |
| R6 | Chi phí vẽ: mỗi run vẽ ghi số hình, chi phí thực, chi phí mỗi hình; `request.review.estimate` trả ước tính cho "Vẽ flow này" và cho sinh lại. |
| R7 | **Spike** đo chất lượng/chi phí giữa engine `agent` (skill `diagram-design`) và `builtin` trên cùng mô hình của #2318; kết luận ghi vào SOL-005 trước khi đổi mặc định `auto` (chưa quyết, README O3). |
| R8 | Chỉ số mới: tỷ lệ kết quả bị từ chối theo loại vi phạm; số lần dùng ngân sách thử lại; chi phí trung bình mỗi hình; tỷ lệ flow ứng viên `undrawable`. |

## 4. Tiêu chí chấp nhận

| # | Kiểm | Cách đo |
|---|---|---|
| AC1 | Kết quả có `expected: "true"` ⇒ được nhận, `expected = true`, cảnh báo `coerced:items.expected` | test domain, dùng đúng khối đã làm hỏng #2318 |
| AC2 | Kết quả có 4 container `source=code` trong thiết kế ⇒ nhận, 4 container bị bỏ, 4 cảnh báo | test domain |
| AC3 | Kết quả không phải JSON ⇒ từ chối, vào retry; hết ngân sách ⇒ không retry | test usecase |
| AC4 | Run lỗi có `error_detail` với tối đa 30 dòng, không chứa chuỗi giống bí mật (canary) | test usecase + test redaction |
| AC5 | Context sau code lỗi + có lock ⇒ review có hình design và `codeCompared=false`; Approve vẫn trả `REQUEST_REVIEW_CONTEXT_FAILED` | test usecase + vitest |
| AC6 | Hai lần chạy thật liên tiếp trên BL-E05 không còn lần nào bị từ chối vì hai lỗi của #2318 | e2e thật |
| AC7 | Báo cáo spike R7 có bảng: số hình, chi phí, thời gian, kết quả kiểm cho cả hai engine | tài liệu trong SOL-005 |

## 5. Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Sửa/bỏ phần lỗi che mất agent làm sai | Mọi sửa/bỏ đều thành cảnh báo hiện trong khung chạy và đo bằng chỉ số R8; lỗi nặng vẫn từ chối. |
| `error_detail` lộ dữ liệu | Khử bí mật bằng `RedactRaw` trước khi lưu; giới hạn dòng và độ dài; test canary. |
| Migration thêm cột | Cột tùy chọn, mặc định rỗng; cả hai dialect (postgres, mysql) có `down`. |
