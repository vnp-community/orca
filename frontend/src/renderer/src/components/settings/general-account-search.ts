import { translate } from '@/i18n/i18n'
import { createLocalizedCatalog } from '@/i18n/localized-catalog'
import { translateSearchKeyword } from './settings-search-keywords'

export const getGeneralAccountSearchEntries = createLocalizedCatalog(() => [
  {
    title: translate('auto.components.settings.general.search.account_logout_title', 'Log out'),
    description: translate(
      'auto.components.settings.general.search.account_logout_desc',
      'Sign out of your Orca account on this device.'
    ),
    keywords: [
      ...translateSearchKeyword(
        'auto.components.settings.general.search.account_kw_logout',
        'logout'
      ),
      ...translateSearchKeyword(
        'auto.components.settings.general.search.account_kw_signout',
        'sign out'
      ),
      ...translateSearchKeyword(
        'auto.components.settings.general.search.account_kw_account',
        'account'
      )
    ]
  }
])
