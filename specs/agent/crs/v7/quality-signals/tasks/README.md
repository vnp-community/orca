# quality-signals (v7) tasks: index (agent)

> 📋 Proposed, chưa triển khai. Mọi task bắt đầu `Status: [ ] TODO`. Lệnh test chạy từ `/opt/repos/orca/agent` (`pnpm exec vitest run <file>`); CI hiện không chạy test của `agent/` (hợp đồng §10) nên cần job `code-intel-contract` của CR-CV-070. Solution: [`../solutions/README.md`](../solutions/README.md).

Task đầu tiên của mỗi solution có nhiều "chưa chạy" là task **thu fixture/bằng chứng thật** (080-01, 081-11, 082-01, 083-01, 084-01, 091-01): làm trước khi viết logic phụ thuộc định dạng công cụ.

| Task | Tiêu đề | Depends on (chính) |
|---|---|---|
| AG-CV-TASK-080-01 | Thu fixture `codegraph status`/`gitnexus status`/`meta.json` | — |
| 080-02 | `classifyIndexBasis` | 080-01 |
| 080-03 | Probe git: mergeBase, changedFilesNotInIndex, dirty | 080-01 |
| 080-04 | `host` snapshot | — |
| 080-05 | Trường index basis + `host` vào `codeintel.status` | 080-02..04, SOL-001/002/003 |
| 080-06 | `reindex`: `trigger`, `ifStale`, `expectHead`, `skipped_scope_repo_root` | 080-02, 080-03, SOL-004 |
| 080-07 | `indexChanged` thêm trường | 080-02, 080-06 |
| 081-01 | Env con allowlist/deny | — |
| 081-02 | Che secret/đường dẫn | — |
| 081-03 | Diệt cây tiến trình | — |
| 081-04 | Executor một bước + kiểu chạy | 081-01, 081-03 |
| 081-05 | Cổng việc nặng + nối reindex | 081-04, SOL-004 |
| 081-06 | Run manager, journal, hàng đợi | 081-04, 081-05 |
| 081-07 | Kho kết quả, phân trang | 081-02, 081-06 |
| 081-08 | Dispatcher `quality.*`, method table, mã lỗi | 081-06, 081-07, SOL-001 |
| 081-09 | Thông báo progress/finished, capability | 081-06, 081-08 |
| 081-10 | Schema profile, `definitionHash` | — |
| 081-11 | Bằng chứng catalog (`--help`, `--version`, tệp) | — |
| 081-12 | Catalog tích hợp + suite | 081-10, 081-11 |
| 081-13 | Ghi đè theo host (L2) | 081-10, 081-12 |
| 081-14 | Gốc worktree, tệp đổi, `dirtyFingerprint` | SOL-001 |
| 081-15 | Lập kế hoạch run | 081-01, 081-10, 081-12, 081-14 |
| 081-16 | Preflight + bộ phân giải binary | 081-10, 081-12 |
| 081-17 | `quality.listProfiles` | 081-08, 081-13, 081-16, 080-04 |
| 081-18 | Nối `quality.run` | 081-06, 081-08, 081-15, 081-16 |
| 082-01 | Repo mẫu, script chụp, MANIFEST | 081-11 |
| 082-02 | Kiểu parser, ánh xạ đường dẫn | — |
| 082-03 | Fingerprint v1 | 082-02 |
| 082-04 | Pipeline, giới hạn, suy trạng thái | 082-02, 082-03, 081-02 |
| 082-05 | Parser oxlint/tsc/vitest | 082-01, 082-02, 082-04 |
| 082-06 | Parser go vet/go test | 082-01, 082-02, 082-04 |
| 082-07 | Parser golangci/buf/opa | 082-01, 082-02, 082-04 |
| 082-08 | Parser `check-*` | 082-02, 082-04 |
| 082-09 | Hỗ trợ phiên bản + registry | 082-04..08, 081-04, 081-06 |
| 083-01 | Bằng chứng coverage Go | — |
| 083-02 | Parse coverprofile | 083-01 |
| 083-03 | Diff dòng thêm (dùng chung 084/091) | 081-14 |
| 083-04 | Tính diff coverage | 083-02, 083-03 |
| 083-05 | Profile `coverage-go` + bộ thu | 083-02..04, 081-12, 081-15, 081-06 |
| 083-06 | Handler `quality.coverage` | 083-05, 081-08 |
| 083-07 | Giai đoạn B vitest (cần duyệt) | 083-04, duyệt O12 |
| 084-01 | Bằng chứng script `check-*` | 082-01 |
| 084-02 | Schema và dữ liệu pack | — |
| 084-03 | Bộ so khớp diff | 084-02, 083-03 |
| 084-04 | Luật diff ORCA-007, 010-015 | 084-03 |
| 084-05 | Định vị và chạy script rules | 084-01, 084-02, 081-04, 082-04 |
| 084-06 | `ruleResults[]` | 084-04, 084-05, 081-07 |
| 084-07 | Profile `repo-rules*` | 084-04..06, 081-12, 081-15 |
| 091-01 | Bằng chứng công cụ quét (cần duyệt) | duyệt O12/O-9 |
| 091-02 | Che bí mật + canary | 081-02 |
| 091-03 | Bộ quét bí mật tích hợp | 091-02, 083-03, 082-04 |
| 091-04 | Diff phụ thuộc lockfile | 081-14, 082-04 |
| 091-05 | Parser govulncheck/osv | 091-01, 082-02, 082-04 |
| 091-06 | 4 profile + gating | 091-03, 091-04, 081-12, 081-16 |
| 091-07 | Adapter gitleaks (tuỳ chọn, cần duyệt) | 091-01, 091-02 |
| 091-08 | Canary đầu-cuối + dọn | 091-03, 081-06, 081-07 |

Tên file đầy đủ: `AG-CV-TASK-<CR>-<NN>-<slug>.md` trong thư mục này.
