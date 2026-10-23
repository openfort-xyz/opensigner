# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:26.12.0-alpine3.23@sha256:2ae3a23c9c5d2129398f4844c7c43400c061faf6973852f0fa837323bff9a5a8

RUN npm install -g http-server

WORKDIR /app

COPY . .

USER node
ENTRYPOINT [ "npx", "http-server", "." ]
