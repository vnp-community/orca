# FE-CV-TASK-087-01: Kiểu, parser, hằng kênh và lỗi của `codeIntel.quality.*`

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.1, 2.2
**Priority:** P0
**Area:** frontend / shared
**File:** `frontend/src/shared/code-intel-quality-types.ts`, `code-intel-quality-wire-parsers.ts`, `code-intel-quality-errors.ts` (mới); mục quality trong `code-intel-rpc-methods.ts`, `code-intel-errors.ts`, `code-intel-wire-parsers.ts` (sửa; CR-050 sở hữu file) và `*.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (bridge, `classifyCodeIntelError`); không cần backend
**Status:** [ ] TODO

## Context

- Hợp đồng `CONTRACT-codeintel-ui-api.md` §4.7 là nguồn kiểu (mirror nguyên văn, không dùng bản CR-087 2.1); §3.2 là tên và tham số 20 kênh; §2.3 bảng lỗi (PQ-02, PQ-03); §5 push (PQ-11).
- Không có `codeIntel` nào trong `frontend/src` (đã grep): mọi thứ mới. Enum lạ → `'unknown'`, không ném (U4).
- Dữ liệu lỗi nằm ở hậu tố ` | {json}` của `message`, không có `error.data` (U5).

## Việc cần làm

1. `code-intel-quality-types.ts`: copy §4.7 (QualityFinding, QualityStep, QualityRun, QualityGate, GateResult, QualityWaiver, RunnableProfile, QualityProfile, QualityTrendPoint, CoverageReport, CiComparison, AgentTurn nếu CR khác cần thì để CR đó) và các kiểu push `PushQualityProgress|Finished|GateChanged`.
2. `code-intel-quality-wire-parsers.ts`: `parseQualityRun`, `parseQualityGate`, `parseQualityFinding`, `parseRunnableProfile`, `parseQualityWaiver`, `parseCiComparison`: enum lạ → `'unknown'`, số âm/NaN → 0, mảng thiếu → `[]`, `percent` giữ `null`.
3. Thêm 20 hằng kênh vào `code-intel-rpc-methods.ts` và kiểu tham số/kết quả cho `CodeIntelRpcContract` (các kênh quality dùng ở solution 1-3: gate, runs, run, start, cancel, findings, waive, profile.get, trend, coverage, ci).
4. `code-intel-quality-errors.ts`: `parseEnvNotReadyData` (`missing[]`, `reason?`), `parseRunInProgressData` (`runId`, `reason`), `parseProfileUnknownData` (`available[]`), `parseWaiverExpiryData` (`maxDays`), `parseRetryAfter`; dữ liệu hỏng → `undefined`, không ném.
5. Đảm bảo `classifyCodeIntelError` (CR-050) trả các `kind`: `quality-disabled`, `profile-unknown`, `env-not-ready`, `run-in-progress`, `run-cancelled`, `rate-limited`, `conflict`, `forbidden`; thêm nếu thiếu và sửa test đếm mã của CR-050 theo bảng §2.3 (không còn "đúng 10 mã").
6. Thêm `parseCodeIntelPushEvent` ba biến thể quality (kể cả `status:'interrupted'`).

## Kiểm thử

Vitest node: mỗi parser với dữ liệu hợp lệ, enum lạ, thiếu trường; lỗi mẫu `CODEINTEL_ENV_NOT_READY: x | {"missing":["node_modules"]}`; push thiếu `worktreeId` bị từ chối. Chạy `pnpm --filter orca-frontend test -- src/shared/code-intel-quality`.

## Tiêu chí hoàn thành

- [ ] Kiểu khớp §4.7 từng trường; test enum lạ xanh.
- [ ] Không ném khi dữ liệu hỏng; `tsc` không thêm lỗi mới.
- [ ] Không dependency mới.

## Rủi ro

- File chung với CR-050: phối hợp merge. `QualityRun.status` không có `interrupted` nhưng push có (câu hỏi mở SOL-087 #1).
