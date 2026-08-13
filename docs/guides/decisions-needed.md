# Quyết định cần con người chốt — không tự triển khai

**Cập nhật:** 2026-08-13

> Danh sách các điểm audit ra có **nhiều phương án hợp lý**, hoặc đụng vào ranh giới bảo mật/dữ
> liệu thật — cố ý không tự chọn 1 phương án và triển khai. Mỗi mục có bối cảnh, các phương án,
> và khuyến nghị (nếu có) — nhưng quyết định cuối thuộc về người, không phải AI.
>
> Nguồn: [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md). Giải pháp cho
> các bug không cần quyết định xem [fix-proposals-per-issue.md](./fix-proposals-per-issue.md).
> Kế hoạch thực thi tổng hợp xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## 1. F38 Workspace: hoàn thiện (release) hay dọn dẹp (shelve)?

**Bối cảnh**: cụm ~32 file (`components/workspace/` 18 file + ~14 component dùng `useWorkspace()`
ở nơi khác) đã tồn tại nhiều tháng, chất lượng code không tệ, backend đứng sau (`ProjectService`,
`WorkspaceService`) đã thật và chạy — nhưng chưa từng mount vào `App.tsx`.

**Phương án**:
- (a) Hoàn thiện: sửa các bug RPC-contract, nối tab Agent/terminal, mount vào layout thật.
- (b) Dọn dẹp: dán nhãn rõ "không dùng, chờ xoá" hoặc xoá hẳn cụm 32 file, tránh tích luỹ thêm
  nợ kỹ thuật và tránh nguy cơ tái diễn bug "code chết đè lên code thật" (đã xảy ra 1 lần với
  `workspace-slice.ts` trong phiên làm việc này).

**Không khuyến nghị nghiêng về phương án nào** — đây là quyết định sản phẩm (có cần 1 Workspace
UI hợp nhất khác với sidebar hiện tại không?), không phải kỹ thuật.

## 2. `WorkspaceContextV6` — có kế hoạch thay V5 không?

**Bối cảnh**: `WorkspaceContextV6.tsx` + `WorkspaceContextBridge.ts` tồn tại song song với
`WorkspaceContext.tsx` (V5), có cơ chế bridge qua flag `__ORCA_WORKSPACE_V6__` — nhưng
`main.tsx` bỏ qua bridge, luôn dùng V5. Không tìm thấy bằng chứng nào (comment, doc, commit
message) giải thích ý định của V6.

**Câu hỏi cần trả lời**: V6 là dở dang (đang làm nửa chừng, sẽ hoàn thiện) hay đã bỏ (nên xoá)?
Nếu không ai nhớ/biết, khuyến nghị coi là đã bỏ và xoá — giữ code chết không rõ mục đích rủi ro
hơn lợi ích.

## 3. Team/Department profile cascade — rule merge khi 1 user thuộc nhiều Team

**Bối cảnh**: Company→Department→Team→User, 1 user chỉ thuộc 1 Department nhưng có thể thuộc
nhiều Team. Khi 2 Team cấu hình khác nhau cho cùng field, ai thắng?

**Phương án**:
- (a) `priority: number` trên `TeamMember`, số cao thắng — đơn giản, nhưng cần ai đó set đúng
  priority, dễ quên khi thêm team mới.
- (b) Team join sau cùng thắng (theo `addedAt`) — không cần config thêm, nhưng thứ tự thêm
  không phản ánh đúng "team nào quan trọng hơn".
- (c) Không cho override — nếu 2 team xung đột, giữ giá trị company/dept (an toàn nhất, nhưng
  giảm giá trị của Team profile).

**Khuyến nghị nhẹ**: (a), vì tường minh và audit được (`_sources` ghi rõ `team:<teamId>` nào
thắng) — nhưng cần người xác nhận đây đúng là hành vi mong muốn trước khi code.

## 4. `TaskGrantService` vs `ProjectMember` — có nên hợp nhất 2 hệ RBAC không?

**Bối cảnh**: `TaskGrantService` (task-level, scope `user/team/role/everyone`, kế thừa theo
cây) và `ProjectMember` (project-level, role `owner/member/viewer`) là **2 hệ độc lập, đã chạy
thật, có dữ liệu thật**. Không chia sẻ code.

**Phương án**:
- (a) Giữ nguyên 2 hệ tách biệt — Task và Project là 2 phạm vi khác nhau, có lý do chính đáng để
  RBAC riêng (task có thể share hẹp hơn cả project chứa nó).
- (b) Hợp nhất — `OrcaTask.projectId` thừa hưởng quyền từ `ProjectMember`, `TaskGrantService`
  chỉ override khi cần hẹp hơn. Rủi ro: đây là thay đổi trên dữ liệu/logic đang chạy thật, cần
  kế hoạch migrate rõ ràng, không nên làm vội.

**Không khuyến nghị nghiêng phương án nào** — cần người hiểu rõ ý định sản phẩm ban đầu khi tách
2 hệ này (có thể là chủ ý, không phải sơ suất).

## 5. `OrcaTask.execute` chạy đơn hay chạy qua Orchestration coordinator?

**Bối cảnh**: hiện có **2 con đường "chạy 1 task" độc lập, không giao nhau**:
- (a) `task.execute` (RPC thật, `TaskAgentExecutor`) → gọi thẳng `agentSpawner.spawn()` — 1
  agent chạy đơn, không có lead/worker.
- (b) `orchestration.run` (RPC thật, `coordinator.ts`) → điều phối nhiều agent lead/worker qua
  `TaskRow` riêng của orchestration.

**Câu hỏi cần trả lời**: khi hoàn thiện pipeline Source→Plan→Execute
([task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2), `OrcaTask` nên luôn đi qua (a), hay có thể chọn (b) khi task cần phân rã cho nhiều
worker (task có nhiều `subTaskIds`/`dependsOn` phức tạp)? Nếu chọn (b) cho task phức tạp, cần
thiết kế cách `TaskAgentExecutor` seed `orchestration.run` thay vì tự spawn — thay đổi kiến trúc
thật, không nhỏ.

## 6. Field `integrations`/`fleet`/`security.require2FA` trong Profile UI — thêm vào backend hay bỏ khỏi frontend?

**Bối cảnh**: `frontend/src/renderer/src/types/profile-types.ts` (UI thật đang dùng) khai các
field này, nhưng backend hoàn toàn không có — ghi vào sẽ **lưu được nhưng không bao giờ đọc lại
được** (`ProfileResolver.merge()` không duyệt qua các section này).

**Phương án**:
- (a) Thêm các field/section này vào backend `OrcaProfile` thật — nếu tính năng (tích hợp
  GitHub/Linear mặc định, 2FA bắt buộc, giới hạn theo fleet tag) thật sự cần.
- (b) Bỏ khỏi type frontend — nếu đây chỉ là field dự phòng chưa ai dùng thật.

**Không khuyến nghị** — phụ thuộc roadmap sản phẩm có cần các tính năng này không, chưa rõ từ
audit.

## 7. `OrcaProject` cross-user sharing — thiết kế bảo mật cần review riêng

**Bối cảnh**: đề xuất luồng đọc-chéo-user ở
[terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
đụng vào ranh giới cô lập bảo mật giữa các user (mỗi user hiện có tiến trình/file riêng biệt
hoàn toàn). Đây **không phải case chọn 1-trong-N phương án đơn giản** — cần 1 vòng review bảo
mật riêng, có thể cần threat-model trước khi viết bất kỳ dòng code nào cho phần đọc-chéo-user.

**Không tự triển khai dù kỹ thuật khả thi** — xem
[roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md) sáng kiến B.

## 8. Nguồn `currentWorktree` cho Workspace (nếu quyết định 1 = "hoàn thiện F38")

**Bối cảnh**: `WorkspaceContext`'s `currentWorktree` chưa bao giờ được set — cần nguồn cấp
`worktreeId` thật để tab Agent/terminal panel hoạt động.

**Phương án**:
- (a) Tái dùng cơ chế chọn worktree đã có ở sidebar chính (`WorktreeList.tsx`) — nhất quán với
  UX hiện tại, nhưng cần thiết kế cách 2 UI (sidebar + Workspace) đồng bộ lựa chọn.
- (b) Workspace tự có bộ chọn worktree riêng — độc lập hơn, nhưng tạo 2 nơi chọn worktree khác
  nhau trong cùng app, dễ gây nhầm lẫn UX.

**Khuyến nghị nhẹ**: (a), nhất quán UX — nhưng chỉ liên quan nếu mục 1 chọn "hoàn thiện F38".
