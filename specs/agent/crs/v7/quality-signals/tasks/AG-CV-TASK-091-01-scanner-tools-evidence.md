# AG-CV-TASK-091-01: Thu bằng chứng công cụ quét (`--help`, JSON mẫu) — chỉ sau khi được duyệt cài

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 4,9
**Priority:** P2
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/quality-security/` (mới), `agent/scripts/capture-security-fixtures.mjs` (mới), `agent/src/relay/quality-security-fixture-contract.test.ts`
**Depends on:** duyệt O12/O-9; AG-CV-TASK-082-01 (khung MANIFEST)
**Status:** [ ] BLOCKED (tools not installed)

## Context

Chưa có công cụ nào trên máy khảo sát. Task thu: cờ `govulncheck -format json`, `osv-scanner --format json --lockfile`, `gitleaks detect --redact …`; hình dạng JSON; hỗ trợ `pnpm-lock.yaml`; quyền cần mạng.

## Việc cần làm

1. Ghi `--version`, `--help` (cắt 20 KiB) mỗi công cụ vào fixture có phiên bản.
2. Chạy trên **repo mẫu nhỏ** có lỗ hổng đã biết (module Go cũ; `pnpm-lock.yaml` nhỏ), ghi JSON thật; ghi ai cần mạng (nơi nào).
3. Với `gitleaks`: xác nhận `--redact` che hoàn toàn trường `Secret`/`Match`; ghi vào PR.
4. Cập nhật mục 5.3 nếu cờ khác.

## Kiểm thử

Test hợp đồng fixture (băm, ngân sách, không đường dẫn tuyệt đối). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-security-fixture-contract.test.ts`.

## Tiêu chí hoàn thành

- [ ] Fixture thật cho từng công cụ đã duyệt; công cụ chưa duyệt ghi `BLOCKED`.

## Rủi ro

Chạy công cụ với mạng thật gửi tên gói ra ngoài: chỉ trên repo mẫu.
