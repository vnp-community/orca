# BE-CV-TASK-040-25: Kênh `codeIntel.quality.gate`, `.profile.get`, `.profile.save`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_gate.go` (mới), `channels_codeintel_quality_gate_test.go` (mới)
**Depends on:** TASK-040-23; stub `GetQualityGate`, `GetQualityProfile`, `SaveQualityProfile` (BE-CV-SOL-085-quality-gate-evaluator-and-profiles)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `gate` (sel, `base?`, `profileName?`, `turnKey?`, `record?`, `includeWaivedDetail?`) => `{gate, waivers[], evaluatedAt, profileDefinitionDigest, comparison[]}`; `profile.get` (sel, `name?`) => `{profile, origin, version, runnableProfiles[]}`; `profile.save` (sel, `scope` tenant|repo, `name`, `definition`, `expectedVersion`) => `{profile, warnings[]}`, `args[0] ≤ 96 KiB`, quyền `quality_profile_write`; 8 s. PQ-34: `reasons[]` có `category`, `tool`; `profile = "<name>@<scope>/v<version>"`. H7: `verdict` `unknown` không bao giờ thành `pass`; `mode:'block'` vẫn chỉ cảnh báo (O9). Lỗi `PROFILE_INVALID | {field}`, `PROFILE_UNKNOWN`, `VERSION_CONFLICT`.

## Việc cần làm

1. `qualityGateArgs{sel, Base, ProfileName, TurnKey, Record *bool, IncludeWaivedDetail *bool}` (chuỗi ≤ 128, ref qua `checkGitRefLike`); `GetQualityGate`; `waivers`, `comparison`, `reasons` luôn mảng.
2. `qualityProfileGetArgs{sel, Name}`; `GetQualityProfile`; `runnableProfiles[].missing[]`, `.scopes[]` mảng.
3. `qualityProfileSaveArgs{sel, Scope, Name, Definition json.RawMessage, ExpectedVersion *int64}`: `definition` phải là **object JSON** (`{` đầu), `json.Valid`; `name` 1..128; `expectedVersion >= 0`; chuyển sang proto theo CR-085 (chuỗi JSON hoặc message; lỗi => `INVALID_PARAMS` nêu `definition`); `MaxArgsBytes 96 KiB`.
4. `record:true` ghi điểm xu hướng (tác dụng phụ ở service): không retry.

## Kiểm thử

- Validate: `definition` mảng/chuỗi/rỗng/quá 96 KiB; `scope` `org`; `expectedVersion` thiếu.
- Fake: `verdict:"unknown"` giữ nguyên (không đổi); `reasons[].result` enum chữ thường; `PROFILE_INVALID | {"field":"checks[0].id"}` đi qua; `VERSION_CONFLICT`.
- Cô lập tenant/`DeviceID`/deadline 8 s.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityGate'`.

## Tiêu chí hoàn thành

- [x] Ba kênh đúng shape UI-API 4.7; `definition` chỉ object hợp lệ.

## Rủi ro và lưu ý

- Gateway không kiểm lược đồ `QualityProfile` (service, `PROFILE_INVALID`).
- Profile bảo mật khi cờ tắt: `PROFILE_UNKNOWN`, không phải `DISABLED` (PQ-01 điểm 4).
