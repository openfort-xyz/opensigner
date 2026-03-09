FROM golang:1.24.5-alpine3.22

RUN apk add --no-cache curl

COPY sample /app

WORKDIR /app

RUN go build .

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

EXPOSE 3000

USER appuser
CMD ["./sample"]
