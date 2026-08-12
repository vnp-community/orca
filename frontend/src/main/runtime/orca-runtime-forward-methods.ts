// frontend/src/main/runtime/orca-runtime-forward-methods.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-083): replaces the repetitive
// `name: Type['name'] = this.instance.name.bind(this.instance)` forwarding
// field pattern used ~545 times across orca-runtime.ts's composition
// wiring. Binds exactly the named methods from `source` onto `target`, so
// external callers keep calling `runtime.methodName(...)` unchanged — only
// the declaration mechanics change, not the public API surface.
//
// Safety story (deliberately different from every other TASK-BIGFILE-0NN
// extraction in this effort): `methods` is typed as `readonly (keyof
// Source)[]`, so tsc rejects a typo'd or renamed method name at the call
// site. Each call site still needs a matching `declare name: Source['name']`
// field for tsc to type-check `this.name(...)` elsewhere in the class — this
// helper only removes the redundant runtime `.bind()` assignment, not the
// compile-time contract. But unlike every prior task, the actual runtime
// wiring (does `this.name` really become a callable, correctly-bound
// function after construction?) is NOT verified by tsc alone anymore —
// `declare` fields emit no runtime code, so a bug in this function would
// compile clean and fail silently at call time. Covered by
// orca-runtime-forward-methods.test.ts instead.
export function forwardMethods<Source extends object, K extends keyof Source>(
  target: object,
  source: Source,
  methods: readonly K[]
): void {
  for (const method of methods) {
    const value = source[method]
    if (typeof value !== 'function') {
      throw new Error(
        `forwardMethods: '${String(method)}' is not a function on ${source.constructor.name}`
      )
    }
    ;(target as Record<K, unknown>)[method] = (value as (...args: unknown[]) => unknown).bind(
      source
    )
  }
}
