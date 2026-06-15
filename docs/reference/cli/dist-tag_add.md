# wpm dist-tag add

<!-- prettier-ignore-start -->
<!---MARKER_GEN_START-->
Point a dist tag at a package version


<!---MARKER_GEN_END-->
<!-- prettier-ignore-end -->

## Description

Point a distribution tag at an already-published version of a package.

The spec is `<pkg>@<version>`, where the version must be a concrete semantic
version that already exists in the registry and the tag always resolves to an
exact release, never to another tag. If you omit the tag, it defaults to
`latest`.

You must be logged in (`wpm auth login`) or have `WPM_TOKEN` set, and you need
write access to the package. On success wpm prints a one-line summary:

```console
$ wpm dist-tag add acme-blocks@1.4.0 beta
+beta: acme-blocks@1.4.0
```

Moving an existing tag is the same operation as creating one: re-run `add` with
a different version and the tag is re-pointed.

## Examples

### Tag a version as `latest`

```console
$ wpm dist-tag add acme-blocks@1.4.0
+latest: acme-blocks@1.4.0
```

### Create a pre-release tag

```console
$ wpm dist-tag add acme-blocks@2.0.0-beta.1 beta
+beta: acme-blocks@2.0.0-beta.1
```
