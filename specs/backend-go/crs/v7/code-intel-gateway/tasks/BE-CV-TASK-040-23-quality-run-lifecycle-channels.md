# BE-CV-TASK-040-23: Kênh `codeIntel.quality.start`, `.cancel`, `.run`, `.runs`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_run.go` (mới), `channels_codeintel_quality_run_test.go` (mới), `channels_codeintel_register.go`
**Depends on:** TASK-040-07 (và 040-01 cho `QualityGate` client); stub `StartQualityRun`, `CancelQualityRun`, `GetQualityRun`, `ListQualityRuns` (BE-CV-SOL-082-quality-run-storage-and-ingest, 085)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `start` (sel, `profile`, `scope`, `base?`) => `{run}` quyền `review_write`; `cancel` (sel, `runId`) => `{run}`; `run` => `QualityRun` (4.7); `runs` (sel, `limit ≤ 50`, `source`, `pageToken`) => `{runs, nextPageToken}`; tất cả 8 s. Mã: `ENV_NOT_READY | {missing[],reason?}`, `RUN_IN_PROGRESS | {runId,reason}`, `RUN_NOT_FOUND`, `RUN_CANCELLED`, `PROFILE_UNKNOWN | {available[]}` (đi qua nguyên). H3/O11: chỉ **tên profile**; không lệnh/`env`/`cwd`/`timeout`.

## Việc cần làm

1. `qualityStartArgs{sel, Profile, Scope, Base}`: `profile` 1..128 (ký tự `[A-Za-z0-9._:/@-]`); `scope` worktree|changed|commitRange; `base` `checkGitRefLike`; `Core`→`Quality.StartQualityRun`.
2. `qualityRunIDArgs{sel, RunID}` dùng cho `cancel` và `run` (`checkOpaqueID`).
3. `qualityRunsArgs{sel, Limit, Source, PageToken}`: `limit` 1..50; `source` local|ci.
4. Kết quả `{run}` / `QualityRun` / `{runs, nextPageToken}`: `steps[]`, `indexBasis[]` luôn mảng; `startedAt/finishedAt` `null` khi chưa; `summary` đủ đếm.
5. Đăng ký 4 kênh 8 s trong `registerCodeIntelQualityChannels` (hàm mới).

## Kiểm thử

- Validate: `profile` rỗng/129/`a b`; `scope` `all`; `limit` 0/51; khoá `command`, `env`, `cwd`, `timeout`, `args` => `INVALID_PARAMS` nêu tên.
- Fake: `RUN_IN_PROGRESS`/`ENV_NOT_READY` đi qua; `start` nhận selector; `QualityGate_DISABLED` đi qua; `runs` mảng rỗng `[]`.
- `DeviceID` => `NOT_AUTHORIZED`; deadline 8 s.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityRun'`.

## Tiêu chí hoàn thành

- [ ] Bốn kênh đúng shape UI-API 4.7; không trường thực thi tuỳ ý.

## Rủi ro và lưu ý

- `start` có tác dụng phụ nặng: không retry, FE dùng `invoke`.
- Chặn cờ tắt ở service, không ở gateway.
