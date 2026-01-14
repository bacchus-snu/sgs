# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sgs-runc-wrapper ./cmd/sgs-containerd-shim

# Final stage - minimal image with just the binary
FROM scratch

COPY --from=builder /app/sgs-runc-wrapper /sgs-runc-wrapper

ENTRYPOINT ["/sgs-runc-wrapper"]
