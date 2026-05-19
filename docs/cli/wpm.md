# wpm

<!---MARKER_GEN_START-->
Package Manager for WordPress ecosystem

### Subcommands

| Name                        | Description                                                        |
|:----------------------------|:-------------------------------------------------------------------|
| [`auth`](auth.md)           | Authenticate with the wpm registry                                 |
| [`init`](init.md)           | Initialize a new WordPress package or init wpm in existing project |
| [`install`](install.md)     | Install project dependencies and add new packages                  |
| [`ls`](ls.md)               | List installed dependencies                                        |
| [`outdated`](outdated.md)   | Check for outdated dependencies                                    |
| [`publish`](publish.md)     | Publish a package to the wpm registry                              |
| [`uninstall`](uninstall.md) | Remove dependencies from the project                               |
| [`whoami`](whoami.md)       | Display the current user                                           |
| [`why`](why.md)             | Show why a package is installed                                    |


### Options

| Name                | Type     | Default                  | Description                                                       |
|:--------------------|:---------|:-------------------------|:------------------------------------------------------------------|
| `--config`          | `string` | `/home/thelovekesh/.wpm` | Location of client config files                                   |
| `-D`, `--debug`     | `bool`   |                          | Enable debug mode                                                 |
| `-l`, `--log-level` | `string` | `info`                   | Set the logging level ("debug", "info", "warn", "error", "fatal") |
| `--registry`        | `string` | `registry.wpm.so`        | Set specific registry to use                                      |


<!---MARKER_GEN_END-->

