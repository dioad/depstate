---
name: architecture-review
description: >
  Activate when the user asks for a comprehensive architecture review of the
  codebase, or wants to systematically address findings from a prior review.
  Generates a ranked findings document (claude-review-architecture.md) or
  works through existing findings one commit at a time.
---

# Architecture Review

A two-phase skill for reviewing and improving codebase architecture. Phase 1
generates a ranked findings document; Phase 2 addresses findings
systematically, one commit at a time.

## When to activate

- User asks for an "architecture review" or "code review"
- User asks to "review the codebase" or "find architectural issues"
- User asks to "address findings" from a prior architecture review
- User references `claude-review-architecture.md`
- User says "architecture-review review" or "architecture-review address"

## Determine the phase

Examine the user's request to determine which phase to run:

- **Phase 1** — the user wants a new review generated. Keywords: "review",
  "analyse", "generate findings", "what's wrong with", "architecture-review review".
- **Phase 2** — the user wants to act on existing findings. Keywords: "address",
  "fix findings", "work through", "architecture-review address".

If unclear, ask: "Do you want to generate a new architecture review, or address
findings from an existing one?"

---

## Phase 1: Generate Review

Perform a comprehensive architecture and engineering review of the current
codebase and write the output to `claude-review-architecture.md`.

### Review dimensions

Analyse the codebase across these five dimensions:

- **Inconsistencies** -- mismatched patterns, duplicate logic, divergent
  conventions across packages or files
- **Correctness** -- bugs, race conditions, error handling gaps, incorrect
  assumptions
- **Maintainability** -- coupling, cohesion, complexity, testability
- **Modern practices** -- alignment with current language idioms and community
  standards
- **Architecture** -- apply hexagonal architecture thinking to identify areas
  of high coupling and candidates for restructuring

### Execution steps

1. Read the project's entry points, core packages, and test files to build a
   mental model of the system.
2. Analyse each dimension in turn. For each finding, record:
   - Which file(s) are affected
   - Which dimension(s) it falls under
   - A clear description of the problem
   - A concrete recommended fix
   - A priority rating: **High**, **Medium**, or **Low**
3. Write all findings to `claude-review-architecture.md` using the output
   format below.
4. After writing, report the total finding count and top three High-priority
   items to the user.

### Output format

Write `claude-review-architecture.md` with this structure:

```
# Architecture Review

## Executive Summary
<2-4 sentences: overall health, most critical theme, recommended focus>

## Findings

### <Finding Title>
- **File(s):** <path(s)>
- **Dimension(s):** <one or more of: Inconsistencies, Correctness, Maintainability, Modern Practices, Architecture>
- **Priority:** High | Medium | Low
- **Description:** <what the problem is and why it matters>
- **Recommended fix:** <concrete, actionable steps>

... (repeat for each finding)

## Priority Table

| Priority | Title | File(s) |
|----------|-------|---------|
| High     | ...   | ...     |
| Medium   | ...   | ...     |
| Low      | ...   | ...     |
```

---

## Phase 2: Address Findings

Work through findings in `claude-review-architecture.md` one at a time. Each
finding gets exactly one conventional commit.

### Scope

If the user specifies a round label (e.g. "Round-2"), process only findings
tagged with that label. Otherwise process all unresolved findings in priority
order (High first).

### Per-finding workflow

For each finding:

1. **Plan** -- read the finding and identify the minimal correct fix.

2. **Baseline complexity** -- before touching any file, record its cognitive
   complexity:
   ```bash
   gocognit <file>
   ```

3. **Fix** -- implement the change.
   - If the correct fix belongs in an upstream `github.com/dioad` repository,
     create a GitHub issue instead of a local change:
     ```bash
     gh issue create --repo github.com/dioad/<repo> --title "..." --body "..."
     ```

4. **Verify** -- run all pre-completion checks before committing:
   ```bash
   go fix ./...
   go fmt ./...
   go vet ./...
   go test -race ./...
   go build .
   ```
   Do not proceed to the commit step if any check fails.

5. **Post-fix complexity** -- record cognitive complexity after the fix:
   ```bash
   gocognit <file>
   ```
   The complexity score must stay the same or decrease. If it increases,
   rethink the approach.

6. **Commit** -- one conventional commit per finding:
   ```
   fix: <short description matching the finding title>
   ```

7. **Update document** -- in `claude-review-architecture.md`, mark the finding
   resolved and record the commit SHA and complexity delta:
   ```
   - **Status:** Resolved in <sha>
   - **Complexity delta:** <before> -> <after>
   ```

### Constraints

- Do not batch multiple unrelated findings into a single commit.
- Do not skip the pre-completion checks step.
- Do not let complexity increase after a fix.
- Do not push to remote; committing locally is sufficient.
