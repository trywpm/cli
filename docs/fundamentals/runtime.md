---
title: Runtime compatibility
description:
  How `requires` and `config.runtime` define WordPress and PHP compatibility,
  what your package supports and what you develop against.
---

`wpm.json` has two fields that talk about WordPress and PHP versions:

- **`requires`**: what your package needs from WordPress and PHP. "I need at
  least WordPress 6.0."
- **`config.runtime`**: what you're testing your package against. "I'm
  developing this on WordPress 6.9 and PHP 8.2."

They look similar but answer different questions. Here's the distinction at a
glance:

| Field            | Lives in   | Answers...                                  | Format                   |
| :--------------- | :--------- | :------------------------------------------ | :----------------------- |
| `requires`       | `wpm.json` | "What does my package need from the host?"  | SemVer constraint string |
| `config.runtime` | `wpm.json` | "What am I developing and testing against?" | Concrete SemVer version  |

The rest of this page covers each field in detail and how they cooperate during
`wpm install`.

## `requires`: what your package needs

`requires.wp` and `requires.php` are SemVer constraint strings. Every package on
the registry has them, so consumers can see what hosts you support.

```json
{
	"requires": {
		"wp": ">=6.0",
		"php": ">=7.4 <8.4"
	}
}
```

Examples of valid constraint strings:

| Constraint   | Matches                                         |
| :----------- | :---------------------------------------------- |
| `>=6.0`      | WordPress 6.0 and anything newer.               |
| `>=6.0 <7.0` | WordPress 6.0 through 6.x.                      |
| `^6.4`       | WordPress 6.4 through 6.x (caret = same major). |
| `~6.4`       | WordPress 6.4.x only (tilde = same minor).      |
| `*`          | Any version. Same as omitting the field.        |

Leave a constraint empty to mean "no opinion." That's the default when
`wpm init --existing` can't read it from your plugin or theme headers.

The `requires` block is informational by default. wpm doesn't check it against
your project unless you opt in (see below).

## `config.runtime`: what you target

`config.runtime` is your project's declared host environment:

```json
{
	"config": {
		"runtime": {
			"wp": "6.9",
			"php": "8.2"
		}
	}
}
```

The values are concrete version strings, not constraints. They are specific
versions you've tested with.

As soon as you set `runtime.wp` or `runtime.php`, wpm starts checking each
dependency's compatibility during install. This is **strict runtime mode**.
Here's how it works:

- If `requires.wp` is set on a dependency and your `runtime.wp` does not satisfy
  it, install fails with a message like:

  ```
  package akismet@5.3.1 incompatible:
    requires WordPress >=6.5, but runtime WordPress version is 6.4
  ```

- If `requires.php` is set and your `runtime.php` does not satisfy it, install
  fails with the same shape of error.

- If a dependency has no `requires` field, or that field is empty, the check
  passes silently.

Without `config.runtime`, wpm skips the check entirely. Installs go through
without testing compatibility.

## How they interact

A worked example. You're building `my-plugin`. You target WordPress 6.9 and PHP
8.2:

```json
{
	"name": "my-plugin",
	"version": "1.0.0",
	"type": "plugin",
	"requires": {
		"wp": ">=6.0",
		"php": ">=7.4"
	},
	"dependencies": {
		"akismet": "5.3.1"
	},
	"config": {
		"runtime": {
			"wp": "6.9",
			"php": "8.2"
		}
	}
}
```

Two things are happening here:

- **For consumers**: anyone who installs `my-plugin` as a dependency sees your
  `requires`. If they have strict mode on, their install fails when their host
  doesn't meet it.

- **For you**: every dependency's `requires` is checked against your
  `config.runtime` (6.9 / 8.2) during install. For `akismet@5.3.1`:
  - If it declares `requires.wp: ">=6.5"`, the check passes. Your 6.9 satisfies
    it.
  - If it declares `requires.php: ">=8.3"`, install fails. Your 8.2 doesn't
    satisfy it.

## When to opt in to strict mode

Set `config.runtime` when:

- You want CI to catch dependencies that quietly bumped their minimum WordPress
  or PHP version.
- You're testing your plugin against a specific WordPress and PHP matrix and
  want installs to fail fast on incompatible dependencies.
- You're locking down a long-term-support branch and want to be sure nothing in
  the tree drifts past your supported versions.

Leave it unset when:

- You're experimenting and want installs to succeed even if some dependencies
  declare conservative `requires`.
- Your project is a library that targets a broad range and you rely on the
  registry to surface compatibility issues separately.

You can toggle strict mode by adding or removing the runtime block; no other
configuration changes are needed.

## Troubleshooting

- `package <name>@<version> incompatible: requires <X> <constraint>, but runtime <X> version is <Y>`:
  a dependency in your tree declares a runtime requirement your `config.runtime`
  does not meet. Either bump your runtime, swap the dependency for a version
  that supports your target, or remove the runtime block to opt out of the
  check.
- `invalid <X> requirement in package <pkg>`: the package's `requires.<wp|php>`
  is not a valid SemVer constraint. This is a bug in the dependency's manifest;
  report it to the maintainer.
- `invalid runtime <X> version provided`: `config.runtime.<wp|php>` is not a
  SemVer version. Fix the value in your `wpm.json`.

## Related

- [`wpm.json` reference](wpm-json.md): the full schema and all field rules.
- [`wpm install`](../reference/cli/install.md): where the strict check runs.
- [Dependencies](dependencies.md): the difference between dependency specifiers
  (no ranges) and `requires` constraints (ranges allowed).
