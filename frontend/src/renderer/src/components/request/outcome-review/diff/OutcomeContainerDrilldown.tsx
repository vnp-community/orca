/**
 * OutcomeContainerDrilldown — FE-OR-T213
 *
 * "Inside containers": the containers whose parts changed, with a way into the code review. It lives outside the drawing on
 * purpose: a drawing in a sandboxed frame cannot be clicked into, and a button drawn inside it would not work.
 *
 * @module components/request/outcome-review/diff/OutcomeContainerDrilldown
 */

import React from 'react'
import { containerItemsText, containersText } from './outcome-diff-copy'

type Props = {
  containers: { id: string; items: number }[]
  /** What goes beside a container: the way into the code review. Absent leaves the list without a button. */
  renderAction?: (containerId: string) => React.ReactNode
}

export function OutcomeContainerDrilldown({
  containers,
  renderAction
}: Props): React.JSX.Element | null {
  if (containers.length === 0) {
    return null
  }
  const text = containersText()
  return (
    <section
      className="flex flex-col gap-1"
      aria-label={text.title}
      data-testid="outcome-container-drilldown"
    >
      <h5 className="text-xs font-medium text-muted-foreground">{text.title}</h5>
      <ul className="flex flex-col gap-1">
        {containers.map((c) => (
          <li
            key={c.id}
            className="flex items-center gap-2 text-sm"
            data-testid="outcome-container"
          >
            <span className="font-mono text-xs">{c.id}</span>
            <span className="text-xs text-muted-foreground">{containerItemsText(c.items)}</span>
            {renderAction && <span className="ml-auto">{renderAction(c.id)}</span>}
          </li>
        ))}
      </ul>
    </section>
  )
}
