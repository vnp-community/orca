# AG-CV-TASK-082-02: Kiểu parser và ánh xạ đường dẫn về gốc repo

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.1,5.2
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-types.ts`, `quality-repo-path-mapping.ts` (mới) + `quality-repo-path-mapping.test.ts`
**Depends on:** không
**Status:** [ ] TODO

## Context

CR-082 2.4, 2.6. Công cụ in đường dẫn tương đối `cwd` của profile; `file` ra RPC luôn tương đối gốc repo.

## Việc cần làm

1. Kiểu `QualityParserInput { stepId; stdoutPath; stderrPath; extraPath?; exitCode; timedOut; cancelled; cwd; repoRoot; platform; toolVersion; scopeFiles; readSourceLine }`, `QualityParserOutput { findings: RawQualityFinding[]; failure: null | {kind:"format_drift"|"parser_error"|"env"; envReason?; detail}; stats }`, `QualityParser { key; parse }` (5.2). `detail` không chứa đầu ra thô.
2. `toRepoRelative(printed, cwd, repoRoot, platform) → { file; outside }`: tuyệt đối thì giữ, tương đối thì `path.resolve(cwd, p)`; `path.relative(repoRoot, abs)` đổi `\` → `/`; bắt đầu `..` hoặc khác ổ → `file:""`, `outside:true`; không dùng `realpath` từng phát hiện; so khớp scope chuẩn hoá theo nền.
3. Bộ đếm `outsideRepoCount` do pipeline cộng.

## Kiểm thử

Bảng ca POSIX và Windows giả (`path.win32`): tương đối `internal/x.go` với `cwd=backend-go/services/a`; tuyệt đối trong repo; module cache Go ngoài repo; `..`; symlink thoát (giả `realpath` không dùng); tên có khoảng trắng/Unicode. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-repo-path-mapping.test.ts`.

## Tiêu chí hoàn thành

- [ ] `file` không bao giờ tuyệt đối hay chứa `..`.

## Rủi ro

macOS/Windows hoa-thường chưa kiểm chứng.
