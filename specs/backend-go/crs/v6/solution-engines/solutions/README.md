# Solutions backend: solution-engines (v6)

> 📋 Proposed, chưa triển khai. Phạm vi backend của CR-REQ-026 trong `request-service` (service mới, chưa có mã lúc soạn). Hợp đồng chung: [README v6](../../../../../../docs/crs/v6/README.md), mục 8 thắng mục 3.

## Bảng CR, Solution, Task

| CR | Solution | Task | Ghi chú |
|----|----------|------|---------|
| [CR-REQ-026](../../../../../../docs/crs/v6/solution-engines/CR-REQ-026-openspec-solution-engine.md) | [BE-REQ-SOL-026](./BE-REQ-SOL-026-openspec-solution-engine.md) | TASK-REQ-026-01 đến 08 | Giao diện `SolutionEngine`, `nativeEngine`, `openspecEngine`, cờ project, nhánh proposal, `tasks.md`, archive |

Frontend và agent: N/A trong thư mục này. Hiển thị engine và nhãn "`tasks.md` chưa đồng bộ" do solution frontend; agent không đổi (dùng `agent.exec`, `agent.execPrompt` có sẵn).

## Thứ tự phụ thuộc

```
SOL-007, 008 ─┐
SOL-012       ├─▶ SOL-026 ◀── SOL-027 (RenderArtifact, khối orca-json, kiểm ngữ nghĩa)
SOL-013       ┘        ▲
                       └── CR-REQ-033 (agent, mềm)
```

SOL-026 chỉ bắt đầu phần domain, migration, parser (task 01 đến 03) khi SOL-007 chưa xong; task 05 trở đi cần `GenerateSolution` và `GeneratePlan` thật.

## Quyết định chung

| # | Quyết định | Lý do |
|---|-----------|-------|
| E1 | Giao diện `SolutionEngine` bao cả phân tích lẫn Plan; phần sau kết quả (validate, Approval, chuyển trạng thái) không đổi | Máy trạng thái và `solutions` giữ nguyên |
| E2 | Không thêm method `openspec.*` vào agent | Dùng `agent.exec`, `agent.execPrompt` |
| E3 | `ProposalWorkspace` dựng trên git-gateway (`worktree_id`) | Đường dẫn tương đối trong worktree, không dùng repo gốc |
| E4 | Engine ghim theo Request (`requests.solution_engine`) | Cờ đổi giữa chừng không làm lệch |
| E5 | Preflight lỗi thì dừng, không tự lùi `native` | Hai engine cho kết quả khác nhau |
| E6 | Đồng bộ `tasks.md` một chiều, best-effort, không bỏ tick | Orca là nguồn chính |

## Số migration

`request-service/migrations` chưa tồn tại lúc soạn. Task 026-02 bắt buộc đọc thư mục thật và lấy số kế tiếp (ghi `NNNN`); nếu migration `analysis_runs` của SOL-007 chưa merge thì gộp hai cột và giá trị CHECK vào đó.

## Mâu thuẫn và điểm chưa kiểm chứng

- `FlowDefinition` phẳng (SOL-003) với `FlowFor(...).Analysis.MinOptions` (SOL-007): mâu thuẫn có sẵn, xem SOL-026 mục 1 C5.
- `CreateWorktreeRequest` cần `repo_id`; `ResolveConnection` không trả: nguồn `repo_id` chưa rõ (Q4 của SOL-026).
- OpenSpec chưa cài và chưa chạy thử; mọi cờ CLI chưa kiểm chứng. CR-REQ-033 chưa có trong cây `docs/crs/v6` lúc soạn.
