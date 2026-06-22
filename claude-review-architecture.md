# Architecture Review

## Executive Summary

`depstate` is a small, focused library with a clear purpose and a reasonable
generic design. The core state-tracking logic is sound and concurrency-safe for
the data layer (`sync.Map`, `atomic.Value`). The main weaknesses are in the
testing layer — test helper code ships in the production binary, testify
assertions are almost entirely absent despite being mandated by CLAUDE.md,
and `time.Sleep`-based synchronisation is endemic — and in a confirmed
goroutine/subscription leak in `Chan()`. There are also a handful of
medium-priority correctness gaps (out-of-order state notifications, full-timeout
waits on missed transitions) that affect the library's reliability guarantees.

---

## Findings

### 1. `test_helpers.go` compiles into the production binary
- **File(s):** `test_helpers.go`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** High
- **Description:** `test_helpers.go` carries the `package depstate` declaration and
  is not suffixed `_test.go`, so the Go toolchain compiles `testDep`,
  `testIDStateFunc`, and `setupTest` into every binary that imports this library.
  Test types and helpers have no place in production code.
- **Recommended fix:** Rename to `test_helpers_test.go`. Also remove or restore the
  three large commented-out helper functions (lines 60–96); if they were removed
  intentionally, delete them; otherwise restore them.

---

### 2. Tests bypass testify in violation of project policy
- **File(s):** `dependency_state_test.go`, `dependency_state_features_test.go`,
  `dependency_state_helpers_test.go`, `dependency_state_race_test.go`,
  `test_helpers.go`
- **Dimension(s):** Inconsistencies, Maintainability
- **Priority:** High
- **Description:** CLAUDE.md mandates `github.com/stretchr/testify/assert` and
  `require` for all test assertions. The entire test suite uses bare `t.Errorf`,
  `t.Fatalf`, and manual `if` comparisons instead. Examples:
  - `assertStateEquals` (`test_helpers.go:49`) calls `t.Fatalf` and `t.Errorf`
    directly instead of `require.NoError` / `assert.Equal`.
  - `TestSet`, `TestChan`, `TestIsDependencyMet`, `TestWaitForAny`, and all race
    tests follow the same pattern.
  The failure messages produced are weaker, and the inconsistency makes the test
  code harder to extend correctly.
- **Recommended fix:** Add `testify` to `go.mod`, then replace every bare
  `t.Errorf`/`t.Fatalf` with the equivalent `assert.*`/`require.*` call. Change
  `assertStateEquals` to use `require.NoError` and `assert.Equal`.

---

### 3. `Chan()` leaks a pubsub subscription and a goroutine on every call ✓ Resolved
- **File(s):** `dependency_state.go:275`
- **Dimension(s):** Correctness, Architecture
- **Priority:** High
- **Description:** `Chan()` calls `d.transitions.Subscribe()`, which registers a
  new `chan any` in the topic's internal subscription list, then wraps it with
  `pubsub.CastChan[State]`. `CastChan` spawns a goroutine to forward typed values
  from the raw channel to a new `<-chan State`. The raw channel is never exposed
  to the caller, so there is no way to call `d.transitions.Unsubscribe(rawChan)`.
  Both the goroutine and the registered subscription accumulate for the lifetime of
  the `dependencyState` struct. By contrast, `WaitUntilState` and `WaitForAny`
  correctly `defer d.transitions.Unsubscribe(rawChan)`.
- **Recommended fix:** Return the raw channel alongside the typed channel, or wrap
  the call in a helper that defers unsubscription when the caller closes/stops
  consuming. The cleanest API change is to add a `context.Context` parameter to
  `Chan()` and close the subscription when the context is cancelled:
  ```go
  func (d *dependencyState[T]) Chan(ctx context.Context) <-chan State {
      rawChan := d.transitions.Subscribe()
      typedChan := pubsub.CastChan[State](rawChan)
      go func() {
          <-ctx.Done()
          d.transitions.Unsubscribe(rawChan)
      }()
      return typedChan
  }
  ```
- **Status:** Resolved in 3325e4d
- **Complexity delta:** Chan: 1 → 1 (unchanged)

---

### 4. `defer ctx.Done()` in benchmarks is a no-op
- **File(s):** `benchmark_test.go:50,64,80,95,107`
- **Dimension(s):** Correctness
- **Priority:** High
- **Description:** Every benchmark calls `defer ctx.Done()` expecting to cancel a
  context at cleanup time. `ctx.Done()` returns a channel (`<-chan struct{}`), it
  does not cancel the context. `defer` on a channel receive expression is legal Go
  but does nothing useful here: the deferred call evaluates the channel at setup
  time and then at defer-run time does nothing with it. The `context.Background()`
  passed to `NewDependencyState` is never cancelled, so the background goroutines
  (the `processEvents` and `forwardStateUpdates` loops) leak for the duration of
  the test binary.
- **Recommended fix:** Replace `context.Background()` with
  `context.WithCancel(context.Background())` and defer `cancel()`:
  ```go
  ctx, cancel := context.WithCancel(context.Background())
  b.Cleanup(cancel)
  ```

---

### 5. `WaitUntilState` and `WaitForAny` miss transitions between check and subscribe ✓ Resolved
- **File(s):** `dependency_state.go:282,355-370`
- **Dimension(s):** Correctness
- **Priority:** Medium
- **Description:** Both methods check current state *before* subscribing to the
  transitions topic. If the desired state is reached in the window between the
  initial check and the `d.transitions.Subscribe()` call, the notification is
  missed. Both methods recover on context cancellation (they re-check current state
  at that point), but not before: the caller waits the full timeout even though the
  condition was already satisfied. In `WaitForAny`, a dependency can transition to
  the desired state and back within that window with no observable effect on the
  result.
- **Recommended fix:** Subscribe first, then check current state, then enter the
  wait loop:
  ```go
  rawChan := d.transitions.Subscribe()
  defer d.transitions.Unsubscribe(rawChan)
  stateChan := pubsub.CastChan[State](rawChan)

  if d.CurrentState() == expectedState { // check after subscribing
      return nil
  }
  // ... wait loop
  ```
- **Status:** Resolved in a169449
- **Complexity delta:** waitForStateWithTimeout: N/A → 1; WaitForAny: 23 → 23 (unchanged)

---

### 6. `time.Sleep` used for synchronisation throughout tests and benchmarks
- **File(s):** `dependency_state_features_test.go:48,94`, `dependency_state_helpers_test.go:30,44,93`,
  `dependency_state_race_test.go:48,135,226`, `benchmark_test.go:175,208`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Medium
- **Description:** Multiple tests call `time.Sleep` to allow asynchronous state
  changes to propagate before asserting. This violates the "Deterministic" and
  "Fast Feedback" desiderata from CLAUDE.md and is the most common source of
  flaky tests in Go. On a loaded CI machine or a slow environment, 10 ms or 50 ms
  sleeps will be insufficient.
- **Recommended fix:** Replace sleeps with `assertStateEquals` (which already calls
  `WaitUntilState`), or wait on the channel returned by `NewDependencyState` using
  a `select` with a timeout. The existing `WaitUntilState` / `WaitForDependencies`
  methods are exactly the right synchronisation primitives.

---

### 7. No tests call `t.Parallel()`
- **File(s):** All `*_test.go` files
- **Dimension(s):** Maintainability
- **Priority:** Medium
- **Description:** CLAUDE.md states: "call `t.Parallel()` in tests that are safe
  to run concurrently. Design tests to be parallelisable by default." None of the
  unit tests call `t.Parallel()`. All tests use independent state created in setup
  functions, so most are safe to parallelise without changes.
- **Recommended fix:** Add `t.Parallel()` as the first line of each top-level
  `Test*` function. Tests that share a `topic` or `ds` created in a closure are
  already independent; they just need the declaration.

---

### 8. Concurrent `assessState` calls can publish stale or out-of-order state ✓ Resolved
- **File(s):** `dependency_state.go:245-253`
- **Dimension(s):** Correctness
- **Priority:** Medium
- **Description:** `assessState` is not atomic: it reads `calculateState()`,
  then reads `currentState`, then conditionally writes `currentState` and
  publishes. Two goroutines calling `assessState` simultaneously can each read
  the same old `currentState`, then race to store. The slower goroutine stores
  a state calculated from an earlier snapshot of `sync.Map`, potentially
  publishing a stale transition *after* the correct one. For a library whose
  primary contract is correct state notification, out-of-order publications
  undermine that contract.
- **Recommended fix:** Guard the check-store-publish sequence with a mutex,
  or use a `compare-and-swap` on the atomic value:
  ```go
  func (d *dependencyState[T]) assessState() {
      newState := d.calculateState()
      for {
          current := d.currentState.Load().(State)
          if current == newState {
              return
          }
          if d.currentState.CompareAndSwap(current, newState) {
              d.publishState(newState)
              return
          }
      }
  }
  ```
  Note: `atomic.Value` does not expose `CompareAndSwap` before Go 1.17; use a
  `sync.Mutex` wrapping the store+publish if a simpler solution is preferred.
- **Status:** Resolved in 967efbe
- **Complexity delta:** assessState: 1 → 5

---

### 9. `WaitForAny` with an empty ID list silently times out ✓ Resolved
- **File(s):** `dependency_state.go:355`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Low
- **Description:** When called with `ids = []string{}`, `WaitForAny` subscribes
  to the transitions topic and blocks for the full timeout, then returns
  `("", context.DeadlineExceeded)`. The caller receives no indication that the
  input was invalid. The test at `dependency_state_helpers_test.go:115` even
  relies on this timeout-as-validation, which wastes 50 ms per test run.
- **Recommended fix:** Return a sentinel error immediately:
  ```go
  if len(ids) == 0 {
      return "", errors.New("WaitForAny requires at least one dependency ID")
  }
  ```
- **Status:** Resolved in 98da526 (also fixes pre-existing flaky TestContextCancellation)
- **Complexity delta:** WaitForAny: 23 → 24

---

### 10. Dead commented-out code in `test_helpers.go`
- **File(s):** `test_helpers.go:60-96`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Description:** Three functions (`waitForStateChange`, `publishDependencyState`,
  `setupDependencies`) are fully commented out. There is no TODO or explanation.
  Dead code increases reading overhead and signals an incomplete or abandoned
  refactor.
- **Recommended fix:** Delete the commented blocks. The git history preserves them
  if they are ever needed again.

---

### 11. `calculateState` returns `DependenciesMet` for zero tracked dependencies ✓ Resolved
- **File(s):** `dependency_state.go:256-266`
- **Dimension(s):** Correctness
- **Priority:** Low
- **Description:** When the dependency map is empty (either initially or after
  `Remove` empties it), `calculateState` returns `DependenciesMet` (vacuous
  truth). The initial stored state is `DependenciesUnknown`, so on the first
  `Add` followed by `Remove`-of-all, the state machine transitions
  `Unknown → NotMet → Met`. Callers who call `Remove` on the last dependency
  may be surprised to receive a `DependenciesMet` signal.
- **Recommended fix:** Decide the intended semantic and document it explicitly.
  If "no dependencies" should mean `DependenciesUnknown`, add a guard:
  ```go
  func (d *dependencyState[T]) calculateState() State {
      empty := true
      newState := DependenciesMet
      d.dependencies.Range(func(key, value any) bool {
          empty = false
          if value.(State) != d.desiredState {
              newState = DependenciesNotMet
              return false
          }
          return true
      })
      if empty {
          return DependenciesUnknown
      }
      return newState
  }
  ```
- **Status:** Resolved in 4ecbef2
- **Complexity delta:** calculateState: 2 → 3

---

### 12. Magic 100 ms timeout in `waitForEventProcessor` ✓ Resolved
- **File(s):** `dependency_state.go:173`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Description:** `waitForEventProcessor` waits up to `100 * time.Millisecond`
  for the inner `processEvents` goroutine to exit. This value is unexplained and
  not configurable. In practice the goroutine exits almost immediately after the
  context is cancelled, but the timeout silently hides cases where it doesn't.
- **Recommended fix:** Add a comment explaining the choice, or surface a logged
  warning when the timeout fires (e.g., log the goroutine is taking longer than
  expected, to aid debugging).
- **Status:** Resolved in 0554add
- **Complexity delta:** waitForEventProcessor: 1 → 1 (unchanged)

---

### 13. `set` internal method uses a boolean `assess` parameter ✓ Resolved
- **File(s):** `dependency_state.go:97-102`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Description:** The unexported `set(id, state, assess bool)` is called with
  `assess=false` only in `Add` to batch the assessment. Boolean control-flow
  parameters obscure intent and make call sites harder to read at a glance.
- **Recommended fix:** Inline the `d.dependencies.Store(id, state)` calls directly
  in `Add` and drop the `assess` parameter from `set`, or rename the method to
  `storeState` to clarify it never assesses.
- **Status:** Resolved in 6a1dd59
- **Complexity delta:** Set: 1 → 0 (inlined); Add: 1 → 1 (unchanged)

---

## Priority Table

| Priority | Title | File(s) |
|----------|-------|---------|
| High | `test_helpers.go` compiles into production binary | `test_helpers.go` |
| High | Tests bypass testify in violation of project policy | All `*_test.go` files |
| High | `Chan()` leaks a pubsub subscription and goroutine | `dependency_state.go:275` |
| High | `defer ctx.Done()` in benchmarks is a no-op | `benchmark_test.go:50,64,80,95,107` |
| Medium | `WaitUntilState`/`WaitForAny` miss transitions between check and subscribe | `dependency_state.go:282,355` |
| Medium | `time.Sleep` used for synchronisation in tests | Multiple `*_test.go` files |
| Medium | No tests call `t.Parallel()` | All `*_test.go` files |
| Medium | Concurrent `assessState` can publish stale/out-of-order state | `dependency_state.go:245` |
| Low | `WaitForAny` with empty ID list silently times out | `dependency_state.go:355` |
| Low | Dead commented-out code in `test_helpers.go` | `test_helpers.go:60-96` |
| Low | `calculateState` returns `DependenciesMet` for zero dependencies | `dependency_state.go:256` |
| Low | Magic 100 ms timeout in `waitForEventProcessor` | `dependency_state.go:173` |
| Low | `set` internal method uses a boolean `assess` parameter | `dependency_state.go:97` |
