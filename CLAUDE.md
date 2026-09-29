# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

precept is a Go module of `go/analysis` analyzers driven by rules a project
declares in its configuration. It ships two:

- `paircheck`: a call that opens an obligation on a value (a trigger) must be
  followed by a call that discharges it (a satisfier) on every path that
  returns;
- `defcheck`: a name a configured pattern forbids, in the kinds of
  declaration the rule chooses, is reported where it is declared.

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
- One test: `go test -count=1 ./paircheck/ -run 'TestFlow$'`,
  `go test -count=1 ./defcheck/ -run 'TestLanguage$'`, or
  `go test -count=1 ./e2e/ -run 'TestCommandReports$'`. Keep `-count=1` for
  `e2e/`: the test cache cannot see the commands, which the suite builds, nor
  the golangci-lint binary it runs.
- The commands from a checkout: `go run ./cmd/paircheck -c rules.yml ./...`,
  `go run ./cmd/paircheck validate -c rules.yml`, and
  `go run ./cmd/defcheck -c rules.yml ./...`.
- Commits are checked by the `commit-msg`, `pre-commit` and
  `prepare-commit-msg` hooks in `.git/hooks/`, which git does not version: a
  new clone installs them with `/git:commit-setup --apply`. The subject is
  `type: description`, with no scope.

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
     command loads with `packages.NeedModule` (`name.go`, `protocol.go`). A
     rule that names anything relative applies only in a module being
     developed, with a path and no version (`developed`): a dependency, the
     standard library or no module leave it out;
   - it applies each rule on an interface to the types of its
     `implementations`, with their methods in place of the interface's
     (`implementation.go`);
   - it finds the trigger and the satisfiers by qualified name (`name.go`);
   - it deduces by type the slot that carries the value (receiver, argument or
     result), leaving `context.Context` out of the deduction;
   - type parameters link only through slots written on both sides
     (`correspondence.go`).

   A pass that sees every function of a rule and cannot bind it returns an
   error, which fails the whole run (in golangci-lint, every `go/analysis`
   linter of it). A pass that sees only some of them skips the rule. Since the
   analyzer exports facts it runs on the dependencies too, so the first pass to
   fail is usually the package declaring the rule's functions.
   `validate.go` runs the same resolution strictly over a separate load, for
   the `validate` subcommand.
3. **Search.** `flow.go` starts at each trigger call and walks the SSA blocks
   depth first. The state is small and finite (open and deferred counts
   saturated at 2, an uncertain flag and a displaced-error flag, and under
   `on-success` a called flag and the set of errors the path knows to be
   failures, among those the function can return), so the search always
   terminates. The set is not compared with the rest: a point already searched
   knowing some failures is searched again only knowing fewer, since knowing
   more only keeps a return from leaking. Instructions become events, and a
   path leaks at a `return` (`leakAtExit`), under `defer-first` at a call made
   before the deferred satisfier (`leakBeforeDefer`), or under `on-success` at
   a return that hands back no failure with no satisfier called on the path,
   nor decided by a closure the path deferred (`leakOnSuccess`). A `panic` or a
   call that does not return abandons the path. `explore` keeps how every path
   ended, for a summary.
4. **The rest of the search:**
   - `failure.go`: the error the trigger returned, and the branches that check
     it;
   - `success.go`: under `on-success`, whether the error a return hands back is
     a failure, and the checks on the path that prove one;
   - `failures.go`: what a pass knows of the errors that are not nil, the
     functions and error variables of the analyzed package included;
   - `identity.go`: access paths and a three-valued sameness, of values and of
     the memory they are stored in;
   - `closure.go`: deferred closures, as `deferred-closure` says, and whether
     one decides how an obligation ends under `on-success`;
   - `summary.go`: what a function does with a value it receives, searched
     from its entry before any other search (`learnSummaries`), and the fact
     that takes it to the packages that import it: a call of one that always
     discharges is a satisfier, one that always keeps it takes nothing along;
   - `transfer.go`: the exits by return, store and argument;
   - `slot.go`: the value a call carries in a slot, including the receiver a
     method value binds, read back from a field or a map entry where the
     function stored it, and the callee for the `call` satisfier;
   - `diagnostic.go`: the message and the source expression of the value.

The design prefers a false negative to a false positive: an unknown identity
discharges, and an unrecognized check leaves the path uncertain and unreported.
golangci-lint accepts linters, not detectors.

### `defcheck/`, the analyzer

- `config.go` decodes `Config`, `Rule` and `Kind`, and compiles each rule into
  a `prohibition` (`prohibition.go`): the pattern through `regexp.Compile`,
  the kinds normalized (none written is every kind, a repeated one counts
  once), and a rule with the pattern, kinds and message of another refused.
  `New(Config)` refuses an invalid configuration before any analysis.
- `defcheck.go` takes every name the package declares from
  `TypesInfo.Defs`, and the variable of a type switch from
  `TypesInfo.Implicits`, once for the switch; sorts them by position, since
  `Defs` is a map and the checker prints in the order of the reports; and
  reports each for each rule that forbids it, in the order of the rules.
- `classify.go` turns a `types.Object` into a `Kind`: a `*types.Var` by
  `Var.Kind()`, a `*types.TypeName` of a `*types.TypeParam` as
  `type-parameter`. A package name, a label and an embedded field have none.
- It has no `Requires` and no fact, and never fails a pass.

### `internal/driver/`, what the commands share

The command of each analyzer is a `driver.Command` and what it does once its
flags are parsed; the rest is here. `command.go` is the command line
(`Command.Run`: the flags, the usage and the version), `driver.go` runs the
analysis (`Analyze`, over the analyzer's constructor and its configuration),
`loading.go` how the packages load (`Loading`, `LoadingOf`, `split`),
`config.go` the reading of the file (`Read`, generic over the configuration,
and `Setup`, what a golangci-lint configuration says of the run),
`generated.go` which files are generated (`Generated`), and `report.go` what
is printed and where (`report`, and `placed`, where golangci-lint places a
finding). An analyzed package that failed because a dependency failed prints
the dependency's error (`causes`), as golangci-lint does, where the checker
gives only its name. `vet.go` is each command as go vet's vet tool
(`VetTool`, `Vet`): the rules by `-config`, an absolute path, read on the first
package, and any error ends the tool with status 1, since go vet keeps the
facts of a run whose error came in its JSON and loses the error on the next.

### `cmd/paircheck/` and `cmd/defcheck/`, the commands

`main.go` of each declares its `driver.Command`, and
`cmd/paircheck/validate.go` is the mode only paircheck has. What follows holds
for both, through the driver. Handed what go vet hands a vet tool, each runs
as one instead (`driver.Vet`).

- GNU-style flags through pflag; for paircheck, `validate` is the first
  positional argument. defcheck has no `validate`: every rule is checked by
  `New`, before any package loads.
- paircheck loads `LoadAllSyntax | NeedModule`, and defcheck `LoadAllSyntax`:
  from export data, as `LoadSyntax` loads, go list compiles each package and
  reports a compile error the type checker then reports again.
- It loads the packages with their tests and analyzes what golangci-lint
  analyzes (`split`): the test variant in place of its package, and no
  generated test main. A test main is one a loaded variant is built for, so a
  main package whose path ends in `.test` is analyzed, where golangci-lint
  drops it. The analyzer itself does not skip the test main: only a driver
  sees which one a variant names (README Limits).
- `Loading` is what the run section of a `.golangci.yml` says about the load,
  read weakly typed as golangci-lint reads it (`LoadingOf`): `run.tests`,
  `run.build-tags` and `run.modules-download-mode`. As in golangci-lint,
  `--tests` and `--modules-download-mode` written on the command line replace
  their key, which is then not read from the file, `--build-tags` adds tags,
  and the load passes them to go as its `makeBuildFlags` does. The keys of the
  file are folded to lower case as viper folds them (`folded`), and two that
  fold alike are refused.
- A finding in a generated file is dropped as golangci-lint drops it:
  `linters.exclusions.generated` of a `.golangci.yml`, read weakly, `strict`
  (`ast.IsGenerated`) when it is not written or empty, `lax` by the markers of
  golangci-lint's matcher, `disable` keeping every finding, and any other
  text `lax`. The file is parsed up to its package clause, as golangci-lint
  parses it, and a list or a map that is not empty is refused. Whatever the
  mode, a finding is printed, and read as generated or not, where
  golangci-lint places it (`placed`, `report.go`): where a `//line` directive
  maps it when that is a Go file, as cgo maps the file it rewrites in the build
  cache to the user's, and otherwise where the code is; in its own file when a
  directive maps the package clause too. A finding outside a Go file is
  dropped. When the file a finding is placed in cannot be read, the command
  warns and drops no finding in a generated file, as golangci-lint does. The
  one difference is a bug of golangci-lint the README Limits state: it looks
  the package clause up by the name the directive maps to, and so moves into a
  file the findings of the Go file of that name.
- The rules come from the command's own YAML (`rules:` at the top), or from a
  `.golangci.yml`, as native settings (`linters.settings.<linter>`) or as
  plugin settings (`linters.settings.custom.<linter>.settings`). A file
  carrying both is refused: golangci-lint applies the plugin's.
- It runs `checker.Analyze` over `packages.Load` and prints each distinct error
  once, in the order of the build: the load errors of the packages as they
  build without their tests, and those of the test variants only once the
  packages load, since a variant holds its package and fails with it. Exit
  codes: 0 clean, 3 with diagnostics, 1 on failure.
- paircheck's `validate` resolves relative names against the module of the
  current directory, read from the `go.mod` that `go env GOMOD` names. It loads
  under the build flags of the analysis, but without the tests, whatever
  `--tests` says: a rule on a function declared in a `_test.go` is refused, a
  limit the README states.

### `plugin/`

`register.Plugin("paircheck", …)` and `register.Plugin("defcheck", …)` in the
module's only `init()`.
`plugin/.custom-gcl.yml` builds `golangci-lint-precept` locally. It must never
sit at the repository root: golangci-lint-action builds any root
`.custom-gcl.yml` in place of golangci-lint.

### Tests

- **Unit tests** use analysistest over the GOPATH-style
  `<analyzer>/testdata/src/<package>`. In paircheck, `resource` is the
  stand-in API the other testdata packages import, rules are built with
  `rule(id, trigger, satisfiers...)` and `build(t, rules...)`, and a case that
  expects the analysis to fail passes `recorder`, an `analysistest.Testing`,
  in place of `t`, and runs over `resource`, which with facts is where a rule
  that does not bind fails. analysistest asks a `// want` of every fact an
  analyzed package exports: only exported functions export one. In defcheck,
  `external` stands for a dependency, and rules are built with
  `build(t, rules...)`.
- **The commands** are tested in process, through `command.Run`, over their own
  `testdata` modules; the behavior of the driver is tested through both:
  `cmd/paircheck/testdata/generated` for generated files, and, in
  `cmd/defcheck/testdata`, `cgo` for a file that imports `C` and `directive`
  for `//line` directives.
- **End to end:** a `linter` is an analyzer as the suite checks it: its name,
  its project, its rules and the configurations it refuses. `linters()` is
  both, for a test that checks each alike. `e2e/testdata/project` (paircheck) and
  `e2e/testdata/defcheck/project` are modules with only the standard library,
  whose `// want` comments say what each rule of `e2e/testdata/rules.yml` and
  `e2e/testdata/defcheck/rules.yml` reports. The same expectations are
  checked for:
  - the built command;
  - the plugin, run in process with settings shaped as golangci-lint hands
    them, over the packages golangci-lint analyzes (`analyzed`, a copy of
    golangci-lint's filter, kept apart from the command's);
  - behind the `golangci` build tag, golangci-lint with the plugin, whose path
    comes from `PRECEPT_GOLANGCI_LINT`;
  - go vet with the built command as its vet tool (`vet_test.go`), which drops
    no generated finding and prints the findings of `template/view.go` where
    its `//line` directive places them, in `view.tmpl`.

  A run meets only some expectations: those of the `_test.go` files with the
  tests loaded, those of `tagged/`, built only under the `precept` tag, with
  that tag, and those of `generated/` as the generated mode keeps them:
  `generated/strict.go` only under `disable`, and `generated/lax.go` but under
  `lax`. `expected(t, tests, tagged, generated)` picks them. The plugin
  harness drops what golangci-lint's default `strict` drops (`generated`, in
  `plugin_test.go`).

  `e2e/testdata/refused/*.yml` must fail with the error in the test's
  `refusals` table, and `e2e/testdata/defcheck/refused/*.yml` with the error
  in `defcheckRefusals`. `e2e/testdata/testmain.yml` holds a rule only the
  generated test main could bind, and must pass silently in all three.

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
- **What the commands copy from golangci-lint**, in `internal/driver`:
  `split` follows its `filterDuplicatePackages` and, but for the test main,
  its `filterTestMainPackages` (`pkg/lint/package.go`); `Loading.Config` its
  `makeBuildFlags`; `testsOf`, `tagsOf`, `modeOf` and `text` the weak decoding
  of mapstructure; `folded` viper's `insensitiviseMap`; `LoadingOf` the
  precedence of viper and of `applyStringSliceHack`; `generatedOf`,
  `laxMarkers` and `Generated.generated` the default of `pkg/config/loader.go`
  and the matcher of
  `pkg/result/processors/exclusion_generated_file_matcher.go`; `placed` its
  `GetFilePositionFor` (`pkg/goanalysis/position.go`) and, but for the lookup
  by name, its `FilenameUnadjuster`; `placedFindings` its `InvalidIssue`,
  which drops a finding outside a Go file; and `report` the warning of
  `processIssues` (`pkg/lint/runner.go`) when a processor fails. Its `Cgo`
  processor is not copied: every file go/packages hands over from the build
  cache is named without `.go`, and is dropped as outside a Go file. The e2e
  harness's `analyzed` copies the package filter as it is, its `generated` the
  strict matcher, and its `placed` `GetFilePositionFor`. When the pin moves, compare them: the e2e
  with the `golangci` tag is what sees a drift.
- **The CI lint job's Go:** it builds golangci-lint with the latest stable Go,
  because gofmt and golines format as the Go they are compiled with.

## Adding a rule key

In paircheck: `Rule` field with its `json` tag (`config.go`) → `protocol`
field and `compile` → where the search or the binding reads it; the README
keys table; both example files, which write every key at its default and are
validated by `e2e`; the PRD §9 keys block; an `e2e` rule and case.

In defcheck: `Rule` field with its `json` tag → `prohibition` field, `compile`
and `key`, which tells a duplicate apart → where `forbids` or `diagnose`
reads it; the README keys table; `defcheck.example.yml` and the defcheck
settings of `golangci.example.yml`; the PRD §7 block; an `e2e` rule and case.
A new kind is a `Kind` constant, its place in `every`, a case of `kindOf`, a
row of the README kinds table and the PRD §6 list. `encoding/json` matches keys
case-insensitively, so only a hyphenated key (`defer-first`) can fail a decode
test before its tag exists.

## Gotchas

- Flags, in both commands, are `-c`/`--config`, `--tests`, `--build-tags`,
  `--modules-download-mode` and `-v`/`--version` only: under pflag,
  `-config rules.yml` parses as `-c onfig`, with `rules.yml` as a package, and
  `--tests false` takes `false` as a package.
- golangci-lint takes the first file named exactly
  `.golangci.{yml,yaml,toml,json}` in `./`, then in the directory of its first
  package argument and each parent up to `/`, then in the home directory
  (`getConfigSearchPaths`, `pkg/config/base_loader.go`). The root's
  `.golangci.yml` is this project's own lint configuration, which is why the
  example for users is `golangci.example.yml`; the e2e passes `-c` to every
  run, so no configuration of the tree reaches it.
- `make golangci` runs `golangci-lint custom`, which clones golangci-lint under
  `os.TempDir()` (`pkg/commands/custom.go`): where `/tmp` is not writable, as
  in a sandbox, set `TMPDIR` to a directory that is.
- Never move or recreate a pushed tag: Go keeps the old content in the download
  cache, the VCS clone under `$(go env GOMODCACHE)/cache/vcs` and the module
  index in GOCACHE; recovering needed `go clean -cache`.
- `GOPRIVATE=github.com/pragmabits/precept go install …/cmd/paircheck@<tag>`
  fetches from GitHub without the proxy or sum.golang.org. A request for a
  version to proxy.golang.org caches it for good; whether it already has one is
  read from the `index.golang.org` feed, which fetches nothing.
- A checkout build's `--version` ends in `+dirty` while any untracked file that
  is not ignored sits in the tree.
