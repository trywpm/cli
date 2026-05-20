# Getting started

This walkthrough takes you from a freshly installed wpm to a working
project with declared dependencies in about ten minutes. No registry
account is needed for the first four steps.

If wpm is not on your machine yet, follow the
[installation guide](../installation/index.md) first.

## What you'll build

A small WordPress plugin called `my-first-plugin` with two dependencies
declared in `wpm.json` and a lockfile that pins them. By the end you'll
know how to inspect, update, and publish your own packages.

## Step 1: Scaffold a project

Create an empty directory and run `wpm init`. The `-y` flag accepts
the defaults so you can see the shape of a minimal package without
answering prompts.

```console
$ mkdir my-first-plugin
$ cd my-first-plugin
$ wpm init -y --type plugin
config created at /work/my-first-plugin/wpm.json
```

Open `wpm.json` in your editor. It should look like this:

```json
{
  "name": "my-first-plugin",
  "version": "1.0.0",
  "type": "plugin",
  "license": "GPL-2.0-or-later"
}
```

The three required fields (`name`, `version`, `type`) are filled in
from the defaults. Everything else is yours to add as the project
grows. See the [`wpm.json` reference](../wpm-json/index.md) for the
full schema.

## Step 2: Add dependencies

`wpm install` adds packages to `wpm.json` and downloads them under
`wp-content/plugins/`. You can mix version specifiers in a single
call:

```console
$ wpm install akismet hello-dolly@1.7.2
wpm install v0.1.0

+ akismet 5.3.1
+ hello-dolly 1.7.2

2 packages installed
```

After this runs, your `wpm.json` has a `dependencies` block, and a new
`wpm.lock` lives next to it. The lockfile records the exact resolved
versions and the SHA-256 digest of every tarball. Commit `wpm.lock`
to version control; it's how the next install on a different machine
produces the same on-disk state.

For development-only tools (like `query-monitor`), use `-D` so they
land in `devDependencies` and can be skipped with `--no-dev` in
production:

```console
$ wpm install -D query-monitor
wpm install v0.1.0

+ query-monitor 3.20.2

1 package installed
```

## Step 3: Inspect the tree

`wpm ls` prints the dependency tree from your lockfile:

```console
$ wpm ls
my-first-plugin
├── akismet@5.3.1
├── hello-dolly@1.7.2
└── query-monitor@3.20.2
```

If a transitive dependency surprises you, `wpm why` traces the chain
back to the root:

```console
$ wpm why akismet
my-first-plugin (dependencies)
└─ akismet@5.3.1
```

## Step 4: Check for updates

`wpm outdated` calls the registry and compares each installed version
against the `latest` dist tag. It classifies the gap as major, minor,
or patch using SemVer rules:

```console
$ wpm outdated
wpm outdated v0.1.0

akismet [plugin]
├── current: 5.3.1
└── latest:  5.4.0 (minor update)
```

To actually bump a package, install it again at the desired version:

```console
$ wpm install akismet@5.4.0
```

The `wpm.json` and `wpm.lock` are both updated in place.

## Step 5: Publish (optional)

If you have a registry account, you can publish your project as a
package. Skip this step if you don't have an account yet.

First, log in. You only need to do this once per machine:

```console
$ wpm auth login
Token: 
welcome <your-username>!
```

Then preview a publish before committing to it. `--dry-run` packs
the tarball and prints the summary block, but skips the upload:

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

When you're satisfied, drop `--dry-run` to publish for real:

```console
$ wpm publish
📦 my-first-plugin@1.0.0
...

✔ published my-first-plugin@1.0.0
```

Before publishing anything you didn't write yourself, double-check
that `.wpmignore` excludes everything you don't want in the tarball.
See [`.wpmignore`](../wpmignore/index.md) for the syntax.

## What you have now

- A `wpm.json` with three required fields, declared `dependencies`,
  and declared `devDependencies`.
- A `wpm.lock` pinning the resolved tree.
- An optional `.wpmignore` if you reached step 5.
- Installed dependencies under `wp-content/plugins/`.

Everything except `wp-content/` should be committed to version
control.

## Where to go next

| If you want to...                              | Read                                                    |
|:-----------------------------------------------|:--------------------------------------------------------|
| Understand `wpm.json` in detail                 | [`wpm.json` reference](../wpm-json/index.md)            |
| Run wpm in CI                                   | [CI/CD guide](../guides/ci.md)                          |
| Manage tokens and multiple accounts             | [Authentication](../authentication/index.md)            |
| Learn the plugin/theme/mu-plugin differences    | [Package types](../package-types/index.md)              |
| Browse all CLI commands                         | [`wpm`](../cli/wpm.md)                                  |
| Adopt wpm in an existing plugin or theme        | [`wpm init`](../cli/init.md) (see the `--existing` mode) |
