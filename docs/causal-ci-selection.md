# Causal CI selection contract

This repository is an external consumer of the Gooo compiler boundary. The
workflow obtains `gooo check --semantic --json` and `gooo graph dump` from a
pinned release, then `prepare` binds the observed source bytes, semantic digest,
graph digest, claim evidence, and the fixed test registry into one input.

The planner traverses `claim.activity_path` values through activity names that
are present in the released semantic graph. A test is selected only when its
registered semantic activity intersects that path. A test outside the path is
`SKIP` only when it has exactly one exclusion evidence record whose path does
not intersect the changed causal activity set. There is no extension-based or
file-name selection rule.

The denominator is fixed at twelve cells:

- `FOUNDATION`, `COHERENCE`, and `REGRESSION` each have four cells;
- `DRIVER`, `OUTCOME`, and `GUARDRAIL` each have four cells;
- every cell binds one Gooo activity, one semantic graph node, one binding
  digest, one generated artifact path, and this evaluator.

Stale graph, missing activity binding, source/semantic/claim digest mismatch,
or absent causal evidence is `UNKNOWN`. It preserves `stage`, `step`, `reason`,
`unknown_class`, `next_operation`, and `blocked_by`, and emits no executable
full-suite fallback. Malformed input, an unrecognized `FIXED_POINT` decision,
authority escalation, and an incorrect exclusion are `REFUTED` and fail closed.

The fixture observes a four-test full run and a generated selected run. The
selected run has two executed tests, one exact prior receipt reused, and one
test skipped with causal exclusion evidence. Full and selected wall-clock
measurements are integers. `saved_test_wall_ms` is an integer only if the
input, toolchain, and contract digests form an exact pair; otherwise it is the
literal `UNKNOWN`.

Generated artifacts are written outside the repository. `artifact files/bytes`
in `report.json` counts the generated evidence files other than
`manifest.json`; the manifest records every file and its SHA-256 digest.
