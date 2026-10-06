# AG-CV-TASK-070-03: Script chụp fixture `capture-codeintel-fixtures.mjs` và bộ chạy entry TS

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** agent/scripts/capture-codeintel-fixtures.mjs (mới), agent/scripts/esbuild-run-typescript-entry.mjs (mới), `agent/src/relay/codeintel/fixture-capture-plan.ts` (mới), `agent/src/relay/codeintel/fixture-capture-plan.test.ts` (mới)
**Depends on:** 070-01, 070-02; AG-CV-SOL-001/002/003 (hằng argv whitelist)
**Status:** [ ] TODO

## Context

CR-070 §2.4; agent-rpc §11 đặt tên script. Chạy tay, **không** trong CI; chụp lại là PR riêng nêu phiên bản.
Agent không có runner TS (`ls agent/node_modules/.bin` không có `tsx`); `agent/build.mjs` dùng `esbuild`. Do đó script bundle một entry `.ts` vào thư mục tạm rồi `import()` (cách này cũng được AG-CV-SOL-071 dùng lại).
`gitnexus analyze` luôn `--index-only` (PQ-37c). Registry toàn cục `~/.gitnexus/registry.json` theo `HOME` (CR-073 §6 rủi ro): đặt `HOME`/`USERPROFILE` của tiến trình con về thư mục tạm.

## Việc cần làm

1. `esbuild-run-typescript-entry.mjs`: `build({entryPoints,bundle:true,platform:"node",format:"esm",write:false})` → ghi vào `os.tmpdir()` → `import()` → gọi `main(args)`; thoát ≠ 0 nếu lỗi.
2. `fixture-capture-plan.ts`: `export function buildCapturePlan(tool, version): CaptureStep[]` dựng từ **hằng argv của whitelist** (import từ module của 001/002/003); không có chuỗi lệnh thứ hai. Mỗi bước: tên tệp ra, argv mẫu (`<repo>`, `<tmp>`), có ghi stderr/exit code không.
3. Script: kiểm `--version` khớp `--tool gitnexus@1.6.9`; sao `mini-repo` ra tmp, `git init`, hai commit; dựng chỉ mục trong tmp; chạy plan; che đường dẫn/thời gian; gọi `scanTextForLeaks`; ghi `MANIFEST.json` (kèm `captureEnv`); in `git diff --stat`. Cảnh báo nếu mtime `~/.gitnexus/registry.json` thật đổi.
4. Cờ `--bench <dir>` chỉ ghi ra ngoài git (dành chỗ cho AG-CV-SOL-071).

## Kiểm thử

- `fixture-capture-plan.test.ts`: mọi `argv[0]` trong plan thuộc whitelist hợp đồng §9.1; không `analyze` ngoài bước dựng chỉ mục (và có `--index-only`); không đường dẫn tuyệt đối thật trong argv mẫu.
- Thủ công (CHƯA CHẠY): chạy script với công cụ thật trên máy phát triển, kiểm `git diff --stat`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Plan dựng từ hằng whitelist; script từ chối phiên bản lệch; chỉ mục chỉ trong tmp, HOME cô lập; fixture đầu tiên của hai công cụ được commit (đủ danh sách solution 2.3).
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Chưa biết tên lệnh `codegraph init/index` và tác dụng phụ của `analyze --index-only` lên registry.
- `esbuild` có thể không phân giải từ `agent/scripts/` dưới pnpm: dùng đường dẫn tương đối nếu cần (chưa kiểm).
