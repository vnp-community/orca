# BE-CV-TASK-020-04: Domain `SymbolRef`, khoá, ánh xạ kind/cạnh, kiểm lại `key` của agent

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/symbol_ref.go`, `symbol_key_normalization.go`, `symbol_kind_mapping.go`, `edge_kind_mapping.go` (mới) và các `_test.go` cùng thư mục
**Depends on:** TASK-020-01 (vector), BE-CV-SOL-010 (module)
**Status:** [ ] TODO

---

## Context

PQ-20: agent chuẩn hoá; backend **kiểm lại** và chỉ cộng +1 khi `line_base` là 0 hoặc vắng. Domain thuần: không import `proto/gen`, `database/sql`, `grpc` (`arch/03`). Tên file theo khái niệm (AGENTS.md).

## Việc cần làm

1. Viết test bảng **trước** từ `testdata/symbol-vectors/symbol-key-vectors.json`.
2. `symbol_ref.go`: `SymbolKind` (13 giá trị, `String()` chữ thường), struct `SymbolRef` (đủ trường hợp đồng agent §2.6 kể cả `Signature, IsExported, Docstring, Ordinal`), `NewSymbolKey(kind, filePath, qualifiedName, name)` → `"<kind>:<filePath>:<qn||name>"`, NFC, không đổi hoa/thường.
3. `symbol_kind_mapping.go`: hai `map` hằng và `KindFromGitNexusLabel`, `KindFromCodeGraphKind` (bảng CR mục 2.4); kind lạ → `(KindValue, unknown=true)` để caller tăng bộ đếm.
4. `edge_kind_mapping.go`: `EdgeKindFromGitNexus`, `EdgeKindFromCodeGraph`; cạnh cấu trúc (`DEFINES, MEMBER_OF, STEP_IN_PROCESS, ENTRY_POINT_OF`) trả `(0, false)` (không thành `SymbolEdge`).
5. `symbol_key_normalization.go`: `SymbolRefFromGitNexus` (bỏ `<Label>:` và `<filePath>:`, bỏ `#<n>` lưu `Ordinal`, cộng 1 dòng khi `lineBase != 1`), `SymbolRefFromCodeGraph` (`::`→`.`; file bỏ `qualifiedName`), `ResolveKeyCollisions` (`#<arity>` rồi `#L<startLine>`, chỉ thêm khi va chạm trong cùng kết quả), `ValidateAgentSymbolRef(in, workspaceRoot, plat)` (dựng lại khoá, khác thì dùng khoá backend + trả cờ `KeyMismatch`).
6. Không phụ thuộc `NormalizeRepoPath` ở task này bằng cách nhận hàm qua tham số (`PathNormalizer func(string) (string, error)`); TASK-020-05 nối vào.

## Kiểm thử

- `cd backend-go/services/code-intel-service && go test ./internal/domain/ -run 'SymbolRef|SymbolKey|KindMapping|EdgeKind' -race`.
- Ca bắt buộc: `Relay.Execute#2` → khoá không hậu tố; `startLine` 33 → 34 (`lineBase 0`) và 34 → 34 (`lineBase 1`); hai `Method` khác arity → hai khoá có `#<arity>`; không va chạm → không hậu tố; property test `NewSymbolKey` idempotent; kind lạ không panic.
- Quét tĩnh: `go list -deps ./internal/domain | grep -E 'proto/gen|grpc|database/sql'` rỗng.

## Tiêu chí hoàn thành

- [ ] Mọi vector dương/âm liên quan khoá và kind xanh.
- [ ] `ValidateAgentSymbolRef` không từ chối khi `key` lệch, chỉ sửa và báo cờ.
- [ ] Domain không import proto/DB.
- [ ] Không file nào tên `helpers/utils/common/misc`.

## Rủi ro và lưu ý

- `arity` của hàm biến tham số do GitNexus tính chưa rõ (CR mục 6); chỉ ảnh hưởng nút va chạm.
- Hậu tố `#L<startLine>` đổi khi dòng đổi: chấp nhận, chỉ cho nút va chạm.
