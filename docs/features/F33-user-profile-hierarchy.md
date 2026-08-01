# F33 — User Profile Hierarchy

| Trường | Giá trị |
|--------|---------|
| **ID** | F33 |
| **Tên** | User Profile Hierarchy |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-007 |
| **HLD References** | C3.10, C4.7 |

---

## Mô tả

Trong Web Server Mode, mỗi developer đăng nhập vào hệ thống có **profile riêng** chứa đầy đủ cấu hình như Orca desktop. Profile này kế thừa từ **Department** (phòng ban) và **Company** (công ty) theo mô hình 3 tầng. Settings ở tầng cao hơn là default, tầng thấp hơn có thể override.

---

## Vấn đề cần giải quyết

Trong môi trường enterprise multi-user:
- Mỗi developer cần cấu hình riêng (agent model ưa thích, theme, keybindings)
- Team cần cấu hình chung (shared GitHub org, Linear workspace, SSH defaults)
- Company cần cấu hình global (approved AI models, security policies, audit settings)
- Không có cơ chế inherit giữa các tầng — admin phải config thủ công cho từng user

---

## Tầng Profile (3 cấp)

```
Company (root)
│  name, logo, global defaults
│  AI policy: approved_models, max_token_limit
│  Security: require_2fa, session_timeout_hours
│  Integrations: github_org, linear_workspace
│
└── Department (intermediate)
    │  name, team_lead_id
    │  Override: preferred AI model per team
    │  Fleet access: allowed_server_tags
    │  Shared env: PATH additions, NODE_ENV
    │
    └── User (leaf)
         name, avatar, email (from orca_users)
         Personal override: theme, font_size, keybindings
         Personal credentials: per-user tokens (WebCredentialStore)
         Personal preferences: preferred_agent, default_branch
```

---

## Tính năng chi tiết

### Profile Schema

```typescript
interface OrcaProfile {
  // AI Agent Settings
  agent?: {
    preferredModel?: string        // 'claude-opus-4-5' | 'codex' | ...
    trustPreset?: 'minimal' | 'standard' | 'permissive'
    maxTokensPerSession?: number
    autoApproveFileRead?: boolean
    approvedModels?: string[]      // Company-level whitelist
  }

  // Editor Preferences
  editor?: {
    theme?: 'dark' | 'light' | 'system'
    fontSize?: number              // 12–24
    fontFamily?: string            // 'JetBrains Mono', 'Fira Code'
    tabSize?: number               // 2 | 4
    keybindings?: 'vscode' | 'vim' | 'emacs'
    wordWrap?: boolean
  }

  // Shell Environment
  shell?: {
    defaultShell?: string          // '/bin/bash' | '/bin/zsh' | '/bin/fish'
    pathAdditions?: string[]       // prepend to $PATH on dev server
    envVars?: Record<string, string>  // EDITOR=vim, PAGER=less, ...
    startupCommands?: string[]     // run on PTY session open
  }

  // Integration defaults
  integrations?: {
    githubOrg?: string             // default org for PR creation
    linearWorkspace?: string
    defaultReviewer?: string       // GitHub username
    prTemplate?: string            // PR body template
  }

  // Fleet & Server Access
  fleet?: {
    allowedServerTags?: string[]   // only show servers with these tags
    defaultConnectionType?: 'relay-ssh' | 'relay-websocket' | 'direct-websocket'
    sshKeyPath?: string
  }

  // Security (Company-level only)
  security?: {
    require2FA?: boolean
    sessionTimeoutHours?: number   // default 8
    allowedIpRanges?: string[]     // CIDR blocks
    auditAllActions?: boolean
  }
}
```

### Inheritance Resolution Algorithm

```typescript
function resolveProfile(userId: string): ResolvedProfile {
  const user = getUser(userId)            // layer 3
  const dept = getDepartment(user.departmentId)   // layer 2
  const company = getCompany()            // layer 1 (root)

  // Deep merge: company ← department ← user
  // User values WIN over department WIN over company
  return deepMerge(
    company.profile ?? {},
    dept?.profile ?? {},
    user.profile ?? {}
  )
}

// deepMerge rules:
// - Scalar: last value wins
// - Array: union (no duplicates) — e.g. pathAdditions
// - Object: recursive merge
// - null/undefined: skip (don't override parent value)
// Security fields: company-level ONLY (user cannot override)
```

### Profile Editor UI

**Company Admin:**
- Settings → Company Profile → JSON/form editor
- Fields: approved_models, max_tokens, require_2fa, github_org

**Department Lead:**
- Settings → Department → select dept → edit profile
- Fields: preferred_model, allowed_server_tags, shared_env_vars

**User (self):**
- Settings → My Profile → edit personal overrides
- Fields: theme, fontSize, keybindings, preferred_agent, default_branch
- Read-only display: "Inherited from Department/Company" với effective values

### Effective Profile Panel

```
My Profile — effective settings (merged)
┌──────────────────────────────────────────────┐
│ AI Agent:                                     │
│   Model: claude-opus-4-5  [from: Department]  │
│   Trust: standard         [from: Company]     │
│   Max tokens: 100K        [from: Company]     │
│                                               │
│ Editor:                                       │
│   Theme: dark             [from: User ✏️]     │
│   Font: JetBrains Mono    [from: User ✏️]    │
│   Tab size: 2             [from: Department]  │
│                                               │
│ Shell:                                        │
│   PATH: /usr/local/go/bin [from: Department]  │
│   EDITOR=vim              [from: User ✏️]     │
└──────────────────────────────────────────────┘
```

---

## Luồng người dùng

```
1. Admin tạo Company profile (global defaults)
2. Lead tạo Department + cấu hình team settings
3. Admin tạo User account → assign vào Department
4. User đăng nhập → profile resolved (merge 3 tầng)
5. User vào Settings → My Profile → override personal prefs
6. Khi user tạo agent → effective profile inject vào agent environment
7. Admin thay đổi Company policy → all users inherit ngay lập tức
```

---

## Tiêu chí chấp nhận

- [ ] `OrcaProfile` schema với đầy đủ 6 sections (agent, editor, shell, integrations, fleet, security)
- [ ] `orca_company`, `orca_departments`, `orca_users.profile_json` trong DB schema
- [ ] `resolveProfile(userId)` deep-merge 3 tầng đúng theo inheritance rules
- [ ] Security fields (require2FA, allowedIpRanges) chỉ company có thể set
- [ ] Company Profile Editor UI (admin only)
- [ ] Department Profile Editor UI (lead + admin)
- [ ] User Profile Editor UI — hiển thị inherited values + override UI
- [ ] "Effective Profile" panel hiển thị merged result với source attribution
- [ ] Profile changes propagate ngay lập tức (không cần re-login)
- [ ] Audit log khi admin thay đổi company/department profile

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Profile types | `src/shared/profile-types.ts` |
| Profile resolver | `src/main/profile/profile-resolver.ts` |
| Profile service | `src/main/profile/profile-service.ts` |
| Company service | `src/main/profile/company-service.ts` |
| Department service | `src/main/profile/department-service.ts` |
| DB migration | `src/main/db/migrations/0006_profile_schema.ts` |
| RPC methods | `src/main/runtime/rpc/methods/profile.ts` |
| Company Profile UI | `src/renderer/src/components/profile/CompanyProfileEditor.tsx` |
| Department Profile UI | `src/renderer/src/components/profile/DepartmentProfileEditor.tsx` |
| User Profile UI | `src/renderer/src/components/profile/UserProfileEditor.tsx` |
| Effective Profile Panel | `src/renderer/src/components/profile/EffectiveProfilePanel.tsx` |

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| `resolveProfile()` latency | < 5ms (cached) |
| Profile cache TTL | 60s (refresh on update) |
| Profile JSON size | < 10KB per level |
