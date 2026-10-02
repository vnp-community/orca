import { useState } from 'react'
import type {
  McpDecision,
  McpOAuthClient,
  McpRisk,
  McpToolPolicy,
  McpToolView
} from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { McpInlineAlert } from './McpListStates'
import { MCP_POLICY_NOTE_MAX, validatePolicyDraft, type McpPolicyDraft } from './mcp-policy-form'
import { policyIssueText } from './mcp-policy-issue-text'
import { useMcpPolicyEditor } from './use-mcp-policy-editor'

const ANY = '__any__'
const RISKS: McpRisk[] = ['read', 'write_reversible', 'exec', 'destructive', 'admin']

function decisionLabel(d: McpDecision): string {
  if (d === 'allow') {
    return translate('auto.mcp.policies.decision.allow', 'Allow')
  }
  return d === 'deny'
    ? translate('auto.mcp.policies.decision.deny', 'Deny')
    : translate('auto.mcp.policies.decision.require_approval', 'Require approval')
}
export { decisionLabel as mcpDecisionLabel }

type Props = {
  policy: McpToolPolicy | null
  tools: McpToolView[]
  clients: McpOAuthClient[]
  onClose: () => void
  onSaved: (p: McpToolPolicy) => void
  onGone: () => void
}

export function McpPolicyEditorDialog({
  policy,
  tools,
  clients,
  onClose,
  onSaved,
  onGone
}: Props): React.JSX.Element {
  const [draft, setDraft] = useState<McpPolicyDraft>(() => ({
    ...(policy ? { id: policy.id, version: policy.version } : {}),
    match: { ...policy?.match },
    decision: policy?.decision ?? 'require_approval',
    note: policy?.note ?? ''
  }))
  const [yourDraft, setYourDraft] = useState<string | null>(null)
  const editor = useMcpPolicyEditor(onSaved)
  const { errors, warnings, matched } = validatePolicyDraft(draft, tools)
  const { failure } = editor
  const setMatch = (m: Partial<McpPolicyDraft['match']>): void =>
    setDraft({ ...draft, match: { ...draft.match, ...m } })
  const namespaces = [...new Set(tools.map((t) => t.namespace))].sort()
  const roles = draft.match.roles ?? []
  const toggleRole = (r: 'admin' | 'user', on: boolean): void =>
    setMatch({ roles: on ? [...new Set([...roles, r])] : roles.filter((x) => x !== r) })
  const denied = matched.filter((t) => t.hardDenied).length

  const reloadLatest = (): void => {
    if (failure?.kind !== 'conflict') {
      return
    }
    setYourDraft(
      JSON.stringify({ match: draft.match, decision: draft.decision, note: draft.note }, null, 2)
    )
    setDraft({
      id: failure.latest.id,
      version: failure.latest.version,
      match: { ...failure.latest.match },
      decision: failure.latest.decision,
      note: failure.latest.note ?? ''
    })
    editor.clearFailure()
  }
  const overwrite = (): void => {
    if (failure?.kind === 'conflict') {
      void editor.save({ ...draft, version: failure.latest.version })
    }
  }

  return (
    <Dialog open onOpenChange={(o) => (!o && !editor.saving ? onClose() : undefined)}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {policy
              ? translate('auto.mcp.policies.editTitle', 'Edit rule')
              : translate('auto.mcp.policies.addTitle', 'Add rule')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.mcp.policies.editorDesc',
              'Choose what the rule matches, then what happens. Leave a field on Any to ignore it.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1 sm:col-span-2">
            <Label htmlFor="mcp-pol-tool">{translate('auto.mcp.policies.tool', 'Tool')}</Label>
            <Input
              id="mcp-pol-tool"
              list="mcp-pol-tools"
              className="font-mono"
              value={draft.match.tool ?? ''}
              placeholder={translate('auto.mcp.policies.any', 'Any')}
              onChange={(e) => setMatch({ tool: e.target.value })}
            />
            <datalist id="mcp-pol-tools">
              {tools.map((t) => (
                <option key={t.name} value={t.name}>
                  {t.hardDenied
                    ? translate('auto.mcp.policies.permDenied', 'Permanently denied')
                    : t.title}
                </option>
              ))}
            </datalist>
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-pol-ns">
              {translate('auto.mcp.policies.namespace', 'Namespace')}
            </Label>
            <Select
              value={draft.match.namespace ?? ANY}
              onValueChange={(v) => setMatch({ namespace: v === ANY ? undefined : v })}
            >
              <SelectTrigger id="mcp-pol-ns" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>{translate('auto.mcp.policies.any', 'Any')}</SelectItem>
                {namespaces.map((n) => (
                  <SelectItem key={n} value={n}>
                    {n}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-pol-risk">{translate('auto.mcp.policies.risk', 'Risk')}</Label>
            <Select
              value={draft.match.risk ?? ANY}
              onValueChange={(v) => setMatch({ risk: v === ANY ? undefined : (v as McpRisk) })}
            >
              <SelectTrigger id="mcp-pol-risk" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>{translate('auto.mcp.policies.any', 'Any')}</SelectItem>
                {RISKS.map((r) => (
                  <SelectItem key={r} value={r}>
                    {mcpRiskLabel(r)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="mcp-pol-client">
              {translate('auto.mcp.policies.client', 'Client')}
            </Label>
            <Select
              value={draft.match.clientId ?? ANY}
              onValueChange={(v) => setMatch({ clientId: v === ANY ? undefined : v })}
            >
              <SelectTrigger id="mcp-pol-client" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>{translate('auto.mcp.policies.any', 'Any')}</SelectItem>
                {clients.map((c) => (
                  <SelectItem key={c.clientId} value={c.clientId}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <fieldset className="space-y-1">
            <legend className="text-sm font-medium">
              {translate('auto.mcp.policies.roles', 'Roles')}
            </legend>
            {(['admin', 'user'] as const).map((r) => (
              <div key={r} className="flex items-center gap-2">
                <Checkbox
                  id={`mcp-pol-role-${r}`}
                  checked={roles.includes(r)}
                  onCheckedChange={(c) => toggleRole(r, c === true)}
                />
                <Label htmlFor={`mcp-pol-role-${r}`}>
                  {r === 'admin'
                    ? translate('auto.mcp.policies.role.admin', 'Admins')
                    : translate('auto.mcp.policies.role.user', 'Users')}
                </Label>
              </div>
            ))}
          </fieldset>
        </div>
        <div className="space-y-1">
          <Label id="mcp-pol-decision-label">
            {translate('auto.mcp.policies.decisionLabel', 'Decision')}
          </Label>
          <ToggleGroup
            type="single"
            variant="outline"
            aria-labelledby="mcp-pol-decision-label"
            value={draft.decision}
            onValueChange={(v) => v && setDraft({ ...draft, decision: v as McpDecision })}
          >
            {(['allow', 'require_approval', 'deny'] as McpDecision[]).map((d) => (
              <ToggleGroupItem key={d} value={d}>
                {decisionLabel(d)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.mcp.policies.strictest',
              'If several rules match, the strictest decision wins (Deny › Require approval › Allow).'
            )}
          </p>
        </div>
        <div className="space-y-1">
          <Label htmlFor="mcp-pol-note">{translate('auto.mcp.policies.note', 'Note')}</Label>
          <Textarea
            id="mcp-pol-note"
            value={draft.note ?? ''}
            maxLength={MCP_POLICY_NOTE_MAX}
            onChange={(e) => setDraft({ ...draft, note: e.target.value })}
          />
          <p className="text-xs text-muted-foreground">
            {(draft.note ?? '').length}/{MCP_POLICY_NOTE_MAX}
          </p>
        </div>
        <div className="space-y-1 text-sm" aria-live="polite">
          <p>
            {translate('auto.mcp.policies.impact', 'Matches {{count}} tools', {
              count: matched.length
            })}
            {denied > 0
              ? ` ${translate('auto.mcp.policies.impactDenied', '({{count}} permanently denied)', { count: denied })}`
              : ''}
          </p>
          {matched.length > 0 ? (
            <p className="font-mono text-xs text-muted-foreground">
              {matched
                .slice(0, 5)
                .map((t) => t.name)
                .join(', ')}
            </p>
          ) : null}
        </div>
        {errors.length > 0 ? (
          <ul id="mcp-pol-errors" className="space-y-1 text-sm text-destructive">
            {errors.map((i) => (
              <li key={i.kind}>{policyIssueText(i)}</li>
            ))}
          </ul>
        ) : null}
        {warnings.map((i) => (
          <p key={i.kind} className="text-sm text-muted-foreground">
            {policyIssueText(i)}
          </p>
        ))}
        {failure?.kind === 'hard_deny' || failure?.kind === 'other' ? (
          <McpInlineAlert message={failure.message} />
        ) : null}
        {failure?.kind === 'forbidden' ? (
          <McpInlineAlert
            message={translate('auto.mcp.admin.required', 'Administrator access required.')}
          />
        ) : null}
        {failure?.kind === 'not_found' ? (
          <div role="alert" className="space-y-2 rounded-md border border-border px-3 py-2 text-sm">
            <p>{translate('auto.mcp.policies.gone', 'This rule no longer exists.')}</p>
            <Button size="sm" variant="ghost" onClick={onGone}>
              {translate('auto.mcp.policies.closeReload', 'Close and reload')}
            </Button>
          </div>
        ) : null}
        {failure?.kind === 'conflict' ? (
          <div role="alert" className="space-y-2 rounded-md border border-border px-3 py-2 text-sm">
            <p>
              {translate('auto.mcp.policies.conflict', 'This rule was changed by someone else.')}
            </p>
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={reloadLatest}>
                {translate('auto.mcp.policies.reloadLatest', 'Reload latest')}
              </Button>
              <Button size="sm" variant="ghost" onClick={overwrite}>
                {translate('auto.mcp.policies.overwrite', 'Overwrite with my changes')}
              </Button>
            </div>
          </div>
        ) : null}
        {yourDraft ? (
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">
              {translate('auto.mcp.policies.yourDraft', 'Your draft')}
            </p>
            <pre className="whitespace-pre-wrap break-all rounded-md border border-border bg-muted p-2 font-mono text-xs">
              {yourDraft}
            </pre>
          </div>
        ) : null}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={editor.saving}>
            {translate('auto.mcp.common.cancel', 'Cancel')}
          </Button>
          <Button
            disabled={errors.length > 0 || editor.saving || failure?.kind === 'conflict'}
            aria-busy={editor.saving}
            aria-describedby={errors.length > 0 ? 'mcp-pol-errors' : undefined}
            onClick={() => void editor.save(draft)}
          >
            {translate('auto.mcp.policies.save', 'Save rule')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
