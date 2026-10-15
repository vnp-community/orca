# Chạy một Change Request từ đầu đến cuối bằng `orca` CLI

Hướng dẫn này đi qua toàn bộ vòng đời của một Request loại `change_request` (CR) bằng dòng lệnh: từ tạo, phân tích, lập kế hoạch, thực thi đến hoàn thành và bàn giao. Các bước giống hệt UI; CLI chỉ gọi REST của api-gateway.

Danh sách lệnh đầy đủ: [backend-go/cmd/orca-cli/README.md](../../../backend-go/cmd/orca-cli/README.md). Cài đặt engine và tri thức trên dev server: [knowledge-hub-setup.md](./knowledge-hub-setup.md). Bài e2e tự động hoá đúng luồng này: `tests/stages/e2e/orca-flows/run_cli_flow.py` (mục [Tự động hoá](#tự-động-hoá)).

## 0. Điều kiện trước

| Cần | Kiểm bằng |
|---|---|
| Stack Orca chạy, có dev server **connected** của project | `orca daemon status`, UI mục Dev servers |
| Project đã có repo trên dev server đó | project không có repo thì mọi bước AI trả `REQUEST_SOLUTION_NO_CONNECTION` |
| Agent (Claude) đăng nhập được trên dev server | chạy `claude` một lần trên máy đó |
| (Tuỳ chọn) engine `openspec` + Solution có tri thức | `orca engine show --project <id> --check` |

Project, repo và dev server hiện tạo bằng UI hoặc REST (`/v1/projects`); CLI chưa có lệnh `project`. Bật engine `openspec` cần quyền admin (xem [knowledge-hub-setup.md](./knowledge-hub-setup.md)). Không bật thì Request chạy engine `native`, các bước dưới vẫn đúng, chỉ khác phần Plan (mục 5).

Tuỳ chọn nhưng nên làm: đặt `AGENT_WORK_DIR` của agent bao cả repo lẫn các worktree nằm cạnh nó, nếu không agent ghi tệp thất bại giữa chừng.

## 1. Cài và đăng nhập

```bash
cd backend-go/cmd/orca-cli && go build -o orca .     # đặt file `orca` vào PATH
ORCA_PASSWORD='...' orca login --api-url http://localhost:8081 --email you@example.com
```

- **Token đăng nhập sống khoảng 10 phút và không gia hạn được.** Lệnh chờ lâu (`plan generate`, `request wait`) có thể chết giữa chừng với `UNAUTHENTICATED` trong khi run phía server vẫn chạy tiếp. Lúc đó đăng nhập lại rồi đọc trạng thái bằng `orca plan proposal <request-id>` hoặc `orca request show`; đừng chạy lại lệnh sinh.
- Mọi lệnh nhận `--json` (một đối tượng JSON trên stdout). Mã thoát: `0` xong, `1` lỗi dịch vụ, mạng hoặc hết hạn chờ, `2` dùng sai lệnh.
- Dùng được `ORCA_API_URL` + `ORCA_API_TOKEN` thay cho `login`.

## 2. Bản đồ trạng thái

```
new → classifying → awaiting_type_confirmation → (awaiting_information) → analyzing
    → awaiting_analysis_approval → planning → awaiting_plan_approval → executing → completed
```

Ngoài luồng chính: `awaiting_information` (AI cần bạn trả lời), `request_backlog` (dừng vì task thất bại hoặc cần người; mở lại được), `cancelled`.

Theo dõi ở bất kỳ bước nào:

```bash
orca request show $R             # trạng thái, phiên bản, bàn giao khi đã xong
orca request wait $R --until awaiting_analysis_approval --timeout 30m
orca approval pending            # mọi gate đang chờ bạn, ở mọi Request
```

`request wait` luôn dừng khi Request `completed` hoặc `cancelled`. Mặc định chờ 10 phút; chờ lâu hơn thì đặt `--timeout`, và nhớ giới hạn token ở mục 1.

## 3. Tạo, phân loại, xác nhận loại

Viết mô tả CR thành một tệp: mục tiêu, phạm vi, **tiêu chí chấp nhận** dạng GIVEN/WHEN/THEN. Tiêu chí viết rõ ở đây sẽ thành scenario có tên trong gói OpenSpec và thành điều kiện hoàn thành của task.

```bash
R=$(orca --json request create --project <project-id> --title "Thêm endpoint /healthz" --body-file cr.md | jq -r .request.id)
orca request classify $R                                   # AI gợi ý loại, cỡ, mức khẩn (chạy nền)
orca request wait $R --until awaiting_type_confirmation
orca request show $R                                       # đọc gợi ý
orca request confirm-type $R --type change_request --size M
```

- `--size` quyết định ai được duyệt. Theo chính sách mặc định, người tạo Request tự duyệt được cỡ S và M. **Cỡ L cần người duyệt khác người tạo** (Solution, Plan, danh sách task): stack chỉ có một người dùng sẽ không qua được gate, cần chính sách riêng do admin cấp. Đừng hạ cỡ để né; cỡ phải đúng thực tế.
- Tạo lại cùng một Request an toàn nếu truyền `--client-request-id` (cùng khoá trả cùng Request).

## 4. Phân tích: trả lời câu hỏi, chọn phương án, duyệt Solution

### 4.1 Câu hỏi của AI

Sau khi xác nhận loại, AI có thể hỏi thêm; Request sang `awaiting_information`.

```bash
orca clarification list $R                                 # clarification đang mở và các câu hỏi
orca clarification answer <clarification-id> \
  --answer <question-id>="văn bản" \
  --accept-default <question-id>
```

- Câu hỏi dạng danh sách (tiêu chí chấp nhận) nhận mỗi dòng một mục: truyền chuỗi nhiều dòng.
- `--partial` lưu nháp chưa gửi. Gửi xong Request tự chạy tiếp.

### 4.2 Sinh Solution

Thường Request tự vào `analyzing` sau khi bạn trả lời. Khi cần sinh hay sinh lại:

```bash
orca solution generate $R --wait --timeout 30m             # in các phương án; * là phương án khuyến nghị
orca solution generate $R --feedback "ưu tiên không đổi schema"   # làm lại theo góp ý
```

Với `solution_agent` bật, Solution do agent chỉ đọc viết và có `design_references`: đã đọc phần tử tri thức nào, thiếu gì, xung đột với quyết định hay ràng buộc nào. **Đọc phần xung đột trước khi chọn**: đó là giá trị chính của luồng tri thức.

### 4.3 Câu hỏi chặn trong Solution

Nếu Solution có câu hỏi mở loại chặn, gate Solution đóng cho tới khi chúng được trả lời:

```bash
orca clarification ask $R --solution <solution-id>        # biến câu hỏi chặn thành clarification
orca clarification answer <clarification-id> --answer <question-id>="..."
# phân tích chạy lại; lấy Solution mới bằng: orca solution list $R
```

### 4.4 Chọn và duyệt

```bash
orca solution list $R
orca solution choose $R <solution-id> --option opt-1 --comment "ghi chú cho người duyệt"
orca approval list $R --subject solution
orca approval show <approval-id>                           # version và digest đang có hiệu lực
orca approval approve <approval-id> --comment "đã đọc xung đột ADR-12"
```

- Chọn phương án **không** được khuyến nghị bắt buộc có `--rationale`.
- `approve` mặc định duyệt đúng `version` và `digest` đọc lúc đó (in ra stderr). Muốn duyệt đúng cái bạn đã xem thì truyền cả `--version` và `--digest`; nếu gate đổi trong lúc đó, lệnh bị từ chối thay vì duyệt nhầm.
- `reject` bắt buộc có `--comment`.

### 4.5 Phương án rủi ro cao

`solution choose` có thể báo cần xác nhận. Đó là hành động của **một người**: gate Solution đóng cho tới khi có người gõ lại tiêu đề của phương án đã chọn.

```bash
orca decision list $R
orca decision confirm <decision-id> --text "tiêu đề đúng của phương án đã chọn"
```

Chỉ làm khi bạn thực sự là người có thẩm quyền quyết định. Không nhờ agent hay script xác nhận thay.

## 5. Lập kế hoạch (Plan) và duyệt

```bash
orca plan generate $R                  # đề xuất, chờ AI, rồi commit kế hoạch
orca plan generate $R --feedback "tách task migration khỏi task code"   # sinh lại theo góp ý
```

Thời gian chờ mặc định là 4 phút với engine `native`, 64 phút khi agent viết một change OpenSpec (cỡ L có thể mất 13 đến 25 phút). Lệnh chết vì token hết hạn thì plan vẫn chạy: đăng nhập lại, chờ `orca plan proposal $R` báo `succeeded`, rồi `orca plan commit $R` để commit kế hoạch. Đừng chạy lại `plan generate`, vì nó bắt đầu một run mới và tốn thêm.

```bash
orca plan proposal $R                  # trạng thái run, kế hoạch đề xuất, kết quả kiểm cấu trúc
orca plan tree $R                      # phase, task, trạng thái, "(after ...)"
```

### Engine `openspec`: đọc gói trước khi duyệt

Kế hoạch là một change OpenSpec trên nhánh `request/<số>-proposal`. Quyết định duyệt gắn với digest của **cả gói**.

```bash
orca plan bundle $R                              # danh sách tệp
orca plan bundle $R --file proposal.md
orca plan bundle $R --file design.md
orca plan bundle $R --file specs/<capability>/spec.md
orca plan bundle $R --file tasks.md
```

Đọc để trả lời các câu hỏi này:

- Mỗi tiêu chí chấp nhận của CR có một scenario, tên nêu mã AC không?
- Mỗi task có đủ bốn dòng: `Serves:` (phục vụ AC nào), `Code:` (đường dẫn được phép sửa), `Verify:` (đúng một lệnh kiểm), `Done when:`?
- Các cảnh báo xung đột tri thức trong `orca plan proposal` đã được xử lý trong thiết kế hay chưa?
- `Code:` có chạm vùng cấm (cấu hình dự án, tài liệu gốc) không?

Sinh lại hay sửa gói sau khi gate mở làm quyết định cũ mất hiệu lực, nên đọc xong mới duyệt.

```bash
orca approval list $R --subject plan
orca approval approve <plan-gate-id> --comment "đã đọc proposal, design, spec, tasks"
```

Cỡ L còn có gate danh sách task cần người duyệt khác. Dọn worktree của proposal khi không cần nữa: `orca request cleanup $R` (giữ nhánh và gói đã lưu).

## 6. Thực thi theo từng Phase

Mỗi Phase có một gate. Duyệt gate rồi giao Phase cho agent:

```bash
orca approval list $R --subject phase              # cột subject là id của Phase
orca approval approve <phase-gate-id>
orca plan start-phase $R <phase-task-id>           # task của Phase đi tới agent
orca plan tree $R                                  # theo dõi trạng thái task
```

Lặp cho tới khi hết Phase. Gate Phase kế tiếp chỉ mở khi Phase trước xong, nên dùng `orca approval pending` để thấy cái nào đang chờ bạn.

Với mỗi task xong, xem agent đã làm gì:

```bash
orca plan result <task-id>      # kiểm tra đã chạy (lệnh, exit), tệp đã đổi, điều agent báo đã đọc ghi trong tri thức
```

Quy tắc cần biết:

- Task có dòng `Verify:` chỉ được coi là `done` khi có ít nhất một kiểm tra **thoát mã 0**. Báo xong mà không có bằng chứng tính là một lần thất bại (`VERIFY_NOT_PASSED`) và được thử lại tới số lần tối đa.
- Phần `knowledge` và `writeback` trong kết quả là lời của agent, Orca không kiểm.
- Agent làm việc trên một worktree dùng chung cho cả plan (`<repo>-task-<id>`) và **để code chưa commit**. Xem diff trong worktree đó, không phải trên nhánh.
- Mọi lượt thực thi task chạy với quyền đầy đủ (rủi ro B4): cổng duyệt, kiểm phạm vi `Code:` và bằng chứng `checks_run` là lớp bù, không thay cho việc xem diff trước khi hợp nhất.

### Khi dừng giữa chừng

Request về `request_backlog` khi task thất bại hết lượt thử hoặc cần thông tin từ người (`needs_info`, ví dụ "ai là người duyệt bản sửa này?").

```bash
orca request show $R                                  # lý do và task nào
orca plan result <task-id>                            # agent đã thử và vướng ở đâu
orca request reopen $R --note "đã có người duyệt: ..."   # quay lại bước xác nhận loại
```

Sau `reopen` Request ở `analyzing` và chưa có run nào: chạy `orca solution generate $R` rồi đi lại từ mục 4 (xác nhận loại, Solution, Plan). Không có lệnh dừng một task đang chạy; `request cancel` bị chặn (`REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`) tới khi các task chạy xong.

## 7. Hoàn thành và bàn giao

Khi mọi task xong, Request sang `completed`. Với engine `openspec`, vài chục giây sau Orca tự:

1. đánh dấu `- [x]` từng task trong `tasks.md` của nhánh proposal (một commit mỗi lượt quét, chỉ đụng tệp đó);
2. chạy `openspec archive <id> --yes` trên nhánh đó khi Request `completed` (kiểm phạm vi thay đổi; thử tối đa 5 lần với giãn cách tăng dần, lỗi cuối cùng không đổi trạng thái Request).

```bash
orca request show $R           # mục handover: việc còn lại cho người
orca plan proposal $R          # trạng thái gói: ready, archived; lỗi đồng bộ nếu có
```

`handover` liệt kê, kèm lệnh gợi ý (Orca không tự chạy): worktree của các task, nhánh proposal và việc hợp nhất khi gói đã `archived`, lệnh `openspec archive <id> --yes` chạy tay nếu archive thất bại, các đề xuất thay đổi tri thức do agent tạo, xung đột mà Solution đã báo. Người làm các việc cuối: **commit code từ worktree task, hợp nhất, và nếu có đề xuất tri thức thì áp rồi chạy `tdd-agent generate`**. Orca không áp thay đổi vào đồ thị tri thức và không push.

Dọn:

```bash
orca request cleanup $R        # gỡ worktree proposal (nhánh và gói đã lưu còn)
```

Worktree của task gỡ bằng `git worktree remove` sau khi bạn đã lấy code.

## 8. Bảng tra nhanh

| Tình huống | Làm gì |
|---|---|
| `UNAUTHENTICATED` giữa lúc chờ | `orca login` lại; đọc `orca plan proposal $R`; đừng chạy lại lệnh sinh |
| Gate không duyệt được, nói sai version hoặc digest | `orca approval show <id>` lấy cái mới, đọc lại nội dung, duyệt lại |
| `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` ở cỡ L | cần người duyệt khác người tạo, hoặc chính sách riêng do admin cấp |
| `REQUEST_SOLUTION_NO_CONNECTION` | dev server không connected hoặc project thiếu repo trên đó |
| `REQUEST_ENGINE_*`, `REQUEST_KNOWLEDGE_*` | `orca engine show --project <id> --check` và làm theo gợi ý |
| Plan lỗi với `Path outside project root` | `AGENT_WORK_DIR` chưa bao worktree nằm cạnh repo |
| Dịch vụ vừa redeploy, lệnh lỗi `REQUEST_NOT_IMPLEMENTED` hoặc `REQUEST_ENGINE_NO_CONNECTION` | khởi động lại các dịch vụ gọi nó (infra-fleet, git-gateway, task-service, request-service, api-gateway) |
| Muốn làm lại Solution hay Plan | `--feedback "..."` ở `solution generate` hoặc `plan generate` |
| Bỏ Request | `orca request cancel $R --reason "..."` rồi `orca request cleanup $R` |

## Tự động hoá

Script `tests/stages/e2e/orca-flows/run_cli_flow.py --real knowledge` chạy đúng luồng trên (repo tạm, cỡ M, Claude thật, tốn vài đô đến vài chục đô), tự đăng nhập lại trước khi token hết hạn và tự duyệt mọi gate của Request thử. Dùng nó làm mẫu cho một script của bạn, nhưng **không** để script tự duyệt gate của Request thật: duyệt là quyết định của người, và mọi gate hiển thị digest để bạn xác nhận đúng cái đã đọc.

Mẫu vòng lặp Phase cho script (cần `jq`):

```bash
while [ "$(orca --json request show $R | jq -r .request.status)" != completed ]; do
  orca --json approval list $R --subject phase \
    | jq -r '.approvals[] | select(.status=="pending") | "\(.id) \(.subjectId)"' \
    | while read -r gate phase; do
        orca approval approve "$gate" --comment "đã đọc" && orca plan start-phase $R "$phase"
      done
  orca request wait $R --until completed,request_backlog --timeout 5m || true
  [ "$(orca --json request show $R | jq -r .request.status)" = request_backlog ] && break
done
```
