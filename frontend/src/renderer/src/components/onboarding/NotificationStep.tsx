import { useCallback, useRef, useState } from 'react'
import { BellRing, Bell, BellOff, FileAudio, Upload, Smartphone, Check } from 'lucide-react'
import { toast } from 'sonner'
import type { GlobalSettings } from '../../../../shared/types'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { sendNotificationSettingsTestNotification } from '@/components/settings/NotificationsPane'
import { getNotificationSoundOptions } from '@/components/notification-sound-options'
import {
  MacNotificationPermissionCard,
  useMacNotificationPermissionState
} from '@/components/notifications/mac-notification-permission-card'
import { useMountedRef } from '@/hooks/useMountedRef'
import { translate } from '@/i18n/i18n'
import { isWebClientLocation } from '@/lib/web-client-location'
import { useBrowserNotificationPermission } from '@/hooks/useBrowserNotificationPermission'
import { useWebPushSubscription } from '@/hooks/useWebPushSubscription'

import { shellPickAudio } from '../../runtime/runtime-shell-client'
type NotificationStepProps = {
  settings: GlobalSettings | null
  updateSettings: (updates: Partial<GlobalSettings>) => Promise<void> | void
}

const CHOOSE_CUSTOM_SOUND_VALUE = 'choose-custom-file'

type NotificationSoundSelectValue =
  | GlobalSettings['notifications']['customSoundId']
  | typeof CHOOSE_CUSTOM_SOUND_VALUE

function isNotificationSoundId(
  value: NotificationSoundSelectValue
): value is GlobalSettings['notifications']['customSoundId'] {
  return value !== CHOOSE_CUSTOM_SOUND_VALUE
}

export function NotificationStep({
  settings,
  updateSettings
}: NotificationStepProps): React.JSX.Element {
  // Detect web mode — different UI for browser vs Electron
  const isWebMode = isWebClientLocation()

  const notificationSettings = settings?.notifications
  const notificationSettingsRef = useRef(notificationSettings)
  // Why: undefined settings are still loading — assume enabled (the default)
  // so the fresh-install permission flow starts without waiting.
  const [macPermissionState, setMacPermissionState] = useMacNotificationPermissionState(
    notificationSettings?.enabled !== false
  )
  const [isPickingSound, setIsPickingSound] = useState(false)
  const [selectPortalRoot, setSelectPortalRoot] = useState<HTMLElement | null>(null)
  const syncedNotificationSettingsRef = useRef(notificationSettings)
  const mountedRef = useMountedRef()

  if (syncedNotificationSettingsRef.current !== notificationSettings) {
    syncedNotificationSettingsRef.current = notificationSettings
    // Why: handlers optimistically update the ref before persisted settings
    // flow back through props, so local re-renders must not overwrite it.
    notificationSettingsRef.current = notificationSettings
  }

  const setSelectPortalHost = useCallback((node: HTMLDivElement | null) => {
    // Why: onboarding sits above body-level portals, so the select menu must
    // portal into the overlay to stay clickable.
    setSelectPortalRoot(node?.closest<HTMLElement>('[data-onboarding-overlay]') ?? node)
  }, [])

  const updateNotificationSettings = async (
    updates: Partial<GlobalSettings['notifications']>
  ): Promise<void> => {
    const current = notificationSettingsRef.current
    if (!current) {
      return
    }
    const nextNotifications = {
      ...current,
      ...updates
    }
    notificationSettingsRef.current = nextNotifications
    await updateSettings({
      notifications: nextNotifications
    })
  }

  const getCustomSoundVolume = (): number =>
    notificationSettingsRef.current?.customSoundVolume ?? 100

  const previewSound = async (
    customSoundId: GlobalSettings['notifications']['customSoundId']
  ): Promise<void> => {
    if (customSoundId === 'system') {
      return
    }
    const result = await window.api.notifications.playSound({
      force: true,
      volume: getCustomSoundVolume()
    })
    if (!result.played) {
      if (mountedRef.current) {
        toast.error(
          translate(
            'auto.components.onboarding.NotificationStep.b6a994e36e',
            'Notification sound could not be played'
          )
        )
      }
    }
  }

  const handleChooseCustomSound = async (): Promise<void> => {
    setIsPickingSound(true)
    try {
      const soundPath = await shellPickAudio()
      if (soundPath) {
        await updateNotificationSettings({ customSoundId: 'custom', customSoundPath: soundPath })
        await previewSound('custom')
      }
    } finally {
      if (mountedRef.current) {
        setIsPickingSound(false)
      }
    }
  }

  const handleSoundSelect = async (value: NotificationSoundSelectValue): Promise<void> => {
    if (!isNotificationSoundId(value)) {
      await handleChooseCustomSound()
      return
    }
    await updateNotificationSettings({ customSoundId: value })
    await previewSound(value)
  }

  const handleSendTestNotification = async (): Promise<void> => {
    if (!notificationSettings) {
      toast.error(
        translate(
          'auto.components.onboarding.NotificationStep.3cd5374e22',
          'Notification settings are still loading'
        )
      )
      return
    }
    const showsMacPermissionCard = macPermissionState !== null
    const outcome = await sendNotificationSettingsTestNotification(
      notificationSettings,
      getCustomSoundVolume(),
      showsMacPermissionCard ? { suppressSystemPermissionToasts: true } : undefined
    )
    if (!mountedRef.current || !showsMacPermissionCard) {
      return
    }
    // Why: the test doubles as a permission re-check — its confirmed outcome
    // is fresher than whatever the mount-time probe reported.
    if (outcome === 'delivered') {
      setMacPermissionState('enabled')
    } else if (outcome === 'not-displayed') {
      setMacPermissionState('blocked')
    }
  }

  if (!notificationSettings) {
    return (
      <div className="rounded-xl border border-border bg-muted/20 px-5 py-4 text-sm text-muted-foreground">
        {translate(
          'auto.components.onboarding.NotificationStep.e52aacf380',
          'Loading notification settings…'
        )}
      </div>
    )
  }

  const customPath = notificationSettings.customSoundPath
  const selectedSoundId = notificationSettings.customSoundId
  const soundOptions = getNotificationSoundOptions(customPath)

  // Web mode: use browser notification + web push UI
  if (isWebMode) {
    return <WebModeNotificationStep settings={settings} updateSettings={updateSettings} />
  }

  return (
    <div ref={setSelectPortalHost} className="space-y-5">
      <MacNotificationPermissionCard state={macPermissionState} />

      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-sm font-semibold text-foreground">
            {translate('auto.components.onboarding.NotificationStep.0af746e41f', 'Choose a sound')}
          </h2>
          <p className="text-[13px] leading-relaxed text-muted-foreground">
            {translate(
              'auto.components.onboarding.NotificationStep.0fe570690c',
              'Pick the alert Orca plays after a desktop notification is delivered.'
            )}
          </p>
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2 text-sm font-medium text-foreground">
            <FileAudio className="size-4" />
            {translate(
              'auto.components.onboarding.NotificationStep.53aaffe49a',
              'Notification Sound'
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select
              value={selectedSoundId}
              disabled={isPickingSound}
              onValueChange={(value) =>
                void handleSoundSelect(value as NotificationSoundSelectValue)
              }
            >
              <SelectTrigger className="w-[360px] max-w-full" size="sm">
                <SelectValue
                  placeholder={translate(
                    'auto.components.onboarding.NotificationStep.dc897423e1',
                    'Choose notification sound'
                  )}
                />
              </SelectTrigger>
              <SelectContent
                portalContainer={selectPortalRoot}
                align="start"
                className="w-[--radix-select-trigger-width]"
              >
                {soundOptions.map((option) => {
                  const OptionIcon = option.icon
                  return (
                    <SelectItem key={option.id} value={option.id}>
                      <OptionIcon className="size-4" />
                      <span className="truncate">{option.title}</span>
                    </SelectItem>
                  )
                })}
                <SelectSeparator />
                <SelectItem value={CHOOSE_CUSTOM_SOUND_VALUE}>
                  <Upload className="size-4" />
                  <span>
                    {customPath
                      ? translate(
                          'auto.components.onboarding.NotificationStep.ac80d97e02',
                          'Change Custom File'
                        )
                      : translate(
                          'auto.components.onboarding.NotificationStep.c0692baa52',
                          'Choose Custom File'
                        )}
                  </span>
                </SelectItem>
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-2"
              onClick={() => void handleSendTestNotification()}
            >
              <BellRing className="size-3.5" />
              {translate(
                'auto.components.onboarding.NotificationStep.3bede04483',
                'Send Test Notification'
              )}
            </Button>
          </div>
        </div>
      </section>
    </div>
  )
}

// ── WebModeNotificationStep ───────────────────────────────────────────────────

// eslint-disable-next-line @typescript-eslint/no-unused-vars -- web-mode variant doesn't need desktop settings, kept for prop-type parity with caller
function WebModeNotificationStep(_props: NotificationStepProps): React.JSX.Element {
  const { state: permState, requestPermission } = useBrowserNotificationPermission()
  const {
    state: pushState,
    subscribe,
    unsubscribe,
    isSupported: isPushSupported
  } = useWebPushSubscription()

  const sendTestNotification = useCallback(() => {
    if (permState === 'granted') {
      // eslint-disable-next-line no-new
      new Notification('Orca Test', { body: 'Notifications are working correctly.' })
    }
  }, [permState])

  return (
    <div className="space-y-5" data-web-notification-step>
      {/* Section 1: Browser Notifications */}
      <section className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-sm font-semibold text-foreground">Browser Notifications</h2>
          <p className="text-[13px] leading-relaxed text-muted-foreground">
            Allow Orca to send desktop notifications from your browser.
          </p>
        </div>

        {permState === 'unsupported' && (
          <div className="flex items-center gap-2 rounded-xl border border-border bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
            <BellOff className="size-4 shrink-0" />
            Browser notifications are not supported in this environment.
          </div>
        )}

        {permState === 'denied' && (
          <div className="flex items-center gap-2 rounded-xl border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm">
            <BellOff className="size-4 shrink-0 text-amber-500" />
            <p className="text-muted-foreground">
              Notifications are blocked. Enable them in your browser settings, then reload.
            </p>
          </div>
        )}

        {permState === 'granted' && (
          <div className="flex items-center gap-2 rounded-xl border border-green-500/30 bg-green-500/5 px-4 py-3 text-sm text-green-700 dark:text-green-400">
            <Check className="size-4 shrink-0" />
            Browser notifications are enabled.
          </div>
        )}

        {permState === 'default' && (
          <Button
            id="enable-browser-notif-btn"
            type="button"
            variant="outline"
            size="sm"
            className="gap-2"
            onClick={() => void requestPermission()}
          >
            <Bell className="size-3.5" />
            Enable Browser Notifications
          </Button>
        )}
      </section>

      {/* Section 2: Push Notifications (only when browser granted + SW supported) */}
      {permState === 'granted' && isPushSupported && (
        <section className="space-y-3">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-foreground">Push Notifications</h2>
            <p className="text-[13px] leading-relaxed text-muted-foreground">
              Receive notifications even when Orca is not in the foreground.
            </p>
          </div>
          {pushState === 'subscribed' ? (
            <div className="flex items-center justify-between rounded-xl border border-green-500/30 bg-green-500/5 px-4 py-3">
              <div className="flex items-center gap-2 text-sm text-green-700 dark:text-green-400">
                <Smartphone className="size-4" />
                Push notifications active.
              </div>
              <Button type="button" variant="outline" size="sm" onClick={() => void unsubscribe()}>
                Disable
              </Button>
            </div>
          ) : (
            <Button
              id="subscribe-push-btn"
              type="button"
              variant="outline"
              size="sm"
              className="gap-2"
              disabled={pushState === 'subscribing'}
              onClick={() => void subscribe()}
            >
              <Smartphone className="size-3.5" />
              {pushState === 'subscribing' ? 'Subscribing…' : 'Enable Push Notifications'}
            </Button>
          )}
        </section>
      )}

      {/* Section 3: Other Channels */}
      <section className="space-y-2">
        <h2 className="text-sm font-semibold text-foreground">Other Channels</h2>
        <p className="text-[13px] leading-relaxed text-muted-foreground">
          Additional notification channels (Slack, email, webhooks) can be configured in server
          settings.
        </p>
      </section>

      {/* Send Test Notification */}
      {permState === 'granted' && (
        <Button
          id="test-notification-btn"
          type="button"
          variant="outline"
          size="sm"
          className="gap-2"
          onClick={sendTestNotification}
        >
          <BellRing className="size-3.5" />
          Send Test Notification
        </Button>
      )}
    </div>
  )
}
