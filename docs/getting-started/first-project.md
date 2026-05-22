---
title: First project
description:
  Build your first wpm plugin or theme project in ten minutes by scaffolding
  dependencies, locking versions, and publishing.
---

In about ten minutes, you'll go from a fresh wpm install to a working project
with dependencies. You don't need a registry account for the first five steps.

If you don't have wpm yet, follow the [installation guide](installation.md)
first.

## Before you start

You'll need:

- wpm installed and on your `PATH`. Run `wpm --version` to check.
- A shell. The examples use bash, but zsh, fish, and PowerShell all work.
- An empty directory for your project.

You don't need a running WordPress site. wpm manages plugin and theme files
directly in your project directory. You'll only need a real WordPress install
when you want to use the plugins you've downloaded.

## What you'll build

A small WordPress plugin called `my-first-plugin`. It will end up with three
packages installed: two production dependencies and one dev dependency, all
locked to specific versions. You'll also learn how to inspect dependencies,
update packages, and publish your project.

## Step 1: Scaffold a project

Make an empty directory and run `wpm init -y --type plugin`:

```console
$ mkdir my-first-plugin
$ cd my-first-plugin
$ wpm init -y --type plugin
config created at /work/my-first-plugin/wpm.json
```

The `-y` flag tells wpm to skip the prompts and use defaults. `--type plugin`
tells wpm to create a WordPress plugin package.

Open `wpm.json` in your editor. It should look like this:

```json
{
	"name": "my-first-plugin",
	"version": "1.0.0",
	"type": "plugin",
	"license": "GPL-2.0-or-later"
}
```

wpm created the required fields for you (`name`, `version`, and `type`) and
added a default license. You can add everything else later as your project
grows. See the [`wpm.json` reference](../fundamentals/wpm-json.md) for the full
list of fields.

## Step 2: Install production dependencies

`wpm install` adds packages to your `wpm.json` and downloads them into
`wp-content/plugins/`. You can install several at once:

```console
$ wpm install akismet hello-dolly@1.7.2
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2

2 packages installed
```

wpm wrote two things just now: a `dependencies` block in `wpm.json`, and a new
`wpm.lock` next to it. The lockfile records the exact versions wpm installed.

Commit `wpm.lock` to version control alongside `wpm.json`. This keeps installs
consistent across machines and CI.

## Step 3: Install development dependencies

For tools you only need during development, use `-D`:

```console
$ wpm install -D query-monitor
wpm install v0.1.0

+ query-monitor 3.20.2

1 package installed
```

wpm adds these packages to `devDependencies`.

Later, you can skip them in production with `wpm install --no-dev`.

## Step 4: Inspect the tree

`wpm ls` shows what's installed:

```console
$ wpm ls
my-first-plugin
├── akismet@5.3.1
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

`wpm why` shows why a package is installed:

```console
$ wpm why akismet
my-first-plugin (dependencies)
└─ akismet@5.3.1
```

## Step 5: Check for updates

`wpm outdated` shows outdated packages. It labels each update as major, minor,
or patch:

```console
$ wpm outdated
wpm outdated v0.1.0

akismet [plugin]
├── current: 5.3.1
└── latest:  5.4.0 (minor update)
```

To upgrade, install the package again at the new version:

```console
$ wpm install akismet@5.4.0
```

wpm updates both `wpm.json` and `wpm.lock` for you.

## Step 6: Publish (optional)

If you have a registry account, you can publish your project. Skip this step if
you don't have an account yet.

First, log in. You only need to do this once per machine:

```console
$ wpm auth login
Token:
welcome <your-username>!
```

It's a good idea to preview the package before publishing. `--dry-run` packs the
tarball, prints the summary, and stops there. Nothing is uploaded:

```console
$ wpm publish --dry-run
📦 my-first-plugin@1.0.0

├─ Tag:     latest
├─ Access:  private
├─ Files:   3
├─ Size:    1.2 KB (2.1 KB unpacked)
└─ Digest:  9j6Q7l8s...=

dry run complete, my-first-plugin@1.0.0 is ready to be published
```

When the summary looks right, drop `--dry-run` to publish for real:

```console
$ wpm publish
📦 my-first-plugin@1.0.0
...

✔ published my-first-plugin@1.0.0
```

Before you publish, check that `.wpmignore` excludes everything you don't want
to ship: build artifacts, secrets, test fixtures. See
[`.wpmignore`](../fundamentals/wpmignore.md) for the syntax.

## What you have now

- A `wpm.json` with the three required fields, plus `dependencies` and
  `devDependencies`.
- A `wpm.lock` that pins the exact versions wpm installed.
- An optional `.wpmignore` if you got to step 6.
- The packages themselves under `wp-content/plugins/`.

Commit everything except `wp-content/` to version control. Those files can
always be reinstalled from `wpm.lock`.

## Where to go next

| If you want to...                            | Read                                                               |
| :------------------------------------------- | :----------------------------------------------------------------- |
| Understand `wpm.json` in detail              | [`wpm.json` reference](../fundamentals/wpm-json.md)                |
| Run wpm in CI                                | [CI/CD guide](../guides/ci.md)                                     |
| Manage tokens and multiple accounts          | [Authentication](../guides/authentication.md)                      |
| Learn the plugin/theme/mu-plugin differences | [Package types](../fundamentals/package-types.md)                  |
| Browse all CLI commands                      | [`wpm`](../reference/cli/wpm.md)                                   |
| Adopt wpm in an existing plugin or theme     | [`wpm init`](../reference/cli/init.md) (see the `--existing` mode) |
