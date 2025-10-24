FROM node:24-alpine3.22

# The following dependencies are required by @better-auth/cli@latest.
RUN apk add --no-cache build-base python3

WORKDIR /app

COPY . .

RUN yarn install

RUN npm install @better-auth/cli@1.3.4

EXPOSE 3000

ENTRYPOINT ["./entrypoint.sh"]
