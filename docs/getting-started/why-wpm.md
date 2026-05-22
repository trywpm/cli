---
title: Why wpm?
description:
  Why wpm exists, how it improves WordPress dependency management, and when to
  use it in plugin and theme projects.
---

Managing dependencies in a WordPress project has always been a patchwork.

A mature site pulls from many places: free plugins, commercial plugins, themes
from independent vendors, and code your team writes itself. None of this follows
a shared standard for installing, updating, or auditing.

So teams improvise. Some extend Composer with mixed repositories. Some maintain
shell scripts that download ZIPs during CI. Some commit vendor plugins straight
into the repo as Git submodules. These patterns work fine for small sites, but
production WordPress at scale exposes the gaps.

wpm is the native answer. It treats WordPress plugins and themes as first-class
packages and gives them the tooling that Node, Rust, and Go developers take for
granted.

## Where current workflows break down

If you've shipped serious WordPress, you've felt some version of these problems.

### Vendor distribution is fragmented

WordPress doesn't define a standard for how commercial plugin and theme vendors
distribute their software. So every vendor invents their own:

- Some publish a ZIP behind an account-gated download URL.
- Some expose a native API you can hit from automation.
- Some support Composer through a custom repository.
- Some only offer a download link on their dashboard.

A serious project ends up writing per-vendor integration for each pattern.
There's no single workflow that handles them all.

### ZIPs are operationally heavy

Every vendor update means a new ZIP. Teams stage it somewhere their CI can reach
(S3, a private CDN, Git LFS), update the reference in each project, and verify
each install. Multiply by the number of vendor plugins a serious site uses, and
the operational tax adds up fast.

### Updates don't integrate with your toolchain

Dependabot, Renovate, and the rest of the modern dependency tooling work by
reading declared dependency metadata. ZIP-based distribution sidesteps that
entirely. There's no machine-readable place to watch for new versions, so
monitoring becomes a manual workflow: check the vendor portal, hope you don't
miss a critical release, repeat.

### There's no WordPress-specific advisory database

npm has the [GitHub Advisory Database](https://github.com/advisories). PyPI has
its own. RubyGems has `bundler-audit`. WordPress has Patchstack and WPScan, both
excellent, but neither is wired into a native package manager. Auditing
installed plugins for known vulnerabilities is a manual, separate workflow.

<!-- prettier-ignore -->
> [!NOTE]
> A WordPress-specific advisory database integrated into wpm is on the
> roadmap. Once it lands, `wpm audit` will surface known
> vulnerabilities in your installed packages without leaving the CLI.

### No standardized dependency metadata

WordPress plugins don't declare what they depend on. The
`dinopack-for-elementor` plugin needs `elementor` to function, but nothing in
the WordPress.org listing tells your tooling that. With Composer plus
wpackagist:

```console
$ composer require wpackagist-plugin/dinopack-for-elementor
# Installs dinopack-for-elementor.
# Does not install elementor.
# You have a broken site.
```

You have to figure out the right Elementor version yourself, then install it in
a second command.

With wpm, the dependency is part of the package's manifest in the registry:

```console
$ wpm install dinopack-for-elementor
wpm install v0.1.7

+ dinopack-for-elementor 1.0.5
+ elementor 3.34.1

2 packages installed
```

`wpm why` gives you full visibility into why each package is present:

```console
$ wpm why elementor
my-cool-project (dependencies)
└─ dinopack-for-elementor@1.0.5
   └─ elementor@3.34.1
```

## How wpm closes the gap

Four design choices get wpm there.

### A registry that speaks WordPress

The wpm registry enforces a strict metadata schema (`wpm.json`) for every
package. Dependencies are declared, not inferred. Public plugins, commercial
plugins, and packages you publish yourself all live in the same registry with
the same publishing model. Your CI pulls them through one tool.

### An installer, not a solver

wpm doesn't compute the "best intersection of versions" the way most package
managers do. It installs the exact versions you commit.

WordPress plugins can change database schema, write options, register cron jobs,
and modify content. A solver-based upgrade can trigger a database migration you
weren't ready for. wpm refuses to make that choice automatically. Every upgrade
is an explicit decision you make.

This is why wpm rejects SemVer ranges in `dependencies` and only accepts exact
versions or `*`. See [Dependencies](../fundamentals/dependencies.md) for the
full reasoning.

### A single static binary

wpm ships as a Go binary with no runtime dependencies. It brings its own
networking stack and its own Zstandard decompressor. It runs on the OS, not the
PHP runtime, so it works on the restrictive hosting environments where PHP-based
tools can't.

### Supply-chain security as a first-class concern

Every package wpm installs is cryptographically verifiable. You can trust that
what you install is what the author published.

The registry signs each manifest at publish time. Every install verifies the
SHA-256 digest against what the registry recorded. If the bytes don't match
(corrupted download, man-in-the-middle, compromised mirror), the install fails
before anything lands on disk.

This is the foundation for the upcoming advisory database. Verified provenance
comes first; auditing for known vulnerabilities builds on top of it.

## Beyond the CLI

wpm is one piece of a broader effort. Sister projects:

- **[wp-to-wpm](https://github.com/trywpm/wp-to-wpm)**: migrates plugins and
  themes from WordPress.org SVN to the wpm registry, designed to run as a GitHub
  Action.
- **[setup-wpm](https://github.com/trywpm/setup-wpm)**: a GitHub Action that
  installs and configures the wpm CLI for your CI/CD pipelines.

All of it is open source. Contributions, design discussions, and ecosystem
feedback are welcome.

## When wpm is the right call

Reach for wpm when:

- You're running WordPress in production and want reproducible installs across
  environments.
- You manage a mix of public and commercial plugins.
- You want cryptographic supply-chain guarantees, not just downloaded ZIPs.
- Your hosting environment is locked down and you can't rely on PHP CLI being
  available.
- You want your WordPress dependency workflow to integrate with the modern
  dependency-management toolchain (CI, security scanners, audit logs).

Composer remains a great tool for general PHP libraries. The two can coexist in
a single project: Composer for PHP packages, wpm for WordPress plugins and
themes.

## Where to go next

- **[Install wpm](installation.md)** in a single command.
- **[First project](first-project.md)**: a ten-minute walkthrough from empty
  directory to working project.
- **[Dependencies](../fundamentals/dependencies.md)**: the full reasoning behind
  exact-version pinning.
- **[wpm registry](../fundamentals/registry.md)**: how it stores manifests and
  verifies provenance.
