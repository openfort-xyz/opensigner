# Base images are pinned by digest, not tag. Tags are mutable, so a tag alone
# means the image a build produces today need not match the one it produced
# yesterday.
FROM golang:1.25.11-alpine3.22@sha256:65b4400aee0927412e9ed791a11893273a49d55df24841f7599660fb80dae464 AS builder

WORKDIR /src

# Copy manifests first so the dependency layer is cached independently of source
# changes.
COPY sample/go.mod sample/go.sum ./
RUN go mod download

COPY sample/ ./

# CGO_ENABLED=0 yields a static binary, which is what lets the runtime stage be a
# bare image with no libc. -trimpath keeps build-host paths out of the binary, and
# -w -s drop DWARF and the symbol table.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/hot_storage .

# ---------------------------------------------------------------------------

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# ca-certificates is needed to verify TLS when fetching the JWKS; curl serves the
# container healthcheck. Nothing else from the build stage reaches this image --
# no compiler, no module cache, no source.
RUN apk add --no-cache ca-certificates curl \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup

COPY --from=builder /out/hot_storage /usr/local/bin/hot_storage

USER appuser

# Matches the service's default listen port; override with PORT.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/hot_storage"]
