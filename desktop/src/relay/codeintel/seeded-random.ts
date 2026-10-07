/**
 * Deterministic pseudo-random number generator using Mulberry32.
 * Used for zero-dependency seeded property fuzzing.
 */
export function mulberry32(seed: number): () => number {
  let s = seed >>> 0
  return function (): number {
    s = (s + 0x6d2b79f5) | 0
    let t = Math.imul(s ^ (s >>> 15), 1 | s)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export class SeededRandom {
  private rng: () => number

  constructor(public readonly seed: number) {
    this.rng = mulberry32(seed)
  }

  nextFloat(): number {
    return this.rng()
  }

  nextInt(min: number, max: number): number {
    return Math.floor(this.rng() * (max - min + 1)) + min
  }

  nextChoice<T>(arr: T[]): T {
    return arr[this.nextInt(0, arr.length - 1)]
  }

  nextString(maxLen = 32): string {
    const len = this.nextInt(0, maxLen)
    const alphabet = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./\\ \t\r\n\x00;$\'`"(){}[]\uFF0D\u2212\u00A0'
    let res = ''
    for (let i = 0; i < len; i++) {
      res += this.nextChoice(alphabet.split(''))
    }
    return res
  }
}
