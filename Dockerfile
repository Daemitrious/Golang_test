FROM golang:1.23-alpine AS builder

ARG APP=service

WORKDIR /app
COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${APP}

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/app /app/app

EXPOSE 8080
CMD ["/app/app"]
