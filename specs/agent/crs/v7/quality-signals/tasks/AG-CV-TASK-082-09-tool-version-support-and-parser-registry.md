# AG-CV-TASK-082-09: Hỗ trợ phiên bản công cụ, phát hiện trôi và registry parser

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-tool-version-support.ts`, `quality-parser-registry.ts` (mới) + test
**Depends on:** AG-CV-TASK-082-04..08, AG-CV-TASK-081-04/06
**Status:** [ ] TODO

## Context

CR-082 2.8 + CR-070 2.6: `verified|untested|incompatible|format_drift`. Registry là điểm cắm vào executor/manager (`StepParser`).

## Việc cần làm

1. `SUPPORTED_QUALITY_TOOL_VERSIONS` (dữ liệu, khởi từ fixture task 01).
2. `classifyToolVersion(tool, versionText) → verified|untested|incompatible`: cùng major khác minor/patch → `untested` (vẫn chạy, cảnh báo ở `QualityStepResult`); major khác → `incompatible` (bước `env_not_ready tool_incompatible`, không chạy; áp dụng golangci-lint 2.x).
3. `readToolVersion(tool, runner)` (`<tool> --version`, cache 10 phút, chạy qua executor với timeout 5 s, không shell).
4. `getParser(key)`; `createStepParser(registry, ctx)` trả `StepParser` cho manager: gọi parser → pipeline → `inferStepStatus` → ghi `QualityStepResult` + findings vào kho kết quả; log một lần mỗi `(tool, version, format)` khi drift; số đếm metric nội bộ (chỉ log, chưa Prometheus).
5. Chặn: parser không thấy trong registry → bước `failed (parser_error)`.

## Kiểm thử

Bảng phiên bản (1.71.0 verified; 1.72.0 untested; 2.0.0 incompatible), registry thiếu khoá, drift mutation: lấy mỗi fixture, sửa tên một khoá bắt buộc → `format_drift`, tích hợp manager giả: bước → `QualityStepResult` đúng. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-tool-version-support.test.ts src/relay/quality-parser-registry.test.ts && pnpm test`.

## Tiêu chí hoàn thành

- [ ] Mutation test phủ mọi parser; `incompatible` không spawn.

## Rủi ro

Phân tích `--version` của từng công cụ khác định dạng: dùng regex theo công cụ, test bằng fixture `--version` của task 11/01.
