# Shape of the code

Functions are short and flat. The numbers in `.golangci.yml` are 50 lines,
cyclomatic complexity 10, and at most 3 levels of control nesting, with
early return preferred over an else branch.

Tests are exempt from all three, and the reason is specific: a table of cases
grows in the table, not in the logic. Splitting one up to fit a line limit hides
what it covers.

## A line fits in 100 columns

A line holds 100 columns, counting a tab as two: the ruler the code is read by.
What does not fit is broken by `golines`, a formatter in the gate, and
`golangci-lint fmt` writes the break:

- a signature puts each parameter on a line of its own, with a trailing comma,
  and closes the parenthesis on the line of its results;
- a call puts each argument on a line of its own, and a composite literal each
  element;
- a condition joined by `&&` or `||` breaks after the operator, and a chain of
  calls at its dots.

```go
func diagnose(
	rule Rule,
	expression string,
	trigger token.Pos,
	exit token.Pos,
) analysis.Diagnostic {
```

A parameter of a broken signature carries its own type, as above. Tests are
held to the limit as well: a formatter has no exemption, and a case broken
across lines is read by its braces, as below. Decided by the developer.

**Known blind spots.** `golines` keeps grouped parameters on one line —
`trigger, exit token.Pos,` — so giving each its own type is carried
by review. A string literal or a comment past 100 columns stays as it is:
`golines` cannot split a string, comment shortening is off, and nothing reports
either line. And `golines` breaks a positional case over several lines without
keying it, which the section below refuses: a case it breaks is keyed by hand.

## A case is read by its braces

A case in a test table that does not fit on one line is a keyed literal with
its braces on lines of their own: one field per line, and a function field last.

```go
{
	defect: "no satisfier",
	want:   ErrNoSatisfier,
	change: func(rule *Rule) {
		rule.Satisfiers = nil
	},
},
```

A positional case carrying a function or a nested literal opens the case's
brace and the function's on the same line and closes them together with the
expected value, `}, ErrNoSatisfier},`, and nobody can tell where the case
ends. A multi-line raw string inside a case does the same to the indentation.
A case of a few short scalars may stay positional on one line. Decided by the
developer.

## Tests come first

Code is written test-first. For every behaviour, refusal or fix:

1. **Red.** Write the case and run it, and see it fail on an assertion. A
   compile error is not red: declare the name the test reaches — the sentinel,
   the type, the empty function — so the package builds and the case fails for
   the reason it exists.
2. **Green.** Write the least code that makes it pass, with the rest of the
   suite still passing.
3. **Refactor** with the suite green, or say there was nothing to refactor.

A fix starts with the case that reproduces the defect. A refusal starts with the
case that plants it, which is the shape both PRDs give their tests: a case the
analyzer reports beside a case it leaves alone.

The reason is what a test written afterwards cannot show. A case written against
code that already exists is shaped by that code and confirms what it does,
including what it does wrong.
The red run is the only evidence that the case can fail at all.

Checking a finished suite by disabling a check and watching a case fail is
still worth doing, and it is extra evidence, never a substitute for the red run.
A `depguard` rule is held to the same order: the planted violation comes before
the rule is trusted.

**Known blind spot.** Nothing enforces the order. The history shows the test
and the code landing together, never which ran red first. It is carried by
review, and a report of the work quotes the red output — a claim of TDD without
it is indistinguishable from a test written after.

## Errors are first class

Every error is handled or explicitly dismissed with a stated reason. Go:
`errcheck`. Its one exclusion is by the shape of the line rather than by a list
of function names — a deferred call is the last thing a function does and has
nowhere left to return an error to, while a list of names is wrong the moment
somebody forgets to add to it. The same call written without `defer` is still
reported. Blind spot this accepts: a deferred `Close` on a *write* path can lose
buffered data, and that error is worth handling; nothing will flag it.

Concurrency is verified, not trusted: `datarace`, `forbidden-call-in-wg-go`, and
`defer` restricted to loop, recover and immediate-recover.

## Type escapes

`any` is not a way out, and `interface{}` is the same escape spelled longer.

Configuration is where the temptation lives, because settings arrive from a
driver with no schema of their own: golangci-lint hands a plugin its settings
as `any`. They are still not carried inward as `map[string]any`: the naming PRD
models the configuration as Go structs, so the settings are decoded into them
once, at a named boundary, where an invalid rule is refused before a single
diagnostic is emitted. A setting typed as `any` moves that failure to whoever
reads the value three files later, without the rule it came from.

Where a value truly crosses out of what the types know, name the type at that
boundary in one line, exactly where the knowledge ends:
`pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)`. The author is then made to
declare what they believe they received.
