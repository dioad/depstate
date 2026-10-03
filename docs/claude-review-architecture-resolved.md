# Architecture Review: Resolved Findings — `github.com/dioad/depstate`

_Reviewed: 2026-06-18 — See [open findings](./claude-review-architecture.md)_

---

## Resolved Findings

### 1. `Chan()` leaks a pubsub subscription and a goroutine on every call ✅ Resolved

- **File(s):** `dependency_state.go:275`
- **Dimension(s):** Correctness, Architecture
- **Priority:** High
- **Resolved in:** 3325e4d
- **Description:** `Chan()` called `d.transitions.Subscribe()` but never exposed the raw channel to the caller, so there was no way to call `d.transitions.Unsubscribe(rawChan)`. Both the goroutine and the registered subscription accumulated for the lifetime of the `dependencyState` struct.
- **Outcome:** Added a `context.Context` parameter to `Chan()`. A goroutine now watches `ctx.Done()` and calls `d.transitions.Unsubscribe(rawChan)` when the context is cancelled. Complexity delta: `Chan` 1→1 (unchanged).

---

### 2. `WaitUntilState` and `WaitForAny` miss transitions between check and subscribe ✅ Resolved

- **File(s):** `dependency_state.go:282,355-370`
- **Dimension(s):** Correctness
- **Priority:** Medium
- **Resolved in:** a169449
- **Description:** Both methods checked current state before subscribing to the transitions topic. If the desired state was reached between the initial check and the `Subscribe()` call, the notification was missed and the caller waited the full timeout.
- **Outcome:** Subscribe first, then check current state, then enter the wait loop. Complexity delta: `waitForStateWithTimeout` N/A→1; `WaitForAny` 23→23 (unchanged).

---

### 3. Concurrent `assessState` calls can publish stale or out-of-order state ✅ Resolved

- **File(s):** `dependency_state.go:245-253`
- **Dimension(s):** Correctness
- **Priority:** Medium
- **Resolved in:** 967efbe
- **Description:** `assessState` was not atomic: two goroutines could each read the same old `currentState`, race to store, and the slower goroutine could publish a stale transition after the correct one, undermining the library's state-notification contract.
- **Outcome:** Used compare-and-swap on `atomic.Value` to guard the check-store-publish sequence. Only one goroutine successfully transitions state; others loop until the new state is stable. Complexity delta: `assessState` 1→5.

---

### 4. `WaitForAny` with an empty ID list silently times out ✅ Resolved

- **File(s):** `dependency_state.go:355`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** Low
- **Resolved in:** 98da526
- **Description:** Called with `ids = []string{}`, `WaitForAny` subscribed and blocked for the full timeout, returning `("", context.DeadlineExceeded)` with no indication the input was invalid.
- **Outcome:** Return a sentinel error immediately when `len(ids) == 0`. Also fixed a pre-existing flaky `TestContextCancellation`. Complexity delta: `WaitForAny` 23→24.

---

### 5. `calculateState` returns `DependenciesMet` for zero tracked dependencies ✅ Resolved

- **File(s):** `dependency_state.go:256-266`
- **Dimension(s):** Correctness
- **Priority:** Low
- **Resolved in:** 4ecbef2
- **Description:** When the dependency map was empty, `calculateState` returned `DependenciesMet` (vacuous truth). Callers who called `Remove` on the last dependency received a surprising `DependenciesMet` signal.
- **Outcome:** Added an empty-map guard: when `Range` finds no entries, return `DependenciesUnknown`. Complexity delta: `calculateState` 2→3.

---

### 6. Magic 100 ms timeout in `waitForEventProcessor` ✅ Resolved

- **File(s):** `dependency_state.go:173`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Resolved in:** 0554add
- **Description:** `waitForEventProcessor` waited up to `100 * time.Millisecond` for the inner `processEvents` goroutine to exit. This value was unexplained and silently hid cases where the goroutine took longer.
- **Outcome:** Added a comment explaining the choice and surfaced a logged warning when the timeout fires. Complexity delta: `waitForEventProcessor` 1→1 (unchanged).

---

### 7. `set` internal method uses a boolean `assess` parameter ✅ Resolved

- **File(s):** `dependency_state.go:97-102`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Resolved in:** 6a1dd59
- **Description:** The unexported `set(id, state, assess bool)` used a boolean control-flow parameter, obscuring intent at call sites.
- **Outcome:** Inlined the `d.dependencies.Store(id, state)` calls directly in `Add` and dropped the `assess` parameter from `set`. Complexity delta: `Set` 1→0 (inlined); `Add` 1→1 (unchanged).

---

### 8. `test_helpers.go` compiles into the production binary ✅ Resolved

- **File(s):** `test_helpers.go` → `test_helpers_test.go`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** High
- **Resolved in:** 05a9221
- **Description:** `test_helpers.go` carried the `package depstate` declaration and was not suffixed `_test.go`, so the Go toolchain compiled `testDep`, `testIDStateFunc`, and `setupTest` into every binary that imports this library.
- **Outcome:** Renamed the file to `test_helpers_test.go` via `git mv`, preserving history. The commented-out dead code at the end of the file was left for finding 6, tracked separately. Complexity delta: `assertStateEquals` 2→2 (unchanged).

---

### 9. Tests bypass testify in violation of project policy ✅ Resolved

- **File(s):** `dependency_state_test.go`, `dependency_state_features_test.go`, `dependency_state_helpers_test.go`, `dependency_state_race_test.go`, `test_helpers_test.go`
- **Dimension(s):** Inconsistencies, Maintainability
- **Priority:** High
- **Resolved in:** 02608ae
- **Description:** CLAUDE.md mandates `github.com/stretchr/testify/assert` and `require` for all test assertions. The entire test suite used bare `t.Errorf`, `t.Fatalf`, and manual `if` comparisons instead.
- **Outcome:** Promoted `testify` to a direct `go.mod` dependency via `go mod tidy` and replaced all ~38 bare assertions with `assert.*`/`require.*` calls across the four test files and `assertStateEquals`. `example_test.go` was left untouched — it contains only `Example*` functions with no bare assertions. Complexity deltas all decreased or stayed flat: `TestChan` 3→1, `TestSet` 2→0, `TestGetDependencyStates` 4→0, `TestWaitForDependencies` 2→0, `TestWaitForDependenciesContextCancellation` 1→0, `TestIsDependencyMet` 7→0, `TestWaitForAny` 9→0, `TestNewDependencyStateWithBuffer` 2→0, `TestNewDependencyStateWithTopic` 2→0, `TestCalculateStateEmpty` 1→0, `TestRaceCondition` 11→9, `TestContextCancellation` 8→6, `TestConcurrentAccess` 8→8 (unchanged), `assertStateEquals` 2→0.

---

### 10. `defer ctx.Done()` in benchmarks is a no-op ✅ Resolved

- **File(s):** `benchmark_test.go`
- **Dimension(s):** Correctness
- **Priority:** High
- **Resolved in:** f945f35
- **Description:** Every benchmark called `defer ctx.Done()` expecting to cancel a context at cleanup time. `ctx.Done()` returns a channel and does not cancel anything, so `context.Background()` passed to `NewDependencyState` was never cancelled, leaking the `processEvents` and `forwardStateUpdates` goroutines for the life of the test binary.
- **Outcome:** Replaced `context.Background()` with `b.Context()` in `setupBenchmark`, which `testing.B` cancels automatically when the benchmark function returns — less code than the reviewed `context.WithCancel`/`b.Cleanup` recommendation and consistent with the file's existing use of `b.Context()` in `BenchmarkStateChange`. Removed the now-pointless `defer ctx.Done()` from all 8 benchmarks and dropped the now-unused `ctx` return value from the 6 benchmarks that never passed it to an API call. Complexity deltas unchanged across all affected functions.

---

### 11. No tests call `t.Parallel()` ✅ Resolved

- **File(s):** All `*_test.go` files
- **Dimension(s):** Maintainability
- **Priority:** Medium
- **Resolved in:** 5dac286
- **Description:** CLAUDE.md states tests safe to run concurrently should call `t.Parallel()`. None of the unit tests did.
- **Outcome:** Already resolved prior to this review cycle — `5dac286` ("test: run tests in parallel with t.Parallel") added `t.Parallel()` as the first line of all 17 top-level `Test*` functions across `dependency_state_test.go`, `dependency_state_features_test.go`, `dependency_state_helpers_test.go`, and `dependency_state_race_test.go`. The open findings doc was stale on this point; no new commit was needed.

---

### 12. Dead commented-out code in `test_helpers.go` ✅ Resolved

- **File(s):** `test_helpers_test.go:60-96` (originally `test_helpers.go`)
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Resolved in:** bd6ddda
- **Description:** Three functions (`waitForStateChange`, `publishDependencyState`, `setupDependencies`) were fully commented out with no TODO or explanation.
- **Outcome:** Deleted the commented blocks; git history preserves them if ever needed again. Complexity delta: N/A (no live code affected).
