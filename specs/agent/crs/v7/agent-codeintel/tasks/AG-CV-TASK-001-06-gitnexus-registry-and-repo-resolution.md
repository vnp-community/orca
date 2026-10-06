# AG-CV-TASK-001-06: Đọc registry GitNexus và phân giải `workspaceRoot` -> repo

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.5
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-registry-reader.ts` (mới), `agent/src/relay/codeintel-git-exec.ts` (mới), `agent/src/relay/codeintel-repo-resolution.ts` (mới), và test cùng tên (`gitnexus-registry-reader.test.ts`, `codeintel-repo-resolution.test.ts`)
**Depends on:** [01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md)
**Status:** [ ] TODO

## Context

Contract §9 mục 2 chốt thứ tự phân giải. Đã đọc: `git-handler.ts:1514-1542` (`readRepoLocation`: `GitCapabilityCache.runWithFallback('rev-parse-path-format', …)` + `hasUnsupportedRevParsePathFormatEcho`), `shared/git-worktree-command-capabilities.ts:26,32` (hai hàm nhận diện lỗi/echo đã xuất), `shared/git-capability-cache.ts` (`runWithFallback`, `rememberUnsupported`, retry 30 phút), `context.ts:31-33` (`registerRoot` no-op: agent **không có** gốc workspace đăng ký). `~/.gitnexus/registry.json` là mảng JSON mỗi phần tử `name, path, storagePath, indexedAt, lastCommit, remoteUrl, stats, branch` (CR-001 1.4, 12 repo trên máy khảo sát; **định dạng nội bộ GitNexus 1.6.9, không có cam kết**). `.gitnexus/meta.json` cũng có `lastCommit`, `indexedAt` ở cấp gốc (đã thấy 2026-10-06) nhưng nặng 2,8 MB: không đọc ở đây.

AGENTS.md "Git Binary Compatibility": baseline 2.25; dùng `GitCapabilityCache` với predicate hẹp; kiểm thử lần dự phòng đầu, lần sau dùng cache, probe đồng thời, cô lập theo máy chạy.

## Việc cần làm

1. `codeintel-git-exec.ts`: `runGit(args, cwd, {timeoutMs, signal, maxBuffer})` bằng `execFile('git', …)` (không shell), env `process.env` nhưng thêm `GIT_OPTIONAL_LOCKS=0`, `LC_ALL=C` cho lệnh đọc; trả `{stdout, stderr}` hoặc ném lỗi giữ `code`. Dùng lại cho SOL-005. **Không** import `agent-git-handler-extended.ts` (cây `desktop/` không có file đó; lõi phải sao chép được).
2. `gitnexus-registry-reader.ts`: `readGitNexusRegistry(config)` đọc `path.join(config.toolEnv.HOME ?? os.homedir(), '.gitnexus', 'registry.json')`; kiểm `Array.isArray`, từng phần tử có `path` string tuyệt đối; cache theo `mtimeMs`; hỏng/không phải mảng -> `CodeIntelError('CODEINTEL_TOOL_FAILED', …, {tool:'gitnexus', reason:'registry_unreadable'})`; tệp không tồn tại -> `[]` (không lỗi). `findRegistryEntry(entries, realToplevel)`: so `realpath(entry.path) === realToplevel` (đường dẫn chính xác, không khớp tiền tố/tên); nhiều mục trùng -> `indexedAt` mới nhất + cảnh báo `registry_duplicate_path`.
3. `codeintel-repo-resolution.ts`: `resolveCodeIntelRepo(workspaceRoot, config, deps)` theo thứ tự: (a) hình dạng (đã ở task 01); (b) `realpath.native` (không tồn tại/không phải thư mục -> `PATH_NOT_ALLOWED` không lộ `ENOENT`); (c) `ORCA_CODEINTEL_ALLOWED_ROOTS` (`path.delimiter`; rỗng = không giới hạn); (d) `rev-parse` qua `GitCapabilityCache` (một instance module `getCodeIntelGitCapabilities()`; bản ưu tiên có `--path-format=absolute`, bản dự phòng bỏ cờ và `path.resolve(workspaceRoot, line)` cho `git-common-dir`); `toplevel` (realpath) khác `workspaceRoot` -> `PATH_NOT_ALLOWED data.hint='workspaceRoot must be a git work tree root'`; (e) `git worktree list --porcelain` mục `worktree <path>` đầu tiên = checkout chính (không `-z`); `toplevel !== main` -> `linkedWorktree=true`; (f) khớp registry theo `toplevel`, rồi theo checkout chính nếu liên kết (khớp ở đó -> `worktreeMismatch=true`); (g) CodeGraph: thư mục chứa `.codegraph/codegraph.db` (toplevel rồi checkout chính); (h) không có cả hai -> `REPO_NOT_REGISTERED data.hint='run codeintel.reindex'`.
4. Cache kết quả 30 s theo `workspaceRoot`; `invalidateRepoBindings(workspaceRoot?)` xuất cho SOL-004; cache không giữ lỗi.
5. Trả `CodeIntelRepoBinding` (solution 2.5). Không spawn bất kỳ công cụ GitNexus/CodeGraph nào ở đây.

## Kiểm thử

`gitnexus-registry-reader.test.ts`: registry hợp lệ; rỗng `[]`; tệp vắng; hỏng; không phải mảng; trùng `path` (chọn `indexedAt` mới); đường dẫn qua symlink; phần tử thiếu `path`.
`codeintel-repo-resolution.test.ts` (repo git thật trong thư mục tạm, `git init`; `HOME` giả chứa registry giả 2 repo): gốc worktree; thư mục con -> `PATH_NOT_ALLOWED`; không phải git; tương đối/NUL (từ task 01); ngoài allowed roots; worktree liên kết (`git worktree add`) -> `linkedWorktree:true`, `worktreeMismatch:true` khi chỉ mục ở checkout chính; repo trong registry khác không bị chạm; không có registry và không `.codegraph` -> `REPO_NOT_REGISTERED`; hai nhánh `rev-parse-path-format` (bản ưu tiên; bản dự phòng bằng cách giả `git` in lại cờ lạ theo mẫu `git-handler-worktree-git-capabilities.test.ts` nếu có; **chưa kiểm tên file mẫu**), lần gọi thứ hai dùng cache, hai probe đồng thời chỉ spawn một lần, hai `GitCapabilityCache` riêng không chia sẻ trạng thái; cache 30 s (đồng hồ giả) và `invalidateRepoBindings`.

Lệnh: `pnpm exec vitest run src/relay/gitnexus-registry-reader.test.ts src/relay/codeintel-repo-resolution.test.ts`

## Tiêu chí hoàn thành

- [ ] Mọi nhánh lỗi không spawn công cụ; không lộ `ENOENT`/stack.
- [ ] Khớp registry chỉ theo đường dẫn chính xác sau `realpath`.
- [ ] Chỉ dùng lệnh Git <= 2.25 (kèm dự phòng `--path-format`).
- [ ] Worktree liên kết cho `worktreeMismatch:true`.

## Rủi ro và lưu ý

- Registry có thể đổi vị trí bằng biến môi trường của GitNexus (chưa kiểm chứng tên biến); agent chạy dưới user khác có `HOME` khác.
- `realpath.native` chuẩn hoá hoa/thường trên macOS/Windows: chưa chạy thử; Windows/WSL/UNC bị chặn (O-14).
- Test bằng `git worktree add` cần git >= 2.5; chạy với git baseline 2.25 nếu CI có ma trận (chưa kiểm tên job).
