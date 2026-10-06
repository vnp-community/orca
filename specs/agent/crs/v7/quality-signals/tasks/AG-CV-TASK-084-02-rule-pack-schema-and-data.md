# AG-CV-TASK-084-02: Schema rule pack và dữ liệu pack Orca (mã TS)

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.2
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-pack-schema.ts`, `quality-rule-pack-orca.ts` (mới), `quality-rule-pack-schema.test.ts`
**Depends on:** không
**Status:** [ ] TODO

## Context

PQ-26: `ruleId` `ORCA-NNN`; ID 008/009/016/017 dành riêng. Pack là dữ liệu của agent (R1).

## Việc cần làm

1. Kiểu `Rule`, `RulePack` (5.2) và `validateRulePack(raw)`: `id` `^ORCA-\d{3}$` duy nhất và ∉ {008,009,016,017}; `pattern` biên dịch được bằng `new RegExp` trong try/catch và qua kiểm "tập con an toàn" (từ chối lookbehind, nhóm lặp lồng `(a+)+`, `{n,}` lồng); `maxLineLength` 1..2000; `script.args` luôn `[]`; `kind:"profile-ref"` phải có `ref.profileId` ∈ catalog.
2. `quality-rule-pack-orca.ts`: `ORCA_RULE_PACK` với ORCA-001..007, 010..015 theo bảng 5.3 (thông điệp/`fixHint` tiếng Việt hay Anh theo dữ liệu hiện có, không câu cứng trong UI — chuỗi i18n do backend/frontend).
3. `packVersion` = 1.

## Kiểm thử

Bảng ca: id xấu/trùng/đã dành riêng; pattern ReDoS (`(a+)+$`) bị từ chối; `profile-ref` tới id lạ; pack Orca hợp lệ; test khẳng định vắng `ORCA-008/009/016/017`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-pack-schema.test.ts`.

## Tiêu chí hoàn thành

- [ ] Pack Orca hợp lệ, test cấm ID dành riêng.

## Rủi ro

Kiểm "tập con an toàn" của regex là heuristic; lớp bảo vệ thật là giới hạn thời gian ở task 03.
