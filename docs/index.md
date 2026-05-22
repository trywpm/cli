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

See the [installation guide](guide/installation.md) for Docker, `go install`,
and source builds.

## Guide

Step-by-step guides for the things you'll do most:

- **[Installation](guide/installation.md)**: install methods, shell completion,
  troubleshooting, and where wpm keeps its files.
- **[Getting started](guide/getting-started.md)**: scaffold a project, add
  dependencies, inspect the tree, optionally publish.
- **[Authentication](guide/authentication.md)**: tokens, identity, multi-account
  patterns, rotation.
- **[Publishing](guide/publishing.md)**: versioning, dist tags, visibility, and
  the release lifecycle.
- **[CI/CD](guide/ci.md)**: recipes for GitHub Actions, GitLab, and Docker.

## Reference

Detailed reference for every file format and core concept:

- **[`wpm.json`](reference/wpm-json.md)**: the package manifest schema.
- **[Dependencies](reference/dependencies.md)**: the dependency maps and the
  resolver's rules.
- **[Runtime compatibility](reference/runtime.md)**: `requires` versus
  `config.runtime`.
- **[`wpm.lock`](reference/wpm-lock.md)**: the lockfile format.
- **[`.wpmignore`](reference/wpmignore.md)**: publish exclusions.
- **[Package types](reference/package-types.md)**: `plugin`, `theme`, and
  `mu-plugin`.
- **[Registry](reference/registry.md)**: dist tags, visibility, and self-hosted
  setups.
- **[Glossary](reference/glossary.md)**: definitions of wpm-specific terms.

## CLI reference

The full [CLI reference](cli/wpm.md) covers every command, every flag, and every
exit code. Most projects reach for these every day:

- [`wpm init`](cli/init.md) to start a new project.
- [`wpm install`](cli/install.md) to add and install dependencies.
- [`wpm ls`](cli/ls.md) to see what's installed.
- [`wpm outdated`](cli/outdated.md) to check for updates.
- [`wpm publish`](cli/publish.md) to share your own packages.
- [`wpm auth login`](cli/auth_login.md) to log in to the registry.

## Community

- Source: <https://github.com/trywpm/cli>
- Report a bug: <https://github.com/trywpm/cli/issues>
- Ask a question: <https://github.com/trywpm/cli/discussions>
