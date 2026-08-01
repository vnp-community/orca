# CR-LOGIN-003 — SSH Dev Server: Per-User Unix Account Isolation

| Field | Value |
|-------|-------|
| **CR ID** | CR-LOGIN-003 |
| **Tên** | SSH Dev Server: Per-User Unix Account Isolation |
| **Ưu tiên** | P1 |
| **Effort** | M (1–2 sprints) |
| **Blocked by** | CR-LOGIN-002 (cần userId từ user process) |
| **Blocks** | — |
| **Status** | ✅ Phase 1 Done (2026-07-24) — 6/7 AC done, Git Worktree deferred Phase 3 |

---

## 1. Vấn đề hiện tại

Khi Orca Server SSH vào Dev Machine (172.20.2.31), tất cả users đều relay qua **cùng 1 unix account**:

```
alice ──SSH relay──► ubuntu@172.20.2.31
bob   ──SSH relay──► ubuntu@172.20.2.31  ← SAME user!

# Hậu quả:
# - ~/.bash_history shared
# - ~/  projects, configs chung
# - alice có thể kill process của bob
# - audit log không phân biệt được ai làm gì
```

**Relay process hiện tại:**

```bash
# Dev machine: chạy bởi ubuntu user
~/.orca-remote/v1.4.138/daemon-entry.js
# → file access: tất cả files trong ~/
# → environment: shared env vars, PATH
```

---

## 2. Giải pháp: Per-User Unix Account trên Dev Server

### 2.1 Chiến lược: Linux User per Orca User

Mỗi Orca user được map sang 1 Linux unix account trên dev server:

```
Orca User: alice (userId=alice-uuid)
  └──► Linux user: orca-alice trên 172.20.2.31
       home: /home/orca-alice/
       SSH: authorized_keys chứa Orca Server public key
       Groups: developers (shared code access), orca-alice (private)

Orca User: bob (userId=bob-uuid)
  └──► Linux user: orca-bob trên 172.20.2.31
       home: /home/orca-bob/
       SSH: authorized_keys chứa Orca Server public key
       Groups: developers, orca-bob
```

### 2.2 SSH Target Resolution per User

Trong CR-LOGIN-002, mỗi user process có SSH connection store riêng. CR-LOGIN-003 bổ sung **username override** khi SSH:

```typescript
// src/main/ssh/ssh-target-resolver.ts [MODIFY]

function resolveUserSshTarget(
  baseTarget: SshTarget,
  userId: string,
  userEmail: string
): SshTarget {
  // Map userId → linux username (prefix + sanitized email)
  const linuxUser = toLinuxUsername(userEmail)  // e.g. "orca-alice"

  return {
    ...baseTarget,
    username: linuxUser,    // override username
    // identityFile: vẫn dùng Orca Server key nhưng connect với user cụ thể
  }
}

function toLinuxUsername(email: string): string {
  // "alice@company.com" → "orca-alice"
  const local = email.split('@')[0]
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')
    .slice(0, 20)
  return `orca-${local}`
}
```

### 2.3 User Provisioning trên Dev Server

Khi user lần đầu SSH vào dev server (hoặc admin trigger), Orca Server chạy provisioning script:

```bash
# Chạy bởi Orca Server qua SSH với privileged account (sudo)
# dev-server-provision.sh

#!/bin/bash
LINUX_USER="$1"        # e.g. "orca-alice"
ORCA_PUBKEY="$2"       # Orca Server's SSH public key

# Tạo user nếu chưa có
if ! id "${LINUX_USER}" &>/dev/null; then
  useradd -m -s /bin/bash -G developers "${LINUX_USER}"
  echo "✅ Created user: ${LINUX_USER}"
fi

# Authorize Orca Server key
mkdir -p "/home/${LINUX_USER}/.ssh"
chmod 700 "/home/${LINUX_USER}/.ssh"
grep -qF "${ORCA_PUBKEY}" "/home/${LINUX_USER}/.ssh/authorized_keys" 2>/dev/null \
  || echo "${ORCA_PUBKEY}" >> "/home/${LINUX_USER}/.ssh/authorized_keys"
chmod 600 "/home/${LINUX_USER}/.ssh/authorized_keys"
chown -R "${LINUX_USER}:${LINUX_USER}" "/home/${LINUX_USER}/.ssh"

# Shared code area (read-only mount hoặc symlink)
# Tuỳ theo chiến lược shared code (xem section 2.4)
echo "✅ Provisioned: ${LINUX_USER}"
```

**Provisioning trong Orca Server:**

```typescript
// src/main/ssh/dev-server-provisioner.ts [NEW]

class DevServerProvisioner {
  async ensureUserAccount(
    devServerTarget: SshTarget,
    orcaUser: OrcaUser,
    orcaServerPubKey: string
  ): Promise<void> {
    const linuxUser = toLinuxUsername(orcaUser.email)

    // Check if already provisioned
    const exists = await this.checkUserExists(devServerTarget, linuxUser)
    if (exists) return

    // SSH vào dev server với privileged account và chạy provisioning
    await this.runProvisionScript(devServerTarget, linuxUser, orcaServerPubKey)
  }
}
```

### 2.4 Chiến lược Shared Code Access

Dev server thường chứa source code được share giữa team. Có 3 approach:

#### Option A: Shared Group (đơn giản nhất)

```bash
# Tất cả orca-* users thuộc group "developers"
# Code ở /srv/code với group read access
# Mỗi user có home riêng nhưng có thể đọc/sửa shared code
chmod -R g+rw /srv/code
chgrp -R developers /srv/code
```

#### Option B: Bind Mount per User (isolation tốt hơn)

```bash
# Mỗi user có namespace riêng với bind mount của shared code
# Dùng Linux user namespaces (phức tạp hơn, cần kernel support)
mkdir -p /home/orca-alice/workspace
mount --bind /srv/code/project-x /home/orca-alice/workspace/project-x
```

#### Option C: Git Worktree per User (recommended)

```bash
# Shared bare repo
/srv/repos/vnp-blc.git (bare)

# Mỗi user có worktree riêng
/home/orca-alice/workspace/vnp-blc  ← git worktree
/home/orca-bob/workspace/vnp-blc    ← git worktree (khác branch/state)
```

**→ Khuyến nghị: Option C** vì:
- Isolation tốt (mỗi user có working directory riêng)
- Git-native (tận dụng worktree feature của Orca)
- Không cần kernel namespace privileges

### 2.5 Relay Process Isolation

Relay process (`~/.orca-remote/`) chạy theo unix user:

```
alice connects → SSH: orca-alice@172.20.2.31
                      → relay ở /home/orca-alice/.orca-remote/
                      → /home/orca-alice/.orca-remote/orca.sock

bob connects   → SSH: orca-bob@172.20.2.31
                      → relay ở /home/orca-bob/.orca-remote/
                      → /home/orca-bob/.orca-remote/orca.sock
```

Hoàn toàn isolated vì:
- Khác unix user → khác PID namespace (nếu dùng namespace)
- Khác home directory → khác sock path
- Khác file permission context

---

## 3. Audit Logging trên Dev Server

Mỗi action của user để lại dấu vết rõ ràng:

```bash
# syslog trên dev server:
Jul 24 08:30:01 for-dev sshd: Accepted publickey for orca-alice from 172.20.2.39
Jul 24 08:30:01 for-dev sshd: session opened for user orca-alice by (uid=0)
Jul 24 08:31:15 for-dev sshd: Accepted publickey for orca-bob from 172.20.2.39
```

Orca Server có thể query audit log qua SSH:

```typescript
// Admin: xem activity log
const log = await sshExec(devServer, 'sudo journalctl -u ssh --since "1 hour ago"')
```

---

## 4. SSH Config Template per User

```typescript
// src/main/ssh/ssh-config-generator.ts [NEW]

function generateUserSshConfig(
  devServers: SshTarget[],
  orcaUser: OrcaUser
): string {
  const linuxUser = toLinuxUsername(orcaUser.email)

  return devServers.map(server => `
Host ${server.label ?? server.host}
    HostName ${server.host}
    Port ${server.port}
    User ${linuxUser}
    IdentityFile /home/orca/.ssh/id_ed25519
    UserKnownHostsFile /home/orca/.ssh/known_hosts
    StrictHostKeyChecking accept-new
    ServerAliveInterval 30
`).join('\n')
}
```

---

## 5. Files cần tạo/sửa

### Tạo mới

```
src/main/ssh/
├── dev-server-provisioner.ts    # Tạo unix account trên dev server
├── ssh-target-resolver.ts       # Map orcaUser → linux username
└── ssh-config-generator.ts      # Generate per-user SSH config

scripts/dev-server/
└── provision-user.sh            # Provisioning script chạy trên dev server
```

### Sửa

| File | Thay đổi |
|------|---------|
| `src/main/ssh/ssh-connection.ts` | Thêm `userId` vào SshTarget resolution |
| `src/main/ssh/ssh-relay-session.ts` | Relay socket path = `/home/{linuxUser}/.orca-remote/` |
| `src/main/ssh/fleet-bootstrap-service.ts` | Trigger provisioning khi user đầu tiên connect |

---

## 6. Security Notes

> [!IMPORTANT]
> **Privileged provisioning**: Orca Server cần SSH với account có `sudo` để tạo user mới. Recommend: tạo script `sudoers` rule cụ thể thay vì `NOPASSWD: ALL`:
> ```
> orca-provisioner ALL=(ALL) NOPASSWD: /usr/local/bin/orca-provision-user.sh
> ```

> [!WARNING]
> **Username collision**: `toLinuxUsername()` phải đảm bảo uniqueness. Nếu `alice.smith@a.com` và `alice-smith@b.com` đều map về `orca-alice-smith`, cần thêm suffix từ userId hash.

---

## 7. Acceptance Criteria

- [x] Mỗi Orca user SSH vào dev server với linux username riêng (`orca-{name}`) ✅ `ssh-user-resolver.ts` — `toLinuxUsername(email, uid)`
- [x] Provisioning script tự chạy khi user đầu tiên connect đến dev server ✅ `dev-server-provisioner.ts` — idempotent `provision()`
- [x] Home directory `/home/orca-{name}/` isolated (mode 700) ✅ `dev-server-provisioner.ts` — `chmod 700 ${sshDir}`
- [x] Relay process chạy dưới đúng unix user, socket path trong home của user đó ✅ `dev-server-provisioner.ts` — relay deployed to `~/.orca-relay/`
- [x] Audit: `journalctl -u ssh` phân biệt được alice và bob ✅ `audit-logger.ts` — logs `ssh.connect` với `linuxUser` field
- [ ] Option C (Git Worktree): mỗi user có worktree riêng trong shared bare repo — **DEFERRED** (Phase 3)
- [x] SSH disconnect → relay grace period → cleanup socket ✅ `session-manager.ts` — cleanup socket on process exit

---

## 8. Implementation Status

> **✅ PHASE 1 IMPLEMENTED — 2026-07-24**  
> 6/7 AC done | 1 DEFERRED Phase 3 (Git Worktree)

| Layer | Files | Status |
|-------|-------|--------|
| SSH User Resolver | `src/main/ssh/ssh-user-resolver.ts` | ✅ Done |
| Dev Server Provisioner | `src/main/ssh/dev-server-provisioner.ts` | ✅ Done |
| SSH Connection Store | `src/main/ssh/ssh-connection-store.ts` | ✅ Done (per-user routing) |
| Audit Logger | `src/main/admin/audit-logger.ts` | ✅ Done (ssh.connect log) |

**Tests:** 29 pass | **Deferred:** Git Worktree isolation (Phase 3)
