/**
 * ContractCompatibilityBadge.tsx — FE-CV-TASK-059-04
 *
 * Maps the backend `compatibility` to icon + text. `unknown` gets its own icon and wording and is
 * never styled like `compatible`; nothing here decides what is breaking.
 *
 * @module components/review-map/contract/ContractCompatibilityBadge
 */

import { AlertTriangle, Check, HelpCircle, ShieldAlert } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { normalizeCompatibility } from './contract-grouping'
import type { CompatibilityState } from './contract-grouping'
import { tc } from './contract-i18n'

export function compatibilityLabel(state: CompatibilityState): string {
  switch (state) {
    case 'breaking':
      return tc('compat.breaking', 'Breaking')
    case 'risky':
      return tc('compat.risky', 'Risky')
    case 'compatible':
      return tc('compat.compatible', 'Compatible')
    default:
      return tc('compat.unknown', 'Not classified')
  }
}

export function ContractCompatibilityBadge({
  compatibility,
  ruleId
}: {
  compatibility: unknown
  ruleId?: string
}): React.JSX.Element {
  const state = normalizeCompatibility(compatibility)
  const label = compatibilityLabel(state)
  const variant = state === 'breaking' ? 'destructive' : state === 'risky' ? 'secondary' : 'outline'
  const Icon =
    state === 'breaking' ? ShieldAlert : state === 'risky' ? AlertTriangle : state === 'compatible' ? Check : HelpCircle
  return (
    <Badge variant={variant} data-compat={state} title={ruleId || undefined}>
      <Icon aria-hidden />
      {label}
    </Badge>
  )
}
