# AG-CV-TASK-082-04: Pipeline sau parser: ánh xạ, che, scope, fingerprint, sắp/cắt, suy trạng thái bước

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.2
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-finding-pipeline.ts`, `quality-finding-limits.ts` (mới) + test
**Depends on:** AG-CV-TASK-082-02, 082-03, AG-CV-TASK-081-02
**Status:** [ ] TODO

## Context

CR-082 2.4-2.5; hợp đồng §5.3, §5.5.

## Việc cần làm

1. `quality-finding-limits.ts`: `MAX_FINDINGS_PER_STEP=5000`, `MAX_FINDINGS_PER_RUN=20000`, `MESSAGE_MAX_BYTES=2048`, `FIX_HINT_MAX=500` (có ghi đè `ORCA_QUALITY_*`).
2. `runFindingPipeline(raw[], ctx) → { findings: QualityFinding[]; counts: {error,warning,info,total, outsideScope}; truncated; outsideRepoCount }` theo thứ tự ở 5.2; `message` cắt ở ranh giới UTF-8 + `…`; `QualityFinding` camelCase đúng §5.5.
3. `inferStepStatus({ exec, parsed, profileExit }) → { status; failureKind; envReason? }` theo 5.2, gồm `format_drift` khi mã thoát ∈ `exit.findings` nhưng 0 phát hiện (cờ `skipDriftGuard` cho go vet -json).
4. Giới hạn theo run: pipeline nhận ngân sách còn lại.
5. Không ghi dòng nguồn, `SourceLines`, `snippet`.

## Kiểm thử

Bảng ca: 6 000 phát hiện; thứ tự cắt; `inScope`; message 5 KiB nhiều byte (UTF-8 ngắt giữa ký tự); drift guard; exit lạ; `env`; đếm trước cắt. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-finding-pipeline.test.ts src/relay/quality-finding-limits.test.ts`.

## Tiêu chí hoàn thành

- [ ] Số đếm là trước cắt; thứ tự ổn định; không đường dẫn tuyệt đối trong `file`/`message`.

## Rủi ro

Bộ nhớ khi 20 000 phát hiện: dùng mảng + xử lý theo lô.
