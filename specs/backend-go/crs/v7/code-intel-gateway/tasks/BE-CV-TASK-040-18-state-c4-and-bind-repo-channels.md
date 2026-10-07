# BE-CV-TASK-040-18: Kênh `codeIntel.c4.get`, `codeIntel.c4.save`, `codeIntel.bindRepo`

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_state_review.go` (thêm c4), `channels_codeintel_state_settings.go` (mới; bindRepo), tests tương ứng
**Depends on:** TASK-040-17; stub `GetC4Overrides`/`SaveC4Overrides` (BE-CV-SOL-033-c4-overrides-yaml), `BindRepo` (BE-CV-SOL-012-target-resolution-and-bindings)
**Status:** [ ] TODO

---

## Context

UI-API 3.1: `c4.get` (sel, `container?`) => `{container, document, version, updatedBy, updatedAt, seedSource?}` (8 s, `read`); `c4.save` (sel, `container`, `document` YAML ≤ 64 KiB, `expectedVersion`) => `{version, warnings[{code,message}]}` (quyền `c4_write`, `args[0] ≤ 96 KiB`); `bindRepo` (sel) => `{binding, status}` idempotent, **không** nhận đường dẫn/tên repo (O4, U3). `ListRepoBindings` không có kênh.

## Việc cần làm

1. `c4GetArgs{sel, Container}` (≤ 512); `Core.GetC4Overrides`; trả phẳng (không envelope).
2. `c4SaveArgs{sel, Container, Document *string, ExpectedVersion *int64}`: `container` bắt buộc; `document` có mặt, `len ≤ 64 KiB`, UTF-8 hợp lệ, không NUL; `expectedVersion >= 0`; `Core.SaveC4Overrides`; `warnings` luôn mảng; lỗi YAML do service (`CODEINTEL_INVALID_PARAMS | {"field":"document"}`) đi qua.
3. `bindRepoArgs{sel}`: `Core.BindRepo`; kết quả `{binding, status}` (binding.indexScope exact|repo_root|unresolved).
4. Tuyệt đối không log `document`.

## Kiểm thử

- Validate: `document` 64 KiB+1, chứa NUL, thiếu; `container` rỗng; `expectedVersion` thiếu; `bindRepo` với `repo`/`path` => `INVALID_PARAMS`.
- Fake: `VERSION_CONFLICT` qua `Aborted`; `warnings` nil => `[]`; `bindRepo` gọi hai lần => cùng kết quả (idempotent do service; test chỉ khẳng định không có trạng thái ở gateway).
- Cỡ: `c4.save` args 96 KiB+1 => `too_large`.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelState(C4|Bind)'`.

## Tiêu chí hoàn thành

- [x] Ba kênh đúng shape; trần 64/96 KiB; bindRepo không nhận đường dẫn.

## Rủi ro và lưu ý

- Hợp đồng đặt `c4.save` theo `document` 64 KiB (CR-040 ghi 24 KiB): lệch W3, theo hợp đồng.
- Quyền `c4_write` chỉ owner/admin (service).
