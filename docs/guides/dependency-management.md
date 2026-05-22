---
title: Managing dependencies
description:
  Add, remove, update, and inspect dependencies in your wpm project using the
  commands you'll use most often.
---

This guide covers the day-to-day work of adding, removing, updating, and
inspecting your project's dependencies.

For the schema rules and the reasoning behind exact-version pinning, see
[Dependencies](../fundamentals/dependencies.md).

## Adding a production dependency

```console
$ wpm install akismet
```

wpm fetches the latest version from the registry, downloads it into
`wp-content/plugins/`, and records the resolved version in both `wpm.json` and
`wpm.lock`.

To pin a specific version, use `name@version`:

```console
$ wpm install akismet@5.3.1
```

## Adding a dev dependency

For tools you only need during development (debuggers, code quality checks,
sample data), use `-D`:

```console
$ wpm install -D query-monitor
```

wpm adds the package to `devDependencies` instead of `dependencies`. A
production install with `wpm install --no-dev` will skip them.

## Adding several at once

You can mix production and dev in a single call. Specifiers can be plain names
or `name@version`:

```console
$ wpm install akismet hello-dolly@1.7.2
$ wpm install -D query-monitor
```

## Moving between production and dev

Use `-P` to move a dev dependency into `dependencies`:

```console
$ wpm install -P query-monitor
```

And `-D` to move a production dependency into `devDependencies`:

```console
$ wpm install -D akismet
```

In both cases, wpm removes the old entry and writes the new one in the same
step.

## Removing a dependency

```console
$ wpm uninstall hello-dolly
```

wpm deletes the entry from `wpm.json` (whichever map it was in), updates
`wpm.lock`, and removes the package's files from `wp-content/`.

`wpm rm` and `wpm remove` are aliases for `wpm uninstall`.

## Checking for updates

```console
$ wpm outdated
```

`wpm outdated` asks the registry for the latest version of each installed
package and flags anything that's behind. Each update is labeled as major,
minor, or patch.

This command only reads. It never changes your project for you.

## Updating to a new version

To upgrade, install the package again at the new version:

```console
$ wpm install akismet@5.4.0
```

wpm updates both `wpm.json` and `wpm.lock`. To follow the registry's `latest`
dist tag, use `@latest`:

```console
$ wpm install akismet@latest
```

<!-- prettier-ignore -->
> [!NOTE]
> wpm uses exact versions, not ranges. Every upgrade is an explicit choice
> you make. See [Dependencies](../fundamentals/dependencies.md) for the
> reasoning.

## Inspecting the tree

`wpm ls` shows the full dependency tree from `wpm.lock`:

```console
$ wpm ls
my-plugin
├── akismet@5.3.1
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

`wpm why` shows why a package is installed. Useful when you see a package you
don't recognize:

```console
$ wpm why akismet
my-plugin (dependencies)
└─ akismet@5.3.1
```

## Verify before you deploy

<!-- prettier-ignore -->
> [!TIP]
> Run `wpm ls` before deploying to a live WordPress site. It shows the
> exact plugin and theme versions you're about to ship, so nothing
> surprises you in production.

## Related

- [Dependencies](../fundamentals/dependencies.md): the schema, the rules, and
  why wpm rejects version ranges.
- [`wpm install`](../reference/cli/install.md): every flag and example.
- [`wpm uninstall`](../reference/cli/uninstall.md): every flag.
- [`wpm ls`](../reference/cli/ls.md), [`wpm why`](../reference/cli/why.md),
  [`wpm outdated`](../reference/cli/outdated.md): the read-only inspection
  commands.
