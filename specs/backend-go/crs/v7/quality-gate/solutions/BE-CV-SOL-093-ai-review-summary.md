# BE-CV-SOL-093-ai-review-summary: `GenerateReviewSummary` (mặc định tắt), danh mục dữ liệu gửi đi, chống injection, ngoại lệ timeout `ai.complete` 120 s

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Priority P2. Dữ liệu mã rời dev server tới nhà cung cấp LLM: mặc định `ai_review_level=off`.

**CR:** [CR-CV-093](../../../../../../docs/crs/v7/quality-gate/CR-CV-093-ai-review-summary.md)
**Service:** `code-intel-service` (mới) · `infra-fleet-service` (một dòng timeout) · `codeintel_ai_review.proto` (mới)
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, PQ-13, PQ-24, §2.1 dòng 24, §3.2, §3.3, §4.2 T1/T3, §6.1), [CONTRACT-codeintel-agent-rpc.md](../../CONTRACT-codeintel-agent-rpc.md) (§2.5 hàng `ai.complete`), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§3.2 `quality.summary` + ghi chú timeout, §4.7 `AiReviewSummary`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Input validation, §Audit logging), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§Talking to the Dev Server Agent), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (§Resilience patterns), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-01 | AI tắt (`ai_review_level=off` hoặc công tắc env) → `CODEINTEL_AI_REVIEW_DISABLED`; cờ chất lượng tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; cờ lớn → `CODEINTEL_DISABLED` |
| PQ-13 | `ai.complete`: agent 120 s, Go (`infra-fleet`) **120 s**; WS không chờ nổi: `quality.summary` (`dryRun:false`) trả `CODEINTEL_TIMEOUT` + `{"inProgress":true,"retryAfterMs":3000}` sau ≤ 24 s và **tiếp tục nền** (singleflight), cache 24 h |
| PQ-24 | Cột `ai_review_level`, `ai_review_model` đã ở T1 (migration `0002`); hiệu lực = `CODEINTEL_ENABLED ∧ tenant ∧ quality_gate_enabled ∧ ai_review_level≠off ∧ CODEINTEL_AI_REVIEW_ENABLED` |
| PQ-23 | Biến `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_AI_REVIEW_MODEL_ALLOWLIST` (CR; giá trị khởi điểm tiền tố `claude|gpt|o*|gemini`) |
| PQ-03 | Model ngoài allowlist → `CODEINTEL_INVALID_PARAMS`; hạn mức → `CODEINTEL_RATE_LIMITED`/`CODEINTEL_CONCURRENCY_LIMIT`; mã mới `CODEINTEL_AI_NO_RELAY`, `CODEINTEL_AI_BAD_OUTPUT` |
| §3.2 | `GenerateReviewSummary(base_ref?, profile_name?, level, dry_run, force_refresh, locale) → {summary, manifest, cache, labels}`; quyền `quality_read ∧ read_source`; kênh `codeIntel.quality.summary` |
| §3.3 | Đường gọi agent: `InfraFleetService.RelayByDevServer` |
| T3 | Cache `graph_snapshots` view `aiSummary`; TTL riêng **24 h** (ghi đè TTL 7 ngày mặc định), ≤ 32 KiB |
| H7/H8 | Không bao giờ vào cổng; không secret/mã trong log, span, cache ngoài `read_source` |

### 0.2 Lệch giữa CR và hợp đồng

| # | CR nói | Hợp đồng | Xử lý |
|---|---|---|---|
| L1 | `CODE_INTEL_AI_REVIEW_ENABLED`; `CODEINTEL_INVALID_ARGUMENT` | PQ-23, PQ-03 | `CODEINTEL_AI_REVIEW_ENABLED`; `CODEINTEL_INVALID_PARAMS` |
| L2 | Migration của CR thêm cột `ai_review_*` | PQ-24: có sẵn ở `0002` | Không viết migration |
| L3 | `ai_review.proto`; `repo_binding_id` | PQ-07/04 | `codeintel_ai_review.proto`; `selector` |
| L4 | Timeout "≥ 90 s" | PQ-13/agent §2.5: **120 s** | 120 s |
| L5 | Hạn mức 10 lượt/giờ/người, 1 đồng thời/binding (đề xuất) | Hợp đồng không có số | Giữ làm cấu hình, giá trị khởi điểm chưa hiệu chỉnh; cơ chế qua `AgentCallGate` (BE-CV-SOL-013-agent-call-gate-and-quotas) |
| L6 | `labels{ai_inferred, model, level, generatedAt}`; `cache{hit, createdAt, expiresAt}` | ui-api §3.2 cùng tên | Theo |
| L7 | Gọi `ai.complete` chỉ gửi `prompt` (như git-gateway); `model` chọn qua tham số | Handler agent nhận `model?` (đã đọc qua CR) | Gửi `prompt`, `format:"json"` và `model` (nếu tenant cấu hình; còn rỗng = mặc định agent); không gửi `accountId`/`resolvedApiKey` (Q1 CR) |

### 0.3 Phụ thuộc chéo khu vực

| Hướng | Solution | Dùng gì |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-093-ai-summary-panel` (thẻ "Tóm tắt do AI suy luận", manifest + xác nhận lần đầu, thử lại khi `CODEINTEL_TIMEOUT inProgress`) · `FE-CV-SOL-090-review-report-export` (chèn có nhãn) | `AiReviewSummary`, `manifest`, mã lỗi |
| AG | Không có solution AG (§8.2). Handler `ai.complete` đã có (120 s, `max_tokens` 4096 — theo CR; **không** đổi) | — |
| infra-fleet | Tự sửa trong solution này: `execTimeoutForMethod` (xem 1, 2.1) | — |
| BE | BE-CV-SOL-013 (che bí mật, `AgentCallGate`, quyền, audit, `PathPolicy`), SOL-085-evaluator (cờ, `read_source` OPA đã ở 013), BE-CV-SOL-036 (overlay), 082/037 (phát hiện), 022 (snapshot cache), 023 (vận chuyển), 090 (chèn nhãn) | |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ở phiên này: `infra-fleet-service/internal/adapter/devserveragent/client.go` (:395–435: `execPromptTimeout`=15 phút; `execTimeoutForMethod` chỉ `agent.execPrompt`, các method khác trả 0 ⇒ `callWithTimeout` rơi về mặc định `cfg.RequestTimeout` 30 s; `Exec` gọi `execTimeoutForMethod(method)`), `usecase/relay_by_dev_server.go` (`Exec` lỗi → `INFRA_AGENT_EXEC_FAILED`), `git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (:1085–1100 `Complete` gửi **chỉ** `prompt`), `client_test.go:309` (test nhắc `execTimeoutForMethod`). Theo CR (chưa mở lại): handler agent, `generate_commit_message.go`, `commit_message_prompt.go`.

### Correction relative to CR-CV-093

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | Ngoại lệ ở `client.go:412` | Đúng (`execTimeoutForMethod` tại :412) | Thêm `case "ai.complete"` trả `aiCompleteTimeout = 120 * time.Second` |
| C2 | Chỉ code-intel hưởng lợi | `git-gateway` `GenerateCommitMessage` cũng qua `Exec` với `ai.complete` ⇒ **cũng** đổi 30 s → 120 s | Ghi vào rủi ro; sửa là hành vi mong muốn nhưng phải báo chủ git-gateway; deadline gRPC của caller vẫn cắt trước nếu ngắn hơn |
| C3 | Che bí mật có sẵn | Chưa tồn tại (BE-013 đề xuất) | Port `TextRedactor`, `PathPolicy`; thêm `AiHighEntropyRedactor` riêng solution này |
| C4 | `execTimeoutForMethod` "Go cắt, agent vẫn chạy" | Theo hợp đồng agent §2.5 | Agent hết hạn nội bộ 120 s; Go 120 s ⇒ cần biên: đặt Go = 120 s như hợp đồng nhưng ghi rõ biên hẹp (Q2) |

## 2. Giải pháp chi tiết

### 2.1 Cây file (mới, trừ dòng sửa)

```
backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go       # (sửa) + client_test.go: case "ai.complete"
backend-go/proto/orca/codeintel/v1/codeintel_ai_review.proto                             # AiReviewSummary, AiReviewManifest + req-resp
services/code-intel-service/internal/
  domain/ai_review_input_builder.go    # danh sách cho phép, manifest, giới hạn (thuần)
  domain/ai_review_text_sanitizer.go   # ANSI, zero-width, bidi, độ dài dòng, cụm dẫn dắt
  domain/ai_review_entropy_redactor.go # chuỗi entropy cao
  domain/ai_review_prompt.go           # promptVersion="v1", nonce, thoát "</DATA-"
  domain/ai_review_output_validator.go # JSON chặt, refs thuộc tập đầu vào, bỏ URL/HTML
  usecase/generate_review_summary.go
  usecase/ai_review_ports.go           # OverlayReader, GateReader, FindingReader, DiffHunkReader, AiCompleter, SnapshotCache, TextRedactor, PathPolicy
  adapter/grpcclient/ai_complete_relay.go   # InfraFleet.RelayByDevServer, qua AgentCallGate
  adapter/grpc/ai_review_server.go     # phương thức của QualityGateServer
```

### 2.2 Luồng `GenerateReviewSummary`

Chuỗi: cờ (4 lớp, fail closed) → OPA `quality_read` ∧ `read_source` → `selector → binding` → kiểm `level ≤ cấu hình tenant`, `locale ∈ ^[a-z]{2}(-[A-Za-z]{2,4})?$`, `ai_review_model` ∈ allowlist → dựng đầu vào + manifest → `dry_run`? trả manifest, **không gọi agent** → kiểm cache (**quyền trước cache**) → `AgentCallGate` → `ai.complete` → kiểm đầu ra → ghi cache → audit `codeintel.ai.summary` (kèm `level`, `model`, số tệp, byte, số lần che; không nội dung) → trả. Không có dev server kết nối → `CODEINTEL_AI_NO_RELAY` (không dùng thông điệp của infra-fleet).

### 2.3 Dữ liệu gửi đi và chống injection

Bảng danh sách cho phép, giới hạn (≤ 60 tệp, 80 symbol, 20 lý do, 20 phát hiện; `diff`: ≤ 10 tệp × ≤ 60 dòng, tổng ≤ 16 KB; đầu vào dựng ≤ 24 000 ký tự), **không bao giờ gửi** (đường dẫn tuyệt đối, tên người, email, env, tệp chặn nội dung, tệp sinh/lock/nhị phân, tệp bị ignore, lý do miễn trừ, ghi chú review, prompt agent, `ai_context`): theo CR §2.3. Che hai lớp (`TextRedactor` + entropy ≥ 32 ký tự); tệp ≥ 3 lần che bị loại (`withheld:"redaction_density"`); tổng > 20 lần che → hạ `metadata` + cảnh báo. Tệp untracked chỉ vào mức `diff` khi `git check-ignore` xác nhận không bị ignore (thông qua port `DiffHunkReader`; cách gọi agent cho việc này **chưa chốt**, nếu không có thì loại mọi tệp untracked; Q3). Khung nhắc: `<DATA-{nonce}>`, nonce 16 byte hex, thoát `</DATA-`; làm sạch ký tự điều khiển/bidi/zero-width, dòng ≤ 300; cụm dẫn dắt → `[LINE REMOVED: suspected instruction]` và `suspectedInjection:true` (chỉ giảm nhiễu). **Lớp quyết định là kiểm đầu ra**: JSON chặt (`DisallowUnknownFields`, `summary ≤ 600`, `risks ≤ 5` mỗi ≤ 200, `readFirst ≤ 5` mỗi `why ≤ 120`), `refs`/`file` ∉ tập đầu vào bị loại (đếm `refsDropped`), bỏ URL/Markdown link/HTML/khối mã/ký tự điều khiển; không parse được → thử lại **một lần** rồi `CODEINTEL_AI_BAD_OUTPUT`, không bao giờ trả văn bản thô.

### 2.4 Timeout, nền, cache

`infra-fleet`: `aiCompleteTimeout = 120 s` (PQ-13). Service: lời gọi chạy trong singleflight theo khoá cache với context nền hạn 100 s + 5 s dự phòng (cùng nguyên tắc PQ-13); handler đồng bộ chờ ≤ 24 s rồi trả `CODEINTEL_TIMEOUT` `{"inProgress":true,"retryAfterMs":3000}`; lần gọi sau trúng cache. Khoá cache `(repo_binding_id, view="aiSummary", commit=head, params_hash=sha256(base|profile_ref|model|level|promptVersion|locale|inputDigest))`; `expires_at = now_db + 24h`; `force_refresh` bỏ qua; nội dung cache coi như dữ liệu mã (chỉ người có `read_source` đọc được; quyền kiểm trước đọc cache).

### 2.5 Tách khỏi cổng

`EvaluateGate` (SOL-085-evaluator) **không** import package `ai_review`; test cấu trúc bằng `go/parser` chạy trên `domain/quality_gate_*.go`. `AiReviewSummary` không đi vào commit/PR tự động.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Mặc định tắt, `metadata` trước | O13 |
| D2 | Danh sách cho phép + manifest xem trước | Người dùng thấy cái rời máy |
| D3 | Kiểm đầu ra là lớp chính | Không dựa vào "dặn" mô hình |
| D4 | Hoàn tất nền, trả `CODEINTEL_TIMEOUT inProgress` | WS không vượt 25 s (PQ-13) |
| D5 | Không đổi cách chọn khoá AI | Dùng khoá dev server như `GenerateCommitMessage` (CR Q1 để mở) |
| D6 | Không tool, không tự áp dụng | Giảm bề mặt tấn công |

## 4. Tiêu chí chấp nhận

- [x] `off`/cờ tắt → `CODEINTEL_AI_REVIEW_DISABLED`, **không** có lời gọi `ai.complete` (agent giả đếm).
- [x] `dry_run` không gọi agent và manifest khớp byte/tệp đầu vào thật.
- [x] Đầu vào không chứa các mục cấm (repo fixture); token giả (`ghp_…`, `AKIA…`, JWT, `PRIVATE KEY`, hex 64) bị che; tệp ≥ 3 lần che bị loại.
- [x] Bộ injection (≥ 20 mẫu): đầu ra không có `refs` ngoài tập, URL, HTML; không đổi `verdict`; văn bản tự do → `CODEINTEL_AI_BAD_OUTPUT`.
- [x] Cache: hit không gọi agent; đổi 1 dòng diff → miss; hết hạn 24 h theo đồng hồ DB; thiếu `read_source` không đọc được dù có cache.
- [x] `infra-fleet`: `execTimeoutForMethod("ai.complete")` = 120 s, `agent.execPrompt` = 15 phút, method khác = 0 (test bảng).
- [x] `CODEINTEL_TIMEOUT` có hậu tố `inProgress`; lần gọi sau trúng cache.
- [x] Hạn mức vượt → `CODEINTEL_RATE_LIMITED`; audit không chứa nội dung; tenant A không đọc cache tenant B.

## 5. Kiểm thử

Unit (builder, sanitizer, entropy, prompt, validator, khoá cache); use case với `AiCompleter` giả (JSON hợp lệ/hỏng/độc, chậm > 24 s); infra-fleet test bảng; cấu trúc (không import); fixture injection chung CR-CV-070; chạy mô hình thật bằng tay theo CR §2.9, **không** trong CI.

## 6. Rủi ro và điểm chưa kiểm chứng

- Nhà cung cấp LLM nhận mã ngoài kiểm soát Orca; `model` do agent báo, không kiểm chứng.
- Đổi timeout `ai.complete` ảnh hưởng cả git-gateway (C2).
- Biên 120 s giữa agent và Go quá sát; có thể cần Go > agent (Q2).
- Đường diff của git-gateway chưa kiểm có lọc tệp nhạy cảm (CR; chưa mở lại) — lỗ hổng ngoài phạm vi.
- Mọi ngưỡng (3 lần che, 20, 24 000, 16 KB, hạn mức, chất lượng ≥ 70%) là giá trị khởi điểm chưa hiệu chỉnh.
- Chưa chạy bất kỳ test nào.

## 7. Câu hỏi mở

- **Q1.** Dùng khoá/tài khoản `ai-provider-service` thay khoá trên dev server?
- **Q2.** Có đặt Go 125–130 s để agent luôn hết hạn trước (như nguyên tắc §2.5)?
- **Q3.** Cách xác nhận tệp untracked không bị ignore (method agent nào được phép)?
- **Q4.** Mức `diff` có cờ theo repo ngoài tenant?
- **Q5.** Bổ sung che bí mật cho `GenerateCommitMessage` (CR của git-gateway).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-093-ai-review-summary.md`
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` (§2.5), `CONTRACT-codeintel-ui-api.md` (§3.2)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`, `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
