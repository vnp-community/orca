import { join } from 'node:path'
import { describeTranslateCallLocales } from '../../test-support/translate-call-locale-assertions'

describeTranslateCallLocales('git ref picker', [
  join(__dirname, 'RepoRefPicker.tsx'),
  join(__dirname, '../settings/BaseRefPicker.tsx')
])
