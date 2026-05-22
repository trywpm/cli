# Welcome to wpm

wpm is an open source package manager for WordPress plugins and themes.

It's built in Go for performance and reliability, with a globally distributed
registry powered by Cloudflare Workers for fast installs around the world.

wpm makes WordPress dependencies reproducible, shareable, and easy to manage.

## Why wpm?

- Built for WordPress out of the box. No extra configuration required.
- Plugins, themes, and mu-plugins are first-class package types.
- Dependency resolution works automatically.
- Reproducible installs with `wpm.lock`.
- Built-in package integrity and supply chain security checks.
- Fast installs and reliable caching.
- Fully open source.

## Quick install

### Linux and macOS

```bash
curl -fsSL https://wpm.so/install | bash
```

### Windows (PowerShell)

```powershell
irm https://wpm.so/install.ps1 | iex
```

For Docker, `go install`, source builds, and shell completions, see the
[installation guide](getting-started/installation.md).

After installation, verify that wpm is available on your system path:

```bash
wpm --version
```

## First steps

Create a new project:

```bash
mkdir my-wp-project
cd my-wp-project

wpm init -y
```

Install dependencies:

```bash
wpm install akismet hello-dolly
```

wpm downloads the packages into `wp-content/plugins/` and records the exact
versions in `wpm.lock`.

Inspect the dependency tree:

```bash
wpm ls
```

Continue with:

- [Installation](getting-started/installation.md)
- [First project](getting-started/first-project.md)

## Learn wpm

### Fundamentals

Learn how the wpm ecosystem works:

- [Dependencies](fundamentals/dependencies.md)
- [`wpm.json`](fundamentals/wpm-json.md)
- [`wpm.lock`](fundamentals/wpm-lock.md)
- [Runtime compatibility](fundamentals/runtime.md)
- [Package types](fundamentals/package-types.md)
- [Registry](fundamentals/registry.md)

### Guides

Common workflows and real-world usage:

- [Authentication](guides/authentication.md)
- [Publishing packages](guides/publishing.md)
- [CI/CD](guides/ci.md)

### Reference

- [CLI reference](reference/cli/wpm.md)
- [Glossary](reference/glossary.md)

## Community

- GitHub: https://github.com/trywpm/cli
- Issues: https://github.com/trywpm/cli/issues
- Discussions: https://github.com/trywpm/cli/discussions
