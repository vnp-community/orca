# BE-CV-TASK-080-03: Hàm thuần `PlanRefresh`, `RefreshStateFromJob`, `dedupe_key`

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/index_refresh_plan.go`, `index_refresh_state.go`, `index_dedupe_key.go` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-080-01
**Status:** [x] DONE

## Context
Bảng quyết định ở SOL-080 mục 2.C[6]. `domain` chỉ import stdlib (arch/03). Phân loại `indexScope/freshness` do agent tính (C-AG §4.1); backend chỉ đọc.

## Việc cần làm
1. `RefreshInputs{Basis []IndexBasisView, Policy, Host{Cores,Load1}, QualityRunActive, LastAnalyzeAt, Now, Thresholds}`; `RefreshAction{Kind: NoOp|Tier1|Tier2|Defer, Tools []string, Outcome, Reason}`.
2. Cài đúng 7 dòng của bảng; `Tier1+Tier2` thành một `Tools` theo thứ tự codegraph rồi gitnexus.
3. `RefreshStateFromJob(status, outcome, message) → idle|queued|running|deferred|failed|skipped`.
4. `DedupeKey(bindingID, head, dirtyFingerprint) = hex(sha256(...))`.

## Kiểm thử
- Bảng test phủ 7 dòng + `indexedCommit` đã gc, `fresh_base`, hai công cụ lệch, tải đúng ngưỡng 0,7 (biên), `MIN_INTERVAL`.
- Hàm xác định (cùng đầu vào, cùng kết quả).

## Tiêu chí hoàn thành
- [x] Không import ngoài stdlib/domain; không `max-lines` disable.

## Rủi ro
Ngưỡng mặc định là giả định CR-080 (chưa đo); để tham số hoá.
