FROM golang:1.21-bullseye AS builder

RUN apt-get update && apt-get install -y librdkafka-dev pkg-config && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/transaction-service ./main.go

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y librdkafka1 ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/transaction-service .

EXPOSE 8080

CMD ["./transaction-service"]
