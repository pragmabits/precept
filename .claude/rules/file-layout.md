# Layout of a file

A file is read from the top. What is written first is what has to be known first.

## The order

```
package doc
imports

var ( sentinel errors )                 the vocabulary of failure this file adds
const ( ... )                           file-level constants

interfaces the subject calls through    the contract it is written against
types and enums the subject consumes    you cannot read the subject without them
type Subject                            what the file is about
func NewSubject                         the constructor
func (s Subject) Public                 methods, public first
func (s Subject) private
func (s Subject) MarshalJSON            what a stdlib interface asks of it
func (s Subject) String                 the rendering closes that group

type Secondary + its methods
interfaces the subject only declares    a port, satisfied elsewhere, uncalled here
```

**The package doc lives in the file named after the package**, and in no other
file: a package `obligation` is documented in `obligation/obligation.go`. There
is no `doc.go`. The file a reader opens first is the one carrying the package's
name, and a `doc.go` is a file with no subject of its own. A package that is
still only documentation has just that file; a package with one subject names
the subject's file after the package. Decided by the developer.

**Sentinel errors before constants**, and the reason is not taste: a sentinel is
what a caller reaches for first, with `errors.Is`. A constant is more often an
internal threshold. The most-reached-for thing goes where it is found first.

The standard library was measured for this and has no answer to give — of 20 files
carrying both a `const` block and a `var Err` sentinel, exactly 10 put each one
first. `os/file.go` opens with a *method*, on line 63, before any declaration.
There is no convention here to defer to, so the reasoned choice wins over the
imitated one.

## The subject comes first, except when it cannot

`type Subject` goes before the other types — **unless the subject names one of
them in a field or a signature.** Then that one goes above, because
`Rules []Rule` cannot be read by someone who has not met `Rule`.

So "the subject first" means first among peers, not first in the file.

## An interface goes at the top or at the bottom, and one question decides

Two lines of the order block name an interface. The question that separates them:

**Does the code in this file call through it?**

- **Yes — it goes above the first type.** It is an input to the subject, not a
  consequence of it. `resolve` takes a `Loader` and calls `Load` on it for each
  package a rule names, so nobody reads the subject without having met the
  interface first. Put last, it is a forward reference across the whole file,
  which this file permits, but a hundred lines of it is not "you keep reading
  and you meet it".
- **No — it closes the file.** It is a port this file declares for somebody else
  to satisfy.

**The position is fixed, not judged per file.** An interface the subject calls
through goes first even when some other consumed type is needed a few lines
sooner, and the reason is that this whole document is convention carried by
review with no tool behind it: "always first" survives a reader in a hurry, and
"whichever is needed soonest" does not.

## An enum's values stay with the enum

The constants rule governs file-level constants — a threshold, a limit, a
published policy value. The values of an enumeration are part of the type, and
they sit with it wherever the order above put it:

```go
type Coverage string

const (
	CoverageAnyPath   Coverage = "any-path"
	CoverageEveryPath Coverage = "every-path"
	CoverageNone      Coverage = "none"
)
```

Hoisting those to the top of the file separates a type from what it means.

## What the standard library asks for goes last

A method that exists because an interface outside this project requires it is
not what the type is about. It is reached *through* the interface — by `%v`, by
`errors.Is`, by `json.Marshal` — and hardly ever looked up by name, so it is the
one method nobody scrolls to find.

It goes below the type's own methods, private ones included. That is the single
place a public method sits under a private one, and the reason is that it is not
really the type's method: it is the outside's.

Last **among the methods**, not last in the file. A free helper written for one
of them keeps its place at the bottom, and the method naming it is a forward
reference, which this file already permits.

Within the group, one interface's methods stay together — `MarshalJSON` beside
`UnmarshalJSON`, `Scan` beside `Value` — and the group closes with `String`. A
`String` is a convenience the outside asked for, the furthest thing from what
the type *does*, so it sits furthest from the declaration of what the type *is*.

**`error` is the case that draws the line, and it falls on the other side.**
`Error() string` has the signature of a `String` and none of its standing: a
type that exists to be an error is *for* that rendering, so `Error` stays among
the type's own methods. `Unwrap` is what the standard library asks of it, and
`Unwrap` is what goes last.

## No jump backwards

The direction of the jump is what matters, and this is the checkable half of
"reads top to bottom".

A **forward** reference is not a jump: you keep reading and you meet it. A
**backward** reference makes the reader scroll up and lose the thread. So a
declaration may freely name something defined below it, and the file is wrong
when understanding line 40 requires having read line 90 and come back.

So a constructor sits directly under its type even when it names an error and a
helper defined further down. The constructor belongs with its type; those names
are ahead of the reader, not behind.

## One file, one subject

This is the rule that makes ordering mostly answer itself. `config.go` holds
the configuration and what it needs; `classify.go` holds the classification of
a declaration.

**When the order does not resolve itself, the symptom is not missing order — it
is a file with more than one subject.** Split by subject before reordering.

A subject is not a type. A file may hold `Config`, `Rule`, `Slot`, `Transfer`
and `InvalidRuleError`: one subject — the configuration, from the whole of it
down to a single slot, and what it refuses — seen at five points. Splitting
that into five files would be the failure in the other direction.

## Godoc says what holds. It is not a diary.

A doc comment states what the thing is and the invariant that holds for it.

It does **not** carry the argument that produced the decision, the history of
another codebase, or what changed and when. Those go where they can be read as a
whole — the PRDs, in `.local/docs/prds/`. A
comment that recounts why a decision was reasonable is not documenting the code,
it is defending the author, and it is charged to every future reader.

The test: **who is this sentence for?** There is no team here. A comment that
explains a choice to someone who was in the conversation where the choice was
made is paying rent for nothing.

## Comments inside a body

Only where the code cannot say it. A body comment records a decision the
surrounding lines do not reveal — see
[exceptions-and-comments.md](exceptions-and-comments.md). Anything narrating what
the next line does is noise the reader has to skip.

## Known blind spot

**No tool checks any of this.** `gofmt` never reorders declarations, and
`decorder` — the golangci-lint linter for declaration order — enforces the
opposite: its `dec-order` requires every `type` before every `func`, which
forbids putting a type's methods under it as soon as a file holds two types.
Verified:

```
a.go:15:1: type must not be placed after func (desired order: var,const,type,func)
```

Its `dec-num` check would also forbid a file carrying two `const` blocks, which
any file holding an enum and a threshold does.
So `decorder` stays out of the enabled set, and this whole file is convention
carried by review, the same standing as the naming blind spot in
[naming.md](naming.md).
