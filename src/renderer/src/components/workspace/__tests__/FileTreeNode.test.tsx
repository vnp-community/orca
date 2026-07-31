// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { FileTreeNode } from '../FileTreeNode'
import type { FileNode } from '@shared/workspace-types'

describe('FileTreeNode', () => {
  const mockToggle = vi.fn()
  const mockSelect = vi.fn()
  const mockContextMenu = vi.fn()
  
  const dirNode: FileNode = { type: 'directory', name: 'src', path: 'src', children: [] }
  const fileNode: FileNode = { type: 'file', name: 'index.ts', path: 'src/index.ts', size: 1024 }

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('directory node: shows ChevronRight when collapsed', () => {
    render(<FileTreeNode node={dirNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    const nodeEl = screen.getByTestId('file-node-src')
    // lucide-react chevron-right is rendered as SVG
    expect(nodeEl.innerHTML).toContain('lucide-chevron-right')
  })

  it('directory node: shows ChevronDown when expanded', () => {
    render(<FileTreeNode node={dirNode} depth={0} isExpanded={true} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    const nodeEl = screen.getByTestId('file-node-src')
    expect(nodeEl.innerHTML).toContain('lucide-chevron-down')
  })

  it('file node: shows file icon, no chevron', () => {
    render(<FileTreeNode node={fileNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    const nodeEl = screen.getByTestId('file-node-src/index.ts')
    expect(nodeEl.innerHTML).not.toContain('lucide-chevron')
    expect(nodeEl.innerHTML).toContain('lucide-file')
  })

  it('selected file → has bg-accent class', () => {
    render(<FileTreeNode node={fileNode} depth={0} isExpanded={false} isSelected={true} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    const nodeEl = screen.getByTestId('file-node-src/index.ts')
    expect(nodeEl).toHaveClass('bg-accent')
    expect(nodeEl).not.toHaveClass('hover:bg-accent/50')
  })

  it('unselected file → has hover:bg-accent/50 class', () => {
    render(<FileTreeNode node={fileNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    const nodeEl = screen.getByTestId('file-node-src/index.ts')
    expect(nodeEl).not.toHaveClass('bg-accent')
    expect(nodeEl).toHaveClass('hover:bg-accent/50')
  })

  it('clicking directory calls onToggle', () => {
    render(<FileTreeNode node={dirNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    fireEvent.click(screen.getByTestId('file-node-src'))
    expect(mockToggle).toHaveBeenCalledWith('src')
    expect(mockSelect).not.toHaveBeenCalled()
  })

  it('clicking file calls onSelect', () => {
    render(<FileTreeNode node={fileNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    fireEvent.click(screen.getByTestId('file-node-src/index.ts'))
    expect(mockSelect).toHaveBeenCalledWith('src/index.ts')
    expect(mockToggle).not.toHaveBeenCalled()
  })

  it('right clicking calls onContextMenu', () => {
    render(<FileTreeNode node={fileNode} depth={0} isExpanded={false} isSelected={false} onToggle={mockToggle} onSelect={mockSelect} onContextMenu={mockContextMenu} />)
    fireEvent.contextMenu(screen.getByTestId('file-node-src/index.ts'))
    expect(mockContextMenu).toHaveBeenCalledWith(expect.anything(), fileNode)
  })
})
