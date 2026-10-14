# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o telemetry-service

# Runtime stage
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/telemetry-service .

EXPOSE 8083

CMD ["./telemetry-service"]
