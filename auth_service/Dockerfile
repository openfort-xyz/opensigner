FROM node:24-alpine3.22

# The following dependencies are required by @better-auth/cli@latest.
RUN apk add --no-cache build-base python3

WORKDIR /app

COPY . .

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
RUN pnpm install --frozen-lockfile

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

EXPOSE 3000

USER appuser
ENTRYPOINT ["./entrypoint.sh"]
