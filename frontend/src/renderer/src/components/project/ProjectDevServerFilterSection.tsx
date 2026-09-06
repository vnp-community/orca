// ProjectDevServerFilterSection.tsx — General tab's dev-server FILTER.
// Phase 10 made OrcaProject stop owning a single dev_server_id (each repo
// carries its own binding now) — this replaced the old single-project
// rebind Select (project.rebindDevServer, now a dead write nothing reads;
// see ProjectDevServerSection.tsx's per-repo branch for the real, still-live
// rebind action). This control's only job is picking which dev server(s)'
// repos show up as candidates in the Repos tab below — pure client-side
// state, no RPC write.
import { useEffect, useState } from 'react'
import { Checkbox } from '../ui/checkbox'
import { Label } from '../ui/label'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { translate } from '@/i18n/i18n'

type DevServerOption = { id: string; name: string; status: string }

type ProjectDevServerFilterSectionProps = {
  selectedDevServerIds: ReadonlySet<string>
  onChange: (next: ReadonlySet<string>) => void
}

export function ProjectDevServerFilterSection({
  selectedDevServerIds,
  onChange
}: ProjectDevServerFilterSectionProps) {
  const [devServers, setDevServers] = useState<DevServerOption[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<DevServerOption[]>(target, 'devServer.list', null)
      .then((list) => setDevServers(Array.isArray(list) ? list : []))
      .catch(() => setDevServers([]))
      .finally(() => setLoading(false))
  }, [])

  const toggle = (id: string): void => {
    const next = new Set(selectedDevServerIds)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    onChange(next)
  }

  return (
    <div className="space-y-2">
      <div className="space-y-1">
        <Label>
          {translate('auto.components.project.ProjectDevServerFilterSection.title', 'Dev servers')}
        </Label>
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.project.ProjectDevServerFilterSection.description',
            "Filter which dev servers' repos show up as candidates in the Repos tab below. Leave none checked to see repos on every dev server."
          )}
        </p>
      </div>
      {loading ? (
        <p className="text-xs text-muted-foreground">
          {translate('auto.components.project.ProjectDevServerFilterSection.loading', 'Loading…')}
        </p>
      ) : devServers.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.project.ProjectDevServerFilterSection.empty',
            'No dev servers available yet.'
          )}
        </p>
      ) : (
        <div className="flex flex-wrap gap-3" data-testid="dev-server-filter">
          {devServers.map((ds) => (
            <label
              key={ds.id}
              htmlFor={`dev-server-filter-${ds.id}`}
              className="flex items-center gap-1.5 text-xs"
            >
              <Checkbox
                id={`dev-server-filter-${ds.id}`}
                data-testid={`dev-server-filter-${ds.id}`}
                checked={selectedDevServerIds.has(ds.id)}
                onCheckedChange={() => toggle(ds.id)}
              />
              {ds.name}
            </label>
          ))}
        </div>
      )}
    </div>
  )
}
