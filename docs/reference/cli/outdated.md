# wpm outdated

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Check for outdated dependencies


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Report which of your installed dependencies have newer versions on the registry.

For every package that appears in both `wpm.json` and `wpm.lock`, `wpm outdated`
fetches the `latest` dist tag from the registry, compares the resolved version
to the lockfile, and lists anything that's behind. Up to 16 packages are checked
in parallel.

`wpm outdated` only reads. It never changes your project. To upgrade a flagged
package, install it again at the new version:

```console
$ wpm install akismet@5.4.0
```

### Reading the output

Each outdated package is printed as a small two-line block:

```
akismet [plugin]
├── current: 5.3.1
└── latest:  5.4.0 (minor update)
```

- The first line shows the package name, its `[type]` (`plugin` or `theme`), and
  a faint `(dev)` marker when the package lives in `devDependencies`.
- `current` is the version recorded in `wpm.lock`.
- `latest` is what the registry returns for the `latest` tag.

The label after `latest` tells you the severity of the bump, classified purely
by semver:

| Label              | Meaning                                             | Color in terminals |
| :----------------- | :-------------------------------------------------- | :----------------- |
| `(major update)`   | Major version changed; breaking changes are likely. | Red                |
| `(minor update)`   | Minor version changed; new features, no breakage.   | Yellow             |
| `(patch update)`   | Patch version changed; bug fixes only.              | Green              |
| `(unknown update)` | Versions don't parse as semver; can't classify.     | Default            |

When nothing is behind, `wpm outdated` prints `Already up-to-date!`.

### Caveats

- A package is only checked if it appears in both `wpm.json` and `wpm.lock`.
  Adding a dependency to `wpm.json` without running `wpm install` first will
  leave it out of the report.
- "Latest" is whatever the registry returns for the `latest` dist tag. If the
  package maintainer has not tagged anything, the entry is silently skipped.
- Pre-release versions sort below their release counterparts in semver, so
  `1.2.0-beta.1` will not flag as newer than `1.2.0`.

### Troubleshooting

- `no wpm.json found, so nothing to check`: run from the project root.
- `no wpm.lock found. Run 'wpm install' first to generate a lockfile`: the
  lockfile is missing. Install dependencies first so we have something to
  compare against.
- Output stays empty with no `Already up-to-date!` line: every package in
  `wpm.json` is missing from `wpm.lock`. Run `wpm install` to seed the lockfile.

## Examples

### Check for updates

```console
$ wpm outdated
wpm outdated v0.1.0

akismet [plugin]
├── current: 5.3.1
└── latest:  5.4.0 (minor update)

hello-dolly [plugin]
├── current: 1.7.2
└── latest:  1.7.3 (patch update)

query-monitor [plugin] (dev)
├── current: 3.20.2
└── latest:  4.0.0 (major update)
```

### Everything is current

```console
$ wpm outdated
wpm outdated v0.1.0

Already up-to-date!
```

### Upgrade a flagged package

`wpm outdated` only reports. To act on the report, re-install at the desired
version:

```console
$ wpm install akismet@5.4.0
```

Or bump the entry in `wpm.json` and re-run `wpm install`.
