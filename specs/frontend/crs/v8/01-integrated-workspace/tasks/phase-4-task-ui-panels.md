# Phase 4 — Task UI Panels

**Phase:** 4/6  
**Priority:** P2  
**Effort:** ~1 ngày  
**Depends on:** Phase 3 (task-enrichment slice, use-task-enrichment hook, task-types)  
**Unlocks:** Phase 5  
**Source solutions:** FE-IW-SOL-002, FE-IW-SOL-005

---

## Mục tiêu Phase

Tạo UI components cho task detail view: KG context panel (hiển thị CRS/Feature data), task form KG fields, và peer-invoke error component. Các task trong phase này có thể chạy song song.

---

## T14 — Tạo `task-kg-context-panel.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/task-kg-context-panel.tsx
Solution:  FE-IW-SOL-002 §SOL-2D
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/tasks/task-kg-context-panel.tsx
// Panel hiển thị KG context trong task detail (CR-ORCA-002)
//
// Rules:
// - Ẩn khi kg_context absent (khớp backend: CRSID == "" → skip enrichment)
// - Best-effort: render dù crsContext/featureContext undefined
// - Convention hint: "// implements: CRS-XXX" (từ BuildSystemPrompt — SOL-ORCA-002 §2.6)
// - CRS status badge từ raw crsContext JSON (json.RawMessage)

import type { FC } from 'react'
import type { Task } from '../../types/task-types'
import { useTaskEnrichment } from '../../hooks/use-task-enrichment'

interface Props {
  task: Task
  environmentId?: string
}

// CRS status values từ backend validation (SOL-ORCA-003 §2.2)
type CRSStatus = 'pending' | 'in-progress' | 'done' | 'cancelled'

const KGStatusBadge: FC<{ status: string }> = ({ status }) => (
  <span
    className={`kg-status-badge kg-status-badge--${status}`}
    title={`Status: ${status}`}
  >
    {status}
  </span>
)

export const TaskKGContextPanel: FC<Props> = ({ task, environmentId }) => {
  const { entry, loading } = useTaskEnrichment(task, environmentId)

  // Ẩn khi không có KG context (backend: skip enrichment nếu CRSID == "")
  if (!task.kg_context?.crs_id && !task.kg_context?.feature_ref) return null

  const kgCtx = task.kg_context
  const crsNode = entry?.crsContext as Record<string, unknown> | undefined
  const featureNode = entry?.featureContext as Record<string, unknown> | undefined

  return (
    <section
      className="task-kg-context-panel"
      aria-label="Knowledge Graph Context"
      id={`task-kg-context-${task.id}`}
    >
      <h4 className="task-kg-context-panel__title">KG Context</h4>

      {loading && (
        <div className="task-kg-context-panel__loading" aria-live="polite">
          Loading KG context…
        </div>
      )}

      {/* CRS node */}
      {kgCtx.crs_id && (
        <div className="task-kg-context-panel__crs">
          <span className="kg-badge kg-badge--crs">CRS</span>
          <code>{kgCtx.crs_id}</code>
          {crsNode?.status && (
            <KGStatusBadge status={crsNode.status as CRSStatus} />
          )}
          {crsNode?.layer && (
            <span className="kg-meta">Layer: {String(crsNode.layer)}</span>
          )}
          {crsNode?.feature_ref && (
            <span className="kg-meta">Feature: {String(crsNode.feature_ref)}</span>
          )}
        </div>
      )}

      {/* Feature node */}
      {kgCtx.feature_ref && (
        <div className="task-kg-context-panel__feature">
          <span className="kg-badge kg-badge--feature">Feature</span>
          <code>{kgCtx.feature_ref}</code>
          {featureNode?.description && (
            <span className="kg-meta">{String(featureNode.description)}</span>
          )}
        </div>
      )}

      {/* Service metadata */}
      {kgCtx.service && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Service:</span>
          <code>{kgCtx.service}</code>
        </div>
      )}

      {/* RepoTarget — dùng cho quality gate convention verify (SOL-ORCA-003) */}
      {kgCtx.repo_target && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Repo:</span>
          <code>{kgCtx.repo_target}</code>
        </div>
      )}

      {/* Convention hint từ BuildSystemPrompt() (SOL-ORCA-002 §2.6) */}
      {kgCtx.crs_id && (
        <div className="task-kg-context-panel__convention-hint">
          <span className="kg-meta-label">Convention:</span>
          <code>// implements: {kgCtx.crs_id}</code>
          <span className="kg-meta"> — required in changed files</span>
        </div>
      )}
    </section>
  )
}
```

**Verify:**
- Render null khi `task.kg_context = undefined`
- Convention hint `// implements: CRS-045` xuất hiện khi `crs_id` có
- `id="task-kg-context-{task.id}"` unique per task

---

## T15 — Tạo `task-form-kg-fields.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/task-form-kg-fields.tsx
Solution:  FE-IW-SOL-002 §SOL-2E
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/tasks/task-form-kg-fields.tsx
// Form fields cho TaskKGContext trong task create/edit (CR-ORCA-002)
//
// Fields submit snake_case JSON để match API (SOL-ORCA-002 §2.4):
//   crs_id, feature_ref, service, repo_target
// Empty string → undefined (không gửi null lên API)

import type { FC } from 'react'
import type { TaskKGContext } from '../../types/task-types'

interface Props {
  value: TaskKGContext
  onChange: (ctx: TaskKGContext) => void
}

export const TaskFormKGFields: FC<Props> = ({ value, onChange }) => (
  <fieldset className="task-form-kg-fields" id="task-form-kg-fields">
    <legend>KG Context <span className="optional">(Optional)</span></legend>

    <div className="form-field">
      <label htmlFor="task-crs-id">CRS ID</label>
      <input
        id="task-crs-id"
        type="text"
        placeholder="CRS-045"
        value={value.crs_id ?? ''}
        onChange={(e) =>
          onChange({ ...value, crs_id: e.target.value || undefined })
        }
      />
      <span className="form-field__hint">
        PIKB CRS implementing this task — enables KG context auto-inject
      </span>
    </div>

    <div className="form-field">
      <label htmlFor="task-feature-ref">Feature Ref</label>
      <input
        id="task-feature-ref"
        type="text"
        placeholder="F27"
        value={value.feature_ref ?? ''}
        onChange={(e) =>
          onChange({ ...value, feature_ref: e.target.value || undefined })
        }
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-service">Target Service</label>
      <input
        id="task-service"
        type="text"
        placeholder="ai-service"
        value={value.service ?? ''}
        onChange={(e) =>
          onChange({ ...value, service: e.target.value || undefined })
        }
      />
    </div>

    <div className="form-field">
      <label htmlFor="task-repo-target">Repo Target</label>
      <input
        id="task-repo-target"
        type="text"
        placeholder="vnp-workspace"
        value={value.repo_target ?? ''}
        onChange={(e) =>
          onChange({ ...value, repo_target: e.target.value || undefined })
        }
      />
      <span className="form-field__hint">
        Used for <code>// implements: CRS-XXX</code> convention verify
      </span>
    </div>
  </fieldset>
)
```

**Verify:**
- 4 inputs, mỗi cái có unique `id`
- Empty string → `undefined` trong onChange (không gửi empty string lên API)

---

## T16 — Tạo `task-peer-invoke-error.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/task-peer-invoke-error.tsx
Solution:  FE-IW-SOL-005 §SOL-5D
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/tasks/task-peer-invoke-error.tsx
// Surface peer-invoke errors trong task context (CR-ORCA-005)
//
// Classify theo exact backend error strings từ ws.Client.Call() (SOL-ORCA-005 §2.2):
//   "peer-invoke not configured: set ORCA_MCP_PROXY_URL"  → not-configured
//   "peer-invoke: no grant for..."                         → auth-error
//   "peer-invoke ... HTTP 503: ..."                        → network-error

import type { FC } from 'react'

interface Props {
  toolName: string
  errorMessage: string
}

type PeerInvokeErrorType = 'not-configured' | 'auth-error' | 'network-error' | 'unknown'

// Classify error message từ SOL-ORCA-005 §2.2 Call() error cases
function classifyError(msg: string): PeerInvokeErrorType {
  if (msg.includes('not configured') || msg.includes('ORCA_MCP_PROXY_URL')) {
    return 'not-configured'
  }
  if (msg.includes('no grant') || msg.includes('HTTP 403') || msg.includes('HTTP 401')) {
    return 'auth-error'
  }
  if (msg.includes('peer-invoke') && /HTTP \d/.test(msg)) {
    return 'network-error'
  }
  return 'unknown'
}

export const TaskPeerInvokeError: FC<Props> = ({ toolName, errorMessage }) => {
  const errorType = classifyError(errorMessage)

  const errorConfig = {
    'not-configured': {
      icon: '⚙️',
      title: 'KG Integration Not Configured',
      fix: (
        <p>
          Set <code>ORCA_MCP_PROXY_URL</code> và{' '}
          <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code> trong orca-agent deploy config,
          sau đó restart orca-agent.
        </p>
      ),
    },
    'auth-error': {
      icon: '🔑',
      title: 'Peer-Invoke Auth Failed',
      fix: (
        <p>
          Kiểm tra <code>ORCA_MCP_PROXY_PEER_TOKEN</code> — có thể expired hoặc thiếu
          grant cho <code>{toolName}</code>.
        </p>
      ),
    },
    'network-error': {
      icon: '🔌',
      title: 'MCP Proxy Unreachable',
      fix: (
        <p>
          Verify mcp-proxy đang chạy tại <code>ORCA_MCP_PROXY_URL</code> và reachable
          từ orca-agent.
        </p>
      ),
    },
    unknown: {
      icon: '❌',
      title: 'KG Query Failed',
      fix: null,
    },
  }[errorType]

  return (
    <div
      className={`task-peer-invoke-error task-peer-invoke-error--${errorType}`}
      role="alert"
      id={`peer-invoke-error-${toolName}`}
    >
      <span className="task-peer-invoke-error__icon" aria-hidden="true">
        {errorConfig.icon}
      </span>
      <div className="task-peer-invoke-error__content">
        <strong>{errorConfig.title}</strong>
        <p>
          Tool <code>{toolName}</code>: {errorMessage}
        </p>
        {errorConfig.fix}
      </div>
    </div>
  )
}
```

**Verify:**
- `classifyError("pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID")` → `"not-configured"` ✓
- `classifyError("peer-invoke: no grant for pis_query_crs")` → `"auth-error"` ✓
- `role="alert"` trên tất cả variants

---

## Checklist Phase 4

- [ ] T14: `task-kg-context-panel.tsx` — null khi no kg_context, convention hint hiển thị
- [ ] T15: `task-form-kg-fields.tsx` — 4 inputs unique IDs, empty→undefined
- [ ] T16: `task-peer-invoke-error.tsx` — 4 error types, role=alert
- [ ] `task-peer-invoke-error`: `classifyError("ORCA_MCP_PROXY_URL")` → "not-configured"
- [ ] TypeScript compile: `npx tsc --noEmit` — 0 errors
