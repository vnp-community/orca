# AG-CV-TASK-002-05: `SymbolRef` chuẩn hoá và khoá

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.4
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-symbol-ref.ts`, `codeintel-symbol-ref.test.ts` (mới)
**Depends on:** không (SOL-001 chỉ cho kiểu `warnings`)
**Status:** [x] DONE

## Context
Contract §2.6, PQ-20: agent chuẩn hoá (`key`, dòng 1-based, `qualifiedName` `::`→`.`, NFC, bỏ `#n` lưu `ordinal`, `lineBase 1`). GitNexus 0-based (`runToolCommand` thật 72-113, GitNexus báo 71-112). File tạo ở đây với bảng kind đầy đủ cả CodeGraph (CR-003 chỉ thêm hàm).

## Việc cần làm
1. Bảng kind (§2.6): function, method, type, value, file, folder, route, component, namespace, import, cluster, flow, doc; kind lạ -> `value` + `nativeKind` + `unknown_native_kind`.
2. `buildSymbolRef` từ cột GitNexus (+1 dòng); `normalizeKey`; `assignUniqueKeys(refs)` xử lý va chạm `#<arity>` rồi `#L<startLine>` (+ `key_collision`); id `Section`, cluster, flow, file theo quy tắc; chịu khoảng trắng, `:` và Unicode.
3. `parseGitNexusId(id)` -> `{label, filePath, qualified, ordinal}`.
4. Xuất hằng vector kiểm thử dùng chung với backend (CR-070).

## Kiểm thử
`Method:agent/src/relay/context.ts:RelayContext.registerRoot#1` -> key `method:agent/src/relay/context.ts:RelayContext.registerRoot`; `Method:…project.pb.go:ListReposResponse.GetRepos#0`; hai `handler#1/#2`; `Section:CLAUDE.md:L23:GitNexus — Code Intelligence`; `RelayContext::registerRoot`; mọi nhãn trong bảng; +1 dòng.
Lệnh: `pnpm exec vitest run src/relay/codeintel-symbol-ref.test.ts`

## Tiêu chí hoàn thành
- [x] Mọi nhãn ánh xạ; không cộng dòng hai lần.

## Rủi ro
- `arity` chưa có nguồn (solution mục 8); chỉ `#L<startLine>` nếu không tính được.
