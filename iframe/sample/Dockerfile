# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:26.13.1-alpine3.23@sha256:39d8c143e60e99ae98f014bebea1e72fc96d3c9bdf9504bc3a27af5c05bfa39f

RUN npm install -g http-server

WORKDIR /app

COPY . .

USER node
ENTRYPOINT [ "npx", "http-server", "." ]
