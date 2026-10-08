import { describeQualityCopyLocales } from '../test-support/quality-copy-locale-assertions'
import { QUALITY_FINDINGS_COPY } from '../components/review-map/quality/findings/quality-findings-copy'

const IDENTICAL = ['rowTool', 'waiveReasonCount']

describeQualityCopyLocales('findings', QUALITY_FINDINGS_COPY, {
  es: IDENTICAL,
  ja: IDENTICAL,
  ko: IDENTICAL,
  zh: IDENTICAL
})
