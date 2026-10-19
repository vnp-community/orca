# CR-REQ-039 — Chỉ định nhánh gốc để tạo worktree cho Request (mặc định vẫn là nhánh mặc định của repo)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-039 |
| **Tên** | Trường `base_ref` trên Request: người dùng chọn nhánh (hoặc tag) mà worktree của Request rẽ ra; để trống thì dùng nhánh mặc định của repo. Áp cho worktree đề xuất OpenSpec và worktree thực thi task |
| **Loại** | Feature (kèm sửa một lệch hành vi, xem mục 1.3) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-19 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge |
| **Phụ thuộc** | CR-REQ-001 và 002 (bảng và proto `Request`), CR-REQ-013 (thực thi Phase và task), CR-REQ-016 (kênh gateway), CR-REQ-026 (worktree đề xuất của engine OpenSpec), CR-REQ-038 (cùng chỗ ghim `engine`, `execution_mode`) |
| **Mở khoá** | [CR-REQ-040](../request-frontend/CR-REQ-040-request-base-branch-picker.md) (giao diện chọn nhánh) |
| **Tác động** | `backend-go/services/request-service` (domain, usecase `CreateRequest`, `ProposalWorkspace`, `AdvanceExecution`, migration mới), `backend-go/services/task-service` (`WorktreeProvisioner`, `TaskServiceExecuteRequest`), `backend-go/services/api-gateway` (kênh `request.*`, MCP), `backend-go/proto/orca/{request,task}/v1` |

> Nguồn: câu hỏi "người dùng có chọn được nhánh tạo worktree không" ngày 2026-10-19, trả lời bằng đọc mã (mục 1). Chưa chạy Request thật để quan sát nhánh gốc.

---

## 1. Bối cảnh và vấn đề

### 1.1 Hiện trạng: ba nơi tạo worktree theo Request, không nơi nào cho chọn nhánh

| Luồng | Tạo worktree ở đâu | Nhánh gốc hiện tại | Người dùng chọn? |
|---|---|---|---|
| Tạo Request từ task source (Jira, GitHub), nhập tay, MCP, webhook | Không tạo worktree. `CreateRequestRequest` (`request.proto`, trường 1 đến 8) không có trường nhánh | Không áp dụng | Không |
| Plan OpenSpec (mặc định cho `change_request`, `refactor`) | `adapter/grpcclient/proposal_workspace.go` `Ensure`: nhánh `ProposalBranch(số Request)` | `git-gateway.BaseRefDefault` của repo, tức nhánh mặc định | Không |
| Thực thi task (native và OpenSpec) | `task-service/internal/adapter/grpcclient/worktree_provisioner.go` `EnsureWorktree`: nhánh `task/<taskId>`, tại task đầu của Plan, các task sau dùng chung (`advance_execution.go` `sharedWorktree`) | `CreateWorktreeRequest.base_ref` để trống, nên `git worktree add -b` rẽ từ **HEAD hiện tại của checkout chính trên dev server** (`agent/src/relay/agent-git-worktree-handler.ts`) | Không |
| Tạo worktree thủ công (New workspace, Start work từ issue; kênh `worktree.create`) | Do người dùng bấm | `baseBranch` tuỳ chọn | **Có** (ngoài luồng Request) |

Cả hai luồng Request chọn repo là repo đầu tiên của dự án (`ListRepos`, position thấp nhất): `proposal_workspace.go` `repoFor` và `worktree_provisioner.go` `resolveRepoID`.

### 1.2 Vấn đề

1. Người dùng không điều khiển được nhánh mà code của Request được viết trên đó. Ví dụ một bản vá phải rẽ từ nhánh `release/1.4` thay vì `main`, hoặc một tính năng phụ thuộc nhánh tính năng chưa merge.
2. Mọi worktree thực thi của Request phụ thuộc vào nhánh đang được checkout ở checkout chính trên dev server, một trạng thái bên ngoài Orca mà người dùng không thấy và thay đổi theo ai đang làm gì ở đó.

### 1.3 Lệch hành vi cần sửa kèm

Yêu cầu là "mặc định vẫn là nhánh mặc định". Worktree OpenSpec đã đúng, nhưng worktree thực thi task thì rẽ từ HEAD hiện tại, không phải nhánh mặc định. CR này đưa cả hai về cùng một quy tắc (mục 2.2). Đây là **thay đổi hành vi** cho dự án đang chạy: task sẽ rẽ từ nhánh mặc định thay vì HEAD của checkout chính. Xem rủi ro R1.

## 2. Giải pháp đề xuất

### 2.1 Trường `base_ref` trên Request

- `Request.base_ref` (chuỗi): tên nhánh (local hoặc remote, ví dụ `main`, `origin/release/1.4`) hoặc tag. **Rỗng = nhánh mặc định của repo.** Không lưu SHA đã phân giải; ref được phân giải tại thời điểm tạo từng worktree (sau khi fetch, mục 2.5).
- Đặt ở hai chỗ: `CreateRequestRequest.base_ref` (trường mới) và RPC `SetRequestBaseRef`.
- Request cũ có `base_ref` rỗng nên hành vi mặc định là nhánh mặc định (xem mục 1.3 cho phần thay đổi của task).

### 2.2 Quy tắc phân giải (một chỗ duy nhất)

Hàm `ResolveWorktreeBaseRef(request, repo)` trong request-service, dùng cho cả hai luồng:

1. `Request.base_ref` nếu khác rỗng;
2. nếu không, `git-gateway.BaseRefDefault(repo)`.

Không dùng "HEAD hiện tại" làm giá trị mặc định ở bất kỳ nhánh nào. (Cài đặt `worktreeBaseRef` ở mức repo của giao diện là câu hỏi mở 7.1; nếu chốt có thì chèn làm bước giữa.)

### 2.3 Khoá sau khi worktree đầu tiên được tạo

Nhánh gốc chỉ đổi được **trước** khi Request có worktree nào (cả worktree đề xuất lẫn worktree task đầu). Sau đó `SetRequestBaseRef` trả `REQUEST_BASE_REF_LOCKED`. Lý do: một Plan dùng chung một worktree cho mọi task (CR-REQ-013 mục 2.7); đổi gốc giữa chừng không có nghĩa. Muốn đổi nhánh sau đó thì người dùng huỷ hoặc mở lại Request theo luồng hiện có.

- Cờ khoá suy ra, không lưu: `base_ref_locked = (đã có worktree đề xuất) hoặc (có task có worktree_id)`.
- `SetRequestBaseRef` dùng khoá lạc quan `expectedVersion` như các ghi khác (C12 của CONTRACT).
- Quyền: người có quyền sửa Request (cùng mức với `ChangeRequestType`).

### 2.4 Kiểm tra giá trị

- Cú pháp: không rỗng toàn khoảng trắng, không bắt đầu bằng `-`, không chứa `..`, byte NUL, ký tự điều khiển; tối đa 255 ký tự. Lỗi: `REQUEST_BASE_REF_INVALID`.
- Tồn tại: gọi `git-gateway.PrefetchCreateBase{repo_id, base_ref}` ngay lúc đặt; không tìm thấy thì `REQUEST_BASE_REF_NOT_FOUND` kèm danh sách gợi ý rút gọn (git-gateway đã trả `available` trong `WORKTREE_BASE_REF_NOT_FOUND`). Nếu dev server không với tới được lúc đặt thì **vẫn lưu** và kiểm lại lúc tạo worktree, kèm cảnh báo (tránh chặn tạo Request khi agent offline).
- Lỗi lúc tạo worktree (ref biến mất giữa chừng): Request giữ nguyên trạng thái, lỗi hiện ở Execution/Activity với nguyên nhân `base_ref_not_found` để người dùng sửa `base_ref` (chưa khoá vì chưa có worktree).

### 2.5 Truyền xuống các nơi tạo worktree

1. **Worktree đề xuất OpenSpec**: `ProposalWorkspace.Ensure` nhận `BaseRef` trong `EnsureWorkspaceInput`; `usecase` gọi `ResolveWorktreeBaseRef` thay cho `BaseRefDefault` cứng. Khoá idempotency `request-proposal:<id>` giữ nguyên.
2. **Worktree thực thi task**: `AdvanceExecution` (`advance_execution.go:185`, `Tasks.Execute`) truyền `base_ref` đã phân giải. `task.proto`: `TaskServiceExecuteRequest.base_ref` (trường mới). `ExecuteTaskInput.BaseRef` → `WorktreeProvisioner.EnsureWorktree` gửi `CreateWorktreeRequest.base_ref`. Khi `base_ref` rỗng (gọi từ nơi không phải Request, ví dụ chạy task lẻ) task-service tự gọi `BaseRefDefault` thay vì để trống, để quy tắc 2.2 đúng ở mọi đường vào.
3. **Adopt worktree của issue** (`findIssueWorktree`): nếu task nhận lại worktree đã tạo cho cùng issue ngoài, nhánh gốc là của worktree đó. Request chưa khoá `base_ref`, nhưng worktree nhận lại làm `base_ref_locked=true` và `base_ref` hiển thị lấy từ `worktree.base_ref` của project-service nếu có. Xem câu hỏi mở 7.3.

### 2.6 Tương thích git và môi trường chạy

- Git baseline 2.25 (`guides/reference/git-compatibility.md`): chỉ dùng `git worktree add -b <branch> <path> <ref>` và `git fetch`, không dùng tuỳ chọn mới hơn. Không thêm lệnh git mới ngoài hai lệnh này.
- Chạy được trên dev server local, SSH và WSL: mọi thao tác ref đi qua git-gateway và agent như hiện nay (không giả định đường dẫn hay `origin`). Tên remote không cố định `origin`: `PrefetchCreateBase` xử lý, request-service không ghép chuỗi.
- Không đặt tên chỉ-GitHub: trường là `base_ref`, không phải `base_pr` hay `base_branch`.

### 2.7 API và kênh

| Kênh WS / MCP | Thay đổi |
|---|---|
| `request.create` | nhận `baseRef?` |
| `request.setBaseRef` (mới) | `{id, baseRef, expectedVersion}` → `{request}`; `GroupWrite` |
| `request.get`, `request.list` | thêm `baseRef`, `baseRefLocked` |
| MCP | `request_create` nhận `baseRef`; thêm `request_setBaseRef` hoặc dòng loại trừ có lý do; `parity_test` xanh |
| Lỗi mới | `REQUEST_BASE_REF_INVALID`, `REQUEST_BASE_REF_NOT_FOUND`, `REQUEST_BASE_REF_LOCKED` |

### 2.8 Migration

Số tiếp theo của `request-service` (hiện `0098_execution_mode`, kiểm lại khi triển khai): `0099_request_base_ref`, cả Postgres và MySQL: `requests.base_ref text NOT NULL DEFAULT ''` (MySQL: `VARCHAR(255) NOT NULL DEFAULT ''`). Có file `.down.sql` cả hai dialect. Không backfill.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| B1 | `base_ref` ở mức Request, không ở mức task hoặc Phase | Một Plan dùng chung một worktree; chọn theo task không có nghĩa |
| B2 | Rỗng = nhánh mặc định, phân giải lúc tạo worktree | Đúng yêu cầu; nhánh mặc định có thể đổi tên giữa chừng |
| B3 | Khoá khi đã có worktree | Tránh worktree lệch gốc trong cùng Plan |
| B4 | Không lưu SHA | Giữ ý nghĩa "tên nhánh"; SHA là việc của git-gateway lúc tạo |
| B5 | Task rẽ từ nhánh mặc định thay vì HEAD hiện tại | Cùng một quy tắc cho mọi worktree của Request; bỏ phụ thuộc vào trạng thái checkout chính |
| B6 | Dev server offline lúc đặt thì vẫn lưu, kiểm lại sau | Không chặn tạo Request vì agent tạm mất kết nối |

## 4. Tiêu chí chấp nhận

- [ ] `CreateRequest` không có `baseRef` giữ hành vi như cũ cho worktree đề xuất (nhánh mặc định).
- [ ] `CreateRequest` với `baseRef="release/1.4"`: worktree đề xuất OpenSpec của Request được tạo từ `release/1.4`; `worktree.base_ref` trong project-service ghi đúng giá trị.
- [ ] Request `bug`/`task` native với `baseRef="release/1.4"`: worktree `task/<id>` của task đầu rẽ từ `release/1.4`; các task sau dùng chung worktree đó.
- [ ] Request không đặt `baseRef`: worktree task rẽ từ **nhánh mặc định**, không phải HEAD của checkout chính, kể cả khi checkout chính đang ở một nhánh khác.
- [ ] `SetRequestBaseRef` trước worktree đầu tiên thành công và tăng `version`; sau khi có worktree trả `REQUEST_BASE_REF_LOCKED` và không đổi dữ liệu.
- [ ] Giá trị không hợp lệ (`-x`, `a..b`, rỗng khoảng trắng) trả `REQUEST_BASE_REF_INVALID`; nhánh không tồn tại trả `REQUEST_BASE_REF_NOT_FOUND`.
- [ ] Dev server offline lúc đặt: lưu được, lỗi `base_ref_not_found` hiện đúng chỗ nếu tạo worktree thất bại, và sửa được `base_ref` rồi chạy lại.
- [ ] `request.get`/`list` trả `baseRef` và `baseRefLocked`; kênh mới có `ToolSpec` hoặc dòng loại trừ; `parity_test` xanh.
- [ ] Migration xuôi và ngược trên Postgres và MySQL; Request cũ đọc được.
- [ ] Chạy đúng trên dev server local và SSH (một ca mỗi loại).

## 5. Kiểm thử

- Unit: `ResolveWorktreeBaseRef` (rỗng, có giá trị); kiểm tra cú pháp ref; cờ `base_ref_locked` suy ra; `SetRequestBaseRef` khoá lạc quan.
- Usecase có fake: `ProposalWorkspace.Ensure` nhận ref; `AdvanceExecution` truyền `base_ref` vào `Tasks.Execute`; task-service `EnsureWorktree` gửi `base_ref` hoặc tự điền `BaseRefDefault` khi rỗng.
- Contract test hai dialect: cột `base_ref`, mặc định rỗng, đọc/ghi.
- Gateway: `wscompat` test cho `request.create` và `request.setBaseRef`; `parity_test` MCP.
- Git thật: test với repo tạm hai nhánh (`main`, `release/x`) và checkout chính đang ở nhánh thứ ba, kiểm `git rev-parse` của worktree tạo ra. Chạy trong CI ở các phiên bản git của `guides/reference/git-compatibility.md`.
- E2E (`tests/stages/e2e/orca-flows`): suite mới, tên đề xuất `suite_request_base_ref.py`: tạo Request với `baseRef`, `request.get` trả đúng, tạo worktree qua luồng thật với claude giả, kiểm nhánh gốc của worktree bằng `terminal.list`/`git` trên dev server; ca khoá; ca nhánh không tồn tại.

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: đổi mặc định của task từ HEAD sang nhánh mặc định.** Người đang quen làm việc từ nhánh checkout chính sẽ thấy khác. Giảm thiểu: ghi chú phát hành; không có cờ quay lại (giữ hành vi HEAD sẽ phá quy tắc 2.2). Cần đồng ý của người yêu cầu (câu hỏi 7.2).
- **R2: nhiều repo trong một dự án.** Cả hai luồng chỉ dùng repo đầu tiên, nên `base_ref` được hiểu theo repo đó. Chọn repo khác nằm ngoài CR này; nếu sau này có thì `base_ref` cần theo cặp (repo, ref).
- **R3: ref remote cũ.** Nếu không fetch trước, worktree rẽ từ bản cũ. Dựa vào `PrefetchCreateBase`; chưa kiểm chứng hành vi khi remote không với tới được trên SSH.
- **R4: nhánh bị xoá hoặc đổi tên giữa lúc đặt và lúc tạo worktree.** Xử lý bằng mục 2.4 (lỗi rõ, sửa được vì chưa khoá).
- **R5:** số migration `0099` và số trường proto có thể bị CR khác giữ khi triển khai; kiểm tra lại.
- Chưa chạy hệ thống; mô tả hành vi hiện tại dựa trên đọc mã ngày 2026-10-19.

## 7. Câu hỏi còn mở

1. Có dùng cài đặt `worktreeBaseRef` mức repo (đang có ở Settings → Repository của giao diện) làm bước giữa trong quy tắc 2.2 không? Cần xem nơi lưu của nó ở backend (project-service) trước khi chốt.
2. Đồng ý đổi mặc định task sang nhánh mặc định (R1) không?
3. Worktree nhận lại từ issue (mục 2.5.3): `base_ref` hiển thị lấy từ worktree đó có đủ không, hay chặn tạo Request với `base_ref` khác khi đã có worktree của issue?
4. Có cho chọn SHA hoặc chỉ nhánh và tag? Mặc định đề xuất: nhánh và tag; SHA để dành.
5. Task source (Jira, GitHub) có gợi ý sẵn nhánh gốc không (ví dụ PR đã có nhánh đích)? Đề xuất: không trong CR này, `base_ref` vẫn do người dùng đặt.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-002 | `Request` thêm `base_ref`; `CreateRequestRequest` thêm trường |
| CR-REQ-013 | Mục 2.7 (worktree dùng chung): ghi rằng gốc của worktree lấy từ `Request.base_ref`; `Tasks.Execute` mang `base_ref` |
| CR-REQ-016 | Kênh `request.create` nhận `baseRef`, kênh mới `request.setBaseRef`; `request.get` trả `baseRef`, `baseRefLocked` |
| CR-REQ-017 | `ToolSpec` cho `request_setBaseRef` hoặc dòng loại trừ |
| CR-REQ-026 | Worktree đề xuất OpenSpec rẽ từ `base_ref` thay vì luôn `BaseRefDefault` |
| CR-REQ-038 | Cùng chỗ ghim ở `Request`; `request.get` thêm hai trường cạnh `engine`, `executionMode` |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/adapter/grpcclient/proposal_workspace.go` (`Ensure`, `repoFor`)
- `backend-go/services/request-service/internal/usecase/advance_execution.go` (`Tasks.Execute`, `sharedWorktree`)
- `backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go` (`EnsureWorktree`, `resolveRepoID`), `worktree_issue_reuse.go`
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go` (`BaseRef`, `WORKTREE_BASE_REF_NOT_FOUND`)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`CreateWorktreeRequest.base_ref`, `BaseRefDefault`, `PrefetchCreateBase`)
- `backend-go/proto/orca/request/v1/request.proto` (`CreateRequestRequest`), `backend-go/proto/orca/task/v1/task.proto` (`TaskServiceExecuteRequest`)
- `agent/src/relay/agent-git-worktree-handler.ts` (hành vi khi `baseRef` trống)
- [CR-REQ-013](./CR-REQ-013-phase-execution-and-feedback-loop.md), [CR-REQ-038](./CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md), [CR-REQ-040](../request-frontend/CR-REQ-040-request-base-branch-picker.md)
- Mới: `internal/domain/worktree_base_ref.go`, `internal/usecase/set_request_base_ref.go`, migration `0099_request_base_ref`
