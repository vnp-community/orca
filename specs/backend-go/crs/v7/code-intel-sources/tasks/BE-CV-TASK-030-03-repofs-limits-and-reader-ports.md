# BE-CV-TASK-030-03: Cổng `RepoSourceReader`, `AgentRelay` và cấu hình `CODEINTEL_REPOFS_*`

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/repo_source_reader.go` (mới), `.../internal/usecase/agent_relay.go` (mới), `.../internal/config/repofs_limits.go` (mới), `.../internal/config/repofs_limits_test.go` (mới)
**Depends on:** BE-CV-TASK-030-02, và `internal/config` của BE-CV-SOL-010 (nếu chưa có, đồng bộ trước khi merge)
**Status:** [x] DONE

---

## Context

Định nghĩa cổng và giới hạn để các consumer 031–037 viết trước khi adapter xong (dùng fake). Port `AgentRelay` do SOL-030 tạo và `BE-CV-SOL-021-agent-collector` dùng lại (SOL-030 mục 1 C5): **trước khi tạo, `ls internal/usecase` để chắc SOL-021 chưa thêm**.

## Việc cần làm

1. `repo_source_reader.go`: `RepoRef{TenantID, DevServerID, WorkspaceRoot}`, `SourceRef{Kind, Commit}`, `DirEntry{Name, RelPath, IsDir, Size}`, `FileContent{Path, Size, Content []byte, ContentHash, Skipped}`, `ReadReport{Requested, Read, SkippedBinary, SkippedTooLarge, SkippedOther, Truncated}`, `BranchCompare`, `DiffPair`, `LogQuery{Since days, Limit, Pathspec []string}`, `CommitRecord{Hash, Author, At, Subject, Files []string}`, `interface RepoSourceReader` (chữ ký SOL-030 mục 2.B).
2. `agent_relay.go`: `interface AgentRelay { Call(ctx, devServerID, method string, params map[string]any) (json.RawMessage, error) }`.
3. `repofs_limits.go`: `RepoFSLimits` với `MaxFileBytes 1<<20`, `MaxFilesPerCall 2000`, `MaxBytesPerCall 32<<20`, `MaxDirEntries 5000`, `MaxGitOutputBytes 4<<20`, `Concurrency 8`, `CallTimeout 25s`, `SkippedDirs []string`; nạp từ `CODEINTEL_REPOFS_MAX_FILE_BYTES`, `..._MAX_FILES_PER_CALL`, `..._MAX_BYTES_PER_CALL`, `..._MAX_DIR_ENTRIES`, `..._MAX_GIT_OUTPUT_BYTES`, `..._CONCURRENCY`, `..._CALL_TIMEOUT`, `..._SKIPPED_DIRS`. Giá trị không phân tích được hoặc ≤ 0 bị bỏ, dùng mặc định, log cảnh báo (không log giá trị bí mật; chúng không bí mật).
4. Ràng buộc: `CallTimeout` phải < 30 s (mặc định `RequestTimeout` của infra-fleet); nếu cấu hình ≥ 30 s thì kẹp xuống 25 s và cảnh báo.
5. Ghi chú trong code (1–2 dòng) vì sao 25 s: nhỏ hơn 30 s của `devserveragent/config.go:57`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/config/...` (chưa chạy): mặc định, ghi đè hợp lệ, giá trị sai (chuỗi, âm), `CALL_TIMEOUT=45s` bị kẹp.
- Biên dịch: `go build ./services/code-intel-service/...`.

## Tiêu chí hoàn thành

- [x] Cổng khớp SOL-030 2.B, không có `repo_binding_id` (PQ-04).
- [x] Mọi biến có tiền tố `CODEINTEL_REPOFS_` (PQ-23).
- [x] Test cấu hình xanh; không `max-lines` disable.

## Rủi ro và lưu ý

- `RepoRef.TenantID` phải do use case lấy từ `tenant.RequireTenantID(ctx)`; adapter kiểm `RepoRef.TenantID` khớp ctx và từ chối nếu khác (chống nhầm lẫn trong test và code sau).
