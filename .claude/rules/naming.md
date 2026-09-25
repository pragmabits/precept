# Naming

**A name is a word.** Shortening a name means removing a redundant word. It never
means mutilating a word.

```
signedUserSessionInTheSystem   too many words
userSession                    one redundant word, if no other session is near
session                        right
sess                           not a word
```

The two extremes are the same mistake pointed in opposite directions: one makes
the reader sweep past useless words, the other makes the reader translate. Both
charge the reader for the writer's convenience.

## The test

Does expanding it cost a beat of thought?

- `err`, `ctx`, `id` — no. You read them and move on.
- `sess`, `mu`, `conn`, `cfg`, `buf`, `req`, `res` — yes. And that cost is paid on
  every read, by everyone, forever, so the author could save four keystrokes once.

Scope carries meaning. `session` needs no `user` prefix when no other session is
in scope and nothing imported collides with it. Say what the scope does not
already say, and nothing more.

## The name must be true, and must name a thing

Two failures the abbreviation rule does not catch.

**A name that asserts an origin has to hold for everything it holds.** `mutex`,
for the analyzer whose rule is a trigger followed by a satisfier, passes the
test above — nobody expands it, and the standard library names packages `http`,
`json`, `csv`, `sql`. It fails for a different reason: a mutex is one pair among
the ones a user configures. `Begin → Commit | Rollback` and `Open → Close` are
not locks, and neither is the API a user configures next. `obligation` is true of all of them: it names what a
trigger opens without claiming what the API is.

**A name has to name a subject, not the container it arrived in.** `yaml`, for
the package that reads the naming analyzer's rules, is a word, so the
abbreviation rule lets it through, and it still names nothing — no package has
"the YAML" as its subject, the naming PRD models the configuration as Go
structs that do not depend on YAML, and the decoder the package imports is itself named
`yaml`. `ruleyaml` is this defect, and `rulefile` is the same defect with the
subject kept as a qualifier: a qualifier that names the container still names
the container. The subject there is a thing that already has a word, in that
PRD's title: the rules together are a naming `policy`. When no candidate names
a subject, look for the unnamed concept before the next word.

A candidate is also out for colliding with what this project already reads by
that word: `analysis` is taken by `golang.org/x/tools/go/analysis`, which every
analyzer imports, and `channel` reads as `chan` to every Go reader for a beat.
A word true in another vocabulary is out the same way: to a user an import path
names a package, and to anyone who has written an analyzer a `Package` is a
`*types.Package`, so a field holding an import path is named for what it holds,
its `Path`.

## A name the language's convention settles

Three kinds of name follow Go's convention instead of the word rule, because the
convention is what every Go reader reads without translating — the test above,
answered by the language rather than by this project.

- **A receiver is one letter:** `func (p parts)`.
- **A type parameter is one capital letter**, as the standard library writes
  them: `T` for the type a function is written over and `U` for a second one,
  `E` for an element, `K` and `V` for a key and a value — `slices`, `maps`,
  `errors.AsType`. `decode[T any]`, not `decode[Entry any]`. Decided by the
  developer.
- **A test's parameter is `t`, and a fuzz target's is `f`:**
  `func TestParse(t *testing.T)`, `func FuzzParse(f *testing.F)`, as the
  `testing` package documents them. Decided by the developer.

## A name that comes from an API

A name that comes from an API is not a name you chose. `flag.Args()` stays,
because the standard library decides it. The rule governs names the author picks;
it has no claim on names the author merely reaches.

`forbidigo` cannot tell the two apart — it sees the identifier, not who chose it
— so reaching one of these produces a finding. When that happens the exception
goes into `.golangci.yml`, scoped to the path where the library is reached and
matched on the exact spelling, so a name written by hand is still caught there.
The case known to produce one is a library's field set as a bare key in a
composite literal, which `forbidigo` reads as a bare identifier.

## Where this is enforced

`forbidigo`, `revive` and `varnamelen` in `.golangci.yml`. The `forbidigo`
patterns are anchored (`^...$`) so they match a bare identifier and never a
component of one — that anchor is what keeps `configFile` from being reported as
`cfg`.

`revive`'s `receiver-naming` holds a receiver to one character. `varnamelen`
checks neither receivers nor type parameters: its minimum of two characters
would refuse every one.

**Known blind spots.** Nothing verifies that a name is a word: no linter carries
a dictionary, and a denylist is reactive by nature — `conn` only gets listed
after somebody writes `conn`. And `forbidigo` reads usage sites only, so a
forbidden abbreviation in a struct field is reported at every read of it and
never at the line that declares it. Nothing checks that a type parameter is one
capital letter either: review is what carries the convention.
