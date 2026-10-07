# AG-CV-TASK-081-17: Handler `quality.listProfiles` (`ready`, `missing`, `host`, `limits`, `suites`)

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.1,5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-list-profiles.ts` (mới), `.test.ts` (mới); điền dòng trong `quality-method-table.ts`
**Depends on:** AG-CV-TASK-081-08, 081-13, 081-16; AG-CV-TASK-080-04
**Status:** [x] DONE

## Context

Hợp đồng §5.1: không lỗi khi thiếu môi trường; timeout agent 40 s (preflight ≤ 10 s); lọc cờ tenant là việc backend (agent luôn liệt kê `security-*`).

## Việc cần làm

1. Lấy catalog đã áp L2, chạy `preflightProfile` song song (≤ 4, mỗi cái ≤ 10 s; quá hạn → `ready:false`, `missing:[]` kèm không lỗi).
2. Trả `{ profiles:[{id,title,kind,scopes,heavy,ready,missing,definitionHash,source,display}], suites:[{id,profiles}], host: readHostSnapshot(), limits:{maxConcurrentRuns, queueMax, runTimeoutMs} }`.
3. Không có argv đầy đủ, env, đường dẫn tuyệt đối; `display` từ `displayOf`.
4. Trả lỗi chỉ cho `INVALID_PARAMS`/`PATH_NOT_ALLOWED`/`TOOL_UNAVAILABLE`.

## Kiểm thử

Repo mẫu giả; bin thiếu → `ready:false` có `missing`; JSON không chứa chuỗi đường dẫn tuyệt đối của worktree (test regex); `definitionHash` khớp task 10; preflight treo → vẫn trả trong < 40 s. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-list-profiles.test.ts`.

## Tiêu chí hoàn thành

- [x] Đúng hình dạng §5.1; không lỗi khi thiếu môi trường.
- [x] Không rò đường dẫn/argv.

## Rủi ro

Preflight song song làm tăng tải khi máy bận; giới hạn 4.
