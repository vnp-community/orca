// ProjectSwitcher.tsx — Command-palette style project selector (TDD-FE-12)
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { Check, ChevronsUpDown, Loader2, Plus } from 'lucide-react'
import { Button } from '../ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '../ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '../ui/popover'
import { cn } from '../../lib/utils'

type OrcaProjectListItem = { id: string; name: string; devServerId: string }

export function ProjectSwitcher() {
  const { project, switchProject, isInitializing } = useWorkspace()
  const [projects, setProjects] = useState<OrcaProjectListItem[]>([])
  const [open, setOpen]     = useState(false)
  const [search, setSearch] = useState('')

  // `useAppStore`'s `projects` field is session-grant string[], not OrcaProject[] —
  // fetch the real OrcaProject list from the backend instead of reading that field.
  useEffect(() => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<OrcaProjectListItem[]>(target, 'project.list', null)
      .then(list => setProjects(list ?? []))
      .catch(() => setProjects([]))
  }, [])

  const filtered = projects.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={isInitializing}
          data-testid="project-switcher-trigger"
          className="w-52 justify-between"
        >
          {isInitializing ? (
            <Loader2 className="animate-spin" size={16} />
          ) : (
            <span className="truncate">{project?.name ?? 'Select Project'}</span>
          )}
          <ChevronsUpDown size={14} className="ml-auto opacity-50 shrink-0" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0" align="start">
        <Command>
          <CommandInput
            placeholder="Search projects..."
            value={search}
            onValueChange={setSearch}
          />
          <CommandList>
            <CommandEmpty>No projects found</CommandEmpty>
            <CommandGroup>
              {filtered.map(p => (
                <CommandItem
                  key={p.id}
                  value={p.id}
                  onSelect={() => {
                    switchProject(p.id)
                    setOpen(false)
                  }}
                >
                  <Check
                    className={cn('mr-2 shrink-0', p.id === project?.id ? 'opacity-100' : 'opacity-0')}
                    size={14}
                  />
                  <span className="truncate">{p.name}</span>
                  <span className="ml-auto text-xs text-muted-foreground shrink-0">
                    {p.devServerId}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
            <CommandSeparator />
            <CommandItem data-testid="create-project-item">
              <Plus size={14} className="mr-2" />
              Create New Project
            </CommandItem>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
