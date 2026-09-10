# CR-EVM-010 — Auto-destroy/suspend VM sau khi task hoàn thành

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-EVM-010 |
| **Tên** | Nối sự kiện "agent run hoàn thành" tới suspend/destroy VM tự động |
| **Loại** | Feature Completion |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Proposed — cần khảo sát event source trước khi ước lượng effort |
| **Tác giả** | Audit trực tiếp mã nguồn sau khi xác nhận CR-EVM-001..005 đã Done |
| **Tác động HLD** | Infra-Fleet domain, Worktree/Runtime lifecycle |
| **Tác động Features** | F18 (Ephemeral VM) — tiêu chí chấp nhận "VM bị destroy tự động sau khi task xong" |

---

## Bối cảnh & Vấn đề gốc

F18's tiêu chí chấp nhận
([`docs/features/F18-ephemeral-vm.md:55`](../../../features/F18-ephemeral-vm.md)):
"VM bị destroy tự động sau khi task xong" — chưa đạt. Toàn bộ đường
destroy/cleanup hiện tại đều **thủ công hoặc gián tiếp**:

- **Thủ công**: nút "Cleanup"/"Retry cleanup" trong Settings → Runtime
  Environments (`EphemeralVmRuntimesSection.tsx:88-132`).
- **Gián tiếp qua xoá workspace/repo**: `worktrees.ts:3423`,
  `repos.ts:3492` gọi `cleanupEphemeralVmRuntimesForDeleted`
  (`ephemeral-vm-runtime-cleanup.ts:21-56`) — chỉ chạy khi user tự xoá
  workspace/repo, không phải khi "task xong".
- **"Sleep workspace"** (`sleep-worktree-flow.ts:150-163`) gọi
  `suspendRuntimeEphemeralVmWorkspace` — nhưng đây là suspend thủ công
  khi user đóng tab, không phải destroy tự động khi agent hoàn thành
  việc.

**Không có hook nào** từ "agent task/run hoàn thành" tới suspend/destroy
VM.

## Giải pháp đề xuất

### Bước 1 — Khảo sát: "agent run hoàn thành" đã có event source nào chưa?

Trước khi thiết kế hook, xác định: hệ thống đã phát sự kiện "agent run
kết thúc" ở đâu cho mục đích khác chưa (thông báo UI, task-graph status,
notification) — nếu có, tái dùng nguồn đó. **Đây cùng câu hỏi
[CR-AUTO-005](../../v4/automations/CR-AUTO-005-real-event-triggers.md)
(nhóm Automations) đặt ra cho "khi agent kết thúc" event trigger** — nếu
2 CR làm gần nhau về thời gian, nên dùng chung 1 nguồn phát sự kiện,
tránh 2 implementation event-source khác nhau cho cùng 1 khái niệm.

### Bước 2 — Chính sách auto-destroy vs auto-suspend

Cần quyết định (không tự chọn mặc định mà không hỏi): auto-**destroy**
(mất toàn bộ state VM, tiết kiệm resource tối đa) hay auto-**suspend**
(giữ state, resume nhanh hơn nếu user quay lại workspace) khi task xong?
Khuyến nghị: mặc định **suspend** (an toàn hơn, đã có RPC
`suspendRuntimeEphemeralVmWorkspace` sẵn sàng), cho phép user cấu hình
sang destroy nếu muốn tiết kiệm resource tối đa — nhất quán với cách
"Sleep workspace" hiện đã dùng suspend làm hành vi mặc định khi đóng tab.

### Bước 3 — Wiring

Trong handler xử lý sự kiện "agent run hoàn thành" (từ bước 1), với
workspace backed bởi ephemeral VM (`experimentalEphemeralVms` bật +
workspace có `hostId` kiểu ephemeral runtime), gọi
`suspendRuntimeEphemeralVmWorkspace` (hoặc `cleanupRuntimeEphemeralVmWorkspace`
theo chính sách bước 2) — tái dùng nguyên xi 2 hàm này, không viết logic
suspend/destroy mới.

### Cấu hình per-workspace (tuỳ chọn, nếu cần)

Cho phép user tắt auto-suspend/destroy cho 1 workspace cụ thể (vd. đang
debug, muốn giữ VM sống để inspect) — checkbox trong runtime settings
panel, mặc định bật.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Chưa xác định event source có sẵn hay chưa | Cao | Effort thật phụ thuộc hoàn toàn bước 1 — nếu chưa có, CR phình to hơn (phải thêm cả event source) |
| Trùng lặp effort với CR-AUTO-005 nếu làm độc lập, không phối hợp | Trung bình | Nên track chung tiến độ khảo sát bước 1 giữa 2 CR |
| Auto-destroy nhầm khi user còn đang cần VM (task "xong" nhưng user muốn tiếp tục làm việc thủ công trong VM) | Trung bình | Mặc định suspend (không mất state), + cấu hình tắt per-workspace giảm rủi ro này |
| Sự kiện "agent hoàn thành" bắn nhiều lần hoặc race với action khác trên cùng workspace | Thấp | Cần idempotency ở lệnh suspend/destroy (gọi 2 lần không lỗi) — kiểm tra hành vi hiện tại của 2 hàm trước khi wire |

## Không thuộc phạm vi CR này

- Định nghĩa lại "task xong" ở tầng automation/task-graph nói chung —
  CR này chỉ tiêu thụ sự kiện đã có/được thêm ở bước 1, không thiết kế
  lại khái niệm "task" toàn hệ thống.

## Liên quan

- `frontend/src/renderer/src/components/settings/EphemeralVmRuntimesSection.tsx:88-132`
- `frontend/src/renderer/src/lib/ephemeral-vm-runtime-cleanup.ts:21-56`
- `frontend/src/renderer/src/lib/sleep-worktree-flow.ts:150-163`
- `frontend/src/renderer/src/store/slices/worktrees.ts:3423`, `repos.ts:3492`
- [CR-AUTO-005](../../v4/automations/CR-AUTO-005-real-event-triggers.md) (cùng câu hỏi event-source, nhóm Automations)
- [CR-EVM-009](./CR-EVM-009-worktree-mount-copy-out-reconciliation.md) (định nghĩa "VM xong" liên quan tới copy-out nếu Nhánh B được chọn)
