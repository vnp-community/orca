# FE-CV-TASK-050-03: Mã lỗi, tách tiền tố `message`, parser phong bì và push

**From Solution:** [FE-CV-SOL-050-types-and-runtime-bridge](../solutions/FE-CV-SOL-050-types-and-runtime-bridge.md) mục 4.3
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/code-intel-errors.ts` (mới), `frontend/src/shared/code-intel-wire-parsers.ts` (mới), tests cùng tên
**Depends on:** FE-CV-TASK-050-01
**Status:** [x] DONE

## Context

- PQ-02/U5: mã ở **tiền tố `message`**; regex `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`; `error.code` luôn `"internal"`, không có `error.data`.
- UI-API §2.3: bảng mã -> `kind` (client) và dạng `data`; ánh xạ khi không có tiền tố (`Unavailable`...). Số mã trong test là bảng này.
- PQ-12: phong bì phẳng; U9: chuỗi tự do không tin cậy.

## Việc cần làm

1. `CODE_INTEL_ERROR_CODES` (đủ mã ở §2.3), `CodeIntelErrorKind` (26 giá trị, xem SOL mục 4.3), `CODE_INTEL_ERROR_KIND_BY_CODE`.
2. `parseCodeIntelErrorMessage(message): {code: string|null; text: string; data: Record<string,unknown>|null}`: JSON hậu tố hỏng hoặc > 2 KiB ⇒ `data:null`, không ném; mã lạ `CODEINTEL_*` giữ nguyên chuỗi, kind `unknown`.
3. `parseCodeIntelEnvelope<T>(raw, parseData)`: kiểm `worktreeId/view/sources/headCommit/stale/truncated/totalCount/etag`, mặc định an toàn (`stale:false`, `truncated:false`, `totalCount:0`, `sources:[]`); `notModified` không có `data`.
4. `parseIndexStatus` (enum lạ ⇒ `'unknown'`/giá trị an toàn; `overall` lạ ⇒ `UNKNOWN`), `parseCodeIntelPushEvent(raw)` trả `null` cho `event` lạ.
5. `data` của `AMBIGUOUS_SYMBOL.candidates` cắt ≤ 10.

## Kiểm thử

- `code-intel-errors.test.ts`: từng dòng bảng §2.3 (bảng mã ⇒ kind); hậu tố JSON hỏng; mã lạ; chuỗi `rpc error: code = ... desc = CODEINTEL_X: m` KHÔNG cần (gateway đã bóc) nhưng không được vỡ.
- `code-intel-wire-parsers.test.ts`: phong bì thiếu trường, enum lạ, `notModified`, push lạ.

## Tiêu chí hoàn thành

- [ ] Mọi mã ở §2.3 có kind đúng; mã lạ -> `unknown`.
- [ ] Parser không bao giờ ném với đầu vào bất kỳ (fuzz nhỏ).

## Rủi ro

- Bảng mã có thể thêm mã mới ở backend: hằng số chỉ là "đã biết", mã lạ vẫn xử lý được.
