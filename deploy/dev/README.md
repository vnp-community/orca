# Orca Dev Server — README Triển khai

## Tổng quan

Mô hình **build local → sync → run minimal container**:

```
[Máy Developer / CI]                  [Orca Server]
─────────────────────                 ─────────────────────────────────────
1. build-local.sh                     4. docker compose up -d
   ↓ pnpm build                          ┌──────────────────────┐
   ↓ electron-builder --linux --dir      │  orca  (debian-slim) │ ← app mount
   → dist/linux-unpacked/               │  nginx (alpine 5MB)  │ ← TLS proxy
                                         └──────────────────────┘
2. gen-certs.sh  (1 lần)
   → docker/nginx/certs/server.crt

3. sync-to-server.sh
   → rsync dist/ + docker/ lên server
```

## Cấu trúc thư mục

```
deploy/dev/
├── docker-compose.yml          # Chạy trên server
├── .env.example                # Config template
├── .env                        # Config thực (gitignored)
├── .gitignore
│
├── scripts/
│   ├── build-local.sh          # [LOCAL] Build Orca → dist/linux-unpacked/
│   ├── sync-to-server.sh       # [LOCAL] Rsync lên server + restart
│   ├── gen-certs.sh            # [LOCAL] Tạo self-signed TLS cert
│   ├── setup-ssh-keys.sh       # [LOCAL] Setup SSH key: Orca Server → Dev Machine
│   └── get-pairing-url.sh      # [LOCAL] Lấy Pairing URL/Code để vào web UI
│
├── docker/
│   ├── orca/
│   │   ├── Dockerfile          # debian:bookworm-slim + runtime libs
│   │   ├── entrypoint.sh       # Xvfb → orca serve
│   │   └── ssh/                # SSH keys (gitignored)
│   │       ├── id_ed25519      # Private key → dev servers
│   │       └── config          # SSH config
│   └── nginx/
│       ├── nginx.conf          # Nginx global config
│       ├── conf.d/
│       │   └── orca.conf       # HTTPS + WebSocket proxy
│       └── certs/              # TLS certs (gitignored)
│           ├── server.crt
│           └── server.key
│
├── dist/                       # Build output (gitignored)
│   └── linux-unpacked/         # Electron app (synced từ local build)
│
└── specs/                      # Tài liệu kiến trúc
    ├── 00-overview.md
    └── ...
```

## Quick Start

### Lần đầu tiên

```bash
cd /path/to/orca   # root của Orca repo

# 1. Copy và điền config
cp deploy/dev/.env.example deploy/dev/.env
nano deploy/dev/.env
# → Set ORCA_DOMAIN, SERVER_HOST, DEV_SERVER_HOST, etc.

# 2. Tạo TLS certificate
ORCA_DOMAIN=orca.vnpblc.internal \
  bash deploy/dev/scripts/gen-certs.sh

# 3. Chuẩn bị SSH key để Orca Server SSH vào Dev Machine
bash deploy/dev/scripts/setup-ssh-keys.sh
# → Sinh key tại deploy/dev/docker/orca/ssh/
# → Authorize public key vào 172.20.2.31 tự động
# → Sync SSH dir lên Orca Server (172.20.2.39)

# 4. Build Orca (chạy 1 lần, ~10-15 phút)
bash deploy/dev/scripts/build-local.sh

# 5. Sync lên server + build Docker image + start
bash deploy/dev/scripts/sync-to-server.sh --restart

# 6. Lấy pairing URL
ssh ubuntu@172.20.2.39 \
  "docker logs orca-server 2>&1 | grep 'Web UI'"
```

### Deploy sau khi có thay đổi code

```bash
# Build lại
bash deploy/dev/scripts/build-local.sh

# Sync và restart (image KHÔNG rebuild, chỉ restart container)
bash deploy/dev/scripts/sync-to-server.sh

# Nếu Dockerfile thay đổi:
bash deploy/dev/scripts/sync-to-server.sh --restart
```

## Image Size

| Image | Base | Size |
|-------|------|------|
| `orca` runtime | `debian:bookworm-slim` | ~160 MB |
| `nginx` proxy | `nginx:alpine` | ~5 MB |

Container KHÔNG chứa build tools (Node.js, pnpm, TypeScript, v.v.) → image nhỏ nhất có thể.

## Developer Access — Lấy Pairing URL

```bash
# Cách đơn giản nhất: in URL + code
bash deploy/dev/scripts/get-pairing-url.sh

# Mở browser ngay (macOS/Linux)
bash deploy/dev/scripts/get-pairing-url.sh --open

# Tạo token mới (revoke token cũ chưa dùng)
bash deploy/dev/scripts/get-pairing-url.sh --rotate

# Chỉ lấy URL (dùng trong script/pipe)
ORCA_URL=$(bash deploy/dev/scripts/get-pairing-url.sh --url)
echo "${ORCA_URL}"

# JSON output đầy đủ
bash deploy/dev/scripts/get-pairing-url.sh --json
```

**Output:**
```
🔗 Orca Pairing Info
═══════════════════════════════════════════════════════════

Cách 1 — Mở URL trực tiếp (auto-connect):
https://b15.openledger.vn/#pairing=eyJ2Ij...

Cách 2 — Paste vào field "Pairing URL or code":
eyJ2IjoyLCJlbmRwb2ludCI6IndzczovL...
```

## Quản lý trên Server

```bash
# SSH vào server
ssh ubuntu@orca.vnpblc.internal

# Trên server:
cd ~/orca-deploy

docker compose ps           # xem trạng thái
docker compose logs -f orca # xem logs realtime
docker compose restart orca # restart Orca
docker compose down          # dừng tất cả
docker compose up -d         # start lại

# Xem pairing URL
docker logs orca-server 2>&1 | grep "Web UI"

# Monitor resource usage
docker stats
```

## SSH Keys cho Dev Servers

Orca Server (container tại 172.20.2.39) cần SSH vào Dev Machine (172.20.2.31)
để relay filesystem, terminal, và git operations.

### Cách nhanh (recommended) — dùng script

```bash
# Sinh key LOCAL + authorize vào dev server + sync lên Orca Server
bash deploy/dev/scripts/setup-ssh-keys.sh

# Chỉ authorize thêm một server khác:
bash deploy/dev/scripts/setup-ssh-keys.sh --authorize 172.20.2.32

# In public key để copy thủ công:
bash deploy/dev/scripts/setup-ssh-keys.sh --print-pubkey

# Test SSH từ Orca container → dev server:
bash deploy/dev/scripts/setup-ssh-keys.sh --test
```

### Cách thủ công

```bash
# 1. Sinh key trên máy local
mkdir -p deploy/dev/docker/orca/ssh
ssh-keygen -t ed25519 \
  -f deploy/dev/docker/orca/ssh/id_ed25519 \
  -N "" \
  -C "orca-server@172.20.2.39"

# 2. Authorize lên dev server
ssh-copy-id -i deploy/dev/docker/orca/ssh/id_ed25519.pub \
  ubuntu@172.20.2.31

# 3. Tạo SSH config
cat > deploy/dev/docker/orca/ssh/config << 'EOF'
Host dev-local
    HostName 172.20.2.31
    User ubuntu
    IdentityFile /home/orca/.ssh/id_ed25519
    UserKnownHostsFile /home/orca/.ssh/known_hosts
    StrictHostKeyChecking accept-new
EOF

# 4. Lấy fingerprint
ssh-keyscan -H 172.20.2.31 > deploy/dev/docker/orca/ssh/known_hosts

# 5. Sync SSH dir lên Orca Server
rsync -az deploy/dev/docker/orca/ssh/ \
  ubuntu@172.20.2.39:~/orca-deploy/docker/orca/ssh/

# 6. Restart container để mount SSH dir mới
ssh ubuntu@172.20.2.39 \
  "cd ~/orca-deploy && docker compose -f docker-compose.orca.yml restart orca"
```

### Thêm dev server vào Orca (sau khi SSH đã setup)

```
https://b15.openledger.vn
→ Add Remote Host
  Host:     172.20.2.31
  Port:     22
  Username: ubuntu
→ Connect
```

Orca sẽ tự động SSH vào 172.20.2.31 và deploy relay process (`~/.orca-remote/`).

### Files SSH (trong deploy/dev/docker/orca/ssh/)

| File | Gitignore | Mô tả |
|------|-----------|-------|
| `id_ed25519` | ✅ YES | Private key — không commit |
| `id_ed25519.pub` | ❌ no | Public key — an toàn commit |
| `config` | ❌ no | SSH config — an toàn commit |
| `known_hosts` | ❌ no | Host fingerprints — an toàn commit |
