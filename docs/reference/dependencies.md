# Dependencies

`dependencies` and `devDependencies` are the two dependency maps in `wpm.json`.
They tell wpm which packages to install, at which versions, and in which role
(production or development).

## Shape

Both fields are JSON objects whose keys are package names and whose values are
version specifiers. They are completely independent maps, not arrays.

```json
{
	"dependencies": {
		"akismet": "5.3.1",
		"hello-dolly": "*"
	},
	"devDependencies": {
		"query-monitor": "3.20.2"
	}
}
```

A package lives in one map or the other, never both. When `wpm install` moves a
package, the old entry goes away.

## Allowed values

| Form      | Meaning                                                                                  |
| :-------- | :--------------------------------------------------------------------------------------- |
| `"X.Y.Z"` | A strict SemVer version. wpm fetches exactly this build.                                 |
| `"*"`     | Any version. wpm resolves it to whatever the registry returns for the `latest` dist tag. |

That's it. The dependency value does not accept SemVer ranges, carets, or
tildes. This is a deliberate design choice, not a limitation we plan to remove.

<!-- prettier-ignore -->
> [!IMPORTANT]
> WordPress plugins and themes are not pure libraries. They can
> change database schema, write options, register cron jobs, and modify content.
> A solver cannot tell whether a "compatible" upgrade is safe for _your_ site,
> because that answer depends on runtime state the solver cannot see. You are
> the only one who can make that call.

So wpm doesn't let the resolver decide for you. You choose every version
deliberately. Every install reproduces the exact tree you last committed.

A few consequences:

- The version saved in `wpm.json` is the version the registry returned, not the
  one you typed. Ask for `akismet@latest`, the registry returns `5.3.1`, and
  `wpm.json` records `"akismet": "5.3.1"`.
- Ranges are reserved for `requires.wp` and `requires.php` (which describe what
  your package needs from WordPress and PHP), not for picking dependencies. See
  [Runtime compatibility](runtime.md).
- Before you deploy to a live site, run `wpm ls` to see the exact versions
  you're about to ship. No surprises hit production.

If you want "newest in 1.x" behavior, run `wpm install pkg@latest` whenever you
want to refresh. The trigger is always explicit, and the new version becomes a
clear change in your commit history.

## Production vs development

`dependencies` are installed every time. `devDependencies` are installed by
default but skipped (and pruned from disk) when `wpm install --no-dev` is used.
The typical mapping:

| Map               | Examples                                                                                 |
| :---------------- | :--------------------------------------------------------------------------------------- |
| `dependencies`    | Packages your code requires at runtime: integrations, sibling plugins you depend on.     |
| `devDependencies` | Packages used only during development: debugging tools, code quality tools, sample data. |

If you move a package from `dependencies` to `devDependencies`, the next
`wpm install --no-dev` will delete it from `wp-content/`.

## How entries are added

You rarely edit these maps by hand. `wpm install` writes them for you. The
placement rules:

| Command                                | Where the entry lands                                                                 |
| :------------------------------------- | :------------------------------------------------------------------------------------ |
| `wpm install <pkg>`                    | Stays in `devDependencies` if it was already there; otherwise goes to `dependencies`. |
| `wpm install -P <pkg>` (`--save-prod`) | `dependencies`. Removed from `devDependencies` if present.                            |
| `wpm install -D <pkg>` (`--save-dev`)  | `devDependencies`. Removed from `dependencies` if present.                            |

wpm writes the resolved version, not the one you typed. So
`wpm install akismet@latest` ends up as `"akismet": "5.3.1"` (or whatever the
registry returns). That's what keeps installs identical across machines.

## How entries are removed

`wpm uninstall <pkg>` deletes the entry from both maps (whichever one has it)
and rewrites `wpm.json`. The same call also reconciles the lockfile and the
filesystem, removing anything no longer reachable from the root.

If you want to remove a package from `wpm.json` but keep the extracted files on
disk, edit the file directly and skip running install. `wpm install` will
reconcile the disk on its next run, so that state is temporary.

## Limits

Each map is capped at 16 entries. If you hit this limit:

- Look for accidental duplicates or per-environment packages that should be
  conditional, not committed.
- Consider whether some "dependencies" are actually transitive and could be
  removed (wpm resolves the full tree from the lockfile).
- If you still need more, split the package into smaller packages with their own
  manifests.

## Versions, tags, and the resolver

`wpm install pkg@<value>` parses the part after `@` in this order:

1. **A valid SemVer** (`X.Y.Z`, no `v` prefix). wpm uses it directly as a
   version request.

2. **A dist tag**. Same character rules as a package name, up to 64 characters.
   The registry resolves it to a specific version.

3. **Anything else** is rejected.

Either way, `wpm.json` ends up with a concrete version, not the tag. To switch
versions later, run `wpm install pkg@<new>` again.

When two parts of the dependency tree disagree about a version, `wpm install`
runs conflict resolution. The summary:

- **Your `dependencies` entry wins** over transitive requests, as long as it
  satisfies them.
- **Strictly lower** root pins than a transitive needs produce a "version
  downgrade" error. Bump your pin.
- **No root pin** plus disagreeing transitives produces an "unresolvable
  conflict" error. Add an explicit entry to `dependencies` to break the tie.

See the [`wpm install`](../cli/install.md) reference for the full output format.

## Sharing across the tree

wpm keeps one copy of each package per project, shared across everything that
depends on it. Two packages that need the same library end up using the same
copy on disk; there is no per-dependency private copy.

The shared copy is whichever version the resolver settled on: your root pin if
any, otherwise the version the resolver visited first. This is why the resolver
is strict about conflicts; there is no fallback to a package-specific copy.

## Related

- [`wpm install`](../cli/install.md): the command that reads and writes the
  maps.
- [`wpm uninstall`](../cli/uninstall.md): removes entries and reconciles disk.
- [`wpm.lock`](../reference/wpm-lock.md): the resolved snapshot.
- [Runtime compatibility](runtime.md): for `requires` constraints, which are
  different from dependency specifiers.
