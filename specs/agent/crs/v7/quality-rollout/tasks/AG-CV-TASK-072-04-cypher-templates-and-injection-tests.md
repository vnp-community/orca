# AG-CV-TASK-072-04: Mẫu Cypher chỉ-đọc, bộ mã hoá và vector tiêm

**From Solution:** [AG-CV-SOL-072-security-tests-agent](../solutions/AG-CV-SOL-072-security-tests-agent.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/security-cypher.test.ts` (mới), `agent/src/relay/codeintel/__fixtures__/cypher-injection-vectors.json` (mới)
**Depends on:** 072-01; AG-CV-SOL-002
**Status:** [ ] TODO

## Context

Agent-rpc §2.4: `assertReadOnlyCypher` (≤ 16 KiB, một câu, bắt đầu `MATCH `, cấm `CREATE|MERGE|DELETE|SET|REMOVE|DROP|ALTER|COPY|DETACH|CALL|LOAD|INSTALL|ATTACH|EXPORT|IMPORT|FOREACH|UNWIND`), bốn bộ mã hoá. CLI không có tham số ràng buộc; `CALL show_tables()` vẫn chạy trên công cụ (CR-072).
Từ cấm nằm **trong literal** (symbol tên `Set`, `Import`) có thể bị chặn nhầm: cần quyết định (solution câu hỏi 5); test ghi hành vi thực của 002 và đánh dấu.
Test viết theo hợp đồng; mã bị test thuộc AG-CV-SOL-001/002/003/004/081 (chưa tồn tại): import qua hằng đường dẫn ở đầu tệp. Chưa chạy.

## Việc cần làm

1. Test từng mục bảng solution 2.5: mọi mẫu qua `assertReadOnlyCypher`; vector độc (`' OR 1=1 //`, `'}) MATCH (x) DETACH DELETE x //`, `CALL show_tables()`, `;`, `/*`, NUL, xuống dòng) bị từ chối hoặc thoát đúng và chuỗi chuẩn hoá (literal→`?`) bằng mẫu gốc.
2. Validator tham số (`processId`, `uid/center`, `kinds`, `limit`, `depth`), `-l` và `LIMIT` luôn có, không còn `{{` sau thay.
3. Vector ở JSON để tầng live (đêm) dùng lại.

## Kiểm thử

- Như mục 2; live (không chạy PR): gửi vector tới `gitnexus cypher` thật trên `mini-repo` → `{error}`/`[]`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Mọi vector bị chặn; không mẫu nào có từ cấm ngoài literal.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Hành vi `-l` và `LIMIT` trên công cụ thật chưa kiểm.
