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

A package belongs to one map or the other, never both. When you ask
`wpm install` to move it, the entry is removed from the original map in the same
write.

## Allowed values

| Form      | Meaning                                                                                  |
| :-------- | :--------------------------------------------------------------------------------------- |
| `"X.Y.Z"` | A strict SemVer version. wpm fetches exactly this build.                                 |
| `"*"`     | Any version. wpm resolves it to whatever the registry returns for the `latest` dist tag. |

That's it. The dependency value does not accept SemVer ranges, carets, or
tildes. Two reasons:

- wpm pins to exact versions for reproducibility. The version recorded in
  `wpm.json` is the version that was actually resolved, not the request that
  produced it.
- Ranges are reserved for `requires.wp` and `requires.php` (the compatibility
  constraints your package imposes on its host), not for dependency selection.
  See [Runtime compatibility](runtime.md).

If you need a range-style "newest in 1.x" effect, run `wpm install pkg@latest`
whenever you want to refresh, or maintain the pin yourself when you want to
upgrade.

## Production vs development

`dependencies` are installed every time. `devDependencies` are installed by
default but skipped (and pruned from disk) when `wpm install --no-dev` is used.
The typical mapping:

| Map               | Examples                                                                                 |
| :---------------- | :--------------------------------------------------------------------------------------- |
| `dependencies`    | Packages your code requires at runtime: integrations, sibling plugins you depend on.     |
| `devDependencies` | Packages used only during development: debugging tools, code quality tools, sample data. |

If you split a previously-prod dependency into a dev-only one, the next
`wpm install --no-dev` removes its extracted files.

## How entries are added

You rarely edit these maps by hand. `wpm install` writes them for you. The
placement rules:

| Command                                | Where the entry lands                                                                 |
| :------------------------------------- | :------------------------------------------------------------------------------------ |
| `wpm install <pkg>`                    | Stays in `devDependencies` if it was already there; otherwise goes to `dependencies`. |
| `wpm install -P <pkg>` (`--save-prod`) | `dependencies`. Removed from `devDependencies` if present.                            |
| `wpm install -D <pkg>` (`--save-dev`)  | `devDependencies`. Removed from `dependencies` if present.                            |

The version that wpm writes is the version the registry resolved, not the
specifier you typed. Asking for `akismet@latest` records the specific build the
registry returned, for example `"akismet": "5.3.1"`. This is what makes installs
reproducible across machines.

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

1. If `<value>` is a valid SemVer (`X.Y.Z`, no `v` prefix), it is treated as a
   version request.
2. Otherwise, if `<value>` looks like a dist tag (the same character rules as a
   package name, up to 64 characters), it is treated as a tag. The registry
   resolves the tag to a specific version.
3. Anything else is rejected.

Either way, what lands in `wpm.json` is a concrete SemVer string, not the tag.
To install a different build later, run `wpm install pkg@<new>` again.

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
- [`wpm.lock`](../wpm-lock/index.md): the resolved snapshot.
- [Runtime compatibility](runtime.md): for `requires` constraints, which are
  different from dependency specifiers.
