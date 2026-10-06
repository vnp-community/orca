# AG-CV-TASK-082-01: Repo mẫu `quality-mini-repo`, script chụp fixture, `MANIFEST.json`, phiên bản hỗ trợ

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 4,5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/quality-mini-repo/` (mới), `agent/src/relay/__fixtures__/quality/<tool>/<version>/` (mới), `agent/scripts/capture-quality-fixtures.mjs` (mới), `agent/src/relay/quality-fixture-contract.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-11 (argv catalog), AG-CV-SOL-070 (cơ chế MANIFEST)
**Status:** [ ] TODO

## Context

Mọi định dạng đầu ra (trừ 3 script `check-*`) chưa chạy. Task này tạo repo mẫu CÓ CHỦ Ý SAI và chụp đầu ra thật bằng **đúng argv của profile** (cùng hằng số với catalog, một nguồn sự thật). Công cụ chạy trên repo mẫu trong thư mục tạm, không trên repo người dùng.

## Việc cần làm

1. `quality-mini-repo/`: TS có biến không dùng (oxlint) và lỗi kiểu (tsc); test vitest cố ý fail + suite lỗi import; module Go `go 1.22` (để golangci v1.62.2 chạy) có `fmt.Printf` sai định dạng (vet), test Go fail + biến thể lỗi biên dịch, `errcheck` (golangci); `.proto` vi phạm tên (buf lint) và commit thứ hai phá tương thích (breaking); test Rego fail; tên tệp có khoảng trắng/Unicode; thông điệp có CRLF/rất dài; hai phát hiện giống hệt. Không có `node_modules`, `.gitnexus`.
2. `capture-quality-fixtures.mjs`: copy repo mẫu vào `mkdtemp`, `git init` + commit, chạy đúng argv catalog, dừng nếu `--version` ≠ phiên bản đích, che đường dẫn, ghi `MANIFEST.json` (`tool`, `toolVersion`, `argv`, `markers`, `capturedAt`, `sha256`), in `git diff --stat`.
3. `quality-fixture-contract.test.ts`: mỗi thư mục phiên bản có `MANIFEST.json`; băm khớp; ≤ 20 KiB/tệp, ≤ 300 KiB/thư mục; không có `/home/`, `/opt/repos`, `/tmp/`; `argv` trong MANIFEST bằng `argv` catalog; test đọc `package.json` (`oxlint`, `typescript`, `vitest`) khẳng định khoảng caret chứa một phiên bản có fixture.
4. PR ghi: (a) `go vet -json` ra stdout hay stderr, thoát mấy; (b) `FailedBuild` của `go test -json`; (c) hình dạng oxlint JSON và quyết định `@json|@github`; (d) dòng lỗi tsc 7.0.2; (e) buf/opa/golangci JSON thật. Cập nhật solution mục 5.4 nếu khác CR.
5. `SUPPORTED_QUALITY_TOOL_VERSIONS` khởi đầu = các phiên bản đã chụp.

## Kiểm thử

Test hợp đồng ở mục 3. Chạy script thủ công. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-fixture-contract.test.ts`.

## Tiêu chí hoàn thành

- [ ] Có fixture cho cả 9 công cụ/định dạng ở 5.4 hoặc ghi `BLOCKED` kèm lý do cho công cụ vắng.
- [ ] Hình dạng thật được ghi vào PR trước khi bất kỳ task parser nào bắt đầu.

## Rủi ro

Công cụ không cài (golangci-lint chỉ v1.62.2) → task chuyển một phần BLOCKED. Chụp lại là PR riêng nêu phiên bản.
