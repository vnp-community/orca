# TASK-TDD-002-T2-10: Cài đặt engine theo project, cổng khả năng (preflight) và cài skill

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.2, 3.4 (bước 1)
**Checklist:** [T2.10](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service, api-gateway, orca-cli, agent
**Files:** `internal/usecase/{manage_project_engine_settings,engine_readiness_gate}.go`, `proto/orca/request/v1` (Get/SetProjectEngineSettings), gateway REST `GET/PUT /v1/projects/{id}/request-engine`, `orca-cli` (`orca engine show|set`), `agent/src/shared/agent-feature-install-commands.ts`, test đi kèm
**Depends on:** T2.1, T2.3, T2.9
**Status:** `[ ]` TODO

---

## Context

`ManageProjectEngineSettings.SetProjectEngineSettings` hiện chỉ có chú thích "auth checks etc." rồi `Upsert`, luôn trả cảnh báo `preflight_not_run`; không kiểm quyền, không kiểm engine hợp lệ, không chạy preflight. `EngineReadinessGate` chỉ kiểm `openspec --version` (và, vì `DevServerExecutor` là stub, luôn đạt). Cần cổng thật để chỉ bật chế độ mới cho project có đủ điều kiện, và hiển thị lý do khi thiếu.

## Việc cần làm

1. `SetProjectEngineSettings`: chỉ admin (`roleAdmin` của tenant); kiểm `engine ∈ {native, openspec}`, `MinVersion` đúng dạng; `expectedVersion` ⇒ `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT`; chạy `Preflight` khi bật `openspec` hoặc `solution_agent` và trả kết quả (không chặn việc lưu nhưng cảnh báo rõ); ghi audit.
2. Mở rộng `EngineReadinessGate.Check`: kết nối dev server; `openspec --version` thật và so `MinVersion`; feature của agent (`agent.execPrompt.knowledgeHub`, `agent.execPrompt.workspaceKind`, `agent.execPrompt.resultBlock`, `agent.execPrompt.readonly`); `tdd-agent` có trong PATH; skill `tdd-knowledge`, `tdd-openspec-author`, `tdd-knowledge-writeback`, `tdd-orca-task` đã cài (từ báo cáo khả năng T2.1); `<gốc repo>/.tdd-agent/config.toml` có mặt (kiểm sự hiện diện qua `StatFile`, không truy vấn KG); git bẩn chỉ cảnh báo.
3. Cache kết quả 10 phút theo `(dev_server_id, repo_path)` (đồng hồ có thể giả trong test), thất bại không cache; mã lỗi `REQUEST_ENGINE_*`/`REQUEST_OPENSPEC_*` kèm `fix_hint`.
4. Gateway và CLI: `orca engine show --project <id>` và `orca engine set --project <id> --engine openspec --solution-agent=true`; hiển thị kết quả preflight.
5. `agent-feature-install-commands.ts`: thêm tên skill, lệnh cài `npx skills add … --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global` và lệnh cập nhật; ghi hash phiên bản skill vào bản ghi run khi chạy (đọc từ báo cáo khả năng).
6. Tài liệu: hướng dẫn cài `tdd-agent`, `openspec`, skill trên dev server.

## Tiêu chí hoàn thành

- [ ] Không admin không đổi được cài đặt; xung đột phiên bản bị từ chối.
- [ ] Preflight báo thiếu từng thứ (openspec, tdd-agent, skill, `.tdd-agent`, feature) với `fix_hint`; đủ thì cho phép.
- [ ] Khi thiếu khả năng, run vẫn dùng chế độ cũ và hiển thị lý do, không lỗi.

## Kiểm thử

- Test bảng cho cổng (mỗi thiếu sót một ca), test RPC/REST/CLI, test cache.

## Rủi ro và ghi chú

- Mặc định tắt cả hai cờ; bật cho pilot ở T4.3.
