# Runtime compatibility

`wpm.json` has two fields that talk about WordPress and PHP versions:

- **`requires`**: the constraints YOUR PACKAGE places on the host environment.
  "I need at least WordPress 6.0."
- **`config.runtime`**: the runtime versions YOU ARE DEVELOPING AGAINST. "I'm
  testing this against WordPress 6.9 and PHP 8.2."

They look similar and they cooperate, but they answer different questions.
Here's the distinction at a glance:

| Field            | Lives in   | Answers...                                  | Format                   |
| :--------------- | :--------- | :------------------------------------------ | :----------------------- |
| `requires`       | `wpm.json` | "What does my package need from the host?"  | SemVer constraint string |
| `config.runtime` | `wpm.json` | "What am I developing and testing against?" | Concrete SemVer version  |

The rest of this page covers each field in detail and how they cooperate during
`wpm install`.

## `requires`: what your package needs

`requires.wp` and `requires.php` are SemVer constraint strings. They live on
every package's manifest and become metadata that the registry serves to
consumers.

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

Empty constraints are allowed and mean "no opinion." That's the default when
`wpm init --existing` cannot infer one from your plugin or theme headers.

The `requires` block is informational by default. Nothing in wpm checks it for
your project unless you opt in.

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

Setting **either** `runtime.wp` or `runtime.php` switches wpm into **strict
runtime mode** for this project. In strict mode, every package that
`wpm install` resolves has its `requires` checked against your runtime:

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

When `config.runtime` is **not** set, the strict check is skipped entirely.
Installs go through without checking compatibility.

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

Two things are happening:

- Anyone who installs `my-plugin` as a dependency sees your `requires` and (if
  they have strict mode on) gets blocked if their host doesn't meet it.
- When you run `wpm install`, every direct or transitive dependency has its
  `requires` checked against your `config.runtime` (6.9 / 8.2). If
  `akismet@5.3.1` declares `requires.wp: ">=6.5"`, the check passes (6.9
  satisfies it). If it declares `requires.php: ">=8.3"`, install fails (8.2
  doesn't satisfy it).

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
- [`wpm install`](../cli/install.md): where the strict check runs.
- [Dependencies](dependencies.md): the difference between dependency specifiers
  (no ranges) and `requires` constraints (ranges allowed).
