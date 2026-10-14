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
orca plan tree $R                                           # phase, task, trạng thái, "(after ...)"
orca approval approve <plan-gate-id>
orca approval list $R --subject phase                       # cột "subject" là id của Phase
orca approval approve <phase-gate-id>
orca plan start-phase $R <phase-task-id>                    # giao task cho agent
```

`orca approval pending` liệt kê gate đang chờ bạn ở mọi Request. `approve` / `reject` mặc định duyệt đúng `version` + `digest` đọc lúc đó (in ra stderr); truyền cả `--version` và `--digest` nếu muốn duyệt đúng cái bạn đã xem. `reject` bắt buộc có `--comment`.

Lệnh có từ trước: `worktree create`, `agent status|send|wait`, `snapshot`, `daemon status|stop`, `serve`.

## Đã biết chưa làm được

- **Phương án rủi ro cao**: `solution choose` báo `requiresConfirmation`, nhưng gateway chưa có kênh xác nhận (nhập lại tiêu đề), nên duyệt bị từ chối (`REQUEST_DECISION_NOT_EFFECTIVE`). Chọn phương án khuyến nghị hoặc xác nhận trên UI. (TASK-FE-E2E-028)
- **Dừng một task đang chạy**: chưa có lệnh/REST. `request cancel` bị chặn (`REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`) tới khi các task chạy xong.
- **Project/repo/dev server**: chưa có lệnh `project`; tạo bằng UI hoặc REST (`/v1/projects`). Project không có repo trên dev server đang kết nối thì các bước AI trả `REQUEST_SOLUTION_NO_CONNECTION`.
- `plan tree` sắp xếp theo tiêu đề vì dịch vụ không trả thứ tự.
