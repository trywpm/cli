# wpm init

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Initialize a new WordPress package or init wpm in existing project

### Options

| Name          | Type     | Default | Description                             |
|:--------------|:---------|:--------|:----------------------------------------|
| `--existing`  | `bool`   |         | Init wpm.json for an existing project   |
| `--license`   | `string` |         | Package license                         |
| `--name`      | `string` |         | Package name                            |
| `--type`      | `string` |         | Package type (plugin, theme, mu-plugin) |
| `--version`   | `string` |         | Semver-compliant version                |
| `-y`, `--yes` | `bool`   |         | Skip prompts and use default values     |


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Create a `wpm.json` file in the current directory.

`wpm init` is the first command you run when adopting wpm in a project. It has
two modes:

- **New project** (default): walks you through creating a fresh `wpm.json` from
  scratch, either interactively or non-interactively with `-y`.
- **Existing project** (`--existing`): reads metadata out of an existing
  plugin's PHP headers, theme's `style.css`, and `readme.txt`, then writes a
  `wpm.json` that already reflects what the project declares about itself.

In both modes, individual flags act as overrides for the fields they target.

### New project mode

This is the default. It refuses to overwrite an existing `wpm.json` and exits
with an error if it finds one in the current directory.

When stdout is a terminal and `-y` is not set, wpm prompts for four fields in
order:

```console
$ wpm init
package name (my-plugin):
version (1.0.0):
license (GPL-2.0-or-later):
type (plugin):
```

Press enter on any prompt to accept the default shown in parentheses. The
default for `package name` is the current directory's name. Invalid input is
rejected and the prompt repeats.

When stdout is not a terminal (a pipe, a CI job, a subprocess), `-y` is forced
automatically so the command can run without interaction.

| Field   | Default                |
| :------ | :--------------------- |
| name    | current directory name |
| version | `1.0.0`                |
| license | `GPL-2.0-or-later`     |
| type    | `plugin` (prompt only) |

<!-- prettier-ignore -->
> [!NOTE]
> When you run `wpm init -y` without `--type`, the `type` field is left
> unset. Validation is skipped for that field in the `-y` path, so the command
> succeeds, but you will need to set `type` manually before publishing. Prefer
> `wpm init -y --type plugin` (or `theme`, `mu-plugin`) when scripting.

### Existing project mode (`--existing`)

Use this when you're adopting wpm in a plugin or theme that already exists. The
command is idempotent: if `wpm.json` is already present, it prints a notice and
exits cleanly.

Detection rules:

- If `style.css` is present in the current directory, the type is `theme`.
  Otherwise it is `plugin`.
- For `theme`: headers are parsed from `style.css`.
- For `plugin`: every `.php` file in the current directory is scanned, and the
  first one with valid plugin headers is treated as the main file.
- If a `readme.txt` is present (the WordPress.org readme format), its metadata
  is parsed and merged in.

Fields populated from the project sources:

| wpm.json field | Source                                                     |
| :------------- | :--------------------------------------------------------- |
| `name`         | current directory name (override with `--name`)            |
| `type`         | auto-detected (override with `--type`)                     |
| `version`      | plugin header `Version` or theme header `Version`          |
| `description`  | header `Description`, falling back to `readme.txt`         |
| `license`      | header `License`, falling back to `readme.txt` `License`   |
| `homepage`     | header `Plugin URI` or `Theme URI` (must be http/https)    |
| `team`         | header `Author`, falling back to `readme.txt` Contributors |
| `tags`         | `readme.txt` Tags, falling back to header `Tags`           |
| `requires.wp`  | header `Requires at least` and `readme.txt` Requires       |
| `requires.php` | header `Requires PHP` and `readme.txt` Requires PHP        |
| `dependencies` | header `Requires Plugins` (each pinned to `*`)             |

If a `readme.txt` exists but no `readme.md`, wpm also converts the
WordPress.org-flavored `readme.txt` into a Markdown `readme.md` next to it.

Constraints applied automatically:

- `tags` are trimmed to a maximum of 5, each 2 to 64 characters, deduplicated,
  and sorted.
- `team` is trimmed to a maximum of 100 entries, each 2 to 100 characters,
  deduplicated, and sorted.
- `description` is trimmed to at most 512 characters, preferring to cut at a
  sentence boundary.
- `license` is cleared if it falls outside 3 to 100 characters.
- `requires.wp` becomes `>=X` (and `<=Y` when `Tested up to` is also present and
  different from `Requires`).
- `requires.php` becomes `>=X`.

If no version can be extracted and `--version` is not provided, the command
errors. Pass `--version` to recover.

### Field validation rules

Both modes validate `name`, `version`, `type`, and `license` before writing
`wpm.json`. The full schema and its rules live in
[`wpm.json` reference](../../fundamentals/wpm-json.md).

### Troubleshooting

- `wpm.json already exists in <dir>`: you ran the default (new) mode in a
  directory that already has a config. Switch to `--existing` if you want to
  re-derive metadata, or delete the file first.
- `style.css not found in <dir>`: `--existing` detected a theme but the expected
  file is missing. Pass `--version` to skip header extraction, or run the
  command from the theme's root.
- `failed to identify main plugin file`: no `.php` file in the directory has a
  valid `Plugin Name` header. Pass `--version` to override, or add the standard
  WordPress plugin headers to your main file.
- `unable to determine version; please specify it with --version`: the command
  could not extract a version and you did not supply one. Pass `--version 1.0.0`
  (or whatever your project's version is).
- `name must consist of lowercase alphanumeric characters separated by hyphens`:
  package names cannot contain uppercase letters, spaces, or underscores. Rename
  with `--name my-package`.

## Examples

### Scaffold a new package with defaults

```console
$ wpm init -y --type plugin
config created at /work/awesome-plugin/wpm.json
```

### Walk through the interactive prompts

```console
$ wpm init
package name (awesome-plugin): awesome-plugin
version (1.0.0): 1.2.0
license (GPL-2.0-or-later): MIT
type (plugin): plugin
config created at /work/awesome-plugin/wpm.json
```

### Adopt wpm in an existing plugin

```console
$ wpm init --existing
using package type: plugin
main plugin file found: /work/awesome-plugin/awesome-plugin.php
readme.md created from readme.txt
wpm.json created at /work/awesome-plugin/wpm.json
```

### Adopt wpm in an existing theme with overrides

```console
$ wpm init --existing --name corporate-theme --version 2.0.0 --license MIT
wpm.json created at /work/corporate-theme/wpm.json
```

### Re-run `--existing` when wpm.json already exists

```console
$ wpm init --existing
wpm.json already exists at /work/awesome-plugin/wpm.json, skipping init.
```
