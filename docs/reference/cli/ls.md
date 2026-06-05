# wpm ls

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
List installed dependencies

### Aliases

`wpm ls`, `wpm list`, `wpm tree`

### Options

| Name            | Type  | Default | Description                              |
|:----------------|:------|:--------|:-----------------------------------------|
| `-d`, `--depth` | `int` | `-1`    | Max display depth of the dependency tree |


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Print the project's resolved dependency tree as it currently sits in the
lockfile.

`wpm ls` reads `wpm.json` to find the root dependencies, then walks `wpm.lock`
to expand each one into its sub-dependencies. The output is a plain text tree,
sorted alphabetically at every level. It does not touch the network; if you want
to know what's available upstream, use `wpm outdated` instead.

Both `wpm.json` and `wpm.lock` must exist. If you have never installed, run
`wpm install` first.

<!-- prettier-ignore -->
> [!TIP]
> Run `wpm ls` before deploying to a live WordPress site. It shows the
> exact plugin and theme versions you're about to ship, so nothing
> surprises you in production.

### Reading the output

The first line is the package name from `wpm.json` (or the directory name if
`name` is unset). Direct dependencies are grouped by their package type
(`plugin` or `theme`), and each resolved package is drawn beneath its type with
standard tree connectors:

```
my-plugin
├── plugin
│   ├── akismet@5.3.1
│   │   └── jetpack@13.0.0
│   └── query-monitor@3.20.2
└── theme
    └── twentytwentyfour@1.0.0
```

Dependencies missing from the lockfile have no known type and appear under an
`unknown` group.

The annotations on the right of each line tell you about the package's state:

| Annotation                 | Meaning                                                                                                  |
| :------------------------- | :------------------------------------------------------------------------------------------------------- |
| `name@<version>`           | Resolved version from the lockfile.                                                                      |
| `(invalid: "<requested>")` | Lockfile version does not satisfy the version requested in `wpm.json`. Refresh by running `wpm install`. |
| `UNMET DEPENDENCY`         | Listed in `wpm.json` but missing from the lockfile. Run `wpm install`.                                   |
| `(cycle)`                  | Cycle detected while expanding sub-dependencies; recursion stops here.                                   |

The `(invalid: ...)` marker is skipped when `wpm.json` pins the package to `*`
(any version), since any resolved version satisfies it.

### Limiting depth

By default `wpm ls` expands the tree as deep as the lockfile goes. Pass
`-d`/`--depth` to cap the depth. Depth `0` shows only direct dependencies (still
grouped by type); `1` adds their immediate children; and so on. The default,
`-1`, means unlimited. The type headings are always shown.

### Filtering by type

Pass `theme` or `plugin` as a positional argument to restrict the output to a
single type. Grouping and depth behave the same; only the matching type is
shown.

### Comparison with related commands

- Use `wpm ls` when you want the full shape of what's installed.
- Use `wpm why <package>` to trace one package back to the root.
- Use `wpm outdated` to check what could be upgraded.

### Troubleshooting

- `no wpm.json found, so nothing to list`: run the command from the project
  root, or run `wpm init` first.
- `no wpm.lock found, you need to run 'wpm install' first`: the lockfile is
  missing. Run `wpm install` to generate it.
- `no dependencies found in wpm.json`: `wpm.json` declares neither
  `dependencies` nor `devDependencies`. Add some, or you have nothing to list.
- `UNMET DEPENDENCY` lines appear after editing `wpm.json` by hand without
  re-running install. Run `wpm install`.

## Examples

### Print the full tree

```console
$ wpm ls
my-plugin
├── plugin
│   ├── akismet@5.3.1
│   │   └── jetpack@13.0.0
│   ├── hello-dolly@1.7.2
│   └── query-monitor@3.20.2
└── theme
    └── twentytwentyfour@1.0.0
```

### List only one type

```console
$ wpm ls plugin
my-plugin
└── plugin
    ├── akismet@5.3.1
    └── query-monitor@3.20.2
```

### Limit the tree to direct dependencies

```console
$ wpm ls --depth 0
my-plugin
├── plugin
│   ├── akismet@5.3.1
│   ├── hello-dolly@1.7.2
│   └── query-monitor@3.20.2
└── theme
    └── twentytwentyfour@1.0.0
```

### Show one level of transitive dependencies

```console
$ wpm ls -d 1
my-plugin
└── plugin
    ├── akismet@5.3.1
    │   └── jetpack@13.0.0
    ├── hello-dolly@1.7.2
    └── query-monitor@3.20.2
```

### Spot an unmet dependency

A package listed in `wpm.json` but missing from the lockfile has no known type,
so it appears under `unknown`.

```console
$ wpm ls
my-plugin
├── plugin
│   └── akismet@5.3.1
└── unknown
    └── hello-dolly@1.7.2 UNMET DEPENDENCY
```

### Spot a version drift

The lockfile resolved `1.7.2`, but `wpm.json` now requests `1.7.3`. Run
`wpm install` to re-resolve.

```console
$ wpm ls
my-plugin
└── plugin
    └── hello-dolly@1.7.2 (invalid: "1.7.3")
```

### Spot a cycle

A cycle is annotated and not expanded again under itself.

```console
$ wpm ls
my-plugin
└── plugin
    └── package-a@1.0.0
        └── package-b@1.0.0
            └── package-a@1.0.0 (cycle)
```
