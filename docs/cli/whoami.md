# wpm whoami

<!---MARKER_GEN_START-->
Display the current user


<!---MARKER_GEN_END-->

## Description

Print the username that the wpm registry resolves the current token to.

`wpm whoami` is the canonical way to confirm that authentication is working.
It picks up the token using the same lookup the rest of wpm uses:

1. `authToken` from your config file (`~/.wpm/config.json` by default),
   populated by `wpm auth login`.
2. The `WPM_TOKEN` environment variable, used only when the config file has
   no token.

The token is then sent to the registry's identity endpoint. The command
prints only the resolved username on stdout, followed by a newline. There
is no other formatting, which makes the output easy to capture in scripts:

```sh
user=$(wpm whoami)
```

This is a network call, not just a local file read. The username comes from
the registry, so it always reflects the *real* owner of the token, even if
the value cached in `defaultUser` has drifted (for example, after the
account was renamed).

### Choosing a different config or registry

`whoami` honors the global flags. To check identity against a non-default
config directory or registry, combine it with `--config` or `--registry`:

```console
$ wpm --config ~/.wpm-work whoami
$ wpm --registry registry.staging.wpm.so whoami
```

### Troubleshooting

- `failed to retrieve username`: the registry accepted the request but
  returned an empty username. This usually means the registry is
  misconfigured. Report it to the registry operator.
- Authentication errors from the registry: no token is stored locally and
  `WPM_TOKEN` is not set, or the token has been revoked or has expired. Run
  `wpm auth login` again, or refresh the value of `WPM_TOKEN`.

## Examples

### Check the currently logged-in user

```console
$ wpm whoami
thelovekesh
```

### Verify a CI token without logging in

```console
$ WPM_TOKEN="$WPM_TOKEN" wpm whoami
ci-bot
```

### Capture the username in a shell script

```console
$ user=$(wpm whoami)
$ echo "publishing as $user"
publishing as thelovekesh
```

### Check identity in an isolated config directory

```console
$ wpm --config ~/.wpm-work whoami
alice
```
