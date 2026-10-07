# BE-CV-TASK-030-05: `ListDir`, `ReadFile`, `ReadFiles` (worktree và commit) với quy tắc tổ tiên và cache nội dung

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/agentrepofs/list_dir.go` (mới), `.../read_file.go` (mới), `.../content_cache.go` (mới), `.../reader.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-030-04
**Status:** [x] DONE

---

## Context

SOL-030 mục 2.C, 2.D, 2.E. `fs.readDir` luôn `depth:1`; đệ quy ở backend để áp bộ lọc. Symlink = mục `type:"file"` không có `size` (`fs-agent-extensions.ts:80–85`).

## Việc cần làm

1. `list_dir.go`: gọi `fs.readDir {path: JoinWorkspace(root, rel), depth: 1}`; loại mục `type=="file"` mà `size` vắng; loại thư mục thuộc `SkippedDirs`; chuẩn hoá `RelPath` (bỏ tiền tố root); cắt ở `MaxDirEntries` và `ReadReport.Truncated=true`; sắp tên tăng dần (ổn định cho test).
2. `readSession` (struct nội bộ của use-case-call): cache `ListDir` theo thư mục; hàm `ensureAncestorsListed(rel)` liệt kê từ gốc xuống từng cấp, từ chối nếu một cấp không có trong danh sách (`CODEINTEL_PATH_NOT_ALLOWED`). Đường dẫn cố định cũng đi qua cha.
3. `read_file.go` worktree: sau tổ tiên, kiểm entry là file thường với `size ≤ MaxFileBytes` (nếu lớn: `Skipped:"too_large"`, không gọi `fs.readFile`); `Classify` (task 02) → `not_allowed`; gọi `fs.readFile`; `isBinary:true` → `Skipped:"binary"`; không giải mã `encoding:"base64"`.
4. `read_file.go` commit: kiểm `Commit` khớp `^[0-9a-f]{40}$|^[0-9a-f]{64}$`, `rel` không chứa `:`/NUL; qua whitelist; `git.exec {args:["show", commit+":"+rel], cwd:root, timeout:<CallTimeout ms>}`; cắt ở `MaxGitOutputBytes` (vượt: `CODEINTEL_OUTPUT_TOO_LARGE`); không tính nội dung nhị phân (có NUL trong 8 KiB đầu → `binary`).
5. `ReadFiles`: song song `Concurrency`, tôn trọng `MaxFilesPerCall`/`MaxBytesPerCall` (vượt: dừng, `Truncated=true`, các file còn lại `Skipped` rỗng và không đọc); kết quả theo thứ tự đầu vào.
6. `content_cache.go`: LRU `(tenantID, devServerID, contentHash)`, ≤ 64 MiB, `sync.Mutex` hoặc gói LRU có sẵn trong `go.mod` (không thêm dependency mới nếu tránh được); khoá luôn có tenant; không ghi đĩa.
7. `ContentHash`: file sạch → blob oid (từ task 06 `BlobIDs` + `DirtyPaths`); file bẩn → `sha256:<hex>` của nội dung vừa đọc. Cổng nhận một `hashResolver` (hàm) để task 06 gắn vào; ở task này có thể dùng `sha256` cho mọi file rồi nâng cấp ở task 06.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/agentrepofs/...` (chưa chạy). Dùng agent giả từ fixture task 01.
- Symlink không bao giờ gọi `fs.readFile` (đếm số cuộc gọi của agent giả = 0 cho đường dẫn qua symlink thư mục).
- File 2 MiB: `too_large`, không gọi `fs.readFile`; `isBinary:true` chỉ đếm; `.env` → `not_allowed`.
- `ReadFiles` 2500 đường dẫn: dừng ở 2000, `Truncated`.
- Cache: hai tenant cùng `contentHash`, lần hai của tenant B **không** trúng cache của A (agent giả đếm cuộc gọi).
- `git show` với `rel` chứa `:` bị từ chối; commit ngắn 7 ký tự bị từ chối.

## Tiêu chí hoàn thành

- [x] Các tiêu chí SOL-030 mục 6 về `ListDir`, ancestor, kích thước, nhị phân, cache đều có test.
- [x] Không log nội dung file; log chỉ `relPath`, `size`.

## Rủi ro và lưu ý

- `size` từ `readDir` có thể lệch nếu file đổi giữa hai bước (chấp nhận: agent vẫn chặn 10 MiB).
- 128 file migration của `infra-fleet-service` nhỏ hơn `MaxDirEntries`; thư mục lớn hơn sẽ `Truncated` và consumer phải xử lý cảnh báo, không im lặng.
