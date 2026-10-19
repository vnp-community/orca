## ADDED Requirements

### Requirement: Cấu hình `case_ai_config` lưu trong MySQL theo hình dạng UI
Hệ thống SHALL lưu cấu hình sinh case bằng AI trong bảng `case_ai_config` của case-management theo khoá `(user_id, project_id)` (ADR-06: mỗi bảng thuộc đúng một service, migration `V###__*.sql`), và `GET /functional/case/ai/get/config` SHALL trả hình dạng `CaseAiChatConfig{designConfig, templateConfig}` — với giá trị mặc định tài liệu hoá khi người dùng chưa lưu cấu hình nào.

#### Scenario: Chưa lưu cấu hình thì trả mặc định tài liệu hoá
- **GIVEN** người dùng chưa từng lưu cấu hình AI cho dự án
- **WHEN** người dùng gọi `GET /functional/case/ai/get/config`
- **THEN** hệ thống trả `templateConfig.caseEditType = 'STEP'` với `caseName`, `caseSteps`, `expectedResult` bật và các tuỳ chọn còn lại tắt

#### Scenario: Cấu hình đã lưu được đọc lại nguyên trạng
- **GIVEN** người dùng đã lưu một `CaseAiChatConfig` hợp lệ cho dự án
- **WHEN** người dùng gọi lại `GET /functional/case/ai/get/config`
- **THEN** hệ thống trả đúng cấu hình đã lưu của chính người dùng đó trong chính dự án đó

### Requirement: Ba trường bắt buộc của `case_ai_config`
Hệ thống SHALL từ chối việc lưu và việc dùng `case_ai_config` khi thiếu (không bật) một trong ba trường bắt buộc `templateConfig.caseName`, `templateConfig.caseSteps`, `templateConfig.expectedResult`, kèm lỗi cấu hình nêu rõ tên trường thiếu; khi bị từ chối, hệ thống MUST NOT gọi mô hình và MUST NOT ghi dữ liệu case nào.

#### Scenario: AC-5 thiếu một trong ba trường bắt buộc thì từ chối kèm lỗi cấu hình rõ ràng
- **GIVEN** `case_ai_config` của người dùng trong dự án có `templateConfig.expectedResult` tắt
- **WHEN** người dùng dùng AI sinh case (`POST /functional/case/ai/chat`, `/transform` hoặc `/batch/save`)
- **THEN** hệ thống từ chối kèm lỗi cấu hình nêu rõ trường `expectedResult` còn thiếu, không gọi mô hình và không lưu case nào

#### Scenario: Lưu cấu hình thiếu trường bắt buộc bị chặn ngay khi lưu
- **GIVEN** một thân `CaseAiChatConfig` có `templateConfig.caseName` tắt
- **WHEN** người dùng gọi `POST /functional/case/ai/save/config`
- **THEN** hệ thống trả lỗi theo trường và bảng `case_ai_config` không có thay đổi nào
