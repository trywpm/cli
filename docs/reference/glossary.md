# Glossary

Terms that appear across the wpm docs. Listed alphabetically.

## Access level

The visibility a package has on the registry. Set with
`wpm publish --access <public|private>`. A `public` package is visible to
everyone the registry serves; a `private` package is visible only to authorized
accounts. Distinct from `"private": true` in `wpm.json`, which prevents
publishing entirely. See [Registry concepts](../reference/registry.md).

## Auth token

The opaque string the registry issues to identify you. wpm sends it on every
authenticated request. Stored in `~/.wpm/config.json` after `wpm auth login`.
Equivalent to a password; treat it accordingly. See
[Authentication](../guide/authentication.md).

## Conflict resolution

The resolver's process for deciding which version to pick when two parts of the
dependency tree disagree. Root pins in `wpm.json` win over transitive requests,
as long as they satisfy the requests. Unresolvable conflicts produce an error
pointing at the responsible packages. See [`wpm install`](../cli/install.md).

## Content directory

The directory wpm extracts dependencies into. Default `wp-content/`.
Configurable via `config.content-dir` in `wpm.json`. Inside it, packages are
organized into `plugins/`, `themes/`, and `mu-plugins/` subdirectories based on
each package's `type`.

## Dev dependency

A package declared in the `devDependencies` map of `wpm.json`. Installed by
default but pruned with `wpm install --no-dev`. Use for development-only tools
(debuggers, code quality checks, sample data). See
[Dependencies](../reference/dependencies.md).

## Digest

The SHA-256 hash of a published tarball. Recorded in `wpm.lock` and on every
entry in the registry's manifest. wpm uses the digest to verify integrity after
downloading.

## Direct dependency

A package listed in `dependencies` or `devDependencies` in your `wpm.json`. The
opposite of _transitive dependency_, which is a dependency of another package in
your tree.

## Dist tag

A label that points at a specific version of a package on the registry. The
canonical tag is `latest`, which marks the version new consumers should pick up.
Maintainers can create others (`beta`, `next`, `lts`) to publish pre-releases
without disturbing `latest`. Set with `wpm publish --tag <name>`; selected at
install time with `wpm install pkg@<tag>`. See
[Registry concepts](../reference/registry.md).

## Lockfile

`wpm.lock`. The frozen snapshot of the resolved dependency tree, including exact
versions, tarball URLs, and SHA-256 digests. Commit to version control. See
[`wpm.lock`](../reference/wpm-lock.md).

## Lockfile version

The integer field at the top of `wpm.lock` that identifies the file format's
revision. Currently `1`. wpm refuses to load a lockfile with a newer version
than it understands.

## Manifest

Two related things in wpm, distinguished by context:

- **Project manifest**: your `wpm.json`. Describes the local package and its
  dependencies.
- **Registry manifest**: the JSON document the registry serves for a published
  version. Includes everything in the project manifest plus distribution
  metadata (tag, digest, sizes, visibility).

## Mu-plugin

A "must-use" plugin. Installed under `wp-content/mu-plugins/` and loaded
automatically by WordPress on every request. Cannot be deactivated from the
admin UI. See [Package types](../reference/package-types.md).

## Package name

The unique identifier for a package on the registry. 3 to 164 characters,
lowercase alphanumeric and hyphens. Matched by the regex
`^[a-z0-9]+(-[a-z0-9]+)*$`. See
[`wpm.json` reference](../reference/wpm-json.md).

## Package type

One of `plugin`, `theme`, or `mu-plugin`. Declared in `wpm.json`, required for
every package. Determines where wpm extracts the package and how WordPress loads
it. See [Package types](../reference/package-types.md).

## Plan

The list of `Install`, `Update`, and `Remove` actions that `wpm install`
calculates by comparing the resolved tree against `wpm.lock` and the filesystem.
Printed line by line during the install. Use `--dry-run` to see the plan without
applying it.

## Pre-release

A SemVer version with a pre-release suffix, like `1.0.0-beta.1`. Sorts below the
corresponding release in SemVer ordering, so `wpm outdated` will not flag a
pre-release as an update over the release.

## Private

Two distinct meanings, distinguished by context:

- `"private": true` in `wpm.json` prevents publishing entirely. A tripwire on
  internal-only projects.
- `--access private` on `wpm publish` publishes the package with private
  visibility on the registry.

See [Registry concepts](../reference/registry.md).

## Registry

The server wpm talks to. Hosts published packages, dist tags, manifests, and
tarballs. Default `registry.wpm.so`. Switch with `--registry <url>`. See
[Registry concepts](../reference/registry.md).

## Resolution

The process of expanding a `wpm.json` into a flat set of resolved packages, by
recursively fetching each dependency's manifest from the registry. Performed by
`wpm install`. The output is cached in `wpm.lock`.

## Resolved version

The concrete version that the resolver picked for a package, after applying
conflict resolution. Recorded in `wpm.lock` as the `version` field, and copied
into `wpm.json` whenever the package is added via the CLI.

## Runtime (strict mode)

The opt-in compatibility check `wpm install` performs when `config.runtime.wp`
or `config.runtime.php` is set in `wpm.json`. Every dependency's `requires` is
checked against your declared runtime; mismatches fail the install. See
[Runtime compatibility](../reference/runtime.md).

## SemVer

[Semantic Versioning](https://semver.org/) (`X.Y.Z`). wpm requires strict SemVer
for `version` and for entries in `dependencies` (or `*`). Range syntax (`^1.0`,
`~1.2`, `>=6.0 <7.0`) is only allowed in `requires.wp` and `requires.php`.

## Tarball

The compressed archive that the registry stores for each published version.
Format is Zstandard-compressed tar (`.tar.zst`). Capped at 128 MiB at publish
time.

## Transitive dependency

A dependency of one of your direct dependencies. Reachable through the
dependency graph but not declared directly in your `wpm.json`. Trace one to its
root with `wpm why <pkg>`.

## Visibility

See _Access level_.

## Workspace lock

A file-based lock wpm holds inside your project's content directory while it
runs install or uninstall. Prevents two wpm processes from corrupting each other
in the same project. Distinct from `wpm.lock` (the lockfile). Released when the
holding process exits.

## wpm.json

The project manifest. Lives at the root of every wpm package. Required fields
are `name`, `version`, `type`. See
[`wpm.json` reference](../reference/wpm-json.md).

## wpm.lock

The lockfile. See _Lockfile_ and [`wpm.lock`](../reference/wpm-lock.md).

## .wpmignore

A gitignore-style file controlling which paths `wpm publish` includes in the
published tarball. Optional. See [`.wpmignore`](../reference/wpmignore.md).
