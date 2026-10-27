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

## Images

Images are published to GHCR for `linux/amd64`; Phoenix deploys them by digest, never by floating tag.

| Variant | Dockerfile | Contents |
| --- | --- | --- |
| `smoke` | `Dockerfile` | Go worker only |
| `jdk17` | `Dockerfile.jdk17` | Go worker, Git, JDK 17, Maven 3.9.11 |

Build and smoke-test the JDK 17 image locally:

```bash
docker buildx build --platform linux/amd64 -f Dockerfile.jdk17 -t patchwrit-worker:jdk17 --load .
docker run --rm patchwrit-worker:jdk17
```

The JDK 17 run should report non-null Git, Java, and Maven versions. Repository cloning and Maven dependency resolution are not implemented yet.

The binary reports detected git/Java/Maven versions (null when absent) and never receives Fly control-plane or GitHub write credentials. Maven dependency resolution, OSV scanning, and source investigation follow in later milestones.
