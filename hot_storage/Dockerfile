FROM golang:1.24.5-alpine3.22

RUN apk add --no-cache curl

COPY sample /app

WORKDIR /app

RUN go build .

EXPOSE 3000

CMD ["./sample"]
