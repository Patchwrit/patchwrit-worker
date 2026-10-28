# Phase One — Independent Worker Review

**Status:** Second-pass static code review; findings ready for discussion, not a completed security audit. **Date:** 2026-10-27. **Branch reviewed:** main. No tests or live Fly Machines executed by reviewer.

## Scope
Read: `cmd/patchwrit-worker/main.go`, `internal/worker/{worker.go,deliver.go,worker_test.go}`, `internal/checkout/{checkout.go,checkout_test.go}`, `internal/contract/result.go`, both JDK Dockerfiles, README, worker analysis contract, and API handoff/readiness documents. Also reviewed `internal/maven/{graph.go,graph_test.go}`, `internal/worker/deliver_test.go`, `.github/workflows/image.yml`, `Dockerfile`, and worker handoff/contract docs. Live scan counts in handoff are historical claims, not reproduced here.

## Observed strengths
- Phoenix/Go separation and digest-pinned runtime images.
- Exact pinned-SHA checkout verification.
- Structured worker contract, error categories, dependency introduction paths, timings.
- Checkout tests cover success and wrong SHA.

## Findings

### P1-W001 — Git subprocess inherits callback credentials — HIGH
**File:** `internal/checkout/checkout.go` (`gitOutput`).
`cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")` inherits `PATCHWRIT_CALLBACK_TOKEN` and other environment variables. Git may invoke configured helpers or filters; Maven environment scrubbing does not cover this path. **Action:** allowlist Git subprocess environment, block unexpected Git helpers/config and protocols as feasible, test that secrets cannot reach subprocesses. This is an exposure risk, not evidence of compromise.

### P1-W002 — Silent POM inventory truncation — MEDIUM
**File:** `internal/checkout/checkout.go` (`findPOMs`).
At 100 POMs, `filepath.SkipAll` silently stops enumeration. **Action:** report truncation explicitly or return a classified limit error; test 100+ POMs. Do not represent incomplete inventories as complete.

### P1-W003 — Module identity/multiple provenance paths need verification — MEDIUM (design concern)
**File:** `internal/contract/result.go` (`Dependency`).
Dependency records carry one `introduced_via` path but no explicit module identifier. Multi-module and multiple-path interpretations may be ambiguous. **Action:** inspect Maven resolver before changing schema; consider module IDs, stable node IDs, and multiple introduction paths.

### P1-W004 — Callback transport failure loses otherwise successful result — MEDIUM
**Files:** `internal/worker/deliver.go`, `cmd/patchwrit-worker/main.go`.
Single callback POST with 15-second timeout; transient failure exits nonzero. **Action:** assess bounded retries for transport/429/5xx with idempotent server callback; test temporary outage and cleanup.

### P1-W005 — Stale implementation status in documentation — LOW
**Files:** worker README, analysis contract, and comments.
Some text still claims clone/Maven functionality is unimplemented despite current code and API handoff. **Action:** reconcile docs with current code after review.

### P1-W006 — Dependency parser silently truncates at 2,000 — HIGH
**File:** `internal/maven/graph.go` (`ParseTree`).
When `len(deps) >= maxDependencies`, traversal returns without error, warning, or completeness metadata. A completed scan can therefore omit dependencies. **Action:** return an explicit limit error or completeness flag; test a tree with 2,001+ nodes and ensure API does not label a partial result complete.

### P1-W007 — Reactor/multi-module dependency coverage is not explicitly preserved — HIGH (coverage concern)
**File:** `internal/maven/graph.go` (`Resolve`).
A single Maven invocation writes one `dependency-tree.json` and parses it as one root. The worker does not explicitly enumerate each module's effective tree or preserve module identity. Maven reactor behavior and output-file semantics need integration verification before claiming all modules are covered. **Action:** add a two-module integration fixture with different vulnerable transitive dependencies; capture per-module graph, module provenance and completeness. This does not assert Maven cannot handle reactors.

### P1-W008 — Image publication workflow does not run tests — MEDIUM
**File:** `.github/workflows/image.yml`.
The workflow builds and publishes three images on pushes to `main`, with no `go test ./...` or static checks before publishing. **Action:** add a test/check job that gates publishing, plus PR checks.

### P1-W009 — Callback payload size not enforced client-side — MEDIUM
**Files:** `internal/worker/deliver.go`, `internal/contract/result.go`.
`Deliver` marshals and sends the complete result without checking the documented 256 KB limit. Dependency paths and 2,000 nodes may exceed it, causing otherwise successful analyses to fail at callback. **Action:** establish a shared payload limit and explicit oversize error/partial-result behavior; add a realistic large-tree serialization test. Verify API's actual enforcement separately.

### P1-W010 — Untrusted worker process is root by default; resource isolation is mostly external — HIGH (deployment risk)
**Files:** `Dockerfile.jdk17`, `Dockerfile.jdk21`, `internal/maven/graph.go`, `internal/checkout/checkout.go`.
No `USER` directive in the runtime Dockerfiles. Maven is an execution surface for repository-controlled plugins/extensions, even with a scrubbed environment and isolated HOME. `git clone` has no explicit byte-size bound and `exec.CommandContext` only directly manages the launched process, not necessarily all descendants. **Action:** verify Fly Machine isolation, egress and resource caps; run as a non-root user, constrain Git checkout/network and subprocess trees, and test a malicious POM/plugin. This is not a claim of host escape.

### P1-W011 — Maven subprocess secret filtering is denylist-based — MEDIUM (hardening)
**File:** `internal/maven/graph.go` (`ScrubbedEnv`).
Filtering keys containing TOKEN/SECRET/PASSWORD and FLY_/PATCHWRIT_CALLBACK prefixes is a useful safeguard but can miss other sensitive variables (e.g. provider-specific API keys, credential-file paths). **Action:** construct a minimal allowlisted environment with only required PATH, HOME, Java/Maven configuration and safe network settings; add tests for arbitrary secret names.

## Follow-up verification before final review
1. Run `go test ./...` and representative Maven integration fixtures in a suitable execution environment; no tests were executed in this review.
2. Verify actual Fly Machine configuration, network egress, memory/disk caps, and crash cleanup in the API repo.
3. Verify API callback payload limit, idempotency and large-result handling.
4. Discuss findings and priority with Muse Code, including actual Maven reactor output and provenance semantics.
5. Review and adjust severity after runtime verification; do not merge this draft until agreed.

**Phase Two:** intentionally not planned yet; discuss after Phase One review.
