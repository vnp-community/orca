# BE-CV-TASK-035-05: Bộ quét cuối `storage_map_leak_scan` và lỗi `CODEINTEL_SECRET_LEAK_BLOCKED`

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/secretmasking/token_shape_detection.go`, `storage_map_leak_scan.go` và `_test.go` (mới); `backend-go/services/code-intel-service/internal/domain/secretmasking/leak_blocked_error.go` (mới)
**Depends on:** BE-CV-TASK-035-04; BE-CV-SOL-010 (`apperrors` Kind mới)
**Status:** [ ] TODO

---

## Context

Lớp phòng thủ thứ hai: dù parser đã che, mọi payload `StorageMap` **đã serialize** phải qua bộ quét trước khi ghi cache hoặc trả RPC. Quét cuối là fail-closed: thấy gì nghi ngờ thì **không trả view** (không trả "một phần"). Số đo ở solution 035 C5 cho thấy entropy toàn chuỗi che nhầm; vì vậy entropy chỉ chạy **theo token** kèm điều kiện ký tự.

## Việc cần làm

1. `token_shape_detection.go`: `LooksLikeSecretToken(tok string) bool` đúng khi (i) có tiền tố đã biết `hvs.`, `s.` (chỉ khi token còn lại ≥ 20 ký tự), `ghp_`, `gho_`, `glpat-`, `xoxb-`/`xoxp-`, `sk-`, `AKIA`; hoặc (ii) dài ≥ 20, toàn ký tự `[A-Za-z0-9+/=_-]`, có đủ chữ hoa + chữ thường + chữ số, entropy Shannon ≥ 4,0 bit/ký tự. Hằng số ngưỡng đặt tên (`minTokenLength`, `minTokenEntropy`) kèm comment: "giá trị khởi điểm chưa hiệu chỉnh".
2. `storage_map_leak_scan.go`: `Scan(payload []byte) []Finding` (`Finding{Kind, Offset}` **không** chứa chuỗi khớp). Kiểm: (a) userinfo trong URL, regex `://[^/@\s]+:[^/@\s]+@`; (b) tách token theo `/ : @ . , ; =`, dấu ngoặc, khoảng trắng và gọi `LooksLikeSecretToken`; (c) khối PEM (`-----BEGIN `); (d) literal bắt đầu `«redacted»` thì **bỏ qua** (đã che). Không dùng regex backtracking nặng (đặt giới hạn độ dài input 4 MiB; vượt ⇒ coi như nghi ngờ).
3. `leak_blocked_error.go`: lỗi domain `ErrSecretLeakBlocked`; adapter gRPC (TASK-035-08) ánh xạ sang `apperrors` Kind `Internal` với thông điệp `CODEINTEL_SECRET_LEAK_BLOCKED: storage map blocked` (không giá trị, không đường dẫn tuyệt đối). Ghi metric (`codeintel_secret_leak_blocked_total`, tên đề xuất, `BE-CV-SOL-071` chốt) và log **chỉ** `{path nguồn, kind}`.
4. Ghi vào comment: quét chạy trên **byte payload cuối**, không trên cấu trúc, để bắt cả lỗi serialize/ánh xạ về sau (ai thêm trường chuỗi mới cũng bị quét).

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/secretmasking/...`: dương tính: payload có `postgresql://orca:CANARY_pw@h/db`, token `ghp_` giả 36 ký tự, chuỗi base64 40 ký tự trộn hoa/thường/số, khối PEM; âm tính: toàn bộ tên service thật (`credential-broker-service`, `infra-fleet-service:9090`), image (`hashicorp/vault:1.17`, `gcr.io/distroless/static-debian12:nonroot`), hash `sha256` hex 64 (chữ thường + số, không chữ hoa ⇒ không dính; **ghi lại** nếu muốn bắt hash thì cần quyết định riêng), payload `StorageMap` vàng của TASK-035-08 ⇒ 0 finding.
- Test "bất biến": log bắt được khi chạy ca dương tính không chứa chuỗi canary (dùng logger ghi vào buffer).
- Fuzz ngắn `FuzzScan` không panic với input ngẫu nhiên.

## Tiêu chí hoàn thành

- [ ] Ca dương tính bị chặn, ca âm tính (toàn bộ tên thật nêu trên) không bị chặn.
- [ ] `Finding` không mang nội dung khớp; log/metric không chứa giá trị.
- [ ] Ngưỡng là hằng có tên, có comment "chưa hiệu chỉnh".

## Rủi ro và lưu ý

- Báo nhầm trên tên dài có thể xảy ra khi service đặt tên có chữ hoa; theo dõi qua metric sau khi bật, hạ ngưỡng có kiểm soát.
- Báo thiếu: secret chữ thường toàn bộ (hex 32) không bị bắt; chấp nhận vì parser không bao giờ đưa giá trị khoá lạ vào payload (lớp một).
