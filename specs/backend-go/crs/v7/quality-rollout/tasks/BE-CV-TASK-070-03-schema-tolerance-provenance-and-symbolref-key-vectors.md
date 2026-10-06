# BE-CV-TASK-070-03: Dung sai schema, dấu vết phiên bản và vector khoá `SymbolRef`

**From Solution:** BE-CV-SOL-070
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/symbolref-key-vectors.json` (mới, dùng chung với agent), `.../internal/usecase/agent_result_schema_test.go` (mới), `.../internal/domain/symbol_ref_key_vectors_test.go` (mới)
**Depends on:** BE-CV-TASK-070-02, BE-CV-SOL-020
**Status:** `[ ] TODO`

---

## Context

- PQ-20: cùng hàm xử lý va chạm khoá ở agent và backend (`#<arity>` rồi `#L<startLine>`), **cùng vector kiểm thử** (CR-CV-070).
- agent-rpc §2.2: `sources[].lineBase` luôn 1; thiếu `lineBase` thì backend coi GitNexus 0-based.
- Mã lỗi `CODEINTEL_RESULT_INVALID` do Go sinh khi giải mã thất bại (agent-rpc §3.3).

## Việc cần làm

1. `symbolref-key-vectors.json`: mảng `{name, input: {tool, nativeId, name, qualifiedName, filePath, startLine, arity?, lineBase?}, existingKeys?: [], expectKey, note}` gồm: ca thường; hai symbol cùng tên khác tệp; cùng tệp cùng tên khác arity (`#2`); cùng arity (`#L42`); `::` của CodeGraph so với `.`; Unicode; tên chứa `|`; `lineBase` thiếu (GitNexus 0-based, cộng 1 một lần); `lineBase:1` (không cộng). Tệp do agent test cũng đọc (`AG-CV-SOL-070`).
2. `TestSymbolRefKeyVectors`: chạy từng vector qua hàm khoá của domain (CR-020); khẳng định `expectKey` và không cộng dòng hai lần.
3. `TestAgentResultSchemaVersionTolerance`: (a) thêm trường lạ cấp phong bì và trong `data` thì được bỏ qua; (b) thiếu `sources` hoặc `data` thì `CODEINTEL_RESULT_INVALID` (không panic); (c) kết quả là mảng hoặc chuỗi thì `CODEINTEL_RESULT_INVALID`; (d) `sources[].version` rỗng bị từ chối khi `tool` có mặt.
4. `TestToolVersionRecorded`: sau chuẩn hoá mọi kết quả còn `sources[].version`, `indexedAt`, `commit` (codegraph `commit` là `null`, không phải chuỗi rỗng).

## Kiểm thử

- Ba test trên, chạy `go test ./internal/domain/... ./internal/usecase/... -run 'Vectors|Tolerance|ToolVersion'`.
- Không DB.

## Tiêu chí hoàn thành

- [ ] Vector chạy xanh ở Go; tệp vector được tham chiếu trong `MANIFEST.json`.
- [ ] Ca `lineBase` thiếu và `lineBase:1` cùng có mặt.
- [ ] Lỗi giải mã không bao giờ panic.

## Rủi ro và lưu ý

- Hàm khoá nằm ở CR-020; nếu ngữ nghĩa va chạm đổi, đổi vector và báo `AG-CV-SOL-070` cùng PR.
- Nội dung `data` ở từng method chưa chạy thật (O-3): chỉ kiểm phong bì ở đây, không kiểm sâu hình dạng `data` ngoài những gì CR-021 giải mã.
