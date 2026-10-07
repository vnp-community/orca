# BE-CV-TASK-030-07: Bộ kiểm thử hợp đồng cổng (agent giả), cô lập tenant, không rò nội dung

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/agentrepofs/reader_contract_test.go` (mới), `.../agentrepofs/fake_agent_relay_test.go` (mới), `.../agentrepofs/no_content_leak_test.go` (mới)
**Depends on:** BE-CV-TASK-030-05, BE-CV-TASK-030-06
**Status:** [x] DONE

---

## Context

Chốt các tiêu chí SOL-030 mục 6 bằng một suite chạy được trong CI không cần dev server. Cổng không có DB nên "hai dialect" không áp dụng; cô lập tenant kiểm ở cache và metadata (H10).

## Việc cần làm

1. `fake_agent_relay_test.go`: `AgentRelay` giả đọc fixture task 01, ghi lại mọi `(method, params)`, cấu hình được độ trễ/lỗi/offline.
2. `reader_contract_test.go`: kịch bản đầu-cuối trên cây giả (thư mục `db/migrations` với `.sql`, symlink thư mục trỏ ra ngoài, `.env`, file nhị phân, file 2 MiB, 2500 file): kiểm số cuộc gọi, kết quả, `ReadReport`.
3. Offline: relay giả trả `FailedPrecondition INFRA_DEV_SERVER_NOT_CONNECTED` → `CODEINTEL_DEV_SERVER_OFFLINE`, hoàn tất < `CallTimeout`; chậm quá hạn → `CODEINTEL_TIMEOUT` (dùng đồng hồ giả hoặc timeout ngắn).
4. Cô lập tenant: hai `RepoRef` khác `TenantID` nhưng cùng `devServerID`/`contentHash`; kiểm cache không trúng chéo và mọi cuộc gọi gắn đúng metadata tenant; `RepoRef.TenantID` ≠ tenant trong ctx → lỗi.
5. `no_content_leak_test.go`: chạy kịch bản với log `slog` bắt vào buffer và `apperrors` message; khẳng định không chứa chuỗi sentinel nằm trong nội dung file (`SECRET_SENTINEL_...`) và không chứa đường dẫn tuyệt đối.
6. Test "whitelist đúng N": liệt kê tập method/subcommand hợp lệ và so với hằng.
7. Ghi `README` ngắn trong thư mục `testdata` cách bổ sung ca mới.

## Kiểm thử

- `cd backend-go && go test ./services/code-intel-service/internal/adapter/agentrepofs/...` (chưa chạy). Thêm `-race` cho `ReadFiles` song song: `go test -race ./services/code-intel-service/internal/adapter/agentrepofs/...`.

## Tiêu chí hoàn thành

- [x] Mọi mục của SOL-030 mục 6 có ít nhất một test tương ứng.
- [x] `-race` sạch.
- [x] Không rò sentinel vào log/lỗi.

## Rủi ro và lưu ý

- Suite dùng fixture "viết tay từ mã" cho tới khi `BE-CV-SOL-070` thay bằng bản ghi thật; đánh dấu rõ trong tên test (`TestReader_...HandWrittenFixture`).
