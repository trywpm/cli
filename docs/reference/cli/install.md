# wpm install

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Install project dependencies and add new packages

### Aliases

`wpm install`, `wpm i`, `wpm add`

### Options

| Name                    | Type   | Default | Description                                                    |
|:------------------------|:-------|:--------|:---------------------------------------------------------------|
| `--dry-run`             | `bool` |         | Do not write anything to disk                                  |
| `--ignore-scripts`      | `bool` |         | Do not run lifecycle scripts                                   |
| `--network-concurrency` | `int`  | `16`    | Number of concurrent network requests when installing packages |
| `--no-dev`              | `bool` |         | Do not install dev dependencies                                |
| `-D`, `--save-dev`      | `bool` |         | Install package as a dev dependency                            |
| `-P`, `--save-prod`     | `bool` |         | Install package as a production dependency (default)           |


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Install packages into the current project's content directory and sync the
lockfile with the registry.

`wpm install` has two related uses:

- **Without arguments:** install everything declared in `wpm.json`. wpm resolves
  the full dependency tree, downloads the tarballs, extracts them under
  `wp-content/`, and writes `wpm.lock` so the next install is reproducible.
- **With one or more package arguments:** add those packages to `wpm.json` (as
  production or dev dependencies depending on flags), then install them along
  with the rest of the tree.

The aliases `wpm i` and `wpm add` are equivalent to `wpm install`. `add` is
idiomatic when you're introducing new packages; `i` is a shorthand for the plain
install.

### Package specifiers

Each positional argument is a package specifier. Supported forms:

| Form              | Meaning                                             |
| :---------------- | :-------------------------------------------------- |
| `name`            | Same as `name@latest`                               |
| `name@<version>`  | A strict semver version (for example, `1.7.2`)      |
| `name@<dist-tag>` | A registry dist tag (for example, `latest`, `beta`) |

`name` must be 3 to 164 characters, lowercase, alphanumeric with hyphens.
`<version>` must be strict SemVer (`X.Y.Z`, no `v` prefix). Tags follow the same
character rules as names and may be up to 64 characters.

The exact version that the registry resolves the specifier to is what wpm
records in `wpm.json`. If you run `wpm install akismet@latest` and the registry
returns `5.3.1`, `wpm.json` will contain `"akismet": "5.3.1"`, not `"latest"`.
This pinning is intentional. It keeps every install identical.

### Dependency placement

When you add packages, wpm decides whether to place them in `dependencies` or
`devDependencies`:

| Flag                | Effect                                                                      |
| :------------------ | :-------------------------------------------------------------------------- |
| `-D`, `--save-dev`  | Add to `devDependencies`; remove from `dependencies` if present.            |
| `-P`, `--save-prod` | Add to `dependencies`; remove from `devDependencies` if present.            |
| (neither)           | Stay in `devDependencies` if already there; otherwise go to `dependencies`. |

`--save-dev`, `--save-prod`, and `--no-dev` are mutually exclusive. You can also
use `-P` to move an existing dev dependency to production (and `-D` the other
way around) without changing anything else.

### How resolution works

wpm reads `wpm.json` (root direct dependencies) and `wpm.lock` (a frozen
snapshot of the previous resolution), then expands the dependency graph one
level at a time, starting from your direct dependencies:

1. Seed a queue with all root dependencies and dev dependencies.
2. For each request, prefer the lockfile entry when its version matches.
   Otherwise call the registry to fetch the package manifest. Up to 16 manifest
   requests run in parallel.
3. Record the resolved version, tarball URL, and SHA-256 digest, then enqueue
   the package's own dependencies.
4. When the same package is requested at two different versions, run conflict
   resolution (see below).
5. Continue until the queue drains.

Once the tree is resolved, wpm computes a plan by comparing the resolved tree
against the lockfile and the filesystem state under `wp-content/`:

- **Install:** the package is in the resolved tree but absent from the lockfile,
  or present in the lockfile but missing on disk.
- **Update:** the package is in the lockfile but the resolved version or digest
  has changed.
- **Remove:** the package is in the lockfile but no longer in the resolved tree
  (you removed it from `wpm.json`), or `--no-dev` is set and the package is
  dev-only.

If the plan is empty, wpm prints `Already up-to-date!` and exits.

### Conflict resolution

When two parts of the tree disagree about a version, wpm applies these rules:

- **Root wins.** If `wpm.json` directly pins the package, the root's version is
  used. wpm continues silently as long as the root's version is greater than or
  equal to every transitive requirement.
- **Root is too old.** If a transitive dependency requires a higher version than
  the root pin, wpm errors with the upgrade target so you can bump the pin in
  `wpm.json`.
- **No root pin.** When two transitive dependencies request different versions
  and the root does not pin the package, the conflict is unresolvable. wpm
  errors with an `Action:` line suggesting that you add an explicit pin to the
  root.

### Runtime compatibility (opt-in)

If `wpm.json` sets `config.runtime.wp` or `config.runtime.php`, wpm checks every
resolved package's `requires.wp` and `requires.php` constraints against those
runtime versions. A mismatch errors out before anything is downloaded. When
`config.runtime` is not set, this check is skipped entirely.

### Lockfile

`wpm.lock` records the exact version, tarball URL, SHA-256 digest, type,
binaries, and direct dependencies of every package in the resolved tree. After a
successful install it is rewritten in alphabetical order. Commit this file to
version control. It keeps installs reproducible across machines and CI, and wpm
uses it to skip network calls when the recorded versions still satisfy
`wpm.json`.

### Workspace locking

wpm holds a file lock under the project's content directory while it runs. If
you start a second `wpm install` in the same workspace before the first one
finishes, the second one waits and prints:

```
waiting for another wpm process to finish in this workspace...
```

The lock is released when the first process exits, successfully or not.

### Dry runs

`--dry-run` calculates the plan and prints it without touching the filesystem,
the lockfile, or `wpm.json`. The output ends with a summary phrased as
`N packages can be installed`. Use this to preview changes before committing to
them.

### Tuning concurrency

`--network-concurrency` (default `16`) caps how many tarballs wpm downloads in
parallel during installation, and how many manifests it fetches when adding new
packages with explicit arguments. The resolution phase itself is capped at 16
parallel requests internally. Increase the flag on fast networks; decrease it on
flaky or rate-limited registries.

### Lifecycle scripts

`--ignore-scripts` is reserved for an upcoming lifecycle scripts feature and is
safe to pass today. It currently has no observable effect because script
execution is not yet wired up.

### Troubleshooting

- `failed to acquire workspace lock`: another wpm process holds the lock. Wait
  for it to finish, or check for stale lock files in the content directory if no
  other process is running.
- `Dependency version conflict for package <name>`: two transitive dependencies
  want incompatible versions and there is no root pin. Add the package to
  `dependencies` in `wpm.json` to force the version, as the `Action:` line
  suggests.
- `Version downgrade detected for package <name>`: a transitive dependency needs
  a newer version than your root pin. Bump the pin in `wpm.json`.
- `package <name> incompatible: requires <X> <constraint>, but runtime <X> version is <Y>`:
  runtime strict mode is on and a package does not match your declared runtime.
  Loosen `config.runtime` or drop the offending dependency.
- `invalid package name`: the specifier is wrong. Confirm spelling and remember
  that names are lowercase with hyphens; underscores and uppercase letters are
  rejected.
- `invalid version or tag`: the value after `@` is neither valid semver nor a
  valid dist tag. Use `1.2.3` style or a known tag like `latest`.
- **Install feels slow**: the registry may be far or under load, the HTTP cache
  may be cold, or the resolver may be paying for repeated conflict lookups. Tune
  `--network-concurrency` (default `16`), let the cache warm up after one
  install, or pin conflicting transitives explicitly in `dependencies` to
  short-circuit lookups.
- **`Already up-to-date!` after editing `wpm.json`**: the resolved set didn't
  change. Maybe you changed formatting but not values. Verify with `wpm ls`. If
  you bumped a version, double-check the new value is valid SemVer; invalid
  versions are rejected at validation time.
- **Force a clean reinstall** when the tree is in a confusing state:
  ```sh
  rm -rf wpm.lock wp-content/plugins wp-content/themes
  wpm install
  ```
  Review the new `wpm.lock` before committing.
- **Clear the registry response cache** to force fresh manifest fetches:
  `rm -rf ~/.wpm/cache`. The lockfile and `wp-content/` are untouched.

## Examples

### Install everything in `wpm.json`

```console
$ wpm install
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2
+ query-monitor 3.20.2

3 packages installed
```

### Install only production dependencies

Use this in production builds and CI deploy steps to avoid pulling dev tools you
do not need at runtime. Any dev-only packages currently on disk are removed.

```console
$ wpm install --no-dev
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2

2 packages installed
```

### Add a new package at its latest version

```console
$ wpm add akismet
wpm install v0.1.0

+ akismet 5.3.1

1 package installed
```

### Add a package at a pinned version

```console
$ wpm install hello-dolly@1.7.2
wpm install v0.1.0

+ hello-dolly 1.7.2

1 package installed
```

### Add a dev dependency

```console
$ wpm install -D query-monitor@latest
wpm install v0.1.0

+ query-monitor 3.20.2

1 package installed
```

### Add multiple packages at once

Wpm fetches each manifest in parallel (capped by `--network-concurrency`).

```console
$ wpm add akismet hello-dolly@1.7.2 query-monitor
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2
+ query-monitor 3.20.2

3 packages installed
```

### Move a dev dependency to production

Pass the same package name with `-P`. wpm rewrites `wpm.json` to remove it from
`devDependencies` and add it to `dependencies` at the version the registry
returns.

```console
$ wpm install -P query-monitor
wpm install v0.1.0

1 package installed
```

### Preview changes without writing anything

```console
$ wpm install --dry-run
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2

2 packages can be installed
```

### Re-run when everything is current

```console
$ wpm install
wpm install v0.1.0
Already up-to-date!
```

### Tune network concurrency for a slow registry

```console
$ wpm install --network-concurrency 4
```

### When another wpm process is already running

```console
$ wpm install
wpm install v0.1.0
waiting for another wpm process to finish in this workspace...
+ akismet 5.3.1

1 package installed
```
