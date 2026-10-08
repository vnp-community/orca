/**
 * ContractSignatureCell.tsx — FE-CV-TASK-059-04
 *
 * Monospace signature with changed tokens marked. Marking uses a token color plus an underline /
 * line-through so it does not rely on color alone. HTML in the signature renders as text.
 *
 * @module components/review-map/contract/ContractSignatureCell
 */

import type { SignatureToken } from './contract-signature-diff'
import { tc } from './contract-i18n'

const STATE_CLASS: Record<SignatureToken['state'], string> = {
  same: '',
  added: 'bg-muted font-semibold text-[color:var(--git-decoration-added)] underline',
  removed: 'bg-muted text-[color:var(--git-decoration-deleted)] line-through'
}

export function ContractSignatureCell({
  tokens,
  absentLabel
}: {
  tokens: readonly SignatureToken[] | null
  absentLabel?: string
}): React.JSX.Element {
  if (!tokens || tokens.length === 0) {
    return <span className="text-muted-foreground">{absentLabel ?? tc('signature.absent', '(none)')}</span>
  }
  return (
    <code className="font-mono text-[11px] break-all">
      {tokens.map((token, i) => (
        <span key={i} className={STATE_CLASS[token.state]}>
          {token.text}
          {/* Word-like neighbours need a separator; punctuation hugs. */}
          {i < tokens.length - 1 && /\w$/.test(token.text) && /^\w/.test(tokens[i + 1].text) ? ' ' : ''}
        </span>
      ))}
    </code>
  )
}
