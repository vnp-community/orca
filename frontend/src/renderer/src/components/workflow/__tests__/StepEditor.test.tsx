// @vitest-environment happy-dom
// FE-TASK-004 (workflow v4): WorkflowStepType vocabulary fix (notify → notification,
// backend-aligned webhook/condition added, approval kept pending product confirmation).
// No StepEditor.test.tsx existed before this task — created new.
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, afterEach } from 'vitest'
import { StepEditor } from '../StepEditor'
import type { WorkflowStep } from '../../../../../shared/workflow-types'
import type { ReactNode } from 'react'

// Render Select's subtree directly (no Radix portal/open-state to drive in tests) — same
// pattern as ModelSelector.test.tsx.
type TestSelectProps = { value?: string; children?: ReactNode }
type TestSelectItemProps = { value: string; children?: ReactNode }

vi.mock('../../ui/select', () => ({
  Select: (p: TestSelectProps) => (
    <div data-testid="type-select" data-value={p.value}>
      {p.children}
    </div>
  ),
  SelectTrigger: (p: TestSelectProps) => <button>{p.children}</button>,
  SelectValue: () => <span />,
  SelectContent: (p: TestSelectProps) => <div>{p.children}</div>,
  SelectItem: (p: TestSelectItemProps) => (
    <div data-testid={`select-item-${p.value}`}>{p.children}</div>
  )
}))

describe('StepEditor Type dropdown', () => {
  afterEach(cleanup)

  const baseStep: WorkflowStep = {
    id: 's1',
    name: 'Step 1',
    type: 'agent',
    dependsOn: [],
    config: { type: 'agent', prompt: '', worktreePath: '' },
    serverSpec: '',
    continueOnError: false,
    timeout: 0
  }

  it('renders exactly agent, shell, notification, webhook, condition, approval — no stale "notify"', () => {
    render(
      <StepEditor step={baseStep} allSteps={[baseStep]} onUpdate={vi.fn()} onDelete={vi.fn()} />
    )
    for (const type of ['agent', 'shell', 'notification', 'webhook', 'condition', 'approval']) {
      expect(screen.getByTestId(`select-item-${type}`)).toBeInTheDocument()
    }
    expect(screen.queryByTestId('select-item-notify')).not.toBeInTheDocument()
  })

  it("step.type='notification' → Select's value reflects 'notification', not the old 'notify'", () => {
    render(
      <StepEditor
        step={{ ...baseStep, type: 'notification' }}
        allSteps={[baseStep]}
        onUpdate={vi.fn()}
        onDelete={vi.fn()}
      />
    )
    expect(screen.getByTestId('type-select')).toHaveAttribute('data-value', 'notification')
  })
})
