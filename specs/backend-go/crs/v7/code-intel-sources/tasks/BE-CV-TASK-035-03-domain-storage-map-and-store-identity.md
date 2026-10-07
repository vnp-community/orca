# BE-CV-TASK-035-03: Domain `storagemap`: kiểu, `Store.id` ổn định, thứ tự chuẩn, hợp nhất

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/storagemap/storage_map.go`, `store_identity.go`, `canonical_order.go` và các `_test.go` (mới)
**Depends on:** BE-CV-SOL-010 (module `code-intel-service`)
**Status:** [x] DONE

---

## Context

`Store.id = "<kind>:<env>:<name>"` được `DataFlow.stores` của `BE-CV-SOL-034-data-flow-model` tham chiếu, nên quy tắc id là hợp đồng nội bộ giữa hai solution. `domain/` chỉ import stdlib (arch/03); không import proto ở đây (proto ↔ domain ánh xạ ở handler, TASK-035-08).

## Việc cần làm

1. `storage_map.go`: các struct `StorageMap`, `Store`, `Binding`, `Topic`, `SourceRef` (bản domain, không phải proto), kiểu chuỗi có tên cho `StoreKind` (`postgres|mysql|redis|object|volume|vault|queue|other`), `Environment` (`dev|prod|legacy`), `Confidence` (`declared|derived|inferred`), `Access` (`rw|ro`), `Delivery` (`durable|ephemeral|unknown`). Hàm `Validate()` trả lỗi khi `kind`/`env`/`confidence` ngoài tập.
2. `store_identity.go`: `NewStoreID(kind, env, name)` trả `kind:env:name`; chuẩn hoá `name` về chữ thường, thay ký tự ngoài `[a-z0-9._-]` bằng `-`, cắt 128; từ chối `name` rỗng. `ParseStoreID` đảo lại. Test các id mong đợi: `postgres:dev:orca-go-postgres`, `vault:dev:shared-external`, `volume:dev:git-gateway-repos`, `mysql:dev:supported-by-code`.
3. `canonical_order.go`: sắp xếp xác định `Store` theo `(kind, env, name)`, `Binding` theo `(service, store, via)`, `Topic` theo `name`; `evidence` theo `(path, line)`; `schemas` theo từ điển.
4. Hàm `MergeStores(a, b []Store) []Store`: gộp theo `(kind, env, name)`; hợp `evidence`; `deployed` là OR; `supportedByCode` là OR; `external` là OR; `confidence` lấy mức mạnh hơn theo thứ tự `declared > derived > inferred`; `schemas` hợp có khử trùng; `owner` giữ giá trị khác rỗng đầu tiên theo thứ tự id.
5. Hàm `FilterByEnv(m StorageMap, env string, includeLegacy bool) StorageMap`: `env` rỗng = mọi env trừ `legacy` khi `includeLegacy=false`; Binding tham chiếu Store đã bị lọc cũng bị lọc; Topic không phụ thuộc env.
6. Không có I/O, không `time.Now()`; mọi hàm thuần, kiểm thử bằng bảng.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/storagemap/...` (table-driven): id hợp lệ/không hợp lệ; gộp hai Store cùng khoá (đặc biệt `mysql` `supportedByCode` + `deployed=false` gặp compose không có MySQL); thứ tự không đổi qua 100 hoán vị đầu vào (`rand` với seed cố định, so JSON byte-by-byte); lọc env.

## Tiêu chí hoàn thành

- [x] `go test` xanh, không import ngoài stdlib trong `domain/storagemap`.
- [x] Id ổn định theo bảng ví dụ; ghi quy tắc id vào comment đầu `store_identity.go` (một dòng lý do: `BE-CV-SOL-034` tham chiếu).
- [x] Thông báo cho chủ `BE-CV-SOL-034-data-flow-model` (comment PR) về quy tắc id.

## Rủi ro và lưu ý

- Đổi quy tắc id về sau phá `DataFlow.stores`; coi như hợp đồng đóng băng ngay khi merge.
- Chuẩn hoá `name` không được làm hai Store khác nhau trùng id (ví dụ `a_b` và `a-b`): test va chạm và quyết định (gắn hậu tố `~2` hay lỗi) ở `MergeStores`; chọn lỗi rõ ràng.
