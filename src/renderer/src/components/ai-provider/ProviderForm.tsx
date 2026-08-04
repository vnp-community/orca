// ProviderForm.tsx — Add/Edit AI provider account dialog (TASK-V5-08)
import { useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Tracers } from '../../../../shared/trace/tracers'
import { CredentialInput } from './CredentialInput'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import type { AIProviderAccount, AIProviderType, AIProviderScope } from '../../types/ai-provider-types'
import { toast } from 'sonner'

interface ProviderFormProps {
  account?: AIProviderAccount
  onClose:  () => void
}

export function ProviderForm({ account, onClose }: ProviderFormProps) {
  const [provider,  setProvider]  = useState<AIProviderType>(account?.provider  ?? 'anthropic')
  const [label,     setLabel]     = useState(account?.label     ?? '')
  const [model,     setModel]     = useState(account?.model     ?? '')
  const [baseUrl,   setBaseUrl]   = useState(account?.baseUrl   ?? '')
  const [scope,     setScope]     = useState<AIProviderScope>(account?.scope ?? 'server')
  const [devServer, setDevServer] = useState(account?.devServerId ?? '')
  const [quota,     setQuota]     = useState(account?.quotaLimitDay ?? 0)
  const [isSaving,  setIsSaving]  = useState(false)

  const [encryptedCred, setEncryptedCred] = useState<{ encryptedBlob: string; iv: string } | null>(null)
  const [hasNewCred, setHasNewCred]       = useState(false)

  const handleSave = async () => {
    setIsSaving(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      const payload = { provider, label, model, baseUrl, scope, devServerId: devServer, quotaLimitDay: quota }
      let accountId = account?.id

      // Metadata create/update: KHÔNG traced riêng — CRUD đơn (không băng qua boundary
      // quan trọng ngoài WS RPC 1 hop, latency thấp). Chỉ writeCredential đáng trace vì
      // nó băng qua relay tới Dev Server và có thể timeout/fail độc lập.
      if (!accountId) {
        const created = await callRuntimeRpc(target, 'aiProvider.create', payload) as AIProviderAccount
        accountId = created.id
      } else {
        await callRuntimeRpc(target, 'aiProvider.update', { accountId, ...payload })
      }

      // Write credential if new one provided — BL-AIP-01, băng qua relay tới Dev Server.
      if (hasNewCred && encryptedCred) {
        // SECURITY: span fields chỉ chứa accountId/provider/blobLength — KHÔNG bao giờ
        // encryptedCred.encryptedBlob/iv.
        const span = Tracers.uiAiProviderWriteCredFlow.start({
          accountId, provider, blobLength: encryptedCred.encryptedBlob.length,
        })
        try {
          await callRuntimeRpc(target, 'aiProvider.writeCredential', {
            accountId,
            encryptedBlob: encryptedCred.encryptedBlob,
            iv:            encryptedCred.iv,
            traceId:       span.id,
          })
          span.ok({ accountId })
        } catch (err: any) {
          // SECURITY: err có thể chứa message từ backend — không đưa toàn bộ err object
          // vào fields nếu nó có khả năng echo lại input; chỉ truyền qua span.fail(err).
          span.fail(err, { accountId })
          throw err
        }
      }

      toast.success(account ? 'Account updated' : 'Account created')
      onClose()
    } catch (err: any) {
      toast.error(err?.message ?? 'Save failed')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="max-w-md" data-testid="provider-form">
        <DialogHeader>
          <DialogTitle>{account ? 'Edit' : 'Add'} AI Provider</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Provider type */}
          <div>
            <Label>Provider</Label>
            <Select value={provider} onValueChange={v => setProvider(v as AIProviderType)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(['anthropic', 'openai', 'gemini', 'azure', 'bedrock', 'ollama', 'vllm'] as AIProviderType[]).map(p => (
                  <SelectItem key={p} value={p} className="capitalize">{p}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div><Label>Label</Label><Input value={label} onChange={e => setLabel(e.target.value)} /></div>
          <div><Label>Default Model</Label><Input value={model} onChange={e => setModel(e.target.value)} /></div>

          {['ollama', 'vllm'].includes(provider) && (
            <div>
              <Label>Base URL</Label>
              <Input value={baseUrl} placeholder="http://localhost:11434" onChange={e => setBaseUrl(e.target.value)} />
            </div>
          )}

          <div>
            <Label>Dev Server ID</Label>
            <Input value={devServer} onChange={e => setDevServer(e.target.value)} placeholder="server-id" />
          </div>

          <div>
            <Label>Scope</Label>
            <Select value={scope} onValueChange={v => setScope(v as AIProviderScope)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="server">Server</SelectItem>
                <SelectItem value="project">Project</SelectItem>
                <SelectItem value="user">User</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div><Label>Daily Quota (0 = unlimited)</Label>
            <Input type="number" value={quota} onChange={e => setQuota(+e.target.value)} />
          </div>

          <CredentialInput
            provider={provider}
            hasExisting={!!account?.id}
            onEncrypted={(blob, iv) => { setEncryptedCred({ encryptedBlob: blob, iv }); setHasNewCred(true) }}
            onClear={() => setHasNewCred(false)}
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={isSaving} data-testid="save-provider-btn">
            {isSaving ? 'Saving...' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
