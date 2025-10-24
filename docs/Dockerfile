FROM node:24-alpine3.21 AS builder

ARG NODE_AUTH_TOKEN
ENV NODE_AUTH_TOKEN=$NODE_AUTH_TOKEN

WORKDIR /app

# Install D2
RUN apk add --no-cache curl git make
RUN curl -fsSL https://d2lang.com/install.sh | sh -s --

COPY . .
COPY ./tsconfig.base.container.json ../tsconfig.base.json

# Install TS dependencies
RUN yarn

RUN yarn build

ENTRYPOINT ["yarn", "preview"]
