# BE-CV-TASK-035-04: Gói `secretmasking`: đường dẫn bị cấm, chính sách giá trị biến môi trường, che URL kết nối

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/secretmasking/sensitive_source_paths.go`, `environment_value_policy.go`, `connection_url_masking.go` và `_test.go` (mới)
**Depends on:** BE-CV-SOL-010
**Status:** [x] DONE

---

## Context

Đây là phần dễ sai nhất của series (README v7: nơi dễ lộ secret nhất). Solution 035 §2.E đặt thứ tự quyết định và sửa ba điểm của CR (C3 mâu thuẫn `DSN`, C4 `${…}` trong userinfo, C5 entropy che nhầm địa chỉ). Gói này là **hàm thuần**, không I/O, `domain/` chỉ stdlib (`net/url`, `regexp`, `strings`). Tên gói `secretmasking` theo gợi ý O-16 của hợp đồng (không `utils`).

## Việc cần làm

1. `sensitive_source_paths.go`: `IsForbiddenSourcePath(rel string) bool`. Cấm: `**/.env`, `**/.env.*` ngoại trừ đuôi `.env.example` (trả kèm `NamesOnly=true` để caller chỉ lấy tên khoá), `*.pem`, `*.key`, `*.p12`, tiền tố `/vault/secrets/`, tên `orca-policy.hcl`, đường dẫn tuyệt đối, chứa `..`, chứa `\`. So khớp không phân biệt hoa thường; chuẩn hoá NFKC trước khi so (cùng tinh thần `CODEINTEL_PATH_NOT_ALLOWED`).
2. `connection_url_masking.go`: `MaskConnectionValue(raw string) (MaskedURL, bool)` với `MaskedURL{Scheme, HostClass, Port, DBName}`: (a) thay mọi `${…}` bằng token cố định `x`; (b) `url.Parse`; lỗi ⇒ `false` (bỏ cả giá trị); (c) bỏ userinfo, query, fragment; (d) `HostClass`: `"external"` nếu host là IP literal hoặc chứa `.` và không thuộc tập tên dịch vụ compose do caller truyền vào (tham số `composeHosts map[string]struct{}`), ngược lại `"compose:<tên>"`; **không bao giờ** trả host của `external`; (e) `DBName` = phần path bỏ `/` đầu, chỉ khi khớp `^[a-z0-9_]{1,63}$`. Hỗ trợ scheme `postgresql`, `postgres`, `mysql`, `tidb`, `nats`, `http`, `https`; scheme lạ ⇒ `false`.
3. `environment_value_policy.go`: `Decide(key, value string, rule KeyRules) Decision` với `Decision{Action: Keep|NameOnly|SecretRef|Structured|Redact, Value string, Masked *MaskedURL}` theo đúng thứ tự: (a) `${…}` ⇒ `NameOnly` (bỏ mặc định/thông báo); (b) khoá thuộc `urlKeys` ⇒ `Structured` qua mục 2; (c) khoá khớp `(?i)(SECRET|TOKEN|PASSWORD|PASSWD|PASSPHRASE|PRIVATE|CREDENTIAL|API_?KEY|AUTH|SALT|SIGNING|CURSOR_KEY|DSN)` ⇒ `SecretRef`; (d) khoá thuộc allowlist không nhạy cảm **và** giá trị khớp văn phạm riêng của khoá ⇒ `Keep`; (e) còn lại ⇒ `Redact` (và caller tăng `redacted_count`). `KeyRules` là cấu hình bất biến: `urlKeys` (`DATABASE_DSN`, `NATS_URL`, `VAULT_ADDR`, `*_ADDR` dạng `host:port`), allowlist + văn phạm (`*_PORT` số 1–65535; `*_ENABLED` bool; `ORCA_SERVER_DEPLOYMENT` bool; `FLEET_POLL_INTERVAL_SEC` số; `EPHEMERAL_VM_SSH_MODE` `^[a-z-]{1,32}$`; `OPA_BUNDLE_PATH` `^[A-Za-z0-9._/-]{1,200}$`).
4. Thứ tự (b) trước (c) là cố ý (C3): ghi bằng comment hai dòng ở đầu `Decide`.
5. Không có "chế độ hiện secret": không tham số `reveal`, không biến môi trường mở (kiểm bằng test phản chiếu chữ ký).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/secretmasking/...`: bảng ≥ 40 ca: DSN thật dạng `postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/infra?sslmode=disable` ⇒ `DBName=infra`, không còn `orca`/`POSTGRES_PASSWORD`; `${VAR:-x}` và `${VAR:?thông báo nội bộ}` ⇒ chỉ tên; `VAULT_ADDR: http://172.20.2.21:8200` ⇒ `external`, không có `172.20`; khoá `MCP_CURSOR_KEY`, `OAUTH_STATE_SECRET`, `APNS_KEY_ID` ⇒ `SecretRef`; `INFRA_FLEET_SERVICE_ADDR=infra-fleet-service:9090` ⇒ `Keep` (không bị che bởi entropy); DSN hỏng ⇒ bỏ; scheme lạ ⇒ bỏ.
- Fuzz: `go test -fuzz=FuzzDecide -fuzztime=30s` với corpus canary `CANARY_*`: không bao giờ `Keep`/`Structured` chứa canary. (Chạy ở CI nightly; PR chạy 10 s.)
- Ca đường dẫn: `deploy/dev/.env`, `.ENV`, `a/.env.local`, `..%2f.env` (sau decode), `deploy/dev/.env.example` (NamesOnly).

## Tiêu chí hoàn thành

- [x] Mọi ca ở bảng và fuzz xanh.
- [x] `Decide` không có nhánh nào trả nguyên giá trị cho khoá không thuộc allowlist.
- [x] Hai literal thật của `backend-go/docker-compose.yml` (bản fixture CANARY ở TASK-035-01) cho `SecretRef`.
- [x] Không phụ thuộc ngoài stdlib.

## Rủi ro và lưu ý

- Giả định `net/url` từ chối `{`/`}` trong userinfo chưa được chạy; bước thay `${…}` làm cho kết quả không phụ thuộc giả định đó.
- `HostClass` dựa vào `composeHosts` từ compose đang parse: nếu compose parse lỗi thì mọi host coi là `external` (an toàn hơn).
- Văn phạm theo khoá cần mở rộng khi service thêm khoá mới; khoá chưa biết mặc định `Redact` (cố ý, lắm khi gây `redacted_count` cao; báo ở UI).
