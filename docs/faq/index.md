# FAQ and troubleshooting

Common questions and recovery recipes that don't fit cleanly into one command's
reference page.

## Installation

### `wpm: command not found` after running the install script

The install script downloads the binary into a system path, but your shell's
`PATH` might not include it yet. Open a new terminal session, or run `hash -r`
in your current one to refresh the shell's command cache. If the problem
persists, check that `/usr/local/bin` is in `PATH` (Linux/macOS) or that
`%USERPROFILE%\.wpm\bin` is in `PATH` (Windows).

### How do I install a specific version of wpm?

The install scripts accept a version argument:

```sh
curl -fsSL https://wpm.so/install | bash -s -- 0.1.0
```

For `go install`:

```sh
go install go.wpm.so/cli/cmd/wpm@v0.1.0
```

### Where does wpm keep its files?

| Path                      | Purpose                                          |
| :------------------------ | :----------------------------------------------- |
| `~/.wpm/config.json`      | Client config: auth token, default user.         |
| `~/.wpm/cache/`           | Registry response cache and download cache.      |
| `~/.wpm/cache/install/`   | HTTP cache for install-time GET requests.        |
| Per project `wpm.json`    | Package manifest.                                |
| Per project `wpm.lock`    | Resolved dependency snapshot.                    |
| Per project `wp-content/` | Where dependencies are extracted (configurable). |

The config directory can be moved with `--config` or `WPM_CONFIG`.

## `wpm.json` and `wpm.lock`

### Should I commit `wpm.lock`?

Yes. The lockfile is what makes installs reproducible. Without it, every install
resolves against the registry from scratch and may produce different versions
over time. Treat it the same way you'd treat `package-lock.json` or
`composer.lock`: commit it; review it on pull requests.

### `wpm.json` says one version but `wpm ls` shows another

`wpm.lock` is what got resolved. If your `wpm.json` was edited by hand or by
another tool, the two can drift. `wpm ls` shows the "invalid" marker in red on
the offending line. Run `wpm install` to re-resolve and bring everything back in
sync.

### Why is the dependency version recorded as an exact string rather than a range?

wpm pins to exact versions for reproducibility. When you run
`wpm install pkg@latest`, the version the registry resolves is recorded in
`wpm.json`. To follow the registry's `latest` tag, re-run
`wpm install pkg@latest` whenever you want to refresh.

### I see `wpm upgrade required: lockfile version is newer than this version of wpm`

Someone else on your team is using a newer wpm and produced a lockfile your
version doesn't understand. Upgrade wpm and try again. See
[Installation](../installation/index.md) for upgrade instructions.

## Install and resolution

### `wpm install` is slow

A few common causes:

- The registry is far away or under load. Increase parallelism with
  `--network-concurrency` (default `16`). A higher value helps on fast
  connections; lower it on flaky links.
- The HTTP cache is being repopulated. After one successful install, re-runs are
  faster because the registry's manifest responses are cached at
  `~/.wpm/cache/install/`.
- A lot of resolution conflicts are forcing repeated registry calls. Check for
  diverging transitive requirements; pinning the package in your `dependencies`
  short-circuits the lookup.

### How do I clear the install cache?

Delete the directory:

```sh
rm -rf ~/.wpm/cache
```

The next `wpm install` will refetch everything from the registry. The lockfile
and your `wp-content/` are untouched.

### "Already up-to-date!" but I just edited `wpm.json`

`wpm install` compares the resolved tree against the lockfile and the
filesystem. If your edit didn't actually change the resolved set (for example,
you changed the formatting but not any value), no work is needed. Verify with
`wpm ls`.

If the edit changed a version, double-check that the new version is valid
SemVer. Invalid values are caught at validation time and the manifest is
rejected before reconciliation.

### How do I force a clean re-install?

The simplest path:

```sh
rm -rf wpm.lock wp-content/plugins wp-content/themes wp-content/mu-plugins
wpm install
```

Don't commit the result of that sequence until you've verified the new lockfile
is what you expect.

### `failed to acquire workspace lock`

Another wpm process is running in the same project directory, or it crashed and
left the lock file in place. wpm holds a file lock inside the project's content
directory while it installs.

If you're sure no other wpm process is running:

```sh
ls wp-content/.wpm.lock
rm wp-content/.wpm.lock
wpm install
```

Adjust the path if you've changed `config.content-dir`.

## Publish

### `package marked as private cannot be published`

Your `wpm.json` has `"private": true`. This is a tripwire that prevents
accidental publishing. Remove the flag from `wpm.json` when the package is ready
to release.

Note: this is different from `wpm publish --access private`, which publishes the
package with private visibility on the registry. See
[Registry concepts](../registry/index.md) for the distinction.

### `tarball size exceeds 134217728 bytes`

The packed package is over 128 MiB. Common causes:

- A `node_modules/` or `vendor/` directory that wasn't excluded.
- Pre-built binaries or large images committed to the source tree.
- Test fixtures that were never meant to ship.

Add the offending paths to `.wpmignore` and try `wpm publish --dry-run` again to
check the new size.

### `tarball size is zero, cannot publish empty package`

`.wpmignore` excluded everything. Look for a pattern that's broader than you
intended. `wpm publish --verbose --dry-run` will list each file that would be
included, so you can see what's left after the exclusions apply.

### My readme isn't showing on the registry

Two checks:

- The file must be named `readme.md` (case-insensitive) at the project root.
- It must be under 50 KiB. Content beyond that is dropped silently.

`wpm publish --verbose` shows whether the readme made it into the tarball.

## Authentication

### `user must be logged in to perform this action`

`wpm publish` requires both `authToken` and `defaultUser` to be set in your
config file. Run `wpm auth login` first. If you've already logged in but still
see this error, your config file may have been truncated or replaced; run
`wpm whoami` to confirm the current state.

### `WPM_TOKEN` is set but commands still complain about authentication

`wpm auth login` doesn't read `WPM_TOKEN`. The variable is only used as a
fallback by _other_ commands when the config file has no token. If you want to
log in non-interactively, pass `--token "$WPM_TOKEN"` to `auth login` instead.

For CI, you don't need to call `auth login` at all. Just set `WPM_TOKEN` and run
the command you actually want.

### How do I switch between two accounts on the same machine?

Use `--config` to keep their state separate:

```sh
wpm --config ~/.wpm-personal auth login
wpm --config ~/.wpm-work auth login
```

See [Authentication](../authentication/index.md) for the full workflow.

## Logs and output

### How do I get color back in piped output?

By default wpm disables color when stdout is not a terminal (for example, when
piped to `less`). Force it on with:

```sh
FORCE_COLOR=1 wpm install
```

This also works through `CLICOLOR_FORCE=1`. Either takes precedence over the
terminal check; neither overrides `NO_COLOR`.

### How do I keep wpm quiet in CI logs?

Set both:

```sh
export CI=true       # disables the progress spinner
export NO_COLOR=1    # strips ANSI color codes
```

See the [CI/CD guide](../guides/ci.md) for full examples.

### How do I get more detailed logs?

Two related options:

- `--debug` (or `-D`) enables debug-level logging across wpm.
- `--log-level debug` sets the underlying logger's level explicitly. The valid
  values are `debug`, `info`, `warn`, `error`, `fatal`.

The two flags can be used together; `--debug` is shorter for the common case.

## Exit codes

### What do the exit codes mean?

| Code      | Meaning                                                                   |
| :-------- | :------------------------------------------------------------------------ |
| `0`       | Success.                                                                  |
| `1`       | A generic, unclassified error. Read the stderr message.                   |
| `125`     | A flag or usage error. Run the command with `--help`.                     |
| `128 + N` | The process was terminated by signal `N` (for example, `130` for SIGINT). |

If you send three termination signals (Ctrl+C / SIGTERM) within a short window,
wpm force-exits with code `1` even if a command is still running.

## Related

- [Installation](../installation/index.md)
- [Getting started](../getting-started/index.md)
- [Authentication](../authentication/index.md)
- [CI/CD guide](../guides/ci.md)
- [Glossary](../glossary/index.md)
