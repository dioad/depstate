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
