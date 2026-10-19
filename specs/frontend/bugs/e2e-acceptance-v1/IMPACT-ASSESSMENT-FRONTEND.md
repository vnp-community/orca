# Đánh giá ảnh hưởng lên frontend của các bug nghiệm thu e2e

**Nguồn:** [IMPACT-ASSESSMENT backend](../../../backend-go/bugs/e2e-acceptance/v1/IMPACT-ASSESSMENT.md) và [báo cáo e2e](../../../../tests/ORCA-E2E-ACCEPTANCE-REPORT.md). Tài liệu này chỉ nêu **ảnh hưởng nhìn thấy ở giao diện** và những chỗ mã frontend góp phần làm lỗi khó thấy.

> **Phạm vi bằng chứng:** đọc mã `frontend/src` (`useClarifications.ts`, `request-errors.ts`, `ClarificationQuestionField.tsx`, `request-rpc-client.ts`). **Chưa chạy UI** trên stack thật và chưa chạy các suite Playwright `request-web`; vì vậy hành vi hiển thị bên dưới là suy ra từ mã, ghi rõ "suy luận".

## 1. Ảnh hưởng theo bug backend

| Bug backend | Điều người dùng thấy (suy luận từ mã) | Mức | Frontend có làm nặng thêm không |
|---|---|---|---|
| BUG-E2E-001 thiếu `clarification.*` | Request dừng ở "chờ thông tin" (`awaiting_information`), panel làm rõ **trống**, không có chỗ trả lời | **Critical** | **Có.** `useClarifications.ts` coi lỗi `unsupported` là "không có Clarification" và đặt `error=null`; người dùng không thấy lý do |
| BUG-E2E-002 token/OPA | mọi thao tác Request báo "không có quyền" | Critical | Một phần: lỗi `REQUEST_FORBIDDEN` hiển thị như thiếu quyền, không gợi ý đây là sự cố triển khai |
| BUG-E2E-003, 004 | tạo project gắn dev server báo lỗi chung | High | Có thể: thông báo `PROJECT_DEV_SERVER_*` chưa được dịch thành hướng dẫn |
| BUG-E2E-005 mount sau build | UI trả 403 (trang trắng) hoặc mọi thao tác bị từ chối | High | Không (lỗi hạ tầng), nhưng không có trang lỗi thân thiện khi `index` không tải được |
| BUG-E2E-006 task thiếu owner grant | cây task con trống; tiến độ cascade không cập nhật theo mong đợi | Medium | Có thể: trạng thái rỗng và trạng thái lỗi trông giống nhau |
| BUG-E2E-007 xoá task lạ báo `Internal` | thông báo lỗi hệ thống khi xoá lặp | Low | Nên coi `NotFound` khi xoá là đã xong |
| BUG-E2E-008 workflow mất user | bước workflow `failed` với thông điệp kỹ thuật sâu | High | Thông điệp lỗi thô hiển thị nguyên |
| BUG-E2E-009 Request vào project lạ | không thấy (Request mồ côi không hiện trong danh sách) | Medium | Không |
| BUG-E2E-010 không có khoá AI | phân loại không có đề xuất; "Tạo Solution/Plan" thất bại với thông báo chung | **Critical** (bước AI) | Thông báo không nói cần cấu hình tài khoản AI |
| OBS-B `time_box_hours` | ô văn bản nhận chuỗi, từ chối câu dài với lỗi về "number" | Low | **Có.** `ClarificationQuestionField` dựng ô nhập theo `kind` mà không có kiểu giá trị |
| OBS-C giới hạn ghi | thao tác liên tiếp nhanh bị từ chối | Medium (suy luận) | **Có.** `rate_limited` là một kiểu lỗi đã khai báo nhưng `REQUEST_RATE_LIMITED` **không có trong bảng ánh xạ** `CODE_TO_KIND`, nên sẽ bị phân loại `unknown` |

## 2. Lỗi phía frontend được phát hiện khi đối chiếu

| ID | Lỗi | Bằng chứng | Mức |
|---|---|---|---|
| FE-E2E-A | Lỗi `unsupported` của `clarification.list` bị nuốt im lặng | `useClarifications.ts`: `if (listed.error.kind === 'unsupported') {setError(null)}`; comment "an unsupported runtime simply has no clarifications" | **High** (che BUG-001) |
| FE-E2E-B | `REQUEST_RATE_LIMITED`, `REQUEST_FLOW_DISABLED`, `REQUEST_PROJECT_NOT_FOUND`, `REQUEST_AI_*` thiếu trong `CODE_TO_KIND` | `shared/request-errors.ts` có `rate_limited` trong union nhưng không có mã nào ánh xạ tới | Medium |
| FE-E2E-C | Không có kiểm tra "kênh frontend gọi có tồn tại ở gateway" | test frontend dùng mock `callRequestRpc` | High (lỗ hổng cho RC-A) |
| FE-E2E-D | Ô nhập câu hỏi dựng theo `kind` (`text`, `single_choice`, …) không biết kiểu giá trị | `ClarificationQuestionField.tsx` | Low |
| FE-E2E-E | Trạng thái rỗng và trạng thái lỗi của cây task không được phân biệt (suy luận) | cần xác minh ở `components/task/*` | Low |

## 3. Gom theo nguyên nhân gốc

| Gốc (xem backend) | Phía frontend | Hướng xử lý |
|---|---|---|
| RC-A hợp đồng trôi lệch | FE-E2E-A, C | kiểm tra hợp đồng kênh + không nuốt lỗi `unsupported` ([SOL-FE-E2E-001](./solutions/SOL-FE-E2E-001-rpc-contract-and-dead-end.md)) |
| RC-C/RC-E phân loại lỗi, cấu hình AI | FE-E2E-B, thông điệp AI | bảng ánh xạ lỗi đầy đủ + thông báo có hướng khắc phục ([SOL-FE-E2E-003](./solutions/SOL-FE-E2E-003-error-taxonomy-and-rate-limit.md)) |
| RC-F kiểu giá trị câu hỏi | FE-E2E-D | ô nhập theo `valueType` ([SOL-FE-E2E-002](./solutions/SOL-FE-E2E-002-typed-clarification-inputs.md)) |

## 4. Ảnh hưởng nếu không sửa

1. Dù backend sửa xong, **một lỗi hợp đồng kế tiếp sẽ lại im lặng** ở giao diện (FE-E2E-A, C): người dùng thấy Request "đang chờ" không có cách tiến.
2. Các lỗi tạm thời (giới hạn tốc độ) hiện ra như lỗi không rõ, khiến người dùng bấm lại và tự làm nặng thêm (FE-E2E-B, suy luận).
3. Người vận hành không phân biệt được "chưa cấu hình AI" với "AI lỗi".
