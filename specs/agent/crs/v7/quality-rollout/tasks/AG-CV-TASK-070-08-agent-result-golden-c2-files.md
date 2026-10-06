# AG-CV-TASK-070-08: Tệp vàng C2 `testdata/agent-results/` và `TestResultMatchesServiceGolden`

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/agent-result-golden.test.ts` (mới); `backend-go/services/code-intel-service/testdata/agent-results/**` (mới, ghi bởi test)
**Depends on:** 070-04, 070-05, 070-06, 070-07; AG-CV-SOL-001 (`buildCodeIntelResult`), 002, 003, 005; BE-CV-TASK-070-01
**Status:** [ ] TODO

## Context

Cổng G1 (proto-map §7.1). Bố cục và tên tệp **theo BE-CV-SOL-070 mục 5.1** (solution AG-070 mục 2.5): `MANIFEST.json`, `gitnexus-1.6.9/`, `codegraph-1.4.1/`, `merged/`, `errors/`. Go không có cờ `-update`; chỉ agent ghi.
Agent-rpc §8: so sánh bỏ `perf`, `startedAt`. `indexedAt`, `headCommit` cố định.

## Việc cần làm

1. Test dựng kết quả `codeintel.*` từ fixture C1 bằng đường code thật (đồng hồ/`headCommit` tiêm), chuẩn hoá khoá sắp xếp, thụt 2 dấu cách, `\n` cuối.
2. Chế độ mặc định: so với tệp; `ORCA_UPDATE_GOLDEN=1`: ghi tệp và `MANIFEST.json` (sha256, bytes, phiên bản công cụ, commit `mini-repo`, hợp đồng áp dụng). Ghi vào cây `backend-go/` bằng đường dẫn tương đối từ `import.meta.dirname`.
3. Phủ: status (repo_root stale; codegraph ready), overview, processes, process, subgraph, impact found/ambiguous, symbol found/not-found, routes, detectChanges, structuralFacts, codegraphSearch, files, `merged/symbol-two-sources.json`; `errors/*` theo danh sách BE (phong bì lỗi JSON-RPC `{code,message,data{code,…}}`).
4. Test phụ: không tệp nào chứa `perf`; không `codeintel.node`; mỗi tệp ≤ 20 KiB; quét rò rỉ (task 02).

## Kiểm thử

- Chạy thường (so sánh) xanh; chạy `ORCA_UPDATE_GOLDEN=1` hai lần cho cùng kết quả (idempotent).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Tệp vàng do agent sinh, BE đọc được; thay đổi hình dạng làm đỏ cả hai phía.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Cần thống nhất với BE-CV-TASK-070-01 về tên tệp trước khi ghi lần đầu.
- Nếu BE đã tạo tệp tay thì tệp của agent thắng (D2 của BE).
