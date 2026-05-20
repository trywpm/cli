# Authentication

This page consolidates everything wpm does around identity:
how tokens are obtained, where they're stored, how they're chosen at
request time, and how to manage multiple identities or rotate
credentials.

For the per-command reference, see [`wpm auth`](../cli/auth.md),
[`wpm auth login`](../cli/auth_login.md), and
[`wpm whoami`](../cli/whoami.md).

## Why wpm needs a token

Most wpm commands talk to the registry. The registry decides what
you're allowed to see (private packages, your organization's
packages) and what you're allowed to do (publish under a name you
own). It identifies you by the **token** your client sends with
each request.

A token is a long, opaque string issued by the registry. It carries
your account's identity and permissions. Treat it like a password:
anyone with the token can act as you against the registry.

## Getting a token

Create a personal access token from your account settings on the
registry's web interface, typically at
`https://wpm.so/account/tokens`.

A token can usually be scoped (read-only, publish-only) and given
an expiration. The exact options depend on the registry; consult the
registry's documentation for what's available.

Once issued, the token is shown to you once. Copy it somewhere safe.
You cannot recover it later from the registry; if it leaks, revoke
it and issue a new one.

## Logging in

`wpm auth login` validates a token against the registry and stores
it locally. The recommended flow on a personal machine is the
interactive prompt, which disables terminal echo so the token is
not written to your shell history:

```console
$ wpm auth login
Token: 
welcome <your-username>!
```

On a workstation, this is the safest path.

In automated environments (CI runners, image builds), pass the
token directly with `--token`. wpm prints a warning because the
flag value can appear in the process list and in build logs:

```console
$ wpm auth login --token "$WPM_TOKEN"
WARNING! Using --token via the CLI is insecure.
welcome ci-bot!
```

For CI, you usually don't even need to log in. Setting `WPM_TOKEN`
in the environment makes wpm pick the token up automatically (see
"How tokens are resolved" below).

## Where the token is stored

After login, wpm writes two fields into your client config file:

| Field         | What it holds                                              |
|:--------------|:-----------------------------------------------------------|
| `authToken`   | The token itself, base64-encoded.                          |
| `defaultUser` | The username the registry resolved the token to.           |

The file lives at `~/.wpm/config.json` by default. You can move it
with the `--config` flag or the `WPM_CONFIG` environment variable.

A few security notes:

- The base64 encoding is **obfuscation, not encryption.** Anyone who
  can read the file can recover the token. Restrict file access.
- The directory itself is created with mode `0700`, so only your
  user account can list its contents.
- The token is sent over HTTPS to the registry.

## How tokens are resolved

Whenever wpm needs to authenticate, it looks for a token in this
order:

1. The `authToken` field in your client config file.
2. The `WPM_TOKEN` environment variable, used only when (1) is
   empty.

If both are missing, the registry rejects the request with an
authentication error. wpm reports whatever the registry returns.

A subtle point: `wpm auth login` itself does **not** read
`WPM_TOKEN`. It only takes a token from `--token` or the
interactive prompt. The environment fallback applies to every
other command (install, publish, whoami, and so on).

## Logging out

`wpm auth logout` clears the `authToken` and `defaultUser` fields
from the config file. It's a local operation; the token itself
remains valid on the registry until you revoke it from your account
settings.

If a token is leaked, treat the registry revocation as the real
fix. `wpm auth logout` only protects the machine you ran it on.

## Multiple identities

You can keep multiple wpm identities side by side by pointing each
one at its own config directory. The most common reason to do this
is to switch between personal and work accounts, or between a
production registry and a staging one.

```console
$ wpm --config ~/.wpm-personal auth login
Token: 
welcome alice!

$ wpm --config ~/.wpm-work --registry registry.org.example auth login
Token: 
welcome alice-at-org!
```

Subsequent commands need the same `--config` value to find the
right token:

```console
$ wpm --config ~/.wpm-personal whoami
alice

$ wpm --config ~/.wpm-work whoami
alice-at-org
```

A few practical patterns:

- Use shell aliases (`alias wpm-work='wpm --config ~/.wpm-work'`)
  to keep invocations short.
- Set `WPM_CONFIG` in a directory-scoped tool like
  [direnv](https://direnv.net) so the right identity is picked up
  automatically when you `cd` into a project.

## CI patterns

For one-off jobs, skip `wpm auth login` and use the environment:

```sh
export WPM_TOKEN="<token>"
wpm install
```

For longer-lived runners that should publish under a specific
identity:

```sh
wpm auth login --token "$WPM_TOKEN"
wpm publish --access public
```

For multi-tenant CI (publishing under more than one account from the
same runner), use a separate `--config` directory per identity. See
the [CI/CD guide](../guides/ci.md) for full recipes.

## Rotating tokens

The recommended rotation process:

1. Issue a new token from the registry's web interface.
2. On every machine and CI runner that uses the old token, run
   `wpm auth login` (or update the `WPM_TOKEN` secret) with the new
   value.
3. Verify each one with `wpm whoami`.
4. Revoke the old token from the registry.

`wpm whoami` is your friend during rotation. It's the only command
that hits the registry without doing anything else, so it's a safe
way to check that a token is recognized.

## Troubleshooting

- **`token cannot be empty`**: you pressed Enter at the interactive
  prompt without typing anything. Re-run `wpm auth login`.
- **`failed to retrieve username`**: the registry accepted the
  request but returned an empty username. This usually means the
  registry is misconfigured. Report it to the registry's operator.
- **Authentication errors from the registry**: the token is
  unrecognized. It may have been revoked, may have expired, or may
  be for a different registry. Run `wpm whoami` against the
  registry you intend to use, and confirm the token is current.
- **`user must be logged in to perform this action`** (during
  `wpm publish`): no token is stored locally. Run `wpm auth login`
  or set `WPM_TOKEN`. Note that `publish` does not read `WPM_TOKEN`
  as a fallback in the same way other commands do; you need to be
  logged in.

## Related

- [`wpm auth`](../cli/auth.md): the parent command for `login` and
  `logout`.
- [`wpm whoami`](../cli/whoami.md): the canonical "am I logged in?"
  check.
- [Registry concepts](../registry/index.md): visibility, dist
  tags, and what the registry does with your identity.
- [CI/CD guide](../guides/ci.md): worked examples for running wpm
  from build pipelines.
