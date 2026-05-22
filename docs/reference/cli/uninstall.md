# wpm uninstall

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Remove dependencies from the project

### Aliases

`wpm uninstall`, `wpm remove`, `wpm rm`


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Remove one or more packages from `wpm.json` and from your project's content
directory.

`wpm uninstall` is the inverse of `wpm install`. For each name you pass, it
deletes the entry from both `dependencies` and `devDependencies` in `wpm.json`,
then runs the same install pipeline as `wpm install` to bring the lockfile and
the filesystem back in sync. Transitive packages that nothing else depends on
are removed too; shared dependencies are kept.

The command requires at least one package name. It does not match by pattern.

### Aliases

`wpm remove` and `wpm rm` are aliases. They behave identically and are handy
when "uninstall" feels too long or "remove" reads better in your shell history.

### What happens on disk

Removing a package is the same operation as installing with that package missing
from `wpm.json`:

1. Acquire the workspace lock at the project's content directory so two wpm
   processes do not fight.
2. Delete the named packages from `wpm.json` (in memory).
3. Re-resolve the dependency tree using the updated config and the existing
   lockfile.
4. Compute a plan of `Remove` actions for anything no longer reachable, plus any
   drift the resolver finds.
5. Apply the plan, rewrite `wpm.lock`, and save `wpm.json`.

Because step 3 reuses the resolver, the lockfile stays consistent even when
transitive packages disappear or reshuffle.

### No-op behavior

If none of the names you pass appear in `wpm.json`, the command exits cleanly
with `No matching packages found to uninstall.` It does not touch the lockfile
or the filesystem. This is intentional: it lets scripts call `wpm uninstall`
without checking first whether a package is present.

### Troubleshooting

- `no wpm.json found, so nothing to uninstall`: run from the project root.
- `failed to acquire workspace lock`: another wpm process is running in the same
  workspace. Wait for it to finish.
- `No matching packages found to uninstall.`: the names you passed are not in
  `wpm.json`. Check spelling, or use `wpm ls` to see what's actually installed.
- If you only want to remove a package from `wpm.json` but leave the files on
  disk, edit `wpm.json` by hand. `wpm uninstall` always reconciles disk with the
  new config.

## Examples

### Remove one package

```console
$ wpm uninstall hello-dolly
wpm uninstall v0.1.0

- hello-dolly 1.7.2

1 package uninstalled
```

### Remove several at once

```console
$ wpm rm akismet hello-dolly query-monitor
wpm uninstall v0.1.0

- akismet 5.3.1
- hello-dolly 1.7.2
- query-monitor 3.20.2

3 packages uninstalled
```

### Remove with the `remove` alias

```console
$ wpm remove akismet
wpm uninstall v0.1.0

- akismet 5.3.1

1 package uninstalled
```

### When nothing matches

```console
$ wpm uninstall does-not-exist
wpm uninstall v0.1.0

No matching packages found to uninstall.
```
