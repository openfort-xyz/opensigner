# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:26.12.0-alpine3.23@sha256:2ae3a23c9c5d2129398f4844c7c43400c061faf6973852f0fa837323bff9a5a8 AS deps

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

# Manifests only, so the dependency layer stays cached across source edits.
# .npmrc must be present or the install will not honour the supply-chain
# settings it carries.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
RUN pnpm install --frozen-lockfile

# ---------------------------------------------------------------------------

FROM deps AS builder

COPY . .

# Compile ahead of time so the runtime image executes plain node with no
# TypeScript loader in the request path.
RUN pnpm build

# ---------------------------------------------------------------------------
# Migration stage. The compiled migration entry point calls the migration API
# exported by the same better-auth package that serves traffic. Keeping one
# package version prevents a separately versioned CLI from deriving a different
# schema than the runtime expects.
FROM builder AS migrate

ENV NODE_ENV=production

ENTRYPOINT ["node", "dist/migrate.js"]

# ---------------------------------------------------------------------------
# Runtime: production dependencies only.
FROM node:26.12.0-alpine3.23@sha256:2ae3a23c9c5d2129398f4844c7c43400c061faf6973852f0fa837323bff9a5a8 AS runtime

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
RUN pnpm install --frozen-lockfile --prod

COPY --from=builder /app/dist ./dist

# Several library safety checks -- including the refusal to boot on a default
# secret -- are gated on production mode, so set it in the image rather than
# relying on the deployment to supply it.
ENV NODE_ENV=production

RUN addgroup -S appgroup && adduser -S appuser -G appgroup \
    && chown -R appuser:appgroup /app

EXPOSE 3000

USER appuser

ENTRYPOINT ["node", "dist/server.js"]
