# SOL-E2E-004: Triển khai tự kiểm tra: cấu hình bắt buộc, sinh bí mật, sẵn sàng thật sự, build không phá mount

**Resolves:** BUG-E2E-002, BUG-E2E-005 · **Nguyên nhân gốc:** RC-D
**Service:** `deploy/dev`, `request-service`, các service dùng `internalcaller` và OPA
**Tasks:** TASK-E2E-018 đến 022

## Vấn đề

Cấu hình nội bộ (token, mount OPA) rơi vào im lặng: thiếu thì service chạy nhưng từ chối mọi thứ với lỗi trông như phân quyền. Sau mỗi lần build, mount thư mục bị vô hiệu và hệ thống lại "bị phân quyền sai" cho tới khi restart.

## Thiết kế

### 1. Khai báo bắt buộc, sinh tự động

- Trong `deploy/dev/docker-compose.yml`, token liên quan `request-service` là bắt buộc khi cờ luồng bật: `${REQUEST_INTERNAL_CALLER_TOKEN:?…}` ở gateway và `request-service`, cùng giá trị. Khi `REQUEST_FLOW_ENABLED=false` không bắt buộc, để không chặn môi trường chưa dùng.
- Script `deploy/dev/scripts/ensure-request-env.sh` theo mẫu `ensure-mcp-env.sh`: chỉ điền khoá **thiếu hoặc rỗng**, sinh ngẫu nhiên trên chính máy chạy, không in giá trị. `sync-to-server.sh` gọi nó trước `up`.
- Mount OPA và `OPA_BUNDLE_PATH` của `request-service` nằm trong compose (đã thêm); thêm vào `backend-go/ci/check-opa-bundle-in-images.sh` hoặc kiểm tra compose tương ứng để service nào dùng OPA mà thiếu mount bị bắt.

### 2. Sẵn sàng thật sự

`request-service` `/readyz` trả không-sẵn-sàng khi: bundle OPA chưa nạp được, `GATEWAY_INTERNAL_TOKEN` hoặc `SERVICE_INTERNAL_TOKEN` rỗng, hoặc DB không tới được. Compose thêm `healthcheck` dùng nó. Mục đích: lỗi cấu hình **hiện thành container không healthy**, không thành `REQUEST_FORBIDDEN` mơ hồ.

Rà các service khác dùng `internalcaller`/OPA để chắc chắn token rỗng luôn **từ chối** (fail-closed) và không có nhánh nào coi rỗng là "tắt guard". Kết quả rà ghi trong task.

### 3. Build không phá mount; quy trình triển khai cục bộ rõ ràng

- `build-local.sh` dùng `rsync -a --delete` cho `dist/` và `policy/` (đã làm), giữ inode.
- Mount theo **file nhị phân** vẫn cần restart để nạp bản mới: thêm `deploy/dev/scripts/redeploy-local.sh <service…>` = build service đó, `docker compose up -d`, `docker restart`, rồi chạy `platform` của `orca-flows`. Hoạt động trên macOS và Linux.
- Tài liệu hoá overlay `docker-compose.local.yml` (Vault dev trong bộ nhớ) và biến `DOCKER_HOST` cho testcontainers dưới Colima.

### 4. Cổng kiểm tra sau triển khai

Suite `platform` của `orca-flows` (`PLT-01..11`, 3 giây) là bài smoke bắt buộc sau mỗi lần dựng: nó đã bắt đúng bốn lỗi của nhóm này.

## Tương thích

Không đổi mã nghiệp vụ. Môi trường hiện có chưa có token sẽ thấy compose báo lỗi rõ khi bật cờ: đó là mong muốn. Chạy `ensure-request-env.sh` một lần để vá.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| `healthcheck` mới làm `depends_on: service_healthy` chậm khởi động | chỉ đặt `start_period` đủ; không đặt `depends_on` healthy cho service khác ở bước này |
| `rsync` không có trên Windows | script bash dành cho macOS/Linux/Git-Bash; tài liệu nêu rõ; thay thế `robocopy` không nằm trong phạm vi |

## Tiêu chí hoàn thành

- Dựng stack từ `.env` trống (sau `ensure-request-env.sh`) rồi `PLT-10` xanh, không phải sửa tay.
- Tắt token thì container `request` không healthy trong 60 giây.
- Chạy `build-local.sh` khi stack đang chạy: UI và `PLT-10` vẫn xanh **không restart** (đã xác nhận một lần bằng tay).
