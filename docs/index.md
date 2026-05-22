# wpm

A package manager for WordPress plugins and themes.

List the plugins and themes you need in a `wpm.json` file. Run `wpm install` and
wpm downloads them into `wp-content/`, locking the exact versions in `wpm.lock`.
The next person to clone your project gets the same set of files. Use
`wpm publish` to share your own plugins and themes.

If you've used npm or Composer, you already know how this works.

<!-- prettier-ignore -->
> [!IMPORTANT]
> wpm is in active development and pre-1.0. Expect occasional
> breaking changes to CLI flags and to the `wpm.lock` format until 1.0.
> Significant changes are called out in release notes.

## Try it in 60 seconds

If wpm is already on your machine:

```sh
$ mkdir my-plugin && cd my-plugin
$ wpm init -y --type plugin
$ wpm install akismet
```

That's the loop: declare in `wpm.json`, run `wpm install`, get a reproducible
tree pinned in `wpm.lock`. Commit both files to version control.

Don't have wpm yet?

```sh
$ curl -fsSL https://wpm.so/install | bash      # Linux, macOS
$ powershell -c "irm wpm.so/install.ps1|iex"    # Windows
```

See the [installation guide](getting-started/installation.md) for Docker,
`go install`, and source builds.

## Get started

- **[Overview](getting-started/overview.md)**: what wpm is, the files involved,
  and the basic loop.
- **[Installation](getting-started/installation.md)**: install methods, shell
  completion, where wpm keeps its files.
- **[First project](getting-started/first-project.md)**: a ten-minute
  walkthrough from empty directory to working project.

## Fundamentals

The files and concepts every wpm user should understand:

- **[`wpm.json`](fundamentals/wpm-json.md)**: the package manifest schema.
- **[Dependencies](fundamentals/dependencies.md)**: the dependency maps and the
  resolver's rules.
- **[Runtime compatibility](fundamentals/runtime.md)**: `requires` versus
  `config.runtime`.
- **[`wpm.lock`](fundamentals/wpm-lock.md)**: the lockfile format.
- **[`.wpmignore`](fundamentals/wpmignore.md)**: publish exclusions.
- **[Package types](fundamentals/package-types.md)**: `plugin`, `theme`, and
  `mu-plugin`.
- **[Registry](fundamentals/registry.md)**: dist tags, visibility, and
  self-hosted setups.

## Guides

Step-by-step walkthroughs for common tasks:

- **[Authentication](guides/authentication.md)**: tokens, identity,
  multi-account patterns, rotation.
- **[Managing dependencies](guides/dependency-management.md)**: add, remove,
  update, and inspect.
- **[Runtime compatibility](guides/runtime-compatibility.md)**: enable strict
  mode and read its errors.
- **[Publishing](guides/publishing.md)**: versioning, dist tags, visibility, and
  the release lifecycle.
- **[CI/CD](guides/ci.md)**: recipes for GitHub Actions, GitLab, and Docker.

## Reference

- **[CLI reference](reference/cli/wpm.md)**: every command, flag, and exit code.
- **[Glossary](reference/glossary.md)**: definitions of wpm-specific terms.

Most projects reach for these CLI commands every day:

- [`wpm init`](reference/cli/init.md) to start a new project.
- [`wpm install`](reference/cli/install.md) to add and install dependencies.
- [`wpm ls`](reference/cli/ls.md) to see what's installed.
- [`wpm outdated`](reference/cli/outdated.md) to check for updates.
- [`wpm publish`](reference/cli/publish.md) to share your own packages.
- [`wpm auth login`](reference/cli/auth_login.md) to log in to the registry.

## Community

- Source: <https://github.com/trywpm/cli>
- Report a bug: <https://github.com/trywpm/cli/issues>
- Ask a question: <https://github.com/trywpm/cli/discussions>
