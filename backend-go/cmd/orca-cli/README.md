# orca — dùng Orca từ dòng lệnh (không cần UI)

CLI gọi REST của api-gateway với bearer token, cùng các bước mà UI thực hiện cho một Request: tạo → phân loại → xác nhận loại → trả lời câu hỏi → phân tích (Solution) → chọn phương án → duyệt → Plan → duyệt Plan/Phase → chạy Phase.

## Cài và đăng nhập

```bash
cd backend-go/cmd/orca-cli && go build -o orca .          # đặt file `orca` vào PATH
ORCA_PASSWORD='...' orca login --api-url http://localhost:8081 --email you@example.com
# hoặc: printf '%s' "$PW" | orca login --api-url ... --email ... --password-stdin
```

Token lưu ở thư mục cấu hình người dùng (`credentials.json`, mode 0600). Có thể thay bằng `ORCA_API_URL` + `ORCA_API_TOKEN`. Mọi lệnh nhận `--json` (một đối tượng JSON trên stdout). Mã thoát: `0` xong, `1` lỗi từ dịch vụ/mạng/hết hạn chờ, `2` dùng sai lệnh.

## Một Request từ đầu đến cuối

Hướng dẫn từng bước cho một CR bất kỳ (trạng thái, gate, khi dừng giữa chừng, bàn giao): [docs/guides/request/cli-end-to-end.md](../../../docs/guides/request/cli-end-to-end.md). Dưới đây là bản tóm tắt lệnh.

```bash
R=$(orca --json request create --project <project-id> --title "..." --body-file spec.md | jq -r .request.id)
orca request classify $R                                    # AI gợi ý loại/cỡ (chạy nền)
orca request wait $R --until awaiting_type_confirmation
orca request confirm-type $R --type change_request --size S
orca clarification list $R                                  # nếu Request chuyển sang awaiting_information
orca clarification answer <clarification-id> --answer <question-id>="văn bản" --accept-default <question-id>
orca solution generate $R --wait                            # phân tích; in các phương án (* = khuyến nghị)
orca solution choose $R <solution-id> --option opt-1        # phương án không khuyến nghị cần --rationale
orca approval list $R --subject solution
orca approval approve <approval-id>                         # đọc version + digest ngay trước khi duyệt
orca plan generate $R                                       # propose → chờ AI → commit
orca plan proposal $R                                       # trạng thái run và kế hoạch; với engine openspec: nhánh, commit, kết quả kiểm, tham chiếu tri thức
orca plan bundle $R                                         # (openspec) danh sách tệp của gói OpenSpec đứng sau kế hoạch
orca plan bundle $R --file tasks.md                         # đọc một tệp: proposal.md, design.md, specs/<capability>/spec.md, tasks.md
orca request cleanup $R                                      # gỡ worktree của proposal (giữ nhánh và gói đã lưu); chủ Request hoặc admin
orca plan result <task-id>                                   # kết quả lượt chạy mới nhất của task: kiểm tra, tệp, và điều agent báo đã đọc/ghi trong đồ thị tri thức
orca plan tree $R                                           # phase, task, trạng thái, "(after ...)"
orca approval approve <plan-gate-id>
orca approval list $R --subject phase                       # cột "subject" là id của Phase
orca approval approve <phase-gate-id>
orca plan start-phase $R <phase-task-id>                    # giao task cho agent
```

`orca approval pending` liệt kê gate đang chờ bạn ở mọi Request. `approve` / `reject` mặc định duyệt đúng `version` + `digest` đọc lúc đó (in ra stderr); truyền cả `--version` và `--digest` nếu muốn duyệt đúng cái bạn đã xem. `reject` bắt buộc có `--comment`.

Với engine `openspec` (kế hoạch do agent viết thành một change OpenSpec trên nhánh `request/<số>-proposal`), hãy đọc gói bằng `orca plan bundle` trước khi chạy `orca approval approve <plan-gate-id>`: quyết định duyệt gắn với digest của cả gói, nên sinh lại hoặc sửa gói sau khi mở cổng làm quyết định cũ không còn hiệu lực. Engine `native` không có gói; `orca plan bundle` nói rõ điều đó.

Bật engine `openspec` và Solution bằng agent cho một project (chỉ admin; mặc định tắt cả hai):

```bash
orca engine show --project <project-id> --check                       # trạng thái hai cờ + kiểm dev server (openspec, agent, skill, .tdd-agent)
orca engine set  --project <project-id> --engine openspec --solution-agent=true
```

`engine set` chỉ đổi mục được nêu và gửi lại `version` vừa đọc; thiếu khả năng thì vẫn lưu, in lý do kèm lệnh sửa, và Request tiếp tục chạy chế độ cũ. Cài đặt trên dev server: [docs/guides/request/knowledge-hub-setup.md](../../../docs/guides/request/knowledge-hub-setup.md).

Lệnh có từ trước: `worktree create`, `agent status|send|wait`, `snapshot`, `daemon status|stop`, `serve`.

## Đã biết chưa làm được

- **Lệnh dành cho admin** (`orca engine set`): token của `orca login` hiện chưa mang claim `role` nên request-service coi người gọi không phải admin (`REQUEST_FORBIDDEN: admin role required`). Dùng phiên web của admin (kênh `request.engineSettingsSet` hoặc UI) cho tới khi token mang role; `orca engine show` đọc được bình thường.
- **Phương án rủi ro cao**: `solution choose` báo `requiresConfirmation`, nhưng gateway chưa có kênh xác nhận (nhập lại tiêu đề), nên duyệt bị từ chối (`REQUEST_DECISION_NOT_EFFECTIVE`). Chọn phương án khuyến nghị hoặc xác nhận trên UI. (TASK-FE-E2E-028)
- **Dừng một task đang chạy**: chưa có lệnh/REST. `request cancel` bị chặn (`REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`) tới khi các task chạy xong.
- **Project/repo/dev server**: chưa có lệnh `project`; tạo bằng UI hoặc REST (`/v1/projects`). Project không có repo trên dev server đang kết nối thì các bước AI trả `REQUEST_SOLUTION_NO_CONNECTION`.
- `plan tree` sắp xếp theo tiêu đề vì dịch vụ không trả thứ tự.
