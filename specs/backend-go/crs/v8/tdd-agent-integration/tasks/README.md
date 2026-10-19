# Tasks của SOL-TDD-002 (tdd-agent knowledge hub)

Mỗi file là một task của [CHECKLIST-TDD-002](CHECKLIST-TDD-002-knowledge-hub.md), theo quy ước của Orca: bối cảnh đã kiểm chứng, file chạm tới, việc cần làm, tiêu chí hoàn thành, kiểm thử, rủi ro. Mã trong checklist (T2.4…) là khoá nối; tên file dùng dạng `T2-04`. Ba task đã xong (T0.1, T0.2, T0.4) không có file riêng.

Thứ tự giai đoạn: 0 chuẩn bị, 1 kiểm chứng (không sửa Orca, cổng go/no-go T1.9), 2 Orca (engine, agent, kiểm cấu trúc), 3 hoàn tất và ghi ngược, 4 e2e trên CR thật, 5 củng cố.

| Mã | Việc | File |
|---|---|---|
| T0.3 | Đưa worktree pilot về trạng thái sạch sau spike | [TASK-TDD-002-T0-03-clean-pilot-worktree.md](TASK-TDD-002-T0-03-clean-pilot-worktree.md) |
| T0.5 | Chốt phiên bản Orca dùng cho pilot: commit CLI, route gateway, skill và tài liệu | [TASK-TDD-002-T0-05-commit-orca-work.md](TASK-TDD-002-T0-05-commit-orca-work.md) |
| T1.1 | Đo recall và nhiễu của việc tra KG do agent làm | [TASK-TDD-002-T1-01-measure-kg-recall.md](TASK-TDD-002-T1-01-measure-kg-recall.md) |
| T1.2 | So Solution có KG (agent) với `ai.complete` hiện tại | [TASK-TDD-002-T1-02-solution-with-kg-vs-complete.md](TASK-TDD-002-T1-02-solution-with-kg-vs-complete.md) |
| T1.3 | Thử bộ skill trên agent thật (tuân thủ chỉ dẫn) | [TASK-TDD-002-T1-03-skill-compliance-on-real-agent.md](TASK-TDD-002-T1-03-skill-compliance-on-real-agent.md) |
| T1.4 | Chế độ chỉ đọc có gọi được tool MCP không | [TASK-TDD-002-T1-04-readonly-mode-mcp-feasibility.md](TASK-TDD-002-T1-04-readonly-mode-mcp-feasibility.md) |
| T1.5 | Spike: AI tự viết bộ OpenSpec cho BL-H01 bằng KG | [TASK-TDD-002-T1-05-ai-authored-openspec-spike.md](TASK-TDD-002-T1-05-ai-authored-openspec-spike.md) |
| T1.6 | Đo nợ duyệt của ghi ngược trên bản sao KG | [TASK-TDD-002-T1-06-measure-writeback-review-debt.md](TASK-TDD-002-T1-06-measure-writeback-review-debt.md) |
| T1.7 | Quyết định kênh commit cho ghi ngược KG | [TASK-TDD-002-T1-07-decide-writeback-commit-channel.md](TASK-TDD-002-T1-07-decide-writeback-commit-channel.md) |
| T1.8 | Chốt nơi lưu và cách agent task đọc gói OpenSpec | [TASK-TDD-002-T1-08-proposal-bundle-location-and-reading.md](TASK-TDD-002-T1-08-proposal-bundle-location-and-reading.md) |
| T1.9 | Cổng go/no-go sau Giai đoạn 1 | [TASK-TDD-002-T1-09-go-no-go-gate.md](TASK-TDD-002-T1-09-go-no-go-gate.md) |
| T2.1 | agent.execPrompt: tham số `knowledgeRoot` (MCP chỉ đọc) và khai báo khả năng | [TASK-TDD-002-T2-01-agent-knowledge-hub-param.md](TASK-TDD-002-T2-01-agent-knowledge-hub-param.md) |
| T2.2 | Chế độ `agent_proposal`, nối EngineRegistry vào luồng, PinSolutionEngine thật | [TASK-TDD-002-T2-02-analysis-mode-agent-proposal-and-engine-registry.md](TASK-TDD-002-T2-02-analysis-mode-agent-proposal-and-engine-registry.md) |
| T2.3 | ProposalWorkspace và DevServerExecutor thật (thay stub) | [TASK-TDD-002-T2-03-real-workspace-and-executor-adapters.md](TASK-TDD-002-T2-03-real-workspace-and-executor-adapters.md) |
| T2.4 | Engine `openspec`: sinh gói OpenSpec bằng agent và dựng đề xuất Plan | [TASK-TDD-002-T2-04-openspec-engine-generate-plan.md](TASK-TDD-002-T2-04-openspec-engine-generate-plan.md) |
| T2.5 | Kiểm cấu trúc gói OpenSpec sau run và thử lại | [TASK-TDD-002-T2-05-bundle-structure-checks.md](TASK-TDD-002-T2-05-bundle-structure-checks.md) |
| T2.6 | Dựng Plan từ `tasks.md` và lưu gói OpenSpec | [TASK-TDD-002-T2-06-build-plan-and-persist-bundle.md](TASK-TDD-002-T2-06-build-plan-and-persist-bundle.md) |
| T2.7 | Gate Plan và CLI hiển thị gói OpenSpec, kiểm cấu trúc, tham chiếu KG | [TASK-TDD-002-T2-07-gate-and-cli-bundle-display.md](TASK-TDD-002-T2-07-gate-and-cli-bundle-display.md) |
| T2.8 | Thời gian chạy dài: giới hạn riêng cho engine `openspec`, lease, trần của agent, timeout của UI và CLI | [TASK-TDD-002-T2-08-long-run-timeouts.md](TASK-TDD-002-T2-08-long-run-timeouts.md) |
| T2.9 | Solution bằng agent chỉ đọc có KG, kèm `design_references` | [TASK-TDD-002-T2-09-solution-by-readonly-agent-with-kg.md](TASK-TDD-002-T2-09-solution-by-readonly-agent-with-kg.md) |
| T2.10 | Cài đặt engine theo project, cổng khả năng (preflight) và cài skill | [TASK-TDD-002-T2-10-engine-settings-preflight-skill-install.md](TASK-TDD-002-T2-10-engine-settings-preflight-skill-install.md) |
| T2.11 | Thực thi task: `knowledgeRoot`, prompt có tiêu chí và `Verify`, kết quả có `Knowledge`/`Verification` | [TASK-TDD-002-T2-11-execution-prompt-and-result-contract.md](TASK-TDD-002-T2-11-execution-prompt-and-result-contract.md) |
| T2.12 | Dọn worktree và nhánh proposal | [TASK-TDD-002-T2-12-cleanup-proposal-worktrees-and-branches.md](TASK-TDD-002-T2-12-cleanup-proposal-worktrees-and-branches.md) |
| T2.13 | Hồi quy: cờ tắt thì mọi thứ như cũ | [TASK-TDD-002-T2-13-regression-with-flags-off.md](TASK-TDD-002-T2-13-regression-with-flags-off.md) |
| T2.14 | Quan sát: ghi số lần gọi tool, snapshot KG, hash skill, thời gian, chi phí | [TASK-TDD-002-T2-14-observability-tool-calls-and-metrics.md](TASK-TDD-002-T2-14-observability-tool-calls-and-metrics.md) |
| T2.15 | Tài liệu và ghi chú thay đổi hợp đồng | [TASK-TDD-002-T2-15-documentation-and-contract-notes.md](TASK-TDD-002-T2-15-documentation-and-contract-notes.md) |
| T3.1 | tdd-agent `kg apply`: áp một đề xuất ChangeSet đã lưu (nếu chọn phương án a) | [TASK-TDD-002-T3-01-kg-apply-command-in-tdd-agent.md](TASK-TDD-002-T3-01-kg-apply-command-in-tdd-agent.md) |
| T3.2 | Lưu và hiển thị đề xuất ghi ngược trong kết quả task | [TASK-TDD-002-T3-02-store-and-show-writeback-proposals.md](TASK-TDD-002-T3-02-store-and-show-writeback-proposals.md) |
| T3.3 | TasksMdSyncer và archive thật (CR-REQ-026 TASK-07) | [TASK-TDD-002-T3-03-tasks-md-syncer-and-archive.md](TASK-TDD-002-T3-03-tasks-md-syncer-and-archive.md) |
| T3.4 | Bàn giao khi Request hoàn tất: liệt kê việc còn lại cho người | [TASK-TDD-002-T3-04-completion-handover-list.md](TASK-TDD-002-T3-04-completion-handover-list.md) |
| T3.5 | Báo cáo nợ duyệt của KG | [TASK-TDD-002-T3-05-review-debt-report.md](TASK-TDD-002-T3-05-review-debt-report.md) |
| T4.1 | An toàn cho pilot: chặn push, dấu vân tay, ảnh chụp dữ liệu, watchdog | [TASK-TDD-002-T4-01-pilot-safety-harness.md](TASK-TDD-002-T4-01-pilot-safety-harness.md) |
| T4.2 | Bước 1 và 2: vào worktree TMS, dựng và kiểm KG | [TASK-TDD-002-T4-02-steps-1-2-worktree-and-kg.md](TASK-TDD-002-T4-02-steps-1-2-worktree-and-kg.md) |
| T4.3 | Đăng ký vnp-qa-platform thành project Orca và bật cờ | [TASK-TDD-002-T4-03-register-project-and-enable-flags.md](TASK-TDD-002-T4-03-register-project-and-enable-flags.md) |
| T4.4 | Bước 3: chạy luồng Orca cho BL-H01 từ Request đến completed | [TASK-TDD-002-T4-04-step-3-bl-h01-end-to-end.md](TASK-TDD-002-T4-04-step-3-bl-h01-end-to-end.md) |
| T4.5 | Kiểm mã đã sinh và phạm vi thay đổi | [TASK-TDD-002-T4-05-verify-code-and-scope.md](TASK-TDD-002-T4-05-verify-code-and-scope.md) |
| T4.6 | Chạy BL-H02 sau khi BL-H01 hoàn tất | [TASK-TDD-002-T4-06-bl-h02-end-to-end.md](TASK-TDD-002-T4-06-bl-h02-end-to-end.md) |
| T4.7 | Ghi số liệu pilot và đối chứng | [TASK-TDD-002-T4-07-collect-metrics-and-compare.md](TASK-TDD-002-T4-07-collect-metrics-and-compare.md) |
| T4.8 | Dọn sau pilot | [TASK-TDD-002-T4-08-cleanup-after-pilot.md](TASK-TDD-002-T4-08-cleanup-after-pilot.md) |
| T5.1 | Test e2e tự động cho luồng mới trên repo tạm | [TASK-TDD-002-T5-01-automated-e2e-for-new-flow.md](TASK-TDD-002-T5-01-automated-e2e-for-new-flow.md) |
| T5.2 | Đánh giá bảo mật engine `openspec` và agent có MCP | [TASK-TDD-002-T5-02-security-assessment.md](TASK-TDD-002-T5-02-security-assessment.md) |
| T5.3 | Quyết định mở rộng | [TASK-TDD-002-T5-03-expansion-decisions.md](TASK-TDD-002-T5-03-expansion-decisions.md) |
| T5.4 | Tổng kết pilot và chuyển nợ thành task | [TASK-TDD-002-T5-04-retrospective-and-debt.md](TASK-TDD-002-T5-04-retrospective-and-debt.md) |
