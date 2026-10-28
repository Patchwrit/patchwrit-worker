# Phase One — Independent Worker Review

**Status:** Initial code-reading review, not a completed audit. **Date:** 2026-10-27. **Branch reviewed:** main. No tests or live Fly Machines executed by reviewer.

## Scope
Read: `cmd/patchwrit-worker/main.go`, `internal/worker/{worker.go,deliver.go,worker_test.go}`, `internal/checkout/{checkout.go,checkout_test.go}`, `internal/contract/result.go`, both JDK Dockerfiles, README, worker analysis contract, and API handoff/readiness documents. Maven resolver implementation and full CI still need inspection. Live scan counts in handoff are historical claims, not reproduced here.

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

## Follow-up verification before final review
1. Read Maven resolver and tests, including subprocess env, timeout, scopes, omitted nodes, multi-module support and error handling.
2. Check worker runtime privileges, egress restrictions, clone size/disk limits, process termination.
3. Confirm 256 KB callback payload bound against large dependency trees and API behavior.
4. Check CI and independently run local tests if execution access becomes available.
5. Validate each finding with Muse Code before implementation.

**Phase Two:** intentionally not planned yet; discuss after Phase One review.
