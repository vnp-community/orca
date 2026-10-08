import { QUALITY_VISUALIZATION_COPY } from '../components/review-map/quality/quality-visualization-copy'
import { describeQualityCopyLocales } from '../test-support/quality-copy-locale-assertions'

// Keys that are legitimately identical to English in a locale (loan words, acronyms, plain templates).
describeQualityCopyLocales('visualization', QUALITY_VISUALIZATION_COPY, {
  es: ['coverage.excludedItem', 'trend.groupCommit', 'trend.sourceLocal', 'trend.sourceCi'],
  ja: ['trend.sourceCi'],
  ko: ['coverage.excludedItem', 'trend.sourceCi'],
  zh: ['trend.sourceCi']
})
