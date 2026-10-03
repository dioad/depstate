# Architecture Review: `github.com/dioad/depstate`

_Reviewed: 2026-06-18 — branch `master`_

---

## Executive Summary

`depstate` is a small, focused library with a clear purpose and a reasonable generic design. The core state-tracking logic is sound and concurrency-safe for the data layer (`sync.Map`, `atomic.Value`). Six findings remain open. The most critical are `test_helpers.go` compiling into the production binary (High) and tests bypassing testify entirely despite it being mandated by CLAUDE.md (High). Four resolved findings addressed real correctness issues: a goroutine/subscription leak in `Chan()`, a subscribe-before-check race in `WaitUntilState`, a concurrent `assessState` stale-publication race, and an unintuitive `DependenciesMet` result for an empty dependency set.

---

## Findings

### 1. `test_helpers.go` compiles into the production binary

- **File(s):** `test_helpers.go`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** High
- **Status:** Open
- **Description:** `test_helpers.go` carries the `package depstate` declaration and is not suffixed `_test.go`, so the Go toolchain compiles `testDep`, `testIDStateFunc`, and `setupTest` into every binary that imports this library. Test types and helpers have no place in production code.
- **Recommended fix:** Rename to `test_helpers_test.go`. Also remove or restore the three large commented-out helper functions (lines 60–96); if they were removed intentionally, delete them; otherwise restore them.

---

### 2. Tests bypass testify in violation of project policy

- **File(s):** `dependency_state_test.go`, `dependency_state_features_test.go`, `dependency_state_helpers_test.go`, `dependency_state_race_test.go`, `test_helpers.go`
- **Dimension(s):** Inconsistencies, Maintainability
- **Priority:** High
- **Status:** Open
- **Description:** CLAUDE.md mandates `github.com/stretchr/testify/assert` and `require` for all test assertions. The entire test suite uses bare `t.Errorf`, `t.Fatalf`, and manual `if` comparisons instead. The failure messages produced are weaker, and the inconsistency makes the test code harder to extend correctly.
- **Recommended fix:** Add `testify` to `go.mod`, then replace every bare `t.Errorf`/`t.Fatalf` with the equivalent `assert.*`/`require.*` call. Change `assertStateEquals` to use `require.NoError` and `assert.Equal`.

---

### 3. `defer ctx.Done()` in benchmarks is a no-op

- **File(s):** `benchmark_test.go:50,64,80,95,107`
- **Dimension(s):** Correctness
- **Priority:** High
- **Status:** Open
- **Description:** Every benchmark calls `defer ctx.Done()` expecting to cancel a context at cleanup time. `ctx.Done()` returns a channel (`<-chan struct{}`), it does not cancel the context. The `context.Background()` passed to `NewDependencyState` is never cancelled, so background goroutines (`processEvents` and `forwardStateUpdates` loops) leak for the duration of the test binary.
- **Recommended fix:** Replace `context.Background()` with `context.WithCancel(context.Background())` and defer `cancel()` via `b.Cleanup(cancel)`.

---

### 4. `time.Sleep` used for synchronisation throughout tests and benchmarks

- **File(s):** `dependency_state_features_test.go:48,94`, `dependency_state_helpers_test.go:30,44,93`, `dependency_state_race_test.go:48,135,226`, `benchmark_test.go:175,208`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Medium
- **Status:** Open
- **Description:** Multiple tests call `time.Sleep` to allow asynchronous state changes to propagate before asserting. On a loaded CI machine, 10 ms or 50 ms sleeps will be insufficient, causing flakiness.
- **Recommended fix:** Replace sleeps with `assertStateEquals` (which already calls `WaitUntilState`), or wait on the channel returned by `NewDependencyState` using a `select` with a timeout. The existing `WaitUntilState`/`WaitForDependencies` methods are the right synchronisation primitives.

---

### 5. No tests call `t.Parallel()`

- **File(s):** All `*_test.go` files
- **Dimension(s):** Maintainability
- **Priority:** Medium
- **Status:** Open
- **Description:** CLAUDE.md states: "call `t.Parallel()` in tests that are safe to run concurrently. Design tests to be parallelisable by default." None of the unit tests call `t.Parallel()`. All tests use independent state created in setup functions, so most are safe to parallelise without changes.
- **Recommended fix:** Add `t.Parallel()` as the first line of each top-level `Test*` function.

---

### 6. Dead commented-out code in `test_helpers.go`

- **File(s):** `test_helpers.go:60-96`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Status:** Open
- **Description:** Three functions (`waitForStateChange`, `publishDependencyState`, `setupDependencies`) are fully commented out with no TODO or explanation. Dead code increases reading overhead and signals an incomplete or abandoned refactor.
- **Recommended fix:** Delete the commented blocks. Git history preserves them if ever needed again.

---

## Priority Table

| # | Priority | Status | Finding | File(s) |
|---|----------|--------|---------|---------|
| 1 | High | Open | `test_helpers.go` compiles into production binary | `test_helpers.go` |
| 2 | High | Open | Tests bypass testify in violation of project policy | All `*_test.go` files |
| 3 | High | Open | `defer ctx.Done()` in benchmarks is a no-op | `benchmark_test.go` |
| 4 | Medium | Open | `time.Sleep` used for synchronisation in tests | Multiple `*_test.go` files |
| 5 | Medium | Open | No tests call `t.Parallel()` | All `*_test.go` files |
| 6 | Low | Open | Dead commented-out code in `test_helpers.go` | `test_helpers.go:60-96` |
