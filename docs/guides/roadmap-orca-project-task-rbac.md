# Roadmap: hợp nhất OrcaProject, RBAC, F38 Workspace, và Task/Automation/Orchestration

**Cập nhật:** 2026-08-13

> Kế hoạch thực thi cho toàn bộ đề xuất trong 4 guide:
> [terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md),
> [user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md),
> [project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md),
> [task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md).
> Chỉ lập kế hoạch — chưa code gì trong tài liệu này.

## Vì sao không làm cả 4 cùng lúc

4 guide gộp lại là ~4 hệ thống lớn (RBAC/Team/Department, `OrcaProject` sharing, F38 Workspace
UI, `OrcaTask` + orchestration) — mỗi cái đụng vào DB migration, backend service, RPC, và UI
trên 1 server đang có user thật. Có 2 điểm **cần quyết định của người, không phải kỹ thuật**,
chặn tiến độ nếu bỏ qua:

- **F38 Bước 0**: có release F38 hay không? (đã nêu ở guide F38, chưa có câu trả lời)
- **`OrcaProject` cross-user read**: thiết kế xuyên ranh giới cô lập bảo mật giữa user — cần
  review kỹ hơn 1 câu xác nhận nhanh, không tự triển khai ngay cả khi kỹ thuật khả thi.

## Sơ đồ phụ thuộc giữa 4 sáng kiến

```
[Sáng kiến A] RBAC nền tảng (Team/Department/hợp nhất OrcaProfile)
       │
       │  bắt buộc phải xong trước — cả 2 sáng kiến dưới đều dùng lại RBAC này
       ▼
[Sáng kiến B] OrcaProject sharing layer  ──┐
       │                                    │  cả 2 đều cần OrcaProject.visibility
       ▼                                    │  4 tầng (private/team/department/company)
[Sáng kiến D] OrcaTask + Orchestration ◄────┘  và ProjectMember đã ổn định
       (không phụ thuộc gì thêm ngoài B)

[Sáng kiến C] F38 Workspace UI
       — độc lập, không phụ thuộc A/B/D, nhưng bị chặn bởi quyết định Bước 0 (con người)
       — có thể làm SONG SONG với A/B/D nếu Bước 0 được trả lời sớm
```

**Thứ tự khuyến nghị: A → B → D, C chạy song song bất kỳ lúc nào sau khi có câu trả lời Bước 0.**

## Sáng kiến A — RBAC nền tảng

| Giai đoạn | Việc | Rủi ro | Có thể làm ngay? |
|---|---|---|---|
| A1 | Hợp nhất `main/profile/OrcaProfile.ts` (TDD-14) và `shared/profile-types.ts` (TDD-FE-11) về 1 nguồn — chọn 1 bản làm chuẩn, migrate chỗ dùng bản còn lại | **Thấp** — thuần dọn dẹp type, không đổi hành vi | ✅ Có — quy mô giống các fix đã làm hôm nay |
| A2 | Thêm entity `Team`/`TeamMember` (SQL mới, migration mới) | Trung bình — SQL migration mới trên DB đang chạy | Cần review migration trước khi apply lên server thật |
| A3 | Thêm `departmentId` vào `OrcaUser`, dùng `setUserDepartment()` đã có sẵn | Thấp | ✅ Có, sau A2 |
| A4 | Quyết định rule merge multi-team cho profile cascade (ai thắng khi 1 user thuộc nhiều team) | — (quyết định thiết kế) | Cần chốt trước khi code A2 dùng field `priority` |

**Điều kiện hoàn thành A**: `Team`/`TeamMember`/`departmentId` tồn tại và có dữ liệu thật (kể cả
rỗng), 1 nguồn `OrcaProfile` duy nhất, cascade Company→Department→Team→User chạy được (test
tối thiểu: 1 user 2 team, xác nhận đúng field nào thắng theo rule đã chốt ở A4).

## Sáng kiến B — `OrcaProject` sharing layer

**Điều kiện tiên quyết**: Sáng kiến A xong (cần `Team`/`departmentId` cho 4-tier visibility).

| Giai đoạn | Việc | Rủi ro |
|---|---|---|
| B1 | Thiết kế chi tiết luồng đọc-chéo-user (đã phác thảo ở guide RBAC mục "Luồng đọc-chéo-user") — **cần review bảo mật trước khi code**, không tự triển khai từ kế hoạch chat | Cao nếu bỏ qua review |
| B2 | Bảng `OrcaProjectSourceProject` (join `OrcaProject` ↔ `Project` per-user) | Trung bình — migration mới |
| B3 | API `orcaProjects.list()`/`getProjectData()` với kiểm tra quyền qua `ProjectSharing`/`OrcaProjectSourceProject` trước khi đọc file JSON user khác | Cao — đây là điểm duy nhất xuyên ranh giới cô lập user, phải test kỹ voi trường hợp quyền bị thu hồi giữa chừng |
| B4 | Mở rộng `OrcaProject.visibility` thêm `'department'` (4 tầng) | Thấp, sau A xong |

**Điều kiện hoàn thành B**: user B (member của 1 `OrcaProject`) đọc được đúng phần dữ liệu đã
share của user A, và **không** đọc được phần chưa share — có test case xác nhận rõ ràng cho cả
2 chiều (thấy đúng phần được phép, không thấy phần không được phép).

## Sáng kiến C — F38 Workspace UI

**Điều kiện tiên quyết**: câu trả lời cho Bước 0 (release hay không). Không phụ thuộc A/B/D.

| Giai đoạn | Việc | Ghi chú |
|---|---|---|
| C0 | **Quyết định của người dùng**: release F38 hay dán nhãn/dọn cluster đã chết? | Chặn toàn bộ phần dưới |
| C1 | Viết lại F38 doc theo code thật (layout, tên file, shape `WorkspaceContext`) | Chỉ nếu C0 = release |
| C2 | Quyết định nguồn `currentWorktree` (tái dùng sidebar hiện có hay xây bộ chọn riêng) | Khoá cho C3/C4 |
| C3 | Nối tab Agent (`AgentPanel.tsx` đã có sẵn, chỉ thiếu `worktreeId`) | Việc cụ thể, giới hạn rõ |
| C4 | Nối terminal panel — tái dùng `terminal-pane`/PTY infra hiện có | Không xây hệ terminal thứ 2 |
| C5 | Ghép `ServerStatusBar` từ `RuntimeHostStatusRow`/`SshStatusSegment` có sẵn | Tái dùng, không viết mới |
| C6 | Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật — **làm cuối cùng** | Chỉ sau C1–C5 xong, tránh lộ tab dở cho user thật |

## Sáng kiến D — `OrcaTask` + Orchestration integration

**Điều kiện tiên quyết**: Sáng kiến B xong (cần `OrcaProject`/`ProjectMember` ổn định để
`OrcaTask` thừa hưởng RBAC, tránh tự xây `TaskGrant` song song).

| Giai đoạn | Việc | Rủi ro |
|---|---|---|
| D1 | Xây `OrcaTask` tối giản (`parentId`/`dependsOn`/`projectId`/`promptTemplate`/`sourceContext`, **không** có `TaskGrant`) — chỉ data model + service CRUD | Thấp — dữ liệu mới, chưa nối gì |
| D2 | Nối ①: `OrcaTask.sourceContext` — liên kết optional với `TaskSourceContext` (GitHub/Linear/Jira) | Thấp |
| D3 | Nối ②③: "Run Agent" seed phiên orchestration từ `OrcaTask`, ghi ngược kết quả khi phiên xong | Trung bình — động vào `main/runtime/orchestration/coordinator.ts` đang chạy thật, cần test kỹ không phá luồng orchestration-skill hiện có |
| D4 | Mở rộng `Automation` thêm `taskId` optional | Thấp, additive |
| D5 | Xây UI (Tree/Board/Graph view F37) | Chỉ sau D1–D4 chạy được qua RPC/script, tránh lặp lỗi "tab bấm vào trống" như F38 |

**Điều kiện hoàn thành D**: tạo 1 `OrcaTask`, bấm "Run Agent" (qua script/RPC), xác nhận phiên
orchestration được seed đúng nội dung, xác nhận `OrcaTask.status` tự cập nhật khi phiên xong —
trước khi động vào D3 (`coordinator.ts`), chạy lại toàn bộ test hiện có của orchestration để
đảm bảo không phá luồng multi-agent đang chạy thật.

## Việc có thể bắt đầu ngay hôm nay (an toàn, không cần quyết định thêm)

- **A1** — hợp nhất 2 bản `OrcaProfile`/`profile-types.ts`.
- **C0** — chỉ cần 1 câu trả lời từ bạn, không phải code.

## Việc cần quyết định/review trước khi code (không tự triển khai từ kế hoạch này)

- **A4** — rule merge multi-team.
- **B1** — thiết kế bảo mật cho đọc-chéo-user (nên có 1 vòng review riêng, không lẫn vào 1 lượt
  code thông thường).
- **C0** — release F38 hay không.
