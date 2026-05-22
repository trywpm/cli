# wpm

<!---MARKER_GEN_START-->

Package Manager for WordPress ecosystem

### Subcommands

| Name                        | Description                                                        |
| :-------------------------- | :----------------------------------------------------------------- |
| [`auth`](auth.md)           | Authenticate with the wpm registry                                 |
| [`init`](init.md)           | Initialize a new WordPress package or init wpm in existing project |
| [`install`](install.md)     | Install project dependencies and add new packages                  |
| [`ls`](ls.md)               | List installed dependencies                                        |
| [`outdated`](outdated.md)   | Check for outdated dependencies                                    |
| [`publish`](publish.md)     | Publish a package to the wpm registry                              |
| [`uninstall`](uninstall.md) | Remove dependencies from the project                               |
| [`whoami`](whoami.md)       | Display the current user                                           |
| [`why`](why.md)             | Show why a package is installed                                    |

### Options

| Name                | Type     | Default                  | Description                                                       |
| :------------------ | :------- | :----------------------- | :---------------------------------------------------------------- |
| `--config`          | `string` | `/home/thelovekesh/.wpm` | Location of client config files                                   |
| `-D`, `--debug`     | `bool`   |                          | Enable debug mode                                                 |
| `-l`, `--log-level` | `string` | `info`                   | Set the logging level ("debug", "info", "warn", "error", "fatal") |
| `--registry`        | `string` | `registry.wpm.so`        | Set specific registry to use                                      |

<!---MARKER_GEN_END-->

## Description

<!-- prettier-ignore -->
> [!IMPORTANT]
> wpm is in active development and pre-1.0. Expect occasional
> breaking changes to CLI flags and to the `wpm.lock` format until 1.0.
> Significant changes are called out in release notes.

`wpm` is a package manager for the WordPress ecosystem. It treats plugins,
themes, and must-use plugins as versioned packages that you can install, update,
publish, and depend on, the same way you would with npm or Composer in their
respective ecosystems.

A wpm project is any directory that contains a `wpm.json` manifest. Most
commands run against the current directory and produce or consume two files at
its root: `wpm.json` (what you declare) and `wpm.lock` (what was resolved).
Installed packages land under `wp-content/` by default.

### Mental model

| File                 | Role                                                                                          |
| :------------------- | :-------------------------------------------------------------------------------------------- |
| `wpm.json`           | Your package's manifest. Declares name, version, type, dependencies, and runtime constraints. |
| `wpm.lock`           | A frozen snapshot of the resolved dependency tree. Commit this to version control.            |
| `wp-content/`        | Where wpm extracts plugins, themes, and mu-plugins. Configurable via `config.content-dir`.    |
| `.wpmignore`         | Optional gitignore-style file that excludes paths from `wpm publish`.                         |
| `~/.wpm/config.json` | Your client config, including the auth token after `wpm auth login`.                          |

The registry at `registry.wpm.so` hosts the packages. You can point wpm at a
different registry with `--registry`.

### A typical workflow

The most common path through wpm looks like this:

1. **Start a project**: `wpm init` (or `wpm init --existing` when adopting wpm
   in an existing plugin or theme).
2. **Add dependencies**: `wpm install akismet hello-dolly@1.7.2`. The packages
   are downloaded, extracted under `wp-content/`, and recorded in both
   `wpm.json` and `wpm.lock`.
3. **Develop**: re-run `wpm install` to bring the lockfile and the filesystem
   back in sync after editing `wpm.json` by hand. Use `wpm ls`, `wpm outdated`,
   and `wpm why` to understand what's installed.
4. **Release**: `wpm auth login` once, then `wpm publish` to upload a new
   version of your own package.

### Global options

The global flags apply to every subcommand and are parsed before the subcommand
name. Put them between `wpm` and the subcommand:

```console
$ wpm --registry registry.staging.wpm.so install
```

| Flag                | Default           | Notes                                                                                        |
| :------------------ | :---------------- | :------------------------------------------------------------------------------------------- |
| `--config <dir>`    | `~/.wpm`          | Directory holding `config.json`. Useful for keeping multiple identities side by side.        |
| `-D`, `--debug`     | off               | Enables debug-level logging across wpm. Equivalent to setting `WPM_DEBUG` to any value.      |
| `-l`, `--log-level` | `info`            | Sets the underlying logger's level. Accepts `debug`, `info`, `warn`, `error`, `fatal`.       |
| `--registry <url>`  | `registry.wpm.so` | Point wpm at a different registry (for example, a staging deployment or self-hosted mirror). |
| `-v`, `--version`   | n/a               | Print the version and the git commit, then quit.                                             |
| `-h`, `--help`      | n/a               | Show help for the current command.                                                           |

### Environment variables

wpm reads the following variables. Flags always win over environment variables
when both are present.

| Variable                        | Effect                                                                                   |
| :------------------------------ | :--------------------------------------------------------------------------------------- |
| `WPM_CONFIG`                    | Override the config directory. Same as `--config`.                                       |
| `WPM_TOKEN`                     | Fallback auth token used when no token is stored in `config.json`.                       |
| `WPM_DEBUG`                     | Enable debug mode when set to any non-empty value. Same as `--debug`.                    |
| `NO_COLOR`                      | Disable all ANSI color output. Takes precedence over `FORCE_COLOR`.                      |
| `FORCE_COLOR`, `CLICOLOR_FORCE` | Force color output, even when stdout is not a terminal. Set to any value other than `0`. |
| `CLICOLOR=0`                    | Disable color output. Has the same effect as `NO_COLOR`.                                 |
| `TERM=dumb`                     | Disable color output and spinners.                                                       |
| `CI`                            | When set to any non-empty value, disables the progress spinner. Useful in build logs.    |
| `NORAW`                         | Disable raw terminal mode for stdin. Niche; only matters when wpm prompts for input.     |

### Exit codes

| Code      | Meaning                                                                             |
| :-------- | :---------------------------------------------------------------------------------- |
| `0`       | Success.                                                                            |
| `1`       | A generic, unclassified error. The error message is printed on stderr.              |
| `125`     | A flag or usage error. Run the command with `--help` to see the correct invocation. |
| `128 + N` | The process was terminated by signal `N` (for example, `130` for Ctrl+C / SIGINT).  |

Sending three termination signals (`SIGINT` or `SIGTERM`) within a short window
force-exits wpm with code `1`. This lets you bail out of a hung operation even
when the first signal is caught and graceful shutdown is taking too long.

### Output and progress

- Most progress is emitted on **stderr** so that the **stdout** output of
  commands like `wpm whoami` and `wpm ls` is clean and easy to capture in
  scripts.
- Color is enabled when stdout is a terminal and none of the disabling
  environment variables apply. See the table above for the precedence rules.
- The spinner shown during long-running operations is suppressed when `CI` is
  set, when `TERM=dumb`, or when the output is not a terminal.

### Where to go next

- New to wpm? Start with the [getting started](../getting-started/index.md)
  walkthrough.
- Releasing a package? See the [publishing guide](../guides/publishing.md).
- Setting up CI? See the [CI/CD guide](../guides/ci.md).
- File-format references: [`wpm.json`](../wpm-json/index.md),
  [`wpm.lock`](../wpm-lock/index.md), and [`.wpmignore`](../wpmignore/index.md).
- Concept references: [authentication](../authentication/index.md),
  [package types](../package-types/index.md), and
  [registry concepts](../registry/index.md).
- Stuck? Check the [FAQ and troubleshooting](../faq/index.md) page, or the
  [glossary](../glossary/index.md) for term definitions.

## Examples

### Print the installed version

```console
$ wpm --version
wpm version v0.1.0 (abc1234)
```

### Run any command against a non-default registry

```console
$ wpm --registry registry.staging.wpm.so install
```

### Use a separate identity for one command

```console
$ wpm --config ~/.wpm-work whoami
alice
```

### Enable debug logging for troubleshooting

```console
$ wpm -D install
```

### Keep wpm quiet in CI logs

Setting `CI` suppresses the spinner. `NO_COLOR` strips ANSI escape codes from
the output so log viewers stay readable.

```console
$ CI=true NO_COLOR=1 wpm install
```

### First five minutes with wpm

For a full walkthrough from scaffold to first publish, see
[Getting started](../getting-started/index.md).
