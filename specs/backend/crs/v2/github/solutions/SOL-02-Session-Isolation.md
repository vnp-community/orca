# Solution cho CR-GH-004: Session Isolation bằng Linux Account

## Bối cảnh & TDD Specs liên kết
- Theo **TDD-05 (SSH Relay) - Addendum v4.0**, `DevServerProvisioner` tự động ánh xạ Orca `userId` thành một tài khoản Linux riêng biệt (e.g., `orca-alice-a1b2c3`) trên Dev Server.
- Bất kỳ kết nối SSH nào thông qua relay của người dùng này đều được thực thi dưới quyền của tài khoản Linux đó.

## Đánh giá vấn đề cũ
Ở CR ban đầu, chúng ta dự định dùng biến môi trường `GH_CONFIG_DIR` trỏ vào `/tmp/orca-sessions/<sessionId>` để cô lập trạng thái auth của nhiều người dùng trên cùng một Dev Server.

## Thiết kế giải pháp mới

**KHÔNG CẦN SỬ DỤNG `GH_CONFIG_DIR` HAY `GLAB_CONFIG_DIR`.**

Do SSH Relay kết nối vào tài khoản Linux riêng biệt của từng người dùng, thư mục Home (`$HOME`) của họ được OS cô lập hoàn toàn:
- User A (orca-user-A): `~/.config/gh/hosts.yml`
- User B (orca-user-B): `~/.config/gh/hosts.yml`

Sự cô lập ở tầng hệ điều hành (OS-level isolation) cung cấp độ an toàn cao hơn nhiều so với việc ghi đè biến môi trường.

### Xác nhận hoạt động
1. Khi gọi `gh auth login` thông qua `relay.call('pty.spawn')`, tiến trình sẽ chạy bằng UID của `orca-user-A`.
2. Khi tiến trình CLI đọc/ghi credentials, nó sẽ tự động sử dụng `/home/orca-user-A/.config/gh`.
3. Tương tự đối với `glab` và `/home/orca-user-A/.config/glab-cli`.

## Ưu điểm
- Zero-config: Không cần sửa mã nguồn gọi các CLI `gh`/`glab` trên Orca Server để tiêm env vars.
- Bảo mật tuyệt đối: OS ngăn chặn User A đọc credentials của User B.
- Loại bỏ hoàn toàn script dọn dẹp (cleanup) session thư mục `/tmp`.

---

## ✅ Implementation Status — COMPLETED (pre-existing + verified 2026-07-25)

### Trạng thái: Đây là phần **đã tồn tại sẵn** trong codebase, không cần implement thêm.

#### `src/main/ssh/dev-server-provisioner.ts` [EXISTS — VERIFIED]
- ✅ `DevServerProvisioner.ensureUserAccount(conn, userId, userEmail)` — tạo Linux account per-user
- ✅ `toLinuxUsername(userEmail, userId)` — map Orca userId → Linux username (`orca-{user}-{hash}`)
- ✅ `createUser()` — `sudo useradd -m -s /bin/bash {linuxUser}` 
- ✅ `authorizeKey()` — thêm Orca Server SSH public key vào `~/.ssh/authorized_keys` của user
- ✅ Idempotent: `checkUserExists()` trước khi tạo, không duplicate

#### `src/main/ssh/ssh-user-resolver.ts` [EXISTS — VERIFIED]
- ✅ `toLinuxUsername()` — function chuyển đổi userId thành Linux-safe username

### Isolation flow đã hoạt động
```
PTY spawn via relay (pty.spawn) 
  → relay kết nối với SSH key của orca-user-A 
  → tiến trình chạy với UID(orca-user-A)
  → gh/glab đọc ~/.config từ $HOME của orca-user-A
  → credentials hoàn toàn tách biệt với orca-user-B
```

### Phần KHÔNG cần implement (đúng thiết kế)
- ❌ `GH_CONFIG_DIR` / `GLAB_CONFIG_DIR` env override — **không cần**, OS isolation đủ
- ❌ Cleanup scripts cho `/tmp/orca-sessions/` — **không cần**
- ❌ Sửa source code `gh`/`glab` calls — **không cần**
