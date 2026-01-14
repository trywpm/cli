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

*Coming soon - prebuilt binaries for various platforms will be available via a install script.*

For now, you can build from source:
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

* `install`: (WIP) Install dependencies from `wpm.json`

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
  "dependencies": {
    "wp": ">=6.0"
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
