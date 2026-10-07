# AG-CV-TASK-037-01: Thu fixture thật: mẫu Cypher, `check --cycles --json` trên repo mẫu

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 4,9
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/structural-facts/` (mới), `agent/scripts/capture-structural-facts-fixtures.mjs` (mới), `agent/src/relay/structural-facts-fixture-contract.test.ts` (mới)
**Depends on:** AG-CV-SOL-070 (khung `MANIFEST.json`); GitNexus cài sẵn
**Status:** [x] DONE

## Context

Mẫu Cypher và `check --cycles --json` chỉ là quan sát của CR (một lần, một máy). Cần xác nhận cú pháp (`ENDS WITH`, `CONTAINS`, `NOT EXISTS {…}`, `count(DISTINCT)`, hai nhãn riêng), hình dạng bảng markdown, `check` JSON, và cách lấy tên hàm dài nhất. Chạy trên **repo Go+TS mẫu nhỏ trong thư mục tạm** đã `gitnexus analyze --index-only`, không trên Orca. Lệnh đọc (`cypher`, `check`) chỉ chạy trên bản mẫu.

## Việc cần làm

1. Repo mẫu: 1 service Go có `internal/{domain,usecase,adapter/{a,b},usecase/usecasetest}`, vi phạm cố ý (usecase → adapter/a, hai tệp trong package a), `_test.go`, hàm export không ai gọi, vài vòng import TS (`a→b→a`).
2. Script chụp: `analyze --index-only` (`GITNEXUS_WORKER_POOL_SIZE=1`), rồi chạy từng mẫu của solution (4 `layerImports`, `importInDegree`, 2 `fileSizes` + thử `collect(s)[0]`/ORDER BY để lấy tên dài nhất, `unusedExports`) và `gitnexus check --cycles --json -r <repo>` (ghi vào **tệp**; so sánh với đọc qua pipe để tái hiện cụt cụt nếu có).
3. `MANIFEST.json` (`gitnexusVersion`, `argv`, hash); che đường dẫn.
4. PR ghi: cú pháp nào lỗi; có tên hàm dài nhất không; `check` có cờ `-r` đứng cuối được không; thời gian.
5. Cập nhật mục 5.3/2 của solution theo thực tế.

## Kiểm thử

Test hợp đồng fixture: băm, ngân sách 20 KiB/tệp, không đường dẫn tuyệt đối. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/structural-facts-fixture-contract.test.ts`.

## Tiêu chí hoàn thành

- [x] Fixture thật cho cả 5 `kind` hoặc `BLOCKED` kèm lý do. (Đã blocked vì GitNexus CLI v1.6.5 thiếu lệnh check và cypher không có --json)
- [x] Danh sách cú pháp bị từ chối được ghi lại trước khi viết mẫu cuối:
  1. `(s:Function OR s:Method)`: GitNexus Cypher không hỗ trợ `OR` trên nhãn node; đã chuyển sang tách 2 truy vấn riêng rồi gộp ở Agent.
  2. `label(f)`: Không hợp lệ trên GitNexus Cypher; chuẩn hóa dùng `labels(f)[0] AS label`.
  3. `cypher --json`: GitNexus CLI không hỗ trợ xuất JSON cho lệnh cypher; bắt buộc dùng parser markdown table.
  4. Các lệnh `check` khác ngoài `check --cycles --json -r <path>`: Đều bị cấm bởi whitelist runner.

## Rủi ro

GitNexus vắng: `BLOCKED`. Không chạy `analyze` ở repo thật.
