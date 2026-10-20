# Bật engine OpenSpec và Solution bằng agent (tri thức tdd-agent)

Hai cờ theo project, mặc định **tắt**; tắt thì Request chạy y như trước.

| Cờ | Tác dụng khi bật |
|---|---|
| `engine = openspec` | `plan generate` cho agent viết một change OpenSpec (trên nhánh `request/<số>-proposal` của worktree riêng); Orca kiểm cấu trúc rồi dựng Plan từ `tasks.md`. |
| `solution_agent = true` | Solution do agent chỉ đọc viết, có đọc đồ thị tri thức của project qua MCP; kết quả có `design_references` (đã đọc gì, thiếu gì, xung đột với Decision/Constraint nào). |

Orca **không** gọi MCP hay đồ thị tri thức; agent mà Orca chạy trên dev server mới gọi. Thiếu phần tri thức thì Solution và Plan vẫn được viết, chỉ không có tham chiếu (cờ `available=false`).

## Cài trên dev server của project

1. **OpenSpec CLI**: `openspec --version` phải chạy được (nếu cần đặt bản tối thiểu: `orca engine set --project <id> --min-version 1.2.0`).
2. **tdd-agent** có trong `PATH` và repo của project đã có `.tdd-agent/config.toml`. Orca chỉ kiểm tệp có mặt, không truy vấn đồ thị.
3. **Skill** (một lần cho mỗi dev server):

   ```bash
   npx skills add https://github.com/vnp-community/orca --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global
   ```

   Nguồn là repo `vnp-community/orca` (thư mục `skills/`), không phải repo gốc `stablyai/orca` vì repo gốc không có bốn skill này. `npx skills add` đọc nhánh mặc định của repo, nên skill phải đã nằm trên nhánh đó; máy chủ cần quyền đọc repo nếu repo riêng tư. Muốn thử một nhánh chưa hợp nhất, chép tay `skills/tdd-*` vào `~/.claude/skills/`.

   Cập nhật từng skill: `npx skills update <tên> --global`. Sau khi mục `Language` của `tdd-openspec-author` (CR-REQ-046) được hợp nhất vào nhánh mặc định, chạy `npx skills update tdd-openspec-author --global` trên từng dev server để skill biết ngôn ngữ văn xuôi do prompt nêu. Máy chủ có thể trỏ nguồn khác bằng `REQUEST_SKILLS_REPOSITORY_URL` (hiện trong gợi ý sửa lỗi).
4. **Thư mục làm việc của agent** (`AGENT_WORK_DIR`) phải chứa cả repo lẫn các worktree nằm cạnh nó: worktree proposal và worktree task là `<repo>-<nhánh>` cạnh repo, và agent từ chối ghi tệp ngoài thư mục này (`Path outside project root`), nên một repo đứng ngoài khiến run `plan generate` thất bại sau khi đã tốn chi phí. Kiểm trước khi chạy agent thật bằng `python3 tests/stages/e2e/orca-flows/workspace_probe.py --worktree-id <id> --worktree-path <thư mục>` trên worktree proposal của một Request thử: nó chạy đúng các lời gọi tệp và git mà Orca dùng (stat, đọc, ghi, commit, huỷ) và không tốn lượt AI.
5. **Agent Orca** đủ mới (báo `agent.execPrompt.knowledgeHub`, `agent.execPrompt.permissionProfile`, ...). Agent cũ không bị hỏng: chỉ không dùng được hai cờ.

## Bật mặc định cho cả triển khai

Muốn mọi project **chưa có cài đặt riêng** chạy luồng mới (UI cũng vậy, không cần `engine set` từng project), đặt cho request-service:

```bash
REQUEST_DEFAULT_ENGINE=openspec          # openspec (mặc định của deploy/dev) hoặc native
REQUEST_DEFAULT_SOLUTION_AGENT=true      # true (mặc định của deploy/dev) hoặc false
```

Trong `deploy/dev` hai biến này đọc từ `.env` qua `docker-compose.yml`; đổi xong phải tạo lại container (`./deploy/dev/scripts/redeploy-local.sh request-service api-gateway`). Cài đặt đã lưu của một project luôn thắng giá trị mặc định, kể cả khi nó tắt hết; project mặc định hiện `version 0`. Giá trị không đọc được thì giữ `native` và tắt, kèm một dòng cảnh báo trong log. Lưu ý: engine của một Request bị ghim ở lần sinh đầu tiên, nên Request đã sinh trước khi bật mặc định vẫn chạy engine cũ.

## Bật và kiểm tra

```bash
orca engine show --project <id> --check
orca engine set  --project <id> --engine openspec --solution-agent=true     # chỉ admin
```

`--check` (và lúc lưu cài đặt) hỏi lại chính dev server và bỏ qua cả cache kết quả sạch lẫn hồ sơ khả năng của agent (hồ sơ này hạn một ngày nên skill vừa cài sẽ không còn báo thiếu); lúc chạy một run vẫn dùng cache sạch 10 phút, lỗi thì không cache nên sửa xong thấy ngay:

- **blocked** (chặn engine `openspec`): không có dev server hoặc repo path; `openspec` thiếu/cũ/không trả lời; agent không viết được change trong worktree dưới profile quyền.
- **knowledge hub unused** (chỉ cảnh báo, engine vẫn chạy không có tri thức): agent không tải được hub; `tdd-agent` không có trong `PATH`; thiếu skill; repo không có `.tdd-agent/config.toml`.
- **warning**: repo có tệp chưa commit (đề xuất dùng worktree riêng nên không ảnh hưởng); không kiểm được tệp cấu hình.

Mỗi dòng kèm `code` (`REQUEST_ENGINE_*`, `REQUEST_KNOWLEDGE_*`) và gợi ý sửa. Lưu cài đặt không bao giờ bị chặn bởi kiểm tra; đổi cài đặt được ghi audit (`request.engine_settings.set`).

## Khi thiếu khả năng lúc chạy

- `solution_agent` bật nhưng dev server không tải được hub, không có đường dẫn repo hay không đọc được khả năng: Solution đi `ai.complete` như cũ và ghi log lý do.
- Engine `openspec` mà dev server thiếu `openspec` hoặc agent quá cũ: run thất bại với mã `REQUEST_ENGINE_*` kèm gợi ý, Request không bị hỏng.

## Chạy task của một Plan OpenSpec

Task của Plan do engine `openspec` viết chạy trên đường hợp đồng của task-service: agent kết thúc bằng một khối kết quả có nonce, Orca đọc `status`, `checks_run`, `knowledge` và `writeback`; nếu dev server có `agent.execPrompt.knowledgeHub` thì agent còn đọc được đồ thị tri thức từ **thư mục gốc của repo** (không phải worktree của task). Task có dòng `Verify:` chỉ được coi là xong khi `checks_run` có ít nhất một kiểm tra thoát với mã 0; báo `done` mà không có bằng chứng thì tính là một lần thất bại (`VERIFY_NOT_PASSED`, được thử lại theo số lần tối đa). Xem kết quả bằng `orca plan result <task-id>`; phần `knowledge`/`writeback` là lời của agent, Orca không kiểm.

**Rủi ro đã biết (B4):** mọi lần thực thi task vẫn chạy với `trustPreset: full` (agent không có người duyệt từng lệnh). Cổng duyệt Plan và Phase, kiểm phạm vi `Code:` và bằng chứng `checks_run` là các lớp bù, không thay thế việc xem diff trước khi merge.

## Dọn worktree của proposal

Mỗi lần `plan generate` bằng engine `openspec` tạo một worktree trên nhánh cục bộ `request/<số>-proposal` (không bao giờ đẩy lên remote). Quy tắc dọn, chạy tức thì khi huỷ Request và định kỳ 10 phút một lần:

| Tình huống | Worktree | Nhánh và gói đã lưu |
|---|---|---|
| Request bị huỷ | gỡ ngay; gói chuyển `abandoned` | giữ |
| Request hoàn thành | gỡ sau `REQUEST_PROPOSAL_RETENTION` (mặc định 7 ngày; đổi bằng `72h` hoặc `3d`), hoặc ngay khi gói đã `archived` | giữ |
| Run lỗi (worktree giữ lại để điều tra) | gỡ sau thời hạn trên; gói vẫn `preparing` vì Request còn thể thử lại | giữ |
| Request còn hoạt động, gói `ready` | giữ | giữ |

Worktree của một run **đang chạy** không bao giờ bị đụng, và chỉ gỡ worktree nằm đúng trên nhánh `request/<số>-proposal` của chính Request đó. Gỡ tay (chủ Request hoặc admin): `orca request cleanup <request-id>`; gói vẫn đọc được bằng `orca plan bundle`.

Kiểm tra mồ côi: `python3 tests/stages/e2e/orca-flows/report_orphan_worktrees.py --repo <đường dẫn repo của project>` (thoát 1 nếu có worktree của Request đã huỷ hoặc không còn tồn tại; worktree của Request đã hoàn thành chỉ được liệt kê).

## Ghi ngược tri thức và nợ duyệt của KG

Orca **không bao giờ** áp thay đổi vào đồ thị tri thức (quyết định T1.7): agent chỉ sửa tài liệu thiết kế trong worktree của task và gọi `changeset_propose` (chạy thử, không commit). `orca request show` cho thấy báo cáo của các agent: tài liệu đã đổi, `proposal_id`, ok hay không, số phần tử bị chạm, và (khi Request `completed`) danh sách việc còn lại cho người. Một người áp đề xuất và chạy `tdd-agent -C <repo> generate`.

### Bàn giao khi Request hoàn thành

`orca request show <id>` (thêm `--json` cho script, khoá `handover.items[]` với `kind`, `text`, `command`) in những việc còn lại cho người, hoặc ghi rõ không còn gì: worktree của các task (kiểm xem đã hợp nhất chưa), nhánh proposal và trạng thái archive (`archived` ⇒ gợi ý `git merge`; archive thử 5 lần vẫn lỗi ⇒ cảnh báo kèm lệnh `openspec archive <id> --yes` để chạy tay, Request vẫn `completed`), đề xuất ChangeSet và tài liệu đã đổi, xung đột Decision/Constraint mà Solution đã duyệt báo. Các lệnh chỉ là gợi ý, Orca không chạy.

Đo nợ duyệt trước và sau mỗi lần áp hay `generate` (Orca không đọc KG nên số liệu do chính tdd-agent in):

```bash
python3 specs/backend-go/crs/v8/tdd-agent-integration/evidence/review_debt.py snapshot --repo <repo> --out before.json
# ... áp đề xuất / generate ...
python3 specs/backend-go/crs/v8/tdd-agent-integration/evidence/review_debt.py snapshot --repo <repo> --out after.json
python3 specs/backend-go/crs/v8/tdd-agent-integration/evidence/review_debt.py compare before.json after.json
```

Ngưỡng và cách gom lô (từ phép đo T1.6 trên KG pilot):

- Đề xuất chỉ **thêm phần tử `own`** (`ChangeSpecItem`, `WorkItem`, `TestReceipt`) và cạnh giữa chúng, không cạnh tới tài liệu: nợ duyệt thêm **0** ở cỡ đến 20 thao tác; gom lô thoải mái và áp theo từng Request.
- Đề xuất **sửa phần tử tài liệu** hoặc nối `traces_to` vào chúng: một thao tác có thể đặt lại cả thành phần liên thông (597 phần tử trên KG pilot, bất kể cỡ ChangeSet). Hoãn áp tới khi chủ KG chuẩn bị một đợt duyệt; sửa tài liệu rồi `generate` thay vì áp từng đề xuất.
- Duyệt có chọn lọc theo lớp: `review_debt.py show` liệt kê số phần tử theo lớp và tài liệu; duyệt trước các lớp nhỏ, dễ kiểm (Decision, Constraint, Component) rồi mới đến Requirement và Attribute.
- Quy tắc ngón tay cái: chạy `snapshot` ngay trước khi áp; nếu `compare` cho thấy hơn vài chục phần tử quay lại `needs_review` thì dừng và hỏi chủ KG.
