# AG-CV-TASK-081-16: Preflight môi trường chỉ đọc và bộ phân giải binary (`qualityToolPath`)

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-tool-resolution.ts`, `quality-environment-preflight.ts` (mới) + test
**Depends on:** AG-CV-TASK-081-10, 081-12
**Status:** [ ] TODO

## Context

Hợp đồng §5.1 (`missing[].reason`: `binary_missing|node_modules_missing|native_runtime_unavailable|go_missing|go_too_old|go_modcache_empty|tool_too_old|tool_incompatible|base_ref_missing|tmp_space_low|home_missing|coverage_provider_missing|network_policy`). Không `pnpm install|rebuild`, không `go mod download`.

## Việc cần làm

1. `buildQualityToolPath(config, deps)`: `config.toolPath` + `~/go/bin` + `$(go env GOPATH)/bin` (spawn `go env GOPATH` một lần, cache 60 s, lỗi → bỏ) + `/usr/local/go/bin` + `~/.local/share/pnpm`, loại trùng, `path.delimiter`.
2. `resolveQualityBinary(name, { cwd, repoRoot, qualityToolPath })`: `<cwd>/node_modules/.bin/<name>` → `<repoRoot>/node_modules/.bin/<name>` (repoRoot = worktree **hiện tại**) → PATH; `fs.access(X_OK)`; không thấy → `binary_missing`.
3. `preflightProfile(profile, ctx) → { ready, missing: QualityEnvMissing[] }` theo bảng: `node_modules/.modules.yaml` (cảnh báo lockfile mới hơn, không chặn); native: `execFile(process.execPath,[ensure-native-runtime.mjs,"--check-only"],{cwd: desktop, timeout:15000})` thoát 1 → `native_runtime_unavailable`; Go: `go version` ≥ `go` của `go.work` (`go_missing/go_too_old`), `go env GOMODCACHE` thư mục tồn tại và có ≥ 1 mục (`go_modcache_empty`); `golangci-lint --version` (`built with go…` < yêu cầu → `tool_too_old` kèm `built`,`required`; major ≠ 1 → `tool_incompatible`); `buf`/`opa --version`; `base_ref_missing`; `fs.statfs(os.tmpdir()) < 256 MiB` → `tmp_space_low`; `HOME` → `home_missing`. Cờ mạng: nếu `profile.network` và env `ORCA_QUALITY_NETWORK=deny` (đề xuất, chưa có trong hợp đồng) → `network_policy`.
4. Cache 60 s theo `(repoRoot, profileId)`; mỗi `missing` có `hint` (không chứa đường dẫn ngoài worktree).
5. Phân loại sau chạy (do parser): để SOL-082.

## Kiểm thử

Bin giả trong PATH tạm; `golangci-lint` giả in `golangci-lint has version 1.62.2 built with go1.22.2`; `go.work` giả `go 1.26.0`; `statfs` giả; `execFile` native giả thoát 1; cache 60 s (đồng hồ giả); khẳng định không có lệnh nào trong `{pnpm install, pnpm rebuild, go mod download, ensure-native-runtime --runtime=…}` được gọi (spy). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-environment-preflight.test.ts`.

## Tiêu chí hoàn thành

- [ ] `missing[].reason` luôn thuộc enum hợp đồng.
- [ ] Không ghi đĩa ngoài tệp tạm của test.

## Rủi ro

`golangci-lint --version` định dạng chưa kiểm chứng (task 11). `ORCA_QUALITY_NETWORK` chưa có trong hợp đồng (câu hỏi mở của solution 091).
