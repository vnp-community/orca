# Bảng ánh xạ: 74 test Playwright mock `request-web` → e2e thật trên repo `deepseek-harness`

**Solution:** [SOL-FE-E2E-006](./solutions/SOL-FE-E2E-006-real-e2e-on-user-repo-replaces-mocked-request-web.md) · **Tasks:** [028](./tasks/TASK-FE-E2E-028-real-e2e-on-deepseek-harness.md), [029](./tasks/TASK-FE-E2E-029-retire-mocked-request-web-and-coverage-matrix.md)

Bộ mock (15 file, 74 test) đã gỡ. Cột "Thật": **R** = có spec thật đã chạy xanh phụ trách; **R\*** = phủ có điều kiện (phụ thuộc AI có đưa ra tình huống đó; nếu không, spec ghi `not-exercised`); **một phần** = spec thật phủ một phần ý của ca; **—** = KHÔNG còn trong e2e (cần tiêm lỗi vào hạ tầng, hoặc điều kiện stack không có, hoặc chưa làm: lý do ở cột cuối). Ca **—** không được tính là đã phủ.

Viết tắt: các spec nằm ở `tests/stages/e2e/orca-flows/ui-live/` (`deepseek-*-real.web.e2e.ts`, và các spec `--real` cũ `add-project-own-project-request-real`, `agent-session-monitoring-real`).


## add-project-host-binding (6)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: một host kết nối được chọn sẵn, `project.create` mang `devServerId` | một phần | UI: `project.create` mang tên/host/đường dẫn và không có `repo.add` (kiểm qua tap WebSocket); việc host được CHỌN SẴN (không phải bấm chọn) chưa được kiểm |
| b: hai host kết nối, không chọn sẵn | — | stack chỉ có một dev server kết nối; dựng agent thứ hai thật để lại bản ghi dev server mồ côi (không có API xoá) |
| c: không host kết nối, bị chặn, có đường tới Dev Servers | — | phải tắt agent duy nhất giữa lúc chạy; làm được nhưng gián đoạn mọi thứ khác; chưa làm |
| d: host rớt giữa lúc duyệt thư mục | — | tiêm lỗi (ngắt agent đúng lúc duyệt) |
| e: clone đăng ký repo đúng host | — | chưa làm: làm thật được (Clone từ đường dẫn cục bộ vào thư mục tạm); ghi backlog |
| f: create trên host cùng `devServerId` | — | chưa làm: như trên (Create project trong thư mục tạm); ghi backlog |

## add-project-own-project (6)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: tên mặc định = tên thư mục, sửa được | R | mọi spec `deepseek-*` (bước Add Project của harness: kiểm mặc định rồi đổi tên) |
| b: thư mục thứ hai có project riêng | R | `add-project-own-project-request-real` (alpha, bravo) |
| c: cùng host+đường dẫn bị từ chối, nêu project đang chứa | R | `deepseek-ui`, `add-project-own-project-request-real` |
| d: "Mở project đó" không thêm gì | R | `deepseek-ui`, `add-project-own-project-request-real` |
| f: có `/` cuối vẫn là cùng thư mục | R | `deepseek-ui`, `add-project-own-project-request-real` |
| e: trùng phía backend (tab khác thắng) | R | `add-project-own-project-request-real`: `project.create` trùng bị `PROJECT_REPO_ALREADY_REGISTERED`, nêu project đang giữ |

## dev-server-busy-state (3)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: rảnh, bận khi chạy, rảnh lại, biến mất khi agent ngắt | một phần | `deepseek-task` và `agent-session-monitoring-real`: thẻ dev server rảnh → bận khi agent chạy → rảnh (có số liệu); "biến mất khi ngắt" cần ngắt agent: chưa làm |
| b: luồng bị server đóng được mở lại | — | tiêm lỗi |
| c: gateway cũ không có `busy` | — | tiêm lỗi |

## repo-host-rebind (3)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| g: repo không host bị gắn cờ, "Choose dev server" gắn lại | — | backend từ chối tạo repo không host (`ORCA_ALLOW_LOCAL_REPOS` tắt): không có repo như vậy để thao tác thật |
| h: gắn lại bị từ chối được giải thích | — | như trên |
| i: repo không host riêng hiện host của project, "kế thừa" | một phần | thay bằng `deepseek-ui`: nhãn host của repo = `dev-local-mac`, KHÔNG kế thừa, không có `repo-host-unbound` |

## request-agent-live-activity (6)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: hiện việc agent đang làm, rồi "Finished" | R | `deepseek-task`: panel running có dòng, sau đó finished (14 dòng; loại dòng info/text/tool/result/end) |
| b: Request task không bao giờ hiện "no orchestration dispatch" | một phần | chưa có khẳng định riêng (`task-dispatch-none`); sheet được đọc nhiều lần và không bị judge bắt |
| c: mở sheet giữa lúc chạy, mở lại phát lại | R | `deepseek-task`: `judgeHistoryReplay` sau tải lại |
| d: luồng bị server đóng thì panel kết nối lại | — | tiêm lỗi |
| e: thông tin xác thực/markup trong output được che | một phần | quét bí mật trên mọi màn hình và khung hoạt động (sạch); che một chuỗi cắm sẵn thì không làm (không cắm thứ giống khoá thật vào repo của người dùng) |
| f: chạy dài giữ cửa sổ có hạn dòng | — | cần output hàng trăm dòng; không ép Claude thật in nhiều |

## request-answer-and-project-note (2)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: ô Answer hiện `answer_markdown`, độ tin cậy, trích dẫn | R | `deepseek-readonly` (đối chiếu `package.json` thật), `add-project-own-project-request-real` |
| b: Create request nêu repo nó chạy | một phần | mọi spec `deepseek-*` kiểm ghi chú `repo <tên> (<đường dẫn>) on dev-local-mac`; "đầu tiên trong nhiều repo" không làm (project một repo) |

## request-clarification-risk (3)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| clarification: chip "Awaiting information", một lần gửi mọi câu trả lời | R\* | mọi spec có Request, khi AI hỏi (`answerClarificationIfAsked`) |
| risk gate khi duyệt plan | R\* | không có phát hiện rủi ro cao ở Plan trong các lượt; hộp thoại rủi ro CAO của quyết định (gõ lại tên phương án) được xử lý có điều kiện ở `deepseek-review` |
| panel kết quả thực thi: khai báo của tác nhân cạnh Orca chạy lại | — | dành cho task contract/orchestration; task agent trực tiếp không có phần "Orca re-run" |

## request-execution-output (4)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: output cuối của task hiện là "Agent output" trung tính | R | `deepseek-execute`, `deepseek-task` (tab Result có `execution-agent-output` không rỗng) |
| b: lượt contract giữ khung kết quả có cấu trúc | — | cần task contract |
| c: không có tab Result khi chưa có gì để hiện | R | `deepseek-task`: tab của sheet trước khi chạy là [Details, Subtasks, AI Agent, Comments, Access], sau khi chạy có thêm Result |
| d: gateway không có `execution.get` | — | tiêm lỗi |

## request-graph (3)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: "View graph" mở được, lens Flow hiện | R | `deepseek-execute` (`expectRequestGraphSheet`: Flow chọn sẵn, canvas, List) |
| b: `impact.graph` 120/300 nút, phím `/`, Enter | — | cần chỉ mục codegraph của repo trong backend; stack local chưa có |
| c: Plan tab Tree/Graph | R | `deepseek-execute` (`expectTreeGraphToggle`) |

## request-list-detail (6)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: mở trang Requests, chi tiết đã mở | R | `deepseek-readonly` (chọn dòng của Request vừa tạo) |
| b: danh sách, chi tiết, xác nhận loại gọi `request.confirmType` | R | mọi spec (thẻ xác nhận loại) |
| c: runtime không có kênh request thì ẩn mục sidebar | — | tiêm lỗi |
| d: j/k đổi dòng chọn | R | `deepseek-readonly` (`expectRequestsShell`, hai Request) |
| shell: đếm chờ duyệt, tab, Escape đóng | R | `deepseek-readonly` |
| create: Request thủ công không gửi `source`, mở Request mới | R | mọi spec `deepseek-*` (`request.create` kiểm qua tap WebSocket) |

## request-plan-tree (5)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: Plan > Phase theo thứ tự, tiến độ | R | `deepseek-execute` (`expectPlanTabShape` đối chiếu `task.list`; tên Phase là nút tiêu đề, không phải "Phase 1: ...") |
| b (skipped): Board ẩn Plan/Phase | — | đã skipped từ bản mock; có vitest `TaskGraph-planning-toggle` |
| c: duyệt plan; lý do reject ngắn bị khoá | R | `deepseek-execute` |
| d: Phase chưa duyệt khoá Start; duyệt rồi Start | R | `deepseek-execute` và các spec `--real` cũ (driver kiểm Start bị khoá trước khi duyệt); đã lộ lỗi backend thứ tự cổng Phase (đã sửa) |
| e: runtime thiếu kênh task, có thông báo | — | tiêm lỗi |

## request-solution-open-questions (4)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a, b, c: câu hỏi chặn / không chặn, từ chối duyệt giải thích | R\* | `approveAnalysis` xử lý khi AI để lại câu hỏi chặn; không ép được AI làm vậy |
| d: "Ask back" gửi `clarification.request` | R\* | như trên |
| e: bị từ chối khi hỏi ngược giữ hộp thoại | — | tiêm lỗi |
| f: sau Clarification, màn hình theo Solution MỚI không cần tải lại | một phần | `deepseek-review`: sau Regenerate, Solution mới hiện (qua Reopen); chưa phải đúng đường "sau Clarification" |

## request-solution-review (5)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: hai phương án, thẻ và bảng so sánh | R\* | `deepseek-review` yêu cầu đúng hai phương án; ghi `not-exercised` nếu AI chỉ đưa một (các lượt đã chạy: 2 phương án, bảng so sánh hiện) |
| b: chọn rồi duyệt: `solution.choose` trước `approval.approve` | R | `deepseek-review` (kết thúc `accepted`, hoặc `refused-then-recommended`, hoặc `high-risk-confirmed`) |
| c: reject lý do ngắn bị khoá, hợp lệ gửi `approval.reject` | R | `deepseek-review` |
| d: Mod+Enter gửi hộp thoại reject | R | `deepseek-review` (Enter thường không gửi) |
| e: hotfix không có thanh quyết định | R | `deepseek-hotfix` (kèm giới hạn stack một người dùng, xem lỗi #3) |

## request-solution-start-and-ai-setup (7)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: analyzing chưa có run thì có nút; một lần bấm một run | R | mọi spec (tự khởi động, hoặc bấm "Generate solution" nếu không; chỉ một run) |
| b, c: không có nút khi đang chạy; không nhân đôi | một phần | chỉ một run được ghi trong `solution.list`; chưa có khẳng định về nút |
| d: khởi động lỗi hiện mã lỗi | — | tiêm lỗi |
| e: Solution lỗi vì thiếu AI nêu cách sửa | — | AI đã cấu hình trên stack thật |
| f: phân loại lỗi vì thiếu AI hiện banner | — | như trên |
| g: runtime thiếu `clarification.list` | — | tiêm lỗi |
| h: tab nêu đúng tên Request và loại node của Plan | R | `deepseek-execute` (Plan tab theo `task.list` thật) |

## request-task-sheet (8)

| Ca mock | Thật | Spec và ghi chú |
|---|---|---|
| a: "Open in Git" sang Source Control của worktree task | R | `deepseek-task`: danh sách có `clamp.mjs` và `clamp.test.mjs` của worktree task |
| a2: worktree sidebar chưa tải thì được tra rồi tải | R | `deepseek-task`: sau tải lại, sidebar chưa có worktree task vẫn mở được |
| b: worktree không còn trên đĩa thì nói rõ | R | `deepseek-task`: gỡ worktree thật, tải lại, Open in Git giải thích (không tải lại thì sidebar giữ bản cũ và chỉ hiện Source Control rỗng + lỗi console: xem lỗi #10) |
| c: id worktree host không biết thì nói rõ | — | cần id không tồn tại; không thao tác thật được qua UI |
| d: Escape chỉ đóng sheet, Request vẫn chọn | R | `deepseek-execute`, `deepseek-task` |
| f: sau chạy, panel rảnh trỏ sang tab Result | R | `deepseek-execute`, `deepseek-task` |
| g: chưa có output thì panel rảnh không có con trỏ | một phần | chỉ ghi nhận tab trước khi chạy không có Result; chưa khẳng định con trỏ |
| e: sheet vừa cửa sổ 1440/700/390 px, không tràn | R | `deepseek-execute`, `deepseek-task` |

## Tổng kết

Trong 71 dòng (74 test mock; một số dòng gộp 2 đến 3 ca cùng bản chất; ca `request-plan-tree b` đã skipped từ trước): **R = 33**, **R\* = 5**, **một phần = 9**, **— = 24**. Ca **—** là khoảng trống có chủ đích, không phải đã phủ: phần lớn chỉ tồn tại nhờ tiêm lỗi (gateway cũ thiếu kênh, luồng bị đóng, host rớt giữa lúc duyệt) và nên được giữ ở cấp component (vitest), không phải e2e. Hai ca làm thật được nhưng chưa làm (clone/create trong thư mục tạm) nằm ở backlog của TASK-FE-E2E-028.
