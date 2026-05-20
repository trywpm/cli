# wpm auth

<!---MARKER_GEN_START-->
Authenticate with the wpm registry

### Subcommands

| Name                       | Description                   |
|:---------------------------|:------------------------------|
| [`login`](auth_login.md)   | Log in to the wpm registry    |
| [`logout`](auth_logout.md) | Log out from the wpm registry |



<!---MARKER_GEN_END-->

## Description

`wpm auth` groups the subcommands that manage your session with the wpm
registry. Use them to store a registry token on disk and to clear it when
you're done with the machine.

Once you log in, every other command that talks to the registry (such as
`install`, `publish`, `whoami`, and `outdated`) reuses the stored token
automatically. You typically only need to run `wpm auth login` once per
machine.

### Where credentials are stored

Credentials live in `config.json` inside your wpm config directory:

| Source                    | Effective config directory      |
|:--------------------------|:--------------------------------|
| Default                   | `~/.wpm`                        |
| `WPM_CONFIG` env variable | `$WPM_CONFIG`                   |
| `--config <dir>` flag     | `<dir>` (overrides the env var) |

The directory is created with mode `0700`, so only your user account can
read it. The token itself is base64-encoded inside `config.json`. That is
obfuscation, not encryption: anyone who can read the file can recover the
token, so treat the file like an SSH private key.

### Authenticating without `wpm auth login`

If no token is stored locally, registry calls fall back to the `WPM_TOKEN`
environment variable. This is the recommended pattern on CI runners and
ephemeral environments where credentials should not touch disk:

```console
$ export WPM_TOKEN="<your-token>"
$ wpm publish
```

Note that `wpm auth login` itself does not read `WPM_TOKEN`. It only accepts
a token through the interactive prompt or the `--token` flag. The fallback
applies to the other commands.
