---
task_id: T-P2-004
phase: 2
cr_id: CR-ORCA-004
solution_ref: solutions/CR-ORCA-004-solution.md
service: orca-agent
target_files:
  - deploy/orca-agent/config/orca-agent.yaml
  - deploy/orca-agent/docker-compose.yml
  - deploy/orca-agent/.env.example
priority: P1
status: done
depends_on: T-P2-001
estimated_effort: 30 phút
---

# T-P2-004 — Cập nhật deploy config: bridges + mcp_proxy sections

## Context

`deploy/orca-agent/` cần được cập nhật để operators có thể cấu hình PIKB KG bridge và MCP proxy. Đây là task deploy kit — không liên quan đến Go code, chỉ là YAML/bash config files.

## Target files

```
deploy/orca-agent/config/orca-agent.yaml
deploy/orca-agent/docker-compose.yml
deploy/orca-agent/.env.example      ← tạo mới nếu chưa có
```

## Instructions

### 1. `deploy/orca-agent/config/orca-agent.yaml`

Đọc file hiện tại, sau đó **thêm vào cuối** (hoặc sau section phù hợp):

```yaml
# Bridges — peer agent IDs cho cross-agent tool calls (CR-ORCA-004)
# Để trống ở đây, override bằng env vars khi deploy.
bridges:
  products_agent_id: ""    # Override: ORCA_BRIDGES_PRODUCTS_AGENT_ID
  doc_agent_id: ""         # Override: ORCA_BRIDGES_DOC_AGENT_ID

# MCP Proxy — peer-invoke via HTTP cho pis_* tools (CR-ORCA-005)
mcp_proxy:
  url: ""          # Override: ORCA_MCP_PROXY_URL (vd: http://mcp-proxy:3012)
  peer_token: ""   # Override: ORCA_MCP_PROXY_PEER_TOKEN
```

### 2. `deploy/orca-agent/docker-compose.yml`

Trong section `environment:` của service `orca-agent`, **thêm** các dòng sau:

```yaml
- ORCA_BRIDGES_PRODUCTS_AGENT_ID=${ORCA_BRIDGES_PRODUCTS_AGENT_ID:-}
- ORCA_BRIDGES_DOC_AGENT_ID=${ORCA_BRIDGES_DOC_AGENT_ID:-}
- ORCA_MCP_PROXY_URL=${ORCA_MCP_PROXY_URL:-http://mcp-proxy:3012}
- ORCA_MCP_PROXY_PEER_TOKEN=${ORCA_MCP_PROXY_PEER_TOKEN:-}
```

### 3. `deploy/orca-agent/.env.example`

Tạo file (nếu chưa tồn tại) hoặc thêm vào cuối file hiện có:

```bash
# =============================================
# orca-agent — Environment Variables Reference
# =============================================

# --- Required ---
ORCA_BOOTSTRAP_TOKEN=<from workplace admin>
ORCA_API_URL=http://orca-web:9000
ORCA_API_KEY=<orca service key>

# --- CR-ORCA-004: PIKB KG Bridge (optional) ---
# Nếu set, orca-agent sẽ expose pis_* tools cho AI agent.
# Để trống nếu không cần tích hợp PIKB KG.
ORCA_BRIDGES_PRODUCTS_AGENT_ID=products-agent-prod
ORCA_BRIDGES_DOC_AGENT_ID=doc-agent-prod

# --- CR-ORCA-005: MCP Proxy peer-invoke ---
# Cần thiết nếu ORCA_BRIDGES_PRODUCTS_AGENT_ID được set.
ORCA_MCP_PROXY_URL=http://mcp-proxy:3012
ORCA_MCP_PROXY_PEER_TOKEN=<peer token from admin>
```

## Acceptance criteria

- [ ] `orca-agent.yaml` có `bridges:` section với 2 keys
- [ ] `orca-agent.yaml` có `mcp_proxy:` section với 2 keys
- [ ] `docker-compose.yml` có đủ 4 env var lines mới
- [ ] `.env.example` tồn tại và có đủ documentation cho tất cả env vars
- [ ] YAML hợp lệ (không có indent error): `python3 -c "import yaml; yaml.safe_load(open('orca-agent.yaml'))"`

## Verify command

```bash
cd deploy/orca-agent
python3 -c "import yaml; print('OK:', list(yaml.safe_load(open('config/orca-agent.yaml')).keys()))"
```
