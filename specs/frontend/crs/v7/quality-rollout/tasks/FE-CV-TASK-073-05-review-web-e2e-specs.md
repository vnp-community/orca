# FE-CV-TASK-073-05: Specs web: tóm tắt Review, trạng thái/lỗi, reindex, các lens

**From Solution:** [FE-CV-SOL-073](../solutions/FE-CV-SOL-073-flag-gating-and-web-e2e.md) mục 2.5
**Priority:** P1
**Area:** tests / Playwright web
**File:** `tests/e2e/code-intel-web/{review-summary,states,reindex,lenses}.web.e2e.ts` (mới; `review-summary`, `lenses` và `review-notes` có thể được các task lens tạo trước — hợp nhất, không ghi đè)
**Depends on:** FE-CV-TASK-073-02, 073-03; các lens/khung đã có (050–061)
**Status:** [x] DONE

## Context

- Khẳng định DOM theo `tests/e2e/AGENTS.md`. U9: nhãn chứa HTML/`<script>`/Mermaid hiển thị như văn bản. Mọi chuỗi qua `translate()` ⇒ không lộ khoá i18n thô.

## Việc cần làm

1. `review-summary`: mở tab `review` từ Source Control; thanh tóm tắt, chip index (`indexedAt`, `stale`), thứ tự đọc + tiến độ, điều hướng bàn phím.
2. `states`: `TOOL_UNAVAILABLE`, `INDEX_MISSING` (nút Làm mới), `TIMEOUT` kèm `{inProgress,retryAfterMs}` (tự thử lại), `AMBIGUOUS_SYMBOL` + `candidates`, `truncated`, `NOT_AUTHORIZED` trung tính, `OFFLINE` không treo, `RESPONSE_TOO_LARGE`.
3. `reindex`: `reindex` → `reindexProgress` (cả `percent:null`) → `changed`; lần hai `REINDEX_IN_PROGRESS` gắn vào job; `REINDEX_COOLDOWN` hiển thị `retryAfterSeconds`; `dropStream()` ⇒ resync và tải lại.
4. `lenses`: Ảnh hưởng, ERD, Lưu trữ, Hợp đồng/Phát hiện từ fixture; canary DSN không có trong DOM; nhãn độc hại là văn bản.

## Kiểm thử

- Chạy từng file bằng `pnpm run test:e2e:code-intel-web -- <file>` (**chưa chạy**).

## Tiêu chí hoàn thành

- [ ] Mỗi spec xanh trên fake backend.
- [ ] Không phụ thuộc thứ tự chạy giữa spec.

## Rủi ro

- Phụ thuộc lens đã hoàn thành; ca chưa có component để `test.fixme` có tên task chủ.
