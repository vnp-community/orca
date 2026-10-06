# Tích hợp OpenSpec và các công cụ AI hỗ trợ vào Request flow

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Khuyến nghị thiết kế (chưa phải CR, chưa có code) |
| **Phương pháp** | Đọc code agent và infra-fleet (`agent/src/relay/`, `backend-go/services/infra-fleet-service`). **Chưa cài hay chạy thử công cụ nào.** Mô tả OpenSpec và các công cụ khác dựa trên hiểu biết chung của người soạn, cần kiểm chứng với phiên bản định dùng. |
| **Phát hiện trong repo** | Không có thư mục `openspec/`, không có `.claude/commands/`, không có lệnh `openspec` trên máy đã khảo sát, không có tham chiếu OpenSpec nào trong mã nguồn hay docs |
| **Liên quan** | [ai-steps-and-dev-server-connection-flows.md](./ai-steps-and-dev-server-connection-flows.md), [solution-plan-execution-walkthrough.md](./solution-plan-execution-walkthrough.md), [CR v6](../../crs/v6/README.md) |

## 1. Có dùng được OpenSpec để hỗ trợ sinh Solution và Task không?

**Được.** OpenSpec lưu mỗi thay đổi thành một thư mục trong repo (`openspec/changes/<id>/`) gồm `proposal.md`, `design.md` (tuỳ chọn), `tasks.md` và các delta của spec; trình tự là propose, validate, apply, archive. Trình tự này khớp với các cổng duyệt của Request flow.

### 1.1 Ánh xạ

| Bước Orca | Thành phần OpenSpec | Ghi chú |
|---|---|---|
| Request đã xác nhận loại | Một change mới `openspec/changes/<request-number>-<slug>/` | `request_id` ghi trong `proposal.md` |
| Solution (nhiều phương án) | `proposal.md` (vì sao, thay đổi gì) + `design.md` (các phương án, đánh đổi, quyết định) | Phương án được chọn ghi ở phần Decision |
| Yêu cầu hành vi | Delta spec trong `specs/` | Phù hợp `change_request`, `refactor` |
| Plan, Phase | `tasks.md`, mỗi Phase là một nhóm `##` | Parse thành `PlanProposal` |
| Task | Từng dòng checkbox trong `tasks.md` | Sinh task thật qua `CreatePlanTree` |
| Cổng duyệt | `openspec validate` chạy trước, rồi Approval của Orca | Validate chỉ kiểm cấu trúc, không thay người duyệt |
| Thực thi | Tương đương `apply`: agent làm từng task và tick checkbox | Orca vẫn là bên điều phối |
| Hoàn tất | `archive`: gộp delta vào `openspec/specs/` | Kết quả nằm trong repo, không chỉ trong DB Orca |

### 1.2 Phù hợp theo loại Request

| Loại | Dùng OpenSpec |
|---|---|
| `change_request`, `refactor` | Phù hợp nhất (delta spec, design, tasks) |
| `bug`, `security`, `performance` | Dùng nhẹ: `proposal.md` + `tasks.md`, không cần delta spec |
| `task`, `docs`, `ops_request` | Không cần |
| `spike`, `question`, `hotfix` | Không dùng: quá nặng so với giá trị |

### 1.3 Tích hợp với agent và dev server

OpenSpec nằm trong repo nên phải chạy trên dev server nơi có repo. Dùng lại các đường đã có, không thêm method mới:

```
request-service ─Relay→ infra-fleet-service ─→ Dev Server Agent
   1. agent.exec        : `openspec validate <id>` / `openspec list` (thu stdout, exit code)
   2. agent.execPrompt  : claude --print trong worktree, prompt theo hướng dẫn OpenSpec
                          → tạo proposal.md, design.md, tasks.md (bước Solution và Plan)
   3. fs.readFile       : request-service đọc file về, parse, lưu vào solutions / PlanProposal
   4. agent.execPrompt  : thực thi từng task (apply), tick checkbox
   5. agent.exec        : `openspec archive <id>` khi Request completed
```

Các RPC `fs.readFile`, `fs.glob`, `fs.writeFile`, `fs.readDir` của agent đã tồn tại (`agent/src/relay/agent-rpc-dispatch-fs.ts`).

**Điểm có lợi:**
- Bước Solution và Plan dùng `agent.execPrompt` thay vì `ai.complete`. Agent đọc được repo và spec hiện có, nên chất lượng tốt hơn chỉ có ngữ cảnh văn bản. Đây cũng là cách vượt giới hạn "`ai.complete` không đọc được repo" của CR-REQ-007.
- Tài liệu giải pháp nằm trong git, có lịch sử và review được bằng PR.
- `openspec validate` là kiểm tra cấu trúc rẻ, thay một phần `ValidateProposal` của CR-REQ-012.

### 1.4 Ràng buộc (từ code agent)

| Ràng buộc | Hệ quả |
|---|---|
| `agent.execPrompt` chỉ chạy `claude` và bắt buộc có `worktreePath` | Sinh Solution bằng OpenSpec cần một worktree, mất lợi thế "không cần worktree" của `ai.complete`. Phải chọn: worktree tạm theo Request, hoặc `repo_path` gốc (chưa kiểm chứng việc này an toàn) |
| Không ép được chỉ đọc | Agent ghi `openspec/changes/...` nên sinh giải pháp **làm thay đổi repo**. Phải tách khỏi nhánh làm việc (ví dụ nhánh `request/<n>-proposal`) để không lẫn vào code của task |
| Slash command `/openspec:*` chưa chắc dùng được với `claude --print` | Chưa kiểm chứng. Cách an toàn: đặt chỉ dẫn OpenSpec vào chính prompt (đọc `openspec/AGENTS.md` rồi tạo file), và dùng CLI `openspec` qua `agent.exec` cho validate, archive |
| Dev server phải có CLI `openspec` và đã đăng nhập `claude` | Cần thêm vào kiểm tra điều kiện của dev server |
| Parse markdown tự do dễ vỡ | Ép định dạng bằng prompt, chạy `openspec validate`, parser chặt; sai thì thử lại một lần như CR-007 |
| Hai nguồn sự thật (task trong Orca và checkbox trong `tasks.md`) | Orca là nguồn chính cho trạng thái; `tasks.md` chỉ được cập nhật một chiều khi task xong |

## 2. Có nên đổi luồng hiện tại hoặc mở rộng agent để khớp OpenSpec không?

**Khuyến nghị: giữ luồng Request, giữ agent ở dạng chung, chỉ thêm vài khả năng nhỏ vào agent.**

### 2.1 Không đổi luồng Request

- Luồng Request đã có đủ cổng duyệt (Solution, Plan, Phase) và máy trạng thái. Trình tự của OpenSpec khớp với nó, không cần bẻ lại luồng.
- Coi OpenSpec là **một cách sinh nội dung** nằm sau một giao diện, ví dụ `SolutionEngine` với hai cài đặt: `native` (`ai.complete`, đã thiết kế ở CR-REQ-007 và 012) và `openspec`. Máy trạng thái, Approval, `solutions` và `CreatePlanTree` giữ nguyên.
- Chỉ thêm một bước: khi Request `completed`, chạy `openspec archive` nếu dùng OpenSpec. Đây là tác dụng phụ sau hoàn tất, không đổi trạng thái.
- Nếu đổi luồng theo OpenSpec, toàn bộ Request flow (11 loại, cổng, backlog) bị gắn vào một công cụ ngoài mà `hotfix`, `spike`, `question` không dùng. Phiên bản OpenSpec đổi thì luồng vỡ theo.

### 2.2 Không làm agent hiểu OpenSpec

Agent hiện là lớp chuyển tiếp chung (`agent.exec`, `agent.execPrompt`, `fs.*`) và `Relay` không dịch method. Mọi logic OpenSpec (dựng prompt, đọc file, parse `tasks.md`, validate) có thể nằm ở `request-service`. Thêm method `openspec.*` vào agent sẽ buộc triển khai lại agent trên mọi dev server và gắn agent với một công cụ cụ thể.

### 2.3 Thay đổi nhỏ đáng làm ở agent

Đều là thay đổi thêm vào, không phá hành vi cũ. Chúng hữu ích cho CR-REQ-008 và CR-REQ-013 dù có dùng OpenSpec hay không.

| Thay đổi | Lý do | Ưu tiên |
|---|---|---|
| Tham số chế độ chỉ đọc cho `agent.execPrompt` (hạn chế công cụ ghi) | Hiện không ép được chỉ đọc. Cần kiểm tra claude CLI có cờ hạn chế công cụ hay chế độ chỉ lập kế hoạch không (chưa kiểm chứng tên cờ) | Cao |
| Cho `agent.execPrompt` chạy khi `worktreePath` là thư mục tạm hoặc repo gốc, và nêu rõ việc đó | Hiện bắt buộc `worktreePath`, làm `spike`, `question` và sinh Solution không có chỗ chạy sạch | Cao |
| Trả thêm danh sách file đã đổi (hoặc dựa vào `git status` trước và sau) | `request-service` cần biết agent đã tạo những file nào trong `openspec/changes/...` | Trung bình |
| Kiểm tra điều kiện dev server: có CLI `openspec`, đã đăng nhập `claude` | Báo lỗi sớm thay vì lỗi giữa luồng. Làm được bằng `agent.exec`, không cần method mới | Trung bình |
| `agent.execPrompt` hỗ trợ model ngoài `claude`, hoặc báo lỗi rõ hơn | Chỉ claude được kiểm chứng; không bắt buộc cho OpenSpec | Thấp |

Chưa cần: method `openspec.*`, đổi giao thức khung truyền, đổi cơ chế kết nối.

### 2.4 Cần kiểm tra trước khi sửa agent

Kết quả tìm kiếm cho thấy một số file (ví dụ `agent-hook-server.ts`) có ở cả `agent/src/relay/` và `desktop/src/relay/`. Chưa xác nhận hai bản có được đồng bộ tự động hay phải sửa cả hai. Nếu sửa lệch, dev server và bản desktop sẽ chạy hai hành vi khác nhau.

## 3. Công cụ hỗ trợ khác ngoài OpenSpec

### 3.1 Nguyên tắc chọn

Mỗi công cụ mới là một phụ thuộc phải có trên **mọi dev server** (đường chạy AI duy nhất là agent trên dev server). Ưu tiên:
1. Công cụ không cần cài thêm, hoặc đã có trong repo.
2. Công cụ chạy được bằng `agent.exec` (đã có, không đổi agent).
3. Tích hợp qua MCP hoặc adapter ở `request-service`, không đưa logic vào agent.

### 3.2 Đề xuất theo bước của luồng

| Bước | Công cụ | Ưu điểm khai thác | Cách tích hợp | Ưu tiên |
|---|---|---|---|---|
| Chẩn đoán, Solution (hiểu code) | **CodeGraph, GitNexus** (repo này đã dùng, có index sẵn) | Tìm symbol, đường gọi, phạm vi ảnh hưởng mà không đọc cả file; hợp với "phạm vi ảnh hưởng" của `security`, `bug` và `affected_areas` của Solution | CLI qua `agent.exec`, hoặc nạp làm MCP server cho agent. Cần có index trên dev server | Cao |
| Solution, Plan | **OpenSpec** | Cấu trúc proposal, design, tasks nằm trong git | Adapter sau giao diện `SolutionEngine` | Cao |
| Solution, Plan (quy trình có sẵn) | **GitHub Spec Kit** (spec → plan → tasks) | Tương tự OpenSpec | Trùng vai trò với OpenSpec: chọn **một** trong hai | Chọn một |
| Plan (tách task từ tài liệu yêu cầu) | **Taskmaster AI** | Parse tài liệu yêu cầu thành task có phụ thuộc, tương tự `AIDecompose` | Chỉ mượn ý tưởng prompt và schema; hai nơi quản lý task sẽ lệch và trùng `task-service` | Thấp |
| Lấy tài liệu thư viện | **Context7** (MCP trả tài liệu đúng phiên bản) | Giảm bịa API khi agent lập Plan và viết code | MCP server cho agent; `mcp-service` có registry và chính sách cho MCP ngoài (CR-MCP-014, v5) | Trung bình |
| Thực thi | **Subagent, skill, hook của Claude Code** | Hook chặn lệnh nguy hiểm hoặc ép chạy test; skill đóng gói quy trình theo loại Request; subagent tách vai viết và review | Cấu hình trong repo của project (`.claude/`), `claude --print` đọc được. Không đổi agent. Chưa kiểm chứng hook hoạt động với `--print` | Cao |
| Kiểm tra sau thực thi | **Linter, kiểm tra kiểu, test của project; Semgrep hoặc ast-grep** cho `security` | Biến "task xong" thành điều kiện kiểm chứng được thay vì tin lời agent | Task nhãn `check:*` chạy bằng `agent.exec`, exit code quyết định (CR-REQ-014) | Cao |
| Kiểm tra giao diện | **Playwright** (Orca đã có phần e2e) | Xác nhận thay đổi frontend chạy thật | Như dòng trên, chỉ cho Request có phần UI | Trung bình |
| Đánh giá chất lượng AI | **Promptfoo** hoặc bộ so sánh đầu ra tự viết | Đo chất lượng prompt phân loại, Solution, Plan bằng bộ mẫu cố định, tránh hồi quy khi đổi prompt | Chạy trong CI, không nằm trên đường chạy thật | Trung bình |
| Quan sát chi phí và lỗi AI | **OpenTelemetry** (repo có `common/tracing`), **Langfuse** nếu muốn xem prompt | Theo dõi độ trễ, tỉ lệ JSON hỏng, số lần thử, chi phí theo loại Request | Ghi vào trace và metric sẵn có (CR-REQ-024). Agent hiện chỉ ghi độ dài prompt, giữ nguyên | Trung bình |

### 3.3 Nên tránh

- **Công cụ quản lý task riêng** làm nguồn sự thật thứ hai cho task: trùng `task-service`, sinh lệch trạng thái.
- **Framework nhiều vai trò** (như BMAD): quá nặng, buộc quy trình của họ lên quy trình của Orca, khó khớp 11 loại Request.
- **IDE hoặc công cụ chỉ có giao diện** (như Kiro): không chạy được trên dev server qua agent.
- **Cài hàng loạt trên mọi dev server** trước khi chứng minh giá trị.

### 3.4 Khái niệm chung `AnalysisTool`

Thiết kế một khái niệm ở `request-service`: **`AnalysisTool`**, với đầu vào là Request và ngữ cảnh, đầu ra là bằng chứng có cấu trúc đưa vào Solution (danh sách file ảnh hưởng, kết quả quét, tài liệu tham chiếu). Mỗi công cụ là một cài đặt, bật theo project. Cách này khai thác được nhiều công cụ mà không đổi máy trạng thái hay agent, và tắt được công cụ không đáng giá.

## 4. Thứ tự đề xuất

1. **Không cần cài thêm:** CodeGraph và GitNexus (đã có), hook và skill của Claude Code, kiểm tra bằng linter và test của project. Khai thác nhiều nhất tài sản sẵn có, hợp trực tiếp CR-REQ-008 (chẩn đoán) và CR-REQ-014 (kiểm tra theo loại).
2. **Thêm một công cụ có cấu trúc cho Solution và Plan:** OpenSpec hoặc Spec Kit, chọn một, sau giao diện `SolutionEngine`.
3. **Khi đã chạy thật:** Context7 cho tài liệu thư viện, Promptfoo cho đánh giá, Semgrep cho `security`.

Với OpenSpec: chạy thử **không đổi agent** trên một project mẫu (dùng `agent.execPrompt` và `agent.exec` hiện có) để xem agent có tạo đúng cấu trúc và `openspec validate` có qua không. Chỉ sau đó mới quyết định thêm chế độ chỉ đọc và làm rõ `worktreePath` cho agent.

## 5. Việc đề xuất làm tiếp

| Việc | Ghi chú |
|---|---|
| CR-REQ-026: OpenSpec solution engine | Phụ thuộc CR-007, 008, 012, 013. Cờ cấp project `solution_engine = openspec | native` (mặc định `native`) |
| CR nhỏ cho agent | Hai thay đổi ưu tiên cao ở 2.3 (chế độ chỉ đọc, làm rõ `worktreePath`) |
| CR cho `AnalysisTool` và hook/skill theo loại Request | Theo 3.4 |
| Cập nhật README v6 | Thêm các CR mới vào bảng và đợt thực thi |

## 6. Chưa kiểm chứng và cần bạn xác nhận

| Điểm | Trạng thái |
|---|---|
| Bản OpenSpec định dùng và cách cài trên dev server | Cần xác nhận |
| Bật theo project hay theo loại Request | Cần chọn |
| Dev server hiện cài sẵn những công cụ nào | Cần biết để chọn điểm bắt đầu |
| `claude --print` có chạy được slash command và hook không | Chưa kiểm chứng |
| Cờ của claude CLI để hạn chế công cụ ghi hoặc chạy chế độ chỉ lập kế hoạch | Chưa kiểm chứng tên cờ |
| Việc chạy agent trên repo gốc có an toàn không | Chưa kiểm chứng |
| Mô tả các công cụ ngoài (OpenSpec, Spec Kit, Taskmaster, Context7, Promptfoo, Semgrep, BMAD, Kiro) | Hiểu biết chung, chưa đối chiếu tài liệu hiện hành |
| `agent/` và `desktop/src/relay/` có đồng bộ tự động không | Chưa kiểm tra |
