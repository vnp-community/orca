# Nguồn dữ liệu cần có để tạo Solution, Plan, Thực thi và Task tốt nhất, và kế hoạch kết nối qua MCP

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Danh mục nguồn dữ liệu và đề xuất kiến trúc kết nối (chưa phải CR, chưa có code) |
| **Mục đích** | Liệt kê mọi nguồn dữ liệu bổ sung hoặc hỗ trợ để AI tạo Solution, Plan, thực thi Plan và Task chính xác và đầy đủ; làm cơ sở để tích hợp và kết nối sau này, chủ yếu qua MCP |
| **Phương pháp** | Rà cấu trúc repo (`.github/`, `docs/adrs`, `docs/hld`, `backend-go/policy`, `backend-go/deploy`, `specs/`, `skills/`, `orca.yaml`) và các CR v5/v6. Các nguồn ngoài repo (Jira, GitHub, CI, quan sát, quét lỗ hổng...) là đề xuất dựa trên hiểu biết chung, **chưa kiểm tra khả năng kết nối thực tế**. |
| **Liên quan** | [artifact-formats-ontology-and-execution-readiness.md](./artifact-formats-ontology-and-execution-readiness.md), [impact-assessment-and-risk-scoring.md](./impact-assessment-and-risk-scoring.md), [openspec-and-ai-tooling-integration.md](./openspec-and-ai-tooling-integration.md), [CR v5 (MCP)](../../crs/v5/README.md), [CR v6](../../crs/v6/README.md) |

> Phần "Ghi chú hiện trạng" ở mỗi bảng chỉ nêu điều tôi thấy trong repo. Chưa kiểm chứng điều gì chạy được trên môi trường thật.

## 1. Nguyên tắc

1. **Chất lượng đầu ra bị giới hạn bởi ngữ cảnh đầu vào.** AI chỉ sinh được Solution đúng với kiến trúc và quy ước thật khi nó được cung cấp dữ liệu thật về chúng. Hiện bước `ai.complete` chỉ nhận một chuỗi prompt gồm nội dung Request, ngữ cảnh dự án và tech stack.
2. **Mỗi nguồn có hợp đồng chung:** loại dữ liệu, mức tin cậy, độ mới, quyền truy cập, giới hạn kích thước, cách trích dẫn. Nguồn nào cũng ghi nguồn gốc vào `Evidence` để người duyệt kiểm tra được.
3. **Ưu tiên nguồn xác định, đã có sẵn trong repo, rẻ để lấy** trước nguồn ngoài.
4. **Kết nối ngoài đi qua MCP**, qua registry và chính sách của `mcp-service` (đã có approval, consent, kill switch, audit), không để agent tự kết nối tuỳ ý.
5. **Nội dung từ nguồn ngoài mặc định là dữ liệu không tin cậy** (có thể chứa chỉ dẫn gài vào prompt).

## 2. Mô hình tích hợp: Source Registry và Context Pack

```
Nguồn nội bộ (repo, DB Orca, CodeGraph/GitNexus)          Nguồn ngoài (Jira, GitHub, CI, quan sát, ...)
        │  adapter nội bộ                                          │  máy chủ MCP bên ngoài
        └──────────────┬─────────────────────────────────────────┘
                       ▼
          Source Registry (request-service / mcp-service)
            id, loại, giao thức, quyền, mức tin cậy, độ mới (TTL), giới hạn, che dữ liệu
                       ▼
          Context Pack Builder  (theo bước: Solution | Plan | Execute | Task | Risk)
            chọn nguồn theo bước, xếp hạng, cắt theo ngân sách, gắn trích dẫn (Evidence)
                       ▼
          Prompt / ExecutionPacket  →  agent (ai.complete | agent.execPrompt)
```

**Hợp đồng của một nguồn (adapter):** `search(query)`, `get(ref)`, `list(filter)`, tuỳ chọn `subscribe(change)`; mỗi kết quả kèm `source_id`, `ref`, `retrieved_at`, `freshness`, `trust`, `digest`, `size`. Mục 6 mô tả Context Pack.

## 3. Danh mục nguồn dữ liệu

Ký hiệu bước: **S** = Solution, **P** = Plan, **E** = Thực thi, **T** = Task (TaskSpec), **R** = Đánh giá rủi ro. Ưu tiên: **1** = làm trước, **2** = tiếp theo, **3** = khi đã chạy thật.

### A. Hiểu yêu cầu (nội dung gốc của Request)

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Jira (issue, epic, link, comment, attachment, workflow, trường tuỳ biến) | Mô tả, tiêu chí chấp nhận, issue type, priority, label, người liên quan, lịch sử trao đổi, liên kết issue | S, P, T | Đầy đủ yêu cầu gốc, ngữ cảnh nghiệp vụ | Đã có adapter Jira ở `issue-tracking-service`; thêm MCP khi chuẩn hoá | Thấp (người nhập) | 1 |
| GitHub, GitLab (issue, PR, review, discussion) | Tương tự, kèm bàn luận kỹ thuật | S, P | Quyết định đã bàn trước đó | `scm-integration-service` hiện có; thêm MCP | Thấp | 1 |
| Linear | Issue, project, cycle | S, P | Như Jira | `task_sources` đã hỗ trợ `linear` | Thấp | 2 |
| Thông báo hệ thống đi vào (webhook, email, chat) | Bản gốc kèm bối cảnh | S | Thu thập yêu cầu từ kênh ngoài | MCP (CR-REQ-017 mô tả nguồn MCP) | Thấp | 2 |
| Tệp đính kèm, ảnh chụp, log, video | Bằng chứng tái hiện lỗi | S, P | Chẩn đoán `bug`, `performance` | Kho tệp của Orca (chưa thấy trong CR v6) | Thấp | 2 |

### B. Hiểu hệ thống (kiến trúc, quy ước, quyết định)

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Quy ước của dự án: `AGENTS.md`, `CLAUDE.md`, `guides/STYLEGUIDE.md`, `.oxlintrc.json`, `.oxfmtrc.json`, `config/max-lines-baseline.txt` | Quy tắc đặt tên, cấm `max-lines` disable, hỗ trợ SSH, đa nền tảng, tương thích Git và provider | S, P, T, E | Nội dung Plan và Task tuân quy ước thay vì bị từ chối ở bước review | Đọc trực tiếp từ repo (`fs.readFile`) | Cao | 1 |
| ADR và HLD: `docs/adrs/`, `docs/hld/` | Quyết định kiến trúc đã chốt và lý do | S, R | Tránh đề xuất đi ngược quyết định cũ | Đọc từ repo; lập chỉ mục tìm kiếm | Cao (nhưng có thể lỗi thời, đã gặp lệch docs và code) | 1 |
| Specs và CR: `specs/`, `docs/crs/`, `docs/logic/`, `docs/features/` | Yêu cầu và thiết kế đã có, nhật ký thay đổi | S, P | Trùng lặp, mâu thuẫn với CR đang có | Đọc từ repo | Trung bình (đã gặp lệch với code) | 1 |
| Danh mục service và sơ đồ triển khai (`backend-go/go.work`, `deploy/`, `orca.yaml`, `docker-compose`) | Có những service nào, cổng, phụ thuộc, thứ tự deploy | S, R | Đánh giá tác động liên service | Quét tệp cấu hình; xuất thành đồ thị | Cao | 1 |
| Từ điển thuật ngữ miền | Nghĩa của Request, Task, Plan, Worktree, Dev server... | S, P | Giảm nhầm khái niệm | Soạn mới (chưa có) | Cao | 2 |
| Skills và hướng dẫn agent (`skills/`, `.claude/`) | Quy trình chuẩn của dự án | E | Agent làm theo cách dự án quy định | Đọc từ repo | Cao | 2 |

### C. Hiểu mã nguồn

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| CodeGraph (`.codegraph/`), GitNexus (`.gitnexus/`) | Symbol, người gọi, luồng thực thi, vùng ảnh hưởng | S, P, R, E | Phạm vi ảnh hưởng, chẩn đoán đúng chỗ | CLI qua `agent.exec` hoặc máy chủ MCP của chúng | Cao khi index mới; **có thể lỗi thời** | 1 |
| Cây tệp, nội dung tệp, lịch sử git (blame, log, đồ thị thay đổi cùng nhau) | Ai sửa, sửa gì, file nào hay đổi cùng nhau | S, R | Tìm chủ sở hữu thực tế, vùng rủi ro | `fs.*`, `git.*` của agent | Cao | 1 |
| Hợp đồng: `backend-go/proto/orca/**`, bảng kênh WS (`wscompat`), tool MCP, schema sự kiện outbox | Cấu trúc API và sự kiện | S, R | Tương thích hợp đồng | Đọc từ repo; `buf breaking` | Cao | 1 |
| Schema CSDL: `migrations/postgres`, `migrations/mysql` | Bảng, ràng buộc, số migration tiếp theo | S, P, R | Chiều Dữ liệu, tương thích hai dialect | Đọc từ repo | Cao | 1 |
| Phụ thuộc bên thứ ba (`go.mod`, `package.json`, `pnpm-lock.yaml`) | Thư viện, phiên bản | S, R | Chọn công nghệ có sẵn, rủi ro nâng cấp | Đọc từ repo | Cao | 2 |
| Tài liệu thư viện đúng phiên bản (kiểu Context7) | API thư viện | S, P, E | Giảm bịa API | MCP ngoài | Trung bình | 2 |
| Mẫu code tốt trong repo (ví dụ mẫu `AIDecompose` → `AIApply`) | Ví dụ chuẩn cho từng kiểu thay đổi | P, T, E | Agent bắt chước mẫu đúng | Tập "ví dụ vàng" do người duy trì chọn | Cao | 2 |

### D. Chất lượng, kiểm thử, CI

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Quy trình CI: `.github/workflows/*.yml` (mỗi service một tệp, có `mcp-conformance`) | Lệnh build, lint, test, kiểm tra bắt buộc | T, E, R | Check của TaskSpec dùng đúng lệnh thật | Đọc từ repo | Cao | 1 |
| Kết quả CI gần đây, test không ổn định, thời gian chạy | Trạng thái nền, test hay lỗi | E, R | Biết nền có xanh không trước khi sửa | MCP tới nhà cung cấp CI hoặc `gh` | Cao | 2 |
| Độ phủ test theo symbol | Phần bị ảnh hưởng có test chưa | R, T | Chiều Chất lượng, chọn Check | Sinh từ báo cáo độ phủ | Trung bình | 2 |
| `Makefile`, script (`backend-go/ci/`, `scripts/`, `tests/`) | Lệnh kiểm tra có sẵn | T, E | Check dùng đúng lệnh | Đọc từ repo | Cao | 1 |
| Mẫu PR và Issue (`.github/pull_request_template.md`, `ISSUE_TEMPLATE/`) | Thông tin PR và issue phải có | T, E | Nội dung PR do agent tạo đúng mẫu | Đọc từ repo | Cao | 2 |

### E. Vận hành và quan sát (đánh giá tác động thực tế)

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Metric, log, trace (Prometheus, OpenTelemetry; `backend-go/deploy/alerts`) | Lưu lượng, độ trễ, lỗi theo service | S, R, E | Chiều Phạm vi và Vận hành: đo tải thật của đường bị sửa | MCP tới hệ thống quan sát | Cao | 2 |
| Sự cố và hậu kiểm (incident, postmortem) | Lỗi từng gặp, vùng dễ vỡ | S, R | Học từ lịch sử | MCP tới công cụ sự cố hoặc kho tài liệu | Cao | 2 |
| Cờ tính năng (feature flag) và cấu hình triển khai | Cờ hiện có, trạng thái theo môi trường | P, R | Kế hoạch rollout và quay lui | MCP hoặc đọc cấu hình | Cao | 2 |
| Lịch phát hành, cửa sổ triển khai, đóng băng | Khi nào được deploy | P, R | Phase và `pre_deploy` đúng thời điểm | MCP tới lịch | Trung bình | 3 |
| Trạng thái môi trường (dev, staging, prod), phiên bản đang chạy | Phiên bản triển khai | P, R | Biết thay đổi lên môi trường nào | MCP | Cao | 3 |

### F. Bảo mật và tuân thủ

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Chính sách OPA (`backend-go/policy/orca-authz`) | Luật phân quyền hiện có | S, R | Chiều Bảo mật: thay đổi có chạm luật không | Đọc từ repo | Cao | 1 |
| Quét lỗ hổng phụ thuộc (SCA), quét mã (SAST), quét bí mật | Danh sách lỗ hổng, bí mật lộ | S, R, E | Đầu vào cho `security`; kiểm sau thực thi | MCP tới công cụ quét hoặc CLI qua `agent.exec` | Cao | 2 |
| Cơ sở dữ liệu lỗ hổng công khai (CVE) | Chi tiết lỗ hổng | S | Chẩn đoán `security` | MCP | Cao | 3 |
| Chính sách tuân thủ, dữ liệu cá nhân, giấy phép | Quy định nội bộ | S, R | Ràng buộc phi chức năng | Soạn mới và đọc từ kho tài liệu | Cao | 3 |

### G. Con người và tổ chức

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Sở hữu mã (`CODEOWNERS`): **chưa có trong repo** | Ai chịu trách nhiệm phần nào | S, R | Gợi ý người duyệt đúng, giảm Approval nhầm người | Tạo mới, hoặc suy ra từ git blame | Cao | 1 |
| Người dùng, team, vai trò (`auth-service`, `tenant-service`) | Quyền duyệt theo vai trò | S, P | Chính sách duyệt (CR-REQ-010) | Dịch vụ nội bộ | Cao | 1 |
| Khả năng và tải của đội | Ai rảnh | P | Phân việc | MCP tới công cụ lập lịch | Thấp | 3 |
| Thành viên liên quan trong Jira, GitHub | Người báo cáo, người được giao | S | Hỏi đúng người (Clarification) | Như nhóm A | Trung bình | 2 |

### H. Lịch sử và học từ kết quả (nội bộ Orca)

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Request, Solution, Plan, Task trước đây và kết quả thật (xong, lỗi, quay lui, bị từ chối, lý do) | Các trường hợp tương tự đã xử lý | S, P, T, R | Gợi ý dựa trên tiền lệ, hiệu chỉnh điểm rủi ro | Dữ liệu của `request-service`, `task-service` | Cao | 1 |
| `ExecutionResult`, `Failure.class`, số lần thử | Task nào hay lỗi, vì sao | T, E | Cải thiện prompt và TaskSpec | Cùng nguồn | Cao | 1 |
| Bản ghi Decision, Clarification | Lựa chọn và câu hỏi đã có | S, P | Không hỏi lại điều đã hỏi | Cùng nguồn | Cao | 2 |
| Bộ ví dụ chuẩn (golden set) cho đánh giá prompt | Request mẫu và đầu ra mong đợi | S, P | Đo hồi quy khi đổi prompt | Soạn và duy trì | Cao | 2 |
| Chi phí và thời lượng AI theo bước | Token, thời gian, tỉ lệ lỗi JSON | Tất cả | Tối ưu và đặt ngân sách | `usage-service`, trace | Cao | 2 |

### I. Môi trường thực thi (dev server)

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Hồ sơ dev server: công cụ cài sẵn, phiên bản, HĐH, tài nguyên, đăng nhập `claude` | Có `go`, `node`, `openspec` không | E, T | Cổng sẵn sàng môi trường | `agent.exec` và `devServer.*`; thêm báo cáo năng lực (capability) | Cao | 1 |
| Trạng thái worktree, nhánh gốc, git (phiên bản, tương thích) | Sạch hay bẩn, nhánh | E | Điều kiện chạy | `git.*` của agent | Cao | 1 |
| Biến môi trường và bí mật (chỉ tên, không giá trị) | Biến cần có đã đặt chưa | E, T | `requires.env_names` | `agent.exec` | Cao | 1 |

### J. Tri thức bên ngoài

| Nguồn | Dữ liệu | Bước | Giá trị | Kết nối | Tin cậy | Ưu tiên |
|---|---|---|---|---|---|---|
| Wiki, Confluence, Notion, Google Drive | Tài liệu sản phẩm, quy trình | S, P | Bối cảnh nghiệp vụ | MCP | Trung bình | 2 |
| Chat, email (Slack, Teams) | Bàn luận, quyết định miệng | S | Ngữ cảnh | MCP (nhạy cảm, cần chính sách chặt) | Thấp | 3 |
| Tìm kiếm web, tài liệu công khai | Thực hành tốt, thông báo thư viện | S | Tham khảo | MCP hoặc công cụ tìm kiếm | Thấp | 3 |
| Tiêu chuẩn ngành, hợp đồng với khách hàng | Ràng buộc | S, R | Yêu cầu phi chức năng | Kho tài liệu | Cao | 3 |

## 4. Ma trận: bước nào cần nguồn nào nhất

Ký hiệu: ● bắt buộc, ○ nên có.

| Nhóm nguồn | Solution | Plan | Task | Thực thi | Rủi ro |
|---|---|---|---|---|---|
| A. Yêu cầu | ● | ● | ○ | | |
| B. Kiến trúc, quy ước | ● | ● | ● | ● | ● |
| C. Mã nguồn | ● | ● | ● | ● | ● |
| D. Chất lượng, CI | ○ | ○ | ● | ● | ● |
| E. Vận hành, quan sát | ○ | ○ | | ○ | ● |
| F. Bảo mật | ○ | | | ○ | ● |
| G. Con người | ○ | ○ | | | ○ |
| H. Lịch sử và kết quả | ● | ● | ● | ○ | ● |
| I. Môi trường thực thi | | | ● | ● | ○ |
| J. Tri thức ngoài | ○ | ○ | | | |

## 5. Kiến trúc kết nối qua MCP

### 5.1 Hai chiều

| Chiều | Mô tả | Hiện trạng trong repo |
|---|---|---|
| **Orca là MCP client** | Kết nối tới máy chủ MCP ngoài (Jira, GitHub, CI, quan sát, quét lỗ hổng, tài liệu) | `mcp-service` có domain `external_server`, use case `check_external_server_health`, `resolve_agent_mcp_config`; CR-MCP-014 (registry máy chủ ngoài) nằm ở v5. Mức hoàn thiện chưa kiểm chứng |
| **Orca là MCP server** | Cho agent và công cụ ngoài đọc và ghi Request, Solution, Plan, Task | Có tool `task.*`, `workflow.*`, `terminal.*`, `project.*`; `request_*`, `solution_*`, `approval_*` thuộc CR-REQ-017 (đề xuất) |

### 5.2 Đề xuất quy ước

| Hạng mục | Đề xuất |
|---|---|
| **Registry nguồn** | Mỗi nguồn là một bản ghi: `id`, `kind` (từ danh mục mục 3), `transport` (`internal` hoặc `mcp`), `server_ref`, `scopes`, `trust`, `ttl`, `max_bytes`, `redaction`, `enabled_for` (project, loại Request, bước) |
| **Tài nguyên (resource) dạng URI** | `orca://request/{id}`, `orca://solution/{id}`, `orca://plan/{id}`, `orca://task/{id}`, `orca://evidence/{id}`, `orca://impact/{assessment_id}`, `orca://context/{request_id}/{stage}` (Context Pack đã lắp) |
| **Tool** | Đọc: `source_search`, `source_get`, `source_list`. Ghi: chỉ qua tool có phê duyệt của `mcp-service` (mức rủi ro ghi) |
| **Đăng ký thay đổi** | `subscribe` cho nguồn có thể báo đổi (issue được sửa, CI đổi trạng thái, index mới) để làm mới Context Pack hoặc đánh dấu Solution là có thể lỗi thời |
| **Gắn nguồn gốc** | Mọi mảnh dữ liệu đưa vào prompt có `source_id`, `ref`, `retrieved_at`, `digest`; hiển thị cho người duyệt như `Evidence` |
| **Cấp quyền cho agent** | Agent do Orca chạy chỉ thấy các nguồn được cấp cho đúng project và bước; qua `resolve_agent_mcp_config` |
| **Kiểm toán** | Ghi lời gọi nguồn ngoài vào audit (đã có ở `mcp-service`) |

### 5.3 Nơi Context Pack được lắp

Đề xuất lắp ở `request-service` (trước khi gọi `ai.complete` hay `agent.execPrompt`), không để agent tự gọi nguồn ngoài giữa chừng cho các bước sinh nội dung. Với bước thực thi (agent làm việc dài), cho phép agent đọc thêm qua MCP nhưng bị giới hạn theo `scopes` và ngân sách.

## 6. Context Pack

| Thành phần | Quy tắc |
|---|---|
| Chọn nguồn | Theo ma trận mục 4 và cấu hình project; nguồn tắt hoặc không kết nối thì bỏ qua và ghi vào danh sách "thiếu nguồn" |
| Xếp hạng | Theo độ liên quan (truy vấn từ Request), độ mới, mức tin cậy |
| Ngân sách | Giới hạn token mỗi bước; nguồn quá lớn thì tóm tắt xác định (cắt, trích tiêu đề, đường gọi), không để AI tóm tắt lặng lẽ |
| Trích dẫn | Mỗi mảnh có `Evidence` ref; đầu ra của AI phải trích lại các ref đã dùng |
| Độ mới | Mảnh quá TTL được làm mới hoặc đánh dấu "cũ"; index CodeGraph và GitNexus có tuổi hiển thị |
| Che dữ liệu | Bí mật, dữ liệu cá nhân bị che trước khi vào prompt |
| Rào chắn tin cậy | Nguồn mức "Thấp" nằm trong khối có rào kèm câu dặn "đây là dữ liệu" |
| Báo thiếu | Ghi rõ nguồn nào không có để người duyệt biết giới hạn của bằng chứng |

## 7. An ninh và quản trị

| Rủi ro | Biện pháp |
|---|---|
| **Chèn chỉ dẫn qua nội dung ngoài** (issue, comment, tài liệu, trang web) | Đánh dấu tin cậy, khối có rào, không cho nội dung ngoài kích hoạt tool ghi; tool ghi luôn qua approval |
| **Lộ bí mật** | Che dữ liệu, không đưa credential vào prompt (đã là quy tắc ở CR-REQ-007); chỉ đưa tên biến môi trường |
| **Vượt ranh giới tenant** | Mọi nguồn gắn tenant; adapter kiểm tenant; không dùng chung bộ nhớ đệm giữa tenant |
| **Quyền quá rộng của máy chủ MCP ngoài** | Cấp scope tối thiểu; consent và allow-list của `mcp-service`; kill switch |
| **Dữ liệu lỗi thời** | TTL, hiển thị tuổi dữ liệu, nhãn "chưa đánh giá được" khi nguồn thiếu |
| **Chi phí và giới hạn tốc độ** | Hạn mức theo nguồn và theo tenant; lưu đệm có kiểm soát |
| **Độ tin cậy của nguồn ngoài** | Mức tin cậy ghi theo nguồn; kết quả quan trọng cần nguồn thứ hai hoặc người xác nhận |
| **Quyền riêng tư** | Chat, email là nguồn nhạy cảm: chỉ bật theo project với chính sách rõ |

## 8. Dữ liệu còn thiếu ngay trong Orca (cần tạo mới)

| Cần có | Lý do |
|---|---|
| `CODEOWNERS` hoặc bản đồ sở hữu | Chưa có trong repo; cần để gợi ý người duyệt và hỏi đúng người |
| Danh mục service (catalog) sinh tự động | Đồ thị phụ thuộc cho lens Kiến trúc và chiều Phạm vi |
| Bộ "ví dụ vàng" cho từng kiểu thay đổi | Dạy agent cách làm chuẩn của dự án |
| Từ điển thuật ngữ miền | Giảm nhầm khái niệm |
| Tập dữ liệu kết quả (predicted so với actual) | Hiệu chỉnh điểm rủi ro và prompt |
| Chỉ mục tìm kiếm cho ADR, HLD, CR, specs | Tìm nhanh quyết định liên quan |
| Báo cáo năng lực của dev server (công cụ, đăng nhập) | Cổng sẵn sàng môi trường |
| Hồ sơ nguồn (Source Registry) theo project | Bật tắt nguồn theo loại Request và bước |

## 9. Lộ trình đề xuất

| Đợt | Nguồn | Lý do |
|---|---|---|
| **1. Không phụ thuộc bên ngoài** | A (Jira, GitHub đang có), B (quy ước, ADR, HLD, CR), C (CodeGraph, GitNexus, git, proto, migration), D (CI, script), F (OPA), G (người dùng và vai trò), H (lịch sử Orca), I (hồ sơ dev server) | Phần lớn đã nằm trong repo hoặc dịch vụ nội bộ; chỉ cần adapter và Context Pack |
| **2. MCP ngoài thiết yếu** | Kết quả CI, quan sát (metric, log, sự cố), quét lỗ hổng, tài liệu thư viện, wiki | Làm giàu đánh giá tác động và chẩn đoán |
| **3. Mở rộng** | Chat, email, lịch phát hành, khả năng đội, CVE, tiêu chuẩn | Giá trị phụ, rủi ro riêng tư cao hơn |

Điều kiện sẵn sàng trước khi thêm một nguồn: có chủ sở hữu, có mức tin cậy, có TTL, có che dữ liệu, có giới hạn tốc độ, có cách tắt nhanh, có bản ghi kiểm toán.

## 10. Câu hỏi mở và chưa kiểm chứng

| Điểm | Trạng thái |
|---|---|
| Registry máy chủ MCP ngoài của `mcp-service` đã dùng được chưa (mức hoàn thiện của `external_server`, CR-MCP-014) | Chưa kiểm chứng |
| Agent do Orca chạy nhận được cấu hình MCP ngoài (`resolve_agent_mcp_config`) và `claude --print` có dùng được MCP server không | Chưa kiểm chứng |
| Máy chủ MCP của CodeGraph và GitNexus có chạy trên dev server và trả dữ liệu theo dạng dùng được bằng chương trình không | Chưa kiểm chứng |
| Hệ thống CI, quan sát, quét lỗ hổng đang dùng ở đâu và có máy chủ MCP tương ứng chưa | Cần bạn cho biết |
| Jira, GitHub, GitLab của bạn dùng bản nào (cloud hay tự lưu trữ) và có MCP server nào đang dùng | Cần bạn cho biết |
| Nên lắp Context Pack ở `request-service` hay một dịch vụ riêng | Đề xuất `request-service`; chưa chốt |
| Ngân sách ngữ cảnh mỗi bước và mức cắt | Chưa đo; cần thử với prompt thật |
| Quy tắc quyền riêng tư cho chat, email | Cần chính sách của tổ chức |
| Mức tin cậy mặc định từng nguồn | Đề xuất ở bảng, chưa đối chiếu với chính sách của bạn |

## 11. Việc đề xuất làm tiếp

| Việc | Ghi chú |
|---|---|
| **CR-REQ-031:** Source Registry và Context Pack Builder | Phụ thuộc CR-REQ-027, 017; mô tả hợp đồng nguồn, bảng registry, bộ lắp ngữ cảnh |
| **CR nhỏ:** Hồ sơ năng lực dev server | Báo cáo công cụ cài sẵn, đăng nhập; dùng cho cổng sẵn sàng |
| **Tạo dữ liệu còn thiếu** (mục 8) | `CODEOWNERS`, danh mục service, ví dụ vàng, từ điển thuật ngữ |
| **Thử nghiệm nhỏ:** nối CodeGraph hoặc GitNexus làm nguồn thử đầu tiên | Kiểm tra đường MCP và định dạng đầu ra trước khi mở rộng |
