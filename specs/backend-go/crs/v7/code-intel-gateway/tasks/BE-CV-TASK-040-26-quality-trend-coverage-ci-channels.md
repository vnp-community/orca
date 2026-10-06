# BE-CV-TASK-040-26: Kênh `codeIntel.quality.trend`, `.coverage`, `.ci`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_signals.go` (mới), `channels_codeintel_quality_signals_test.go` (mới)
**Depends on:** TASK-040-23; stub `GetQualityTrend` (BE-CV-SOL-085-waivers-and-trend), `GetCoverage` (BE-CV-SOL-083-coverage-storage-and-diff), `RefreshCiRun` (BE-CV-SOL-086-ci-run-merge-and-comparison)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `trend` (sel, `from?`, `to?`, `limit ≤ 200`, `groupBy?` commit|turn) => `{points, truncated, totalCount}` (8 s); `coverage` (sel, `runId?` hoặc `headCommit?`) => `{report: CoverageReport|null, reason?}` (20 s); `ci` (sel, `force?`) => `{run|null, comparison[], stale, rateLimited, resetAt?}` (20 s). PQ-33: `QualityTrendPoint.metrics` khoá vắng = không có số (không điền `0`); `CoverageReport.source` measured|estimated; PQ-25: `RefreshCiRun` gọi `ListCommitChecks`, tôn trọng giới hạn `gh`/provider (AGENTS.md); GitLab và provider khác không chỉ GitHub (`capability_unsupported` không lỗi).

## Việc cần làm

1. `qualityTrendArgs{sel, From, To, Limit, GroupBy}`: RFC 3339; `from <= to` (nếu cả hai); `limit` 1..200; `groupBy` commit|turn.
2. `qualityCoverageArgs{sel, RunID, HeadCommit}`: không cho phép cả hai (`invalidParam("runId","exclusive_with_headCommit")`); `headCommit` `checkGitRefLike`.
3. `qualityCIArgs{sel, Force *bool}`.
4. Dịch kết quả: `metrics` là message với `optional` => khoá vắng khi không có (khẳng định bằng test); `report` vắng => `null`; `diff` `null` khi không có; `uncoveredRanges` `[[a,b],...]`; `comparison[].relation` giữ chuỗi.
5. Không retry `ci` (`force` có thể gọi API ngoài); giới hạn tốc độ là của service/scm-integration.

## Kiểm thử

- Validate: `limit` 201; `from` sau `to`; `runId` + `headCommit`; `groupBy` `day`.
- Fake: `points` rỗng `[]`; `metrics` không có `diffCoverage` => khoá vắng; `ci.rateLimited:true` có `resetAt`; `report:null` + `reason`.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualitySignals'`.

## Tiêu chí hoàn thành

- [ ] Ba kênh đúng shape UI-API 4.7; khoá vắng không thành `0`.

## Rủi ro và lưu ý

- Chọn encoder: số vắng phải phân biệt được với `0` (cần `optional` trong proto CR-085/083).
- `ci` tốn quota provider: `force` chỉ khi người dùng bấm (FE).
