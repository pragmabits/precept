# precept

[`go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) analyzers for
rules a project declares in its configuration.

## paircheck

paircheck reports a call that opens an obligation on a value when a path
returns from the function before a call that discharges it. The pairs are
yours to declare: any function or method opens, any function or method
discharges.

```go
func Rename(db *sql.DB, from, to string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE names SET name = ? WHERE name = ?`, to, from); err != nil {
		return err // tx is still open on this path
	}
	return tx.Commit()
}
```

```
store.go:6:13: [transaction] Begin requires one of [Commit, Rollback] on tx before function exit
```

### Rules

```yaml
rules:
  - id: transaction
    trigger: (*database/sql.DB).Begin
    satisfiers:
      - (*database/sql.Tx).Commit
      - (*database/sql.Tx).Rollback
    transfer: {return: true, store: false, argument: false}

  - id: file
    trigger: os.Open
    satisfiers:
      - (*os.File).Close

  - id: lock
    trigger: (*sync.Mutex).Lock
    satisfiers:
      - (*sync.Mutex).Unlock
```

The `transaction` rule narrows `transfer`: a transaction is ended by the
function that opens it, or by the caller it is returned to. Stored in a field or
handed to a helper, it is reported.

A function or method is named as
[`types.Func.FullName`](https://pkg.go.dev/go/types#Func.FullName) spells it:
`os.Open`, `(*os.File).Close`. The `*` is optional, since a type cannot declare
a method name on both its value and its pointer. A method of a generic type is
named with or without its type parameters, and a generic function without them.

A package of your own module is named relative to it: `./` followed by its
path inside the module, or `.` alone for the package at the module's root.

```yaml
  - id: platform
    trigger: (./internal/platform/transaction.Provider).New
    satisfiers:
      - (./internal/platform/transaction.Transaction).Commit
      - (./internal/platform/transaction.Transaction).Rollback
```

`..Open` is `Open` in the root package. A relative name is resolved against the
module of each package analyzed, as its `go.mod` declares it, and a package
that belongs to no module fails the analysis. `validate` resolves it against
the module of the current directory.

The value the obligation is on is found by its type. It may sit in the
trigger's receiver, an argument or a result, and in a satisfier's receiver or
an argument:

```go
r.Open()               // receiver → receiver
tx, err := db.Begin()  // result → receiver: tx.Commit()
Acquire(r)             // argument → argument: Release(r)
h, err := Open(path)   // result → argument: CloseHandle(h)
```

A `context.Context` is left out of that search: nearly every call of an API
takes one, beside the value. A rule whose value is the context itself names
the slot on each side.

A type parameter is found only through a named slot: the `L` of
`Hold[L Leased](lease L)` and the `L` of `Free[L Leased](lease L)` are
different types, so a rule whose value is a type parameter names the slot on
each side. So does a rule whose value is built from type parameters, such as
`[]L` or `map[K]V`: the two slots then have the same shape, and each type
parameter of one side stands for exactly one of the other.

When more than one slot has that type, the rule names the slot:

```yaml
  - id: pooled
    trigger:
      name: (*example.com/pool.Cache).Swap # Swap(old *Conn) *Conn
      slot: result 0
    satisfiers:
      - (*example.com/pool.Cache).Put # Put(conn *Conn)
```

When the value is a function the code has to call, `call` in the satisfiers
stands for calling it:

```yaml
  - id: permit
    trigger: (*example.com/sem.Semaphore).Acquire # Acquire() (release func())
    satisfiers:
      - call
```

Only a call of a function value of the value's own type counts, so a callback
of another signature is not taken for it. `call` takes no slot.

| Key | Default | Meaning |
| --- | --- | --- |
| `id` | required | Starts every diagnostic of the rule. Unique. |
| `trigger` | required | The call that opens the obligation. A name, or `{name, slot}`. |
| `satisfiers` | required | The calls that discharge it. Names, `{name, slot}`, or `call`. |
| `open-on-error` | `false` | Open the obligation also where the trigger returned a non-nil error. |
| `deferred-closure` | `any-path` | When a satisfier inside a deferred closure counts: `any-path`, `every-path` or `none`. |
| `transfer` | `all` | Which ways out of the function take the obligation with the value: `all`, `none`, or `{return, store, argument}`. |
| `require-defer` | `false` | Count only a deferred satisfier: one called on the path does not discharge. |
| `idempotent` | `false` | A trigger on a value whose obligation is open opens no other. |
| `defer-first` | `false` | Require a deferred satisfier before any other call once the obligation opens. |
| `on-success` | `false` | On every return that hands back no failure, require a satisfier called on the path: a deferred one alone does not count. |

A slot is `receiver`, `argument N` or `result N`, counted from zero.

Beside `rules`, the configuration takes `failures`: the functions and methods
whose call returns an error that is not nil, besides `errors.New` and
`fmt.Errorf`. A rule that is `on-success` reads a return of what one of them
returned as a failure.

```yaml
failures:
  - ./internal/errs.Wrap

rules:
  - id: platform
    trigger: (./internal/platform/transaction.Provider).New
    satisfiers:
      - (./internal/platform/transaction.Transaction).Commit
      - (./internal/platform/transaction.Transaction).Rollback
    require-defer: true
    on-success: true
```

`paircheck.example.yml` and `golangci.example.yml`, at the root of this
repository, carry the `transaction`, `file` and `lock` rules with every optional
key written at its default, and an empty `failures`: in the command's own format
and as golangci-lint settings.

### What it checks

- A path is checked where it returns. A satisfier counts if it was called on
  the path or deferred. A `panic`, `os.Exit`, `log.Fatal`, or any call that does
  not return abandons the path.
- A recovered panic, as in an HTTP server, leaves open an obligation whose
  satisfier was called on the path. `require-defer` counts only a deferred
  satisfier. `defer-first` also reports a call made before that `defer`, where
  a panic would leave the obligation open; a satisfier, a builtin and the
  arguments of the deferred satisfier do not count. A panic that comes from no
  call, such as a nil dereference, is still not seen.
- Where the trigger returns an error, the obligation exists only on the path
  where it is nil: after `if err != nil { return err }` there is nothing to
  discharge. A check reads the trigger's error until another value is stored in
  its variable: after `err = step()`, `if err != nil` is about `step`. A check
  paircheck does not read, such as `errors.Is`, leaves the path uncertain, and
  an uncertain path is not reported.
- A satisfier inside a deferred closure counts as `deferred-closure` says. The
  default counts one called on any path of the closure, as in
  `defer func() { if err != nil { tx.Rollback() } }()`.
- The same value is followed through aliases and fields: `s.res.Open()` and
  `defer s.res.Close()` are one value. A satisfier on a value that may be the
  same, such as another parameter of the same type, discharges; one on a value
  proven to be another, such as a different allocation, does not.
- A call through an interface discharges when the interface holds the value:
  `var rc io.ReadCloser = f; defer rc.Close()` discharges `os.Open`.
- A satisfier called through a method value discharges:
  `closer := f.Close; defer closer()`. The method value stands for the value:
  kept in a field, returned or passed along, it takes the obligation with it.
- A value that leaves the function takes its obligation along: returned,
  stored in a field, a package variable, an element or a channel, or passed to
  another function, in a plain or a deferred call, a `go` statement or a closure
  that is not deferred. `transfer` narrows this.
- Two triggers on the same value need two satisfiers, unless the rule is
  `idempotent`: then a second `Serve` on a running server needs no second
  `Shutdown`.
- A deferred `Rollback` discharges every path, so nothing asks for the
  `Commit`, and a function that never commits loses its write without an
  error. `on-success` asks every return that is not a failure for a satisfier
  called on the path, `Commit` or `Rollback`, while the `defer` still covers
  the error and the panic. `return tx.Commit(ctx)` calls it on the path. A
  return is a failure when the error it hands back cannot be nil: a concrete
  value such as `&ValidationError{}`, what `errors.New`, `fmt.Errorf` or a
  function in `failures` returned, a package-level error such as `io.EOF`, or
  an error a check on the path found not nil, as in
  `if err != nil { return err }`, until something else is stored where it was.
  Anything else owes the call, `nil` and the result of any other function
  included. In a function without an `error` result, every return owes it. A
  value that leaves the function still takes its obligation along.

When paircheck cannot tell, it stays silent: a false negative is preferred to a
false positive. `on-success` is the exception, by design: a return whose error
paircheck cannot prove a failure owes the call.

### Running it

```sh
go install github.com/pragmabits/precept/cmd/paircheck@latest

paircheck --config rules.yml ./...
```

From a checkout, `make install` installs the command of every analyzer where
`go install` puts it: `GOBIN`, or the `bin` of the first `GOPATH` entry.

The exit code is 0 with no diagnostic, 3 with diagnostics, and 1 when the
configuration fails to load, or the packages do, a test file that does not
compile included, or a rule cannot bind (see Validating the rules). The load
errors of the packages come first, and those of their tests only once the
packages load.

`-c`, `--config` takes a file with the rules, as above, or a `.golangci.yml`
carrying them in its settings, native or as a module plugin. A `.golangci.yml`
carrying both is refused: golangci-lint applies the module plugin's, and the
file does not say which the command should. Flags follow the GNU style:
before, between or after the packages, and `--` ends them.

The `_test.go` files are analyzed too, those of the package and those of its
external `_test` package, as golangci-lint analyzes them. `--tests=false`
leaves them out. When `-c` names a `.golangci.yml`, its `run.tests` decides
where the command line does not, read as golangci-lint reads it: a `--tests`
written there wins, as in golangci-lint. A boolean flag takes its value after
`=`: in `--tests false`, `false` is a package.

The build flags follow golangci-lint's too. `--build-tags` takes build tags,
comma-separated, and adds them to the `run.build-tags` of a `.golangci.yml`.
`--modules-download-mode`, which is `mod`, `readonly` or `vendor`, replaces
its `run.modules-download-mode`. The load passes both to go, as `-tags` and
`-mod`, for the analysis and for `validate`. A flag that replaces a key,
`--tests` or `--modules-download-mode`, leaves the file's key unread, as
golangci-lint does.

A finding in a generated file is dropped, as golangci-lint drops it by
default: a file with a `// Code generated … DO NOT EDIT.` comment before its
`package` clause. The `linters.exclusions.generated` of a `.golangci.yml`
says which files: `strict`, the default, `lax`, which also drops a file whose
comments before or right after the `package` clause say it is generated, or
`disable`, which drops none. As in golangci-lint, any other text reads as
`lax`. Whatever the mode, a finding is printed, and read as generated or not,
where golangci-lint places it. After a `//line` directive, which a generator
writes to say where the code below it came from, that is where the directive
points when it is a Go file, and otherwise where the code is: after
`//line view.tmpl:5`, in `view.go`. In a file that imports `C`, it is that
file, not the one cgo rewrites in the build cache, and a finding outside a Go
file, such as in a file cgo generates, is never printed. When a directive
points to a file that cannot be read, the command warns, as golangci-lint
does, and drops no finding in a generated file.

From a `.golangci.yml` the command reads only the paircheck settings, those
three keys of `run`, `tests`, `build-tags` and `modules-download-mode`, and
`linters.exclusions.generated`. Their keys may be written in any case, as
golangci-lint reads them, and two keys that differ only in case are refused.
It does not follow the rest of `linters.exclusions`, `//nolint`, or any other
key of golangci-lint's.

`-v`, `--version` prints the version the binary was built from, such as `v0.1.0`.

#### Validating the rules

A misspelled name matches nothing, and the analyzer cannot tell: it sees only
the package it analyzes and what that package imports. A package that sees
every function a rule names, and still cannot bind it, fails the analysis with
the rule's error: its types do not settle one slot on each side, or a written
slot is not in the signature. Inside golangci-lint, that error stops every
`go/analysis` linter of the run. A package that sees only some of them skips
the rule, since a satisfier it does not see may be what settles the slot. The
`validate` mode loads the packages the rules name, under the build tags and
the modules download mode of the analysis, and checks every rule: the
names exist, one type links the trigger to every satisfier, and each slot is in
the signature. It also checks that every entry of `failures` exists and returns
an error as its last result.

```sh
paircheck validate --config .golangci.yml
```

Run it in CI next to the analysis.

### golangci-lint

paircheck is available as a
[module plugin](https://golangci-lint.run/docs/plugins/module-plugins/). The
project that uses it declares the build in its own `.custom-gcl.yml`:

```yaml
version: v2.14.0
plugins:
  - module: github.com/pragmabits/precept
    import: github.com/pragmabits/precept/plugin
    version: <version>
```

`golangci-lint custom` builds the binary carrying it, and `.golangci.yml`
configures it:

```yaml
version: "2"
linters:
  enable:
    - paircheck
  settings:
    custom:
      paircheck:
        type: module
        description: Reports an obligation not discharged on every path.
        settings:
          rules:
            - id: file
              trigger: os.Open
              satisfiers:
                - (*os.File).Close
```

golangci-lint analyzes the test files unless `run.tests` is `false`. An
exclusion rule drops paircheck's findings in them, for paircheck alone:

```yaml
linters:
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - paircheck
```

The exclusion drops findings, not the analysis: a rule that cannot bind in the
test variant of a package still stops the run, and only `run.tests: false`
keeps the test files out of it.

### Limits

- A helper that discharges the obligation is not a satisfier on its own:
  passing the value to it is a transfer, and `transfer: {argument: false}`
  reports it. A helper that always discharges can be listed among the
  satisfiers, as `example.com/app.closeQuietly`.
- A rule on an interface method matches calls through that interface, not
  calls on the concrete types implementing it.
- A function value is followed while it stays in the function. Called from a
  field or a map, it is not matched; putting it there already took the
  obligation along.
- Under `call`, a call of another function value of the same type, such as a
  `func()` parameter, may be the value: its identity is unknown, so it
  discharges, and paircheck stays silent.
- The command does not run as a `go vet -vettool`: it does not speak the
  protocol `go vet` uses with a vet tool.
- The test main that `go test` generates is left out by the command and by
  golangci-lint, not by the analyzer: inside a pass, only its name would tell
  it apart. Another driver that loads the tests, such as the `singlechecker`
  or `multichecker` of `golang.org/x/tools`, whose `-test` defaults to true,
  runs paircheck over it too, and a rule over a package `testing` imports,
  which the project never sees, can fail the run there. Such a driver drops
  the test main a loaded test variant is built for, or runs with `-test=false`.
- A package whose import path is a tested package's path plus `.test`, such
  as `m/a.test` next to `m/a` with tests, stops the load while the tests are
  loaded: the test main of `m/a` has that path, and `go list` gives
  conflicting information for it. golangci-lint stops the same way;
  `--tests=false` loads it.
- A test whose signature `go test` refuses, such as
  `func TestX(b *testing.B)`, is not a load error: the file compiles, and the
  command runs on, as golangci-lint does. `go vet` reports it.
- A `//line` directive over the `package` clause of a Go file, pointing to
  another Go file of the packages analyzed, makes golangci-lint print the
  findings of that other file in this one, at positions that are not theirs,
  and read them as this file's when it drops generated files. The command
  leaves each finding in its own file.
- `validate` loads the packages the rules name without their tests, whatever
  `--tests` or `run.tests` say: a rule naming a function or method declared in
  a `_test.go` file is refused as unknown, although the analysis applies it.
- Under `on-success`, an error checked with `errors.Is`, `errors.As` or a
  `switch` is not known to be a failure, and neither is one a wrapper outside
  `failures` returned: a return of it with no satisfier called on the path is
  reported. A package-level error variable left nil is read as a failure.
- Under `on-success`, a satisfier inside a deferred closure is not a call on
  the path: `defer func() { … err = tx.Commit(ctx) }()` is reported.

## defcheck

defcheck reports a declaration whose name a pattern you configure forbids, in
the kinds of declaration you choose. It reads a name where it is declared: a
use of it, an assignment to it, and a name another package declares are never
reported.

```go
type Server struct {
	cfg settings.Config // reported: a field
}

func Start(path string) (*Server, error) {
	cfg, err := settings.Load(path) // reported: a local variable
	if err != nil {
		return nil, err
	}
	if err := settings.Check(cfg); err != nil {
		cfg, err = settings.Load(cfg.Path + ".default") // an assignment
	}
	return &Server{cfg: cfg}, err
}
```

```
server.go:6:2: declaration name "cfg" is forbidden: avoid the cfg abbreviation
server.go:10:2: declaration name "cfg" is forbidden: avoid the cfg abbreviation
```

### Rules

```yaml
rules:
  - pattern: '^cfg$'
    kinds: [local-var, parameter, field]
    message: avoid the cfg abbreviation

  - pattern: '(?i)manager$'
    kinds: [package-var, field, function, method]
```

A pattern is a regular expression of the standard library's
[`regexp`](https://pkg.go.dev/regexp), matched anywhere in the name: `cfg`
forbids every name that holds it, `^cfg$` the name alone, and `(?i)^cfg$` the
name in any case.

| Key | Default | Meaning |
| --- | --- | --- |
| `pattern` | required | The names the rule forbids. |
| `kinds` | every kind | The kinds of declaration the rule applies to. |
| `message` | none | Said in the diagnostic in place of the pattern and the kind. The name is always said. |

A kind is what [`go/types`](https://pkg.go.dev/go/types) makes of the object
a declaration defines:

| Kind | Declared by | `go/types` |
| --- | --- | --- |
| `function` | `func F()` | `*types.Func` without a receiver |
| `method` | `func (t T) M()`, and a method of an interface | `*types.Func` with a receiver |
| `package-var` | `var` at the package level | `*types.Var`, `PackageVar` |
| `local-var` | `var`, `:=`, `range` and a type switch inside a function | `*types.Var`, `LocalVar` |
| `constant` | `const` | `*types.Const` |
| `field` | a field with a name of its own | `*types.Var`, `FieldVar` |
| `parameter` | a parameter of a function, a method, a function literal or type, or a method of an interface | `*types.Var`, `ParamVar` |
| `receiver` | the receiver of a method | `*types.Var`, `RecvVar` |
| `result` | a named result | `*types.Var`, `ResultVar` |
| `type` | `type T …`, and an alias, `type T = U` | `*types.TypeName` |
| `type-parameter` | `T` in `[T any]`, of a function, a type or the receiver of a method | `*types.TypeName` of a `*types.TypeParam` |

The diagnostic names the pattern and the kind, or says the rule's message:

```
declaration name "cfg" is forbidden by pattern "^cfg$" for local-var
declaration name "cfg" is forbidden: avoid the cfg abbreviation
```

A declaration is reported once for each rule that forbids it, in the order
the rules are written. A configuration is refused before any package is
analyzed when a pattern is empty or does not compile, when a kind is not one of
the table, and when a rule has the pattern, the kinds and the message of
another: the kinds are compared as a set, so their order, a kind written twice
and `kinds` left out, which is every kind, do not tell two rules apart.

`defcheck.example.yml` and `golangci.example.yml`, at the root of this
repository, carry these rules with every key written: in the command's own
format and as golangci-lint settings.

### What it checks

- Every name the package declares: each identifier
  [`types.Info.Defs`](https://pkg.go.dev/go/types#Info) gives an object, and
  the variable of a type switch, `switch v := x.(type)`, once for the switch.
- In `x, err := f()` after `x` is declared, only `err` is a declaration. A
  variable declared again in an inner scope is a declaration of its own, and
  so is each method's type parameter: in `func (l *List[E]) Push(v E)`, `E` is
  declared by `Push`.
- `_` declares nothing, and neither does a field embedded without a name of
  its own: `*http.Client` in a struct is not a field named `Client`.
- A renamed import, `import cfg "example.com/config"`, and a label are not
  kinds of declaration. revive's `import-alias-naming`, with its `denyRegex`,
  forbids patterns in import names.

### Running it

```sh
go install github.com/pragmabits/precept/cmd/defcheck@latest

defcheck --config rules.yml ./...
```

The command reads its rules, loads the packages and prints its findings as
paircheck's does (see Running it, under paircheck): the same flags, the same
exit codes, the test files analyzed unless `--tests=false`, generated files
dropped as `linters.exclusions.generated` says, and, from a `.golangci.yml`,
the defcheck settings, native or as a module plugin. It has no `validate`
mode: every rule is checked before any package loads, and a rule names nothing
a package declares.

### golangci-lint

defcheck is in the module plugin paircheck is in: the `.custom-gcl.yml` above
builds both, and `.golangci.yml` enables and configures each.

```yaml
version: "2"
linters:
  enable:
    - defcheck
  settings:
    custom:
      defcheck:
        type: module
        description: Reports a declaration whose name a configured pattern forbids.
        settings:
          rules:
            - pattern: '^cfg$'
              kinds: [local-var, parameter, field]
              message: avoid the cfg abbreviation
```

### Limits

- A name the language, the toolchain or an interface imposes is reported like
  any other the pattern matches: `init`, a `TestXxx` function, a method
  `MarshalJSON` that implements `json.Marshaler`. Narrow the pattern or the
  kinds, or, in golangci-lint, write `//nolint:defcheck` on the line.
- The test main that `go test` generates is analyzed by another driver that
  loads the tests, such as the `singlechecker` of `golang.org/x/tools`, and a
  rule that matches a name in it, such as `m`, reports it there, in a file of
  the build cache. The command and golangci-lint leave it out; such a driver
  runs with `-test=false`.

## Development

`make` lists the targets. `make test`, `make lint` and `make e2e` run what the
CI runs, and `make format` rewrites what the formatters would change. `make e2e`
builds golangci-lint with the plugins first, which needs git and the network,
and refuses to when golangci-lint's tag no longer names the commit the Makefile
pins.

## Use of AI

The code, tests and documentation of this repository are written with Claude
(Anthropic), directed and reviewed by the author, who makes the design
decisions. The code is written test-first.

## License

[MIT](LICENSE)
