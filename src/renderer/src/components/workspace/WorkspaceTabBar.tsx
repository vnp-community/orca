// WorkspaceTabBar.tsx — Tab navigation bar for workspace panels
import type { ReactNode } from 'react'
import { GitBranch, CheckSquare, Workflow, Bot } from 'lucide-react'
import { cn } from '../../lib/utils'

export type WorkspaceTab = 'git' | 'tasks' | 'workflows' | 'agent'

interface WorkspaceTabBarProps {
  activeTab:   WorkspaceTab
  onTabChange: (tab: WorkspaceTab) => void
}

const TABS: Array<{ id: WorkspaceTab; label: string; icon: ReactNode }> = [
  { id: 'git',       label: 'Git',       icon: <GitBranch   size={14} /> },
  { id: 'tasks',     label: 'Tasks',     icon: <CheckSquare size={14} /> },
  { id: 'workflows', label: 'Workflows', icon: <Workflow    size={14} /> },
  { id: 'agent',     label: 'Agent',     icon: <Bot         size={14} /> },
]

export function WorkspaceTabBar({ activeTab, onTabChange }: WorkspaceTabBarProps) {
  return (
    <div className="workspace-tab-bar flex border-b bg-muted/30">
      {TABS.map(tab => (
        <button
          key={tab.id}
          onClick={() => onTabChange(tab.id)}
          data-testid={`tab-${tab.id}`}
          className={cn(
            'flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 transition-colors',
            activeTab === tab.id
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          )}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </div>
  )
}
