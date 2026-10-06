# FE-CV-TASK-050-02: Hằng 46 kênh, 5 sự kiện push và test đối chiếu hợp đồng

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.1
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/code-intel-rpc-methods.ts` (mới), `frontend/src/shared/code-intel-contract-conformance.test.ts` (mới)
**Depends on:** FE-CV-TASK-050-01
**Status:** [ ] TODO

## Context

- UI-API §3.1 (26 kênh, tiền tố `codeIntel.`) và §3.2 (20 kênh `codeIntel.quality.*`): tổng 46; `subscribe` là stream duy nhất.
- Mẫu: `shared/mcp-contract-conformance.test.ts` (đọc CONTRACT bằng `readFileSync`, `describe.skipIf(!hasContract)`, đường dẫn tính từ `frontend/src` lên gốc repo).

## Việc cần làm

1. `CODE_INTEL_RPC_METHODS` (as const, đủ 46), `CODE_INTEL_STREAM_METHODS = ['codeIntel.subscribe']`, `CODE_INTEL_PUSH_EVENTS` (5 giá trị `event`), `CodeIntelMethod`.
2. `CodeIntelRpcContract: {[M]: {params; result}}` cho các kênh review (status…subscribe, settings); `quality.*` để `params/result: unknown` có chú thích "kiểu do 085/087 điền".
3. Bảng quyền và timeout tham khảo (`CODE_INTEL_METHOD_LIMITS`: `maxArgsBytes` theo UI-API §2.4: `reviewState.save` 256 KiB, `c4.save` 96 KiB, `quality.profile.save` 96 KiB, `quality.trace.confirm|link` 8 KiB, còn lại 16 KiB).
4. Test đối chiếu: parse bảng kênh ở UI-API §3, so với hằng; so tập sự kiện push ở §5; cấm literal `codeIntel.send`; bỏ qua khi thiếu tệp hợp đồng.

## Kiểm thử

- `pnpm --dir frontend test -- src/shared/code-intel-contract-conformance`. Thêm ca: thêm một kênh giả vào hằng thì test đỏ.

## Tiêu chí hoàn thành

- [ ] Đúng 46 kênh (26+20) khớp hợp đồng; 5 sự kiện push.
- [ ] `CODE_INTEL_METHOD_LIMITS` khớp §2.4.
- [ ] Test xanh, và bị bỏ qua (không đỏ) khi không có tệp hợp đồng.

## Rủi ro

- Phân tích markdown dễ vỡ khi bảng đổi định dạng: dùng regex theo cột đầu tiên có dạng `` `tên` ``.
