# BE-CV-TASK-040-27: Kênh `codeIntel.quality.trace`, `.trace.confirm`, `.trace.link`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_trace.go` (mới), `channels_codeintel_quality_trace_test.go` (mới)
**Depends on:** TASK-040-23; stub `GetRequirementTrace`, `ConfirmRequirementEvidence`, `LinkWorktreeTask` (BE-CV-SOL-092-requirement-trace)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `trace` (sel, `base?`, `includeInferred?` mặc định true, `taskId?`, `turnKey?`) => `{trace: RequirementTrace, evaluatedAt}` (20 s, `quality_read`); `trace.confirm` (sel, `requirementKey`, `evidenceKind`, `evidenceRef`, `linkKind` confirm|reject, `scope?`) => `{trace}`; `trace.link` (sel, `taskId`; rỗng = gỡ liên kết) => `{trace}`; hai kênh ghi `args[0] ≤ 8 KiB`, 8 s, `review_write`. Task thuộc `task-service` (`GetTask`, `ResolvePermission`) do service gọi, gateway không.

## Việc cần làm

1. `qualityTraceArgs{sel, Base, IncludeInferred *bool, TaskID, TurnKey}` (≤ 128; `base` ref).
2. `qualityTraceConfirmArgs{sel, RequirementKey, EvidenceKind, EvidenceRef, LinkKind, Scope}`: `requirementKey` 1..256; `evidenceKind` change|test|check_run|manual_confirmation; `evidenceRef` 1..1024; `linkKind` confirm|reject; `scope` ≤ 64.
3. `qualityTraceLinkArgs{sel, TaskID *string}`: khoá `taskId` **bắt buộc có mặt** (giá trị rỗng hợp lệ = gỡ); ≤ 128.
4. `MaxArgsBytes 8 KiB` cho hai kênh ghi (catalog đã có).
5. Dịch `RequirementTrace`: `requirements[].evidence[]`, `unlinkedChanges[]`, `warnings[]` mảng; `state` giữ chữ thường; `linkConfidence` `none` hợp lệ.

## Kiểm thử

- Validate: `evidenceKind` `guess`; `taskId` thiếu khoá (link) => từ chối; `taskId:""` => hợp lệ; args 8 KiB+1.
- Fake: tenant khác => `NOT_AUTHORIZED` từ service đi qua; `trace.summary` đủ đếm.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityTrace'`.

## Tiêu chí hoàn thành

- [ ] Ba kênh đúng shape; phân biệt `taskId` vắng với rỗng.

## Rủi ro và lưu ý

- Phân biệt "vắng" và "rỗng" cần `*string` (không dùng `omitempty` thuần).
- `FindTaskBySource` chưa có (proto-and-data-map §3.3): không ảnh hưởng gateway.
