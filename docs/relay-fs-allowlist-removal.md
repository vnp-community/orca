# Relay FS Allowlist Removal

**Status:** Decided and implemented (`agent/src/relay/context.ts`, `RelayContext`)

## What was removed

An earlier version of the SSH-relay daemon (`agent/src/relay/relay.ts` +
`RelayContext`) enforced a **workspace root allowlist** for `fs.*` RPC
methods: the client (Orca) called `session.registerRoot(rootPath)` once per
workspace, and every subsequent `fs.readFile`/`fs.writeFile`/`fs.readDir`/…
call was checked against the registered root(s) before touching disk.

That enforcement has been removed. `RelayContext.registerRoot()` is now an
intentional **no-op** — see the comment at the top of `context.ts`. The RPC
methods (`session.registerRoot` notification and request forms) are still
registered and still return success, purely so an old client talking to a
new relay (or a new client talking to an old relay, during a rolling
deploy) never sees a spurious "Method not found" error. Tracked for deletion
once the relay-version floor moves past the cutover that introduced this
change.

## Why

The relay process runs **as the SSH user**, on a host the user (or their
organization) already controls, and trusts the renderer/backend process it's
connected to — the same trust boundary as every other RPC method on this
channel. Given that trust boundary, the FS allowlist did not meaningfully
narrow the blast radius of a compromised renderer, because two other
RPC surfaces on the *same connection* already grant equivalent-or-greater
filesystem reach without any comparable allowlist:

- **`pty.spawn`** hands the caller an interactive shell as the SSH user —
  trivially able to read/write/delete anything that user can, from inside
  the shell itself.
- **`git.exec`** (see
  [`specs/agent/api/agent-rpc-catalog-git-fs.md`](../specs/agent/api/agent-rpc-catalog-git-fs.md))
  runs a real `git` subprocess with the SSH user's filesystem permissions;
  several of its allowed subcommands (`config --get -f <file>` before that
  specific flag was denied, `log`, `show`) can already read file content
  outside any "workspace root" concept.

A separate FS-path allowlist checked only on the `fs.*` methods was
therefore friction for legitimate multi-root/multi-workspace use (every new
root needed an explicit `registerRoot` round-trip) without closing a real
attack surface — an attacker who can reach `fs.*` at all can already reach
`pty.spawn`/`git.exec` on the same authenticated channel.

## What's still enforced (the real boundary)

Removing the FS allowlist did **not** remove every safety check on this
channel — the checks that actually reduce risk stayed:

- **`git.exec`'s command whitelist** (`agent/src/relay/git-exec-validator.ts`)
  — a fixed, small set of subcommands, most of them read-only, with explicit
  per-flag denylists (e.g. `config`'s `--file`/`-f` is denied specifically
  *because* it can redirect a read to an arbitrary file).
- **`git.clone`'s argument validation** — both param shapes (`{args, cwd,
  progressId}` and `{url, targetPath}`) reject a leading `-` to prevent argv
  injection (a `-`-prefixed "URL" being interpreted as a git flag). See
  [`gaps-and-findings.md`](../specs/agent/api/gaps-and-findings.md) #3 for
  where the `{url, targetPath}` shape's validation was added.
- **Windows shell-override restriction** (`pty.spawn`'s
  `ALLOWED_WINDOWS_SHELL_OVERRIDES`) — bounds which shell binaries a caller
  can request, platform-specific hardening unrelated to the FS allowlist.
- **Terminal-artifact identity verification**
  (`fs.readTerminalArtifact`/`fs.writeTerminalArtifact`) — a TOCTOU-safe
  check that the file being read/written is *exactly* the one a prior
  operation (e.g. a screenshot save) produced, via realpath + dev/ino/nlink/
  size/mtime matching. This is orthogonal to workspace confinement — it's
  about not following a symlink swapped in between grant and use, not about
  restricting *which* directories are reachable.

## Current `fs.*` surface

The full, current `fs.*` method catalog (both the direct-WebSocket and
SSH-relay implementations) is documented in
[`specs/agent/api/agent-rpc-catalog-git-fs.md`](../specs/agent/api/agent-rpc-catalog-git-fs.md),
which states this same trust-boundary rationale inline for each handler that
has no per-path check.
