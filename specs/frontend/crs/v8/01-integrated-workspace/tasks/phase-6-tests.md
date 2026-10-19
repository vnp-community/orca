# Phase 6 — Tests

**Phase:** 6/6  
**Priority:** P1 (quality gate)  
**Effort:** ~2–3 ngày  
**Depends on:** Phase 1–5 hoàn thành  
**Source solutions:** FE-IW-SOL-001 đến FE-IW-SOL-005 (§Tests sections)

---

## Mục tiêu Phase

Viết Vitest unit tests cho tất cả hooks, slices, selectors và components đã tạo ở Phase 1–5. Mỗi task tương ứng 1 test file. Các task có thể chạy song song.

---

## T19a — Tests cho Phase 1: MCP Proxy

### `use-mcp-proxy-status.test.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/__tests__/use-mcp-proxy-status.test.ts
Solution:  FE-IW-SOL-005 §Tests
```

```typescript
// src/renderer/src/hooks/__tests__/use-mcp-proxy-status.test.ts
import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useMCPProxyStatus } from '../use-mcp-proxy-status'
import * as rpcClient from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client')
const mockCallRpc = vi.mocked(rpcClient.callRuntimeRpc)

describe('useMCPProxyStatus', () => {
  beforeEach(() => vi.clearAllMocks())

  it('initial status là unconfigured', () => {
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    expect(result.current.statusInfo.status).toBe('unconfigured')
  })

  it('status=unconfigured khi configured=false', async () => {
    mockCallRpc.mockResolvedValueOnce({ configured: false })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unconfigured')
  })

  it('status=unreachable khi reachable=false', async () => {
    mockCallRpc.mockResolvedValueOnce({ configured: true, url: 'http://proxy:3012', reachable: false, error: 'connection refused' })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unreachable')
    expect(result.current.statusInfo.url).toBe('http://proxy:3012')
  })

  it('status=auth-error khi authOk=false', async () => {
    mockCallRpc.mockResolvedValueOnce({ configured: true, url: 'http://proxy:3012', reachable: true, authOk: false })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('auth-error')
  })

  it('status=connected khi reachable=true && authOk=true', async () => {
    mockCallRpc.mockResolvedValueOnce({ configured: true, url: 'http://proxy:3012', reachable: true, authOk: true })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('connected')
    expect(result.current.statusInfo.latencyMs).toBeGreaterThanOrEqual(0)
  })

  it('status=checking trong khi call đang chạy', async () => {
    let resolve: (v: unknown) => void
    mockCallRpc.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    act(() => { result.current.checkStatus() })
    expect(result.current.statusInfo.status).toBe('checking')
    await act(() => { resolve!({ configured: false }) })
  })

  it('status=unreachable khi callRuntimeRpc throw', async () => {
    mockCallRpc.mockRejectedValueOnce(new Error('network error'))
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unreachable')
  })
})
```

### `mcp-proxy-status-indicator.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/__tests__/mcp-proxy-status-indicator.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect } from 'vitest'
import { MCPProxyStatusIndicator } from '../mcp-proxy-status-indicator'

describe('MCPProxyStatusIndicator', () => {
  const defaultProps = {
    environmentId: 'env-1',
    onRecheck: vi.fn(),
  }

  it('render 🟢 và aria-label "MCP Proxy Connected" khi connected', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'connected', url: 'http://proxy:3012', latencyMs: 50 }} />)
    expect(screen.getByLabelText('MCP Proxy Connected')).toBeTruthy()
    expect(screen.getByText('50ms')).toBeTruthy()
  })

  it('render endpoint URL hint khi url present', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'connected', url: 'http://proxy:3012' }} />)
    expect(screen.getByText(/\/internal\/v1\/agent-peer\/mcp\/proxy/)).toBeTruthy()
  })

  it('không render recheck button khi unconfigured', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'unconfigured' }} />)
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('recheck button disabled khi checking', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'checking' }} />)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('hint "ORCA_MCP_PROXY_URL" khi unconfigured', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'unconfigured' }} />)
    expect(screen.getByText(/ORCA_MCP_PROXY_URL/)).toBeTruthy()
  })

  it('auth error hint "ORCA_MCP_PROXY_PEER_TOKEN"', () => {
    render(<MCPProxyStatusIndicator {...defaultProps} statusInfo={{ status: 'auth-error', error: 'no grant' }} />)
    expect(screen.getByText(/ORCA_MCP_PROXY_PEER_TOKEN/)).toBeTruthy()
  })
})
```

---

## T19b — Tests cho Phase 2: PIS Hook & Components

### `use-agent-tool-capabilities.test.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/__tests__/use-agent-tool-capabilities.test.ts
```

```typescript
import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect } from 'vitest'
import { useAgentToolCapabilities, PIS_TOOL_NAMES } from '../use-agent-tool-capabilities'
import * as rpcClient from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client')
const mockCallRpc = vi.mocked(rpcClient.callRuntimeRpc)

describe('useAgentToolCapabilities', () => {
  it('hasPisTools=true khi catalog chứa pis_query_crs', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [{ name: 'pis_query_crs', description: 'query crs' }] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(true)
  })

  it('hasPisTools=false khi catalog không chứa pis_* tools', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [{ name: 'orca.create_task', description: '...' }] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(false)
  })

  it('hasPisTools=false khi catalog rỗng (ProductsAgentID chưa set)', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [] })
    const { result } = renderHook(() => useAgentToolCapabilities('env-1'))
    await act(() => result.current.fetchCapabilities())
    expect(result.current.hasPisTools).toBe(false)
  })

  it('PIS_TOOL_NAMES có đúng 5 tools', () => {
    expect(PIS_TOOL_NAMES).toHaveLength(5)
    expect(PIS_TOOL_NAMES).toContain('pis_query_crs')
    expect(PIS_TOOL_NAMES).toContain('pis_kg_search')
  })

  it('loading=false sau khi fetchCapabilities hoàn thành', async () => {
    mockCallRpc.mockResolvedValueOnce({ tools: [] })
    const { result } = renderHook(() => useAgentToolCapabilities())
    await act(() => result.current.fetchCapabilities())
    expect(result.current.loading).toBe(false)
  })
})
```

### `pis-tool-not-configured-error.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/agent/__tests__/pis-tool-not-configured-error.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { PISTooLNotConfiguredError } from '../pis-tool-not-configured-error'

describe('PISTooLNotConfiguredError', () => {
  it('render error khi errorMessage chứa "ORCA_BRIDGES_PRODUCTS_AGENT_ID"', () => {
    render(<PISTooLNotConfiguredError toolName="pis_query_crs" errorMessage="pis tools not configured — set ORCA_BRIDGES_PRODUCTS_AGENT_ID" />)
    expect(screen.getByRole('alert')).toBeTruthy()
    expect(screen.getByText(/KG Integration Not Configured/)).toBeTruthy()
  })

  it('return null khi error không liên quan đến config', () => {
    const { container } = render(<PISTooLNotConfiguredError toolName="pis_query_crs" errorMessage="network timeout" />)
    expect(container.firstChild).toBeNull()
  })

  it('có role=alert khi render', () => {
    render(<PISTooLNotConfiguredError toolName="pis_query_crs" errorMessage="ORCA_BRIDGES_PRODUCTS_AGENT_ID not set" />)
    expect(screen.getByRole('alert')).toBeTruthy()
  })
})
```

### `agent-pis-health-check.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/__tests__/agent-pis-health-check.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect } from 'vitest'
import { AgentPISHealthCheck } from '../agent-pis-health-check'
import { PIS_TOOL_NAMES } from '../../../hooks/use-agent-tool-capabilities'

describe('AgentPISHealthCheck', () => {
  it('render đúng 5 tools sau khi run theo thứ tự PIS_TOOL_NAMES', async () => {
    const mockCheck = vi.fn().mockResolvedValue(
      PIS_TOOL_NAMES.map((name) => ({ name, status: 'ok' as const }))
    )
    render(<AgentPISHealthCheck environmentId="env-1" onRunCheck={mockCheck} />)
    await userEvent.click(screen.getByRole('button', { name: /Run Check/i }))
    PIS_TOOL_NAMES.forEach((name) => {
      expect(screen.getByText(name)).toBeTruthy()
    })
  })

  it('✅ icon khi status=ok', async () => {
    const mockCheck = vi.fn().mockResolvedValue([{ name: 'pis_query_crs', status: 'ok' }])
    render(<AgentPISHealthCheck environmentId="env-1" onRunCheck={mockCheck} />)
    await userEvent.click(screen.getByRole('button'))
    // ✅ icon present
    expect(document.querySelector('.pis-tool-check-item--ok')).toBeTruthy()
  })

  it('❌ icon khi status=missing', async () => {
    const mockCheck = vi.fn().mockResolvedValue([{ name: 'pis_query_crs', status: 'missing', error: 'not found' }])
    render(<AgentPISHealthCheck environmentId="env-1" onRunCheck={mockCheck} />)
    await userEvent.click(screen.getByRole('button'))
    expect(document.querySelector('.pis-tool-check-item--missing')).toBeTruthy()
  })

  it('Run button disabled khi running=true', async () => {
    let resolve: (v: unknown) => void
    const mockCheck = vi.fn().mockReturnValue(new Promise((r) => { resolve = r }))
    render(<AgentPISHealthCheck environmentId="env-1" onRunCheck={mockCheck} />)
    await userEvent.click(screen.getByRole('button'))
    expect(screen.getByRole('button')).toBeDisabled()
    resolve!([])
  })
})
```

---

## T19c — Tests cho Phase 3: Store & Hook

### `task-enrichment.test.ts`

```
Action:    NEW
File:      src/renderer/src/store/slices/__tests__/task-enrichment.test.ts
```

```typescript
import { describe, it, expect } from 'vitest'
import { createStore } from 'zustand'
import { createTaskEnrichmentSlice } from '../task-enrichment'

function makeStore() {
  return createStore(createTaskEnrichmentSlice)
}

describe('TaskEnrichmentSlice', () => {
  it('setTaskEnrichmentLoading: loading=true', () => {
    const store = makeStore()
    store.getState().setTaskEnrichmentLoading('T-1')
    expect(store.getState().taskEnrichments['T-1'].loading).toBe(true)
  })

  it('setTaskEnrichment: lưu crsContext, featureContext, loading=false', () => {
    const store = makeStore()
    const crs = { id: 'CRS-045', status: 'pending' }
    store.getState().setTaskEnrichment('T-1', crs, undefined)
    const entry = store.getState().taskEnrichments['T-1']
    expect(entry.crsContext).toEqual(crs)
    expect(entry.loading).toBe(false)
    expect(entry.fetchedAt).toBeGreaterThan(0)
  })

  it('setTaskEnrichmentError: error set, loading=false', () => {
    const store = makeStore()
    store.getState().setTaskEnrichmentError('T-1', 'fetch failed')
    const entry = store.getState().taskEnrichments['T-1']
    expect(entry.error).toBe('fetch failed')
    expect(entry.loading).toBe(false)
  })

  it('updateEnrichmentCRSStatus: patch status trong crsContext', () => {
    const store = makeStore()
    store.getState().setTaskEnrichment('T-1', { id: 'CRS-045', status: 'pending' }, undefined)
    store.getState().updateEnrichmentCRSStatus('T-1', 'done')
    const crs = store.getState().taskEnrichments['T-1'].crsContext as Record<string, unknown>
    expect(crs.status).toBe('done')
  })

  it('updateEnrichmentCRSStatus: no-op khi task không có crsContext', () => {
    const store = makeStore()
    store.getState().updateEnrichmentCRSStatus('T-nonexistent', 'done')
    expect(store.getState().taskEnrichments['T-nonexistent']).toBeUndefined()
  })

  it('updateEnrichmentCRSStatus: không mutate tasks khác', () => {
    const store = makeStore()
    store.getState().setTaskEnrichment('T-1', { status: 'pending' }, undefined)
    store.getState().setTaskEnrichment('T-2', { status: 'pending' }, undefined)
    store.getState().updateEnrichmentCRSStatus('T-1', 'done')
    const t2 = store.getState().taskEnrichments['T-2'].crsContext as Record<string, unknown>
    expect(t2.status).toBe('pending')
  })
})
```

### `use-task-enrichment.test.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/__tests__/use-task-enrichment.test.ts
```

```typescript
import { renderHook, act } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { useTaskEnrichment } from '../use-task-enrichment'
import type { Task } from '../../types/task-types'

vi.mock('../../runtime/runtime-rpc-client')
vi.mock('../../store', () => ({
  useAppStore: vi.fn(() => ({
    taskEnrichments: {},
    setTaskEnrichmentLoading: vi.fn(),
    setTaskEnrichment: vi.fn(),
    setTaskEnrichmentError: vi.fn(),
  })),
}))

import * as rpcClient from '../../runtime/runtime-rpc-client'
const mockCallRpc = vi.mocked(rpcClient.callRuntimeRpc)

const taskWithCRS: Task = {
  id: 'T-1',
  status: 'pending',
  kg_context: { crs_id: 'CRS-045', feature_ref: 'F27' },
} as Task

const taskNoCRS: Task = { id: 'T-2', status: 'pending' } as Task

describe('useTaskEnrichment', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockCallRpc.mockResolvedValue({ id: 'CRS-045', status: 'pending' })
  })

  it('không fetch khi task không có kg_context (backend: CRSID == "" → skip)', () => {
    renderHook(() => useTaskEnrichment(taskNoCRS))
    expect(mockCallRpc).not.toHaveBeenCalled()
  })

  it('fetch khi task có crs_id', async () => {
    await act(async () => {
      renderHook(() => useTaskEnrichment(taskWithCRS, 'env-1'))
    })
    expect(mockCallRpc).toHaveBeenCalledWith(
      expect.anything(),
      'agent.tool_call',
      expect.objectContaining({ tool: 'pis_query_crs' })
    )
  })

  it('concurrent fetch: gọi cả pis_query_crs và pis_query_feature_context', async () => {
    await act(async () => {
      renderHook(() => useTaskEnrichment(taskWithCRS, 'env-1'))
    })
    const calls = mockCallRpc.mock.calls
    const tools = calls.map((c) => (c[2] as Record<string, unknown>).tool)
    expect(tools).toContain('pis_query_crs')
    expect(tools).toContain('pis_query_feature_context')
  })
})
```

---

## T19d — Tests cho Phase 4 & 5: Components

### `task-kg-context-panel.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/__tests__/task-kg-context-panel.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { TaskKGContextPanel } from '../task-kg-context-panel'
import type { Task } from '../../../types/task-types'

vi.mock('../../../hooks/use-task-enrichment', () => ({
  useTaskEnrichment: vi.fn(() => ({
    entry: { crsContext: { id: 'CRS-045', status: 'done', layer: 'backend' }, fetchedAt: Date.now() },
    loading: false,
  })),
}))

describe('TaskKGContextPanel', () => {
  it('render null khi task không có kg_context', () => {
    const task = { id: 'T-1', status: 'pending' } as Task
    const { container } = render(<TaskKGContextPanel task={task} />)
    expect(container.firstChild).toBeNull()
  })

  it('hiển thị CRS ID và status badge', () => {
    const task = { id: 'T-1', status: 'pending', kg_context: { crs_id: 'CRS-045' } } as Task
    render(<TaskKGContextPanel task={task} />)
    expect(screen.getByText('CRS-045')).toBeTruthy()
    expect(screen.getByText('done')).toBeTruthy()  // status badge
  })

  it('hiển thị convention hint: // implements: CRS-045', () => {
    const task = { id: 'T-1', status: 'pending', kg_context: { crs_id: 'CRS-045' } } as Task
    render(<TaskKGContextPanel task={task} />)
    expect(screen.getByText(/\/\/ implements: CRS-045/)).toBeTruthy()
  })

  it('loading state với aria-live=polite', () => {
    vi.mocked(require('../../../hooks/use-task-enrichment').useTaskEnrichment).mockReturnValueOnce({
      entry: undefined,
      loading: true,
    })
    const task = { id: 'T-1', status: 'pending', kg_context: { crs_id: 'CRS-045' } } as Task
    render(<TaskKGContextPanel task={task} />)
    const loadingEl = screen.getByText(/Loading KG context/i)
    expect(loadingEl.getAttribute('aria-live')).toBe('polite')
  })
})
```

### `task-rejection-banner.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/__tests__/task-rejection-banner.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { TaskRejectionBanner } from '../task-rejection-banner'
import type { Task } from '../../../types/task-types'

const rejectedTask: Task = {
  id: 'T-100',
  status: 'rejected',
  rejection: {
    task_id: 'T-100',
    status: 'rejected',
    reason: "convention check failed: 2 files missing '// implements: CRS-045', 0 invalid",
    action_required: "Add '// implements: CRS-045' comment to each changed file",
  },
} as Task

describe('TaskRejectionBanner', () => {
  it('render null khi task.status !== "rejected"', () => {
    const task = { id: 'T-1', status: 'done' } as Task
    const { container } = render(<TaskRejectionBanner task={task} />)
    expect(container.firstChild).toBeNull()
  })

  it('render null khi task.rejection absent', () => {
    const task = { id: 'T-1', status: 'rejected' } as Task
    const { container } = render(<TaskRejectionBanner task={task} />)
    expect(container.firstChild).toBeNull()
  })

  it('parse convention error — "2 files missing // implements: CRS-045"', () => {
    render(<TaskRejectionBanner task={rejectedTask} />)
    expect(screen.getByText(/2 file/)).toBeTruthy()
    expect(screen.getByText(/\/\/ implements: CRS-045/)).toBeTruthy()
  })

  it('hiển thị action_required với code formatting', () => {
    render(<TaskRejectionBanner task={rejectedTask} />)
    expect(screen.getByText(/Add '\/\/ implements: CRS-045'/)).toBeTruthy()
  })

  it('role="alert"', () => {
    render(<TaskRejectionBanner task={rejectedTask} />)
    expect(screen.getByRole('alert')).toBeTruthy()
  })

  it('id="task-rejection-T-100"', () => {
    render(<TaskRejectionBanner task={rejectedTask} />)
    expect(document.getElementById('task-rejection-T-100')).toBeTruthy()
  })
})
```

### `task-peer-invoke-error.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/__tests__/task-peer-invoke-error.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { TaskPeerInvokeError } from '../task-peer-invoke-error'

describe('TaskPeerInvokeError', () => {
  it('not-configured: detect "ORCA_MCP_PROXY_URL" trong error', () => {
    render(<TaskPeerInvokeError toolName="pis_query_crs" errorMessage="peer-invoke not configured: set ORCA_MCP_PROXY_URL" />)
    expect(screen.getByText(/KG Integration Not Configured/)).toBeTruthy()
    expect(screen.getByText(/ORCA_MCP_PROXY_URL/)).toBeTruthy()
  })

  it('auth-error: detect "no grant" trong error', () => {
    render(<TaskPeerInvokeError toolName="pis_query_crs" errorMessage="peer-invoke: no grant for pis_query_crs on products-agent" />)
    expect(screen.getByText(/Peer-Invoke Auth Failed/)).toBeTruthy()
    expect(screen.getByText(/ORCA_MCP_PROXY_PEER_TOKEN/)).toBeTruthy()
  })

  it('network-error: detect "HTTP 503"', () => {
    render(<TaskPeerInvokeError toolName="pis_query_crs" errorMessage="peer-invoke pis_query_crs HTTP 503: service unavailable" />)
    expect(screen.getByText(/MCP Proxy Unreachable/)).toBeTruthy()
  })

  it('role=alert trên tất cả variants', () => {
    render(<TaskPeerInvokeError toolName="pis_query_crs" errorMessage="peer-invoke not configured: set ORCA_MCP_PROXY_URL" />)
    expect(screen.getByRole('alert')).toBeTruthy()
  })

  it('id="peer-invoke-error-{toolName}"', () => {
    render(<TaskPeerInvokeError toolName="pis_query_crs" errorMessage="network error" />)
    expect(document.getElementById('peer-invoke-error-pis_query_crs')).toBeTruthy()
  })
})
```

### `onboarding-pikb-bridge-step.test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/onboarding/__tests__/pikb-bridge-step.test.tsx
```

```typescript
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect } from 'vitest'
import { PIKBBridgeStep } from '../pikb-bridge-step'

describe('PIKBBridgeStep', () => {
  it('Continue button disabled khi productsAgentId empty', () => {
    render(<PIKBBridgeStep onSkip={vi.fn()} onComplete={vi.fn()} />)
    expect(screen.getByRole('button', { name: /Configure/i })).toBeDisabled()
  })

  it('Continue enabled khi productsAgentId và mcpProxyUrl có giá trị', async () => {
    render(<PIKBBridgeStep onSkip={vi.fn()} onComplete={vi.fn()} />)
    await userEvent.type(screen.getByLabelText(/Products Agent ID/i), 'products-agent-prod')
    // mcpProxyUrl đã có default 'http://mcp-proxy:3012'
    expect(screen.getByRole('button', { name: /Configure/i })).toBeEnabled()
  })

  it('gọi onSkip khi click Skip', async () => {
    const onSkip = vi.fn()
    render(<PIKBBridgeStep onSkip={onSkip} onComplete={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /Skip/i }))
    expect(onSkip).toHaveBeenCalledOnce()
  })

  it('gọi onComplete với config khi click Configure', async () => {
    const onComplete = vi.fn()
    render(<PIKBBridgeStep onSkip={vi.fn()} onComplete={onComplete} />)
    await userEvent.type(screen.getByLabelText(/Products Agent ID/i), 'products-agent-prod')
    await userEvent.click(screen.getByRole('button', { name: /Configure/i }))
    expect(onComplete).toHaveBeenCalledWith(
      expect.objectContaining({ productsAgentId: 'products-agent-prod' })
    )
  })

  it('peer token field có type=password', () => {
    render(<PIKBBridgeStep onSkip={vi.fn()} onComplete={vi.fn()} />)
    expect(screen.getByLabelText(/Peer Token/i).getAttribute('type')).toBe('password')
  })

  it('4 inputs có unique IDs', () => {
    render(<PIKBBridgeStep onSkip={vi.fn()} onComplete={vi.fn()} />)
    const ids = [
      'onboarding-products-agent-id',
      'onboarding-doc-agent-id',
      'onboarding-mcp-proxy-url',
      'onboarding-peer-token',
    ]
    ids.forEach((id) => {
      expect(document.getElementById(id)).toBeTruthy()
    })
  })
})
```

---

## Checklist Phase 6

- [ ] T19a: `use-mcp-proxy-status.test.ts` — 7 test cases pass
- [ ] T19a: `mcp-proxy-status-indicator.test.tsx` — 6 test cases pass
- [ ] T19b: `use-agent-tool-capabilities.test.ts` — 5 test cases pass
- [ ] T19b: `pis-tool-not-configured-error.test.tsx` — 3 test cases pass
- [ ] T19b: `agent-pis-health-check.test.tsx` — 4 test cases pass
- [ ] T19c: `task-enrichment.test.ts` — 6 test cases pass
- [ ] T19c: `use-task-enrichment.test.ts` — 3 test cases pass
- [ ] T19d: `task-kg-context-panel.test.tsx` — 4 test cases pass
- [ ] T19d: `task-rejection-banner.test.tsx` — 6 test cases pass
- [ ] T19d: `task-peer-invoke-error.test.tsx` — 5 test cases pass
- [ ] T19d: `pikb-bridge-step.test.tsx` — 6 test cases pass
- [ ] `pnpm test` (hoặc `npx vitest run`) — 0 failures
