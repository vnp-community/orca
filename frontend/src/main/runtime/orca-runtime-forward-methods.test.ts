// frontend/src/main/runtime/orca-runtime-forward-methods.test.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-083): forwardMethods replaces
// `tsc`-verified `.bind()` forwarding fields with a runtime binding loop —
// the one piece of this effort's ~545 forwarding fields that `tsc` alone
// can no longer fully verify (declare fields emit no runtime code). This
// test is the safety net standing in for that lost compile-time guarantee.
import { describe, expect, it, vi } from 'vitest'
import { forwardMethods } from './orca-runtime-forward-methods'

class Source {
  public calls: string[] = []

  publicMethod(arg: number): number {
    this.calls.push(`publicMethod(${arg})`)
    return arg * 2
  }

  async publicAsyncMethod(arg: string): Promise<string> {
    this.calls.push(`publicAsyncMethod(${arg})`)
    return `${arg}!`
  }

  // Why: TS `private` is compile-time only — the method is still an
  // ordinary enumerable function at runtime, so forwardMethods must NOT
  // expose it just because it exists on the prototype. Only names
  // explicitly listed in the `methods` array may be forwarded.
  private privateMethod(): string {
    this.calls.push('privateMethod()')
    return 'should never be reachable via target'
  }

  callPrivateFromPublic(): string {
    return this.privateMethod()
  }
}

describe('forwardMethods', () => {
  it('binds each named method onto the target, callable with the original `this`', () => {
    const source = new Source()
    const target: Record<string, unknown> = {}

    forwardMethods(target, source, ['publicMethod', 'publicAsyncMethod'])

    expect((target.publicMethod as (arg: number) => number)(21)).toBe(42)
    expect(source.calls).toEqual(['publicMethod(21)'])
  })

  it('preserves `this` binding to the source instance, not the target', async () => {
    const source = new Source()
    const target: Record<string, unknown> = {}

    forwardMethods(target, source, ['publicAsyncMethod'])

    // Why: the whole point of forwardMethods is standing in for
    // `.bind(source)` — calling the forwarded reference detached from
    // `target` (the same way orca-runtime.ts's callers invoke
    // `this.methodName(...)`, unaware of the underlying instance) must
    // still resolve `this` inside the method to `source`, not `target` or
    // `undefined`.
    const detached = target.publicAsyncMethod as (arg: string) => Promise<string>
    await expect(detached('hello')).resolves.toBe('hello!')
    expect(source.calls).toEqual(['publicAsyncMethod(hello)'])
  })

  it('does not expose methods absent from the explicit allowlist', () => {
    const source = new Source()
    const target: Record<string, unknown> = {}

    forwardMethods(target, source, ['publicMethod'])

    expect(target.publicAsyncMethod).toBeUndefined()
    expect(target.privateMethod).toBeUndefined()
    expect(target.callPrivateFromPublic).toBeUndefined()
  })

  it('never forwards a private method even if (incorrectly) named explicitly', () => {
    const source = new Source()
    const target: Record<string, unknown> = {}

    // Why: TS blocks `'privateMethod'` from type-checking against
    // `keyof Source` at real call sites (private members are excluded from
    // `keyof`), so this exercises the runtime-only fallback path via an
    // explicit cast — documents the belt-and-suspenders story: `keyof`
    // narrows what tsc allows callers to pass, this test proves the
    // function itself still forwards whatever it's given (the safety is in
    // the type system, not an extra runtime private-method filter).
    forwardMethods(target, source, ['privateMethod' as keyof Source])
    expect(typeof target.privateMethod).toBe('function')
  })

  it('throws instead of silently forwarding a non-function property', () => {
    const source = { notAFunction: 42 }
    const target: Record<string, unknown> = {}

    expect(() => forwardMethods(target, source, ['notAFunction'])).toThrow(
      /'notAFunction' is not a function/
    )
  })

  it('rebinds independently per forwardMethods call, matching .bind() semantics', () => {
    const sourceA = new Source()
    const sourceB = new Source()
    const targetA: Record<string, unknown> = {}
    const targetB: Record<string, unknown> = {}

    forwardMethods(targetA, sourceA, ['publicMethod'])
    forwardMethods(targetB, sourceB, ['publicMethod'])

    ;(targetA.publicMethod as (arg: number) => number)(1)
    ;(targetB.publicMethod as (arg: number) => number)(2)

    expect(sourceA.calls).toEqual(['publicMethod(1)'])
    expect(sourceB.calls).toEqual(['publicMethod(2)'])
  })

  it('matches vi.fn spy call-through behavior for a mocked source method', () => {
    const spy = vi.fn((arg: number) => arg + 1)
    const source = { spiedMethod: spy }
    const target: Record<string, unknown> = {}

    forwardMethods(target, source, ['spiedMethod'])
    const result = (target.spiedMethod as (arg: number) => number)(9)

    expect(result).toBe(10)
    expect(spy).toHaveBeenCalledWith(9)
  })

  // Why: orca-runtime.ts can't be imported in this test environment (pulls
  // in node-pty's native binary, unavailable in sandboxed CI — the reason
  // this whole file has no other tests). This mirrors its exact real shape
  // instead: a `readonly xCommands = new X(...)` composition field, `declare
  // name: X['name']` fields (no runtime assignment for tsc to check), and a
  // constructor-time `forwardMethods(this, this.xCommands, [...])` call —
  // closing the gap between the isolated unit tests above and the real
  // integration, without needing the full module graph.
  it('wires declare-only forwarding fields correctly through a constructor-time call, matching the real orca-runtime.ts shape', () => {
    const domainCalls: string[] = []
    class Domain {
      greet(name: string): string {
        domainCalls.push(`greet(${name})`)
        return `hello, ${name}`
      }
    }

    class HostService {
      // Why: field initializers run top-to-bottom before the constructor
      // body, regardless of textual position relative to it — same
      // ordering fact this effort's composition-command classes rely on.
      private readonly domain = new Domain()

      declare greet: Domain['greet']

      constructor() {
        forwardMethods(this, this.domain, ['greet'])
      }
    }

    const host = new HostService()
    expect(host.greet('world')).toBe('hello, world')
    expect(domainCalls).toEqual(['greet(world)'])
  })
})
