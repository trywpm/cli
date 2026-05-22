# Installation

wpm ships as a single static binary. There are no runtime dependencies, no PHP
requirement on the machine that runs wpm, and no per-project toolchain. Pick the
install method that fits your environment.

## Linux and macOS

The recommended method is the install script. It detects your platform,
downloads the matching release binary from GitHub, and drops it on your `PATH`.

```sh
curl -fsSL https://wpm.so/install | bash
```

The script removes any previous binary at `/usr/local/bin/wpm` (using `sudo`)
before installing the new one. You can also pin a specific release:

```sh
curl -fsSL https://wpm.so/install | bash -s -- 0.1.0
```

Supported platforms:

| OS    | Architectures                            |
| :---- | :--------------------------------------- |
| Linux | `amd64`, `arm64`                         |
| macOS | `amd64` (Intel), `arm64` (Apple Silicon) |

If your platform isn't on the list, build from source (see below).

## Windows

Run from PowerShell:

```powershell
powershell -c "irm wpm.so/install.ps1|iex"
```

This pulls the Windows release binary and installs it on your `PATH`.

## Docker

A pre-built image is available on Docker Hub:

```sh
docker pull trywpm/cli
```

Run wpm against your project by mounting it as a volume:

```sh
docker run --rm -v "$PWD":/work -w /work trywpm/cli wpm install
```

The image is useful for CI pipelines that don't want to manage binaries
themselves, and for ephemeral environments where you don't want wpm on the host.

## Go toolchain

If you have Go installed, you can install wpm directly from source:

```sh
go install go.wpm.so/cli/cmd/wpm@latest
```

The binary lands in `$GOPATH/bin` (typically `~/go/bin`). Make sure that
directory is on your `PATH`. To pin a release:

```sh
go install go.wpm.so/cli/cmd/wpm@v0.1.0
```

## Build from source

For development, a fork, or an unsupported platform:

```sh
git clone https://github.com/trywpm/cli wpm
cd wpm
go build -o wpm ./cmd/wpm
```

The resulting `wpm` binary in the current directory is fully functional. Move it
onto your `PATH` to use it from anywhere:

```sh
sudo mv wpm /usr/local/bin/wpm
```

## Shell completion

The install script for Linux and macOS, and the PowerShell installer for
Windows, set up shell completion for you. There's nothing extra to do if you
used one of them.

What the installers do:

- **Linux and macOS (`install.sh`)**: generates completion scripts under
  `~/.wpm/completions/` for `bash`, `zsh`, and `fish`, then adds a one-line
  `source` to your shell's startup file (`.bashrc`, `.zshrc`, or
  `~/.config/fish/config.fish`). Open a new shell or `source` the file once to
  pick the changes up.
- **Windows (`install.ps1`)**: generates a `wpm.ps1` completion script under the
  install directory and adds a load line to your PowerShell profile
  (`$PROFILE`). Either run `. $PROFILE` once or open a new PowerShell window.

### Windows: enable script execution

<!-- prettier-ignore -->
> [!IMPORTANT]
> PowerShell blocks profile scripts by default on Windows. If
> completion doesn't work after installing, your execution policy is the most
> likely reason.

Set the policy once per user with:

```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

Then reopen PowerShell. `RemoteSigned` allows local scripts (including your
profile) to run while still requiring signatures on scripts downloaded from the
internet. Microsoft has the full reference at
<https://learn.microsoft.com/powershell/module/microsoft.powershell.security/set-executionpolicy>.

### Manual setup

If you installed wpm a different way (`go install`, source build, Docker without
the installers), wire completion up yourself. wpm can emit the right script for
any supported shell:

```sh
wpm completion <bash|zsh|fish|powershell>
```

One-time setup per shell:

```sh
# bash
wpm completion bash | sudo tee /etc/bash_completion.d/wpm > /dev/null

# zsh (with compinit enabled)
wpm completion zsh > "${fpath[1]}/_wpm"

# fish
wpm completion fish > ~/.config/fish/completions/wpm.fish
```

```powershell
# powershell
wpm completion powershell | Out-File -Encoding utf8 "$PROFILE.d\wpm.ps1"
```

For a one-off in the current session, source the output instead:

```sh
source <(wpm completion bash)
```

The same works for `zsh` and `fish` with the appropriate shell-source syntax.

## Verify the install

After installing, confirm wpm is on your `PATH` and check its version:

```console
$ wpm --version
wpm version v0.1.0 (abc1234)
```

The output shows the release version and the short git commit it was built from.

If you see `command not found`, the install directory is not on your `PATH` yet.
Open a new terminal session (so the shell's startup file is re-sourced), or run
`hash -r` in your current one to refresh the shell's command cache. If the
problem persists, confirm that the install path is in `PATH`:

- Linux/macOS script: `/usr/local/bin` should be in `PATH`.
- Windows PowerShell: `%USERPROFILE%\.wpm\bin` should be in `PATH`.
- `go install`: `$(go env GOPATH)/bin` should be in `PATH`.

## Where wpm keeps its files

| Path                      | Purpose                                          |
| :------------------------ | :----------------------------------------------- |
| `~/.wpm/config.json`      | Client config: auth token, default user.         |
| `~/.wpm/cache/`           | Registry response cache and download cache.      |
| `~/.wpm/cache/install/`   | HTTP cache for install-time GET requests.        |
| Per project `wpm.json`    | Package manifest.                                |
| Per project `wpm.lock`    | Resolved dependency snapshot.                    |
| Per project `wp-content/` | Where dependencies are extracted (configurable). |

The config directory can be moved with the global `--config` flag or the
`WPM_CONFIG` environment variable. To clear the registry response cache, delete
`~/.wpm/cache`; the lockfile and `wp-content/` are untouched.

## Upgrade

Re-run whichever install method you used the first time. The install script and
the PowerShell installer both replace the existing binary in place. For
`go install`, run it again with `@latest`. For source builds, `git pull` and
rebuild.

To upgrade to a specific version, use the version-pinned form of your install
method (shown in each section above).

## Uninstall

The binary is the entire install. Delete it:

| Install method     | Path to remove                   |
| :----------------- | :------------------------------- |
| Linux/macOS script | `/usr/local/bin/wpm`             |
| Windows PowerShell | `%USERPROFILE%\.wpm\bin\wpm.exe` |
| Docker             | `docker rmi trywpm/cli`          |
| `go install`       | `$(go env GOPATH)/bin/wpm`       |
| Source build       | wherever you placed the binary   |

Your project's `wpm.json`, `wpm.lock`, and `.wpmignore` are not touched by
uninstalling wpm.

If you want a clean slate, also remove the config directory:

```sh
rm -rf ~/.wpm
```

That clears your saved auth token, your `defaultUser` cache, and the install
cache under `~/.wpm/cache/install`.

## Next steps

- New to wpm? See [Getting started](../guide/getting-started.md) for a 10-minute
  end-to-end walkthrough.
- Setting up a build pipeline? See the [CI/CD guide](../guide/ci.md).
- Looking for the `wpm.json` reference? See
  [`wpm.json`](../reference/wpm-json.md).
