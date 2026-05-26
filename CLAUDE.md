# wpm

Go package manager for WordPress plugins and themes. Cobra-based CLI, binary
name `wpm`. See `@README.md` for the user-facing overview.

**This is a Go project.** `package.json`, `bun.lock`, and `node_modules/` exist
only to vendor `oxfmt` for Markdown/JSON/YAML formatting. Never propose
JavaScript for runtime code, and never add a Node dep for anything but
formatting.

## Commands

- Build: `go build -o build/wpm ./cmd/wpm` (or `./scripts/build/binary`)
- Run without building: `go run ./cmd/wpm <command>`
- Format Go: `golangci-lint fmt` — **never** call `go fmt`, `gofmt`, or
  `gofumpt` directly. Project `gci` order, `gofumpt` rules, and
  `interface{} → any` rewrite all flow through this command.
- Lint Go: `golangci-lint run`
- Format non-Go: `bunx format`
- Test single package: `go test ./pkg/pm/installer/...`
- Test with race detector (required for `pkg/pm/installer` or
  `pkg/pm/resolution` work): `go test -race ./...`
- Regenerate CLI reference docs: `./scripts/docs/generate-md`

## Layout

Three-layer separation — do not collapse layers for convenience:

- `cmd/wpm/` — entrypoint and signal handling
- `cmd/docgen/` — regenerates marker regions in `docs/cli/*.md`
- `cli/` — arg parsing, output, error→exit-code mapping. Commands registered in
  `cli/command/commands/commands.go`. **The `cli/` layer must not contain
  package-manager logic** — it builds an options struct and delegates to `pkg/`.
- `pkg/pm/{wpmjson,wpmlock,wpmignore,registry,resolution,installer,workspace,signatures}`
  — the package-manager engine
- `pkg/api/` — registry HTTP client and cache
- `pkg/archive/` — tar/zip extraction (handles untrusted content)
- `pkg/output/`, `pkg/progress/`, `pkg/streams/` — CLI UI primitives
- `pkg/{config,version,wp,jsonpretty,asciisanitizer,unsafeconv}` — utilities

## Project conventions

- **No `fmt.Printf`/`Println` in `pkg/`.** Return values or use `pkg/output` /
  `pkg/progress` so the CLI can honor `--verbose` and future JSON output.
  `forbidigo` enforces.
- **Errors:** never `panic()` outside `main`. Wrap with `%w`. Inspect with
  `errors.Is` / `errors.As` — never string-match error messages.
- **Concurrency:** use `golang.org/x/sync/errgroup`. Always honor
  `context.Context` cancellation.
- **Logging:** use `github.com/rs/zerolog` (via the package-level `log`
  global), not stdlib `log` (depguard enforces). Stdlib
  `io/ioutil` is denied — use `os` / `io`.
- **CLI reference docs** (`docs/cli/*.md`) have an auto-generated marker block
  (`<!---MARKER_GEN_START-->` / `<!---MARKER_GEN_END-->`). **Never edit inside
  the markers.** Only `## Description` and `## Examples` are read by the
  generator — don't add other top-level `##` headings. Full conventions and
  workflow: `@docs/README.md`.

## Testing

- **Append tests to the existing `_test.go`** for the code you're changing.
  Create a new test file only when adding a new source file with no natural test
  home.
- **Verify the test actually tests your change.** A passing test that would also
  pass on `origin/main` isn't testing your fix:

  ```sh
  git stash
  go test -run TestName ./path/to/pkg   # should FAIL
  git stash pop
  go test -run TestName ./path/to/pkg   # should PASS
  ```

  If it passes in both states, rewrite the test.

- **Never use `time.Sleep` to wait for a condition.** Use channels,
  `sync.WaitGroup`, `context` deadlines, or bounded polling against the actual
  condition.
- Mock the registry with `net/http/httptest`. Existing patterns: `pkg/api/`,
  `pkg/pm/registry/`.
- Use `t.TempDir()`, `t.Setenv()`, `t.Helper()` (lint-enforced).
- For CLI output assertions, drive through `cli/command.Cli` and capture `Out()`
  / `Err()`. Don't touch global state.

## Git & remote operations

- **Explicit ask → do it.** "Commit", "push", "open a PR" — perform the
  operation and report what was done. Don't re-ask.
- **Ambiguous ask → confirm first.** If changes were described but no
  push/PR/tag word appeared, confirm before any remote-write operation. Local
  commits on a working branch don't need confirmation.
- **Never push directly to `main`.** Always open a PR.
- **Never bypass safety controls**: no `--no-verify`, no force-push to shared
  branches, no skipping signature verification.
- Branch names: any reasonable name. Attribution belongs in the PR body.

## Before referencing a symbol

Don't assume functions, interfaces, fields, methods, flags, or config keys
exist. Search the repo or read the file first. Common traps:

- CLI flags → `cli/flags/` and `cli/command/<name>/`
- `wpm.json` fields → `pkg/pm/wpmjson/types/types.go`
- Command names / aliases → `cli/command/commands/commands.go`
- `Cli` interface methods → `cli/command/cli.go`
- Issue / PR numbers (in tests, comments, commits) — must be real, never
  placeholders

## Deeper context — fetch when relevant

Project documentation is hosted at <https://wpm.so/docs>. Do not preload these;
fetch on demand when the task touches the area.

- **Discover what's available**: <https://wpm.so/docs/sitemap.xml>
- **Read a page as markdown** — transform the URL:

  ```
  page:  https://wpm.so/docs/fundamentals/dependencies
  md:    https://wpm.so/docs/llms.mdx/docs/fundamentals/dependencies/content.md
  ```

Typical triggers: `wpm.json` schema or scripts work → fundamentals docs;
registry/auth/tokens → registry docs; resolution or lockfile semantics →
resolution docs; signature verification → signatures docs.

## Gotchas

1. **Project root**: don't assume `os.Getwd()` is the `wpm.json` root in nested
   package logic. Walk up, or accept the root as a parameter.
2. **`wpm.json` field changes** require all of: update
   `pkg/pm/wpmjson/types/types.go` (and `validator/` if constrained), update
   `README.md` and `docs/`, add a manifest test case.
3. **Untrusted archives**: `pkg/archive` and `pkg/pm/installer` extract registry
   content. Zip Slip is a real risk — reuse the existing extraction helpers,
   don't reimplement.
