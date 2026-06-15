# wpm dist-tag

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Manage package distribution tags

### Aliases

`wpm dist-tag`, `wpm dist-tags`

### Subcommands

| Name                     | Description                           |
|:-------------------------|:--------------------------------------|
| [`add`](dist-tag_add.md) | Point a dist tag at a package version |



<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

`wpm dist-tag` groups the subcommands that manage a package's distribution tags.

A distribution tag is a human-friendly label, such as `latest` or `beta`, that
points at a specific published version. Tags give consumers a stable name to
install against instead of pinning an exact version: `wpm install acme-blocks`
resolves through the `latest` tag to whatever version it currently points at.

The `latest` tag is special and it is what `wpm install <pkg>` uses when no
version or tag is requested. Any other tag (`beta`, `next`, `canary`, …) is a
convention you define for your own release workflow.
