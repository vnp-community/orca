# BE-CV-TASK-040-14: Kênh `codeIntel.changeOverlay`, `codeIntel.readingOrder`, `codeIntel.findings`

**From Solution:** BE-CV-SOL-040-codeintel-view-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_view_change.go` (thêm), `channels_codeintel_view_change_test.go` (mới)
**Depends on:** TASK-040-12; stub `GetChangeOverlay`, `GetReadingOrder` (BE-CV-SOL-036-change-overlay-pipeline, -reading-order-and-risk), `ListFindings` (BE-CV-SOL-037-structure-findings-and-dismissals)
**Status:** [ ] TODO

---

## Context

UI-API 3.1, 4.3, 4.5: `changeOverlay` `Env<ChangeOverlay>` (`detail:'summary'` chỉ `scope`, đếm, `risk`, `components`, `indexFreshness`; mảng chi tiết rỗng); `readingOrder` `Env<{steps, components}>`; `findings` `Env<{findings, dismissedCount, indexFreshness}>` với `scope` mặc định `changed`, `includeDismissed` mặc định false, `limit ≤ 200`. `risk.level` HOA (PQ-32), `ReadingStep.reason` chữ thường, `emptyReason:'unborn-head'`.

## Việc cần làm

1. `changeOverlayArgs{sel, Base, Head, Mode, Detail, IfNoneMatch}`: refs `checkGitRefLike`; `mode` worktree|committed; `detail` summary|full; `Core.GetChangeOverlay`.
2. `readingOrderArgs{sel, Base, Head, IfNoneMatch}`; `Core.GetReadingOrder`.
3. `findingsArgs{sel, Rules[], Severities[], PathPrefix, IncludeDismissed *bool, Scope, Base, Limit *int, PageToken, IfNoneMatch}`: `rules ≤ 32`, `severities` ⊂ error|warning|info, `pathPrefix` an toàn khi khác rỗng, `scope` all|changed, `limit` 1..200; `Core.ListFindings`; `nextPageToken`, `totalCount`.
4. Đăng ký ba kênh 20 s.

## Kiểm thử

- Validate: `mode` `auto`, `detail` `tiny`, `severities` `high`, `limit` 201, `pathPrefix` `/abs`, `rules` 33 phần tử.
- Fake: overlay `detail:summary` => mảng chi tiết là `[]`, `limits.totalCounts` object; `touchedContracts[].compatibility`; `finding.origin` ∈ introduced|touched|preexisting|unknown giữ nguyên; `dismissed` vắng khi chưa; `risk.level` giữ HOA, `confidence` chữ thường.
- `CODEINTEL_INDEX_MISSING`, `CODEINTEL_TIMEOUT | {"inProgress":true}` đi qua nguyên.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelView(Change)'`.

## Tiêu chí hoàn thành

- [x] Ba kênh đúng shape UI-API 4.3/4.5.
- [x] Mặc định (`scope`, `includeDismissed`) do service áp, gateway không điền.

## Rủi ro và lưu ý

- Gateway không biết `base` mặc định (merge-base, O7); service quyết.
- `findings` và `quality.findings` là hai nguồn tách (PQ-06); đừng gộp ở đây.
