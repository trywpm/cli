# wpm publish

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Publish a package to the wpm registry

### Options

| Name             | Type     | Default   | Description                                                         |
|:-----------------|:---------|:----------|:--------------------------------------------------------------------|
| `-a`, `--access` | `string` | `private` | Set the package access level to either public or private            |
| `--dry-run`      | `bool`   |           | Perform a publish operation without actually publishing the package |
| `--tag`          | `string` | `latest`  | Set the package tag                                                 |
| `--verbose`      | `bool`   |           | Enable verbose output                                               |


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Pack the current directory and upload it to the wpm registry as a new version of
your package.

`wpm publish` is the final step in releasing a plugin or theme. It reads
`wpm.json`, validates it, packs the project into a `.tar.zst` archive respecting
`.wpmignore`, computes a SHA-256 digest, and uploads the tarball along with a
manifest describing the release.

### Prerequisites

Before publishing, make sure:

- You're logged in: `wpm whoami` should print your username. The command refuses
  to publish if either `authToken` or `defaultUser` is missing from your config
  file.
- `wpm.json` exists and is valid. Required fields (`name`, `version`, `type`,
  and so on) are checked before anything is packed.
- `wpm.json` does _not_ set `"private": true`. That flag exists specifically to
  block accidental publishing; remove it (or use a separate config) when you are
  ready to release.

### What gets included

The tarball is built from the current directory. Files are filtered through
`.wpmignore`, which uses gitignore-style patterns. If no `.wpmignore` is
present, everything in the directory is packed.

Two soft limits apply:

| Limit          | Cap     | Behavior on overrun                                 |
| :------------- | :------ | :-------------------------------------------------- |
| `readme.md`    | 50 KiB  | Content is truncated; the rest is dropped silently. |
| Packed tarball | 128 MiB | Publishing aborts before upload.                    |

A `readme.md` (case-insensitive match) at the project root is read and attached
to the published manifest so the registry can render it. An empty tarball (zero
bytes after packing) is rejected.

### Summary block

Before any upload happens, wpm prints a tree-style summary of what it just
packed:

```
├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=
```

This is your last chance to verify before the upload starts. In `--dry-run`
mode, the command stops here and prints a `dry run complete` line; nothing is
sent to the registry.

### Tag and access

`--tag` (default `latest`) controls the dist tag assigned to this release. Use
it to publish pre-releases under a separate tag without moving `latest`:

```console
$ wpm publish --tag beta
```

Consumers can install from a tag with `wpm install my-pkg@beta`.

`--access` (`-a`, default `private`) controls who can see the package on the
registry. Pass `--access public` to make the release discoverable by everyone,
or leave it at `private` for organization-only releases. Any value other than
`public` or `private` is rejected.

<!-- prettier-ignore -->
> [!IMPORTANT]
> `--access private` is not the same as `"private": true` in
> `wpm.json`. `--access private` publishes the package with private visibility
> on the registry. `"private": true` blocks publishing entirely. See
> [wpm registry](../../fundamentals/registry.md) for the full distinction.

### Republishing a version

<!-- prettier-ignore -->
> [!CAUTION]
> A published version cannot be replaced. The registry rejects any
> publish for a version it already holds, and yanked versions stay reserved (you
> cannot reuse the number for a new build).

Each `version` in `wpm.json` is single-use. To release again, bump the `version`
field to a higher SemVer value and run `wpm publish` again.

### Verbose output

`--verbose` switches the packing phase from a single spinner to a line-by-line
listing of each file as it's added, with its size. Use it when you want to
confirm that `.wpmignore` is excluding the right things.

<!-- prettier-ignore -->
> [!TIP]
> Combine `--verbose --dry-run` to preview the exact file list and the
> final tarball size before a real publish. This is the safest way to catch a
> stray `node_modules` directory or a forgotten secrets file.

### Troubleshooting

- `user must be logged in to perform this action`: no auth token is in your
  config. Run `wpm auth login`.
- `no wpm.json found in the current directory`: run from the project root.
- `package marked as private cannot be published`: `"private": true` is set in
  `wpm.json`. Remove it, or work from a different config.
- `access must be either public or private`: `--access` got an invalid value.
  Use `public` or `private`.
- `tarball size is zero, cannot publish empty package`: `.wpmignore` excluded
  everything. Loosen the patterns or check that the directory is what you think
  it is.
- `tarball size exceeds 134217728 bytes, refusing to continue`: the packed
  archive went over 128 MiB. Add more entries to `.wpmignore`, remove binary
  blobs from the source tree, or move large assets to a CDN.
- Validation errors mentioning a specific field (`name`, `version`, `license`,
  ...): fix the field in `wpm.json` and re-run. See `wpm init` for the rules.
- **Readme not showing on the registry**: the file must be named `readme.md`
  (case-insensitive) at the project root, and under 50 KiB. Content beyond that
  is dropped silently. `wpm publish --verbose` shows whether the readme made it
  into the tarball.

## Examples

### Publish the current package

```console
$ wpm publish
📦 my-plugin@1.0.0

├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=

✔ published my-plugin@1.0.0
```

### Preview a publish without uploading

```console
$ wpm publish --dry-run

├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=

dry run complete, my-plugin@1.0.0 is ready to be published
```

### Publish a public package

```console
$ wpm publish --access public
```

### Publish under a pre-release tag

The `latest` tag is left alone; `wpm install my-plugin@beta` will pick this
build up.

```console
$ wpm publish --tag beta
```

### Inspect packed files

`--verbose` prints each file as it's added so you can confirm `.wpmignore` is
doing its job.

```console
$ wpm publish --verbose --dry-run
📦 my-plugin@1.0.0

packed 2.1 KB   wpm.json
packed 14.3 KB  readme.md
packed 1.2 MB   src/main.php
...

├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=

dry run complete, my-plugin@1.0.0 is ready to be published
```

### A package marked private cannot be published

```console
$ wpm publish
package marked as private cannot be published
```

Edit `wpm.json` and remove `"private": true` to allow publishing.
