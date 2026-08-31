# Causal improvement frontier protocol v1

The scheduler answers only graph questions: which operations can share a
parallel batch, which transitions require a serial cut, and which minimal
frontiers are blocked or refuted. It never ranks operations and never executes
the plan.

## Input binding

Every node binds a stable `claim_id`, `operation_id`, `current` state, `stage`,
`step`, `proof_choice`, `depends_on`, `blocked_by`, mutation authorities,
resource locks, immutable input digests, and `next_operation`. The only valid
proof choices are `FOUNDATION`, `COHERENCE`, and `REGRESSION`; these are the
explicit Münchhausen choices used at causal cycles.

An operation is eligible only when its causal dependencies are `CLOSED`, it is
not in a cycle boundary, and all required immutable evidence matches the exact
identity and digest bound by the node. Cache hits, green checks, and previous
execution are otherwise ignored as evidence.

## Scheduling

The scheduler computes transitive dependency closures from `depends_on`. Two
operations may share a batch only when both their closures and their combined
mutation-authority/resource-lock sets are disjoint. Shared protected branches,
release tags, receipts, denominators, and artifact authorities therefore
produce serial cuts. Lexical order is used only to make equivalent output
reproducible; it is never a priority or score.

The state lattice is explicit and ordered `REFUTED > UNKNOWN > CLOSED`.
Refuted nodes and their causal descendants are never schedulable. An unknown
external credential creates a minimal blocked frontier with its preserved
stage, step, reason, unknown class, next operation, and blockers; unrelated
components remain schedulable.

Cycles are never silently broken. A cycle with one consistent explicit proof
choice is emitted as a `CYCLE_MUNCHHAUSEN_CHOICE` serial cut and remains an
unordered proof boundary. A missing or mixed choice is emitted as
`CYCLE_REQUIRES_MUNCHHAUSEN_CHOICE` and the cycle is `UNKNOWN`.

The output is a plan plus machine receipt and human dossier. It does not merge
pull requests, change branches, alter CI, write repositories, or run downstream
operations.

