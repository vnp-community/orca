// NoProjectSelected.tsx — Empty state when no project is active
import { FolderOpen } from 'lucide-react'

export function NoProjectSelected() {
  return (
    <div
      className="flex flex-col items-center justify-center h-full gap-4 text-muted-foreground"
      data-testid="no-project-selected"
    >
      <FolderOpen size={48} className="opacity-30" />
      <p className="text-lg font-medium">No project selected</p>
      <p className="text-sm opacity-70">Select a project from the switcher above to get started</p>
    </div>
  )
}
