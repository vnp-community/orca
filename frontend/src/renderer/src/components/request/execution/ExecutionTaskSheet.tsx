/**
 * ExecutionTaskSheet — FE-REQ-TASK-037-05
 *
 * Opens TaskDetail (full log, result, rerun) in a Sheet. TaskDetail reads the active task from the store, so the task is seeded
 * there first, as PlanTree does.
 *
 * @module components/request/execution/ExecutionTaskSheet
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { useAppStore } from '../../../store'
import { TaskDetail } from '../../task/TaskDetail'
import type { OrcaTask } from '../../../../../shared/task-types'

const T = 'auto.components.request.execution.ExecutionTaskSheet.'

export function useExecutionTaskSheet(): {
  open: boolean
  openTask: (task: OrcaTask) => void
  onOpenChange: (open: boolean) => void
} {
  const setActiveTask = useAppStore((s) => s.setActiveTask)
  const [open, setOpen] = React.useState(false)
  return {
    open,
    openTask: (task) => {
      const state = useAppStore.getState()
      if (!state.tasks.some((t) => t.id === task.id)) {
        state.addTask(task)
      }
      setActiveTask(task.id)
      setOpen(true)
    },
    onOpenChange: (next) => {
      setOpen(next)
      if (!next) {
        setActiveTask(null)
      }
    }
  }
}

export function ExecutionTaskSheet({
  open,
  onOpenChange
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}): React.JSX.Element {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[520px] max-w-full overflow-auto sm:max-w-[520px]">
        <SheetHeader>
          <SheetTitle>{translate(`${T}title`, 'Task')}</SheetTitle>
          <SheetDescription className="sr-only">
            {translate(`${T}description`, 'Task details')}
          </SheetDescription>
        </SheetHeader>
        {open && <TaskDetail />}
      </SheetContent>
    </Sheet>
  )
}
