import type React from 'react'
import { useState } from 'react'
import { LogOut } from 'lucide-react'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { Separator } from '../ui/separator'
import { SearchableSetting } from './SearchableSetting'
import { SettingsSubsectionHeader } from './SettingsFormControls'
import { isWebClientLocation } from '@/lib/web-client-location'
import { useAuthUser } from '../../hooks/useAuthSession'
import { useLogout } from '../../hooks/useLogout'
import { translate } from '@/i18n/i18n'

type GeneralAccountSectionProps = {
  hasPrecedingSections: boolean
}

// CR-LOGIN-001 follow-up: the sidebar's UserAvatarMenu is the only existing
// logout affordance and is easy to miss (a small unlabeled avatar icon) —
// this surfaces the same action somewhere people already look for account
// controls. Web-only, matching WebUserAvatarSection's own gate: desktop
// mode has no equivalent session to log out of here.
export function GeneralAccountSection({
  hasPrecedingSections
}: GeneralAccountSectionProps): React.JSX.Element | null {
  const authUser = useAuthUser()
  const logout = useLogout()
  const [isLoggingOut, setIsLoggingOut] = useState(false)

  if (!isWebClientLocation() || !authUser) {
    return null
  }

  const handleLogout = async (): Promise<void> => {
    setIsLoggingOut(true)
    try {
      await logout()
    } catch {
      setIsLoggingOut(false)
    }
  }

  return (
    <div className="space-y-4">
      {hasPrecedingSections ? <Separator /> : null}
      <SettingsSubsectionHeader
        title={translate('auto.components.settings.GeneralAccountSection.account', 'Account')}
      />
      <SearchableSetting
        title={translate('auto.components.settings.GeneralAccountSection.logout_title', 'Log out')}
        description={authUser.email}
        keywords={['logout', 'log out', 'sign out', 'account', authUser.email]}
        className="flex items-center justify-between gap-4 py-2"
      >
        <div className="flex flex-col">
          <Label>{authUser.name}</Label>
          <span className="text-xs text-muted-foreground">{authUser.email}</span>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void handleLogout()}
          disabled={isLoggingOut}
          className="shrink-0 gap-1.5"
        >
          <LogOut className="size-3.5" />
          {isLoggingOut
            ? translate(
                'auto.components.settings.GeneralAccountSection.logging_out',
                'Logging out…'
              )
            : translate('auto.components.settings.GeneralAccountSection.logout_title', 'Log out')}
        </Button>
      </SearchableSetting>
    </div>
  )
}
