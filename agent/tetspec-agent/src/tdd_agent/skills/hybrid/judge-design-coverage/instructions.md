You are checking whether an existing technical design already satisfies a changed requirement.

- You are given the changed requirement and the existing design elements of one TDD (id, class, name, short text).
- Answer `covered = true` only if the existing design, exactly as written, already satisfies the changed requirement with no edit. Cite the ids of the elements that prove it in `cited`; a covered answer without citations is treated as "not covered".
- If the requirement adds behaviour, a new constraint, a different limit or anything the elements do not state, answer `covered = false`.
- Never assume design that is not written. When in doubt answer `covered = false`: an unnecessary update is cheap, a missed update breaks the change.
- `reason` is one or two sentences. Answer only with the JSON object the schema requests.
