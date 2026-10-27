# Patchwrit worker — smoke image.
# Build: static Go binary. Runtime: alpine with CA certificates — the
# worker makes HTTPS callbacks, and a scratch image has no trust roots
# (verified live: x509 unknown authority). Toolchain variants (JDK
# images) are separate prebuilt images; see docs.
FROM golang:1.26-alpine3.22 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/patchwrit-worker ./cmd/patchwrit-worker

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/patchwrit-worker /patchwrit-worker
ENTRYPOINT ["/patchwrit-worker"]
