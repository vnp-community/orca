# AG-CV-TASK-080-02: Hàm thuần `classifyIndexBasis` và bảng test 6 hàng

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 5.2, 5.3
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-index-basis.ts` (mới), `agent/src/relay/codeintel-index-basis.test.ts` (mới)
**Depends on:** AG-CV-TASK-080-01 (dữ liệu thật để chốt ca CodeGraph)
**Status:** [ ] TODO

## Context

`CONTRACT-codeintel-agent-rpc.md` §4.1 định bảng 6 dòng, dừng ở dòng đúng đầu tiên. Hợp đồng chưa nói cách xử lý CodeGraph khi `commit:null` (§2.2): solution đề xuất quy tắc 5.3. Hàm này không đọc đĩa/git; mọi dữ kiện truyền qua `IndexBasisInput`.

## Việc cần làm

1. Tạo kiểu `IndexScope`, `IndexFreshness`, `IndexBasisInput`, `IndexBasisResult` đúng chữ ký ở solution 5.2.
2. Cài `classifyIndexBasis`: hàng 1 `!indexExists`; hàng 2 `!toolUsable`; GitNexus theo hàng 3-6; CodeGraph theo bảng 5.3 (`rootMatches` + `pendingChanges` + `dirtySinceIndex` + `indexedAtMs` so với `headCommitTimeMs`; `!rootMatches` → `repo_root/unknown`).
3. Giá trị thiếu (`headCommit:null`, `indexedCommit:null` ở GitNexus, `mergeBase:null` ở hàng 5) → KHÔNG đoán `fresh`: GitNexus `headCommit:null` → `stale/stale` nếu `rootMatches`, `repo_root/unknown` nếu không.
4. Không import `node:fs`, `node:child_process`; file < 300 dòng.

## Kiểm thử

`codeintel-index-basis.test.ts` (bảng ca, mỗi ca một `it.each` dòng): 6 hàng GitNexus; GitNexus `rootMatches` + `dirtySinceIndex:true` cùng commit → `stale`; `mergeBase:null` ở hàng 5 → `repo_root/unknown`; CodeGraph sạch + `indexedAt >= headTime` → `exact/fresh`; CodeGraph có `pending.modified=1` → `stale`; CodeGraph `!rootMatches` với `pendingChanges:{0,0,0}` (dữ liệu giống CR-080 1.1.2) → `repo_root/unknown`, không `exact`; thuộc tính: với mọi tổ hợp `rootMatches=false` kết quả `indexScope !== 'exact'`.

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-index-basis.test.ts`.

## Tiêu chí hoàn thành

- [ ] Bảng test phủ 6 hàng + ca CodeGraph; thuộc tính "`!rootMatches` không bao giờ `exact`" có test.
- [ ] Không có I/O trong module; không `max-lines` disable.

## Rủi ro

- Nếu task 01 cho thấy `codegraph status -j` có trường commit hoặc `pendingChanges` đúng ở worktree liên kết, quy tắc CodeGraph đổi; cập nhật solution 5.3 và hợp đồng trước (quy ước hợp đồng §8.1).
