# Bộ e2e nghiệm thu các luồng ORCA (`orca-flows`)

Bộ test chạy trên **stack Docker local đang chạy** (`deploy/dev`), đi qua đúng đường mà UI dùng: đăng nhập `POST /auth/local`, rồi gọi các kênh `invoke` trên WebSocket `/ws` của api-gateway. Không mock service nào. Báo cáo kết quả và các lỗi tìm được: [`../../../ORCA-E2E-ACCEPTANCE-REPORT.md`](../../../ORCA-E2E-ACCEPTANCE-REPORT.md).

Chỉ dùng thư viện chuẩn của Python 3.9+ (có client WebSocket riêng, `ws_rpc_client.py`), nên không cần `pip install`.

## Chạy

```bash
cd tests/stages/e2e/orca-flows
python3 run_acceptance.py                  # toàn bộ
python3 run_acceptance.py request matrix   # một số suite (tự thêm devserver)
python3 run_acceptance.py --list
python3 run_acceptance.py --strict         # BLOCKED cũng tính là lỗi: dùng cho phiên nghiệm thu chính thức
python3 report_orphan_requests.py          # liệt kê Request mở mà project đã bị xoá (chỉ đọc); --cancel để huỷ có lý do
```

Mã thoát 1 nếu có `FAIL` (hoặc `BLOCKED` khi `--strict`). Kết quả ghi vào `reports/last-run.{json,md}` (thư mục này bị git bỏ qua).

## Điều kiện chạy

| Cần | Cách có |
|---|---|
| Stack `deploy/dev` đang chạy | xem `deploy/dev/README.md` |
| Tài khoản admin | đọc từ `deploy/dev/.env` (`BOOTSTRAP_ADMIN_*`), hoặc `ORCA_E2E_ADMIN_EMAIL/PASSWORD`. Mật khẩu không bao giờ được in ra |
| Dev server (agent) đã kết nối và được duyệt | tên mặc định `dev-local-mac` (`ORCA_E2E_DEV_SERVER_NAME`). Thiếu thì các test liên quan là `BLOCKED` |
| Repo git trên dev server | tự tạo repo tạm; nếu dev server là máy khác, đặt `ORCA_E2E_REPO_PATH` |
| `docker` CLI (tuỳ chọn) | để kiểm tra container và đọc kết quả từng step workflow; thiếu thì test đó là `BLOCKED` |
| API key AI cho agent (tuỳ chọn) | `ANTHROPIC_API_KEY` trong môi trường của agent; thiếu thì các test dùng AI là `BLOCKED` |
| `ORCA_E2E_EXECUTE_AGENT=1` (tuỳ chọn) | bật test chạy agent thật (`task.execute`); mặc định `SKIP` vì tốn lượt dùng AI |

## Bốn trạng thái kết quả

| Trạng thái | Nghĩa |
|---|---|
| `PASS` | hành vi đúng tiêu chí nghiệm thu |
| `FAIL` | sản phẩm sai (hoặc một điều kiện mà chính sản phẩm phải đảm bảo bị hỏng) |
| `BLOCKED` | môi trường thiếu thứ test cần (API key, agent, docker, MCP bật). **Không** nói gì về chất lượng sản phẩm |
| `SKIP` | test tuỳ chọn chưa được bật |

Tách `BLOCKED` khỏi `FAIL` để báo cáo không lẫn lỗi sản phẩm với việc thiếu môi trường, và để phiên `--strict` buộc phải dựng đủ môi trường trước khi ký nghiệm thu.

## Nguyên tắc thiết kế

- **Oracle độc lập với mã.** `expected_request_flows.py` ghi lại bảng 11 loại Request từ README v6 mục 3.4 và CR-REQ-003; test so sánh với đáp ứng của `request.flow`, không sinh kỳ vọng từ chính registry Go.
- **Đi qua cổng công khai.** Mọi bước trả lời Clarification, xác nhận loại, huỷ… đều qua kênh WS. Nhờ vậy test bắt được cả lỗi "RPC có nhưng gateway không mở" (xem lỗi D1 trong báo cáo).
- **Tự dọn.** Dữ liệu tạo ra mang tiền tố `e2e-<mmddhhmmss>`; Request được huỷ, task và project được xoá ở cuối. Template workflow không có API xoá nên vẫn còn (tên có tiền tố để dễ nhận).
- **Giãn nhịp ghi.** `request-service` giới hạn ghi 5 lần/giây (burst 10). Client tự thử lại khi gặp `REQUEST_RATE_LIMITED`; riêng `RQL-20` tắt thử lại để chứng minh bộ giới hạn hoạt động. Số lần bị giới hạn được in cuối mỗi lượt.

## Danh mục test

| Suite | ID | Kiểm tra |
|---|---|---|
| `platform` | PLT-01..11 | UI phục vụ được, service worker, bundle JS; đăng nhập đúng/sai; `/auth/me`; WS ẩn danh bị chặn; container chạy, không crash-loop; gateway tới được `request-service` và `workflow-service` |
| `devserver` | DEV-01..07 | dev server đăng ký, được duyệt, đang kết nối; tạo project gắn dev server (agent kiểm tra đường dẫn repo); từ chối đường dẫn không tồn tại; từ chối dev server lạ |
| `workflow` | WFL-01..13 | DAG tuyến tính chạy hết; thứ tự wave; fan-out/fan-in; điều kiện sai; biểu thức hỏng; chu trình, phụ thuộc thiếu, id trùng không bao giờ `completed`; trạng thái cuối bất biến; bước `shell` chạy trên dev server (đích `server:` và `project:`) |
| `request` | RQL-01..20 (không có 18) | cờ luồng bật/tắt và lối thoát an toàn khi tắt; tạo Request, idempotency, phân loại luôn kết thúc ở `awaiting_type_confirmation`; AI đề xuất loại; xác nhận loại, version cũ, đồng thời; Clarification (liệt kê, nháp, hoàn tất, không ghi đè); lọc danh sách; giới hạn tốc độ |
| `matrix` | MTX-01..13 | mỗi trong 11 loại: trạng thái đầu sau khi xác nhận và đường đi/cổng/Phase đúng bảng; đúng 11 loại; loại câu hỏi khớp điều kiện hợp lệ |
| `exits` | EXT-01..11 | trả backlog (cần lý do), mở lại, huỷ (cần lý do), trạng thái cuối bất biến, đổi loại (chỉ được mở rộng), Request con, id lạ |
| `ai` | AIS-01..06 | khi không có AI, sinh Solution/Plan thất bại an toàn (run `failed` có mã lỗi, Request đứng yên); khi có AI, sinh được Solution/Plan; hộp thư duyệt và backlog trả lời |
| `plan` | PLN-01..08 | giai đoạn Plan với AI thật: đề xuất chạy nền và không ghi gì; đề xuất hợp lệ; vòng phụ thuộc bị từ chối; commit ghi Plan, chuyển `awaiting_plan_approval` và mở Approval `task_list`; commit lặp lại không nhân đôi; từ chối Approval về backlog; sai trạng thái; `change_request` đi theo Phase từ Solution đã duyệt. Không bao giờ duyệt Plan (sẽ chạy agent AI trên máy dev) |
| `notfound` | NFC-01..08 | id lạ là NotFound ở task, project (kể cả id sai định dạng), workflow, request, clarification, approval |
| `task` | TSK-01..10, MCP-01..02 | CRUD task, task con và cây con, không đặt được `in_progress` bằng update, xoá task lạ là `NOT_FOUND`, `task.execute` (tuỳ chọn); MCP công bố metadata và từ chối ẩn danh |

## Thêm test mới

Một test là hàm không tham số; ném `AssertionError` khi sai, `Blocked("lý do")` khi thiếu môi trường, `Skipped("lý do")` khi chưa bật. Đăng ký bằng `rec.run("ID", "Tiêu đề", fn)` trong file suite. Dùng `ctx.on_cleanup(...)` cho mọi dữ liệu tạo ra. Đặt tên file theo nghiệp vụ, không dùng tên chung chung.
