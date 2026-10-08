// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { NodeProps } from '@xyflow/react'

vi.mock('@xyflow/react', () => ({
  Handle: () => null,
  Position: { Left: 'left', Right: 'right' }
}))

import { ErdTableNode } from './ErdTableNode'
import { ErdToolbar, type ErdToolbarProps } from './ErdToolbar'
import { ErdTableList } from './ErdTableList'
import { ErdTableDetail } from './ErdTableDetail'
import { ErdWarningsStrip } from './ErdWarningsStrip'
import { buildErdViewModel } from './erd-view-model'
import { erdColumn, erdTable, sampleErdModel } from './erd-model.fixture'

afterEach(cleanup)

const vm = buildErdViewModel(sampleErdModel(), null)
const devServers = vm.tables[0]

function nodeProps(table = devServers, over: Record<string, unknown> = {}) {
  const onSelect = vi.fn()
  const onToggleExpand = vi.fn()
  const props = {
    data: { table, expanded: false, query: '', dimmed: false, onSelect, onToggleExpand, ...over },
    selected: false
  } as unknown as NodeProps
  return { props, onSelect, onToggleExpand }
}

describe('ErdTableNode', () => {
  it('shows + ~ − symbols and text labels for changed columns without relying on colour', () => {
    render(<ErdTableNode {...nodeProps().props} />)
    expect(screen.getByTestId('erd-col-vault_ssh_role')).toHaveTextContent('+')
    expect(screen.getByTestId('erd-col-status')).toHaveTextContent('~')
    expect(screen.getByTestId('erd-col-status')).toHaveTextContent('varchar(32) → text')
    expect(screen.getByTestId('erd-col-old_flag')).toHaveTextContent('−')
    expect(screen.getByTestId('erd-col-old_flag')).toHaveTextContent('Removed')
    expect(screen.getByRole('group')).toHaveAccessibleName(/dev_servers, 4 columns, Changed/)
  })

  it('Enter selects, and "+N more columns" expands via keyboard activation', () => {
    const big = {
      ...devServers,
      columns: Array.from({ length: 20 }, (_, i) => ({ ...erdColumn(`c${i}`), masked: false }))
    }
    const { props, onSelect, onToggleExpand } = nodeProps(big)
    render(<ErdTableNode {...props} />)
    fireEvent.keyDown(screen.getByRole('group'), { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('infra.dev_servers')
    const more = screen.getByRole('button', { name: '+8 more columns' })
    expect(more).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(more)
    expect(onToggleExpand).toHaveBeenCalledWith('infra.dev_servers')
  })

  it('renders a column default that contained a secret as masked text', () => {
    render(<ErdTableNode {...nodeProps().props} />)
    expect(screen.queryByText(/hunter2/)).toBeNull()
    expect(screen.getByLabelText('Sensitive text was masked')).toBeInTheDocument()
  })

  it('explains a table-level-only highlight and a dropped table', () => {
    const touched = { ...devServers, fromTouchedOnly: true }
    render(<ErdTableNode {...nodeProps(touched).props} />)
    expect(screen.getByText('No column detail available')).toBeInTheDocument()
    cleanup()
    render(<ErdTableNode {...nodeProps({ ...devServers, dropped: true, columns: [] }).props} />)
    expect(screen.getByText('Table dropped by this change')).toBeInTheDocument()
  })
})

describe('ErdToolbar', () => {
  const base: ErdToolbarProps = {
    services: [{ name: 'infra', dialects: ['postgres'], tableCount: 3 }],
    service: 'infra',
    dialect: 'postgres',
    dialects: ['postgres'],
    schemas: ['infra'],
    selectedSchemas: new Set(),
    query: '',
    mode: 'all',
    view: 'graph',
    asOfMigration: '0042',
    disabled: false,
    onService: vi.fn(),
    onDialect: vi.fn(),
    onSchemas: vi.fn(),
    onQuery: vi.fn(),
    onMode: vi.fn(),
    onView: vi.fn()
  }
  it('shows the dialect toggle only when the service has more than one dialect', () => {
    render(<ErdToolbar {...base} />)
    expect(screen.queryByRole('radio', { name: 'mysql' })).toBeNull()
    cleanup()
    render(<ErdToolbar {...base} dialects={['postgres', 'mysql']} />)
    expect(screen.getByRole('radio', { name: 'mysql' })).toBeInTheDocument()
  })
  it('reports the migration, forwards search input and disables while loading', () => {
    const onQuery = vi.fn()
    render(<ErdToolbar {...base} onQuery={onQuery} disabled />)
    expect(screen.getByText('As of migration 0042')).toBeInTheDocument()
    expect(screen.getByLabelText('Search the ERD')).toBeDisabled()
    cleanup()
    render(<ErdToolbar {...base} onQuery={onQuery} />)
    fireEvent.change(screen.getByLabelText('Search the ERD'), { target: { value: 'agents' } })
    expect(onQuery).toHaveBeenCalledWith('agents')
  })
})

describe('ErdTableList', () => {
  it('lists tables with text change symbols and selects on click', () => {
    const onSelect = vi.fn()
    render(
      <ErdTableList
        tables={vm.tables}
        selectedKey="infra.agents"
        dimmed={new Set()}
        onSelect={onSelect}
      />
    )
    expect(screen.getByTestId('erd-list-infra.agents')).toHaveAttribute('aria-current', 'true')
    expect(screen.getByTestId('erd-list-infra.dev_servers')).toHaveTextContent('~')
    fireEvent.click(screen.getByTestId('erd-list-infra.audit'))
    expect(onSelect).toHaveBeenCalledWith('infra.audit')
  })
})

describe('ErdWarningsStrip', () => {
  it('renders nothing without warnings and shows backend text as plain text', () => {
    const { container } = render(<ErdWarningsStrip warnings={[]} degradedTables={[]} />)
    expect(container).toBeEmptyDOMElement()
    cleanup()
    render(
      <ErdWarningsStrip
        warnings={[
          { file: 'm.sql', line: 4, code: 'W1', message: '<script>alert(1)</script> password=abc' }
        ]}
        degradedTables={['audit']}
      />
    )
    expect(document.querySelector('script')).toBeNull()
    expect(screen.getByText(/<script>alert\(1\)<\/script>/)).toBeInTheDocument()
    expect(screen.queryByText(/password=abc/)).toBeNull()
  })
})

describe('ErdTableDetail', () => {
  const agents = vm.tables[1]
  const overlay = {
    changedFiles: [{ path: 'a.go' }],
    changedSymbols: [{ symbol: { key: 'sym:a' } }]
  }
  const base = {
    relations: vm.relations,
    changes: [],
    service: 'infra-fleet',
    overlay,
    onOpenSymbol: vi.fn(),
    onOpenDiff: vi.fn(),
    onOpenService: vi.fn(),
    onClose: vi.fn()
  }

  it('groups accessors, marks changed symbols and enables "View diff" only for changed files', () => {
    const onOpenDiff = vi.fn()
    const onOpenSymbol = vi.fn()
    render(
      <ErdTableDetail
        {...base}
        table={agents}
        onOpenDiff={onOpenDiff}
        onOpenSymbol={onOpenSymbol}
      />
    )
    const diffs = screen.getAllByRole('button', { name: 'View diff' })
    expect(diffs.map((b) => (b as HTMLButtonElement).disabled)).toEqual([true, false])
    fireEvent.click(diffs[1])
    expect(onOpenDiff).toHaveBeenCalledWith('a.go', undefined)
    fireEvent.click(screen.getAllByRole('button', { name: 'Open' })[1])
    expect(onOpenSymbol).toHaveBeenCalledWith('sym:a')
    expect(screen.getByText('changed')).toBeInTheDocument()
    expect(screen.getByText(/Hint from a table-name scan/)).toBeInTheDocument()
  })

  it('warns (as a hint) when changed columns are still used by untouched code, and never says "safe"', () => {
    const risky = { ...devServers, accessedBy: agents.accessedBy }
    render(<ErdTableDetail {...base} table={risky} />)
    expect(screen.getByRole('status')).toHaveTextContent('Hint, may be wrong')
    expect(document.body.textContent).not.toMatch(/\bsafe\b/i)
  })

  it('offers a review note only inside a worktree, attached to the latest migration file', () => {
    const { rerender } = render(<ErdTableDetail {...base} table={agents} />)
    expect(screen.queryByRole('button', { name: 'Note' })).toBeNull()
    rerender(
      <ErdTableDetail
        {...base}
        worktreeId="wt"
        table={{ ...agents, lastMigration: 'db/migrations/002_agents.sql' }}
      />
    )
    expect((screen.getByRole('button', { name: 'Note' }) as HTMLButtonElement).disabled).toBe(false)
    rerender(
      <ErdTableDetail {...base} worktreeId="wt" table={{ ...agents, lastMigration: '', firstMigration: '' }} />
    )
    expect((screen.getByRole('button', { name: 'Note' }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('No file to attach to')).toBeInTheDocument()
  })

  it('does not call an empty accessor list "unused"', () => {
    render(<ErdTableDetail {...base} table={vm.tables[2]} />)
    expect(screen.getByText(/No links to code were found/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/unused|nobody/i)
  })

  it('shows masked defaults only and opens the migration diff', () => {
    const onOpenDiff = vi.fn()
    render(
      <ErdTableDetail
        {...base}
        table={devServers}
        changes={sampleErdModel().changes}
        onOpenDiff={onOpenDiff}
      />
    )
    expect(document.body.textContent).not.toContain('hunter2')
    fireEvent.click(screen.getAllByRole('button', { name: 'View migration diff' })[0])
    expect(onOpenDiff).toHaveBeenCalledWith('m/0042.sql', 3)
  })

  it('filters accessors by write', () => {
    render(<ErdTableDetail {...base} table={agents} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Write' }))
    expect(screen.queryByText('ListAgents')).toBeNull()
    expect(screen.getByText('SaveAgent')).toBeInTheDocument()
  })
})

describe('table fixtures', () => {
  it('keeps the helper honest', () => {
    expect(erdTable('x', [erdColumn('id', { isPk: true })]).pk).toEqual(['id'])
  })
})
