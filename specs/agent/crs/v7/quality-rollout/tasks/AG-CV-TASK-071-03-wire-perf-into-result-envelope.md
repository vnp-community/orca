# AG-CV-TASK-071-03: Cắm `PerfRecorder` vào runner/envelope; `perf` không vào cache và không vào tệp vàng

**From Solution:** [AG-CV-SOL-071-perf-block-and-bench](../solutions/AG-CV-SOL-071-perf-block-and-bench.md)
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/perf-envelope-integration.test.ts` (mới); sửa nhỏ runner/`buildCodeIntelResult` của AG-CV-SOL-001
**Depends on:** 071-01, 071-02; AG-CV-SOL-001
**Status:** [ ] TODO

## Context

Agent-rpc §2.2: `perf` "KHÔNG vào cache snapshot"; §2.3 cache 60 s khoá `(registryPath, indexedAt|lastCommit, method, hash(params))`; §8: so sánh Part A/B trừ `perf`.
Quy tắc đề xuất (hợp đồng thiếu): cache hit → `cliCalls:0`; lỗi không có `perf`.

## Việc cần làm

1. Runner tạo `PerfRecorder` mỗi lời gọi method; `recordQueueWait` từ bộ giới hạn; `recordCli` sau mỗi CLI (với `stdoutBytes`, `rssPeakKb` từ sampler); `recordParse` quanh parser; `markTruncated` khi cắt.
2. `buildCodeIntelResult` thêm `perf` ở phong bì (cùng cấp `data`), áp cho đúng các method dùng phong bì (không áp `reindex*`, `watch`, `quality.*`).
3. Giá trị cache **không** chứa `perf` (so sánh `JSON.stringify` của mục cache với chuỗi `"perf"`); hit dựng `perfForCacheHit`.
4. Giúp so sánh vàng: hàm `stripVolatileResultFields(result)` (tên theo nội dung) bỏ `perf`, `startedAt` — tái dùng bởi AG-CV-SOL-070 task 08.

## Kiểm thử

- `every enveloped method returns a perf object` (lặp theo bảng method)
- `non-enveloped methods have no perf`
- `cache entries never contain perf`
- `cache hit reports cliCalls 0`
- `error responses carry no perf`
- `perf.cli commands are all in the closed set across all methods`

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Test xanh; tệp vàng C2 không có `perf`.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Chạy trên CLI giả; đụng mã của 001 chỉ qua điểm cắm đã định.
