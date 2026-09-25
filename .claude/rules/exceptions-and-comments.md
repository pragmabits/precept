# Comments, exceptions and tools

## A comment records a decision

Not what the code does — the code already says that. A comment says what was
decided and what motivated it. Everything else is noise the next reader has to
skip.

## Every exception states why it exists

> An exclusion without a justification is indistinguishable from the rule never
> having been enabled.

That is the whole standard. An exception with the reason written next to it is a
decision. An exception without one is an unexplained hole, and no reader can tell
it apart from a rule that was never turned on.

Scope an exception as narrowly as the reason justifies. If the false positive is
known in one file, exclude that file — not the rule.

## A rule holds or it does not

There is no rule at half strength. If a gate has to go green before the debt
behind a rule is paid, remove that rule from the enabled set and say why. Do not
loosen it into something that passes.

When something fails, fix the code — never the rule. If the rule genuinely does
not fit, take it out whole, with the reason written down.

## Document what the tool cannot see

Silence from a tool is not approval. Where a check has a blind spot, that blind
spot is written down at the place it matters, so the next reader does not mistake
one for the other.

## Do not let the tool flatter you

Output caps are off in `.golangci.yml`, on purpose: the defaults turn "the gate is
green" into "the gate got tired". A trustworthy count is worth more than a green
check.

The same standard governs `//nolint`. Nobody silences a rule mid file: either the
code is fixed, or the exception goes into the config with its reason written,
where a reader looking for the holes finds all of them in one place. **Known
blind spot:** nothing enforces that — `nolintlint` is not in the enabled
set, so an inline directive would pass.

## The gate

`golangci-lint run ./...` and `go test ./...`. They exit 0 or the work is not
done. Never report a result you have not run — and a green gate proves what it
checks and nothing else.
