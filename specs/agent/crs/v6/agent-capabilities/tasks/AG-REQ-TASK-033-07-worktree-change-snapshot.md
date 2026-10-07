# AG-REQ-TASK-033-07: Chụp và so sánh danh sách file thay đổi (`agent-worktree-change-snapshot.ts`)

**From Solution:** [AG-REQ-SOL-033-result-block-changes-and-output-cap](../solutions/AG-REQ-SOL-033-result-block-changes-and-output-cap.md) mục 2.4
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-worktree-change-snapshot.ts` (mới), `agent/src/relay/agent-worktree-change-snapshot.test.ts` (mới)
**Depends on:** không (hàm độc lập; task 08 nối vào handler)
**Status:** [x] DONE

## Context

CR-033 mục 2.4: khi `reportChanges=true` agent chụp trạng thái trước và sau lần chạy, chỉ khi `cwd` thuộc repo Git. Lệnh: `git --no-optional-locks status --porcelain=v1 -z --untracked-files=all` và `git rev-parse HEAD`. Chữ ký mỗi đường dẫn: `status` + `size` + `mtimeMs` của file đang bẩn; `changedFiles` là các đường dẫn có chữ ký khác trước và sau, kể cả file đã bẩn từ trước bị sửa tiếp. Kết quả `changes: { available: true, headBefore, headAfter, headMoved, changedFiles: [{path, change}], truncated }`, tối đa 2000 đường dẫn. Không phải repo Git: `{ available: false, reason: 'NOT_A_GIT_REPO' }`; với `scratch` đi bộ thư mục tối đa 5000 mục theo `size` + `mtimeMs`.

Về tương thích Git (AGENTS.md mục Git Binary Compatibility, `guides/reference/git-compatibility.md`): `--no-optional-locks` có từ Git 2.15, `status --porcelain=v1 -z --untracked-files=all` và `rev-parse HEAD` có từ trước; tất cả dưới baseline 2.25, không cần `GitCapabilityCache`. Tuỳ chọn toàn cục `--no-optional-locks` PHẢI đứng trước subcommand `status`. Lệnh chạy bằng `execFile` không qua shell, trên chính host của agent (đúng cho SSH, WSL).

Hành vi cần chốt vì CR để ngỏ: (a) đường dẫn có trong `before` nhưng không có trong `after` (bị hoàn tác hoặc commit) được tính là thay đổi với `change: 'modified'`; (b) `-z` phát mục đổi tên kiểu `R  <đích>\0<nguồn>\0`: ghi đường dẫn ĐÍCH, bỏ mục nguồn; (c) mỗi lần chụp có hạn 30 giây, quá thì `unavailable` với `reason: 'SNAPSHOT_TIMEOUT'`; lỗi khác `SNAPSHOT_FAILED` (hai lý do này ngoài CR, backend phải chấp nhận chuỗi lạ ở `reason`).

## Việc cần làm

1. Tạo `agent-worktree-change-snapshot.ts` với kiểu theo solution 2.4: `SnapshotEntry`, `WorkspaceSnapshot` (`kind: 'git' | 'directory' | 'unavailable'`), `ChangeReport`, `SnapshotDeps` (`runGit(args, cwd): Promise<{ stdout: string; code: number }>` và `statFile`, tiêm được trong test).
2. `captureSnapshot(cwd, mode, deps)`:
   - Mode `git`: chạy `git rev-parse --is-inside-work-tree` (mã khác 0 hoặc stdout khác `true` thì `unavailable NOT_A_GIT_REPO`). Rồi `git rev-parse HEAD` (lỗi, ví dụ repo chưa có commit, thì `head = null`). Rồi `git --no-optional-locks status --porcelain=v1 -z --untracked-files=all`.
   - Phân tích đầu ra `-z`: tách theo `\0`; mỗi mục có tối thiểu 4 ký tự `XY<space>path`; mục có `X` hoặc `Y` là `R` hoặc `C` thì mục kế tiếp là đường dẫn nguồn, bỏ qua. Đường dẫn giữ nguyên byte (không dịch dấu nháy, vì `-z` không quote).
   - Với mỗi đường dẫn bẩn còn tồn tại: `fs.lstat` lấy `size`, `mtimeMs`; không tồn tại (xoá) thì `size: null, mtimeMs: null`.
   - Mode `directory`: đi bộ đệ quy tối đa 5000 mục bằng `fs.readdir(withFileTypes)`, KHÔNG theo symlink, bỏ thư mục `.git`; ghi `size`, `mtimeMs`; quá 5000 thì `truncated: true`.
3. `diffSnapshots(before, after): ChangeReport`:
   - Một trong hai là `unavailable` thì `{ available: false, reason }` (ưu tiên lý do của `before`).
   - `git`: `headMoved = headBefore !== headAfter`. Với mỗi đường dẫn trong hợp của hai bản đồ, đưa vào `changedFiles` nếu chữ ký (`status|size|mtimeMs`) khác hoặc chỉ có ở một bên.
   - Ánh xạ `change` từ mã XY của bản `after` (hoặc `before` nếu chỉ có ở `before`): `??` thì `untracked`; chứa `R` hoặc `C` thì `renamed`; chứa `D` thì `deleted`; chứa `A` thì `added`; còn lại `modified`.
   - Sắp xếp theo `path` (ổn định, dễ test); cắt ở 2000 và đặt `truncated: true`.
   - `directory`: so `size`+`mtimeMs`; có ở `after` không ở `before` thì `added`; ngược lại `deleted`; khác chữ ký thì `modified`.
4. Mọi lỗi không mong đợi được bắt và trả `unavailable SNAPSHOT_FAILED`; không ném ra ngoài (`changes` là thông tin bổ sung, không được làm hỏng lần chạy chính).
5. Không đọc nội dung file (chỉ `lstat`); không log đường dẫn ở mức info (có thể nhạy cảm), chỉ log số lượng.
6. Không thêm lệnh Git nào khác ngoài ba lệnh trên.

## Kiểm thử

`agent-worktree-change-snapshot.test.ts` dùng repo thật trong thư mục tạm (`git init`, đặt `-c user.name=t -c user.email=t@t` ở từng lệnh commit, đặt `GIT_CONFIG_GLOBAL` trỏ tệp rỗng của thư mục tạm để không phụ thuộc cấu hình máy; bỏ qua toàn bộ `describe` bằng `describe.skipIf(!gitAvailable)` khi thiếu `git`):
- `reports added, modified, deleted, untracked and renamed files after a simulated run` (chụp, sửa cây, chụp, so).
- `reports a file that was already dirty before and edited again` (chữ ký đổi do `size`/`mtimeMs`; ép `mtime` bằng `fs.utimes` để khỏi flaky).
- `does not report a file that was dirty before and is untouched`.
- `reports headMoved when the run created a commit`.
- `reports a path reverted during the run as modified` (hành vi (a)).
- `parses rename entries from -z output and ignores the source path` (dùng `runGit` giả trả chuỗi `-z` dựng tay `R  new.ts\0old.ts\0`).
- `returns available:false NOT_A_GIT_REPO outside a repository`.
- `handles a repository with no commits (headBefore null)`.
- `caps changedFiles at 2000 and sets truncated` (dùng `runGit` giả trả 2500 mục).
- `returns SNAPSHOT_TIMEOUT when git hangs past the limit` (dùng giờ giả `vi.useFakeTimers`).
- `directory mode walks at most 5000 entries and does not follow symlinks` (Windows: bỏ qua phần symlink).
- `does not read file contents` (khẳng định `deps.statFile` chỉ được gọi, không có `readFile` trong deps).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-worktree-change-snapshot.test.ts`; rồi `pnpm test`. Git thật có trên máy chạy test chưa kiểm chứng (các `git-handler*.test.ts` hiện có cho thấy repo đã dựa vào Git ở test, nhưng chưa đọc kỹ cách chúng dựng).

## Tiêu chí hoàn thành

- [x] `added/modified/deleted/untracked/renamed` đúng trên repo thật; `headMoved` đúng khi có commit.
- [x] Ngoài repo Git thì `available:false, reason:'NOT_A_GIT_REPO'`, không ném.
- [x] Chỉ ba lệnh Git đã nêu được gọi, `--no-optional-locks` đứng trước `status`.
- [x] Giới hạn 2000 đường dẫn và 5000 mục thư mục được thực thi và có `truncated`.
- [x] Không đọc nội dung file; không log đường dẫn ở mức info.

## Rủi ro và lưu ý

- File bị `.gitignore` không thấy; thay đổi ngoài `cwd` (ví dụ `$HOME`) không thấy. Đây là phát hiện, không hoàn tác; test ghi nhận giới hạn này bằng một `it` mô tả (`known limitation: ignored files are invisible`).
- `git status --untracked-files=all` trên repo có hàng trăm nghìn file untracked (ví dụ `node_modules` chưa ignore) có thể chậm; mốc 30 giây chưa đo.
- Worktree liên kết, submodule, sparse checkout, `core.fsmonitor`: chưa thử. `status` có thể báo submodule bẩn như một mục duy nhất.
- Độ phân giải `mtimeMs` khác nhau theo hệ tệp (một giây trên một số hệ tệp mạng); sửa nhanh trong cùng giây với cùng kích thước có thể không thấy; chấp nhận, nêu rõ cho CR-REQ-008 (không dùng `changes` làm bằng chứng duy nhất).
- Repo tên đường dẫn có ký tự Unicode: `-z` không quote nên đúng; test thêm một ca tên tiếng Việt.

## Không làm trong task này

- Không nối vào `agent-print-mode-exec.ts` (task 08).
- Không hoàn tác thay đổi, không `git stash`, không `git checkout`, không `git clean`.
- Không đọc nội dung file, không tính băm nội dung (chỉ `size` và `mtimeMs`, đúng như CR).
- Không thêm lệnh Git thứ tư.

## Ghi chú thực thi (điền khi làm)

- Thời gian chụp trên repo lớn thật (số file, thời gian): chưa đo.
- Hành vi với worktree liên kết và submodule: chưa thử.
