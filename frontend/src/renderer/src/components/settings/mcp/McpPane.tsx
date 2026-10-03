import { Suspense, lazy, useEffect, useMemo, useState, type ComponentType } from 'react'
import { useAppStore } from '@/store'
import {
  selectMcpEnabled,
  selectMcpKillSwitchActive,
  type McpTabId
} from '@/store/slices/mcp-slice'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { mcpUiStage } from '@/lib/mcp-labels'
import { trackMcpSettingsOpened } from '@/lib/mcp-telemetry'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { McpPaneSkeleton } from './McpPaneSkeleton'
import {
  MCP_ADMIN_ENABLE_CARD,
  getVisibleMcpTabs,
  resolveMcpTab,
  type McpTabDefinition
} from './mcp-tab-registry'

// Why: lazy components must be stable across renders or tabs remount and refetch.
const lazyCache = new Map<() => Promise<{ default: ComponentType }>, ComponentType>()
function lazyOnce(load: () => Promise<{ default: ComponentType }>): ComponentType {
  let c = lazyCache.get(load)
  if (!c) {
    c = lazy(load)
    lazyCache.set(load, c)
  }
  return c
}

function Notice({ children }: { children: React.ReactNode }): React.JSX.Element {
  return <p className="text-sm text-muted-foreground">{children}</p>
}

function AdminSetup(): React.JSX.Element {
  const load = MCP_ADMIN_ENABLE_CARD.load
  const Card = load ? lazyOnce(load) : null
  return (
    <div className="space-y-3">
      <p className="text-sm font-medium">
        {translate('auto.mcp.pane.disabledTitle', 'MCP is turned off for your organization')}
      </p>
      {Card ? (
        <Suspense fallback={<McpPaneSkeleton />}>
          <Card />
        </Suspense>
      ) : (
        <Notice>
          {translate('auto.mcp.pane.disabledAdminHint', 'Ask an administrator to turn MCP on.')}
        </Notice>
      )}
    </div>
  )
}

export function McpPane(): React.JSX.Element {
  const info = useAppStore((s) => s.mcpServerInfo)
  const status = useAppStore((s) => s.mcpServerInfoStatus)
  const error = useAppStore((s) => s.mcpServerInfoError)
  const enabled = useAppStore(selectMcpEnabled)
  const killSwitchActive = useAppStore(selectMcpKillSwitchActive)
  const adminSetup = useAppStore((s) => s.mcpAdminSetupAvailable)
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  const navigation = useAppStore((s) => s.mcpNavigation)
  const refresh = useAppStore((s) => s.refreshMcpServerInfo)
  const clearNavigation = useAppStore((s) => s.clearMcpNavigation)

  const visibleTabs: McpTabDefinition[] = useMemo(
    () => (info && enabled ? getVisibleMcpTabs(info, isAdmin) : []),
    [info, enabled, isAdmin]
  )
  const [requestedTab, setRequestedTab] = useState<McpTabId | undefined>(undefined)
  const activeTab = resolveMcpTab(requestedTab, visibleTabs)

  // Deep link: consumed once.
  useEffect(() => {
    if (!navigation) {
      return
    }
    setRequestedTab(navigation.tab)
    clearNavigation()
  }, [navigation, clearNavigation])

  // Why: coarse tab-view signal only (enum + role); fires on tab change, not on re-render.
  const trackedRole = isAdmin ? 'admin' : 'user'
  useEffect(() => {
    if (activeTab) {
      trackMcpSettingsOpened({ tab: activeTab, role: trackedRole })
    }
  }, [activeTab, trackedRole])

  if (!info && (status === 'idle' || status === 'loading')) {
    return <McpPaneSkeleton />
  }
  if (!enabled) {
    return adminSetup && isAdmin ? (
      <AdminSetup />
    ) : (
      <Notice>{translate('auto.mcp.pane.unsupported', 'MCP is not available.')}</Notice>
    )
  }

  return (
    <div className="space-y-4">
      {mcpUiStage() === 'beta' ? (
        <Badge variant="secondary">{translate('auto.mcp.nav.badge', 'Beta')}</Badge>
      ) : null}
      {killSwitchActive ? (
        <div
          role="status"
          className="rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
        >
          {translate('auto.mcp.killswitch.banner', 'MCP access is paused by an administrator.')}
          {info?.killSwitch.reason ? ` ${info.killSwitch.reason}` : ''}
        </div>
      ) : null}
      {status === 'error' ? (
        <div role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{error || translate('auto.mcp.pane.error', "Couldn't refresh MCP status")}</span>
          <Button variant="ghost" size="sm" onClick={() => void refresh()}>
            {translate('auto.mcp.pane.retry', 'Retry')}
          </Button>
        </div>
      ) : null}
      {activeTab === null ? (
        <Notice>{translate('auto.mcp.pane.noTabs', 'Nothing to show here yet.')}</Notice>
      ) : (
        <Tabs value={activeTab} onValueChange={(v) => setRequestedTab(v as McpTabId)}>
          <TabsList>
            {visibleTabs.map((t) => (
              <TabsTrigger key={t.id} value={t.id}>
                {translate(t.titleKey, t.titleDefault)}
              </TabsTrigger>
            ))}
          </TabsList>
          {visibleTabs.map((t) => {
            const Content = lazyOnce(t.load)
            return (
              <TabsContent key={t.id} value={t.id}>
                {t.id === activeTab ? (
                  <Suspense fallback={<McpPaneSkeleton />}>
                    <Content />
                  </Suspense>
                ) : null}
              </TabsContent>
            )
          })}
        </Tabs>
      )}
    </div>
  )
}
