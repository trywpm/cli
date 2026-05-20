# wpm why

<!---MARKER_GEN_START-->
Show why a package is installed


<!---MARKER_GEN_END-->

## Description

Explain which chain of dependencies pulled a specific package into the
project.

`wpm why <package>` traces backwards through the lockfile from the
target package to the root of `wpm.json`. It prints every distinct
chain it finds. Use it when you're surprised to see something in
`wpm ls` and want to know which root dependency is responsible.

The command needs exactly one package name. It does not contact the
registry; everything is computed from `wpm.json` and `wpm.lock`. The
target must already exist in `wpm.lock`. If it doesn't, you probably
need to run `wpm install` first.

### Reading the output

Each chain is rendered top-down, starting at the project root and
ending at the package you asked about. Versions are pulled from
`wpm.lock`. The root node shows whether the entry sits in
`dependencies` or `devDependencies`:

```
my-plugin (dependencies)
└─ akismet@5.3.1
   └─ jetpack@13.0.0
      └─ query-monitor@3.20.2
```

If the package is reachable from more than one root path (for example,
both a direct dependency and a transitive one), each path is printed in
turn, separated by a blank line.

The root label uses the `name` field from `wpm.json` (or the directory
name if `name` is unset). The suffix `(dependencies)` or
`(devDependencies)` tells you which section of `wpm.json` the chain
starts from.

### Orphaned packages

If the target exists in `wpm.lock` but no chain reaches a root entry in
`wpm.json` (this can happen if you edit `wpm.json` by hand without
re-running install), wpm prints:

```
<package> is present in lockfile but has no apparent dependents (orphaned?).
```

Run `wpm install` to clean up. The next install drops anything the root
doesn't pull in.

### Cycles

If the dependency graph contains a cycle, the backward traversal
detects it and refuses to add a parent that's already in the current
path. As a result, cycles do not produce duplicated paths in the
output.

### Comparison with related commands

- `wpm ls` shows the *whole* tree forward from the root.
- `wpm why` shows *paths* backward from one package to the root.

If you're inspecting the project broadly, start with `wpm ls`. If you
already see the package and want to know who pulled it in, use
`wpm why`.

### Troubleshooting

- `no wpm.json found in the current directory`: run from the project
  root.
- `no wpm.lock found. Run 'wpm install' first to generate a lockfile.`:
  generate the lockfile first.
- `package '<name>' is not found in wpm.lock`: the name is wrong or the
  package was never installed. Check `wpm ls` for the exact name.

## Examples

### A direct dependency

```console
$ wpm why akismet
my-plugin (dependencies)
└─ akismet@5.3.1
```

### A transitive dependency

`query-monitor` is pulled in indirectly through `akismet` and
`jetpack`.

```console
$ wpm why query-monitor
my-plugin (dependencies)
└─ akismet@5.3.1
   └─ jetpack@13.0.0
      └─ query-monitor@3.20.2
```

### A package pulled by both a direct and transitive path

```console
$ wpm why jetpack
my-plugin (dependencies)
└─ jetpack@13.0.0

my-plugin (dependencies)
└─ akismet@5.3.1
   └─ jetpack@13.0.0
```

### A dev-only dependency

```console
$ wpm why query-monitor
my-plugin (devDependencies)
└─ query-monitor@3.20.2
```

### An orphan in the lockfile

```console
$ wpm why stale-package
stale-package is present in lockfile but has no apparent dependents (orphaned?).
```

### A name that isn't installed

```console
$ wpm why typo-name
package 'typo-name' is not found in wpm.lock
```
