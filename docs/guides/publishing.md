# Publishing packages

This guide walks the full publishing lifecycle: your first release, how to
choose version numbers and dist tags, and what to do when a release needs to
come back.

If you only want the command reference, see [`wpm publish`](../cli/publish.md).
If you haven't built a package yet, start with
[Getting started](../getting-started/index.md) and come back when you have a
`wpm.json` you'd like to share.

## Before you start

A few preconditions:

- You have a registry account and a personal access token. Create one from
  [your dashboard](https://wpm.so/dashboard/tokens).
- You're logged in. `wpm whoami` should print your username.
- Your `wpm.json` has the three required fields (`name`, `version`, `type`) and
  `"private": true` is **not** set.
- A `readme.md` exists at the project root if you want the registry to render
  documentation alongside the listing.
- A `.wpmignore` excludes anything you don't want in the published tarball:
  build artifacts, secrets, test fixtures, IDE files.

## Your first publish

Preview the publish before you upload. `--dry-run` packs the tarball locally and
prints the summary block, but skips the upload:

```console
$ wpm publish --dry-run
📦 my-plugin@0.1.0

├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=

dry run complete, my-plugin@0.1.0 is ready to be published
```

If the summary block looks right, drop `--dry-run`:

```console
$ wpm publish
📦 my-plugin@0.1.0

├─ Tag:     latest
├─ Access:  private
├─ Files:   42
├─ Size:    1.5 MB (3.4 MB unpacked)
└─ Digest:  9j6Q7l8s...=

✔ published my-plugin@0.1.0
```

The package is now on the registry. Anyone with read access can install it with
`wpm install my-plugin`.

<!-- prettier-ignore -->
> [!TIP]
> Run `wpm publish --verbose --dry-run` before every real publish. It
> prints every file that will be included, with sizes, so you can catch a stray
> `node_modules/` directory or a forgotten secrets file.

## Versioning your releases

Every published version must be strict SemVer (`X.Y.Z`, no `v` prefix). The
standard convention applies:

| Bump  | When                                                  | Example         |
| :---- | :---------------------------------------------------- | :-------------- |
| MAJOR | A change consumers' code or configuration depends on. | `1.0.0 → 2.0.0` |
| MINOR | A new feature, backward-compatible.                   | `1.0.0 → 1.1.0` |
| PATCH | A bug fix, no new behavior.                           | `1.0.0 → 1.0.1` |

For pre-1.0 packages, treat `0.X.Y → 0.(X+1).0` as a minor and
`0.X.Y → 0.X.(Y+1)` as a patch. Breaking changes are still possible across minor
bumps below 1.0, but be courteous to your users and call them out in release
notes.

### Pre-releases

Use a SemVer pre-release suffix to publish a build that consumers must opt into
explicitly:

| Version        | What it signals                                   |
| :------------- | :------------------------------------------------ |
| `1.2.0-beta.1` | A pre-release of 1.2.0; not picked up by default. |
| `1.2.0-rc.1`   | A release candidate of 1.2.0.                     |
| `1.2.0`        | The actual 1.2.0 release.                         |

Pre-releases sort below their release counterparts in SemVer. `wpm outdated`
will not surface `1.2.0-beta.1` as an update over `1.1.5`. Consumers have to
install the pre-release explicitly:

```console
$ wpm install my-plugin@1.2.0-beta.1
```

## Choosing a dist tag

`wpm publish` writes to the `latest` tag by default. That's what new consumers
pick up when they run `wpm install my-plugin`.

For pre-releases, publish under a separate tag so `latest` doesn't move:

```console
$ wpm publish --tag beta
```

Consumers then opt in with `wpm install my-plugin@beta`. Common conventions:

| Tag      | Use case                                                            |
| :------- | :------------------------------------------------------------------ |
| `latest` | The current stable release. Default for `wpm install`.              |
| `beta`   | Pre-releases for early testers.                                     |
| `next`   | Releases of the upcoming major while the current major still ships. |
| `lts`    | Long-term-support releases on an older major branch.                |

A dist tag always points at exactly one version. Publishing again with the same
`--tag` moves the tag to your new release. `latest` always points at whatever
you most recently published with the default `--tag`, unless you explicitly
chose a different tag.

## Visibility (public vs. private)

`--access public` makes the release discoverable by everyone the registry
serves. `--access private` keeps it limited to authorized accounts.

```console
$ wpm publish --access public
```

The default is `private`. To change visibility on an already-published version,
publish a new version with the new `--access` value; there is no separate
"change visibility" command.

<!-- prettier-ignore -->
> [!IMPORTANT]
> `--access private` is not the same as `"private": true` in
> `wpm.json`. The flag sets the registry's visibility. The manifest flag
> prevents publishing entirely. See
> [Registry concepts](../registry/index.md#private-true-is-not-the-same-as---access-private).

## Republishing the same version

<!-- prettier-ignore -->
> [!CAUTION]
> A published version cannot be replaced. The registry rejects any
> publish for a version it already holds, and yanked versions stay reserved (you
> cannot reuse the number for a new build).

This isn't just a registry convention; it's a load-bearing part of
reproducibility. `wpm.lock` pins versions by SHA-256 digest, so consumers count
on a given version always meaning the same bytes.

To release again, bump the `version` field in `wpm.json` to the next SemVer
value and run `wpm publish` again.

## Taking a release back

You can't undo a publish from the CLI. If you need to take a release back
(broken build, accidental secret leak, contractual issue), use the registry's
web interface to **yank** the release.

Yanking is reversible-ish: the version stays in the registry's history and
continues to work for consumers who already have it in their `wpm.lock`, but new
installs cannot pick it up. The number stays reserved either way; the next
publish has to be a new version.

## A typical release cycle

A simple flow for a stable package:

```sh
# 1. Make changes, run tests
$ vim src/main.php
$ ./vendor/bin/phpunit

# 2. Bump the version
$ vim wpm.json   # 1.0.0 → 1.0.1 for a bugfix

# 3. Preview
$ wpm publish --verbose --dry-run

# 4. Publish for real
$ wpm publish
```

A flow for shipping a pre-release of an upcoming major:

```sh
$ vim wpm.json           # 1.0.0 → 2.0.0-beta.1
$ wpm publish --tag beta
```

When the pre-release graduates:

```sh
$ vim wpm.json           # 2.0.0-beta.1 → 2.0.0
$ wpm publish            # Goes to latest
```

## Common mistakes

- **Publishing without bumping the version.** You'll get the "version already
  exists" error. Bump the `version` field in `wpm.json`.
- **Forgetting `.wpmignore`.** A 200 MB `node_modules` directory in the tarball,
  or worse, a `.env` with secrets. Preview every publish with
  `wpm publish --verbose --dry-run`.
- **Mixing tags.** Publishing a beta accidentally with `--tag latest` moves
  consumers' next install to the beta build. Double-check the summary block
  before confirming.
- **Leaving `"private": true` set.** The publish fails with
  `package marked as private cannot be published`. Edit `wpm.json` to remove the
  flag.

## Related

- [`wpm publish`](../cli/publish.md): every flag, with troubleshooting.
- [Registry concepts](../registry/index.md): tags, visibility, and the
  difference between `private: true` and `--access private`.
- [Authentication](../authentication/index.md): tokens and CI patterns.
- [`.wpmignore`](../wpmignore/index.md): syntax for the exclusion file.
- [`wpm.json`](../wpm-json/index.md): the manifest schema.
