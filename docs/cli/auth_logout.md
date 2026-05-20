# wpm auth logout

<!---MARKER_GEN_START-->
Log out from the wpm registry


<!---MARKER_GEN_END-->

## Description

Clear locally stored wpm credentials.

`wpm auth logout` removes the `authToken` and `defaultUser` fields from your
config file (`~/.wpm/config.json` by default) and rewrites it. Other entries
in the file, such as per-user or per-plugin tokens, are left untouched.

### What this does and does not do

`logout` is a local operation. It only edits the file on your disk. It does
not contact the registry, and the token itself remains valid on the server
until you revoke it.

If you need to invalidate a token (for example, because it was leaked or
you are decommissioning a CI runner), revoke it from your account settings
at https://wpm.so/account/tokens before or in addition to running this
command.

### Errors

If no token is stored locally, the command exits with:

```
user must be logged in to perform this action
```

This is harmless; it just means there is nothing to remove. To check the
current state at any time, run `wpm whoami`.

## Examples

### Log out from the default config

```console
$ wpm auth logout
user logged out successfully
```

### Log out from a specific config directory

```console
$ wpm --config ~/.wpm-work auth logout
user logged out successfully
```
