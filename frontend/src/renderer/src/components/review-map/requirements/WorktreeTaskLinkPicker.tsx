/**
 * WorktreeTaskLinkPicker.tsx — FE-CV-TASK-092-04
 *
 * Picker for linking a worktree task to a requirement trace.
 * Uses Command inside Popover for searchable list.
 * No extra dependencies — uses existing ui/command.tsx and ui/popover.tsx.
 *
 * @module components/review-map/requirements/WorktreeTaskLinkPicker
 */

import React, { useState } from 'react'
import { Command, CommandInput, CommandList, CommandItem, CommandEmpty } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Link, Unlink } from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type WorktreeTask = {
  id: string
  /** Short reference like #TG-123 */
  ref: string
  title: string
  url: string
}

export type WorktreeTaskLinkPickerProps = {
  tasks: WorktreeTask[]
  /** Currently linked task URL, if any */
  linkedTaskUrl: string | null
  onLink: (taskUrl: string) => void
  onUnlink: (taskUrl: string) => void
  translate: (key: string, params?: Record<string, unknown>) => string
  disabled?: boolean
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function WorktreeTaskLinkPicker({
  tasks,
  linkedTaskUrl,
  onLink,
  onUnlink,
  translate,
  disabled = false,
}: WorktreeTaskLinkPickerProps): React.ReactElement {
  const [open, setOpen] = useState(false)

  const linkedTask = linkedTaskUrl ? tasks.find((t) => t.url === linkedTaskUrl) : null

  const handleSelect = (task: WorktreeTask) => {
    setOpen(false)
    onLink(task.url)
  }

  const handleUnlink = () => {
    if (linkedTaskUrl) {
      onUnlink(linkedTaskUrl)
    }
  }

  return (
    <div className="flex items-center gap-2">
      {linkedTask ? (
        <>
          <span className="text-xs text-muted-foreground">
            {linkedTask.ref} {linkedTask.title}
          </span>
          <Button
            variant="ghost"
            size="xs"
            onClick={handleUnlink}
            disabled={disabled}
            aria-label={translate('auto.components.reviewMap.requirements.taskPicker.unlink')}
          >
            <Unlink className="size-3.5" aria-hidden />
            {translate('auto.components.reviewMap.requirements.taskPicker.unlink')}
          </Button>
        </>
      ) : (
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              size="xs"
              disabled={disabled}
              aria-label={translate('auto.components.reviewMap.requirements.taskPicker.link')}
            >
              <Link className="size-3.5 mr-1" aria-hidden />
              {translate('auto.components.reviewMap.requirements.taskPicker.link')}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-72 p-0" align="start">
            <Command>
              <CommandInput
                placeholder={translate(
                  'auto.components.reviewMap.requirements.taskPicker.searchPlaceholder'
                )}
                autoFocus
              />
              <CommandList>
                <CommandEmpty>
                  {translate('auto.components.reviewMap.requirements.taskPicker.noResults')}
                </CommandEmpty>
                {tasks.map((task) => (
                  <CommandItem
                    key={task.id}
                    value={`${task.ref} ${task.title}`}
                    onSelect={() => handleSelect(task)}
                  >
                    <span className="text-muted-foreground mr-2 text-xs shrink-0">{task.ref}</span>
                    <span className="truncate">{task.title}</span>
                  </CommandItem>
                ))}
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      )}
    </div>
  )
}
