# FE-CV-TASK-050-08: Dùng và mở rộng fixture của fake backend 073-02 (cổng G4)

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.4
**Priority:** P0
**Area:** frontend / test-support
**File:** `frontend/src/renderer/src/test-support/code-intel-fixtures.ts` (mở rộng; tệp do [FE-CV-TASK-073-02](../../quality-rollout/tasks/FE-CV-TASK-073-02-code-intel-fake-backend.md) sở hữu), test `code-intel-fixtures-review.test.ts` (mới)
**Depends on:** FE-CV-TASK-073-02, FE-CV-TASK-050-01, FE-CV-TASK-050-03
**Status:** [ ] TODO

## Context

- Hợp đồng G4: fake backend làm trước kênh thật. 073-02 sở hữu `createFakeCodeIntelBackend` (46 kênh, ép hợp đồng U1/U3, push có `event`, `failNext`, `dropStream`); 073-02 lại phụ thuộc kiểu/bộ phân loại của SOL-050. **Không tạo bản fake thứ hai.** Mẫu gốc: `test-support/mcp-fake-backend.ts`, `mcp-fixtures.ts`.

## Việc cần làm

1. Phối hợp thứ tự: 050-01/03/07 xong trước ⇒ 073-02 dựng; trước đó test của 050-04/05/07 dùng stub cục bộ trong file test.
2. Bổ sung fixture mà review-frontend cần nếu 073-02 chưa có (một nguồn, kiểu §4): `IndexStatus` đủ 9 `overall`; `ChangeOverlay` nhỏ/vừa/`emptyReason`/`truncated`; `ReadingStep`+`ComponentGroup`; `ImpactGraph` không cạnh; `SymbolDetail` (kèm `sourceOmitted`); `ModuleGraph` phân trang; `C4ComponentView`+`ContainerRef[]`; `DataFlow`+`SequenceModel` (`partial`, `gaps`); `ReviewState` `version:0`.
3. Kịch bản lỗi dùng API sẵn của 073-02 (`failNext`): `CODEINTEL_TIMEOUT | {"retryAfterMs":..,"inProgress":true}`, `VERSION_CONFLICT`, `AMBIGUOUS_SYMBOL`, `REINDEX_COOLDOWN`.
4. Fixture không chứa đường dẫn tuyệt đối hay secret.

## Kiểm thử

- Mỗi fixture bổ sung qua `parse*` của 050-03; không trùng tên với fixture 073-02.

## Tiêu chí hoàn thành

- [ ] Không có file fake backend thứ hai; mọi test lens dùng fixture ở một nguồn.

## Rủi ro

- Hai nhóm cùng sửa `code-intel-fixtures.ts`: chỉ thêm, không đổi fixture đang có.
