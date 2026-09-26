# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

precept is a Go module of `go/analysis` analyzers driven by rules a project
declares in its configuration. It ships one analyzer, `paircheck`: a call that
opens an obligation on a value (a trigger) must be followed by a call that
discharges it (a satisfier) on every path that returns. A second analyzer,
`defcheck` (forbidden patterns in declaration names), is specified but not
implemented.

The project's rules are in `.claude/rules/` and are loaded with this file. The
PRDs are in `.local/docs/prds/`, in Portuguese and outside version control:
`ordering-obligations.md` (paircheck), `declaration-naming-policy.md`
(defcheck) and `golangci-lint.md` (what native inclusion in golangci-lint
requires). Handoff reports of earlier sessions are in `.claude/sessions/`.

## Commands

The gate, which exits 0 or the work is not done:

```sh
make lint   # go mod tidy -diff, golangci-lint config verify, golangci-lint run, and the unused run without tests
make test   # go test -race -count=1 ./...
```

- `make format` rewrites what gofmt and golines (100 columns, a tab counting
  two) would change.
- `make e2e` builds golangci-lint with the plugins (`make golangci`, which needs
  git and the network) and runs the e2e suite with the `golangci` build tag.
- One test: `go test -count=1 ./paircheck/ -run 'TestFlow$'`, or
  `go test -count=1 ./e2e/ -run 'TestCommandReports$'`. Keep `-count=1` for
  `e2e/`: the test cache cannot see `cmd/paircheck`, which the suite builds, nor
  the golangci-lint binary it runs.
- The command from a checkout: `go run ./cmd/paircheck -c rules.yml ./...`, and
  `go run ./cmd/paircheck validate -c rules.yml`.

## Architecture

### `paircheck/`, the analyzer

1. **Configuration.** `config.go` decodes `Config`, `Rule` and `Call`, and
   compiles each rule into a `protocol` (`protocol.go`). `New(Config)` refuses
   an invalid configuration before any analysis. A value written as text or as
   an object implements both `UnmarshalText` and `UnmarshalJSON`, so it decodes
   both through golangci-lint's native settings (mapstructure) and through the
   module plugin (JSON).
2. **Binding, per pass.** `bind.go` resolves each protocol against the packages
   the analyzed package can see:
   - it first resolves a name relative to the module (`./internal/x`, `.` for
     the root package) against `pass.Module`, which golangci-lint fills and the
     command loads with `packages.NeedModule` (`name.go`, `protocol.go`);
   - it finds the trigger and the satisfiers by qualified name (`name.go`);
   - it deduces by type the slot that carries the value (receiver, argument or
     result), leaving `context.Context` out of the deduction;
   - type parameters link only through slots written on both sides
     (`correspondence.go`).

   A pass that sees every function of a rule and cannot bind it returns an
   error, which fails the whole run (in golangci-lint, every `go/analysis`
   linter of it). A pass that sees only some of them skips the rule.
   `validate.go` runs the same resolution strictly over a separate load, for
   the `validate` subcommand.
3. **Search.** `flow.go` starts at each trigger call and walks the SSA blocks
   depth first. The state is small and finite (open and deferred counts
   saturated at 2, an uncertain flag and a displaced-error flag, and under
   `on-success` a called flag and the error value and variable the path knows
   to hold a failure), so the search always terminates. Instructions become
   events, and a path leaks at a `return` (`leakAtExit`), under `defer-first`
   at a call made before the deferred satisfier (`leakBeforeDefer`), or under
   `on-success` at a return that hands back no failure with no satisfier called
   on the path (`leakOnSuccess`). A `panic` or a call that does not return
   abandons the path.
4. **The rest of the search:**
   - `failure.go`: the error the trigger returned, and the branches that check
     it;
   - `success.go`: under `on-success`, whether the error a return hands back is
     a failure;
   - `identity.go`: access paths and a three-valued sameness;
   - `closure.go`: deferred closures, as `deferred-closure` says;
   - `transfer.go`: the exits by return, store and argument;
   - `slot.go`: the value a call carries in a slot, including the receiver a
     method value binds and the callee for the `call` satisfier;
   - `diagnostic.go`: the message and the source expression of the value.

The design prefers a false negative to a false positive: an unknown identity
discharges, and an unrecognized check leaves the path uncertain and unreported.
golangci-lint accepts linters, not detectors.

### `cmd/paircheck/`, the command

- GNU-style flags through pflag; `validate` is the first positional argument.
- It loads the packages with their tests and analyzes what golangci-lint
  analyzes (`analyzed`): the test variant in place of its package, and no
  generated test main. `--tests` defaults to `run.tests` when `-c` names a
  `.golangci.yml`, and a `--tests` written on the command line wins, as in
  golangci-lint.
- The rules come from the command's own YAML (`rules:` at the top), or from a
  `.golangci.yml`, as native settings (`linters.settings.paircheck`) or as
  plugin settings (`linters.settings.custom.paircheck.settings`).
- It runs `checker.Analyze` over `packages.Load` and prints each distinct error
  once. Exit codes: 0 clean, 3 with diagnostics, 1 on failure.
- `validate` resolves relative names against the module of the current
  directory, read from the `go.mod` that `go env GOMOD` names.

### `plugin/`

`register.Plugin("paircheck", …)` in the module's only `init()`.
`plugin/.custom-gcl.yml` builds `golangci-lint-precept` locally. It must never
sit at the repository root: golangci-lint-action builds any root
`.custom-gcl.yml` in place of golangci-lint.

### Tests

- **Unit tests** use analysistest over the GOPATH-style
  `paircheck/testdata/src/<package>`. `resource` is the stand-in API the other
  testdata packages import. Rules are built with `rule(id, trigger,
  satisfiers...)` and `build(t, rules...)`. A case that expects the analysis to
  fail passes `recorder`, an `analysistest.Testing`, in place of `t`.
- **End to end:** `e2e/testdata/project` is a module with only the standard
  library, whose `// want` comments say what each rule of
  `e2e/testdata/rules.yml` reports. The same expectations are checked for:
  - the built command;
  - the plugin, run in process with settings shaped as golangci-lint hands
    them, over the packages golangci-lint analyzes (`analyzed`, the same
    filter as the command's);
  - behind the `golangci` build tag, golangci-lint with the plugin, whose path
    comes from `PRECEPT_GOLANGCI_LINT`.

  `e2e/testdata/refused/*.yml` must fail with the error in the test's
  `refusals` table.

## Pins that move together

- **golangci-lint's version** is written in five places:
  - `plugin/.custom-gcl.yml`;
  - both golangci-lint-action steps in `.github/workflows/ci.yml`;
  - the `.custom-gcl.yml` example in `README.md`;
  - `golangci_commit` in the `Makefile`, the commit the tag must name, or
    `make golangci` refuses to build.

  Dependabot updates none of them.
- **Dependency versions:** `golang.org/x/tools`, `golang.org/x/mod`,
  `plugin-module-register`, `go.yaml.in/yaml/v3` and `pflag` must not be newer
  than the ones the pinned golangci-lint uses.
- **The CI lint job's Go:** it builds golangci-lint with the latest stable Go,
  because gofmt and golines format as the Go they are compiled with.

## Adding a rule key

`Rule` field with its `json` tag (`config.go`) → `protocol` field and `compile`
→ where the search or the binding reads it; the README keys table; both example
files, which write every key at its default and are validated by `e2e`; the
PRD §9 keys block; an `e2e` rule and case. `encoding/json` matches keys
case-insensitively, so only a hyphenated key (`defer-first`) can fail a decode
test before its tag exists.

## Gotchas

- Flags are `-c`/`--config`, `--tests` and `-v`/`--version` only: under
  pflag, `-config rules.yml` parses as `-c onfig`, with `rules.yml` as a
  package, and `--tests false` takes `false` as a package.
- golangci-lint looks for `.golangci.{yml,yaml,toml,json}` from the directory of
  its first package argument: a configuration anywhere in the tree is named
  otherwise (`golangci.example.yml`).
- Never move or recreate a pushed tag: Go keeps the old content in the download
  cache, the VCS clone under `$(go env GOMODCACHE)/cache/vcs` and the module
  index in GOCACHE; recovering needed `go clean -cache`.
- `GOPRIVATE=github.com/pragmabits/precept go install …/cmd/paircheck@<tag>`
  fetches from GitHub without the proxy or sum.golang.org. A request for a
  version to proxy.golang.org caches it for good; whether it already has one is
  read from the `index.golang.org` feed, which fetches nothing.
- A checkout build's `--version` ends in `+dirty` while any untracked file that
  is not ignored sits in the tree.
