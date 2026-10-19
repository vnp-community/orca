import { join } from 'node:path'
import { describeTranslateCallLocales } from '../../../test-support/translate-call-locale-assertions'

describeTranslateCallLocales('analysis revision', [
  join(__dirname, 'analysis-revision-errors.ts'),
  join(__dirname, 'revision-suggestions.ts')
])
