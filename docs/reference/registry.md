# Registry concepts

The wpm registry is the server that hosts published packages and serves them to
`wpm install`. By default wpm talks to `registry.wpm.so`. You can point it at a
different deployment with the global `--registry` flag.

This page covers the registry concepts you'll run into: dist tags, package
visibility, and the tricky difference between "don't publish this" and "publish
this privately."

## The default registry

Out of the box, every wpm command uses `registry.wpm.so`. Override it on a
per-invocation basis with `--registry`:

```console
$ wpm --registry registry.staging.wpm.so install
```

`--registry` is a global flag, so it must appear before the subcommand name.
There's no environment variable for it today. To set a different default across
all invocations, use a shell alias.

## Dist tags

A **dist tag** is a friendly label that points at a specific version. The
default tag is `latest`, which marks the version that new consumers should pick
up. Maintainers can create extra tags like `next`, `beta`, or `lts` to ship
pre-releases or parallel release lines without moving `latest`.

You set the tag with `--tag` at publish time:

```console
$ wpm publish --tag beta
```

Consumers select a tag by appending `@<tag>` to a package specifier just like a
version:

```console
$ wpm install akismet@latest
$ wpm install akismet@beta
```

A few important details:

- `--tag` defaults to `latest`. Publishing with no `--tag` updates `latest` to
  point at your new release.
- Once `wpm install` resolves a tag, the resulting `wpm.json` and `wpm.lock`
  record the concrete version, not the tag. The next install will use the
  version, not chase the tag. To follow a tag, re-run install with the `@<tag>`
  specifier.
- `wpm outdated` always checks against `latest`. Pre-release tags (`beta`, `rc`,
  and so on) won't surface as updates.
- Tag names follow the package name rules (lowercase, hyphens, no whitespace),
  with an upper bound of 64 characters.

## Package visibility

Every published package has a **visibility** setting on the registry. It is set
at publish time:

```console
$ wpm publish --access public
$ wpm publish --access private
```

The two valid values are `public` and `private`. The default is `private`. Any
other value is rejected.

- **`public`** packages are visible to everyone the registry serves.
- **`private`** packages are only visible to authorized accounts (typically
  members of the publishing organization).

Visibility is metadata on the published manifest. Changing it requires
publishing a new version with the desired `--access` value.

### `private: true` is not the same as `--access private`

<!-- prettier-ignore -->
> [!IMPORTANT]
> These two settings share a word but produce opposite outcomes.
> The table below makes the distinction explicit.

| Setting            | Lives in           | Effect                                                                                                  |
| :----------------- | :----------------- | :------------------------------------------------------------------------------------------------------ |
| `"private": true`  | `wpm.json`         | `wpm publish` refuses to upload the package at all. A tripwire on internal-only projects.               |
| `--access private` | `wpm publish` flag | Publish proceeds, and the registry stores the release with private visibility (the default if not set). |

If your project should never reach a registry, set `"private": true` in
`wpm.json`. If your project should be on the registry but only visible to your
organization, leave that out and publish with `--access private`. Trying to
publish a `"private": true` package fails with:

```
package marked as private cannot be published
```

There is no way to unset `private` from the command line: edit `wpm.json` to
remove the flag.

## Manifests on the registry

When you publish, wpm uploads two things together:

- A JSON **manifest** describing the release: name, version, type, `requires`,
  `dependencies`, `devDependencies`, tag, visibility, the SHA-256 digest of the
  tarball, and a few size counters.
- The **tarball** itself, a Zstandard-compressed tar archive built from your
  working directory (minus what `.wpmignore` excludes).

`wpm install` reads the manifest during resolution and uses it to fill out each
entry in `wpm.lock`. wpm fetches the tarball separately, only when it needs to
extract the package.

The 128 MiB cap on the packed tarball and the 50 KiB cap on the attached
`readme.md` are enforced client-side, before anything is uploaded.

## Self-hosted and staging registries

The wpm client treats the registry as a plain HTTP endpoint, so any deployment
that speaks the same API works. Common use cases:

- A staging deployment for testing changes against real packages without
  affecting the production registry.
- An internal mirror for organizations that prefer to host their own packages.

Switch by passing `--registry <url>`. Your token is tied to whichever registry
was active when you logged in. Logging in against a different registry
overwrites the existing token, unless you use a separate config directory:

```console
$ wpm --config ~/.wpm-staging --registry registry.staging.wpm.so auth login
$ wpm --config ~/.wpm-prod --registry registry.wpm.so auth login
```

After that, any command run with `--config ~/.wpm-staging` targets the staging
registry, and any command with `--config ~/.wpm-prod` targets production.

## Related

- [`wpm auth`](../cli/auth.md): how credentials are obtained and stored.
- [`wpm publish`](../cli/publish.md): the command that uploads new versions and
  sets the dist tag and visibility.
- [`wpm install`](../cli/install.md): the command that reads manifests and
  downloads tarballs.
- [`wpm.json`](../reference/wpm-json.md): the `private` flag, distinct from
  registry visibility.
