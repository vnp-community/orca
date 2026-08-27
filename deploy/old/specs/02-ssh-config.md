# SSH Config (Internal) — Orca Server ↔ Dev Servers

**Lưu ý:** File này chỉ mô tả cấu hình SSH **giữa Orca Server và các Dev Servers** (internal).  
Developer **KHÔNG cần** cài SSH hay cấu hình SSH key. Họ kết nối trực tiếp qua Browser/App/Mobile.

---

## 1. Mục đích của SSH trong mô hình mới

```
Developer → Browser/App → [HTTPS/WSS] → Orca Server
                                             │
                               SSH (internal only)
                                             │
                                    ┌────────┼────────┐
                                    ▼        ▼        ▼
                               Dev Alpha  Dev Beta  Dev Gamma
                               (Projects)
```

SSH chỉ được dùng **từ Orca Server → Dev Servers** để:
- Chạy git commands
- Spawn AI agents (PTY)
- Đọc/ghi file projects
- Orca Relay (nếu dùng)

---

## 2. Setup SSH Key cho Orca Server → Dev Servers

### 2.1 Tạo SSH key của Orca Server

```bash
# Trên Orca Server, với user orca
sudo su - orca

ssh-keygen -t ed25519 -C "orca-server@vnpblc.internal" -f ~/.ssh/orca_server_key -N ""

# Key sẽ được tạo:
# ~/.ssh/orca_server_key       (private — không share)
# ~/.ssh/orca_server_key.pub   (public — thêm vào từng dev server)

cat ~/.ssh/orca_server_key.pub
# → Copy output, thêm vào từng dev server
```

### 2.2 Thêm public key vào từng Dev Server

```bash
# Trên dev-alpha (mỗi dev server làm tương tự)
sudo useradd -m -s /bin/bash dev
sudo mkdir -p /home/dev/.ssh
sudo chmod 700 /home/dev/.ssh

# Thêm public key của orca-server
echo "ssh-ed25519 AAAA... orca-server@vnpblc.internal" | \
  sudo tee /home/dev/.ssh/authorized_keys

sudo chmod 600 /home/dev/.ssh/authorized_keys
sudo chown -R dev:dev /home/dev/.ssh
```

### 2.3 SSH Config trên Orca Server

```sshconfig
# /home/orca/.ssh/config (trên Orca Server)

# Dev Server Alpha (Project vnp-blc)
Host dev-alpha
  HostName dev-alpha.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22
  StrictHostKeyChecking accept-new
  ServerAliveInterval 60
  ServerAliveCountMax 5

# Dev Server Beta (Project vnp-ai-ops)
Host dev-beta
  HostName dev-beta.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22
  StrictHostKeyChecking accept-new
  ServerAliveInterval 60
  ServerAliveCountMax 5

# Dev Server Gamma (Project vnp-claw)
Host dev-gamma
  HostName dev-gamma.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22
  StrictHostKeyChecking accept-new
  ServerAliveInterval 60
  ServerAliveCountMax 5
```

### 2.4 Test kết nối

```bash
# Trên Orca Server
ssh dev-alpha "echo 'OK: dev-alpha'"
ssh dev-beta  "echo 'OK: dev-beta'"
ssh dev-gamma "echo 'OK: dev-gamma'"
```

---

## 3. Firewall cho Dev Servers

```bash
# Trên mỗi dev server: chỉ nhận SSH từ Orca Server
sudo ufw default deny incoming
sudo ufw allow from 10.10.0.100 to any port 22   # Chỉ từ Orca Server IP
sudo ufw enable
```

---

## 4. Checklist

- [ ] Orca Server key pair đã tạo (`~/.ssh/orca_server_key`)
- [ ] Public key đã thêm vào `authorized_keys` trên từng dev server
- [ ] SSH Config `/home/orca/.ssh/config` đã tạo
- [ ] Test: `ssh dev-alpha "echo OK"` từ orca-server thành công
- [ ] Firewall: dev servers chỉ nhận SSH từ Orca Server IP
