# BE-CV-TASK-022-02: Domain `Snapshot`, `CanonicalParamsHash`, ETag

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/domain/{snapshot.go,snapshot_params_hash.go,snapshot_etag.go}` (mới) và test
**Depends on:** BE-CV-TASK-020-07 (`ViewKind.CacheName`)
**Status:** [ ] TODO

---

## Context

Khoá, hash, ETag xác định và không rò dữ liệu (SOL-022 2.B).

## Việc cần làm

1. `Snapshot`, `SnapshotKey{Tenant,Binding,View,HeadCommit,ParamsHash}`, `GraphModelVersion`.
2. `CanonicalParamsHash(params any)`: JSON khoá sắp xếp, điền mặc định, loại trường không thuộc hợp đồng, tiền tố `GraphModelVersion`; SHA-256 hex.
3. `ContentETag` (32 ký tự) và `ClientETag(content, head, stale)` có nháy; `ValidateIfNoneMatch` ≤ 80 ký tự.

## Kiểm thử

- `go test ./internal/domain/ -run 'Snapshot|ParamsHash|ETag' -race -count=2`.
- Ca: đổi thứ tự khoá → cùng hash; đổi `GraphModelVersion` → hash khác; HEAD đổi hoặc `stale` lật → ETag đổi, payload giữ; `if_none_match` 81 ký tự → INVALID_PARAMS.

## Tiêu chí hoàn thành

- [ ] Xác định và nhạy với đúng các đầu vào nêu. - [ ] Domain không import proto/DB.

## Rủi ro và lưu ý

- Tên file cụ thể; không `utils`.
