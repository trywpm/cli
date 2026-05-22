# wpm auth login

<!---MARKER_GEN_START-->

Log in to the wpm registry

### Options

| Name      | Type     | Default | Description                     |
| :-------- | :------- | :------ | :------------------------------ |
| `--token` | `string` |         | Token to use for authentication |

<!---MARKER_GEN_END-->

## Description

Authenticate to the wpm registry and persist the credentials locally so other
commands can reuse them.

`wpm auth login` validates the token by calling the registry's identity
endpoint. On success it writes two fields to your config file
(`~/.wpm/config.json` by default):

- `authToken`: the token you supplied, stored base64-encoded
- `defaultUser`: the username the registry resolved the token to

After login, every command that needs registry access (such as `publish` and
`whoami`) reads these values automatically.

### Where to get a token

Create a personal access token from your account settings at
https://wpm.so/dashboard/tokens.

> [!WARNING] Tokens grant full access to the packages you own. Treat them like
> passwords: never commit them to version control, never paste them in chat, and
> rotate them immediately if you suspect a leak.

### Interactive login (recommended)

When stdin is a terminal and `--token` is not set, `wpm auth login` prompts for
the token with terminal echo turned off. The value is not printed on screen and
is not saved to your shell history.

```console
$ wpm auth login
Token:
welcome thelovekesh!
```

This is the safest way to log in on a workstation.

### Non-interactive login

For scripts and CI pipelines, pass the token with `--token`. wpm prints a
warning because the value may show up in shell history, in the process list, or
in build logs.

```console
$ wpm auth login --token "$WPM_TOKEN"
WARNING! Using --token via the CLI is insecure.
welcome ci-bot!
```

If you do not need to persist credentials, you can skip `wpm auth login`
entirely and let other commands read the token from the `WPM_TOKEN` environment
variable. See `wpm auth` for details.

### Working with multiple accounts

`wpm auth login` overwrites whatever token is currently stored in the active
config file. To keep separate sessions side by side, point each one at its own
config directory using the global `--config` flag or the `WPM_CONFIG`
environment variable:

```console
$ wpm --config ~/.wpm-work auth login
$ wpm --config ~/.wpm-personal auth login
```

Subsequent commands need the same `--config` value (or matching `WPM_CONFIG`) to
use the right credentials.

### Troubleshooting

- `token cannot be empty`: you pressed enter at the prompt without typing a
  token, or you passed `--token ""`. Re-run the command and supply a valid
  token.
- `failed to retrieve username`: the registry accepted the request but returned
  no username. This usually means the registry is misconfigured; report it to
  the registry operator.
- Validation errors during `validating token`: the token was rejected. Confirm
  it has not been revoked or expired. You can also hit this when the configured
  registry is unreachable, so double-check `--registry` and your network
  connection.

## Examples

### Log in interactively

```console
$ wpm auth login
Token:
welcome thelovekesh!
```

### Log in from a CI pipeline

```console
$ wpm auth login --token "$WPM_TOKEN"
WARNING! Using --token via the CLI is insecure.
welcome ci-bot!
```

### Log in to an isolated config directory

```console
$ wpm --config ~/.wpm-work auth login
Token:
welcome alice!
```
