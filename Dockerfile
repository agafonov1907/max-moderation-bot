# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first to leverage cache
COPY go.mod go.sum ./
RUN go mod download

# Install goose migration tool
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /app/bot .
# Copy goose binary from builder
COPY --from=builder /go/bin/goose /usr/local/bin/goose
# Copy migrations
COPY --from=builder /app/migrations ./migrations
# Copy startup script
COPY --from=builder /app/start.sh .

# Make startup script executable
RUN chmod +x start.sh

# Create a non-root user
RUN adduser -D -g '' botuser
USER botuser

# Expose port (if needed for webhook)
EXPOSE 8080
EXPOSE 9090

CMD ["./start.sh"]
