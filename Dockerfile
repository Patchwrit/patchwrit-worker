# Patchwrit worker — smoke image (W2).
# Build: static Go binary. Runtime: scratch (no shell, no tools), so the
# smoke result honestly reports git/java/maven as null. JDK images with
# git + JDK + Maven are separate prebuilt variants (see docs).
FROM golang:1.26-alpine3.22 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/patchwrit-worker ./cmd/patchwrit-worker

FROM scratch
COPY --from=build /out/patchwrit-worker /patchwrit-worker
ENTRYPOINT ["/patchwrit-worker"]
