# Dev Server Setup — Cài đặt và Cấu hình từng Dev Server

**Mục tiêu:** Mỗi Dev Server phục vụ cho 1 project riêng biệt. Developer SSH vào dev server (qua orca-server) để làm việc với project đó.

---

## 1. Mô hình phân chia Dev Server

```
orca-server (gateway)
│
├── dev-alpha  →  Project Alpha  (e.g., vnp-blc backend)
│                 - repo: /srv/projects/vnp-blc
│                 - services: postgres, redis, kafka
│
├── dev-beta   →  Project Beta   (e.g., vnp-ai-ops)
│                 - repo: /srv/projects/vnp-ai-ops
│                 - services: postgres, elasticsearch
│
└── dev-gamma  →  Project Gamma  (e.g., vnp-claw)
                  - repo: /srv/projects/vnp-claw
                  - services: postgres, rabbitmq
```

---

## 2. Setup cơ bản mỗi Dev Server

### 2.1 Chuẩn bị hệ thống

```bash
# Chạy trên dev server (ví dụ: dev-alpha)
sudo apt-get update && sudo apt-get upgrade -y
sudo apt-get install -y \
  curl wget git build-essential \
  openssh-server \
  docker.io docker-compose-plugin \
  postgresql-client redis-tools \
  htop tmux vim

# Cài Node.js 22 LTS
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs

# Kiểm tra
git --version && node --version && docker --version
```

### 2.2 Tạo user `dev` chung

```bash
# User chung cho tất cả developer access
sudo useradd -m -s /bin/bash dev
sudo usermod -aG docker dev   # Cho phép dùng docker không cần sudo

# Tạo thư mục SSH
sudo mkdir -p /home/dev/.ssh
sudo chmod 700 /home/dev/.ssh
sudo chown dev:dev /home/dev/.ssh
```

### 2.3 Cấp quyền SSH cho Developer

```bash
# Thêm public keys (từ orca-server hoặc trực tiếp từ developer)
# Cách 1: Copy từ orca-server
cat /home/orca/.ssh/authorized_keys | sudo tee /home/dev/.ssh/authorized_keys

# Cách 2: Thêm thủ công
echo "ssh-ed25519 AAAA... dev@vnpblc.com" | sudo tee -a /home/dev/.ssh/authorized_keys

sudo chmod 600 /home/dev/.ssh/authorized_keys
sudo chown dev:dev /home/dev/.ssh/authorized_keys
```

---

## 3. Cấu hình Project Repository

### 3.1 Tạo thư mục project

```bash
sudo mkdir -p /srv/projects
sudo chown dev:dev /srv/projects

# Với user dev
sudo su - dev
cd /srv/projects
```

### 3.2 Clone project repo

```bash
# Ví dụ cho Project Alpha
git clone git@github.com:vnpblc/vnp-blc.git /srv/projects/vnp-blc

# Cấu hình git
git config --global user.name "VNP Dev Server"
git config --global user.email "devserver@vnpblc.com"
git config --global core.autocrlf false
```

### 3.3 Cấu hình SSH Deploy Key cho GitHub

```bash
# Tạo deploy key cho dev server
sudo su - dev
ssh-keygen -t ed25519 -C "dev-alpha-deploy@vnpblc.com" -f ~/.ssh/deploy_key -N ""
cat ~/.ssh/deploy_key.pub
# → Copy và thêm vào GitHub repo → Settings → Deploy keys
```

```sshconfig
# /home/dev/.ssh/config
Host github.com
  HostName github.com
  User git
  IdentityFile ~/.ssh/deploy_key
```

---

## 4. Cấu hình Docker Compose cho Services

### 4.1 Template `docker-compose.dev.yml`

```yaml
# /srv/projects/vnp-blc/docker-compose.dev.yml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: dev
      POSTGRES_PASSWORD: devpassword
      POSTGRES_DB: vnpblc_dev
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    restart: unless-stopped

  kafka:
    image: bitnami/kafka:latest
    ports:
      - "9092:9092"
    environment:
      KAFKA_CFG_PROCESS_ROLES: controller,broker
      KAFKA_CFG_NODE_ID: 1
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
    restart: unless-stopped

volumes:
  postgres_data:
```

### 4.2 Khởi động services

```bash
cd /srv/projects/vnp-blc
docker compose -f docker-compose.dev.yml up -d

# Kiểm tra
docker compose ps
docker compose logs -f
```

---

## 5. Environment Variables

### 5.1 File `.env.dev` (không commit vào git)

```bash
# /srv/projects/vnp-blc/.env.dev
NODE_ENV=development
DATABASE_URL=postgresql://dev:devpassword@localhost:5432/vnpblc_dev
REDIS_URL=redis://localhost:6379
KAFKA_BROKERS=localhost:9092

# AI API Keys (mỗi developer set key riêng của mình)
# ANTHROPIC_API_KEY=sk-ant-...
# OPENAI_API_KEY=sk-...
```

### 5.2 Shared secrets qua Vault (production pattern)

```bash
# Tuỳ chọn: dùng HashiCorp Vault để quản lý secrets
vault kv get secret/vnp-blc/dev
```

---

## 6. Cấu hình AI Agents trên Dev Server

Developer muốn chạy AI agent trực tiếp trên dev server (tận dụng CPU/RAM server, không tốn tài nguyên máy local):

### 6.1 Cài Claude Code per-developer

```bash
# Developer A SSH vào server
ssh dev-alpha

# Tạo user profile riêng nếu dùng user chung
mkdir -p ~/.config/claude ~/.local/share/claude

# Cài claude code
npm install -g @anthropic-ai/claude-code

# Set API key (mỗi developer set key riêng)
export ANTHROPIC_API_KEY="sk-ant-..."
echo 'export ANTHROPIC_API_KEY="sk-ant-..."' >> ~/.bashrc
```

### 6.2 Cài Gemini/Antigravity CLI

```bash
npm install -g @google/gemini-cli
export GEMINI_API_KEY="..."
echo 'export GEMINI_API_KEY="..."' >> ~/.bashrc
```

### 6.3 Isolate per-developer với tmux

```bash
# Mỗi developer mở session tmux riêng
tmux new-session -s dev-alice
tmux new-session -s dev-bob

# List sessions
tmux list-sessions
```

---

## 7. Cấu hình Orca Worktrees trên Dev Server

Khi developer kết nối SSH Worktree từ Orca Desktop, Orca sẽ:
1. Deploy `orca-relay` binary lên `~/.local/bin/orca-relay`
2. Start relay process
3. Tạo worktree trong thư mục project

### 7.1 Cấu hình `orca.yaml` per-project

```yaml
# /srv/projects/vnp-blc/orca.yaml
scripts:
  setup: |
    npm install
    cp .env.dev .env
    docker compose -f docker-compose.dev.yml up -d
```

Khi developer tạo worktree mới trong Orca, script `setup` sẽ chạy tự động.

---

## 8. Checklist mỗi Dev Server

- [ ] OS updated, Node.js 22, Git 2.35+ đã cài
- [ ] User `dev` đã tạo với quyền docker
- [ ] SSH authorized_keys đã thêm cho team
- [ ] Project repo đã clone tại `/srv/projects/<name>`
- [ ] Docker Compose services đã khởi động
- [ ] `.env.dev` đã cấu hình
- [ ] `orca.yaml` đã tạo trong repo
- [ ] Test: `ssh dev-alpha` từ orca-server thành công
- [ ] Test: Orca Desktop → SSH Worktree → dev-alpha thành công

---

## 9. Naming Convention

| Dev Server | Hostname | Project | IP |
|-----------|----------|---------|-----|
| dev-alpha | dev-alpha.vnpblc.internal | vnp-blc | 10.10.1.10 |
| dev-beta | dev-beta.vnpblc.internal | vnp-ai-ops | 10.10.1.20 |
| dev-gamma | dev-gamma.vnpblc.internal | vnp-claw | 10.10.1.30 |
| dev-delta | dev-delta.vnpblc.internal | (dự trữ) | 10.10.1.40 |
