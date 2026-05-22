# Overview

wpm is a package manager for WordPress plugins and themes. It lets you declare
what your project needs, install it reproducibly, and publish your own plugins
or themes to a registry.

If you've used npm or Composer, the model will feel familiar.

## What you'll work with

Every wpm project revolves around these files:

| File          | Who writes it  | Purpose                                                                            |
| :------------ | :------------- | :--------------------------------------------------------------------------------- |
| `wpm.json`    | You            | Lists your package's name, version, type, dependencies, and runtime needs.         |
| `wpm.lock`    | wpm            | Records the exact versions wpm installed. Commit it to version control.            |
| `wp-content/` | wpm            | Where wpm extracts plugins, themes, and mu-plugins. WordPress reads from here too. |
| `.wpmignore`  | You (optional) | Lists files `wpm publish` should leave out of the published tarball.               |

A fifth file, `~/.wpm/config.json`, lives outside your project. wpm writes your
auth token there after `wpm auth login`.

## The loop

The basic workflow looks like this:

1. **Declare** what you need in `wpm.json`.
2. **Install** with `wpm install`. wpm fetches the packages from the registry,
   extracts them into `wp-content/`, and records the exact versions in
   `wpm.lock`.
3. **Commit** both `wpm.json` and `wpm.lock` to version control. Anyone else who
   clones your project and runs `wpm install` gets the same set of files.
4. **Publish** your own plugins or themes with `wpm publish` when you're ready
   to share.

## Where to go next

- **[Install wpm](installation.md)** on your machine.
- **[First project](first-project.md)**: a ten-minute walkthrough from empty
  directory to a working project with dependencies.
- **[`wpm.json` reference](../fundamentals/wpm-json.md)**: the full manifest
  schema.
- **[CLI reference](../reference/cli/wpm.md)**: every command and flag.
