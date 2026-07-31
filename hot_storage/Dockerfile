# Base images are pinned by digest, not tag. Tags are mutable, so a tag alone
# means the image a build produces today need not match the one it produced
# yesterday.
FROM golang:1.26.4-alpine3.22@sha256:727cfc3c40be55cd1bc9a4a059406b28a059857e3be752aa9d09531e12c20c56 AS builder

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

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

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
