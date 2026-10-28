# Patchwrit Worker — Execution Contract (Proposed)

**Status:** Design only. No code has been verified implemented.

## Ownership boundary

**Phoenix / patchwrit-api** owns authorization, request validation, job state, resource policy, Fly Machines provisioning/deletion, AI reasoning, persistence, and retry/reconciliation.

**Go / patchwrit-worker** owns bounded execution inside a disposable Fly Machine: toolchain checks, cloning pinned public GitHub commits, Maven dependency resolution, evidence acquisition, and (later, explicitly authorized) remediation validation. It does not own Fly control-plane tokens or GitHub write credentials.

## Schema v2 (implemented)

The worker emits `schema_version: 2`. Dependencies carry `module`
(owning reactor module coordinate), `pom_path`, and
`introduction_paths` (all observed paths), and the result carries a
`graph` completeness block (`complete`, `modules_total`,
`modules_resolved`, `modules_failed`, `dependency_limit_hit`,
`pom_inventory_truncated`). Partial results are explicit, never silent.
Canonical contract: `docs/ANALYSIS_CONTRACT.md` in patchwrit-api.

## Phase 0: Smoke-test payload (provisional)

Launch Go worker with job inputs via configuration or scoped job payload; don't expose secrets in command-line arguments or logs.

Example output for local smoke test, not actual measurements:

```json
{
  "schema_version": 1,
  "job_id": "example-job-id",
  "status": "completed",
  "worker_version": "dev",
  "environment": {
    "go_version": "go1.x",
    "java_version": null,
    "maven_version": null
  },
  "timings_ms": {
    "execution": 0
  },
  "errors": []
}
```

Actual implementation must define and validate exact field names and status enums jointly with Phoenix before deployment. The worker sends structured JSON via a short-lived, job-scoped authenticated callback to a reachable control plane (or through an explicitly designed authenticated polling mechanism); localhost on a development laptop is not reachable from Fly.

## Initial milestones

1. A reproducibly built Go executable prints a JSON result and exits 0; unit test the serialization and exit behavior.
2. Package it in a pinned container image and run locally.
3. Phoenix creates a Fly Machine with that image; observe exit, callback/result, and Machine deletion.
4. Add `java -version`, `mvn -version`, `git --version` environment checks; report installed toolchain versions, not inferred project requirements.
5. Clone PetClinic at an immutable SHA, inspect POMs and approved Maven wrapper metadata to determine probable requirements.
6. Resolve transitive Maven dependencies, with resource/time/network limits, then query OSV and return evidence.

## Runtime policy

Prebuild approved JDK-specific images rather than building on every analysis. Start with JDK 17 and 21; expand 8/11/25 as required. Automatic project inference is best effort; user override remains available. A Java bytecode target does **not** uniquely determine a build JDK. Default to trusted Maven 3.9.x or approved wrapper version; never blindly execute a downloaded wrapper script.

## Untrusted execution safety

Maven configuration/plugins/extensions can execute untrusted code even during discovery. Never include Fly credentials, database secrets, or broadly scoped GitHub write tokens in the worker. Bound CPU, memory, disk, execution time, callback payload size, egress and network access where practical. Pin repository SHA, runtime image digest, and log provenance. Treat error categories separately (private artifact, Maven misconfiguration, toolchain incompatibility, network failure, timeout).

## Cleanup

The worker cannot be responsible for its own guaranteed VM deletion. Phoenix persists Machine IDs, tracks cleanup separately from analysis success, and reconciles orphaned Machines after errors or lost callbacks.

## Test inputs

- https://github.com/spring-petclinic/spring-framework-petclinic
- https://github.com/ryanmcmorrowsnyk/vulnerable-java-app

Both are test cases, not hardcoded only accepted repositories.
