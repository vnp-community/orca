# AG-CV-TASK-091-05: Parser `govulncheck` JSON và `osv-scanner` JSON, gom theo module

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.3
**Priority:** P2
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-govulncheck.ts`, `quality-parser-osv-scanner.ts` (mới) + test
**Depends on:** AG-CV-TASK-091-01 (fixture thật), AG-CV-TASK-082-02, 082-04
**Status:** [x] DONE

## Context

Hình dạng chưa xác nhận (task 01). `govulncheck -format json` là luồng thông điệp (`config`, `progress`, `osv`, `finding`) theo tài liệu — chưa chạy.

## Việc cần làm

1. `govulncheck@json`: finding có `trace` có hàm/vị trí → "được gọi" → `SEC-GOVULN/<GO-ID>` error; chỉ module/package (không hàm) → `info`; `message` có module, phiên bản, `fixedVersion`; `anchorOverride:"<modulePath>@<vulnId>"`.
2. `osv-scanner@json`: `results[].packages[].vulnerabilities[]` → `SEC-OSV/<OSV-ID>`; severity theo CVSS (≥ 7 error, 4-6.9 warning, còn lại info); `file` = lockfile; gom theo `(vuln, package)`; dev/prod nếu có, không thì ghi `scopeUnknown` vào message.
3. Gom nhiều module Go cùng `(vuln, module path)` thành một finding liệt kê module (≤ 20).
4. Shape guard → `format_drift`; không đưa đầu ra thô vào `detail`.

## Kiểm thử

Fixture thật (khi có) + ca tổng hợp: called/import-only, 21 module cùng lỗ hổng, CVSS biên, lockfile lạ. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-govulncheck.test.ts src/relay/quality-parser-osv-scanner.test.ts`.

## Tiêu chí hoàn thành

- [x] Import-only luôn `info`; gom module đúng.

## Rủi ro

Phụ thuộc phiên bản công cụ; không có `reachable` ở osv-scanner nên có thể ồn.
