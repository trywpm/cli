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

wpm rejects the entire manifest if any required field is missing or invalid.
`wpm publish` always validates first. Commands that change the file
(`wpm install`, `wpm uninstall`) validate the new version before saving it.

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

wpm rejects ASCII control characters and a few look-alike Unicode code points
(zero-width separators and similar) in `tags`, `team`, `description`, and
`license`.

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

| Field      | Type   | Notes                                                                                    |
| :--------- | :----- | :--------------------------------------------------------------------------------------- |
| `requires` | object | What your package needs from WordPress and PHP. See [Runtime compatibility](runtime.md). |

`requires.wp` and `requires.php` are SemVer constraint strings, for example
`>=6.0` or `>=7.4 <8.2`. Leave a field empty to mean "no opinion."

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

You can edit `wpm.json` directly. wpm keeps your indentation style (two spaces
by default). Commands that change the file (`wpm install`, `wpm uninstall`)
rewrite it the same way.

A few things to know:

- After editing dependency entries by hand, run `wpm install` to bring
  `wpm.lock` and `wp-content/` back in sync.
- Validation errors tell you which field broke, by path (for example, `tags[2]`
  or `config.content-dir`).
- If another tool writes an invalid manifest, the next wpm command that
  validates (like `wpm publish`) will fail with a clear message pointing at the
  bad field.

## Related

- [Dependencies](dependencies.md): how `dependencies` and `devDependencies`
  interact with `wpm install`.
- [Runtime compatibility](runtime.md): when wpm enforces `requires` during
  install, and how `config.runtime` participates.
- [`wpm.lock`](wpm-lock.md): the resolved snapshot that pairs with `wpm.json`.
- [`.wpmignore`](wpmignore.md): controls what `wpm publish` packs.
- [Registry concepts](registry.md): dist tags, visibility, and the difference
  between `private: true` and `--access private`.
