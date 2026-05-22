---
title: Running wpm in CI
description:
  Run wpm in GitHub Actions, GitLab CI, and shell runners with secure token
  handling and reproducible installs.
---

This guide covers how to run wpm in continuous integration pipelines: GitHub
Actions, GitLab CI, and generic shell-based runners. The patterns apply equally
to other systems.

## The shape of a wpm CI step

Every wpm CI step boils down to the same four lines:

1. Install wpm (or use a container that ships it).
2. Make a token available as `WPM_TOKEN`.
3. Run `wpm install` (or the command you actually want).
4. Make sure logs stay clean by setting `CI=true` and `NO_COLOR=1`.

The key insight: you usually don't need `wpm auth login` in CI. Just set
`WPM_TOKEN` and wpm picks it up. CI runners start fresh each time with no token
on disk, so the environment variable always wins.

## Token storage

<!-- prettier-ignore -->
> [!WARNING]
> Never commit `WPM_TOKEN` to version control, paste it into chat or
> issue trackers, or echo it in build logs. CI secrets managers exist for
> exactly this; use them.

Treat your wpm token like a deployment key. Store it in your CI system's secret
manager (GitHub Actions secrets, GitLab CI variables, HashiCorp Vault, your CI's
equivalent), inject it into the job's environment as `WPM_TOKEN`, and never echo
it to the log.

`wpm auth login` won't read your input in a non-interactive session unless you
pass `--token`. The cleaner approach in CI: skip `auth login` entirely and let
`WPM_TOKEN` do the work.

If you do need `auth login` (say, to populate `defaultUser`), use
`--token "$WPM_TOKEN"`. wpm will print a security warning, but CI runners are
ephemeral, so shell-history leakage isn't a real concern.

## Output and logging

By default wpm prints a spinner and ANSI color codes. Both are noisy in CI logs.
Two environment variables clean things up:

- `CI=true` turns off the spinner. wpm checks for `CI` directly and skips the
  progress indicator when it sees it.
- `NO_COLOR=1` strips ANSI color escapes from all output. Useful when your log
  viewer doesn't render them.

With both set, the output is plain text that grep, awk, and log search tools can
handle.

## Reproducible installs

Three rules for reproducible installs in CI:

- Commit `wpm.lock` to version control.
- Pin the wpm version your CI uses, either by installing a tagged release or by
  using a versioned Docker image.
- Don't run commands that modify `wpm.json` (`wpm install <pkg>`,
  `wpm uninstall`) in the same job that performs the build. If you do, the
  lockfile and the manifest will diverge and the next developer to run
  `wpm install` locally will see noisy changes.

For a deploy-time install, `wpm install --no-dev` is the right choice. It skips
dev-only packages and removes them from disk if they're there.

## GitHub Actions

A typical workflow for a wpm-backed project:

```yaml
name: build

on:
  push:
    branches: [main]
  pull_request:

jobs:
  install:
    runs-on: ubuntu-latest
    env:
      CI: "true"
      NO_COLOR: "1"
      WPM_TOKEN: ${{ secrets.WPM_TOKEN }}
    steps:
      - uses: actions/checkout@v4

      - name: Install wpm
        run: |
          curl -fsSL https://wpm.so/install | bash
          wpm --version

      - name: Install dependencies
        run: wpm install --no-dev
```

A few notes:

- `secrets.WPM_TOKEN` is set in the repository's secret manager (Settings →
  Secrets and variables → Actions). It's exposed only to the workflow that
  references it.
- Pin the wpm version by passing it to the install script if you want hermetic
  builds:
  ```sh
  curl -fsSL https://wpm.so/install | bash -s -- 0.1.0
  ```
- For matrix builds, the `env` block applies to every matrix combination.

To publish a release on a tag push:

```yaml
publish:
  runs-on: ubuntu-latest
  if: startsWith(github.ref, 'refs/tags/v')
  env:
    CI: "true"
    NO_COLOR: "1"
    WPM_TOKEN: ${{ secrets.WPM_TOKEN }}
  steps:
    - uses: actions/checkout@v4
    - run: curl -fsSL https://wpm.so/install | bash
    - run: wpm publish --access public
```

## GitLab CI

```yaml
variables:
  CI: "true"
  NO_COLOR: "1"

install:
  stage: build
  image: ubuntu:24.04
  before_script:
    - apt-get update && apt-get install -y curl ca-certificates
    - curl -fsSL https://wpm.so/install | bash
  script:
    - wpm install --no-dev
  rules:
    - if: $CI_COMMIT_BRANCH

publish:
  stage: deploy
  image: ubuntu:24.04
  before_script:
    - apt-get update && apt-get install -y curl ca-certificates
    - curl -fsSL https://wpm.so/install | bash
  script:
    - wpm publish --access public
  rules:
    - if: $CI_COMMIT_TAG
```

Define `WPM_TOKEN` as a masked, protected variable in _Settings → CI/CD →
Variables_. Mark it masked so it never appears in logs.

If you prefer the official Docker image to a fresh install, swap the `image:`
and drop `before_script`:

```yaml
install:
  image: trywpm/cli:latest
  variables:
    CI: "true"
    NO_COLOR: "1"
  script:
    - wpm install --no-dev
```

## Generic shell runners

For anything that runs a shell (Jenkins, CircleCI, self-hosted agents), the bare
pattern is the same:

```sh
#!/usr/bin/env bash
set -euo pipefail

export CI=true
export NO_COLOR=1
export WPM_TOKEN="${WPM_TOKEN:?WPM_TOKEN must be set}"

curl -fsSL https://wpm.so/install | bash
wpm install --no-dev
```

The `${WPM_TOKEN:?...}` syntax aborts immediately if the variable is missing,
with a clear error in the log.

## Docker-based pipelines

If your build container is `trywpm/cli`, wpm is already inside. Just run it:

```dockerfile
FROM trywpm/cli AS deps
WORKDIR /work
COPY wpm.json wpm.lock ./
RUN --mount=type=secret,id=wpm_token \
    WPM_TOKEN=$(cat /run/secrets/wpm_token) \
    CI=true \
    NO_COLOR=1 \
    wpm install --no-dev
```

The `--mount=type=secret` form keeps `WPM_TOKEN` out of the image's layer
history. Build with:

```sh
docker build --secret id=wpm_token,env=WPM_TOKEN .
```

## Caching strategies

wpm maintains an HTTP cache under `~/.wpm/cache/install` for registry GET
requests. Caching this directory between CI runs can speed up manifest
resolution.

GitHub Actions example:

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.wpm/cache
    key: wpm-${{ runner.os }}-${{ hashFiles('wpm.lock') }}
    restore-keys: |
      wpm-${{ runner.os }}-
```

GitLab CI example:

```yaml
cache:
  key:
    files:
      - wpm.lock
  paths:
    - .wpm-cache/

before_script:
  - export WPM_CONFIG="$CI_PROJECT_DIR/.wpm-cache"
```

In the GitLab example we set `WPM_CONFIG` so wpm uses a project-relative
directory that GitLab knows how to cache.

The cache is purely an optimization. Builds remain correct without it; they just
spend more time talking to the registry.

## Identity for multi-tenant setups

If your CI has to publish under more than one identity (different organizations,
different registries), use `--config` to keep their config directories separate:

```sh
wpm --config "$RUNNER_TEMP/.wpm-org-a" --registry registry.org-a.example install
wpm --config "$RUNNER_TEMP/.wpm-org-b" --registry registry.org-b.example install
```

Set `WPM_TOKEN` from a different secret for each block. Because `WPM_TOKEN` is
only read when the local config lacks a token, the isolation is automatic.

## Checklist

Before merging your CI configuration, confirm:

- [ ] `WPM_TOKEN` is stored as a secret, not in plain text.
- [ ] `CI=true` and `NO_COLOR=1` are set on the job.
- [ ] `wpm.lock` is committed and not produced inside the job.
- [ ] The wpm version is pinned, either by version string or by a tagged Docker
      image.
- [ ] `wpm install` uses `--no-dev` in deploy steps, but not in verification
      steps that need dev tools.

## Related

- [Installation](../getting-started/installation.md): how the install script
  works.
- [Authentication](authentication.md): the token lifecycle.
- [`wpm`](../reference/cli/wpm.md): the full list of global flags and
  environment variables.
- [wpm registry](../fundamentals/registry.md): dist tags and visibility settings
  used by `wpm publish`.
