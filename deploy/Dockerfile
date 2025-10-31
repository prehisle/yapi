# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/gateway

FROM alpine:3.19
RUN addgroup -S gateway && adduser -S gateway -G gateway
COPY --from=builder /out/gateway /usr/local/bin/gateway
USER gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
