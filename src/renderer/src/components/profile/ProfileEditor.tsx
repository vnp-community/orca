// ProfileEditor.tsx — Edit user/dept/company profiles (TDD-FE-11)
import { useState } from 'react'
import { Button } from '../ui/button'
import { Tabs, TabsList, TabsTrigger } from '../ui/tabs'
import { useProfile, useProfileActions } from '../../hooks/useProfile'
import { ProfileFieldRow } from './ProfileFieldRow'
import { ModelSelector } from './ModelSelector'
import type { OrcaProfile } from '../../types/profile-types'

interface ProfileEditorProps {
  scope:     'user' | 'dept' | 'company'
  scopeId?:  string
  readOnly?: boolean
}

export function ProfileEditor({ scope, scopeId, readOnly = false }: ProfileEditorProps) {
  const { resolvedProfile, userProfile } = useProfile()
  const { saveProfile } = useProfileActions()
  const [localProfile, setLocalProfile]  = useState<OrcaProfile>(
    scope === 'user' ? (userProfile ?? {}) : {}
  )
  const [activeTab, setActiveTab] = useState<'own' | 'resolved'>('own')

  const setField = (path: string, value: unknown) => {
    const keys = path.split('.')
    setLocalProfile(prev => {
      const next = structuredClone(prev)
      let cur: any = next
      for (let i = 0; i < keys.length - 1; i++) {
        cur[keys[i]] ??= {}
        cur = cur[keys[i]]
      }
      cur[keys[keys.length - 1]] = value
      return next
    })
  }

  const displayProfile = activeTab === 'resolved' ? resolvedProfile : localProfile

  return (
    <div className="profile-editor space-y-4 p-4" data-testid="profile-editor">
      {scope === 'user' && (
        <Tabs value={activeTab} onValueChange={v => setActiveTab(v as any)}>
          <TabsList>
            <TabsTrigger value="own">My Settings</TabsTrigger>
            <TabsTrigger value="resolved">Effective Settings</TabsTrigger>
          </TabsList>
        </Tabs>
      )}

      {/* Agent section */}
      <section>
        <h3 className="font-semibold text-sm mb-2">Agent</h3>
        <ProfileFieldRow
          label="Preferred Model"
          source={resolvedProfile?._sources?.['agent.preferredModel']}
        >
          <ModelSelector
            value={(displayProfile as any)?.agent?.preferredModel}
            onChange={v => setField('agent.preferredModel', v)}
            disabled={readOnly || activeTab === 'resolved'}
            approvedModels={resolvedProfile?.security?.approvedModels}
          />
        </ProfileFieldRow>
      </section>

      {/* Security section — company only */}
      <section>
        <h3 className="font-semibold text-sm mb-2">
          Security {scope !== 'company' && '🔒'}
        </h3>
        {scope !== 'company' && (
          <ProfileFieldRow label="Approved Models" locked>
            <span className="text-sm text-muted-foreground">Managed by company admin</span>
          </ProfileFieldRow>
        )}
      </section>

      {!readOnly && activeTab !== 'resolved' && (
        <Button
          onClick={() => saveProfile(scope, localProfile, scopeId)}
          data-testid="save-profile-btn"
        >
          Save Changes
        </Button>
      )}
    </div>
  )
}
