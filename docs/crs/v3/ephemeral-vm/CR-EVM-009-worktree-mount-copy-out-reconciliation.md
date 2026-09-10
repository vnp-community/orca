# CR-EVM-009 — Đối chiếu spec vs implementation: mount/copy-out worktree↔VM

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-EVM-009 |
| **Tên** | Quyết định sản phẩm: mô hình "trỏ vào folder VM" hiện tại là v1 chủ đích, hay cần mount/copy-out thật như spec mô tả |
| **Loại** | Quyết định sản phẩm (Architecture Decision), có thể trở thành spec-correction HOẶC feature lớn |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔲 Proposed — cần quyết định sản phẩm trước khi ước lượng effort thật |
| **Tác giả** | Audit trực tiếp mã nguồn sau khi xác nhận CR-EVM-001..005 đã Done |
| **Tác động HLD** | Infra-Fleet domain, Worktree creation flow |
| **Tác động Features** | F18 (Ephemeral VM) — "Integration với Worktrees" |

---

## Bối cảnh & Vấn đề gốc

F18's spec
([`docs/features/F18-ephemeral-vm.md:44-47`](../../../features/F18-ephemeral-vm.md)):

> **Integration với Worktrees**
> - Ephemeral VM worktree: worktree được mount trong VM
> - Kết quả được copy ra sau khi VM xong
> - Port forwarding từ VM về local

Mô hình đã ship (audit 2026-09-09) là **khác về bản chất**, không phải
"làm dở":

- `frontend/src/renderer/src/lib/ephemeral-vm-worktree-creation.ts` +
  `ephemeral-vm-workspace-target.ts:42-142` — provision VM, sau đó gọi
  `setupExistingFolder` trỏ **workspace mới** vào folder recipe's project
  root đã sẵn có **bên trong VM**
  (`getEphemeralVmRecipeResultProjectRoot`), route qua `hostId`
  (`toRuntimeExecutionHostId`/`toSshExecutionHostId`, dòng 56-59).
- Nói cách khác: Orca "trỏ vào" 1 checkout đã tồn tại trong VM (VM tự
  tạo ra checkout đó qua recipe's `create` command), chứ không phải
  "mount checkout cục bộ vào trong VM".
- **Không có bước mount nào**: 0 kết quả cho "mount" trong mọi file
  `ephemeral-vm-*.ts` ở `frontend/src/shared`, `agent/src/shared`,
  `agent/src/relay`.
- **Không có bước copy-out nào**: workspace *chính là* checkout sống
  trong VM suốt vòng đời workspace đó — không gì copy nó về local.

Đây là câu hỏi cần trả lời trước: **mô hình "trỏ vào VM" có phải là ý
định sản phẩm thật (đơn giản hơn, tránh đồng bộ 2 chiều phức tạp), hay
đội ngũ trước đó build tắt và spec chưa được cập nhật lại?**

## Giải pháp đề xuất

### Bước 1 — Quyết định sản phẩm (không code trước)

2 nhánh:

**Nhánh A — Xác nhận mô hình hiện tại là v1 đúng ý.** Lý do hợp lý có
thể có: mount 2 chiều phức tạp, dễ lỗi đồng bộ, trong khi "VM tự tạo
checkout, Orca trỏ vào" đơn giản và đủ dùng cho use case ("cần môi
trường sạch hoàn toàn" — spec's "Vấn đề cần giải quyết"). Nếu xác nhận:
CR này trở thành **spec-correction** — sửa
`docs/features/F18-ephemeral-vm.md:44-47` khớp thực tế, đóng CR bằng 1
lần sửa doc, không code.

**Nhánh B — Xác nhận cần mount/copy-out thật.** Trường hợp hợp lý: user
cần bắt đầu từ 1 worktree cục bộ đã có sẵn (đã checkout, đã có
uncommitted changes) rồi mới chạy trong VM cô lập — recipe's `create`
command không tự tạo checkout được trong trường hợp này. Nếu xác nhận,
CR này trở thành feature lớn (xem "Nếu Nhánh B" dưới).

### Nếu Nhánh B — Sơ bộ scope kỹ thuật

- **Mount**: sau khi VM provision xong, agent (SSH outbound client hoặc
  orca-server path) cần copy/rsync worktree cục bộ vào VM (không phải
  "mount" theo nghĩa filesystem thật nếu VM là remote host qua SSH —
  cần làm rõ "mount" trong spec nghĩa là gì với target không share
  filesystem cục bộ).
- **Copy-out**: sau khi VM "xong" (định nghĩa "xong" là gì — user bấm
  nút, hay agent task hoàn thành — liên quan
  [CR-EVM-010](./CR-EVM-010-auto-destroy-on-task-completion.md)), copy
  kết quả về local trước khi destroy.
- Đây là subsystem có quy mô tương đương CR-EVM-005 gốc (agent transport
  mới) — không nên bắt đầu code trước khi Nhánh A/B được chốt.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Code trước khi có quyết định sản phẩm | Cao | Rủi ro lớn nhất của CR này — xây nhầm hướng nếu build mount/copy-out trong khi sản phẩm thực ra muốn giữ mô hình hiện tại |
| Nếu Nhánh B: mount/copy-out cho VM remote (SSH) phức tạp hơn local container | Cao | Cần thiết kế riêng cho từng `connection_type`, không có giải pháp chung 1 lần cho cả 2 |
| Nếu Nhánh A: user hiện tại có thể đã hiểu nhầm tính năng theo spec cũ | Thấp | Sửa spec + có thể cần thông báo/help text trong UI làm rõ mô hình thật |

## Không thuộc phạm vi CR này

- Port forwarding (dù cùng nằm trong spec section "Integration với
  Worktrees") — tách riêng ở [CR-EVM-008](./CR-EVM-008-ssh-target-port-forwards.md)
  vì đã có schema sẵn, không cần quyết định sản phẩm lại từ đầu.

## Liên quan

- `docs/features/F18-ephemeral-vm.md:44-47`
- `frontend/src/renderer/src/lib/ephemeral-vm-worktree-creation.ts`
- `frontend/src/renderer/src/lib/ephemeral-vm-workspace-target.ts:42-142`
- [CR-EVM-005](./CR-EVM-005-ssh-connection-type-outbound-client.md) (transport nền, đã Done)
- [CR-EVM-008](./CR-EVM-008-ssh-target-port-forwards.md), [CR-EVM-010](./CR-EVM-010-auto-destroy-on-task-completion.md) (liên quan cùng spec section)
