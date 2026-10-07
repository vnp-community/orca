# BE-CV-TASK-072-07: Quét canary secret toàn pipeline

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`, `api-gateway`
**File:** `.../code-intel-service/e2e/secret_canary_test.go` (mới, tag `e2e`), `.../code-intel-service/testdata/security/canary-config-files/` (mới), `.../internal/domain/secret_masking_parity_test.go` (mới)
**Depends on:** BE-CV-SOL-073 task 05 (harness e2e T1), BE-CV-SOL-035 (StorageMap, P2: phần này có điều kiện), BE-CV-SOL-030, `AG-CV-SOL-070` (repo mẫu)
**Status:** `[x] DONE`

---

## Context

- Canary (CR-072 §2.6): DSN, `REDIS_PASSWORD`, `API_KEY`, PEM giả, `ghp_`/`AKIA` giả, URL userinfo; tên khoá/đường dẫn Vault được phép, giá trị thì không.
- Bộ che hiện có: `api-gateway/.../tools/redaction_rules.go`, `mcp-service/internal/domain/params_hash.go` (`Redactor`). O-16: một bộ duy nhất.
- ui-api §2.3 `CODEINTEL_SECRET_LEAK_BLOCKED`: quét cuối thấy secret ⇒ lỗi, không hiển thị thô.

## Việc cần làm

1. Tệp canary tổng hợp (không chuỗi thật) trong `testdata/security/`.
2. Chạy pipeline e2e T1 với tệp canary; quét: `graph_snapshots.payload` và mọi cột JSON (dump), log `slog` (handler bắt), lỗi gRPC/WS, span (tracer bộ nhớ), khung push, `GetSymbol`, `StorageMap` (nếu có).
3. Khẳng định không `CANARY-`/PEM/`ghp_`; nếu quét cuối bắt thì kết quả là `CODEINTEL_SECRET_LEAK_BLOCKED`.
4. `secret_masking_parity_test`: nếu có hai bảng mẫu (gateway và service), kiểm hai bản cho cùng đầu ra trên tập mẫu (đỏ khi lệch).

## Kiểm thử

- `go test -tags=e2e ./e2e/... -run Canary` (Docker). Chưa chạy.

## Tiêu chí hoàn thành

- [x] Không canary ở mọi điểm quét.
- [x] Giới hạn che theo regex được ghi trong tài liệu threat model.

## Rủi ro và lưu ý

- Chỉ mục công cụ có thể đã chứa secret; không sửa được ở v7.
