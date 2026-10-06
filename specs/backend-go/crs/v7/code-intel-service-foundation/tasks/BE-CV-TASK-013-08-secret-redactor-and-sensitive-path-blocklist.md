# BE-CV-TASK-013-08: Bộ che bí mật và danh sách đường dẫn nhạy cảm

**From Solution:** BE-CV-SOL-013-agent-call-gate-and-quotas
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/{secret_redactor.go,sensitive_path.go}` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-010-03
**Status:** [ ] TODO

---

## Context

Mẫu `mcp-service/internal/domain/secret_redactor.go` (9 regex, `[REDACTED]`, `Redact(s) (string, bool)`; mẫu cuối giữ nhóm 1). Viết lại, không import chéo service. Danh sách chặn nội dung ở CR-CV-013 mục 2.5 (`**/.env`, `**/.env.*`, `**/*.pem`, `**/*.key`, `**/*.p12`, `**/*.pfx`, `**/*.kdbx`, `**/id_rsa*`, `**/id_ed25519*`, `**/*.tfstate`, `**/*.tfvars`, `**/credentials*`, `**/.npmrc`, `**/.netrc`, `**/secrets/**`, `**/.aws/**`, `**/.ssh/**`). Agent là tuyến đầu tôn trọng `.gitignore`; đây là lớp phòng thủ thứ hai.

## Việc cần làm

1. `SecretRedactor.Redact(s string) (string, bool)` (giữ đúng 9 mẫu), `RedactWithCount(s) (string, int)` đếm số lần thay.
2. `IsSensitivePath(rel string) bool`: chuẩn hoá `\`→`/`, hạ chữ thường, bỏ `./` đầu; khớp bằng so sánh đoạn (không dùng `path.Match` cho `**`; viết hàm khớp theo đoạn/hậu tố); không có ngoại lệ thư mục test.
3. `WithholdSource(rel, content string) (content string, withheld string, redactions int, err error)`: đường dẫn nhạy cảm → `"", "sensitive_path", 0, nil`; `len(content) > 200<<10` → `CODEINTEL_OUTPUT_TOO_LARGE` (`FailedPrecondition`); còn lại `Redact`.
4. Hằng `MaxSourceContentBytes = 200 << 10` (ghi riêng khỏi trần response 320 KiB).
5. Chú thích ngắn nêu lý do bảo thủ (dương tính giả chỉ hiện `[REDACTED]`).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/... -run 'Redact|SensitivePath|Withhold'`
- Bảng: token `ghp_…`, `github_pat_…`, `AKIA…`, `Bearer …`, `sk-…`, `xox…`, JWT, khối `-----BEGIN … PRIVATE KEY-----`, `password=abc123`, chuỗi sạch; đường dẫn (`.env`, `a/.ENV.local`, `x/secrets/y.yaml`, `.AWS/credentials`, `C:\p\id_rsa.pub` dạng `\`, `docs/env.md` không khớp); 200 KiB + 1 byte.

## Tiêu chí hoàn thành

- [ ] Tất cả mẫu bị che; không dương tính giả trên chuỗi sạch của bảng.
- [ ] Đường dẫn nhạy cảm cho `content=""` không lỗi; quá 200 KiB bị từ chối.

## Rủi ro và lưu ý

- Regex không đầy đủ; không phải bảo đảm tuyệt đối.
- Nếu O-16 chọn gói dùng chung, chuyển file sang gói đặt tên cụ thể (ví dụ `secretmasking`) trong PR riêng.
