# BE-CV-SOL-091-security-scan-flag-and-ingest: Cờ quét bảo mật theo tenant và nạp phát hiện `security`/`dependency` an toàn

> **📋 Proposed (P2).** Chưa triển khai, chưa chạy test nào. Phần backend của CR-CV-091; công cụ quét, parser, quét bí mật trên diff, `dependency-diff` là phần agent (`AG-CV-SOL-091-security-and-dependency-profiles`). Làm **sau** SOL-085 (C-DM §7.2).

**CR:** [CR-CV-091](../../../../../../docs/crs/v7/quality-signals/CR-CV-091-security-and-dependency-scanning.md)
**Service:** `code-intel-service` (mới)
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (cô lập tenant, audit append-only, input validation & supply chain: `govulncheck` trong CI), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

## 0. Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| **PQ-01 (4)** | `quality_security_scan_enabled` tắt ⇒ profile `security-*`/`dependency-diff` **không có** với tenant (không liệt kê trong `runnableProfiles`); `StartQualityRun` với tên đó ⇒ **`CODEINTEL_PROFILE_UNKNOWN`** (không `CODEINTEL_DISABLED` như CR-091 2.7) |
| PQ-24 | Hiệu lực = `… ∧ quality_gate_enabled ∧ quality_security_scan_enabled`; cache cờ ≤ 5 s; lỗi đọc = tắt |
| PQ-26 | `ruleId`: `SEC-GOVULN/<id>`, `SEC-OSV/<id>`, `SEC-SECRET/<loại>`, `DEP-*`; regex `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$` |
| H8 | Không secret trong kết quả/log/cache/span/push; quét bí mật chỉ vị trí và loại |
| C-DM T1, §6.1, §6.3 | Cột `quality_security_scan_enabled` (đã trong `0002`), admin đổi (audit); action OPA chỉ 8 giá trị có sẵn (không thêm) |
| C-AG §3.2, §5.1 | `CODEINTEL_ENV_NOT_READY` (`missingTools[]`, `reason` ∈ `network_policy`, `go_modules_unavailable`); `kind ∈ security\|dependency` trong `listProfiles`; lọc theo cờ là việc của backend |
| §8.3 | Hai dialect cho phần chạm DB (không bảng mới); cô lập tenant |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: ba file hợp đồng, CR-091 (1.1: repo không có công cụ quét; `pr.yml`, `Makefile`, `go.work`), `.github/workflows/` (liệt kê 18 workflow `backend-go-*`), `backend-go/.golangci.yml` (không `gosec`), `backend-go/common/{tenant/tenant.go, dbcapability/capability.go}`. `code-intel-service` và `proto/orca/codeintel` chưa tồn tại (`ls`). Chưa chạy gì; không có công cụ quét nào trên máy soạn (theo CR).

### Correction relative to CR / hợp đồng

| # | CR-091 nói | Hợp đồng / mã thật | Xử lý |
|---|---|---|---|
| C1 | Cờ tắt ⇒ `StartQualityRun` trả `CODEINTEL_DISABLED` (2.7, tiêu chí 4) | PQ-01(4): `CODEINTEL_PROFILE_UNKNOWN` | Theo hợp đồng; frontend không ẩn cả tính năng |
| C2 | Tắt ⇒ profile không xuất hiện trong `quality.listProfiles` | Agent không biết cờ tenant (C-AG §5.1) | Backend lọc `runnable_profiles` ở `GetQualityProfile`, không đổi agent |
| C3 | Chỉ nêu **profile** | Agent có `suites` (`fast/standard/full`); một suite có thể chứa profile bảo mật ⇒ chạy quét mạng dù cờ tắt | Suite chứa profile `kind∈{security,dependency}` coi như profile bảo mật (ẩn + `PROFILE_UNKNOWN` khi cờ tắt); yêu cầu `AG-CV-SOL-081` không đưa profile bảo mật vào suite mặc định (mục 7) |
| C4 | Tên profile (`security-go-vuln`…) | C-AG §5.1: tên là đề xuất, CR-081 chốt | Lọc theo `kind` **hoặc** tiền tố tên, không chỉ tên |
| C5 | Quét `scope:"worktree"` cần "admin project" (Q4) | Hợp đồng chỉ có 8 action OPA, `StartQualityRun`=`review_write` | Dùng action có sẵn `quality_profile_write` (owner/admin) cho `scope=worktree` + profile bảo mật; câu hỏi mở Q3 |
| C6 | Bí mật "không dẫn xuất từ giá trị (độ dài, tiền tố)" | `QualityFinding` có `column/end_column`; hiệu số lộ độ dài | Backend đặt `col=end_col=0` cho `SEC-SECRET/*`, thay `message`/`fix_hint` bằng văn bản cố định theo loại |
| C7 | Bật cờ cần admin + audit | `SetSettings` thuộc SOL-073/013 | Không làm lại; solution chỉ kiểm thử đường đó và ghi audit khi **chạy** quét |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/services/code-intel-service/internal/
  domain/security_profile_visibility.go   # IsSecurityProfile, FilterRunnable, SuiteContainsSecurity
  domain/security_finding_rules.go        # bảng loại bí mật -> message/fixHint cố định; ruleId prefix
  usecase/security_scan_policy.go         # ProfileGate: Allow(start), Filter(runnable)
  usecase/security_finding_sanitizer.go   # FindingPostProcessor cho IngestQualityRun (SOL-082)
```

### B. `ProfileGate` (PQ-01)

`IsSecurityProfile(p)`: `p.kind ∈ {security, dependency}` **hoặc** `p.id` bắt đầu `security-` hoặc bằng `dependency-diff`. `Filter(runnable, flags)`: cờ tắt ⇒ bỏ mọi profile bảo mật và mọi suite chứa chúng. `AllowStart(profileOrSuite, scope, role)`: cờ tắt ⇒ `CODEINTEL_PROFILE_UNKNOWN` (kèm `available[]` đã lọc); `scope=worktree` + bảo mật ⇒ yêu cầu `quality_profile_write`. SOL-085 gọi `Filter` trong `GetQualityProfile` và `AllowStart` trong `StartQualityRun` (cổng cắm, SOL-091 không sửa file SOL-085 ngoài điểm gọi). Danh sách profile lấy từ `quality.listProfiles` (cache 60 s ở agent) — backend không giữ lệnh.

### C. `security_finding_sanitizer` (hậu xử lý ở `IngestQualityRun`)

Với mỗi finding `category ∈ {security, dependency}`:
- `ruleId` phải khớp tiền tố cho phép (`SEC-GOVULN/`, `SEC-OSV/`, `SEC-SECRET/`, `DEP-`), ngược lại bỏ dòng và đếm.
- `SEC-SECRET/<loại>`: `message` và `fix_hint` **bị thay hoàn toàn** bằng bảng cố định theo loại (loại lạ ⇒ loại `unknown`); `col=end_col=0`; giữ `file`, `line`, `end_line`; `fingerprint` do agent tính (backend kiểm độ dài/hex, không thể xác minh nguồn gốc).
- `SEC-GOVULN`/`SEC-OSV`/`DEP-*`: `message` ≤ 2 KiB đã che, không chứa đường dẫn tuyệt đối.
- Mọi dòng qua `FindingSanitizer` chung (SOL-082) **trước** bước này; sau bước này không nơi nào còn văn bản gốc của agent cho `SEC-SECRET`.

### D. Audit và quyền

`StartQualityRun` cho profile bảo mật ghi audit (`AuditWriter` của SOL-013; `auditclient.Append` hiện thiếu `actor_type/target_type` và nuốt lỗi — C-DM §3.3 — nên ghi vào `details` tới khi SOL-013 sửa). Bật/tắt cờ: do `SetSettings` (admin + audit) của SOL-073/013, solution chỉ có test tích hợp.

### E. Lỗi môi trường

`CODEINTEL_ENV_NOT_READY` với `reason ∈ {network_policy, go_modules_unavailable}` và `missingTools[]` được SOL-023 chuyển nguyên (trailer ≤ 4 KiB, hậu tố ≤ 2 KiB); solution thêm hai giá trị vào danh sách `reason` cho phép ở bộ lọc dữ liệu lỗi (PQ-02) và test hợp đồng.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Lọc theo `kind` + tên + suite | Tên profile chưa chốt; suite có thể lách cờ |
| Thay hoàn toàn message `SEC-SECRET` | Chặn rò rỉ ngược (CR S3), phòng agent lỗi |
| Zero cột | Hiệu số cột lộ độ dài |
| `PROFILE_UNKNOWN` khi cờ tắt | Không làm frontend ẩn cả tính năng (PQ-01) |
| Dùng action OPA có sẵn | Không thêm action ngoài C-DM §6.3 |

## 4. Tiêu chí chấp nhận

- [ ] Cờ tắt: `runnable_profiles` không chứa profile/suite bảo mật; `StartQualityRun` ⇒ `CODEINTEL_PROFILE_UNKNOWN` (không agent call); lỗi đọc cờ ⇒ như tắt.
- [ ] Cờ bật nhưng `quality_gate_enabled` tắt ⇒ `CODEINTEL_QUALITY_GATE_DISABLED` (thứ tự ưu tiên đúng).
- [ ] `scope=worktree` + profile bảo mật bởi `member` ⇒ bị từ chối; `changed` ⇒ cho phép như `review_write`.
- [ ] `SEC-SECRET/*`: message/fix cố định, `col=end_col=0`; canary trong **mọi** trường của đầu vào không xuất hiện ở DB, log, sự kiện outbox, lỗi, span.
- [ ] `ruleId` ngoài tiền tố cho phép bị bỏ; regex PQ-26 được kiểm.
- [ ] `ENV_NOT_READY (network_policy|go_modules_unavailable)` tới client với `missingTools[]`.
- [ ] Mỗi lần chạy profile bảo mật có audit; tenant A không thấy run/finding của tenant B.
- [ ] Không `max-lines` disable; tên file không `helpers/utils/common/misc`.

## 5. Kiểm thử (chưa chạy test nào)

Unit: `security_profile_visibility_test.go` (kind, tên, suite), `security_scan_policy_test.go`, `security_finding_sanitizer_test.go`. Canary: chuỗi bí mật giả đặt vào `message`, `fixHint`, `file`, `ruleId`, `stepId` của agent giả ⇒ quét toàn bộ đầu ra (DB qua truy vấn, log capture, outbox, lỗi). Tích hợp hai dialect cho đường Ingest + cờ; cô lập tenant (CR-072).

## 6. Rủi ro và chưa kiểm chứng

- Chưa có công cụ quét nào được duyệt (O-9/O12); agent contract cho profile bảo mật chưa chạy.
- Che bí mật bất hoàn hảo; chặn thật là "không lưu gì dẫn xuất từ giá trị" (agent) + thay thế ở backend.
- `fingerprint` bí mật do agent tính; backend không xác minh nó không dẫn xuất từ giá trị.
- Egress tới `osv.dev`/`vuln.go.dev` do chính sách tenant (Q2 CR-091): backend chỉ chuyển lỗi `network_policy`.
- Thứ tự phụ thuộc: sau SOL-085 (điểm cắm `ProfileGate`).
- Go CI 1.25 vs `go.work` 1.26; SSH: không đổi; Git/Provider: không liên quan.

## 7. Điểm hợp đồng thiếu/mâu thuẫn (không tự sửa)

1. C-AG không nêu suite mặc định có chứa profile bảo mật hay không (C3).
2. CR-091 2.7 (`CODEINTEL_DISABLED`) mâu thuẫn PQ-01 (đã theo PQ-01).
3. C-DM không nêu quyền chạy quét toàn bộ (Q4 CR-091); đề xuất dùng `quality_profile_write`.

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-091-security-and-dependency-profiles` | Profile, parser, quét bí mật chỉ vị trí; suite mặc định không chứa profile bảo mật |
| AG | `AG-CV-SOL-081-quality-profile-catalog-and-preflight` | `kind` trong `listProfiles` |
| BE | `BE-CV-SOL-085-quality-gate-evaluator-and-profiles` (điểm cắm), `BE-CV-SOL-082-quality-run-storage-and-ingest` (hậu xử lý), `BE-CV-SOL-013-authorization-flags-and-audit`, `BE-CV-SOL-073-settings-flag-and-rollout`, `BE-CV-SOL-072-security-tests-service-gateway` (canary), `BE-CV-SOL-023-infra-fleet-codeintel-transport` | Theo C-DM §7.2: `091-BE (sau 085)` |
| FE | `FE-CV-SOL-073-flag-gating-and-web-e2e`, `FE-CV-SOL-087-quality-scorecard-and-state` | Cờ trong settings; hiển thị category `security/dependency`; xử lý `PROFILE_UNKNOWN` |

## 9. Câu hỏi mở

- **Q1.** Công cụ nào được duyệt (O-9)? **Q2.** Chính sách egress. **Q3.** Quyền quét toàn bộ (C5). **Q4.** Suite mặc định tách riêng profile bảo mật (C3)?

## 10. Tham chiếu

[CR-CV-091](../../../../../../docs/crs/v7/quality-signals/CR-CV-091-security-and-dependency-scanning.md); C-DM PQ-01/24/26, T1, §6.1; C-AG §3.2, §5.1; `/opt/repos/orca/AGENTS.md`.
