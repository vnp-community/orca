# Quality Gate — Change Requests (v7)

> Từ tín hiệu chất lượng (kiểm tra, phát hiện, coverage, CI) đến **kết luận có lý do** và các tính năng quanh nó: cổng chất lượng, dấu vết agent, báo cáo xuất được, truy vết yêu cầu, tóm tắt AI, đo hiệu quả. Bối cảnh, quyết định D1–D7, mặc định O1–O14 và hợp đồng chung ở [README v7](../README.md) (mục 3.10 là hợp đồng kiểm soát chất lượng; mục 8 là điều chỉnh, **khi README và CR khác nhau thì theo CR**). Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-CV-085](./CR-CV-085-quality-gate.md) | Chưa có định nghĩa "đạt" lưu được, thuật toán kết luận `pass\|warn\|fail\|unknown` có lý do, miễn trừ có hạn, xu hướng theo lượt, cảnh báo trước commit/Create PR | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-089](./CR-CV-089-agent-provenance-and-claim-reconciliation.md) | Backend chưa có "lượt agent" bền; chưa đối chiếu được lời agent tự báo với kết quả chạy lại độc lập | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-090](./CR-CV-090-exportable-review-report.md) | Chưa xuất được kết quả review ra Markdown/HTML an toàn, tái lập, dùng cho mô tả PR/MR GitHub và GitLab | 🟠 P1 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-092](./CR-CV-092-requirement-traceability.md) | Chưa nối thay đổi với Task/Request/tiêu chí và chưa thấy tiêu chí nào thiếu bằng chứng | ⚪ P2 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-CV-093](./CR-CV-093-ai-review-summary.md) | Chưa có tóm tắt AI có kiểm soát dữ liệu gửi đi, chống prompt injection, không dùng làm căn cứ cổng (mặc định tắt) | ⚪ P2 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-CV-095](./CR-CV-095-review-quality-telemetry.md) | Chưa đo Review/cổng có hữu ích hay gây nhiễu; chưa có ngưỡng số để nâng cổng lên chặn | ⚪ P2 | Small | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
(đã có trong series: CR-011 ─▶ CR-013, CR-036, CR-037, CR-059, CR-060, CR-061, CR-073)
(nhóm quality-signals: CR-080, 081 ─▶ CR-082 ─▶ CR-083/084/086)

CR-082 ─▶ CR-085 (cổng; cần 011, 013, 036, 037) ─┬▶ CR-089 (lượt agent, đối chiếu)
                                                  ├▶ CR-090 (báo cáo xuất được)
                                                  ├▶ CR-095 (telemetry; cần 061, 059)
                                                  ├▶ CR-092 (truy vết yêu cầu)
                                                  └▶ CR-093 (tóm tắt AI; sau 085 ổn định; có thể chèn vào 090)
CR-085 ─▶ CR-087 (scorecard, nhóm quality-visualization) dùng dữ liệu của cả sáu CR này
```

CR-CV-085 trước hết: định nghĩa bảng `quality_*`, quyền `quality_*`, cờ `quality_gate_enabled` và chuỗi `turn_key` mà 089/090/095 dùng lại. 089 và 090 độc lập nhau. 095 làm sau cùng của nhóm P1 (chỉ cần 085 và các điểm vào của 061). 092 và 093 chỉ làm khi 085 đã ổn định; 092 chạy độc lập v6 (chỉ Task hiện có), nhánh Request bật sau. Theo bảng đợt README v7 mục 5: đợt 8 (085) rồi đợt 9 (089, 090, 092, 093, 095).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Chế độ **chỉ báo** ở mọi điểm tích hợp; `mode=block` chưa lưu được ở MVP | O9; nâng cấp bằng tiêu chí số của CR-CV-095 mục 2.8 và CR riêng |
| F2 | `verdict: unknown` là trạng thái thật, ưu tiên cao hơn `warn` (`fail > unknown > warn > pass`); không bao giờ suy diễn thành `pass` | README 3.10; research 11 §7 |
| F3 | Kết luận tính **khi đọc** từ dữ liệu có sẵn (hàm thuần); chỉ điểm xu hướng được lưu | Tránh kết luận lưu cũ; dễ kiểm thử |
| F4 | `finding_dismissals` (phân loại của người review, không hết hạn) **tách** khỏi `quality_waivers` (ngoại lệ của cổng, có hạn, có người, có lý do) | Gộp sẽ làm "bỏ qua" vô thời hạn lặng lẽ biến lỗi chặn thành đạt (CR-CV-085 2.4) |
| F5 | `turn_key` là chuỗi mờ; chuẩn là `turnId = "${paneKey}:${doneAt}"` của CR-CV-060; `agent_turns.client_turn_id` dùng cùng giá trị | Các bảng nối được mà không phụ thuộc thứ tự triển khai |
| F6 | Backend trả **mô hình có cấu trúc**, chữ hiển thị do frontend dựng từ khoá i18n | README v7 mục 6 (mọi chuỗi qua `translate()`); không catalog chuỗi thứ hai ở Go |
| F7 | Lời agent tự báo và tóm tắt AI **không bao giờ** đi vào cổng | O13; tránh biến dữ liệu suy luận thành căn cứ chặn |
| F8 | Không lưu prompt/transcript/lệnh nguyên văn; chỉ digest, tóm tắt lệnh chuẩn hoá, trích đoạn ngắn đã che | Đồng nhất CR-CV-060 ("không lưu prompt lên backend"); CR-CV-013 2.5 |
| F9 | Telemetry chỉ ở client, chỉ enum/khoảng; số liệu theo tenant lấy từ bảng của tenant | Mẫu `mcp-telemetry-events.ts`; backend không có kênh PostHog |
| F10 | Chữ hiển thị không overclaim: "Chưa thấy bằng chứng", "Suy luận", "Các kiểm tra bắt buộc đã chạy đều đạt"; không "đã đáp ứng", "an toàn để merge", "AI đã review" | STYLEGUIDE; CR-CV-087 |
| F11 | Mọi RPC/kênh dưới hai cờ `code_intel_enabled` ∧ `quality_gate_enabled` (fail closed); AI thêm `ai_review_level ≠ off` | O8, O9, O13 |

## Phạm vi ngoài feature này

- Bộ chạy kiểm tra, profile chạy có tên, tiến độ, huỷ (CR-CV-081); mô hình `QualityFinding`/`QualityRun` và parser (CR-CV-082); coverage (083); rule pack (084); gộp CI GitHub/GitLab (086); quét bảo mật/phụ thuộc (091); đánh giá tool GitNexus chưa dùng (094); index bắt kịp worktree agent (080).
- Giao diện scorecard, chú thích trên diff, biểu đồ xu hướng, nền đồ hoạ (CR-CV-087, 088). Các CR ở đây chỉ cấp dữ liệu và quy tắc chữ.
- Chặn cứng commit/Create PR/push (chưa có CR; O9).
- Truy vết yêu cầu theo v6 đầy đủ (CR-REQ-027/029): CR-CV-092 chỉ có adapter bật sau.
- Sửa v6/v7 README, `finding_dismissals` (CR-CV-011/037/059) và bộ sinh commit message của `git-gateway-service`: ghi ở "Điều chỉnh hợp đồng", không sửa ở đây.

## Việc phải kiểm chứng trước khi viết mã (tổng hợp từ các CR)

1. **Spike hook của agent (CR-CV-089):** ghi lại thật các envelope `agent.hook` của Claude và Codex ở cả đường renderer-launched và backend-spawned; xác nhận Part A (`direct-websocket`) có phát `agent.hook` không (chưa tìm thấy bộ phát), có `state=done`, `toolName/toolInput` theo từng lần dùng tool, độ dài `lastAssistantMessage`.
2. **Thống nhất với CR-CV-082:** `QualityRun` cần phân biệt "công cụ chạy hỏng" với "tìm thấy lỗi" (`errorCode`), `source` (`local|ci`) và nên có `treeFingerprint` (CR-CV-085 Q4).
3. **Cơ chế đồng bộ các bản `shared/` của telemetry (CR-CV-095):** 4 vị trí chứa `telemetry-events.ts`; chưa tìm thấy script đồng bộ.
4. **Giới hạn độ dài mô tả PR/MR và việc render Mermaid** ở GitHub, GitLab (và Azure DevOps, Gitea) (CR-CV-090): chưa kiểm chứng; đọc tài liệu hiện hành.
5. **Đo ngưỡng mặc định `orca-default`** trên 10–20 worktree thật (CR-CV-085 mục 5) và tỉ lệ rút đúng tiêu chí từ mô tả task (CR-CV-092 mục 5) trước khi coi là hữu ích.
6. **Đánh giá chất lượng tóm tắt AI và bộ test injection** (CR-CV-093 2.9) trước khi bật cho tenant thật.
7. **`useTemplate` và nút chèn mô tả PR** tương tác thế nào (CR-CV-090 mục 6), và `Server.GetTask` kiểm grant ở đâu (CR-CV-092 1.1).
8. **Timeout `ai.complete`** qua `Exec` (mặc định 30 s) và ngoại lệ ở `execTimeoutForMethod` (CR-CV-093 2.5).

## Điểm lệch giữa README v7 / nghiên cứu / CR khác và code, phát hiện khi viết feature này (2026-10-06)

**README v7 mục 3.10 và mục 3.5**

- `QualityGate.reasons[].result` chỉ có `pass|warn|fail`, nhưng quy tắc "thiếu dữ liệu → `unknown`" cần `unknown` ở từng lý do; CR-CV-085 thêm `unknown` và các trường tuỳ chọn `code`, `params`, `runId`, `waivedCount`; `basedOn` thêm `headCommit`, `baseCommit`, `evaluatedAt`, `profileVersion`.
- `QualityRun` thiếu `errorCode` (phân biệt hỏng công cụ với phát hiện lỗi) và `treeFingerprint`; `QualityGate` do CR-CV-082 khai báo theo README, CR-CV-085 định nghĩa hành vi (phối hợp chủ sở hữu message).
- Thiếu RPC/kênh: `RecordAgentTurn`, `ListAgentTurns`, `GetAgentTurn` (kênh `codeIntel.quality.turn.record|turns|turn`) cho `agent_turns`; `ConfirmRequirementEvidence`, `LinkWorktreeTask` cho truy vết; push `codeIntel.quality.gateChanged`. `ExportReviewReport` trả **mô hình** (`ReviewReportModel`), không phải văn bản đã dịch.
- Thiếu bảng: `requirement_trace_links`. Cột thêm vào `tenant_settings`: `quality_gate_enabled`, `ai_review_level`, `ai_review_model`, `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled`. `quality_profiles` khoá theo `scope_key` (`tenant` | `repo:<id>`); `quality_waivers` có `scope_key`, `active_key` UNIQUE, `expires_at` bắt buộc; `quality_trend_points` khoá theo `repo_binding_id` + `turn_key`.
- Thiếu sự kiện `orca.codeintel.agent_turn.recorded`; README 3.10 chỉ có `run_finished`, `gate_changed`.
- Thiếu hành động OPA `quality_read`, `quality_waive`, `quality_profile_write` (và dùng lại `read_source`, `review_write`) và audit `codeintel.quality.profile.save`, `codeintel.quality.waive[.revoke]`, `codeintel.report.export`, `codeintel.ai.summary`, `codeintel.trace.confirm`.
- Mã lỗi mới: `CODEINTEL_QUALITY_GATE_DISABLED`, `CODEINTEL_PROFILE_INVALID`, `CODEINTEL_WAIVER_EXPIRY_INVALID`, `CODEINTEL_AI_REVIEW_DISABLED`, `CODEINTEL_AI_NO_RELAY`, `CODEINTEL_AI_BAD_OUTPUT`.
- Research 11 C1 đề xuất "mở rộng `finding_dismissals`" cho miễn trừ; CR-CV-085 chọn **tách** (F4).

**Giữa các CR cùng series**

- Khoá `finding_dismissals`: README 3.5, CR-CV-037 2.5 và CR-CV-059 ghi `repo_binding_id`; CR-CV-011 và README mục 8 điểm 10 ghi `repo_id`. CR-CV-085 theo CR-CV-011. Bảng còn thiếu `disposition` (CR-CV-059 mục 6).
- Tên mã lỗi khi cờ tắt: CR-CV-013 dùng `CODEINTEL_FEATURE_DISABLED`, CR-CV-073 dùng `CODEINTEL_DISABLED`.
- CR-CV-060 quyết định **không** lưu prompt người dùng lên backend; research 11 C5 muốn ghi prompt: CR-CV-089 chỉ lưu `prompt_digest`, trích đoạn là tuỳ chọn tenant mặc định tắt.

**So với code**

- Research 11 C2 và nhiệm vụ nhắc `pr-create-dialog`: `components/code-review/pr-create-dialog.tsx` là code chết (README v7 mục 1); điểm chèn thật là `CreateHostedReviewComposer` + `SourceControl.tsx` (`handleCommit` `:1792`, `handleCreatePullRequest` `:3008`, `runCreatePrIntent` `:3483`, composer `:5247`) và `CommitArea`.
- Go chỉ giải mã `worktreeId`, `ptyId`, `providerSession` của `agent.hook` (`devserveragent/session.go:539`); `state/prompt/tool/lastAssistantMessage` bị bỏ. Bộ phát `agent.hook` chỉ thấy ở `relay.ts:554` (Part B); chưa thấy ở đường `direct-websocket`. `AgentStatusPayload` không có mã thoát của lệnh; lịch sử trạng thái cố ý bỏ `toolName/toolInput/lastAssistantMessage`.
- `ai.complete`: Go chỉ gửi `prompt` (`relay_executor.go`), agent dùng model/khoá mặc định của dev server; `max_tokens` 4096 cứng; không system prompt cho `text`; timeout mặc định 30 s; đường `GenerateCommitMessage` hiện **không** che bí mật (chưa thấy).
- `task-service`: không có RPC tra ngược `(provider, ref)` → task (chỉ `GetTaskSource(task_id)`), `Task` không có tiêu chí chấp nhận có cấu trúc; `#TG-N` được thiết kế cho commit nhưng chưa thấy ai phân tích; `Server.GetTask` chưa thấy kiểm grant.
- Telemetry: web build là no-op; `telemetry-events.ts` có ≥ 4 bản sao trong repo.
- `AGENTS.md` trỏ `docs/STYLEGUIDE.md` và `docs/reference/git-compatibility.md` không tồn tại; đường dẫn thật là `guides/...` (README v7 mục 8 điểm 19).

**Điểm chưa ai chốt (cần quyết khi duyệt)**: khi nào `mode=block`; có xác nhận một lần cho chuỗi "commit → push → tạo PR" khi cổng `fail`; `member` có được miễn trừ `error`; tập lý do bỏ qua phát hiện; dùng khoá/tài khoản AI từ `ai-provider-service` hay khoá trên dev server; nguồn tiêu chí từ Jira.
