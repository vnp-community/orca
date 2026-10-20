import { join } from 'node:path'
import { describeTranslateCallLocales } from '../../../test-support/translate-call-locale-assertions'

describeTranslateCallLocales('analysis revision panel and version list', [
  join(__dirname, 'AnalysisRevisionPanel.tsx'),
  join(__dirname, 'RevisionPromptBox.tsx'),
  join(__dirname, 'OlderVersionBanner.tsx'),
  join(__dirname, 'SolutionRevisionFeedback.tsx'),
  join(__dirname, 'SolutionVersionSwitcher.tsx')
])
