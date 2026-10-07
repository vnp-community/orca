# FE-CV-TASK-050-01: Kiểu mirror `code-intel-types.ts` theo hợp đồng

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.1
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/code-intel-types.ts` (mới), `frontend/src/shared/code-intel-quality-types.ts` (mới, chỉ khung và chú thích: kiểu §4.7 do FE-CV-SOL-085/087 điền), test `code-intel-types.test.ts`
**Depends on:** không
**Status:** [x] DONE

## Context

- UI-API §4.1-§4.6 là bản mirror TS chuẩn; §2.2 là `CodeIntelEnvelope<T>`; §5 là `PushBase/PushChanged/PushReindexProgress/PushQualityProgress/PushQualityFinished/PushGateChanged`.
- Enum lạ phải rơi về `'unknown'` (U4); PQ-32: `risk.level`, `ImpactGraph.risk`, `IndexStatus.overall` chữ HOA, còn lại thường.
- `Finding`, `ContractDiff` (§4.5) khai báo luôn ở đây (CR-059 chỉ thêm hành vi).

## Việc cần làm

1. Sao chép nguyên kiểu §4.1-§4.6 (không đổi tên trường, không thêm trường); `WorktreeSel`, `CodeIntelEnvelope<T>`, 5 kiểu push và union `CodeIntelPushEvent`.
2. Thêm `type WithUnknown<T extends string> = T | 'unknown'` chỉ cho enum mà parser sẽ ép (ghi chú: parser ở 050-03).
3. Tạo `code-intel-quality-types.ts` chỉ có chú thích "§4.7 do FE-CV-SOL-085/087 điền"; export rỗng hợp lệ.
4. Không `any`; không đặt tên `helpers/utils`.

## Kiểm thử

- `code-intel-types.test.ts` (kiểm kiểu bằng `expectTypeOf`/gán fixture): fixture mẫu từ UI-API cho `ChangeOverlay`, `ReviewState`, `IndexStatus` biên dịch được; `SymbolKind` đủ 13 giá trị.
- `pnpm --dir frontend test -- src/shared/code-intel-types`.

## Tiêu chí hoàn thành

- [ ] Mọi kiểu §4.1-§4.6 có mặt, khớp từng trường (rà tay theo bảng).
- [ ] Không phụ thuộc runtime; `tsc` sạch cho file mới.

## Rủi ro

- Hợp đồng còn "Proposed"; đổi hợp đồng thì đổi file này trước (PR riêng).
