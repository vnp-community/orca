import { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { buildTokenCliSnippet, type McpCliShell } from '@/lib/mcp-connect-snippets'
import { McpCopyButton } from './McpCopyButton'

const defaultShell = (): McpCliShell =>
  typeof navigator !== 'undefined' && navigator.userAgent.includes('Windows')
    ? 'powershell'
    : 'posix'

export function McpTokenCliSnippet({ url }: { url: string }): React.JSX.Element {
  const [shell, setShell] = useState<McpCliShell>(defaultShell)
  const code = buildTokenCliSnippet(shell, url)
  return (
    <div className="space-y-2">
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={shell}
        aria-label={translate('auto.mcp.tokens.shellAria', 'Shell')}
        onValueChange={(v) => v && setShell(v as McpCliShell)}
      >
        <ToggleGroupItem value="posix">macOS / Linux</ToggleGroupItem>
        <ToggleGroupItem value="powershell">PowerShell</ToggleGroupItem>
      </ToggleGroup>
      <div className="relative rounded-md border border-border bg-muted">
        <pre className="overflow-x-auto p-3 pr-24 font-mono text-xs">
          <code>{code}</code>
        </pre>
        <div className="absolute top-1 right-1">
          <McpCopyButton
            text={code}
            ariaLabel={translate('auto.mcp.connect.copySnippet', 'Copy snippet')}
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.mcp.tokens.cliCaption',
          'Any MCP client that supports HTTP transport and a custom Authorization header works the same way.'
        )}
      </p>
    </div>
  )
}
