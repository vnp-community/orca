# AG-CV-TASK-081-07: Kho kết quả tạm theo run: phân trang `findings|steps|log` (`quality-results-store.ts`)

**From Solution:** [AG-CV-SOL-081-quality-runner-core](../solutions/AG-CV-SOL-081-quality-runner-core.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-results-store.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-02, 081-06
**Status:** [ ] TODO

## Context

Hợp đồng §5.5: trang ≤ 500 mục và ≤ 1 MiB; TTL 1 h; 20 run/worktree; `view=log` ≤ 64 KiB đã che, cần `stepId`. Dữ liệu vào từ pipeline SOL-082 (đã sắp, đã che, đã fingerprint).

## Việc cần làm

1. `createResultsStore(deps: { now; ttlMs; maxRunsPerWorktree; redact })` với `put(root, runId, { steps: QualityStepResult[]; findings: QualityFinding[]; logPaths: Record<stepId,{stdout,stderr}> })`, `getFindings(root, runId, {offset,limit})`, `getSteps(...)`, `getLog(root, runId, stepId, offset)`.
2. Findings trả `{view:"findings", totalCount, truncated, outsideScopeCount, nextOffset|null, items}`; cắt trang theo 500 mục hoặc 1 MiB JSON (cái nào tới trước), `nextOffset` đúng.
3. `view=log`: đọc vùng `[offset, offset+64KiB+512)` của stdout rồi stderr nối, cắt ở dòng, che bằng `redact`, `nextOffset` hoặc `null`; thiếu `stepId` → INVALID_PARAMS ở tầng RPC.
4. TTL và LRU theo worktree (xoá thư mục tạm của run bị loại).
5. Run `interrupted`/đã hết TTL: findings/steps trả trang rỗng hợp lệ (quyết định của solution, câu hỏi mở 3); `runId` không thuộc `root` → `RUN_NOT_FOUND`.
6. `limit` ngoài 1..500 hoặc `offset<0` do tầng RPC từ chối.

## Kiểm thử

Test 1200 phát hiện phân trang 500+500+200; mục lớn làm trang chạm 1 MiB trước 500; hết TTL (đồng hồ giả); 21 run → run cũ nhất bị loại; `view=log` có secret giả bị che và không cắt giữa dòng; `outsideScopeCount`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-results-store.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không trang nào vượt 1 MiB; phân trang không mất/trùng mục.
- [ ] Log luôn đi qua bộ che.

## Rủi ro

Che theo trang có thể cắt đôi một secret ở biên: đọc dư 512 byte và cắt ở dòng để giảm rủi ro; chưa loại bỏ hoàn toàn.
