# wpm

`wpm` is a package manager designed to manage WordPress plugins and themes as packages, similar to how npm works for Node.js or Composer for PHP.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
    - [Common Commands](#common-commands)
- [Configuration with wpm.json](#configuration-with-wpmjson)
    - [Required Fields](#required-fields)
    - [Optional Fields](#optional-fields)
- [Excluding Files from Publishing](#excluding-files-from-publishing)
- [Documentation](#documentation)
- [Support](#support)
- [License](#license)

## Overview

`wpm` provides a structured way to manage WordPress plugins and themes. It uses a `wpm.json` file to define package metadata, including dependencies, versioning, and other relevant information. The tool interacts with a remote registry (currently in a development/conceptual stage at `registry.wpm.so`) to publish and retrieve packages.

## Installation

**Linux and Mac**
```
curl -fsSL https://wpm.so/install | bash
```

**Windows**
```
powershell -c "irm wpm.so/install.ps1|iex"
```

**Docker**
```
docker pull trywpm/cli
```

**Build from Source**
```bash
git clone git@github.com:trywpm/cli.git wpm
cd wpm
go build -o wpm ./cmd/wpm
```

or download the binaries from the [release](https://github.com/trywpm/cli/releases) page.

## Quick Start

1. **Initialize a new package:**
   ```bash
   wpm init
   ```
   This will create a `wpm.json` file in your project.

2. **Install dependencies:**
   ```bash
   wpm install
   ```

3. **Publish your package:**
   ```bash
   wpm publish
   ```

## Usage

```bash
wpm [OPTIONS] COMMAND
```

### Common Commands

* `auth`: Authenticate with the wpm registry
  * `login`: Log in to the registry
  * `logout`: Log out from the registry

* `init`: Initialize a new WordPress package
  * Use `-y` or `--yes` to accept all defaults

* `install`: Install dependencies from `wpm.json`
  * `--no-dev`: Skip installing dev dependencies
  * `--ignore-scripts`: Do not run lifecycle scripts
  * `--dry-run`: Simulate installation without making changes
  * `--save-dev`: Save installed packages as dev dependencies
  * `--save-prod`: Save installed packages as production dependencies (default)
  * `--network-concurrency`: Set number of concurrent network requests (default 16)

* `publish`: Publish a package to the registry
  * `--dry-run`: Validate without publishing
  * `--tag`: Set the package tag (default: latest)
  * `--access`: Set access level (public/private)
  * `--verbose`: Show detailed output

* `whoami`: Display the current logged-in user

### Global Options

* `--config`: Location of client config files (default: `~/.wpm`)
* `-D, --debug`: Enable debug mode
* `-l, --log-level`: Set logging level (`debug`, `info`, `warn`, `error`, `fatal`)
* `-v, --version`: Print version information
* `-h, --help`: Show help

Run `wpm COMMAND --help` for more information about a specific command.

## Configuration with wpm.json

The `wpm.json` file defines your package and its dependencies:

```json
{
  "name": "my-awesome-plugin",
  "description": "A short description of my plugin",
  "type": "plugin",
  "version": "1.0.0",
  "license": "GPL-2.0-or-later",
  "requires": {
    "wp": ">=6.0",
    "php": ">=7.4"
  },
  "dependencies": {
    "akismet": "*", // always fetch latest version
    "hello-dolly": "1.7.2"
  },
  "devDependencies": {
    "some-dev-plugin": "3.20.2"
  },
  "config": {
    "bin-dir": "wp-bin",
    "content-dir": "wp-content",
    "runtime": {
      "wp": "6.9",
      "php": "8.2"
    }
  }
}
```

### Required Fields

* `name`: Package name (lowercase, alphanumeric, hyphens)
* `type`: Either `plugin` or `theme`
* `version`: SemVer compatible version

### Optional Fields

* `description`: Brief package description
* `private`: Set `true` to prevent accidental publishing
* `license`: License identifier
* `homepage`: URL to your package's homepage
* `tags`: Keywords (maximum 5)
* `dependencies`: Production dependencies
* `devDependencies`: Development-only dependencies
* `requires`: Minimum requirements which the package supports
* `config`: Custom configuration options

## Configuration Options
* `bin-dir`: Directory for executable files (default: `wp-bin`)
* `content-dir`: WordPress content directory (default: `wp-content`)
* `runtime`: Runtime environment versions this project is geared to run on
* `runtime.wp`: WordPress version (e.g., `6.7`, `6.8`, `6.9`)
* `runtime.php`: PHP version (e.g., `7.4`, `8.0`, `8.1`, `8.2`)

## Excluding Files from Publishing

Create a `.wpmignore` file in your project root to exclude files when publishing:

```
node_modules/
.git/
.github/
*.zip
*.log
```

## Documentation

*Documentation will be available soon on the docs.wpm.so site. For now, you can refer to the command line help for detailed usage instructions.*

## Support

* GitHub: [https://github.com/trywpm/cli/discussions](https://github.com/trywpm/cli/discussions)
* Twitter: [@thelovekesh](https://twitter.com/thelovekesh)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
