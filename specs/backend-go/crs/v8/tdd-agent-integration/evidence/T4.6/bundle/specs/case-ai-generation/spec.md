## ADDED Requirements

### Requirement: FR-SYS-07 Chat AI sinh khối test case qua đường thật
Hệ thống SHALL cho người dùng gửi một mô tả tính năng tới `POST /functional/case/ai/chat` và trả về **một chuỗi markdown** trong đó mỗi test case là một khối `featureCaseStart / ## <tên> / caseExpand / <thân> / featureCaseEnd`; hệ thống MUST NOT trả 501 cho route này, và khi `AI_ENABLED=false` hệ thống SHALL từ chối bằng 503 kèm thông điệp thật (không 200 giả, theo ADR-12, BRL-64, NFR-CO-02).

#### Scenario: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ
- **GIVEN** `AI_ENABLED=true` và `case_ai_config` của người dùng trong dự án đủ ba trường bắt buộc
- **WHEN** người dùng gửi một mô tả tính năng tới `POST /functional/case/ai/chat` rồi gọi `POST /functional/case/ai/transform` cho từng khối nhận được
- **THEN** phản hồi chat chứa ít nhất 10 khối `featureCaseStart … featureCaseEnd` và mỗi khối chuyển đổi được thành một test case hợp lệ theo `CaseTransformSchema` (`ValidateTransform` không báo lỗi)

#### Scenario: Phản hồi không có marker được giữ nguyên
- **GIVEN** mô hình trả lời bằng văn bản thường, không có marker
- **WHEN** người dùng gọi `POST /functional/case/ai/chat`
- **THEN** hệ thống trả đúng chuỗi đó và không suy diễn thêm khối nào

#### Scenario: Cờ AI tắt thì route từ chối rõ ràng
- **GIVEN** `AI_ENABLED=false`
- **WHEN** người dùng gọi bất kỳ route `POST /functional/case/ai/{chat,transform,batch/save}`
- **THEN** hệ thống trả 503 kèm thông điệp nêu rõ AI đang tắt và không gọi provider nào

### Requirement: Chuyển đổi khối thành dữ liệu test case với đúng một lần thử lại
Hệ thống SHALL gọi mô hình ở chế độ JSON theo `CaseTransformSchema`, kiểm tra kết quả bằng `ValidateTransform`, và nếu kết quả không phải JSON hợp lệ theo schema thì SHALL thử lại **đúng một lần** kèm chỉ dẫn chỉ trả JSON; nếu lần thử lại vẫn sai, hệ thống SHALL trả 502 với thông điệp thật. `Transform` MUST NOT ghi bất kỳ dữ liệu case nào trong mọi trường hợp.

#### Scenario: AC-3 JSON sai định dạng thì thử lại một lần rồi trả lỗi và không lưu case nào
- **GIVEN** mô hình trả đầu ra sai định dạng JSON ở cả hai lần gọi
- **WHEN** người dùng gọi `POST /functional/case/ai/transform`
- **THEN** hệ thống đã gọi mô hình đúng hai lần (lần đầu và một lần thử lại), trả 502 kèm thông điệp thật, và số case của dự án không thay đổi

#### Scenario: JSON hỏng ở lần đầu nhưng đúng ở lần thử lại
- **GIVEN** mô hình trả JSON hỏng ở lần đầu và JSON hợp lệ theo `CaseTransformSchema` ở lần thứ hai
- **WHEN** người dùng gọi `POST /functional/case/ai/transform`
- **THEN** hệ thống trả 200 với `AiCaseTransformResult` của lần thứ hai và không lưu case nào

### Requirement: Lưu hàng loạt case do AI sinh trong một giao dịch có audit
Hệ thống SHALL tách các khối từ `prompt` của `POST /functional/case/ai/batch/save`, tạo toàn bộ case của lần lưu đó trong **một** giao dịch — cấp số `num` bằng một lần `AllocNums(projectID, n)` trong chính giao dịch đó (INT-03, NFR-DA-04), ghi lịch sử case trong cùng giao dịch (FR-CAS-03, loại nhập như BRL-21) và phát một sự kiện `case.imported` qua outbox cùng giao dịch — và SHALL sinh một dòng `operation_log` INSERT-only cho mỗi lần lưu (NFR-AU-01, INT-07) kèm chi tiết thay đổi của từng case.

#### Scenario: AC-2 lưu hàng loạt các case đã chọn thành công trong một giao dịch và có audit
- **GIVEN** một danh sách khối case do AI sinh và người dùng đã chọn một số khối
- **WHEN** người dùng gọi `POST /functional/case/ai/batch/save` với các khối đã chọn
- **THEN** đúng số case đã chọn tồn tại trong module đích với `num` liên tục không trùng, và có một dòng `operation_log` module `FUNCTIONAL_CASE` cho lần lưu đó cùng chi tiết thay đổi của từng case

#### Scenario: AC-6 lỗi giữa chừng thì không có case nào của lần lưu đó tồn tại
- **GIVEN** một lần lưu hàng loạt mà việc tạo case thứ 5 thất bại
- **WHEN** giao dịch kết thúc
- **THEN** không có case nào của lần lưu đó tồn tại (số case của dự án bằng số trước khi gọi) và không có sự kiện `case.imported` nào được phát

#### Scenario: Một sự kiện nhập thay cho nhiều sự kiện tạo
- **GIVEN** một lần lưu hàng loạt 12 case thành công
- **WHEN** giao dịch commit
- **THEN** outbox có đúng một sự kiện `case.imported` với `count = 12` và không có sự kiện `case.created` nào

### Requirement: Giới hạn lô của lưu hàng loạt kiểm trước mọi lệnh ghi
Hệ thống SHALL từ chối một yêu cầu `POST /functional/case/ai/batch/save` có hơn 20 khối hoặc hơn 100 case bằng 400 kèm thông điệp nêu rõ giới hạn bị vượt, và việc kiểm tra này SHALL xảy ra **trước** mọi lời gọi mô hình và mọi lệnh ghi dữ liệu.

#### Scenario: AC-4 yêu cầu vượt giới hạn bị từ chối và không có case nào được lưu
- **GIVEN** một yêu cầu lưu hàng loạt chứa 21 khối (hoặc hơn 100 case)
- **WHEN** người dùng gọi `POST /functional/case/ai/batch/save`
- **THEN** hệ thống trả 400 nêu rõ giới hạn 20 khối / 100 case, không gọi mô hình lần nào, và số case của dự án không thay đổi

#### Scenario: Lô đúng bằng giới hạn vẫn được chấp nhận
- **GIVEN** một yêu cầu lưu hàng loạt chứa đúng 20 khối sinh ra tổng 100 case
- **WHEN** người dùng gọi `POST /functional/case/ai/batch/save`
- **THEN** hệ thống xử lý bình thường và lưu đủ 100 case trong một giao dịch
