# BE-CV-TASK-072-01: Bộ vector đường dẫn `path-attack-vectors.json` và kiểm đường dẫn ở service

**From Solution:** BE-CV-SOL-072
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/security/path-attack-vectors.json` (mới), `.../internal/usecase/path_attack_vectors_test.go` (mới), `.../internal/usecase/repo_path_symlink_test.go` (mới)
**Depends on:** BE-CV-SOL-030 (`RepoSourceReader`, `NormalizeRepoPath`), BE-CV-SOL-012 (`workspace_root` trong `repo_bindings`)
**Status:** `[x] DONE`

---

## Context

- Vector dùng chung ba nơi (agent vitest, service, gateway) theo SOL-072 D1; ca lấy từ CR-072 §2.4. Mẫu logic: `api-gateway/.../tools/sensitive_path_rules.go` (`CleanWorktreePath`: NFKC, `\`, `/` đầu, `%2e` hai lớp, `X:\`, >1024, UTF-8 hỏng).
- agent-rpc §2.1: `workspaceRoot` phải là gốc worktree; đường dẫn tham số tương đối, không `..`/`\`.

## Việc cần làm

1. Viết JSON `{version, vectors:[{input, field, expect, reasonCode, note}]}` với mọi ca ở SOL-072 mục 5.1 (kể cả tiền tố `orca` so với `orca-old`, `.env`, `id_rsa`, `.git/config`, thư mục con của worktree làm `workspaceRoot`).
2. `TestPathAttackVectors`: chạy từng vector qua hàm kiểm của service; `deny` ⇒ `CODEINTEL_PATH_NOT_ALLOWED`; `allow` ⇒ qua.
3. `TestSymlinkEscapeDenied`: dựng cây tạm (`t.TempDir`) có symlink trỏ ra ngoài (`repo/link -> /etc`), `workspaceRoot` là symlink; từ chối sau `realpath`; root có `/` cuối, `//`.
4. Ghi vào `MANIFEST` của `testdata/` (BE-CV-TASK-070-01) nếu cùng cây; báo `AG-CV-SOL-072` đường dẫn tệp.

## Kiểm thử

- Hai test trên; `go test ./internal/usecase/... -run 'PathAttack|Symlink'`. Không DB.

## Tiêu chí hoàn thành

- [x] Mọi vector cho kết quả đúng; symlink thoát root bị từ chối.
- [x] Tệp vector không chứa đường dẫn thật hay secret.

## Rủi ro và lưu ý

- Windows/UNC chưa có dev server thật; chỉ kiểm chuỗi. TOCTOU chưa kiểm chứng.
