# BE-CV-TASK-020-05: `NormalizeRepoPath` (Windows/WSL/SSH, từ chối đường dẫn ngoài gốc)

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/repo_path.go` (mới), `repo_path_test.go` (mới)
**Depends on:** TASK-020-01
**Status:** [x] DONE

---

## Context

Dữ liệu đúng hợp đồng là đường dẫn **tương đối gốc repo, dấu `/`** (agent §2.6). Hàm phòng thủ khi agent cũ/lỗi trả tuyệt đối. `hostPlatform` lấy từ `HandshakeInfo.Platform` (`devserveragent/session.go`, struct `HandshakeInfo`: `win32|darwin|linux`; đã đọc). MVP chỉ POSIX (hợp đồng O-14).

## Việc cần làm

1. `type Platform string` (`PlatformLinux`, `PlatformDarwin`, `PlatformWin32`); `NormalizeRepoPath(raw, workspaceRoot string, plat Platform) (string, error)`.
2. Quy tắc theo thứ tự: từ chối chuỗi rỗng/NUL; từ chối tiền tố UNC `\\wsl$\`, `\\wsl.localhost\`, `\\?\` (lỗi `CODEINTEL_PATH_NOT_ALLOWED`); nếu tuyệt đối và nằm dưới `workspaceRoot` thì cắt tiền tố (so sánh gập hoa/thường chỉ khi `win32`/`darwin`, chữ ổ đĩa viết hoa); tuyệt đối ngoài root → lỗi; `\`→`/` chỉ khi `win32`; `path.Clean`; loại `./`; từ chối kết quả bắt đầu bằng `..`; NFC.
3. Lỗi dùng `apperrors.New(apperrors.KindPermissionDenied, "CODEINTEL_PATH_NOT_ALLOWED", <≤200 ký tự, không in đường dẫn thô>, nil)`.
4. Hàm phụ `FoldCaseForCompare(path string, plat Platform) string` dùng khi so sánh `changedFiles` với nút (không đổi chuỗi lưu).
5. Nối `NormalizeRepoPath` làm `PathNormalizer` cho `ValidateAgentSymbolRef` (TASK-020-04).

## Kiểm thử

- `cd backend-go/services/code-intel-service && go test ./internal/domain/ -run RepoPath -race`.
- Ca: `C:\repo\src\a.ts` root `c:\repo` `win32` → `src/a.ts`; `/home/u/repo/../x` root `/home/u/repo` → lỗi; `\\wsl$\Ubuntu\home\u\a.ts` → lỗi; `linux` với `a\b.ts` giữ nguyên; `darwin` gập hoa/thường khi so sánh nhưng giữ nguyên khi lưu; NFD → NFC; đường dẫn tương đối `./src//a.ts` → `src/a.ts`.
- Thông điệp lỗi không chứa đường dẫn đầu vào (test `assert.NotContains`).

## Tiêu chí hoàn thành

- [x] Mọi ca ở trên xanh.
- [x] Không panic với đầu vào ≥ 4096 ký tự (trả lỗi).
- [x] Domain không import thư viện ngoài `golang.org/x/text/unicode/norm` (nếu đã có trong `go.mod` workspace; nếu chưa, ghi nhận phụ thuộc mới vào PR).

## Rủi ro và lưu ý

- Chưa kiểm chứng GitNexus/CodeGraph báo đường dẫn nào trên Windows; chỉ POSIX được kiểm.
- Thêm `golang.org/x/text` là phụ thuộc mới nếu chưa có: cần duyệt nhẹ khi review (kiểm `grep -r "x/text" backend-go/go.work.sum` trước).
