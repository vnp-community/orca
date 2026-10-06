# Solution Engines: Change Requests (v6, bổ sung)

> Giao diện `SolutionEngine` ở `request-service` với hai cài đặt: `native` (`ai.complete`, CR-REQ-007 và 012) và `openspec` (sinh Solution và Plan bằng OpenSpec trên dev server). Hợp đồng chung ở [README v6](../README.md); nguồn nghiên cứu: [`openspec-and-ai-tooling-integration.md`](../../../research/receive-request/openspec-and-ai-tooling-integration.md).

> CR này **không sửa** CR hiện có hay README v6. Mục 9 của CR liệt kê chính xác phần cần sửa; việc sửa do người duyệt series thực hiện.

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-026](./CR-REQ-026-openspec-solution-engine.md) | `ai.complete` không đọc repo nên Solution và Plan chỉ ở mức văn bản; muốn dùng OpenSpec (tài liệu nằm trong git, agent đọc được repo) mà không đổi máy trạng thái, Approval hay agent | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-007, 008, 012 ──▶ CR-REQ-013 ──┐
CR-REQ-027 (bản chiếu Markdown/YAML) ──┼──▶ CR-REQ-026
CR-REQ-033 (agent, mềm) ───────────────┘
```

| Bước | Lý do thứ tự |
|------|--------------|
| 007, 008, 012 trước | Cung cấp `analysis_runs`, `GenerateSolution`, `GeneratePlan`, `PlanProposal`; CR-REQ-026 chỉ đặt giao diện lên trên và bọc mã đó thành `nativeEngine` |
| 013 trước | `TasksMdSyncer` và `archive` dựa vào `task_run_outcomes` và sự kiện `request.completed` |
| 027 trước | Định nghĩa vùng Orca, khối `orca-json`, `RenderArtifact`, provenance; parser `tasks.md` dựa vào đó |
| 033 (mềm) | Chế độ chỉ đọc và `worktreePath` của agent; CR-REQ-026 không cần vì chủ ý ghi trong worktree riêng, nhưng danh sách file đổi giúp kiểm sau chạy. Chưa đọc được CR-REQ-033 lúc soạn |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| E1 | OpenSpec nằm sau giao diện; máy trạng thái, `Approval`, `solutions`, `CreatePlanTree` không đổi | `hotfix`, `spike`, `question` không dùng; công cụ ngoài đổi phiên bản không được làm vỡ luồng |
| E2 | Không thêm method `openspec.*` vào agent | Dùng `agent.exec`, `agent.execPrompt`, `fs.*` đã có |
| E3 | Cờ cấp project `solution_engine` (mặc định `native`), ghim theo Request | Đổi cờ giữa chừng không làm lệch Request đang chạy |
| E4 | Hồ sơ `full` (`change_request`, `refactor`), `light` (`bug`, `security`, `performance`: chỉ Plan), `none` (còn lại, luôn `native`) | Khuyến nghị nghiên cứu mục 1.2 |
| E5 | Nhánh `request/<number>-proposal` và worktree riêng; không dùng repo gốc | Sinh giải pháp ghi vào repo, không lẫn với nhánh task |
| E6 | Preflight lỗi thì dừng, không tự lùi về `native`; người dùng chọn `engine_override` | Hai engine cho kết quả khác nhau |
| E7 | Orca là nguồn chính; `tasks.md` đồng bộ một chiều, best-effort; `archive` sau `request.completed` | Hai nguồn sự thật |

## Điểm cần xác nhận và chưa kiểm chứng

- **OpenSpec chưa được cài hay chạy thử**; cấu trúc thư mục, `validate`, `archive`, cờ dòng lệnh, định dạng `tasks.md` đều là hiểu biết chung. Chạy thử trên một project mẫu không đổi agent là điều kiện trước khi bật cờ.
- `claude --print` có chạy slash command và hook của repo không; lệnh kiểm `claude` đã đăng nhập không tốn token; việc `CreateWorktree` có phát `worktree.created` làm đồng bộ Jira cho Request nguồn Jira.
- Số liệu timeout (600 giây), cache preflight (10 phút), giới hạn kích thước (256 KB) là đề xuất.
- CR-REQ-033 chưa có trong cây `docs/crs/v6` lúc soạn.
- Hai CR khác cùng thêm cột và giá trị vào `analysis_runs`: CR-REQ-026 (`engine`, `mode=agent_proposal`) và CR-REQ-007/008 (`mode`); nên làm trong cùng migration khi triển khai.
