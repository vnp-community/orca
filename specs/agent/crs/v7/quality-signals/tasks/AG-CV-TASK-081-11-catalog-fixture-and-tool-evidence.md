# AG-CV-TASK-081-11: Thu bằng chứng thật cho catalog: tệp cấu hình, `--help`, `--version`

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 4,5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/quality-catalog/` (mới), `agent/scripts/capture-quality-catalog-evidence.mjs` (mới), `agent/src/relay/quality-profile-catalog-evidence.test.ts` (mới)
**Depends on:** không (làm trước task 12)
**Status:** [ ] TODO

## Context

Catalog trong CR-081 2.3 chưa được chạy. Cần biết: `vitest` có subcommand `related` và cờ `--reporter=json --outputFile`; `tsc` 7.0.2 có `--pretty false`; `oxlint --format json`; `golangci-lint --version` in `built with`; `buf` `--error-format json`; `opa test --format json`; `buf breaking --against` với đường dẫn `.git` chung ở worktree liên kết. Chỉ chạy `--help`/`--version` và `ls`/`[ -e ]`; **không** chạy lint/test/build.

## Việc cần làm

1. Script `capture-quality-catalog-evidence.mjs`: ghi `--version` và `--help` (cắt 20 KiB, che đường dẫn) của oxlint, vitest, tsc, go (`go help vet`, `go version`), golangci-lint, buf (`lint`, `breaking`), opa (`test`), vào `__fixtures__/quality-catalog/<tool>/<version>/`.
2. Danh sách tệp tồn tại phải kiểm (tsconfig, vitest config, script `check-*`, `reliability-gates.jsonc`, `backend-go/go.work`, `backend-go/.golangci.yml`, `backend-go/proto/buf.yaml`, `backend-go/policy/orca-authz`) vào `evidence.json`.
3. Thử trên repo git mẫu có worktree liên kết: `buf breaking --against <gitCommonDir>#branch=<base>,subdir=...` chấp nhận hay không (đây là lệnh đọc, chạy trên bản sao nhỏ).
4. Test `quality-profile-catalog-evidence.test.ts`: mỗi cờ catalog dùng (`--reporter=json`, `--outputFile`, `--out-format json`, `--error-format json`, `--format json`, `--pretty`) xuất hiện trong help đã thu của công cụ tương ứng; tệp trong `evidence.json` còn tồn tại.
5. PR ghi bảng điều quan sát được; chỗ nào khác CR thì sửa solution 5.3 trước task 12.

## Kiểm thử

Test đọc fixture (không chạy công cụ). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-profile-catalog-evidence.test.ts`. Chạy script thủ công một lần.

## Tiêu chí hoàn thành

- [ ] Fixture có phiên bản; mọi cờ catalog được chứng minh trong help hoặc chuyển sang `enabled:false` kèm lý do.
- [ ] Không công cụ nào chạy ngoài `--help/--version`.

## Rủi ro

Công cụ vắng trên máy: task `BLOCKED` cho công cụ đó (không dựng fixture tay).
