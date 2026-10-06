# Feature: quality-signals — Tín hiệu chất lượng thật cho "Xem code & kiểm soát chất lượng"

> **Trạng thái:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code chỉ-đọc ngày 2026-10-06; chưa chạy hệ thống, chưa viết code.
> **Hợp đồng chung:** [`../README.md`](../README.md) (O9-O14, mục 3.10, mục 8). **Nguồn:** [research 11](../../../research/view-code/11-additions-for-quality-control.md).

## 1. Mục tiêu

Cho Orca các **tín hiệu chất lượng đo được** trên worktree do agent sửa, trước khi có PR: index bắt kịp code vừa sinh, chạy kiểm tra có tên, chuẩn hoá phát hiện, độ phủ test, quy ước dự án, kết quả CI, và (tuỳ chọn) bảo mật/phụ thuộc. Folder này **không** quyết định cổng đạt/không (CR-CV-085, `quality-gate`) và không vẽ UI (CR-CV-087, `quality-visualization`).

## 2. Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc chính | Nguồn research 11 |
|----|-----|----------|--------|-----------------|-------------------|
| [CR-CV-080](./CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md) | Chiến lược index cho worktree của agent, tự làm mới khi agent xong | 🔴 P0 | Large | 001, 004, 012, 023 | A1-A3 |
| CR-CV-081 ([`CR-CV-081-quality-runner-on-agent.md`](./CR-CV-081-quality-runner-on-agent.md)) | Bộ chạy kiểm tra trên agent: profile có tên, tiến độ, huỷ, giới hạn tài nguyên, sẵn sàng môi trường (`quality.*`) | 🔴 P0 | Large | 001 | B1, E2, E3 |
| CR-CV-082 ([`CR-CV-082-quality-finding-model-and-parsers.md`](./CR-CV-082-quality-finding-model-and-parsers.md)) | `QualityFinding`, `QualityRun`, parser (oxlint, tsc, vitest, go vet/test, golangci-lint, buf, opa) kèm fixture | 🔴 P0 | Large | 081, 011 | B2, E4 |
| [CR-CV-083](./CR-CV-083-coverage-and-diff-coverage.md) | Coverage và diff coverage (Go trước; TS sau khi duyệt `@vitest/coverage-v8`); fallback "ước lượng" | 🟠 P1 | Large | 081, 082, 036 | B3 |
| [CR-CV-084](./CR-CV-084-project-convention-rule-pack.md) | Rule pack quy ước Orca (`ORCA-xxx`) từ `check-*`, `pr.yml`, AGENTS.md | 🟠 P1 | Large | 081, 082 | B6 |
| [CR-CV-086](./CR-CV-086-ci-and-local-results-merge.md) | Gộp kết quả CI/PR (GitHub, GitLab) với cục bộ: `source="ci"`, so khớp SHA, rate limit | 🟠 P1 | Large | 082, 012 | B7 |
| [CR-CV-091](./CR-CV-091-security-and-dependency-scanning.md) | Quét bảo mật/phụ thuộc (tuỳ chọn, mặc định tắt): lỗ hổng, bí mật trên diff, diff phụ thuộc | ⚪ P2 | Medium | 081, 082, 013 | B5 |
| [CR-CV-094](./CR-CV-094-gitnexus-unused-tools-spike.md) | Spike: đánh giá tool GitNexus chưa dùng (`check`, `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query`, `group_*`) | ⚪ P2 | Small | 002, 037, 038 | E6 |

CR-CV-080, 081, 082 do người soạn khác viết đồng thời; README này chỉ tham chiếu theo ID và hợp đồng README v7 mục 3.10. CR-CV-083, 084, 086, 091, 094 bám đúng các tên đó.

## 3. Thứ tự thực thi

```
CR-080 (index) ─────────────────────────────┐
CR-081 (bộ chạy) ─▶ CR-082 (mô hình+parser) ─┼▶ CR-083 (coverage)   ─┐
                                             ├▶ CR-084 (rule pack)  ─┼▶ CR-085 (cổng, quality-gate)
                                             ├▶ CR-086 (gộp CI)     ─┘
                                             └▶ CR-091 (bảo mật, P2, sau 085)
CR-094 (spike): sau khi CR-085 ổn định; độc lập với các CR khác trong folder
```

Theo README v7 mục 5: đợt 7 (080, 081, 082), đợt 8 (084, 083, 086 cùng 085), đợt 9 (091, 094). CR-083, 084, 086 chạy song song được sau 082; chỉ chung `quality_runs`/`quality_findings`. CR-091 dùng lại khuôn lấy "dòng thêm của diff" của CR-083 (2.5) và CR-084 (2.3): nên thống nhất một thành phần phân tích diff trên agent.

## 4. Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Mọi tín hiệu đi qua **profile có tên** của CR-CV-081; không nhận lệnh/args tự do (O11) | Chống chạy mã tuỳ ý; nhất quán với D5 |
| F2 | Mọi tín hiệu thành `QualityFinding`/`QualityRun` (README 3.10) với `fingerprint` không chứa số dòng; cột `category` đúng tập: `lint\|typecheck\|test\|coverage\|complexity\|security\|dependency\|convention\|architecture\|ai` | Một mô hình để lọc, miễn trừ, xu hướng |
| F3 | Phân tích dựa trên **diff của agent** (dòng thêm / tệp đổi) cho quy ước, coverage, bí mật | Không phạt nợ cũ; khớp "agent vừa làm gì" |
| F4 | Thiếu dữ liệu → `unknown`, không bao giờ `pass`; số ước lượng luôn gắn nhãn khác số đo | README 3.10; research 11 §7 |
| F5 | Công cụ/phụ thuộc mới (`@vitest/coverage-v8`, `govulncheck`, `osv-scanner`, `gitleaks`) cần duyệt riêng, không thêm mặc định (O12); quét bảo mật mặc định tắt | O12 |
| F6 | Kết quả luôn ghi `headCommit`, `indexCommit`, `toolVersion`, `source` (`local`/`ci`) | Tránh kết luận từ dữ liệu cũ hoặc lẫn nguồn |
| F7 | Không đưa secret vào kết quả/log/cache; chỉ vị trí và loại (CR-CV-091) | README mục 6 |
| F8 | Trung lập nhà cung cấp git (GitHub, GitLab; `capability_unsupported` cho phần còn lại) | AGENTS.md "Git Provider Compatibility" |
| F9 | Chế độ chỉ báo (O9): các CR ở đây cung cấp dữ liệu, không chặn Create PR/commit | O9 |

## 5. Phạm vi ngoài feature này

- Cổng chất lượng, ngưỡng, miễn trừ, lịch sử/xu hướng: CR-CV-085 (`quality-gate`).
- UI scorecard, chú thích trên diff, biểu đồ: CR-CV-087/088 (`quality-visualization`).
- Dấu vết agent, báo cáo xuất được, AI review, truy vết yêu cầu, telemetry: CR-CV-089, 090, 092, 093, 095.
- Độ phức tạp/trùng lặp bằng `gocyclo`/`gocognit`/`dupl` (research 11 B4): chưa có CR (nên bàn ở CR-CV-082 hoặc CR-CV-085).
- Thay đổi CI chính thức (`pr.yml`, workflow `backend-go-*`): không sửa; tín hiệu cục bộ là phản hồi nhanh, CI vẫn là cổng chính thức.
- Phân tích tĩnh mới của mã (`gosec`, `semgrep`, taint): không làm ở đây (CR-CV-091 loại bước đầu; CR-CV-094 chỉ đánh giá).

## 6. Điều chỉnh hợp đồng đã phát hiện (khi viết 083, 084, 086, 091, 094)

README v7 **không được sửa**; khi README và CR khác nhau, theo CR.

| # | Nội dung | CR |
|---|---|---|
| 1 | **Cấu trúc repo đã tách** (`frontend/`, `desktop/`, `agent/`, `backend/`, `packages/*`; `pnpm-workspace.yaml`) nhưng `package.json` gốc và `pr.yml` vẫn trỏ `config/scripts/...`, `config/vitest.config.ts`, `config/tsconfig.*.json`. Ở gốc `config/scripts/` chỉ có `check-max-lines-ratchet.mjs` (+ test, `rebuild-native-deps.mjs`); các script `check-*`/`verify-*`, `reliability-gates.jsonc`, `vitest.config.ts` thật nằm ở `desktop/config/`. Hai bản `max-lines-baseline.txt` khác nhau (gốc nhiều hơn 260 dòng). Guard `.d.ts` của `pr.yml` tìm `src/preload src/shared` ở gốc (không tồn tại). README v7 mục 4 mô tả "`config/scripts/check-*`" và research "354 dòng baseline": cần hiểu theo `desktop/config/` (359 dòng, 336 `inline` + 19 `mobile-config`) | 084, 083 |
| 2 | README 3.10 chưa có RPC/kênh cho nhập CI: đề xuất thêm `RefreshCiRun` (`QualityGateService`) và kênh `codeIntel.quality.ci`; cần RPC mới `ListCommitChecks` ở `scmintegration.v1` (hiện không có check/pipeline) | 086 |
| 3 | `quality_runs` cần cột `provider`, `external_ref`, `external_url`, `fetched_at`, `stale_after`, `dirty` (cho `source="ci"` và so khớp SHA); `QualityProfile.config.ciMappings` | 086, 085 |
| 4 | `coverage_reports` cần cột cụ thể (`quality_run_id`, `repo_binding_id`, `base_commit`, `dirty`, `tree_hash`, `language`, `scope_key`, `source measured\|estimated`, `diff_executable`, `diff_covered`, `payload`…); khoá worktree theo `repo_binding_id` (id worktree có 3 dạng chuỗi, mục 8 #10) | 083 |
| 5 | Tên profile trong tài liệu này (`coverage-go`, `coverage-ts`, `repo-rules`, `repo-rules-scripts`, `security-go-vuln`, `security-deps-osv`, `security-secrets-diff`, `dependency-diff`) là **đề xuất**; tên cuối do CR-CV-081 chốt | 083, 084, 091 |
| 6 | Cờ tenant mới `quality_security_scan_enabled` (mặc định tắt) trong `tenant_settings` (mục 8 #10) | 091 |
| 7 | README mục 1 nói CLI GitNexus có các tool chưa dùng; thực tế `gitnexus 1.6.9` chỉ có CLI cho `check` và `group *`; `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query` chỉ có ở MCP (chưa dùng được qua whitelist CLI của D5). Chỉ mục Orca không có lớp PDG | 094 |
| 8 | `getPRChecks` của desktop bỏ qua tham số `headSha` và `PRCheckDetail` không mang SHA; nên so khớp SHA phải đi qua nguồn backend mới | 086 |
| 9 | CI dùng `go-version: "1.25"` trong `backend-go-task-service.yml` trong khi `backend-go/go.work` ghi `go 1.26.0`; `backend-go/Makefile` bỏ `cmd/orca-cli` khỏi `SERVICES` | 083 |

## 7. Việc phải kiểm chứng trước khi viết mã (tổng hợp)

1. Chạy `go test -covermode=set -coverprofile` trên 21 module Go (thời gian/RAM), xác nhận định dạng profile và `go tool cover -func` (CR-CV-083, S1).
2. Chạy từng script `check-*` từ `desktop/` để lấy mã thoát, định dạng đầu ra, thời gian; xác nhận cách repo thật sự chạy `pnpm lint` sau khi tách monorepo (CR-CV-084).
3. Xác nhận quyền token OAuth của backend cho check-runs/pipelines và quota thực (CR-CV-086).
4. Cài và đọc `--help` của `govulncheck`, `osv-scanner`, `gitleaks` trên dev server; chốt chính sách egress (CR-CV-091).
5. Chạy spike CR-CV-094 trên **bản sao**, không chạy `analyze` ở repo thật.

## 8. Điểm chưa ai chốt

- Tên profile và định dạng cấu hình profile (CR-CV-081); thành phần phân tích diff dùng chung (083/084/091).
- Có duyệt `@vitest/coverage-v8` và công cụ quét không (O12); chính sách mạng ra ngoài.
- Check CI nào là bắt buộc cho cổng.
- Nguồn thật của baseline `max-lines`, và của đường dẫn UI (`frontend/` hay `desktop/src/renderer`).
- Ngưỡng báo nhầm của luật `diff` (ORCA-013/014/015) chưa đo.
- Số đo hiệu năng chưa có (chưa chạy gì).
