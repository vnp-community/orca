import { describe, expect, it } from 'vitest'
import {
  QUALITY_SCORECARD_COPY,
  QUALITY_SCORECARD_COPY_GROUP
} from '../components/review-map/quality/quality-scorecard-copy'
import { describeQualityCopyLocales } from '../test-support/quality-copy-locale-assertions'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Strings that are legitimately identical to English in a given locale.
describeQualityCopyLocales(QUALITY_SCORECARD_COPY_GROUP, QUALITY_SCORECARD_COPY, {
  es: ['source.ci', 'source.local', 'step.duration'],
  ja: ['source.ci', 'run.percent'],
  ko: ['source.ci', 'run.percent'],
  zh: ['source.ci', 'run.percent']
})

describe('quality lens tab and dock labels', () => {
  it.each([
    ['en', en],
    ['es', es],
    ['ja', ja],
    ['ko', ko],
    ['zh', zh]
  ] as const)('%s has the lens label', (_l, catalog) => {
    const label = (
      catalog as never as {
        auto: { components: { reviewMap: { lens: { quality: { label: string } } } } }
      }
    ).auto.components.reviewMap.lens.quality.label
    expect(label.trim()).not.toBe('')
    const dock = (
      catalog as never as {
        auto: { components: { reviewMap: { shell: { dock: { qualityFindings: string } } } } }
      }
    ).auto.components.reviewMap.shell.dock.qualityFindings
    expect(dock.trim()).not.toBe('')
  })
})

describe('scorecard wording', () => {
  it('no locale string claims safety, cleanliness or a score in English', () => {
    const text = Object.values(QUALITY_SCORECARD_COPY).join(' ').toLowerCase()
    expect(text).not.toMatch(/\bsafe\b|\bclean\b|\bscore\b/)
  })
})
