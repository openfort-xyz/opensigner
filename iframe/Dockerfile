FROM node:24-alpine3.22

ARG NODE_AUTH_TOKEN
ENV NODE_AUTH_TOKEN=$NODE_AUTH_TOKEN

WORKDIR /app

COPY ./iframe .

RUN yarn

RUN yarn build

ENTRYPOINT [ "yarn", "dev" ]
