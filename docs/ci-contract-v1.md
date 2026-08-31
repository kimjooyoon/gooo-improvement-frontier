# CI and repository contract v1

GitHub Actions is the execution authority. The workflow uses Go 1.27 and is
the place where formatting, build, test, vet, source compilation, conformance,
and deterministic replay are run. Developers do not run local Go build, test,
vet, formatting, compiler, harness, or conformance commands for this project.

Every attempt writes an append-only NDJSON record under the caller-owned
temporary output root. Logs, two deterministic conformance output trees,
machine receipts, human reports, inventory, and runtime metrics are uploaded as
one CI artifact even when a required phase fails.

The runtime receipt reports exact build and test wall times, peak RSS, executed,
reused, skipped, and not-observed counts, as well as directory, file,
physical-line, Go-file, Go-line, and Gooo-line counts. Root `README.md` is
excluded from the inventory. Product authority is always
`repository_writes=0`, `local_test_executions=0`, and
`cross_project_required_gates=0`; checkout, build, test, vet, compile,
conformance, and determinism are recorded separately as development actions.

All evaluator outputs are written only to absolute caller-owned output paths.
The evaluator is a planner and receipt producer: it does not write source
repositories, merge pull requests, alter CI, or execute downstream operations.

## Release process

Main is bootstrapped with exactly `.gitignore`, `LICENSE`, and `README.md`; the
bootstrap commit and paths are recorded in `docs/bootstrap-record.json`.
Substantive implementation is delivered through one implementation pull
request. Main is validated again after merge. Immutable release enforcement is
enabled before publishing. A failed or non-immutable release attempt is kept
in its release/process receipt and a corrected patch release is published
without rewriting history.

