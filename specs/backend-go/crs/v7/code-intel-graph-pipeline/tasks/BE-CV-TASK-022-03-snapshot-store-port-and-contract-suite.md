# BE-CV-TASK-022-03: Cổng `SnapshotStore`, store bộ nhớ và suite hợp đồng hai dialect

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/snapshot_store.go` (mới), `.../internal/adapter/snapshotstorecontract/snapshot_store_contract.go` (mới), `.../internal/adapter/memorystore/snapshot_store_memory.go` (mới, chỉ cho test)
**Depends on:** TASK-022-02, SOL-011 (cài SQL)
**Status:** [x] DONE

---

## Context

SOL-011-repositories cài SQL; solution này đặc tả cổng và bộ test để cả hai dialect chạy cùng một suite (§8.3).

## Việc cần làm

1. Cổng 8 phương thức (SOL-022 2.D).
2. Suite `Run(t, newStore func() SnapshotStore, seedTenant func() string)`: upsert đồng thời, `GetLatest`, `EvictOldest` giữ commit mới nhất, `DeleteExpired` (đồng hồ DB), payload 3 MiB, **cô lập tenant** (hai tenant cùng khoá).
3. Bản bộ nhớ chạy suite này trong unit test.
4. Phối hợp SOL-011: PR của SOL-011 gọi suite với PG và MySQL.

## Kiểm thử

- `go test ./internal/adapter/memorystore/ -race`.
- `go test -tags=integration ./internal/adapter/{postgres,mysql}/ -run SnapshotStoreContract -race` (khi SOL-011 có).

## Tiêu chí hoàn thành

- [x] Suite bao phủ cô lập tenant và upsert. - [x] Bản bộ nhớ xanh.

## Rủi ro và lưu ý

- Test RLS PG cần role `NOSUPERUSER NOBYPASSRLS`.
