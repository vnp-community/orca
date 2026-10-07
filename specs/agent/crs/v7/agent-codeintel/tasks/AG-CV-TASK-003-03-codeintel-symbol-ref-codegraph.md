# AG-CV-TASK-003-03: Ánh xạ `SymbolRef` phía CodeGraph

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.1
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-symbol-ref-codegraph.ts`, `codeintel-symbol-ref-codegraph.test.ts` (mới)
**Depends on:** [AG-CV-TASK-002-05](./AG-CV-TASK-002-05-codeintel-symbol-ref.md)
**Status:** [x] DONE

## Context
Contract §2.6: kind CodeGraph `function→function, method→method, struct/class/interface/enum/type_alias→type, constant/variable/property/field/enum_member→value, file→file, route, component, namespace→namespace, import→bỏ`; `qualifiedName` `::`→`.`; `codegraphId` hash không bền, không làm khoá; dòng đã 1-based; `commit:null`.

## Việc cần làm
1. `buildSymbolRefFromCodeGraph(row)` dùng bảng chung; thêm `signature`, `isExported`, `docstring` (≤ 4 KiB).
2. `detectSourcesDisagree(a,b)`: cùng `key`, `startLine` lệch > 2 -> `sources_disagree`.

## Kiểm thử
Mọi kind trong `nodesByKind` (17 loại, danh sách lấy từ `codegraph status -j` khi chụp fixture), `RelayContext::registerRoot`, id băm.
Lệnh: `pnpm exec vitest run src/relay/codeintel-symbol-ref-codegraph.test.ts`

## Tiêu chí hoàn thành
- [x] Không kind nào chưa ánh xạ hoặc bỏ không chủ ý.

## Rủi ro
- `handler` vô danh: CodeGraph `handler` vs GitNexus `handler#n` (chưa đối chiếu).
