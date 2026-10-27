# Patchwrit Worker

Ephemeral Go execution worker for [Patchwrit API](https://github.com/Patchwrit/patchwrit-api).

**Current status:** repository initialized; no implementation has been verified. The Go worker will execute constrained, isolated tasks inside disposable Fly.io Machines. See the API repository's `docs/ARCHITECTURE.md` and `docs/HANDOFF.md` for the broader plan.

The initial task is a minimal Go process that reports environment information and terminates. Maven dependency resolution, OSV scanning, and source investigation follow only after the Machine lifecycle is proven.
