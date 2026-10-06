# quality-signals (v7) solutions: index (agent)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. "Đã đọc" = đọc code/CR/hợp đồng, chưa chạy hệ thống.

Solution phía `agent/` (TypeScript, `agent/src/relay/`) cho feature `quality-signals` của series v7 "Xem code và kiểm soát chất lượng". Hợp đồng chuẩn tắc: [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (§2, §3, §5, §6, §9), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, 16, 17, 21, 26, 33; §7, §8.2, §8.3), [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md). Khi CR và hợp đồng khác nhau, theo hợp đồng (mỗi solution có mục "Lệch giữa CR và hợp đồng"). CR nguồn: [`docs/crs/v7/quality-signals/`](../../../../../../docs/crs/v7/quality-signals/README.md). CR-CV-094 là spike tài liệu, **không có solution**. CR-CV-086 không có việc ở agent.

## Bảng CR → Solution → Task

| CR | Solution | Task |
|---|---|---|
| CR-CV-080 | [AG-CV-SOL-080-index-basis-and-reindex-triggers](./AG-CV-SOL-080-index-basis-and-reindex-triggers.md) | 080-01 đến 07 |
| CR-CV-081 | [AG-CV-SOL-081-quality-runner-core](./AG-CV-SOL-081-quality-runner-core.md) (A) | 081-01 đến 09 |
| CR-CV-081 | [AG-CV-SOL-081-quality-profile-catalog-and-preflight](./AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) (B) | 081-10 đến 18 |
| CR-CV-082 | [AG-CV-SOL-082-quality-parsers-and-fingerprint](./AG-CV-SOL-082-quality-parsers-and-fingerprint.md) | 082-01 đến 09 |
| CR-CV-083 | [AG-CV-SOL-083-coverage-collection](./AG-CV-SOL-083-coverage-collection.md) | 083-01 đến 07 |
| CR-CV-084 | [AG-CV-SOL-084-convention-rule-pack](./AG-CV-SOL-084-convention-rule-pack.md) | 084-01 đến 07 |
| CR-CV-091 | [AG-CV-SOL-091-security-and-dependency-profiles](./AG-CV-SOL-091-security-and-dependency-profiles.md) | 091-01 đến 08 |

Tổng: 7 solution, 56 task. Quy ước ID: `AG-CV-SOL-<CR>-<slug>`, `AG-CV-TASK-<CR>-<NN>-<slug>` (NN liên tục trong cùng CR, kể cả hai solution của CR-081).

## Sơ đồ phụ thuộc

```
AG-CV-SOL-001 (nền codeintel) ─┬─▶ 004 ─▶ 080 (index basis, reindex triggers)
                               └─▶ 081-A (lõi chạy) ─▶ 081-B (catalog/preflight/listProfiles/run)
                                          └────────────▶ 082 (parser, fingerprint) ─┬─▶ 083 (coverage; task 083-03 diff dùng chung)
                                                                                     ├─▶ 084 (rule pack; dùng 083-03)
                                                                                     └─▶ 091 (bảo mật, P2; dùng 083-03)
080-04 (host snapshot) ─▶ 081-17 (listProfiles)
081-05 (cổng nặng) ─▶ codeintel.reindex (SOL-004)
```
Đợt (hợp đồng §7.3): đợt 7 = 080, 081, 082; đợt 8 = 083, 084; đợt 9 = 091.

## Quyết định chung

| # | Quyết định | Nguồn |
|---|---|---|
| 1 | Chỉ **profile có tên** (catalog trong mã agent + L2 `~/.orca/quality/profiles.json` thêm/vô hiệu); không nhận `command|argv|args|env|cwd|timeout` | O11, §2.1, §9.5 |
| 2 | Env tiến trình con xây từ **rỗng** + allowlist + deny pattern (không dùng `config.toolEnv`, vốn chứa toàn bộ `process.env`) | §2.4, README v7 điểm 22 |
| 3 | Mọi bước: `shell:false`, `detached`, tệp đầu ra, timeout, `maxOutputBytes`, `nice 10`, diệt cây `SIGTERM → 5 s → SIGKILL` (Windows `taskkill` không shell) | §5.2, §5.4 |
| 4 | Preflight chỉ đọc; không `pnpm install|rebuild`, `go mod download`, `ensure-native-runtime --runtime=…` (chỉ `--check-only`); không `--init`/`--prune` của `check-max-lines-ratchet` (ghi baseline) | CR-081, đã đọc script |
| 5 | Công cụ/phụ thuộc mới (`@vitest/coverage-v8`, `govulncheck`, `osv-scanner`, `gitleaks`) **cần duyệt**, không thêm mặc định | O12 |
| 6 | Fingerprint v1 tính ở agent; dòng nguồn chỉ băm; `anchorOverride` cho bí mật/lỗ hổng gói | §5.5 |
| 7 | Thiếu dữ liệu → `unknown`/`env_not_ready`, không giả `pass`; `skipReason`, `ruleResults` là đề xuất cần sửa hợp đồng | PQ-26, F4 |
| 8 | Dữ liệu cấu hình là mã TS (agent đóng gói một `agent.js` bằng esbuild), không YAML | `agent/build.mjs` |
| 9 | Không `max-lines` disable (`.oxlintrc.json:89-101`: 300 dòng `.ts`); tên file theo khái niệm | AGENTS.md |
| 10 | Git: chỉ lệnh có từ < Git 2.25, giữ `-c` đầu lệnh, không `GitCapabilityCache` vì không dùng cờ mới | AGENTS.md, `git-compatibility.md` |
| 11 | Part A (direct-websocket và `--stdio`); Part B (`desktop/src/relay`) không có `quality.*`; Windows `unsupported_platform` | §1.1, §5 |

## Chỗ hợp đồng thiếu hoặc mâu thuẫn (tổng hợp; không sửa hợp đồng)

1. CodeGraph `commit:null` nhưng bảng `classifyIndexBasis` so commit (080).
2. Mẫu `argv` thiếu `{gitCommonDir}` (cho `buf breaking`) và `scopeArgv` (vitest `related`) (081-B).
3. Thiếu trường `skipReason` ở trạng thái bước; thiếu chỗ đặt `ruleResults[]` (PQ-26 nêu mảng nhưng §5.3/§5.5 không có) (081-A, 084).
4. Ví dụ `quality.progress` (`percent:16` khi bước 1 đang chạy) lệch định nghĩa `completed/stepCount` (081-A).
5. `quality` capability "≥ 1 profile ready" không xác định được ở handshake (không có `workspaceRoot`) (081-A).
6. `network_policy` cần nguồn quyết định ở agent (đề xuất `ORCA_QUALITY_NETWORK`, chưa có trong §2.4) (091).
7. §5 nói `relay-ssh` không có `quality.*` trong khi §1.1 nói Go chạy `--stdio` = Part A (081-A).
8. `results` của run `interrupted`/hết TTL và `coverage` hết TTL: hợp đồng không nêu (081-A, 083).
9. CR-084 ORCA-001..003 trùng profile `repo-check-*` (082): bí danh, không chạy lại (084).
10. Số `go.work` thực 21 mục (CR-081 ghi 22).
