/** ErdLegend.tsx — FE-CV-TASK-057-05. Symbol and line legend; mirrors what the canvas draws. */

import { translate } from '@/i18n/i18n'
import { ERD_CHANGE_SYMBOL } from './erd-column-changes'
import { ERD_CHANGE_TEXT_CLASS } from './erd-change-style'

export function ErdLegend(): React.JSX.Element {
  const items = [
    { kind: 'added', text: translate('auto.components.reviewMap.ErdLegend.added', 'added') },
    {
      kind: 'modified',
      text: translate('auto.components.reviewMap.ErdLegend.modified', 'changed')
    },
    { kind: 'removed', text: translate('auto.components.reviewMap.ErdLegend.removed', 'removed') }
  ] as const
  return (
    <ul
      className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground"
      aria-label={translate('auto.components.reviewMap.ErdLegend.label', 'Legend')}
    >
      {items.map((i) => (
        <li key={i.kind} className={ERD_CHANGE_TEXT_CLASS[i.kind]}>
          <span className="font-mono" aria-hidden="true">
            {ERD_CHANGE_SYMBOL[i.kind]}
          </span>{' '}
          {i.text}
        </li>
      ))}
      <li>
        <span aria-hidden="true">━</span>{' '}
        {translate('auto.components.reviewMap.ErdLegend.foreignKey', 'foreign key')}
      </li>
      <li>
        <span aria-hidden="true">┄</span>{' '}
        {translate('auto.components.reviewMap.ErdLegend.logicalLink', 'logical link')}
      </li>
    </ul>
  )
}
