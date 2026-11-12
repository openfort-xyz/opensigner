# Pinned by digest: a tag can be repointed at a different image at any time.
FROM node:26.12.1-alpine3.23@sha256:bc2b334006f15c7167bbeb2f08f28c2df883f7594a0470e04454ebe7560cfe68

RUN npm install -g http-server

WORKDIR /app

COPY . .

USER node
ENTRYPOINT [ "npx", "http-server", "." ]
