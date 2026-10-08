/**
 * BacklogTaskSheet — CR-REQ-023-06
 *
 * Opens TaskDetail (which reads activeTaskId from the store) in a Sheet and
 * puts the previous selection back on close.
 *
 * @module components/request/backlog/BacklogTaskSheet
 */

import React, { useEffect, useRef } from 'react'
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { TaskDetail } from '../../task/TaskDetail'

export function BacklogTaskSheet({
  taskId,
  onClose
}: {
  taskId: string | null
  onClose: () => void
}): React.JSX.Element {
  const previous = useRef<string | null>(null)

  useEffect(() => {
    if (!taskId) {return}
    const store = useAppStore.getState()
    previous.current = store.activeTaskId
    store.setActiveTask(taskId)
    return () => {
      useAppStore.getState().setActiveTask(previous.current)
    }
  }, [taskId])

  return (
    <Sheet open={taskId !== null} onOpenChange={(next) => { if (!next) {onClose()} }}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-2xl">
        <SheetTitle className="sr-only">{translate('auto.components.request.backlog.BacklogTaskSheet.title', 'Task details')}</SheetTitle>
        <SheetDescription className="sr-only">
          {translate('auto.components.request.backlog.BacklogTaskSheet.description', 'Details of the selected task')}
        </SheetDescription>
        {taskId && <TaskDetail />}
      </SheetContent>
    </Sheet>
  )
}
