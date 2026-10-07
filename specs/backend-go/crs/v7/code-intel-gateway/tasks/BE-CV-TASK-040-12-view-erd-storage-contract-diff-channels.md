# BE-CV-TASK-040-12: Kênh `codeIntel.erd`, `codeIntel.storage`, `codeIntel.contractDiff`

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_sources.go` (thêm erd, storage), `channels_codeintel_view_change.go` (mới; contractDiff), tương ứng `_test.go`
**Depends on:** TASK-040-11; stub `GetErd` (BE-CV-SOL-031-erd-model-and-access-scan), `GetStorageMap` (BE-CV-SOL-035-storage-map), `GetContractDiff` (BE-CV-SOL-038-contract-diff)
**Status:** [ ] TODO

---

## Context

UI-API 3.1: `erd` => `Env<ErdModel>` hoặc (không `service`) `Env<{services: ErdServiceInfo[]}>`; `storage` => `Env<StorageMap>` (chuỗi tự do đã che ở backend, UI-API 4.4); `contractDiff` => `Env<ContractDiff>`. `contractDiff` và `storage` là P2 của series nhưng kênh vẫn đăng ký (G3); RPC chưa có thì placeholder.

## Việc cần làm

1. `erdArgs{sel, Service, Dialect, Base, Head, IncludeAccess *bool, IncludeInferred *bool, IfNoneMatch}`: `dialect` postgres|mysql; `base`/`head` qua `checkGitRefLike`; mặc định `includeAccess=true`, `includeInferred=false` do service áp (gateway **không** điền mặc định, dùng con trỏ); `Core.GetErd`; `data` hai dạng theo `service` (encoder dùng `oneof`/message do CR-031 chốt, Q3).
2. `storageArgs{sel, Env, IncludeLegacy, IfNoneMatch}`: `env` dev|prod|legacy; `Core.GetStorageMap`.
3. `contractDiffArgs{sel, Base, Kinds[], Detail, IfNoneMatch}`: `kinds ≤ 32` và ⊂ {proto, ws-channel, route, migration}; `detail` summary|full; `Core.GetContractDiff`; `consumers: []` giữ mảng rỗng (UI: "Chưa tìm thấy nơi dùng").
4. Đăng ký ba kênh 20 s.

## Kiểm thử

- Validate: `dialect` lạ, `base` `-x`, `kinds` chứa `sql`, `env` `staging`.
- Fake: `StorageMap.redactedCount`, `topics[]` rỗng `[]`; `contractDiff.summary` đủ bốn đếm; `truncated`; `compatibility` giữ chuỗi chữ thường, `ruleId` lạ giữ nguyên.
- `erd` không `service` trả `services` (không `tables`).
- Quét snake_case; `CODEINTEL_INDEX_MISSING | {"tool":..., "hint":...}` đi qua.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelView(Sources|Change)'`.

## Tiêu chí hoàn thành

- [x] Ba kênh đúng shape UI-API 4.4, 4.5.
- [x] Gateway không tự điền mặc định (con trỏ), không tự phân loại `breaking`.

## Rủi ro và lưu ý

- StorageMap có thể chứa chuỗi cấu hình; gateway không che lại (đã che ở backend, UI che lớp hai); test quét không có khoá `secret|password|token` giá trị thật là trách nhiệm CR-072.
- ERD lớn dễ vượt 2 MiB; xem rủi ro encoder.
