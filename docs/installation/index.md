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

## Verify the install

After installing, confirm wpm is on your `PATH` and check its version:

```console
$ wpm --version
wpm version v0.1.0 (abc1234)
```

The output shows the release version and the short git commit it was built from.
If you see "command not found", the install directory is not on your `PATH` yet.
Check your shell's startup file (`~/.bashrc`, `~/.zshrc`, etc.).

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

- New to wpm? See [Getting started](../getting-started/index.md) for a 10-minute
  end-to-end walkthrough.
- Setting up a build pipeline? See the [CI/CD guide](../guides/ci.md).
- Looking for the `wpm.json` reference? See [`wpm.json`](../wpm-json/index.md).
