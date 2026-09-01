// ProjectSwitcher.tsx — Command-palette style project selector (TDD-FE-12)
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { useWorkspace } from '../../context/WorkspaceContext'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { Check, ChevronsUpDown, Loader2, Plus, Settings } from 'lucide-react'
import { Button } from '../ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator
} from '../ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '../ui/popover'
import { cn } from '../../lib/utils'
import { CreateProjectDialog } from './CreateProjectDialog'
import { ProjectSettings } from './ProjectSettings'
import type { OrcaProject } from '../../types/workspace-types'

// project.list's wscompat handler returns the full Project proto message —
// this is a narrower view of the same OrcaProject shape for the picker list.
type OrcaProjectListItem = Pick<OrcaProject, 'id' | 'name' | 'devServerId'>

export function ProjectSwitcher() {
  const { project, switchProject, isInitializing } = useWorkspace()
  const [projects, setProjects] = useState<OrcaProjectListItem[]>([])
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

  const refetchProjects = () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    return callRuntimeRpc<OrcaProjectListItem[]>(target, 'project.list', null)
      .then((list) => setProjects(list ?? []))
      .catch(() => setProjects([]))
  }

  // `useAppStore`'s `projects` field is session-grant string[], not OrcaProject[] —
  // fetch the real OrcaProject list from the backend instead of reading that field.
  useEffect(() => {
    void refetchProjects()
  }, [])

  const filtered = projects.filter((p) => p.name.toLowerCase().includes(search.toLowerCase()))

  return (
    <div className="flex items-center gap-1">
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
                {filtered.map((p) => (
                  <CommandItem
                    key={p.id}
                    value={p.id}
                    onSelect={() => {
                      switchProject(p.id)
                      setOpen(false)
                    }}
                  >
                    <Check
                      className={cn(
                        'mr-2 shrink-0',
                        p.id === project?.id ? 'opacity-100' : 'opacity-0'
                      )}
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
              <CommandItem
                data-testid="create-project-item"
                onSelect={() => {
                  setOpen(false)
                  setCreateOpen(true)
                }}
              >
                <Plus size={14} className="mr-2" />
                Create New Project
              </CommandItem>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      <Button
        variant="ghost"
        size="icon"
        disabled={!project}
        onClick={() => setSettingsOpen(true)}
        data-testid="project-settings-trigger"
        aria-label="Project settings"
      >
        <Settings size={14} />
      </Button>
      <CreateProjectDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={(created) => {
          void refetchProjects()
          // Why: the project row is already created at this point — a
          // failure here is only "couldn't switch to it yet", not "project
          // creation failed" — so surface it as a toast instead of letting
          // it fall out as a silent unhandled promise rejection (the
          // reported symptom before this catch existed).
          switchProject(created.id).catch(() => {
            toast.error('Project created, but could not switch to it. Select it from the list.')
          })
        }}
      />
      {/* Why built-but-never-mounted before: ProjectSettings/MemberManager
          existed fully wired to real RPCs, but nothing in the app ever
          rendered a trigger to open them. */}
      {project ? (
        <ProjectSettings
          projectId={project.id}
          open={settingsOpen}
          onClose={() => setSettingsOpen(false)}
        />
      ) : null}
    </div>
  )
}
