import type {
  McpGrant,
  McpKillSwitchEntry,
  McpOAuthClient,
  McpSessionView
} from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { useMcpQuery } from '@/hooks/useMcpQuery'

type Option = { id: string; label: string }
type Props = { value: string; onChange: (id: string) => void }

function TargetSelect({ options, value, onChange }: Props & { options: Option[] }) {
  return (
    <div className="space-y-1">
      <Label htmlFor="mcp-ks-target">{translate('auto.mcp.killswitch.target', 'Target')}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id="mcp-ks-target" className="w-full">
          <SelectValue
            placeholder={translate('auto.mcp.killswitch.targetPlaceholder', 'Choose…')}
          />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.id} value={o.id}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

function ClientTargets(p: Props): React.JSX.Element {
  const q = useMcpQuery('mcp.admin.client.list', undefined, [] as McpOAuthClient[])
  return <TargetSelect {...p} options={q.data.map((c) => ({ id: c.clientId, label: c.name }))} />
}

function GrantTargets(p: Props): React.JSX.Element {
  const q = useMcpQuery('mcp.admin.grant.list', {}, [] as McpGrant[])
  return (
    <TargetSelect
      {...p}
      options={q.data.map((g) => ({
        id: g.id,
        label: `${g.clientName}${g.userName ? ` · ${g.userName}` : ''}`
      }))}
    />
  )
}

function SessionTargets(p: Props): React.JSX.Element {
  const q = useMcpQuery('mcp.admin.session.list', undefined, [] as McpSessionView[])
  return (
    <TargetSelect
      {...p}
      options={q.data.map((s) => ({
        id: s.id,
        label: `${s.clientName}${s.userName ? ` · ${s.userName}` : ''} · ${s.id.slice(-6)}`
      }))}
    />
  )
}

/** One query per scope: only the chosen scope's list is requested. */
export function McpKillSwitchTargetSelect({
  scope,
  ...rest
}: Props & { scope: Exclude<McpKillSwitchEntry['scope'], 'tenant'> }): React.JSX.Element {
  if (scope === 'client') {
    return <ClientTargets {...rest} />
  }
  return scope === 'grant' ? <GrantTargets {...rest} /> : <SessionTargets {...rest} />
}
