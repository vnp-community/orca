import { describe, expect, it } from 'vitest'
import { revisionSuggestions } from './revision-suggestions'

describe('revisionSuggestions', () => {
  it.each(['solution', 'diagnosis', 'findings', 'answer'] as const)(
    'offers four distinct, non-empty chips for %s',
    (kind) => {
      const chips = revisionSuggestions(kind)
      expect(chips).toHaveLength(4)
      expect(new Set(chips.map((c) => c.id)).size).toBe(4)
      expect(new Set(chips.map((c) => c.text)).size).toBe(4)
      for (const chip of chips) {
        expect(chip.text.trim()).not.toBe('')
      }
    }
  )

  it('differs per kind of document', () => {
    const texts = (['solution', 'diagnosis', 'findings', 'answer'] as const).flatMap((kind) =>
      revisionSuggestions(kind).map((c) => c.text)
    )
    expect(new Set(texts).size).toBe(16)
  })

  it('offers nothing for a kind this client does not know', () => {
    expect(revisionSuggestions('unknown')).toEqual([])
  })
})
