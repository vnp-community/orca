# AG-REQ-TASK-033-03: Kiểm vùng làm việc `worktree | repo_root | scratch` (`agent-workspace-validation.ts`)

**From Solution:** [AG-REQ-SOL-033-exec-prompt-readonly-and-workspace](../solutions/AG-REQ-SOL-033-exec-prompt-readonly-and-workspace.md) mục 2.4
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-workspace-validation.ts` (mới), `agent/src/relay/agent-workspace-validation.test.ts` (mới)
**Depends on:** không (kiểu `accessMode`/`workspaceKind` có thể khai báo cục bộ; nếu task 01 đã xong thì import kiểu từ `agent-exec-prompt-options.ts`)
**Status:** [x] DONE

## Context

Hiện `worktreePath` chỉ bị kiểm "không rỗng". Đường dẫn không tồn tại làm `spawn` phát lỗi `ENOENT` và kết quả là `exitCode: null` với `stderr` chứa thông điệp lỗi, nên backend không phân biệt được với lần hết giờ (cũng `exitCode: null`). CR-033 mục 2.3 đặt `workspaceKind` để khai báo `worktreePath` là gì: `worktree` (mặc định, KHÔNG kiểm thêm), `repo_root` (phải tồn tại, là thư mục, nằm trong repo Git, và BẮT BUỘC `accessMode=readonly`), `scratch` (tồn tại, là thư mục, nằm dưới `os.tmpdir()` hoặc `<config.workDir>/.orca-scratch/`, chống thoát bằng `realpath`; người gọi tạo thư mục trước bằng `fs.mkdir`, agent không tự tạo hay xoá).

Lý do `repo_root` buộc chỉ đọc: `spike`, `question`, `diagnosis` (CR-REQ-008) dùng `repo_path` của project và hiện không có gì ngăn ghi vào checkout chính. Lệnh Git duy nhất cần: `git rev-parse --show-toplevel`, có từ rất sớm, không tuỳ chọn mới nên không đổi baseline Git 2.25 (`guides/reference/git-compatibility.md`); không cần `GitCapabilityCache`. Agent chạy trên chính host (native, WSL, SSH) nên đúng ở cả ba (AGENTS.md mục SSH).

`AgentConfig.workDir` có sẵn (`agent-config.ts` dòng 29; mặc định `AGENT_WORK_DIR` hoặc `process.cwd()`). Mẫu chạy Git: `execFile` ở `agent-git-handler.ts`.

## Việc cần làm

1. Tạo `agent-workspace-validation.ts` với kiểu và hàm theo solution mục 2.4:
   ```ts
   export type WorkspaceErrorCode =
     'REPO_ROOT_REQUIRES_READONLY' | 'WORKSPACE_PATH_NOT_FOUND' | 'WORKSPACE_NOT_A_DIRECTORY'
     | 'WORKSPACE_NOT_A_GIT_REPO' | 'SCRATCH_OUTSIDE_ALLOWED_ROOTS'
   export type WorkspaceCheckInput = {
     kind: 'worktree' | 'repo_root' | 'scratch'
     path: string
     accessMode: 'write' | 'readonly'
     scratchRoots: string[]
     gitToplevel?: (cwd: string) => Promise<string | null>
   }
   export async function validateWorkspace(input): Promise<
     { ok: true; realPath: string } | { ok: false; code: WorkspaceErrorCode; message: string }>
   export function defaultScratchRoots(workDir: string): string[]  // [os.tmpdir(), path.join(workDir, '.orca-scratch')]
   ```
2. Thứ tự kiểm cho `repo_root`: (a) nếu `accessMode !== 'readonly'` thì `REPO_ROOT_REQUIRES_READONLY` NGAY, trước mọi truy cập tệp; (b) `fs.realpath(path)` (ENOENT thì `WORKSPACE_PATH_NOT_FOUND`); (c) `fs.stat(real).isDirectory()` (không thì `WORKSPACE_NOT_A_DIRECTORY`); (d) `gitToplevel(real)` (mặc định `execFile('git', ['rev-parse', '--show-toplevel'], { cwd: real, timeout: 5000, windowsHide: true })`, `stdout.trim()` hoặc `null` khi lỗi); `null` thì `WORKSPACE_NOT_A_GIT_REPO`. Không đặt tuỳ chọn Git toàn cục.
3. Cho `scratch`: `realpath` và `stat` như trên, rồi với mỗi gốc trong `scratchRoots` lấy `realpath` (gốc không tồn tại thì bỏ qua gốc đó, không lỗi); đường dẫn hợp lệ khi `path.relative(rootReal, real)` không rỗng, không bắt đầu bằng `..` và không tuyệt đối (tuyệt đối xảy ra trên Windows khác ổ đĩa). Đúng bằng thư mục gốc (relative rỗng) bị từ chối vì `os.tmpdir()` dùng chung; không hợp lệ thì `SCRATCH_OUTSIDE_ALLOWED_ROOTS`.
4. Cho `worktree`: trả `{ ok: true, realPath: input.path }` mà KHÔNG gọi hệ tệp (giữ hành vi cũ, ENOENT vẫn lộ ở bước `spawn`).
5. Thông điệp lỗi: ngắn, có đường dẫn người dùng gửi, không có đường dẫn đã giải `realpath` của gốc scratch (tránh lộ cấu trúc thư mục của host cho người gọi ngoài ý).
6. Hàm trả KẾT QUẢ, không ném; việc đổi sang phản hồi JSON-RPC là của task 04.

## Kiểm thử

`agent-workspace-validation.test.ts` dùng thư mục tạm thật (`fs.mkdtemp(path.join(os.tmpdir(), 'orca-ws-'))`) và `gitToplevel` giả:
- `worktree kind never touches the filesystem` (đường dẫn không tồn tại vẫn `ok: true`).
- `repo_root with write is rejected before any filesystem access` (đường dẫn không tồn tại vẫn ra `REPO_ROOT_REQUIRES_READONLY`).
- `repo_root readonly rejects missing path / file path / non-git directory` (ba `it`).
- `repo_root readonly accepts a directory when gitToplevel returns a path`.
- `scratch accepts a directory under os.tmpdir()` và `... under <workDir>/.orca-scratch`.
- `scratch rejects a path outside the allowed roots`.
- `scratch rejects os.tmpdir() itself`.
- `scratch rejects a symlink under tmp that points outside the roots` (tạo symlink bằng `fs.symlink`; `it.skipIf(process.platform === 'win32')`).
- `scratch does not confuse /tmp/foo with /tmp/foobar prefix` (thư mục `foobar` cạnh gốc giả).
- `scratch ignores an allowed root that does not exist`.
- Một `it` với Git thật: `repo_root accepts a real git init directory` (dùng `execFile('git', ['init', dir])`; bỏ qua nếu không có `git` trên máy bằng thăm dò `git --version`).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-workspace-validation.test.ts`; rồi `pnpm test`. Windows và WSL: chưa chạy; test chỉ dùng `path` thuần và bỏ qua symlink trên Windows.

## Tiêu chí hoàn thành

- [x] `repo_root` + `write` luôn là `REPO_ROOT_REQUIRES_READONLY` và không đọc hệ tệp.
- [x] `scratch` thoát được bằng symlink hoặc `..` đều bị `SCRATCH_OUTSIDE_ALLOWED_ROOTS`.
- [x] `worktree` không thay đổi hành vi (không gọi `fs`, không gọi Git).
- [x] Chỉ lệnh Git là `rev-parse --show-toplevel`, gọi bằng `execFile` không qua shell.
- [x] Mọi hàm tuân thứ tự kiểm ở bước 2; không ném lỗi ra ngoài.

## Rủi ro và lưu ý

- Trên macOS `os.tmpdir()` thường là liên kết (`/var` trỏ `/private/var`); phải `realpath` cả gốc lẫn đường dẫn (đã có ở bước 3).
- Trên Windows so sánh đường dẫn không phân biệt hoa thường; `path.relative` của `node:path` (win32) xử lý ổ đĩa nhưng chưa kiểm chứng ở đây.
- `git rev-parse --show-toplevel` trong worktree liên kết trả thư mục worktree đó (không phải kho chính); chấp nhận vì `repo_root` chỉ cần "nằm trong repo Git".
- "Gần như đúng" cho thư mục git bị `safe.directory` chặn (chủ sở hữu khác): `rev-parse` thất bại và ta báo `WORKSPACE_NOT_A_GIT_REPO` dù là repo; lỗi này lộ khi agent chạy bằng người dùng khác; ghi vào thông điệp lỗi gợi ý kiểm `safe.directory`.
- Mã `WORKSPACE_PATH_NOT_FOUND` dùng `InvalidParams` thay `PathNotFound -33003` (câu hỏi mở 2 của solution); nếu chốt khác, chỉ sửa ở task 04.
