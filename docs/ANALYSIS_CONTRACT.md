# Analysis Contract — Worker Mirror (W1)

**Status:** Agreed design (W1). No Go code implemented yet.

The canonical contract lives in the API repo:
`patchwrit-api/docs/ANALYSIS_CONTRACT.md`. This page is a short mirror for
worker development. If the two disagree, the API document wins.

## Worker's side of the contract

**Input** arrives as environment variables (never CLI args, never logged):
`PATCHWRIT_JOB_ID`, `PATCHWRIT_REPOSITORY_URL`, `PATCHWRIT_PINNED_SHA`
(Phoenix pins the ref before dispatch; the worker checks out exactly this
SHA), `PATCHWRIT_CALLBACK_URL`, `PATCHWRIT_CALLBACK_TOKEN` (job-scoped,
short-lived), `PATCHWRIT_TIMEOUT_SECONDS`, `PATCHWRIT_JDK`,
`PATCHWRIT_MAVEN`.

**Output** is one bounded JSON POST (max 256 KB) to the callback URL with
the token as a Bearer credential:

```json
{
  "schema_version": 1,
  "job_id": "analysis-uuid",
  "status": "completed",
  "worker_version": "0.1.0",
  "environment": {"go_version": "go1.26.2", "git_version": "2.x", "java_version": null, "maven_version": null},
  "timings_ms": {"execution": 0},
  "errors": []
}
```

Rules that matter to this repo:

- `status` is the worker job outcome (`completed | failed`), not the
  Analysis lifecycle. Phoenix maps between them.
- Errors are categorized (`toolchain_missing`, `clone_failed`,
  `invalid_pom`, `private_artifact`, `jdk_incompatible`, `network`,
  `timeout`, `internal`), never free-form only.
- Exit 0 after a successfully reported `failed` result. Non-zero exit
  means the worker crashed.
- The worker never receives Fly control-plane or GitHub write credentials
  and cannot delete its own Machine. Phoenix guarantees cleanup.

## Images

Smoke image first (worker binary only), then prebuilt `jdk17` / `jdk21`
variants with git + JDK + Maven 3.9.x, published to GHCR
(`ghcr.io/patchwrit/patchwrit-worker`). Phoenix selects an image per
analysis and pins its digest. Images are never built per request.

## Next step in this repo

W2: minimal Go module (`cmd/patchwrit-worker`) emitting the JSON above
locally, with unit tests and a pinned multi-stage Dockerfile.
