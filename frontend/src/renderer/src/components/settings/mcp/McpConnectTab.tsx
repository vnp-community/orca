import { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { buildMcpSnippet, isInsecureMcpUrl, type McpClientKind } from '@/lib/mcp-connect-snippets'
import { selectMcpKillSwitchActive } from '@/store/slices/mcp-slice'
import { McpCopyButton } from './McpCopyButton'
import { McpSessionsTable } from './McpSessionsTable'
import { McpAgentTerminalsList } from './McpAgentTerminalsList'
import { MCP_TABS } from './mcp-tab-registry'

const CLIENTS: { kind: McpClientKind; label: string }[] = [
  { kind: 'claude-code', label: 'Claude Code' },
  { kind: 'claude-desktop', label: 'Claude Desktop' },
  { kind: 'cursor', label: 'Cursor' }
]

function SnippetPanel({ kind, url }: { kind: McpClientKind; url: string }): React.JSX.Element {
  const [mode, setMode] = useState<'oauth' | 'token'>('oauth')
  const openTab = useAppStore((s) => s.openMcpTab)
  const snippet = buildMcpSnippet(kind, { resourceUrl: url })
  const code = mode === 'oauth' ? snippet.withOAuth : snippet.withToken
  const hasTokensTab = MCP_TABS.some((t) => t.id === 'tokens')
  return (
    <div className="space-y-3">
      <ToggleGroup
        type="single"
        variant="outline"
        value={mode}
        aria-label={translate('auto.mcp.connect.modeAria', 'Authentication method')}
        onValueChange={(v) => v && setMode(v as 'oauth' | 'token')}
      >
        <ToggleGroupItem value="oauth">
          {translate('auto.mcp.connect.oauthMode', 'Sign in with browser (OAuth)')}
        </ToggleGroupItem>
        <ToggleGroupItem value="token">
          {translate('auto.mcp.connect.tokenMode', 'Use an access token')}
        </ToggleGroupItem>
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
      {snippet.notes.includes('claudeDesktopConnectorsHint') ? (
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.mcp.connect.claudeDesktopConnectorsHint',
            'Newer Claude Desktop versions can add a remote server from Settings → Connectors instead.'
          )}
        </p>
      ) : null}
      {mode === 'token' && hasTokensTab ? (
        <Button variant="link" size="sm" className="px-0" onClick={() => openTab('tokens')}>
          {translate('auto.mcp.connect.createToken', 'Create an access token')}
        </Button>
      ) : null}
    </div>
  )
}

export function McpConnectTab(): React.JSX.Element {
  const info = useAppStore((s) => s.mcpServerInfo)
  const killSwitch = useAppStore(selectMcpKillSwitchActive)
  const url = info?.resourceUrl ?? ''
  return (
    <div className="space-y-6">
      <section className="space-y-2">
        <Label htmlFor="mcp-server-address">
          {translate('auto.mcp.connect.serverAddress', 'Server address')}
        </Label>
        <div className="flex items-center gap-2">
          <Input id="mcp-server-address" readOnly value={url} className="font-mono text-xs" />
          <McpCopyButton
            text={url}
            ariaLabel={translate('auto.mcp.connect.copyAddress', 'Copy server address')}
          />
        </div>
        {url && isInsecureMcpUrl(url) ? (
          <p className="text-xs text-destructive">
            {translate(
              'auto.mcp.connect.insecureUrl',
              "This address isn't HTTPS. Most agents refuse insecure remote MCP servers."
            )}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {(info?.protocolVersions ?? []).map((v) => (
            <Badge key={v} variant="outline" className="font-mono">
              {v}
            </Badge>
          ))}
          {info?.authorizationServer ? (
            <span>
              {translate('auto.mcp.connect.authServer', 'Authorization server: {{value}}', {
                value: info.authorizationServer
              })}
            </span>
          ) : null}
        </div>
        {killSwitch ? (
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.mcp.connect.killSwitchNote',
              'Agents are currently blocked by an administrator.'
            )}
          </p>
        ) : null}
      </section>
      <section className="space-y-3">
        <h3 className="text-sm font-medium">
          {translate('auto.mcp.connect.connectAgent', 'Connect an agent')}
        </h3>
        <Tabs defaultValue="claude-code">
          <TabsList>
            {CLIENTS.map((c) => (
              <TabsTrigger key={c.kind} value={c.kind}>
                {c.label}
              </TabsTrigger>
            ))}
          </TabsList>
          {CLIENTS.map((c) => (
            <TabsContent key={c.kind} value={c.kind}>
              <SnippetPanel kind={c.kind} url={url} />
            </TabsContent>
          ))}
        </Tabs>
      </section>
      <McpSessionsTable />
      <McpAgentTerminalsList />
    </div>
  )
}
