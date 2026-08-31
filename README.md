# gooo-improvement-frontier

`gooo-improvement-frontier` is a Go 1.27 causal-graph scheduler for a concrete
self-improvement question: which operations may run in parallel, which state
transitions must be serialized, and which blocked operation must not stop
unrelated progress.

The evaluator derives its answer from explicit graph edges, mutation
authorities, resource locks, immutable identities, and state precedence. It
does not use percentages, scores, inferred business value, or estimated
priority. It emits a deterministic plan, machine receipts, and a human dossier
only; it does not mutate repositories, merge pull requests, alter CI, or
execute downstream operations.

The source chain is:

`.gooo source → semantic IR → generated Go/evaluator → machine receipts → human report`

The fixed denominator is exactly 12 meta-activities. The canonical corpus has
six static cases covering normal parallelism and serialization, the real-shaped
frontier, a missing Guardian App credential, REST changed-file truncation as a
known refutation, an unresolved cycle, and an immutable-identity cache mismatch.

Run the CLI with caller-owned absolute output directories:

```text
gooo-improvement-frontier conformance --output-dir /tmp/gooo-frontier-out
```

CI is the execution authority. Local Go build, test, vet, formatting, compiler,
harness, and conformance execution are intentionally not part of the repository
process.
