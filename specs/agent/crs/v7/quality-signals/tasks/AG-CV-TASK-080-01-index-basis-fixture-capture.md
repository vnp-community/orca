# AG-CV-TASK-080-01: Thu fixture thật cho `codegraph status -j`, `gitnexus status`, `meta.json` (worktree chính và liên kết)

**From Solution:** [AG-CV-SOL-080-index-basis-and-reindex-triggers](../solutions/AG-CV-SOL-080-index-basis-and-reindex-triggers.md) mục 4, 9
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/index-basis/` (mới, gồm `MANIFEST.json` và các tệp thô), `agent/scripts/capture-index-basis-fixtures.mjs` (mới)
**Depends on:** không (làm đầu tiên; cơ chế `MANIFEST.json` theo CR-CV-070 do AG-CV-SOL-070 sở hữu, nếu chưa có thì dùng đúng khung tối thiểu ghi trong "Việc cần làm")
**Status:** [ ] TODO

## Context

Mọi hình dạng đầu ra của `codegraph status -j` (`pendingChanges`, `worktreeMismatch`, `index.state`, `lastIndexed`), `gitnexus status` và `.gitnexus/meta.json` trong CR-CV-080 mục 1.1 chỉ là quan sát một lần của người soạn CR, **chưa được chạy lại** (hợp đồng đầu trang: "Trạng thái: Proposed"). `classifyIndexBasis` (task 02) và probe (task 03) phải được viết theo dữ liệu thật, đặc biệt: (a) ở worktree liên kết `pendingChanges` có đúng là của checkout chính không, (b) `codegraph status -j` có trường commit nào không (hợp đồng §2.2 nói `commit` luôn `null`), (c) `meta.json` có `lastCommit` và `indexedAt` như CR nêu.

Task này **chỉ thu dữ liệu**, không viết logic. Lệnh chỉ đọc; **không** chạy `analyze`, `sync`, `index`, `init` ở repo Orca. Phép thử ghi (`analyze`, `sync`) nếu cần chạy trên **repo mẫu nhỏ trong thư mục tạm**, không trên `/opt/repos/orca` (CR-080 mục 2.7).

## Việc cần làm

1. Tạo repo git mẫu nhỏ trong thư mục tạm (`fs.mkdtemp`), 2 commit, 1 `git worktree add` liên kết. Trên checkout chính: `gitnexus analyze --index-only <path>` và `codegraph init`/`index` (chỉ trên repo mẫu; cần cả hai công cụ có trên máy; nếu thiếu thì ghi rõ trong `MANIFEST.json` `skipped` và dừng).
2. Thu các đầu ra sau (đã che đường dẫn tuyệt đối thành `<repo>`, `<worktree>`): `codegraph status <path> -j` ở checkout chính (sạch), sau khi sửa 1 tệp chưa commit, ở worktree liên kết (sạch và sau khi sửa tệp trong worktree); `gitnexus status` ở cả hai; nội dung `.gitnexus/meta.json`; `~/.gitnexus/registry.json` đã lọc còn mục của repo mẫu.
3. `MANIFEST.json` mỗi thư mục ghi: `tool`, `toolVersion`, `argv`, `capturedAt`, `cwdKind` (`main|linked`), `gitHead`, băm sha256 từng tệp.
4. `capture-index-basis-fixtures.mjs`: chạy được lại; dừng nếu `--version` khác phiên bản đích trong `MANIFEST`; không bao giờ trỏ vào repo thật.
5. Ghi vào PR: bảng "điều quan sát được" trả lời (a), (b), (c) ở Context và 2 câu hỏi M1 (`sync` ở worktree đọc cây nào) nếu chạy được trên repo mẫu.

## Kiểm thử

- `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/index-basis-fixtures.test.ts` (test mới): mọi tệp trong `__fixtures__/index-basis/` có mục trong `MANIFEST.json`, băm khớp, mỗi tệp ≤ 20 KiB, không chứa chuỗi `/home/`, `/opt/repos`, `/tmp/` (đã che).
- Chạy script thủ công một lần; không có test chạy công cụ thật trong CI.

## Tiêu chí hoàn thành

- [ ] Có fixture cho 4 tình huống: checkout chính sạch / bẩn, worktree liên kết sạch / bẩn.
- [ ] PR ghi rõ điều quan sát được về `pendingChanges` ở worktree liên kết và về trường commit của CodeGraph; nếu **khác** CR-080 thì cập nhật mục 5.2/5.3 của solution trước khi làm task 02.
- [ ] Không lệnh ghi nào chạy ngoài repo mẫu tạm.

## Rủi ro

- Máy không có `gitnexus`/`codegraph`: task chuyển `BLOCKED`, tránh dựng fixture tay (sẽ biến giả định thành "sự thật").
- `gitnexus analyze` trên repo mẫu cần thời gian và RAM; đặt `GITNEXUS_WORKER_POOL_SIZE=1`.
- Phiên bản công cụ trên máy khác 1.6.9/1.4.1: ghi đúng phiên bản vào `MANIFEST`, không sửa số.
