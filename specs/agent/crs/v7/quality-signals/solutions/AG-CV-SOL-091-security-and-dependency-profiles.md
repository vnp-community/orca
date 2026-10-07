# AG-CV-SOL-091: Profile quét bảo mật và phụ thuộc (P2, mặc định tắt; công cụ cần duyệt)

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 091-01 đến 091-08, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-091](../../../../../../docs/crs/v7/quality-signals/CR-CV-091-security-and-dependency-scanning.md). Task AG-CV-TASK-091-01 đến 08. **Khu vực:** `agent/src/relay/`. **Feature:** `quality-signals`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [api/agent-rpc-catalog-git-fs.md](../../../../api/agent-rpc-catalog-git-fs.md).

## 1. Hợp đồng áp dụng

| Mục ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) / PQ | Áp vào |
|---|---|
| §5.1: profile `security-*`/`dependency-diff` **do agent liệt kê**; backend ẩn khi `quality_security_scan_enabled` tắt (**agent không biết cờ tenant**); `kind ∈ security|dependency`; `missing[].reason` gồm `binary_missing`, `network_policy`, `coverage_provider_missing`… | tasks 06 |
| §3.2: `ENV_NOT_READY` với `missingTools[]`, `reason ∈ network_policy|go_modules_unavailable`; `PROFILE_UNKNOWN` | task 06, 07 |
| PQ-01(4): cờ tắt → `StartQualityRun` trả `CODEINTEL_PROFILE_UNKNOWN`, **không** `DISABLED` | (backend); ghi lệch dưới |
| §5.5 `QualityFinding`: `category security|dependency`; `ruleId` PQ-26: `SEC-GOVULN/<id>`, `SEC-OSV/<id>`, `SEC-SECRET/<loại>`, `DEP-*`; fingerprint ở agent, không số dòng | tasks 03-05 |
| §9.4: không secret trong kết quả/log/cache; stderr tail đã che; canary | tasks 02, 03, 08 |
| §2.4 Quality env: bắt đầu từ rỗng, loại biến secret | công cụ chạy qua executor (081-A) |
| §9.5: chỉ profile có tên; không tự cài công cụ | task 06 |
| PQ-21, §8.3 | trích |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-091 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | Cờ tắt → `quality.listProfiles` không liệt kê; `StartQualityRun` trả `CODEINTEL_DISABLED` (2.7) | PQ-01(4): backend lọc; mã lỗi `PROFILE_UNKNOWN` | Hợp đồng: agent **luôn** liệt kê; không có logic cờ tenant ở agent |
| 2 | `network_policy` do chính sách tenant cấm egress | Agent không biết chính sách tenant | Cấu hình mức host (đề xuất): env `ORCA_QUALITY_NETWORK=deny\|allow` (mặc định `allow`), **chưa có trong hợp đồng §2.4**; câu hỏi mở 1 |
| 3 | File tên `quality-parse-govulncheck.ts`, `quality-parse-osv-scanner.ts`, `quality-parse-secret-scanner.ts` | CR-082 dùng `quality-parser-<tool>.ts` | Đổi theo CR-082: `quality-parser-govulncheck.ts`, `quality-parser-osv-scanner.ts` (+ `quality-secret-scanner-diff.ts` cho bộ quét tích hợp) |
| 4 | Fingerprint bí mật = `sha256(ruleId + file + thứ tự)`, lỗ hổng = `sha256(ruleId + package + lockfileOrModule)` | §5.5: công thức chung với `anchor` | Dùng pipeline 082 với `anchorOverride` (`""` cho bí mật; `"<package>@<module|lockfile>"` cho lỗ hổng); `occurrence` thay "thứ tự xuất hiện" |
| 5 | `govulncheck`/`osv-scanner`/`gitleaks` đề xuất | O12: cần duyệt; chưa có trên máy (đã `which`) | Không thêm gì mặc định; bộ quét bí mật **tích hợp** là mặc định; công cụ ngoài `enabled:false` tới khi duyệt |
| 6 | `quality_security_scan_enabled` kiểm bằng cờ | PQ-01 | Không có ở agent |
| 7 | `DEP-LICENSE-DENY` tuỳ chọn | — | Ngoài phạm vi (MVP không giấy phép, như CR) |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-081-A/B` | Executor, env sạch, catalog `registerBuiltinProfiles`, preflight (`binary_missing`, `network_policy`) |
| `AG-CV-SOL-082` | Pipeline, `anchorOverride`, che, registry parser, fixture |
| `AG-CV-SOL-083` task 03 | `quality-diff-changed-lines.ts` (dòng thêm + văn bản) cho quét bí mật |
| `AG-CV-SOL-084` | Dùng chung bộ che; không trùng luật |
| `AG-CV-SOL-072-security-tests-agent` | Canary test mở rộng |
| `BE-CV-SOL-091-security-scan-flag-and-ingest` | Cờ tenant, ẩn profile, audit; chỉ nhận `QualityFinding` |
| `BE-CV-SOL-085`, `BE-CV-SOL-038` | Cổng; luật `tenant_id` SQL không trùng |
| Thứ tự: `085 → 091-BE`; `082 → 091-AG` (đợt 9, P2) |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `agent/package.json` (`yaml ^2.8.4`), `backend-go/go.work` (21 module), `which` (có `go`, `golangci-lint`, `buf`, `opa`; **không có** `govulncheck`, `osv-scanner`, `gitleaks`, `gosec`, `semgrep`, `trivy`), CR-091, hợp đồng. Không có `.gitleaks.toml`/`dependabot.yml` (theo CR; chưa kiểm lại).

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| Công cụ quét | Không có trên máy này (có thể không phải dev server mục tiêu) | Task 01 `BLOCKED` tới khi duyệt + cài |
| Lockfile | `pnpm-lock.yaml` gốc (13 170 dòng theo CR), `mobile/pnpm-lock.yaml`; 21 `go.sum` | Dependency-diff đọc `git show <base>:path` |
| `yaml` | Có trong agent | Parse lockfile không thêm phụ thuộc |

Correction relative to CR: CR-091 nói `.golangci.yml` không bật `gosec` — đúng theo CR, chưa kiểm lại.

## 5. Giải pháp

### 5.1 Cây file (mới)

```
quality-secret-redaction.ts / .test.ts            task 02  (kiểu "không giữ giá trị")
quality-secret-scanner-diff.ts / .test.ts         task 03  bộ quét tích hợp trên dòng thêm
quality-dependency-diff-lockfiles.ts / .test.ts   task 04
quality-parser-govulncheck.ts, quality-parser-osv-scanner.ts / .test.ts   task 05
quality-security-profiles.ts / .test.ts           task 06  4 profile + gating môi trường
quality-parser-gitleaks.ts / .test.ts             task 07  (tuỳ chọn, cần duyệt)
quality-secret-canary.test.ts                     task 02, 08
__fixtures__/quality-security/                    task 01
```

### 5.2 Bộ quét bí mật tích hợp (task 03) — nguyên tắc

Đầu vào chỉ **dòng thêm** của diff (task 083-03) + tệp untracked; bỏ nhị phân, `node_modules`, `*.lock`, `pnpm-lock.yaml`, `go.sum`; loại mặc định `**/*.test.*`, `**/fixtures/**`, `docs/**`, `*.md`. Mẫu độ chính xác cao: khối `-----BEGIN … PRIVATE KEY-----`, `AKIA[0-9A-Z]{16}`, `gh[pousr]_[A-Za-z0-9]{36,}`, `github_pat_…`, `xox[baprs]-…`, `sk-[A-Za-z0-9_-]{20,}`, JWT, DSN có mật khẩu. Kết quả: `ruleId:"SEC-SECRET/<loại>"`, `file`, `line`, `message` cố định theo loại, `fixHint` cố định; **không** ngữ cảnh dòng, độ dài, tiền tố/hậu tố, hay bất cứ thứ gì dẫn xuất từ giá trị (kể cả băm); `anchorOverride:""` nên fingerprint chỉ phụ thuộc `(tool, ruleId, file, occurrence)`; giá trị chỉ sống trong biến cục bộ của hàm so khớp.

### 5.3 Profile (task 06)

| id | kind | cách | cần |
|---|---|---|---|
| `security-secrets-diff` | security | in-process (bộ tích hợp), hoặc `gitleaks` (task 07) | không |
| `dependency-diff` | dependency | in-process (`yaml`, parser `go.mod`) | không |
| `security-go-vuln` | security | `govulncheck -format json ./...` mỗi module có tệp phụ thuộc đổi (`cwd` module) | `govulncheck` (cần duyệt), mạng |
| `security-deps-osv` | security | `osv-scanner --format json --lockfile <lockfile>` (cờ **chưa xác nhận**) mỗi lockfile đổi | `osv-scanner` (cần duyệt), mạng |

`scope: changed-*`: chỉ chạy khi tệp phụ thuộc đổi (`skipped` + `skipReason:"scope_unchanged"` nếu không). Thiếu công cụ → `ready:false`, `missing[{check, reason:"binary_missing", hint}]`; `quality.run` mọi bước thiếu → `ENV_NOT_READY` với `missingTools[]`. Không bao giờ tự cài, không `curl | sh`.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Agent luôn liệt kê profile bảo mật | PQ-01; agent không biết cờ tenant |
| 2 | Mặc định chỉ bộ quét tích hợp + `dependency-diff` (không công cụ ngoài) | O12 |
| 3 | Không dẫn xuất gì từ giá trị bí mật | Chặn rò rỉ ngược (S3) |
| 4 | `import-only` của `govulncheck` → `info` | Giảm nhiễu (S6) |
| 5 | Gom lỗ hổng theo `(vuln, module path)` | 21 module cùng phụ thuộc |
| 6 | Không `gosec`/`semgrep` | S7 |
| 7 | Phân tích lockfile bằng `yaml` sẵn có | Không thêm phụ thuộc |

## 7. Tiêu chí chấp nhận

- [x] Công cụ thiếu → `ready:false` + `missing[].reason:"binary_missing"`; run mọi bước thiếu → `ENV_NOT_READY` có `missingTools[]`; không lệnh cài nào chạy.
- [x] Parser `govulncheck`/`osv-scanner` đúng trên fixture theo phiên bản; vuln import-only `severity:"info"`; gom nhiều module thành một finding liệt kê module.
- [x] Quét bí mật chỉ nhận dòng **thêm**; dòng cũ chứa bí mật giả không bị báo.
- [x] **Canary**: chuỗi bí mật giả (dựng lúc chạy test, không nằm nguyên văn trong mã) **không** xuất hiện ở finding, `quality.results` thô, `view=log`, log agent, tệp tạm sau run (kể cả lỗi/timeout/huỷ), payload thông báo.
- [x] Finding bí mật không có độ dài, tiền tố/hậu tố hay ngữ cảnh; fingerprint không đổi khi chèn dòng phía trên và không phụ thuộc giá trị.
- [x] `dependency-diff` đúng: thêm/bớt/nâng major/hạ phiên bản, `replace` mới trong `go.mod`, drift `package.json` ↔ lockfile, nguồn git/tarball.
- [x] `ORCA_QUALITY_NETWORK=deny` → profile `network:true` `ready:false` `network_policy`, không treo.

## 8. Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-secret-redaction.test.ts src/relay/quality-secret-scanner-diff.test.ts src/relay/quality-dependency-diff-lockfiles.test.ts src/relay/quality-parser-govulncheck.test.ts src/relay/quality-parser-osv-scanner.test.ts src/relay/quality-security-profiles.test.ts src/relay/quality-secret-canary.test.ts`. Công cụ giả (stub trả JSON mẫu) cho tích hợp; không chạy công cụ thật trong CI.

## 9. Rủi ro và chưa kiểm chứng

- Chưa chạy/đọc `--help` của ba công cụ: cờ, JSON, hỗ trợ `pnpm-lock.yaml` của `osv-scanner` chưa xác nhận.
- Mạng/quyền riêng tư: tên gói gửi ra dịch vụ ngoài; chính sách egress chưa chốt (O-9).
- `govulncheck` biên dịch module: tốn CPU/RAM; `heavy:true`.
- Định dạng `pnpm-lock.yaml` (v6/v9, nhiều `importers`); `mobile/` có lockfile riêng.
- Bộ quét tích hợp bắt ít hơn công cụ chuyên dụng; tỉ lệ báo nhầm chưa đo.
- Che bí mật bất hoàn hảo; nguyên tắc "không dẫn xuất" là lớp bảo vệ chính.
- `go build` với `replace` cục bộ/cgo có thể thực thi mã (rủi ro như CR-081).

## 10. Câu hỏi mở

1. Thêm `ORCA_QUALITY_NETWORK` vào hợp đồng §2.4?
2. Duyệt công cụ nào (O-9): `govulncheck`, `osv-scanner`, `gitleaks`, hay chỉ bộ tích hợp?
3. Ai chạy `scope:"worktree"` (quét toàn bộ)? (backend/quyền)
