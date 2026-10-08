# FE-CV-TASK-059-01: Nhóm thay đổi hợp đồng, hàng `details` và diff chữ ký (hàm thuần)

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.2
**Priority:** P1
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/contract/contract-grouping.ts`, `contract-detail-rows.ts`, `contract-signature-diff.ts` (mới) + `*.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ContractChange`, `ContractDiff`); FE-CV-TASK-057-01 (`maskSensitiveText`)
**Status:** [x] DONE (verified 2026-10-07: contract-grouping/contract-signature-diff/contract-detail-rows tests 15/15 PASS, tsc/oxlint sạch)

## Context

- UI-API §4.5: `ContractChange {id, kind, name, service?, change, compatibility, ruleId, details, files, consumers, evidence}`, `ContractDiff {scope, summary, changes, migrations, truncated, totalCount}`. UI **không tự phân loại** `breaking`; mã lạ ⇒ nguyên văn.
- `ContractChange` không có `before`/`after` (thiếu hợp đồng, SOL-059 mục 7); quy ước tạm `details.before|after`.

## Việc cần làm

1. `groupContractChanges(changes)` → cây `service → kind → ContractChange[]` (service vắng ⇒ nhóm "(không rõ service)"), đếm theo `compatibility` (`breaking|risky|compatible|unknown`), sắp `breaking` trước; hàm lọc `filterContractChanges({kinds, service, onlyBreaking, onlyChangedByAgent, query})` (`onlyChangedByAgent` = có `files[]` giao `changedFiles`).
2. `contractDetailRows(change)` → `{signature?: {before?: string; after?: string}, rows: {key, value}[]}`; nhận diện `details.before|after`; phần còn lại thành hàng key/value đã `maskSensitiveText`; giới hạn độ dài hiển thị (cắt + dấu "…").
3. `diffSignatureTokens(before, after)` → hai dãy token có nhãn `same|added|removed` (chuẩn hoá khoảng trắng, tách theo ranh giới `\w+|\W`); chuỗi rỗng/`undefined` ⇒ toàn bộ `added`/`removed`.
4. Enum lạ (`kind`, `change`, `compatibility`) → `'unknown'`.

## Kiểm thử

- Nhóm/đếm với fixture trộn; `onlyBreaking`; `kinds`; `details` có/không `before|after`; diff token (thêm/xoá/đổi, rỗng, khoảng trắng); `details` có DSN bị che; enum lạ.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/contract/contract-`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh, không phụ thuộc React/store.
- [ ] Không có nhánh nào tự suy ra `breaking`/`compatible`.

## Rủi ro

- Quy ước `details.before|after` chưa được BE chốt; nếu đổi, chỉ sửa `contract-detail-rows.ts`.

## Ghi chú triển khai (2026-10-07)

- Thêm `listContractServices`, bộ lọc chip `compatibility` và `NO_SERVICE_GROUP_KEY=''` (nhóm "(unknown service)" xếp cuối). `onlyChangedByAgent` nhận `changedFiles` qua tham số (không đọc store).
- Fixture dùng chung: `test-support/contract-findings-fixtures.ts` (mới).
