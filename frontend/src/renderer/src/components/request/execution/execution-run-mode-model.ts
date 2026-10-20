/**
 * execution-run-mode-model.ts — FE-REQ-TASK-043-04
 *
 * Which run-mode label the Execution tab shows. Step by step is only worth saying when the flow has steps to ask about;
 * a mode the summary does not carry (not pinned yet, or an older gateway) shows nothing.
 *
 * @module components/request/execution/execution-run-mode-model
 */

export type RunModeLabel = 'continuous' | 'step_by_step'

export function runModeLabelOf(
  mode: 'continuous' | 'step_by_step' | undefined,
  hasPhases: boolean | undefined
): RunModeLabel | null {
  if (mode === 'continuous') {
    return 'continuous'
  }
  return mode === 'step_by_step' && hasPhases === true ? 'step_by_step' : null
}
