FROM node:24-alpine3.21 AS builder

ARG NODE_AUTH_TOKEN

WORKDIR /app

# Install D2
RUN apk add --no-cache curl git make
RUN curl -fsSL https://d2lang.com/install.sh | sh -s --

COPY . .
COPY ./tsconfig.base.container.json ../tsconfig.base.json

# Install TS dependencies
RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
RUN NODE_AUTH_TOKEN=$NODE_AUTH_TOKEN pnpm install --frozen-lockfile

RUN pnpm build

FROM node:24-alpine3.21

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/node_modules ./node_modules

USER node
ENTRYPOINT ["pnpm", "preview"]
