# Architecture Review: `github.com/dioad/depstate`

_Reviewed: 2026-06-18 — branch `master`_

---

## Executive Summary

`depstate` is a small, focused library with a clear purpose and a reasonable generic design. The core state-tracking logic is sound and concurrency-safe for the data layer (`sync.Map`, `atomic.Value`). One finding remains open: `time.Sleep` used for synchronisation throughout tests and benchmarks, which risks flakiness on loaded CI machines. The test-suite and production-boundary issues raised by this review — `test_helpers.go` compiling into the binary, tests bypassing testify, a no-op `defer ctx.Done()` in benchmarks, missing `t.Parallel()`, and dead commented-out code — have all been resolved; see [resolved findings](./claude-review-architecture-resolved.md).

---

## Findings

### 4. `time.Sleep` used for synchronisation throughout tests and benchmarks

- **File(s):** `dependency_state_features_test.go:48,94`, `dependency_state_helpers_test.go:30,44,93`, `dependency_state_race_test.go:48,135,226`, `benchmark_test.go:175,208`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Medium
- **Status:** Open
- **Description:** Multiple tests call `time.Sleep` to allow asynchronous state changes to propagate before asserting. On a loaded CI machine, 10 ms or 50 ms sleeps will be insufficient, causing flakiness.
- **Recommended fix:** Replace sleeps with `assertStateEquals` (which already calls `WaitUntilState`), or wait on the channel returned by `NewDependencyState` using a `select` with a timeout. The existing `WaitUntilState`/`WaitForDependencies` methods are the right synchronisation primitives.

---

## Priority Table

| # | Priority | Status | Finding | File(s) |
|---|----------|--------|---------|---------|
| 4 | Medium | Open | `time.Sleep` used for synchronisation in tests | Multiple `*_test.go` files |
