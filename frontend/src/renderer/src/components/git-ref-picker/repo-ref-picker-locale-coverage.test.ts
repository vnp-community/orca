import { join } from 'node:path'
import { describeTranslateCallLocales } from '../../test-support/translate-call-locale-assertions'

describeTranslateCallLocales('git ref picker', [
  join(__dirname, 'RepoRefPicker.tsx'),
  join(__dirname, 'RepoBranchPicker.tsx'),
  join(__dirname, 'RepoBranchListPicker.tsx'),
  join(__dirname, 'repo-branch-list-parts.tsx'),
  join(__dirname, '../settings/BaseRefPicker.tsx')
])
