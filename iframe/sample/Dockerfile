# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:24.22.1-alpine3.23@sha256:0468540aa58203ff9e40c2aab3907c52644ab818af6176b1c70a026137dc92a2

RUN npm install -g http-server

WORKDIR /app

COPY . .

USER node
ENTRYPOINT [ "npx", "http-server", "." ]
