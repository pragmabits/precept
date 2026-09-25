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

  - id: file
    trigger: os.Open
    satisfiers:
      - (*os.File).Close

  - id: lock
    trigger: (*sync.Mutex).Lock
    satisfiers:
      - (*sync.Mutex).Unlock
```

A function or method is named as
[`types.Func.FullName`](https://pkg.go.dev/go/types#Func.FullName) spells it:
`os.Open`, `(*os.File).Close`. The `*` is optional, since a type cannot declare
a method name on both its value and its pointer. A method of a generic type is
named with or without its type parameters.

The value the obligation is on is found by its type. It may sit in the
trigger's receiver, an argument or a result, and in a satisfier's receiver or
an argument:

```go
r.Open()               // receiver → receiver
tx, err := db.Begin()  // result → receiver: tx.Commit()
Acquire(r)             // argument → argument: Release(r)
h, err := Open(path)   // result → argument: CloseHandle(h)
```

When more than one slot has that type, the rule names the slot:

```yaml
  - id: pooled
    trigger:
      name: (*example.com/pool.Cache).Swap # Swap(old *Conn) *Conn
      slot: result 0
    satisfiers:
      - (*example.com/pool.Cache).Put # Put(conn *Conn)
```

| Key | Default | Meaning |
| --- | --- | --- |
| `id` | required | Starts every diagnostic of the rule. Unique. |
| `trigger` | required | The call that opens the obligation. A name, or `{name, slot}`. |
| `satisfiers` | required | The calls that discharge it. Names, or `{name, slot}`. |
| `open-on-error` | `false` | Open the obligation also where the trigger returned a non-nil error. |
| `deferred-closure` | `any-path` | When a satisfier inside a deferred closure counts: `any-path`, `every-path` or `none`. |
| `transfer` | `all` | Which ways out of the function take the obligation with the value: `all`, `none`, or `{return, store, argument}`. |

A slot is `receiver`, `argument N` or `result N`, counted from zero.

### What it checks

- A path is checked where it returns. A satisfier counts if it was called on
  the path or deferred. A `panic`, `os.Exit`, `log.Fatal`, or any call that does
  not return abandons the path.
- Where the trigger returns an error, the obligation exists only on the path
  where it is nil: after `if err != nil { return err }` there is nothing to
  discharge. A check paircheck does not read, such as `errors.Is`, leaves the
  path uncertain, and an uncertain path is not reported.
- A satisfier inside a deferred closure counts as `deferred-closure` says. The
  default counts one called on any path of the closure, as in
  `defer func() { if err != nil { tx.Rollback() } }()`.
- The same value is followed through aliases and fields: `s.res.Open()` and
  `defer s.res.Close()` are one value. A satisfier on a value that may be the
  same, such as another parameter of the same type, discharges; one on a value
  proven to be another, such as a different allocation, does not.
- A call through an interface discharges when the interface holds the value:
  `var rc io.ReadCloser = f; defer rc.Close()` discharges `os.Open`.
- A value that leaves the function takes its obligation along: returned,
  stored in a field, a package variable, an element or a channel, or passed to
  another function, a `go` statement or a closure that is not deferred.
  `transfer` narrows this.
- Two triggers on the same value need two satisfiers.

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
configuration or the packages fail to load.

`-c`, `--config` takes a file with the rules, as above, or a `.golangci.yml`
carrying them in its settings, native or as a module plugin. Flags come before
the packages, as in the `go` command.

#### Validating the rules

A misspelled name matches nothing, and the analyzer cannot tell: it sees only
the package it analyzes and what that package imports. The `validate` mode
loads the packages the rules name and checks every rule: the names exist, one
type links the trigger to every satisfier, and each slot is in the signature.

```sh
paircheck validate --config .golangci.yml
```

Run it in CI next to the analysis.

### golangci-lint

paircheck is available as a
[module plugin](https://golangci-lint.run/docs/plugins/module-plugins/).

From a checkout, `make golangci` builds golangci-lint with the plugins of this
repository as `golangci-lint-precept` in `GOBIN`, beside the golangci-lint
already installed, which it needs to run. The build clones golangci-lint, so it
needs git and the network.

```sh
make golangci
golangci-lint-precept run ./...
```

In another project, the same build comes from `.custom-gcl.yml`:

```yaml
version: v2.13.2
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

- A helper that discharges the obligation is not a satisfier: passing the value
  to it is a transfer, and `transfer: {argument: false}` reports it.
- A rule on an interface method matches calls through that interface, not
  calls on the concrete types implementing it.
- A call through a function value is not matched.

## Development

`make` lists the targets. `make test` and `make lint` run what the CI runs, and
`make format` rewrites what the formatters would change.

## Status

No version has been tagged yet.

## Use of AI

The code, tests and documentation of this repository are written with Claude
(Anthropic), directed and reviewed by the author, who makes the design
decisions. The code is written test-first.

## License

[MIT](LICENSE)
