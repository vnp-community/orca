# AG-CV-TASK-037-06: `unusedExports` (Function export không có cạnh vào) → `SymbolRef`

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-unused-exports.ts` (mới), mẫu trong `codeintel-structural-facts-queries.ts`, `.test.ts`
**Depends on:** AG-CV-TASK-037-01, 037-02, AG-CV-SOL-002 (`codeintel-symbol-ref.ts`)
**Status:** [ ] TODO

## Context

CR-037 2.2: `Function` có `isExported=true` và `NOT EXISTS { MATCH (x)-[r:CodeRelation]->(f) WHERE r.type IN [CALLS, ACCESSES, IMPORTS] }`; loại `_test.go`, `/cmd/`, `usecasetest`. Chỉ `Function` (không `Method`); mặc định Go ở `backend-go/services/` do `pathPrefixes`.

## Việc cần làm

1. Mẫu theo fixture task 01; trả cột `id, name, label, filePath, startLine, endLine` đủ cho `SymbolRef`.
2. Chuyển bằng `toSymbolRef` (SOL-002): `key`, 1-based (+1 dòng GitNexus), `qualifiedName` chuẩn hoá; va chạm khoá → `#arity`/`#L<line>` như SOL-002.
3. Sắp theo `filePath`, `startLine`, `name`; phân trang.
4. Không đọc nội dung nguồn; không lọc ngôn ngữ (backend quyết qua `pathPrefixes`).
5. Ghi `warnings:["kind_label_unreliable"]`? **Không** (chỉ ghi trong tài liệu hàm); backend gắn `confidence:"medium"`.

## Kiểm thử

Fixture: hàm không ai gọi xuất hiện, hàm được gọi vắng, `_test.go`/`cmd` bị loại; `SymbolRef` 1-based đúng; va chạm khoá. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-unused-exports.test.ts`.

## Tiêu chí hoàn thành

- [ ] `rows[].symbol` là `SymbolRef` đầy đủ trường bắt buộc; không chứa nội dung mã.

## Rủi ro

Phân loại `kind` sai của GitNexus (biến lỗi mang nhãn Function); gọi qua interface/reflection không có cạnh → báo nhầm.
