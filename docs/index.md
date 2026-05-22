# wpm

The package manager for WordPress plugins and themes.

wpm makes WordPress dependencies declarative, reproducible, and shareable.
Declare what you need in `wpm.json`, run `wpm install`, and get a locked tree
recorded in `wpm.lock`. Publish your own packages with `wpm publish`. If you've
used npm or Composer, wpm will feel familiar.

> [!IMPORTANT] wpm is in active development and pre-1.0. Expect occasional
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

See the [installation guide](installation/index.md) for Docker, `go install`,
and source builds.

## Where to next

### New here

- **[Install wpm](installation/index.md)** in a single command, with shell
  completion set up for you.
- **[Getting started](getting-started/index.md)** walks through a complete
  project, from scaffold to your first published version, in about ten minutes.

### Reference

- **[CLI reference](cli/wpm.md)**: every command with options, examples, and
  exit codes.
- **[`wpm.json`](wpm-json/index.md)**: the package manifest.
- **[`wpm.lock`](wpm-lock/index.md)**: the lockfile format.
- **[`.wpmignore`](wpmignore/index.md)**: what `wpm publish` excludes.

### Concepts

- **[Package types](package-types/index.md)**: when to choose `plugin`, `theme`,
  or `mu-plugin`.
- **[Authentication](authentication/index.md)**: tokens, identity, and
  multi-account patterns.
- **[Registry](registry/index.md)**: dist tags, visibility, and self-hosted
  registries.
- **[Dependencies](wpm-json/dependencies.md)**: how `wpm.json` and the resolver
  work together.
- **[Runtime compatibility](wpm-json/runtime.md)**: the difference between
  `requires` and `config.runtime`.

### Guides

- **[CI/CD](guides/ci.md)**: recipes for GitHub Actions, GitLab, and Docker
  builds.

### Help

- **[FAQ and troubleshooting](faq/index.md)**: common errors with recovery
  steps.
- **[Glossary](glossary/index.md)**: definitions of wpm-specific terms.

## Community

- Source: <https://github.com/trywpm/cli>
- Report a bug: <https://github.com/trywpm/cli/issues>
- Ask a question: <https://github.com/trywpm/cli/discussions>
