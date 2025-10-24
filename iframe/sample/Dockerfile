FROM node:24-alpine3.22

RUN npm install -g http-server

WORKDIR /app

COPY . .

ENTRYPOINT [ "npx", "http-server", "." ]
