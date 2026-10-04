import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import es from './locales/es.json'
import ja from './locales/ja.json'
import ko from './locales/ko.json'
import zh from './locales/zh.json'

// Keys behind the Jira task link setting and the "Generate with AI" prompt
// action. They are read-by-name (not hashed), so nothing generates their
// catalog entries: this test is what keeps non-English UIs from silently
// showing English for them.
const KEYS = [
  'auto.components.settings.ExperimentalPane.jiraTaskLink.title',
  'auto.components.settings.ExperimentalPane.jiraTaskLink.description',
  'auto.components.settings.ExperimentalPane.jiraTaskLink.copy',
  'auto.components.settings.ExperimentalPane.jiraTaskLink.toggleLabel',
  'auto.components.task.GenerateAgentPromptButton.label',
  'auto.components.task.GenerateAgentPromptButton.confirmReplace',
  'auto.hooks.useGenerateAgentPrompt.busy',
  'auto.hooks.useGenerateAgentPrompt.failed',
  'auto.hooks.useGenerateAgentPrompt.noDevServer',
  'auto.hooks.useGenerateAgentPrompt.permissionDenied',
  'auto.hooks.useGenerateAgentPrompt.unsupported'
]

const CATALOGS: Record<string, unknown> = { en, es, ja, ko, zh }

function lookup(catalog: unknown, key: string): unknown {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) {
      return undefined
    }
    node = (node as Record<string, unknown>)[part]
  }
  return node
}

describe('Jira task link / Generate with AI catalog entries', () => {
  for (const [locale, catalog] of Object.entries(CATALOGS)) {
    it(`${locale} has every key as a non-empty string`, () => {
      for (const key of KEYS) {
        const value = lookup(catalog, key)
        expect(typeof value, `${locale}: ${key}`).toBe('string')
        expect((value as string).trim(), `${locale}: ${key}`).not.toBe('')
      }
    })
  }

  for (const locale of ['es', 'ja', 'ko', 'zh']) {
    it(`${locale} is translated, not a copy of the English text`, () => {
      for (const key of KEYS) {
        expect(lookup(CATALOGS[locale], key), `${locale}: ${key}`).not.toBe(lookup(en, key))
      }
    })
  }
})
