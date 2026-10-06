# AG-CV-TASK-082-07: Parser golangci-lint, buf (lint/breaking), opa test

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.4
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-golangci.ts` (+ `-severity.ts`), `quality-parser-buf.ts`, `quality-parser-opa.ts` + test
**Depends on:** AG-CV-TASK-082-01, 082-02, 082-04
**Status:** [ ] TODO

## Context

CR-082 2.3, 2.7. `Issues[].SourceLines` bị bỏ (có thể chứa secret).

## Việc cần làm

1. `golangci@json`: `Issues[].{FromLinter,Text,Severity,Pos,Replacement}`; bảng severity trong `-severity.ts` (error: typecheck, errcheck, staticcheck, govet, bodyclose, noctx, errorlint; warning: ineffassign, unused, gosimple, gocritic; info: gofmt, goimports; lạ: warning); `fixHint` từ `Replacement` (cắt, không áp dụng).
2. `buf@json`: mỗi dòng một đối tượng `{path,start_line,start_column,end_line,end_column,type,message}`; lint → `buf/<type>` warning, breaking → `buf-breaking/<type>` error, category `architecture`; `path` tương đối thư mục `buf.yaml`.
3. `opa@json`: mảng `{location,package,name,fail?,error?}`; `fail` → `opa/test-failed` (`testAnchor=<package>.<name>`); `error` biên dịch → `opa/compile-error` (typecheck).
4. golangci-lint major ≠ 1 → `failure.kind="env"`, `envReason:"tool_incompatible"` (xác định ở task 09).

## Kiểm thử

Test đọc `backend-go/.golangci.yml` thật và khẳng định mọi linter đã bật có dòng trong bảng severity; fixture từng công cụ; `SourceLines` không xuất hiện trong đầu ra. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-golangci.test.ts src/relay/quality-parser-buf.test.ts src/relay/quality-parser-opa.test.ts`.

## Tiêu chí hoàn thành

- [ ] Bảng severity phủ cấu hình; không có `SourceLines`.

## Rủi ro

Linter bật thêm sau này làm test đỏ (có chủ ý).
