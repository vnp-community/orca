# Orca — Product Roadmap

**Cập nhật:** 2026-09-08 | **Nguồn:** [`docs/features/`](../features/README.md) (42 features, F01–F42)

Tài liệu này phân loại toàn bộ 42 features theo **mức độ quan trọng** (Priority) và **nhu cầu sử dụng theo thời gian** (khi nào cần có để tạo giá trị cho người dùng / mở khoá feature khác), từ đó xây dựng roadmap phát triển theo pha (phase). Đây là góc nhìn *forward-looking* — bổ sung cho Feature Registry (vốn là góc nhìn *trạng thái hiện tại*).

> Xem thêm [`feature-completion-matrix.md`](./feature-completion-matrix.md) — đối chiếu từng feature với code thật trong `frontend/`, `backend-go/`, `agent/` để biết phần nào đã port sang kiến trúc mới, phần nào còn khoảng trống theo từng lớp.

---

## 1. Phương pháp phân loại

### 1.1 Mức độ quan trọng (Priority — lấy từ từng feature spec)

| Priority | Ý nghĩa | Số lượng |
|----------|---------|----------|
| **P0 — Must Have** | Chặn (blocking): sản phẩm không hoạt động / không bán được nếu thiếu | 16 |
| **P1 — Should Have** | Giá trị cao, cạnh tranh, nhưng có thể trễ vài tuần không gãy sản phẩm | 15 |
| **P2 — Could Have** | Tăng trải nghiệm, không chặn go-live, làm khi còn năng lực | 9 |
| **P3 — Nice to Have** | Experimental / gamification, không có cam kết thời gian | 2 |

### 1.2 Nhu cầu sử dụng theo thời gian (trục thời gian roadmap)

Vì **30/42 features đã phát hành**, phần "roadmap" thực chất chỉ còn ý nghĩa với nhóm **🚧 Đang phát triển** (12) và **📋 Kế hoạch** (1) — tổng 13 features là trọng tâm điều phối nguồn lực từ nay trở đi. Các feature này được xếp vào 5 track theo thứ tự phụ thuộc kỹ thuật (đã xác nhận trong `docs/features/README.md` §"v5.0 Implementation Order" và migration DAG) cộng với đánh giá mức độ khẩn cấp nghiệp vụ.

| Track | Khung thời gian đề xuất | Tiêu chí xếp vào track |
|-------|------------------------|------------------------|
| **Đã hoàn thành** | — (nền tảng hiện có) | Trạng thái ✅, không cần hành động |
| **Track 1 — Foundation** | Q3 2026 (đang chạy) | P0, mọi feature v5.0 khác phụ thuộc trực tiếp |
| **Track 2 — AI Provider** | Q4 2026 | P0, cần Foundation xong; chặn agent execution có provider quản lý tập trung |
| **Track 3 — Workspace** | Q4 2026 → Q1 2027 | P0, cần Foundation + AI Provider; là "mặt tiền" UI hợp nhất mà user thấy hàng ngày |
| **Track 4 — Task & Workflow** | Q1 2027 | P0/P1, cần Workspace làm nơi hiển thị; giá trị cao nhất khi 3 track trước đã ổn định |
| **Track 5 — Continuous (P2 song song)** | Q3 2026 – Q2 2027, chạy song song không chặn v5.0 | P2, cải thiện trải nghiệm, độc lập với chuỗi v5.0 |
| **Backlog** | Chưa cam kết | P3 / experimental |

> Khung thời gian là **đề xuất dựa trên phụ thuộc kỹ thuật**, không phải cam kết ngày cụ thể — cần team lead xác nhận capacity trước khi đưa vào sprint planning.

---

## 2. Roadmap theo Track

### ✅ Đã hoàn thành (nền tảng — 30 features)

Không cần phân bổ thêm resource roadmap; giữ nguyên trong maintenance mode (bugfix + hardening theo yêu cầu vận hành).

| Nhóm | Features |
|------|----------|
| Core Desktop IDE | F01 Parallel Worktrees, F02 Terminal Splits, F03 Mobile Companion, F04 AI Agent Support, F05 Design Mode, F06 GitHub/Linear Integration, F07 SSH Worktrees, F08 Annotate AI Diffs, F09 Orca CLI, F10 Quick Open, F11 Notifications, F12 File Explorer & Editor, F13 Text Search |
| Advanced Desktop | F16 Rich Repo Previews, F19 Localization, F21 Auto Update |
| Web Server & Enterprise | F22 Web Server Mode, F23 Multi-User Auth (Phase 1), F24 Per-User Sandbox, F25 Admin Panel, F26 Multi-Database, F27 Fleet Health Monitoring, F28 Dev Server Onboarding, F29 Agent WebSocket Protocol, F30 Remote Integrations, F31 Fleet Provisioning |
| Observability | F40 Full-Flow Tracing |
| Engagement/Onboarding | F41 Desktop Pet Companion, F42 Contextual Onboarding Tours |

---

### 🔵 Track 1 — Foundation (Q3 2026, đang chạy)

Nền móng bắt buộc: mọi tính năng v5.0 khác (Provider, Workspace, Task, Workflow) đều `relay.call()` dựa trên `devServerId` do F34 xác định và env/agent settings do F33 resolve — không thể song song hoá trước khi 2 feature này ổn định.

| ID | Feature | Priority | Trạng thái | Vì sao cần ngay |
|----|---------|----------|------------|-----------------|
| [F33](../features/F33-user-profile-hierarchy.md) | User Profile Hierarchy | P0 | 🚧 | Mọi agent spawn (F34, F38) cần `resolveProfile()` để lấy model, trust preset, env — chưa xong thì các track sau không có input hợp lệ |
| [F34](../features/F34-project-dev-server-binding.md) | Project-Dev Server Binding | P0 | 🚧 | Xác định "project X chạy trên server Y" — là điều kiện tiên quyết cho routing của F35/F36/F37/F38/F39 |

**Migration:** `0006 company+dept` (F33), `0007 projects` (F34).

---

### 🟢 Track 2 — AI Provider Management (Q4 2026)

| ID | Feature | Priority | Trạng thái | Vì sao cần |
|----|---------|----------|------------|-----------|
| [F35](../features/F35-ai-provider-account-management.md) | AI Provider Account Management | P0 | 🚧 | Không có quản lý tập trung → mỗi agent spawn vẫn phải lấy key từ `process.env` global, xung đột multi-user. Track 3 (Agent Tab trong Workspace) và Track 4 (Workflow multi-provider) đều hiển thị/đọc account do F35 quản lý |

**Migration:** `0008 ai_providers`.

---

### 🟠 Track 3 — Project Workspace (Q4 2026 → Q1 2027)

Đây là "mặt tiền" — UI hợp nhất mà developer thấy hàng ngày sau khi đăng nhập. Giá trị demo/bán hàng cao nhất trong v5.0.

| ID | Feature | Priority | Trạng thái | Vì sao cần |
|----|---------|----------|------------|-----------|
| [F38](../features/F38-project-workspace.md) | Project Workspace — Unified IDE | P0 | 🚧 | Khung UI hợp nhất (Explorer/Agent/Workflow/Task/Git) — các tab Workflow (F36) và Task (F37) chỉ có chỗ hiển thị sau khi F38 tồn tại |
| [F39](../features/F39-remote-git-ui.md) | Remote Git UI | P0 | 🚧 | Đóng vòng lặp "agent sửa code → review diff → commit/PR" mà không cần SSH thủ công; là tab con của F38 |

**Migration:** không có migration riêng (dùng chung `0007` projects); phụ thuộc `relay-git-bridge` mở rộng.

---

### 🟣 Track 4 — Task & Workflow Orchestration (Q1 2027)

| ID | Feature | Priority | Trạng thái | Vì sao cần |
|----|---------|----------|------------|-----------|
| [F37](../features/F37-task-graph-management.md) | Task Graph Management | P0 | 🚧 | Quản lý công việc có cấu trúc + AI decomposition; cần Workspace (F38) làm nơi hiển thị Task tab và cần Project binding (F34) để route "Run Agent" |
| [F36](../features/F36-multi-server-workflow-orchestration.md) | Multi-Server Workflow Orchestration | P1 | 🚧 | Điều phối nhiều bước/nhiều server/nhiều provider; phụ thuộc cả F34 (server resolver), F35 (provider resolver) và hưởng lợi từ F37 (task linkage). Xếp P1 vì automation cơ bản (F14) đã tồn tại như phương án tạm |

**Migration:** `0009 workflows` (F36), `0010 tasks` (F37) — F37 nên đi trước hoặc song song F36 vì F36 tham chiếu `{{outputs.<stepId>.*}}` và task linkage.

---

### 🟡 Track 5 — Continuous P2 (song song, không chặn v5.0)

Nhóm P2 "Could Have" đã bắt đầu (🚧) hoặc đã phát hành nhưng còn phase chưa xong. Không nằm trên đường găng (critical path) của v5.0 — nên phân bổ resource dư ra ngoài 4 track trên, không kéo lùi Foundation/Provider/Workspace/Task.

| ID | Feature | Priority | Trạng thái | Ghi chú |
|----|---------|----------|------------|---------|
| [F32](../features/F32-team-rbac.md) | Team RBAC — Phase 2 (SSO) | P2 | ⚠️ Phase 1 xong, Phase 2 pending | OIDC/SAML — cần khi khách hàng enterprise yêu cầu SSO, không chặn multi-user cơ bản (đã có local login từ F23) |
| [F17](../features/F17-memory-ai-vault.md) | Memory / AI Vault | P2 | 🚧 | Tăng năng suất khi resume session cũ; độc lập kỹ thuật với chuỗi v5.0 |
| [F18](../features/F18-ephemeral-vm.md) | Ephemeral VM | P2 | 🚧 | Cần cho các tác vụ cần môi trường sạch/sandbox không tin cậy; không phải core flow |
| [F14](../features/F14-automations.md) | Automations | P2 | 🚧 | Là tiền thân đơn-server của F36 — nên tiếp tục hoàn thiện làm phương án fallback khi F36 chưa xong, sau đó có thể hợp nhất |
| [F15](../features/F15-computer-use.md) | Computer Use | P2 | 🚧 | Dùng cho ứng dụng không có API; nhu cầu thấp hơn các track chính |

---

### ⚪ Backlog — chưa cam kết thời gian

| ID | Feature | Priority | Trạng thái | Ghi chú |
|----|---------|----------|------------|---------|
| [F20](../features/F20-speech-input.md) | Speech Input | P3 | 🔬 Experimental | Offline STT qua Sherpa-ONNX; giữ ở dạng thử nghiệm, chỉ đầu tư thêm khi có tín hiệu nhu cầu rõ ràng từ người dùng |

---

## 3. Ma trận đầy đủ (42 features)

| ID | Feature | Priority | Trạng thái | Track |
|----|---------|----------|------------|-------|
| F01 | Parallel Worktrees | P0 | ✅ | Đã hoàn thành |
| F02 | Terminal Splits | P0 | ✅ | Đã hoàn thành |
| F03 | Mobile Companion | P0 | ✅ | Đã hoàn thành |
| F04 | AI Agent Support | P0 | ✅ | Đã hoàn thành |
| F05 | Design Mode | P1 | ✅ | Đã hoàn thành |
| F06 | GitHub/Linear Integration | P1 | ✅ | Đã hoàn thành |
| F07 | SSH Worktrees | P1 | ✅ | Đã hoàn thành |
| F08 | Annotate AI Diffs | P1 | ✅ | Đã hoàn thành |
| F09 | Orca CLI | P1 | ✅ | Đã hoàn thành |
| F10 | Quick Open | P1 | ✅ | Đã hoàn thành |
| F11 | Notifications | P1 | ✅ | Đã hoàn thành |
| F12 | File Explorer & Editor | P1 | ✅ | Đã hoàn thành |
| F13 | Text Search | P1 | ✅ | Đã hoàn thành |
| F14 | Automations | P2 | 🚧 | Track 5 — Continuous |
| F15 | Computer Use | P2 | 🚧 | Track 5 — Continuous |
| F16 | Rich Repo Previews | P2 | ✅ | Đã hoàn thành |
| F17 | Memory / AI Vault | P2 | 🚧 | Track 5 — Continuous |
| F18 | Ephemeral VM | P2 | 🚧 | Track 5 — Continuous |
| F19 | Localization | P2 | ✅ | Đã hoàn thành |
| F20 | Speech Input | P3 | 🔬 | Backlog |
| F21 | Auto Update | P0 | ✅ | Đã hoàn thành |
| F22 | Web Server Mode | P0 | ✅ | Đã hoàn thành |
| F23 | Multi-User Auth (Phase 1) | P0 | ✅ | Đã hoàn thành |
| F24 | Per-User Sandbox | P0 | ✅ | Đã hoàn thành |
| F25 | Admin Panel | P1 | ✅ | Đã hoàn thành |
| F26 | Multi-Database | P1 | ✅ | Đã hoàn thành |
| F27 | Fleet Health Monitoring | P1 | ✅ | Đã hoàn thành |
| F28 | Dev Server Onboarding | P1 | ✅ | Đã hoàn thành |
| F29 | Agent WebSocket Protocol | P1 | ✅ | Đã hoàn thành |
| F30 | Remote Integrations | P1 | ✅ | Đã hoàn thành |
| F31 | Fleet Provisioning | P1 | ✅ | Đã hoàn thành |
| F32 | Team RBAC | P2 | ⚠️ Phase 1/2 | Track 5 — Continuous |
| F33 | User Profile Hierarchy | P0 | 🚧 | **Track 1 — Foundation** |
| F34 | Project-Dev Server Binding | P0 | 🚧 | **Track 1 — Foundation** |
| F35 | AI Provider Account Management | P0 | 🚧 | **Track 2 — AI Provider** |
| F36 | Multi-Server Workflow Orchestration | P1 | 🚧 | **Track 4 — Task & Workflow** |
| F37 | Task Graph Management | P0 | 🚧 | **Track 4 — Task & Workflow** |
| F38 | Project Workspace | P0 | 🚧 | **Track 3 — Workspace** |
| F39 | Remote Git UI | P0 | 🚧 | **Track 3 — Workspace** |
| F40 | Full-Flow Tracing | P1 | ✅ | Đã hoàn thành |
| F41 | Desktop Pet Companion | P3 | ✅ | Đã hoàn thành |
| F42 | Contextual Onboarding Tours | P2 | ✅ | Đã hoàn thành |

---

## 4. Trình tự phụ thuộc kỹ thuật (dependency chain)

```
Track 1 (F33 Profile, F34 Project Binding)
        │
        ▼
Track 2 (F35 AI Provider)
        │
        ▼
Track 3 (F38 Workspace shell → F39 Git UI trong Workspace)
        │
        ▼
Track 4 (F37 Task Graph, F36 Workflow — dùng chung Workspace + Provider + Server resolver)

Track 5 (F14, F15, F17, F18, F32-Phase2) — độc lập, chạy song song bất kỳ lúc nào có capacity dư
Backlog (F20) — không lịch trình
```

Vi phạm thứ tự này (vd. bắt đầu F38 trước khi F33/F34 ổn định) sẽ buộc phải hardcode `devServerId`/profile tạm thời rồi refactor lại — nên giữ đúng trình tự Track 1 → 2 → 3 → 4.

---

## 5. Rủi ro

| Rủi ro | Ảnh hưởng | Track liên quan |
|--------|-----------|------------------|
| F33/F34 trễ tiến độ | Chặn toàn bộ chuỗi Track 2–4 (dây chuyền domino) | Track 1 |
| F35 thiếu health-check/quota trước khi Track 3 demo | Agent tab (F38) hiển thị provider nhưng không có fallback khi provider unhealthy | Track 2 → 3 |
| F36 phụ thuộc chéo F37 (task linkage) nhưng cả hai đều 🚧 song song | Có thể cần điều phối interface `{{outputs}}` sớm giữa 2 team con | Track 4 |
| F14 (Automations, đơn-server) và F36 (Workflow, multi-server) trùng lặp một phần chức năng | Cân nhắc hợp nhất/deprecate F14 sau khi F36 ổn định để tránh 2 hệ thống automation song song | Track 5 → Track 4 |

---

## 6. Cách cập nhật tài liệu này

Khi một feature đổi trạng thái (🚧 → ✅) hoặc đổi priority trong `docs/features/*.md`, cập nhật lại:
1. Bảng trạng thái ở §1.2 (số lượng track còn lại)
2. Track tương ứng ở §2 (chuyển feature sang "Đã hoàn thành")
3. Ma trận đầy đủ ở §3
