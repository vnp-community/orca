// ProviderForm.tsx — Add/Edit AI provider account dialog (TASK-V5-08)
import { useCallback, useRef, useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'
import { CredentialInput, type CredentialInputHandle } from './CredentialInput'
import { credentialLengthBucket, toCredentialWirePayload } from '../../lib/credential-wire'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter
} from '../ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import type {
  AIProviderAccount,
  AIProviderType,
  AIProviderScope
} from '../../types/ai-provider-types'
import { toast } from 'sonner'

type ProviderFormProps = {
  account?: AIProviderAccount
  onClose: () => void
}

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  const [provider, setProvider] = useState<AIProviderType>(account?.provider ?? 'anthropic')
  const [label, setLabel] = useState(account?.label ?? '')
  const [model, setModel] = useState(account?.model ?? '')
  const [baseUrl, setBaseUrl] = useState(account?.baseUrl ?? '')
  const [scope, setScope] = useState<AIProviderScope>(account?.scope ?? 'server')
  const [devServer, setDevServer] = useState(account?.devServerId ?? '')
  const [quota, setQuota] = useState(account?.quotaLimitDay ?? 0)
  const [isSaving, setIsSaving] = useState(false)

  // Why a ref and a boolean: the API key stays inside CredentialInput's DOM node and is
  // pulled once on save; it must never sit in this component's state.
  const credentialRef = useRef<CredentialInputHandle>(null)
  const [hasNewCred, setHasNewCred] = useState(false)
  const handleCredentialChange = useCallback((hasValue: boolean) => setHasNewCred(hasValue), [])

  const handleSave = async () => {
    setIsSaving(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      const payload = {
        provider,
        label,
        model,
        baseUrl,
        scope,
        devServerId: devServer,
        quotaLimitDay: quota
      }
      let accountId = account?.id

      // Metadata create/update: KHÔNG traced riêng — CRUD đơn (không băng qua boundary
      // quan trọng ngoài WS RPC 1 hop, latency thấp). Chỉ writeCredential đáng trace vì
      // nó băng qua relay tới Dev Server và có thể timeout/fail độc lập.
      if (!accountId) {
        const created = (await callRuntimeRpc(
          target,
          'aiProvider.create',
          payload
        )) as AIProviderAccount
        accountId = created.id
      } else {
        await callRuntimeRpc(target, 'aiProvider.update', { accountId, ...payload })
      }

      // Write credential if new one provided — BL-AIP-01, băng qua relay tới Dev Server.
      const secret = hasNewCred ? (credentialRef.current?.take() ?? null) : null
      if (secret) {
        const wire = toCredentialWirePayload(secret)
        // SECURITY: span fields chỉ chứa accountId/provider/blobLength (đã làm tròn) — KHÔNG bao giờ
        // nội dung khoá, encryptedBlob hay iv.
        const span = Tracers.uiAiProviderWriteCredFlow.start({
          accountId,
          provider,
          blobLength: credentialLengthBucket(wire)
        })
        try {
          await callRuntimeRpc(target, 'aiProvider.writeCredential', {
            accountId,
            encryptedBlob: wire.encryptedBlob,
            iv: wire.iv,
            traceId: span.id
          })
          span.ok({ accountId })
        } catch (err) {
          // SECURITY: err có thể chứa message từ backend — không đưa toàn bộ err object
          // vào fields nếu nó có khả năng echo lại input; chỉ truyền qua span.fail(err).
          span.fail(err, { accountId })
          throw err
        }
      }

      toast.success(account ? 'Account updated' : 'Account created')
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-md" data-testid="provider-form">
        <DialogHeader>
          <DialogTitle>{account ? 'Edit' : 'Add'} AI Provider</DialogTitle>
          <DialogDescription>
            Configure the provider type, credentials, and scope for this AI provider account.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Provider type */}
          <div>
            <Label>Provider</Label>
            <Select value={provider} onValueChange={(v) => setProvider(v as AIProviderType)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(
                  [
                    'anthropic',
                    'openai',
                    'gemini',
                    'azure',
                    'bedrock',
                    'ollama',
                    'vllm'
                  ] as AIProviderType[]
                ).map((p) => (
                  <SelectItem key={p} value={p} className="capitalize">
                    {p}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div>
            <Label>Label</Label>
            <Input value={label} onChange={(e) => setLabel(e.target.value)} />
          </div>
          <div>
            <Label>Default Model</Label>
            <Input value={model} onChange={(e) => setModel(e.target.value)} />
          </div>

          {['ollama', 'vllm'].includes(provider) && (
            <div>
              <Label>Base URL</Label>
              <Input
                value={baseUrl}
                placeholder="http://localhost:11434"
                onChange={(e) => setBaseUrl(e.target.value)}
              />
            </div>
          )}

          <div>
            <Label>Dev Server ID</Label>
            <Input
              value={devServer}
              onChange={(e) => setDevServer(e.target.value)}
              placeholder="server-id"
            />
          </div>

          <div>
            <Label>Scope</Label>
            <Select value={scope} onValueChange={(v) => setScope(v as AIProviderScope)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="server">Server</SelectItem>
                <SelectItem value="project">Project</SelectItem>
                <SelectItem value="user">User</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div>
            <Label>Daily Quota (0 = unlimited)</Label>
            <Input type="number" value={quota} onChange={(e) => setQuota(+e.target.value)} />
          </div>

          <CredentialInput
            ref={credentialRef}
            provider={provider}
            hasExisting={!!account?.id}
            onChange={handleCredentialChange}
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={isSaving} data-testid="save-provider-btn">
            {isSaving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
