# Tasks: quality-gate (frontend, v7)

> 🚧 **In Progress.** Rà soát 2026-10-07: SOL-085 4/7 done, SOL-089 4/7 done, SOL-090 3/8 done, SOL-092 1/7 done, SOL-093 1/6 done, SOL-095 1/7 done. Tổng ~10/42 tasks (~25%). Hầu hết model/parser done, UI components và wiring chưa.

Mỗi task: `Status: [x] DONE`, nhỏ, kiểm thử độc lập (Vitest). NN tăng liên tục theo CR.

## CR-CV-085: Cảnh báo cổng chất lượng ở Source Control ([FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-085-01-quality-feature-flags-hook](./FE-CV-TASK-085-01-quality-feature-flags-hook.md) | Hook gộp cờ code-intel/quality/AI | P0 | ✅ Done |
| [FE-CV-TASK-085-02-quality-gate-notice-view-model](./FE-CV-TASK-085-02-quality-gate-notice-view-model.md) | View-model thuần của khối cảnh báo cổng | P0 | ✅ Done |
| [FE-CV-TASK-085-03-use-source-control-quality-gate](./FE-CV-TASK-085-03-use-source-control-quality-gate.md) | Hook `useSourceControlQualityGate` | P0 | ✅ Done |
| [FE-CV-TASK-085-04-quality-gate-notice-component](./FE-CV-TASK-085-04-quality-gate-notice-component.md) | Component `SourceControlQualityGateNotice` | P0 | ✅ Done |
| [FE-CV-TASK-085-05-composer-and-commit-area-notice-slots](./FE-CV-TASK-085-05-composer-and-commit-area-notice-slots.md) | Thêm khe `qualityNotice` vào composer và CommitArea | P0 | ✅ Done |
| [FE-CV-TASK-085-06-source-control-and-checks-panel-wiring](./FE-CV-TASK-085-06-source-control-and-checks-panel-wiring.md) | Nối hook và notice vào SourceControl và ChecksPanel | P1 | 🟡 Partial (`qualityNotice={null}`, hook chưa nối) |
| [FE-CV-TASK-085-07-quality-gate-notice-locale-and-regression-tests](./FE-CV-TASK-085-07-quality-gate-notice-locale-and-regression-tests.md) | Khoá i18n năm locale và test hồi quy | P1 | ❌ TODO |

## CR-CV-089: Ghi lượt agent và đối chiếu "agent tự báo" ([FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-089-01-agent-turn-record-params-builder](./FE-CV-TASK-089-01-agent-turn-record-params-builder.md) | Hàm dựng tham số `quality.turn.record` và digest | P0 | ✅ Done |
| [FE-CV-TASK-089-02-agent-tool-use-command-summarizer](./FE-CV-TASK-089-02-agent-tool-use-command-summarizer.md) | Bộ chuẩn hoá lệnh và thu thập công cụ theo pane | P0 | ✅ Done |
| [FE-CV-TASK-089-03-agent-turn-record-queue](./FE-CV-TASK-089-03-agent-turn-record-queue.md) | Hàng đợi gửi, thử lại, khử trùng lặp | P1 | ✅ Done |
| [FE-CV-TASK-089-04-use-agent-turn-backend-recorder](./FE-CV-TASK-089-04-use-agent-turn-backend-recorder.md) | Hook ghi lượt lên backend | P0 | ❌ TODO (SOL-060/061 chưa sẵn) |
| [FE-CV-TASK-089-05-agent-turn-verification-view-model](./FE-CV-TASK-089-05-agent-turn-verification-view-model.md) | View-model đối chiếu lượt agent | P1 | ✅ Done |
| [FE-CV-TASK-089-06-agent-turn-claims-line-component](./FE-CV-TASK-089-06-agent-turn-claims-line-component.md) | Component `AgentTurnVerificationLine` | P2 | ❌ TODO |
| [FE-CV-TASK-089-07-agent-turn-privacy-guard-and-locale-tests](./FE-CV-TASK-089-07-agent-turn-privacy-guard-and-locale-tests.md) | Test riêng tư và khoá i18n | P1 | 🟡 Partial (privacy guard ✅, locale test ❌) |

## CR-CV-090: Xuất báo cáo review, chèn vào mô tả PR/MR ([FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-090-01-review-report-model-parser](./FE-CV-TASK-090-01-review-report-model-parser.md) | Parser mô hình báo cáo | P0 | ✅ Done |
| [FE-CV-TASK-090-02-review-report-markdown-builder](./FE-CV-TASK-090-02-review-report-markdown-builder.md) | Bộ dựng Markdown cho mô tả PR/MR | P0 | ✅ Done |
| [FE-CV-TASK-090-03-review-report-diagram-guard](./FE-CV-TASK-090-03-review-report-diagram-guard.md) | Guard cho chuỗi Mermaid của backend | P0 | ✅ Done |
| [FE-CV-TASK-090-04-review-report-html-builder](./FE-CV-TASK-090-04-review-report-html-builder.md) | Bộ dựng HTML độc lập và token màu | P1 | ❌ TODO |
| [FE-CV-TASK-090-05-use-review-report-and-export-actions](./FE-CV-TASK-090-05-use-review-report-and-export-actions.md) | Hook `useReviewReport` và hành động xuất | P0 | ❌ TODO |
| [FE-CV-TASK-090-06-review-report-menu](./FE-CV-TASK-090-06-review-report-menu.md) | Menu "Xuất báo cáo" | P1 | ❌ TODO |
| [FE-CV-TASK-090-07-hosted-review-body-merge-and-insert-button](./FE-CV-TASK-090-07-hosted-review-body-merge-and-insert-button.md) | Chèn báo cáo vào mô tả PR/MR | P0 | ❌ TODO |
| [FE-CV-TASK-090-08-review-report-locale-and-xss-tests](./FE-CV-TASK-090-08-review-report-locale-and-xss-tests.md) | Khoá i18n và test XSS | P1 | ❌ TODO |

## CR-CV-092: Lens Yêu cầu (truy vết) ([FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-092-01-requirement-trace-view-model](./FE-CV-TASK-092-01-requirement-trace-view-model.md) | View-model và quy tắc chữ của truy vết | P0 | ✅ Done |
| [FE-CV-TASK-092-02-use-requirement-trace](./FE-CV-TASK-092-02-use-requirement-trace.md) | Hook `useRequirementTrace` (đọc) | P0 | ❌ TODO |
| [FE-CV-TASK-092-03-requirement-evidence-actions](./FE-CV-TASK-092-03-requirement-evidence-actions.md) | Hành động xác nhận/bỏ gợi ý và liên kết task | P1 | ❌ TODO |
| [FE-CV-TASK-092-04-worktree-task-link-picker](./FE-CV-TASK-092-04-worktree-task-link-picker.md) | Picker task bằng Command + Popover | P1 | ❌ TODO |
| [FE-CV-TASK-092-05-requirement-trace-panel](./FE-CV-TASK-092-05-requirement-trace-panel.md) | Panel, hàng yêu cầu, danh sách bằng chứng và thay đổi chưa gắn | P0 | ❌ TODO |
| [FE-CV-TASK-092-06-requirements-lens-registration](./FE-CV-TASK-092-06-requirements-lens-registration.md) | Đăng ký lens `requirements` | P1 | ❌ TODO |
| [FE-CV-TASK-092-07-requirement-trace-locale-and-wording-tests](./FE-CV-TASK-092-07-requirement-trace-locale-and-wording-tests.md) | Khoá i18n và test cấm từ | P1 | ❌ TODO |

## CR-CV-093: Thẻ tóm tắt AI (mặc định tắt) ([FE-CV-SOL-093-ai-summary-panel](../solutions/FE-CV-SOL-093-ai-summary-panel.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-093-01-ai-summary-wire-parser-and-text-guard](./FE-CV-TASK-093-01-ai-summary-wire-parser-and-text-guard.md) | Parser phản hồi và bảo vệ văn bản | P0 | ✅ Done |
| [FE-CV-TASK-093-02-use-review-ai-summary](./FE-CV-TASK-093-02-use-review-ai-summary.md) | Hook `useReviewAiSummary` | P0 | ❌ TODO |
| [FE-CV-TASK-093-03-ai-summary-data-preview-dialog](./FE-CV-TASK-093-03-ai-summary-data-preview-dialog.md) | Dialog xem trước dữ liệu gửi và xác nhận | P0 | ❌ TODO |
| [FE-CV-TASK-093-04-review-ai-summary-card](./FE-CV-TASK-093-04-review-ai-summary-card.md) | Thẻ tóm tắt AI | P1 | ❌ TODO |
| [FE-CV-TASK-093-05-ai-summary-report-section](./FE-CV-TASK-093-05-ai-summary-report-section.md) | Mục Markdown có nhãn cho báo cáo (090) | P2 | ❌ TODO |
| [FE-CV-TASK-093-06-ai-summary-locale-and-injection-render-tests](./FE-CV-TASK-093-06-ai-summary-locale-and-injection-render-tests.md) | Khoá i18n và test hiển thị injection | P1 | ❌ TODO |

## CR-CV-095: Telemetry Review/cổng (enum/khoảng) ([FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md))

| Task | Tiêu đề | Ưu tiên | Status |
|---|---|---|---|
| [FE-CV-TASK-095-01-review-telemetry-event-schemas](./FE-CV-TASK-095-01-review-telemetry-event-schemas.md) | Schema tám sự kiện telemetry | P0 | ✅ Done |
| [FE-CV-TASK-095-02-telemetry-shared-copies-sync-and-parity-test](./FE-CV-TASK-095-02-telemetry-shared-copies-sync-and-parity-test.md) | Đồng bộ sáu bản sao `shared/` và test parity | P0 | ❌ TODO |
| [FE-CV-TASK-095-03-review-telemetry-wrappers-and-buckets](./FE-CV-TASK-095-03-review-telemetry-wrappers-and-buckets.md) | Hàm bọc và hàm chia khoảng | P0 | ❌ TODO |
| [FE-CV-TASK-095-04-review-decision-tracker](./FE-CV-TASK-095-04-review-decision-tracker.md) | Bộ theo dõi "agent xong → quyết định" | P1 | ❌ TODO |
| [FE-CV-TASK-095-05-decision-tracker-wiring](./FE-CV-TASK-095-05-decision-tracker-wiring.md) | Nối tracker vào commit, tạo review, gửi ghi chú, đánh dấu đã xem | P1 | ❌ TODO |
| [FE-CV-TASK-095-06-review-surface-event-call-sites](./FE-CV-TASK-095-06-review-surface-event-call-sites.md) | Nối các sự kiện còn lại ở bề mặt Review | P2 | ❌ TODO |
| [FE-CV-TASK-095-07-review-telemetry-privacy-tests](./FE-CV-TASK-095-07-review-telemetry-privacy-tests.md) | Test quyền riêng tư và consent | P1 | ❌ TODO |

## Sơ đồ phụ thuộc giữa các task

```
085-01 (cờ) ──▶ 085-03 ──▶ 085-04 ──▶ 085-05 ──▶ 085-06 ; 085-02 ─▶ 085-03 ; 085-07
   │
   ├──▶ 089-01 ─▶ 089-02 ─▶ 089-04 ; 089-03 ─▶ 089-04 ; 089-05 ─▶ 089-06 ; 089-07
   ├──▶ 090-01 ─▶ 090-02 ─▶ 090-05 ─▶ 090-06, 090-07 ; 090-03 ─▶ 090-02, 090-04 ─▶ 090-05 ; 090-08
   ├──▶ 092-01 ─▶ 092-02 ─▶ 092-03 ─▶ 092-04 ─▶ 092-05 ─▶ 092-06 ; 092-07
   ├──▶ 093-01 ─▶ 093-02 ─▶ 093-03 ─▶ 093-04 ; 093-05 (cần 090-02) ; 093-06
   └──▶ 095-01 ─▶ 095-02, 095-03 ─▶ 095-04 ─▶ 095-05 ; 095-06 (sau các bề mặt) ; 095-07
```

Có thể làm ngay không cần backend: 085-01/02, 089-01/02/03/05, 090-01/02/03/04, 092-01, 093-01, 095-01/02/03/04 (hàm thuần).
