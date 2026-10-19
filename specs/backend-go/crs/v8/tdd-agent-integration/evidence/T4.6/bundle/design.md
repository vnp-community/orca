## Context

- **Hiện trạng mã**: `internal/usecase/ai/ai.go:23-29` trả `httpx.NotImplemented` cho `Chat`/`Transform`; `BatchSave` (`ai.go:31-50`) gọi `caseUC.Add` trong một vòng lặp, mỗi case một giao dịch riêng ⇒ lỗi ở case thứ k để lại k-1 case; `GetConfig/SaveConfig` (`ai.go:52-72`) lưu Redis `ai:config:case-gen:<workspaceId>` với `domain.AIConfig` (`internal/domain/ai.go`), khác hình dạng `CaseAiChatConfig` UI đọc. Route đã có sẵn ở `internal/handler/router.go:257-265`.
- **Hợp đồng UI** (Đính chính D-9, `backend/specs/crs/v4/solutions/SOL-P3c-E_files-ai-f18-f24.md` §3.3): chat trả chuỗi markdown với marker `featureCaseStart / ## <tên> / caseExpand / <thân> / featureCaseEnd`; transform trả `AiCaseTransformResult`; `batch/save` nhận `prompt` là các khối nối bằng `join('')`; config là `CaseAiChatConfig{designConfig{…}, templateConfig{caseEditType, caseName, preCondition, caseSteps, expectedResult, remark}}`, trong đó `caseName`, `caseSteps`, `expectedResult` là checkbox **disabled luôn bật** ⇒ đây là **ba trường bắt buộc** của AC-5.
- **Tri thức thiết kế** (KG snapshot `snap-000001`): BO-02, FR-SYS-07, FR-CAS-02, FR-CAS-03, BRL-21, BRL-64, ADR-06, ADR-12, INT-03, INT-07, NFR-AU-01, NFR-CO-01, NFR-CO-02, NFR-DA-04, NFR-PE-01, ENT-17, SVC-03.
- **Hạ tầng sẵn có**: `internal/events/bus.go:42 Do` (một giao dịch, outbox cùng tx), `bus.go:106 Imported` + `bus.go:62 Silent`, `repository/mysql/num_seq.go:16 AllocNums`, mẫu lô `repository/mysql/case_batch.go:129 CopyCases`, `casehistory` recorder dùng trong `usecase/case/case.go:93-101`, audit middleware `internal/middleware/audit_log.go:45` và `shared/pkg/audit/track.go:141 RecordAdded`.

## Goals / Non-Goals

**Goals:**
- Sáu tiêu chí AC-1…AC-6 có mã thật và test chứng minh, không `NotImplemented` nào còn lại trong `internal/usecase/ai`.
- Mọi lệnh ghi của `batch/save` nằm trong **một** giao dịch; giới hạn lô kiểm **trước** mọi lời gọi LLM và mọi lệnh ghi.
- Slice biên dịch và test xanh **trước** khi CR-H01 hoàn tất (nhờ interface + `fake`).

**Non-Goals:**
- Không đánh giá chất lượng prompt với LLM thật (⏸ ENV B10).
- Không thêm cột vào `functional_cases`; không sinh case từ thiết kế (F24.9) hay F24.10.
- Không sửa SAD, không sửa `services/infrastructure/integration-hub/`.

## Decisions

### D-1 — Cổng AI là một interface nhỏ trong case-management, có `fake`
`usecase/ai/gate.go` khai báo `Gate` với `Chat(ctx, req) (string, error)`, `ResolveModel`, `AppendTurn`, `ListMessages`. Bản thật bọc `shared/pkg/aiclient` + `AIService` của CR-H01; `fake.go` trả chuỗi/JSON dựng sẵn cho test.
*Thay cho*: (a) chờ CR-H01 rồi mới làm — khoá tiến độ và giữ 501 lâu hơn, trái BRL-64/NFR-CO-02; (b) gọi `aiclient` trực tiếp — không test được nếu không có provider.

### D-2 — `Transform` thử lại **đúng một lần**, rồi 502, và **không ghi gì**
Gọi JSON mode theo `CaseTransformSchema` ⇒ `json.Unmarshal` + `ValidateTransform`. Lỗi ⇒ một lần gọi lại kèm chỉ dẫn "chỉ trả JSON" ⇒ vẫn lỗi ⇒ 502 với thông điệp thật (AC-3). `Transform` không bao giờ tạo case: UI mở form cho người dùng xem và sửa.
*Thay cho*: thử lại nhiều lần (độ trễ và chi phí không giới hạn, AC-3 nói rõ "một lần"); tự sửa JSON bằng heuristic (sai âm thầm).

### D-3 — Giới hạn lô là **cổng vào**, không phải kiểm tra sau
`BatchSave` làm theo thứ tự: `ParseCaseBlocks(prompt)` ⇒ nếu `len(blocks) > 20` hoặc tổng case dự kiến `> 100` ⇒ 400 ngay, **trước** mọi lời gọi LLM và mọi lệnh ghi (AC-4) ⇒ `SELECT count(*)` của dự án không đổi.
*Thay cho*: chặn trong giao dịch rồi rollback — tốn lời gọi LLM và vẫn cấp số `num` (lỗ số).

### D-4 — Một giao dịch, một lần cấp số
Transform các khối (song song ≤ 4) **ngoài** giao dịch; sau đó `AddMany` chạy trong **một** `uc.ev.Do` (`internal/events/bus.go:42`): `AllocNums(projectID, n)` một lần (INT-03, NFR-DA-04), `CreateInBatches` cho case + blob + custom values theo mẫu `case_batch.go:129`, `casehistory.Recorder` ghi trong cùng tx (FR-CAS-03, loại IMPORT như BRL-21), rồi một `ev.Imported` + `caseev.Silent` thay N `case.created` (BRL-63: outbox cùng giao dịch). Lỗi bất kỳ ⇒ rollback toàn bộ (AC-2, AC-6).
*Thay cho*: giữ vòng lặp `caseUC.Add` (hiện trạng) — mỗi case một tx, vi phạm AC-6; hoặc một tx bọc N lần `Add` — N lần `AllocNums` tạo lỗ số và N sự kiện.

### D-5 — Audit hai lớp
Dòng `operation_log` do middleware ghi tự động vì `POST /functional/case/ai/batch/save` khớp tiền tố `/functional/case` ⇒ module `FUNCTIONAL_CASE` (`internal/middleware/audit_log.go:45-59`), INSERT-only (NFR-AU-01, INT-07). Thêm `audit.RecordAdded` cho từng case để có chi tiết thay đổi.
*Thay cho*: ghi audit thủ công trong usecase — trùng dòng và lệch module so với các route khác.

### D-6 — `case_ai_config` sang MySQL, ba trường bắt buộc
Bảng `case_ai_config(user_id, project_id, config JSON, update_time, PRIMARY KEY(user_id, project_id))`, migration `V025__case_ai_config.sql` (`CREATE TABLE IF NOT EXISTS`, không gap số) + khai báo trong `backend/db/ownership.yaml` (ADR-06). `GET` trả mặc định tài liệu hoá khi chưa lưu (`templateConfig.caseEditType='STEP'`, `caseName/caseSteps/expectedResult = true`, còn lại false). Lưu hoặc dùng mà một trong ba trường đó không bật ⇒ từ chối kèm lỗi **theo trường** (AC-5). Không backfill từ Redis: khoá cũ theo `workspaceId` là khái niệm khác.
*Thay cho*: giữ Redis (không TTL, không truy vết, không thuộc chủ bảng nào — trái ADR-06).

### D-7 — Cờ `AI_ENABLED`
Thêm vào `internal/config/config.go::Config` + `LoadConfig`. Tắt ⇒ các route AI trả 503 có thông điệp (UI không bị ẩn nút vì UI-frozen, NFR-CO-01). Đây cũng là đường rollback.

## Risks / Trade-offs

- **LLM không tuân marker** → `ParseCaseBlocks` chịu được khoảng trắng và `##`; không có marker ⇒ trả văn bản thường, **không đoán**; kịch bản mock-ext gồm: marker đúng / thiếu `featureCaseEnd` / JSON hỏng.
- **AC-1 cần ≥ 10 case hợp lệ** nhưng số case là do mô hình quyết định → prompt nêu rõ số lượng tối thiểu; nghiệm thu dùng mock-ext trả ≥ 10 khối (chất lượng với LLM thật: ⏸ ENV B10).
- **Độ trễ lời gọi LLM so với NFR-PE-01** → các route AI không nằm trong tập "đọc thường" của p95 ≤ 200 ms; cần loại trừ tường minh (open question OQ-3).
- **Transform song song ≤ 4 cho 20 khối** → tới 20 lời gọi LLM mỗi lần lưu; chi phí và timeout. Gateway không cần sửa: `upstream.response_header_timeout` mặc định 300 s (`backend/services/gateway/internal/config/config.go:201`, `internal/proxy/proxy.go:47`).
- **CR-H01 chưa có** → nhánh thật của `Gate` chỉ bật khi H01 vào; nếu H01 đổi chữ ký, `gate.go` là điểm sửa duy nhất.
- **Hợp đồng UI đổi hình dạng thân/response** (NFR-CO-01) → phải đi qua ngoại lệ UI đã đăng ký (B1 trong SOL-P3c-E §3.3) và golden route/Playwright phải được cập nhật cùng lúc.

## Migration Plan

1. Triển khai `V025__case_ai_config.sql` (chỉ thêm bảng mới ⇒ vô hại khi tính năng tắt) và cập nhật `backend/db/ownership.yaml`; `dbown` phải xanh.
2. Bật `AI_ENABLED=true` cùng cấu hình provider của CR-H01.
3. **Rollback**: `AI_ENABLED=false` ⇒ route AI từ chối ngay (503, không gọi provider). Lùi hẳn: revert commit handler/usecase; bảng `case_ai_config` để lại (không có dữ liệu nghiệp vụ); case đã lưu không bị ảnh hưởng.
4. Khoá Redis `ai:config:case-gen:*` cũ: không di trú; xoá bằng tay sau khi ổn định (không nằm trong phạm vi CR này).

## Open Questions

- **OQ-1 — FR-SYS-07**: tài liệu SRS (`docs/SRS.md:432`) vẫn yêu cầu chat/transform trả 501 khi "chưa nối AI", trong khi ADR-12/BRL-64/NFR-CO-02 cấm route 501. Thay đổi này chọn đường thật. **Ai duyệt bản sửa dòng FR-SYS-07** (câu yêu cầu + cột Nghiệm thu) — chủ SRS? PH-6 chuẩn bị bản sửa, cần người duyệt xác nhận trước khi merge. Lưu ý đường dẫn: tệp SRS mà KG lấy nguồn là `docs/SRS.md` của repo dự án (nhánh làm việc BL-H01/H02); nhánh đề xuất này chưa có tệp đó, nên T6.2 phải chạy trên nhánh có `docs/SRS.md`.
- **OQ-2 — ba trường bắt buộc của `case_ai_config`**: không có yêu cầu nào trong SRS/URD định nghĩa `case_ai_config`; ba trường bắt buộc (`caseName`, `caseSteps`, `expectedResult`) suy ra từ UI (`caseConfigModal.vue:24-37`, checkbox disabled luôn bật) qua SOL-P3c-E §3.3. Cần xác nhận đây đúng là ba trường mà AC-5 nói, hoặc tài liệu hoá chúng thành một yêu cầu mới.
- **OQ-3 — mục tiêu hiệu năng cho lời gọi LLM**: NFR-PE-01 chỉ đặt p95 ≤ 200 ms cho "thao tác đọc thông thường" và không nói gì về route gọi LLM; chưa có timeout/ngân sách token tài liệu hoá cho `chat`/`transform`/`batch/save`. Cần một chỉ tiêu riêng (hoặc loại trừ tường minh).
- **OQ-4 — kho dữ liệu của SVC-03**: SAD liệt kê `functional_case*`, `case_review*`, `test_*`; `case_ai_config` không khớp tiền tố nào. Giữ tên này (ADR-06 chỉ đòi một chủ bảng) hay đổi thành `functional_case_ai_config`? Đổi tên rẻ nhất **trước** khi migration chạy.
- **OQ-5 — chủ sở hữu cấu hình**: ENT-17 (`ai_model_source`, `ai_conversation`) thuộc system-setting, còn `case_ai_config` thuộc case-management; tài liệu không nói cấu hình AI của case thuộc thực thể nào. Cần một ENT mới hoặc mở rộng ENT-17 trong lần cập nhật SRS kế tiếp.
- **OQ-6 — "≥ 10 test case hợp lệ" (AC-1) đo ở đâu**: với mock-ext (nghiệm thu của CR này) hay với LLM thật (⏸ ENV B10)? CR này đo trên mock-ext; nếu người duyệt muốn số trên LLM thật thì AC-1 phải chờ ENV B10.
- **OQ-7 — hành vi khi lô có một khối transform lỗi**: AC-6 chỉ nói "không lưu một phần" khi **lưu** lỗi. Nếu 1/20 khối transform lỗi: từ chối cả lô, hay lưu 19 khối và báo `errorDetail`? Thiết kế hiện chọn **từ chối cả lô** (an toàn, khớp AC-6); cần xác nhận với người viết backlog.

### Gaps/conflicts tri thức đã rà
- KG `Q5` (gap) và `Q6` (drift) trả 0 dòng. Gap ở metadata snapshot không liên quan trực tiếp: 238 gap `class_mapping` cho tiền tố `AD-*`, 6 gap `definition` (ví dụ `FR-JIR` được tham chiếu nhưng không được định nghĩa), `NFR-PE-07` thiếu load profile, `NFR-RE-01` thiếu recovery objective.
- **Xung đột đã ghi nhận**: FR-SYS-07 (501) ⟷ ADR-12 + BRL-64 + NFR-CO-02 (cấm 501) — ADR-12 tự ghi đây là ngoại lệ OI-02; CR này xoá ngoại lệ cho chat/transform của case (OQ-1).
- Không có yêu cầu/quy tắc nào trong KG nói về giới hạn 20 khối / 100 case (AC-4) hay retry JSON một lần (AC-3): hai con số đến từ Request và SOL-P3c-E §3.3, không từ SRS/URD.
