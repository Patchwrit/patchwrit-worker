# Patchwrit Worker

Ephemeral Go execution worker for [Patchwrit API](https://github.com/Patchwrit/patchwrit-api).

**Current status:** W2 smoke binary implemented. The Go worker executes constrained, isolated tasks inside disposable Fly.io Machines. See the API repository's `docs/ARCHITECTURE.md` and `docs/HANDOFF.md` for the broader plan, and `docs/ANALYSIS_CONTRACT.md` in this repo for the worker's contract.

## Smoke binary

```bash
go run ./cmd/patchwrit-worker   # prints the schema_version 1 result JSON, exits 0
go test ./...
docker build -t patchwrit-worker:smoke .
docker run --rm patchwrit-worker:smoke
```

The binary reports detected git/Java/Maven versions (null when absent) and never receives Fly control-plane or GitHub write credentials. Maven dependency resolution, OSV scanning, and source investigation follow in later milestones.
