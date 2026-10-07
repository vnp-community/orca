# BE-CV-TASK-030-06: `ResolveHead`, `BlobIDs`, `DirtyPaths`, `ChangedFiles`, `FileDiff`, `Log`

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/agentrepofs/git_read_ops.go` (mới), `.../git_log_parser.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-030-04 (song song được với 030-05; gắn `ContentHash` vào 030-05 sau khi xong)
**Status:** [x] DONE

---

## Context

SOL-030 mục 2.C. Chỉ dùng lệnh có từ git 2.25 trở về trước và không dính `SHELL_METACHARACTERS = /[&|;$\`<>\\!]/` (`agent-git-handler.ts:70`); cờ đứng trước subcommand bị từ chối (không dùng `-c core.quotePath=false` với `git.exec`; riêng `git.status` agent đã tự chèn). Theo AGENTS.md "Git Binary Compatibility": cổng chỉ dùng lệnh nền nên **không cần** `GitCapabilityCache`; nếu sau này thêm tuỳ chọn mới hơn phải có phương án lùi và khoá theo `dev_server_id` + test hợp đồng git 2.25.5/2.38.1/2.49.1 (BE-CV-SOL-070).

## Việc cần làm

1. `ResolveHead`: `git.exec rev-parse --verify HEAD`; kiểm đầu ra khớp 40/64 hex; (tuỳ chọn) lấy `head` từ kết quả `git.status` nếu cùng lượt đã gọi.
2. `BlobIDs`: chia lô 100 đường dẫn, `rev-parse <c>:<p>`; mỗi dòng một oid theo thứ tự đầu vào; đường dẫn không tồn tại → lỗi một phần: oid rỗng + `Skipped:"not_found"` (xác định cách `git rev-parse` báo lỗi với đường dẫn không có: ghi vào fixture, **chưa kiểm chứng**).
3. `DirtyPaths`: `git.status {worktreePath, limit:0}`; gộp `entries[].path` của `staged|unstaged|untracked`; lọc theo `scope` (tiền tố); trả tập; `didHitLimit` phải false, nếu true → `CODEINTEL_OUTPUT_TOO_LARGE`.
4. `ChangedFiles`: `git.branchCompare {worktreePath, baseRef}`; từ chối `baseRef` bắt đầu `-` hoặc chứa ký tự metachar; `summary.status != "ready"` → `CODEINTEL_INVALID_PARAMS` với `reason ∈ invalid-base|unborn-head|no-merge-base`; trả `BranchCompare{BaseOid, HeadOid, MergeBase, Entries}`.
5. `FileDiff`: `git.branchDiff {..., includePatch:true, filePath, oldPath?}` → `DiffPair{Original, Modified, binary flags}`; nhị phân → `Skipped:"binary"`; vượt `MaxFileBytes` mỗi phía → `too_large`.
6. `git_log_parser.go` + `Log`: tham số cố định `--no-merges --format=%H%x1f%an%x1f%at%x1f%s -z --since=<n>.days -n<N> -- <pathspec>`; luôn có `-n` và `--since` (agent không giới hạn dung lượng); pathspec loại trừ dùng `:(exclude)`; phân tách bản ghi bằng NUL, trường bằng `0x1f`; ASCII hoá/che email theo quy tắc `BE-CV-SOL-013`; tên tác giả không ASCII giữ nguyên nếu hợp lệ UTF-8, byte hỏng thay `�`.
7. Gắn `ContentHash` cho 030-05: sạch → oid từ `BlobIDs`, bẩn → `sha256:`.

## Kiểm thử

- Unit parser `Log`: tên có `|`, dấu phẩy, ký tự đặc biệt, UTF-8 bị cắt đôi, thông điệp rỗng, 0 commit.
- Agent giả: `git.status` có `untracked`; `branchCompare` trạng thái `no-merge-base`; `branchDiff` nhị phân.
- Test tham số: không có chuỗi `|`, `!`, `:!`, `$`, backtick trong **bất kỳ** `args` nào cổng sinh ra (quét bảng ca, regex `SHELL_METACHARACTERS`).
- Hợp đồng git thật: `BE-CV-SOL-070` (git 2.25.5/2.38.1/2.49.1) — liên kết, không làm ở đây.
- `go test ./services/code-intel-service/internal/adapter/agentrepofs/...` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Mỗi hàm có test với agent giả; oid từ `BlobIDs` khớp thứ tự đầu vào.
- [x] `DirtyPaths` thấy file chưa theo dõi (ví dụ migration mới).
- [x] Không tham số nào vi phạm `SHELL_METACHARACTERS`.
- [x] `Log` luôn có `-n` và `--since`.

## Rủi ro và lưu ý

- `git.branchCompare`/`branchDiff` thuộc `agent-git-handler-extended.ts`; hành vi khi `baseRef` là commit rời chưa kiểm chứng.
- `git.branchDiff` trả nội dung hai phía cho **một** file mỗi lần; chi phí N lần gọi với RTT 100 ms: consumer phải gom, không gọi theo từng dòng.
