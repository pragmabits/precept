# Reading before writing, and the order of work

A step is planned from what the project says, in the order its dependencies
allow, and shown before any of it is written. Decided by the developer.

## Read first

Before a line of code, and before an answer about the project, read what it
rests on: the code the step touches and what calls it, these rules, the PRDs
under `.local/docs/prds/`, the latest session reports under
`.claude/sessions/`, and whatever else is available.

An answer about the project rests on something concrete in it: a file and a
line, a decision in a PRD, a measurement run in the session. What was not read
or run is said to be unverified, and never stated as fact.

A document is read as a claim to check, not a list to relay. A report's pending
items, a next step named in any document and a name in a PRD's proposed
architecture are premises, and a premise nobody examined is examined before
work is built on it.

## The order follows the dependencies

A part is built after what it depends on exists. What a part needs in order to
be used — the operation that calls it, the entry point it is reached through,
the type it receives, the reader of what it returns — exists first, or comes in
the same step. Nothing is built against a dependency that is only planned.

Each step leaves its result usable from outside, through an exported function
or type that a test outside the package calls. A package whose only exports are
sentinels, or whose code only its own tests reach, is not a finished step.
`golangci-lint run` counts a test as use, so it does not see it. This run does,
and it is part of `make lint` and of the CI:

```
golangci-lint run --tests=false --enable-only unused ./...
```

## No surprise refactor

A change outside what the step is about — a type moved between packages,
another package's output changed, existing code renamed or restructured — is
named in the plan with the alternative that avoids it, and approved before it
is made. A change of that kind found while implementing stops the work: it is
reported, with its alternative, and waits.

## The plan is shown before the code

Before writing, the plan says:

1. what exported function or type leaves the result usable, and what calls it;
2. what the step touches outside its package, each with the alternative that
   avoids it;
3. what inherited names, orders and premises it builds on, and that each was
   examined.

Work starts once the plan is approved. The report at the end says what can be
done with the result from outside, and what was not verified.

A question only the developer can weigh — scope, what a PRD leaves open — is
asked with `AskUserQuestion`, during the plan or when the work finds it. What
research, these rules or engineering judgement settles is not asked: it arrives
decided, with its reason.

## Known blind spot

Nothing enforces any of this. No linter reads a plan, and the gate is green on
a package no caller reaches. It is carried by whoever does the work and by
review, with the same standing as the order of test-first in `code-shape.md`.
