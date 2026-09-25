# The secure option is the default

When two options differ in security, the more secure one is taken. It does not
have to win an argument. The other one does.

Scope: a Go module of static analyzers, built by one developer, run standalone
and inside golangci-lint with the permissions of whoever runs it — on their
machine and in their CI, over their code. The PRDs aim at native integration
upstream, which would put this code in the binary of everyone who runs
golangci-lint. That is what the exceptions are measured against.

## The three exceptions

An exception is taken deliberately and named where it is taken. There are three.

| Exception | What it means |
| --- | --- |
| **High complexity** | The secure option costs machinery the project carries forever — not effort spent once. Writing more code is not complexity; a mechanism that has to be understood before anything near it can be changed is |
| **Infra out of proportion** | It adds a component to deploy, monitor and keep alive, to a project that ships a module and an executable and runs no service of its own |
| **Conflicts with a decision already taken** | The cost is churning something settled and everything downstream of it |

The third one has a limit worth writing down, because it is the one that decays:
**a prior decision does not win by seniority.** If it was taken without security
in view, the conflict reopens it. The exception covers the cost of the churn, not
the merit of what is being churned.

## A dominated option is not an option

The rule governs what gets *presented*, not only what gets chosen.

An option that loses on security and triggers none of the three exceptions is not
an alternative. Naming it as one asserts that a decision exists where none does,
and the cost lands on the reader, who spends the time to discover it was never a
question. A comparison table of two columns makes that claim by its shape, before
any argument in it is read.

Say what it was, say why it is out, move on.

Where such an option is already recorded as open, close it with the reason rather
than carrying it forward. **A list of open questions is where an unexamined
premise survives** — nobody re-derives an item that is only being copied, so it
accumulates the authority of having been repeated.

## Known blind spot

No tool enforces this. No linter ranks two designs by security, and the rule is
about a judgement made before any code exists for a tool to read.

So the only durable trace is the exception. When one is taken, it is written down
with its reason; when none is written, a reader is entitled to assume the secure
option was taken. That is the same standard as
[exceptions-and-comments.md](exceptions-and-comments.md): an exception without a
justification is indistinguishable from the rule never having been applied.
