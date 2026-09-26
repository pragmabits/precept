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

A slot is `receiver`, `argument N` or `result N`, counted from zero.

`paircheck.example.yml` and `golangci.example.yml`, at the root of this
repository, carry the `transaction`, `file` and `lock` rules with every optional
key written at its default: in the command's own format and as golangci-lint
settings.

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

When paircheck cannot tell, it stays silent: a false negative is preferred to a
false positive.

### Running it

```sh
go install github.com/pragmabits/precept/cmd/paircheck@latest

paircheck --config rules.yml ./...
```

From a checkout, `make install` installs the command of every analyzer into
`GOBIN`.

The exit code is 0 with no diagnostic, 3 with diagnostics, and 1 when the
configuration or the packages fail to load, or a rule cannot bind (see
Validating the rules).

`-c`, `--config` takes a file with the rules, as above, or a `.golangci.yml`
carrying them in its settings, native or as a module plugin. Flags follow the
GNU style: before, between or after the packages, and `--` ends them.

`-v`, `--version` prints the version the binary was built from, such as `v0.1.0`.

#### Validating the rules

A misspelled name matches nothing, and the analyzer cannot tell: it sees only
the package it analyzes and what that package imports. A package that sees
every function a rule names, and still cannot bind it, fails the analysis with
the rule's error: its types do not settle one slot on each side, or a written
slot is not in the signature. Inside golangci-lint, that error stops every
`go/analysis` linter of the run. A package that sees only some of them skips
the rule, since a satisfier it does not see may be what settles the slot. The
`validate` mode loads the packages the rules name and checks every rule: the
names exist, one type links the trigger to every satisfier, and each slot is in
the signature.

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

## Development

`make` lists the targets. `make test`, `make lint` and `make e2e` run what the
CI runs, and `make format` rewrites what the formatters would change. `make e2e`
builds golangci-lint with the plugins first, which needs git and the network,
and refuses to when golangci-lint's tag no longer names the commit the Makefile
pins.

## Status

The first tagged version is `v0.1.0`.

## Use of AI

The code, tests and documentation of this repository are written with Claude
(Anthropic), directed and reviewed by the author, who makes the design
decisions. The code is written test-first.

## License

[MIT](LICENSE)
