# AG-CV-TASK-081-14: Gốc worktree, danh sách tệp đổi theo scope, `dirtyFingerprint`

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-workspace-root.ts`, `quality-changed-files.ts`, `quality-dirty-fingerprint.ts` (mới) + test
**Depends on:** AG-CV-SOL-001 (`codeintel-repo-resolution.ts`)
**Status:** [ ] TODO

## Context

Hợp đồng §2.1 (workspaceRoot), §5.2 (`scope`, `base`), CR-081 2.5. Lệnh git: `rev-parse --show-toplevel`, `diff --name-only -z --diff-filter=ACMR <base>...HEAD`, `status --porcelain=v1 -z --untracked-files=normal`, `rev-parse --verify --quiet <base>^{commit}`: tất cả < Git 2.25 (AGENTS.md), giữ `-c core.quotePath=false` ở đầu, không `GitCapabilityCache`.

## Việc cần làm

1. `resolveWorktreeRoot(workspaceRoot)`: gọi hàm xuất khẩu của SOL-001 (bước 1-4); nếu chưa xuất, trích ra từ `codeintel-repo-resolution.ts`. Lỗi → `CODEINTEL_PATH_NOT_ALLOWED data.hint`. Quality **không** cần registry GitNexus (không `REPO_NOT_REGISTERED`).
2. `isSafeBaseRef(base)`: `^[0-9a-f]{7,64}$` hoặc tên ref hợp lệ theo luật `check-ref-format --branch` (hàm thuần: không `..`, `@{`, `//`, khoảng trắng, ký tự `~^:?*[\`, không kết thúc `.`/`/`/`.lock`), không bắt đầu `-`; sai → `INVALID_PARAMS field="base"`.
3. `changedFiles({ root, scope, base })`: `worktree` → `null`; `commitRange` → `diff ... <base>...HEAD`; `changed` → hợp của `diff` và `status` (kể cả untracked, đổi tên lấy đường dẫn mới). `base` thiếu với `changed|commitRange` → `INVALID_PARAMS reason="base_required"`; không resolve được → `reason="unresolved_ref"`; không merge-base → `no_merge_base`.
4. `dirtyFingerprint(root)`: sha256 của danh sách `git status --porcelain=v1 -z` + `mtimeMs:size` từng tệp (cắt 5 000 mục), dạng `sha256:<hex>`.
5. Mọi tên tệp trả về: tương đối gốc, `/`, loại NUL/điều khiển, loại tên bắt đầu `-` hoặc `realpath` thoát worktree.

## Kiểm thử

Repo git thật trong thư mục tạm: commit, sửa chưa commit, tệp mới, đổi tên, tên có khoảng trắng/Unicode, `git worktree add` (worktree liên kết), `base` không tồn tại, `base:"--upload-pack=x"`; fingerprint đổi khi sửa tệp và không đổi khi không sửa. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-changed-files.test.ts src/relay/quality-workspace-root.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không lệnh git nào dùng cờ mới hơn 2.25 (test quét argv).
- [ ] `base` lạ không bao giờ tới `git` (kiểm trước khi spawn).

## Rủi ro

`diff <base>...HEAD` cần `base` có trong object store; shallow clone chưa kiểm chứng. Đường dẫn Windows chưa làm.
