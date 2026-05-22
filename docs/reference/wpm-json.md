# wpm.json

`wpm.json` is the manifest at the root of every wpm package. It declares what
the package is, what it depends on, and which versions of WordPress and PHP it
targets. Every wpm command that touches the current project reads this file.

## A minimal manifest

The three required fields are `name`, `version`, and `type`. Everything else is
optional.

```json
{
	"name": "my-awesome-plugin",
	"version": "1.0.0",
	"type": "plugin"
}
```

## A complete example

The following manifest exercises every field. Use it as a reference when you're
not sure what a section looks like.

```json
{
	"name": "my-awesome-plugin",
	"description": "Doing awesome things since 1066.",
	"type": "plugin",
	"version": "1.0.0",
	"private": false,
	"license": "GPL-2.0-or-later",
	"homepage": "https://example.com/my-awesome-plugin",
	"tags": ["seo", "performance"],
	"team": ["alice", "bob"],
	"requires": {
		"wp": ">=6.0",
		"php": ">=7.4"
	},
	"dependencies": {
		"akismet": "5.3.1",
		"hello-dolly": "*"
	},
	"devDependencies": {
		"query-monitor": "3.20.2"
	},
	"config": {
		"bin-dir": "wp-bin",
		"content-dir": "wp-content",
		"runtime": {
			"wp": "6.9",
			"php": "8.2"
		}
	}
}
```

## Field reference

### Required fields

| Field     | Type   | Rules                                                                                 |
| :-------- | :----- | :------------------------------------------------------------------------------------ |
| `name`    | string | 3 to 164 characters; lowercase alphanumeric and hyphens (`^[a-z0-9]+(-[a-z0-9]+)*$`). |
| `version` | string | Strict SemVer `X.Y.Z`; 5 to 64 characters; no `v` prefix.                             |
| `type`    | string | One of `plugin`, `theme`, or `mu-plugin`.                                             |

Validation fails for the entire manifest if any of these are missing or invalid.
`wpm publish` always validates before packing; commands that modify the file
(such as `wpm install` and `wpm uninstall`) validate after editing it in memory.

### Optional metadata

| Field         | Type     | Rules                                                                                                  |
| :------------ | :------- | :----------------------------------------------------------------------------------------------------- |
| `description` | string   | 3 to 512 characters.                                                                                   |
| `private`     | boolean  | When `true`, `wpm publish` refuses to upload. Use it as a tripwire on internal-only packages.          |
| `license`     | string   | 3 to 100 characters. SPDX-style identifiers like `GPL-2.0-or-later` are conventional but not enforced. |
| `homepage`    | string   | A `http` or `https` URL, 10 to 200 characters.                                                         |
| `tags`        | string[] | Up to 5 entries; each 2 to 64 characters; duplicates rejected.                                         |
| `team`        | string[] | Up to 100 entries; each 2 to 100 characters; duplicates rejected.                                      |

`tags` are short keywords the registry uses for search and discovery. Pick words
a consumer would type when looking for your package, like `seo`, `caching`, or
`payments`. Tags are not categories; they're free-form.

`team` lists the people or organizations responsible for the package, typically
by username or display name. wpm populates this automatically from plugin and
theme headers (`Author`, `Contributors`) when you run `wpm init --existing`.

`tags` and `team` strings cannot contain ASCII control characters or a handful
of look-alike Unicode code points (zero-width separators and similar). The same
restriction applies to `description` and `license`.

### Dependencies

| Field             | Type                   | Notes                                                                                    |
| :---------------- | :--------------------- | :--------------------------------------------------------------------------------------- |
| `dependencies`    | object<string, string> | Production dependencies. Installed by default. Up to 16 entries.                         |
| `devDependencies` | object<string, string> | Development dependencies. Skipped when `wpm install --no-dev` is used. Up to 16 entries. |

Each key must satisfy the package name rules. Each value must be strict SemVer
or `*`. See [Dependencies](dependencies.md) for the full behavior, including how
`wpm install`, `wpm uninstall`, and the conflict resolver interact with these
maps.

### Compatibility

| Field      | Type   | Notes                                                                                                           |
| :--------- | :----- | :-------------------------------------------------------------------------------------------------------------- |
| `requires` | object | Version constraints YOUR PACKAGE places on the WordPress and PHP host. See [Runtime compatibility](runtime.md). |

`requires.wp` and `requires.php` are SemVer constraint strings (for example,
`>=6.0`, `>=7.4 <8.2`). Empty constraints are allowed and mean "no opinion."

### Build and runtime configuration

| Field     | Type                   | Notes                                                                                              |
| :-------- | :--------------------- | :------------------------------------------------------------------------------------------------- |
| `config`  | object                 | Project-relative paths and the runtime you're developing against.                                  |
| `bin`     | object<string, string> | Reserved. Binary linking is declared by some packages but not yet wired into install.              |
| `scripts` | object<string, string> | Reserved. Lifecycle scripts are not yet wired up; `wpm install --ignore-scripts` is a no-op today. |

The `config` object accepts:

- `bin-dir` (string, default `wp-bin`): where wpm would place executable links.
  Currently informational; binary linking is not yet wired up. Must be a
  relative, in-tree path.
- `content-dir` (string, default `wp-content`): where `wpm install` extracts
  packages. Must be a relative, in-tree path.
- `runtime` (object): the WordPress and PHP versions your project runs against.
  Setting either `runtime.wp` or `runtime.php` turns on a strict compatibility
  check during `wpm install`. See [Runtime compatibility](runtime.md).

## Editing the file by hand

You can edit `wpm.json` directly. wpm preserves whatever indentation it detects
(two-space by default). Commands that modify the file (`wpm install`,
`wpm uninstall`) rewrite it with the same indentation.

A few practical notes:

- Run `wpm install` after editing dependency entries so `wpm.lock` and the
  filesystem stay in sync.
- Validation errors reference the offending field by path (for example,
  `tags[2]` or `config.content-dir`). Use that path to locate the problem.
- If an invalid manifest is written by a tool that doesn't validate, the next
  operation that does validate (publish, or any read in a stricter context) will
  fail with a clear message.

## Related

- [Dependencies](dependencies.md): how `dependencies` and `devDependencies`
  interact with `wpm install`.
- [Runtime compatibility](runtime.md): when wpm enforces `requires` during
  install, and how `config.runtime` participates.
- [`wpm.lock`](../reference/wpm-lock.md): the resolved snapshot that pairs with
  `wpm.json`.
- [`.wpmignore`](../reference/wpmignore.md): controls what `wpm publish` packs.
- [Registry concepts](../reference/registry.md): dist tags, visibility, and the
  difference between `private: true` and `--access private`.
