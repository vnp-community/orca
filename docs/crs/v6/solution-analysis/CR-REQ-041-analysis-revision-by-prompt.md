# CR-REQ-041 — Chỉnh phương án phân tích bằng prompt ngay tại cổng duyệt Analysis

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-041 |
| **Tên** | Ô "Ask for changes" ở bước Analysis: người dùng gõ yêu cầu chỉnh, agent sinh phiên bản mới của Solution, chẩn đoán, Findings hoặc Answer; danh sách phiên bản và so sánh; không giới hạn số lần chỉnh |
| **Loại** | Feature (chủ yếu frontend, kèm bổ sung nhỏ ở backend) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-19 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai (câu hỏi mở 1 và 2 đã chốt 2026-10-19; còn 1 câu mở) |
| **Phụ thuộc** | CR-REQ-007 (sinh Solution, `feedback`, `TriggerAnalysisRevision`), CR-REQ-008 (chẩn đoán, Findings, Answer chạy qua agent chỉ-đọc), CR-REQ-010 (Approval), CR-REQ-016 (kênh gateway), CR-REQ-018 đến 021 (trang Request), CR-REQ-028 (Clarification), CR-REQ-037 (tab Activity) |
| **Mở khoá** | Không |
| **Tác động** | `backend-go/services/request-service` (`usecase/generate_solution.go`, `solution_authorization.go`, `list_solutions.go`, `domain/request_activity.go`, migration nhỏ), `backend-go/proto/orca/request/v1/solution.proto`, `backend-go/services/api-gateway` (`solution.*`, MCP), `frontend/src/renderer/src/components/request/solution/`, `request/activity/`, 5 file locale |

> Nguồn: yêu cầu "ở bước Analysis cho phép người dùng prompt lại để agent điều chỉnh phương án đề xuất" ngày 2026-10-19. Hiện trạng dựa trên đọc mã (mục 1), chưa dựng giao diện hay chạy thử.

---

## 1. Bối cảnh và vấn đề

### 1.1 Backend đã có đường "sinh lại kèm phản hồi"

- `GenerateSolutionRequest.feedback` (tối đa 2000 ký tự, `maxFeedbackRunes`). `checkAnalysisState` cho phép gọi khi Request ở `analyzing`, **hoặc ở `awaiting_analysis_approval` khi có `feedback`** (`generate_solution.go`).
- Trên đường đó, trong cùng một transaction: huỷ Approval đang chờ với lý do `revision_requested`, đánh dấu các Decision và đề xuất đang mở là `superseded`, và chuyển Request về `analyzing` bằng `TriggerAnalysisRevision`. Lượt chạy mới nhận `Feedback` và đưa vào prompt như dữ liệu không phải lệnh (`solution_prompt.go`, khối `<feedback>`), kèm các phiên bản trước và nhận xét của người duyệt (`priorArtifacts`).
- Cả Solution lẫn chẩn đoán/Findings/Answer dùng chung bộ dựng prompt đó: `RunAgentReadonlyAnalysis` gọi `BuildSolutionPrompt` với `Feedback` và `PriorArtifacts`. Nên đường chỉnh bằng phản hồi có sẵn cho mọi loại phân tích có cổng duyệt.
- Ai được sinh lại hiện nay: `AuthorizeGenerate` = người báo cáo Request, hoặc vai trò `admin`/`lead`/`owner`. Người có quyền duyệt (approver) mà không thuộc nhóm đó không sinh lại được, dù `AuthorizeChoose` đã cho họ chọn phương án (`ApproverAwareAuthorizer`). CR này sửa điểm lệch đó (mục 2.8).

### 1.2 Giao diện chỉ đi đường vòng

`useSolutionDecision.reject(comment, regenerate)` thực hiện hai lời gọi liên tiếp: từ chối Approval kèm lý do, rồi (nếu bật) gọi sinh lại với `feedback` là lý do đó. Người dùng muốn "chỉnh một chút" lại phải đi qua hành động từ chối; không có nơi xem các phiên bản cũ; lỗi giữa hai lời gọi để lại Request ở trạng thái lưng chừng. Tôi chưa kiểm chứng từ chối có đưa Request về backlog hay không.

### 1.3 Vấn đề

1. Không có cách chỉnh nhẹ một phương án đã đề xuất mà không từ chối nó.
2. Không thấy lịch sử các phiên bản và yêu cầu chỉnh đã dẫn tới chúng.
3. Phản hồi của người duyệt không lộ ra API để giao diện hiển thị (chỉ nằm trong `analysis_runs.feedback`).

## 2. Giải pháp đề xuất

### 2.1 Phạm vi

Áp dụng cho mọi loại Request có cổng duyệt Analysis (`AnalysisGate` khác rỗng trong `request_flow_registry.go`) và đúng với loại tài liệu nó sinh:

| Loại Request | Tài liệu | Có chỉnh bằng prompt |
|---|---|---|
| `change_request`, `refactor` | Solution (nhiều phương án) | Có |
| `bug`, `security`, `performance` | Chẩn đoán (kind `solution`) | Có |
| `spike` | Findings | Có |
| `question` | Answer | Có |
| `hotfix` | Chẩn đoán, **không có cổng duyệt** | Không (không có điểm dừng `awaiting_analysis_approval`) |
| `task`, `docs`, `ops_request` | Không có Analysis | Không áp dụng |

Với `spike` và `question`, Request hoàn tất ngay sau khi duyệt, nên chỉnh chỉ có ý nghĩa trước khi duyệt.

### 2.2 Luồng

1. Request ở `awaiting_analysis_approval`, người dùng có quyền sinh lại thấy ô **Ask for changes** dưới tài liệu hiện tại.
2. Gõ yêu cầu (tối đa 2000 ký tự, bộ đếm hiện sẵn), hoặc bấm chip gợi ý để chèn câu mẫu. Bấm **Revise analysis**.
3. Frontend gọi `solution.generate` với `feedback` và `idempotencyKey` mới. **Không từ chối Approval trước.**
4. Backend đi đường sẵn có (mục 1.1). Request về `analyzing`; giao diện hiện "Analyzing version n+1" và khoá Approve, Revise, Choose option.
5. Xong, Request về `awaiting_analysis_approval` với phiên bản mới và Approval mới. Bản cũ nhãn Superseded.
6. Lặp lại tuỳ ý. Duyệt thì sang `planning`; từ chối vẫn là lối thoát riêng, không đổi.

Nếu agent cần hỏi thêm trong lúc chỉnh, Request đi sang `awaiting_information` theo luồng Clarification hiện có (CR-REQ-028); trả lời xong phân tích tự chạy tiếp và **vẫn mang theo phản hồi đã nhập** (kiểm tra ở tiêu chí 4).

### 2.3 Danh sách phiên bản

- Khối "Versions" liệt kê mọi tài liệu cùng loại của Request, mới nhất trước: `Version n`, ghi chú rút gọn từ yêu cầu chỉnh (80 ký tự), thời điểm, trạng thái (Current, Superseded, Rejected, Approved).
- Bấm một phiên bản cũ thì xem được nội dung ở chế độ chỉ đọc, nhãn "Superseded", không có Approve hay Choose. Chọn so sánh hai phiên bản hiện cạnh nhau ở mức phương án (tên phương án, tóm tắt); so sánh từng dòng nằm ngoài CR này.
- Số phiên bản dùng `Solution.seq` (đã có, trường 12; cấp theo Request ở `MintSolutionID` khi tài liệu được lập chỉ mục). Lượt chạy thất bại trước khi lập chỉ mục có thể không có `seq`; giao diện không đếm các bản đó thành phiên bản.

### 2.4 Bổ sung ở backend

| Thay đổi | Lý do |
|---|---|
| `Solution` thêm trường `revision_feedback` (chuỗi, rỗng cho bản đầu): yêu cầu chỉnh đã sinh ra phiên bản này. Lấy từ `analysis_runs.feedback` của `generation_run_id`; không cần cột mới | Giao diện hiển thị ghi chú và nội dung yêu cầu |
| `ListSolutions` đã nhận lọc `status`; giao diện gọi không lọc để lấy cả `superseded`. Xác nhận trả đủ và phân trang ổn định | Danh sách phiên bản |
| Activity kind mới `analysis.revision_requested` (cấp Request, tác nhân `user`), tóm tắt "Requested changes to the analysis"; **không chứa nội dung phản hồi** | Tab Activity (CR-REQ-037) thấy vòng chỉnh; giữ C9 (nội dung Request/Solution không vào log hay sự kiện) |
| MCP: `solution_generate` đã nhận `feedback`; thêm `revision_feedback` vào kết quả đọc | Agent khác đọc được lịch sử chỉnh |

Nội dung phản hồi là dữ liệu của người dùng: lưu như hiện nay trong `analysis_runs.feedback`, trả về cho người có quyền đọc Request, không ghi vào log, span hay khung sự kiện, và vẫn được bọc `fenceUntrusted` khi đưa vào prompt.

### 2.5 Giới hạn và bảo vệ (quyết định: không giới hạn số lần chỉnh)

Không có trần số lần chỉnh. Các rào đã có vẫn áp dụng và CR này **không nới**:

- Một Request chỉ có một lượt phân tích chạy tại một thời điểm (lease); gọi chồng trả lỗi trạng thái sai.
- `MaxAgentRunsPerProject`: số lượt agent chỉ-đọc đồng thời tối đa theo dự án (`StartRunOptions`).
- Khoá idempotency trên mỗi lần bấm, tránh sinh trùng khi bấm đúp.
- Giao diện hiện chi phí của từng phiên bản nếu có (`AnalysisRun.stats`: token, thời lượng) để người dùng tự cân nhắc. Không hiện khi lượt chạy không báo số liệu.

Ngân sách AI theo tenant (CR-REQ-034) hiện chưa có (`AiBudgetAdminService` trả `Unimplemented`), nên không có rào chi phí tổng. Ghi nhận ở rủi ro R1.

### 2.6 Giao diện

Thư mục `frontend/src/renderer/src/components/request/solution/`:

- `AnalysisRevisionPanel.tsx`: textarea, bộ đếm, chip gợi ý, nút Revise analysis; trạng thái đang chạy; lỗi.
- `AnalysisVersionList.tsx`: danh sách phiên bản, chế độ xem phiên bản cũ.
- `useAnalysisRevision.ts`: gọi `generate` trực tiếp (dùng lại `actions.generate` của `useSolutionDecision`), thay vì đi qua `reject`. `useSolutionDecision.reject` giữ nguyên cho luồng từ chối.
- Chip gợi ý theo loại tài liệu (nhãn tiếng Anh; 5 locale): Solution "Make it smaller", "Keep the public API", "Avoid a migration", "Compare with another option"; chẩn đoán "Look deeper at the root cause", "Check the logs path too"; Findings/Answer "Cover X as well", "Shorten the answer".
- Mỗi file dưới 300 dòng mã; không thêm `max-lines` disable. Dùng token của `main.css` và primitive shadcn (`guides/STYLEGUIDE.md`).
- Trạng thái đang chạy dùng `request.subscribe` và `useRequestFlow` hiện có để chuyển `analyzing` ↔ `awaiting_analysis_approval`, không polling mới.

### 2.7 Lỗi

| Tình huống | Hiển thị |
|---|---|
| Phản hồi rỗng | "Describe what to change first" (không gọi API) |
| Quá 2000 ký tự | chặn nhập; nếu qua API: `REQUEST_SOLUTION_FEEDBACK_TOO_LONG` → "That's too long. Shorten it to 2000 characters." |
| Không có quyền (không phải reporter, admin/lead/owner, hay approver của cổng đang chờ) | ô hiện cùng chỗ với thanh quyết định (UI không biết trước quyền, giống nút Approve); server từ chối bằng `REQUEST_SOLUTION_FORBIDDEN`, giao diện hiện thông báo quyền chuẩn và giữ nguyên bản hiện tại |
| Dev server offline (`REQUEST_ANALYSIS_NO_CONNECTION` hoặc `REQUEST_SOLUTION_NO_CONNECTION`) | báo rõ, giữ nguyên bản hiện tại và Approval (kiểm tra trước khi ghi nên không mất gì) |
| Request không còn ở `awaiting_analysis_approval` (người khác vừa duyệt) | toast trung tính, làm mới |
| Lượt chạy thất bại | thẻ lỗi có nút Try again; bản cũ đã `superseded` nên cần bảo đảm Request không kẹt: xem R2 |

### 2.8 Quyền chỉnh (quyết định 2026-10-19)

Người có quyền duyệt cổng Analysis đang chờ được chỉnh, cùng với người báo cáo và `admin`/`lead`/`owner`.

- Thêm `SolutionActorAuthorizer.AuthorizeRevise(ctx, req)`, cài ở `ApproverAwareAuthorizer` theo đúng mẫu `AuthorizeChoose`: cho qua nếu `Base.AuthorizeGenerate` cho qua; nếu không, duyệt các Approval **đang chờ** của Request thuộc cổng Analysis (`solution`, `findings` hoặc `answer`, không chỉ `solution` như `AuthorizeChoose`) và cho qua khi `Decider.CanDecide(ctx, req, approval)` thành công cho bất kỳ Approval nào. Nhờ vậy người duyệt theo user, team hoặc role được tính y như lúc bấm Approve (`AuthorizeApprovalDecision`).
- `GenerateSolution.prepare` dùng `AuthorizeRevise` khi lời gọi là đường chỉnh (Request ở `awaiting_analysis_approval` và có `feedback`), và vẫn dùng `AuthorizeGenerate` cho lần sinh đầu (Request ở `analyzing`). Người duyệt không được tự khởi động phân tích đầu tiên.
- Kiểm quyền xảy ra **trước** khi transaction huỷ Approval cũ (hiện `prepare` đã chạy trước `InTx`), vì sau khi huỷ người đó không còn là approver của Approval nào. Test phải khẳng định thứ tự này.
- Approval mới sau mỗi vòng chỉnh vẫn lấy approver theo chính sách như thường; người chỉnh không tự động thành người duyệt bản mới, và ràng buộc tách nhiệm (`self_approval_allowed`) áp dụng như cũ ở bước Approve, không đổi ở bước chỉnh.
- Mọi người đọc được Request đều đọc được nội dung phản hồi (`revision_feedback`) trong danh sách phiên bản; không có tầng ẩn riêng cho nội dung phản hồi.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| E1 | Chỉnh là hành động riêng, không đi qua "từ chối" | Từ chối là quyết định; chỉnh là hội thoại |
| E2 | Không giới hạn số lần chỉnh | Quyết định của người yêu cầu 2026-10-19 |
| E3 | Áp dụng cho Solution, chẩn đoán, Findings, Answer | Quyết định của người yêu cầu; cùng bộ dựng prompt nên không cần đường mới |
| E4 | Bản cũ giữ lại ở `superseded`, xem được, không duyệt được | Có lịch sử mà không mở lại quyết định cũ |
| E5 | Activity chỉ ghi "đã yêu cầu chỉnh", không ghi nội dung | Giữ C9 |
| E6 | Frontend gọi `generate` trực tiếp | Backend đã xử lý huỷ Approval và chuyển trạng thái trong một transaction |
| E7 | Approver của cổng đang chờ được chỉnh (`AuthorizeRevise`) | Quyết định 2026-10-19: người đọc và quyết định phương án phải chỉnh được nó; cùng mẫu với `AuthorizeChoose` |
| E8 | Phản hồi đọc được bởi mọi người đọc được Request | Quyết định 2026-10-19; không thêm tầng quyền riêng |

## 4. Tiêu chí chấp nhận

- [ ] Request `change_request` ở `awaiting_analysis_approval`: gõ yêu cầu và bấm Revise analysis đưa Request về `analyzing` rồi trở lại `awaiting_analysis_approval` với phiên bản mới; Approval cũ có lý do `revision_requested`, đề xuất cũ là `superseded`.
- [ ] Lặp lại ít nhất 5 lần liên tiếp cho cùng một Request mà không bị chặn; số phiên bản tăng đúng.
- [ ] Cùng luồng chạy đúng cho `bug` (chẩn đoán), `spike` (Findings) và `question` (Answer) với claude giả.
- [ ] Phản hồi xuất hiện trong prompt của lượt mới, trong khối `<feedback>`, cùng phiên bản trước; trả lời Clarification giữa chừng vẫn giữ phản hồi.
- [ ] Danh sách phiên bản hiện đủ bản `superseded`; xem bản cũ ở chế độ chỉ đọc, không có Approve, Choose, Revise.
- [ ] `Solution.revision_feedback` trả đúng nội dung cho người có quyền đọc Request; không xuất hiện trong log, span hay khung sự kiện (kiểm bằng test log).
- [ ] Activity có `analysis.revision_requested` không chứa nội dung.
- [ ] `hotfix`, `task`, `docs`, `ops_request` không hiện ô chỉnh.
- [ ] Approver (theo user, team hoặc role) của cổng đang chờ nhưng không phải reporter/admin/lead/owner chỉnh được; Request chuyển `analyzing` và Approval cũ bị huỷ như thường.
- [ ] Người không phải reporter, admin/lead/owner, hay approver của cổng đang chờ bị từ chối `REQUEST_SOLUTION_FORBIDDEN`; Approval và bản hiện tại còn nguyên.
- [ ] Approver không khởi động được lần sinh đầu (Request ở `analyzing`): vẫn bị `AuthorizeGenerate` chặn.
- [ ] `AuthorizeRevise` chạy trước khi Approval cũ bị huỷ (test thứ tự); approver theo team và theo role đều được tính.
- [ ] Người chỉ có quyền đọc Request thấy `revision_feedback` của mọi phiên bản.
- [ ] Dev server offline: lỗi rõ, Approval và bản hiện tại còn nguyên.
- [ ] Bấm đúp Revise chỉ sinh một lượt (idempotency).
- [ ] Đủ khoá i18n 5 locale; `pnpm lint` xanh; không file nào vượt `max-lines`.

## 5. Kiểm thử

- Unit backend: `checkAnalysisState` với/không `feedback`; `ListSolutions` trả cả `superseded`; ánh xạ `revision_feedback` từ run; activity kind mới và không lộ nội dung.
- Quyền: bảng (reporter, admin, lead, owner, approver theo user/team/role, người chỉ đọc, người lạ) × (lần sinh đầu ở `analyzing`, chỉnh ở `awaiting_analysis_approval`); thứ tự kiểm quyền trước khi huỷ Approval.
- Usecase có fake: nhiều vòng liên tiếp; vòng bị gián đoạn bởi Clarification; lượt chạy thất bại giữa chừng (R2).
- Gateway: `wscompat` test cho `solution.generate` có `feedback` và kết quả `solution.list` có `revisionFeedback`; `parity_test` MCP.
- Component: panel (rỗng, đếm, chip, đang chạy, lỗi), danh sách phiên bản (bản cũ chỉ đọc), ẩn theo quyền và theo loại.
- E2E (`ui-live`, `run_ui_flow.py --ai-stub`): `change_request` và `bug` qua hộp thoại, chỉnh hai lần bằng giao diện, kiểm `solution.list` có 3 phiên bản (1 current, 2 superseded) và Approval mới sau mỗi vòng; `spike` và `question` một vòng. Ca nhiều vòng dài và Claude thật chỉ chạy khi người dùng đồng ý chi phí.
- Lưu ý khi chạy e2e thật: không sửa checkout trong lúc `--real` chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: không có trần số lần chỉnh và chưa có ngân sách AI.** Một người có thể tạo nhiều lượt agent tốn tiền. Giảm thiểu bằng hiển thị chi phí từng phiên bản và các rào ở mục 2.5; quyết định đã chốt, theo dõi sau khi CR-REQ-034 có ngân sách.
- **R2: lượt chạy thất bại sau khi bản cũ đã bị `superseded`.** Mã hiện huỷ Approval và đánh dấu đề xuất cũ ngay khi bắt đầu lượt mới; nếu lượt đó thất bại, Request có thể không còn đề xuất nào để duyệt. Cần kiểm tra cách `recover_interrupted_analysis_runs` và đường thất bại xử lý, rồi nếu cần khôi phục bản trước hoặc cho "Try again" từ `analyzing`. Chưa kiểm chứng.
- **R3 (đã giải quyết bằng E7):** người duyệt không phải reporter/admin/lead/owner trước đây không chỉnh được. Rủi ro còn lại: nhiều approver cùng chỉnh đè nhau; lượt chỉnh thứ hai khi Request đang `analyzing` bị chặn bởi trạng thái, giao diện báo "someone else is already revising" và làm mới.
- **R4: Solution tạo qua engine OpenSpec** hoặc chế độ có knowledge hub dùng prompt khác (`openspec_solution_prompt.go` không thấy xử lý `Feedback`). Cần kiểm tra phản hồi có tới được agent khi `solutionAgent=true` và repo có `.tdd-agent/config.toml`. Chưa kiểm chứng.
- **R5:** phản hồi dài kéo prompt lớn; đã có giới hạn 2000 ký tự và `truncateRunes` cho nhận xét cũ.
- Chưa chạy hệ thống; mọi mô tả dựa trên đọc mã ngày 2026-10-19.

## 7. Quyết định đã chốt và câu hỏi còn mở

**Đã chốt (người yêu cầu, 2026-10-19):**

1. Người có quyền duyệt (approver) mà không phải reporter/admin/lead/owner **được chỉnh** (E7, mục 2.8).
2. Nội dung phản hồi đọc được bởi **mọi người đọc được Request** (E8).
3. Không giới hạn số lần chỉnh (E2); áp dụng cho Solution, chẩn đoán, Findings, Answer (E3); làm theo luồng đề xuất (E1, E4, E5, E6).

**Còn mở:**

1. Có cần "hoàn tác" quay về một phiên bản cũ làm bản hiện tại không? Ngoài phạm vi; nếu cần thì sinh lại với phản hồi tham chiếu bản đó.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-007 | Ghi rõ đường sinh lại kèm phản hồi là hành động chính thức của giao diện, không chỉ của luồng từ chối |
| CR-REQ-008 | Xác nhận chẩn đoán, Findings, Answer nhận `feedback` |
| CR-REQ-016 | `solution.generate` kèm `feedback`; `solution.list` trả `revisionFeedback` |
| CR-REQ-017 | `ToolSpec` cập nhật cho kết quả đọc |
| CR-REQ-021 (frontend) | Khối Analysis thêm panel chỉnh và danh sách phiên bản |
| CR-REQ-037 | Activity có kind `analysis.revision_requested` |
| CR-REQ-010 | `AuthorizeRevise` dùng `CanDecide` của engine duyệt để xác định approver; ghi chú vào mô tả quyền |
| CR-REQ-035 | Phản hồi tuân C9 (không vào log hay sự kiện); quyền đọc phản hồi bằng quyền đọc Request |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/usecase/generate_solution.go` (`start`, `prepare`, `checkAnalysisState`), `solution_prompt.go` (`<feedback>`), `run_agent_readonly_analysis.go`, `solution_authorization.go` (`AuthorizeGenerate`, `ApproverAwareAuthorizer`), `solution_regenerator.go`, `recover_interrupted_analysis_runs.go`
- `backend-go/proto/orca/request/v1/solution.proto` (`GenerateSolutionRequest.feedback`, `Solution`, `SolutionStatus`)
- `backend-go/services/request-service/internal/domain/request_flow_registry.go` (`AnalysisGate`)
- `frontend/src/renderer/src/components/request/solution/useSolutionDecision.ts` (`reject`, `regenerate`), `RejectReasonDialog.tsx`, `solution-view-model.ts` (`clampFeedback`, 2000)
- [CR-REQ-007](./CR-REQ-007-solution-generation-options-and-selection.md), [CR-REQ-008](./CR-REQ-008-diagnosis-findings-answer-analysis.md), [CR-REQ-037](../request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
- Mới: `components/request/solution/{AnalysisRevisionPanel,AnalysisVersionList}.tsx`, `useAnalysisRevision.ts`, trường proto `Solution.revision_feedback`
