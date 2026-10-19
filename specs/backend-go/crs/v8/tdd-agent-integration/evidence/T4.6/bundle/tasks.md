# Tasks — REQ-1235 (BL-H02 AI sinh test case)

Mỗi giai đoạn dưới đây tự rà soát và tự phát hành được. Các lệnh `Verify` cần MySQL tạm qua `TEST_MYSQL_DSN`
(`internal/testutil/mysql.go`); PH-6 cần stack local với `mock-ext` OpenAI.

<!-- orca:begin plan request=REQ-1235 schema=1 -->
## PH-1 Thư viện prompt, marker và schema trong `shared/pkg/aiprompt`
> Giao một package thuần, không phụ thuộc service nào: system prompt bắt buộc marker, bộ tách khối, JSON schema và bộ kiểm tra. Xong khi `go test` của package xanh và không package nào khác phải sửa.
- [ ] T1.1 Thêm package `aiprompt` với hằng marker và `CaseChatSystemPrompt(lang, cfg, projectCtx)` bắt buộc định dạng `featureCaseStart/caseExpand/featureCaseEnd` [type=feature] [h=3] [ac=AC-1]
  Serves: FR-SYS-07, BO-02, NFR-CO-01
  Code: create `backend/shared/pkg/aiprompt/prompt.go::CaseChatSystemPrompt`
  Verify: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ via `go test -C backend/shared/pkg ./aiprompt/...`
  Done when: prompt sinh ra nêu rõ marker, số case tối thiểu và áp `designConfig`/`templateConfig` được truyền vào; không import package nào của service.
- [ ] T1.2 Thêm `ParseCaseBlocks(s) []Block{Title,Body}` chịu được khoảng trắng, `##` và khối thiếu `featureCaseEnd` [type=feature] [h=2.5] [ac=AC-1]
  Serves: FR-SYS-07, BO-02
  Code: create `backend/shared/pkg/aiprompt/blocks.go::ParseCaseBlocks`
  Verify: Phản hồi không có marker được giữ nguyên via `go test -C backend/shared/pkg ./aiprompt/...`
  Done when: chuỗi không có marker trả về 0 khối (không lỗi, không đoán); khối thiếu `featureCaseEnd` bị bỏ qua có lý do rõ ràng trong test.
- [ ] T1.3 Thêm `CaseTransformSchema` và `ValidateTransform` (tên ≤ 255, ≥ 1 bước khi `STEP`, loại chuỗi rỗng) [type=feature] [h=3] [ac=AC-1,AC-3]
  Serves: FR-CAS-02, FR-SYS-07
  Code: create `backend/shared/pkg/aiprompt/transform.go::ValidateTransform`
  Verify: AC-3 JSON sai định dạng thì thử lại một lần rồi trả lỗi và không lưu case nào via `go test -C backend/shared/pkg ./aiprompt/...`
  Done when: `ValidateTransform` trả lỗi nêu tên trường sai cho từng trường hợp (tên quá dài, 0 bước với `STEP`, trường rỗng) và `CaseTransformSchema` là JSON schema dùng được cho JSON mode.
- [ ] T1.4 Viết test bảng cho `aiprompt` (prompt, tách khối, kiểm tra schema) [type=task] [h=2.5] [ac=AC-1,AC-3] [labels=test:regression] [depends=T1.1,T1.2,T1.3]
  Serves: FR-SYS-07, FR-CAS-02
  Code: create `backend/shared/pkg/aiprompt/aiprompt_test.go::TestParseCaseBlocks`
  Verify: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ via `go test -C backend/shared/pkg ./aiprompt/...`
  Done when: test bao phủ 10 khối hợp lệ, không marker, thiếu `featureCaseEnd`, JSON hỏng và JSON hợp lệ; `go test` xanh.
## PH-2 `case_ai_config` sang MySQL với ba trường bắt buộc
> Giao cấu hình đúng hình dạng UI (`CaseAiChatConfig`), lưu trong bảng của case-management, có mặc định tài liệu hoá và lỗi theo trường. Xong khi `GET`/`POST …/config` chạy không cần Redis và `dbown` xanh.
- [ ] T2.1 Thêm migration `V025__case_ai_config.sql` và khai báo bảng `case_ai_config` trong `backend/db/ownership.yaml` [type=task] [h=1.5] [ac=AC-5]
  Serves: ADR-06, INT-01
  Code: create `backend/services/qa-core/case-management/migrations/V025__case_ai_config.sql::case_ai_config`
  Verify: Chưa lưu cấu hình thì trả mặc định tài liệu hoá via `go test -C backend/services/qa-core/case-management ./migrations/...`
  Done when: bảng `case_ai_config(user_id, project_id, config JSON, update_time, PRIMARY KEY(user_id,project_id))` tạo bằng `CREATE TABLE IF NOT EXISTS`, số phiên bản không gap, và `go test -C backend/tools/dbown ./...` xanh.
- [ ] T2.2 Thay `domain.AIConfig` bằng `CaseAiChatConfig{designConfig, templateConfig}` kèm hàm mặc định tài liệu hoá [type=feature] [h=2] [ac=AC-5]
  Serves: NFR-CO-01, FR-SYS-07
  Code: modify `backend/services/qa-core/case-management/internal/domain/ai.go::CaseAiChatConfig`
  Verify: Chưa lưu cấu hình thì trả mặc định tài liệu hoá via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: `DefaultCaseAiChatConfig()` trả `caseEditType='STEP'` với `caseName/caseSteps/expectedResult` bật, các tuỳ chọn còn lại tắt; `domain.AIConfig` và `domain.AIGenerateRequest` cũ không còn được tham chiếu.
- [ ] T2.3 Thêm repository MySQL đọc/ghi `case_ai_config` theo khoá `(user_id, project_id)` [type=feature] [h=2.5] [ac=AC-5] [depends=T2.1,T2.2]
  Serves: ADR-06, INT-01
  Code: create `backend/services/qa-core/case-management/internal/repository/mysql/case_ai_config.go::CaseAIConfigRepository`
  Verify: Cấu hình đã lưu được đọc lại nguyên trạng via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/repository/mysql/... -run TestCaseAIConfig`
  Done when: upsert ghi đúng một dòng cho mỗi `(user_id, project_id)`; đọc dự án khác hoặc người dùng khác không thấy cấu hình của nhau.
- [ ] T2.4 Viết lại `GetConfig`/`SaveConfig` và handler theo hợp đồng UI, từ chối theo trường khi thiếu ba trường bắt buộc, bỏ nhánh Redis [type=feature] [h=3] [ac=AC-5] [depends=T2.3]
  Serves: FR-SYS-07, NFR-CO-01
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::SaveConfig`
  Verify: AC-5 thiếu một trong ba trường bắt buộc thì từ chối kèm lỗi cấu hình rõ ràng via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: `repository.RedisClient` không còn xuất hiện trong `internal/usecase/ai`; lỗi trả về nêu tên trường còn thiếu; `GET` trả mặc định khi chưa có dòng nào.
- [ ] T2.5 Viết test cấu hình: mặc định, lưu/đọc lại, từ chối theo từng trường bắt buộc [type=task] [h=2.5] [ac=AC-5] [labels=test:regression] [depends=T2.4]
  Serves: FR-SYS-07
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/config_test.go::TestCaseAIConfigRequiredFields`
  Verify: Lưu cấu hình thiếu trường bắt buộc bị chặn ngay khi lưu via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: ba ca thiếu `caseName`, thiếu `caseSteps`, thiếu `expectedResult` đều bị chặn và bảng không có thay đổi; `go test` xanh.
## PH-3 Cổng AI có bản fake và chat trả chuỗi markdown
> Giao đường chat thật qua một interface có `fake`, cùng cờ `AI_ENABLED`. Xong khi `POST /functional/case/ai/chat` trả chuỗi markdown có marker trong test và không còn `NotImplemented` cho chat.
- [ ] T3.1 Khai báo interface `Gate` (`Chat`, `ResolveModel`, `AppendTurn`, `ListMessages`) làm điểm nối duy nhất tới `aiclient` của CR-H01 [type=feature] [h=2]
  Serves: FR-SYS-07, ADR-12
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/gate.go::Gate`
  Verify: Cờ AI tắt thì route từ chối rõ ràng via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: interface không import `shared/pkg/aiclient` (chưa tồn tại) và `usecase/ai` biên dịch chỉ với interface.
- [ ] T3.2 Thêm `FakeGate` trả chuỗi/JSON dựng sẵn (marker đúng, thiếu `featureCaseEnd`, JSON hỏng) cho test [type=task] [h=2] [depends=T3.1]
  Serves: FR-SYS-07
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/fake.go::FakeGate`
  Verify: Phản hồi không có marker được giữ nguyên via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: `FakeGate` đếm số lần gọi (để test retry) và chọn được kịch bản trả về theo từng ca test.
- [ ] T3.3 Thêm cờ `AI_ENABLED` vào config và cổng 503 cho nhóm route `/functional/case/ai` [type=feature] [h=2.5]
  Serves: ADR-12, BRL-64, NFR-CO-02
  Code: modify `backend/services/qa-core/case-management/internal/config/config.go::LoadConfig`
  Verify: Cờ AI tắt thì route từ chối rõ ràng via `go test -C backend/services/qa-core/case-management ./internal/handler/... -run TestAIRoutes`
  Done when: `AI_ENABLED=false` ⇒ cả năm route AI trả 503 có thông điệp và không gọi `Gate`; route vẫn được mount (UI-frozen).
- [ ] T3.4 Viết lại `Chat` trả chuỗi markdown theo hợp đồng UI (bind `{prompt, chatModelId, conversationId, organizationId}`, bỏ SSE) [type=feature] [h=4] [ac=AC-1,AC-5] [depends=T3.1,T3.3]
  Serves: FR-SYS-07, BO-02, NFR-CO-01
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::Chat`
  Verify: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: `Chat` dùng `aiprompt.CaseChatSystemPrompt` với cấu hình đã kiểm, trả `data` là chuỗi (không `text/event-stream`), và `httpx.NotImplemented` không còn trong `Chat`.
- [ ] T3.5 Viết test chat: marker đúng, không marker, cờ tắt, cấu hình thiếu trường bắt buộc [type=task] [h=3] [ac=AC-1,AC-5] [labels=test:regression] [depends=T3.4]
  Serves: FR-SYS-07, NFR-CO-01
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/chat_test.go::TestChatReturnsMarkdownBlocks`
  Verify: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: test chứng minh `Chat` gọi `Gate` đúng một lần, trả ≥ 10 khối với `FakeGate` chuẩn, và bị chặn khi cấu hình thiếu trường; `go test` xanh.
## PH-4 `Transform` theo JSON schema với đúng một lần thử lại
> Giao đường chuyển đổi một khối thành `AiCaseTransformResult`, thử lại một lần rồi 502, không ghi gì. Xong khi test chứng minh đúng hai lần gọi mô hình và không có case nào được tạo.
- [ ] T4.1 Gọi JSON mode theo `CaseTransformSchema` và map sang `AiCaseTransformResult` với `stepDescription[{id,num,desc,result}]` [type=feature] [h=4] [ac=AC-1]
  Serves: FR-CAS-02, FR-SYS-07, NFR-CO-01
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::Transform`
  Verify: JSON hỏng ở lần đầu nhưng đúng ở lần thử lại via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: `num` của bước bắt đầu từ 1, `id` là UUID cục bộ, `caseEditType` lấy từ `templateConfig`, và kết quả đi qua `aiprompt.ValidateTransform`.
- [ ] T4.2 Thử lại đúng một lần kèm chỉ dẫn "chỉ trả JSON", vẫn sai thì 502 với thông điệp thật và không ghi gì [type=feature] [h=2.5] [ac=AC-3] [depends=T4.1]
  Serves: FR-SYS-07, ADR-12
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::transformOnce`
  Verify: AC-3 JSON sai định dạng thì thử lại một lần rồi trả lỗi và không lưu case nào via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: số lần gọi `Gate` đúng bằng 2 khi cả hai lần sai; lỗi trả về 502 mang thông điệp của provider (đã che khoá); không có lệnh ghi nào trong đường mã này.
- [ ] T4.3 Viết test transform: JSON hợp lệ, hỏng rồi đúng, hỏng cả hai lần (đếm số lần gọi và số case) [type=task] [h=3] [ac=AC-1,AC-3] [labels=test:regression] [depends=T4.2]
  Serves: FR-SYS-07, FR-CAS-02
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/transform_test.go::TestTransformRetriesOnce`
  Verify: AC-3 JSON sai định dạng thì thử lại một lần rồi trả lỗi và không lưu case nào via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: ba ca trên xanh và ca "hỏng cả hai lần" khẳng định `SELECT count(*)` của `functional_cases` không đổi.
## PH-5 Lưu hàng loạt: giới hạn vào cổng, một giao dịch, audit
> Giao `batch/save` thật: chặn giới hạn trước mọi lời gọi LLM, tạo toàn bộ case trong một giao dịch với một lần cấp số, ghi audit. Xong khi test rollback và test giới hạn chứng minh không có case nào sót lại.
- [ ] T5.1 Chặn > 20 khối hoặc > 100 case bằng 400 ngay sau `ParseCaseBlocks`, trước mọi lời gọi mô hình và mọi lệnh ghi [type=feature] [h=3] [ac=AC-4]
  Serves: FR-CAS-07, FR-SYS-07
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::BatchSave`
  Verify: AC-4 yêu cầu vượt giới hạn bị từ chối và không có case nào được lưu via `go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: 21 khối ⇒ 400 nêu rõ giới hạn, `FakeGate` đếm 0 lần gọi; đúng 20 khối/100 case vẫn được chấp nhận.
- [ ] T5.2 Thêm hàm repository tạo lô case + blob + custom values bằng `CreateInBatches` trong một giao dịch, theo mẫu `CopyCases` [type=feature] [h=4] [ac=AC-2,AC-6]
  Serves: INT-03, NFR-DA-04, FR-CAS-02
  Code: create `backend/services/qa-core/case-management/internal/repository/mysql/case_ai_batch.go::CreateManyWithValues`
  Verify: AC-6 lỗi giữa chừng thì không có case nào của lần lưu đó tồn tại via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/repository/mysql/... -run TestCreateManyWithValues`
  Done when: toàn bộ lệnh ghi dùng handle giao dịch được truyền vào; lỗi ở phần tử thứ k làm rollback toàn bộ lô.
- [ ] T5.3 Thêm `AddMany`: một `ev.Do`, một `AllocNums(projectID, n)`, lịch sử trong cùng tx, một `case.imported` thay N `case.created` [type=feature] [h=4] [ac=AC-2,AC-6] [depends=T5.2]
  Serves: INT-03, NFR-DA-04, FR-CAS-03, BRL-21, BRL-63
  Code: create `backend/services/qa-core/case-management/internal/usecase/case/batch_add.go::AddMany`
  Verify: Một sự kiện nhập thay cho nhiều sự kiện tạo via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/case/... -run TestAddMany`
  Done when: `AllocNums` được gọi đúng một lần cho cả lô, `num` liên tục không trùng, outbox có đúng một `case.imported` với `count = n` và không có `case.created`.
- [ ] T5.4 Viết lại `BatchSave`: transform các khối (song song ≤ 4) ngoài giao dịch rồi gọi `AddMany` một lần [type=feature] [h=3.5] [ac=AC-2,AC-4,AC-6] [depends=T5.1,T5.3]
  Serves: FR-SYS-07, FR-CAS-02, NFR-CO-01
  Code: modify `backend/services/qa-core/case-management/internal/usecase/ai/ai.go::BatchSave`
  Verify: AC-2 lưu hàng loạt các case đã chọn thành công trong một giao dịch và có audit via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/ai/...`
  Done when: một khối transform lỗi ⇒ cả lô bị từ chối và không ghi gì (OQ-7 của design.md); `grep -rn "NotImplemented" internal/usecase/ai` không còn kết quả.
- [ ] T5.5 Ghi chi tiết thay đổi cho từng case bằng `audit.RecordAdded` và khẳng định route khớp `routeTable` module `FUNCTIONAL_CASE` [type=feature] [h=2] [ac=AC-2] [depends=T5.3]
  Serves: NFR-AU-01, INT-07, FR-SYS-04
  Code: modify `backend/services/qa-core/case-management/internal/usecase/case/batch_add.go::AddMany`
  Verify: AC-2 lưu hàng loạt các case đã chọn thành công trong một giao dịch và có audit via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/case/... -run TestAudit`
  Done when: một lần `batch/save` sinh đúng một dòng `operation_log` (INSERT-only) kèm chi tiết thay đổi của từng case; lô bị rollback không để lại dòng chi tiết nào.
- [ ] T5.6 Viết test lưu hàng loạt: 21 khối ⇒ 400 và số case không đổi, lỗi ở case thứ 5 ⇒ 0 dòng, lô thành công ⇒ audit và `num` liên tục [type=task] [h=4] [ac=AC-2,AC-4,AC-6] [labels=test:regression] [depends=T5.4,T5.5]
  Serves: FR-SYS-07, NFR-AU-01, NFR-DA-04
  Code: create `backend/services/qa-core/case-management/internal/usecase/ai/batch_save_test.go::TestBatchSaveIsAllOrNothing`
  Verify: AC-6 lỗi giữa chừng thì không có case nào của lần lưu đó tồn tại via `TEST_MYSQL_DSN=$TEST_MYSQL_DSN go test -C backend/services/qa-core/case-management ./internal/usecase/...`
  Done when: cả ba ca xanh với `REQUIRE_DB=1` (không skip) và `go test` của service xanh.
## PH-6 Nghiệm thu đầu-cuối với mock-ext và cập nhật tài liệu
> Giao bằng chứng ở mức API và giao diện trên stack local (mock-ext OpenAI), cùng bản sửa tài liệu cho yêu cầu đã đổi. Xong khi suite tích hợp và e2e của tính năng này xanh và dòng FR-SYS-07 đã được cập nhật.
- [ ] T6.1 Thêm test tích hợp `test_f3_case_ai.py` với mock-ext: chat ≥ 10 khối, transform, batch/save, case có trong `/functional/case/page` [type=task] [h=4] [ac=AC-1,AC-2,AC-3,AC-4,AC-5,AC-6]
  Serves: FR-SYS-07, FR-CAS-02, NFR-AU-01
  Code: create `tests/integrated/api/v4_f3/test_f3_case_ai.py::test_case_ai_chat_transform_batch_save`
  Verify: AC-2 lưu hàng loạt các case đã chọn thành công trong một giao dịch và có audit via `pytest tests/integrated/api/v4_f3/test_f3_case_ai.py -q`
  Done when: sáu ca tương ứng AC-1…AC-6 đều có trong tệp và xanh trên stack local với `mock-ext` OpenAI (không gọi LLM thật).
- [ ] T6.2 Cập nhật tài liệu tính năng F24 và dòng FR-SYS-07 của SRS cho đường AI thật, nêu `AI_ENABLED=false ⇒ 503` [type=task] [h=2]
  Serves: FR-SYS-07, ADR-12, NFR-CO-02
  Code: modify `docs/features/08-ai-advanced/F24_ai-integration.md::F24.1-F24.4`
  Verify: Cờ AI tắt thì route từ chối rõ ràng via `pytest tests/integrated/api/v4_f3/test_f3_case_ai.py -q -k flag_off`
  Done when: F24 tả đúng hợp đồng chat/transform/batch-save mới; dòng FR-SYS-07 trong `docs/SRS.md` của nhánh làm việc (câu yêu cầu + cột Nghiệm thu) không còn nói "trả 501" và OQ-1 của design.md đã có người duyệt; SAD không bị sửa.
- [ ] T6.3 Thêm ca e2e drawer AI đến Transform đến lưu vào module, rồi chạy lại toàn bộ test của sáu tiêu chí [type=task] [h=3.5] [ac=AC-1,AC-2] [labels=test:regression] [depends=T6.1,T6.2]
  Serves: FR-SYS-07, BO-02, NFR-CO-01
  Code: modify `tests/e2e/tms/features/test_09_ai_advanced.py::test_ai_case_generation_drawer_to_module`
  Verify: AC-1 mô tả tính năng qua chat rồi chuyển đổi nhận ít nhất 10 test case hợp lệ via `pytest tests/e2e/tms/features/test_09_ai_advanced.py -q -k ai_case_generation`
  Done when: ca e2e mở drawer, chuyển đổi một khối, lưu vào module và thấy case trong danh sách; suite PH-1…PH-5 chạy lại xanh.
<!-- orca:end plan -->
