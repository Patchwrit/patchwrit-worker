# Patchwrit Worker — Muse Code Handoff

**Updated:** 2026-10-26. **Status:** No Go implementation verified.

Related control-plane repo: https://github.com/Patchwrit/patchwrit-api

Read the API repo's `docs/PROJECT.md`, `docs/ARCHITECTURE.md`, `docs/HANDOFF.md` and this repo's `docs/WORKER_CONTRACT.md` before coding. Confirm current Git branches and files first.

## Next task

Coordinate with Phoenix's Milestone 1 (Fly create/inspect/destroy). Do not race ahead into Maven or agents.

- Initialize a minimal Go module and `cmd/patchwrit-worker` entrypoint.
- Emit bounded structured JSON with worker version, outcome, and basic environment details.
- Ensure deterministic process exit, errors on stderr without leaking secrets, and simple Go unit tests.
- Provide a minimal multi-stage Dockerfile pinned to verified base image versions (or record reason for temporary unpinned dev image).
- Smoke test locally before publishing a worker image.
- Agree on the exact job request/response contract with the API repo before adding callback logic.
- Keep worker free from Fly control-plane credentials and broad GitHub tokens.
- Once running on Fly, measure creation-to-start, execution and destroy time; do not invent benchmarks.

## End-of-session update format

Write: timestamp; branch and SHA; implemented files; commands/tests and actual results; environment and issues; what remains unimplemented; exact next step. Commit updates with code. Protect secrets and never claim an unverified feature is finished.
