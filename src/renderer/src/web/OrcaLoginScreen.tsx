// OrcaLoginScreen — SSO/pairing code login screen (CR-006, TASK-006-B)
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { translate } from '@/i18n/i18n'

// Why: these types reflect the server auth config returned by auth:getServerConfig.
// The actual API call happens in the parent that renders this screen.
type SsoProvider = {
  id: string
  label: string
  iconUrl?: string
}

type OrcaServerAuthConfig = {
  requiresAuth: boolean
  orgName?: string
  ssoProviders?: SsoProvider[]
}

type OrcaLoginScreenProps = {
  serverConfig: OrcaServerAuthConfig
  onUsePairingCode: () => void
}

export function OrcaLoginScreen({
  serverConfig,
  onUsePairingCode
}: OrcaLoginScreenProps): React.JSX.Element {
  const [isLoading, setIsLoading] = useState(false)

  const handleSsoLogin = async (provider: SsoProvider): Promise<void> => {
    setIsLoading(true)
    try {
      // Redirect to SSO provider — response handled by backend IPC
      await window.api.auth?.startSsoFlow?.({ providerId: provider.id })
    } catch {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-[380px] space-y-6">
        {/* Header */}
        <div className="space-y-2 text-center">
          <h1 className="text-2xl font-bold">
            {translate('login.title', 'Sign in to Orca')}
          </h1>
          {serverConfig.orgName && (
            <p className="text-sm text-muted-foreground">{serverConfig.orgName}</p>
          )}
        </div>

        {/* SSO provider buttons */}
        {serverConfig.ssoProviders && serverConfig.ssoProviders.length > 0 && (
          <div className="space-y-2">
            {serverConfig.ssoProviders.map((provider) => (
              <Button
                key={provider.id}
                variant="outline"
                className="w-full gap-2"
                disabled={isLoading}
                onClick={() => void handleSsoLogin(provider)}
              >
                {provider.iconUrl && (
                  <img
                    src={provider.iconUrl}
                    alt=""
                    className="size-4 object-contain"
                  />
                )}
                {translate('login.continueWith', `Continue with ${provider.label}`)}
              </Button>
            ))}
            <Separator className="my-4" />
          </div>
        )}

        {/* Fallback: pairing code flow */}
        <Button
          variant="ghost"
          className="w-full text-sm text-muted-foreground"
          onClick={onUsePairingCode}
        >
          {translate('login.usePairingCode', 'Use pairing code instead')}
        </Button>
      </div>
    </div>
  )
}
