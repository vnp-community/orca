/**
 * WorktreeTaskLinkPicker.tsx — FE-CV-TASK-092-04
 *
 * Searchable task picker (Popover + Command) for linking the worktree to a task.
 * Tasks come from `useTasks(projectId)`; selecting calls `onLink(task.id)`, removal
 * calls `onUnlink()` (the contract clears a link with an empty taskId).
 *
 * @module components/review-map/requirements/WorktreeTaskLinkPicker
 */

import { useState } from 'react'
import { Link2, Unlink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import { useTasks } from '../../../hooks/useTasks'

const BASE = 'auto.components.reviewMap.requirements.taskPicker'

export type PickerTask = { id: string; title: string; taskNumber?: number }

export type WorktreeTaskLinkPickerViewProps = {
  tasks: PickerTask[]
  linkedTaskId: string | null
  disabled?: boolean
  onLink: (taskId: string) => void
  onUnlink: () => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

export function taskPickerLabel(task: PickerTask): string {
  return task.taskNumber != null ? `#TG-${task.taskNumber} ${task.title}` : task.title
}

export function WorktreeTaskLinkPickerView({
  tasks,
  linkedTaskId,
  disabled = false,
  onLink,
  onUnlink,
  translate = translateCatalogKey
}: WorktreeTaskLinkPickerViewProps): React.JSX.Element {
  const [open, setOpen] = useState(false)
  return (
    <div className="flex items-center gap-1.5">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button type="button" variant="outline" size="xs" disabled={disabled}>
            <Link2 aria-hidden />
            {translate(linkedTaskId ? `${BASE}.change` : `${BASE}.link`)}
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-72 p-0">
          <Command>
            <CommandInput placeholder={translate(`${BASE}.searchPlaceholder`)} />
            <CommandList>
              <CommandEmpty>{translate(`${BASE}.noResults`)}</CommandEmpty>
              {tasks.map((task) => (
                <CommandItem
                  key={task.id}
                  value={`${taskPickerLabel(task)} ${task.id}`}
                  onSelect={() => {
                    setOpen(false)
                    onLink(task.id)
                  }}
                >
                  <span className="truncate">{taskPickerLabel(task)}</span>
                </CommandItem>
              ))}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      {linkedTaskId ? (
        <Button type="button" variant="ghost" size="xs" disabled={disabled} onClick={onUnlink}>
          <Unlink aria-hidden />
          {translate(`${BASE}.unlink`)}
        </Button>
      ) : null}
    </div>
  )
}

/** Loads the project's tasks, so it is only mounted when the picker is actually shown. */
export function WorktreeTaskLinkPicker(
  props: Omit<WorktreeTaskLinkPickerViewProps, 'tasks'> & { projectId: string }
): React.JSX.Element {
  const { filteredTasks } = useTasks(props.projectId)
  return <WorktreeTaskLinkPickerView {...props} tasks={filteredTasks} />
}
