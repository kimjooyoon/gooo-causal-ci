# Gooo causal CI

`gooo-causal-ci` is a small, read-only meta-program that generates CI work
from Gooo source binding, a released semantic graph, and claim evidence. It
does not select work from file-name patterns.

The repository is intentionally self-contained as an external consumer:

- `examples/causal-ci-policy/main.gooo` is the policy source authority;
- the workflow obtains the semantic IR/graph from the released Gooo CLI;
- the Go evaluator binds graph activities and claim evidence one-for-one to
  the fixed denominator in `contracts/`;
- a normal fixture observes a full test set and the generated selected set;
- stale graph, missing binding, digest mismatch, and an incorrectly excluded
  test are fail-closed as typed `UNKNOWN` or `REFUTED` evidence.

The program writes only to caller-owned output directories. It never edits the
input checkout. Local test, build, formatting, and vetting are deliberately
not part of the development workflow; GitHub Actions is the verification
authority.

See `docs/causal-ci-selection.md` for the contract, evidence model, and exact
paired-run rule for timing claims.
