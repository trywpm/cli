# wpm ls

<!---MARKER_GEN_START-->

List installed dependencies

### Options

| Name            | Type  | Default | Description                              |
| :-------------- | :---- | :------ | :--------------------------------------- |
| `-d`, `--depth` | `int` | `-1`    | Max display depth of the dependency tree |

<!---MARKER_GEN_END-->

## Description

Print the project's resolved dependency tree as it currently sits in the
lockfile.

`wpm ls` reads `wpm.json` to find the root dependencies, then walks `wpm.lock`
to expand each one into its sub-dependencies. The output is a plain text tree,
sorted alphabetically at every level. It does not touch the network; if you want
to know what's available upstream, use `wpm outdated` instead.

Both `wpm.json` and `wpm.lock` must exist. If you have never installed, run
`wpm install` first.

### Reading the output

The first line is the package name from `wpm.json` (or the directory name if
`name` is unset). Each child line is one resolved package, drawn with standard
tree connectors:

```
my-plugin
├── akismet@5.3.1
│   └── jetpack@13.0.0
└── query-monitor@3.20.2
```

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
`-d`/`--depth` to cap the depth. Depth `0` shows only direct dependencies; `1`
adds their immediate children; and so on. The default, `-1`, means unlimited.

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
├── akismet@5.3.1
│   └── jetpack@13.0.0
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

### Limit the tree to direct dependencies

```console
$ wpm ls --depth 0
my-plugin
├── akismet@5.3.1
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

### Show one level of transitive dependencies

```console
$ wpm ls -d 1
my-plugin
├── akismet@5.3.1
│   └── jetpack@13.0.0
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

### Spot an unmet dependency

```console
$ wpm ls
my-plugin
├── akismet@5.3.1
└── hello-dolly@1.7.2 UNMET DEPENDENCY
```

### Spot a version drift

The lockfile resolved `1.7.2`, but `wpm.json` now requests `1.7.3`. Run
`wpm install` to re-resolve.

```console
$ wpm ls
my-plugin
└── hello-dolly@1.7.2 (invalid: "1.7.3")
```

### Spot a cycle

A cycle is annotated and not expanded again under itself.

```console
$ wpm ls
my-plugin
└── package-a@1.0.0
    └── package-b@1.0.0
        └── package-a@1.0.0 (cycle)
```
