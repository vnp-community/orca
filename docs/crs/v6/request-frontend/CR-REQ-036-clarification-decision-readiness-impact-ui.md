# CR-REQ-036 — Giao diện hỏi lại, xác nhận quyết định, sẵn sàng thực thi, tác động và rủi ro

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-036 |
| **Tên** | UI cho Clarification, Decision, ReadinessReport, thẻ rủi ro và so sánh phương án theo chiều, RiskAcceptance, dải cảnh báo lệch kế hoạch, kết quả thực thi có cấu trúc |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 đến 021, 032 (`riskPresentation`, `GraphPanel`, `GraphMini`); backend CR-REQ-028 (Clarification, Decision, `awaiting_information`), CR-REQ-029 (`TaskSpec`, `ReadinessReport`, hợp đồng kết quả), CR-REQ-030 (đánh giá tác động, `RiskAcceptance`), CR-REQ-009, 010, 013; kênh CR-REQ-016 (chờ cập nhật). CR-REQ-028, 029, 030 **đang soạn bởi agent khác**; CR này dựa vào nghiên cứu, tên trường có thể phải đổi |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/request/{clarification,decision,readiness,impact,execution}/` (mới), `shared/request-types.ts` (thêm kiểu), `hooks/` (mới), `i18n/locales/*.json` |

> Nguồn: [artifact-formats-ontology-and-execution-readiness.md](../../../research/receive-request/artifact-formats-ontology-and-execution-readiness.md) mục 6, 10, 11; [impact-assessment-and-risk-scoring.md](../../../research/receive-request/impact-assessment-and-risk-scoring.md) mục 5, 6; [frontend-visualization-and-ux.md](../../../research/receive-request/frontend-visualization-and-ux.md) mục 6, 7.

---

## 1. Bối cảnh và vấn đề

README v6 và CR-REQ-019 đến 023 chỉ có: duyệt hoặc từ chối một đối tượng, chọn phương án, trả về backlog kèm lý do. Nghiên cứu ngày 2026-10-06 chỉ ra sáu khoảng trống mà người dùng sẽ gặp:

1. **Không có vòng hỏi đáp.** Thiếu dữ liệu thì Request dừng hẳn ở backlog. Cần trạng thái `awaiting_information` và chỗ trả lời ngay tại bước đang chờ.
2. **Chọn phương án không có lý do và không có chốt hai lần.** CR-REQ-020 chỉ có `chooseOption` rồi `approve`; không ghi `rationale` khi người dùng chọn khác đề xuất, không chống chọn nhầm bản cũ (`subject_digest`).
3. **Không biết task có chạy được không.** Hiện `ExecuteTask` không kiểm tra gì trước; kết quả có `ready|needs_info|spec_defect|env_defect` cần hiển thị.
4. **Không thấy rủi ro khi duyệt.** Thẻ phương án chỉ có "rủi ro" dạng chữ tự do (CR-REQ-020 2.1).
5. **Duyệt rủi ro cao chỉ bằng một cú bấm.** Cần `RiskAcceptance` từng phát hiện.
6. **Thực thi là hộp đen.** Chỉ có trạng thái task; không thấy lệch so với kế hoạch hay kết quả kiểm chứng độc lập.

Hiện trạng code (đã đọc 2026-10-06): chưa có `components/request/` nào (CR-REQ-018 đến 023 chưa triển khai). `RequestStatus` ở CR-REQ-018 có 11 giá trị, **không có** `awaiting_information`. `TaskDAGView.tsx` và Board không có khái niệm "sẵn sàng". Task execution của backend trả `stdout` cắt 8 KB (`simple_executor.go`, theo nghiên cứu), nên "kết quả có cấu trúc" chưa tồn tại ở bất kỳ đâu.

## 2. Giải pháp đề xuất

### 2.1 Cây component và vị trí trong các màn đã có

```
RequestDetailPane (CR-REQ-019)
├─ ClarificationPanel            khi status = awaiting_information (thay nút chính của bước đang chờ)
│    ├─ ClarificationQuestionList
│    │    └─ ClarificationQuestionField  (text | single | multi | file | confirm)
│    └─ ClarificationDeadlineNote
├─ RequestAnalysisTab (CR-REQ-020)
│    └─ SolutionPanel
│         ├─ RiskSummaryCard x N          (trong SolutionOptionCard)
│         ├─ SolutionDimensionTable       (mở rộng SolutionComparisonTable)
│         ├─ ImpactFindingList            (phát hiện kèm bằng chứng, mở GraphPanel)
│         └─ SolutionDecisionBar
│              ├─ DecisionRationaleField
│              ├─ HighRiskDecisionConfirmDialog
│              └─ RiskAcceptanceChecklist
└─ RequestPlanTab (CR-REQ-021)
     ├─ PlanDriftBanner  ──▶ PlanDriftReviewSheet
     ├─ PlanTree
     │    └─ PlanTaskRow ── ReadinessBadge ──▶ ReadinessReportSheet
     └─ (TaskDetail Sheet hiện có) ── ExecutionResultPanel
```

### 2.2 Clarification: trả lời ngay tại bước chờ

- Hiện khi `status = awaiting_information` (trạng thái mới, README v6 chưa có; CR-REQ-028 đề xuất). Không mở trang riêng (nghiên cứu mục 3). Bước đang chờ trong `RequestStageTimeline` hiện nhãn "Chờ bổ sung thông tin" thay vì nút chính của bước.
- Kiểu câu hỏi (theo nghiên cứu 6.1): `text`, `single` (radio), `multi` (checkbox), `file`, `confirm` (có/không). Mỗi câu hiển thị: nội dung, **lý do hỏi** (dòng phụ), **mặc định đề xuất** (điền sẵn, đánh nhãn "Đề xuất", không tự gửi), bắt buộc hay không, đối tượng trả lời.
- Nguồn câu hỏi (nhãn chip nhỏ): `readiness` (kiểm tra sẵn sàng của Request), `open_question` (Solution), `assumption` (Plan), `task_blocked` (thực thi). Người dùng thấy câu hỏi đến từ đâu.
- Đối tượng trả lời: người báo cáo hoặc người khác. Người xem không phải đối tượng thấy câu hỏi **chỉ đọc** kèm "Đang chờ `<tên>` trả lời"; không có nút gửi.
- Gửi **một lần cho cả danh sách** (`clarification.answer`), vì câu trả lời tạo phiên bản mới của Request rồi chạy lại bước đang chờ (nghiên cứu 6.1 mục 4). Nút chính "Gửi câu trả lời" khoá tới khi mọi câu bắt buộc có giá trị; lỗi từng trường hiện cạnh trường (`aria-invalid`, như STYLEGUIDE).
- Sau khi gửi: hiện "Đã nhận, AI đang chạy lại bước `<tên bước>`", Request chuyển trạng thái do backend; UI chỉ phản ánh `request.status_changed`.
- Hạn trả lời: dòng "Còn 2 ngày" (cùng cách hiển thị hạn của CR-REQ-022); hết hạn thì Request về backlog với phân loại `missing_info` (CR-REQ-023 hiện lý do).
- `file`: dùng cơ chế đính kèm hiện có của app nếu có; **chưa kiểm chứng** app có thành phần tải tệp dùng lại được (xem 7).
- Bản nháp câu trả lời giữ trong state của hook trong phiên (không `localStorage`, tránh dữ liệu nhạy cảm nằm trên máy); mất khi tải lại, hiện cảnh báo khi rời có nháp.
- Phím tắt: `Mod+Enter` gửi khi tiêu điểm trong ô văn bản (`isScreenSubmitShortcut`, nhãn `ShortcutKeyCombo`).

### 2.3 Decision: lý do và xác nhận lần hai

Mở rộng `SolutionDecisionBar` (CR-REQ-020); cùng cơ chế dùng được cho Decision khác (đề xuất chọn phạm vi, `assumption`).

| Tình huống | UI |
|---|---|
| Chọn **đúng** phương án đề xuất | Không bắt buộc lý do; ô `DecisionRationaleField` tuỳ chọn |
| Chọn **khác** đề xuất | `DecisionRationaleField` **bắt buộc**, tối thiểu 10 ký tự sau cắt khoảng trắng (cùng quy tắc `RejectReasonDialog`); nút Duyệt khoá và hiện "Cần nhập lý do khi chọn khác đề xuất" |
| Phương án rủi ro cao (`risk` ≥ `high`, hoặc có `breaking_change`, không đảo ngược, nhiều dịch vụ) | `HighRiskDecisionConfirmDialog`: liệt kê lý do rủi ro, bắt **gõ lại tên phương án** (so khớp sau cắt khoảng trắng, phân biệt hoa thường; cho dán) rồi bấm "Xác nhận chọn" |
| `subject_digest` đổi sau khi mở màn (đánh giá hoặc Solution đổi) | Banner "Nội dung vừa thay đổi, hãy xem lại" và khoá Duyệt tới khi tải lại; ghi nhận lựa chọn cũ nếu phương án còn |
| Duyệt Plan khi Solution chưa có Decision | Nút Duyệt Plan khoá kèm tooltip "Chưa ghi nhận quyết định chọn phương án" |
| Chọn lại khi Approval còn `pending` | Cho phép; danh sách lịch sử chọn (`DecisionHistoryList`: ai, lúc nào, chọn gì, lý do) |

- Xác nhận lần hai **không** dùng `variant=destructive`: chọn phương án không mất dữ liệu (STYLEGUIDE: destructive chỉ cho hành động mất dữ liệu hoặc không hoàn tác). Nút là mặc định; mức rủi ro thể hiện bằng nội dung hộp thoại và `RiskBadge`.
- Focus mặc định vào ô gõ tên; `Enter` xác nhận khi khớp; `Esc` thoát; Huỷ là nút ghost yên lặng.
- Thứ tự gọi (theo CR-REQ-020 2.3 và CR-REQ-028): ghi Decision (`decision.record`) rồi `solution.chooseOption`/`approval.approve`. Nếu CR-REQ-028 gộp thành một RPC thì hook gộp theo, UI không đổi.

### 2.4 Thẻ rủi ro và bảng so sánh phương án theo chiều

- `RiskSummaryCard` trong mỗi `SolutionOptionCard`: `RiskBadge` (chữ, icon, theo CR-REQ-032 2.5), điểm 0 đến 100, **ba lý do chính**, và dòng độ tin cậy "Đánh giá lúc `<giờ>`, dựa trên `<công cụ>`". Chưa có bản đánh giá: "Chưa đánh giá" (không hiện Thấp). `stale`: nhãn "Index cũ, chưa đánh giá được đầy đủ" (nghiên cứu mục 4.1).
- `SolutionDimensionTable`: hàng là chiều, cột là phương án. Chiều theo nghiên cứu mục 4: Kiến trúc, Tương thích hợp đồng, Dữ liệu, Phạm vi ảnh hưởng, Bảo mật và quyền, Vận hành, Chất lượng, Bất định, Quy mô; cộng hàng Effort và Quay lui (từ CR-REQ-020). Mỗi ô: mức (chữ và icon), điểm, ghi chú một dòng. Cột đề xuất được đánh dấu, **không chọn sẵn**. Ô khác biệt được đánh dấu như `SolutionComparisonTable` hiện có.
- `ImpactFindingList`: mỗi phát hiện có chiều, mức, mô tả, bằng chứng (kết quả truy vấn, đường gọi); nút "Xem trên đồ thị" mở `GraphPanel` (CR-REQ-032) đúng lens và node. Bằng chứng thô mở bằng `Sheet`, nội dung là văn bản (không HTML).
- Luật cứng hiển thị: nếu mức tổng bị nâng bởi luật kích hoạt cứng, hiện dòng "Nâng mức vì: `<luật>`".
- Giải thích bằng ngôn ngữ do AI viết được đánh nhãn "Diễn giải bởi AI"; điểm và mức do quy tắc cố định, UI **không có chỗ nào sửa điểm**.

### 2.5 RiskAcceptance và hệ quả theo mức

Theo nghiên cứu mục 5 và 6:

| Mức | Hành vi UI |
|---|---|
| Thấp | Luồng thường |
| Trung bình | Phần tác động nằm trên nút Duyệt, phải cuộn qua hoặc mở trước khi nút bật; không có "Duyệt nhanh" ở hộp duyệt (cập nhật CR-REQ-022) |
| Cao | `RiskAcceptanceChecklist`: mỗi phát hiện mức Cao trở lên một ô xác nhận **kèm lý do** (tối thiểu 10 ký tự); nút Duyệt chỉ bật khi đủ; hiện cổng `pre_deploy` bổ sung và yêu cầu cờ tính năng, đường quay lui nếu thiếu |
| Nghiêm trọng | Như Cao, thêm trạng thái "Chờ người duyệt thứ hai" (`approval.status` và `decided_by` thứ hai); nhắc chia Phase nhỏ và thử nghiệm quay lui |

- Mỗi xác nhận gửi `impact.accept {assessmentId, findingId, rationale, assessmentDigest}`; `digest` đổi thì xác nhận cũ mất hiệu lực và UI hiện lại danh sách (chống duyệt bản đánh giá cũ).
- Người không đủ vai trò (trưởng nhóm hoặc kiến trúc) thấy nội dung và dòng "Cần người duyệt có vai trò `<vai trò>`"; nút ẩn. Vai trò thật do CR-REQ-010; chỉ có `admin|user` và team (README v6 mục 8, điểm 10), nên UI chỉ hiện tên team khi backend trả.
- **Ghi đè** cổng: người có quyền thấy menu thừa "Bỏ qua cổng" yêu cầu lý do, ghi kiểm toán; không có nút chính.

### 2.6 ReadinessReport

- `ReadinessBadge` trên `PlanTaskRow` và đầu `TaskDetail`: `ready` (`CircleCheck`), `needs_info` (`MessageCircleQuestion`), `spec_defect` (`FileWarning`), `env_defect` (`ServerCrash`), kèm chữ; chưa kiểm tra thì không badge. Chỉ task làm việc (không `plan`, `phase`).
- `ReadinessReportSheet`: ba tầng theo nghiên cứu mục 10: **Cấu trúc** (đúng schema, phủ `AC-n`, có Check), **Ngữ nghĩa** (đường dẫn `scope` tồn tại, lệnh Check có thật, phụ thuộc đã `done`, kích thước trong ngân sách), **Môi trường** (dev server kết nối, `claude` đăng nhập, công cụ, tên biến môi trường có mặt, worktree sạch, Check nền xanh). Mỗi mục: đạt/không, lý do có cấu trúc, thời điểm; chỉ hiện **tên** biến môi trường, không giá trị.
- Hành động theo kết quả:

| Kết quả | Nút chính | Ghi chú |
|---|---|---|
| `ready` | "Chạy" (nút hiện có của `TaskDetail`) | Không thêm đường chạy mới |
| `needs_info` | "Trả lời câu hỏi" | Mở `ClarificationPanel`; Request về `awaiting_information` |
| `spec_defect` | "Sinh lại task" | Trả về bước Plan; hiện lý do spec sai |
| `env_defect` | "Kết nối dev server" hoặc "Báo người vận hành" | Dòng "Không tính vào số lần thử" |

- Nút "Kiểm tra sẵn sàng" gọi `readiness.check {taskId}`; việc chạy thật vẫn tự kiểm lại phía backend. Với Phase: hàng tóm tắt "7 sẵn sàng, 1 thiếu thông tin, 1 lỗi môi trường".
- Task chưa `ready` không có nút Chạy bật (tooltip nêu lý do), nối tiếp quy tắc "Phase chưa duyệt" của CR-REQ-021 2.3. Nếu runtime chưa có `readiness.*` (`unsupported`) thì giữ hành vi CR-REQ-021, không khoá.

### 2.7 Dải cảnh báo lệch kế hoạch khi thực thi

- `PlanDriftBanner` ở đầu `RequestPlanTab` khi nhận `impact.drift_detected`: "Thay đổi thực tế vượt kế hoạch. Phase `<tên>` đã tạm dừng." Kèm nút chính "Xem và duyệt lại".
- `PlanDriftReviewSheet`: bảng **dự kiến so với thực tế**: file và service đã đổi ngoài `scope`, điểm thực tế so với dự kiến, ngưỡng đã vượt, bằng chứng (`git diff` tóm tắt). Có `GraphPanel` lens `execution` kèm nhãn "lệch".
- Hành động: "Chấp nhận lệch và tiếp tục" (lý do bắt buộc, kiểm toán, yêu cầu quyền duyệt), "Trả về Plan" (Request về `planning`), "Huỷ Phase". Không có phím tắt.
- Lệch ở mức thấp hơn ngưỡng chỉ hiện chip trên `PlanTaskRow`, không dải.

### 2.8 Kết quả thực thi có cấu trúc (`ExecutionResultPanel`)

Trong `TaskDetail` (Sheet hiện có), tab "Kết quả", dựa hợp đồng kết quả nghiên cứu mục 11:

- Khối đầu: `status` (`done|blocked|failed|needs_info`), `summary`.
- `files_changed`: danh sách, mỗi file nhãn trong hoặc ngoài `scope`; ngoài `scope` nổi bật (icon và chữ).
- `checks_run`: bảng Check với hai cột **"Agent báo"** và **"Orca chạy lại"** và kết quả so với `expect`; lệch nhau được đánh dấu "Không khớp". Dòng "Orca không dựa vào lời agent" giải thích ngắn.
- Quét bí mật trong diff: đạt, không đạt, hoặc "Chưa chạy" (công cụ chưa chọn, nghiên cứu mục 16).
- `outputs` có tên và kiểu; `questions` có nút "Chuyển thành câu hỏi" mở Clarification khi `needs_info`.
- `Failure.class` (`retryable|needs_info|spec_defect|env_defect|agent_defect`) kèm định tuyến bằng lời ("Thử lại tối đa N lần", "Trả về bước Plan"...). Số lần thử còn lại nếu backend trả.
- Không có khối kết quả hoặc sai schema: "Kết quả không đúng định dạng" kèm `stdout` rút gọn (văn bản thuần) thay vì ẩn.
- Task cũ không có khối cấu trúc (trước CR-REQ-029): hiện `stdout` cũ như hiện nay; không lỗi.

### 2.9 Kênh WS và sự kiện (đề xuất, chờ CR-REQ-016 cập nhật)

Tên theo quy ước `request.*`, `solution.*`, `approval.*`, `backlog.*`; thêm bốn nhóm. Tên cuối do CR-REQ-028, 029, 030 và CR-REQ-016 chốt.

| Nhóm | Kênh (đề xuất) | Dùng ở |
|---|---|---|
| `clarification.*` | `list {requestId}`, `get {id}`, `answer {id, answers[], version}` | 2.2 |
| `decision.*` | `record {subjectType, subjectId, chosen, rationale?, subjectDigest}`, `list {subjectId}` | 2.3 |
| `impact.*` | `get {subjectType, subjectId}`, `graph` (CR-REQ-032), `accept {assessmentId, findingId, rationale, assessmentDigest}`, `drift {phaseId}` | 2.4 đến 2.7 |
| `readiness.*` | `check {taskId}`, `get {taskId}`, `list {phaseId}` | 2.6 |

Sự kiện (subject `orca.request.<entity>.<event>`, README v6 mục 8 điểm 4; kênh stream `request.subscribe`): `clarification.requested`, `clarification.answered`, `clarification.expired`, `decision.recorded`, `impact.assessed`, `impact.drift_detected`, `readiness.reported`. Mỗi sự kiện chỉ kích hoạt `refetch` (CR-REQ-018 mục 3).

Phía frontend: hook mới `useClarifications`, `useDecisions`, `useImpactAssessment`, `useReadiness`; hằng kênh ở `request-rpc-methods.ts`; lỗi qua `RequestRpcError` (CR-REQ-018). `RequestStatus` thêm `awaiting_information` (CR-REQ-018 phải đổi, mục 8).

### 2.10 Trạng thái rỗng, tải, lỗi, quyền

| Tình huống | UI |
|---|---|
| Chưa có Clarification mở | Không render `ClarificationPanel` |
| Đang sinh đánh giá | `RiskSummaryCard` skeleton + nhãn giai đoạn ("Đang truy vấn đồ thị"), hoãn 200 ms với SSH |
| Không có đánh giá (CR-REQ-030 chưa bật) | Hàng rủi ro: "Chưa đánh giá"; Duyệt hoạt động như CR-REQ-020 |
| Không có dev server | "Chưa có dev server kết nối" kèm nút kết nối (đánh giá và kiểm tra sẵn sàng cần dev server) |
| `forbidden` | Ẩn nút ghi, dòng "Bạn không có quyền ..." |
| `conflict`, `invalid_state` | Tải lại, giữ lý do đã gõ nếu còn hợp lệ |
| `unsupported` | Ẩn khối tương ứng, không lỗi đỏ |

i18n: tiền tố `auto.components.request.{clarification,decision,readiness,impact,execution}.`, đủ `en, es, ja, ko, zh`, có test phủ khoá; nhãn 5 mức rủi ro dùng chung `auto.components.graph.RiskLevel.*` (CR-REQ-032).

## 3. Quyết định thiết kế

- Hỏi lại diễn ra tại bước đang chờ, một lần gửi cho cả danh sách.
- Lý do bắt buộc khi chọn khác đề xuất và khi chấp nhận rủi ro, cùng ngưỡng 10 ký tự với các cổng khác.
- Xác nhận lần hai bằng gõ tên phương án nhưng không dùng màu `destructive`.
- UI không bao giờ sửa điểm rủi ro; chỉ chấp nhận, ghi đè (có lý do, kiểm toán).
- "Orca chạy lại" tách khỏi "Agent báo", phản ánh nguyên tắc không tin lời agent.
- Không hiện chữ "an toàn" hay "đã phân tích xong" khi chưa có kết quả.

## 4. Tiêu chí chấp nhận

- [ ] Request `awaiting_information` hiện `ClarificationPanel` đủ 5 kiểu câu hỏi; câu bắt buộc thiếu thì khoá gửi; người không phải đối tượng chỉ đọc.
- [ ] Gửi trả lời gọi `clarification.answer` một lần với cả danh sách và phản ánh `request.status_changed`.
- [ ] Chọn khác đề xuất không có lý do thì không duyệt được; chọn đúng đề xuất thì lý do tuỳ chọn.
- [ ] Phương án rủi ro cao đòi gõ đúng tên; gõ sai thì nút khoá; nút không mang `variant=destructive`.
- [ ] `subject_digest` đổi thì hiện banner và khoá Duyệt.
- [ ] Duyệt Plan khi Solution chưa có Decision bị khoá kèm lý do.
- [ ] `ReadinessBadge` hiện đủ 4 giá trị; mỗi giá trị có đúng nút chính theo bảng 2.6; `env_defect` ghi "không tính lần thử".
- [ ] `RiskSummaryCard` hiện mức (chữ và icon), điểm, ba lý do, thời điểm và công cụ; thiếu đánh giá thì "Chưa đánh giá", không hiện Thấp.
- [ ] `SolutionDimensionTable` có đủ chiều, đánh dấu cột đề xuất mà không chọn sẵn.
- [ ] Mức Cao: không Duyệt được khi còn phát hiện chưa xác nhận; mức Nghiêm trọng hiện "Chờ người duyệt thứ hai".
- [ ] `RiskAcceptance` cũ mất hiệu lực khi `assessmentDigest` đổi.
- [ ] `impact.drift_detected` hiện `PlanDriftBanner`; "Chấp nhận lệch" đòi lý do.
- [ ] `ExecutionResultPanel` tách "Agent báo" và "Orca chạy lại", đánh dấu file ngoài `scope`, hiện `Failure.class`; kết quả sai schema hiển thị thay vì ẩn.
- [ ] Runtime không có kênh mới: không lỗi đỏ, hành vi CR-REQ-020, 021 giữ nguyên.
- [ ] Không có màu hex; mọi chuỗi có 5 locale; phím tắt đúng `metaKey` Mac, `ctrlKey` nơi khác.

## 5. Kiểm thử

- Unit: bộ kiểm hợp lệ trả lời theo kiểu câu hỏi; quy tắc lý do (khác đề xuất, 10 ký tự); so khớp tên khi gõ; điều kiện bật nút Duyệt theo mức (4 mức, bảng test); phân giải hành động theo `ReadinessReport` (4 giá trị); so khớp "Agent báo" và "Orca chạy lại"; parser (chịu enum lạ).
- Component: `ClarificationPanel`, `HighRiskDecisionConfirmDialog` (focus, Enter, Esc), `RiskAcceptanceChecklist`, `SolutionDimensionTable`, `ReadinessReportSheet`, `PlanDriftBanner`, `ExecutionResultPanel` (kết quả sai schema, task cũ).
- Hook: các hook mới (huỷ khi unmount, `refetch` theo sự kiện, `conflict`).
- E2E (cần CR-REQ-028, 029, 030): Request thiếu dữ liệu, trả lời, chạy lại; chọn khác đề xuất; duyệt phương án rủi ro cao; task `env_defect`; lệch kế hoạch tạm dừng Phase.
- Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Backend (CR-REQ-028, 029, 030) chưa được viết xong; tên kênh, tên trường (`assessmentDigest`, `Failure.class`) có thể đổi. Cần đối soát khi ba CR hoàn tất.
- Nhiều lớp xác nhận (lý do, gõ tên, chấp nhận từng phát hiện, người duyệt thứ hai) có thể gây mệt mỏi bấm cho qua; chưa có nghiên cứu người dùng.
- Dữ liệu tác động có thể đánh giá thấp tác động xuyên gRPC, WS, outbox (nghiên cứu mục 9); thẻ rủi ro Thấp có thể gây tự tin sai.
- Tải tệp ở Clarification chưa rõ cơ chế lưu và giới hạn kích thước.
- Quyền "người duyệt thứ hai" và vai trò kiến trúc phụ thuộc mô hình quyền chưa có (README v6 mục 8, điểm 10).
- Quét bí mật trong diff chưa chọn công cụ; panel kết quả có thể luôn "Chưa chạy".

## 7. Câu hỏi mở

1. Decision có RPC riêng hay gộp vào `solution.chooseOption` (CR-REQ-028)? Có bao giờ cần `confirmToken` cho xác nhận lần hai phía server?
2. Có thành phần tải tệp dùng lại được ở frontend cho câu hỏi kiểu `file`?
3. Điểm và ba lý do chính do backend trả sẵn hay frontend suy ra?
4. Người duyệt thứ hai chọn thế nào (team nào, tự gán hay theo chính sách)?
5. `readiness.check` chạy đồng bộ hay trả `runId` rồi sự kiện? Có chạy lại tự động khi Check nền thay đổi?
6. Ngưỡng lệch kế hoạch nằm ở backend hay cấu hình theo tenant; UI có cho đổi?
7. Nút Chạy có bị khoá cứng khi chưa `ready`, hay chỉ cảnh báo?

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-018 | `RequestStatus` thêm `awaiting_information` (11 thành 12 giá trị) và `REQUEST_FLOW_REGISTRY`; thêm kiểu `Clarification`, `Decision`, `ImpactAssessment`, `RiskAcceptance`, `ReadinessReport`, `ExecutionResult`; thêm hằng kênh mục 2.9 |
| CR-REQ-019 | `RequestStageTimeline` hiện trạng thái chờ bổ sung; `RequestListToolbar` thêm bộ lọc nhanh "Chờ bổ sung"; header thêm nút "Trả lời" khi đúng đối tượng |
| CR-REQ-020 | `SolutionOptionCard` thêm `RiskSummaryCard`; `SolutionComparisonTable` thêm hàng theo chiều; `SolutionDecisionBar` thêm lý do bắt buộc, xác nhận lần hai, `RiskAcceptanceChecklist`; thứ tự gọi `chooseOption` rồi `approve` chèn `decision.record`; mục 7 câu 1 liên quan |
| CR-REQ-021 | `PlanTaskRow` thêm `ReadinessBadge`; tab Plan thêm `PlanDriftBanner`; nút Chạy tuân điều kiện sẵn sàng; "Duyệt Plan" cần Decision |
| CR-REQ-022 | Bỏ "Duyệt nhanh" cho Approval có mức rủi ro từ Trung bình; thêm cổng `risk_acceptance` hiển thị mở bằng "Mở"; `ApprovalRow` hiện `RiskBadge` |
| CR-REQ-023 | Lý do backlog `missing_info` và `awaiting_information` quá hạn có nhãn riêng; Execute backlog hiện `ReadinessBadge` |
| CR-REQ-016, 013 (backend) | Chờ cập nhật: thêm kênh `clarification.*`, `decision.*`, `impact.*`, `readiness.*` và sự kiện mục 2.9 |
| CR-REQ-032 | Cung cấp `riskPresentation`, `GraphMini`, `GraphPanel`; tiêu thụ ở mục 2.4, 2.7 |

## 9. Tham chiếu

- `/opt/repos/orca/docs/research/receive-request/artifact-formats-ontology-and-execution-readiness.md`, `impact-assessment-and-risk-scoring.md`, `frontend-visualization-and-ux.md`
- `/opt/repos/orca/docs/crs/v6/request-frontend/CR-REQ-019` đến `023`; `/opt/repos/orca/docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md`
- `/opt/repos/orca/docs/crs/v6/README.md` (mục 3.3, 3.5, mục 8)
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`, `components/ui/{dialog,sheet,table,textarea,checkbox,progress}.tsx`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/request/{clarification,decision,readiness,impact,execution}/*`, `hooks/{useClarifications,useDecisions,useImpactAssessment,useReadiness}.ts`
