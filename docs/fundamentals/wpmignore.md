# .wpmignore

`.wpmignore` is an optional file at the root of your project. It tells
`wpm publish` which paths to exclude from the tarball that gets uploaded to the
registry. Without it, every file in the project directory is packed.

The syntax is the same as `.gitignore`. If you've used `.gitignore`, you already
know how to write a `.wpmignore`.

## A small example

```gitignore
# Build artifacts
dist/
node_modules/

# Anything that came out of your editor
.idea/
.vscode/
*.swp

# Files for local development only
.env
.env.local
docker-compose.override.yml

# wpm's own bookkeeping
wpm.lock

# Tests don't belong in a published package
tests/
phpunit.xml
*.test.php
```

The intent is to ship only the runtime code, assets, and the readme. Everything
else stays out.

## Pattern syntax

The patterns follow gitignore semantics. The most useful subset:

| Pattern          | Matches                                                                                 |
| :--------------- | :-------------------------------------------------------------------------------------- |
| `name`           | A file or directory called `name` anywhere in the tree.                                 |
| `name/`          | A directory called `name` (and everything under it).                                    |
| `/name`          | A file or directory called `name` only at the project root.                             |
| `*.log`          | Any file with the `.log` extension at any depth.                                        |
| `build/*.zip`    | `.zip` files directly inside `build/` (one level only).                                 |
| `**/cache`       | A `cache` entry at any depth (the `**` matches any number of intermediate directories). |
| `!important.log` | Re-include a file that an earlier rule excluded.                                        |
| `# comment`      | A comment. The whole line is ignored.                                                   |

A few practical reminders:

- Patterns are applied to paths relative to the project root.
- The order of rules matters: a later `!pattern` line can re-include something
  an earlier line excluded.
- Whitespace at the end of a line is significant; trim accidental trailing
  spaces.
- Empty lines are skipped.

## What gets packed

`wpm publish` walks your project directory and checks each file against the
patterns. Anything that matches a pattern gets dropped from the tarball.

A few wpm-specific notes about what to include and exclude:

- **`wpm.json`** must be present in the tarball. Don't exclude it.
- **`readme.md`** at the project root is read and attached to the published
  manifest as the package's readme. Capped at 50 KiB; if you exclude it, the
  registry won't render a readme.
- **`wpm.lock`** is usually safe to exclude from published packages. Consumers
  run their own resolver.
- **`.wpmignore` itself** is excluded automatically by the underlying archive
  code; you don't need to list it.
- Source control directories (`.git`, `.hg`, `.svn`) are not excluded
  automatically. Add them to `.wpmignore` if you don't want to ship a copy of
  your VCS metadata.

## Scope

`.wpmignore` only affects `wpm publish`. It doesn't change what `wpm install`,
`wpm ls`, or any other command do. Installed dependencies under `wp-content/`
are untouched by your project's `.wpmignore`.

A dependency might have its own `.wpmignore` in its source, but that only shaped
the tarball when the maintainer published it. By the time wpm extracts the
package on your machine, the file isn't there.

## Sizing constraints

`wpm publish` caps the packed tarball at 128 MiB. If you see this error:

```
tarball size exceeds 134217728 bytes, refusing to continue
```

your project is too big even after `.wpmignore` is applied. Common culprits:

- A `node_modules/` or `vendor/` directory that wasn't excluded.
- Pre-built binaries or large images committed to the source tree.
- Test fixtures that were never meant to ship.

Add the offending paths to `.wpmignore` and try again.

If `.wpmignore` excludes too much and the tarball is empty, `wpm publish` errors
with:

```
tarball size is zero, cannot publish empty package
```

Loosen the patterns until at least one file is included.

## Preview what will be packed

The fastest way to check `.wpmignore` is a verbose dry run:

```console
$ wpm publish --verbose --dry-run
```

Each included file is printed with its packed size before the summary block. Use
this to confirm that the right files are in and the right files are out.

## Related

- [`wpm publish`](../reference/cli/publish.md): the only command that reads
  `.wpmignore`.
- [`wpm.json`](wpm-json.md): the manifest whose required fields must remain in
  the tarball.
