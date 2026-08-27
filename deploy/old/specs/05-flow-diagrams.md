# Flow Diagrams — Sơ đồ Luồng Chi tiết (v2 — Web-First)

---

## Flow 1: Onboarding Developer — Dùng Browser (Đơn giản nhất)

```mermaid
sequenceDiagram
    actor DevOps as DevOps/Infra
    actor Dev as Developer
    participant Browser as Browser (Chrome/Firefox)
    participant OrcaServer as Orca Server (orca serve)
    participant DevServer as Dev Server (Projects)

    Note over OrcaServer: orca serve --pairing-address wss://orca.vnpblc.internal

    DevOps->>OrcaServer: Lấy pairing URL từ logs
    OrcaServer-->>DevOps: https://orca.vnpblc.internal/web-index.html?pair=orca://pair?code=abc123...
    DevOps->>Dev: Gửi URL qua Slack/Email

    Dev->>Browser: Paste URL → Open
    Browser->>OrcaServer: GET https://orca.vnpblc.internal/web-index.html?pair=...
    OrcaServer-->>Browser: Trả về Orca Web UI bundle (HTML/JS/CSS)
    Browser->>Browser: Load React app + auto-parse pairing param

    Browser->>OrcaServer: WebSocket connect (wss://) + pairing handshake
    Note over Browser,OrcaServer: E2E encrypted channel (TweetNaCl)
    OrcaServer-->>Browser: ✅ Authenticated

    Browser-->>Dev: 🎉 Orca Web UI ready!
    Note over Dev,Browser: Toàn bộ giao diện Orca hiện ra trong browser
```

---

## Flow 2: Onboarding Developer — Dùng Orca Desktop App

```mermaid
sequenceDiagram
    actor DevOps as DevOps/Infra
    actor Dev as Developer
    participant OrcaApp as Orca Desktop App
    participant OrcaServer as Orca Server

    DevOps->>Dev: Gửi Pairing URL: orca://pair?code=abc123...

    Dev->>Dev: Cài Orca Desktop (brew/dmg/exe)
    Dev->>OrcaApp: Mở Orca Desktop
    OrcaApp-->>Dev: Màn hình "Connect to Orca"

    Dev->>OrcaApp: Paste pairing URL → Connect
    OrcaApp->>OrcaServer: WebSocket connect (wss://orca.vnpblc.internal)
    OrcaApp->>OrcaServer: Pairing handshake + token exchange
    OrcaServer-->>OrcaApp: ✅ Runtime connected

    OrcaApp-->>Dev: 🎉 Orca Desktop ready! (full native experience)
    Note over Dev,OrcaApp: Native terminal, keyboard shortcuts, better performance
```

---

## Flow 3: Onboarding Developer — Dùng Orca Mobile (QR)

```mermaid
sequenceDiagram
    actor DevOps as DevOps/Infra
    actor Dev as Developer (Phone)
    participant OrcaMobile as Orca Mobile App
    participant OrcaServer as Orca Server

    DevOps->>OrcaServer: orca serve --mobile-pairing → QR code
    OrcaServer-->>DevOps: QR image + orca://mobile-pair?code=...

    DevOps->>Dev: Gửi QR code qua Slack (hoặc hiện màn hình)

    Dev->>Dev: Cài Orca Mobile (App Store/APK)
    Dev->>OrcaMobile: Mở app → "Add Server" → Scan QR
    OrcaMobile->>OrcaMobile: Parse QR → extract endpoint + mobile token
    OrcaMobile->>OrcaServer: WebSocket connect + mobile handshake
    OrcaServer-->>OrcaMobile: ✅ Mobile connected (limited scope)

    OrcaMobile-->>Dev: 🎉 Paired! Đang theo dõi agents...
    Note over Dev,OrcaMobile: Nhận push notifications khi agent hoàn thành
```

---

## Flow 4: Developer làm việc hàng ngày — Web UI

```mermaid
sequenceDiagram
    actor Dev as Developer (Browser)
    participant Browser as Orca Web UI
    participant OrcaServer as Orca Server (Runtime)
    participant DevServer as Dev Server (Project)
    participant Agent as AI Agent (Claude/Gemini)
    participant Git as Git (on DevServer)

    Dev->>Browser: Mở https://orca.vnpblc.internal (auto-reconnect)
    Browser->>OrcaServer: WebSocket reconnect (stored pairing in localStorage)
    OrcaServer-->>Browser: ✅ Connected

    Dev->>Browser: Click "New Worktree"
    Browser->>OrcaServer: createWorktree({repo: "/srv/projects/vnp-blc", branch: "feature/auth"})
    OrcaServer->>DevServer: SSH → git worktree add .wt/feature-auth -b feature/auth develop
    DevServer-->>OrcaServer: ✅ Worktree created
    OrcaServer->>DevServer: Run orca.yaml setup script
    DevServer-->>OrcaServer: Setup done

    OrcaServer->>Agent: Spawn Claude Code (PTY on DevServer)
    Agent-->>Browser: Streaming output (via WS)

    Dev->>Browser: Type prompt in agent input
    Browser->>OrcaServer: sendToAgent({worktreeId: ..., input: "Implement JWT auth"})
    OrcaServer->>Agent: stdin write
    Agent->>DevServer: Read/write files, run tests
    Agent-->>Browser: Stream output (realtime, via WS relay)

    Dev->>Browser: Click "Diff" tab
    Browser->>OrcaServer: getDiff({worktreeId: ...})
    OrcaServer->>Git: git diff HEAD
    Git-->>Browser: Diff content

    Dev->>Browser: Annotate line: "Fix error handling"
    Browser->>OrcaServer: sendAnnotation({line: 42, comment: "Fix error handling"})
    OrcaServer->>Agent: Follow-up prompt with context
    Agent->>DevServer: Sửa file
    Agent-->>Browser: ✅ Done streaming

    Dev->>Browser: Click "Commit"
    Browser->>OrcaServer: commitChanges({message: "feat: JWT auth"})
    OrcaServer->>Git: git commit + git push
    Git-->>Browser: ✅ Committed and pushed
```

---

## Flow 5: Fan-out — 1 Prompt → N Agents Song Song

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant UI as Orca UI (Web/App)
    participant Server as Orca Server
    participant WT1 as Worktree #1 (Claude)
    participant WT2 as Worktree #2 (Gemini)
    participant WT3 as Worktree #3 (Codex)

    Dev->>UI: Click "Fan Out" → Prompt + 3 agents
    UI->>Server: fanOut({count:3, agents:["claude","gemini","codex"], prompt:"..."})

    par Create 3 worktrees
        Server->>WT1: git worktree add .wt/fanout-1 -b fanout-1 develop
        Server->>WT2: git worktree add .wt/fanout-2 -b fanout-2 develop
        Server->>WT3: git worktree add .wt/fanout-3 -b fanout-3 develop
    end

    par Spawn & run agents
        Server->>WT1: spawn claude-code + send prompt
        Server->>WT2: spawn gemini-cli + send prompt
        Server->>WT3: spawn codex + send prompt
    end

    par Stream all outputs simultaneously
        WT1-->>UI: Panel 1: streaming...
        WT2-->>UI: Panel 2: streaming...
        WT3-->>UI: Panel 3: streaming...
    end

    UI-->>Dev: Compare 3 implementations side-by-side
    Dev->>UI: Select Worktree #2 (best result)
    Dev->>UI: "Merge to develop"
    UI->>Server: mergeWorktree({id: "fanout-2", target: "develop"})
    Server->>WT2: git merge --no-ff fanout-2
    Server->>Server: Delete worktrees #1, #3
    UI-->>Dev: ✅ Merged!
```

---

## Flow 6: Mobile Monitoring và Remote Dispatch

```mermaid
sequenceDiagram
    actor Dev as Developer (Phone)
    participant Mobile as Orca Mobile App
    participant Server as Orca Server
    participant Agent as AI Agent (running on DevServer)

    Note over Server,Agent: Agent đang chạy task dài trên server

    Agent->>Server: Task complete event
    Server->>Mobile: Push notification (WebSocket E2E TweetNaCl)
    Mobile-->>Dev: 🔔 "Claude hoàn thành feature/auth — review diff?"

    Dev->>Mobile: Tap notification
    Mobile->>Server: getWorktreeStatus({id: ...})
    Server-->>Mobile: Status + summary + diff preview
    Mobile-->>Dev: Hiện kết quả tóm tắt

    Dev->>Mobile: Type follow-up: "Add integration tests too"
    Mobile->>Server: dispatch({worktreeId: ..., input: "Add integration tests too"})
    Note over Mobile,Server: E2E encrypted dispatch
    Server->>Agent: stdin: "Add integration tests too"

    Agent->>Server: Running... (stream output)
    Server->>Mobile: Status update
    Mobile-->>Dev: "Agent đang viết tests..."

    Agent->>Server: ✅ Done
    Server->>Mobile: Push: "Tests added, ready to review"
    Mobile-->>Dev: 🔔 "Tests ready!"
```

---

## Flow 7: Orca Architecture Tổng thể (Web-First)

```mermaid
graph TB
    subgraph CLIENTS["Client Access Layer"]
        WEB["🌐 Browser<br/>(Web UI @ HTTPS)"]
        APP["💻 Orca Desktop App<br/>(macOS/Win/Linux)"]
        MOB["📱 Orca Mobile<br/>(iOS/Android)"]
    end

    subgraph NGINX["Nginx Reverse Proxy"]
        NG["Nginx<br/>TLS termination<br/>WebSocket upgrade<br/>Port 443"]
    end

    subgraph ORCA["Orca Server (orca serve)"]
        WS["WebSocket Server<br/>Port 6768"]
        RT["Runtime Engine<br/>(worktrees, agents, PTY)"]
        WEB_BUNDLE["Web UI Bundle<br/>(React SPA)"]
        PAIRING["Pairing System<br/>(token-based auth)"]
    end

    subgraph DEV_SERVERS["Dev Servers (Projects)"]
        DS_A["Dev Server Alpha<br/>Project vnp-blc<br/>SSH from Orca Server"]
        DS_B["Dev Server Beta<br/>Project vnp-ai-ops<br/>SSH from Orca Server"]
    end

    subgraph AGENTS["AI Agents (on Dev Servers)"]
        CLAUDE["Claude Code"]
        GEMINI["Gemini CLI"]
        CODEX["Codex"]
    end

    WEB -->|HTTPS wss://| NG
    APP -->|WSS| NG
    MOB -->|WSS E2E| NG

    NG -->|HTTP/WS proxy| WS
    NG -->|serve static| WEB_BUNDLE

    WS --> RT
    WS --> PAIRING
    RT -->|SSH| DS_A
    RT -->|SSH| DS_B
    DS_A --> CLAUDE
    DS_A --> GEMINI
    DS_B --> CODEX

    style CLIENTS fill:#1e3a5f,color:#fff
    style NGINX fill:#2d5a27,color:#fff
    style ORCA fill:#5a2d82,color:#fff
    style DEV_SERVERS fill:#5a3500,color:#fff
    style AGENTS fill:#5a0028,color:#fff
```

---

## Flow 8: Pairing — Cách token hoạt động

```mermaid
sequenceDiagram
    participant OrcaServer as Orca Server
    participant Nginx as Nginx (HTTPS)
    participant Browser as Browser

    Note over OrcaServer: orca serve --pairing-address wss://orca.vnpblc.internal

    OrcaServer->>OrcaServer: Generate keypair (pubKey, privKey)
    OrcaServer->>OrcaServer: Generate deviceToken (random 32 bytes)
    OrcaServer->>OrcaServer: Build pairing URL:
    Note over OrcaServer: orca://pair?code=TOKEN&endpoint=wss://orca.vnpblc.internal&pk=BASE64_PUBKEY

    Note over OrcaServer: URL in stdout / logs

    Browser->>Nginx: GET /web-index.html?pair=orca://pair?code=TOKEN&...
    Nginx->>OrcaServer: Proxy → serve web bundle
    OrcaServer-->>Browser: Orca Web React SPA

    Browser->>Browser: Parse ?pair= param → extract TOKEN + endpoint + pubKey
    Browser->>Browser: Generate own keypair (browserPub, browserPriv)

    Browser->>Nginx: WebSocket upgrade: wss://orca.vnpblc.internal/ws
    Nginx->>OrcaServer: Proxy WebSocket → ws://127.0.0.1:6768

    Browser->>OrcaServer: Handshake: {deviceToken: TOKEN, clientPub: browserPub}
    OrcaServer->>OrcaServer: Verify TOKEN
    OrcaServer->>OrcaServer: Derive sharedSecret = nacl.box(browserPub, serverPriv)
    Browser->>Browser: Derive sharedSecret = nacl.box(serverPub, browserPriv)

    OrcaServer-->>Browser: ✅ {ok: true, sessionId: ...}
    Note over Browser,OrcaServer: All subsequent messages encrypted with sharedSecret
```
