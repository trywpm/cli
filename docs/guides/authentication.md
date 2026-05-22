---
title: Authentication
description:
  How to generate a wpm token, where wpm stores it on disk, and how to manage
  multiple registry accounts from a single machine.
---

This page covers everything you need to know about wpm tokens: how to get one,
where wpm stores it, and how to manage multiple accounts.

For the per-command reference, see [`wpm auth`](../reference/cli/auth.md),
[`wpm auth login`](../reference/cli/auth_login.md), and
[`wpm whoami`](../reference/cli/whoami.md).

## Why wpm needs a token

Most wpm commands talk to the registry. The registry needs to know who you are
before it shows you private packages or lets you publish under a name. It
identifies you by the **token** that wpm sends with each request.

A token is a long string of letters and numbers that proves who you are. Treat
it like a password: anyone who has it can act as you.

## Getting a token

Create a personal access token from your account settings on the registry's web
interface, typically at `https://wpm.so/dashboard/tokens`.

A token can usually be scoped (read-only, publish-only) and given an expiration.
The exact options depend on the registry; consult the registry's documentation
for what's available.

Once issued, the token is shown to you once. Copy it somewhere safe. You cannot
recover it later from the registry; if it leaks, revoke it and issue a new one.

## Logging in

`wpm auth login` checks your token with the registry, then saves it locally.

On your own machine, use the interactive prompt. wpm hides the token as you
type, so it doesn't end up in your shell history:

```console
$ wpm auth login
Token:
welcome <your-username>!
```

In CI runners and image builds, pass the token with `--token`. wpm prints a
warning because the flag value can show up in process lists and build logs:

```console
$ wpm auth login --token "$WPM_TOKEN"
WARNING! Using --token via the CLI is insecure.
welcome ci-bot!
```

In CI, you usually don't need to log in at all. Set `WPM_TOKEN` in the
environment and other wpm commands will use it automatically. See "How tokens
are resolved" below.

## Where the token is stored

After login, wpm writes two fields into your client config file:

| Field         | What it holds                                    |
| :------------ | :----------------------------------------------- |
| `authToken`   | The token itself, base64-encoded.                |
| `defaultUser` | The username the registry resolved the token to. |

The file lives at `~/.wpm/config.json` by default. You can move it with the
`--config` flag or the `WPM_CONFIG` environment variable.

<!-- prettier-ignore -->
> [!CAUTION]
> The base64 encoding of `authToken` is obfuscation, not encryption.
> Anyone with read access to `config.json` can recover your token. Treat the
> file like an SSH private key.

A few additional security notes:

- The directory itself is created with mode `0700`, so only your user account
  can list its contents.
- The token is sent over HTTPS to the registry.

## How tokens are resolved

wpm finds your token in this order:

1. The `authToken` field in your client config file.
2. The `WPM_TOKEN` environment variable, used only when (1) is empty.

If neither is set, the registry rejects the request with an authentication
error.

One catch: `wpm auth login` itself does **not** read `WPM_TOKEN`. It only
accepts a token from `--token` or the interactive prompt. The environment
fallback applies to every other command (install, publish, whoami, and so on).

## Logging out

`wpm auth logout` removes the `authToken` and `defaultUser` fields from your
config file. This is a local change. The token itself stays valid on the
registry until you revoke it from your dashboard.

If a token leaks, you have to revoke it from the registry. `wpm auth logout`
only protects the machine where you ran it.

## Multiple identities

You can keep multiple wpm identities side by side. Point each one at its own
config directory. This is useful when you have personal and work accounts, or
when you switch between production and staging registries.

```console
$ wpm --config ~/.wpm-personal auth login
Token:
welcome alice!

$ wpm --config ~/.wpm-work --registry registry.org.example auth login
Token:
welcome alice-at-org!
```

Subsequent commands need the same `--config` value to find the right token:

```console
$ wpm --config ~/.wpm-personal whoami
alice

$ wpm --config ~/.wpm-work whoami
alice-at-org
```

A few practical patterns:

- Use shell aliases (`alias wpm-work='wpm --config ~/.wpm-work'`) to keep
  invocations short.
- Set `WPM_CONFIG` in a directory-scoped tool like [direnv](https://direnv.net)
  so the right identity is picked up automatically when you `cd` into a project.

## CI patterns

For one-off jobs, skip `wpm auth login` and use the environment:

```sh
export WPM_TOKEN="<token>"
wpm install
```

For longer-lived runners that should publish under a specific identity:

```sh
wpm auth login --token "$WPM_TOKEN"
wpm publish --access public
```

For multi-tenant CI (publishing under more than one account from the same
runner), use a separate `--config` directory per identity. See the
[CI/CD guide](ci.md) for full recipes.

## Rotating tokens

The recommended rotation process:

1. Issue a new token from the registry's web interface.
2. On every machine and CI runner that uses the old token, run `wpm auth login`
   (or update the `WPM_TOKEN` secret) with the new value.
3. Verify each one with `wpm whoami`.
4. Revoke the old token from the registry.

`wpm whoami` is handy during rotation. It hits the registry without changing
anything, so it's a safe way to confirm that a token works.

## Troubleshooting

- **`token cannot be empty`**: you pressed Enter at the interactive prompt
  without typing anything. Re-run `wpm auth login`.
- **`failed to retrieve username`**: the registry accepted the request but
  returned an empty username. This usually means the registry is misconfigured.
  Report it to the registry's operator.
- **Authentication errors from the registry**: the token is unrecognized. It may
  have been revoked, may have expired, or may be for a different registry. Run
  `wpm whoami` against the registry you intend to use, and confirm the token is
  current.
- **`user must be logged in to perform this action`** (during `wpm publish`): no
  token is stored locally. Run `wpm auth login` or set `WPM_TOKEN`. Note that
  `publish` does not read `WPM_TOKEN` as a fallback in the same way other
  commands do; you need to be logged in.

## Related

- [`wpm auth`](../reference/cli/auth.md): the parent command for `login` and
  `logout`.
- [`wpm whoami`](../reference/cli/whoami.md): the canonical "am I logged in?"
  check.
- [wpm registry](../fundamentals/registry.md): visibility, dist tags, and what
  the registry does with your identity.
- [CI/CD guide](ci.md): worked examples for running wpm from build pipelines.
