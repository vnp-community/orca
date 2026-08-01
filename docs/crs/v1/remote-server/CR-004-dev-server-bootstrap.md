# CR-004 — Dev Server Bootstrap Automation

**CR-ID:** CR-004  
**Ngày:** 2026-07-22  
**Priority:** 🟠 High  
**Effort:** Medium (2–3 ngày)  
**Status:** Implemented  

---

## 1. Vấn đề

Orca tự động deploy `orca-relay` binary lên server. Nhưng để relay hoạt động và dev có thể làm việc, server cần được **bootstrap** trước:

1. **Node.js 22+** — Orca relay cần, auto-detect có (nhưng không auto-install)
2. **Git 2.35+** — Git worktree cần
3. **Project repos** — clone về đúng path
4. **SSH key** setup cho git push
5. **`orca.yaml` setup script** — chưa chạy tự động

**Hiện tại:**
- Orca tự detect Node.js version (`ssh-remote-node-resolution.ts`) và show hướng dẫn install nếu thiếu
- Orca **không** tự install Node.js, Git, clone repos

---

## 2. Phân tích codebase

### 2.1 Node.js detection hiện có

```typescript
// src/main/ssh/ssh-remote-node-resolution.ts
// Detect Node.js path trên remote host
export async function resolveRemoteNodePath(
  connection: SshConnection
): Promise<string>
// → Thử: node, nodejs, ~/.nvm/versions/node/*/bin/node, ~/.fnm/...
// Nếu không tìm thấy → throw với hướng dẫn install
```

```typescript
// src/main/ssh/ssh-remote-node-install-guidance.ts
// Tạo error message với hướng dẫn install Node.js
export function formatNodeInstallGuidance(platform: RemoteHostPlatform): string
```

**Gap:** Chỉ detect và guide — không auto-install.

### 2.2 `orca.yaml` setup script

```typescript
// orca.yaml
scripts:
  setup: |
    npm install
    # → Chạy khi tạo worktree (Orca trigger tự động)
    # NHƯNG chỉ chạy trong worktree context, không phải server bootstrap
```

**Gap:** `orca.yaml` setup chỉ chạy cho worktree, không phải toàn bộ server.

### 2.3 Ephemeral VM recipe

```typescript
// src/shared/ephemeral-vm-recipes.ts
// EphemeralVmRecipe có `projectRoot` nhưng không có bootstrap steps
```

---

## 3. Giải pháp đề xuất

### 3.1 Bootstrap script cho từng dev server

```bash
#!/usr/bin/env bash
# deploy/dev/scripts/bootstrap-dev-server.sh
# Chạy trên dev server để chuẩn bị cho Orca kết nối
#
# Usage: bash bootstrap-dev-server.sh [--project vnp-blc] [--repo-path /srv/projects]

set -euo pipefail

PROJECT="${1:-}"
REPO_ROOT="${REPO_ROOT:-/srv/projects}"

echo "======================================================"
echo " Dev Server Bootstrap"
echo "======================================================"

# ── 1. Node.js 22 ─────────────────────────────────────────
echo "[1/5] Installing Node.js 22..."
if ! command -v node &>/dev/null || [ "$(node --version | cut -d. -f1 | tr -d v)" -lt 22 ]; then
    curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi
echo "  ✅ Node.js: $(node --version)"

# ── 2. Git 2.35+ ──────────────────────────────────────────
echo "[2/5] Installing Git..."
if ! command -v git &>/dev/null; then
    sudo add-apt-repository ppa:git-core/ppa -y
    sudo apt-get update && sudo apt-get install -y git
fi
echo "  ✅ Git: $(git --version)"

# ── 3. SSH key cho git operations ─────────────────────────
echo "[3/5] Setting up SSH for git..."
mkdir -p ~/.ssh && chmod 700 ~/.ssh
# → SSH key được inject qua authorized_keys hoặc secrets management

# ── 4. Clone/update project repos ─────────────────────────
echo "[4/5] Setting up project repos..."
mkdir -p "${REPO_ROOT}"

# Đọc repos từ fleet config (nếu có)
if [ -f /etc/orca/fleet-repos.conf ]; then
    while IFS='|' read -r REPO_URL REPO_PATH; do
        if [ ! -d "${REPO_PATH}/.git" ]; then
            echo "  → Cloning $REPO_URL → $REPO_PATH"
            git clone "$REPO_URL" "$REPO_PATH"
        else
            echo "  → Updating $REPO_PATH"
            git -C "$REPO_PATH" fetch --all
        fi
    done < /etc/orca/fleet-repos.conf
fi

# ── 5. Verify Orca relay requirements ─────────────────────
echo "[5/5] Verifying Orca relay requirements..."
node --version
git --version
echo "  ✅ Server ready for Orca relay"

echo ""
echo "======================================================"
echo " ✅ Bootstrap complete!"
echo "======================================================"
echo "  Next: Add this server to Orca fleet config"
echo "        orca fleet import deploy/dev/orca-fleet.yaml"
echo "        orca fleet provision --server $(hostname)"
```

### 3.2 Extend `orca-fleet.yaml` với bootstrap config

```yaml
# deploy/dev/orca-fleet.yaml
version: "1"

# Bootstrap config áp dụng cho tất cả servers
bootstrap:
  nodeVersion: "22"
  gitVersion: "2.35"
  packages:
    - git
    - curl
    - xvfb    # nếu server cần chạy browser automation

servers:
  - id: dev-alpha
    label: "Dev Alpha — vnp-blc"
    host: dev-alpha.vnpblc.internal
    bootstrap:
      repos:
        - url: git@github.com:vnpblc/vnp-blc.git
          path: /srv/projects/vnp-blc
          branch: develop
        - url: git@github.com:vnpblc/vnp-blc-infra.git
          path: /srv/projects/vnp-blc-infra
          branch: main
      setupScript: |
        cd /srv/projects/vnp-blc
        go mod download
        cp .env.example .env
```

### 3.3 Orca bootstrap command (đề xuất)

```bash
# Bootstrap một server từ fleet config
orca fleet bootstrap dev-alpha

# Bootstrap tất cả servers
orca fleet bootstrap --all

# Output:
# [dev-alpha] Installing Node.js... ✅
# [dev-alpha] Verifying Git... ✅
# [dev-alpha] Cloning vnp-blc... ✅
# [dev-alpha] Running setup script... ✅
# [dev-alpha] ✅ Bootstrap complete
```

---

## 4. Changes Required

### 4.1 Orca codebase

| File | Thay đổi |
|------|---------|
| `src/main/ssh/ssh-remote-node-resolution.ts` | Thêm auto-install option (opt-in) |
| `src/main/ssh/ssh-remote-commands.ts` | Thêm `installNodeCommand()`, `cloneRepoCommand()` |
| `src/cli/specs/fleet.ts` | Thêm `fleet bootstrap` command |
| `src/cli/handlers/fleet.ts` | Implement bootstrap handler |

### 4.2 Deploy scripts

| File | Thay đổi |
|------|---------|
| `deploy/dev/scripts/bootstrap-dev-server.sh` | [NEW] Bootstrap script |
| `deploy/dev/orca-fleet.yaml` | Thêm `bootstrap` section |

---

## 5. Workaround hiện tại

Chạy bootstrap script thủ công trên mỗi dev server:

```bash
# SSH vào dev server
ssh dev@dev-alpha.vnpblc.internal

# Chạy script bootstrap
bash <(curl -fsSL https://raw.githubusercontent.com/vnpblc/orca-deploy/main/bootstrap.sh)
```

Hoặc dùng Ansible:

```yaml
# deploy/dev/scripts/bootstrap.yml (Ansible playbook)
---
- hosts: dev_servers
  become: true
  tasks:
    - name: Install Node.js 22
      shell: curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && apt-get install -y nodejs

    - name: Install Git
      apt:
        name: git
        state: present
        update_cache: yes

    - name: Clone project repos
      git:
        repo: "{{ item.url }}"
        dest: "{{ item.path }}"
        version: "{{ item.branch | default('develop') }}"
      loop: "{{ repos }}"

    - name: Setup SSH key for git
      authorized_key:
        user: dev
        key: "{{ lookup('file', 'docker/orca/ssh/id_ed25519.pub') }}"
```

```ini
# deploy/dev/scripts/hosts.ini (Ansible inventory)
[dev_servers]
dev-alpha.vnpblc.internal
dev-beta.vnpblc.internal
dev-gamma.vnpblc.internal

[dev_servers:vars]
ansible_user=ubuntu
ansible_ssh_private_key_file=~/.ssh/id_ed25519
```

**Chạy:**
```bash
ansible-playbook -i scripts/hosts.ini scripts/bootstrap.yml \
  -e "repos=[{url:'git@github.com:vnpblc/vnp-blc.git', path:'/srv/projects/vnp-blc'}]"
```

---

## 6. Acceptance Criteria

- [x] `orca fleet bootstrap <server-id>` cài Node.js nếu thiếu
- [x] Bootstrap clone tất cả repos theo `orca-fleet.yaml`
- [x] Bootstrap chạy `setup` script từ `orca.yaml` của từng project
- [x] Idempotent: chạy lại an toàn (skip đã có)
- [x] Report rõ ràng: mỗi step success/fail với `BootstrapResult` type
- [x] Rollback on failure: step-level error capture, không tối đa hóa server state

---

## 7. Implementation Notes

> **Implemented:** 2026-07-23

| File | Status |
|------|--------|
| `src/main/ssh/fleet-bootstrap-service.ts` | ✅ [NEW] Orchestrate bootstrap: node-check → node-install → git-check → packages → repo-clone → setup-script → verify |
| `src/main/ssh/fleet-remote-commands.ts` | ✅ [NEW] `installNodeJs()`, `ensureGitInstalled()`, `cloneOrUpdateRepo()`, `installPackages()`, `runRemoteScript()` |
| `src/shared/fleet-types.ts` | ✅ [NEW] `BootstrapResult`, `BootstrapStep`, `BootstrapStepName` types |
| `src/cli/specs/fleet.ts` | ✅ [MODIFY] `fleet bootstrap` command spec |
| `src/cli/handlers/fleet.ts` | ✅ [MODIFY] `fleet bootstrap` handler với 2-server concurrent limit |
| `src/main/ipc/ssh.ts` | ✅ [MODIFY] `ssh.bootstrapServer` IPC handler |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | 6/6 AC done**

Dev server bootstrap implemented — SSH relay deploy, agent detection, and health initialization.

| File | Status |
|------|--------|
| `src/main/ssh/fleet-bootstrap-service.ts` | ✅ Bootstrap orchestration |
| `src/main/ssh/fleet-remote-commands.ts` | ✅ `detectAgentOnRemote()` |
| `src/main/ssh/fleet-status-service.ts` | ✅ `FleetStatusService` |
| `src/main/server-bootstrap.ts` | ✅ `FleetHealthMonitor.start()` wired |
