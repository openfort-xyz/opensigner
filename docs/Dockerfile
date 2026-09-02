# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:24-alpine3.21@sha256:b8f7c9056af700568c1ce76173f1c93743fb64ca1343e18cdf3a6ded8985ad3d AS builder

WORKDIR /app

RUN apk add --no-cache curl git make

# D2 is installed from a pinned release tarball verified against a known
# checksum; the install script is the single source of that pin, shared with CI.
COPY scripts/install-d2.sh scripts/install-d2.sh
RUN PREFIX=/usr/local scripts/install-d2.sh

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

# Manifests first, so the dependency layer stays cached across content edits.
# .npmrc must be present or the install will not honour the supply-chain settings
# it carries.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./

# The registry token is mounted as a BuildKit secret rather than passed as a build
# arg. Build args are recorded in image metadata and readable with `docker
# history`; a secret mount exists only for the duration of this layer.
#
# CI=true tells pnpm it is non-interactive, so it reports a dependency mismatch
# instead of waiting on a confirmation prompt that can never arrive.
ENV CI=true
RUN --mount=type=secret,id=node_auth_token \
    NODE_AUTH_TOKEN="$(cat /run/secrets/node_auth_token 2>/dev/null || true)" \
    pnpm install --frozen-lockfile

COPY . .

RUN pnpm build

# Runtime: a static file server over the prerendered output.
#
# The build is full-static, so nothing here needs Node, pnpm, or the build
# dependency tree — which also keeps that tree's advisories out of the served
# image. Pinned by digest: a tag can be repointed at a different image at any time.
FROM nginx:alpine@sha256:4a73073bd557c65b759505da037898b61f1be6cbcc3c2c3aeac22d2a470c1752

COPY --from=builder /app/dist/public /usr/share/nginx/html
COPY docs_nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 4173
