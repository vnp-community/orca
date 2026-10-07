# BE-CV-TASK-040-02: Tách `CleanWorktreePath` thành gói `pathsafety`

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/pathsafety/worktree_path.go` (mới), `internal/adapter/pathsafety/worktree_path_test.go` (mới), `internal/adapter/mcpserver/tools/sensitive_path_rules.go`, `internal/adapter/mcpserver/tools/sensitive_path_rules_test.go`
**Depends on:** không
**Status:** [x] DONE

---

## Context

`CleanWorktreePath` (`tools/sensitive_path_rules.go:27`) là logic duy nhất chặn `..`, `/abs`, `\`, `%XX` còn dư, ký tự điều khiển/bidi, đoạn kết thúc bằng `.`/khoảng trắng và dạng NFKC (`．．`). Gói `tools` import `wscompat` (`spec.go:15`) nên `wscompat` không dùng được trực tiếp; hợp đồng O-16 chốt tách gói đặt tên cụ thể (`pathsafety`). Người gọi hiện có: `files_sensitive_guard.go:35`, `mcpserver/resources/uri.go:117,122`.

## Việc cần làm

1. Chạy `gitnexus_impact({target:"CleanWorktreePath", direction:"upstream"})` và ghi kết quả vào PR (kỳ vọng: `guardInputPath`, `resources/uri.go`, test).
2. Tạo `pathsafety/worktree_path.go`: chuyển nguyên `ErrUnsafePath`, `doubleEncoded`, `CleanWorktreePath`, `checkPathForm` (giữ import `golang.org/x/text/unicode/norm`). **Không đổi hành vi**, không đổi thông điệp lỗi.
3. `tools/sensitive_path_rules.go`: xoá phần đã chuyển; thêm `var ErrUnsafePath = pathsafety.ErrUnsafePath` và `func CleanWorktreePath(raw string) (string, error) { return pathsafety.CleanWorktreePath(raw) }` (một dòng chú thích: giữ chữ ký cho `resources`/files guard). `IsSensitivePath`, `containsPrivateKey` ở lại `tools`.
4. Chuyển `TestCleanWorktreePath_RejectsTraversalAndTricks` sang `pathsafety/worktree_path_test.go`, bổ sung vector của hợp đồng: `..`, `a/../b`, `/etc/passwd`, `C:\x`, `a\b`, `%2e%2e/x`, `a/%2e%2e`, `．．/x` (U+FF0E), `a/\u202ebad` (bidi), `a/.`, `a/`, chuỗi rỗng, `0xC0 0xAE` (UTF-8 quá dài), chuỗi > 1024 byte; và vector hợp lệ: `src/a.go`, `docs/crs/v7/x.md`.
5. Giữ `sensitive_path_rules_test.go` cho `IsSensitivePath` và `containsPrivateKey`.

## Kiểm thử

- `go test ./internal/adapter/pathsafety/... ./internal/adapter/mcpserver/...` (toàn bộ gói `tools`, `resources` phải giữ xanh, kể cả `files_sensitive_guard_test.go`).
- Kiểm không import vòng: `go build ./...` và `go vet ./...` trong `api-gateway`.

## Tiêu chí hoàn thành

- [x] `wscompat` import được `pathsafety`; `pathsafety` không import `tools`/`wscompat`.
- [x] Mọi test cũ của `tools` và `resources` xanh không sửa.
- [x] Vector độc hại của hợp đồng đều trả `ErrUnsafePath`.

## Rủi ro và lưu ý

- Hàm từ chối chuỗi rỗng và dấu `/` cuối: nơi gọi mới (TASK-040-03) phải chỉ gọi khi trường có mặt và khác rỗng.
- Đổi tên/di chuyển ký hiệu: không dùng find-and-replace (quy ước GitNexus `rename`); ở đây không đổi tên nên rủi ro thấp.
