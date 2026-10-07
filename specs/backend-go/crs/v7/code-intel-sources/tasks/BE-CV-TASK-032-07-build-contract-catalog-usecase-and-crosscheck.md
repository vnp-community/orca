# BE-CV-TASK-032-07: Use case `BuildContractCatalog`, cache `contractCatalog`, kiểm chéo descriptor sinh sẵn

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/build_contract_catalog.go` (mới), `.../usecase/contract_catalog_descriptor_crosscheck_test.go` (mới), `.../usecase/build_contract_catalog_test.go`
**Depends on:** BE-CV-TASK-032-04, -05, -06; BE-CV-SOL-022 (snapshot)
**Status:** [x] DONE

---

## Context

SOL-032 mục 2.D. Dùng nội bộ bởi 033/034/038.

## Việc cần làm

1. Ghép proto (task 03) + server (04) + client (05) + kênh (06); `Warnings`; `truncated` theo ngân sách (700 RPC / 3 000 cạnh / 800 kênh).
2. `params_hash` = băm `(include_shapes, tập (path,ContentHash) bẩn, schema_version)`; lưu `graph_snapshots(view="contractCatalog")`; mọi truy vấn có `tenant_id`; singleflight.
3. Test kiểm chéo: tập `(service, rpc, input, output, streaming)` so với descriptor trong `proto/gen/go` (`protoregistry`), lệch → đỏ.
4. Không mã nguồn thân hàm trong kết quả (chỉ `SymbolRef`, vị trí).

## Kiểm thử

`go test ./services/code-intel-service/internal/usecase/... -run Contract` (chưa chạy); hai tenant cùng commit: khoá cache khác; ma trận hai dialect cho phần ghi snapshot (repository của SOL-011).

## Tiêu chí hoàn thành

- [x] Descriptor không lệch.
- [x] Cache cô lập tenant.
- [x] `headCommit`, `stale`, `truncated` có mặt.

## Rủi ro và lưu ý

- Chi phí đọc toàn `internal/**` lần đầu lớn; chỉ lần đầu (cache oid).
