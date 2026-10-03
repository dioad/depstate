# Architecture Review: `github.com/dioad/depstate`

_Reviewed: 2026-06-18 — branch `master`_

---

## Executive Summary

`depstate` is a small, focused library with a clear purpose and a reasonable generic design. The core state-tracking logic is sound and concurrency-safe for the data layer (`sync.Map`, `atomic.Value`). All six original findings are now resolved; see [resolved findings](./claude-review-architecture-resolved.md). One new finding was discovered while verifying finding 4's fix: a pre-existing flaky `Example_basic`, unrelated to any of the changes made so far.

---

## Findings

### 13. `Example_basic` is flaky: async print ordering races against the main goroutine's sleeps

- **File(s):** `example_test.go:15-76`
- **Dimension(s):** Correctness
- **Priority:** Medium
- **Status:** Open
- **Description:** `Example_basic` starts a goroutine that prints "Not all services are ready yet." / "All services are ready!" whenever `depChan` receives a transition, synchronised with the main goroutine only via two `time.Sleep(10 * time.Millisecond)` calls. The expected `// Output:` assumes the goroutine's first print lands strictly between the two "Making serviceN ready..." lines, but nothing guarantees that ordering. Reproduced failures show the expected print missing entirely from that slot (observed ~1 in 5 runs under `-race`, e.g. `go test -race -count=1 .`).
- **Root cause:** when `service1` alone becomes ready, the *overall* state does not change (still `DependenciesNotMet`, since `service2` isn't ready), so `assessState` does not publish a new transition at all — the "Not all services are ready yet." print the example expects at that point is actually the one-time transition from `Add` (`DependenciesUnknown` → `DependenciesNotMet`), whose delivery time relative to the two sleeps is unconstrained scheduling, not a causal consequence of `service1` becoming ready.
- **Recommended fix:** needs a design decision, not a mechanical sleep replacement — unlike the other sleeps fixed under finding 4, there is no transition to wait on after `service1` alone updates (none is published), so `WaitForAny`/`WaitUntilState` don't apply directly here. Two options: (a) drop the async consumer goroutine and print based on `ds.CurrentState()` synchronously in the main flow, which is deterministic but no longer demonstrates `depChan`; or (b) keep the channel demonstration but make the narrative only print on actual transitions (i.e. drop the "Not all services are ready yet." line, since it reflects the `Add`-triggered transition, not a `service1`-specific one). Flagged for a decision rather than fixed outright.

---

## Priority Table

| # | Priority | Status | Finding | File(s) |
|---|----------|--------|---------|---------|
| 13 | Medium | Open | `Example_basic` is flaky: async print ordering races against sleeps | `example_test.go` |
