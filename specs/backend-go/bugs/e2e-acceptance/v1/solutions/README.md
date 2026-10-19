# Solutions — Bug nghiệm thu e2e (v1)

> **Quyết định đã chốt (2026-10-09):** (1) `ai.complete` dùng **phiên đăng nhập của dev server** thay cho khoá API làm đường mặc định ([SOL-E2E-005](./SOL-E2E-005-ai-credential-path.md)); (2) thêm RPC nội bộ `ProjectService.ResolveProjectAccess` ([SOL-E2E-003](./SOL-E2E-003-project-access-and-error-taxonomy.md)).

Sáu solution, mỗi solution xử lý **một nguyên nhân gốc** và mọi bug thuộc nó (xem [IMPACT-ASSESSMENT](../IMPACT-ASSESSMENT.md) mục 4). Mục tiêu chung: **thông luồng Request/Workflow/Task trên dev server thật và giữ ổn định**, không phá vỡ kiến trúc hiện có.

## Ràng buộc kiến trúc (áp dụng cho mọi solution)

| # | Ràng buộc | Hệ quả khi thiết kế |
|---|---|---|
| G1 | Hexagonal `internal/{domain,usecase,adapter,config}`; quy tắc nghiệp vụ nằm ở `domain`/`usecase` của service sở hữu | Gateway chỉ ánh xạ hình dạng dữ liệu, gắn `Identity`, đặt timeout, ánh xạ lỗi (CONTRACT C10). Không đưa quy tắc vào gateway hay agent |
| G2 | Proto-first, chỉ thay đổi **cộng thêm** (RPC, trường mới); `buf lint` và `buf breaking` phải xanh | Không đổi số trường, không đổi nghĩa trường đã có. Thêm trường cho client cũ phải an toàn khi thiếu |
| G3 | `tenantId`/`userId` chỉ lấy từ `Identity`/metadata đã xác thực, không nhận từ tham số kênh | Server tự suy ra người tạo, không tin trường do client gửi (trừ gọi nội bộ có token) |
| G4 | RPC chỉ dành cho service nội bộ đặt sau `internalcaller` (token chung) | Thiếu token là **fail-closed**, không bao giờ "rỗng thì bỏ kiểm tra" |
| G5 | Service có hai dialect (Postgres và MySQL) thì mọi migration/SQL mới có cả hai bản và cùng test | Áp dụng cho `task`, `request`, `workflow`, `infra-fleet`, `notification` |
| G6 | Mọi thay đổi phải tính đến SSH: dev server có thể ở máy khác | Không giả định hệ tệp cục bộ; việc đụng đường dẫn đi qua agent |
| G7 | Kênh gateway mới phải có `ToolSpec` MCP hoặc dòng trong `excluded_channels.yaml` (có lý do) | `parity_test.go` bắt buộc |
| G8 | Khoá, token, giá trị bí mật không vào log, không vào tham số kênh có thể được ghi lại (`channel_args_redaction`) | Áp dụng cho `resolvedApiKey` |
| G9 | AGENTS.md: comment nêu "vì sao", ngắn; không dùng tên tệp chung chung (`helpers`, `utils`, `common`); không thêm `max-lines` disable; script phải chạy được trên macOS, Linux, Windows | Đặt tên theo nghiệp vụ |
| G10 | Trước khi sửa một symbol: chạy `gitnexus impact`; trước khi commit: `detect_changes` | Ghi kết quả vào mô tả PR |

## Danh mục

| Solution | Nguyên nhân gốc | Bug | Tóm tắt | Ưu tiên | Tasks |
|---|---|---|---|---|---|
| [SOL-E2E-001](./SOL-E2E-001-contract-parity-guard.md) | RC-A | 001, 004 | ba kiểm tra tĩnh khoá hợp đồng service ↔ gateway ↔ agent ↔ frontend | P0 | 001–005 |
| [SOL-E2E-002](./SOL-E2E-002-identity-propagation.md) | RC-B | 003, 006, 008 | interceptor truyền danh tính một chỗ; tác vụ nền giữ danh tính; vá dữ liệu | P0 | 006–012 |
| [SOL-E2E-003](./SOL-E2E-003-project-access-and-error-taxonomy.md) | RC-C | 007, 009, OBS-A | RPC `ResolveProjectAccess`; quy ước lỗi "không có" | P1 | 013–017 |
| [SOL-E2E-004](./SOL-E2E-004-deployment-wiring-and-readiness.md) | RC-D | 002, 005 | cấu hình bắt buộc, sinh bí mật, `/readyz` thật, build không phá mount | P0 | 018–022 |
| [SOL-E2E-005](./SOL-E2E-005-ai-credential-path.md) | RC-E | 010 | `ai.complete` dùng phiên đăng nhập CLI của dev server (đã chốt), kiểm tra sẵn sàng, lỗi có hướng khắc phục; khoá phía server là tuỳ chọn | P0 | 023–026, 033–035 |
| [SOL-E2E-006](./SOL-E2E-006-tests-docs-and-tuning.md) | RC-F | OBS-B, C, D | e2e vào CI, giới hạn ghi cấu hình được, `value_type`, tài liệu | P2 | 027–032 |

## Vì sao gom theo nguyên nhân gốc

| Cách vá từng bug | Cách vá theo gốc |
|---|---|
| thêm `withTenantMetadata` vào mỗi adapter bị lỗi | một interceptor, một script lint cấm dial thủ công |
| thêm kênh/method khi có người phát hiện thiếu | ba kiểm tra tĩnh bắt thiếu ngay lúc thêm RPC |
| sửa `GetProject` cho từng nơi cần | một RPC trả `exists` và `role` cùng lúc, dùng ở mọi nơi |
| nhắc nhớ restart sau build | build không phá mount; `/readyz` biến lỗi cấu hình thành lỗi nhìn thấy |
| yêu cầu người vận hành tự đặt khoá cho agent | agent dùng phiên đăng nhập CLI của dev server, có thông báo khi chưa đăng nhập |

## Thứ tự áp dụng

```
Giai đoạn 0  Chốt các bản vá đang chờ commit: TASK 001, 004, 007(một phần), 011, 018, 021, 027
Giai đoạn 1  Lan can (không đổi hành vi): SOL-001 (002, 003, 005) ; SOL-004 (019, 020, 022)
Giai đoạn 2  Sửa theo gốc: SOL-002 (006 → 007/008/009 → 010, 011 → 012) ; SOL-003 (013 → 014 → 015 → 016, 017)
Giai đoạn 3  Thông luồng AI: SOL-005 (033 → 034 → 024 → 035 → 026; 025 song song; 023 tuỳ chọn)
Giai đoạn 4  Ổn định: SOL-006 (028, 029, 030, 031, 032)
```

Giai đoạn 1 đặt trước giai đoạn 2 có chủ đích: lan can được viết khi lỗi còn tái hiện được, nên chứng minh được chúng bắt đúng lỗi.

## Quan hệ với frontend

Solution phía frontend ở [`specs/frontend/bugs/e2e-acceptance-v1`](../../../../../frontend/bugs/e2e-acceptance-v1/solutions/README.md). Hai phía chia sẻ: tệp vàng kênh (TASK-E2E-003 ↔ TASK-FE-E2E-001), trường `value_type` (TASK-E2E-032 ↔ TASK-FE-E2E-003), mã lỗi mới (TASK-E2E-015, 024, 031 ↔ TASK-FE-E2E-004, 005).
