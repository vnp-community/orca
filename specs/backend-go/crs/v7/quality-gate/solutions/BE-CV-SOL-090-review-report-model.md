# BE-CV-SOL-090-review-report-model: `ExportReviewReport` trả mô hình báo cáo có cấu trúc, tái lập được, đã che bí mật

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Backend chỉ trả **mô hình** (`ReviewReportModel`); dựng Markdown/HTML, dịch nhãn, chèn vào mô tả PR là việc của frontend (`FE-CV-SOL-090-review-report-export`).

**CR:** [CR-CV-090](../../../../../../docs/crs/v7/quality-gate/CR-CV-090-exportable-review-report.md)
**Service:** `code-intel-service` (mới) · `codeintel_review_report.proto` (mới) · dòng `rpc ExportReviewReport` trong `codeintel_quality_gate.proto`
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, PQ-03, PQ-04, PQ-32, §2.1 dòng 23, §3.2, §6.1, §6.3), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§3.2 `quality.report`, §4.7 `ReviewReportModel`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (§Layer contracts), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Audit logging, §Input validation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (§Resilience patterns)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-32 | `risk.level ∈ LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN` (chữ HOA; CR-090 viết thường là sai); enum khác chữ thường |
| PQ-04 | Request nhận `selector`; không `repo_binding_id` |
| PQ-01 | `quality_gate_enabled` tắt → `CODEINTEL_QUALITY_GATE_DISABLED` cho **cả** RPC (xem L1) |
| PQ-06 | `findings.top[].origin ∈ quality|structure` cho nguồn (hai kiểu phát hiện, không trộn danh sách) — đây là **nguồn dữ liệu**, khác `Finding.origin` (introduced…) của PQ-06; tên trùng, cần tránh nhầm (xem L5) |
| PQ-30 | `contracts.*` lấy từ `ContractChange` (`compatibility`, `kind`) của CR-038 |
| PQ-34 | `gate.reasons[]` là `QualityGate.reasons[]` nguyên dạng (có `category`, `tool`) |
| §3.2 | `ExportReviewReport(selector, base_ref?, profile_name?, turn_key?, sections[], max_findings, max_reading_steps, include_people, include_waiver_reasons) → {model, generated_for, warnings[]}`; quyền `quality_read ∧ read`, kênh `codeIntel.quality.report` |
| H6, H7, H8 | Chỉ khoá/`code`+`params`; thiếu dữ liệu hiện nguyên trạng; không mã nguồn, không đường dẫn tuyệt đối |
| ui-api §2.4 | Trần phản hồi 2 MiB; kênh 20 s, tham số ≤ 16 KiB; `maxFindings ≤ 50`, `maxReadingSteps ≤ 50` ngoài khoảng bị từ chối (không kẹp ngầm) |

### 0.2 Lệch giữa CR và hợp đồng

| # | CR nói | Hợp đồng / phát hiện | Xử lý |
|---|---|---|---|
| L1 | Q4: khi chỉ `code_intel_enabled` bật, vẫn xuất báo cáo không có phần cổng | PQ-01/§3.2: mọi RPC `QualityGateService` (có `ExportReviewReport`) → `CODEINTEL_QUALITY_GATE_DISABLED` khi `quality_gate_enabled` tắt | **Theo hợp đồng**: không xuất một phần. Nếu muốn khác, phải sửa hợp đồng (chuyển RPC sang `CodeIntelService`); ghi Q1 |
| L2 | `review_report.proto` | PQ-07 `codeintel_review_report.proto` | Theo hợp đồng |
| L3 | `risk.level` chữ thường | PQ-32 chữ HOA | Theo hợp đồng |
| L4 | Mô hình (2.2) chứa `diagrams[].mermaid`, nhưng §2.1/2.8 lại giao việc dựng Mermaid cho frontend (`review-report-diagram-mermaid.ts`) | ui-api §4.7 giữ `diagrams[{kind, mermaid, alt, truncated}]` do backend trả | **Backend dựng văn bản Mermaid** (sinh chuỗi, không bố cục; Go không cần thư viện) từ dữ liệu C4/ERD; frontend dùng nguyên chuỗi và chỉ render SVG. Báo FE-CV-SOL-090 bỏ file `review-report-diagram-mermaid.ts` (hoặc chỉ dùng làm dự phòng); Q2 |
| L5 | `findings.top[].origin: "quality|structure"` | PQ-06 dùng `origin` cho `introduced|touched|preexisting|unknown` ở `Finding` | Giữ tên `origin` ở dạng dây của hợp đồng ui-api (đã cố định) nhưng domain Go đặt tên `source` để tránh nhầm, ánh xạ ở lớp gRPC |
| L6 | `provider` trong `subject` (github/gitlab/…) | Hợp đồng không nói backend lấy từ đâu; `Repo` (project-service) chỉ có `url` (đã grep `project.proto`), `scmintegration` có `ScmProvider` nhưng gọi cần repo slug | Điền **nỗ lực tối đa** từ `Repo.url` (host `github.com`→`github`, chứa `gitlab`→`gitlab`, `dev.azure.com`/`visualstudio.com`→`azure-devops`, chứa `gitea`→`gitea`, còn lại `""`); frontend ưu tiên `resolveSupportedHostedReviewCopyProvider` của nó. Chưa kiểm chứng với self-hosted; Q3 |
| L7 | `include_people`, `include_waiver_reasons` | Mô hình v1 (ui-api §4.7) **không có trường nào** chứa tên người hay lý do miễn trừ | Hai cờ được nhận, kiểm hợp lệ, **chưa tác động** (không có chỗ ghi) cho tới khi mô hình thêm trường additive; Q4 |
| L8 | `warnings[]` ví dụ `index_stale`, `no_gate`, `overlay_partial` | Hợp đồng không có bảng mã cảnh báo | Solution này đặt bảng mã (§2.5), đề nghị bổ sung vào hợp đồng; Q5 |

### 0.3 Phụ thuộc chéo khu vực

| Hướng | Solution | Dùng gì |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-090-review-report-export` (bộ dựng Markdown/HTML, nút "Chèn báo cáo review", kiểm `modelDigest`) · `FE-CV-SOL-051-review-workspace-shell` (menu xuất) · `FE-CV-SOL-093-ai-summary-panel` (chèn có nhãn, ngoài mục cổng) | JSON `ReviewReportModel`; golden fixture |
| AG | Không (§8.2) | — |
| BE | SOL-085-evaluator (`EvaluateQualityGate`), BE-CV-SOL-036 (`ChangeOverlay`, `RiskAssessment`, `readingOrder`), 037 (`ListFindings`), 038 (`ContractDiff`), 031/033 (`GetErd`, `GetArchitecture`), 082 (`ListQualityFindings`), 013 (che bí mật, danh sách đường dẫn bị chặn, hạn mức), 040-quality-channels | Port; thiếu nguồn → `warnings[]` |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-090 đủ; ba file hợp đồng; `proto/orca/project/v1/project.proto` (`Repo.url` có, `provider` không); `proto/orca/scmintegration/v1/scmintegration.proto` (`ScmProvider`); `common/auditclient/client.go`. Frontend (`CreateHostedReviewComposer*.tsx`, `MermaidBlock.tsx`) **không đọc lại** ở phiên này (thuộc FE).

Xác nhận: chưa có `code-intel-service`, chưa có proto `codeintel`, chưa có `GetChangeOverlay`/`ListFindings`/`GetErd`/`GetContractDiff` (do SOL-036/037/038/031 dựng).

### Correction relative to CR-CV-090

| # | CR nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | "Nguồn đọc từ cùng use case/cache của các view" | Các use case đó **chưa tồn tại** | Port `ReviewReportSources` gồm 6 phương thức; fake trong test; adapter nối vào use case thật của SOL-036/037/038/031/033/082/085 khi có |
| C2 | Audit `codeintel.report.export` "thêm vào bảng 2.4 CR-013" | `auditclient.Append(ctx, tenantID, actorID, action, target, outcome, ip)` chỉ có sáu tham số | Dùng đúng chữ ký; `modelDigest` ghi vào log `slog` `audit=true`, không vào `audit_log` |
| C3 | `secret_redactor.go` (CR-013 2.5) | Chưa tồn tại | Port `TextRedactor`, `PathPolicy` (danh sách chặn nội dung `.env`, `*.pem`…) do SOL-013 cung cấp |

## 2. Giải pháp chi tiết

### 2.1 Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_review_report.proto     # ReviewReportModel và phần con; ExportReviewReportRequest/Response
services/code-intel-service/internal/
  domain/review_report_model.go          # kiểu miền, ReviewReportBuilder (thuần)
  domain/review_report_digest.go         # JSON chuẩn hoá + sha256
  domain/review_report_diagram.go        # sinh văn bản Mermaid + alt[]; MermaidLabel() thoát nhãn
  domain/review_report_redaction.go      # quy tắc withheld, cắt message, bỏ đường dẫn tuyệt đối
  usecase/export_review_report.go        # ghép nguồn, lỗi từng phần → warnings
  usecase/review_report_ports.go         # ReviewReportSources, TextRedactor, PathPolicy
  adapter/grpc/review_report_server.go   # phương thức ExportReviewReport của QualityGateServer
```

### 2.2 Use case `ExportReviewReport`

Chuỗi (hợp đồng §3): cờ → OPA (`quality_read` **và** `read`) → `selector → binding` → kiểm tham số → thu thập → dựng → che/giới hạn → audit → trả. **Không gọi agent trực tiếp**; các nguồn đi qua use case/collector đã có (singleflight, hạn mức của SOL-013).

| Nguồn (port) | Cho phần | Lỗi/thiếu |
|---|---|---|
| `Overlay` (`GetChangeOverlay`) | `subject`, `summary`, `risk`, `readingOrder`, `contracts.tables`, `contracts.protoRpc|wsChannels` (qua `touchedTables`, `touchedContracts`) | `warnings += overlay_unavailable`, phần vắng |
| `Gate` (`EvaluateQualityGate`) | `gate`, `reproducibility.profileRef/runIds` | cổng `unknown` giữ nguyên (không thay `pass`); `warnings += no_gate` chỉ khi **không tính được** (lỗi nguồn); thiếu run ⇒ `gate.verdict=unknown` + `reasons[]` |
| `QualityFindings`, `StructureFindings` | `findings.counts/top` | `findings_unavailable` |
| `ContractDiff` | `contracts.*` | `contracts_unavailable` |
| `Architecture`, `Erd` | `diagrams[]` | `diagrams_unavailable`; sơ đồ bỏ chứ không cắt giữa |

Thất bại **từng phần** không làm thất bại cả RPC; chỉ lỗi cờ/quyền/`selector`/tham số mới trả lỗi. Tổng JSON ≤ 2 MiB (hợp đồng PQ-14) — mô hình thực tế nhỏ (vài chục KB) nhờ giới hạn bên dưới.

**Giới hạn:** `max_findings` mặc định 10, tối đa 50; `max_reading_steps` mặc định 15, tối đa 50; `readingOrder`/`findings.top` cắt có `limits.truncated.*` và `limits.totalCounts`; sơ đồ: `components` ≤ 30 nút, `erd` ≤ 15 bảng, ≤ 6 KiB văn bản; vượt → `truncated=true` và **bỏ sơ đồ** (không cắt giữa). Sắp xếp xác định: `findings.top` theo `(severity desc, ruleId, file, line)`, tên thành phần/bảng theo thứ tự chữ cái.

### 2.3 Che bí mật, đường dẫn, mã thừa (`review_report_redaction.go`)

- Chuỗi tự do (`message`, tên symbol, tên bảng, mã lý do) qua `TextRedactor`; `message` ≤ 160 ký tự, bỏ xuống dòng; **không** có mã nguồn (chỉ đường dẫn tương đối gốc repo, tên symbol, số dòng).
- Đường dẫn khớp `PathPolicy` (chặn nội dung): giữ tên tệp, đặt `contentWithheld=true` (trường additive của `findings.top[]`, quyết định của solution này, chưa có trong ui-api §4.7; Q6) và **bỏ `message`**.
- Không có đường dẫn tuyệt đối, `workspace_root`, hostname dev server, URL cục bộ; mọi chuỗi được quét `^(/|[A-Za-z]:\\)` ≥ 3 phân đoạn → `<path>` (cùng quy tắc ui-api §2.3).
- Không có tên người/email trong v1 (L7).
- Không có lý do miễn trừ trong v1 (L7); chỉ `gate.waivers{count, earliestExpiry}`.

### 2.4 Tái lập: `modelDigest`

`reproducibility.modelDigest` = sha256 hex của **JSON chuẩn hoá** của mô hình **không** có `generatedAt`/`modelDigest`: dùng struct miền với thứ tự trường cố định, slice đã sắp, map thay bằng slice cặp khoá-sắp (`toolVersions`, `byCategory`, `totalCounts` được chuyển qua một hàm sắp khoá). **Không** dùng `protojson` làm chuỗi gốc để băm (đầu ra không đảm bảo ổn định giữa bản). Cùng `(headCommit, baseCommit, indexCommit, profileRef, runIds, tham số)` ⇒ cùng digest ở hai lần gọi, hai dialect, hai tiến trình (có test chéo bằng golden). `generated_for` = `{commit, index_commit, stale}` ở response (ngoài digest).

### 2.5 Bảng mã `warnings[]` (đề xuất, L8)

`index_stale`, `no_gate`, `overlay_partial`, `overlay_unavailable`, `findings_unavailable`, `contracts_unavailable`, `diagrams_unavailable`, `diagrams_truncated`, `provider_unknown`. Mã là khoá ổn định, không câu chữ.

### 2.6 Quyền, cờ, audit, giới hạn

- Cờ: `CODEINTEL_DISABLED` / `CODEINTEL_QUALITY_GATE_DISABLED` (L1). Không cần `read_source` (không mã nguồn).
- Audit `codeintel.report.export` (`target: review:<binding>`, `outcome: allowed`); `slog` `audit=true` kèm `modelDigest` (không nội dung).
- Hạn mức: dùng nhóm hạn mức đọc nặng của SOL-013 (đề xuất cùng nhóm với `GetChangeOverlay`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | RPC trả mô hình, không văn bản | H6; một catalog chuỗi ở frontend |
| D2 | Backend sinh văn bản Mermaid | Mô hình ui-api có `mermaid`; sinh chuỗi không cần bố cục (L4) |
| D3 | Không xuất một phần khi cờ chất lượng tắt | Theo hợp đồng (L1) |
| D4 | Digest dựa JSON chuẩn hoá tự viết | Tái lập; `protojson` không bảo đảm |
| D5 | Thất bại từng phần → `warnings[]` | Báo cáo hữu ích kể cả khi một nguồn lỗi; không điền giá trị giả |
| D6 | Mặc định ẩn người/lý do miễn trừ (và v1 chưa có chỗ chứa) | Mô tả PR có thể công khai hơn Orca |

## 4. Tiêu chí chấp nhận

- [x] Cùng đầu vào → cùng `modelDigest` (hai lần gọi, hai dialect, hai tiến trình); đổi `runIds` hoặc `indexCommit` → digest đổi.
- [x] Mô hình không chứa: đường dẫn tuyệt đối, `workspace_root`, email, tên người, lý do miễn trừ, mã nguồn, chuỗi khớp mẫu secret (token giả trong `message` → `[REDACTED]`).
- [x] Phát hiện thuộc tệp bị chặn nội dung không kèm `message`.
- [x] Thiếu run bắt buộc → `gate.verdict=unknown` (không `pass`); lỗi nguồn → `warnings[]` tương ứng; RPC vẫn thành công.
- [x] `risk.level` chữ HOA; `UNKNOWN` khi overlay không có.
- [x] `max_findings=51` hoặc `max_reading_steps=0/51` → `CODEINTEL_INVALID_PARAMS` (không kẹp ngầm); mặc định 10/15.
- [x] Sơ đồ Mermaid: nhãn có `"`, `]`, `;`, xuống dòng, backtick, `%%` không làm vỡ cú pháp (test thoát); > 6 KiB hoặc > 30 nút → `truncated=true`, sơ đồ bị bỏ.
- [x] Cờ chất lượng tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; người không có `quality_read` hoặc `read` → `CODEINTEL_NOT_AUTHORIZED`; audit được ghi.
- [x] Tenant khác không dùng được `selector` của tenant này (test cách ly).
- [x] Không file nào dùng `max-lines` disable; không file `helpers/utils`.

## 5. Kiểm thử

- **Unit (Go):** dựng mô hình từ fixture (overlay, gate, findings) → golden JSON; thiếu từng nguồn; sắp xếp xác định; digest không phụ thuộc thứ tự map; `MermaidLabel` với tên "xấu"; redaction; cắt giới hạn.
- **Use case:** port giả (từng nguồn lỗi/timeout), cờ tắt, quyền, audit.
- **Hợp đồng:** golden `ReviewReportModel` dùng chung FE (CR-CV-070 fixtures).
- **Hiệu năng:** overlay 2 000 tệp dưới ngân sách thời gian (CR-CV-071 chốt; chưa đo — không hứa số).
- **Thủ công (không thuộc solution):** dán Markdown vào PR GitHub / MR GitLab (việc của FE).

## 6. Rủi ro và điểm chưa kiểm chứng

- Hình dạng thật của `ChangeOverlay`, `Finding`, `ContractDiff` chưa chốt (CR-036/037/038 là đề xuất): mô hình có thể phải chỉnh theo.
- `provider` từ `Repo.url` chưa kiểm chứng với self-hosted GitLab/Gitea; sai chỉ ảnh hưởng nhãn "PR/MR" nếu FE không tự suy.
- Mô tả PR là bề mặt công khai; tên tệp/bảng/service **không** ẩn (chỉ ẩn người, lý do). Cần người dùng chủ động bấm "Chèn" (FE).
- Bộ che bí mật dùng chung chưa có chủ (O-16).
- Giới hạn độ dài mô tả PR/MR và việc render Mermaid chưa kiểm chứng (CR §6) — thuộc FE nhưng ảnh hưởng ngân sách `diagrams`.

## 7. Câu hỏi mở

- **Q1.** Xuất báo cáo không phần cổng khi chỉ `code_intel_enabled` bật (CR Q4): đổi hợp đồng để chuyển RPC sang `CodeIntelService`?
- **Q2.** Chốt chủ dựng chuỗi Mermaid (backend theo hợp đồng) với FE-CV-SOL-090.
- **Q3.** Nguồn `provider` đáng tin hơn `Repo.url` (ví dụ `GetPullRequestForBranch`/cấu hình tích hợp)?
- **Q4.** Thêm trường additive cho `include_people`/`include_waiver_reasons` (vd `gate.waivers.reasons[]`, `people[]`) hay bỏ hai tham số?
- **Q5.** Đưa bảng mã `warnings[]` vào hợp đồng.
- **Q6.** Thêm `contentWithheld` vào `findings.top[]` của `ReviewReportModel` (ui-api §4.7).
- **Q7 (CR).** Cập nhật mô tả PR đã tạo, tool MCP/CLI xuất báo cáo (cần dựng văn bản phía Go): ngoài phạm vi.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-090-exportable-review-report.md`
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md`, `CONTRACT-codeintel-ui-api.md` (§4.7)
- `/opt/repos/orca/backend-go/proto/orca/project/v1/project.proto` (`Repo.url`), `/opt/repos/orca/backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
