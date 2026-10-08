// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { C4Component, C4Relation } from '../../../../../shared/code-intel-architecture-types'
import { C4ComponentDetail, C4_EVIDENCE_PAGE } from './C4ComponentDetail'
import { C4RelationsTable } from './C4RelationsTable'

afterEach(cleanup)
const flags = { changed: false, untested: false, violation: false, affected: false }
const comp = (over: Partial<C4Component> = {}): C4Component => ({
  id: 'x', name: 'X', kind: 'usecase', path: 'p/x', descriptionSource: 'none', symbolCount: 1,
  origin: 'derived', packagePaths: ['p/x'], hidden: false, ...over
})
const evidence = (n: number): C4Relation['evidence'] =>
  Array.from({ length: n }, (_, i) => ({ key: `k${i}`, kind: 'func', name: `f${i}`, filePath: `a${i}.go`, startLine: 3 })) as unknown as C4Relation['evidence']
const rel = (over: Partial<C4Relation> = {}): C4Relation => ({
  from: 'x', to: 'y', kind: 'uses', evidence: [], count: 1, origin: 'derived', confidence: 1, violatesLayering: false, ...over
})
const names = new Map([['x', 'X'], ['y', 'Y']])
const baseProps = { names, relations: [] as C4Relation[], onSelectRelation: vi.fn(), onOpenDiff: vi.fn(), onClose: vi.fn() }

describe('C4ComponentDetail', () => {
  it.each([['derived', 'Inferred'], ['merged', 'Partly edited'], ['declared', 'Declared']] as const)('labels origin %s', (origin, label) => {
    render(<C4ComponentDetail {...baseProps} selection={{ kind: 'component', component: comp({ origin }), flags }} />)
    expect(screen.getByText(label)).toBeInTheDocument()
  })

  it('offers adding a description when there is none', () => {
    const onAdd = vi.fn()
    render(<C4ComponentDetail {...baseProps} onAddDescription={onAdd} selection={{ kind: 'component', component: comp(), flags }} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add description' }))
    expect(onAdd).toHaveBeenCalled()
  })

  it('renders backend text as plain text, never HTML', () => {
    const { container } = render(
      <C4ComponentDetail {...baseProps} selection={{ kind: 'component', component: comp({ name: '<img src=x onerror=alert(1)>', description: '<b>bold</b>', descriptionSource: 'c4.yaml' }), flags }} />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container).toHaveTextContent('<b>bold</b>')
  })

  it('pages evidence at 20 with a show-more and opens the diff', () => {
    const onOpenDiff = vi.fn()
    render(<C4ComponentDetail {...baseProps} onOpenDiff={onOpenDiff} selection={{ kind: 'relation', relation: rel({ evidence: evidence(25) }) }} />)
    expect(screen.getAllByRole('button', { name: 'View diff' })).toHaveLength(C4_EVIDENCE_PAGE)
    fireEvent.click(screen.getByRole('button', { name: /Show more/ }))
    expect(screen.getAllByRole('button', { name: 'View diff' })).toHaveLength(25)
    fireEvent.click(screen.getAllByRole('button', { name: 'View diff' })[0])
    expect(onOpenDiff).toHaveBeenCalledWith('a0.go', 3)
  })

  it('flags a layering violation on a relation', () => {
    render(<C4ComponentDetail {...baseProps} selection={{ kind: 'relation', relation: rel({ violatesLayering: true }) }} />)
    expect(screen.getByRole('status')).toHaveTextContent(/layer boundary/)
  })
})

describe('C4RelationsTable', () => {
  it('shows names, count, flags and the shown/total counter; selects on click', () => {
    const onSelect = vi.fn()
    const r = rel({ count: 7, violatesLayering: true, confidence: 0.5 })
    render(<C4RelationsTable relations={[r]} names={names} total={9} onSelect={onSelect} />)
    expect(screen.getByText('Showing 1/9')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByText('layering')).toBeInTheDocument()
    expect(screen.getByText('low confidence')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Y'))
    expect(onSelect).toHaveBeenCalledWith(r)
  })
})
