import { useEffect, useRef, useState } from 'react'
import type { McpToken } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { McpCopyButton } from './McpCopyButton'
import { McpTokenCliSnippet } from './McpTokenCliSnippet'

type Props = {
  token: McpToken
  /** Held only by the parent dialog's state; never stored elsewhere. */
  secret: string
  resourceUrl: string
  saved: boolean
  onSavedChange: (saved: boolean) => void
  onDone: () => void
}

export function McpTokenSecretReveal({
  token,
  secret,
  resourceUrl,
  saved,
  onSavedChange,
  onDone
}: Props): React.JSX.Element {
  const [visible, setVisible] = useState(false)
  const [copyFailed, setCopyFailed] = useState(false)
  const heading = useRef<HTMLHeadingElement>(null)
  // Focus the title, not Copy/Done: nothing destructive or irreversible is one keypress away.
  useEffect(() => heading.current?.focus(), [])
  const shown = visible || copyFailed
  return (
    <>
      <DialogHeader>
        <DialogTitle ref={heading} tabIndex={-1} className="outline-none">
          {translate('auto.mcp.tokens.copyNowTitle', 'Copy your token now')}
        </DialogTitle>
        <DialogDescription>
          {translate(
            'auto.mcp.tokens.copyNowBody',
            "This is the only time Orca shows this token. Store it in a secret manager — it can't be recovered, only revoked."
          )}
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-3">
        <div className="flex items-center gap-2 rounded-md border border-border bg-muted p-2">
          <code
            className="min-w-0 flex-1 font-mono text-xs break-all"
            data-testid="mcp-token-secret"
          >
            {shown ? secret : '•'.repeat(32)}
          </code>
          <Button type="button" variant="ghost" size="sm" onClick={() => setVisible((v) => !v)}>
            {visible
              ? translate('auto.mcp.tokens.hide', 'Hide')
              : translate('auto.mcp.tokens.show', 'Show')}
          </Button>
          <McpCopyButton
            text={secret}
            quiet
            ariaLabel={translate('auto.mcp.tokens.copyToken', 'Copy token')}
            onFailed={() => setCopyFailed(true)}
          />
        </div>
        {copyFailed ? (
          <p role="status" aria-live="polite" className="text-xs text-muted-foreground">
            {translate(
              'auto.mcp.tokens.copyFailed',
              "Couldn't access the clipboard. Select the token above and copy it manually."
            )}
          </p>
        ) : null}
        <p className="text-xs text-muted-foreground">
          {translate('auto.mcp.tokens.summary', '{{name}} · expires {{date}}', {
            name: token.name,
            date: new Date(token.expiresAt).toLocaleDateString()
          })}
        </p>
        <McpTokenCliSnippet url={resourceUrl} />
        <div className="flex items-center gap-2">
          <Checkbox
            id="mcp-token-saved"
            checked={saved}
            onCheckedChange={(v) => onSavedChange(v === true)}
          />
          <Label htmlFor="mcp-token-saved" className="text-sm font-normal">
            {translate('auto.mcp.tokens.savedCheck', 'I have saved this token')}
          </Label>
        </div>
      </div>
      <DialogFooter>
        <Button disabled={!saved} onClick={onDone}>
          {translate('auto.mcp.tokens.done', 'Done')}
        </Button>
      </DialogFooter>
    </>
  )
}
