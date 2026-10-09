# SOL-E2E-005: `ai.complete` dùng phiên đăng nhập CLI của dev server; kiểm tra sẵn sàng trước khi chạy; lỗi có hướng khắc phục

**Resolves:** BUG-E2E-010 · **Nguyên nhân gốc:** RC-E
**Service:** `agent`, `request-service`, `infra-fleet-service`; (tuỳ chọn) `ai-provider-service`, `credential-broker-service`
**Tasks:** TASK-E2E-023 đến 026, 033 đến 035
**Quyết định đã chốt (2026-10-09):** dùng **phiên đăng nhập của dev server** (Claude Code CLI đã đăng nhập) thay cho khoá API làm đường mặc định.

## Vấn đề

`ai.complete` của agent gọi thẳng HTTP `api.anthropic.com` bằng `x-api-key`; nó không dùng CLI. Khoá chỉ đến từ biến môi trường của agent hoặc `resolvedApiKey` do server gửi, và không có đoạn mã phía server nào điền `resolvedApiKey`. Người dùng đã đăng nhập Claude Code trên dev server vẫn bị đòi khoá, và thông báo lỗi chỉ là "classifier error".

## Thiết kế

### 1. Ba nguồn xác thực cho `ai.complete`, theo thứ tự

| # | Nguồn | Khi nào dùng | Ghi chú |
|---|---|---|---|
| 1 | Biến môi trường của agent (`ANTHROPIC_API_KEY`, …) | môi trường tự đặt khoá | giữ nguyên hành vi hiện tại |
| 2 | `resolvedApiKey` do server gửi | đội ngũ muốn khoá quản trị tập trung | **tuỳ chọn**, TASK-E2E-023 (P2) |
| 3 | **Phiên đăng nhập CLI trên dev server** | mặc định khi không có 1 và 2 | **mới**, TASK-E2E-033 |

Mọi lời gọi `ai.complete` đi qua một hàm chọn nguồn duy nhất (`resolveAiSource`) trong agent, để thứ tự nằm ở một chỗ và test được.

### 2. Nguồn 3: chạy `claude` ở chế độ không công cụ, chỉ để sinh văn bản

Agent thêm một provider `ai-complete-cli-session.ts` (tên theo nghiệp vụ). Lệnh (đã đối chiếu với `claude --help` bản 2.1.266 trên máy phát triển):

```
claude -p --output-format json --tools "" --no-session-persistence \
       --system-prompt "<chỉ dẫn: chỉ trả lời đúng định dạng yêu cầu>" [--model <alias>]
```
Prompt gửi qua **stdin**, không qua đối số dòng lệnh (tránh lộ trong danh sách tiến trình và giới hạn độ dài).

Điểm bắt buộc và lý do:

| Quy tắc | Lý do |
|---|---|
| `--tools ""` | tắt mọi công cụ: lời gọi chỉ sinh văn bản nên **không thể đọc/ghi tệp hay chạy lệnh** dù prompt chứa chỉ dẫn độc hại (nội dung Request là dữ liệu không tin cậy) |
| **không dùng `--bare`** | theo `--help`, chế độ bare "không bao giờ đọc OAuth và keychain", tức sẽ làm mất đăng nhập mà ta cần |
| `--no-session-persistence` | không để lại phiên/lịch sử trên dev server cho mỗi lần phân loại |
| `cwd` là thư mục tạm rỗng | tránh nạp `CLAUDE.md`, hook, cấu hình dự án của repo thật; không lộ mã nguồn |
| môi trường tiến trình tối giản | chỉ truyền `HOME`, `PATH`, biến proxy; **không** truyền biến bí mật của agent |
| thời gian chờ và giới hạn đầu ra | huỷ tiến trình sau `timeoutMs` (mặc định 120 giây); cắt đầu ra quá lớn |
| tối đa 2 tiến trình đồng thời | tránh hết tài nguyên máy dev và vượt hạn mức của gói |

Ánh xạ `model` của `ai.complete` sang bí danh mà CLI hiểu; model lạ → bỏ `--model`, dùng mặc định của CLI. Sản phẩm trả về `{content, usage}`: lấy `usage` và chi phí từ JSON của CLI nếu có để ghi vào `usage-service` như đường API.

### 3. Phát hiện và báo cáo khả năng

Agent thăm dò `claude auth status --json` (kết quả có `loggedIn`, `authMethod`), lưu cache 60 giây, và công bố trong `agent.capabilities` một trường `ai.sources` ví dụ `["env_key","cli_session"]`. Không bao giờ in nội dung xác thực; chỉ công bố "có/không".

Công tắc tắt: `ORCA_AI_CLI_SESSION=off` trên agent (theo mẫu `ORCA_CODEINTEL_DISABLED`). Mặc định bật theo quyết định.

### 4. `request-service` kiểm tra sẵn sàng bằng khả năng thật

Trước bước AI, hỏi `infra-fleet` khả năng của dev server (`GetDevServerCapabilities`, CR-REQ-033). Lỗi có cấu trúc thay cho "classifier error":

| Mã | Khi | Hiển thị gợi ý |
|---|---|---|
| `REQUEST_AI_NOT_CONFIGURED` | `ai.sources` rỗng và không có tài khoản provider | "Dev server chưa đăng nhập Claude Code. Chạy `claude auth login` trên dev server, hoặc đặt khoá API" |
| `REQUEST_AI_AGENT_UNAVAILABLE` | không có dev server/agent kết nối | "Kết nối dev server" |
| `REQUEST_AI_CREDENTIAL_UNAVAILABLE` | chỉ khi dùng nguồn 2 mà đọc khoá lỗi | "Kiểm tra credential" |
| `REQUEST_AI_SESSION_EXPIRED` | CLI báo hết phiên khi đang chạy | "Đăng nhập lại Claude Code trên dev server" |

Run phân loại/sinh được ghi lý do bằng mã này. Giữ nguyên thoái hoá an toàn: Request vẫn tới `awaiting_type_confirmation`, run `failed` có mã lỗi.

### 5. Hệ quả cần chấp nhận và ghi rõ

- **Hạn mức thuộc về tài khoản đăng nhập trên dev server**, không theo từng người dùng ORCA. Mọi người dùng chung dev server tiêu thụ cùng một gói. `usage-service` ghi theo người dùng ORCA gọi, tài khoản bị tính phí là của dev server (ghi nhãn `source=cli_session`).
- **Điều khoản dịch vụ:** dùng một phiên đăng nhập cá nhân cho nhiều người dùng trên máy chung, hoặc chạy tự động hoá bằng gói thuê bao, có thể bị hạn chế bởi điều khoản nhà cung cấp. Quyết định này do chủ sản phẩm chốt; khuyến nghị xác nhận điều khoản trước khi bật trên dev server dùng chung nhiều người. Không nằm trong phạm vi kỹ thuật của solution.
- Độ trễ cao hơn gọi API trực tiếp (khởi động tiến trình). Phân loại có thời gian chờ 60 giây (`defaultClassifierTimeout`) nên cần đo; nếu vượt, tăng timeout hoặc giữ tiến trình nóng (ngoài phạm vi).

### 6. Nguồn 2 trở thành tuỳ chọn

Việc phân giải khoá phía server (`aiprovider.ResolveProvider` + `credentialbroker.ResolveCredential` → `resolvedApiKey`) giữ lại ở mức P2 cho đội ngũ muốn khoá tập trung hoặc dev server không đăng nhập CLI. Thiết kế giữ nguyên như bản trước, chỉ đổi mức ưu tiên và không còn là điều kiện để thông luồng.

### 7. Kiểm thử không cần tài khoản thật

- Một tập lệnh giả `claude` (stub) đặt trong `PATH` của agent khi test: nhận cờ như trên, đọc stdin, in JSON xác định cho phân loại/Solution/Plan, trả mã thoát để mô phỏng "chưa đăng nhập". Nhờ đó CI chạy nguồn 3 mà không cần đăng nhập.
- Biến `ORCA_AI_ANTHROPIC_BASE_URL` (chỉ để kiểm thử) cho phép trỏ nguồn 1/2 tới stub HTTP.

## Tương thích

Không đổi proto. Agent cũ không có nguồn 3 sẽ không công bố `ai.sources`; `request-service` xử lý thiếu trường như "không biết" và vẫn thử gọi (hành vi hiện tại), tránh chặn nhầm.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Prompt độc hại trong nội dung Request điều khiển CLI | `--tools ""`, cwd tạm, môi trường tối giản: CLI không có khả năng hành động ngoài trả văn bản |
| Phiên hết hạn giữa chừng | nhận biết lỗi xác thực, trả `REQUEST_AI_SESSION_EXPIRED`, cache khả năng 60 giây |
| Quá tải tiến trình | giới hạn 2 đồng thời + hàng đợi; từ chối nhanh khi đầy |
| CLI đổi cờ ở bản mới | kiểm tra cờ khi khởi động agent (`claude --help` chứa `--tools`, `--no-session-persistence`); thiếu thì tắt nguồn 3 và báo `ai.sources` không có nó |
| Hạn mức dùng chung cạn | mã `REQUEST_AI_QUOTA_EXHAUSTED` khi CLI báo, hiển thị rõ |

## Tiêu chí hoàn thành

- Trên dev server **không có** khoá API nhưng đã `claude auth login`: `request.classify` cho đề xuất loại/size/urgency thật (kiểm bằng tay một lần với tài khoản thật, và bằng stub trong CI).
- Chưa đăng nhập: thông báo là `REQUEST_AI_NOT_CONFIGURED` kèm hướng dẫn, không phải "classifier error".
- Không có công cụ nào chạy trong lời gọi: test chứng minh stub nhận `--tools ""` và cwd rỗng.
- `RQL-09`, `AIS-02`, `AIS-04` chuyển BLOCKED → PASS với stub; `--strict` xanh.
- Khảo sát log và tiến trình: không có nội dung prompt hay xác thực trong log/argv.
