/**
 * CreateRequestDialog — CR-REQ-019-06
 *
 * Creates a request manually or from an external issue. The idempotency key
 * is generated once per open so a double submit cannot create two requests;
 * `created=false` means the issue already had one.
 *
 * @module components/request/CreateRequestDialog
 */

import React, { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import { getScreenSubmitShortcutLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { parseRequest } from '../../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../../shared/request-rpc-methods'
import { requestErrorMessage } from './request-error-message'
import { FILTERABLE_TYPES } from './request-list-filters'
import { REQUEST_BODY_MAX, REQUEST_TITLE_MAX } from './create-request-from-issue'
import type { CreateRequestParams } from './create-request-from-issue'
import type { OrcaProject } from '../../types/workspace-types'
import type { RequestType, RequestUrgency } from '../../../../shared/request-types'

const T = 'auto.components.request.CreateRequestDialog.'
const LET_AI_CLASSIFY = '__ai__'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Pre-filled values from an issue; absent for a manual request. */
  initial?: Partial<CreateRequestParams>
  /** Picks the project once the list loads (e.g. Jira key mapping). */
  pickProjectId?: (projects: OrcaProject[]) => string | null
}

export function CreateRequestDialog({ open, onOpenChange, initial, pickProjectId }: Props): React.JSX.Element {
  const [title, setTitle] = useState(initial?.title ?? '')
  const [body, setBody] = useState(initial?.body ?? '')
  const [projects, setProjects] = useState<OrcaProject[] | null>(null)
  const [projectId, setProjectId] = useState<string>(
    initial?.projectId || useAppStore.getState().requestPage.listFilters.projectId || ''
  )
  const [urgency, setUrgency] = useState<RequestUrgency>('normal')
  const [typeHint, setTypeHint] = useState<string>(LET_AI_CLASSIFY)
  const [submitting, setSubmitting] = useState(false)
  const [errorText, setErrorText] = useState<string | null>(null)
  // Why: one key per open so retries and double clicks stay idempotent.
  const clientRequestId = useRef(crypto.randomUUID())

  useEffect(() => {
    if (!open) {return}
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    void callRuntimeRpc<OrcaProject[]>(target, 'project.list', {})
      .then((list) => {
        if (cancelled) {return}
        const safe = Array.isArray(list) ? list : []
        setProjects(safe)
        setProjectId((current) => current || pickProjectId?.(safe) || (safe.length === 1 ? safe[0].id : ''))
      })
      .catch(() => !cancelled && setProjects([]))
    return () => {
      cancelled = true
    }
  }, [open, pickProjectId])

  const blocked = submitting || title.trim().length === 0 || projectId === ''

  const submit = async (): Promise<void> => {
    if (blocked) {return}
    setSubmitting(true)
    setErrorText(null)
    const result = await callRequestRpc<{ request?: unknown; created?: boolean }>(REQUEST_RPC_METHODS.CREATE, {
      projectId,
      title: title.trim(),
      ...(body.trim() ? { body: body.trim() } : {}),
      ...(initial?.source ? { source: initial.source } : {}),
      hints: { priority: urgency === 'urgent' ? 'high' : undefined, ...(typeHint !== LET_AI_CLASSIFY ? { issueType: typeHint } : {}) },
      clientRequestId: clientRequestId.current
    })
    setSubmitting(false)
    if (!result.ok) {
      setErrorText(
        result.error.code === 'REQUEST_PENDING_LIMIT'
          ? translate(`${T}pendingLimit`, 'Too many requests are waiting for classification. Try again later.')
          : requestErrorMessage(result.error.kind)
      )
      return
    }
    const request = parseRequest(result.value.request)
    const store = useAppStore.getState()
    store.upsertRequests([request])
    onOpenChange(false)
    if (result.value.created === false) {
      toast(translate(`${T}duplicate`, 'This issue already has a request'), {
        action: {
          label: translate(`${T}open`, 'Open'),
          onClick: () => {
            const s = useAppStore.getState()
            s.setRequestPageData({ section: 'requests', requestId: request.id })
            s.setActiveView('requests')
          }
        }
      })
      return
    }
    store.setRequestPageData({ section: 'requests', requestId: request.id })
    store.setActiveView('requests')
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !submitting && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{translate(`${T}title`, 'Create request')}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="create-request-title">{translate(`${T}titleLabel`, 'Title')}</Label>
            <Input id="create-request-title" value={title} maxLength={REQUEST_TITLE_MAX} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="create-request-body">{translate(`${T}body`, 'Description')}</Label>
            <Textarea
              id="create-request-body"
              rows={6}
              value={body}
              maxLength={REQUEST_BODY_MAX}
              onChange={(e) => setBody(e.target.value)}
              onKeyDown={(e) => {
                if (isScreenSubmitShortcut(e)) {
                  e.preventDefault()
                  void submit()
                }
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>{translate(`${T}project`, 'Project')}</Label>
            <Select value={projectId} onValueChange={setProjectId}>
              <SelectTrigger aria-label={translate(`${T}project`, 'Project')} aria-invalid={projectId === ''}>
                <SelectValue placeholder={translate(`${T}projectPlaceholder`, 'Choose a project')} />
              </SelectTrigger>
              <SelectContent>
                {(projects ?? []).map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {projectId === '' && projects !== null && (
              <p className="text-xs text-muted-foreground">{translate(`${T}projectRequired`, 'Choose the project this request belongs to.')}</p>
            )}
          </div>
          <div className="flex gap-3">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label>{translate(`${T}urgency`, 'Urgency')}</Label>
              <Select value={urgency} onValueChange={(v) => setUrgency(v as RequestUrgency)}>
                <SelectTrigger aria-label={translate(`${T}urgency`, 'Urgency')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="normal">{translate('auto.components.request.RequestOverviewTab.urgency.normal', 'Normal')}</SelectItem>
                  <SelectItem value="urgent">{translate('auto.components.request.RequestOverviewTab.urgency.urgent', 'Urgent')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <Label>{translate(`${T}typeHint`, 'Type hint')}</Label>
              <Select value={typeHint} onValueChange={setTypeHint}>
                <SelectTrigger aria-label={translate(`${T}typeHint`, 'Type hint')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={LET_AI_CLASSIFY}>{translate(`${T}letAi`, 'Let the AI classify')}</SelectItem>
                  {FILTERABLE_TYPES.map((t: RequestType) => (
                    <SelectItem key={t} value={t}>
                      {translate(`auto.components.request.RequestType.${t}.label`, t)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          {errorText && (
            <p role="alert" className="text-sm text-destructive">
              {errorText}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
            {translate('auto.components.request.RequestReasonDialog.cancel', 'Cancel')}
          </Button>
          <Button disabled={blocked} onClick={() => void submit()} title={getScreenSubmitShortcutLabel()}>
            {translate(`${T}submit`, 'Create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
