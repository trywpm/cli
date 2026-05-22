# wpm.lock

`wpm.lock` records the exact dependency tree your project resolved to. It keeps
installs identical across machines and CI runs.

The next install reads `wpm.lock` first. If the versions you've asked for still
match what's in the file, wpm skips the registry entirely and uses the lockfile
directly.

<!-- prettier-ignore -->
> [!IMPORTANT]
> Commit `wpm.lock` to version control. Without it, installs are
> not reproducible across machines or CI runs.

## Lifecycle

- `wpm install` creates the lockfile on the first run and rewrites it after
  every subsequent install. The packages in the file are always sorted
  alphabetically.
- `wpm uninstall` rewrites the lockfile to remove anything no longer reachable
  from `wpm.json`.
- `wpm ls`, `wpm outdated`, and `wpm why` read the lockfile but do not modify
  it. They refuse to run if it is missing.
- `wpm publish` does not read the lockfile. The published tarball is built from
  your source tree, not from resolved dependencies.

The lockfile is the source of truth for what's installed. Every install
reconciles `wp-content/` against it:

- Delete an extracted plugin or theme by hand, and the next install puts it
  back.
- Delete an entry from the lockfile, and the next install treats the package as
  new and downloads it again.

## File format

`wpm.lock` is a JSON file with two top-level fields:

```json
{
	"lockfileVersion": 1,
	"packages": {
		"akismet": {
			"version": "5.3.1",
			"resolved": "/akismet/5.3.1.tar.zst",
			"digest": "sha256:9j6Q7l8s...",
			"type": "plugin",
			"dependencies": {
				"jetpack": "13.0.0"
			}
		},
		"jetpack": {
			"version": "13.0.0",
			"resolved": "/jetpack/13.0.0.tar.zst",
			"digest": "sha256:1pP4mY7Z...",
			"type": "plugin"
		}
	}
}
```

### Top-level fields

| Field             | Type                        | Notes                                                      |
| :---------------- | :-------------------------- | :--------------------------------------------------------- |
| `lockfileVersion` | integer                     | Current format version. Today this is `1`.                 |
| `packages`        | object<string, LockPackage> | Every package in the resolved tree, keyed by package name. |

The `packages` map is flat. wpm does not nest dependencies under their parents;
both direct and transitive packages live at the same level. This matches how wpm
extracts them under `wp-content/`.

### `LockPackage` entries

Each entry in `packages` has the following shape:

| Field          | Type                   | Notes                                                                              |
| :------------- | :--------------------- | :--------------------------------------------------------------------------------- |
| `version`      | string                 | The resolved SemVer (`X.Y.Z`).                                                     |
| `resolved`     | string                 | The registry-relative path to the tarball (for example, `/akismet/5.3.1.tar.zst`). |
| `digest`       | string                 | The `sha256:<hex>` digest of the tarball, used for integrity verification.         |
| `type`         | string                 | One of `plugin`, `theme`, `mu-plugin`. Determines the extraction directory.        |
| `bin`          | object<string, string> | Optional. Binary mappings declared by the package. Currently informational.        |
| `dependencies` | object<string, string> | Optional. The package's own direct dependencies, copied from its manifest.         |

Because each entry stores its own dependency list, wpm can rebuild the full
install tree without calling the registry. When `wpm install` finds a matching
version in the lockfile, it reads the dependencies straight from the entry
instead of fetching a fresh manifest.

## Reproducibility

Two `wpm install` runs against the same `wpm.json` and `wpm.lock`, on the same
registry, produce the same on-disk state. Concretely:

- The package versions on disk match the `version` field of every lockfile
  entry.
- The tarballs on disk hash to the `digest` field. Mismatches cause the affected
  package to be re-extracted.
- Adding or removing entries from `wpm.json` is the only way to change the
  resolved tree. The lockfile passively records what happened; it does not
  impose its own preferences.

This is why CI builds should always run `wpm install` against a committed
`wpm.lock`: it lets the install step skip resolution entirely when nothing has
changed.

## Forward-compatibility

wpm refuses to load a lockfile whose `lockfileVersion` is newer than the version
it understands. The error reads:

```
wpm upgrade required: lockfile version is newer than this version of wpm
```

When you see this, upgrade wpm. A teammate's newer wpm wrote the lockfile. If
you run with the older version, you'll lose fields the older format doesn't
understand.

## Hand-editing

<!-- prettier-ignore -->
> [!CAUTION]
> Never edit `wpm.lock` by hand in normal use. Use `wpm install` to
> regenerate it from `wpm.json`. Direct edits can desync the lockfile from
> what's actually on disk and produce hard-to-diagnose install failures.

If you've gotten into a state where you think you need to:

- To reset the lockfile, delete it and run `wpm install`. wpm will rebuild it
  from scratch using `wpm.json` and the registry.
- To force a re-extraction of a package, delete its directory under
  `wp-content/`. The next `wpm install` will reinstall it because the lockfile
  entry no longer matches what's on disk.
- To change a resolved version, edit `wpm.json` (not the lockfile) and re-run
  `wpm install`. The resolver picks up the new request and rewrites the lockfile
  accordingly.

If you do edit the file directly, take care to keep `lockfileVersion` at the
supported value and to preserve the required fields on every entry. Invalid
package names in the `packages` map are rejected on load.

## Related

- [`wpm.json`](wpm-json.md): the request that the lockfile records the answer
  to.
- [Dependencies](dependencies.md): how the request is expressed.
- [`wpm install`](../reference/cli/install.md): the command that writes the
  lockfile.
- [`wpm ls`](../reference/cli/ls.md) and [`wpm why`](../reference/cli/why.md):
  the two read-only commands that work from the lockfile.
