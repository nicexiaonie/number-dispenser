# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o number-dispenser cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/number-dispenser .

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 6380

# Run the application
ENTRYPOINT ["./number-dispenser"]
CMD ["-addr", ":6380", "-data", "/app/data"]

