# Package types

Every wpm package has a `type` field. It tells wpm how to install the package,
and it tells WordPress how to load it. Three values are supported:

| Type        | WordPress directory      | Loaded by WordPress                                               |
| :---------- | :----------------------- | :---------------------------------------------------------------- |
| `plugin`    | `wp-content/plugins/`    | Only when activated.                                              |
| `theme`     | `wp-content/themes/`     | Only when set as the active theme (or as a child theme's parent). |
| `mu-plugin` | `wp-content/mu-plugins/` | Always, automatically, on every request.                          |

The `type` field is required. There is no automatic fallback if you omit it.

## `plugin`

A standard WordPress plugin. Lives in `wp-content/plugins/<name>/` and is loaded
by WordPress when an administrator activates it. Plugins are the right choice
for most things you'd publish: an integration with an external service, a
feature you want administrators to opt into, an admin UI addition.

A `plugin` package usually has a "main plugin file" with standard plugin headers
(`Plugin Name:`, `Version:`, `Description:`, and so on). `wpm init --existing`
looks for this file automatically.

## `theme`

A WordPress theme. Lives in `wp-content/themes/<name>/` and is loaded only when
an administrator selects it as the active theme. Themes are detected by the
presence of `style.css` at the project root; the headers in that file
(`Theme Name:`, `Version:`, `Author:`, `Theme URI:`, and so on) carry the
theme's metadata.

Use this type for visual templates, child themes, and block themes.

## `mu-plugin`

A "must-use" plugin. Lives in `wp-content/mu-plugins/<name>/` and is loaded by
WordPress on every request, before any regular plugin. You cannot deactivate an
mu-plugin from the admin UI; the only way to disable it is to remove its files.

Use this type sparingly. Common cases:

- Hosting-side modifications that must always be present.
- Bootstrapping code that other plugins depend on.
- Internal customizations that should never be deactivated.

If you're not sure whether you need a `plugin` or an `mu-plugin`, start with a
`plugin`. mu-plugins have no off-switch in the admin UI, so removing them later
means deleting the files directly.

## How wpm chooses where to extract

When `wpm install` extracts a dependency, it reads the `type` from that
dependency's manifest and picks the matching subdirectory inside your project's
content directory (default `wp-content/`):

| Dependency `type` | Extracted into                  |
| :---------------- | :------------------------------ |
| `plugin`          | `wp-content/plugins/<name>/`    |
| `theme`           | `wp-content/themes/<name>/`     |
| `mu-plugin`       | `wp-content/mu-plugins/<name>/` |

wpm uses each dependency's _own_ type to pick the path, not your project's type.
A theme can depend on a plugin; that plugin still extracts into
`wp-content/plugins/`.

You can change the content directory itself with `config.content-dir` in
`wpm.json`. The subdirectory choice (`plugins/`, `themes/`, `mu-plugins/`) is
hard-coded and matches WordPress's expectations.

## How `wpm init --existing` detects the type

When you adopt wpm in an existing project, the detection runs on your file tree:

1. If `style.css` exists at the project root, the type is `theme`.
2. Otherwise the type is `plugin`.

There is no automatic detection for `mu-plugin`. If you're adopting wpm for an
existing must-use plugin, override the detection explicitly:

```console
$ wpm init --existing --type mu-plugin
```

The override applies to both the type field in `wpm.json` and the header parsing
rules wpm uses.

## Choosing a type for a new project

| You're building...                                | Use                                        |
| :------------------------------------------------ | :----------------------------------------- |
| Most reusable, opt-in functionality               | `plugin`                                   |
| A visual template or block theme                  | `theme`                                    |
| A child theme of an existing theme                | `theme`                                    |
| Hosting-baked customizations that must always run | `mu-plugin`                                |
| Code that bootstraps other plugins                | `mu-plugin`                                |
| A library that has no WordPress entry points      | (Not a fit; wpm is for WordPress packages) |

When in doubt, start with `plugin`. You can publish a new version with a
different `type` later, but anyone who installed the old type keeps it until
they re-install.

## Naming conventions

Package names follow the same rules regardless of type: 3 to 164 lowercase
characters, hyphens allowed, no underscores or uppercase letters (see the
[`wpm.json` reference](../reference/wpm-json.md) for the full regex). A few
practical conventions:

- Plugins are usually named after what they do (`woocommerce-stripe-gateway`,
  `query-monitor`, `redirection`).
- Themes are usually named with the theme's display name in hyphen-case
  (`twenty-twenty-four`, `astra`, `divi`).
- Must-use plugins often carry a host or org prefix to make their ownership
  obvious (`acme-hosting-cache`, `acme-saml-auth`).

The registry doesn't enforce these patterns, but consistent naming makes search
and discovery work better.

## Related

- [`wpm.json` reference](../reference/wpm-json.md): the schema where `type` is
  declared.
- [`wpm init`](../cli/init.md): the detection logic for existing projects.
- [`wpm install`](../cli/install.md): how the extracted path is computed at
  install time.
