---
title: Strict runtime mode
description:
  Turn on strict runtime mode so wpm verifies that your dependencies work with
  the WordPress and PHP versions you target.
---

By default, wpm doesn't check whether your dependencies are compatible with the
WordPress and PHP versions you're running. **Strict runtime mode** turns that
check on. This guide walks through enabling it, reading the errors, and deciding
when to use it.

For the field-level reference (what `requires` and `config.runtime` each mean),
see [Runtime compatibility](../fundamentals/runtime.md).

## When to enable strict mode

Turn it on when:

- You want CI to catch dependencies that quietly bumped their minimum WordPress
  or PHP version.
- You're testing your plugin against a specific WordPress and PHP matrix and
  want installs to fail fast on incompatible dependencies.
- You're maintaining a long-term-support branch and want to be sure nothing in
  the tree drifts past your supported versions.

Leave it off when:

- You're experimenting and want installs to succeed even when some dependencies
  declare conservative `requires`.
- Your project is a library that targets a broad range and you rely on the
  registry to surface compatibility issues separately.

## Setting it up

Add a `config.runtime` block to your `wpm.json`:

```json
{
	"name": "my-plugin",
	"version": "1.0.0",
	"type": "plugin",
	"config": {
		"runtime": {
			"wp": "6.9",
			"php": "8.2"
		}
	}
}
```

The values are concrete versions, not constraints. They are the specific
versions you're testing against.

Setting either `runtime.wp` or `runtime.php` is enough to turn strict mode on
for the whole project.

## What wpm checks

During every `wpm install`, wpm reads each dependency's `requires` field and
compares it against your `config.runtime`. If any dependency declares a
constraint your runtime can't satisfy, the install fails before anything is
downloaded.

## Reading the error

A typical strict-mode failure looks like this:

```
package akismet@5.3.1 incompatible:
  requires WordPress >=6.5, but runtime WordPress version is 6.4
```

This tells you:

- **Which package** failed (`akismet@5.3.1`).
- **What it needs** (`WordPress >=6.5`).
- **What you have** (`runtime WordPress version is 6.4`).

You have three options:

- **Bump your runtime**: edit `config.runtime.wp` in `wpm.json` to a version
  that satisfies the requirement.
- **Pick a different version** of the dependency that supports your current
  runtime.
- **Opt out**: remove `config.runtime` from `wpm.json` to skip the check
  entirely.

## Turning it off

Delete `config.runtime` from `wpm.json`, or just remove `runtime.wp` and
`runtime.php`. No other configuration change is needed.

## Related

- [Runtime compatibility](../fundamentals/runtime.md): the field reference and a
  worked example.
- [`wpm install`](../reference/cli/install.md): where the check runs.
- [`wpm.json`](../fundamentals/wpm-json.md): the manifest schema.
