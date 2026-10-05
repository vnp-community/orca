# CR-REQ-020 — Xem, so sánh, chọn và duyệt Solution (kèm Chẩn đoán, Findings, Answer)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-020 |
| **Tên** | Màn xem và so sánh nhiều phương án Solution, chọn phương án, duyệt, từ chối; hiển thị Chẩn đoán, Findings, Answer |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 (kiểu, `useSolutions`, `useApprovals`), CR-REQ-019 (`RequestDetailPane`, tab Phân tích); backend CR-REQ-007 (Solution), CR-REQ-008 (Chẩn đoán, Findings, Answer), CR-REQ-009 (Approval); kênh CR-REQ-016 |
| **Mở khoá** | CR-REQ-021, 022 |
| **Tác động** | `frontend/src/renderer/src/components/request/solution/` (mới), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Bước phân tích (`analyzing`, `awaiting_analysis_approval`) là cổng đầu tiên mà người dùng phải ra quyết định thật: chọn một trong nhiều phương án (`change_request`, `refactor`, `bug`, `performance`, `security`), hoặc chấp nhận một kết quả đơn (Findings của `spike`, Answer của `question`, Chẩn đoán của `bug`, `hotfix`, `security`, `performance`). README 3.4 gom bốn dạng này vào `Solution.kind` ∈ `solution|diagnosis|findings|answer` với `options` (JSON), `chosen_option`, `status` ∈ `draft|proposed|approved|rejected|superseded`.

Frontend chưa có thành phần nào gần giống: `TaskAIDecompose.tsx` chỉ đề xuất danh sách subtask rồi `AIApply`, không so sánh phương án và không có cổng duyệt. Không có UI cho `DecisionGate` (README mục 1); O4 cấm trộn với Approval.

## 2. Giải pháp đề xuất

### 2.1 Cây component

`RequestAnalysisTab` (CR-REQ-019 chừa chỗ) render `SolutionPanel`:

```
SolutionPanel                       props: requestId, requestType, requestStatus
├─ SolutionVersionSwitcher          (các Solution theo thời gian; `superseded` thu gọn)
├─ SolutionStatusBanner             (draft / proposed / approved / rejected / superseded)
├─ SolutionBody  (rẽ theo kind)
│    ├─ SolutionOptionCompare       kind=solution
│    │    ├─ SolutionOptionCard x N (title, summary, ưu, nhược, công sức, rủi ro, "Khuyến nghị")
│    │    └─ SolutionComparisonTable (bảng so sánh các tiêu chí giữa phương án, ui/table.tsx)
│    ├─ DiagnosisView               kind=diagnosis (nguyên nhân gốc, phạm vi ảnh hưởng, baseline đo)
│    ├─ FindingsView                kind=findings (câu hỏi, phát hiện, nguồn, kết luận)
│    └─ AnswerView                  kind=answer (câu trả lời, trích dẫn)
├─ SolutionGenerationState          (đang sinh, lỗi sinh, "Sinh lại")
└─ SolutionDecisionBar              (Duyệt / Từ chối / Chọn phương án, theo quyền)
     └─ RejectReasonDialog          (lý do bắt buộc)
```

Bố cục trong tab: cột trái nội dung (cuộn, `scrollbar-sleek`), thanh quyết định dính đáy. Với `solution`, hiển thị `SolutionOptionCard` dạng lưới 1 đến 3 cột theo bề rộng; nút "So sánh" chuyển sang `SolutionComparisonTable` (hàng là tiêu chí, cột là phương án, đánh dấu ô khác biệt).

### 2.2 Nguồn dữ liệu và trạng thái

- `useSolutions(requestId)` gọi `solution.list`; `generate` gọi `solution.generate`; `chooseOption` gọi `solution.chooseOption`. Cổng duyệt đọc từ `useApprovals({requestId, subjectType})` (`approval.list`), duyệt qua `approval.approve`/`approval.reject`.
- `Approval.subject_type` theo README 3.4: `solution` cho Solution và Chẩn đoán (kèm `security`, `performance`, `bug`, `refactor`), `findings` cho Findings (`spike`), `answer` cho Answer (`question`). `hotfix`: Chẩn đoán nhanh không có cổng, chỉ hiển thị, không hiện `SolutionDecisionBar`.
- Làm mới khi có `solution.proposed`, `solution.approved`, `approval.requested`, `approval.decided`; nếu không có luồng sự kiện thì polling của CR-REQ-018.
- Ánh xạ trạng thái Solution sang UI:

| `Solution.status` | Banner | Hành động hiện |
|---|---|---|
| `draft` | "AI đang soạn" + `Loader2`, hiện phần đã có nếu có | Không (chờ) |
| `proposed` | "Chờ duyệt" kèm hạn `due_at` nếu có | Chọn phương án (chỉ `solution`), Duyệt, Từ chối |
| `approved` | "Đã duyệt" (token `status-success`), phương án đã chọn được đánh dấu | "Sinh Plan" nếu loại có Plan và chưa có (CR-REQ-021) |
| `rejected` | "Bị từ chối" kèm lý do, người, thời điểm | "Sinh lại" |
| `superseded` | Thu gọn, nhãn "Bản cũ" | Không |

### 2.3 Chọn, duyệt, từ chối

- `kind=solution`: người dùng chọn đúng một phương án (radio trong `SolutionOptionCard`), nút "Duyệt phương án này". Thứ tự gửi: `solution.chooseOption {solutionId, optionId, version}` rồi `approval.approve {approvalId, comment?, version}`. Nếu bước hai lỗi thì giữ lựa chọn, báo lỗi và cho thử lại (không gọi lại bước một nếu `chosen_option` đã khớp). Chưa kiểm chứng việc backend có gộp hai bước thành một (xem mục 7).
- Số phương án nhỏ hơn 2 với `change_request`: hiện cảnh báo "Cần ít nhất 2 phương án" (README 3.4: ≥2) và khoá nút Duyệt; cho "Sinh lại".
- `diagnosis`, `findings`, `answer`: không có chọn phương án; nút "Duyệt" ("Chấp nhận" với `answer`, đúng README: người dùng chấp nhận Answer).
- **Từ chối bắt buộc lý do**: `RejectReasonDialog` có `Textarea`, tối thiểu 10 ký tự sau khi cắt khoảng trắng; nút "Từ chối" (`variant=destructive`) khoá khi chưa đủ; ô lỗi `aria-invalid` với dòng "Cần nhập lý do". Gửi `approval.reject {approvalId, comment, version}`. Không có đường tắt từ chối không lý do (kể cả từ hộp duyệt CR-REQ-022). Sau khi từ chối, ô "Sinh lại dựa trên phản hồi" mặc định bật, gọi `solution.generate {requestId, feedback: comment}` (tham số `feedback` chưa có trong README, xem mục 7).
- `Mod+Enter` (dùng `isScreenSubmitShortcut`) gửi trong hộp từ chối. Không gán phím tắt cho Duyệt để tránh duyệt nhầm.
- Xung đột phiên bản (`conflict`): tải lại, hiện banner "Solution vừa được cập nhật", giữ nguyên lựa chọn nếu phương án còn tồn tại.
- Mọi nút ghi khoá khi đang gửi (tránh bấm đôi); gửi lặp được backend bỏ qua (idempotent, README mục 6).

### 2.4 Quyền

- Hiện nút Duyệt/Từ chối/Chọn khi người xem có quyền duyệt cổng `solution`/`findings`/`answer` (nguồn: trường quyền nếu backend trả, nếu không thì hiện và xử lý lỗi `forbidden`). Người chỉ có quyền xem thấy toàn bộ nội dung, nút bị ẩn và có dòng "Bạn không có quyền duyệt".
- Chế độ chỉ đọc cũng áp dụng khi `requestStatus` đã qua `awaiting_analysis_approval` hoặc Request `cancelled`.
- Quy tắc tự duyệt (người báo cáo tự duyệt) thuộc CR-REQ-010; UI chỉ hiện lỗi `forbidden` từ backend.

### 2.5 Trạng thái rỗng, đang tải, lỗi

| Tình huống | UI |
|---|---|
| Chưa có Solution, Request `analyzing` | Skeleton + "AI đang phân tích" |
| Chưa có Solution, loại không có bước phân tích (`task`, `docs`, `ops_request`) | Không render tab Phân tích |
| Sinh lỗi (backend báo thất bại) | `SolutionGenerationState`: thông báo lỗi, nút "Sinh lại" |
| `solution.list` lỗi mạng | Banner, "Thử lại" |
| Kênh không hỗ trợ | Cả `RequestPage` ẩn (CR-REQ-018) |
| Phương án thiếu trường (`SolutionOption` thiếu `pros`) | Ô để trống kèm "Không có dữ liệu", không ném lỗi (parser) |

Nội dung dài (`content_ref`) tải theo yêu cầu: hiện tóm tắt, nút "Xem đầy đủ" mở `Sheet` (`ui/sheet.tsx`); render Markdown bằng thành phần Markdown sẵn có của app, không dùng `dangerouslySetInnerHTML`.

### 2.6 i18n

Tiền tố `auto.components.request.solution.`, đủ 5 locale: `SolutionPanel.title`, `SolutionStatusBanner.{draft,proposed,approved,rejected,superseded}`, `SolutionOptionCard.{recommended,pros,cons,effort,risk}`, `SolutionOptionCompare.needTwo`, `SolutionComparisonTable.toggle`, `DiagnosisView.{rootCause,impact,baseline}`, `FindingsView.conclusion`, `AnswerView.accept`, `SolutionDecisionBar.{approve,reject,choose,noPermission}`, `RejectReasonDialog.{title,placeholder,required,regenerate}`, `SolutionGenerationState.{generating,failed,regenerate}`, và lỗi `error.conflict`, `error.invalidState` dùng chung từ CR-REQ-018.

## 3. Quyết định thiết kế

- Một `SolutionPanel` cho cả bốn `kind`, rẽ ở `SolutionBody`, vì cổng duyệt và vòng đời giống nhau.
- So sánh dạng bảng bên cạnh thẻ: thẻ dễ đọc, bảng cho quyết định khó; cả hai dựa cùng dữ liệu.
- Từ chối bắt buộc lý do, kiểm tra ở client và dựa backend kiểm lại; lý do được dùng để sinh lại.
- Không gộp với `DecisionGate` (README O4); khác biệt về nơi hiển thị là cố ý.
- Khoá điều hướng nhầm: Duyệt không có phím tắt.

## 4. Tiêu chí chấp nhận

- [ ] `solution` với 2 phương án trở lên hiện thẻ và bảng so sánh; `change_request` có 1 phương án không duyệt được.
- [ ] Chọn phương án rồi Duyệt gửi `chooseOption` rồi `approve`; Solution thành `approved` và phương án đã chọn đánh dấu.
- [ ] `diagnosis`, `findings`, `answer` hiện đúng khung và nhãn nút ("Chấp nhận" cho `answer`).
- [ ] Từ chối với lý do rỗng hoặc dưới 10 ký tự: nút bị khoá, không có yêu cầu gửi đi.
- [ ] Từ chối hợp lệ gửi `comment` và Solution thành `rejected` kèm lý do hiển thị.
- [ ] `hotfix` chỉ xem, không có thanh quyết định.
- [ ] Người không có quyền duyệt không thấy nút; lỗi `forbidden` hiện toast và UI trở về chỉ đọc.
- [ ] `conflict` tải lại và giữ lựa chọn nếu phương án còn.
- [ ] Solution `superseded` thu gọn và không có hành động.
- [ ] `Mod+Enter` gửi hộp từ chối (`metaKey` Mac, `ctrlKey` nơi khác), nhãn chip khớp.
- [ ] Mọi chuỗi mới có 5 locale; không có màu hex.

## 5. Kiểm thử

- Unit: bộ chọn `SolutionBody` theo `kind`; hàm dựng `SolutionComparisonTable` (đánh dấu khác biệt, thiếu trường); validate lý do (cắt khoảng trắng, độ dài); ánh xạ `status` → banner và hành động.
- Component: `SolutionOptionCompare` (chọn, khoá khi dưới 2), `RejectReasonDialog` (bắt buộc lý do, phím tắt), `SolutionDecisionBar` theo quyền, `SolutionGenerationState`, từng `kind`.
- Hook: `useSolutions` (chuỗi chooseOption rồi approve, lỗi bước hai, `conflict`).
- E2E (cần CR-REQ-007/008/009): sinh Solution, chọn, duyệt, thấy Request sang `planning`; từ chối rồi sinh lại.
- Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Schema `SolutionOption` chưa định nghĩa trong README; bảng so sánh phụ thuộc các trường chung giữa phương án.
- Chuỗi hai lời gọi (chọn rồi duyệt) có thể để lại trạng thái nửa chừng nếu mạng đứt.
- Nội dung do AI sinh có thể rất dài hoặc chứa Markdown độc hại; cần dùng renderer đã khử HTML.
- Chưa rõ `Approval` gắn với `Solution` nào khi có nhiều bản (`superseded`): UI giả định cổng `pending` chỉ có cho bản `proposed` mới nhất.

## 7. Câu hỏi mở

1. `ChooseSolutionOption` và `Approve` có gộp được không (một RPC)? README liệt kê hai RPC riêng.
2. `GenerateSolution` có nhận `feedback` từ lần từ chối trước không? README không nêu.
3. Chẩn đoán của `bug`/`security`/`performance` dùng `subject_type=solution` (README 3.4 ghi cổng `solution`); xác nhận để UI tìm đúng Approval.
4. Có cần hạn duyệt (`due_at`) hiển thị đếm ngược không, và hết hạn (`expired`) thì UI làm gì? CR-REQ-010 phụ trách; UI mặc định chỉ hiện "Quá hạn".
5. Người báo cáo có được duyệt Solution của chính mình không (tách nhiệm vụ)?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (3.4, 3.5 `solutions`, `approvals`; O4)
- `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskAIDecompose.tsx` (mẫu đề xuất rồi áp dụng)
- `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/{table,sheet,dialog,textarea,skeleton,progress}.tsx`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/request/solution/{SolutionPanel,SolutionOptionCompare,SolutionComparisonTable,DiagnosisView,FindingsView,AnswerView,SolutionDecisionBar,RejectReasonDialog}.tsx`
