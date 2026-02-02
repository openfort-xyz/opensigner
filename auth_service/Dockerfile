FROM node:24-alpine3.22

# The following dependencies are required by @better-auth/cli@latest.
RUN apk add --no-cache build-base python3

WORKDIR /app

COPY . .

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
RUN pnpm install --frozen-lockfile

EXPOSE 3000

ENTRYPOINT ["./entrypoint.sh"]
