# BE-CV-TASK-030-02: `CleanRel` và chính sách nguồn (allowlist, deny-list, thư mục bỏ qua)

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/repo_relative_path.go` (mới), `.../internal/domain/repo_source_policy.go` (mới), `.../internal/domain/repo_relative_path_test.go` (mới), `.../internal/domain/repo_source_policy_test.go` (mới)
**Depends on:** BE-CV-TASK-030-01
**Status:** [x] DONE

---

## Context

Miền thuần (không I/O), nền của mọi chặn đường dẫn (SOL-030 mục 2.D). Agent Part A không chặn đọc ngoài workspace (`fs-agent-extensions.ts`), nên đây là lớp bảo vệ duy nhất. Lỗi ra `CODEINTEL_PATH_NOT_ALLOWED` (PQ-03).

## Việc cần làm

1. `CleanRel(p string) (string, error)`: chuẩn hoá NFC; từ chối rỗng, bắt đầu `/`, có chữ ổ đĩa kiểu `X:`, chứa NUL/ký tự điều khiển/`\`, bắt đầu `-`, mọi phần tử `..` sau `path.Clean`, phần trăm-mã hoá `%2e`/`%2f` (không giải mã, chỉ từ chối `%`). Trả đường dẫn dùng `/`. Lỗi là `ErrPathNotAllowed` (miền) để adapter đổi sang `apperrors`.
2. `JoinWorkspace(root, rel string) string` (hàm thuần, chỉ POSIX): `root + "/" + rel`; `root` phải tuyệt đối, đã lấy từ binding.
3. `repo_source_policy.go`: hằng/bảng cấu hình được: `AllowedExtensions` (`.sql .proto .go .yaml .yml .json .toml .md`), `AllowedBaseNames` (`Dockerfile`, `CODEOWNERS`), mẫu `docker-compose*.yml`; `DeniedPatterns` (`.env*`, `*.pem`, `*.key`, `id_*`, `*secret*`, không phân biệt hoa/thường); `SkippedDirs` (`.git node_modules vendor dist out .gitnexus .codegraph target .next build coverage`). Hàm `Classify(rel string) Skipped` trả `""|"not_allowed"`; deny-list thắng allowlist.
4. `IsSkippedDir(name string) bool`.
5. Không phụ thuộc gói ngoài ngoài thư viện chuẩn; không import `adapter`/`usecase`.

## Kiểm thử

- `cd backend-go && go test ./services/code-intel-service/internal/domain/...` (chưa chạy).
- Bảng ca xấu: `""`, `/etc/passwd`, `../x`, `a/../../x`, `a\b`, `-rf`, `a%2e%2e/b`, `C:\x`, `a\u0000b`, `a/./b` (hợp lệ → `a/b`), chuỗi U+FF0D đầu (NFKC → `-`: phải bị từ chối; so khớp sau chuẩn hoá NFKC cho kiểm tiền tố).
- `Classify`: `config/.env.local` → `not_allowed`; `db/0001_init.up.sql` → `""`; `server.PEM` → `not_allowed`; `docs/note.txt` → `not_allowed` (ngoài allowlist).

## Tiêu chí hoàn thành

- [x] Mọi ca xấu bị từ chối; ca tốt chuẩn hoá đúng.
- [x] Bảng `SkippedDirs` và deny-list có test khẳng định nguyên văn.
- [x] Không có tên `helpers/utils/common/misc`; không `max-lines` disable.

## Rủi ro và lưu ý

- Tên file không ASCII phải qua được (NFC), vì repo có thể có tên Việt; chỉ từ chối điều khiển và `\`.
- Windows không nằm trong MVP (O-14): `JoinWorkspace` ghi chú rõ chỉ POSIX.
