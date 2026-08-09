import { translate } from '@/i18n/i18n'
import type { SettingsNavSection } from '@/lib/settings-navigation-types'

export function getDevServerPaneSearchEntries(): SettingsNavSection['searchEntries'] {
  return [
    {
      title: translate(
        'auto.hooks.useSettingsNavigationMetadata.devServerSearchTitle',
        'Dev Servers'
      ),
      description: translate(
        'auto.hooks.useSettingsNavigationMetadata.devServerSearchDescription',
        'Connect remote developer machines so Orca agents run on your actual dev environment.'
      ),
      keywords: [
        translate('auto.hooks.useSettingsNavigationMetadata.devServerSearchKeywordDev', 'dev'),
        translate('auto.hooks.useSettingsNavigationMetadata.devServerSearchKeywordServer', 'server'),
        translate('auto.hooks.useSettingsNavigationMetadata.devServerSearchKeywordRemote', 'remote')
      ]
    }
  ]
}
