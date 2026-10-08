/**
 * PlanTree — CR-REQ-021-03
 *
 * Plan → Phase → Task, read-only. Task rows open TaskDetail in a Sheet; TaskDetail
 * reads the active task from the store, so the task is seeded there first.
 *
 * @module components/request/plan/PlanTree
 */

import React, { useState } from 'react'
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
import { PhaseNode } from './PhaseNode'
import { PlanTaskRow } from './PlanTaskRow'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { Approval } from '../../../../../shared/request-types'
import type { OrcaTask } from '../../../../../shared/task-types'
import type { ApprovalMap } from './plan-approval-model'

type Props = {
  tree: PlanSubtree
  approvals: ApprovalMap
  /** Renders the approve / start actions of one phase. */
  renderPhaseActions?: (phase: OrcaTask, approval: Approval | null) => React.ReactNode
}

export function PlanTree({ tree, approvals, renderPhaseActions }: Props): React.JSX.Element {
  const setActiveTask = useAppStore((s) => s.setActiveTask)
  const [sheetOpen, setSheetOpen] = useState(false)

  const openTask = (task: OrcaTask): void => {
    const state = useAppStore.getState()
    if (!state.tasks.some((t) => t.id === task.id)) {
      state.addTask(task)
    }
    setActiveTask(task.id)
    setSheetOpen(true)
  }
  const onSheetChange = (open: boolean): void => {
    setSheetOpen(open)
    if (!open) {
      setActiveTask(null)
    }
  }

  return (
    <div className="flex flex-col gap-2 p-3" data-testid="plan-tree">
      {tree.phases.map((phase) => {
        const approval = approvals.byPhaseId[phase.id] ?? null
        return (
          <PhaseNode
            key={phase.id}
            phase={phase}
            tasks={tree.tasksByPhase[phase.id] ?? []}
            approval={approval}
            onOpenTask={openTask}
            actions={renderPhaseActions?.(phase, approval)}
          />
        )
      })}
      {tree.flatTasks.length > 0 && (
        <div className="rounded-md border border-border p-1" data-testid="plan-flat-tasks">
          {tree.flatTasks.map((t) => (
            <PlanTaskRow key={t.id} task={t} onOpen={openTask} />
          ))}
        </div>
      )}
      <Sheet open={sheetOpen} onOpenChange={onSheetChange}>
        <SheetContent className="w-[520px] max-w-full overflow-auto sm:max-w-[520px]">
          <SheetHeader>
            <SheetTitle>
              {translate('auto.components.request.plan.PlanTree.taskSheetTitle', 'Task')}
            </SheetTitle>
            <SheetDescription className="sr-only">
              {translate(
                'auto.components.request.plan.PlanTree.taskSheetDescription',
                'Task details'
              )}
            </SheetDescription>
          </SheetHeader>
          {sheetOpen && <TaskDetail />}
        </SheetContent>
      </Sheet>
    </div>
  )
}
